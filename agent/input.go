// Package agent - unified input event kind constants.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

// InputKind identifies the semantic type of a unified input event.
// These constants replace magic strings in the future InputSource event
// stream (P2.5 input unification). They are defined now so that both
// output events (events.go) and input events live in the same enum pair.
// See docs/output-architecture.md section 3.6.2.
const (
	InputLine      = "line"      // full line input (standard streaming)
	InputKey       = "key"       // single key (reactive)
	InputArrowUp   = "arrow_up"  // up arrow (after control char parsing)
	InputArrowDn   = "arrow_dn"  // down arrow
	InputArrowLt   = "arrow_lt"  // left arrow
	InputArrowRt   = "arrow_rt"  // right arrow
	InputEsc       = "esc"       // ESC (interrupt)
	InputCtrlC     = "ctrl_c"    // Ctrl+C (cancel)
	InputTab       = "tab"       // tab
	InputBackspace = "backspace" // backspace
	InputEnter     = "enter"     // enter
	InputEOF       = "eof"       // standard streaming EOF
)
