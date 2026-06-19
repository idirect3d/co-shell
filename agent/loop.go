// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-05-22
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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/browser"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/memory"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/shell"
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
	CmdConfirmApproveG                      // Approve and disable confirmation for this tool (G option)
	CmdConfirmApproveD                      // Permanently disable this tool (D option)
	CmdConfirmCancel                        // User cancelled, return to REPL
	CmdConfirmModify                        // User entered custom input to modify the command
)

// Agent is the core AI agent that orchestrates tool calls and LLM interactions.
type Agent struct {
	mu            sync.Mutex
	llmClient     llm.Client
	mcpMgr        *mcp.Manager
	store         *store.DualStore
	memoryManager *memory.Manager
	systemPrompt  string
	messages      []llm.Message
	maxIterations int
	// toolModes stores per-tool mode settings.
	// Key is the tool name, "default" is the default for all tools.
	// Value is one of: "disabled" (not sent to LLM), "confirm" (enabled, requires user confirmation),
	// "auto" (enabled, auto-approved without confirmation).
	// If a tool is not in the map, the default mode is "confirm".
	toolModes    map[string]string
	approveAll   bool // if true, skip confirmation for all commands in this request
	approveCount int  // remaining number of commands to auto-approve (decremented on each use)
	// Per-tool confirmation state
	toolApproveCounts  map[string]int  // remaining auto-approve count per tool name
	toolDisableConfirm map[string]bool // tools where confirmation is disabled via G option

	cfg          *config.Config // configuration for timeout settings
	resultMode   config.ResultMode
	modelManager *config.ModelManager // model manager for multi-model switching

	// Output control switches (ENHANCEMENT-126)
	showLlmThinking   bool
	showLlmContent    bool
	showTool          bool
	showToolInput     bool
	showToolOutput    bool
	showCommand       bool
	showCommandOutput bool

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

	emojiEnabled    bool // whether emoji prefixes are enabled for output
	toolCallEnabled bool // whether tool calling is enabled
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

	// Token usage statistics
	totalPromptTokens     int // accumulated prompt tokens across all LLM calls
	totalCompletionTokens int // accumulated completion tokens across all LLM calls
	totalTokens           int // accumulated total tokens across all LLM calls

	// completed is set to true when attempt_completion is called.
	// RunStream checks this before treating 0-tool-call as final answer.
	completed bool

	// Loop detection (FIX-179)
	loopDetector   *LoopDetector // monitors LLM output for repeating patterns
	loopDetectOn   bool          // whether loop detection is enabled for current request
	loopDetectCrit bool          // set to true when loop intervention occurs

	// Loop temperature controller (FEATURE-230)
	// Automatically adjusts LLM temperature when a loop is detected.
	// Re-initialized at the start of each RunStream call.
	loopTempCtrl *LoopTempController

	// ToolCallModeMgr manages tool call mode (openai/xml/custom)
	toolCallModeMgr *ToolCallModeMgr

	// lastUserInput stores the raw user instruction (before formatUserMessage formatting)
	// for use as {TASK} in the system prompt Objective section.
	lastUserInput string

	// Persistent shell session for interactive command execution (FEATURE-192)
	shellSession     *shell.Session
	shellEnabled     bool   // whether persistent shell tools are enabled
	shellSessionMode string // "confirm" or "auto" - user confirmation mode

	// Browser automation via CDP (FEATURE-200)
	chromeMgr             *browser.ChromeManager
	browserEnabled        bool   // whether browser tools are enabled
	browserScreenshotData string // cached base64 screenshot data for multimodal context

	// Interrupt channel for ESC key (FEATURE-201)
	interruptCh chan struct{} // signals LLM stream to stop

	// Cancel channel for Ctrl+C (FEATURE-239)
	// When signaled, the agent immediately exits the current iteration
	// and returns to the REPL prompt without confirmation.
	cancelCh chan struct{} // signals immediate cancellation

	// UserIO for terminal interaction (FEATURE-201 fix)
	io UserIO

	// commandRunning is set to true while a system command is executing with
	// stdin connected. The ESC monitor goroutine checks this flag to avoid
	// competing with the sub-process for stdin reads (FIX-209).
	commandRunning bool
}

// SetCommandRunning sets a flag indicating whether a system command is currently
// being executed with stdin connected (e.g. sudo, passwd). When true, the ESC
// monitor goroutine skips polling stdin to avoid stealing input bytes from
// the sub-process (FIX-209).
func (a *Agent) SetCommandRunning(running bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commandRunning = running
}

// IsCommandRunning returns true if a system command is currently executing
// with stdin connected. The ESC monitor should skip polling stdin when
// this is true to avoid data races on stdin with the sub-process.
func (a *Agent) IsCommandRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.commandRunning
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
func (a *Agent) buildContextMessages() []llm.Message {
	var msgs []llm.Message

	if a.cfg != nil && a.cfg.LLM.ContextLimit != -1 {
		// Apply context limit: truncate history, keep system + history + current
		if len(a.messages) <= 1 {
			msgs = a.messages
		} else {
			systemMsg := a.messages[0]
			currentMsg := a.messages[len(a.messages)-1]

			startIdx := 1
			if a.messagePointer > 0 && a.messagePointer < len(a.messages) {
				startIdx = a.messagePointer
			}

			historyMsgs := a.messages[startIdx : len(a.messages)-1]

			if a.cfg.LLM.ContextLimit > 0 && len(historyMsgs) > a.cfg.LLM.ContextLimit {
				historyMsgs = historyMsgs[len(historyMsgs)-a.cfg.LLM.ContextLimit:]
			}

			msgs = make([]llm.Message, 0, 2+len(historyMsgs))
			msgs = append(msgs, systemMsg)
			msgs = append(msgs, historyMsgs...)
			msgs = append(msgs, currentMsg)
		}
	} else {
		// Unlimited: use all messages (copy to avoid modifying originals)
		msgs = a.addIndexPrefixToMessages(a.messages, 0)
	}

	// Strip old <environment_details> blocks from all messages,
	// then inject fresh envelope into the last user message.
	msgs = a.stripEnvelopes(msgs)
	msgs = a.injectEnvelopeToLastUser(msgs)
	return msgs
}

// addIndexPrefixToMessages returns the messages as-is, without adding index prefixes.
// Previously this function added "index: content" prefix to help LLM understand
// conversation order, but this was removed because it interfered with message content.
// The function is kept for backwards compatibility and may be removed in the future.
func (a *Agent) addIndexPrefixToMessages(msgs []llm.Message, startIdx int) []llm.Message {
	result := make([]llm.Message, len(msgs))
	copy(result, msgs)
	return result
}

// defaultIO is a package-level fallback for output operations before SetIO is called.
var defaultIO UserIO = &fmtIO{}

// fmtIO is a minimal UserIO that delegates output to fmt package.
// Used as the default before the REPL sets a proper UserIO.
type fmtIO struct{}

func (f *fmtIO) Print(args ...interface{})                 { fmt.Print(args...) }
func (f *fmtIO) Printf(fmtStr string, args ...interface{}) { fmt.Printf(fmtStr, args...) }
func (f *fmtIO) Println(args ...interface{})               { fmt.Println(args...) }
func (f *fmtIO) ErrPrintf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}
func (f *fmtIO) ReadLine() (string, error) { return "", nil }
func (f *fmtIO) ReadKey() (byte, error)    { return 0, nil }
func (f *fmtIO) IsReading() bool           { return false }

// nonStreamingFallback handles the case when streaming is not available.
func (a *Agent) nonStreamingFallback(ctx context.Context, tools []llm.Tool, cb StreamCallback) (string, string, []llm.ToolCall, error) {
	// Apply context limit to messages
	contextMsgs := a.buildContextMessages()
	resp, err := a.llmClient.Chat(ctx, contextMsgs, tools)
	if err != nil {
		return "", "", nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Accumulate token usage from API response
	if resp.Usage != nil {
		a.mu.Lock()
		a.totalPromptTokens += resp.Usage.PromptTokens
		a.totalCompletionTokens += resp.Usage.CompletionTokens
		a.totalTokens += resp.Usage.TotalTokens
		// Persist token usage to database
		if a.store != nil {
			entry := &store.TokenUsageEntry{
				ID:               fmt.Sprintf("%020d", time.Now().UnixNano()),
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
				Timestamp:        time.Now(),
			}
			if err := a.store.SaveTokenUsage(entry); err != nil {
				log.Warn("Failed to save token usage: %v", err)
			}
		}
		a.mu.Unlock()
		log.Debug("Agent.nonStreamingFallback: accumulated token usage: prompt=%d, completion=%d, total=%d",
			a.totalPromptTokens, a.totalCompletionTokens, a.totalTokens)
	}

	if a.showLlmThinking && resp.ReasoningContent != "" {
		cb("thinking", resp.ReasoningContent)
	}

	// In XML mode, the LLM returns tool calls embedded in the content as XML tags.
	// We ALWAYS parse XML tool calls from content in XML mode, and IGNORE any
	// API-level tool_calls. This prevents conflicts where the LLM returns both
	// XML tool calls in content AND API-level tool_calls simultaneously.
	toolCalls := resp.ToolCalls
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Current()
		if mode != nil && !mode.SendTools {
			xmlCalls := ParseXMLToolCalls(resp.Content)
			if len(xmlCalls) > 0 {
				// Filter out _xml_parse_error calls - these are parse errors that
				// should be returned directly to the LLM as feedback, not executed.
				var validCalls []llm.ToolCall
				var parseErrors []string
				for _, c := range xmlCalls {
					if c.Name == "_xml_parse_error" {
						var args map[string]interface{}
						if err := json.Unmarshal([]byte(c.Arguments), &args); err == nil {
							if errMsg, ok := args["error"].(string); ok {
								parseErrors = append(parseErrors, errMsg)
							}
						}
					} else {
						validCalls = append(validCalls, c)
					}
				}
				if len(parseErrors) > 0 {
					// Return parse errors directly to the LLM as assistant content,
					// so it can see and fix the format issues immediately.
					content := strings.Join(parseErrors, "\n---\n")
					toolCalls = nil
					log.Debug("Agent.nonStreamingFallback: returning %d XML parse errors to LLM as content (no tool calls)",
						len(parseErrors))
					return content, resp.ReasoningContent, nil, nil
				}
				toolCalls = validCalls
				log.Debug("Agent.nonStreamingFallback: parsed %d XML tool calls from content (ignored %d API-level tool calls)",
					len(validCalls), len(toolCalls))
			} else {
				// No XML tool calls found; clear any API-level tool calls in XML mode
				toolCalls = nil
			}
		}
	}

	return resp.Content, resp.ReasoningContent, toolCalls, nil
}

// TokenUsage returns the accumulated token usage statistics.
// Returns prompt tokens, completion tokens, and total tokens.
func (a *Agent) TokenUsage() (prompt, completion, total int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalPromptTokens, a.totalCompletionTokens, a.totalTokens
}

// ResetTokenUsage resets the accumulated token usage statistics to zero.
func (a *Agent) ResetTokenUsage() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totalPromptTokens = 0
	a.totalCompletionTokens = 0
	a.totalTokens = 0
}
