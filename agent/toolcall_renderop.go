// Author: L.Shuang
// Created: 2026-08-04
// Last Modified: 2026-08-05
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// RenderOpKind identifies the kind of a unified tool-call render operation.
// Both the XML and the OpenAI JSON streaming parsers emit the same RenderOp
// sequence, so the renderer is a single place that does not know the source.
type RenderOpKind int

const (
	// OpPlainText marks a fragment of ordinary LLM content that sits outside
	// any tool call. The content router emits it via the normal
	// EventContentChunk path so it is shown verbatim without tool styling.
	OpPlainText RenderOpKind = iota
	// OpToolStart marks the beginning of a recognised tool call.
	// Text holds the tool name (e.g. "write_to_file").
	OpToolStart
	// OpParamKey marks the start of a parameter.
	// Text holds the parameter name (e.g. "path", "content", "search").
	OpParamKey
	// OpValueFragment is a fragment of the current parameter value.
	// Text holds an arbitrary piece of the value string.
	OpValueFragment
	// OpParamEnd marks the end of the current parameter value.
	// Text holds the parameter name.
	OpParamEnd
	// OpToolEnd marks the closing of the current tool call.
	OpToolEnd
)

// RenderOp is a unified tool-call render operation emitted by a streaming
// parser (XML FSA or streaming JSON tokenizer). The ToolCallRenderer consumes
// the stream and produces user-facing text.
type RenderOp struct {
	Kind RenderOpKind
	Text string
}

// ToolCallRenderer renders a stream of RenderOp into user-facing incremental
// text. It is the single rendering entry point shared by XML and OpenAI modes.
//
// The renderer is display-gated:
//   - OpToolStart is emitted only when showTool is enabled.
//   - parameter keys / value fragments are emitted only when showToolInput is
//     enabled.
//
// Method-specific rendering (FEATURE-328, git-diff style):
//   - write_to_file: the "content" value is rendered line by line with an
//     incrementing line number and a "+" marker (e.g. "     1+ foo"), so the
//     file being written visibly grows line by line on screen.
//   - replace_in_file: search value fragments are rendered line by line with a
//     "-" marker; replace value fragments with a "+" marker. When a
//     "start_line" parameter precedes the block inside the same replacement,
//     both markers are prefixed with the real line numbers starting at
//     start_line; otherwise no line numbers are shown.
type ToolCallRenderer struct {
	showTool       bool
	showToolInput  bool
	currentTool    string
	pendingParam   string
	haveToolHeader bool

	writeLineBuf strings.Builder // line-mode buffer for write_to_file content
	writeLineNo  int

	replaceStartLine     int // 0 = not specified (no line numbers)
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
// The content router uses this to decide whether raw LLM content should keep
// flowing to the user or be diverted into the tool-call rendering path.
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
		// A nested tool call re-uses the same renderer: close the previous
		// one first so the second call is visually separated.
		if r.currentTool != "" {
			r.emitToolEnd(emit)
		}
		r.currentTool = op.Text
		r.pendingParam = ""
		if r.showTool {
			r.haveToolHeader = true
			emit("⚙️ " + op.Text + "\n")
		}
	case OpParamKey:
		r.pendingParam = op.Text
		if !r.showToolInput {
			return
		}
		// replace_in_file values are rendered inside diff blocks, not as
		// "key: value" lines; the diff lines are emitted at OpValueFragment.
		if r.currentTool == "replace_in_file" {
			switch r.pendingParam {
			case "search", "replace", "start_line":
				return
			}
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
			case "search":
				feedLined(&r.replaceSearchBuf, r.replaceStartLine, &r.replaceSearchLineNo, "-", "   ", op.Text, emit)
				return
			case "replace":
				feedLined(&r.replaceReplaceBuf, r.replaceStartLine, &r.replaceReplaceLineNo, "+", "   ", op.Text, emit)
				return
			case "start_line":
				r.startLineBuf.WriteString(op.Text)
				return
			}
		}
		if r.currentTool == "write_to_file" && r.pendingParam == "content" {
			feedLined(&r.writeLineBuf, 1, &r.writeLineNo, "+", "     ", op.Text, emit)
			return
		}
		// FEATURE-235: Emit the value fragment verbatim in the granularity the
		// underlying parser produced (which follows the LLM stream chunks).
		emit(op.Text)
	case OpParamEnd:
		r.finaliseParameter(emit)
	case OpToolEnd:
		r.emitToolEnd(emit)
	}
}

// finaliseParameter is called when a parameter value ends (OpParamEnd).
// It flushes trailing line-mode buffers and resets per-block state.
func (r *ToolCallRenderer) finaliseParameter(emit func(text string)) {
	switch r.currentTool {
	case "write_to_file":
		if r.pendingParam == "content" {
			flushLined(&r.writeLineBuf, 1, &r.writeLineNo, "+", "     ", emit)
			r.pendingParam = ""
			return
		}
	case "replace_in_file":
		switch r.pendingParam {
		case "search":
			flushLined(&r.replaceSearchBuf, r.replaceStartLine, &r.replaceSearchLineNo, "-", "   ", emit)
			r.replaceHaveSearch = true
			r.replacePairClosed = false
			r.pendingParam = ""
			return
		case "replace":
			flushLined(&r.replaceReplaceBuf, r.replaceStartLine, &r.replaceReplaceLineNo, "+", "   ", emit)
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
		// Each regular parameter value ends on its own line.
		emit("\n")
	}
	r.pendingParam = ""
}

// emitToolEnd finishes the current tool line. It emits a trailing newline so
// the next tool call (or subsequent prose) starts on a fresh line.
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
// the marker ("+"/"-") is shown. indent aligns the line under its block:
// write_to_file content uses 5 spaces, replace_in_file diff lines use 3.
func linePrefix(baseLine, no int, marker, indent string) string {
	if baseLine > 0 {
		return fmt.Sprintf("%s%d%s ", indent, baseLine+no-1, marker)
	}
	return fmt.Sprintf("%s%s ", indent, marker)
}

// feedLined appends text to buf and emits every completed line (split on '\n')
// immediately. The line counter increments for each emitted line, so a long
// parameter value visibly grows line by line on screen.
func feedLined(buf *strings.Builder, baseLine int, lineNo *int, marker, indent, text string, emit func(string)) {
	buf.WriteString(text)
	for {
		s := buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			return
		}
		line := s[:idx]
		*lineNo++
		emit(linePrefix(baseLine, *lineNo, marker, indent) + line + "\n")
		buf.Reset()
		buf.WriteString(s[idx+1:])
	}
}

// flushLined emits the remaining (newline-less) tail of a line buffer when a
// parameter ends. Empty buffers emit nothing.
func flushLined(buf *strings.Builder, baseLine int, lineNo *int, marker, indent string, emit func(string)) {
	if buf.Len() == 0 {
		return
	}
	line := buf.String()
	*lineNo++
	emit(linePrefix(baseLine, *lineNo, marker, indent) + line + "\n")
	buf.Reset()
}

// trimToolCallContent is a small helper used by tests and future extensions.
func trimToolCallContent(s string) string {
	return strings.TrimSpace(s)
}
