// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// dialogHandleTimeout bounds a single Page.handleJavaScriptDialog call.
const dialogHandleTimeout = 5 * time.Second

// DialogPolicy controls how JavaScript dialogs are answered.
type DialogPolicy string

const (
	// DialogAccept accepts every dialog.
	DialogAccept DialogPolicy = "accept"
	// DialogDismiss dismisses every dialog.
	DialogDismiss DialogPolicy = "dismiss"
	// DialogAuto accepts informative dialogs (alert/prompt) and dismisses
	// confirm-like ones (confirm/beforeunload), so the agent never silently
	// agrees to a destructive action the page asked to confirm.
	DialogAuto DialogPolicy = "auto"
)

// ParseDialogPolicy normalises a configured value, falling back to
// DialogAuto for empty/unknown input.
func ParseDialogPolicy(value string) (DialogPolicy, bool) {
	switch DialogPolicy(value) {
	case DialogAccept:
		return DialogAccept, true
	case DialogDismiss:
		return DialogDismiss, true
	case DialogAuto:
		return DialogAuto, true
	default:
		return DialogAuto, false
	}
}

// DialogInfo describes a dialog that was answered on behalf of the agent.
type DialogInfo struct {
	Type    string
	Message string
	Handled string
	At      time.Time
}

// DialogManager answers JavaScript dialogs on one CDP connection according to
// a policy. Without it a page that opens alert/confirm/prompt blocks every
// subsequent CDP call until the timeout fires.
type DialogManager struct {
	client *CDPClient
	cancel func()

	mu      sync.Mutex
	policy  DialogPolicy
	pending []DialogInfo
}

// NewDialogManager starts watching Page.javascriptDialogOpening on client.
func NewDialogManager(client *CDPClient, policy DialogPolicy) *DialogManager {
	if parsed, ok := ParseDialogPolicy(string(policy)); ok {
		policy = parsed
	} else {
		policy = DialogAuto
	}

	manager := &DialogManager{client: client, policy: policy}
	events, cancel := client.Watch("Page.javascriptDialogOpening")
	manager.cancel = cancel
	go manager.loop(events)
	return manager
}

// Close stops watching dialogs.
func (m *DialogManager) Close() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
	m.cancel = nil
}

// SetPolicy changes how subsequent dialogs are answered.
func (m *DialogManager) SetPolicy(policy DialogPolicy) {
	parsed, ok := ParseDialogPolicy(string(policy))
	if !ok {
		parsed = DialogAuto
	}
	m.mu.Lock()
	m.policy = parsed
	m.mu.Unlock()
}

// Policy returns the current policy.
func (m *DialogManager) Policy() DialogPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.policy
}

// TakeMessages returns the dialogs handled so far and clears the buffer.
func (m *DialogManager) TakeMessages() []DialogInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.pending) == 0 {
		return nil
	}
	out := m.pending
	m.pending = nil
	return out
}

// Descriptions renders the handled dialogs for tool output.
func (m *DialogManager) Descriptions() []string {
	dialogs := m.TakeMessages()
	if len(dialogs) == 0 {
		return nil
	}
	out := make([]string, 0, len(dialogs))
	for _, d := range dialogs {
		out = append(out, fmt.Sprintf("[%s] %s (%s)", d.Type, d.Message, d.Handled))
	}
	return out
}

// loop answers dialogs as they arrive.
func (m *DialogManager) loop(events <-chan Event) {
	for ev := range events {
		m.answer(ev)
	}
}

// answer replies to one dialog and records what happened.
func (m *DialogManager) answer(ev Event) {
	dialogType := eventString(ev.Params, "type")
	message := eventString(ev.Params, "message")

	accept := m.shouldAccept(dialogType)
	ctx, cancel := context.WithTimeout(context.Background(), dialogHandleTimeout)
	defer cancel()

	handled := "accepted"
	if !accept {
		handled = "dismissed"
	}
	if _, err := m.client.Call(ctx, "Page.handleJavaScriptDialog", map[string]interface{}{
		"accept": accept,
	}); err != nil {
		handled = "failed"
		log.Debug("cannot answer %s dialog: %v", dialogType, err)
	} else {
		log.Info("Browser dialog %s answered: %s (%s)", dialogType, message, handled)
	}

	m.mu.Lock()
	m.pending = append(m.pending, DialogInfo{
		Type:    dialogType,
		Message: message,
		Handled: handled,
		At:      time.Now(),
	})
	m.mu.Unlock()
}

// shouldAccept decides the answer for a dialog type under the current policy.
func (m *DialogManager) shouldAccept(dialogType string) bool {
	switch m.Policy() {
	case DialogAccept:
		return true
	case DialogDismiss:
		return false
	default:
		switch dialogType {
		case "confirm", "beforeunload":
			return false
		default:
			return true
		}
	}
}

// eventString reads a string field from CDP event params.
func eventString(params map[string]interface{}, key string) string {
	if params == nil {
		return ""
	}
	value, _ := params[key].(string)
	return value
}
