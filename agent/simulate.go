// Author: L.Shuang
// Created: 2026-06-11
// Last Modified: 2026-06-11
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// ToolCallResult holds the result of a single tool call execution during simulation.
type ToolCallResult struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SimulateToolCall parses raw content as if it were an LLM response, extracts
// tool calls using the same parsing logic as the agent's normal LLM pipeline,
// and executes them through the same executeToolCall pipeline (including per-tool
// mode confirmation, timeout handling, and callback execution).
//
// The current tool call mode (XML or OpenAI) determines the parsing strategy:
//   - XML mode: uses ParseXMLToolCallsWithTools to parse XML-formatted tool calls
//   - OpenAI mode: expects a JSON array of ToolCall objects
//
// Results are NOT added to a.messages. This is a debug/testing tool.
// The input IS recorded in the REPL command history via the caller.
func (a *Agent) SimulateToolCall(ctx context.Context, rawContent string) ([]ToolCallResult, error) {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return nil, fmt.Errorf("content is empty")
	}

	// Determine the current tool call mode
	isXMLMode := false
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Current()
		if mode != nil && !mode.SendTools {
			isXMLMode = true
		}
	}

	// Step 1: Parse tool calls using the same logic as the LLM pipeline
	var toolCalls []llm.ToolCall
	if isXMLMode {
		// XML mode: reuse ParseXMLToolCallsWithTools with the full tool list
		tools := a.buildToolsInternal()
		cleanContent := stripREPLMaskMarkers(rawContent)
		xmlCalls := ParseXMLToolCallsWithTools(cleanContent, tools)

		// Filter out _xml_parse_error calls
		for _, c := range xmlCalls {
			if c.Name == "_xml_parse_error" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(c.Arguments), &args); err == nil {
					if errMsg, ok := args["error"].(string); ok {
						return nil, fmt.Errorf("XML parse error: %s", errMsg)
					}
				}
				return nil, fmt.Errorf("XML parse error for tag %s", c.Name)
			}
			toolCalls = append(toolCalls, c)
		}
	} else {
		// OpenAI mode: try to parse as JSON array of ToolCall
		// First try as single object
		var singleTC llm.ToolCall
		if err := json.Unmarshal([]byte(rawContent), &singleTC); err == nil && singleTC.Name != "" {
			toolCalls = append(toolCalls, singleTC)
		} else {
			// Try as array
			var arrTC []llm.ToolCall
			if err := json.Unmarshal([]byte(rawContent), &arrTC); err != nil {
				return nil, fmt.Errorf("cannot parse content as JSON tool call: %w", err)
			}
			toolCalls = arrTC
		}
	}

	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls parsed from content")
	}

	// Step 2: Execute each tool call through the same executeToolCall pipeline
	results := make([]ToolCallResult, 0, len(toolCalls))
	for _, tc := range toolCalls {
		log.Info("SimulateToolCall: executing tool %s (ID: %s)", tc.Name, tc.ID)

		result := ToolCallResult{
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}

		resp, err := a.executeToolCall(ctx, tc)
		if err != nil {
			errStr := err.Error()
			// Check if user cancelled the whole agent
			if strings.HasPrefix(errStr, "CANCEL_AGENT") {
				result.Error = "cancelled by user"
				results = append(results, result)
				return results, fmt.Errorf("simulation cancelled by user")
			}
			result.Error = errStr
			log.Error("SimulateToolCall: tool %s failed: %v", tc.Name, err)
		} else {
			result.Result = resp
		}

		results = append(results, result)
	}

	return results, nil
}
