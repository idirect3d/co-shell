// Author: L.Shuang
// Created: 2026-10-02
//
// MIT License - Copyright (c) 2026 L.Shuang
//
// FEATURE-579 follow-up: the ten token-optimization / UI rendering switches
// (FEATURE-579 + FEATURE-524) were moved from the "Agent Settings" group to the
// "Memory & Context" group in both the Web settings panel and the TUI :set list.
// These tests print evidence lines (SETTINGS/...) so a reviewer can verify the
// placement without reading the source.
package cmd

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// movedSettingsKeys are the switches relocated to the Memory & Context group:
// the FEATURE-524 UI context pruning switch followed by the FEATURE-579 token
// optimization switches (same order as the group definition). ui-enabled was put
// back to the Agent Settings group.
var movedSettingsKeys = []string{
	"ui-context-prune",
	"env-history-prune",
	"env-history-full-keep",
	"env-history-lite-keep",
	"env-tool-batch-single",
	"env-board-roster-brief",
	"env-dir-change-detect",
	"env-runtime-info-once",
	"tool-desc-compact",
}

// TestMovedSettingsGroupPlacement verifies the ten switches belong to the
// "Memory & Context" group in the Web settings panel (SettingsJSON) and appear
// inside that group's section of the TUI :set output (showSettingsHelp).
func TestMovedSettingsGroupPlacement(t *testing.T) {
	h, _, _ := setupUISwitchTestHandler(t)

	memoryTitle := i18n.T(i18n.KeySettingsGroupMemory)
	agentTitle := i18n.T(i18n.KeySettingsGroupModel)

	// Web settings panel: collect the owning group title of every item.
	groupOf := map[string]string{}
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			groupOf[it.Key] = g.Title
		}
	}

	// TUI :set output: locate the Memory & Context and Developer section headers
	// (the memory section ends where the developer section begins).
	help := h.showSettingsHelp()
	memStart := strings.Index(help, "\n  "+memoryTitle+"\n")
	devStart := strings.Index(help, "\n  "+i18n.T(i18n.KeySettingsGroupSearchDebug)+"\n")
	if memStart < 0 || devStart < 0 || devStart <= memStart {
		t.Fatalf("SETTINGS|group|sections not found in :set output (memory=%d developer=%d)", memStart, devStart)
	}

	for _, key := range movedSettingsKeys {
		t.Run(key, func(t *testing.T) {
			got := groupOf[key]
			if got == "" {
				t.Fatalf("SETTINGS|%s|missing from the Web settings panel", key)
			}
			if got != memoryTitle {
				t.Errorf("SETTINGS|%s|Web group = %q, want %q (agent group = %q)", key, got, memoryTitle, agentTitle)
			}
			idx := strings.Index(help, key)
			if idx < 0 {
				t.Fatalf("SETTINGS|%s|missing from the :set output", key)
			}
			if idx < memStart || idx > devStart {
				t.Errorf("SETTINGS|%s|not inside the %q section of the :set output", key, memoryTitle)
			}
			t.Logf("SETTINGS|%s|ok|group=%s", key, memoryTitle)
		})
	}
}

// TestUiEnabledBackInAgentGroup verifies ui-enabled belongs to the "Agent
// Settings" group in both the Web settings panel and the :set output, while
// ui-context-prune stays in the "Memory & Context" group.
func TestUiEnabledBackInAgentGroup(t *testing.T) {
	h, _, _ := setupUISwitchTestHandler(t)

	agentTitle := i18n.T(i18n.KeySettingsGroupModel)
	memoryTitle := i18n.T(i18n.KeySettingsGroupMemory)

	groupOf := map[string]string{}
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			groupOf[it.Key] = g.Title
		}
	}
	if got := groupOf["ui-enabled"]; got != agentTitle {
		t.Errorf("SETTINGS|ui-enabled|Web group = %q, want %q", got, agentTitle)
	}
	if got := groupOf["ui-context-prune"]; got != memoryTitle {
		t.Errorf("SETTINGS|ui-context-prune|Web group = %q, want %q", got, memoryTitle)
	}

	help := h.showSettingsHelp()
	agentStart := strings.Index(help, "\n  "+agentTitle+"\n")
	memStart := strings.Index(help, "\n  "+memoryTitle+"\n")
	if agentStart < 0 || memStart <= agentStart {
		t.Fatalf("SETTINGS|group|sections not found in :set output (agent=%d memory=%d)", agentStart, memStart)
	}
	idx := strings.Index(help, "ui-enabled")
	if idx < 0 {
		t.Fatalf("SETTINGS|ui-enabled|missing from the :set output")
	}
	if idx < agentStart || idx > memStart {
		t.Errorf("SETTINGS|ui-enabled|not inside the %q section of the :set output", agentTitle)
	}
	pruneIdx := strings.Index(help, "ui-context-prune")
	if pruneIdx < memStart {
		t.Errorf("SETTINGS|ui-context-prune|not inside the %q section of the :set output", memoryTitle)
	}
	t.Logf("SETTINGS|ui-enabled|ok|group=%s", agentTitle)
	t.Logf("SETTINGS|ui-context-prune|ok|group=%s", memoryTitle)
}
