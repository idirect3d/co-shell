// FIX-585: a tool timeout must not be reported as a user interrupt.
//
// FEATURE-582 routes both a user interrupt (InterruptTool) and the tool timeout
// (context.WithTimeout in executeToolCall) through the same cancelled context.
// Before this fix a plain timeout was therefore reported as "interrupted by
// user" and made the run loop discard the whole round — the user saw the turn
// end although they never pressed ESC. These tests pin the two paths apart.
//
// MIT License - Copyright (c) 2026 L.Shuang

//go:build !windows

package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// newFIX585Agent returns a minimal agent wired with a process registry so the
// command tools can register and kill process groups.
func newFIX585Agent() *Agent {
	a := newTestAgentWithCommandTimeout(0)
	a.procReg = newProcRegistry()
	return a
}

// TestFIX585TimeoutIsNotUserInterrupt: when the tool context hits its deadline
// (the tool timeout), execute_command must report a timeout error and must not
// claim a user interrupt, so the round survives.
func TestFIX585TimeoutIsNotUserInterrupt(t *testing.T) {
	a := newFIX585Agent()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := a.executeSystemCommand(ctx, map[string]interface{}{
		"command":         "sleep 5",
		"timeout_seconds": float64(1),
		"on_timeout":      "kill",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if errors.Is(err, ErrToolInterrupted) {
		t.Fatalf("tool timeout reported as a user interrupt: %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
	if a.IsInterrupted() {
		t.Fatal("a tool timeout must not flag the agent as interrupted")
	}
	if elapsed > 4*time.Second {
		t.Fatalf("command was not killed at the deadline (took %v)", elapsed)
	}
}

// TestFIX585UserInterruptIsStillReported: a real interrupt (ESC / pause button /
// Ctrl+C) keeps the FEATURE-582 behaviour — the tool fails with
// ErrToolInterrupted and the interrupt reason is preserved.
func TestFIX585UserInterruptIsStillReported(t *testing.T) {
	a := newFIX585Agent()
	a.ResetCancel()

	done := make(chan error, 1)
	go func() {
		_, err := a.executeSystemCommand(a.CancelContext(), map[string]interface{}{
			"command":         "sleep 30",
			"timeout_seconds": float64(60),
			"on_timeout":      "kill",
		})
		done <- err
	}()

	time.Sleep(300 * time.Millisecond)
	a.InterruptTool("web-interrupt")

	select {
	case err := <-done:
		if !errors.Is(err, ErrToolInterrupted) {
			t.Fatalf("expected ErrToolInterrupted for a user interrupt, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the user interrupt did not terminate the command")
	}
	if got := a.InterruptReason(); got != "web-interrupt" {
		t.Fatalf("interrupt reason = %q, want %q", got, "web-interrupt")
	}
}

// TestFIX585IsTimeoutCancel: the discriminator itself — a deadline context is a
// timeout, an explicitly cancelled context is not.
func TestFIX585IsTimeoutCancel(t *testing.T) {
	deadlineCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	<-deadlineCtx.Done()
	if !isTimeoutCancel(deadlineCtx) {
		t.Error("a deadline context must be reported as a timeout")
	}

	cancelCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if isTimeoutCancel(cancelCtx) {
		t.Error("an explicit cancel must not be reported as a timeout")
	}
	if isTimeoutCancel(nil) {
		t.Error("a nil context must not be reported as a timeout")
	}
}
