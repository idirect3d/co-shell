// Author: L.Shuang
// Created: 2026-08-05
// Last Modified: 2026-08-05
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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

func fix329InitI18n(t *testing.T) {
	t.Helper()
	i18n.Init("")
}

// FIX-329 UC-0001: indented "  }" and top-level "}" must hash differently.
// The original false positive was caused by TrimSpace collapsing these two
// lines into the same "}" before hashing.
func TestFix329_IndentedVsPlainBrace_NoFalsePositive(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(2, 200)

	err := ld.AddChunk("  }\n}\n", time.Now())
	if err != nil {
		t.Fatalf("indented '  }' followed by '}' should NOT trigger (they hash differently), got: %v", err)
	}
}

// FIX-329 UC-0002: realistic code-block ending (statement + two differently
// indented closing braces) must not trigger.
func TestFix329_RealisticCodeBlockEnding_NoFalsePositive(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(2, 200)

	err := ld.AddChunk("    renumberStageRows(parent);\n  }\n}\n", time.Now())
	if err != nil {
		t.Fatalf("code block ending with differently-indented braces should NOT trigger, got: %v", err)
	}
}

// FIX-329 UC-0003: two identical top-level "}" lines are suppressed by the
// quantity-block limit (1 char × 2 = 2 ≤ 200).
func TestFix329_TwoIdenticalBraceLines_SuppressedByBlockLimit(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(2, 200)

	err := ld.AddChunk("}\n}\n", time.Now())
	if err != nil {
		t.Fatalf("two '}' lines (1×2=2 ≤ 200) should be suppressed by blockLimit, got: %v", err)
	}
}

// FIX-329 UC-0004: a long line repeated twice (120 chars × 2 = 240 > 200)
// still triggers, and the error carries LoopType "single_repeat".
func TestFix329_LongLineRepeat_TriggersWithSingleRepeatType(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(2, 200)

	longLine := strings.Repeat("x", 120)
	err := ld.AddChunk(longLine+"\n"+longLine+"\n", time.Now())
	if err == nil {
		t.Fatal("120-char line repeated twice (120×2=240 > 200) should trigger, but got nil")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	if ldErr.LoopType != "single_repeat" {
		t.Errorf("expected LoopType=single_repeat, got %q", ldErr.LoopType)
	}
}

// FIX-329 UC-0005: multi-line period (p=2) is NOT affected by the p=1
// quantity-block limit and still triggers; LoopType is "multi_line".
func TestFix329_MultiLinePeriod_NotAffectedByBlockLimit(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(2, 200)

	err := ld.AddChunk("lineA\nlineB\nlineA\nlineB\n", time.Now())
	if err == nil {
		t.Fatal("ABAB period-2 should trigger (p>=2 is not gated by blockLimit), but got nil")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	if ldErr.LoopType != "multi_line" {
		t.Errorf("expected LoopType=multi_line, got %q", ldErr.LoopType)
	}
}

// FIX-329 UC-0006: checkLoop must populate LoopType for both p=1 and p>=2.
func TestFix329_LoopTypePopulated(t *testing.T) {
	// single_repeat
	ld := NewLoopDetectorWithBlockLimit(2, 200)
	longLine := strings.Repeat("y", 120)
	err := ld.AddChunk(longLine+"\n"+longLine+"\n", time.Now())
	e1 := err.(*LoopDetectedError)
	if e1.LoopType != "single_repeat" {
		t.Errorf("p=1 should be single_repeat, got %q", e1.LoopType)
	}

	// multi_line
	ld2 := NewLoopDetectorWithBlockLimit(2, 200)
	err = ld2.AddChunk("lineA\nlineB\nlineA\nlineB\n", time.Now())
	e2 := err.(*LoopDetectedError)
	if e2.LoopType != "multi_line" {
		t.Errorf("p=2 should be multi_line, got %q", e2.LoopType)
	}
}

// FIX-329 UC-0007: getAllUserPrompts filters XML tool results, continue
// prompts, and loop feedback messages; keeps only genuine user input.
func TestFix329_GetAllUserPromptsFiltering(t *testing.T) {
	fix329InitI18n(t)

	xmlPrefix := i18n.T(i18n.KeyXMLToolResultTemplate)
	// Build an env block that marks a loop feedback message.
	loopEnv := setLoopFeedbackInText("<environment_details></environment_details>", true)

	a := &Agent{
		lastUserInput: "fallback",
		messages: []llm.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "请修改 app.js 的删除逻辑"},
			{Role: "user", ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: strings.ReplaceAll(xmlPrefix, "{TOOL_CALL}", "read_file") + "\n{TOOL_RESULT}\n"},
			}},
			{Role: "user", Content: i18n.T(i18n.KeyContinuePrompt)},
			{Role: "user", ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "再来一次"},
				{Type: llm.ContentPartText, Text: loopEnv},
			}},
			{Role: "user", Content: "最后修正"},
		},
	}

	out := a.getAllUserPrompts()
	if strings.Contains(out, "返回结果") || strings.Contains(out, "[read_file]") {
		t.Errorf("XML tool result should be filtered, got:\n%s", out)
	}
	if strings.Contains(out, "Decide immediately") || strings.Contains(out, "请立即决定") {
		t.Errorf("continue prompt should be filtered, got:\n%s", out)
	}
	if strings.Contains(out, "再来一次") {
		t.Errorf("loop feedback message should be filtered, got:\n%s", out)
	}
	for _, want := range []string{"1. 请修改 app.js 的删除逻辑", "2. 最后修正"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

// FIX-329 UC-0008: getAllUserPrompts strips the <environment_details> suffix.
func TestFix329_GetAllUserPromptsStripsEnv(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "user", ContentParts: []llm.ContentPart{
				{Type: llm.ContentPartText, Text: "指令内容"},
				{Type: llm.ContentPartText, Text: "<environment_details>...tokens...</environment_details>"},
			}},
		},
	}
	out := a.getAllUserPrompts()
	if strings.Contains(out, "<environment_details>") {
		t.Errorf("environment_details should be stripped, got:\n%s", out)
	}
	if !strings.Contains(out, "指令内容") {
		t.Errorf("instruction should be kept, got:\n%s", out)
	}
}

// FIX-329 UC-0009: getIterationTools lists tool names per iteration.
func TestFix329_GetIterationTools(t *testing.T) {
	a := &Agent{
		messages: []llm.Message{
			{Role: "assistant", ToolCalls: []llm.ToolCall{
				{Name: "read_file"},
				{Name: "search_files"},
			}},
			{Role: "assistant", ToolCalls: []llm.ToolCall{
				{Name: "replace_in_file"},
			}},
		},
	}
	out := a.getIterationTools()
	if !strings.Contains(out, "iter1: read_file, search_files") {
		t.Errorf("iter1 should list both tools, got:\n%s", out)
	}
	if !strings.Contains(out, "iter2: replace_in_file") {
		t.Errorf("iter2 should list replace_in_file, got:\n%s", out)
	}
}

// FIX-329 UC-0010: buildLoopJudgeUserPrompt fills {USER_PROMPTS} and
// {ITERATION_TOOLS} and leaves no placeholders.
func TestFix329_BuildLoopJudgeUserPromptFillsNewBlocks(t *testing.T) {
	fix329InitI18n(t)

	a := &Agent{
		lastUserInput: "原始指令",
		messages: []llm.Message{
			{Role: "user", Content: "原始指令"},
			{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "read_file"}}},
		},
	}

	prompt := a.buildLoopJudgeUserPrompt("无", "疑似内容")
	for _, ph := range []string{"{USER_PROMPTS}", "{ITERATION_TOOLS}", "{LAST_INPUT}", "{CONTEXT}", "{SUSPECT_CONTENT}"} {
		if strings.Contains(prompt, ph) {
			t.Errorf("placeholder %s not replaced in prompt:\n%s", ph, prompt)
		}
	}
	if !strings.Contains(prompt, "1. 原始指令") {
		t.Errorf("prompt should contain numbered user prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "iter1: read_file") {
		t.Errorf("prompt should contain iteration tools, got:\n%s", prompt)
	}
}

// FIX-329 UC-0011: loopTypeFromError maps detector error types.
func TestFix329_LoopTypeFromError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&LoopDetectedError{LoopType: "single_repeat", period: 1}, "single_repeat"},
		{&LoopDetectedError{LoopType: "multi_line", period: 2}, "multi_line"},
		// fallback inference from error text
		{&LoopDetectedError{period: 1}, "single_repeat"},
		{&LoopDetectedError{period: 2}, "multi_line"},
		{&ToolCallLoopDetectedError{}, "tool_call"},
	}
	for _, c := range cases {
		if got := loopTypeFromError(c.err); got != c.want {
			t.Errorf("loopTypeFromError(%T) = %q, want %q", c.err, got, c.want)
		}
	}
	// long output (generic error with matching prefix)
	longErr := &LoopDetectedError{}
	_ = longErr
	if got := loopTypeFromError(nil); got != "" {
		t.Errorf("nil error should return empty, got %q", got)
	}
}

// TestFix329_HandleLoopDetectionTypeBanner asserts that the non-judge path
// renders the typed loop banner via the agent's UserIO. We capture output
// with a memory UserIO.
func TestFix329_HandleLoopDetectionTypeBanner(t *testing.T) {
	fix329InitI18n(t)

	io := &memUserIO{}
	a := &Agent{
		cfg: &config.Config{LLM: config.LLMConfig{LoopJudgeEnabled: false}},
		io:  io,
	}
	a.handleLoopDetection("content", "", &LoopDetectedError{LoopType: "single_repeat", period: 1})

	joined := io.String()
	// The rendered banner is the formatter (TF) result, e.g.
	// "检测到疑似循环内容（类型: 单行重复）..." — assert on the resolved
	// label rather than the template/placeholder.
	if !strings.Contains(joined, "类型") {
		t.Errorf("typed banner should include the '类型' marker, got:\n%s", joined)
	}
	if !strings.Contains(joined, i18n.T(i18n.KeyLoopTypeSingleRepeat)) {
		t.Errorf("banner should include the type label, got:\n%s", joined)
	}
}

// memUserIO captures output for assertions.
type memUserIO struct {
	sb strings.Builder
}

func (m *memUserIO) Print(args ...interface{})            { m.sb.WriteString(fmt.Sprint(args...)) }
func (m *memUserIO) Printf(f string, args ...interface{}) { m.sb.WriteString(fmt.Sprintf(f, args...)) }
func (m *memUserIO) Println(args ...interface{})          { m.sb.WriteString(fmt.Sprintln(args...)) }
func (m *memUserIO) ErrPrintf(f string, args ...interface{}) {
	m.sb.WriteString(fmt.Sprintf(f, args...))
}
func (m *memUserIO) ReadLine() (string, error) { return "", nil }
func (m *memUserIO) ReadKey() (byte, error)    { return 0, nil }
func (m *memUserIO) IsReading() bool           { return false }
func (m *memUserIO) String() string            { return m.sb.String() }
