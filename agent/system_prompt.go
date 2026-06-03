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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

// getWorkModeSectionNames returns the list of section names for the given work mode name.
// Falls back to default sections if the mode doesn't exist in config.
func getWorkModeSectionNames(cfg *config.Config, modeName string) []string {
	if cfg == nil || len(cfg.WorkModes) == 0 {
		return config.DefaultBuiltInSections()
	}
	if modeName == "" {
		modeName = cfg.LLM.WorkMode
	}
	for _, wm := range cfg.WorkModes {
		if wm.Name == modeName {
			if len(wm.Sections) > 0 {
				return wm.Sections
			}
			return config.DefaultBuiltInSections()
		}
	}
	return config.DefaultBuiltInSections()
}

// buildSectionWithPlaceholders returns a prompt section after applying all
// named placeholders (e.g. {AGENT_NAME}, {CWD}, {OS}, etc.) to the given text.
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
	text = strings.ReplaceAll(text, "{CWD}", env.cwd)
	text = strings.ReplaceAll(text, "{WORKSPACE}", env.cwd)
	text = strings.ReplaceAll(text, "{COMMAND}", env.execName)
	text = strings.ReplaceAll(text, "{CURRENT_TIME}", env.currentTime)
	text = strings.ReplaceAll(text, "{CURRENT_FILES}", env.currentFiles)
	text = strings.ReplaceAll(text, "{TASK}", env.taskDesc)
	text = strings.ReplaceAll(text, "{TASK_TRACKING}", env.taskPlanText)
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
	currentTime           string
	currentFiles          string
	taskDesc              string
	taskPlanText          string
	customRules           string
	shellEnabled          bool
	mode                  config.ResultMode
}

// buildNamedSection builds a single named prompt section. The section name determines
// the source: custom sections look for {Name}.md, built-in names use i18n keys.
// toolUsageText is passed through for sections that may need it (ToolUsage).
func buildNamedSection(name string, env *promptEnv, shellEnabled bool, toolUsageText []string) string {
	switch name {
	case "Identity":
		text := loadExternalFile(env.cwd, "IDENTITY.md")
		if text == "" {
			text = i18n.TF(i18n.KeySystemPromptIdentity, env.agentName)
		}
		return buildSectionWithPlaceholders(text, env)

	case "ToolUsage":
		key := i18n.KeySystemPromptToolUsageShell
		if !shellEnabled {
			key = i18n.KeySystemPromptToolUsage
		}
		text := loadExternalFile(env.cwd, "TOOL_USAGE.md")
		if text == "" {
			text = i18n.T(key)
		}
		if len(toolUsageText) > 0 && toolUsageText[0] != "" {
			text = toolUsageText[0]
		}
		return buildSectionWithPlaceholders(text, env)

	case "ResultMode":
		text := loadExternalFile(env.cwd, "RESULT_MODE.md")
		if text == "" {
			text = i18n.TF(i18n.KeySystemPromptResultMode, env.resultModeInstruction)
		}
		return buildSectionWithPlaceholders(text, env)

	case "Capabilities":
		key := i18n.KeySystemPromptCapabilitiesShell
		if !shellEnabled {
			key = i18n.KeySystemPromptCapabilities
		}
		text := loadExternalFile(env.cwd, "CAPABILITIES.md")
		if text == "" {
			text = i18n.T(key)
		}
		return buildSectionWithPlaceholders(text, env)

	case "Rules":
		key := i18n.KeySystemPromptRulesShell
		if !shellEnabled {
			key = i18n.KeySystemPromptRules
		}
		text := loadExternalFile(env.cwd, "RULES.md")
		if text == "" {
			text = i18n.T(key)
		}
		return buildSectionWithPlaceholders(text, env)

	case "Objective":
		text := loadExternalFile(env.cwd, "OBJECTIVE.md")
		if text == "" {
			text = i18n.T(i18n.KeySystemPromptObjective)
		}
		return buildSectionWithPlaceholders(text, env)

	case "Environment":
		text := loadExternalFile(env.cwd, "ENVIRONMENT.md")
		if text == "" {
			text = i18n.T(i18n.KeySystemPromptEnvironment)
		}
		return buildSectionWithPlaceholders(text, env)

	default:
		// Custom user-defined section: try {Name}.md file, then inline Content,
		// then fall back to i18n key "system_prompt_" + lowercase(name).
		text := loadExternalFile(env.cwd, name+".md")
		if text != "" {
			return buildSectionWithPlaceholders(text, env)
		}
		// Try i18n fallback for custom named sections
		i18nKey := "system_prompt_" + strings.ToLower(name)
		if i18n.T(i18nKey) != "" {
			return buildSectionWithPlaceholders(i18n.T(i18nKey), env)
		}
		return ""
	}
}

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
// The cfg parameter provides work mode configuration (can be nil, uses default order).
// shellEnabled: when true, uses Shell-session-specific prompts (no execute_command).
//
// The prompt is assembled from the sections defined by the current work mode (cfg.LLM.WorkMode),
// or uses the default 7-section order if no work mode is configured.
//
// Named placeholders (e.g. {AGENT_NAME}, {CWD}, {TASK}) are resolved for all sections
// regardless of source (external file or i18n).
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
	env.currentTime = time.Now().Format("2006-01-02 15:04:05 Monday")
	env.currentFiles = strings.TrimRight(listFilesForPrompt(env.cwd, true, 100), "\n")
	env.taskDesc = taskDesc
	env.taskPlanText = taskPlanText
	env.customRules = rules
	env.resultModeInstruction = resultModeInstruction(mode)

	// Get section names from work mode config (or default order)
	sectionNames := getWorkModeSectionNames(nil, "")
	var sections []string
	for _, name := range sectionNames {
		section := buildNamedSection(name, env, shellEnabled, toolUsageText)
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
