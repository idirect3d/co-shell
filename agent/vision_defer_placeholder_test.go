// Author: L.Shuang
// Created: 2026-08-14
// Last Modified: 2026-08-14
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// TestShouldDeferVisionToolResult verifies that the vision tool placeholder
// result is deferred ONLY when a FEATURE-343 minimal recognition round is
// guaranteed to follow (minimal mode + pending intent + loaded images). In
// full mode — or whenever the recognition round cannot trigger — the
// placeholder must be written so the assistant tool_calls message is never
// left without a tool response.
func TestShouldDeferVisionToolResult(t *testing.T) {
	newAgent := func(mode string, intent string, images []string) *Agent {
		cfg := config.DefaultConfig()
		cfg.LLM.VisionContextMode = mode
		a := &Agent{cfg: cfg}
		a.visionPendingIntent = intent
		a.imagePaths = images
		return a
	}

	tests := []struct {
		name     string
		toolName string
		agent    *Agent
		want     bool
	}{
		{"visual_analysis minimal", "visual_analysis", newAgent("minimal", "OCR", []string{"a.png"}), true},
		{"browser_screenshot minimal", "browser_screenshot", newAgent("minimal", "read page", []string{"s.jpg"}), true},
		{"visual_analysis full mode keeps placeholder", "visual_analysis", newAgent("full", "OCR", []string{"a.png"}), false},
		{"minimal but no pending intent", "visual_analysis", newAgent("minimal", "", []string{"a.png"}), false},
		{"minimal but no images loaded", "visual_analysis", newAgent("minimal", "OCR", nil), false},
		{"non-vision tool", "execute_command", newAgent("minimal", "OCR", []string{"a.png"}), false},
		{"nil cfg", "visual_analysis", &Agent{visionPendingIntent: "OCR", imagePaths: []string{"a.png"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.agent.shouldDeferVisionToolResult(tt.toolName); got != tt.want {
				t.Errorf("shouldDeferVisionToolResult(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

// TestAbortVisionRecognitionRound verifies that aborting an in-flight minimal
// recognition round still backfills exactly one tool message for the pending
// vision tool call (keeping the OpenAI tool_call/tool protocol valid) and
// clears the recognition state. A second abort must be a no-op.
func TestAbortVisionRecognitionRound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LLM.VisionContextMode = "minimal"
	a := &Agent{
		cfg:             cfg,
		toolCallModeMgr: NewToolCallModeManager(),
	}
	a.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{{ID: "call_v1", Name: "visual_analysis"}}},
	}
	a.visionRecognitionActive = true
	a.visionPendingIntent = "OCR"
	a.lastVisionToolCallID = "call_v1"
	a.lastVisionToolCallName = "visual_analysis"

	a.abortVisionRecognitionRound()

	if len(a.messages) != 3 {
		t.Fatalf("expected 3 messages after abort, got %d", len(a.messages))
	}
	last := a.messages[2]
	if last.Role != "tool" || last.ToolCallID != "call_v1" {
		t.Errorf("abort backfill message = role %q id %q, want tool/call_v1", last.Role, last.ToolCallID)
	}
	// injectTimeAndMessageNoToLast may move the content into ContentParts and
	// clear Content — accept either representation, but the cancelled marker
	// must be present.
	text := last.Content
	for _, p := range last.ContentParts {
		if p.Type == llm.ContentPartText {
			text += p.Text
		}
	}
	if text == "" {
		t.Error("abort backfill content is empty, want cancelled marker")
	}
	if a.visionRecognitionActive || a.visionPendingIntent != "" {
		t.Error("recognition state not cleared after abort")
	}

	// Second abort: no-op, no extra message.
	a.abortVisionRecognitionRound()
	if len(a.messages) != 3 {
		t.Fatalf("second abort appended a message: len=%d", len(a.messages))
	}
}

// TestAbortVisionRecognitionRoundNoopWhenInactive verifies the abort helper is
// a no-op when no recognition round is in flight (e.g. full vision-context
// mode, where visionRecognitionActive is never set).
func TestAbortVisionRecognitionRoundNoopWhenInactive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LLM.VisionContextMode = "full"
	a := &Agent{cfg: cfg, toolCallModeMgr: NewToolCallModeManager()}
	a.messages = []llm.Message{{Role: "system", Content: "sys"}}

	a.abortVisionRecognitionRound()

	if len(a.messages) != 1 {
		t.Fatalf("abort with no active recognition round changed messages: len=%d", len(a.messages))
	}
}
