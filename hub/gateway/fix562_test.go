package gateway

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// FIX-562: the read-only 「Agent描述」 block that ends the Agent-settings form
// used to hug the hint line above it. The hint carries only margin-top and the
// field only margin-bottom, so the gap between them collapsed to 0px (measured
// in the browser), while every other pair of fields is 14px apart. The block now
// carries .field-sep, which adds an explicit top margin.
//
// These tests cover the A group of use-case/FIX-562/FIX-562-UC-0001.md by
// checking the static page source; the real rendering is verified by the B group.

// fix562SepRule returns the body of the ".field-sep {" CSS rule.
func fix562SepRule(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, ".field-sep {")
	if start < 0 {
		t.Fatal(".field-sep rule not found in webui_static.go")
	}
	rest := page[start:]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal(".field-sep rule is not terminated by \"}\"")
	}
	return rest[:end]
}

// A-001/A-002: the separator rule exists exactly once and lifts the block by at
// least the 14px that separates the ordinary fields.
func TestFix562DescriptionBlockSeparatorRule(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	rule := fix562SepRule(t, page)
	m := regexp.MustCompile(`margin-top:(\d+)px`).FindStringSubmatch(rule)
	if m == nil {
		t.Fatalf("margin-top not found in the .field-sep rule:\n%s", rule)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("margin-top: bad pixel value %q: %v", m[1], err)
	}
	if n < 14 {
		t.Errorf("field-sep margin-top = %dpx, want >= 14px (the gap used between ordinary fields)", n)
	}
	if got := strings.Count(page, ".field-sep {"); got != 1 {
		t.Errorf(".field-sep is defined %d times, want exactly 1 (a stale rule would override the fix)", got)
	}
}

// A-003: only the read-only Agent描述 field carries the separator class, and it
// sits before the "Save changes" footer.
func TestFix562DescriptionFieldUsesSeparator(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	const field = `<div class="field field-sep"><label data-i18n="lblAgentDescReported">`
	if !strings.Contains(page, field) {
		t.Error("the Agent描述 field does not carry the field-sep class")
	}
	if got := strings.Count(page, `class="field field-sep"`); got != 1 {
		t.Errorf("field-sep is used by %d fields, want exactly 1 (the Agent描述 block)", got)
	}
	if idx := strings.Index(page, field); idx >= 0 {
		if foot := strings.Index(page[idx:], `id="d-save"`); foot < 0 {
			t.Error("the Agent描述 field is no longer the last block before the save button")
		}
	}
}
