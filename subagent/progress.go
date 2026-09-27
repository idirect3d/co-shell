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
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// progressTailBytes caps how much of a sub-agent log is scanned when looking
// for its last task_plan event. The tail is enough: the newest snapshot of a
// plan is always written last, and reading the whole log of a long run would
// cost far more than the few kilobytes that matter (FEATURE-564).
const progressTailBytes = 512 * 1024

// progressPollInterval is how often a running sub-agent's log is re-read. Only
// running sub-agents have a watcher, so the cost disappears once they exit.
var progressPollInterval = 700 * time.Millisecond

// PlanProgress is how far a sub-agent got through its own task plan. Total is
// 0 and HasPlan false when the sub-agent never built a plan.
type PlanProgress struct {
	HasPlan bool `json:"has_plan"`
	Done    int  `json:"done"`
	Total   int  `json:"total"`
}

// Row is one line of the sub-agent status list shown below the task-plan panel
// in the Web UI: id, instruction and plan progress (FEATURE-564).
type Row struct {
	ID          string `json:"id"`
	Instruction string `json:"instruction"`
	Status      Status `json:"status"`
	Run         int    `json:"run"`
	Done        int    `json:"done"`
	Total       int    `json:"total"`
	HasPlan     bool   `json:"has_plan"`
}

// Rows renders the status list of one session: only the sub-agents that are
// running right now are listed (FEATURE-565), ordered by pool slot (A1 first).
// A finished sub-agent leaves the list as soon as it exits, so the panel is a
// live view of the pool. Without a task plan the progress is binary: Total is 1
// and Done stays 0 while the sub-agent is working.
func (m *Manager) Rows(sessionID string) []Row {
	out := make([]Row, 0)
	if strings.TrimSpace(sessionID) == "" {
		return out
	}

	m.mu.Lock()
	m.loadLocked()
	rows := make([]*Proc, 0, len(m.procs))
	for _, p := range m.procs {
		if p.SessionID == sessionID && p.Status == StatusRunning {
			cp := *p
			rows = append(rows, &cp)
		}
	}
	m.mu.Unlock()

	// Order by pool slot so the rows read A1, A2, ...
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && poolSlotOf(rows[j].ID) < poolSlotOf(rows[j-1].ID); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}

	for _, p := range rows {
		row := Row{ID: p.ID, Instruction: p.Instruction, Status: p.Status, Run: p.Run}
		if pr := planProgressOf(p.LogPath); pr.HasPlan {
			row.HasPlan, row.Done, row.Total = true, pr.Done, pr.Total
		} else {
			row.Total = 1
		}
		out = append(out, row)
	}
	return out
}

// RowsJSON marshals Rows for the Web UI event payload (nil becomes "[]").
func (m *Manager) RowsJSON(sessionID string) string {
	data, err := json.Marshal(m.Rows(sessionID))
	if err != nil {
		return "[]"
	}
	return string(data)
}

// planProgressOf scans the tail of a sub-agent JSON-Lines log for its last
// task_plan event and counts the steps of that snapshot. Missing logs, logs
// without any task_plan event and unparsable snapshots all yield PlanProgress{}
// (i.e. "no plan"), which the caller renders as a binary 0/1 or 1/1.
func planProgressOf(logPath string) PlanProgress {
	last := ""
	for _, line := range tailLines(logPath, progressTailBytes) {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `"task_plan"`) {
			last = trimmed
		}
	}
	if last == "" {
		return PlanProgress{}
	}
	var ev struct {
		Type string `json:"type"`
		Meta struct {
			Plan string `json:"plan"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(last), &ev); err != nil || ev.Type != "task_plan" {
		return PlanProgress{}
	}
	return countPlanSteps(ev.Meta.Plan)
}

// countPlanSteps counts the steps of a task-plan snapshot. A step counts as
// done as soon as it left "pending" (completed, cancelled or failed) — the
// ratio then means "steps the sub-agent no longer has to work on".
func countPlanSteps(planJSON string) PlanProgress {
	if strings.TrimSpace(planJSON) == "" {
		return PlanProgress{}
	}
	var plan struct {
		Steps []struct {
			Status string `json:"status"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil || len(plan.Steps) == 0 {
		return PlanProgress{}
	}
	done := 0
	for _, step := range plan.Steps {
		if !strings.EqualFold(strings.TrimSpace(step.Status), "pending") {
			done++
		}
	}
	return PlanProgress{HasPlan: true, Done: done, Total: len(plan.Steps)}
}

// tailLines returns the lines of the last maxBytes of a text file. When the
// read starts mid-file the first (partial) line is dropped so callers only see
// complete JSON-Lines records.
func tailLines(path string, maxBytes int) []string {
	if maxBytes <= 0 || strings.TrimSpace(path) == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil
	}
	size := info.Size()
	start := int64(0)
	truncated := false
	if size > int64(maxBytes) {
		start = size - int64(maxBytes)
		truncated = true
	}
	buf := make([]byte, size-start)
	n, err := f.ReadAt(buf, start)
	if n <= 0 {
		return nil
	}
	if err != nil && err != io.EOF {
		return nil
	}
	lines := strings.Split(string(buf[:n]), "\n")
	if truncated && len(lines) > 1 {
		lines = lines[1:]
	}
	return lines
}

// watchProgress re-reads a running sub-agent's log until it exits and fires the
// notifier whenever the rendered progress changes (plan step advance, or the
// final state after the process ended). One goroutine per running sub-agent.
func (m *Manager) watchProgress(logPath, sessionID string, done chan struct{}) {
	ticker := time.NewTicker(progressPollInterval)
	defer ticker.Stop()

	last := progressSignature(PlanProgress{})
	for {
		select {
		case <-done:
			m.notify(sessionID)
			return
		case <-ticker.C:
			sig := progressSignature(planProgressOf(logPath))
			if sig != last {
				last = sig
				m.notify(sessionID)
			}
		}
	}
}

// progressSignature is the comparable form of a plan progress (used to trigger
// a push only when the displayed numbers actually move).
func progressSignature(p PlanProgress) string {
	return strconv.Itoa(p.Done) + "/" + strconv.Itoa(p.Total) + ":" + strconv.FormatBool(p.HasPlan)
}

// ---------- session binding & change notification (FEATURE-564) ----------

// SetSessionID records the session that subsequently launched sub-agents
// belong to. The Web UI status list only renders the current session's rows.
func (m *Manager) SetSessionID(id string) {
	m.mu.Lock()
	m.sessionID = id
	m.mu.Unlock()
}

// SetNotifier registers the callback fired whenever the progress rendered for
// a sub-agent moves (plan step advance, or its final state after exit). The
// callback receives the session the sub-agent is bound to and must not block;
// it is invoked from the sub-agent watcher goroutine.
func (m *Manager) SetNotifier(fn func(sessionID string)) {
	m.mu.Lock()
	m.notifier = fn
	m.mu.Unlock()
}

// notify fires the registered notifier, if any.
func (m *Manager) notify(sessionID string) {
	m.mu.Lock()
	fn := m.notifier
	m.mu.Unlock()
	if fn != nil {
		fn(sessionID)
	}
}

// SetCompletionNotifier registers the callback fired once per sub-agent that
// reaches a natural terminal state (Completed or Failed) — FEATURE-566. The
// callback receives a copy of the final row and must not block: it is invoked
// from the reaper goroutine, after the row has been persisted.
func (m *Manager) SetCompletionNotifier(fn func(Proc)) {
	m.mu.Lock()
	m.completionNotifier = fn
	m.mu.Unlock()
}

// notifyCompletion fires the completion callback, if any. Callers must not
// hold m.mu (the callback may inspect the manager).
func (m *Manager) notifyCompletion(p Proc) {
	m.mu.Lock()
	fn := m.completionNotifier
	m.mu.Unlock()
	if fn != nil {
		fn(p)
	}
}