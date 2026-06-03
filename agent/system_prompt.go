// Author: L.Shuang
// Created: 2026-04-30
// Last Modified: 2026-06-01
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

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
// shellEnabled: when true, uses Shell-session-specific prompts (no execute_command).
func buildSystemPromptWithMode(rules string, mode config.ResultMode, shellEnabled bool, agentName, agentDescription, agentPrinciples, userName, channel, taskDesc, taskPlanText string, toolUsageText ...string) string {
	sh := shellName()

	cwd, _ := os.Getwd()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	execName := filepath.Base(os.Args[0])
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}

	if agentName == "" {
		agentName = "co-shell"
	}
	if agentDescription == "" {
	}
	if agentPrinciples == "" {
	}

	// Part 1: Identity — agentDescription and agentPrinciples are now embedded in the Identity i18n resource
	identityText := i18n.TF(i18n.KeySystemPromptIdentity, agentName)

	// Part 2: Tool Usage Guide — select based on shellEnabled
	toolUsageKey := i18n.KeySystemPromptToolUsageShell
	if !shellEnabled {
		toolUsageKey = i18n.KeySystemPromptToolUsage
	}
	toolUsageSection := i18n.T(toolUsageKey)
	if len(toolUsageText) > 0 && toolUsageText[0] != "" {
		toolUsageSection = toolUsageText[0]
	}

	// Part 3: Result Mode
	resultModeText := i18n.TF(i18n.KeySystemPromptResultMode, resultModeInstruction(mode))

	// Part 4: Capabilities — select based on shellEnabled
	capabilitiesKey := i18n.KeySystemPromptCapabilitiesShell
	if !shellEnabled {
		capabilitiesKey = i18n.KeySystemPromptCapabilities
	}
	capabilities := loadExternalFile(cwd, "CAPABILITIES.md")
	if capabilities == "" {
		capabilities = strings.ReplaceAll(i18n.T(capabilitiesKey), "{CWD}", cwd)
	}

	// Part 5: Rules — select based on shellEnabled
	rulesKey := i18n.KeySystemPromptRulesShell
	if !shellEnabled {
		rulesKey = i18n.KeySystemPromptRules
	}
	rulesText := loadExternalFile(cwd, "RULES.md")
	if rulesText == "" {
		rulesText = strings.ReplaceAll(i18n.T(rulesKey), "{CWD}", cwd)
	}
	if rules != "" {
		rulesText = strings.ReplaceAll(rulesText, "{CUSTOM_RULES}", rules)
	} else {
		rulesText = strings.ReplaceAll(rulesText, "{CUSTOM_RULES}", "")
	}

	// Part 6: Objective
	objectiveText := i18n.T(i18n.KeySystemPromptObjective)
	if taskDesc != "" {
		objectiveText = strings.ReplaceAll(objectiveText, "{TASK}", taskDesc)
	}
	objectiveText = strings.ReplaceAll(objectiveText, "{TASK_TRACKING}", taskPlanText)

	// Part 7: Static Environment
	envText := i18n.T(i18n.KeySystemPromptEnvironment)
	envText = strings.ReplaceAll(envText, "{OS}", runtime.GOOS)
	envText = strings.ReplaceAll(envText, "{ARCH}", runtime.GOARCH)
	envText = strings.ReplaceAll(envText, "{COMMAND}", execName)
	envText = strings.ReplaceAll(envText, "{SHELL}", sh)
	envText = strings.ReplaceAll(envText, "{HOME}", homeDir)
	envText = strings.ReplaceAll(envText, "{CWD}", cwd)
	envText = strings.ReplaceAll(envText, "{WORKSPACE}", cwd)

	displayUser := userName
	if displayUser == "" {
		displayUser = i18n.T(i18n.KeyAnonymousUser)
	}
	displayChannel := channel
	if displayChannel == "" {
		displayChannel = "co-shell"
	}
	channelInfo := displayUser + " @ " + displayChannel

	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	envText = strings.ReplaceAll(envText, "{CURRENT_TIME}", now)
	envText = strings.ReplaceAll(envText, "{CWD}", cwd)
	envText = strings.ReplaceAll(envText, "{CURRENT_FILES}", strings.TrimRight(listFilesForPrompt(cwd, true, 100), "\n"))
	envText = strings.ReplaceAll(envText, "{CHANNEL}", channelInfo)

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
