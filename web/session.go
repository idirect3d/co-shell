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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/store"
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

	// eventUserEcho records the user's own message in the session's event stream
	// (FEATURE-548). It is persisted only — never forwarded live — because the
	// browser already renders the YOU block from its local echo when the input is
	// sent; replaying the event rebuilds that same block.
	eventUserEcho = "user_echo"

	// FEATURE-542: interactive requests and their resolutions are first-class
	// persisted events (GAP-1). A browser renders its dialogs from these events
	// instead of from direct ask/interaction frames, so several ends stay in sync
	// and a reconnecting browser replays whatever is still unanswered (R4).
	eventInteractionRequest  = "interaction_request"  // {id, source, mode|interaction}
	eventInteractionResolved = "interaction_resolved" // {id, resolved_by, answer|result|cancelled}
)

// SessionFactory returns the repl session factory bound to this server.
// main.go registers it via repl.RegisterSessionFactory("web", ...).
func (s *Server) SessionFactory() func(repl.SessionDeps) (repl.SessionIO, error) {
	return func(deps repl.SessionDeps) (repl.SessionIO, error) {
		return newWebSession(s, deps)
	}
}

// inputQueue is the explicit FIFO queue of browser inputs (FEATURE-542, GAP-3).
//
// The previous unbuffered channel coupled the sender to the receiver: once
// several tabs were connected each read loop became an independent sender and
// Go picks a ready sender at random, so "commands run serially in seq order"
// could not be guaranteed. Enqueue order is now fixed under a mutex and every
// entry keeps the seq it was assigned plus the label of the端 it came from.
type inputQueue struct {
	mu     sync.Mutex
	seq    int
	items  []queuedInput
	notify chan struct{} // buffered(1): wakes a waiting ReadLine
}

// queuedInput is one queued browser input.
type queuedInput struct {
	seq    int
	source string // human-readable source label ("端 1"); "" for internal input
	msg    clientMessage
}

// newInputQueue creates an empty queue.
func newInputQueue() *inputQueue {
	return &inputQueue{notify: make(chan struct{}, 1)}
}

// push appends msg at the tail and wakes a waiting reader. The assigned seq is
// recorded on the message as well, so the rest of the pipeline can order and
// label it. push never blocks.
func (q *inputQueue) push(msg clientMessage) int {
	q.mu.Lock()
	q.seq++
	msg.QueueSeq = q.seq
	q.items = append(q.items, queuedInput{seq: q.seq, source: msg.Source, msg: msg})
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return q.seq
}

// pop removes and returns the oldest entry (FIFO).
func (q *inputQueue) pop() (queuedInput, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return queuedInput{}, false
	}
	it := q.items[0]
	q.items = q.items[1:]
	return it, true
}

// WebSession is the browser-backed SessionIO (FEATURE-307c). Input lines
// arrive as WebSocket "input" messages; interrupt requests map to
// agent.Interrupt (the ESC equivalent).
type WebSession struct {
	srv *Server
	ag  *agent.Agent
	wio *WebIO

	// settings handles settings_get/settings_set messages (FEATURE-391).
	// It is nil when the session was created without a SettingsHandler
	// (e.g. in tests), in which case settings messages are ignored.
	settings *cmd.SettingsHandler
	// mcp handles mcp_get/mcp_add/mcp_update/mcp_remove messages (FEATURE-464).
	// It is nil when the session was created without an MCPHandler (e.g. in
	// tests), in which case MCP messages are ignored.
	mcp *cmd.MCPHandler
	// session handles :session pop to for the retry-from block action
	// (FEATURE-409).
	session *cmd.SessionHandler

	// mode handles the work-mode switcher (FEATURE-410): mode_get returns
	// the current mode + all available modes, mode_switch switches mode.
	mode *cmd.ModeHandler

	// model handles the model manager (FEATURE-422): model_get returns the
	// model/template lists, model_switch/enable/disable/remove/set_priority
	// mutate models, model_add/model_edit launch the wizards, and
	// model_wizard_cancel aborts a running wizard.
	model *cmd.ModelHandler

	// msgIndex is the current turn counter, incremented on each user input.
	// It is used for token-statistics display (iteration numbering).
	// NOTE: it is NOT the message array index — see msgIndexForRetry.
	msgIndex int

	inputQ *inputQueue

	// lastInputSource / lastInputSourceID identify the端 whose instruction is
	// currently being processed (FEATURE-542). The label is echoed into the YOU
	// block of the recorded turn and into that turn's interaction_request events;
	// the id lets the originating browser recognise (and skip) its own echo when
	// the event is broadcast to every end.
	lastInputSource   string
	lastInputSourceID string
	// pendingUserEcho holds the text of the user message that started the current
	// turn (FEATURE-548). It is persisted to the session's event stream by the
	// WebRenderer on the turn's first event, when the message's index in the
	// message array is known — see WebRenderer.persistPendingUserEcho.
	pendingUserEcho string
	closed          chan struct{}

	// boardAgg merges board messages that arrive within a short window so a
	// burst delivered while the agent is busy is injected as one block
	// (FEATURE-530). Nil when the board switch is off.
	boardAgg *agent.BoardAggregator
}

// newWebSession creates the session, installs the WebIO on the agent for
// the whole session lifetime (builtin command output must stay visible in
// the browser), and registers the message handler on the server.
func newWebSession(srv *Server, deps repl.SessionDeps) (*WebSession, error) {
	if deps.Ag == nil {
		return nil, errors.New("web session requires SessionDeps.Ag")
	}
	sess := &WebSession{
		srv:      srv,
		ag:       deps.Ag,
		settings: deps.SettingsHandler,
		mcp:      deps.MCPHandler,
		session:  cmd.NewSessionHandler(deps.Ag, deps.Cfg),
		mode:     cmd.NewModeHandler(deps.Cfg, deps.Ag),
		model:    cmd.NewModelHandler(deps.Cfg, deps.Ag),
		inputQ:   newInputQueue(),
		closed:   make(chan struct{}),
	}
	sess.wio = &WebIO{srv: srv, pending: map[string]chan askResult{}, sessionClosed: sess.closed}
	// Wire the session-list push callback so WebIO can refresh the frontend
	// session menu/count when the current session changes (FEATURE-387).
	sess.wio.pushSessionList = sess.pushSessionList	// FEATURE-566: let the agent start a turn for a sub-agent that finished on
	// its own while the agent was idle.
	sess.wio.startSubAgentTurn = sess.startSubAgentCompletionTurn
	// FEATURE-542 (GAP-1): interactive requests travel through the persisted
	// event stream. WebIO asks the session to write and fan out the events.
	sess.wio.emit = sess.emitInteractionEvent
	sess.wio.source = func() string { return sess.lastInputSource }
	deps.Ag.SetIO(sess.wio)
	// FEATURE-530: wire the message board (aggregator + hub registration).
	// FEATURE-538: the announced 职责 is the agent's "Agent描述" (IDENTITY.md),
	// read by the session itself so edits are picked up without a restart.
	sess.initBoard()
	// FEATURE-530: install the board sender so board_send can push frames back
	// to the hub over the same WebSocket connection. The hub is the WS client
	// that connected to us.
	deps.Ag.SetBoardSender(func(msg map[string]interface{}) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		return srv.sendRaw(data)
	})

	srv.SetMessageHandler(sess.handleMessage)
	// FEATURE-542 R7: a browser that disconnects is only "temporarily
	// unreachable". The old disconnect hook failed every pending interaction
	// with errNoWebClient, which corrupted the running task; the connection is
	// now simply dropped from the fan-out list and parked interactions keep
	// waiting (they are released by an answer or by the pause/stop key).
	// FEATURE-542 (§9.5): tell the agent whether a user is actually reachable.
	// Only a directly connected browser counts — the hub link does not, because
	// nobody may be behind it (design §9.2).
	deps.Ag.SetUserPresenceProvider(srv.HasClient)
	srv.SetPlanProvider(sess.currentPlanJSON)
	// FEATURE-564: the sub-agent list below the task plan needs the same
	// snapshot, otherwise a page load or reconnect shows the plan but no
	// sub-agents until the next sub-agent event.
	srv.SetSubAgentsProvider(sess.ag.SubAgentsSnapshot)
	srv.SetModelInfoProvider(sess.ag.ModelInfo)
	// FEATURE-499: expose the agent busy state via GET /api/status for the hub.
	srv.SetBusyProvider(sess.ag.IsBusy)
	return sess, nil
}

// handleMessage dispatches one browser message (input / answer / interrupt /
// session_list / session_switch / session_delete).
func (s *WebSession) handleMessage(msg clientMessage) {
	switch msg.Type {
	case "input":
		// FEATURE-524: a message typed while render_ui(waiting=true) is parked
		// releases that wait (the tool reports that the user did not interact),
		// and the message itself then starts the next turn as usual.
		s.ag.ReleaseUIWait()
		// FEATURE-542 (GAP-3): enqueue on the explicit FIFO queue. push never
		// blocks, so the read loop of one tab can never stall the others.
		s.inputQ.push(msg)
	case "ui_action":
		// FEATURE-524: a component interaction (form submit, chart drill-down,
		// button action) starts a fresh user turn. The action and its structured
		// payload are injected as the user message so the agent continues with
		// exactly the data the user provided (decision 5: continuation semantics).
		if msg.UIID == "" || msg.UIActionID == "" {
			log.Warn("session: ui_action ignored (ui_id=%q action_id=%q)", msg.UIID, msg.UIActionID)
			return
		}
		// FEATURE-524: while a render_ui(waiting=true) call is parked, the action
		// is that call's result and the agent continues in the same turn;
		// otherwise it starts a fresh turn as described above.
		if s.ag.SubmitUIAction(msg.UIID, msg.UIActionID, msg.Payload) {
			return
		}
		// FEATURE-524 window mode (Q3-A): while a turn is running, the action is
		// injected into that turn -- the agent sees it before its next LLM call --
		// instead of starting a new one, so a window stays interactive while the
		// agent keeps working. The dynamic queue survives a turn boundary: an
		// action that arrives just as the turn ends is injected at the start of
		// the next turn instead of being lost.
		if s.ag.IsBusy() {
			// A blocking action that finds no parked wait cannot be returned as a
			// tool result. It degrades to the non-blocking path on purpose: better
			// an immediate in-turn delivery than hanging until the wait deadline.
			if msg.Blocking {
				log.Warn("session: blocking ui_action %q/%q has no parked wait; injected into the running turn", msg.UIID, msg.UIActionID)
			}
			s.ag.AddDynamicEvent(agent.DynamicUIAction, agent.UIActionMessage(msg.UIID, msg.UIActionID, msg.Payload))
			return
		}
		s.inputQ.push(clientMessage{
			Type:     "input",
			Text:     agent.UIActionMessage(msg.UIID, msg.UIActionID, msg.Payload),
			Source:   msg.Source,
			SourceID: msg.SourceID,
		})
	case "viewport":
		// FEATURE-524: the browser reports its window size (CSS pixels) once it
		// connects and again on resize, so <runtime_info><viewport> can tell the
		// LLM how much room the current surface has. Invalid sizes are ignored.
		s.ag.SetViewport(msg.ViewportW, msg.ViewportH)
	case "ui_window_state":
		// FEATURE-571: the frontend mirrors every window change (✕/Esc close,
		// tab switch, surface flip) and sends the full list on connect, so the
		// agent-side registry behind <ui_windows> matches what the user sees.
		s.ag.SyncUIWindowsPayload(msg.Windows, msg.Closed)
	case "dynamic_event":
		// FEATURE-471: a user-action event reported while a task is running
		// (clip_object / upload_file / user_message / open_file). Enqueue it
		// into the agent's dynamic perception queue so the next tool/user
		// message injection surfaces it to the LLM.
		// FEATURE-611: in `full` mode a supplemental user message is delivered
		// as a complete user turn (queued for the next ReadLine) instead of a
		// dynamic-perception event.
		kind := agent.DynamicEventKind(msg.Kind)
		if kind == agent.DynamicUserMessage && s.ag.UserMessageMode() == agent.UserMessageModeFull {
			s.inputQ.push(clientMessage{Type: "input", Text: msg.Value, Source: msg.Source, SourceID: msg.SourceID})
			return
		}
		s.ag.AddDynamicEvent(kind, msg.Value)
	case "board_registered", "board_roster", "board_sent", "board_message", "board_timeout":
		// FEATURE-530: flat board frames pushed by the hub.
		s.handleBoardFrame(msg)
	case "answer":
		// FEATURE-542: the answering端 is recorded in the resolution event.
		s.wio.resolve(msg.ID, msg.Value, msg.Source)
	case "interaction_answer":
		// FIX-513: decode the browser result straight into agent.InteractionResult
		// so every field survives the wire — the previous hand-written mirror
		// struct silently dropped newer fields (notably Answers for multi-question
		// forms), which left ask_user with no answer at all.
		if len(msg.Result) > 0 {
			var res agent.InteractionResult
			if err := json.Unmarshal(msg.Result, &res); err != nil {
				log.Warn("interaction_answer: invalid result JSON: %v", err)
				break
			}
			s.wio.resolveInteraction(msg.ID, res, msg.Source)
		}
	case "interrupt":
		if s.ag.IsToolRunning() {
			// FEATURE-582 Q3-A: during tool execution ESC / the pause button
			// interrupts the tool immediately (kills its process groups) and
			// returns to the prompt, with no confirmation prompt.
			s.ag.InterruptTool("web-interrupt")
		} else {
			// FEATURE-201/427: during LLM output ESC keeps the existing
			// "pause the stream and ask" semantics.
			s.ag.Interrupt()
		}
		// FEATURE-524: ESC also ends a parked render_ui(waiting=true) instead of
		// leaving the turn blocked until the timeout.
		s.ag.ReleaseUIWait()
		// FEATURE-542 GAP-2/R8: the pause/stop key must also release a parked
		// interaction (anything waiting for human input) with "user cancelled"
		// semantics — the old failAll only did that by accident on disconnect.
		s.wio.cancelAll(msg.Source)
	case "restart":
		// FEATURE-398: send a restart signal to the current process so an
		// external supervisor (launchd/systemd) restarts the service.
		requestRestart()
	case "session_list":
		s.pushSessionList()
	case "session_switch":
		s.switchSessionRequest(msg.Value)
	case "session_delete":
		s.deleteSession(msg.Value)
	case "session_rename":
		s.renameSession(msg.Value)
	case "session_new":
		s.newSession()
	case "history_get":
		// FEATURE-507: the browser asks for a page of persisted UI events so it
		// can replay history after a refresh. Nothing is cached in memory.
		// FEATURE-542: with a cursor (after > 0) the client only wants what it
		// missed, so those events are replayed as ordinary frames and appended to
		// the stream it already holds (no reset, no duplicate blocks).
		if msg.After > 0 {
			s.pushHistoryAfter(msg.Value, msg.After, msg.Count)
			return
		}
		s.pushHistory(msg.Value, msg.Count, msg.Before)
	case "session_pop":
		// FEATURE-409: retry-from — pop the session back to the given message
		// index (equivalent to :session pop to N).
		s.popTo(msg.Value)
	case "settings_get":
		s.handleSettingsGet()
	case "settings_set":
		s.handleSettingsSet(msg.Key, msg.Value)
	case "mcp_get":
		s.handleMCPGet()
	case "mcp_add":
		s.handleMCPAdd(msg.Name, msg.Command, msg.Args, msg.URL)
	case "mcp_update":
		s.handleMCPUpdate(msg.Name, msg.Command, msg.Args, msg.Enabled, msg.URL)
	case "mcp_remove":
		s.handleMCPRemove(msg.Name)
	case "mcp_test":
		s.handleMCPTest(msg.Name)
	case "identity_get":
		s.handleIdentityGet()
	case "identity_set":
		s.handleIdentitySet(msg.Key, msg.Value)
	case "mode_get":
		s.handleModeGet()
	case "mode_switch":
		s.handleModeSwitch(msg.Value)
	case "model_get":
		s.handleModelGet()
	case "model_switch":
		s.handleModelAction("switch", msg.Value)
	case "model_enable":
		s.handleModelAction("enable", msg.Value)
	case "model_disable":
		s.handleModelAction("disable", msg.Value)
	case "model_remove":
		s.handleModelAction("remove", msg.Value)
	case "model_set_priority":
		s.handleModelSetPriority(msg.Value, msg.Priority)
	case "model_auto_select":
		s.handleModelAutoSelect(msg.AutoSelect)
	case "model_unbind":
		s.handleModelUnbind(msg.Value)
	case "model_bind":
		s.handleModelBind(msg.Key, msg.Value)
	case "model_add":
		s.handleModelWizard("add", "")
	case "model_edit":
		s.handleModelWizard("edit", msg.Value)
	case "model_wizard_cancel":
		s.handleModelWizardCancel()
	case "model_wizard_start":
		s.handleModelWizardStart(msg.Value)
	case "model_wizard_next":
		s.handleModelWizardNext(msg.Step, msg.WizardData)
	case "model_wizard_prev":
		s.handleModelWizardPrev(msg.Step, msg.WizardData)
	case "model_wizard_refresh":
		s.handleModelWizardRefresh(msg.Step, msg.WizardData)
	case "model_wizard_detect":
		s.handleModelWizardDetect(msg.WizardData)
	case "model_wizard_submit":
		s.handleModelWizardSubmit(msg.WizardData)
	case "yolo_set":
		s.handleYOLOSet(msg.YOLO)
	case "yolo_get":
		s.handleYOLOGet()
	}
}

// handleSettingsGet sends the current settings (grouped setting items) to the
// browser (FEATURE-391).
func (s *WebSession) handleSettingsGet() {
	if s.settings == nil {
		return
	}
	groups := s.settings.SettingsJSON()
	raw, err := json.Marshal(groups)
	if err != nil {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "settings", Settings: raw})
}

// handleSettingsSet applies a setting change by delegating to the SettingsHandler
// and reports the result to the browser (FEATURE-391).
func (s *WebSession) handleSettingsSet(key, value string) {
	if s.settings == nil || key == "" {
		return
	}
	result, err := s.settings.Handle([]string{key, value})
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "settings_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "settings_result", OK: true, Message: result})
}

// handleMCPGet sends the current MCP server list to the browser (FEATURE-464).
func (s *WebSession) handleMCPGet() {
	if s.mcp == nil {
		return
	}
	servers := s.mcp.MCPServersJSON()
	raw, err := json.Marshal(servers)
	if err != nil {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mcp", MCPServers: raw})
}

// handleMCPAdd adds a new MCP server from the browser (FEATURE-464).
func (s *WebSession) handleMCPAdd(name, command string, args []string, url string) {
	if s.mcp == nil || name == "" {
		return
	}
	result, err := s.mcp.AddServerJSON(name, command, args, url)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: true, Message: result})
	s.handleMCPGet()
}

// handleMCPUpdate updates an existing MCP server from the browser (FEATURE-464).
func (s *WebSession) handleMCPUpdate(name, command string, args []string, enabled bool, url string) {
	if s.mcp == nil || name == "" {
		return
	}
	result, err := s.mcp.UpdateServerJSON(name, command, args, enabled, url)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: true, Message: result})
	s.handleMCPGet()
}

// handleMCPRemove removes an MCP server from the browser (FEATURE-464).
func (s *WebSession) handleMCPRemove(name string) {
	if s.mcp == nil || name == "" {
		return
	}
	result, err := s.mcp.RemoveServerJSON(name)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: true, Message: result})
	s.handleMCPGet()
}

// handleMCPTest reconnects to an MCP server (connectivity test) and refreshes
// its tool list, then pushes the updated server list to the browser
// (FEATURE-498 ext).
func (s *WebSession) handleMCPTest(name string) {
	if s.mcp == nil || name == "" {
		return
	}
	if _, err := s.mcp.TestServerJSON(name); err != nil {
		s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mcp_result", OK: true, Message: name})
	s.handleMCPGet()
}

// browser (FEATURE-393).
func (s *WebSession) handleIdentityGet() {
	if s.settings == nil {
		return
	}
	fields := s.settings.IdentityJSON()
	raw, err := json.Marshal(fields)
	if err != nil {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "identity", Identity: raw})
}

// handleIdentitySet persists a single identity & personality field and reports
// the result to the browser (FEATURE-393).
func (s *WebSession) handleIdentitySet(key, value string) {
	if s.settings == nil || key == "" {
		return
	}
	if err := s.settings.SaveIdentity(key, value); err != nil {
		s.srv.sendJSON(serverMessage{Kind: "identity_result", OK: false, Message: err.Error()})
		return
	}
	// FIX-544: the saved key travels in its own field. It used to be stuffed
	// into Message, which the browser rendered verbatim - the user saw a raw key
	// like "name" instead of a readable confirmation.
	s.srv.sendJSON(serverMessage{Kind: "identity_result", OK: true, Key: key})
}

// handleModeGet sends the current work mode and all available modes to the
// browser for the mode switcher (FEATURE-410).
func (s *WebSession) handleModeGet() {
	if s.mode == nil {
		return
	}
	current := s.mode.CurrentMode()
	modes := s.mode.ListModes()
	infos := make([]modeInfo, 0, len(modes))
	for _, m := range modes {
		infos = append(infos, modeInfo{
			Name:        m.Name,
			Description: m.Description,
			Current:     m.Name == current,
		})
	}
	s.srv.sendJSON(serverMessage{Kind: "mode", Modes: infos})
}

// handleModeSwitch switches the active work mode and reports the result to
// the browser (FEATURE-410).
func (s *WebSession) handleModeSwitch(name string) {
	if s.mode == nil || name == "" {
		return
	}
	result, err := s.mode.Handle([]string{"switch", name})
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "mode_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "mode_result", OK: true, Message: result})
}

// handleModelGet sends the configured models and built-in templates to the
// browser for the model manager (FEATURE-422).
func (s *WebSession) handleModelGet() {
	if s.model == nil {
		return
	}
	models, templates := s.model.ModelWebJSON()
	mRaw, err := json.Marshal(models)
	if err != nil {
		return
	}
	tRaw, err := json.Marshal(templates)
	if err != nil {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "models", Models: mRaw, Templates: tRaw, AutoSelect: s.model.ModelAutoSelect()})
}

// handleModelAction applies a non-interactive model operation (switch / enable
// / disable / remove) and reports the result to the browser (FEATURE-422).
func (s *WebSession) handleModelAction(action, id string) {
	if s.model == nil || id == "" {
		return
	}
	var result string
	var err error
	switch action {
	case "switch":
		result, err = s.model.ModelSwitch(id)
	case "enable":
		result, err = s.model.ModelEnable(id)
	case "disable":
		result, err = s.model.ModelDisable(id)
	case "remove":
		result, err = s.model.ModelRemove(id)
	default:
		return
	}
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	// Refresh the model list so the frontend reflects the change.
	s.handleModelGet()
}

// handleModelAutoSelect persists the automatic-selection (degrade-on-failure)
// switch and refreshes the model list so the browser shows the new state
// (FEATURE-602).
func (s *WebSession) handleModelAutoSelect(on bool) {
	if s.model == nil {
		return
	}
	result, err := s.model.ModelSetAutoSelect(on)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	s.handleModelGet()
}

// handleModelSetPriority sets a model's priority and reports the result
// (FEATURE-422).
func (s *WebSession) handleModelSetPriority(id string, priority int) {
	if s.model == nil || id == "" {
		return
	}
	result, err := s.model.ModelSetPriority(id, priority)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	s.handleModelGet()
}

// handleModelWizard launches the add/edit model wizard. The wizard runs through
// the agent's WebIO, so its prompts and input requests flow to the browser as
// ui_text / ask / interaction messages (FEATURE-422).
func (s *WebSession) handleModelWizard(mode, id string) {
	if s.model == nil {
		return
	}
	var result string
	var err error
	if mode == "edit" {
		result, err = s.model.StartEditWizard(id)
	} else {
		result, err = s.model.StartAddWizard()
	}
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	s.handleModelGet()
}

// handleModelWizardCancel aborts a running model wizard by failing all pending
// ask/interaction requests. The wizard's ReadLine then returns an error, which
// readLine() maps to wizardCancel so the wizard exits from any step (FEATURE-422).
func (s *WebSession) handleModelWizardCancel() {
	s.wio.cancelAll("system")
}

// handleModelWizardStart begins the structured add/edit wizard (FEATURE-429)
// and pushes the first step's form to the browser.
func (s *WebSession) handleModelWizardStart(id string) {
	if s.model == nil {
		return
	}
	mode := "add"
	if id != "" {
		mode = "edit"
	}
	step, data, err := s.model.WebWizardStart(mode, id)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.sendWizardStep(step, data)
}

// handleModelWizardNext advances the structured wizard to the next step.
func (s *WebSession) handleModelWizardNext(step string, raw json.RawMessage) {
	if s.model == nil {
		return
	}
	data, err := decodeWizardData(raw)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	next, err := s.model.WebWizardNext(data, cmd.WebWizardStep(step))
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.sendWizardStep(next, data)
}

// handleModelWizardPrev goes back to the previous step of the structured wizard.
func (s *WebSession) handleModelWizardPrev(step string, raw json.RawMessage) {
	if s.model == nil {
		return
	}
	data, err := decodeWizardData(raw)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	prev, err := s.model.WebWizardPrev(data, cmd.WebWizardStep(step))
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.sendWizardStep(prev, data)
}

// handleModelWizardRefresh re-renders the current step's form (used by the
// "refresh model list" button on the model_name step).
func (s *WebSession) handleModelWizardRefresh(step string, raw json.RawMessage) {
	if s.model == nil {
		return
	}
	data, err := decodeWizardData(raw)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	stepData, err := s.model.WebWizardRefresh(data, cmd.WebWizardStep(step))
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.sendWizardStep(stepData, data)
}

// handleModelWizardDetect forces a capability re-detection on the capabilities
// step of the structured wizard (FEATURE-601: the manual "detect" button). The
// backend reuses the same detectModelCapabilities path as the automatic
// detection when adding a model, then re-renders the step with the result.
func (s *WebSession) handleModelWizardDetect(raw json.RawMessage) {
	if s.model == nil {
		return
	}
	data, err := decodeWizardData(raw)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	// Stream per-capability progress to the browser while the detection runs
	// (FEATURE-601 follow-up), then clear the callback.
	s.model.SetDetectProgress(func(cap, phase string, ok bool) {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard_detect_progress", Cap: cap, Phase: phase, OK: ok})
	})
	defer s.model.SetDetectProgress(nil)
	stepData, err := s.model.WebWizardDetect(data)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.sendWizardStep(stepData, data)
}

// handleModelWizardSubmit submits the structured wizard and saves the model.
func (s *WebSession) handleModelWizardSubmit(raw json.RawMessage) {
	if s.model == nil {
		return
	}
	data, err := decodeWizardData(raw)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	result, err := s.model.WebWizardSubmit(data)
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: true, Message: result})
	s.handleModelGet()
}

// handleYOLOSet applies the YOLO master switch state from the browser and
// reports the resulting state back (FEATURE-439).
func (s *WebSession) handleYOLOSet(on bool) {
	if s.ag == nil {
		return
	}
	s.ag.SetYOLO(on)
	s.srv.sendJSON(serverMessage{Kind: "yolo", YOLO: s.ag.IsYOLO()})
}

// handleYOLOGet sends the current YOLO master switch state to the browser
// (FEATURE-439).
func (s *WebSession) handleYOLOGet() {
	if s.ag == nil {
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "yolo", YOLO: s.ag.IsYOLO()})
}

// sendWizardStep pushes a wizard step form and the accumulated data to the
// browser (FEATURE-429).
func (s *WebSession) sendWizardStep(step *cmd.WebWizardStepData, data *cmd.WebWizardData) {
	stepRaw, _ := json.Marshal(step)
	dataRaw, _ := json.Marshal(data)
	s.srv.sendJSON(serverMessage{Kind: "model_wizard", OK: true, WizardStep: stepRaw, WizardData: dataRaw})
}

// decodeWizardData unmarshals the accumulated wizard data from the browser.
func decodeWizardData(raw json.RawMessage) (*cmd.WebWizardData, error) {
	if len(raw) == 0 {
		return &cmd.WebWizardData{}, nil
	}
	var data cmd.WebWizardData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// handleModelUnbind clears the current work mode's model binding for the given
// target ("text" or "vision"), restoring the global default model. It delegates
// to the mode handler's .mode <name> model <target> none command (FEATURE-422).
func (s *WebSession) handleModelUnbind(target string) {
	if s.mode == nil || (target != "text" && target != "vision") {
		return
	}
	result, err := s.mode.Handle([]string{s.mode.CurrentMode(), "model", target, "none"})
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	s.handleModelGet()
}

// handleModelBind binds the given model to the current work mode for the given
// target ("text" or "vision"), overriding the global default. It delegates to
// the mode handler's .mode <name> model <target> <id> command (FEATURE-422).
func (s *WebSession) handleModelBind(target, id string) {
	if s.mode == nil || (target != "text" && target != "vision") || id == "" {
		return
	}
	result, err := s.mode.Handle([]string{s.mode.CurrentMode(), "model", target, id})
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "model_result", OK: false, Message: err.Error()})
		return
	}
	s.srv.sendJSON(serverMessage{Kind: "model_result", OK: true, Message: result})
	s.handleModelGet()
}

// defaultHistoryMessages is how many message groups the browser replays after
// a refresh (FEATURE-507). It is only a fallback: the browser sends the
// configured page-buffer-size (FIX-509) and this value is used when the request
// carries no usable count.
const defaultHistoryMessages = 20

// pushHistory sends a page of persisted UI events to the browser (FEATURE-507).
//
// The page is measured in message groups (meta.msg_index), not raw events: the
// caller asks for the last N messages and gets every event belonging to them,
// which matches the user's notion of "the last 20 messages". Events are read
// straight from bbolt and never cached, so the backend keeps no history in
// memory. beforeSeq > 0 pages further back (events strictly older than it).
func (s *WebSession) pushHistory(sessionID string, count, beforeSeq int) {
	if sessionID == "" {
		sessionID = s.ag.CurrentSessionID()
	}
	events, hasMore, oldestSeq := loadHistoryPage(s.ag.Store(), sessionID, count, beforeSeq)
	s.srv.sendJSON(serverMessage{Kind: "history", Events: events, HasMore: hasMore, OldestSeq: oldestSeq})
}

// loadHistoryPage reads one page of persisted UI events: up to `count` message
// groups (meta.msg_index) whose events all sit just before beforeSeq, together
// with whether older events remain and the cursor for the next (older) page.
// beforeSeq == 0 starts from the newest event.
//
// FEATURE-548: grouping moved into the store (LoadEventsByGroups), which returns
// whole message groups. The previous implementation read a raw window of
// count*maxEventsPerMessage events here and stopped once it had seen `count`
// group keys, so a message whose events outnumbered that window was sent
// half-complete and the remainder reappeared as a second block on the next page.
func loadHistoryPage(st *store.DualStore, sessionID string, count, beforeSeq int) (events []json.RawMessage, hasMore bool, oldestSeq int) {
	if count <= 0 {
		count = defaultHistoryMessages
	}
	entries, hasMore, oldestSeq, err := st.LoadEventsByGroups(sessionID, count, beforeSeq)
	if err != nil {
		log.Warn("loadHistoryPage LoadEventsByGroups: %v", err)
		return nil, false, 0
	}
	for _, e := range entries {
		events = append(events, withEventSeq(e.Data, e.Seq))
	}
	return events, hasMore, oldestSeq
}

// withEventSeq stamps one persisted event with the store sequence it was stored
// under (FEATURE-542). The seq lives in the bbolt key rather than in the payload,
// so it is added on the way out; the client keeps it as the cursor for an
// incremental catch-up after a reconnect.
func withEventSeq(data json.RawMessage, seq int) json.RawMessage {
	var ev eventJSON
	if err := json.Unmarshal(data, &ev); err != nil {
		return data
	}
	ev.Seq = seq
	out, err := json.Marshal(ev)
	if err != nil {
		return data
	}
	return out
}

// pushHistoryAfter replays every persisted event strictly newer than afterSeq as
// ordinary event frames (FEATURE-542, R3). Reusing the live frame shape means the
// browser appends them through exactly the same path as a continuous read, so a
// reconnect and an uninterrupted session are indistinguishable.
func (s *WebSession) pushHistoryAfter(sessionID string, afterSeq, limit int) {
	if sessionID == "" {
		sessionID = s.ag.CurrentSessionID()
	}
	if limit <= 0 {
		limit = defaultHistoryMessages
	}
	// Bounded catch-up: a long disconnect may span many pages.
	const maxPages = 50
	for page := 0; page < maxPages; page++ {
		entries, hasMore, err := s.ag.Store().LoadEventsAfter(sessionID, afterSeq, limit)
		if err != nil {
			log.Warn("pushHistoryAfter: %v", err)
			return
		}
		if len(entries) == 0 {
			return
		}
		for _, e := range entries {
			var ev eventJSON
			if err := json.Unmarshal(e.Data, &ev); err != nil {
				continue
			}
			ev.Seq = e.Seq
			s.srv.sendJSON(serverMessage{Kind: "event", Event: &ev})
			afterSeq = e.Seq
		}
		if !hasMore {
			return
		}
	}
}

// eventGroupKey returns the message-group key of a persisted event.
//
// FEATURE-548: a thin wrapper over store.EventGroupKey, so paging here and
// group-aware trimming in the store agree on what a message group is.
func eventGroupKey(eventJSON []byte) string {
	return store.EventGroupKey(eventJSON)
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
			ID:           e.ID,
			Title:        e.Title,
			Keywords:     e.Keywords,
			CreatedAt:    e.CreatedAt.Format("2006-01-02 15:04"),
			Current:      e.ID == currentID,
			MessageCount: e.MessageCount,
		})
	}
	s.srv.sendJSON(serverMessage{Kind: "sessions", Sessions: infos})
}

// switchSessionRequest handles a browser session_switch request (FEATURE-548).
//
// A switch is refused while a task is running: every recorded event is written
// under whichever session is current when it arrives, so switching mid-task
// would append the running task's blocks to another session's record and leave
// both records incomplete.
func (s *WebSession) switchSessionRequest(id string) {
	if s.ag.IsBusy() {
		s.srv.sendJSON(serverMessage{Kind: "session_switch_result", OK: false, Message: "busy"})
		return
	}
	// FEATURE-565: the sub-agent pool, its working directories and the on-screen
	// list are all bound to the session that launched them, so leaving a session
	// whose sub-agents are still running would hide them and orphan their
	// results. Refuse the switch and tell the browser why (localized).
	if err := s.ag.RunningSubAgentsError(); err != nil {
		s.srv.sendJSON(serverMessage{Kind: "session_switch_result", OK: false, Message: err.Error()})
		return
	}
	s.switchSession(id)
}

// switchSession switches the current session to the target ID (FEATURE-387).
// It mirrors the REPL :session switch command (cmd/session.go handleSwitch):
// the current session's messages are flushed to the DB and the target
// session's messages are loaded into the agent's context, so :context reflects
// the switched session (FIX-407).
func (s *WebSession) switchSession(id string) {
	if id == "" {
		return
	}
	// Flush current session messages back to DB before switching.
	if err := s.ag.FlushCurrentSession(); err != nil {
		log.Warn("switchSession FlushCurrentSession: %v", err)
	}
	// Load the target session and swap its messages into the agent context.
	target, found, err := s.ag.Store().LoadNamedSession(id)
	if err != nil {
		log.Warn("switchSession LoadNamedSession: %v", err)
		return
	}
	if !found || target == nil {
		log.Warn("switchSession: session %q not found", id)
		return
	}
	var messages []llm.Message
	if err := json.Unmarshal(target.Messages, &messages); err != nil {
		log.Warn("switchSession unmarshal messages: %v", err)
		return
	}
	// Build full history with the current system prompt.
	current := s.ag.Messages()
	systemPrompt := ""
	if len(current) > 0 && current[0].Role == "system" {
		systemPrompt = current[0].Content
	}
	newMsgs := []llm.Message{{Role: "system", Content: systemPrompt}}
	newMsgs = append(newMsgs, messages...)
	s.ag.SetHistory(newMsgs)

	s.ag.SetCurrentSessionID(id)
	if err := s.ag.Store().SaveCurrentSessionID(id); err != nil {
		log.Warn("switchSession SaveCurrentSessionID: %v", err)
	}
	// SetCurrentSessionID already pushes the updated session list (FEATURE-387),
	// so the frontend reflects the new current without an extra push here.
	//
	// FEATURE-548: tell the browser which session is now current so it discards
	// the previous session's blocks and replays this one's recorded stream — the
	// record is per session, so the visible stream must follow the switch.
	s.srv.sendJSON(serverMessage{Kind: "session_switched", Message: id})
}

// newSession creates a new empty session and switches to it (FEATURE-401),
// mirroring the REPL :new command.
func (s *WebSession) newSession() {
	// FEATURE-548: refusing while busy keeps the running task's events in the
	// session they belong to (same reason as switchSessionRequest).
	if s.ag.IsBusy() {
		s.srv.sendJSON(serverMessage{Kind: "session_switch_result", OK: false, Message: "busy"})
		return
	}
	// FEATURE-565: starting a new session abandons the current one just like an
	// explicit switch does, so it is refused for the same reason (same check as
	// switchSessionRequest).
	if err := s.ag.RunningSubAgentsError(); err != nil {
		s.srv.sendJSON(serverMessage{Kind: "session_switch_result", OK: false, Message: err.Error()})
		return
	}
	if err := s.ag.FlushCurrentSession(); err != nil {
		log.Warn("newSession FlushCurrentSession: %v", err)
	}
	s.ag.Reset()
	now := time.Now()
	randBytes := make([]byte, 4)
	randBytes[0] = byte(now.Nanosecond() & 0xFF)
	randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
	randBytes[2] = byte(now.Second() & 0xFF)
	randBytes[3] = byte(now.Minute() & 0xFF)
	sessionID := fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
	title := s.nextDefaultSessionTitle()
	entry := &store.SessionEntry{
		ID:           sessionID,
		Title:        title,
		Keywords:     "",
		SystemPrompt: "",
		Messages:     []byte("[]"),
		MessageCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.ag.Store().SaveNamedSession(entry); err != nil {
		log.Warn("newSession SaveNamedSession: %v", err)
	}
	s.ag.SetCurrentSessionID(sessionID)
	if err := s.ag.Store().SaveCurrentSessionID(sessionID); err != nil {
		log.Warn("newSession SaveCurrentSessionID: %v", err)
	}
	// FEATURE-548: tell the browser which session is now current so it discards
	// the previous session's blocks and replays this (empty) session's record —
	// the same reload signal switchSession pushes.
	s.srv.sendJSON(serverMessage{Kind: "session_switched", Message: sessionID})
}

// nextDefaultSessionTitle returns the next default placeholder title
// (e.g. "新会话N" / "New session N"), mirroring newSession's numbering
// (FEATURE-500). Used when a session title is cleared to unlock it.
func (s *WebSession) nextDefaultSessionTitle() string {
	nextN := 1
	sessionNumRe := regexp.MustCompile(`(\d+)$`)
	if entries, err := s.ag.Store().ListNamedSessions(); err == nil {
		maxN := 0
		for _, e := range entries {
			if m := sessionNumRe.FindStringSubmatch(e.Title); m != nil {
				if suffix, err := strconv.Atoi(m[1]); err == nil && suffix > maxN {
					maxN = suffix
				}
			}
		}
		if maxN > 0 {
			nextN = maxN + 1
		}
	}
	return fmt.Sprintf(i18n.T(i18n.KeyNewSessionTitle), nextN)
}

// popTo implements the retry-from block action (FEATURE-409): it pops the
// session back to the given message index (equivalent to :session pop to N)
// and reports the result to the browser.
func (s *WebSession) popTo(value string) {
	if s.session == nil {
		return
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		s.srv.sendJSON(serverMessage{Kind: "pop_result", OK: false, Message: "invalid message index"})
		return
	}
	result, err := s.session.Handle([]string{"pop", "to", strconv.Itoa(n)})
	if err != nil {
		s.srv.sendJSON(serverMessage{Kind: "pop_result", OK: false, Message: err.Error()})
		return
	}
	// FEATURE-507: keep the persisted event stream consistent with the popped
	// conversation, so a refresh does not replay the discarded turns.
	if err := s.ag.Store().DeleteEventsAfter(s.ag.CurrentSessionID(), n); err != nil {
		log.Warn("popTo DeleteEventsAfter: %v", err)
	}
	s.srv.sendJSON(serverMessage{Kind: "pop_result", OK: true, Message: result})
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

// renameSession renames the current session (FEATURE-425). The new title is
// persisted via UpdateNamedSession and the session list is refreshed so the
// status-bar menu and the main message area title bar stay in sync.
// FEATURE-500: a non-empty manual title is locked by prefixing "$" (so task
// completion no longer overwrites it); an empty title restores the default
// placeholder title and unlocks it, letting the next task completion
// auto-generate a new title.
func (s *WebSession) renameSession(title string) {
	title = strings.TrimSpace(title)
	id := s.ag.CurrentSessionID()
	if id == "" {
		return
	}
	entry, found, err := s.ag.Store().LoadNamedSession(id)
	if err != nil || !found || entry == nil {
		log.Warn("renameSession LoadNamedSession: %v", err)
		return
	}
	if title == "" {
		// FEATURE-500: clearing the title restores the default placeholder
		// title and unlocks it.
		entry.Title = s.nextDefaultSessionTitle()
	} else {
		// FEATURE-500: lock the manual title with a "$" prefix (strip any
		// existing leading "$" first so re-editing stays stable).
		entry.Title = "$" + strings.TrimLeft(title, "$")
	}
	if err := s.ag.Store().UpdateNamedSession(id, entry); err != nil {
		log.Warn("renameSession UpdateNamedSession: %v", err)
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
// --image flag path. When the main model does not support vision, the image
// bytes are NOT injected (the dynamic-context text already tells the model
// about the uploaded files) — FEATURE-469.
// Turn boundaries are signalled to the browser around the wait: await_input
// before blocking, turn_start once an input arrives.
func (s *WebSession) ReadLine(prompt string) (string, error) {
	s.srv.sendEvent(agent.NewStreamEvent(eventAwaitInput, agent.ChannelSystem, agent.LevelInfo, ""))
	for {
		// FEATURE-542 (GAP-3): the explicit FIFO queue fixes the order once and
		// for all; the old unbuffered channel left it to a random pick among the
		// connected tabs' read loops.
		if it, ok := s.inputQ.pop(); ok {
			msg := it.msg
			// Remember which端 sent this instruction so the YOU block of the
			// recorded turn and its interaction requests carry the source.
			s.lastInputSource = it.source
			s.lastInputSourceID = it.msg.SourceID
			// FEATURE-409: each user input starts a new message; bump the index so
			// stream events can be mapped back to a message for retry-from.
			s.msgIndex++
			s.srv.sendEvent(agent.NewStreamEvent(eventTurnStart, agent.ChannelSystem, agent.LevelInfo, ""))
			if len(msg.Attachments) > 0 && s.ag.MainModelSupportsVision() {
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
			text := strings.TrimSpace(msg.Text)
			// FEATURE-548: remember the user's own message so the recorded stream
			// replays it too. The message's index in the agent's array is only known
			// once the agent has appended it, so the WebRenderer persists it on this
			// turn's first stream event.
			s.pendingUserEcho = text
			return text, nil
		}
		select {
		case <-s.inputQ.notify:
		case <-s.closed:
			return "", io.EOF
		}
	}
}

// emitInteractionEvent persists one interaction lifecycle event and fans it out
// to every connected client (FEATURE-542). Persisting FIRST is what makes an
// interactive request survive a disconnect: a browser that reconnects replays it
// from the event stream and re-opens the dialog (R4). The event goes through the
// same store/seq machinery as every other event, so its order relative to LLM
// and tool events is preserved.
func (s *WebSession) emitInteractionEvent(evType string, meta map[string]string) {
	ev := agent.NewStreamEvent(evType, agent.ChannelSystem, agent.LevelInfo, "")
	ev.Meta = meta
	if ev.Meta == nil {
		ev.Meta = map[string]string{}
	}
	// Mirror the renderer: attach the current message index so history paging
	// keeps this event together with the turn it belongs to.
	if n := len(s.ag.Messages()) - 1; n >= 0 {
		ev.Meta["msg_index"] = strconv.Itoa(n)
	}
	if data, err := marshalEvent(ev); err == nil {
		if seq, err := s.ag.Store().AppendEventSeq(s.ag.CurrentSessionID(), data); err != nil {
			log.Warn("append %s event: %v", evType, err)
		} else {
			s.srv.sendEventSeq(ev, seq)
			return
		}
	}
	s.srv.sendEvent(ev)
}

// Acquire wires the per-run renderer. The WebIO stays installed for the
// whole session (see newWebSession), so release is a no-op.
func (s *WebSession) Acquire(ag *agent.Agent) (agent.EventRenderer, func()) {
	return &WebRenderer{s: s, coalescer: newEventCoalescer()}, func() {}
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

	// startSubAgentTurn delivers a finished sub-agent's completion notice as a
	// formal user message that starts a new turn (FEATURE-566). Set by
	// newWebSession to WebSession.startSubAgentCompletionTurn.
	startSubAgentTurn func(sessionID, notice string) bool

	mu      sync.Mutex
	pending map[string]chan askResult

	// emit persists and broadcasts one interaction lifecycle event
	// (FEATURE-542, GAP-1). Installed by newWebSession; nil in tests that build a
	// bare WebIO, in which case only the in-memory pending table is used.
	emit func(evType string, meta map[string]string)

	// source reports the label of the端 whose instruction started the current
	// turn (FEATURE-542); it is recorded as the source of the turn's requests.
	source func() string

	seq     atomic.Uint64
	reading atomic.Bool
}

// emitRequest writes one interaction_request event (FEATURE-542, GAP-1). The
// request is persisted before it is pushed, so a browser that is offline right
// now still finds it in the event stream when it reconnects (R4).
func (w *WebIO) emitRequest(id, mode, interactionJSON string) {
	if w.emit == nil {
		return
	}
	meta := map[string]string{"id": id, "source": w.currentSource()}
	if mode != "" {
		meta["mode"] = mode
	}
	if interactionJSON != "" {
		meta["interaction"] = interactionJSON
	}
	w.emit(eventInteractionRequest, meta)
}

// emitResolved records that a request was answered or cancelled. It is written
// before the agent is woken, so the other ends close the matching dialog
// (FEATURE-542, §5.6.2).
func (w *WebIO) emitResolved(id, by string, extra map[string]string) {
	if w.emit == nil {
		return
	}
	meta := map[string]string{"id": id, "resolved_by": by}
	for k, v := range extra {
		meta[k] = v
	}
	w.emit(eventInteractionResolved, meta)
}

// currentSource returns the label of the端 that started the current turn.
func (w *WebIO) currentSource() string {
	if w.source == nil {
		return ""
	}
	return w.source()
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

// PushSubAgents implements agent.SubAgentsPusher: it pushes a sub_agents event
// to the browser so the frontend refreshes the sub-agent list shown below the
// task-plan panel (FEATURE-564). sessionID is the session the listed
// sub-agents belong to; agentsJSON is the list as a JSON array.
func (w *WebIO) PushSubAgents(sessionID, agentsJSON string) {
	w.srv.sendEvent(agent.SubAgentsEvent(sessionID, agentsJSON))
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
	// FEATURE-542 (GAP-1): the request becomes a first-class persisted event
	// before it is fanned out. With no browser connected this is NOT an error any
	// more: the request is recorded, everyone who connects later replays it, and
	// this call simply keeps waiting (R7 — no timeout).
	w.emitRequest(id, "", string(inJSON))
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

	// FEATURE-542 (GAP-1/§5.6.4): same persisted-event path as Ask — one id
	// space, one event set, one pending table — so tool confirmations and
	// ask_user behave identically with several ends connected.
	w.emitRequest(id, mode, "")
	select {
	case res := <-ch:
		return res.value, res.err
	case <-w.sessionClosed:
		return "", io.EOF
	}
}

// resolve delivers one browser answer to the waiting ask. FEATURE-542: the
// first answer wins — the pending entry is removed and the resolution recorded
// under the same lock, so a second端 answering the same request is silently
// ignored (no error, no second resolved event, §5.6.3).
func (w *WebIO) resolve(id, value, by string) {
	w.mu.Lock()
	ch, ok := w.pending[id]
	if ok {
		delete(w.pending, id)
	}
	w.mu.Unlock()
	if !ok {
		return // late answer: silently ignored
	}
	// Record the resolution before waking the agent: the other ends use it to
	// close the matching dialog (§5.6.2).
	w.emitResolved(id, by, map[string]string{"answer": value})
	ch <- askResult{value: value}
}

// resolveInteraction delivers a browser interaction_answer to the waiting Ask.
// It shares the pending table and the single-winner rule with resolve (§5.6.4).
func (w *WebIO) resolveInteraction(id string, res agent.InteractionResult, by string) {
	w.mu.Lock()
	ch, ok := w.pending[id]
	if ok {
		delete(w.pending, id)
	}
	w.mu.Unlock()
	if !ok {
		return // late answer: silently ignored
	}
	extra := map[string]string{"resolved_by": by}
	if data, err := json.Marshal(res); err == nil {
		extra["result"] = string(data)
	}
	w.emitResolved(id, by, extra)
	ch <- askResult{result: res}
}

// cancelAll wakes every pending interaction with the "user cancelled" semantics
// (FEATURE-542, GAP-2/R8). It replaces the old disconnect-failAll path: the
// pause/stop key of any端 must be able to release a parked interaction, and the
// agent must see a cancellation rather than a connection failure.
func (w *WebIO) cancelAll(by string) {
	w.mu.Lock()
	pending := w.pending
	w.pending = map[string]chan askResult{}
	w.mu.Unlock()
	for id, ch := range pending {
		w.emitResolved(id, by, map[string]string{"cancelled": "true"})
		ch <- askResult{err: agent.ErrInteractionCancelled}
	}
}

// ---------------------------------------------------------------------------
// WebRenderer: agent.EventRenderer pushing events to the browser as-is
// ---------------------------------------------------------------------------

// WebRenderer forwards every stream event to the browser; partitioning is
// done by the frontend based on Type/Chan/Level.
type WebRenderer struct {
	s *WebSession
	// coalescer merges streaming fragments into complete blocks before they
	// are persisted for replay (FEATURE-507).
	coalescer *eventCoalescer
}

// msgIndexForRetry returns the index of the last message in the agent's
// message array. It is attached to stream events as "msg_index" so the
// frontend can map a block back to a message for the retry-from action
// (FEATURE-409).
//
// FIX-506: this MUST be the real message array index (not the turn counter),
// because the backend popTo() truncates by array index
// (a.SetHistory(aMsg[:n+1])). Using the turn counter here made the frontend
// send a much smaller number, truncating far more history than intended.
//
// FIX-506 (续修): the value is the index of the LAST message at render time,
// which is only meaningful for the YOU block when read on the FIRST event of a
// turn: at that point the agent has just appended the user message and no
// assistant/tool message exists yet, so len-1 IS that user message's index.
// The frontend therefore backfills the YOU block's index from the first event
// of the turn (see backfillPendingUserIndex in app.js) instead of using the
// stale value captured when the block was created.
func (r *WebRenderer) msgIndexForRetry() int {
	if r.s.ag == nil {
		return 0
	}
	n := len(r.s.ag.Messages()) - 1
	if n < 0 {
		return 0
	}
	return n
}

// Render pushes one event. task_plan and all other event types pass through
// uniformly; the frontend decides how to display them. The current message
// index is attached so the frontend can map a block back to a message for
// the retry-from action (FEATURE-409).
func (r *WebRenderer) Render(ev agent.StreamEvent) {
	if ev.Meta == nil {
		ev.Meta = map[string]string{}
	}
	ev.Meta["msg_index"] = strconv.Itoa(r.msgIndexForRetry())
	// FEATURE-548: record the user message that started this turn. It is written
	// here (not when the input arrived) because only now is its index in the
	// message array known. The event is persisted to the session's own stream and
	// deliberately NOT forwarded to the browser, which already rendered the YOU
	// block locally when the input was sent.
	r.persistPendingUserEcho()
	// FEATURE-507: persist the event so the browser can replay history after a
	// refresh. Streaming fragments are coalesced into complete blocks first —
	// replaying raw fragments would open one stray block per fragment because
	// the browser's streaming cursors do not exist during replay. The backend
	// keeps no history in memory; a write failure must not block the live stream.
	if r.coalescer == nil {
		r.coalescer = newEventCoalescer()
	}
	lastSeq := 0
	for _, pe := range r.coalescer.Add(ev) {
		data, err := marshalEvent(pe)
		if err != nil {
			continue
		}
		seq, err := r.s.ag.Store().AppendEventSeq(r.s.ag.CurrentSessionID(), data)
		if err != nil {
			log.Warn("append event stream: %v", err)
			continue
		}
		lastSeq = seq
	}
	// FEATURE-542: the frame carries the seq the event was persisted under, so a
	// reconnecting client can resume from the cursor it last saw (R3).
	r.s.srv.sendEventSeq(ev, lastSeq)
	// FEATURE-471: when a task ends (done event), hand any unconsumed
	// user_message events back to the browser so they can be re-submitted.
	if ev.Type == agent.EventDone {
		if pending := r.s.ag.PendingUserMessages(); len(pending) > 0 {
			r.s.srv.sendJSON(serverMessage{Kind: "dynamic_backfill", Backfill: pending})
		}
	}
}

// persistPendingUserEcho writes the user message that started the current turn
// into the session's event stream (FEATURE-548) and clears it. It is a no-op
// when no input is pending.
//
// The event carries the same meta.msg_index as the turn's first event, so it
// belongs to the same message group: paging keeps it together with that turn's
// blocks and :session pop removes it together with the message it started.
func (r *WebRenderer) persistPendingUserEcho() {
	text := r.s.pendingUserEcho
	if text == "" {
		return
	}
	r.s.pendingUserEcho = ""
	ev := agent.NewStreamEvent(eventUserEcho, agent.ChannelSystem, agent.LevelInfo, text)
	ev.Meta = map[string]string{"msg_index": strconv.Itoa(r.msgIndexForRetry())}
	// FEATURE-542: record which端 sent the message so the replayed YOU block
	// shows the same source label.
	if src := r.s.lastInputSource; src != "" {
		ev.Meta["source"] = src
	}
	if sid := r.s.lastInputSourceID; sid != "" {
		ev.Meta["source_id"] = sid
	}
	data, err := marshalEvent(ev)
	if err != nil {
		return
	}
	seq, err := r.s.ag.Store().AppendEventSeq(r.s.ag.CurrentSessionID(), data)
	if err != nil {
		log.Warn("append user echo: %v", err)
		return
	}
	// FEATURE-542: with several ends connected the echo must reach the OTHER ends
	// too — otherwise a second tab would show the assistant's answer without the
	// question that produced it. The end that typed it already rendered the YOU
	// block locally and skips this frame (it carries its own source_id).
	r.s.srv.sendEventSeq(ev, seq)
}

// taskPlanJSON marshals a task plan snapshot for the state message.
func taskPlanJSON(plan interface{}) string {
	data, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	return string(data)
}
