// Author: L.Shuang
// Created: 2026-08-03
// Last Modified: 2026-08-03
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build !windows

package agent

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
)

// newTestAgentWithCommandTimeout returns an Agent whose command timeout is
// configured to the given seconds. Only the fields used by the tested command
// paths are set; everything else stays at zero values.
func newTestAgentWithCommandTimeout(sec int) *Agent {
	return &Agent{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				CommandTimeout: sec,
			},
		},
	}
}

// assertProcessAlive fails the test if the process identified by pid is not
// currently running. signal(0) is a standard existence probe on Unix.
func assertProcessAlive(t *testing.T, pid int, what string) {
	t.Helper()
	proc, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("%s: cannot lookup PID %d: %v", what, pid, err)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("%s: process %d is not running: %v", what, pid, err)
	}
}

// assertProcessGone fails the test if a process matching the pgrep pattern
// is still running. Used to verify that a timed-out pipeline's grandchildren
// (e.g. `sleep 60` from `sleep 60 | cat`) were killed with the process group.
func assertProcessGone(t *testing.T, pattern string) {
	t.Helper()
	out, _ := exec.Command("pgrep", "-f", pattern).Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		t.Fatalf("process matching %q still running after timeout: %s", pattern, out)
	}
}

// waitPastTimeout sleeps a little longer than the configured timeout so that
// any lingering timeout goroutine would have fired by the time we check.
func waitPastTimeout(sec int) {
	time.Sleep(time.Duration(sec)*time.Second + 500*time.Millisecond)
}

// TestExecuteSystemCommand_BackgroundSurvivesAfterEarlyExit verifies the core
// FIX-320 fix for the LLM path (UC-0001): when the command finishes before the
// timeout, the timeout goroutine must abort its pending kill so a background
// job (e.g. `sleep 300 &`) that stays in the same process group is NOT killed.
func TestExecuteSystemCommand_BackgroundSurvivesAfterEarlyExit(t *testing.T) {
	a := newTestAgentWithCommandTimeout(2)

	start := time.Now()
	// The background job redirects its stdout/stderr so it does NOT hold the
	// pipe open — bash exits immediately, cmd.Wait() returns, and the background
	// sleep keeps running on its own. This is the exact scenario FIX-320 fixes:
	// the lingering timeout goroutine must NOT kill it.
	out, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command": "sleep 300 >/dev/null 2>&1 & echo $!",
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected early success, got error: %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("command returned after timeout window (%v) instead of immediately", elapsed)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("cannot parse background PID from output %q: %v", out, err)
	}
	if pid <= 0 {
		t.Fatalf("invalid background PID: %d", pid)
	}

	// Wait past the 2s timeout so a lingering goroutine would have killed the group.
	waitPastTimeout(2)

	// The background sleep must still be alive.
	assertProcessAlive(t, pid, "background sleep")

	// Cleanup: terminate the still-running background job.
	if proc, err := os.FindProcess(pid); err == nil {
		proc.Kill()
	}
}

// TestExecuteCommandDirectly_BackgroundSurvivesAfterEarlyExit applies the same
// check to the REPL path (ExecuteCommandDirectly), which previously used
// exec.CommandContext and lacked the lingering-goroutine protection.
func TestExecuteCommandDirectly_BackgroundSurvivesAfterEarlyExit(t *testing.T) {
	a := newTestAgentWithCommandTimeout(2)

	start := time.Now()
	out, err := a.ExecuteCommandDirectly("sleep 300 >/dev/null 2>&1 & echo $!")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected early success, got error: %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("command returned after timeout window (%v) instead of immediately", elapsed)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("cannot parse background PID from output %q: %v", out, err)
	}

	waitPastTimeout(2)
	assertProcessAlive(t, pid, "background sleep (REPL path)")

	if proc, err := os.FindProcess(pid); err == nil {
		proc.Kill()
	}
}

// TestExecuteSystemCommand_TimeoutKillsProcessGroup verifies the timeout still
// works normally (UC-0002): a long-running foreground command is killed via
// the whole process group after the timeout fires.
func TestExecuteSystemCommand_TimeoutKillsProcessGroup(t *testing.T) {
	a := newTestAgentWithCommandTimeout(2)

	start := time.Now()
	_, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command": "sleep 60",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got success")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed < 2*time.Second {
		t.Fatalf("returned too early (%v) — timeout should fire at ~2s", elapsed)
	}

	assertProcessGone(t, "sleep 60")
}

// TestExecuteCommandDirectly_TimeoutKillsPipeline verifies the REPL path with a
// pipeline (UC-0003): `sleep 60 | cat` — both children share the bash process
// group, so the timeout kill must take down the grandchild too. Before FIX-320
// this path had no process-group setup and leaked `sleep 60`.
func TestExecuteCommandDirectly_TimeoutKillsPipeline(t *testing.T) {
	a := newTestAgentWithCommandTimeout(2)

	start := time.Now()
	_, err := a.ExecuteCommandDirectly("sleep 60 | cat")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got success")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed < 2*time.Second {
		t.Fatalf("returned too early (%v) — timeout should fire at ~2s", elapsed)
	}

	assertProcessGone(t, "sleep 60")
}

// TestExecuteCommandDirectly_NoTimeoutBackgroundRunsToCompletion verifies that
// with CommandTimeout=0 (no timeout) a background job starts and finishes on
// its own with no interruption from co-shell (UC-0004 for the REPL path).
func TestExecuteCommandDirectly_NoTimeoutBackgroundRunsToCompletion(t *testing.T) {
	a := newTestAgentWithCommandTimeout(0)

	out, err := a.ExecuteCommandDirectly("sleep 2 >/dev/null 2>&1 & echo $!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("cannot parse background PID from output %q: %v", out, err)
	}

	// Immediately after the shell returns, the background sleep is running.
	assertProcessAlive(t, pid, "background sleep (no timeout)")

	// After it naturally finishes, the process must be gone.
	time.Sleep(2500 * time.Millisecond)
	if proc, err := os.FindProcess(pid); err == nil {
		if err := proc.Signal(syscall.Signal(0)); err == nil {
			t.Fatalf("background sleep %d still running after natural completion", pid)
		}
	}
}
