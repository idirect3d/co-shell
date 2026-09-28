// Package agent - FEATURE-568: attempt_completion executes its optional demo
// command BEFORE the completion-confirm dialog (it used to run only after the
// user confirmed completion).
//
// Author: L.Shuang
// Created: 2026-09-28

package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// feature568OrderManager records, for every Ask call, whether the demo-command
// marker file already existed when the dialog was presented. That answers the
// core FEATURE-568 question: "did the command run before the user was asked to
// confirm?". results holds one scripted result per Ask call (FIFO).
type feature568OrderManager struct {
	marker       string              // file the demo command creates
	results      []InteractionResult // one result per Ask call, in order
	calls        int
	cmdDoneAtAsk []bool // per Ask call: marker existed when Ask was entered
}

// Ask implements InteractionManager, recording the ordering observation.
func (m *feature568OrderManager) Ask(_ context.Context, _ Interaction) (InteractionResult, error) {
	_, err := os.Stat(m.marker)
	m.cmdDoneAtAsk = append(m.cmdDoneAtAsk, err == nil)
	if m.calls < len(m.results) {
		r := m.results[m.calls]
		m.calls++
		return r, nil
	}
	m.calls++
	return InteractionResult{Action: ActionApprove}, nil
}

// newFeature568Agent builds a minimal Agent whose completion-confirm behavior
// mode is the given one (active/simple/exit).
func newFeature568Agent(mgr InteractionManager, mode string) *Agent {
	ag := &Agent{
		interactionMgr:       mgr,
		taskInstructionCache: bytes.Buffer{},
	}
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{CompletionMode: mode}})
	return ag
}

// TestFeature568CommandRunsBeforeConfirmDialog verifies UC-0001: in active
// mode the demo command has already run when the completion-confirm dialog is
// presented, and its output is still appended to the final message.
func TestFeature568CommandRunsBeforeConfirmDialog(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uc1.marker")
	mgr := &feature568OrderManager{
		marker:  marker,
		results: []InteractionResult{{Action: ActionSelect, Value: "exit", Raw: "-"}},
	}
	ag := newFeature568Agent(mgr, "active")

	msg, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
		"command":          "touch " + marker + " && echo FEATURE568-UC1",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if len(mgr.cmdDoneAtAsk) != 1 {
		t.Fatalf("Ask calls = %d, want 1 (active mode shows one dialog)", len(mgr.cmdDoneAtAsk))
	}
	if !mgr.cmdDoneAtAsk[0] {
		t.Error("command had NOT run when the confirm dialog appeared; want it already executed (FEATURE-568)")
	}
	if !ag.completed {
		t.Error("completed = false, want true after selecting exit")
	}
	if !strings.Contains(msg, "FEATURE568-UC1") {
		t.Errorf("final message does not carry the command output: %q", msg)
	}
}

// TestFeature568CommandRunsOnceWhenUserContinues verifies UC-0002: when the
// user picks "continue" the command has already run exactly once — it is not
// re-run on the follow-up input.
func TestFeature568CommandRunsOnceWhenUserContinues(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uc2.log")
	mgr := &feature568OrderManager{
		marker: marker,
		results: []InteractionResult{
			{Action: ActionSelect, Value: "continue"},
			{Action: ActionInput, Value: "再改一下"},
		},
	}
	ag := newFeature568Agent(mgr, "simple")

	if _, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
		"command":          "echo FEATURE568 >> " + marker,
	}); err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if len(mgr.cmdDoneAtAsk) == 0 || !mgr.cmdDoneAtAsk[0] {
		t.Error("command had NOT run when the confirm dialog appeared; want it already executed (FEATURE-568)")
	}
	if ag.completed {
		t.Error("completed = true, want false (user chose continue)")
	}
	if got := ag.taskInstructionCache.String(); got != "再改一下" {
		t.Errorf("taskInstructionCache = %q, want 再改一下", got)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if n := strings.Count(string(data), "\n"); n != 1 {
		t.Errorf("command ran %d time(s), want exactly 1 (%q)", n, string(data))
	}
}

// TestFeature568NoCommandNoOutput verifies UC-0003 (regression): without a
// command nothing is executed and the final message stays exactly as before.
func TestFeature568NoCommandNoOutput(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "never")
	mgr := &feature568OrderManager{
		marker:  marker,
		results: []InteractionResult{{Action: ActionSelect, Value: "exit", Raw: "-"}},
	}
	ag := newFeature568Agent(mgr, "simple")

	msg, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if !ag.completed {
		t.Error("completed = false, want true after selecting exit")
	}
	if want := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_612), "done"); msg != want {
		t.Errorf("message = %q, want %q (no command output segment)", msg, want)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("marker exists, want no command execution when command is empty")
	}
}

// TestFeature568ExitModeStillRunsCommand verifies UC-0003 (regression): with
// completion-mode=exit there is no dialog at all, and the command still runs
// before the task is marked completed.
func TestFeature568ExitModeStillRunsCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uc3.marker")
	mgr := &feature568OrderManager{marker: marker}
	ag := newFeature568Agent(mgr, "exit")

	msg, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "t",
		"session_keywords": "k",
		"command":          "touch " + marker + " && echo FEATURE568-UC3",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if len(mgr.cmdDoneAtAsk) != 0 {
		t.Errorf("Ask calls = %d, want 0 in exit mode (no dialog)", len(mgr.cmdDoneAtAsk))
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("marker missing, want the command executed in exit mode: %v", err)
	}
	if !ag.completed {
		t.Error("completed = false, want true in exit mode")
	}
	if !strings.Contains(msg, "FEATURE568-UC3") {
		t.Errorf("final message does not carry the command output: %q", msg)
	}
}

// TestFeature568ParamValidationSkipsCommand verifies UC-0003 step 3: argument
// validation still rejects the call (and never executes the command) before
// any of the new ordering behavior applies.
func TestFeature568ParamValidationSkipsCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uc3b.marker")
	mgr := &feature568OrderManager{marker: marker}
	ag := newFeature568Agent(mgr, "simple")

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{"missing result", map[string]interface{}{"session_title": "t", "session_keywords": "k", "command": "touch " + marker}},
		{"missing session_title", map[string]interface{}{"result": "done", "session_keywords": "k", "command": "touch " + marker}},
		{"missing session_keywords", map[string]interface{}{"result": "done", "session_title": "t", "command": "touch " + marker}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ag.attemptCompletionTool(context.Background(), tt.args); err == nil {
				t.Fatal("error = nil, want a validation error")
			}
			if _, err := os.Stat(marker); err == nil {
				t.Error("marker exists, want no command execution when validation fails")
			}
		})
	}
	if len(mgr.cmdDoneAtAsk) != 0 {
		t.Errorf("Ask calls = %d, want 0 when validation fails", len(mgr.cmdDoneAtAsk))
	}
}
