// Package web - FIX-506 (续修) regression tests: the YOU block's retry-from
// index must be the index of THAT user message, which is only knowable on the
// first event of the turn (the backend appends the user message inside
// agent.RunStream, after the frontend has already rendered the YOU block).
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

// TestFirstEventOfTurnCarriesUserMessageIndex verifies the invariant the
// frontend relies on: on the FIRST event of a turn, msg_index equals the index
// of the user message that started the turn (no assistant/tool message has been
// appended yet). This is what makes the YOU block's backfilled index correct.
func TestFirstEventOfTurnCarriesUserMessageIndex(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)

	// Turn 1: system + user("hello 1") + assistant reply.
	ag.SetHistory([]llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello 1"},
		{Role: "assistant", Content: "hi 1"},
	})

	// Turn 2 begins: the agent appends the new user message. At this instant the
	// last message IS the user message, so len-1 is its real index.
	ag.SetHistory([]llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello 1"},
		{Role: "assistant", Content: "hi 1"},
		{Role: "user", Content: "hello 2"},
	})
	wantUserIdx := 3 // index of "hello 2"

	r := &WebRenderer{s: ws}
	r.Render(agent.NewStreamEvent("content", agent.ChannelLLM, agent.LevelInfo, "hi 2"))

	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event == nil {
		t.Fatalf("expected event message, got kind=%q", msg.Kind)
	}
	got := msg.Event.Meta["msg_index"]
	if got != strconv.Itoa(wantUserIdx) {
		t.Fatalf("first-event msg_index = %q, want %q (index of the user message that started the turn)",
			got, strconv.Itoa(wantUserIdx))
	}
}

// TestPopToKeepsUpToUserMessage verifies the end-to-end semantics the YOU block
// relies on: popTo(n) keeps [0..n], so popping to a user message's index keeps
// that message and drops everything after it.
func TestPopToKeepsUpToUserMessage(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)
	ag.SetHistory([]llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello 1"},
		{Role: "assistant", Content: "hi 1"},
		{Role: "user", Content: "hello 2"},
		{Role: "assistant", Content: "hi 2"},
		{Role: "user", Content: "hello 3"},
		{Role: "assistant", Content: "hi 3"},
	})

	// Pop to "hello 2" (index 3): keep [0..3] = sys, hello 1, hi 1, hello 2.
	ws.popTo("3")

	got := ag.Messages()
	if len(got) != 4 {
		t.Fatalf("after popTo(3): %d messages, want 4", len(got))
	}
	if got[3].Role != "user" || got[3].Content != "hello 2" {
		t.Fatalf("last kept message = %q/%q, want user/hello 2", got[3].Role, got[3].Content)
	}
}
