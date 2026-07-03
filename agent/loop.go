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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/browser"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
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
	imagePaths      []string // paths to image files for multimodal input (cleared after one-shot delivery)
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
	totalPromptTokens     int // accumulated prompt tokens across all LLM calls (session level)
	totalCompletionTokens int // accumulated completion tokens across all LLM calls
	totalTokens           int // accumulated total tokens across all LLM calls

	// Task-level token usage (reset per RunStream)
	taskPromptTokens     int // prompt tokens for the current task
	taskCompletionTokens int // completion tokens for the current task
	taskTokens           int // total tokens for the current task

	// Per-iteration token delta tracking
	iterPromptTokens     int // prompt tokens for the current iteration (most recent LLM call)
	iterCompletionTokens int // completion tokens for the current iteration
	iterTokens           int // total tokens for the current iteration

	// LLM performance timing (per-call, reset before each ChatStream)
	llmCallStartTime time.Time // when ChatStream/Chat is initiated
	firstTokenTime   time.Time // when first content/thinking token arrives
	llmStreamEndTime time.Time // when stream completes
	prevPromptTokens int       // prompt tokens from previous call for computing per-call delta
	prevTotalTokens  int       // total tokens from previous call for computing per-call delta

	// completed is set to true when attempt_completion is called.
	// RunStream checks this before treating 0-tool-call as final answer.
	completed bool

	// Loop detection (FIX-179 / FIX-240)
	loopDetector         *LoopDetector         // monitors LLM output for repeating patterns
	loopDetectOn         bool                  // whether loop detection is enabled for current request
	loopDetectCrit       bool                  // set to true when loop intervention occurs
	toolCallLoopDetector *ToolCallLoopDetector // monitors repeated identical tool calls across iterations

	// Loop temperature controller (FEATURE-230)
	// Automatically adjusts LLM temperature when a loop is detected.
	// Re-initialized at the start of each RunStream call.
	loopTempCtrl *LoopTempController

	// Asynchronous loop judgment state (FEATURE-241)
	// When LoopJudgeEnabled is on, the loop detection during streaming does NOT
	// immediately interrupt. Instead, it fires a goroutine to call the judge model
	// while the stream continues. The result is checked after the stream completes.
	loopJudgeInflight      bool             // true while async judgment is in progress
	loopJudgePendingResult *LoopJudgeResult // set by goroutine when judgment completes
	loopJudgeResultCh      chan struct{}    // closed when result is ready
	loopJudgeTriggered     bool             // true if loop was detected during this stream call

	// loopLongOutputTriggered is set to true when the streaming output exceeds
	// LoopLongOutputThreshold during the current stream call. Prevents multiple
	// judge triggers for the same long output chunk. Reset at the start of each
	// streamLLMResponse call.
	loopLongOutputTriggered bool

	// loopDetectSyncErr stores the loop detection error for the sync (non-judge) path.
	// When LoopJudgeEnabled is false, handleLoopDetection sets this and the stream
	// event loop checks it to break out immediately.
	loopDetectSyncErr error

	// streamCb stores the active StreamCallback during streaming, so that
	// handleLoopDetection() can display progress via the callback which
	// properly handles raw terminal \r\n conversion.
	streamCb StreamCallback

	// ToolCallModeMgr manages tool call mode (openai/xml/custom)
	toolCallModeMgr *ToolCallModeMgr

	// lastUserInput stores the raw user instruction (before formatUserMessage formatting)
	// for use as {TASK} in the system prompt Objective section.
	lastUserInput string

	// lastLlmOutput stores the complete content of the most recent LLM response.
	// Used by the loop judgment mechanism (judgeLoop) to provide the full context
	// of the suspected loop iteration. Updated at the end of each LLM call.
	// (FEATURE-241)
	lastLlmOutput string

	// lastAssistantContent stores the complete assistant response content from the
	// previous iteration. Used to detect exact content duplicates — when the LLM
	// returns the exact same content without calling any tools, it indicates a
	// "stuck" state that needs different feedback. (FEATURE-249)
	lastAssistantContent string

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

	// debugMode: when enabled, displays messages to be sent to LLM on the prompt
	// line for review and editing before sending.
	debugMode bool

	// UserIO for terminal interaction (FEATURE-201 fix)
	io UserIO

	// commandRunning is set to true while a system command is executing with
	// stdin connected. The ESC monitor goroutine checks this flag to avoid
	// competing with the sub-process for stdin reads (FIX-209).
	commandRunning bool

	// taskInstructionCache collects user supplementary instructions and other
	// task-level hints (e.g., context overflow warnings) during tool execution.
	// At the end of each iteration, all cached content is flushed as a single
	// <task> ContentPart appended to the last user message. (FEATURE-255)
	taskInstructionCache bytes.Buffer
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

// buildContextMessages returns a truncated message list based on ContextLimit, messagePointer,
// and ContextStartMode.
// Message layout: [0]=system, [1..n-2]=history, [n-1]=current user input
// The current user input (last message) is ALWAYS kept.
//
// Mode-specific behavior:
//
//	"window": fixed window — ContextLimit controls window size. Respects messagePointer
//	          for the start position, then truncates to last N messages.
//	"task":   full history — ContextLimit is ignored. messagePointer follows task plan
//	          boundaries automatically.
//	"smart":  full history — ContextLimit is ignored. messagePointer is only adjusted
//	          via attempt_completion's task_message_no; task plan changes do NOT move it.
func (a *Agent) buildContextMessages() []llm.Message {
	var msgs []llm.Message

	contextStartMode := "smart"
	if a.cfg != nil && a.cfg.LLM.ContextPolicy != "" {
		contextStartMode = a.cfg.LLM.ContextPolicy
	}

	// Only "window" mode uses ContextLimit for truncation.
	// "task" and "smart" modes always use full history (unlimited).
	effectiveContextLimit := a.cfg.LLM.ContextLimit
	if a.cfg != nil && contextStartMode != "window" {
		effectiveContextLimit = -1 // unlimited
	}

	if a.cfg != nil && effectiveContextLimit != -1 {
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

			if effectiveContextLimit > 0 && len(historyMsgs) > effectiveContextLimit {
				historyMsgs = historyMsgs[len(historyMsgs)-effectiveContextLimit:]
			}

			msgs = make([]llm.Message, 0, 2+len(historyMsgs))
			msgs = append(msgs, systemMsg)
			msgs = append(msgs, historyMsgs...)
			msgs = append(msgs, currentMsg)
		}
	} else {
		// Unlimited: use messages from messagePointer onwards, respecting pointer position.
		// When context-policy is "reorganize", the messagePointer is moved by reorganize_context
		// to skip old history - this must be honored here.
		if len(a.messages) <= 1 {
			msgs = a.messages
		} else {
			startIdx := 1
			if a.messagePointer > 1 && a.messagePointer < len(a.messages) {
				startIdx = a.messagePointer
			}
			systemMsg := a.messages[0]
			historyMsgs := a.messages[startIdx:]
			msgs = make([]llm.Message, 0, 1+len(historyMsgs))
			msgs = append(msgs, systemMsg)
			msgs = append(msgs, historyMsgs...)
		}
	}

	// Note: <environment_details> is NOT injected here — it was already attached
	// when each message was first created (see buildUserMessage, buildXMLToolResultMessage,
	// and the tool result creation paths in run_stream.go/run.go).
	// This ensures the envelope is frozen at message creation time and does not
	// change or accumulate across LLM iterations.

	// Inject cached images into the last user message dynamically.
	// When add_images has been called by the LLM, image paths are stored in
	// a.imagePaths. We read and encode them here so that every LLM call sees
	// the actual image data as ContentParts appended to the last user message.
	// The images are injected only into the returned msgs slice — it does
	// NOT pollute a.messages (the persistent history).
	// No text is injected — the add_images tool result already carries the
	// recognition intent as text.
	if len(a.imagePaths) > 0 && len(msgs) > 0 {
		lastIdx := len(msgs) - 1
		lastMsg := msgs[lastIdx]
		if lastMsg.Role == "user" {
			// Read and encode each cached image, append as ContentPart
			for _, imgPath := range a.imagePaths {
				// Resolve relative paths
				absPath := imgPath
				if !filepath.IsAbs(imgPath) {
					cwd, err := os.Getwd()
					if err != nil {
						log.Warn("buildContextMessages: cannot get cwd for image %q: %v", imgPath, err)
						continue
					}
					absPath = filepath.Join(cwd, imgPath)
				}

				// Read image file
				data, err := os.ReadFile(absPath)
				if err != nil {
					log.Warn("buildContextMessages: cannot read image %q: %v", imgPath, err)
					continue
				}

				// Detect MIME type
				ext := strings.ToLower(filepath.Ext(absPath))
				mimeType := "image/png"
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
				}

				// Encode as base64 data URI
				base64Data := base64.StdEncoding.EncodeToString(data)
				dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

				lastMsg.ContentParts = append(lastMsg.ContentParts, llm.ContentPart{
					Type: llm.ContentPartImageURL,
					ImageURL: &llm.ContentPartImage{
						URL:    dataURI,
						Detail: "auto",
					},
				})
			}
			msgs[lastIdx] = lastMsg
		}

		// One-shot: clear image paths after injection so they are not re-sent
		a.imagePaths = nil
	}

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

// isXMLMode returns true if the current tool call mode is XML (no API-level tool calls).
func (a *Agent) isXMLMode() bool {
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Current()
		return mode != nil && !mode.SendTools
	}
	return false
}

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

// getLoopJudgeModel returns the model config to use for loop judgment.
// Priority:
//  1. Current WorkMode's ProblemModelID (if set)
//  2. Current WorkMode's ModelID (fallback, text model)
//  3. Current active model (final fallback)
func (a *Agent) getLoopJudgeModel() *config.ModelConfig {
	// Priority 1 & 2: mode-bound model (ProblemModelID or ModelID)
	modelID := a.getProblemModelID()
	if modelID != "" {
		// Look up by ID in cfg.Models
		if a.cfg != nil {
			for _, m := range a.cfg.Models {
				if m.ID == modelID && m.Enabled {
					log.Debug("getLoopJudgeModel: using mode-bound model %q", modelID)
					return m
				}
			}
		}
		// Fallback: try ModelManager
		if a.modelManager != nil {
			if m := a.modelManager.GetModel(modelID); m != nil && m.Enabled {
				log.Debug("getLoopJudgeModel: using mode-bound model %q (from ModelManager)", modelID)
				return m
			}
		}
	}

	// Priority 3: current active model
	if a.modelManager != nil {
		current := a.modelManager.GetActiveModel(false)
		if current != nil {
			log.Debug("getLoopJudgeModel: falling back to current active model %q", current.ID)
			return current
		}
	}

	log.Warn("getLoopJudgeModel: no model found for loop judgment")
	return nil
}

// judgeLoop uses an independent LLM model to perform secondary judgment
// on suspected loop content. It builds a clean minimal context without
// system prompt noise, and expects a JSON-formatted judgment result.
func (a *Agent) judgeLoop(ctx context.Context, err error, suspectContent string) *LoopJudgeResult {
	if a.cfg == nil || !a.cfg.LLM.LoopJudgeEnabled {
		log.Debug("judgeLoop: loop judgment disabled, returning nil")
		return nil
	}

	modelCfg := a.getLoopJudgeModel()
	if modelCfg == nil {
		log.Warn("judgeLoop: no model available for loop judgment, skipping")
		return nil
	}

	// Build task plan text
	taskPlanText := a.getTaskPlanPrompt()
	if taskPlanText == "" {
		taskPlanText = "（无活跃任务计划 / No active task plan）"
	}

	// Determine the type of loop: content or tool call
	loopType := "content"
	if _, isToolCallLoop := err.(*ToolCallLoopDetectedError); isToolCallLoop {
		loopType = "tool_call"
	}

	// Build the clean judgment context (system prompt + user message)
	systemText := i18n.T(i18n.KeyLoopJudgeSystemPrompt)
	userText := a.buildLoopJudgeUserPrompt(taskPlanText, suspectContent)

	log.Debug("judgeLoop: using model=%q, suspectContent=%d chars, loopType=%s",
		modelCfg.ID, len(suspectContent), loopType)

	// Display the full user prompt via streamCb before calling the judge API
	if cb := a.streamCb; cb != nil {
		showDetail := a.cfg == nil || a.cfg.LLM.ShowLoopDetection
		if showDetail {
			cb("info", "发送给判定模型的完整提示词:\n"+strings.TrimSpace(userText)+"\n")
		}
	}

	// Resolve judge timeout: from config, default 60s, 0 = no timeout
	judgeTimeout := 60
	if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout > 0 {
		judgeTimeout = a.cfg.LLM.LoopJudgeTimeout
	} else if a.cfg != nil && a.cfg.LLM.LoopJudgeTimeout == 0 {
		judgeTimeout = 0 // 0 = no timeout
	}

	// Create a temporary LLM client for the judgment model.
	// Use 1024 max_tokens to leave room for the judge model to produce JSON output
	// without needing to output thinking text first.
	// Use an independent HTTP transport (DisableKeepAlives=true) so this request
	// is NOT blocked by an active streaming connection in Go's default connection pool.
	judgeClient := llm.NewClient(
		modelCfg.Endpoint,
		modelCfg.APIKey,
		modelCfg.Model,
		0.0,  // low temperature for deterministic judgment
		8192, // max_tokens: allow enough room for JSON output
		judgeTimeout,
	)
	if oc, ok := judgeClient.(interface{ SetHTTPClient(*http.Client) }); ok {
		oc.SetHTTPClient(&http.Client{
			Transport: &http.Transport{
				MaxConnsPerHost:     1,
				MaxIdleConnsPerHost: 0,
				IdleConnTimeout:     0,
				DisableKeepAlives:   true,
			},
			Timeout: time.Duration(judgeTimeout) * time.Second,
		})
		log.Debug("judgeLoop: using independent HTTP transport for judge client")
	}
	if judgeClient != nil {
		defer judgeClient.Close()
	}

	// Resolve temperature: model-level has priority
	finalTemp := 0.3
	if modelCfg.Temperature != nil {
		finalTemp = *modelCfg.Temperature
	}
	if a.cfg.LLM.Temperature != 0 {
		finalTemp = a.cfg.LLM.Temperature
	}
	judgeClient.SetTemperature(finalTemp)
	// Disable thinking/reasoning for the judgment call so the model outputs
	// pure JSON directly without a reasoning preamble.
	judgeClient.SetThinkingEnabled(false)

	// Build messages
	messages := []llm.Message{
		{Role: "system", Content: systemText},
		{Role: "user", Content: userText},
	}

	// Log the judgment request for debugging
	log.Info("LoopJudge request: model=%q, system=%d chars, user=%d chars, suspectContent=%d chars",
		modelCfg.ID, len(systemText), len(userText), len(suspectContent))
	log.Debug("LoopJudge request detail: system=%q, user=%q", systemText, userText)

	// Make the judgment call (non-streaming, no tools)
	ctxTimeout := judgeTimeout + 5 // ctx timeout slightly larger than client timeout
	if judgeTimeout <= 0 {
		ctxTimeout = 0 // no timeout
	}
	var ctxWithTimeout context.Context
	var cancel context.CancelFunc
	if ctxTimeout > 0 {
		ctxWithTimeout, cancel = context.WithTimeout(ctx, time.Duration(ctxTimeout)*time.Second)
	} else {
		ctxWithTimeout, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	resp, err := judgeClient.Chat(ctxWithTimeout, messages, nil)
	if err != nil {
		log.Warn("judgeLoop: judgment call failed: %v, falling back to direct feedback", err)
		return nil
	}

	// Log the judge model's raw response for debugging
	log.Info("LoopJudge response: model=%q, resp_content=%d chars", modelCfg.ID, len(resp.Content))
	log.Debug("LoopJudge response detail: raw=%q", resp.Content)

	// Display the judge model's full response
	if cb := a.streamCb; cb != nil {
		showDetail := a.cfg == nil || a.cfg.LLM.ShowLoopDetection
		if showDetail {
			cb("info", "判定模型的完整返回:\n"+strings.TrimSpace(resp.Content)+"\n")
		}
	}

	// Parse JSON response
	result := &LoopJudgeResult{}
	content := strings.TrimSpace(resp.Content)

	// Try to extract JSON from the response (may be wrapped in markdown code blocks)
	if idx := strings.Index(content, "{"); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "}"); idx >= 0 {
		content = content[:idx+1]
	}

	if err := json.Unmarshal([]byte(content), result); err != nil {
		log.Warn("judgeLoop: failed to parse JSON response: %v, content=%q", err, content)
		log.Info("LoopJudge result: parse FAILED, falling back to direct feedback")
		return nil
	}

	log.Info("LoopJudge result: is_loop=%v, reason=%q, exit_strategy=%q",
		result.IsLoop, result.Reason, result.ExitStrategy)

	return result
}

// handleLoopDetection is called when a loop pattern is detected during streaming.
// It synchronously calls the judge model (if enabled) to confirm the loop,
// then sets loopDetectSyncErr to interrupt the stream if confirmed.
//
// Judgment result handling:
//   - Judge enabled AND confirmed loop → set syncErr (stream will break)
//   - Judge enabled AND NOT confirmed → reset detectors (stream continues)
//   - Judge disabled → always set syncErr (stream breaks immediately)
func (a *Agent) handleLoopDetection(content, reasoning string, detectErr error) {
	a.mu.Lock()

	// Save the accumulated content for judgment
	if reasoning != "" {
		a.lastLlmOutput = reasoning
	} else {
		a.lastLlmOutput = content
	}

	useJudge := a.cfg != nil && a.cfg.LLM.LoopJudgeEnabled

	if !useJudge {
		// No judge: always interrupt immediately.
		a.loopDetectSyncErr = detectErr
		a.loopDetectCrit = true
		a.mu.Unlock()
		log.Debug("handleLoopDetection: sync mode, set loopDetectSyncErr")
		return
	}

	// Show progress BEFORE the synchronous judge API call.
	// Use direct stdout write with \r\n to bypass potential stream callback buffering
	// in raw terminal mode.
	io := a.defaultIO()
	io.Println()
	io.Println("────────────────────────────────────────────")
	io.Println("检测到疑似循环内容...")

	// Judge mode: synchronously call judgeLoop.
	// (judgeLoop will display the full user prompt via streamCb)
	// Release lock to avoid holding during the API call.
	suspectContent := a.lastLlmOutput
	a.mu.Unlock()

	log.Debug("handleLoopDetection: judge mode, calling judgeLoop synchronously")
	ctx := context.Background()
	result := a.judgeLoop(ctx, detectErr, suspectContent)

	// Show judge result after API completes
	cb := a.streamCb
	if cb != nil {
		if result != nil && result.IsLoop {
			cb("info", fmt.Sprintf("判定模型返回: is_loop=true, reason=%q, exit_strategy=%q\n", result.Reason, result.ExitStrategy))
		} else if result != nil && !result.IsLoop {
			cb("info", fmt.Sprintf("判定模型返回: is_loop=false, reason=%q\n", result.Reason))
		} else {
			cb("info", "判定模型返回: 失败/超时\n")
		}
	}

	if result != nil && result.IsLoop {
		// Judge confirmed loop: interrupt the stream.
		a.mu.Lock()
		a.loopDetectSyncErr = detectErr
		a.loopDetectCrit = true
		a.mu.Unlock()
		log.Debug("handleLoopDetection: judge confirmed loop, set loopDetectSyncErr")
	} else if result != nil && !result.IsLoop {
		// Judge explicitly NOT a loop: reset detectors, stream continues.
		log.Debug("handleLoopDetection: judge says NOT a loop, resetting detectors and continuing stream")
		if a.loopDetector != nil {
			a.loopDetector.Reset()
		}
		if a.toolCallLoopDetector != nil {
			a.toolCallLoopDetector.Reset()
		}
		a.loopLongOutputTriggered = false
	} else {
		// Judge returned nil (failed/disabled): fallback to direct feedback.
		// Treat as loop confirmed to prevent the stream continuing in a loop.
		log.Warn("handleLoopDetection: judge returned nil (judgment failed/disabled), falling back to direct loop feedback")
		a.mu.Lock()
		a.loopDetectSyncErr = detectErr
		a.loopDetectCrit = true
		a.mu.Unlock()
	}
}

// getLastUserCommand returns the content of the last <task> tag found in user
// messages, walking backwards through a.messages. Returns empty string if none
// found. This provides the judge model with the most recent task instruction.
func (a *Agent) getLastUserCommand() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := len(a.messages) - 1; i >= 0; i-- {
		m := a.messages[i]
		if m.Role == "user" {
			content := strings.TrimSpace(m.CombineContentParts())
			if content == "" {
				content = strings.TrimSpace(m.Content)
			}
			if content != "" {
				// Try to extract content from the last <task> tag
				if taskStart := strings.LastIndex(content, "<task>"); taskStart >= 0 {
					taskStart += len("<task>")
					if taskEnd := strings.Index(content[taskStart:], "</task>"); taskEnd >= 0 {
						taskContent := strings.TrimSpace(content[taskStart : taskStart+taskEnd])
						if taskContent != "" {
							return taskContent
						}
					}
				}
			}
		}
	}
	return ""
}

// buildLoopJudgeUserPrompt constructs the user message for loop judgment.
func (a *Agent) buildLoopJudgeUserPrompt(taskPlanText, suspectContent string) string {
	userTemplate := i18n.T(i18n.KeyLoopJudgeUserPrompt)

	// Find the last non-. user command from message history
	lastInput := a.getLastUserCommand()
	if lastInput == "" {
		lastInput = a.lastUserInput
	}

	userTemplate = strings.ReplaceAll(userTemplate, "{TASK}", a.lastUserInput)
	userTemplate = strings.ReplaceAll(userTemplate, "{TASK_PLAN}", taskPlanText)
	userTemplate = strings.ReplaceAll(userTemplate, "{LAST_INPUT}", lastInput)
	userTemplate = strings.ReplaceAll(userTemplate, "{SUSPECT_CONTENT}", suspectContent)
	return userTemplate
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

// TaskTokenUsage returns the task-level accumulated token usage statistics.
func (a *Agent) TaskTokenUsage() (prompt, completion, total int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.taskPromptTokens, a.taskCompletionTokens, a.taskTokens
}

// ResetTaskTokenUsage resets the task-level token usage statistics to zero.
func (a *Agent) ResetTaskTokenUsage() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.taskPromptTokens = 0
	a.taskCompletionTokens = 0
	a.taskTokens = 0
}

// IterTokenDelta returns the token delta for the most recent LLM call (per-iteration).
// These are the non-zero fresh values from iter* fields.
func (a *Agent) IterTokenDelta() (prompt, completion, total int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.iterPromptTokens, a.iterCompletionTokens, a.iterTokens
}

// GetMaxModelLen returns the maximum context length (in tokens) of the current active model.
// Returns 0 if no model manager or model is configured.
func (a *Agent) GetMaxModelLen() int {
	if a.modelManager != nil {
		if modelCfg := a.modelManager.GetActiveModel(false); modelCfg != nil {
			return modelCfg.MaxModelLen
		}
	}
	return 0
}

// LLMTiming holds performance timing for the most recent LLM call.
type LLMTiming struct {
	FirstTokenLatency string // time to first token (e.g. "1.2s")
	InputTPS          string // input tokens per second (prompt_tokens / time_to_first_token)
	OutputTPS         string // output tokens per second (completion_tokens / generation_time)
}

// GetLLMTiming computes and returns the performance timing for the current LLM call.
// Results are reset after reading so subsequent calls get fresh data.
func (a *Agent) GetLLMTiming() LLMTiming {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Calculate per-call token deltas
	promptDelta := a.totalPromptTokens - a.prevPromptTokens
	totalDelta := a.totalTokens - a.prevTotalTokens
	if totalDelta == 0 {
		// No new data, return empty timing
		return LLMTiming{}
	}

	firstLat := 0.0
	if !a.firstTokenTime.IsZero() && !a.llmCallStartTime.IsZero() {
		firstLat = a.firstTokenTime.Sub(a.llmCallStartTime).Seconds()
	}
	genDuration := 0.0
	if !a.llmStreamEndTime.IsZero() && !a.firstTokenTime.IsZero() {
		genDuration = a.llmStreamEndTime.Sub(a.firstTokenTime).Seconds()
	}

	var result LLMTiming
	if firstLat > 0 {
		result.FirstTokenLatency = fmt.Sprintf("%.1fs", firstLat)
	} else {
		result.FirstTokenLatency = "-"
	}
	if firstLat > 0 && promptDelta > 0 {
		result.InputTPS = fmt.Sprintf("%.0f", float64(promptDelta)/firstLat)
	} else {
		result.InputTPS = "-"
	}
	compDelta := totalDelta - promptDelta
	if genDuration > 0 && compDelta > 0 {
		result.OutputTPS = fmt.Sprintf("%.0f", float64(compDelta)/genDuration)
	} else {
		result.OutputTPS = "-"
	}

	// Update previous counters
	a.prevPromptTokens = a.totalPromptTokens
	a.prevTotalTokens = a.totalTokens
	a.llmCallStartTime = time.Time{}
	a.firstTokenTime = time.Time{}
	a.llmStreamEndTime = time.Time{}

	return result
}
