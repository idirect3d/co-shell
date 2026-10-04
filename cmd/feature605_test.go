// FEATURE-605 unit tests (cmd level): the per-session sub-agent pool size is
// configurable through the settings handler (shared by `:set` and the Web
// settings page), validated to 0..MaxPoolSize and persisted.

package cmd

import (
	"strconv"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/subagent"
)

// TestFeature605MaxSubAgentsSetting covers the `.set max-sub-agents` path:
// legal values take effect and persist, out-of-range / non-numeric values are
// rejected without touching the config, and the no-argument form reports the
// current pool.
func TestFeature605MaxSubAgentsSetting(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)
	if got := cfg.LLM.MaxSubAgents; got != subagent.DefaultMaxSubAgents {
		t.Fatalf("default max_sub_agents = %d, want %d", got, subagent.DefaultMaxSubAgents)
	}

	// A legal value applies and persists.
	if _, err := h.handleAgentSetting("max-sub-agents", []string{"max-sub-agents", "3"}); err != nil {
		t.Fatalf("set 3: %v", err)
	}
	if cfg.LLM.MaxSubAgents != 3 {
		t.Errorf("cfg.LLM.MaxSubAgents = %d, want 3", cfg.LLM.MaxSubAgents)
	}
	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.LLM.MaxSubAgents != 3 {
		t.Errorf("persisted max_sub_agents = %d, want 3", reloaded.LLM.MaxSubAgents)
	}

	// Both boundaries (0 disables, MaxPoolSize is the hard ceiling) are legal.
	for _, n := range []int{0, subagent.MaxPoolSize} {
		if _, err := h.handleAgentSetting("max-sub-agents", []string{"max-sub-agents", strconv.Itoa(n)}); err != nil {
			t.Errorf("set %d: %v", n, err)
		}
		if cfg.LLM.MaxSubAgents != n {
			t.Errorf("cfg.LLM.MaxSubAgents = %d, want %d", cfg.LLM.MaxSubAgents, n)
		}
	}

	// Invalid values are rejected and leave the config untouched.
	if _, err := h.handleAgentSetting("max-sub-agents", []string{"max-sub-agents", "3"}); err != nil {
		t.Fatalf("reset 3: %v", err)
	}
	for _, bad := range []string{"11", "-1", "abc", "", "3.5"} {
		if _, err := h.handleAgentSetting("max-sub-agents", []string{"max-sub-agents", bad}); err == nil {
			t.Errorf("value %q must be rejected", bad)
		}
		if cfg.LLM.MaxSubAgents != 3 {
			t.Fatalf("a rejected value %q changed the config to %d", bad, cfg.LLM.MaxSubAgents)
		}
	}

	// The no-argument form reports the current pool.
	msg, err := h.handleAgentSetting("max-sub-agents", []string{"max-sub-agents"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if msg == "" {
		t.Error("query must report the current pool")
	}
}

// TestFeature605SettingsJSONExposesMaxSubAgents covers the Web settings page:
// the Agent group carries an editable number item whose value and default match
// the config, so the panel renders and marks it correctly (FEATURE-470).
func TestFeature605SettingsJSONExposesMaxSubAgents(t *testing.T) {
	h, cfg, _ := setupDefaultsTestHandler(t)
	cfg.LLM.MaxSubAgents = 4

	found := false
	for _, g := range h.SettingsJSON() {
		for _, it := range g.Items {
			if it.Key != "max-sub-agents" {
				continue
			}
			found = true
			if it.Type != "number" {
				t.Errorf("type = %q, want number", it.Type)
			}
			if it.Value != "4" {
				t.Errorf("value = %q, want 4", it.Value)
			}
			if it.Default != strconv.Itoa(subagent.DefaultMaxSubAgents) {
				t.Errorf("default = %q, want %d", it.Default, subagent.DefaultMaxSubAgents)
			}
			if it.Desc == "" {
				t.Error("description must not be empty")
			}
		}
	}
	if !found {
		t.Error("SettingsJSON is missing the max-sub-agents item")
	}
}
