// SessionIO abstraction: per-session input reading and per-run I/O wiring
// (FEATURE-307b). The REPL main loop programs against SessionIO instead of
// branching on the input mode; each implementation owns its input source
// (StdioSource / unified InputReader) and the per-run agent I/O assembly
// (UserIO installation, ESC consumer, command hooks, event renderer).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/log"
)

// SessionIO abstracts one input session of the REPL (FEATURE-307b).
type SessionIO interface {
	// ReadLine reads one user input line (prompt is only displayed in
	// interactive sessions).
	ReadLine(prompt string) (string, error)
	// Acquire wires per-run I/O for one agent execution: installs the UserIO
	// on the agent, starts the ESC consumer / command hooks as the mode
	// requires, and returns the event renderer plus a release func.
	Acquire(ag *agent.Agent) (agent.EventRenderer, func())
	// Interactive reports whether prompt/welcome decorations are shown.
	Interactive() bool
	// Close releases terminal resources (reader goroutine, raw mode).
	Close() error
}

// SessionDeps carries the dependencies a session factory needs.
type SessionDeps struct {
	Cfg *config.Config
	// HistoryFn returns the current in-memory history. It is a function
	// because the REPL reassigns its history slice on every save; capturing
	// the slice itself would go stale.
	HistoryFn func() []string
	// OutputFormat is "text" (default, decorated terminal lines via
	// LineRenderer) or "json" (JSON-Lines via StreamRenderer).
	OutputFormat string
}

// sessionFactories maps the input mode name to its session constructor.
var sessionFactories = map[string]func(deps SessionDeps) (SessionIO, error){
	"stdio": newStdioSession,
	"tui":   newTUISession,
}

// stdioSession is the standard-input session: a persistent StdioSource for
// reading and StdioIO for per-run agent I/O. With OutputFormat "json" it is
// non-interactive (no prompt/welcome decorations) and renders events as
// JSON-Lines on stdout.
type stdioSession struct {
	deps SessionDeps
	src  *StdioSource
}

func newStdioSession(deps SessionDeps) (SessionIO, error) {
	return &stdioSession{deps: deps, src: NewStdioSource()}, nil
}

func (s *stdioSession) ReadLine(prompt string) (string, error) {
	if s.Interactive() {
		fmt.Print(prompt)
	}
	ev, err := s.src.NextEvent(context.Background())
	if err != nil {
		return "", err
	}
	if ev.Kind == agent.InputEOF {
		return "", io.EOF
	}
	return strings.TrimSpace(ev.Data), nil
}

func (s *stdioSession) Acquire(ag *agent.Agent) (agent.EventRenderer, func()) {
	log.Debug("REPL.handleAgentInput: setting up StdioIO")
	sio := NewStdioIO()
	ag.SetIO(sio)
	var renderer agent.EventRenderer
	if s.deps.OutputFormat == "json" {
		renderer = agent.NewStreamRenderer(os.Stdout)
	} else {
		ep := config.GetEmojiPrefixes(s.deps.Cfg.LLM.EmojiEnabled)
		renderer = agent.NewLineRenderer(sio, ep, agent.StreamModeREPL)
	}
	release := func() {
		// Reset agent's UserIO so the agent defaults back to fmtIO for any
		// remaining output.
		ag.SetIO(nil)
	}
	return renderer, release
}

func (s *stdioSession) Interactive() bool { return s.deps.OutputFormat != "json" }

func (s *stdioSession) Close() error { return nil }

// tuiSession is the interactive terminal session: it owns the unified
// InputReader (single stdin goroutine + raw terminal mode), the ESC consumer
// and the command hooks that pause/resume the reader around system commands.
type tuiSession struct {
	deps   SessionDeps
	reader *InputReader
}

// newTUISession creates the unified input reader and starts it (enters raw
// mode, spawns the reader goroutine). A start failure returns the error so
// the REPL can fall back to the stdio session.
func newTUISession(deps SessionDeps) (SessionIO, error) {
	reader := NewInputReader(NewRawKeySource())
	if err := reader.Start(); err != nil {
		_ = reader.Close()
		return nil, err
	}
	return &tuiSession{deps: deps, reader: reader}, nil
}

func (s *tuiSession) ReadLine(prompt string) (string, error) {
	// While the unified reader is paused (a builtin command wizard owns stdin
	// in cooked mode), the event stream is suspended — read a plain line
	// directly instead. Raw mode is already restored to cooked by Pause.
	if s.reader != nil && s.reader.IsPaused() {
		fmt.Print(prompt)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		return strings.TrimSpace(scanner.Text()), nil
	}
	ei := NewEnhancedInput(prompt, s.deps.HistoryFn())
	input, err := ei.ReadLineFrom(s.reader)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (s *tuiSession) Acquire(ag *agent.Agent) (agent.EventRenderer, func()) {
	log.Debug("REPL.handleAgentInput: setting up EnhancedIO and ESC consumer (mode=tui)")
	eio := NewEnhancedIO(s.deps.HistoryFn(), s.reader)
	ag.SetIO(eio)
	stopConsumer := s.startEscConsumer(ag)
	// Register command hooks: while a system command runs, pause the input
	// reader (stop reading stdin + restore cooked mode) so interactive
	// commands (sudo, passwd, etc.) can read stdin with echo and line
	// buffering; resume afterwards. Without this, raw mode (no ECHO/ICRNL)
	// makes interactive commands hang with no visible feedback.
	ag.SetCommandHooks(agent.CommandHooks{
		BeforeCommand: func() {
			if s.reader != nil {
				s.reader.Pause()
			}
		},
		AfterCommand: func() {
			if s.reader != nil {
				s.reader.Resume()
			}
		},
	})
	ep := config.GetEmojiPrefixes(s.deps.Cfg.LLM.EmojiEnabled)
	renderer := agent.NewLineRenderer(eio, ep, agent.StreamModeREPL)
	release := func() {
		// Stop the ESC consumer and clear command hooks.
		log.Debug("REPL.handleAgentInput: stopping ESC consumer")
		stopConsumer()
		ag.SetCommandHooks(agent.CommandHooks{})
		// Reset agent's UserIO so the agent defaults back to fmtIO for any
		// remaining output.
		ag.SetIO(nil)
	}
	return renderer, release
}

func (s *tuiSession) Interactive() bool { return true }

func (s *tuiSession) Close() error {
	if s.reader != nil {
		return s.reader.Close()
	}
	return nil
}
