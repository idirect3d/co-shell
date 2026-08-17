// Package agent - LineRenderer layout table tests (FEATURE-307a).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// renderOne renders a single event through a LineRenderer backed by a
// capturing UserIO and returns the rendered output.
func renderOne(mode StreamMode, ev StreamEvent) string {
	io := &outTestIO{}
	r := NewLineRenderer(io, config.GetEmojiPrefixes(true), mode)
	r.Render(ev)
	return io.String()
}

// TestLineRendererInfoLevels locks the Level-driven layout of EventInfo:
// emoji prefixes and decorative newlines are applied by the renderer, never
// carried in the event payload.
func TestLineRendererInfoLevels(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	tests := []struct {
		name  string
		level Level
		want  string
	}{
		{"info verbatim", LevelInfo, "纯文本"},
		{"success", LevelSuccess, "\n" + ep.Success + "纯文本" + "\n"},
		{"warning", LevelWarning, "\n" + ep.Warning + "纯文本" + "\n"},
		{"error", LevelError, "\n" + ep.Error + "纯文本" + "\n"},
		{"debug loop prefix", LevelDebug, ep.Loop + "纯文本"},
	}
	for _, tt := range tests {
		got := renderOne(StreamModeREPL, NewStreamEvent(EventInfo, ChannelSystem, tt.level, "纯文本"))
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestLineRendererWarningErrorTypes locks the historical layout of the
// EventWarning/EventError types (prefix + text + newline, no leading blank
// line, Level ignored).
func TestLineRendererWarningErrorTypes(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	if got := renderOne(StreamModeREPL, NewStreamEvent(EventWarning, ChannelSystem, LevelInfo, "警告")); got != ep.Warning+"警告\n" {
		t.Errorf("EventWarning: got %q", got)
	}
	if got := renderOne(StreamModeREPL, NewStreamEvent(EventError, ChannelSystem, LevelInfo, "错误")); got != ep.Error+"错误\n" {
		t.Errorf("EventError: got %q", got)
	}
}

// TestLineRendererModeDifferences locks the two StreamMode presentation
// differences: REPL adds a leading newline before command/tool_call events;
// single-command does not.
func TestLineRendererModeDifferences(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	if got := renderOne(StreamModeREPL, NewStreamEvent(EventCommand, ChannelCommand, LevelInfo, "ls")); got != "\n"+ep.CommandInput+"ls\n" {
		t.Errorf("REPL command: got %q", got)
	}
	if got := renderOne(StreamModeSingleCmd, NewStreamEvent(EventCommand, ChannelCommand, LevelInfo, "ls")); got != ep.CommandInput+"ls\n" {
		t.Errorf("SingleCmd command: got %q", got)
	}
	if got := renderOne(StreamModeREPL, NewStreamEvent(EventToolCall, ChannelTool, LevelInfo, "read_file")); got != "\n"+ep.ToolCallInput+"read_file\n" {
		t.Errorf("REPL tool_call: got %q", got)
	}
	if got := renderOne(StreamModeSingleCmd, NewStreamEvent(EventToolCall, ChannelTool, LevelInfo, "read_file")); got != ep.ToolCallInput+"read_file\n" {
		t.Errorf("SingleCmd tool_call: got %q", got)
	}
}

// TestLineRendererTokenEvents locks the token usage rendering driven by the
// structured Meta map (no string re-parsing).
func TestLineRendererTokenEvents(t *testing.T) {
	sep := strings.Repeat("─", 80)
	ep := config.GetEmojiPrefixes(true)

	iter := TokenIterEvent(100, 50, 150, 200, "deepseek", "10", "20")
	got := renderOne(StreamModeREPL, iter)
	want := "\n" + sep + "\n" +
		"Token用量: 首字=deepseek | 输入=100(10 t/s) | 输出=50(20 t/s) | 总计=150 (75.0%)" +
		"\n" + sep + "\n"
	if got != want {
		t.Errorf("REPL token_iter:\ngot  %q\nwant %q", got, want)
	}

	got = renderOne(StreamModeSingleCmd, iter)
	want = "\n" + ep.Info + sep + "\n" +
		ep.Info + " Token用量: 首字=deepseek | 输入=100(10 t/s) | 输出=50(20 t/s) | 总计=150 (75.0%)\n" +
		ep.Info + sep + "\n"
	if got != want {
		t.Errorf("SingleCmd token_iter:\ngot  %q\nwant %q", got, want)
	}

	// Unknown model max length (max=0): REPL shows the unknown-length hint.
	iterNoMax := TokenIterEvent(100, 50, 150, 0, "deepseek", "10", "20")
	got = renderOne(StreamModeREPL, iterNoMax)
	if !strings.Contains(got, "模型最大长度未知") || !strings.Contains(got, "Token用量") {
		t.Errorf("REPL token_iter max=0: got %q", got)
	}

	// Zero total renders nothing in REPL mode (FIX-295).
	if got := renderOne(StreamModeREPL, TokenIterEvent(0, 0, 0, 200, "x", "0", "0")); got != "" {
		t.Errorf("REPL token_iter total=0: got %q, want empty", got)
	}

	task := TokenTaskEvent(100, 50, 150)
	got = renderOne(StreamModeREPL, task)
	want = "\n" + sep + "\n" + "本次任务 Token 总计: 输入=100, 输出=50, 总计=150\n" + sep + "\n"
	if got != want {
		t.Errorf("token_task:\ngot  %q\nwant %q", got, want)
	}

	// Zero task total renders nothing.
	if got := renderOne(StreamModeREPL, TokenTaskEvent(0, 0, 0)); got != "" {
		t.Errorf("token_task total=0: got %q, want empty", got)
	}

	// Incomplete Meta renders nothing (mirrors the former Sscanf failure).
	bad := StreamEvent{Type: EventTokenIter, Meta: map[string]string{MetaKeyTotal: "150"}}
	if got := renderOne(StreamModeREPL, bad); got != "" {
		t.Errorf("token_iter incomplete meta: got %q, want empty", got)
	}
}

// TestLineRendererDone locks the done marker layout.
func TestLineRendererDone(t *testing.T) {
	if got := renderOne(StreamModeREPL, NewStreamEvent(EventDone, ChannelSystem, LevelInfo, "")); got != "\n" {
		t.Errorf("EventDone: got %q, want newline", got)
	}
}
