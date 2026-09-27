// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// FEATURE-566: a sub-agent that finishes on its own must reach the agent that
// launched it. These tests pin the agent-side dispatch: mid-turn it is injected
// as a dynamic event, idle it starts a fresh turn through the Web UI, and a
// surface that cannot start a turn degrades to a printed, buffered notice.

package agent

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/subagent"
)

// notifyTestIO is a UserIO that records the text printed to it. It cannot start
// a turn by itself, so it exercises the degraded path.
type notifyTestIO struct {
	mu      sync.Mutex
	printed []string
}

func (o *notifyTestIO) Print(args ...interface{})   {}
func (o *notifyTestIO) Println(args ...interface{}) {}
func (o *notifyTestIO) ReadLine() (string, error)   { return "", io.EOF }
func (o *notifyTestIO) ReadKey() (byte, error)      { return '\n', nil }
func (o *notifyTestIO) IsReading() bool             { return false }

func (o *notifyTestIO) ErrPrintf(format string, args ...interface{}) {}

func (o *notifyTestIO) Printf(format string, args ...interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.printed = append(o.printed, fmt.Sprintf(format, args...))
}

func (o *notifyTestIO) printedAll() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.printed...)
}

// notifyStarterIO is the Web-UI-like IO: it can start a turn itself.
type notifyStarterIO struct {
	notifyTestIO
	mu      sync.Mutex
	started []string
	refuse  bool
}

func (o *notifyStarterIO) StartSubAgentCompletionTurn(sessionID, notice string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.refuse {
		return false
	}
	o.started = append(o.started, notice)
	return true
}

func (o *notifyStarterIO) startedAll() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.started...)
}

// newSubAgentNotifyTestAgent builds the minimal agent the dispatch needs: a
// sub-agent manager and a current session.
func newSubAgentNotifyTestAgent(t *testing.T) *Agent {
	t.Helper()
	a := &Agent{subAgentMgr: subagent.NewManager(subagent.Options{
		Workspace:   t.TempDir(),
		NoStoreLoad: true,
	})}
	a.SetCurrentSessionID("sess-1")
	t.Cleanup(func() { a.subAgentMgr.Close() })
	return a
}

// doneProc is a sub-agent row that reached a terminal state.
func doneProc(status subagent.Status, exitCode int) subagent.Proc {
	return subagent.Proc{
		ID:        "A1",
		SessionID: "sess-1",
		Status:    status,
		Run:       2,
		ExitCode:  exitCode,
		StartedAt: time.Now().Add(-12 * time.Second),
		EndedAt:   time.Now(),
	}
}

// TestOnSubAgentCompletedBusyInjectsDynamicEvent covers UC-0003: mid-turn the
// notice joins the running iteration as a dynamic event instead of starting a
// second one.
func TestOnSubAgentCompletedBusyInjectsDynamicEvent(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)
	io := &notifyTestIO{}
	a.SetIO(io)
	a.SetBusy(true)
	defer a.SetBusy(false)

	a.onSubAgentCompleted(doneProc(subagent.StatusCompleted, 0))

	if got := io.printedAll(); len(got) != 0 {
		t.Errorf("busy agent printed %q, want nothing", got)
	}
	block := a.consumeDynamicEvents(true)
	if !strings.Contains(block, `sub_agent_done id="A1" run="2" status="completed"`) {
		t.Errorf("dynamic event block = %q, want the completion notice", block)
	}
}

// TestOnSubAgentCompletedIdleStartsTurn covers UC-0001/UC-0009: an idle agent
// hands the notice to a surface that starts a turn, and does not also buffer it.
func TestOnSubAgentCompletedIdleStartsTurn(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)
	io := &notifyStarterIO{}
	a.SetIO(io)

	a.onSubAgentCompleted(doneProc(subagent.StatusFailed, 3))

	started := io.startedAll()
	if len(started) != 1 {
		t.Fatalf("started turns = %d, want 1", len(started))
	}
	if !strings.Contains(started[0], `status="failed" exit_code="3"`) {
		t.Errorf("started notice = %q, want the failed status", started[0])
	}
	if block := a.consumeDynamicEvents(true); block != "" {
		t.Errorf("dynamic events = %q, want none when a turn was started", block)
	}
}

// TestOnSubAgentCompletedIdleDegradesWithoutTurnStarter covers UC-0011: the
// terminal surface prints the notice and buffers it for the next turn.
func TestOnSubAgentCompletedIdleDegradesWithoutTurnStarter(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)
	io := &notifyTestIO{}
	a.SetIO(io)

	a.onSubAgentCompleted(doneProc(subagent.StatusCompleted, 0))

	printed := io.printedAll()
	if len(printed) != 1 || !strings.Contains(printed[0], `id="A1"`) {
		t.Errorf("printed = %q, want the completion notice", printed)
	}
	if block := a.consumeDynamicEvents(true); !strings.Contains(block, `id="A1"`) {
		t.Errorf("dynamic events = %q, want the buffered notice", block)
	}
}

// TestOnSubAgentCompletedRefusedBySessionBuffers covers the hand-off failure:
// when the surface refuses the notice the agent keeps it in the buffer.
func TestOnSubAgentCompletedRefusedBySessionBuffers(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)
	io := &notifyStarterIO{refuse: true}
	a.SetIO(io)

	a.onSubAgentCompleted(doneProc(subagent.StatusCompleted, 0))

	if len(io.startedAll()) != 0 {
		t.Error("a refused notice must not be recorded as a started turn")
	}
	if block := a.consumeDynamicEvents(true); !strings.Contains(block, `id="A1"`) {
		t.Errorf("dynamic events = %q, want the buffered notice", block)
	}
}

// TestOnSubAgentCompletedDropsForeignSessionAndRunningRows covers UC-0008 and
// the "only terminal states are reported" rule.
func TestOnSubAgentCompletedDropsForeignSessionAndRunningRows(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)
	io := &notifyStarterIO{}
	a.SetIO(io)

	foreign := doneProc(subagent.StatusCompleted, 0)
	foreign.SessionID = "sess-other"
	a.onSubAgentCompleted(foreign)

	noSession := doneProc(subagent.StatusCompleted, 0)
	noSession.SessionID = ""
	a.onSubAgentCompleted(noSession)

	running := doneProc(subagent.StatusKilled, 0)
	a.onSubAgentCompleted(running)

	if len(io.startedAll()) != 0 {
		t.Errorf("started turns = %v, want none", io.startedAll())
	}
	if block := a.consumeDynamicEvents(true); block != "" {
		t.Errorf("dynamic events = %q, want none", block)
	}
}

// TestSubAgentDoneNoticeContent covers UC-0005: the notice carries the status
// facts and the pointer to the status tool, nothing else.
func TestSubAgentDoneNoticeContent(t *testing.T) {
	a := newSubAgentNotifyTestAgent(t)

	notice := a.subAgentDoneNotice(doneProc(subagent.StatusCompleted, 0))
	for _, want := range []string{
		`<sub_agent_done id="A1" run="2" status="completed" exit_code="0" elapsed="12s"`,
		"</sub_agent_done>",
		"sub_agent_status",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice = %q, want it to contain %q", notice, want)
		}
	}
	if strings.Contains(notice, "final answer") {
		t.Errorf("notice = %q, want no result body", notice)
	}
}
