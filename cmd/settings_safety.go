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

	case "loop-temp-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.LoopTempEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("循环温度调节: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.LoopTempEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.LoopTempEnabled = false
		default:
			return "", fmt.Errorf("使用方法: .set loop-temp-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.LoopTempEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Loop temp enabled set to %s", status)
		return fmt.Sprintf("✅ 循环温度调节已设置为: %s", status), nil

	case "loop-temp-step-up":
		if len(args) < 2 {
			return fmt.Sprintf("循环温度上升步长: %.2f", h.cfg.LLM.LoopTempStepUp), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if v <= 0 || v > 1.0 {
			return "", fmt.Errorf("上升步长必须在 0.01 ~ 1.0 之间")
		}
		h.cfg.LLM.LoopTempStepUp = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp step up set to %.2f", v)
		return fmt.Sprintf("✅ 循环温度上升步长已设置为: %.2f", v), nil

	case "loop-temp-step-down":
		if len(args) < 2 {
			return fmt.Sprintf("循环温度下降步长: %.2f", h.cfg.LLM.LoopTempStepDown), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if v <= 0 || v > 1.0 {
			return "", fmt.Errorf("下降步长必须在 0.01 ~ 1.0 之间")
		}
		h.cfg.LLM.LoopTempStepDown = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp step down set to %.2f", v)
		return fmt.Sprintf("✅ 循环温度下降步长已设置为: %.2f", v), nil

	case "loop-temp-max":
		if len(args) < 2 {
			return fmt.Sprintf("循环温度上限: %.2f", h.cfg.LLM.LoopTempMax), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if v <= h.cfg.LLM.LoopTempMin || v > 2.0 {
			return "", fmt.Errorf("温度上限必须大于下限且 <= 2.0")
		}
		h.cfg.LLM.LoopTempMax = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp max set to %.2f", v)
		return fmt.Sprintf("✅ 循环温度上限已设置为: %.2f", v), nil

	case "loop-temp-min":
		if len(args) < 2 {
			return fmt.Sprintf("循环温度下限: %.2f", h.cfg.LLM.LoopTempMin), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if v >= h.cfg.LLM.LoopTempMax || v < 0 {
			return "", fmt.Errorf("温度下限必须小于上限且 >= 0")
		}
		h.cfg.LLM.LoopTempMin = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp min set to %.2f", v)
		return fmt.Sprintf("✅ 循环温度下限已设置为: %.2f", v), nil

	case "loop-judge-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.LoopJudgeEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("LLM循环二次判定: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.LoopJudgeEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.LoopJudgeEnabled = false
		default:
			return "", fmt.Errorf("使用方法: .set loop-judge-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.LoopJudgeEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Loop judge enabled set to %s", status)
		return fmt.Sprintf("✅ LLM循环二次判定已设置为: %s", status), nil

	case "duplicate-content-threshold":
		if len(args) < 2 {
			return fmt.Sprintf("内容重复判定阈值: %.2f（0.0-1.0，默认0.95）", h.cfg.LLM.DuplicateContentThreshold), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil || v < 0 || v > 1.0 {
			return "", fmt.Errorf("无效的阈值: %s（请输入 0.0-1.0 之间的小数）", args[1])
		}
		h.cfg.LLM.DuplicateContentThreshold = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Duplicate content threshold set to %.2f", v)
		return fmt.Sprintf("✅ 内容重复判定阈值已设置为: %.2f", v), nil

	case "loop-reorganize-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.LoopReorganizeEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("循环检测重整上下文: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.LoopReorganizeEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.LoopReorganizeEnabled = false
		default:
			return "", fmt.Errorf("使用方法: .set loop-reorganize-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.LoopReorganizeEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Loop reorganize enabled set to %s", status)
		return fmt.Sprintf("✅ 循环检测重整上下文已设置为: %s", status), nil

	default:
		return "", fmt.Errorf("unknown safety setting: %s", subcommand)
	}
}

// showToolModes displays the current tool mode configuration.
func (h *SettingsHandler) showToolModes() string {
	var sb strings.Builder

	modeName := h.cfg.LLM.WorkMode
	if modeName == "" || modeName == "default" {
		modeName = "act"
	}
	sb.WriteString(fmt.Sprintf("当前工作模式: %s\n\n", modeName))
	sb.WriteString("工具模式配置 (有效值):\n")

	// Use agent's effective toolModes if available (already computed by SyncToolModes)
	// Otherwise compute them the same way as SyncToolModes does.
	var effectiveModes map[string]string
	if h.agent != nil {
		// Access internal toolModes safely - for display only
		if tm := h.agent.ToolModes(); tm != nil {
			effectiveModes = tm
		}
	}
	if effectiveModes == nil {
		effectiveModes = agent.DefaultToolModes()
	}

	defaultMode := effectiveModes["default"]
	if defaultMode == "" {
		defaultMode = "confirm"
	}
	sb.WriteString(fmt.Sprintf("  默认: %s\n\n", defaultMode))

	allTools := make([]string, 0, len(agent.DefaultToolModes()))
	for name := range agent.DefaultToolModes() {
		if name != "default" {
			allTools = append(allTools, name)
		}
	}
	sort.Strings(allTools)

	for _, toolName := range allTools {
		mode := effectiveModes[toolName]
		if mode == "" {
			mode = defaultMode
		}
		sb.WriteString(fmt.Sprintf("  %-35s %s\n", toolName, mode))
	}

	return sb.String()
}
