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
	"github.com/idirect3d/co-shell/store"
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
			Description: fmt.Sprintf("Execute a system command (%s) and return its output. Use this to run shell commands, scripts, or any CLI tools. You MUST specify timeout_seconds (0 = wait forever) and on_timeout (what to do when the timeout fires).", sh),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Timeout in seconds. 0 means wait forever (no timeout). Set this based on your estimate of how long the command will take. When greater than 0, the actual timeout used will be the maximum of this value and the user-configured minimum timeout.",
					},
					"on_timeout": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"kill", "detach"},
						"description": "What to do when timeout_seconds fires. \"kill\": terminate the whole process group and return an error (use for ordinary foreground commands). \"detach\": stop waiting and return the PID, partial output and a log file path while the process keeps running in the background (use for servers, long builds, watchers); you can later inspect the log file or kill the PID with another execute_command call. Ignored when timeout_seconds is 0.",
					},
				},
				"required": []string{"meta", "command", "timeout_seconds", "on_timeout"},
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
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
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
				"required": []string{"meta", "command"},
			},
			Callback: a.shellSendTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_window_content",
			Description: "Get the current virtual terminal window content as a text snapshot. This shows what is currently displayed on the terminal screen. Use this to check the state of a long-running process, review previous command output, or inspect the current terminal state without sending a new command. Returns the full window content as rows x cols text.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.shellWindowContentTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_get_output",
			Description: "Retrieve output from the persistent shell session since the last time shell_send or shell_get_output was called (auto-increment mode), or from a specific position.\n\nAuto-increment mode (no last_from/count): returns only the new content that has been produced since the last shell_send or shell_get_output call. This is useful for checking progress of a long-running command or REPL session.\n\nLegacy mode (with last_from/count): returns terminal scrollback history — like scrolling up in a terminal window. Use this to review what happened before the last command.\n\nParameters: wait_ms (optional, default 200ms — wait this long for new output before returning), last_from (optional, 1-based from end where 1=most recent line), count (optional, number of lines to return), timeout_seconds (optional, total timeout in seconds).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"wait_ms": map[string]interface{}{
						"type":        "number",
						"description": "Optional observation wait time in milliseconds (default: 200). Wait this long for new output before returning. Increase for checking progress of a running command.",
					},
					"last_from": map[string]interface{}{
						"type":        "number",
						"description": "Starting position from the end (1-based, 1=most recent line). If not provided, uses auto-increment mode (returns only new content since last call).",
					},
					"count": map[string]interface{}{
						"type":        "number",
						"description": "Number of lines to return. If not provided with last_from, uses auto-increment mode.",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional total timeout in seconds. Set this to prevent infinite waiting. 0 or omitted means no total timeout (use the default shell-session-timeout).",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.shellGetOutputTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "shell_reset",
			Description: "Reset the persistent shell session to a clean state. This closes the current session and starts a new one with a fresh terminal. Use this when the shell is in an unexpected state (e.g., inside a REPL with errors, or stuck in a process). The shell session is normally managed automatically — use this only when a manual reset is needed.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.shellResetTool,
		})
	}

	// File operation tools (always available)
	tools = append(tools, llm.Tool{
		Name:        "read_file",
		Description: "Read the contents of a file at the specified path. Use this to examine the contents of an existing file. Returns the file content with line numbers. Both start_line and end_line are REQUIRED — you must specify the line range to read. IMPORTANT: This tool can ONLY read text files (.txt, .md, .go, .py, .js, .html, .css, .json, .xml, .yaml, .csv, .log, etc.). Do NOT use this tool to read image files (.png, .jpg, .gif, .webp, .bmp) or other binary files — use add_images to load images for multimodal analysis instead.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
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
			"required": []string{"meta", "path", "start_line", "end_line"},
		},
		Callback: a.readFileTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "search_files",
		Description: "Search for a regex pattern across files in a specified directory. Returns matching lines with surrounding context. Use this to find specific code patterns, function definitions, or text across multiple files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
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
			"required": []string{"meta", "path", "regex"},
		},
		Callback: a.searchFilesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "list_files",
		Description: "List files and directories within the specified directory. recursive controls recursion depth: 0=top-level only (default), 1=one level deep, 2=two levels, etc. Use this to explore directory structures and find files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to list contents for (absolute or relative to current working directory)",
				},
				"recursive": map[string]interface{}{
					"type":        "number",
					"description": "Recursion depth: 0=top-level only (default), 1=one level deep, 2=two levels, etc.",
				},
			},
			"required": []string{"meta", "path"},
		},
		Callback: a.listFilesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "list_code_definition_names",
		Description: "List definition names (functions, types, methods, etc.) in source code files at the top level of a specified directory. Use this to quickly understand the structure and API of a codebase.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory path to list definitions for (absolute or relative to current working directory)",
				},
			},
			"required": []string{"meta", "path"},
		},
		Callback: a.listCodeDefinitionNamesTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "visual_analysis",
		Description: "Load one or more image/video files for multimodal visual analysis by the LLM's vision capability. Provide an array of file paths and specify what to analyze. This tool supports:\n- OCR / text recognition: recognize and extract text from images, scanned documents, screenshots, signs, handwriting\n- Image understanding: describe scenes, objects, people, layouts, colors, and visual relationships\n- Table/data extraction: extract structured data from tables, charts, graphs, and infographics\n- Document analysis: extract content from report pages, forms, certificates, invoices\n- Video frame analysis: analyze screenshots or extracted frames from videos\n\nThe files are sent to the LLM exactly once in the next iteration and automatically removed from cache after delivery. To analyze more files, call this tool again. The maximum number of files per call is controlled by the 'visual-analysis-max-images' config setting (default: 5). IMPORTANT: You MUST specify the 'intent' parameter to describe what specific information you need from the visual input.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Array of image/video file paths to load for visual analysis (e.g., ['page1.png', 'page2.png', 'diagram.jpg']). Maximum controlled by visual-analysis-max-images config.",
				},
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"instruct": map[string]interface{}{
					"type":        "string",
					"description": "The explicit instruction for the vision model describing what to analyze/extract from the image(s). This is distinct from meta.intent (which is the intent shown to the user).",
				},
			},
			"required": []string{"meta", "paths", "instruct"},
		},
		Callback: a.visualAnalysisTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "replace_in_file",
		Description: "Replace sections of content in an existing file using 'search'/'replace' blocks. Accepts a 'replacements' array where each element is an object with 'search' (the exact content to find), 'replace' (the new content), and optional 'start_line' (the 1-based line number in the original file for precise positioning). Supports multiple replacements in a single call. The 'search' content must match the file exactly (including whitespace and indentation). When 'start_line' is provided, the search is anchored to that line (adjusted for previous replacements' line changes). A backup is automatically created before writing. Returns detailed diff information showing which lines were changed. Use this to make targeted changes to specific parts of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				// FEATURE-328: primary display param (path) first, then
				// intent — so streaming header "replace_in_file <path>"
				// renders as early as possible.
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to modify (absolute or relative to current working directory)",
				},
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"replacements": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"intent": map[string]interface{}{
								"type":        "string",
								"description": "Explain why you are calling this tool and what you expect to accomplish. This helps track and debug LLM decision-making.",
							},
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
								"description": "the 1-based line number in the original file where this 'search' content is expected to start. Used for precise positioning and to avoid duplicate matches. The system automatically adjusts for line count changes from previous replacements.",
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
			"required": []string{"meta", "path", "replacements"},
		},
		Callback: a.replaceInFileTool,
	})
	tools = append(tools, llm.Tool{
		Name:        "write_to_file",
		Description: "Write content to a file at the specified path. The 'mode' parameter controls the operation:\n  - 'new': creates a NEW file. Fails if the file already exists.\n  - 'rewrite': overwrites an EXISTING file with new content. Fails if the file doesn't exist.\n  - 'append': appends content to an EXISTING file. Fails if the file doesn't exist.\n\nThe three modes are mutually exclusive and non-interchangeable — use the correct mode for your operation. Any necessary parent directories are created automatically only in 'new' mode.\n\nPERFORMANCE TIP: When writing large files (over ~100 lines), avoid putting all content in a single call — this may trigger long-output loop detection. Instead, use 'new' mode for the first ~100 lines, then follow up with multiple 'append' mode calls for the remaining content.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				// FEATURE-328: primary display param (path) first, then
				// intent — so streaming header "write_to_file <path>"
				// renders as early as possible.
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the file to write to",
				},
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "The write mode. One of: 'new' (create new file), 'rewrite' (overwrite existing file), 'append' (append to existing file). The three modes are mutually exclusive and non-interchangeable.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file. For 'append' mode, this content is appended to the end of the file.",
				},
			},
			"required": []string{"meta", "mode", "path", "content"},
		},
		Callback: a.writeToFileTool,
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
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
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
					"required": []string{"meta", "sub_agent_name", "instruction"},
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
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
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
				"required": []string{"meta", "name", "cron", "instruction"},
			},
			Callback: a.scheduleTaskTool,
		})
	}

	// Add task plan tools only if plan enabled
	if a.planEnabled {
		planTools := []llm.Tool{
			{
				Name:        "track_task_progress",
				Description: "PRIMARILY used to INITIALIZE a task plan: record task content and create the execution plan by passing the complete array of steps as the desired state — the system handles creation or replacement automatically. After the plan is created, DO NOT call this tool repeatedly to update progress; instead track execution via the meta.progress field of other tool calls (see the system prompt). DESCRIPTION usage: for detailed plans, write the full plan context, background, constraints, and acceptance criteria. STEP.DESCRIPTION usage: the first line is the step title/summary; subsequent lines provide detailed content. STATUS values: \"[ ]\" (pending/todo), \"[=]\" (in_progress), \"[X]\" (completed), \"[C]\" (cancelled), \"[F]\" (failed). XML example:\n<track_task_progress>\n  <title>Implement user login</title>\n  <description>Full plan with background, constraints...\n  </description>\n  <steps>\n    <item>\n      <description>Design login API\nImplement POST /auth/login endpoint</description>\n      <status>[X]</status>\n    </item>\n    <item>\n      <description>Write login form component\nUse React Hook Form + Zod validation</description>\n      <status>[=]</status>\n    </item>\n    <item>\n      <description>Write and run tests\nTest login success, failure, rate limiting</description>\n      <status>[ ]</status>\n    </item>\n  </steps>\n</track_task_progress>",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "The title of the task plan. Required when creating a new plan; optional when updating.",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "A detailed description of the task plan. For detailed plans, include the full context, background, constraints, technical approach, and acceptance criteria.",
						},
						"acceptance_criteria": map[string]interface{}{
							"type":        "array",
							"description": "Verifiable acceptance criteria of the whole plan (FEATURE-514). The supervisor model checks the delivery against each item and MUST reject it when any item is unmet. Use concrete, checkable statements (e.g. \"go build ./... passes\"). Optional; when omitted, previous criteria are preserved.",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"steps": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"description": map[string]interface{}{
										"type":        "string",
										"description": "Step description. The first line is the step title/summary; subsequent lines provide detailed content. Can contain multi-line text for complex steps.",
									},
									"status": map[string]interface{}{
										"type":        "string",
										"enum":        []string{"[ ]", "[=]", "[X]", "[C]", "[F]", "pending", "in_progress", "completed", "cancelled", "failed"},
										"description": "Step status: \"[ ]\" or \"pending\" for todo, \"[=]\" or \"in_progress\" for in progress, \"[X]\" or \"completed\" for completed, \"[C]\" or \"cancelled\" for cancelled, \"[F]\" or \"failed\" for failed.",
									},
								},
								"required": []string{"description", "status"},
							},
							"description": "Array of step objects, each with description and status. Passing the complete array sets the desired state. Empty array archives and deletes the current plan.",
						},
					},
					"required": []string{"title", "description", "steps"},
				},
				Callback: a.trackTaskProgressTool,
			},
			{
				Name:        "view_task_plan",
				Description: "View the current task plan (checklist) with its progress summary, including all steps with their statuses. Use this to check the current progress of the active task plan.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
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
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"last_from": map[string]interface{}{
							"type":        "number",
							"description": "Starting position from the end (inclusive). 1 = most recent message. Must be >= last_to.",
						},
						"last_to": map[string]interface{}{
							"type":        "number",
							"description": "Ending position from the end (inclusive). 1 = most recent message.",
						},
					},
					"required": []string{"meta", "last_from", "last_to"},
				},
				Callback: a.getMemorySliceTool,
			},
			{
				Name:        "memory_search",
				Description: "Search persistent conversation memory for messages matching given keywords or criteria. Use this to find specific information from past conversations. Supports keyword search (AND logic), time-based filtering (since), and speaker name filtering.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
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
					"required": []string{"meta"},
				},
				Callback: a.memorySearchTool,
			},
			{
				Name:        "delete_memory",
				Description: "Delete a range of conversation history from persistent memory. Use this to remove outdated or incorrect information from memory. Parameters: last_from (starting position from the end, 1=most recent), last_to (ending position from the end, 1=most recent). Example: last_from=5, last_to=1 deletes the 5 most recent messages.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"last_from": map[string]interface{}{
							"type":        "number",
							"description": "Starting position from the end (inclusive). 1 = most recent message. Must be >= last_to.",
						},
						"last_to": map[string]interface{}{
							"type":        "number",
							"description": "Ending position from the end (inclusive). 1 = most recent message.",
						},
					},
					"required": []string{"meta", "last_from", "last_to"},
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
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"settings": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"intent": map[string]interface{}{
								"type":        "string",
								"description": "Explain why you are calling this tool and what you expect to accomplish. This helps track and debug LLM decision-making.",
							},
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
			"required": []string{"meta", "settings"},
		},
		Callback: a.updateSettingsTool,
	})

	// Add list_settings tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "list_settings",
		Description: "List all available co-shell system configuration parameters with their current values, valid ranges, and descriptions. Use this to discover what settings can be modified via the update_settings tool. This is useful when you need to understand the available configuration options before making changes.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
			},
			"required": []string{"meta"},
		},
		Callback: a.listSettingsTool,
	})

	// Add ask_user tool (always available, FEATURE-512)
	tools = append(tools, a.buildAskUserTool())

	// Add evaluate_expression tool (always available)
	tools = append(tools, llm.Tool{
		Name:        "evaluate_expression",
		Description: "Evaluate a mathematical expression and return the exact result. Supports: basic arithmetic (+, -, *, /, %), exponentiation (^), parentheses for grouping, trigonometric functions (sin, cos, tan, asin, acos, atan), logarithms (log=base10, ln=natural), square root (sqrt), absolute value (abs), rounding (ceil, floor, round), and constants (pi, e). All trigonometric functions use radians. Use this for precise calculations instead of relying on Python or shell commands.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The mathematical expression to evaluate. Examples: '3 + 4 * 2', 'sin(pi/2)', '2 ^ 10', 'sqrt(144)', 'log(100) + ln(e)', '45 * (1 + 0.05) ^ 10', 'abs(-5) + round(3.7)'",
				},
			},
			"required": []string{"meta", "expression"},
		},
		Callback: a.evaluateExpressionTool,
	})

	// Add render_ui tool (FEATURE-524): lets the LLM answer with a structured
	// component tree instead of plain text. Not advertised at all when the
	// ui_enabled switch is off.
	if a.uiEnabled() {
		tools = append(tools, a.buildRenderUITool())
	}

	// Add attempt_completion tool (always available)
	tools = append(tools, llm.Tool{
		Name: "attempt_completion",
		Description: `After each tool use, the user will respond with the result of that tool use, i.e. if it succeeded or failed, along with any reasons for failure. Once you've received the results of tool uses and can confirm that the task is complete, use this tool to present the result of your work to the user. Optionally you may provide a CLI command to showcase the result of your work. The user may respond with feedback if they are not satisfied with the result, which you can use to make improvements and try again.
IMPORTANT NOTE: This tool CANNOT be used until you've confirmed from the user that any previous tool uses were successful. Failure to do so will result in code corruption and system failure. Before using this tool, you must ask yourself in <thinking></thinking> tags if you've confirmed from the user that any previous tool uses were successful. If not, then DO NOT use this tool.
If you were using create_task_plan/update_task_step/... to manage the task progress, all unfinished tasks will be set to finish state.
Besides result and command, this tool also requires session_title (a brief title ≤60 chars) and session_keywords (comma-separated keywords) for automatic session saving.
When the completion-confirm switch is enabled (default), this tool presents the result and asks the user to choose a next step: the user may pick one of your next_steps suggestions, ask for more suggestions, report the task is not yet done, or confirm completion to exit.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"result": map[string]interface{}{
					"type":        "string",
					"description": "The result of the tool use. This should be a clear, specific description of the result.",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "A CLI command to execute to show a live demo of the result to the user. For example, use 'open index.html' to display a created html website, or 'open localhost:3000' to display a locally running development server. But DO NOT use commands like 'echo' or 'cat' that merely print text. This command should be valid for the current operating system. Ensure the command is properly formatted and does not contain any harmful instructions.",
				},
				"task_message_no": map[string]interface{}{
					"type":        "integer",
					"description": "The message number to set as the new context start pointer after task completion. This truncates older conversation history, keeping only recent context. The value should be taken from the message_no field in <environment_details>. Use this when the task involved many iterations and the conversation context has grown long.",
				},
				"session_title": map[string]interface{}{
					"type":        "string",
					"description": "A brief title summarizing this session (≤60 characters). Combine the current session title (if any, from <session_title> in <environment_details>) with the newly completed work into a concise session-level summary. Use keywords / parallel phrases to densely condense all distinct topics done so far in this session; do not drop earlier distinct work when adding new topics. This will be used to name the saved session.",
				},
				"session_keywords": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated keywords summarizing the task's technology, domain, and purpose for future classification and retrieval.",
				},
				"next_steps": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional. 1 or more suggested next steps the user could choose to continue the task. When provided, these are shown as selectable options in the completion-confirm dialog. Omit when the task is fully done and no further work is suggested.",
				},
			},
			"required": []string{"meta", "result", "session_title", "session_keywords"},
		},
		Callback: a.attemptCompletionTool,
	})

	// Add reorganize_context tool (FEATURE-249, always available)
	tools = append(tools, llm.Tool{
		Name: "reorganize_context",
		Description: `Reorganize the conversation context by replacing the current history with a self-contained summary prompt generated by the LLM. Use this when:
1. The context window is nearly full and you need to compress the conversation
2. You are stuck in a loop and need to reset context and try a different approach
3. The conversation has drifted from the original goal and needs refocusing

After calling this tool, ALL previous messages are removed from the active context (the messagePointer is moved past them), and the summary_prompt becomes the new user message. The system will then resume with a fresh context consisting only of the system prompt and this summary_prompt.

**IMPORTANT — How to write an effective summary_prompt:**
The summary_prompt is your continuation prompt that replaces all previous conversation history. It MUST be self-contained and follow these four layers:

1. **Original Goal** — Restate the user's original task, success criteria, and what the final deliverable should be.
2. **Progress Assessment** — Summarize what has been completed (e.g., "30% done"), which steps have been verified working, which are blocked and why.
3. **Method Review** — List each approach tried, marking each as "effective/ineffective/partial". For ineffective approaches, analyze WHY they failed. This is critical to avoid repeating the same mistakes.
4. **Strategy Revision** — Based on the review, propose NEW, different approaches. If the current path is not working, YOU MUST pivot to an alternative. Do NOT simply continue the same failed strategy. Consider breaking down the problem differently, using different tools, or asking the user for guidance.

**Constraints:**
- Do NOT simply compress information — you MUST analyze and optimize the strategy.
- Preserve ALL critical hard data: file paths, error log snippets, key code snippets, URLs. Losing these will make the task unrecoverable.
- If you have called reorganize_context multiple times with no progress, consider: lowering goals, splitting the task, asking the user to simplify requirements, or switching models.
- The summary_prompt will be submitted as a user message to the next LLM invocation, so it must be a complete, actionable instruction that can independently guide task continuation.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
				},
				"summary_prompt": map[string]interface{}{
					"type":        "string",
					"description": "The context reorganization summary and task continuation prompt. Generated by the LLM to replace all previous conversation history. MUST include: 1) Original task goal and success criteria 2) Completed work and status assessment 3) Tried methods with effectiveness analysis 4) Optimized strategy revision with new approach 5) All preserved critical data (file paths, error messages, code snippets). This prompt must be fully self-contained to guide the next LLM invocation independently.",
				},
			},
			"required": []string{"summary_prompt"},
		},
		Callback: a.reorganizeContextTool,
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
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to navigate to (must include protocol, e.g. https://example.com)",
						},
					},
					"required": []string{"meta", "url"},
				},
				Callback: a.browserNavigateTool,
			},
			{
				Name:        "browser_screenshot",
				Description: "Capture a screenshot of the current browser page. The screenshot is automatically sent to the vision model for analysis (vision models only). The `intent` parameter is used as the vision-recognition instruction for the screenshot, so state a specific analysis goal — what information to extract or verify from the page — rather than a vague request. Use this to observe the page content, layout, and elements. Parameters: intent (required), quality (optional, 1-100, default 80), full_page (optional, boolean, default false). For full-page screenshots, set full_page=true.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"instruct": map[string]interface{}{
							"type":        "string",
							"description": "The explicit instruction for the vision model describing what to analyze/extract from the image(s). This is distinct from meta.intent (which is the intent shown to the user).",
						},
						"quality": map[string]interface{}{
							"type":        "number",
							"description": "Screenshot quality (1-100, default 80). Higher quality gives better visual detail for analysis.",
						},
						"full_page": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to capture the full page (true) or just the visible viewport (false, default). Full page captures all scrollable content.",
						},
					},
					"required": []string{"meta", "instruct"},
				},
				Callback: a.browserScreenshotTool,
			},
			{
				Name:        "browser_click",
				Description: "Click at the specified coordinates (x, y) on the browser page. Use coordinates from browser_get_interactive_elements results (centerX, centerY). After clicking, use browser_screenshot to observe the result.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"x": map[string]interface{}{
							"type":        "number",
							"description": "The x-coordinate to click at (in pixels from the left edge of the viewport)",
						},
						"y": map[string]interface{}{
							"type":        "number",
							"description": "The y-coordinate to click at (in pixels from the top edge of the viewport)",
						},
					},
					"required": []string{"meta", "x", "y"},
				},
				Callback: a.browserClickTool,
			},
			{
				Name:        "browser_type",
				Description: "Type text into the currently focused element on the browser page. Use this to fill in form fields, search boxes, or text areas. Optionally clear existing content first by setting clear=true.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The text to type into the focused element",
						},
						"clear": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to clear existing content before typing (default: false)",
						},
					},
					"required": []string{"meta", "text"},
				},
				Callback: a.browserTypeTool,
			},
			{
				Name:        "browser_evaluate",
				Description: "Execute JavaScript code in the browser page context and return the result. Use this for advanced operations like extracting data, modifying page content, or getting specific information from the page.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "The JavaScript expression or code to execute in the browser",
						},
					},
					"required": []string{"meta", "expression"},
				},
				Callback: a.browserEvaluateTool,
			},
			{
				Name:        "browser_get_rendered_html",
				Description: "Get the rendered DOM HTML of the current browser page after all JavaScript has executed. The HTML is serialized from the live DOM tree in Chrome's memory — it reflects the final rendered state including SPA framework output, dynamic content, and all JS modifications. This is NOT the raw source HTML; you get the page as the browser sees it after rendering, so there is NO need to separately download JS, JSON, or other resources. Use this to analyze the complete document structure, extract rendered data, or process interactive web applications.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserGetHTMLTool,
			},
			{
				Name:        "browser_scroll",
				Description: "Scroll the browser page by the specified delta. Use this to view content below or above the current viewport. Positive delta_y scrolls down, negative scrolls up. Default: scroll down 500 pixels.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"delta_x": map[string]interface{}{
							"type":        "number",
							"description": "Horizontal scroll delta in pixels (positive = right, negative = left, default: 0)",
						},
						"delta_y": map[string]interface{}{
							"type":        "number",
							"description": "Vertical scroll delta in pixels (positive = down, negative = up, default: 500)",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserScrollTool,
			},
			{
				Name:        "browser_get_interactive_elements",
				Description: "Get a list of interactive elements (buttons, links, inputs, etc.) on the current page with their positions, text, and attributes. Use this to find clickable elements and their coordinates for use with browser_click.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserGetInteractiveElementsTool,
			},
			{
				Name:        "browser_go_back",
				Description: "Navigate back to the previous page in browser history. After going back, use browser_screenshot to view the resulting page.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserGoBackTool,
			},
			{
				Name:        "browser_go_forward",
				Description: "Navigate forward to the next page in browser history. After going forward, use browser_screenshot to view the resulting page.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserGoForwardTool,
			},
			{
				Name:        "browser_close",
				Description: "Close the Chrome browser and clean up resources. Use this when you are done with browser automation tasks.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.browserCloseTool,
			},
		}
		tools = append(tools, browserTools...)
	}

	// Add vault tools if vault store is initialized
	if a.vaultStore != nil {
		vaultTools := []llm.Tool{
			{
				Name:        "vault_list",
				Description: "List all vault entry names. Use this to discover available credential entries. The LLM can see entry names but NOT passwords/usernames. After listing, use @pwd:entry_name or @user:entry_name in other tool calls to reference the credentials.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
					},
					"required": []string{"meta"},
				},
				Callback: a.vaultListTool,
			},
			{
				Name:        "vault_add",
				Description: "Add a new entry to the password vault. LLM provides the entry name (e.g., 'prod_db', 'email'), and an optional URL and notes. The username and password will be prompted interactively from the user for security - they are NOT passed through the LLM. Use this when the user asks to save credentials for an account or service.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The entry name for referencing via @pwd:name, @user:name, or @vault:name. Example: 'prod_db', 'email', 'aws_console'",
						},
						"url": map[string]interface{}{
							"type":        "string",
							"description": "Optional URL associated with this account",
						},
						"notes": map[string]interface{}{
							"type":        "string",
							"description": "Optional notes for this entry",
						},
					},
					"required": []string{"meta", "name"},
				},
				Callback: a.vaultAddTool,
			},
			{
				Name:        "vault_remove",
				Description: "Remove a vault entry by name. This permanently deletes the stored credentials. Use with caution and confirm with the user before removing.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type":        "object",
							"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The name of the vault entry to remove",
						},
					},
					"required": []string{"meta", "name"},
				},
				Callback: a.vaultRemoveTool,
			},
		}
		tools = append(tools, vaultTools...)
	}

	// Add Excel tools (FEATURE-120) — always available
	excelTools := []llm.Tool{
		{
			Name:        "excel_open",
			Description: "Open an XLSX file and return a session ID for subsequent operations. Use this first before any other excel_* tools. The session keeps the file in memory for efficient multi-step operations. mode is REQUIRED: 'create' (create new file, must not exist), 'read' (open existing file, read-only, save will fail), 'copy' (copy file to new name with timestamp before opening). Example: excel_open(path=\"report.xlsx\", mode=\"read\") or excel_open(path=\"new.xlsx\", mode=\"create\") or excel_open(path=\"report.xlsx\", mode=\"copy\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the XLSX file to open (absolute or relative to current working directory)",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "Open mode: 'create' (new empty file), 'read' (read-only), 'copy' (copy with timestamp, then edit copy)",
					},
				},
				"required": []string{"meta", "path", "mode"},
			},
			Callback: a.excelOpenTool,
		},
		{
			Name:        "excel_close",
			Description: "Close an open Excel session. This saves changes to disk (if any) and releases memory. Always close sessions when you are done working with the file. Example: excel_close(session_id=\"xl_1234567890\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.excelCloseTool,
		},
		{
			Name:        "excel_save",
			Description: "Save changes to disk without closing the session. Use this periodically after making edits to persist progress. Example: excel_save(session_id=\"xl_1234567890\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.excelSaveTool,
		},
		{
			Name:        "excel_overview",
			Description: "Get an overview of all sheets in the opened workbook. Returns metadata only (sheet names, data ranges, row/column counts, header hints) — NO cell data is returned. Use this FIRST after opening a file to understand its structure before reading specific ranges. Example: excel_overview(session_id=\"xl_1234567890\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.excelOverviewTool,
		},
		{
			Name:        "excel_read",
			Description: "Read cell data from a specified range in multiple output formats. format='html' (default): HTML table with proper indentation. format='full': HTML table with cell formatting info. format='text': plain text with tab-separated values (each row prefixed with 'N: '). format='md': Markdown table format (each row prefixed with 'N: '). format='grid': original grid format with column letters and row numbers. REQUIRED: session_id, sheet, start_row, end_row, start_col, end_col. max_cells defaults to 1000. Example: excel_read(session_id=\"xl_123\", sheet=\"Sheet1\", start_row=1, end_row=10, start_col=1, end_col=5, format=\"html\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name (e.g. \"Sheet1\") or 1-based index",
					},
					"start_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based start row",
					},
					"end_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based end row",
					},
					"start_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based start column",
					},
					"end_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based end column",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"html", "full", "text", "md", "grid"},
						"description": "output format. Choose from: 'html' (HTML table with indentation), 'full' (HTML with cell formatting), 'text' (TSV with row prefix), 'md' (Markdown table), 'grid' (column letters + row numbers with type prefixes)",
					},
					"max_cells": map[string]interface{}{
						"type":        "number",
						"description": "maximum cells to return (default 1000). If the requested range exceeds this, the tool will return an error asking you to reduce the range.",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "start_row", "end_row", "start_col", "end_col", "format"},
			},
			Callback: a.excelReadTool,
		},
		{
			Name:        "excel_edit",
			Description: "Write values to cells starting from a target cell. Values is a 2D array of strings. If a value starts with '=', it is interpreted as a formula. Example: excel_edit(session_id=\"xl_1234567890\", sheet=\"Sheet1\", start_cell=\"C5\", values=[[\"Q4 Total\", 12500, \"=SUM(C2:C4)\"]])",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name (e.g. \"Sheet1\")",
					},
					"start_cell": map[string]interface{}{
						"type":        "string",
						"description": "Starting cell reference (e.g. \"A1\", \"C5\")",
					},
					"values": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "Row values",
						},
						"description": "2D array of values to write. Example: [[\"Name\", \"Age\"], [\"Alice\", \"30\"]]",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "start_cell", "values"},
			},
			Callback: a.excelEditTool,
		},
		{
			Name:        "excel_copy",
			Description: "Copy a range of cells to the session clipboard. Supports cut mode (cut=true) which marks the source area for deletion on paste. The clipboard is per-session and is automatically cleared on the next excel_read call. Example (copy): excel_copy(session_id=\"xl_1234567890\", sheet=\"Sheet1\", start_row=1, end_row=5, start_col=1, end_col=3) Example (cut): excel_copy(session_id=\"xl_1234567890\", sheet=\"Sheet1\", start_row=1, end_row=5, start_col=1, end_col=3, cut=true)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name",
					},
					"start_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based start row",
					},
					"end_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based end row",
					},
					"start_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based start column",
					},
					"end_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based end column",
					},
					"cut": map[string]interface{}{
						"type":        "boolean",
						"description": "if true, marks as cut (paste will clear source). Default false.",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "start_row", "end_row", "start_col", "end_col"},
			},
			Callback: a.excelCopyTool,
		},
		{
			Name:        "excel_paste",
			Description: "Paste the clipboard content (from excel_copy or excel_cut) to a target cell. If the clipboard is from a cut operation, the source area is automatically cleared after paste. Example: excel_paste(session_id=\"xl_1234567890\", sheet=\"Sheet2\", target_cell=\"F2\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name",
					},
					"target_cell": map[string]interface{}{
						"type":        "string",
						"description": "Target cell reference (e.g. \"F2\")",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "target_cell"},
			},
			Callback: a.excelPasteTool,
		},
		{
			Name:        "excel_insert",
			Description: "Insert rows or columns at a specified position. what must be 'rows' or 'cols'. position is 1-based. count defaults to 1. Existing data shifts down/right. Example: excel_insert(session_id=\"xl_1234567890\", sheet=\"Sheet1\", what=\"rows\", position=3, count=2)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name",
					},
					"what": map[string]interface{}{
						"type":        "string",
						"description": "'rows' or 'cols'",
					},
					"position": map[string]interface{}{
						"type":        "number",
						"description": "1-based position to insert at",
					},
					"count": map[string]interface{}{
						"type":        "number",
						"description": "number of rows/cols to insert (default 1)",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "what", "position"},
			},
			Callback: a.excelInsertTool,
		},
		{
			Name:        "excel_delete",
			Description: "Delete rows, columns, or cell content. what='rows': delete row range; what='cols': delete column range; what='cells': clear cell content without shifting. Example (rows): excel_delete(session_id=\"xl_1234567890\", sheet=\"Sheet1\", what=\"rows\", position=5, count=3) Example (cells): excel_delete(session_id=\"xl_1234567890\", sheet=\"Sheet1\", what=\"cells\", start_row=2, end_row=5, start_col=1, end_col=3)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name",
					},
					"what": map[string]interface{}{
						"type":        "string",
						"description": "'rows', 'cols', or 'cells'",
					},
					"position": map[string]interface{}{
						"type":        "number",
						"description": "For rows/cols: 1-based position to start deletion",
					},
					"count": map[string]interface{}{
						"type":        "number",
						"description": "For rows/cols: number to delete (default 1)",
					},
					"start_row": map[string]interface{}{
						"type":        "number",
						"description": "For cells: 1-based start row",
					},
					"end_row": map[string]interface{}{
						"type":        "number",
						"description": "For cells: 1-based end row",
					},
					"start_col": map[string]interface{}{
						"type":        "number",
						"description": "For cells: 1-based start column",
					},
					"end_col": map[string]interface{}{
						"type":        "number",
						"description": "For cells: 1-based end column",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "what"},
			},
			Callback: a.excelDeleteTool,
		},
		{
			Name:        "excel_sheet",
			Description: "Manage sheets (create, delete, rename, copy, list). action='create': creates a new sheet. action='delete': deletes a sheet. action='rename': renames a sheet. action='copy': copies a sheet. action='list': lists all sheets. Example: excel_sheet(session_id=\"xl_1234567890\", action=\"list\") Example: excel_sheet(session_id=\"xl_1234567890\", action=\"create\", name=\"Q4 Data\") Example: excel_sheet(session_id=\"xl_1234567890\", action=\"rename\", name=\"Sheet1\", new_name=\"Summary\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action: 'create', 'delete', 'rename', 'copy', or 'list'",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name (required for create, delete, rename, copy)",
					},
					"new_name": map[string]interface{}{
						"type":        "string",
						"description": "New sheet name (required for rename and copy)",
					},
				},
				"required": []string{"meta", "session_id", "action"},
			},
			Callback: a.excelSheetTool,
		},
		{
			Name:        "excel_format",
			Description: "Apply formatting to a range of cells. Supports: font (name, size, bold, italic, underline, color), fill (background color), border (style, color, per-side control), alignment (horizontal, vertical, wrap text), number_format, merge/unmerge cells, row_height, col_width. Use the what parameter as an array to specify which operations to perform. Example: excel_format(session_id=\"xl_123\", sheet=\"Sheet1\", what=[\"font\",\"fill\",\"border\"], start_row=1, end_row=1, start_col=1, end_col=5, font_bold=true, fill_color=\"#4472C4\", border_style=\"thin\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by excel_open",
					},
					"sheet": map[string]interface{}{
						"type":        "string",
						"description": "Sheet name",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "'reset' (default) replaces all style properties; 'merge' only updates the properties in what[] and preserves existing styles. Example: mode=\"merge\"",
					},
					"what": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Array of operations to perform. Options: 'font', 'fill', 'border', 'alignment', 'number_format', 'merge', 'unmerge', 'row_height', 'col_width'. Example: [\"font\", \"fill\", \"border\"]",
					},
					"start_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based start row (default: 1)",
					},
					"end_row": map[string]interface{}{
						"type":        "number",
						"description": "1-based end row (default: start_row)",
					},
					"start_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based start column (default: 1)",
					},
					"end_col": map[string]interface{}{
						"type":        "number",
						"description": "1-based end column (default: start_col)",
					},
					// Font
					"font_name": map[string]interface{}{
						"type":        "string",
						"description": i18n.T(i18n.KeySettingCmd_614),
					},
					"font_size": map[string]interface{}{
						"type":        "number",
						"description": "Font size in points",
					},
					"font_bold": map[string]interface{}{
						"type":        "boolean",
						"description": "Bold text",
					},
					"font_italic": map[string]interface{}{
						"type":        "boolean",
						"description": "Italic text",
					},
					"font_underline": map[string]interface{}{
						"type":        "boolean",
						"description": "Underline text",
					},
					"font_color": map[string]interface{}{
						"type":        "string",
						"description": "Font color as hex RGB (e.g. '#FF0000')",
					},
					// Fill
					"fill_color": map[string]interface{}{
						"type":        "string",
						"description": "Background fill color as hex RGB (e.g. '#FFFFCC')",
					},
					// Border
					"border_style": map[string]interface{}{
						"type":        "string",
						"description": "Border style: 'thin', 'medium', 'thick', 'dashed', 'dotted', 'double'",
					},
					"border_color": map[string]interface{}{
						"type":        "string",
						"description": "Border color as hex RGB",
					},
					"border_top": map[string]interface{}{
						"type":        "boolean",
						"description": "Apply border to top (default: true when border is in what)",
					},
					"border_bottom": map[string]interface{}{
						"type":        "boolean",
						"description": "Apply border to bottom (default: true)",
					},
					"border_left": map[string]interface{}{
						"type":        "boolean",
						"description": "Apply border to left (default: true)",
					},
					"border_right": map[string]interface{}{
						"type":        "boolean",
						"description": "Apply border to right (default: true)",
					},
					// Alignment
					"h_align": map[string]interface{}{
						"type":        "string",
						"description": "Horizontal alignment: 'left', 'center', 'right', 'fill', 'justify'",
					},
					"v_align": map[string]interface{}{
						"type":        "string",
						"description": "Vertical alignment: 'top', 'center', 'bottom', 'justify'",
					},
					"wrap_text": map[string]interface{}{
						"type":        "boolean",
						"description": "Wrap text in cell",
					},
					// Number format
					"number_format": map[string]interface{}{
						"type":        "string",
						"description": "Number format string (e.g. '0.00', '#,##0', 'yyyy-mm-dd')",
					},
					// Row/Col
					"row_height": map[string]interface{}{
						"type":        "number",
						"description": "Row height in points (for row_height operation)",
					},
					"col_width": map[string]interface{}{
						"type":        "number",
						"description": "Column width in character units (for col_width operation)",
					},
				},
				"required": []string{"meta", "session_id", "sheet", "what"},
			},
			Callback: a.excelFormatTool,
		},
	}
	tools = append(tools, excelTools...)

	// Word tools (FEATURE-121)

	wordTools := []llm.Tool{
		{
			Name:        "word_open",
			Description: "Open a DOCX file and return a session ID for subsequent operations. mode is REQUIRED: 'create' (create new file, must not exist), 'read' (open existing file, read-only, save will fail), 'copy' (copy file to new name with timestamp before opening). Example: word_open(path=\"report.docx\", mode=\"read\") or word_open(path=\"new.docx\", mode=\"create\") or word_open(path=\"report.docx\", mode=\"copy\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the DOCX file to open",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "Open mode: 'create' (new empty file), 'read' (read-only), 'copy' (copy with timestamp, then edit copy)",
					},
				},
				"required": []string{"meta", "path", "mode"},
			},
			Callback: a.wordOpenTool,
		},
		{
			Name:        "word_close",
			Description: "Close a DOCX session (auto-saves if modified). Example: word_close(session_id=\"doc_1\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.wordCloseTool,
		},
		{
			Name:        "word_save",
			Description: "Save the DOCX file without closing the session. Example: word_save(session_id=\"doc_1\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.wordSaveTool,
		},
		{
			Name:        "word_overview",
			Description: "Get document structure overview: paragraph count, style usage (which styles used by how many paragraphs), and table count. Call this FIRST after opening to understand the document layout. Example: word_overview(session_id=\"doc_1\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
				},
				"required": []string{"meta", "session_id"},
			},
			Callback: a.wordOverviewTool,
		},
		{
			Name:        "word_read",
			Description: "Read a range of paragraphs in multiple output formats. format='simple' (default): HTML structural tags (h1/h2/h3/p/li) with 'N| ' line prefixes. format='full': HTML with inline CSS style attributes. format='text': plain text with 'N| ' line prefixes. format='md': Markdown format with 'N| ' line prefixes. Example: word_read(session_id=\"doc_1\", from_para=1, to_para=20, format=\"simple\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"from_para": map[string]interface{}{
						"type":        "number",
						"description": "1-based starting paragraph index",
					},
					"to_para": map[string]interface{}{
						"type":        "number",
						"description": "1-based ending paragraph index",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"simple", "full", "text", "md"},
						"description": "output format. Choose from: 'simple' (HTML with 'N| ' prefix), 'full' (HTML + CSS), 'text' (plain text with 'N| ' prefix), 'md' (Markdown with 'N| ' prefix)",
					},
				},
				"required": []string{"meta", "session_id", "from_para", "to_para", "format"},
			},
			Callback: a.wordReadTool,
		},
		{
			Name:        "word_table_read",
			Description: "Read a table and return it as HTML. format='simple' (default): plain table with colspan/rowspan attributes. format='full': includes cell background colors. Example: word_table_read(session_id=\"doc_1\", table_index=0)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"table_index": map[string]interface{}{
						"type":        "number",
						"description": "0-based table index (from word_overview)",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "'simple' (default) or 'full'",
					},
				},
				"required": []string{"meta", "session_id", "table_index"},
			},
			Callback: a.wordTableReadTool,
		},
		{
			Name:        "word_continue",
			Description: i18n.T(i18n.KeySettingCmd_615),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to insert. Supports Markdown: # Heading1, ## Heading2, ### Heading3, - list item. Use \\n for line breaks.",
					},
					"after_para": map[string]interface{}{
						"type":        "number",
						"description": "1-based paragraph number to insert content after. Use same_style_as to match paragraph numbering.",
					},
					"same_style_as": map[string]interface{}{
						"type":        "number",
						"description": "inherit style from this paragraph number. Overridden by explicit style parameter.",
					},
					"style": map[string]interface{}{
						"type":        "string",
						"description": "explicit style name like 'Heading1', 'Heading2', 'Normal'. Use word_inspect_style to see available styles. Takes precedence over same_style_as.",
					},
				},
				"required": []string{"meta", "session_id", "content"},
			},
			Callback: a.wordContinueTool,
		},
		{
			Name:        "word_erase",
			Description: "Delete a range of paragraphs. Example: word_erase(session_id=\"doc_1\", from_para=10, to_para=15)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"from_para": map[string]interface{}{
						"type":        "number",
						"description": "1-based starting paragraph index",
					},
					"to_para": map[string]interface{}{
						"type":        "number",
						"description": "1-based ending paragraph index",
					},
				},
				"required": []string{"meta", "session_id", "from_para", "to_para"},
			},
			Callback: a.wordEraseTool,
		},
		{
			Name:        "word_inspect_style",
			Description: "Inspect a named style definition. Returns style properties like font name, size, bold/italic, color, spacing, alignment. Use word_overview first to see available styles. Example: word_inspect_style(session_id=\"doc_1\", name=\"Heading 2\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Style name to inspect (e.g. 'Heading 1', 'Normal')",
					},
				},
				"required": []string{"meta", "session_id", "name"},
			},
			Callback: a.wordInspectStyleTool,
		},
		{
			Name:        "word_format",
			Description: "Change paragraph formatting. target='style:Heading1' affects all paragraphs with that style. target='para:3-5' affects paragraphs 3-5. what supports: 'style' (change style), 'font_name', 'font_size', 'bold', 'italic', 'color'. Example: word_format(session_id=\"doc_1\", target=\"style:Heading 2\", what=\"font_size\", value=\"14\")",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID returned by word_open",
					},
					"what": map[string]interface{}{
						"type":        "string",
						"description": "Property to change: 'style', 'font_name', 'font_size', 'bold', 'italic', 'color'",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "New value for the property",
					},
					"target": map[string]interface{}{
						"type":        "string",
						"description": "Target: 'style:StyleName' or 'para:start-end' (e.g., 'style:Heading 2', 'para:3-5')",
					},
				},
				"required": []string{"meta", "session_id", "what", "value", "target"},
			},
			Callback: a.wordFormatTool,
		},
	}
	tools = append(tools, wordTools...)

	// Add introspect_capability tool (FEATURE-466) — meta-capability awareness.
	// Available when meta-capability awareness is enabled.
	if a.metaCapabilityEnabled {
		tools = append(tools, llm.Tool{
			Name:        "introspect_capability",
			Description: "Query co-shell's native meta-capabilities (self-modification, model routing, problem-solving strategies, sub-agent collaboration, context management, self-configuration). Pass a capability ID to get the full instructions, or pass a keyword array for multi-condition fuzzy search. With no arguments, returns the full capability index.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
					},
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The capability ID to query (e.g. cap.self-modify). When provided, returns the full detail for that capability.",
					},
					"keywords": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Fuzzy search keywords (multi-condition AND match against ID/name/description/category).",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.introspectCapabilityTool,
		})
	}

	// Add MCP tools
	for _, mcpTool := range a.mcpMgr.GetAllTools() {
		tool := mcpTool // capture
		mcpLLMTool := llm.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			// FIX-465: deep-copy the InputSchema so injectMetaParam does not
			// mutate the MCP manager's shared schema map. buildTools runs on
			// every LLM call; without a copy, repeated injectMetaParam calls
			// would accumulate duplicate "meta" entries in required.
			Parameters: deepCopyMap(tool.InputSchema),
			Callback: func(ctx context.Context, args map[string]interface{}) (string, error) {
				return a.mcpMgr.CallTool(ctx, tool.Name, args)
			},
		}
		// FEATURE-447: MCP tool InputSchema comes from the MCP server and has no
		// meta parameter — inject it so MCP tools also carry the required
		// transparency metadata.
		injectMetaParam(&mcpLLMTool)
		tools = append(tools, mcpLLMTool)
	}

	// FEATURE-490: board collaboration tools (only when the board switch is on).
	if a.BoardEnabled() {
		tools = append(tools, llm.Tool{
			Name:        "board_result",
			Description: "Report the result of a hub bulletin-board task back to the hub (FEATURE-490). Call this after executing a board_task to deliver the outcome to the requester.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "The task id from the board_task message"},
					"result":  map[string]interface{}{"type": "string", "description": "The result text to return to the requester"},
				},
				"required": []string{"task_id", "result"},
			},
			Callback: a.boardResultTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "board_post",
			Description: "Publish a help request to the hub bulletin board (FEATURE-490). Other agents whose role matches may claim it.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":         map[string]interface{}{"type": "string", "description": "Short title of the request"},
					"description":   map[string]interface{}{"type": "string", "description": "Detailed description of what help is needed"},
					"required_role": map[string]interface{}{"type": "string", "description": "Optional role that should respond"},
				},
				"required": []string{"title", "description"},
			},
			Callback: a.boardPostTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "board_list",
			Description: "Poll the hub bulletin board for open help requests (FEATURE-490). Use this to discover requests relevant to your role.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			Callback: a.boardListTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "board_claim",
			Description: "Claim an open help request on the hub bulletin board (FEATURE-490). Only the requester and the assignee can then discuss via board_dm.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request_id": map[string]interface{}{"type": "string", "description": "The request id to claim"},
				},
				"required": []string{"request_id"},
			},
			Callback: a.boardClaimTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "board_dm",
			Description: "Send a direct message to the other participant of a board request thread (FEATURE-490). Use to clarify requirements before execution.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request_id": map[string]interface{}{"type": "string", "description": "The request id"},
					"content":    map[string]interface{}{"type": "string", "description": "The message content"},
				},
				"required": []string{"request_id", "content"},
			},
			Callback: a.boardDMTool,
		})
		tools = append(tools, llm.Tool{
			Name:        "board_confirm",
			Description: "Confirm a claimed board request to start execution (FEATURE-490). Only the requester can confirm; the assignee then receives a board_task.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request_id": map[string]interface{}{"type": "string", "description": "The request id to confirm"},
				},
				"required": []string{"request_id"},
			},
			Callback: a.boardConfirmTool,
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

	// FEATURE-447: the unified meta object parameter is declared directly in
	// each tool definition above. MCP tools (whose InputSchema comes from the
	// MCP server) get the meta parameter injected at build time so they also
	// carry the required transparency metadata.

	// FEATURE-447: when intent exposure is disabled, strip the meta parameter
	// from every tool definition (properties + required) so the LLM is not
	// asked to report transparency metadata.
	if !a.intentExposureEnabled {
		for i := range tools {
			params := tools[i].Parameters
			if props, ok := params["properties"].(map[string]interface{}); ok {
				delete(props, "meta")
			}
			// required may be declared as []string or []interface{}; handle both.
			switch req := params["required"].(type) {
			case []string:
				filtered := req[:0]
				for _, r := range req {
					if r != "meta" {
						filtered = append(filtered, r)
					}
				}
				params["required"] = filtered
			case []interface{}:
				filtered := req[:0]
				for _, r := range req {
					if r != "meta" {
						filtered = append(filtered, r)
					}
				}
				params["required"] = filtered
			}
		}
	}

	return tools
}

// injectMetaParam adds the unified meta object parameter to a tool definition
// (FEATURE-447). It inserts meta as the first property, marks it required, and
// removes the legacy top-level intent parameter (now inside meta.intent).
// Vision tools (visual_analysis / browser_screenshot) additionally get an
// "instruct" parameter that carries the explicit instruction for the vision
// model (distinct from meta.intent which is for the user).
func injectMetaParam(tool *llm.Tool) {
	props, _ := tool.Parameters["properties"].(map[string]interface{})
	if props == nil {
		props = map[string]interface{}{}
		tool.Parameters["properties"] = props
	}

	// Remove the legacy top-level intent parameter (moved into meta.intent).
	delete(props, "intent")

	// Add the meta object parameter.
	props["meta"] = map[string]interface{}{
		"type":        "object",
		"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
	}

	// Vision tools get an explicit instruct parameter for the vision model.
	if tool.Name == "visual_analysis" || tool.Name == "browser_screenshot" {
		props["instruct"] = map[string]interface{}{
			"type":        "string",
			"description": "The explicit instruction for the vision model describing what to analyze/extract from the image(s). This is distinct from meta.intent (which is the intent shown to the user).",
		}
	}

	// Update the required list: remove intent, add meta (and instruct for
	// vision tools). The required field may be []interface{} (built-in tools)
	// or []string (MCP tools, whose schema comes from the MCP server) — handle
	// both so MCP tools keep their original required fields (FIX-465).
	// meta is deduplicated so repeated injectMetaParam calls (buildTools runs
	// on every LLM call and shares the MCP tool's InputSchema map) do not
	// accumulate duplicate "meta" entries, which the LLM API rejects as
	// non-unique (FIX-465).
	newRequired := make([]interface{}, 0, 4)
	hasMeta := false
	hasInstruct := false
	switch req := tool.Parameters["required"].(type) {
	case []interface{}:
		for _, r := range req {
			s, _ := r.(string)
			if s == "intent" {
				continue // intent moved into meta
			}
			if s == "meta" {
				hasMeta = true
				continue
			}
			if s == "instruct" {
				hasInstruct = true
				continue
			}
			newRequired = append(newRequired, r)
		}
	case []string:
		for _, s := range req {
			if s == "intent" {
				continue // intent moved into meta
			}
			if s == "meta" {
				hasMeta = true
				continue
			}
			if s == "instruct" {
				hasInstruct = true
				continue
			}
			newRequired = append(newRequired, s)
		}
	}
	// meta is always required and placed first (deduplicated).
	if !hasMeta {
		newRequired = append([]interface{}{"meta"}, newRequired...)
	}
	if (tool.Name == "visual_analysis" || tool.Name == "browser_screenshot") && !hasInstruct {
		newRequired = append(newRequired, "instruct")
	}
	tool.Parameters["required"] = newRequired
}

// deepCopyMap returns a deep copy of a map[string]interface{} so that mutating
// the copy (e.g. injectMetaParam adding meta) does not affect the original
// (FIX-465). Nested maps and slices are copied recursively.
func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = deepCopyValue(v)
	}
	return dst
}

// deepCopyValue returns a deep copy of an arbitrary value, recursing into maps
// and slices (FIX-465).
func deepCopyValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	default:
		return v
	}
}

// recordVisionToolCall stores the ToolCall ID and tool name of the most
// recent vision tool call (visual_analysis / browser_screenshot) so the
// vision recognition round can backfill its result with the correct
// tool_call_id (OpenAI mode) and tool name (XML mode). No-op for other tools
// (FEATURE-343 + FEATURE-346).
func (a *Agent) recordVisionToolCall(tc llm.ToolCall) {
	if (tc.Name != "visual_analysis" && tc.Name != "browser_screenshot") || tc.ID == "" {
		return
	}
	a.mu.Lock()
	a.lastVisionToolCallID = tc.ID
	a.lastVisionToolCallName = tc.Name
	a.mu.Unlock()
}

// shouldDeferVisionToolResult reports whether the placeholder tool result of a
// vision tool call should NOT be appended to a.messages: in FEATURE-343
// minimal vision-context mode the recognition round that follows backfills the
// recognition output as this tool call's ONLY tool message. Writing the
// placeholder too would leave two tool messages sharing one tool_call_id,
// which strict providers reject (400 "Messages with role 'tool' must be a
// response to a preceding message with 'tool_calls'").
// The conditions mirror the recognition-round trigger in buildContextMessages
// exactly; if they diverge, the assistant tool_calls message would be left
// without any tool response — an equally invalid history.
// Note: with multiple vision tool calls in a single assistant turn only the
// last one is backfilled (lastVisionToolCallID) — same limitation as the
// backfill itself.
func (a *Agent) shouldDeferVisionToolResult(toolName string) bool {
	if toolName != "visual_analysis" && toolName != "browser_screenshot" {
		return false
	}
	if a.cfg == nil || a.cfg.LLM.VisionContextMode != "minimal" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.visionPendingIntent != "" && len(a.imagePaths) > 0
}

// executeToolCall runs a single tool call and returns the result.
func (a *Agent) executeToolCall(ctx context.Context, tc llm.ToolCall) (string, error) {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return "", fmt.Errorf("cannot parse tool arguments: %w", err)
	}

	// FIX-451: tolerate LLMs that mistakenly nest tool parameters (e.g. path,
	// regex, command) inside the meta object instead of at the top level. The
	// meta object only legitimately holds intent/risk/risk_reason/
	// affected_objects/progress; any other key found inside it is a misplaced
	// tool parameter and is promoted to the top level so the tool can run.
	promoteMisplacedMetaParams(args)

	// Check for vault placeholders and optionally mask them for confirmation display.
	// Placeholders like @pwd:entry_name, @user:entry_name need the vault to be unlocked.
	hasPlaceholders := store.HasPlaceholders(args)
	if hasPlaceholders && a.vaultStore != nil {
		// Check if vault is unlocked, try to resolve for confirmation display
		if !a.vaultStore.IsUnlocked() {
			return "", fmt.Errorf("vault is locked - cannot resolve placeholders. Use :vault unlock first")
		}
	}

	// Determine if confirmation is needed for this tool call.
	// Check per-tool mode: first check specific tool, then "default", then default to "confirm".
	// Tools with vault placeholders ALWAYS require confirmation.
	mode := "confirm"
	if a.toolModes != nil {
		if v, ok := a.toolModes[tc.Name]; ok {
			mode = v
		} else if v, ok := a.toolModes["default"]; ok {
			mode = v
		}
	}
	// YOLO master switch (FEATURE-439): when enabled, every tool call that
	// reaches the confirmation entry is auto-approved and executed without
	// asking the user. Disabled tools are not sent to the LLM at all, so they
	// are unaffected by YOLO.
	needsConfirm := mode == "confirm" && !a.yoloMode

	if needsConfirm {
		// Skip confirmation if:
		// - User chose "approve all" for this request, OR
		// - User disabled confirmation for this specific tool (G option), OR
		// - There are remaining auto-approve counts for this tool
		toolCount := a.toolApproveCounts[tc.Name]
		if !a.approveAll && !a.toolDisableConfirm[tc.Name] && toolCount <= 0 {
			// Build a human-readable summary (friendly name + intent + key params)
			// so the user can grasp the impact of the call before confirming (FEATURE-310).
			displayStr := buildToolSummary(tc.Name, args).Text
			result, modifyInput := promptToolConfirmation(tc.Name, displayStr, a.interactionManager())
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
				// Store the user's supplementary input in the task instruction cache.
				// At the end of the iteration, all cached instructions will be flushed
				// as a single <task> ContentPart appended to the last user message.
				// This separates user instructions from tool results, keeping the
				// tool result clean and the instructions visible as a distinct task block.
				if a.taskInstructionCache.Len() > 0 {
					a.taskInstructionCache.WriteString("\n\n")
				}
				a.taskInstructionCache.WriteString(modifyInput)
				return i18n.T(i18n.KeySettingCmd_592), nil
			}
			// CmdConfirmApprove: continue execution
		} else if toolCount > 0 {
			// Decrement per-tool approve count and auto-approve
			a.toolApproveCounts[tc.Name]--
			remaining := a.toolApproveCounts[tc.Name]
			a.defaultIO().Println(fmt.Sprintf(i18n.T(i18n.KeySettingCmd_593), tc.Name, remaining))
		}
	}

	// Find and execute the tool
	// Use buildToolsInternal() instead of buildTools() because we need the full
	// tool list including callbacks, regardless of the current tool call mode.
	// buildTools() may return an empty list in XML mode (where tools are described
	// in the system prompt rather than sent as API parameters), but we still need
	// the tool callbacks to execute the tool.
	// Resolve vault placeholders before executing the tool.
	// This decrypts @pwd:, @user:, @vault: placeholders and injects real values
	// into the arguments. The placeholders are resolved after user confirmation
	// but before the tool callback, ensuring credentials never reach the LLM.
	if hasPlaceholders && a.vaultStore != nil {
		result, _ := a.vaultStore.ResolveVaultPlaceholders(args)
		if len(result.MissingEntries) > 0 {
			io := a.defaultIO()
			for entryName, missingTags := range result.MissingEntries {
				missingTagStr := strings.Join(missingTags, ", ")
				io.Printf(i18n.T(i18n.KeySettingCmd_594), entryName, missingTagStr)
				io.Println(i18n.T(i18n.KeySettingCmd_595))
				io.Printf(i18n.T(i18n.KeySettingCmd_596), entryName)
				io.Println(i18n.T(i18n.KeySettingCmd_597))
				io.Printf(i18n.T(i18n.KeySettingCmd_598))

				response, err := io.ReadLine()
				if err != nil {
					return "", fmt.Errorf("failed to read user input: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))

				if response == "a" || response == "" {
					tags := make(map[string]string)
					for _, tag := range missingTags {
						io.Printf(i18n.T(i18n.KeySettingCmd_599), tag, entryName)
						val, err := io.ReadLine()
						if err != nil {
							return "", fmt.Errorf("failed to read value for tag %q: %w", tag, err)
						}
						val = strings.TrimSpace(val)
						if val == "" {
							return "", fmt.Errorf("tag %q cannot be empty", tag)
						}
						tags[tag] = val
					}

					entry := &store.VaultEntry{
						Name: entryName,
						Tags: tags,
					}
					if err := a.vaultStore.Put(entry); err != nil {
						return "", fmt.Errorf("cannot save vault entry %q: %w", entryName, err)
					}
					io.Printf(i18n.T(i18n.KeySettingCmd_600), entryName)
				} else {
					return "", fmt.Errorf("user cancelled tool call because vault entry %q does not exist", entryName)
				}
			}
			a.vaultStore.ResolveVaultPlaceholders(args)
		}
	}

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

			// FEATURE-280: Check if the user pressed Ctrl+C before executing the tool.
			// This catches cancellation between iterations when the tool is confirmed
			// but Ctrl+C was pressed during the confirmation prompt.
			select {
			case <-ctx.Done():
				log.Info("Tool call cancelled: %s (context canceled)", tc.Name)
				return "", ctx.Err()
			default:
			}

			log.Info("Tool call: %s, effective timeout=%s (user min: %ds, LLM suggested: %ds), args=%v",
				tc.Name, timeoutStr, userMinSec, llmSuggested, args)

			// FEATURE-343 + FEATURE-346: record the ToolCall ID and tool name of
			// the most recent vision tool call (visual_analysis /
			// browser_screenshot) so the vision recognition round can backfill
			// its result with the correct tool_call_id (OpenAI mode) and tool
			// name (XML mode).
			a.recordVisionToolCall(tc)

			result, err := tool.Callback(ctx, args)
			if err != nil {
				log.Error("Tool call failed: %s, error: %v", tc.Name, err)
				return "", err
			}
			log.Debug("Tool call result: %s -> %s", tc.Name, result)

			// If the tool was called with an intent in its meta object, wrap the
			// result with explicit labels (FIX-324). The result is clearly
			// marked with [Result] so that an empty result cannot be confused
			// with the [Intent] reminder text — the LLM sees exactly what the
			// tool returned versus what the original purpose was. This applies
			// to both OpenAI (tool message) and XML (tool result template)
			// modes, which share this same return value.
			if intent := argString(metaObject(args), "intent"); intent != "" {
				if result == "" {
					result = i18n.T(i18n.KeyToolNoOutput)
				}
				result = fmt.Sprintf("%s%s\n\n%s %s",
					i18n.T(i18n.KeyToolResultLabel), result,
					i18n.T(i18n.KeyToolIntentLabel), intent)
			}

			return result, nil
		}
	}

	return "", fmt.Errorf("tool %q not found", tc.Name)
}

// storeUserReply writes a user reply into the task instruction cache, separated
// from any prior content by a blank line.
func (a *Agent) storeUserReply(content string) {
	if a.taskInstructionCache.Len() > 0 {
		a.taskInstructionCache.WriteString("\n\n")
	}
	a.taskInstructionCache.WriteString(content)
}

// attemptCompletionTool presents the final result to the user, optionally executing a demo command.
func (a *Agent) attemptCompletionTool(ctx context.Context, args map[string]interface{}) (string, error) {
	result, _ := args["result"].(string)
	if result == "" {
		return "", fmt.Errorf("result is required")
	}

	command, _ := args["command"].(string)

	// Validate session_title and session_keywords (FEATURE-276)
	sessionTitle, _ := args["session_title"].(string)
	sessionKeywords, _ := args["session_keywords"].(string)
	if sessionTitle == "" {
		return "", fmt.Errorf("session_title is required — provide a brief session title (≤60 chars)")
	}
	if sessionKeywords == "" {
		return "", fmt.Errorf("session_keywords is required — provide comma-separated keywords for this session")
	}

	// FEATURE-456: supervisor review at intervention point A (attempt_completion).
	// Synchronously ask the supervisor whether the delivery meets the user's
	// ultimate goal. If rejected, return the feedback as a user message so the
	// main LLM reworks and re-delivers (the loop continues).
	approved, feedback, report := a.runSupervisorReview(ctx, SupervisorEntryA, result)
	if !approved {
		// Rejected: append feedback as a user message to the main LLM context
		// and continue the loop for rework.
		a.storeUserReply(feedback)
		return feedback, nil
	}
	if report != "" {
		// Approved: report the supervisor's reason+suggestion to the user via
		// the dedicated supervisor channel (FEATURE-456).
		a.emitSupervisorReport(report)
	}

	// FEATURE-479: completion-confirm behavior mode (active/simple/exit).
	//   active — show the full confirm dialog (LLM next_steps + next-step
	//            suggestion + cancel + supplement + complete).
	//   simple — show only two fixed options: confirm task complete, or
	//            continue with more input (no LLM-collected questions).
	//   exit   — complete directly without showing any confirm dialog.
	// When CompletionMode is empty, fall back to AttemptCompletionConfirm
	// (true=active, false=exit) for backward compatibility (FEATURE-452).
	mode := "simple"
	if a.cfg != nil {
		mode = a.cfg.LLM.CompletionMode
		if mode == "" {
			if a.cfg.LLM.AttemptCompletionConfirm {
				mode = "simple"
			} else {
				mode = "exit"
			}
		}
	}
	confirmed := true
	if mode == "active" {
		var options []string
		if ns, ok := args["next_steps"].([]interface{}); ok {
			for _, s := range ns {
				if str, ok := s.(string); ok && str != "" {
					options = append(options, str)
				}
			}
		}
		options = append(options, i18n.T(i18n.KeyAttemptCompletionSuggestNext))
		// FEATURE-459: the dialog body shows the main LLM's final delivery report
		// (result) first, then the supervisor's conclusion/reason/suggestion (if
		// any) below it, so the human reviewer sees the full picture.
		body := "【任务完成报告】\n" + result
		if report != "" {
			body += "\n\n" + report
		}
		in := Interaction{
			Kind:    InteractionSelect,
			Title:   i18n.T(i18n.KeyAttemptCompletionPrompt),
			Body:    body,
			Options: options,
			Keys: []KeyOption{
				{Label: i18n.T(i18n.KeyAttemptCompletionNotDone), Key: "+", Value: "not_done"},
				{Label: i18n.T(i18n.KeyAttemptCompletionExit), Key: "-", Value: "exit"},
			},
		}
		res, err := a.interactionManager().Ask(ctx, in)
		if err != nil {
			return "", fmt.Errorf("failed to read user input: %w", err)
		}
		switch res.Action {
		case ActionCancel:
			return "", fmt.Errorf("CANCEL_AGENT")
		case ActionSelect:
			if res.Value == "exit" {
				confirmed = true
			} else {
				confirmed = false
				a.storeUserReply(res.Value)
			}
		case ActionInput:
			confirmed = false
			a.storeUserReply(res.Value)
		default:
			confirmed = true
		}
	} else if mode == "simple" {
		// FEATURE-479: simple mode shows only two fixed options — confirm the
		// task is complete (exit), or continue with more input (the user's
		// typed reply is sent back to the LLM so the loop continues). No
		// LLM-collected next_steps questions are shown.
		body := "【任务完成报告】\n" + result
		if report != "" {
			body += "\n\n" + report
		}
		in := Interaction{
			Kind:  InteractionSelect,
			Title: i18n.T(i18n.KeyAttemptCompletionPrompt),
			Body:  body,
			// FEATURE-479: simple mode carries only the two fixed key actions
			// (confirm complete / continue with more input) as Keys, with no
			// Options. The Web frontend renders a keys-only select as the two
			// [Key] Label buttons (mirroring the TUI askSelect), so exactly two
			// options are shown.
			Keys: []KeyOption{
				{Label: i18n.T(i18n.KeyAttemptCompletionSimpleConfirm), Key: "-", Value: "exit"},
				{Label: i18n.T(i18n.KeyAttemptCompletionSimpleContinue), Key: "+", Value: "continue"},
			},
		}
		res, err := a.interactionManager().Ask(ctx, in)
		if err != nil {
			return "", fmt.Errorf("failed to read user input: %w", err)
		}
		switch res.Action {
		case ActionCancel:
			return "", fmt.Errorf("CANCEL_AGENT")
		case ActionSelect:
			if res.Value == "exit" {
				confirmed = true
			} else {
				// continue: prompt the user for more input, then send it back.
				confirmed = false
				contIn := Interaction{
					Kind:  InteractionInput,
					Title: i18n.T(i18n.KeyAttemptCompletionSimpleContinuePrompt),
				}
				contRes, cerr := a.interactionManager().Ask(ctx, contIn)
				if cerr != nil {
					return "", fmt.Errorf("failed to read user input: %w", cerr)
				}
				a.storeUserReply(contRes.Value)
			}
		case ActionInput:
			confirmed = false
			a.storeUserReply(res.Value)
		default:
			confirmed = true
		}
	}
	// mode == "exit": confirmed stays true, complete directly without a dialog.

	if !confirmed {
		// User chose to continue — send the choice back to the LLM and keep looping.
		return i18n.T(i18n.KeyAttemptCompletionContinue), nil
	}

	// Handle task_message_no for context pointer adjustment.
	// In "reorganize" mode, task_message_no has no effect.
	// In other modes, it is optional.
	contextPolicy := "smart"
	if a.cfg != nil && a.cfg.LLM.ContextPolicy != "" {
		contextPolicy = a.cfg.LLM.ContextPolicy
	}
	if contextPolicy == "reorganize" {
		log.Info("attemptCompletion: reorganize mode active, ignoring task_message_no")
	} else if taskMsgNoRaw, ok := args["task_message_no"].(float64); ok {
		taskMsgNo := int(taskMsgNoRaw)
		if taskMsgNo >= 0 {
			a.mu.Lock()
			if taskMsgNo < len(a.messages) {
				a.messagePointer = taskMsgNo
				a.needAdjustPointer = true
				log.Info("attemptCompletion: context pointer adjusted to message %d (from task_message_no)", taskMsgNo)
			}
			a.mu.Unlock()
		}
	}

	// Update current session with LLM-provided title and keywords (not create new).
	// FEATURE-500: when the current session title is locked (starts with "$"),
	// keep the manual title and only update the keywords.
	finalTitle := sessionTitle
	if id := a.CurrentSessionID(); id != "" {
		if entry, found, err := a.store.LoadNamedSession(id); err == nil && found && entry != nil && strings.HasPrefix(entry.Title, "$") {
			finalTitle = entry.Title
		}
	}
	if err := a.UpdateCurrentSession(finalTitle, sessionKeywords); err != nil {
		log.Warn("attemptCompletion: failed to update session: %v", err)
	} else {
		log.Info("attemptCompletion: session updated with title=%q, keywords=%q", finalTitle, sessionKeywords)
	}

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
			cmdOutput = fmt.Sprintf(i18n.T(i18n.KeySettingCmd_610), err, decoded)
		} else {
			cmdOutput = fmt.Sprintf(i18n.T(i18n.KeySettingCmd_611), strings.TrimSpace(decoded))
		}
	}

	// Mark the task as completed so RunStream knows this is the final answer
	a.SetCompleted()

	// Build the final completion message
	message := fmt.Sprintf(i18n.T(i18n.KeySettingCmd_612), result)
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
