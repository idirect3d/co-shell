// Web UI identity & personality settings support (FEATURE-393, FEATURE-531).
// IdentityJSON returns the current identity fields (name/description/
// principles/capabilities/rules) so the browser can render an identity form,
// and SaveIdentity persists a single field.
//
// Since FEATURE-531 every agent personalization value is stored in workspace
// files - ./IDENTITY.md (structured fields: agent-name, agent-description,
// user-name, user-description), ./PRINCIPLES.md, ./RULES.md and
// ./CAPABILITIES.md - and never in config.json, so that several agents sharing
// one config.json keep their own identity.
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

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// IdentityField is one identity & personality field exposed to the Web UI.
type IdentityField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`  // "text" (single-line) | "textarea" (multi-line)
	Label string `json:"label"` // localized field label
}

// identityDir returns the directory holding the personalization files: the
// agent workspace root when set, else the process working directory.
func (h *SettingsHandler) identityDir() string {
	if h.agent != nil {
		if d := h.agent.WorkspacePath(); d != "" {
			return d
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// identityFileContent reads a personalization file (e.g. RULES.md) from the
// identity directory; returns "" when the file is missing.
func (h *SettingsHandler) identityFileContent(name string) string {
	dir := h.identityDir()
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// IdentityJSON returns the current identity & personality fields for the Web UI.
// name/description/user-name/user-description come from ./IDENTITY.md, the rest
// from the matching workspace file (FEATURE-531 follow-up). name and user-name
// are single-line; the others are multi-line textareas.
func (h *SettingsHandler) IdentityJSON() []IdentityField {
	dir := h.identityDir()

	// name (./IDENTITY.md, fallback "co-shell")
	name := agent.ResolveAgentName(dir)

	// description (./IDENTITY.md, fallback i18n default; no longer per mode)
	desc := agent.ResolveAgentDescription(dir)

	// user-name / user-description (./IDENTITY.md, FEATURE-534). The user name
	// falls back to the legacy config value and finally to the anonymous
	// default, so the form shows the effective value.
	userFallback := ""
	if h.cfg != nil {
		userFallback = h.cfg.LLM.UserName
	}
	userName := agent.ResolveUserName(dir, userFallback)
	userDesc := agent.ResolveUserDescription(dir)

	// principles (./PRINCIPLES.md, fallback built-in default)
	principles := h.identityFileContent("PRINCIPLES.md")
	if principles == "" {
		principles = i18n.T(i18n.KeyAgentDefaultPrinciples)
	}

	// capabilities (./CAPABILITIES.md, fallback built-in default)
	capabilities := h.identityFileContent("CAPABILITIES.md")
	if capabilities == "" {
		capabilities = i18n.T(i18n.KeySystemPromptCapabilities)
	}

	// rules (./RULES.md; falls back to the built-in default so the form shows
	// what is actually in effect — FEATURE-535)
	rules := h.identityFileContent("RULES.md")
	if rules == "" {
		rules = strings.TrimSpace(i18n.T(i18n.KeySystemPromptRules))
	}

	return []IdentityField{
		{Key: "name", Value: name, Type: "text", Label: i18n.T(i18n.KeyCol3Name)},
		{Key: "description", Value: desc, Type: "textarea", Label: i18n.T(i18n.KeyCol3Desc)},
		{Key: "user-name", Value: userName, Type: "text", Label: i18n.T(i18n.KeyIdentityUserName)},
		{Key: "user-description", Value: userDesc, Type: "textarea", Label: i18n.T(i18n.KeyIdentityUserDesc)},
		{Key: "principles", Value: principles, Type: "textarea", Label: i18n.T(i18n.KeyCol3Principles)},
		{Key: "capabilities", Value: capabilities, Type: "textarea", Label: i18n.T(i18n.KeyCol3Capabilities)},
		{Key: "rules", Value: rules, Type: "textarea", Label: i18n.T(i18n.KeyIdentityRules)},
	}
}

// SaveIdentity persists a single identity & personality field (FEATURE-393).
// Since FEATURE-531 the value is written to the matching workspace file
// (./IDENTITY.md field, ./PRINCIPLES.md, ./RULES.md, ./CAPABILITIES.md);
// config.json is left untouched.
func (h *SettingsHandler) SaveIdentity(key, value string) error {
	dir := h.identityDir()
	switch key {
	case "name":
		if err := agent.UpdateIdentityField(dir, "agent-name", value); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.SetName(value)
			h.agent.RebuildSystemPrompt()
		}
		log.Info("Agent name written to %s", agent.IdentityPath(dir))
		return nil

	case "description":
		if err := agent.UpdateIdentityField(dir, "agent-description", value); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.RebuildSystemPrompt()
		}
		log.Info("Agent description written to %s", agent.IdentityPath(dir))
		return nil

	case "user-name", "user-description":
		// FEATURE-534: the ./IDENTITY.md user fields; both feed the Identity
		// section ({USER_NAME} / {USER_DESCRIPTION}) so rebuild the prompt.
		if err := agent.UpdateIdentityField(dir, key, value); err != nil {
			return err
		}
		if h.agent != nil {
			h.agent.RebuildSystemPrompt()
		}
		log.Info("Identity field %s written to %s", key, agent.IdentityPath(dir))
		return nil

	case "principles":
		return h.saveExternalFile("PRINCIPLES.md", value)

	case "capabilities":
		return h.saveExternalFile("CAPABILITIES.md", value)

	case "rules":
		return h.saveExternalFile("RULES.md", value)

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
