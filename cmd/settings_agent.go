// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-05-21
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

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleAgentSetting handles agent identity and behavior settings: name, description,
// principles, max-iterations, max-retries, memory-enabled, plan-enabled, subagent-enabled,
// context-limit, context-start.
func (h *SettingsHandler) handleAgentSetting(subcommand string, args []string) (string, error) {
	switch subcommand {
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
				desc = i18n.T(i18n.KeyDefaultAgentDescription)
			}
			return fmt.Sprintf("Agent 描述: %s", desc), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentDescription = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Agent description set to %s", value)
		return fmt.Sprintf("✅ Agent 描述已设置为: %s", value), nil

	case "principles":
		if len(args) < 2 {
			principles := h.cfg.LLM.AgentPrinciples
			if principles == "" {
				principles = i18n.T(i18n.KeyDefaultAgentPrinciples)
			}
			return fmt.Sprintf("Agent 核心原则: %s", principles), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentPrinciples = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Agent principles set to %s", value)
		return fmt.Sprintf("✅ Agent 核心原则已设置为: %s", value), nil

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
		h.agent.SetSubAgentEnabled(h.cfg.LLM.SubAgentEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.SubAgentEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("SubAgent enabled set to %s", status)
		return fmt.Sprintf("✅ 子代理功能已设置为: %s", status), nil

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

	case "context-start":
		if len(args) < 2 {
			mode := i18n.T(i18n.KeyContextStartTask)
			if h.cfg.LLM.ContextStartMode == "window" {
				mode = i18n.T(i18n.KeyContextStartWindow)
			} else if h.cfg.LLM.ContextStartMode == "smart" {
				mode = i18n.T(i18n.KeyContextStartSmart)
			}
			return fmt.Sprintf("上下文起始模式: %s", mode), nil
		}
		switch args[1] {
		case "window", "task", "smart":
			h.cfg.LLM.ContextStartMode = args[1]
		default:
			return "", fmt.Errorf("无效的上下文起始模式: %s（可选值: window, task, smart）", args[1])
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		modeDesc := i18n.T(i18n.KeyContextStartTask)
		switch args[1] {
		case "window":
			modeDesc = i18n.T(i18n.KeyContextStartWindow)
		case "smart":
			modeDesc = i18n.T(i18n.KeyContextStartSmart)
		}
		log.Info("Context start mode set to %s (%s)", args[1], modeDesc)
		return fmt.Sprintf("✅ 上下文起始模式已设置为: %s (%s)", args[1], modeDesc), nil

	default:
		return "", fmt.Errorf("unknown agent setting: %s", subcommand)
	}
}
