// Package web - FIX-619 runtime tests for the server-side heartbeat: a browser
// connection is pinged periodically and dropped once it stops answering, while
// traffic (a pong included) keeps it alive.
//
// Author: L.Shuang
// Created: 2026-10-07
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"testing"
	"time"
)

// TestFix619PingIsSent verifies the server pings a browser connection on the
// configured cadence, so the peer's stack answers with a pong.
func TestFix619PingIsSent(t *testing.T) {
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		stop := c.StartPing(20 * time.Millisecond)
		defer stop()
		c.SetReadTimeout(5 * time.Second)
		for {
			if _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	if err := client.conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		op, _, err := client.readFrame()
		if err != nil {
			t.Fatalf("read frame: %v", err)
		}
		if op == wsOpPing {
			return // heartbeat observed
		}
	}
}

// TestFix619ReadTimeoutDropsSilentConnection verifies a connection that stops
// talking is closed instead of lingering in conns forever.
func TestFix619ReadTimeoutDropsSilentConnection(t *testing.T) {
	errCh := make(chan error, 1)
	dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		c.SetReadTimeout(80 * time.Millisecond)
		_, err := c.ReadMessage()
		errCh <- err
	}))
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("ReadMessage returned nil error for a silent connection")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("silent connection was not dropped by the read timeout")
	}
}

// TestFix619ActivityRefreshesDeadline verifies data frames keep the connection
// alive well past the read timeout.
func TestFix619ActivityRefreshesDeadline(t *testing.T) {
	dropped := make(chan error, 1)
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		c.SetReadTimeout(150 * time.Millisecond)
		for {
			if _, err := c.ReadMessage(); err != nil {
				dropped <- err
				return
			}
		}
	}))
	// ~600ms of traffic in 40ms steps: more than three timeout windows.
	for i := 0; i < 15; i++ {
		if err := client.writeFrame(true, wsOpText, []byte("hi")); err != nil {
			t.Fatalf("write frame: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
	}
	select {
	case err := <-dropped:
		t.Fatalf("active connection was dropped: %v", err)
	default:
	}
}

// TestFix619PongRefreshesDeadline verifies pong frames count as activity: a
// browser that only answers protocol pings (an older page without the app-level
// heartbeat) must not be dropped while it is alive.
func TestFix619PongRefreshesDeadline(t *testing.T) {
	result := make(chan error, 1)
	client := dialWS(t, wsHandler(func(c *wsConn) {
		defer c.Close()
		c.SetReadTimeout(150 * time.Millisecond)
		if _, err := c.ReadMessage(); err != nil {
			result <- err
			return
		}
		result <- nil
	}))
	for i := 0; i < 8; i++ {
		if err := client.writeFrame(true, wsOpPong, nil); err != nil {
			t.Fatalf("write pong: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
	}
	if err := client.writeFrame(true, wsOpText, []byte("x")); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("pong did not refresh the read deadline: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("read did not complete")
	}
}
