// Package agent - per-turn result-mode override tests (FEATURE-610 P1).
//
// Verifies:
//  1. parsePerTurnResultMode parses a <result_mode> tag from the user's message
//     and sets a per-turn override.
//  2. An invalid <result_mode> value is ignored (persistent mode stays).
//  3. effectiveResultMode prefers the per-turn override over the persistent mode.
//  4. clearPerTurnResultMode resets the override so it does not leak.
//
// Author: L.Shuang
// Created: 2026-10-06
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestParsePerTurnResultMode_Valid verifies a valid <result_mode> tag sets the
// per-turn override.
func TestParsePerTurnResultMode_Valid(t *testing.T) {
	a := &Agent{}
	a.parsePerTurnResultMode("请用 <result_mode>machine</result_mode> 输出")
	if !a.perTurnResultModeSet {
		t.Fatal("perTurnResultModeSet should be true after parsing a valid tag")
	}
	if a.perTurnResultMode != config.ResultModeMachine {
		t.Errorf("perTurnResultMode = %d, want %d", a.perTurnResultMode, config.ResultModeMachine)
	}
}

// TestParsePerTurnResultMode_NoTag verifies no tag leaves the override unset.
func TestParsePerTurnResultMode_NoTag(t *testing.T) {
	a := &Agent{}
	a.parsePerTurnResultMode("普通消息，没有标签")
	if a.perTurnResultModeSet {
		t.Error("perTurnResultModeSet should be false when no tag present")
	}
}

// TestParsePerTurnResultMode_Invalid verifies an invalid value is ignored.
func TestParsePerTurnResultMode_Invalid(t *testing.T) {
	a := &Agent{}
	a.parsePerTurnResultMode("用 <result_mode>bogus</result_mode> 输出")
	if a.perTurnResultModeSet {
		t.Error("perTurnResultModeSet should be false for an invalid value")
	}
}

// TestParsePerTurnResultMode_UnclosedTag verifies an unclosed tag is ignored.
func TestParsePerTurnResultMode_UnclosedTag(t *testing.T) {
	a := &Agent{}
	a.parsePerTurnResultMode("用 <result_mode>machine 输出")
	if a.perTurnResultModeSet {
		t.Error("perTurnResultModeSet should be false for an unclosed tag")
	}
}

// TestEffectiveResultMode_PrefersOverride verifies the per-turn override wins
// over the persistent resultMode.
func TestEffectiveResultMode_PrefersOverride(t *testing.T) {
	a := &Agent{resultMode: config.ResultModeAnalyze}
	a.parsePerTurnResultMode("<result_mode>machine</result_mode>")
	if got := a.effectiveResultMode(); got != config.ResultModeMachine {
		t.Errorf("effectiveResultMode = %d, want %d (override should win)", got, config.ResultModeMachine)
	}
}

// TestEffectiveResultMode_FallsBackToPersistent verifies the persistent mode is
// used when no override is set.
func TestEffectiveResultMode_FallsBackToPersistent(t *testing.T) {
	a := &Agent{resultMode: config.ResultModeExplain}
	if got := a.effectiveResultMode(); got != config.ResultModeExplain {
		t.Errorf("effectiveResultMode = %d, want %d (persistent)", got, config.ResultModeExplain)
	}
}

// TestClearPerTurnResultMode verifies clearing resets the override.
func TestClearPerTurnResultMode(t *testing.T) {
	a := &Agent{resultMode: config.ResultModeFree}
	a.parsePerTurnResultMode("<result_mode>minimal</result_mode>")
	if !a.perTurnResultModeSet {
		t.Fatal("override should be set before clearing")
	}
	a.clearPerTurnResultMode()
	if a.perTurnResultModeSet {
		t.Error("perTurnResultModeSet should be false after clearing")
	}
	if got := a.effectiveResultMode(); got != config.ResultModeFree {
		t.Errorf("effectiveResultMode after clear = %d, want %d (persistent)", got, config.ResultModeFree)
	}
}

// TestBuildEnvironmentDetails_PerTurnOverride verifies the injected
// <result_mode> reflects the per-turn override when set.
func TestBuildEnvironmentDetails_PerTurnOverride(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.resultMode = config.ResultModeAnalyze
	a.parsePerTurnResultMode("<result_mode>machine</result_mode>")
	env := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env, "<result_mode>machine</result_mode>") {
		t.Errorf("environment_details missing per-turn <result_mode>machine, got:\n%s", env)
	}
}
