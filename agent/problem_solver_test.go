// Author: L.Shuang
// Created: 2026-08-10
// Last Modified: 2026-08-10
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// UC-0011: parse a valid report_problem tool call into ProblemReport.
func TestParseProblemReport_Basic(t *testing.T) {
	args := `{"type":"loop","reason":"repeated same tool","guidance":"用 read_file 读取文件后修改","suggested_action":"prompt_feedback"}`
	r, err := parseProblemReport(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !r.IsLoop() {
		t.Fatal("expected loop type")
	}
	if r.Guidance == "" || r.Reason == "" {
		t.Fatal("guidance/reason should be preserved")
	}
	if r.SuggestedAction != ActionPromptFeedback {
		t.Fatalf("unexpected action %q", r.SuggestedAction)
	}
}

// UC-0012: no_anomaly maps to continue and is not a loop.
func TestParseProblemReport_NoAnomaly(t *testing.T) {
	args := `{"type":"no_anomaly","reason":"normal output","guidance":"","suggested_action":"continue"}`
	r, err := parseProblemReport(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if r.IsLoop() {
		t.Fatal("no_anomaly must not be loop")
	}
	if !r.IsNoAnomaly() {
		t.Fatal("expected no_anomaly")
	}
	if r.SuggestedAction != ActionContinue {
		t.Fatalf("unexpected action %q", r.SuggestedAction)
	}
}

// UC-0013: pure JSON fallback (judgeLoop style) is parseable.
func TestParseProblemReport_PureJSON(t *testing.T) {
	args := `{"is_loop":true,"type":"loop","reason":"x","guidance":"do y","suggested_action":"retry"}`
	r, err := parseProblemReport(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !r.IsLoop() || r.SuggestedAction != ActionRetry {
		t.Fatalf("unexpected parse result: %+v", r)
	}
}

// UC-0013a: OpenAI tool_calls response path.
func TestCallProblemSolver_OpenAIToolCall(t *testing.T) {
	// The tolerant parse handles a tool-call arguments payload directly.
	args := `{"type":"tool_format_error","reason":"bad args","guidance":"call with correct args","suggested_action":"delete_last_msg"}`
	r, err := parseProblemReport(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if r.Type != ProblemTypeToolFormatError || r.SuggestedAction != ActionDeleteLastMsg {
		t.Fatalf("unexpected result: %+v", r)
	}
}

// UC-0015: unknown suggested_action falls back to notify_user.
func TestResolveProblemAction_Unknown(t *testing.T) {
	if got := resolveProblemAction("unknown_thing"); got != ActionNotifyUser {
		t.Fatalf("expected notify_user, got %q", got)
	}
	if got := resolveProblemAction(""); got != ActionNotifyUser {
		t.Fatalf("expected notify_user for empty, got %q", got)
	}
}

// UC-0016/17/18/19: HTTP status sufficient conditions classify connection errors.
func TestClassifyConnectionError_Sufficient(t *testing.T) {
	cases := []struct {
		status int
		ok     bool
	}{
		{401, true}, {403, true}, {404, true}, {429, true}, {500, true}, {200, false},
	}
	for _, c := range cases {
		err := &llm.OpenAIError{StatusCode: c.status}
		_, ok := classifyConnectionError(err)
		if ok != c.ok {
			t.Fatalf("status %d: expected ok=%v got %v", c.status, c.ok, ok)
		}
	}
}

// UC-0020: ambiguous error (non-OpenAIError, no keyword) is not sufficient.
func TestClassifyConnectionError_Ambiguous(t *testing.T) {
	_, ok := classifyConnectionError(errString("some weird business error"))
	if ok {
		t.Fatal("expected ambiguous=not classified")
	}
}

// reportProblemTool must expose the required fields.
func TestReportProblemTool_Schema(t *testing.T) {
	tool := reportProblemTool()
	if tool.Name != "report_problem" {
		t.Fatalf("unexpected tool name %q", tool.Name)
	}
	props, _ := tool.Parameters["properties"].(map[string]interface{})
	if props == nil {
		t.Fatal("missing properties")
	}
	for _, field := range []string{"type", "reason", "guidance", "suggested_action"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("missing required field %q", field)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// FEATURE-345: applyProblemAction maps suggested_action to run-stream behavior.
func TestApplyProblemAction(t *testing.T) {
	cases := []struct {
		name       string
		action     SuggestedAction
		guidance   string
		wantFB     bool
		wantDelete bool
		wantStop   bool
	}{
		{name: "prompt_feedback returns guidance", action: ActionPromptFeedback, guidance: "call read_file next", wantFB: true},
		{name: "compact_context returns guidance", action: ActionCompactContext, guidance: "reorganize now", wantFB: true},
		{name: "delete_last_msg", action: ActionDeleteLastMsg, wantDelete: true},
		{name: "notify_user stops", action: ActionNotifyUser, wantStop: true},
		{name: "continue keeps existing", action: ActionContinue},
		{name: "retry keeps existing", action: ActionRetry},
		{name: "unknown keeps existing", action: "weird_action"},
		{name: "nil report keeps existing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var report *ProblemReport
			if c.name != "nil report keeps existing" {
				report = &ProblemReport{Type: ProblemTypeUnknown, Guidance: c.guidance, SuggestedAction: c.action}
			}
			fb, del, stop := applyProblemAction(report)
			if (fb != "") != c.wantFB {
				t.Fatalf("feedback got %q want empty=%v", fb, !c.wantFB)
			}
			if del != c.wantDelete {
				t.Fatalf("deleteLast got %v want %v", del, c.wantDelete)
			}
			if stop != c.wantStop {
				t.Fatalf("stop got %v want %v", stop, c.wantStop)
			}
		})
	}
}

// FEATURE-345: solveProblem is gated by ProblemSolverEnabled.
func TestSolveProblem_Gated(t *testing.T) {
	a := &Agent{cfg: nil}
	r, err := a.solveProblem(context.Background(), ProblemTypeToolFormatError, "bad xml")
	if err != nil {
		t.Fatalf("solveProblem with nil cfg should not error: %v", err)
	}
	if r != nil {
		t.Fatalf("solveProblem with nil cfg should return nil report, got %+v", r)
	}
	cfg := &config.Config{}
	cfg.LLM.ProblemSolverEnabled = false
	a.cfg = cfg
	r, err = a.solveProblem(context.Background(), ProblemTypeToolFormatError, "bad xml")
	if err != nil {
		t.Fatalf("solveProblem with disabled solver should not error: %v", err)
	}
	if r != nil {
		t.Fatalf("solveProblem with disabled solver should return nil report, got %+v", r)
	}
}

// FEATURE-345: buildProblemSolverUserPrompt fills all placeholders and truncates.
func TestBuildProblemSolverUserPrompt(t *testing.T) {
	a := &Agent{}
	a.lastUserInput = "write a report"
	longDetail := strings.Repeat("x", 5000)
	prompt := a.buildProblemSolverUserPrompt(ProblemTypeContextOverflow, longDetail)
	if strings.Contains(prompt, "{ANOMALY_HINT}") || strings.Contains(prompt, "{ERROR_DETAIL}") {
		t.Fatal("unfilled placeholders remain")
	}
	if !strings.Contains(prompt, "context_overflow") {
		t.Fatal("anomaly hint not embedded")
	}
	if len(prompt) > 4000+2000 {
		t.Fatalf("prompt too large, detail not truncated: %d bytes", len(prompt))
	}
}

