// Package gateway - FEATURE-547 tests: switching to an agent from the hub drawer
// must move the keyboard focus into that agent's co-shell input, so the user can
// start typing without an extra click.
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

// TestHubSwitchFocusesAgentInput pins the FEATURE-547 behaviour: focusing an
// agent's co-shell input after the drawer switches to it.
func TestHubSwitchFocusesAgentInput(t *testing.T) {
	body := uiFuncBody(webIndexHTML, "function focusAgentInput(id)")
	if body == "" {
		t.Fatal("focusAgentInput is missing — switching agents would leave the keyboard focus in the hub")
	}
	// The frame is same-origin (reverse-proxied under /agent/{id}/), so the
	// co-shell input can be focused directly; a still-loading frame is covered
	// by the one-shot load listener.
	for _, want := range []string{
		"frames[id]",
		"f.contentDocument",
		"getElementById('input')",
		"inp.focus()",
		"addEventListener('load', doFocus, { once: true })",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("focusAgentInput missing %q", want)
		}
	}
}

// TestHubFocusOnEverySwitchEntry pins the agreed scope: every hub entry point
// that switches to an agent and opens its frame focuses that agent's input —
// the drawer card click, the power switch starting a stopped agent, and the
// create-then-start flow. The boot-time auto-selection keeps the previous
// behaviour and must not steal the focus. Exactly four occurrences (one
// definition + three calls) enforce the agreed set.
func TestHubFocusOnEverySwitchEntry(t *testing.T) {
	if got := strings.Count(webIndexHTML, "focusAgentInput("); got != 4 {
		t.Errorf("focusAgentInput( occurrences = %d, want 4 (definition + card click + start + create)", got)
	}
	if !strings.Contains(webIndexHTML, "closePanel(); focusAgentInput(current);") {
		t.Error("the agent card click does not focus the newly opened agent")
	}
	if body := uiFuncBody(webIndexHTML, "function startAgent(id, sw)"); !strings.Contains(body, "focusAgentInput(id)") {
		t.Error("starting a stopped agent does not focus its co-shell input")
	}
	if body := uiFuncBody(webIndexHTML, "function createLocalAgent()"); !strings.Contains(body, "focusAgentInput(nid)") {
		t.Error("the create-then-start flow does not focus the new agent's co-shell input")
	}
	if strings.Contains(webIndexHTML, "ensureFrame(current);\n      focusAgentInput") {
		t.Error("the boot-time auto-selection must not steal the keyboard focus")
	}
}
