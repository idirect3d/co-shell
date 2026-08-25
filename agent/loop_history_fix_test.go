// Author: L.Shuang
// Created: 2026-08-26
// Last Modified: 2026-08-26
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
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// TestApplyHistoryFixes_MessageIndexAndReplace verifies UC-0007: a fix located
// by message index replaces the search text in the target assistant message.
func TestApplyHistoryFixes_MessageIndexAndReplace(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "task"},
			{Role: "assistant", Content: "让我看看这个文件的内容"},
			{Role: "user", Content: "continue"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 2, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 1 {
		t.Fatalf("expected 1 fix applied, got %d", got)
	}
	if !strings.Contains(a.messages[2].Content, "请直接读取") {
		t.Errorf("message[2] not replaced: %q", a.messages[2].Content)
	}
	if strings.Contains(a.messages[2].Content, "让我看看") {
		t.Errorf("message[2] still contains search text: %q", a.messages[2].Content)
	}
}

// TestApplyHistoryFixes_OnlyFirstOccurrence verifies UC-0008: only the first
// occurrence of search is replaced, later occurrences are preserved.
func TestApplyHistoryFixes_OnlyFirstOccurrence(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "让我看看，让我看看这个文件"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 1 {
		t.Fatalf("expected 1 fix applied, got %d", got)
	}
	got := a.messages[1].Content
	if !strings.Contains(got, "请直接读取，让我看看这个文件") {
		t.Errorf("only first occurrence should be replaced, got: %q", got)
	}
}

// TestApplyHistoryFixes_SkipNonAssistant verifies UC-0009: fixes targeting
// non-assistant messages are skipped.
func TestApplyHistoryFixes_SkipNonAssistant(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "让我看看这个任务"},
			{Role: "assistant", Content: "好的"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (non-assistant target), got %d", got)
	}
	if !strings.Contains(a.messages[1].Content, "让我看看") {
		t.Errorf("user message should not be modified: %q", a.messages[1].Content)
	}
}

// TestApplyHistoryFixes_AdjacentFallback verifies UC-0010: when the search text
// is not found in the target message, the adjacent messages (index±1) are tried.
func TestApplyHistoryFixes_AdjacentFallback(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "让我看看这个文件"},
			{Role: "assistant", Content: "继续处理"},
		},
		loopHistoryFixes: []HistoryFix{
			// Target index 2 (out of range) but search is in message[1] (adjacent).
			{MessageIndex: 2, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 1 {
		t.Fatalf("expected 1 fix applied via adjacent fallback, got %d", got)
	}
	if !strings.Contains(a.messages[1].Content, "请直接读取") {
		t.Errorf("adjacent message[1] not replaced: %q", a.messages[1].Content)
	}
}

// TestApplyHistoryFixes_NoMatchSkipped verifies UC-0011: when neither the target
// nor adjacent messages contain the search text, the fix is skipped.
func TestApplyHistoryFixes_NoMatchSkipped(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "处理文件"},
			{Role: "assistant", Content: "继续"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "不存在的文本", Replace: "替换"},
		},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (no match), got %d", got)
	}
}

// TestApplyHistoryFixes_OnlyRecentThree verifies UC-0012: fixes targeting
// assistant messages beyond the most recent 3 are skipped. The adjacent
// fallback must not reach an in-scope message either, so the out-of-scope
// target's neighbors are made to not contain the search text.
func TestApplyHistoryFixes_OnlyRecentThree(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "第一条让我看看"}, // oldest assistant (index 1, out of last 3)
			{Role: "assistant", Content: "第二条处理文件"}, // index 2 (in scope, no search text)
			{Role: "assistant", Content: "第三条处理文件"}, // index 3 (in scope, no search text)
			{Role: "assistant", Content: "第四条处理文件"}, // index 4 (most recent, no search text)
		},
		loopHistoryFixes: []HistoryFix{
			// index 1 is the 4th most recent assistant → out of the last 3.
			// Its adjacent in-scope messages (2,3,4) do not contain the search
			// text, so the fix must be skipped.
			{MessageIndex: 1, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (out of recent-3 scope), got %d", got)
	}
	if !strings.Contains(a.messages[1].Content, "让我看看") {
		t.Errorf("out-of-scope message should not be modified: %q", a.messages[1].Content)
	}
}

// TestApplyHistoryFixes_EmptySearchIgnored verifies UC-0013: fixes with an empty
// search are ignored.
func TestApplyHistoryFixes_EmptySearchIgnored(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "让我看看"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "   ", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (empty search), got %d", got)
	}
}

// TestApplyHistoryFixes_SearchEqualsReplaceIgnored verifies UC-0014: fixes where
// search equals replace are ignored.
func TestApplyHistoryFixes_SearchEqualsReplaceIgnored(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "让我看看"},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "让我看看", Replace: "让我看看"},
		},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (search==replace), got %d", got)
	}
}

// TestApplyHistoryFixes_ContentParts verifies replacement works when the message
// text lives in ContentParts rather than the plain Content field.
func TestApplyHistoryFixes_ContentParts(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "让我看看这个文件"},
			}},
		},
		loopHistoryFixes: []HistoryFix{
			{MessageIndex: 1, Search: "让我看看", Replace: "请直接读取"},
		},
	}
	if got := a.applyHistoryFixes(); got != 1 {
		t.Fatalf("expected 1 fix applied, got %d", got)
	}
	if !strings.Contains(a.messages[1].ContentParts[0].Text, "请直接读取") {
		t.Errorf("ContentParts text not replaced: %q", a.messages[1].ContentParts[0].Text)
	}
}

// TestApplyHistoryFixes_EmptyList verifies no-op when there are no fixes.
func TestApplyHistoryFixes_EmptyList(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{{Role: "system", Content: "system"}},
	}
	if got := a.applyHistoryFixes(); got != 0 {
		t.Fatalf("expected 0 fixes applied (empty list), got %d", got)
	}
}

// TestGetRecentAssistantHistory verifies the judge prompt history is formatted
// as "[index] content" lines using the REAL index in a.messages, so the judge
// can return it as history_fixes.message_index.
func TestGetRecentAssistantHistory(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "task"},
			{Role: "assistant", Content: "让我看看这个文件"}, // index 2
			{Role: "assistant", Content: "让我执行这个命令"}, // index 3
			{Role: "assistant", Content: "让我看看这个文件"}, // index 4 (most recent)
		},
	}
	got := a.getRecentAssistantHistory()
	// Most recent 3 assistant messages: indices 2,3,4 (chronological order).
	if !strings.Contains(got, "[2] 让我看看这个文件") {
		t.Errorf("history missing [2]: %q", got)
	}
	if !strings.Contains(got, "[3] 让我执行这个命令") {
		t.Errorf("history missing [3]: %q", got)
	}
	if !strings.Contains(got, "[4] 让我看看这个文件") {
		t.Errorf("history missing [4]: %q", got)
	}
}

// TestGetRecentAssistantHistory_SkipsToolCalls verifies assistant messages with
// tool_calls are excluded from the judge history (they carry no loop wording).
func TestGetRecentAssistantHistory_SkipsToolCalls(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{{Name: "read_file"}}}, // index 1, tool call
			{Role: "assistant", Content: "让我看看这个文件"}, // index 2
		},
	}
	got := a.getRecentAssistantHistory()
	if strings.Contains(got, "[1]") {
		t.Errorf("tool-call assistant message should be excluded: %q", got)
	}
	if !strings.Contains(got, "[2] 让我看看这个文件") {
		t.Errorf("history missing [2]: %q", got)
	}
}

// TestBuildLoopJudgeUserPrompt_IncludesHistory verifies the built judge prompt
// contains the {HISTORY} section with indexed assistant messages.
func TestBuildLoopJudgeUserPrompt_IncludesHistory(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "task"},
			{Role: "assistant", Content: "让我看看这个文件"}, // index 2
		},
	}
	prompt := a.buildLoopJudgeUserPrompt("plan", "suspect")
	if !strings.Contains(prompt, "[2] 让我看看这个文件") {
		t.Errorf("judge prompt missing indexed history: %q", prompt)
	}
	if strings.Contains(prompt, "{HISTORY}") {
		t.Errorf("judge prompt left {HISTORY} placeholder unfilled: %q", prompt)
	}
}
