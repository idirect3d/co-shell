// Package agent - attempt_completion completion-confirm dialog tests
// (FEATURE-452).
//
// Author: L.Shuang
// Created: 2026-08-30

package agent

import (
	"bytes"
	"context"
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
