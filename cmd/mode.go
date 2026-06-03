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
// copies or portions of the Software.
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
	"bufio"
	"fmt"
	"os"
	"strconv"
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
//	.mode                   - show current mode and list all modes
//	.mode list              - list all work modes
//	.mode switch            - interactive mode selection
//	.mode switch <name>     - switch to named mode
//	.mode create            - interactive mode creation
//	.mode edit              - interactive mode editing (reorder sections)
//	.mode edit <name>       - edit named mode
//	.mode remove            - interactive mode removal
//	.mode remove <name>     - remove named mode
func (h *ModeHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showCurrent(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.listModes(), nil
	case "switch":
		return h.interactiveSwitch(args[1:])
	case "create":
		return h.interactiveCreate()
	case "edit":
		return h.interactiveEdit(args[1:])
	case "remove", "rm":
		return h.interactiveRemove(args[1:])
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

	for i, m := range modes {
		marker := " "
		if m.Name == current {
			marker = "*"
		}
		desc := m.Description
		if desc == "" {
			desc = "-"
		}
		sb.WriteString(fmt.Sprintf("  %s [%d] %s: %s\n", marker, i+1, m.Name, desc))
		// Show section count
		if len(m.Sections) > 0 {
			sb.WriteString(fmt.Sprintf("      节数: %d\n", len(m.Sections)))
		}
	}
	return sb.String()
}

// getAllAvailableSections returns all available section names (built-in + custom).
func (h *ModeHandler) getAllAvailableSections() []string {
	var sections []string
	sections = append(sections, config.DefaultBuiltInSections()...)
	for _, s := range h.cfg.PromptSections {
		sections = append(sections, s.Name)
	}
	return sections
}

// selectModeByNumber interactively selects a mode by number.
func (h *ModeHandler) selectModeByNumber(prompt string) (*config.WorkMode, error) {
	modes := h.cfg.WorkModes
	if len(modes) == 0 {
		modes = config.DefaultWorkModes()
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Println(prompt)
	fmt.Println()
	for i, m := range modes {
		fmt.Printf("  [%d] %s", i+1, m.Name)
		if m.Description != "" {
			fmt.Printf(" - %s", m.Description)
		}
		fmt.Println()
	}
	fmt.Print("\n  请选择 (输入编号): ")

	if !scanner.Scan() {
		return nil, fmt.Errorf(i18n.T(i18n.KeyCancelled))
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return nil, fmt.Errorf(i18n.T(i18n.KeyCancelled))
	}
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(modes) {
		return nil, fmt.Errorf(i18n.T(i18n.KeyInvalidChoice))
	}
	return &modes[num-1], nil
}

// interactiveSwitch switches to a mode interactively or by name.
func (h *ModeHandler) interactiveSwitch(args []string) (string, error) {
	if len(args) > 0 {
		// Direct name provided
		name := args[0]
		if !h.modeExists(name) && name != "default" {
			return "", fmt.Errorf(i18n.T(i18n.KeyModeNotFound))
		}
		h.cfg.LLM.WorkMode = name
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil {
			h.ag.SetConfig(h.cfg)
		}
		return fmt.Sprintf(i18n.T(i18n.KeyModeSwitched), name), nil
	}

	// Interactive selection
	selected, err := h.selectModeByNumber("选择要切换的工作模式:")
	if err != nil {
		return "", err
	}
	h.cfg.LLM.WorkMode = selected.Name
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyModeSwitched), selected.Name), nil
}

// interactiveCreate creates a new work mode interactively.
func (h *ModeHandler) interactiveCreate() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("\n  新建工作模式名称: ")
	if !scanner.Scan() {
		return "", fmt.Errorf(i18n.T(i18n.KeyCancelled))
	}
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		return "", fmt.Errorf("名称不能为空")
	}

	// Check duplicates
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return "", fmt.Errorf(i18n.T(i18n.KeyModeExists))
		}
	}
	if name == "default" {
		return "", fmt.Errorf("不能创建名为 'default' 的模式")
	}

	fmt.Print("  模式描述 (可选): ")
	desc := ""
	if scanner.Scan() {
		desc = strings.TrimSpace(scanner.Text())
	}

	// Select sections
	sections := h.interactiveSelectSections("选择此模式要包含的节 (输入编号切换，空行继续):")

	newMode := config.WorkMode{
		Name:        name,
		Description: desc,
		Sections:    sections,
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes, newMode)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyModeAdded), name), nil
}

// interactiveSelectSections presents all available sections with numbers,
// allowing the user to toggle inclusion and reorder.
func (h *ModeHandler) interactiveSelectSections(prompt string) []string {
	allSections := h.getAllAvailableSections()
	selected := make(map[int]bool)
	var order []int

	// Initialize: all sections selected by default
	for i := range allSections {
		selected[i] = true
		order = append(order, i)
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println()
		fmt.Println(prompt)
		fmt.Println()
		for i, name := range allSections {
			marker := " "
			if selected[i] {
				marker = "✓"
			}
			fmt.Printf("  [%d] [%s] %s\n", i+1, marker, name)
		}
		fmt.Println()
		fmt.Println("  操作说明:")
		fmt.Println("    [编号]   - 切换选择/取消该节")
		fmt.Println("    +<编号>  - 将节上移一位 (如 +3)")
		fmt.Println("    -<编号>  - 将节下移一位 (如 -3)")
		fmt.Println("    <回车>   - 完成选择")
		fmt.Print("\n  请输入: ")

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			break
		}

		// Handle move up: +N
		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				fmt.Println("  无效编号")
				continue
			}
			idx := num - 1
			// Find position in order
			for pos, v := range order {
				if v == idx && pos > 0 {
					order[pos], order[pos-1] = order[pos-1], order[pos]
					break
				}
			}
			continue
		}

		// Handle move down: -N
		if strings.HasPrefix(input, "-") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				fmt.Println("  无效编号")
				continue
			}
			idx := num - 1
			for pos, v := range order {
				if v == idx && pos < len(order)-1 {
					order[pos], order[pos+1] = order[pos+1], order[pos]
					break
				}
			}
			continue
		}

		// Handle toggle
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(allSections) {
			fmt.Println("  无效输入")
			continue
		}
		idx := num - 1
		selected[idx] = !selected[idx]
	}

	// Build result
	var result []string
	for _, v := range order {
		if selected[v] {
			result = append(result, allSections[v])
		}
	}
	if len(result) == 0 {
		result = config.DefaultBuiltInSections()
	}
	return result
}

// interactiveEdit allows interactive reordering of sections for a mode.
func (h *ModeHandler) interactiveEdit(args []string) (string, error) {
	var mode *config.WorkMode
	if len(args) > 0 {
		for i := range h.cfg.WorkModes {
			if h.cfg.WorkModes[i].Name == args[0] {
				mode = &h.cfg.WorkModes[i]
				break
			}
		}
		if mode == nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyModeNotFound))
		}
	} else {
		selected, err := h.selectModeByNumber("选择要编辑的工作模式:")
		if err != nil {
			return "", err
		}
		// Find the actual pointer
		for i := range h.cfg.WorkModes {
			if h.cfg.WorkModes[i].Name == selected.Name {
				mode = &h.cfg.WorkModes[i]
				break
			}
		}
		if mode == nil {
			return "", fmt.Errorf("cannot find mode")
		}
	}

	// Show current sections and allow reordering
	allSections := h.getAllAvailableSections()

	// Build current section index mapping
	currentIndices := make([]int, 0, len(mode.Sections))
	for _, s := range mode.Sections {
		for i, a := range allSections {
			if a == s {
				currentIndices = append(currentIndices, i)
				break
			}
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\n  编辑模式: %s\n", mode.Name)
		fmt.Println("  当前节顺序:")
		for pos, idx := range currentIndices {
			fmt.Printf("    [%d] %s\n", pos+1, allSections[idx])
		}
		fmt.Println()
		fmt.Println("  操作说明:")
		fmt.Println("    +<序号>  - 上移 (如 +2)")
		fmt.Println("    -<序号>  - 下移 (如 -3)")
		fmt.Println("    a<编号>  - 添加未包含的节 (如 a5)")
		fmt.Println("    d<序号>  - 移除此节 (如 d2)")
		fmt.Println("    完成    - 保存并退出")
		fmt.Print("\n  请输入: ")

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" || input == "完成" {
			break
		}

		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				fmt.Println("  无效序号")
				continue
			}
			pos := num - 1
			if pos > 0 {
				currentIndices[pos], currentIndices[pos-1] = currentIndices[pos-1], currentIndices[pos]
			}
			continue
		}

		if strings.HasPrefix(input, "-") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				fmt.Println("  无效序号")
				continue
			}
			pos := num - 1
			if pos < len(currentIndices)-1 {
				currentIndices[pos], currentIndices[pos+1] = currentIndices[pos+1], currentIndices[pos]
			}
			continue
		}

		if strings.HasPrefix(input, "a") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				fmt.Println("  无效编号")
				continue
			}
			// Check if already in current
			already := false
			for _, v := range currentIndices {
				if v == num-1 {
					already = true
					break
				}
			}
			if !already {
				currentIndices = append(currentIndices, num-1)
			}
			continue
		}

		if strings.HasPrefix(input, "d") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				fmt.Println("  无效序号")
				continue
			}
			pos := num - 1
			currentIndices = append(currentIndices[:pos], currentIndices[pos+1:]...)
			continue
		}

		fmt.Println("  无效输入")
	}

	// Build updated sections
	mode.Sections = make([]string, 0, len(currentIndices))
	for _, idx := range currentIndices {
		mode.Sections = append(mode.Sections, allSections[idx])
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}

	return fmt.Sprintf("已更新模式 %s 的节顺序 (%d 节)", mode.Name, len(mode.Sections)), nil
}

// interactiveRemove removes a mode interactively or by name.
func (h *ModeHandler) interactiveRemove(args []string) (string, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		selected, err := h.selectModeByNumber("选择要删除的工作模式:")
		if err != nil {
			return "", err
		}
		name = selected.Name
	}

	// Confirm deletion
	fmt.Printf("  确定要删除工作模式 '%s'? (y/N): ", name)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf(i18n.T(i18n.KeyCancelled))
	}
	confirm := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if confirm != "y" && confirm != "yes" {
		return "", fmt.Errorf(i18n.T(i18n.KeyCancelled))
	}

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

	if h.cfg.LLM.WorkMode == name {
		h.cfg.LLM.WorkMode = "default"
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes[:idx], h.cfg.WorkModes[idx+1:]...)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyModeRemoved), name), nil
}

func (h *ModeHandler) modeExists(name string) bool {
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return true
		}
	}
	return false
}
