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
	"os"
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
		fmt.Printf("   旧值: %s\n", c.oldValue)
		fmt.Printf("   新值: %s\n", c.newValue)
		if c.reason != "" {
			fmt.Printf("   原因: %s\n", c.reason)
		}
		fmt.Println()
	}

	fmt.Println(i18n.T(i18n.KeySettingsConfirmRiskWarning))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeySettingsConfirmPrompt))
	fmt.Println()

	// Read user input
	var lineBuf []byte
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
		lineBuf = append(lineBuf, buf[0])
	}

	response := strings.TrimSpace(string(lineBuf))
	lower := strings.ToLower(response)

	if lower == "c" || lower == "no" || lower == "n" {
		// User rejected
		fmt.Println()
		fmt.Println("❌ " + i18n.T(i18n.KeySettingsConfirmRejected))
		fmt.Println()
		return i18n.T(i18n.KeySettingsConfirmRejectedResult), nil
	}

	if response == "" || lower == "y" || lower == "yes" || lower == "a" {
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
	switch param {
	case "api-key":
		return maskKey(cfg.LLM.APIKey)
	case "endpoint":
		return cfg.LLM.Endpoint
	case "model":
		return cfg.LLM.Model
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
	case "confirm-command":
		return boolToString(cfg.LLM.ConfirmCommand)
	case "result-mode":
		return config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))
	case "vision":
		return boolToString(cfg.LLM.VisionSupport)
	case "thinking-enabled":
		return boolToString(cfg.LLM.ThinkingEnabled)
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
	default:
		return "(unknown)"
	}
}

// applySetting applies a setting change to the config and syncs to the agent.
func applySetting(a *Agent, param, value string) error {
	cfg := a.cfg

	switch param {
	case "api-key":
		cfg.LLM.APIKey = value
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("API key updated via LLM tool")

	case "endpoint":
		cfg.LLM.Endpoint = value
		if err := cfg.Save(); err != nil {
			return err
		}
		a.rebuildLLMClient()
		log.Info("Endpoint updated via LLM tool: %s", value)

	case "model":
		cfg.LLM.Model = value
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

	case "confirm-command":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ConfirmCommand = b
		if err := cfg.Save(); err != nil {
			return err
		}
		a.SetConfirmCommand(b)
		log.Info("Confirm command set via LLM tool: %v", b)

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

	case "thinking-enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.LLM.ThinkingEnabled = b
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

	default:
		return fmt.Errorf("unknown setting: %s", param)
	}

	return nil
}

// rebuildLLMClient creates a new LLM client from current config and replaces it in the agent.
func (a *Agent) rebuildLLMClient() {
	client := llm.NewClient(
		a.cfg.LLM.Endpoint,
		a.cfg.LLM.APIKey,
		a.cfg.LLM.Model,
		a.cfg.LLM.Temperature,
		a.cfg.LLM.MaxTokens,
		a.cfg.LLM.LLMTimeout,
	)
	client.SetThinkingEnabled(a.cfg.LLM.ThinkingEnabled)
	client.SetReasoningEffort(a.cfg.LLM.ReasoningEffort)
	a.SetLLMClient(client)
	log.Info("LLM client rebuilt and replaced in agent")
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
	sb.WriteString("以下是 co-shell 所有可配置的系统参数清单：\n\n")

	// Helper to format a setting line
	formatLine := func(name, current, validRange, desc string) string {
		return fmt.Sprintf("  • %s\n    当前值: %s\n    取值范围: %s\n    说明: %s\n\n", name, current, validRange, desc)
	}

	// Group 1: Identity & Personality
	sb.WriteString("━━━ [ 身份与个性 ] ━━━\n\n")
	agentName := cfg.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	agentDesc := cfg.LLM.AgentDescription
	if agentDesc == "" {
		agentDesc = "(未设置)"
	}
	agentPrinciples := cfg.LLM.AgentPrinciples
	if agentPrinciples == "" {
		agentPrinciples = "(未设置)"
	}
	sb.WriteString(formatLine("name", agentName, "任意字符串", "Agent 的名称，用于标识和日志"))
	sb.WriteString(formatLine("description", agentDesc, "任意字符串", "Agent 的身份描述，告诉 LLM 它是什么"))
	sb.WriteString(formatLine("principles", agentPrinciples, "任意字符串", "Agent 的行为准则和原则"))

	// Group 2: Model Parameters
	sb.WriteString("━━━ [ 模型参数 ] ━━━\n\n")
	sb.WriteString(formatLine("api-key", maskKey(cfg.LLM.APIKey), "任意 API Key 字符串", "大模型 API 的认证密钥"))
	sb.WriteString(formatLine("endpoint", cfg.LLM.Endpoint, "有效的 API 端点 URL", "大模型 API 的服务地址"))
	sb.WriteString(formatLine("model", cfg.LLM.Model, "模型名称（如 deepseek-chat, gpt-4 等）", "当前使用的大模型名称"))
	sb.WriteString(formatLine("temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature), "0.0 ~ 2.0（浮点数）", "模型输出的随机性，值越高越有创造性"))
	sb.WriteString(formatLine("max-tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens), "1 ~ 128000（整数）", "每次 LLM 调用返回的最大 token 数"))
	maxIterStr := fmt.Sprintf("%d", cfg.LLM.MaxIterations)
	if cfg.LLM.MaxIterations <= 0 {
		maxIterStr = "无限制"
	}
	sb.WriteString(formatLine("max-iterations", maxIterStr, ">= 1 的整数，或 -1（无限制）", "单次任务中 LLM 调用的最大迭代次数"))
	sb.WriteString(formatLine("max-retries", fmt.Sprintf("%d", cfg.LLM.MaxRetries), ">= 0 的整数", "LLM 调用失败时的最大重试次数"))
	visionStr := "关闭"
	if cfg.LLM.VisionSupport {
		visionStr = "开启"
	}
	sb.WriteString(formatLine("vision", visionStr, "on/off, 1/0, true/false, yes/no", "是否启用多模态视觉识别能力"))
	thinkingStr := "关闭"
	if cfg.LLM.ThinkingEnabled {
		thinkingStr = "开启"
	}
	sb.WriteString(formatLine("thinking-enabled", thinkingStr, "on/off, 1/0, true/false, yes/no", "是否启用模型的思考（推理）能力"))
	sb.WriteString(formatLine("reasoning-effort", cfg.LLM.ReasoningEffort, "low / medium / high", "模型推理的深度级别"))

	// Group 3: Display & Output
	sb.WriteString("━━━ [ 显示与输出 ] ━━━\n\n")
	llmThinkingStr := "关闭"
	if cfg.LLM.ShowLlmThinking {
		llmThinkingStr = "开启"
	}
	sb.WriteString(formatLine("show-llm-thinking", llmThinkingStr, "on/off, 1/0, true/false, yes/no", "是否显示 LLM 的思考过程"))
	llmContentStr := "关闭"
	if cfg.LLM.ShowLlmContent {
		llmContentStr = "开启"
	}
	sb.WriteString(formatLine("show-llm-content", llmContentStr, "on/off, 1/0, true/false, yes/no", "是否显示 LLM 返回的主要内容"))
	toolStr := "关闭"
	if cfg.LLM.ShowTool {
		toolStr = "开启"
	}
	sb.WriteString(formatLine("show-tool", toolStr, "on/off, 1/0, true/false, yes/no", "是否显示工具调用名称"))
	toolInputStr := "关闭"
	if cfg.LLM.ShowToolInput {
		toolInputStr = "开启"
	}
	sb.WriteString(formatLine("show-tool-input", toolInputStr, "on/off, 1/0, true/false, yes/no", "是否显示工具调用的输入参数"))
	toolOutputStr := "关闭"
	if cfg.LLM.ShowToolOutput {
		toolOutputStr = "开启"
	}
	sb.WriteString(formatLine("show-tool-output", toolOutputStr, "on/off, 1/0, true/false, yes/no", "是否显示工具调用的返回数据"))
	cmdStr := "关闭"
	if cfg.LLM.ShowCommand {
		cmdStr = "开启"
	}
	sb.WriteString(formatLine("show-command", cmdStr, "on/off, 1/0, true/false, yes/no", "是否显示要执行的系统命令"))
	cmdOutputStr := "关闭"
	if cfg.LLM.ShowCommandOutput {
		cmdOutputStr = "开启"
	}
	sb.WriteString(formatLine("show-command-output", cmdOutputStr, "on/off, 1/0, true/false, yes/no", "是否显示命令执行结果"))
	resultModeStr := config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))
	sb.WriteString(formatLine("result-mode", resultModeStr, "minimal / explain / analyze / free", "结果处理模式：极简/解释/分析/自由"))

	// Group 4: Safety & Confirmation
	sb.WriteString("━━━ [ 安全与确认 ] ━━━\n\n")
	confirmStr := "关闭"
	if cfg.LLM.ConfirmCommand {
		confirmStr = "开启"
	}
	sb.WriteString(formatLine("confirm-command", confirmStr, "on/off, 1/0, true/false, yes/no", "执行系统命令前是否等待用户确认"))
	toolTimeoutStr := fmt.Sprintf("%d秒", cfg.LLM.ToolTimeout)
	if cfg.LLM.ToolTimeout <= 0 {
		toolTimeoutStr = "无限制"
	}
	sb.WriteString(formatLine("tool-timeout", toolTimeoutStr, ">= 0 的整数（秒），0=无限制", "工具调用的超时时间"))
	cmdTimeoutStr := fmt.Sprintf("%d秒", cfg.LLM.CommandTimeout)
	if cfg.LLM.CommandTimeout <= 0 {
		cmdTimeoutStr = "无限制"
	}
	sb.WriteString(formatLine("cmd-timeout", cmdTimeoutStr, ">= 0 的整数（秒），0=无限制", "系统命令执行的超时时间"))
	llmTimeoutStr := fmt.Sprintf("%d秒", cfg.LLM.LLMTimeout)
	if cfg.LLM.LLMTimeout <= 0 {
		llmTimeoutStr = "无限制"
	}
	sb.WriteString(formatLine("llm-timeout", llmTimeoutStr, ">= 0 的整数（秒），0=无限制", "LLM API 调用的超时时间"))
	sb.WriteString(formatLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), ">= 0 的整数", "相同错误的最大出现次数，超过后提示用户"))
	sb.WriteString(formatLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), ">= 0 的整数", "最大错误类型数，超过后提示用户"))

	// Group 5: Memory & Context
	sb.WriteString("━━━ [ 记忆与上下文 ] ━━━\n\n")
	memStr := "关闭"
	if cfg.LLM.MemoryEnabled {
		memStr = "开启"
	}
	sb.WriteString(formatLine("memory-enabled", memStr, "on/off, 1/0, true/false, yes/no", "是否启用持久化记忆功能"))
	contextLimitStr := fmt.Sprintf("%d", cfg.LLM.ContextLimit)
	if cfg.LLM.ContextLimit == 0 {
		contextLimitStr = "关闭（仅当前输入）"
	} else if cfg.LLM.ContextLimit == -1 {
		contextLimitStr = "无限制"
	}
	sb.WriteString(formatLine("context-limit", contextLimitStr, "-1（无限制）/ 0（仅当前输入）/ N（最近N条）", "发送给 LLM 的历史消息数量限制"))
	sb.WriteString(formatLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), ">= 0 的整数", "记忆搜索返回结果中每条内容的最大字符数"))
	sb.WriteString(formatLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), ">= 0 的整数", "记忆搜索返回的最大结果数量"))

	// Group 6: Tasks & Sub-Agents
	sb.WriteString("━━━ [ 任务与子代理 ] ━━━\n\n")
	planStr := "关闭"
	if cfg.LLM.PlanEnabled {
		planStr = "开启"
	}
	sb.WriteString(formatLine("plan-enabled", planStr, "on/off, 1/0, true/false, yes/no", "是否启用任务计划（checklist）功能"))
	subStr := "关闭"
	if cfg.LLM.SubAgentEnabled {
		subStr = "开启"
	}
	sb.WriteString(formatLine("subagent-enabled", subStr, "on/off, 1/0, true/false, yes/no", "是否允许启动子代理（sub-agent）"))

	// Group 7: Search & Debug
	sb.WriteString("━━━ [ 搜索与调试 ] ━━━\n\n")
	sb.WriteString(formatLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), ">= 0 的整数", "文件搜索时单行最大字符数，超长截断"))
	sb.WriteString(formatLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), ">= 0 的整数", "文件搜索返回结果的最大总字节数"))
	sb.WriteString(formatLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), ">= 0 的整数", "文件搜索时匹配行上下文的行数"))
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	sb.WriteString(formatLine("log", logLevel, "debug / info / warn / error / off", "日志输出级别"))

	sb.WriteString("\n使用 update_settings 工具可以修改以上参数。每次修改需要提供参数名、新值和修改原因，系统会提示用户确认。\n")

	return sb.String(), nil
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
