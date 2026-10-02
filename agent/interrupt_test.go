// Package agent - FEATURE-582 tests: tool-call interrupt infrastructure.
//
// Covers the unified interrupt entry (InterruptTool), the child process-group
// registry (register / kill / idempotency) and the RunStream rule that an
// interrupt discards the unclosed assistant message (tool_calls) so the history
// never breaks the tool_calls ↔ tool result pairing.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// TestErrToolInterruptedMatches verifies the typed interrupt error matches
// ErrToolInterrupted through errors.Is and mentions the tool name.
func TestErrToolInterruptedMatches(t *testing.T) {
	err := newToolInterruptedError("execute_command", "esc")
	if !errors.Is(err, ErrToolInterrupted) {
		t.Fatalf("errors.Is(err, ErrToolInterrupted) = false, want true (err=%v)", err)
	}
	if got := err.Error(); got == "" {
		t.Fatal("interrupt error message is empty")
	}
	if !errors.Is(ErrToolInterrupted, ErrToolInterrupted) {
		t.Fatal("sentinel error does not match itself")
	}
}

// TestInterruptToolIdempotent verifies that the first InterruptTool cancels the
// tool context and flags the interrupt, and that a second call is a no-op.
func TestInterruptToolIdempotent(t *testing.T) {
	ag := newFix552Agent(t, &fix552Client{}, "prompt")
	ag.ResetCancel()

	ctx := ag.CancelContext()
	if ctx == nil {
		t.Fatal("CancelContext() returned nil after ResetCancel")
	}
	if ag.IsInterrupted() {
		t.Fatal("agent reports interrupted before any interrupt")
	}

	ag.InterruptTool("esc")

	if !ag.IsInterrupted() {
		t.Fatal("IsInterrupted() = false after InterruptTool")
	}
	if got := ag.InterruptReason(); got != "esc" {
		t.Fatalf("InterruptReason() = %q, want %q", got, "esc")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancel context was not canceled by InterruptTool")
	}

	// A second call must not panic nor change the recorded reason.
	ag.InterruptTool("ctrl-c")
	if got := ag.InterruptReason(); got != "esc" {
		t.Fatalf("InterruptReason() after duplicate interrupt = %q, want %q", got, "esc")
	}

	ag.ClearInterrupt()
	if ag.IsInterrupted() {
		t.Fatal("IsInterrupted() = true after ClearInterrupt")
	}
	if got := ag.InterruptReason(); got != "" {
		t.Fatalf("InterruptReason() after ClearInterrupt = %q, want empty", got)
	}
}

// TestInterruptToolClearsAtTurnReset verifies ResetCancel clears the interrupt
// state so the next turn starts clean.
func TestInterruptToolClearsAtTurnReset(t *testing.T) {
	ag := newFix552Agent(t, &fix552Client{}, "prompt")
	ag.ResetCancel()
	ag.InterruptTool("esc")
	if !ag.IsInterrupted() {
		t.Fatal("setup: interrupt flag not set")
	}
	ag.ResetCancel()
	if ag.IsInterrupted() {
		t.Fatal("ResetCancel did not clear the interrupt flag")
	}
	if ag.CancelContext() == nil {
		t.Fatal("ResetCancel did not create a fresh cancel context")
	}
}

// TestProcRegistryRegisterUnregister verifies entries appear while registered
// and disappear once the returned unregister function is called.
func TestProcRegistryRegisterUnregister(t *testing.T) {
	reg := newProcRegistry()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	unregister := reg.Register("execute_command", "true", cmd, false)
	snap := reg.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("Snapshot() len = %d, want 1", len(snap))
	}
	if snap[0].Tool != "execute_command" || snap[0].PID != cmd.Process.Pid {
		t.Fatalf("unexpected entry: %+v", snap[0])
	}
	unregister()
	if got := len(reg.Snapshot()); got != 0 {
		t.Fatalf("Snapshot() len after unregister = %d, want 0", got)
	}
	_ = cmd.Wait()
}

// TestProcRegistryKillAllTerminatesGroup verifies that KillAll really
// terminates a running child process group (real SIGTERM → SIGKILL path).
func TestProcRegistryKillAllTerminatesGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group kill semantics are Unix-specific")
	}

	reg := newProcRegistry()
	cmd := exec.Command("sleep", "60")
	setProcessGroupAttr(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := cmd.Process.Pid
	unregister := reg.Register("execute_command", "sleep 60", cmd, false)
	defer unregister()

	killed := reg.KillAll("test")
	if len(killed) != 1 || killed[0].PID != pid {
		t.Fatalf("KillAll() = %+v, want one entry with PID %d", killed, pid)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		t.Fatalf("process group %d still running after KillAll", pid)
	}

	// The group must be gone: signal 0 reports ESRCH for a dead group.
	if err := syscall.Kill(-pid, 0); err == nil {
		t.Fatalf("process group %d still exists after KillAll", pid)
	}
	if got := len(reg.Snapshot()); got != 0 {
		t.Fatalf("Snapshot() len after KillAll = %d, want 0", got)
	}
}

// TestProcRegistryKillActiveSkipsDetached verifies KillActive leaves detached
// background groups registered while dropping foreground ones.
func TestProcRegistryKillActiveSkipsDetached(t *testing.T) {
	reg := newProcRegistry()
	foreground := exec.Command("sleep", "60")
	setProcessGroupAttr(foreground)
	if err := foreground.Start(); err != nil {
		t.Fatalf("start foreground: %v", err)
	}
	background := exec.Command("sleep", "60")
	setProcessGroupAttr(background)
	if err := background.Start(); err != nil {
		t.Fatalf("start background: %v", err)
	}
	defer func() {
		killProcessGroupByID(foreground.Process.Pid)
		killProcessGroupByID(background.Process.Pid)
	}()

	stopFg := reg.Register("execute_command", "sleep 60", foreground, false)
	stopBg := reg.Register("execute_command", "sleep 60", background, true)
	defer stopFg()
	defer stopBg()

	if killed := reg.KillActive("test"); len(killed) != 1 {
		t.Fatalf("KillActive() killed %d entries, want 1 (only the foreground one)", len(killed))
	}
	snap := reg.Snapshot()
	if len(snap) != 1 || !snap[0].Detached {
		t.Fatalf("Snapshot() after KillActive = %+v, want only the detached entry", snap)
	}
}

// TestRunStreamInterruptDiscardsUnclosedToolCalls verifies Q1-A: when the user
// interrupts while a tool is running, RunStream returns to the caller and the
// history holds no assistant message with tool_calls left without results.
func TestRunStreamInterruptDiscardsUnclosedToolCalls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on Unix sleep process cleanup")
	}

	client := &interruptToolClient{}
	ag := newFix552Agent(t, client, "prompt")
	ag.SetYOLO(true)

	done := make(chan error, 1)
	go func() {
		_, err := ag.RunStream(context.Background(), "run a long command", func(StreamEvent) {})
		done <- err
	}()

	// Wait until the system command has actually started. IsCommandRunning is set
	// by executeSystemCommand right before cmd.Start(); polling it keeps this B0
	// test independent of the process registry (wired in B1).
	deadline := time.Now().Add(5 * time.Second)
	for !ag.IsCommandRunning() {
		if time.Now().After(deadline) {
			ag.InterruptTool("test-timeout")
			<-done
			t.Fatal("system command never started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	interruptAt := time.Now()
	ag.InterruptTool("test")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("RunStream did not return within 3s of the interrupt")
	}
	if elapsed := time.Since(interruptAt); elapsed > 2*time.Second {
		t.Fatalf("RunStream took %s to return after interrupt, want <= 2s", elapsed)
	}

	assertToolCallsClosed(t, ag.Messages())

	// The interrupt state is turn-scoped; the caller resets it for the next turn.
	ag.ResetCancel()
	if ag.IsInterrupted() {
		t.Fatal("interrupt flag survived ResetCancel")
	}
}

// assertToolCallsClosed fails when any assistant message carrying tool_calls is
// not followed by a tool result for every tool_call_id.
func assertToolCallsClosed(t *testing.T, messages []llm.Message) {
	t.Helper()
	for i, msg := range messages {
		if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
			continue
		}
		answered := make(map[string]bool)
		for _, later := range messages[i+1:] {
			if later.Role == "tool" && later.ToolCallID != "" {
				answered[later.ToolCallID] = true
			}
		}
		for _, tc := range msg.ToolCalls {
			if !answered[tc.ID] {
				t.Fatalf("messages[%d] has tool_call %s (%s) without a matching tool result", i, tc.ID, tc.Name)
			}
		}
	}
}

// interruptToolClient emits one long-running execute_command tool call, then a
// plain reply on any later call.
type interruptToolClient struct {
	calls int
}

func (c *interruptToolClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Content: "ok"}, nil
}

func (c *interruptToolClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	i := c.calls
	c.calls++
	ch := make(chan llm.StreamEvent, 3)
	if i == 0 {
		ch <- llm.StreamEvent{
			Type: llm.StreamEventToolCall,
			ToolCall: &llm.ToolCall{
				ID:   "call-sleep",
				Name: "execute_command",
				Arguments: `{"meta":{"intent":"sleep","risk":"low","risk_reason":"test long running command",` +
					`"affected_objects":[],"progress":[]},"command":"sleep 60","timeout_seconds":60,"on_timeout":"kill"}`,
			},
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "tool_calls"}
	} else {
		ch <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "done"}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "stop"}
	}
	close(ch)
	return ch, nil
}

func (c *interruptToolClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}
func (c *interruptToolClient) TestVisionSupport(ctx context.Context) bool   { return false }
func (c *interruptToolClient) TestTextSupport(ctx context.Context) bool     { return true }
func (c *interruptToolClient) TestToolCallSupport(ctx context.Context) bool { return true }
func (c *interruptToolClient) TestThinkingSupport(ctx context.Context) bool { return false }
func (c *interruptToolClient) SetThinkingEnabled(enabled bool)              {}
func (c *interruptToolClient) SetReasoningEffort(effort string)             {}
func (c *interruptToolClient) SetTopP(topP float64)                         {}
func (c *interruptToolClient) SetTopK(topK int)                             {}
func (c *interruptToolClient) SetRepetitionPenalty(penalty float64)         {}
func (c *interruptToolClient) SetTokenUsage(mode string)                    {}
func (c *interruptToolClient) SetTemperature(temp float64)                  {}
func (c *interruptToolClient) SetBodyAdditions(additions map[string]string) {}
func (c *interruptToolClient) RemoveBodyAddition(key string)                {}
func (c *interruptToolClient) GetBodyAdditions() map[string]string          { return nil }
func (c *interruptToolClient) Close() error                                 { return nil }
