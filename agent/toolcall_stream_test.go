// Author: L.Shuang
// Created: 2026-08-04
// Last Modified: 2026-08-05
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// toolcallTestTools returns a minimal tool list used by the streaming parsers.
func toolcallTestTools() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "write_to_file",
			Description: "write a file",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"mode":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name:        "replace_in_file",
			Description: "replace content in a file",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"path":         map[string]interface{}{"type": "string"},
					"replacements": map[string]interface{}{"type": "array"},
				},
			},
		},
		{
			Name:        "update_setting",
			Description: "update a setting",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"key":   map[string]interface{}{"type": "string"},
					"value": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
}

// collectRenderText applies a sequence of RenderOps to a renderer and returns
// the accumulated display text.
func collectRenderText(r *ToolCallRenderer, ops []RenderOp) string {
	var sb strings.Builder
	for _, op := range ops {
		r.Apply(op, func(text string) { sb.WriteString(text) })
	}
	return sb.String()
}

// TestToolCallStream_XMLToolNameFirst verifies UC-0001: the tool name header is
// emitted as soon as <cs:write_to_file> is recognised, before parameters flow.
func TestToolCallStream_XMLToolNameFirst(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:path>foo.go</cs:path>",
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
	if !strings.Contains(text, "⚙️ write_to_file") {
		t.Errorf("tool name header missing, got: %q", text)
	}
	if !strings.Contains(text, "   path: foo.go") {
		t.Errorf("path param missing, got: %q", text)
	}
}

// TestToolCallStream_XMLWriteContent verifies FEATURE-328 UC-0001: content
// fragments are rendered incrementally line by line with a "+" marker and an
// incrementing line number.
func TestToolCallStream_XMLWriteContent(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>line1\n",
		"line2</cs:content>",
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
	if !strings.Contains(text, "     1+ line1") || !strings.Contains(text, "     2+ line2") {
		t.Errorf("content not rendered line-by-line with line numbers, got: %q", text)
	}
	if !strings.Contains(text, "   content:") {
		t.Errorf("content title line missing, got: %q", text)
	}
}

// TestToolCallStream_XMLReplaceNoLineNo verifies FEATURE-328 UC-0002: without a
// start_line parameter, replace_in_file renders search/replace lines with
// "-"/"+" markers and no line numbers.
func TestToolCallStream_XMLReplaceNoLineNo(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:replace_in_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:replacements>",
		"<item><cs:search>alpha</cs:search><cs:replace>beta</cs:replace></item>",
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
	if !strings.Contains(text, "   - alpha") || !strings.Contains(text, "   + beta") {
		t.Errorf("search/replace lines missing, got: %q", text)
	}
	if strings.Contains(text, "- alpha") && strings.ContainsAny(text, "123456789") {
		// No line numbers should be present on diff lines when start_line is
		// not specified.
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "- alpha") && line != "   - alpha" {
				t.Errorf("unexpected line number on '-' line, got: %q", line)
			}
		}
	}
}

// TestToolCallStream_XMLReplaceStartLine verifies FEATURE-328 UC-0003: when a
// start_line parameter precedes the block, search/replace lines show the real
// line numbers starting at start_line.
func TestToolCallStream_XMLReplaceStartLine(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:replace_in_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:replacements>",
		"<item><cs:start_line>10</cs:start_line><cs:search>old1\nold2</cs:search><cs:replace>new1\nnew2</cs:replace></item>",
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
	if !strings.Contains(text, "   10- old1") || !strings.Contains(text, "   11- old2") {
		t.Errorf("search lines with start_line line numbers missing, got: %q", text)
	}
	if !strings.Contains(text, "   10+ new1") || !strings.Contains(text, "   11+ new2") {
		t.Errorf("replace lines with start_line line numbers missing, got: %q", text)
	}
}

// TestToolCallStream_JSONIncremental verifies UC-0004: the JSON parser emits
// param keys as soon as the key closes, AND streams value fragments as plain
// characters arrive (without waiting for the closing quote). A long parameter
// value that spans multiple delta chunks grows incrementally on screen.
func TestToolCallStream_JSONIncremental(t *testing.T) {
	p := NewJSONToolCallParser()
	r := NewToolCallRenderer(true, true)

	p.SetToolName("write_to_file")
	// The stream router emits OpToolStart when the name arrives.
	r.Apply(RenderOp{Kind: OpToolStart, Text: "write_to_file"}, func(string) {})

	// Simulate a long write_to_file "content" value streamed as small deltas.
	// Each delta ends mid-string (no closing quote yet) — the parser must emit
	// the plain characters immediately as OpValueFragment so the text grows
	// incrementally instead of waiting for the quote that closes the value.
	deltas := []string{
		`{"path":"a.go","content":"Hel`,
		`lo world, this is an incr`,
		`emental content fragm`,
		`ent!"}`,
	}
	var text strings.Builder
	var fragmentCount int
	for n, d := range deltas {
		ops, err := p.Feed(d)
		if err != nil {
			t.Fatalf("Feed(%q) unexpected error: %v", d, err)
		}
		for _, op := range ops {
			if op.Kind == OpValueFragment {
				fragmentCount++
			}
			r.Apply(op, func(s string) { text.WriteString(s) })
		}
		if n < len(deltas)-1 {
			// Before the last delta (value unfinished), at least one content
			// fragment should already have been emitted for this delta.
			if fragmentCount == 0 {
				t.Fatalf("delta %d: no value fragment emitted before value closed", n)
			}
		}
		fragmentCount = 0
	}

	if !r.haveToolHeader {
		t.Errorf("tool header not emitted")
	}
	if !strings.Contains(text.String(), "   path: a.go") {
		t.Errorf("path param not rendered incrementally, got: %q", text.String())
	}
	// The full content value must have been rendered.
	want := "Hello world, this is an incremental content fragment!"
	if !strings.Contains(text.String(), want) {
		t.Errorf("content value not fully rendered (got %q), want substring %q", text.String(), want)
	}
}

// TestToolCallStream_JSONNewlineDecode verifies FEATURE-328 UC-0004: a JSON
// "\n" escape in a content value is decoded to a real newline so the renderer
// splits it into separate lines with incrementing line numbers.
func TestToolCallStream_JSONNewlineDecode(t *testing.T) {
	p := NewJSONToolCallParser()
	r := NewToolCallRenderer(true, true)
	p.SetToolName("write_to_file")
	r.Apply(RenderOp{Kind: OpToolStart, Text: "write_to_file"}, func(string) {})

	deltas := []string{
		`{"path":"a.go","content":"line1\n`,
		`line2"}`,
	}
	var text strings.Builder
	for _, d := range deltas {
		ops, err := p.Feed(d)
		if err != nil {
			t.Fatalf("Feed(%q) unexpected error: %v", d, err)
		}
		for _, op := range ops {
			r.Apply(op, func(s string) { text.WriteString(s) })
		}
	}

	got := text.String()
	if !strings.Contains(got, "     1+ line1") || !strings.Contains(got, "     2+ line2") {
		t.Errorf("JSON \\n escape not decoded to separate lines, got: %q", got)
	}
}

// TestToolCallStream_XMLReplaceDiff verifies FEATURE-328 UC-0002 (rewritten):
// a replace block without start_line renders "-"/"+" lines with no numbers.
// The old "🔄 ... ──> ..." one-line diff format is replaced by git-diff style.
func TestToolCallStream_XMLReplaceDiff(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:replace_in_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:replacements>",
		"<item><cs:search>alpha</cs:search><cs:replace>beta</cs:replace></item>",
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
	if strings.Contains(text, "🔄") {
		t.Errorf("legacy one-line diff format should no longer be used, got: %q", text)
	}
	if !strings.Contains(text, "   - alpha") {
		t.Errorf("search '-' line missing, got: %q", text)
	}
	if !strings.Contains(text, "   + beta") {
		t.Errorf("replace '+' line missing, got: %q", text)
	}
}

// TestToolCallStream_RenderConsistency verifies UC-0006: XML and OpenAI modes
// render the same tool call to the same accumulated text.
func TestToolCallStream_RenderConsistency(t *testing.T) {
	xmlText := renderXMLWrite(t)
	jsonText := renderJSONWrite(t)

	// Both should contain the tool header and both recognised params.
	for name, text := range map[string]string{"xml": xmlText, "json": jsonText} {
		if !strings.Contains(text, "⚙️ write_to_file") {
			t.Errorf("%s: missing tool header: %q", name, text)
		}
		if !strings.Contains(text, "   path: a.go") || !strings.Contains(text, "   mode: new") {
			t.Errorf("%s: missing path/mode params: %q", name, text)
		}
	}
}

// renderXMLWrite renders a write_to_file call through the XML parser.
func renderXMLWrite(t *testing.T) string {
	t.Helper()
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)
	chunks := []string{
		"<cs:write_to_file>",
		"<cs:path>a.go</cs:path>",
		"<cs:mode>new</cs:mode>",
		"</cs:write_to_file>",
	}
	var all []RenderOp
	for _, c := range chunks {
		ops, err := p.Feed(c)
		if err != nil {
			t.Fatalf("XML Feed(%q) error: %v", c, err)
		}
		all = append(all, ops...)
	}
	return collectRenderText(r, all)
}

// renderJSONWrite renders the same write_to_file call through the JSON parser.
func renderJSONWrite(t *testing.T) string {
	t.Helper()
	p := NewJSONToolCallParser()
	r := NewToolCallRenderer(true, true)
	p.SetToolName("write_to_file")
	var sb strings.Builder
	r.Apply(RenderOp{Kind: OpToolStart, Text: "write_to_file"}, func(s string) { sb.WriteString(s) })
	ops, err := p.Feed(`{"path":"a.go","mode":"new"}`)
	if err != nil {
		t.Fatalf("JSON Feed error: %v", err)
	}
	for _, op := range ops {
		r.Apply(op, func(s string) { sb.WriteString(s) })
	}
	return sb.String()
}

// TestToolCallStream_XMLPlainTextAndPending verifies the XML content router
// contract: ordinary non-"cs:" text is emitted as OpPlainText (handed back to
// the normal LLM content output), and while a "cs:" tag is split across chunks
// the parser reports PendingToolCall so the router does not fall back to plain
// content rendering.
func TestToolCallStream_XMLPlainTextAndPending(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())

	// Ordinary prose outside any tool: emitted as OpPlainText.
	ops, err := p.Feed("这是普通内容，不调用工具。")
	if err != nil {
		t.Fatalf("Feed(prose) error: %v", err)
	}
	if len(ops) != 1 || ops[0].Kind != OpPlainText {
		t.Fatalf("expected one OpPlainText, got %v", ops)
	}
	if p.PendingToolCall() {
		t.Errorf("prose should not be a pending tool call")
	}

	// A "cs:" tag that is split across chunks: first chunk ends with an
	// unclosed "<cs:write", so the parser must report PendingToolCall and the
	// router must not fall back to plain content.
	ops, err = p.Feed("<cs:write")
	if err != nil {
		t.Fatalf("Feed('<cs:write') error: %v", err)
	}
	if !p.PendingToolCall() {
		t.Errorf("expected PendingToolCall while cs: tag unclosed")
	}
}

// TestToolCallStream_JSONParseError verifies UC-0007: an unmatched closing
// bracket triggers an immediate abort error in the JSON parser.
func TestToolCallStream_JSONParseError(t *testing.T) {
	p := NewJSONToolCallParser()
	p.SetToolName("write_to_file")

	// A balanced object must not error.
	if _, err := p.Feed(`{"path":"a.go"}`); err != nil {
		t.Fatalf("balanced JSON should not error, got: %v", err)
	}
	// An unmatched closing bracket (depth below zero) is a fatal error.
	if _, err := p.Feed(`]`); err == nil {
		t.Fatal("expected ToolCallParseError from stray closing bracket, got nil")
	}
}

// TestToolCallStream_XMLWriteGating verifies FEATURE-328 UC-0006: with
// showToolInput=false no content lines are rendered while the tool header is.
func TestToolCallStream_XMLWriteGating(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, false)

	chunks := []string{
		"<cs:write_to_file>",
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

	text := collectRenderText(r, all)
	if !strings.Contains(text, "⚙️ write_to_file") {
		t.Errorf("tool header missing with showToolInput=false, got: %q", text)
	}
	if strings.Contains(text, "+") || strings.Contains(text, "content:") {
		t.Errorf("content lines should be gated off, got: %q", text)
	}
}
