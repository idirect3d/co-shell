// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
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

// FEATURE-556 Phase 3: declarations of the additional browser tools (console,
// upload, PDF, network, tabs, mocking). They are registered together with the
// other browser tools whenever the browser is enabled.

package agent

import "github.com/idirect3d/co-shell/llm"

// browserToolsP3 returns the browser tools of FEATURE-556 Phase 3.
func (a *Agent) browserToolsP3() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "browser_console",
			Description: "Report the browser console messages (JavaScript console output, uncaught exceptions and browser log entries) collected since the previous call. Capture is enabled on the first call and Chrome replays the messages the page produced before that, like DevTools does. Parameters: clear (optional, boolean: drop the collected messages and return nothing), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"clear": map[string]interface{}{
						"type":        "boolean",
						"description": "When true, discard the collected messages instead of returning them (default: false)",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.browserConsoleTool,
		},
		{
			Name:        "browser_upload",
			Description: "Attach files to a file input matched by a CSS selector (DOM.setFileInputFiles), which is the only way to fill a file input without a native file dialog. Paths are resolved inside the workspace and must exist; relative paths are workspace-relative. Parameters: selector (required, CSS selector of the file input), files (required, array of workspace-relative or absolute file paths), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector of the file input element, e.g. input[type=file]",
					},
					"files": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Files to upload, workspace-relative or absolute; the order is kept",
					},
				},
				"required": []string{"meta", "selector", "files"},
			},
			Callback: a.browserUploadTool,
		},
		{
			Name:        "browser_pdf",
			Description: "Export the current page as PDF (Page.printToPDF) and save it to disk. Parameters: path (optional output path; relative paths are workspace-relative, default <workspace>/download/browser/<page title>.pdf), landscape (optional boolean), print_background (optional boolean, default true), paper_width and paper_height (optional, inches), scale (optional, 0.1-2), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Output file path (workspace-relative when not absolute); defaults to <workspace>/download/browser/<page title>.pdf",
					},
					"landscape": map[string]interface{}{
						"type":        "boolean",
						"description": "Print in landscape orientation (default: false)",
					},
					"print_background": map[string]interface{}{
						"type":        "boolean",
						"description": "Print CSS backgrounds (default: true)",
					},
					"paper_width": map[string]interface{}{
						"type":        "number",
						"description": "Paper width in inches (Chrome default when omitted)",
					},
					"paper_height": map[string]interface{}{
						"type":        "number",
						"description": "Paper height in inches (Chrome default when omitted)",
					},
					"scale": map[string]interface{}{
						"type":        "number",
						"description": "Page scale, 0.1 to 2 (Chrome default when omitted)",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.browserPdfTool,
		},
		{
			Name:        "browser_network",
			Description: "Report the network requests observed since observation was enabled (status, MIME type, resource type and failure reason are included). Observation is enabled on the first call and CDP does not replay past events. Parameters: only_failures (optional boolean), limit (optional, max entries to list, default 20, max 100), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"only_failures": map[string]interface{}{
						"type":        "boolean",
						"description": "Only report failed requests (default: false)",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of entries to list (default 20, maximum 100)",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.browserNetworkTool,
		},
		{
			Name:        "browser_tab",
			Description: "Manage browser tabs: list them, open a new one, close one, or switch the session to another tab. Switching reconnects the CDP session to the selected page, so all later tools operate on that tab. Parameters: action (required: list, open, close, switch), url (optional, for open), index (optional, 1-based tab index for switch/close), target_id (optional target id for switch/close), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Tab action: list, open, close or switch",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL of the new tab (action=open; about:blank when omitted)",
					},
					"index": map[string]interface{}{
						"type":        "number",
						"description": "1-based tab index from action=list (used by switch/close)",
					},
					"target_id": map[string]interface{}{
						"type":        "string",
						"description": "Target id from action=list (used by switch/close)",
					},
				},
				"required": []string{"meta", "action"},
			},
			Callback: a.browserTabTool,
		},
		{
			Name:        "browser_mock",
			Description: "Mock network requests with in-memory rules built on the Fetch domain: a rule answers a matching request with a canned response (fulfill), fails it, or lets it through. Rules match the request URL as a substring, or as a glob when glob is set ('*' matches any characters including '/'). Interception is only enabled while at least one rule exists. Parameters: action (required: add, list, clear, off), match (required for add), glob (optional), rule_action (optional: fulfill, fail, continue; default fulfill, or fail when error_reason is given), status (optional, default 200), headers (optional object), body (optional string), base64_body (optional), error_reason (optional, for fail), meta (required).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Mock action: add, list, clear or off",
					},
					"match": map[string]interface{}{
						"type":        "string",
						"description": "URL substring (or glob pattern when glob=true) the rule applies to",
					},
					"glob": map[string]interface{}{
						"type":        "boolean",
						"description": "Treat match as a glob pattern ('*' and '?') instead of a substring (default: false)",
					},
					"rule_action": map[string]interface{}{
						"type":        "string",
						"description": "How to answer the matched request: fulfill, fail or continue (default: fulfill)",
					},
					"status": map[string]interface{}{
						"type":        "number",
						"description": "Response status code for fulfill (default 200)",
					},
					"headers": map[string]interface{}{
						"type":        "object",
						"description": "Response headers for fulfill, e.g. {\"Content-Type\":\"text/plain\"}",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Response body for fulfill (text, or base64 when base64_body=true)",
					},
					"base64_body": map[string]interface{}{
						"type":        "boolean",
						"description": "The body is already base64 encoded (default: false)",
					},
					"error_reason": map[string]interface{}{
						"type":        "string",
						"description": "Failure reason for fail, e.g. Failed, Aborted, NameNotResolved",
					},
				},
				"required": []string{"meta", "action"},
			},
			Callback: a.browserMockTool,
		},
	}
}
