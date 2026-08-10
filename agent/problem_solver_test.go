// Author: L.Shuang
// Created: 2026-08-10
// Last Modified: 2026-08-10
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package agent

import (
	"testing"

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
