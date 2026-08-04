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

// JSONToolCallParser incrementally parses OpenAI-style tool-call argument
// deltas into RenderOps.
//
// OpenAI streams tool-call arguments as JSON fragments; the first chunk
// contains the name, subsequent chunks append fragments of the JSON document.
// This parser maintains a lightweight, error-tolerant FSA that recognises:
//   - the top-level key name (start of a parameter) in "key": order,
//   - string values (e.g. "path", "mode", "content") emitted as fragments,
//   - the end of a parameter value (closing quote, comma, or brace).
//
// The parser does not fully decode the JSON; array/object parameter values
// (e.g. "replacements": [...]) are emitted as raw fragments, keeping the parser
// fast and incremental. Structural errors (unbalanced braces, stray closing
// brackets) are reported via ToolCallParseError so the caller can abort the
// stream immediately (UC-0005).
type JSONToolCallParser struct {
	// inTool is set once the tool name has been seen ("" until then).
	inTool string
	// pendingKey is the top-level key currently being read.
	pendingKey string
	// inString reports whether we are inside a quoted string.
	inString bool
	// keyPhase is true while inside a string being interpreted as a key.
	keyPhase bool
	// expectValue is true immediately after a ':' separator, so the next
	// string/literal is parsed as a value (not a key). It is the authoritative
	// signal that distinguishes a key string from a value string.
	expectValue bool
	// braceDepth counts nested {} / [] — used for error detection.
	braceDepth int
	// buffer accumulates the current raw string (key or value).
	buffer strings.Builder
	// partialQuote carries a partial string from a previous chunk if the
	// string spans a chunk boundary.
	partialQuote bool
}

// NewJSONToolCallParser creates a JSON incremental parser.
func NewJSONToolCallParser() *JSONToolCallParser {
	return &JSONToolCallParser{}
}

// InToolCall reports whether the parser has recognised the tool name.
func (p *JSONToolCallParser) InToolCall() bool {
	return p.inTool != ""
}

// Reset clears the parser state for a new stream call.
func (p *JSONToolCallParser) Reset() {
	p.inTool = ""
	p.pendingKey = ""
	p.inString = false
	p.keyPhase = false
	p.expectValue = false
	p.braceDepth = 0
	p.buffer.Reset()
	p.partialQuote = false
}

// SetToolName tells the parser the tool name. OpenAI delta chunks carry the
// name only once; the stream router sets it as soon as it is available.
func (p *JSONToolCallParser) SetToolName(name string) {
	if p.inTool == "" {
		p.inTool = name
	}
}

// Feed processes one arguments fragment. The fragment is the raw JSON delta
// text appended by this chunk (NOT accumulated from the beginning).
func (p *JSONToolCallParser) Feed(fragment string) ([]RenderOp, error) {
	var ops []RenderOp
	s := p.breathe(fragment)
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case p.inString:
			// FEATURE-235: Incremental value streaming. Scan a run of plain
			// characters (no quote / backslash) and emit them immediately as an
			// OpValueFragment, so long parameter values (e.g. write_to_file's
			// "content") visibly grow on screen chunk by chunk instead of being
			// buffered until the closing quote. Keys still accumulate in
			// p.buffer because they must be emitted whole.
			start := i
			for i < len(s) && s[i] != '"' && s[i] != '\\' {
				i++
			}
			if i > start {
				seg := s[start:i]
				if p.keyPhase {
					p.buffer.WriteString(seg)
				} else if seg != "" {
					ops = append(ops, RenderOp{Kind: OpValueFragment, Text: seg})
				}
			}
			if i >= len(s) {
				// Reached the end of the chunk inside a string; keep the
				// quoted state for the next fragment.
				continue
			}
			ch := s[i]
			if ch == '\\' && i+1 < len(s) {
				// Escaped char — copy both bytes (e.g. \n, \").
				pair := s[i : i+2]
				if p.keyPhase {
					p.buffer.WriteString(pair)
				} else {
					ops = append(ops, RenderOp{Kind: OpValueFragment, Text: pair})
				}
				i += 2
				continue
			}
			if ch == '"' {
				// End of string.
				val := p.buffer.String()
				p.buffer.Reset()
				p.inString = false
				p.partialQuote = false
				if p.keyPhase {
					// A key ended: remember it; the following ':' is consumed
					// next, then the value starts.
					p.pendingKey = val
					p.keyPhase = false
					ops = append(ops, RenderOp{Kind: OpParamKey, Text: val})
				} else {
					// A value ended. The value text was already streamed via
					// OpValueFragment; only close the parameter now.
					p.expectValue = false
					if p.pendingKey != "" {
						ops = append(ops, RenderOp{Kind: OpParamEnd, Text: p.pendingKey})
						p.pendingKey = ""
					}
				}
				i++
				continue
			}
			// Unreachable: ch is neither '"' nor '\\' (loop above consumed all
			// such chars). Keep the compiler satisfied.
			i++
		case ch == '{' || ch == '[':
			p.braceDepth++
			p.buffer.Reset()
			p.expectValue = false
			i++
		case ch == '}' || ch == ']':
			p.braceDepth--
			if p.braceDepth < 0 {
				return nil, &ToolCallParseError{Message: "unbalanced closing brace/square bracket", Tool: p.inTool}
			}
			// End of a value whose raw JSON (object/array) was buffered.
			p.expectValue = false
			if p.pendingKey != "" {
				val := p.buffer.String()
				p.buffer.Reset()
				ops = append(ops, RenderOp{Kind: OpValueFragment, Text: val})
				ops = append(ops, RenderOp{Kind: OpParamEnd, Text: p.pendingKey})
				p.pendingKey = ""
			}
			i++
		case ch == '"':
			// Start of a string: it is a key when we are NOT right after a
			// ':' separator (i.e. not expecting a value); otherwise it is the
			// value of the pending key.
			p.inString = true
			p.buffer.Reset()
			p.partialQuote = true
			p.keyPhase = !p.expectValue
			p.expectValue = false
			i++
		case ch == ':':
			// Colon between key and value: the next string is a value.
			p.keyPhase = false
			p.expectValue = true
			i++
		case ch == ',':
			// Separator — a new key follows.
			p.expectValue = false
			i++
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			i++
		default:
			// Raw value characters (numbers, booleans, unquoted strings in
			// error-tolerant mode): accumulate into the buffer.
			p.buffer.WriteByte(ch)
			p.expectValue = false
			i++
		}
	}

	// If a string was left open at the end of this chunk, keep the partial
	// content for the next fragment.
	if p.inString {
		p.partialQuote = true
	}
	return ops, nil
}

// breathe returns the fragment possibly prefixed with a trailing partial
// string from a previous Feed. In practice the OpenAI delta stream always
// aligns on JSON token boundaries at the client (llm/client.go accumulates
// fragments then emits whole deltas), so partialQuote is rarely used.
func (p *JSONToolCallParser) breathe(fragment string) string {
	if p.partialQuote && p.buffer.Len() > 0 {
		// The previous chunk ended inside a string; the fragment continues it.
		// We simply prepend the buffered tail (the string is already open).
		merged := p.buffer.String() + fragment
		p.buffer.Reset()
		p.partialQuote = false
		return merged
	}
	return fragment
}
