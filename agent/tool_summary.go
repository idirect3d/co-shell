// Author: L.Shuang
// Created: 2026-08-02
// Last Modified: 2026-08-02
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
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// maxSummaryParamLen is the maximum length of a single parameter value shown
// in a tool call summary. Longer values are truncated with a suffix marker.
const maxSummaryParamLen = 80

// buildToolSummary constructs a human-readable summary of a tool call:
// friendly tool name + intent + key parameters (long content truncated).
// The phrasing is personalized per tool via i18n templates (FEATURE-310).
// It is used in the confirmation prompt and the showTool display path.
func buildToolSummary(toolName string, args map[string]interface{}) string {
	intent := argString(args, "intent")

	// Tools with a dedicated i18n template are rendered with personalized
	// phrasing so the user can grasp the impact of the call at a glance.
	switch toolName {
	case "execute_command":
		return i18n.TF(i18n.KeyToolCallSummaryExecCmd, argString(args, "command"), intent)
	case "read_file":
		return i18n.TF(i18n.KeyToolCallSummaryReadFile,
			argString(args, "path"),
			argNum(args, "start_line"),
			argNum(args, "end_line"),
			intent)
	case "write_to_file":
		return i18n.TF(i18n.KeyToolCallSummaryWriteFile,
			argString(args, "path"),
			argString(args, "mode"),
			intent)
	case "replace_in_file":
		n := argSliceLen(args, "replacements")
		return i18n.TF(i18n.KeyToolCallSummaryReplaceInFile,
			argString(args, "path"),
			fmt.Sprintf("%d", n),
			intent)
	case "search_files":
		return i18n.TF(i18n.KeyToolCallSummarySearchFiles,
			argString(args, "path"),
			truncate(argString(args, "regex")),
			intent)
	case "list_files":
		rec := argNum(args, "recursive")
		if rec == "" {
			rec = "0"
		}
		return i18n.TF(i18n.KeyToolCallSummaryListFiles,
			argString(args, "path"),
			rec,
			intent)
	case "list_code_definition_names":
		return i18n.TF(i18n.KeyToolCallSummaryListDefs,
			argString(args, "path"),
			intent)
	case "shell_send":
		return i18n.TF(i18n.KeyToolCallSummaryShellSend,
			truncate(argString(args, "command")),
			intent)
	case "visual_analysis":
		return i18n.TF(i18n.KeyToolCallSummaryVisualAnalysis,
			fmt.Sprintf("%d", argSliceLen(args, "paths")),
			intent)
	case "excel_open", "word_open":
		key := i18n.KeyToolCallSummaryExcelOpen
		if toolName == "word_open" {
			key = i18n.KeyToolCallSummaryWordOpen
		}
		return i18n.TF(key,
			argString(args, "path"),
			argString(args, "mode"),
			intent)
	case "update_settings":
		return i18n.TF(i18n.KeyToolCallSummaryUpdateSettings,
			fmt.Sprintf("%d", argSliceLen(args, "settings")),
			intent)
	case "ask_followup_question":
		return i18n.TF(i18n.KeyToolCallSummaryAskQuestion,
			truncate(argString(args, "question")))
	case "launch_sub_agent":
		return i18n.TF(i18n.KeyToolCallSummaryLaunchSubAgent,
			argString(args, "sub_agent_name"),
			intent)
	}

	// Generic fallback for all other tools.
	return genericToolSummary(toolName, intent)
}

// genericToolSummary renders the generic fallback summary: tool name + intent.
func genericToolSummary(toolName, intent string) string {
	if intent == "" {
		return toolName
	}
	return i18n.TF(i18n.KeyToolCallSummaryGeneric, toolName, intent)
}

// buildToolOutcome constructs a concise "action receipt" shown to the user
// AFTER a tool executes successfully (FIX-316). It summarizes what happened
// with the key result data (actual line counts / match counts / replace
// counts), as opposed to buildToolSummary which describes the call intent
// BEFORE execution. Rendered through the EventToolCall stream channel and
// gated by the showTool switch — distinct from showToolOutput (full detail).
func buildToolOutcome(toolName string, args map[string]interface{}, resultText string) string {
	switch toolName {
	case "read_file":
		return i18n.TF(i18n.KeyToolOutcomeReadFile,
			argString(args, "path"),
			argNum(args, "start_line"),
			argNum(args, "end_line"))
	case "write_to_file":
		lines := countTextLines(resultText)
		return i18n.TF(i18n.KeyToolOutcomeWriteFile,
			argString(args, "path"),
			fmt.Sprintf("%d", lines))
	case "replace_in_file":
		n := argSliceLen(args, "replacements")
		return i18n.TF(i18n.KeyToolOutcomeReplaceFile,
			argString(args, "path"),
			fmt.Sprintf("%d", n))
	case "search_files":
		n := countMatchesInResult(resultText)
		return i18n.TF(i18n.KeyToolOutcomeSearchFiles,
			argString(args, "path"),
			fmt.Sprintf("%d", n))
	case "execute_command":
		// Show the actual command and its intent (execute_command requires intent).
		return i18n.TF(i18n.KeyToolOutcomeExecCmd,
			truncate(argString(args, "command")),
			truncate(argString(args, "intent")))
	}
	// Generic success receipt for any other tool.
	return i18n.TF(i18n.KeyToolOutcomeGeneric, toolName)
}

// countTextLines returns the number of lines in a text result. It is used to
// report actual line count of a file write. A single-line result (or empty)
// counts as 1 line.
func countTextLines(s string) int {
	if s == "" {
		return 1
	}
	// Result typically starts with a header line (file info) — count non-empty
	// body lines after the first header for a more meaningful "N lines" figure.
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 1
	}
	lines := strings.Split(trimmed, "\n")
	return len(lines)
}

// countMatchesInResult extracts the number of matches from a search_files
// result text. The result usually contains a line like
// "Found N matches for pattern ... in ...". Falls back to counting lines that
// contain ".go:"-style file:line references, or 0.
func countMatchesInResult(s string) int {
	// Pattern: "found N matches" / "找到 N 处匹配" (case-insensitive).
	lower := strings.ToLower(s)
	idx := strings.Index(lower, "found ")
	if idx >= 0 {
		rest := lower[idx+len("found "):]
		var n int
		if _, err := fmt.Sscanf(rest, "%d", &n); err == nil {
			return n
		}
	}
	// Fallback: count lines containing "found N" (e.g. "found 5 matches").
	lineCount := 0
	for _, line := range trimmedLines(s) {
		lower := strings.ToLower(line)
		idx := strings.Index(lower, "found ")
		if idx >= 0 {
			var n int
			if _, err := fmt.Sscanf(lower[idx+len("found "):], "%d", &n); err == nil {
				lineCount += n
			}
		}
	}
	return lineCount
}

// trimmedLines splits s into lines and trims each line.
func trimmedLines(s string) []string {
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// argString returns the string value of a named argument, or "" if absent.
func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		if f, ok := v.(float64); ok {
			return fmt.Sprintf("%.0f", f)
		}
		if b, ok := v.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
	}
	return ""
}

// argNum returns the numeric value of a named argument as string, or "" if absent.
func argNum(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		switch t := v.(type) {
		case float64:
			if t == float64(int(t)) {
				return fmt.Sprintf("%d", int(t))
			}
			return fmt.Sprintf("%.0f", t)
		case int:
			return fmt.Sprintf("%d", t)
		case string:
			return t
		}
	}
	return ""
}

// argSliceLen returns the number of elements in a named array argument.
func argSliceLen(args map[string]interface{}, key string) int {
	if v, ok := args[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			return len(arr)
		}
	}
	return 0
}

// truncate shortens a string to maxSummaryParamLen with a truncation suffix.
func truncate(s string) string {
	if len(s) <= maxSummaryParamLen {
		return s
	}
	return s[:maxSummaryParamLen] + i18n.T(i18n.KeyToolCallSummaryTruncated)
}
