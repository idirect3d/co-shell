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
// When isXMLMode is true, the envelope is appended as a ContentPart instead of
// concatenated to the Content string, keeping the envelope as a separate structured
// segment in the message array.
func injectTimeAndMessageNo(msgs []llm.Message) []llm.Message {
	// Find the last user message index — it will later receive a full
	// <environment_details> from injectEnvelopeToLastUser, so skip it here
	// to avoid duplicate envelope parts (FEATURE-248).
	lastUserIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	for i := range msgs {
		if i == lastUserIdx {
			continue // skip last user, injectEnvelopeToLastUser handles it with full content
		}
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
			// Convert to ContentParts if not already, so environment_details is always
			// a separate text part from the actual content.
			if len(msgs[i].ContentParts) == 0 {
				msgs[i].ContentParts = []llm.ContentPart{
					{Type: llm.ContentPartText, Text: msgs[i].Content},
				}
				msgs[i].Content = ""
			}
			msgs[i].AppendTextPart(sb.String())
		}
	}
	return msgs
}

// stripEnvelopes removes all <environment_details>...</environment_details> blocks from
// all user/tool/assistant message contents. This prevents stale data from being sent to the LLM.
// In XML mode when messages use ContentParts, stripping searches within text parts instead.
func (a *Agent) stripEnvelopes(msgs []llm.Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	copy(result, msgs)
	for i := range result {
		msg := &result[i]
		if msg.Role == "user" || msg.Role == "tool" {
			if len(msg.ContentParts) > 0 {
				// XML mode: strip envelopes from ContentParts
				var cleaned []llm.ContentPart
				for _, cp := range msg.ContentParts {
					if cp.Type == llm.ContentPartText {
						cleanedText := stripSingleEnvelope(cp.Text)
						if cleanedText != "" {
							cleaned = append(cleaned, llm.ContentPart{
								Type: llm.ContentPartText,
								Text: cleanedText,
							})
						}
					} else {
						cleaned = append(cleaned, cp)
					}
				}
				msg.ContentParts = cleaned
			} else {
				msg.Content = stripSingleEnvelope(msg.Content)
			}
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

// buildFullEnvironmentDetails constructs the complete <environment_details> block
// with time, message_no, context_window, cwd, files, bin, research, and task_plan.
// Used by both injectEnvelopeToLastUser (for user messages) and
// injectTimeAndMessageNoToLast (for tool result messages).
func (a *Agent) buildFullEnvironmentDetails(messageNo int) string {
	cwd, _ := os.Getwd()
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	taskPlan := a.getTaskPlanPrompt()
	skipTaskPlan := a.isLastToolTaskPlan()

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
	sb.WriteString("</environment_details>")
	return sb.String()
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

// injectTimeAndMessageNoToLast appends a full <environment_details> block to the LAST
// message in a.messages that is a user or tool message. Uses the shared
// buildFullEnvironmentDetails method so all messages get consistent env context.
// This is called after adding a tool result to freeze its environment context.
func (a *Agent) injectTimeAndMessageNoToLast() {
	if len(a.messages) == 0 {
		return
	}
	lastIdx := len(a.messages) - 1
	msg := &a.messages[lastIdx]
	if msg.Role != "user" && msg.Role != "tool" {
		return
	}

	envText := a.buildFullEnvironmentDetails(lastIdx)

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
	envText := a.buildFullEnvironmentDetails(messageNo)

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
