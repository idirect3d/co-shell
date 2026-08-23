package agent

import (
	"strings"
	"testing"
)

// TestToolCallStream_WriteIntent verifies write_to_file renders its intent.
func TestToolCallStream_WriteIntent(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:intent>create file</cs:intent>",
		"<cs:content>line1</cs:content>",
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

	text := collectRenderText(r, all)
	t.Logf("write_to_file render: %q", text)
	if !strings.Contains(text, "create file") {
		t.Errorf("write_to_file intent not rendered, got: %q", text)
	}
}

// TestToolCallStream_ReplaceIntent verifies replace_in_file renders its intent.
func TestToolCallStream_ReplaceIntent(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:replace_in_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:intent>edit file</cs:intent>",
		"<cs:replacements>",
		"<item><cs:search>old</cs:search><cs:replace>new</cs:replace></item>",
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

	text := collectRenderText(r, all)
	t.Logf("replace_in_file render: %q", text)
	if !strings.Contains(text, "edit file") {
		t.Errorf("replace_in_file intent not rendered, got: %q", text)
	}
}
