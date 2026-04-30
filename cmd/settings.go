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
	"github.com/idirect3d/co-shell/llm"
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

// rebuildLLMClient creates a new LLM client from current config and replaces it in the agent.
// This is called when LLM-related settings (api-key, endpoint, model, temperature, max-tokens, vision)
// are changed at runtime so the changes take effect immediately without restart.
func (h *SettingsHandler) rebuildLLMClient() {
	client := llm.NewClient(
		h.cfg.LLM.Endpoint,
		h.cfg.LLM.APIKey,
		h.cfg.LLM.Model,
		h.cfg.LLM.Temperature,
		h.cfg.LLM.MaxTokens,
	)
	h.agent.SetLLMClient(client)
	log.Info("LLM client rebuilt and replaced in agent")
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
		// Rebuild LLM client to apply new API key immediately
		h.rebuildLLMClient()
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
		// Rebuild LLM client to apply new endpoint immediately
		h.rebuildLLMClient()
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
		// Rebuild LLM client to apply new model immediately
		h.rebuildLLMClient()
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
		// Rebuild LLM client to apply new temperature immediately
		h.rebuildLLMClient()
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
		// Rebuild LLM client to apply new max tokens immediately
		h.rebuildLLMClient()
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
		// Sync to agent immediately (rebuilds system prompt)
		h.agent.SetResultMode(config.ResultMode(mode))
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

	case "max-retries":
		if len(args) < 2 {
			return fmt.Sprintf("LLM 重试次数: %d", h.cfg.LLM.MaxRetries), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的重试次数: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("重试次数必须 >= 0")
		}
		h.cfg.LLM.MaxRetries = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Max retries set to %d", n)
		return fmt.Sprintf("✅ LLM 重试次数已设置为: %d", n), nil

	case "name":
		if len(args) < 2 {
			name := h.cfg.LLM.AgentName
			if name == "" {
				name = "co-shell"
			}
			return fmt.Sprintf("Agent 名称: %s", name), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentName = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetName(value)
		log.Info("Agent name set to %s", value)
		return fmt.Sprintf("✅ Agent 名称已设置为: %s", value), nil

	case "description":
		if len(args) < 2 {
			desc := h.cfg.LLM.AgentDescription
			if desc == "" {
				desc = "（未设置）"
			}
			return fmt.Sprintf("Agent 描述: %s", desc), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentDescription = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Rebuild system prompt to apply new description immediately
		h.agent.SetConfig(h.cfg)
		log.Info("Agent description set to %s", value)
		return fmt.Sprintf("✅ Agent 描述已设置为: %s", value), nil

	case "principles":
		if len(args) < 2 {
			principles := h.cfg.LLM.AgentPrinciples
			if principles == "" {
				principles = "（未设置）"
			}
			return fmt.Sprintf("Agent 核心原则: %s", principles), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentPrinciples = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Rebuild system prompt to apply new principles immediately
		h.agent.SetConfig(h.cfg)
		log.Info("Agent principles set to %s", value)
		return fmt.Sprintf("✅ Agent 核心原则已设置为: %s", value), nil

	case "vision":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.VisionSupport {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("视觉识别: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.VisionSupport = true
		case "off", "0", "false", "no":
			h.cfg.LLM.VisionSupport = false
		default:
			return "", fmt.Errorf("usage: .set vision on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Rebuild LLM client to apply new vision setting immediately
		h.rebuildLLMClient()
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.VisionSupport {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Vision support set to %s", status)
		return fmt.Sprintf("✅ 视觉识别已设置为: %s", status), nil

	case "context-limit":
		if len(args) < 2 {
			limitStr := fmt.Sprintf("%d", h.cfg.LLM.ContextLimit)
			if h.cfg.LLM.ContextLimit == 0 {
				limitStr = i18n.T(i18n.KeyOff)
			} else if h.cfg.LLM.ContextLimit == -1 {
				limitStr = i18n.T(i18n.KeyUnlimited)
			}
			return fmt.Sprintf("对话上下文限制: %s", limitStr), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的对话上下文限制值: %s", args[1])
		}
		if n < -1 {
			return "", fmt.Errorf("对话上下文限制必须 >= -1")
		}
		h.cfg.LLM.ContextLimit = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Context limit set to %d", n)
		return i18n.TF(i18n.KeyContextLimitUpdated, n, n), nil

	case "memory-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.MemoryEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("记忆功能: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.MemoryEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.MemoryEnabled = false
		default:
			return "", fmt.Errorf("usage: .set memory-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetMemoryEnabled(h.cfg.LLM.MemoryEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.MemoryEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Memory enabled set to %s", status)
		return fmt.Sprintf("✅ 记忆功能已设置为: %s", status), nil

	case "plan-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.PlanEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("任务计划功能: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.PlanEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.PlanEnabled = false
		default:
			return "", fmt.Errorf("usage: .set plan-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetPlanEnabled(h.cfg.LLM.PlanEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.PlanEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Plan enabled set to %s", status)
		return fmt.Sprintf("✅ 任务计划功能已设置为: %s", status), nil

	case "output-mode":
		if len(args) < 2 {
			currentMode := config.OutputModeString(config.OutputMode(h.cfg.LLM.OutputMode))
			return fmt.Sprintf("输出模式: %s", currentMode), nil
		}
		mode, ok := config.ParseOutputMode(args[1])
		if !ok {
			return "", fmt.Errorf("无效的输出模式: %s（可选值: compact, normal, debug）", args[1])
		}
		h.cfg.LLM.OutputMode = int(mode)
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetOutputMode(config.OutputMode(mode))
		log.Info("Output mode set to %s", args[1])
		return i18n.TF(i18n.KeyOutputModeUpdated, config.OutputModeString(mode)), nil

	case "search-max-line-length":
		if len(args) < 2 {
			return fmt.Sprintf("搜索单行最大字符数: %d", h.cfg.LLM.SearchMaxLineLength), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("搜索单行最大字符数必须 >= 0")
		}
		h.cfg.LLM.SearchMaxLineLength = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Search max line length set to %d", n)
		return fmt.Sprintf("✅ 搜索单行最大字符数已设置为: %d", n), nil

	case "search-max-result-bytes":
		if len(args) < 2 {
			return fmt.Sprintf("搜索结果最大字节数: %d", h.cfg.LLM.SearchMaxResultBytes), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("搜索结果最大字节数必须 >= 0")
		}
		h.cfg.LLM.SearchMaxResultBytes = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Search max result bytes set to %d", n)
		return fmt.Sprintf("✅ 搜索结果最大字节数已设置为: %d", n), nil

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
