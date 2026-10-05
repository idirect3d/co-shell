// Package agent - result-mode env injection tests (FEATURE-610).
//
// Verifies:
//  1. buildFullEnvironmentDetails injects <result_mode> reflecting the current
//     agent resultMode field.
//  2. The injected value updates when the agent's resultMode changes.
//
// Author: L.Shuang
// Created: 2026-10-05
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestBuildEnvironmentDetails_ResultMode verifies <result_mode> is injected into
// the environment details block and reflects the agent's current resultMode.
func TestBuildEnvironmentDetails_ResultMode(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.resultMode = config.ResultModeMachine
	env := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env, "<result_mode>machine</result_mode>") {
		t.Errorf("environment_details missing <result_mode>machine, got:\n%s", env)
	}
}

// TestBuildEnvironmentDetails_ResultModeDefault verifies the default "free" is
// injected when resultMode is unset (zero value = ResultModeMinimal).
func TestBuildEnvironmentDetails_ResultModeDefault(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	env := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env, "<result_mode>minimal</result_mode>") {
		t.Errorf("environment_details missing <result_mode>minimal, got:\n%s", env)
	}
}

// TestBuildEnvironmentDetails_ResultModeReflectsChange verifies the injected
// value updates when the agent's resultMode changes between calls.
func TestBuildEnvironmentDetails_ResultModeReflectsChange(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.resultMode = config.ResultModeAnalyze
	env1 := a.buildFullEnvironmentDetails(1, nil)
	if !strings.Contains(env1, "<result_mode>analyze</result_mode>") {
		t.Errorf("env1 missing <result_mode>analyze, got:\n%s", env1)
	}

	a.resultMode = config.ResultModeFree
	env2 := a.buildFullEnvironmentDetails(2, nil)
	if !strings.Contains(env2, "<result_mode>free</result_mode>") {
		t.Errorf("env2 missing <result_mode>free, got:\n%s", env2)
	}
}
