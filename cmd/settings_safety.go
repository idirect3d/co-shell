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

// handleSafetySetting handles safety and confirmation settings: confirm-tool,
// error-max-single-count, error-max-type-count, loop-detect-*, dedup-*.
func (h *SettingsHandler) handleSafetySetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "confirm-tool":
		if len(args) < 2 {
			var sb strings.Builder
			sb.WriteString("工具模式配置:\n")
			confirmDefault := "confirm"
			if v, ok := h.cfg.LLM.ToolModes["default"]; ok {
				confirmDefault = v
			}
			sb.WriteString(fmt.Sprintf("  默认: %s\n\n", confirmDefault))
			allTools := []string{
				"execute_command", "read_file", "write_to_file",
				"replace_in_file", "search_files", "list_code_definition_names",
				"add_images", "remove_images", "clear_images",
				"update_settings", "list_settings", "ask_followup_question",
				"adjust_context_start",
				"launch_sub_agent", "schedule_task",
				"create_task_plan", "update_task_step", "insert_task_steps",
				"remove_task_steps", "view_task_plan",
				"get_memory_slice", "memory_search", "delete_memory",
			}
			for _, toolName := range allTools {
				mode := confirmDefault
				if v, ok := h.cfg.LLM.ToolModes[toolName]; ok {
					mode = v
				}
				sb.WriteString(fmt.Sprintf("  %-35s %s\n", toolName, mode))
			}
			return sb.String(), nil
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
		case "confirm", "auto", "disabled":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = toolName
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", toolName)
			log.Info("Confirm tool default set to %s", toolName)
			return fmt.Sprintf("工具默认模式已设置为: %s", toolName), nil
		}
		if len(args) < 3 {
			mode := "confirm"
			if v, ok := h.cfg.LLM.ToolModes[toolName]; ok {
				mode = v
			} else if v, ok := h.cfg.LLM.ToolModes["default"]; ok {
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
			return "", fmt.Errorf("usage: .set confirm-tool [<tool_name>] on|off|confirm|auto|disabled")
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
