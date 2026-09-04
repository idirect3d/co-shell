// Package cmd - web-input-dir setting tests (FEATURE-469): default value,
// invalid-path rejection and presence in the Web UI settings JSON.
//
// Author: L.Shuang
// Created: 2026-09-04
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

func TestWebInputDirValueDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	if got := webInputDirValue(cfg); got != "input" {
		t.Errorf("webInputDirValue(default) = %q, want input", got)
	}
	cfg.WebInputDir = "uploads"
	if got := webInputDirValue(cfg); got != "uploads" {
		t.Errorf("webInputDirValue(uploads) = %q, want uploads", got)
	}
}

func TestWebInputDirRejectsBadPaths(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &SettingsHandler{cfg: cfg}
	for _, bad := range []string{"/abs", "../escape", "a/../../b", ""} {
		if _, err := h.handleWebSetting("web-input-dir", []string{"web-input-dir", bad}); err == nil {
			t.Errorf("web-input-dir %q: expected error", bad)
		}
	}
}

func TestSettingsJSONIncludesWebInputDir(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &SettingsHandler{cfg: cfg}
	groups := h.SettingsJSON()
	for _, g := range groups {
		for _, it := range g.Items {
			if it.Key == "web-input-dir" {
				if it.Value != "input" {
					t.Errorf("settings web-input-dir value = %q, want input", it.Value)
				}
				if !strings.Contains(it.Desc, "input") {
					t.Errorf("settings web-input-dir desc should mention default input, got %q", it.Desc)
				}
				return
			}
		}
	}
	t.Error("SettingsJSON missing web-input-dir item")
}

// TestSettingsJSONFillsDefaults verifies every generic setting item carries a
// non-empty Default value (FEATURE-470) so the Web UI can show it in tooltips
// and mark values that differ from the default.
func TestSettingsJSONFillsDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &SettingsHandler{cfg: cfg}
	groups := h.SettingsJSON()
	if len(groups) == 0 {
		t.Fatal("SettingsJSON returned no groups")
	}
	checked := 0
	for _, g := range groups {
		if g.Kind == "mcp" {
			continue // MCP manager group has no generic items
		}
		for _, it := range g.Items {
			if it.Default == "" {
				t.Errorf("setting %q (group %q) has empty Default", it.Key, g.Title)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no generic setting items checked")
	}
}

// TestSettingsJSONDefaultMatchesValue verifies that when the config equals the
// default, each item's Value equals its Default (FEATURE-470).
func TestSettingsJSONDefaultMatchesValue(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &SettingsHandler{cfg: cfg}
	groups := h.SettingsJSON()
	for _, g := range groups {
		if g.Kind == "mcp" {
			continue
		}
		for _, it := range g.Items {
			if it.Default != "" && it.Value != it.Default {
				// web-whitelist: the default is a readable description of the
				// empty value (loopback only), so an empty current value matches.
				if it.Key == "web-whitelist" && it.Value == "" {
					continue
				}
				t.Errorf("setting %q value=%q default=%q should match on default config", it.Key, it.Value, it.Default)
			}
		}
	}
}
