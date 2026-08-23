package agent

import (
	"strings"
	"testing"
)

// TestToolCallDiff_JSONReplace verifies FEATURE-424: in JSON (OpenAI) mode a
// replace_in_file call also emits a tool_call_diff event with per-line
// add/delete/unchanged status, and that start_line is honoured (not always 1).
func TestToolCallDiff_JSONReplace(t *testing.T) {
	p := NewJSONToolCallParser()
	r := NewToolCallRenderer(true, true)
	var lines []ToolDiffLine
	r.SetDiffEmit(func(d []ToolDiffLine) { lines = d })

	// Simulate the JSON tool-call argument deltas for a replace_in_file call.
	// The tool name arrives first, then the arguments stream in fragments.
	r.Apply(RenderOp{Kind: OpToolStart, Text: "replace_in_file"}, func(string) {})
	chunks := []string{
		`{"path": "a.go", "replacements": [{"search": "line1\nline2", "replace": "line1\nline3", "start_line": 10}]}`,
	}
	for _, c := range chunks {
		ops, err := p.Feed(c)
		if err != nil {
			t.Fatalf("Feed(%q) unexpected error: %v", c, err)
		}
		for _, op := range ops {
			r.Apply(op, func(string) {})
		}
	}
	// Finalise the tool call (JSON mode never emits OpToolEnd from the parser).
	r.Apply(RenderOp{Kind: OpToolEnd}, func(string) {})

	if len(lines) == 0 {
		t.Fatal("replace_in_file should emit diff lines via diffEmit")
	}
	// line1 unchanged (ctx), line2 deleted (del), line3 added (add).
	if len(lines) != 3 {
		t.Fatalf("expected 3 diff lines, got %d: %+v", len(lines), lines)
	}
	if l := findDiffLine(lines, "line1"); l == nil || l.Status != "ctx" {
		t.Errorf("line1 should be ctx, got: %+v", l)
	}
	if l := findDiffLine(lines, "line2"); l == nil || l.Status != "del" {
		t.Errorf("line2 should be del, got: %+v", l)
	}
	if l := findDiffLine(lines, "line3"); l == nil || l.Status != "add" {
		t.Errorf("line3 should be add, got: %+v", l)
	}
	// start_line=10 should be honoured: line1 at line 10, line2 at line 11.
	if l := findDiffLine(lines, "line1"); l == nil || !strings.Contains(l.Line, "   10 ") {
		t.Errorf("line1 should be at line 10, got: %+v", l)
	}
	if l := findDiffLine(lines, "line2"); l == nil || !strings.Contains(l.Line, "   11-") {
		t.Errorf("line2 should be at line 11, got: %+v", l)
	}
}
