// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// FEATURE-566 scenario: the main agent's iteration ends FIRST (the launch window
// expires while both sub-agents keep running in the background), and the two
// sub-agents finish AFTERWARDS — A1 first, A2 later. This pins the two dispatch
// branches end to end on real child processes:
//
//	1. A1 finishes while the agent is idle → a new turn is started with the
//	   notice as the user message (UC-0001).
//	2. A2 finishes while that new turn is running → it joins the running turn as
//	   a dynamic event (UC-0003), in completion order (UC-0007).

package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/subagent"
)

// lateCompletionScript is the child fixture: A1 finishes after 300ms, A2 after
// 700ms, so both outlive the launch window of the main iteration.
const lateCompletionScript = `#!/bin/sh
case "$CO_SHELL_SUB_AGENT_ID" in
  A1) sleep 0.3 ;;
  A2) sleep 0.7 ;;
esac
echo '{"type":"content_chunk","text":"final answer"}'
exit ${FAKE_EXIT:-0}
`

// writeLateCompletionFixture writes the child fixture and returns its path.
func writeLateCompletionFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture requires a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "late-co-shell")
	if err := os.WriteFile(path, []byte(lateCompletionScript), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// lateCompletionIO is the Web-UI-like surface: it starts a turn for the first
// notice and then stays "in a turn" (busy) the way a real run does, so the
// second notice has to take the dynamic-event path.
type lateCompletionIO struct {
	notifyTestIO
	agent *Agent

	mu      sync.Mutex
	started []string
}

func (o *lateCompletionIO) StartSubAgentCompletionTurn(sessionID, notice string) bool {
	o.mu.Lock()
	o.started = append(o.started, notice)
	o.mu.Unlock()
	// A started turn makes the agent busy until it ends; keeping it busy models
	// the new iteration still running when the later sub-agent finishes.
	o.agent.SetBusy(true)
	return true
}

func (o *lateCompletionIO) startedAll() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.started...)
}

// TestSubAgentLateCompletionAfterTurnEnd covers UC-0001/0003/0007 in the order
// the user asked for: the iteration ends first, then both sub-agents report.
func TestSubAgentLateCompletionAfterTurnEnd(t *testing.T) {
	session := "sess-late"
	execPath := writeLateCompletionFixture(t)
	mgr := subagent.NewManager(subagent.Options{
		Workspace:   t.TempDir(),
		ExecPath:    execPath,
		NoStoreLoad: true,
	})
	mgr.SetSessionID(session)
	t.Cleanup(mgr.Close)

	a := &Agent{subAgentMgr: mgr}
	a.SetCurrentSessionID(session)
	io := &lateCompletionIO{agent: a}
	a.SetIO(io)

	// Count the manager's reports and keep the manager's callback wired exactly
	// like New does, so the assertion sees the same dispatch as production.
	var mu sync.Mutex
	var fired []subagent.Proc
	mgr.SetCompletionNotifier(func(p subagent.Proc) {
		mu.Lock()
		fired = append(fired, p)
		mu.Unlock()
		a.onSubAgentCompleted(p)
	})

	// The main iteration launches both sub-agents and ends while they run: the
	// 50ms window expires, so the returned rows are marked as background work.
	procs, err := mgr.Start(context.Background(), []subagent.Task{
		{ID: "A1", Instruction: "slow"},
		{ID: "A2", Instruction: "slower"},
	}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for _, p := range procs {
		if !p.WaitTimedOut || p.Status != subagent.StatusRunning {
			t.Fatalf("sub-agent %s = %s wait_timed_out=%v, want running in the background",
				p.ID, p.Status, p.WaitTimedOut)
		}
	}
	if a.IsBusy() {
		t.Fatal("the main iteration should have ended (agent idle) before the sub-agents finish")
	}

	// Wait for both late completions to be reported, in order.
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		n := len(fired)
		mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("completion notices = %d, want 2", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	order := append([]subagent.Proc(nil), fired...)
	mu.Unlock()
	if order[0].ID != "A1" || order[1].ID != "A2" {
		t.Fatalf("notice order = %s, %s; want A1 then A2 (completion order)", order[0].ID, order[1].ID)
	}

	// 1. A1 arrived while the agent was idle → it started a new turn.
	started := io.startedAll()
	if len(started) != 1 {
		t.Fatalf("started turns = %d, want exactly 1 (the first late completion)", len(started))
	}
	for _, want := range []string{`id="A1"`, `run="1"`, `status="completed"`, `exit_code="0"`, "sub_agent_status"} {
		if !strings.Contains(started[0], want) {
			t.Errorf("started notice = %q, want it to contain %q", started[0], want)
		}
	}

	// 2. A2 arrived while the new turn was running → it joined that turn as a
	// dynamic event instead of starting a second turn.
	if len(io.startedAll()) != 1 {
		t.Errorf("started turns = %d after A2, want 1", len(io.startedAll()))
	}
	block := a.consumeDynamicEvents(true)
	if !strings.Contains(block, `<sub_agent_done id="A2" run="1" status="completed"`) {
		t.Errorf("dynamic events = %q, want A2's notice injected into the running turn", block)
	}
	if strings.Contains(block, `id="A1"`) {
		t.Errorf("dynamic events = %q, want no duplicate of A1's notice", block)
	}
	if strings.Contains(block, "final answer") {
		t.Errorf("dynamic events = %q, want no result body (UC-0005)", block)
	}
}
