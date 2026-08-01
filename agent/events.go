// Package agent - output stream event constants.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

// Event type constants replace magic string literals passed to StreamCallback
// (e.g. cb(EventContentChunk) instead of a bare string).
// They are the single source of truth for stream event types emitted by the
// agent loop and consumed by REPL/main renderers.
// See docs/output-architecture.md section 3.3.
const (
	EventContentChunk  = "content_chunk"  // LLM streaming content
	EventThinkingChunk = "thinking_chunk" // LLM streaming thinking
	EventContent       = "content"        // LLM non-streaming content
	EventThinking      = "thinking"       // LLM non-streaming thinking
	EventCommand       = "command"        // system command
	EventOutput        = "output"         // command output
	EventToolCall      = "tool_call"      // tool call (name/args/result)
	EventTokenIter     = "token_iter"     // iteration token usage
	EventTokenTask     = "token_task"     // task token usage
	EventInfo          = "info"           // informational message
	EventWarning       = "warning"        // warning message
	EventError         = "error"          // error message
	EventDone          = "done"           // done marker
)
