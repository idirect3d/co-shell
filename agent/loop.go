// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/subagent"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	defaultMaxIterations = 10
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

// decodeToUTF8 converts GBK encoded bytes to UTF-8 string on Windows.
// On non-Windows platforms, it returns the raw string as-is.
func decodeToUTF8(data []byte) string {
	if runtime.GOOS != "windows" {
		return string(data)
	}
	// Try GBK decode first; if it fails, return raw string
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}

// StreamCallback is a function called for each streaming event from the LLM.
type StreamCallback func(eventType string, content string)

// CmdConfirmResult represents the result of a command confirmation prompt.
type CmdConfirmResult int

const (
	CmdConfirmApprove    CmdConfirmResult = iota
	CmdConfirmApproveAll                  // Approve all commands for this request
	CmdConfirmCancel                      // User cancelled, return to REPL
	CmdConfirmModify                      // User entered custom input to modify the command
)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	mu             sync.Mutex
	llmClient      llm.Client
	mcpMgr         *mcp.Manager
	store          *store.Store
	systemPrompt   string
	messages       []llm.Message
	showThinking   bool
	showCommand    bool
	showOutput     bool
	maxIterations  int
	confirmCommand bool
	approveAll     bool           // if true, skip confirmation for all commands in this request
	cfg            *config.Config // configuration for timeout settings
	resultMode     config.ResultMode
	rules          string // user-defined rules for rebuilding system prompt
	subAgentMgr    *subagent.Manager
	scheduler      *scheduler.Scheduler
	name           string   // agent name for identification (default: "co-shell")
	imagePaths     []string // paths to image files for multimodal input
}

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.Store, rules string) *Agent {
	systemPrompt := buildSystemPrompt(rules)

	return &Agent{
		llmClient:     llmClient,
		mcpMgr:        mcpMgr,
		store:         s,
		systemPrompt:  systemPrompt,
		maxIterations: defaultMaxIterations,
		rules:         rules,
		subAgentMgr:   subagent.NewManager(),
		name:          "co-shell",
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// SetName sets the agent name for identification.
// The name is used in log messages, sub-agent workspace naming, and output.
func (a *Agent) SetName(name string) {
	if name == "" {
		name = "co-shell"
	}
	a.name = name
}

// Name returns the agent name.
func (a *Agent) Name() string {
	return a.name
}

// Said returns a formatted string with timestamp and agent name.
// Format: "2026-12-31 15:30:10 co-shell said:"
func (a *Agent) Said() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	return i18n.TF(i18n.KeyAgentSaid, now, a.name)
}

// SetShowThinking sets whether to display thinking process.
func (a *Agent) SetShowThinking(show bool) {
	a.showThinking = show
}

// SetShowCommand sets whether to display commands before execution.
func (a *Agent) SetShowCommand(show bool) {
	a.showCommand = show
}

// SetShowOutput sets whether to display command output before LLM analysis.
func (a *Agent) SetShowOutput(show bool) {
	a.showOutput = show
}

// SetMaxIterations sets the maximum number of LLM call iterations.
// n <= 0 means unlimited; n > 0 sets a specific limit.
func (a *Agent) SetMaxIterations(n int) {
	if n <= 0 {
		a.maxIterations = -1 // unlimited
	} else {
		a.maxIterations = n
	}
}

// SetConfirmCommand sets whether to prompt the user for confirmation before executing commands.
func (a *Agent) SetConfirmCommand(confirm bool) {
	a.confirmCommand = confirm
}

// SetConfig sets the configuration for timeout settings and agent identity.
// It also rebuilds the system prompt with identity information.
func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
	// Rebuild system prompt with identity info from config
	a.rebuildSystemPrompt()
}

// rebuildSystemPrompt rebuilds the system prompt with current config identity info.
func (a *Agent) rebuildSystemPrompt() {
	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
	}
	a.systemPrompt = buildSystemPromptWithMode(a.rules, a.resultMode, agentName, agentDesc, agentPrinciples)
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
}

// SetImagePaths sets the paths to image files for multimodal input.
// These images will be included in the next user message.
func (a *Agent) SetImagePaths(paths []string) {
	a.imagePaths = paths
}

// buildMultimodalMessage creates a Message with multimodal content from text and image paths.
// Images are read from disk and encoded as base64 data URIs.
func (a *Agent) buildMultimodalMessage(text string, imagePaths []string) (llm.Message, error) {
	parts := make([]llm.ContentPart, 0, 1+len(imagePaths))

	// Add text part
	parts = append(parts, llm.ContentPart{
		Type: llm.ContentPartText,
		Text: text,
	})

	// Add image parts
	for _, imgPath := range imagePaths {
		// Resolve relative paths
		absPath := imgPath
		if !filepath.IsAbs(imgPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return llm.Message{}, fmt.Errorf("cannot get current working directory: %w", err)
			}
			absPath = filepath.Join(cwd, imgPath)
		}

		// Read image file
		data, err := os.ReadFile(absPath)
		if err != nil {
			return llm.Message{}, fmt.Errorf("cannot read image %q: %w", imgPath, err)
		}

		// Detect MIME type from extension
		ext := strings.ToLower(filepath.Ext(absPath))
		mimeType := ""
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".bmp":
			mimeType = "image/bmp"
		default:
			mimeType = "image/png" // default fallback
		}

		// Encode as base64 data URI
		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		parts = append(parts, llm.ContentPart{
			Type: llm.ContentPartImageURL,
			ImageURL: &llm.ContentPartImage{
				URL:    dataURI,
				Detail: "auto",
			},
		})
	}

	return llm.Message{
		Role:         "user",
		Content:      text,
		ContentParts: parts,
	}, nil
}

// SetScheduler sets the scheduler for this agent.
func (a *Agent) SetScheduler(s *scheduler.Scheduler) {
	a.scheduler = s
}

// Scheduler returns the scheduler instance.
func (a *Agent) Scheduler() *scheduler.Scheduler {
	return a.scheduler
}

// SetResultMode sets the result processing mode and rebuilds the system prompt.
// This resets the conversation history to apply the new mode.
func (a *Agent) SetResultMode(mode config.ResultMode) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.resultMode = mode
	// Rebuild system prompt with current identity info from config
	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
	}
	a.systemPrompt = buildSystemPromptWithMode(a.rules, mode, agentName, agentDesc, agentPrinciples)
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Result mode set to %s, system prompt rebuilt", config.ResultModeString(mode))
}

// getToolTimeout returns the tool call timeout duration.
// Returns 0 (no timeout) if not configured.
func (a *Agent) getToolTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.ToolTimeout > 0 {
		return time.Duration(a.cfg.LLM.ToolTimeout) * time.Second
	}
	return 0
}

// getCommandTimeout returns the system command execution timeout duration.
// Returns 0 (no timeout) if not configured.
func (a *Agent) getCommandTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.CommandTimeout > 0 {
		return time.Duration(a.cfg.LLM.CommandTimeout) * time.Second
	}
	return 0
}

// buildSystemPrompt constructs the system prompt with rules and context.
func buildSystemPrompt(rules string) string {
	return buildSystemPromptWithMode(rules, config.ResultModeMinimal, "", "", "")
}

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
// The prompt is built using the current i18n language setting.
// agentName, agentDescription, agentPrinciples are optional identity fields from config.
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

	capabilities := i18n.TF(i18n.KeySystemPromptCapabilities, sh)

	rulesText := i18n.T(i18n.KeySystemPromptRules)

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
		return `When you execute a system command and receive its output, provide a brief explanation of what the output means. Keep your explanation concise (2-3 sentences max). Focus on the key information the user would want to know.`
	case config.ResultModeAnalyze:
		return `When you execute a system command and receive its output, perform a thorough analysis. Explain patterns, anomalies, and implications in detail. Provide actionable insights and recommendations based on the output.`
	case config.ResultModeFree:
		return `You have full autonomy to decide how to present command execution results. Use your judgment to determine the best way to respond based on the context and the user's needs.`
	default:
		return ""
	}
}

// Run processes a user input through the agent loop.
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.mu.Lock()
	// If there are image paths, create a multimodal message
	if len(a.imagePaths) > 0 {
		multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)
		if err != nil {
			a.mu.Unlock()
			return "", fmt.Errorf("cannot build multimodal message: %w", err)
		}
		a.messages = append(a.messages, multimodalMsg)
		a.imagePaths = nil // clear after use
	} else {
		// Add user message to history
		a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})
	}
	a.mu.Unlock()

	log.Info("Agent.Run: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Call LLM
		resp, err := a.llmClient.Chat(ctx, a.messages, tools)

		if err != nil {
			log.Error("Agent.Run: LLM call failed at iteration %d: %v", iteration, err)
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// If no tool calls, this is the final answer
		if len(resp.ToolCalls) == 0 {
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          resp.Content,
				ReasoningContent: resp.ReasoningContent,
			})
			a.mu.Unlock()
			log.Info("Agent.Run: completed after %d iterations", iteration+1)
			return resp.Content, nil
		}

		// Add assistant message with tool calls
		a.mu.Lock()
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ToolCalls:        resp.ToolCalls,
			ReasoningContent: resp.ReasoningContent,
		})
		a.mu.Unlock()

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			log.Info("Agent.Run: executing tool %s (ID: %s)", tc.Name, tc.ID)
			result, err := a.executeToolCall(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				log.Error("Agent.Run: tool %s failed: %v", tc.Name, err)
			}

			// Add tool result to messages
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
			a.mu.Unlock()
		}
	}

	log.Error("Agent.Run: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// RunStream processes a user input through the agent loop with streaming output.
// It sends stream events to the provided callback function.
func (a *Agent) RunStream(ctx context.Context, userInput string, cb StreamCallback) (string, error) {
	// Reset approveAll flag for each new request
	a.approveAll = false

	a.mu.Lock()
	// If there are image paths, create a multimodal message
	if len(a.imagePaths) > 0 {
		multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)
		if err != nil {
			a.mu.Unlock()
			return "", fmt.Errorf("cannot build multimodal message: %w", err)
		}
		a.messages = append(a.messages, multimodalMsg)
		a.imagePaths = nil // clear after use
	} else {
		// Add user message to history
		a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})
	}
	a.mu.Unlock()

	log.Info("Agent.RunStream: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Step 1: Stream the LLM response

		finalContent, finalReasoning, toolCalls, streamErr := a.streamLLMResponse(ctx, tools, cb)
		if streamErr != nil {
			log.Error("Agent.RunStream: stream error at iteration %d: %v", iteration, streamErr)
			return "", streamErr
		}

		// Step 2: If no tool calls, this is the final answer
		if len(toolCalls) == 0 {
			cb("done", "")

			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          finalContent,
				ReasoningContent: finalReasoning,
			})
			a.mu.Unlock()
			log.Info("Agent.RunStream: completed after %d iterations", iteration+1)
			return finalContent, nil
		}

		// Step 3: First add assistant message with tool_calls to history
		// This must come BEFORE tool result messages to satisfy the API requirement
		// that tool messages must follow a message with tool_calls.
		a.mu.Lock()
		assistantMsgIdx := len(a.messages)
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          finalContent,
			ToolCalls:        toolCalls,
			ReasoningContent: finalReasoning,
		})
		a.mu.Unlock()

		// Step 4: Execute tool calls and add results
		modifyRequested := false
		cancelled := false
		for _, tc := range toolCalls {
			// Show command if enabled
			if a.showCommand && tc.Name == "execute_command" {
				var cmdArgs map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &cmdArgs); err == nil {
					if cmd, ok := cmdArgs["command"].(string); ok {
						cb("command", cmd)
					}
				}
			}

			cb("tool_call", fmt.Sprintf("🛠 Calling tool: %s\n", tc.Name))

			log.Info("Agent.RunStream: executing tool %s (ID: %s)", tc.Name, tc.ID)
			result, execErr := a.executeToolCall(ctx, tc)
			if execErr != nil {
				errStr := execErr.Error()
				// Check if user cancelled
				if strings.HasPrefix(errStr, "CANCEL_AGENT") {
					cancelled = true
					// Remove the incomplete assistant message (with tool_calls) from history
					a.mu.Lock()
					a.messages = a.messages[:assistantMsgIdx]
					a.mu.Unlock()
					break
				}
				// Check if this is a USER_MODIFY_REQUEST (user wants to modify and re-evaluate)
				if strings.HasPrefix(errStr, "USER_MODIFY_REQUEST:") {
					modifyRequested = true
					modifyInput := strings.TrimPrefix(errStr, "USER_MODIFY_REQUEST:")
					// Remove the incomplete assistant message (with tool_calls) from history
					// since its tool_calls were not fully executed.
					a.mu.Lock()
					a.messages = a.messages[:assistantMsgIdx]
					// Add the user's modification as a new user message
					a.messages = append(a.messages, llm.Message{
						Role:    "user",
						Content: modifyInput,
					})
					a.mu.Unlock()
					cb("info", fmt.Sprintf("\n🔄 用户补充说明: %s\n", modifyInput))
					break
				}
				result = fmt.Sprintf("Error: %v", execErr)
				log.Error("Agent.RunStream: tool %s failed: %v", tc.Name, execErr)
			}

			// Show command output if enabled (before LLM analysis)
			if a.showOutput && tc.Name == "execute_command" && result != "" {
				cb("output", result)
			}

			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
			a.mu.Unlock()
		}

		// If user cancelled, return to REPL
		if cancelled {
			return "", nil
		}

		// If user requested modification, continue the loop to re-ask the LLM
		if modifyRequested {
			continue
		}

	}

	log.Error("Agent.RunStream: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// streamLLMResponse streams the LLM response and returns the complete content, reasoning, and tool calls.
// If streaming fails, it falls back to non-streaming Chat.
func (a *Agent) streamLLMResponse(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	// Try streaming first
	eventCh, err := a.llmClient.ChatStream(ctx, a.messages, tools)
	if err != nil {
		// Fall back to non-streaming
		log.Debug("ChatStream not available, falling back to non-streaming: %v", err)
		return a.nonStreamingFallback(ctx, tools, cb)
	}

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var toolCalls []llm.ToolCall

	// Filter function for tool calls that may have incomplete data from stream deltas
	// (e.g., empty name or ID which would cause "missing field 'name'" API errors)
	isValidToolCall := func(tc llm.ToolCall) bool {
		return tc.Name != "" && tc.ID != ""
	}

	for event := range eventCh {
		switch event.Type {
		case llm.StreamEventContent:
			contentBuilder.WriteString(event.Content)
			cb("content_chunk", event.Content)

		case llm.StreamEventReasoning:
			reasoningBuilder.WriteString(event.Content)
			if a.showThinking {
				cb("thinking_chunk", event.Content)
			}

		case llm.StreamEventToolCall:
			if event.ToolCall != nil && isValidToolCall(*event.ToolCall) {
				toolCalls = append(toolCalls, *event.ToolCall)
			}

		case llm.StreamEventDone:
			// Stream finished - tool calls are already accumulated from stream deltas.
			// No need for an extra non-streaming API call.
			finalContent := contentBuilder.String()
			finalReasoning := reasoningBuilder.String()
			return finalContent, finalReasoning, toolCalls, nil

		case llm.StreamEventError:
			return "", "", nil, event.Err
		}
	}

	// If we get here, the channel closed without a Done event
	// Fall back to non-streaming
	log.Debug("Stream channel closed without Done event, falling back to non-streaming")
	return a.nonStreamingFallback(ctx, tools, cb)
}

// nonStreamingFallback handles the case when streaming is not available.
func (a *Agent) nonStreamingFallback(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	resp, err := a.llmClient.Chat(ctx, a.messages, tools)
	if err != nil {
		return "", "", nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if a.showThinking && resp.ReasoningContent != "" {
		cb("thinking", resp.ReasoningContent)
	}

	return resp.Content, resp.ReasoningContent, resp.ToolCalls, nil
}

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
			Description: "Replace sections of content in an existing file using SEARCH/REPLACE blocks. The SEARCH content must match the file exactly (including whitespace and indentation). Only the first match is replaced. Use this to make targeted changes to specific parts of a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The absolute path to the file to modify",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "The exact content to find in the file (must match character-for-character including whitespace and indentation)",
					},
					"replace": map[string]interface{}{
						"type":        "string",
						"description": "The new content to replace the matched section with",
					},
				},
				"required": []string{"path", "search", "replace"},
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
		{
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
		},
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
			if !a.approveAll {
				result, modifyInput := promptCommandConfirmation(cmd)
				switch result {
				case CmdConfirmCancel:
					return i18n.T(i18n.KeyCmdConfirmCancelled), fmt.Errorf("CANCEL_AGENT")
				case CmdConfirmApproveAll:
					a.approveAll = true
					// fall through to execute
				case CmdConfirmModify:
					// Use the user's input directly as supplementary instructions
					// for the LLM to re-evaluate the command
					return "", fmt.Errorf("USER_MODIFY_REQUEST: %s", modifyInput)
				}
				// CmdConfirmApprove: continue execution
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

// promptCommandConfirmation displays the command to the user and asks for confirmation.
// Returns the user's choice and any supplementary input.
// - Enter: approve and execute
// - c/C: cancel, return to REPL
// - Any other input: treated as supplementary instructions for the LLM to re-evaluate
func promptCommandConfirmation(command string) (CmdConfirmResult, string) {
	fmt.Println()
	fmt.Println(i18n.TF(i18n.KeyCmdConfirmTitle, command))
	fmt.Println()

	// Read a single line from stdin using os.Stdin.Read() which works
	// even when go-prompt has set the terminal to raw mode.
	// We read byte by byte until we get a newline.
	for {
		fmt.Print(i18n.T(i18n.KeyCmdConfirmPrompt))

		var lineBuf []byte
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				break
			}
			if buf[0] == '\n' || buf[0] == '\r' {
				break
			}
			lineBuf = append(lineBuf, buf[0])
		}

		response := strings.TrimSpace(string(lineBuf))

		if response == "" {
			return CmdConfirmApprove, ""
		}

		lower := strings.ToLower(response)
		if lower == "c" {
			return CmdConfirmCancel, ""
		}

		if lower == "a" {
			return CmdConfirmApproveAll, ""
		}

		// Any other input is treated as supplementary instructions
		// for the LLM to re-evaluate the command
		return CmdConfirmModify, response

	}
}

// readLine reads a line of input from stdin using os.Stdin.Read() which works
// even when go-prompt has set the terminal to raw mode.
func readLine() string {
	var lineBuf []byte
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
		lineBuf = append(lineBuf, buf[0])
	}
	return strings.TrimSpace(string(lineBuf))
}

// executeSystemCommand runs a system command with timeout.

func (a *Agent) executeSystemCommand(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	// Determine timeout: use args timeout_seconds first, then configured command timeout
	var timeout int
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	} else {
		cmdTimeout := a.getCommandTimeout()
		if cmdTimeout > 0 {
			timeout = int(cmdTimeout.Seconds())
		}
	}

	// Only set timeout if a positive value is specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	shell, shellArg := shellCmd()
	log.Debug("Executing command: %s (timeout: %ds, shell: %s)", command, timeout, shell)
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
	output, err := cmd.CombinedOutput()
	// Decode GBK to UTF-8 on Windows
	decoded := decodeToUTF8(output)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Warn("Command timed out after %d seconds: %s", timeout, command)
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		log.Error("Command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(decoded), nil
}

// ExecuteCommandDirectly runs a system command directly without LLM involvement.
// This is used by the REPL when user input is detected as a direct system command.
func (a *Agent) ExecuteCommandDirectly(command string) (string, error) {
	timeout := a.getCommandTimeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		shell, shellArg := shellCmd()
		log.Info("Direct command: %s (timeout: %ds, shell: %s)", command, int(timeout.Seconds()), shell)
		cmd := exec.CommandContext(ctx, shell, shellArg, command)
		output, err := cmd.CombinedOutput()
		// Decode GBK to UTF-8 on Windows
		decoded := decodeToUTF8(output)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Warn("Direct command timed out: %s", command)
				return "", fmt.Errorf("command timed out after %d seconds", int(timeout.Seconds()))
			}
			log.Error("Direct command failed: %s, error: %v", command, err)
			return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
		}

		log.Debug("Direct command completed: %s (output length: %d)", command, len(output))
		return strings.TrimSpace(decoded), nil
	}

	// No timeout - use background context
	shell, shellArg := shellCmd()
	log.Info("Direct command: %s (no timeout, shell: %s)", command, shell)
	cmd := exec.CommandContext(context.Background(), shell, shellArg, command)

	output, err := cmd.CombinedOutput()
	// Decode GBK to UTF-8 on Windows
	decoded := decodeToUTF8(output)
	if err != nil {
		log.Error("Direct command failed: %s, error: %v", command, err)
		return decoded, fmt.Errorf("command failed: %w\nOutput: %s", err, decoded)
	}

	log.Debug("Direct command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(decoded), nil
}

// readFileTool reads the contents of a file and returns it with line numbers.
func (a *Agent) readFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	// Determine start and end lines
	startLine := 1
	endLine := 0 // 0 means read to end
	if s, ok := args["start_line"].(float64); ok {
		startLine = int(s)
	}
	if e, ok := args["end_line"].(float64); ok {
		endLine = int(e)
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	// Validate start_line
	if startLine < 1 {
		startLine = 1
	}
	if startLine > totalLines {
		return "", fmt.Errorf("start_line %d exceeds file length (%d lines)", startLine, totalLines)
	}

	// Determine end_line
	if endLine <= 0 || endLine > totalLines {
		endLine = totalLines
	}
	if endLine < startLine {
		endLine = startLine
	}

	// Limit output to 1000 lines
	if endLine-startLine+1 > 1000 {
		endLine = startLine + 999
	}

	// Build output with line numbers
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s (%d lines total, showing %d-%d)\n\n", path, totalLines, startLine, endLine))
	for i := startLine - 1; i < endLine; i++ {
		result.WriteString(fmt.Sprintf("%d | %s\n", i+1, lines[i]))
	}

	if endLine < totalLines {
		result.WriteString(fmt.Sprintf("... (%d more lines)\n", totalLines-endLine))
	}

	return result.String(), nil
}

// searchFilesTool searches for a regex pattern across files in a directory.
func (a *Agent) searchFilesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	dirPath, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	pattern, ok := args["regex"].(string)
	if !ok {
		return "", fmt.Errorf("regex argument is required")
	}

	filePattern, _ := args["file_pattern"].(string)

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		dirPath = filepath.Join(cwd, dirPath)
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex %q: %w", pattern, err)
	}

	// Walk the directory
	var result strings.Builder
	var matchCount int
	const maxMatches = 100

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() {
			return nil
		}

		// Check file pattern if specified
		if filePattern != "" {
			matched, err := filepath.Match(filePattern, info.Name())
			if err != nil || !matched {
				return nil
			}
		}

		// Read the file
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				if matchCount >= maxMatches {
					return filepath.SkipDir
				}
				relPath, _ := filepath.Rel(dirPath, path)
				result.WriteString(fmt.Sprintf("%s:%d: %s\n", relPath, i+1, strings.TrimSpace(line)))
				matchCount++
			}
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if matchCount == 0 {
		return fmt.Sprintf("No matches found for pattern %q in %s", pattern, dirPath), nil
	}

	return fmt.Sprintf("Found %d matches for pattern %q in %s:\n%s", matchCount, pattern, dirPath, result.String()), nil
}

// listCodeDefinitionNamesTool lists definition names in source code files at the top level of a directory.
func (a *Agent) listCodeDefinitionNamesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	dirPath, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		dirPath = filepath.Join(cwd, dirPath)
	}

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read directory %q: %w", dirPath, err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Definitions in %s:\n\n", dirPath))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process source code files
		ext := filepath.Ext(entry.Name())
		switch ext {
		case ".go", ".py", ".js", ".ts", ".java", ".c", ".h", ".cpp", ".hpp", ".rs", ".rb", ".php":
			// supported
		default:
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			continue
		}

		content := string(data)
		var definitions []string

		switch ext {
		case ".go":
			// Match Go function/method/type definitions
			goRe := regexp.MustCompile(`(?:^|\n)\s*(?:func\s+(?:\([^)]*\)\s*)?(\w+)|type\s+(\w+)\s+(?:struct|interface|func))`)
			matches := goRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				if name != "" {
					definitions = append(definitions, fmt.Sprintf("  func/type: %s", name))
				}
			}
		case ".py":
			pyRe := regexp.MustCompile(`(?:^|\n)\s*(?:def\s+(\w+)|class\s+(\w+))`)
			matches := pyRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				definitions = append(definitions, fmt.Sprintf("  def/class: %s", name))
			}
		case ".js", ".ts":
			jsRe := regexp.MustCompile(`(?:^|\n)\s*(?:function\s+(\w+)|(?:export\s+)?(?:const|let|var)\s+(\w+)\s*[:=]\s*(?:function|\(|=>)|class\s+(\w+))`)
			matches := jsRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				name := m[1]
				if name == "" {
					name = m[2]
				}
				if name == "" {
					name = m[3]
				}
				if name != "" {
					definitions = append(definitions, fmt.Sprintf("  func/class: %s", name))
				}
			}
		case ".java":
			javaRe := regexp.MustCompile(`(?:^|\n)\s*(?:public|private|protected)?\s*(?:static\s+)?(?:class|interface|enum)\s+(\w+)`)
			matches := javaRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				definitions = append(definitions, fmt.Sprintf("  class: %s", m[1]))
			}
		default:
			// Generic: look for function/class definitions
			genericRe := regexp.MustCompile(`(?:^|\n)\s*(?:function|def|class|type|struct)\s+(\w+)`)
			matches := genericRe.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				definitions = append(definitions, fmt.Sprintf("  def: %s", m[1]))
			}
		}

		if len(definitions) > 0 {
			result.WriteString(fmt.Sprintf("%s:\n", entry.Name()))
			for _, d := range definitions {
				result.WriteString(d + "\n")
			}
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// replaceInFileTool replaces sections of content in an existing file using SEARCH/REPLACE.
func (a *Agent) replaceInFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	search, ok := args["search"].(string)
	if !ok {
		return "", fmt.Errorf("search argument is required")
	}

	replace, ok := args["replace"].(string)
	if !ok {
		return "", fmt.Errorf("replace argument is required")
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", path, err)
	}

	content := string(data)

	// Find the search string
	idx := strings.Index(content, search)
	if idx < 0 {
		return "", fmt.Errorf("search content not found in file %q. The SEARCH content must match the file exactly (including whitespace and indentation)", path)
	}

	// Replace only the first occurrence
	newContent := content[:idx] + replace + content[idx+len(search):]

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", path, err)
	}

	return fmt.Sprintf("Successfully replaced content in %s", path), nil
}

// writeToFileTool writes content to a file, creating directories as needed.
func (a *Agent) writeToFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content argument is required")
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create directories for %q: %w", path, err)
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", path, err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// subAgentMemoryKey returns the memory key for a sub-agent by ID.
func subAgentMemoryKey(id int) string {
	return fmt.Sprintf("sub_agent:%d", id)
}

// getNextSubAgentID finds the next available sub-agent ID by scanning memory.
func (a *Agent) getNextSubAgentID() (int, error) {
	entries, err := a.store.SearchMemory("sub_agent:")
	if err != nil {
		return 1, nil // start from 1 if search fails
	}

	maxID := 0
	for _, entry := range entries {
		var info subagent.SubAgentInfo
		if err := json.Unmarshal([]byte(entry.Value), &info); err != nil {
			continue
		}
		if info.ID > maxID {
			maxID = info.ID
		}
	}
	return maxID + 1, nil
}

// getSubAgentInfo retrieves sub-agent info from memory by ID.
func (a *Agent) getSubAgentInfo(id int) (*subagent.SubAgentInfo, error) {
	val, found, err := a.store.GetMemory(subAgentMemoryKey(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("sub-agent #%d not found in memory", id)
	}

	var info subagent.SubAgentInfo
	if err := json.Unmarshal([]byte(val), &info); err != nil {
		return nil, fmt.Errorf("cannot parse sub-agent info: %w", err)
	}
	return &info, nil
}

// saveSubAgentInfo saves sub-agent info to memory.
func (a *Agent) saveSubAgentInfo(info *subagent.SubAgentInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("cannot marshal sub-agent info: %w", err)
	}
	return a.store.SaveMemory(subAgentMemoryKey(info.ID), string(data))
}

// launchSubAgentTool launches a sub-agent process and returns its results.
// Sub-agent workspaces are auto-created under {parent_workspace}/sub-agents/{id}/.
// Each sub-agent is tracked in memory with its ID, workspace path, and purpose.
func (a *Agent) launchSubAgentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	instruction, ok := args["instruction"].(string)
	if !ok {
		return "", fmt.Errorf("instruction argument is required")
	}

	var timeout int
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	}

	purpose, _ := args["purpose"].(string)

	// Determine parent workspace
	parentWorkspace, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get parent workspace: %w", err)
	}

	// Check if reusing an existing sub-agent
	var subID int
	var workspacePath string
	var isNew bool

	if idVal, ok := args["sub_agent_id"].(float64); ok {
		// Reuse existing sub-agent
		subID = int(idVal)
		info, err := a.getSubAgentInfo(subID)
		if err != nil {
			return "", fmt.Errorf("cannot reuse sub-agent #%d: %v", subID, err)
		}
		workspacePath = info.Workspace
		// Update last instruction
		info.LastInstruction = instruction
		if purpose != "" {
			info.Purpose = purpose
		}
		if err := a.saveSubAgentInfo(info); err != nil {
			log.Warn("Cannot update sub-agent #%d memory: %v", subID, err)
		}
		fmt.Printf("\n🔄 Reusing sub-agent #%d (workspace: %s)\n\n", subID, workspacePath)
	} else {
		// Create new sub-agent
		subID, err = a.getNextSubAgentID()
		if err != nil {
			return "", fmt.Errorf("cannot allocate sub-agent ID: %w", err)
		}
		// Use agent name in workspace folder: {name}-{id}
		workspacePath = filepath.Join(parentWorkspace, "sub-agents", fmt.Sprintf("%s-%d", a.name, subID))

		// Save to memory
		info := &subagent.SubAgentInfo{
			ID:              subID,
			Workspace:       workspacePath,
			Purpose:         purpose,
			CreatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			LastInstruction: instruction,
		}
		if err := a.saveSubAgentInfo(info); err != nil {
			log.Warn("Cannot save sub-agent #%d memory: %v", subID, err)
		}
		isNew = true
		fmt.Printf("\n📂 [%s] Creating sub-agent #%d (workspace: %s)\n\n", a.name, subID, workspacePath)
	}

	cfg := subagent.SubAgentConfig{
		Workspace:      workspacePath,
		Instruction:    instruction,
		TimeoutSeconds: timeout,
		Purpose:        purpose,
		ImagePaths:     a.imagePaths,
	}

	log.Info("Launching sub-agent #%d: workspace=%s, instruction=%s, timeout=%ds", subID, workspacePath, instruction, timeout)

	result, err := a.subAgentMgr.LaunchSubAgent(ctx, cfg)
	if err != nil {
		log.Error("Failed to launch sub-agent #%d: %v", subID, err)
		return "", fmt.Errorf("failed to launch sub-agent #%d: %w", subID, err)
	}

	// Build result summary
	var sb strings.Builder
	if isNew {
		sb.WriteString(fmt.Sprintf("Sub-agent #%d completed.\n", subID))
	} else {
		sb.WriteString(fmt.Sprintf("Sub-agent #%d (reused) completed.\n", subID))
	}
	sb.WriteString(result.ResultSummary())

	// Include output file contents if any
	for _, f := range result.OutputFiles {
		filePath := filepath.Join(workspacePath, "output", f)
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			sb.WriteString(fmt.Sprintf("\n  ⚠️ Cannot read output file %s: %v\n", f, readErr))
			continue
		}
		sb.WriteString(fmt.Sprintf("\n📄 Output file: %s\n", f))
		sb.WriteString(string(data))
		if !strings.HasSuffix(string(data), "\n") {
			sb.WriteString("\n")
		}
	}

	log.Info("Sub-agent #%d completed: duration=%s, exitCode=%d", subID, result.Duration, result.ExitCode)
	return sb.String(), nil
}

// scheduleTaskTool schedules a recurring task using a cron expression.
func (a *Agent) scheduleTaskTool(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("name argument is required")
	}

	cron, ok := args["cron"].(string)
	if !ok {
		return "", fmt.Errorf("cron argument is required")
	}

	instruction, ok := args["instruction"].(string)
	if !ok {
		return "", fmt.Errorf("instruction argument is required")
	}

	if a.scheduler == nil {
		return "", fmt.Errorf("scheduler is not initialized")
	}

	id, err := a.scheduler.Add(name, cron, instruction)
	if err != nil {
		return "", fmt.Errorf("cannot schedule task: %w", err)
	}

	// Persist to store
	if err := a.persistSchedulerEntries(); err != nil {
		log.Warn("Cannot persist scheduler entries: %v", err)
	}

	return fmt.Sprintf("✅ 定时任务 #%d (%s) 已创建\n  Cron: %s\n  指令: %s\n  下次执行: %s",
		id, name, cron, instruction, scheduler.FormatNextRun(a.scheduler.Get(id).NextRun)), nil
}

// OnScheduledTaskTriggered is called by the scheduler when a task is triggered.
// It launches a sub-agent with the task's instruction.
func (a *Agent) OnScheduledTaskTriggered(entry *scheduler.CronEntry) {
	fmt.Printf("\n⏰ [%s] 定时任务 #%d (%s) 已触发\n\n", time.Now().Format("2006-01-02 15:04:05"), entry.ID, entry.Name)

	// Build instruction with context about being a scheduled task
	instruction := fmt.Sprintf("[定时任务触发] 任务名称: %s\n\n%s\n\n注意：你是被定时任务调度器自动启动的 sub-agent。请执行上述指令，完成后退出。",
		entry.Name, entry.Instruction)

	// Determine parent workspace
	parentWorkspace, err := os.Getwd()
	if err != nil {
		log.Error("Cannot get parent workspace for scheduled task: %v", err)
		return
	}

	// Create a workspace for this scheduled task execution
	workspacePath := filepath.Join(parentWorkspace, "sub-agents", fmt.Sprintf("scheduled-%s-%d", a.name, entry.ID))

	cfg := subagent.SubAgentConfig{
		Workspace:   workspacePath,
		Instruction: instruction,
		Purpose:     fmt.Sprintf("Scheduled task: %s", entry.Name),
	}

	log.Info("Scheduled task #%d (%s): launching sub-agent, workspace=%s", entry.ID, entry.Name, workspacePath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := a.subAgentMgr.LaunchSubAgent(ctx, cfg)
	if err != nil {
		log.Error("Scheduled task #%d sub-agent failed: %v", entry.ID, err)
		fmt.Printf("❌ 定时任务 #%d (%s) 执行失败: %v\n", entry.ID, entry.Name, err)
		return
	}

	fmt.Printf("\n✅ 定时任务 #%d (%s) 执行完成 (退出码: %d, 耗时: %s)\n",
		entry.ID, entry.Name, result.ExitCode, result.Duration)

	// Persist updated entries (next run time may have changed)
	if err := a.persistSchedulerEntries(); err != nil {
		log.Warn("Cannot persist scheduler entries after trigger: %v", err)
	}
}

// persistSchedulerEntries saves all scheduler entries to the store.
func (a *Agent) persistSchedulerEntries() error {
	if a.store == nil || a.scheduler == nil {
		return nil
	}

	entries := a.scheduler.GetEntriesForStorage()
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("cannot marshal scheduler entry #%d: %w", entry.ID, err)
		}
		if err := a.store.SaveSchedule(entry.ID, data); err != nil {
			return fmt.Errorf("cannot save scheduler entry #%d: %w", entry.ID, err)
		}
	}
	return nil
}

// Reset clears the conversation history but keeps the system prompt.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Agent history reset")
}

// GetHistory returns the current conversation history.
func (a *Agent) GetHistory() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messages
}

// SetHistory restores a previous conversation history.
func (a *Agent) SetHistory(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = messages
}

// GetMessages returns the current messages slice (thread-safe).
func (a *Agent) GetMessages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}
