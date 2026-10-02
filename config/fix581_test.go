// Package config - FIX-581: demo command timeout default and config loading.
//
// Author: L.Shuang
// Created: 2026-10-02

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFIX581DemoCommandTimeoutDefaults verifies UC-0001 and UC-0002: the default
// is 3s, and a config file that lacks the field keeps the 3s default instead of
// falling back to 0 (= no timeout, i.e. no protection).
func TestFIX581DemoCommandTimeoutDefaults(t *testing.T) {
	if got := DefaultConfig().LLM.DemoCommandTimeout; got != 3 {
		t.Errorf("DefaultConfig DemoCommandTimeout = %d, want 3", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Legacy config: the llm object exists but carries no demo_command_timeout.
	legacy := `{"llm":{"command_timeout":0,"tool_timeout":0},"log_level":"info"}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("cannot write legacy config: %v", err)
	}

	cfg, _, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile error: %v", err)
	}
	if got := cfg.LLM.DemoCommandTimeout; got != 3 {
		t.Errorf("legacy config DemoCommandTimeout = %d, want 3 (default fallback)", got)
	}
}
