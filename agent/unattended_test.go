// Package agent - unattended mode and run-level tool shielding tests
// (FEATURE-563, use-case A-group item 9).
//
// Author: L.Shuang
// Created: 2026-09-27
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"testing"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
)

// hasToolNamed reports whether the tool list advertises the given name.
func hasToolNamed(tools []llm.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// newShieldingTestAgent builds the minimal agent the tool registry needs.
func newShieldingTestAgent() *Agent {
	return &Agent{
		mcpMgr:                mcp.NewManager(),
		toolCallEnabled:       true,
		intentExposureEnabled: true,
		toolModes:             DefaultToolModes(),
	}
}

// TestUnattendedDisablesInteractiveTools verifies an unattended run never
// offers a tool that would block on a human (UC: 无人值守禁用交互等待).
func TestUnattendedDisablesInteractiveTools(t *testing.T) {
	ag := newShieldingTestAgent()
	if !hasToolNamed(ag.buildToolsInternal(), "ask_user") {
		t.Fatal("ask_user must be advertised before unattended mode is on")
	}

	ag.SetUnattended(true)
	if !ag.Unattended() {
		t.Error("Unattended() must report the mode SetUnattended turned on")
	}
	tools := ag.buildToolsInternal()
	for _, name := range unattendedToolNames {
		if hasToolNamed(tools, name) {
			t.Errorf("tool %q must not be advertised in unattended mode", name)
		}
	}
}

// TestDisabledToolsAreShielded verifies --disabled-tools removes exactly the
// named tools and nothing else (UC: --disabled-tools).
func TestDisabledToolsAreShielded(t *testing.T) {
	ag := newShieldingTestAgent()
	ag.SetDisabledTools([]string{"read_file", "  ", "execute_command"})

	tools := ag.buildToolsInternal()
	for _, name := range []string{"read_file", "execute_command"} {
		if hasToolNamed(tools, name) {
			t.Errorf("tool %q must be shielded by SetDisabledTools", name)
		}
	}
	if !hasToolNamed(tools, "list_files") {
		t.Error("tools outside the disabled list must stay advertised")
	}
}
