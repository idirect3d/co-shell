package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/liangshuang/co-shell/llm"
	"github.com/liangshuang/co-shell/mcp"
	"github.com/liangshuang/co-shell/store"
)

const (
	maxIterations = 10
	toolTimeout   = 30 * time.Second
)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	llmClient llm.Client
	mcpMgr    *mcp.Manager
	store     *store.Store
	systemPrompt string
	messages  []llm.Message
}

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.Store, rules string) *Agent {
	systemPrompt := buildSystemPrompt(rules)

	return &Agent{
		llmClient:    llmClient,
		mcpMgr:       mcpMgr,
		store:        s,
		systemPrompt: systemPrompt,
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// buildSystemPrompt constructs the system prompt with rules and context.
func buildSystemPrompt(rules string) string {
	prompt := `You are co-shell, an intelligent command-line assistant that helps users interact with their system through natural language.

You have access to the following capabilities:
1. Execute system commands (bash, zsh, etc.)
2. Call MCP (Model Context Protocol) tools
3. Read and write files
4. Manage memory and context

When responding to the user:
- If you need to execute a system command, use the "execute_command" tool
- If you need to call an MCP tool, use the appropriate tool name
- Always explain what you're doing before executing commands
- For destructive operations (delete, overwrite), ask for confirmation first
- Use the user's preferred language for responses

Available tools will be provided to you as function definitions.`

	if rules != "" {
		prompt += fmt.Sprintf("\n\nUser-defined Rules:\n%s", rules)
	}

	return prompt
}

// Run processes a user input through the agent loop.
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	// Add user message to history
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userInput})

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; iteration < maxIterations; iteration++ {
		// Call LLM
		resp, err := a.llmClient.Chat(ctx, a.messages, tools)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// If no tool calls, this is the final answer
		if len(resp.ToolCalls) == 0 {
			a.messages = append(a.messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			return resp.Content, nil
		}

		// Add assistant message with tool calls
		a.messages = append(a.messages, llm.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result, err := a.executeToolCall(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool result to messages
			a.messages = append(a.messages, llm.Message{
				Role:    "tool",
				Content: result,
			})
		}
	}

	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", maxIterations)
}

// buildTools constructs the list of available tools for the LLM.
func (a *Agent) buildTools() []llm.Tool {
	tools := []llm.Tool{
		{
			Name:        "execute_command",
			Description: "Execute a system command (bash/zsh) and return its output. Use this to run shell commands, scripts, or any CLI tools.",
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

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// Reset clears the conversation history but keeps the system prompt.
func (a *Agent) Reset() {
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
}

// GetHistory returns the current conversation history.
func (a *Agent) GetHistory() []llm.Message {
	return a.messages
}

// SetHistory restores a previous conversation history.
func (a *Agent) SetHistory(messages []llm.Message) {
	a.messages = messages
}
