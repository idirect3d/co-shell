// Package web - FIX-513 regression tests: the answers of a multi-question
// ask_user form must survive the WebSocket wire. A hand-written mirror DTO
// (interactionResultJSON) used to lack the Answers field, so the browser's
// result.answers was silently dropped and ask_user returned an empty tool
// result — the LLM never saw the answer.
//
// Author: L.Shuang
// Created: 2026-09-13
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// resultJSON marshals an interaction result into the wire payload carried by
// an interaction_answer message.
func resultJSON(t *testing.T, res agent.InteractionResult) json.RawMessage {
	t.Helper()
	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal interaction result: %v", err)
	}
	return blob
}

// TestWebIOAskInteractionAnswers verifies a multi-question form round-trips
// through the browser channel with every answer field intact (FIX-513).
func TestWebIOAskInteractionAnswers(t *testing.T) {
	_, _, ag, client := newSessionFixture(t)
	readServerMsg(t, client) // initial state

	wio, ok := ag.IO().(*WebIO)
	if !ok {
		t.Fatalf("agent IO is not *WebIO")
	}

	done := make(chan agent.InteractionResult, 1)
	go func() {
		res, err := wio.Ask(nil, agent.Interaction{
			Kind: agent.InteractionQuestions,
			Questions: []agent.Question{
				{Title: "用哪个数据库？", Options: []string{"MySQL", "PostgreSQL"}},
				{Title: "还有其他说明吗？"},
			},
		})
		if err != nil {
			t.Errorf("Ask error: %v", err)
		}
		done <- res
	}()

	msg := readServerMsg(t, client)
	if msg.Kind != "interaction" {
		t.Fatalf("expected interaction message, got kind=%q", msg.Kind)
	}
	var in agent.Interaction
	if err := json.Unmarshal(msg.Interaction, &in); err != nil {
		t.Fatalf("unmarshal interaction: %v", err)
	}
	if in.Kind != agent.InteractionQuestions || len(in.Questions) != 2 {
		t.Fatalf("interaction payload wrong: %+v", in)
	}

	sendClient(t, client, clientMessage{
		Type: "interaction_answer",
		ID:   msg.ID,
		Result: resultJSON(t, agent.InteractionResult{
			Action: agent.ActionSubmit,
			Answers: []agent.QuestionAnswer{
				{
					Question: "用哪个数据库？",
					Selected: []string{"PostgreSQL"},
					Notes:    []agent.AnswerNote{{Option: "PostgreSQL", Note: "需要只读实例"}},
				},
				{Question: "还有其他说明吗？", Text: "请保留旧表"},
			},
		}),
	})

	select {
	case res := <-done:
		if res.Action != agent.ActionSubmit {
			t.Fatalf("Ask result action = %q, want submit", res.Action)
		}
		if len(res.Answers) != 2 {
			t.Fatalf("Answers = %+v, want 2 entries (answers dropped on the wire?)", res.Answers)
		}
		if len(res.Answers[0].Selected) != 1 || res.Answers[0].Selected[0] != "PostgreSQL" {
			t.Errorf("Q1 selected = %v, want [PostgreSQL]", res.Answers[0].Selected)
		}
		if len(res.Answers[0].Notes) != 1 || res.Answers[0].Notes[0].Note != "需要只读实例" {
			t.Errorf("Q1 notes = %+v, want one note on PostgreSQL", res.Answers[0].Notes)
		}
		if res.Answers[1].Text != "请保留旧表" {
			t.Errorf("Q2 text = %q, want 请保留旧表", res.Answers[1].Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Ask did not return after interaction_answer")
	}
}

// TestInteractionAnswerWireCarriesAnswers pins the wire contract: a browser
// payload (action + answers) decodes into agent.InteractionResult with every
// field intact, and a clientMessage carrying it survives re-encoding. It fails
// if a hand-written mirror DTO without Answers is reintroduced (FIX-513).
func TestInteractionAnswerWireCarriesAnswers(t *testing.T) {
	payload := []byte(`{"action":"submit","value":"","raw":"","answers":[` +
		`{"question":"Q1","selected":["A","B"],"notes":[{"option":"A","note":"n1"}]},` +
		`{"question":"Q2","text":"free text"}]}`)

	var res agent.InteractionResult
	if err := json.Unmarshal(payload, &res); err != nil {
		t.Fatalf("unmarshal interaction result: %v", err)
	}
	if res.Action != agent.ActionSubmit {
		t.Errorf("action = %q, want submit", res.Action)
	}
	if len(res.Answers) != 2 {
		t.Fatalf("Answers = %+v, want 2 entries", res.Answers)
	}
	if len(res.Answers[0].Selected) != 2 || res.Answers[0].Selected[1] != "B" {
		t.Errorf("Q1 selected = %v, want [A B]", res.Answers[0].Selected)
	}
	if len(res.Answers[0].Notes) != 1 || res.Answers[0].Notes[0].Option != "A" {
		t.Errorf("Q1 notes = %+v, want one note on A", res.Answers[0].Notes)
	}
	if res.Answers[1].Text != "free text" {
		t.Errorf("Q2 text = %q, want free text", res.Answers[1].Text)
	}

	blob, err := json.Marshal(clientMessage{Type: "interaction_answer", ID: "ask-1", Result: payload})
	if err != nil {
		t.Fatalf("marshal client message: %v", err)
	}
	var back clientMessage
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal client message: %v", err)
	}
	var resBack agent.InteractionResult
	if err := json.Unmarshal(back.Result, &resBack); err != nil {
		t.Fatalf("unmarshal result after wire round trip: %v", err)
	}
	if len(resBack.Answers) != 2 || resBack.Answers[1].Text != "free text" {
		t.Errorf("wire round trip lost answers: %+v", resBack)
	}
}
