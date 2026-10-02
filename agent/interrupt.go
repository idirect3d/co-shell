// Tool-call interrupt entry point (FEATURE-582).
//
// Unifies the three user exits — Web ESC / pause button, TUI ESC / Ctrl+C and
// SIGINT — into one operation: flag the interrupt, cancel the tool context,
// terminate every process group a tool started, and wake the run loop so it
// returns to the prompt immediately instead of waiting for the tool.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// ErrToolInterrupted marks a tool result that was cut short because the user
// interrupted the call (ESC / pause button / Ctrl+C). It is distinct from a
// timeout or a generic failure so callers can pick the right notice.
var ErrToolInterrupted = errors.New("tool interrupted by user")

// interruptedToolError carries the tool name and interrupt reason while still
// matching ErrToolInterrupted via errors.Is.
type interruptedToolError struct {
	Tool   string
	Reason string
}

func (e *interruptedToolError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("tool %s interrupted: %v", e.Tool, ErrToolInterrupted)
	}
	return fmt.Sprintf("tool %s interrupted (%s): %v", e.Tool, e.Reason, ErrToolInterrupted)
}

// Is makes errors.Is(err, ErrToolInterrupted) return true.
func (e *interruptedToolError) Is(target error) bool { return target == ErrToolInterrupted }

// newToolInterruptedError builds the typed interrupt error for a tool.
func newToolInterruptedError(tool, reason string) error {
	return &interruptedToolError{Tool: tool, Reason: reason}
}

// interruptWatchdogDelay is how long after an interrupt the watchdog waits
// before warning about tools that still have not returned (§8 of the design:
// a tool that ignores its context keeps running as a leaked goroutine, which
// must be observable rather than silently stuck).
const interruptWatchdogDelay = 5 * time.Second

// InterruptTool aborts every running tool of the current turn. It is safe to
// call repeatedly: only the first call takes effect until the next turn resets
// the state (ResetCancel).
//
// Steps:
//  1. flag the interrupt (so RunStream can tell it apart from a tool failure);
//  2. cancel cancelCtx, waking every tool that selects on ctx.Done();
//  3. terminate all registered child process groups (SIGTERM → SIGKILL);
//  4. signal the cancel channel so the run loop returns to the prompt;
//  5. arm a watchdog that warns if a tool is still running 5s later.
func (a *Agent) InterruptTool(reason string) {
	if a == nil {
		return
	}

	a.mu.Lock()
	if a.interrupted.Load() {
		a.mu.Unlock()
		log.Debug("Agent.InterruptTool: already interrupted, ignoring duplicate (reason=%s)", reason)
		return
	}
	a.interrupted.Store(true)
	a.interruptReason = reason
	cancelFunc := a.cancelFunc
	a.cancelFunc = nil
	select {
	case a.cancelCh <- struct{}{}:
	default:
	}
	a.mu.Unlock()

	if cancelFunc != nil {
		cancelFunc()
	}

	killed := a.procReg.KillAll(reason)
	// FEATURE-582 Q6-B: the persistent shell session cannot survive its
	// process group being killed; reset it so shell_start can rebuild it.
	a.markShellSessionTerminated(killed)
	if len(killed) > 0 {
		a.defaultIO().Println(i18n.TF(i18n.KeyInterruptToolKilled, len(killed), FormatProcEntries(killed)))
	}
	log.Warn("Agent.InterruptTool: interrupted (reason=%s, killed_process_groups=%d)", reason, len(killed))

	a.startInterruptWatchdog(reason)
}

// IsInterrupted reports whether the current turn has been interrupted by the user.
func (a *Agent) IsInterrupted() bool {
	if a == nil {
		return false
	}
	return a.interrupted.Load()
}

// InterruptReason returns the reason of the current interrupt (empty when none).
func (a *Agent) InterruptReason() string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interruptReason
}

// ClearInterrupt clears the interrupt flag (called at the start of a new turn).
func (a *Agent) ClearInterrupt() {
	if a == nil {
		return
	}
	a.interrupted.Store(false)
	a.mu.Lock()
	a.interruptReason = ""
	a.mu.Unlock()
}

// trackToolInFlight records a tool execution that is currently running and
// returns a function that must be called once it returns.
func (a *Agent) trackToolInFlight(tool string) func() {
	id := atomic.AddInt64(&a.inFlightSeq, 1)
	a.mu.Lock()
	if a.inFlight == nil {
		a.inFlight = make(map[int64]string)
	}
	a.inFlight[id] = tool
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		delete(a.inFlight, id)
		a.mu.Unlock()
	}
}

// startInterruptWatchdog warns about tools that are still running after an
// interrupt and re-issues the process-group kill as a fallback.
func (a *Agent) startInterruptWatchdog(reason string) {
	go func() {
		time.Sleep(interruptWatchdogDelay)
		a.mu.Lock()
		names := make([]string, 0, len(a.inFlight))
		for _, n := range a.inFlight {
			names = append(names, n)
		}
		a.mu.Unlock()
		if len(names) == 0 {
			return
		}
		log.Warn("Agent.InterruptTool: %d tool(s) still running %s after interrupt (reason=%s): %v — re-issuing process-group kill",
			len(names), interruptWatchdogDelay, reason, names)
		a.procReg.KillAll("interrupt-watchdog")
	}()
}

// IsToolRunning reports whether a tool call is currently executing. The Web
// pause button, the TUI ESC/Ctrl+C consumer and the SIGINT handler use it to
// choose between "interrupt the tool" and the legacy stream-pause/exit paths
// (FEATURE-582).
func (a *Agent) IsToolRunning() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.inFlight) > 0
}

// inFlightTools returns the names of tools currently executing.
func (a *Agent) inFlightTools() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	names := make([]string, 0, len(a.inFlight))
	for _, n := range a.inFlight {
		names = append(names, n)
	}
	return names
}
