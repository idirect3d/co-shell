package gateway

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Message envelope types exchanged over the gateway TCP connection.
const (
	// MsgTypeAuth is sent by a client to authenticate with its API key.
	MsgTypeAuth = "auth"
	// MsgTypeAuthAck is sent by the server after successful authentication.
	MsgTypeAuthAck = "auth_ack"
	// MsgTypeError is sent by the server to report an error.
	MsgTypeError = "error"
	// MsgTypePing / MsgTypePong implement a lightweight keep-alive.
	MsgTypePing = "ping"
	MsgTypePong = "pong"
)

// Envelope is the top-level JSON frame exchanged over the wire.
// After authentication, arbitrary business messages are carried in the
// "payload" field and forwarded transparently by the gateway (no business
// logic is interpreted here).
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// AuthRequest is the payload of MsgTypeAuth.
type AuthRequest struct {
	APIKey string `json:"api_key"`
}

// frameHeaderSize is the size of the length prefix (4-byte big-endian).
const frameHeaderSize = 4

// writeFrame writes a length-prefixed JSON frame to w.
func writeFrame(w io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}
	if len(data) > maxFrameSize {
		return fmt.Errorf("frame too large: %d bytes", len(data))
	}
	header := make([]byte, frameHeaderSize)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// readFrame reads a length-prefixed JSON frame from r and unmarshals it into v.
func readFrame(r io.Reader, maxSize int, v interface{}) error {
	header := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(header)
	if int(length) > maxSize {
		return fmt.Errorf("frame too large: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
