// Author: L.Shuang
// Created: 2026-10-01
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
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// FIX-575: a merged tool block must carry an explicit "block starts here"
// marker (meta.phase = stream). The replay decides TOOL block boundaries from
// this value; without it the browser fell back to render state (curTool.raw /
// hasResult) and merged or dropped tool calls at random after a reload.
func TestFIX575CoalescedToolBlockCarriesStreamPhase(t *testing.T) {
	c := newEventCoalescer()

	// Two fragments of one tool call; the first carries the "⚙️ <tool>" header.
	if out := c.Add(agent.NewStreamEvent(agent.EventToolCallStream, agent.ChannelTool, agent.LevelInfo,
		"⚙️ read_file\n{\"path\":")); len(out) != 0 {
		t.Fatalf("first tool fragment must be buffered, got %d event(s)", len(out))
	}
	if out := c.Add(agent.NewStreamEvent(agent.EventToolCallStream, agent.ChannelTool, agent.LevelInfo,
		"\"main.go\"}")); len(out) != 0 {
		t.Fatalf("second tool fragment must be buffered, got %d event(s)", len(out))
	}

	// The tool_call(phase=input) event flushes the accumulated block first.
	input := agent.NewStreamEvent(agent.EventToolCall, agent.ChannelTool, agent.LevelInfo, "TOOL: 读文件 - main.go")
	input.Meta = map[string]string{agent.MetaKeyPhase: agent.PhaseInput}
	out := c.Add(input)
	if len(out) != 2 {
		t.Fatalf("flush + input: got %d event(s), want 2", len(out))
	}
	if out[0].Type != agent.EventToolCall {
		t.Fatalf("flushed block type = %q, want %q", out[0].Type, agent.EventToolCall)
	}
	if got := out[0].Meta[agent.MetaKeyPhase]; got != agent.PhaseStream {
		t.Fatalf("flushed tool block meta[%s] = %q, want %q", agent.MetaKeyPhase, got, agent.PhaseStream)
	}
	if out[1].Type != agent.EventToolCall || out[1].Meta[agent.MetaKeyPhase] != agent.PhaseInput {
		t.Fatalf("input event must pass through unchanged, got type=%q phase=%q",
			out[1].Type, out[1].Meta[agent.MetaKeyPhase])
	}

	// The marker must survive serialization: that JSON is what the browser reads.
	data, err := marshalEvent(out[0])
	if err != nil {
		t.Fatalf("marshalEvent: %v", err)
	}
	var ev struct {
		Type string            `json:"type"`
		Meta map[string]string `json:"meta"`
	}
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal persisted event: %v", err)
	}
	if ev.Type != agent.EventToolCall {
		t.Fatalf("persisted type = %q, want %q", ev.Type, agent.EventToolCall)
	}
	if ev.Meta[agent.MetaKeyPhase] != agent.PhaseStream {
		t.Fatalf("persisted meta[%s] = %q, want %q (FIX-575)",
			agent.MetaKeyPhase, ev.Meta[agent.MetaKeyPhase], agent.PhaseStream)
	}
	if !strings.Contains(string(data), "⚙️") {
		t.Fatalf("persisted block must keep the streamed header text, got %s", data)
	}
}

// FIX-575: each tool invocation must produce its OWN merged block. Before the
// fix the second call's parameters could be appended to the first block (or
// dropped), which is exactly the "tool card disappeared after restore" report.
func TestFIX575EachToolCallEmitsItsOwnStreamBlock(t *testing.T) {
	c := newEventCoalescer()
	var blocks []agent.StreamEvent

	runTool := func(name, args string) {
		c.Add(agent.NewStreamEvent(agent.EventToolCallStream, agent.ChannelTool, agent.LevelInfo, "⚙️ "+name+"\n"))
		c.Add(agent.NewStreamEvent(agent.EventToolCallStream, agent.ChannelTool, agent.LevelInfo, args))
		input := agent.NewStreamEvent(agent.EventToolCall, agent.ChannelTool, agent.LevelInfo, "TOOL: "+name)
		input.Meta = map[string]string{agent.MetaKeyPhase: agent.PhaseInput}
		out := c.Add(input)
		if len(out) != 2 {
			t.Fatalf("%s: flush + input = %d event(s), want 2", name, len(out))
		}
		blocks = append(blocks, out[0])
		// Result event: must NOT start a block.
		res := agent.NewStreamEvent(agent.EventToolCall, agent.ChannelTool, agent.LevelInfo, "  Result:\nok")
		res.Meta = map[string]string{agent.MetaKeyPhase: agent.PhaseResult}
		blocks = append(blocks, c.Add(res)...)
	}

	runTool("read_file", `{"path":"a.go"}`)
	runTool("read_file", `{"path":"b.go"}`)

	var streamBlocks []agent.StreamEvent
	for _, ev := range blocks {
		if ev.Meta[agent.MetaKeyPhase] == agent.PhaseStream {
			streamBlocks = append(streamBlocks, ev)
		}
	}
	if len(streamBlocks) != 2 {
		t.Fatalf("merged tool blocks = %d, want 2 (one per tool call)", len(streamBlocks))
	}
	if strings.Contains(streamBlocks[1].Text, "a.go") {
		t.Fatalf("second tool block must not carry the first call's parameters: %q", streamBlocks[1].Text)
	}
	if !strings.Contains(streamBlocks[1].Text, "b.go") {
		t.Fatalf("second tool block lost its own parameters: %q", streamBlocks[1].Text)
	}
}

// FIX-575: non-tool blocks must not be stamped (the marker means "TOOL block
// starts here", and misusing it would open a spurious tool block on replay).
func TestFIX575OnlyToolBlocksAreStamped(t *testing.T) {
	c := newEventCoalescer()
	c.Add(agent.NewStreamEvent(agent.EventContentChunk, agent.ChannelLLM, agent.LevelInfo, "hello "))
	out := c.Add(agent.NewStreamEvent(agent.EventContentChunk, agent.ChannelLLM, agent.LevelInfo, "world"))
	if len(out) != 0 {
		t.Fatalf("content fragments must stay buffered, got %d event(s)", len(out))
	}
	out = c.Add(agent.NewStreamEvent(agent.EventTokenIter, agent.ChannelLLM, agent.LevelInfo, ""))
	if len(out) != 2 {
		t.Fatalf("flush + token_iter = %d event(s), want 2", len(out))
	}
	if out[0].Type != agent.EventContent {
		t.Fatalf("flushed content type = %q, want %q", out[0].Type, agent.EventContent)
	}
	if _, ok := out[0].Meta[agent.MetaKeyPhase]; ok {
		t.Fatalf("content block must not carry %s, got %q", agent.MetaKeyPhase, out[0].Meta[agent.MetaKeyPhase])
	}
}
