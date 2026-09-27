// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// FEATURE-566: the Web UI is the surface that can start a turn for a sub-agent
// that finished on its own. These tests pin the hand-off: the notice becomes
// this session's next user message, and a notice for another session is refused
// so the agent can fall back to the dynamic-event path.

package web

import (
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// subAgentNoticeText stands in for a rendered <sub_agent_done> notice.
const subAgentNoticeText = "sub_agent_done A1 run 2 status completed"

// TestWebSessionStartSubAgentCompletionTurn covers the WebSession hand-off.
func TestWebSessionStartSubAgentCompletionTurn(t *testing.T) {
	ag := &agent.Agent{}
	ag.SetCurrentSessionID("sess-1")
	s := &WebSession{
		ag:      ag,
		inputCh: make(chan clientMessage, 1),
		closed:  make(chan struct{}),
	}

	if !s.startSubAgentCompletionTurn("sess-1", subAgentNoticeText) {
		t.Fatal("a notice for the current session must start a turn")
	}
	select {
	case msg := <-s.inputCh:
		if msg.Type != "input" || msg.Text != subAgentNoticeText {
			t.Errorf("queued message = %+v, want the notice as user input", msg)
		}
	default:
		t.Error("no message was queued")
	}

	// A notice for another session must be refused: the turn would otherwise run
	// in the current session with a foreign context.
	if s.startSubAgentCompletionTurn("sess-other", subAgentNoticeText) {
		t.Error("a notice for another session must be refused")
	}
}

// TestWebIOStartSubAgentCompletionTurnUnwired covers the fallback: a WebIO with
// no session wired reports false, so the agent buffers the notice instead.
func TestWebIOStartSubAgentCompletionTurnUnwired(t *testing.T) {
	w := &WebIO{}
	if w.StartSubAgentCompletionTurn("sess-1", subAgentNoticeText) {
		t.Error("an unwired WebIO must report false")
	}
}
