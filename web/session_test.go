// Package web - WebSession/WebIO/WebRenderer tests (FEATURE-307c) over an
// in-memory WebSocket pair: input -> ReadLine, Print -> ui_text event,
// ReadLine -> ask/answer round-trip, interrupt -> agent.Interrupt, and
// attachments -> agent.SetImagePaths.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/taskplan"
	"github.com/idirect3d/co-shell/workspace"
)

// newSessionFixture builds a Server + WebSession + connected browser client
// over a temp workspace. The initial state message is consumed by the
// caller via readServerMsg.
func newSessionFixture(t *testing.T) (*Server, repl.SessionIO, *agent.Agent, *wsTestClient) {
	t.Helper()

	ws, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ag := agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")

	srv := NewServer(ws.Root(), ServerOptions{Lang: "en"})
	sess, err := srv.SessionFactory()(repl.SessionDeps{Cfg: &config.Config{}, Ag: ag})
	if err != nil {
		t.Fatalf("session factory: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	client := dialWS(t, srv.mux)
	return srv, sess, ag, client
}

// sendClient sends one clientMessage upstream.
func sendClient(t *testing.T, c *wsTestClient, msg clientMessage) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.writeFrame(true, wsOpText, data); err != nil {
		t.Fatalf("client send %s: %v", msg.Type, err)
	}
}

// readServerMsg reads one downstream server message.
func readServerMsg(t *testing.T, c *wsTestClient) serverMessage {
	t.Helper()
	op, payload, err := c.readFrame()
	if err != nil {
		t.Fatalf("read server message: %v", err)
	}
	if op != wsOpText {
		t.Fatalf("server message opcode = %d, want text", op)
	}
	var msg serverMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal server message: %v", err)
	}
	return msg
}

// TestSessionInputToReadLine verifies a browser input message is returned
// by the session ReadLine, trimmed.
func TestSessionInputToReadLine(t *testing.T) {
	_, sess, _, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := sess.ReadLine("")
		done <- result{line, err}
	}()
	sendClient(t, client, clientMessage{Type: "input", Text: "  hello  "})
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("ReadLine error: %v", r.err)
		}
		if r.line != "hello" {
			t.Errorf("ReadLine = %q, want trimmed \"hello\"", r.line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadLine did not return after input message")
	}
}

// TestSessionAttachments verifies input attachments are resolved to
// workspace-absolute paths and installed on the agent.
func TestSessionAttachments(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	done := make(chan string, 1)
	go func() {
		line, _ := sess.ReadLine("")
		done <- line
	}()
	sendClient(t, client, clientMessage{Type: "input", Text: "hi", Attachments: []string{"a.png"}})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ReadLine did not return")
	}
	if len(ag.ImagePaths()) != 1 {
		t.Errorf("image paths = %v, want 1", ag.ImagePaths())
	}
}

// TestWebIOPrintAsUIText verifies WebIO.Print is pushed as a ui_text event.
func TestWebIOPrintAsUIText(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ag.IO().Print("confirm? ")
	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event == nil {
		t.Fatalf("expected event, got %+v", msg)
	}
	if msg.Event.Type != "ui_text" || msg.Event.Text != "confirm? " {
		t.Errorf("event = %+v, want ui_text \"confirm? \"", msg.Event)
	}
}

// TestWebIOAskAnswer verifies ReadLine sends an ask and blocks until the
// matching answer arrives.
func TestWebIOAskAnswer(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	done := make(chan string, 1)
	go func() {
		v, err := ag.IO().ReadLine()
		if err != nil {
			done <- "ERR:" + err.Error()
			return
		}
		done <- v
	}()
	msg := readServerMsg(t, client)
	if msg.Kind != "ask" {
		t.Fatalf("expected ask, got %+v", msg)
	}
	sendClient(t, client, clientMessage{Type: "answer", ID: msg.ID, Value: "42"})
	select {
	case v := <-done:
		if v != "42" {
			t.Errorf("ReadLine = %q, want 42", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadLine did not return after answer")
	}
}

// TestWebIOReadKey verifies key mode asks map the answer to one byte.
func TestWebIOReadKey(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	done := make(chan byte, 1)
	go func() {
		b, err := ag.IO().ReadKey()
		if err != nil {
			done <- 0
			return
		}
		done <- b
	}()
	msg := readServerMsg(t, client)
	if msg.Kind != "ask" || msg.Mode != "key" {
		t.Fatalf("expected key ask, got %+v", msg)
	}
	sendClient(t, client, clientMessage{Type: "answer", ID: msg.ID, Value: "y"})
	select {
	case b := <-done:
		if b != 'y' {
			t.Errorf("ReadKey = %q, want 'y'", b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadKey did not return after answer")
	}
}

// TestSessionInterrupt verifies an interrupt message reaches the agent's
// interrupt channel (the ESC equivalent). ResetInterrupt simulates the
// RunStream start, which is when the channel exists.
func TestSessionInterrupt(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state
	ag.ResetInterrupt()

	sendClient(t, client, clientMessage{Type: "interrupt"})
	select {
	case <-ag.InterruptChan():
	case <-time.After(2 * time.Second):
		t.Fatal("interrupt did not reach agent")
	}
}

// TestWebRendererForwardsEvents verifies the renderer pushes stream events
// to the browser unchanged.
func TestWebRendererForwardsEvents(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	renderer, release := sess.Acquire(ag)
	defer release()
	renderer.Render(agent.OKEvent(agent.ChannelTool, "done well"))
	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event.Type != agent.EventInfo ||
		msg.Event.Level != "success" || msg.Event.Chan != "tool" || msg.Event.Text != "done well" {
		t.Errorf("event = %+v, want success/tool \"done well\"", msg.Event)
	}
}

// TestWebSessionNonInteractive verifies web sessions suppress terminal
// decorations.
func TestWebSessionNonInteractive(t *testing.T) {
	_, sess, _, _ := newSessionFixture(t)
	if sess.Interactive() {
		t.Errorf("web session must be non-interactive")
	}
}

// TestWebIOSetSessionPushesTaskPlan verifies that switching the session via
// Agent.SetCurrentSessionID pushes a task_plan event to the browser so the
// frontend refreshes its plan panel (FEATURE-386).
func TestWebIOSetSessionPushesTaskPlan(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	// Create a task plan bound to session A.
	ag.TaskPlanManager().SetSessionID("sess-A")
	if _, err := ag.TaskPlanManager().UpdateSteps("Plan A", "desc", []taskplan.StepInput{
		{Description: "step one", Status: "[ ]"},
	}); err != nil {
		t.Fatalf("UpdateSteps: %v", err)
	}

	// Switch to session A: should push a task_plan event with the plan.
	ag.SetCurrentSessionID("sess-A")
	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event == nil || msg.Event.Type != agent.EventTaskPlan {
		t.Fatalf("expected task_plan event, got kind=%q type=%q", msg.Kind, msg.Event.Type)
	}
	planJSON := msg.Event.Meta["plan"]
	if planJSON == "" {
		t.Fatalf("task_plan event carries empty plan snapshot")
	}
	var plan taskplan.TaskPlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		t.Fatalf("plan snapshot not valid JSON: %v", err)
	}
	if plan.Title != "Plan A" || len(plan.Steps) != 1 {
		t.Fatalf("plan snapshot = %+v, want title \"Plan A\" with 1 step", plan)
	}
}

// TestSessionList verifies the session_list message returns the session list
// to the browser (FEATURE-387).
func TestSessionList(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	now := time.Now()
	for i, id := range []string{"sess-A", "sess-B"} {
		entry := &store.SessionEntry{
			ID:           id,
			Title:        "Session " + string(rune('A'+i)),
			Keywords:     "kw" + id,
			Messages:     []byte("[]"),
			MessageCount: 0,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := ag.Store().SaveNamedSession(entry); err != nil {
			t.Fatalf("SaveNamedSession: %v", err)
		}
	}
	ag.SetCurrentSessionID("sess-A")
	readServerMsg(t, client) // consume task_plan event from SetCurrentSessionID

	sendClient(t, client, clientMessage{Type: "session_list"})
	msg := readServerMsg(t, client)
	if msg.Kind != "sessions" {
		t.Fatalf("expected sessions message, got kind=%q", msg.Kind)
	}
	if len(msg.Sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(msg.Sessions))
	}
	foundCurrent := false
	for _, s := range msg.Sessions {
		if s.ID == "sess-A" && s.Current {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatalf("sess-A should be marked current, got %+v", msg.Sessions)
	}
}

// TestSessionListPushedOnSetCurrent verifies that changing the current
// session via Agent.SetCurrentSessionID pushes a fresh session list to the
// browser (FEATURE-387). This covers the :new path, which switches the
// current session outside the web session handler.
func TestSessionListPushedOnSetCurrent(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	now := time.Now()
	for _, id := range []string{"sess-A", "sess-B"} {
		entry := &store.SessionEntry{
			ID: id, Title: id, Messages: []byte("[]"), CreatedAt: now, UpdatedAt: now,
		}
		if err := ag.Store().SaveNamedSession(entry); err != nil {
			t.Fatalf("SaveNamedSession: %v", err)
		}
	}

	// SetCurrentSessionID pushes a task_plan event (FEATURE-386) then a
	// sessions list (FEATURE-387).
	ag.SetCurrentSessionID("sess-A")
	msg := readServerMsg(t, client)
	if msg.Kind != "event" {
		t.Fatalf("expected task_plan event, got kind=%q", msg.Kind)
	}
	msg = readServerMsg(t, client)
	if msg.Kind != "sessions" {
		t.Fatalf("expected sessions message, got kind=%q", msg.Kind)
	}
	if len(msg.Sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(msg.Sessions))
	}
}

// TestSessionSwitch verifies the session_switch message changes the current
// session (FEATURE-387).
func TestSessionSwitch(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	now := time.Now()
	for _, id := range []string{"sess-A", "sess-B"} {
		entry := &store.SessionEntry{
			ID: id, Title: id, Messages: []byte("[]"), CreatedAt: now, UpdatedAt: now,
		}
		if err := ag.Store().SaveNamedSession(entry); err != nil {
			t.Fatalf("SaveNamedSession: %v", err)
		}
	}
	ag.SetCurrentSessionID("sess-A")
	readServerMsg(t, client) // consume task_plan event from SetCurrentSessionID
	readServerMsg(t, client) // consume sessions list from SetCurrentSessionID

	sendClient(t, client, clientMessage{Type: "session_switch", Value: "sess-B"})
	// The switch pushes a task_plan event (FEATURE-386) then a sessions list.
	msg := readServerMsg(t, client)
	if msg.Kind != "event" {
		t.Fatalf("expected task_plan event after switch, got kind=%q", msg.Kind)
	}
	msg = readServerMsg(t, client)
	if msg.Kind != "sessions" {
		t.Fatalf("expected sessions message after switch, got kind=%q", msg.Kind)
	}
	if ag.CurrentSessionID() != "sess-B" {
		t.Fatalf("current session = %q, want sess-B", ag.CurrentSessionID())
	}
}

// TestSessionDelete verifies the session_delete message removes a session and
// protects the current session (FEATURE-387).
func TestSessionDelete(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	now := time.Now()
	for _, id := range []string{"sess-A", "sess-B"} {
		entry := &store.SessionEntry{
			ID: id, Title: id, Messages: []byte("[]"), CreatedAt: now, UpdatedAt: now,
		}
		if err := ag.Store().SaveNamedSession(entry); err != nil {
			t.Fatalf("SaveNamedSession: %v", err)
		}
	}
	ag.SetCurrentSessionID("sess-A")
	readServerMsg(t, client) // consume task_plan event from SetCurrentSessionID
	readServerMsg(t, client) // consume sessions list from SetCurrentSessionID

	// Deleting the current session is a no-op.
	sendClient(t, client, clientMessage{Type: "session_delete", Value: "sess-A"})
	msg := readServerMsg(t, client)
	if msg.Kind != "sessions" {
		t.Fatalf("expected sessions message, got kind=%q", msg.Kind)
	}
	if len(msg.Sessions) != 2 {
		t.Fatalf("current session should not be deleted, count = %d", len(msg.Sessions))
	}

	// Deleting a non-current session removes it.
	sendClient(t, client, clientMessage{Type: "session_delete", Value: "sess-B"})
	msg = readServerMsg(t, client)
	if msg.Kind != "sessions" {
		t.Fatalf("expected sessions message, got kind=%q", msg.Kind)
	}
	if len(msg.Sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1 after delete", len(msg.Sessions))
	}
	if _, found, _ := ag.Store().LoadNamedSession("sess-B"); found {
		t.Fatalf("sess-B should be deleted")
	}
}
