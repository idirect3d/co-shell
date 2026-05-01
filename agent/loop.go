// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/memory"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/subagent"
	"github.com/idirect3d/co-shell/taskplan"
)

// StreamCallback is a function called for each streaming event from the LLM.
type StreamCallback func(eventType string, content string)

// CmdConfirmResult represents the result of a command confirmation prompt.
type CmdConfirmResult int

const (
	CmdConfirmApprove      CmdConfirmResult = iota
	CmdConfirmApproveAll                    // Approve all commands for this request
	CmdConfirmApproveCount                  // Approve N commands (user entered a number)
	CmdConfirmCancel                        // User cancelled, return to REPL
	CmdConfirmModify                        // User entered custom input to modify the command
)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	mu              sync.Mutex
	llmClient       llm.Client
	mcpMgr          *mcp.Manager
	store           *store.Store
	memoryManager   *memory.Manager
	systemPrompt    string
	messages        []llm.Message
	showThinking    bool
	showCommand     bool
	showOutput      bool
	maxIterations   int
	confirmCommand  bool
	approveAll      bool           // if true, skip confirmation for all commands in this request
	approveCount    int            // remaining number of commands to auto-approve (decremented on each use)
	cfg             *config.Config // configuration for timeout settings
	resultMode      config.ResultMode
	outputMode      config.OutputMode
	rules           string // user-defined rules for rebuilding system prompt
	subAgentMgr     *subagent.Manager
	taskPlanMgr     *taskplan.Manager
	scheduler       *scheduler.Scheduler
	name            string   // agent name for identification (default: "co-shell")
	imagePaths      []string // paths to image files for multimodal input
	workspacePath   string   // workspace root path for loading external config files
	memoryEnabled   bool     // whether persistent memory tools are enabled
	planEnabled     bool     // whether task plan tools are enabled
	subAgentEnabled bool     // whether sub-agent tools are enabled

	// messagePointer is the index in a.messages that marks the starting position
	// for sending to LLM. Messages before this index are ignored when building
	// context for LLM calls. When a new checklist is created or updated, the
	// pointer is moved to the end, effectively ignoring prior conversation.
	messagePointer int

	// needAdjustPointer is set by createTaskPlanTool/insertTaskStepsTool/removeTaskStepsTool
	// when the task plan is successfully modified. The agent loop checks this flag after
	// all tool messages have been appended, and adjusts messagePointer to skip past
	// the tool messages, so the next LLM iteration starts fresh from the checklist context.
	needAdjustPointer bool

	// errorCounter tracks the number of times each distinct error message has occurred
	// during the current request. Key is the error message string, value is the count.
	// Reset at the start of each RunStream call.
	errorCounter map[string]int

	// errorApproveAll is set to true when the user chooses to ignore all error limits
	// for the current request.
	errorApproveAll bool
}

func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
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
		// Add user message to history with timestamp prefix
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{Role: "user", Content: tsPrefix + userInput})
		// Sync to memory (content without timestamp prefix, Datetime field stores the time)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
				log.Warn("Failed to save user message to memory: %v", err)
			}
		}
	}
	a.mu.Unlock()

	log.Info("Agent.Run: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Call LLM
		resp, err := a.llmClient.Chat(ctx, a.messages, tools)

		if err != nil {
			log.Error("Agent.Run: LLM call failed at iteration %d: %v", iteration, err)
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// If no tool calls, this is the final answer
		if len(resp.ToolCalls) == 0 {
			a.mu.Lock()
			tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          tsPrefix + resp.Content,
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

		// Add assistant message with tool calls
		a.mu.Lock()
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          tsPrefix + resp.Content,
			ToolCalls:        resp.ToolCalls,
			ReasoningContent: resp.ReasoningContent,
		})
		// Sync to memory (content without timestamp prefix)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage(a.name, resp.Content, time.Now()); err != nil {
				log.Warn("Failed to save assistant message to memory: %v", err)
			}
		}
		a.mu.Unlock()

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
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
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
			})
			a.mu.Unlock()
		}

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		a.mu.Lock()
		if a.needAdjustPointer {
			a.messagePointer = len(a.messages) - 1
			a.adjustMessagePointer()
			a.needAdjustPointer = false
		}
		a.mu.Unlock()
	}

	log.Error("Agent.Run: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// RunStream processes a user input through the agent loop with streaming output.
// It sends stream events to the provided callback function.
func (a *Agent) RunStream(ctx context.Context, userInput string, cb StreamCallback) (string, error) {
	// Reset approveAll and error tracking flags for each new request
	a.approveAll = false
	a.errorCounter = make(map[string]int)
	a.errorApproveAll = false

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
		// Add user message to history with timestamp prefix
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{Role: "user", Content: tsPrefix + userInput})
		// Sync to memory (content without timestamp prefix, Datetime field stores the time)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
				log.Warn("Failed to save user message to memory: %v", err)
			}
		}
	}
	a.mu.Unlock()

	log.Info("Agent.RunStream: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Step 1: Stream the LLM response
		var finalContent, finalReasoning string
		var toolCalls []llm.ToolCall
		var streamErr error

		finalContent, finalReasoning, toolCalls, streamErr = a.streamLLMResponse(ctx, tools, cb)
		if streamErr != nil {
			// Track error count for this request
			errMsg := streamErr.Error()
			a.errorCounter[errMsg]++
			singleCount := a.errorCounter[errMsg]
			typeCount := len(a.errorCounter)

			// Get configured limits
			maxSingle := 10
			maxType := 100
			if a.cfg != nil {
				if a.cfg.LLM.ErrorMaxSingleCount > 0 {
					maxSingle = a.cfg.LLM.ErrorMaxSingleCount
				}
				if a.cfg.LLM.ErrorMaxTypeCount > 0 {
					maxType = a.cfg.LLM.ErrorMaxTypeCount
				}
			}

			// Check if we need to prompt the user
			needUserPrompt := false
			promptReason := ""

			if singleCount >= maxSingle && !a.errorApproveAll {
				needUserPrompt = true
				promptReason = fmt.Sprintf("相同错误已出现 %d 次（上限 %d 次）", singleCount, maxSingle)
			} else if typeCount >= maxType && !a.errorApproveAll {
				needUserPrompt = true
				promptReason = fmt.Sprintf("不同错误类型已达 %d 种（上限 %d 种）", typeCount, maxType)
			}

			if needUserPrompt {
				// Prompt user for action
				fmt.Printf("\n⚠️ 错误反复出现: %s\n", promptReason)
				fmt.Printf("  最新错误: %v\n", streamErr)
				fmt.Println()
				fmt.Println(i18n.T(i18n.KeyErrorRiskWarning))
				fmt.Println()
				fmt.Println("  请选择操作:")
				fmt.Println("  [Enter] 继续让 LLM 尝试处理")
				fmt.Println("  [C] 取消，返回 REPL")
				fmt.Println("  [A] 忽略限制，继续执行")
				fmt.Println()
				fmt.Print("  请选择 (Enter/C/A): ")

				var lineBuf []byte
				buf := make([]byte, 1)
				for {
					n, err := os.Stdin.Read(buf)
					if err != nil || n == 0 {
						break
					}
					if buf[0] == '\n' || buf[0] == '\r' {
						break
					}
					lineBuf = append(lineBuf, buf[0])
				}

				userChoice := strings.TrimSpace(string(lineBuf))
				lower := strings.ToLower(userChoice)

				if lower == "c" {
					// User cancelled, return to REPL
					cb("info", "\n🛑 用户取消了操作\n")
					return "", nil
				} else if lower == "a" {
					// User chose to ignore all error limits
					a.errorApproveAll = true
					fmt.Println("\n✅ 已忽略错误限制，继续执行")
				} else {
					// Continue (Enter pressed)
					fmt.Println("\n✅ 继续让 LLM 尝试处理")
				}
			}

			// Feed all errors back to the LLM so it can decide how to handle them.
			// The LLM can determine whether the error is recoverable (e.g., invalid arguments,
			// temporary timeout) and retry with corrections, or unrecoverable (e.g., auth failure,
			// model not found) and report to the user.
			log.Warn("Agent.RunStream: stream error at iteration %d: %v, feeding back to LLM", iteration, streamErr)
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"注意：刚才的 LLM 调用返回了错误，请根据错误信息判断如何处理。\n"+
						"如果错误是可恢复的（如参数格式问题、临时超时），请修正后重试。\n"+
						"如果错误是不可恢复的（如认证失败、模型不存在），请向用户报告错误并终止。\n\n"+
						"错误信息：%s",
					streamErr.Error(),
				),
			})
			a.mu.Unlock()
			cb("info", fmt.Sprintf("\n⚠️ LLM 调用出错: %v\n正在请求 LLM 判断如何处理...\n", streamErr))
			continue
		}

		// Step 2: If no tool calls, this is the final answer
		if len(toolCalls) == 0 {
			cb("done", "")

			a.mu.Lock()
			tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          tsPrefix + finalContent,
				ReasoningContent: finalReasoning,
			})
			// Sync to memory (content without timestamp prefix)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			a.mu.Unlock()
			log.Info("Agent.RunStream: completed after %d iterations", iteration+1)
			return finalContent, nil
		}

		// Step 3: First add assistant message with tool_calls to history
		// This must come BEFORE tool result messages to satisfy the API requirement
		// that tool messages must follow a message with tool_calls.
		a.mu.Lock()
		assistantMsgIdx := len(a.messages)
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          tsPrefix + finalContent,
			ToolCalls:        toolCalls,
			ReasoningContent: finalReasoning,
		})
		// Sync to memory (content without timestamp prefix)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
				log.Warn("Failed to save assistant message to memory: %v", err)
			}
		}
		a.mu.Unlock()

		// Step 4: Execute tool calls and add results
		modifyRequested := false
		cancelled := false
		for _, tc := range toolCalls {
			// Show command if enabled (normal/debug mode only)
			if a.outputMode >= config.OutputModeNormal && a.showCommand && tc.Name == "execute_command" {
				var cmdArgs map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &cmdArgs); err == nil {
					if cmd, ok := cmdArgs["command"].(string); ok {
						cb("command", cmd)
					}
				}
			}

			// Show tool call name (normal/debug mode only)
			if a.outputMode >= config.OutputModeNormal {
				cb("tool_call", fmt.Sprintf("🛠 Calling tool: %s\n", tc.Name))
			}

			log.Info("Agent.RunStream: executing tool %s (ID: %s)", tc.Name, tc.ID)
			result, execErr := a.executeToolCall(ctx, tc)
			if execErr != nil {
				errStr := execErr.Error()
				// Check if user cancelled
				if strings.HasPrefix(errStr, "CANCEL_AGENT") {
					cancelled = true
					// Remove the incomplete assistant message (with tool_calls) from history
					a.mu.Lock()
					a.messages = a.messages[:assistantMsgIdx]
					a.mu.Unlock()
					break
				}
				// Check if this is a USER_MODIFY_REQUEST (user wants to modify and re-evaluate)
				if strings.HasPrefix(errStr, "USER_MODIFY_REQUEST:") {
					modifyRequested = true
					modifyInput := strings.TrimPrefix(errStr, "USER_MODIFY_REQUEST:")
					// Remove the incomplete assistant message (with tool_calls) from history
					// since its tool_calls were not fully executed.
					a.mu.Lock()
					a.messages = a.messages[:assistantMsgIdx]
					// Add the user's modification as a new user message
					a.messages = append(a.messages, llm.Message{
						Role:    "user",
						Content: modifyInput,
					})
					a.mu.Unlock()
					cb("info", fmt.Sprintf("\n🔄 用户补充说明: %s\n", modifyInput))
					break
				}
				result = fmt.Sprintf("Error: %v", execErr)
				log.Error("Agent.RunStream: tool %s failed: %v", tc.Name, execErr)
			}

			// Show command output if enabled (debug mode only)
			if a.outputMode >= config.OutputModeDebug && a.showOutput && tc.Name == "execute_command" && result != "" {
				cb("output", result)
			}

			// If the result is empty, provide a clear message to the LLM
			toolContent := result
			if toolContent == "" {
				toolContent = "（工具调用无输出）"
			}

			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
			})
			a.mu.Unlock()
		}

		// If user cancelled, return to REPL
		if cancelled {
			return "", nil
		}

		// If user requested modification, continue the loop to re-ask the LLM
		if modifyRequested {
			continue
		}

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		a.mu.Lock()
		if a.needAdjustPointer {
			a.messagePointer = len(a.messages) - 1
			a.adjustMessagePointer()
			a.needAdjustPointer = false
		}
		a.mu.Unlock()

	}

	log.Error("Agent.RunStream: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// buildContextMessages returns a truncated message list based on ContextLimit and messagePointer.
// Message layout: [0]=system, [1..n-2]=history, [n-1]=current user input
// The current user input (last message) is ALWAYS kept.
// ContextLimit == 0: only system prompt + current user input (no history)
// ContextLimit == -1: all messages (unlimited)
// ContextLimit > 0: system prompt + current user input + last N history messages
// If messagePointer > 0, messages before the pointer are ignored (the pointer message
// and everything after it are kept). This is used when a checklist is created/updated
// to focus the LLM on the current task plan.
// Each message's content is prefixed with its original index in a.messages,
// e.g. "123: 2026-05-01 12:09:24 - ...", to help the LLM understand the conversation order.
func (a *Agent) buildContextMessages() []llm.Message {
	if a.cfg == nil || a.cfg.LLM.ContextLimit == -1 {
		return a.addIndexPrefixToMessages(a.messages, 0)
	}

	// Always keep system prompt (first message)
	if len(a.messages) <= 1 {
		return a.messages
	}

	systemMsg := a.messages[0]

	// The last message is always the current user input, always keep it
	currentMsg := a.messages[len(a.messages)-1]

	// Determine the effective start index based on messagePointer
	// If pointer > 0, start from pointer (ignore messages before it)
	startIdx := 1
	if a.messagePointer > 0 && a.messagePointer < len(a.messages) {
		startIdx = a.messagePointer
	}

	// History messages are between startIdx and current user input
	historyMsgs := a.messages[startIdx : len(a.messages)-1]

	if a.cfg.LLM.ContextLimit == 0 {
		// Only system prompt + current user input, no history
		result := []llm.Message{systemMsg, currentMsg}
		return a.addIndexPrefixToMessages(result, 0)
	}

	// Keep last N history messages
	if len(historyMsgs) > a.cfg.LLM.ContextLimit {
		// When truncating history, we need to adjust the startIdx for prefix calculation
		truncatedCount := len(historyMsgs) - a.cfg.LLM.ContextLimit
		historyMsgs = historyMsgs[truncatedCount:]
		startIdx += truncatedCount
	}

	result := make([]llm.Message, 0, 2+len(historyMsgs))
	result = append(result, systemMsg)
	result = append(result, historyMsgs...)
	result = append(result, currentMsg)
	return a.addIndexPrefixToMessages(result, startIdx)
}

// addIndexPrefixToMessages adds the original message index prefix to each message's content.
// The format is: "index: content"
// For example: "123: 2026-05-01 12:09:24 - 现在来更新主报告。"
// The index is the position in a.messages (0-based), which helps the LLM
// understand the conversation order even when context truncation is applied.
// System messages are not prefixed (they are always at index 0 and have no timestamp).
// startIdx is the index in a.messages where msgs[0] corresponds to.
// If startIdx < 0, the function falls back to content-based matching (legacy behavior).
func (a *Agent) addIndexPrefixToMessages(msgs []llm.Message, startIdx int) []llm.Message {
	result := make([]llm.Message, len(msgs))
	for i, msg := range msgs {
		// Determine the original index
		origIdx := -1
		if startIdx >= 0 {
			// Use sequential indexing starting from startIdx
			origIdx = startIdx + i
		} else {
			// Fallback: find the original index in a.messages by matching content and role
			a.mu.Lock()
			for j := range a.messages {
				if a.messages[j].Role == msg.Role && a.messages[j].Content == msg.Content {
					origIdx = j
					break
				}
			}
			a.mu.Unlock()
		}

		if origIdx >= 0 && msg.Role != "system" {
			// Add index prefix before the content
			result[i] = msg
			result[i].Content = fmt.Sprintf("%d: %s", origIdx, msg.Content)
		} else {
			result[i] = msg
		}
	}
	return result
}

// streamLLMResponse streams the LLM response and returns the complete content, reasoning, and tool calls.
// If streaming fails, it falls back to non-streaming Chat.
func (a *Agent) streamLLMResponse(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	// Apply context limit to messages
	contextMsgs := a.buildContextMessages()

	// Try streaming first
	eventCh, err := a.llmClient.ChatStream(ctx, contextMsgs, tools)
	if err != nil {
		// Fall back to non-streaming
		log.Debug("ChatStream not available, falling back to non-streaming: %v", err)
		return a.nonStreamingFallback(ctx, tools, cb)
	}

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
		return tc.Name != "" && tc.ID != "" && tc.Arguments != ""
	}

	for event := range eventCh {
		switch event.Type {
		case llm.StreamEventContent:
			contentBuilder.WriteString(event.Content)
			cb("content_chunk", event.Content)

		case llm.StreamEventReasoning:
			reasoningBuilder.WriteString(event.Content)
			if a.showThinking {
				cb("thinking_chunk", event.Content)
			}

		case llm.StreamEventToolCall:
			hasToolCallEvents = true
			if event.ToolCall != nil {
				if isValidToolCall(*event.ToolCall) {
					toolCalls = append(toolCalls, *event.ToolCall)
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
				}
			}

		case llm.StreamEventDone:
			// Stream finished - tool calls are already accumulated from stream deltas.
			// No need for an extra non-streaming API call.
			finalContent := contentBuilder.String()
			finalReasoning := reasoningBuilder.String()

			// If the LLM intended to call tools but all were invalid (e.g., empty arguments),
			// treat this as an error so the agent can retry rather than returning empty content.
			// Provide detailed feedback about which tool calls were invalid and why.
			if hasToolCallEvents && len(toolCalls) == 0 {
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

			return finalContent, finalReasoning, toolCalls, nil

		case llm.StreamEventError:
			return "", "", nil, event.Err
		}
	}

	// If we get here, the channel closed without a Done event
	// Fall back to non-streaming
	log.Debug("Stream channel closed without Done event, falling back to non-streaming")
	return a.nonStreamingFallback(ctx, tools, cb)
}

// nonStreamingFallback handles the case when streaming is not available.
func (a *Agent) nonStreamingFallback(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	// Apply context limit to messages
	contextMsgs := a.buildContextMessages()
	resp, err := a.llmClient.Chat(ctx, contextMsgs, tools)
	if err != nil {
		return "", "", nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if a.showThinking && resp.ReasoningContent != "" {
		cb("thinking", resp.ReasoningContent)
	}

	return resp.Content, resp.ReasoningContent, resp.ToolCalls, nil
}
