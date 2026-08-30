// Author: L.Shuang
// Created: 2026-08-30
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

// SupervisorEntry identifies which of the three intervention points triggered
// a supervision review (FEATURE-456).
type SupervisorEntry string

const (
	// SupervisorEntryA: the main LLM called attempt_completion (explicit delivery).
	SupervisorEntryA SupervisorEntry = "attempt_completion"
	// SupervisorEntryB: the main LLM exited without calling any tool (auto exit).
	SupervisorEntryB SupervisorEntry = "no_tool_exit"
	// SupervisorEntryC: the main LLM updated task progress and marked steps completed.
	SupervisorEntryC SupervisorEntry = "task_progress_complete"
)

// SupervisorReview is the structured payload the supervisor model returns via
// the submit_review tool (FEATURE-456). All three fields are required.
type SupervisorReview struct {
	// Approved: whether the main LLM's delivery meets the user's ultimate goal.
	Approved bool `json:"approved"`
	// Reason: why the supervisor approves or rejects the delivery.
	Reason string `json:"reason"`
	// Suggestion: concrete, actionable advice for the main LLM (or for the user).
	Suggestion string `json:"suggestion"`
}

// supervisorContext holds the supervisor LLM's session-bound, independent
// conversation context (FEATURE-456). It accumulates across reviews within a
// session unless clearContext is enabled.
type supervisorContext struct {
	// messages is the supervisor's own conversation history (system + user/assistant).
	messages []llm.Message
	// lastUserMsgNo records the last user message index already fed to the
	// supervisor, so subsequent reviews only send incremental user messages.
	lastUserMsgNo int
	// rejectCount counts consecutive rejections for the current delivery
	// (anti-dead-loop, FEATURE-456).
	rejectCount int
	// lastReason records the most recent rejection reason (for the max-retry report).
	lastReason string
}

// supervisorState is the per-Agent supervisor runtime state (FEATURE-456).
type supervisorState struct {
	ctx *supervisorContext
}

// newSupervisorState creates a fresh supervisor state for a new session.
func newSupervisorState() *supervisorState {
	return &supervisorState{ctx: &supervisorContext{messages: nil, lastUserMsgNo: 0, rejectCount: 0}}
}

// supervisorEnabled reports whether the supervisor feature is enabled.
func (a *Agent) supervisorEnabled() bool {
	return a.cfg != nil && a.cfg.LLM.Supervisor.Enabled
}

// supervisorEntryEnabled reports whether the given intervention point is enabled.
func (a *Agent) supervisorEntryEnabled(entry SupervisorEntry) bool {
	if a.cfg == nil {
		return false
	}
	switch entry {
	case SupervisorEntryA:
		return a.cfg.LLM.Supervisor.EntryObject
	case SupervisorEntryB:
		return a.cfg.LLM.Supervisor.EntryExit
	case SupervisorEntryC:
		return a.cfg.LLM.Supervisor.EntryTask
	}
	return false
}

// supervisorMaxRetries returns the configured max rejection count (default 20).
func (a *Agent) supervisorMaxRetries() int {
	if a.cfg != nil && a.cfg.LLM.Supervisor.MaxRetries > 0 {
		return a.cfg.LLM.Supervisor.MaxRetries
	}
	return 20
}

// supervisorClearContext reports whether the supervisor context should be
// cleared before each review (default false = accumulate).
func (a *Agent) supervisorClearContext() bool {
	return a.cfg != nil && a.cfg.LLM.Supervisor.ClearContext
}

// supervisorAllowedTools returns the configured tool whitelist (scheme B).
// When empty, a safe default whitelist is used.
func (a *Agent) supervisorAllowedTools() []string {
	if a.cfg != nil && len(a.cfg.LLM.Supervisor.AllowedTools) > 0 {
		return a.cfg.LLM.Supervisor.AllowedTools
	}
	return []string{"read_file", "execute_command", "memory_search", "list_files", "search_files"}
}

// supervisorToolAllowed reports whether a tool name is in the supervisor whitelist.
func (a *Agent) supervisorToolAllowed(name string) bool {
	for _, t := range a.supervisorAllowedTools() {
		if t == name {
			return true
		}
	}
	return false
}

// getSupervisorModel returns the model config to use for supervision. It reuses
// the problem-solver model resolution chain (FEATURE-456: "use the problem
// solver LLM, same fallback").
func (a *Agent) getSupervisorModel() *config.ModelConfig {
	return a.getLoopJudgeModel()
}

// buildSupervisorUserPrompt assembles the user message sent to the supervisor
// model for a review (FEATURE-456). It includes:
//   - all user messages (incremental since last review, with timestamps)
//   - the current task plan description
//   - the main LLM's final delivery report
//   - the intervention entry info (full task vs sub-task)
//   - the current task progress list
func (a *Agent) buildSupervisorUserPrompt(entry SupervisorEntry, finalReport string) string {
	var sb strings.Builder

	// 1. User messages (incremental).
	sb.WriteString("# 用户消息（含时间）\n")
	userMsgs := a.getIncrementalUserMessages()
	if userMsgs == "" {
		userMsgs = "(无新增用户消息)"
	}
	sb.WriteString(userMsgs)
	sb.WriteString("\n\n")

	// 2. Task plan description.
	sb.WriteString("# 当前任务计划\n")
	taskPlan := a.getTaskPlanPrompt()
	if taskPlan == "" {
		taskPlan = i18n.T(i18n.KeyNoActiveTaskPlan)
	}
	sb.WriteString(taskPlan)
	sb.WriteString("\n\n")

	// 3. Main LLM final delivery report.
	sb.WriteString("# 主 LLM 最终交付报告\n")
	if strings.TrimSpace(finalReport) == "" {
		finalReport = "(无)"
	}
	sb.WriteString(finalReport)
	sb.WriteString("\n\n")

	// 4. Entry info (full task vs sub-task).
	sb.WriteString("# 场景入口信息\n")
	switch entry {
	case SupervisorEntryA:
		sb.WriteString("主 LLM 调用了 attempt_completion，声明完成整个任务。请审查是否达到用户终极目标。\n")
	case SupervisorEntryB:
		sb.WriteString("主 LLM 未调用任何工具自动退出，声明任务完成。请审查是否达到用户终极目标。\n")
	case SupervisorEntryC:
		sb.WriteString("主 LLM 更新了任务进度并将一个或多个步骤标记为完成（子任务完成）。请审查该子任务是否真正完成。\n")
	}
	sb.WriteString("\n")

	// 5. Current task progress list.
	sb.WriteString("# 当前任务执行进度清单\n")
	progress := a.getTaskPlanText()
	if progress == "" {
		progress = "(无活动任务计划)"
	}
	sb.WriteString(progress)
	sb.WriteString("\n")

	return sb.String()
}

// getIncrementalUserMessages returns user messages added since the last review
// (incremental, with timestamps). It records the last fed message index so
// subsequent reviews only send the delta (FEATURE-456).
func (a *Agent) getIncrementalUserMessages() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	var sb strings.Builder
	start := a.supervisorState.ctx.lastUserMsgNo
	if start < 1 {
		start = 1 // skip system prompt (index 0)
	}
	lastFed := start
	for i := start; i < len(a.messages); i++ {
		m := a.messages[i]
		if m.Role != "user" {
			continue
		}
		content := strings.TrimSpace(m.CombineContentParts())
		if content == "" {
			content = strings.TrimSpace(m.Content)
		}
		if content == "" {
			continue
		}
		// Strip environment_details for cleaner display.
		if envIdx := strings.Index(content, "<environment_details>"); envIdx > 0 {
			content = strings.TrimSpace(content[:envIdx])
		}
		if content == "" {
			continue
		}
		ts := time.Now()
		sb.WriteString(fmt.Sprintf("[%s] %s\n", ts.Format("2006-01-02 15:04:05"), content))
		lastFed = i + 1
	}
	a.supervisorState.ctx.lastUserMsgNo = lastFed
	return sb.String()
}

// callSupervisor invokes the supervisor model with the submit_review tool and
// parses its tool-call arguments into a SupervisorReview. It follows the main
// model's tool-call mechanism (openai function calling or XML tags), mirroring
// callProblemSolver (FEATURE-456).
func (a *Agent) callSupervisor(ctx context.Context, prompt string) (*SupervisorReview, error) {
	modelCfg := a.getSupervisorModel()
	if modelCfg == nil {
		return nil, fmt.Errorf("no supervisor model available")
	}

	timeout := 60
	if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout > 0 {
		timeout = a.cfg.LLM.LoopJudgeTimeout
	} else if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout == 0 {
		timeout = 0
	}

	client := llm.NewClient(modelCfg.Endpoint, modelCfg.APIKey, modelCfg.Model, 0.3, 8192, timeout)
	if client != nil {
		defer client.Close()
	}
	if modelCfg.Temperature != nil {
		client.SetTemperature(*modelCfg.Temperature)
	}

	xmlMode := a.isXMLMode()
	// Build the tool set: whitelisted low-risk tools (scheme B) + submit_review.
	supervisorTools := a.buildSupervisorTools()
	var tools []llm.Tool
	systemPrompt := i18n.T(i18n.KeySupervisorSystemPrompt)
	if xmlMode {
		systemPrompt += "\n\n" + BuildToolUsagePrompt(ToolCallModeXML, supervisorTools, string(i18n.GetLang()), true)
	} else {
		tools = supervisorTools
	}

	// Use the accumulated supervisor context (session-bound, independent) plus
	// the current review prompt. When clearContext is enabled, the context was
	// already reset by the caller.
	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	baseLen := 1 // system prompt
	if a.supervisorState != nil && a.supervisorState.ctx != nil && len(a.supervisorState.ctx.messages) > 0 {
		messages = append(messages, a.supervisorState.ctx.messages...)
		baseLen += len(a.supervisorState.ctx.messages)
	}
	messages = append(messages, llm.Message{Role: "user", Content: prompt})

	ctxTimeout := timeout + 5
	if timeout <= 0 {
		ctxTimeout = 0
	}
	var cctx context.Context
	var cancel context.CancelFunc
	if ctxTimeout > 0 {
		cctx, cancel = context.WithTimeout(ctx, time.Duration(ctxTimeout)*time.Second)
	} else {
		cctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Multi-round tool-call loop: the supervisor may call whitelisted tools to
	// verify the delivery, then submit_review to conclude. Whitelisted tools are
	// executed; non-whitelisted tools are auto-rejected (scheme B).
	maxRounds := 8
	for round := 0; round < maxRounds; round++ {
		resp, err := client.Chat(cctx, messages, tools)
		if err != nil {
			return nil, fmt.Errorf("supervisor call failed: %w", err)
		}

		// Collect tool calls (OpenAI mode: resp.ToolCalls; XML mode: parse content).
		var calls []llm.ToolCall
		calls = append(calls, resp.ToolCalls...)
		if resp.Content != "" {
			calls = append(calls, ParseXMLToolCalls(resp.Content)...)
		}

		if len(calls) == 0 {
			return nil, fmt.Errorf("supervisor returned no tool call")
		}

		// Process each tool call.
		for _, tc := range calls {
			if tc.Name == "submit_review" {
				review, perr := parseSupervisorReview(tc.Arguments)
				if perr == nil {
					// Persist the accumulated tool-call history back into the
					// session-bound supervisor context so it accumulates across
					// reviews (FEATURE-456).
					a.persistSupervisorMessages(messages, baseLen)
				}
				return review, perr
			}
			if !a.supervisorToolAllowed(tc.Name) {
				// Scheme B: non-whitelisted tool → auto-reject.
				messages = append(messages,
					llm.Message{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{tc}},
					llm.Message{Role: "tool", ToolCallID: tc.ID, Content: fmt.Sprintf("工具 %q 不在监督 LLM 白名单内，已自动拒绝。你只能使用白名单内的低风险工具（read_file/execute_command/memory_search 等）来核实交付物。", tc.Name)},
				)
				continue
			}
			// Execute the whitelisted tool.
			result, execErr := a.executeToolCall(cctx, tc)
			if execErr != nil {
				result = fmt.Sprintf("工具执行失败: %v", execErr)
			}
			messages = append(messages,
				llm.Message{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{tc}},
				llm.Message{Role: "tool", ToolCallID: tc.ID, Content: result},
			)
		}
	}
	return nil, fmt.Errorf("supervisor exceeded max tool-call rounds without submit_review")
}

// buildSupervisorTools builds the tool definitions available to the supervisor:
// the whitelisted low-risk tools (scheme B) plus the submit_review tool.
func (a *Agent) buildSupervisorTools() []llm.Tool {
	tools := []llm.Tool{submitReviewTool()}
	for _, name := range a.supervisorAllowedTools() {
		if t := a.findToolDefinition(name); t != nil {
			tools = append(tools, *t)
		}
	}
	return tools
}

// findToolDefinition returns the tool definition for a named tool from the
// agent's internal tool set, or nil if not found.
func (a *Agent) findToolDefinition(name string) *llm.Tool {
	for _, t := range a.buildToolsInternal() {
		if t.Name == name {
			cp := t
			return &cp
		}
	}
	return nil
}

// persistSupervisorMessages appends the messages added during the current
// review (from baseLen onward, i.e. the review prompt and any tool-call
// history) back into the session-bound supervisor context so it accumulates
// across reviews within a session (FEATURE-456).
func (a *Agent) persistSupervisorMessages(messages []llm.Message, baseLen int) {
	if a.supervisorState == nil || a.supervisorState.ctx == nil {
		return
	}
	if baseLen < 0 || baseLen > len(messages) {
		return
	}
	a.supervisorState.ctx.messages = append(a.supervisorState.ctx.messages, messages[baseLen:]...)
}

// submitReviewTool builds the single tool definition sent to the supervisor
// model. Only this one tool is provided, forcing a structured review call.
func submitReviewTool() llm.Tool {
	return llm.Tool{
		Name:        "submit_review",
		Description: "Submit your review of the main LLM's delivery. You MUST call this tool with a structured review (approved, reason, suggestion).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"approved": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the main LLM's delivery meets the user's ultimate goal. true = approve (pass to user), false = reject (send back for rework).",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Why you approve or reject the delivery. Be specific and reference the user's goal.",
				},
				"suggestion": map[string]interface{}{
					"type":        "string",
					"description": "Concrete, actionable advice. When rejecting, tell the main LLM exactly what to fix. When approving, give the user a summary of what was verified.",
				},
			},
			"required": []string{"approved", "reason", "suggestion"},
		},
	}
}

// parseSupervisorReview unmarshals submit_review arguments (JSON) into a
// SupervisorReview. It is lenient about extra fields.
func parseSupervisorReview(args string) (*SupervisorReview, error) {
	var review SupervisorReview
	if err := json.Unmarshal([]byte(args), &review); err != nil {
		s := strings.TrimSpace(args)
		if idx := strings.Index(s, "{"); idx >= 0 {
			s = s[idx:]
		}
		if idx := strings.LastIndex(s, "}"); idx >= 0 {
			s = s[:idx+1]
		}
		if err2 := json.Unmarshal([]byte(s), &review); err2 != nil {
			return nil, fmt.Errorf("cannot parse submit_review args: %v", err)
		}
	}
	return &review, nil
}

// runSupervisorReview is the synchronous entry point that performs a review at
// one of the three intervention points (FEATURE-456). It returns:
//   - approved: true when the delivery passes (or when supervision is disabled /
//     unavailable / max retries exceeded → force pass to user).
//   - feedback: non-empty when rejected — the reason+suggestion to append as a
//     user message to the main LLM context for rework.
//   - report: the review text to show the user (reason+suggestion on approval).
func (a *Agent) runSupervisorReview(ctx context.Context, entry SupervisorEntry, finalReport string) (approved bool, feedback string, report string) {
	// Feature disabled → pass through.
	if !a.supervisorEnabled() {
		return true, "", ""
	}
	// Entry switch off → pass through.
	if !a.supervisorEntryEnabled(entry) {
		return true, "", ""
	}

	// Anti-dead-loop: if rejections exceed maxRetries, force pass to user.
	if a.supervisorState.ctx.rejectCount >= a.supervisorMaxRetries() {
		msg := fmt.Sprintf("监督 LLM 已打回 %d 次，最后一次理由：%s。已强制放行，请人工判断。",
			a.supervisorState.ctx.rejectCount, a.supervisorState.ctx.lastReason)
		log.Warn("runSupervisorReview: max retries (%d) exceeded, forcing pass to user", a.supervisorState.ctx.rejectCount)
		return true, "", msg
	}

	// Clear context if configured.
	if a.supervisorClearContext() {
		a.supervisorState.ctx.messages = nil
	}

	prompt := a.buildSupervisorUserPrompt(entry, finalReport)
	review, err := a.callSupervisor(ctx, prompt)
	if err != nil {
		// Supervisor unavailable → degrade to pass-through (safe default).
		log.Warn("runSupervisorReview: supervisor call failed, passing through: %v", err)
		return true, "", ""
	}

	// Record the review in the supervisor's own context (accumulate).
	a.supervisorState.ctx.messages = append(a.supervisorState.ctx.messages,
		llm.Message{Role: "user", Content: prompt},
		llm.Message{Role: "assistant", Content: formatReviewAsText(review)},
	)

	if review.Approved {
		// Approved: reset reject count, report reason+suggestion to user, and
		// persist the review as a user message into permanent memory (FEATURE-456).
		a.supervisorState.ctx.rejectCount = 0
		report = formatReviewAsText(review)
		if a.memoryEnabled && a.memoryManager != nil {
			if err := a.memoryManager.AddMessage(a.name, report, time.Now()); err != nil {
				log.Warn("runSupervisorReview: failed to save review to memory: %v", err)
			}
		}
		return true, "", report
	}

	// Rejected: increment reject count, build feedback for the main LLM.
	a.supervisorState.ctx.rejectCount++
	a.supervisorState.ctx.lastReason = review.Reason
	feedback = formatReviewFeedback(review)
	return false, feedback, ""
}

// formatReviewAsText renders a review for user display (approval path).
func formatReviewAsText(r *SupervisorReview) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【监督 LLM 审查】\n")
	if r.Approved {
		sb.WriteString("结论：放行 ✅\n")
	} else {
		sb.WriteString("结论：打回 ❌\n")
	}
	if r.Reason != "" {
		sb.WriteString("理由：" + r.Reason + "\n")
	}
	if r.Suggestion != "" {
		sb.WriteString("建议：" + r.Suggestion + "\n")
	}
	return sb.String()
}

// formatReviewFeedback renders the rejection feedback as a user message to be
// appended to the main LLM context for rework (FEATURE-456).
func formatReviewFeedback(r *SupervisorReview) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【监督 LLM 打回，请重做】\n")
	sb.WriteString("你的交付未达到用户终极目标，监督 LLM 已打回。请根据以下理由和建议重新完成：\n")
	if r.Reason != "" {
		sb.WriteString("理由：" + r.Reason + "\n")
	}
	if r.Suggestion != "" {
		sb.WriteString("建议：" + r.Suggestion + "\n")
	}
	sb.WriteString("请重新检查并完成遗漏的部分，然后再次交付。")
	return sb.String()
}

// emitSupervisorReport sends the supervisor review report to the user via the
// dedicated supervisor channel (FEATURE-456). Falls back to stderr when no
// stream callback is active.
func (a *Agent) emitSupervisorReport(report string) {
	if report == "" {
		return
	}
	a.mu.Lock()
	cb := a.streamCb
	a.mu.Unlock()
	if cb != nil {
		cb(InfoEvent(ChannelSupervisor, report))
	} else {
		a.defaultIO().ErrPrintf("%s\n", report)
	}
}
