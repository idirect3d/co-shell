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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// buildTools constructs the list of available tools for the LLM.
func (a *Agent) buildTools() []llm.Tool {
	// If tool calling is disabled, return empty tools list
	if !a.toolCallEnabled {
		return []llm.Tool{}
	}

	sh := shellName()
	tools := []llm.Tool{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
			Name:        "replace_in_file",
			Description: "Replace sections of content in an existing file using SEARCH/REPLACE blocks. Accepts a 'replacements' array where each element is an object with 'search' (the exact content to find), 'replace' (the new content), and optional 'start_line' (the 1-based line number in the original file for precise positioning). Supports multiple replacements in a single call. The SEARCH content must match the file exactly (including whitespace and indentation). When 'start_line' is provided, the search is anchored to that line (adjusted for previous replacements' line changes). A backup is automatically created before writing. Returns detailed diff information showing which lines were changed. Use this to make targeted changes to specific parts of a file.",
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
									"description": "Optional: the 1-based line number in the original file where this SEARCH block is expected to start. Used for precise positioning and to avoid duplicate matches. The system automatically adjusts for line count changes from previous replacements.",
								},
							},
							"required": []string{"search", "replace"},
						},
						"description": "An array of replacement objects, each with 'search' and 'replace' string fields, and optional 'start_line' number. All replacements are performed sequentially in order.",
					},
				},
				"required": []string{"path", "replacements"},
			},
			Callback: a.replaceInFileTool,
		},
		{
			Name:        "write_to_file",
			Description: "Write content to a file at the specified path. If the file exists, it will be overwritten. If the file doesn't exist, it will be created. Any necessary directories will be created automatically. IMPORTANT: When fixing errors in an existing file, prefer using replace_in_file instead of write_to_file. Using write_to_file to rewrite complex files often reintroduces the same issues. Use write_to_file primarily for creating new files or when a complete rewrite is truly necessary.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The absolute path to the file to write to",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The full content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
			Callback: a.writeToFileTool,
		},
		{
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
		},
		{
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
		},
		{
			Name:        "clear_images",
			Description: "Clear all cached image file paths. After calling this, no images will be included in subsequent conversations. Use this when you want to stop sending images to the LLM.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
			Callback: a.clearImagesTool,
		},
	}

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
				Name:        "list_task_plans",
				Description: "Show the current task plan (checklist) with its progress summary. Returns the plan's ID, title, completion percentage, and all steps with their statuses. Use this to check the current progress of the active task plan.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
				Callback: a.listTaskPlansTool,
			},
			{
				Name:        "view_task_plan",
				Description: "View the full details of the current task plan (checklist), including all steps (checklist items) with their statuses and notes. Use this to examine the progress of the current plan in detail.",
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
								"description": "The parameter name to change. See the .set command help for all available parameters (e.g., model, temperature, max-tokens, show-llm-thinking, confirm-command, etc.)",
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
		Description: "Adjust the context start pointer position. Allows the LLM to dynamically decide how much conversation history to keep based on context content, ignoring irrelevant early messages. Only available when context_start_mode is set to 'smart'.",
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

	return tools
}

// executeToolCall runs a single tool call and returns the result.
func (a *Agent) executeToolCall(ctx context.Context, tc llm.ToolCall) (string, error) {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return "", fmt.Errorf("cannot parse tool arguments: %w", err)
	}

	// If confirmCommand is enabled and this is an execute_command call,
	// prompt the user for confirmation before proceeding
	if a.confirmCommand && tc.Name == "execute_command" {
		if cmd, ok := args["command"].(string); ok {
			// Skip confirmation if user chose "approve all" for this request
			// or if there are remaining auto-approve counts
			if !a.approveAll && a.approveCount <= 0 {
				result, modifyInput := promptCommandConfirmation(cmd)
				switch result {
				case CmdConfirmCancel:
					return i18n.T(i18n.KeyCmdConfirmCancelled), fmt.Errorf("CANCEL_AGENT")
				case CmdConfirmApproveAll:
					a.approveAll = true
					// fall through to execute
				case CmdConfirmApproveCount:
					// Parse the number of commands to auto-approve
					if n, err := strconv.Atoi(modifyInput); err == nil && n > 0 {
						a.approveCount = n
						fmt.Printf("\n✅ 已批准后续 %d 次命令执行\n", a.approveCount)
					}
					// fall through to execute
				case CmdConfirmModify:
					// Use the user's input directly as supplementary instructions
					// for the LLM to re-evaluate the command
					return "", fmt.Errorf("USER_MODIFY_REQUEST: %s", modifyInput)
				}
				// CmdConfirmApprove: continue execution
			} else if a.approveCount > 0 {
				// Decrement approve count and auto-approve
				a.approveCount--
				fmt.Printf("\n✅ 已自动批准（剩余 %d 次）\n", a.approveCount)
			}
		}
	}

	// Find and execute the tool
	tools := a.buildTools()
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

	// Display the question
	fmt.Println()
	fmt.Printf("❓ %s\n", question)

	if len(options) > 0 {
		fmt.Println()
		fmt.Println("  可选回复:")
		for i, opt := range options {
			fmt.Printf("    [%d] %s\n", i+1, opt)
		}
		fmt.Println()
	}

	fmt.Print("  请输入回复: ")

	// Read user input using bufio.Scanner on stdin
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	// If options provided and user entered a number, return the selected option
	if len(options) > 0 && input != "" {
		if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(options) {
			selected := options[idx-1]
			fmt.Printf("  ✅ 已选择: %s\n", selected)
			return selected, nil
		}
	}

	// Return the user's input as-is
	return input, nil
}
