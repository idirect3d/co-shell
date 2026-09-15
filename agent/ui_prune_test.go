// Package agent - unit tests for component-tree pruning (FEATURE-524, Stage 4).
//
// The pruning rewrites render_ui tool-call arguments on the copy handed to the
// provider, so these tests cover: the switch itself, both history shapes (tool
// calls and XML content), graceful handling of malformed input, and that the
// persistent history keeps its trees.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// pruneTestTree is a tree that ParseUITree accepts (card > kv).
const pruneTestTree = `{"type":"card","props":{"title":"销售"},"children":[{"type":"kv"}]}`

func pruneTestAgent(prune bool) *Agent {
	return &Agent{cfg: &config.Config{UIEnabled: true, UIContextPrune: prune}}
}

// UC-40: the switch defaults to pruning, but switching it off must leave the
// history untouched — trees and all.
func TestUIPruneDisabledKeepsTree(t *testing.T) {
	ag := pruneTestAgent(false)
	args := `{"meta":{"intent":"x"},"tree":` + pruneTestTree + `}`
	msgs := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "render_ui", Arguments: args}}}}

	got := ag.pruneUIContext(msgs)
	if got[0].ToolCalls[0].Arguments != args {
		t.Fatalf("UC-40: pruning disabled but arguments changed: %s", got[0].ToolCalls[0].Arguments)
	}
}

// UC-41: with the switch on, the tree becomes a summary while the rest of the
// arguments (meta, update, waiting) survive verbatim.
func TestUIPruneReplacesTreeKeepsRest(t *testing.T) {
	ag := pruneTestAgent(true)
	args := `{"meta":{"intent":"展示"},"update":"ui-9","waiting":true,"tree":` + pruneTestTree + `}`
	msgs := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "render_ui", Arguments: args}}}}

	got := ag.pruneUIContext(msgs)
	out := got[0].ToolCalls[0].Arguments
	if strings.Contains(out, `"children"`) {
		t.Fatalf("UC-41: tree body survived pruning: %s", out)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("UC-41: pruned arguments are not JSON: %v (%s)", err, out)
	}
	tree, ok := parsed["tree"].(map[string]interface{})
	if !ok {
		t.Fatalf("UC-41: tree argument is not an object: %v", parsed["tree"])
	}
	summary, _ := tree["summary"].(string)
	if summary == "" || !strings.Contains(summary, "card") {
		t.Fatalf("UC-41: summary does not describe the tree: %q", summary)
	}
	if parsed["update"] != "ui-9" || parsed["waiting"] != true {
		t.Fatalf("UC-41: sibling arguments were altered: %v", parsed)
	}
	if _, ok := parsed["meta"]; !ok {
		t.Fatalf("UC-41: meta argument was dropped: %v", parsed)
	}
}

// UC-42: pruning must not write through to the persistent history — the
// ToolCalls slice is shared with a.messages, so it has to be cloned.
func TestUIPruneLeavesHistoryIntact(t *testing.T) {
	ag := pruneTestAgent(true)
	args := `{"tree":` + pruneTestTree + `}`
	ag.messages = []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "render_ui", Arguments: args}}},
	}

	// buildContextMessages copies the slice headers only; pass a copy here so
	// the test focuses on the shared ToolCalls backing array.
	ctx := make([]llm.Message, len(ag.messages))
	copy(ctx, ag.messages)
	ctx = ag.pruneUIContext(ctx)

	if strings.Contains(ctx[1].ToolCalls[0].Arguments, `"children"`) {
		t.Fatalf("UC-42: context copy was not pruned: %s", ctx[1].ToolCalls[0].Arguments)
	}
	if !strings.Contains(ag.messages[1].ToolCalls[0].Arguments, `"children"`) {
		t.Fatalf("UC-42: pruning leaked into the persistent history: %s", ag.messages[1].ToolCalls[0].Arguments)
	}
}

// UC-43: XML-mode history keeps the call inside the assistant content.
func TestUIPruneXMLContent(t *testing.T) {
	ag := pruneTestAgent(true)
	content := "<render_ui>\n<tree>" + pruneTestTree + "</tree>\n<waiting>false</waiting>\n</render_ui>"
	msgs := []llm.Message{{Role: "assistant", Content: content}}

	got := ag.pruneUIContext(msgs)
	out := got[0].Content
	if strings.Contains(out, `"children"`) {
		t.Fatalf("UC-43: XML tree body survived pruning: %s", out)
	}
	if !strings.Contains(out, "<tree>") || !strings.Contains(out, "</tree>") {
		t.Fatalf("UC-43: XML envelope was damaged: %s", out)
	}
	if !strings.Contains(out, "summary") {
		t.Fatalf("UC-43: no summary replaced the XML tree: %s", out)
	}
}

// UC-44: anything that is not a prunable render_ui call stays as it was —
// malformed trees, other tools, plain messages.
func TestUIPruneLeavesOtherContent(t *testing.T) {
	ag := pruneTestAgent(true)
	bad := `{"tree":{"type":"sparkline"}}`          // unknown component
	broken := `{"tree":`                            // truncated JSON
	other := `{"tree":{"type":"card"}}`             // tool that is not render_ui
	plain := "just a text answer with <tree> no markup"

	msgs := []llm.Message{
		{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "render_ui", Arguments: bad}}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "render_ui", Arguments: broken}}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{Name: "read_file", Arguments: other}}},
		{Role: "assistant", Content: plain},
		{Role: "user", Content: "hello"},
	}

	got := ag.pruneUIContext(msgs)
	if got[0].ToolCalls[0].Arguments != bad {
		t.Fatalf("UC-44: invalid tree was rewritten: %s", got[0].ToolCalls[0].Arguments)
	}
	if got[1].ToolCalls[0].Arguments != broken {
		t.Fatalf("UC-44: broken JSON was rewritten: %s", got[1].ToolCalls[0].Arguments)
	}
	if got[2].ToolCalls[0].Arguments != other {
		t.Fatalf("UC-44: non-render_ui tool was rewritten: %s", got[2].ToolCalls[0].Arguments)
	}
	if got[3].Content != plain || got[4].Content != "hello" {
		t.Fatalf("UC-44: plain messages were altered: %q %q", got[3].Content, got[4].Content)
	}
}
