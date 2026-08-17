// Package cmd - model setup wizard non-interactive guard tests (FIX-358).
//
// Author: L.Shuang
// Created: 2026-08-18
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// TestAddModelWizardNonInteractive verifies the wizard fails fast with the
// dedicated error when stdin is not a terminal, instead of spinning on EOF
// and flooding stdout (FIX-358: a first piped run flooded gigabytes).
func TestAddModelWizardNonInteractive(t *testing.T) {
	cfg := &config.Config{}
	h := NewModelHandler(cfg, nil)
	h.stdinIsTTY = func() bool { return false }

	done := make(chan error, 1)
	go func() {
		out, err := h.AddModelWizard()
		if out != "" {
			t.Errorf("got non-empty wizard output %q, want empty", out)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), i18n.T(i18n.KeySetupNonInteractive)) {
			t.Fatalf("got err %v, want KeySetupNonInteractive", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AddModelWizard did not return within 2s on non-terminal stdin")
	}
}
