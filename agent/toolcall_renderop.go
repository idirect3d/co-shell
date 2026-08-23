// Author: L.Shuang
// Created: 2026-08-04
// Last Modified: 2026-08-05
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// RenderOpKind identifies the kind of a unified tool-call render operation.
type RenderOpKind int

const (
	OpPlainText RenderOpKind = iota
	OpToolStart
	OpParamKey
	OpValueFragment
	OpParamEnd
	OpToolEnd
)

// RenderOp is a unified tool-call render operation emitted by a streaming
// parser (XML FSA or streaming JSON tokenizer).
type RenderOp struct {
	Kind RenderOpKind
	Text string
}

// ToolCallRenderer renders a stream of RenderOp into user-facing incremental
// text. Display-gated by showTool/showToolInput.
//
// Method-specific rendering (FEATURE-328, git-diff style):
//   - write_to_file "content": line by line with incrementing line number and
//     "+" marker ("     1+ foo").
//   - replace_in_file: deferred header "⚙️ replace_in_file <path>" on line 1,
//     "(intent)" on line 2, then top-level diff lines: search "-" / replace
//     "+". With start_line the lines are prefixed with real line numbers glued
//     to the marker ("5-: ", "5+: ") and a location header "5-6 行: " is
//     emitted after the search block. The "replacements" array wrapper and its
//     <item>/tag noise are not rendered.
type ToolCallRenderer struct {
	showTool       bool
	showToolInput  bool
	currentTool    string
	pendingParam   string
	haveToolHeader bool

	writeLineBuf strings.Builder
	writeLineNo  int

	// intent accumulates the current tool's intent value so it can be rendered
	// as "(<intent>)" uniformly across tools (FEATURE-424).
	intent string

	replaceHeaderPending bool
	replaceHeaderPath    string
	replaceIntent        string

	replaceStartLine     int
	startLineBuf         strings.Builder
	replaceSearchBuf     strings.Builder
	replaceSearchLineNo  int
	replaceHaveSearch    bool
	replaceReplaceBuf    strings.Builder
	replaceReplaceLineNo int
	replacePairClosed    bool

	// replaceSearchAccum / replaceReplaceAccum accumulate the full search and
	// replace text across streaming fragments (feedLined resets the render
	// buffers line by line, so a separate accumulator is needed for the diff,
	// FEATURE-424).
	replaceSearchAccum  strings.Builder
	replaceReplaceAccum strings.Builder

	// replaceSearchContent is retained for the diff computation when the
	// replace parameter ends (FEATURE-424).
	replaceSearchContent string

	// leadingEmitted tracks whether a leading newline has been emitted for the
	// current stream call, so the first tool header is visually separated from
	// the preceding LLM content exactly once.
	leadingEmitted bool

	// diffEmit, when set, receives the FEATURE-424 unified diff rendering text
	// for a replace_in_file call once its search/replace blocks are complete.
	// The frontend uses it to re-render the params sub-block with per-line
	// add/delete/unchanged status (green/red/default).
	diffEmit func(string)

	// diffText accumulates the unified diff rendering for the current
	// replace_in_file call (computed at emitToolEnd).
	diffText strings.Builder
}

// NewToolCallRenderer constructs a renderer gated by showTool and showToolInput.
func NewToolCallRenderer(showTool, showToolInput bool) *ToolCallRenderer {
	return &ToolCallRenderer{
		showTool:      showTool,
		showToolInput: showToolInput,
	}
}

// SetDiffEmit installs a callback that receives the unified diff rendering
// text for a replace_in_file call once its search/replace blocks complete
// (FEATURE-424).
func (r *ToolCallRenderer) SetDiffEmit(fn func(string)) {
	r.diffEmit = fn
}

// InToolCall reports whether the renderer is currently inside a tool call.
func (r *ToolCallRenderer) InToolCall() bool {
	return r.currentTool != ""
}

// Reset clears the renderer state for a new stream call.
func (r *ToolCallRenderer) Reset() {
	r.currentTool = ""
	r.pendingParam = ""
	r.haveToolHeader = false
	r.writeLineBuf.Reset()
	r.writeLineNo = 0
	r.intent = ""
	r.replaceHeaderPending = false
	r.replaceHeaderPath = ""
	r.replaceIntent = ""
	r.replaceStartLine = 0
	r.startLineBuf.Reset()
	r.replaceSearchBuf.Reset()
	r.replaceSearchLineNo = 0
	r.replaceHaveSearch = false
	r.replaceReplaceBuf.Reset()
	r.replaceReplaceLineNo = 0
	r.replacePairClosed = false
	r.replaceSearchContent = ""
	r.replaceSearchAccum.Reset()
	r.replaceReplaceAccum.Reset()
	r.leadingEmitted = false
	r.diffText.Reset()
}

// Apply consumes one RenderOp and emits the incremental display text through
// emit. The emitted text is a delta: it must not repeat anything already
// emitted for a previous op.
func (r *ToolCallRenderer) Apply(op RenderOp, emit func(text string)) {
	switch op.Kind {
	case OpToolStart:
		if r.currentTool != "" {
			r.emitToolEnd(emit)
		}
		r.currentTool = op.Text
		r.pendingParam = ""
		if r.showTool {
			r.haveToolHeader = true
			if r.currentTool == "replace_in_file" && r.showToolInput {
				// Defer the header until the path is known so it can share
				// the line ("⚙️ replace_in_file 作文.md").
				r.replaceHeaderPending = true
			} else {
				r.emitToolHeader(emit, "⚙️ "+op.Text+"\n")
			}
		}
	case OpParamKey:
		r.pendingParam = op.Text
		if !r.showToolInput {
			return
		}
		if r.currentTool == "replace_in_file" {
			if r.pendingParam == "replacements" {
				// The array wrapper produces no rendered line; flush the
				// deferred header before the diff lines begin.
				r.flushReplaceHeader(emit)
				return
			}
			switch r.pendingParam {
			case "path", "intent", "search", "replace", "start_line":
				return
			}
			r.flushReplaceHeader(emit)
		}
		// write_to_file content is rendered as a line-numbered block headed
		// by its own "content:" title line. The intent is rendered uniformly
		// as "(<intent>)" when the intent parameter ends (finaliseParameter).
		if r.currentTool == "write_to_file" && r.pendingParam == "content" {
			emit("   content:\n")
			return
		}
		// FEATURE-424: intent is rendered uniformly as "(<intent>)" (matching
		// replace_in_file), so it is not shown as a separate "intent:" param
		// line for any tool.
		if r.pendingParam == "intent" {
			return
		}
		emit("   " + op.Text + ": ")
	case OpValueFragment:
		if !r.showToolInput || op.Text == "" {
			return
		}
		if r.currentTool == "replace_in_file" {
			switch r.pendingParam {
			case "replacements":
				// Array wrapper noise (<item>...</item> text) is not rendered.
				return
			case "path":
				r.replaceHeaderPath += op.Text
				return
			case "intent":
				r.replaceIntent += op.Text
				return
			case "search":
				r.flushReplaceHeader(emit)
				r.replaceSearchAccum.WriteString(op.Text)
				feedLined(&r.replaceSearchBuf, r.replaceStartLine, &r.replaceSearchLineNo, "-", "", true, op.Text, emit)
				return
			case "replace":
				r.flushReplaceHeader(emit)
				r.replaceReplaceAccum.WriteString(op.Text)
				feedLined(&r.replaceReplaceBuf, r.replaceStartLine, &r.replaceReplaceLineNo, "+", "", true, op.Text, emit)
				return
			case "start_line":
				r.startLineBuf.WriteString(op.Text)
				return
			}
		}
		if r.currentTool == "write_to_file" && r.pendingParam == "content" {
			feedLined(&r.writeLineBuf, 1, &r.writeLineNo, "+", "     ", false, op.Text, emit)
			return
		}
		// FEATURE-424: accumulate the intent so it can be shown as "(<intent>)"
		// uniformly across tools.
		if r.pendingParam == "intent" {
			r.intent += op.Text
			return
		}
		// Emit the value fragment verbatim in the granularity the underlying
		// parser produced.
		emit(op.Text)
	case OpParamEnd:
		r.finaliseParameter(emit)
	case OpToolEnd:
		r.emitToolEnd(emit)
	}
}

// emitToolHeader emits a tool-call header line, prefixing a leading newline
// before the first header of a stream call so the tool call is visually
// separated from the preceding LLM content.
func (r *ToolCallRenderer) emitToolHeader(emit func(text string), text string) {
	prefix := ""
	if !r.leadingEmitted {
		prefix = "\n"
		r.leadingEmitted = true
	}
	emit(prefix + text)
}

// flushReplaceHeader emits the deferred replace_in_file tool header and the
// intent line once the parameter stream reaches the diff section (or an
// unknown parameter). The header consumes the path value
// ("⚙️ replace_in_file 作文.md") and the intent is rendered on the second line
// in parentheses.
func (r *ToolCallRenderer) flushReplaceHeader(emit func(text string)) {
	if !r.replaceHeaderPending {
		return
	}
	r.replaceHeaderPending = false
	h := "⚙️ replace_in_file"
	if r.replaceHeaderPath != "" {
		h += " " + r.replaceHeaderPath
	}
	r.emitToolHeader(emit, h+"\n")
	if r.replaceIntent != "" {
		emit("(" + r.replaceIntent + ")\n")
	}
}

// finaliseParameter is called when a parameter value ends (OpParamEnd).
func (r *ToolCallRenderer) finaliseParameter(emit func(text string)) {
	// FEATURE-424: when the intent parameter ends, render it uniformly as
	// "(<intent>)" for every tool (matching replace_in_file).
	if r.pendingParam == "intent" {
		if r.intent != "" {
			emit("(" + r.intent + ")\n")
		}
		r.intent = ""
		r.pendingParam = ""
		return
	}
	switch r.currentTool {
	case "write_to_file":
		if r.pendingParam == "content" {
			flushLined(&r.writeLineBuf, 1, &r.writeLineNo, "+", "     ", false, emit)
			r.pendingParam = ""
			return
		}
	case "replace_in_file":
		switch r.pendingParam {
		case "replacements", "path", "intent":
			r.pendingParam = ""
			return
		case "search":
			flushLined(&r.replaceSearchBuf, r.replaceStartLine, &r.replaceSearchLineNo, "-", "", true, emit)
			if r.replaceStartLine > 0 {
				// Location header: "10-11 行: " (start-end of the search block).
				endLine := r.replaceStartLine + r.replaceSearchLineNo - 1
				emit(i18n.TF(i18n.KeyXMLStreamLineRange, r.replaceStartLine, endLine))
			}
			r.replaceHaveSearch = true
			r.replacePairClosed = false
			r.pendingParam = ""
			return
		case "replace":
			// FEATURE-424: compute the unified diff from the accumulated full
			// search/replace content (the render buffers are reset line by line
			// by feedLined, so the accumulators hold the complete text).
			flushLined(&r.replaceReplaceBuf, r.replaceStartLine, &r.replaceReplaceLineNo, "+", "", true, emit)
			if d := buildDiffText(r.replaceSearchAccum.String(), r.replaceReplaceAccum.String(), r.replaceStartLine); d != "" {
				r.diffText.WriteString(d)
			}
			// The block is finished: reset per-block state so the next
			// replacement starts fresh with no residual line numbers.
			r.replaceHaveSearch = false
			r.replaceSearchBuf.Reset()
			r.replaceSearchLineNo = 0
			r.replaceReplaceBuf.Reset()
			r.replaceReplaceLineNo = 0
			r.replaceStartLine = 0
			r.replacePairClosed = true
			r.pendingParam = ""
			return
		case "start_line":
			if val, err := strconv.Atoi(strings.TrimSpace(r.startLineBuf.String())); err == nil && val > 0 {
				r.replaceStartLine = val
			}
			r.startLineBuf.Reset()
			r.pendingParam = ""
			return
		}
	}
	if r.showToolInput {
		emit("\n")
	}
	r.pendingParam = ""
}

// emitToolEnd finishes the current tool line.
func (r *ToolCallRenderer) emitToolEnd(emit func(text string)) {
	if r.currentTool == "" {
		return
	}
	// FEATURE-424: for a completed replace_in_file call, hand the accumulated
	// unified diff rendering (per-line add/delete/unchanged status) to the
	// diffEmit callback so the frontend can re-render the params sub-block with
	// green/red/default colours.
	if r.currentTool == "replace_in_file" && r.diffEmit != nil && r.diffText.Len() > 0 {
		r.diffEmit(r.diffText.String())
	}
	if r.showTool {
		emit("\n")
	}
	r.currentTool = ""
	r.pendingParam = ""
	r.haveToolHeader = false
	r.writeLineBuf.Reset()
	r.writeLineNo = 0
	r.intent = ""
	r.replaceHeaderPending = false
	r.replaceHeaderPath = ""
	r.replaceIntent = ""
	r.replaceStartLine = 0
	r.startLineBuf.Reset()
	r.replaceSearchBuf.Reset()
	r.replaceSearchLineNo = 0
	r.replaceHaveSearch = false
	r.replaceReplaceBuf.Reset()
	r.replaceReplaceLineNo = 0
	r.replacePairClosed = false
	r.replaceSearchAccum.Reset()
	r.replaceReplaceAccum.Reset()
	r.diffText.Reset()
}

// linePrefix builds the git-diff style prefix for a rendered line. When
// baseLine > 0 the real line number (baseLine+no-1) is shown; otherwise only
// the marker ("+"/"-") is shown. indent aligns the line under its block
// (write_to_file content uses 5 spaces; replace_in_file diff lines are flush
// left). When colon is true the marker is glued to the line number followed
// by a colon ("5-: "); otherwise a plain space follows ("   1+ ").
//
// For non-colon lines (write_to_file content, FEATURE-338) the line number is
// right-aligned to a fixed 5-digit width so the "+" marker always stays in the
// same column regardless of how many digits the line number has — including
// mid-stream growth from 9 to 10 or 99 to 100 lines (the prefix width never
// changes once a line of a new digit count is emitted).
func linePrefix(baseLine, no int, marker, indent string, colon bool) string {
	if baseLine > 0 {
		if colon {
			return fmt.Sprintf("%s%d%s: ", indent, baseLine+no-1, marker)
		}
		return fmt.Sprintf("%s%5d%s ", indent, baseLine+no-1, marker)
	}
	return fmt.Sprintf("%s%s ", indent, marker)
}

// feedLined appends text to buf and emits every completed line (split on '\n')
// immediately. The line counter increments for each emitted line.
func feedLined(buf *strings.Builder, baseLine int, lineNo *int, marker, indent string, colon bool, text string, emit func(string)) {
	if strings.Contains(text, `\`) {
		log.Debug("toolcall feedLined: marker=%s text=%q (contains backslash)", marker, text)
	}
	buf.WriteString(text)
	for {
		s := buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			return
		}
		line := s[:idx]
		*lineNo++
		emit(linePrefix(baseLine, *lineNo, marker, indent, colon) + line + "\n")
		buf.Reset()
		buf.WriteString(s[idx+1:])
	}
}

// flushLined emits the remaining (newline-less) tail of a line buffer when a
// parameter ends. Empty buffers emit nothing.
func flushLined(buf *strings.Builder, baseLine int, lineNo *int, marker, indent string, colon bool, emit func(string)) {
	if buf.Len() == 0 {
		return
	}
	line := buf.String()
	*lineNo++
	emit(linePrefix(baseLine, *lineNo, marker, indent, colon) + line + "\n")
	buf.Reset()
}

// buildReplaceDiff computes the unified diff rendering for a completed
// replace_in_file call (FEATURE-424). It diffs the search and replace blocks
// line by line: lines present in both are marked unchanged (" "), lines only
// in search are marked deleted ("-"), and lines only in replace are marked
// added ("+"). Each line is rendered as "{1 space}{5-digit right-aligned line
// number}{status} {content}". Line numbers start at replaceStartLine (or 1
// when no start_line was given). Returns "" when there is nothing to diff.
func (r *ToolCallRenderer) buildReplaceDiff() string {
	return buildDiffText(r.replaceSearchBuf.String(), r.replaceReplaceBuf.String(), r.replaceStartLine)
}

// buildDiffText computes the unified diff rendering for a search/replace pair
// (FEATURE-424). It diffs the two blocks line by line: lines present in both
// are marked unchanged (" "), lines only in search are marked deleted ("-"),
// and lines only in replace are marked added ("+"). Each line is rendered as
// "{1 space}{5-digit right-aligned line number}{status} {content}". Line
// numbers start at startLine (or 1 when startLine <= 0). Returns "" when there
// is nothing to diff.
func buildDiffText(search, replace string, startLine int) string {
	search = strings.TrimSuffix(search, "\n")
	replace = strings.TrimSuffix(replace, "\n")
	if search == "" && replace == "" {
		return ""
	}
	searchLines := splitLines(search)
	replaceLines := splitLines(replace)

	// LCS table to align unchanged lines between search and replace.
	n, m := len(searchLines), len(replaceLines)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if searchLines[i] == replaceLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	base := startLine
	if base <= 0 {
		base = 1
	}
	var sb strings.Builder
	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && searchLines[i] == replaceLines[j] {
			// Unchanged line: present in both search and replace.
			sb.WriteString(diffLine(base+i, " ", searchLines[i]))
			i++
			j++
		} else if j < m && (i >= n || lcs[i][j+1] >= lcs[i+1][j]) {
			// Added line: only in replace.
			sb.WriteString(diffLine(base+i, "+", replaceLines[j]))
			j++
		} else {
			// Deleted line: only in search.
			sb.WriteString(diffLine(base+i, "-", searchLines[i]))
			i++
		}
	}
	return sb.String()
}

// diffLine renders one unified-diff line: "{1 space}{5-digit right-aligned
// line number}{status} {content}" (FEATURE-424).
func diffLine(lineNo int, status, content string) string {
	return fmt.Sprintf(" %5d%s %s\n", lineNo, status, content)
}

// trimToolCallContent is a small helper used by tests and future extensions.
func trimToolCallContent(s string) string {
	return strings.TrimSpace(s)
}
