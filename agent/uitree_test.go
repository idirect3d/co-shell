// Package agent - unit tests for the LLM component tree protocol
// (FEATURE-524, use-case group A: UC-01 .. UC-06).
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"strings"
	"testing"
)

// UC-01: a legal minimal tree passes validation and is counted correctly.
func TestUIUC01ValidMinimalTree(t *testing.T) {
	root, err := ParseUITree(`{"type":"card","props":{"title":"销售分析"}}`)
	if err != nil {
		t.Fatalf("UC-01: valid tree rejected: %v", err)
	}
	if root.Type != UICompCard {
		t.Fatalf("UC-01: root type = %q, want %q", root.Type, UICompCard)
	}
	if got := CountUINodes(root); got != 1 {
		t.Fatalf("UC-01: node count = %d, want 1", got)
	}
	summary := UISummary(root, "ui-1")
	if !strings.Contains(summary, "ui-1") {
		t.Fatalf("UC-01: receipt %q must carry the tree id", summary)
	}
	if !strings.Contains(summary, UICompCard) {
		t.Fatalf("UC-01: receipt %q must name the root component", summary)
	}

	// A tree with children is counted recursively, and its receipt reports the
	// child count instead of the "no children" wording.
	nested, err := ParseUITree(`{"type":"card","children":[{"type":"kv"},{"type":"progress"}]}`)
	if err != nil {
		t.Fatalf("UC-01: valid nested tree rejected: %v", err)
	}
	if got := CountUINodes(nested); got != 3 {
		t.Fatalf("UC-01: nested node count = %d, want 3", got)
	}
	if !strings.Contains(UISummary(nested, "ui-2"), "2") {
		t.Fatalf("UC-01: receipt %q must report 2 children", UISummary(nested, "ui-2"))
	}
}

// UC-02: an unregistered component type is rejected with an actionable error.
func TestUIUC02UnknownTypeRejected(t *testing.T) {
	_, err := ParseUITree(`{"type":"sparkline"}`)
	if err == nil {
		t.Fatal("UC-02: unknown component type must be rejected")
	}
	if !strings.Contains(err.Error(), "sparkline") {
		t.Fatalf("UC-02: error %q must name the offending type", err.Error())
	}
	if !strings.Contains(err.Error(), UICompCard) {
		t.Fatalf("UC-02: error %q must list the available types", err.Error())
	}

	// A node without a type is rejected too, and the error names the catalogue
	// rather than the empty string.
	if _, err := ParseUITree(`{"props":{}}`); err == nil {
		t.Fatal("UC-02: missing type must be rejected")
	}
}

// UC-03: a tree deeper than UIMaxTreeDepth is rejected.
func TestUIUC03DepthLimit(t *testing.T) {
	raw := `{"type":"card"}`
	for i := 0; i < UIMaxTreeDepth; i++ {
		raw = `{"type":"card","children":[` + raw + `]}`
	}
	if _, err := ParseUITree(raw); err == nil {
		t.Fatalf("UC-03: depth %d must be rejected", UIMaxTreeDepth+1)
	}

	// Exactly at the limit is still accepted.
	ok := `{"type":"card"}`
	for i := 0; i < UIMaxTreeDepth-1; i++ {
		ok = `{"type":"card","children":[` + ok + `]}`
	}
	if _, err := ParseUITree(ok); err != nil {
		t.Fatalf("UC-03: depth %d must be accepted: %v", UIMaxTreeDepth, err)
	}
}

// UC-04: a tree with more than UIMaxTreeNodes nodes is rejected.
func TestUIUC04NodeLimit(t *testing.T) {
	children := make([]string, 0, UIMaxTreeNodes+1)
	for i := 0; i <= UIMaxTreeNodes; i++ {
		children = append(children, `{"type":"kv"}`)
	}
	raw := `{"type":"card","children":[` + strings.Join(children, ",") + `]}`
	_, err := ParseUITree(raw)
	if err == nil {
		t.Fatalf("UC-04: %d nodes must be rejected", UIMaxTreeNodes+1)
	}
	if !strings.Contains(err.Error(), "200") {
		t.Fatalf("UC-04: error %q must state the node limit", err.Error())
	}
}

// UC-05: a node whose props exceed UIMaxNodeProps is rejected.
func TestUIUC05PropsTooLarge(t *testing.T) {
	blob := strings.Repeat("x", UIMaxNodeProps+1)
	raw := `{"type":"callout","props":{"text":"` + blob + `"}}`
	_, err := ParseUITree(raw)
	if err == nil {
		t.Fatalf("UC-05: props larger than %d bytes must be rejected", UIMaxNodeProps)
	}
	if !strings.Contains(err.Error(), "callout") {
		t.Fatalf("UC-05: error %q must name the offending node", err.Error())
	}
}

// UC-06: illegal action declarations are rejected.
func TestUIUC06InvalidActions(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"unknown trigger", `{"type":"card","actions":[{"on":"hover","id":"a"}]}`, "hover"},
		{"missing id", `{"type":"card","actions":[{"on":"click"}]}`, "id"},
	}
	for _, tc := range cases {
		_, err := ParseUITree(tc.raw)
		if err == nil {
			t.Fatalf("UC-06 (%s): must be rejected", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("UC-06 (%s): error %q must mention %q", tc.name, err.Error(), tc.want)
		}
	}

	// A well-formed action passes.
	if _, err := ParseUITree(`{"type":"chart","actions":[{"on":"select","id":"drill","payload":"point"}]}`); err != nil {
		t.Fatalf("UC-06: valid action rejected: %v", err)
	}
}

// TestUIUC05DecodeErrors covers the non-JSON and empty inputs, which must fail
// with a message the LLM can act on rather than a panic.
func TestUIUC05DecodeErrors(t *testing.T) {
	if _, err := ParseUITree("   "); err == nil {
		t.Fatal("empty tree must be rejected")
	}
	if _, err := ParseUITree("{not json"); err == nil {
		t.Fatal("malformed JSON must be rejected")
	}
}

// UC-45: layout containers (row / col) are accepted, nest, and stay inside the
// existing depth guard (FEATURE-524 Stage 5).
func TestUIRowColLayoutContainers(t *testing.T) {
	const tree = `{"type":"row","props":{"gap":12},"children":[` +
		`{"type":"col","props":{"flex":2},"children":[{"type":"steps"}]},` +
		`{"type":"col","children":[` +
		`{"type":"card","props":{"icon":"u","title":"user info"}},` +
		`{"type":"form","props":{"title":"signup"},"actions":[{"on":"submit","id":"save"}]}` +
		`]}]}`

	root, err := ParseUITree(tree)
	if err != nil {
		t.Fatalf("UC-45: row/col layout tree rejected: %v", err)
	}
	if root.Type != UICompRow {
		t.Fatalf("UC-45: root type = %q, want %q", root.Type, UICompRow)
	}
	if got := CountUINodes(root); got != 6 {
		t.Fatalf("UC-45: node count = %d, want 6", got)
	}
	for _, typ := range []string{UICompRow, UICompCol} {
		if !UIComponentTypeSupported(typ) {
			t.Fatalf("UC-45: component %q must be registered", typ)
		}
		if !strings.Contains(UIComponentCatalog(), typ) {
			t.Fatalf("UC-45: catalogue %q must list %q", UIComponentCatalog(), typ)
		}
	}

	// Rows nest (a row inside a col, e.g. a grid of columns) ...
	if _, err := ParseUITree(`{"type":"col","children":[{"type":"row","children":[{"type":"kv"}]}]}`); err != nil {
		t.Fatalf("UC-45: nested row rejected: %v", err)
	}

	// ... but the depth guard still applies to layout containers.
	deep := `{"type":"kv"}`
	for i := 0; i < UIMaxTreeDepth; i++ {
		deep = `{"type":"row","children":[` + deep + `]}`
	}
	if _, err := ParseUITree(deep); err == nil {
		t.Fatalf("UC-45: depth %d must be rejected for row containers", UIMaxTreeDepth+1)
	}
}
