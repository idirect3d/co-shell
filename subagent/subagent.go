// Author: L.Shuang
// Created: 2026-04-26
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

// Package subagent launches and manages co-shell sub-agent processes
// (FEATURE-563).
//
// A sub-agent is an independent co-shell process running in its own workspace
// directory ({parent-workspace}/sub-agents/{id}/) in unattended mode. The
// manager keeps a process table (id/pid/status/run/…), persists it through the
// provided Persister so it survives a restart, and exposes the table so the
// parent agent can inject it into <environment_details>.
//
// The manager never blocks indefinitely: Start waits at most waitTimeout and
// then returns while the remaining processes keep running in the background
// (they are not killed — killing is an explicit decision via Kill).
package subagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status is the lifecycle status of a sub-agent process.
type Status string

// Process lifecycle statuses.
const (
	// StatusRunning means the process is alive.
	StatusRunning Status = "running"
	// StatusCompleted means the process exited with code 0.
	StatusCompleted Status = "completed"
	// StatusFailed means the process exited with a non-zero code.
	StatusFailed Status = "failed"
	// StatusKilled means the process was terminated by Kill (whole group).
	StatusKilled Status = "killed"
	// StatusStale means the record was restored from the store after a parent
	// restart and the process is no longer alive.
	StatusStale Status = "stale"
)

// Bounds for the caller-provided wait timeout (seconds).
const (
	MinWaitSeconds = 1
	MaxWaitSeconds = 600
)

// storeKey is the memory key holding the persisted process table.
const storeKey = "subagent:procs"

// defaultResultLimit caps how many bytes of the sub-agent's final output are
// kept/injected, so a verbose sub-agent cannot blow up the parent context.
const defaultResultLimit = 8 * 1024

// idPattern restricts sub-agent ids: they are used as directory names, so path
// separators, dots and whitespace are rejected (path-traversal guard).
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// Task is one sub-agent launch request.
type Task struct {
	// ID is the caller-chosen sub-agent id (also the workspace folder name).
	ID string
	// Instruction is the one-shot instruction handed to the sub-agent.
	Instruction string
}

// Proc is one row of the sub-agent process table.
type Proc struct {
	ID          string    `json:"id"`
	PID         int       `json:"pid"`
	PGID        int       `json:"pgid"`
	Workspace   string    `json:"workspace"`
	Status      Status    `json:"status"`
	Run         int       `json:"run"`
	Instruction string    `json:"instruction"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	ExitCode    int       `json:"exit_code"`
	LogPath     string    `json:"log_path"`
	ErrLogPath  string    `json:"err_log_path"`
	Result      string    `json:"result"`
	ToolCalls   []string  `json:"tool_calls"`
	// WaitTimedOut records that the launch/wait window expired while this
	// process was still running (it keeps running in the background).
	WaitTimedOut bool `json:"wait_timed_out"`
}

// Finished reports whether the process reached a terminal state.
func (p *Proc) Finished() bool {
	switch p.Status {
	case StatusCompleted, StatusFailed, StatusKilled, StatusStale:
		return true
	}
	return false
}

// Elapsed returns the process run time (until now while running).
func (p *Proc) Elapsed() time.Duration {
	end := p.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	if p.StartedAt.IsZero() {
		return 0
	}
	return end.Sub(p.StartedAt)
}

// Persister persists the process table across restarts. It is implemented by
// the agent layer (store.DualStore) so this package stays storage-agnostic.
type Persister interface {
	SaveMemory(key, value string) error
	GetMemory(key string) (string, bool, error)
}

// Options configures a Manager.
type Options struct {
	// Workspace is the parent workspace root; sub-agent workspaces are created
	// under {Workspace}/sub-agents/{id}/.
	Workspace string
	// ExecPath is the co-shell executable to spawn (defaults to os.Executable).
	ExecPath string
	// ConfigPath is the parent config file shared with sub-agents
	// (CO_SHELL_CONFIG_PATH). Empty means "inherit the parent's env value".
	ConfigPath string
	// ResultLimit caps the bytes kept from a sub-agent's final output.
	ResultLimit int
	// Persister persists the process table; nil disables persistence.
	Persister Persister
	// NoStoreLoad skips loading the previous table on first use (tests).
	NoStoreLoad bool
}

// Manager owns the sub-agent process table.
type Manager struct {
	mu     sync.Mutex
	opts   Options
	procs  map[string]*Proc
	cmds   map[string]*execHandle
	done   map[string]chan struct{}
	loaded bool
	// noStream drops the default machine-readable display flags from launches
	// (set only when the operator explicitly switched the display off).
	noStream bool
}

// execHandle couples a running command with its kill bookkeeping.
type execHandle struct {
	killed   bool          // set by Kill so the waiter keeps the killed status
	exitWait chan struct{} // closed by the reaper once the process exited
}

// NewManager creates a manager. The process table is loaded lazily on first use.
func NewManager(opts Options) *Manager {
	if opts.ResultLimit <= 0 {
		opts.ResultLimit = defaultResultLimit
	}
	return &Manager{
		opts:  opts,
		procs: make(map[string]*Proc),
		cmds:  make(map[string]*execHandle),
		done:  make(map[string]chan struct{}),
	}
}

// SetPersister attaches/replaces the persister after construction.
func (m *Manager) SetPersister(p Persister) {
	m.mu.Lock()
	m.opts.Persister = p
	m.mu.Unlock()
}

// SetStreamFlags controls whether every launch carries the machine-readable
// display flags (--show-tool on --show-llm-content on). The parent recovers a
// sub-agent's final answer from its tool_call events, so those events must be
// emitted even when the interactive display is off; pass false only when the
// operator explicitly disabled the display (FEATURE-563).
func (m *Manager) SetStreamFlags(enabled bool) {
	m.mu.Lock()
	m.noStream = !enabled
	m.mu.Unlock()
}

// ValidateID reports whether id is a legal sub-agent id.
func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid sub-agent id %q: must match %s", id, idPattern.String())
	}
	return nil
}

// ParseTasks normalises and validates a batch of tasks.
func ParseTasks(tasks []Task) ([]Task, error) {
	if len(tasks) == 0 {
		return nil, errors.New("at least one task is required")
	}
	seen := make(map[string]bool, len(tasks))
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		id := strings.TrimSpace(t.ID)
		if err := ValidateID(id); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate sub-agent id %q in one batch", id)
		}
		if strings.TrimSpace(t.Instruction) == "" {
			return nil, fmt.Errorf("sub-agent %q: instruction must not be empty", id)
		}
		seen[id] = true
		out = append(out, Task{ID: id, Instruction: t.Instruction})
	}
	return out, nil
}

// ValidateWaitSeconds checks the mandatory wait timeout.
func ValidateWaitSeconds(seconds int) error {
	if seconds < MinWaitSeconds || seconds > MaxWaitSeconds {
		return fmt.Errorf("timeout_seconds must be between %d and %d, got %d",
			MinWaitSeconds, MaxWaitSeconds, seconds)
	}
	return nil
}

// Lookup returns a copy of one process-table row.
func (m *Manager) Lookup(id string) (*Proc, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	p, ok := m.procs[id]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// List returns process-table rows (all when ids is empty), newest run first.
func (m *Manager) List(ids []string) []*Proc {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()

	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := make([]*Proc, 0, len(m.procs))
	for id, p := range m.procs {
		if len(want) > 0 && !want[id] {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

// SubAgentDir returns the workspace directory of a sub-agent.
func (m *Manager) SubAgentDir(id string) string {
	return filepath.Join(m.opts.Workspace, "sub-agents", id)
}

// ---------- persistence ----------

// loadLocked restores the process table from the store once, marking records
// whose process is gone as stale. Caller must hold m.mu.
func (m *Manager) loadLocked() {
	if m.loaded {
		return
	}
	m.loaded = true
	if m.opts.NoStoreLoad || m.opts.Persister == nil {
		return
	}
	raw, ok, err := m.opts.Persister.GetMemory(storeKey)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return
	}
	var rows []*Proc
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return
	}
	for _, row := range rows {
		if row == nil || row.ID == "" {
			continue
		}
		if row.Status == StatusRunning && !processAlive(row.PID) {
			row.Status = StatusStale
			if row.EndedAt.IsZero() {
				row.EndedAt = time.Now()
			}
		}
		m.procs[row.ID] = row
	}
}

// saveLocked persists the table. Caller must hold m.mu.
func (m *Manager) saveLocked() {
	if m.opts.Persister == nil {
		return
	}
	rows := make([]*Proc, 0, len(m.procs))
	for _, p := range m.procs {
		cp := *p
		rows = append(rows, &cp)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	if data, err := json.Marshal(rows); err == nil {
		_ = m.opts.Persister.SaveMemory(storeKey, string(data))
	}
}

// ---------- helpers shared by the other files ----------

// resultLimit is the configured cap for injected output.
func (m *Manager) resultLimit() int {
	if m.opts.ResultLimit > 0 {
		return m.opts.ResultLimit
	}
	return defaultResultLimit
}

// OutputFiles lists the files a sub-agent produced under its own
// {workspace}/output/ directory.
func (m *Manager) OutputFiles(id string) []string {
	dir := filepath.Join(m.SubAgentDir(id), "output")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}
