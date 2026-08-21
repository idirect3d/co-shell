// Web UI identity & personality settings support (FEATURE-393): IdentityJSON
// returns the current identity fields (name/description/principles/capabilities/
// rules) so the browser can render an identity form, and SaveIdentity persists
// a single field. name/description/principles are stored in config.json,
// capabilities in the external CAPABILITIES.md file, and rules in cfg.Rules.
//
// Author: L.Shuang
// Created: 2026-08-21
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/log"
)

// IdentityField is one identity & personality field exposed to the Web UI.
type IdentityField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"` // "text" (single-line) | "textarea" (multi-line)
}

// IdentityJSON returns the current identity & personality fields for the Web UI.
// name is single-line; description/principles/capabilities/rules are multi-line.
func (h *SettingsHandler) IdentityJSON() []IdentityField {
	cfg := h.cfg

	// name
	name := cfg.LLM.AgentName
	if name == "" {
		name = "co-shell"
	}

	// description (current work mode)
	workMode := cfg.LLM.WorkMode
	if workMode == "" {
		workMode = "act"
	}
	desc := ""
	if cfg.LLM.ModeDescriptions != nil {
		if md, ok := cfg.LLM.ModeDescriptions[workMode]; ok {
			desc = md
		}
	}
	if desc == "" {
		desc = cfg.LLM.AgentDescription
	}

	// principles (config value; PRINCIPLES.md external file takes priority)
	principles := cfg.LLM.AgentPrinciples
	if h.agent != nil {
		if p := h.agent.ExternalFile("PRINCIPLES.md"); p != "" {
			principles = p
		}
	}

	// capabilities (external CAPABILITIES.md file)
	capabilities := ""
	if h.agent != nil {
		capabilities = h.agent.ExternalFile("CAPABILITIES.md")
	}

	// rules (config rules joined by newline)
	rules := strings.Join(cfg.Rules, "\n")

	return []IdentityField{
		{Key: "name", Value: name, Type: "text"},
		{Key: "description", Value: desc, Type: "textarea"},
		{Key: "principles", Value: principles, Type: "textarea"},
		{Key: "capabilities", Value: capabilities, Type: "textarea"},
		{Key: "rules", Value: rules, Type: "textarea"},
	}
}

// SaveIdentity persists a single identity & personality field (FEATURE-393).
// name/description/principles are stored in config.json, capabilities in the
// external CAPABILITIES.md file, and rules in cfg.Rules (one per line).
func (h *SettingsHandler) SaveIdentity(key, value string) error {
	cfg := h.cfg
	switch key {
	case "name":
		cfg.LLM.AgentName = value
		if err := cfg.Save(); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.SetName(value)
		}
		log.Info("Agent name set to %s", value)
		return nil

	case "description":
		workMode := cfg.LLM.WorkMode
		if workMode == "" {
			workMode = "act"
		}
		if cfg.LLM.ModeDescriptions == nil {
			cfg.LLM.ModeDescriptions = make(map[string]string)
		}
		cfg.LLM.ModeDescriptions[workMode] = value
		if err := cfg.Save(); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.SetConfig(cfg)
		}
		log.Info("Agent description set for mode %s", workMode)
		return nil

	case "principles":
		cfg.LLM.AgentPrinciples = value
		if err := cfg.Save(); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.SetConfig(cfg)
		}
		log.Info("Agent principles set")
		return nil

	case "capabilities":
		return h.saveExternalFile("CAPABILITIES.md", value)

	case "rules":
		cfg.Rules = splitLines(value)
		if err := cfg.Save(); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.SetConfig(cfg)
		}
		log.Info("Agent rules set (%d rules)", len(cfg.Rules))
		return nil

	default:
		return fmt.Errorf("unknown identity field: %s", key)
	}
}

// saveExternalFile writes a text file to the workspace root (or cwd when no
// workspace is set), creating parent directories as needed.
func (h *SettingsHandler) saveExternalFile(filename, content string) error {
	dir := ""
	if h.agent != nil {
		dir = h.agent.WorkspacePath()
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = cwd
	}
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	if h.agent != nil {
		h.agent.RebuildSystemPrompt()
	}
	log.Info("External file %s written (%d bytes)", filename, len(content))
	return nil
}

// splitLines splits a multi-line string into non-empty trimmed lines.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
