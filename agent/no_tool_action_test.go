// Package agent - no-tool-action prompt optimization tests.
//
// Verifies:
//  1. noToolAction() returns the configured strategy, defaulting to "exit".
//  2. buildFullEnvironmentDetails injects <no_tool_action> reflecting the
//     current config, and it updates when the config changes.
//
// Author: L.Shuang
// Created: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestNoToolAction_Default verifies noToolAction() defaults to "exit" when the
// config is nil or the field is unset.
func TestNoToolAction_Default(t *testing.T) {
	// nil config
	a := &Agent{}
	if got := a.noToolAction(); got != "exit" {
		t.Errorf("noToolAction() with nil config = %q, want \"exit\"", got)
	}

	// config with empty field
	a2 := &Agent{cfg: &config.Config{}}
	if got := a2.noToolAction(); got != "exit" {
		t.Errorf("noToolAction() with empty field = %q, want \"exit\"", got)
	}
}

// TestNoToolAction_Configured verifies noToolAction() returns the configured
// strategy when set.
func TestNoToolAction_Configured(t *testing.T) {
	for _, val := range []string{"exit", "retry", "prompt"} {
		a := &Agent{cfg: &config.Config{}}
		a.cfg.LLM.NoToolAction = val
		if got := a.noToolAction(); got != val {
			t.Errorf("noToolAction() with %q = %q, want %q", val, got, val)
		}
	}
}

// TestBuildEnvironmentDetails_NoToolAction verifies <no_tool_action> is injected
// into the environment details block and reflects the current config.
func TestBuildEnvironmentDetails_NoToolAction(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.NoToolAction = "prompt"
	env := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env, "<no_tool_action>prompt</no_tool_action>") {
		t.Errorf("environment_details missing <no_tool_action>prompt, got:\n%s", env)
	}
}

// TestBuildEnvironmentDetails_NoToolActionDefault verifies the default "exit"
// is injected when the field is unset.
func TestBuildEnvironmentDetails_NoToolActionDefault(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	env := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env, "<no_tool_action>exit</no_tool_action>") {
		t.Errorf("environment_details missing <no_tool_action>exit, got:\n%s", env)
	}
}

// TestBuildEnvironmentDetails_NoToolActionReflectsChange verifies the injected
// value updates when the config changes between calls (dynamic reflection).
func TestBuildEnvironmentDetails_NoToolActionReflectsChange(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.cfg.LLM.NoToolAction = "retry"
	env1 := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env1, "<no_tool_action>retry</no_tool_action>") {
		t.Errorf("env1 missing <no_tool_action>retry, got:\n%s", env1)
	}

	// Change config and rebuild — must reflect the new value.
	a.cfg.LLM.NoToolAction = "prompt"
	env2 := a.buildFullEnvironmentDetails(2, nil)
	if !strings.Contains(env2, "<no_tool_action>prompt</no_tool_action>") {
		t.Errorf("env2 missing <no_tool_action>prompt, got:\n%s", env2)
	}
}
