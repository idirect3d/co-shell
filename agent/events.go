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
	EventToolCallDiff   = "tool_call_diff"   // FEATURE-424: unified diff rendering for a completed replace_in_file call
	EventTaskPlan       = "task_plan"        // FEATURE-307c: full task plan snapshot (Meta[MetaKeyPlan] = plan JSON, "" when archived)

	// FEATURE-524: LLM-driven rich component output. EventUIRender carries a
	// validated component tree for the Web UI to paint; EventUIUpdate replaces
	// the content of an already rendered node addressed by Meta[MetaKeyUIID].
	EventUIRender = "ui_render" // component tree to render (Meta[MetaKeyUIID] + Meta[MetaKeyUITree])
	EventUIUpdate = "ui_update" // in-place update of a rendered tree (Meta[MetaKeyUIID] + Meta[MetaKeyUIPatch])

	// FEATURE-524 window mode: EventUIWindow opens or closes the floating UI
	// window (Meta[MetaKeyUIWindowAction] + Meta[MetaKeyUIWindowTitle]). The
	// window only carries its title on open; its content is written by
	// ui_render/ui_update events whose Meta[MetaKeyUITarget] is UITargetWindow.
	EventUIWindow = "ui_window"
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

// MetaKeySupScenario is the Meta key of a content_chunk event on the
// supervisor channel carrying the SUP block title (e.g. "SUP·问题解决") for
// the three SUP LLM-interaction scenarios (FEATURE-460). The frontend uses it
// to render a streaming SUP block with the correct scenario title.
const MetaKeySupScenario = "sup_scenario"

// MetaKeySupPart is the Meta key of a content_chunk event on the supervisor
// channel carrying which part of the SUP block the chunk belongs to: "prompt"
// (the prompt sent to the LLM), "content" (the streaming reply), or "tool"
// (a tool call's input) (FEATURE-461). The frontend routes each chunk to the
// corresponding section of the SUP block.
const MetaKeySupPart = "sup_part"

// MetaKeyToolSummary is the Meta key of EventToolCall carrying the structured
// ToolSummary JSON (FEATURE-388). Web/JSON consumers use it to render a
// structured tool card; the LineRenderer ignores Meta, so terminal output is
// unaffected.
const MetaKeyToolSummary = "tool_summary"

// MetaKeyDiffLines is the Meta key of EventToolCallDiff carrying the
// structured per-line diff data (JSON array of {line, status}) so the frontend
// can colour each line directly without re-parsing text markers (FEATURE-424).
const MetaKeyDiffLines = "diff_lines"

// MetaKeyPhase marks the phase of an EventToolCall within one tool
// invocation (FEATURE-362): PhaseInput carries the pre-execution summary,
// PhaseResult carries the post-execution result. Web/JSON consumers use it
// to merge input+result into a single block; the LineRenderer ignores Meta,
// so terminal output is unaffected.
const (
	MetaKeyPhase = "phase"
	PhaseInput   = "input"
	PhaseResult  = "result"
)

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

// withPhase returns ev with Meta[MetaKeyPhase] set (FEATURE-362). It exists
// so tool-call emitters can tag input/result without breaking the
// NewStreamEvent nil-Meta shape that existing tests rely on.
func withPhase(ev StreamEvent, phase string) StreamEvent {
	if ev.Meta == nil {
		ev.Meta = map[string]string{}
	}
	ev.Meta[MetaKeyPhase] = phase
	return ev
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

// MetaKeyUIID is the Meta key of the UI events (FEATURE-524) carrying the id
// of a rendered component tree — the handle the frontend (and a later
// ui_update) uses to address it.
const MetaKeyUIID = "ui_id"

// MetaKeyUITree is the Meta key of EventUIRender carrying the validated
// component tree as a JSON string.
const MetaKeyUITree = "ui_tree"

// MetaKeyUIPatch is the Meta key of EventUIUpdate carrying the replacement
// subtree as a JSON string.
const MetaKeyUIPatch = "ui_patch"

// MetaKeyUITarget is the Meta key of EventUIRender/EventUIUpdate carrying the
// surface the tree must be painted on: UITargetStream ("") or UITargetWindow.
// It is only set when the target is not the default chat stream, so a stream
// render keeps its original wire shape (FEATURE-524 window mode).
const MetaKeyUITarget = "ui_target"

// MetaKeyUIWindowAction and MetaKeyUIWindowTitle are the Meta keys of
// EventUIWindow: the action (open/close) and the window title (open only).
const (
	MetaKeyUIWindowAction = "ui_window_action"
	MetaKeyUIWindowTitle  = "ui_window_title"
	// MetaKeyUIWindowSize carries the size preset of an open window
	// (auto/small/medium/large); it is absent on close.
	MetaKeyUIWindowSize = "ui_window_size"
)

// UIRenderEvent builds an EventUIRender event carrying one component tree
// (FEATURE-524). It is emitted on ChannelSystem on purpose: the LLM/tool/
// command channels are filtered by the seven show-* switches, while the Web
// UI must receive the tree however verbose the user wants the stream. The
// LineRenderer has no case for this type, so terminals ignore the event and
// fall back to the LLM's plain-text reply.
func UIRenderEvent(uiID, treeJSON string) StreamEvent {
	return StreamEvent{
		Type:  EventUIRender,
		Chan:  ChannelSystem,
		Level: LevelInfo,
		Meta:  map[string]string{MetaKeyUIID: uiID, MetaKeyUITree: treeJSON},
	}
}

// UIUpdateEvent builds an EventUIUpdate event replacing the content of the
// tree addressed by uiID (FEATURE-524).
func UIUpdateEvent(uiID, patchJSON string) StreamEvent {
	return StreamEvent{
		Type:  EventUIUpdate,
		Chan:  ChannelSystem,
		Level: LevelInfo,
		Meta:  map[string]string{MetaKeyUIID: uiID, MetaKeyUIPatch: patchJSON},
	}
}

// UIRenderEventTo is UIRenderEvent for a named surface: the target is added to
// the Meta only when it is not the default chat stream, so stream renders keep
// the exact wire shape of UC-08 (FEATURE-524 window mode).
func UIRenderEventTo(uiID, treeJSON, target string) StreamEvent {
	ev := UIRenderEvent(uiID, treeJSON)
	if target != "" && target != UITargetStream {
		ev.Meta[MetaKeyUITarget] = target
	}
	return ev
}

// UIUpdateEventTo is UIUpdateEvent for a named surface (FEATURE-524 window
// mode): the patch is addressed to the window when target is UITargetWindow.
func UIUpdateEventTo(uiID, patchJSON, target string) StreamEvent {
	ev := UIUpdateEvent(uiID, patchJSON)
	if target != "" && target != UITargetStream {
		ev.Meta[MetaKeyUITarget] = target
	}
	return ev
}

// UIWindowEvent builds an EventUIWindow opening (action=open) or closing
// (action=close) the floating UI window (FEATURE-524 window mode). The size
// preset rides along on open (FEATURE-524 size presets); the frontend turns it
// into pixels, clamped to the viewport. It rides
// ChannelSystem for the same reason as the other UI events: it belongs to the
// Web UI regardless of the show-* switches, and terminals have no case for it.
func UIWindowEvent(action, title, size string) StreamEvent {
	meta := map[string]string{MetaKeyUIWindowAction: action}
	if title != "" {
		meta[MetaKeyUIWindowTitle] = title
	}
	if size != "" {
		meta[MetaKeyUIWindowSize] = size
	}
	return StreamEvent{
		Type:  EventUIWindow,
		Chan:  ChannelSystem,
		Level: LevelInfo,
		Meta:  meta,
	}
}
