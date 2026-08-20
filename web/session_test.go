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
	"path/filepath"
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
	sendClient(t, client, clientMessage{Type: "input", Text: "  ls -la  "})
	select {
	case r := <-done:
		if r.err != nil || r.line != "ls -la" {
			t.Errorf("ReadLine = (%q, %v), want (\"ls -la\", nil)", r.line, r.err)
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
	sendClient(t, client, clientMessage{Type: "input", Text: "look", Attachments: []string{"pic.png", "../evil.png"}})
	<-done
	paths := ag.ImagePaths()
	if len(paths) != 1 || filepath.Base(paths[0]) != "pic.png" || !filepath.IsAbs(paths[0]) {
		t.Errorf("ImagePaths = %v, want one absolute pic.png path", paths)
	}
}

// TestWebIOPrintAsUIText verifies WebIO.Print is pushed as a ui_text event.
func TestWebIOPrintAsUIText(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ag.IO().Print("confirm? ")
	msg := readServerMsg(t, client)
	if msg.Kind != "event" || msg.Event == nil {
		t.Fatalf("message = %+v, want event", msg)
	}
	if msg.Event.Type != "ui_text" || msg.Event.Text != "confirm? " {
		t.Errorf("event = %+v, want ui_text \"confirm? \"", msg.Event)
	}
	_ = sess
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
			t.Errorf("ReadLine: %v", err)
		}
		done <- v
	}()

	ask := readServerMsg(t, client)
	if ask.Kind != "ask" || ask.Mode != "line" || ask.ID == "" {
		t.Fatalf("ask = %+v, want kind=ask mode=line with id", ask)
	}
	sendClient(t, client, clientMessage{Type: "answer", ID: ask.ID, Value: "yes"})
	select {
	case v := <-done:
		if v != "yes" {
			t.Errorf("ReadLine = %q, want \"yes\"", v)
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
			t.Errorf("ReadKey: %v", err)
		}
		done <- b
	}()
	ask := readServerMsg(t, client)
	if ask.Kind != "ask" || ask.Mode != "key" {
		t.Fatalf("ask = %+v, want kind=ask mode=key", ask)
	}
	sendClient(t, client, clientMessage{Type: "answer", ID: ask.ID, Value: "c"})
	select {
	case b := <-done:
		if b != 'c' {
			t.Errorf("ReadKey = %q, want 'c'", b)
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
		t.Fatal("interrupt message did not reach the agent")
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
