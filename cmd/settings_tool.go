// Author: L.Shuang
// Created: 2026-05-23
// Last Modified: 2026-05-23
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleToolSubCommand handles the ".set tool" subcommand tree.
// Supported subcommands:
//   mode openai|xml  — switch tool call mode
func (h *SettingsHandler) handleToolSubCommand(args []string) (string, error) {
	if len(args) == 0 {
		// Show current tool call mode
		mode := h.agent.ToolCallMode()
		return fmt.Sprintf("%s: %s", i18n.T(i18n.KeyToolCallMode), mode), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "mode":
		return h.handleToolMode(args[1:])
	default:
		return "", fmt.Errorf("unknown tool subcommand: %s（可用: mode）", subcommand)
	}
}

// handleToolMode handles ".set tool mode [openai|xml]".
func (h *SettingsHandler) handleToolMode(args []string) (string, error) {
	if len(args) == 0 {
		// Show current mode
		mode := h.agent.ToolCallMode()
		return fmt.Sprintf("%s: %s（可选值: openai, xml）", i18n.T(i18n.KeyToolCallMode), mode), nil
	}

	mode := args[0]
	switch mode {
	case "openai", "xml":
		// Update config
		h.cfg.LLM.ToolCallMode = mode
		if err := h.cfg.Save(); err != nil {
			return "", err
		}

		// Update agent
		h.agent.SetToolCallMode(mode)

		log.Info("Tool call mode set to %s", mode)
		return fmt.Sprintf(i18n.T(i18n.KeyToolCallModeUpdated), mode), nil
	default:
		return "", fmt.Errorf(i18n.T(i18n.KeyInvalidToolCallMode), mode)
	}
}
