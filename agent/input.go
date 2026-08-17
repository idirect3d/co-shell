// Package agent - unified input event kind constants.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import "context"

// InputKind identifies the semantic type of a unified input event.
// These constants replace magic strings in the InputSource event
// stream (P2.5 input unification). They are defined so that both
// output events (events.go) and input events live in the same enum pair.
// See docs/output-architecture.md section 3.6.2.
type InputKind string

const (
	InputLine      = "line"      // full line input (standard streaming)
	InputKey       = "key"       // single key (reactive)
	InputArrowUp   = "arrow_up"  // up arrow (after control char parsing)
	InputArrowDn   = "arrow_dn"  // down arrow
	InputArrowLt   = "arrow_lt"  // left arrow
	InputArrowRt   = "arrow_rt"  // right arrow
	InputHome      = "home"      // Home key (CSI H/F, SS3 H/F, CSI 1~/4~)
	InputEnd       = "end"       // End key
	InputDelete    = "delete"    // Delete forward key (CSI 3~)
	InputEsc       = "esc"       // ESC (interrupt)
	InputCtrlC     = "ctrl_c"    // Ctrl+C (cancel)
	InputTab       = "tab"       // tab
	InputBackspace = "backspace" // backspace
	InputEnter     = "enter"     // enter
	InputEOF       = "eof"       // standard streaming EOF
)

// InputEvent is a single unified input event produced by an InputSource.
// Kind is one of the InputKind constants; Data carries the payload
// (line content for InputLine, the key byte/character for InputKey, etc.).
type InputEvent struct {
	Kind InputKind
	Data string
}

// InputSource abstracts input across modes (standard streaming vs reactive).
// Implementations: StdioSource (line/EOF, pipe-friendly) and RawKeySource
// (single keys with control characters) plus future WSSource (web mode).
// See docs/output-architecture.md section 3.6.2.
type InputSource interface {
	// NextEvent blocks until the next input event or the context is cancelled.
	// It returns (InputEvent{Kind: InputEOF}, io.EOF) on end-of-input.
	NextEvent(ctx context.Context) (InputEvent, error)
}
