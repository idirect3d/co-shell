package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/liangshuang/co-shell/config"
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
		return "✅ API key updated", nil

	case "endpoint":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings endpoint <url>")
		}
		h.cfg.LLM.Endpoint = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		return "✅ Endpoint updated", nil

	case "model":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings model <model>")
		}
		h.cfg.LLM.Model = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
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
		return fmt.Sprintf("✅ Max tokens set to %d", tokens), nil

	default:
		return "", fmt.Errorf("unknown setting: %s\n\nAvailable settings:\n  api-key      - Set API key\n  endpoint     - Set API endpoint URL\n  model        - Set model name\n  temperature  - Set temperature (0.0-2.0)\n  max-tokens   - Set max tokens", subcommand)
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
  .settings temperature <value>      Set temperature (0.0-2.0)
  .settings max-tokens <count>       Set max tokens

Examples:
  .settings api-key sk-xxx
  .settings endpoint https://api.deepseek.com/v1
  .settings model deepseek-chat
  .settings temperature 0.3`
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
