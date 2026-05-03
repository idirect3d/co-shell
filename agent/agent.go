// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-05-01
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

// rebuildSystemPrompt rebuilds the system prompt with current config identity info.
// It preserves the conversation history (only replaces the system message at index 0).
func (a *Agent) rebuildSystemPrompt() {
	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
	}
	a.systemPrompt = buildSystemPromptWithMode(a.rules, a.resultMode, agentName, agentDesc, agentPrinciples)
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
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
	}
	a.systemPrompt = buildSystemPromptWithMode(a.rules, mode, agentName, agentDesc, agentPrinciples)
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
