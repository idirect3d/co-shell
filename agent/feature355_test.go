// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
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

package agent

import (
	"context"
	"os"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FEATURE-355: execute_command requires timeout_seconds (0 = wait forever)
// and on_timeout ("kill" / "detach"). Detach mode leaves the process running
// on timeout and returns its PID + log file path.

func TestFEATURE355_TimeoutSecondsRequired(t *testing.T) {
	a := &Agent{}
	_, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":    "echo hi",
		"on_timeout": "kill",
	})
	if err == nil || !strings.Contains(err.Error(), "timeout_seconds") {
		t.Errorf("want timeout_seconds required error, got %v", err)
	}
}

func TestFEATURE355_OnTimeoutRequired(t *testing.T) {
	a := &Agent{}
	_, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": float64(5),
	})
	if err == nil || !strings.Contains(err.Error(), "on_timeout") {
		t.Errorf("want on_timeout required error, got %v", err)
	}
}

func TestFEATURE355_OnTimeoutInvalid(t *testing.T) {
	a := &Agent{}
	_, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": float64(5),
		"on_timeout":      "explode",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid on_timeout") {
		t.Errorf("want invalid on_timeout error, got %v", err)
	}
}

func TestFEATURE355_KillModeNormalCompletion(t *testing.T) {
	a := &Agent{}
	out, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "echo hello-355",
		"timeout_seconds": float64(10),
		"on_timeout":      "kill",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello-355") {
		t.Errorf("output = %q, want it to contain hello-355", out)
	}
}

func TestFEATURE355_ZeroTimeoutWaitsForever(t *testing.T) {
	a := &Agent{}
	start := time.Now()
	out, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "sleep 1; echo done-355",
		"timeout_seconds": float64(0),
		"on_timeout":      "kill",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "done-355") {
		t.Errorf("output = %q, want it to contain done-355", out)
	}
	if time.Since(start) < time.Second {
		t.Errorf("returned too early, 0 timeout should wait for completion")
	}
}

func TestFEATURE355_KillModeTimeoutKills(t *testing.T) {
	a := &Agent{}
	_, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "sleep 30",
		"timeout_seconds": float64(1),
		"on_timeout":      "kill",
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}

func TestFEATURE355_DetachModeKeepsProcessRunning(t *testing.T) {
	a := &Agent{}
	start := time.Now()
	out, err := a.executeSystemCommand(context.Background(), map[string]interface{}{
		"command":         "echo before-sleep-355; sleep 30; echo after-sleep-355",
		"timeout_seconds": float64(1),
		"on_timeout":      "detach",
	})
	if err != nil {
		t.Fatalf("detach should not return error, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("detach took too long: %v", time.Since(start))
	}

	// The result must carry the PID and the log file path.
	pidRe := regexp.MustCompile(`PID: (\d+)`)
	m := pidRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no PID in detach result: %q", out)
	}
	logRe := regexp.MustCompile(`日志文件: (\S+)`)
	lm := logRe.FindStringSubmatch(out)
	if lm == nil {
		t.Fatalf("no log file path in detach result: %q", out)
	}
	logPath := lm[1]

	// Partial output produced before the timeout must be included.
	if !strings.Contains(out, "before-sleep-355") {
		t.Errorf("partial output missing from detach result: %q", out)
	}

	// The detached process must still be running.
	pid := m[1]
	proc, perr := os.FindProcess(atoi(pid))
	if perr != nil {
		t.Fatalf("FindProcess(%s): %v", pid, perr)
	}
	if serr := proc.Signal(syscall.Signal(0)); serr != nil {
		t.Errorf("detached process %s not alive: %v", pid, serr)
	}

	// Clean up: kill the detached process group (PGID = bash PID) and the log.
	syscall.Kill(-atoi(pid), syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	os.Remove(logPath)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
