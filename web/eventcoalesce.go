// Author: L.Shuang
// Created: 2026-09-11
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package web

import (
	"encoding/json"

	"github.com/idirect3d/co-shell/agent"
)

// eventCoalescer turns the live event stream into a replayable one
// (FEATURE-507).
//
// The live stream is incremental: content_chunk / thinking_chunk /
// tool_call_stream carry small fragments that the browser appends to the
// *currently open* block, tracked by module-level cursors (curLLM, curTool, …).
// Those cursors do not exist during replay, so persisting the fragments
// verbatim would make every fragment open its own block — a long session
// produced hundreds of stray SYS blocks.
//
// The coalescer therefore accumulates fragments per block and emits a single
// complete event when the block ends, so replay sees exactly the same block
// structure as the live stream.
type eventCoalescer struct {
	// pending holds the accumulated fragments of the block currently open.
	pending *pendingBlock
}

// pendingBlock is one in-flight block being accumulated.
type pendingBlock struct {
	kind string // "content" | "thinking" | "tool" | "sup"
	chan_ agent.ChannelID
	text string
	meta map[string]string
}

// newEventCoalescer creates a coalescer for one session's stream.
func newEventCoalescer() *eventCoalescer {
	return &eventCoalescer{}
}

// Add feeds one live event and returns the events that should be persisted.
// It returns nil when the event was absorbed into the pending block.
func (c *eventCoalescer) Add(ev agent.StreamEvent) []agent.StreamEvent {
	switch ev.Type {
	case agent.EventContentChunk:
		return c.appendFragment("content", ev)
	case agent.EventThinkingChunk:
		return c.appendFragment("thinking", ev)
	case agent.EventToolCallStream:
		return c.appendFragment("tool", ev)
	}
	// Any other event closes the open block first, so the persisted order
	// matches the live order.
	out := c.flush()
	out = append(out, ev)
	return out
}

// appendFragment accumulates a streaming fragment, flushing first when the
// fragment starts a different block than the one being accumulated.
func (c *eventCoalescer) appendFragment(kind string, ev agent.StreamEvent) []agent.StreamEvent {
	var out []agent.StreamEvent
	// A tool fragment carrying the "⚙️ <tool>" header starts a new TOOL block.
	startsNew := c.pending == nil || c.pending.kind != kind
	if kind == "tool" && c.pending != nil && c.pending.kind == "tool" && isToolHeader(ev.Text) {
		startsNew = true
	}
	if startsNew {
		out = c.flush()
		c.pending = &pendingBlock{kind: kind, chan_: ev.Chan, meta: cloneMeta(ev.Meta)}
	}
	c.pending.text += ev.Text
	return out
}

// flush emits the accumulated block as one complete event and clears it.
func (c *eventCoalescer) flush() []agent.StreamEvent {
	p := c.pending
	if p == nil || p.text == "" {
		c.pending = nil
		return nil
	}
	c.pending = nil
	ev := agent.StreamEvent{
		Type: coalescedType(p.kind),
		Chan: p.chan_,
		Text: p.text,
		Meta: p.meta,
	}
	return []agent.StreamEvent{ev}
}

// coalescedType maps an accumulated block kind to the event type the browser
// renders as a complete block.
func coalescedType(kind string) string {
	switch kind {
	case "content":
		return agent.EventContent
	case "thinking":
		return agent.EventThinking
	case "tool":
		return agent.EventToolCall
	}
	return agent.EventContent
}

// isToolHeader reports whether a tool fragment carries the "⚙️ <tool>" header
// that opens a new TOOL block.
func isToolHeader(text string) bool {
	return len(text) > 0 && containsGear(text)
}

// containsGear checks for the gear emoji (U+2699) without relying on a regex
// literal, matching the frontend's indexOf-based check.
func containsGear(s string) bool {
	for _, r := range s {
		if r == '\u2699' {
			return true
		}
	}
	return false
}

// cloneMeta copies an event meta map so later mutations cannot affect the
// accumulated block.
func cloneMeta(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// marshalEvent serialises one event for persistence.
//
// It MUST go through eventJSON (the same wire form sendEvent uses) rather than
// marshalling agent.StreamEvent directly: StreamEvent has no JSON tags, so a
// direct marshal emits capitalised keys ("Type", "Chan", …) that the browser
// cannot read — every replayed block then fell through to the SYS fallback.
func marshalEvent(ev agent.StreamEvent) ([]byte, error) {
	ej := &eventJSON{Type: ev.Type, Text: ev.Text, Meta: ev.Meta}
	if ev.Level != agent.LevelInfo {
		ej.Level = ev.Level.String()
	}
	if ev.Chan != "" {
		ej.Chan = string(ev.Chan)
	}
	return json.Marshal(ej)
}
