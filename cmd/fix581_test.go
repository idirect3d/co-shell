// Package cmd - FIX-581: demo-cmd-timeout parameter in the .set list.
//
// Author: L.Shuang
// Created: 2026-10-02

package cmd

import (
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// TestFIX581DemoCmdTimeoutParam verifies UC-0003 and UC-0004: the new parameter
// is part of the .set safety group, can be read/set, rejects invalid values
// without changing the current value, accepts 0 (= unlimited) and resets to 3.
func TestFIX581DemoCmdTimeoutParam(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &ConfigHandler{cfg: cfg}

	params := h.safetyParams()
	var param *ConfigParam
	for i := range params {
		if params[i].Name == "demo-cmd-timeout" {
			param = &params[i]
			break
		}
	}
	if param == nil {
		t.Fatal("demo-cmd-timeout not present in the .set safety params")
	}

	if got := param.CurrentValue(); got != "3s" {
		t.Errorf("CurrentValue = %q, want 3s", got)
	}
	if _, err := param.SetValue("5"); err != nil {
		t.Fatalf("SetValue(5) error: %v", err)
	}
	if cfg.LLM.DemoCommandTimeout != 5 {
		t.Errorf("DemoCommandTimeout = %d, want 5", cfg.LLM.DemoCommandTimeout)
	}
	if _, err := param.SetValue("abc"); err == nil {
		t.Error("SetValue(abc) error = nil, want a validation error")
	}
	if _, err := param.SetValue("-1"); err == nil {
		t.Error("SetValue(-1) error = nil, want a validation error")
	}
	if cfg.LLM.DemoCommandTimeout != 5 {
		t.Errorf("invalid input changed the value to %d, want 5", cfg.LLM.DemoCommandTimeout)
	}
	if _, err := param.SetValue("0"); err != nil {
		t.Fatalf("SetValue(0) error: %v", err)
	}
	if got := param.CurrentValue(); got != i18n.T(i18n.KeyUnlimited) {
		t.Errorf("CurrentValue after 0 = %q, want %q", got, i18n.T(i18n.KeyUnlimited))
	}
	if got := param.ResetValue(); got == "" {
		t.Error("ResetValue returned an empty message")
	}
	if cfg.LLM.DemoCommandTimeout != 3 {
		t.Errorf("after reset DemoCommandTimeout = %d, want 3", cfg.LLM.DemoCommandTimeout)
	}
}
