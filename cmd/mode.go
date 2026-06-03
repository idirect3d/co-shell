// Author: L.Shuang
// Created: 2026-06-03
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// ModeHandler handles the .mode built-in command.
type ModeHandler struct {
	cfg *config.Config
	ag  *agent.Agent
}

// NewModeHandler creates a new ModeHandler.
func NewModeHandler(cfg *config.Config, ag *agent.Agent) *ModeHandler {
	return &ModeHandler{cfg: cfg, ag: ag}
}

// Handle processes .mode commands.
// Syntax:
//
//	.mode                         - show current mode
//	.mode list                    - list all work modes
//	.mode switch <name>           - switch to a work mode
//	.mode create <name> [desc]    - create a new work mode with default sections
//	.mode remove <name>           - remove a work mode
func (h *ModeHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showCurrent(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.listModes(), nil
	case "switch":
		return h.switchMode(args[1:])
	case "create":
		return h.createMode(args[1:])
	case "remove", "rm":
		return h.removeMode(args[1:])
	default:
		return "", fmt.Errorf("unknown mode subcommand: %s", subcommand)
	}
}

func (h *ModeHandler) showCurrent() string {
	modeName := h.cfg.LLM.WorkMode
	if modeName == "" {
		modeName = "default"
	}
	return fmt.Sprintf(i18n.T(i18n.KeyModeCurrent), modeName)
}

func (h *ModeHandler) listModes() string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyModeList))
	sb.WriteString("\n")

	modes := h.cfg.WorkModes
	if len(modes) == 0 {
		modes = config.DefaultWorkModes()
	}

	current := h.cfg.LLM.WorkMode
	if current == "" {
		current = "default"
	}

	for _, m := range modes {
		marker := " "
		if m.Name == current {
			marker = "*"
		}
		desc := m.Description
		if desc == "" {
			desc = "-"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", marker, m.Name, desc))
	}
	return sb.String()
}

func (h *ModeHandler) switchMode(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mode switch <name>")
	}

	name := args[0]

	// Find the mode in config
	found := false
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			found = true
			break
		}
	}
	if !found && name != "default" {
		return "", fmt.Errorf(i18n.T(i18n.KeyModeNotFound))
	}

	h.cfg.LLM.WorkMode = name
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	// Rebuild system prompt
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyModeSwitched), name), nil
}

func (h *ModeHandler) createMode(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mode create <name> [description]")
	}

	name := args[0]
	desc := ""
	if len(args) > 1 {
		desc = strings.Join(args[1:], " ")
	}

	// Check for duplicate
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return "", fmt.Errorf(i18n.T(i18n.KeyModeExists))
		}
	}

	newMode := config.WorkMode{
		Name:        name,
		Description: desc,
		Sections:    config.DefaultBuiltInSections(),
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes, newMode)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyModeAdded), name), nil
}

func (h *ModeHandler) removeMode(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .mode remove <name>")
	}

	name := args[0]
	idx := -1
	for i, m := range h.cfg.WorkModes {
		if m.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf(i18n.T(i18n.KeyModeNotFound))
	}

	// If switching away from removed current mode
	if h.cfg.LLM.WorkMode == name {
		h.cfg.LLM.WorkMode = "default"
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes[:idx], h.cfg.WorkModes[idx+1:]...)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	// Rebuild system prompt
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyModeRemoved), name), nil
}
