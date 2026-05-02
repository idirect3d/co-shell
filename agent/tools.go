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
	"strconv"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// buildTools constructs the list of available tools for the LLM.
func (a *Agent) buildTools() []llm.Tool {
	sh := shellName()
	tools := []llm.Tool{
		{
			Name:        "execute_command",
			Description: fmt.Sprintf("Execute a system command (%s) and return its output. Use this to run shell commands, scripts, or any CLI tools.", sh),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Timeout in seconds (0 = no timeout, default: 0)",
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
			Description: "Replace sections of content in an existing file using SEARCH/REPLACE blocks. Accepts a 'replacements' array where each element is an object with 'search' (the exact content to find) and 'replace' (the new content). Supports multiple replacements in a single call. The SEARCH content must match the file exactly (including whitespace and indentation). A backup is automatically created before writing. Returns detailed diff information showing which lines were changed. Use this to make targeted changes to specific parts of a file.",
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
							},
							"required": []string{"search", "replace"},
						},
						"description": "An array of replacement objects, each with 'search' and 'replace' string fields. All replacements are performed sequentially in order.",
					},
				},
				"required": []string{"path", "replacements"},
			},
			Callback: a.replaceInFileTool,
		},
		{
			Name:        "write_to_file",
			Description: "Write content to a file at the specified path. If the file exists, it will be overwritten. If the file doesn't exist, it will be created. Any necessary directories will be created automatically. Use this to create new files or completely rewrite existing files.",
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
				Description: "Launch a sub-agent process that runs independently in its own workspace under the parent's sub-agents/ directory. Each sub-agent gets a sequential ID (1, 2, 3, ...) and its workspace is auto-created at {parent_workspace}/sub-agents/{id}/. The sub-agent shares the same terminal (stdin/stdout/stderr) with the parent agent. After the sub-agent completes, its results (including output files) are collected and reported. Use this to delegate complex or long-running tasks to a separate co-shell instance. You can reuse an existing sub-agent by specifying its ID to continue working on the same task.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"sub_agent_id": map[string]interface{}{
							"type":        "number",
							"description": "Optional: the ID of an existing sub-agent to reuse. If provided, the sub-agent's existing workspace will be used. If omitted, a new sub-agent with a new ID will be created.",
						},
						"instruction": map[string]interface{}{
							"type":        "string",
							"description": "The natural language instruction or system command for the sub-agent to execute.",
						},
						"purpose": map[string]interface{}{
							"type":        "string",
							"description": "A brief description of what this sub-agent is used for. This is stored in memory for future reference. Required when creating a new sub-agent.",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "number",
							"description": "Maximum time in seconds to wait for the sub-agent to complete. 0 means no timeout (default: 0).",
						},
					},
					"required": []string{"instruction"},
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
		}
		tools = append(tools, memoryTools...)
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
			timeout := a.getToolTimeout()
			timeoutStr := "no timeout"
			if timeout > 0 {
				timeoutStr = timeout.String()
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			log.Info("Tool call: %s, timeout=%s, args=%v", tc.Name, timeoutStr, args)
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
