// Package agent - unit tests for the render_ui tool and its stream events
// (FEATURE-524, use-case group B: UC-07 .. UC-10).
//
// UC-08/UC-09 (event constructors) live in this file rather than a separate
// events_test.go so that the whole FEATURE-524 surface stays in one place.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
)

// newUITestAgent builds the minimal agent the render_ui tool needs.
func newUITestAgent(uiEnabled bool) *Agent {
	cfg := &config.Config{UIEnabled: uiEnabled}
	ag := &Agent{
		toolCallEnabled:       true,
		intentExposureEnabled: true,
		mcpMgr:                mcp.NewManager(),
		toolModes:             map[string]string{},
	}
	ag.SetConfig(cfg)
	ag.SyncToolModes(cfg)
	return ag
}

// hasTool reports whether the tool list contains the named tool.
func hasTool(tools []llm.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// UC-07: render_ui returns a one-line receipt (never the tree) and hands the
// validated tree to the stream loop exactly once.
func TestUIUC07RenderUIToolReceipt(t *testing.T) {
	ag := newUITestAgent(true)
	args := map[string]interface{}{
		"meta": map[string]interface{}{"intent": "show sales"},
		"tree": map[string]interface{}{
			"type":     "card",
			"children": []interface{}{map[string]interface{}{"type": "kv"}},
		},
	}

	receipt, err := ag.renderUITool(context.Background(), args)
	if err != nil {
		t.Fatalf("UC-07: render_ui failed: %v", err)
	}
	if !strings.Contains(receipt, "card") {
		t.Fatalf("UC-07: receipt %q must name the root component", receipt)
	}
	if !strings.Contains(receipt, "ui-") {
		t.Fatalf("UC-07: receipt %q must return the tree id for later ui_update", receipt)
	}
	if strings.Contains(receipt, "children") {
		t.Fatalf("UC-07: receipt %q must not echo the tree back into the context", receipt)
	}

	root, id := ag.takePendingUITree()
	if root == nil || root.Type != UICompCard {
		t.Fatalf("UC-07: pending tree = %+v, want a card", root)
	}
	if id == "" {
		t.Fatal("UC-07: pending tree must carry an id")
	}
	if again, _ := ag.takePendingUITree(); again != nil {
		t.Fatal("UC-07: the tree must be consumed exactly once")
	}

	// A tree handed over as a JSON string (providers without a native object
	// type) is accepted as well.
	strArgs := map[string]interface{}{
		"meta": map[string]interface{}{"intent": "show"},
		"tree": `{"type":"progress","props":{"value":3,"max":4}}`,
	}
	if _, err := ag.renderUITool(context.Background(), strArgs); err != nil {
		t.Fatalf("UC-07: JSON-string tree rejected: %v", err)
	}
	if root, _ := ag.takePendingUITree(); root == nil || root.Type != UICompProgress {
		t.Fatalf("UC-07: JSON-string tree not parked: %+v", root)
	}

	// An invalid tree is rejected and parks nothing.
	badArgs := map[string]interface{}{
		"meta": map[string]interface{}{"intent": "show"},
		"tree": map[string]interface{}{"type": "sparkline"},
	}
	if _, err := ag.renderUITool(context.Background(), badArgs); err == nil {
		t.Fatal("UC-07: invalid tree must be rejected")
	}
	if root, _ := ag.takePendingUITree(); root != nil {
		t.Fatal("UC-07: a rejected call must not park a tree")
	}
}

// UC-08: the ui_render event carries id + tree and rides ChannelSystem so the
// show-* switches cannot filter it away from the Web UI.
func TestUIUC08UIRenderEvent(t *testing.T) {
	tree := `{"type":"card","children":[{"type":"kv"}]}`
	ev := UIRenderEvent("ui-7", tree)

	if ev.Type != EventUIRender {
		t.Fatalf("UC-08: type = %q, want %q", ev.Type, EventUIRender)
	}
	if ev.Chan != ChannelSystem {
		t.Fatalf("UC-08: channel = %q, want %q (must not be gated by show-* switches)", ev.Chan, ChannelSystem)
	}
	if ev.Level != LevelInfo {
		t.Fatalf("UC-08: level = %q, want %q", ev.Level, LevelInfo)
	}
	if got := ev.Meta[MetaKeyUIID]; got != "ui-7" {
		t.Fatalf("UC-08: meta[%s] = %q, want ui-7", MetaKeyUIID, got)
	}
	if got := ev.Meta[MetaKeyUITree]; got != tree {
		t.Fatalf("UC-08: meta[%s] = %q, want the tree verbatim", MetaKeyUITree, got)
	}
}

// UC-09: the ui_update event carries id + patch on the same channel.
func TestUIUC09UIUpdateEvent(t *testing.T) {
	patch := `{"type":"progress","id":"ui-7","props":{"value":80,"max":100}}`
	ev := UIUpdateEvent("ui-7", patch)

	if ev.Type != EventUIUpdate {
		t.Fatalf("UC-09: type = %q, want %q", ev.Type, EventUIUpdate)
	}
	if ev.Chan != ChannelSystem {
		t.Fatalf("UC-09: channel = %q, want %q", ev.Chan, ChannelSystem)
	}
	if got := ev.Meta[MetaKeyUIID]; got != "ui-7" {
		t.Fatalf("UC-09: meta[%s] = %q, want ui-7", MetaKeyUIID, got)
	}
	if got := ev.Meta[MetaKeyUIPatch]; got != patch {
		t.Fatalf("UC-09: meta[%s] = %q, want the patch verbatim", MetaKeyUIPatch, got)
	}
	if _, ok := ev.Meta[MetaKeyUITree]; ok {
		t.Fatalf("UC-09: update event must not carry %s", MetaKeyUITree)
	}
}

// UC-10: render_ui is auto-approved by default and is advertised to the LLM
// only while the ui_enabled switch is on.
func TestUIUC10RenderUIPreApproved(t *testing.T) {
	if got := DefaultToolModes()["render_ui"]; got != "auto" {
		t.Fatalf("UC-10: default mode = %q, want auto (read-only rendering needs no confirmation)", got)
	}

	on := newUITestAgent(true)
	if !hasTool(on.buildToolsInternal(), "render_ui") {
		t.Fatal("UC-10: render_ui must be advertised when ui_enabled is on")
	}

	off := newUITestAgent(false)
	if hasTool(off.buildToolsInternal(), "render_ui") {
		t.Fatal("UC-10: render_ui must not be advertised when ui_enabled is off")
	}
}
