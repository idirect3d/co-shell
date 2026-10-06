// Package web - gateway slot tests (FEATURE-530): the hub link (WS query
// role=hub) lives in a slot of its own, so a browser tab showing this agent's
// UI never evicts it, and vice versa. Browser clients keep the single-client
// takeover semantics among themselves.
//
// Author: L.Shuang
// Created: 2026-09-17
// Last Modified: 2026-09-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"bufio"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// dialWSPath performs the client handshake against srv for the given request
// path (e.g. "/ws" or "/ws?role=hub").
func dialWSPath(t *testing.T, srv *httptest.Server, path string) *wsTestClient {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "http://")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	key := "dGhlIHNhbXBsZSBub25jZQ=="
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: keep-alive, Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host, key)
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("handshake status for %s = %q, want 101", path, status)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake header: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}
	return &wsTestClient{conn: conn, br: br}
}

// waitUntil polls cond for at most 2s (the connect hooks run in goroutines).
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// readFrameWithin reads one frame within d; it returns io.EOF when the peer
// closed the connection and a timeout error when nothing arrived.
func readFrameWithin(c *wsTestClient, d time.Duration) ([]byte, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	_, payload, err := c.readFrame()
	return payload, err
}

// TestGatewayLinkCoexistsWithBrowser covers the FEATURE-530 fix (a browser client
// must not evict the hub gateway link, and board frames reach both) together with
// the FEATURE-542 change: browsers no longer evict each other either — every tab
// keeps its own connection and all of them receive the same fan-out.
func TestGatewayLinkCoexistsWithBrowser(t *testing.T) {
	s := NewServer(t.TempDir(), ServerOptions{})
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	gw := dialWSPath(t, srv, "/ws?role=hub")
	waitUntil(t, "gateway slot", func() bool { return s.gwConn != nil })

	b1 := dialWSPath(t, srv, "/ws")
	waitUntil(t, "browser slot", func() bool { return s.HasClient() })

	// The browser's initial state frame must arrive (browser path unchanged).
	if payload, err := readFrameWithin(b1, 2*time.Second); err != nil {
		t.Fatalf("browser state frame: %v", err)
	} else if !strings.Contains(string(payload), `"state"`) {
		t.Errorf("browser first frame = %s, want a state frame", payload)
	}

	// The browser must not have taken the gateway slot away.
	if s.gwConn == nil {
		t.Fatal("browser client evicted the hub gateway link")
	}
	if !s.HasClient() {
		t.Fatal("browser must be registered while the gateway link is up")
	}

	// board frames reach both the hub link and the browser.
	if err := s.sendRaw([]byte(`{"kind":"board_roster","rev":7}`)); err != nil {
		t.Fatalf("sendRaw: %v", err)
	}
	if payload, err := readFrameWithin(gw, 2*time.Second); err != nil {
		t.Fatalf("gateway board frame: %v", err)
	} else if !strings.Contains(string(payload), "board_roster") {
		t.Errorf("gateway frame = %s, want board_roster", payload)
	}
	if payload, err := readFrameWithin(b1, 2*time.Second); err != nil {
		t.Fatalf("browser board frame: %v", err)
	} else if !strings.Contains(string(payload), "board_roster") {
		t.Errorf("browser frame = %s, want board_roster", payload)
	}

	// FEATURE-542 (R5): a second browser does NOT take over the first one — every
	// tab keeps its own connection and both receive the same fan-out.
	b2 := dialWSPath(t, srv, "/ws")
	waitUntil(t, "second browser", func() bool { return s.ClientCount() == 2 })
	if payload, err := readFrameWithin(b2, 2*time.Second); err != nil {
		t.Fatalf("second browser state frame: %v", err)
	} else if !strings.Contains(string(payload), `"state"`) {
		t.Errorf("second browser first frame = %s, want a state frame", payload)
	}
	if err := s.sendRaw([]byte(`{"kind":"board_roster","rev":8}`)); err != nil {
		t.Fatalf("sendRaw to both browsers: %v", err)
	}
	if payload, err := readFrameWithin(gw, 2*time.Second); err != nil {
		t.Fatalf("gateway frame after a second browser: %v", err)
	} else if !strings.Contains(string(payload), `"rev":8`) {
		t.Errorf("gateway frame = %s, want rev 8", payload)
	}
	for name, c := range map[string]*wsTestClient{"first": b1, "second": b2} {
		if payload, err := readFrameWithin(c, 2*time.Second); err != nil {
			t.Fatalf("%s browser frame after fan-out: %v", name, err)
		} else if !strings.Contains(string(payload), `"rev":8`) {
			t.Errorf("%s browser frame = %s, want rev 8", name, payload)
		}
	}
	if s.gwConn == nil {
		t.Fatal("gateway slot empty after a second browser connected")
	}
}

// TestBrowserRoleStaysInBrowserSlot checks that only role=hub is routed to the
// gateway slot: a plain browser dial (and any other role) goes to the browser
// list instead.
func TestBrowserRoleStaysInBrowserSlot(t *testing.T) {
	s := NewServer(t.TempDir(), ServerOptions{})
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	b := dialWSPath(t, srv, "/ws?role=browser")
	waitUntil(t, "browser slot", func() bool { return s.HasClient() })
	if s.gwConn != nil {
		t.Fatal("a non-hub role must not use the gateway slot")
	}
	_ = b
}
