// Package web - WebSocket protocol tests (FEATURE-307c): handshake accept
// vector from RFC 6455, frame round-trip (mask / 16-bit length / continuation
// reassembly), ping->pong, close handshake and oversize-frame rejection,
// using a minimal stdlib-only test client.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAcceptKey verifies the Sec-WebSocket-Accept computation against the
// known test vector from RFC 6455 section 1.3.
func TestAcceptKey(t *testing.T) {
	got := acceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Errorf("acceptKey = %q, want %q", got, want)
	}
}

// wsTestClient is a minimal client-side WebSocket connection over a raw
// net.Conn (the server under test hijacks the other end).
type wsTestClient struct {
	conn net.Conn
	br   *bufio.Reader
}

// dialWS performs the client handshake against a test server hosting the
// given handler (whose "/ws" route is expected to upgrade the connection).
func dialWS(t *testing.T, handler http.Handler) *wsTestClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, err := net.Dial("tcp", strings.TrimPrefix(url, "ws://"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	key := "dGhlIHNhbXBsZSBub25jZQ=="
	fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: keep-alive, Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		strings.TrimPrefix(url, "ws://"), key)
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("handshake status = %q, want 101", status)
	}
	acceptSeen := false
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake header: %v", err)
		}
		if strings.HasPrefix(line, "Sec-WebSocket-Accept:") {
			if !strings.Contains(line, "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=") {
				t.Errorf("bad accept header: %q", line)
			}
			acceptSeen = true
		}
		if line == "\r\n" {
			break
		}
	}
	if !acceptSeen {
		t.Fatalf("handshake response missing Sec-WebSocket-Accept")
	}

	return &wsTestClient{conn: conn, br: br}
}

// wsHandler adapts a func(*wsConn) to an http.Handler that upgrades "/ws".
func wsHandler(fn func(*wsConn)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrade(w, r)
		if err != nil {
			return
		}
		fn(c)
	})
}

// writeFrame writes one masked client frame (clients must mask, RFC 6455
// section 5.3).
func (c *wsTestClient) writeFrame(fin bool, opcode byte, payload []byte) error {
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	header := []byte{b0}
	maskBit := byte(0x80)
	switch n := len(payload); {
	case n < 126:
		header = append(header, maskBit|byte(n))
	case n <= 0xFFFF:
		header = append(header, maskBit|126, byte(n>>8), byte(n))
	default:
		header = append(header, maskBit|127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(n))
		header = append(header, buf[:]...)
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if _, err := c.conn.Write(mask[:]); err != nil {
		return err
	}
	_, err := c.conn.Write(masked)
	return err
}

// readFrame reads one server frame (unmasked).
func (c *wsTestClient) readFrame() (opcode byte, payload []byte, err error) {
	b0, err := c.br.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	b1, err := c.br.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	opcode = b0 & 0x0F
	length := uint64(b1 & 0x7F)
	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}
	payload = make([]byte, length)
	_, err = io.ReadFull(c.br, payload)
	return opcode, payload, err
}

// TestWSRoundTrip verifies masked text frames (including a 16-bit extended
// length payload) are echoed back unmasked and intact.
func TestWSRoundTrip(t *testing.T) {
	echo := make(chan struct{})
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		for {
			msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if err := c.WriteMessage(msg); err != nil {
				return
			}
			echo <- struct{}{}
		}
	}))

	payloads := []string{
		"hello",
		strings.Repeat("x", 70000), // exceeds 0xFFFF? no: 70000 > 65535 -> 64-bit length
		strings.Repeat("y", 1000),  // 16-bit extended length
	}
	for _, p := range payloads {
		if err := client.writeFrame(true, wsOpText, []byte(p)); err != nil {
			t.Fatalf("client write: %v", err)
		}
		op, got, err := client.readFrame()
		if err != nil {
			t.Fatalf("client read: %v", err)
		}
		if op != wsOpText {
			t.Errorf("echo opcode = %d, want text(%d)", op, wsOpText)
		}
		if string(got) != p {
			t.Errorf("echo payload length %d, want %d", len(got), len(p))
		}
		<-echo
	}
}

// TestWSContinuation verifies fragmented client messages are reassembled.
func TestWSContinuation(t *testing.T) {
	done := make(chan string, 1)
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		msg, err := c.ReadMessage()
		if err != nil {
			t.Errorf("server ReadMessage: %v", err)
			return
		}
		done <- string(msg)
	}))

	full := "fragmented-message-payload"
	if err := client.writeFrame(false, wsOpText, []byte(full[:10])); err != nil {
		t.Fatalf("write fragment 1: %v", err)
	}
	if err := client.writeFrame(false, wsOpContinuation, []byte(full[10:20])); err != nil {
		t.Fatalf("write fragment 2: %v", err)
	}
	if err := client.writeFrame(true, wsOpContinuation, []byte(full[20:])); err != nil {
		t.Fatalf("write fragment 3: %v", err)
	}
	if got := <-done; got != full {
		t.Errorf("reassembled = %q, want %q", got, full)
	}
}

// TestWSPingPong verifies the server answers a ping with a pong carrying the
// same payload, and keeps reading afterwards.
func TestWSPingPong(t *testing.T) {
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		for {
			if _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	if err := client.writeFrame(true, wsOpPing, []byte("hb")); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	op, payload, err := client.readFrame()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if op != wsOpPong || string(payload) != "hb" {
		t.Errorf("pong = (op %d, %q), want (pong, \"hb\")", op, payload)
	}
}

// TestWSClose verifies a client close frame is answered with a close frame.
func TestWSClose(t *testing.T) {
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		for {
			if _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	if err := client.writeFrame(true, wsOpClose, nil); err != nil {
		t.Fatalf("write close: %v", err)
	}
	op, _, err := client.readFrame()
	if err != nil {
		t.Fatalf("read close reply: %v", err)
	}
	if op != wsOpClose {
		t.Errorf("close reply opcode = %d, want close(%d)", op, wsOpClose)
	}
}

// TestWSOversizeFrameRejected verifies a frame declaring a payload above the
// limit makes ReadMessage fail.
func TestWSOversizeFrameRejected(t *testing.T) {
	done := make(chan error, 1)
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		_, err := c.ReadMessage()
		done <- err
	}))
	// Declare a 5 MiB payload (over the 4 MiB limit) but send nothing; the
	// server must reject at the length check before reading the payload.
	header := []byte{0x80 | wsOpText, 0x80 | 127}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(5<<20))
	header = append(header, buf[:]...)
	if _, err := client.conn.Write(header); err != nil {
		t.Fatalf("write oversize header: %v", err)
	}
	if err := <-done; err == nil {
		t.Errorf("oversize frame: ReadMessage succeeded, want error")
	}
}
