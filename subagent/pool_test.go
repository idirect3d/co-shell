// Package subagent - per-session pool tests (FEATURE-565): id syntax, the
// 0..10 limit, the per-session process table, running-only listings and slot
// reuse across sessions.
//
// Author: L.Shuang
// Created: 2026-09-27
// MIT License - Copyright (c) 2026 L.Shuang

package subagent

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestPoolHelpers pins the id helpers the agent layer builds its prompts and
// its localized errors from.
func TestPoolHelpers(t *testing.T) {
	if got := PoolID(3); got != "A3" {
		t.Errorf("PoolID(3) = %q, want A3", got)
	}
	ids := PoolIDs(3)
	if len(ids) != 3 || ids[0] != "A1" || ids[2] != "A3" {
		t.Errorf("PoolIDs(3) = %v, want [A1 A2 A3]", ids)
	}
	if got := PoolIDs(0); len(got) != 0 {
		t.Errorf("PoolIDs(0) = %v, want empty", got)
	}

	for in, want := range map[string]struct {
		slot int
		ok   bool
	}{
		"A1":          {1, true},
		"A10":         {10, true},
		"A0":          {0, true},
		"a1":          {0, false},
		"A":           {0, false},
		"A 1":         {0, false},
		"B1":          {0, false},
		"scheduled-3": {0, false},
		"":            {0, false},
	} {
		slot, ok := ParsePoolID(in)
		if ok != want.ok || (ok && slot != want.slot) {
			t.Errorf("ParsePoolID(%q) = (%d, %v), want (%d, %v)", in, slot, ok, want.slot, want.ok)
		}
	}

	if err := ValidateID("A1"); err != nil {
		t.Errorf("ValidateID(A1) = %v, want nil", err)
	}
	for _, bad := range []string{"a1", "A", "A 1", "B1", "scheduled-3", ""} {
		if err := ValidateID(bad); err == nil {
			t.Errorf("ValidateID(%q) = nil, want a format error", bad)
		}
	}

	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "disabled"},
		{1, "A1"},
		{DefaultMaxSubAgents, "A1..A5"},
	} {
		if got := PoolRange(tc.n); got != tc.want {
			t.Errorf("PoolRange(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestValidateMaxSubAgentsBounds pins the legal pool sizes (0 disables
// creation, MaxPoolSize is the hard ceiling).
func TestValidateMaxSubAgentsBounds(t *testing.T) {
	for _, n := range []int{0, 1, DefaultMaxSubAgents, MaxPoolSize} {
		if err := ValidateMaxSubAgents(n); err != nil {
			t.Errorf("ValidateMaxSubAgents(%d) = %v, want nil", n, err)
		}
	}
	for _, n := range []int{-1, MaxPoolSize + 1} {
		if err := ValidateMaxSubAgents(n); err == nil {
			t.Errorf("ValidateMaxSubAgents(%d) = nil, want an error", n)
		}
	}
}

// TestSetMaxSubAgentsRejectsOutOfRange covers the manager setter used by config
// and --max-sub-agents.
func TestSetMaxSubAgentsRejectsOutOfRange(t *testing.T) {
	m := NewManager(Options{Workspace: t.TempDir(), NoStoreLoad: true})
	if got := m.MaxSubAgents(); got != DefaultMaxSubAgents {
		t.Errorf("default pool = %d, want %d", got, DefaultMaxSubAgents)
	}
	if err := m.SetMaxSubAgents(3); err != nil {
		t.Fatalf("SetMaxSubAgents(3) = %v", err)
	}
	if got := m.MaxSubAgents(); got != 3 {
		t.Errorf("pool = %d, want 3", got)
	}
	if err := m.SetMaxSubAgents(MaxPoolSize + 1); err == nil {
		t.Error("SetMaxSubAgents(11) = nil, want an error")
	}
	if got := m.MaxSubAgents(); got != 3 {
		t.Errorf("a rejected size changed the pool to %d, want 3", got)
	}
}

// TestDisabledPoolRefusesCreation covers max_sub_agents = 0.
func TestDisabledPoolRefusesCreation(t *testing.T) {
	m, _ := newTestManager(t, nil)
	if err := m.SetMaxSubAgents(0); err != nil {
		t.Fatalf("SetMaxSubAgents(0) = %v", err)
	}
	_, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, time.Second)
	pe, ok := AsPoolError(err)
	if !ok || pe.Kind != PoolErrDisabled {
		t.Fatalf("Start with the pool disabled = %v, want a PoolErrDisabled", err)
	}
}

// TestConfiguredSizeBoundsTheRange checks the range error carries the size in
// force when --max-sub-agents shrinks the pool.
func TestConfiguredSizeBoundsTheRange(t *testing.T) {
	m, _ := newTestManager(t, nil)
	if err := m.SetMaxSubAgents(2); err != nil {
		t.Fatalf("SetMaxSubAgents(2) = %v", err)
	}
	if err := m.ValidatePoolID(testSessionID, "A2"); err != nil {
		t.Errorf("ValidatePoolID(A2) with max 2 = %v, want nil", err)
	}
	_, err := m.Start(context.Background(), []Task{{ID: "A3", Instruction: "x"}}, time.Second)
	pe, ok := AsPoolError(err)
	if !ok || pe.Kind != PoolErrIDOutOfRange || pe.Max != 2 {
		t.Fatalf("Start(A3) with max 2 = %v, want an out-of-range error carrying max 2", err)
	}
}

// TestSessionsIsolateTheirPools covers the strict per-session isolation: the
// process table is keyed by session + id, so two sessions run their own A1 at
// the same time, in their own workspaces, without blocking or killing each
// other (FEATURE-565).
func TestSessionsIsolateTheirPools(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, ws := newTestManager(t, nil)
	ctx := context.Background()

	m.SetSessionID("sess-a")
	if _, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "a"}}, 100*time.Millisecond); err != nil {
		t.Fatalf("Start(sess-a): %v", err)
	}

	m.SetSessionID("sess-b")
	procs, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "b"}}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Start(sess-b) on the same slot: %v", err)
	}
	if want := filepath.Join(ws, "sub-agents", "sess-b", "A1"); procs[0].Workspace != want {
		t.Errorf("sess-b workspace = %q, want %q", procs[0].Workspace, want)
	}
	if _, ok := m.Lookup("A1"); !ok {
		t.Error("Lookup must find the current session's A1")
	}
	if got := m.SessionRunningIDs("sess-a"); len(got) != 1 || got[0] != "A1" {
		t.Errorf("SessionRunningIDs(sess-a) = %v, want [A1]", got)
	}
	status := m.PoolStatus("sess-b")
	if status.Max != DefaultMaxSubAgents || len(status.Running) != 1 || status.Running[0] != "A1" {
		t.Errorf("PoolStatus(sess-b) = %+v, want A1 running with max %d", status, DefaultMaxSubAgents)
	}
	if rows := m.Rows("sess-a"); len(rows) != 1 || rows[0].ID != "A1" || rows[0].Instruction != "a" {
		t.Errorf("Rows(sess-a) = %+v, want sess-a's own A1", rows)
	}

	// A running slot only blocks its own session.
	if _, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "again"}}, time.Second); err == nil {
		t.Error("Start must refuse a slot that is running in the current session")
	}

	// Killing in one session leaves the other session's A1 running.
	if _, err := m.Kill([]string{"A1"}); err != nil {
		t.Fatalf("Kill(sess-b A1): %v", err)
	}
	if got := m.SessionRunningIDs("sess-a"); len(got) != 1 {
		t.Errorf("SessionRunningIDs(sess-a) after killing sess-b's A1 = %v, want [A1]", got)
	}
	if got := m.SessionRunningIDs("sess-b"); len(got) != 0 {
		t.Errorf("SessionRunningIDs(sess-b) after Kill = %v, want empty", got)
	}
}
