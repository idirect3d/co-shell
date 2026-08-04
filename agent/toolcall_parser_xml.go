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

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/llm"
)

// ToolCallParseError is returned when a streaming parser detects a structural
// error in the tool call content. The caller must abort the LLM stream and
// route the error through the existing parse-error-action handling.
type ToolCallParseError struct {
	Message string
	Tool    string // tool name at fault, or "" if not recognised yet
}

func (e *ToolCallParseError) Error() string {
	if e.Tool != "" {
		return fmt.Sprintf("tool call stream parse error (tool=%s): %s", e.Tool, e.Message)
	}
	return fmt.Sprintf("tool call stream parse error: %s", e.Message)
}

// toolNameOf extracts the tool name from a ToolCallParseError, or "" when the
// error is not a ToolCallParseError. It lets the streaming caller record the
// faulty tool name into the parse-error cache.
func toolNameOf(err error) string {
	if pe, ok := err.(*ToolCallParseError); ok {
		return pe.Tool
	}
	return ""
}

// xmlParserState is the state of the XML tool call FSA.
type xmlParserState int

const (
	xmlStateOutside    xmlParserState = iota // scanning for tags / plain content
	xmlStateInOpenTag                        // reading tag name after '<' (prefix matched)
	xmlStateInCloseTag                       // reading tag name after '</' (prefix matched)
)

// XMLToolCallParser incrementally parses LLM stream content into RenderOps.
//
// The parser recognises only tool-level and parameter-level tags that use the
// configured XML tag prefix (e.g. "cs:"). Chunks may split a tag arbitrarily;
// when the current chunk ends inside an unclosed tag, the whole unclosed tail
// is carried over to the next Feed call so the FSA stays correct across chunk
// boundaries without duplicating the tag name.
//
// Fatal structural errors (an unknown tool name at tool level) are reported via
// ToolCallParseError so the streaming caller can abort immediately (UC-0005).
type XMLToolCallParser struct {
	toolNames map[string]bool // known tool names (unprefixed)
	prefix    string          // current XML tag prefix (e.g. "cs:")

	state xmlParserState
	// inTool is the name of the current tool ("" = outside any tool).
	inTool string
	// inParam is the name of the current parameter inside the tool.
	inParam string
	// partial is the unclosed tag tail from the previous chunk. It is prepended
	// to the next chunk and re-scanned.
	partial string
	// toolCallDepth is 1 while inside a tool, 0 otherwise.
	toolCallDepth int
}

// NewXMLToolCallParser creates a parser over the given known tool definitions.
func NewXMLToolCallParser(tools []llm.Tool) *XMLToolCallParser {
	p := &XMLToolCallParser{
		toolNames: make(map[string]bool, len(tools)),
		prefix:    xmlTagPrefix(),
		state:     xmlStateOutside,
	}
	for _, t := range tools {
		p.toolNames[t.Name] = true
	}
	return p
}

// InToolCall reports whether the parser is currently inside a recognised tool
// call. The content router uses this to divert raw content into tool rendering.
func (p *XMLToolCallParser) InToolCall() bool {
	return p.inTool != ""
}

// PendingToolCall reports whether the parser holds an unclosed prefixed tag
// (p.partial) or is inside a recognised tool call. The partial buffer is only
// populated for a tag that matched the configured "cs:" prefix but whose ">"
// has not arrived yet, so a non-empty partial always means a tool call is in
// progress (or about to be). The content router must not fall back to plain
// content output for such chunks.
func (p *XMLToolCallParser) PendingToolCall() bool {
	return p.partial != "" || p.inTool != ""
}

// Reset clears the parser state for a new stream call.
func (p *XMLToolCallParser) Reset() {
	p.state = xmlStateOutside
	p.inTool = ""
	p.inParam = ""
	p.partial = ""
	p.toolCallDepth = 0
}

// Feed processes one content chunk and returns the RenderOps produced by it.
// If a fatal structural error is detected, a *ToolCallParseError is returned.
func (p *XMLToolCallParser) Feed(chunk string) ([]RenderOp, error) {
	var ops []RenderOp
	content := p.partial + chunk
	p.partial = ""

	for i := 0; i < len(content); {
		switch p.state {
		case xmlStateOutside:
			ltIdx := strings.IndexByte(content[i:], '<')
			if ltIdx < 0 {
				// No tag in the remainder; the text is parameter content when
				// we are inside a tool+param, or ordinary content to hand back
				// to the plain LLM output when we are outside a tool.
				seg := content[i:]
				if p.inTool != "" && p.inParam != "" {
					if seg != "" {
						ops = append(ops, RenderOp{Kind: OpValueFragment, Text: seg})
					}
				} else if seg != "" {
					ops = append(ops, RenderOp{Kind: OpPlainText, Text: seg})
				}
				i = len(content)
				continue
			}
			// Text before the '<' is parameter content (inside a tool) or
			// ordinary content (outside a tool).
			if ltIdx > 0 {
				seg := content[i : i+ltIdx]
				if p.inTool != "" && p.inParam != "" {
					ops = append(ops, RenderOp{Kind: OpValueFragment, Text: seg})
				} else if seg != "" {
					ops = append(ops, RenderOp{Kind: OpPlainText, Text: seg})
				}
				i += ltIdx
				continue
			}
			// At a '<': decide open tag, close tag, or literal '<'.
			// Note: the prefix (e.g. "cs:") is part of the tag name; we skip
			// past '<' plus the prefix so the extracted name is unprefixed and
			// matches p.toolNames.
			if i+1 < len(content) && strings.HasPrefix(content[i+1:], p.prefix) {
				p.state = xmlStateInOpenTag
				i += 1 + len(p.prefix)
				continue
			}
			if i+2 < len(content) && content[i+1] == '/' && strings.HasPrefix(content[i+2:], p.prefix) {
				p.state = xmlStateInCloseTag
				i += 2 + len(p.prefix)
				continue
			}
			// Literal '<' (HTML inside content, prose, etc.).
			if p.inTool != "" && p.inParam != "" {
				ops = append(ops, RenderOp{Kind: OpValueFragment, Text: "<"})
			} else {
				ops = append(ops, RenderOp{Kind: OpPlainText, Text: "<"})
			}
			i++
		case xmlStateInOpenTag, xmlStateInCloseTag:
			gtIdx := strings.IndexByte(content[i:], '>')
			if gtIdx < 0 {
				// The tag name is not closed within this chunk. Carry the
				// whole unclosed tail to the next feed so it is re-scanned
				// cleanly from the '<'. The parser state is reset to Outside
				// so the next Feed re-detects the prefixed tag with the
				// correct skip length.
				start := i
				for start-1 >= 0 && content[start-1] != '<' {
					start--
				}
				if start > 0 && content[start-1] == '<' {
					start--
				}
				p.partial = content[start:]
				p.state = xmlStateOutside
				i = len(content)
				continue
			}
			// Complete tag name.
			name := strings.TrimSpace(content[i : i+gtIdx])
			i += gtIdx + 1
			if p.state == xmlStateInOpenTag {
				if err := p.handleOpenTag(name, &ops); err != nil {
					return nil, err
				}
			} else {
				p.handleCloseTag(name, &ops)
			}
			p.state = xmlStateOutside
		}
	}

	return ops, nil
}

// handleOpenTag processes a completed opening tag name at tool or param level.
// It returns an error when an unknown tool name is encountered at tool level,
// which lets the caller abort the stream immediately (UC-0005).
func (p *XMLToolCallParser) handleOpenTag(name string, ops *[]RenderOp) error {
	if p.toolCallDepth == 0 {
		if !p.toolNames[name] {
			return &ToolCallParseError{
				Message: fmt.Sprintf("unknown tool tag <%s>", name),
				Tool:    name,
			}
		}
		if p.inTool != "" {
			// Nested tool: abandon the previous one.
			p.inTool = ""
		}
		p.inTool = name
		p.inParam = ""
		p.toolCallDepth = 1
		*ops = append(*ops, RenderOp{Kind: OpToolStart, Text: name})
		return nil
	}
	// Param-level open tag.
	p.inParam = name
	*ops = append(*ops, RenderOp{Kind: OpParamKey, Text: name})
	return nil
}

// handleCloseTag processes a completed closing tag name. It finalises the
// current tool or parameter and fences against unknown close tags.
func (p *XMLToolCallParser) handleCloseTag(name string, ops *[]RenderOp) {
	if p.toolCallDepth == 1 && name == p.inTool {
		// Tool close.
		*ops = append(*ops, RenderOp{Kind: OpToolEnd, Text: name})
		p.inTool = ""
		p.inParam = ""
		p.toolCallDepth = 0
		return
	}
	if p.inParam == name {
		// Parameter close.
		*ops = append(*ops, RenderOp{Kind: OpParamEnd, Text: name})
		p.inParam = ""
		return
	}
	// Unmatched/invalid close tag: lenient — ignore (non-fatal).
}
