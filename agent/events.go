// Package agent - output stream event constants and the structured
// StreamEvent protocol (FEATURE-307a).
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import "strconv"

// Event type constants replace magic string literals passed to StreamCallback
// (e.g. cb(EventContentChunk) instead of a bare string).
// They are the single source of truth for stream event types emitted by the
// agent loop and consumed by REPL/main renderers.
// See docs/output-architecture.md section 3.3.
const (
	EventContentChunk   = "content_chunk"    // LLM streaming content
	EventThinkingChunk  = "thinking_chunk"   // LLM streaming thinking
	EventContent        = "content"          // LLM non-streaming content
	EventThinking       = "thinking"         // LLM non-streaming thinking
	EventCommand        = "command"          // system command
	EventOutput         = "output"           // command output
	EventToolCall       = "tool_call"        // tool call (name/args/result)
	EventTokenIter      = "token_iter"       // iteration token usage
	EventTokenTask      = "token_task"       // task token usage
	EventInfo           = "info"             // informational message
	EventWarning        = "warning"          // warning message
	EventError          = "error"            // error message
	EventDone           = "done"             // done marker
	EventToolCallStream = "tool_call_stream" // FEATURE-235: streaming tool-call render (show-tool / show-tool-input gated)
	EventTaskPlan       = "task_plan"        // FEATURE-307c: full task plan snapshot (Meta[MetaKeyPlan] = plan JSON, "" when archived)
)

// StreamEvent is the structured stream event emitted by the agent loop and
// consumed by renderers (FEATURE-307a). It replaces the former
// (eventType string, content string) pair: the payload Text is purely
// semantic — no emoji prefixes, no separator lines, no decorative newlines;
// all presentation decoration is applied by the renderer (LineRenderer)
// based on Type and Level. Meta carries structured numeric data (e.g. token
// statistics) so renderers no longer re-parse formatted strings.
//
// The Type values reuse the Event* constants above unchanged, so the event
// protocol can be serialized directly (e.g. JSON-Lines) in later phases.
type StreamEvent struct {
	Type  string            // one of the Event* constants
	Level Level             // importance level (out.go), drives emoji/layout
	Chan  ChannelID         // business channel (out.go), for filtering/routing
	Text  string            // pure semantic content, undecorated
	Meta  map[string]string // structured data (token stats etc.)
}

// Meta keys used by the token usage events (EventTokenIter/EventTokenTask).
const (
	MetaKeyPrompt     = "prompt"     // prompt tokens of the iteration/task
	MetaKeyCompletion = "completion" // completion tokens of the iteration/task
	MetaKeyTotal      = "total"      // total tokens of the iteration/task
	MetaKeyMax        = "max"        // model max context length (0 = unknown)
	MetaKeyFT         = "ft"         // first-token latency display string
	MetaKeyInTPS      = "in_tps"     // input tokens-per-second display string
	MetaKeyOutTPS     = "out_tps"    // output tokens-per-second display string
)

// MetaKeyPlan is the Meta key of EventTaskPlan carrying the full task plan
// as a JSON string ("" means the plan was archived/cleared — consumers hide
// the plan panel).
const MetaKeyPlan = "plan"

// EventRenderer is the single sink interface for structured stream events
// (FEATURE-307b). LineRenderer renders events as decorated terminal lines;
// StreamRenderer renders them as JSON-Lines for machine consumption.
type EventRenderer interface {
	Render(ev StreamEvent)
}

// NewStreamEvent builds a StreamEvent with the given type, channel, level
// and text. Meta is left nil.
func NewStreamEvent(typ string, ch ChannelID, lv Level, text string) StreamEvent {
	return StreamEvent{Type: typ, Chan: ch, Level: lv, Text: text}
}

// InfoEvent builds an EventInfo event at LevelInfo (rendered verbatim).
func InfoEvent(ch ChannelID, text string) StreamEvent {
	return NewStreamEvent(EventInfo, ch, LevelInfo, text)
}

// OKEvent builds an EventInfo event at LevelSuccess (success emoji + layout).
func OKEvent(ch ChannelID, text string) StreamEvent {
	return NewStreamEvent(EventInfo, ch, LevelSuccess, text)
}

// WarnEvent builds an EventInfo event at LevelWarning (warning emoji + layout).
func WarnEvent(ch ChannelID, text string) StreamEvent {
	return NewStreamEvent(EventInfo, ch, LevelWarning, text)
}

// ErrEvent builds an EventInfo event at LevelError (error emoji + layout).
func ErrEvent(ch ChannelID, text string) StreamEvent {
	return NewStreamEvent(EventInfo, ch, LevelError, text)
}

// TokenIterEvent builds an EventTokenIter event with the per-iteration token
// usage carried in Meta (numbers as decimal strings).
func TokenIterEvent(prompt, completion, total, max int, ft, inTPS, outTPS string) StreamEvent {
	return StreamEvent{
		Type:  EventTokenIter,
		Chan:  ChannelLLM,
		Level: LevelInfo,
		Meta: map[string]string{
			MetaKeyPrompt:     strconv.Itoa(prompt),
			MetaKeyCompletion: strconv.Itoa(completion),
			MetaKeyTotal:      strconv.Itoa(total),
			MetaKeyMax:        strconv.Itoa(max),
			MetaKeyFT:         ft,
			MetaKeyInTPS:      inTPS,
			MetaKeyOutTPS:     outTPS,
		},
	}
}

// TokenTaskEvent builds an EventTokenTask event with the task-level token
// usage carried in Meta (numbers as decimal strings).
func TokenTaskEvent(prompt, completion, total int) StreamEvent {
	return StreamEvent{
		Type:  EventTokenTask,
		Chan:  ChannelLLM,
		Level: LevelInfo,
		Meta: map[string]string{
			MetaKeyPrompt:     strconv.Itoa(prompt),
			MetaKeyCompletion: strconv.Itoa(completion),
			MetaKeyTotal:      strconv.Itoa(total),
		},
	}
}

// TaskPlanEvent builds an EventTaskPlan event carrying the full task plan
// snapshot as a JSON string in Meta[MetaKeyPlan] (FEATURE-307c). An empty
// planJSON signals that the plan was archived/cleared so web consumers hide
// the plan panel. The LineRenderer has no case for this type, so terminal
// output is unaffected.
func TaskPlanEvent(planJSON string) StreamEvent {
	return StreamEvent{
		Type:  EventTaskPlan,
		Chan:  ChannelTaskPlan,
		Level: LevelInfo,
		Meta:  map[string]string{MetaKeyPlan: planJSON},
	}
}
