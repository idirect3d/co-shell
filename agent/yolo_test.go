// Package agent - YOLO (You Only Live Once) master switch tests (FEATURE-439).
//
// Author: L.Shuang
// Created: 2026-08-26
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
)

// TestYOLODefaultOff verifies the YOLO master switch defaults to off (UC-0001).
func TestYOLODefaultOff(t *testing.T) {
	a := &Agent{}
	if a.IsYOLO() {
		t.Errorf("IsYOLO() = true, want false (default off)")
	}
}

// TestYOLOSetGet verifies SetYOLO/IsYOLO round-trip (UC-0002).
func TestYOLOSetGet(t *testing.T) {
	a := &Agent{}
	a.SetYOLO(true)
	if !a.IsYOLO() {
		t.Errorf("IsYOLO() = false after SetYOLO(true), want true")
	}
	a.SetYOLO(false)
	if a.IsYOLO() {
		t.Errorf("IsYOLO() = true after SetYOLO(false), want false")
	}
}

// TestYOLOConfirmSkip verifies that when YOLO is on, a confirm-mode tool call
// skips the confirmation prompt and executes directly (UC-0003). When YOLO is
// off, the same tool call reaches the confirmation entry (UC-0004).
func TestYOLOConfirmSkip(t *testing.T) {
	// Build a tool call for a confirm-mode tool (execute_command).
	args, _ := json.Marshal(map[string]interface{}{"command": "echo hi", "intent": "test"})
	tc := llm.ToolCall{Name: "execute_command", Arguments: string(args)}

	// A capture manager records whether Ask (confirmation) was invoked.
	mgr := &captureInteractionManager{askResult: InteractionResult{Action: ActionApprove}}

	// YOLO OFF: confirmation should be reached (Ask called).
	aOff := &Agent{toolModes: map[string]string{"execute_command": "confirm"}, mcpMgr: mcp.NewManager()}
	aOff.interactionMgr = mgr
	_, err := aOff.executeToolCall(context.Background(), tc)
	if err == nil {
		t.Errorf("YOLO off: expected an error (no tool callback installed), got nil")
	}
	if mgr.captured.Kind != InteractionConfirm {
		t.Errorf("YOLO off: Ask not called with confirm interaction (captured=%+v), want confirmation", mgr.captured)
	}

	// YOLO ON: confirmation should be skipped (Ask NOT called).
	mgr2 := &captureInteractionManager{askResult: InteractionResult{Action: ActionApprove}}
	aOn := &Agent{toolModes: map[string]string{"execute_command": "confirm"}, mcpMgr: mcp.NewManager()}
	aOn.interactionMgr = mgr2
	aOn.SetYOLO(true)
	_, err = aOn.executeToolCall(context.Background(), tc)
	if err == nil {
		t.Errorf("YOLO on: expected an error (no tool callback installed), got nil")
	}
	if mgr2.captured.Kind == InteractionConfirm {
		t.Errorf("YOLO on: Ask was called with confirm interaction, want confirmation skipped")
	}
}

// TestYOLODisabledUnaffected verifies that a disabled tool is not sent to the
// LLM (so it never reaches the confirmation entry), regardless of YOLO
// (UC-0005). We check DefaultToolModes does not include disabled tools in the
// built tool list.
func TestYOLODisabledUnaffected(t *testing.T) {
	// A tool set to "disabled" is filtered out of the tool list sent to the LLM.
	a := &Agent{toolModes: map[string]string{"execute_command": "disabled"}}
	a.SetYOLO(true)
	tools := a.buildTools()
	for _, tl := range tools {
		if tl.Name == "execute_command" {
			t.Errorf("disabled tool execute_command still present in tool list, want filtered out")
		}
	}
}
