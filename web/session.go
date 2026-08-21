// Package web - REPL session wiring for `co-shell serve` (FEATURE-307c):
// WebSession implements repl.SessionIO (browser input via WebSocket),
// WebIO implements agent.UserIO (prints become ui_text events, ReadLine/
// ReadKey become ask/answer round-trips), WebRenderer implements
// agent.EventRenderer (stream events pushed to the browser as-is).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/repl"
)

// errNoWebClient is returned by WebIO input methods when no browser is
// connected (an unanswered ask would otherwise block the agent forever).
var errNoWebClient = errors.New("no web client connected")

// Turn-boundary event types emitted by WebSession itself, straight to the
// browser via sendEvent — they never pass through a renderer, so the TUI
// and stdio modes never see them. The frontend uses them to flip its
// merged send/interrupt button (FEATURE-369).
const (
	eventAwaitInput = "await_input" // REPL is about to block for the next input (turn ended)
	eventTurnStart  = "turn_start"  // an input was consumed from the queue (turn begins)
)

// SessionFactory returns the repl session factory bound to this server.
// main.go registers it via repl.RegisterSessionFactory("web", ...).
func (s *Server) SessionFactory() func(repl.SessionDeps) (repl.SessionIO, error) {
	return func(deps repl.SessionDeps) (repl.SessionIO, error) {
		return newWebSession(s, deps)
	}
}

// WebSession is the browser-backed SessionIO (FEATURE-307c). Input lines
// arrive as WebSocket "input" messages; interrupt requests map to
// agent.Interrupt (the ESC equivalent).
type WebSession struct {
	srv *Server
	ag  *agent.Agent
	wio *WebIO

	inputCh chan clientMessage
	closed  chan struct{}
}

// newWebSession creates the session, installs the WebIO on the agent for
// the whole session lifetime (builtin command output must stay visible in
// the browser), and registers the message handler on the server.
func newWebSession(srv *Server, deps repl.SessionDeps) (*WebSession, error) {
	if deps.Ag == nil {
		return nil, errors.New("web session requires SessionDeps.Ag")
	}
	sess := &WebSession{
		srv:     srv,
		ag:      deps.Ag,
		inputCh: make(chan clientMessage),
		closed:  make(chan struct{}),
	}
	sess.wio = &WebIO{srv: srv, pending: map[string]chan askResult{}, sessionClosed: sess.closed}
	// Wire the session-list push callback so WebIO can refresh the frontend
	// session menu/count when the current session changes (FEATURE-387).
	sess.wio.pushSessionList = sess.pushSessionList
	deps.Ag.SetIO(sess.wio)

	srv.SetMessageHandler(sess.handleMessage)
	srv.SetDisconnectHook(sess.wio.failAll)
	srv.SetPlanProvider(sess.currentPlanJSON)
	srv.SetModelInfoProvider(sess.ag.ModelInfo)
	return sess, nil
}

// handleMessage dispatches one browser message (input / answer / interrupt /
// session_list / session_switch / session_delete).
func (s *WebSession) handleMessage(msg clientMessage) {
	switch msg.Type {
	case "input":
		select {
		case s.inputCh <- msg:
		case <-s.closed:
		}
	case "answer":
		s.wio.resolve(msg.ID, msg.Value)
	case "interaction_answer":
		if msg.Result != nil {
			s.wio.resolveInteraction(msg.ID, agent.InteractionResult{
				Action: agent.InteractionResultAction(msg.Result.Action),
				Value:  msg.Result.Value,
				Raw:    msg.Result.Raw,
			})
		}
	case "interrupt":
		s.ag.Interrupt()
	case "session_list":
		s.pushSessionList()
	case "session_switch":
		s.switchSession(msg.Value)
	case "session_delete":
		s.deleteSession(msg.Value)
	}
}

// pushSessionList sends the current session list to the browser (FEATURE-387).
func (s *WebSession) pushSessionList() {
	entries, err := s.ag.Store().ListNamedSessions()
	if err != nil {
		return
	}
	currentID := s.ag.CurrentSessionID()
	infos := make([]sessionInfo, 0, len(entries))
	for _, e := range entries {
		infos = append(infos, sessionInfo{
			ID:        e.ID,
			Title:     e.Title,
			Keywords:  e.Keywords,
			CreatedAt: e.CreatedAt.Format("2006-01-02 15:04"),
			Current:   e.ID == currentID,
		})
	}
	s.srv.sendJSON(serverMessage{Kind: "sessions", Sessions: infos})
}

// switchSession switches the current session to the target ID (FEATURE-387).
func (s *WebSession) switchSession(id string) {
	if id == "" {
		return
	}
	s.ag.SetCurrentSessionID(id)
	if err := s.ag.Store().SaveCurrentSessionID(id); err != nil {
		log.Warn("switchSession SaveCurrentSessionID: %v", err)
	}
	// SetCurrentSessionID already pushes the updated session list (FEATURE-387),
	// so the frontend reflects the new current without an extra push here.
}

// deleteSession deletes a named session by ID (FEATURE-387). The current
// session is protected from deletion.
func (s *WebSession) deleteSession(id string) {
	if id == "" {
		return
	}
	// The current session is protected from deletion; still push the list so
	// the frontend reflects that the delete was rejected.
	if id == s.ag.CurrentSessionID() {
		s.pushSessionList()
		return
	}
	if err := s.ag.Store().DeleteNamedSession(id); err != nil {
		log.Warn("deleteSession: %v", err)
		return
	}
	s.pushSessionList()
}

// currentPlanJSON returns the current task plan as a JSON string ("" when
// no plan exists), for the state push on client connect.
func (s *WebSession) currentPlanJSON() string {
	plan, err := s.ag.TaskPlanManager().GetCurrent()
	if err != nil || plan == nil {
		return ""
	}
	return taskPlanJSON(plan)
}

// ReadLine waits for the next browser "input" message. Attachments (image
// paths) are installed on the agent before returning, mirroring the CLI
// --image flag path. Turn boundaries are signalled to the browser around
// the wait: await_input before blocking, turn_start once an input arrives.
func (s *WebSession) ReadLine(prompt string) (string, error) {
	s.srv.sendEvent(agent.NewStreamEvent(eventAwaitInput, agent.ChannelSystem, agent.LevelInfo, ""))
	select {
	case msg := <-s.inputCh:
		s.srv.sendEvent(agent.NewStreamEvent(eventTurnStart, agent.ChannelSystem, agent.LevelInfo, ""))
		if len(msg.Attachments) > 0 {
			paths := make([]string, 0, len(msg.Attachments))
			for _, rel := range msg.Attachments {
				if abs, err := s.srv.resolvePath(rel); err == nil {
					paths = append(paths, abs)
				}
			}
			if len(paths) > 0 {
				s.ag.SetImagePaths(paths)
			}
		}
		return strings.TrimSpace(msg.Text), nil
	case <-s.closed:
		return "", io.EOF
	}
}

// Acquire wires the per-run renderer. The WebIO stays installed for the
// whole session (see newWebSession), so release is a no-op.
func (s *WebSession) Acquire(ag *agent.Agent) (agent.EventRenderer, func()) {
	return &WebRenderer{srv: s.srv}, func() {}
}

// Interactive reports false: the page provides its own UI, so all terminal
// decorations (welcome banner, prompt, Said line) stay suppressed.
func (s *WebSession) Interactive() bool { return false }

// Close releases the session (REPL shutdown).
func (s *WebSession) Close() error {
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	return nil
}

// ---------------------------------------------------------------------------
// WebIO: agent.UserIO over the WebSocket ask/answer protocol
// ---------------------------------------------------------------------------

// askResult is the resolution of one pending ask request.
type askResult struct {
	value  string
	result agent.InteractionResult
	err    error
}

// WebIO implements agent.UserIO for the browser session. Print* calls are
// pushed as ui_text events (prompts appear naturally in the event stream);
// ReadLine/ReadKey send an ask message and block until the browser answers.
type WebIO struct {
	srv           *Server
	sessionClosed chan struct{}

	// pushSessionList refreshes the frontend session menu/count. Set by
	// newWebSession to WebSession.pushSessionList (FEATURE-387).
	pushSessionList func()

	mu      sync.Mutex
	pending map[string]chan askResult

	seq     atomic.Uint64
	reading atomic.Bool
}

// pushText wraps user-interface text into a ui_text stream event (repl
// channel) and sends it to the browser, so REPL builtin command output
// (:set, :mcp, ...) renders as a REPL block in the event stream.
func (w *WebIO) pushText(text string) {
	w.srv.sendEvent(agent.NewStreamEvent("ui_text", agent.ChannelREPL, agent.LevelInfo, text))
}

// PushTaskPlan implements agent.TaskPlanPusher: it pushes a task_plan event
// to the browser so the frontend refreshes its plan panel. Called by the
// agent when the session changes (FEATURE-386). planJSON is the full task
// plan snapshot as JSON ("" when archived/cleared).
func (w *WebIO) PushTaskPlan(planJSON string) {
	w.srv.sendEvent(agent.TaskPlanEvent(planJSON))
}

// PushSessionList implements agent.SessionListPusher: it refreshes the
// frontend session menu/count. Called by the agent when the current session
// changes (FEATURE-387), covering paths like :new that switch the session
// outside the web session handler.
func (w *WebIO) PushSessionList() {
	if w.pushSessionList != nil {
		w.pushSessionList()
	}
}

func (w *WebIO) Print(args ...interface{})                 { w.pushText(fmt.Sprint(args...)) }
func (w *WebIO) Printf(format string, args ...interface{}) { w.pushText(fmt.Sprintf(format, args...)) }
func (w *WebIO) Println(args ...interface{}) {
	w.pushText(fmt.Sprintln(args...))
}
func (w *WebIO) ErrPrintf(format string, args ...interface{}) {
	w.pushText(fmt.Sprintf(format, args...))
}

// ReadLine asks the browser for one line of input (ask mode "line").
func (w *WebIO) ReadLine() (string, error) {
	return w.ask("line")
}

// ReadKey asks the browser for a single key (ask mode "key"); an empty
// answer maps to Enter.
func (w *WebIO) ReadKey() (byte, error) {
	v, err := w.ask("key")
	if err != nil {
		return 0, err
	}
	if v == "" {
		return '\n', nil
	}
	return v[0], nil
}

// IsReading reports whether an ask request is currently awaiting an answer.
func (w *WebIO) IsReading() bool { return w.reading.Load() }

// Ask implements agent.InteractionManager for the browser session. It pushes
// a structured interaction request and blocks until the browser answers with
// an interaction_answer (FEATURE-388).
func (w *WebIO) Ask(ctx context.Context, in agent.Interaction) (agent.InteractionResult, error) {
	id := fmt.Sprintf("ask-%d", w.seq.Add(1))
	ch := make(chan askResult, 1)
	w.mu.Lock()
	w.pending[id] = ch
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.pending, id)
		w.mu.Unlock()
	}()

	w.reading.Store(true)
	defer w.reading.Store(false)

	inJSON, err := json.Marshal(in)
	if err != nil {
		return agent.InteractionResult{}, err
	}
	if !w.srv.sendInteraction(id, inJSON) {
		return agent.InteractionResult{}, errNoWebClient
	}
	select {
	case res := <-ch:
		return res.result, res.err
	case <-w.sessionClosed:
		return agent.InteractionResult{}, io.EOF
	}
}

// ask sends one ask message and blocks for the matching answer.
func (w *WebIO) ask(mode string) (string, error) {
	id := fmt.Sprintf("ask-%d", w.seq.Add(1))
	ch := make(chan askResult, 1)
	w.mu.Lock()
	w.pending[id] = ch
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.pending, id)
		w.mu.Unlock()
	}()

	w.reading.Store(true)
	defer w.reading.Store(false)

	if !w.srv.sendAsk(id, mode) {
		return "", errNoWebClient
	}
	select {
	case res := <-ch:
		return res.value, res.err
	case <-w.sessionClosed:
		return "", io.EOF
	}
}

// resolve delivers a browser answer to the waiting ask.
func (w *WebIO) resolve(id, value string) {
	w.mu.Lock()
	ch, ok := w.pending[id]
	w.mu.Unlock()
	if ok {
		ch <- askResult{value: value}
	}
}

// resolveInteraction delivers a browser interaction_answer to the waiting Ask.
func (w *WebIO) resolveInteraction(id string, res agent.InteractionResult) {
	w.mu.Lock()
	ch, ok := w.pending[id]
	w.mu.Unlock()
	if ok {
		ch <- askResult{result: res}
	}
}

// failAll fails every pending ask (client disconnected or was replaced), so
// the agent never blocks on an answer that can no longer arrive.
func (w *WebIO) failAll() {
	w.mu.Lock()
	pending := w.pending
	w.pending = map[string]chan askResult{}
	w.mu.Unlock()
	for _, ch := range pending {
		ch <- askResult{err: errNoWebClient}
	}
}

// ---------------------------------------------------------------------------
// WebRenderer: agent.EventRenderer pushing events to the browser as-is
// ---------------------------------------------------------------------------

// WebRenderer forwards every stream event to the browser; partitioning is
// done by the frontend based on Type/Chan/Level.
type WebRenderer struct {
	srv *Server
}

// Render pushes one event. task_plan and all other event types pass through
// uniformly; the frontend decides how to display them.
func (r *WebRenderer) Render(ev agent.StreamEvent) {
	r.srv.sendEvent(ev)
}

// taskPlanJSON marshals a task plan snapshot for the state message.
func taskPlanJSON(plan interface{}) string {
	data, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	return string(data)
}
