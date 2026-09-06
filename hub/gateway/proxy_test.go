package gateway

import (
	"context"
	"encoding/json"
	"net"
	"testing"
)

// TestAcceptKey verifies the WebSocket accept key against the RFC 6455
// section 1.3 example.
func TestAcceptKey(t *testing.T) {
	// RFC 6455 example: key "dGhlIHNhbXBsZSBub25jZQ==" →
	// accept "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	got := acceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("acceptKey mismatch: got %q want %q", got, want)
	}
}

// TestProxyRouting verifies that a client message is forwarded to the current
// agent and that switch_agent changes the target. It uses a fake agent conn
// (no real WebSocket) by injecting a stub into the proxy.
func TestProxyRouting(t *testing.T) {
	ctx := context.Background()
	p := NewProxy(ctx, nil) // no real agents

	// Inject two fake agent conns.
	fakeA := &fakeAgentConn{id: "a", name: "Agent A"}
	fakeB := &fakeAgentConn{id: "b", name: "Agent B"}
	p.agents["a"] = fakeA
	p.agents["b"] = fakeB
	p.order = []string{"a", "b"}

	// Simulate a client connection backed by an in-memory pipe so Conn.Send
	// (which writes to the socket) does not panic. A goroutine drains reads.
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := srv.Read(buf); err != nil {
				return
			}
		}
	}()
	client := &Conn{conn: cli}
	p.OnConnect(ctx, client)

	// Forward a business message → should go to agent "a" (default).
	env := &Envelope{Type: "input", Payload: json.RawMessage(`{"type":"input","text":"hi"}`)}
	p.HandleMessage(ctx, client, env)
	if fakeA.last == nil || string(fakeA.last) != string(env.Payload) {
		t.Fatalf("expected message forwarded to agent a, got %v", fakeA.last)
	}
	if fakeB.last != nil {
		t.Fatalf("agent b should not have received the message")
	}

	// Switch to agent b.
	sw, _ := json.Marshal(switchAgentRequest{AgentID: "b"})
	p.HandleMessage(ctx, client, &Envelope{Type: MsgTypeSwitchAgent, Payload: sw})

	// Forward another message → should go to agent b now.
	env2 := &Envelope{Type: "input", Payload: json.RawMessage(`{"type":"input","text":"yo"}`)}
	p.HandleMessage(ctx, client, env2)
	if fakeB.last == nil || string(fakeB.last) != string(env2.Payload) {
		t.Fatalf("expected message forwarded to agent b after switch, got %v", fakeB.last)
	}
}

// fakeAgentConn is a stub AgentConn for routing tests.
type fakeAgentConn struct {
	id    string
	name  string
	last  []byte
	subs  map[*Conn]struct{}
}

func (f *fakeAgentConn) ID() string   { return f.id }
func (f *fakeAgentConn) Name() string { return f.name }
func (f *fakeAgentConn) Subscribe(c *Conn) {
	if f.subs == nil {
		f.subs = map[*Conn]struct{}{}
	}
	f.subs[c] = struct{}{}
}
func (f *fakeAgentConn) Unsubscribe(c *Conn) {
	if f.subs != nil {
		delete(f.subs, c)
	}
}
func (f *fakeAgentConn) Send(data []byte) error { f.last = data; return nil }
func (f *fakeAgentConn) Close()                 {}
