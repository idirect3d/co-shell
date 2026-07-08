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
		workMode := h.cfg.LLM.WorkMode
		if workMode == "" {
			workMode = "act"
		}
		if len(args) < 2 {
			// Display current description for this mode
			desc := ""
			if h.cfg.LLM.ModeDescriptions != nil {
				if md, ok := h.cfg.LLM.ModeDescriptions[workMode]; ok && md != "" {
					desc = md
				}
			}
			if desc == "" {
				desc = h.cfg.LLM.AgentDescription
			}
			if desc == "" {
				desc = i18n.T(i18n.KeyAgentDefaultDescription)
			}
			return fmt.Sprintf("Agent 描述(%s): %s", workMode, desc), nil
		}
		value := strings.Join(args[1:], " ")
		// Set mode-specific description
		if h.cfg.LLM.ModeDescriptions == nil {
			h.cfg.LLM.ModeDescriptions = make(map[string]string)
		}
		h.cfg.LLM.ModeDescriptions[workMode] = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Agent description set for mode %s: %s", workMode, value)
		return fmt.Sprintf("✅ Agent 描述(%s)已设置为: %s", workMode, value), nil

	case "principles":
		if len(args) < 2 {
			principles := h.cfg.LLM.AgentPrinciples
			if principles == "" {
				principles = ""
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

	case "shell-session-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShellSessionEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("持续Shell会话: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShellSessionEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShellSessionEnabled = false
		default:
			return "", fmt.Errorf("usage: .set shell-session-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetShellEnabled(h.cfg.LLM.ShellSessionEnabled)
		if !h.cfg.LLM.ShellSessionEnabled {
			h.agent.CloseShellSession()
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShellSessionEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Shell session enabled set to %s", status)
		return fmt.Sprintf("✅ 持续Shell会话已设置为: %s", status), nil

	case "shell-session-timeout":
		if len(args) < 2 {
			return fmt.Sprintf("持续Shell超时: %d秒（0=无限制）", h.cfg.LLM.ShellSessionTimeout), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", fmt.Errorf("无效的超时值: %s（请输入 >= 0 的整数，单位秒）", args[1])
		}
		h.cfg.LLM.ShellSessionTimeout = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell session timeout set to %d", n)
		if n == 0 {
			return "✅ 持续Shell超时已设置为: 无限制", nil
		}
		return fmt.Sprintf("✅ 持续Shell超时已设置为: %d秒", n), nil

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

	case "shell-vt-rows":
		if len(args) < 2 {
			return fmt.Sprintf("虚拟终端行数: %d", h.cfg.LLM.ShellVTRows), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 5 || n > 200 {
			return "", fmt.Errorf("无效的行数: %s（请输入 5-200 的整数）", args[1])
		}
		h.cfg.LLM.ShellVTRows = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell VT rows set to %d", n)
		return fmt.Sprintf("✅ 虚拟终端行数已设置为: %d", n), nil

	case "shell-vt-cols":
		if len(args) < 2 {
			return fmt.Sprintf("虚拟终端列数: %d", h.cfg.LLM.ShellVTCols), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 20 || n > 500 {
			return "", fmt.Errorf("无效的列数: %s（请输入 20-500 的整数）", args[1])
		}
		h.cfg.LLM.ShellVTCols = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell VT cols set to %d", n)
		return fmt.Sprintf("✅ 虚拟终端列数已设置为: %d", n), nil

	case "context-reorganize-threshold":
		if len(args) < 2 {
			return fmt.Sprintf("上下文重新整理阈值: %d%%（0=关闭自动触发）", h.cfg.LLM.ContextReorganizeThreshold), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 || n > 100 {
			return "", fmt.Errorf("无效的阈值: %s（请输入 0-100 的整数，0=关闭自动触发）", args[1])
		}
		h.cfg.LLM.ContextReorganizeThreshold = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Context reorganize threshold set to %d", n)
		if n == 0 {
			return "✅ 上下文自动重新整理已关闭", nil
		}
		return fmt.Sprintf("✅ 上下文重新整理阈值已设置为: %d%%", n), nil

	case "context-policy":
		if len(args) < 2 {
			mode := i18n.T(i18n.KeyContextPolicyTask)
			if h.cfg.LLM.ContextPolicy == "window" {
				mode = i18n.T(i18n.KeyContextPolicyWindow)
			} else if h.cfg.LLM.ContextPolicy == "smart" {
				mode = i18n.T(i18n.KeyContextPolicySmart)
			} else if h.cfg.LLM.ContextPolicy == "reorganize" {
				mode = i18n.T(i18n.KeyContextPolicyReorganize)
			}
			return fmt.Sprintf("上下文策略: %s", mode), nil
		}
		switch args[1] {
		case "window", "task", "smart", "reorganize":
			h.cfg.LLM.ContextPolicy = args[1]
		default:
			return "", fmt.Errorf("无效的上下文策略: %s（可选值: window, task, smart, reorganize）", args[1])
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		modeDesc := i18n.T(i18n.KeyContextPolicyTask)
		switch args[1] {
		case "window":
			modeDesc = i18n.T(i18n.KeyContextPolicyWindow)
		case "smart":
			modeDesc = i18n.T(i18n.KeyContextPolicySmart)
		case "reorganize":
			modeDesc = i18n.T(i18n.KeyContextPolicyReorganize)
		}
		log.Info("Context policy set to %s (%s)", args[1], modeDesc)
		return fmt.Sprintf("✅ 上下文策略已设置为: %s (%s)", args[1], modeDesc), nil

	case "browser-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.BrowserEnabled {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf("浏览器: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.BrowserEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.BrowserEnabled = false
		default:
			return "", fmt.Errorf("usage: .set browser-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetBrowserEnabled(h.cfg.LLM.BrowserEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.BrowserEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Browser enabled set to %s", status)
		return fmt.Sprintf("✅ 浏览器已设置为: %s", status), nil

	case "browser-port":
		if len(args) < 2 {
			return fmt.Sprintf("浏览器端口: %d", h.cfg.LLM.BrowserPort), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的端口: %s", args[1])
		}
		if n < 1 || n > 65535 {
			return "", fmt.Errorf("端口必须在 1 ~ 65535 之间")
		}
		h.cfg.LLM.BrowserPort = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Browser port set to %d", n)
		return fmt.Sprintf("✅ 浏览器端口已设置为: %d", n), nil

	case "browser-headless":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.BrowserHeadless {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf("无头模式: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.BrowserHeadless = true
		case "off", "0", "false", "no":
			h.cfg.LLM.BrowserHeadless = false
		default:
			return "", fmt.Errorf("usage: .set browser-headless on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.BrowserHeadless {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Browser headless set to %s", status)
		return fmt.Sprintf("✅ 无头模式已设置为: %s", status), nil

	case "read-file-max-size":
		if len(args) < 2 {
			return fmt.Sprintf("当前文件读取大小限制: %d bytes (%d KB)", h.cfg.LLM.ReadFileMaxSize, h.cfg.LLM.ReadFileMaxSize/1024), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1024 {
			return "", fmt.Errorf("usage: .set read-file-max-size <字节数> (最小 1024，0=不限制)")
		}
		h.cfg.LLM.ReadFileMaxSize = n
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to save config: %v", err)
		}
		log.Info("Read file max size set to %d", n)
		return fmt.Sprintf("✅ 文件读取大小限制已设置为: %d bytes (%d KB)", n, n/1024), nil

	case "browser-max-html-size":
		if len(args) < 2 {
			return fmt.Sprintf("当前 HTML 下载阈值: %d bytes (%d KB)", h.cfg.LLM.BrowserMaxHTMLSize, h.cfg.LLM.BrowserMaxHTMLSize/1024), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1024 {
			return "", fmt.Errorf("usage: .set browser-max-html-size <字节数> (最小 1024)")
		}
		h.cfg.LLM.BrowserMaxHTMLSize = n
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to save config: %v", err)
		}
		log.Info("Browser max HTML size set to %d", n)
		return fmt.Sprintf("✅ HTML 下载阈值已设置为: %d bytes (%d KB)", n, n/1024), nil

	case "input-mode":
		if len(args) < 2 {
			mode := h.cfg.LLM.InputMode
			if mode == "" {
				mode = "enhanced"
			}
			return fmt.Sprintf("REPL 输入模式: %s", mode), nil
		}
		switch args[1] {
		case "enhanced", "stdio":
			h.cfg.LLM.InputMode = args[1]
		default:
			return "", fmt.Errorf("无效的输入模式: %s（可选值: enhanced, stdio）", args[1])
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Input mode set to %s", args[1])
		return fmt.Sprintf("✅ REPL 输入模式已设置为: %s（重启后生效）", args[1]), nil

	case "excel-max-sessions":
		if len(args) < 2 {
			return fmt.Sprintf("Excel 最大并发会话数: %d", h.cfg.LLM.ExcelMaxSessions), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 50 {
			return "", fmt.Errorf("无效的并发会话数: %s（请输入 1-50 的整数）", args[1])
		}
		h.cfg.LLM.ExcelMaxSessions = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Excel max sessions set to %d", n)
		return fmt.Sprintf("✅ Excel 最大并发会话数已设置为: %d", n), nil

	case "excel-max-cells":
		if len(args) < 2 {
			return fmt.Sprintf("Excel 单次读取最大单元格数: %d", h.cfg.LLM.ExcelMaxCells), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 10 || n > 100000 {
			return "", fmt.Errorf("无效的单元格数: %s（请输入 10-100000 的整数）", args[1])
		}
		h.cfg.LLM.ExcelMaxCells = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Excel max cells set to %d", n)
		return fmt.Sprintf("✅ Excel 单次读取最大单元格数已设置为: %d", n), nil

	case "docx-max-sessions":
		if len(args) < 2 {
			return fmt.Sprintf("Word 最大并发会话数: %d", h.cfg.LLM.DocxMaxSessions), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 50 {
			return "", fmt.Errorf("无效的并发会话数: %s（请输入 1-50 的整数）", args[1])
		}
		h.cfg.LLM.DocxMaxSessions = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Docx max sessions set to %d", n)
		return fmt.Sprintf("✅ Word 最大并发会话数已设置为: %d", n), nil

	case "docx-max-read-paras":
		if len(args) < 2 {
			return fmt.Sprintf("Word 单次读取最大段落数: %d", h.cfg.LLM.DocxMaxReadParas), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 10 || n > 5000 {
			return "", fmt.Errorf("无效的段落数: %s（请输入 10-5000 的整数）", args[1])
		}
		h.cfg.LLM.DocxMaxReadParas = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Docx max read paras set to %d", n)
		return fmt.Sprintf("✅ Word 单次读取最大段落数已设置为: %d", n), nil

	case "debug":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.DebugMode {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyDebugMode)+": %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.DebugMode = true
		case "off", "0", "false", "no":
			h.cfg.LLM.DebugMode = false
		default:
			return "", fmt.Errorf("usage: .set agent debug on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetDebugMode(h.cfg.LLM.DebugMode)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.DebugMode {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Debug mode set to %s", status)
		return fmt.Sprintf("✅ 调试模式已设置为: %s", status), nil

	default:
		return "", fmt.Errorf("unknown agent setting: %s", subcommand)
	}
}
