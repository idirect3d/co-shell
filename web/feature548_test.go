// Package web - FEATURE-548 tests: per-session UI event records. A user
// message is persisted with the turn it started (UC-0005), is never forwarded
// live (UC-0006), switching sessions is refused while a task runs (UC-0007),
// a successful switch tells the browser to reload (UC-0008), history is read
// by explicit session id (UC-0009) and the frontend implements both halves of
// the reload (UC-0010 / UC-0011).
//
// Author: L.Shuang
// Created: 2026-09-21
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package web

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/store"
)

// uc548Event builds a minimal persisted event belonging to a message group.
func uc548Event(msgIndex int, text string) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"type": "content",
		"chan": "llm",
		"text": text,
		"meta": map[string]string{"msg_index": strconv.Itoa(msgIndex)},
	})
	return data
}

// uc548Input sends one input message and waits until ReadLine has consumed it,
// i.e. until the session remembered it as the echo of the current turn.
func uc548Input(t *testing.T, readLine func(string) (string, error), c *wsTestClient, text string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_, _ = readLine("")
		close(done)
	}()
	sendClient(t, c, clientMessage{Type: "input", Text: text})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ReadLine did not return after the input message")
	}
}

// uc548PersistedTexts returns the types/texts of a session's persisted events,
// oldest first.
func uc548PersistedTexts(t *testing.T, ag *agent.Agent, sessionID string) ([]string, []string) {
	t.Helper()
	entries, _, err := ag.Store().LoadEvents(sessionID, 200, 0)
	if err != nil {
		t.Fatalf("LoadEvents(%s): %v", sessionID, err)
	}
	types := make([]string, 0, len(entries))
	texts := make([]string, 0, len(entries))
	for _, e := range entries {
		var ev struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(e.Data, &ev); err != nil {
			t.Fatalf("unmarshal persisted event: %v", err)
		}
		types = append(types, ev.Type)
		texts = append(texts, ev.Text)
	}
	return types, texts
}

// uc548Window returns at most size bytes of src starting at start.
func uc548Window(src string, start, size int) string {
	end := start + size
	if end > len(src) {
		end = len(src)
	}
	if start >= len(src) {
		return ""
	}
	return src[start:end]
}

// UC-0005: the user message is persisted on the turn's first event, with the
// message's index in the agent message array, and it precedes the turn's first
// business event.
func TestFeature548UC0005UserEchoPersistedFirst(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	const sid = "sess-548-uc0005"
	uc548Input(t, sess.ReadLine, client, "你好")
	ag.SetCurrentSessionID(sid)
	ag.SetHistory([]llm.Message{{Role: "user", Content: "你好"}})

	ws := sess.(*WebSession)
	r := &WebRenderer{s: ws}
	r.Render(agent.NewStreamEvent("content", agent.ChannelLLM, agent.LevelInfo, "回复"))

	types, texts := uc548PersistedTexts(t, ag, sid)
	if len(types) != 2 {
		t.Fatalf("persisted %d events (%v), want 2 (user_echo + content)", len(types), types)
	}
	if types[0] != eventUserEcho {
		t.Fatalf("first persisted event is %q, want %q (the YOU block must come first)", types[0], eventUserEcho)
	}
	if texts[0] != "你好" {
		t.Fatalf("persisted user echo = %q, want 你好", texts[0])
	}

	entries, _, err := ag.Store().LoadEvents(sid, 200, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	var echo struct {
		Meta map[string]string `json:"meta"`
	}
	if err := json.Unmarshal(entries[0].Data, &echo); err != nil {
		t.Fatalf("unmarshal user echo: %v", err)
	}
	wantIdx := strconv.Itoa(len(ag.Messages()) - 1)
	if got := echo.Meta["msg_index"]; got != wantIdx {
		t.Fatalf("user echo msg_index = %q, want %q (the user message index)", got, wantIdx)
	}
}

// UC-0006: the user message is persisted but NOT forwarded to the browser,
// which renders the YOU block locally when the input is sent.
func TestFeature548UC0006UserEchoNotForwarded(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	const sid = "sess-548-uc0006"
	uc548Input(t, sess.ReadLine, client, "你好")
	ag.SetCurrentSessionID(sid)
	ag.SetHistory([]llm.Message{{Role: "user", Content: "你好"}})

	ws := sess.(*WebSession)
	r := &WebRenderer{s: ws}
	r.Render(agent.NewStreamEvent("content", agent.ChannelLLM, agent.LevelInfo, "回复"))

	seenContent := false
	for i := 0; i < 8 && !seenContent; i++ {
		msg := readServerMsg(t, client)
		if msg.Kind != "event" || msg.Event == nil {
			continue
		}
		if msg.Event.Type == eventUserEcho {
			t.Fatal("user_echo was forwarded to the browser (duplicates the locally rendered YOU block)")
		}
		if msg.Event.Type == "content" {
			seenContent = true
		}
	}
	if !seenContent {
		t.Fatal("the turn's business event was not forwarded to the browser")
	}

	types, _ := uc548PersistedTexts(t, ag, sid)
	if len(types) == 0 || types[0] != eventUserEcho {
		t.Fatalf("persisted events %v, want the user_echo to be recorded for replay", types)
	}
}

// UC-0007: a session switch is refused while a task is running, because every
// recorded event goes to whichever session is current when it arrives.
func TestFeature548UC0007SwitchRefusedWhileBusy(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	const current = "sess-548-uc0007-A"
	ag.SetCurrentSessionID(current)
	ag.SetBusy(true)
	defer ag.SetBusy(false)

	sendClient(t, client, clientMessage{Type: "session_switch", Value: "sess-548-uc0007-B"})

	msg := uc548SeekMsg(t, client, "session_switch_result")
	if msg.OK {
		t.Fatalf("switch was accepted while the agent was busy: %+v", msg)
	}
	if got := ag.CurrentSessionID(); got != current {
		t.Fatalf("CurrentSessionID = %q, want unchanged %q", got, current)
	}
}

// UC-0008: a successful switch tells the browser to reload and keeps the two
// sessions' records separate.
func TestFeature548UC0008SwitchPushesReload(t *testing.T) {
	_, sess, _, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	ws := sess.(*WebSession)
	ag := ws.ag
	const from = "sess-548-uc0008-A"
	const to = "sess-548-uc0008-B"

	// Two sessions, each with its own recorded stream.
	ag.SetCurrentSessionID(from)
	if err := ag.Store().AppendEvent(from, uc548Event(0, "from-A")); err != nil {
		t.Fatalf("AppendEvent(A): %v", err)
	}
	now := time.Now()
	entry := &store.SessionEntry{
		ID:        to,
		Title:     "B",
		Messages:  []byte("[]"),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := ag.Store().SaveNamedSession(entry); err != nil {
		t.Fatalf("SaveNamedSession(B): %v", err)
	}
	if err := ag.Store().AppendEvent(to, uc548Event(0, "from-B")); err != nil {
		t.Fatalf("AppendEvent(B): %v", err)
	}

	sendClient(t, client, clientMessage{Type: "session_switch", Value: to})

	switched := false
	for i := 0; i < 6 && !switched; i++ {
		msg := readServerMsg(t, client)
		if msg.Kind == "session_switched" {
			if msg.Message != to {
				t.Fatalf("session_switched carries %q, want %q", msg.Message, to)
			}
			switched = true
		}
	}
	if !switched {
		t.Fatal("no session_switched frame was pushed to the browser")
	}
	if got := ag.CurrentSessionID(); got != to {
		t.Fatalf("CurrentSessionID = %q, want %q", got, to)
	}

	types, texts := uc548PersistedTexts(t, ag, to)
	if len(types) != 1 || texts[0] != "from-B" {
		t.Fatalf("session %s record = %v, want only its own event (streams must not leak across sessions)", to, texts)
	}
}

// UC-0009: history is read for the session explicitly passed in, not for the
// agent's current session.
func TestFeature548UC0009HistoryUsesExplicitSession(t *testing.T) {
	_, sess, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	const sessionA = "sess-548-uc0009-A"
	const sessionB = "sess-548-uc0009-B"
	if err := ag.Store().AppendEvent(sessionA, uc548Event(0, "from-A")); err != nil {
		t.Fatalf("AppendEvent(A): %v", err)
	}
	if err := ag.Store().AppendEvent(sessionB, uc548Event(0, "from-B")); err != nil {
		t.Fatalf("AppendEvent(B): %v", err)
	}
	ag.SetCurrentSessionID(sessionB)

	ws := sess.(*WebSession)
	ws.pushHistory(sessionA, 10, 0)

	msg := uc548SeekMsg(t, client, "history")
	if len(msg.Events) != 1 {
		t.Fatalf("history returned %d events, want 1 (session A's only event)", len(msg.Events))
	}
	var ev struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Events[0], &ev); err != nil {
		t.Fatalf("unmarshal history event: %v", err)
	}
	if ev.Text != "from-A" {
		t.Fatalf("history returned %q, want from-A (current session is %q)", ev.Text, ag.CurrentSessionID())
	}
}

// UC-0010: the frontend clears the message area and re-requests the history of
// the session it just switched to.
func TestFeature548UC0010FrontendReloadsHistory(t *testing.T) {
	src := uc548ReadAppJS(t)

	idx := strings.Index(src, `"session_switched"`)
	if idx < 0 {
		t.Fatal("app.js does not handle the session_switched signal")
	}
	seg := uc548Window(src, idx, 800)
	if !strings.Contains(seg, "resetStreamView(") {
		t.Fatalf("session_switched handler does not clear the message area: %q", seg)
	}
	if !strings.Contains(seg, "history_get") {
		t.Fatalf("session_switched handler does not re-request history: %q", seg)
	}
	if !strings.Contains(seg, "msg.message") {
		t.Fatalf("history_get does not carry the target session id: %q", seg)
	}
}

// UC-0011: the frontend renders a replayed user message as a YOU block.
func TestFeature548UC0011FrontendRendersUserEcho(t *testing.T) {
	src := uc548ReadAppJS(t)

	renderAt := strings.Index(src, "function renderEvent(")
	if renderAt < 0 {
		t.Fatal("app.js has no renderEvent function")
	}
	rel := strings.Index(src[renderAt:], "user_echo")
	if rel < 0 {
		t.Fatal("renderEvent has no user_echo branch: replayed YOU blocks would be dropped")
	}
	seg := uc548Window(src, renderAt+rel, 300)
	if !strings.Contains(seg, "renderUserEcho(") {
		t.Fatalf("user_echo branch does not call renderUserEcho: %q", seg)
	}
}

// uc548SeekMsg reads frames until one with the wanted kind arrives, skipping
// the frames the connection pushes on its own (state, session list, events).
func uc548SeekMsg(t *testing.T, c *wsTestClient, kind string) serverMessage {
	t.Helper()
	for i := 0; i < 8; i++ {
		msg := readServerMsg(t, c)
		if msg.Kind == kind {
			return msg
		}
	}
	t.Fatalf("no %q frame arrived", kind)
	return serverMessage{}
}

// uc548ReadAppJS loads the embedded frontend source from disk.
func uc548ReadAppJS(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read static/app.js: %v", err)
	}
	return string(data)
}
