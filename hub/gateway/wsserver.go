package gateway

import (
	"bufio"
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

// wsServerConn is a server-side WebSocket connection (for browser clients of
// the hub Web UI). It mirrors the frame logic of co-shell's web/ws.go but is
// self-contained in the hub module. Outbound messages are written directly
// under a mutex with a write deadline.
type wsServerConn struct {
	conn net.Conn
	br   *bufio.Reader
	wmu  sync.Mutex
}

// upgradeWS performs the server-side WebSocket handshake on an HTTP request.
func upgradeWS(w http.ResponseWriter, r *http.Request) (*wsServerConn, error) {
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
	return &wsServerConn{conn: conn, br: rw.Reader}, nil
}

// headerHasToken reports whether the named header contains the given token.
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

// WriteText sends one text message (server frames are unmasked).
func (c *wsServerConn) WriteText(payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer c.conn.SetWriteDeadline(time.Time{})
	header := []byte{0x80 | 0x1}
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

// ReadMessage reads one complete text/binary message, answering pings.
func (c *wsServerConn) ReadMessage() ([]byte, error) {
	var frag []byte
	fragActive := false
	for {
		f, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch f.opcode {
		case 0x9: // ping
			if err := c.writeFrame(0xA, f.payload); err != nil {
				return nil, err
			}
			continue
		case 0xA: // pong
			continue
		case 0x8: // close
			_ = c.writeFrame(0x8, f.payload)
			return nil, io.EOF
		case 0x1, 0x2:
			if f.fin {
				return f.payload, nil
			}
			frag = append(frag[:0], f.payload...)
			fragActive = true
		case 0x0:
			if !fragActive {
				return nil, errors.New("continuation without active message")
			}
			frag = append(frag, f.payload...)
			if f.fin {
				return frag, nil
			}
		default:
			return nil, fmt.Errorf("unsupported opcode %d", f.opcode)
		}
	}
}

// readFrame reads one frame from the browser (client frames are masked).
func (c *wsServerConn) readFrame() (*wsFrame, error) {
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
	if length > 4<<20 {
		return nil, fmt.Errorf("frame payload %d exceeds limit", length)
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

// writeFrame writes one frame (used for pong/close replies).
func (c *wsServerConn) writeFrame(opcode byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
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

// Close closes the underlying connection.
func (c *wsServerConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
