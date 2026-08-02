// Package agent - semantic render command abstraction.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

// RenderKind is a semantic rendering instruction type. UI components
// (Box/Menu/Step/Dialog) publish RenderCommand, and renderers decide the
// actual presentation (line / full-screen / stream / web). This decouples
// UI components from rendering mode (docs/output-architecture.md 3.7).
type RenderKind string

const (
	RenderText     RenderKind = "text"     // plain text info
	RenderTitle    RenderKind = "title"    // title
	RenderBox      RenderKind = "box"      // box / panel
	RenderMenu     RenderKind = "menu"     // menu with selectable items
	RenderStep     RenderKind = "step"     // wizard step
	RenderSep      RenderKind = "sep"      // separator
	RenderDialog   RenderKind = "dialog"   // confirmation dialog
	RenderProgress RenderKind = "progress" // progress indicator
)

// RenderCommand is produced by UI components and consumed by renderers.
// It carries no ANSI / layout details — those are internal to renderers.
type RenderCommand struct {
	Kind    RenderKind
	Content string
	Level   Level             // color / weight (applied by renderer)
	Chan    ChannelID         // business channel for filtering
	Meta    map[string]string // reserved: menu items, step numbers, etc.
}
