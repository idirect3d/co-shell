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
func renderTUIFixture() []struct {
	event   string
	content string
} {
	return []struct {
		event   string
		content string
	}{
		{agent.EventContentChunk, "你好"},
		{agent.EventThinkingChunk, "让我思考"},
		{agent.EventContent, "完整回答内容"},
		{agent.EventThinking, "完整思考内容"},
		{agent.EventCommand, "ls -la"},
		{agent.EventOutput, "file1.txt\nfile2.txt"},
		{agent.EventToolCall, "get_weather(北京)"},
		{agent.EventTokenIter, "prompt=100 completion=50 total=150 max=200 ft=deepseek in_tps=10 out_tps=20"},
		{agent.EventTokenTask, "prompt=100 completion=50 total=150"},
		{agent.EventInfo, "调试信息"},
		{agent.EventWarning, "警告信息"},
		{agent.EventError, "错误信息"},
		{agent.EventDone, ""},
	}
}

func TestRenderTUIGolden(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.EmojiEnabled = true
	r := &REPL{cfg: cfg, userIO: &bufferIO{}}

	// Render the full fixture sequence through streamCallback.
	io := r.userIO.(*bufferIO)
	for _, ev := range renderTUIFixture() {
		r.streamCallback(ev.event, ev.content)
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
