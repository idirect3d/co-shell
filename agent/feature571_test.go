// Package agent - multi-window registry tests (FEATURE-571).
//
// The panel itself is painted by the Web UI; these tests pin the agent side of
// the protocol: target=panel registers a window on the panel surface, window
// names and surfaces are validated, <ui_windows> reports what is open, and the
// frontend's ui_window_state mirror keeps the registry in step with the screen.
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestFeature571PanelRenderRegistersWindow covers the panel target: the window
// is registered on the panel surface, the tree is parked on it, and an unnamed
// render keeps addressing the active window instead of opening a new one.
func TestFeature571PanelRenderRegistersWindow(t *testing.T) {
	ag := &Agent{}
	receipt, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": UITargetPanel,
		"window": "p1",
		"tree":   map[string]interface{}{"type": UICompCard, "id": "ui-9"},
	})
	if err != nil {
		t.Fatalf("FEATURE-571: target=panel rejected: %v", err)
	}
	if receipt == "" {
		t.Fatal("FEATURE-571: a panel render must return a receipt")
	}
	if got := ag.takePendingUITarget(); got != UITargetPanel {
		t.Fatalf("FEATURE-571: parked target = %q, want %q", got, UITargetPanel)
	}
	if got := ag.takePendingUIWindowName(); got != "p1" {
		t.Fatalf("FEATURE-571: parked window = %q, want p1", got)
	}
	if root, _ := ag.takePendingUITree(); root == nil {
		t.Fatal("FEATURE-571: the tree must be parked")
	}

	windows := ag.UIWindowsSnapshot()
	if len(windows) != 1 {
		t.Fatalf("FEATURE-571: registry holds %d windows, want 1", len(windows))
	}
	w := windows[0]
	if w.Name != "p1" || w.Surface != UIWindowSurfacePanel || !w.Active {
		t.Fatalf("FEATURE-571: window = %+v, want p1/panel/active", w)
	}
	// The summary is type(tree id); the id is assigned by the backend, so only
	// the shape is pinned here.
	if !strings.HasPrefix(w.Summary, "card(ui-") || !strings.HasSuffix(w.Summary, ")") {
		t.Fatalf("FEATURE-571: summary = %q, want card(<tree id>)", w.Summary)
	}

	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": UITargetPanel,
		"tree":   map[string]interface{}{"type": UICompKV},
	}); err != nil {
		t.Fatalf("FEATURE-571: unnamed panel render rejected: %v", err)
	}
	if got := ag.takePendingUIWindowName(); got != "p1" {
		t.Fatalf("FEATURE-571: unnamed render addressed %q, want the active p1", got)
	}
	_, _ = ag.takePendingUITree()
	if len(ag.UIWindowsSnapshot()) != 1 {
		t.Fatal("FEATURE-571: an unnamed render must reuse the active window")
	}

	// target=window floats on the float surface, and the two coexist.
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": UITargetWindow,
		"window": "f1",
		"tree":   map[string]interface{}{"type": UICompProgress},
	}); err != nil {
		t.Fatalf("FEATURE-571: target=window rejected: %v", err)
	}
	_, _ = ag.takePendingUITree()
	if got := ag.uiWindowSurface("f1"); got != UIWindowSurfaceFloat {
		t.Fatalf("FEATURE-571: float surface = %q, want %q", got, UIWindowSurfaceFloat)
	}
	if len(ag.UIWindowsSnapshot()) != 2 {
		t.Fatal("FEATURE-571: a panel and a float window must coexist")
	}
}

// TestFeature571NameAndSurfaceValidation refuses anything that could break the
// window list or a tab label, and keeps the documented defaults.
func TestFeature571NameAndSurfaceValidation(t *testing.T) {
	ag := &Agent{}
	bad := []string{"a/b", "a\"b", "窗口:1", "a\nb", strings.Repeat("x", UIMaxWindowName+1)}
	for _, name := range bad {
		if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
			"target": UITargetPanel,
			"window": name,
			"tree":   map[string]interface{}{"type": UICompCard},
		}); err == nil {
			t.Fatalf("FEATURE-571: window name %q must be refused", name)
		}
		if req := ag.takePendingUIWindow(); req != nil {
			t.Fatalf("FEATURE-571: refused name %q must not park anything", name)
		}
	}
	good := []string{"面板 1", "user-info", "a_b.c", "任务"}
	for _, name := range good {
		if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
			"target": UITargetPanel,
			"window": name,
			"tree":   map[string]interface{}{"type": UICompCard},
		}); err != nil {
			t.Fatalf("FEATURE-571: window name %q must be accepted: %v", name, err)
		}
		_, _ = ag.takePendingUITree()
		_ = ag.takePendingUIWindowName()
	}

	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action":  UIWindowOpen,
		"window":  "s1",
		"surface": "sheet",
	}); err == nil {
		t.Fatal("FEATURE-571: ui_window must refuse an unknown surface")
	}

	// activate names an open window and parks an action that carries it.
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": UIWindowOpen,
		"window": "x",
		"title":  "面板 X",
	}); err != nil {
		t.Fatalf("FEATURE-571: open rejected: %v", err)
	}
	_ = ag.takePendingUIWindow()
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": UIWindowActivate,
		"window": "x",
	}); err != nil {
		t.Fatalf("FEATURE-571: activate rejected: %v", err)
	}
	req := ag.takePendingUIWindow()
	if req == nil || req.Action != UIWindowActivate || req.Name != "x" {
		t.Fatalf("FEATURE-571: activate parked %+v, want x/activate", req)
	}

	// close drops the window at once and stays idempotent, so a window the user
	// already dismissed never turns a defensive close into an error.
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": UIWindowClose,
		"window": "x",
	}); err != nil {
		t.Fatalf("FEATURE-571: close rejected: %v", err)
	}
	if ag.uiWindowSurface("x") != "" {
		t.Fatal("FEATURE-571: close must drop the window from the registry")
	}
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": UIWindowClose,
		"window": "x",
	}); err != nil {
		t.Fatalf("FEATURE-571: a repeated close must stay harmless: %v", err)
	}
}

// TestFeature571WindowCap bounds how many windows one session may open while
// keeping a re-open of an existing window possible at the cap.
func TestFeature571WindowCap(t *testing.T) {
	ag := &Agent{}
	for i := 0; i < UIMaxWindows; i++ {
		if err := ag.ensureUIWindow(string(rune('a'+i)), "", UIWindowSurfaceFloat); err != nil {
			t.Fatalf("FEATURE-571: window %d refused: %v", i, err)
		}
	}
	if err := ag.ensureUIWindow("overflow", "", UIWindowSurfaceFloat); err == nil {
		t.Fatal("FEATURE-571: the window cap must be enforced")
	}
	if n := len(ag.UIWindowsSnapshot()); n != UIMaxWindows {
		t.Fatalf("FEATURE-571: cap produced %d windows, want %d", n, UIMaxWindows)
	}
	if err := ag.ensureUIWindow("a", "renamed", ""); err != nil {
		t.Fatalf("FEATURE-571: reopening at cap must work: %v", err)
	}
	if got := ag.uiWindowTitle("a"); got != "renamed" {
		t.Fatalf("FEATURE-571: title = %q, want renamed", got)
	}
}

// TestFeature571UIWindowsBlock renders the block the LLM reads, including the
// "none" case that must stay obvious.
func TestFeature571UIWindowsBlock(t *testing.T) {
	ag := &Agent{}
	if got := ag.buildUIWindows(); !strings.Contains(got, "<ui_windows>none</ui_windows>") {
		t.Fatalf("FEATURE-571: empty registry block = %q, want none", got)
	}
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": UITargetPanel,
		"window": "结果",
		"tree":   map[string]interface{}{"type": UICompTable, "id": "ui-3"},
	}); err != nil {
		t.Fatalf("FEATURE-571: render: %v", err)
	}
	_, _ = ag.takePendingUITree()
	_ = ag.takePendingUIWindowName()
	block := ag.buildUIWindows()
	for _, want := range []string{"<ui_windows>", "结果", UIWindowSurfacePanel, "active", "table(ui-", "</ui_windows>"} {
		if !strings.Contains(block, want) {
			t.Fatalf("FEATURE-571: block %q must contain %q", block, want)
		}
	}
	if !strings.Contains(ag.buildOpenedResources(), "<ui_windows>") {
		t.Fatal("FEATURE-571: <opened_resources> must carry the window list")
	}
}

// TestFeature571SyncFromFrontend mirrors the frontend: it closes what the user
// closed, adopts title/surface/active changes and restores a window list after
// a reload — while never growing past the cap.
func TestFeature571SyncFromFrontend(t *testing.T) {
	ag := &Agent{}
	raw, err := json.Marshal([]UIWindowInfo{
		{Name: "p1", Title: "面板", Surface: UIWindowSurfacePanel, Active: true, Summary: "card(ui-1)"},
		{Name: "f1", Title: "浮窗", Surface: UIWindowSurfaceFloat},
	})
	if err != nil {
		t.Fatalf("FEATURE-571: marshal: %v", err)
	}
	ag.SyncUIWindowsPayload(raw, nil)
	windows := ag.UIWindowsSnapshot()
	if len(windows) != 2 {
		t.Fatalf("FEATURE-571: sync adopted %d windows, want 2", len(windows))
	}
	if windows[0].Name != "p1" || !windows[0].Active || windows[0].Summary != "card(ui-1)" {
		t.Fatalf("FEATURE-571: restored window = %+v", windows[0])
	}
	if got := ag.activeUIWindowName(); got != "p1" {
		t.Fatalf("FEATURE-571: active = %q, want p1", got)
	}

	// The user's ✕ (or a tab switch) arrives as a closed name plus the new list.
	flip, err := json.Marshal([]UIWindowInfo{
		{Name: "f1", Title: "浮窗", Surface: UIWindowSurfaceFloat, Active: true},
	})
	if err != nil {
		t.Fatalf("FEATURE-571: marshal: %v", err)
	}
	ag.SyncUIWindowsPayload(flip, []string{"p1"})
	windows = ag.UIWindowsSnapshot()
	if len(windows) != 1 || windows[0].Name != "f1" {
		t.Fatalf("FEATURE-571: after close = %+v, want only f1", windows)
	}
	if got := ag.activeUIWindowName(); got != "f1" {
		t.Fatalf("FEATURE-571: active after close = %q, want f1", got)
	}

	// A malformed payload is ignored instead of failing the turn.
	ag.SyncUIWindowsPayload(json.RawMessage("{"), []string{""})
	if len(ag.UIWindowsSnapshot()) != 1 {
		t.Fatal("FEATURE-571: a malformed payload must be ignored")
	}

	// The snapshot is what a reconnect sends back: the cap still holds.
	big := make([]UIWindowInfo, 0, UIMaxWindows+4)
	for i := 0; i < UIMaxWindows+4; i++ {
		big = append(big, UIWindowInfo{Name: string(rune('a'+i)) + "-extra"})
	}
	more, err := json.Marshal(big)
	if err != nil {
		t.Fatalf("FEATURE-571: marshal: %v", err)
	}
	ag.SyncUIWindowsPayload(more, nil)
	if n := len(ag.UIWindowsSnapshot()); n > UIMaxWindows {
		t.Fatalf("FEATURE-571: sync produced %d windows, cap is %d", n, UIMaxWindows)
	}
}
