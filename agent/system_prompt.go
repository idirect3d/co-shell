// Author: L.Shuang
// Created: 2026-04-30
// Last Modified: 2026-04-30
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

// buildSystemPrompt constructs the system prompt with rules and context.
// Uses the default OpenAI-style tool usage text.
func buildSystemPrompt(rules string) string {
	return buildSystemPromptWithMode(rules, config.ResultModeMinimal, "", "", "", "", "", "", "", i18n.T(i18n.KeySystemPromptToolUsage))
}

// loadExternalFile attempts to load a text file from the workspace root directory.
// If the file does not exist or cannot be read, returns empty string.
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
// The prompt is built using the current i18n language setting.
// agentName, agentDescription, agentPrinciples are optional identity fields from config.
// userName is the user's name for LLM to identify different users (default: OS username).
// channel is the communication channel (co-shell, feishu, co-tor, agent).
// If workspacePath is non-empty, it tries to load capabilities.md and rules.md from the workspace
// root to override the built-in i18n defaults.
// toolUsageText is the tool usage section content to inject into the prompt.
// If empty, defaults to the standard OpenAI-style tool usage text from i18n.
// taskPlanText is an optional pre-formatted task plan text to include at the start of Objective.
// If non-empty, it replaces the {TASK_PLAN} placeholder in the Objective section.
//
// Assembly order (FIX-181):
//
//	Identity → ToolUsage → ResultMode → Capabilities → Rules → Objective → StaticEnv → Custom → DynamicEnv
func buildSystemPromptWithMode(rules string, mode config.ResultMode, agentName, agentDescription, agentPrinciples, userName, channel, taskDesc, taskPlanText string, toolUsageText ...string) string {
	sh := shellName()

	// Gather static environment context
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

	// Use i18n defaults for empty identity fields
	if agentName == "" {
		agentName = "co-shell"
	}
	if agentDescription == "" {
		agentDescription = i18n.T(i18n.KeyDefaultAgentDescription)
	}
	if agentPrinciples == "" {
		agentPrinciples = i18n.T(i18n.KeyDefaultAgentPrinciples)
	}

	// Part 1: Identity
	identityText := "IDENTITY\n\n" + i18n.TF(i18n.KeySystemPromptIdentity, agentName, agentDescription, agentPrinciples)

	// Part 2: Tool Usage Guide
	// Use the provided toolUsageText if given, otherwise default to OpenAI-style tool usage.
	toolUsageSection := "TOOL USE\n\n" + i18n.T(i18n.KeySystemPromptToolUsage)
	if len(toolUsageText) > 0 && toolUsageText[0] != "" {
		toolUsageSection = toolUsageText[0]
	}

	// Part 3: Result Mode
	resultModeText := "RESULT MODE\n\n" + i18n.TF(i18n.KeySystemPromptResultMode, resultModeInstruction(mode))

	// Part 4: Capabilities — try external file first, fall back to i18n
	capabilities := loadExternalFile(cwd, "CAPABILITIES.md")
	if capabilities == "" {
		capabilities = strings.ReplaceAll("CAPABILITIES\n\n"+i18n.T(i18n.KeySystemPromptCapabilities), "{CWD}", cwd)
	}

	// Part 5: Rules — try external file first, fall back to i18n
	rulesText := loadExternalFile(cwd, "RULES.md")
	if rulesText == "" {
		rulesText = strings.ReplaceAll("RULES\n\n"+i18n.T(i18n.KeySystemPromptRules), "{CWD}", cwd)
	}
	// Inject custom rules into the {CUSTOM_RULES} placeholder
	if rules != "" {
		rulesText = strings.ReplaceAll(rulesText, "{CUSTOM_RULES}", rules)
	} else {
		rulesText = strings.ReplaceAll(rulesText, "{CUSTOM_RULES}", "")
	}

	// Part 6: Objective (task execution methodology)
	objectiveText := "OBJECTIVE\n\n" + i18n.T(i18n.KeySystemPromptObjective)
	if taskDesc != "" {
		objectiveText = strings.ReplaceAll(objectiveText, "{TASK}", taskDesc)
	}
	// Replace task tracking placeholder: if there is an active task plan with unfinished steps,
	// taskPlanText will contain the formatted plan; otherwise it's empty.
	objectiveText = strings.ReplaceAll(objectiveText, "{TASK_TRACKING}", taskPlanText)

	// Part 7: Static Environment (no dynamic fields like time)
	envText := "SYSTEM INFORMATION\n\n" + i18n.T(i18n.KeySystemPromptEnvironment)
	envText = strings.ReplaceAll(envText, "{OS}", runtime.GOOS)
	envText = strings.ReplaceAll(envText, "{ARCH}", runtime.GOARCH)
	envText = strings.ReplaceAll(envText, "{COMMAND}", execName)
	envText = strings.ReplaceAll(envText, "{SHELL}", sh)
	envText = strings.ReplaceAll(envText, "{HOME}", homeDir)
	envText = strings.ReplaceAll(envText, "{CWD}", cwd)
	envText = strings.ReplaceAll(envText, "{WORKSPACE}", cwd)

	// Build channel info: "user-name @ channel-type"
	// Default userName to anonymous user string if not set
	displayUser := userName
	if displayUser == "" {
		displayUser = i18n.T(i18n.KeyAnonymousUser)
	}
	// Default channel to "co-shell" if not set
	displayChannel := channel
	if displayChannel == "" {
		displayChannel = "co-shell"
	}
	channelInfo := displayUser + " @ " + displayChannel

	// Replace dynamic placeholders before assembling into prompt,
	// so the SYSTEM INFORMATION section is complete in one pass.
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	envText = strings.ReplaceAll(envText, "{CURRENT_TIME}", now)
	envText = strings.ReplaceAll(envText, "{CWD}", cwd)
	envText = strings.ReplaceAll(envText, "{CURRENT_FILES}", strings.TrimRight(listFilesForPrompt(cwd, true, 100), "\n"))
	envText = strings.ReplaceAll(envText, "{CHANNEL}", channelInfo)

	// Separator between major sections (sections with English uppercase titles)
	sep := "\n\n====\n\n"

	// Assemble Parts 1-7 with separator
	prompt := identityText + sep +
		toolUsageSection + sep +
		resultModeText + sep +
		capabilities + sep +
		rulesText + sep +
		objectiveText + sep +
		envText

	return prompt
}

// resultModeInstruction returns the instruction text for the given result mode.
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
