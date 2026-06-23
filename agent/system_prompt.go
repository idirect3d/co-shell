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
		var modeDescKey string
		switch modeName {
		case "act":
			modeDescKey = i18n.KeyWorkModeAct
		case "plan":
			modeDescKey = i18n.KeyWorkModePlan
		case "research":
			modeDescKey = i18n.KeyWorkModeResearch
		}
		if modeDescKey != "" {
			if desc := i18n.T(modeDescKey); desc != "" && desc != modeDescKey {
				return i18n.TF(i18n.KeySystemPromptResultMode, desc)
			}
		}
		return ""
	case "Capabilities":
		return i18n.T(i18n.KeySystemPromptCapabilities)
	case "Rules":
		return i18n.T(i18n.KeySystemPromptRules)
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
		return i18n.T(i18n.KeySystemPromptXMLExamples)
	case "TaskProgress":
		return i18n.T(i18n.KeySystemPromptXMLTaskProgress)
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
	text = strings.ReplaceAll(text, "{TASK}", env.taskDesc)
	text = strings.ReplaceAll(text, "{CUSTOM_RULES}", env.customRules)
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
	os                    string
	arch                  string
	shell                 string
	homeDir               string
	cwd                   string
	execName              string
	taskDesc              string
	customRules           string
	shellEnabled          bool
	mode                  config.ResultMode
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
func buildNamedSection(name string, env *promptEnv, cfg *config.Config, shellEnabled bool, toolUsageText []string) string {
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
		key := i18n.KeySystemPromptToolUsageShell
		if !shellEnabled {
			key = i18n.KeySystemPromptToolUsage
		}
		text := loadSectionText(env.cwd, modeName, "TOOL_USAGE", func() string {
			return i18n.T(key)
		})
		if len(toolUsageText) > 0 && toolUsageText[0] != "" {
			text = toolUsageText[0]
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
			// Priority 2: get mode-specific description and wrap with WORK MODE template
			var modeDescKey string
			switch modeName {
			case "act":
				modeDescKey = i18n.KeyWorkModeAct
			case "plan":
				modeDescKey = i18n.KeyWorkModePlan
			case "research":
				modeDescKey = i18n.KeyWorkModeResearch
			}
			if modeDescKey != "" {
				if desc := i18n.T(modeDescKey); desc != "" && desc != modeDescKey {
					return i18n.TF(i18n.KeySystemPromptResultMode, desc)
				}
			}
			return ""
		})
		return buildSectionWithPlaceholders(text, env)

	case "Capabilities":
		key := i18n.KeySystemPromptCapabilitiesShell
		if !shellEnabled {
			key = i18n.KeySystemPromptCapabilities
		}
		// Plan mode: use read-only capabilities (no write/execute guidance)
		if modeName == "plan" {
			key = i18n.KeySystemPromptCapabilitiesReadOnly
		}
		text := loadSectionText(env.cwd, modeName, "CAPABILITIES", func() string {
			return i18n.T(key)
		})
		return buildSectionWithPlaceholders(text, env)

	case "Rules":
		key := i18n.KeySystemPromptRulesShell
		if !shellEnabled {
			key = i18n.KeySystemPromptRules
		}
		// Plan mode: use read-only rules (no write/execute guidance)
		if modeName == "plan" {
			key = i18n.KeySystemPromptRulesReadOnly
		}
		text := loadSectionText(env.cwd, modeName, "RULES", func() string {
			return i18n.T(key)
		})
		return buildSectionWithPlaceholders(text, env)

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
			return i18n.T(i18n.KeySystemPromptXMLExamples)
		})
		return buildSectionWithPlaceholders(text, env)

	case "TaskProgress":
		text := loadSectionText(env.cwd, modeName, "TASK_PROGRESS", func() string {
			return i18n.T(i18n.KeySystemPromptXMLTaskProgress)
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
func buildSystemPromptWithMode(cfg *config.Config, rules string, mode config.ResultMode, shellEnabled bool, agentName, agentDescription, agentPrinciples, userName, channel, taskDesc, taskPlanText string, toolUsageText ...string) string {
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

	// Get section names from work mode config (or default order)
	sectionNames := getWorkModeSectionNames(cfg, "")
	separator := i18n.T(i18n.KeySectionSeparator)
	var sections []string
	for _, name := range sectionNames {
		section := buildNamedSection(name, env, cfg, shellEnabled, toolUsageText)
		if section == "" {
			continue
		}
		if len(sections) > 0 {
			sections = append(sections, separator)
		}
		sections = append(sections, section)
	}
	return strings.Join(sections, "")
}

func resultModeInstruction(mode config.ResultMode) string {
	switch mode {
	case config.ResultModeMinimal:
		return `When you execute a system command and receive its output, do NOT repeat the command output in your response. Instead, simply indicate whether the command succeeded or failed. If it succeeded, respond with a brief success confirmation (e.g., "✅ 命令执行成功" or "✅ Command executed successfully"). If it failed, respond with a brief error message. Do not add any additional explanation, analysis, or commentary.`
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
