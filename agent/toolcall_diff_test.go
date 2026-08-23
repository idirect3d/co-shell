package agent

import (
	"strings"
	"testing"
)

// TestBuildReplaceDiff_UnchangedLines verifies FEATURE-424 UC-003: lines present
// in both search and replace are marked unchanged (" "), lines only in search
// are marked deleted ("-"), and lines only in replace are marked added ("+").
func TestBuildReplaceDiff_UnchangedLines(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	r.replaceSearchBuf.WriteString("line1\nline2\nline3\n")
	r.replaceReplaceBuf.WriteString("line1\nline2\nline4\n")
	r.replaceStartLine = 10

	got := r.buildReplaceDiff()
	if !strings.Contains(got, "    10  line1") {
		t.Errorf("unchanged line1 should be marked ' ', got: %q", got)
	}
	if !strings.Contains(got, "    11  line2") {
		t.Errorf("unchanged line2 should be marked ' ', got: %q", got)
	}
	if !strings.Contains(got, "    12- line3") {
		t.Errorf("deleted line3 should be marked '-', got: %q", got)
	}
	if !strings.Contains(got, "    12+ line4") {
		t.Errorf("added line4 should be marked '+', got: %q", got)
	}
}

// TestBuildReplaceDiff_NoStartLine verifies FEATURE-424: without a start_line,
// line numbers start at 1.
func TestBuildReplaceDiff_NoStartLine(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	r.replaceSearchBuf.WriteString("alpha\n")
	r.replaceReplaceBuf.WriteString("beta\n")

	got := r.buildReplaceDiff()
	if !strings.Contains(got, "     1- alpha") {
		t.Errorf("deleted line should start at line 1, got: %q", got)
	}
	if !strings.Contains(got, "     1+ beta") {
		t.Errorf("added line should start at line 1, got: %q", got)
	}
}

// TestBuildReplaceDiff_Empty verifies FEATURE-424: empty search/replace produce
// no diff.
func TestBuildReplaceDiff_Empty(t *testing.T) {
	r := NewToolCallRenderer(true, true)
	if got := r.buildReplaceDiff(); got != "" {
		t.Errorf("empty search/replace should produce no diff, got: %q", got)
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
	// "   257- old" — 1 space + 5-digit right-aligned 257 ("  257") + "-" + " " + content.
	if !strings.Contains(got, "   257- old") {
		t.Errorf("deleted line format wrong, got: %q", got)
	}
	if !strings.Contains(got, "   257+ new") {
		t.Errorf("added line format wrong, got: %q", got)
	}
}

// TestToolCallDiff_EmitOnToolEnd verifies FEATURE-424 UC-004: when a
// replace_in_file call completes (OpToolEnd), the diffEmit callback receives
// the unified diff rendering.
func TestToolCallDiff_EmitOnToolEnd(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)
	var diff string
	r.SetDiffEmit(func(d string) { diff = d })

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

	if diff == "" {
		t.Fatal("diffEmit should have been called with a non-empty diff")
	}
	if !strings.Contains(diff, "     1  line1") {
		t.Errorf("unchanged line1 should be marked ' ', got: %q", diff)
	}
	if !strings.Contains(diff, "     2- line2") {
		t.Errorf("deleted line2 should be marked '-', got: %q", diff)
	}
	if !strings.Contains(diff, "     2+ line3") {
		t.Errorf("added line3 should be marked '+', got: %q", diff)
	}
}
