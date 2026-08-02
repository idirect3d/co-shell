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
	"errors"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleDisplaySetting handles display/output-related settings: show-llm-thinking,
// show-llm-content, show-command, show-tool, show-tool-input, show-tool-output,
// show-command-output, emoji-enabled, show-logo, result-mode.
func (h *SettingsHandler) handleDisplaySetting(subcommand string, args []string) (string, error) {
	switch subcommand {
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

	case "show-loop-detection":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.ShowLoopDetection {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf("显示循环检测过程: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowLoopDetection = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowLoopDetection = false
		default:
			return "", fmt.Errorf("usage: .set show-loop-detection on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowLoopDetection {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show loop detection set to %s", status)
		return fmt.Sprintf("✅ 显示循环检测过程已设置为: %s", status), nil

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
		h.agent.SetResultMode(config.ResultMode(mode))
		log.Info("Result mode set to %s", args[1])
		return fmt.Sprintf("✅ 结果处理模式已设置为: %s", config.ResultModeString(mode)), nil

	case "show-logo":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShowLogo {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("启动 Logo 显示: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowLogo = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowLogo = false
		default:
			return "", fmt.Errorf("usage: .set show-logo on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowLogo {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show logo set to %s", status)
		return fmt.Sprintf("✅ 启动 Logo 显示已设置为: %s", status), nil

	case "token-usage":
		if len(args) < 2 {
			current := h.cfg.LLM.TokenUsage
			if current == "" {
				current = "on"
			}
			return fmt.Sprintf("Token 用量显示模式: %s", current), nil
		}
		switch args[1] {
		case "on", "off", "none":
			h.cfg.LLM.TokenUsage = args[1]
		default:
			return "", fmt.Errorf("usage: :set token-usage on|off|none")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.GetLLMClient().SetTokenUsage(h.cfg.LLM.TokenUsage)
		log.Info("Token usage set to %s", h.cfg.LLM.TokenUsage)
		return fmt.Sprintf("✅ Token 用量显示模式已设置为: %s", h.cfg.LLM.TokenUsage), nil

	case "output-categories":
		if len(args) < 2 {
			var sb strings.Builder
			sb.WriteString(i18n.T(i18n.KeyOutputCategoriesTitle))
			sb.WriteString("\n")
			for _, cat := range config.DefaultOutputCategories() {
				status := i18n.T(i18n.KeyOn)
				if !h.cfg.OutputCategoryShown(cat) {
					status = i18n.T(i18n.KeyOff)
				}
				sb.WriteString(fmt.Sprintf("  %-12s %s\n", cat, status))
			}
			return sb.String(), nil
		}
		if len(args) < 3 {
			return "", errors.New(i18n.T(i18n.KeyOutputCategoriesUsage))
		}
		cat := args[1]
		if !validOutputCategory(cat) {
			return "", fmt.Errorf(i18n.TF(i18n.KeyOutputCategoriesUnknown), cat)
		}
		if h.cfg.LLM.OutputCategories == nil {
			h.cfg.LLM.OutputCategories = map[string]bool{}
			// New map: seed with all default categories on.
			for _, c := range config.DefaultOutputCategories() {
				h.cfg.LLM.OutputCategories[c] = true
			}
		}
		switch args[2] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.OutputCategories[cat] = true
		case "off", "0", "false", "no":
			h.cfg.LLM.OutputCategories[cat] = false
		default:
			return "", errors.New(i18n.T(i18n.KeyOutputCategoriesUsage))
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.OutputCategoryShown(cat) {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Output category %s set to %s", cat, status)
		return fmt.Sprintf(i18n.T(i18n.KeyOutputCategoriesSet), cat, status), nil

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
		h.agent.SetEmojiEnabled(h.cfg.LLM.EmojiEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.EmojiEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Emoji enabled set to %s", status)
		return fmt.Sprintf("✅ 表情符号前缀已设置为: %s", status), nil

	default:
		return "", fmt.Errorf("unknown display setting: %s", subcommand)
	}
}

// validOutputCategory reports whether cat is a known output channel category.
func validOutputCategory(cat string) bool {
	for _, c := range config.DefaultOutputCategories() {
		if c == cat {
			return true
		}
	}
	return false
}
