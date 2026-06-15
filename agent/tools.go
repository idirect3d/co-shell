// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// buildTools constructs the list of available tools for the LLM.
// In OpenAI mode, tools are returned as a JSON array for the "tools" parameter.
// In XML mode, tools are described in the system prompt, so an empty list is returned
// (the tools are still registered internally for execution).
func (a *Agent) buildTools() []llm.Tool {
	// If tool calling is disabled, return empty tools list
	if !a.toolCallEnabled {
		return []llm.Tool{}
	}

	// In XML mode, tools are described in the system prompt, not sent as API parameter.
	// Return empty list so the LLM API doesn't receive the "tools" parameter.
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Current()
		if mode != nil && !mode.SendTools {
			return []llm.Tool{}
		}
	}

	return a.buildToolsInternal()
}

// buildToolsInternal returns the list of tools, filtered by current tool modes.
// This is used for generating the XML tool usage prompt in the system prompt.
// Tools marked as "disabled" in a.toolModes are excluded.
func (a *Agent) buildToolsInternal() []llm.Tool {
	sh := shellName()
	var tools []llm.Tool

	// Conditional tool shielding based on shell-session-enabled:
	// When shellEnabled is true, use shell tools instead of execute_command
	// (more human-like interaction).
	// When shellEnabled is false, shell tools are hidden and execute_command
	// is available.
	// Each tool also respects per-tool mode from .set confirm-tool:
	// a tool set to "disabled" will be filtered out later, regardless of
	// the shell-enabled state.
	if !a.shellEnabled {
		// Shell session disabled: use execute_command
		tools = append(tools, llm.Tool{
			Name:        "execute_command",
			Description: fmt.Sprintf("Execute a system command (%s) and return its output. Use this to run shell commands, scripts, or any CLI tools. You can optionally specify a timeout_seconds to limit execution time based on the task complexity.", sh),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional timeout in seconds. Set this based on your estimate of how long the command will take. The actual timeout used will be the maximum of this value and the user-configured minimum timeout. 0 or omitted means use only the user-configured timeout.",
					},
				},
				"required": []string{"command"},
			},
			Callback: a.executeSystemCommand,
		})
	}

	if a.shellEnabled {
		// Shell session enabled: use shell tools (more human-like terminal interaction).
		// shell_start/shell_stop are managed automatically by the system.
		tools = append(tools, llm.Tool{
			Name:        "shell_send",
			Description: "Send content (command, Python statement, control character, etc.) to the persistent shell session and observe the terminal screen output. The content runs in the same shell environment as previous shell_send calls, preserving all state (current directory, environment variables, Python REPL state, etc.).\n\nThe command is sent VERBATIM to the shell's stdin. You MUST explicitly include any required newline (\\n) — it is NOT added automatically.\n\nThe return value is the full text content of the virtual terminal window (rows x cols character grid). This is like looking at a real terminal screen — you see the complete window content as a human would. Review the output carefully to understand command results, error messages, and prompt states.\n\nIMPORTANT: Send one logical unit at a time. Observe the result before sending the next unit. When you see a shell prompt (like '$ ' or '# ') at the end of the output, it means the command has completed and the shell is ready for the next command.\n\nControl characters (send these as literal byte values in the command string):\n  \\n  = Enter (execute/submit input)\n  \\x03 = Ctrl+C (SIGINT)\n  \\x04 = Ctrl+D (EOF, exit REPL)\n  \\x0c = Ctrl+L (clear screen)\n  \\x09 = Tab\n  \\x1b = ESC\n  \\x1b[A = Up arrow\n  \\x1b[B = Down arrow\n  \\x1b[D = Left arrow\n  \\x1b[C = Right arrow\n\nThe wait_ms parameter (optional, default 200ms) controls the idle timeout. For long-running processes, set a higher value or call shell_window_content afterward.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The content to send to the shell session — a single shell command, Python statement, or input line",
					},
					"wait_ms": map[string]interface{}{
						"type":        "number",
						"description": "Optional idle timeout in milliseconds (default: 200). Wait this long for new output after the last received output before returning the result. Increase for long-running processes.",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional total timeout in seconds. Set this based on your estimate of how long the entire operation will take. 0 or omitted means no total timeout (use the default shell-session-timeout).",
					},
				},
				"required": []string{"command"},
			},
			Callback: a.shellSendTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_window_content",
			Description: "Get the current virtual terminal window content as a text snapshot. This shows what is currently displayed on the terminal screen. Use this to check the state of a long-running process, review previous command output, or inspect the current terminal state without sending a new command. Returns the full window content as rows x cols text.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
			Callback: a.shellWindowContentTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_get_output",
			Description: "Retrieve output from the persistent shell session since the last time shell_send or shell_get_output was called (auto-increment mode), or from a specific position.\n\nAuto-increment mode (no last_from/count): returns only the new content that has been produced since the last shell_send or shell_get_output call. This is useful for checking progress of a long-running command or REPL session.\n\nLegacy mode (with last_from/count): returns terminal scrollback history — like scrolling up in a terminal window. Use this to review what happened before the last command.\n\nParameters: wait_ms (optional, default 200ms — wait this long for new output before returning), last_from (optional, 1-based from end where 1=most recent line), count (optional, number of lines to return), timeout_seconds (optional, total timeout in seconds).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"wait_ms": map[string]interface{}{
						"type":        "number",
						"description": "Optional observation wait time in milliseconds (default: 200). Wait this long for new output before returning. Increase for checking progress of a running command.",
					},
					"last_from": map[string]interface{}{
						"type":        "number",
						"description": "Optional: Starting position from the end (1-based, 1=most recent line). If not provided, uses auto-increment mode (returns only new content since last call).",
					},
					"count": map[string]interface{}{
						"type":        "number",
						"description": "Optional: Number of lines to return. If not provided with last_from, uses auto-increment mode.",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional total timeout in seconds. Set this to prevent infinite waiting. 0 or omitted means no total timeout (use the default shell-session-timeout).",
					},
				},
				"required": []string{},
			},
			Callback: a.shellGetOutputTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_reset",
			Description: "Reset the persistent shell session to a clean state. This closes the current session and starts a new one with a fresh terminal. Use this when the shell is in an unexpected state (e.g., inside a REPL with errors, or stuck in a process). The shell session is normally managed automatically — use this only when a manual reset is needed.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
			Callback: a.shellResetTool,
		})
	}

	// File operation tools (always available)
	tools = append(tools, llm.Tool{
		Name:        "read_file",
		Description: "Read the contents of a file at the specified path. Use this to examine the contents of an existing file. Returns the file content with line numbers. Supports start_line and end_line to read specific sections of large files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the file to read (absolute or relative to current working directory)",
				},
				"start_line": map[string]interface{}{
					"type":        "number",
					"description": "The 1-based line number to start reading from (inclusive). Default: 1",
				},
				"end_line": map[string]interface{}{
					"type":        "number",
					"description": "The 1-based line number to stop reading at (inclusive). Default: start_line + 1000",
				},
			},
			"required": []string{"path"},
		},
		Callback: a.readFileTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "search_files",
		Description: "Search for a regex pattern across files in a specified directory. Returns matching lines with surrounding context. Use this to find specific code patterns, function definitions, or text across multiple files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory path to search in (absolute or relative to current working directory)",
				},
				"regex": map[string]interface{}{
					"type":        "string",
					"description": "The regular expression pattern to search for",
				},
				"file_pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern to filter files (e.g., '*.go' for Go files). If not provided, searches all files.",
				},
			},
			"required": []string{"path", "regex"},
		},
		Callback: a.searchFilesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "list_files",
		Description: "List files and directories within the specified directory. recursive controls recursion depth: 0=top-level only (default), 1=one level deep, 2=two levels, etc. Use this to explore directory structures and find files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to list contents for (absolute or relative to current working directory)",
				},
				"recursive": map[string]interface{}{
					"type":        "number",
					"description": "Recursion depth: 0=top-level only (default), 1=one level deep, 2=two levels, etc.",
				},
			},
			"required": []string{"path"},
		},
		Callback: a.listFilesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "list_code_definition_names",
		Description: "List definition names (functions, types, methods, etc.) in source code files at the top level of a specified directory. Use this to quickly understand the structure and API of a codebase.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory path to list definitions for (absolute or relative to current working directory)",
				},
			},
			"required": []string{"path"},
		},
		Callback: a.listCodeDefinitionNamesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "replace_in_file",
		Description: "Replace sections of content in an existing file using 'search'/'replace' blocks. Accepts a 'replacements' array where each element is an object with 'search' (the exact content to find), 'replace' (the new content), and optional 'start_line' (the 1-based line number in the original file for precise positioning). Supports multiple replacements in a single call. The 'search' content must match the file exactly (including whitespace and indentation). When 'start_line' is provided, the search is anchored to that line (adjusted for previous replacements' line changes). A backup is automatically created before writing. Returns detailed diff information showing which lines were changed. Use this to make targeted changes to specific parts of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to modify (absolute or relative to current working directory)",
				},
				"replacements": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"search": map[string]interface{}{
								"type":        "string",
								"description": "The exact content to find in the file (must match character-for-character including whitespace and indentation)",
							},
							"replace": map[string]interface{}{
								"type":        "string",
								"description": "The new content to replace the matched section with",
							},
							"start_line": map[string]interface{}{
								"type":        "number",
								"description": "Optional: the 1-based line number in the original file where this 'search' content is expected to start. Used for precise positioning and to avoid duplicate matches. The system automatically adjusts for line count changes from previous replacements.",
							},
						},
						"required": []string{"search", "replace"},
					},
					"description": `An array of replacement objects, each with 'search' and 'replace' string fields, and optional 'start_line' number. All replacements are performed sequentially in order.

Critical rules:
1. The 'search' field must match the file EXACTLY (character-for-character including whitespace, indentation, line endings, comments, docstrings, etc.). The system first attempts exact match, then falls back to whitespace-tolerant fuzzy matching (trailing whitespace ignored) if exact match fails.
2. Each replacement replaces only the FIRST match. For multiple matches, use multiple unique 'search' values.
3. Keep replacements concise: break large changes into smaller blocks. Include just enough context lines for uniqueness. Each line must be complete — never truncate.
4. Special operations:
   - To move code: Use two replacements (one to delete from original, one to insert at new location)
   - To delete code: Leave 'replace' empty
5. If source context came from read_file with line labels (e.g. "42 | const x = 1"), do NOT include the line label prefix in 'search'. Match only the raw file text.
6. The optional 'start_line' is 1-based and refers to the line number in the ORIGINAL file (before any replacements). The system automatically adjusts for line count changes from previous replacements. Use 'start_line' for precise positioning and to avoid duplicate matches.`,
				},
			},
			"required": []string{"path", "replacements"},
		},
		Callback: a.replaceInFileTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "write_to_file",
		Description: "Write content to a file at the specified path. If the file exists, it will be overwritten. If the file doesn't exist, it will be created. Any necessary directories will be created automatically. **CRITICAL: This tool REQUIRES BOTH 'path' AND 'content' parameters. The 'content' parameter is MANDATORY and must contain the complete file content. Omitting 'content' will cause an error.** IMPORTANT: When fixing errors in an existing file, prefer using replace_in_file instead of write_to_file. Using write_to_file to rewrite complex files often reintroduces the same issues. Use write_to_file primarily for creating new files or when a complete rewrite is truly necessary.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the file to write to",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "**REQUIRED/MANDATORY**: The full content to write to the file. This parameter MUST be provided in every call. The tool will fail if this parameter is omitted. Content should be the complete intended file content, not a partial update.",
				},
			},
			"required": []string{"path", "content"},
		},
		Callback: a.writeToFileTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "add_images",
		Description: "Add image file paths to the image cache. These images will be included in all subsequent conversations with the LLM for multimodal (vision) understanding. Multiple paths can be separated by commas. Use this when you need the LLM to see additional images.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated list of image file paths to add to the cache",
				},
			},
			"required": []string{"paths"},
		},
		Callback: a.addImagesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "remove_images",
		Description: "Remove image file paths from the image cache. Multiple paths can be separated by commas. Use this when you no longer need certain images in the conversation.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated list of image file paths to remove from the cache",
				},
			},
			"required": []string{"paths"},
		},
		Callback: a.removeImagesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "clear_images",
		Description: "Clear all cached image file paths. After calling this, no images will be included in subsequent conversations. Use this when you want to stop sending images to the LLM.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Callback: a.clearImagesTool,
	})

	// Add sub-agent tools only if sub-agent enabled
	if a.subAgentEnabled {
		subAgentTools := []llm.Tool{
			{
				Name:        "launch_sub_agent",
				Description: "Launch a sub-agent process to communicate with another co-shell agent for information sharing. The target agent's workspace is a sibling folder of the current agent's workspace, identified by sub_agent_name. The sub-agent shares the same terminal (stdin/stdout/stderr) with the parent agent. After the sub-agent completes, its results (including output files) are collected and reported. Use this to ask questions and get information from another agent — **this is equal information sharing, not task delegation**.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"sub_agent_name": map[string]interface{}{
							"type":        "string",
							"description": "Required: the name of the target co-shell agent. This name is used as the sibling workspace folder name.",
						},
						"instruction": map[string]interface{}{
							"type":        "string",
							"description": "The natural language instruction or system command for the sub-agent to execute.",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "number",
							"description": "Maximum time in seconds to wait for the sub-agent to complete. 0 means no timeout (default: 0).",
						},
					},
					"required": []string{"sub_agent_name", "instruction"},
				},
				Callback: a.launchSubAgentTool,
			},
		}
		tools = append(tools, subAgentTools...)
	}

	// Add schedule_task tool only if sub-agent enabled (it depends on sub-agent)
	if a.subAgentEnabled {
		tools = append(tools, llm.Tool{
			Name:        "schedule_task",
			Description: "Schedule a recurring task using a cron expression. The task will launch a sub-agent at the specified times. The cron expression uses 5 fields: minute hour day month weekday. Use * for any value, or a specific number. Example: '0 9 * * *' means every day at 9:00 AM. When the scheduled time arrives, a sub-agent will be launched with the given instruction. If a previous execution is still running, the next scheduled run will be skipped to avoid overlap.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "A human-readable name for this scheduled task (e.g., 'Daily Report', 'Health Check').",
					},
					"cron": map[string]interface{}{
						"type":        "string",
						"description": "5-field cron expression: minute hour day month weekday. Example: '0 9 * * *' for daily at 9:00 AM.",
					},
					"instruction": map[string]interface{}{
						"type":        "string",
						"description": "The instruction to pass to the sub-agent when the task is triggered.",
					},
				},
				"required": []string{"name", "cron", "instruction"},
			},
			Callback: a.scheduleTaskTool,
		})
	}

	// Add task plan tools only if plan enabled
	if a.planEnabled {
		planTools := []llm.Tool{
			{
				Name:        "create_task_plan",
				Description: "Create a new task plan (checklist) with a title, description, and a list of steps. Each step represents a sub-task to be completed. Use this to break down complex tasks into a structured checklist of manageable steps that can be tracked individually. The checklist should have moderate granularity: not too fine-grained (e.g., 'which character was typed'), nor too coarse (e.g., 'complete the entire project'). Each step should be a verifiable, independent unit with clear completion criteria. IMPORTANT: Only one task plan can exist at a time. If there are unfinished steps in the current plan, you cannot create a new one — you must first complete all steps or adjust the existing plan. If the current plan is fully completed, it will be automatically archived to memory before creating the new one.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "The title of the task plan",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "A brief description of the overall task plan",
						},
						"steps": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "An array of step descriptions, each representing a sub-task",
						},
					},
					"required": []string{"title", "steps"},
				},
				Callback: a.createTaskPlanTool,
			},
			{
				Name:        "update_task_step",
				Description: "Update the status of a specific step (checklist item) in the current task plan (checklist). Use this to mark steps as in_progress, completed, failed, or cancelled. Optionally add a note to provide context about the status change. After completing each step, immediately call this tool to update the checklist progress.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"step_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the step to update",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed", "failed", "cancelled"},
							"description": "The new status for the step",
						},
						"note": map[string]interface{}{
							"type":        "string",
							"description": "Optional note to add context about the status change",
						},
					},
					"required": []string{"step_id", "status"},
				},
				Callback: a.updateTaskStepTool,
			},
			{
				Name:        "insert_task_steps",
				Description: "Insert one or more new steps (checklist items) after a specified step in the current task plan (checklist). The new steps are added as pending. IMPORTANT: there must be no completed steps after the insertion point. Use after_step_id=0 to insert at the beginning. Use this when the plan needs additional steps based on new information — the checklist is dynamic and can be adjusted as needed.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"after_step_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the step after which to insert new steps. Use 0 to insert at the beginning. Example: if plan has steps 1,2,3 and after_step_id=1, new steps are inserted between step 1 and step 2.",
						},
						"steps": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "An array of step descriptions to insert after the specified step",
						},
					},
					"required": []string{"after_step_id", "steps"},
				},
				Callback: a.insertTaskStepsTool,
			},
			{
				Name:        "remove_task_steps",
				Description: "Remove one or more steps (checklist items) from the current task plan (checklist) by specifying a step ID range (from, to inclusive). Steps before the range are preserved, steps in the range are removed, and steps after the range are renumbered. IMPORTANT: completed steps cannot be removed. Use this to delete unnecessary or obsolete steps from a plan — the checklist is dynamic and can be adjusted as needed.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"from": map[string]interface{}{
							"type":        "number",
							"description": "The starting step ID of the range to remove (inclusive)",
						},
						"to": map[string]interface{}{
							"type":        "number",
							"description": "The ending step ID of the range to remove (inclusive)",
						},
					},
					"required": []string{"from", "to"},
				},
				Callback: a.removeTaskStepsTool,
			},
			{
				Name:        "view_task_plan",
				Description: "View the current task plan (checklist) with its progress summary, including all steps with their statuses and notes. Use this to check the current progress of the active task plan.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.viewTaskPlanTool,
			},
		}
		tools = append(tools, planTools...)
	}

	// Add memory tools only if persistent memory is enabled
	if a.memoryEnabled {
		memoryTools := []llm.Tool{
			{
				Name:        "get_memory_slice",
				Description: "Retrieve a slice of recent conversation history from persistent memory. Use this to recall what was discussed in previous conversations. Parameters: last_from (starting position from the end, 1=most recent), last_to (ending position from the end, 1=most recent). Example: last_from=5, last_to=1 returns the 5 most recent messages in chronological order.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"last_from": map[string]interface{}{
							"type":        "number",
							"description": "Starting position from the end (inclusive). 1 = most recent message. Must be >= last_to.",
						},
						"last_to": map[string]interface{}{
							"type":        "number",
							"description": "Ending position from the end (inclusive). 1 = most recent message.",
						},
					},
					"required": []string{"last_from", "last_to"},
				},
				Callback: a.getMemorySliceTool,
			},
			{
				Name:        "memory_search",
				Description: "Search persistent conversation memory for messages matching given keywords or criteria. Use this to find specific information from past conversations. Supports keyword search (AND logic), time-based filtering (since), and speaker name filtering.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"keywords": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Keywords to search for (AND logic: all keywords must match). Empty array returns all messages matching other filters.",
						},
						"since": map[string]interface{}{
							"type":        "string",
							"description": "Only return messages after this time (ISO 8601 format, e.g. '2026-04-01T00:00:00Z'). Empty string means no time filter.",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Filter by speaker name (case-insensitive). Empty string means no name filter.",
						},
					},
					"required": []string{},
				},
				Callback: a.memorySearchTool,
			},
			{
				Name:        "delete_memory",
				Description: "Delete a range of conversation history from persistent memory. Use this to remove outdated or incorrect information from memory. Parameters: last_from (starting position from the end, 1=most recent), last_to (ending position from the end, 1=most recent). Example: last_from=5, last_to=1 deletes the 5 most recent messages.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"last_from": map[string]interface{}{
							"type":        "number",
							"description": "Starting position from the end (inclusive). 1 = most recent message. Must be >= last_to.",
						},
						"last_to": map[string]interface{}{
							"type":        "number",
							"description": "Ending position from the end (inclusive). 1 = most recent message.",
						},
					},
					"required": []string{"last_from", "last_to"},
				},
				Callback: a.deleteMemoryTool,
			},
		}
		tools = append(tools, memoryTools...)
	}

	// Add update_settings tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "update_settings",
		Description: "Update co-shell system configuration parameters. Use this to modify settings such as model, temperature, display options, safety settings, etc. You must provide a reason for each change. The user will be prompted to confirm all changes before they are applied. IMPORTANT: Only use this when the user explicitly asks to change settings, or when a setting change is necessary to complete the user's request.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"settings": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param": map[string]interface{}{
								"type":        "string",
								"description": "The parameter name to change. See the .set command help for all available parameters (e.g., model, temperature, max-tokens, show-llm-thinking, confirm-tool, etc.)",
							},
							"value": map[string]interface{}{
								"type":        "string",
								"description": "The new value for the parameter",
							},
							"reason": map[string]interface{}{
								"type":        "string",
								"description": "Explain why this change is needed. This will be shown to the user for confirmation.",
							},
						},
						"required": []string{"param", "value", "reason"},
					},
					"description": "An array of setting changes to apply. Each change must include param, value, and reason.",
				},
			},
			"required": []string{"settings"},
		},
		Callback: a.updateSettingsTool,
	})

	// Add list_settings tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "list_settings",
		Description: "List all available co-shell system configuration parameters with their current values, valid ranges, and descriptions. Use this to discover what settings can be modified via the update_settings tool. This is useful when you need to understand the available configuration options before making changes.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Callback: a.listSettingsTool,
	})

	// Add ask_followup_question tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "ask_followup_question",
		Description: "Ask the user a question to gather additional information needed to complete the task. Use this when you encounter ambiguities, need clarification, or require more details to proceed effectively. It allows interactive problem-solving by enabling direct communication with the user. Use this tool judiciously to maintain a balance between gathering necessary information and avoiding excessive back-and-forth.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "The question to ask the user. This should be a clear, specific question that addresses the information you need.",
				},
				"options": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "An array of 2-5 options for the user to choose from. Each option should be a string describing a possible answer. You may not always need to provide options, but it may be helpful in many cases where it can save the user from having to type out a response manually.",
				},
			},
			"required": []string{"question"},
		},
		Callback: a.askFollowupQuestionTool,
	})

	// Add adjust_context_start tool (always available, but will check mode in callback)
	tools = append(tools, llm.Tool{
		Name:        "adjust_context_start",
		Description: "Adjust the context start pointer position. Allows the LLM to dynamically decide how much conversation history to keep based on context content, ignoring irrelevant early messages. Only available when context_start_mode is set to 'smart'. The target_index should be taken from the 'Message No' (or '消息序号' in zh) value in each message's <environment_details>.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target_index": map[string]interface{}{
					"type":        "number",
					"description": "The message index to set as the new context start. Messages before this index will be ignored when building context for LLM calls. Set to a value >= the current messagePointer.",
				},
			},
			"required": []string{"target_index"},
		},
		Callback: a.adjustContextStartTool,
	})

	// Add evaluate_expression tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "evaluate_expression",
		Description: "Evaluate a mathematical expression and return the exact result. Supports: basic arithmetic (+, -, *, /, %), exponentiation (^), parentheses for grouping, trigonometric functions (sin, cos, tan, asin, acos, atan), logarithms (log=base10, ln=natural), square root (sqrt), absolute value (abs), rounding (ceil, floor, round), and constants (pi, e). All trigonometric functions use radians. Use this for precise calculations instead of relying on Python or shell commands.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The mathematical expression to evaluate. Examples: '3 + 4 * 2', 'sin(pi/2)', '2 ^ 10', 'sqrt(144)', 'log(100) + ln(e)', '45 * (1 + 0.05) ^ 10', 'abs(-5) + round(3.7)'",
				},
			},
			"required": []string{"expression"},
		},
		Callback: a.evaluateExpressionTool,
	})

	// Add attempt_completion tool (always available)
	tools = append(tools, llm.Tool{
		Name: "attempt_completion",
		Description: `After each tool use, the user will respond with the result of that tool use, i.e. if it succeeded or failed, along with any reasons for failure. Once you've received the results of tool uses and can confirm that the task is complete, use this tool to present the result of your work to the user. Optionally you may provide a CLI command to showcase the result of your work. The user may respond with feedback if they are not satisfied with the result, which you can use to make improvements and try again.
IMPORTANT NOTE: This tool CANNOT be used until you've confirmed from the user that any previous tool uses were successful. Failure to do so will result in code corruption and system failure. Before using this tool, you must ask yourself in <thinking></thinking> tags if you've confirmed from the user that any previous tool uses were successful. If not, then DO NOT use this tool.
If you were using create_task_plan/update_task_step/... to manage the task progress, all unfinished tasks will be set to finish state.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type":        "string",
					"description": "The result of the tool use. This should be a clear, specific description of the result.",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Optional: A CLI command to execute to show a live demo of the result to the user. For example, use 'open index.html' to display a created html website, or 'open localhost:3000' to display a locally running development server. But DO NOT use commands like 'echo' or 'cat' that merely print text. This command should be valid for the current operating system. Ensure the command is properly formatted and does not contain any harmful instructions.",
				},
			},
			"required": []string{"result"},
		},
		Callback: a.attemptCompletionTool,
	})

	// Add browser tools if browser is enabled
	if a.browserEnabled {
		browserTools := []llm.Tool{
			{
				Name:        "browser_navigate",
				Description: "Navigate the Chrome browser to a specified URL. Use this to load a web page. After navigation, use browser_screenshot to view the page and browser_get_interactive_elements to see clickable elements.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to navigate to (must include protocol, e.g. https://example.com)",
						},
					},
					"required": []string{"url"},
				},
				Callback: a.browserNavigateTool,
			},
			{
				Name:        "browser_screenshot",
				Description: "Capture a screenshot of the current browser page. The screenshot will be automatically sent to the LLM for visual analysis (vision models only). Use this to observe the page content, layout, and elements. Parameters: quality (optional, 1-100, default 80), full_page (optional, boolean, default false). For full-page screenshots, set full_page=true.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"quality": map[string]interface{}{
							"type":        "number",
							"description": "Screenshot quality (1-100, default 80). Higher quality gives better visual detail for analysis.",
						},
						"full_page": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to capture the full page (true) or just the visible viewport (false, default). Full page captures all scrollable content.",
						},
					},
					"required": []string{},
				},
				Callback: a.browserScreenshotTool,
			},
			{
				Name:        "browser_click",
				Description: "Click at the specified coordinates (x, y) on the browser page. Use coordinates from browser_get_interactive_elements results (centerX, centerY). After clicking, use browser_screenshot to observe the result.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type":        "number",
							"description": "The x-coordinate to click at (in pixels from the left edge of the viewport)",
						},
						"y": map[string]interface{}{
							"type":        "number",
							"description": "The y-coordinate to click at (in pixels from the top edge of the viewport)",
						},
					},
					"required": []string{"x", "y"},
				},
				Callback: a.browserClickTool,
			},
			{
				Name:        "browser_type",
				Description: "Type text into the currently focused element on the browser page. Use this to fill in form fields, search boxes, or text areas. Optionally clear existing content first by setting clear=true.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The text to type into the focused element",
						},
						"clear": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to clear existing content before typing (default: false)",
						},
					},
					"required": []string{"text"},
				},
				Callback: a.browserTypeTool,
			},
			{
				Name:        "browser_evaluate",
				Description: "Execute JavaScript code in the browser page context and return the result. Use this for advanced operations like extracting data, modifying page content, or getting specific information from the page.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "The JavaScript expression or code to execute in the browser",
						},
					},
					"required": []string{"expression"},
				},
				Callback: a.browserEvaluateTool,
			},
			{
				Name:        "browser_get_rendered_html",
				Description: "Get the rendered DOM HTML of the current browser page after all JavaScript has executed. The HTML is serialized from the live DOM tree in Chrome's memory — it reflects the final rendered state including SPA framework output, dynamic content, and all JS modifications. This is NOT the raw source HTML; you get the page as the browser sees it after rendering, so there is NO need to separately download JS, JSON, or other resources. Use this to analyze the complete document structure, extract rendered data, or process interactive web applications.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.browserGetHTMLTool,
			},
			{
				Name:        "browser_scroll",
				Description: "Scroll the browser page by the specified delta. Use this to view content below or above the current viewport. Positive delta_y scrolls down, negative scrolls up. Default: scroll down 500 pixels.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"delta_x": map[string]interface{}{
							"type":        "number",
							"description": "Horizontal scroll delta in pixels (positive = right, negative = left, default: 0)",
						},
						"delta_y": map[string]interface{}{
							"type":        "number",
							"description": "Vertical scroll delta in pixels (positive = down, negative = up, default: 500)",
						},
					},
					"required": []string{},
				},
				Callback: a.browserScrollTool,
			},
			{
				Name:        "browser_get_interactive_elements",
				Description: "Get a list of interactive elements (buttons, links, inputs, etc.) on the current page with their positions, text, and attributes. Use this to find clickable elements and their coordinates for use with browser_click.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.browserGetInteractiveElementsTool,
			},
			{
				Name:        "browser_go_back",
				Description: "Navigate back to the previous page in browser history. After going back, use browser_screenshot to view the resulting page.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.browserGoBackTool,
			},
			{
				Name:        "browser_go_forward",
				Description: "Navigate forward to the next page in browser history. After going forward, use browser_screenshot to view the resulting page.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.browserGoForwardTool,
			},
			{
				Name:        "browser_close",
				Description: "Close the Chrome browser and clean up resources. Use this when you are done with browser automation tasks.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.browserCloseTool,
			},
		}
		tools = append(tools, browserTools...)
	}

	// Add MCP tools
	for _, mcpTool := range a.mcpMgr.GetAllTools() {
		tool := mcpTool // capture
		tools = append(tools, llm.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
			Callback: func(ctx context.Context, args map[string]interface{}) (string, error) {
				return a.mcpMgr.CallTool(ctx, tool.Name, args)
			},
		})
	}

	// Filter out disabled tools.
	// Each disabled entry in toolModes causes that tool to be skipped.
	// If "default" is disabled, all tools are skipped unless they have
	// their own explicit non-disabled mode set.
	disabledDefault := false
	disabledExplicit := make(map[string]bool)
	for k, v := range a.toolModes {
		if v == "disabled" {
			if k == "default" {
				disabledDefault = true
			} else {
				disabledExplicit[k] = true
			}
		}
	}
	if disabledDefault || len(disabledExplicit) > 0 {
		filtered := make([]llm.Tool, 0, len(tools))
		for _, tool := range tools {
			if disabledExplicit[tool.Name] {
				log.Debug("Tool %s is disabled, skipping registration", tool.Name)
				continue
			}
			if disabledDefault {
				// Default is disabled: only keep tools with explicit non-disabled mode
				ownMode, hasOwn := a.toolModes[tool.Name]
				if !hasOwn || ownMode == "disabled" {
					log.Debug("Tool %s is disabled (default=disabled), skipping registration", tool.Name)
					continue
				}
				// hasOwn && ownMode != "disabled" → keep
			}
			filtered = append(filtered, tool)
		}
		tools = filtered
	}

	return tools
}

// executeToolCall runs a single tool call and returns the result.
func (a *Agent) executeToolCall(ctx context.Context, tc llm.ToolCall) (string, error) {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return "", fmt.Errorf("cannot parse tool arguments: %w", err)
	}

	// Determine if confirmation is needed for this tool call.
	// Check per-tool mode: first check specific tool, then "default", then default to "confirm".
	mode := "confirm"
	if a.toolModes != nil {
		if v, ok := a.toolModes[tc.Name]; ok {
			mode = v
		} else if v, ok := a.toolModes["default"]; ok {
			mode = v
		}
	}
	needsConfirm := mode == "confirm"

	if needsConfirm {
		// Skip confirmation if:
		// - User chose "approve all" for this request, OR
		// - User disabled confirmation for this specific tool (G option), OR
		// - There are remaining auto-approve counts for this tool
		toolCount := a.toolApproveCounts[tc.Name]
		if !a.approveAll && !a.toolDisableConfirm[tc.Name] && toolCount <= 0 {
			// Build a display string from the tool arguments
			displayStr := tc.Name
			if cmd, ok := args["command"].(string); ok {
				displayStr = cmd
			}
			result, modifyInput := promptToolConfirmation(tc.Name, displayStr, a.defaultIO())
			switch result {
			case CmdConfirmCancel:
				return i18n.T(i18n.KeyCmdConfirmCancelled), fmt.Errorf("CANCEL_AGENT")
			case CmdConfirmApproveAll:
				a.approveAll = true
				// fall through to execute
			case CmdConfirmApproveG:
				a.toolDisableConfirm[tc.Name] = true
				a.defaultIO().Println(i18n.T(i18n.KeyCmdConfirmDisableTool))
				// fall through to execute
			case CmdConfirmApproveD:
				// Permanently disable this tool
				if a.toolModes == nil {
					a.toolModes = make(map[string]string)
				}
				a.toolModes[tc.Name] = "disabled"
				a.defaultIO().Println(i18n.T(i18n.KeyCmdConfirmDisableToolD))
				return "", fmt.Errorf("tool %q has been permanently disabled by user (D option)", tc.Name)
			case CmdConfirmApproveCount:
				// Parse the number of tool calls to auto-approve for this tool
				if n, err := strconv.Atoi(modifyInput); err == nil && n > 0 {
					a.toolApproveCounts[tc.Name] = n
					a.defaultIO().Println(fmt.Sprintf("%s%s %s", i18n.T(i18n.KeyCmdConfirmCountPrefix), modifyInput, tc.Name))
				}
				// fall through to execute
			case CmdConfirmModify:
				// Return the user's supplementary input as the tool result,
				// telling the LLM that the user modified the command and
				// the original tool call was not executed.
				return fmt.Sprintf("用户暂时取消了工具调用，并补充说明如下: %s\n\n请根据用户补充内容重新评估后再继续下一步操作。", modifyInput), nil
			}
			// CmdConfirmApprove: continue execution
		} else if toolCount > 0 {
			// Decrement per-tool approve count and auto-approve
			a.toolApproveCounts[tc.Name]--
			remaining := a.toolApproveCounts[tc.Name]
			a.defaultIO().Println(fmt.Sprintf("✅ 已自动批准 %s（剩余 %d 次）", tc.Name, remaining))
		}
	}

	// Find and execute the tool
	// Use buildToolsInternal() instead of buildTools() because we need the full
	// tool list including callbacks, regardless of the current tool call mode.
	// buildTools() may return an empty list in XML mode (where tools are described
	// in the system prompt rather than sent as API parameters), but we still need
	// the tool callbacks to execute the tool.
	tools := a.buildToolsInternal()
	for _, tool := range tools {
		if tool.Name == tc.Name {
			// Get LLM-suggested timeout from args (optional)
			llmSuggested := 0
			if t, ok := args["timeout_seconds"].(float64); ok {
				llmSuggested = int(t)
			}

			// Effective timeout = max(user-configured minimum, LLM-suggested)
			userMin := a.getToolTimeout()
			userMinSec := int(userMin.Seconds())
			effectiveTimeout := userMinSec
			if llmSuggested > effectiveTimeout {
				effectiveTimeout = llmSuggested
			}

			timeoutStr := "no timeout"
			if effectiveTimeout > 0 {
				timeoutStr = fmt.Sprintf("%ds", effectiveTimeout)
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
				defer cancel()
			}
			log.Info("Tool call: %s, effective timeout=%s (user min: %ds, LLM suggested: %ds), args=%v",
				tc.Name, timeoutStr, userMinSec, llmSuggested, args)
			result, err := tool.Callback(ctx, args)
			if err != nil {
				log.Error("Tool call failed: %s, error: %v", tc.Name, err)
				return "", err
			}
			log.Debug("Tool call result: %s -> %s", tc.Name, result)
			return result, nil
		}
	}

	return "", fmt.Errorf("tool %q not found", tc.Name)
}

// askFollowupQuestion presents a question with optional options to the user
// and returns their selection. This tool allows interactive problem-solving
// by enabling direct communication with the user.
func (a *Agent) askFollowupQuestionTool(ctx context.Context, args map[string]interface{}) (string, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}

	// Get options (optional)
	var options []string
	if opts, ok := args["options"].([]interface{}); ok {
		for _, opt := range opts {
			if optStr, ok := opt.(string); ok {
				options = append(options, optStr)
			}
		}
	}

	io := a.defaultIO()

	// Display the question
	io.Println()
	io.Printf("❓ %s\n", question)

	if len(options) > 0 {
		io.Println()
		io.Println("  可选回复:")
		for i, opt := range options {
			io.Printf("    [%d] %s\n", i+1, opt)
		}
		io.Println()
	}

	io.Printf("  请输入回复: ")

	// Read user input via UserIO
	input, err := io.ReadLine()
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}
	input = strings.TrimSpace(input)

	// If options provided and user entered a number, return the selected option
	if len(options) > 0 && input != "" {
		if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(options) {
			selected := options[idx-1]
			io.Printf("  ✅ 已选择: %s\n", selected)
			return selected, nil
		}
	}

	// Return the user's input as-is
	return input, nil
}

// attemptCompletionTool presents the final result to the user, optionally executing a demo command.
func (a *Agent) attemptCompletionTool(ctx context.Context, args map[string]interface{}) (string, error) {
	result, _ := args["result"].(string)
	if result == "" {
		return "", fmt.Errorf("result is required")
	}

	command, _ := args["command"].(string)

	// If a command was provided, execute it as a demo
	var cmdOutput string
	if command != "" {
		log.Info("attemptCompletion: executing demo command: %s", command)
		shell, shellArg := shellCmd()
		cmd := exec.CommandContext(ctx, shell, shellArg, command)
		output, err := cmd.CombinedOutput()
		decoded := decodeToUTF8(output)
		if err != nil {
			log.Warn("attemptCompletion: demo command failed: %v\nOutput: %s", err, decoded)
			cmdOutput = fmt.Sprintf("\n命令执行失败: %v\n输出: %s", err, decoded)
		} else {
			cmdOutput = fmt.Sprintf("\n命令执行成功，输出:\n%s", strings.TrimSpace(decoded))
		}
	}

	// Mark the task as completed so RunStream knows this is the final answer
	a.SetCompleted()

	// Build the final completion message
	message := fmt.Sprintf("✅ 任务完成\n\n%s", result)
	if cmdOutput != "" {
		message += "\n" + cmdOutput
	}

	return message, nil
}

// SetCompleted marks the current task as completed.
// RunStream checks this flag before deciding whether to exit on 0 tool calls.
func (a *Agent) SetCompleted() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.completed = true
}
