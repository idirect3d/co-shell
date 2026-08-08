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
	"strconv"
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
	// FEATURE-338: line numbers are right-aligned to 5 digits, so single-digit
	// lines start with 9 spaces (5 indent + 4 pad) before "1+".
	if !strings.Contains(text, "        1+ line1") || !strings.Contains(text, "        2+ line2") {
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
	if !strings.Contains(text, "⚙️ replace_in_file a.go") {
		t.Errorf("tool header should share the path line, got: %q", text)
	}
	if !strings.Contains(text, "- alpha") || !strings.Contains(text, "+ beta") {
		t.Errorf("search/replace lines missing, got: %q", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "- alpha") && line != "- alpha" {
			t.Errorf("unexpected line number on '-' line, got: %q", line)
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
	if !strings.Contains(text, "10-: old1") || !strings.Contains(text, "11-: old2") {
		t.Errorf("search lines with start_line line numbers missing, got: %q", text)
	}
	if !strings.Contains(text, "10+: new1") || !strings.Contains(text, "11+: new2") {
		t.Errorf("replace lines with start_line line numbers missing, got: %q", text)
	}
	if !strings.Contains(text, "10-11") {
		t.Errorf("location header missing, got: %q", text)
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
	// FEATURE-338: single-digit lines use 9-space prefix (5 indent + 4 pad).
	if !strings.Contains(got, "        1+ line1") || !strings.Contains(got, "        2+ line2") {
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
	if !strings.Contains(text, "- alpha") {
		t.Errorf("search '-' line missing, got: %q", text)
	}
	if !strings.Contains(text, "+ beta") {
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

// TestToolCallStream_XMLChunkBoundaryLoneLessThan verifies FIX-331: when the
// '<' opening a prefixed tool tag is the LAST character of a chunk, it must be
// carried over to the next Feed (via partial) so the next chunk's "cs:write..."
// is still recognised as a tag. Before the fix, '<' was emitted as ordinary
// plain text and the subsequent "cs:write_to_file>" fragment was treated as
// plain content because it no longer started with '<' — corrupting the whole
// tool call structure (params merged, closing text leaked).
func TestToolCallStream_XMLChunkBoundaryLoneLessThan(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	// Simulate the LLM stream splitting "<cs:write_to_file>" as "<" + "cs:write_to_file>".
	chunks := []string{
		"<",
		"cs:write_to_file>",
		"<cs:path>a.go</cs:path>",
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
		t.Errorf("tool header missing when '<' spans chunk boundary, got: %q", text)
	}
	if !strings.Contains(text, "   path: a.go") {
		t.Errorf("path param missing when '<' spans chunk boundary, got: %q", text)
	}
	// The bare '<' must NOT leak into ordinary content.
	if strings.Contains(text, "<") && !strings.Contains(text, "a.go") {
		t.Errorf("stray '<' leaked into content when '<' spans chunk boundary, got: %q", text)
	}
}

// TestToolCallStream_XMLChunkBoundaryPartialPrefix verifies FIX-331: when the
// tag prefix itself is split at the chunk boundary (e.g. "<c" then "s:write..."),
// the parser must also carry the partial prefix to the next Feed.
func TestToolCallStream_XMLChunkBoundaryPartialPrefix(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<c",
		"s:write_to_file>",
		"<cs:path>a.go</cs:path>",
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
		t.Errorf("tool header missing when prefix spans chunk boundary, got: %q", text)
	}
	if !strings.Contains(text, "   path: a.go") {
		t.Errorf("path param missing when prefix spans chunk boundary, got: %q", text)
	}
}

// assertPlusAligned verifies FEATURE-338: every write_to_file content line in
// text has its "+" marker at the same column. Lines follow "         N+ content"
// with the line number right-aligned to 5 digits.
func assertPlusAligned(t *testing.T, text string) {
	t.Helper()
	plusCol := -1
	for _, line := range strings.Split(text, "\n") {
		idx := strings.Index(line, "+")
		if idx < 0 {
			continue
		}
		// Only consider write_to_file content lines: prefix is 5 spaces +
		// right-aligned 5-digit line number, so "+" sits at the same column.
		if !strings.HasPrefix(line, "     ") {
			continue
		}
		if plusCol < 0 {
			plusCol = idx
			continue
		}
		if idx != plusCol {
			t.Errorf("'+' column drifted: line %q has + at col %d, want col %d", line, idx, plusCol)
		}
	}
}

// buildContentLines builds a multi-line content string with n lines.
func buildContentLines(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		sb.WriteString("line" + strconv.Itoa(i))
		if i < n {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// TestToolCallStream_XMLWriteAlignSingleDigit verifies FEATURE-338 UC-0001:
// 1~9 line content renders "+" under the same column for every line.
func TestToolCallStream_XMLWriteAlignSingleDigit(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>" + buildContentLines(6) + "</cs:content>",
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
	for i := 1; i <= 6; i++ {
		want := "         " + strconv.Itoa(i) + "+ "
		if !strings.Contains(text, want) {
			t.Errorf("line %d prefix mismatch, want %q in %q", i, want, text)
		}
	}
	assertPlusAligned(t, text)
}

// TestToolCallStream_XMLWriteAlignTwoDigit verifies FEATURE-338 UC-0002:
// 10~99 line content right-aligns 1-digit line numbers with one pad space.
func TestToolCallStream_XMLWriteAlignTwoDigit(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>" + buildContentLines(12) + "</cs:content>",
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
	// 1..9 right-aligned to 5 digits: "         N+ " (9 spaces before 1-digit).
	if !strings.Contains(text, "        1+ ") || !strings.Contains(text, "        9+ ") {
		t.Errorf("single-digit lines not right-aligned, got: %q", text)
	}
	// 10..99: "        NN+ " (8 spaces before 2-digit).
	if !strings.Contains(text, "       10+ ") || !strings.Contains(text, "       12+ ") {
		t.Errorf("two-digit lines not right-aligned, got: %q", text)
	}
	assertPlusAligned(t, text)
}

// TestToolCallStream_XMLWriteAlignBoundary verifies FEATURE-338 UC-0003:
// crossing the 9→10 boundary mid-stream keeps the "+" column stable.
func TestToolCallStream_XMLWriteAlignBoundary(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	// Split the content so line 9 and line 10 arrive in different chunks.
	content9 := buildContentLines(9)
	line10 := "line10"
	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>" + content9 + "\n",
		line10 + "</cs:content>",
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
	if !strings.Contains(text, "        9+ line9") || !strings.Contains(text, "       10+ line10") {
		t.Errorf("boundary lines missing or misaligned, got: %q", text)
	}
	assertPlusAligned(t, text)
}

// TestToolCallStream_XMLWriteAlignThreeDigit verifies FEATURE-338 UC-0004:
// 100~999 line content right-aligns to 3-digit width without extra padding.
func TestToolCallStream_XMLWriteAlignThreeDigit(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, true)

	// Use 101 lines so lines 100 and 101 cross into three digits.
	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>" + buildContentLines(101) + "</cs:content>",
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
	// 1..9: 9 spaces before; 10..99: 8 spaces before; 100..101: 7 spaces before.
	if !strings.Contains(text, "        1+ ") {
		t.Errorf("1-digit line not padded, got: %q", text)
	}
	if !strings.Contains(text, "       10+ ") {
		t.Errorf("2-digit line not padded, got: %q", text)
	}
	if !strings.Contains(text, "      100+ ") || !strings.Contains(text, "      101+ ") {
		t.Errorf("3-digit lines not aligned, got: %q", text)
	}
	assertPlusAligned(t, text)
}

// TestToolCallStream_XMLReplaceColonUnaffected verifies FEATURE-338 UC-0005:
// replace_in_file with start_line keeps the legacy "N-:"/"N+:" format — the
// 3-digit right-alignment applies ONLY to non-colon (write_to_file) lines.
func TestToolCallStream_XMLReplaceColonUnaffected(t *testing.T) {
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
	if !strings.Contains(text, "10-: old1") || !strings.Contains(text, "11-: old2") {
		t.Errorf("colon search lines changed, got: %q", text)
	}
	if !strings.Contains(text, "10+: new1") || !strings.Contains(text, "11+: new2") {
		t.Errorf("colon replace lines changed, got: %q", text)
	}
	if !strings.Contains(text, "10-11") {
		t.Errorf("location header missing, got: %q", text)
	}
}

// TestToolCallStream_XMLWriteAlignGating verifies FEATURE-338 UC-0006:
// with showToolInput=false no content lines render (gating unchanged).
func TestToolCallStream_XMLWriteAlignGating(t *testing.T) {
	p := NewXMLToolCallParser(toolcallTestTools())
	r := NewToolCallRenderer(true, false)

	chunks := []string{
		"<cs:write_to_file>",
		"<cs:content>" + buildContentLines(11) + "</cs:content>",
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
