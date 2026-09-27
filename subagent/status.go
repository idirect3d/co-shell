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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SetWorkspace injects the parent workspace root once the agent layer knows it
// (the manager is constructed before the workspace is resolved).
func (m *Manager) SetWorkspace(dir string) {
	m.mu.Lock()
	m.opts.Workspace = dir
	m.mu.Unlock()
}

// StatusBlock renders the <sub_agents> block injected into
// <environment_details> (FEATURE-563/565): the pool upper bound (max="N") plus
// the sub-agents of the *current session* that are running right now. Finished
// sub-agents do not appear, so the block always mirrors the live pool. It is
// emitted even when nothing is running, because max is what tells the model
// which ids it may use.
func (m *Manager) StatusBlock() string {
	m.mu.Lock()
	m.loadLocked()
	max := m.maxSubAgents
	sessionID := m.sessionID
	rows := make([]Proc, 0, len(m.procs))
	for _, p := range m.procs {
		if p.SessionID != sessionID || p.Status != StatusRunning {
			continue
		}
		rows = append(rows, *p)
	}
	m.mu.Unlock()

	// Rows are ordered by pool slot so the block stays diff-friendly.
	sort.Slice(rows, func(i, j int) bool { return poolSlotOf(rows[i].ID) < poolSlotOf(rows[j].ID) })

	var sb strings.Builder
	fmt.Fprintf(&sb, "<sub_agents max=%q>\n", strconv.Itoa(max))
	for i := range rows {
		p := &rows[i]
		fmt.Fprintf(&sb, "  <agent id=%q status=%q run=%q pid=%q elapsed=%q/>\n",
			p.ID, string(p.Status), strconv.Itoa(p.Run), strconv.Itoa(p.PID), formatElapsed(p.Elapsed()))
	}
	sb.WriteString("</sub_agents>")
	return sb.String()
}

// LogTail returns the last maxBytes of a sub-agent's stdout log (the
// JSON-Lines stream). Missing logs yield an empty string.
func (m *Manager) LogTail(id string, maxBytes int) string {
	m.mu.Lock()
	p, ok := m.procs[procKey(m.sessionID, id)]
	path := ""
	if ok {
		path = p.LogPath
	}
	m.mu.Unlock()
	if !ok {
		return ""
	}
	return tailFile(path, maxBytes)
}

// formatElapsed renders a duration as whole seconds ("12s"), which is precise
// enough for the status block and keeps the injected text small.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return strconv.Itoa(int(d.Seconds())) + "s"
}
