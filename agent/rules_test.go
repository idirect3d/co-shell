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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// --- loadRulesDir pure function tests (UC-0004 ~ 0007) ---

func TestLoadRulesDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// UC-0004: multi-file sorting with "# {filename}" header format
	files := map[string]string{
		"b.md": "rule b content",
		"a.md": "rule a content",
		"c.md": "rule c content",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got := loadRulesDir(dir)
	want := "# a.md\n\nrule a content\n\n# b.md\n\nrule b content\n\n# c.md\n\nrule c content"
	if got != want {
		t.Errorf("loadRulesDir sorting/format mismatch:\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "# a.md\n\nrule a content") {
		t.Errorf("expected a.md with # header, got: %q", got)
	}
}

func TestLoadRulesDir_NonMDIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// UC-0005: non-.md files ignored; .MD (uppercase) accepted via case-insensitive match
	if err := os.WriteFile(filepath.Join(dir, "real.md"), []byte("md rule"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("txt rule"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "upper.MD"), []byte("upper rule"), 0644); err != nil {
		t.Fatal(err)
	}
	got := loadRulesDir(dir)
	if strings.Contains(got, "txt rule") {
		t.Errorf("non-.md file should be ignored, got: %q", got)
	}
	if strings.Contains(got, "notes.txt") {
		t.Errorf("notes.txt should not appear, got: %q", got)
	}
	if !strings.Contains(got, "upper.MD") {
		t.Errorf("uppercase .MD should be accepted, got: %q", got)
	}
	if !strings.Contains(got, "real.md") {
		t.Errorf("real.md should be loaded, got: %q", got)
	}
}

func TestLoadRulesDir_EmptyOrMissing(t *testing.T) {
	// UC-0006: table-driven — nonexistent dir / empty dir / no .md files
	nonexistent := filepath.Join(t.TempDir(), "no-such-dir")
	emptyDir := t.TempDir()
	noMDDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(noMDDir, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		dir  string
	}{
		{"nonexistent dir", nonexistent},
		{"empty dir", emptyDir},
		{"only non-md files", noMDDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := loadRulesDir(tt.dir); got != "" {
				t.Errorf("expected empty string for %s, got %q", tt.name, got)
			}
		})
	}
	// Empty dir string
	if got := loadRulesDir(""); got != "" {
		t.Errorf("expected empty for empty dir arg, got %q", got)
	}
}

func TestLoadRulesDir_EmptyFileContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// UC-0007: empty/whitespace-only file produces no "# {filename}" header
	if err := os.WriteFile(filepath.Join(dir, "blank.md"), []byte("   \n  "), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.md"), []byte("good rule"), 0644); err != nil {
		t.Fatal(err)
	}
	got := loadRulesDir(dir)
	if strings.Contains(got, "blank.md") {
		t.Errorf("blank file should not produce header, got: %q", got)
	}
	if !strings.Contains(got, "# good.md") {
		t.Errorf("good.md should be loaded, got: %q", got)
	}
}

// --- resolveAgentPrinciples priority chain tests (UC-0008 ~ 0013) ---

func newTestAgentWithCfg(cfg *config.Config) *Agent {
	a := &Agent{cfg: cfg}
	return a
}

func TestResolveAgentPrinciples(t *testing.T) {
	i18n.Init("en")

	t.Run("PRINCIPLES.md in workspace path takes priority", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("file principles"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = "config principles"
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveAgentPrinciples(); got != "file principles" {
			t.Errorf("expected file principles, got %q", got)
		}
	})

	t.Run("cwd fallback when workspace has no PRINCIPLES.md", func(t *testing.T) {
		ws := t.TempDir() // empty workspace
		cwd := t.TempDir()
		if err := os.WriteFile(filepath.Join(cwd, "PRINCIPLES.md"), []byte("cwd principles"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = "config principles"
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		oldWD, _ := os.Getwd()
		defer os.Chdir(oldWD)
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
		if got := a.resolveAgentPrinciples(); got != "cwd principles" {
			t.Errorf("expected cwd principles, got %q", got)
		}
	})

	t.Run("config value when no PRINCIPLES.md", func(t *testing.T) {
		ws := t.TempDir()
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = "config principles"
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveAgentPrinciples(); got != "config principles" {
			t.Errorf("expected config principles, got %q", got)
		}
	})

	t.Run("i18n default when no file and empty config", func(t *testing.T) {
		ws := t.TempDir()
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = ""
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		got := a.resolveAgentPrinciples()
		want := i18n.T(i18n.KeyAgentDefaultPrinciples)
		if got != want {
			t.Errorf("expected i18n default %q, got %q", want, got)
		}
	})
}

func TestResolveAgentPrinciples_BlankFileFallsBackToConfig(t *testing.T) {
	// UC-0013: PRINCIPLES.md exists but content is blank → fall back to cfg
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("  \n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.LLM.AgentPrinciples = "config principles"
	a := newTestAgentWithCfg(cfg)
	a.workspacePath = ws
	if got := a.resolveAgentPrinciples(); got != "config principles" {
		t.Errorf("expected fallback to config, got %q", got)
	}
}

func TestResolveAgentPrinciplesPackageFunc(t *testing.T) {
	i18n.Init("en")
	// UC-0025: package-level ResolveAgentPrinciples used by --unload-principles
	t.Run("workspace PRINCIPLES.md wins", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("pkg file principles"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = "pkg config principles"
		if got := ResolveAgentPrinciples(cfg, ws); got != "pkg file principles" {
			t.Errorf("expected pkg file principles, got %q", got)
		}
	})
	t.Run("cfg fallback when no file", func(t *testing.T) {
		ws := t.TempDir()
		cfg := config.DefaultConfig()
		cfg.LLM.AgentPrinciples = "pkg config principles"
		if got := ResolveAgentPrinciples(cfg, ws); got != "pkg config principles" {
			t.Errorf("expected pkg config principles, got %q", got)
		}
	})
	t.Run("nil cfg returns empty", func(t *testing.T) {
		if got := ResolveAgentPrinciples(nil, ""); got != "" {
			t.Errorf("expected empty for nil cfg, got %q", got)
		}
	})
}

// --- resolveRules merge priority tests (UC-0014 ~ 0018) ---

func TestResolveRules(t *testing.T) {
	t.Run("only config rules", func(t *testing.T) {
		ws := t.TempDir()
		cfg := config.DefaultConfig()
		cfg.Rules = []string{"rule 1", "rule 2"}
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveRules(); got != "rule 1\nrule 2" {
			t.Errorf("expected joined config rules, got %q", got)
		}
	})

	t.Run("only rules dir", func(t *testing.T) {
		ws := t.TempDir()
		rulesDir := filepath.Join(ws, ".rules")
		if err := os.MkdirAll(rulesDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rulesDir, "team.md"), []byte("team rule"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveRules(); got != "# team.md\n\nteam rule" {
			t.Errorf("expected rules dir output, got %q", got)
		}
	})

	t.Run("config rules + rules dir merged, config first", func(t *testing.T) {
		ws := t.TempDir()
		rulesDir := filepath.Join(ws, ".rules")
		if err := os.MkdirAll(rulesDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rulesDir, "team.md"), []byte("team rule"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		cfg.Rules = []string{"config rule"}
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		got := a.resolveRules()
		if !strings.HasPrefix(got, "config rule\n\n# team.md") {
			t.Errorf("expected config rule first then rules dir, got: %q", got)
		}
	})

	t.Run("empty rules dir equals only config rules", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, ".rules"), 0755); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		cfg.Rules = []string{"only rule"}
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveRules(); got != "only rule" {
			t.Errorf("expected only config rule, got %q", got)
		}
	})

	t.Run("workspace rules dir wins over cwd", func(t *testing.T) {
		ws := t.TempDir()
		wsRules := filepath.Join(ws, ".rules")
		if err := os.MkdirAll(wsRules, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wsRules, "ws.md"), []byte("ws rule"), 0644); err != nil {
			t.Fatal(err)
		}
		cwd := t.TempDir()
		cwdRules := filepath.Join(cwd, ".rules")
		if err := os.MkdirAll(cwdRules, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cwdRules, "cwd.md"), []byte("cwd rule"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultConfig()
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		oldWD, _ := os.Getwd()
		defer os.Chdir(oldWD)
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
		got := a.resolveRules()
		if !strings.Contains(got, "ws rule") {
			t.Errorf("expected workspace rules dir to win, got %q", got)
		}
		if strings.Contains(got, "cwd rule") {
			t.Errorf("cwd rules dir should be ignored when workspace has one, got %q", got)
		}
	})

	t.Run("no config rules and no rules dir returns empty", func(t *testing.T) {
		ws := t.TempDir()
		cfg := config.DefaultConfig()
		a := newTestAgentWithCfg(cfg)
		a.workspacePath = ws
		if got := a.resolveRules(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// --- integration: rebuildSystemPrompt reflects live changes (UC-0019 ~ 0022) ---

func TestRebuildSystemPrompt_LiveRulesAndPrinciples(t *testing.T) {
	i18n.Init("en")

	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ConfigPath() // no-op, placeholder to reference method
	a := newTestAgentWithCfg(cfg)
	a.workspacePath = ws

	// UC-0019: config rules become visible after rebuild
	cfg.Rules = []string{"rule after add"}
	a.rebuildSystemPrompt()
	msg := a.Messages()
	if len(msg) == 0 || msg[0].Role != "system" {
		t.Fatalf("expected system message, got %d messages", len(msg))
	}
	if !strings.Contains(msg[0].Content, "rule after add") {
		t.Errorf("expected newly added config rule in system prompt, got: %q", msg[0].Content)
	}

	// UC-0020: .rules/ file content becomes visible after rebuild
	rulesDir := filepath.Join(ws, ".rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "a.md"), []byte("external rule"), 0644); err != nil {
		t.Fatal(err)
	}
	a.rebuildSystemPrompt()
	msg = a.Messages()
	if !strings.Contains(msg[0].Content, "# a.md") || !strings.Contains(msg[0].Content, "external rule") {
		t.Errorf("expected .rules/a.md content in system prompt, got: %q", msg[0].Content)
	}

	// UC-0021: PRINCIPLES.md content appears in Identity section
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("my principles content"), 0644); err != nil {
		t.Fatal(err)
	}
	a.rebuildSystemPrompt()
	msg = a.Messages()
	if !strings.Contains(msg[0].Content, "my principles content") {
		t.Errorf("expected PRINCIPLES.md content in system prompt, got: %q", msg[0].Content)
	}
}

func TestSetResultMode_UsesResolvedRulesAndPrinciples(t *testing.T) {
	i18n.Init("en")

	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.LLM.AgentPrinciples = "config principles"
	cfg.Rules = []string{"config rule"}
	a := newTestAgentWithCfg(cfg)
	a.workspacePath = ws

	// UC-0022: SetResultMode path also picks up PRINCIPLES.md + .rules/
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("file principles"), 0644); err != nil {
		t.Fatal(err)
	}
	rulesDir := filepath.Join(ws, ".rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "team.md"), []byte("team rule"), 0644); err != nil {
		t.Fatal(err)
	}

	a.SetResultMode(config.ResultModeMinimal)
	msg := a.Messages()
	if len(msg) == 0 {
		t.Fatal("expected messages after SetResultMode")
	}
	content := msg[0].Content
	if !strings.Contains(content, "file principles") {
		t.Errorf("expected file principles in SetResultMode output, got: %q", content)
	}
	if !strings.Contains(content, "team rule") {
		t.Errorf("expected team rule in SetResultMode output, got: %q", content)
	}
	if !strings.Contains(content, "config rule") {
		t.Errorf("expected config rule in SetResultMode output, got: %q", content)
	}
}
