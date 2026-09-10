// Author: L.Shuang
// Created: 2026-04-30
// Last Modified: 2026-06-03
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

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// loadExternalFile attempts to load a text file from the workspace root directory.
func loadExternalFile(workspacePath, filename string) string {
	if workspacePath == "" || filename == "" {
		return ""
	}
	filePath := filepath.Join(workspacePath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// loadRulesDir reads all *.md files from dir, sorted by filename (case-insensitive,
// so .md and .MD are both accepted). Each file is formatted as
// "\n====\n{basename-without-.md}\n\n{content}" and joined by "\n\n".
// The title is the filename with the ".md" suffix removed (no "# " prefix).
// Empty or whitespace-only files are skipped.
//
// FEATURE-417/418: subdirectories under dir are NOT loaded (their content stays
// out of the system prompt to avoid bloat). Instead, each subdirectory and its
// .md files are listed as an on-demand rule tree, formatted with markdown
// headings so the LLM knows what on-demand rules exist and can read them with
// read_file when needed. Subdirectories are traversed recursively (deeper
// nesting = deeper heading level).
// Returns "" when the directory does not exist and there is nothing to show.
func loadRulesDir(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, e.Name())
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	sort.Strings(subdirs)
	var sb strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		// Title: filename without ".md" suffix, preceded by a "====" separator line.
		// A blank line follows the separator so the title does not hug the line.
		title := strings.TrimSuffix(name, filepath.Ext(name))
		sb.WriteString("====\n\n" + title + "\n\n" + trimmed)
	}
	// FEATURE-417/418: append the on-demand rule tree (subdirectories + their
	// .md files), formatted with markdown headings and recursive nesting.
	if len(subdirs) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("====\n\n可用规则类型（按需加载，需要时直接用 read_file 读取对应路径）\n\n")
		for _, sd := range subdirs {
			appendRulesTree(&sb, filepath.Join(dir, sd), sd, 1)
		}
	}
	return strings.TrimSpace(sb.String())
}

// appendRulesTree recursively lists a subdirectory's .md files (and nested
// subdirectories) as a markdown heading tree. level is the current heading depth
// (1 = "#", 2 = "##", ...). Only file paths are listed, never file contents.
func appendRulesTree(sb *strings.Builder, dir, name string, level int) {
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(strings.Repeat("#", level) + " " + name + "\n")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, e.Name())
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	sort.Strings(subdirs)
	for _, f := range files {
		sb.WriteString(strings.Repeat("#", level+1) + " " + strings.TrimSuffix(f, filepath.Ext(f)) + ": " + filepath.Join(dir, f) + "\n")
	}
	for _, sd := range subdirs {
		appendRulesTree(sb, filepath.Join(dir, sd), sd, level+1)
	}
}

// loadExternalFileWithMode attempts to load a text file with mode support.
// Priority:
// 1. {cwd}/mode/{modeName}/{filename} (if modeName is set and file exists)
// 2. {cwd}/{filename} (root fallback)
// 3. empty string (caller should use i18n fallback)
func loadExternalFileWithMode(cwd, modeName, filename string) string {
	if cwd == "" || filename == "" {
		return ""
	}
	// Priority 1: mode-specific path
	if modeName != "" {
		modePath := filepath.Join(cwd, "mode", modeName, filename)
		if data, err := os.ReadFile(modePath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	// Priority 2: root path
	rootPath := filepath.Join(cwd, filename)
	if data, err := os.ReadFile(rootPath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getWorkModeSectionNames returns the list of section names for the given work mode name.
// Falls back to default sections if the mode doesn't exist in config.
func getWorkModeSectionNames(cfg *config.Config, modeName string) []string {
	if modeName == "" && cfg != nil {
		modeName = cfg.LLM.WorkMode
	}
	// Search user-defined modes first
	if cfg != nil {
		for _, wm := range cfg.WorkModes {
			if wm.Name == modeName && len(wm.Sections) > 0 {
				return wm.Sections
			}
		}
	}
	// Fall back to built-in modes (act, plan)
	for _, wm := range config.DefaultWorkModes() {
		if wm.Name == modeName && len(wm.Sections) > 0 {
			return wm.Sections
		}
	}
	return config.DefaultBuiltInSections()
}

// sectionFileName maps built-in section names (e.g. "Identity") to their
// external filename convention (e.g. "IDENTITY") used by loadSectionText.
// Custom sections return the name as-is.
func sectionFileName(name string) string {
	switch name {
	case "Identity":
		return "IDENTITY"
	case "ToolUsage":
		return "TOOL_USAGE"
	case "ResultMode":
		return "RESULT_MODE"
	case "Capabilities":
		return "CAPABILITIES"
	case "Rules":
		return "RULES"
	case "Skills":
		return "SKILLS"
	case "Objective":
		return "OBJECTIVE"
	case "ExternalTools":
		return "EXTERNAL_TOOLS"
	case "Environment":
		return "ENVIRONMENT"
	case "ToolExamples":
		return "TOOL_EXAMPLES"
	case "TaskProgress":
		return "TASK_PROGRESS"
	case "EditingFiles":
		return "EDITING_FILES"
	case "BrowserUsage":
		return "BROWSER_USAGE"
	}
	return name
}

// UnloadModeSections exports all section content for the given mode to
// mode/{modeName}/ directory as .md files. Each file contains the raw
// section text with placeholders intact, so users can edit them before
// co-shell loads them at runtime.
func UnloadModeSections(cfg *config.Config, modeName string) error {
	if modeName == "" {
		return fmt.Errorf("mode name is required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}

	// Get section names for this mode
	sectionNames := getWorkModeSectionNames(cfg, modeName)

	// Create mode directory
	modeDir := filepath.Join(cwd, "mode", modeName)
	if err := os.MkdirAll(modeDir, 0755); err != nil {
		return fmt.Errorf("cannot create mode directory %s: %w", modeDir, err)
	}

	written := 0
	for _, name := range sectionNames {
		// Get raw text without placeholder substitution
		text := getRawSectionText(name, modeName, cwd, cfg)
		if text == "" {
			continue
		}

		// Use the same filename convention as loadSectionText (UPPER_CASE for builtins)
		fileName := sectionFileName(name) + ".md"
		filePath := filepath.Join(modeDir, fileName)
		if err := os.WriteFile(filePath, []byte(text), 0644); err != nil {
			return fmt.Errorf("cannot write section %s: %w", name, err)
		}
		written++
	}

	if written == 0 {
		return fmt.Errorf("no sections found for mode: %s", modeName)
	}

	return nil
}

// getRawSectionText returns the raw section text (with placeholders intact)
// for the given section name. It mirrors the i18n fallback logic of
// buildNamedSection but skips placeholder substitution.
func getRawSectionText(name, modeName, cwd string, cfg *config.Config) string {
	switch name {
	case "Identity":
		return i18n.T(i18n.KeySystemPromptIdentity)
	case "ToolUsage":
		return i18n.T(i18n.KeySystemPromptToolUsage)
	case "ResultMode":
		if cwd != "" {
			rootPath := filepath.Join(cwd, "WORKMODE.md")
			if data, err := os.ReadFile(rootPath); err == nil {
				return strings.TrimSpace(string(data))
			}
		}
		// FEATURE-472: statically list all configured work modes.
		return buildResultModeSection(cfg)
	case "Capabilities":
		return i18n.T(i18n.KeySystemPromptCapabilities)
	case "Rules":
		return i18n.T(i18n.KeySystemPromptRules)
	case "Skills":
		return i18n.T(i18n.KeySystemPromptSkills)
	case "Objective":
		if modeName == "plan" {
			if planObj := i18n.T(i18n.KeySystemPromptObjectivePlan); planObj != "" && planObj != i18n.KeySystemPromptObjectivePlan {
				return planObj
			}
		}
		return i18n.T(i18n.KeySystemPromptObjective)
	case "ExternalTools":
		return i18n.T(i18n.KeySystemPromptExternalTools)
	case "Environment":
		return i18n.T(i18n.KeySystemPromptEnvironment)
	case "ToolExamples":
		// OpenAI mode uses JSON-style tool call examples, XML mode uses
		// XML-style examples with the configurable tag prefix.
		if cfg != nil && cfg.LLM.ToolCallMode == "openai" {
			return i18n.T(i18n.KeySystemPromptToolUsageExamples)
		}
		return i18n.T(i18n.KeySystemPromptXMLExamples)
	case "TaskProgress":
		return i18n.T(i18n.KeySystemPromptToolUsageTaskProgress)
	case "EditingFiles":
		return i18n.T(i18n.KeySystemPromptEditingFiles)
	case "BrowserUsage":
		return i18n.T(i18n.KeySystemPromptBrowserUsage)
	default:
		// Custom section
		if cfg != nil {
			for _, ps := range cfg.PromptSections {
				if ps.Name == name && ps.Content != "" {
					return ps.Content
				}
			}
		}
		i18nKey := "system_prompt_" + strings.ToLower(name)
		if i18n.T(i18nKey) != "" {
			return i18n.T(i18nKey)
		}
		return ""
	}
}

// modeDetailKey maps a built-in work mode name to its detailed-description
// i18n key (used in the RESULT MODE section). Custom modes have no dedicated
// key and fall back to their WorkMode.Description.
func modeDetailKey(name string) string {
	switch name {
	case "act":
		return i18n.KeyWorkModeAct
	case "plan":
		return i18n.KeyWorkModePlan
	case "research":
		return i18n.KeyWorkModeResearch
	}
	return ""
}

// collectAllWorkModes merges the built-in default work modes with the
// user-defined ones from cfg.WorkModes. A user-defined mode with the same name
// as a built-in overrides it (kept at the built-in's original position).
// Returns the merged list in built-in order followed by extra custom modes.
func collectAllWorkModes(cfg *config.Config) []config.WorkMode {
	builtins := config.DefaultWorkModes()
	if cfg == nil || len(cfg.WorkModes) == 0 {
		return builtins
	}
	// Map user-defined modes by name for override lookup.
	customByName := make(map[string]config.WorkMode, len(cfg.WorkModes))
	for _, wm := range cfg.WorkModes {
		customByName[wm.Name] = wm
	}
	result := make([]config.WorkMode, 0, len(builtins)+len(cfg.WorkModes))
	seen := make(map[string]bool, len(builtins)+len(cfg.WorkModes))
	// Built-ins first; a same-named custom mode replaces the built-in entry.
	for _, b := range builtins {
		if c, ok := customByName[b.Name]; ok {
			result = append(result, c)
		} else {
			result = append(result, b)
		}
		seen[b.Name] = true
	}
	// Append custom modes not shadowing any built-in.
	for _, wm := range cfg.WorkModes {
		if !seen[wm.Name] {
			result = append(result, wm)
			seen[wm.Name] = true
		}
	}
	return result
}

// modeDescription returns the human-readable description of a work mode for
// the RESULT MODE section. Priority: a user-defined override's Description,
// then the built-in detailed i18n key (KeyWorkModeAct/Plan/Research), then the
// mode's own Description.
func modeDescription(cfg *config.Config, m config.WorkMode) string {
	// User-defined override wins.
	if cfg != nil {
		for _, wm := range cfg.WorkModes {
			if wm.Name == m.Name && wm.Description != "" {
				return wm.Description
			}
		}
	}
	// Built-in detailed i18n key (act/plan/research).
	if key := modeDetailKey(m.Name); key != "" {
		if desc := i18n.T(key); desc != "" && desc != key {
			return desc
		}
	}
	return m.Description
}

// buildResultModeSection builds the static RESULT MODE section that lists all
// configured work modes (built-in + user-defined) so the LLM understands the
// differences between every mode, not just the currently active one (FEATURE-472).
// Format:
//
//	ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE
//
//	In each user message, the environment_details will specify the current mode. There are 3 modes:
//
//	# ACT MODE
//	<description>
//
//	# PLAN MODE
//	<description>
//
//	# RESEARCH MODE
//	<description>
func buildResultModeSection(cfg *config.Config) string {
	modes := collectAllWorkModes(cfg)
	if len(modes) == 0 {
		return ""
	}
	var sb strings.Builder
	// Title: ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE
	for i, m := range modes {
		if i > 0 {
			sb.WriteString(" V.S. ")
		}
		sb.WriteString(strings.ToUpper(m.Name))
		sb.WriteString(" MODE")
	}
	sb.WriteString("\n\n")
	// Lead sentence.
	lead := i18n.T(i18n.KeySystemPromptResultModeLead)
	if lead == "" || lead == i18n.KeySystemPromptResultModeLead {
		lead = "In each user message, the environment_details will specify the current mode. There are %d modes:"
	}
	sb.WriteString(fmt.Sprintf(lead, len(modes)))
	sb.WriteString("\n\n")
	// Each mode description: a markdown heading followed by the (possibly
	// multi-line) description body.
	for _, m := range modes {
		desc := modeDescription(cfg, m)
		if desc == "" {
			continue
		}
		sb.WriteString("# ")
		sb.WriteString(strings.ToUpper(m.Name))
		sb.WriteString(" MODE\n\n")
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}
	// FEATURE-503: trailing note telling the LLM how to export and tweak the
	// per-mode strategies via --unload-mode.
	if note := i18n.T(i18n.KeySystemPromptResultModeNote); note != "" && note != i18n.KeySystemPromptResultModeNote {
		sb.WriteString(note)
	}
	return strings.TrimSpace(sb.String())
}

// buildSectionWithPlaceholders returns a prompt section after applying all
// named placeholders (e.g. {AGENT_NAME}, {OS}, etc.) to the given text.
func buildSectionWithPlaceholders(text string, env *promptEnv) string {
	text = strings.ReplaceAll(text, "{AGENT_NAME}", env.agentName)
	text = strings.ReplaceAll(text, "{AGENT_DESCRIPTION}", env.agentDescription)
	text = strings.ReplaceAll(text, "{AGENT_PRINCIPLES}", env.agentPrinciples)
	text = strings.ReplaceAll(text, "{USER_NAME}", env.userName)
	text = strings.ReplaceAll(text, "{CHANNEL}", env.channelInfo)
	text = strings.ReplaceAll(text, "{RESULT_MODE_INSTRUCTION}", env.resultModeInstruction)
	text = strings.ReplaceAll(text, "{OS}", env.os)
	text = strings.ReplaceAll(text, "{ARCH}", env.arch)
	text = strings.ReplaceAll(text, "{SHELL}", env.shell)
	text = strings.ReplaceAll(text, "{HOME}", env.homeDir)
	text = strings.ReplaceAll(text, "{COMMAND}", env.execName)
	text = strings.ReplaceAll(text, "{WORKSPACE}", env.cwd)
	text = strings.ReplaceAll(text, "{LANG}", env.lang)
	text = strings.ReplaceAll(text, "{TASK}", env.taskDesc)
	text = strings.ReplaceAll(text, "{CUSTOM_RULES}", env.customRules)
	// FEATURE-472: inject the configured context-reorganize-threshold percentage
	// into the RULES section so the LLM knows when to proactively reorganize.
	text = strings.ReplaceAll(text, "{CONTEXT_REORGANIZE_THRESHOLD}", env.contextReorganizeThreshold)
	return text
}

// promptEnv holds all environment values reused across multiple sections.
type promptEnv struct {
	agentName             string
	agentDescription      string
	agentPrinciples       string
	userName              string
	channelInfo           string
	resultModeInstruction string
	// contextReorganizeThreshold is the configured token-usage percentage that
	// triggers an automatic reorganize_context suggestion (FEATURE-472).
	contextReorganizeThreshold string
	os                         string
	arch                       string
	shell                      string
	homeDir                    string
	cwd                        string
	execName                   string
	taskDesc                   string
	customRules                string
	shellEnabled               bool
	mode                       config.ResultMode
	lang                       string
}

// getModeSectionPath returns the path to a section file for the current work mode.
// Format: {cwd}/mode/{modeName}/{sectionName}.md
func getModeSectionPath(cwd, modeName, sectionName string) string {
	if modeName == "" {
		return ""
	}
	return filepath.Join(cwd, "mode", modeName, sectionName+".md")
}

// loadSectionText loads section content for a given work mode.
// Priority:
// 1. {cwd}/mode/{modeName}/{name}.md (if modeName is set and file exists)
// 2. i18n fallback (handled by caller via fallbackFn)
func loadSectionText(cwd, modeName, name string, fallbackFn func() string) string {
	if modeName != "" {
		modePath := getModeSectionPath(cwd, modeName, name)
		if data, err := os.ReadFile(modePath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return fallbackFn()
}

// buildNamedSection builds a single named prompt section. The section name determines
// the source: custom sections look for {Name}.md, built-in names use i18n keys.
// toolUsageText is passed through for sections that may need it (ToolUsage).
func buildNamedSection(name string, env *promptEnv, cfg *config.Config, shellEnabled bool, toolUsageText []string, intentExposureEnabled bool) string {
	modeName := ""
	if cfg != nil {
		modeName = cfg.LLM.WorkMode
	}
	switch name {
	case "Identity":
		text := loadSectionText(env.cwd, modeName, "IDENTITY", func() string {
			return i18n.T(i18n.KeySystemPromptIdentity)
		})
		return buildSectionWithPlaceholders(text, env)

	case "ToolUsage":
		text := loadSectionText(env.cwd, modeName, "TOOL_USAGE", func() string {
			return i18n.T(i18n.KeySystemPromptToolUsage)
		})
		if len(toolUsageText) > 0 && toolUsageText[0] != "" {
			text = toolUsageText[0]
		}
		// FEATURE-447: inject the standalone meta object description into the
		// {META_DESCRIPTION} placeholder when intent exposure is enabled.
		if intentExposureEnabled {
			text = strings.ReplaceAll(text, "{META_DESCRIPTION}", i18n.T(i18n.KeySystemPromptToolUsageMetaOpenAI))
		} else {
			text = strings.ReplaceAll(text, "{META_DESCRIPTION}", "")
		}
		return buildSectionWithPlaceholders(text, env)

	case "ResultMode":
		text := loadSectionText(env.cwd, modeName, "RESULT_MODE", func() string {
			// Priority 1: root-level {cwd}/WORKMODE.md (multi-mode config file)
			if env.cwd != "" {
				rootPath := filepath.Join(env.cwd, "WORKMODE.md")
				if data, err := os.ReadFile(rootPath); err == nil {
					return strings.TrimSpace(string(data))
				}
			}
			// Priority 2 (FEATURE-472): statically list all configured work modes
			// (built-in + user-defined) so the LLM understands the differences
			// between every mode, not just the currently active one.
			return buildResultModeSection(cfg)
		})
		return buildSectionWithPlaceholders(text, env)

	case "Capabilities":
		text := loadSectionText(env.cwd, modeName, "CAPABILITIES", func() string {
			return i18n.T(i18n.KeySystemPromptCapabilities)
		})
		// FEATURE-466: when meta-capability awareness is enabled, append the
		// meta-capability index (list of native meta-capabilities with stable
		// IDs) to the CAPABILITIES section. The LLM can query full details via
		// the introspect_capability tool.
		if cfg != nil && cfg.LLM.MetaCapabilityEnabled {
			index := buildMetaCapabilityIndex()
			if index != "" {
				header := i18n.T(i18n.KeySystemPromptCapabilitiesIndex)
				if header == "" || header == i18n.KeySystemPromptCapabilitiesIndex {
					header = "META-CAPABILITIES"
				}
				if text != "" {
					text += "\n\n"
				}
				text += header + "\n\n" + index
			}
		}
		return buildSectionWithPlaceholders(text, env)

	case "Rules":
		text := loadSectionText(env.cwd, modeName, "RULES", func() string {
			return i18n.T(i18n.KeySystemPromptRules)
		})
		return buildSectionWithPlaceholders(text, env)

	case "Skills":
		// FEATURE-453: list skill index (name + description + path) without
		// loading SKILL.md content. The LLM reads SKILL.md on demand via read_file.
		skills := scanSkills(env.cwd, env.homeDir)
		index := buildSkillsIndex(skills)
		header := i18n.T(i18n.KeySystemPromptSkills)
		if header == "" || header == i18n.KeySystemPromptSkills {
			header = "SKILLS"
		}
		if index == "" {
			return header
		}
		// Insert the index before the trailing parenthesized note (the last
		// non-empty paragraph of the header), so the note stays at the very end.
		// The note opens with a full-width paren in zh and a half-width one in en.
		for _, open := range []string{"\n\n\uff08", "\n\n("} {
			if i := strings.LastIndex(header, open); i >= 0 {
				return header[:i] + "\n\n" + index + header[i:]
			}
		}
		return header + "\n\n" + index

	case "Objective":
		text := loadSectionText(env.cwd, modeName, "OBJECTIVE", func() string {
			if modeName == "plan" {
				if planObj := i18n.T(i18n.KeySystemPromptObjectivePlan); planObj != "" && planObj != i18n.KeySystemPromptObjectivePlan {
					return planObj
				}
			}
			return i18n.T(i18n.KeySystemPromptObjective)
		})
		return buildSectionWithPlaceholders(text, env)

	case "ExternalTools":
		text := loadSectionText(env.cwd, modeName, "EXTERNAL_TOOLS", func() string {
			return i18n.T(i18n.KeySystemPromptExternalTools)
		})
		return buildSectionWithPlaceholders(text, env)

	case "Environment":
		text := loadSectionText(env.cwd, modeName, "ENVIRONMENT", func() string {
			return i18n.T(i18n.KeySystemPromptEnvironment)
		})
		return buildSectionWithPlaceholders(text, env)

	case "ToolExamples":
		text := loadSectionText(env.cwd, modeName, "TOOL_EXAMPLES", func() string {
			// OpenAI mode uses JSON-style tool call examples, XML mode uses
			// XML-style examples with the configurable tag prefix.
			if cfg != nil && cfg.LLM.ToolCallMode == "openai" {
				return i18n.T(i18n.KeySystemPromptToolUsageExamples)
			}
			return i18n.T(i18n.KeySystemPromptXMLExamples)
		})
		return buildSectionWithPlaceholders(text, env)

	case "TaskProgress":
		text := loadSectionText(env.cwd, modeName, "TASK_PROGRESS", func() string {
			return i18n.T(i18n.KeySystemPromptToolUsageTaskProgress)
		})
		return buildSectionWithPlaceholders(text, env)

	case "EditingFiles":
		text := loadSectionText(env.cwd, modeName, "EDITING_FILES", func() string {
			return i18n.T(i18n.KeySystemPromptEditingFiles)
		})
		return buildSectionWithPlaceholders(text, env)

	case "BrowserUsage":
		text := loadSectionText(env.cwd, modeName, "BROWSER_USAGE", func() string {
			return i18n.T(i18n.KeySystemPromptBrowserUsage)
		})
		return buildSectionWithPlaceholders(text, env)

	default:
		// Custom user-defined section: try mode/{modeName}/{name}.md, then {name}.md,
		// then Content from PromptSection, then i18n fallback.
		text := loadSectionText(env.cwd, modeName, name, func() string {
			// Check PromptSections from config for inline content
			if cfg != nil {
				for _, ps := range cfg.PromptSections {
					if ps.Name == name && ps.Content != "" {
						return ps.Content
					}
				}
			}
			// Try i18n fallback
			i18nKey := "system_prompt_" + strings.ToLower(name)
			if i18n.T(i18nKey) != "" {
				return i18n.T(i18nKey)
			}
			return ""
		})
		return buildSectionWithPlaceholders(text, env)
	}
}

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
// The cfg parameter provides work mode configuration (can be nil, uses default order).
// shellEnabled: when true, uses Shell-session-specific prompts (no execute_command).
//
// Named placeholders (e.g. {AGENT_NAME}, {CWD}, {TASK}) are resolved for all sections
// regardless of source (external file or i18n).
//
// Each built-in section is separated by the section separator (i18n KeySectionSeparator),
// but only between non-empty sections.
func buildSystemPromptWithMode(cfg *config.Config, rules string, mode config.ResultMode, shellEnabled bool, agentName, agentDescription, agentPrinciples, userName, channel, taskDesc, taskPlanText string, intentExposureEnabled bool, toolUsageText ...string) string {
	env := &promptEnv{}
	env.cwd, _ = os.Getwd()
	env.shell = shellName()
	env.os = runtime.GOOS
	env.arch = runtime.GOARCH
	env.execName = filepath.Base(os.Args[0])
	env.homeDir, _ = os.UserHomeDir()
	if env.homeDir == "" {
		env.homeDir = os.Getenv("HOME")
	}
	if env.homeDir == "" {
		env.homeDir = os.Getenv("USERPROFILE")
	}
	env.mode = mode
	env.shellEnabled = shellEnabled
	env.lang = string(i18n.GetLang())

	if agentName == "" {
		agentName = "co-shell"
	}
	env.agentName = agentName
	env.agentDescription = agentDescription
	env.agentPrinciples = agentPrinciples
	env.userName = userName
	if env.userName == "" {
		env.userName = i18n.T(i18n.KeyAnonymousUser)
	}
	displayChannel := channel
	if displayChannel == "" {
		displayChannel = "co-shell"
	}
	env.channelInfo = env.userName + " @ " + displayChannel
	env.taskDesc = taskDesc
	env.customRules = rules
	env.resultModeInstruction = resultModeInstruction(mode)
	// FEATURE-472: expose the configured context-reorganize-threshold percentage
	// (default 80) so the RULES section can tell the LLM when to proactively
	// reorganize context before the system forces it.
	threshold := 80
	if cfg != nil && cfg.LLM.ContextReorganizeThreshold > 0 {
		threshold = cfg.LLM.ContextReorganizeThreshold
	}
	env.contextReorganizeThreshold = strconv.Itoa(threshold)

	// Get section names from work mode config (or default order)
	sectionNames := getWorkModeSectionNames(cfg, "")
	separator := i18n.T(i18n.KeySectionSeparator)
	var sections []string
	for _, name := range sectionNames {
		section := buildNamedSection(name, env, cfg, shellEnabled, toolUsageText, intentExposureEnabled)
		if section == "" {
			continue
		}
		// Trim each section so the separator's surrounding blank lines are
		// consistent regardless of each section's leading/trailing newlines.
		section = strings.TrimSpace(section)
		if len(sections) > 0 {
			sections = append(sections, separator)
		}
		sections = append(sections, section)
	}
	return strings.Join(sections, "")
}

// resolveAgentDescription returns the agent description for the current work
// mode using the same priority chain as the main system prompt:
//  1. ModeDescriptions[workMode]
//  2. cfg.LLM.AgentDescription
//  3. mode-specific i18n default (act/plan/research)
//  4. global i18n default
//
// FEATURE-346: used by the minimal recognition round so the vision model also
// receives the agent identity description.
func resolveAgentDescription(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	workMode := cfg.LLM.WorkMode
	if workMode == "" {
		workMode = "act"
	}
	if cfg.LLM.ModeDescriptions != nil {
		if md, ok := cfg.LLM.ModeDescriptions[workMode]; ok && md != "" {
			return md
		}
	}
	if cfg.LLM.AgentDescription != "" {
		return cfg.LLM.AgentDescription
	}
	switch workMode {
	case "plan":
		if d := i18n.T(i18n.KeyAgentDefaultDescriptionPlan); d != "" && d != i18n.KeyAgentDefaultDescriptionPlan {
			return d
		}
	case "research":
		if d := i18n.T(i18n.KeyAgentDefaultDescriptionResearch); d != "" && d != i18n.KeyAgentDefaultDescriptionResearch {
			return d
		}
	default:
		if d := i18n.T(i18n.KeyAgentDefaultDescriptionAct); d != "" && d != i18n.KeyAgentDefaultDescriptionAct {
			return d
		}
	}
	return i18n.T(i18n.KeyAgentDefaultDescription)
}

// buildVisionIdentityPrompt returns a minimal system prompt containing ONLY the
// Identity section, used for the vision recognition round in minimal
// vision-context mode (FEATURE-343). The visual model performing recognition
// should not receive tool-usage instructions or other agent context — it is a
// dedicated OCR/vision pass, not a tool-calling round.
func buildVisionIdentityPrompt(cfg *config.Config) string {
	if cfg == nil {
		// Minimal fallback: plain Identity text without placeholders.
		return i18n.T(i18n.KeySystemPromptIdentity)
	}
	env := &promptEnv{}
	env.cwd, _ = os.Getwd()
	env.shell = shellName()
	env.os = runtime.GOOS
	env.arch = runtime.GOARCH
	env.execName = filepath.Base(os.Args[0])
	env.homeDir, _ = os.UserHomeDir()
	if env.homeDir == "" {
		env.homeDir = os.Getenv("HOME")
	}
	if env.homeDir == "" {
		env.homeDir = os.Getenv("USERPROFILE")
	}
	env.mode = config.ResultMode(cfg.LLM.ResultMode)
	env.agentName = cfg.LLM.AgentName
	if env.agentName == "" {
		env.agentName = "co-shell"
	}
	env.agentDescription = resolveAgentDescription(cfg)
	env.agentPrinciples = cfg.LLM.AgentPrinciples
	env.userName = cfg.LLM.UserName
	if env.userName == "" {
		env.userName = i18n.T(i18n.KeyAnonymousUser)
	}
	env.channelInfo = env.userName + " @ " + cfg.LLM.Channel
	env.customRules = ""

	// Build only the Identity section with the mode's section order so that
	// a mode-specific IDENTITY.md (or work mode identity) is respected.
	modeName := cfg.LLM.WorkMode
	if modeName == "" {
		modeName = "act"
	}
	return buildNamedSection("Identity", env, cfg, false, nil, false)
}

func resultModeInstruction(mode config.ResultMode) string {
	switch mode {
	case config.ResultModeMinimal:
		return i18n.T(i18n.KeyCmdExecOkNoRepeat)
	case config.ResultModeExplain:
		return `When you execute a system command and receive its output, provide a brief explanation of what the output means. Keep your explanation concise (2-3 sentences max). Focus on the key information the user would want to know.`
	case config.ResultModeAnalyze:
		return `When you execute a system command and receive its output, perform a thorough analysis. Explain patterns, anomalies, and implications in detail. Provide actionable insights and recommendations based on the output.`
	case config.ResultModeFree:
		return `You have full autonomy to decide how to present command execution results. Use your judgment to determine the best way to respond based on the context and the user's needs.`
	default:
		return ""
	}
}
