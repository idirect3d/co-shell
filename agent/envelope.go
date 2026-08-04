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

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
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

// Loop feedback/retry tag helpers.
// The <loop_feedback> and <retried_count> tags live INSIDE an <environment_details>
// block so that loop feedback messages carry the same environment context as every
// other user message, while keeping the retry counter on the message itself.
const (
	loopFeedbackTag = "loop_feedback"
	retriedCountTag = "retried_count"
)

// lastEnvText returns the text of the last ContentPart of msg if it contains an
// <environment_details> block, otherwise "".
// Plain-Content messages (no ContentParts) always return "".
func lastEnvText(msg *llm.Message) string {
	if msg == nil || len(msg.ContentParts) == 0 {
		return ""
	}
	last := msg.ContentParts[len(msg.ContentParts)-1]
	if strings.Contains(last.Text, "<environment_details>") {
		return last.Text
	}
	return ""
}

// setEnvText replaces the last ContentPart that contains an <environment_details>
// block with envText. If no such part exists, a new text part is appended.
// The first text part (instruction / feedback) is never touched.
func setEnvText(msg *llm.Message, envText string) {
	if msg == nil {
		return
	}
	for i := len(msg.ContentParts) - 1; i >= 0; i-- {
		if strings.Contains(msg.ContentParts[i].Text, "<environment_details>") {
			msg.ContentParts[i].Text = envText
			return
		}
	}
	msg.AppendTextPart(envText)
}

// isLoopFeedbackText reports whether the env text carries the
// <loop_feedback>true</loop_feedback> marker.
func isLoopFeedbackText(envText string) bool {
	return strings.Contains(envText, "<"+loopFeedbackTag+">true</"+loopFeedbackTag+">")
}

// isLoopFeedbackMessage reports whether the last user message is a loop feedback
// message produced by the system (marked with <loop_feedback>true</loop_feedback>).
func isLoopFeedbackMessage(msg *llm.Message) bool {
	return isLoopFeedbackText(lastEnvText(msg))
}

// setLoopFeedbackInText sets (inserting or replacing) the <loop_feedback> tag
// inside an <environment_details> block.
func setLoopFeedbackInText(envText string, val bool) string {
	tag := fmt.Sprintf("<%s>%v</%s>", loopFeedbackTag, val, loopFeedbackTag)
	return setEnvTag(envText, tag)
}

// setRetriedCountInText sets (inserting or replacing) the <retried_count> tag
// inside an <environment_details> block.
func setRetriedCountInText(envText string, n int) string {
	tag := fmt.Sprintf("<%s>%d</%s>", retriedCountTag, n, retriedCountTag)
	return setEnvTag(envText, tag)
}

// getRetriedCountFromText returns the numeric value of the <retried_count> tag
// in the env text (0 when absent or unparsable).
func getRetriedCountFromText(envText string) int {
	startTag := "<" + retriedCountTag + ">"
	endTag := "</" + retriedCountTag + ">"
	start := strings.Index(envText, startTag)
	if start < 0 {
		return 0
	}
	start += len(startTag)
	end := strings.Index(envText[start:], endTag)
	if end < 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(envText[start : start+end]))
	if err != nil {
		return 0
	}
	return n
}

// setEnvTag removes any existing tag with the same name from the block, then
// inserts the given full tag (e.g. "<retried_count>2</retried_count>") right before
// the closing </environment_details> tag. When the block has no closing tag,
// the tag is appended to the text.
func setEnvTag(envText, fullTag string) string {
	// Remove existing tag with the same element name.
	tagName := fullTag[1:strings.Index(fullTag, ">")]
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"
	for {
		s := strings.Index(envText, openTag)
		if s < 0 {
			break
		}
		e := strings.Index(envText[s:], closeTag)
		if e < 0 {
			break
		}
		envText = envText[:s] + envText[s+e+len(closeTag):]
	}

	closeIdx := strings.LastIndex(envText, "</environment_details>")
	if closeIdx < 0 {
		return envText + fullTag
	}
	return envText[:closeIdx] + fullTag + "\n" + envText[closeIdx:]
}

// retryCountCancelError is returned by checkRetryCountLimit when the user
// chooses to cancel the current task (C option) after the retry count limit
// has been reached.
type retryCountCancelError struct{}

func (e *retryCountCancelError) Error() string {
	return "user canceled after retry count limit reached"
}

// checkRetryCountLimit inspects the <retried_count> tag on the last user/tool
// message and compares it against ErrorMaxSingleCount (default 10). When the
// count reaches the threshold and the user has not chosen "ignore all", the
// user is prompted to decide:
//
//   - Enter: reset retried_count to 1 and continue normally
//   - C:     cancel the current task (returns a retryCountCancelError)
//   - A:     set errorApproveAll so this request never prompts again
//
// The existing errorCounter mechanism (error-max-single-count /
// error-max-type-count in run_stream.go) is intentionally unchanged.
//
// Returns (true, nil) when the caller should continue normally, and
// (false, err) when the caller should terminate the current task.
func (a *Agent) checkRetryCountLimit() (bool, error) {
	// Locate the last user/tool message and snapshot it WITHOUT holding the
	// lock, so prompt I/O and env rewriting below cannot deadlock.
	a.mu.Lock()
	lastUserIdx := -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		role := a.messages[i].Role
		if role == "user" || role == "tool" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		a.mu.Unlock()
		return true, nil
	}
	lastMsg := a.messages[lastUserIdx]
	a.mu.Unlock()

	// Read the current retried count from the last user/tool message envelope.
	count := getRetriedCountFromText(lastEnvText(&lastMsg))

	// Get the configured limit (must be > 0 to take effect, like run_stream.go).
	maxSingle := 10
	if a.cfg != nil && a.cfg.LLM.ErrorMaxSingleCount > 0 {
		maxSingle = a.cfg.LLM.ErrorMaxSingleCount
	}

	// Below threshold, or the user already chose "ignore all": no prompt.
	if count < maxSingle || a.errorApproveAll {
		return true, nil
	}

	// Prompt the user for action via UserIO interface.
	ep := config.GetEmojiPrefixes(a.emojiEnabled)
	io := a.defaultIO()
	promptReason := fmt.Sprintf(i18n.TF(i18n.KeyErrRepeatPrompt), count, maxSingle)
	io.Printf("\n%s %s: %s\n", ep.Warning, i18n.T(i18n.KeyErrRepeatWarn), promptReason)
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
	lower := strings.ToLower(strings.TrimSpace(response))

	switch lower {
	case "c":
		// User cancelled — terminate the task.
		io.Printf("\n%s %s\n", ep.Error, i18n.T(i18n.KeyUserCancelled))
		return false, &retryCountCancelError{}
	case "a":
		// User chose to ignore all error limits for this request.
		a.errorApproveAll = true
		io.Printf("\n%s %s\n", ep.Success, i18n.T(i18n.KeyErrIgnoredContinue))
		return true, nil
	default:
		// Enter pressed — reset retried_count to 1 and continue.
		newEnv := setRetriedCountInText(lastEnvText(&lastMsg), 1)
		a.mu.Lock()
		if lastUserIdx < len(a.messages) {
			setEnvText(&a.messages[lastUserIdx], newEnv)
		}
		a.mu.Unlock()
		io.Printf("\n%s %s\n", ep.Success, i18n.T(i18n.KeyErrRetryContinue))
		return true, nil
	}
}

// applyLoopFeedback applies a loop intervention to the message history (FIX-321).
//
// Two behaviors:
//
//  1. feedback != "" (prompt / reorganize strategies): the intervention carries
//     a user-facing prompt. A "loop feedback message" is a system-generated user
//     message whose env carries <loop_feedback>true</loop_feedback>. If the last
//     user message is already such a message, its feedback text is replaced and
//     <retried_count> is incremented (no new message). Otherwise a NEW feedback
//     message is appended — a real user/tool message is never overwritten.
//
//  2. feedback == "" (retry / temperature strategies): no new message is
//     appended. The <retried_count> tag on the last user/tool message's env is
//     incremented (the tag is created first when absent, building a full env
//     block when the message has none). The original instruction text is never
//     modified.
//
// The retried count lives only on the message envelope; there is no agent-level
// state and no reset logic — a fresh user request starts a new chain naturally.
//
// Returns the retried count recorded on the affected message (0 when no
// user/tool message exists).
func (a *Agent) applyLoopFeedback(feedback string) int {
	// Locate the last user/tool message and snapshot it WITHOUT holding the
	// lock, so env construction (which re-acquires a.mu via
	// buildOpenedResources / IterTokenDelta) cannot deadlock.
	a.mu.Lock()
	if len(a.messages) == 0 {
		a.mu.Unlock()
		return 0
	}
	var lastUserIdx, messageTotal int
	lastUserIdx = -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		role := a.messages[i].Role
		if role == "user" || role == "tool" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		a.mu.Unlock()
		return 0
	}
	messageTotal = len(a.messages)
	lastMsg := a.messages[lastUserIdx]
	a.mu.Unlock()

	lastEnv := lastEnvText(&lastMsg)

	if feedback == "" {
		// retry / temperature: bump the counter on the last user/tool message.
		count := getRetriedCountFromText(lastEnv) + 1
		if lastEnv == "" {
			// Message has no env block — build a complete one.
			newEnv := a.buildFullEnvironmentDetails(messageTotal-1, nil)
			newEnv = setRetriedCountInText(newEnv, count)
			a.mu.Lock()
			msg := &a.messages[lastUserIdx]
			if len(msg.ContentParts) == 0 {
				msg.ContentParts = []llm.ContentPart{
					{Type: llm.ContentPartText, Text: msg.Content},
				}
				msg.Content = ""
			}
			setEnvText(msg, newEnv)
			a.mu.Unlock()
			return count
		}
		newEnv := setRetriedCountInText(lastEnv, count)
		a.mu.Lock()
		setEnvText(&a.messages[lastUserIdx], newEnv)
		a.mu.Unlock()
		return count
	}

	// prompt / reorganize: create or update a loop feedback message.
	if isLoopFeedbackMessage(&lastMsg) {
		// Update the existing feedback message in place — no new message.
		count := getRetriedCountFromText(lastEnv) + 1
		newEnv := setRetriedCountInText(lastEnv, count)
		a.mu.Lock()
		msg := &a.messages[lastUserIdx]
		if len(msg.ContentParts) > 0 {
			// First text part is the feedback prompt — replace it.
			msg.ContentParts[0].Text = feedback
		} else {
			msg.Content = feedback
		}
		setEnvText(msg, newEnv)
		a.mu.Unlock()
		return count
	}

	// Otherwise append a NEW feedback message after the last user message.
	count := 1
	newEnv := a.buildFullEnvironmentDetails(messageTotal, nil)
	newEnv = setLoopFeedbackInText(newEnv, true)
	newEnv = setRetriedCountInText(newEnv, count)
	a.mu.Lock()
	a.messages = append(a.messages, llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: feedback},
			{Type: llm.ContentPartText, Text: newEnv},
		},
	})
	a.mu.Unlock()
	return count
}
