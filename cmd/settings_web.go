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
}

// WebSettingGroup is a titled group of setting items.
type WebSettingGroup struct {
	Title string           `json:"title"`
	Items []WebSettingItem `json:"items"`
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

	// Group 1: Agent Settings (matches showSettingsHelp Group 2)
	agentGroup := []WebSettingItem{
		{Key: "temperature", Value: fmt.Sprintf("%.1f", llm.Temperature), Desc: i18n.T(i18n.KeySettingCmd_301), Type: "number"},
		{Key: "max-tokens", Value: strconv.Itoa(llm.MaxTokens), Desc: i18n.T(i18n.KeySettingCmd_302), Type: "number"},
		{Key: "max-iterations", Value: strconv.Itoa(llm.MaxIterations), Desc: i18n.T(i18n.KeyCol3MaxIter), Type: "number"},
		{Key: "vision", Value: boolStr(llm.VisionSupport), Desc: i18n.T(i18n.KeyCol3Vision), Type: "bool"},
		{Key: "vision-context-mode", Value: visionContextModeValue(llm.VisionContextMode), Desc: i18n.T(i18n.KeySettingCmd_303), Type: "enum", Options: []string{"minimal", "full"}},
		{Key: "thinking-enabled", Value: thinkingEnabledValue(llm.ThinkingEnabled), Desc: "on/off/default", Type: "enum", Options: []string{"default", "on", "off"}},
		{Key: "reasoning-effort", Value: reasoningEffortValue(llm.ReasoningEffort), Desc: "low/medium/high/max/none/default", Type: "enum", Options: []string{"default", "none", "low", "medium", "high", "max"}},
		{Key: "toolcall-enabled", Value: boolStr(llm.ToolCallEnabled), Desc: i18n.T(i18n.KeyCol3ToolCallEnabled), Type: "bool"},
		{Key: "toolcall-mode", Value: toolCallModeValue(llm.ToolCallMode), Desc: i18n.T(i18n.KeyCol3ToolCallMode), Type: "enum", Options: []string{"openai", "xml"}},
		{Key: "xml-tag-prefix", Value: llm.XMLTagPrefix, Desc: i18n.T(i18n.KeySettingCmd_304), Type: "string"},
		{Key: "xml-stream-validate", Value: boolStr(llm.XMLStreamValidate), Desc: i18n.T(i18n.KeySettingCmd_305), Type: "bool"},
		{Key: "plan-enabled", Value: boolStr(llm.PlanEnabled), Desc: i18n.T(i18n.KeyCol3PlanEnabled), Type: "bool"},
		{Key: "subagent-enabled", Value: boolStr(llm.SubAgentEnabled), Desc: i18n.T(i18n.KeyCol3SubAgentEnabled), Type: "bool"},
		{Key: "result-mode", Value: config.ResultModeString(config.ResultMode(llm.ResultMode)), Desc: i18n.T(i18n.KeyCol3ResultMode), Type: "enum", Options: []string{"minimal", "explain", "analyze", "free"}},
		{Key: "shell-session-enabled", Value: boolStr(llm.ShellSessionEnabled), Desc: i18n.T(i18n.KeyCol3ShellSessionEnabled), Type: "bool"},
		{Key: "shell-session-timeout", Value: strconv.Itoa(llm.ShellSessionTimeout), Desc: i18n.T(i18n.KeyCol3ShellSessionTimeout), Type: "number"},
		{Key: "shell-vt-rows", Value: strconv.Itoa(shellVTRowsValue(llm.ShellVTRows)), Desc: i18n.T(i18n.KeySettingCmd_306), Type: "number"},
		{Key: "shell-vt-cols", Value: strconv.Itoa(shellVTColsValue(llm.ShellVTCols)), Desc: i18n.T(i18n.KeySettingCmd_307), Type: "number"},
		{Key: "browser-enabled", Value: boolStr(llm.BrowserEnabled), Desc: i18n.T(i18n.KeyCol3BrowserEnabled), Type: "bool"},
		{Key: "browser-port", Value: strconv.Itoa(llm.BrowserPort), Desc: i18n.T(i18n.KeyCol3BrowserPort), Type: "number"},
		{Key: "browser-headless", Value: boolStr(llm.BrowserHeadless), Desc: i18n.T(i18n.KeyCol3BrowserHeadless), Type: "bool"},
		{Key: "browser-max-html-size", Value: strconv.Itoa(llm.BrowserMaxHTMLSize), Desc: i18n.T(i18n.KeySettingCmd_308), Type: "number"},
		{Key: "excel-max-sessions", Value: strconv.Itoa(llm.ExcelMaxSessions), Desc: i18n.T(i18n.KeySettingCmd_309), Type: "number"},
		{Key: "excel-max-cells", Value: strconv.Itoa(llm.ExcelMaxCells), Desc: i18n.T(i18n.KeySettingCmd_310), Type: "number"},
		{Key: "docx-max-sessions", Value: strconv.Itoa(llm.DocxMaxSessions), Desc: i18n.T(i18n.KeySettingCmd_311), Type: "number"},
		{Key: "docx-max-read-paras", Value: strconv.Itoa(llm.DocxMaxReadParas), Desc: i18n.T(i18n.KeySettingCmd_312), Type: "number"},
		{Key: "visual-analysis-max-images", Value: strconv.Itoa(llm.VisualAnalysisMaxImages), Desc: i18n.T(i18n.KeyCol3VisualAnalysisMaxImages), Type: "number"},
		{Key: "search-max-line-length", Value: strconv.Itoa(llm.SearchMaxLineLength), Desc: i18n.T(i18n.KeyCol3SearchMaxLineLength), Type: "number"},
		{Key: "search-max-result-bytes", Value: strconv.Itoa(llm.SearchMaxResultBytes), Desc: i18n.T(i18n.KeyCol3SearchMaxResultBytes), Type: "number"},
		{Key: "search-context-lines", Value: strconv.Itoa(llm.SearchContextLines), Desc: i18n.T(i18n.KeyCol3SearchContextLines), Type: "number"},
		{Key: "no-tool-action", Value: noToolActionValue(llm.NoToolAction), Desc: i18n.T(i18n.KeySettingCmd_313), Type: "enum", Options: []string{"exit", "retry", "prompt"}},
		{Key: "parse-error-action", Value: parseErrorActionValue(llm.ParseErrorAction), Desc: i18n.T(i18n.KeySettingCmd_314), Type: "enum", Options: []string{"exit", "retry", "prompt"}},
	}

	// Group 2: Display & Output (matches showSettingsHelp Group 3)
	displayGroup := []WebSettingItem{
		{Key: "emoji-enabled", Value: boolStr(llm.EmojiEnabled), Desc: i18n.T(i18n.KeyCol3EmojiEnabled), Type: "bool"},
		{Key: "show-llm-thinking", Value: boolStr(llm.ShowLlmThinking), Desc: i18n.T(i18n.KeyCol3LlmThinking), Type: "bool"},
		{Key: "show-llm-content", Value: boolStr(llm.ShowLlmContent), Desc: i18n.T(i18n.KeyCol3LlmContent), Type: "bool"},
		{Key: "show-tool", Value: boolStr(llm.ShowTool), Desc: i18n.T(i18n.KeyCol3Tool), Type: "bool"},
		{Key: "show-tool-input", Value: boolStr(llm.ShowToolInput), Desc: i18n.T(i18n.KeyCol3ToolInput), Type: "bool"},
		{Key: "show-tool-output", Value: boolStr(llm.ShowToolOutput), Desc: i18n.T(i18n.KeyCol3ToolOutput), Type: "bool"},
		{Key: "show-command", Value: boolStr(llm.ShowCommand), Desc: i18n.T(i18n.KeyCol3Command), Type: "bool"},
		{Key: "show-command-output", Value: boolStr(llm.ShowCommandOutput), Desc: i18n.T(i18n.KeyCol3CommandOutput), Type: "bool"},
		{Key: "show-loop-detection", Value: boolStr(llm.ShowLoopDetection), Desc: i18n.T(i18n.KeyCol3ShowLoopDetection), Type: "bool"},
		{Key: "show-parse-error-raw", Value: boolStr(llm.ShowParseErrorRaw), Desc: i18n.T(i18n.KeySettingCmd_337), Type: "bool"},
		{Key: "token-usage", Value: tokenUsageValue(llm.TokenUsage), Desc: i18n.T(i18n.KeyCol3TokenUsage), Type: "enum", Options: []string{"on", "off", "none"}},
	}

	// Group 3: Safety & Confirmation (matches showSettingsHelp Group 4)
	safetyGroup := []WebSettingItem{
		{Key: "confirm-tool", Value: confirmToolValue(llm.ToolModes), Desc: i18n.T(i18n.KeyCol3Confirm), Type: "enum", Options: []string{"disabled", "auto", "confirm"}},
		{Key: "tool-timeout", Value: strconv.Itoa(llm.ToolTimeout), Desc: i18n.T(i18n.KeyCol3ToolTimeout), Type: "number"},
		{Key: "cmd-timeout", Value: strconv.Itoa(llm.CommandTimeout), Desc: i18n.T(i18n.KeyCol3CmdTimeout), Type: "number"},
		{Key: "llm-timeout", Value: strconv.Itoa(llm.LLMTimeout), Desc: i18n.T(i18n.KeyCol3LLMTimeout), Type: "number"},
		{Key: "error-max-single-count", Value: strconv.Itoa(llm.ErrorMaxSingleCount), Desc: i18n.T(i18n.KeyCol3ErrorMaxSingleCount), Type: "number"},
		{Key: "error-max-type-count", Value: strconv.Itoa(llm.ErrorMaxTypeCount), Desc: i18n.T(i18n.KeyCol3ErrorMaxTypeCount), Type: "number"},
		{Key: "loop-intervention", Value: loopInterventionValue(llm.LoopIntervention), Desc: i18n.T(i18n.KeySettingCmd_315), Type: "enum", Options: []string{"off", "retry", "prompt", "reorganize", "temperature", "random", "auto"}},
		{Key: "loop-auto-reorganize-threshold", Value: strconv.Itoa(llm.LoopAutoReorganizeThreshold), Desc: i18n.T(i18n.KeyCol3LoopAutoReorgThresh), Type: "number"},
		{Key: "loop-detect-threshold", Value: strconv.Itoa(llm.LoopDetectThreshold), Desc: i18n.T(i18n.KeyCol3LoopDetectThreshold), Type: "number"},
		{Key: "loop-temp-step-up", Value: fmt.Sprintf("%.2f", llm.LoopTempStepUp), Desc: i18n.T(i18n.KeySettingCmd_316), Type: "number"},
		{Key: "loop-temp-step-down", Value: fmt.Sprintf("%.2f", llm.LoopTempStepDown), Desc: i18n.T(i18n.KeySettingCmd_317), Type: "number"},
		{Key: "loop-temp-max", Value: fmt.Sprintf("%.2f", llm.LoopTempMax), Desc: i18n.T(i18n.KeySettingCmd_318), Type: "number"},
		{Key: "loop-temp-min", Value: fmt.Sprintf("%.2f", llm.LoopTempMin), Desc: i18n.T(i18n.KeySettingCmd_319), Type: "number"},
		{Key: "loop-judge-enabled", Value: boolStr(llm.LoopJudgeEnabled), Desc: i18n.T(i18n.KeyCol3LoopJudgeEnabled), Type: "bool"},
		{Key: "loop-history-fix-enabled", Value: boolStr(llm.LoopHistoryFixEnabled), Desc: i18n.T(i18n.KeyCol3LoopHistoryFixEnabled), Type: "bool"},
		{Key: "loop-judge-timeout", Value: strconv.Itoa(llm.LoopJudgeTimeout), Desc: i18n.T(i18n.KeySettingCmd_320), Type: "number"},
		{Key: "loop-long-output-threshold", Value: strconv.Itoa(llm.LoopLongOutputThreshold), Desc: i18n.T(i18n.KeySettingCmd_321), Type: "number"},
		{Key: "loop-single-line-length", Value: strconv.Itoa(llm.LoopSingleLineLength), Desc: i18n.T(i18n.KeySettingCmd_322), Type: "number"},
		{Key: "loop-single-line-window", Value: strconv.Itoa(llm.LoopSingleLineWindow), Desc: i18n.T(i18n.KeySettingCmd_323), Type: "number"},
		{Key: "loop-single-line-block-limit", Value: strconv.Itoa(llm.LoopSingleLineBlockLimit), Desc: i18n.T(i18n.KeySettingCmd_324), Type: "number"},
		{Key: "problem-solver-enabled", Value: boolStr(llm.ProblemSolverEnabled), Desc: i18n.T(i18n.KeyCol3ProblemSolverEnabled), Type: "bool"},
		{Key: "default-problem-model", Value: defaultModelValue(llm.DefaultProblemModelID), Desc: "auto/<model-id>", Type: "string"},
		{Key: "default-tool-model", Value: defaultModelValue(llm.DefaultToolModelID), Desc: "auto/<model-id>", Type: "string"},
	}

	// Group 4: Memory & Context (matches showSettingsHelp Group 5)
	memoryGroup := []WebSettingItem{
		{Key: "memory-enabled", Value: boolStr(llm.MemoryEnabled), Desc: i18n.T(i18n.KeyCol3MemoryEnabled), Type: "bool"},
		{Key: "context-limit", Value: strconv.Itoa(llm.ContextLimit), Desc: i18n.T(i18n.KeyCol3ContextLimit), Type: "number"},
		{Key: "context-policy", Value: contextPolicyValue(llm.ContextPolicy), Desc: "window/task/smart/reorganize", Type: "enum", Options: []string{"window", "task", "smart", "reorganize"}},
		{Key: "context-reorganize-threshold", Value: strconv.Itoa(llm.ContextReorganizeThreshold), Desc: "0-100%", Type: "number"},
		{Key: "memory-search-max-content-len", Value: strconv.Itoa(llm.MemorySearchMaxContentLen), Desc: i18n.T(i18n.KeyCol3MemorySearchMaxContentLen), Type: "number"},
		{Key: "memory-search-max-results", Value: strconv.Itoa(llm.MemorySearchMaxResults), Desc: i18n.T(i18n.KeyCol3MemorySearchMaxResults), Type: "number"},
	}

	// Group 5: Developer (matches showSettingsHelp Group 6)
	developerGroup := []WebSettingItem{
		{Key: "debug", Value: boolStr(llm.DebugMode), Desc: i18n.T(i18n.KeyCol3Debug), Type: "bool"},
		{Key: "log", Value: log.LogLevelString(log.GetLevel()), Desc: i18n.T(i18n.KeyCol3Log), Type: "enum", Options: []string{"debug", "info", "warn", "error", "off"}},
		{Key: "web-whitelist", Value: strings.Join(cfg.WebWhitelist, ","), Desc: i18n.T(i18n.KeyCol3WebWhitelist), Type: "string"},
	}

	return []WebSettingGroup{
		{Title: i18n.T(i18n.KeySettingsGroupModel), Items: agentGroup},
		{Title: i18n.T(i18n.KeySettingsGroupDisplay), Items: displayGroup},
		{Title: i18n.T(i18n.KeySettingsGroupSafety), Items: safetyGroup},
		{Title: i18n.T(i18n.KeySettingsGroupMemory), Items: memoryGroup},
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
