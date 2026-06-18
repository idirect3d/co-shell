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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/browser"
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
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.DualStore, rules string) *Agent {
	systemPrompt := buildSystemPromptWithMode(nil, rules, config.ResultModeMinimal, false, "", "", "", "", "", "", "", i18n.T(i18n.KeySystemPromptToolUsage))

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

// SetIO sets the UserIO implementation used by this agent for user interaction.
// Must be called before RunStream if enhanced input is desired.
func (a *Agent) SetIO(io UserIO) {
	a.io = io
}

// IO returns the current UserIO implementation (may be nil).
func (a *Agent) IO() UserIO {
	return a.io
}

// defaultIO returns the UserIO for output operations that happen before SetIO is called.
// When io is nil, falls back to direct fmt.Print.
func (a *Agent) defaultIO() UserIO {
	if a.io != nil {
		return a.io
	}
	return defaultIO
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
		a.toolModes["default"] = mode
	} else {
		a.toolModes[toolName] = mode
	}
}

// ToolModes returns the current tool mode settings (for display purposes only).
func (a *Agent) ToolModes() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.toolModes == nil {
		return nil
	}
	result := make(map[string]string, len(a.toolModes))
	for k, v := range a.toolModes {
		result[k] = v
	}
	return result
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
		"track_task_progress":        "auto",
		"view_task_plan":             "auto",
		"get_memory_slice":           "auto",
		"memory_search":              "auto",
		"delete_memory":              "confirm",
		"shell_send":                 "confirm",
		"shell_get_output":           "auto",
		"shell_window_content":       "auto",
		"shell_reset":                "auto",
		"attempt_completion":         "auto",
		"evaluate_expression":        "auto",
		// Browser tools (FEATURE-200) - all auto since screenshots are non-destructive
		"browser_navigate":                 "auto",
		"browser_screenshot":               "auto",
		"browser_click":                    "auto",
		"browser_type":                     "auto",
		"browser_evaluate":                 "auto",
		"browser_get_html":                 "auto",
		"browser_scroll":                   "auto",
		"browser_get_interactive_elements": "auto",
		"browser_go_back":                  "auto",
		"browser_go_forward":               "auto",
		"browser_close":                    "auto",
	}
}

// SyncToolModes synchronizes tool mode settings from config to agent.
// It applies per-tool overrides, global defaults, and mode-specific restrictions.
func (a *Agent) SyncToolModes(cfg *config.Config) {
	modes := DefaultToolModes()

	// Check if the current WorkMode has its own ToolModes.
	// If a WorkMode has explicit ToolModes, use them as the COMPLETE base —
	// the work mode's ToolModes represent the full intention for that mode,
	// and global cfg.LLM.ToolModes should NOT override mode-specific restrictions.
	workModeName := cfg.LLM.WorkMode
	if workModeName != "" {
		hasModeToolModes := false
		// Search user-defined modes first
		for _, wm := range cfg.WorkModes {
			if wm.Name == workModeName && wm.ToolModes != nil && len(wm.ToolModes) > 0 {
				modes = cloneToolModes(wm.ToolModes)
				hasModeToolModes = true
				break
			}
		}
		// Fall back to built-in modes (act, plan)
		if !hasModeToolModes {
			for _, wm := range config.DefaultWorkModes() {
				if wm.Name == workModeName && wm.ToolModes != nil && len(wm.ToolModes) > 0 {
					modes = cloneToolModes(wm.ToolModes)
					hasModeToolModes = true
					break
				}
			}
		}

		if hasModeToolModes {
			// Mode has its own ToolModes — apply per-tool overrides from config,
			// but ONLY for tools that already have an explicit setting in the mode.
			// The mode's "default" setting is respected; global default does NOT override it.
			if cfg.LLM.ToolModes != nil {
				for k, v := range cfg.LLM.ToolModes {
					if k == "default" {
						continue
					}
					// Only override if there's an explicit setting in the mode
					if _, hasExplicit := modes[k]; hasExplicit {
						modes[k] = v
					}
				}
			}
			a.toolModes = modes
			return
		}
	}

	// No mode-specific ToolModes: use defaults with global overrides.
	// Apply per-tool overrides from config.LLM.ToolModes (runtime overrides).
	if cfg.LLM.ToolModes != nil {
		for k, v := range cfg.LLM.ToolModes {
			if k == "default" {
				continue
			}
			if _, exists := modes[k]; exists {
				modes[k] = v
			}
		}
	}

	// Apply global default override if set to confirm/auto/disabled.
	if globalDefault, ok := cfg.LLM.ToolModes["default"]; ok && globalDefault != "" && globalDefault != "custom" {
		modes["default"] = globalDefault
		for k := range modes {
			if k != "default" {
				modes[k] = globalDefault
			}
		}
	}

	a.toolModes = modes
}

// cloneToolModes returns a copy of a tool modes map.
func cloneToolModes(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

func (a *Agent) SetStore(s *store.DualStore) { a.store = s }

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

// IsShellEnabled returns whether the shell session mode is enabled.
func (a *Agent) IsShellEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.shellEnabled
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
		// Resolve description with priority:
		// 1. Mode-specific description from ModeDescriptions
		// 2. Global AgentDescription
		// 3. Mode-specific i18n default (act/plan/research)
		// 4. Global i18n default
		workMode := a.cfg.LLM.WorkMode
		if workMode == "" {
			workMode = "act"
		}
		// Try mode-specific description first
		if a.cfg.LLM.ModeDescriptions != nil {
			if md, ok := a.cfg.LLM.ModeDescriptions[workMode]; ok && md != "" {
				agentDesc = md
			}
		}
		// Fall back to global description
		if agentDesc == "" {
			agentDesc = a.cfg.LLM.AgentDescription
		}
		// Fall back to mode-specific i18n default
		if agentDesc == "" {
			switch workMode {
			case "plan":
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionPlan)
			case "research":
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionResearch)
			default:
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionAct)
			}
		}
		// Fall back to global i18n default
		if agentDesc == "" {
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescription)
		}
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
			workMode := ""
			if a.cfg != nil {
				workMode = a.cfg.LLM.WorkMode
			}
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang, workMode)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.getCurrentTaskDescription()

	a.systemPrompt = buildSystemPromptWithMode(a.cfg, a.rules, a.resultMode, a.shellEnabled, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)
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

// selectModelForCall selects the appropriate model based on vision requirements
// and the current work mode's model bindings.
func (a *Agent) selectModelForCall() *config.ModelConfig {
	if a.modelManager == nil {
		return nil
	}

	// Determine which model ID to use based on current work mode
	modelID := a.getModelIDForCall()
	if modelID != "" {
		// Look up the model by ID in cfg.Models
		if a.cfg != nil {
			for _, m := range a.cfg.Models {
				if m.ID == modelID && m.Enabled {
					return m
				}
			}
		}
		// Fallback: try ModelManager
		if m := a.modelManager.GetModel(modelID); m != nil && m.Enabled {
			return m
		}
	}

	// No mode-specific model: use global priority
	visionRequired := len(a.imagePaths) > 0
	return a.modelManager.GetActiveModel(visionRequired)
}

// getModelIDForCall returns the model ID to use based on the current work mode.
// Returns the VisionModelID if vision is needed and set, otherwise ModelID.
// Returns empty string if neither is set (use global).
func (a *Agent) getModelIDForCall() string {
	if a.cfg == nil {
		return ""
	}
	workModeName := a.cfg.LLM.WorkMode
	if workModeName == "" {
		workModeName = "act"
	}

	// Search user-defined modes first, then built-in defaults
	var mode *config.WorkMode
	for i := range a.cfg.WorkModes {
		if a.cfg.WorkModes[i].Name == workModeName {
			mode = &a.cfg.WorkModes[i]
			break
		}
	}
	if mode == nil {
		for _, m := range config.DefaultWorkModes() {
			if m.Name == workModeName {
				mode = &m // note: this is a copy, but we only read ModelID/VisionModelID
				break
			}
		}
	}
	if mode == nil {
		return ""
	}

	visionRequired := len(a.imagePaths) > 0
	if visionRequired && mode.VisionModelID != nil {
		return *mode.VisionModelID
	}
	if mode.ModelID != nil {
		return *mode.ModelID
	}
	return ""
}

// ApplyWorkModeConfig creates a new LLM client using the current work mode's
// model binding and parameter overrides. Parameter priority:
//  1. WorkMode overrides (highest)
//  2. ModelConfig overrides (model-level)
//  3. Global cfg.LLM defaults (lowest)
//
// Call this when switching modes or when RunStream needs to establish the client.
func (a *Agent) ApplyWorkModeConfig() {
	if a.cfg == nil {
		return
	}

	// Step 1: Select the model
	var mode *config.WorkMode
	workModeName := a.cfg.LLM.WorkMode
	if workModeName == "" {
		workModeName = "act"
	}
	for i := range a.cfg.WorkModes {
		if a.cfg.WorkModes[i].Name == workModeName {
			mode = &a.cfg.WorkModes[i]
			break
		}
	}
	if mode == nil {
		for i, m := range config.DefaultWorkModes() {
			if m.Name == workModeName {
				mode = &config.DefaultWorkModes()[i]
				break
			}
		}
	}

	modelID := a.getModelIDForCall()
	var modelCfg *config.ModelConfig
	if modelID != "" {
		for _, m := range a.cfg.Models {
			if m.ID == modelID && m.Enabled {
				modelCfg = m
				break
			}
		}
	}
	if modelCfg == nil {
		visionRequired := len(a.imagePaths) > 0
		if a.modelManager != nil {
			modelCfg = a.modelManager.GetActiveModel(visionRequired)
		}
		if modelCfg == nil {
			modelCfg = config.GetActiveModelFromConfig(a.cfg)
		}
	}
	if modelCfg == nil {
		log.Warn("applyWorkModeConfig: no model config found, cannot switch")
		return
	}

	// Step 2: Merge parameters (mode > model config > global)
	temperature := a.cfg.LLM.Temperature
	if modelCfg.Temperature != nil {
		temperature = *modelCfg.Temperature
	}
	if mode != nil && mode.Temperature != nil {
		temperature = *mode.Temperature
	}

	maxTokens := a.cfg.LLM.MaxTokens
	if modelCfg.MaxTokens != nil {
		maxTokens = *modelCfg.MaxTokens
	}
	if mode != nil && mode.MaxTokens != nil {
		maxTokens = *mode.MaxTokens
	}

	thinkingEnabled := a.cfg.LLM.ThinkingEnabled
	if modelCfg.ThinkingEnabled != nil {
		thinkingEnabled = *modelCfg.ThinkingEnabled
	}
	if mode != nil && mode.ThinkingEnabled != nil {
		thinkingEnabled = *mode.ThinkingEnabled
	}

	reasoningEffort := a.cfg.LLM.ReasoningEffort
	if modelCfg.ReasoningEffort != nil {
		reasoningEffort = *modelCfg.ReasoningEffort
	}
	if mode != nil && mode.ReasoningEffort != nil {
		reasoningEffort = *mode.ReasoningEffort
	}

	topP := a.cfg.LLM.TopP
	if modelCfg.TopP != nil {
		topP = *modelCfg.TopP
	}
	if mode != nil && mode.TopP != nil {
		topP = *mode.TopP
	}

	topK := a.cfg.LLM.TopK
	if modelCfg.TopK != nil {
		topK = *modelCfg.TopK
	}
	if mode != nil && mode.TopK != nil {
		topK = *mode.TopK
	}

	repetitionPenalty := a.cfg.LLM.RepetitionPenalty
	if modelCfg.RepetitionPenalty != nil {
		repetitionPenalty = *modelCfg.RepetitionPenalty
	}
	if mode != nil && mode.RepetitionPenalty != nil {
		repetitionPenalty = *mode.RepetitionPenalty
	}

	// Create the LLM client
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

	// Merge body additions: global + model custom params
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

	// Update mode-level config settings that affect agent behavior
	if mode != nil && mode.MaxIterations != nil {
		a.SetMaxIterations(*mode.MaxIterations)
	}
	if mode != nil && mode.ContextLimit != nil {
		if a.cfg != nil {
			a.cfg.LLM.ContextLimit = *mode.ContextLimit
		}
	}
	if mode != nil && mode.ToolCallMode != nil {
		a.SetToolCallMode(*mode.ToolCallMode)
	}

	a.SetLLMClient(newClient)
	log.Info("applyWorkModeConfig: switched to model=%s, temperature=%.2f, maxTokens=%d, vision=%v (mode=%s)",
		modelCfg.Model, temperature, maxTokens, modelCfg.Capabilities.Vision, workModeName)
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

func (a *Agent) formatXMLToolResult(toolName, toolArgs, toolResult string, messageNo int) string {
	template := i18n.T(i18n.KeyXMLToolResultTemplate)
	result := strings.ReplaceAll(template, "{TOOL_CALL}", toolName)
	result = strings.ReplaceAll(result, "{TOOL_CALL_PARAMETERS}", toolArgs)
	result = strings.ReplaceAll(result, "{TOOL_RESULT}", toolResult)
	result = strings.ReplaceAll(result, "{TASK_TRACKING}", a.getTaskPlanPrompt())
	result = strings.ReplaceAll(result, "{MESSAGE_NO}", strconv.Itoa(messageNo))
	result = strings.ReplaceAll(result, "{CURRENT_TIME}", time.Now().Format("2006-01-02 15:04:05 Monday"))
	return result
}

func (a *Agent) formatUserMessage(instruction string, messageNo int) string {
	template := i18n.T(i18n.KeyUserMessageTemplate)
	result := strings.ReplaceAll(template, "{INSTRUCTION}", instruction)
	return result
}

// getCurrentTaskDescription returns the current task description for {TASK} in the
// system prompt. Priority:
// 1. Active task plan title (if one exists with unfinished steps)
// 2. The first user message at or after the messagePointer (context start)
// Returns empty string if neither is available.
func (a *Agent) getCurrentTaskDescription() string {
	// Priority 1: task plan title (even if all steps completed)
	if a.taskPlanMgr != nil {
		plan, err := a.taskPlanMgr.GetCurrent()
		if err == nil && plan != nil && plan.Title != "" {
			return plan.Title
		}
	}
	// Priority 2: first user message at/after messagePointer
	a.mu.Lock()
	defer a.mu.Unlock()
	startIdx := 1 // skip system prompt (index 0)
	if a.messagePointer > 0 && a.messagePointer < len(a.messages) {
		startIdx = a.messagePointer
	}
	for i := startIdx; i < len(a.messages); i++ {
		if a.messages[i].Role == "user" && a.messages[i].Content != "" {
			content := a.messages[i].Content
			// Strip <environment_details> if present for cleaner display
			if envStart := strings.Index(content, "<environment_details>"); envStart > 0 {
				content = strings.TrimSpace(content[:envStart])
			}
			// Truncate to reasonable length
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			return content
		}
	}
	return ""
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

// Interrupt signals the agent to stop receiving LLM stream data.
// Multiple calls are safe; subsequent signals are no-ops until ResetInterrupt.
func (a *Agent) Interrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	select {
	case a.interruptCh <- struct{}{}:
	default:
	}
}

// InterruptChan returns the interrupt channel for select-based listening.
func (a *Agent) InterruptChan() <-chan struct{} {
	return a.interruptCh
}

// ResetInterrupt re-creates the interrupt channel for a new request.
func (a *Agent) ResetInterrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.interruptCh = make(chan struct{}, 1)
}

func (a *Agent) TaskPlanManager() *taskplan.Manager  { return a.taskPlanMgr }
func (a *Agent) SetScheduler(s *scheduler.Scheduler) { a.scheduler = s }
func (a *Agent) Scheduler() *scheduler.Scheduler     { return a.scheduler }

func (a *Agent) SetResultMode(mode config.ResultMode) {
	// Build system prompt outside the lock to avoid deadlock:
	// getCurrentTaskDescription() acquires a.mu internally.
	a.mu.Lock()
	a.resultMode = mode
	a.mu.Unlock()

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
			workMode := ""
			if a.cfg != nil {
				workMode = a.cfg.LLM.WorkMode
			}
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang, workMode)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.getCurrentTaskDescription()

	a.systemPrompt = buildSystemPromptWithMode(a.cfg, a.rules, mode, a.shellEnabled, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)

	a.mu.Lock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	a.mu.Unlock()
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

// SetBrowserEnabled enables or disables browser tools.
func (a *Agent) SetBrowserEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.browserEnabled = enabled
	if enabled && a.chromeMgr == nil {
		// Browser manager will be initialized when first tool call is made
		log.Info("Browser tools enabled (will auto-start Chrome on first use)")
	} else if !enabled && a.chromeMgr != nil {
		a.chromeMgr.Stop()
		a.chromeMgr = nil
		log.Info("Browser tools disabled, Chrome stopped")
	}
	// Rebuild system prompt to include/exclude browser tool descriptions
	// Run in goroutine to avoid deadlock with mu
	go a.rebuildSystemPrompt()
}

// EnsureBrowser prepares Chrome for the agent if browser is enabled.
// Called during initialization to pre-launch Chrome when configured.
// IsBrowserEnabled returns whether browser tools are enabled.
func (a *Agent) IsBrowserEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.browserEnabled
}

// EnsureBrowserStarted ensures a Chrome browser instance is available.
// It first tries to connect to an already-running Chrome on the configured
// remote debugging port. Only falls back to starting a new Chrome instance
// if no existing instance is detected. This prevents creating duplicate
// browser windows when co-shell restarts or when Chrome is already running.
func (a *Agent) EnsureBrowserStarted() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.chromeMgr != nil && a.chromeMgr.IsRunning() {
		return nil
	}

	if a.cfg == nil {
		return fmt.Errorf("config not set")
	}

	port := a.cfg.LLM.BrowserPort
	if port <= 0 {
		port = 9222
	}

	// Use a persistent browser data directory under the workspace, so
	// Chrome state (cookies, sessions, downloads) survives co-shell restarts.
	// This also makes it possible to trace back issues from browser data.
	browserDataDir := filepath.Join(a.workspacePath, "browser-data")

	// Step 1: Try to reuse an already-running Chrome instance on the same port.
	// This avoids creating a new browser window when co-shell restarts or when
	// Chrome was left running from a previous session.
	debugURL := fmt.Sprintf("http://localhost:%d", port)
	if browser.IsEndpointAvailable(debugURL) {
		// Existing Chrome detected — create a ChromeManager without starting a new process.
		log.Info("Browser detected on port %d, reusing existing instance", port)
		mgr := browser.NewChromeManager(port, a.cfg.LLM.BrowserHeadless, browserDataDir)
		mgr.SetStarted() // Mark as started so Start() won't launch a new process
		a.chromeMgr = mgr
		return nil
	}

	// Step 2: No existing Chrome — start a new one.
	mgr := browser.NewChromeManager(port, a.cfg.LLM.BrowserHeadless, browserDataDir)
	if _, err := mgr.Start(); err != nil {
		return fmt.Errorf("cannot start browser: %w", err)
	}

	a.chromeMgr = mgr
	log.Info("Browser started (port=%d, headless=%v)", port, a.cfg.LLM.BrowserHeadless)
	return nil
}

// CloseBrowser stops the Chrome browser if running.
func (a *Agent) CloseBrowser() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.chromeMgr != nil {
		a.chromeMgr.Stop()
		a.chromeMgr = nil
		a.browserScreenshotData = ""
		log.Info("Browser closed")
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
