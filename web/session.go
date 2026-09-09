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

	// msgIndex is the current message index, incremented on each user input.
	// It is attached to stream events so the frontend can map a block back to
	// the message index for the retry-from action (FEATURE-409).
	msgIndex int

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
		srv:      srv,
		ag:       deps.Ag,
		settings: deps.SettingsHandler,
		mcp:      deps.MCPHandler,
		session:  cmd.NewSessionHandler(deps.Ag, deps.Cfg),
		mode:     cmd.NewModeHandler(deps.Cfg, deps.Ag),
		model:    cmd.NewModelHandler(deps.Cfg, deps.Ag),
		inputCh:  make(chan clientMessage),
		closed:   make(chan struct{}),
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
	// FEATURE-499: expose the agent busy state via GET /api/status for the hub.
	srv.SetBusyProvider(sess.ag.IsBusy)
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
	case "dynamic_event":
		// FEATURE-471: a user-action event reported while a task is running
		// (clip_object / upload_file / user_message / open_file). Enqueue it
		// into the agent's dynamic perception queue so the next tool/user
		// message injection surfaces it to the LLM.
		s.ag.AddDynamicEvent(agent.DynamicEventKind(msg.Kind), msg.Value)
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
	case "restart":
		// FEATURE-398: send a restart signal to the current process so an
		// external supervisor (launchd/systemd) restarts the service.
		requestRestart()
	case "session_list":
		s.pushSessionList()
	case "session_switch":
		s.switchSession(msg.Value)
	case "session_delete":
		s.deleteSession(msg.Value)
	case "session_rename":
		s.renameSession(msg.Value)
	case "session_new":
		s.newSession()
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
	s.srv.sendJSON(serverMessage{Kind: "identity_result", OK: true, Message: key})
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
	s.srv.sendJSON(serverMessage{Kind: "models", Models: mRaw, Templates: tRaw})
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
	s.wio.failAll()
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
}

// newSession creates a new empty session and switches to it (FEATURE-401),
// mirroring the REPL :new command.
func (s *WebSession) newSession() {
	if err := s.ag.FlushCurrentSession(); err != nil {
		log.Warn("newSession FlushCurrentSession: %v", err)
	}
	// Find the next "New session N" number.
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
	s.ag.Reset()
	now := time.Now()
	randBytes := make([]byte, 4)
	randBytes[0] = byte(now.Nanosecond() & 0xFF)
	randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
	randBytes[2] = byte(now.Second() & 0xFF)
	randBytes[3] = byte(now.Minute() & 0xFF)
	sessionID := fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
	title := fmt.Sprintf(i18n.T(i18n.KeyNewSessionTitle), nextN)
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
func (s *WebSession) renameSession(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		s.pushSessionList()
		return
	}
	id := s.ag.CurrentSessionID()
	if id == "" {
		return
	}
	entry, found, err := s.ag.Store().LoadNamedSession(id)
	if err != nil || !found || entry == nil {
		log.Warn("renameSession LoadNamedSession: %v", err)
		return
	}
	entry.Title = title
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
	select {
	case msg := <-s.inputCh:
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
		return strings.TrimSpace(msg.Text), nil
	case <-s.closed:
		return "", io.EOF
	}
}

// Acquire wires the per-run renderer. The WebIO stays installed for the
// whole session (see newWebSession), so release is a no-op.
func (s *WebSession) Acquire(ag *agent.Agent) (agent.EventRenderer, func()) {
	return &WebRenderer{s: s}, func() {}
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
	s *WebSession
}

// Render pushes one event. task_plan and all other event types pass through
// uniformly; the frontend decides how to display them. The current message
// index is attached so the frontend can map a block back to a message for
// the retry-from action (FEATURE-409).
func (r *WebRenderer) Render(ev agent.StreamEvent) {
	if ev.Meta == nil {
		ev.Meta = map[string]string{}
	}
	ev.Meta["msg_index"] = strconv.Itoa(r.s.msgIndex)
	r.s.srv.sendEvent(ev)
	// FEATURE-471: when a task ends (done event), hand any unconsumed
	// user_message events back to the browser so they can be re-submitted.
	if ev.Type == agent.EventDone {
		if pending := r.s.ag.PendingUserMessages(); len(pending) > 0 {
			r.s.srv.sendJSON(serverMessage{Kind: "dynamic_backfill", Backfill: pending})
		}
	}
}

// taskPlanJSON marshals a task plan snapshot for the state message.
func taskPlanJSON(plan interface{}) string {
	data, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	return string(data)
}
