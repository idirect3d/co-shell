// Unit tests for the CDP event channel (readLoop dispatch, response path,
// mixed streams, WaitEvent timeout and unsubscribe).
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0019.md (group A, A-001..A-005).
// These tests talk to a local WebSocket server, so they need no Chrome.

package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestCDPServer starts a WebSocket server running handler for each
// connection and returns its ws:// URL plus a cleanup function.
func newTestCDPServer(t *testing.T, handler func(*websocket.Conn)) (string, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}))

	return "ws" + strings.TrimPrefix(server.URL, "http"), server.Close
}

// TestReadLoopDispatchesEvents covers A-001: an event message (method, no id)
// reaches the subscriber with its params intact.
func TestReadLoopDispatchesEvents(t *testing.T) {
	release := make(chan struct{})
	wsURL, cleanup := newTestCDPServer(t, func(conn *websocket.Conn) {
		<-release
		_ = conn.WriteJSON(map[string]interface{}{
			"method": "Page.loadEventFired",
			"params": map[string]interface{}{"timestamp": 42.0},
		})
		time.Sleep(300 * time.Millisecond)
	})
	defer cleanup()

	client, err := NewCDPClient(wsURL)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	ch, cancel := client.Watch("Page.loadEventFired")
	defer cancel()
	close(release)

	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed unexpectedly")
		}
		if ev.Method != "Page.loadEventFired" {
			t.Fatalf("method = %q, want Page.loadEventFired", ev.Method)
		}
		if got := jsonFloat(ev.Params["timestamp"]); got != 42 {
			t.Fatalf("timestamp = %v, want 42", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event dispatched to subscriber")
	}
}

// TestCallResponsePath covers A-002: responses still reach the Call waiter.
func TestCallResponsePath(t *testing.T) {
	wsURL, cleanup := newTestCDPServer(t, func(conn *websocket.Conn) {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				return
			}
			_ = conn.WriteJSON(map[string]interface{}{
				"id": req.ID,
				"result": map[string]interface{}{
					"ok":     true,
					"method": req.Method,
				},
			})
		}
	})
	defer cleanup()

	client, err := NewCDPClient(wsURL)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := client.Call(ctx, "Runtime.evaluate", nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("result = %v, want ok=true", result)
	}
	if got, _ := result["method"].(string); got != "Runtime.evaluate" {
		t.Fatalf("method = %q, want Runtime.evaluate", got)
	}
}

// TestEventAndResponseInterleaved covers A-003: events (even ones carrying a
// synthetic "id":0) and command responses each reach their own consumer.
func TestEventAndResponseInterleaved(t *testing.T) {
	const rounds = 3

	wsURL, cleanup := newTestCDPServer(t, func(conn *websocket.Conn) {
		for i := 0; i < rounds; i++ {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				return
			}
			_ = conn.WriteJSON(map[string]interface{}{
				"id":     0,
				"method": "Page.domContentEventFired",
				"params": map[string]interface{}{"seq": float64(i)},
			})
			_ = conn.WriteJSON(map[string]interface{}{
				"id":     req.ID,
				"result": map[string]interface{}{"seq": float64(i)},
			})
		}
		time.Sleep(300 * time.Millisecond)
	})
	defer cleanup()

	client, err := NewCDPClient(wsURL)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	ch, cancel := client.Watch("Page.domContentEventFired")
	defer cancel()

	for i := 0; i < rounds; i++ {
		callCtx, cancelCall := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := client.Call(callCtx, "Runtime.evaluate", nil)
		cancelCall()
		if err != nil {
			t.Fatalf("round %d: Call failed: %v", i, err)
		}
		if got := jsonFloat(result["seq"]); got != float64(i) {
			t.Fatalf("round %d: response seq = %v, want %d", i, got, i)
		}

		select {
		case ev := <-ch:
			if got := jsonFloat(ev.Params["seq"]); got != float64(i) {
				t.Fatalf("round %d: event seq = %v, want %d", i, got, i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("round %d: no event delivered", i)
		}
	}
}

// TestWaitEventTimeout covers A-004: WaitEvent honours its deadline.
func TestWaitEventTimeout(t *testing.T) {
	wsURL, cleanup := newTestCDPServer(t, func(conn *websocket.Conn) {
		time.Sleep(500 * time.Millisecond)
	})
	defer cleanup()

	client, err := NewCDPClient(wsURL)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := client.WaitEvent(ctx, "Page.loadEventFired"); err == nil {
		t.Fatal("WaitEvent should fail when no event arrives")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("WaitEvent took %v, want a bounded wait", elapsed)
	}
}

// TestWatchCancelStopsDelivery covers A-005: a cancelled subscription is
// closed and receives nothing else.
func TestWatchCancelStopsDelivery(t *testing.T) {
	release := make(chan struct{})
	wsURL, cleanup := newTestCDPServer(t, func(conn *websocket.Conn) {
		<-release
		for i := 0; i < 3; i++ {
			_ = conn.WriteJSON(map[string]interface{}{
				"method": "Page.loadEventFired",
				"params": map[string]interface{}{"seq": float64(i)},
			})
		}
		time.Sleep(300 * time.Millisecond)
	})
	defer cleanup()

	client, err := NewCDPClient(wsURL)
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	ch, cancel := client.Watch("Page.loadEventFired")
	cancel()
	close(release)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("event delivered after the subscription was cancelled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled subscription channel was not closed")
	}
}
