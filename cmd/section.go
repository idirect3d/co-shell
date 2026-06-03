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
//	.section                          - list all sections
//	.section add <name> [content]     - add a new custom section
//	.section remove <name>            - remove a custom section
//	.section clear                    - clear all custom sections
func (h *SectionHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.listSections(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "add":
		return h.addSection(args[1:])
	case "remove", "rm":
		return h.removeSection(args[1:])
	case "clear":
		return h.clearSections()
	default:
		return "", fmt.Errorf(i18n.T(i18n.KeySectionInvalid))
	}
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
			return "", fmt.Errorf(i18n.T(i18n.KeySectionInvalid))
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
		return "", fmt.Errorf("usage: .section remove <name>")
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
