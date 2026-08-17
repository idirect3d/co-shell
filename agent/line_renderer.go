// Package agent - line-oriented stream event renderer (FEATURE-307a,
// formerly stream_renderer.go / StreamRenderer, renamed to LineRenderer to
// align with the ROADMAP P5 naming; the StreamRenderer name is reserved for
// the JSON-Lines stdio renderer of FEATURE-307b).
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"strconv"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// StreamMode distinguishes interactive rendering behavior that historically
// differed between REPL and single-command mode. Both modes share the same
// event-to-render pipeline; only these two presentation details vary:
//
//   - REPL adds a leading "\n" before command/tool_call events (visual
//     separation between consecutive stream events).
//   - Single-command token_iter includes the [ℹ️] prefix (FIX-289
//     enhancement); REPL token_iter omits it.
type StreamMode int

const (
	// StreamModeREPL is the interactive REPL rendering style.
	StreamModeREPL StreamMode = iota
	// StreamModeSingleCmd is the single-command mode rendering style.
	StreamModeSingleCmd
)

// LineRenderer is the single event-to-render pipeline consumed by both
// REPL and single-command mode. It converts structured StreamEvent values
// to terminal lines via UserIO. All presentation decoration (emoji prefixes,
// separator lines, decorative newlines) lives here, driven by the event
// Type and Level; the event payload Text is purely semantic.
// docs/output-architecture.md 3.4.
type LineRenderer struct {
	io   UserIO
	ep   config.EmojiPrefixes
	mode StreamMode
}

// NewLineRenderer creates a LineRenderer writing to io with the given
// emoji prefixes and rendering mode.
func NewLineRenderer(io UserIO, ep config.EmojiPrefixes, mode StreamMode) *LineRenderer {
	return &LineRenderer{io: io, ep: ep, mode: mode}
}

// Render renders one stream event. This is the merged implementation of the
// former repl.RepL.streamCallback and main.renderSingleCmdEvent switches.
//
// EventInfo is laid out by Level (FEATURE-307a):
//
//	LevelInfo    → Text verbatim (no prefix)
//	LevelSuccess → "\n" + Success prefix + Text + "\n"
//	LevelWarning → "\n" + Warning prefix + Text + "\n"
//	LevelError   → "\n" + Error prefix + Text + "\n"
//	LevelDebug   → Loop prefix + Text (loop detection messages)
//
// EventWarning/EventError keep their historical layout
// (Warning/Error prefix + Text + "\n") regardless of Level.
func (r *LineRenderer) Render(ev StreamEvent) {
	switch ev.Type {
	case EventContentChunk:
		r.io.Print(ev.Text)
	case EventThinkingChunk:
		r.io.Print(ev.Text)
	case EventContent:
		r.io.Print(r.ep.LlmOutput)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventThinking:
		r.io.Print(r.ep.Thinking)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventCommand:
		if r.mode == StreamModeREPL {
			r.io.Print("\n")
		}
		r.io.Print(r.ep.CommandInput)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventOutput:
		r.io.Print("\n")
		r.io.Print(r.ep.OutputTitle)
		r.io.Print("\n")
		r.io.Print(r.ep.OutputSep)
		r.io.Print("\n")
		r.io.Print(ev.Text)
		r.io.Print("\n")
		r.io.Print(r.ep.OutputSep)
		r.io.Print("\n")
	case EventToolCall:
		if r.mode == StreamModeREPL {
			r.io.Print("\n")
		}
		r.io.Print(r.ep.ToolCallInput)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventToolCallStream:
		// FEATURE-235: streaming tool-call render (show-tool / show-tool-input
		// gated inside the agent). The text is already incremental and
		// pre-formatted, so it is emitted verbatim without extra newlines.
		r.io.Print(ev.Text)
	case EventTokenIter:
		r.renderTokenIter(ev)
	case EventTokenTask:
		r.renderTokenTask(ev)
	case EventInfo:
		r.renderInfo(ev)
	case EventWarning:
		r.io.Print(r.ep.Warning)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventError:
		r.io.Print(r.ep.Error)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case EventDone:
		r.io.Print("\n")
	}
}

// renderInfo renders an EventInfo event with the Level-driven layout.
func (r *LineRenderer) renderInfo(ev StreamEvent) {
	switch ev.Level {
	case LevelSuccess:
		r.io.Print("\n")
		r.io.Print(r.ep.Success)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case LevelWarning:
		r.io.Print("\n")
		r.io.Print(r.ep.Warning)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case LevelError:
		r.io.Print("\n")
		r.io.Print(r.ep.Error)
		r.io.Print(ev.Text)
		r.io.Print("\n")
	case LevelDebug:
		r.io.Print(r.ep.Loop)
		r.io.Print(ev.Text)
	default:
		r.io.Print(ev.Text)
	}
}

// metaInt reads an integer value from the event Meta. The boolean result
// reports whether the key existed and parsed cleanly.
func metaInt(meta map[string]string, key string) (int, bool) {
	s, ok := meta[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// renderTokenTask renders the task-level token usage summary. Nothing is
// rendered when the Meta is incomplete or the total is zero (matches the
// historical behavior of the string-parsed variant).
func (r *LineRenderer) renderTokenTask(ev StreamEvent) {
	prompt, okP := metaInt(ev.Meta, MetaKeyPrompt)
	completion, okC := metaInt(ev.Meta, MetaKeyCompletion)
	total, okT := metaInt(ev.Meta, MetaKeyTotal)
	if !okP || !okC || !okT || total <= 0 {
		return
	}
	r.io.Print("\n────────────────────────────────────────────────────────────────────────────────\n")
	r.io.Print(i18n.TF(i18n.KeyTotalTokensLine, prompt, completion, total))
	r.io.Print("────────────────────────────────────────────────────────────────────────────────\n")
}

// renderTokenIter renders the token usage line. The single-command variant
// (FIX-289) adds the [ℹ️] prefix and the LLM timing line; REPL omits them.
func (r *LineRenderer) renderTokenIter(ev StreamEvent) {
	prompt, okP := metaInt(ev.Meta, MetaKeyPrompt)
	completion, okC := metaInt(ev.Meta, MetaKeyCompletion)
	total, okT := metaInt(ev.Meta, MetaKeyTotal)
	maxLen, okM := metaInt(ev.Meta, MetaKeyMax)
	if !okP || !okC || !okT || !okM {
		return
	}
	ft := ev.Meta[MetaKeyFT]
	inTPS := ev.Meta[MetaKeyInTPS]
	outTPS := ev.Meta[MetaKeyOutTPS]

	if r.mode == StreamModeSingleCmd {
		// Single-command mode: show timing line and Info-prefixed separators.
		r.io.Printf("\n%s────────────────────────────────────────────────────────────────────────────────\n", r.ep.Info)
		pct := 0.0
		if maxLen > 0 && total > 0 {
			pct = float64(total) * 100.0 / float64(maxLen)
		}
		r.io.Printf("%s %s\n", r.ep.Info, fmt.Sprintf(i18n.T(i18n.KeyTokenUsageDisplay), ft, prompt, inTPS, completion, outTPS, total, pct))
		r.io.Printf("%s────────────────────────────────────────────────────────────────────────────────\n", r.ep.Info)
		return
	}

	// REPL mode: only render when total > 0 (FIX-295), no Info prefix.
	if total > 0 {
		r.io.Print("\n────────────────────────────────────────────────────────────────────────────────\n")
		pct := 0.0
		if maxLen > 0 && total > 0 {
			pct = float64(total) * 100.0 / float64(maxLen)
		}
		if maxLen == 0 {
			r.io.Print(i18n.T(i18n.KeyModelMaxLenUnknown))
		}
		r.io.Print(fmt.Sprintf(i18n.T(i18n.KeyTokenUsageDisplay), ft, prompt, inTPS, completion, outTPS, total, pct))
		r.io.Print("\n")
		r.io.Print("────────────────────────────────────────────────────────────────────────────────\n")
	}
}
