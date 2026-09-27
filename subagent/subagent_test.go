// Package subagent - validation, stream parsing and status block tests
// (FEATURE-563, use-case A-group items 1/2/5).
//
// Author: L.Shuang
// Created: 2026-09-27
// MIT License - Copyright (c) 2026 L.Shuang

package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateID accepts the documented id shape and rejects everything a
// path traversal or an unusable folder name could hide in (UC: id 校验).
func TestValidateID(t *testing.T) {
	for _, id := range []string{"a", "A9", "sub-agent_1", strings.Repeat("x", 32)} {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range []string{"", strings.Repeat("x", 33), "../evil", "a/b", "a b", "a.b", "子agent"} {
		if err := ValidateID(id); err == nil {
			t.Errorf("ValidateID(%q) = nil, want error", id)
		}
	}
}

// TestParseTasks covers the batch shape: at least one task, valid unique ids
// and a non-empty instruction (UC: 批量创建).
func TestParseTasks(t *testing.T) {
	if _, err := ParseTasks(nil); err == nil {
		t.Error("ParseTasks(nil) must fail")
	}
	if _, err := ParseTasks([]Task{{ID: "a", Instruction: "  "}}); err == nil {
		t.Error("blank instruction must fail")
	}
	if _, err := ParseTasks([]Task{{ID: "../x", Instruction: "x"}}); err == nil {
		t.Error("invalid id must fail")
	}
	if _, err := ParseTasks([]Task{{ID: "a", Instruction: "x"}, {ID: "a", Instruction: "y"}}); err == nil {
		t.Error("duplicate id within one batch must fail")
	}
	got, err := ParseTasks([]Task{{ID: " a ", Instruction: "do it"}})
	if err != nil {
		t.Fatalf("ParseTasks: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" || got[0].Instruction != "do it" {
		t.Fatalf("ParseTasks = %+v, want id trimmed to \"a\"", got)
	}
}

// TestValidateWaitSeconds pins the 1..600 window (UC: timeout_seconds 范围).
func TestValidateWaitSeconds(t *testing.T) {
	for _, s := range []int{MinWaitSeconds, MaxWaitSeconds} {
		if err := ValidateWaitSeconds(s); err != nil {
			t.Errorf("ValidateWaitSeconds(%d) = %v, want nil", s, err)
		}
	}
	for _, s := range []int{-1, 0, MaxWaitSeconds + 1} {
		if err := ValidateWaitSeconds(s); err == nil {
			t.Errorf("ValidateWaitSeconds(%d) = nil, want error", s)
		}
	}
}

// TestParseStreamKeepsLastContentRun verifies the final answer is the last
// uninterrupted content run: thinking and tool calls close a run, so the
// reasoning text never reaches the parent (UC: 结果提取不含 thinking).
func TestParseStreamKeepsLastContentRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	lines := []string{
		`{"type":"thinking_chunk","text":"secret reasoning"}`,
		`{"type":"content_chunk","text":"draft answer"}`,
		`{"type":"tool_call","meta":{"name":"read_file"}}`,
		`{"type":"content","text":"final "}`,
		`{"type":"content_chunk","text":"answer"}`,
		`{not json at all`,
		``,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	result, tools := parseStream(path, 0)
	if result != "final answer" {
		t.Errorf("result = %q, want %q", result, "final answer")
	}
	if strings.Contains(result, "secret") || strings.Contains(result, "draft") {
		t.Errorf("result leaked an earlier segment: %q", result)
	}
	if len(tools) != 1 || tools[0] != "read_file" {
		t.Errorf("tool calls = %v, want [read_file]", tools)
	}
}

// TestParseStreamTruncates verifies the result cap keeps the parent context
// small (UC: 结果长度受限).
func TestParseStreamTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	body := strings.Repeat("a", 100)
	line := `{"type":"content_chunk","text":"` + body + `"}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	result, _ := parseStream(path, 10)
	if !strings.HasPrefix(result, strings.Repeat("a", 10)) {
		t.Errorf("truncated result = %q", result)
	}
	if !strings.HasSuffix(result, truncationSuffix) {
		t.Errorf("truncated result must carry the marker, got %q", result)
	}
}

// TestParseStreamMissingLog degrades to an empty result instead of failing.
func TestParseStreamMissingLog(t *testing.T) {
	result, tools := parseStream(filepath.Join(t.TempDir(), "absent.jsonl"), 0)
	if result != "" || tools != nil {
		t.Errorf("parseStream(missing) = (%q, %v), want empty", result, tools)
	}
}

// TestStatusBlockOmittedWhenEmpty keeps <environment_details> clean while no
// sub-agent exists (UC: 状态块省略).
func TestStatusBlockOmittedWhenEmpty(t *testing.T) {
	m := NewManager(Options{Workspace: t.TempDir(), NoStoreLoad: true})
	if got := m.StatusBlock(); got != "" {
		t.Errorf("StatusBlock() = %q, want empty", got)
	}
}

// TestSubAgentDirIsScopedToWorkspace pins the {workspace}/sub-agents/{id}/
// layout that replaced the former sibling-directory convention.
func TestSubAgentDirIsScopedToWorkspace(t *testing.T) {
	ws := t.TempDir()
	m := NewManager(Options{Workspace: ws, NoStoreLoad: true})
	want := filepath.Join(ws, "sub-agents", "alpha")
	if got := m.SubAgentDir("alpha"); got != want {
		t.Errorf("SubAgentDir = %q, want %q", got, want)
	}
}

// TestParseStreamFallsBackToCompletionResult verifies the fallback chain: when
// the last content run is empty (co-shell finishes by calling
// attempt_completion, whose text is echoed as a tool result), the completion
// result is reported instead of an empty answer.
func TestParseStreamFallsBackToCompletionResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	lines := []string{
		`{"type":"thinking_chunk","text":"hidden reasoning"}`,
		`{"type":"content_chunk","text":"I will report the listing now."}`,
		`{"type":"tool_call","chan":"tool","text":"[🔧 attempt_completion] 目的: 回报输出","meta":{"phase":"input","tool_summary":"{\"text\":\"[🔧 attempt_completion]\",\"tool_name\":\"attempt_completion\",\"params\":null}"}}`,
		`{"type":"tool_call","chan":"tool","text":"[返回结果]✅ 任务完成\n\ntotal 48\ndrwxr-xr-x  tmp\n\n[意图] 回报输出","meta":{"phase":"result"}}`,
		`{"type":"done","chan":"system"}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	result, tools := parseStream(path, 0)
	if !strings.Contains(result, "total 48") {
		t.Errorf("result = %q, want the attempt_completion result", result)
	}
	if strings.Contains(result, "[返回结果]") || strings.Contains(result, "[意图]") {
		t.Errorf("result kept renderer decorations: %q", result)
	}
	if strings.Contains(result, "hidden") || strings.Contains(result, "I will report") {
		t.Errorf("result leaked reasoning/earlier content: %q", result)
	}
	if len(tools) != 1 || tools[0] != "attempt_completion" {
		t.Errorf("tool calls = %v, want [attempt_completion]", tools)
	}
}

// TestParseStreamLastSegmentWins pins the ordering rule: content produced AFTER
// attempt_completion wins over its result, and a completion that comes last
// wins over earlier content.
func TestParseStreamLastSegmentWins(t *testing.T) {
	completion := `{"type":"tool_call","chan":"tool","text":"[返回结果]✅ 任务完成\n\nanswer from completion","meta":{"phase":"result"}}`

	// completion → later content: content wins.
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	lines := []string{
		`{"type":"tool_call","chan":"tool","text":"[🔧 attempt_completion]","meta":{"phase":"input","tool_summary":"{\"tool_name\":\"attempt_completion\"}"}}`,
		completion,
		`{"type":"content","text":"corrected answer"}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if got, _ := parseStream(path, 0); got != "corrected answer" {
		t.Errorf("content after completion: result = %q, want %q", got, "corrected answer")
	}

	// content → completion: the completion result wins.
	path2 := filepath.Join(t.TempDir(), "run-2.jsonl")
	lines2 := []string{
		`{"type":"content","text":"chatter before completing"}`,
		`{"type":"tool_call","chan":"tool","text":"[🔧 attempt_completion]","meta":{"phase":"input","tool_summary":"{\"tool_name\":\"attempt_completion\"}"}}`,
		completion,
	}
	if err := os.WriteFile(path2, []byte(strings.Join(lines2, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if got, _ := parseStream(path2, 0); !strings.Contains(got, "answer from completion") {
		t.Errorf("completion last: result = %q, want the completion result", got)
	}
}

// TestParseStreamCompletionResultTruncated verifies the result cap also applies
// to the attempt_completion fallback and never splits a UTF-8 rune.
func TestParseStreamCompletionResultTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	body := strings.Repeat("结", 40)
	lines := []string{
		`{"type":"tool_call","chan":"tool","text":"[🔧 attempt_completion]","meta":{"phase":"input","tool_summary":"{\"tool_name\":\"attempt_completion\"}"}}`,
		`{"type":"tool_call","chan":"tool","text":"[返回结果]` + body + `","meta":{"phase":"result"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	result, _ := parseStream(path, 10)
	trimmed := strings.TrimSuffix(result, truncationSuffix)
	if !strings.HasSuffix(result, truncationSuffix) {
		t.Fatalf("result must carry the truncation marker, got %q", result)
	}
	if len(trimmed) == 0 || len(trimmed) > 10 {
		t.Fatalf("truncated body = %d bytes, want (0,10]", len(trimmed))
	}
	if !strings.HasPrefix(body, trimmed) {
		t.Errorf("truncated body %q is not a prefix of the result text", trimmed)
	}
	if strings.ToValidUTF8(trimmed, "") != trimmed {
		t.Errorf("truncation split a UTF-8 rune: %q", trimmed)
	}
}

// TestToolLabelDropsLongText verifies long tool output never becomes a trail
// entry: only structured names, the renderer's bracketed marker, or a short
// single line are kept.
func TestToolLabelDropsLongText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-1.jsonl")
	longFirstLine := strings.Repeat("x", maxToolLabelLen+1)
	lines := []string{
		`{"type":"tool_call","chan":"tool","text":"` + longFirstLine + `"}`,
		`{"type":"tool_call","chan":"tool","text":"[🔧 read_file] 目的: 读取","meta":{"phase":"input"}}`,
		`{"type":"tool_call","chan":"tool","text":"` + strings.Repeat("y", 200) + `"}`,
		`{"type":"tool_call","chan":"tool","text":"执行命令: ls -la（超时: 30s）"}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	_, tools := parseStream(path, 0)
	assertTrailEntries(t, tools, []string{"[🔧 read_file]", "执行命令: ls -la（超时: 30s）"}, longFirstLine)
}

// assertTrailEntries checks the tool-call trail holds exactly want, that no
// entry exceeds maxToolLabelLen, and that the forbidden (long) text never leaks
// into it.
func assertTrailEntries(t *testing.T, got, want []string, forbidden string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("tool calls = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("tool calls[%d] = %q, want %q", i, got[i], w)
		}
	}
	for _, g := range got {
		if len(g) > maxToolLabelLen {
			t.Errorf("trail entry %q exceeds maxToolLabelLen (%d)", g, maxToolLabelLen)
		}
		if forbidden != "" && strings.Contains(g, forbidden) {
			t.Errorf("long tool output leaked into the trail: %q", g)
		}
	}
}
