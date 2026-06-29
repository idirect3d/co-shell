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

	// Log the operation
	log.Info("reorganizeContextTool: context reorganized, summary_prompt (%d chars)", len(summaryPrompt))
	log.Debug("reorganizeContextTool: summary_prompt: %s", summaryPrompt)

	// Clean up old context: keep only the system prompt.
	// The tool result (containing the embedded <task>) will be appended by the
	// caller (run_stream.go / run.go), then injected with <environment_details>
	// by injectTimeAndMessageNoToLast. This gives the LLM a fresh start with
	// a single message: result + <task> + <env>.
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		systemMsg := a.messages[0]
		a.messages = []llm.Message{systemMsg}
	}
	a.messagePointer = 1
	a.needAdjustPointer = true

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
	a.mu.Unlock()

	// Store the summary_prompt in the task instruction cache.
	// The caller (run_stream.go) will wrap it with <task> tags when flushing,
	// so we must NOT add <task> here to avoid double-wrapping.
	if a.taskInstructionCache.Len() > 0 {
		a.taskInstructionCache.WriteString("\n\n")
	}
	a.taskInstructionCache.WriteString(summaryPrompt)

	// Build the result message (without embedded <task>).
	// The <task> will be appended as a separate ContentPart by the flush mechanism.
	result := fmt.Sprintf(i18n.T(i18n.KeyReorganizeResult), len(summaryPrompt))
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
		// smart/task/reorganize: do NOT clear context here — only return a suggestion
		// for the LLM to call reorganize_context if needed. This preserves all prior
		// conversation history so the user does not lose context on Ctrl+C/ESC exit.
		// The loop feedback message is appended by the caller (run_stream.go).
		log.Info("reorganizeContextOnLoop: policy=%s, returning reorganize suggestion (context preserved)", policy)
		suggestion := i18n.T(i18n.KeyLoopReorganizeSuggestion)
		return suggestion
	}
}
