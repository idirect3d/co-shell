// Author: L.Shuang
// Created: 2026-10-02
//
// # MIT License - Copyright (c) 2026 L.Shuang
//
// FEATURE-579: table-driven tests for the eight token-optimization switches.
// The tests print evidence lines (SETTINGS/... and I18N/...) so a reviewer can
// verify the wiring without reading the source.
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// tokenOptSettingsTestName is the top-level test; the "-run TestTokenOptSettings"
// filter also matches its subtests.
func TestTokenOptSettings(t *testing.T) {
	h, cfg, _ := setupUISwitchTestHandler(t)

	// Derive the expected defaults straight from the config package so the test
	// fails if DefaultConfig ever drifts away from the registered defaults.
	defRaw, err := json.Marshal(config.DefaultConfig().LLM)
	if err != nil {
		t.Fatalf("marshal DefaultConfig: %v", err)
	}
	var defMap map[string]any
	if err := json.Unmarshal(defRaw, &defMap); err != nil {
		t.Fatalf("unmarshal DefaultConfig: %v", err)
	}

	cases := []struct {
		key      string // CLI / Web setting key (hyphen form)
		jsonName string // field name in config.json
		labelKey string // i18n label key
		kind     string // "bool" or "int"
		newValue string // value written through the setter
	}{
		{"env-history-prune", "env_history_prune", i18n.KeyCol3EnvHistoryPrune, "bool", "off"},
		{"env-history-full-keep", "env_history_full_keep", i18n.KeyCol3EnvHistoryFullKeep, "int", "3"},
		{"env-history-lite-keep", "env_history_lite_keep", i18n.KeyCol3EnvHistoryLiteKeep, "int", "2"},
		{"env-tool-batch-single", "env_tool_batch_single", i18n.KeyCol3EnvToolBatchSingle, "bool", "off"},
		{"env-board-roster-brief", "env_board_roster_brief", i18n.KeyCol3EnvBoardRosterBrief, "bool", "off"},
		{"env-dir-change-detect", "env_dir_change_detect", i18n.KeyCol3EnvDirChangeDetect, "bool", "off"},
		{"env-runtime-info-once", "env_runtime_info_once", i18n.KeyCol3EnvRuntimeInfoOnce, "bool", "off"},
		{"tool-desc-compact", "tool_desc_compact", i18n.KeyCol3ToolDescCompact, "bool", "off"},
	}

	// Index the Web/CLI registry (SettingsJSON is what both the Web panel and
	// the TUI :set list are driven from).
	registry := map[string]WebSettingItem{}
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			registry[it.Key] = it
		}
	}
	cliList := h.showSettingsHelp()

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			// ---- SETTINGS: present in the registry + CLI list, default matches ----
			item, ok := registry[tc.key]
			if !ok {
				t.Fatalf("SETTINGS|%s|missing from SettingsJSON registry", tc.key)
			}
			if !strings.Contains(cliList, tc.key) {
				t.Fatalf("SETTINGS|%s|missing from :set list", tc.key)
			}
			rawDef, ok := defMap[tc.jsonName]
			if !ok {
				t.Fatalf("SETTINGS|%s|missing %q in DefaultConfig JSON", tc.key, tc.jsonName)
			}
			wantDefault := defaultString(rawDef, tc.kind)
			if item.Default != wantDefault {
				t.Errorf("SETTINGS|%s|registry default %q != config default %q", tc.key, item.Default, wantDefault)
			}
			if item.Value != wantDefault {
				t.Errorf("SETTINGS|%s|initial value %q != config default %q", tc.key, item.Value, wantDefault)
			}
			fmt.Printf("SETTINGS|%s|ok|%s\n", tc.key, wantDefault)

			// ---- I18N: zh and en labels exist and differ ----
			i18n.Init("zh")
			zh := i18n.T(tc.labelKey)
			i18n.Init("en")
			en := i18n.T(tc.labelKey)
			if strings.TrimSpace(zh) == "" || strings.TrimSpace(en) == "" {
				t.Fatalf("I18N|%s|empty label (zh=%q en=%q)", tc.key, zh, en)
			}
			if zh == en {
				t.Fatalf("I18N|%s|zh and en labels are identical: %q", tc.key, zh)
			}
			fmt.Printf("I18N|%s|%s|%s\n", tc.key, zh, en)
			i18n.Init("zh")

			// ---- setter path: new value lands in cfg ----
			if _, err := h.Handle([]string{tc.key, tc.newValue}); err != nil {
				t.Fatalf("set %s=%s: %v", tc.key, tc.newValue, err)
			}
			got := readTokenOptValue(t, cfg, tc.key, tc.kind)
			if got != tc.newValue {
				t.Errorf("set %s=%s: cfg now %q", tc.key, tc.newValue, got)
			}
			fmt.Printf("SETTER|%s|%s\n", tc.key, got)
		})
	}

	// ---- negative keep values are rejected and leave the old value intact ----
	for _, key := range []string{"env-history-full-keep", "env-history-lite-keep"} {
		before := readTokenOptValue(t, cfg, key, "int")
		if _, err := h.Handle([]string{key, "-1"}); err == nil {
			t.Errorf("NEG|%s|expected rejection for -1", key)
			continue
		}
		after := readTokenOptValue(t, cfg, key, "int")
		if after != before {
			t.Errorf("NEG|%s|value changed after rejection: %q -> %q", key, before, after)
		}
		fmt.Printf("NEG|%s|rejected|value=%s\n", key, after)
	}
}

// defaultString renders a decoded DefaultConfig JSON value as the registry does.
func defaultString(v any, kind string) string {
	switch kind {
	case "bool":
		b, _ := v.(bool)
		return boolStr(b)
	case "int":
		f, _ := v.(float64)
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%v", v)
}

// readTokenOptValue reads the current cfg value in the same string form the
// registry uses ("on"/"off" for bools, decimal for ints).
func readTokenOptValue(t *testing.T, cfg *config.Config, key, kind string) string {
	t.Helper()
	llm := cfg.LLM
	if kind == "bool" {
		var b bool
		switch key {
		case "env-history-prune":
			b = llm.EnvHistoryPrune
		case "env-tool-batch-single":
			b = llm.EnvToolBatchSingle
		case "env-board-roster-brief":
			b = llm.EnvBoardRosterBrief
		case "env-dir-change-detect":
			b = llm.EnvDirChangeDetect
		case "env-runtime-info-once":
			b = llm.EnvRuntimeInfoOnce
		case "tool-desc-compact":
			b = llm.ToolDescCompact
		default:
			t.Fatalf("unknown bool setting %q", key)
		}
		return boolStr(b)
	}
	switch key {
	case "env-history-full-keep":
		return fmt.Sprintf("%d", llm.EnvHistoryFullKeep)
	case "env-history-lite-keep":
		return fmt.Sprintf("%d", llm.EnvHistoryLiteKeep)
	}
	t.Fatalf("unknown int setting %q", key)
	return ""
}
