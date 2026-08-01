// Package main - golden tests for single-command stream event rendering.
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package main

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

// cmdBufferIO implements agent.UserIO capturing all output into a buffer.
// It is the single-command counterpart of repl's bufferIO test helper.
type cmdBufferIO struct {
	buf bytes.Buffer
}

func (b *cmdBufferIO) Print(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
}

func (b *cmdBufferIO) Printf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *cmdBufferIO) Println(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
	b.buf.WriteString("\n")
}

func (b *cmdBufferIO) ErrPrintf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *cmdBufferIO) ReadLine() (string, error) { return "", nil }
func (b *cmdBufferIO) ReadKey() (byte, error)    { return 0, nil }
func (b *cmdBufferIO) IsReading() bool           { return false }

func (b *cmdBufferIO) String() string { return b.buf.String() }

// renderSingleCmdFixture returns the fixed event sequence covering the 8
// event types handled by renderSingleCmdEvent in single-command mode.
// Content values match repl's renderTUIFixture where the event is shared,
// so that the P2 merge baseline (UC-0009) can compare both renderers.
func renderSingleCmdFixture() []struct {
	event   string
	content string
} {
	return []struct {
		event   string
		content string
	}{
		{agent.EventContentChunk, "你好"},
		{agent.EventThinkingChunk, "让我思考"},
		{agent.EventCommand, "ls -la"},
		{agent.EventOutput, "file1.txt\nfile2.txt"},
		{agent.EventToolCall, "get_weather(北京)"},
		{agent.EventTokenIter, "prompt=100 completion=50 total=150 max=200 ft=deepseek in_tps=10 out_tps=20"},
		{agent.EventError, "错误信息"},
		{agent.EventDone, ""},
	}
}

func TestRenderSingleCmdGolden(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	io := &cmdBufferIO{}

	for _, ev := range renderSingleCmdFixture() {
		renderSingleCmdEvent(io, ep, ev.event, ev.content)
	}

	goldenPath := filepath.Join("testdata", "render_single_cmd.golden")
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
