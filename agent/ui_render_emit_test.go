// Package agent - regression tests for the ui_render emission timing
// (FEATURE-524 fix).
//
// A render_ui(waiting=true) call parks until the user acts on the rendered
// component. The stream loop can only emit the ui_render event after the tool
// call returns, so a waiting call used to keep its component invisible for the
// whole wait (it surfaced only after an interrupt). The tool therefore
// publishes the tree itself, right before parking.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorder collects the events an installed emitter receives.
type recorder struct {
	mu     sync.Mutex
	events []StreamEvent
}

func (r *recorder) emit(ev StreamEvent) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []StreamEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]StreamEvent(nil), r.events...)
}

// UC-60: a waiting call publishes its component tree before it parks, so the
// frontend can paint it while the user is still deciding.
func TestUIRenderUC60EmittedBeforeWaitParks(t *testing.T) {
	ag := &Agent{}
	rec := &recorder{}
	ag.setUIEmitter(rec.emit)

	result := make(chan string, 1)
	go func() {
		text, err := ag.renderUITool(context.Background(), map[string]interface{}{
			"tree":    map[string]interface{}{"type": UICompCard},
			"waiting": true,
		})
		if err != nil {
			t.Errorf("UC-60: render_ui failed: %v", err)
		}
		result <- text
	}()

	waitForPark(t, ag)
	if !waitParked(ag) {
		t.Fatal("UC-60: the call must be parked while it waits for the user")
	}

	events := rec.snapshot()
	if len(events) != 1 {
		t.Fatalf("UC-60: got %d events while parked, want exactly 1", len(events))
	}
	if events[0].Type != EventUIRender || events[0].Chan != ChannelSystem {
		t.Fatalf("UC-60: event = %+v, want ui_render on ChannelSystem", events[0])
	}
	if events[0].Meta[MetaKeyUIID] == "" || !strings.Contains(events[0].Meta[MetaKeyUITree], "card") {
		t.Fatalf("UC-60: event must carry the id and the tree, got %+v", events[0].Meta)
	}

	// The loop's own post-call emission must find nothing: the tree was already
	// published, and a second render would duplicate the component.
	if _, ok := ag.takeUIRenderEvent(); ok {
		t.Fatal("UC-60: the tree must be published exactly once")
	}

	// The wait itself is unchanged: the user's action is still the result.
	if !ag.SubmitUIAction("ui-1", "send", json.RawMessage(`{"amount":7}`)) {
		t.Fatal("UC-60: the parked call must accept the component action")
	}
	select {
	case text := <-result:
		if !strings.Contains(text, "amount") {
			t.Fatalf("UC-60: result %q must carry the action payload", text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("UC-60: the action did not release the parked call")
	}
	if waitParked(ag) {
		t.Fatal("UC-60: the wait must be cleared after release")
	}
}

// UC-60 (2): without an installed callback (unit tests, embedders) nothing is
// consumed, so the loop's post-call emission still delivers the tree.
func TestUIRenderUC60NoEmitterKeepsTreeParked(t *testing.T) {
	ag := &Agent{}
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"tree": map[string]interface{}{"type": UICompCard},
	}); err != nil {
		t.Fatalf("UC-60: render_ui failed: %v", err)
	}
	if ag.emitUIRender() {
		t.Fatal("UC-60: no emitter is installed, nothing may be emitted")
	}
	if root, _ := ag.takePendingUITree(); root == nil {
		t.Fatal("UC-60: the tree must stay parked for the stream loop")
	}
}

// UC-60 (3): the emitter is cleared when the turn ends, so a later call from
// outside a turn cannot publish into a dead frontend.
func TestUIRenderUC60EmitterLifecycle(t *testing.T) {
	ag := &Agent{}
	rec := &recorder{}
	ag.setUIEmitter(rec.emit)
	if !ag.emitUIRender() && len(rec.snapshot()) != 0 {
		t.Fatal("UC-60: an empty park must not emit anything")
	}
	ag.setUIEmitter(nil)
	if ag.uiEmit != nil {
		t.Fatal("UC-60: clearing the emitter must be effective")
	}
}
