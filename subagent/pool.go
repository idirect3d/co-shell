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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The sub-agent id model is a fixed, per-session pool (FEATURE-565): every
// session owns its own A1..An, and a slot can be re-used once its process is no
// longer running.
const (
	// MaxPoolSize is the hard upper bound of one session's pool.
	MaxPoolSize = 10
	// DefaultMaxSubAgents is the pool size used when neither config.json
	// (max_sub_agents) nor --max-sub-agents sets one.
	DefaultMaxSubAgents = 5
)

// poolIDPattern matches the pool ids A1..A10 (digits only: "A0", "A011" and
// "a1" are format errors, not aliases).
var poolIDPattern = regexp.MustCompile(`^A([0-9]+)$`)

// PoolID returns the i-th pool id (1-based): A1..An.
func PoolID(i int) string { return "A" + strconv.Itoa(i) }

// PoolIDs returns the pool A1..An (nil when n <= 0).
func PoolIDs(n int) []string {
	if n <= 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, PoolID(i))
	}
	return out
}

// ValidateMaxSubAgents reports whether n is a legal pool size (0..MaxPoolSize;
// 0 disables sub-agent creation).
func ValidateMaxSubAgents(n int) error {
	if n < 0 || n > MaxPoolSize {
		return fmt.Errorf("max_sub_agents must be between 0 and %d, got %d", MaxPoolSize, n)
	}
	return nil
}

// PoolRange renders the usable range of a pool of size n ("A1..A5", "A1", or
// "disabled" when creation is switched off).
func PoolRange(n int) string {
	switch {
	case n <= 0:
		return "disabled"
	case n == 1:
		return PoolID(1)
	default:
		return PoolID(1) + ".." + PoolID(n)
	}
}

// ParsePoolID splits an "A<n>" id into its 1-based slot. ok is false for every
// other string (including "a1", "A", "A 1", "B1", "scheduled-3").
func ParsePoolID(id string) (int, bool) {
	m := poolIDPattern.FindStringSubmatch(id)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// ValidateID checks the syntax of a pool id, independent of the configured pool
// size. Callers that know the size validate the range through Start.
func ValidateID(id string) error {
	if _, ok := ParsePoolID(id); !ok {
		return fmt.Errorf("invalid sub-agent id %q: must be A1..A%d", id, MaxPoolSize)
	}
	return nil
}

// PoolErrorKind classifies a pool validation failure so the agent layer can
// render a localized message (FEATURE-565).
type PoolErrorKind int

// Pool validation failure kinds.
const (
	// PoolErrDisabled means creation is switched off (max_sub_agents = 0).
	PoolErrDisabled PoolErrorKind = iota
	// PoolErrIDRequired means the id was missing/blank.
	PoolErrIDRequired
	// PoolErrIDFormat means the id is not an "A<n>" id at all.
	PoolErrIDFormat
	// PoolErrIDOutOfRange means the id is outside the current pool (A0, A{n+1}).
	PoolErrIDOutOfRange
	// PoolErrIDBusy means the id's sub-agent is still running in this session.
	PoolErrIDBusy
	// PoolErrIDDuplicate means the same id appeared twice in one batch.
	PoolErrIDDuplicate
)

// PoolError is the typed error every pool validation path returns.
type PoolError struct {
	Kind PoolErrorKind
	// ID is the offending id (empty for PoolErrDisabled).
	ID string
	// Max is the pool size in force when the check ran (0 = disabled).
	Max int
	// Run and PID describe the running sub-agent behind a PoolErrIDBusy.
	Run int
	PID int
}

// Error renders a diagnostic (English) message; the user-facing text is
// localized by the agent layer from Kind.
func (e *PoolError) Error() string {
	switch e.Kind {
	case PoolErrDisabled:
		return "sub-agent creation is disabled (max_sub_agents = 0)"
	case PoolErrIDRequired:
		return "sub-agent id is required"
	case PoolErrIDFormat:
		return fmt.Sprintf("invalid sub-agent id %q: must be A1..A%d", e.ID, MaxPoolSize)
	case PoolErrIDOutOfRange:
		return fmt.Sprintf("sub-agent id %q is outside the pool (%s)", e.ID, PoolRange(e.Max))
	case PoolErrIDBusy:
		return fmt.Sprintf("sub-agent %q is already running (pid %d, run %d)", e.ID, e.PID, e.Run)
	case PoolErrIDDuplicate:
		return fmt.Sprintf("duplicate sub-agent id %q in one batch", e.ID)
	}
	return "invalid sub-agent pool request"
}

// AsPoolError extracts the typed pool error (nil, false when err is not one).
func AsPoolError(err error) (*PoolError, bool) {
	var pe *PoolError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

// stampPoolError fills in the pool size of a PoolError built outside a manager
// (ParseTasks, FEATURE-565) so the agent layer can render the localized message
// with the limit actually in force. Errors of any other type pass through.
func (m *Manager) stampPoolError(err error) error {
	pe, ok := AsPoolError(err)
	if !ok || pe.Max != 0 {
		return err
	}
	m.mu.Lock()
	pe.Max = m.maxSubAgents
	m.mu.Unlock()
	return err
}

// SetMaxSubAgents sets the per-session pool size (0..MaxPoolSize).
func (m *Manager) SetMaxSubAgents(n int) error {
	if err := ValidateMaxSubAgents(n); err != nil {
		return err
	}
	m.mu.Lock()
	m.maxSubAgents = n
	m.mu.Unlock()
	return nil
}

// MaxSubAgents returns the per-session pool size currently in force.
func (m *Manager) MaxSubAgents() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxSubAgents
}

// PoolStatus describes one session's pool (FEATURE-565).
type PoolStatus struct {
	// Max is the pool size (0 = creation disabled).
	Max int `json:"max"`
	// Running lists the ids that are currently running, in slot order.
	Running []string `json:"running"`
}

// PoolStatus reports one session's pool state.
func (m *Manager) PoolStatus(sessionID string) PoolStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	return PoolStatus{Max: m.maxSubAgents, Running: runningIDsLocked(m.procs, sessionID)}
}

// SessionRunningIDs returns the ids of one session's running sub-agents in slot
// order (A1 first). It is used by the session-switch guard: a session whose
// sub-agents are still running may not be left (FEATURE-565).
func (m *Manager) SessionRunningIDs(sessionID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	return runningIDsLocked(m.procs, sessionID)
}

// ValidatePoolID reports whether id names a free slot of the given session's
// pool: it must be A1..An and must not point at a running sub-agent.
func (m *Manager) ValidatePoolID(sessionID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	return m.validateOneLocked(sessionID, id)
}

// procKey is the process-table key of one sub-agent: the session that launched
// it plus its pool id. Two sessions therefore never share a row (FEATURE-565).
func procKey(sessionID, id string) string { return sessionID + "\x00" + id }

// sessionDirName turns a session id into a path-safe directory name. The
// session id is normally generated by co-shell, but it can come from
// --session-id, so anything outside [A-Za-z0-9._-] is replaced.
func sessionDirName(sessionID string) string {
	s := strings.TrimSpace(sessionID)
	if s == "" {
		return defaultSessionDirName
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// defaultSessionDirName is the workspace folder of the pool of runs without a
// session (single-shot CLI runs).
const defaultSessionDirName = "default"

// runningIDsLocked collects the running ids of one session in slot order.
// Caller must hold m.mu.
func runningIDsLocked(procs map[string]*Proc, sessionID string) []string {
	out := make([]string, 0, len(procs))
	for _, p := range procs {
		if p.SessionID != sessionID || p.Status != StatusRunning {
			continue
		}
		out = append(out, p.ID)
	}
	sortByPoolSlot(out)
	return out
}

// sortByPoolSlot orders ids A1, A2, ... A10 (stable for non-pool ids).
func sortByPoolSlot(ids []string) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && poolSlotOf(ids[j]) < poolSlotOf(ids[j-1]); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

// poolSlotOf maps an id to its pool slot: A1 -> 1 ... A10 -> 10. Ids that do
// not belong to the pool sort after it (defensive: the table may still carry
// rows written by an older build).
func poolSlotOf(id string) int {
	if n, ok := ParsePoolID(id); ok {
		return n
	}
	return MaxPoolSize + 1
}

// validateOneLocked checks one id against the session's pool. Caller must hold
// m.mu and have loaded the table.
func (m *Manager) validateOneLocked(sessionID, id string) error {
	if m.maxSubAgents <= 0 {
		return &PoolError{Kind: PoolErrDisabled, Max: 0}
	}
	if strings.TrimSpace(id) == "" {
		return &PoolError{Kind: PoolErrIDRequired, Max: m.maxSubAgents}
	}
	slot, ok := ParsePoolID(id)
	if !ok {
		return &PoolError{Kind: PoolErrIDFormat, ID: id, Max: m.maxSubAgents}
	}
	if slot < 1 || slot > m.maxSubAgents {
		return &PoolError{Kind: PoolErrIDOutOfRange, ID: id, Max: m.maxSubAgents}
	}
	if prev, ok := m.procs[procKey(sessionID, id)]; ok && prev.Status == StatusRunning {
		return &PoolError{Kind: PoolErrIDBusy, ID: id, Max: m.maxSubAgents, Run: prev.Run, PID: prev.PID}
	}
	return nil
}

// validatePoolLocked validates a whole batch before anything is spawned: pool
// size, per-id syntax/range, duplicates inside the batch and slots already
// running in this session. Caller must hold m.mu.
func (m *Manager) validatePoolLocked(sessionID string, tasks []Task) error {
	seen := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if err := m.validateOneLocked(sessionID, t.ID); err != nil {
			return err
		}
		if seen[t.ID] {
			return &PoolError{Kind: PoolErrIDDuplicate, ID: t.ID, Max: m.maxSubAgents}
		}
		seen[t.ID] = true
	}
	return nil
}
