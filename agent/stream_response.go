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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// streamLLMResponse streams the LLM response and returns the complete content, reasoning, and tool calls.
// If streaming fails, it falls back to non-streaming Chat.
// Before each call, it dynamically selects the appropriate model based on current context.
func (a *Agent) streamLLMResponse(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	log.Debug("Agent.streamLLMResponse: ENTER")

	// Dynamically select and switch to the appropriate model based on current context
	if modelCfg := a.selectModelForCall(); modelCfg != nil {
		log.Debug("Agent.streamLLMResponse: switching model to %s", modelCfg.Name)
		a.switchToModel(modelCfg)
	} else {
		log.Debug("Agent.streamLLMResponse: no model switch needed")
	}

	// Apply context limit to messages
	log.Debug("Agent.streamLLMResponse: building context messages (total a.messages=%d)", len(a.messages))
	contextMsgs := a.buildContextMessages()
	log.Debug("Agent.streamLLMResponse: context messages built, count=%d", len(contextMsgs))

	// Try streaming first
	log.Debug("Agent.streamLLMResponse: calling ChatStream with %d context messages and %d tools",
		len(contextMsgs), len(tools))
	eventCh, err := a.llmClient.ChatStream(ctx, contextMsgs, tools)
	if err != nil {
		// Fall back to non-streaming
		log.Debug("Agent.streamLLMResponse: ChatStream not available, falling back to non-streaming: %v", err)
		return a.nonStreamingFallback(ctx, tools, cb)
	}
	log.Debug("Agent.streamLLMResponse: ChatStream returned eventCh, waiting for events...")

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var toolCalls []llm.ToolCall

	// Track whether we saw any tool call events (even invalid ones) from the stream.
	// This helps distinguish between "LLM returned no tool calls" (final answer)
	// and "LLM returned tool calls but all were invalid" (should retry).
	hasToolCallEvents := false

	// Collect details about invalid tool calls for better error reporting.
	type invalidToolCallInfo struct {
		Name   string
		ID     string
		Issues []string
	}
	var invalidCalls []invalidToolCallInfo

	// Filter function for tool calls that may have incomplete data from stream deltas
	// (e.g., empty name, ID, or arguments which would cause API errors)
	isValidToolCall := func(tc llm.ToolCall) bool {
		valid := tc.Name != "" && tc.ID != "" && tc.Arguments != ""
		log.Debug("Agent.streamLLMResponse: isValidToolCall: name=%q, id=%q, args_len=%d → valid=%v",
			tc.Name, tc.ID, len(tc.Arguments), valid)
		return valid
	}

	eventCount := 0
	for event := range eventCh {
		eventCount++
		// Log every stream event for diagnosing infinite loop issues (FIX-97)
		eventName := "unknown"
		switch event.Type {
		case llm.StreamEventContent:
			eventName = "content"
		case llm.StreamEventReasoning:
			eventName = "reasoning"
		case llm.StreamEventToolCall:
			eventName = "tool_call"
		case llm.StreamEventDone:
			eventName = "done"
		case llm.StreamEventError:
			eventName = "error"
		}
		log.Debug("Agent.streamLLMResponse: event #%d: type=%s, content=%q, content_len=%d, done=%v, err=%v",
			eventCount, eventName, event.Content, len(event.Content), event.Done, event.Err)

		switch event.Type {
		case llm.StreamEventContent:
			log.Debug("Agent.streamLLMResponse: processing StreamEventContent, content_len=%d, contentBuilder_before=%d",
				len(event.Content), contentBuilder.Len())
			// FIX-179: Check for loop patterns in LLM output
			if a.loopDetectOn && a.loopDetector != nil {
				if err := a.loopDetector.AddChunk(event.Content, time.Now()); err != nil {
					// Loop detected! Set flag and return error for intervention
					a.loopDetectCrit = true
					log.Warn("Agent.streamLLMResponse: loop detected: %v", err)
					return "", "", nil, err
				}
			}
			contentBuilder.WriteString(event.Content)
			cb("content_chunk", event.Content)
			log.Debug("Agent.streamLLMResponse: contentBuilder now %d bytes", contentBuilder.Len())

		case llm.StreamEventReasoning:
			log.Debug("Agent.streamLLMResponse: processing StreamEventReasoning, content_len=%d, reasoningBuilder_before=%d",
				len(event.Content), reasoningBuilder.Len())
			reasoningBuilder.WriteString(event.Content)
			if a.showLlmThinking {
				cb("thinking_chunk", event.Content)
			}
			log.Debug("Agent.streamLLMResponse: reasoningBuilder now %d bytes", reasoningBuilder.Len())

		case llm.StreamEventToolCall:
			log.Debug("Agent.streamLLMResponse: processing StreamEventToolCall, toolCall=%v", event.ToolCall)
			hasToolCallEvents = true
			if event.ToolCall != nil {
				log.Debug("Agent.streamLLMResponse: toolCall name=%q, id=%q, args=%q",
					event.ToolCall.Name, event.ToolCall.ID, event.ToolCall.Arguments)
				if isValidToolCall(*event.ToolCall) {
					toolCalls = append(toolCalls, *event.ToolCall)
					log.Debug("Agent.streamLLMResponse: valid tool call added, total toolCalls=%d", len(toolCalls))
				} else {
					// Collect details about why this tool call is invalid
					info := invalidToolCallInfo{
						Name: event.ToolCall.Name,
						ID:   event.ToolCall.ID,
					}
					if event.ToolCall.Name == "" {
						info.Issues = append(info.Issues, "name is empty")
					}
					if event.ToolCall.ID == "" {
						info.Issues = append(info.Issues, "ID is empty")
					}
					if event.ToolCall.Arguments == "" {
						info.Issues = append(info.Issues, "arguments is empty")
					}
					invalidCalls = append(invalidCalls, info)
					log.Debug("Agent.streamLLMResponse: invalid tool call collected, issues=%v", info.Issues)
				}
			} else {
				log.Debug("Agent.streamLLMResponse: tool call event with nil ToolCall")
			}

		case llm.StreamEventDone:
			log.Debug("Agent.streamLLMResponse: processing StreamEventDone, contentBuilder=%d bytes, reasoningBuilder=%d bytes, toolCalls=%d",
				contentBuilder.Len(), reasoningBuilder.Len(), len(toolCalls))
			// Stream finished - tool calls are already accumulated from stream deltas.
			// No need for an extra non-streaming API call.
			finalContent := contentBuilder.String()
			finalReasoning := reasoningBuilder.String()

			// In XML mode, the LLM returns tool calls embedded in the content as XML tags.
			// Parse them here if no API-level tool calls were returned.
			if len(toolCalls) == 0 && a.toolCallModeMgr != nil {
				mode := a.toolCallModeMgr.Current()
				if mode != nil && !mode.SendTools {
					xmlCalls := ParseXMLToolCalls(finalContent)
					if len(xmlCalls) > 0 {
						toolCalls = xmlCalls
						log.Debug("Agent.streamLLMResponse: parsed %d XML tool calls from content", len(xmlCalls))
					}
				}
			}

			// Accumulate token usage from the stream response (if provided by the API).
			if event.Usage != nil {
				log.Debug("Agent.streamLLMResponse: token usage from stream: prompt=%d, completion=%d, total=%d",
					event.Usage.PromptTokens, event.Usage.CompletionTokens, event.Usage.TotalTokens)
				a.mu.Lock()
				a.totalPromptTokens += event.Usage.PromptTokens
				a.totalCompletionTokens += event.Usage.CompletionTokens
				a.totalTokens += event.Usage.TotalTokens
				// Persist token usage to database
				if a.store != nil {
					entry := &store.TokenUsageEntry{
						ID:               fmt.Sprintf("%020d", time.Now().UnixNano()),
						PromptTokens:     event.Usage.PromptTokens,
						CompletionTokens: event.Usage.CompletionTokens,
						TotalTokens:      event.Usage.TotalTokens,
						Timestamp:        time.Now(),
					}
					if err := a.store.SaveTokenUsage(entry); err != nil {
						log.Warn("Failed to save token usage: %v", err)
					}
				}
				a.mu.Unlock()
				log.Debug("Agent.streamLLMResponse: accumulated token usage from stream: prompt=%d, completion=%d, total=%d",
					a.totalPromptTokens, a.totalCompletionTokens, a.totalTokens)
			} else {
				log.Debug("Agent.streamLLMResponse: no token usage in stream Done event")
			}

			// If the LLM intended to call tools but all were invalid (e.g., empty arguments),
			// treat this as an error so the agent can retry rather than returning empty content.
			// Provide detailed feedback about which tool calls were invalid and why.
			if hasToolCallEvents && len(toolCalls) == 0 {
				log.Debug("Agent.streamLLMResponse: all tool calls were invalid, returning error")
				var sb strings.Builder
				sb.WriteString("LLM returned tool calls with invalid arguments (all filtered out). Details:\n")
				for _, ic := range invalidCalls {
					sb.WriteString(fmt.Sprintf("  - tool call"))
					if ic.Name != "" {
						sb.WriteString(fmt.Sprintf(" %q", ic.Name))
					}
					if ic.ID != "" {
						sb.WriteString(fmt.Sprintf(" (ID: %s)", ic.ID))
					}
					sb.WriteString(fmt.Sprintf(": %s\n", strings.Join(ic.Issues, ", ")))
				}
				sb.WriteString("Please check the tool definitions and ensure all required parameters are provided correctly.")
				return "", "", nil, errors.New(sb.String())
			}

			log.Debug("Agent.streamLLMResponse: returning finalContent=%q, finalReasoning_len=%d, toolCalls=%d",
				finalContent, len(finalReasoning), len(toolCalls))
			return finalContent, finalReasoning, toolCalls, nil

		case llm.StreamEventError:
			log.Debug("Agent.streamLLMResponse: StreamEventError: %v", event.Err)
			return "", "", nil, event.Err
		}
	}

	// If we get here, the channel closed without a Done event
	// Fall back to non-streaming
	log.Debug("Agent.streamLLMResponse: eventCh closed after %d events without Done event, falling back to non-streaming", eventCount)
	return a.nonStreamingFallback(ctx, tools, cb)
}
