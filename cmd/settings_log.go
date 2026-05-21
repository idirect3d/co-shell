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

	"github.com/idirect3d/co-shell/log"
)

// handleLogSetting handles the log setting.
func (h *SettingsHandler) handleLogSetting(subcommand string, args []string) (string, error) {
	if len(args) < 2 {
		currentLevel := log.LogLevelString(log.GetLevel())
		return fmt.Sprintf("日志级别: %s（可选值: debug, info, warn, error, off）", currentLevel), nil
	}
	level, ok := log.ParseLogLevel(args[1])
	if !ok {
		return "", fmt.Errorf("无效的日志级别: %s（可选值: debug, info, warn, error, off）", args[1])
	}
	h.cfg.LogLevel = args[1]
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
}
