// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// FEATURE-566: a sub-agent that reaches a natural terminal state — success or
// failure — must report itself to the agent that launched it. These tests pin
// the sub-agent side of the contract: the notifier fires for a finished process
// and never for one the caller killed.

package subagent

import (
	"context"
	"sync"
	"testing"
	"time"
)

// notifyRecorder collects the rows the completion notifier reports.
type notifyRecorder struct {
	mu    sync.Mutex
	procs []Proc
}

func (r *notifyRecorder) add(p Proc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.procs = append(r.procs, p)
}

func (r *notifyRecorder) all() []Proc {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Proc(nil), r.procs...)
}

// waitNotifications polls until the recorder holds at least want notifications.
func waitNotifications(t *testing.T, r *notifyRecorder, want int, timeout time.Duration) []Proc {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := r.all()
		if len(got) >= want {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("completion notifications = %d, want %d", len(got), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCompletionNotifierReportsFinishedProcess covers UC-0001: a sub-agent that
// finished on its own is reported once, with the row it ended in.
func TestCompletionNotifierReportsFinishedProcess(t *testing.T) {
	m, _ := newTestManager(t, nil)
	rec := &notifyRecorder{}
	m.SetCompletionNotifier(rec.add)

	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "work"}}, 10*time.Second); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := waitNotifications(t, rec, 1, 10*time.Second)
	p := got[0]
	if p.ID != "A1" || p.SessionID != testSessionID {
		t.Errorf("notified row = %s/%s, want A1/%s", p.SessionID, p.ID, testSessionID)
	}
	if p.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", p.Status)
	}
	if p.EndedAt.IsZero() {
		t.Error("the reported row carries no end time")
	}
	if got := waitNotifications(t, rec, 1, 50*time.Millisecond); len(got) != 1 {
		t.Errorf("notifications = %d, want exactly 1", len(got))
	}
}

// TestCompletionNotifierReportsFailure covers UC-0002: a failing sub-agent is
// reported too, carrying its exit code.
func TestCompletionNotifierReportsFailure(t *testing.T) {
	t.Setenv("FAKE_EXIT", "3")
	m, _ := newTestManager(t, nil)
	rec := &notifyRecorder{}
	m.SetCompletionNotifier(rec.add)

	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "fail"}}, 10*time.Second); err != nil {
		t.Fatalf("Start: %v", err)
	}

	p := waitNotifications(t, rec, 1, 10*time.Second)[0]
	if p.Status != StatusFailed || p.ExitCode != 3 {
		t.Errorf("notified row = %s exit %d, want failed exit 3", p.Status, p.ExitCode)
	}
}

// TestCompletionNotifierSkipsKilledProcess covers UC-0006: the caller asked for
// the termination and already has the result, so a killed sub-agent is silent.
func TestCompletionNotifierSkipsKilledProcess(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)
	rec := &notifyRecorder{}
	m.SetCompletionNotifier(rec.add)

	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "loop"}}, 100*time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := m.Kill([]string{"A1"}); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	if got := rec.all(); len(got) != 0 {
		t.Errorf("killed sub-agent was notified: %+v", got)
	}
}
