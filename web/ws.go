// Package web - WebSocket protocol implementation (RFC 6455) on the standard
// library only: handshake (SHA1+base64 accept), frame encode/decode, client
// mask handling, continuation reassembly, ping/pong and close (FEATURE-307c).
// The project's dependency policy (docs/output-architecture.md 3.8) forbids
// new third-party dependencies, so no gorilla/websocket here.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// wsGUID is the fixed WebSocket handshake GUID from RFC 6455 section 1.3.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Frame/message size limits guard the loopback server against absurd frames.
const (
	maxWSFramePayload   = 4 << 20 // 4 MiB per frame
	maxWSMessagePayload = 4 << 20 // 4 MiB per reassembled message
)

// FIX-475: wsWriteTimeout bounds each socket write so a stalled browser (full
// TCP receive buffer) cannot block the writer goroutine forever. On timeout the
// connection is closed and the agent keeps running (events are dropped instead
// of freezing the agent loop).
const wsWriteTimeout = 10 * time.Second

// wsOutQueueSize is the bounded outbound message queue. It decouples the agent
// goroutine from the socket write: producers enqueue (non-blocking) and a
// dedicated writer goroutine drains the queue. When the queue is full (client
// reading slower than the agent emits), the oldest events are dropped so the
// agent never blocks on the network.
const wsOutQueueSize = 512

// WebSocket opcodes (RFC 6455 section 5.2).
const (
	wsOpContinuation = 0x0
	wsOpText         = 0x1
	wsOpBinary       = 0x2
	wsOpClose        = 0x8
	wsOpPing         = 0x9
	wsOpPong         = 0xA
)

// acceptKey computes the Sec-WebSocket-Accept value for a client key:
// base64(SHA1(key + GUID)) (RFC 6455 section 4.2.2).
func acceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// headerHasToken reports whether the named header contains the given token
// (comma-separated, case-insensitive), e.g. "Connection: keep-alive, Upgrade".
func headerHasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

// wsConn is one server-side WebSocket connection. Outbound messages are
// enqueued to outCh (non-blocking) and drained by a dedicated writer goroutine,
// so producers (the agent loop, ask requests) never block on the socket. The
// writer goroutine and the read goroutine's pong/close replies both write via
// writeFrame, which serializes on wmu and applies a write deadline.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader

	wmu sync.Mutex

	// FIX-475: async outbound queue + writer lifecycle.
	outCh      chan []byte
	done       chan struct{}
	closeOnce  sync.Once
	writerDone chan struct{}

	// Fragmented message reassembly state (continuation frames).
	fragBuf    []byte
	fragActive bool
}

// upgrade validates the WebSocket handshake request, hijacks the HTTP
// connection and writes the 101 response.
func upgrade(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !headerHasToken(r.Header, "Connection", "upgrade") ||
		!headerHasToken(r.Header, "Upgrade", "websocket") {
		return nil, errors.New("not a websocket upgrade request")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey(key) + "\r\n\r\n"
	if _, err := rw.WriteString(resp); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	c := &wsConn{
		conn:       conn,
		br:         rw.Reader,
		outCh:      make(chan []byte, wsOutQueueSize),
		done:       make(chan struct{}),
		writerDone: make(chan struct{}),
	}
	// FIX-475: start the dedicated writer goroutine that drains outCh and
	// writes frames with a deadline, so producers never block on the socket.
	go c.writeLoop()
	return c, nil
}

// writeLoop drains the outbound queue and writes each message as a text frame
// with a write deadline. On a write error (stalled/dead client) it closes the
// connection so the server stops trying to push to a client that cannot keep
// up; producers keep running and simply drop further events.
func (c *wsConn) writeLoop() {
	defer close(c.writerDone)
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.outCh:
			if err := c.writeFrame(wsOpText, msg); err != nil {
				c.Close()
				return
			}
		}
	}
}

// wsFrame is one decoded WebSocket frame. Payload is already unmasked.
type wsFrame struct {
	fin     bool
	opcode  byte
	payload []byte
}

// readFrame reads and decodes one frame from the client. Masked and
// unmasked payloads are both accepted (loopback-only server); the 16/64-bit
// extended lengths and the per-frame size limit are enforced.
func (c *wsConn) readFrame() (*wsFrame, error) {
	b0, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	b1, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	f := &wsFrame{fin: b0&0x80 != 0, opcode: b0 & 0x0F}
	masked := b1&0x80 != 0
	length := uint64(b1 & 0x7F)
	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}
	if length > maxWSFramePayload {
		return nil, fmt.Errorf("websocket frame payload %d exceeds limit %d", length, maxWSFramePayload)
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, maskKey[:]); err != nil {
			return nil, err
		}
	}
	f.payload = make([]byte, length)
	if _, err := io.ReadFull(c.br, f.payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range f.payload {
			f.payload[i] ^= maskKey[i%4]
		}
	}
	return f, nil
}

// ReadMessage reads one complete text/binary message, reassembling
// continuation frames. Ping frames are answered with pong automatically;
// pong frames are ignored. A close frame produces a close reply and io.EOF.
func (c *wsConn) ReadMessage() ([]byte, error) {
	for {
		f, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch f.opcode {
		case wsOpPing:
			if err := c.writeFrame(wsOpPong, f.payload); err != nil {
				return nil, err
			}
			continue
		case wsOpPong:
			continue
		case wsOpClose:
			// Best-effort close reply, then report EOF to the caller.
			_ = c.writeFrame(wsOpClose, f.payload)
			return nil, io.EOF
		case wsOpText, wsOpBinary:
			if f.fin {
				return f.payload, nil
			}
			c.fragBuf = append(c.fragBuf[:0], f.payload...)
			c.fragActive = true
		case wsOpContinuation:
			if !c.fragActive {
				return nil, errors.New("websocket continuation frame without active message")
			}
			if len(c.fragBuf)+len(f.payload) > maxWSMessagePayload {
				return nil, fmt.Errorf("websocket message exceeds limit %d", maxWSMessagePayload)
			}
			c.fragBuf = append(c.fragBuf, f.payload...)
			if f.fin {
				msg := c.fragBuf
				c.fragBuf = nil
				c.fragActive = false
				return msg, nil
			}
		default:
			return nil, fmt.Errorf("unsupported websocket opcode %d", f.opcode)
		}
	}
}

// writeFrame encodes and writes one frame. Server-to-client frames are
// never masked (RFC 6455 section 5.1). A write deadline bounds each write so a
// stalled client cannot block the caller forever (FIX-475).
func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	defer c.conn.SetWriteDeadline(time.Time{})
	header := []byte{0x80 | opcode}
	switch n := len(payload); {
	case n < 126:
		header = append(header, byte(n))
	case n <= 0xFFFF:
		header = append(header, 126, byte(n>>8), byte(n))
	default:
		header = append(header, 127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(n))
		header = append(header, buf[:]...)
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

// WriteMessage enqueues one complete text message to the outbound queue and
// returns immediately (non-blocking). The dedicated writer goroutine drains the
// queue and performs the actual socket write with a deadline, so the caller
// (agent loop / ask request) never blocks on the network. When the queue is
// full (client reading slower than the producer emits) the message is dropped
// and nil is returned, so the producer keeps running. Returns an error only
// when the connection is already closed.
func (c *wsConn) WriteMessage(payload []byte) error {
	select {
	case <-c.done:
		return errors.New("websocket connection closed")
	default:
	}
	select {
	case c.outCh <- payload:
		return nil
	case <-c.done:
		return errors.New("websocket connection closed")
	default:
		// Queue full: drop the message rather than block the producer.
		return nil
	}
}

// Close stops the writer goroutine, sends a close frame (best effort) and
// closes the connection. It is safe to call multiple times.
func (c *wsConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.writeFrame(wsOpClose, nil)
		_ = c.conn.Close()
	})
	return nil
}
