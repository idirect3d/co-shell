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

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleSearchSetting handles search-related settings: search-max-line-length,
// search-max-result-bytes, search-context-lines, memory-search-max-content-len,
// memory-search-max-results.
func (h *SettingsHandler) handleSearchSetting(subcommand string, args []string) (string, error) {
	switch subcommand {
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
			return "", fmt.Errorf("usage: .set debug on|off")
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
		return "", fmt.Errorf("unknown search setting: %s", subcommand)
	}
}
