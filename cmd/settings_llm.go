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

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleLLMSetting handles LLM-related settings: api-key, endpoint, model, temperature,
// max-tokens, vision, thinking-enabled, reasoning-effort, toolcall-enabled, top-p, top-k,
// repetition-penalty, max-model-len.
func (h *SettingsHandler) handleLLMSetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "api-key":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set api-key <key>")
		}
		activeModel := config.GetActiveModelFromConfig(h.cfg)
		if activeModel != nil {
			activeModel.APIKey = args[1]
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("API key updated")
		return i18n.T(i18n.KeySettingsUpdated), nil

	case "endpoint":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set endpoint <url>")
		}
		activeModel := config.GetActiveModelFromConfig(h.cfg)
		if activeModel != nil {
			activeModel.Endpoint = args[1]
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("Endpoint updated to %s", args[1])
		return i18n.T(i18n.KeyEndpointUpdated), nil

	case "model":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set model <model>")
		}
		activeModel := config.GetActiveModelFromConfig(h.cfg)
		if activeModel != nil {
			activeModel.Model = args[1]
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
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
		if tokens < -1 {
			return "", fmt.Errorf("max-tokens must be -1 (not sent) or >= 0")
		}
		h.cfg.LLM.MaxTokens = tokens
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("Max tokens set to %d", tokens)
		return i18n.TF(i18n.KeyMaxTokensUpdated, tokens), nil

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
		h.rebuildLLMClient()
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.VisionSupport {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Vision support set to %s", status)
		return fmt.Sprintf("✅ 视觉识别已设置为: %s", status), nil

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
		h.rebuildLLMClient()
		log.Info("Reasoning effort set to %s", effort)
		return fmt.Sprintf("✅ 推理努力程度已设置为: %s", effort), nil

	case "toolcall-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ToolCallEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf("工具调用: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ToolCallEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ToolCallEnabled = false
		default:
			return "", fmt.Errorf("usage: .set toolcall-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetToolCallEnabled(h.cfg.LLM.ToolCallEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ToolCallEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("ToolCall enabled set to %s", status)
		return fmt.Sprintf("✅ 工具调用已设置为: %s", status), nil

	case "top-p":
		if len(args) < 2 {
			return fmt.Sprintf("Top-P 采样参数: %.1f", h.cfg.LLM.TopP), nil
		}
		val, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的 Top-P 值: %s", args[1])
		}
		if val < -1 || val > 1 {
			return "", fmt.Errorf("Top-P 必须在 -1.0 ~ 1.0 之间（-1 表示不发送）")
		}
		h.cfg.LLM.TopP = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("Top-P set to %.1f", val)
		return fmt.Sprintf("✅ Top-P 采样参数已设置为: %.1f", val), nil

	case "top-k":
		if len(args) < 2 {
			return fmt.Sprintf("Top-K 采样参数: %d", h.cfg.LLM.TopK), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 Top-K 值: %s", args[1])
		}
		if n < -1 {
			return "", fmt.Errorf("Top-K 必须 >= -1（-1 表示不发送）")
		}
		h.cfg.LLM.TopK = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("Top-K set to %d", n)
		return fmt.Sprintf("✅ Top-K 采样参数已设置为: %d", n), nil

	case "repetition-penalty":
		if len(args) < 2 {
			return fmt.Sprintf("重复惩罚参数: %.1f", h.cfg.LLM.RepetitionPenalty), nil
		}
		val, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("无效的重复惩罚值: %s", args[1])
		}
		if val < -1 || val > 2 {
			return "", fmt.Errorf("重复惩罚参数必须在 -1.0 ~ 2.0 之间（-1 表示不发送）")
		}
		h.cfg.LLM.RepetitionPenalty = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.rebuildLLMClient()
		log.Info("Repetition penalty set to %.1f", val)
		return fmt.Sprintf("✅ 重复惩罚参数已设置为: %.1f", val), nil

	case "max-model-len":
		if len(args) < 2 {
			return fmt.Sprintf("模型最大上下文长度: %d", h.cfg.LLM.MaxModelLen), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的数值: %s", args[1])
		}
		if n < 0 {
			return "", fmt.Errorf("模型最大上下文长度必须 >= 0")
		}
		h.cfg.LLM.MaxModelLen = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Max model len set to %d", n)
		return fmt.Sprintf("✅ 模型最大上下文长度已设置为: %d", n), nil

	case "toolcall-mode":
		if len(args) < 2 {
			mode := h.cfg.LLM.ToolCallMode
			if mode == "" {
				mode = "openai"
			}
			return fmt.Sprintf("工具调用模式: %s（可选值: openai, xml）", mode), nil
		}
		mode := args[1]
		switch mode {
		case "openai", "xml":
			h.cfg.LLM.ToolCallMode = mode
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolCallMode(mode)
			log.Info("Tool call mode set to %s", mode)
			return fmt.Sprintf("✅ 工具调用模式已设置为: %s", mode), nil
		default:
			return "", fmt.Errorf("无效的工具调用模式: %s（可选值: openai, xml）", mode)
		}

	default:
		return "", fmt.Errorf("unknown LLM setting: %s", subcommand)
	}
}
