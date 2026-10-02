// Active child-process-group registry (FEATURE-582).
//
// Tools that spawn child processes register their process group here after
// cmd.Start(), so a user interrupt can terminate every process group that a
// tool started (SIGTERM, then SIGKILL after a short grace period). This is the
// handle behind "interrupt immediately kills the tool's processes".
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// procKillGrace is how long a process group is given to exit after SIGTERM
// before it is SIGKILLed.
const procKillGrace = 200 * time.Millisecond

// ProcEntry describes one process group started by a tool.
type ProcEntry struct {
	Tool      string    // tool name that started it (e.g. "execute_command")
	Command   string    // human-readable command line
	PGID      int       // process group id (== leader PID, set by setProcessGroupAttr)
	PID       int       // leader PID
	StartedAt time.Time // when the group was started
	Detached  bool      // on_timeout=detach: still killed on interrupt (Q4-B)
}

// ProcRegistry tracks the live process groups started by tools.
type ProcRegistry struct {
	mu     sync.Mutex
	groups map[int]*ProcEntry // keyed by PGID
}

// newProcRegistry creates an empty registry.
func newProcRegistry() *ProcRegistry {
	return &ProcRegistry{groups: make(map[int]*ProcEntry)}
}

// Register records the process group of a started command and returns a
// function that removes it again. The caller MUST call the returned function
// once the command has exited (normally via defer), otherwise a later
// interrupt would try to kill a stale (possibly reused) PID.
//
// cmd must have been started successfully. Windows has no real process group
// concept; the leader PID is used as the identifier there.
func (r *ProcRegistry) Register(tool, command string, cmd *exec.Cmd, detached bool) func() {
	if r == nil || cmd == nil || cmd.Process == nil {
		return func() {}
	}
	return r.RegisterPID(tool, command, cmd.Process.Pid, detached)
}

// RegisterPID records a process group identified by its leader PID instead of
// an *exec.Cmd. It is used for long-lived children whose *exec.Cmd is owned
// elsewhere (e.g. the persistent shell session, FEATURE-582).
func (r *ProcRegistry) RegisterPID(tool, command string, pid int, detached bool) func() {
	if r == nil || pid <= 0 {
		return func() {}
	}
	entry := &ProcEntry{
		Tool:      tool,
		Command:   command,
		PGID:      pid,
		PID:       pid,
		StartedAt: time.Now(),
		Detached:  detached,
	}

	r.mu.Lock()
	r.groups[pid] = entry
	r.mu.Unlock()

	log.Debug("ProcRegistry: registered %s process group pgid=%d (detached=%v): %s", tool, pid, detached, command)

	return func() {
		r.mu.Lock()
		delete(r.groups, pid)
		r.mu.Unlock()
	}
}

// KillAll terminates every registered process group: SIGTERM first, then
// SIGKILL after procKillGrace. It returns the entries it acted on (Q4-B: the
// interrupt kills every process group a tool started, including detach
// background services and sub-agent children).
func (r *ProcRegistry) KillAll(reason string) []ProcEntry {
	return r.kill(reason, true)
}

// KillActive terminates the non-detached process groups only. It is kept for
// callers that want to stop foreground work while leaving explicitly detached
// background services alone.
func (r *ProcRegistry) KillActive(reason string) []ProcEntry {
	return r.kill(reason, false)
}

func (r *ProcRegistry) kill(reason string, includeDetached bool) []ProcEntry {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	entries := make([]ProcEntry, 0, len(r.groups))
	for _, e := range r.groups {
		if !includeDetached && e.Detached {
			continue
		}
		entries = append(entries, *e)
	}
	r.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	for _, e := range entries {
		terminateProcessGroup(e.PGID)
	}
	time.Sleep(procKillGrace)
	for _, e := range entries {
		killProcessGroupByID(e.PGID)
	}

	r.mu.Lock()
	for _, e := range entries {
		delete(r.groups, e.PGID)
	}
	remaining := len(r.groups)
	r.mu.Unlock()

	log.Warn("ProcRegistry: terminated %d process group(s) (reason=%s, includeDetached=%v, stillRegistered=%d): %s",
		len(entries), reason, includeDetached, remaining, FormatProcEntries(entries))
	return entries
}

// Snapshot returns a copy of the currently registered entries, sorted by start
// time, for logging or UI display.
func (r *ProcRegistry) Snapshot() []ProcEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	entries := make([]ProcEntry, 0, len(r.groups))
	for _, e := range r.groups {
		entries = append(entries, *e)
	}
	r.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedAt.Before(entries[j].StartedAt)
	})
	return entries
}

// FormatProcEntries renders entries as "tool/pgid" pairs for human-readable logs.
func FormatProcEntries(entries []ProcEntry) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.Tool+"/"+strconv.Itoa(e.PGID))
	}
	return strings.Join(parts, ", ")
}
