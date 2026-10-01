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
	"strconv"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// eventTimeKey is the internal Meta key carrying the moment a coalesced block
// started (epoch milliseconds). marshalEvent turns it into the wire field "ts"
// and drops it from the serialized meta, so the browser never sees the internal
// key (FIX-549).
const eventTimeKey = "ts"

// newEventJSON builds the wire form of one event. ts is when the event happened
// (epoch milliseconds); <= 0 falls back to now.
//
// FIX-549: the value used to be missing entirely, and the browser stamped every
// block with the moment it rendered — a replay paints a whole page in one tick,
// which is why all replayed timestamp lines looked identical.
func newEventJSON(ev agent.StreamEvent, ts int64) *eventJSON {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	ej := &eventJSON{Type: ev.Type, Text: ev.Text, Meta: ev.Meta, Ts: ts}
	if ev.Level != agent.LevelInfo {
		ej.Level = ev.Level.String()
	}
	if ev.Chan != "" {
		ej.Chan = string(ev.Chan)
	}
	return ej
}

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
	kind  string // "content" | "thinking" | "tool" | "sup"
	chan_ agent.ChannelID
	text  string
	meta  map[string]string
	// startedAt is when the block's first fragment arrived (epoch milliseconds).
	// FIX-549: the block is stamped with this instead of the flush moment, so a
	// replayed block shows when it really happened.
	startedAt int64
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
		c.pending = &pendingBlock{kind: kind, chan_: ev.Chan, meta: cloneMeta(ev.Meta), startedAt: time.Now().UnixMilli()}
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
	if p.startedAt > 0 {
		// FIX-549: carry the block's start time out through meta; newEventJSON (via
		// marshalEvent) turns it into the wire field ts.
		if p.meta == nil {
			p.meta = map[string]string{}
		}
		p.meta[eventTimeKey] = strconv.FormatInt(p.startedAt, 10)
	}
	ev := agent.StreamEvent{
		Type: coalescedType(p.kind),
		Chan: p.chan_,
		Text: p.text,
		Meta: p.meta,
	}
	// FIX-575: a merged tool block IS the start of a TOOL block, but the live
	// signal for that (the "⚙️ <tool>" header on the first streamed fragment) is
	// swallowed by the coalescing itself: what gets persisted is one tool_call
	// event without any phase, so the replay had to infer the block boundary from
	// render state (curTool.raw / curTool.hasResult) - an inference that merged a
	// tool call into the previous block or dropped its text altogether, which is
	// why tool cards went missing at random after a reload/restart. Stamp the
	// boundary explicitly instead. See agent.PhaseStream.
	if p.kind == "tool" {
		if ev.Meta == nil {
			ev.Meta = map[string]string{}
		}
		ev.Meta[agent.MetaKeyPhase] = agent.PhaseStream
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
//
// FIX-549: the persisted JSON also carries "ts": a coalesced block keeps the
// moment it started (carried in eventTimeKey), any other event is stamped with
// the moment it is written, which is its emission moment.
func marshalEvent(ev agent.StreamEvent) ([]byte, error) {
	ts := int64(0)
	if raw, ok := ev.Meta[eventTimeKey]; ok {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			ts = v
		}
		// Drop the internal key so it never reaches the browser's meta.
		filtered := make(map[string]string, len(ev.Meta))
		for k, v := range ev.Meta {
			if k != eventTimeKey {
				filtered[k] = v
			}
		}
		ev.Meta = filtered
	}
	return json.Marshal(newEventJSON(ev, ts))
}
