// Package agent - JSON-Lines stream event renderer (FEATURE-307b). The
// StreamRenderer name was reserved by the FEATURE-307a rename (see
// line_renderer.go) for exactly this machine-readable renderer.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonEventLine is the wire format of one JSON-Lines event. Field rules:
// type is always present; level only when non-info (the zero value);
// chan only when set; text/meta only when non-empty. No timestamps, no ANSI
// escape sequences, no emoji prefixes — the payload is purely semantic.
type jsonEventLine struct {
	Type  string            `json:"type"`
	Level string            `json:"level,omitempty"`
	Chan  string            `json:"chan,omitempty"`
	Text  string            `json:"text,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// StreamRenderer renders each StreamEvent as one JSON line (JSON-Lines) to
// an io.Writer, for machine consumption (--output-format json). Incremental
// events (EventContentChunk, EventToolCallStream) are emitted as-is with
// their incremental Text fragment; consumers concatenate the fragments.
// docs/output-architecture.md 3.4.
type StreamRenderer struct {
	w io.Writer
}

// NewStreamRenderer creates a StreamRenderer writing JSON-Lines to w.
func NewStreamRenderer(w io.Writer) *StreamRenderer {
	return &StreamRenderer{w: w}
}

// Render serializes one stream event as a single JSON line. Events whose
// Type is empty are skipped; marshal failures are dropped silently (a
// StreamEvent of plain strings cannot realistically fail to marshal).
func (r *StreamRenderer) Render(ev StreamEvent) {
	if ev.Type == "" {
		return
	}
	line := jsonEventLine{
		Type: ev.Type,
		Text: ev.Text,
		Meta: ev.Meta,
	}
	if ev.Level != LevelInfo {
		line.Level = ev.Level.String()
	}
	if ev.Chan != "" {
		line.Chan = string(ev.Chan)
	}
	data, err := json.Marshal(line)
	if err != nil {
		return
	}
	fmt.Fprintf(r.w, "%s\n", data)
}
