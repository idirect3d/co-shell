// Package agent - FIX-581: attempt_completion demo command execution timeout.
//
// Author: L.Shuang
// Created: 2026-10-02

package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// newFIX581Agent builds a minimal Agent with the demo-command timeout (seconds)
// used by the FIX-581 tests. The completion-confirm switch is off, so the
// delivery tool returns without showing a dialog.
func newFIX581Agent(timeoutSec int) *Agent {
	mgr := &captureInteractionManager{}
	mgr.askResult = InteractionResult{Action: ActionSelect, Value: "exit", Raw: "-"}
	ag := &Agent{interactionMgr: mgr}
	ag.SetConfig(&config.Config{LLM: config.LLMConfig{
		AttemptCompletionConfirm: false,
		DemoCommandTimeout:       timeoutSec,
	}})
	return ag
}

// fix581TimeoutNotice is the expected timeout notice for an empty output.
func fix581TimeoutNotice(sec int) string {
	return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_784), sec, "")
}

// fix581ProcessAlive reports whether a process matching pattern is running.
func fix581ProcessAlive(pattern string) bool {
	out, err := exec.Command("pgrep", "-f", pattern).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// fix581KillPattern is a best-effort cleanup of helper processes.
func fix581KillPattern(pattern string) {
	_ = exec.Command("pkill", "-f", pattern).Run()
}

// TestFIX581DefaultTimeoutIsThreeSeconds verifies the default of the new system
// parameter (UC-0001): 3 seconds, not 0.
func TestFIX581DefaultTimeoutIsThreeSeconds(t *testing.T) {
	if got := config.DefaultConfig().LLM.DemoCommandTimeout; got != 3 {
		t.Errorf("DemoCommandTimeout default = %d, want 3", got)
	}
}

// TestFIX581DemoCommandTimeoutReturnsAndKillsGroup verifies UC-0005: a
// non-returning command returns within the timeout and its process group is
// killed (no leftover child).
func TestFIX581DemoCommandTimeoutReturnsAndKillsGroup(t *testing.T) {
	ag := newFIX581Agent(1)
	start := time.Now()
	out := ag.runDemoCommand(context.Background(), "sleep 5810")
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("runDemoCommand took %s, want it bounded by the 1s timeout", elapsed)
	}
	if !strings.HasPrefix(out, fix581TimeoutNotice(1)) {
		t.Errorf("output = %q, want the timeout notice %q", out, fix581TimeoutNotice(1))
	}
	if fix581ProcessAlive("sleep 5810") {
		fix581KillPattern("sleep 5810")
		t.Error("sleep 5810 still alive, want the process group killed on timeout")
	}
}

// TestFIX581TimeoutKillsPipelineChildren verifies UC-0006: grandchildren of the
// timed-out command (here a shell pipeline) are killed as well.
func TestFIX581TimeoutKillsPipelineChildren(t *testing.T) {
	ag := newFIX581Agent(1)
	out := ag.runDemoCommand(context.Background(), "sleep 5811 | sleep 5812")

	if !strings.HasPrefix(out, fix581TimeoutNotice(1)) {
		t.Errorf("output = %q, want the timeout notice", out)
	}
	for _, pattern := range []string{"sleep 5811", "sleep 5812"} {
		if fix581ProcessAlive(pattern) {
			fix581KillPattern(pattern)
			t.Errorf("%s still alive, want the whole process group killed", pattern)
		}
	}
}

// TestFIX581NormalFinishKeepsBackgroundChild verifies UC-0008: only a timeout
// kills children — a command that finishes on its own keeps its background
// child running (FIX-320 rule).
func TestFIX581NormalFinishKeepsBackgroundChild(t *testing.T) {
	ag := newFIX581Agent(3)
	out := ag.runDemoCommand(context.Background(), "sleep 5814 >/dev/null 2>&1 & echo $!")
	defer fix581KillPattern("sleep 5814")

	if strings.HasPrefix(out, fix581TimeoutNotice(3)) {
		t.Errorf("output = %q, want a normal message (the command finished on its own)", out)
	}
	if !fix581ProcessAlive("sleep 5814") {
		t.Error("background child was killed although the command finished on its own")
	}
}

// TestFIX581LingeringPipeDoesNotBlock verifies UC-0009: a child holding the pipe
// must neither block the call nor be killed when the command itself exited.
func TestFIX581LingeringPipeDoesNotBlock(t *testing.T) {
	ag := newFIX581Agent(3)
	start := time.Now()
	out := ag.runDemoCommand(context.Background(), "sleep 5815 &")
	elapsed := time.Since(start)
	defer fix581KillPattern("sleep 5815")

	if elapsed > 5*time.Second {
		t.Fatalf("runDemoCommand took %s, want no blocking on the lingering pipe", elapsed)
	}
	if strings.HasPrefix(out, fix581TimeoutNotice(3)) {
		t.Errorf("output = %q, want a normal message (the command itself exited)", out)
	}
	if !fix581ProcessAlive("sleep 5815") {
		t.Error("background child was killed although the command finished on its own")
	}
}

// TestFIX581ZeroTimeoutMeansNoLimit verifies UC-0010: 0 disables the timeout.
func TestFIX581ZeroTimeoutMeansNoLimit(t *testing.T) {
	ag := newFIX581Agent(0)
	out := ag.runDemoCommand(context.Background(), "sleep 2 && echo done")

	if !strings.Contains(out, "done") {
		t.Errorf("output = %q, want it to contain done", out)
	}
	if strings.HasPrefix(out, fix581TimeoutNotice(0)) {
		t.Errorf("output = %q, want no timeout notice when the timeout is 0", out)
	}
}

// TestFIX581FailureIsNotReportedAsTimeout verifies UC-0011: a non-zero exit is
// reported as a failure, never as a timeout.
func TestFIX581FailureIsNotReportedAsTimeout(t *testing.T) {
	ag := newFIX581Agent(3)
	out := ag.runDemoCommand(context.Background(), "echo boom && exit 3")

	if !strings.Contains(out, "boom") {
		t.Errorf("output = %q, want it to contain the command output", out)
	}
	if strings.HasPrefix(out, fix581TimeoutNotice(3)) {
		t.Errorf("output = %q, want a failure message, not a timeout notice", out)
	}
}

// TestFIX581TimeoutNoticeI18n verifies UC-0012: the notice exists in zh and en.
func TestFIX581TimeoutNoticeI18n(t *testing.T) {
	orig := i18n.GetLang()
	defer i18n.SetLang(string(orig))

	if !i18n.SetLang("zh") {
		t.Fatal("cannot switch to zh")
	}
	zh := i18n.T(i18n.KeySettingCmd_784)
	if !i18n.SetLang("en") {
		t.Fatal("cannot switch to en")
	}
	en := i18n.T(i18n.KeySettingCmd_784)

	if zh == "" || en == "" || zh == en {
		t.Errorf("timeout notice zh=%q en=%q, want distinct non-empty texts", zh, en)
	}
	for _, s := range []string{zh, en} {
		if strings.Contains(s, "MISSING") || strings.Contains(s, "%!") {
			t.Errorf("unresolved format placeholder in %q", s)
		}
		if !strings.Contains(s, "%d") {
			t.Errorf("notice must carry the timeout value: %q", s)
		}
	}
}

// TestFIX581AttemptCompletionDoesNotBlock covers the tool level (UC-0005,
// UC-0007 shape): the delivery tool returns promptly even though the demo
// command never exits on its own.
func TestFIX581AttemptCompletionDoesNotBlock(t *testing.T) {
	ag := newFIX581Agent(1)
	start := time.Now()
	_, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "fix581",
		"session_keywords": "fix581",
		"command":          "sleep 5816",
	})
	elapsed := time.Since(start)
	defer fix581KillPattern("sleep 5816")

	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if elapsed > 6*time.Second {
		t.Fatalf("attemptCompletionTool took %s, want it bounded by the demo timeout", elapsed)
	}
	if !ag.completed {
		t.Error("completed = false, want true (confirm disabled → exit mode)")
	}
}

// TestFIX581NoCommandKeepsBehaviour verifies UC-0015: without a command the tool
// behaves exactly as before.
func TestFIX581NoCommandKeepsBehaviour(t *testing.T) {
	ag := newFIX581Agent(3)
	msg, err := ag.attemptCompletionTool(context.Background(), map[string]interface{}{
		"result":           "done",
		"session_title":    "fix581",
		"session_keywords": "fix581",
	})
	if err != nil {
		t.Fatalf("attemptCompletionTool error: %v", err)
	}
	if !ag.completed {
		t.Error("completed = false, want true")
	}
	if !strings.Contains(msg, "done") {
		t.Errorf("message = %q, want it to carry the result", msg)
	}
}
