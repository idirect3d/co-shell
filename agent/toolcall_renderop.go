// Author: L.Shuang
// Created: 2026-08-04
// Last Modified: 2026-08-04
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

import "strings"

// RenderOpKind identifies the kind of a unified tool-call render operation.
// Both the XML and the OpenAI JSON streaming parsers emit the same RenderOp
// sequence, so the renderer is a single place that does not know the source.
type RenderOpKind int

const (
	// OpPlainText marks a fragment of ordinary LLM content that sits outside
	// any tool call (prose, HTML-like text, etc.). The content router emits it
	// via the normal EventContentChunk path so it is shown verbatim without
	// the tool-render styling.
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
	// Text holds the parameter name. It lets the renderer finalise
	// method-specific rendering (e.g. a replace_in_file diff block needs to
	// know where the search value ends and the replace value begins).
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
//   - OpToolStart is emitted only when showTool is enabled (the method name
//     line is an Emoji-guided header controlled by show-tool).
//   - parameter keys / value fragments are emitted only when showToolInput is
//     enabled (the dynamic call content is controlled by show-tool-input).
//
// The renderer accumulates state so that fragments of the same value are
// printed continuously without repeating already-rendered parts.
//
// Method-specific rendering:
//   - write_to_file: the "content" parameter value is appended verbatim as it
//     streams (complete expansion of the file content being written).
//   - replace_in_file: every search/replace value pair is finalised at
//     OpParamEnd into a "SEARCH ──> REPLACE" diff block.
type ToolCallRenderer struct {
	showTool      bool
	showToolInput bool
	// currentTool is the name of the tool currently being rendered.
	currentTool string
	// pendingParam is the name of the parameter whose value is being rendered.
	pendingParam string
	// haveToolHeader tracks whether the tool name line has been emitted.
	haveToolHeader bool
	// replaceBookkeeping: for replace_in_file a diff block needs the finished
	// search value (old) before the replace value (new) is finalised.
	replaceSearchVal  strings.Builder
	replacePendingVal strings.Builder // buffers the current replace value fragments
	replaceHaveSearch bool            // true once a search value has been captured
	replacePairClosed bool            // true after a search/replace pair is emitted
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
	r.replaceSearchVal.Reset()
	r.replacePendingVal.Reset()
	r.replaceHaveSearch = false
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
		// "key: value" lines; the diff block is emitted at OpParamEnd.
		if r.currentTool == "replace_in_file" {
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
				// search fragments are appended directly to the search value
				// builder so the diff block can render the old value.
				r.replaceSearchVal.WriteString(op.Text)
			case "replace":
				// replace fragments are buffered until OpParamEnd.
				r.replacePendingVal.WriteString(op.Text)
			}
			return
		}
		// FEATURE-235: Emit the value fragment verbatim in the granularity the
		// underlying parser produced (which follows the LLM stream chunks).
		// No artificial re-slicing: each real stream chunk's content reaches
		// the user as-is, so the screen grows chunk by chunk.
		emit(op.Text)
	case OpParamEnd:
		r.finaliseParameter(emit)
	case OpToolEnd:
		r.emitToolEnd(emit)
	}
}

// finaliseParameter is called when a parameter value ends (OpParamEnd).
// It emits method-specific finalisation: for replace_in_file the completed
// search/replace pair is rendered as a diff block line.
func (r *ToolCallRenderer) finaliseParameter(emit func(text string)) {
	if !r.showToolInput {
		r.pendingParam = ""
		return
	}
	if r.currentTool != "replace_in_file" {
		if r.showToolInput {
			// Each parameter value ends on its own line.
			emit("\n")
		}
		r.pendingParam = ""
		return
	}
	switch r.pendingParam {
	case "search":
		// The search value has finished streaming. It is already in
		// replaceSearchVal (accumulated by OpValueFragment). Keep it buffered;
		// the replace value (if any) will be paired into a diff block.
		r.replaceHaveSearch = true
		r.replacePendingVal.Reset()
		r.replacePairClosed = false
	case "replace":
		oldVal := r.replaceSearchVal.String()
		newVal := r.replacePendingVal.String()
		if r.replaceHaveSearch && oldVal != "" {
			emit("   🔄 " + oldVal + " ──> " + newVal + "\n")
		} else if newVal != "" {
			emit("   🔄 (new) " + newVal + "\n")
		}
		r.replacePendingVal.Reset()
		r.replaceHaveSearch = false
		r.replaceSearchVal.Reset()
		r.replacePairClosed = true
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
	r.replaceSearchVal.Reset()
	r.replacePendingVal.Reset()
	r.replaceHaveSearch = false
	r.replacePairClosed = false
}

// trimToolCallContent is a small helper used by tests and future extensions.
func trimToolCallContent(s string) string {
	return strings.TrimSpace(s)
}
