// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
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
		return i18n.T(i18n.KeySettingsUpdated), nil

	case "endpoint":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings endpoint <url>")
		}
		h.cfg.LLM.Endpoint = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Endpoint updated to %s", args[1])
		return i18n.T(i18n.KeyEndpointUpdated), nil

	case "model":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .settings model <model>")
		}
		h.cfg.LLM.Model = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Model updated to %s", args[1])
		return i18n.T(i18n.KeyModelUpdated), nil

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
		return i18n.TF(i18n.KeyTempUpdated, temp), nil

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
		return i18n.TF(i18n.KeyMaxTokensUpdated, tokens), nil

	case "show-thinking":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowThinking {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyShowThinking), status), nil
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
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowThinking {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show thinking set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowThinking), status), nil

	case "show-command":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowCommand {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyShowCommand), status), nil
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
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowCommand {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show command set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowCommand), status), nil

	case "show-output":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowOutput {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyShowOutput), status), nil
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
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowOutput {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show output set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowOutput), status), nil

	case "log":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LogEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyLogEnabled), status), nil
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
		if err := log.SetEnabled(h.cfg.LogEnabled); err != nil {
			return "", fmt.Errorf("failed to update logger: %w", err)
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LogEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Logging set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyLogEnabled), status), nil

	default:
		return "", fmt.Errorf("unknown setting: %s", subcommand)
	}
}

// formatSettings formats the settings for display.
func formatSettings(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyConfigTitle) + "\n")
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigProvider), cfg.LLM.Provider))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigEndpoint), cfg.LLM.Endpoint))
	sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyConfigModel), cfg.LLM.Model))
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
