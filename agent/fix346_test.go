// Author: L.Shuang
// Created: 2026-08-10
// Last Modified: 2026-08-10
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// TestFeature346RecognitionRoundImageInjectionToolLast verifies that the minimal
// recognition round collapses to [system(Identity), user(intent+image)] even
// when the last history message is a tool message (OpenAI tool-call mode) —
// the injected image must NOT be lost (FEATURE-346 UC-0026).
func TestFeature346RecognitionRoundImageInjectionToolLast(t *testing.T) {
	cfg := minimalVisionCfg()
	a := visionAgent(cfg)
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
		{Role: "user", Content: "look at the page"},
		{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{{ID: "call_browser_1", Name: "browser_screenshot"}}},
		{Role: "tool", Content: "Screenshot saved to: ./download/screenshot/x.jpg", ToolCallID: "call_browser_1"},
	}
	a.visionPendingIntent = "analyze the page"
	imgPath := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(imgPath, []byte("fake-image-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.imagePaths = []string{imgPath}

	msgs := a.buildContextMessages()

	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (collapsed recognition round)", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role = %q, want system", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Fatalf("msgs[1].Role = %q, want user", msgs[1].Role)
	}
	foundIntent := false
	foundImage := false
	for _, cp := range msgs[1].ContentParts {
		if cp.Type == llm.ContentPartText && cp.Text == a.visionPendingIntent {
			foundIntent = true
		}
		if cp.Type == llm.ContentPartImageURL {
			foundImage = true
		}
	}
	if !foundIntent {
		t.Errorf("minimal user message missing intent text %q", a.visionPendingIntent)
	}
	if !foundImage {
		t.Error("minimal user message missing injected image part (OpenAI tool-last path)")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.visionRecognitionActive {
		t.Error("visionRecognitionActive not set for the recognition round")
	}
	if a.imagePaths != nil {
		t.Error("imagePaths not cleared after injection")
	}
}

// TestFeature346RecognitionRoundSkipsMissingImage verifies that a missing image
// file is skipped with a warning but the minimal collapse still happens and the
// intent text is preserved (FEATURE-346 UC-0026 missing-file branch).
func TestFeature346RecognitionRoundSkipsMissingImage(t *testing.T) {
	cfg := minimalVisionCfg()
	a := visionAgent(cfg)
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
		{Role: "user", Content: "look"},
		{Role: "tool", Content: "out", ToolCallID: "call_x"},
	}
	a.visionPendingIntent = "OCR"
	a.imagePaths = []string{"/nonexistent/fake.png"}

	msgs := a.buildContextMessages()

	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (collapse still happens)", len(msgs))
	}
	foundIntent := false
	for _, cp := range msgs[1].ContentParts {
		if cp.Type == llm.ContentPartText && cp.Text == "OCR" {
			foundIntent = true
		}
		if cp.Type == llm.ContentPartImageURL {
			t.Error("unexpected image part for a missing file")
		}
	}
	if !foundIntent {
		t.Error("intent text missing from collapsed user message")
	}
}

// TestFeature346RecordVisionToolCall verifies recordVisionToolCall stores the
// ToolCall ID and name for browser_screenshot and visual_analysis, and is a
// no-op for other tools (FEATURE-346 UC-0027).
func TestFeature346RecordVisionToolCall(t *testing.T) {
	cfg := minimalVisionCfg()
	a := visionAgent(cfg)

	a.recordVisionToolCall(llm.ToolCall{ID: "call_shot_1", Name: "browser_screenshot"})
	a.mu.Lock()
	id, name := a.lastVisionToolCallID, a.lastVisionToolCallName
	a.mu.Unlock()
	if id != "call_shot_1" || name != "browser_screenshot" {
		t.Fatalf("browser_screenshot not recorded: id=%q name=%q", id, name)
	}

	// Non-vision tool must not overwrite the vision record.
	a.recordVisionToolCall(llm.ToolCall{ID: "call_exec", Name: "execute_command"})
	a.mu.Lock()
	id, name = a.lastVisionToolCallID, a.lastVisionToolCallName
	a.mu.Unlock()
	if id != "call_shot_1" || name != "browser_screenshot" {
		t.Errorf("non-vision tool overwrote vision record: id=%q name=%q", id, name)
	}

	// visual_analysis is still recorded (regression for FEATURE-343).
	a.recordVisionToolCall(llm.ToolCall{ID: "call_va_1", Name: "visual_analysis"})
	a.mu.Lock()
	id, name = a.lastVisionToolCallID, a.lastVisionToolCallName
	a.mu.Unlock()
	if id != "call_va_1" || name != "visual_analysis" {
		t.Errorf("visual_analysis not recorded: id=%q name=%q", id, name)
	}
}

// TestFeature346XMLBackfillUsesRecordedToolName verifies that the XML-mode
// recognition backfill message embeds the recorded tool name
// (browser_screenshot) instead of a hard-coded visual_analysis
// (FEATURE-346 UC-0028).
func TestFeature346XMLBackfillUsesRecordedToolName(t *testing.T) {
	cfg := minimalVisionCfg()
	a := visionAgent(cfg)
	msg := a.buildXMLToolResultMessage("browser_screenshot", "", "recognition result", 5)
	got := ""
	if len(msg.ContentParts) > 0 {
		got = msg.ContentParts[0].Text
	}
	if !strings.Contains(got, "browser_screenshot") {
		t.Errorf("XML backfill message missing browser_screenshot tool name: %.200s", got)
	}
}

// TestFeature346VisionIdentityPromptIncludesDescription verifies that the
// minimal recognition round system prompt includes the agent identity
// description resolved through the same priority chain as the main prompt,
// even when the global AgentDescription is not explicitly set (FEATURE-346).
func TestFeature346VisionIdentityPromptIncludesDescription(t *testing.T) {
	cfg := minimalVisionCfg()
	desc := resolveAgentDescription(cfg)
	if strings.TrimSpace(desc) == "" {
		t.Fatal("resolveAgentDescription returned an empty description for the act mode")
	}
	prompt := buildVisionIdentityPrompt(cfg)
	if !strings.Contains(prompt, desc) {
		t.Errorf("vision identity prompt missing agent description %q; prompt=%.200s", desc, prompt)
	}
}

// TestFeature346I18nDefaultIntentKey verifies the new default-intent i18n key
// exists in both zh and en (FEATURE-346 UC-0029).
func TestFeature346I18nDefaultIntentKey(t *testing.T) {
	if !i18nKeyExists("browser_screenshot_vision_intent_default") {
		t.Error("browser_screenshot_vision_intent_default key missing in zh/en")
	}
}

