// Author: L.Shuang
// Created: 2026-09-15
// Last Modified: 2026-09-15
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// setupUISwitchTestHandler builds a SettingsHandler backed by a throwaway
// workspace, so :set writes land in a temp config.json instead of the real one.
// It returns the handler, the config and the config.json path of that workspace.
func setupUISwitchTestHandler(t *testing.T) (*SettingsHandler, *config.Config, string) {
	t.Helper()
	i18n.Init("zh")

	wsPath := t.TempDir()
	wsObj, err := workspace.New(wsPath)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)

	cfg, _, err := config.LoadWithPath(wsObj)
	if err != nil {
		t.Fatalf("cannot load config: %v", err)
	}

	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(wsPath)
	ag.SetConfig(cfg)

	return &SettingsHandler{cfg: cfg, agent: ag}, cfg, wsObj.ConfigPath()
}

// TestSettingsJSONIncludesUISwitches verifies the Web settings panel (Agent
// group) exposes the FEATURE-524 UI rendering switches with their defaults.
func TestSettingsJSONIncludesUISwitches(t *testing.T) {
	h, _, _ := setupUISwitchTestHandler(t)

	found := map[string]WebSettingItem{}
	groupTitle := map[string]string{}
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			if it.Key == "ui-enabled" || it.Key == "ui-context-prune" {
				found[it.Key] = it
				groupTitle[it.Key] = g.Title
			}
		}
	}

	for _, key := range []string{"ui-enabled", "ui-context-prune"} {
		it, ok := found[key]
		if !ok {
			t.Fatalf("SettingsJSON is missing the %q item", key)
		}
		if it.Type != "bool" {
			t.Errorf("%s: type = %q, want bool", key, it.Type)
		}
		// DefaultConfig has both switches on.
		if it.Value != "on" {
			t.Errorf("%s: value = %q, want on", key, it.Value)
		}
		if it.Default != "on" {
			t.Errorf("%s: default = %q, want on", key, it.Default)
		}
		if strings.TrimSpace(it.Desc) == "" {
			t.Errorf("%s: empty description", key)
		}
		if !strings.Contains(it.Desc, "on|off") {
			t.Errorf("%s: description %q should hint on|off", key, it.Desc)
		}
		if strings.TrimSpace(groupTitle[key]) == "" {
			t.Errorf("%s: empty group title", key)
		}
	}
}

// TestSetUISwitchesPersist verifies `:set ui-enabled|ui-context-prune on|off`
// (the same path the Web panel uses) updates the config and writes it to disk.
func TestSetUISwitchesPersist(t *testing.T) {
	h, cfg, cfgPath := setupUISwitchTestHandler(t)

	msg, err := h.Handle([]string{"ui-enabled", "off"})
	if err != nil {
		t.Fatalf("set ui-enabled off: %v", err)
	}
	if cfg.UIEnabled {
		t.Error("cfg.UIEnabled should be false after setting it off")
	}
	if !strings.Contains(msg, i18n.T(i18n.KeyOff)) {
		t.Errorf("message %q should report %q", msg, i18n.T(i18n.KeyOff))
	}

	msg, err = h.Handle([]string{"ui-context-prune", "off"})
	if err != nil {
		t.Fatalf("set ui-context-prune off: %v", err)
	}
	if cfg.UIContextPrune {
		t.Error("cfg.UIContextPrune should be false after setting it off")
	}
	if !strings.Contains(msg, i18n.T(i18n.KeyOff)) {
		t.Errorf("message %q should report %q", msg, i18n.T(i18n.KeyOff))
	}

	onDisk := map[string]any{}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("cannot read saved config %s: %v", cfgPath, err)
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("cannot parse saved config: %v", err)
	}
	if v, ok := onDisk["ui_enabled"]; !ok || v != false {
		t.Errorf("saved config ui_enabled = %v (present=%v), want false", v, ok)
	}
	if v, ok := onDisk["ui_context_prune"]; !ok || v != false {
		t.Errorf("saved config ui_context_prune = %v (present=%v), want false", v, ok)
	}

	// Turning the master switch back on must be persisted too.
	if _, err := h.Handle([]string{"ui-enabled", "on"}); err != nil {
		t.Fatalf("set ui-enabled on: %v", err)
	}
	if !cfg.UIEnabled {
		t.Error("cfg.UIEnabled should be true after setting it on")
	}
}

// TestSetUISwitchesQueryAndInvalidValue verifies the query form (no value)
// reports the current state and that an invalid value is rejected.
func TestSetUISwitchesQueryAndInvalidValue(t *testing.T) {
	h, _, _ := setupUISwitchTestHandler(t)

	msg, err := h.Handle([]string{"ui-enabled"})
	if err != nil {
		t.Fatalf("query ui-enabled: %v", err)
	}
	if !strings.Contains(msg, i18n.T(i18n.KeyOn)) {
		t.Errorf("query message %q should report %q", msg, i18n.T(i18n.KeyOn))
	}

	if _, err := h.Handle([]string{"ui-enabled", "maybe"}); err == nil {
		t.Error("invalid value should be rejected")
	}
	if _, err := h.Handle([]string{"ui-context-prune", "maybe"}); err == nil {
		t.Error("invalid value should be rejected")
	}
}
