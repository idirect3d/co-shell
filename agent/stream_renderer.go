// Package agent - unified stream event renderer (P2 render merge).
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

// StreamMode distinguishes interactive rendering behavior that historically
// differed between REPL and single-command mode. Both modes share the same
// event-to-render pipeline; only these two presentation details vary:
//
//   - REPL adds a leading "\n" before command/tool_call events (visual
//     separation between consecutive stream events).
//   - Single-command token_iter includes the [ℹ️] prefix and the LLM timing
//     line (FIX-289 enhancement); REPL token_iter omits both.
type StreamMode int

const (
	// StreamModeREPL is the interactive REPL rendering style.
	StreamModeREPL StreamMode = iota
	// StreamModeSingleCmd is the single-command mode rendering style.
	StreamModeSingleCmd
)

// StreamRenderer is the single event-to-render pipeline consumed by both
// REPL and single-command mode. It converts stream events to semantic
// RenderCommand and renders them via UserIO with emoji prefixes.
// docs/output-architecture.md 3.4 (TerminalRenderer merge).
type StreamRenderer struct {
	io   UserIO
	ep   config.EmojiPrefixes
	mode StreamMode
}

// NewStreamRenderer creates a StreamRenderer writing to io with the given
// emoji prefixes and rendering mode.
func NewStreamRenderer(io UserIO, ep config.EmojiPrefixes, mode StreamMode) *StreamRenderer {
	return &StreamRenderer{io: io, ep: ep, mode: mode}
}

// Render renders one stream event. This is the merged implementation of the
// former repl.RepL.streamCallback and main.renderSingleCmdEvent switches.
func (r *StreamRenderer) Render(eventType string, content string) {
	switch eventType {
	case EventContentChunk:
		r.io.Print(content)
	case EventThinkingChunk:
		r.io.Print(content)
	case EventContent:
		r.io.Print(r.ep.LlmOutput)
		r.io.Print(content)
		r.io.Print("\n")
	case EventThinking:
		r.io.Print(r.ep.Thinking)
		r.io.Print(content)
		r.io.Print("\n")
	case EventCommand:
		if r.mode == StreamModeREPL {
			r.io.Print("\n")
		}
		r.io.Print(r.ep.CommandInput)
		r.io.Print(content)
		r.io.Print("\n")
	case EventOutput:
		r.io.Print("\n")
		r.io.Print(r.ep.OutputTitle)
		r.io.Print("\n")
		r.io.Print(r.ep.OutputSep)
		r.io.Print("\n")
		r.io.Print(content)
		r.io.Print("\n")
		r.io.Print(r.ep.OutputSep)
		r.io.Print("\n")
	case EventToolCall:
		if r.mode == StreamModeREPL {
			r.io.Print("\n")
		}
		r.io.Print(r.ep.ToolCallInput)
		r.io.Print(content)
		r.io.Print("\n")
	case EventTokenIter:
		r.renderTokenIter(content)
	case EventTokenTask:
		var prompt, completion, total int
		if _, err := fmt.Sscanf(content, "prompt=%d completion=%d total=%d", &prompt, &completion, &total); err == nil && total > 0 {
			r.io.Print("\n────────────────────────────────────────────────────────────────────────────────\n")
			r.io.Print(fmt.Sprintf("本次任务 Token 总计: 输入=%d, 输出=%d, 总计=%d\n", prompt, completion, total))
			r.io.Print("────────────────────────────────────────────────────────────────────────────────\n")
		}
	case EventInfo:
		r.io.Print(content)
	case EventWarning:
		r.io.Print(r.ep.Warning)
		r.io.Print(content)
		r.io.Print("\n")
	case EventError:
		r.io.Print(r.ep.Error)
		r.io.Print(content)
		r.io.Print("\n")
	case EventDone:
		r.io.Print("\n")
	}
}

// renderTokenIter renders the token usage line. The single-command variant
// (FIX-289) adds the [ℹ️] prefix and the LLM timing line; REPL omits them.
func (r *StreamRenderer) renderTokenIter(content string) {
	var prompt, completion, total, maxLen int
	var ft, inTPS, outTPS string
	if _, err := fmt.Sscanf(content, "prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
		&prompt, &completion, &total, &maxLen, &ft, &inTPS, &outTPS); err != nil {
		return
	}

	if r.mode == StreamModeSingleCmd {
		// Single-command mode: show timing line and Info-prefixed separators.
		r.io.Printf("\n%s────────────────────────────────────────────────────────────────────────────────\n", r.ep.Info)
		pct := 0.0
		if maxLen > 0 && total > 0 {
			pct = float64(total) * 100.0 / float64(maxLen)
		}
		r.io.Printf("%s %s\n", r.ep.Info, fmt.Sprintf(i18n.T(i18n.KeyTokenUsageDisplay), ft, prompt, inTPS, completion, outTPS, total, pct))
		if ft != "" {
			r.io.Printf("%s   %s\n", r.ep.Info, fmt.Sprintf(i18n.T(i18n.KeyTokenUsageTiming), ft, inTPS, outTPS))
		}
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
			r.io.Print(" (模型最大长度未知) ")
		}
		r.io.Print(fmt.Sprintf(i18n.T(i18n.KeyTokenUsageDisplay), ft, prompt, inTPS, completion, outTPS, total, pct))
		r.io.Print("\n")
		r.io.Print("────────────────────────────────────────────────────────────────────────────────\n")
	}
}
