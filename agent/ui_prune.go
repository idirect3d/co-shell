// Package agent - component-tree pruning for the LLM context (FEATURE-524).
//
// A render_ui call writes the whole component tree into the message history as
// tool-call arguments, so every later LLM call pays for it again: a table with
// fifty rows is re-sent on every iteration even though the tool itself only
// ever returns a one-line receipt. UIContextPrune (default on) replaces those
// trees with a short summary on their way into the context.
//
// The pruning happens in buildContextMessages, i.e. on the copy handed to the
// provider, so the persisted history and the Web UI replay keep the full tree.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// pruneUIContext rewrites render_ui tool-call arguments in msgs so the trees
// become summaries. It returns msgs unchanged when the switch is off.
//
// The slice is the per-call copy built by buildContextMessages, but its
// elements still share the ToolCalls backing array with the persistent history,
// so the slice is cloned before any argument is replaced.
func (a *Agent) pruneUIContext(msgs []llm.Message) []llm.Message {
	if a.cfg == nil || !a.cfg.UIContextPrune {
		return msgs
	}
	for i := range msgs {
		if len(msgs[i].ToolCalls) > 0 {
			calls := make([]llm.ToolCall, len(msgs[i].ToolCalls))
			copy(calls, msgs[i].ToolCalls)
			changed := false
			for j := range calls {
				if calls[j].Name != uiToolName {
					continue
				}
				if pruned, ok := pruneUITreeArgs(calls[j].Arguments); ok {
					calls[j].Arguments = pruned
					changed = true
				}
			}
			if changed {
				msgs[i].ToolCalls = calls
			}
		}
		// XML mode carries the call in the assistant content instead.
		if content, ok := pruneUITreeXML(msgs[i].Content); ok {
			msgs[i].Content = content
		}
	}
	return msgs
}

// uiToolName is the tool whose tree argument gets pruned.
const uiToolName = "render_ui"

// pruneUITreeArgs replaces the "tree" argument of one render_ui call with its
// summary. Anything unexpected (no tree, not JSON, not a valid tree) is left
// untouched so a malformed history still reaches the model as it was.
func pruneUITreeArgs(arguments string) (string, bool) {
	if !strings.Contains(arguments, `"tree"`) {
		return "", false
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", false
	}
	raw, ok := args["tree"]
	if !ok || raw == nil {
		return "", false
	}
	summary, ok := uiTreeSummaryOf(raw)
	if !ok {
		return "", false
	}
	args["tree"] = map[string]interface{}{"summary": summary}
	out, err := json.Marshal(args)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// pruneUITreeXML prunes the <tree> payloads embedded in XML-mode assistant
// messages. Segments that do not validate as a component tree (or that were
// already pruned) are kept verbatim.
func pruneUITreeXML(content string) (string, bool) {
	if !strings.Contains(content, "<tree>") || !strings.Contains(content, uiToolName) {
		return "", false
	}
	var b strings.Builder
	rest := content
	changed := false
	for {
		open := strings.Index(rest, "<tree>")
		if open < 0 {
			break
		}
		closeIdx := strings.Index(rest[open:], "</tree>")
		if closeIdx < 0 {
			break
		}
		closeIdx += open
		body := rest[open+len("<tree>") : closeIdx]
		b.WriteString(rest[:open+len("<tree>")])
		if summary, ok := uiTreeSummaryOf(body); ok {
			// A JSON string keeps the payload parseable for the model.
			encoded, err := json.Marshal(map[string]string{"summary": summary})
			if err == nil {
				b.Write(encoded)
				changed = true
			} else {
				b.WriteString(body)
			}
		} else {
			b.WriteString(body)
		}
		b.WriteString("</tree>")
		rest = rest[closeIdx+len("</tree>"):]
	}
	b.WriteString(rest)
	if !changed {
		return "", false
	}
	return b.String(), true
}

// uiTreeSummaryOf parses raw (tree object or already-encoded JSON string) and
// describes it in one line, reusing the same validator the tool uses.
func uiTreeSummaryOf(raw interface{}) (string, bool) {
	treeJSON, err := uiTreeArgJSON(raw)
	if err != nil {
		return "", false
	}
	root, err := ParseUITree(treeJSON)
	if err != nil {
		return "", false
	}
	return i18n.TF(i18n.KeyUIPrunedTree, root.Type, CountUINodes(root)), true
}
