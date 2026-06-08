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
		subcommand == "show-logo":
		return h.handleDisplaySetting(subcommand, args)

	// Agent settings
	case subcommand == "name", subcommand == "description", subcommand == "principles",
		subcommand == "max-iterations", subcommand == "max-retries",
		subcommand == "memory-enabled", subcommand == "plan-enabled",
		subcommand == "subagent-enabled", subcommand == "context-limit",
		subcommand == "context-start", subcommand == "result-mode",
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
		subcommand == "loop-detect-max-window",
		subcommand == "dedup-enabled", subcommand == "dedup-feature-ratio",
		subcommand == "dedup-match-ratio", subcommand == "dedup-similarity-threshold",
		subcommand == "dedup-max-history", subcommand == "dedup-repeat-limit":
		return h.handleSafetySetting(subcommand, args)

	// Shell settings
	case subcommand == "shell-session-enabled", subcommand == "shell-session-timeout",
		subcommand == "shell-vt-rows", subcommand == "shell-vt-cols",
		subcommand == "input-mode":
		return h.handleAgentSetting(subcommand, args)

	// Search settings
	case subcommand == "search-max-line-length", subcommand == "search-max-result-bytes",
		subcommand == "search-context-lines",
		subcommand == "memory-search-max-content-len",
		subcommand == "memory-search-max-results":
		return h.handleSearchSetting(subcommand, args)

	// Log setting
	case subcommand == "log":
		return h.handleLogSetting(subcommand, args)

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

	agentDescDisplay := cfg.LLM.AgentDescription
	if agentDescDisplay == "" {
		agentDescDisplay = i18n.T(i18n.KeyAgentDefaultDescription)
	}

	// Collect all lines
	var allLines []settingLine

	// Group 1: Identity & Personality
	allLines = append(allLines,
		makeLine("name", agentName, i18n.T(i18n.KeyCol3Name)),
		makeLine("description", agentDescDisplay, i18n.T(i18n.KeyCol3Desc)),
	)

	// Show current work mode
	modeName := cfg.LLM.WorkMode
	if modeName == "" {
		modeName = "default"
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

	allLines = append(allLines,
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
	)

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
	)

	// Loop detection (FIX-179)
	loopDetectStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.LoopDetectEnabled {
		loopDetectStatus = i18n.T(i18n.KeyOn)
	}

	// Message dedup (FIX-179)
	dedupStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.DedupEnabled {
		dedupStatus = i18n.T(i18n.KeyOn)
	}

	// Group 4: Safety & Confirmation
	allLines = append(allLines,
		makeLine("confirm-tool", confirmStatus, i18n.T(i18n.KeyCol3Confirm)),
		makeLine("tool-timeout", toolTimeoutStr, i18n.T(i18n.KeyCol3ToolTimeout)),
		makeLine("cmd-timeout", cmdTimeoutStr, i18n.T(i18n.KeyCol3CmdTimeout)),
		makeLine("llm-timeout", llmTimeoutStr, i18n.T(i18n.KeyCol3LLMTimeout)),
		makeLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), i18n.T(i18n.KeyCol3ErrorMaxSingleCount)),
		makeLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), i18n.T(i18n.KeyCol3ErrorMaxTypeCount)),
		// FIX-179: Loop detection
		makeLine("loop-detect-enabled", loopDetectStatus, i18n.T(i18n.KeyCol3LoopDetectEnabled)),
		makeLine("loop-detect-threshold", fmt.Sprintf("%d", cfg.LLM.LoopDetectThreshold), i18n.T(i18n.KeyCol3LoopDetectThreshold)),
		makeLine("loop-detect-max-window", fmt.Sprintf("%d", cfg.LLM.LoopDetectMaxWindow), i18n.T(i18n.KeyCol3LoopDetectMaxWindow)),
		// FIX-179: Message dedup
		makeLine("dedup-enabled", dedupStatus, "消息去重(on|off)"),
		makeLine("dedup-feature-ratio", fmt.Sprintf("%.1f", cfg.LLM.DedupFeatureRatio), "特征词比例(0.0~1.0)"),
		makeLine("dedup-match-ratio", fmt.Sprintf("%.1f", cfg.LLM.DedupMatchRatio), "特征匹配率(0.0~1.0)"),
		makeLine("dedup-similarity-threshold", fmt.Sprintf("%d%%", cfg.LLM.DedupSimilarityThreshold), "相似度阈值(1~100)"),
		makeLine("dedup-max-history", fmt.Sprintf("%d", cfg.LLM.DedupMaxHistory), "历史消息数"),
		makeLine("dedup-repeat-limit", fmt.Sprintf("%d", cfg.LLM.DedupRepeatLimit), "重复次数"),
	)

	// Group 5: Memory & Context
	contextStartMode := i18n.T(i18n.KeyContextStartTask)
	if cfg.LLM.ContextStartMode == "window" {
		contextStartMode = i18n.T(i18n.KeyContextStartWindow)
	} else if cfg.LLM.ContextStartMode == "smart" {
		contextStartMode = i18n.T(i18n.KeyContextStartSmart)
	}
	dbEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.DB.Enabled {
		dbEnabledStatus = i18n.T(i18n.KeyOn)
	}
	allLines = append(allLines,
		makeLine("memory-enabled", memoryEnabledStatus, i18n.T(i18n.KeyCol3MemoryEnabled)),
		makeLine("context-limit", contextLimitStr, i18n.T(i18n.KeyCol3ContextLimit)),
		makeLine("context-start", contextStartMode, i18n.T(i18n.KeyCol3ContextStartMode)),
		makeLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), i18n.T(i18n.KeyCol3MemorySearchMaxContentLen)),
		makeLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), i18n.T(i18n.KeyCol3MemorySearchMaxResults)),
		makeLine("db", dbEnabledStatus, i18n.T(i18n.KeyDBSubCmdDesc)),
	)

	// Group 6: Search & Debug
	allLines = append(allLines,
		makeLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), i18n.T(i18n.KeyCol3SearchMaxLineLength)),
		makeLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), i18n.T(i18n.KeyCol3SearchMaxResultBytes)),
		makeLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), i18n.T(i18n.KeyCol3SearchContextLines)),
		makeLine("log", logStatus, i18n.T(i18n.KeyCol3Log)),
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
	writeGroup(i18n.T(i18n.KeySettingsGroupModel), nextLines(19)...)

	// Group 3: Display & Output
	writeGroup(i18n.T(i18n.KeySettingsGroupDisplay), nextLines(8)...)

	// Group 4: Safety & Confirmation (6 + 9 new = 15)
	writeGroup(i18n.T(i18n.KeySettingsGroupSafety), nextLines(15)...)

	// Group 5: Memory & Context
	writeGroup(i18n.T(i18n.KeySettingsGroupMemory), nextLines(6)...)

	// Group 6: Search & Debug
	writeGroup(i18n.T(i18n.KeySettingsGroupSearchDebug), nextLines(4)...)

	return sb.String()
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
