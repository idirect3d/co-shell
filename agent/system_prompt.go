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

// overridableSections lists the prompt sections that may be overridden by a
// same-named file in the workspace root (FEATURE-531), e.g. ./CAPABILITIES.md.
// IDENTITY.md is no longer a raw-text override: it stores the structured
// identity fields (FEATURE-531 follow-up).
var overridableSections = map[string]bool{
	"Capabilities": true,
	"Rules":        true,
}

// modeStrategyFile returns the path of a work mode's strategy file:
// {cwd}/mode/{MODE NAME}.md (upper-case mode name, no per-mode sub-directory).
func modeStrategyFile(cwd, modeName string) string {
	if cwd == "" || modeName == "" {
		return ""
	}
	return filepath.Join(cwd, "mode", strings.ToUpper(modeName)+".md")
}

// loadModeStrategy reads a work mode's strategy text from {cwd}/mode/{MODE}.md.
// Returns "" when the file is missing or blank.
func loadModeStrategy(cwd, modeName string) string {
	path := modeStrategyFile(cwd, modeName)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// loadRootSectionFile reads the workspace-root override file for a section
// ({cwd}/{SECTION}.md). Only sections in overridableSections are overridable.
// Returns "" when the file is missing or blank.
func loadRootSectionFile(cwd, name string) string {
	if cwd == "" || !overridableSections[name] {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(cwd, strings.ToUpper(name)+".md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ExportSections implements the --init flag (FEATURE-531): it writes the three
// overridable prompt sections (IDENTITY.md, CAPABILITIES.md, RULES.md) plus one
// strategy file per configured work mode (mode/{MODE NAME}.md) into the current
// working directory. Existing target files are backed up to {name}.bak first.
// Placeholders are written verbatim so the files can be edited in place.
// Returns the list of written file paths.
func ExportSections(cfg *config.Config) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %w", err)
	}
	var written []string
	// Identity fields: structured ./IDENTITY.md. Existing values are kept so
	// re-running --init never discards the user's personalization.
	id := LoadIdentity(cwd)
	if strings.TrimSpace(id.AgentName) == "" {
		id.AgentName = "co-shell"
	}
	if strings.TrimSpace(id.AgentDescription) == "" {
		id.AgentDescription = i18n.T(i18n.KeyAgentDefaultDescription)
	}
	if strings.TrimSpace(id.UserName) == "" {
		id.UserName = i18n.T(i18n.KeyAnonymousUser)
	}
	idPath := IdentityPath(cwd)
	if err := writeWithBackup(idPath, strings.TrimSpace(IdentityText(id))); err != nil {
		return written, err
	}
	written = append(written, idPath)
	// Principles: keep the existing ./PRINCIPLES.md, else export the default.
	principles := loadExternalFile(cwd, "PRINCIPLES.md")
	if strings.TrimSpace(principles) == "" {
		principles = i18n.T(i18n.KeyAgentDefaultPrinciples)
	}
	if strings.TrimSpace(principles) != "" {
		pPath := filepath.Join(cwd, "PRINCIPLES.md")
		if err := writeWithBackup(pPath, strings.TrimSpace(principles)); err != nil {
			return written, err
		}
		written = append(written, pPath)
	}
	// Section files (workspace root, upper-case names).
	for _, name := range sectionExportNames() {
		text := rawSectionText(name)
		if strings.TrimSpace(text) == "" {
			continue
		}
		path := filepath.Join(cwd, strings.ToUpper(name)+".md")
		if err := writeWithBackup(path, text); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	// Mode strategy files: mode/{MODE}.md, no per-mode sub-directory.
	modeDir := filepath.Join(cwd, "mode")
	if err := os.MkdirAll(modeDir, 0755); err != nil {
		return written, fmt.Errorf("cannot create mode directory %s: %w", modeDir, err)
	}
	for _, m := range collectAllWorkModes(cfg) {
		text := rawModeStrategy(cfg, m)
		if strings.TrimSpace(text) == "" {
			continue
		}
		path := modeStrategyFile(cwd, m.Name)
		if err := writeWithBackup(path, text); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// sectionExportNames returns the sections exported by --init.
// sectionExportNames lists the sections exported as raw-text files by --init.
// The IDENTITY section is exported separately as structured fields.
func sectionExportNames() []string {
	return []string{"Capabilities", "Rules"}
}

// writeWithBackup writes text to path, backing up an existing file to
// {path}.bak first (FEATURE-531).
func writeWithBackup(path, text string) error {
	if _, err := os.Stat(path); err == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("cannot read existing %s: %w", path, readErr)
		}
		if writeErr := os.WriteFile(path+".bak", data, 0644); writeErr != nil {
			return fmt.Errorf("cannot back up %s: %w", path, writeErr)
		}
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(text)+"\n"), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// rawSectionText returns a section's default text with placeholders intact.
func rawSectionText(name string) string {
	switch name {
	case "Identity":
		return i18n.T(i18n.KeySystemPromptIdentity)
	case "Capabilities":
		return i18n.T(i18n.KeySystemPromptCapabilities)
	case "Rules":
		return i18n.T(i18n.KeySystemPromptRules)
	}
	return ""
}

// rawModeStrategy returns a work mode's default strategy text with placeholders
// intact: the built-in i18n key when available, otherwise the mode Description.
func rawModeStrategy(cfg *config.Config, m config.WorkMode) string {
	if key := modeDetailKey(m.Name); key != "" {
		if desc := i18n.T(key); desc != "" && desc != key {
			return desc
		}
	}
	return m.Description
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
func modeDescription(cwd string, cfg *config.Config, m config.WorkMode) string {
	if text := loadModeStrategy(cwd, m.Name); text != "" {
		return text
	}
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
func buildResultModeSection(cwd string, cfg *config.Config) string {
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
		desc := modeDescription(cwd, cfg, m)
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
	text = strings.ReplaceAll(text, "{USER_DESCRIPTION}", env.userDescription)
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
	userDescription       string
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

// loadSectionText loads a prompt section: a workspace-root override file
// (only for sections listed in overridableSections) wins over the built-in
// i18n fallback.
func loadSectionText(cwd, name string, fallbackFn func() string) string {
	if text := loadRootSectionFile(cwd, name); text != "" {
		return text
	}
	return fallbackFn()
}

// buildNamedSection builds a single named prompt section. The section name determines
// the source: custom sections look for {Name}.md, built-in names use i18n keys.
// toolUsageText is passed through for sections that may need it (ToolUsage).
func buildNamedSection(name string, env *promptEnv, cfg *config.Config, shellEnabled bool, toolUsageText []string, intentExposureEnabled bool) string {
	switch name {
	case "Identity":
		// FEATURE-531 follow-up: the IDENTITY section text is always the i18n
		// template; its values come from ./IDENTITY.md (structured fields) and
		// ./PRINCIPLES.md via placeholders. Free-form text kept in IDENTITY.md
		// is appended verbatim.
		text := i18n.T(i18n.KeySystemPromptIdentity)
		if extra := identityExtra(env.cwd); extra != "" {
			text += "\n\n" + extra
		}
		return buildSectionWithPlaceholders(text, env)

	case "ToolUsage":
		text := loadSectionText(env.cwd, name, func() string {
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
		// FEATURE-531: always generated statically by listing every configured work
		// mode's strategy (mode/{MODE}.md when present, else the i18n default).
		text := buildResultModeSection(env.cwd, cfg)
		return buildSectionWithPlaceholders(text, env)

	case "Capabilities":
		text := loadSectionText(env.cwd, name, func() string {
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
		text := loadSectionText(env.cwd, name, func() string {
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
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptObjective)
		})
		return buildSectionWithPlaceholders(text, env)

	case "ExternalTools":
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptExternalTools)
		})
		return buildSectionWithPlaceholders(text, env)

	case "Environment":
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptEnvironment)
		})
		return buildSectionWithPlaceholders(text, env)

	case "ToolExamples":
		text := loadSectionText(env.cwd, name, func() string {
			// OpenAI mode uses JSON-style tool call examples, XML mode uses
			// XML-style examples with the configurable tag prefix.
			if cfg != nil && cfg.LLM.ToolCallMode == "openai" {
				return i18n.T(i18n.KeySystemPromptToolUsageExamples)
			}
			return i18n.T(i18n.KeySystemPromptXMLExamples)
		})
		return buildSectionWithPlaceholders(text, env)

	case "TaskProgress":
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptToolUsageTaskProgress)
		})
		return buildSectionWithPlaceholders(text, env)

	case "EditingFiles":
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptEditingFiles)
		})
		return buildSectionWithPlaceholders(text, env)

	case "BrowserUsage":
		text := loadSectionText(env.cwd, name, func() string {
			return i18n.T(i18n.KeySystemPromptBrowserUsage)
		})
		return buildSectionWithPlaceholders(text, env)

	default:
		// Custom user-defined section: Content from PromptSection, then i18n fallback.
		text := loadSectionText(env.cwd, name, func() string {
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

	// Get the global section list shared by every work mode (FEATURE-531).
	sectionNames := config.DefaultSections()
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

// resolveAgentDescription returns the agent description for dir: the
// ./IDENTITY.md value when set, otherwise the built-in i18n default.
//
// FEATURE-531 follow-up: per-mode descriptions were dropped - every work mode
// shares the single identity description stored in ./IDENTITY.md, and
// config.json is no longer read.
//
// FEATURE-346: used by the minimal recognition round so the vision model also
// receives the agent identity description.
func resolveAgentDescription(dir string) string {
	if d := ResolveAgentDescription(dir); d != "" {
		return d
	}
	return i18n.T(i18n.KeyAgentDefaultDescription)
}

// resolveAgentPrinciples returns the agent principles for dir: the
// ./PRINCIPLES.md content when present, otherwise the built-in i18n default.
//
// FEATURE-531 follow-up: config.json no longer carries personalization, so
// ./PRINCIPLES.md is the only source of user-defined principles.
func resolveAgentPrinciples(dir string) string {
	if dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "PRINCIPLES.md")); err == nil {
			if p := strings.TrimSpace(string(data)); p != "" {
				return p
			}
		}
	}
	return i18n.T(i18n.KeyAgentDefaultPrinciples)
}

// buildVisionIdentityPrompt returns a minimal system prompt containing ONLY the
// Identity section, used for the vision recognition round in minimal
// vision-context mode (FEATURE-343). The visual model performing recognition
// should not receive tool-usage instructions or other agent context — it is a
// dedicated OCR/vision pass, not a tool-calling round.
func buildVisionIdentityPrompt(cfg *config.Config, dir string) string {
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
	env.agentName = ResolveAgentName(dir)
	env.agentDescription = resolveAgentDescription(dir)
	env.agentPrinciples = resolveAgentPrinciples(dir)
	env.userName = ResolveUserName(dir, cfg.LLM.UserName)
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
