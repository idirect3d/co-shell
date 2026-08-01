// Package agent - event constant table tests (FEATURE-301).
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import "testing"

// TestEventConstantsVerbatim verifies each output event constant equals its
// original magic string value (UC-0003). This guarantees zero behavior change
// for consumers that match on raw string values.
func TestEventConstantsVerbatim(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"EventContentChunk", EventContentChunk, "content_chunk"},
		{"EventThinkingChunk", EventThinkingChunk, "thinking_chunk"},
		{"EventContent", EventContent, "content"},
		{"EventThinking", EventThinking, "thinking"},
		{"EventCommand", EventCommand, "command"},
		{"EventOutput", EventOutput, "output"},
		{"EventToolCall", EventToolCall, "tool_call"},
		{"EventTokenIter", EventTokenIter, "token_iter"},
		{"EventTokenTask", EventTokenTask, "token_task"},
		{"EventInfo", EventInfo, "info"},
		{"EventWarning", EventWarning, "warning"},
		{"EventError", EventError, "error"},
		{"EventDone", EventDone, "done"},
	}
	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
		}
	}
}

// TestEventConstantsUnique verifies all output event constants are distinct
// and non-empty (guards against accidental duplicates in the const block).
func TestEventConstantsUnique(t *testing.T) {
	values := []string{
		EventContentChunk, EventThinkingChunk, EventContent, EventThinking,
		EventCommand, EventOutput, EventToolCall, EventTokenIter, EventTokenTask,
		EventInfo, EventWarning, EventError, EventDone,
	}
	seen := make(map[string]bool)
	for _, v := range values {
		if v == "" {
			t.Error("event constant must not be empty")
		}
		if seen[v] {
			t.Errorf("duplicate event constant value %q", v)
		}
		seen[v] = true
	}
}

// TestInputKindsNonEmptyUnique verifies all input kind constants are non-empty
// and mutually distinct (UC-0004).
func TestInputKindsNonEmptyUnique(t *testing.T) {
	values := []string{
		InputLine, InputKey, InputArrowUp, InputArrowDn, InputArrowLt, InputArrowRt,
		InputEsc, InputCtrlC, InputTab, InputBackspace, InputEnter, InputEOF,
	}
	seen := make(map[string]bool)
	for _, v := range values {
		if v == "" {
			t.Error("input kind constant must not be empty")
		}
		if seen[v] {
			t.Errorf("duplicate input kind constant value %q", v)
		}
		seen[v] = true
	}
}
