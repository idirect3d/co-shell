package cmd

import (
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// fix592Param returns the named entry of a ConfigParam list.
func fix592Param(t *testing.T, params []ConfigParam, name string) ConfigParam {
	t.Helper()
	for _, p := range params {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("param %q not found", name)
	return ConfigParam{}
}

// newFix592ConfigHandler builds the .config wizard handler on top of a real
// (throwaway) agent so SetValue/ResetValue can re-point it without panicking.
func newFix592ConfigHandler(t *testing.T, cfg *config.Config) *ConfigHandler {
	t.Helper()
	wsPath := t.TempDir()
	wsObj, err := workspace.New(wsPath)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ag := agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")
	ag.SetWorkspacePath(wsPath)
	ag.SetConfig(cfg)
	return &ConfigHandler{cfg: cfg, agent: ag}
}

// TestFix592BoardEnabledDefaultsOnPersistsAndResets covers UC-0001 case E for
// board-enabled: the new default is on, an explicit off is persisted, and the
// wizard reset restores the default (on) instead of the old hard-coded off.
func TestFix592BoardEnabledDefaultsOnPersistsAndResets(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)
	if !cfg.BoardEnabled {
		t.Fatalf("BoardEnabled = false right after load, want the new default true")
	}

	if _, err := h.handleAgentSetting("board-enabled", []string{"board-enabled", "off"}); err != nil {
		t.Fatalf("handleAgentSetting(board-enabled off): %v", err)
	}
	if cfg.BoardEnabled {
		t.Error("board-enabled off did not take effect")
	}
	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload after off: %v", err)
	}
	if reloaded.BoardEnabled {
		t.Error(":set board-enabled off was not persisted (must survive a reload)")
	}

	ch := newFix592ConfigHandler(t, cfg)
	param := fix592Param(t, ch.agentParams(), "board-enabled")
	if got := param.ResetValue(); got == "" {
		t.Error("board-enabled ResetValue returned an empty message")
	}
	if !cfg.BoardEnabled {
		t.Error("board-enabled ResetValue must restore the default (on)")
	}
	afterReset, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload after reset: %v", err)
	}
	if !afterReset.BoardEnabled {
		t.Error("board-enabled reset to on was not persisted")
	}
}

// TestFix592SettingsStillWritable covers UC-0001 case E: the six settings keep
// their ranges/validation and every accepted change is persisted.
func TestFix592SettingsStillWritable(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)

	if _, err := h.handleDisplaySetting("stream-window-max-blocks", []string{"stream-window-max-blocks", "64"}); err != nil {
		t.Fatalf("set stream-window-max-blocks 64: %v", err)
	}
	if cfg.LLM.StreamWindowMaxBlocks != 64 {
		t.Errorf("StreamWindowMaxBlocks = %d, want 64", cfg.LLM.StreamWindowMaxBlocks)
	}
	if _, err := h.handleDisplaySetting("stream-window-max-blocks", []string{"stream-window-max-blocks", "9"}); err == nil {
		t.Error("stream-window-max-blocks below the lower bound (10) must be rejected")
	}
	if _, err := h.handleDisplaySetting("stream-window-max-nodes", []string{"stream-window-max-nodes", "20000"}); err != nil {
		t.Fatalf("set stream-window-max-nodes 20000: %v", err)
	}
	if _, err := h.handleDisplaySetting("stream-window-max-nodes", []string{"stream-window-max-nodes", "999"}); err == nil {
		t.Error("stream-window-max-nodes below the lower bound (1000) must be rejected")
	}
	if _, err := h.handleAgentSetting("browser-headless", []string{"browser-headless", "off"}); err != nil {
		t.Fatalf("set browser-headless off: %v", err)
	}
	if cfg.LLM.BrowserHeadless {
		t.Error("browser-headless off did not take effect")
	}
	if _, err := h.handleLLMSetting("temperature", []string{"temperature", "0.8"}); err != nil {
		t.Fatalf("set temperature 0.8: %v", err)
	}
	if cfg.LLM.Temperature != 0.8 {
		t.Errorf("Temperature = %v, want 0.8", cfg.LLM.Temperature)
	}
	if _, err := h.handleLLMSetting("temperature", []string{"temperature", "3"}); err == nil {
		t.Error("temperature above 2.0 must be rejected")
	}

	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.LLM.StreamWindowMaxBlocks != 64 {
		t.Errorf("persisted StreamWindowMaxBlocks = %d, want 64", reloaded.LLM.StreamWindowMaxBlocks)
	}
	if reloaded.LLM.StreamWindowMaxNodes != 20000 {
		t.Errorf("persisted StreamWindowMaxNodes = %d, want 20000", reloaded.LLM.StreamWindowMaxNodes)
	}
	if reloaded.LLM.Temperature != 0.8 {
		t.Errorf("persisted Temperature = %v, want 0.8", reloaded.LLM.Temperature)
	}
	if reloaded.LLM.BrowserHeadless {
		t.Error("persisted browser-headless = true, want false")
	}
}

// TestFix592SettingsPanelDefaults covers UC-0001 case G: the Web settings panel
// advertises the new system defaults (they come from config.DefaultConfig()).
func TestFix592SettingsPanelDefaults(t *testing.T) {
	h, _, _ := setupDefaultsTestHandler(t)

	items := map[string]WebSettingItem{}
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			items[it.Key] = it
		}
	}
	want := map[string]string{
		"temperature":              "0.5",
		"board-enabled":            "on",
		"search-max-result-bytes":  "128000",
		"stream-window-max-blocks": "128",
		"stream-window-max-nodes":  "12800",
		"browser-headless":         boolStr(config.DefaultConfig().LLM.BrowserHeadless),
	}
	for key, wantDefault := range want {
		it, ok := items[key]
		if !ok {
			t.Errorf("setting %q is missing from SettingsJSON()", key)
			continue
		}
		if it.Default != wantDefault {
			t.Errorf("%s default = %q, want %q", key, it.Default, wantDefault)
		}
	}
}
