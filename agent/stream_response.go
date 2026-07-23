// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-06-04
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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// InterruptedError is returned when the user presses ESC to interrupt the LLM stream.
type InterruptedError struct{}

func (e *InterruptedError) Error() string { return "user interrupted the LLM output" }

// CanceledError is returned when the user presses Ctrl+C to cancel the current task.
// Unlike InterruptedError, this causes immediate exit to the REPL prompt without
// any confirmation prompt (FEATURE-239).
type CanceledError struct{}

func (e *CanceledError) Error() string { return "user canceled the task via Ctrl+C" }

// lastNChars returns the last n characters of a string.
// If the string is shorter than n, the entire string is returned.
func lastNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// streamLLMResponse streams the LLM response and returns the complete content, reasoning, and tool calls.
// If streaming fails, it falls back to non-streaming Chat.
// Before each call, it dynamically selects the appropriate model based on current context.
// It also listens on the interrupt channel for user ESC keypress (FEATURE-201).
func (a *Agent) streamLLMResponse(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	log.Debug("Agent.streamLLMResponse: ENTER")

	// Reset per-stream-call flags
	a.mu.Lock()
	a.loopLongOutputTriggered = false
	a.loopJudgeSkipped = false
	a.streamCb = cb
	a.mu.Unlock()

	// Dynamically select and switch to the appropriate model based on current mode
	a.ApplyWorkModeConfig()

	// Apply context limit to messages
	log.Debug("Agent.streamLLMResponse: building context messages (total a.messages=%d)", len(a.messages))
	contextMsgs := a.buildContextMessages()
	log.Debug("Agent.streamLLMResponse: context messages built, count=%d", len(contextMsgs))

	// Record call start time for performance timing
	a.mu.Lock()
	a.llmCallStartTime = time.Now()
	a.mu.Unlock()

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

	// Log loop detector status at INFO level so it's always visible.
	// This helps confirm whether the loop detection mechanism is active.
	log.Info("Agent.streamLLMResponse: loopDetectOn=%v, loopDetector=%v",
		a.loopDetectOn, a.loopDetector != nil)

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

	// Track whether first token has arrived for timing
	firstTokenArrived := false

	eventCount := 0
	done := false
	for !done {
		select {
		case <-a.cancelCh:
			log.Debug("Agent.streamLLMResponse: received cancel signal (Ctrl+C), aborting stream")
			_ = contentBuilder.String()
			return "", "", nil, &CanceledError{}

		case <-a.interruptCh:
			log.Debug("Agent.streamLLMResponse: received interrupt signal, stopping stream")
			// Drain any remaining content from the contentBuilder
			_ = contentBuilder.String()
			return "", "", nil, &InterruptedError{}

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed
				done = true
				break
			}
			eventCount++

			// Record first token arrival time for performance timing
			if !firstTokenArrived && (event.Type == llm.StreamEventContent || event.Type == llm.StreamEventReasoning) {
				firstTokenArrived = true
				a.mu.Lock()
				a.firstTokenTime = time.Now()
				a.mu.Unlock()
			}

			switch event.Type {
			case llm.StreamEventContent:
				contentBuilder.WriteString(event.Content)
				cb("content_chunk", event.Content)

				// FIX-179: Check for loop patterns in LLM output.
				// Skip detection if judge already said "not a loop" in this stream call.
				if a.loopDetectOn && a.loopDetector != nil && !a.loopJudgeSkipped {
					if err := a.loopDetector.AddChunk(event.Content, time.Now()); err != nil {
						log.Warn("Agent.streamLLMResponse: loop detected: %v", err)
						a.handleLoopDetection(contentBuilder.String(), reasoningBuilder.String(), err)
					}
				}

				// Check long output threshold: when accumulated output exceeds
				// LoopLongOutputThreshold, trigger secondary judgment to check
				// if the LLM is stuck or making no progress. 0 = disabled.
				if a.loopDetectOn && a.cfg != nil && a.cfg.LLM.LoopLongOutputThreshold > 0 {
					threshold := a.cfg.LLM.LoopLongOutputThreshold
					totalLen := contentBuilder.Len() + reasoningBuilder.Len()
					if totalLen >= threshold && !a.loopLongOutputTriggered {
						a.loopLongOutputTriggered = true
						log.Info("Agent.streamLLMResponse: long output threshold reached (%d >= %d chars), triggering loop judgment",
							totalLen, threshold)
						longErr := fmt.Errorf("LLM output exceeds %d characters without calling any tool, triggering loop judgment", threshold)
						a.handleLoopDetection(contentBuilder.String(), reasoningBuilder.String(), longErr)
					}
				}

				// Check if sync mode detected a loop (non-judge path) and break immediately.
				if a.loopDetectSyncErr != nil {
					log.Warn("Agent.streamLLMResponse: sync loop detection triggered, aborting stream")
					finalContent := contentBuilder.String()
					finalReasoning := reasoningBuilder.String()
					a.mu.Lock()
					a.lastLlmOutput = finalContent
					syncErr := a.loopDetectSyncErr
					a.loopDetectSyncErr = nil
					a.loopDetectCrit = true
					a.mu.Unlock()
					return finalContent, finalReasoning, nil, syncErr
				}

			case llm.StreamEventReasoning:
				reasoningBuilder.WriteString(event.Content)
				if a.showLlmThinking {
					cb("thinking_chunk", event.Content)
				}

				// FIX-179: Check for loop patterns in reasoning output too.
				if a.loopDetectOn && a.loopDetector != nil {
					if err := a.loopDetector.AddChunk(event.Content, time.Now()); err != nil {
						log.Warn("Agent.streamLLMResponse: loop detected in reasoning: %v", err)
						a.handleLoopDetection(contentBuilder.String(), reasoningBuilder.String(), err)
					}
				}

				// Check if sync mode detected a loop and break immediately.
				if a.loopDetectSyncErr != nil {
					log.Warn("Agent.streamLLMResponse: sync loop detection triggered (reasoning), aborting stream")
					finalContent := contentBuilder.String()
					finalReasoning := reasoningBuilder.String()
					a.mu.Lock()
					a.lastLlmOutput = finalReasoning
					syncErr := a.loopDetectSyncErr
					a.loopDetectSyncErr = nil
					a.loopDetectCrit = true
					a.mu.Unlock()
					return finalContent, finalReasoning, nil, syncErr
				}

			case llm.StreamEventToolCall:
				// In XML mode, strictly ignore API-level tool_calls from the LLM response.
				// Tool calls are only parsed from the content as XML tags.
				if a.toolCallModeMgr != nil {
					mode := a.toolCallModeMgr.Current()
					if mode != nil && !mode.SendTools {
						log.Debug("Agent.streamLLMResponse: ignoring StreamEventToolCall in XML mode")
						continue
					}
				}
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
				// Record stream end time for performance timing
				a.mu.Lock()
				a.llmStreamEndTime = time.Now()
				a.mu.Unlock()

				log.Debug("Agent.streamLLMResponse: processing StreamEventDone, contentBuilder=%d bytes, reasoningBuilder=%d bytes, toolCalls=%d",
					contentBuilder.Len(), reasoningBuilder.Len(), len(toolCalls))
				// Stream finished - tool calls are already accumulated from stream deltas.
				// No need for an extra non-streaming API call.
				finalContent := contentBuilder.String()
				finalReasoning := reasoningBuilder.String()

				// In XML mode, the LLM returns tool calls embedded in the content as XML tags.
				// We ALWAYS parse XML tool calls from content in XML mode, and IGNORE any
				// API-level tool_calls. This prevents conflicts where the LLM returns both
				// XML tool calls in content AND API-level tool_calls simultaneously.
				//
				// Before parsing, strip REPL input masking markers (|mask_start|...|mask_end|)
				// that may have leaked into the LLM context via shell session scrollback output.
				// These markers are injected by the external REPL (go-prompt) during input masking
				// and are not crafted by the LLM, so they should never trigger XML parse errors.
				if a.toolCallModeMgr != nil {
					mode := a.toolCallModeMgr.Current()
					if mode != nil && !mode.SendTools {
						cleanContent := stripREPLMaskMarkers(finalContent)
						// Log the first 200 chars of cleaned content to help debug parsing issues
						contentPreview := cleanContent
						if len(contentPreview) > 200 {
							contentPreview = contentPreview[:200] + "..."
						}
						log.Info("Agent.streamLLMResponse: XML mode parsing finalContent (%d bytes), preview: %s",
							len(cleanContent), contentPreview)
						// FEATURE-287: Use buildToolsInternal() instead of the empty tools parameter
						// so ParseXMLToolCallsWithTools has the full known tool list and can
						// skip unknown HTML tags (e.g., <div id="mainText">) in LLM explanatory text.
						fullTools := a.buildToolsInternal()
						xmlCalls := ParseXMLToolCallsWithTools(cleanContent, fullTools)
						log.Info("Agent.streamLLMResponse: ParseXMLToolCallsWithTools returned %d call(s)", len(xmlCalls))
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
								// Store parse errors as structured JSON in taskInstructionCache,
								// including the tool name so RunStream can look up the correct format.
								// Format: {"tool": "excel_edit", "error": "...details..."}
								// When multiple errors belong to different tools, separate with \n---\n.
								var cacheLines []string
								for _, pe := range parseErrors {
									// Extract tool name from the error message (format: "tag": "tool_name")
									toolName := ""
									if tagStart := strings.Index(pe, `"tag": "`); tagStart >= 0 {
										tagStart += len(`"tag": "`)
										if tagEnd := strings.Index(pe[tagStart:], `"`); tagEnd >= 0 {
											toolName = pe[tagStart : tagStart+tagEnd]
										}
									}
									if toolName == "" {
										cacheLines = append(cacheLines, fmt.Sprintf(`{"tool": "", "error": %q}`, pe))
									} else {
										cacheLines = append(cacheLines, fmt.Sprintf(`{"tool": %q, "error": %q}`, toolName, pe))
									}
								}
								a.taskInstructionCache.WriteString(strings.Join(cacheLines, "\n---\n"))
								finalContent = ""
								toolCalls = nil
								log.Warn("Agent.streamLLMResponse: %d XML parse errors stored in taskInstructionCache (no tool calls)",
									len(parseErrors))
								for _, pe := range parseErrors {
									log.Warn("  Parse error: %s", pe)
								}
							} else {
								toolCalls = validCalls
								log.Info("Agent.streamLLMResponse: parsed %d valid XML tool calls from content", len(validCalls))
								for _, c := range validCalls {
									log.Info("  Tool call: %s (args: %d bytes)", c.Name, len(c.Arguments))
								}
							}
						} else {
							// No XML tool calls found; clear any API-level tool calls in XML mode
							toolCalls = nil
							log.Info("Agent.streamLLMResponse: 0 XML tool calls found in content (tools list len=%d)", len(tools))
						}
					}
				}

				// Save the final content as the last LLM output for loop judgment (FEATURE-241).
				a.mu.Lock()
				a.lastLlmOutput = finalContent
				a.mu.Unlock()

				// Accumulate token usage from the stream response (if provided by the API).
				if event.Usage != nil {
					log.Debug("Agent.streamLLMResponse: token usage from stream: prompt=%d, completion=%d, total=%d",
						event.Usage.PromptTokens, event.Usage.CompletionTokens, event.Usage.TotalTokens)
					a.mu.Lock()
					a.totalPromptTokens += event.Usage.PromptTokens
					a.totalCompletionTokens += event.Usage.CompletionTokens
					a.totalTokens += event.Usage.TotalTokens
					a.taskPromptTokens += event.Usage.PromptTokens
					a.taskCompletionTokens += event.Usage.CompletionTokens
					a.taskTokens += event.Usage.TotalTokens
					a.iterPromptTokens = event.Usage.PromptTokens
					a.iterCompletionTokens = event.Usage.CompletionTokens
					a.iterTokens = event.Usage.TotalTokens
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
	}

	// If we get here, the channel closed without a Done event
	// Fall back to non-streaming
	log.Debug("Agent.streamLLMResponse: eventCh closed after %d events without Done event, falling back to non-streaming", eventCount)
	return a.nonStreamingFallback(ctx, tools, cb)
}
