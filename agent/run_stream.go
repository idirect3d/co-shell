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

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// RunStream processes a user input through the agent loop with streaming output.
// It sends stream events to the provided callback function.
func (a *Agent) RunStream(ctx context.Context, userInput string, cb StreamCallback) (string, error) {
	// Ensure non-system messages are persisted on any exit path
	defer func() {
		if err := a.PersistSessionNonSystem(); err != nil {
			log.Warn("Failed to persist non-system session: %v", err)
		}
	}()

	// Reset interrupt channel for ESC key monitoring (FEATURE-201)
	a.ResetInterrupt()

	// Reset cancel channel for Ctrl+C monitoring (FEATURE-239)
	a.ResetCancel()

	// Reset approveAll, per-tool counters, completion flag, and error tracking for each new request
	a.approveAll = false
	a.approveCount = 0
	a.toolApproveCounts = make(map[string]int)
	a.toolDisableConfirm = make(map[string]bool)
	a.completed = false
	a.errorCounter = make(map[string]int)
	a.errorApproveAll = false

	// Reset task-level token tracking for this new request
	a.ResetTaskTokenUsage()

	// Reset task instruction cache for this new request (FEATURE-255)
	a.taskInstructionCache.Reset()

	// Initialize loop detectors and temperature controller for this request
	a.loopDetectCrit = false
	if a.cfg != nil && a.cfg.LLM.LoopIntervention != "off" {
		a.loopDetectOn = true
		threshold := a.cfg.LLM.LoopDetectThreshold
		if threshold <= 0 {
			threshold = 5
		}
		a.loopDetector = NewLoopDetector(threshold)

		// Attach SingleLineLoopDetector sub-detector for long-line and
		// character-level period detection (FEATURE-273).
		singleLineDetector := NewSingleLineLoopDetector(
			a.cfg.LLM.LoopSingleLineLength,
			a.cfg.LLM.LoopSingleLineWindow,
		)
		a.loopDetector.SetSingleLineDetector(singleLineDetector)
		// FEATURE-273: ToolCallLoopDetector uses threshold=2 (trigger on first duplicate)
		// instead of the content loop threshold, so a single repeated tool call is caught.
		toolCallThreshold := 2
		a.toolCallLoopDetector = NewToolCallLoopDetector(toolCallThreshold)

		// FEATURE-230: Initialize loop temperature controller
		if a.cfg.LLM.LoopTempEnabled {
			initialTemp := a.cfg.LLM.Temperature
			if a.modelManager != nil {
				if modelCfg := a.modelManager.GetActiveModel(len(a.imagePaths) > 0); modelCfg != nil && modelCfg.Temperature != nil {
					initialTemp = *modelCfg.Temperature
				}
			}
			a.loopTempCtrl = NewLoopTempController(
				initialTemp,
				a.cfg.LLM.LoopTempStepUp,
				a.cfg.LLM.LoopTempStepDown,
				a.cfg.LLM.LoopTempMax,
				a.cfg.LLM.LoopTempMin,
			)
			log.Debug("Agent.RunStream: loop temperature controller initialized (initial=%.2f, up=%.2f, down=%.2f, max=%.2f, min=%.2f)",
				initialTemp, a.cfg.LLM.LoopTempStepUp, a.cfg.LLM.LoopTempStepDown,
				a.cfg.LLM.LoopTempMax, a.cfg.LLM.LoopTempMin)
		} else {
			a.loopTempCtrl = nil
		}
	} else {
		a.loopDetectOn = false
		a.loopDetector = nil
		a.loopTempCtrl = nil
	}

	// Save raw user input for potential use in system prompt.
	a.lastUserInput = userInput

	// When userInput is empty (for .continue command), do NOT append a new user message.
	// The existing messages (including the last user message with environment_details)
	// are sent directly to the LLM for continuation.
	if userInput != "" {
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
			// Build user message with ContentParts for structured content.
			// All user messages use the array format: [{"type":"text","text":"instruction"}]
			// Environment_details is attached at creation time and frozen — never re-injected.
			userMsg := a.buildUserMessage(userInput)
			a.messages = append(a.messages, userMsg)
			// Sync to memory (content without timestamp prefix, Datetime field stores the time)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
					log.Warn("Failed to save user message to memory: %v", err)
				}
			}
		}
		a.mu.Unlock()

		// Inject environment_details for the last user message at creation time.
		lastIdx := len(a.messages) - 1
		if lastIdx >= 0 && a.messages[lastIdx].Role == "user" {
			msgCopy := a.messages[lastIdx]
			a.messages[lastIdx] = a.injectEnvelopeToLastUser([]llm.Message{msgCopy})[0]
		}
	} else {
		log.Info("Agent.RunStream: .continue mode — sending existing context without new user message")
	}

	log.Info("Agent.RunStream: user input: %s", userInput)

	// Rebuild system prompt to refresh {TASK} with current context
	a.rebuildSystemPrompt()

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Refresh the last user message's <environment_details> so retries and
		// loop-back iterations always show current time and opened resources.
		a.refreshLastUserEnvelope()

		// FIX-240: Reset content loop detector per-iteration.
		// Content loops are intra-iteration phenomena; counting across iterations
		// can cause false positives when the LLM reuses common phrases.
		if a.loopDetector != nil {
			a.loopDetector.Reset()
		}

		// Step 1: Debug mode - allow review/edit of user message before sending
		a.debugIntercept()

		// Step 2: Stream the LLM response
		var finalContent, finalReasoning string
		var toolCalls []llm.ToolCall
		var streamErr error

		finalContent, finalReasoning, toolCalls, streamErr = a.streamLLMResponse(ctx, tools, cb)

		// FEATURE-239: Handle user cancel (Ctrl+C) — immediate exit, no confirmation
		// FIX-264: No need to clean up a.messages — CanceledError is returned before the current
		// iteration's assistant message is added, so there is nothing to remove.
		if _, isCanceled := streamErr.(*CanceledError); isCanceled {
			ep := config.GetEmojiPrefixes(a.emojiEnabled)
			cb("info", fmt.Sprintf("\n%s 已取消本次操作。\n", ep.Error))
			return "", nil
		}

		// FEATURE-201: Handle user interrupt (ESC key)
		if _, isInterrupted := streamErr.(*InterruptedError); isInterrupted {
			// Reset interruptCh before the confirmation prompt so ESC works for the retry
			a.ResetInterrupt()
			// User pressed ESC during LLM output. Show confirmation prompt.
			ep := config.GetEmojiPrefixes(a.emojiEnabled)
			cb("info", fmt.Sprintf("\n%s 已暂停接收 LLM 返回的数据。\n", ep.Warning))
			cb("info", "是否确认取消？[C]取消本次响应 [Enter]继续接收: ")

			// Read user's choice via UserIO interface.
			// In enhanced mode, EnhancedIO sets IsReading=true so ESC monitor skips stdin.
			// In stdio mode, StdioIO.ReadLine works with bufio.Scanner.
			io := a.defaultIO()
			userChoice, _ := io.ReadLine()
			userChoice = strings.TrimSpace(userChoice)

			// Handle :debug on/off commands without cancel or retry
			if strings.HasPrefix(userChoice, ":debug ") {
				switch strings.TrimSpace(userChoice[7:]) {
				case "on":
					a.SetDebugMode(true)
					cb("info", "调试模式已开启\n")
				case "off":
					a.SetDebugMode(false)
					cb("info", "调试模式已关闭\n")
				}
				// Retry the LLM call with the same context after toggling debug
				a.ResetInterrupt()
				cb("info", fmt.Sprintf("%s 继续接收 LLM 返回数据...\n", ep.Success))
				finalContent, finalReasoning, toolCalls, streamErr = a.streamLLMResponse(ctx, tools, cb)
				if streamErr != nil {
					cb("info", fmt.Sprintf("\n%s 重新接收数据失败: %v\n", ep.Error, streamErr))
					cb("info", fmt.Sprintf("%s 已取消本次响应。\n", ep.Error))
					return "", nil
				}
				// Fall through to tool call handling below
				goto afterESC
			}
			if userChoice == "C" || userChoice == "c" {
				// User confirmed cancel: discard incomplete message and return to REPL
				// FIX-264: No need to clean up a.messages — InterruptedError is returned before the
				// current iteration's assistant message is added, so there is nothing to remove.
				cb("info", fmt.Sprintf("\n%s 已取消本次响应，丢弃不完整内容。\n", ep.Error))
				return "", nil
			}

			// User chose to continue: reset interrupt channel for next ESC detection,
			// then retry the LLM call with same context
			// FIX-264: No need to clean up a.messages — InterruptedError is returned before the
			// current iteration's assistant message is added, so there is nothing to remove.
			a.ResetInterrupt()
			cb("info", fmt.Sprintf("%s 继续接收 LLM 返回数据...\n", ep.Success))

			finalContent, finalReasoning, toolCalls, streamErr = a.streamLLMResponse(ctx, tools, cb)
			if streamErr != nil {
				// Retry failed too - treat it like user cancelled
				cb("info", fmt.Sprintf("\n%s 重新接收数据失败: %v\n", ep.Error, streamErr))
				cb("info", fmt.Sprintf("%s 已取消本次响应。\n", ep.Error))
				return "", nil
			}
		}

	afterESC:
		// Log the LLM response content and tool calls at DEBUG level for diagnostics.
		// This helps identify issues like the LLM including historical message prefixes
		// in its response content when returning tool calls.
		if streamErr == nil {
			log.Debug("Agent.RunStream: LLM response at iteration %d: content=%q, tool_calls=%d, reasoning_len=%d",
				iteration, finalContent, len(toolCalls), len(finalReasoning))
			for i, tc := range toolCalls {
				log.Debug("Agent.RunStream: LLM tool call #%d: name=%q, id=%q, args=%q",
					i, tc.Name, tc.ID, tc.Arguments)
			}
		}

		// NOTE: Loop judgment is now handled synchronously inside
		// handleLoopDetection() in loop.go. When judge is enabled, the stream
		// is paused during the judgment call. If the judge confirms a loop,
		// streamLLMResponse returns the LoopDetectedError which is handled
		// in the if streamErr != nil block below. If not confirmed, the
		// detector is reset and the stream continues normally.

		if streamErr != nil {
			// FIX-240: Handle loop detection error.
			// Unlike FIX-179, we do NOT remove previous assistant+tool messages here.
			// Loop detection occurs during streaming of the CURRENT iteration, before
			// the assistant message has been appended to a.messages. The problematic content
			// is already discarded by the LoopDetectedError in streamLLMResponse.
			// Removing previous iteration's messages would lose valuable context.
			// FIX-240 / FEATURE-241: Handle sync mode loop detection.
			// In sync mode (LoopJudgeEnabled=false), streamLLMResponse returns
			// LoopDetectedError immediately. Adjust temperature and retry.
			// In async mode (LoopJudgeEnabled=true), streamLLMResponse does NOT
			// return error for content loops; checkLoopJudgeResult handles it.
			if a.loopDetectCrit {
				// LOG: read actual LoopIntervention from a.cfg for diagnostics
				var diagLoopAction string
				if a.cfg != nil {
					diagLoopAction = a.cfg.LLM.LoopIntervention
				}
				log.Warn("Agent.RunStream: sync loop detected at iteration %d, cfg=%p, loop_intervention=%q, adjusting...", iteration, a.cfg, diagLoopAction)

				loopAction := ""
				if a.cfg != nil {
					loopAction = strings.TrimSpace(a.cfg.LLM.LoopIntervention)
				}
				// Fallback: try loading from config file directly
				if loopAction == "" && a.cfg != nil {
					if cfgPath := a.cfg.ConfigPath(); cfgPath != "" {
						if freshCfg, _, err := config.LoadFromFile(cfgPath, nil); err == nil {
							loopAction = strings.TrimSpace(freshCfg.LLM.LoopIntervention)
							a.cfg.LLM.LoopIntervention = freshCfg.LLM.LoopIntervention
						}
					}
				}
				if loopAction == "" {
					loopAction = "prompt" // fallback default
				}

				// Build feedback and actions based on loop intervention strategy
				loopFeedback := fmt.Sprintf(i18n.T(i18n.KeyLoopDetectFeedback), streamErr.Error())

				var strategyParts []string
				switch loopAction {
				case "retry":
					// Just resend context without any additional feedback
					loopFeedback = ""
					strategyParts = append(strategyParts, "重发上下文")

				case "prompt":
					// Append corrective prompt (same as default behavior)
					template := a.cfg.LLM.LoopPromptTemplate
					if template != "" {
						template = strings.ReplaceAll(template, "{ERROR}", streamErr.Error())
						loopFeedback = fmt.Sprintf(i18n.T(i18n.KeyLoopDetectFeedback), template)
					}
					strategyParts = append(strategyParts, "发送纠错提示")

				case "reorganize":
					// Append reorganize context suggestion
					suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
					if suggestion != "" {
						loopFeedback += "\n" + suggestion
					}
					strategyParts = append(strategyParts, "重整上下文")

				case "temperature":
					// Adjust temperature and resend
					if a.loopTempCtrl != nil {
						oldTemp := a.loopTempCtrl.Temperature()
						newTemp, changed := a.loopTempCtrl.Apply()
						if changed {
							a.llmClient.SetTemperature(newTemp)
							strategyParts = append(strategyParts, fmt.Sprintf("温度调整(%.2f→%.2f)", oldTemp, newTemp))
							log.Warn("Agent.RunStream: temperature adjusted from %.2f to %.2f after loop detection (direction=%d)",
								oldTemp, newTemp, a.loopTempCtrl.direction)
						}
					} else {
						strategyParts = append(strategyParts, "温度控制器未初始化")
					}
					strategyParts = append(strategyParts, "发送纠错提示")

				case "random":
					// Randomly pick one action
					actions := []string{"retry", "prompt", "reorganize", "temperature"}
					choice := actions[time.Now().UnixNano()%4]
					switch choice {
					case "retry":
						loopFeedback = ""
						strategyParts = append(strategyParts, "随机选择: 重发上下文")
					case "prompt":
						strategyParts = append(strategyParts, "随机选择: 发送纠错提示")
					case "reorganize":
						suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
						if suggestion != "" {
							loopFeedback += "\n" + suggestion
						}
						strategyParts = append(strategyParts, "随机选择: 重整上下文")
					case "temperature":
						if a.loopTempCtrl != nil {
							oldTemp := a.loopTempCtrl.Temperature()
							newTemp, changed := a.loopTempCtrl.Apply()
							if changed {
								a.llmClient.SetTemperature(newTemp)
								strategyParts = append(strategyParts, fmt.Sprintf("随机选择: 温度调整(%.2f→%.2f)", oldTemp, newTemp))
							}
						}
						strategyParts = append(strategyParts, "随机选择: 发送纠错提示")
					}

				default:
					// Unknown strategy: clear feedback to avoid sending prompt unexpectedly
					loopFeedback = ""
					strategyParts = append(strategyParts, fmt.Sprintf("未知策略(%s)，按retry处理", loopAction))
				}

				// Only append feedback message if there's content to send
				if loopFeedback != "" {
					a.mu.Lock()
					a.messages = append(a.messages, llm.Message{Role: "user", Content: loopFeedback})
					a.mu.Unlock()
				}

				if a.loopDetector != nil {
					a.loopDetector.Reset()
				}
				a.loopDetectCrit = false

				// Show summary at the end, after all handling
				cb("info", fmt.Sprintf("检测到循环输出（策略: %s）\n", loopAction))
				cb("info", fmt.Sprintf("处理方式: %s\n", strings.Join(strategyParts, " → ")))
				if loopFeedback != "" {
					cb("info", fmt.Sprintf("发送给 LLM 的提示:\n%s\n", loopFeedback))
				} else {
					cb("info", "（无反馈，仅重发上下文）\n")
				}
				cb("info", "────────────────────────────────────────────\n")
				continue
			}

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
				// Get emoji prefixes
				ep := config.GetEmojiPrefixes(a.emojiEnabled)

				// Prompt user for action via UserIO interface
				io := a.defaultIO()
				io.Printf("\n%s 错误反复出现: %s\n", ep.Warning, promptReason)
				io.Printf("  最新错误: %v\n", streamErr)
				io.Println()
				io.Println(i18n.T(i18n.KeyErrorRiskWarning))
				io.Println()
				io.Println("  请选择操作:")
				io.Println("  [Enter] 继续让 LLM 尝试处理")
				io.Println("  [C] 取消，返回 REPL")
				io.Println("  [A] 忽略限制，继续执行")
				io.Println()
				io.Printf("  请选择 (Enter/C/A): ")

				response, _ := io.ReadLine()
				userChoice := strings.TrimSpace(response)
				lower := strings.ToLower(userChoice)

				if lower == "c" {
					// User cancelled, return to REPL
					cb("info", fmt.Sprintf("\n%s 用户取消了操作\n", ep.Error))
					return "", nil
				} else if lower == "a" {
					// User chose to ignore all error limits
					a.errorApproveAll = true
					io.Printf("\n%s 已忽略错误限制，继续执行\n", ep.Success)
				} else {
					// Continue (Enter pressed)
					io.Printf("\n%s 继续让 LLM 尝试处理\n", ep.Success)
				}
			}

			// FIX-146: Determine how to handle the error based on the context.
			// The error occurs when sending messages to the LLM API. The problematic
			// message is already in a.messages from a previous iteration.
			//
			// We check if there is an assistant message with tool_calls in the recent
			// context (the last few messages). If so, the error is likely caused by
			// malformed tool call arguments in that assistant message. We remove that
			// assistant message and all subsequent messages (tool results, etc.) from
			// the context, and include the removed content in the error feedback.
			//
			// If there is no recent assistant message with tool_calls, the error is
			// likely caused by invalid user input, and we should exit the iteration
			// and report the error to the user.
			a.mu.Lock()
			removedContent := a.removeLastAssistantWithToolCalls()
			a.mu.Unlock()

			if removedContent != "" {
				// Found and removed a problematic assistant message with tool_calls.
				log.Warn("Agent.RunStream: stream error at iteration %d: %v, removed problematic assistant+tool messages (%d bytes)",
					iteration, streamErr, len(removedContent))

				// Build error feedback message that includes the removed content
				errorFeedback := fmt.Sprintf(
					"注意：刚才的 LLM 调用返回了错误，请根据错误信息判断如何处理。\n"+
						"如果错误是可恢复的（如参数格式问题、临时超时），请修正后重试。\n"+
						"如果错误是不可恢复的（如认证失败、模型不存在），请向用户报告错误并终止。\n\n"+
						"错误信息：%s\n\n"+
						"以下是你刚才返回的有问题的消息内容，已被从上下文中移除，请参考修正：\n%s",
					streamErr.Error(),
					removedContent,
				)

				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:    "user",
					Content: errorFeedback,
				})
				a.mu.Unlock()

				ep := config.GetEmojiPrefixes(a.emojiEnabled)
				cb("info", fmt.Sprintf("\n%s LLM 调用出错: %v\n已移除有问题的上下文，正在请求 LLM 修正后重试...\n", ep.Warning, streamErr))
				continue
			} else {
				// No recent assistant message with tool_calls found - the error is likely
				// caused by invalid user input. Exit the iteration and report to the user.
				log.Error("Agent.RunStream: stream error at iteration %d: %v, no assistant tool_calls found, exiting", iteration, streamErr)
				cb("error", fmt.Sprintf("LLM 调用出错: %v\n请检查您的输入是否有问题，或稍后重试。", streamErr))
				cb("done", "")
				return "", fmt.Errorf("LLM call failed: %w", streamErr)
			}
		}

		// Step 2: Handle XML parse errors (stored in taskInstructionCache by streamLLMResponse
		// or nonStreamingFallback). The malformed assistant message is NOT in a.messages yet,
		// so we simply apply loop-intervention strategy and continue. loop-intervention=off
		// falls back to retry (no feedback).
		//
		// The cache contains structured JSON lines: {"tool": "tool_name", "error": "..."}
		// Extract the tool name and use buildReferenceFormat to provide a preventive format tip.
		a.mu.Lock()
		xmlParseData := a.taskInstructionCache.String()
		a.mu.Unlock()
		if xmlParseData != "" {
			a.mu.Lock()
			a.taskInstructionCache.Reset()
			a.mu.Unlock()

			// Parse the first error entry to get the tool name
			lines := strings.SplitN(xmlParseData, "\n---\n", 2)
			firstLine := strings.TrimSpace(lines[0])
			toolName := ""
			if strings.HasPrefix(firstLine, "{") {
				var entry struct {
					Tool  string `json:"tool"`
					Error string `json:"error"`
				}
				if err := json.Unmarshal([]byte(firstLine), &entry); err == nil {
					toolName = entry.Tool
				}
			}

			// Get the format suggestion for the tool (XML mode)
			formatSuggestion := buildReferenceFormat(toolName)

			// Build preventive prompt using i18n template
			preventiveTemplate := i18n.T(i18n.KeyXMLParseErrorSuggestion)
			fullFeedback := strings.ReplaceAll(preventiveTemplate, "{TOOL_NAME}", toolName)
			fullFeedback = strings.ReplaceAll(fullFeedback, "{FORMAT}", formatSuggestion)

			loopAction := "retry"
			if a.cfg != nil && a.cfg.LLM.LoopIntervention != "" {
				loopAction = a.cfg.LLM.LoopIntervention
			}
			if loopAction == "off" {
				loopAction = "retry"
			}

			loopFeedback := ""
			var strategyParts []string
			switch loopAction {
			case "retry":
				loopFeedback = ""
				strategyParts = append(strategyParts, "重发上下文（无反馈）")
			case "prompt":
				loopFeedback = fullFeedback
				strategyParts = append(strategyParts, "发送纠错提示")
			case "temperature":
				if a.loopTempCtrl != nil {
					oldTemp := a.loopTempCtrl.Temperature()
					newTemp, changed := a.loopTempCtrl.Apply()
					if changed {
						a.llmClient.SetTemperature(newTemp)
						strategyParts = append(strategyParts, fmt.Sprintf("温度调整(%.2f→%.2f)", oldTemp, newTemp))
					}
				}
				strategyParts = append(strategyParts, "重发上下文")
			case "reorganize":
				suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
				if suggestion != "" {
					loopFeedback = suggestion
				}
				strategyParts = append(strategyParts, "重整上下文")
			case "random":
				actions := []string{"retry", "prompt", "reorganize", "temperature"}
				choice := actions[time.Now().UnixNano()%4]
				switch choice {
				case "retry":
					strategyParts = append(strategyParts, "随机选择: 重发上下文")
				case "prompt":
					loopFeedback = fullFeedback
					strategyParts = append(strategyParts, "随机选择: 发送纠错提示")
				case "reorganize":
					if suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion); suggestion != "" {
						loopFeedback = suggestion
					}
					strategyParts = append(strategyParts, "随机选择: 重整上下文")
				case "temperature":
					if a.loopTempCtrl != nil {
						oldTemp := a.loopTempCtrl.Temperature()
						newTemp, changed := a.loopTempCtrl.Apply()
						if changed {
							a.llmClient.SetTemperature(newTemp)
							strategyParts = append(strategyParts, fmt.Sprintf("温度调整(%.2f→%.2f)", oldTemp, newTemp))
						}
					}
					strategyParts = append(strategyParts, "随机选择: 重发上下文")
				}
			}

			if loopFeedback != "" {
				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("<task>\n%s\n</task>", loopFeedback),
				})
				a.mu.Unlock()
			}

			cb("info", fmt.Sprintf("检测到XML解析错误\n"))
			cb("info", fmt.Sprintf("处理方式: %s\n", strings.Join(strategyParts, " → ")))
			cb("info", "────────────────────────────────────────────\n")
			continue
		}

		// Step 3: Handle responses with no tool calls.
		// Exit conditions:
		//   1. attempt_completion IS available AND was called (completed=true) → exit
		//   2. attempt_completion IS available AND NOT called → prompt LLM to continue or call attempt_completion
		//   3. attempt_completion is NOT available → exit immediately (final content is the answer)
		if len(toolCalls) == 0 {
			// Check if attempt_completion tool is available.
			// Use buildToolsInternal() instead of the API-level tools list to handle
			// XML mode where buildTools() returns empty (FIX-219).
			attemptCompAvailable := a.toolCallEnabled
			if attemptCompAvailable {
				fullTools := a.buildToolsInternal()
				attemptCompAvailable = false
				for _, t := range fullTools {
					if t.Name == "attempt_completion" {
						attemptCompAvailable = true
						break
					}
				}
			}

			// Rule 3: attempt_completion not available → exit immediately
			if !attemptCompAvailable {
				cb("done", "")
				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:             "assistant",
					Content:          finalContent,
					ReasoningContent: finalReasoning,
				})
				if a.memoryEnabled {
					if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
						log.Warn("Failed to save assistant message to memory: %v", err)
					}
				}
				a.mu.Unlock()
				if err := a.PersistSession(); err != nil {
					log.Warn("Failed to persist session: %v", err)
				}
				log.Info("Agent.RunStream: exiting after %d iterations (0 tool calls, attempt_completion not available)", iteration+1)
				return finalContent, nil
			}

			// Rule 1: attempt_completion was called — exit
			// Send per-iteration token usage before completing (skip if "off" mode)
			iterPrompt, iterComp, iterTotal := a.IterTokenDelta()
			maxModelLen := a.GetMaxModelLen()
			timing := a.GetLLMTiming()
			if iterTotal > 0 {
				tokenUsageMode := "on"
				if a.cfg != nil {
					tokenUsageMode = a.cfg.LLM.TokenUsage
				}
				if tokenUsageMode != "off" {
					cb("token_iter", fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
						iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
				}
			}

			// Send task-level token usage before done
			taskP, taskC, taskT := a.TaskTokenUsage()
			if taskT > 0 {
				cb("token_task", fmt.Sprintf("prompt=%d completion=%d total=%d", taskP, taskC, taskT))
			}

			if a.completed {
				cb("done", "")

				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:             "assistant",
					Content:          finalContent,
					ReasoningContent: finalReasoning,
				})
				if a.memoryEnabled {
					if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
						log.Warn("Failed to save assistant message to memory: %v", err)
					}
				}
				a.mu.Unlock()
				if err := a.PersistSession(); err != nil {
					log.Warn("Failed to persist session: %v", err)
				}
				log.Info("Agent.RunStream: completed after %d iterations (via attempt_completion)", iteration+1)
				return finalContent, nil
			}

			// Rule 2: attempt_completion is available but was NOT called — prompt LLM to continue
			// FEATURE-273: Check if this response is a duplicate of the previous assistant response.
			// If so, produce a LoopEvent and call applyLoopIntervention() instead of appending
			// a generic continuePrompt.
			var event *LoopEvent
			a.mu.Lock()
			// When loop intervention is "off", skip content duplicate detection entirely.
			loopIntervention := ""
			if a.cfg != nil {
				loopIntervention = a.cfg.LLM.LoopIntervention
			}
			skipLoopDetect := loopIntervention == "off"
			if a.lastAssistantContent != "" && !skipLoopDetect {
				threshold := 0.95
				if a.cfg != nil && a.cfg.LLM.DuplicateContentThreshold > 0 {
					threshold = a.cfg.LLM.DuplicateContentThreshold
				}
				dup, similarity := IsDuplicateContent(a.lastAssistantContent, finalContent, threshold)
				if dup {
					log.Warn("Agent.RunStream: zero-tool-call content %.1f%% similar to previous, triggering loop intervention", similarity*100)
					event = &LoopEvent{
						Type:     LoopEventContentDuplicate,
						Detector: "cross-iteration content duplicate",
						Content:  finalContent,
						Reason:   fmt.Sprintf("content %.0f%% similar to previous iteration", similarity*100),
						Suggestion: "Your response is very similar to your previous one. " +
							"Please take a different approach. If your task is complete, " +
							"call attempt_completion. Otherwise, try a different tool or strategy.",
					}
				}
			}
			a.lastAssistantContent = finalContent
			if event != nil {
				// Duplicate detected: do NOT append the assistant message.
				// applyLoopIntervention will send feedback, then continue.
				a.mu.Unlock()
				log.Debug("Agent.RunStream: content duplicate detected, applying loop intervention (assistant discarded)")
				a.applyLoopIntervention(event)
				continue
			}
			// No duplicate: append the assistant message first, then the continue prompt
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          finalContent,
				ReasoningContent: finalReasoning,
			})
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			continuePrompt := i18n.T(i18n.KeyContinuePrompt)
			a.messages = append(a.messages, llm.Message{
				Role:    "user",
				Content: continuePrompt,
			})
			a.mu.Unlock()

			log.Debug("Agent.RunStream: 0 tool calls but attempt_completion not called, prompting LLM to continue")
			continue
		}

		// Determine if we're in XML mode (no API-level tool calls)
		isXMLMode := false
		if a.toolCallModeMgr != nil {
			mode := a.toolCallModeMgr.Current()
			if mode != nil && !mode.SendTools {
				isXMLMode = true
			}
		}

		// FEATURE-273: Check for tool call loop using unified LoopEvent + applyLoopIntervention.
		// The ToolCallLoopDetector now triggers on the FIRST duplicate (count >= 2).
		if a.loopDetectOn && a.toolCallLoopDetector != nil && len(toolCalls) > 0 {
			loopDetected := false
			var firstToolName, firstToolArgs string
			for _, tc := range toolCalls {
				if err := a.toolCallLoopDetector.AddCall(tc.Name, tc.Arguments); err != nil {
					log.Warn("Agent.RunStream: tool call loop detected: %v", err)
					loopDetected = true
					firstToolName = tc.Name
					firstToolArgs = tc.Arguments
					break
				}
			}

			if loopDetected {
				a.toolCallLoopDetector.Reset()
				event := &LoopEvent{
					Type:     LoopEventToolCallRepeat,
					Detector: "tool call loop detector",
					ToolName: firstToolName,
					ToolArgs: firstToolArgs,
					Reason:   fmt.Sprintf("tool %q called with the same arguments twice consecutively", firstToolName),
				}
				a.applyLoopIntervention(event)
				continue
			}
		}

		// FEATURE-264: Check context usage threshold BEFORE executing tool calls.
		// If the context is over the threshold, set reorganizePending to skip
		// adding the assistant message and executing tools for this iteration.
		// The reorganize instruction will be appended after the iteration's normal
		// end-of-cycle processing (token_iter + flush + env injection).
		var reorganizePending bool
		maxModelLen := a.GetMaxModelLen()
		if a.cfg != nil && a.cfg.LLM.ContextPolicy == "reorganize" && a.cfg.LLM.ContextReorganizeThreshold > 0 && maxModelLen > 0 {
			_, _, iterTotal := a.IterTokenDelta()
			usagePct := float64(iterTotal) * 100.0 / float64(maxModelLen)
			threshold := float64(a.cfg.LLM.ContextReorganizeThreshold)

			if usagePct >= threshold {
				log.Info("Agent.RunStream: context usage %.1f%% exceeds threshold %.0f%%, skipping tool calls",
					usagePct, threshold)
				reorganizePending = true
				ep := config.GetEmojiPrefixes(a.emojiEnabled)
				cb("warning", fmt.Sprintf("\n%s 上下文超限 (%.1f%% > %.0f%%)，已跳过此轮工具执行\n", ep.Warning, usagePct, threshold))
			}
		}

		// Move cancelled declaration outside the if block so both the assignment
		// inside and the check below are in scope.
		var cancelled bool

		// If the LLM is already calling reorganize_context, do NOT skip it.
		hasReorganizeCall := false
		if reorganizePending && len(toolCalls) > 0 {
			for _, tc := range toolCalls {
				if tc.Name == "reorganize_context" {
					hasReorganizeCall = true
					break
				}
			}
		}

		if !reorganizePending || hasReorganizeCall {
			// First add assistant message with tool_calls to history
			// This must come BEFORE tool result messages to satisfy the API requirement
			// that tool messages must follow a message with tool_calls.
			// In XML mode, do NOT set ToolCalls on the assistant message — tool calls
			// are embedded in the content as XML tags and the LLM expects results
			// returned as user messages (not tool messages).
			a.mu.Lock()
			assistantMsgIdx := len(a.messages)
			assistantMsg := llm.Message{
				Role:             "assistant",
				Content:          finalContent,
				ReasoningContent: finalReasoning,
			}
			if !isXMLMode {
				assistantMsg.ToolCalls = toolCalls
			}
			log.Debug("Agent.RunStream: preparing to add assistant message to a.messages at index %d: role=%s, content_len=%d, reasoning_len=%d, tool_calls=%d",
				assistantMsgIdx, assistantMsg.Role, len(assistantMsg.Content), len(assistantMsg.ReasoningContent), len(assistantMsg.ToolCalls))
			for i, tc := range toolCalls {
				log.Debug("  tool_call[%d]: name=%s, id=%s, args_len=%d", i, tc.Name, tc.ID, len(tc.Arguments))
			}
			a.messages = append(a.messages, assistantMsg)
			// Sync to memory (content without timestamp prefix)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			a.mu.Unlock()

			// Step 4: Execute tool calls and add results
			for _, tc := range toolCalls {
				// Show command if enabled
				if a.showCommand && tc.Name == "execute_command" {
					var cmdArgs map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Arguments), &cmdArgs); err == nil {
						if cmd, ok := cmdArgs["command"].(string); ok {
							cb("command", cmd)
						}
					}
				}

				// Show tool call name (and input arguments if enabled)
				if a.showTool {
					msg := tc.Name
					if a.showToolInput {
						// Pretty-print the JSON arguments
						var argsPretty string
						var argsMap map[string]interface{}
						if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err == nil {
							if pretty, err := json.MarshalIndent(argsMap, "", "  "); err == nil {
								argsPretty = string(pretty)
							}
						}
						if argsPretty == "" {
							argsPretty = tc.Arguments
						}
						msg += "\n" + argsPretty
					}
					msg += "\n"
					cb("tool_call", msg)
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
					// Format structured error feedback to help LLM understand and fix the issue
					result = formatToolError(tc.Name, execErr)
					log.Error("Agent.RunStream: tool %s failed: %v", tc.Name, execErr)
				}

				// Show tool call output if enabled (for all tools)
				if a.showToolOutput && result != "" {
					cb("tool_call", fmt.Sprintf("  Result:\n%s\n", result))
				}

				// If the result is empty, provide a clear message to the LLM

				toolContent := result
				if toolContent == "" {
					toolContent = "（工具调用无输出）"
				}

				if isXMLMode {
					// In XML mode, return tool results as user messages with ContentParts structure.
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

				// FEATURE-255 / FIX-257: Flush task instruction cache BEFORE injecting
				// <environment_details>, so <task> appears between tool result and <env>.
				// This collects reorganize_context summary, CmdConfirmModify supplemental
				// instructions, and other task-level hints, appending them as a <task>
				// ContentPart to the just-added tool result message.
				if a.taskInstructionCache.Len() > 0 {
					taskContent := fmt.Sprintf("<task>\n%s\n</task>", a.taskInstructionCache.String())
					log.Debug("Agent.RunStream: flushing task instruction cache: %s", taskContent)

					a.mu.Lock()
					lastIdx := len(a.messages) - 1
					if lastIdx >= 0 && a.messages[lastIdx].Role == "user" {
						msg := &a.messages[lastIdx]
						if len(msg.ContentParts) == 0 {
							msg.ContentParts = []llm.ContentPart{
								{Type: llm.ContentPartText, Text: msg.Content},
							}
							msg.Content = ""
						}
						msg.AppendTextPart(taskContent)
					} else {
						a.messages = append(a.messages, llm.Message{Role: "user", Content: taskContent})
					}
					a.mu.Unlock()
					a.taskInstructionCache.Reset()
				}

				// Attach environment_details to the just-added tool result message.
				// This must come AFTER the task instruction cache flush so that
				// <environment_details> is the last ContentPart.
				// IMPORTANT: Must NOT hold a.mu here because injectTimeAndMessageNoToLast
				// calls buildFullEnvironmentDetails -> isLastToolTaskPlan which acquires a.mu.
				a.injectTimeAndMessageNoToLast()
			}
		} // end if !reorganizePending

		// If attempt_completion was called during tool execution, finalize and exit
		if a.completed {
			// Send per-iteration token usage before done (skip if "off" mode)
			iterPrompt, iterComp, iterTotal := a.IterTokenDelta()
			maxModelLen := a.GetMaxModelLen()
			timing := a.GetLLMTiming()
			if iterTotal > 0 {
				tokenUsageMode := "on"
				if a.cfg != nil {
					tokenUsageMode = a.cfg.LLM.TokenUsage
				}
				if tokenUsageMode != "off" {
					cb("token_iter", fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
						iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
				}
			}
			// Send task-level token usage before done
			taskP, taskC, taskT := a.TaskTokenUsage()
			if taskT > 0 {
				cb("token_task", fmt.Sprintf("prompt=%d completion=%d total=%d", taskP, taskC, taskT))
			}
			cb("done", "")
			log.Info("Agent.RunStream: completed after %d iterations (via attempt_completion in same iteration)", iteration+1)
			return finalContent, nil
		}

		// If user cancelled, return to REPL
		if cancelled {
			return "", nil
		}

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		// Only "task" mode auto-adjusts the pointer — "window" and "smart" modes do not.
		a.mu.Lock()
		if a.needAdjustPointer {
			contextStartMode := "smart"
			if a.cfg != nil && a.cfg.LLM.ContextPolicy != "" {
				contextStartMode = a.cfg.LLM.ContextPolicy
			}
			if contextStartMode == "task" {
				a.messagePointer = len(a.messages) - 1
				a.adjustMessagePointer()
			}
			a.needAdjustPointer = false
		}
		a.mu.Unlock()

		// Send per-iteration token usage at the end of each iteration (skip if "off" mode)
		iterPrompt, iterComp, iterTotal := a.IterTokenDelta()
		timing := a.GetLLMTiming()
		if iterTotal > 0 {
			tokenUsageMode := "on"
			if a.cfg != nil {
				tokenUsageMode = a.cfg.LLM.TokenUsage
			}
			if tokenUsageMode != "off" {
				cb("token_iter", fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
					iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
			}
		}

		// FEATURE-255: Flush task instruction cache at the end of each iteration.
		// This collects user supplementary inputs from CmdConfirmModify and other
		// task-level hints (e.g., context overflow warnings) and appends them as
		// a single <task> ContentPart to the last user message. This separates
		// user instructions from tool results, keeping the structure clean.
		if a.taskInstructionCache.Len() > 0 {
			taskContent := fmt.Sprintf("<task>\n%s\n</task>", a.taskInstructionCache.String())
			log.Debug("Agent.RunStream: flushing task instruction cache: %s", taskContent)

			a.mu.Lock()
			lastIdx := len(a.messages) - 1
			if lastIdx >= 0 && a.messages[lastIdx].Role == "user" {
				msg := &a.messages[lastIdx]
				if len(msg.ContentParts) == 0 {
					msg.ContentParts = []llm.ContentPart{
						{Type: llm.ContentPartText, Text: msg.Content},
					}
					msg.Content = ""
				}
				msg.AppendTextPart(taskContent)
			} else {
				a.messages = append(a.messages, llm.Message{Role: "user", Content: taskContent})
			}
			a.mu.Unlock()
			a.taskInstructionCache.Reset()
		}

		// FEATURE-264: If reorganize was pending (and the LLM did NOT already call
		// reorganize_context this iteration), append a clean user message containing
		// only the reorganize instruction. This is done AFTER the normal end-of-cycle
		// processing (token_iter, flush, env injection), so the LLM sees a fresh,
		// standalone instruction on the next iteration.
		if reorganizePending && !hasReorganizeCall {
			reorgMsg := "<task>\n你必须马上进行上下文整理。\n</task>\n\n当前上下文已经超限，立即调用 reorganize_context 工具。"
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:    "user",
				Content: reorgMsg,
			})
			a.mu.Unlock()
			// Inject environment_details for the new reorganize message
			lastIdx := len(a.messages) - 1
			if lastIdx >= 0 && a.messages[lastIdx].Role == "user" {
				msgCopy := a.messages[lastIdx]
				a.messages[lastIdx] = a.injectEnvelopeToLastUser([]llm.Message{msgCopy})[0]
			}
		}
	}

	log.Error("Agent.RunStream: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}
