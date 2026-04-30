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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// shellCmd returns the appropriate shell command and argument for the current platform.
func shellCmd() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/c"
	}
	return "bash", "-c"
}

// shellName returns the human-readable shell name for the current platform.
func shellName() string {
	if runtime.GOOS == "windows" {
		return "cmd/powershell"
	}
	return "bash/zsh"
}

// buildSystemPrompt constructs the system prompt with rules and context.
func buildSystemPrompt(rules string) string {
	return buildSystemPromptWithMode(rules, config.ResultModeMinimal, "", "", "")
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
// If workspacePath is non-empty, it tries to load capabilities.md and rules.md from the workspace
// root to override the built-in i18n defaults.
func buildSystemPromptWithMode(rules string, mode config.ResultMode, agentName, agentDescription, agentPrinciples string) string {
	sh := shellName()

	// Gather environment context
	cwd, _ := os.Getwd()
	hostname, _ := os.Hostname()
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	// Build prompt using i18n translations
	title := i18n.TF(i18n.KeySystemPromptTitle,
		runtime.GOOS, runtime.GOARCH, sh, now, cwd, hostname, username)

	// Try to load external CAPABILITIES.md and RULES.md from workspace
	// If not found, fall back to built-in i18n values
	capabilities := loadExternalFile(cwd, "CAPABILITIES.md")
	if capabilities == "" {
		capabilities = i18n.TF(i18n.KeySystemPromptCapabilities, sh)
	}

	rulesText := loadExternalFile(cwd, "RULES.md")
	if rulesText == "" {
		rulesText = i18n.T(i18n.KeySystemPromptRules)
	}

	resultModeText := i18n.TF(i18n.KeySystemPromptResultMode, resultModeInstruction(mode))

	prompt := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\nAvailable tools will be provided to you as function definitions.",
		title, capabilities, rulesText, resultModeText)

	// Add agent identity if configured
	if agentName != "" || agentDescription != "" || agentPrinciples != "" {
		identityText := i18n.TF(i18n.KeySystemPromptIdentity, agentName, agentDescription, agentPrinciples)
		prompt = fmt.Sprintf("%s\n\n%s", identityText, prompt)
	}

	if rules != "" {
		prompt += fmt.Sprintf("\n\n%s:\n%s", i18n.T(i18n.KeyCustom), rules)
	}

	return prompt
}

// resultModeInstruction returns the instruction text for the given result mode.
func resultModeInstruction(mode config.ResultMode) string {
	switch mode {
	case config.ResultModeMinimal:
		return `When you execute a system command and receive its output, do NOT repeat the command output in your response. Instead, simply indicate whether the command succeeded or failed. If it succeeded, respond with a brief success confirmation (e.g., "✅ 命令执行成功" or "✅ Command executed successfully"). If it failed, respond with a brief error message. Do not add any additional explanation, analysis, or commentary.`

	case config.ResultModeExplain:
