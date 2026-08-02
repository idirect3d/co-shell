// Package agent - unified output channel abstraction.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// ChannelID identifies the business category of an output, used for
// filtering and region-based routing (docs/output-architecture.md 3.2).
type ChannelID string

const (
	ChannelLLM      ChannelID = "llm"      // LLM content / thinking
	ChannelTool     ChannelID = "tool"     // tool call, args, result
	ChannelCommand  ChannelID = "command"  // system command and output
	ChannelDebug    ChannelID = "debug"    // debug / loop detection / diagnostics
	ChannelWizard   ChannelID = "wizard"   // interactive wizard / menu
	ChannelSystem   ChannelID = "system"   // welcome / help / status / cleanup
	ChannelMCP      ChannelID = "mcp"      // MCP server status
	ChannelMemory   ChannelID = "memory"   // memory management
	ChannelTaskPlan ChannelID = "taskplan" // task plan
	ChannelDB       ChannelID = "db"       // database / sync / migration
	ChannelBridge   ChannelID = "bridge"   // feishu / bridge / hub entry
	ChannelSubAgent ChannelID = "subagent" // sub-agent
)

// Level is the importance level of an output, used for styling
// (emoji prefix / color / weight).
type Level int

const (
	LevelInfo Level = iota
	LevelSuccess
	LevelWarning
	LevelError
	LevelDebug
)

// Out is the unified entry point for all user-facing output.
// Implementations route by ChannelID (for show-xx filters) and render
// by Level (emoji prefix / color). This is the P2 base interface;
// region-based rendering (Area) lands in P5.
type Out interface {
	// Emit renders one formatted output line on the given channel.
	Emit(ch ChannelID, lv Level, format string, args ...interface{})

	// Convenience methods preserving level intent.
	Info(ch ChannelID, format string, args ...interface{})
	Success(ch ChannelID, format string, args ...interface{})
	Warning(ch ChannelID, format string, args ...interface{})
	Error(ch ChannelID, format string, args ...interface{})
	Debug(ch ChannelID, format string, args ...interface{})
}

// TerminalOut renders outputs to a UserIO sink with emoji prefixes
// resolved from config. It preserves the exact current formatting:
// the prefix order and separators are unchanged from repl.go streamCallback.
type TerminalOut struct {
	io UserIO
	ep config.EmojiPrefixes
}

// NewTerminalOut creates a TerminalOut writing to io with emoji prefixes
// resolved by config.GetEmojiPrefixes(emojiEnabled).
func NewTerminalOut(io UserIO, ep config.EmojiPrefixes) *TerminalOut {
	return &TerminalOut{io: io, ep: ep}
}

// Emit renders a formatted line with the level-matched emoji prefix.
func (o *TerminalOut) Emit(ch ChannelID, lv Level, format string, args ...interface{}) {
	content := format
	if len(args) > 0 {
		content = fmt.Sprintf(format, args...)
	}
	switch lv {
	case LevelInfo:
		// Info has no prefix — matches streamCallback EventInfo behavior.
		o.io.Print(content)
	case LevelSuccess:
		o.io.Print(o.ep.Success)
		o.io.Print(content)
		o.io.Print("\n")
	case LevelWarning:
		o.io.Print(o.ep.Warning)
		o.io.Print(content)
		o.io.Print("\n")
	case LevelError:
		o.io.Print(o.ep.Error)
		o.io.Print(content)
		o.io.Print("\n")
	case LevelDebug:
		o.io.Print(o.ep.Info)
		o.io.Print(content)
		o.io.Print("\n")
	}
}

// Info renders with LevelInfo (no prefix).
func (o *TerminalOut) Info(ch ChannelID, format string, args ...interface{}) {
	o.Emit(ch, LevelInfo, format, args...)
}

// Success renders with LevelSuccess.
func (o *TerminalOut) Success(ch ChannelID, format string, args ...interface{}) {
	o.Emit(ch, LevelSuccess, format, args...)
}

// Warning renders with LevelWarning.
func (o *TerminalOut) Warning(ch ChannelID, format string, args ...interface{}) {
	o.Emit(ch, LevelWarning, format, args...)
}

// Error renders with LevelError.
func (o *TerminalOut) Error(ch ChannelID, format string, args ...interface{}) {
	o.Emit(ch, LevelError, format, args...)
}

// Debug renders with LevelDebug.
func (o *TerminalOut) Debug(ch ChannelID, format string, args ...interface{}) {
	o.Emit(ch, LevelDebug, format, args...)
}

// Box renders a titled box/panel (P3 UI component).
func (o *TerminalOut) Box(title string, content []string) {
	o.io.Print(title)
	o.io.Print("\n")
	for _, line := range content {
		o.io.Print("  ")
		o.io.Print(line)
		o.io.Print("\n")
	}
	o.io.Print("\n")
}

// Menu renders a numbered menu with selectable items (P3 UI component).
func (o *TerminalOut) Menu(items []string) {
	for i, item := range items {
		o.io.Printf("  [%d] %s\n", i+1, item)
	}
	o.io.Print("\n")
}

// Step renders a wizard step header (P3 UI component).
func (o *TerminalOut) Step(n int, name string) {
	o.io.Printf("--- %s ---\n", i18n.TF(i18n.KeyStepHeader, n, name))
	o.io.Print("\n")
}

// Sep renders a separator line (P3 UI component).
func (o *TerminalOut) Sep() {
	o.io.Print("────────────────────────────────────────────\n")
	o.io.Print("\n")
}
