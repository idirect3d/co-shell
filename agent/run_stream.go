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

// emitParseErrorRaw emits the raw offending content when the
// show-parse-error-raw switch is enabled (FEATURE-336).
func (a *Agent) emitParseErrorRaw(cb StreamCallback, rawDetail string) {
	if a.cfg != nil && a.cfg.LLM.ShowParseErrorRaw && rawDetail != "" {
		cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyXMLParseErrorRaw), rawDetail))
	}
}

// abortVisionRecognitionRound finalizes an in-flight FEATURE-343 minimal
// recognition round that was aborted (ESC cancel or a failed retry): since the
// placeholder tool result was deferred, the pending vision tool call still
// needs a tool result — otherwise the assistant tool_calls message would be
// left without a response and strict providers would reject the next request.
// Appends a cancelled marker and clears the recognition-round state. No-op in
// full vision-context mode (visionRecognitionActive is never set there).
func (a *Agent) abortVisionRecognitionRound() {
	a.mu.Lock()
	if !a.visionRecognitionActive {
		a.mu.Unlock()
		return
	}
	a.visionRecognitionActive = false
	a.visionPendingIntent = ""
	toolID := a.lastVisionToolCallID
	toolName := a.lastVisionToolCallName
	a.mu.Unlock()

	content := i18n.T(i18n.KeyVisionRecognitionCancelled)
	if a.isXMLMode() {
		msg := a.buildXMLToolResultMessage(toolName, "", content, len(a.messages))
		a.mu.Lock()
		a.messages = append(a.messages, msg)
		a.mu.Unlock()
	} else {
		a.mu.Lock()
		a.messages = append(a.messages, llm.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: toolID,
		})
		a.mu.Unlock()
	}
	a.injectTimeAndMessageNoToLast()
	log.Info("Agent.RunStream: FEATURE-343 recognition round aborted, backfilled cancelled marker for %s (ID: %s)", toolName, toolID)
}

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
		blockLimit := a.cfg.LLM.LoopSingleLineBlockLimit
		if blockLimit <= 0 {
			// FIX-329: legacy behavior — 0 disables the check. The default
			// value is applied by DefaultConfig (200) unless explicitly set 0.
			blockLimit = 0
		}
		a.loopDetector = NewLoopDetectorWithBlockLimit(threshold, blockLimit)

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
		// FEATURE-349: a new genuine user instruction starts a fresh loop
		// chain — forget strategies that failed for the previous task.
		a.loopFailedStrategies = nil
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

iterationLoop:
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

		// FIX-285: Reset loopJudgeExitStrategy at the start of each iteration.
		// This ensures stale exit_strategy from a previous loop detection is
		// not carried over if no loop is detected in the current iteration.
		a.loopJudgeExitStrategy = ""

		// Step 1: Debug mode - allow review/edit of user message before sending.
		// Skip for .continue mode (first iteration with empty userInput) to avoid
		// blocking on ReadLine when the user explicitly wants to send existing context.
		if userInput != "" || iteration > 0 {
			a.debugIntercept()
		}

		// Step 2: Stream the LLM response
		var finalContent, finalReasoning string
		var toolCalls []llm.ToolCall
		var streamErr error
		var hasToolAttempt bool

		finalContent, finalReasoning, toolCalls, hasToolAttempt, streamErr = a.streamLLMResponse(ctx, tools, cb)

		// FEATURE-239: Handle user cancel (Ctrl+C) — immediate exit, no confirmation
		// FIX-264: No need to clean up a.messages — CanceledError is returned before the current
		// iteration's assistant message is added, so there is nothing to remove.
		if _, isCanceled := streamErr.(*CanceledError); isCanceled {
			ep := config.GetEmojiPrefixes(a.emojiEnabled)
			cb(EventInfo, fmt.Sprintf("\n%s %s\n", ep.Error, i18n.T(i18n.KeyOutputCancelled)))
			return "", nil
		}

		// FEATURE-201: Handle user interrupt (ESC key)
		if _, isInterrupted := streamErr.(*InterruptedError); isInterrupted {
			// Reset interruptCh before the confirmation prompt so ESC works for the retry
			a.ResetInterrupt()
			// User pressed ESC during LLM output. Show confirmation prompt.
			ep := config.GetEmojiPrefixes(a.emojiEnabled)
			cb(EventInfo, fmt.Sprintf("\n%s %s\n", ep.Warning, i18n.T(i18n.KeyOutputPaused)))
			cb(EventInfo, i18n.T(i18n.KeyOutputCancelPrompt))

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
					cb(EventInfo, i18n.T(i18n.KeyDebugModeOn))
				case "off":
					a.SetDebugMode(false)
					cb(EventInfo, i18n.T(i18n.KeyDebugModeOff))
				}
				// Retry the LLM call with the same context after toggling debug
				a.ResetInterrupt()
				cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyOutputResume), ep.Success))
				finalContent, finalReasoning, toolCalls, _, streamErr = a.streamLLMResponse(ctx, tools, cb)
				// HACK: the `_` here is hasToolAttempt; we don't use it on retry paths
				// because the content is discarded and the stream call will be re-issued.
				if streamErr != nil {
					cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyOutputRetryFailed), ep.Error, streamErr))
					cb(EventInfo, fmt.Sprintf("%s %s\n", ep.Error, i18n.T(i18n.KeyOutputCancelled)))
					a.abortVisionRecognitionRound()
					return "", nil
				}
				// Fall through to tool call handling below
				goto afterESC
			}
			if userChoice == "C" || userChoice == "c" {
				// User confirmed cancel: discard incomplete message and return to REPL
				// FIX-264: No need to clean up a.messages — InterruptedError is returned before the
				// current iteration's assistant message is added, so there is nothing to remove.
				cb(EventInfo, fmt.Sprintf("\n%s %s\n", ep.Error, i18n.T(i18n.KeyOutputCancelledDiscard)))
				a.abortVisionRecognitionRound()
				return "", nil
			}

			// User chose to continue: reset interrupt channel for next ESC detection,
			// then retry the LLM call with same context
			// FIX-264: No need to clean up a.messages — InterruptedError is returned before the
			// current iteration's assistant message is added, so there is nothing to remove.
			a.ResetInterrupt()
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyOutputResume), ep.Success))

			finalContent, finalReasoning, toolCalls, _, streamErr = a.streamLLMResponse(ctx, tools, cb)
			if streamErr != nil {
				// Retry failed too - treat it like user cancelled
				cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyOutputRetryFailed), ep.Error, streamErr))
				cb(EventInfo, fmt.Sprintf("%s %s\n", ep.Error, i18n.T(i18n.KeyOutputCancelled)))
				a.abortVisionRecognitionRound()
				return "", nil
			}
		}

	afterESC:
		// FEATURE-343 + FEATURE-346: vision recognition round — the current LLM
		// call was the dedicated OCR/vision pass (system=Identity-only,
		// tools=[]). Backfill the visual model's output as the vision tool's
		// (visual_analysis / browser_screenshot) result and do NOT append an
		// assistant message to the main conversation history. The recognition
		// round's input (intent + images) is NOT persisted; only the tool
		// result is, so the main model sees
		//   assistant(tool_calls: <vision tool>) → tool(user)/user(识别结果)
		// on the next iteration.
		a.mu.Lock()
		isRecognitionRound := a.visionRecognitionActive
		var toolID, toolName string
		if isRecognitionRound {
			a.visionRecognitionActive = false
			a.visionRecognitionExecuted = true
			toolID = a.lastVisionToolCallID
			toolName = a.lastVisionToolCallName
			// FEATURE-343: clear the pending intent after the recognition round
			// so a later text-only turn does NOT accidentally trigger another
			// minimal collapse.
			a.visionPendingIntent = ""
		}
		a.mu.Unlock()
		if isRecognitionRound {
			// Build the recognition result (or a failure marker).
			recognitionContent := finalContent
			if streamErr != nil {
				recognitionContent = fmt.Sprintf(i18n.TF(i18n.KeyVisionRecognitionFailed), streamErr)
			} else if strings.TrimSpace(recognitionContent) == "" {
				recognitionContent = i18n.T(i18n.KeyVisionRecognitionEmpty)
			}
			if toolName == "" {
				toolName = "visual_analysis"
			}

			isXML := false
			if a.toolCallModeMgr != nil {
				mode := a.toolCallModeMgr.Current()
				if mode != nil && !mode.SendTools {
					isXML = true
				}
			}

			if isXML {
				// XML mode: recognition result as a user tool-result message.
				toolResultMsg := a.buildXMLToolResultMessage(toolName, "", recognitionContent, len(a.messages))
				a.mu.Lock()
				a.messages = append(a.messages, toolResultMsg)
				a.mu.Unlock()
			} else {
				// OpenAI mode: backfill as the vision tool's tool message.
				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:       "tool",
					Content:    recognitionContent,
					ToolCallID: toolID,
				})
				a.mu.Unlock()
			}
			// Attach <environment_details> to the backfilled result.
			a.injectTimeAndMessageNoToLast()

			log.Info("Agent.RunStream: FEATURE-343 recognition round completed, backfilled %s result (%d bytes)", toolName, len(recognitionContent))
			// Continue to the next iteration so the main model can see the
			// recognition result and act on it; the loop will exit naturally
			// when the main model reaches a final answer.
			continue
		}

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
			// However, before retrying we strip the stale assistant+continuePrompt pair
			// from the PREVIOUS iteration so the LLM sees a clean slate.
			// FIX-240 / FEATURE-241: loop judgment is synchronous inside
			// handleLoopDetection(); on a confirmed loop streamLLMResponse
			// returns LoopDetectedError and we end up here.
			if a.loopDetectCrit {
				// Strip stale assistant+continuePrompt pair from previous iteration.
				a.stripLastAssistantAndContinue()
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
					strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyResend))

				case "prompt":
					// FIX-285: Use judge model's exit_strategy if available,
					// otherwise fall back to the configured LoopPromptTemplate
					// or the generic loop detection feedback.
					if a.loopJudgeExitStrategy != "" {
						loopFeedback = a.loopJudgeExitStrategy
					} else {
						template := a.cfg.LLM.LoopPromptTemplate
						if template != "" {
							template = strings.ReplaceAll(template, "{ERROR}", streamErr.Error())
							loopFeedback = fmt.Sprintf(i18n.T(i18n.KeyLoopDetectFeedback), template)
						}
					}
					strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyPrompt))

				case "reorganize":
					// Append reorganize context suggestion
					suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
					if suggestion != "" {
						loopFeedback += "\n" + suggestion
					}
					strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyReorganize))

				case "temperature":
					// Adjust temperature and resend
					if a.loopTempCtrl != nil {
						oldTemp := a.loopTempCtrl.Temperature()
						newTemp, changed := a.loopTempCtrl.Apply()
						if changed {
							a.llmClient.SetTemperature(newTemp)
							strategyParts = append(strategyParts, fmt.Sprintf(i18n.TF(i18n.KeyStrategyTempAdjust), oldTemp, newTemp))
							log.Warn("Agent.RunStream: temperature adjusted from %.2f to %.2f after loop detection (direction=%d)",
								oldTemp, newTemp, a.loopTempCtrl.direction)
						}
					} else {
						strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyTempNoInit))
					}
					strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyPrompt))

				case "random":
					// Randomly pick one action
					actions := []string{"retry", "prompt", "reorganize", "temperature"}
					choice := actions[time.Now().UnixNano()%4]
					switch choice {
					case "retry":
						loopFeedback = ""
						strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyRandomResend))
					case "prompt":
						strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyRandomPrompt))
					case "reorganize":
						suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
						if suggestion != "" {
							loopFeedback += "\n" + suggestion
						}
						strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyRandomReorg))
					case "temperature":
						if a.loopTempCtrl != nil {
							oldTemp := a.loopTempCtrl.Temperature()
							newTemp, changed := a.loopTempCtrl.Apply()
							if changed {
								a.llmClient.SetTemperature(newTemp)
								strategyParts = append(strategyParts, fmt.Sprintf(i18n.TF(i18n.KeyStrategyRandomTemp), oldTemp, newTemp))
							}
						}
						strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyRandomPrompt))
					}

				default:
					// Unknown strategy: clear feedback to avoid sending prompt unexpectedly
					loopFeedback = ""
					strategyParts = append(strategyParts, fmt.Sprintf(i18n.TF(i18n.KeyStrategyUnknown), loopAction))
				}

				// FIX-321: Apply loop feedback via the unified helper. When
				// loopFeedback is non-empty (prompt/reorganize) a loop feedback
				// message with a full <environment_details> block is created or
				// updated in place; when empty (retry/temperature) only the
				// <retried_count> tag on the last user message is incremented.
				a.applyLoopFeedback(loopFeedback)

				// FEATURE-327: Check the retried_count limit. When the count
				// reaches error-max-single-count, the user is prompted to
				// decide (Enter/C/A). If the user cancels, terminate the task.
				if ok, err := a.checkRetryCountLimit(); err != nil {
					cb(EventInfo, fmt.Sprintf("\n%s %s\n", config.GetEmojiPrefixes(a.emojiEnabled).Error, i18n.T(i18n.KeyUserCancelled)))
					return "", nil
				} else if !ok {
					return "", nil
				}

				if a.loopDetector != nil {
					a.loopDetector.Reset()
				}
				a.loopDetectCrit = false

				// Show summary at the end, after all handling
				ep := config.GetEmojiPrefixes(a.emojiEnabled)
				cb(EventInfo, ep.Loop+fmt.Sprintf(i18n.TF(i18n.KeyLoopDetectedSummary), loopAction))
				cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopHandling), strings.Join(strategyParts, " → ")))
				if loopFeedback != "" {
					cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopFeedbackSent), loopFeedback))
				} else {
					cb(EventInfo, i18n.T(i18n.KeyLoopNoFeedback))
				}
				cb(EventInfo, "────────────────────────────────────────────\n")
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
				promptReason = fmt.Sprintf(i18n.TF(i18n.KeyErrRepeatPrompt), singleCount, maxSingle)
			} else if typeCount >= maxType && !a.errorApproveAll {
				needUserPrompt = true
				promptReason = fmt.Sprintf(i18n.TF(i18n.KeyErrTypePrompt), typeCount, maxType)
			}

			if needUserPrompt {
				// Get emoji prefixes
				ep := config.GetEmojiPrefixes(a.emojiEnabled)

				// Prompt user for action via UserIO interface
				io := a.defaultIO()
				io.Printf("\n%s %s: %s\n", ep.Warning, i18n.T(i18n.KeyErrRepeatWarn), promptReason)
				io.Printf("  %s\n", fmt.Sprintf(i18n.TF(i18n.KeyErrLatest), streamErr))
				io.Println()
				io.Println(i18n.T(i18n.KeyErrorRiskWarning))
				io.Println()
				io.Println(i18n.T(i18n.KeyErrActionTitle))
				io.Println(i18n.T(i18n.KeyErrActionEnter))
				io.Println(i18n.T(i18n.KeyErrActionCancel))
				io.Println(i18n.T(i18n.KeyErrActionIgnore))
				io.Println()
				io.Print(i18n.T(i18n.KeyErrActionChoose))

				response, _ := io.ReadLine()
				userChoice := strings.TrimSpace(response)
				lower := strings.ToLower(userChoice)

				if lower == "c" {
					// User cancelled, return to REPL
					cb(EventInfo, fmt.Sprintf("\n%s %s\n", ep.Error, i18n.T(i18n.KeyUserCancelled)))
					return "", nil
				} else if lower == "a" {
					// User chose to ignore all error limits
					a.errorApproveAll = true
					io.Printf("\n%s %s\n", ep.Success, i18n.T(i18n.KeyErrIgnoredContinue))
				} else {
					// Continue (Enter pressed)
					io.Printf("\n%s %s\n", ep.Success, i18n.T(i18n.KeyErrRetryContinue))
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

				// FEATURE-287: Apply parse-error-action strategy
				parseAction := "retry"
				if a.cfg != nil && a.cfg.LLM.ParseErrorAction != "" {
					parseAction = a.cfg.LLM.ParseErrorAction
				}

				// exit: exit the loop and report error
				if parseAction == "exit" {
					cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyLLMErrorExit), streamErr))
					cb(EventDone, "")
					return "", fmt.Errorf("LLM call failed: %w", streamErr)
				}

				// FEATURE-345: consult the problem model for ambiguous
				// connection errors before applying the default strategy.
				// Hard-coded sufficient conditions (HTTP 401/403/404/429/5xx)
				// are skipped — only ambiguous errors reach the model.
				if a.cfg != nil && a.cfg.LLM.ProblemSolverEnabled {
					if _, ok := classifyConnectionError(streamErr); !ok {
						if report, perr := a.solveProblem(context.Background(), ProblemTypeLLMConnectionError, streamErr.Error()); perr == nil && report != nil {
							cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverClassified), report.Type, report.Reason, report.SuggestedAction))
							feedback, _, stop := applyProblemAction(report)
							if stop {
								cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverNotifyUser), report.Reason, report.Guidance))
								cb(EventDone, "")
								return "", fmt.Errorf("problem model recommended stopping: %s", report.Reason)
							}
							if feedback != "" {
								a.mu.Lock()
								a.messages = append(a.messages, llm.Message{Role: "user", Content: feedback})
								a.mu.Unlock()
								cb(EventInfo, fmt.Sprintf("\n%s\n", feedback))
								continue
							}
						} else if perr != nil {
							log.Debug("RunStream: problem solver failed for connection error, falling back to built-in handling: %v", perr)
							cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverFailed), perr))
						}
					}
				}

				if parseAction == "retry" {
					// No feedback, just resend context
					ep := config.GetEmojiPrefixes(a.emojiEnabled)
					cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLLMErrorRetry), ep.Warning, streamErr))
					continue
				}

				// prompt (default): build error feedback and append
				errorFeedback := fmt.Sprintf(
					i18n.T(i18n.KeyLLMErrorFeedbackHead)+
						i18n.T(i18n.KeyLLMErrorFeedbackR1)+
						i18n.T(i18n.KeyLLMErrorFeedbackR2)+
						i18n.T(i18n.KeyLLMErrorFeedbackErr)+
						i18n.T(i18n.KeyLLMErrorFeedbackRef),
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
				cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLLMErrorFixRetry), ep.Warning, streamErr))
				continue
			} else {
				// No recent assistant message with tool_calls found - the error is likely
				// caused by invalid user input. Exit the iteration and report to the user.
				log.Error("Agent.RunStream: stream error at iteration %d: %v, no assistant tool_calls found, exiting", iteration, streamErr)
				cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyLLMErrorCheckInput), streamErr))
				cb(EventDone, "")
				return "", fmt.Errorf("LLM call failed: %w", streamErr)
			}
		}

		// Step 2: Handle XML parse errors and invalid tool call errors
		// (stored in taskInstructionCache by streamLLMResponse or nonStreamingFallback).
		// The malformed assistant message is NOT in a.messages yet, so we simply apply
		// parse-error-action strategy and continue.
		//
		// The cache contains structured JSON lines: {"tool": "tool_name", "error": "..."}
		// Extract the tool name and use buildReferenceFormat to provide a preventive format tip.
		// FEATURE-287: Replaced LoopIntervention with ParseErrorAction, simplified to
		// only support exit/retry/prompt.
		a.mu.Lock()
		xmlParseData := a.taskInstructionCache.String()
		a.mu.Unlock()
		if xmlParseData != "" {
			a.mu.Lock()
			a.taskInstructionCache.Reset()
			a.mu.Unlock()

			// Get the parse error action strategy
			parseAction := "retry"
			if a.cfg != nil && a.cfg.LLM.ParseErrorAction != "" {
				parseAction = a.cfg.LLM.ParseErrorAction
			}

			// exit action: exit the loop and report error
			if parseAction == "exit" {
				lines := strings.SplitN(xmlParseData, "\n---\n", 2)
				firstLine := strings.TrimSpace(lines[0])
				errDetail := firstLine
				var rawDetail string
				if strings.HasPrefix(firstLine, "{") {
					var entry struct {
						Tool  string `json:"tool"`
						Error string `json:"error"`
						Raw   string `json:"raw"`
					}
					if err := json.Unmarshal([]byte(firstLine), &entry); err == nil {
						errDetail = entry.Error
						rawDetail = entry.Raw
					}
				}
				cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyXMLParseErrorExit), errDetail))
				// FEATURE-336: show the raw offending content when the switch is on.
				a.emitParseErrorRaw(cb, rawDetail)
				cb(EventDone, "")
				return "", fmt.Errorf("tool call parse error: %s", errDetail)
			}

			// Parse the first error entry to get the tool name
			lines := strings.SplitN(xmlParseData, "\n---\n", 2)
			firstLine := strings.TrimSpace(lines[0])
			toolName := ""
			var rawDetail string
			if strings.HasPrefix(firstLine, "{") {
				var entry struct {
					Tool  string `json:"tool"`
					Error string `json:"error"`
					Raw   string `json:"raw"`
				}
				if err := json.Unmarshal([]byte(firstLine), &entry); err == nil {
					toolName = entry.Tool
					rawDetail = entry.Raw
				}
			}

			// Get the format suggestion for the tool (XML mode)
			formatSuggestion := buildReferenceFormat(toolName)

			// Build preventive prompt using i18n template
			preventiveTemplate := i18n.T(i18n.KeyXMLParseErrorSuggestion)
			fullFeedback := strings.ReplaceAll(preventiveTemplate, "{TOOL_NAME}", toolName)
			fullFeedback = strings.ReplaceAll(fullFeedback, "{FORMAT}", formatSuggestion)

			// FEATURE-345: consult the problem model for tool format errors
			// when the unified solver is enabled. Its diagnosis may override
			// the generic preventive template (prompt_feedback) or recommend
			// removing the last assistant message (delete_last_msg) / stopping
			// (notify_user). parse-error-action=exit keeps precedence.
			if parseAction != "exit" && a.cfg != nil && a.cfg.LLM.ProblemSolverEnabled {
				// Extract the error detail from the cached JSON entry.
				errDetailForSolver := firstLine
				if strings.HasPrefix(firstLine, "{") {
					var entry struct {
						Tool  string `json:"tool"`
						Error string `json:"error"`
						Raw   string `json:"raw"`
					}
					if err := json.Unmarshal([]byte(firstLine), &entry); err == nil && entry.Error != "" {
						errDetailForSolver = entry.Error
					}
				}
				if report, perr := a.solveProblem(context.Background(), ProblemTypeToolFormatError, errDetailForSolver); perr == nil && report != nil {
					cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverClassified), report.Type, report.Reason, report.SuggestedAction))
					feedback, deleteLast, stop := applyProblemAction(report)
					if stop {
						cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverNotifyUser), report.Reason, report.Guidance))
						cb(EventDone, "")
						return "", fmt.Errorf("problem model recommended stopping: %s", report.Reason)
					}
					if deleteLast {
						a.mu.Lock()
						removed := a.removeLastAssistantWithToolCalls()
						a.mu.Unlock()
						log.Warn("RunStream: problem model recommended delete_last_msg for tool format error, removed %d bytes", len(removed))
						continue
					}
					if feedback != "" {
						// Use the problem model's targeted guidance instead of
						// the generic preventive template.
						fullFeedback = feedback
					}
				} else if perr != nil {
					log.Debug("RunStream: problem solver failed for tool format error, falling back to built-in handling: %v", perr)
					cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverFailed), perr))
				}
			}

			loopFeedback := ""
			var strategyParts []string
			if parseAction == "prompt" {
				loopFeedback = fullFeedback
				strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyPrompt))
			} else {
				// retry: no feedback
				strategyParts = append(strategyParts, i18n.T(i18n.KeyStrategyResend))
			}

			if loopFeedback != "" {
				a.mu.Lock()
				a.messages = append(a.messages, llm.Message{
					Role:    "user",
					Content: loopFeedback,
				})
				a.mu.Unlock()
			}

			// Display a concise error summary extracted from the parse error data.
			errorSummary := ""
			if strings.HasPrefix(firstLine, "{") {
				var entry struct {
					Tool  string `json:"tool"`
					Error string `json:"error"`
					Raw   string `json:"raw"`
				}
				if err := json.Unmarshal([]byte(firstLine), &entry); err == nil && entry.Error != "" {
					errorSummary = entry.Error
					// Truncate very long error messages for the one-line display
					if len(errorSummary) > 120 {
						errorSummary = errorSummary[:120] + "..."
					}
					rawDetail = entry.Raw
				}
			}
			if errorSummary == "" {
				errorSummary = firstLine
				if len(errorSummary) > 120 {
					errorSummary = errorSummary[:120] + "..."
				}
			}
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyXMLParseErrorSummary), errorSummary))
			// FEATURE-336: show the raw offending content when the switch is on.
			a.emitParseErrorRaw(cb, rawDetail)
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopHandling), strings.Join(strategyParts, " → ")))
			cb(EventInfo, "────────────────────────────────────────────\n")
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
				cb(EventDone, "")
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

			if a.completed {
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
						cb(EventTokenIter, fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
							iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
					}
				}

				// Send task-level token usage before done
				taskP, taskC, taskT := a.TaskTokenUsage()
				if taskT > 0 {
					cb(EventTokenTask, fmt.Sprintf("prompt=%d completion=%d total=%d", taskP, taskC, taskT))
				}

				cb(EventDone, "")

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

			// FEATURE-293 phase 3: If the LLM attempted to call a tool but the XML
			// was too broken for stage1/stage2 to catch, divert to parse-error-action
			// instead of following no-tool-action.
			if hasToolAttempt {
				log.Info("Agent.RunStream: hasToolAttempt=true, diverting to parse-error-action instead of no-tool-action")

				// Construct a parse error message and store in taskInstructionCache
				// so the existing XML parse error handling branch processes it.
				errMsg := fmt.Sprintf(
					`{"tool": "", "error": %q}`, i18n.T(i18n.KeyXMLFormatError))
				a.mu.Lock()
				a.taskInstructionCache.WriteString(errMsg)
				a.mu.Unlock()
				continue
			}

			// Rule 2: attempt_completion is available but was NOT called.
			// Apply NoToolAction strategy to decide behavior (FEATURE-286).
			noToolAction := "retry" // default
			if a.cfg != nil && a.cfg.LLM.NoToolAction != "" {
				noToolAction = a.cfg.LLM.NoToolAction
			}

			switch noToolAction {
			case "exit":
				// Treat as final answer — append assistant, send done, return
				iterPrompt, iterComp, iterTotal := a.IterTokenDelta()
				maxModelLen := a.GetMaxModelLen()
				timing := a.GetLLMTiming()
				if iterTotal > 0 {
					tokenUsageMode := "on"
					if a.cfg != nil {
						tokenUsageMode = a.cfg.LLM.TokenUsage
					}
					if tokenUsageMode != "off" {
						cb(EventTokenIter, fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
							iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
					}
				}
				taskP, taskC, taskT := a.TaskTokenUsage()
				if taskT > 0 {
					cb(EventTokenTask, fmt.Sprintf("prompt=%d completion=%d total=%d", taskP, taskC, taskT))
				}
				cb(EventDone, "")

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
				log.Info("Agent.RunStream: exiting after %d iterations (0 tool calls, no-tool-action=exit)", iteration+1)
				return finalContent, nil

			case "retry":
				// Discard assistant content entirely — no messages, no memory.
				// Just continue to the next iteration (FEATURE-286).
				log.Debug("Agent.RunStream: 0 tool calls, no-tool-action=retry, discarding and resending")
				continue

			default: // "prompt" or unknown — backward compatible with FEATURE-17
				// DON'T append the assistant message to history (FEATURE-17 方案C).
				// Only append continuePrompt as a user message to force next action.
				a.mu.Lock()
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

				log.Debug("Agent.RunStream: 0 tool calls, no-tool-action=prompt, sending continue instruction")
				continue
			}
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
				// FEATURE-327: applyLoopIntervention now checks the retried_count
				// limit and may return an error when the user cancels (C option)
				// after the count reaches error-max-single-count. Terminate the
				// task in that case.
				if err := a.applyLoopIntervention(event); err != nil {
					log.Warn("Agent.RunStream: loop intervention cancelled: %v", err)
					return "", nil
				}
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
				cb(EventWarning, fmt.Sprintf(i18n.TF(i18n.KeyContextOverLimit), ep.Warning, usagePct, threshold))

				// FEATURE-345: consult the problem model for context overflow.
				// If it recommends stop (notify_user), surface the problem and
				// terminate the task instead of blindly reorganizing.
				if a.cfg != nil && a.cfg.LLM.ProblemSolverEnabled {
					detail := fmt.Sprintf("context usage %.1f%% exceeded threshold %.0f%% (max model len %d)",
						usagePct, threshold, maxModelLen)
					if report, perr := a.solveProblem(context.Background(), ProblemTypeContextOverflow, detail); perr == nil && report != nil {
						cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverClassified), report.Type, report.Reason, report.SuggestedAction))
						feedback, _, stop := applyProblemAction(report)
						if stop {
							cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverNotifyUser), report.Reason, report.Guidance))
							cb(EventDone, "")
							return "", fmt.Errorf("problem model recommended stopping: %s", report.Reason)
						}
						if feedback != "" {
							// Prepend the model's guidance so the reorganize
							// message includes it on the next iteration.
							a.mu.Lock()
							a.messages = append(a.messages, llm.Message{Role: "user", Content: feedback})
							a.mu.Unlock()
							cb(EventInfo, fmt.Sprintf("\n%s\n", feedback))
							// Keep reorganizePending true so the reorganize
							// instruction still fires after end-of-cycle.
						}
					} else if perr != nil {
						log.Debug("RunStream: problem solver failed for context overflow, falling back to built-in reorganize: %v", perr)
						cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyProblemSolverFailed), perr))
					}
				}
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
							cb(EventCommand, cmd)
						}
					}
				}

				// Show tool call summary (friendly name + intent + key params).
				// The summary is rendered from the parsed args so the user can
				// grasp the impact intention of the call (FEATURE-310).
				// It shares the same display control as show-tool.
				if a.showTool {
					var argsMap map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err == nil {
						cb(EventToolCall, buildToolSummary(tc.Name, argsMap)+"\n")
					} else {
						cb(EventToolCall, tc.Name+"\n")
					}
				}

				log.Info("Agent.RunStream: executing tool %s (ID: %s)", tc.Name, tc.ID)

				// FEATURE-280: Use cancelCtx as parent so Ctrl+C propagates to tool execution.
				// If cancelCtx was already canceled (Ctrl+C pressed during confirmation),
				// Derive from it so the timeout context is also canceled.
				toolCtx := a.CancelContext()
				if toolCtx == nil {
					toolCtx = ctx // fallback if not set
				}
				result, execErr := a.executeToolCall(toolCtx, tc)
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
					// FEATURE-287: Apply parse-error-action strategy
					parseAction := "retry"
					if a.cfg != nil && a.cfg.LLM.ParseErrorAction != "" {
						parseAction = a.cfg.LLM.ParseErrorAction
					}
					if parseAction == "exit" {
						// Exit the loop and report error
						cb(EventError, fmt.Sprintf(i18n.TF(i18n.KeyToolExecFailed), tc.Name, execErr))
						// FEATURE-336: show the raw offending content when the switch is on.
						a.emitParseErrorRaw(cb, tc.Arguments)
						cb(EventDone, "")
						return "", fmt.Errorf("tool %s execution failed: %w", tc.Name, execErr)
					}
					if parseAction == "retry" && !isXMLMode && strings.Contains(errStr, "cannot parse tool arguments") {
						// FIX-314 / FIX-317: OpenAI mode — a JSON parse error
						// means the tool call itself is malformed and cannot be
						// corrected from feedback alone. Remove the assistant
						// message (with tool_calls) so the erroneous call and its
						// error result do NOT enter the context; resend cleanly
						// on the next iteration so the LLM can self-correct the
						// JSON format.
						// Memory keeps the assistant text (finalContent) which is
						// a legitimate reply without tool results.
						a.mu.Lock()
						a.messages = a.messages[:assistantMsgIdx]
						a.mu.Unlock()
						log.Error("Agent.RunStream: tool %s failed, OpenAI retry discarding assistant+tool result: %v", tc.Name, execErr)
						// Show the user a concise error notice (UI only — does NOT
						// enter the LLM context). The invalid call is discarded and
						// the next iteration resends cleanly.
						ep := config.GetEmojiPrefixes(a.emojiEnabled)
						cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyToolExecRetry), ep.Warning, tc.Name, execErr))
						// FEATURE-336: show the raw offending content when the switch is on.
						a.emitParseErrorRaw(cb, tc.Arguments)
						continue iterationLoop
					}
					if parseAction == "retry" && !isXMLMode {
						// FIX-317: OpenAI mode, non-JSON error (bad path, unmatched
						// SEARCH block, missing parameter). Feed the structured
						// error back to the LLM so it can correct the tool call;
						// do NOT discard the assistant+tool messages because the
						// LLM must see the error details to fix them.
						result = formatToolError(tc.Name, execErr)
					} else if parseAction == "retry" {
						// XML mode: simple error message without structured feedback
						result = fmt.Sprintf("Error: %v", execErr)
					} else {
						// prompt (default): Format structured error feedback
						result = formatToolError(tc.Name, execErr)
					}
					// FEATURE-336: show the raw offending content when the switch is on.
					// JSON-syntax errors are displayed above before continue;
					// XML mode carries raw content via taskInstructionCache, so
					// only OpenAI-mode non-syntax failures reach here.
					if !isXMLMode {
						a.emitParseErrorRaw(cb, tc.Arguments)
					}
					log.Error("Agent.RunStream: tool %s failed: %v", tc.Name, execErr)
				}

				// FIX-316: Must-show tool output gated by showTool (distinct from
				// showToolOutput full-detail). attempt_completion / task-plan tools
				// show their full result. For ordinary tools (FEATURE-323), the
				// post-execution outcome receipt is redundant with the pre-execution
				// summary (shown above) — only show it when the tool FAILED, and
				// render it as an error so the user sees the failure reason.
				if a.showTool {
					switch tc.Name {
					case "attempt_completion", "track_task_progress", "view_task_plan":
						if result != "" {
							cb(EventToolCall, result+"\n")
						}
					default:
						if execErr != nil {
							// Tool failed: show the error reason (matches the
							// structured result that was fed back to the LLM).
							cb(EventError, fmt.Sprintf("%s: %s\n", tc.Name, result))
						}
						// Success: outcome receipt omitted to avoid duplicating
						// the pre-execution summary.
					}
				}

				// Show tool call output if enabled (for all tools)
				if a.showToolOutput && result != "" {
					cb(EventToolCall, fmt.Sprintf("  Result:\n%s\n", result))
				}

				// If the result is empty, provide a clear message to the LLM

				toolContent := result
				if toolContent == "" {
					toolContent = i18n.T(i18n.KeyToolNoOutput)
				}

				if isXMLMode {
					// In XML mode, return tool results as user messages with ContentParts structure.
					toolResultMsg := a.buildXMLToolResultMessage(tc.Name, tc.Arguments, toolContent, len(a.messages))
					a.mu.Lock()
					a.messages = append(a.messages, toolResultMsg)
					a.mu.Unlock()
				} else if a.shouldDeferVisionToolResult(tc.Name) {
					// FEATURE-343 minimal mode: skip the placeholder tool
					// message — the recognition round backfills the recognition
					// result as this tool call's ONLY tool message. Writing the
					// placeholder too would duplicate the tool_call_id and get
					// rejected by strict providers. The backfill attaches
					// <environment_details> itself, so skip the post-append
					// steps below as well.
					log.Debug("Agent.RunStream: FEATURE-343 minimal mode, deferring %s placeholder (ID: %s); recognition round will backfill", tc.Name, tc.ID)
					continue
				} else {
					a.mu.Lock()
					a.messages = append(a.messages, llm.Message{
						Role:       "tool",
						Content:    toolContent,
						ToolCallID: tc.ID,
					})
					a.mu.Unlock()
				}

				// FEATURE-292: Flush task instruction cache BEFORE injecting
				// <environment_details>, so the content appears between tool result and <env>.
				// This collects reorganize_context summary, CmdConfirmModify supplemental
				// instructions, and other task-level hints. The <task> wrapper was removed
				// because it distorted LLM attention priority.
				if a.taskInstructionCache.Len() > 0 {
					taskContent := a.taskInstructionCache.String()
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
				// calls buildFullEnvironmentDetails which may need to acquire a.mu
				// for iterating a.messages to find tool call names.
				a.injectTimeAndMessageNoToLast()
			}
		} // end if !reorganizePending

		// FIX-318: After ALL tool results have been appended (and the summary
		// prompt + environment_details have been flushed into the final user
		// message), collapse the history if reorganize_context was called.
		a.collapseAfterReorganize()

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
					cb(EventTokenIter, fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
						iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
				}
			}
			// Send task-level token usage before done
			taskP, taskC, taskT := a.TaskTokenUsage()
			if taskT > 0 {
				cb(EventTokenTask, fmt.Sprintf("prompt=%d completion=%d total=%d", taskP, taskC, taskT))
			}
			cb(EventDone, "")
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
				cb(EventTokenIter, fmt.Sprintf("prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
					iterPrompt, iterComp, iterTotal, maxModelLen, timing.FirstTokenLatency, timing.InputTPS, timing.OutputTPS))
			}
		}

		// FEATURE-292: Flush task instruction cache at the end of each iteration.
		// This collects user supplementary inputs from CmdConfirmModify and other
		// task-level hints (e.g., context overflow warnings). The <task> wrapper
		// was removed because it distorted LLM attention priority.
		if a.taskInstructionCache.Len() > 0 {
			taskContent := a.taskInstructionCache.String()
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
			reorgMsg := i18n.T(i18n.KeyReorganizeUrgent)
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
