// Package agent - LLM-driven UI component tree protocol (FEATURE-524).
//
// This file defines the declarative component tree that the render_ui tool
// accepts from the LLM, together with the whitelist/limit validation that
// keeps the tree predictable and safe. The frontend registry
// (web/static/ui.js) renders each node type; adding a component means adding
// one entry here (validation) and one registration there (rendering) — the
// tool schema, the event protocol and the agent loop stay untouched.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// UI component type names. These are the registry keys shared verbatim with
// web/static/ui.js — keep both sides in sync.
const (
	UICompCard     = "card"     // container with a title/body
	UICompKV       = "kv"       // key/value list
	UICompTable    = "table"    // enhanced table
	UICompChart    = "chart"    // SVG chart (bar/line/pie)
	UICompSteps    = "steps"    // step / timeline list
	UICompCallout  = "callout"  // info/warn/success/error notice
	UICompProgress = "progress" // progress bar
	UICompFile     = "file"     // file card with open/reveal
	UICompForm     = "form"     // form with buttons/selects/inputs
	UICompHTML     = "html"     // sandboxed HTML escape hatch
	UICompRow      = "row"      // horizontal layout container
	UICompCol      = "col"      // one vertical column inside a row
)

// Tree limits. They bound memory, layout cost and (most importantly) how
// much of a component tree can end up in the conversation context.
const (
	// UIMaxTreeDepth is the maximum root-to-leaf node depth.
	UIMaxTreeDepth = 6
	// UIMaxTreeNodes is the maximum number of nodes in one tree.
	UIMaxTreeNodes = 200
	// UIMaxNodeProps is the maximum serialized size of a single node's props.
	UIMaxNodeProps = 8 * 1024
)

// Interaction kinds a node may declare in its actions list.
const (
	UIActionClick  = "click"
	UIActionSelect = "select"
	UIActionSubmit = "submit"
	UIActionChange = "change"
)

// Payload shaping modes: what the frontend sends back when the action fires.
const (
	UIPayloadNode  = "node"
	UIPayloadValue = "value"
	UIPayloadRow   = "row"
	UIPayloadPoint = "point"
	UIPayloadForm  = "form"
)

// uiComponentTypes is the whitelist of supported node types.
var uiComponentTypes = []string{
	UICompCallout,
	UICompCard,
	UICompChart,
	UICompCol,
	UICompFile,
	UICompForm,
	UICompHTML,
	UICompKV,
	UICompProgress,
	UICompRow,
	UICompSteps,
	UICompTable,
}

// uiActionKinds is the whitelist of action trigger names.
var uiActionKinds = map[string]bool{
	UIActionClick:  true,
	UIActionSelect: true,
	UIActionSubmit: true,
	UIActionChange: true,
}

// UIComponentTypeSupported reports whether typ is a registered component.
func UIComponentTypeSupported(typ string) bool {
	for _, t := range uiComponentTypes {
		if t == typ {
			return true
		}
	}
	return false
}

// UIComponentCatalog returns the supported component type names as a
// comma-separated list, for error messages and the system prompt.
func UIComponentCatalog() string {
	types := append([]string(nil), uiComponentTypes...)
	sort.Strings(types)
	return strings.Join(types, ", ")
}

// UIAction is one declared interaction point on a node. When the user
// performs it, the frontend sends {"type":"ui_action", ui_id, action_id,
// payload} upstream, carrying Payload-shaped data back to the agent.
type UIAction struct {
	On      string `json:"on"`                // click | select | submit | change
	ID      string `json:"id"`                // action id, echoed back on fire
	Payload string `json:"payload,omitempty"` // node | value | row | point | form
}

// UINode is one node of the component tree. Props is kept as raw JSON so the
// exact bytes the LLM produced reach the frontend without a lossy round trip
// through map[string]interface{}.
type UINode struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	Props    json.RawMessage `json:"props,omitempty"`
	Children []UINode        `json:"children,omitempty"`
	Actions  []UIAction      `json:"actions,omitempty"`
}

// ParseUITree decodes raw JSON into a component tree and validates it. Every
// failure is reported as an error the LLM can act on (it names the offending
// node and lists the allowed values), so a malformed call can be corrected
// without human intervention.
func ParseUITree(raw string) (*UINode, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s", i18n.T(i18n.KeyUIErrTreeEmpty))
	}
	var root UINode
	dec := json.NewDecoder(strings.NewReader(trimmed))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTreeDecode, err.Error()))
	}
	if err := ValidateUITree(&root); err != nil {
		return nil, err
	}
	return &root, nil
}

// ValidateUITree checks the whole tree against the protocol limits and the
// component whitelist.
func ValidateUITree(root *UINode) error {
	if root == nil {
		return fmt.Errorf("%s", i18n.T(i18n.KeyUIErrTreeEmpty))
	}
	total := 0
	if err := validateUINode(root, 1, &total); err != nil {
		return err
	}
	return nil
}

// validateUINode validates one node and recurses into its children. depth is
// 1-based; total accumulates the node count across the whole tree.
func validateUINode(n *UINode, depth int, total *int) error {
	if depth > UIMaxTreeDepth {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTreeTooDeep, UIMaxTreeDepth))
	}
	*total++
	if *total > UIMaxTreeNodes {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTreeTooManyNodes, UIMaxTreeNodes))
	}
	if n.Type == "" {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrNodeTypeMissing, UIComponentCatalog()))
	}
	if !UIComponentTypeSupported(n.Type) {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrNodeTypeUnknown, n.Type, UIComponentCatalog()))
	}
	if len(n.Props) > UIMaxNodeProps {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrNodePropsTooLarge, n.Type, UIMaxNodeProps))
	}
	for _, a := range n.Actions {
		if !uiActionKinds[a.On] {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrActionKind, a.On, n.Type))
		}
		if strings.TrimSpace(a.ID) == "" {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrActionID, n.Type))
		}
	}
	for i := range n.Children {
		if err := validateUINode(&n.Children[i], depth+1, total); err != nil {
			return err
		}
	}
	return nil
}

// CountUINodes returns the node count of a tree (0 for nil).
func CountUINodes(root *UINode) int {
	if root == nil {
		return 0
	}
	n := 0
	countUINodes(root, &n)
	return n
}

// countUINodes accumulates the node count of the subtree rooted at n.
func countUINodes(n *UINode, out *int) {
	*out++
	for i := range n.Children {
		countUINodes(&n.Children[i], out)
	}
}

// UISummary describes a validated tree in one short line, e.g.
// "已渲染 card（id=ui-7，3 个子节点）". It is what the tool returns to the LLM: a
// receipt that confirms the render — and carries the id the LLM needs to
// address the tree later — without putting the whole tree back into the
// conversation context.
func UISummary(root *UINode, id string) string {
	if root == nil {
		return ""
	}
	children := len(root.Children)
	if children == 0 {
		return i18n.TF(i18n.KeyUISummaryNoChildren, root.Type, id)
	}
	return i18n.TF(i18n.KeyUISummaryChildren, root.Type, id, children)
}

// MarshalUITree serializes a validated tree back to compact JSON for the
// ui_render event Meta.
func MarshalUITree(root *UINode) (string, error) {
	b, err := json.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
