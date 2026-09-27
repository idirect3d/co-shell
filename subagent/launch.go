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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// terminateGrace is how long a sub-agent gets to shut down after SIGTERM
// before Kill escalates to SIGKILL on the whole process group.
const terminateGrace = 500 * time.Millisecond

// machineReadableArgs keep the JSON stream parseable: the child inherits
// config.json, whose show_tool/show_llm_content switches target an interactive
// terminal, but the parent recovers the final answer from the
// attempt_completion tool_call result, so those events must always be emitted
// unless the operator explicitly switched the display off (FEATURE-563).
var machineReadableArgs = []string{"--show-tool", "on", "--show-llm-content", "on"}

// prepared is a validated launch request whose log files are already open.
// All requests are prepared before the first process starts, so an invalid
// batch never leaves half of it running.
type prepared struct {
	task Task
	// session is the session the sub-agent belongs to; it is captured under
	// the lock so a batch always lands in the pool it was validated against
	// (FEATURE-565).
	session string
	run     int
	dir     string
	logPath string
	errPath string
	stdout  *os.File
	stderr  *os.File
}

// close closes the open log files (idempotent enough for the error paths).
func (p *prepared) close() {
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

// Start launches one co-shell process per task and waits at most waitTimeout
// for them to finish.
//
// Processes still running when the window expires are left running in the
// background (their row carries WaitTimedOut); nothing is ever killed
// implicitly — termination is an explicit Kill/Close decision.
//
// ctx is only consulted before spawning: a sub-agent must survive the parent
// context being cancelled (its lifetime is governed by Kill/Close), which is
// why exec.Command rather than exec.CommandContext is used.
func (m *Manager) Start(ctx context.Context, tasks []Task, waitTimeout time.Duration) ([]*Proc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parsed, err := ParseTasks(tasks)
	if err != nil {
		// ParseTasks builds the duplicate PoolError without knowing the pool
		// size, so stamp the limit in before the agent layer localizes it.
		return nil, m.stampPoolError(err)
	}

	preps, err := m.prepare(parsed)
	if err != nil {
		return nil, err
	}

	procs := make([]*Proc, 0, len(preps))
	for _, pr := range preps {
		p, err := m.startOne(pr)
		if err != nil {
			pr.close()
			return procs, err
		}
		procs = append(procs, p)
	}
	return m.waitBatch(procs, waitTimeout), nil
}

// prepare validates every task against the process table, creates the
// sub-agent workspaces and opens the per-run log files.
func (m *Manager) prepare(tasks []Task) ([]*prepared, error) {
	if strings.TrimSpace(m.opts.Workspace) == "" {
		return nil, errors.New("sub-agent workspace is not configured")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()

	// The pool belongs to the session bound right now: validate the whole batch
	// against it before anything is created (FEATURE-565).
	session := m.sessionID
	if err := m.validatePoolLocked(session, tasks); err != nil {
		return nil, err
	}

	preps := make([]*prepared, 0, len(tasks))
	for _, t := range tasks {
		run := 1
		if prev, ok := m.procs[procKey(session, t.ID)]; ok {
			run = prev.Run + 1
		}
		dir := m.SubAgentDir(session, t.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			closeAll(preps)
			return nil, fmt.Errorf("create sub-agent workspace %s: %w", dir, err)
		}
		logPath := filepath.Join(dir, fmt.Sprintf("run-%d.jsonl", run))
		errPath := filepath.Join(dir, fmt.Sprintf("run-%d.err.log", run))
		stdout, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			closeAll(preps)
			return nil, fmt.Errorf("open sub-agent log %s: %w", logPath, err)
		}
		stderr, err := os.OpenFile(errPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = stdout.Close()
			closeAll(preps)
			return nil, fmt.Errorf("open sub-agent error log %s: %w", errPath, err)
		}
		preps = append(preps, &prepared{
			task: t, session: session, run: run, dir: dir,
			logPath: logPath, errPath: errPath,
			stdout: stdout, stderr: stderr,
		})
	}
	return preps, nil
}

// closeAll closes the log files of every prepared request.
func closeAll(preps []*prepared) {
	for _, p := range preps {
		p.close()
	}
}

// streamArgs returns the display flags injected into a launch: the
// machine-readable default unless the caller opted out (SetStreamFlags).
func (m *Manager) streamArgs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.noStream {
		return nil
	}
	return machineReadableArgs
}

// startOne spawns the sub-agent process of one prepared request and registers
// the reaper goroutine that finalises its row. A second goroutine watches the
// sub-agent's log while it runs so plan progress advances reach the caller
// (FEATURE-564).
func (m *Manager) startOne(pr *prepared) (*Proc, error) {
	execPath := m.opts.ExecPath
	if execPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("locate co-shell executable: %w", err)
		}
		execPath = exe
	}
	args := []string{
		"-w", pr.dir,
		"--name", "sub-" + pr.task.ID,
		"--unattended", "on",
		"--subagent-enabled", "off",
	}
	args = append(args, m.streamArgs()...)
	args = append(args, "--output-format", "json", "--", pr.task.Instruction)
	cmd := exec.Command(execPath, args...)
	cmd.Dir = pr.dir
	cmd.Env = m.childEnv(pr.task.ID)
	cmd.Stdin = nil
	cmd.Stdout = pr.stdout
	cmd.Stderr = pr.stderr
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start sub-agent %q: %w", pr.task.ID, err)
	}
	// The child owns its own descriptors now.
	pr.close()

	pid := cmd.Process.Pid
	pgid := readPGID(pid)
	done := make(chan struct{})
	handle := &execHandle{exitWait: done}
	proc := &Proc{
		ID:          pr.task.ID,
		PID:         pid,
		PGID:        pgid,
		Workspace:   pr.dir,
		SessionID:   pr.session,
		Status:      StatusRunning,
		Run:         pr.run,
		Instruction: pr.task.Instruction,
		StartedAt:   time.Now(),
		LogPath:     pr.logPath,
		ErrLogPath:  pr.errPath,
	}
	key := procKey(pr.session, pr.task.ID)

	m.mu.Lock()
	m.procs[key] = proc
	m.cmds[key] = handle
	m.done[key] = done
	m.saveLocked()
	m.mu.Unlock()

	go m.reap(pr, cmd, handle, done)
	go m.watchProgress(pr.logPath, proc.SessionID, done)

	cp := *proc
	return &cp, nil
}

// reap waits for the sub-agent to exit and finalises its row: status,
// exit code, duration, parsed final answer and tool-call trail.
func (m *Manager) reap(pr *prepared, cmd *exec.Cmd, handle *execHandle, done chan struct{}) {
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	result, toolCalls := parseStream(pr.logPath, m.resultLimit())

	// completed keeps the final row of a naturally finished sub-agent so the
	// notifier can fire outside the lock (FEATURE-566).
	var completed *Proc

	m.mu.Lock()
	if p, ok := m.procs[procKey(pr.session, pr.task.ID)]; ok {
		killed := handle.killed
		p.ExitCode = exitCode
		p.EndedAt = time.Now()
		// The process finished, so it is no longer "running in the background":
		// the wait window that expired earlier is history (FEATURE-563).
		p.WaitTimedOut = false
		switch {
		case killed:
			p.Status = StatusKilled
		case exitCode == 0:
			p.Status = StatusCompleted
		default:
			p.Status = StatusFailed
		}
		p.Result = result
		p.ToolCalls = toolCalls
		// A sub-agent that finished on its own must report itself to the agent
		// that launched it (FEATURE-566). A killed one is not reported: the
		// caller asked for the termination and already has the result.
		if p.Status == StatusCompleted || p.Status == StatusFailed {
			cp := *p
			completed = &cp
		}
	}
	delete(m.cmds, procKey(pr.session, pr.task.ID))
	m.saveLocked()
	m.mu.Unlock()

	close(done)
	if completed != nil {
		m.notifyCompletion(*completed)
	}
}

// waitBatch waits for the given processes until waitTimeout expires (a
// non-positive timeout returns immediately) and returns their fresh rows.
func (m *Manager) waitBatch(procs []*Proc, waitTimeout time.Duration) []*Proc {
	keys := make([]string, 0, len(procs))
	for _, p := range procs {
		keys = append(keys, procKey(p.SessionID, p.ID))
	}
	if waitTimeout > 0 {
		waitChans(m.waitChans(keys), waitTimeout)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	out := make([]*Proc, 0, len(keys))
	for _, key := range keys {
		p, ok := m.procs[key]
		if !ok {
			continue
		}
		p.WaitTimedOut = !p.Finished()
		cp := *p
		out = append(out, &cp)
	}
	m.saveLocked()
	return out
}

// Wait blocks until the given sub-agents finish or timeout expires. An empty
// ids list means "every sub-agent of the current session that is still
// running"; a non-positive timeout returns the current state without waiting.
// Processes are never killed here.
func (m *Manager) Wait(ids []string, timeout time.Duration) []*Proc {
	targets := m.pendingKeys(ids)
	if timeout > 0 {
		waitChans(m.waitChans(targets), timeout)
	}
	return m.List(ids)
}

// pendingKeys resolves the pool keys to watch in the current session: the
// requested ids (existing ones only), or every unfinished sub-agent of that
// session when ids is empty.
func (m *Manager) pendingKeys(ids []string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	if len(ids) > 0 {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			key := procKey(m.sessionID, id)
			if _, ok := m.procs[key]; ok {
				out = append(out, key)
			}
		}
		return out
	}
	out := make([]string, 0, len(m.procs))
	for key, p := range m.procs {
		if p.SessionID == m.sessionID && !p.Finished() {
			out = append(out, key)
		}
	}
	return out
}

// waitChans snapshots the completion channels of the given pool keys.
func (m *Manager) waitChans(keys []string) []chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	chans := make([]chan struct{}, 0, len(keys))
	for _, key := range keys {
		if ch, ok := m.done[key]; ok {
			chans = append(chans, ch)
		}
	}
	return chans
}

// waitChans blocks until every channel is closed or timeout expires.
func waitChans(chans []chan struct{}, timeout time.Duration) {
	if len(chans) == 0 {
		return
	}
	all := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for _, ch := range chans {
			wg.Add(1)
			go func(c chan struct{}) {
				defer wg.Done()
				<-c
			}(ch)
		}
		wg.Wait()
		close(all)
	}()
	select {
	case <-all:
	case <-time.After(timeout):
	}
}

// Kill terminates the given sub-agents of the current session and their whole
// process group: SIGTERM first, SIGKILL after terminateGrace if the process is
// still alive. An unknown id fails the call before anything is signalled.
// Already finished sub-agents are returned unchanged.
func (m *Manager) Kill(ids []string) ([]*Proc, error) {
	targets, err := m.resolveKillTargets(ids)
	if err != nil {
		return nil, err
	}
	m.killTargets(targets)
	return m.List(ids), nil
}

// killTargets signals the whole process group of every target (SIGTERM, then
// SIGKILL after the grace period) and marks the matching rows as killed.
func (m *Manager) killTargets(targets []killTarget) {
	for _, t := range targets {
		_ = killGroup(t.pid, t.pgid, syscall.SIGTERM)
	}
	if len(targets) > 0 {
		time.Sleep(terminateGrace)
		for _, t := range targets {
			if processAlive(t.pid) {
				_ = killGroup(t.pid, t.pgid, syscall.SIGKILL)
			}
		}
	}

	m.mu.Lock()
	now := time.Now()
	for _, t := range targets {
		if p, ok := m.procs[t.key]; ok && p.Status == StatusRunning {
			p.Status = StatusKilled
			p.ExitCode = -1
			p.EndedAt = now
			p.WaitTimedOut = false
		}
	}
	m.saveLocked()
	m.mu.Unlock()
}

// killTarget is a resolved running sub-agent about to be terminated.
type killTarget struct {
	key  string // session-scoped pool key
	id   string
	pid  int
	pgid int
}

// resolveKillTargets validates ids against the current session's pool and flags
// the matching running processes as killed so the reapers keep that status.
func (m *Manager) resolveKillTargets(ids []string) ([]killTarget, error) {
	if len(ids) == 0 {
		return nil, errors.New("at least one sub-agent id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()

	seen := make(map[string]bool, len(ids))
	targets := make([]killTarget, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		key := procKey(m.sessionID, id)
		p, ok := m.procs[key]
		if !ok {
			return nil, fmt.Errorf("unknown sub-agent %q", id)
		}
		if p.Status != StatusRunning {
			continue
		}
		if h, ok := m.cmds[key]; ok {
			h.killed = true
		}
		targets = append(targets, killTarget{key: key, id: id, pid: p.PID, pgid: p.PGID})
	}
	return targets, nil
}

// markKeysForKill resolves explicit pool keys; used by Close, which spans every
// session (FEATURE-565). Missing or already finished keys are skipped.
func (m *Manager) markKeysForKill(keys []string) []killTarget {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadLocked()
	targets := make([]killTarget, 0, len(keys))
	for _, key := range keys {
		p, ok := m.procs[key]
		if !ok || p.Status != StatusRunning {
			continue
		}
		if h, ok := m.cmds[key]; ok {
			h.killed = true
		}
		targets = append(targets, killTarget{key: key, id: p.ID, pid: p.PID, pgid: p.PGID})
	}
	return targets
}

// Close terminates every running sub-agent of every session (process group
// included). It is called when the parent agent shuts down and is safe to call
// more than once.
func (m *Manager) Close() {
	m.mu.Lock()
	m.loadLocked()
	keys := make([]string, 0, len(m.procs))
	for key, p := range m.procs {
		if p.Status == StatusRunning {
			keys = append(keys, key)
		}
	}
	m.mu.Unlock()
	if len(keys) == 0 {
		return
	}
	m.killTargets(m.markKeysForKill(keys))
}

// childEnv builds the sub-agent environment: the parent environment plus the
// sub-agent markers and the shared config path. Inherited copies of these
// variables are dropped first so the explicit values always win.
func (m *Manager) childEnv(id string) []string {
	src := os.Environ()
	env := make([]string, 0, len(src)+3)
	for _, kv := range src {
		if strings.HasPrefix(kv, "CO_SHELL_SUB_AGENT=") ||
			strings.HasPrefix(kv, "CO_SHELL_SUB_AGENT_ID=") ||
			strings.HasPrefix(kv, "CO_SHELL_CONFIG_PATH=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "CO_SHELL_SUB_AGENT=true", "CO_SHELL_SUB_AGENT_ID="+id)
	cfg := m.opts.ConfigPath
	if cfg == "" {
		cfg = os.Getenv("CO_SHELL_CONFIG_PATH")
	}
	if cfg != "" {
		env = append(env, "CO_SHELL_CONFIG_PATH="+cfg)
	}
	return env
}
