// Author: L.Shuang
// Created: 2026-09-27
// Last Modified: 2026-09-27
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

package subagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeJSONL writes one JSON-Lines log used by the progress tests.
func writeJSONL(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
}

// countPlanSteps counts non-pending steps and the plan size (FEATURE-564).
func TestCountPlanSteps(t *testing.T) {
	cases := []struct {
		name    string
		plan    string
		wantHas bool
		want    PlanProgress
	}{
		{"empty", "", false, PlanProgress{}},
		{"malformed", "{not json", false, PlanProgress{}},
		{"no steps", `{"steps":[]}`, false, PlanProgress{}},
		{
			"mixed",
			`{"steps":[{"status":"completed"},{"status":"in_progress"},{"status":"pending"},{"status":"failed"},{"status":"cancelled"}]}`,
			true,
			PlanProgress{HasPlan: true, Done: 4, Total: 5},
		},
		{"all done", `{"steps":[{"status":"completed"},{"status":"completed"}]}`, true, PlanProgress{HasPlan: true, Done: 2, Total: 2}},
		{"none done", `{"steps":[{"status":"pending"},{"status":"pending"}]}`, true, PlanProgress{HasPlan: true, Done: 0, Total: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countPlanSteps(tc.plan)
			if got != tc.want {
				t.Fatalf("countPlanSteps(%q) = %+v, want %+v", tc.plan, got, tc.want)
			}
			_ = tc.wantHas
		})
	}
}

// planProgressOf uses the last task_plan event of the log tail.
func TestPlanProgressOfUsesLastPlanEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run-1.jsonl")
	writeJSONL(t, path, []string{
		`{"type":"content","text":"hello"}`,
		`{"type":"task_plan","meta":{"plan":"{\"steps\":[{\"status\":\"pending\"},{\"status\":\"pending\"}]}"}}`,
		`{"type":"task_plan","meta":{"plan":"{\"steps\":[{\"status\":\"completed\"},{\"status\":\"pending\"}]}"}}`,
	})
	got := planProgressOf(path)
	if !got.HasPlan || got.Done != 1 || got.Total != 2 {
		t.Fatalf("planProgressOf = %+v, want has plan 1/2", got)
	}

	// A log without any task_plan event reports "no plan".
	plain := filepath.Join(dir, "run-2.jsonl")
	writeJSONL(t, plain, []string{`{"type":"content","text":"no plan here"}`})
	if got := planProgressOf(plain); got.HasPlan {
		t.Fatalf("planProgressOf(no plan) = %+v, want no plan", got)
	}
	if got := planProgressOf(filepath.Join(dir, "missing.jsonl")); got.HasPlan {
		t.Fatalf("planProgressOf(missing) = %+v, want no plan", got)
	}
}

// newRowsTestManager builds a manager with one row per given proc.
func newRowsTestManager(rows ...*Proc) *Manager {
	m := NewManager(Options{NoStoreLoad: true, Workspace: "/tmp/ws"})
	for _, row := range rows {
		m.procs[row.ID] = row
	}
	m.loaded = true
	return m
}

// Rows lists only the current session's sub-agents and falls back to a binary
// 0/1 (or 1/1) when the sub-agent never built a task plan (FEATURE-564).
func TestRowsSessionFilterAndProgressFallback(t *testing.T) {
	dir := t.TempDir()
	planLog := filepath.Join(dir, "planner.jsonl")
	writeJSONL(t, planLog, []string{
		`{"type":"task_plan","meta":{"plan":"{\"steps\":[{\"status\":\"completed\"},{\"status\":\"in_progress\"},{\"status\":\"pending\"}]}"}}`,
	})

	m := newRowsTestManager(
		&Proc{ID: "done", SessionID: "sess-A", Status: StatusCompleted, Instruction: "finished", LogPath: filepath.Join(dir, "none.jsonl")},
		&Proc{ID: "running", SessionID: "sess-A", Status: StatusRunning, Instruction: "still going", LogPath: filepath.Join(dir, "none.jsonl")},
		&Proc{ID: "planner", SessionID: "sess-A", Status: StatusRunning, Instruction: "three steps", LogPath: planLog},
		&Proc{ID: "other", SessionID: "sess-B", Status: StatusRunning, Instruction: "another session", LogPath: filepath.Join(dir, "none.jsonl")},
		&Proc{ID: "legacy", Status: StatusRunning, Instruction: "no session", LogPath: filepath.Join(dir, "none.jsonl")},
	)

	rows := m.Rows("sess-A")
	if len(rows) != 3 {
		t.Fatalf("Rows(sess-A) returned %d rows, want 3: %+v", len(rows), rows)
	}
	byID := map[string]Row{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	if row := byID["done"]; !row.HasPlan && (row.Done != 1 || row.Total != 1) {
		t.Fatalf("finished sub-agent without plan = %+v, want 1/1", row)
	}
	if row := byID["running"]; !row.HasPlan && (row.Done != 0 || row.Total != 1) {
		t.Fatalf("running sub-agent without plan = %+v, want 0/1", row)
	}
	if row := byID["planner"]; !row.HasPlan || row.Done != 2 || row.Total != 3 {
		t.Fatalf("sub-agent with plan = %+v, want 2/3", row)
	}
	if len(m.Rows("sess-B")) != 1 {
		t.Fatalf("Rows(sess-B) = %+v, want the other session's row only", m.Rows("sess-B"))
	}
	if len(m.Rows("")) != 0 {
		t.Fatalf("Rows(\"\") = %+v, want empty", m.Rows(""))
	}
	if got := m.RowsJSON("sess-A"); got == "" || got[0] != '[' {
		t.Fatalf("RowsJSON = %q, want a JSON array", got)
	}
}

// The watcher fires the notifier when the plan progress of a running
// sub-agent moves (B2 refresh, FEATURE-564).
func TestWatchProgressNotifiesOnPlanChange(t *testing.T) {
	oldInterval := progressPollInterval
	progressPollInterval = 10 * time.Millisecond
	defer func() { progressPollInterval = oldInterval }()

	dir := t.TempDir()
	path := filepath.Join(dir, "watch.jsonl")
	writeJSONL(t, path, []string{`{"type":"content","text":"start"}`})

	m := NewManager(Options{NoStoreLoad: true, Workspace: dir})
	notified := make(chan string, 8)
	m.SetNotifier(func(sessionID string) { notified <- sessionID })

	done := make(chan struct{})
	go m.watchProgress(path, "sess-A", done)

	// Give the watcher a moment to start, then advance the plan.
	time.Sleep(30 * time.Millisecond)
	writeJSONL(t, path, []string{
		`{"type":"content","text":"start"}`,
		`{"type":"task_plan","meta":{"plan":"{\"steps\":[{\"status\":\"completed\"},{\"status\":\"pending\"}]}"}}`,
	})

	select {
	case got := <-notified:
		if got != "sess-A" {
			t.Fatalf("notifier session = %q, want sess-A", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not notify after the plan advanced")
	}

	// The process exit signal always pushes once more and stops the watcher.
	close(done)
	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not notify on process exit")
	}
}
