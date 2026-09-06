package gateway

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

// testHandler records received messages for assertions.
type testHandler struct {
	mu       sync.Mutex
	messages []*Envelope
	connects int
}

func (h *testHandler) HandleMessage(_ context.Context, _ *Conn, env *Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, env)
}

func (h *testHandler) OnConnect(_ context.Context, _ *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connects++
}

func (h *testHandler) OnDisconnect(_ context.Context, _ *Conn) {}

// dialAndAuth connects to the server and performs the auth handshake.
func dialAndAuth(t *testing.T, addr string, apiKey string) (net.Conn, *Envelope) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	payload, _ := json.Marshal(AuthRequest{APIKey: apiKey})
	if err := writeFrame(conn, &Envelope{Type: MsgTypeAuth, Payload: payload}); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	var ack Envelope
	if err := readFrame(conn, maxFrameSize, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	return conn, &ack
}

func startTestServer(t *testing.T, apiKey string) (*Server, *testHandler, string) {
	t.Helper()
	h := &testHandler{}
	cfg := DefaultConfig()
	cfg.ListenAddr = "127.0.0.1:0"
	cfg.APIKey = apiKey
	srv := NewServer(cfg, h)
	if err := srv.Listen(); err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return srv, h, srv.Addr().String()
}

func TestAuthSuccess(t *testing.T) {
	_, _, addr := startTestServer(t, "secret-key")
	conn, ack := dialAndAuth(t, addr, "secret-key")
	defer conn.Close()
	if ack.Type != MsgTypeAuthAck {
		t.Fatalf("expected auth_ack, got %q", ack.Type)
	}
}

func TestAuthWrongKey(t *testing.T) {
	_, _, addr := startTestServer(t, "secret-key")
	conn, ack := dialAndAuth(t, addr, "wrong-key")
	defer conn.Close()
	if ack.Type != MsgTypeError {
		t.Fatalf("expected error, got %q", ack.Type)
	}
}

func TestAuthRequiredBeforeBusiness(t *testing.T) {
	_, _, addr := startTestServer(t, "secret-key")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// Send a business message without authenticating first.
	if err := writeFrame(conn, &Envelope{Type: "input", Payload: json.RawMessage(`"hi"`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var resp Envelope
	if err := readFrame(conn, maxFrameSize, &resp); err != nil {
		t.Fatalf("read: %v", err)
	}
	if resp.Type != MsgTypeError {
		t.Fatalf("expected error, got %q", resp.Type)
	}
}

func TestBusinessMessageForwarded(t *testing.T) {
	_, h, addr := startTestServer(t, "secret-key")
	conn, _ := dialAndAuth(t, addr, "secret-key")
	defer conn.Close()

	payload := json.RawMessage(`{"agent_id":"a1","content":"hello"}`)
	if err := writeFrame(conn, &Envelope{Type: "input", Payload: payload}); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		h.mu.Lock()
		n := len(h.messages)
		h.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for forwarded message")
		}
		time.Sleep(10 * time.Millisecond)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(h.messages))
	}
	if h.messages[0].Type != "input" {
		t.Fatalf("expected type input, got %q", h.messages[0].Type)
	}
	if string(h.messages[0].Payload) != string(payload) {
		t.Fatalf("payload mismatch: %s != %s", h.messages[0].Payload, payload)
	}
}

func TestPingPong(t *testing.T) {
	_, _, addr := startTestServer(t, "secret-key")
	conn, _ := dialAndAuth(t, addr, "secret-key")
	defer conn.Close()

	if err := writeFrame(conn, &Envelope{Type: MsgTypePing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	var resp Envelope
	if err := readFrame(conn, maxFrameSize, &resp); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if resp.Type != MsgTypePong {
		t.Fatalf("expected pong, got %q", resp.Type)
	}
}
