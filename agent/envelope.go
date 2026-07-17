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

// buildOpenedResources builds an <opened_resources> block listing all currently open
// resources that can be persisted across iterations: browser tabs, Excel sessions,
// Word sessions, and shell session.
func (a *Agent) buildOpenedResources() string {
	var sb strings.Builder
	sb.WriteString("<opened_resources>\n")

	// Browser — check if Chrome manager is running
	a.mu.Lock()
	chromeRunning := a.chromeMgr != nil && a.chromeMgr.IsRunning()
	a.mu.Unlock()
	if chromeRunning {
		sb.WriteString("  <browser>running</browser>\n")
	} else {
		sb.WriteString("  <browser>none</browser>\n")
	}

	// Excel sessions
	if a.excelSessionMgr != nil {
		sessions := a.excelSessionMgr.listSessions()
		if len(sessions) > 0 {
			sb.WriteString("  <excel>\n")
			for _, s := range sessions {
				sb.WriteString(fmt.Sprintf("    <session>%s</session>\n", s))
			}
			sb.WriteString("  </excel>\n")
		} else {
			sb.WriteString("  <excel>none</excel>\n")
		}
	}

	// Word/DOCX sessions
	if a.docxSessionMgr != nil {
		sessions := a.docxSessionMgr.listSessions()
		if len(sessions) > 0 {
			sb.WriteString("  <word>\n")
			for _, s := range sessions {
				sb.WriteString(fmt.Sprintf("    <session>%s</session>\n", s))
			}
			sb.WriteString("  </word>\n")
		} else {
			sb.WriteString("  <word>none</word>\n")
		}
	}

	// Shell session
	a.mu.Lock()
	hasShell := a.shellSession != nil && a.shellSession.IsRunning()
	a.mu.Unlock()
	if hasShell {
		sb.WriteString("  <shell>running</shell>\n")
	} else {
		sb.WriteString("  <shell>none</shell>\n")
	}

	sb.WriteString("</opened_resources>")
	return sb.String()
}

// buildFullEnvironmentDetails constructs the complete <environment_details> block
// with time, message_no, context_window, cwd, files, bin, research, task_plan,
// and opened_resources.
// Used by both injectEnvelopeToLastUser (for user messages) and
// injectTimeAndMessageNoToLast (for tool result messages).
//
// toolCallNames: when non-empty, names of the tool calls that produced this result message.
// The <task_plan> block is skipped only when the current result is produced by
// track_task_progress, view_task_plan, or attempt_completion — because the plan
// content is already captured in the tool result message.
// For user messages (toolCallNames is nil/empty), <task_plan> is always included.
func (a *Agent) buildFullEnvironmentDetails(messageNo int, toolCallNames []string) string {
	cwd, _ := os.Getwd()
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	taskPlan := a.getTaskPlanPrompt()

	// Skip <task_plan> only when the current result message is a direct response
	// to a task plan tool or attempt_completion (content already in the result).
	// For user messages (toolCallNames is nil/empty), always include <task_plan>.
	skipTaskPlan := false
	if len(toolCallNames) > 0 {
		for _, name := range toolCallNames {
			if name == "track_task_progress" || name == "view_task_plan" || name == "attempt_completion" {
				skipTaskPlan = true
				break
			}
		}
	}

	// Top-level files (depth=0) and two-level listing (depth=1) for bin and research
	files := strings.TrimRight(listFilesForPrompt(cwd, 0, 128).listing, "\n")
	binFiles := strings.TrimRight(listFilesForPrompt(filepath.Join(cwd, "bin"), 0, 64).listing, "\n")
	researchFiles := strings.TrimRight(listFilesForPrompt(filepath.Join(cwd, "research"), 0, 64).listing, "\n")

	// Get per-iteration token usage for context_window (most recent LLM call only)
	_, _, totalTokens := a.IterTokenDelta()

	// Get max model length from the current active model
	maxModelLen := 0
	if a.modelManager != nil {
		if modelCfg := a.modelManager.GetActiveModel(false); modelCfg != nil {
			maxModelLen = modelCfg.MaxModelLen
		}
	}

	var sb strings.Builder
	sb.WriteString("<environment_details>\n")
	sb.WriteString("<time>")
	sb.WriteString(now)
	sb.WriteString("</time>\n")
	sb.WriteString("<message_no>")
	sb.WriteString(strconv.Itoa(messageNo))
	sb.WriteString("</message_no>\n")
	sb.WriteString("<context_window>")
	sb.WriteString(formatTokens(totalTokens))
	sb.WriteString(" / ")
	sb.WriteString(formatTokenSize(maxModelLen))
	sb.WriteString(" tokens used (")
	if maxModelLen > 0 {
		pct := int(float64(totalTokens) * 100.0 / float64(maxModelLen))
		sb.WriteString(strconv.Itoa(pct))
		sb.WriteString("%)")
	} else {
		sb.WriteString("?%)")
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
	// Append opened resources block
	sb.WriteString(a.buildOpenedResources())
	sb.WriteString("\n")
	sb.WriteString("</environment_details>")
	return sb.String()
}

// refreshLastUserEnvelope updates only the <time> tag in the last user message's
// <environment_details>. All other content (task_plan, opened_resources, etc.)
// is preserved as-is from when the message was first created.
func (a *Agent) refreshLastUserEnvelope() {
	now := time.Now().Format("2006-01-02 15:04:05 Monday")

	a.mu.Lock()
	defer a.mu.Unlock()

	// Find the last user message
	lastUserIdx := -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return
	}

	msg := &a.messages[lastUserIdx]
	if len(msg.ContentParts) == 0 {
		return
	}

	// Update only the <time> tag in the last ContentPart
	lastPart := &msg.ContentParts[len(msg.ContentParts)-1]
	if strings.Contains(lastPart.Text, "<environment_details>") {
		lastPart.Text = replaceTimeTag(lastPart.Text, now)
	}
}

// replaceTimeTag replaces the content of <time>...</time> in the given text.
func replaceTimeTag(text, newTime string) string {
	start := strings.Index(text, "<time>")
	if start < 0 {
		return text
	}
	start += len("<time>")
	end := strings.Index(text[start:], "</time>")
	if end < 0 {
		return text
	}
	return text[:start] + newTime + text[start+end:]
}

// injectTimeAndMessageNoToLast appends a full <environment_details> block to the LAST
// message in a.messages that is a user or tool message. Uses the shared
// buildFullEnvironmentDetails method so all messages get consistent env context.
// This is called after adding a tool result to freeze its environment context.
//
// FEATURE-17: Instead of searching from the end of history for any task plan tool call,
// we look backward for the immediately preceding assistant message with tool_calls
// and extract those tool names. The <task_plan> block is skipped only when the
// current result message is a direct response to track_task_progress, view_task_plan,
// or attempt_completion — because the plan content is already in the result text.
func (a *Agent) injectTimeAndMessageNoToLast() {
	if len(a.messages) == 0 {
		return
	}
	lastIdx := len(a.messages) - 1
	msg := &a.messages[lastIdx]
	if msg.Role != "user" && msg.Role != "tool" {
		return
	}

	// Find tool call names from the preceding assistant message.
	// This tells us which tool(s) produced this result message.
	var toolCallNames []string
	a.mu.Lock()
	for i := lastIdx - 1; i >= 0; i-- {
		if a.messages[i].Role == "assistant" && len(a.messages[i].ToolCalls) > 0 {
			for _, tc := range a.messages[i].ToolCalls {
				toolCallNames = append(toolCallNames, tc.Name)
			}
			break
		}
	}
	a.mu.Unlock()

	envText := a.buildFullEnvironmentDetails(lastIdx, toolCallNames)

	// Convert to ContentParts if not already
	if len(msg.ContentParts) == 0 {
		msg.ContentParts = []llm.ContentPart{
			{Type: llm.ContentPartText, Text: msg.Content},
		}
		msg.Content = ""
	}
	msg.AppendTextPart(envText)
}

// injectEnvelopeToLastUser finds the last user message in msgs and appends a fresh
// <environment_details> block using the shared buildFullEnvironmentDetails method.
// This ensures all messages (user + tool) use the same env format.
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

	messageNo := len(a.messages) - 1
	// User messages always include <task_plan> — pass nil for toolCallNames.
	envText := a.buildFullEnvironmentDetails(messageNo, nil)

	// Always use ContentParts format for the last user message so the envelope
	// (current time, files, task plan) is a separate text part from the instruction.
	// This ensures the LLM request body uses the array format:
	//   content: [{"type":"text","text":"instruction"}, {"type":"text","text":"<env>"}]
	// If the message already uses ContentParts, just append a new one.
	// If it uses plain Content, convert to ContentParts first, then append.
	existing := result[lastUserIdx]
	if len(existing.ContentParts) == 0 {
		// Convert plain Content to ContentParts format so the envelope
		// can be appended as a separate text part.
		existing.ContentParts = []llm.ContentPart{
			{Type: llm.ContentPartText, Text: existing.Content},
		}
		existing.Content = ""
	}
	existing.AppendTextPart(envText)
	result[lastUserIdx] = existing
	return result
}

// formatTokens formats a number with thousand separators (e.g., 67812 → "67,812").
func formatTokens(n int) string {
	s := strconv.Itoa(n)
	// Insert comma separators every 3 digits from the right
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{s[start:i]}, parts...)
	}
	return strings.Join(parts, ",")
}

// formatTokenSize formats a maximum model length with K/M suffix (e.g., 262144 → "256K").
func formatTokenSize(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n%1024 == 0 {
		return strconv.Itoa(n/1024) + "K"
	}
	if n%1048576 == 0 {
		return strconv.Itoa(n/1048576) + "M"
	}
	// Round to nearest K
	k := (n + 512) / 1024
	return strconv.Itoa(k) + "K"
}
