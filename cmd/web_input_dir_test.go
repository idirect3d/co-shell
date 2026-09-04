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
