// Author: L.Shuang
// Created: 2026-08-03
// Last Modified: 2026-08-03
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

// TestBuildContextMessagesVisionMinimal verifies FEATURE-319 minimal mode:
// when VisionContextMode == "minimal" and images are pending with an intent,
// buildContextMessages collapses the history to [system, user(intent)] and
// the user message contains only the intent text + media parts (no history/
// assistant/tool messages).
func TestBuildContextMessagesVisionMinimal(t *testing.T) {
	a := &Agent{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				VisionContextMode: "minimal",
				ContextLimit:      -1,
				ContextPolicy:     "smart",
			},
		},
	}
	a.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1", ToolCalls: []llm.ToolCall{{ID: "tc1", Name: "read_file"}}},
		{Role: "tool", Content: "out", ToolCallID: "tc1"},
		{Role: "user", ContentParts: []llm.ContentPart{{Type: llm.ContentPartText, Text: "last"}}},
	}
	a.visionPendingIntent = "recognize the invoice amount"
	// Use a non-existent path: buildContextMessages logs a warning and skips
	// encoding, but the minimal-collapse branch still runs and produces the
	// clean message structure we assert on.
	a.imagePaths = []string{"/nonexistent/fake.png"}

	msgs := a.buildContextMessages()

	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (system + user)", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role = %q, want system", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Fatalf("msgs[1].Role = %q, want user", msgs[1].Role)
	}
	// The minimal user message must contain the clean intent text.
	foundIntent := false
	for _, cp := range msgs[1].ContentParts {
		if cp.Type == llm.ContentPartText && cp.Text == a.visionPendingIntent {
			foundIntent = true
			break
		}
	}
	if !foundIntent {
		t.Errorf("minimal user message missing intent text %q; parts=%v", a.visionPendingIntent, msgs[1].ContentParts)
	}
	// No history/tool/assistant leakage.
	for _, m := range msgs {
		if m.Role != "system" && m.Role != "user" {
			t.Errorf("unexpected role %q in collapsed messages", m.Role)
		}
	}
}

// TestBuildContextMessagesVisionMinimalNoIntent verifies that minimal mode
// WITHOUT a pending intent falls back to the existing behavior (no collapse),
// so the image-bearing last message is never lost.
func TestBuildContextMessagesVisionMinimalNoIntent(t *testing.T) {
	a := &Agent{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				VisionContextMode: "minimal",
				ContextLimit:      -1,
				ContextPolicy:     "smart",
			},
		},
	}
	a.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "user", Content: "last"},
	}
	a.visionPendingIntent = ""
	a.imagePaths = []string{"/nonexistent/fake.png"}

	msgs := a.buildContextMessages()

	// Existing behavior: no collapse — both user messages retained.
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3 (no collapse without intent)", len(msgs))
	}
}

// TestBuildContextMessagesVisionFull verifies full mode keeps the existing
// behavior: the full message history is sent (no collapse).
func TestBuildContextMessagesVisionFull(t *testing.T) {
	a := &Agent{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				VisionContextMode: "full",
				ContextLimit:      -1,
				ContextPolicy:     "smart",
			},
		},
	}
	a.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "user", Content: "last"},
	}
	a.visionPendingIntent = "some intent"
	a.imagePaths = []string{"/nonexistent/fake.png"}

	msgs := a.buildContextMessages()

	if len(msgs) != 3 {
		t.Fatalf("got %d messages in full mode, want 3 (no collapse)", len(msgs))
	}
}

// TestBuildContextMessagesVisionMinimalNoImages verifies minimal mode with no
// pending images does NOT trigger the collapse (normal text turns unaffected).
func TestBuildContextMessagesVisionMinimalNoImages(t *testing.T) {
	a := &Agent{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				VisionContextMode: "minimal",
				ContextLimit:      -1,
				ContextPolicy:     "smart",
			},
		},
	}
	a.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
	}
	a.visionPendingIntent = "some intent"
	a.imagePaths = nil

	msgs := a.buildContextMessages()

	if len(msgs) != 2 {
		t.Fatalf("got %d messages with no images, want 2 (no collapse)", len(msgs))
	}
}
