// Package web - FIX-506 regression tests: the retry-from "msg_index" attached
// to stream events must be the REAL message array index (not the turn counter),
// because the backend popTo() truncates by array index.
//
// Author: L.Shuang
// Created: 2026-09-11
// Last Modified: 2026-09-11
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"strconv"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/llm"
)

// TestMsgIndexForRetryIsArrayIndex verifies that the msg_index attached to
// stream events equals len(messages)-1 (the real array index), NOT the turn
// counter. Before FIX-506 the turn counter was used, so clicking the
// second-to-last user message truncated far more history than intended.
func TestMsgIndexForRetryIsArrayIndex(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)

	// Simulate a long conversation: system + 20 messages.
	msgs := []llm.Message{{Role: "system", Content: "sys"}}
	for i := 0; i < 10; i++ {
		msgs = append(msgs,
			llm.Message{Role: "user", Content: "u"},
			llm.Message{Role: "assistant", Content: "a"},
		)
	}
	ag.SetHistory(msgs)

	// The turn counter is deliberately much smaller than the array length.
	ws.msgIndex = 5

	r := &WebRenderer{s: ws}
	got := r.msgIndexForRetry()
	want := len(msgs) - 1 // 20
	if got != want {
		t.Fatalf("msgIndexForRetry() = %d, want %d (real array index, not turn counter %d)",
			got, want, ws.msgIndex)
	}
}

// TestRenderAttachesArrayIndex verifies Render() writes the array index into
// the event meta as "msg_index".
func TestRenderAttachesArrayIndex(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)
	ag.SetHistory([]llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	})
	ws.msgIndex = 2 // turn counter differs from array index (3)

	r := &WebRenderer{s: ws}
	r.Render(agent.NewStreamEvent("content", agent.ChannelLLM, agent.LevelInfo, "hi"))

	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event == nil {
		t.Fatalf("expected event message, got kind=%q", msg.Kind)
	}
	got := msg.Event.Meta["msg_index"]
	if got != strconv.Itoa(len(ag.Messages())-1) {
		t.Fatalf("msg_index = %q, want %q (array index)", got, strconv.Itoa(len(ag.Messages())-1))
	}
}

// TestMsgIndexForRetryEmptyHistory verifies the helper is safe when the agent
// has no messages (returns 0 instead of -1).
func TestMsgIndexForRetryEmptyHistory(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)
	ag.SetHistory(nil)

	r := &WebRenderer{s: ws}
	if got := r.msgIndexForRetry(); got != 0 {
		t.Fatalf("msgIndexForRetry() with empty history = %d, want 0", got)
	}
}
