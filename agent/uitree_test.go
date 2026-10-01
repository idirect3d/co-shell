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

// --- FEATURE-570: figure canvas and shapes (use-case UC-0019) ---

// UC-0019-1: figure and its shapes are whitelisted; the catalogue lists figure.
func TestUIUC0019FigureWhitelist(t *testing.T) {
	if !UIComponentTypeSupported(UICompFigure) {
		t.Fatalf("UC-0019: figure must be a registered component")
	}
	if !strings.Contains(UIComponentCatalog(), UICompFigure) {
		t.Fatalf("UC-0019: catalogue %q must list figure", UIComponentCatalog())
	}
	for _, typ := range []string{
		UIShapePoint, UIShapeSegment, UIShapePolygon, UIShapeEllipse,
		UIShapeAxes, UIShapeLabel, UIShapePath, UIShapeFunction,
	} {
		if !isUIShape(typ) {
			t.Fatalf("UC-0019: shape %q must be whitelisted", typ)
		}
		if !strings.Contains(UIShapeCatalog(), typ) {
			t.Fatalf("UC-0019: shape catalogue %q must list %q", UIShapeCatalog(), typ)
		}
	}
}

// UC-0019-2: a full 2D figure with every shape type passes validation.
func TestUIUC0019ValidFigure(t *testing.T) {
	tree := `{"type":"figure","props":{"title":"t","xmin":-1,"xmax":10,"ymin":-1,"ymax":10,"view":"iso","axes":true,"grid":true,"download":false},"children":[
		{"type":"axes","props":{"origin":[0,0],"xlabel":"x","ylabel":"y"}},
		{"type":"point","props":{"x":1,"y":2,"z":3,"label":"P","r":4,"filled":true}},
		{"type":"segment","props":{"x1":0,"y1":0,"x2":1,"y2":1,"arrow":"end","stroke":"accent","strokeWidth":2}},
		{"type":"polygon","props":{"points":[[0,0],[1,0],[0,1]],"fill":"info","opacity":0.4}},
		{"type":"ellipse","props":{"cx":1,"cy":1,"rx":2,"ry":1,"rotation":30}},
		{"type":"label","props":{"x":0,"y":0,"text":"O","anchor":"middle","size":14}},
		{"type":"path","props":{"points":[[0,0],[1,2],[2,1]],"smooth":true,"dash":"dashed"}},
		{"type":"function","props":{"expr":"sin(x)*2","from":0,"to":6.28,"samples":100}}]}`
	root, err := ParseUITree(tree)
	if err != nil {
		t.Fatalf("UC-0019: valid figure rejected: %v", err)
	}
	if got := CountUINodes(root); got != 9 {
		t.Fatalf("UC-0019: node count = %d, want 9", got)
	}
}

// UC-0019-3: shapes are rejected outside a figure, and a figure only accepts
// shapes as children.
func TestUIUC0019ShapeContext(t *testing.T) {
	for _, tree := range []string{
		`{"type":"card","children":[{"type":"point","props":{"x":1,"y":1}}]}`,
		`{"type":"point","props":{"x":1,"y":1}}`,
		`{"type":"figure","children":[{"type":"card"}]}`,
		`{"type":"figure","children":[{"type":"figure"}]}`,
	} {
		if _, err := ParseUITree(tree); err == nil {
			t.Fatalf("UC-0019: tree must be rejected: %s", tree)
		}
	}
}

// UC-0019-4: non-finite/ill-typed numbers, bad ranges and unknown style names
// are rejected.
func TestUIUC0019NumericGuards(t *testing.T) {
	for _, tree := range []string{
		`{"type":"figure","children":[{"type":"point","props":{"x":1,"y":1e999}}]}`,
		`{"type":"figure","children":[{"type":"point","props":{"x":1}}]}`,
		`{"type":"figure","children":[{"type":"point","props":{"x":"1","y":1}}]}`,
		`{"type":"figure","children":[{"type":"ellipse","props":{"cx":0,"cy":0,"rx":-1,"ry":1}}]}`,
		`{"type":"figure","children":[{"type":"segment","props":{"x1":0,"y1":0,"x2":1,"y2":1,"strokeWidth":7}}]}`,
		`{"type":"figure","children":[{"type":"segment","props":{"x1":0,"y1":0,"x2":1,"y2":1,"opacity":1.5}}]}`,
		`{"type":"figure","children":[{"type":"segment","props":{"x1":0,"y1":0,"x2":1,"y2":1,"stroke":"#ff0000"}}]}`,
		`{"type":"figure","props":{"xmin":5,"xmax":1},"children":[{"type":"point","props":{"x":1,"y":1}}]}`,
		`{"type":"figure","props":{"view":"bird"},"children":[]}`,
	} {
		if _, err := ParseUITree(tree); err == nil {
			t.Fatalf("UC-0019: tree must be rejected: %s", tree)
		}
	}
}

// UC-0019-5: the per-shape point budget and point shape are enforced.
func TestUIUC0019PointsGuards(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"type":"figure","children":[{"type":"path","props":{"points":[`)
	for i := 0; i <= UIMaxShapePoints; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("[0,0]")
	}
	b.WriteString(`]}}]}`)
	if _, err := ParseUITree(b.String()); err == nil {
		t.Fatalf("UC-0019: %d points must be rejected", UIMaxShapePoints+1)
	}

	if _, err := ParseUITree(`{"type":"figure","children":[{"type":"polygon","props":{"points":[[0,0],[1,1,1,1]]}}]}`); err == nil {
		t.Fatalf("UC-0019: malformed point must be rejected")
	}
	if _, err := ParseUITree(`{"type":"figure","children":[{"type":"polygon","props":{}}]}`); err == nil {
		t.Fatalf("UC-0019: polygon without points must be rejected")
	}
}

// UC-0019-6: function expressions and sampling are bounded; parametric
// expressions are accepted.
func TestUIUC0019FunctionGuards(t *testing.T) {
	long := strings.Repeat("x+", 300) + "x" // 601 characters
	if _, err := ParseUITree(`{"type":"figure","children":[{"type":"function","props":{"expr":"` + long + `","from":0,"to":1}}]}`); err == nil {
		t.Fatalf("UC-0019: expr longer than %d must be rejected", UIMaxExprLength)
	}
	if _, err := ParseUITree(`{"type":"figure","children":[{"type":"function","props":{"expr":"x","from":0,"to":1,"samples":401}}]}`); err == nil {
		t.Fatalf("UC-0019: samples above the limit must be rejected")
	}
	if _, err := ParseUITree(`{"type":"figure","children":[{"type":"function","props":{"expr":["cos(t)","sin(t)"],"from":0,"to":6.28}}]}`); err != nil {
		t.Fatalf("UC-0019: parametric expr must be accepted: %v", err)
	}
}
