// Package agent - JSON-Lines StreamRenderer tests (FEATURE-307b).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// streamRendererFixture returns the full 14-event-type sequence shared with
// the LineRenderer golden fixtures (repl/render_test.go renderTUIFixture).
func streamRendererFixture() []StreamEvent {
	return []StreamEvent{
		{Type: EventContentChunk, Chan: ChannelLLM, Text: "你好"},
		{Type: EventThinkingChunk, Chan: ChannelLLM, Text: "让我思考"},
		{Type: EventContent, Chan: ChannelLLM, Text: "完整回答内容"},
		{Type: EventThinking, Chan: ChannelLLM, Text: "完整思考内容"},
		{Type: EventCommand, Chan: ChannelCommand, Text: "ls -la"},
		{Type: EventOutput, Chan: ChannelCommand, Text: "file1.txt\nfile2.txt"},
		{Type: EventToolCall, Chan: ChannelTool, Text: "get_weather(北京)"},
		{Type: EventToolCallStream, Chan: ChannelTool, Text: "{\"city\":"},
		TokenIterEvent(100, 50, 150, 200, "deepseek", "10", "20"),
		TokenTaskEvent(100, 50, 150),
		{Type: EventInfo, Chan: ChannelSystem, Text: "调试信息"},
		{Type: EventWarning, Chan: ChannelSystem, Level: LevelWarning, Text: "警告信息"},
		{Type: EventError, Chan: ChannelSystem, Level: LevelError, Text: "错误信息"},
		{Type: EventDone, Chan: ChannelSystem},
	}
}

// decodeJSONLines splits renderer output into per-line decoded maps, failing
// the test if any line is not a single valid JSON object.
func decodeJSONLines(t *testing.T, out string) []map[string]interface{} {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	var result []map[string]interface{}
	for i, line := range lines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d is not valid JSON: %q: %v", i, line, err)
		}
		result = append(result, m)
	}
	return result
}

// TestStreamRendererFixture covers all 14 event types: one JSON line per
// event, type preserved, chan serialized, text preserved verbatim (including
// embedded newlines escaped by encoding/json).
func TestStreamRendererFixture(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(&buf)
	fixture := streamRendererFixture()
	for _, ev := range fixture {
		r.Render(ev)
	}

	lines := decodeJSONLines(t, buf.String())
	if len(lines) != len(fixture) {
		t.Fatalf("got %d JSON lines, want %d", len(lines), len(fixture))
	}
	for i, ev := range fixture {
		m := lines[i]
		if got := m["type"]; got != ev.Type {
			t.Errorf("line %d: type = %v, want %q", i, got, ev.Type)
		}
		if ev.Chan != "" {
			if got := m["chan"]; got != string(ev.Chan) {
				t.Errorf("line %d: chan = %v, want %q", i, got, ev.Chan)
			}
		}
		if ev.Text != "" {
			if got := m["text"]; got != ev.Text {
				t.Errorf("line %d: text = %v, want %q", i, got, ev.Text)
			}
		}
	}
	// Embedded newlines in text must stay inside the JSON line (escaped).
	if strings.Count(buf.String(), "\n") != len(fixture) {
		t.Errorf("output has %d newlines, want exactly %d (one per event)",
			strings.Count(buf.String(), "\n"), len(fixture))
	}
}

// TestStreamRendererFieldOmission verifies the field rules: level only when
// non-info, chan/text/meta only when non-zero/non-empty.
func TestStreamRendererFieldOmission(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(&buf)

	// Minimal event: only type — level(info)/chan/text/meta all omitted.
	r.Render(StreamEvent{Type: EventDone})
	m := decodeJSONLines(t, buf.String())[0]
	if len(m) != 1 {
		t.Errorf("minimal event should carry only type, got %v", m)
	}

	// Non-info level surfaces; info level does not.
	buf.Reset()
	r.Render(WarnEvent(ChannelSystem, "注意"))
	r.Render(ErrEvent(ChannelSystem, "失败"))
	r.Render(OKEvent(ChannelSystem, "完成"))
	r.Render(InfoEvent(ChannelSystem, "普通"))
	lines := decodeJSONLines(t, buf.String())
	wantLevels := []interface{}{"warning", "error", "success", nil}
	for i, want := range wantLevels {
		if got := lines[i]["level"]; got != want {
			t.Errorf("line %d: level = %v, want %v", i, got, want)
		}
	}
	// InfoEvent etc. always carry a chan.
	if got := lines[3]["chan"]; got != "system" {
		t.Errorf("line 3: chan = %v, want system", got)
	}
}

// TestStreamRendererMeta verifies Meta is serialized as a nested object and
// omitted when nil.
func TestStreamRendererMeta(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(&buf)

	r.Render(TokenTaskEvent(100, 50, 150))
	r.Render(StreamEvent{Type: EventDone, Chan: ChannelSystem})
	lines := decodeJSONLines(t, buf.String())

	meta, ok := lines[0]["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("token_task meta missing or not an object: %v", lines[0])
	}
	if got := meta[MetaKeyPrompt]; got != "100" {
		t.Errorf("meta.prompt = %v, want \"100\"", got)
	}
	if got := meta[MetaKeyTotal]; got != "150" {
		t.Errorf("meta.total = %v, want \"150\"", got)
	}
	if _, has := lines[1]["meta"]; has {
		t.Errorf("nil Meta must be omitted, got %v", lines[1]["meta"])
	}
}

// TestStreamRendererNoDecoration verifies the JSON-Lines output carries no
// ANSI escapes, no emoji prefixes and no timestamps.
func TestStreamRendererNoDecoration(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(&buf)
	for _, ev := range streamRendererFixture() {
		r.Render(ev)
	}
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("output contains ANSI escape: %q", out)
	}
	for _, emoji := range []string{"✅", "⚠️", "❌", "ℹ️", "🤖"} {
		if strings.Contains(out, emoji) {
			t.Errorf("output contains emoji prefix %q", emoji)
		}
	}
}

// TestStreamRendererEmptyTypeSkipped verifies events with an empty Type are
// not emitted at all.
func TestStreamRendererEmptyTypeSkipped(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(&buf)
	r.Render(StreamEvent{Text: "orphan"})
	if buf.Len() != 0 {
		t.Errorf("empty-type event should produce no output, got %q", buf.String())
	}
}

// TestStreamRendererImplementsEventRenderer is a compile-time check that both
// renderers satisfy the EventRenderer interface (FEATURE-307b).
func TestStreamRendererImplementsEventRenderer(t *testing.T) {
	var _ EventRenderer = (*StreamRenderer)(nil)
	var _ EventRenderer = (*LineRenderer)(nil)
}
