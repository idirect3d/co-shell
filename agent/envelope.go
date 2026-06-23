// Author: L.Shuang
// Created: 2026-06-14
// Last Modified: 2026-06-14
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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// injectTimeAndMessageNo appends a minimal <environment_details> block with time and
// message_no to all user and tool messages. This ensures every message carries
// temporal and positional context for the LLM.
func injectTimeAndMessageNo(msgs []llm.Message) []llm.Message {
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	for i := range msgs {
		if msgs[i].Role == "user" || msgs[i].Role == "tool" {
			var sb strings.Builder
			sb.WriteString("\n\n<environment_details>\n")
			sb.WriteString("<time>")
			sb.WriteString(now)
			sb.WriteString("</time>\n")
			sb.WriteString("<message_no>")
			sb.WriteString(strconv.Itoa(i))
			sb.WriteString("</message_no>\n")
			sb.WriteString("</environment_details>")
			msgs[i].Content += sb.String()
		}
	}
	return msgs
}

// stripEnvelopes removes all <environment_details>...</environment_details> blocks from
// all user/tool/assistant message contents. This prevents stale data from being sent to the LLM.
func (a *Agent) stripEnvelopes(msgs []llm.Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	copy(result, msgs)
	for i := range result {
		if result[i].Role == "user" || result[i].Role == "tool" {
			result[i].Content = stripSingleEnvelope(result[i].Content)
		}
	}
	return result
}

// stripSingleEnvelope removes a single <environment_details>...</environment_details> block
// from the given content string.
func stripSingleEnvelope(content string) string {
	start := strings.Index(content, "<environment_details>")
	if start < 0 {
		return content
	}
	end := strings.Index(content, "</environment_details>")
	if end < 0 {
		return strings.TrimSpace(content[:start])
	}
	end += len("</environment_details>")
	before := strings.TrimSpace(content[:start])
	after := strings.TrimSpace(content[end:])
	result := before
	if after != "" {
		if result != "" {
			result += "\n"
		}
		result += after
	}
	return result
}

// taskPlanTools lists the LLM tools that manage task plans. When the LLM has just
// called one of these, the task plan content is already in the tool result message
// and should not be duplicated in <environment_details>.
var taskPlanTools = map[string]bool{
	"track_task_progress": true,
	"view_task_plan":      true,
}

// isLastToolTaskPlan checks whether the most recent tool call in a.messages is one
// of the task plan tools. This is called per-iteration from injectEnvelopeToLastUser.
func (a *Agent) isLastToolTaskPlan() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := len(a.messages) - 1; i >= 0; i-- {
		msg := a.messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if taskPlanTools[tc.Name] {
					return true
				}
			}
			return false
		}
	}
	return false
}

// injectEnvelopeToLastUser finds the last user message in msgs and appends a fresh
// <environment_details> block with current time, working directory, file listing,
// and optionally task plan progress (unless a task plan tool was just called).
func (a *Agent) injectEnvelopeToLastUser(msgs []llm.Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	copy(result, msgs)

	lastUserIdx := -1
	for i := len(result) - 1; i >= 0; i-- {
		if result[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return result
	}

	// Skip task plan in envelope if a task plan tool was just called.
	// Check is done per-iteration against a.messages so it re-evaluates each time.
	skipTaskPlan := a.isLastToolTaskPlan()

	cwd, _ := os.Getwd()
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	taskPlan := a.getTaskPlanPrompt()

	// Message number derived from current a.messages length (last index)
	messageNo := len(a.messages) - 1

	// Top-level files (depth=0) and two-level listing (depth=1) for bin and research
	files := strings.TrimRight(listFilesForPrompt(cwd, 0, 128), "\n")
	binFiles := strings.TrimRight(listFilesForPrompt(filepath.Join(cwd, "bin"), 0, 64), "\n")
	researchFiles := strings.TrimRight(listFilesForPrompt(filepath.Join(cwd, "research"), 0, 64), "\n")

	// Get per-iteration token usage for context_window (most recent LLM call only)
	promptTokens, completionTokens, totalTokens := a.IterTokenDelta()

	// Get max model length from the current active model
	maxModelLen := 0
	if a.modelManager != nil {
		if modelCfg := a.modelManager.GetActiveModel(false); modelCfg != nil {
			maxModelLen = modelCfg.MaxModelLen
		}
	}

	// Calculate context usage percentage
	contextPercentage := ""
	if maxModelLen > 0 && totalTokens > 0 {
		pct := float64(totalTokens) * 100.0 / float64(maxModelLen)
		contextPercentage = fmt.Sprintf("%.1f", pct)
	}

	var sb strings.Builder
	sb.WriteString("\n\n<environment_details>\n")
	sb.WriteString("<time>")
	sb.WriteString(now)
	sb.WriteString("</time>\n")
	sb.WriteString("<message_no>")
	sb.WriteString(strconv.Itoa(messageNo))
	sb.WriteString("</message_no>\n")
	sb.WriteString("<context_window>\n")
	sb.WriteString("<used_tokens>")
	sb.WriteString(strconv.Itoa(totalTokens))
	sb.WriteString("</used_tokens>\n")
	sb.WriteString("<prompt_tokens>")
	sb.WriteString(strconv.Itoa(promptTokens))
	sb.WriteString("</prompt_tokens>\n")
	sb.WriteString("<completion_tokens>")
	sb.WriteString(strconv.Itoa(completionTokens))
	sb.WriteString("</completion_tokens>\n")
	sb.WriteString("<max_tokens>")
	sb.WriteString(strconv.Itoa(maxModelLen))
	sb.WriteString("</max_tokens>\n")
	if contextPercentage != "" {
		sb.WriteString("<percentage>")
		sb.WriteString(contextPercentage)
		sb.WriteString("%</percentage>\n")
	}
	sb.WriteString("</context_window>\n")
	sb.WriteString("<cwd>")
	sb.WriteString(cwd)
	sb.WriteString("</cwd>\n")
	sb.WriteString("<files>\n")
	sb.WriteString(files)
	sb.WriteString("\n</files>\n")
	if binFiles != "" {
		sb.WriteString("<bin>\n")
		sb.WriteString(binFiles)
		sb.WriteString("\n</bin>\n")
	}
	if researchFiles != "" {
		sb.WriteString("<research>\n")
		sb.WriteString(researchFiles)
		sb.WriteString("\n</research>\n")
	}
	if taskPlan != "" && !skipTaskPlan {
		sb.WriteString("<task_plan>\n")
		sb.WriteString(taskPlan)
		sb.WriteString("\n</task_plan>\n")
	}
	sb.WriteString("</environment_details>")

	result[lastUserIdx] = llm.Message{
		Role:    result[lastUserIdx].Role,
		Content: result[lastUserIdx].Content + sb.String(),
	}
	return result
}
