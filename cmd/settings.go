// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// SettingsHandler handles the .settings built-in command.
type SettingsHandler struct {
	cfg   *config.Config
	agent *agent.Agent
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(cfg *config.Config, ag *agent.Agent) *SettingsHandler {
	return &SettingsHandler{cfg: cfg, agent: ag}
}

// Handle processes .settings commands.
func (h *SettingsHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return showSettingsHelp(h.cfg), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "api-key":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set api-key <key>")
		}
		h.cfg.LLM.APIKey = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("API key updated")
		return i18n.T(i18n.KeySettingsUpdated), nil

	case "endpoint":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set endpoint <url>")
		}
		h.cfg.LLM.Endpoint = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Endpoint updated to %s", args[1])
		return i18n.T(i18n.KeyEndpointUpdated), nil

	case "model":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set model <model>")
		}
		h.cfg.LLM.Model = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Model updated to %s", args[1])
		return i18n.T(i18n.KeyModelUpdated), nil

	case "temperature":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set temperature <value>")
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
			return "", fmt.Errorf("usage: .set max-tokens <count>")
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
			return "", fmt.Errorf("usage: .set show-thinking on|off")
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
			return "", fmt.Errorf("usage: .set show-command on|off")
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
			return "", fmt.Errorf("usage: .set show-output on|off")
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

	case "confirm-command":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ConfirmCommand {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmEnabled), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ConfirmCommand = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ConfirmCommand = false
		default:
			return "", fmt.Errorf("usage: .set confirm-command on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetConfirmCommand(h.cfg.LLM.ConfirmCommand)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ConfirmCommand {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Confirm command set to %s", status)

		// Show warning when disabling
		if !h.cfg.LLM.ConfirmCommand {
			return fmt.Sprintf("%s\n%s", fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmDisabled), status), i18n.T(i18n.KeyCmdConfirmDisableWarn)), nil
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmEnabled), status), nil

	case "result-mode":
		if len(args) < 2 {
			currentMode := config.ResultModeString(config.ResultMode(h.cfg.LLM.ResultMode))
			return fmt.Sprintf("结果处理模式: %s", currentMode), nil
		}
		mode, ok := config.ParseResultMode(args[1])
		if !ok {
			return "", fmt.Errorf("无效的结果处理模式: %s（可选值: minimal, explain, analyze, free）", args[1])
		}
		h.cfg.LLM.ResultMode = int(mode)
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Result mode set to %s", args[1])
		return fmt.Sprintf("✅ 结果处理模式已设置为: %s", config.ResultModeString(mode)), nil

	case "max-iterations":
		if len(args) < 2 {
			maxIterStr := fmt.Sprintf("%d", h.cfg.LLM.MaxIterations)
			if h.cfg.LLM.MaxIterations <= 0 {
				maxIterStr = "1000（默认）"
			}
			return fmt.Sprintf("最大迭代次数: %s", maxIterStr), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的迭代次数: %s", args[1])
		}
		if n < -1 || n == 0 {
			return "", fmt.Errorf("迭代次数必须 >= 1，或 -1（不限制）")
		}
		h.cfg.LLM.MaxIterations = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetMaxIterations(n)
		log.Info("Max iterations set to %d", n)
		maxIterStr := fmt.Sprintf("%d", n)
		if n == -1 {
			maxIterStr = "不限制"
		}
		return fmt.Sprintf("✅ 最大迭代次数已设置为: %s", maxIterStr), nil

	case "name":
		if len(args) < 2 {
			name := h.cfg.LLM.AgentName
			if name == "" {
				name = "co-shell"
			}
			return fmt.Sprintf("Agent 名称: %s", name), nil
		}
		h.cfg.LLM.AgentName = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetName(args[1])
		log.Info("Agent name set to %s", args[1])
		return fmt.Sprintf("✅ Agent 名称已设置为: %s", args[1]), nil

	case "description":
		if len(args) < 2 {
			desc := h.cfg.LLM.AgentDescription
			if desc == "" {
				desc = "（未设置）"
			}
			return fmt.Sprintf("Agent 描述: %s", desc), nil
		}
		h.cfg.LLM.AgentDescription = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Agent description set to %s", args[1])
		return fmt.Sprintf("✅ Agent 描述已设置为: %s", args[1]), nil

	case "principles":
		if len(args) < 2 {
			principles := h.cfg.LLM.AgentPrinciples
			if principles == "" {
				principles = "（未设置）"
			}
			return fmt.Sprintf("Agent 核心原则: %s", principles), nil
		}
		h.cfg.LLM.AgentPrinciples = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Agent principles set to %s", args[1])
		return fmt.Sprintf("✅ Agent 核心原则已设置为: %s", args[1]), nil

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
			return "", fmt.Errorf("usage: .set log on|off")
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

// showSettingsHelp displays the current configuration with parameter names and value ranges.
func showSettingsHelp(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySettingsHelpFooter) + "\n")
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySettingsCurrentTitle) + "\n")
	sb.WriteString(cfg.Show())
	return sb.String()
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
