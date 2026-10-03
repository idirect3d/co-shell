// Package agent - FIX-599: resume a stream that a transient failure cut in half.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// streamAttemptTracker records what a single streaming attempt already sent to
// the UI. When a transient failure (network / 5xx / 429) interrupts the stream,
// FIX-598 used to stop the turn as soon as anything had been streamed: retrying
// with the unchanged context makes the model start its answer over, which the
// user would see as duplicated text.
//
// With the tracker the retry can resume instead: the part the user already saw
// is kept as an assistant message and a "continue from where you stopped" user
// message is appended, so the model appends to the existing answer.
//
// Only text is tracked. A half-received tool call cannot be continued safely,
// and streamLLMResponse reports that case through hasToolAttempt — the caller
// then falls back to stopping the turn.
type streamAttemptTracker struct {
	content   strings.Builder
	reasoning strings.Builder
}

// wrap returns cb with the text accumulation in front of it. It is nil-safe so
// a text-only caller can pass no callback at all.
func (t *streamAttemptTracker) wrap(cb StreamCallback) StreamCallback {
	if cb == nil {
		return nil
	}
	return func(ev StreamEvent) {
		switch ev.Type {
		case EventContentChunk, EventContent:
			t.content.WriteString(ev.Text)
		case EventThinkingChunk, EventThinking:
			t.reasoning.WriteString(ev.Text)
		}
		cb(ev)
	}
}

// contentText returns the assistant text already streamed to the user; it is
// empty when the failure happened before any content arrived.
func (t *streamAttemptTracker) contentText() string { return t.content.String() }

// reasoningText returns the thinking text already streamed to the user.
func (t *streamAttemptTracker) reasoningText() string { return t.reasoning.String() }

// resumable reports whether the failure happened after the user had already
// seen part of the answer AND no tool call was half-received. Only then may the
// retry continue the answer instead of stopping the turn.
func (t *streamAttemptTracker) resumable(hasToolAttempt bool) bool {
	return strings.TrimSpace(t.contentText()) != "" && !hasToolAttempt
}

// appendPartialAssistantAndContinue keeps the partial answer the user has
// already seen and asks the model to continue it (FIX-599). The retry therefore
// appends to the visible text instead of repeating it from the beginning.
//
// The partial text is stored as a normal assistant message because that is
// exactly what the model had produced; the follow-up user message mirrors the
// existing "continue" prompt used elsewhere in the loop.
func (a *Agent) appendPartialAssistantAndContinue(content, reasoning string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, llm.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
	})
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: i18n.T(i18n.KeyLLMErrResumePrompt),
	})
}
