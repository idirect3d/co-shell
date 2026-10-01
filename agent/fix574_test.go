// Author: L.Shuang
// Created: 2026-10-01
//
// FIX-574: tests for the actionable meta/progress error messages and for the
// system-prompt progress rule (no more contradictory "unchanged steps can be
// omitted" wording). Use cases: use-case/FIX-574/FIX-574-UC-0001.md
// (UC-0001~UC-0004, UC-0007~UC-0013, UC-0016).

package agent

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestFIX574ProgressReportErrorText verifies the rejection error itself carries
// the missing steps (index, description, status) — UC-0008.
func TestFIX574ProgressReportErrorText(t *testing.T) {
	err := &ProgressReportError{Missing: []MissingProgressStep{
		{Index: 1, Title: "检索校验代码", Status: "in_progress"},
		{Index: 3, Title: "前端表达式解析器", Status: "in_progress"},
	}}
	msg := err.Error()
	for _, want := range []string{"[1]", "检索校验代码", "[3]", "前端表达式解析器", "in_progress", "missing step(s)"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error text %q does not contain %q", msg, want)
		}
	}
}

// TestFIX574FormatToolErrorForProgress verifies the rendered tool result lists
// the missing steps and no longer masquerades as a missing tool argument —
// UC-0007, UC-0009, UC-0012.
func TestFIX574FormatToolErrorForProgress(t *testing.T) {
	err := &ProgressReportError{Missing: []MissingProgressStep{
		{Index: 1, Title: "检索校验代码", Status: "in_progress"},
	}}
	got := formatToolError("read_file", err)

	for _, want := range []string{"REJECTED", "[1]", "检索校验代码", "meta.progress", "re-send the same call"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatted error missing %q:\n%s", want, got)
		}
	}
	// Must not fall back to the tool-argument template.
	for _, unwanted := range []string{"missing required parameters", "'path'", "start_line", "end_line"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("formatted error must not contain %q:\n%s", unwanted, got)
		}
	}
}

// TestFIX574FormatToolErrorForMetaFields verifies a meta field error lists the
// meta fields (and progress' minimal shape) instead of the tool's own
// parameters — UC-0010, UC-0011.
func TestFIX574FormatToolErrorForMetaFields(t *testing.T) {
	cases := []struct {
		name string
		err  string
	}{
		{"risk", "meta.risk is required and must be one of low/medium/high"},
		{"progress-empty", "meta.progress is required and must contain at least 1 current-status record"},
		{"meta-object", "meta object is required"},
		{"progress-shape", "progress[0].index is required and must be a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatToolError("execute_command", errors.New(tc.err))
			for _, want := range []string{"REJECTED", "intent", "risk", "risk_reason", "affected_objects", "progress", "meta"} {
				if !strings.Contains(got, want) {
					t.Errorf("formatted error missing %q:\n%s", want, got)
				}
			}
			for _, unwanted := range []string{"timeout_seconds", "on_timeout", "missing required parameters"} {
				if strings.Contains(got, unwanted) {
					t.Errorf("formatted error must not contain %q:\n%s", unwanted, got)
				}
			}
		})
	}
}

// TestFIX574ToolArgumentErrorKeepsTemplate verifies real tool-argument errors
// still use the original "missing required parameters" template — UC-0013.
func TestFIX574ToolArgumentErrorKeepsTemplate(t *testing.T) {
	got := formatToolError("read_file", errors.New("missing required parameter: path"))
	if !strings.Contains(got, "missing required parameters") || !strings.Contains(got, "'path'") {
		t.Errorf("tool-argument error must keep the argument template:\n%s", got)
	}
	if strings.Contains(got, "meta is REQUIRED") {
		t.Errorf("tool-argument error must not use the meta template:\n%s", got)
	}
}

// TestFIX574MetaErrorClassification verifies the classifier routes meta errors
// and progress errors to the dedicated branch, and everything else to the
// existing templates — UC-0009, UC-0013.
func TestFIX574MetaErrorClassification(t *testing.T) {
	meta := []string{
		"meta.intent is required",
		"meta.risk is required and must be one of low/medium/high",
		"meta.risk_reason is required",
		"meta.affected_objects is required",
		"meta.progress is required and must contain at least 1 current-status record",
		"meta object is required",
		"progress argument must be an array of objects",
		"progress[0].description is required",
	}
	for _, m := range meta {
		if !isMetaValidationError(m) {
			t.Errorf("isMetaValidationError(%q) = false, want true", m)
		}
	}
	notMeta := []string{
		"missing required parameter: path",
		"SEARCH block 1 not found in file",
		"cannot parse tool arguments",
		"index 7 is beyond the appendable range",
	}
	for _, m := range notMeta {
		if isMetaValidationError(m) {
			t.Errorf("isMetaValidationError(%q) = true, want false", m)
		}
	}
}

// TestFIX574SystemPromptProgressRule guards the prompt against the
// self-contradicting wording that caused the reported rejections — UC-0001,
// UC-0002, UC-0003, UC-0004, UC-0005.
func TestFIX574SystemPromptProgressRule(t *testing.T) {
	files := map[string]struct{ banned []string }{
		"../i18n/zh_system.go": {
			banned: []string{"未变化的步骤可以不传"},
		},
		"../i18n/en_system.go": {
			banned: []string{"steps that did not change can be omitted", "report ONLY steps whose status changed"},
		},
	}
	for path, spec := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, b := range spec.banned {
			if strings.Contains(text, b) {
				t.Errorf("%s still contains the contradictory progress wording %q", path, b)
			}
		}
	}

	zh, err := os.ReadFile("../i18n/zh_system.go")
	if err != nil {
		t.Fatalf("read zh prompt: %v", err)
	}
	if !strings.Contains(string(zh), "本次仍保持 in_progress 的步骤") {
		t.Error("zh prompt must require reporting steps that stay in_progress")
	}
	en, err := os.ReadFile("../i18n/en_system.go")
	if err != nil {
		t.Fatalf("read en prompt: %v", err)
	}
	if !strings.Contains(string(en), "stays in_progress") {
		t.Error("en prompt must require reporting steps that stay in_progress")
	}
}
