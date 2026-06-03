// Package agent provides the core agent logic for co-shell.
//
// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-05-23
// Copyright (c) 2026 L.Shuang. All rights reserved.
// MIT License.

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
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

// toolUsageKeyMap maps tool names to their i18n keys for usage examples.
// Used by buildXMLToolDescription to include per-tool usage examples.
var toolUsageKeyMap = map[string]string{
	"execute_command":            i18n.KeyToolUsageExecuteCommand,
	"read_file":                  i18n.KeyToolUsageReadFile,
	"search_files":               i18n.KeyToolUsageSearchFiles,
	"list_files":                 i18n.KeyToolUsageListFiles,
	"list_code_definition_names": i18n.KeyToolUsageListCodeDefNames,
	"replace_in_file":            i18n.KeyToolUsageReplaceInFile,
	"write_to_file":              i18n.KeyToolUsageWriteToFile,
	"add_images":                 i18n.KeyToolUsageAddImages,
	"remove_images":              i18n.KeyToolUsageRemoveImages,
	"clear_images":               i18n.KeyToolUsageClearImages,
	"launch_sub_agent":           i18n.KeyToolUsageLaunchSubAgent,
	"schedule_task":              i18n.KeyToolUsageScheduleTask,
	"create_task_plan":           i18n.KeyToolUsageCreateTaskPlan,
	"update_task_step":           i18n.KeyToolUsageUpdateTaskStep,
	"insert_task_steps":          i18n.KeyToolUsageInsertTaskSteps,
	"remove_task_steps":          i18n.KeyToolUsageRemoveTaskSteps,
	"view_task_plan":             i18n.KeyToolUsageViewTaskPlan,
	"get_memory_slice":           i18n.KeyToolUsageGetMemorySlice,
	"memory_search":              i18n.KeyToolUsageMemorySearch,
	"delete_memory":              i18n.KeyToolUsageDeleteMemory,
	"update_settings":            i18n.KeyToolUsageUpdateSettings,
	"list_settings":              i18n.KeyToolUsageListSettings,
	"ask_followup_question":      i18n.KeyToolUsageAskFollowupQuestion,
	"adjust_context_start":       i18n.KeyToolUsageAdjustContextStart,
	"attempt_completion":         i18n.KeyToolUsageAttemptCompletion,
	"shell_start":                i18n.KeyToolUsageShellStart,
	"shell_send":                 i18n.KeyToolUsageShellSend,
	"shell_get_output":           i18n.KeyToolUsageShellGetOutput,
	"shell_stop":                 i18n.KeyToolUsageShellStop,
}

// sortedParamNames returns the keys of a map in sorted order.
func sortedParamNames(params map[string]bool) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// getToolParamNames extracts the set of valid parameter names for a given tool.
// Returns nil if the tool is not found or has no properties defined.
func getToolParamNames(tools []llm.Tool, toolName string) map[string]bool {
	for _, tool := range tools {
		if tool.Name == toolName {
			props, ok := tool.Parameters["properties"].(map[string]interface{})
			if !ok {
				return nil
			}
			paramNames := make(map[string]bool, len(props))
			for name := range props {
				paramNames[name] = true
			}
			return paramNames
		}
	}
	return nil
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
// If parsing fails for a particular tag (e.g., parameter name typos, missing
// closing tags), it returns an error tool call with detailed diagnostics
// instead of attempting to execute the malformed call. This prevents the LLM
// from looping on the same error without understanding what went wrong.
//
// Only tags whose names match known tool names are parsed as tool calls.
// Unknown tags are silently ignored (treated as regular content).
// Use ParseXMLToolCallsWithTools to provide the list of known tools.
func ParseXMLToolCalls(content string) []llm.ToolCall {
	return ParseXMLToolCallsWithTools(content, nil)
}

// ParseXMLToolCallsWithTools parses XML-formatted tool calls from LLM response content,
// with optional tool definitions for parameter name validation.
// If tools is non-nil, parameter names are validated against the tool definitions,
// AND only tags whose names match known tool names are parsed as tool calls.
// Unknown tags are silently ignored (treated as regular content).
// See ParseXMLToolCalls for details on the XML format.
func ParseXMLToolCallsWithTools(content string, tools []llm.Tool) []llm.ToolCall {
	var calls []llm.ToolCall

	// Use a simple state machine to find top-level XML elements.
	// A top-level element is one that is not nested inside another element.
	remaining := content
	depth := 0
	i := 0

	log.Debug("ParseXMLToolCalls: ENTER, content=%q, len=%d", content, len(content))

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
			// Check if this is a known tool BEFORE any attribute or validity checks.
			// If tools list is provided and tag is NOT in the known tool list, skip
			// the entire block silently — this prevents HTML/Python content with angle
			// brackets (<div>, <for>) from generating spurious XML parse errors.
			if len(tools) > 0 {
				isKnown := false
				for _, t := range tools {
					if t.Name == tagName {
						isKnown = true
						break
					}
				}
				if !isKnown {
					// Unknown tag - skip the entire block and treat as content
					closeTag := "</" + tagName + ">"
					closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
					if closeIdx < 0 {
						closeIdx = findCloseTagLenient(remaining[openEnd:], tagName)
					}
					if closeIdx >= 0 {
						i = openEnd + closeIdx + len(closeTag)
					} else {
						i = openEnd
					}
					log.Debug("ParseXMLToolCalls: skipping unknown tag <%s> (not a known tool)", tagName)
					continue
				}
				// Known tool — now validate syntax. Skip space/attribute and
				// validity checks for non-tool tags.
				if tagEnd < len(remaining) && (remaining[tagEnd] == ' ' || remaining[tagEnd] == '\t') {
					openEnd2 := strings.IndexByte(remaining[tagEnd:], '>')
					if openEnd2 < 0 {
						break
					}
					openEnd2 += tagEnd + 1

					errMsg := fmt.Sprintf("XML解析错误：方法标签 <%s 后面跟有空格，XML 标签中不允许包含属性。正确的格式应为：<%s>...</%s>，不要添加属性", tagName, tagName, tagName)
					log.Debug("ParseXMLToolCalls: %s", errMsg)
					calls = append(calls, llm.ToolCall{
						ID:        fmt.Sprintf("xml_error_%d", len(calls)),
						Name:      "_xml_parse_error",
						Arguments: fmt.Sprintf(`{"error": %q, "tag": %q}`, errMsg, tagName),
					})
					i = openEnd2
					continue
				}
				if valid, reason := isValidTagName(tagName); !valid {
					openEnd2 := strings.IndexByte(remaining[tagEnd:], '>')
					if openEnd2 < 0 {
						break
					}
					openEnd2 += tagEnd + 1

					errMsg := fmt.Sprintf("XML解析错误：方法标签 %s", reason)
					log.Debug("ParseXMLToolCalls: %s", errMsg)
					calls = append(calls, llm.ToolCall{
						ID:        fmt.Sprintf("xml_error_%d", len(calls)),
						Name:      "_xml_parse_error",
						Arguments: fmt.Sprintf(`{"error": %q, "tag": %q}`, errMsg, tagName),
					})
					i = openEnd2
					continue
				}
			} else {
				// No tools list — validate syntax for all tags
				// Check if the tag name is followed by a space (attribute syntax like <execute_command param=value>)
				if tagEnd < len(remaining) && (remaining[tagEnd] == ' ' || remaining[tagEnd] == '\t') {
					openEnd2 := strings.IndexByte(remaining[tagEnd:], '>')
					if openEnd2 < 0 {
						break
					}
					openEnd2 += tagEnd + 1

					errMsg := fmt.Sprintf("XML解析错误：方法标签 <%s 后面跟有空格，XML 标签中不允许包含属性。正确的格式应为：<%s>...</%s>，不要添加属性", tagName, tagName, tagName)
					log.Debug("ParseXMLToolCalls: %s", errMsg)
					calls = append(calls, llm.ToolCall{
						ID:        fmt.Sprintf("xml_error_%d", len(calls)),
						Name:      "_xml_parse_error",
						Arguments: fmt.Sprintf(`{"error": %q, "tag": %q}`, errMsg, tagName),
					})
					i = openEnd2
					continue
				}

				// Validate tag name for illegal characters (e.g., <execute_command=xxx>)
				if valid, reason := isValidTagName(tagName); !valid {
					openEnd2 := strings.IndexByte(remaining[tagEnd:], '>')
					if openEnd2 < 0 {
						break
					}
					openEnd2 += tagEnd + 1

					errMsg := fmt.Sprintf("XML解析错误：方法标签 %s", reason)
					log.Debug("ParseXMLToolCalls: %s", errMsg)
					calls = append(calls, llm.ToolCall{
						ID:        fmt.Sprintf("xml_error_%d", len(calls)),
						Name:      "_xml_parse_error",
						Arguments: fmt.Sprintf(`{"error": %q, "tag": %q}`, errMsg, tagName),
					})
					i = openEnd2
					continue
				}
			}

			// Find the matching closing tag
			closeTag := "</" + tagName + ">"
			closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
			if closeIdx < 0 {
				// No matching close tag found for the exact tag name.
				// Try to find ANY closing tag as a fallback (handles LLM errors
				// like <execute>...</execute_command> where tag names don't match).
				fallbackIdx := findAnyCloseTag(remaining[openEnd:])
				if fallbackIdx >= 0 {
					// Use the fallback close tag
					closeIdx = fallbackIdx
					// Extract the actual close tag name from the content
					closeContent := remaining[openEnd+fallbackIdx:]
					closeEnd := strings.IndexByte(closeContent[2:], '>')
					if closeEnd >= 0 {
						actualCloseName := closeContent[2 : 2+closeEnd]
						closeTag = "</" + actualCloseName + ">"
						log.Debug("ParseXMLToolCalls: using fallback close tag %s for <%s>", closeTag, tagName)
					}
				} else {
					// No matching close tag found at all.
					// Try a more aggressive approach: search for the closing tag
					// by scanning forward from the opening tag position, treating
					// '<' characters inside content as literal text (not XML tags).
					// This handles cases where the LLM puts special chars like
					// '<', '>', '&' in content without CDATA wrapping.
					aggressiveIdx := findCloseTagLenient(remaining[openEnd:], tagName)
					if aggressiveIdx >= 0 {
						closeIdx = aggressiveIdx
						log.Debug("ParseXMLToolCalls: using lenient close tag for <%s> at idx %d", tagName, closeIdx)
					} else {
						// Truly cannot find any close tag. Return an error tool call
						// with detailed position info so the LLM can fix it.
						errMsg := fmt.Sprintf("XML解析错误：找不到 <%s> 的闭合标签 </%s>。位置：从第 %d 字符开始。可能原因：内容中包含未转义的特殊字符（如 <、>、&），请使用 <![CDATA[...]]> 包裹包含特殊字符的内容。",
							tagName, tagName, ltIdx)
						log.Debug("ParseXMLToolCalls: %s", errMsg)
						calls = append(calls, llm.ToolCall{
							ID:        fmt.Sprintf("xml_error_%d", len(calls)),
							Name:      "_xml_parse_error",
							Arguments: fmt.Sprintf(`{"error": %q, "tag": %q, "position": %d}`, errMsg, tagName, ltIdx),
						})
						i = openEnd
						continue
					}
				}
			}

			// Extract the inner content (between opening and closing tags)
			innerContent := remaining[openEnd : openEnd+closeIdx]

			// Check if this is a known non-tool tag
			if knownNonToolTags[tagName] {
				log.Debug("ParseXMLToolCalls: skipping known non-tool tag <%s>", tagName)
				i = openEnd + closeIdx + len(closeTag)
				continue
			}

			// Parse the inner content as parameters, collecting any parse errors
			params, parseErrors := parseXMLChildrenToJSON(innerContent)
			log.Debug("ParseXMLToolCalls: tag=<%s>, innerContent=%q, params=%q, hasChildElements=%v, parseErrors=%v",
				tagName, innerContent, params, hasChildElements(innerContent), parseErrors)

			// If tools are provided, validate parameter names against the tool definition.
			// This catches parameter name typos (e.g., "commmand" instead of "command").
			if len(tools) > 0 {
				validParams := getToolParamNames(tools, tagName)
				if validParams != nil {
					// Parse the params JSON to extract parameter names
					var parsedArgs map[string]interface{}
					if err := json.Unmarshal([]byte(params), &parsedArgs); err == nil {
						for paramName := range parsedArgs {
							if !validParams[paramName] {
								errMsg := fmt.Sprintf("参数名 %q 不是工具 %q 的合法参数。%s 的合法参数有：%s",
									paramName, tagName, tagName, strings.Join(sortedParamNames(validParams), "、"))
								parseErrors = append(parseErrors, errMsg)
							}
						}
					}
				}
			}

			// If there were parameter parse errors, report them to the LLM instead of
			// attempting to execute the malformed call. This prevents the LLM from
			// looping on the same error without understanding what went wrong.
			if len(parseErrors) > 0 {
				errDetail := strings.Join(parseErrors, "; ")

				// Try to get the tool's usage example from i18n for the reference format
				refFormat := buildReferenceFormat(tagName)
				errMsg := fmt.Sprintf(
					"XML参数解析错误：调用 <%s> 时发现以下参数格式问题：\n%s\n\n"+
						"请检查你的调用格式，确保每个参数标签名正确、闭合标签匹配。\n"+
						"如果参数值包含特殊字符（如 <、>、&），请使用 <![CDATA[...]]> 包裹。\n"+
						"参考格式：\n%s",
					tagName, errDetail, refFormat)
				log.Debug("ParseXMLToolCalls: parameter parse errors for <%s>: %s", tagName, errDetail)
				calls = append(calls, llm.ToolCall{
					ID:        fmt.Sprintf("xml_error_%d", len(calls)),
					Name:      "_xml_parse_error",
					Arguments: fmt.Sprintf(`{"error": %q, "tag": %q}`, errMsg, tagName),
				})
				i = openEnd + closeIdx + len(closeTag)
				continue
			}

			// Determine if this is a valid tool call:
			// 1. Has parameters (params != "{}")
			// 2. Has child elements (nested XML structure)
			// 3. Has only whitespace content (no-parameter tool like <view_task_plan></view_task_plan>)
			trimmedInner := strings.TrimSpace(innerContent)
			isNoParamTool := trimmedInner == ""
			if params != "{}" || hasChildElements(innerContent) || isNoParamTool {
				// This looks like a tool call
				log.Debug("ParseXMLToolCalls: ADDING tool call: name=%s, params=%s", tagName, params)
				calls = append(calls, llm.ToolCall{
					ID:        fmt.Sprintf("xml_call_%d", len(calls)),
					Name:      tagName,
					Arguments: params,
				})
			} else {
				log.Debug("ParseXMLToolCalls: SKIPPING tag <%s>: params is empty and no child elements", tagName)
			}

			i = openEnd + closeIdx + len(closeTag)
			continue
		}

		// Nested element - skip to the matching close tag
		closeTag := "</" + tagName + ">"
		closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
		if closeIdx < 0 {
			// Try lenient fallback for nested elements too
			aggressiveIdx := findCloseTagLenient(remaining[openEnd:], tagName)
			if aggressiveIdx >= 0 {
				closeIdx = aggressiveIdx
			} else {
				i = openEnd
				continue
			}
		}
		i = openEnd + closeIdx + len(closeTag)
		depth++
	}

	log.Debug("ParseXMLToolCalls: DONE, found %d tool calls", len(calls))
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
					// Skip to the end of this tag
					tagEnd := strings.IndexByte(content[ltIdx:], '>')
					if tagEnd < 0 {
						return -1
					}
					i = ltIdx + tagEnd + 1
					continue
				}
			}
		}

		// Not a valid XML tag (e.g., '<' in regex like [^<]*), skip this character
		i = ltIdx + 1
	}

	return -1
}

// findAnyCloseTag finds the first closing tag (</...>) in the content.
// This is a fallback for handling LLM errors where opening and closing tag names
// don't match (e.g., <execute>...</execute_command>).
// Returns the index (relative to content start) where the closing tag starts,
// or -1 if not found.
func findAnyCloseTag(content string) int {
	for i := 0; i < len(content); i++ {
		if content[i] == '<' && i+1 < len(content) && content[i+1] == '/' {
			closeEnd := strings.IndexByte(content[i:], '>')
			if closeEnd >= 0 {
				return i
			}
		}
	}
	return -1
}

// findCloseTagLenient finds a closing tag </tagName> in the content using a
// lenient approach that treats '<' characters inside content as literal text
// rather than XML tags. This handles cases where the LLM puts special chars
// like '<', '>', '&' in content without CDATA wrapping.
//
// The algorithm scans for the pattern "</tagName>" directly, ignoring any
// '<' characters that appear before it. This is intentionally simple:
// it does not track nesting depth, so it works best for leaf elements
// (elements that don't contain nested elements of the same name).
//
// Returns the index (relative to content start) where the closing tag starts,
// or -1 if not found.
func findCloseTagLenient(content, tagName string) int {
	closePattern := "</" + tagName + ">"
	idx := strings.Index(content, closePattern)
	if idx >= 0 {
		return idx
	}

	// Also try with whitespace variations (e.g., </tagName >)
	// Some LLMs might add spaces inside the closing tag
	for i := 0; i < len(content); i++ {
		if content[i] == '<' && i+1 < len(content) && content[i+1] == '/' {
			// Check if this looks like our tag name
			potentialEnd := strings.IndexByte(content[i:], '>')
			if potentialEnd < 0 {
				continue
			}
			closeContent := content[i+2 : i+potentialEnd]
			closeContent = strings.TrimSpace(closeContent)
			if closeContent == tagName {
				return i
			}
		}
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

// jsonValue converts a plain text string to a JSON value with automatic type detection.
// - Integers (e.g., "5", "-3", "0") → JSON number (no quotes)
// - Floats (e.g., "3.14", "-0.5") → JSON number (no quotes)
// - Booleans ("true", "false") → JSON boolean (no quotes)
// - Everything else → JSON string (with quotes)
func jsonValue(s string) string {
	// Try integer
	if _, err := fmt.Sscanf(s, "%d", new(int)); err == nil {
		// Verify the entire string is the integer (no extra chars)
		var n int
		var extra string
		if _, e := fmt.Sscanf(s, "%d%s", &n, &extra); e != nil {
			return s // pure integer
		}
	}
	// Try float
	if _, err := fmt.Sscanf(s, "%f", new(float64)); err == nil {
		var f float64
		var extra string
		if _, e := fmt.Sscanf(s, "%f%s", &f, &extra); e != nil {
			return s // pure float
		}
	}
	// Try boolean
	if s == "true" || s == "false" {
		return s
	}
	// Default: JSON string
	return fmt.Sprintf("%q", s)
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

// stripREPLMaskMarkers removes REPL input masking markers from content.
// The external REPL (go-prompt) injects |mask_start|...|mask_end| markers
// when masking sensitive input (e.g., passwords, API keys during confirmation).
// These markers may leak into the shell session output and subsequently into
// the LLM's context. In XML mode, the '|' character in these markers would
// cause XML parse errors ('|' is illegal in XML tag names), so we strip them
// before XML parsing.
//
// The marker format is: <|mask_start|>...<|mask_end|>
// Note: The actual markers may have leading '<' as part of the user input,
// or appear bare as |mask_start|...|mask_end| depending on context.
func stripREPLMaskMarkers(content string) string {
	// First, handle the format with angle brackets: <|mask_start|>...<|mask_end|>
	// Strip the entire block including content between markers
	result := content
	for {
		startIdx := strings.Index(result, "<|mask_start|>")
		if startIdx < 0 {
			startIdx = strings.Index(result, "|mask_start|")
		}
		if startIdx < 0 {
			break
		}

		endIdx := strings.Index(result[startIdx:], "|mask_end|")
		if endIdx < 0 {
			// No end marker found, just remove from start to end
			result = result[:startIdx]
			break
		}
		endIdx += startIdx + len("|mask_end|")

		result = result[:startIdx] + result[endIdx:]
	}

	return result
}

// isValidTagName checks if a tag name contains only valid characters.
// Valid tag names consist of letters, digits, underscores, and hyphens.
// Returns true if the tag name is valid, false otherwise.
// Also returns a descriptive error message if invalid.
func isValidTagName(tagName string) (bool, string) {
	if tagName == "" {
		return false, "标签名为空"
	}

	// Check for attribute-like syntax: <param=value>
	if strings.Contains(tagName, "=") {
		return false, fmt.Sprintf("标签名 %q 包含非法字符 '='，XML 标签中不允许包含属性。标签名只能包含字母、数字、下划线和连字符，不能包含 '='", tagName)
	}

	// Check each character
	for i, ch := range tagName {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		if ch == ' ' {
			return false, fmt.Sprintf("标签名 %q 包含空格，XML 标签中不允许包含属性。正确的格式应为：<%s>值</%s>，不要添加属性", tagName, tagName, tagName)
		}
		return false, fmt.Sprintf("标签名 %q 在第 %d 个字符处包含非法字符 %q，标签名只能包含字母、数字、下划线和连字符", tagName, i+1, ch)
	}

	return true, ""
}

// parseXMLChildrenToJSON parses child XML elements into a JSON string.
// Input: <command>ls -la</command><cwd>/home</cwd>
// Output: {"command": "ls -la", "cwd": "/home"}
// Handles nested elements by flattening them into JSON strings.
//
// Array handling: when a child tag name is "item", it is treated as an
// array element and its value is placed directly into a JSON array without
// the "item" key. For example:
//
//	Input: <replacements>
//	         <item><search>a</search><replace>b</replace></item>
//	         <item><search>c</search><replace>d</replace></item>
//	       </replacements>
//	Output: {"replacements": [{"search": "a", "replace": "b"}, {"search": "c", "replace": "d"}]}
//
// When the same non-item tag name appears multiple times consecutively,
// the values are also merged into a JSON array. For example:
//
//	Input: <item><a>1</a></item><item><a>2</a></item>
//	Output: {"item": [{"a": "1"}, {"a": "2"}]}
//
// Returns the JSON string and any parse errors encountered. If there are parse
// errors, the returned JSON may be incomplete. Callers should check the errors
// and report them to the LLM instead of attempting to execute the malformed call.
func parseXMLChildrenToJSON(xmlContent string) (string, []string) {
	var parseErrors []string

	xmlContent = strings.TrimSpace(xmlContent)
	if xmlContent == "" {
		return "{}", nil
	}

	// If the content does not start with '<', it is plain text (not XML structure).
	// Return it as a JSON value with automatic type detection.
	if xmlContent[0] != '<' {
		return jsonValue(xmlContent), nil
	}

	// First pass: collect all child elements with their tag names and JSON values
	type childEntry struct {
		tagName string
		jsonVal string // the JSON value (quoted string or object)
	}
	var children []childEntry

	remaining := xmlContent
	for {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		// Find the next opening tag
		if remaining[0] != '<' {
			// Skip non-tag content (text between XML elements)
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

		// Check if the tag name is followed by a space (attribute syntax like <param name=value>)
		if tagEnd < len(remaining) && (remaining[tagEnd] == ' ' || remaining[tagEnd] == '\t') {
			errMsg := fmt.Sprintf("标签名 %q 后面跟有空格，XML 标签中不允许包含属性。正确的格式应为：<%s>值</%s>，不要添加属性", tagName, tagName, tagName)
			parseErrors = append(parseErrors, errMsg)
			// Skip past this malformed tag by finding the '>'
			openEnd := strings.IndexByte(remaining[tagEnd:], '>')
			if openEnd < 0 {
				break
			}
			remaining = remaining[tagEnd+openEnd+1:]
			continue
		}

		// Validate tag name for illegal characters (e.g., <param=value>)
		if valid, reason := isValidTagName(tagName); !valid {
			parseErrors = append(parseErrors, reason)
			// Skip past this malformed tag by finding the '>'
			openEnd := strings.IndexByte(remaining[tagEnd:], '>')
			if openEnd < 0 {
				break
			}
			remaining = remaining[tagEnd+openEnd+1:]
			continue
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
			children = append(children, childEntry{tagName: tagName, jsonVal: `""`})
			remaining = remaining[openEnd:]
			continue
		}

		// Find matching closing tag
		closeTag := "</" + tagName + ">"
		closeIdx := findMatchingCloseTag(remaining[openEnd:], tagName)
		if closeIdx < 0 {
			// Try lenient fallback for content with special characters
			lenientIdx := findCloseTagLenient(remaining[openEnd:], tagName)
			if lenientIdx >= 0 {
				closeIdx = lenientIdx
				log.Debug("parseXMLChildrenToJSON: using lenient close tag for <%s> at idx %d", tagName, closeIdx)
			} else {
				// Cannot find closing tag for this parameter - record error
				errMsg := fmt.Sprintf("参数 <%s> 缺少闭合标签 </%s>", tagName, tagName)
				parseErrors = append(parseErrors, errMsg)
				log.Debug("parseXMLChildrenToJSON: %s", errMsg)
				break
			}
		}

		innerContent := remaining[openEnd : openEnd+closeIdx]

		// Check if the entire inner content is wrapped in CDATA
		trimmedInner := strings.TrimSpace(innerContent)
		if strings.HasPrefix(trimmedInner, "<![CDATA[") {
			// CDATA-wrapped content - extract as plain text, do not parse as XML
			value := extractCDATA(trimmedInner)
			children = append(children, childEntry{tagName: tagName, jsonVal: jsonValue(value)})
			remaining = remaining[openEnd+closeIdx+len(closeTag):]
			continue
		}

		// Check if inner content has child elements (nested structure)
		if hasChildElements(innerContent) {
			// Nested elements - recursively parse
			nestedJSON, nestedErrors := parseXMLChildrenToJSON(innerContent)
			if len(nestedErrors) > 0 {
				parseErrors = append(parseErrors, nestedErrors...)
			}
			children = append(children, childEntry{tagName: tagName, jsonVal: nestedJSON})
		} else {
			// Simple text content — preserve all whitespace, do NOT trim.
			// The LLM controls every byte including leading/trailing spaces,
			// tabs, and newlines. For command/shell content, whitespace is
			// semantically significant (e.g., Python indentation, shell args).
			// CDATA is still extracted if present.
			value := innerContent
			if cdataContent := extractCDATA(value); cdataContent != "" {
				value = cdataContent
			}
			children = append(children, childEntry{tagName: tagName, jsonVal: jsonValue(value)})
		}

		remaining = remaining[openEnd+closeIdx+len(closeTag):]

	}

	// Second pass: build JSON.
	// - If ALL children have tag name "item", output as a JSON array directly.
	//   This handles the pattern where the parent tag name is the parameter name
	//   and <item> represents array items:
	//     <replacements>
	//       <item><search>a</search><replace>b</replace></item>
	//       <item><search>c</search><replace>d</replace></item>
	//     </replacements>
	//   → [{"search": "a", "replace": "b"}, {"search": "c", "replace": "d"}]
	//   The caller (parent's second pass) will use "replacements" as the key.
	// - Otherwise, build a JSON object with child tag names as keys.
	// - Consecutive children with the same non-item tag name are merged
	//   into a JSON array.
	if len(children) > 0 {
		allItem := true
		for _, c := range children {
			if c.tagName != "item" {
				allItem = false
				break
			}
		}
		if allItem {
			// All children are <item> - output as a JSON array directly
			var sb strings.Builder
			sb.WriteString("[")
			for k := 0; k < len(children); k++ {
				if k > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(children[k].jsonVal)
			}
			sb.WriteString("]")
			return sb.String(), parseErrors
		}
	}

	var sb strings.Builder
	sb.WriteString("{")
	first := true
	i := 0
	for i < len(children) {
		currentTag := children[i].tagName

		// Count how many consecutive entries have the same tag name
		count := 1
		for j := i + 1; j < len(children); j++ {
			if children[j].tagName == currentTag {
				count++
			} else {
				break
			}
		}

		if !first {
			sb.WriteString(", ")
		}

		if count == 1 {
			// Single occurrence - output as a simple key-value pair
			sb.WriteString(fmt.Sprintf("%q: %s", currentTag, children[i].jsonVal))
		} else {
			// Multiple occurrences - output as a JSON array
			sb.WriteString(fmt.Sprintf("%q: [", currentTag))
			for k := 0; k < count; k++ {
				if k > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(children[i+k].jsonVal)
			}
			sb.WriteString("]")
		}

		first = false
		i += count
	}

	sb.WriteString("}")
	return sb.String(), parseErrors
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
		// For OpenAI mode, append examples, task progress, and editing files
		// assembled from split components (separators are part of i18n/external content).
		var sb strings.Builder
		sb.WriteString(i18n.T(i18n.KeySystemPromptToolUsageExamples))
		sb.WriteString(i18n.T(i18n.KeySystemPromptToolUsageTaskProgress))
		sb.WriteString(i18n.T(i18n.KeySystemPromptEditingFiles))
		return sb.String()
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

	// The TOOL USE header and XML format examples are now in i18n KeySystemPromptToolUsageXML.
	// Write the i18n content first (header + XML examples).
	sb.WriteString(i18n.T(i18n.KeySystemPromptToolUsageXML))
	sb.WriteString("\n")

	// List available tools
	sb.WriteString("# Tools\n\n")

	for _, tool := range tools {
		sb.WriteString(buildXMLToolDescription(tool, lang))
		sb.WriteString("\n")
	}

	// Append the supplementary rules assembled from split components:
	// Tool Use Examples + Task Progress + Editing Files, separated by ====
	// Each section can be overridden by an external .md file in the workspace root.
	cwd, _ := os.Getwd()
	examplesText := loadExternalFile(cwd, "TOOL_EXAMPLES.md")
	if examplesText == "" {
		examplesText = i18n.T(i18n.KeySystemPromptXMLExamples)
	}
	taskProgressText := loadExternalFile(cwd, "TASK_PROGRESS.md")
	if taskProgressText == "" {
		taskProgressText = i18n.T(i18n.KeySystemPromptXMLTaskProgress)
	}
	editingFilesText := loadExternalFile(cwd, "EDITING_FILES.md")
	if editingFilesText == "" {
		editingFilesText = i18n.T(i18n.KeySystemPromptEditingFiles)
	}
	sb.WriteString(examplesText)
	sb.WriteString(taskProgressText)
	sb.WriteString(editingFilesText)

	return sb.String()
}

// buildReferenceFormat extracts the Usage section from the i18n tool description
// for the given tool name. Returns the Usage XML block if found, or a generic
// format string as fallback.
func buildReferenceFormat(toolName string) string {
	// Try to get the tool's usage example from i18n
	if key, ok := toolUsageKeyMap[toolName]; ok {
		example := i18n.T(key)
		if example != "" {
			// Extract the Usage section (everything after "Usage:")
			usageIdx := strings.Index(example, "Usage:")
			if usageIdx >= 0 {
				usageSection := example[usageIdx+len("Usage:"):]
				usageSection = strings.TrimSpace(usageSection)
				if usageSection != "" {
					return usageSection
				}
			}
		}
	}

	// Fallback: generic format
	return fmt.Sprintf("<%s>\n  <参数名1>参数值1</参数名1>\n  <参数名2>参数值2</参数名2>\n</%s>", toolName, toolName)
}

// buildXMLToolDescription builds the usage description for a single tool in XML format.
// Outputs the i18n usage example for the tool (Parameters and Usage sections).
func buildXMLToolDescription(tool llm.Tool, lang string) string {
	var sb strings.Builder

	// Append usage example from i18n if available
	if key, ok := toolUsageKeyMap[tool.Name]; ok {
		example := i18n.T(key)
		if example != "" {
			sb.WriteString(example)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
