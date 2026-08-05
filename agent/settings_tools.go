// Author: L.Shuang
// Created: 2026-05-03
// Last Modified: 2026-05-03
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
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// updateSettingsTool handles the "update_settings" tool call from the LLM.
// It allows the LLM to modify system configuration parameters.
// Before applying changes, it prompts the user for confirmation.
func (a *Agent) updateSettingsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract parameters
	settings, ok := args["settings"].([]interface{})
	if !ok {
		return "", fmt.Errorf("settings argument must be an array")
	}

	if len(settings) == 0 {
		return "", fmt.Errorf("settings array is empty")
	}

	// Parse all requested changes
	type settingChange struct {
		param    string
		oldValue string
		newValue string
		reason   string
	}

	var changes []settingChange
	var changeDescs []string

	for i, s := range settings {
		setting, ok := s.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("setting #%d must be an object", i+1)
		}

		param, _ := setting["param"].(string)
		value, _ := setting["value"].(string)
		reason, _ := setting["reason"].(string)

		if param == "" {
			return "", fmt.Errorf("setting #%d: param is required", i+1)
		}
		if value == "" {
			return "", fmt.Errorf("setting #%d: value is required", i+1)
		}

		// Get the old value for display
		oldValue := getSettingValue(a.cfg, param)

		changes = append(changes, settingChange{
			param:    param,
			oldValue: oldValue,
			newValue: value,
			reason:   reason,
		})

		changeDescs = append(changeDescs, fmt.Sprintf("  • %s: %s → %s", param, oldValue, value))
	}

	// Build the confirmation prompt
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println(i18n.T(i18n.KeySettingsConfirmTitle))
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()

	for _, c := range changes {
		fmt.Printf("📌 %s\n", c.param)
		fmt.Printf(i18n.T(i18n.KeySettingCmd_525), c.oldValue)
		fmt.Printf(i18n.T(i18n.KeySettingCmd_526), c.newValue)
		if c.reason != "" {
			fmt.Printf(i18n.T(i18n.KeySettingCmd_527), c.reason)
		}
		fmt.Println()
	}

	fmt.Println(i18n.T(i18n.KeySettingsConfirmRiskWarning))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeySettingsConfirmPrompt))
	fmt.Println()

	// Read user input via UserIO interface
	io := a.defaultIO()
	response, err := io.ReadLine()
	if err != nil {
		response = ""
	}
	response = strings.TrimSpace(response)
	lower := strings.ToLower(response)

	if lower == "c" {
		// User rejected
		fmt.Println()
		fmt.Println("❌ " + i18n.T(i18n.KeySettingsConfirmRejected))
		fmt.Println()
		return i18n.T(i18n.KeySettingsConfirmRejectedResult), nil
	}

	if response == "" || lower == "a" {
		// User approved - apply all changes
		var applied []string
		var failed []string

		for _, c := range changes {
			if err := applySetting(a, c.param, c.newValue); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", c.param, err))
			} else {
				applied = append(applied, c.param)
			}
		}

		fmt.Println()
		if len(applied) > 0 {
			fmt.Printf("✅ %s\n", i18n.T(i18n.KeySettingsConfirmApplied))
			for _, p := range applied {
				fmt.Printf("  • %s\n", p)
			}
		}
		if len(failed) > 0 {
			fmt.Printf("❌ %s\n", i18n.T(i18n.KeySettingsConfirmFailed))
			for _, f := range failed {
				fmt.Printf("  • %s\n", f)
			}
		}
		fmt.Println()

		// Build result message
		result := i18n.TF(i18n.KeySettingsConfirmResult, len(applied), len(failed))
		if len(applied) > 0 {
			result += "\n" + i18n.T(i18n.KeySettingsConfirmApplied) + ": " + strings.Join(applied, ", ")
		}
		if len(failed) > 0 {
			result += "\n" + i18n.T(i18n.KeySettingsConfirmFailed) + ": " + strings.Join(failed, "; ")
		}
		return result, nil
	}

	// Any other input is treated as supplementary instructions for the LLM
	fmt.Println()
	fmt.Printf("🔄 %s: %s\n", i18n.T(i18n.KeySettingsConfirmPaused), response)
	fmt.Println()
	return "", fmt.Errorf("USER_MODIFY_REQUEST: %s", response)
}

// getSettingValue returns the current string representation of a setting value.
func getSettingValue(cfg *config.Config, param string) string {
	activeModel := config.GetActiveModelFromConfig(cfg)
	switch param {
	case "api-key":
		if activeModel != nil {
			return maskKey(activeModel.APIKey)
		}
		return "(not set)"
	case "endpoint":
		if activeModel != nil {
			return activeModel.Endpoint
		}
		return "(not set)"
	case "model":
		if activeModel != nil {
			return activeModel.Model
		}
		return "(not set)"
	case "temperature":
		return fmt.Sprintf("%.1f", cfg.LLM.Temperature)
	case "max-tokens":
		return fmt.Sprintf("%d", cfg.LLM.MaxTokens)
	case "max-iterations":
		return fmt.Sprintf("%d", cfg.LLM.MaxIterations)
	case "max-retries":
		return fmt.Sprintf("%d", cfg.LLM.MaxRetries)
	case "show-llm-thinking":
		return boolToString(cfg.LLM.ShowLlmThinking)
	case "show-llm-content":
		return boolToString(cfg.LLM.ShowLlmContent)
	case "show-tool":
		return boolToString(cfg.LLM.ShowTool)
	case "show-tool-input":
		return boolToString(cfg.LLM.ShowToolInput)
	case "show-tool-output":
		return boolToString(cfg.LLM.ShowToolOutput)
	case "show-command":
		return boolToString(cfg.LLM.ShowCommand)
	case "show-command-output":
		return boolToString(cfg.LLM.ShowCommandOutput)
	case "confirm-tool":
		confirmDefault := "confirm"
		if v, ok := cfg.LLM.ToolModes["default"]; ok {
			confirmDefault = v
		}
		return confirmDefault
	case "result-mode":

		return config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))
	case "vision":
		return boolToString(cfg.LLM.VisionSupport)
	case "vision-context-mode":
		mode := cfg.LLM.VisionContextMode
		if mode == "" {
			mode = "minimal"
		}
		return mode
	case "thinking-enabled":
		return cfg.LLM.ThinkingEnabled
	case "reasoning-effort":
		return cfg.LLM.ReasoningEffort
	case "memory-enabled":
		return boolToString(cfg.LLM.MemoryEnabled)
	case "plan-enabled":
		return boolToString(cfg.LLM.PlanEnabled)
	case "subagent-enabled":
		return boolToString(cfg.LLM.SubAgentEnabled)
	case "context-limit":
		if cfg.LLM.ContextLimit == 0 {
			return "off"
		} else if cfg.LLM.ContextLimit == -1 {
			return "unlimited"
		}
		return fmt.Sprintf("%d", cfg.LLM.ContextLimit)
	case "name":
		if cfg.LLM.AgentName == "" {
			return "co-shell"
		}
		return cfg.LLM.AgentName
	case "description":
		if cfg.LLM.AgentDescription == "" {
			return "(not set)"
		}
		return cfg.LLM.AgentDescription
	case "principles":
		if cfg.LLM.AgentPrinciples == "" {
			return "(not set)"
		}
		return cfg.LLM.AgentPrinciples
	case "tool-timeout":
		if cfg.LLM.ToolTimeout <= 0 {
			return "unlimited"
		}
		return fmt.Sprintf("%ds", cfg.LLM.ToolTimeout)
	case "cmd-timeout":
		if cfg.LLM.CommandTimeout <= 0 {
			return "unlimited"
		}
		return fmt.Sprintf("%ds", cfg.LLM.CommandTimeout)
	case "llm-timeout":
		if cfg.LLM.LLMTimeout <= 0 {
			return "unlimited"
		}
		return fmt.Sprintf("%ds", cfg.LLM.LLMTimeout)
	case "log":
		return cfg.LogLevel
	case "search-max-line-length":
		return fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength)
	case "search-max-result-bytes":
		return fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes)
	case "search-context-lines":
		return fmt.Sprintf("%d", cfg.LLM.SearchContextLines)
	case "memory-search-max-content-len":
		return fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen)
	case "memory-search-max-results":
		return fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults)
	case "error-max-single-count":
		return fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount)
	case "error-max-type-count":
		return fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount)
	case "emoji-enabled":
		return boolToString(cfg.LLM.EmojiEnabled)
	case "top-p":
		return fmt.Sprintf("%.1f", cfg.LLM.TopP)
	case "top-k":
		return fmt.Sprintf("%d", cfg.LLM.TopK)
	case "repetition-penalty":
		return fmt.Sprintf("%.1f", cfg.LLM.RepetitionPenalty)
	case "context-start":
		switch cfg.LLM.ContextPolicy {
		case "window":
			return i18n.T(i18n.KeyContextStartWindow)
		case "smart":
			return i18n.T(i18n.KeyContextStartSmart)
		default:
			return i18n.T(i18n.KeyContextStartTask)
		}
	case "db-enabled":
		return boolToString(cfg.DB.Enabled)
	case "db-host":
		return cfg.DB.Host
	case "db-port":
		return fmt.Sprintf("%d", cfg.DB.Port)
	case "db-name":
		return cfg.DB.DBName
	case "db-schema":
		return cfg.DB.Schema
	case "db-user":
		return cfg.DB.User
	case "db-password":
		return "****"
	default:
		return "(unknown)"
	}

}

// applySetting applies a setting change to the config and syncs to the agent.
func applySetting(a *Agent, param, value string) error {
	cfg := a.cfg

	switch param {
	case "api-key":
		activeModel := config.GetActiveModelFromConfig(cfg)
		if activeModel != nil {
			activeModel.APIKey = value
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("API key updated via LLM tool")

	case "endpoint":
		activeModel := config.GetActiveModelFromConfig(cfg)
		if activeModel != nil {
			activeModel.Endpoint = value
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Endpoint updated via LLM tool: %s", value)

	case "model":
		activeModel := config.GetActiveModelFromConfig(cfg)
		if activeModel != nil {
			activeModel.Model = value
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Model updated via LLM tool: %s", value)

	case "temperature":
		temp, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid temperature value: %s", value)
		}
		if temp < 0 || temp > 2 {
			return fmt.Errorf("temperature must be between 0.0 and 2.0")
		}
		cfg.LLM.Temperature = temp
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Temperature set via LLM tool: %.1f", temp)

	case "max-tokens":
		tokens, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid token count: %s", value)
		}
		if tokens < 1 || tokens > 128000 {
			return fmt.Errorf("max-tokens must be between 1 and 128000")
		}
		cfg.LLM.MaxTokens = tokens
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Max tokens set via LLM tool: %d", tokens)

	case "top-p":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid top-p value: %s", value)
		}
		if val < 0 || val > 1 {
			return fmt.Errorf("top-p must be between 0.0 and 1.0, or -1 to disable")
		}
		cfg.LLM.TopP = val
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Top-P set via LLM tool: %.1f", val)

	case "top-k":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid top-k value: %s", value)
		}
		if val < 1 && val != -1 {
			return fmt.Errorf("top-k must be >= 1, or -1 to disable")
		}
		cfg.LLM.TopK = val
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Top-K set via LLM tool: %d", val)

	case "repetition-penalty":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid repetition-penalty value: %s", value)
		}
		if val < 0 || val > 2 {
			return fmt.Errorf("repetition-penalty must be between 0.0 and 2.0, or -1 to disable")
		}
		cfg.LLM.RepetitionPenalty = val
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Repetition penalty set via LLM tool: %.1f", val)

	case "max-iterations":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid iteration count: %s", value)
		}
		if n < -1 || n == 0 {
			return fmt.Errorf("max-iterations must be >= 1, or -1 (unlimited)")
		}
		cfg.LLM.MaxIterations = n
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetMaxIterations(n)
		log.Info("Max iterations set via LLM tool: %d", n)

	case "max-retries":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid retry count: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("max-retries must be >= 0")
		}
		cfg.LLM.MaxRetries = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Max retries set via LLM tool: %d", n)

	case "show-llm-thinking":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowLlmThinking = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowLlmThinking(b)
		log.Info("Show LLM thinking set via LLM tool: %v", b)

	case "show-llm-content":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowLlmContent = b
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Show LLM content set via LLM tool: %v", b)

	case "show-tool":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowTool = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowTool(b)
		log.Info("Show tool set via LLM tool: %v", b)

	case "show-tool-input":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowToolInput = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowToolInput(b)
		log.Info("Show tool input set via LLM tool: %v", b)

	case "show-tool-output":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowToolOutput = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowToolOutput(b)
		log.Info("Show tool output set via LLM tool: %v", b)

	case "show-command":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowCommand = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowCommand(b)
		log.Info("Show command set via LLM tool: %v", b)

	case "show-command-output":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ShowCommandOutput = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetShowCommandOutput(b)
		log.Info("Show command output set via LLM tool: %v", b)

	case "confirm-tool":
		// Accept both boolean (on/off) and mode (confirm/auto/disabled) values
		mode := value
		switch strings.ToLower(value) {
		case "on", "1", "true", "yes":
			mode = "confirm"
		case "off", "0", "false", "no":
			mode = "auto"
		}
		if mode != "confirm" && mode != "auto" && mode != "disabled" {
			return fmt.Errorf("invalid confirm-tool value: %s (valid: on/off, confirm/auto/disabled)", value)
		}
		if cfg.LLM.ToolModes == nil {
			cfg.LLM.ToolModes = make(map[string]string)
		}
		cfg.LLM.ToolModes["default"] = mode
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetToolMode("", mode)
		log.Info("Confirm tool set via LLM tool: %s", mode)

	case "result-mode":

		mode, ok := config.ParseResultMode(value)
		if !ok {
			return fmt.Errorf("invalid result mode: %s (valid: minimal, explain, analyze, free)", value)
		}
		cfg.LLM.ResultMode = int(mode)
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetResultMode(mode)
		log.Info("Result mode set via LLM tool: %s", value)

	case "vision":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.VisionSupport = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Vision support set via LLM tool: %v", b)

	case "vision-context-mode":
		switch value {
		case "minimal", "full":
			cfg.LLM.VisionContextMode = value
		default:
			return fmt.Errorf("invalid vision-context-mode value: %s (valid: minimal/full)", value)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Vision context mode set via LLM tool: %s", value)

	case "thinking-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		if b {
			cfg.LLM.ThinkingEnabled = "on"
		} else {
			cfg.LLM.ThinkingEnabled = "off"
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Thinking enabled set via LLM tool: %v", b)

	case "reasoning-effort":
		switch value {
		case "low", "medium", "high":
			cfg.LLM.ReasoningEffort = value
		default:
			return fmt.Errorf("invalid reasoning effort: %s (valid: low, medium, high)", value)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Reasoning effort set via LLM tool: %s", value)

	case "memory-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.MemoryEnabled = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetMemoryEnabled(b)
		log.Info("Memory enabled set via LLM tool: %v", b)

	case "plan-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.PlanEnabled = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetPlanEnabled(b)
		log.Info("Plan enabled set via LLM tool: %v", b)

	case "subagent-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.SubAgentEnabled = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetSubAgentEnabled(b)
		log.Info("SubAgent enabled set via LLM tool: %v", b)

	case "context-limit":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid context limit: %s", value)
		}
		if n < -1 {
			return fmt.Errorf("context-limit must be >= -1")
		}
		cfg.LLM.ContextLimit = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Context limit set via LLM tool: %d", n)

	case "name":
		cfg.LLM.AgentName = value
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetName(value)
		log.Info("Agent name set via LLM tool: %s", value)

	case "description":
		cfg.LLM.AgentDescription = value
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetConfig(cfg)
		log.Info("Agent description set via LLM tool: %s", value)

	case "principles":
		cfg.LLM.AgentPrinciples = value
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetConfig(cfg)
		log.Info("Agent principles set via LLM tool: %s", value)

	case "tool-timeout":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid tool timeout: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("tool-timeout must be >= 0")
		}
		cfg.LLM.ToolTimeout = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Tool timeout set via LLM tool: %d", n)

	case "cmd-timeout":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid command timeout: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("cmd-timeout must be >= 0")
		}
		cfg.LLM.CommandTimeout = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Command timeout set via LLM tool: %d", n)

	case "llm-timeout":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid LLM timeout: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("llm-timeout must be >= 0")
		}
		cfg.LLM.LLMTimeout = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("LLM timeout set via LLM tool: %d", n)

	case "log":
		level, ok := log.ParseLogLevel(value)
		if !ok {
			return fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error, off)", value)
		}
		cfg.LogLevel = value
		cfg.LogEnabled = level != log.LogLevelOff
		if err := cfg.Save(); err != nil {
			return err
		}
		log.SetLevel(level)
		if err := log.SetEnabled(cfg.LogEnabled); err != nil {
			return fmt.Errorf("failed to update logger: %w", err)
		}
		log.Info("Log level set via LLM tool: %s", value)

	case "search-max-line-length":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("search-max-line-length must be >= 0")
		}
		cfg.LLM.SearchMaxLineLength = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Search max line length set via LLM tool: %d", n)

	case "search-max-result-bytes":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("search-max-result-bytes must be >= 0")
		}
		cfg.LLM.SearchMaxResultBytes = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Search max result bytes set via LLM tool: %d", n)

	case "search-context-lines":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("search-context-lines must be >= 0")
		}
		cfg.LLM.SearchContextLines = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Search context lines set via LLM tool: %d", n)

	case "memory-search-max-content-len":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("memory-search-max-content-len must be >= 0")
		}
		cfg.LLM.MemorySearchMaxContentLen = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Memory search max content len set via LLM tool: %d", n)

	case "memory-search-max-results":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("memory-search-max-results must be >= 0")
		}
		cfg.LLM.MemorySearchMaxResults = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Memory search max results set via LLM tool: %d", n)

	case "error-max-single-count":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("error-max-single-count must be >= 0")
		}
		cfg.LLM.ErrorMaxSingleCount = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Error max single count set via LLM tool: %d", n)

	case "error-max-type-count":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value: %s", value)
		}
		if n < 0 {
			return fmt.Errorf("error-max-type-count must be >= 0")
		}
		cfg.LLM.ErrorMaxTypeCount = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("Error max type count set via LLM tool: %d", n)

	case "emoji-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.EmojiEnabled = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetEmojiEnabled(b)
		log.Info("Emoji enabled set via LLM tool: %v", b)

	case "context-start":
		switch value {
		case "window", "task", "smart":
			cfg.LLM.ContextPolicy = value
		default:
			return fmt.Errorf("invalid context-start mode: %s (valid: window, task, smart)", value)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		modeDesc := i18n.T(i18n.KeyContextStartTask)
		switch value {
		case "window":
			modeDesc = i18n.T(i18n.KeyContextStartWindow)
		case "smart":
			modeDesc = i18n.T(i18n.KeyContextStartSmart)
		}
		log.Info("Context start mode set via LLM tool: %s (%s)", value, modeDesc)

	case "db-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.DB.Enabled = b
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB enabled set via LLM tool: %v", b)

	case "db-host":
		cfg.DB.Host = value
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB host set via LLM tool: %s", value)

	case "db-port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid port: %s", value)
		}
		if n < 1 || n > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		cfg.DB.Port = n
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB port set via LLM tool: %d", n)

	case "db-name":
		cfg.DB.DBName = value
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB name set via LLM tool: %s", value)

	case "db-schema":
		cfg.DB.Schema = value
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB schema set via LLM tool: %s", value)

	case "db-user":
		cfg.DB.User = value
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB user set via LLM tool: %s", value)

	case "db-password":
		cfg.DB.Password = value
		if err := cfg.Save(); err != nil {
			return err
		}
		log.Info("DB password updated via LLM tool")

	default:
		return fmt.Errorf("unknown setting: %s", param)

	}

	return nil
}

// rebuildLLMClient creates a new LLM client from current config and replaces it in the agent.
func (a *Agent) rebuildLLMClient() {
	activeModel := config.GetActiveModelFromConfig(a.cfg)
	if activeModel == nil {
		log.Warn("Cannot rebuild LLM client: no active model found")
		return
	}

	// Resolve parameters: model-level takes precedence, fall back to global cfg.LLM
	temperature := a.cfg.LLM.Temperature
	if activeModel.Temperature != nil {
		temperature = *activeModel.Temperature
	}
	maxTokens := a.cfg.LLM.MaxTokens
	if activeModel.MaxTokens != nil {
		maxTokens = *activeModel.MaxTokens
	}
	thinkingEnabled := a.cfg.LLM.ThinkingEnabled == "on"
	if a.cfg.LLM.ThinkingEnabled == "default" && activeModel.ThinkingEnabled != nil {
		thinkingEnabled = *activeModel.ThinkingEnabled
	}
	reasoningEffort := a.cfg.LLM.ReasoningEffort
	if activeModel.ReasoningEffort != nil {
		reasoningEffort = *activeModel.ReasoningEffort
	}
	topP := a.cfg.LLM.TopP
	if activeModel.TopP != nil {
		topP = *activeModel.TopP
	}
	topK := a.cfg.LLM.TopK
	if activeModel.TopK != nil {
		topK = *activeModel.TopK
	}
	repetitionPenalty := a.cfg.LLM.RepetitionPenalty
	if activeModel.RepetitionPenalty != nil {
		repetitionPenalty = *activeModel.RepetitionPenalty
	}

	client := llm.NewClient(
		activeModel.Endpoint,
		activeModel.APIKey,
		activeModel.Model,
		temperature,
		maxTokens,
		a.cfg.LLM.LLMTimeout,
	)
	client.SetThinkingEnabled(thinkingEnabled)
	client.SetReasoningEffort(reasoningEffort)
	client.SetTopP(topP)
	client.SetTopK(topK)
	client.SetRepetitionPenalty(repetitionPenalty)
	client.SetTokenUsage(a.cfg.LLM.TokenUsage)
	if len(a.cfg.LLM.BodyAdditions) > 0 {
		client.SetBodyAdditions(a.cfg.LLM.BodyAdditions)
	}
	a.SetLLMClient(client)
	log.Info("LLM client rebuilt from model %s: endpoint=%s model=%s",
		activeModel.ID, activeModel.Endpoint, activeModel.Model)
}

// parseBool parses a string as a boolean value.
// Accepts: on/off, 1/0, true/false, yes/no
func parseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "on", "1", "true", "yes":
		return true, nil
	case "off", "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s (valid: on/off, 1/0, true/false, yes/no)", value)
	}
}

// boolToString converts a boolean to "on"/"off" string.
func boolToString(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// listSettingsTool handles the "list_settings" tool call from the LLM.
// It returns a formatted list of all available configuration parameters
// with their current values, valid ranges, and descriptions.
func (a *Agent) listSettingsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	cfg := a.cfg
	if cfg == nil {
		return "", fmt.Errorf("configuration not available")
	}

	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySettingCmd_528))

	// Helper to format a setting line: name + current value + keyed suffix
	formatLine := func(name, current string, suffixKey string) string {
		return fmt.Sprintf("  • %s\n"+i18n.T(i18n.KeySettingCmd_591)+i18n.T(suffixKey), name, current)
	}

	// Group 1: Identity & Personality
	sb.WriteString(i18n.T(i18n.KeySettingCmd_529))
	agentName := cfg.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	agentDesc := cfg.LLM.AgentDescription
	if agentDesc == "" {
		agentDesc = i18n.T(i18n.KeySettingCmd_536)
	}
	agentPrinciples := cfg.LLM.AgentPrinciples
	if agentPrinciples == "" {
		agentPrinciples = i18n.T(i18n.KeySettingCmd_536)
	}
	sb.WriteString(formatLine("name", agentName, i18n.KeySettingCmd_542))
	sb.WriteString(formatLine("description", agentDesc, i18n.KeySettingCmd_543))
	sb.WriteString(formatLine("principles", agentPrinciples, i18n.KeySettingCmd_544))

	// Group 2: Model Parameters
	sb.WriteString(i18n.T(i18n.KeySettingCmd_530))
	activeModel := config.GetActiveModelFromConfig(cfg)
	apiKey := "(not set)"
	endpoint := "(not set)"
	modelName := "(not set)"
	if activeModel != nil {
		apiKey = maskKey(activeModel.APIKey)
		endpoint = activeModel.Endpoint
		modelName = activeModel.Model
	}
	sb.WriteString(formatLine("api-key", apiKey, i18n.KeySettingCmd_545))
	sb.WriteString(formatLine("endpoint", endpoint, i18n.KeySettingCmd_546))
	sb.WriteString(formatLine("model", modelName, i18n.KeySettingCmd_547))
	sb.WriteString(formatLine("temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature), i18n.KeySettingCmd_548))
	sb.WriteString(formatLine("max-tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens), i18n.KeySettingCmd_549))
	maxIterStr := fmt.Sprintf("%d", cfg.LLM.MaxIterations)
	if cfg.LLM.MaxIterations <= 0 {
		maxIterStr = i18n.T(i18n.KeySettingCmd_537)
	}
	sb.WriteString(formatLine("max-iterations", maxIterStr, i18n.KeySettingCmd_550))
	sb.WriteString(formatLine("max-retries", fmt.Sprintf("%d", cfg.LLM.MaxRetries), i18n.KeySettingCmd_551))
	visionStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.VisionSupport {
		visionStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("vision", visionStr, i18n.KeySettingCmd_552))
	thinkingStr := cfg.LLM.ThinkingEnabled
	if thinkingStr == "" {
		thinkingStr = "default"
	}
	sb.WriteString(formatLine("thinking-enabled", thinkingStr, i18n.KeySettingCmd_553))
	sb.WriteString(formatLine("reasoning-effort", cfg.LLM.ReasoningEffort, i18n.KeySettingCmd_554))
	sb.WriteString(formatLine("top-p", fmt.Sprintf("%.1f", cfg.LLM.TopP), i18n.KeySettingCmd_555))
	sb.WriteString(formatLine("top-k", fmt.Sprintf("%d", cfg.LLM.TopK), i18n.KeySettingCmd_556))
	sb.WriteString(formatLine("repetition-penalty", fmt.Sprintf("%.1f", cfg.LLM.RepetitionPenalty), i18n.KeySettingCmd_557))

	// Group 3: Display & Output
	sb.WriteString(i18n.T(i18n.KeySettingCmd_531))
	llmThinkingStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowLlmThinking {
		llmThinkingStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-llm-thinking", llmThinkingStr, i18n.KeySettingCmd_558))
	llmContentStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowLlmContent {
		llmContentStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-llm-content", llmContentStr, i18n.KeySettingCmd_559))
	toolStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowTool {
		toolStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-tool", toolStr, i18n.KeySettingCmd_560))
	toolInputStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowToolInput {
		toolInputStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-tool-input", toolInputStr, i18n.KeySettingCmd_561))
	toolOutputStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowToolOutput {
		toolOutputStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-tool-output", toolOutputStr, i18n.KeySettingCmd_562))
	cmdStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowCommand {
		cmdStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-command", cmdStr, i18n.KeySettingCmd_563))
	cmdOutputStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.ShowCommandOutput {
		cmdOutputStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("show-command-output", cmdOutputStr, i18n.KeySettingCmd_564))
	resultModeStr := config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))
	sb.WriteString(formatLine("result-mode", resultModeStr, i18n.KeySettingCmd_565))

	// Group 4: Safety & Confirmation
	sb.WriteString(i18n.T(i18n.KeySettingCmd_532))
	confirmDefault := "confirm"
	if v, ok := cfg.LLM.ToolModes["default"]; ok {
		confirmDefault = v
	}
	sb.WriteString(formatLine("confirm-tool", confirmDefault, i18n.KeySettingCmd_566))
	toolTimeoutStr := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_541), cfg.LLM.ToolTimeout)
	if cfg.LLM.ToolTimeout <= 0 {
		toolTimeoutStr = i18n.T(i18n.KeySettingCmd_537)
	}
	sb.WriteString(formatLine("tool-timeout", toolTimeoutStr, i18n.KeySettingCmd_567))
	cmdTimeoutStr := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_541), cfg.LLM.CommandTimeout)
	if cfg.LLM.CommandTimeout <= 0 {
		cmdTimeoutStr = i18n.T(i18n.KeySettingCmd_537)
	}
	sb.WriteString(formatLine("cmd-timeout", cmdTimeoutStr, i18n.KeySettingCmd_568))
	llmTimeoutStr := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_541), cfg.LLM.LLMTimeout)
	if cfg.LLM.LLMTimeout <= 0 {
		llmTimeoutStr = i18n.T(i18n.KeySettingCmd_537)
	}
	sb.WriteString(formatLine("llm-timeout", llmTimeoutStr, i18n.KeySettingCmd_569))
	sb.WriteString(formatLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), i18n.KeySettingCmd_570))
	sb.WriteString(formatLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), i18n.KeySettingCmd_571))

	// Group 5: Memory & Context
	sb.WriteString(i18n.T(i18n.KeySettingCmd_533))
	memStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.MemoryEnabled {
		memStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("memory-enabled", memStr, i18n.KeySettingCmd_572))
	contextLimitStr := fmt.Sprintf("%d", cfg.LLM.ContextLimit)
	if cfg.LLM.ContextLimit == 0 {
		contextLimitStr = i18n.T(i18n.KeySettingCmd_540)
	} else if cfg.LLM.ContextLimit == -1 {
		contextLimitStr = i18n.T(i18n.KeySettingCmd_537)
	}
	sb.WriteString(formatLine("context-limit", contextLimitStr, i18n.KeySettingCmd_573))
	sb.WriteString(formatLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), i18n.KeySettingCmd_574))
	sb.WriteString(formatLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), i18n.KeySettingCmd_575))
	contextPolicy := i18n.T(i18n.KeyContextPolicyTask)
	if cfg.LLM.ContextPolicy == "window" {
		contextPolicy = i18n.T(i18n.KeyContextPolicyWindow)
	} else if cfg.LLM.ContextPolicy == "smart" {
		contextPolicy = i18n.T(i18n.KeyContextPolicySmart)
	} else if cfg.LLM.ContextPolicy == "reorganize" {
		contextPolicy = i18n.T(i18n.KeyContextPolicyReorganize)
	}
	sb.WriteString(formatLine("context-policy", contextPolicy, i18n.KeySettingCmd_576))

	// Database config (part of Memory & Context)
	dbEnabledStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.DB.Enabled {
		dbEnabledStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("db-enabled", dbEnabledStr, i18n.KeySettingCmd_577))
	sb.WriteString(formatLine("db-host", cfg.DB.Host, i18n.KeySettingCmd_578))
	sb.WriteString(formatLine("db-port", fmt.Sprintf("%d", cfg.DB.Port), i18n.KeySettingCmd_579))
	sb.WriteString(formatLine("db-name", cfg.DB.DBName, i18n.KeySettingCmd_580))
	sb.WriteString(formatLine("db-schema", cfg.DB.Schema, i18n.KeySettingCmd_581))
	sb.WriteString(formatLine("db-user", cfg.DB.User, i18n.KeySettingCmd_582))
	sb.WriteString(formatLine("db-password", "****", i18n.KeySettingCmd_583))

	// Group 6: Tasks & Sub-Agents
	sb.WriteString(i18n.T(i18n.KeySettingCmd_534))
	planStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.PlanEnabled {
		planStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("plan-enabled", planStr, i18n.KeySettingCmd_584))
	subStr := i18n.T(i18n.KeySettingCmd_538)
	if cfg.LLM.SubAgentEnabled {
		subStr = i18n.T(i18n.KeySettingCmd_539)
	}
	sb.WriteString(formatLine("subagent-enabled", subStr, i18n.KeySettingCmd_585))

	// Group 7: Search & Debug
	sb.WriteString(i18n.T(i18n.KeySettingCmd_535))
	sb.WriteString(formatLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), i18n.KeySettingCmd_586))
	sb.WriteString(formatLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), i18n.KeySettingCmd_587))
	sb.WriteString(formatLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), i18n.KeySettingCmd_588))
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	sb.WriteString(formatLine("log", logLevel, i18n.KeySettingCmd_589))

	sb.WriteString(i18n.T(i18n.KeySettingCmd_590))

	return sb.String(), nil
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
