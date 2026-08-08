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
}

// NewToolCallRenderer constructs a renderer gated by showTool and showToolInput.
func NewToolCallRenderer(showTool, showToolInput bool) *ToolCallRenderer {
	return &ToolCallRenderer{
		showTool:      showTool,
		showToolInput: showToolInput,
	}
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
				emit("⚙️ " + op.Text + "\n")
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
		// by its own "content:" title line.
		if r.currentTool == "write_to_file" && r.pendingParam == "content" {
			emit("   content:\n")
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
				feedLined(&r.replaceSearchBuf, r.replaceStartLine, &r.replaceSearchLineNo, "-", "", true, op.Text, emit)
				return
			case "replace":
				r.flushReplaceHeader(emit)
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
		// Emit the value fragment verbatim in the granularity the underlying
		// parser produced.
		emit(op.Text)
	case OpParamEnd:
		r.finaliseParameter(emit)
	case OpToolEnd:
		r.emitToolEnd(emit)
	}
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
	emit(h + "\n")
	if r.replaceIntent != "" {
		emit("(" + r.replaceIntent + ")\n")
	}
}

// finaliseParameter is called when a parameter value ends (OpParamEnd).
func (r *ToolCallRenderer) finaliseParameter(emit func(text string)) {
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
			flushLined(&r.replaceReplaceBuf, r.replaceStartLine, &r.replaceReplaceLineNo, "+", "", true, emit)
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
	if r.showTool {
		emit("\n")
	}
	r.currentTool = ""
	r.pendingParam = ""
	r.haveToolHeader = false
	r.writeLineBuf.Reset()
	r.writeLineNo = 0
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
}

// linePrefix builds the git-diff style prefix for a rendered line. When
// baseLine > 0 the real line number (baseLine+no-1) is shown; otherwise only
// the marker ("+"/"-") is shown. indent aligns the line under its block
// (write_to_file content uses 5 spaces; replace_in_file diff lines are flush
// left). When colon is true the marker is glued to the line number followed
// by a colon ("5-: "); otherwise a plain space follows ("   1+ ").
func linePrefix(baseLine, no int, marker, indent string, colon bool) string {
	if baseLine > 0 {
		if colon {
			return fmt.Sprintf("%s%d%s: ", indent, baseLine+no-1, marker)
		}
		return fmt.Sprintf("%s%d%s ", indent, baseLine+no-1, marker)
	}
	return fmt.Sprintf("%s%s ", indent, marker)
}

// feedLined appends text to buf and emits every completed line (split on '\n')
// immediately. The line counter increments for each emitted line.
func feedLined(buf *strings.Builder, baseLine int, lineNo *int, marker, indent string, colon bool, text string, emit func(string)) {
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

// trimToolCallContent is a small helper used by tests and future extensions.
func trimToolCallContent(s string) string {
	return strings.TrimSpace(s)
}
