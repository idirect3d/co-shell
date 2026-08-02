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

	// FIX-318: Do NOT clear a.messages here. The caller (run_stream.go / run.go)
	// has already appended the assistant message with tool_calls, and will append
	// the tool result message right after this callback returns. Clearing the
	// history now would orphan that tool message (no preceding assistant with
	// tool_calls), which OpenAI rejects with HTTP 400.
	// Instead, mark the flag and let the caller collapse the history to
	// [system, user(summary)] AFTER all tool results have been appended.
	a.reorganizeContextUsed = true

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
	// The caller (run_stream.go) flushes it as a ContentPart appended to the
	// tool result message.
	if a.taskInstructionCache.Len() > 0 {
		a.taskInstructionCache.WriteString("\n\n")
	}
	a.taskInstructionCache.WriteString(summaryPrompt)

	// Build the result message (the summary prompt will be flushed as a
	// separate ContentPart by the caller).
	result := fmt.Sprintf(i18n.T(i18n.KeyReorganizeResult), len(summaryPrompt))
	log.Info("reorganizeContextTool: result=%s", result)
	return result, nil
}

// collapseAfterReorganize collapses the message history after reorganize_context
// was called. It MUST be invoked AFTER all tool results have been appended (and,
// in the streaming path, after the summary prompt + environment_details have been
// flushed into the final user message). It leaves only [system, user(summary)] so
// that no orphaned tool message (without a preceding assistant tool_calls) is ever
// sent to the API (FIX-318).
//
// Two call paths:
//   - RunStream: the summary was already flushed into the last user message via
//     taskInstructionCache; the cache is empty here, so the last user message is
//     reused as-is.
//   - Run (non-streaming): the cache is NOT flushed; the summary is still in
//     taskInstructionCache, so a fresh user message is built from it.
func (a *Agent) collapseAfterReorganize() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.reorganizeContextUsed {
		return
	}
	a.reorganizeContextUsed = false

	if len(a.messages) == 0 {
		return
	}
	systemMsg := a.messages[0]

	// Non-streaming path: summary still pending in cache → build user message.
	summary := a.taskInstructionCache.String()
	a.taskInstructionCache.Reset()
	if summary != "" {
		a.messages = []llm.Message{systemMsg, llm.Message{Role: "user", Content: summary}}
		a.messagePointer = 1
		a.needAdjustPointer = true
		log.Info("Agent.collapseAfterReorganize: collapsed history to [system, user(summary)] using cache")
		return
	}

	// Streaming path: summary already flushed into the last user message.
	var lastUser llm.Message
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "user" {
			lastUser = a.messages[i]
			break
		}
	}
	if lastUser.Role == "user" {
		a.messages = []llm.Message{systemMsg, lastUser}
	} else {
		a.messages = []llm.Message{systemMsg}
	}
	a.messagePointer = 1
	a.needAdjustPointer = true
	log.Info("Agent.collapseAfterReorganize: collapsed history to %d messages", len(a.messages))
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
