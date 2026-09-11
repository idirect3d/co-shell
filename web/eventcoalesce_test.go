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
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// chunk builds a streaming fragment event.
func chunk(typ string, text string) agent.StreamEvent {
	return agent.StreamEvent{Type: typ, Chan: agent.ChannelLLM, Text: text}
}

// TestCoalescerMergesContentChunks verifies many content fragments collapse
// into a single complete content event (FEATURE-507).
func TestCoalescerMergesContentChunks(t *testing.T) {
	c := newEventCoalescer()
	var out []agent.StreamEvent
	for _, s := range []string{"Hello", ", ", "world", "!"} {
		out = append(out, c.Add(chunk(agent.EventContentChunk, s))...)
	}
	// Nothing is emitted while the block is still open.
	if len(out) != 0 {
		t.Fatalf("expected no events while streaming, got %d", len(out))
	}
	// A non-streaming event closes the block.
	out = append(out, c.Add(agent.StreamEvent{Type: agent.EventTokenIter})...)
	if len(out) != 2 {
		t.Fatalf("expected content + token_iter, got %d events", len(out))
	}
	if out[0].Type != agent.EventContent {
		t.Fatalf("expected coalesced type %q, got %q", agent.EventContent, out[0].Type)
	}
	if out[0].Text != "Hello, world!" {
		t.Fatalf("expected merged text, got %q", out[0].Text)
	}
	if out[1].Type != agent.EventTokenIter {
		t.Fatalf("expected token_iter to pass through, got %q", out[1].Type)
	}
}

// TestCoalescerSplitsOnTypeChange verifies a thinking fragment closes the
// content block and opens a new one.
func TestCoalescerSplitsOnTypeChange(t *testing.T) {
	c := newEventCoalescer()
	c.Add(chunk(agent.EventContentChunk, "answer"))
	out := c.Add(chunk(agent.EventThinkingChunk, "reasoning"))
	if len(out) != 1 {
		t.Fatalf("expected the content block to be flushed, got %d events", len(out))
	}
	if out[0].Type != agent.EventContent || out[0].Text != "answer" {
		t.Fatalf("unexpected flushed event: %+v", out[0])
	}
	out = c.Add(agent.StreamEvent{Type: agent.EventDone})
	if len(out) != 2 {
		t.Fatalf("expected thinking + done, got %d events", len(out))
	}
	if out[0].Type != agent.EventThinking || out[0].Text != "reasoning" {
		t.Fatalf("unexpected thinking event: %+v", out[0])
	}
}

// TestCoalescerSplitsToolOnHeader verifies a new "⚙️ <tool>" header opens a
// fresh TOOL block instead of appending to the previous one.
func TestCoalescerSplitsToolOnHeader(t *testing.T) {
	c := newEventCoalescer()
	c.Add(chunk(agent.EventToolCallStream, "\u2699\uFE0F read_file\n{\"path\":\"a\"}"))
	out := c.Add(chunk(agent.EventToolCallStream, "\u2699\uFE0F write_file\n{\"path\":\"b\"}"))
	if len(out) != 1 {
		t.Fatalf("expected the first tool block to be flushed, got %d events", len(out))
	}
	if out[0].Type != agent.EventToolCall {
		t.Fatalf("expected tool_call, got %q", out[0].Type)
	}
	out = c.Add(agent.StreamEvent{Type: agent.EventDone})
	if len(out) != 2 {
		t.Fatalf("expected second tool block + done, got %d events", len(out))
	}
	if out[0].Text != "\u2699\uFE0F write_file\n{\"path\":\"b\"}" {
		t.Fatalf("unexpected second tool text: %q", out[0].Text)
	}
}

// TestCoalescerPassesThroughNonStreaming verifies ordinary events are emitted
// unchanged and in order.
func TestCoalescerPassesThroughNonStreaming(t *testing.T) {
	c := newEventCoalescer()
	evs := []agent.StreamEvent{
		{Type: agent.EventToolCall, Text: "result"},
		{Type: agent.EventTokenIter},
		{Type: agent.EventDone},
	}
	var out []agent.StreamEvent
	for _, ev := range evs {
		out = append(out, c.Add(ev)...)
	}
	if len(out) != len(evs) {
		t.Fatalf("expected %d events, got %d", len(evs), len(out))
	}
	for i := range evs {
		if out[i].Type != evs[i].Type {
			t.Fatalf("event %d: expected %q, got %q", i, evs[i].Type, out[i].Type)
		}
	}
}

// TestCoalescerDropsEmptyBlock verifies a block with no text is not persisted.
func TestCoalescerDropsEmptyBlock(t *testing.T) {
	c := newEventCoalescer()
	c.Add(chunk(agent.EventContentChunk, ""))
	out := c.Add(agent.StreamEvent{Type: agent.EventDone})
	if len(out) != 1 || out[0].Type != agent.EventDone {
		t.Fatalf("expected only the done event, got %+v", out)
	}
}

// TestMarshalEventUsesWireFormat is the regression guard for the replay bug
// where persisted events used capitalised keys ("Type"/"Chan"/…) because
// agent.StreamEvent has no JSON tags. The browser reads ev.type / ev.chan, so
// every replayed block fell through to the SYS fallback.
func TestMarshalEventUsesWireFormat(t *testing.T) {
	ev := agent.StreamEvent{
		Type: agent.EventContent,
		Chan: agent.ChannelLLM,
		Text: "hello",
		Meta: map[string]string{"msg_index": "3"},
	}
	data, err := marshalEvent(ev)
	if err != nil {
		t.Fatalf("marshalEvent: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"type", "chan", "text", "meta"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("persisted event is missing lowercase key %q: %s", key, data)
		}
	}
	for _, key := range []string{"Type", "Chan", "Text", "Meta"} {
		if _, ok := decoded[key]; ok {
			t.Fatalf("persisted event must not use capitalised key %q: %s", key, data)
		}
	}
	if decoded["type"] != agent.EventContent {
		t.Fatalf("unexpected type: %v", decoded["type"])
	}
}

// TestCoalescerReducesEventCount is the regression guard for the replay bug:
// a realistic stream of many fragments must collapse to a handful of events.
func TestCoalescerReducesEventCount(t *testing.T) {
	c := newEventCoalescer()
	persisted := 0
	// 200 content fragments, then a tool call in 100 fragments, then done.
	for i := 0; i < 200; i++ {
		persisted += len(c.Add(chunk(agent.EventContentChunk, "x")))
	}
	for i := 0; i < 100; i++ {
		persisted += len(c.Add(chunk(agent.EventToolCallStream, "y")))
	}
	persisted += len(c.Add(agent.StreamEvent{Type: agent.EventDone}))
	// content block + tool block + done = 3 events instead of 301.
	if persisted != 3 {
		t.Fatalf("expected 3 persisted events, got %d", persisted)
	}
}
