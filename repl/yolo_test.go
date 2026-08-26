// Package repl - YOLO (You Only Live Once) master switch command tests
// (FEATURE-439).
//
// Author: L.Shuang
// Created: 2026-08-26
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"testing"

	"github.com/idirect3d/co-shell/agent"
)

// TestYOLOCommandToggle verifies :YOLO toggles the YOLO state and reports the
// resulting state (UC-0006).
func TestYOLOCommandToggle(t *testing.T) {
	r := &REPL{agent: &agent.Agent{}}

	// First :YOLO turns it on.
	res, err := r.handleYOLOCommand()
	if err != nil {
		t.Fatalf("handleYOLOCommand error: %v", err)
	}
	if !r.agent.IsYOLO() {
		t.Errorf("IsYOLO() = false after first :YOLO, want true")
	}
	if res == "" {
		t.Errorf("handleYOLOCommand returned empty result, want state message")
	}

	// Second :YOLO turns it off.
	res, err = r.handleYOLOCommand()
	if err != nil {
		t.Fatalf("handleYOLOCommand error: %v", err)
	}
	if r.agent.IsYOLO() {
		t.Errorf("IsYOLO() = true after second :YOLO, want false")
	}
	if res == "" {
		t.Errorf("handleYOLOCommand returned empty result, want state message")
	}
}
