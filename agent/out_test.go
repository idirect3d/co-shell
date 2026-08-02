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

// TestOutputCategoryShown verifies OutputCategoryShown semantics:
// nil/empty map = show all; explicit false hides; unknown category = shown (UC-0004).
func TestOutputCategoryShown(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]bool
		channel  string
		expected bool
	}{
		{"nil_map_shows_all", nil, "bridge", true},
		{"empty_map_shows_all", map[string]bool{}, "bridge", true},
		{"explicit_on", map[string]bool{"bridge": true}, "bridge", true},
		{"explicit_off_hidden", map[string]bool{"bridge": false}, "bridge", false},
		{"unknown_category_shown", map[string]bool{"bridge": false}, "llm", true},
		{"empty_channel_shown", map[string]bool{"bridge": false}, "", true},
	}
	for _, tt := range tests {
		cfg := config.DefaultConfig()
		cfg.LLM.OutputCategories = tt.m
		if got := cfg.OutputCategoryShown(tt.channel); got != tt.expected {
			t.Errorf("%s: OutputCategoryShown(%q) = %v, want %v", tt.name, tt.channel, got, tt.expected)
		}
	}
}

// TestTerminalOutCategoryFilter verifies TerminalOut.SetFilter hides channels
// whose category is disabled in OutputCategories while leaving others intact (UC-0005).
func TestTerminalOutCategoryFilter(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	io := &outTestIO{}
	out := NewTerminalOut(io, ep)
	cfg := config.DefaultConfig()
	cfg.LLM.OutputCategories = map[string]bool{"bridge": false}
	out.SetFilter(cfg)

	tests := []struct {
		name      string
		ch        ChannelID
		wantShown bool
	}{
		{"bridge_hidden", ChannelBridge, false},
		{"subagent_shown", ChannelSubAgent, true},
		{"wizard_shown", ChannelWizard, true},
		{"llm_shown", ChannelLLM, true},
		{"db_shown", ChannelDB, true},
	}
	for _, tt := range tests {
		io.buf.Reset()
		out.Info(tt.ch, "hello")
		got := io.String()
		if tt.wantShown && !strings.Contains(got, "hello") {
			t.Errorf("%s: channel should be visible, output=%q", tt.name, got)
		}
		if !tt.wantShown && strings.Contains(got, "hello") {
			t.Errorf("%s: channel should be hidden, output=%q", tt.name, got)
		}
	}
}

// TestTerminalOutNoFilterBackwardCompatible verifies that a TerminalOut without
// SetFilter (or with nil filter) still renders all channels (UC-0005 backward compat).
func TestTerminalOutNoFilterBackwardCompatible(t *testing.T) {
	ep := config.GetEmojiPrefixes(true)
	io := &outTestIO{}

	outNoFilter := NewTerminalOut(io, ep)
	io.buf.Reset()
	outNoFilter.Info(ChannelBridge, "bridge-hidden-check")
	if !strings.Contains(io.String(), "bridge-hidden-check") {
		t.Errorf("TerminalOut without SetFilter must still render all channels")
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
