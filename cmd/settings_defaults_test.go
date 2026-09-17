package cmd

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// setupDefaultsTestHandler builds a SettingsHandler backed by a throwaway
// workspace, so :set defaults writes land in a temp config.json instead of the
// real one. It also chdirs into another temp dir so renameModeRetainedDirs
// never finds a mode/ directory and never blocks on user input (FIX-532).
func setupDefaultsTestHandler(t *testing.T) (*SettingsHandler, *config.Config, string) {
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
	t.Cleanup(func() { _ = boltStore.Close() })
	ds := store.NewDualStore(boltStore, nil)

	cfg, _, err := config.LoadWithPath(wsObj)
	if err != nil {
		t.Fatalf("cannot load config: %v", err)
	}

	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(wsPath)
	ag.SetConfig(cfg)

	t.Chdir(t.TempDir())

	return &SettingsHandler{cfg: cfg, agent: ag}, cfg, wsObj.ConfigPath()
}

// TestSetDefaultsClearsWorkModes verifies FIX-532 UC-0001: :set defaults clears
// the work mode list both in memory and on disk, and falls back to act when the
// active mode pointed to a removed custom mode.
func TestSetDefaultsClearsWorkModes(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)

	cfg.WorkModes = []config.WorkMode{
		{Name: "act", Description: "行动模式 - 可执行系统命令、修改文件、操作浏览器等所有操作"},
		{Name: "plan", Description: "规划模式 - 仅分析和规划，不执行系统命令、不修改文件"},
		{Name: "review", Description: "code review mode"},
	}
	cfg.LLM.WorkMode = "review"

	msg, err := h.handleSetDefault()
	if err != nil {
		t.Fatalf("handleSetDefault: %v", err)
	}
	if strings.TrimSpace(msg) == "" {
		t.Fatal("expected a non-empty reset message")
	}
	if len(cfg.WorkModes) != 0 {
		t.Errorf("WorkModes not cleared in memory: %+v", cfg.WorkModes)
	}
	if cfg.LLM.WorkMode != "act" {
		t.Errorf("WorkMode = %q, want %q", cfg.LLM.WorkMode, "act")
	}

	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.WorkModes) != 0 {
		t.Errorf("work_modes still persisted on disk: %+v", reloaded.WorkModes)
	}
}

// TestSetDefaultsWorkModeFallback verifies FIX-532 UC-0003: built-in modes are
// kept, anything else (custom or empty) falls back to act.
func TestSetDefaultsWorkModeFallback(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"custom falls back to act", "review", "act"},
		{"builtin act kept", "act", "act"},
		{"builtin plan kept", "plan", "plan"},
		{"builtin research kept", "research", "research"},
		{"empty falls back to act", "", "act"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg, _ := setupDefaultsTestHandler(t)
			cfg.LLM.WorkMode = tc.in

			if _, err := h.handleSetDefault(); err != nil {
				t.Fatalf("handleSetDefault: %v", err)
			}
			if cfg.LLM.WorkMode != tc.want {
				t.Errorf("WorkMode = %q, want %q", cfg.LLM.WorkMode, tc.want)
			}
		})
	}
}

// TestSetDefaultsPreservesLLMContextAndDB verifies FIX-532 UC-0004: the reset
// still preserves Memory & Context and DB settings (and its existing reset
// behaviour for display switches), while the work mode list is the only newly
// cleared field.
func TestSetDefaultsPreservesLLMContextAndDB(t *testing.T) {
	h, cfg, _ := setupDefaultsTestHandler(t)

	cfg.LLM.MemoryEnabled = false
	cfg.LLM.ContextLimit = 7
	cfg.LLM.ContextPolicy = "custom-policy"
	cfg.LLM.ContextReorganizeThreshold = 42
	cfg.LLM.MemorySearchMaxContentLen = 123
	cfg.LLM.MemorySearchMaxResults = 9
	cfg.DB.Enabled = true

	def := config.DefaultConfig()
	cfg.LLM.ResultMode = int(config.ResultModeFree)
	cfg.LLM.ShowLogo = !def.LLM.ShowLogo

	if _, err := h.handleSetDefault(); err != nil {
		t.Fatalf("handleSetDefault: %v", err)
	}

	if cfg.LLM.MemoryEnabled {
		t.Error("MemoryEnabled must be preserved (false)")
	}
	if cfg.LLM.ContextLimit != 7 {
		t.Errorf("ContextLimit = %d, want 7", cfg.LLM.ContextLimit)
	}
	if cfg.LLM.ContextPolicy != "custom-policy" {
		t.Errorf("ContextPolicy = %q, want custom-policy", cfg.LLM.ContextPolicy)
	}
	if cfg.LLM.ContextReorganizeThreshold != 42 {
		t.Errorf("ContextReorganizeThreshold = %v, want 0.42", cfg.LLM.ContextReorganizeThreshold)
	}
	if cfg.LLM.MemorySearchMaxContentLen != 123 || cfg.LLM.MemorySearchMaxResults != 9 {
		t.Errorf("memory search limits not preserved: %d/%d",
			cfg.LLM.MemorySearchMaxContentLen, cfg.LLM.MemorySearchMaxResults)
	}
	if !cfg.DB.Enabled {
		t.Error("DB config must be preserved")
	}

	if cfg.LLM.ResultMode != def.LLM.ResultMode {
		t.Errorf("ResultMode = %d, want default %d", cfg.LLM.ResultMode, def.LLM.ResultMode)
	}
	if cfg.LLM.ShowLogo != def.LLM.ShowLogo {
		t.Errorf("ShowLogo = %v, want default %v", cfg.LLM.ShowLogo, def.LLM.ShowLogo)
	}
}

// TestSetDefaultsIdempotent verifies FIX-532 UC-0005: running the reset twice
// (with no work modes configured) stays error-free and stable.
func TestSetDefaultsIdempotent(t *testing.T) {
	h, cfg, _ := setupDefaultsTestHandler(t)
	cfg.WorkModes = nil
	cfg.LLM.WorkMode = "act"

	for i := 1; i <= 2; i++ {
		if _, err := h.handleSetDefault(); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	if len(cfg.WorkModes) != 0 {
		t.Errorf("WorkModes = %+v, want empty", cfg.WorkModes)
	}
	if cfg.LLM.WorkMode != "act" {
		t.Errorf("WorkMode = %q, want act", cfg.LLM.WorkMode)
	}
}
