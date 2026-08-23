package agent

import (
	"strings"
	"testing"
)

// findDiffLine returns the first diff line whose rendered text contains the
// given content substring, or nil.
func findDiffLine(lines []ToolDiffLine, content string) *ToolDiffLine {
	for i := range lines {
		if strings.Contains(lines[i].Line, content) {
			return &lines[i]
		}
	}
	return nil
}

// TestBuildReplaceDiff_UnchangedLines verifies FEATURE-424 UC-003: lines present
// in both search and replace are marked unchanged (" "), lines only in search
// are marked deleted ("-"), and lines only in replace are marked added ("+").
func TestBuildReplaceDiff_UnchangedLines(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	r.replaceSearchBuf.WriteString("line1\nline2\nline3\n")
	r.replaceReplaceBuf.WriteString("line1\nline2\nline4\n")
	r.replaceStartLine = 10

	got := r.buildReplaceDiff()
	if len(got) != 4 {
		t.Fatalf("expected 4 diff lines, got %d: %+v", len(got), got)
	}
	if l := findDiffLine(got, "line1"); l == nil || l.Status != "ctx" {
		t.Errorf("unchanged line1 should be ctx, got: %+v", l)
	}
	if l := findDiffLine(got, "line2"); l == nil || l.Status != "ctx" {
		t.Errorf("unchanged line2 should be ctx, got: %+v", l)
	}
	if l := findDiffLine(got, "line3"); l == nil || l.Status != "del" {
		t.Errorf("deleted line3 should be del, got: %+v", l)
	}
	if l := findDiffLine(got, "line4"); l == nil || l.Status != "add" {
		t.Errorf("added line4 should be add, got: %+v", l)
	}
}

// TestBuildReplaceDiff_NoStartLine verifies FEATURE-424: without a start_line,
// line numbers start at 1.
func TestBuildReplaceDiff_NoStartLine(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	r.replaceSearchBuf.WriteString("alpha\n")
	r.replaceReplaceBuf.WriteString("beta\n")

	got := r.buildReplaceDiff()
	if len(got) != 2 {
		t.Fatalf("expected 2 diff lines, got %d: %+v", len(got), got)
	}
	if l := findDiffLine(got, "alpha"); l == nil || l.Status != "del" {
		t.Errorf("deleted line should be del at line 1, got: %+v", l)
	}
	if l := findDiffLine(got, "beta"); l == nil || l.Status != "add" {
		t.Errorf("added line should be add at line 1, got: %+v", l)
	}
}

// TestBuildReplaceDiff_Empty verifies FEATURE-424: empty search/replace produce
// no diff.
func TestBuildReplaceDiff_Empty(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	if got := r.buildReplaceDiff(); len(got) != 0 {
		t.Errorf("empty search/replace should produce no diff, got: %+v", got)
	}
}

// TestBuildReplaceDiff_Format verifies FEATURE-424 UC-001/UC-002: the unified
// format is "{1 space}{5-digit right-aligned line}{status} {content}".
func TestBuildReplaceDiff_Format(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	r.replaceSearchBuf.WriteString("old\n")
	r.replaceReplaceBuf.WriteString("new\n")
	r.replaceStartLine = 257

	got := r.buildReplaceDiff()
	if len(got) != 2 {
		t.Fatalf("expected 2 diff lines, got %d: %+v", len(got), got)
	}
	// "   257- old" — 1 space + 5-digit right-aligned 257 ("  257") + "-" + " " + content.
	if l := findDiffLine(got, "   257- old"); l == nil || l.Status != "del" {
		t.Errorf("deleted line format wrong, got: %+v", l)
	}
	if l := findDiffLine(got, "   257+ new"); l == nil || l.Status != "add" {
		t.Errorf("added line format wrong, got: %+v", l)
	}
}

// TestToolCallDiff_EmitOnToolEnd verifies FEATURE-424 UC-004: when a
// replace_in_file call completes (OpToolEnd), the diffEmit callback receives
// the structured per-line diff data (each line carries its own status).
func TestToolCallDiff_EmitOnToolEnd(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)
	var lines []ToolDiffLine
	r.SetDiffEmit(func(d []ToolDiffLine) { lines = d })

	chunks := []string{
		"<cs:replace_in_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:replacements>",
		"<item><cs:search>line1\nline2</cs:search><cs:replace>line1\nline3</cs:replace></item>",
		"</cs:replacements>",
		"</cs:replace_in_file>",
	}
	var all []RenderOp
	for _, c := range chunks {
		ops, err := p.Feed(c)
		if err != nil {
			t.Fatalf("Feed(%q) unexpected error: %v", c, err)
		}
		all = append(all, ops...)
	}
	for _, op := range all {
		r.Apply(op, func(string) {})
	}

	if len(lines) == 0 {
		t.Fatal("diffEmit should have been called with non-empty diff lines")
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
}

// TestToolCallDiff_WriteToFile verifies FEATURE-424: a write_to_file call also
// emits a tool_call_diff event so its content lines are coloured (all added →
// green) in the Web UI, matching the replace_in_file diff display.
func TestToolCallDiff_WriteToFile(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)
	var lines []ToolDiffLine
	r.SetDiffEmit(func(d []ToolDiffLine) { lines = d })

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:content>line1\nline2</cs:content>",
		"</cs:write_to_file>",
	}
	var all []RenderOp
	for _, c := range chunks {
		ops, err := p.Feed(c)
		if err != nil {
			t.Fatalf("Feed(%q) unexpected error: %v", c, err)
		}
		all = append(all, ops...)
	}
	for _, op := range all {
		r.Apply(op, func(string) {})
	}

	if len(lines) == 0 {
		t.Fatal("write_to_file should emit diff lines via diffEmit")
	}
	// write_to_file content lines are all added (green).
	if len(lines) != 2 {
		t.Fatalf("expected 2 diff lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Status != "add" || !strings.Contains(lines[0].Line, "line1") {
		t.Errorf("line1 should be add, got: %+v", lines[0])
	}
	if lines[1].Status != "add" || !strings.Contains(lines[1].Line, "line2") {
		t.Errorf("line2 should be add, got: %+v", lines[1])
	}
}
