// Web UI settings support (FEATURE-391): SettingsJSON returns the current
// configuration as a structured, grouped list of setting items so the browser
// can render a graphical settings panel (instead of typing :set commands).
//
// Author: L.Shuang
// Created: 2026-08-21
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// WebSettingItem is one setting item exposed to the Web UI.
type WebSettingItem struct {
	Key     string   `json:"key"`
	Value   string   `json:"value"`
	Desc    string   `json:"desc"`
	Type    string   `json:"type"`              // "bool" | "number" | "enum" | "string"
	Options []string `json:"options,omitempty"` // for enum
	// Default is the system default value for this setting (FEATURE-470). The
	// Web UI shows it in the tooltip and marks the row with a red * when the
	// current value differs from it.
	Default string `json:"default,omitempty"`
}

// WebSettingGroup is a titled group of setting items.
type WebSettingGroup struct {
	Title string           `json:"title"`
	Items []WebSettingItem `json:"items"`
	// Kind marks a special-purpose group rendered by a dedicated UI instead of
	// the generic key/value rows (FEATURE-464). "mcp" renders the MCP server
	// manager; empty means the generic setting rows.
	Kind string `json:"kind,omitempty"`
}

// boolStr converts a bool to "on"/"off".
func boolStr(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// SettingsJSON returns the current configuration as grouped setting items for
// the Web UI. The grouping and order match the TUI :set output
// (showSettingsHelp), except that the [Identity & Personality] group is
// omitted (it will be placed elsewhere in the future).
func (h *SettingsHandler) SettingsJSON() []WebSettingGroup {
	cfg := h.cfg
	llm := cfg.LLM
	// FEATURE-470: the system defaults come from DefaultConfig so the Web UI
	// can show them in tooltips and mark values that differ from the default.
	def := config.DefaultConfig().LLM

	// Group 1: Agent Settings (matches showSettingsHelp Group 2)
	agentGroup := []WebSettingItem{
		{Key: "temperature", Value: fmt.Sprintf("%.1f", llm.Temperature), Desc: i18n.T(i18n.KeySettingCmd_301), Type: "number", Default: fmt.Sprintf("%.1f", def.Temperature)},
		{Key: "max-tokens", Value: strconv.Itoa(llm.MaxTokens), Desc: i18n.T(i18n.KeySettingCmd_302), Type: "number", Default: strconv.Itoa(def.MaxTokens)},
		{Key: "max-iterations", Value: strconv.Itoa(llm.MaxIterations), Desc: i18n.T(i18n.KeyCol3MaxIter), Type: "number", Default: strconv.Itoa(def.MaxIterations)},
		{Key: "vision", Value: boolStr(llm.VisionSupport), Desc: i18n.T(i18n.KeyCol3Vision), Type: "bool", Default: boolStr(def.VisionSupport)},
		{Key: "vision-context-mode", Value: visionContextModeValue(llm.VisionContextMode), Desc: i18n.T(i18n.KeySettingCmd_303), Type: "enum", Options: []string{"minimal", "full"}, Default: visionContextModeValue(def.VisionContextMode)},
		{Key: "thinking-enabled", Value: thinkingEnabledValue(llm.ThinkingEnabled), Desc: "on/off/default", Type: "enum", Options: []string{"default", "on", "off"}, Default: thinkingEnabledValue(def.ThinkingEnabled)},
		{Key: "reasoning-effort", Value: reasoningEffortValue(llm.ReasoningEffort), Desc: "low/medium/high/max/none/default", Type: "enum", Options: []string{"default", "none", "low", "medium", "high", "max"}, Default: reasoningEffortValue(def.ReasoningEffort)},
		{Key: "toolcall-enabled", Value: boolStr(llm.ToolCallEnabled), Desc: i18n.T(i18n.KeyCol3ToolCallEnabled), Type: "bool", Default: boolStr(def.ToolCallEnabled)},
		{Key: "toolcall-mode", Value: toolCallModeValue(llm.ToolCallMode), Desc: i18n.T(i18n.KeyCol3ToolCallMode), Type: "enum", Options: []string{"openai", "xml"}, Default: toolCallModeValue(def.ToolCallMode)},
		{Key: "xml-tag-prefix", Value: llm.XMLTagPrefix, Desc: i18n.T(i18n.KeySettingCmd_304), Type: "string", Default: def.XMLTagPrefix},
		{Key: "xml-stream-validate", Value: boolStr(llm.XMLStreamValidate), Desc: i18n.T(i18n.KeySettingCmd_305), Type: "bool", Default: boolStr(def.XMLStreamValidate)},
		{Key: "plan-enabled", Value: boolStr(llm.PlanEnabled), Desc: i18n.T(i18n.KeyCol3PlanEnabled), Type: "bool", Default: boolStr(def.PlanEnabled)},
		{Key: "intent-exposure-enabled", Value: boolStr(llm.IntentExposureEnabled), Desc: i18n.T(i18n.KeyCol3IntentExposureEnabled), Type: "bool", Default: boolStr(def.IntentExposureEnabled)},
		{Key: "meta-capability-enabled", Value: boolStr(llm.MetaCapabilityEnabled), Desc: i18n.T(i18n.KeyCol3MetaCapabilityEnabled), Type: "bool", Default: boolStr(def.MetaCapabilityEnabled)},
		{Key: "subagent-enabled", Value: boolStr(llm.SubAgentEnabled), Desc: i18n.T(i18n.KeyCol3SubAgentEnabled), Type: "bool", Default: boolStr(def.SubAgentEnabled)},
		{Key: "result-mode", Value: config.ResultModeString(config.ResultMode(llm.ResultMode)), Desc: i18n.T(i18n.KeyCol3ResultMode), Type: "enum", Options: []string{"minimal", "explain", "analyze", "free"}, Default: config.ResultModeString(config.ResultMode(def.ResultMode))},
		{Key: "shell-session-enabled", Value: boolStr(llm.ShellSessionEnabled), Desc: i18n.T(i18n.KeyCol3ShellSessionEnabled), Type: "bool", Default: boolStr(def.ShellSessionEnabled)},
		{Key: "shell-session-timeout", Value: strconv.Itoa(llm.ShellSessionTimeout), Desc: i18n.T(i18n.KeyCol3ShellSessionTimeout), Type: "number", Default: strconv.Itoa(def.ShellSessionTimeout)},
		{Key: "shell-vt-rows", Value: strconv.Itoa(shellVTRowsValue(llm.ShellVTRows)), Desc: i18n.T(i18n.KeySettingCmd_306), Type: "number", Default: strconv.Itoa(shellVTRowsValue(def.ShellVTRows))},
		{Key: "shell-vt-cols", Value: strconv.Itoa(shellVTColsValue(llm.ShellVTCols)), Desc: i18n.T(i18n.KeySettingCmd_307), Type: "number", Default: strconv.Itoa(shellVTColsValue(def.ShellVTCols))},
		{Key: "browser-enabled", Value: boolStr(llm.BrowserEnabled), Desc: i18n.T(i18n.KeyCol3BrowserEnabled), Type: "bool", Default: boolStr(def.BrowserEnabled)},
		{Key: "browser-port", Value: strconv.Itoa(llm.BrowserPort), Desc: i18n.T(i18n.KeyCol3BrowserPort), Type: "number", Default: strconv.Itoa(def.BrowserPort)},
		{Key: "browser-headless", Value: boolStr(llm.BrowserHeadless), Desc: i18n.T(i18n.KeyCol3BrowserHeadless), Type: "bool", Default: boolStr(def.BrowserHeadless)},
		{Key: "browser-max-html-size", Value: strconv.Itoa(llm.BrowserMaxHTMLSize), Desc: i18n.T(i18n.KeySettingCmd_308), Type: "number", Default: strconv.Itoa(def.BrowserMaxHTMLSize)},
		{Key: "excel-max-sessions", Value: strconv.Itoa(llm.ExcelMaxSessions), Desc: i18n.T(i18n.KeySettingCmd_309), Type: "number", Default: strconv.Itoa(def.ExcelMaxSessions)},
		{Key: "excel-max-cells", Value: strconv.Itoa(llm.ExcelMaxCells), Desc: i18n.T(i18n.KeySettingCmd_310), Type: "number", Default: strconv.Itoa(def.ExcelMaxCells)},
		{Key: "docx-max-sessions", Value: strconv.Itoa(llm.DocxMaxSessions), Desc: i18n.T(i18n.KeySettingCmd_311), Type: "number", Default: strconv.Itoa(def.DocxMaxSessions)},
		{Key: "docx-max-read-paras", Value: strconv.Itoa(llm.DocxMaxReadParas), Desc: i18n.T(i18n.KeySettingCmd_312), Type: "number", Default: strconv.Itoa(def.DocxMaxReadParas)},
		{Key: "visual-analysis-max-images", Value: strconv.Itoa(llm.VisualAnalysisMaxImages), Desc: i18n.T(i18n.KeyCol3VisualAnalysisMaxImages), Type: "number", Default: strconv.Itoa(def.VisualAnalysisMaxImages)},
		{Key: "search-max-line-length", Value: strconv.Itoa(llm.SearchMaxLineLength), Desc: i18n.T(i18n.KeyCol3SearchMaxLineLength), Type: "number", Default: strconv.Itoa(def.SearchMaxLineLength)},
		{Key: "search-max-result-bytes", Value: strconv.Itoa(llm.SearchMaxResultBytes), Desc: i18n.T(i18n.KeyCol3SearchMaxResultBytes), Type: "number", Default: strconv.Itoa(def.SearchMaxResultBytes)},
		{Key: "search-context-lines", Value: strconv.Itoa(llm.SearchContextLines), Desc: i18n.T(i18n.KeyCol3SearchContextLines), Type: "number", Default: strconv.Itoa(def.SearchContextLines)},
		{Key: "no-tool-action", Value: noToolActionValue(llm.NoToolAction), Desc: i18n.T(i18n.KeySettingCmd_313), Type: "enum", Options: []string{"exit", "retry", "prompt"}, Default: noToolActionValue(def.NoToolAction)},
		{Key: "parse-error-action", Value: parseErrorActionValue(llm.ParseErrorAction), Desc: i18n.T(i18n.KeySettingCmd_314), Type: "enum", Options: []string{"exit", "retry", "prompt"}, Default: parseErrorActionValue(def.ParseErrorAction)},
		// FEATURE-496: model connectivity pre-check strategy.
		{Key: "model-connectivity-check", Value: modelConnectivityCheckValue(llm.ModelConnectivityCheck), Desc: i18n.T(i18n.KeySettingCmd_780), Type: "enum", Options: []string{"off", "on_submit", "on_send"}, Default: modelConnectivityCheckValue(def.ModelConnectivityCheck)},
	}

	// Group 2: Appearance & Display (matches showSettingsHelp Group 3).
	// theme-mode is a frontend-local setting (stored in localStorage), rendered
	// specially by the Web UI (FEATURE-457).
	displayGroup := []WebSettingItem{
		{Key: "theme-mode", Value: "auto", Desc: "主题", Type: "enum", Options: []string{"auto", "dark", "light", "light-tp", "paper"}, Default: "auto"},
		// FEATURE-477: system logo. Rendered as a special upload block by the
		// Web UI (type "logo"). The target theme follows the current theme-mode
		// (dark/light/light-tp; auto resolves to the parsed theme), so only one
		// logo item is shown at a time.
		{Key: "logo", Value: "", Desc: "当前主题 logo（剪贴板粘贴上传）", Type: "logo"},
		{Key: "emoji-enabled", Value: boolStr(llm.EmojiEnabled), Desc: i18n.T(i18n.KeyCol3EmojiEnabled), Type: "bool", Default: boolStr(def.EmojiEnabled)},
		{Key: "show-llm-thinking", Value: boolStr(llm.ShowLlmThinking), Desc: i18n.T(i18n.KeyCol3LlmThinking), Type: "bool", Default: boolStr(def.ShowLlmThinking)},
		{Key: "show-llm-content", Value: boolStr(llm.ShowLlmContent), Desc: i18n.T(i18n.KeyCol3LlmContent), Type: "bool", Default: boolStr(def.ShowLlmContent)},
		{Key: "show-tool", Value: boolStr(llm.ShowTool), Desc: i18n.T(i18n.KeyCol3Tool), Type: "bool", Default: boolStr(def.ShowTool)},
		{Key: "show-tool-input", Value: boolStr(llm.ShowToolInput), Desc: i18n.T(i18n.KeyCol3ToolInput), Type: "bool", Default: boolStr(def.ShowToolInput)},
		{Key: "show-tool-output", Value: boolStr(llm.ShowToolOutput), Desc: i18n.T(i18n.KeyCol3ToolOutput), Type: "bool", Default: boolStr(def.ShowToolOutput)},
		{Key: "show-command", Value: boolStr(llm.ShowCommand), Desc: i18n.T(i18n.KeyCol3Command), Type: "bool", Default: boolStr(def.ShowCommand)},
		{Key: "show-command-output", Value: boolStr(llm.ShowCommandOutput), Desc: i18n.T(i18n.KeyCol3CommandOutput), Type: "bool", Default: boolStr(def.ShowCommandOutput)},
		{Key: "show-loop-detection", Value: boolStr(llm.ShowLoopDetection), Desc: i18n.T(i18n.KeyCol3ShowLoopDetection), Type: "bool", Default: boolStr(def.ShowLoopDetection)},
		{Key: "show-parse-error-raw", Value: boolStr(llm.ShowParseErrorRaw), Desc: i18n.T(i18n.KeySettingCmd_337), Type: "bool", Default: boolStr(def.ShowParseErrorRaw)},
		{Key: "token-usage", Value: tokenUsageValue(llm.TokenUsage), Desc: i18n.T(i18n.KeyCol3TokenUsage), Type: "enum", Options: []string{"on", "off", "none"}, Default: tokenUsageValue(def.TokenUsage)},
		// FEATURE-508: Web UI stream sliding-window thresholds.
		{Key: "stream-window-max-blocks", Value: strconv.Itoa(llm.StreamWindowMaxBlocks), Desc: i18n.T(i18n.KeyCol3StreamWindowMaxBlocks), Type: "number", Default: strconv.Itoa(def.StreamWindowMaxBlocks)},
		{Key: "stream-window-max-nodes", Value: strconv.Itoa(llm.StreamWindowMaxNodes), Desc: i18n.T(i18n.KeyCol3StreamWindowMaxNodes), Type: "number", Default: strconv.Itoa(def.StreamWindowMaxNodes)},
	}

	// Group 3: Safety & Confirmation (matches showSettingsHelp Group 4)
	safetyGroup := []WebSettingItem{
		{Key: "confirm-tool", Value: confirmToolValue(llm.ToolModes), Desc: i18n.T(i18n.KeyCol3Confirm), Type: "enum", Options: []string{"disabled", "auto", "confirm"}, Default: confirmToolValue(def.ToolModes)},
		{Key: "tool-timeout", Value: strconv.Itoa(llm.ToolTimeout), Desc: i18n.T(i18n.KeyCol3ToolTimeout), Type: "number", Default: strconv.Itoa(def.ToolTimeout)},
		{Key: "cmd-timeout", Value: strconv.Itoa(llm.CommandTimeout), Desc: i18n.T(i18n.KeyCol3CmdTimeout), Type: "number", Default: strconv.Itoa(def.CommandTimeout)},
		{Key: "llm-timeout", Value: strconv.Itoa(llm.LLMTimeout), Desc: i18n.T(i18n.KeyCol3LLMTimeout), Type: "number", Default: strconv.Itoa(def.LLMTimeout)},
		{Key: "error-max-single-count", Value: strconv.Itoa(llm.ErrorMaxSingleCount), Desc: i18n.T(i18n.KeyCol3ErrorMaxSingleCount), Type: "number", Default: strconv.Itoa(def.ErrorMaxSingleCount)},
		{Key: "error-max-type-count", Value: strconv.Itoa(llm.ErrorMaxTypeCount), Desc: i18n.T(i18n.KeyCol3ErrorMaxTypeCount), Type: "number", Default: strconv.Itoa(def.ErrorMaxTypeCount)},
		{Key: "loop-intervention", Value: loopInterventionValue(llm.LoopIntervention), Desc: i18n.T(i18n.KeySettingCmd_315), Type: "enum", Options: []string{"off", "retry", "prompt", "reorganize", "temperature", "random", "auto"}, Default: loopInterventionValue(def.LoopIntervention)},
		{Key: "loop-auto-reorganize-threshold", Value: strconv.Itoa(llm.LoopAutoReorganizeThreshold), Desc: i18n.T(i18n.KeyCol3LoopAutoReorgThresh), Type: "number", Default: strconv.Itoa(def.LoopAutoReorganizeThreshold)},
		{Key: "loop-detect-threshold", Value: strconv.Itoa(llm.LoopDetectThreshold), Desc: i18n.T(i18n.KeyCol3LoopDetectThreshold), Type: "number", Default: strconv.Itoa(def.LoopDetectThreshold)},
		{Key: "loop-temp-step-up", Value: fmt.Sprintf("%.2f", llm.LoopTempStepUp), Desc: i18n.T(i18n.KeySettingCmd_316), Type: "number", Default: fmt.Sprintf("%.2f", def.LoopTempStepUp)},
		{Key: "loop-temp-step-down", Value: fmt.Sprintf("%.2f", llm.LoopTempStepDown), Desc: i18n.T(i18n.KeySettingCmd_317), Type: "number", Default: fmt.Sprintf("%.2f", def.LoopTempStepDown)},
		{Key: "loop-temp-max", Value: fmt.Sprintf("%.2f", llm.LoopTempMax), Desc: i18n.T(i18n.KeySettingCmd_318), Type: "number", Default: fmt.Sprintf("%.2f", def.LoopTempMax)},
		{Key: "loop-temp-min", Value: fmt.Sprintf("%.2f", llm.LoopTempMin), Desc: i18n.T(i18n.KeySettingCmd_319), Type: "number", Default: fmt.Sprintf("%.2f", def.LoopTempMin)},
		{Key: "loop-judge-enabled", Value: boolStr(llm.LoopJudgeEnabled), Desc: i18n.T(i18n.KeyCol3LoopJudgeEnabled), Type: "bool", Default: boolStr(def.LoopJudgeEnabled)},
		{Key: "loop-history-fix-enabled", Value: boolStr(llm.LoopHistoryFixEnabled), Desc: i18n.T(i18n.KeyCol3LoopHistoryFixEnabled), Type: "bool", Default: boolStr(def.LoopHistoryFixEnabled)},
		{Key: "loop-history-fix-max-messages", Value: strconv.Itoa(llm.LoopHistoryFixMaxMessages), Desc: i18n.T(i18n.KeyCol3LoopHistoryFixMaxMsgs), Type: "number", Default: strconv.Itoa(def.LoopHistoryFixMaxMessages)},
		{Key: "loop-judge-timeout", Value: strconv.Itoa(llm.LoopJudgeTimeout), Desc: i18n.T(i18n.KeySettingCmd_320), Type: "number", Default: strconv.Itoa(def.LoopJudgeTimeout)},
		{Key: "loop-long-output-threshold", Value: strconv.Itoa(llm.LoopLongOutputThreshold), Desc: i18n.T(i18n.KeySettingCmd_321), Type: "number", Default: strconv.Itoa(def.LoopLongOutputThreshold)},
		{Key: "loop-single-line-length", Value: strconv.Itoa(llm.LoopSingleLineLength), Desc: i18n.T(i18n.KeySettingCmd_322), Type: "number", Default: strconv.Itoa(def.LoopSingleLineLength)},
		{Key: "loop-single-line-window", Value: strconv.Itoa(llm.LoopSingleLineWindow), Desc: i18n.T(i18n.KeySettingCmd_323), Type: "number", Default: strconv.Itoa(def.LoopSingleLineWindow)},
		{Key: "loop-single-line-block-limit", Value: strconv.Itoa(llm.LoopSingleLineBlockLimit), Desc: i18n.T(i18n.KeySettingCmd_324), Type: "number", Default: strconv.Itoa(def.LoopSingleLineBlockLimit)},
		{Key: "problem-solver-enabled", Value: boolStr(llm.ProblemSolverEnabled), Desc: i18n.T(i18n.KeyCol3ProblemSolverEnabled), Type: "bool", Default: boolStr(def.ProblemSolverEnabled)},
		{Key: "default-problem-model", Value: defaultModelValue(llm.DefaultProblemModelID), Desc: "auto/<model-id>", Type: "string", Default: defaultModelValue(def.DefaultProblemModelID)},
		{Key: "default-tool-model", Value: defaultModelValue(llm.DefaultToolModelID), Desc: "auto/<model-id>", Type: "string", Default: defaultModelValue(def.DefaultToolModelID)},
		// FEATURE-456: dedicated supervisor LLM switches.
		{Key: "supervisor-enabled", Value: boolStr(llm.Supervisor.Enabled), Desc: "监督 LLM 启用", Type: "bool", Default: boolStr(def.Supervisor.Enabled)},
		{Key: "supervisor-entry-object", Value: boolStr(llm.Supervisor.EntryObject), Desc: "介入点 object (attempt_completion)", Type: "bool", Default: boolStr(def.Supervisor.EntryObject)},
		{Key: "supervisor-entry-exit", Value: boolStr(llm.Supervisor.EntryExit), Desc: "介入点 exit (未调用工具退出)", Type: "bool", Default: boolStr(def.Supervisor.EntryExit)},
		{Key: "supervisor-entry-task", Value: boolStr(llm.Supervisor.EntryTask), Desc: "介入点 task (任务进度完成)", Type: "bool", Default: boolStr(def.Supervisor.EntryTask)},
		{Key: "supervisor-clear-context", Value: boolStr(llm.Supervisor.ClearContext), Desc: "每次审查清空上下文", Type: "bool", Default: boolStr(def.Supervisor.ClearContext)},
		{Key: "supervisor-max-retries", Value: strconv.Itoa(llm.Supervisor.MaxRetries), Desc: "最大打回次数", Type: "number", Default: strconv.Itoa(def.Supervisor.MaxRetries)},
		{Key: "supervisor-max-rounds", Value: strconv.Itoa(llm.Supervisor.MaxRounds), Desc: "单次审查最大迭代次数", Type: "number", Default: strconv.Itoa(def.Supervisor.MaxRounds)},
		{Key: "supervisor-allowed-tools", Value: supervisorAllowedToolsDisplay(llm), Desc: "工具白名单(逗号分隔)", Type: "string", Default: supervisorAllowedToolsDisplay(def)},
		// FEATURE-460: SUP LLM interaction streaming switches.
		{Key: "show-sup-prompt", Value: boolStr(llm.Supervisor.ShowSupPrompt), Desc: i18n.T(i18n.KeyCol3ShowSupPrompt), Type: "bool", Default: boolStr(def.Supervisor.ShowSupPrompt)},
		{Key: "show-sup-stream", Value: boolStr(llm.Supervisor.ShowSupStream), Desc: i18n.T(i18n.KeyCol3ShowSupStream), Type: "bool", Default: boolStr(def.Supervisor.ShowSupStream)},
		// FEATURE-479: attempt_completion completion-confirm behavior mode.
		{Key: "completion-mode", Value: completionModeDisplay(llm), Desc: i18n.T(i18n.KeyCol3CompletionMode), Type: "enum", Options: []string{"active", "simple", "exit"}, Default: completionModeDisplay(def)},
	}

	// Group 4: Memory & Context (matches showSettingsHelp Group 5)
	memoryGroup := []WebSettingItem{
		{Key: "memory-enabled", Value: boolStr(llm.MemoryEnabled), Desc: i18n.T(i18n.KeyCol3MemoryEnabled), Type: "bool", Default: boolStr(def.MemoryEnabled)},
		{Key: "context-limit", Value: strconv.Itoa(llm.ContextLimit), Desc: i18n.T(i18n.KeyCol3ContextLimit), Type: "number", Default: strconv.Itoa(def.ContextLimit)},
		{Key: "context-policy", Value: contextPolicyValue(llm.ContextPolicy), Desc: "window/task/smart/reorganize", Type: "enum", Options: []string{"window", "task", "smart", "reorganize"}, Default: contextPolicyValue(def.ContextPolicy)},
		{Key: "context-reorganize-threshold", Value: strconv.Itoa(llm.ContextReorganizeThreshold), Desc: "0-100%", Type: "number", Default: strconv.Itoa(def.ContextReorganizeThreshold)},
		{Key: "memory-search-max-content-len", Value: strconv.Itoa(llm.MemorySearchMaxContentLen), Desc: i18n.T(i18n.KeyCol3MemorySearchMaxContentLen), Type: "number", Default: strconv.Itoa(def.MemorySearchMaxContentLen)},
		{Key: "memory-search-max-results", Value: strconv.Itoa(llm.MemorySearchMaxResults), Desc: i18n.T(i18n.KeyCol3MemorySearchMaxResults), Type: "number", Default: strconv.Itoa(def.MemorySearchMaxResults)},
		// FEATURE-471: per-block <environment_details> inclusion switches.
		{Key: "env-include-details", Value: boolStr(llm.EnvIncludeDetails), Desc: i18n.T(i18n.KeyCol3EnvIncludeDetails), Type: "bool", Default: boolStr(def.EnvIncludeDetails)},
		{Key: "env-include-current-dir", Value: boolStr(llm.EnvIncludeCurrentDir), Desc: i18n.T(i18n.KeyCol3EnvIncludeCurrentDir), Type: "bool", Default: boolStr(def.EnvIncludeCurrentDir)},
		{Key: "env-include-tools", Value: boolStr(llm.EnvIncludeTools), Desc: i18n.T(i18n.KeyCol3EnvIncludeTools), Type: "bool", Default: boolStr(def.EnvIncludeTools)},
		{Key: "env-include-research", Value: boolStr(llm.EnvIncludeResearch), Desc: i18n.T(i18n.KeyCol3EnvIncludeResearch), Type: "bool", Default: boolStr(def.EnvIncludeResearch)},
		{Key: "env-include-user-dynamic", Value: boolStr(llm.EnvIncludeUserDynamic), Desc: i18n.T(i18n.KeyCol3EnvIncludeUserDynamic), Type: "bool", Default: boolStr(def.EnvIncludeUserDynamic)},
	}

	// Group 5: Developer (matches showSettingsHelp Group 6)
	developerGroup := []WebSettingItem{
		{Key: "debug", Value: boolStr(llm.DebugMode), Desc: i18n.T(i18n.KeyCol3Debug), Type: "bool", Default: boolStr(def.DebugMode)},
		{Key: "log", Value: log.LogLevelString(log.GetLevel()), Desc: i18n.T(i18n.KeyCol3Log), Type: "enum", Options: []string{"debug", "info", "warn", "error", "off"}, Default: log.LogLevelString(log.LogLevelInfo)},
		{Key: "llm-log", Value: boolStr(log.IsLLMInteractionEnabled()), Desc: i18n.T(i18n.KeyCol3LLMInteractionLog), Type: "bool", Default: boolStr(def.LLMInteractionLog)},
		{Key: "web-whitelist", Value: strings.Join(cfg.WebWhitelist, ","), Desc: i18n.T(i18n.KeyCol3WebWhitelist), Type: "string", Default: ""},
		{Key: "web-input-dir", Value: webInputDirValue(cfg), Desc: i18n.T(i18n.KeyCol3WebInputDir), Type: "string", Default: "input"},
		{Key: "dynamic-event-queue-size", Value: strconv.Itoa(dynamicQueueSizeValue(cfg)), Desc: i18n.T(i18n.KeyCol3DynamicQueueSize), Type: "number", Default: strconv.Itoa(100)},
	}

	return []WebSettingGroup{
		{Title: i18n.T(i18n.KeySettingsGroupModel), Items: agentGroup},
		{Title: i18n.T(i18n.KeySettingsGroupDisplay), Items: displayGroup},
		{Title: i18n.T(i18n.KeySettingsGroupSafety), Items: safetyGroup},
		{Title: i18n.T(i18n.KeySettingsGroupMemory), Items: memoryGroup},
		// FEATURE-464: MCP Server manager group (rendered by a dedicated UI).
		{Title: i18n.T(i18n.KeySettingsGroupMCP), Kind: "mcp"},
		{Title: i18n.T(i18n.KeySettingsGroupSearchDebug), Items: developerGroup},
	}
}

// thinkingEnabledValue normalizes the thinking-enabled value.
func thinkingEnabledValue(v string) string {
	if v == "" {
		return "default"
	}
	return v
}

// reasoningEffortValue normalizes the reasoning-effort value.
func reasoningEffortValue(v string) string {
	if v == "" {
		return "default"
	}
	return v
}

// tokenUsageValue normalizes the token-usage value.
func tokenUsageValue(v string) string {
	if v == "" {
		return "on"
	}
	return v
}

// contextPolicyValue normalizes the context policy.
func contextPolicyValue(v string) string {
	if v == "" {
		return "reorganize"
	}
	return v
}

// confirmToolValue returns the default tool confirmation mode.
func confirmToolValue(modes map[string]string) string {
	if v, ok := modes["default"]; ok && v != "" {
		return v
	}
	return "confirm"
}

// loopInterventionValue normalizes the loop intervention strategy.
func loopInterventionValue(v string) string {
	if v == "" {
		return "retry"
	}
	return v
}

// visionContextModeValue normalizes the vision context mode.
func visionContextModeValue(v string) string {
	if v == "" {
		return "minimal"
	}
	return v
}

// toolCallModeValue normalizes the tool call mode.
func toolCallModeValue(v string) string {
	if v == "" {
		return "openai"
	}
	return v
}

// noToolActionValue normalizes the no-tool-action strategy.
func noToolActionValue(v string) string {
	if v == "" {
		return "exit"
	}
	return v
}

// parseErrorActionValue normalizes the parse-error-action strategy.
func parseErrorActionValue(v string) string {
	if v == "" {
		return "retry"
	}
	return v
}

// modelConnectivityCheckValue normalizes the model connectivity check strategy.
func modelConnectivityCheckValue(v string) string {
	if v == "" {
		return "on_submit"
	}
	return v
}

// shellVTRowsValue normalizes the shell VT rows.
func shellVTRowsValue(v int) int {
	if v <= 0 {
		return 24
	}
	return v
}

// shellVTColsValue normalizes the shell VT cols.
func shellVTColsValue(v int) int {
	if v <= 0 {
		return 80
	}
	return v
}

// defaultModelValue normalizes a default model id (empty -> "auto").
func defaultModelValue(v string) string {
	if v == "" {
		return "auto"
	}
	return v
}

// webInputDirValue returns the effective Web UI attachment upload directory:
// the configured web-input-dir, or the default "input" when unset
// (FEATURE-469).
func webInputDirValue(cfg *config.Config) string {
	if cfg.WebInputDir == "" {
		return "input"
	}
	return cfg.WebInputDir
}

// dynamicQueueSizeValue returns the effective dynamic perception queue
// capacity: the configured dynamic-event-queue-size, or the default 100 when
// unset (FEATURE-471).
func dynamicQueueSizeValue(cfg *config.Config) int {
	if cfg.DynamicEventQueueSize > 0 {
		return cfg.DynamicEventQueueSize
	}
	return 100
}
