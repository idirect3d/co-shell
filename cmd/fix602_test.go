// FEATURE-602 unit tests (cmd level): the automatic-selection switch and the
// configurable LLM error retry limit are reachable through the settings handler
// (shared by `:set` and the Web settings page) and persist.

package cmd

import (
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestFix602AutoModelSelectSetting covers UC-0001: off by default, on/off take
// effect, "on" is persisted, and an invalid value is rejected.
func TestFix602AutoModelSelectSetting(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)
	if cfg.LLM.AutoSelectEnabled() {
		t.Fatal("auto-model-select must default off")
	}

	if _, err := h.handleAgentSetting("auto-model-select", []string{"auto-model-select", "on"}); err != nil {
		t.Fatalf("set on: %v", err)
	}
	if !cfg.LLM.AutoSelectEnabled() {
		t.Error("auto-model-select on did not take effect")
	}
	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.LLM.AutoSelectEnabled() {
		t.Error("auto-model-select on was not persisted")
	}

	if _, err := h.handleAgentSetting("auto-model-select", []string{"auto-model-select", "off"}); err != nil {
		t.Fatalf("set off: %v", err)
	}
	if cfg.LLM.AutoSelectEnabled() {
		t.Error("auto-model-select off did not take effect")
	}
	if _, err := h.handleAgentSetting("auto-model-select", []string{"auto-model-select", "maybe"}); err == nil {
		t.Error("an invalid value must be rejected")
	}
}

// TestFix602LLMErrRetryLimitSetting covers UC-0004: the retry budget is a
// number setting, persisted, and values below 1 are rejected.
func TestFix602LLMErrRetryLimitSetting(t *testing.T) {
	h, cfg, cfgPath := setupDefaultsTestHandler(t)
	if _, err := h.handleSafetySetting("llm-err-retry-limit", []string{"llm-err-retry-limit", "5"}); err != nil {
		t.Fatalf("set 5: %v", err)
	}
	if cfg.LLM.LLMErrRetryLimit != 5 {
		t.Errorf("LLMErrRetryLimit = %d, want 5", cfg.LLM.LLMErrRetryLimit)
	}
	reloaded, _, err := config.LoadFromFile(cfgPath, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.LLM.LLMErrRetryLimit != 5 {
		t.Errorf("reloaded LLMErrRetryLimit = %d, want 5", reloaded.LLM.LLMErrRetryLimit)
	}
	for _, bad := range []string{"0", "-1", "abc"} {
		if _, err := h.handleSafetySetting("llm-err-retry-limit", []string{"llm-err-retry-limit", bad}); err == nil {
			t.Errorf("value %q must be rejected", bad)
		}
	}
}
