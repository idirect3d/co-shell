// Package agent - FIX-599 tests: a stream cut by a transient failure resumes
// from what the user already saw instead of repeating it.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// TestStreamAttemptTrackerCollectsStreamedText: only content/thinking text is
// accumulated, and every event still reaches the real callback.
func TestStreamAttemptTrackerCollectsStreamedText(t *testing.T) {
	tr := &streamAttemptTracker{}
	seen := 0
	wrapped := tr.wrap(func(ev StreamEvent) { seen++ })
	wrapped(NewStreamEvent(EventContentChunk, ChannelSystem, LevelInfo, "Hello "))
	wrapped(NewStreamEvent(EventThinkingChunk, ChannelSystem, LevelInfo, "think"))
	wrapped(NewStreamEvent(EventContentChunk, ChannelSystem, LevelInfo, "world"))
	wrapped(NewStreamEvent(EventToolCall, ChannelSystem, LevelInfo, `{"tool":"x"}`))
	if got, want := tr.contentText(), "Hello world"; got != want {
		t.Errorf("contentText = %q, want %q", got, want)
	}
	if got, want := tr.reasoningText(), "think"; got != want {
		t.Errorf("reasoningText = %q, want %q", got, want)
	}
	if seen != 4 {
		t.Errorf("wrapped callback saw %d events, want 4 — events must not be swallowed", seen)
	}
}

// TestStreamAttemptResumable is the FIX-599 decision: resume only when real
// text was already shown and no tool call was half-received.
func TestStreamAttemptResumable(t *testing.T) {
	cases := []struct {
		name     string
		streamed string
		tool     bool
		want     bool
	}{
		{"nothing streamed", "", false, false},
		{"whitespace only", "   \n", false, false},
		{"partial tool call", "some text", true, false},
		{"plain partial answer", "some text", false, true},
	}
	for _, c := range cases {
		tr := &streamAttemptTracker{}
		tr.content.WriteString(c.streamed)
		if got := tr.resumable(c.tool); got != c.want {
			t.Errorf("%s: resumable = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAppendPartialAssistantAndContinue: the visible partial answer is kept as
// an assistant message and followed by a continue prompt, so the retry appends
// instead of repeating.
func TestAppendPartialAssistantAndContinue(t *testing.T) {
	a := &Agent{}
	a.messages = []llm.Message{{Role: "system", Content: "sys"}}
	a.appendPartialAssistantAndContinue("half an answer", "why")
	if len(a.messages) != 3 {
		t.Fatalf("messages = %d, want 3 (system + partial assistant + continue prompt)", len(a.messages))
	}
	if m := a.messages[1]; m.Role != "assistant" || m.Content != "half an answer" || m.ReasoningContent != "why" {
		t.Errorf("kept partial message = %+v, want the streamed text and reasoning", m)
	}
	if m := a.messages[2]; m.Role != "user" || strings.TrimSpace(m.Content) == "" {
		t.Errorf("continue prompt = %+v, want a non-empty user message", m)
	}
	if a.messages[2].Content == a.messages[1].Content {
		t.Error("the continue prompt must not just repeat the partial answer")
	}
}

// TestWrapNilCallback: a caller with no callback still gets a usable tracker.
func TestWrapNilCallback(t *testing.T) {
	tr := &streamAttemptTracker{}
	if tr.wrap(nil) != nil {
		t.Fatal("wrap(nil) must stay nil")
	}
	if tr.resumable(false) {
		t.Fatal("an unused tracker must not report itself resumable")
	}
}
