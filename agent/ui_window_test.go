// Package agent - window mode tests (FEATURE-524, UC-50 ~ UC-55).
//
// The window itself is painted by the Web UI; these tests pin the protocol the
// frontend depends on: the ui_window tool parks exactly one open/close action,
// render_ui addresses the window through its target, the blocking flag survives
// the tree round trip, and an in-turn action is injected as a dynamic event.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// UC-50: ui_window(open) parks the window request and returns a receipt naming
// the title; the parked request is consumed exactly once.
func TestUIWindowUC50OpenParks(t *testing.T) {
	ag := &Agent{}
	receipt, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": "open",
		"title":  "用户信息采集",
	})
	if err != nil {
		t.Fatalf("UC-50: ui_window(open) failed: %v", err)
	}
	if !strings.Contains(receipt, "用户信息采集") {
		t.Fatalf("UC-50: receipt %q must name the window title", receipt)
	}

	req := ag.takePendingUIWindow()
	if req == nil {
		t.Fatal("UC-50: ui_window(open) must park a window request")
	}
	if req.Action != UIWindowOpen || req.Title != "用户信息采集" {
		t.Fatalf("UC-50: parked request = %+v, want open/用户信息采集", req)
	}
	if again := ag.takePendingUIWindow(); again != nil {
		t.Fatal("UC-50: the request must be consumed exactly once")
	}

	// A call without a title still opens a usable window (default title).
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "open"}); err != nil {
		t.Fatalf("UC-50: title must be optional: %v", err)
	}
	if req := ag.takePendingUIWindow(); req == nil || req.Title == "" {
		t.Fatalf("UC-50: a title-less open must fall back to the default title, got %+v", req)
	}

	// Action names are validated against the whitelist; a bad one parks nothing.
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "hide"}); err == nil {
		t.Fatal("UC-50: an unknown action must be refused")
	}
	if req := ag.takePendingUIWindow(); req != nil {
		t.Fatal("UC-50: a refused call must not park a request")
	}
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{}); err == nil {
		t.Fatal("UC-50: a missing action must be refused")
	}
}

// UC-51: render_ui(target="window") parks the window surface, and the emitted
// event carries it — while a stream render keeps its original wire shape.
func TestUIWindowUC51RenderTarget(t *testing.T) {
	ag := &Agent{}
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": "window",
		"tree":   map[string]interface{}{"type": UICompProgress, "props": map[string]interface{}{"value": 30}},
	}); err != nil {
		t.Fatalf("UC-51: target=window rejected: %v", err)
	}
	root, id := ag.takePendingUITree()
	if root == nil || id == "" {
		t.Fatalf("UC-51: the tree must be parked, got %+v/%q", root, id)
	}
	if got := ag.takePendingUITarget(); got != UITargetWindow {
		t.Fatalf("UC-51: parked target = %q, want %q", got, UITargetWindow)
	}

	treeJSON, err := MarshalUITree(root)
	if err != nil {
		t.Fatalf("UC-51: marshal: %v", err)
	}
	ev := UIRenderEventTo(id, treeJSON, UITargetWindow)
	if ev.Type != EventUIRender || ev.Chan != ChannelSystem {
		t.Fatalf("UC-51: window render event = %+v, want ui_render on ChannelSystem", ev)
	}
	if got := ev.Meta[MetaKeyUITarget]; got != UITargetWindow {
		t.Fatalf("UC-51: meta[%s] = %q, want %q", MetaKeyUITarget, got, UITargetWindow)
	}

	// The default target is the chat stream and must not add the Meta key, so
	// existing UC-08 expectations keep holding.
	if got := ag.takePendingUITarget(); got != "" {
		t.Fatalf("UC-51: the target must be consumed with the tree, got %q", got)
	}
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"tree": map[string]interface{}{"type": UICompCard},
	}); err != nil {
		t.Fatalf("UC-51: default render rejected: %v", err)
	}
	streamRoot, streamID := ag.takePendingUITree()
	streamJSON, err := MarshalUITree(streamRoot)
	if err != nil {
		t.Fatalf("UC-51: marshal: %v", err)
	}
	if got := ag.takePendingUITarget(); got != UITargetStream {
		t.Fatalf("UC-51: default target = %q, want %q", got, UITargetStream)
	}
	if ev := UIRenderEventTo(streamID, streamJSON, UITargetStream); ev.Meta[MetaKeyUITarget] != "" {
		t.Fatalf("UC-51: a stream render must not carry %s", MetaKeyUITarget)
	}

	// An unsupported target is refused and parks nothing.
	if _, err := ag.renderUITool(context.Background(), map[string]interface{}{
		"target": "popup",
		"tree":   map[string]interface{}{"type": UICompCard},
	}); err == nil {
		t.Fatal("UC-51: an unsupported target must be refused")
	}
	if root, _ := ag.takePendingUITree(); root != nil {
		t.Fatal("UC-51: a refused target must not park a tree")
	}
}

// UC-52: the blocking flag survives the tree round trip so the frontend can
// echo it, and an in-turn action is injected as a dynamic event.
func TestUIWindowUC52BlockingAndInjection(t *testing.T) {
	tree := `{"type":"form","actions":[{"on":"submit","id":"send","blocking":true}]}`
	root, err := ParseUITree(tree)
	if err != nil {
		t.Fatalf("UC-52: blocking action rejected: %v", err)
	}
	if len(root.Actions) != 1 || !root.Actions[0].Blocking {
		t.Fatalf("UC-52: blocking flag lost: %+v", root.Actions)
	}
	blob, err := MarshalUITree(root)
	if err != nil {
		t.Fatalf("UC-52: marshal: %v", err)
	}
	if !strings.Contains(blob, `"blocking":true`) {
		t.Fatalf("UC-52: blocking must reach the frontend, got %s", blob)
	}
	// A non-blocking action keeps the wire shape free of the flag.
	plain, err := ParseUITree(`{"type":"form","actions":[{"on":"submit","id":"send"}]}`)
	if err != nil {
		t.Fatalf("UC-52: plain action rejected: %v", err)
	}
	plainBlob, err := MarshalUITree(plain)
	if err != nil {
		t.Fatalf("UC-52: marshal: %v", err)
	}
	if strings.Contains(plainBlob, "blocking") {
		t.Fatalf("UC-52: a non-blocking action must not carry the flag, got %s", plainBlob)
	}

	// The running turn is fed through the dynamic-event queue, which is what
	// the session uses when the agent is busy (Q3-A).
	ag := &Agent{}
	payload := json.RawMessage(`{"name":"Ada"}`)
	ag.AddDynamicEvent(DynamicUIAction, UIActionMessage("ui-7", "send", payload))
	block := ag.consumeDynamicEvents(false)
	if !strings.Contains(block, "<ui_action>") {
		t.Fatalf("UC-52: injected action missing from %q", block)
	}
	for _, want := range []string{"ui-7", "send", "Ada"} {
		if !strings.Contains(block, want) {
			t.Fatalf("UC-52: injected action %q must carry %q", block, want)
		}
	}
	if strings.Contains(block, "<user_message>") {
		t.Fatalf("UC-52: a ui_action must not be rendered as a user message: %q", block)
	}
}

// UC-53: a blocking action rides the existing wait channel, and the ui_window
// tool never parks a wait of its own (only render_ui(waiting=true) does).
func TestUIWindowUC53BlockingUsesWaitChannel(t *testing.T) {
	ag := &Agent{}
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "open"}); err != nil {
		t.Fatalf("UC-53: ui_window(open) failed: %v", err)
	}
	if waitParked(ag) {
		t.Fatal("UC-53: ui_window must not park a wait")
	}
	_ = ag.takePendingUIWindow()

	ch := ag.beginUIWait()
	if ch == nil {
		t.Fatal("UC-53: beginUIWait must register the first wait")
	}
	defer ag.endUIWait(ch)
	if !ag.SubmitUIAction("ui-9", "send", json.RawMessage(`{"ok":true}`)) {
		t.Fatal("UC-53: a blocking action must be delivered to the parked wait")
	}
	got := <-ch
	for _, want := range []string{"ui-9", "send", "ok"} {
		if !strings.Contains(got, want) {
			t.Fatalf("UC-53: wait result %q must carry %q", got, want)
		}
	}
}

// UC-54: the close event carries the action and no title, while the open event
// carries the title; both ride ChannelSystem so the Web UI always sees them.
func TestUIWindowUC54LifecycleEvents(t *testing.T) {
	open := UIWindowEvent(UIWindowOpen, "任务窗口", UIWindowSizeLarge)
	if open.Type != EventUIWindow || open.Chan != ChannelSystem || open.Level != LevelInfo {
		t.Fatalf("UC-54: open event = %+v, want ui_window on ChannelSystem", open)
	}
	if got := open.Meta[MetaKeyUIWindowAction]; got != UIWindowOpen {
		t.Fatalf("UC-54: meta[%s] = %q, want %q", MetaKeyUIWindowAction, got, UIWindowOpen)
	}
	if got := open.Meta[MetaKeyUIWindowTitle]; got != "任务窗口" {
		t.Fatalf("UC-54: meta[%s] = %q, want the title", MetaKeyUIWindowTitle, got)
	}
	if got := open.Meta[MetaKeyUIWindowSize]; got != UIWindowSizeLarge {
		t.Fatalf("UC-59: meta[%s] = %q, want %q", MetaKeyUIWindowSize, got, UIWindowSizeLarge)
	}

	closeEv := UIWindowEvent(UIWindowClose, "", "")
	if got := closeEv.Meta[MetaKeyUIWindowAction]; got != UIWindowClose {
		t.Fatalf("UC-54: meta[%s] = %q, want %q", MetaKeyUIWindowAction, got, UIWindowClose)
	}
	if _, ok := closeEv.Meta[MetaKeyUIWindowTitle]; ok {
		t.Fatal("UC-54: a close event must not carry a title")
	}
	if _, ok := closeEv.Meta[MetaKeyUIWindowSize]; ok {
		t.Fatal("UC-59: a close event must not carry a size preset")
	}

	// Closing through the tool parks the same action the stream loop emits.
	ag := &Agent{}
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "close", "title": "ignored"}); err != nil {
		t.Fatalf("UC-54: ui_window(close) failed: %v", err)
	}
	req := ag.takePendingUIWindow()
	if req == nil || req.Action != UIWindowClose || req.Title != "" {
		t.Fatalf("UC-54: parked close = %+v, want close without a title", req)
	}
}

// UC-55: reopening reuses the single window — the tool always parks one open
// request (with the new title) and never a second window, and an over-long
// title is refused instead of being truncated silently.
func TestUIWindowUC55SingleWindowAndTitleLimit(t *testing.T) {
	ag := &Agent{}
	for _, title := range []string{"第一次", "第二次"} {
		if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
			"action": "open",
			"title":  title,
		}); err != nil {
			t.Fatalf("UC-55: open(%q) failed: %v", title, err)
		}
		req := ag.takePendingUIWindow()
		if req == nil || req.Title != title {
			t.Fatalf("UC-55: parked request = %+v, want one open with title %q", req, title)
		}
	}

	long := strings.Repeat("字", UIMaxWindowTitle+1)
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "open", "title": long}); err == nil {
		t.Fatal("UC-55: an over-long title must be refused")
	}
	if req := ag.takePendingUIWindow(); req != nil {
		t.Fatal("UC-55: a refused title must not park a request")
	}
}

// UC-57/UC-58: the size preset is validated by shape only — every known preset
// (and an omitted or empty value, folded to auto) is accepted, parked and named
// in the receipt, while an unknown one is refused without parking anything.
func TestUIWindowUC5758SizePresets(t *testing.T) {
	ag := &Agent{}
	cases := []struct {
		arg  interface{}
		want string
	}{
		{nil, UIWindowSizeAuto},
		{"", UIWindowSizeAuto},
		{UIWindowSizeAuto, UIWindowSizeAuto},
		{"small", UIWindowSizeSmall},
		{" Medium ", UIWindowSizeMedium},
		{"LARGE", UIWindowSizeLarge},
	}
	for _, tc := range cases {
		args := map[string]interface{}{"action": "open"}
		if tc.arg != nil {
			args["size"] = tc.arg
		}
		receipt, err := ag.uiWindowTool(context.Background(), args)
		if err != nil {
			t.Fatalf("UC-57: open(size=%v) failed: %v", tc.arg, err)
		}
		if !strings.Contains(receipt, tc.want) {
			t.Fatalf("UC-57: receipt %q must name the preset %q", receipt, tc.want)
		}
		req := ag.takePendingUIWindow()
		if req == nil || req.Size != tc.want {
			t.Fatalf("UC-57: parked request = %+v, want size %q", req, tc.want)
		}
	}

	for _, bad := range []string{"huge", "LARGE!", "1000", "medium x"} {
		if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{"action": "open", "size": bad}); err == nil {
			t.Fatalf("UC-58: size %q must be refused", bad)
		}
		if req := ag.takePendingUIWindow(); req != nil {
			t.Fatalf("UC-58: a refused size must not park a request, got %+v", req)
		}
	}
}

// UC-59: only an open request carries a size preset. The event exposes it as
// Meta[MetaKeyUIWindowSize]; close clears it exactly like the title.
func TestUIWindowUC59SizeOnEvents(t *testing.T) {
	ev := UIWindowEvent(UIWindowOpen, "面板", UIWindowSizeSmall)
	if got := ev.Meta[MetaKeyUIWindowSize]; got != UIWindowSizeSmall {
		t.Fatalf("UC-59: meta[%s] = %q, want %q", MetaKeyUIWindowSize, got, UIWindowSizeSmall)
	}
	if _, ok := UIWindowEvent(UIWindowClose, "", "").Meta[MetaKeyUIWindowSize]; ok {
		t.Fatal("UC-59: a close event must not carry a size preset")
	}

	ag := &Agent{}
	if _, err := ag.uiWindowTool(context.Background(), map[string]interface{}{
		"action": "close",
		"size":   "large",
	}); err != nil {
		t.Fatalf("UC-59: close failed: %v", err)
	}
	if req := ag.takePendingUIWindow(); req == nil || req.Size != "" || req.Title != "" {
		t.Fatalf("UC-59: parked close = %+v, want no size and no title", req)
	}
}
