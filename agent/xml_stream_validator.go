// Package agent provides the core agent logic for co-shell.
//
// Author: L.Shuang
// Created: 2026-07-30
// Last Modified: 2026-07-30
// Copyright (c) 2026 L.Shuang. All rights reserved.
// MIT License.

package agent

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// xmlValidatorState represents the state of the streaming XML parser state machine.
type xmlValidatorState int

const (
	stateOutside     xmlValidatorState = iota // scanning for '<'
	stateInOpenTag                            // reading tag name after '<'
	stateInContent                            // between open and close tags
	stateInCloseTag                           // reading name after '</'
	stateInComment                            // inside <!-- comment -->
	stateInCDATA                              // inside <![CDATA[ ... ]]>
	stateInCodeBlock                          // inside ``` ... ```
	stateFatal                                // unrecoverable error detected
)

// oTag holds info about an open tag being tracked by the validator.
type oTag struct {
	name     string // unprefixed name (e.g., "execute_command")
	original string // full tag name (e.g., "cs:execute_command")
	isTool   bool   // true if this is a tool-level tag (depth 1 with known tool name)
	parent   string // parent tool name (for param tags, the enclosing tool name)
}

// XMLStreamFatalError is returned when the streaming validator detects
// an unrecoverable XML structure error (wrong tool name, wrong param name,
// mismatched tags, etc.).
type XMLStreamFatalError struct {
	Message string
	Tag     string // the tag that caused the error (original prefixed name)
}

func (e *XMLStreamFatalError) Error() string {
	return e.Message
}

// StreamingXMLValidator performs incremental XML validation on LLM stream output.
// It maintains a state machine that processes XML chunks as they arrive and
// reports fatal errors immediately when they are confidently detectable.
//
// Fatal errors (causes immediate stream termination):
//   - Unknown tool name at depth 1 (tool level)
//   - Unknown parameter name for the current tool
//   - Tag name containing illegal characters like '='
//   - Opening tag and closing tag names don't match
//   - Nested tag mismatch (inner tag not closed before outer close)
//
// Non-fatal conditions (reported only after stream ends):
//   - Missing closing tag (incomplete output) — handled by ParseXMLToolCallsWithTools
//   - Missing required parameters — handled by ParseXMLToolCallsWithTools
type StreamingXMLValidator struct {
	tools      []llm.Tool
	toolNames  map[string]bool            // set of valid tool names (unprefixed)
	paramNames map[string]map[string]bool // tool -> valid param names
	prefix     string                     // current XML tag prefix (e.g., "cs:")

	// State machine
	state    xmlValidatorState
	tagStack []oTag // stack of open tags
	depth    int    // current nesting depth (0 = outside)

	// Buffers
	partialBuf strings.Builder // for partial tags split across chunks
	tagBuf     strings.Builder // for building tag name
	cb         strings.Builder // for CDATA content (just for skipping)

	// Code block tracking
	inCodeBlock bool

	// CDATA tracking
	inCDATA bool

	// Error state
	fatalErr error // set when an unrecoverable error is found
}

// NewStreamingXMLValidator creates a new streaming XML validator.
// tools: the list of known tools with their parameter definitions.
func NewStreamingXMLValidator(tools []llm.Tool) *StreamingXMLValidator {
	v := &StreamingXMLValidator{
		tools:      tools,
		toolNames:  make(map[string]bool),
		paramNames: make(map[string]map[string]bool),
		prefix:     xmlTagPrefix(),
		state:      stateOutside,
	}

	// Build tool name set and parameter name sets
	for _, t := range tools {
		v.toolNames[t.Name] = true
		if props, ok := t.Parameters["properties"].(map[string]interface{}); ok {
			paramSet := make(map[string]bool)
			for name := range props {
				paramSet[name] = true
			}
			v.paramNames[t.Name] = paramSet
		}
	}

	return v
}

// AddChunk processes a chunk of content from the LLM stream output.
// Returns nil if no fatal error is detected (possibly incomplete state).
// Returns *XMLStreamFatalError if an unrecoverable error is found,
// in which case the LLM stream should be terminated immediately.
//
// This method handles tags split across chunks by buffering partial tag content.
// It handles code blocks (```...```), comments (<!--...-->), and CDATA (<![CDATA[...]]>).
func (v *StreamingXMLValidator) AddChunk(chunk string) error {
	if v.fatalErr != nil {
		return v.fatalErr
	}

	// First, check if there's partial content from the previous chunk.
	// If the previous chunk ended with '<' or was inside a tag name,
	// prepend the partial buffer to this chunk.
	combined := chunk
	if v.partialBuf.Len() > 0 {
		combined = v.partialBuf.String() + chunk
		v.partialBuf.Reset()
	}

	// If we were in the middle of reading a tag, resume from state
	combined = v.processContent(combined)
	return v.fatalErr
}

// processContent processes text content with the state machine.
func (v *StreamingXMLValidator) processContent(content string) string {
	i := 0
	for i < len(content) {
		ch := content[i]

		switch v.state {
		case stateOutside:
			// Outside any tag — looking for '<'
			if ch == '<' {
				// Check for code block closing
				if v.tryStartCodeBlock(content, i) {
					v.state = stateInCodeBlock
					v.inCodeBlock = true
					i += 3 // skip ```
					continue
				}
				// Only <{prefix} opens a tool tag — everything else is plain text content
				if i+1 < len(content) {
					if strings.HasPrefix(content[i+1:], xmlTagPrefix()) {
						// Opening tool tag — starts with configured prefix (e.g. "cs:").
						// Write the prefix into tagBuf so tags split across chunks
						// (e.g., "<cs:" then "read_file>") retain the full name.
						v.state = stateInOpenTag
						v.tagBuf.Reset()
						v.tagBuf.WriteString(xmlTagPrefix())
						i += 1 + len(xmlTagPrefix())
						continue
					}
					// Check for closing tool tag: </{prefix} (e.g. </cs:content>)
					if i+2 < len(content) && content[i+1] == '/' && strings.HasPrefix(content[i+2:], xmlTagPrefix()) {
						v.state = stateInCloseTag
						v.tagBuf.Reset()
						i += 2
						continue
					}
					// Anything else: </, <!, <?, HTML tags, <=, etc. — skip to '>'
					closeEnd := strings.IndexByte(content[i:], '>')
					if closeEnd >= 0 {
						i += closeEnd + 1
					} else {
						i = len(content)
					}
					continue
				} else {
					// Stream ends with '<' — buffer it for next chunk
					v.partialBuf.WriteByte('<')
					i++
					continue
				}
			}
			// Regular character — nothing to validate
			i++

		case stateInOpenTag:
			// Reading opening tag name
			if ch == '>' || ch == ' ' || ch == '\t' || ch == '\n' {
				tagName := v.tagBuf.String()
				v.tagBuf.Reset()

				// Check self-closing: <tag/> or <tag />
				if ch == '>' {
					// Check if the previous char was '/' (self-closing)
					if len(tagName) > 0 && tagName[len(tagName)-1] == '/' {
						tagName = tagName[:len(tagName)-1]
						v.processOpenTag(tagName, true)
						v.state = stateOutside
						i++
						continue
					}
					v.processOpenTag(tagName, false)
					v.state = stateInContent
					i++
					continue
				}

				// ch is space/tab/newline — check for self-closing
				// Scan ahead for '/>' indicating self-closing
				rest := content[i+1:]
				selfClose := false
				for j, rc := range rest {
					if rc == '/' && j+1 < len(rest) && rest[j+1] == '>' {
						selfClose = true
						i += 1 + j + 2
						break
					}
					if rc == '>' {
						i += 1 + j + 1
						break
					}
					if rc != ' ' && rc != '\t' {
						// Attribute-like content (e.g., <div class=...)
						// In our simplified parser, skip to '>'
						closeEnd := strings.IndexByte(rest, '>')
						if closeEnd >= 0 {
							i += 1 + closeEnd + 1
						} else {
							i = len(content)
						}
						v.processOpenTag(tagName, selfClose)
						v.state = stateOutside
						break
					}
				}
				if selfClose || i >= len(content) {
					if !selfClose && i < len(content) {
						// Found '>' without '/' before it
						v.processOpenTag(tagName, false)
						v.state = stateInContent
					}
					continue
				}
				v.processOpenTag(tagName, false)
				v.state = stateInContent
				continue
			}
			// Accumulate tag name character
			// Check for illegal characters
			if ch == '=' {
				v.tagBuf.WriteByte(ch)
				// Read the rest of what looks like an attribute
				closeEnd := strings.IndexByte(content[i:], '>')
				if closeEnd >= 0 {
					tagName := v.tagBuf.String()
					reportedTag := tagName + content[i:i+closeEnd]
					v.fatalErr = &XMLStreamFatalError{
						Message: fmt.Sprintf("XML标签名 %q 包含非法字符 '='，XML 标签中不允许使用属性。正确的格式应为：<%s>值</%s>，请使用多标签传参数。",
							reportedTag, reportedTag, reportedTag),
						Tag: reportedTag,
					}
					v.state = stateFatal
					v.tagBuf.Reset()
					return content[i:] // signal to stop processing
				}
				// '>' not found — return rest as unprocessed
				v.partialBuf.WriteString(content[i:])
				return ""
			}
			v.tagBuf.WriteByte(ch)
			i++

		case stateInContent:
			// Between open and close tags — looking for '<'
			if ch == '<' {
				if i+1 < len(content) {
					if strings.HasPrefix(content[i+1:], xmlTagPrefix()) {
						// Nested tool parameter tag — starts with configured prefix (e.g. "cs:").
						// Write the prefix into tagBuf so tags split across chunks
						// (e.g., "<cs:" then "path>") retain the full name.
						v.state = stateInOpenTag
						v.tagBuf.Reset()
						v.tagBuf.WriteString(xmlTagPrefix())
						i += 1 + len(xmlTagPrefix())
						continue
					}
					// Check for closing tool tag: </{prefix} (e.g. </cs:content>)
					if i+2 < len(content) && content[i+1] == '/' && strings.HasPrefix(content[i+2:], xmlTagPrefix()) {
						v.state = stateInCloseTag
						v.tagBuf.Reset()
						i += 2
						continue
					}
					// </, <!, <?, HTML tags, CDATA, etc. — skip to '>' as plain text
					closeEnd := strings.IndexByte(content[i:], '>')
					if closeEnd >= 0 {
						i += closeEnd + 1
					} else {
						i = len(content)
					}
					continue
				}
				// Stream ends with '<' at the end of chunk
				v.partialBuf.WriteByte('<')
				i++
				continue
			}
			i++

		case stateInCloseTag:
			// Reading closing tag name
			if ch == '>' {
				closeName := v.tagBuf.String()
				v.tagBuf.Reset()
				// Only transition state when the close tag actually has the expected prefix.
				// Otherwise, this is an HTML closing tag (like </button>) inside tool parameter
				// content, so we should stay in the current content state.
				if stripXMLTagPrefix(closeName) != "" {
					v.processCloseTag(closeName)
					if v.fatalErr != nil {
						return content[i:] // signal to stop
					}
					if v.depth > 0 {
						v.state = stateInContent // still inside outer tool tag
					} else {
						v.state = stateOutside
					}
				} else {
					// Non-prefixed close tag (HTML) — stay in the content state
					v.state = stateInContent
				}
				i++
				continue
			}
			if ch != ' ' && ch != '\t' {
				v.tagBuf.WriteByte(ch)
			}
			i++

		case stateInComment:
			// Skip until we see '-->'
			if ch == '-' && i+2 < len(content) && content[i+1] == '-' && content[i+2] == '>' {
				v.state = stateOutside
				i += 3
				continue
			}
			i++

		case stateInCDATA:
			// Skip until we see ']]>'
			if ch == ']' && i+2 < len(content) && content[i+1] == ']' && content[i+2] == '>' {
				v.state = stateOutside
				v.inCDATA = false
				i += 3
				continue
			}
			i++

		case stateInCodeBlock:
			// Skip until we see '```'
			if ch == '`' && i+2 < len(content) {
				backtickCount := 1
				for j := i + 1; j < len(content) && content[j] == '`'; j++ {
					backtickCount++
				}
				if backtickCount >= 3 {
					v.state = stateOutside
					v.inCodeBlock = false
					i += backtickCount
					continue
				}
			}
			i++

		case stateFatal:
			// Fatal error already set — skip all further processing
			return content[i:]
		}
	}

	// If we're still in a state that needs more data, buffer the partial content
	switch v.state {
	case stateInOpenTag:
		// Partial tag name saved in tagBuf, but we need to preserve it
		// Store partial tag content in partialBuf for next chunk
	case stateInComment:
		// Preserve partial comment
	case stateInCDATA:
		// Preserve partial CDATA
	case stateInCodeBlock:
		// Preserve partial code block
	}

	return ""
}

// isXMLNameStartChar returns true if b is a valid XML name start character.
// Per XML 1.0 spec: NameStartChar ::= ":" | [A-Z] | "_" | [a-z] | [#xC0-#xD6] ...
// For our purposes, we just check letters, '_', and ':' (ASCII range).
func isXMLNameStartChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == ':'
}

// tryStartCodeBlock checks if the content at position i starts a code block ```.
func (v *StreamingXMLValidator) tryStartCodeBlock(content string, i int) bool {
	if v.inCDATA {
		return false
	}
	// Need at least 3 backticks
	if i+2 < len(content) && content[i] == '`' && content[i+1] == '`' && content[i+2] == '`' {
		// Check if this is at the start of a line (or start of content)
		if i == 0 || content[i-1] == '\n' {
			return true
		}
	}
	return false
}

// processOpenTag processes a complete opening tag name.
func (v *StreamingXMLValidator) processOpenTag(tagName string, selfClosing bool) {
	if tagName == "" {
		return
	}

	// Skip non-tool tags in knownNonToolTags.
	// Check both the raw name and the stripped (prefixed) name so tags like
	// <cs:thinking> are treated as non-tool tags rather than unknown tools.
	if knownNonToolTags[tagName] {
		return
	}

	// Only validate tags that start with the configured prefix (e.g., "cs:").
	// Anything else is not a tool/param tag and is ignored.
	stripped := stripXMLTagPrefix(tagName)
	if stripped == "" {
		return
	}
	if knownNonToolTags[stripped] {
		return
	}

	currentDepth := v.depth

	// FEATURE-298: Validate tool/parameter names as they stream in so a misspelled
	// tool name is reported immediately (and the LLM stream cancelled to save tokens)
	// instead of waiting until the full response is parsed.
	// - Tool-level tags (depth 0) must be known tool names.
	// - Param-level tags (depth > 0) must be valid params of the enclosing tool,
	//   unless the enclosing tool defines no parameters (e.g., view_task_plan which
	//   conventionally accepts an <intent> param not listed in its schema).
	if currentDepth == 0 {
		if !v.toolNames[stripped] {
			errMsg := fmt.Sprintf("XML流式验证错误：未知的工具名 <%s>。请检查方法名拼写并使用正确的工具名。",
				tagName)
			log.Debug("XMLStreamValidator: %s", errMsg)
			v.fatalErr = &XMLStreamFatalError{Message: errMsg, Tag: tagName}
			v.state = stateFatal
			return
		}
	} else {
		// Param-level: validate against the enclosing tool's parameter set.
		if len(v.tagStack) > 0 {
			top := v.tagStack[len(v.tagStack)-1]
			if top.isTool {
				if paramSet, ok := v.paramNames[top.name]; ok && len(paramSet) > 0 {
					if !paramSet[stripped] {
						errMsg := fmt.Sprintf("XML流式验证错误：工具 <%s> 没有参数 <%s>。合法参数有：%s",
							top.name, stripped, strings.Join(sortedParamNames(paramSet), "、"))
						log.Debug("XMLStreamValidator: %s", errMsg)
						v.fatalErr = &XMLStreamFatalError{Message: errMsg, Tag: tagName}
						v.state = stateFatal
						return
					}
				}
			}
		}
	}

	// Track all cs:-prefixed tags on the stack so depth tracking works correctly.
	v.tagStack = append(v.tagStack, oTag{
		name:     stripped,
		original: tagName,
		isTool:   currentDepth == 0,
		parent:   "",
	})
	v.depth++
	log.Debug("XMLStreamValidator: opened tag <%s> (depth=%d)", tagName, v.depth)

	if selfClosing {
		if len(v.tagStack) > 0 {
			v.tagStack = v.tagStack[:len(v.tagStack)-1]
			v.depth--
		}
	}
}

// processCloseTag processes a complete closing tag name.
func (v *StreamingXMLValidator) processCloseTag(closeName string) {
	if closeName == "" {
		return
	}

	// Strip prefix from close tag
	stripped := stripXMLTagPrefix(closeName)
	if stripped == "" {
		// Tag without prefix — not a tool/param tag, ignore
		return
	}

	if len(v.tagStack) == 0 {
		// No open tags — nothing to match
		return
	}

	top := v.tagStack[len(v.tagStack)-1]

	if top.name != stripped {
		// Mismatch: e.g., opened <cs:read_file> but closed </cs:execute_command>.
		// This also covers nested mismatch where an inner tag was not closed before
		// the parent close tag (</cs:read_file> while <cs:path> is still open).
		// Report as fatal so the stream is terminated immediately instead of
		// wasting tokens (FEATURE-298 UC-0003 / UC-0010).
		errMsg := fmt.Sprintf("XML流式验证错误：标签不匹配，<%s> 已打开但遇到 </%s>。请检查每个标签是否正确闭合，且开闭标签名称一致。",
			top.original, closeName)
		log.Debug("XMLStreamValidator: %s", errMsg)
		v.fatalErr = &XMLStreamFatalError{Message: errMsg, Tag: closeName}
		v.state = stateFatal
		return
	}

	// Pop the tag
	v.tagStack = v.tagStack[:len(v.tagStack)-1]
	v.depth--
	log.Debug("XMLStreamValidator: closed tag </%s> (depth=%d)", closeName, v.depth)
}

// hasFatalError returns true if a fatal error has been detected.
func (v *StreamingXMLValidator) hasFatalError() bool {
	return v.fatalErr != nil
}

// fatalErrorMessage returns the fatal error message, or empty string if none.
func (v *StreamingXMLValidator) fatalErrorMessage() string {
	if v.fatalErr != nil {
		return v.fatalErr.Error()
	}
	return ""
}

// Reset clears the validator state for reuse in a new stream.
func (v *StreamingXMLValidator) Reset() {
	v.state = stateOutside
	v.tagStack = nil
	v.depth = 0
	v.partialBuf.Reset()
	v.tagBuf.Reset()
	v.cb.Reset()
	v.fatalErr = nil
	v.inCodeBlock = false
	v.inCDATA = false
}
