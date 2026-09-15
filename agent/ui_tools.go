// Package agent - the render_ui tool: LLM-driven rich component output
// (FEATURE-524).
//
// render_ui lets the LLM answer with a component tree (cards, tables, charts,
// steps, ...) instead of a wall of text. The tool itself only validates the
// tree and parks it on the agent; the stream loop (the only owner of the
// StreamCallback) turns it into a ui_render event that the Web UI paints.
// Terminals and other frontends ignore the event and keep the plain-text
// reply, so no frontend breaks when the LLM uses this tool.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// render_ui targets (FEATURE-524 window mode): where the tree is painted.
const (
	// UITargetStream is the default surface: a block in the chat stream.
	UITargetStream = "stream"
	// UITargetWindow paints into the floating window opened by ui_window.
	UITargetWindow = "window"
)

// ui_window actions.
const (
	// UIWindowOpen shows the floating window (or reuses the open one).
	UIWindowOpen = "open"
	// UIWindowClose dismisses it.
	UIWindowClose = "close"
)

// UIMaxWindowTitle bounds the window title in runes. A title is a UI label, not
// a payload: an over-long one is refused so it cannot flood the event or the
// title bar.
const UIMaxWindowTitle = 60

// uiIDSeq numbers the component trees rendered by this process. Ids only need
// to be unique within one page (they address a node in the Web UI DOM), so a
// process-wide counter is enough and needs no coordination across sessions.
var uiIDSeq uint64

// uiEnabled reports whether the render_ui tool is exposed to the LLM. The
// switch defaults to on: a nil config (tests, embedders) counts as enabled.
func (a *Agent) uiEnabled() bool {
	return a.cfg == nil || a.cfg.UIEnabled
}

// buildRenderUITool declares the render_ui tool. The full component catalogue
// lives in the localized usage section (KeyUIToolUsageRenderUI), which is both
// the XML-mode system prompt section and the OpenAI-mode tool description, so
// the two modes cannot drift apart.
func (a *Agent) buildRenderUITool() llm.Tool {
	return llm.Tool{
		Name:        "render_ui",
		Description: i18n.T(i18n.KeyUIToolUsageRenderUI),
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"tree": map[string]interface{}{
					"type":        "object",
					"description": "Component tree root node {type, id?, props?, children?, actions?}. See the render_ui section of the system prompt for the component catalogue and per-component props.",
				},
				"update": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamUpdate),
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamTarget),
					"enum":        []string{UITargetStream, UITargetWindow},
				},
				"waiting": map[string]interface{}{
					"type":        "boolean",
					"description": "false (default): return immediately; a later user action starts a new turn. true: keep the tool call open until the user acts, and return that action as this tool's result.",
				},
			},
			"required": []string{"meta", "tree"},
		},
		Callback: a.renderUITool,
	}
}

// renderUITool validates the component tree, parks it for the stream loop and
// returns a one-line receipt. The tree is deliberately NOT echoed back: the
// LLM already wrote it, so returning it would only inflate the context.
func (a *Agent) renderUITool(ctx context.Context, args map[string]interface{}) (string, error) {
	raw, ok := args["tree"]
	if !ok || raw == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyUIErrTreeEmpty))
	}
	treeJSON, err := uiTreeArgJSON(raw)
	if err != nil {
		return "", err
	}
	root, err := ParseUITree(treeJSON)
	if err != nil {
		return "", err
	}

	// FEATURE-524 window mode: target selects the surface the tree is painted
	// on; an unsupported value is refused before anything is parked.
	surface, err := uiRenderTarget(args)
	if err != nil {
		return "", err
	}

	if target := uiUpdateTarget(args); target != "" {
		a.mu.Lock()
		a.pendingUIUpdateID = target
		a.pendingUIUpdateNode = root
		a.pendingUITarget = surface
		a.mu.Unlock()
		return i18n.TF(i18n.KeyUIUpdateSummary, target, CountUINodes(root)), nil
	}

	id := fmt.Sprintf("ui-%d", atomic.AddUint64(&uiIDSeq, 1))
	a.mu.Lock()
	a.pendingUITree = root
	a.pendingUIID = id
	a.pendingUITarget = surface
	a.mu.Unlock()

	// waiting=true parks the call until the user acts on a rendered component:
	// the action then becomes the tool result, so the agent continues in the
	// same turn with the structured values the user chose (FEATURE-524, UC-32).
	// A second concurrent wait is refused by beginUIWait and degrades to the
	// normal receipt rather than racing for the same user action.
	if waiting, _ := args["waiting"].(bool); waiting {
		if ch := a.beginUIWait(); ch != nil {
			defer a.endUIWait(ch)
			return a.waitUIAction(ctx, ch), nil
		}
	}
	return UISummary(root, id), nil
}

// takePendingUITree returns and clears the tree parked by the most recent
// render_ui call. It gives the stream loop exclusive ownership of the tree so
// the same render is emitted exactly once.
func (a *Agent) takePendingUITree() (*UINode, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	root, id := a.pendingUITree, a.pendingUIID
	a.pendingUITree, a.pendingUIID = nil, ""
	return root, id
}

// takePendingUIUpdate returns and clears the in-place update parked by the most
// recent render_ui call that carried an update target. It is the sibling of
// takePendingUITree: a call parks exactly one of the two, never both, so the
// stream loop emits exactly one event per call.
func (a *Agent) takePendingUIUpdate() (string, *UINode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	target, node := a.pendingUIUpdateID, a.pendingUIUpdateNode
	a.pendingUIUpdateID, a.pendingUIUpdateNode = "", nil
	return target, node
}

// uiUpdateTarget reads the optional update target of a render_ui call. An empty
// result means "render a new tree" (the default).
func uiUpdateTarget(args map[string]interface{}) string {
	s, ok := args["update"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// UIActionMessage renders a component interaction as the user message that
// starts the next agent turn (FEATURE-524). The payload is inlined as JSON so
// the LLM receives the exact structured values the user provided, without the
// frontend having to invent prose for them.
func UIActionMessage(uiID, actionID string, payload json.RawMessage) string {
	p := strings.TrimSpace(string(payload))
	if p == "" || p == "null" {
		p = "{}"
	}
	return i18n.TF(i18n.KeyUIUserAction, uiID, actionID, p)
}

// takePendingUITarget returns and clears the surface the parked tree/update
// belongs to: UITargetStream (the chat stream) or UITargetWindow. The stream
// loop reads it right after consuming the parked tree.
func (a *Agent) takePendingUITarget() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	target := a.pendingUITarget
	a.pendingUITarget = ""
	return target
}

// uiRenderTarget reads the optional target of a render_ui call. An empty value
// means the default chat stream; an unsupported value is an error so the LLM
// gets told instead of silently rendering into the wrong surface.
func uiRenderTarget(args map[string]interface{}) (string, error) {
	raw, ok := args["target"]
	if !ok || raw == nil {
		return UITargetStream, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTarget, fmt.Sprint(raw)))
	}
	switch target := strings.ToLower(strings.TrimSpace(s)); target {
	case "", UITargetStream:
		return UITargetStream, nil
	case UITargetWindow:
		return UITargetWindow, nil
	default:
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTarget, s))
	}
}

// uiWindowRequest is one parked ui_window call: the action to perform and the
// window title (empty for close).
type uiWindowRequest struct {
	Action string
	Title  string
}

// buildUIWindowTool declares the ui_window tool (FEATURE-524 window mode). It
// only opens/closes the floating window: the content is written by render_ui
// with target="window", so the window stays a surface the LLM can keep pushing
// into while it works, instead of a one-shot payload.
func (a *Agent) buildUIWindowTool() llm.Tool {
	return llm.Tool{
		Name:        "ui_window",
		Description: i18n.T(i18n.KeyUIToolUsageUIWindow),
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"description": "open: show the floating window; close: dismiss it.",
					"enum":        []string{UIWindowOpen, UIWindowClose},
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowTitle),
				},
			},
			"required": []string{"meta", "action"},
		},
		Callback: a.uiWindowTool,
	}
}

// uiWindowTool validates a ui_window call, parks it for the stream loop (the
// only owner of the StreamCallback) and returns a one-line receipt.
func (a *Agent) uiWindowTool(ctx context.Context, args map[string]interface{}) (string, error) {
	rawAction := strings.TrimSpace(uiStringArg(args, "action"))
	action := strings.ToLower(rawAction)
	if action != UIWindowOpen && action != UIWindowClose {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowAction, rawAction))
	}

	title := strings.TrimSpace(uiStringArg(args, "title"))
	if n := len([]rune(title)); n > UIMaxWindowTitle {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowTitle, UIMaxWindowTitle))
	}
	switch action {
	case UIWindowClose:
		// The title only belongs to an open window.
		title = ""
	default:
		if title == "" {
			title = i18n.T(i18n.KeyUIWindowDefaultTitle)
		}
	}

	a.mu.Lock()
	a.pendingUIWindow = &uiWindowRequest{Action: action, Title: title}
	a.mu.Unlock()

	if action == UIWindowClose {
		return i18n.T(i18n.KeyUIWindowCloseSummary), nil
	}
	return i18n.TF(i18n.KeyUIWindowOpenSummary, title), nil
}

// takePendingUIWindow returns and clears the window action parked by the most
// recent ui_window call, so the stream loop emits it exactly once.
func (a *Agent) takePendingUIWindow() *uiWindowRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	req := a.pendingUIWindow
	a.pendingUIWindow = nil
	return req
}

// uiStringArg reads a string argument, tolerating a missing or non-string value.
func uiStringArg(args map[string]interface{}, name string) string {
	s, _ := args[name].(string)
	return s
}

// uiTreeArgJSON normalizes the tree argument to JSON text. Providers without a
// native object type may hand the tree over as an already-encoded JSON string,
// so both shapes are accepted.
func uiTreeArgJSON(raw interface{}) (string, error) {
	if s, ok := raw.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTreeDecode, err.Error()))
	}
	return string(b), nil
}
