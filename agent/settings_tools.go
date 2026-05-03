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

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
