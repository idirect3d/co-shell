// Package cmd - Web settings panel support for the workspace group
// (FEATURE-553): the panel shows the current workspace path as a plain
// sentence and lets the user edit the list of folders hidden from the Web UI
// workspace tree. The list is persisted in the workspace config.json
// (tree_excludes).
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// TreeExcludesSettingKey is the settings key carrying the workspace-tree
// exclusion list: a JSON array of folder names (matched at any depth) or
// workspace-relative paths (matched exactly).
const TreeExcludesSettingKey = "tree-excludes"

// workspaceSettingsGroup builds the "Workspace" group of the Web settings
// panel: the current workspace path rendered as a plain sentence plus the
// editable exclusion list. The list is sent as a JSON array so the browser can
// render and edit it.
func (h *SettingsHandler) workspaceSettingsGroup() WebSettingGroup {
	path := ""
	if h.agent != nil {
		path = h.agent.WorkspacePath()
	}
	if path == "" {
		path = "-"
	}
	raw, err := json.Marshal(h.cfg.EffectiveTreeExcludes())
	if err != nil {
		log.Warn("workspaceSettingsGroup: cannot marshal tree excludes: %v", err)
		raw = []byte("[]")
	}
	return WebSettingGroup{
		Title: i18n.T(i18n.KeyCol3WorkspaceTreeTab),
		Kind:  "workspace",
		Items: []WebSettingItem{
			{
				Key:   "workspace-path",
				Value: path,
				// The path is embedded in the localized sentence so the Web UI
				// can show it as plain text (no read-only input).
				Desc: fmt.Sprintf(i18n.T(i18n.KeyCol3WorkspaceTreePathDesc), path),
				Type: "string",
			},
			{
				Key:   TreeExcludesSettingKey,
				Value: string(raw),
				Desc:  i18n.T(i18n.KeyCol3WorkspaceTreeExcludesDesc),
				Type:  "list",
			},
		},
	}
}

// SaveTreeExcludesJSON validates a JSON array of folder names / relative paths,
// stores it in the config and persists config.json. It returns a user-facing
// confirmation message; invalid or duplicated entries reject the whole list so
// a typo never silently changes the tree.
func (h *SettingsHandler) SaveTreeExcludesJSON(value string) (string, error) {
	var list []string
	if err := json.Unmarshal([]byte(value), &list); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyWorkspaceTreeExcludesInvalid), value)
	}
	cleaned, rejected := config.NormalizeTreeExcludes(list)
	if len(rejected) > 0 {
		return "", fmt.Errorf(i18n.T(i18n.KeyWorkspaceTreeExcludesInvalid), strings.Join(rejected, ", "))
	}
	h.cfg.TreeExcludes = cleaned
	if err := h.cfg.Save(); err != nil {
		return "", errors.New(err.Error())
	}
	return fmt.Sprintf(i18n.T(i18n.KeyWorkspaceTreeExcludesSaved), displayTreeExcludes(cleaned)), nil
}

// handleTreeExcludesSetting handles ".set tree-excludes <json array>": with no
// value it prints the current list, otherwise it validates and saves it.
func (h *SettingsHandler) handleTreeExcludesSetting(args []string) (string, error) {
	if len(args) < 2 {
		return TreeExcludesSettingKey + ": " + displayTreeExcludes(h.cfg.EffectiveTreeExcludes()), nil
	}
	return h.SaveTreeExcludesJSON(args[1])
}

// displayTreeExcludes renders a list for status messages ("-" when empty).
func displayTreeExcludes(list []string) string {
	if len(list) == 0 {
		return "-"
	}
	return strings.Join(list, ", ")
}
