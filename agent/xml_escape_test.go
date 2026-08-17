// decodeXMLEscapes + XML literal-escape decoding tests (replace_in_file display/execution).
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

func TestDecodeXMLEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"literal backslash-n to newline", `a\nb`, "a\nb"},
		{"literal backslash-t to tab", `\t\t`, "\t\t"},
		{"double backslash to single", `C:\\Users`, `C:\Users`},
		{"double backslash-n preserved", `\\n`, `\n`},
		{"unknown escape preserved", `\U path`, `\U path`},
		{"quoted quote decoded", `a\"b`, `a"b`},
		{"no backslash unchanged", `plain`, `plain`},
		{"empty unchanged", ``, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeXMLEscapes(tc.in); got != tc.want {
				t.Fatalf("decodeXMLEscapes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseXMLReplaceLiteralNewlineEscapes verifies that when the LLM writes
// literal \n (backslash+n) inside <cs:search>/<cs:replace> instead of real
// newlines, the parsed arguments carry real newlines so the search matches the
// file's actual line endings.
func TestParseXMLReplaceLiteralNewlineEscapes(t *testing.T) {
	xmlInput := "<cs:replace_in_file>\n" +
		"<cs:path>test.md</cs:path>\n" +
		"<cs:replacements>\n" +
		"<cs:item><cs:search>line1\\nline2</cs:search><cs:replace>new1\\nnew2</cs:replace></cs:item>\n" +
		"</cs:replacements>\n" +
		"</cs:replace_in_file>"

	tools := []llm.Tool{{Name: "replace_in_file"}}
	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	repls, ok := args["replacements"].([]interface{})
	if !ok || len(repls) != 1 {
		t.Fatalf("replacements missing: %+v", args)
	}
	item, _ := repls[0].(map[string]interface{})
	search, _ := item["search"].(string)
	replace, _ := item["replace"].(string)
	if search != "line1\nline2" {
		t.Fatalf("search got %q, want %q", search, "line1\nline2")
	}
	if replace != "new1\nnew2" {
		t.Fatalf("replace got %q, want %q", replace, "new1\nnew2")
	}
}

// TestParseXMLReplaceRealNewlinesUnchanged verifies real newlines pass through
// unchanged (the normal model behaviour).
func TestParseXMLReplaceRealNewlinesUnchanged(t *testing.T) {
	xmlInput := "<cs:replace_in_file>\n" +
		"<cs:path>test.md</cs:path>\n" +
		"<cs:replacements>\n" +
		"<cs:item><cs:search>line1\nline2</cs:search><cs:replace>new1\nnew2</cs:replace></cs:item>\n" +
		"</cs:replacements>\n" +
		"</cs:replace_in_file>"

	tools := []llm.Tool{{Name: "replace_in_file"}}
	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	repls, _ := args["replacements"].([]interface{})
	item, _ := repls[0].(map[string]interface{})
	search, _ := item["search"].(string)
	if search != "line1\nline2" {
		t.Fatalf("search got %q, want %q", search, "line1\nline2")
	}
}

// TestToolCallStream_LeadingNewlineBeforeHeader verifies the first tool-call
// header is preceded by a newline so it is visually separated from the
// preceding LLM content.
func TestToolCallStream_LeadingNewlineBeforeHeader(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:path>a.txt</cs:path>",
		"<cs:content>hello</cs:content>",
		"</cs:write_to_file>",
	}
	var all []RenderOp
	for _, c := range chunks {
		ops, err := p.Feed(c)
		if err != nil {
			t.Fatalf("Feed(%q): %v", c, err)
		}
		all = append(all, ops...)
	}

	text := collectRenderText(r, all)
	if !strings.HasPrefix(text, "\n⚙️ write_to_file") {
		t.Fatalf("first tool header must start with a leading newline, got %q", text)
	}
	// Only one leading newline — subsequent headers are not double-spaced.
	if strings.Count(text, "\n⚙️ ") != 1 {
		t.Fatalf("expected exactly one leading-newline header, got %q", text)
	}
}
