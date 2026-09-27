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
