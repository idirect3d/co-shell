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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	if thinkingEnabled == "default" && activeModel.ThinkingEnabled != nil {
		if *activeModel.ThinkingEnabled {
			thinkingEnabled = "on"
		} else {
			thinkingEnabled = "off"
		}
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
	client.SetTokenUsage(h.cfg.LLM.TokenUsage)

	// Build body additions: cfg.BodyAdditions + thinking adapter + model custom params
	additions := make(map[string]string)
	if len(h.cfg.LLM.BodyAdditions) > 0 {
		for k, v := range h.cfg.LLM.BodyAdditions {
			additions[k] = v
		}
	}
	adapter := llm.GetThinkingAdapter(activeModel.Provider)
	thinkingAdditions := adapter.BuildAdditions(llm.ThinkingConfig{
		Mode:            llm.ThinkingModeFromString(thinkingEnabled),
		ReasoningEffort: reasoningEffort,
	})
	for k, v := range thinkingAdditions {
		additions[k] = v
	}
	if len(activeModel.CustomParams) > 0 {
		for k, v := range activeModel.CustomParams {
			jsonBytes, err := json.Marshal(v)
			if err == nil {
				additions[k] = string(jsonBytes)
			}
		}
	}
	if len(additions) > 0 {
		client.SetBodyAdditions(additions)
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
		subcommand == "vision-context-mode",
		subcommand == "thinking-enabled", subcommand == "reasoning-effort",
		subcommand == "toolcall-enabled", subcommand == "toolcall-mode",
		subcommand == "xml-tag-prefix", subcommand == "xml-stream-validate",
		subcommand == "top-p", subcommand == "top-k",
		subcommand == "repetition-penalty", subcommand == "max-model-len":
		return h.handleLLMSetting(subcommand, args)

	// Display settings
	case subcommand == "show-llm-thinking", subcommand == "show-llm-content",
		subcommand == "show-command", subcommand == "show-tool",
		subcommand == "show-tool-input", subcommand == "show-tool-output",
		subcommand == "show-command-output", subcommand == "emoji-enabled",
		subcommand == "show-logo",
		subcommand == "show-loop-detection",
		subcommand == "token-usage",
		subcommand == "output-categories":
		return h.handleDisplaySetting(subcommand, args)

	// Agent settings
	case subcommand == "name", subcommand == "description", subcommand == "principles",
		subcommand == "max-iterations", subcommand == "max-retries",
		subcommand == "memory-enabled", subcommand == "plan-enabled",
		subcommand == "subagent-enabled", subcommand == "context-limit",
		subcommand == "context-start", subcommand == "context-policy",
		subcommand == "context-reorganize-threshold", subcommand == "result-mode",
		subcommand == "no-tool-action",
		subcommand == "parse-error-action",
		subcommand == "shell-session-enabled", subcommand == "shell-session-timeout",
		subcommand == "shell-vt-rows", subcommand == "shell-vt-cols",
		subcommand == "browser-enabled", subcommand == "browser-port",
		subcommand == "browser-headless", subcommand == "browser-max-html-size",
		subcommand == "read-file-max-size",
		subcommand == "excel-max-cells", subcommand == "excel-max-sessions",
		subcommand == "docx-max-sessions", subcommand == "docx-max-read-paras",
		subcommand == "visual-analysis-max-images":
		return h.handleAgentSetting(subcommand, args)

	// Safety settings
	case subcommand == "confirm-tool", subcommand == "error-max-single-count",
		subcommand == "error-max-type-count",
		subcommand == "loop-intervention",
		subcommand == "loop-detect-threshold",
		subcommand == "loop-temp-enabled", subcommand == "loop-temp-step-up",
		subcommand == "loop-temp-step-down", subcommand == "loop-temp-max",
		subcommand == "loop-temp-min",
		subcommand == "loop-judge-enabled",
		subcommand == "loop-judge-timeout",
		subcommand == "loop-long-output-threshold",
		subcommand == "loop-single-line-length",
		subcommand == "loop-single-line-window",
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

	// Defaults subcommand: reset to system defaults (preserving LLM, Memory, DB)
	case subcommand == "defaults":
		return h.handleSetDefault()

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
	thinkingEnabledStatus := cfg.LLM.ThinkingEnabled
	if thinkingEnabledStatus == "" {
		thinkingEnabledStatus = "default"
	}
	reasoningEffortStr := cfg.LLM.ReasoningEffort
	if reasoningEffortStr == "" {
		reasoningEffortStr = "none"
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

	// Determine current mode name
	modeName := cfg.LLM.WorkMode
	if modeName == "" || modeName == "default" {
		modeName = "act"
	}

	// Resolve mode-specific description (same priority as agent/agent.go)
	modeDesc := ""
	if cfg.LLM.ModeDescriptions != nil {
		modeDesc = cfg.LLM.ModeDescriptions[modeName]
	}
	if modeDesc == "" {
		modeDesc = cfg.LLM.AgentDescription
	}
	if modeDesc == "" {
		switch modeName {
		case "plan":
			modeDesc = i18n.T(i18n.KeyAgentDefaultDescriptionPlan)
		case "research":
			modeDesc = i18n.T(i18n.KeyAgentDefaultDescriptionResearch)
		default:
			modeDesc = i18n.T(i18n.KeyAgentDefaultDescriptionAct)
		}
	}
	if modeDesc == "" {
		modeDesc = i18n.T(i18n.KeyAgentDefaultDescription)
	}

	// Resolve principles display (show full value)
	principlesDisplay := cfg.LLM.AgentPrinciples
	if principlesDisplay == "" {
		principlesDisplay = i18n.T(i18n.KeyAgentDefaultPrinciples)
	}
	if principlesDisplay == "" {
		principlesDisplay = "—"
	}

	// Collect lines by group
	var allGroups [][]settingLine

	// Group 1: Identity & Personality
	allGroups = append(allGroups, []settingLine{
		makeLine("name", agentName, i18n.T(i18n.KeyCol3Name)),
		makeLine("description", modeDesc, i18n.T(i18n.KeyCol3Desc)),
		makeLine("principles", principlesDisplay, i18n.T(i18n.KeyCol3Principles)),
		makeLine("mode", modeName, i18n.T(i18n.KeyCol3WorkMode)),
	})

	// Group 2: Agent Settings
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

	// XML stream validate status
	xmlStreamValidateStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.XMLStreamValidate {
		xmlStreamValidateStatus = i18n.T(i18n.KeyOn)
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

	allGroups = append(allGroups, []settingLine{
		makeLine("temperature", temperatureStr, "0.0 ~ 2.0（浮点数）"),
		makeLine("max-tokens", maxTokensStr, "1 ~ 128000（整数）"),
		makeLine("max-iterations", maxIterStr, i18n.T(i18n.KeyCol3MaxIter)),
		makeLine("vision", visionStatus, i18n.T(i18n.KeyCol3Vision)),
		makeLine("thinking-enabled", thinkingEnabledStatus, "on/off/default"),
		makeLine("reasoning-effort", reasoningEffortStr, "low/medium/high/max/none/default"),
		makeLine("toolcall-enabled", toolCallEnabledStatus, i18n.T(i18n.KeyCol3ToolCallEnabled)),
		makeLine("toolcall-mode", toolCallMode, i18n.T(i18n.KeyCol3ToolCallMode)),
		makeLine("xml-tag-prefix", cfg.LLM.XMLTagPrefix, "XML 标签前缀（如 cs:）"),
		makeLine("xml-stream-validate", xmlStreamValidateStatus, "流式XML校验(开发者选项, 默认开启)"),
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
		makeLine("excel-max-sessions", fmt.Sprintf("%d", cfg.LLM.ExcelMaxSessions), "Excel最大并发会话数(1-50)"),
		makeLine("excel-max-cells", fmt.Sprintf("%d", cfg.LLM.ExcelMaxCells), "Excel单次读取最大单元格数(10-100000)"),
		makeLine("docx-max-sessions", fmt.Sprintf("%d", cfg.LLM.DocxMaxSessions), "Word最大并发会话数(1-50)"),
		makeLine("docx-max-read-paras", fmt.Sprintf("%d", cfg.LLM.DocxMaxReadParas), "Word单次读取最大段落数(10-5000)"),
		makeLine("visual-analysis-max-images", fmt.Sprintf("%d", cfg.LLM.VisualAnalysisMaxImages), i18n.T(i18n.KeyCol3VisualAnalysisMaxImages)),
		makeLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), i18n.T(i18n.KeyCol3SearchMaxLineLength)),
		makeLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), i18n.T(i18n.KeyCol3SearchMaxResultBytes)),
		makeLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), i18n.T(i18n.KeyCol3SearchContextLines)),
	})
	// Add no-tool-action to Agent settings group
	noToolActionVal := cfg.LLM.NoToolAction
	if noToolActionVal == "" {
		noToolActionVal = "retry"
	}
	allGroups[1] = append(allGroups[1], makeLine("no-tool-action", noToolActionVal, "0-tool-call 处理方式(exit/retry/prompt)"))
	// Add parse-error-action to Agent settings group
	parseErrorActionVal := cfg.LLM.ParseErrorAction
	if parseErrorActionVal == "" {
		parseErrorActionVal = "retry"
	}
	allGroups[1] = append(allGroups[1], makeLine("parse-error-action", parseErrorActionVal, "方法调用解析错误处理方式(exit/retry/prompt)"))

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
	tokenUsageStatus := cfg.LLM.TokenUsage
	if tokenUsageStatus == "" {
		tokenUsageStatus = "on"
	}
	outputCategoriesSummary := ""
	for _, cat := range config.DefaultOutputCategories() {
		catStatus := i18n.T(i18n.KeyOn)
		if !cfg.OutputCategoryShown(cat) {
			catStatus = i18n.T(i18n.KeyOff)
		}
		if outputCategoriesSummary != "" {
			outputCategoriesSummary += " "
		}
		outputCategoriesSummary += cat + "=" + catStatus
	}
	allGroups = append(allGroups, []settingLine{
		makeLine("emoji-enabled", emojiStatus, i18n.T(i18n.KeyCol3EmojiEnabled)),
		makeLine("show-llm-thinking", llmThinkingStatus, i18n.T(i18n.KeyCol3LlmThinking)),
		makeLine("show-llm-content", llmContentStatus, i18n.T(i18n.KeyCol3LlmContent)),
		makeLine("show-tool", toolStatus, i18n.T(i18n.KeyCol3Tool)),
		makeLine("show-tool-input", toolInputStatus, i18n.T(i18n.KeyCol3ToolInput)),
		makeLine("show-tool-output", toolOutputStatus, i18n.T(i18n.KeyCol3ToolOutput)),
		makeLine("show-command", commandStatus, i18n.T(i18n.KeyCol3Command)),
		makeLine("show-command-output", commandOutputStatus, i18n.T(i18n.KeyCol3CommandOutput)),
		makeLine("show-loop-detection", loopDetectionShowStatus, i18n.T(i18n.KeyCol3ShowLoopDetection)),
		makeLine("token-usage", tokenUsageStatus, i18n.T(i18n.KeyCol3TokenUsage)),
		makeLine("output-categories", outputCategoriesSummary, "cat=on|off"),
	})

	// Loop detection (FIX-179)
	// Loop intervention (FEATURE-267)
	// Default is set in config.DefaultConfig() — empty means use default "retry"
	loopIntervention := cfg.LLM.LoopIntervention

	// Loop judgment (FEATURE-241)
	loopJudgeStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.LoopJudgeEnabled {
		loopJudgeStatus = i18n.T(i18n.KeyOn)
	}

	// Group 4: Safety & Confirmation
	allGroups = append(allGroups, []settingLine{
		makeLine("confirm-tool", confirmStatus, i18n.T(i18n.KeyCol3Confirm)),
		makeLine("tool-timeout", toolTimeoutStr, i18n.T(i18n.KeyCol3ToolTimeout)),
		makeLine("cmd-timeout", cmdTimeoutStr, i18n.T(i18n.KeyCol3CmdTimeout)),
		makeLine("llm-timeout", llmTimeoutStr, i18n.T(i18n.KeyCol3LLMTimeout)),
		makeLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), i18n.T(i18n.KeyCol3ErrorMaxSingleCount)),
		makeLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), i18n.T(i18n.KeyCol3ErrorMaxTypeCount)),
		// Loop detection (FEATURE-227)
		makeLine("loop-intervention", loopIntervention, "循环介入策略(off/retry/prompt/reorganize/temperature/random)"),
		makeLine("loop-detect-threshold", fmt.Sprintf("%d", cfg.LLM.LoopDetectThreshold), i18n.T(i18n.KeyCol3LoopDetectThreshold)),
		makeLine("loop-temp-step-up", fmt.Sprintf("%.2f", cfg.LLM.LoopTempStepUp), "循环温度上升步长"),
		makeLine("loop-temp-step-down", fmt.Sprintf("%.2f", cfg.LLM.LoopTempStepDown), "循环温度下降步长"),
		makeLine("loop-temp-max", fmt.Sprintf("%.2f", cfg.LLM.LoopTempMax), "循环温度上限"),
		makeLine("loop-temp-min", fmt.Sprintf("%.2f", cfg.LLM.LoopTempMin), "循环温度下限"),
		// Loop judgment (FEATURE-241)
		makeLine("loop-judge-enabled", loopJudgeStatus, i18n.T(i18n.KeyCol3LoopJudgeEnabled)),
		// Loop judge timeout
		makeLine("loop-judge-timeout", fmt.Sprintf("%ds", cfg.LLM.LoopJudgeTimeout), "LLM循环判定超时(秒,0=不限制)"),
		// Long output threshold
		makeLine("loop-long-output-threshold", fmt.Sprintf("%d", cfg.LLM.LoopLongOutputThreshold), "超长输出触发判定字符数(0=不检测)"),
		makeLine("loop-single-line-length", fmt.Sprintf("%d", cfg.LLM.LoopSingleLineLength), "单行超长阈值(0=不检测)"),
		makeLine("loop-single-line-window", fmt.Sprintf("%d", cfg.LLM.LoopSingleLineWindow), "单行窗口重复检测大小(0=不检测)"),
	})
	// loop-reorganize-enabled removed, controlled by loop-intervention

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
	allGroups = append(allGroups, []settingLine{
		makeLine("memory-enabled", memoryEnabledStatus, i18n.T(i18n.KeyCol3MemoryEnabled)),
		makeLine("context-limit", contextLimitStr, i18n.T(i18n.KeyCol3ContextLimit)),
		makeLine("context-policy", contextStartMode, "window/task/smart/reorganize"),
		makeLine("context-reorganize-threshold", reorganizeThresholdStr, "0-100%"),
		makeLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), i18n.T(i18n.KeyCol3MemorySearchMaxContentLen)),
		makeLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), i18n.T(i18n.KeyCol3MemorySearchMaxResults)),
		makeLine("db", dbEnabledStatus, i18n.T(i18n.KeyDBSubCmdDesc)),
	})

	// Group 6: Developer
	llmInteractionLogStatus := i18n.T(i18n.KeyOff)
	if log.IsLLMInteractionEnabled() {
		llmInteractionLogStatus = i18n.T(i18n.KeyOn)
	}
	allGroups = append(allGroups, []settingLine{
		makeLine("debug", debugStatus, i18n.T(i18n.KeyCol3Debug)),
		makeLine("log", logStatus, i18n.T(i18n.KeyCol3Log)),
		makeLine("llm-log", llmInteractionLogStatus, i18n.T(i18n.KeyCol3LLMInteractionLog)),
	})

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

	// Group titles in the same order as allGroups
	groupTitles := []string{
		i18n.T(i18n.KeySettingsGroupIdentity),
		i18n.T(i18n.KeySettingsGroupModel),
		i18n.T(i18n.KeySettingsGroupDisplay),
		i18n.T(i18n.KeySettingsGroupSafety),
		i18n.T(i18n.KeySettingsGroupMemory),
		i18n.T(i18n.KeySettingsGroupSearchDebug),
	}

	for gi, group := range allGroups {
		writeGroup(groupTitles[gi])
		for _, l := range group {
			sb.WriteString(formatLine(l.name, l.value, l.col3))
		}
	}

	// Append footer and :set default hint
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySettingsHelpFooter) + "\n")
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySettingsSetDefaultHint) + "\n")

	return sb.String()
}

// handleSetDefault resets all non-critical settings to system defaults.
// Preserves LLM, Memory & Context, and Database settings.
func (h *SettingsHandler) handleSetDefault() (string, error) {
	// Snapshot preserved values
	savedLLM := h.cfg.LLM
	savedMemory := h.cfg.LLM.MemoryEnabled
	savedContextLimit := h.cfg.LLM.ContextLimit
	savedContextPolicy := h.cfg.LLM.ContextPolicy
	savedContextReorganizeThreshold := h.cfg.LLM.ContextReorganizeThreshold
	savedMemorySearchMaxContentLen := h.cfg.LLM.MemorySearchMaxContentLen
	savedMemorySearchMaxResults := h.cfg.LLM.MemorySearchMaxResults
	savedDB := h.cfg.DB

	// Reset LLMConfig to defaults
	def := config.DefaultConfig()
	h.cfg.LLM = def.LLM

	// Restore LLM config snapshot
	h.cfg.LLM = savedLLM

	// Reset all non-preserved LLM fields to default, then restore preserved ones
	h.cfg.LLM.ShowLlmThinking = def.LLM.ShowLlmThinking
	h.cfg.LLM.ShowLlmContent = def.LLM.ShowLlmContent
	h.cfg.LLM.ShowTool = def.LLM.ShowTool
	h.cfg.LLM.ShowToolInput = def.LLM.ShowToolInput
	h.cfg.LLM.ShowToolOutput = def.LLM.ShowToolOutput
	h.cfg.LLM.ShowCommand = def.LLM.ShowCommand
	h.cfg.LLM.ShowCommandOutput = def.LLM.ShowCommandOutput
	h.cfg.LLM.EmojiEnabled = def.LLM.EmojiEnabled
	h.cfg.LLM.ShowLoopDetection = def.LLM.ShowLoopDetection
	h.cfg.LLM.ShowLogo = def.LLM.ShowLogo
	h.cfg.LLM.ToolModes = def.LLM.ToolModes
	h.cfg.LLM.ResultMode = def.LLM.ResultMode
	h.cfg.LLM.MaxIterations = def.LLM.MaxIterations
	h.cfg.LLM.MaxRetries = def.LLM.MaxRetries
	h.cfg.LLM.MemoryEnabled = savedMemory
	h.cfg.LLM.ContextLimit = savedContextLimit
	h.cfg.LLM.ContextPolicy = savedContextPolicy
	h.cfg.LLM.ContextReorganizeThreshold = savedContextReorganizeThreshold
	h.cfg.LLM.MemorySearchMaxContentLen = savedMemorySearchMaxContentLen
	h.cfg.LLM.MemorySearchMaxResults = savedMemorySearchMaxResults
	h.cfg.LLM.ShellSessionEnabled = def.LLM.ShellSessionEnabled
	h.cfg.LLM.ShellSessionTimeout = def.LLM.ShellSessionTimeout
	h.cfg.LLM.ShellVTRows = def.LLM.ShellVTRows
	h.cfg.LLM.ShellVTCols = def.LLM.ShellVTCols
	h.cfg.LLM.BrowserEnabled = def.LLM.BrowserEnabled
	h.cfg.LLM.BrowserPort = def.LLM.BrowserPort
	h.cfg.LLM.BrowserHeadless = def.LLM.BrowserHeadless
	h.cfg.LLM.BrowserMaxHTMLSize = def.LLM.BrowserMaxHTMLSize
	h.cfg.LLM.ExcelMaxSessions = def.LLM.ExcelMaxSessions
	h.cfg.LLM.ExcelMaxCells = def.LLM.ExcelMaxCells
	h.cfg.LLM.DocxMaxSessions = def.LLM.DocxMaxSessions
	h.cfg.LLM.DocxMaxReadParas = def.LLM.DocxMaxReadParas
	h.cfg.LLM.VisualAnalysisMaxImages = def.LLM.VisualAnalysisMaxImages
	h.cfg.LLM.VisionContextMode = def.LLM.VisionContextMode
	h.cfg.LLM.SearchMaxLineLength = def.LLM.SearchMaxLineLength
	h.cfg.LLM.SearchMaxResultBytes = def.LLM.SearchMaxResultBytes
	h.cfg.LLM.SearchContextLines = def.LLM.SearchContextLines
	h.cfg.LLM.ErrorMaxSingleCount = def.LLM.ErrorMaxSingleCount
	h.cfg.LLM.ErrorMaxTypeCount = def.LLM.ErrorMaxTypeCount
	h.cfg.LLM.ToolTimeout = def.LLM.ToolTimeout
	h.cfg.LLM.CommandTimeout = def.LLM.CommandTimeout
	h.cfg.LLM.LLMTimeout = def.LLM.LLMTimeout
	h.cfg.LLM.LoopIntervention = def.LLM.LoopIntervention
	h.cfg.LLM.LoopDetectThreshold = def.LLM.LoopDetectThreshold
	h.cfg.LLM.LoopTempEnabled = def.LLM.LoopTempEnabled
	h.cfg.LLM.LoopTempStepUp = def.LLM.LoopTempStepUp
	h.cfg.LLM.LoopTempStepDown = def.LLM.LoopTempStepDown
	h.cfg.LLM.LoopTempMax = def.LLM.LoopTempMax
	h.cfg.LLM.LoopTempMin = def.LLM.LoopTempMin
	h.cfg.LLM.LoopJudgeEnabled = def.LLM.LoopJudgeEnabled
	h.cfg.LLM.LoopReorganizeEnabled = def.LLM.LoopReorganizeEnabled
	h.cfg.LLM.LoopLongOutputThreshold = def.LLM.LoopLongOutputThreshold
	h.cfg.LLM.DuplicateContentThreshold = def.LLM.DuplicateContentThreshold
	h.cfg.LLM.LoopJudgeTimeout = def.LLM.LoopJudgeTimeout
	h.cfg.LLM.LoopSingleLineLength = def.LLM.LoopSingleLineLength
	h.cfg.LLM.LoopSingleLineWindow = def.LLM.LoopSingleLineWindow
	h.cfg.LLM.NoToolAction = def.LLM.NoToolAction
	h.cfg.LLM.ParseErrorAction = def.LLM.ParseErrorAction
	h.cfg.LLM.DebugMode = def.LLM.DebugMode
	h.cfg.LLM.TokenUsage = def.LLM.TokenUsage
	h.cfg.LLM.InputMode = def.LLM.InputMode
	h.cfg.LLM.XMLTagPrefix = def.LLM.XMLTagPrefix
	h.cfg.LLM.XMLStreamValidate = def.LLM.XMLStreamValidate
	h.cfg.LLM.ToolCallMode = def.LLM.ToolCallMode
	h.cfg.LLM.MaxModelLen = def.LLM.MaxModelLen
	h.cfg.LLM.LLMInteractionLog = def.LLM.LLMInteractionLog
	h.cfg.LLM.BodyAdditions = def.LLM.BodyAdditions
	h.cfg.LLM.ReadFileMaxSize = def.LLM.ReadFileMaxSize
	h.cfg.LLM.ListMaxItems = def.LLM.ListMaxItems
	h.cfg.LLM.ExcelSessionTTL = def.LLM.ExcelSessionTTL
	h.cfg.LLM.LoopPromptTemplate = def.LLM.LoopPromptTemplate
	h.cfg.LLM.ToolCallModeSystemPrompts = def.LLM.ToolCallModeSystemPrompts
	h.cfg.LLM.ModeDescriptions = def.LLM.ModeDescriptions
	h.cfg.LLM.ToolCallEnabled = def.LLM.ToolCallEnabled
	h.cfg.LLM.PlanEnabled = def.LLM.PlanEnabled
	h.cfg.LLM.SubAgentEnabled = def.LLM.SubAgentEnabled
	h.cfg.LLM.AgentPrinciples = ""

	// Restore DB
	h.cfg.DB = savedDB

	// Reset MCP
	h.cfg.MCP = def.MCP

	// Reset Log settings
	h.cfg.LogEnabled = def.LogEnabled
	h.cfg.LogLevel = def.LogLevel

	// Reset Rules
	h.cfg.Rules = def.Rules

	// Save config
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("save config after reset: %w", err)
	}

	// FIX-300: rename mode retained dirs so the default system prompt
	// (including {AGENT_PRINCIPLES}) is used instead of stale mode files.
	renameReport := h.renameModeRetainedDirs()

	// Sync to agent so the reset takes effect immediately (rebuilds system prompt)
	if h.agent != nil {
		h.agent.SetConfig(h.cfg)
	}

	msg := i18n.T(i18n.KeySettingsResetSuccess)
	if renameReport != "" {
		msg += "\n" + renameReport
	}
	return msg, nil
}

// modeDirRenameEntry represents a mode retained dir to be renamed.
type modeDirRenameEntry struct {
	oldPath string
	newPath string
	oldName string
	newName string
	err     error // set when rename fails
}

// knownModeNames returns the set of known work mode names:
// built-in act/plan/research plus user-defined modes from cfg.WorkModes.
func knownModeNames(cfg *config.Config) map[string]bool {
	names := map[string]bool{"act": true, "plan": true, "research": true}
	if cfg != nil {
		for _, wm := range cfg.WorkModes {
			names[wm.Name] = true
		}
	}
	return names
}

// listModeDirsToRename scans {cwd}/mode/ for directories whose name matches a
// known work mode name and returns rename entries with a .YYYYMMDD suffix
// (append .1, .2 ... when the target already exists).
func listModeDirsToRename(cwd string, cfg *config.Config, dateStr string) []modeDirRenameEntry {
	known := knownModeNames(cfg)
	modeRoot := filepath.Join(cwd, "mode")
	entries, err := os.ReadDir(modeRoot)
	if err != nil {
		return nil
	}
	var result []modeDirRenameEntry
	for _, e := range entries {
		if !known[e.Name()] {
			continue
		}
		oldPath := filepath.Join(modeRoot, e.Name())
		newName := e.Name() + "." + dateStr
		for suffix := 1; ; suffix++ {
			if _, err := os.Stat(filepath.Join(modeRoot, newName)); os.IsNotExist(err) {
				break
			}
			newName = e.Name() + "." + dateStr + "." + strconv.Itoa(suffix)
		}
		result = append(result, modeDirRenameEntry{
			oldPath: oldPath,
			newPath: filepath.Join(modeRoot, newName),
			oldName: e.Name(),
			newName: newName,
		})
	}
	return result
}

// renameModeDirs renames the given entries. It returns (renamed, failed):
// renamed lists succeeded entries, failed lists entries with their error.
func renameModeDirs(entries []modeDirRenameEntry) (renamed, failed []modeDirRenameEntry) {
	sorted := make([]modeDirRenameEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].oldName < sorted[j].oldName })

	for _, ent := range sorted {
		if rerr := os.Rename(ent.oldPath, ent.newPath); rerr != nil {
			ent.err = rerr
			failed = append(failed, ent)
			continue
		}
		renamed = append(renamed, ent)
	}
	return renamed, failed
}

// renameModeRetainedDirs handles FIX-300: after :set defaults resets internal
// parameters, retained mode config dirs (created before the {AGENT_PRINCIPLES}
// placeholder existed) would silently shadow the default system prompt. This
// lists those dirs, asks for confirmation, and renames them with a .YYYYMMDD
// suffix so the system prompt falls back to the default i18n template.
// Returns a human-readable report ("" when there is nothing to do).
func (h *SettingsHandler) renameModeRetainedDirs() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Warn("renameModeRetainedDirs: cannot get cwd: %v", err)
		return ""
	}
	entries := listModeDirsToRename(cwd, h.cfg, time.Now().Format("20060102"))
	if len(entries) == 0 {
		return ""
	}

	var listSB strings.Builder
	for _, e := range entries {
		listSB.WriteString(fmt.Sprintf("  · %s → %s\n", e.oldName, e.newName))
	}

	io := h.io()
	io.Print(fmt.Sprintf(i18n.T(i18n.KeySettingsResetModeDirsFound), listSB.String()))
	io.Print(i18n.T(i18n.KeySettingsResetModeDirsConfirm))
	answer, rerr := io.ReadLine()
	if rerr != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return i18n.T(i18n.KeySettingsResetModeDirsSkipped)
	}

	renamed, failed := renameModeDirs(entries)
	var report strings.Builder
	if len(renamed) > 0 {
		report.WriteString(i18n.T(i18n.KeySettingsResetModeDirsRenamed) + "\n")
		for _, e := range renamed {
			report.WriteString(fmt.Sprintf(i18n.T(i18n.KeySettingsResetModeDirsRenamedB), e.oldName, e.newName) + "\n")
		}
	}
	if len(failed) > 0 {
		report.WriteString(i18n.T(i18n.KeySettingsResetModeDirsFailed) + "\n")
		for _, e := range failed {
			report.WriteString(fmt.Sprintf(i18n.T(i18n.KeySettingsResetModeDirsFailedB), e.oldName, e.err) + "\n")
		}
	}
	return strings.TrimRight(report.String(), "\n")
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
