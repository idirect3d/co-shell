// Author: L.Shuang
// Created: 2026-08-02
// Last Modified: 2026-08-02
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
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// TestBuildToolSummaryFallback covers the generic fallback path (UC-0004).
func TestBuildToolSummaryFallback(t *testing.T) {
	// Generic tool with intent
	got := buildToolSummary("evaluate_expression", map[string]interface{}{
		"intent":     "compute the final total",
		"expression": "3 + 4 * 2",
	})
	if !strings.Contains(got.Text, "compute the final total") {
		t.Errorf("fallback summary missing intent: got %q", got.Text)
	}
	if strings.Contains(got.Text, "3 + 4 * 2") {
		t.Errorf("fallback summary leaked raw args expression: got %q", got.Text)
	}
	if got.ToolName != "evaluate_expression" || got.Intent != "compute the final total" {
		t.Errorf("structured fields wrong: %+v", got)
	}

	// Generic tool without intent
	got = buildToolSummary("evaluate_expression", map[string]interface{}{
		"expression": "1+1",
	})
	if got.Text != "evaluate_expression" {
		t.Errorf("fallback without intent should return tool name only, got %q", got.Text)
	}

	// Generic tool with empty args
	got = buildToolSummary("list_settings", map[string]interface{}{})
	if got.Text != "list_settings" {
		t.Errorf("fallback with empty args should return tool name only, got %q", got.Text)
	}
}

// TestBuildToolSummaryTextTools covers text-oriented tools (UC-0005).
func TestBuildToolSummaryTextTools(t *testing.T) {
	cases := []struct {
		name string
		tool string
		args map[string]interface{}
		want []string // substrings that must appear
	}{
		{
			name: "execute_command",
			tool: "execute_command",
			args: map[string]interface{}{
				"intent":  "list files",
				"command": "ls -la",
			},
			want: []string{"ls -la", "list files"},
		},
		{
			name: "read_file",
			tool: "read_file",
			args: map[string]interface{}{
				"intent":     "examine source",
				"path":       "src/main.go",
				"start_line": float64(1),
				"end_line":   float64(100),
			},
			want: []string{"src/main.go", "1", "100", "examine source"},
		},
		{
			name: "write_to_file",
			tool: "write_to_file",
			args: map[string]interface{}{
				"intent":  "save report",
				"mode":    "new",
				"path":    "report.md",
				"content": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.",
			},
			want: []string{"report.md", "new", "save report"},
		},
		{
			name: "search_files",
			tool: "search_files",
			args: map[string]interface{}{
				"intent": "find function",
				"path":   "src",
				"regex":  "func main",
			},
			want: []string{"src", "func main", "find function"},
		},
		{
			name: "list_files",
			tool: "list_files",
			args: map[string]interface{}{
				"intent":    "explore",
				"path":      "/tmp",
				"recursive": float64(1),
			},
			want: []string{"/tmp", "explore"},
		},
		{
			name: "list_code_definition_names",
			tool: "list_code_definition_names",
			args: map[string]interface{}{
				"intent": "understand API",
				"path":   "src/api",
			},
			want: []string{"src/api", "understand API"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildToolSummary(tc.tool, tc.args)
			for _, w := range tc.want {
				if !strings.Contains(got.Text, w) {
					t.Errorf("summary missing %q, got: %q", w, got.Text)
				}
			}
		})
	}
}

// TestBuildToolSummaryStructured verifies the structured fields (ToolName,
// Intent, Params) are populated for high-frequency tools (UC-0027).
func TestBuildToolSummaryStructured(t *testing.T) {
	got := buildToolSummary("execute_command", map[string]interface{}{
		"intent":  "list files",
		"command": "ls -la",
	})
	if got.ToolName != "execute_command" || got.Intent != "list files" {
		t.Errorf("structured fields wrong: %+v", got)
	}
	if len(got.Params) == 0 {
		t.Fatal("expected params for execute_command")
	}
	if got.Params[0].Name != "command" || got.Params[0].Value != "ls -la" {
		t.Errorf("first param wrong: %+v", got.Params[0])
	}
}

// TestBuildToolSummaryStructuredFallback verifies the generic fallback still
// populates ToolName/Intent (UC-0028).
func TestBuildToolSummaryStructuredFallback(t *testing.T) {
	got := buildToolSummary("list_settings", map[string]interface{}{"intent": "查看配置"})
	if got.ToolName != "list_settings" || got.Intent != "查看配置" {
		t.Errorf("fallback structured fields wrong: %+v", got)
	}
	if got.Text == "" {
		t.Error("fallback Text should be non-empty")
	}
}

// TestBuildToolSummaryTextNonEmpty verifies ToolSummary.Text is always set
// (UC-0029).
func TestBuildToolSummaryTextNonEmpty(t *testing.T) {
	for _, tool := range []string{"execute_command", "read_file", "list_settings", "ask_followup_question"} {
		got := buildToolSummary(tool, map[string]interface{}{"intent": "x"})
		if got.Text == "" {
			t.Errorf("%s Text should be non-empty", tool)
		}
	}
}

// TestBuildToolSummaryTruncation covers long-content truncation (UC-0006).
func TestBuildToolSummaryTruncation(t *testing.T) {
	longContent := strings.Repeat("A", 500)

	cases := []struct {
		name string
		tool string
		args map[string]interface{}
		want []string
	}{
		{
			name: "write_to_file long content not shown",
			tool: "write_to_file",
			args: map[string]interface{}{
				"intent":  "write",
				"mode":    "new",
				"path":    "a.txt",
				"content": longContent,
			},
			want: []string{"a.txt", "new"},
		},
		{
			name: "replace_in_file shows replacement count",
			tool: "replace_in_file",
			args: map[string]interface{}{
				"intent": "edit",
				"path":   "main.go",
				"replacements": []interface{}{
					map[string]interface{}{"search": "x", "replace": "y"},
					map[string]interface{}{"search": "a", "replace": "b"},
				},
			},
			want: []string{"main.go", "2"},
		},
		{
			name: "visual_analysis shows file count",
			tool: "visual_analysis",
			args: map[string]interface{}{
				"intent": "ocr",
				"paths":  []interface{}{"a.png", "b.png", "c.png"},
			},
			want: []string{"3"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildToolSummary(tc.tool, tc.args)
			if strings.Contains(got.Text, longContent) {
				t.Errorf("long content leaked into summary: %q", got.Text)
			}
			for _, w := range tc.want {
				if !strings.Contains(got.Text, w) {
					t.Errorf("summary missing %q, got: %q", w, got.Text)
				}
			}
		})
	}

	// truncate() behavior
	short := truncate("hello")
	if short != "hello" {
		t.Errorf("truncate should keep short strings, got %q", short)
	}
	long := truncate(longContent)
	if len(long) <= maxSummaryParamLen {
		t.Errorf("truncate did not trim, got len %d", len(long))
	}
}

// TestBuildToolSummaryShellTools covers shell tools (UC-0007).
func TestBuildToolSummaryShellTools(t *testing.T) {
	got := buildToolSummary("shell_send", map[string]interface{}{
		"intent":  "run python",
		"command": "python3 -c 'print(1)'",
	})
	if !strings.Contains(got.Text, "python3 -c 'print(1)'") || !strings.Contains(got.Text, "run python") {
		t.Errorf("shell_send summary wrong: %q", got.Text)
	}

	// Intent-only tools
	for _, tool := range []string{"shell_window_content", "shell_reset"} {
		got := buildToolSummary(tool, map[string]interface{}{"intent": "check state"})
		if !strings.Contains(got.Text, "check state") {
			t.Errorf("%s summary missing intent: %q", tool, got.Text)
		}
	}
}

// TestBuildToolSummaryDocTools covers excel/word tools (UC-0009).
func TestBuildToolSummaryDocTools(t *testing.T) {
	got := buildToolSummary("excel_open", map[string]interface{}{
		"intent": "analyze",
		"path":   "data.xlsx",
		"mode":   "read",
	})
	if !strings.Contains(got.Text, "data.xlsx") || !strings.Contains(got.Text, "read") || !strings.Contains(got.Text, "analyze") {
		t.Errorf("excel_open summary wrong: %q", got.Text)
	}

	got = buildToolSummary("word_open", map[string]interface{}{
		"intent": "edit doc",
		"path":   "doc.docx",
		"mode":   "copy",
	})
	if !strings.Contains(got.Text, "doc.docx") || !strings.Contains(got.Text, "copy") || !strings.Contains(got.Text, "edit doc") {
		t.Errorf("word_open summary wrong: %q", got.Text)
	}
}

// TestBuildToolSummaryZeroParamTools covers intent-only tools (UC-0010).
func TestBuildToolSummaryZeroParamTools(t *testing.T) {
	for _, tool := range []string{"view_task_plan", "list_settings"} {
		got := buildToolSummary(tool, map[string]interface{}{"intent": "check status"})
		if !strings.Contains(got.Text, "check status") {
			t.Errorf("%s summary missing intent: %q", tool, got.Text)
		}
	}
}

// TestToolSummaryI18nKeyPairs verifies zh/en translation key parity (UC-0011/0013).
func TestToolSummaryI18nKeyPairs(t *testing.T) {
	keys := []string{
		"tool_call_summary_generic",
		"tool_call_summary_exec_cmd",
		"tool_call_summary_read_file",
		"tool_call_summary_write_file",
		"tool_call_summary_replace_in_file",
		"tool_call_summary_search_files",
		"tool_call_summary_list_files",
		"tool_call_summary_list_defs",
		"tool_call_summary_shell_send",
		"tool_call_summary_visual_analysis",
		"tool_call_summary_excel_open",
		"tool_call_summary_word_open",
		"tool_call_summary_update_settings",
		"tool_call_summary_ask_question",
		"tool_call_summary_launch_sub_agent",
	}

	// Set lang to zh and verify built summary is NOT the raw key.
	original := i18n.GetLang()
	i18n.SetLang("zh")
	for _, k := range keys {
		if got := i18n.T(k); got == k {
			t.Errorf("zh translation missing for key %q", k)
		}
	}

	// Set lang to en and verify built summary is NOT the raw key.
	i18n.SetLang("en")
	for _, k := range keys {
		if got := i18n.T(k); got == k {
			t.Errorf("en translation missing for key %q", k)
		}
	}

	// Restore original language for other tests in the package.
	i18n.SetLang(string(original))
}

// TestBuildToolSummaryAskQuestionFull verifies ask_followup_question shows the
// complete question (no truncation) — the question is the very content the user
// must read and answer, so truncating it would defeat the purpose of asking.
func TestBuildToolSummaryAskQuestionFull(t *testing.T) {
	longQuestion := "模型文件正在后台传输中（deepseek模型156G，预计2-3小时），下载完成后需要重启服务才能生效，请问是否现在重启？"
	got := buildToolSummary("ask_followup_question", map[string]interface{}{
		"question": longQuestion,
	})
	if !strings.Contains(got.Text, longQuestion) {
		t.Errorf("ask_followup_question question should be shown in full, got: %q", got.Text)
	}
	if strings.Contains(got.Text, i18n.T(i18n.KeyToolCallSummaryTruncated)) {
		t.Errorf("ask_followup_question question should NOT be truncated, got: %q", got.Text)
	}
}

// TestToolSummaryJSONSerialization verifies a ToolSummary can be marshaled to
// JSON and carried in an EventToolCall Meta (UC-0030).
func TestToolSummaryJSONSerialization(t *testing.T) {
	summary := buildToolSummary("execute_command", map[string]interface{}{
		"intent":  "list files",
		"command": "ls -la",
	})
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal ToolSummary: %v", err)
	}

	// Build an EventToolCall carrying the summary in Meta.
	ev := NewStreamEvent(EventToolCall, ChannelTool, LevelInfo, summary.Text)
	ev.Meta = map[string]string{MetaKeyToolSummary: string(data)}
	if ev.Meta[MetaKeyToolSummary] == "" {
		t.Error("MetaKeyToolSummary missing from event")
	}

	// Unmarshal back and verify structured fields survive.
	var back ToolSummary
	if err := json.Unmarshal([]byte(ev.Meta[MetaKeyToolSummary]), &back); err != nil {
		t.Fatalf("unmarshal ToolSummary: %v", err)
	}
	if back.ToolName != "execute_command" || back.Intent != "list files" {
		t.Errorf("round-trip structured fields wrong: %+v", back)
	}
	if len(back.Params) == 0 || back.Params[0].Name != "command" {
		t.Errorf("round-trip params wrong: %+v", back.Params)
	}
}
