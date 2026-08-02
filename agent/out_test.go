// Package agent - Out/RenderCommand/NormalizeInputMode table tests (FEATURE-302).
//
// Author: L.Shuang
// Created: 2026-08-01
// Last Modified: 2026-08-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// outTestIO captures UserIO output for TerminalOut snapshot assertions.
type outTestIO struct {
	buf bytes.Buffer
}

func (b *outTestIO) Print(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
}

func (b *outTestIO) Printf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *outTestIO) Println(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
	b.buf.WriteString("\n")
}

func (b *outTestIO) ErrPrintf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *outTestIO) ReadLine() (string, error) { return "", nil }
func (b *outTestIO) ReadKey() (byte, error)    { return 0, nil }
func (b *outTestIO) IsReading() bool           { return false }

func (b *outTestIO) String() string { return b.buf.String() }

// TestChannelIDsNonEmptyUnique verifies all ChannelID constants are non-empty
// and mutually distinct (UC-0003).
func TestChannelIDsNonEmptyUnique(t *testing.T) {
	values := []ChannelID{
		ChannelLLM, ChannelTool, ChannelCommand, ChannelDebug, ChannelWizard,
		ChannelSystem, ChannelMCP, ChannelMemory, ChannelTaskPlan, ChannelDB,
		ChannelBridge, ChannelSubAgent,
	}
	seen := make(map[ChannelID]bool)
	for _, v := range values {
		if v == "" {
			t.Error("ChannelID constant must not be empty")
		}
		if seen[v] {
			t.Errorf("duplicate ChannelID %q", v)
		}
		seen[v] = true
	}
}

// TestLevelsOrdered verifies Level constants are ascending and start at LevelInfo.
func TestLevelsOrdered(t *testing.T) {
	if LevelInfo != 0 {
		t.Errorf("LevelInfo = %d, want 0", LevelInfo)
	}
	if LevelSuccess <= LevelInfo || LevelWarning <= LevelSuccess ||
		LevelError <= LevelWarning || LevelDebug <= LevelError {
		t.Error("Level constants must be strictly ascending: Info < Success < Warning < Error < Debug")
	}
}

// TestRenderKindsNonEmptyUnique verifies all RenderKind constants are non-empty
// and mutually distinct (UC-0004).
func TestRenderKindsNonEmptyUnique(t *testing.T) {
	values := []RenderKind{
		RenderText, RenderTitle, RenderBox, RenderMenu, RenderStep,
		RenderSep, RenderDialog, RenderProgress,
	}
	seen := make(map[RenderKind]bool)
	for _, v := range values {
		if v == "" {
			t.Error("RenderKind constant must not be empty")
		}
		if seen[v] {
			t.Errorf("duplicate RenderKind %q", v)
		}
		seen[v] = true
	}
}

// TestTerminalOutSnapshot verifies TerminalOut convenience methods render the
// exact expected emoji-prefixed output (UC-0008, table-driven loop).
func TestTerminalOutSnapshot(t *testing.T) {
	io := &outTestIO{}
	ep := config.GetEmojiPrefixes(true)
	out := NewTerminalOut(io, ep)

	tests := []struct {
		name           string
		render         func()
		expectContains []string
	}{
		{
			name:           "Info_no_prefix",
			render:         func() { out.Info(ChannelLLM, "hello") },
			expectContains: []string{"hello"},
		},
		{
			name:           "Success_prefix",
			render:         func() { out.Success(ChannelTool, "done") },
			expectContains: []string{ep.Success, "done"},
		},
		{
			name:           "Warning_prefix",
			render:         func() { out.Warning(ChannelCommand, "careful") },
			expectContains: []string{ep.Warning, "careful"},
		},
		{
			name:           "Error_prefix",
			render:         func() { out.Error(ChannelSystem, "boom") },
			expectContains: []string{ep.Error, "boom"},
		},
		{
			name:           "Debug_prefix_uses_info_emoji",
			render:         func() { out.Debug(ChannelDebug, "trace") },
			expectContains: []string{ep.Info, "trace"},
		},
	}

	for i, tt := range tests {
		io.buf.Reset()
		tt.render()
		got := io.String()
		for _, want := range tt.expectContains {
			if want != "" && !strings.Contains(got, want) {
				t.Errorf("case %d (%s): output %q does not contain %q", i, tt.name, got, want)
			}
		}
	}
}

// TestNormalizeInputMode verifies legacy "enhanced" maps to "tui" (UC-0009).
func TestNormalizeInputMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"enhanced", "tui"},
		{"tui", "tui"},
		{"stdio", "stdio"},
		{"", ""},
		{"unknown", "unknown"}, // invalid values pass through for caller reporting
	}
	for _, tt := range tests {
		if got := config.NormalizeInputMode(tt.input); got != tt.want {
			t.Errorf("NormalizeInputMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
