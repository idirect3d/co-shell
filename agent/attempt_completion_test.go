// Package agent - attempt_completion completion-confirm dialog tests
// (FEATURE-452).
//
// Author: L.Shuang
// Created: 2026-08-30

package agent

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// newAttemptCompletionAgent builds a minimal Agent for attemptCompletionTool
// tests with the completion-confirm switch set to the given value.
func newAttemptCompletionAgent(confirm bool) (*Agent, *captureInteractionManager) {
	mgr := &captureInteractionManager{}
	ag := &Agent{
		interactionMgr:        mgr,
		taskInstructionCache:  bytes.Buffer{},
		completed:             false,
	}
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{AttemptCompletionConfirm: confirm}})
	return ag, mgr
}

// TestAttemptCompletionConfirmExit verifies selecting "完成退出" (exit, "-")
// marks the task completed and does not store a user reply (FEATURE-452).
func TestAttemptCompletionConfirmExit(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(true)
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "exit", Raw: "-"}

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if !ag.completed {
		t.Error("completed = false, want true (exit should complete)")
	}
	if ag.taskInstructionCache.Len() != 0 {
		t.Errorf("taskInstructionCache = %q, want empty on exit", ag.taskInstructionCache.String())
	}
}

// TestAttemptCompletionConfirmContinue verifies selecting a next_step sends the
// choice back to the LLM (not completed) and stores it in the task instruction
// cache (FEATURE-452).
func TestAttemptCompletionConfirmContinue(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(true)
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "补充单元测试", Raw: "1"}

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
		"next_steps":       []interface{}{"补充单元测试", "更新文档"},
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if ag.completed {
		t.Error("completed = true, want false (continue should not complete)")
	}
	if got := ag.taskInstructionCache.String(); got != "补充单元测试" {
		t.Errorf("taskInstructionCache = %q, want 补充单元测试", got)
	}
}

// TestAttemptCompletionConfirmNotDone verifies selecting "任务尚未达到目标"
// (not_done, "+") sends the choice back to the LLM (not completed)
// (FEATURE-452).
func TestAttemptCompletionConfirmNotDone(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(true)
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "not_done", Raw: "+"}

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if ag.completed {
		t.Error("completed = true, want false (not_done should not complete)")
	}
	if got := ag.taskInstructionCache.String(); got != "not_done" {
		t.Errorf("taskInstructionCache = %q, want not_done", got)
	}
}

// TestAttemptCompletionConfirmDisabled verifies when the switch is disabled the
// tool completes directly without asking (FEATURE-452).
func TestAttemptCompletionConfirmDisabled(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(false)

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if !ag.completed {
		t.Error("completed = false, want true (disabled should complete directly)")
	}
	if mgr.captured.Kind != "" {
		t.Errorf("interaction captured = %+v, want none when disabled", mgr.captured)
	}
}

// TestAttemptCompletionBodyShowsSupervisorReport verifies that when the
// supervisor review returns a non-empty report (e.g. force-pass after max
// retries), the completion-confirm dialog Body carries the supervisor's
// conclusion/reason/suggestion for human review (FEATURE-459).
func TestAttemptCompletionBodyShowsSupervisorReport(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(true)
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "exit", Raw: "-"}
	// Enable supervisor and set rejectCount beyond maxRetries so
	// runSupervisorReview force-passes with a non-empty report (no real LLM call).
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{
		AttemptCompletionConfirm: true,
		Supervisor: config.SupervisorConfig{
			Enabled:     true,
			EntryObject: true,
			MaxRetries:  3,
		},
	}})
	ag.supervisorState = newSupervisorState()
	ag.supervisorState.ctx.rejectCount = 3
	ag.supervisorState.ctx.lastReason = "still incomplete"

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if mgr.captured.Body == "" {
		t.Error("dialog Body is empty, want supervisor report shown for human review")
	}
	if !strings.Contains(mgr.captured.Body, "强制放行") {
		t.Errorf("dialog Body = %q, want to contain 强制放行 (supervisor force-pass message)", mgr.captured.Body)
	}
}

// TestAttemptCompletionBodyEmptyWithoutSupervisor verifies that when the
// supervisor is disabled (no report), the completion-confirm dialog Body stays
// empty (FEATURE-459).
func TestAttemptCompletionBodyEmptyWithoutSupervisor(t *testing.T) {
	ag, mgr := newAttemptCompletionAgent(true)
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "exit", Raw: "-"}

	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if mgr.captured.Body != "" {
		t.Errorf("dialog Body = %q, want empty when supervisor disabled", mgr.captured.Body)
	}
}
