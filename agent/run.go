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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// Run processes a user input through the agent loop without streaming.
// It returns the final response content.
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	// Save raw user input for potential use in system prompt.
	a.lastUserInput = userInput

	a.mu.Lock()
	// If there are image paths, create a multimodal message with cached images
	if len(a.imagePaths) > 0 {
		multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)
		if err != nil {
			a.mu.Unlock()
			return "", fmt.Errorf("cannot build multimodal message: %w", err)
		}
		a.messages = append(a.messages, multimodalMsg)
		// Keep imagePaths for reuse in subsequent conversations
	} else {
		// Add user message to history with template formatting
		// FEATURE-248: In XML mode, use ContentParts for structured user messages.
		if a.isXMLMode() {
			xmlUserMsg := a.buildXMLUserMessage(userInput, len(a.messages))
			a.messages = append(a.messages, xmlUserMsg)
		} else {
			formattedInput := a.formatUserMessage(userInput, len(a.messages))
			a.messages = append(a.messages, llm.Message{Role: "user", Content: formattedInput})
		}
		// Sync to memory (content without timestamp prefix, Datetime field stores the time)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
				log.Warn("Failed to save user message to memory: %v", err)
			}
		}
	}
	a.mu.Unlock()

	log.Info("Agent.Run: user input: %s", userInput)

	// Rebuild system prompt to refresh {TASK} with current context
	a.rebuildSystemPrompt()

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Dynamically select and switch to the appropriate model based on current mode
		a.ApplyWorkModeConfig()

		// Call LLM
		resp, err := a.llmClient.Chat(ctx, a.messages, tools)

		if err != nil {
			log.Error("Agent.Run: LLM call failed at iteration %d: %v", iteration, err)
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// Accumulate token usage from API response
		if resp.Usage != nil {
			a.mu.Lock()
			a.totalPromptTokens += resp.Usage.PromptTokens
			a.totalCompletionTokens += resp.Usage.CompletionTokens
			a.totalTokens += resp.Usage.TotalTokens
			// Persist token usage to database
			if a.store != nil {
				entry := &store.TokenUsageEntry{
					ID:               fmt.Sprintf("%020d", time.Now().UnixNano()),
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
					Timestamp:        time.Now(),
				}
				if err := a.store.SaveTokenUsage(entry); err != nil {
					log.Warn("Failed to save token usage: %v", err)
				}
			}
			a.mu.Unlock()
			log.Debug("Agent.Run: accumulated token usage: prompt=%d, completion=%d, total=%d",
				a.totalPromptTokens, a.totalCompletionTokens, a.totalTokens)
		} else {
			a.mu.Unlock()
		}

		// In XML mode, the LLM returns tool calls embedded in the content as XML tags.
		// We ALWAYS parse XML tool calls from content in XML mode, and IGNORE any
		// API-level tool_calls. This prevents conflicts where the LLM returns both
		// XML tool calls in content AND API-level tool_calls simultaneously.
		toolCalls := resp.ToolCalls
		if a.toolCallModeMgr != nil {
			mode := a.toolCallModeMgr.Current()
			if mode != nil && !mode.SendTools {
				xmlCalls := ParseXMLToolCalls(resp.Content)
				if len(xmlCalls) > 0 {
					// Filter out _xml_parse_error calls - these are parse errors that
					// should be returned directly to the LLM as feedback, not executed.
					var validCalls []llm.ToolCall
					var parseErrors []string
					for _, c := range xmlCalls {
						if c.Name == "_xml_parse_error" {
							var args map[string]interface{}
							if err := json.Unmarshal([]byte(c.Arguments), &args); err == nil {
								if errMsg, ok := args["error"].(string); ok {
									parseErrors = append(parseErrors, errMsg)
								}
							}
						} else {
							validCalls = append(validCalls, c)
						}
					}
					if len(parseErrors) > 0 {
						// Return parse errors directly to the LLM as assistant content,
						// so it can see and fix the format issues immediately.
						resp.Content = strings.Join(parseErrors, "\n---\n")
						toolCalls = nil
						log.Debug("Agent.Run: returning %d XML parse errors to LLM as content (no tool calls)",
							len(parseErrors))
					} else {
						toolCalls = validCalls
						log.Debug("Agent.Run: parsed %d XML tool calls from content (ignored %d API-level tool calls)",
							len(validCalls), len(toolCalls))
					}
				} else {
					// No XML tool calls found; clear any API-level tool calls in XML mode
					toolCalls = nil
				}
			}
		}

		// If no tool calls, this is the final answer
		if len(toolCalls) == 0 {
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          resp.Content,
				ReasoningContent: resp.ReasoningContent,
			})
			// Sync to memory (content without timestamp prefix)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, resp.Content, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			a.mu.Unlock()
			log.Info("Agent.Run: completed after %d iterations", iteration+1)
			return resp.Content, nil
		}

		// Determine if we're in XML mode (no API-level tool calls)
		isXMLMode := false
		if a.toolCallModeMgr != nil {
			mode := a.toolCallModeMgr.Current()
			if mode != nil && !mode.SendTools {
				isXMLMode = true
			}
		}

		// Add assistant message with tool calls
		// In XML mode, do NOT set ToolCalls on the assistant message — tool calls
		// are embedded in the content as XML tags and the LLM expects results
		// returned as user messages (not tool messages).
		a.mu.Lock()
		assistantMsg := llm.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent,
		}
		if !isXMLMode {
			assistantMsg.ToolCalls = toolCalls
		}
		a.messages = append(a.messages, assistantMsg)
		// Sync to memory (content without timestamp prefix)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage(a.name, resp.Content, time.Now()); err != nil {
				log.Warn("Failed to save assistant message to memory: %v", err)
			}
		}
		a.mu.Unlock()

		// Execute each tool call
		for _, tc := range toolCalls {
			log.Info("Agent.Run: executing tool %s (ID: %s)", tc.Name, tc.ID)
			result, err := a.executeToolCall(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				log.Error("Agent.Run: tool %s failed: %v", tc.Name, err)
			}

			// Add tool result to messages
			// If the result is empty, provide a clear message to the LLM
			toolContent := result
			if toolContent == "" {
				toolContent = "（工具调用无输出）"
			}

			if isXMLMode {
				// In XML mode, return tool results as user messages with ContentParts structure.
				// The tool result becomes a separate text part in the ContentParts array.
				toolResultMsg := a.buildXMLToolResultMessage(tc.Name, tc.Arguments, toolContent, len(a.messages))
				a.mu.Lock()
				a.messages = append(a.messages, toolResultMsg)
				a.mu.Unlock()
			} else {
				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:       "tool",
					Content:    toolContent,
					ToolCallID: tc.ID,
				})
				a.mu.Unlock()
			}
		}

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		// Only "task" mode auto-adjusts the pointer — "window" and "smart" modes do not.
		a.mu.Lock()
		if a.needAdjustPointer {
			contextStartMode := "smart"
			if a.cfg != nil && a.cfg.LLM.ContextStartMode != "" {
				contextStartMode = a.cfg.LLM.ContextStartMode
			}
			if contextStartMode == "task" {
				a.messagePointer = len(a.messages) - 1
				a.adjustMessagePointer()
			}
			a.needAdjustPointer = false
		}
		a.mu.Unlock()
	}

	log.Error("Agent.Run: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}
