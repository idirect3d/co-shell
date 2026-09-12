// Package agent - FEATURE-490 regression tests: the hub bulletin-board tools are
// only registered when the board switch is enabled, each tool sends the wire
// message the hub expects, and the three board dynamic events are rendered into
// the perception block.
//
// Author: L.Shuang
// Created: 2026-09-12
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/mcp"
)

// boardToolNames are the six tools FEATURE-490 registers.
var boardToolNames = []string{
	"board_post", "board_list", "board_claim", "board_dm", "board_confirm", "board_result",
}

// TestBoardToolsRegisteredOnlyWhenEnabled verifies the board collaboration
// tools are absent while the switch is off (the default) and present once it is
// turned on, so an instance never participates unless the user opts in.
//
// buildToolsInternal is used directly because buildTools() intentionally
// returns an empty list in XML mode / when tool calling is disabled.
func TestBoardToolsRegisteredOnlyWhenEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	a := &Agent{cfg: cfg, toolModes: map[string]string{}, mcpMgr: mcp.NewManager()}

	present := func() map[string]bool {
		got := map[string]bool{}
		for _, tl := range a.buildToolsInternal() {
			got[tl.Name] = true
		}
		return got
	}

	cfg.BoardEnabled = false
	off := present()
	for _, name := range boardToolNames {
		if off[name] {
			t.Errorf("tool %s must not be registered when BoardEnabled=false", name)
		}
	}

	cfg.BoardEnabled = true
	on := present()
	for _, name := range boardToolNames {
		if !on[name] {
			t.Errorf("tool %s missing when BoardEnabled=true", name)
		}
	}
}

// TestBoardToolsSendWireMessages verifies every board tool pushes the message
// type and payload the hub's board expects.
func TestBoardToolsSendWireMessages(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	var sent []map[string]interface{}
	a.SetBoardSender(func(msg map[string]interface{}) error {
		sent = append(sent, msg)
		return nil
	})

	if _, err := a.boardPostTool(nil, map[string]interface{}{"title": "T", "description": "D", "required_role": "reviewer"}); err != nil {
		t.Fatalf("board_post: %v", err)
	}
	if _, err := a.boardListTool(nil, nil); err != nil {
		t.Fatalf("board_list: %v", err)
	}
	if _, err := a.boardClaimTool(nil, map[string]interface{}{"request_id": "req-1"}); err != nil {
		t.Fatalf("board_claim: %v", err)
	}
	if _, err := a.boardDMTool(nil, map[string]interface{}{"request_id": "req-1", "content": "hi"}); err != nil {
		t.Fatalf("board_dm: %v", err)
	}
	if _, err := a.boardConfirmTool(nil, map[string]interface{}{"request_id": "req-1"}); err != nil {
		t.Fatalf("board_confirm: %v", err)
	}
	if _, err := a.boardResultTool(nil, map[string]interface{}{"task_id": "task-1", "result": "ok"}); err != nil {
		t.Fatalf("board_result: %v", err)
	}

	if len(sent) != len(boardToolNames) {
		t.Fatalf("sent %d messages, want %d", len(sent), len(boardToolNames))
	}
	for i, want := range boardToolNames {
		if got, _ := sent[i]["type"].(string); got != want {
			t.Errorf("message[%d].type = %v, want %s", i, sent[i]["type"], want)
		}
	}
	// Spot-check payloads so a field rename cannot silently break the hub.
	if sent[0]["title"] != "T" || sent[0]["required_role"] != "reviewer" {
		t.Errorf("board_post payload = %v, want title/required_role", sent[0])
	}
	if sent[2]["request_id"] != "req-1" {
		t.Errorf("board_claim payload = %v, want request_id", sent[2])
	}
	if sent[3]["content"] != "hi" {
		t.Errorf("board_dm payload = %v, want content", sent[3])
	}
	if sent[5]["task_id"] != "task-1" || sent[5]["result"] != "ok" {
		t.Errorf("board_result payload = %v, want task_id/result", sent[5])
	}
}

// TestBoardDynamicEventsRender verifies the three board perception kinds are
// drained into the <user_dynamic_events> block with their payload text, so the
// LLM learns about new requests, direct messages and state changes.
func TestBoardDynamicEventsRender(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.AddDynamicEvent(DynamicBoardRequest, "req-1: needs a Go concurrency review")
	a.AddDynamicEvent(DynamicBoardDM, "req-1: any progress?")
	a.AddDynamicEvent(DynamicBoardNotify, "req-1 claimed by reviewer")

	block := a.consumeDynamicEvents(false)
	for _, want := range []string{
		string(DynamicBoardRequest),
		string(DynamicBoardDM),
		string(DynamicBoardNotify),
		"needs a Go concurrency review",
		"any progress?",
		"claimed by reviewer",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("dynamic block should contain %q, got:\n%s", want, block)
		}
	}
}

// TestBoardToolsRequireSender verifies the tools fail cleanly (instead of
// panicking) when no board sender is installed, e.g. when the agent runs in the
// REPL instead of serve mode.
func TestBoardToolsRequireSender(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	if _, err := a.boardPostTool(nil, map[string]interface{}{"title": "t", "description": "d"}); err == nil {
		t.Error("board_post should fail without a board sender")
	}
	if _, err := a.boardListTool(nil, nil); err == nil {
		t.Error("board_list should fail without a board sender")
	}
	if _, err := a.boardResultTool(nil, map[string]interface{}{"task_id": "t1", "result": "r"}); err == nil {
		t.Error("board_result should fail without a board sender")
	}
}
