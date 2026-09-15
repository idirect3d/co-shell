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
	"sync/atomic"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

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

	id := fmt.Sprintf("ui-%d", atomic.AddUint64(&uiIDSeq, 1))
	a.mu.Lock()
	a.pendingUITree = root
	a.pendingUIID = id
	a.mu.Unlock()

	// Stage 3 (FEATURE-524) adds the waiting=true branch here: the tool call
	// parks on a channel and returns the user's action instead of a receipt.
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
