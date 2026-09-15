// Package agent - the render_ui waiting channel (FEATURE-524).
//
// render_ui(waiting=true) parks the tool call until the user acts on a rendered
// component, so the agent can continue in the same turn with the action as the
// tool result. Three things release the wait:
//
//  1. a component action   -> SubmitUIAction  (the action becomes the result)
//  2. a plain user message -> ReleaseUIWait   (the user typed instead; the
//     message itself starts the next turn as usual)
//  3. interrupt / timeout  -> ReleaseUIWait / the wait deadline
//
// The channel is buffered (capacity 1) so a release never blocks the sender,
// and it is owned by the waiting tool call, which clears it on the way out.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/idirect3d/co-shell/i18n"
)

// uiWaitTimeout bounds how long render_ui(waiting=true) parks. A user who walks
// away must not pin the turn forever; the LLM is told the wait ended instead.
const uiWaitTimeout = 120 * time.Second

// beginUIWait registers a waiting tool call and returns the release channel.
// It returns nil when another wait is already active: two parked calls would
// race for the same user action, so the second one degrades to a normal call.
func (a *Agent) beginUIWait() chan string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.uiWaitCh != nil {
		return nil
	}
	a.uiWaitCh = make(chan string, 1)
	return a.uiWaitCh
}

// endUIWait clears the registration of ch (no-op for a superseded channel).
func (a *Agent) endUIWait(ch chan string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.uiWaitCh == ch {
		a.uiWaitCh = nil
	}
}

// SubmitUIAction delivers a component action to a waiting render_ui call. It
// reports whether the action was consumed: false means no call is waiting, so
// the caller routes the action as a new user turn instead.
func (a *Agent) SubmitUIAction(uiID, actionID string, payload json.RawMessage) bool {
	a.mu.Lock()
	ch := a.uiWaitCh
	a.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- UIActionMessage(uiID, actionID, payload):
		return true
	default:
		return false
	}
}

// ReleaseUIWait ends a waiting render_ui call without a component action (the
// user sent a message instead, or pressed ESC). It reports whether a wait was
// actually released.
func (a *Agent) ReleaseUIWait() bool {
	a.mu.Lock()
	ch := a.uiWaitCh
	a.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- i18n.T(i18n.KeyUIWaitingCancelled):
		return true
	default:
		return false
	}
}

// waitUIAction blocks until the user acts, releases the wait, the turn is
// cancelled or the deadline passes. The returned string is the tool result the
// LLM sees.
func (a *Agent) waitUIAction(ctx context.Context, ch chan string) string {
	select {
	case msg := <-ch:
		return msg
	case <-ctx.Done():
		return i18n.T(i18n.KeyUIWaitingCancelled)
	case <-time.After(uiWaitTimeout):
		return i18n.TF(i18n.KeyUIWaitingTimeout, int(uiWaitTimeout/time.Second))
	}
}
