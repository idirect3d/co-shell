// Author: L.Shuang
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/log"
)

// SettingsHandler handles the .settings built-in command.
type SettingsHandler struct {
	cfg *config.Config
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{cfg: cfg}
}

// Handle processes .settings commands.
// Syntax:
//
//	.settings                          - show current settings
//	.settings api-key <key>            - set API key
//	.settings endpoint <url>           - set API endpoint
//	.settings model <model>            - set model name
//	.settings temperature <value>      - set temperature (0.0-2.0)
//	.settings max-tokens <count>       - set max tokens
//	.settings show-thinking on/off     - show/hide LLM thinking process
//	.settings show-command on/off      - show/hide commands before execution
//	.settings show-output on/off       - show/hide command output before LLM analysis
//	.settings log on/off               - enable/disable file logging
func (h *SettingsHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.cfg.Show(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "api-key":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings api-key <key>")
		}
		h.cfg.LLM.APIKey = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("API key updated")
		return "✅ API key updated", nil

	case "endpoint":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings endpoint <url>")
		}
		h.cfg.LLM.Endpoint = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Endpoint updated to %s", args[1])
		return "✅ Endpoint updated", nil

	case "model":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings model <model>")
		}
		h.cfg.LLM.Model = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Model updated to %s", args[1])
		return "✅ Model updated", nil

	case "temperature":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings temperature <value>")
		}
		temp, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("invalid temperature value: %s", args[1])
		}
		if temp < 0 || temp > 2 {
			return "", fmt.Errorf("temperature must be between 0.0 and 2.0")
		}
		h.cfg.LLM.Temperature = temp
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Temperature set to %.1f", temp)
		return fmt.Sprintf("✅ Temperature set to %.1f", temp), nil

	case "max-tokens":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings max-tokens <count>")
		}
		tokens, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid token count: %s", args[1])
		}
		if tokens < 1 || tokens > 128000 {
			return "", fmt.Errorf("max-tokens must be between 1 and 128000")
		}
		h.cfg.LLM.MaxTokens = tokens
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Max tokens set to %d", tokens)
		return fmt.Sprintf("✅ Max tokens set to %d", tokens), nil

	case "show-thinking":
		if len(args) < 2 {
			status := "on"
			if !h.cfg.LLM.ShowThinking {
				status = "off"
			}
			return fmt.Sprintf("Show thinking is currently %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowThinking = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowThinking = false
		default:
			return "", fmt.Errorf("usage: .settings show-thinking on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := "on"
		if !h.cfg.LLM.ShowThinking {
			status = "off"
		}
		log.Info("Show thinking set to %s", status)
		return fmt.Sprintf("✅ Show thinking set to %s", status), nil

	case "show-command":
		if len(args) < 2 {
			status := "on"
			if !h.cfg.LLM.ShowCommand {
				status = "off"
			}
			return fmt.Sprintf("Show command is currently %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowCommand = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowCommand = false
		default:
			return "", fmt.Errorf("usage: .settings show-command on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := "on"
		if !h.cfg.LLM.ShowCommand {
			status = "off"
		}
		log.Info("Show command set to %s", status)
		return fmt.Sprintf("✅ Show command set to %s", status), nil

	case "show-output":
		if len(args) < 2 {
			status := "on"
			if !h.cfg.LLM.ShowOutput {
				status = "off"
			}
			return fmt.Sprintf("Show output is currently %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowOutput = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowOutput = false
		default:
			return "", fmt.Errorf("usage: .settings show-output on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := "on"
		if !h.cfg.LLM.ShowOutput {
			status = "off"
		}
		log.Info("Show output set to %s", status)
		return fmt.Sprintf("✅ Show output set to %s", status), nil

	case "provider":
		if len(args) < 2 {
			return fmt.Sprintf("Provider is currently %s", h.cfg.LLM.Provider), nil
		}
		provider := config.FindProvider(args[1])
		if provider == nil {
			return "", fmt.Errorf("unknown provider: %s\n\nAvailable providers:\n  deepseek          - DeepSeek\n  qwen              - 阿里千问（通义千问）\n  openai-compatible - OpenAI 兼容（自定义）", args[1])
		}
		h.cfg.LLM.Provider = provider.Name
		if provider.Endpoint != "" {
			h.cfg.LLM.Endpoint = provider.Endpoint
		}
		if provider.DefaultModel != "" {
			h.cfg.LLM.Model = provider.DefaultModel
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Provider set to %s", provider.Name)
		return fmt.Sprintf("✅ Provider set to %s (endpoint: %s, model: %s)", provider.DisplayName, h.cfg.LLM.Endpoint, h.cfg.LLM.Model), nil

	case "max-iterations":
		if len(args) < 2 {
			return fmt.Sprintf("Max iterations is currently %d", h.cfg.LLM.MaxIterations), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid number: %s", args[1])
		}
		if n < -1 {
			return "", fmt.Errorf("max-iterations must be -1 (unlimited) or a positive integer")
		}
		h.cfg.LLM.MaxIterations = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Max iterations set to %d", n)
		if n == -1 {
			return "✅ Max iterations set to unlimited", nil
		}
		return fmt.Sprintf("✅ Max iterations set to %d", n), nil

	case "log":
		if len(args) < 2 {
			status := "on"
			if !h.cfg.LogEnabled {
				status = "off"
			}
			return fmt.Sprintf("Logging is currently %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LogEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LogEnabled = false
		default:
			return "", fmt.Errorf("usage: .settings log on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Apply the change at runtime
		if err := log.SetEnabled(h.cfg.LogEnabled); err != nil {
			return "", fmt.Errorf("failed to update logger: %w", err)
		}
		status := "on"
		if !h.cfg.LogEnabled {
			status = "off"
		}
		log.Info("Logging set to %s", status)
		return fmt.Sprintf("✅ Logging set to %s", status), nil

	default:
		return "", fmt.Errorf("unknown setting: %s\n\nAvailable settings:\n  api-key         - Set API key\n  endpoint        - Set API endpoint URL\n  model           - Set model name\n  provider        - Set provider (deepseek/qwen/openai-compatible)\n  temperature     - Set temperature (0.0-2.0)\n  max-tokens      - Set max tokens\n  max-iterations  - Set max iterations (-1 for unlimited)\n  show-thinking   - Show/hide LLM thinking process (on|off)\n  show-command    - Show/hide commands before execution (on|off)\n  show-output     - Show/hide command output before LLM analysis (on|off)\n  log             - Enable/disable file logging (on|off)", subcommand)
	}
}

// Help returns the help text for the settings command.
func (h *SettingsHandler) Help() string {
	return `Settings Management (.settings)

Usage:
  .settings                          Show current settings
  .settings api-key <key>            Set API key
  .settings endpoint <url>           Set API endpoint URL
  .settings model <model>            Set model name
  .settings provider <name>          Set provider (deepseek/qwen/openai-compatible)
  .settings temperature <value>      Set temperature (0.0-2.0)
  .settings max-tokens <count>       Set max tokens
  .settings max-iterations <n>       Set max iterations (-1 for unlimited)
  .settings show-thinking on|off     Show/hide LLM thinking process
  .settings show-command on|off      Show/hide commands before execution
  .settings show-output on|off       Show/hide command output before LLM analysis
  .settings log on|off               Enable/disable file logging

Examples:
  .settings api-key sk-xxx
  .settings endpoint https://api.deepseek.com/v1
  .settings model deepseek-chat
  .settings provider deepseek
  .settings provider qwen
  .settings provider openai-compatible
  .settings temperature 0.3
  .settings max-iterations 20
  .settings max-iterations -1
  .settings show-thinking off
  .settings show-command off
  .settings show-output off
  .settings log off`
}

// formatSettings formats the settings for display.
func formatSettings(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString("Current Settings:\n")
	sb.WriteString(fmt.Sprintf("  API Key:      %s\n", maskKey(cfg.LLM.APIKey)))
	sb.WriteString(fmt.Sprintf("  Endpoint:     %s\n", cfg.LLM.Endpoint))
	sb.WriteString(fmt.Sprintf("  Model:        %s\n", cfg.LLM.Model))
	sb.WriteString(fmt.Sprintf("  Temperature:  %.1f\n", cfg.LLM.Temperature))
	sb.WriteString(fmt.Sprintf("  Max Tokens:   %d\n", cfg.LLM.MaxTokens))
	return sb.String()
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
