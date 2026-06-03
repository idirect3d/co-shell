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

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// SectionHandler handles the .section built-in command.
type SectionHandler struct {
	cfg *config.Config
}

// NewSectionHandler creates a new SectionHandler.
func NewSectionHandler(cfg *config.Config) *SectionHandler {
	return &SectionHandler{cfg: cfg}
}

// Handle processes .section commands.
// Syntax:
//
//	.section                          - list all sections and available operations
//	.section list                     - list all sections
//	.section add <name> [content]     - add a new custom section
//	.section remove <name>            - remove a custom section
//	.section clear                    - clear all custom sections
func (h *SectionHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showHelp(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.listSections(), nil
	case "add":
		return h.addSection(args[1:])
	case "remove", "rm":
		return h.removeSection(args[1:])
	case "clear":
		return h.clearSections()
	default:
		// Try to treat the first argument as a section name and show its content
		name := subcommand
		// Check built-in sections first
		for _, b := range config.DefaultBuiltInSections() {
			if strings.EqualFold(b, name) {
				return fmt.Sprintf("内置节 %s 的内容可通过 .mode edit 在工作模式中查看或通过外部 .md 文件自定义", name), nil
			}
		}
		// Check custom sections
		for _, s := range h.cfg.PromptSections {
			if strings.EqualFold(s.Name, name) {
				if s.Content != "" {
					return fmt.Sprintf("自定义节 [%s]:\n%s", s.Name, s.Content), nil
				}
				return fmt.Sprintf("自定义节 [%s] (内容保存在 %s.md 文件中)", s.Name, s.Name), nil
			}
		}
		return "", fmt.Errorf("未知节: %s。使用 .section 查看可用节列表", name)
	}
}

func (h *SectionHandler) showHelp() string {
	var sb strings.Builder
	sb.WriteString("提示词节管理\n\n")
	sb.WriteString(h.listSections())
	sb.WriteString("\n可用操作:\n")
	sb.WriteString("  add <name> [text]  - 添加自定义节\n")
	sb.WriteString("  remove <name>      - 删除自定义节\n")
	sb.WriteString("  clear              - 清空所有自定义节")
	return sb.String()
}

func (h *SectionHandler) listSections() string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySectionList))
	sb.WriteString("\n")

	// List built-in sections
	sb.WriteString("\n  [")
	sb.WriteString(i18n.T(i18n.KeyDefault))
	sb.WriteString("]\n")
	for _, name := range config.DefaultBuiltInSections() {
		sb.WriteString(fmt.Sprintf("  - %s\n", name))
	}

	// List custom sections
	if len(h.cfg.PromptSections) > 0 {
		sb.WriteString("\n ")
		sb.WriteString(i18n.T(i18n.KeyCustom))
		sb.WriteString("\n")
		for _, s := range h.cfg.PromptSections {
			sb.WriteString(fmt.Sprintf("  - %s\n", s.Name))
		}
	} else {
		sb.WriteString(fmt.Sprintf("\n  %s\n", i18n.T(i18n.KeySectionNoSects)))
	}
	return sb.String()
}

func (h *SectionHandler) addSection(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .section add <name> [content]")
	}

	name := args[0]
	// Check for duplicate
	for _, s := range h.cfg.PromptSections {
		if strings.EqualFold(s.Name, name) {
			return "", fmt.Errorf("节 '%s' 已存在。使用 .section remove %s 先删除再重新添加，或使用不同名称", name, name)
		}
	}
	// Check for built-in name collision
	for _, builtIn := range config.DefaultBuiltInSections() {
		if strings.EqualFold(builtIn, name) {
			return "", fmt.Errorf("cannot create section with built-in name: %s", name)
		}
	}

	content := ""
	if len(args) > 1 {
		content = strings.Join(args[1:], " ")
	}

	h.cfg.PromptSections = append(h.cfg.PromptSections, config.PromptSection{
		Name:    name,
		Content: content,
		BuiltIn: false,
	})

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeySectionAdded), name), nil
}

func (h *SectionHandler) removeSection(args []string) (string, error) {
	if len(args) < 1 {
		if len(h.cfg.PromptSections) == 0 {
			return "", fmt.Errorf("暂无自定义节可删除")
		}
		var sb strings.Builder
		sb.WriteString("选择要删除的节:\n")
		for i, s := range h.cfg.PromptSections {
			sb.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, s.Name))
		}
		return sb.String(), nil
	}

	name := args[0]
	found := false
	for i, s := range h.cfg.PromptSections {
		if strings.EqualFold(s.Name, name) {
			h.cfg.PromptSections = append(h.cfg.PromptSections[:i], h.cfg.PromptSections[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf(i18n.T(i18n.KeySectionInvalid))
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeySectionRemoved), name), nil
}

func (h *SectionHandler) clearSections() (string, error) {
	h.cfg.PromptSections = nil
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	return i18n.T(i18n.KeySectionCleared), nil
}
