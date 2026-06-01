// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-06-01
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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/memory"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/shell"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/subagent"
	"github.com/idirect3d/co-shell/taskplan"
)

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.Store, rules string) *Agent {
	systemPrompt := buildSystemPrompt(rules)

	return &Agent{
		llmClient:       llmClient,
		mcpMgr:          mcpMgr,
		store:           s,
		memoryManager:   memory.NewManager(s),
		systemPrompt:    systemPrompt,
		maxIterations:   config.DefaultConfig().LLM.MaxIterations,
		rules:           rules,
		subAgentMgr:     subagent.NewManager(),
		taskPlanMgr:     taskplan.NewManager(s),
		name:            "co-shell",
		modelManager:    config.GetDefaultModelManager(),
		toolCallModeMgr: NewToolCallModeManager(),
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

func (a *Agent) Messages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

func (a *Agent) SetName(name string) {
	if name == "" {
		name = "co-shell"
	}
	a.name = name
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Said() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	return i18n.TF(i18n.KeyAgentSaid, now, a.name)
}

func (a *Agent) SetShowLlmThinking(show bool)   { a.showLlmThinking = show }
func (a *Agent) SetShowLlmContent(show bool)    { a.showLlmContent = show }
func (a *Agent) SetShowTool(show bool)          { a.showTool = show }
func (a *Agent) SetShowToolInput(show bool)     { a.showToolInput = show }
func (a *Agent) SetShowToolOutput(show bool)    { a.showToolOutput = show }
func (a *Agent) SetShowCommand(show bool)       { a.showCommand = show }
func (a *Agent) SetShowCommandOutput(show bool) { a.showCommandOutput = show }

func (a *Agent) SetMaxIterations(n int) {
	if n <= 0 {
		a.maxIterations = -1
	} else {
		a.maxIterations = n
	}
}

func (a *Agent) SetToolMode(toolName string, mode string) {
	if a.toolModes == nil {
		a.toolModes = make(map[string]string)
	}
	if toolName == "" {
		a.toolModes = map[string]string{"default": mode}
	} else {
		a.toolModes[toolName] = mode
	}
}

func DefaultToolModes() map[string]string {
	return map[string]string{
		"execute_command":            "confirm",
		"read_file":                  "confirm",
		"write_to_file":              "confirm",
		"replace_in_file":            "confirm",
		"search_files":               "confirm",
		"list_files":                 "auto",
		"list_code_definition_names": "auto",
		"add_images":                 "auto",
		"remove_images":              "auto",
		"clear_images":               "auto",
		"update_settings":            "confirm",
		"list_settings":              "auto",
		"ask_followup_question":      "auto",
		"adjust_context_start":       "auto",
		"launch_sub_agent":           "confirm",
		"schedule_task":              "confirm",
		"create_task_plan":           "auto",
		"update_task_step":           "auto",
		"insert_task_steps":          "auto",
		"remove_task_steps":          "auto",
		"view_task_plan":             "auto",
		"get_memory_slice":           "auto",
		"memory_search":              "auto",
		"delete_memory":              "confirm",
		"shell_send":                 "confirm",
		"shell_get_output":           "auto",
		"shell_window_content":       "auto",
		"shell_reset":                "auto",
		"attempt_completion":         "auto",
	}
}

func (a *Agent) SyncToolModes(cfg *config.Config) {
	modes := DefaultToolModes()
	for k, v := range cfg.LLM.ToolModes {
		modes[k] = v
	}
	a.toolModes = modes
}

func (a *Agent) SetMemoryEnabled(enabled bool)   { a.memoryEnabled = enabled }
func (a *Agent) SetEmojiEnabled(enabled bool)    { a.emojiEnabled = enabled }
func (a *Agent) SetToolCallEnabled(enabled bool) { a.toolCallEnabled = enabled }

func (a *Agent) SetToolCallMode(mode string) {
	if a.toolCallModeMgr == nil {
		a.toolCallModeMgr = NewToolCallModeManager()
	}
	a.toolCallModeMgr.SetCurrentByString(mode)
	a.rebuildSystemPrompt()
	log.Info("Tool call mode set to %s", mode)
}

func (a *Agent) ToolCallMode() string {
	if a.toolCallModeMgr == nil {
		return string(ToolCallModeOpenAI)
	}
	mode := a.toolCallModeMgr.Current()
	if mode == nil {
		return string(ToolCallModeOpenAI)
	}
	return string(mode.Type)
}

func (a *Agent) SetStore(s *store.Store) { a.store = s }

func (a *Agent) RestoreSession() bool {
	if a.store == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	data, found, err := a.store.LoadSession()
	if err != nil || !found {
		return false
	}
	var session store.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}
	if len(session.Messages) == 0 {
		return false
	}
	var messages []llm.Message
	if err := json.Unmarshal(session.Messages, &messages); err != nil {
		return false
	}
	a.messages = messages
	return true
}

func (a *Agent) PersistSession() error {
	if a.store == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(a.messages)
	if err != nil {
		return fmt.Errorf("cannot serialize messages: %w", err)
	}
	return a.store.SaveSession(data)
}

func (a *Agent) MessagePointer() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messagePointer
}

func (a *Agent) SetPlanEnabled(enabled bool) {
	a.planEnabled = enabled
}

func (a *Agent) SetSubAgentEnabled(enabled bool) {
	a.subAgentEnabled = enabled
}

// SetShellEnabled enables or disables shell session mode.
// When enabled, it auto-starts a shell session.
// When disabled, it auto-stops any active shell session.
func (a *Agent) SetShellEnabled(enabled bool) {
	a.mu.Lock()
	a.shellEnabled = enabled
	a.mu.Unlock()

	if enabled {
		// Auto-start shell session
		if a.shellSession == nil || !a.shellSession.IsRunning() {
			sess := &shell.Session{}
			if a.cfg != nil && a.cfg.LLM.ShellVTRows > 0 && a.cfg.LLM.ShellVTCols > 0 {
				sess.SetVT(a.cfg.LLM.ShellVTRows, a.cfg.LLM.ShellVTCols)
			}
			if _, err := sess.Start(); err != nil {
				log.Warn("Failed to auto-start shell session: %v", err)
				return
			}
			a.mu.Lock()
			a.shellSession = sess
			a.mu.Unlock()
			log.Info("Shell session auto-started (shell-session-enabled=on)")
		}
	} else {
		// Auto-stop shell session
		a.CloseShellSession()
	}
}

// CloseShellSession closes the active shell session if one exists.
func (a *Agent) CloseShellSession() {
	a.mu.Lock()
	sess := a.shellSession
	a.shellSession = nil
	a.mu.Unlock()

	if sess != nil {
		sess.Close()
		log.Info("Shell session closed (shell-session-enabled=off)")
	}
}

// EnsureShellSession starts a shell session if one is not already running.
// This is called on startup when shell-session-enabled=on.
func (a *Agent) EnsureShellSession() {
	if !a.shellEnabled {
		return
	}
	a.mu.Lock()
	hasSession := a.shellSession != nil && a.shellSession.IsRunning()
	a.mu.Unlock()
	if !hasSession {
		a.SetShellEnabled(true)
	}
}

func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
	a.rebuildSystemPrompt()
}

func (a *Agent) SetLLMClient(client llm.Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.llmClient != nil {
		a.llmClient.Close()
	}
	a.llmClient = client
	log.Info("LLM client replaced at runtime")
}

func (a *Agent) GetLLMClient() llm.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.llmClient
}

func (a *Agent) rebuildSystemPrompt() {
	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	userName := ""
	channel := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
		userName = a.cfg.LLM.UserName
		channel = a.cfg.LLM.Channel
	}

	toolUsageText := ""
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Mode()
		if mode == ToolCallModeXML {
			tools := a.buildToolsInternal()
			lang := string(i18n.GetLang())
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.lastUserInput

	a.systemPrompt = buildSystemPromptWithMode(a.rules, a.resultMode, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.messages) > 0 {
		a.messages[0] = llm.Message{Role: "system", Content: a.systemPrompt}
	} else {
		a.messages = []llm.Message{
			{Role: "system", Content: a.systemPrompt},
		}
	}
}

func (a *Agent) SetWorkspacePath(path string)            { a.workspacePath = path }
func (a *Agent) SetImagePaths(paths []string)            { a.imagePaths = paths }
func (a *Agent) SetModelManager(mm *config.ModelManager) { a.modelManager = mm }

func (a *Agent) selectModelForCall() *config.ModelConfig {
	if a.modelManager == nil {
		return nil
	}
	visionRequired := len(a.imagePaths) > 0
	return a.modelManager.GetActiveModel(visionRequired)
}

func (a *Agent) switchToModel(modelCfg *config.ModelConfig) {
	if modelCfg == nil || a.cfg == nil {
		return
	}

	temperature := a.cfg.LLM.Temperature
	if modelCfg.Temperature != nil {
		temperature = *modelCfg.Temperature
	}
	maxTokens := a.cfg.LLM.MaxTokens
	if modelCfg.MaxTokens != nil {
		maxTokens = *modelCfg.MaxTokens
	}
	thinkingEnabled := a.cfg.LLM.ThinkingEnabled
	if modelCfg.ThinkingEnabled != nil {
		thinkingEnabled = *modelCfg.ThinkingEnabled
	}
	reasoningEffort := a.cfg.LLM.ReasoningEffort
	if modelCfg.ReasoningEffort != nil {
		reasoningEffort = *modelCfg.ReasoningEffort
	}
	topP := a.cfg.LLM.TopP
	if modelCfg.TopP != nil {
		topP = *modelCfg.TopP
	}
	topK := a.cfg.LLM.TopK
	if modelCfg.TopK != nil {
		topK = *modelCfg.TopK
	}
	repetitionPenalty := a.cfg.LLM.RepetitionPenalty
	if modelCfg.RepetitionPenalty != nil {
		repetitionPenalty = *modelCfg.RepetitionPenalty
	}

	newClient := llm.NewClient(
		modelCfg.Endpoint, modelCfg.APIKey, modelCfg.Model,
		temperature, maxTokens, a.cfg.LLM.LLMTimeout,
	)
	newClient.SetThinkingEnabled(thinkingEnabled)
	newClient.SetReasoningEffort(reasoningEffort)
	newClient.SetTopP(topP)
	newClient.SetTopK(topK)
	newClient.SetRepetitionPenalty(repetitionPenalty)
	newClient.SetTokenUsage(a.cfg.LLM.TokenUsage)

	mergedAdditions := make(map[string]string)
	if len(a.cfg.LLM.BodyAdditions) > 0 {
		for k, v := range a.cfg.LLM.BodyAdditions {
			mergedAdditions[k] = v
		}
	}
	if len(modelCfg.CustomParams) > 0 {
		for k, v := range modelCfg.CustomParams {
			if strVal, ok := v.(string); ok && strVal == "None" {
				delete(mergedAdditions, k)
				continue
			}
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				log.Warn("Failed to marshal CustomParam %s: %v", k, err)
				continue
			}
			mergedAdditions[k] = string(jsonBytes)
		}
	}
	if len(mergedAdditions) > 0 {
		newClient.SetBodyAdditions(mergedAdditions)
	}

	a.SetLLMClient(newClient)
	log.Info("Switched to model: %s (endpoint=%s, vision=%v)", modelCfg.Model, modelCfg.Endpoint, modelCfg.Capabilities.Vision)
}

func (a *Agent) getTaskPlanText() string {
	if a.taskPlanMgr == nil {
		return ""
	}
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil || plan == nil {
		return ""
	}
	if !a.taskPlanMgr.HasUnfinished() {
		return ""
	}
	return taskplan.FormatPlan(plan)
}

func (a *Agent) formatXMLToolResult(toolName, toolArgs, toolResult string) string {
	template := i18n.T(i18n.KeyXMLToolResultTemplate)
	result := strings.ReplaceAll(template, "{TOOL_CALL}", toolName)
	result = strings.ReplaceAll(result, "{TOOL_CALL_PARAMETERS}", toolArgs)
	result = strings.ReplaceAll(result, "{TOOL_RESULT}", toolResult)
	result = strings.ReplaceAll(result, "{TASK_TRACKING}", a.getTaskPlanPrompt())
	result = strings.ReplaceAll(result, "{CURRENT_TIME}", time.Now().Format("2006-01-02 15:04:05 Monday"))
	return result
}

func (a *Agent) formatUserMessage(instruction string) string {
	template := i18n.T(i18n.KeyUserMessageTemplate)
	result := strings.ReplaceAll(template, "{INSTRUCTION}", instruction)
	result = strings.ReplaceAll(result, "{TASK_TRACKING}", a.getTaskPlanPrompt())
	result = strings.ReplaceAll(result, "{CURRENT_TIME}", time.Now().Format("2006-01-02 15:04:05 Monday"))
	return result
}

func (a *Agent) getTaskPlanPrompt() string {
	if a.taskPlanMgr == nil {
		return ""
	}
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil || plan == nil {
		return ""
	}
	if a.taskPlanMgr.HasUnfinished() {
		planText := taskplan.FormatPlan(plan)
		template := i18n.T(i18n.KeyToolResultWithPlan)
		return strings.ReplaceAll(template, "{TASK_PLAN}", planText)
	}
	return i18n.T(i18n.KeyToolResultNoPlan)
}

func (a *Agent) TaskPlanManager() *taskplan.Manager  { return a.taskPlanMgr }
func (a *Agent) SetScheduler(s *scheduler.Scheduler) { a.scheduler = s }
func (a *Agent) Scheduler() *scheduler.Scheduler     { return a.scheduler }

func (a *Agent) SetResultMode(mode config.ResultMode) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.resultMode = mode
	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	userName := ""
	channel := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
		userName = a.cfg.LLM.UserName
		channel = a.cfg.LLM.Channel
	}

	toolUsageText := ""
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Mode()
		if mode == ToolCallModeXML {
			tools := a.buildToolsInternal()
			lang := string(i18n.GetLang())
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.lastUserInput

	a.systemPrompt = buildSystemPromptWithMode(a.rules, mode, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)

	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Result mode set to %s, system prompt rebuilt", config.ResultModeString(mode))
}

func (a *Agent) getToolTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.ToolTimeout > 0 {
		return time.Duration(a.cfg.LLM.ToolTimeout) * time.Second
	}
	return 0
}

func (a *Agent) getCommandTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.CommandTimeout > 0 {
		return time.Duration(a.cfg.LLM.CommandTimeout) * time.Second
	}
	return 0
}

func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Agent history reset")
}

func (a *Agent) GetHistory() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messages
}

func (a *Agent) SetHistory(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = messages
}

func (a *Agent) GetMessages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

func (a *Agent) adjustMessagePointer() {
	for a.messagePointer > 0 && a.messages[a.messagePointer].Role == "tool" {
		a.messagePointer--
	}
}

func (a *Agent) removeLastAssistantWithToolCalls() string {
	lastAssistantIdx := -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "assistant" && len(a.messages[i].ToolCalls) > 0 {
			lastAssistantIdx = i
			break
		}
	}
	if lastAssistantIdx < 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- 已移除的消息 (从索引 %d 开始) ---\n", lastAssistantIdx))
	for i := lastAssistantIdx; i < len(a.messages); i++ {
		msg := a.messages[i]
		sb.WriteString(fmt.Sprintf("[%d] role=%s", i, msg.Role))
		if msg.Content != "" {
			sb.WriteString(fmt.Sprintf(", content=%q", msg.Content))
		}
		if len(msg.ToolCalls) > 0 {
			sb.WriteString(fmt.Sprintf(", tool_calls=%d", len(msg.ToolCalls)))
			for j, tc := range msg.ToolCalls {
				sb.WriteString(fmt.Sprintf("\n    tool_call[%d]: name=%q, id=%q, args=%q", j, tc.Name, tc.ID, tc.Arguments))
			}
		}
		if msg.ToolCallID != "" {
			sb.WriteString(fmt.Sprintf(", tool_call_id=%q", msg.ToolCallID))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("--- 结束 ---")
	a.messages = a.messages[:lastAssistantIdx]
	return sb.String()
}
