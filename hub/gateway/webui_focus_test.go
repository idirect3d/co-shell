// Package gateway - FEATURE-547 tests: switching to an agent from the hub drawer
// must hand the keyboard over to that agent's co-shell, but the focus belongs on
// the main workspace — never on the message input box, which would get in the way
// of the option hotkeys of a pending interaction.
//
// Author: L.Shuang
// Created: 2026-09-21
// MIT License - Copyright (c) 2026 L.Shuang

package gateway

import (
	"strings"
	"testing"
)

// uiFuncBody returns the source of a JS function, from its signature to the
// matching closing brace, or "" when the function is missing.
func uiFuncBody(src, signature string) string {
	i := strings.Index(src, signature)
	if i < 0 {
		return ""
	}
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1]
			}
		}
	}
	return ""
}

// TestHubSwitchFocusesAgentWorkspace pins the FEATURE-547 behaviour: after the
// drawer switches to an agent the keyboard belongs to that agent, but the focus
// lands on the main workspace instead of the message input box.
func TestHubSwitchFocusesAgentWorkspace(t *testing.T) {
	body := uiFuncBody(webIndexHTML, "function focusAgentView(id)")
	if body == "" {
		t.Fatal("focusAgentView is missing — switching agents would leave the keyboard focus in the hub")
	}
	// The frame is same-origin (reverse-proxied under /agent/{id}/), so its
	// document can be reached directly; a still-loading frame is covered by the
	// one-shot load listener. #stream-wrap is the co-shell main workspace.
	for _, want := range []string{
		"frames[id]",
		"f.contentDocument",
		"f.contentWindow.focus()",
		"getElementById('stream-wrap')",
		"doc.body",
		"view.focus()",
		"addEventListener('load', doFocus, { once: true })",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("focusAgentView missing %q", want)
		}
	}
	// The message input box must never be focused by the hub: a focused input
	// box is exactly what the user asked to avoid (option hotkeys).
	if strings.Contains(body, "getElementById('input')") {
		t.Error("focusAgentView must not focus the message input box")
	}
}

// TestHubFocusOnEverySwitchEntry pins the agreed scope: every hub entry point
// that switches to an agent and opens its frame hands the keyboard to that
// agent's workspace — the drawer card click, the power switch starting a stopped
// agent, and the create-then-start flow. The boot-time auto-selection keeps the
// previous behaviour and must not steal the focus. Exactly four occurrences (one
// definition + three calls) enforce the agreed set.
func TestHubFocusOnEverySwitchEntry(t *testing.T) {
	if got := strings.Count(webIndexHTML, "focusAgentView("); got != 4 {
		t.Errorf("focusAgentView( occurrences = %d, want 4 (definition + card click + start + create)", got)
	}
	if strings.Contains(webIndexHTML, "focusAgentInput(") {
		t.Error("the old focusAgentInput (message input box) is still referenced")
	}
	if !strings.Contains(webIndexHTML, "closePanel(); focusAgentView(current);") {
		t.Error("the agent card click does not focus the newly opened agent's workspace")
	}
	if body := uiFuncBody(webIndexHTML, "function startAgent(id, sw)"); !strings.Contains(body, "focusAgentView(id)") {
		t.Error("starting a stopped agent does not focus its co-shell workspace")
	}
	if body := uiFuncBody(webIndexHTML, "function createLocalAgent()"); !strings.Contains(body, "focusAgentView(nid)") {
		t.Error("the create-then-start flow does not focus the new agent's workspace")
	}
	if strings.Contains(webIndexHTML, "ensureFrame(current);\n      focusAgentView") {
		t.Error("the boot-time auto-selection must not steal the keyboard focus")
	}
}
