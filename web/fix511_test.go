// Package web - FIX-511 regression tests: the connect-time state snapshot
// ("kind":"state") must carry the agent's running state, because the frontend
// derives its running indicators (logo breathing, red ⏸ button, session-title
// highlight) from live turn_start/await_input events only — and those are never
// persisted, so a page refresh or a manual reconnect used to leave the UI idle
// while the agent kept working.
//
// The hub polls GET /api/status for the same value; both read the busyFn
// installed via SetBusyProvider.
//
// Author: L.Shuang
// Created: 2026-09-12
// Last Modified: 2026-09-12
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"testing"
)

// newStateTestConn returns a wsConn whose outbound queue can be drained
// directly by a test: WriteMessage only enqueues, so no socket is needed.
func newStateTestConn() *wsConn {
	return &wsConn{outCh: make(chan []byte, 4), done: make(chan struct{})}
}

// drainStateMessage returns the single message sendState enqueued, decoded into
// its raw JSON fields (so a test can tell "absent" from "false").
func drainStateMessage(t *testing.T, c *wsConn) map[string]json.RawMessage {
	t.Helper()
	var raw []byte
	select {
	case raw = <-c.outCh:
	default:
		t.Fatal("sendState did not enqueue a message")
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal state message: %v", err)
	}
	return out
}

// TestSendStateCarriesBusy covers UC-0010: busyFn returning true, false, and no
// provider installed at all (nil).
func TestSendStateCarriesBusy(t *testing.T) {
	cases := []struct {
		name       string
		busyFn     func() bool
		wantInJSON bool
		want       bool
	}{
		{name: "running", busyFn: func() bool { return true }, wantInJSON: true, want: true},
		{name: "idle", busyFn: func() bool { return false }, wantInJSON: true, want: false},
		{name: "no provider", busyFn: nil, wantInJSON: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			if tc.busyFn != nil {
				s.SetBusyProvider(tc.busyFn)
			}
			c := newStateTestConn()
			s.sendState(c)
			msg := drainStateMessage(t, c)

			if got := string(msg["kind"]); got != `"state"` {
				t.Fatalf("kind = %s, want \"state\"", got)
			}
			raw, ok := msg["busy"]
			if ok != tc.wantInJSON {
				t.Fatalf("busy present = %v, want %v", ok, tc.wantInJSON)
			}
			var busy bool
			if err := json.Unmarshal(raw, &busy); err != nil {
				t.Fatalf("unmarshal busy: %v", err)
			}
			if busy != tc.want {
				t.Fatalf("busy = %v, want %v", busy, tc.want)
			}
		})
	}
}

// TestSendStateWithoutPlanOrBusy covers UC-0011: with neither provider
// installed the snapshot must still serialise, reporting a null plan and
// busy=false (the frontend reads "no provider" as idle, never as an error).
func TestSendStateWithoutPlanOrBusy(t *testing.T) {
	s := &Server{}
	c := newStateTestConn()
	s.sendState(c)
	msg := drainStateMessage(t, c)

	if got := string(msg["plan"]); got != "null" {
		t.Fatalf("plan = %s, want null", got)
	}
	if got := string(msg["busy"]); got != "false" {
		t.Fatalf("busy = %s, want false", got)
	}
}

// TestSendStatePreservesPlan guards the pre-existing plan passthrough: adding
// the busy field must not change how the task-plan snapshot is delivered.
func TestSendStatePreservesPlan(t *testing.T) {
	const plan = `{"title":"t","steps":[]}`
	s := &Server{}
	s.SetPlanProvider(func() string { return plan })
	s.SetBusyProvider(func() bool { return true })
	c := newStateTestConn()
	s.sendState(c)
	msg := drainStateMessage(t, c)

	var got map[string]any
	if err := json.Unmarshal(msg["plan"], &got); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if got["title"] != "t" {
		t.Fatalf("plan title = %v, want t", got["title"])
	}
	var busy bool
	if err := json.Unmarshal(msg["busy"], &busy); err != nil {
		t.Fatalf("unmarshal busy: %v", err)
	}
	if !busy {
		t.Fatal("busy = false, want true alongside a plan snapshot")
	}
}
