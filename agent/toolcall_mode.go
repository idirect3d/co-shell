// Package agent provides the core agent logic for co-shell.
//
// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-05-23
// Copyright (c) 2026 L.Shuang. All rights reserved.
// MIT License.

package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// ToolCallMode represents the mode of tool calling style.
type ToolCallMode string

const (
	// ToolCallModeOpenAI represents the standard OpenAI API tool call style.
	// Tools are sent as a "tools" parameter in the LLM request.
	ToolCallModeOpenAI ToolCallMode = "openai"

	// ToolCallModeXML represents the Cline-style XML tool call format.
	// Tools are described in the system prompt, and the LLM returns XML-formatted
	// tool calls in its content.
	ToolCallModeXML ToolCallMode = "xml"
)

// ToolCallModeInfo holds metadata about a tool call mode.
type ToolCallModeInfo struct {
	// Type is the mode type string.
	Type ToolCallMode
	// SendTools indicates whether the "tools" parameter should be sent to the LLM API.
	SendTools bool
	// SystemPromptKey is the i18n key for the system prompt tool usage section.
	// For OpenAI mode, this is the standard key.
	// For XML mode, this is empty (the XML prompt is built dynamically from tools).
	SystemPromptKey string
}

// ParseToolCallMode parses a string into a ToolCallMode.
// Returns the mode and whether the parsing was successful.
func ParseToolCallMode(s string) (ToolCallMode, bool) {
	switch strings.ToLower(s) {
	case "openai":
		return ToolCallModeOpenAI, true
	case "xml":
		return ToolCallModeXML, true
	default:
		return ToolCallModeOpenAI, false
	}
}

// GetToolCallModeInfo returns metadata for the given tool call mode.
func GetToolCallModeInfo(mode ToolCallMode) *ToolCallModeInfo {
	switch mode {
	case ToolCallModeXML:
		return &ToolCallModeInfo{
			Type:            ToolCallModeXML,
			SendTools:       false,
			SystemPromptKey: "",
		}
	default:
		return &ToolCallModeInfo{
			Type:            ToolCallModeOpenAI,
			SendTools:       true,
			SystemPromptKey: "system_prompt_tool_usage",
		}
	}
}

// String returns the string representation of the ToolCallMode.
func (m ToolCallMode) String() string {
	return string(m)
}

// knownNonToolTags are XML tags that the LLM may use for non-tool-call purposes
// (e.g., thinking, reasoning, answer). These should not be parsed as tool calls.
var knownNonToolTags = map[string]bool{
	"thinking":  true,
	"think":     true,
	"answer":    true,
	"result":    true,
	"analysis":  true,
	"reasoning": true,
}

// ParseXMLToolCalls parses XML-formatted tool calls from LLM response content.
// The new format uses the tool name directly as the XML tag, with parameters
// as child elements:
//
//	<execute_command>
//	  <command>ls -la</command>
//	</execute_command>
//
// Returns a list of llm.ToolCall, one for each tool call block found.
func ParseXMLToolCalls(content string) []llm.ToolCall {
	var calls []llm.ToolCall

	// Use a simple state machine to find top-level XML elements.
	// A top-level element is one that is not nested inside another element.
	remaining := content
	depth := 0
	i := 0

	for i < len(remaining) {
		// Find the next '<'
		ltIdx := strings.IndexByte(remaining[i:], '<')
		if ltIdx < 0 {
			break
		}
		ltIdx += i

		// Check if this is a closing tag
		if ltIdx+1 < len(remaining) && remaining[ltIdx+1] == '/' {
			// Closing tag - decrease depth
			closeEnd := strings.IndexByte(remaining[ltIdx:], '>')
			if closeEnd < 0 {
				break
			}
			if depth > 0 {
				depth--
			}
			i = ltIdx + closeEnd + 1
			continue
		}

		// Check if this is a comment <!-- ... -->
		if ltIdx+3 < len(remaining) && remaining[ltIdx:ltIdx+4] == "<!--" {
			commentEnd := strings.Index(remaining[ltIdx:], "-->")
			if commentEnd < 0 {
				break
			}
			i = ltIdx + commentEnd + 3
			continue
		}

		// Check if this is a CDATA section <![CDATA[ ... ]]>
		if ltIdx+8 < len(remaining) && remaining[ltIdx:ltIdx+9] == "<![CDATA[" {
			cdataEnd := strings.Index(remaining[ltIdx:], "]]>")
			if cdataEnd < 0 {
				break
			}
			i = ltIdx + cdataEnd + 3
			continue
		}

		// This is an opening tag
		// Find the tag name (characters after '<' until space, '/', or '>')
		tagStart := ltIdx + 1
		tagEnd := tagStart
		for tagEnd < len(remaining) {
			ch := remaining[tagEnd]
			if ch == '>' || ch == ' ' || ch == '/' || ch == '\t' || ch == '\n' {
				break
			}
			tagEnd++
		}
		tagName := remaining[tagStart:tagEnd]

		if tagName == "" {
			i = ltIdx + 1
			continue
		}

		// Skip self-closing tags like <br/>
		if tagEnd < len(remaining) && remaining[tagEnd] == '/' {
			// Find the '>'
			closeEnd := strings.IndexByte(remaining[tagEnd:], '>')
			if closeEnd < 0 {
				break
			}
			i = tagEnd + closeEnd + 1
			continue
		}

		// Find the end of the opening tag (the '>')
		openEnd := strings.IndexByte(remaining[tagEnd:], '>')
		if openEnd < 0 {
			break
		}
		openEnd += tagEnd + 1

		if depth == 0 {
			// This is a top-level element - could be a tool call
			// Find the matching closing tag
			closeTag := "</" + tagName + ">"
			closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
			if closeIdx < 0 {
				// No matching close tag found, skip
				i = openEnd
				continue
			}

			// Extract the inner content (between opening and closing tags)
			innerContent := remaining[openEnd : openEnd+closeIdx]

			// Check if this is a known non-tool tag
			if knownNonToolTags[tagName] {
				i = openEnd + closeIdx + len(closeTag)
				continue
			}

			// Parse the inner content as parameters
			params := parseXMLChildrenToJSON(innerContent)
			if params != "{}" || hasChildElements(innerContent) {
				// This looks like a tool call (has parameters)
				calls = append(calls, llm.ToolCall{
					ID:        fmt.Sprintf("xml_call_%d", len(calls)),
					Name:      tagName,
					Arguments: params,
				})
			}

			i = openEnd + closeIdx + len(closeTag)
			continue
		}

		// Nested element - skip to the matching close tag
		closeTag := "</" + tagName + ">"
		closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
		if closeIdx < 0 {
			i = openEnd
			continue
		}
		i = openEnd + closeIdx + len(closeTag)
		depth++
	}

	return calls
}

// findMatchingCloseTag finds the matching closing tag for an opening tag,
// accounting for nested elements of the same name.
// It searches in the given content starting from the beginning.
// Returns the index (relative to content start) where the closing tag starts,
// or -1 if not found.
func findMatchingCloseTag(content, tagName string) int {
	depth := 1
	i := 0

	for i < len(content) {
		// Find the next '<'
		ltIdx := strings.IndexByte(content[i:], '<')
		if ltIdx < 0 {
			return -1
		}
		ltIdx += i

		// Check if this is a closing tag
		if ltIdx+1 < len(content) && content[ltIdx+1] == '/' {
			// Get the tag name
			closeStart := ltIdx + 2
			closeEnd := strings.IndexByte(content[closeStart:], '>')
			if closeEnd < 0 {
				return -1
			}
			closeName := content[closeStart : closeStart+closeEnd]
			if closeName == tagName {
				depth--
				if depth == 0 {
					return ltIdx
				}
			}
			i = closeStart + closeEnd + 1
			continue
		}

		// Check if this is a comment or CDATA
		if ltIdx+3 < len(content) && content[ltIdx:ltIdx+4] == "<!--" {
			commentEnd := strings.Index(content[ltIdx:], "-->")
			if commentEnd < 0 {
				return -1
			}
			i = ltIdx + commentEnd + 3
			continue
		}
		if ltIdx+8 < len(content) && content[ltIdx:ltIdx+9] == "<![CDATA[" {
			cdataEnd := strings.Index(content[ltIdx:], "]]>")
			if cdataEnd < 0 {
				return -1
			}
			i = ltIdx + cdataEnd + 3
			continue
		}

		// Check if this is an opening tag of the same name
		if ltIdx+1+len(tagName) <= len(content) {
			potentialName := content[ltIdx+1 : ltIdx+1+len(tagName)]
			if potentialName == tagName {
				// Make sure it's really an opening tag (followed by space, >, /, or newline)
				afterName := content[ltIdx+1+len(tagName)]
				if afterName == '>' || afterName == ' ' || afterName == '\t' || afterName == '\n' || afterName == '/' {
					depth++
				}
			}
		}

		// Skip to the end of this tag
		tagEnd := strings.IndexByte(content[ltIdx:], '>')
		if tagEnd < 0 {
			return -1
		}
		i = ltIdx + tagEnd + 1
	}

	return -1
}

// hasChildElements checks if the given XML content contains any child elements
// (i.e., nested XML tags).
func hasChildElements(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	// Look for '<' followed by a letter (not /, !, or ?)
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '<' && i+1 < len(trimmed) {
			next := trimmed[i+1]
			if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '_' {
				return true
			}
		}
	}
	return false
}

// extractCDATA extracts content from a CDATA section if present.
// Returns the extracted content, or empty string if no CDATA section found.
func extractCDATA(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "<![CDATA[") {
		end := strings.Index(trimmed, "]]>")
		if end >= 9 {
			return trimmed[9:end]
		}
	}
	return ""
}

// parseXMLChildrenToJSON parses child XML elements into a JSON string.
// Input: <command>ls -la</command><cwd>/home</cwd>
// Output: {"command": "ls -la", "cwd": "/home"}
// Handles nested elements by flattening them into JSON strings.
func parseXMLChildrenToJSON(xmlContent string) string {
	xmlContent = strings.TrimSpace(xmlContent)
	if xmlContent == "" {
		return "{}"
	}

	var sb strings.Builder
	sb.WriteString("{")
	first := true

	remaining := xmlContent
	for {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		// Find the next opening tag
		if remaining[0] != '<' {
			// Skip non-tag content
			nextLT := strings.IndexByte(remaining, '<')
			if nextLT < 0 {
				break
			}
			remaining = remaining[nextLT:]
			continue
		}

		// Check for comment or CDATA
		if len(remaining) >= 4 && remaining[:4] == "<!--" {
			end := strings.Index(remaining, "-->")
			if end < 0 {
				break
			}
			remaining = remaining[end+3:]
			continue
		}
		if len(remaining) >= 9 && remaining[:9] == "<![CDATA[" {
			end := strings.Index(remaining, "]]>")
			if end < 0 {
				break
			}
			// CDATA is content, not a child element - skip it
			remaining = remaining[end+3:]
			continue
		}

		// Extract tag name
		tagStart := 1
		tagEnd := tagStart
		for tagEnd < len(remaining) {
			ch := remaining[tagEnd]
			if ch == '>' || ch == ' ' || ch == '/' || ch == '\t' || ch == '\n' {
				break
			}
			tagEnd++
		}
		tagName := remaining[tagStart:tagEnd]
		if tagName == "" {
			break
		}

		// Find end of opening tag
		openEnd := strings.IndexByte(remaining[tagEnd:], '>')
		if openEnd < 0 {
			break
		}
		openEnd += tagEnd + 1

		// Check for self-closing tag
		if remaining[tagEnd] == '/' {
			// Self-closing tag like <param/>
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q: \"\"", tagName))
			first = false
			remaining = remaining[openEnd:]
			continue
		}

		// Find matching closing tag
		closeTag := "</" + tagName + ">"
		closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
		if closeIdx < 0 {
			break
		}

		innerContent := remaining[openEnd : openEnd+closeIdx]

		// Check if inner content has child elements (nested structure)
		if hasChildElements(innerContent) {
			// Nested elements - recursively parse
			nestedJSON := parseXMLChildrenToJSON(innerContent)
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q: %s", tagName, nestedJSON))
			first = false
		} else {
			// Simple text content
			value := strings.TrimSpace(innerContent)
			// Extract CDATA content if present
			if cdataContent := extractCDATA(value); cdataContent != "" {
				value = cdataContent
			}
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q: %q", tagName, value))
			first = false
		}

		remaining = remaining[openEnd+closeIdx+len(closeTag):]
	}

	sb.WriteString("}")
	return sb.String()
}

// ToolCallModeMgr manages the tool calling mode and provides
// mode-specific behavior (openai vs xml).
type ToolCallModeMgr struct {
	mu   sync.RWMutex
	mode ToolCallMode
}

// NewToolCallModeMgr creates a new ToolCallModeMgr with the given mode.
func NewToolCallModeMgr(mode ToolCallMode) *ToolCallModeMgr {
	if mode == "" {
		mode = ToolCallModeOpenAI
	}
	return &ToolCallModeMgr{
		mode: mode,
	}
}

// NewToolCallModeManager creates a new ToolCallModeMgr with the default mode (OpenAI).
// This is a compatibility alias used by agent.go.
func NewToolCallModeManager() *ToolCallModeMgr {
	return NewToolCallModeMgr(ToolCallModeOpenAI)
}

// Mode returns the current tool call mode.
func (mgr *ToolCallModeMgr) Mode() ToolCallMode {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.mode
}

// SetMode sets the tool call mode.
func (mgr *ToolCallModeMgr) SetMode(mode ToolCallMode) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.mode = mode
}

// SetCurrentByString parses and sets the mode from a string ("openai" or "xml").
// Returns true if the mode was valid and set, false otherwise.
func (mgr *ToolCallModeMgr) SetCurrentByString(s string) bool {
	parsed, ok := ParseToolCallMode(s)
	if ok {
		mgr.SetMode(parsed)
	}
	return ok
}

// Current returns information about the current tool call mode.
func (mgr *ToolCallModeMgr) Current() *ToolCallModeInfo {
	mode := mgr.Mode()
	return GetToolCallModeInfo(mode)
}

// ShouldSendTools returns whether the "tools" parameter should be sent
// in the LLM request for the current mode.
// OpenAI mode: true (send tools array)
// XML mode: false (tools described in system prompt)
func (mgr *ToolCallModeMgr) ShouldSendTools() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.mode == ToolCallModeOpenAI
}

// ParseResponseToolCalls parses tool calls from the LLM response.
// For OpenAI mode: returns nil (standard tool_calls are used)
// For XML mode: parses tool calls from the content string
func (mgr *ToolCallModeMgr) ParseResponseToolCalls(content string) []llm.ToolCall {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.mode == ToolCallModeXML {
		return ParseXMLToolCalls(content)
	}
	return nil
}

// BuildToolUsagePrompt builds the tool usage section of the system prompt
// based on the current mode and tools.
// For XML mode, returns detailed XML format instructions.
// For OpenAI mode, returns brief instructions (tools are defined via JSON schema).
func BuildToolUsagePrompt(mode ToolCallMode, tools []llm.Tool, lang string) string {
	switch mode {
	case ToolCallModeXML:
		return buildXMLToolPrompt(tools, lang)
	default:
		// For OpenAI mode, a brief tool usage instruction is enough.
		// The detailed tool definitions are sent via the "tools" parameter.
		return ""
	}
}

// buildXMLToolPrompt builds the XML tool usage instructions for the system prompt.
// Uses the new format where the tool name is the XML tag directly:
//
//	<execute_command>
//	  <command>ls -la</command>
//	</execute_command>
func buildXMLToolPrompt(tools []llm.Tool, lang string) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder

	// Main title: always English uppercase, no markdown heading markers.
	// This is a section separator rendered as a plain line.
	sb.WriteString("TOOL USE\n\n")

	// Determine language for the prompt body
	if lang == "zh" {
		sb.WriteString(`你可以使用以下工具来执行操作。当多个操作相互独立时（如读取多个文件、并行搜索），可以在一次回复中输出多个工具调用。当操作之间存在依赖关系（前一个结果决定下一个操作）时，应按顺序调用工具，等待每个结果后再继续。

工具调用使用 XML 标签格式，工具名称直接作为 XML 标签名，参数作为子元素。XML 元素应按层级缩进，子元素比父元素多缩进 2 个空格：

<read_file>
  <path>main.go</path>
  <start_line>1</start_line>
  <end_line>50</end_line>
</read_file>

如果需要在一次回复中调用多个工具，只需连续使用多个工具标签：

<execute_command>
  <command>ls -la</command>
</execute_command>

<read_file>
  <path>main.go</path>
  <start_line>1</start_line>
  <end_line>50</end_line>
</read_file>

`)
	} else {
		sb.WriteString(`You can use the following tools to interact with the system. When multiple operations are independent (e.g., reading multiple files, searching in parallel), you can output multiple tool calls in a single response. When operations have dependencies (the result of one determines the next), call tools sequentially, waiting for each result before proceeding.

Tool calls use XML tag format, with the tool name directly as the XML tag name and parameters as child elements. XML elements should be indented by hierarchy, with child elements indented 2 spaces more than their parent:

<read_file>
  <path>main.go</path>
  <start_line>1</start_line>
  <end_line>50</end_line>
</read_file>

To call multiple tools in a single response, simply use multiple consecutive tool tags:

<execute_command>
  <command>ls -la</command>
</execute_command>

<read_file>
  <path>main.go</path>
  <start_line>1</start_line>
  <end_line>50</end_line>
</read_file>

`)
	}

	// List available tools
	sb.WriteString("# Available Tools\n\n")

	for _, tool := range tools {
		sb.WriteString(buildXMLToolDescription(tool, lang))
		sb.WriteString("\n")
	}

	if lang == "zh" {
		sb.WriteString(`
# 重要规则
1. 每次回复可以包含多个工具调用，按顺序执行
2. 不要在工具调用标签中包含多余的解释文字
3. 参数值不需要加引号
4. 如果需要在非工具调用的上下文中输出 XML 标签内容（如讨论 XML 格式、展示代码示例等），必须使用 CDATA 包裹，避免被误解析为工具调用

**工具优先级（从高到低）：**
1. **内部工具**（read_file、search_files、replace_in_file 等）— 优先使用内部工具解决问题
2. **MCP 工具** — 当内部工具无法满足需求时，使用 MCP 工具
3. **execute_command** — 当以上工具都无法解决问题时，使用系统命令
   - 优先使用已有系统命令（ls、cat、dir、type、head、tail 等）
   - 其次通过 shell、Python 等方式编程实现
`)
	} else {
		sb.WriteString(`
# Important Rules
1. You can include multiple tool calls in one response, executed in order
2. Do not include extra explanatory text inside tool call tags
3. Parameter values do not need quotes
4. If you need to output XML tag content in a non-tool-call context (e.g., discussing XML format, showing code examples), you MUST wrap it in CDATA to prevent it from being misinterpreted as a tool call

**Tool Priority (highest to lowest):**
1. **Internal tools** (read_file, search_files, replace_in_file, etc.) — Prefer internal tools first
2. **MCP tools** — Use MCP tools when internal tools cannot fulfill the requirement
3. **execute_command** — Use system commands when none of the above can solve the problem
   - Prefer existing system commands (ls, cat, dir, type, head, tail, etc.)
   - Then use shell scripts, Python, or other programming approaches
`)
	}

	// Append the static supplementary content from i18n.
	// This allows adding special-case notes or additional guidance
	// without modifying the dynamic generation code.
	// i18n.T() automatically returns the correct language version.
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySystemPromptToolUsageXML))
	sb.WriteString("\n")

	return sb.String()
}

// paramInfo holds parsed information about a single parameter from JSON Schema.
type paramInfo struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// buildXMLToolDescription builds the description for a single tool in XML format.
// Uses the new format where the tool name is the XML tag directly.
func buildXMLToolDescription(tool llm.Tool, lang string) string {
	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n参数：\n", tool.Name, tool.Description))
	} else {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\nParameters:\n", tool.Name, tool.Description))
	}

	// Parse the JSON schema parameters to extract parameter info
	params := extractParamInfo(tool.Parameters)
	if len(params) == 0 {
		if lang == "zh" {
			sb.WriteString("无\n")
		} else {
			sb.WriteString("None\n")
		}
	} else {
		for _, p := range params {
			if lang == "zh" {
				req := ""
				if p.Required {
					req = "（必需）"
				} else {
					req = "（可选）"
				}
				desc := p.Description
				if desc == "" {
					desc = p.Type
				}
				sb.WriteString(fmt.Sprintf("- <%s> %s %s\n", p.Name, req, desc))
			} else {
				req := ""
				if p.Required {
					req = "(required)"
				} else {
					req = "(optional)"
				}
				desc := p.Description
				if desc == "" {
					desc = p.Type
				}
				sb.WriteString(fmt.Sprintf("- <%s> %s %s\n", p.Name, req, desc))
			}
		}
	}

	return sb.String()
}

// extractParamInfo extracts parameter information from a JSON Schema map.
// The schema is expected to have the standard JSON Schema structure:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "path": {"type": "string", "description": "The file path"},
//	    ...
//	  },
//	  "required": ["path"]
//	}
func extractParamInfo(schema map[string]interface{}) []paramInfo {
	var params []paramInfo

	// Parse required fields list
	requiredFields := make(map[string]bool)
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				requiredFields[s] = true
			}
		}
	}

	// Parse properties
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Iterate over properties in sorted order for deterministic output
	propNames := make([]string, 0, len(props))
	for name := range props {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, name := range propNames {
		prop, ok := props[name].(map[string]interface{})
		if !ok {
			continue
		}

		paramType, _ := prop["type"].(string)
		paramDesc, _ := prop["description"].(string)

		params = append(params, paramInfo{
			Name:        name,
			Type:        paramType,
			Description: paramDesc,
			Required:    requiredFields[name],
		})
	}

	return params
}
