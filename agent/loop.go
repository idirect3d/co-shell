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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/memory"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/subagent"
	"github.com/idirect3d/co-shell/taskplan"
)

// StreamCallback is a function called for each streaming event from the LLM.
type StreamCallback func(eventType string, content string)

// CmdConfirmResult represents the result of a command confirmation prompt.
type CmdConfirmResult int

const (
	CmdConfirmApprove      CmdConfirmResult = iota
	CmdConfirmApproveAll                    // Approve all commands for this request
	CmdConfirmApproveCount                  // Approve N commands (user entered a number)
	CmdConfirmCancel                        // User cancelled, return to REPL
	CmdConfirmModify                        // User entered custom input to modify the command
)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	mu              sync.Mutex
	llmClient       llm.Client
	mcpMgr          *mcp.Manager
	store           *store.Store
	memoryManager   *memory.Manager
	systemPrompt    string
	messages        []llm.Message
	showThinking    bool
	showCommand     bool
	showOutput      bool
	maxIterations   int
	confirmCommand  bool
	approveAll      bool           // if true, skip confirmation for all commands in this request
	approveCount    int            // remaining number of commands to auto-approve (decremented on each use)
	cfg             *config.Config // configuration for timeout settings
	resultMode      config.ResultMode
	outputMode      config.OutputMode
	rules           string // user-defined rules for rebuilding system prompt
	subAgentMgr     *subagent.Manager
	taskPlanMgr     *taskplan.Manager
	scheduler       *scheduler.Scheduler
	name            string   // agent name for identification (default: "co-shell")
	imagePaths      []string // paths to image files for multimodal input
	workspacePath   string   // workspace root path for loading external config files
	memoryEnabled   bool     // whether persistent memory tools are enabled
	planEnabled     bool     // whether task plan tools are enabled
	subAgentEnabled bool     // whether sub-agent tools are enabled

	// messagePointer is the index in a.messages that marks the starting position
	// for sending to LLM. Messages before this index are ignored when building
	// context for LLM calls. When a new checklist is created or updated, the
	// pointer is moved to the end, effectively ignoring prior conversation.
	messagePointer int

	// needAdjustPointer is set by createTaskPlanTool/insertTaskStepsTool/removeTaskStepsTool
	// when the task plan is successfully modified. The agent loop checks this flag after
	// all tool messages have been appended, and adjusts messagePointer to skip past
	// the tool messages, so the next LLM iteration starts fresh from the checklist context.
	needAdjustPointer bool

	// errorCounter tracks the number of times each distinct error message has occurred
	// during the current request. Key is the error message string, value is the count.
	// Reset at the start of each RunStream call.
	errorCounter map[string]int

	// errorApproveAll is set to true when the user chooses to ignore all error limits
	// for the current request.
	errorApproveAll bool
}

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.Store, rules string) *Agent {
	systemPrompt := buildSystemPrompt(rules)

	return &Agent{
		llmClient:     llmClient,
		mcpMgr:        mcpMgr,
		store:         s,
		memoryManager: memory.NewManager(s),
		systemPrompt:  systemPrompt,
		maxIterations: config.DefaultConfig().LLM.MaxIterations,
		rules:         rules,
		subAgentMgr:   subagent.NewManager(),
		taskPlanMgr:   taskplan.NewManager(s),
		name:          "co-shell",
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// Messages returns a copy of the current conversation message queue.
func (a *Agent) Messages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
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

// SetMemoryEnabled sets whether persistent memory tools are enabled.
func (a *Agent) SetMemoryEnabled(enabled bool) {
	a.memoryEnabled = enabled
}

// MessagePointer returns the current message pointer index.
// The pointer marks the starting position for sending to LLM.
// Messages before this index are ignored when building context.
func (a *Agent) MessagePointer() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messagePointer
}

// SetPlanEnabled sets whether task plan tools are enabled.
func (a *Agent) SetPlanEnabled(enabled bool) {
	a.planEnabled = enabled
}

// SetSubAgentEnabled sets whether sub-agent tools are enabled.
func (a *Agent) SetSubAgentEnabled(enabled bool) {
	a.subAgentEnabled = enabled
}

// SetOutputMode sets the output display mode.
func (a *Agent) SetOutputMode(mode config.OutputMode) {
	a.outputMode = mode
}

// SetConfig sets the configuration for timeout settings and agent identity.
// It also rebuilds the system prompt with identity information.
func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
	// Rebuild system prompt with identity info from config
	a.rebuildSystemPrompt()
}

// SetLLMClient replaces the LLM client at runtime.
// This is used when settings like api-key, endpoint, model, temperature,
// max-tokens, or vision are changed via .set command without restarting.
func (a *Agent) SetLLMClient(client llm.Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Close old client if it has a Close method
	if a.llmClient != nil {
		a.llmClient.Close()
	}
	a.llmClient = client
	log.Info("LLM client replaced at runtime")
}

// rebuildSystemPrompt rebuilds the system prompt with current config identity info.
// It preserves the conversation history (only replaces the system message at index 0).
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
	// Preserve conversation history: only replace the system message at index 0
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.messages) > 0 {
		a.messages[0] = llm.Message{Role: "system", Content: a.systemPrompt}
	} else {
		a.messages = []llm.Message{
			{Role: "system", Content: a.systemPrompt},
		}
	}
}

// SetWorkspacePath sets the workspace root path for loading external config files
// such as capabilities.md and rules.md.
func (a *Agent) SetWorkspacePath(path string) {
	a.workspacePath = path
}

// SetImagePaths sets the paths to image files for multimodal input.
// These images will be included in the next user message.
func (a *Agent) SetImagePaths(paths []string) {
	a.imagePaths = paths
}

// AddImages adds image file paths to the image cache.
// paths is a comma-separated list of image file paths.
func (a *Agent) AddImages(paths string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	newPaths := strings.Split(paths, ",")
	added := 0
	for _, p := range newPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check if already in cache
		exists := false
		for _, existing := range a.imagePaths {
			if existing == p {
				exists = true
				break
			}
		}
		if !exists {
			a.imagePaths = append(a.imagePaths, p)
			added++
		}
	}

	return fmt.Sprintf("✅ 已添加 %d 张图片到缓存（当前共 %d 张）", added, len(a.imagePaths)), nil
}

// RemoveImages removes image file paths from the image cache.
// paths is a comma-separated list of image file paths.
func (a *Agent) RemoveImages(paths string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	removePaths := strings.Split(paths, ",")
	removed := 0
	var remaining []string
	for _, p := range a.imagePaths {
		shouldRemove := false
		for _, rp := range removePaths {
			if p == strings.TrimSpace(rp) {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			removed++
		} else {
			remaining = append(remaining, p)
		}
	}
	a.imagePaths = remaining

	return fmt.Sprintf("✅ 已从缓存中移除 %d 张图片（当前共 %d 张）", removed, len(a.imagePaths)), nil
}

// ClearImages clears all cached image file paths.
func (a *Agent) ClearImages() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	count := len(a.imagePaths)
	a.imagePaths = nil
	return fmt.Sprintf("✅ 已清空图片缓存（共移除 %d 张图片）", count), nil
}

// ListImages returns a formatted list of all cached image file paths.
func (a *Agent) ListImages() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.imagePaths) == 0 {
		return "📷 图片缓存为空", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📷 图片缓存（共 %d 张）:\n", len(a.imagePaths)))
	for i, p := range a.imagePaths {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, p))
	}
	return sb.String(), nil
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

// TaskPlanManager returns the task plan manager.
func (a *Agent) TaskPlanManager() *taskplan.Manager {
	return a.taskPlanMgr
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

// Run processes a user input through the agent loop.
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.mu.Lock()
	// If there are image paths, create a multimodal message with cached images
	if len(a.imagePaths) > 0 {
		multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)
		if err != nil {
			a.mu.Unlock()
			return "", fmt.Errorf("cannot build multimodal message: %w", err)
		}
		a.messages = append(a.messages, multimodalMsg)
		// Keep imagePaths for reuse in subsequent conversations
	} else {
		// Add user message to history with timestamp prefix
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{Role: "user", Content: tsPrefix + userInput})
		// Sync to memory (content without timestamp prefix, Datetime field stores the time)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
				log.Warn("Failed to save user message to memory: %v", err)
			}
		}
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
			tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          tsPrefix + resp.Content,
				ReasoningContent: resp.ReasoningContent,
			})
			// Sync to memory (content without timestamp prefix)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, resp.Content, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			a.mu.Unlock()
			log.Info("Agent.Run: completed after %d iterations", iteration+1)
			return resp.Content, nil
		}

		// Add assistant message with tool calls
		a.mu.Lock()
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          tsPrefix + resp.Content,
			ToolCalls:        resp.ToolCalls,
			ReasoningContent: resp.ReasoningContent,
		})
		// Sync to memory (content without timestamp prefix)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage(a.name, resp.Content, time.Now()); err != nil {
				log.Warn("Failed to save assistant message to memory: %v", err)
			}
		}
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
			// If the result is empty, provide a clear message to the LLM
			toolContent := result
			if toolContent == "" {
				toolContent = "（工具调用无输出）"
			}
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
			})
			a.mu.Unlock()
		}

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		a.mu.Lock()
		if a.needAdjustPointer {
			a.messagePointer = len(a.messages) - 1
			a.adjustMessagePointer()
			a.needAdjustPointer = false
		}
		a.mu.Unlock()
	}

	log.Error("Agent.Run: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// RunStream processes a user input through the agent loop with streaming output.
// It sends stream events to the provided callback function.
func (a *Agent) RunStream(ctx context.Context, userInput string, cb StreamCallback) (string, error) {
	// Reset approveAll and error tracking flags for each new request
	a.approveAll = false
	a.errorCounter = make(map[string]int)
	a.errorApproveAll = false

	a.mu.Lock()
	// If there are image paths, create a multimodal message with cached images
	if len(a.imagePaths) > 0 {
		multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)
		if err != nil {
			a.mu.Unlock()
			return "", fmt.Errorf("cannot build multimodal message: %w", err)
		}
		a.messages = append(a.messages, multimodalMsg)
		// Keep imagePaths for reuse in subsequent conversations
	} else {
		// Add user message to history with timestamp prefix
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{Role: "user", Content: tsPrefix + userInput})
		// Sync to memory (content without timestamp prefix, Datetime field stores the time)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage("user", userInput, time.Now()); err != nil {
				log.Warn("Failed to save user message to memory: %v", err)
			}
		}
	}
	a.mu.Unlock()

	log.Info("Agent.RunStream: user input: %s", userInput)

	// Build available tools
	tools := a.buildTools()

	for iteration := 0; a.maxIterations < 0 || iteration < a.maxIterations; iteration++ {
		// Step 1: Stream the LLM response
		var finalContent, finalReasoning string
		var toolCalls []llm.ToolCall
		var streamErr error

		finalContent, finalReasoning, toolCalls, streamErr = a.streamLLMResponse(ctx, tools, cb)
		if streamErr != nil {
			// Track error count for this request
			errMsg := streamErr.Error()
			a.errorCounter[errMsg]++
			singleCount := a.errorCounter[errMsg]
			typeCount := len(a.errorCounter)

			// Get configured limits
			maxSingle := 10
			maxType := 100
			if a.cfg != nil {
				if a.cfg.LLM.ErrorMaxSingleCount > 0 {
					maxSingle = a.cfg.LLM.ErrorMaxSingleCount
				}
				if a.cfg.LLM.ErrorMaxTypeCount > 0 {
					maxType = a.cfg.LLM.ErrorMaxTypeCount
				}
			}

			// Check if we need to prompt the user
			needUserPrompt := false
			promptReason := ""

			if singleCount >= maxSingle && !a.errorApproveAll {
				needUserPrompt = true
				promptReason = fmt.Sprintf("相同错误已出现 %d 次（上限 %d 次）", singleCount, maxSingle)
			} else if typeCount >= maxType && !a.errorApproveAll {
				needUserPrompt = true
				promptReason = fmt.Sprintf("不同错误类型已达 %d 种（上限 %d 种）", typeCount, maxType)
			}

			if needUserPrompt {
				// Prompt user for action
				fmt.Printf("\n⚠️ 错误反复出现: %s\n", promptReason)
				fmt.Printf("  最新错误: %v\n", streamErr)
				fmt.Println()
				fmt.Println("  请选择操作:")
				fmt.Println("  [Enter] 继续让 LLM 尝试处理")
				fmt.Println("  [C] 取消，返回 REPL")
				fmt.Println("  [A] 忽略限制，继续执行")
				fmt.Println()
				fmt.Print("  请选择 (Enter/C/A): ")

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

				userChoice := strings.TrimSpace(string(lineBuf))
				lower := strings.ToLower(userChoice)

				if lower == "c" {
					// User cancelled, return to REPL
					cb("info", "\n🛑 用户取消了操作\n")
					return "", nil
				} else if lower == "a" {
					// User chose to ignore all error limits
					a.errorApproveAll = true
					fmt.Println("\n✅ 已忽略错误限制，继续执行")
				} else {
					// Continue (Enter pressed)
					fmt.Println("\n✅ 继续让 LLM 尝试处理")
				}
			}

			// Feed all errors back to the LLM so it can decide how to handle them.
			// The LLM can determine whether the error is recoverable (e.g., invalid arguments,
			// temporary timeout) and retry with corrections, or unrecoverable (e.g., auth failure,
			// model not found) and report to the user.
			log.Warn("Agent.RunStream: stream error at iteration %d: %v, feeding back to LLM", iteration, streamErr)
			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"注意：刚才的 LLM 调用返回了错误，请根据错误信息判断如何处理。\n"+
						"如果错误是可恢复的（如参数格式问题、临时超时），请修正后重试。\n"+
						"如果错误是不可恢复的（如认证失败、模型不存在），请向用户报告错误并终止。\n\n"+
						"错误信息：%s",
					streamErr.Error(),
				),
			})
			a.mu.Unlock()
			cb("info", fmt.Sprintf("\n⚠️ LLM 调用出错: %v\n正在请求 LLM 判断如何处理...\n", streamErr))
			continue
		}

		// Step 2: If no tool calls, this is the final answer
		if len(toolCalls) == 0 {
			cb("done", "")

			a.mu.Lock()
			tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
			a.messages = append(a.messages, llm.Message{
				Role:             "assistant",
				Content:          tsPrefix + finalContent,
				ReasoningContent: finalReasoning,
			})
			// Sync to memory (content without timestamp prefix)
			if a.memoryEnabled {
				if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
					log.Warn("Failed to save assistant message to memory: %v", err)
				}
			}
			a.mu.Unlock()
			log.Info("Agent.RunStream: completed after %d iterations", iteration+1)
			return finalContent, nil
		}

		// Step 3: First add assistant message with tool_calls to history
		// This must come BEFORE tool result messages to satisfy the API requirement
		// that tool messages must follow a message with tool_calls.
		a.mu.Lock()
		assistantMsgIdx := len(a.messages)
		tsPrefix := time.Now().Format("2006-01-02 15:04:05") + " - "
		a.messages = append(a.messages, llm.Message{
			Role:             "assistant",
			Content:          tsPrefix + finalContent,
			ToolCalls:        toolCalls,
			ReasoningContent: finalReasoning,
		})
		// Sync to memory (content without timestamp prefix)
		if a.memoryEnabled {
			if err := a.memoryManager.AddMessage(a.name, finalContent, time.Now()); err != nil {
				log.Warn("Failed to save assistant message to memory: %v", err)
			}
		}
		a.mu.Unlock()

		// Step 4: Execute tool calls and add results
		modifyRequested := false
		cancelled := false
		for _, tc := range toolCalls {
			// Show command if enabled (normal/debug mode only)
			if a.outputMode >= config.OutputModeNormal && a.showCommand && tc.Name == "execute_command" {
				var cmdArgs map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &cmdArgs); err == nil {
					if cmd, ok := cmdArgs["command"].(string); ok {
						cb("command", cmd)
					}
				}
			}

			// Show tool call name (normal/debug mode only)
			if a.outputMode >= config.OutputModeNormal {
				cb("tool_call", fmt.Sprintf("🛠 Calling tool: %s\n", tc.Name))
			}

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

			// Show command output if enabled (debug mode only)
			if a.outputMode >= config.OutputModeDebug && a.showOutput && tc.Name == "execute_command" && result != "" {
				cb("output", result)
			}

			// If the result is empty, provide a clear message to the LLM
			toolContent := result
			if toolContent == "" {
				toolContent = "（工具调用无输出）"
			}

			a.mu.Lock()
			a.messages = append(a.messages, llm.Message{
				Role:       "tool",
				Content:    toolContent,
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

		// If a task plan was modified (created/inserted/removed), adjust messagePointer
		// to skip past all tool messages, so the next LLM iteration starts fresh
		// from the checklist context (the tool result containing the checklist).
		a.mu.Lock()
		if a.needAdjustPointer {
			a.messagePointer = len(a.messages) - 1
			a.adjustMessagePointer()
			a.needAdjustPointer = false
		}
		a.mu.Unlock()

	}

	log.Error("Agent.RunStream: reached maximum iterations (%d)", a.maxIterations)
	return "", fmt.Errorf("agent reached maximum iterations (%d) without a final answer", a.maxIterations)
}

// buildContextMessages returns a truncated message list based on ContextLimit and messagePointer.
// Message layout: [0]=system, [1..n-2]=history, [n-1]=current user input
// The current user input (last message) is ALWAYS kept.
// ContextLimit == 0: only system prompt + current user input (no history)
// ContextLimit == -1: all messages (unlimited)
// ContextLimit > 0: system prompt + current user input + last N history messages
// If messagePointer > 0, messages before the pointer are ignored (the pointer message
// and everything after it are kept). This is used when a checklist is created/updated
// to focus the LLM on the current task plan.
// Each message's content is prefixed with its original index in a.messages,
// e.g. "123: 2026-05-01 12:09:24 - ...", to help the LLM understand the conversation order.
func (a *Agent) buildContextMessages() []llm.Message {
	if a.cfg == nil || a.cfg.LLM.ContextLimit == -1 {
		return a.addIndexPrefixToMessages(a.messages, 0)
	}

	// Always keep system prompt (first message)
	if len(a.messages) <= 1 {
		return a.messages
	}

	systemMsg := a.messages[0]

	// The last message is always the current user input, always keep it
	currentMsg := a.messages[len(a.messages)-1]

	// Determine the effective start index based on messagePointer
	// If pointer > 0, start from pointer (ignore messages before it)
	startIdx := 1
	if a.messagePointer > 0 && a.messagePointer < len(a.messages) {
		startIdx = a.messagePointer
	}

	// History messages are between startIdx and current user input
	historyMsgs := a.messages[startIdx : len(a.messages)-1]

	if a.cfg.LLM.ContextLimit == 0 {
		// Only system prompt + current user input, no history
		result := []llm.Message{systemMsg, currentMsg}
		return a.addIndexPrefixToMessages(result, 0)
	}

	// Keep last N history messages
	if len(historyMsgs) > a.cfg.LLM.ContextLimit {
		// When truncating history, we need to adjust the startIdx for prefix calculation
		truncatedCount := len(historyMsgs) - a.cfg.LLM.ContextLimit
		historyMsgs = historyMsgs[truncatedCount:]
		startIdx += truncatedCount
	}

	result := make([]llm.Message, 0, 2+len(historyMsgs))
	result = append(result, systemMsg)
	result = append(result, historyMsgs...)
	result = append(result, currentMsg)
	return a.addIndexPrefixToMessages(result, startIdx)
}

// addIndexPrefixToMessages adds the original message index prefix to each message's content.
// The format is: "index: content"
// For example: "123: 2026-05-01 12:09:24 - 现在来更新主报告。"
// The index is the position in a.messages (0-based), which helps the LLM
// understand the conversation order even when context truncation is applied.
// System messages are not prefixed (they are always at index 0 and have no timestamp).
// startIdx is the index in a.messages where msgs[0] corresponds to.
// If startIdx < 0, the function falls back to content-based matching (legacy behavior).
func (a *Agent) addIndexPrefixToMessages(msgs []llm.Message, startIdx int) []llm.Message {
	result := make([]llm.Message, len(msgs))
	for i, msg := range msgs {
		// Determine the original index
		origIdx := -1
		if startIdx >= 0 {
			// Use sequential indexing starting from startIdx
			origIdx = startIdx + i
		} else {
			// Fallback: find the original index in a.messages by matching content and role
			a.mu.Lock()
			for j := range a.messages {
				if a.messages[j].Role == msg.Role && a.messages[j].Content == msg.Content {
					origIdx = j
					break
				}
			}
			a.mu.Unlock()
		}

		if origIdx >= 0 && msg.Role != "system" {
			// Add index prefix before the content
			result[i] = msg
			result[i].Content = fmt.Sprintf("%d: %s", origIdx, msg.Content)
		} else {
			result[i] = msg
		}
	}
	return result
}

// streamLLMResponse streams the LLM response and returns the complete content, reasoning, and tool calls.
// If streaming fails, it falls back to non-streaming Chat.
func (a *Agent) streamLLMResponse(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	// Apply context limit to messages
	contextMsgs := a.buildContextMessages()

	// Try streaming first
	eventCh, err := a.llmClient.ChatStream(ctx, contextMsgs, tools)
	if err != nil {
		// Fall back to non-streaming
		log.Debug("ChatStream not available, falling back to non-streaming: %v", err)
		return a.nonStreamingFallback(ctx, tools, cb)
	}

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var toolCalls []llm.ToolCall

	// Track whether we saw any tool call events (even invalid ones) from the stream.
	// This helps distinguish between "LLM returned no tool calls" (final answer)
	// and "LLM returned tool calls but all were invalid" (should retry).
	hasToolCallEvents := false

	// Collect details about invalid tool calls for better error reporting.
	type invalidToolCallInfo struct {
		Name   string
		ID     string
		Issues []string
	}
	var invalidCalls []invalidToolCallInfo

	// Filter function for tool calls that may have incomplete data from stream deltas
	// (e.g., empty name, ID, or arguments which would cause API errors)
	isValidToolCall := func(tc llm.ToolCall) bool {
		return tc.Name != "" && tc.ID != "" && tc.Arguments != ""
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
			hasToolCallEvents = true
			if event.ToolCall != nil {
				if isValidToolCall(*event.ToolCall) {
					toolCalls = append(toolCalls, *event.ToolCall)
				} else {
					// Collect details about why this tool call is invalid
					info := invalidToolCallInfo{
						Name: event.ToolCall.Name,
						ID:   event.ToolCall.ID,
					}
					if event.ToolCall.Name == "" {
						info.Issues = append(info.Issues, "name is empty")
					}
					if event.ToolCall.ID == "" {
						info.Issues = append(info.Issues, "ID is empty")
					}
					if event.ToolCall.Arguments == "" {
						info.Issues = append(info.Issues, "arguments is empty")
					}
					invalidCalls = append(invalidCalls, info)
				}
			}

		case llm.StreamEventDone:
			// Stream finished - tool calls are already accumulated from stream deltas.
			// No need for an extra non-streaming API call.
			finalContent := contentBuilder.String()
			finalReasoning := reasoningBuilder.String()

			// If the LLM intended to call tools but all were invalid (e.g., empty arguments),
			// treat this as an error so the agent can retry rather than returning empty content.
			// Provide detailed feedback about which tool calls were invalid and why.
			if hasToolCallEvents && len(toolCalls) == 0 {
				var sb strings.Builder
				sb.WriteString("LLM returned tool calls with invalid arguments (all filtered out). Details:\n")
				for _, ic := range invalidCalls {
					sb.WriteString(fmt.Sprintf("  - tool call"))
					if ic.Name != "" {
						sb.WriteString(fmt.Sprintf(" %q", ic.Name))
					}
					if ic.ID != "" {
						sb.WriteString(fmt.Sprintf(" (ID: %s)", ic.ID))
					}
					sb.WriteString(fmt.Sprintf(": %s\n", strings.Join(ic.Issues, ", ")))
				}
				sb.WriteString("Please check the tool definitions and ensure all required parameters are provided correctly.")
				return "", "", nil, errors.New(sb.String())
			}

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
	// Apply context limit to messages
	contextMsgs := a.buildContextMessages()
	resp, err := a.llmClient.Chat(ctx, contextMsgs, tools)
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

// readFileTool reads the contents of a file and returns it with line numbers.
func (a *Agent) readFileTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("readFileTool called: args=%v", args)
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
// It returns results with context lines, handles binary files, and enforces
// configurable limits on line length and total result size.
func (a *Agent) searchFilesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("searchFilesTool called: args=%v", args)
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

	// Get configurable limits from agent config
	maxLineLength := 8192
	maxResultBytes := 65536
	if a.cfg != nil {
		if a.cfg.LLM.SearchMaxLineLength > 0 {
			maxLineLength = a.cfg.LLM.SearchMaxLineLength
		}
		if a.cfg.LLM.SearchMaxResultBytes > 0 {
			maxResultBytes = a.cfg.LLM.SearchMaxResultBytes
		}
	}

	// Binary file extensions to skip
	binaryExts := map[string]bool{
		".exe": true, ".bin": true, ".o": true, ".a": true, ".so": true, ".dll": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".svg": true, ".webp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wav": true, ".flac": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".db": true, ".sqlite": true,
		".pyc": true, ".pyo": true, ".class": true, ".jar": true,
	}

	// Walk the directory
	var result strings.Builder
	var matchCount int
	var truncatedLineCount int
	var totalBytes int
	var headerWritten bool

	// Helper to write the header with match count info
	writeHeader := func() {
		if headerWritten {
			return
		}
		headerWritten = true
		if truncatedLineCount > 0 {
			result.WriteString(i18n.TF(i18n.KeySearchResultFoundTrunc, dirPath, matchCount, pattern, truncatedLineCount) + "\n\n")
		} else {
			result.WriteString(i18n.TF(i18n.KeySearchResultFound, dirPath, matchCount, pattern) + "\n\n")
		}
	}

	// Helper to write a line with truncation protection
	writeLine := func(line string) {
		if len(line) > maxLineLength {
			truncatedLineCount++
			line = line[:maxLineLength] + i18n.TF(i18n.KeySearchLineTruncated, len(line)-maxLineLength)
		}
		lineBytes := len(line) + 1 // +1 for newline
		if totalBytes+lineBytes > maxResultBytes {
			return // skip this line, we've hit the limit
		}
		result.WriteString(line + "\n")
		totalBytes += lineBytes
	}

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() {
			return nil
		}

		// Skip binary files by extension
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if binaryExts[ext] {
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

		// Detect binary content: check for null bytes in first 8KB
		checkLen := len(data)
		if checkLen > 8192 {
			checkLen = 8192
		}
		if bytes.IndexByte(data[:checkLen], 0) >= 0 {
			return nil // skip binary files
		}

		lines := strings.Split(string(data), "\n")
		fileMatched := false
		type matchInfo struct {
			lineNum int
			line    string
		}
		var fileMatches []matchInfo

		for i, line := range lines {
			if re.MatchString(line) {
				fileMatched = true
				fileMatches = append(fileMatches, matchInfo{lineNum: i + 1, line: line})
			}
		}

		if !fileMatched {
			return nil
		}

		// Check if we've hit the max result bytes limit before adding this file
		// Estimate: header + file name + context lines
		relPath, _ := filepath.Rel(dirPath, path)
		estimatedBytes := len(relPath) + 20 + len(fileMatches)*80
		if totalBytes+estimatedBytes > maxResultBytes && headerWritten {
			return filepath.SkipDir
		}

		// Write file header with context range
		writeHeader()
		firstLine := fileMatches[0].lineNum
		lastLine := fileMatches[len(fileMatches)-1].lineNum
		fileHeader := fmt.Sprintf("%s:%d-%d:", relPath, firstLine, lastLine)
		writeLine(fileHeader)

		// Determine context lines from config (default: 5)
		contextLines := 5
		if a.cfg != nil && a.cfg.LLM.SearchContextLines > 0 {
			contextLines = a.cfg.LLM.SearchContextLines
		}
		writtenLines := make(map[int]bool) // track which lines have been written to avoid duplicates
		for _, fm := range fileMatches {
			start := fm.lineNum - 1 - contextLines
			if start < 0 {
				start = 0
			}
			end := fm.lineNum - 1 + contextLines
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for i := start; i <= end; i++ {
				if writtenLines[i] {
					continue
				}
				writtenLines[i] = true
				contextLine := fmt.Sprintf("%d: %s", i+1, lines[i])
				writeLine(contextLine)
			}
		}
		writeLine("") // blank line between files

		matchCount += len(fileMatches)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if matchCount == 0 {
		return i18n.TF(i18n.KeySearchResultNone, pattern, dirPath), nil
	}

	// If we didn't write the header (shouldn't happen, but just in case)
	if !headerWritten {
		writeHeader()
	}

	// Check if we hit the byte limit
	if totalBytes >= maxResultBytes {
		// Remove the last incomplete line and add a truncation notice
		finalResult := result.String()
		lastNewline := strings.LastIndex(finalResult, "\n")
		if lastNewline >= 0 {
			finalResult = finalResult[:lastNewline]
		}
		// Find the last blank line separator to cleanly truncate
		lastSep := strings.LastIndex(finalResult, "\n\n")
		if lastSep >= 0 {
			finalResult = finalResult[:lastSep+1]
		}
		finalResult += i18n.TF(i18n.KeySearchResultFoundPartial, dirPath, matchCount, pattern) + "\n"
		return finalResult, nil
	}

	return result.String(), nil
}

// listCodeDefinitionNamesTool lists definition names in source code files at the top level of a directory.
func (a *Agent) listCodeDefinitionNamesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("listCodeDefinitionNamesTool called: args=%v", args)
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
	log.Debug("replaceInFileTool called: args=%v", args)
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
	log.Debug("writeToFileTool called: args=%v", args)
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

// addImagesTool adds image file paths to the image cache.
func (a *Agent) addImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("addImagesTool called: args=%v", args)
	pathsStr, ok := args["paths"].(string)
	if !ok {
		return "", fmt.Errorf("paths argument is required")
	}

	// Split by comma and trim spaces
	newPaths := strings.Split(pathsStr, ",")
	added := 0
	for _, p := range newPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check if already in cache
		exists := false
		for _, existing := range a.imagePaths {
			if existing == p {
				exists = true
				break
			}
		}
		if !exists {
			a.imagePaths = append(a.imagePaths, p)
			added++
		}
	}

	return fmt.Sprintf("✅ 已添加 %d 张图片到缓存（当前共 %d 张）", added, len(a.imagePaths)), nil
}

// removeImagesTool removes image file paths from the image cache.
func (a *Agent) removeImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("removeImagesTool called: args=%v", args)
	pathsStr, ok := args["paths"].(string)
	if !ok {
		return "", fmt.Errorf("paths argument is required")
	}

	// Split by comma and trim spaces
	removePaths := strings.Split(pathsStr, ",")
	removed := 0
	var remaining []string
	for _, p := range a.imagePaths {
		shouldRemove := false
		for _, rp := range removePaths {
			if p == strings.TrimSpace(rp) {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			removed++
		} else {
			remaining = append(remaining, p)
		}
	}
	a.imagePaths = remaining

	return fmt.Sprintf("✅ 已从缓存中移除 %d 张图片（当前共 %d 张）", removed, len(a.imagePaths)), nil
}

// clearImagesTool clears all cached image file paths.
func (a *Agent) clearImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("clearImagesTool called: args=%v", args)
	count := len(a.imagePaths)
	a.imagePaths = nil
	return fmt.Sprintf("✅ 已清空图片缓存（共移除 %d 张图片）", count), nil
}

// getMemorySliceTool retrieves a slice of conversation history from persistent memory.
func (a *Agent) getMemorySliceTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("getMemorySliceTool called: args=%v", args)
	lastFrom, ok := args["last_from"].(float64)
	if !ok {
		return "", fmt.Errorf("last_from argument is required")
	}
	lastTo, ok := args["last_to"].(float64)
	if !ok {
		return "", fmt.Errorf("last_to argument is required")
	}

	entries, err := a.memoryManager.GetHistorySlice(int(lastFrom), int(lastTo))
	if err != nil {
		return "", fmt.Errorf("cannot get history slice: %w", err)
	}

	formatted := memory.FormatHistorySlice(entries)
	fmt.Println(formatted)
	return formatted, nil
}

// memorySearchTool searches persistent conversation memory for messages matching given criteria.
func (a *Agent) memorySearchTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("memorySearchTool called: args=%v", args)
	params := memory.SearchParams{}

	// Apply config limits
	if a.cfg != nil {
		params.MaxResults = a.cfg.LLM.MemorySearchMaxResults
		params.MaxContentLen = a.cfg.LLM.MemorySearchMaxContentLen
	}

	// Parse keywords
	if keywordsRaw, ok := args["keywords"].([]interface{}); ok {
		params.Keywords = make([]string, 0, len(keywordsRaw))
		for _, kw := range keywordsRaw {
			if kwStr, ok := kw.(string); ok {
				params.Keywords = append(params.Keywords, kwStr)
			}
		}
	}

	// Parse since time
	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		since, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return "", fmt.Errorf("invalid since time format (use ISO 8601, e.g. '2026-04-01T00:00:00Z'): %w", err)
		}
		params.Since = since
	}

	// Parse name filter
	if name, ok := args["name"].(string); ok {
		params.Name = name
	}

	results, err := a.memoryManager.Search(params)
	if err != nil {
		return "", fmt.Errorf("memory search failed: %w", err)
	}

	maxContentLen := 0
	if a.cfg != nil {
		maxContentLen = a.cfg.LLM.MemorySearchMaxContentLen
	}
	formatted := memory.FormatSearchResults(results, maxContentLen)
	fmt.Println(formatted)
	return formatted, nil
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
	log.Debug("launchSubAgentTool called: args=%v", args)
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
		Workspace:         workspacePath,
		Instruction:       instruction,
		TimeoutSeconds:    timeout,
		Purpose:           purpose,
		ImagePaths:        a.imagePaths,
		ConfirmCommandOff: a.approveAll,
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
	log.Debug("scheduleTaskTool called: args=%v", args)
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

// createTaskPlanTool creates a new task plan with title, description, and steps.
// If there is an existing plan with unfinished steps, it returns an error.
// If there is an existing plan (all completed), it is archived to memory first.
// After creating the plan, needAdjustPointer is set to true so the agent loop
// will adjust messagePointer after all tool messages have been appended.
// The checklist content is returned as the tool result (visible to LLM),
// but no extra assistant message is inserted to avoid breaking the
// assistant(tool_calls) -> tool message sequence required by the API.
func (a *Agent) createTaskPlanTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("createTaskPlanTool called: args=%v", args)
	title, ok := args["title"].(string)
	if !ok {
		return "", fmt.Errorf("title argument is required")
	}

	description, _ := args["description"].(string)

	stepsRaw, ok := args["steps"].([]interface{})
	if !ok {
		return "", fmt.Errorf("steps argument is required and must be an array of strings")
	}

	steps := make([]string, 0, len(stepsRaw))
	for _, s := range stepsRaw {
		stepStr, ok := s.(string)
		if !ok {
			return "", fmt.Errorf("each step must be a string")
		}
		steps = append(steps, stepStr)
	}

	plan, err := a.taskPlanMgr.Create(title, description, steps)
	if err != nil {
		return "", fmt.Errorf("cannot create task plan: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// updateTaskStepTool updates the status of a specific step in the current task plan.
func (a *Agent) updateTaskStepTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("updateTaskStepTool called: args=%v", args)
	stepID, ok := args["step_id"].(float64)
	if !ok {
		return "", fmt.Errorf("step_id argument is required")
	}

	statusStr, ok := args["status"].(string)
	if !ok {
		return "", fmt.Errorf("status argument is required")
	}

	note, _ := args["note"].(string)

	status := taskplan.TaskStatus(statusStr)
	plan, err := a.taskPlanMgr.UpdateStepStatus(int(stepID), status, note)
	if err != nil {
		return "", fmt.Errorf("cannot update step status: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)
	return formatted, nil
}

// insertTaskStepsTool inserts new steps after a specified step in the current task plan.
// After inserting steps, needAdjustPointer is set to true so the agent loop
// will adjust messagePointer after all tool messages have been appended.
// The updated checklist content is returned as the tool result (visible to LLM),
// but no extra assistant message is inserted to avoid breaking the
// assistant(tool_calls) -> tool message sequence required by the API.
func (a *Agent) insertTaskStepsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("insertTaskStepsTool called: args=%v", args)
	afterStepID, ok := args["after_step_id"].(float64)
	if !ok {
		return "", fmt.Errorf("after_step_id argument is required")
	}

	stepsRaw, ok := args["steps"].([]interface{})
	if !ok {
		return "", fmt.Errorf("steps argument is required and must be an array of strings")
	}

	steps := make([]string, 0, len(stepsRaw))
	for _, s := range stepsRaw {
		stepStr, ok := s.(string)
		if !ok {
			return "", fmt.Errorf("each step must be a string")
		}
		steps = append(steps, stepStr)
	}

	plan, err := a.taskPlanMgr.InsertStepsAfter(int(afterStepID), steps)
	if err != nil {
		return "", fmt.Errorf("cannot insert steps: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// removeTaskStepsTool removes steps from the current task plan by step ID range.
// After removing steps, needAdjustPointer is set to true so the agent loop
// will adjust messagePointer after all tool messages have been appended.
// The updated checklist content is returned as the tool result (visible to LLM),
// but no extra assistant message is inserted to avoid breaking the
// assistant(tool_calls) -> tool message sequence required by the API.
func (a *Agent) removeTaskStepsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("removeTaskStepsTool called: args=%v", args)
	from, ok := args["from"].(float64)
	if !ok {
		return "", fmt.Errorf("from argument is required")
	}

	to, ok := args["to"].(float64)
	if !ok {
		return "", fmt.Errorf("to argument is required")
	}

	plan, err := a.taskPlanMgr.RemoveSteps(int(from), int(to))
	if err != nil {
		return "", fmt.Errorf("cannot remove steps: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// listTaskPlansTool shows the current task plan.
func (a *Agent) listTaskPlansTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("listTaskPlansTool called")
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("cannot get current task plan: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)
	return formatted, nil
}

// viewTaskPlanTool views the full details of the current task plan.
func (a *Agent) viewTaskPlanTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("viewTaskPlanTool called")
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("cannot get current task plan: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)
	return formatted, nil
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

// adjustMessagePointer moves the messagePointer back past any tool messages
// to ensure the LLM sees a clean context starting from a non-tool message.
// This is called after setting messagePointer to a new position (e.g., after
// creating/updating a checklist). If the pointer position is preceded by tool
// messages, the pointer is moved further back to the first non-tool message.
// Caller must hold a.mu lock.
func (a *Agent) adjustMessagePointer() {
	for a.messagePointer > 0 && a.messages[a.messagePointer].Role == "tool" {
		a.messagePointer--
	}
}
