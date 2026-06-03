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
	if workspacePath == "" {
		return ""
	}
	filePath := filepath.Join(workspacePath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// buildSectionWithPlaceholders returns a prompt section after applying all
// named placeholders (e.g. {AGENT_NAME}, {CWD}, {OS}, etc.) to the given text.
// Placeholders are resolved at call time using the provided environment values.
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

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
// shellEnabled: when true, uses Shell-session-specific prompts (no execute_command).
//
// Each section (Identity, ToolUsage, ResultMode, Capabilities, Rules, Objective, Environment)
// can be overridden by placing a corresponding .md file in the workspace root directory:
//
//	IDENTITY.md, TOOL_USAGE.md, RESULT_MODE.md, CAPABILITIES.md, RULES.md,
//	OBJECTIVE.md, ENVIRONMENT.md
//
// If the external file does not exist, the built-in i18n resource is used as fallback.
//
// Named placeholders (e.g. {AGENT_NAME}, {CWD}, {TASK}) are resolved for all sections
// regardless of source (external file or i18n).
func buildSystemPromptWithMode(rules string, mode config.ResultMode, shellEnabled bool, agentName, agentDescription, agentPrinciples, userName, channel, taskDesc, taskPlanText string, toolUsageText ...string) string {
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

	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
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

	// Part 1: Identity
	identityText := loadExternalFile(env.cwd, "IDENTITY.md")
	if identityText == "" {
		identityText = i18n.TF(i18n.KeySystemPromptIdentity, env.agentName)
		// TF uses %s, which we keep; but also support named placeholder through buildSectionWithPlaceholders
	}
	identityText = buildSectionWithPlaceholders(identityText, env)

	// Part 2: Tool Usage Guide
	toolUsageKey := i18n.KeySystemPromptToolUsageShell
	if !shellEnabled {
		toolUsageKey = i18n.KeySystemPromptToolUsage
	}
	toolUsageSection := loadExternalFile(env.cwd, "TOOL_USAGE.md")
	if toolUsageSection == "" {
		toolUsageSection = i18n.T(toolUsageKey)
	}
	if len(toolUsageText) > 0 && toolUsageText[0] != "" {
		toolUsageSection = toolUsageText[0]
	}
	toolUsageSection = buildSectionWithPlaceholders(toolUsageSection, env)

	// Part 3: Result Mode
	resultModeText := loadExternalFile(env.cwd, "RESULT_MODE.md")
	if resultModeText == "" {
		resultModeText = i18n.TF(i18n.KeySystemPromptResultMode, env.resultModeInstruction)
	}
	resultModeText = buildSectionWithPlaceholders(resultModeText, env)

	// Part 4: Capabilities
	capabilitiesKey := i18n.KeySystemPromptCapabilitiesShell
	if !shellEnabled {
		capabilitiesKey = i18n.KeySystemPromptCapabilities
	}
	capabilities := loadExternalFile(env.cwd, "CAPABILITIES.md")
	if capabilities == "" {
		capabilities = i18n.T(capabilitiesKey)
	}
	capabilities = buildSectionWithPlaceholders(capabilities, env)

	// Part 5: Rules
	rulesKey := i18n.KeySystemPromptRulesShell
	if !shellEnabled {
		rulesKey = i18n.KeySystemPromptRules
	}
	rulesText := loadExternalFile(env.cwd, "RULES.md")
	if rulesText == "" {
		rulesText = i18n.T(rulesKey)
	}
	rulesText = buildSectionWithPlaceholders(rulesText, env)

	// Part 6: Objective
	objectiveText := loadExternalFile(env.cwd, "OBJECTIVE.md")
	if objectiveText == "" {
		objectiveText = i18n.T(i18n.KeySystemPromptObjective)
	}
	objectiveText = buildSectionWithPlaceholders(objectiveText, env)

	// Part 7: Environment
	envText := loadExternalFile(env.cwd, "ENVIRONMENT.md")
	if envText == "" {
		envText = i18n.T(i18n.KeySystemPromptEnvironment)
	}
	envText = buildSectionWithPlaceholders(envText, env)

	prompt := identityText + toolUsageSection + resultModeText + capabilities + rulesText + envText + objectiveText

	return prompt
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
