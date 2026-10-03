// Package gateway - FIX-597 test: the hub drawer's swipe-to-delete gesture must
// not leak window-level listeners.
//
// Author: L.Shuang
// Created: 2026-10-03
// MIT License - Copyright (c) 2026 L.Shuang

package gateway

import (
	"strings"
	"testing"
)

// TestAttachSwipeDoesNotLeakWindowListeners pins FIX-597. attachSwipe() is
// called once per agent card on every renderList(), and it used to register the
// drag listeners on `window` while the card was built — never removing them.
// Two of them per card per rebuild accumulated without bound: each later
// addEventListener had to scan the whole (growing) listener list linearly
// (WebKit's JSEventListener::operator== appeared as a top sampled frame), and
// every leaked closure kept its discarded card's DOM subtree alive.
//
// Measured on a Safari tab left on the hub page (21 agents, 3s poll): 294
// registrations in 22 seconds, 10-70% CPU and ~1.7GB in the WebContent process.
//
// The listeners must therefore exist only while a mouse drag is in progress:
// registered on mousedown, removed on mouseup or when the gesture turns into a
// vertical scroll.
func TestAttachSwipeDoesNotLeakWindowListeners(t *testing.T) {
	body := uiFuncBody(webIndexHTML, "function attachSwipe(card)")
	if body == "" {
		t.Fatal("attachSwipe is missing — the swipe gesture cannot be checked")
	}

	for _, ev := range []string{"mousemove", "mouseup"} {
		add := "window.addEventListener('" + ev + "'"
		del := "window.removeEventListener('" + ev + "'"
		if n := strings.Count(body, add); n != 1 {
			t.Errorf("attachSwipe registers %q on window %d times, want exactly 1 (inside the drag-only helper)", ev, n)
		}
		if n := strings.Count(body, del); n != 1 {
			t.Errorf("attachSwipe removes %q from window %d times, want exactly 1 — an unpaired registration leaks on every list rebuild", ev, n)
		}
	}

	if !strings.Contains(body, "function listen()") || !strings.Contains(body, "function unlisten()") {
		t.Error("attachSwipe has no listen/unlisten pair — drag listeners would leak")
	}
	if !strings.Contains(body, "listening = true") || !strings.Contains(body, "if (listening) return") {
		t.Error("attachSwipe registers window listeners without an idempotence guard — repeated drags would stack them")
	}
	// The card-construction tail must not register anything on window: that was
	// the leak. Only the mousedown handler may start listening.
	if strings.Contains(body, "begin(e.clientX, e.clientY); });") {
		t.Error("attachSwipe still starts a drag without enabling the window listeners")
	}
	if !strings.Contains(body, "function end(){\n      unlisten();") {
		t.Error("attachSwipe's end() must unlisten so a finished drag leaves nothing behind")
	}
	if !strings.Contains(body, "unlisten();\n        setSwipe(card, !!card._open);") {
		t.Error("the vertical-scroll cancel path must unlisten as well")
	}
}
