// Author: L.Shuang
// Created: 2026-08-30
// Last Modified: 2026-08-30
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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
)

// SkillHandler handles the .skill built-in command.
type SkillHandler struct {
	agent *agent.Agent
}

// NewSkillHandler creates a new SkillHandler.
func NewSkillHandler(ag *agent.Agent) *SkillHandler {
	return &SkillHandler{agent: ag}
}

// Handle processes .skill commands: list, show, add, remove.
func (h *SkillHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return i18n.T(i18n.KeySkillCmdHelp), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.handleList()
	case "show":
		return h.handleShow(args[1:])
	case "add":
		return h.handleAdd(args[1:])
	case "remove":
		return h.handleRemove(args[1:])
	case "load":
		return h.handleLoad(args[1:], false)
	case "reload":
		return h.handleLoad(args[1:], true)
	case "unload":
		return h.handleUnload(args[1:])
	default:
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
}

// workspacePath returns the workspace root, falling back to the current
// working directory when no workspace is set.
func (h *SkillHandler) workspacePath() string {
	if h.agent != nil {
		if wp := h.agent.WorkspacePath(); wp != "" {
			return wp
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// homeDir returns the user's home directory.
func (h *SkillHandler) homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// skillsDir returns the skills directory for the given scope ("workspace" or
// "global"), creating it if needed.
func (h *SkillHandler) skillsDir(global bool) string {
	if global {
		return filepath.Join(h.homeDir(), ".co-shell", "skills")
	}
	return filepath.Join(h.workspacePath(), "skills")
}

func (h *SkillHandler) handleList() (string, error) {
	infos := h.skillInfos()
	if len(infos) == 0 {
		return i18n.T(i18n.KeySkillCmdNoSkills), nil
	}
	loadedTag := i18n.T(i18n.KeySkillCmdListLoadedTag)
	changedTag := i18n.T(i18n.KeySkillCmdListChangedTag)
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySkillCmdListHeader))
	sb.WriteString("\n")
	for _, info := range infos {
		sb.WriteString("- ")
		if info.Loaded {
			sb.WriteString(loadedTag)
			sb.WriteString(" ")
		}
		if !info.Enabled {
			sb.WriteString(i18n.T(i18n.KeySettingsSkillsDisabled))
			sb.WriteString(" ")
		}
		sb.WriteString(info.Name)
		sb.WriteString(": ")
		sb.WriteString(info.Description)
		sb.WriteString(fmt.Sprintf(" [%s, %d B]", info.Source, info.Bytes))
		if info.Changed {
			sb.WriteString(" ")
			sb.WriteString(changedTag)
		}
		sb.WriteString(" (")
		sb.WriteString(info.Path)
		sb.WriteString(")\n")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// skillInfos returns the discovered skills with their state. It falls back to a
// plain scan when no agent is attached (offline listing).
func (h *SkillHandler) skillInfos() []agent.SkillInfo {
	if h.agent != nil {
		return h.agent.SkillInfos()
	}
	skills := agent.ScanSkills(h.workspacePath(), h.homeDir())
	out := make([]agent.SkillInfo, 0, len(skills))
	for _, s := range skills {
		out = append(out, agent.SkillInfo{Name: s.Name, Description: s.Description, Path: s.Path, Enabled: true})
	}
	return out
}

// handleLoad implements :skill load <name> and :skill reload <name>.
// reload refuses to re-pin a skill whose file disappeared; both share the same
// implementation because loading an already-loaded skill is a refresh.
func (h *SkillHandler) handleLoad(args []string, reload bool) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	if h.agent == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	name := args[0]
	ls, _, err := h.agent.SkillLoadForUser(name)
	if err != nil {
		return "", err
	}
	if reload {
		return fmt.Sprintf(i18n.T(i18n.KeySkillCmdReloadOk), ls.Name, ls.Bytes, ls.SHA256[:8]), nil
	}
	return fmt.Sprintf(i18n.T(i18n.KeySkillCmdLoadOk), ls.Name, ls.Source, ls.Bytes, h.agent.LoadedSkillBytes()), nil
}

// handleUnload implements :skill unload <name>.
func (h *SkillHandler) handleUnload(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	if h.agent == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	name := args[0]
	if _, ok := h.agent.SkillUnload(name); !ok {
		return "", fmt.Errorf(i18n.T(i18n.KeySkillCmdUnloadNone), name)
	}
	return fmt.Sprintf(i18n.T(i18n.KeySkillCmdUnloadOk), name), nil
}

func (h *SkillHandler) handleShow(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	name := args[0]
	skills := agent.ScanSkills(h.workspacePath(), h.homeDir())
	for _, s := range skills {
		if s.Name == name {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				return "", fmt.Errorf("cannot read %s: %w", s.Path, err)
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf(i18n.T(i18n.KeySkillCmdNotFound), name)
}

func (h *SkillHandler) handleAdd(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	global := false
	var src string
	for _, a := range args {
		if a == "--global" {
			global = true
			continue
		}
		if src == "" {
			src = a
		}
	}
	if src == "" {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}

	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf(i18n.T(i18n.KeySkillCmdSourceNotExist), src)
	}

	// The skill name is the source directory's base name.
	name := filepath.Base(src)
	destDir := filepath.Join(h.skillsDir(global), name)
	if err := copyDir(src, destDir); err != nil {
		return "", err
	}
	return fmt.Sprintf(i18n.T(i18n.KeySkillCmdAdded), destDir), nil
}

func (h *SkillHandler) handleRemove(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeySkillCmdUsage))
	}
	name := args[0]
	skills := agent.ScanSkills(h.workspacePath(), h.homeDir())
	for _, s := range skills {
		if s.Name == name {
			if err := os.RemoveAll(s.Dir); err != nil {
				return "", err
			}
			return fmt.Sprintf(i18n.T(i18n.KeySkillCmdRemoved), name), nil
		}
	}
	return "", fmt.Errorf(i18n.T(i18n.KeySkillCmdNotFound), name)
}

// copyDir recursively copies the source directory to the destination.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
