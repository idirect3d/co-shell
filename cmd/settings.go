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

	case "show-llm-thinking":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowLlmThinking {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyShowLlmThinking), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowLlmThinking = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowLlmThinking = false
		default:
			return "", fmt.Errorf("usage: .set show-llm-thinking on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetShowLlmThinking(h.cfg.LLM.ShowLlmThinking)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowLlmThinking {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show LLM thinking set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowLlmThinking), status), nil

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
		// Sync to agent immediately
		h.agent.SetShowCommand(h.cfg.LLM.ShowCommand)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowCommand {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show command set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowCommand), status), nil

	case "show-tool":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowTool {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("显示工具调用名称: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowTool = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowTool = false
		default:
			return "", fmt.Errorf("usage: .set show-tool on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetShowTool(h.cfg.LLM.ShowTool)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowTool {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show tool set to %s", status)
		return fmt.Sprintf("✅ 显示工具调用名称已设置为: %s", status), nil

	case "show-tool-input":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowToolInput {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("显示工具调用输入参数: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowToolInput = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowToolInput = false
		default:
			return "", fmt.Errorf("usage: .set show-tool-input on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetShowToolInput(h.cfg.LLM.ShowToolInput)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowToolInput {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show tool input set to %s", status)
		return fmt.Sprintf("✅ 显示工具调用输入参数已设置为: %s", status), nil

	case "show-tool-output":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowToolOutput {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("显示工具调用返回数据: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowToolOutput = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowToolOutput = false
		default:
			return "", fmt.Errorf("usage: .set show-tool-output on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetShowToolOutput(h.cfg.LLM.ShowToolOutput)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowToolOutput {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show tool output set to %s", status)
		return fmt.Sprintf("✅ 显示工具调用返回数据已设置为: %s", status), nil

	case "show-command-output":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowCommandOutput {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("显示命令返回数据: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowCommandOutput = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowCommandOutput = false
		default:
			return "", fmt.Errorf("usage: .set show-command-output on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetShowCommandOutput(h.cfg.LLM.ShowCommandOutput)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowCommandOutput {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show command output set to %s", status)
		return fmt.Sprintf("✅ 显示命令返回数据已设置为: %s", status), nil

	case "show-llm-content":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowLlmContent {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyShowLlmContent), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowLlmContent = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowLlmContent = false
		default:
			return "", fmt.Errorf("usage: .set show-llm-content on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowLlmContent {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show LLM content set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyShowLlmContent), status), nil

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

	case "subagent-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.SubAgentEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("子代理功能: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.SubAgentEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.SubAgentEnabled = false
		default:
			return "", fmt.Errorf("usage: .set subagent-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetSubAgentEnabled(h.cfg.LLM.SubAgentEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.SubAgentEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("SubAgent enabled set to %s", status)
		return fmt.Sprintf("✅ 子代理功能已设置为: %s", status), nil

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

	case "search-context-lines":
		if len(args) < 2 {
			return fmt.Sprintf("搜索匹配上下文行数: %d", h.cfg.LLM.SearchContextLines), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("搜索匹配上下文行数必须 >= 0")
		}
		h.cfg.LLM.SearchContextLines = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Search context lines set to %d", n)
		return fmt.Sprintf("✅ 搜索匹配上下文行数已设置为: %d", n), nil

	case "memory-search-max-content-len":
		if len(args) < 2 {
			return fmt.Sprintf("记忆搜索内容最大长度: %d", h.cfg.LLM.MemorySearchMaxContentLen), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("记忆搜索内容最大长度必须 >= 0")
		}
		h.cfg.LLM.MemorySearchMaxContentLen = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Memory search max content len set to %d", n)
		return fmt.Sprintf("✅ 记忆搜索内容最大长度已设置为: %d", n), nil

	case "memory-search-max-results":
		if len(args) < 2 {
			return fmt.Sprintf("记忆搜索最大结果数: %d", h.cfg.LLM.MemorySearchMaxResults), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("记忆搜索最大结果数必须 >= 0")
		}
		h.cfg.LLM.MemorySearchMaxResults = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Memory search max results set to %d", n)
		return fmt.Sprintf("✅ 记忆搜索最大结果数已设置为: %d", n), nil

	case "error-max-single-count":
		if len(args) < 2 {
			return fmt.Sprintf("相同错误最大出现次数: %d", h.cfg.LLM.ErrorMaxSingleCount), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("相同错误最大出现次数必须 >= 0")
		}
		h.cfg.LLM.ErrorMaxSingleCount = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Error max single count set to %d", n)
		return fmt.Sprintf("✅ 相同错误最大出现次数已设置为: %d", n), nil

	case "error-max-type-count":
		if len(args) < 2 {
			return fmt.Sprintf("不同错误类型最大数量: %d", h.cfg.LLM.ErrorMaxTypeCount), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("不同错误类型最大数量必须 >= 0")
		}
		h.cfg.LLM.ErrorMaxTypeCount = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Error max type count set to %d", n)
		return fmt.Sprintf("✅ 不同错误类型最大数量已设置为: %d", n), nil

	case "thinking-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ThinkingEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("AI 思考模式: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ThinkingEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ThinkingEnabled = false
		default:
			return "", fmt.Errorf("usage: .set thinking-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Rebuild LLM client to apply new thinking setting immediately
		h.rebuildLLMClient()
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ThinkingEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Thinking enabled set to %s", status)
		return fmt.Sprintf("✅ AI 思考模式已设置为: %s", status), nil

	case "reasoning-effort":
		if len(args) < 2 {
			return fmt.Sprintf("推理努力程度: %s", h.cfg.LLM.ReasoningEffort), nil
		}
		effort := args[1]
		switch effort {
		case "low", "medium", "high":
			h.cfg.LLM.ReasoningEffort = effort
		default:
			return "", fmt.Errorf("无效的推理努力程度: %s（可选值: low, medium, high）", effort)
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Rebuild LLM client to apply new reasoning effort immediately
		h.rebuildLLMClient()
		log.Info("Reasoning effort set to %s", effort)
		return fmt.Sprintf("✅ 推理努力程度已设置为: %s", effort), nil

	case "emoji-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.EmojiEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("表情符号前缀: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.EmojiEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.EmojiEnabled = false
		default:
			return "", fmt.Errorf("usage: .set emoji-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync to agent immediately
		h.agent.SetEmojiEnabled(h.cfg.LLM.EmojiEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.EmojiEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Emoji enabled set to %s", status)
		return fmt.Sprintf("✅ 表情符号前缀已设置为: %s", status), nil

	case "log":
		if len(args) < 2 {
			currentLevel := log.LogLevelString(log.GetLevel())
			return fmt.Sprintf("日志级别: %s（可选值: debug, info, warn, error, off）", currentLevel), nil
		}
		level, ok := log.ParseLogLevel(args[1])
		if !ok {
			return "", fmt.Errorf("无效的日志级别: %s（可选值: debug, info, warn, error, off）", args[1])
		}
		h.cfg.LogLevel = args[1]
		// Set log enabled based on level: off = disabled, anything else = enabled
		h.cfg.LogEnabled = level != log.LogLevelOff
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.SetLevel(level)
		if err := log.SetEnabled(h.cfg.LogEnabled); err != nil {
			return "", fmt.Errorf("failed to update logger: %w", err)
		}
		log.Info("Log level set to %s", args[1])
		return fmt.Sprintf("✅ 日志级别已设置为: %s", args[1]), nil

	default:
		return "", fmt.Errorf("unknown setting: %s", subcommand)

	}
}

// showSettingsHelp displays the current configuration grouped by category.
func showSettingsHelp(cfg *config.Config) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySettingsHelpFooter) + "\n")
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeySettingsCurrentTitle) + "\n")

	// Prepare all value strings first to calculate max width for alignment
	type settingLine struct {
		name  string
		value string
		col3  string
	}

	// Helper to build a setting line struct
	makeLine := func(name, value, col3 string) settingLine {
		return settingLine{name: name + ":", value: value, col3: col3}
	}

	// Prepare values
	llmThinkingStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowLlmThinking {
		llmThinkingStatus = i18n.T(i18n.KeyOn)
	}
	llmContentStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowLlmContent {
		llmContentStatus = i18n.T(i18n.KeyOn)
	}
	commandStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowCommand {
		commandStatus = i18n.T(i18n.KeyOn)
	}
	toolStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowTool {
		toolStatus = i18n.T(i18n.KeyOn)
	}
	toolInputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowToolInput {
		toolInputStatus = i18n.T(i18n.KeyOn)
	}
	toolOutputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowToolOutput {
		toolOutputStatus = i18n.T(i18n.KeyOn)
	}
	commandOutputStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ShowCommandOutput {
		commandOutputStatus = i18n.T(i18n.KeyOn)
	}

	confirmStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ConfirmCommand {
		confirmStatus = i18n.T(i18n.KeyOn)
	}
	logStatus := log.LogLevelString(log.GetLevel())
	visionStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.VisionSupport {
		visionStatus = i18n.T(i18n.KeyOn)
	}
	memoryEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.MemoryEnabled {
		memoryEnabledStatus = i18n.T(i18n.KeyOn)
	}
	planEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.PlanEnabled {
		planEnabledStatus = i18n.T(i18n.KeyOn)
	}
	subAgentEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.SubAgentEnabled {
		subAgentEnabledStatus = i18n.T(i18n.KeyOn)
	}
	thinkingEnabledStatus := i18n.T(i18n.KeyOff)
	if cfg.LLM.ThinkingEnabled {
		thinkingEnabledStatus = i18n.T(i18n.KeyOn)
	}

	maxIterStr := fmt.Sprintf("%d", cfg.LLM.MaxIterations)
	if cfg.LLM.MaxIterations <= 0 {
		maxIterStr = "1000 (" + i18n.T(i18n.KeyDefault) + ")"
	}

	toolTimeoutStr := fmt.Sprintf("%d", cfg.LLM.ToolTimeout)
	if cfg.LLM.ToolTimeout <= 0 {
		toolTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	cmdTimeoutStr := fmt.Sprintf("%d", cfg.LLM.CommandTimeout)
	if cfg.LLM.CommandTimeout <= 0 {
		cmdTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	llmTimeoutStr := fmt.Sprintf("%d", cfg.LLM.LLMTimeout)
	if cfg.LLM.LLMTimeout <= 0 {
		llmTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}

	contextLimitStr := fmt.Sprintf("%d", cfg.LLM.ContextLimit)
	if cfg.LLM.ContextLimit == 0 {
		contextLimitStr = i18n.T(i18n.KeyOff)
	} else if cfg.LLM.ContextLimit == -1 {
		contextLimitStr = i18n.T(i18n.KeyUnlimited)
	}

	agentName := cfg.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	agentDesc := cfg.LLM.AgentDescription
	if agentDesc == "" {
		agentDesc = "（" + i18n.T(i18n.KeyUnlimited) + "）"
	}
	agentPrinciples := cfg.LLM.AgentPrinciples
	if agentPrinciples == "" {
		agentPrinciples = "（" + i18n.T(i18n.KeyUnlimited) + "）"
	}

	resultModeStr := config.ResultModeString(config.ResultMode(cfg.LLM.ResultMode))

	// Collect all lines
	var allLines []settingLine

	// Group 1: Identity & Personality
	allLines = append(allLines,
		makeLine("name", agentName, i18n.T(i18n.KeyCol3Name)),
		makeLine("description", agentDesc, i18n.T(i18n.KeyCol3Desc)),
		makeLine("principles", agentPrinciples, i18n.T(i18n.KeyCol3Principles)),
	)

	// Group 2: Model Parameters
	allLines = append(allLines,
		makeLine("provider", cfg.LLM.Provider, i18n.T(i18n.KeyCol3Provider)),
		makeLine("endpoint", cfg.LLM.Endpoint, i18n.T(i18n.KeyCol3Endpoint)),
		makeLine("model", cfg.LLM.Model, i18n.T(i18n.KeyCol3Model)),
		makeLine("temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature), i18n.T(i18n.KeyCol3Temperature)),
		makeLine("max-tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens), i18n.T(i18n.KeyCol3MaxTokens)),
		makeLine("max-iterations", maxIterStr, i18n.T(i18n.KeyCol3MaxIter)),
		makeLine("max-retries", fmt.Sprintf("%d", cfg.LLM.MaxRetries), i18n.T(i18n.KeyCol3MaxRetries)),
		makeLine("vision", visionStatus, i18n.T(i18n.KeyCol3Vision)),
		makeLine("thinking-enabled", thinkingEnabledStatus, i18n.T(i18n.KeyCol3ThinkingEnabled)),
		makeLine("reasoning-effort", cfg.LLM.ReasoningEffort, i18n.T(i18n.KeyCol3ReasoningEffort)),
		makeLine("api-key", maskKey(cfg.LLM.APIKey), i18n.T(i18n.KeyCol3APIKey)),
	)

	// Group 3: Display & Output
	allLines = append(allLines,
		makeLine("show-llm-thinking", llmThinkingStatus, i18n.T(i18n.KeyCol3LlmThinking)),
		makeLine("show-llm-content", llmContentStatus, i18n.T(i18n.KeyCol3LlmContent)),
		makeLine("show-tool", toolStatus, i18n.T(i18n.KeyCol3Tool)),
		makeLine("show-tool-input", toolInputStatus, i18n.T(i18n.KeyCol3ToolInput)),
		makeLine("show-tool-output", toolOutputStatus, i18n.T(i18n.KeyCol3ToolOutput)),
		makeLine("show-command", commandStatus, i18n.T(i18n.KeyCol3Command)),
		makeLine("show-command-output", commandOutputStatus, i18n.T(i18n.KeyCol3CommandOutput)),
		makeLine("result-mode", resultModeStr, i18n.T(i18n.KeyCol3ResultMode)),
	)

	// Group 4: Safety & Confirmation
	allLines = append(allLines,
		makeLine("confirm-command", confirmStatus, i18n.T(i18n.KeyCol3Confirm)),
		makeLine("tool-timeout", toolTimeoutStr, i18n.T(i18n.KeyCol3ToolTimeout)),
		makeLine("cmd-timeout", cmdTimeoutStr, i18n.T(i18n.KeyCol3CmdTimeout)),
		makeLine("llm-timeout", llmTimeoutStr, i18n.T(i18n.KeyCol3LLMTimeout)),
		makeLine("error-max-single-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxSingleCount), i18n.T(i18n.KeyCol3ErrorMaxSingleCount)),
		makeLine("error-max-type-count", fmt.Sprintf("%d", cfg.LLM.ErrorMaxTypeCount), i18n.T(i18n.KeyCol3ErrorMaxTypeCount)),
	)

	// Group 5: Memory & Context
	allLines = append(allLines,
		makeLine("memory-enabled", memoryEnabledStatus, i18n.T(i18n.KeyCol3MemoryEnabled)),
		makeLine("context-limit", contextLimitStr, i18n.T(i18n.KeyCol3ContextLimit)),
		makeLine("memory-search-max-content-len", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxContentLen), i18n.T(i18n.KeyCol3MemorySearchMaxContentLen)),
		makeLine("memory-search-max-results", fmt.Sprintf("%d", cfg.LLM.MemorySearchMaxResults), i18n.T(i18n.KeyCol3MemorySearchMaxResults)),
	)

	// Group 6: Tasks & Sub-Agents
	allLines = append(allLines,
		makeLine("plan-enabled", planEnabledStatus, i18n.T(i18n.KeyCol3PlanEnabled)),
		makeLine("subagent-enabled", subAgentEnabledStatus, i18n.T(i18n.KeyCol3SubAgentEnabled)),
	)

	// Group 7: Search & Debug
	allLines = append(allLines,
		makeLine("search-max-line-length", fmt.Sprintf("%d", cfg.LLM.SearchMaxLineLength), i18n.T(i18n.KeyCol3SearchMaxLineLength)),
		makeLine("search-max-result-bytes", fmt.Sprintf("%d", cfg.LLM.SearchMaxResultBytes), i18n.T(i18n.KeyCol3SearchMaxResultBytes)),
		makeLine("search-context-lines", fmt.Sprintf("%d", cfg.LLM.SearchContextLines), i18n.T(i18n.KeyCol3SearchContextLines)),
		makeLine("log", logStatus, i18n.T(i18n.KeyCol3Log)),
	)

	// Helper to format a setting line with fixed column widths
	formatLine := func(name, value, col3 string) string {
		return fmt.Sprintf("  %-32s %-30s %s\n", name, value, col3)
	}

	// Helper to write a group
	writeGroup := func(title string, lines ...string) {
		sb.WriteString("\n  " + title + "\n")
		for _, line := range lines {
			sb.WriteString(line)
		}
	}

	// Track index for iterating through allLines
	lineIdx := 0
	nextLines := func(n int) []string {
		result := make([]string, 0, n)
		for i := 0; i < n && lineIdx < len(allLines); i++ {
			l := allLines[lineIdx]
			result = append(result, formatLine(l.name, l.value, l.col3))
			lineIdx++
		}
		return result
	}

	// Group 1: Identity & Personality
	writeGroup(i18n.T(i18n.KeySettingsGroupIdentity), nextLines(3)...)

	// Group 2: Model Parameters
	writeGroup(i18n.T(i18n.KeySettingsGroupModel), nextLines(11)...)

	// Group 3: Display & Output
	writeGroup(i18n.T(i18n.KeySettingsGroupDisplay), nextLines(8)...)

	// Group 4: Safety & Confirmation
	writeGroup(i18n.T(i18n.KeySettingsGroupSafety), nextLines(6)...)

	// Group 5: Memory & Context
	writeGroup(i18n.T(i18n.KeySettingsGroupMemory), nextLines(4)...)

	// Group 6: Tasks & Sub-Agents
	writeGroup(i18n.T(i18n.KeySettingsGroupTask), nextLines(2)...)

	// Group 7: Search & Debug
	writeGroup(i18n.T(i18n.KeySettingsGroupSearchDebug), nextLines(4)...)

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
