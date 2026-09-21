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

// TestHubFocusOnlyOnCardSwitch pins the agreed scope: only the drawer's agent
// card click focuses the input — the other switch entry points (create / edit
// save) and the boot-time auto-selection keep the previous behaviour. Exactly
// two occurrences (one definition + one call) enforce that.
func TestHubFocusOnlyOnCardSwitch(t *testing.T) {
	if got := strings.Count(webIndexHTML, "focusAgentInput("); got != 2 {
		t.Errorf("focusAgentInput( occurrences = %d, want 2 (definition + card-click call)", got)
	}
	if !strings.Contains(webIndexHTML, "closePanel(); focusAgentInput(current);") {
		t.Error("the agent card click does not focus the newly opened agent")
	}
}
