// Author: L.Shuang
// Created: 2026-06-24
// Last Modified: 2026-06-25
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
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// reorganizeContextTool handles the reorganize_context tool call.
// It appends the LLM-generated summary_prompt as a new user message,
// then moves the messagePointer to the new message position.
//
// The LLM is expected to generate the summary_prompt by:
// 1. Reviewing the original task goal and progress
// 2. Analyzing which approaches worked and which didn't
// 3. Proposing optimized strategies based on the analysis
// 4. Preserving all critical hard data (file paths, error logs, code snippets)
// 5. Generating a self-contained continuation prompt
func (a *Agent) reorganizeContextTool(ctx context.Context, args map[string]interface{}) (string, error) {
	summaryPrompt, _ := args["summary_prompt"].(string)
	if summaryPrompt == "" {
		return "", fmt.Errorf("summary_prompt is required")
	}

	// Strip leading/trailing whitespace
	summaryPrompt = strings.TrimSpace(summaryPrompt)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Append the summary_prompt as a new user message, wrapped in <task> tags.
	// This tells the LLM that this is the primary task objective for the fresh context.
	wrappedContent := fmt.Sprintf("<task>\n%s\n</task>", summaryPrompt)
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: wrappedContent,
	})

	// Move messagePointer to the new message
	newIndex := len(a.messages) - 1
	a.messagePointer = newIndex
	a.needAdjustPointer = true

	// Log the operation
	log.Info("reorganizeContextTool: context reorganized, messagePointer moved to %d (total messages: %d)", newIndex, len(a.messages))
	log.Debug("reorganizeContextTool: summary_prompt (%d chars): %s", len(summaryPrompt), summaryPrompt)

	// Reset loop detection state — the new context should not inherit old loop state
	a.loopDetectCrit = false
	if a.loopDetector != nil {
		a.loopDetector.Reset()
	}
	if a.toolCallLoopDetector != nil {
		a.toolCallLoopDetector.Reset()
	}
	// Reset content comparison and judgment state so reorganize's fresh context
	// is not falsely flagged as a duplicate of pre-reorganize content.
	a.lastAssistantContent = ""
	a.lastLlmOutput = ""

	// Build the result message
	result := fmt.Sprintf(i18n.T(i18n.KeyReorganizeResult), len(summaryPrompt), newIndex)
	log.Info("reorganizeContextTool: result=%s", result)
	return result, nil
}

// reorganizeContextOnLoop is called when a loop is confirmed and LoopReorganizeEnabled is true.
// It performs context reorganization based on the current ContextPolicy:
// - "window" with context-limit=-1: trim context to system + last user message
// - "smart"/"task"/"reorganize": append i18n suggestion to call reorganize_context
// Returns the suggestion message to append to loop feedback, or empty string if none.
func (a *Agent) reorganizeContextOnLoop() string {
	if a.cfg == nil || !a.cfg.LLM.LoopReorganizeEnabled {
		return ""
	}

	policy := a.cfg.LLM.ContextPolicy
	if policy == "" {
		policy = "reorganize"
	}

	switch policy {
	case "window":
		// In window mode with unlimited context, force a clean window
		if a.cfg.LLM.ContextLimit == -1 {
			a.mu.Lock()
			if len(a.messages) > 1 {
				systemMsg := a.messages[0]
				lastUserIdx := len(a.messages) - 1
				lastUserMsg := a.messages[lastUserIdx]
				a.messages = []llm.Message{systemMsg, lastUserMsg}
				a.messagePointer = 1
				a.needAdjustPointer = true
			}
			a.mu.Unlock()
			log.Info("reorganizeContextOnLoop: window mode, context trimmed to system + last user message")
		}
		return ""
	default:
		// smart/task/reorganize: automatically rebuild context into a minimal fresh window.
		// Keep only system prompt + the last user message (the reorganize result or loop feedback).
		// This immediately breaks the loop without waiting for the LLM to call reorganize_context.
		a.mu.Lock()
		if len(a.messages) > 1 {
			systemMsg := a.messages[0]
			lastUserIdx := len(a.messages) - 1
			lastUserMsg := a.messages[lastUserIdx]
			a.messages = []llm.Message{systemMsg, lastUserMsg}
			a.messagePointer = 1
			a.needAdjustPointer = true
		}
		a.mu.Unlock()
		log.Info("reorganizeContextOnLoop: policy=%s, context auto-trimmed to system + last user message", policy)
		// Return a brief suggestion that the LLM should call reorganize_context to better organize
		suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
		return suggestion
	}
}
