// Author: L.Shuang
// Created: 2026-08-30
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/taskplan"
)

// newSupervisorTestAgent builds a minimal Agent with supervisor config for tests.
func newSupervisorTestAgent() *Agent {
	ag := &Agent{
		supervisorState: newSupervisorState(),
	}
	ag.SetConfig(&config.Config{
		LLM: config.LLMConfig{
			Supervisor: config.SupervisorConfig{
				Enabled:     true,
				EntryObject:  true,
				EntryExit:    true,
				EntryTask:    false,
				ClearContext: false,
				MaxRetries:  20,
			},
		},
	})
	return ag
}

// UC-0011: parse a valid submit_review approve call.
func TestParseSupervisorReview_Approve(t *testing.T) {
	args := `{"approved":true,"reason":"delivery meets goal","suggestion":"verified all steps"}`
	r, err := parseSupervisorReview(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !r.Approved {
		t.Fatal("expected approved=true")
	}
	if r.Reason == "" || r.Suggestion == "" {
		t.Fatal("reason/suggestion should be preserved")
	}
}

// UC-0012: parse a valid submit_review reject call.
func TestParseSupervisorReview_Reject(t *testing.T) {
	args := `{"approved":false,"reason":"step 3 missing","suggestion":"implement step 3"}`
	r, err := parseSupervisorReview(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if r.Approved {
		t.Fatal("expected approved=false")
	}
	if r.Reason != "step 3 missing" {
		t.Fatalf("unexpected reason %q", r.Reason)
	}
}

// UC-0013: JSON embedded in surrounding text is parseable (lenient).
func TestParseSupervisorReview_EmbeddedJSON(t *testing.T) {
	args := `Here is my review: {"approved":true,"reason":"ok","suggestion":"none"}`
	r, err := parseSupervisorReview(args)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !r.Approved {
		t.Fatal("expected approved=true")
	}
}

// UC-0014: supervisorToolAllowed respects the whitelist (scheme B).
func TestSupervisorToolAllowed(t *testing.T) {
	ag := newSupervisorTestAgent()
	if !ag.supervisorToolAllowed("read_file") {
		t.Error("read_file should be allowed (default whitelist)")
	}
	if !ag.supervisorToolAllowed("execute_command") {
		t.Error("execute_command should be allowed (default whitelist)")
	}
	if ag.supervisorToolAllowed("write_to_file") {
		t.Error("write_to_file should NOT be allowed (not in whitelist)")
	}
	if ag.supervisorToolAllowed("replace_in_file") {
		t.Error("replace_in_file should NOT be allowed (not in whitelist)")
	}
}

// UC-0015: supervisorToolAllowed respects a custom configured whitelist.
func TestSupervisorToolAllowed_CustomWhitelist(t *testing.T) {
	ag := &Agent{}
	ag.SetConfig(&config.Config{
		LLM: config.LLMConfig{
			Supervisor: config.SupervisorConfig{
				AllowedTools: []string{"read_file", "memory_search"},
			},
		},
	})
	if !ag.supervisorToolAllowed("read_file") {
		t.Error("read_file should be allowed (custom whitelist)")
	}
	if ag.supervisorToolAllowed("execute_command") {
		t.Error("execute_command should NOT be allowed (not in custom whitelist)")
	}
}

// UC-0016: hasCompletedStep detects completed steps.
func TestHasCompletedStep(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"[X]", true},
		{"[x]", true},
		{"completed", true},
		{"[ ]", false},
		{"pending", false},
		{"in_progress", false},
	}
	for _, c := range cases {
		steps := []taskplan.StepInput{{Description: "s", Status: c.status}}
		if got := hasCompletedStep(steps); got != c.want {
			t.Errorf("hasCompletedStep(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}

// UC-0017: formatReviewAsText renders approve/reject text.
func TestFormatReviewAsText(t *testing.T) {
	approve := formatReviewAsText(&SupervisorReview{Approved: true, Reason: "ok", Suggestion: "done"})
	if !strings.Contains(approve, "放行") {
		t.Errorf("approve text should contain 放行, got %q", approve)
	}
	reject := formatReviewAsText(&SupervisorReview{Approved: false, Reason: "no", Suggestion: "fix"})
	if !strings.Contains(reject, "打回") {
		t.Errorf("reject text should contain 打回, got %q", reject)
	}
}

// UC-0018: formatReviewFeedback renders actionable rework feedback.
func TestFormatReviewFeedback(t *testing.T) {
	fb := formatReviewFeedback(&SupervisorReview{Approved: false, Reason: "step 3 missing", Suggestion: "implement step 3"})
	if !strings.Contains(fb, "step 3 missing") {
		t.Errorf("feedback should contain reason, got %q", fb)
	}
	if !strings.Contains(fb, "implement step 3") {
		t.Errorf("feedback should contain suggestion, got %q", fb)
	}
}

// UC-0019: supervisorMaxRetries returns configured value or default 20.
func TestSupervisorMaxRetries(t *testing.T) {
	ag := newSupervisorTestAgent()
	if got := ag.supervisorMaxRetries(); got != 20 {
		t.Errorf("maxRetries = %d, want 20", got)
	}
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{Supervisor: config.SupervisorConfig{MaxRetries: 5}}})
	if got := ag.supervisorMaxRetries(); got != 5 {
		t.Errorf("maxRetries = %d, want 5", got)
	}
}

// UC-0020: supervisorEntryEnabled respects per-entry switches.
func TestSupervisorEntryEnabled(t *testing.T) {
	ag := newSupervisorTestAgent()
	if !ag.supervisorEntryEnabled(SupervisorEntryA) {
		t.Error("EntryObject should be enabled by default")
	}
	if !ag.supervisorEntryEnabled(SupervisorEntryB) {
		t.Error("EntryExit should be enabled by default")
	}
	if ag.supervisorEntryEnabled(SupervisorEntryC) {
		t.Error("EntryTask should be disabled by default")
	}
}

// UC-0021: runSupervisorReview passes through when supervisor disabled.
func TestRunSupervisorReview_Disabled(t *testing.T) {
	ag := newSupervisorTestAgent()
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{Supervisor: config.SupervisorConfig{Enabled: false}}})
	approved, feedback, report := ag.runSupervisorReview(context.Background(), SupervisorEntryA, "report")
	if !approved {
		t.Error("should pass through when disabled")
	}
	if feedback != "" || report != "" {
		t.Errorf("disabled should return empty feedback/report, got %q / %q", feedback, report)
	}
}

// UC-0022: runSupervisorReview passes through when entry switch off.
func TestRunSupervisorReview_EntryOff(t *testing.T) {
	ag := newSupervisorTestAgent()
	// EntryTask is off by default.
	approved, feedback, report := ag.runSupervisorReview(context.Background(), SupervisorEntryC, "report")
	if !approved {
		t.Error("should pass through when entry switch off")
	}
	if feedback != "" || report != "" {
		t.Errorf("entry-off should return empty feedback/report, got %q / %q", feedback, report)
	}
}

// UC-0023: runSupervisorReview forces pass to user when max retries exceeded.
func TestRunSupervisorReview_MaxRetriesExceeded(t *testing.T) {
	ag := newSupervisorTestAgent()
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{Supervisor: config.SupervisorConfig{Enabled: true, EntryObject: true, MaxRetries: 3}}})
	// Simulate 3 prior rejections.
	ag.supervisorState.ctx.rejectCount = 3
	ag.supervisorState.ctx.lastReason = "still incomplete"
	approved, feedback, report := ag.runSupervisorReview(context.Background(), SupervisorEntryA, "report")
	if !approved {
		t.Error("should force pass to user when max retries exceeded")
	}
	if feedback != "" {
		t.Errorf("max-retry pass should have no feedback, got %q", feedback)
	}
	if !strings.Contains(report, "3 次") {
		t.Errorf("report should mention rejection count, got %q", report)
	}
}

// UC-0024: supervisor call failure degrades to pass-through (safe default) and
// surfaces the failure to the user via a non-empty report (FEATURE-459).
func TestRunSupervisorReview_CallFailure(t *testing.T) {
	ag := newSupervisorTestAgent()
	// No model available → callSupervisor returns error → pass through.
	approved, feedback, report := ag.runSupervisorReview(context.Background(), SupervisorEntryA, "report")
	if !approved {
		t.Error("should pass through when supervisor call fails")
	}
	if feedback != "" {
		t.Errorf("call-failure should return empty feedback, got %q", feedback)
	}
	// The failure must be surfaced to the user via a non-empty report.
	if report == "" {
		t.Error("call-failure should return a non-empty report with the failure info")
	}
	if !strings.Contains(report, "审查失败") {
		t.Errorf("report = %q, want to contain 审查失败 (supervisor failure info)", report)
	}
}

// UC-0025: getSettingValue reads supervisor parameter switches.
func TestGetSettingValue_Supervisor(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{Supervisor: config.SupervisorConfig{
		Enabled:      true,
		EntryObject:  true,
		EntryExit:    true,
		EntryTask:    false,
		ClearContext: false,
		MaxRetries:   20,
		AllowedTools: []string{"read_file", "memory_search"},
	}}}
	cases := []struct {
		param string
		want  string
	}{
		{"supervisor-enabled", "on"},
		{"supervisor-entry-object", "on"},
		{"supervisor-entry-exit", "on"},
		{"supervisor-entry-task", "off"},
		{"supervisor-clear-context", "off"},
		{"supervisor-max-retries", "20"},
		{"supervisor-allowed-tools", "read_file,memory_search"},
	}
	for _, c := range cases {
		if got := getSettingValue(cfg, c.param); got != c.want {
			t.Errorf("getSettingValue(%q) = %q, want %q", c.param, got, c.want)
		}
	}
}

// UC-0026: getSettingValue returns default marker for empty allowed-tools.
func TestGetSettingValue_SupervisorAllowedToolsDefault(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{Supervisor: config.SupervisorConfig{}}}
	if got := getSettingValue(cfg, "supervisor-allowed-tools"); got != "(default)" {
		t.Errorf("empty allowed-tools should return (default), got %q", got)
	}
}

// newSupervisorSettingAgent builds an Agent whose config has a real configPath
// (via a temp file) so applySetting's cfg.Save() succeeds.
func newSupervisorSettingAgent(t *testing.T) *Agent {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/config.json"
	// Create a real config file so LoadFromFile sets configPath (Save needs it).
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write config error: %v", err)
	}
	cfg, _, err := config.LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile error: %v", err)
	}
	ag := &Agent{supervisorState: newSupervisorState()}
	ag.SetConfig(cfg)
	return ag
}

// UC-0027: applySetting sets supervisor boolean switches.
func TestApplySetting_SupervisorBooleans(t *testing.T) {
	ag := newSupervisorSettingAgent(t)
	// Start with all false.
	ag.cfg.LLM.Supervisor.Enabled = false
	ag.cfg.LLM.Supervisor.EntryObject = false
	ag.cfg.LLM.Supervisor.EntryExit = false
	ag.cfg.LLM.Supervisor.EntryTask = false
	ag.cfg.LLM.Supervisor.ClearContext = false

	cases := []struct {
		param string
		value string
	}{
		{"supervisor-enabled", "true"},
		{"supervisor-entry-object", "true"},
		{"supervisor-entry-exit", "true"},
		{"supervisor-entry-task", "true"},
		{"supervisor-clear-context", "true"},
	}
	for _, c := range cases {
		if err := applySetting(ag, c.param, c.value); err != nil {
			t.Fatalf("applySetting(%q) error: %v", c.param, err)
		}
	}
	if !ag.cfg.LLM.Supervisor.Enabled {
		t.Error("supervisor-enabled should be true after set")
	}
	if !ag.cfg.LLM.Supervisor.EntryObject || !ag.cfg.LLM.Supervisor.EntryExit || !ag.cfg.LLM.Supervisor.EntryTask {
		t.Error("entry switches should be true after set")
	}
	if !ag.cfg.LLM.Supervisor.ClearContext {
		t.Error("clear-context should be true after set")
	}
}

// UC-0028: applySetting sets supervisor-max-retries with validation.
func TestApplySetting_SupervisorMaxRetries(t *testing.T) {
	ag := newSupervisorSettingAgent(t)
	if err := applySetting(ag, "supervisor-max-retries", "5"); err != nil {
		t.Fatalf("applySetting(max-retries=5) error: %v", err)
	}
	if ag.cfg.LLM.Supervisor.MaxRetries != 5 {
		t.Errorf("max-retries = %d, want 5", ag.cfg.LLM.Supervisor.MaxRetries)
	}
	// Negative value must be rejected.
	if err := applySetting(ag, "supervisor-max-retries", "-1"); err == nil {
		t.Error("negative max-retries should be rejected")
	}
	// Non-numeric value must be rejected.
	if err := applySetting(ag, "supervisor-max-retries", "abc"); err == nil {
		t.Error("non-numeric max-retries should be rejected")
	}
}

// UC-0029: applySetting sets supervisor-allowed-tools (comma-separated).
func TestApplySetting_SupervisorAllowedTools(t *testing.T) {
	ag := newSupervisorSettingAgent(t)
	if err := applySetting(ag, "supervisor-allowed-tools", "read_file, memory_search"); err != nil {
		t.Fatalf("applySetting(allowed-tools) error: %v", err)
	}
	if len(ag.cfg.LLM.Supervisor.AllowedTools) != 2 {
		t.Fatalf("allowed-tools len = %d, want 2", len(ag.cfg.LLM.Supervisor.AllowedTools))
	}
	if ag.cfg.LLM.Supervisor.AllowedTools[0] != "read_file" || ag.cfg.LLM.Supervisor.AllowedTools[1] != "memory_search" {
		t.Errorf("allowed-tools = %v, want [read_file memory_search]", ag.cfg.LLM.Supervisor.AllowedTools)
	}
	// Empty resets to default (nil).
	if err := applySetting(ag, "supervisor-allowed-tools", ""); err != nil {
		t.Fatalf("applySetting(allowed-tools empty) error: %v", err)
	}
	if ag.cfg.LLM.Supervisor.AllowedTools != nil {
		t.Errorf("empty allowed-tools should reset to nil, got %v", ag.cfg.LLM.Supervisor.AllowedTools)
	}
}

// UC-0030: applySetting rejects invalid boolean for supervisor switches.
func TestApplySetting_SupervisorInvalidBool(t *testing.T) {
	ag := newSupervisorSettingAgent(t)
	if err := applySetting(ag, "supervisor-enabled", "notabool"); err == nil {
		t.Error("invalid boolean should be rejected")
	}
}
