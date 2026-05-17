// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-05-13
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
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/subagent"
	"github.com/idirect3d/co-shell/taskplan"
)

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.Store, rules string) *Agent {
	systemPrompt := buildSystemPrompt(rules)

	return &Agent{
		llmClient:     llmClient,
		mcpMgr:        mcpMgr,
		store:         s,
		memoryManager: memory.NewManager(s),
		systemPrompt:  systemPrompt,
		maxIterations: config.DefaultConfig().LLM.MaxIterations,
		rules:         rules,
		subAgentMgr:   subagent.NewManager(),
		taskPlanMgr:   taskplan.NewManager(s),
		name:          "co-shell",
		modelManager:  config.GetDefaultModelManager(),
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// Messages returns a copy of the current conversation message queue.
func (a *Agent) Messages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

// SetName sets the agent name for identification.
// The name is used in log messages, sub-agent workspace naming, and output.
func (a *Agent) SetName(name string) {
	if name == "" {
		name = "co-shell"
	}
	a.name = name
}

// Name returns the agent name.
func (a *Agent) Name() string {
	return a.name
}

// Said returns a formatted string with timestamp and agent name.
// Format: "2026-12-31 15:30:10 co-shell said:"
func (a *Agent) Said() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	return i18n.TF(i18n.KeyAgentSaid, now, a.name)
}

// SetShowLlmThinking sets whether to display LLM thinking content.
func (a *Agent) SetShowLlmThinking(show bool) {
	a.showLlmThinking = show
}

// SetShowLlmContent sets whether to display LLM main content.
func (a *Agent) SetShowLlmContent(show bool) {
	a.showLlmContent = show
}

// SetShowTool sets whether to display tool call name.
func (a *Agent) SetShowTool(show bool) {
	a.showTool = show
}

// SetShowToolInput sets whether to display tool call input parameters.
func (a *Agent) SetShowToolInput(show bool) {
	a.showToolInput = show
}

// SetShowToolOutput sets whether to display tool call return data.
func (a *Agent) SetShowToolOutput(show bool) {
	a.showToolOutput = show
}

// SetShowCommand sets whether to display commands before execution.
func (a *Agent) SetShowCommand(show bool) {
	a.showCommand = show
}

// SetShowCommandOutput sets whether to display command return data.
func (a *Agent) SetShowCommandOutput(show bool) {
	a.showCommandOutput = show
}

// SetMaxIterations sets the maximum number of LLM call iterations.
// n <= 0 means unlimited; n > 0 sets a specific limit.
func (a *Agent) SetMaxIterations(n int) {
	if n <= 0 {
		a.maxIterations = -1 // unlimited
	} else {
		a.maxIterations = n
	}
}

// SetConfirmCommand sets whether to prompt the user for confirmation before executing commands.
func (a *Agent) SetConfirmCommand(confirm bool) {
	a.confirmCommand = confirm
}

// SetMemoryEnabled sets whether persistent memory tools are enabled.
func (a *Agent) SetMemoryEnabled(enabled bool) {
	a.memoryEnabled = enabled
}

// SetEmojiEnabled sets whether emoji prefixes are enabled for output.
func (a *Agent) SetEmojiEnabled(enabled bool) {
	a.emojiEnabled = enabled
}

// SetToolCallEnabled sets whether tool calling is enabled.
func (a *Agent) SetToolCallEnabled(enabled bool) {
	a.toolCallEnabled = enabled
}

// SetStore sets the persistent store for session persistence.
func (a *Agent) SetStore(s *store.Store) {
	a.store = s
}

// RestoreSession restores a previous conversation session from persistent storage.
// If a session exists, it replaces the current messages with the restored ones.
// Returns true if a session was restored, false if no session was found.
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

	// Parse the session data to extract messages
	var session store.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}

	// Only restore if the session has messages
	if len(session.Messages) == 0 {
		return false
	}

	// Parse messages from JSON
	var messages []llm.Message
	if err := json.Unmarshal(session.Messages, &messages); err != nil {
		return false
	}

	// Restore the messages
	a.messages = messages
	return true
}

// PersistSession persists the current conversation session to storage.
// This should be called after each user request is completed.
func (a *Agent) PersistSession() error {
	if a.store == nil {
		return nil // silently skip if no store
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	// Serialize messages to JSON
	data, err := json.Marshal(a.messages)
	if err != nil {
		return fmt.Errorf("cannot serialize messages: %w", err)
	}

	return a.store.SaveSession(data)
}

// MessagePointer returns the current message pointer index.
// The pointer marks the starting position for sending to LLM.
// Messages before this index are ignored when building context.
func (a *Agent) MessagePointer() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messagePointer
}

// SetPlanEnabled sets whether task plan tools are enabled.
func (a *Agent) SetPlanEnabled(enabled bool) {
	a.planEnabled = enabled
}

// SetSubAgentEnabled sets whether sub-agent tools are enabled.
func (a *Agent) SetSubAgentEnabled(enabled bool) {
	a.subAgentEnabled = enabled
}

// SetConfig sets the configuration for timeout settings and agent identity.

// It also rebuilds the system prompt with identity information.
func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
	// Rebuild system prompt with identity info from config
	a.rebuildSystemPrompt()
}

// SetLLMClient replaces the LLM client at runtime.
// This is used when settings like api-key, endpoint, model, temperature,
// max-tokens, or vision are changed via .set command without restarting.
func (a *Agent) SetLLMClient(client llm.Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Close old client if it has a Close method
	if a.llmClient != nil {
		a.llmClient.Close()
	}
	a.llmClient = client
	log.Info("LLM client replaced at runtime")
}

// GetLLMClient returns the current LLM client.
func (a *Agent) GetLLMClient() llm.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.llmClient
}

// rebuildSystemPrompt rebuilds the system prompt with current config identity info.
// It preserves the conversation history (only replaces the system message at index 0).
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
	a.systemPrompt = buildSystemPromptWithMode(a.rules, a.resultMode, agentName, agentDesc, agentPrinciples, userName, channel)
	// Preserve conversation history: only replace the system message at index 0
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

// SetWorkspacePath sets the workspace root path for loading external config files
// such as capabilities.md and rules.md.
func (a *Agent) SetWorkspacePath(path string) {
	a.workspacePath = path
}

// SetImagePaths sets the paths to image files for multimodal input.
// These images will be included in the next user message.
func (a *Agent) SetImagePaths(paths []string) {
	a.imagePaths = paths
}

// SetModelManager sets the model manager for multi-model switching.
func (a *Agent) SetModelManager(mm *config.ModelManager) {
	a.modelManager = mm
}

// selectModelForCall selects the appropriate model based on current context.
// If imagePaths is non-empty, it selects a model with Vision capability.
// Otherwise, it selects a model with ToolCall capability.
// Returns the model config, or nil if no suitable model is found.
func (a *Agent) selectModelForCall() *config.ModelConfig {
	if a.modelManager == nil {
		return nil
	}

	visionRequired := len(a.imagePaths) > 0
	return a.modelManager.GetActiveModel(visionRequired)
}

// switchToModel creates a new LLM client for the given model config and replaces the current one.
// It uses model-level parameters from ModelConfig when set, falling back to global cfg.LLM settings.
// Model-specific CustomParams are also merged into the request body.
func (a *Agent) switchToModel(modelCfg *config.ModelConfig) {
	if modelCfg == nil || a.cfg == nil {
		return
	}

	// Resolve parameters: model-level takes precedence, fall back to global cfg.LLM
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

	// Create a new LLM client for the selected model with resolved parameters
	newClient := llm.NewClient(
		modelCfg.Endpoint,
		modelCfg.APIKey,
		modelCfg.Model,
		temperature,
		maxTokens,
		a.cfg.LLM.LLMTimeout,
	)

	// Apply resolved LLM settings
	newClient.SetThinkingEnabled(thinkingEnabled)
	newClient.SetReasoningEffort(reasoningEffort)
	newClient.SetTopP(topP)
	newClient.SetTopK(topK)
	newClient.SetRepetitionPenalty(repetitionPenalty)
	newClient.SetTokenUsage(a.cfg.LLM.TokenUsage)

	// Merge model-specific CustomParams with global BodyAdditions.
	// CustomParams take precedence over BodyAdditions for the same key.
	// A value of "None" (string) means the parameter should NOT be sent.
	mergedAdditions := make(map[string]string)

	// Start with global BodyAdditions
	if len(a.cfg.LLM.BodyAdditions) > 0 {
		for k, v := range a.cfg.LLM.BodyAdditions {
			mergedAdditions[k] = v
		}
	}

	// Apply model-specific CustomParams (override or add)
	if len(modelCfg.CustomParams) > 0 {
		for k, v := range modelCfg.CustomParams {
			// "None" means remove this parameter entirely
			if strVal, ok := v.(string); ok && strVal == "None" {
				delete(mergedAdditions, k)
				continue
			}
			// Serialize the value to JSON string for bodyAdditions format
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

	// Replace the current LLM client
	a.SetLLMClient(newClient)

	log.Info("Switched to model: %s (endpoint=%s, vision=%v, custom_params=%d)",
		modelCfg.Model, modelCfg.Endpoint, modelCfg.Capabilities.Vision, len(modelCfg.CustomParams))
}

// TaskPlanManager returns the task plan manager.
func (a *Agent) TaskPlanManager() *taskplan.Manager {
	return a.taskPlanMgr
}

// SetScheduler sets the scheduler for this agent.
func (a *Agent) SetScheduler(s *scheduler.Scheduler) {
	a.scheduler = s
}

// Scheduler returns the scheduler instance.
func (a *Agent) Scheduler() *scheduler.Scheduler {
	return a.scheduler
}

// SetResultMode sets the result processing mode and rebuilds the system prompt.
// This resets the conversation history to apply the new mode.
func (a *Agent) SetResultMode(mode config.ResultMode) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.resultMode = mode
	// Rebuild system prompt with current identity info from config
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
	a.systemPrompt = buildSystemPromptWithMode(a.rules, mode, agentName, agentDesc, agentPrinciples, userName, channel)
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Result mode set to %s, system prompt rebuilt", config.ResultModeString(mode))
}

// getToolTimeout returns the tool call timeout duration.
// Returns 0 (no timeout) if not configured.
func (a *Agent) getToolTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.ToolTimeout > 0 {
		return time.Duration(a.cfg.LLM.ToolTimeout) * time.Second
	}
	return 0
}

// getCommandTimeout returns the system command execution timeout duration.
// Returns 0 (no timeout) if not configured.
func (a *Agent) getCommandTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.CommandTimeout > 0 {
		return time.Duration(a.cfg.LLM.CommandTimeout) * time.Second
	}
	return 0
}

// Reset clears the conversation history but keeps the system prompt.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Agent history reset")
}

// GetHistory returns the current conversation history.
func (a *Agent) GetHistory() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messages
}

// SetHistory restores a previous conversation history.
func (a *Agent) SetHistory(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = messages
}

// GetMessages returns the current messages slice (thread-safe).
func (a *Agent) GetMessages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

// adjustMessagePointer moves the messagePointer back past any tool messages
// to ensure the LLM sees a clean context starting from a non-tool message.
// This is called after setting messagePointer to a new position (e.g., after
// creating/updating a checklist). If the pointer position is preceded by tool
// messages, the pointer is moved further back to the first non-tool message.
// Caller must hold a.mu lock.
func (a *Agent) adjustMessagePointer() {
	for a.messagePointer > 0 && a.messages[a.messagePointer].Role == "tool" {
		a.messagePointer--
	}
}

// removeLastAssistantWithToolCalls finds the last assistant message that has
// tool_calls in a.messages, removes it and all subsequent messages (tool results,
// etc.), and returns a string representation of the removed messages for error
// feedback. If no such assistant message is found, returns empty string and does
// nothing.
// Caller must hold a.mu lock.
func (a *Agent) removeLastAssistantWithToolCalls() string {
	// Find the last assistant message with tool_calls from the end
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

	// Collect the removed messages as a string for error feedback
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
				sb.WriteString(fmt.Sprintf("\n    tool_call[%d]: name=%q, id=%q, args=%q",
					j, tc.Name, tc.ID, tc.Arguments))
			}
		}
		if msg.ToolCallID != "" {
			sb.WriteString(fmt.Sprintf(", tool_call_id=%q", msg.ToolCallID))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("--- 结束 ---")

	// Remove messages from lastAssistantIdx to end
	a.messages = a.messages[:lastAssistantIdx]

	return sb.String()
}
