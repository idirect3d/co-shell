// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
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
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
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

// SetConfig sets the configuration for timeout settings.
func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
}

// SetResultMode sets the result processing mode and rebuilds the system prompt.
// This resets the conversation history to apply the new mode.
func (a *Agent) SetResultMode(mode config.ResultMode) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.resultMode = mode
	a.systemPrompt = buildSystemPromptWithMode(a.rules, mode)
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
	return buildSystemPromptWithMode(rules, config.ResultModeMinimal)
}

// buildSystemPromptWithMode constructs the system prompt with rules, context, and result mode.
func buildSystemPromptWithMode(rules string, mode config.ResultMode) string {
	sh := shellName()

	// Gather environment context
	cwd, _ := os.Getwd()
	hostname, _ := os.Hostname()
	now := time.Now().Format("2006-01-02 15:04:05 Monday")
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	prompt := fmt.Sprintf(`You are co-shell, an intelligent command-line assistant that helps users interact with their system through natural language.

Current Environment:
- Platform: %s (%s)
- Shell: %s
- Current Time: %s
- Working Directory: %s
- Hostname: %s
- User: %s

You have access to the following capabilities:
1. Execute system commands (%s)
2. Call MCP (Model Context Protocol) tools
3. Read and write files
4. Manage memory and context

IMPORTANT RULES:
- Use the "execute_command" tool to run system commands, and the appropriate MCP tool names for MCP operations.
- Unless the user specifies otherwise, prefer using standard system commands (e.g., cat, ls, dir, type) over writing scripts or programs.
- Actively explore the system to discover available tools (e.g., check PATH, common tool directories). If the required tool is not found, try to install it, or use scripts and programming languages (Shell, Python, Go, Node.js, etc.) to write custom tools to fulfill the user's needs.
- Always explain what you're doing before executing commands.
- For destructive operations (delete, overwrite, rm -rf, etc.), ask for confirmation first.
- Use the user's preferred language for responses.
- You have full autonomy to choose the best tools and approaches for each task — use your judgment.

RESULT PROCESSING MODE:
%s

Available tools will be provided to you as function definitions.`,
		runtime.GOOS, runtime.GOARCH, sh, now, cwd, hostname, username, sh, resultModeInstruction(mode))

	if rules != "" {
		prompt += fmt.Sprintf("\n\nUser-defined Rules:\n%s", rules)
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
	// Add user message to history
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})
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
	// Add user message to history
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})
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
			if event.ToolCall != nil {
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
