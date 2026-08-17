// Package repl - SessionIO factory and Acquire/release pairing tests
// (FEATURE-307b).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// sessionTestDeps returns SessionDeps with a static (empty) history and the
// given output format.
func sessionTestDeps(outputFormat string) SessionDeps {
	return SessionDeps{
		Cfg:          &config.Config{},
		HistoryFn:    func() []string { return nil },
		OutputFormat: outputFormat,
	}
}

// newSessionTestAgent builds an Agent backed by a temp-workspace store (the
// agent constructor dereferences the store via the task plan manager).
func newSessionTestAgent(t *testing.T) *agent.Agent {
	t.Helper()
	ws, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	return agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")
}

// TestSessionFactoriesRegistered verifies the factory map pairs each input
// mode with its constructor.
func TestSessionFactoriesRegistered(t *testing.T) {
	for _, mode := range []string{"stdio", "tui"} {
		if sessionFactories[mode] == nil {
			t.Errorf("sessionFactories[%q] is nil", mode)
		}
	}
}

// TestNewStdioSession verifies the stdio factory returns the stdio
// implementation and that interactivity follows the output format.
func TestNewStdioSession(t *testing.T) {
	sess, err := sessionFactories["stdio"](sessionTestDeps("text"))
	if err != nil {
		t.Fatalf("stdio factory: %v", err)
	}
	if _, ok := sess.(*stdioSession); !ok {
		t.Fatalf("stdio factory returned %T, want *stdioSession", sess)
	}
	if !sess.Interactive() {
		t.Errorf("stdio text session should be interactive (prompt shown)")
	}

	jsonSess, err := sessionFactories["stdio"](sessionTestDeps("json"))
	if err != nil {
		t.Fatalf("stdio factory (json): %v", err)
	}
	if jsonSess.Interactive() {
		t.Errorf("stdio json session must be non-interactive (no decorations)")
	}
}

// TestStdioSessionAcquireRelease verifies the Acquire/release pairing: the
// UserIO is installed on the agent during the run and removed by release,
// and the renderer matches the output format.
func TestStdioSessionAcquireRelease(t *testing.T) {
	tests := []struct {
		format     string
		wantRender interface{}
	}{
		{"text", &agent.LineRenderer{}},
		{"json", &agent.StreamRenderer{}},
	}
	for _, tt := range tests {
		sess, err := newStdioSession(sessionTestDeps(tt.format))
		if err != nil {
			t.Fatalf("newStdioSession(%q): %v", tt.format, err)
		}
		ag := newSessionTestAgent(t)
		if ag.IO() != nil {
			t.Fatalf("format %q: agent IO should be nil before Acquire", tt.format)
		}
		renderer, release := sess.Acquire(ag)
		if ag.IO() == nil {
			t.Errorf("format %q: agent IO not installed by Acquire", tt.format)
		}
		switch tt.wantRender.(type) {
		case *agent.LineRenderer:
			if _, ok := renderer.(*agent.LineRenderer); !ok {
				t.Errorf("format %q: renderer = %T, want *agent.LineRenderer", tt.format, renderer)
			}
		case *agent.StreamRenderer:
			if _, ok := renderer.(*agent.StreamRenderer); !ok {
				t.Errorf("format %q: renderer = %T, want *agent.StreamRenderer", tt.format, renderer)
			}
		}
		release()
		if ag.IO() != nil {
			t.Errorf("format %q: agent IO not cleared by release", tt.format)
		}
	}
}

// TestTUISessionAcquireRelease verifies the tui Acquire/release pairing with
// a non-started reader (raw mode is unavailable in tests): UserIO installed
// and cleared, ESC consumer subscribed and unsubscribed.
func TestTUISessionAcquireRelease(t *testing.T) {
	sess := &tuiSession{
		deps:   sessionTestDeps("text"),
		reader: NewInputReader(NewRawKeySource()),
	}
	ag := newSessionTestAgent(t)

	renderer, release := sess.Acquire(ag)
	if _, ok := ag.IO().(*EnhancedIO); !ok {
		t.Errorf("tui Acquire installed %T, want *EnhancedIO", ag.IO())
	}
	if _, ok := renderer.(*agent.LineRenderer); !ok {
		t.Errorf("tui renderer = %T, want *agent.LineRenderer", renderer)
	}
	if got := len(sess.reader.subs); got != 1 {
		t.Errorf("ESC consumer not subscribed: %d subscribers, want 1", got)
	}
	release()
	if ag.IO() != nil {
		t.Errorf("agent IO not cleared by release")
	}
	if got := len(sess.reader.subs); got != 0 {
		t.Errorf("ESC consumer leaked: %d subscribers after release, want 0", got)
	}
}

// TestTUIFactoryFallsBackToStdio verifies the REPL fallback path: when the
// tui factory fails (no raw terminal, the common case in test environments)
// the stdio factory still yields a working session. On a real tty the tui
// factory succeeds; then verify the session is interactive and close it.
func TestTUIFactoryFallsBackToStdio(t *testing.T) {
	sess, err := sessionFactories["tui"](sessionTestDeps("text"))
	if err != nil {
		fallback, ferr := sessionFactories["stdio"](sessionTestDeps("text"))
		if ferr != nil {
			t.Fatalf("stdio fallback after tui failure: %v", ferr)
		}
		if !fallback.Interactive() {
			t.Errorf("stdio fallback session should be interactive in text mode")
		}
		return
	}
	defer func() { _ = sess.Close() }()
	if _, ok := sess.(*tuiSession); !ok {
		t.Errorf("tui factory returned %T, want *tuiSession", sess)
	}
	if !sess.Interactive() {
		t.Errorf("tui session must be interactive")
	}
}
