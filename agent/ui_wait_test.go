// Package agent - unit tests for in-place update and the waiting channel
// (FEATURE-524, use-case group E: UC-28 .. UC-32).
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// waitParked reports whether a render_ui(waiting=true) call is parked.
func waitParked(ag *Agent) bool {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	return ag.uiWaitCh != nil
}

// waitForPark polls until the parked call registers (or the deadline passes).
func waitForPark(t *testing.T, ag *Agent) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if waitParked(ag) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("UC-32: render_ui(waiting=true) never parked")
}

// UC-29: a render_ui call carrying an update target parks an in-place update
// instead of a new tree, and the receipt names the updated component.
func TestUIUC29UpdateParksInPlace(t *testing.T) {
	ag := newUITestAgent(true)
	args := map[string]interface{}{
		"tree":   map[string]interface{}{"type": "card", "id": "n1", "props": map[string]interface{}{"title": "新"}},
		"update": "n1",
	}

	receipt, err := ag.renderUITool(context.Background(), args)
	if err != nil {
		t.Fatalf("UC-29: render_ui(update) failed: %v", err)
	}
	if !strings.Contains(receipt, "n1") {
		t.Fatalf("UC-29: receipt %q must name the updated component", receipt)
	}

	target, node := ag.takePendingUIUpdate()
	if target != "n1" || node == nil || node.Type != UICompCard {
		t.Fatalf("UC-29: pending update = (%q, %+v), want (n1, card)", target, node)
	}
	if again, _ := ag.takePendingUIUpdate(); again != "" {
		t.Fatal("UC-29: the update must be consumed exactly once")
	}
	if root, _ := ag.takePendingUITree(); root != nil {
		t.Fatal("UC-29: an update must not also park a new tree")
	}

	// The update target is trimmed, and an empty target means a normal render.
	if got := uiUpdateTarget(map[string]interface{}{"update": "  "}); got != "" {
		t.Fatalf("UC-29: blank update target = %q, want the normal render path", got)
	}
}

// UC-31: waiting=false (default) returns immediately and parks a tree.
func TestUIUC31WaitingFalseDoesNotBlock(t *testing.T) {
	ag := newUITestAgent(true)
	start := time.Now()
	receipt, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"tree":    map[string]interface{}{"type": "progress", "props": map[string]interface{}{"value": 1}},
		"waiting": false,
	})
	if err != nil {
		t.Fatalf("UC-31: render_ui failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("UC-31: waiting=false took %v, want an immediate receipt", elapsed)
	}
	if !strings.Contains(receipt, "ui-") {
		t.Fatalf("UC-31: receipt %q must carry the tree id", receipt)
	}
	if root, _ := ag.takePendingUITree(); root == nil {
		t.Fatal("UC-31: the tree must still be parked for the ui_render event")
	}
	if waitParked(ag) {
		t.Fatal("UC-31: waiting=false must not park a wait")
	}
}

// UC-32 (1)+(2): waiting=true parks, and a component action becomes the tool
// result so the agent continues in the same turn.
func TestUIUC32SubmitReleasesWait(t *testing.T) {
	ag := newUITestAgent(true)
	result := make(chan string, 1)
	go func() {
		text, err := ag.renderUITool(context.Background(), map[string]interface{}{
			"tree":    map[string]interface{}{"type": "form", "id": "f1"},
			"waiting": true,
		})
		if err != nil {
			result <- "error: " + err.Error()
			return
		}
		result <- text
	}()

	waitForPark(t, ag)
	if !ag.SubmitUIAction("ui-7", "go", json.RawMessage(`{"amount":88000}`)) {
		t.Fatal("UC-32: the parked call must accept the component action")
	}

	select {
	case text := <-result:
		if !strings.Contains(text, "go") || !strings.Contains(text, "88000") {
			t.Fatalf("UC-32: tool result %q must carry the action id and payload", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UC-32: the action did not release the parked call")
	}
	if waitParked(ag) {
		t.Fatal("UC-32: the wait must be cleared after release")
	}
	if ag.SubmitUIAction("ui-7", "go", nil) {
		t.Fatal("UC-32: an action with no parked call must not be consumed")
	}
}

// UC-32 (3): a plain message (or ESC) releases the wait, and the LLM is told
// that the user did not interact.
func TestUIUC32ReleaseOnUserMessage(t *testing.T) {
	ag := newUITestAgent(true)
	result := make(chan string, 1)
	go func() {
		text, _ := ag.renderUITool(context.Background(), map[string]interface{}{
			"tree":    map[string]interface{}{"type": "form", "id": "f1"},
			"waiting": true,
		})
		result <- text
	}()

	waitForPark(t, ag)
	if !ag.ReleaseUIWait() {
		t.Fatal("UC-32: an active wait must be releasable")
	}
	select {
	case text := <-result:
		if !strings.Contains(text, "用户未操作") && !strings.Contains(text, "did not interact") {
			t.Fatalf("UC-32: tool result %q must report the cancelled wait", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UC-32: the release did not unblock the parked call")
	}
	if ag.ReleaseUIWait() {
		t.Fatal("UC-32: releasing with no parked call must report false")
	}
}

// UC-32: a cancelled turn (ESC / session shutdown) also releases the wait, and
// a second concurrent wait is refused instead of racing for the same action.
func TestUIUC32CancelAndConcurrentGuard(t *testing.T) {
	ag := newUITestAgent(true)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan string, 1)
	go func() {
		text, _ := ag.renderUITool(ctx, map[string]interface{}{
			"tree":    map[string]interface{}{"type": "form", "id": "f1"},
			"waiting": true,
		})
		result <- text
	}()

	waitForPark(t, ag)
	if ch := ag.beginUIWait(); ch != nil {
		t.Fatal("UC-32: a second concurrent wait must be refused")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("UC-32: cancelling the turn did not release the parked call")
	}
}

// UC-28: the action text handed to the LLM keeps the ui id, the action id and
// the exact payload, and never leaves an empty payload ambiguous.
func TestUIUC28UIActionMessage(t *testing.T) {
	cases := []struct {
		name    string
		uiID    string
		action  string
		payload json.RawMessage
		want    []string
	}{
		{"payload", "ui-3", "drill", json.RawMessage(`{"label":"华东","value":320}`), []string{"ui-3", "drill", "华东", "320"}},
		{"empty payload", "ui-4", "reset", nil, []string{"ui-4", "reset", "{}"}},
		{"null payload", "ui-5", "close", json.RawMessage(`null`), []string{"ui-5", "close", "{}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UIActionMessage(tc.uiID, tc.action, tc.payload)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("UC-28: %q must contain %q", got, want)
				}
			}
		})
	}
}
