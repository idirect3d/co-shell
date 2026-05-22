// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-05-22
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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/llm"
)

// ToolCallModeType defines the type of tool call mode.
type ToolCallModeType string

const (
	// ToolCallModeOpenAI uses the standard OpenAI API tool call mechanism.
	// Tools are sent as a "tools" parameter in the request body.
	// The LLM returns tool_calls in the response.
	ToolCallModeOpenAI ToolCallModeType = "openai"

	// ToolCallModeXML uses a custom XML format embedded in the system prompt.
	// No "tools" parameter is sent; instead, tool usage instructions are
	// described in XML format in the system prompt.
	// The LLM returns tool calls as XML tags in the response content.
	ToolCallModeXML ToolCallModeType = "xml"
)

// ToolCallMode defines a tool call mode with its behavior.
type ToolCallMode struct {
	// Type is the unique identifier for this mode.
	Type ToolCallModeType

	// Name is the human-readable display name.
	Name string

	// Description is a brief description of this mode.
	Description string

	// SendTools indicates whether to send the "tools" parameter in the LLM request.
	// OpenAI mode: true (send tools as JSON array)
	// XML mode: false (tools are described in system prompt)
	SendTools bool

	// SystemPromptKey is the i18n key for the tool usage section in the system prompt.
	// OpenAI mode: KeySystemPromptToolUsage (JSON format examples)
	// XML mode: KeySystemPromptToolUsageXML (XML format examples)
	SystemPromptKey string

	// ParseToolCalls parses the LLM response content to extract tool calls.
	// For OpenAI mode, this is not used (tool calls come from the API response).
	// For XML mode, this parses XML tags from the content.
	ParseToolCalls func(content string) ([]llm.ToolCall, error)
}

// ToolCallModeManager manages all available tool call modes.
type ToolCallModeManager struct {
	modes   map[ToolCallModeType]*ToolCallMode
	current ToolCallModeType
}

// NewToolCallModeManager creates a new manager with built-in modes.
func NewToolCallModeManager() *ToolCallModeManager {
	mgr := &ToolCallModeManager{
		modes:   make(map[ToolCallModeType]*ToolCallMode),
		current: ToolCallModeOpenAI,
	}

	// Register built-in OpenAI mode
	mgr.Register(&ToolCallMode{
		Type:            ToolCallModeOpenAI,
		Name:            "OpenAI API 标准",
		Description:     "使用 OpenAI 兼容 API 的标准 tool call 机制，通过 JSON 格式传递工具定义",
		SendTools:       true,
		SystemPromptKey: "system_prompt_tool_usage",
		ParseToolCalls:  nil, // not used for OpenAI mode
	})

	// Register built-in XML mode (Cline-style)
	mgr.Register(&ToolCallMode{
		Type:            ToolCallModeXML,
		Name:            "XML 自定义格式",
		Description:     "使用 XML 标签格式在 system prompt 中描述工具调用方法，LLM 返回 XML 格式的工具调用",
		SendTools:       false,
		SystemPromptKey: "system_prompt_tool_usage_xml",
		ParseToolCalls:  parseXMLToolCalls,
	})

	return mgr
}

// Register adds a custom tool call mode.
func (m *ToolCallModeManager) Register(mode *ToolCallMode) {
	m.modes[mode.Type] = mode
}

// Get returns the mode for the given type.
// Returns nil if the mode is not found.
func (m *ToolCallModeManager) Get(modeType ToolCallModeType) *ToolCallMode {
	return m.modes[modeType]
}

// Current returns the current tool call mode.
func (m *ToolCallModeManager) Current() *ToolCallMode {
	return m.modes[m.current]
}

// SetCurrent sets the current tool call mode by type.
// Returns false if the mode type is not registered.
func (m *ToolCallModeManager) SetCurrent(modeType ToolCallModeType) bool {
	if _, ok := m.modes[modeType]; ok {
		m.current = modeType
		return true
	}
	return false
}

// SetCurrentByString sets the current tool call mode from a string.
// Returns false if the string does not match any registered mode.
func (m *ToolCallModeManager) SetCurrentByString(s string) bool {
	switch strings.ToLower(s) {
	case "openai":
		return m.SetCurrent(ToolCallModeOpenAI)
	case "xml":
		return m.SetCurrent(ToolCallModeXML)
	default:
		// Try to match by type string directly
		for _, mode := range m.modes {
			if strings.EqualFold(string(mode.Type), s) {
				return m.SetCurrent(mode.Type)
			}
		}
		return false
	}
}

// List returns all registered modes.
func (m *ToolCallModeManager) List() []*ToolCallMode {
	result := make([]*ToolCallMode, 0, len(m.modes))
	for _, mode := range m.modes {
		result = append(result, mode)
	}
	return result
}

// parseXMLToolCalls parses XML-formatted tool calls from LLM response content.
// Expected format:
//
//	<tool_calls>
//	<invoke name="tool_name">
//	<parameter name="param1">value1</parameter>
//	<parameter name="param2">value2</parameter>
//	</invoke>
//	</tool_calls>
func parseXMLToolCalls(content string) ([]llm.ToolCall, error) {
	// Find the <tool_calls> block
	start := strings.Index(content, "<tool_calls>")
	if start < 0 {
		return nil, nil // no tool calls in this response
	}
	end := strings.LastIndex(content, "</tool_calls>")
	if end < 0 {
		return nil, fmt.Errorf("unclosed <tool_calls> tag")
	}

	xmlBlock := content[start : end+len("</tool_calls>")]

	// Parse the XML block
	type parameter struct {
		Name  string `xml:"name,attr"`
		Value string `xml:",chardata"`
	}

	type invoke struct {
		Name       string      `xml:"name,attr"`
		Parameters []parameter `xml:"parameter"`
	}

	type toolCalls struct {
		Invokes []invoke `xml:"invoke"`
	}

	var calls toolCalls
	if err := xml.Unmarshal([]byte(xmlBlock), &calls); err != nil {
		return nil, fmt.Errorf("cannot parse XML tool calls: %w", err)
	}

	if len(calls.Invokes) == 0 {
		return nil, nil
	}

	result := make([]llm.ToolCall, 0, len(calls.Invokes))
	for i, inv := range calls.Invokes {
		if inv.Name == "" {
			continue
		}

		// Build arguments as JSON string
		args := make(map[string]interface{})
		for _, p := range inv.Parameters {
			args[p.Name] = p.Value
		}

		// Serialize arguments to JSON
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal XML tool call arguments: %w", err)
		}

		result = append(result, llm.ToolCall{
			ID:        fmt.Sprintf("xml_call_%d", i),
			Name:      inv.Name,
			Arguments: string(argsJSON),
			Type:      "function",
		})
	}

	return result, nil
}
