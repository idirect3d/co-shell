// Package repl - golden tests for stream event rendering.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// bufferIO implements agent.UserIO capturing all output into a buffer.
// It is used to record render output deterministically in tests.
type bufferIO struct {
	buf bytes.Buffer
}

func (b *bufferIO) Print(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
}

func (b *bufferIO) Printf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *bufferIO) Println(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
	b.buf.WriteString("\n")
}

func (b *bufferIO) ErrPrintf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *bufferIO) ReadLine() (string, error) { return "", nil }
func (b *bufferIO) ReadKey() (byte, error)    { return 0, nil }
func (b *bufferIO) IsReading() bool           { return false }

func (b *bufferIO) String() string { return b.buf.String() }

// renderTUIFixture returns a fixed event sequence covering all 13 event
// types handled by REPL.streamCallback. This is the same sequence used
// for the single-command golden test (UC-0009 consistency check).
// Events carry pure semantic payloads (FEATURE-307a): no emoji prefixes or
// decorative newlines; Level zero value is LevelInfo.
func renderTUIFixture() []agent.StreamEvent {
	return []agent.StreamEvent{
		{Type: agent.EventContentChunk, Chan: agent.ChannelLLM, Text: "你好"},
		{Type: agent.EventThinkingChunk, Chan: agent.ChannelLLM, Text: "让我思考"},
		{Type: agent.EventContent, Chan: agent.ChannelLLM, Text: "完整回答内容"},
		{Type: agent.EventThinking, Chan: agent.ChannelLLM, Text: "完整思考内容"},
		{Type: agent.EventCommand, Chan: agent.ChannelCommand, Text: "ls -la"},
		{Type: agent.EventOutput, Chan: agent.ChannelCommand, Text: "file1.txt\nfile2.txt"},
		{Type: agent.EventToolCall, Chan: agent.ChannelTool, Text: "get_weather(北京)"},
		agent.TokenIterEvent(100, 50, 150, 200, "deepseek", "10", "20"),
		agent.TokenTaskEvent(100, 50, 150),
		{Type: agent.EventInfo, Chan: agent.ChannelSystem, Text: "调试信息"},
		{Type: agent.EventWarning, Chan: agent.ChannelSystem, Text: "警告信息"},
		{Type: agent.EventError, Chan: agent.ChannelSystem, Text: "错误信息"},
		{Type: agent.EventDone, Chan: agent.ChannelSystem},
	}
}

func TestRenderTUIGolden(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.EmojiEnabled = true
	ep := config.GetEmojiPrefixes(true)
	io := &bufferIO{}
	// FEATURE-307b: the per-run renderer is installed by session.Acquire in
	// production; the test installs the same LineRenderer directly.
	r := &REPL{cfg: cfg, renderer: agent.NewLineRenderer(io, ep, agent.StreamModeREPL)}

	// Render the full fixture sequence through streamCallback.
	for _, ev := range renderTUIFixture() {
		r.streamCallback(ev)
	}

	goldenPath := filepath.Join("testdata", "render_tui.golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(io.String()), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update first): %v", err)
	}
	if got := io.String(); got != string(want) {
		t.Errorf("render mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
