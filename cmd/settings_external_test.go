// Author: L.Shuang
// Created: 2026-08-06
// Last Modified: 2026-08-06
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// setupExternalTestAgent creates an agent whose workspace contains external
// config files (PRINCIPLES.md + CAPABILITIES.md + .rules/) plus a config with
// rules, so the :set output can reflect the external file status.
func setupExternalTestAgent(t *testing.T) (*SettingsHandler, string) {
	t.Helper()
	i18n.Init("zh")

	ws := t.TempDir()
	// PRINCIPLES.md with content
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("外部原则"), 0644); err != nil {
		t.Fatal(err)
	}
	// CAPABILITIES.md with content
	if err := os.WriteFile(filepath.Join(ws, "CAPABILITIES.md"), []byte("外部能力"), 0644); err != nil {
		t.Fatal(err)
	}
	// .rules/ dir with two md files
	rulesDir := filepath.Join(ws, ".rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "b.md"), []byte("B 规则"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "a.md"), []byte("A 规则"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Rules = []string{"配置文件规则 1", "配置文件规则 2"}

	// Build a real agent via agent.New so the taskplan manager (which requires
	// a store) is properly initialized.
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)

	h := &SettingsHandler{cfg: cfg, agent: ag}
	return h, ws
}

// TestShowSettingsHelp_ReflectsExternalFiles verifies that the :set output
// (showSettingsHelp) reflects PRINCIPLES.md / CAPABILITIES.md / .rules/ in real
// time via the agent's resolvers.
func TestShowSettingsHelp_ReflectsExternalFiles(t *testing.T) {
	h, _ := setupExternalTestAgent(t)
	out := h.showSettingsHelp()

	// .rules/ file names appear sorted (a.md, b.md)
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "b.md") {
		t.Errorf("expected .rules/ file names in :set output, got:\n%s", out)
	}
	// config rules count reflected
	if !strings.Contains(out, "2 条") {
		t.Errorf("expected config rules count in :set output, got:\n%s", out)
	}
	// PRINCIPLES.md external content reflected in principles value
	if !strings.Contains(out, "外部原则") {
		t.Errorf("expected PRINCIPLES.md content in principles line, got:\n%s", out)
	}
	// CAPABILITIES.md loaded status shown
	if !strings.Contains(out, "CAPABILITIES.md") {
		t.Errorf("expected CAPABILITIES.md status in :set output, got:\n%s", out)
	}
}

// TestShowSettingsHelp_WithoutExternalFiles verifies the fallback display when
// no external files exist: built-in defaults for capabilities, config-only rules.
func TestShowSettingsHelp_WithoutExternalFiles(t *testing.T) {
	i18n.Init("zh")
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Rules = []string{"仅配置文件规则"}
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)
	h := &SettingsHandler{cfg: cfg, agent: ag}

	out := h.showSettingsHelp()
	// Capabilities falls back to built-in default marker
	if !strings.Contains(out, "内置默认") {
		t.Errorf("expected built-in default capabilities marker, got:\n%s", out)
	}
	// Rules shows config-only count without .rules/
	if !strings.Contains(out, "1 条") {
		t.Errorf("expected config rules count 1 条, got:\n%s", out)
	}
	if strings.Contains(out, ".rules/") {
		t.Errorf("expected no .rules/ mention when dir absent, got:\n%s", out)
	}
}
