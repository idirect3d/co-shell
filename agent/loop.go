// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
)

const (
	defaultMaxIterations = 10
	toolTimeout          = 30 * time.Second
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

// StreamCallback is a function called for each streaming event from the LLM.
type StreamCallback func(eventType string, content string)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	mu            sync.Mutex
	llmClient     llm.Client
	mcpMgr        *mcp.Manager
	store         *store.Store
	systemPrompt  string
	messages      []llm.Message
	showThinking  bool
	showCommand   bool
	showOutput    bool
	maxIterations int
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
// If set to -1 or 0, the default value (10) will be used.
func (a *Agent) SetMaxIterations(n int) {
	if n <= 0 {
		a.maxIterations = defaultMaxIterations
	} else {
		a.maxIterations = n
	}
}

// buildSystemPrompt constructs the system prompt with rules and context.
func buildSystemPrompt(rules string) string {
	sh := shellName()
	prompt := fmt.Sprintf(`You are co-shell, an intelligent command-line assistant that helps users interact with their system through natural language.

You have access to the following capabilities:
1. Execute system commands (%s)
2. Call MCP (Model Context Protocol) tools
3. Read and write files
4. Manage memory and context

IMPORTANT RULES:
- When the user asks you to read a file or show file contents, use "cat <filepath>" command directly. Do NOT write or generate code (Go, Python, etc.) to read files.
- When the user asks you to list directory contents, use "ls -la <path>" command directly.
- Always prefer simple shell commands over writing scripts or programs.
- If you need to execute a system command, use the "execute_command" tool
- If you need to call an MCP tool, use the appropriate tool name
- Always explain what you're doing before executing commands
- For destructive operations (delete, overwrite), ask for confirmation first
- Use the user's preferred language for responses

Available tools will be provided to you as function definitions.`, sh)

	if rules != "" {
		prompt += fmt.Sprintf("\n\nUser-defined Rules:\n%s", rules)
	}

	return prompt
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

	for iteration := 0; iteration < a.maxIterations; iteration++ {
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
	a.mu.Lock()
	// Add user message to history
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})
	a.mu.Unlock()

	log.Info("Agent.RunStream: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; iteration < a.maxIterations; iteration++ {
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
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          finalContent,
			ToolCalls:        toolCalls,
			ReasoningContent: finalReasoning,
		})
		a.mu.Unlock()

		// Step 4: Execute tool calls and add results
		for _, tc := range toolCalls {
			// Show command if enabled
			if a.showCommand && tc.Name == "execute_command" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err == nil {
					if cmd, ok := args["command"].(string); ok {
						cb("command", cmd)
					}
				}
			}
			cb("tool_call", fmt.Sprintf("🛠 Calling tool: %s\n", tc.Name))

			log.Info("Agent.RunStream: executing tool %s (ID: %s)", tc.Name, tc.ID)
			result, execErr := a.executeToolCall(ctx, tc)
			if execErr != nil {
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

		case llm.StreamEventDone:
			// Stream finished - now make a non-streaming call to detect tool calls
			finalContent := contentBuilder.String()
			finalReasoning := reasoningBuilder.String()

			// Build temporary messages with the streamed assistant response
			tempMessages := make([]llm.Message, len(a.messages))
			copy(tempMessages, a.messages)
			tempMessages = append(tempMessages, llm.Message{
				Role:             "assistant",
				Content:          finalContent,
				ReasoningContent: finalReasoning,
			})

			resp, chatErr := a.llmClient.Chat(ctx, tempMessages, tools)
			if chatErr != nil {
				return "", "", nil, fmt.Errorf("LLM call failed after stream: %w", chatErr)
			}

			return finalContent, finalReasoning, resp.ToolCalls, nil

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
						"description": "Timeout in seconds (default: 30)",
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

	// Find and execute the tool
	tools := a.buildTools()
	for _, tool := range tools {
		if tool.Name == tc.Name {
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			return tool.Callback(ctx, args)
		}
	}

	return "", fmt.Errorf("tool %q not found", tc.Name)
}

// executeSystemCommand runs a system command with timeout.
func (a *Agent) executeSystemCommand(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	timeout := 30
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = int(t)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	shell, shellArg := shellCmd()
	log.Debug("Executing command: %s (timeout: %ds, shell: %s)", command, timeout, shell)
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Warn("Command timed out after %d seconds: %s", timeout, command)
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		log.Error("Command failed: %s, error: %v", command, err)
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	log.Debug("Command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(string(output)), nil
}

// ExecuteCommandDirectly runs a system command directly without LLM involvement.
// This is used by the REPL when user input is detected as a direct system command.
func (a *Agent) ExecuteCommandDirectly(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()

	shell, shellArg := shellCmd()
	log.Info("Direct command: %s (shell: %s)", command, shell)
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Warn("Direct command timed out: %s", command)
			return "", fmt.Errorf("command timed out after %d seconds", int(toolTimeout.Seconds()))
		}
		log.Error("Direct command failed: %s, error: %v", command, err)
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	log.Debug("Direct command completed: %s (output length: %d)", command, len(output))
	return strings.TrimSpace(string(output)), nil
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
