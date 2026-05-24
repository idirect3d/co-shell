// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-05-22
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
	"strings"
)

// formatToolError formats a structured error message to help LLM understand
// and fix tool call issues. It provides clear guidance on what went wrong
// and how to correct it.
func formatToolError(toolName string, err error) string {
	errMsg := err.Error()

	// Check for missing required parameter errors
	// Common patterns: "missing required parameter", "required field not present"
	if strings.Contains(errMsg, "missing") || strings.Contains(errMsg, "required") || strings.Contains(errMsg, "omit") {
		return fmt.Sprintf(`Tool call "%s" failed due to missing required parameters.

ERROR DETAILS: %s

CORRECTION INSTRUCTIONS:
1. Review the tool definition carefully - it shows all REQUIRED parameters
2. The tool definition for "%s" specifies these required parameters: %s
3. You MUST include ALL required parameters in your next call
4. Do NOT omit any required parameters - the call will fail again

IMPORTANT REMINDER:
- Every tool call must include ALL parameters listed in the "required" array
- Missing any required parameter will cause the same error
- Check the tool definition provided in the system prompt for the complete parameter list`,
			toolName,
			errMsg,
			toolName,
			getRequiredParamsDescription(toolName),
		)
	}

	// Check for argument parsing errors
	if strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "invalid") {
		return fmt.Sprintf(`Tool call "%s" failed due to invalid arguments.

ERROR DETAILS: %s

CORRECTION INSTRUCTIONS:
1. Check that your arguments are valid JSON format
2. Ensure all string values are properly quoted
3. Verify that parameter names match exactly (case-sensitive)
4. Review the tool definition for correct parameter types

Please fix the argument format and retry.`,
			toolName,
			errMsg,
		)
	}

	// Default: generic error with tool name context
	return fmt.Sprintf(`Tool call "%s" failed.

ERROR DETAILS: %s

Please review the error and correct your approach. Check the tool definition for required parameters and correct usage.`,
		toolName,
		errMsg,
	)
}

// getRequiredParamsDescription returns a description of required parameters for common tools.
// This helps the LLM understand what parameters are expected.
func getRequiredParamsDescription(toolName string) string {
	switch toolName {
	case "write_to_file":
		return "'path' (string: file path) AND 'content' (string: complete file content) - BOTH are required"
	case "read_file":
		return "'path' (string: file path) - required; 'start_line' and 'end_line' are optional"
	case "execute_command":
		return "'command' (string: command to execute) - required; 'timeout_seconds' is optional"
	case "search_files":
		return "'path' (string: directory path) AND 'regex' (string: search pattern) - BOTH are required; 'file_pattern' is optional"
	case "replace_in_file":
		return "'path' (string: file path) AND 'replacements' (array: SEARCH/REPLACE blocks) - BOTH are required"
	case "list_code_definition_names":
		return "'path' (string: directory path) - required"
	case "add_images":
		return "'paths' (string: comma-separated image paths) - required"
	case "remove_images":
		return "'paths' (string: comma-separated image paths) - required"
	case "ask_followup_question":
		return "'question' (string: question text) - required; 'options' is optional"
	default:
		return "all parameters listed in the tool definition's 'required' array"
	}
}
