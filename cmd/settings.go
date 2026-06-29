// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-05-21
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// SettingsHandler handles the .settings built-in command.
type SettingsHandler struct {
	cfg   *config.Config
	agent *agent.Agent
	store *store.DualStore
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(cfg *config.Config, ag *agent.Agent, s *store.DualStore) *SettingsHandler {
	return &SettingsHandler{cfg: cfg, agent: ag, store: s}
}

// io returns the UserIO from the agent, falling back to DefaultUserIO.
func (h *SettingsHandler) io() agent.UserIO {
	return agent.GetIO(h.agent)
}

// rebuildLLMClient creates a new LLM client from current config and replaces it in the agent.
// This is called when LLM-related settings (api-key, endpoint, model, temperature, max-tokens, vision,
// top-p, top-k, repetition-penalty) are changed at runtime so the changes take effect immediately
// without restart.
func (h *SettingsHandler) rebuildLLMClient() {
	activeModel := config.GetActiveModelFromConfig(h.cfg)
	if activeModel == nil {
		log.Warn("Cannot rebuild LLM client: no active model found")
		return
	}

	// Resolve parameters: model-level takes precedence, fall back to global cfg.LLM
	temperature := h.cfg.LLM.Temperature
	if activeModel.Temperature != nil {
		temperature = *activeModel.Temperature
	}
	maxTokens := h.cfg.LLM.MaxTokens
	if activeModel.MaxTokens != nil {
		maxTokens = *activeModel.MaxTokens
	}
	thinkingEnabled := h.cfg.LLM.ThinkingEnabled
	if activeModel.ThinkingEnabled != nil {
		thinkingEnabled = *activeModel.ThinkingEnabled
	}
	reasoningEffort := h.cfg.LLM.ReasoningEffort
	if activeModel.ReasoningEffort != nil {
		reasoningEffort = *activeModel.ReasoningEffort
	}
	topP := h.cfg.LLM.TopP
	if activeModel.TopP != nil {
		topP = *activeModel.TopP
	}
	topK := h.cfg.LLM.TopK
	if activeModel.TopK != nil {
		topK = *activeModel.TopK
	}
	repetitionPenalty := h.cfg.LLM.RepetitionPenalty
	if activeModel.RepetitionPenalty != nil {
		repetitionPenalty = *activeModel.RepetitionPenalty
	}

	client := llm.NewClient(
		activeModel.Endpoint,
		activeModel.APIKey,
		activeModel.Model,
		temperature,
		maxTokens,
		h.cfg.LLM.LLMTimeout,
	)
	client.SetTopP(topP)
	client.SetTopK(topK)
	client.SetRepetitionPenalty(repetitionPenalty)
	client.SetThinkingEnabled(thinkingEnabled)
	client.SetReasoningEffort(reasoningEffort)
	client.SetTokenUsage(h.cfg.LLM.TokenUsage)
	if len(h.cfg.LLM.BodyAdditions) > 0 {
		client.SetBodyAdditions(h.cfg.LLM.BodyAdditions)
	}
	h.agent.SetLLMClient(client)
	log.Info("LLM client rebuilt from model %s: endpoint=%s model=%s",
		activeModel.ID, activeModel.Endpoint, activeModel.Model)
}

// Handle processes .settings commands.
func (h *SettingsHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return showSettingsHelp(h.cfg), nil
	}

	subcommand := args[0]

	// Dispatch to specialized handlers based on setting category
	switch {
	// LLM settings
	case subcommand == "api-key", subcommand == "endpoint", subcommand == "model",
		subcommand == "temperature", subcommand == "max-tokens", subcommand == "vision",
		subcommand == "thinking-enabled", subcommand == "reasoning-effort",
		subcommand == "toolcall-enabled", subcommand == "toolcall-mode",
		subcommand == "top-p", subcommand == "top-k",
		subcommand == "repetition-penalty", subcommand == "max-model-len":
		return h.handleLLMSetting(subcommand, args)

	// Display settings
	case subcommand == "show-llm-thinking", subcommand == "show-llm-content",
		subcommand == "show-command", subcommand == "show-tool",
		subcommand == "show-tool-input", subcommand == "show-tool-output",
		subcommand == "show-command-output", subcommand == "emoji-enabled",
		subcommand == "show-logo",
		subcommand == "show-loop-detection":
		return h.handleDisplaySetting(subcommand, args)

	// Agent settings
	case subcommand == "name", subcommand == "description", subcommand == "principles",
		subcommand == "max-iterations", subcommand == "max-retries",
		subcommand == "memory-enabled", subcommand == "plan-enabled",
		subcommand == "subagent-enabled", subcommand == "context-limit",
		subcommand == "context-start", subcommand == "context-policy",
		subcommand == "context-reorganize-threshold", subcommand == "result-mode",
		subcommand == "shell-session-enabled", subcommand == "shell-session-timeout",
		subcommand == "shell-vt-rows", subcommand == "shell-vt-cols",
		subcommand == "browser-enabled", subcommand == "browser-port",
		subcommand == "browser-headless", subcommand == "browser-max-html-size",
		subcommand == "read-file-max-size":
		return h.handleAgentSetting(subcommand, args)

	// Safety settings
	case subcommand == "confirm-tool", subcommand == "error-max-single-count",
		subcommand == "error-max-type-count",
		subcommand == "loop-detect-enabled", subcommand == "loop-detect-threshold",
		subcommand == "loop-detect-min-line-len",
		subcommand == "loop-temp-enabled", subcommand == "loop-temp-step-up",
		subcommand == "loop-temp-step-down", subcommand == "loop-temp-max",
		subcommand == "loop-temp-min",
		subcommand == "loop-judge-enabled",
		subcommand == "loop-reorganize-enabled",
		subcommand == "duplicate-content-threshold":
		return h.handleSafetySetting(subcommand, args)

	// Shell settings
	case subcommand == "shell-session-enabled", subcommand == "shell-session-timeout",
		subcommand == "shell-vt-rows", subcommand == "shell-vt-cols",
		subcommand == "input-mode":
		return h.handleAgentSetting(subcommand, args)

	// Search & Debug settings
	case subcommand == "search-max-line-length", subcommand == "search-max-result-bytes",
		subcommand == "search-context-lines",
		subcommand == "memory-search-max-content-len",
		subcommand == "memory-search-max-results",
		subcommand == "debug":
		return h.handleSearchSetting(subcommand, args)

	// Log setting
	case subcommand == "log":
		return h.handleLogSetting(subcommand, args)

	// LLM interaction log setting
	case subcommand == "llm-log":
		return h.handleLLMInteractionLogSetting(subcommand, args)

	// DB subcommand
	case subcommand == "db":
		return h.handleDBSubCommand(args[1:])

	// Tool call mode subcommand
	case subcommand == "tool":
		return h.handleToolSubCommand(args[1:])

	default:
		return "", fmt.Errorf("unknown setting: %s", subcommand)
	}
}

// showSettingsHelp displays the current configuration grouped by category.
func showSettingsHelp(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySettingsHelpFooter) + "\n")
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySettingsCurrentTitle) + "\n")

	// Prepare all value strings first to calculate max width for alignment
	type settingLine struct {
		name  string
		value string
		col3  string
	}

	// Helper to build a setting line struct
	makeLine := func(name, value, col3 string) settingLine {
		return settingLine{name: name + ":", value: value, col3: col3}
	}

	// Prepare values
	llmThinkingStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowLlmThinking {
		llmThinkingStatus = i18n.T(i18n.KeyOn)
	}
	llmContentStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowLlmContent {
		llmContentStatus = i18n.T(i18n.KeyOn)
	}
	commandStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowCommand {
		commandStatus = i18n.T(i18n.KeyOn)
	}
	toolStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowTool {
		toolStatus = i18n.T(i18n.KeyOn)
	}
	toolInputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowToolInput {
		toolInputStatus = i18n.T(i18n.KeyOn)
	}
	toolOutputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowToolOutput {
		toolOutputStatus = i18n.T(i18n.KeyOn)
	}
	commandOutputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowCommandOutput {
		commandOutputStatus = i18n.T(i18n.KeyOn)
	}

	confirmStatus := "custom"
	if v, ok := cfg.LLM.ToolModes["default"]; ok && v != "" {
		confirmStatus = v
	}
	logStatus := log.LogLevelString(log.GetLevel())
	visionStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.VisionSupport {
		visionStatus = i18n.T(i18n.KeyOn)
	}
	thinkingEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ThinkingEnabled {
		thinkingEnabledStatus = i18n.T(i18n.KeyOn)
	}
	toolCallEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ToolCallEnabled {
		toolCallEnabledStatus = i18n.T(i18n.KeyOn)
	}
	memoryEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.MemoryEnabled {
		memoryEnabledStatus = i18n.T(i18n.KeyOn)
	}
	planEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.PlanEnabled {
		planEnabledStatus = i18n.T(i18n.KeyOn)
	}
	subAgentEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.SubAgentEnabled {
		subAgentEnabledStatus = i18n.T(i18n.KeyOn)
	}

	maxIterStr := fmt.Sprintf("%d", cfg.LLM.MaxIterations)
	if cfg.LLM.MaxIterations <= 0 {
		maxIterStr = "1000 (" + i18n.T(i18n.KeyDefault) + ")"
	}

	toolTimeoutStr := fmt.Sprintf("%d", cfg.LLM.ToolTimeout)
	if cfg.LLM.ToolTimeout <= 0 {
		toolTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	cmdTimeoutStr := fmt.Sprintf("%d", cfg.LLM.CommandTimeout)
	if cfg.LLM.CommandTimeout <= 0 {
		cmdTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	llmTimeoutStr := fmt.Sprintf("%d", cfg.LLM.LLMTimeout)
	if cfg.LLM.LLMTimeout <= 0 {
		llmTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}

	contextLimitStr := fmt.Sprintf("%d", cfg.LLM.ContextLimit)
	if cfg.LLM.ContextLimit == 0 {
		contextLimitStr = i18n.T(i18n.KeyOff)
	} else if cfg.LLM.ContextLimit == -1 {
		contextLimitStr = i18n.T(i18n.KeyUnlimited)
	}

	agentName := cfg.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	resultModeStr := config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))

	// Look up the current work mode's description if available;
	// otherwise fall back to global AgentDescription.
	agentDescDisplay := lookupWorkModeDescription(cfg, cfg.LLM.WorkMode)

	// Collect all lines
	var allLines []settingLine

	// Group 1: Identity & Personality
	allLines = append(allLines,
		makeLine("name", agentName, i18n.T(i18n.KeyCol3Name)),
		makeLine("description", agentDescDisplay, i18n.T(i18n.KeyCol3Desc)),
	)

	// Show current work mode
	modeName := cfg.LLM.WorkMode
	if modeName == "" || modeName == "default" {
		modeName = "act"
	}
	allLines = append(allLines,
		makeLine("mode", modeName, i18n.T(i18n.KeyCol3WorkMode)),
	)

	// Group 2: Agent Settings (19 lines = 15 + 4 browser)
	// Use cfg.Models directly for smart model selection display
	allModels := cfg.Models

	// Sort by priority descending for display
	sortedModels := make([]*config.ModelConfig, len(allModels))
	copy(sortedModels, allModels)
	for i := 0; i < len(sortedModels); i++ {
		for j := i + 1; j < len(sortedModels); j++ {
			if sortedModels[j].Priority > sortedModels[i].Priority {
				sortedModels[i], sortedModels[j] = sortedModels[j], sortedModels[i]
			}
		}
	}

	// Find default tool model (highest priority enabled model with ToolCall capability)
	defaultToolModelID := "-"
	for _, m := range sortedModels {
		if m.Enabled && m.Capabilities.ToolCall {
			defaultToolModelID = m.ID
			break
		}
	}
	if defaultToolModelID == "-" && len(sortedModels) > 0 {
		defaultToolModelID = sortedModels[0].ID
	}

	// Find default vision model (highest priority enabled model with Vision capability)
	// If none found, show "-" (no fallback to first model)
	defaultVisionModelID := "-"
	for _, m := range sortedModels {
		if m.Enabled && m.Capabilities.Vision {
			defaultVisionModelID = m.ID
			break
		}
	}

	// Default problem-solving model: second highest priority enabled model with ToolCall capability
	defaultProblemModelID := "-"
	toolModelCount := 0
	for _, m := range sortedModels {
		if m.Enabled && m.Capabilities.ToolCall {
			toolModelCount++
			if toolModelCount == 2 {
				defaultProblemModelID = m.ID
				break
			}
		}
	}

	// Tool call mode
	toolCallMode := cfg.LLM.ToolCallMode
	if toolCallMode == "" {
		toolCallMode = "openai"
	}

	shellSessionEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShellSessionEnabled {
		shellSessionEnabledStatus = i18n.T(i18n.KeyOn)
	}
	shellTimeoutStr := fmt.Sprintf("%d", cfg.LLM.ShellSessionTimeout)
	if cfg.LLM.ShellSessionTimeout <= 0 {
		shellTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	shellVtRows := cfg.LLM.ShellVTRows
	if shellVtRows <= 0 {
		shellVtRows = 24
	}
	shellVtCols := cfg.LLM.ShellVTCols
	if shellVtCols <= 0 {
		shellVtCols = 80
	}

	browserEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.BrowserEnabled {
		browserEnabledStatus = i18n.T(i18n.KeyOn)
	}
	browserHeadlessStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.BrowserHeadless {
		browserHeadlessStatus = i18n.T(i18n.KeyOn)
	}

	temperatureStr := fmt.Sprintf("%.1f", cfg.LLM.Temperature)
	maxTokensStr := fmt.Sprintf("%d", cfg.LLM.MaxTokens)

	allLines = append(allLines,
		makeLine("temperature", temperatureStr, "0.0 ~ 2.0（浮点数）"),
		makeLine("max-tokens", maxTokensStr, "1 ~ 128000（整数）"),
		makeLine("max-iterations", maxIterStr, i18n.T(i18n.KeyCol3MaxIter)),
		makeLine("vision", visionStatus, i18n.T(i18n.KeyCol3Vision)),
		makeLine("thinking-enabled", thinkingEnabledStatus, i18n.T(i18n.KeyCol3ThinkingEnabled)),
		makeLine("toolcall-enabled", toolCallEnabledStatus, i18n.T(i18n.KeyCol3ToolCallEnabled)),
		makeLine("toolcall-mode", toolCallMode, i18n.T(i18n.KeyCol3ToolCallMode)),
		makeLine("default-tool-model", defaultToolModelID, i18n.T(i18n.KeyCol3DefaultToolModel)),
		makeLine("default-vision-model", defaultVisionModelID, i18n.T(i18n.KeyCol3DefaultVisionModel)),
		makeLine("default-problem-model", defaultProblemModelID, i18n.T(i18n.KeyCol3DefaultProblemModel)),
		makeLine("plan-enabled", planEnabledStatus, i18n.T(i18n.KeyCol3PlanEnabled)),
		makeLine("subagent-enabled", subAgentEnabledStatus, i18n.T(i18n.KeyCol3SubAgentEnabled)),
		makeLine("result-mode", resultModeStr, i18n.T(i18n.KeyCol3ResultMode)),
		makeLine("shell-session-enabled", shellSessionEnabledStatus, i18n.T(i18n.KeyCol3ShellSessionEnabled)),
		makeLine("shell-session-timeout", shellTimeoutStr, i18n.T(i18n.KeyCol3ShellSessionTimeout)),
		makeLine("shell-vt-rows", fmt.Sprintf("%d", shellVtRows), "虚拟终端行数(5-200)"),
		makeLine("shell-vt-cols", fmt.Sprintf("%d", shellVtCols), "虚拟终端列数(20-500)"),
		makeLine("browser-enabled", browserEnabledStatus, i18n.T(i18n.KeyCol3BrowserEnabled)),
		makeLine("browser-port", fmt.Sprintf("%d", cfg.LLM.BrowserPort), i18n.T(i18n.KeyCol3BrowserPort)),
		makeLine("browser-headless", browserHeadlessStatus, i18n.T(i18n.KeyCol3BrowserHeadless)),
		makeLine("browser-max-html-size", fmt.Sprintf("%d bytes (%d KB)", cfg.LLM.BrowserMaxHTMLSize, cfg.LLM.BrowserMaxHTMLSize/1024), "HTML下载阈值"),
		// Search settings
		makeLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), i18n.T(i18n.KeyCol3SearchMaxLineLength)),
		makeLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), i18n.T(i18n.KeyCol3SearchMaxResultBytes)),
		makeLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), i18n.T(i18n.KeyCol3SearchContextLines)),
	)

	// Show loop detection (FEATURE-241)
	loopDetectionShowStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowLoopDetection {
		loopDetectionShowStatus = i18n.T(i18n.KeyOn)
	}
	debugStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.DebugMode {
		debugStatus = i18n.T(i18n.KeyOn)
	}

	// Group 3: Display & Output
	emojiStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.EmojiEnabled {
		emojiStatus = i18n.T(i18n.KeyOn)
	}
	allLines = append(allLines,
		makeLine("emoji-enabled", emojiStatus, i18n.T(i18n.KeyCol3EmojiEnabled)),
		makeLine("show-llm-thinking", llmThinkingStatus, i18n.T(i18n.KeyCol3LlmThinking)),
		makeLine("show-llm-content", llmContentStatus, i18n.T(i18n.KeyCol3LlmContent)),
		makeLine("show-tool", toolStatus, i18n.T(i18n.KeyCol3Tool)),
		makeLine("show-tool-input", toolInputStatus, i18n.T(i18n.KeyCol3ToolInput)),
		makeLine("show-tool-output", toolOutputStatus, i18n.T(i18n.KeyCol3ToolOutput)),
		makeLine("show-command", commandStatus, i18n.T(i18n.KeyCol3Command)),
		makeLine("show-command-output", commandOutputStatus, i18n.T(i18n.KeyCol3CommandOutput)),
		makeLine("show-loop-detection", loopDetectionShowStatus, i18n.T(i18n.KeyCol3ShowLoopDetection)),
	)

	// Loop detection (FIX-179)
	loopDetectStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.LoopDetectEnabled {
		loopDetectStatus = i18n.T(i18n.KeyOn)
	}

	// Loop temperature adjustment (FEATURE-230)
	loopTempStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.LoopTempEnabled {
		loopTempStatus = i18n.T(i18n.KeyOn)
	}

	// Loop judgment (FEATURE-241)
	loopJudgeStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.LoopJudgeEnabled {
		loopJudgeStatus = i18n.T(i18n.KeyOn)
	}

	// Group 4: Safety & Confirmation
	allLines = append(allLines,
		makeLine("confirm-tool", confirmStatus, i18n.T(i18n.KeyCol3Confirm)),
		makeLine("tool-timeout", toolTimeoutStr, i18n.T(i18n.KeyCol3ToolTimeout)),
		makeLine("cmd-timeout", cmdTimeoutStr, i18n.T(i18n.KeyCol3CmdTimeout)),
		makeLine("llm-timeout", llmTimeoutStr, i18n.T(i18n.KeyCol3LLMTimeout)),
		makeLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), i18n.T(i18n.KeyCol3ErrorMaxSingleCount)),
		makeLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), i18n.T(i18n.KeyCol3ErrorMaxTypeCount)),
		// Loop detection (FEATURE-227)
		makeLine("loop-detect-enabled", loopDetectStatus, i18n.T(i18n.KeyCol3LoopDetectEnabled)),
		makeLine("loop-detect-threshold", fmt.Sprintf("%d", cfg.LLM.LoopDetectThreshold), i18n.T(i18n.KeyCol3LoopDetectThreshold)),
		// Loop temperature (FEATURE-230)
		makeLine("loop-temp-enabled", loopTempStatus, "循环温度自动调节"),
		makeLine("loop-temp-step-up", fmt.Sprintf("%.2f", cfg.LLM.LoopTempStepUp), "循环温度上升步长"),
		makeLine("loop-temp-step-down", fmt.Sprintf("%.2f", cfg.LLM.LoopTempStepDown), "循环温度下降步长"),
		makeLine("loop-temp-max", fmt.Sprintf("%.2f", cfg.LLM.LoopTempMax), "循环温度上限"),
		makeLine("loop-temp-min", fmt.Sprintf("%.2f", cfg.LLM.LoopTempMin), "循环温度下限"),
		// Loop judgment (FEATURE-241)
		makeLine("loop-judge-enabled", loopJudgeStatus, i18n.T(i18n.KeyCol3LoopJudgeEnabled)),
	)
	loopReorganizeStatus := i18n.T(i18n.KeyOn)
	if !cfg.LLM.LoopReorganizeEnabled {
		loopReorganizeStatus = i18n.T(i18n.KeyOff)
	}
	allLines = append(allLines,
		makeLine("loop-reorganize-enabled", loopReorganizeStatus, "循环检测重整上下文"),
	)

	// Group 5: Memory & Context
	contextStartMode := i18n.T(i18n.KeyContextPolicyReorganize)
	if cfg.LLM.ContextPolicy == "window" {
		contextStartMode = i18n.T(i18n.KeyContextPolicyWindow)
	} else if cfg.LLM.ContextPolicy == "smart" {
		contextStartMode = i18n.T(i18n.KeyContextPolicySmart)
	} else if cfg.LLM.ContextPolicy == "task" {
		contextStartMode = i18n.T(i18n.KeyContextPolicyTask)
	}
	dbEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.DB.Enabled {
		dbEnabledStatus = i18n.T(i18n.KeyOn)
	}
	reorganizeThresholdStr := fmt.Sprintf("%d%%", cfg.LLM.ContextReorganizeThreshold)
	if cfg.LLM.ContextReorganizeThreshold == 0 {
		reorganizeThresholdStr = "off"
	}
	allLines = append(allLines,
		makeLine("memory-enabled", memoryEnabledStatus, i18n.T(i18n.KeyCol3MemoryEnabled)),
		makeLine("context-limit", contextLimitStr, i18n.T(i18n.KeyCol3ContextLimit)),
		makeLine("context-policy", contextStartMode, "window/task/smart/reorganize"),
		makeLine("context-reorganize-threshold", reorganizeThresholdStr, "0-100%"),
		makeLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), i18n.T(i18n.KeyCol3MemorySearchMaxContentLen)),
		makeLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), i18n.T(i18n.KeyCol3MemorySearchMaxResults)),
		makeLine("db", dbEnabledStatus, i18n.T(i18n.KeyDBSubCmdDesc)),
	)

	// Group 6: Developer
	allLines = append(allLines,
		makeLine("debug", debugStatus, i18n.T(i18n.KeyCol3Debug)),
		makeLine("log", logStatus, i18n.T(i18n.KeyCol3Log)),
	)
	llmInteractionLogStatus := i18n.T(i18n.KeyOff)
	if log.IsLLMInteractionEnabled() {
		llmInteractionLogStatus = i18n.T(i18n.KeyOn)
	}
	allLines = append(allLines,
		makeLine("llm-log", llmInteractionLogStatus, i18n.T(i18n.KeyCol3LLMInteractionLog)),
	)

	// Helper to format a setting line with fixed column widths
	formatLine := func(name, value, col3 string) string {
		return fmt.Sprintf("  %-32s %-30s %s\n", name, value, col3)
	}

	// Helper to write a group
	writeGroup := func(title string, lines ...string) {
		sb.WriteString("\n  " + title + "\n")
		for _, line := range lines {
			sb.WriteString(line)
		}
	}

	// Track index for iterating through allLines
	lineIdx := 0
	nextLines := func(n int) []string {
		result := make([]string, 0, n)
		for i := 0; i < n && lineIdx < len(allLines); i++ {
			l := allLines[lineIdx]
			result = append(result, formatLine(l.name, l.value, l.col3))
			lineIdx++
		}
		return result
	}

	// Group 1: Identity & Personality
	writeGroup(i18n.T(i18n.KeySettingsGroupIdentity), nextLines(3)...)

	// Group 2: Agent Settings
	writeGroup(i18n.T(i18n.KeySettingsGroupModel), nextLines(24)...)

	// Group 3: Display & Output
	writeGroup(i18n.T(i18n.KeySettingsGroupDisplay), nextLines(9)...)

	// Group 4: Safety & Confirmation
	writeGroup(i18n.T(i18n.KeySettingsGroupSafety), nextLines(16)...)

	// Group 5: Memory & Context
	writeGroup(i18n.T(i18n.KeySettingsGroupMemory), nextLines(6)...)

	// Group 6: Developer
	writeGroup(i18n.T(i18n.KeySettingsGroupSearchDebug), nextLines(4)...)

	return sb.String()
}

// lookupWorkModeDescription returns the Identity section content for the current work mode,
// which is the same identity text sent to the LLM (with {AGENT_NAME} and {AGENT_DESCRIPTION} populated).
// This follows the same logic as agent.buildNamedSection("Identity", ...):
//  1. {cwd}/mode/{modeName}/IDENTITY.md (if modeName is set and file exists)
//  2. i18n fallback (KeySystemPromptIdentity)
func lookupWorkModeDescription(cfg *config.Config, modeName string) string {
	if modeName == "" || modeName == "default" {
		modeName = "act"
	}

	// Priority 1: load from mode-specific external file (matches agent.loadSectionText)
	cwd, _ := os.Getwd()
	identityText := loadModeIdentityFile(cwd, modeName)

	// Priority 2: i18n fallback
	if identityText == "" {
		identityText = i18n.T(i18n.KeySystemPromptIdentity)
	}

	// If no identity text resolved at all, fall back to agent description
	if identityText == "" || identityText == i18n.KeySystemPromptIdentity {
		agentDesc := cfg.LLM.AgentDescription
		if agentDesc == "" {
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescription)
		}
		return agentDesc
	}

	agentName := cfg.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	identityText = strings.ReplaceAll(identityText, "{AGENT_NAME}", agentName)

	// Resolve {AGENT_DESCRIPTION} with same priority as rebuildSystemPrompt
	agentDesc := ""
	if cfg.LLM.ModeDescriptions != nil {
		if md, ok := cfg.LLM.ModeDescriptions[modeName]; ok && md != "" {
			agentDesc = md
		}
	}
	if agentDesc == "" {
		agentDesc = cfg.LLM.AgentDescription
	}
	if agentDesc == "" {
		switch modeName {
		case "plan":
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionPlan)
		case "research":
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionResearch)
		default:
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionAct)
		}
	}
	if agentDesc == "" {
		agentDesc = i18n.T(i18n.KeyAgentDefaultDescription)
	}
	identityText = strings.ReplaceAll(identityText, "{AGENT_DESCRIPTION}", agentDesc)
	return identityText
}

// loadModeIdentityFile attempts to load the IDENTITY.md file for the given mode.
func loadModeIdentityFile(cwd, modeName string) string {
	if cwd == "" || modeName == "" {
		return ""
	}
	path := filepath.Join(cwd, "mode", modeName, "IDENTITY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// formatSettings formats the settings for display.
func formatSettings(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyConfigTitle) + "\n")
	activeModel := config.GetActiveModelFromConfig(cfg)
	provider := "(not set)"
	endpoint := "(not set)"
	modelName := "(not set)"
	if activeModel != nil {
		provider = activeModel.Provider
		endpoint = activeModel.Endpoint
		modelName = activeModel.Model
	}
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigProvider), provider))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigEndpoint), endpoint))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigModel), modelName))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigTemperature), cfg.LLM.Temperature))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigMaxTokens), cfg.LLM.MaxTokens))
	return sb.String()
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
