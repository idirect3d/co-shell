// Package web - FIX-549 tests: a replayed record must look like the live view.
//
// The v0.71.0 replay path had three defects: block titles were derived from the
// channel alone (a THINK block was titled "LLM"), a tool block did not close the
// LLM block before it, so text emitted after the tool call reappeared after the
// tool card on replay, and persisted events carried no time at all — the browser
// stamped them while painting, which made every replayed line show the same
// clock value.
//
// UC-0001..UC-0008 map onto the use-case document
// (use-case/FIX-549/FIX-549-UC-0001.md).
//
// Author: L.Shuang
// Created: 2026-09-21
package web

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/agent"
)

// fix549Event builds a stream event with the LLM channel for the tests below.
func fix549Event(typ, text string) agent.StreamEvent {
	return agent.NewStreamEvent(typ, agent.ChannelLLM, agent.LevelInfo, text)
}

// UC-0003: every persisted event carries the moment it happened, so a replay can
// show real times instead of the moment the page happened to be painted.
func TestFix549PersistedEventCarriesTime(t *testing.T) {
	before := time.Now().UnixMilli()
	data, err := marshalEvent(fix549Event(agent.EventToolCall, "x"))
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("marshalEvent: %v", err)
	}
	var got struct {
		Ts int64 `json:"ts"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal persisted event: %v", err)
	}
	if got.Ts < before || got.Ts > after {
		t.Fatalf("persisted ts = %d, want within [%d, %d]", got.Ts, before, after)
	}
}

// UC-0005: a coalesced block is stamped with the moment its FIRST fragment
// arrived, not with the moment it was flushed — otherwise a long block would be
// dated when it ended.
func TestFix549CoalescedBlockKeepsStartTime(t *testing.T) {
	c := newEventCoalescer()
	start := time.Now().UnixMilli()
	if out := c.Add(fix549Event(agent.EventContentChunk, "hello")); len(out) != 0 {
		t.Fatalf("first fragment returned %d events, want 0 (it is absorbed)", len(out))
	}
	time.Sleep(30 * time.Millisecond)
	// A different block kind closes the content block.
	out := c.Add(fix549Event(agent.EventThinkingChunk, "reasoning"))
	if len(out) != 1 || out[0].Type != agent.EventContent {
		t.Fatalf("flush returned %v, want one content block", out)
	}
	data, err := marshalEvent(out[0])
	if err != nil {
		t.Fatalf("marshalEvent: %v", err)
	}
	var got struct {
		Ts   int64             `json:"ts"`
		Meta map[string]string `json:"meta"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal flush: %v", err)
	}
	if got.Ts < start {
		t.Fatalf("block ts = %d, want >= block start %d", got.Ts, start)
	}
	if got.Ts >= time.Now().UnixMilli() {
		t.Fatalf("block ts = %d, want the block START, not the flush moment", got.Ts)
	}
	if _, leaked := got.Meta[eventTimeKey]; leaked {
		t.Fatalf("internal key %q leaked into the browser meta: %v", eventTimeKey, got.Meta)
	}
}

// UC-0004: live frames carry the same timestamp field.
func TestFix549LiveFrameCarriesTime(t *testing.T) {
	if ts := newEventJSON(fix549Event(agent.EventContent, "x"), 0).Ts; ts <= 0 {
		t.Fatalf("live frame ts = %d, want > 0", ts)
	}
	const want = int64(1737000000123)
	if ts := newEventJSON(fix549Event(agent.EventContent, "x"), want).Ts; ts != want {
		t.Fatalf("live frame ts = %d, want the supplied %d", ts, want)
	}
}

// UC-0001: the title of a block comes from its event type, so a replayed THINK
// block is titled THINK (it used to be titled LLM because only the channel was
// consulted).
func TestFix549FrontendLabelsByType(t *testing.T) {
	src := uc548ReadAppJS(t)
	at := strings.Index(src, "function eventBlockLabel(")
	if at < 0 {
		t.Fatal("app.js has no eventBlockLabel helper: replayed titles would fall back to the channel")
	}
	seg := uc548Window(src, at, 700)
	for _, want := range []string{
		`case "content_chunk": case "content": return "LLM"`,
		`case "thinking_chunk": case "thinking": return "THINK"`,
		`case "tool_call": case "tool_call_stream": return "TOOL"`,
	} {
		if !strings.Contains(seg, want) {
			t.Errorf("eventBlockLabel does not map %s", want)
		}
	}
	if !strings.Contains(src, "const label = eventBlockLabel(ev);") {
		t.Error("renderEvent still derives the block title from the channel alone")
	}
}

// UC-0002: starting a tool block closes the LLM block before it, in both the
// streaming and the non-streaming tool paths, so live and replay agree on
// [LLM][TOOL][LLM].
func TestFix549ToolBlockClosesLLMBlock(t *testing.T) {
	src := uc548ReadAppJS(t)

	streamAt := strings.Index(src, `const isNewTool = ev.text && ev.text.includes("⚙️");`)
	if streamAt < 0 {
		t.Fatal("streaming tool branch not found in app.js")
	}
	if seg := uc548Window(src, streamAt, 900); !strings.Contains(seg, "curLLM = null;") {
		t.Error("streaming tool branch does not close the current LLM block")
	}

	plainAt := strings.Index(src, "if (reuse) curTool = reuse;")
	if plainAt < 0 {
		t.Fatal("non-streaming tool branch not found in app.js")
	}
	if seg := uc548Window(src, plainAt, 400); !strings.Contains(seg, "curLLM = null;") {
		t.Error("non-streaming tool branch does not close the current LLM block")
	}
}

// UC-0006: the iteration line shows the event's own time; a render-time stamp
// must no longer exist, because a replay paints the whole page in one tick.
func TestFix549FrontendUsesEventTime(t *testing.T) {
	src := uc548ReadAppJS(t)
	if !strings.Contains(src, "fmtTime(ev.ts ? new Date(ev.ts) : new Date())") {
		t.Error("iteration line does not prefer the event timestamp (ev.ts)")
	}
	if strings.Contains(src, "fmtTime(new Date())") {
		t.Error("a render-time timestamp is still in use")
	}
}

// UC-0007: a replay keeps the natural (append) order. Timestamps are display
// data only: events stored with DESCENDING ts must still come back in the order
// they were appended.
func TestFix549ReplayOrderIgnoresTimestamp(t *testing.T) {
	st := newHistoryStore(t)
	const sessionID = "sess-fix549"

	want := []string{"first", "second", "third"}
	for i, text := range want {
		ev := fix549Event(agent.EventContent, text)
		ev.Meta = map[string]string{
			"msg_index":  "1",
			eventTimeKey: strconv.FormatInt(int64(9000-i*1000), 10), // descending
		}
		data, err := marshalEvent(ev)
		if err != nil {
			t.Fatalf("marshalEvent(%s): %v", text, err)
		}
		if err := st.AppendEvent(sessionID, data); err != nil {
			t.Fatalf("AppendEvent(%s): %v", text, err)
		}
	}

	events, _, _ := loadHistoryPage(st, sessionID, 5, 0)
	if len(events) != len(want) {
		t.Fatalf("history returned %d events, want %d", len(events), len(want))
	}
	for i, data := range events {
		var got struct {
			Text string `json:"text"`
			Ts   int64  `json:"ts"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal history[%d]: %v", i, err)
		}
		if got.Text != want[i] {
			t.Errorf("history[%d] = %q, want %q (append order must win over ts)", i, got.Text, want[i])
		}
		if got.Ts <= 0 {
			t.Errorf("history[%d] (%s) has no ts: replay would fall back to render time", i, got.Text)
		}
	}
}

// UC-0008: the coalescing rules themselves are unchanged — the fragments of one
// block still merge into a single event, and a kind change still closes it.
func TestFix549CoalescingRulesUnchanged(t *testing.T) {
	c := newEventCoalescer()
	c.Add(fix549Event(agent.EventContentChunk, "a"))
	c.Add(fix549Event(agent.EventContentChunk, "b"))
	out := c.Add(fix549Event(agent.EventThinkingChunk, "t"))
	if len(out) != 1 {
		t.Fatalf("flush returned %d events, want 1", len(out))
	}
	if got := out[0].Text; got != "ab" {
		t.Fatalf("merged text = %q, want %q (fragment merging must not change)", got, "ab")
	}
	if out[0].Type != agent.EventContent {
		t.Fatalf("merged type = %q, want %q", out[0].Type, agent.EventContent)
	}
	if rest := c.flush(); len(rest) != 1 || rest[0].Type != agent.EventThinking {
		t.Fatalf("final flush = %v, want the thinking block", fmt.Sprint(rest))
	}
}
