// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-06-04
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleSafetySetting handles safety and confirmation settings: confirm-tool,
// error-max-single-count, error-max-type-count, loop-detect-*, dedup-*.
func (h *SettingsHandler) handleSafetySetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "confirm-tool":
		if len(args) < 2 {
			return h.showToolModes(), nil
		}
		toolName := args[1]
		switch toolName {
		case "on", "1", "true", "yes":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "confirm"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", "confirm")
			log.Info("Confirm tool set to on (confirm)")
			return fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmEnabled), i18n.T(i18n.KeyOn)), nil
		case "off", "0", "false", "no":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "auto"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", "auto")
			log.Info("Confirm tool set to off (auto)")
			return fmt.Sprintf("%s\n%s", fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmDisabled), i18n.T(i18n.KeyOff)), i18n.T(i18n.KeyCmdConfirmDisableWarn)), nil
		case "reset":
			// Reset all tool mode settings: clear config and re-sync from defaults.
			h.cfg.LLM.ToolModes = make(map[string]string)
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SyncToolModes(h.cfg)
			log.Info("Confirm tool modes reset to defaults")
			return "所有工具确认模式已重置为默认值", nil

		case "confirm", "auto", "disabled":
			// Global override: set "default" key. SyncToolModes will apply this
			// value to ALL tools regardless of their individual settings.
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = toolName
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", toolName)
			log.Info("Confirm tool global default set to %s", toolName)
			return fmt.Sprintf("全局工具确认模式已设置为: %s（覆盖所有方法）", toolName), nil
		case "custom":
			// "custom" means no global override — each tool uses its own setting.
			// Save "default": "custom" to config so SyncToolModes knows not to apply
			// a global override, and the value persists across restarts.
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "custom"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SyncToolModes(h.cfg)
			log.Info("Confirm tool default cleared, per-tool mode restored")
			return "全局工具确认模式已设置为: custom（各方法按各自设置运行）", nil
		}
		if len(args) < 3 {
			mode := "confirm"
			if v, ok := h.cfg.LLM.ToolModes[toolName]; ok {
				mode = v
			} else if v, ok := h.cfg.LLM.ToolModes["default"]; ok {
				// If global default is active, individual tool shows the global value
				mode = v
			}
			return fmt.Sprintf("工具 %s 模式: %s", toolName, mode), nil
		}
		switch args[2] {
		case "on", "1", "true", "yes":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = "confirm"
		case "off", "0", "false", "no":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = "auto"
		case "confirm", "auto", "disabled":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = args[2]
		default:
			return "", fmt.Errorf("使用方法: .set confirm-tool [<工具名>] on|off|confirm|auto|disabled")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetToolMode(toolName, h.cfg.LLM.ToolModes[toolName])
		mode := h.cfg.LLM.ToolModes[toolName]
		log.Info("Confirm tool %s set to %s", toolName, mode)
		return fmt.Sprintf("工具 %s 模式已设置为: %s", toolName, mode), nil

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

	case "loop-detect-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.LoopDetectEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCol3LoopDetectEnabled), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.LoopDetectEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.LoopDetectEnabled = false
		default:
			return "", fmt.Errorf("usage: .set loop-detect-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.LoopDetectEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Loop detect enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyLoopDetectEnabledUpdated), status), nil

	case "loop-detect-threshold":
		if len(args) < 2 {
			return fmt.Sprintf("循环检测阈值: %d", h.cfg.LLM.LoopDetectThreshold), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 1 {
			return "", fmt.Errorf("循环检测阈值必须 >= 1")
		}
		h.cfg.LLM.LoopDetectThreshold = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop detect threshold set to %d", n)
		return fmt.Sprintf("✅ 循环检测阈值已设置为: %d", n), nil

	case "loop-detect-max-window":
		if len(args) < 2 {
			return fmt.Sprintf("循环检测滑动窗口大小: %d", h.cfg.LLM.LoopDetectMaxWindow), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 1 {
			return "", fmt.Errorf("循环检测滑动窗口大小必须 >= 1")
		}
		h.cfg.LLM.LoopDetectMaxWindow = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop detect max window set to %d", n)
		return fmt.Sprintf("✅ 循环检测滑动窗口大小已设置为: %d", n), nil

	case "dedup-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.DedupEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("消息去重检测: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.DedupEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.DedupEnabled = false
		default:
			return "", fmt.Errorf("usage: .set dedup-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.DedupEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Dedup enabled set to %s", status)
		return fmt.Sprintf("✅ 消息去重检测已设置为: %s", status), nil

	case "dedup-feature-ratio":
		if len(args) < 2 {
			return fmt.Sprintf("特征词抽取比例: %.1f", h.cfg.LLM.DedupFeatureRatio), nil
		}
		val, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的小数值: %s", args[1])
		}
		if val < 0 || val > 1 {
			return "", fmt.Errorf("特征词抽取比例必须在 0.0 ~ 1.0 之间")
		}
		h.cfg.LLM.DedupFeatureRatio = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Dedup feature ratio set to %.1f", val)
		return fmt.Sprintf("✅ 特征词抽取比例已设置为: %.1f", val), nil

	case "dedup-match-ratio":
		if len(args) < 2 {
			return fmt.Sprintf("特征匹配率阈值: %.1f", h.cfg.LLM.DedupMatchRatio), nil
		}
		val, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的小数值: %s", args[1])
		}
		if val < 0 || val > 1 {
			return "", fmt.Errorf("特征匹配率阈值必须在 0.0 ~ 1.0 之间")
		}
		h.cfg.LLM.DedupMatchRatio = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Dedup match ratio set to %.1f", val)
		return fmt.Sprintf("✅ 特征匹配率阈值已设置为: %.1f", val), nil

	case "dedup-similarity-threshold":
		if len(args) < 2 {
			return fmt.Sprintf("相似度阈值: %d%%", h.cfg.LLM.DedupSimilarityThreshold), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 1 || n > 100 {
			return "", fmt.Errorf("相似度阈值必须在 1 ~ 100 之间")
		}
		h.cfg.LLM.DedupSimilarityThreshold = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Dedup similarity threshold set to %d", n)
		return fmt.Sprintf("✅ 相似度阈值已设置为: %d%%", n), nil

	case "dedup-max-history":
		if len(args) < 2 {
			return fmt.Sprintf("去重检查历史消息数: %d", h.cfg.LLM.DedupMaxHistory), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 1 {
			return "", fmt.Errorf("去重检查历史消息数必须 >= 1")
		}
		h.cfg.LLM.DedupMaxHistory = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Dedup max history set to %d", n)
		return fmt.Sprintf("✅ 去重检查历史消息数已设置为: %d", n), nil

	case "dedup-repeat-limit":
		if len(args) < 2 {
			return fmt.Sprintf("去重触发重复次数: %d", h.cfg.LLM.DedupRepeatLimit), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 1 {
			return "", fmt.Errorf("去重触发重复次数必须 >= 1")
		}
		h.cfg.LLM.DedupRepeatLimit = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Dedup repeat limit set to %d", n)
		return fmt.Sprintf("✅ 去重触发重复次数已设置为: %d", n), nil

	default:
		return "", fmt.Errorf("unknown safety setting: %s", subcommand)
	}
}

// showToolModes displays the current tool mode configuration.
func (h *SettingsHandler) showToolModes() string {
	var sb strings.Builder
	sb.WriteString("工具模式配置:\n")

	// Start with DefaultToolModes, overlay per-tool config overrides.
	effectiveModes := agent.DefaultToolModes()

	// Determine global default mode
	globalDefault := ""
	for k, v := range h.cfg.LLM.ToolModes {
		if k == "default" {
			globalDefault = v
		} else {
			effectiveModes[k] = v
		}
	}

	if globalDefault == "" || globalDefault == "custom" {
		sb.WriteString("  全局默认: custom（各方法按各自设置运行）\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("  全局默认: %s（覆盖所有方法）\n\n", globalDefault))
	}

	// Build tool list dynamically from DefaultToolModes()
	allTools := make([]string, 0, len(agent.DefaultToolModes()))
	for name := range agent.DefaultToolModes() {
		if name != "default" {
			allTools = append(allTools, name)
		}
	}
	sort.Strings(allTools)

	for _, toolName := range allTools {
		mode := effectiveModes[toolName]
		// If global default is active (confirm/auto/disabled) and this tool
		// has no per-tool override in config, show the global default as effective mode.
		if globalDefault != "" && globalDefault != "custom" {
			if _, hasOwn := h.cfg.LLM.ToolModes[toolName]; !hasOwn {
				mode = globalDefault
			}
		}
		sb.WriteString(fmt.Sprintf("  %-35s %s\n", toolName, mode))
	}

	return sb.String()
}
