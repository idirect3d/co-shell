// Author: L.Shuang
// Created: 2026-05-03
// Last Modified: 2026-05-03
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

package main

import (
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// buildUsage builds the complete usage/help message by assembling sections.
func buildUsage(version string) string {
	var sb strings.Builder

	sb.WriteString(i18n.TF(i18n.KeyCLIHelpTitle, version))
	sb.WriteString("\n\n")
	sb.WriteString(buildUsageBasic())
	sb.WriteString(buildUsageLLMBehavior())
	sb.WriteString(buildUsageOutputControl())
	sb.WriteString(buildUsageFeatureSwitches())
	sb.WriteString(buildUsageTimeout())
	sb.WriteString(buildUsageInit())
	sb.WriteString(buildUsageVersionHelp())
	sb.WriteString(buildUsageExamples())

	return sb.String()
}

// buildUsageBasic returns the basic options section of the help message.
func buildUsageBasic() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpUsage) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpUsageREPL) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpUsageCmd) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpOptions) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpName) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpWorkspace) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpConfig) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpModel) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEndpoint) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpAPIKey) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpLang) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpLog) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpMaxIter) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpImage) + "\n")
	return sb.String()
}

// buildUsageLLMBehavior returns the LLM behavior options section.
func buildUsageLLMBehavior() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpTemperature) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpMaxTokens) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowThinking) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowCommand) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpConfirmCommand) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpResultMode) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpDescription) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpPrinciples) + "\n")
	return sb.String()
}

// buildUsageOutputControl returns the output control options section.
func buildUsageOutputControl() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowLlmThinking) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowLlmContent) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowTool) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowToolInput) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowToolOutput) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpShowCommandOutput) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEmojiEnabled) + "\n")
	return sb.String()
}

// buildUsageFeatureSwitches returns the feature switch options section.
func buildUsageFeatureSwitches() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpMemoryEnabled) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpMemoryDisabled) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpPlanEnabled) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpPlanDisabled) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpSubAgentEnabled) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpSubAgentDisabled) + "\n")
	return sb.String()
}

// buildUsageTimeout returns the timeout options section.
func buildUsageTimeout() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpToolTimeout) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpCmdTimeout) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpLLMTimeout) + "\n")
	return sb.String()
}

// buildUsageInit returns the init options section.
func buildUsageInit() string {
	var sb strings.Builder
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpInitCapabilities) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpInitRules) + "\n")
	return sb.String()
}

// buildUsageVersionHelp returns the version and help options section.
func buildUsageVersionHelp() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeyCLIHelpVersion) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpHelp) + "\n")
	return sb.String()
}

// buildUsageExamples returns the examples section of the help message.
func buildUsageExamples() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(i18n.T(i18n.KeyCLIHelpExamples) + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx1) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx2) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx3) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx4) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx5) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx6) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx7) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx8) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx9) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx10) + "\n")
	sb.WriteString("  " + i18n.T(i18n.KeyCLIHelpEx11) + "\n")
	return sb.String()
}
