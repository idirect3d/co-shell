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

const optionPad = 34 // width for option part (e.g. "  -c, --config <path>")

// formatHelpLine takes a full i18n help line and re-formats it with consistent alignment.
// The input format is: "  <option>  <description>"
// The output format is: "  <option>  <description>" with option padded to optionPad width.
func formatHelpLine(text string) string {
	// Trim leading whitespace to get the raw option part
	trimmed := strings.TrimLeft(text, " ")
	if trimmed == "" {
		return text
	}

	// Find the option end: look for the last char before description starts.
	// Options end with either:
	// - a closing bracket '>' (for params) or
	// - a pattern like on|off, or
	// - the last word before description
	// We'll find the option by checking: after the option part, there should be 2+ spaces then description.

	// Strategy: split the trimmed string by double (or more) spaces.
	// Option part is before the first double-space sequence.
	option := strings.TrimSpace(trimmed)
	desc := ""

	// Find the first occurrence of 2+ spaces, which separates option from description
	for i := 0; i < len(trimmed); i++ {
		if i+1 < len(trimmed) && trimmed[i] == ' ' && trimmed[i+1] == ' ' {
			option = strings.TrimSpace(trimmed[:i])
			desc = strings.TrimSpace(trimmed[i:])
			break
		}
	}

	if desc == "" {
		// No description found, just return the original with consistent padding
		return "  " + option + "\n"
	}

	// Format with consistent padding
	line := "  " + option
	if len(line) < optionPad {
		line += strings.Repeat(" ", optionPad-len(line))
	} else {
		line += " "
	}
	line += desc + "\n"
	return line
}

// buildUsage builds the complete usage/help message by assembling sections.
func buildUsage(version string) string {
	var sb strings.Builder

	sb.WriteString(i18n.TF(i18n.KeyCLIHelpTitle, version))
	sb.WriteString("\n\n")
	sb.WriteString(formatHelpLine("  " + i18n.T(i18n.KeyCLIHelpUsage)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpUsageREPL)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpUsageCmd)))
	sb.WriteString(formatHelpLine("  " + i18n.T(i18n.KeyCLIHelpOptions)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpName)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpWorkspace)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpConfig)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpModel)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpEndpoint)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpAPIKey)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpLang)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpLog)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpMaxIter)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpImage)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpInputMode)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpTemperature)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpMaxTokens)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpTopP)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpTopK)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpRepetitionPenalty)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowThinking)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowCommand)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpConfirmTool)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpResultMode)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpDescription)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowLlmThinking)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowLlmContent)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowTool)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowToolInput)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowToolOutput)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowCommandOutput)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpEmojiEnabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpShowLogo)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpContextStart)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpMemoryEnabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpMemoryDisabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpPlanEnabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpPlanDisabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpSubAgentEnabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpSubAgentDisabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpToolCallEnabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpToolCallDisabled)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpToolCallMode)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpLoopDetect)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpDedup)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpToolTimeout)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpCmdTimeout)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpLLMTimeout)))

	sb.WriteString("\n")
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpVersion)))
	sb.WriteString(formatHelpLine(i18n.T(i18n.KeyCLIHelpHelp)))

	sb.WriteString("\n")
	sb.WriteString(formatHelpLine("  " + i18n.T(i18n.KeyCLIHelpExamples)))
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
