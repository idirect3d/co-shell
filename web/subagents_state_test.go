// Package web - FEATURE-564 regression tests: the connect-time state snapshot
// ("kind":"state") restores the task plan, and it must restore the sub-agent
// status list below the plan as well. The list is otherwise pushed on agent
// events only, so a page refresh or a reconnect used to show the plan panel
// with no sub-agents until the next sub-agent activity.
//
// The snapshot travels as a "sub_agents" event frame right after the state
// message, reusing the very event the live path pushes (agent.SubAgentsEvent),
// so the frontend needs one rendering branch only.
//
// Author: L.Shuang
// Created: 2026-09-27
// Last Modified: 2026-09-27
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"testing"
)

// TestSendStateReplaysSubAgents covers the refresh/reconnect path: with a
// provider installed, sendState enqueues the state message and then one
// sub_agents event carrying the session's rows.
func TestSendStateReplaysSubAgents(t *testing.T) {
	const (
		sessionID = "sess-564"
		rows      = `[{"id":"ls-564b","instruction":"ls -la","status":"completed","run":1,"done":1,"total":1,"has_plan":false}]`
	)
	s := &Server{}
	s.SetSubAgentsProvider(func() (string, string) { return sessionID, rows })
	c := newStateTestConn()
	s.sendState(c, "")

	if got := string(drainStateMessage(t, c)["kind"]); got != `"state"` {
		t.Fatalf("first frame kind = %s, want \"state\"", got)
	}

	var raw []byte
	select {
	case raw = <-c.outCh:
	default:
		t.Fatal("sendState did not replay the sub-agent list")
	}
	var msg struct {
		Kind  string `json:"kind"`
		Event struct {
			Type string            `json:"type"`
			Meta map[string]string `json:"meta"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal sub-agent frame: %v", err)
	}
	if msg.Kind != "event" || msg.Event.Type != "sub_agents" {
		t.Fatalf("frame = kind %q type %q, want event/sub_agents", msg.Kind, msg.Event.Type)
	}
	if got := msg.Event.Meta["sub_agents"]; got != rows {
		t.Fatalf("meta.sub_agents = %q, want %q", got, rows)
	}
	if got := msg.Event.Meta["session_id"]; got != sessionID {
		t.Fatalf("meta.session_id = %q, want %q", got, sessionID)
	}
}

// TestSendStateWithoutSubAgentsProvider guards the no-provider case (CLI runs
// and any wiring mistake): only the state message is enqueued.
func TestSendStateWithoutSubAgentsProvider(t *testing.T) {
	s := &Server{}
	c := newStateTestConn()
	s.sendState(c, "")
	_ = drainStateMessage(t, c)

	select {
	case raw := <-c.outCh:
		t.Fatalf("unexpected frame without a provider: %s", raw)
	default:
	}
}
