package gateway

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// wsGUID is the fixed WebSocket handshake GUID from RFC 6455 section 1.3.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsClient is a minimal RFC 6455 WebSocket client (standard library only,
// mirroring the server-side implementation in co-shell's web/ws.go). It is
// used by the hub to connect to each co-shell agent's /ws endpoint.
type wsClient struct {
	conn net.Conn
	br   *bufio.Reader

	wmu sync.Mutex
}

// dialWS performs the WebSocket client handshake against wsURL (e.g.
// "ws://127.0.0.1:8399/ws") and returns an established wsClient.
func dialWS(wsURL string) (*wsClient, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}
	scheme := u.Scheme
	if scheme != "ws" && scheme != "http" {
		return nil, fmt.Errorf("unsupported ws scheme %q", u.Scheme)
	}
	host := u.Host
	if u.Port() == "" {
		if scheme == "ws" {
			host += ":80"
		} else {
			host += ":80"
		}
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}

	// Build the handshake request.
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read handshake response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %s", resp.Status)
	}
	expected := acceptKey(key)
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != expected {
		conn.Close()
		return nil, fmt.Errorf("unexpected Sec-WebSocket-Accept %q", got)
	}
	return &wsClient{conn: conn, br: br}, nil
}

// acceptKey computes the Sec-WebSocket-Accept value for a client key.
func acceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// WriteText sends one text message. Client frames must be masked (RFC 6455
// section 5.1). A write deadline bounds each write.
func (c *wsClient) WriteText(payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer c.conn.SetWriteDeadline(time.Time{})

	header := []byte{0x80 | 0x1} // FIN + text
	switch n := len(payload); {
	case n < 126:
		header = append(header, 0x80|byte(n)) // masked bit set
	case n <= 0xFFFF:
		header = append(header, 0x80|126, byte(n>>8), byte(n))
	default:
		header = append(header, 0x80|127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(n))
		header = append(header, buf[:]...)
	}
	// Mask key.
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	header = append(header, mask[:]...)
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := c.conn.Write(masked)
	return err
}

// ReadMessage reads one complete text/binary message, reassembling
// continuation frames. Ping frames are answered with pong automatically.
func (c *wsClient) ReadMessage() ([]byte, error) {
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
		case 0x1, 0x2: // text / binary
			if f.fin {
				return f.payload, nil
			}
			frag = append(frag[:0], f.payload...)
			fragActive = true
		case 0x0: // continuation
			if !fragActive {
				return nil, errors.New("continuation frame without active message")
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

// wsFrame is one decoded WebSocket frame (server frames are unmasked).
type wsFrame struct {
	fin     bool
	opcode  byte
	payload []byte
}

// readFrame reads and decodes one frame from the server.
func (c *wsClient) readFrame() (*wsFrame, error) {
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

// writeFrame writes one frame (used for pong/close replies; unmasked).
func (c *wsClient) writeFrame(opcode byte, payload []byte) error {
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
func (c *wsClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// isWebSocketURL reports whether s looks like a ws:// or http:// URL.
func isWebSocketURL(s string) bool {
	return strings.HasPrefix(s, "ws://") || strings.HasPrefix(s, "http://")
}
