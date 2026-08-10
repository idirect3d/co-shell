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
	"errors"
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
	mu               sync.Mutex
	llmClient        llm.Client
	mcpMgr           *mcp.Manager
	store            *store.DualStore
	memoryManager    *memory.Manager
	systemPrompt     string
	currentSessionID string // ID of the current named session entry
	messages         []llm.Message
	maxIterations    int
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

	// reorganizeContextUsed is set by reorganizeContextTool (FIX-318).
	// The caller (run_stream.go / run.go) checks it after all tool results have
	// been appended, then collapses the message history to [system, user(summary)]
	// so no orphaned tool message (without a preceding assistant tool_calls) is
	// ever sent to the API.
	reorganizeContextUsed bool

	// visionPendingIntent stores the intent parameter from the most recent
	// visual_analysis call (FEATURE-319). When VisionContextMode == "minimal"
	// and images are pending, buildContextMessages uses this as the clean user
	// instruction sent to the vision model (discarding intermediate history).
	visionPendingIntent string

	// FEATURE-343: vision recognition round state.
	// visionRecognitionActive is set by buildContextMessages when it collapses
	// to [system(Identity-only), user(intent+images)] for the recognition round.
	// streamLLMResponse consumes it to clear the tools list for that call.
	visionRecognitionActive bool
	// visionRecognitionExecuted is set after the recognition-round LLM call
	// returns (success or failure). run_stream.go uses it to backfill the
	// recognition result as the visual_analysis tool result and reset state.
	visionRecognitionExecuted bool
	// lastVisionToolCallID records the ToolCall ID of the most recent
	// visual_analysis call (OpenAI mode) so the recognition result can be
	// backfilled with the correct tool_call_id.
	lastVisionToolCallID string

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

	// loopJudgeSkipped is set to true when the judge model says "not a loop"
	// during the current stream call. Prevents re-triggering loop detection
	// for the remainder of the stream, avoiding infinite judge call loops.
	// Reset at the start of each streamLLMResponse call.
	loopJudgeSkipped bool

	// loopJudgeExitStrategy stores the exit_strategy returned by the judge
	// model in the sync loop detection path. When judge confirms a loop and
	// loop-intervention=prompt, this value is used as the feedback content
	// instead of the generic template. Reset at the start of each loop
	// iteration in RunStream and cleared when judge says not-a-loop.
	// (FIX-285)
	loopJudgeExitStrategy string

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

	// Excel session manager (FEATURE-120)
	excelSessionMgr *excelSessionManager

	// DOCX session manager (FEATURE-121)
	docxSessionMgr *docxSessionManager

	// Vault (FEATURE-274)
	vaultStore  *store.VaultStore
	vaultUnlock func(password string) error
	vaultLock   func()
	vaultInit   func(password string, algorithm string) error

	// Interrupt channel for ESC key (FEATURE-201)
	interruptCh chan struct{} // signals LLM stream to stop

	// Cancel channel for Ctrl+C (FEATURE-239)
	// When signaled, the agent immediately exits the current iteration
	// and returns to the REPL prompt without confirmation.
	cancelCh   chan struct{}      // signals immediate cancellation
	cancelCtx  context.Context    // parent context for tool execution (canceled on Ctrl+C)
	cancelFunc context.CancelFunc // cancel function for cancelCtx

	// debugMode: when enabled, displays messages to be sent to LLM on the prompt
	// line for review and editing before sending.
	debugMode bool

	// UserIO for terminal interaction (FEATURE-201 fix)
	io UserIO

	// commandRunning is set to true while a system command is executing with
	// stdin connected. The ESC monitor goroutine checks this flag to avoid
	// competing with the sub-process for stdin reads (FIX-209).
	commandRunning bool

	// commandHooks stores optional callbacks invoked around system command
	// execution. The REPL registers these to temporarily restore the terminal
	// to cooked mode while an interactive command (e.g. sudo) reads from
	// stdin, then re-enter raw mode afterwards. Without this, raw mode
	// disables echo, line buffering (ICRNL) and Ctrl+C handling, so commands
	// requiring user input hang with no visible feedback.
	commandHooks CommandHooks

	// taskInstructionCache collects user supplementary instructions and other
	// task-level hints (e.g., context overflow warnings) during tool execution.
	// At the end of each iteration, all cached content is flushed as a single
	// <task> ContentPart appended to the last user message. (FEATURE-255)
	taskInstructionCache bytes.Buffer
}

// CommandHooks defines optional callbacks invoked around system command
// execution. The REPL registers these to coordinate terminal mode transitions:
// cooked mode during command execution (so interactive commands can read user
// input normally) and raw mode between commands.
type CommandHooks struct {
	// BeforeCommand is invoked before a system command starts executing.
	// It should restore the terminal to cooked mode.
	BeforeCommand func()
	// AfterCommand is invoked after a system command finishes executing.
	// It should re-enter raw mode.
	AfterCommand func()
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

// SetCommandHooks registers callbacks invoked before/after system command
// execution. The REPL uses this to temporarily restore cooked terminal mode
// while an interactive command reads from stdin, and re-enter raw mode after.
// Pass an empty CommandHooks to clear the hooks.
func (a *Agent) SetCommandHooks(hooks CommandHooks) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commandHooks = hooks
}

// onCommandStart invokes the registered BeforeCommand hook, if any.
// The hook is copied out of the lock before invocation so it can safely
// call back into the agent (e.g. via GetIO).
func (a *Agent) onCommandStart() {
	a.mu.Lock()
	h := a.commandHooks.BeforeCommand
	a.mu.Unlock()
	if h != nil {
		h()
	}
}

// onCommandEnd invokes the registered AfterCommand hook, if any.
func (a *Agent) onCommandEnd() {
	a.mu.Lock()
	h := a.commandHooks.AfterCommand
	a.mu.Unlock()
	if h != nil {
		h()
	}
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

		// FEATURE-319 + FEATURE-343: minimal vision-context mode — send only
		// [system(Identity-only), user(intent + images)] to the vision model.
		// This discards all intermediate history AND replaces the system
		// prompt with ONLY the Identity section (no tool-usage instructions,
		// no capabilities/rules/environment) so the recognition round is a
		// dedicated OCR/vision pass — the model should not attempt tool calls.
		// The intent is taken from the most recent visual_analysis call. If no
		// intent is pending, fall back to the existing behavior (do not
		// collapse) to avoid losing the image-bearing message.
		if a.cfg != nil && a.cfg.LLM.VisionContextMode == "minimal" &&
			a.visionPendingIntent != "" && len(msgs) > 0 {
			// Extract image/video parts from the last user message (injected above).
			var mediaParts []llm.ContentPart
			for _, cp := range lastMsg.ContentParts {
				if cp.Type == llm.ContentPartImageURL || cp.Type == llm.ContentPartVideoURL {
					mediaParts = append(mediaParts, cp)
				}
			}
			cleanMsg := llm.Message{Role: "user"}
			cleanMsg.ContentParts = []llm.ContentPart{
				{Type: llm.ContentPartText, Text: a.visionPendingIntent},
			}
			cleanMsg.ContentParts = append(cleanMsg.ContentParts, mediaParts...)

			// FEATURE-343: replace msgs[0] (full system prompt) with a minimal
			// Identity-only prompt so the visual model never sees tool usage
			// instructions or agent context beyond its own identity.
			identityPrompt := buildVisionIdentityPrompt(a.cfg)
			sysMsg := llm.Message{Role: "system", Content: identityPrompt}

			msgs = []llm.Message{sysMsg, cleanMsg}

			// Mark this as the vision recognition round so streamLLMResponse
			// clears the tools list for this call.
			a.visionRecognitionActive = true
			log.Info("Agent.buildContextMessages: minimal vision-context mode, collapsed to %d messages [FEATURE-343 recognition round]", len(msgs))
		}
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
		cb(EventThinking, resp.ReasoningContent)
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
					// Store parse errors as structured JSON with tool name for format lookup.
					var cacheLines []string
					for _, pe := range parseErrors {
						toolName := ""
						if tagStart := strings.Index(pe, `"tag": "`); tagStart >= 0 {
							tagStart += len(`"tag": "`)
							if tagEnd := strings.Index(pe[tagStart:], `"`); tagEnd >= 0 {
								toolName = pe[tagStart : tagStart+tagEnd]
							}
						}
						// FEATURE-336: include the raw response so the user can
						// inspect the exact malformed content on parse errors.
						if toolName == "" {
							cacheLines = append(cacheLines, fmt.Sprintf(`{"tool": "", "error": %q, "raw": %q}`, pe, resp.Content))
						} else {
							cacheLines = append(cacheLines, fmt.Sprintf(`{"tool": %q, "error": %q, "raw": %q}`, toolName, pe, resp.Content))
						}
					}
					a.taskInstructionCache.WriteString(strings.Join(cacheLines, "\n---\n"))
					toolCalls = nil
					log.Debug("Agent.nonStreamingFallback: %d XML parse errors stored in taskInstructionCache (no tool calls)",
						len(parseErrors))
					return "", resp.ReasoningContent, nil, nil
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
	if a.cfg == nil {
		return nil
	}
	// FEATURE-342: loop judgment requires BOTH the unified problem-solver
	// switch AND the loop-judge switch (dual gating).
	if !a.cfg.LLM.ProblemSolverEnabled || !a.cfg.LLM.LoopJudgeEnabled {
		log.Debug("judgeLoop: loop judgment disabled (problem_solver_enabled=%v, loop_judge_enabled=%v), returning nil",
			a.cfg.LLM.ProblemSolverEnabled, a.cfg.LLM.LoopJudgeEnabled)
		return nil
	}

	modelCfg := a.getLoopJudgeModel()
	if modelCfg == nil {
		log.Warn("judgeLoop: no model available for loop judgment, skipping")
		return nil
	}

	// FEATURE-342: Prefer the structured report_problem path. If the problem
	// solver call succeeds, convert its ProblemReport to the classic
	// LoopJudgeResult and return early (single model call). On failure
	// (e.g. model does not support tools), fall back to the classic
	// free-text JSON judgment below.
	if report, perr := a.callProblemSolver(ctx, a.buildProblemSolverPrompt(suspectContent)); perr == nil && report != nil {
		result := &LoopJudgeResult{
			IsLoop:       report.IsLoop(),
			Reason:       report.Reason,
			ExitStrategy: report.Guidance,
		}
		if result.IsLoop && strings.TrimSpace(result.ExitStrategy) == "" {
			log.Warn("judgeLoop: problem solver confirmed loop with empty guidance, applying fallback")
			result.ExitStrategy = i18n.T(i18n.KeyLoopJudgeFallback)
		}
		log.Info("LoopJudge result (problem-solver): is_loop=%v, reason=%q, exit_strategy=%q",
			result.IsLoop, result.Reason, result.ExitStrategy)
		return result
	}
	log.Debug("judgeLoop: problem solver report unavailable, falling back to classic JSON judgment")

	// Build task plan text
	taskPlanText := a.getTaskPlanPrompt()
	if taskPlanText == "" {
		taskPlanText = i18n.T(i18n.KeyNoActiveTaskPlan)
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
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopJudgePrompt), strings.TrimSpace(userText)))
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

	// Write the judgment request to the LLM interaction log
	if log.IsLLMInteractionEnabled() {
		reqMap := map[string]interface{}{
			"model":    modelCfg.Model,
			"messages": messages,
		}
		if reqJSON, err := json.MarshalIndent(reqMap, "", "  "); err == nil {
			log.WriteLLMInteraction("REQ][judgeLoop", string(reqJSON))
		}
	}

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

	// Write the judgment response to the LLM interaction log
	if log.IsLLMInteractionEnabled() {
		log.WriteLLMInteraction("RESP][judgeLoop", resp.Content)
		log.WriteLLMInteractionEnd()
	}

	// Display the judge model's full response
	if cb := a.streamCb; cb != nil {
		showDetail := a.cfg == nil || a.cfg.LLM.ShowLoopDetection
		if showDetail {
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopJudgeResponse), strings.TrimSpace(resp.Content)))
		}
	}

	// Parse JSON response
	result := &LoopJudgeResult{}
	content := strings.TrimSpace(resp.Content)

	// Skip any think/reasoning content before looking for JSON.
	// The judge model may output ... even when thinking is disabled,
	// and thinking content can contain "{" characters that would
	// interfere with JSON extraction. Find the last </think> tag
	// and start from there.
	if thinkEnd := strings.LastIndex(content, "</think>"); thinkEnd >= 0 {
		content = strings.TrimSpace(content[thinkEnd+len("</think>"):])
	}

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

	// FIX-322: empty exit_strategy fallback. When the judge confirms a loop
	// but does not provide an actionable next instruction (empty string),
	// substitute a concrete fallback directive instead of degrading to the
	// generic template downstream.
	if result.IsLoop && strings.TrimSpace(result.ExitStrategy) == "" {
		log.Warn("judgeLoop: judge confirmed loop with empty exit_strategy, applying fallback")
		result.ExitStrategy = i18n.T(i18n.KeyLoopJudgeFallback)
	}

	log.Info("LoopJudge result: is_loop=%v, reason=%q, exit_strategy=%q",
		result.IsLoop, result.Reason, result.ExitStrategy)

	return result
}

// applyLoopIntervention applies the configured loop intervention strategy
// for a unified LoopEvent. It handles all types of loop events through the
// same pipeline, sending feedback to the LLM, adjusting temperature, or
// reorganizing context based on the cfg.LLM.LoopIntervention setting.
//
// The method returns a non-nil error to signal that the current iteration
// should NOT continue (the feedback message has been appended), or nil
// if the intervention should be skipped and the iteration can proceed.
func (a *Agent) applyLoopIntervention(event *LoopEvent) error {
	// Read loop intervention strategy directly from config.
	// SettingsHandler and Agent share the same cfg pointer via SetConfig(),
	// so :set at runtime is immediately visible via a.cfg.LLM.LoopIntervention.
	loopAction := "retry" // DefaultConfig() default
	if a.cfg != nil && a.cfg.LLM.LoopIntervention != "" {
		loopAction = a.cfg.LLM.LoopIntervention
	}

	log.Info("applyLoopIntervention: type=%d, detector=%q, action=%q, cfg=%p",
		event.Type, event.Detector, loopAction, a.cfg)
	if a.cfg != nil {
		log.Info("  cfg loop_intervention=%q", a.cfg.LLM.LoopIntervention)
	}

	cb := a.streamCb
	if cb != nil {
		ep := config.GetEmojiPrefixes(a.emojiEnabled)
		cb(EventInfo, ep.Loop+fmt.Sprintf(i18n.TF(i18n.KeyLoopDetectEvent), event.Detector))
	}

	// Secondary judgment: when LoopJudgeEnabled, call judge model FIRST to
	// confirm whether this is truly a loop, before applying any strategy.
	// If the judge says NOT a loop, skip intervention entirely.
	// This applies to cross-iteration content duplicate & tool call loop paths.
	if a.cfg != nil && a.cfg.LLM.LoopJudgeEnabled {
		suspectContent := event.Content
		if suspectContent == "" {
			suspectContent = event.Reason
		}
		judgeErr := errors.New(event.Reason)
		result := a.judgeLoop(context.Background(), judgeErr, suspectContent)
		if result != nil && !result.IsLoop {
			// Judge says not a loop — do NOT intervene.
			if cb != nil {
				cb(EventInfo, i18n.T(i18n.KeyLoopJudgeNotLoop))
			}
			return nil
		}
		if result != nil && result.IsLoop && result.ExitStrategy != "" {
			// Store the judge's exit_strategy for the prompt strategy to use
			// as feedback content (more targeted than the generic prompt).
			event.Suggestion = result.ExitStrategy
		}
		// result == nil (judgment failed): continue with normal strategy
		// If loopAction == "off" and judge confirmed, still treat as "off".
		// The action check happens below in the switch.
	}

	// Reset the triggering detector's counters so the same pattern does not
	// re-trigger immediately in the next iteration after intervention.
	// Each event type maps to its corresponding detector.
	switch event.Type {
	case LoopEventContentPeriodic, LoopEventContentDuplicate, LoopEventSingleLineRepeat:
		if a.loopDetector != nil {
			a.loopDetector.Reset()
		}
	case LoopEventToolCallRepeat:
		if a.toolCallLoopDetector != nil {
			a.toolCallLoopDetector.Reset()
		}
	}

	// Build the feedback message based on event type
	var loopFeedback string
	var strategyDesc string

	switch loopAction {
	case "off":
		// No intervention
		if cb != nil {
			cb(EventInfo, i18n.T(i18n.KeyLoopJudgeDisabled))
		}
		return nil

	case "retry":
		// Just resend without any feedback
		loopFeedback = ""
		strategyDesc = i18n.T(i18n.KeyStrategyResend)

	case "prompt":
		// Use judge's exit_strategy if available (set by secondary judgment above),
		// otherwise fall back to the generic loop detection feedback.
		loopFeedback = event.Suggestion
		if loopFeedback == "" {
			loopFeedback = i18n.T(i18n.KeyLoopDetectFeedback)
		}
		strategyDesc = i18n.T(i18n.KeyStrategyPrompt)

	case "temperature":
		if a.loopTempCtrl != nil {
			_, changed := a.loopTempCtrl.Apply()
			if changed {
				a.llmClient.SetTemperature(a.loopTempCtrl.Temperature())
			}
		}
		loopFeedback = ""
		strategyDesc = i18n.T(i18n.KeyStrategyTempAdjust)

	case "reorganize":
		loopFeedback = i18n.T(i18n.KeyLoopReorganizeSuggestion)
		strategyDesc = i18n.T(i18n.KeyStrategyReorganize)

	case "random":
		actions := []string{"retry", "prompt", "reorganize", "temperature"}
		choice := actions[time.Now().UnixNano()%4]
		switch choice {
		case "retry":
			loopFeedback = ""
			strategyDesc = i18n.T(i18n.KeyStrategyRandomResend)
		case "prompt":
			loopFeedback = i18n.T(i18n.KeyLoopDetectFeedback)
			strategyDesc = i18n.T(i18n.KeyStrategyRandomPrompt)
		case "reorganize":
			loopFeedback = i18n.T(i18n.KeyLoopReorganizeSuggestion)
			strategyDesc = i18n.T(i18n.KeyStrategyRandomReorg)
		case "temperature":
			if a.loopTempCtrl != nil {
				_, changed := a.loopTempCtrl.Apply()
				if changed {
					a.llmClient.SetTemperature(a.loopTempCtrl.Temperature())
				}
			}
			loopFeedback = ""
			strategyDesc = i18n.T(i18n.KeyStrategyRandomTemp)
		}

	default:
		// Unknown strategy: clear feedback to avoid sending prompt unexpectedly
		loopFeedback = ""
		strategyDesc = fmt.Sprintf(i18n.TF(i18n.KeyStrategyUnknown), loopAction)
	}

	// Apply loop feedback via the unified helper (FIX-321).
	// When loopFeedback is non-empty (prompt/reorganize) a loop feedback
	// message with a full <environment_details> block is created or updated
	// in place; when empty (retry/temperature) only the <retried_count> tag
	// on the last user message is incremented, no new message is appended.
	a.applyLoopFeedback(loopFeedback)

	// FEATURE-327: Check the retried_count limit. When the count reaches
	// error-max-single-count, the user is prompted to decide (Enter/C/A).
	// If the user cancels, propagate the error to the caller.
	if ok, err := a.checkRetryCountLimit(); err != nil {
		return err
	} else if !ok {
		return errors.New("retry count limit cancelled")
	}

	if cb != nil {
		cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopHandling), strategyDesc))
		if loopFeedback != "" {
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopFeedbackSent), loopFeedback))
		} else {
			cb(EventInfo, i18n.T(i18n.KeyLoopNoFeedback))
		}
		cb(EventInfo, "────────────────────────────────────────────\n")
	}

	return nil
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

	// Always show the loop-detection banner (separator + infinite-loop icon +
	// "suspected loop" message) regardless of LoopJudgeEnabled / ShowLoopDetection,
	// so the user is always informed that the agent may be stuck in a loop.
	// FIX-329: classify the trigger scenario (single_repeat / multi_line /
	// line_too_long / char_period / long_output / tool_call) so the user can
	// tell what kind of repetition was detected.
	io := a.defaultIO()
	ep := config.GetEmojiPrefixes(a.emojiEnabled)
	io.Println()
	io.Println("────────────────────────────────────────────")
	loopType := loopTypeFromError(detectErr)
	if loopType != "" {
		io.Println(ep.Loop + fmt.Sprintf(i18n.TF(i18n.KeyLoopSuspectedWithType), i18n.T(loopTypeKey(loopType))))
	} else {
		io.Println(ep.Loop + i18n.T(i18n.KeyLoopSuspected))
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
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopJudgeResultLoop), result.Reason, result.ExitStrategy))
		} else if result != nil && !result.IsLoop {
			cb(EventInfo, fmt.Sprintf(i18n.TF(i18n.KeyLoopJudgeResultNo), result.Reason))
		} else {
			cb(EventInfo, i18n.T(i18n.KeyLoopJudgeResultFail))
		}
	}

	if result != nil && result.IsLoop {
		// Judge confirmed loop: save exit_strategy and interrupt the stream.
		a.mu.Lock()
		a.loopJudgeExitStrategy = result.ExitStrategy
		a.loopDetectSyncErr = detectErr
		a.loopDetectCrit = true
		a.mu.Unlock()
		log.Debug("handleLoopDetection: judge confirmed loop, saved exit_strategy=%q, set loopDetectSyncErr", result.ExitStrategy)
	} else if result != nil && !result.IsLoop {
		// Judge explicitly NOT a loop: clear exit_strategy, reset detectors,
		// set loopJudgeSkipped to prevent re-triggering for the remainder of
		// this stream call.
		log.Debug("handleLoopDetection: judge says NOT a loop, clearing exit_strategy, resetting detectors and continuing stream")
		a.mu.Lock()
		a.loopJudgeExitStrategy = ""
		a.mu.Unlock()
		if a.loopDetector != nil {
			a.loopDetector.Reset()
		}
		if a.toolCallLoopDetector != nil {
			a.toolCallLoopDetector.Reset()
		}
		a.loopLongOutputTriggered = false
		a.loopJudgeSkipped = true
	} else {
		// Judge returned nil (failed/disabled): fallback to direct feedback.
		// Clear exit_strategy and treat as loop confirmed to prevent the
		// stream continuing in a loop.
		log.Warn("handleLoopDetection: judge returned nil (judgment failed/disabled), falling back to direct loop feedback")
		a.mu.Lock()
		a.loopJudgeExitStrategy = ""
		a.loopDetectSyncErr = detectErr
		a.loopDetectCrit = true
		a.mu.Unlock()
	}
}

// loopTypeFromError extracts the LoopType classification from a loop detection
// error (FIX-329). It handles the detectors that produce typed errors:
//   - *LoopDetectedError → its LoopType field ("single_repeat"/"multi_line"),
//     or "line_too_long"/"char_period" when the single-line sub-detector fired
//   - *ToolCallLoopDetectedError → "tool_call"
//   - Long-output error (text match) → "long_output"
//
// Returns "" when the type cannot be determined (caller falls back to the
// generic suspected-loop banner).
func loopTypeFromError(detectErr error) string {
	switch e := detectErr.(type) {
	case *LoopDetectedError:
		if e.LoopType != "" {
			return e.LoopType
		}
		// Fall back: infer from the error message when the type was not set
		// (defensive; all current construction paths set LoopType).
		msg := e.Error()
		switch {
		case strings.Contains(msg, "exceeds threshold"):
			return "line_too_long"
		case strings.Contains(msg, "char-level period"):
			return "char_period"
		case e.period == 1:
			return "single_repeat"
		default:
			return "multi_line"
		}
	case *ToolCallLoopDetectedError:
		return "tool_call"
	}
	// Long-output trigger is a generic error; identify by its message prefix.
	if detectErr != nil && strings.Contains(detectErr.Error(), "LLM output exceeds") {
		return "long_output"
	}
	return ""
}

// loopTypeKey maps a LoopType identifier to its i18n key constant.
// Unknown identifiers fall back to the generic banner (returning "").
func loopTypeKey(loopType string) string {
	switch loopType {
	case "single_repeat":
		return i18n.KeyLoopTypeSingleRepeat
	case "multi_line":
		return i18n.KeyLoopTypeMultiLine
	case "line_too_long":
		return i18n.KeyLoopTypeLineTooLong
	case "char_period":
		return i18n.KeyLoopTypeCharPeriod
	case "long_output":
		return i18n.KeyLoopTypeLongOutput
	case "tool_call":
		return i18n.KeyLoopTypeToolCall
	}
	return ""
}

// getFirstUserCommand returns the content of the first user message in a.messages
// (excluding system prompts). The <task> wrapper extraction was removed
// because it distorted LLM attention priority (FEATURE-292).
// This provides the judge model with the original task instruction.
func (a *Agent) getFirstUserCommand() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := 0; i < len(a.messages); i++ {
		m := a.messages[i]
		if m.Role != "user" {
			continue
		}
		content := strings.TrimSpace(m.CombineContentParts())
		if content == "" {
			content = strings.TrimSpace(m.Content)
		}
		if content == "" {
			continue
		}

		// Strip environment_details envelope if present (everything after ---)
		if envIdx := strings.Index(content, "\n---"); envIdx >= 0 {
			content = strings.TrimSpace(content[:envIdx])
		}

		// Strip <environment_details> suffix
		if envIdx := strings.Index(content, "<environment_details>"); envIdx > 0 {
			content = strings.TrimSpace(content[:envIdx])
		}

		if content != "" {
			return content
		}
	}
	return ""
}

// getLastUserCommand returns the content of the last user message in a.messages,
// walking backwards through a.messages (FEATURE-292). The <task> tag extraction
// was removed because it distorted LLM attention priority.
// Provides the judge model with the most recent task instruction.
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
				// Strip <environment_details> suffix
				if envIdx := strings.Index(content, "<environment_details>"); envIdx > 0 {
					content = strings.TrimSpace(content[:envIdx])
				}
				if content != "" {
					return content
				}
			}
		}
	}
	return ""
}

// getRecentIterations returns the last 2 assistant responses (without tool calls)
// from a.messages, for the loop judge to analyze. Excludes the current iteration.
func (a *Agent) getRecentIterations() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var sb strings.Builder
	count := 0
	for i := len(a.messages) - 1; i >= 0 && count < 2; i-- {
		m := a.messages[i]
		if m.Role == "assistant" && len(m.ToolCalls) == 0 && m.Content != "" {
			if count > 0 {
				sb.WriteString("\n---\n")
			}
			sb.WriteString(m.Content)
			count++
		}
	}
	return sb.String()
}

// getAllUserPrompts collects ALL genuine user instructions from a.messages in
// chronological order (FIX-329). It filters out system-generated user messages:
//   - XML tool results ("[tool] 返回结果：..." — KeyXMLToolResultTemplate)
//   - continue prompts (KeyContinuePrompt)
//   - loop feedback messages whose env carries <loop_feedback>true</loop_feedback>
//
// Each entry is numbered ("1. ...", "2. ...") and stripped of its trailing
// <environment_details> block. The total output is capped at 4000 characters.
func (a *Agent) getAllUserPrompts() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	xmlToolPrefix := i18n.T(i18n.KeyXMLToolResultTemplate)
	continuePrompt := strings.TrimSpace(i18n.T(i18n.KeyContinuePrompt))

	var prompts []string
	for i := 0; i < len(a.messages); i++ {
		m := a.messages[i]
		if m.Role != "user" {
			continue
		}

		content := strings.TrimSpace(m.CombineContentParts())
		if content == "" {
			content = strings.TrimSpace(m.Content)
		}
		if content == "" {
			continue
		}

		// Skip loop feedback messages (system-generated).
		if lastEnv := lastEnvText(&m); isLoopFeedbackText(lastEnv) {
			continue
		}

		// Skip XML tool result messages ("[tool_name] 返回结果：" prefix).
		// The template uses the literal "{TOOL_CALL}" placeholder which is
		// replaced by the actual tool name at message creation time, so a
		// HasPrefix comparison against the raw template would never match
		// (FIX-329). Instead detect the resolved shape: starts with "[",
		// contains "] 返回结果：" or "] Result:".
		if xmlToolPrefix != "" {
			trimmedPrefix := strings.TrimSpace(xmlToolPrefix)
			containsPrefix := strings.Contains(trimmedPrefix, i18n.T(i18n.KeyXMLToolResultMarker))
			if strings.HasPrefix(content, "[") &&
				((containsPrefix && strings.Contains(content, "] "+i18n.T(i18n.KeyXMLToolResultMarker))) ||
					(!containsPrefix && strings.Contains(content, "] Result:"))) {
				continue
			}
		}

		// Skip continue prompts.
		if continuePrompt != "" && strings.TrimSpace(strings.TrimPrefix(content, continuePrompt)) == "" {
			continue
		}

		// Strip <environment_details> suffix.
		if envIdx := strings.Index(content, "<environment_details>"); envIdx > 0 {
			content = strings.TrimSpace(content[:envIdx])
		}

		if content != "" {
			prompts = append(prompts, content)
		}
	}

	var sb strings.Builder
	total := 0
	for i, p := range prompts {
		line := fmt.Sprintf("%d. %s", i+1, p)
		if total+len(line)+1 > 4000 {
			sb.WriteString("\n...(truncated)")
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		total += len(line) + 1
	}
	return strings.TrimSpace(sb.String())
}

// getIterationTools lists the tool names called per assistant iteration in
// chronological order (FIX-329). This gives the judge model an objective
// "did we make progress" signal — repeatedly calling the same tool suggests
// a stalled approach even when the surrounding text differs.
func (a *Agent) getIterationTools() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	var sb strings.Builder
	iter := 0
	for i := 0; i < len(a.messages); i++ {
		m := a.messages[i]
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		iter++
		names := make([]string, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			names = append(names, tc.Name)
		}
		sb.WriteString(fmt.Sprintf("iter%d: %s\n", iter, strings.Join(names, ", ")))
	}
	return strings.TrimSpace(sb.String())
}

// buildLoopJudgeUserPrompt constructs the user message for loop judgment.
func (a *Agent) buildLoopJudgeUserPrompt(taskPlanText, suspectContent string) string {
	userTemplate := i18n.T(i18n.KeyLoopJudgeUserPrompt)

	// {TASK} = the first user message (original task)
	firstInput := a.getFirstUserCommand()
	if firstInput == "" {
		firstInput = a.lastUserInput
	}

	// {LAST_INPUT} = the most recent user instruction
	lastInput := a.getLastUserCommand()
	if lastInput == "" {
		lastInput = a.lastUserInput
	}

	// {ITERATIONS} = last 2 assistant responses for context
	iterations := a.getRecentIterations()
	if iterations == "" {
		iterations = i18n.T(i18n.KeyNoRecentIterations)
	}

	// {CONTEXT} = workspace & available tools context, so the judge can write a
	// concrete, executable exit_strategy instead of guessing (FIX-322).
	contextText := a.buildJudgeContext()

	// FIX-329: {USER_PROMPTS} = all genuine user instructions (chronological).
	userPrompts := a.getAllUserPrompts()
	if userPrompts == "" {
		userPrompts = a.lastUserInput
	}

	// FIX-329: {ITERATION_TOOLS} = tool call names per iteration.
	iterTools := a.getIterationTools()

	userTemplate = strings.ReplaceAll(userTemplate, "{TASK}", firstInput)
	userTemplate = strings.ReplaceAll(userTemplate, "{TASK_PLAN}", taskPlanText)
	userTemplate = strings.ReplaceAll(userTemplate, "{LAST_INPUT}", lastInput)
	userTemplate = strings.ReplaceAll(userTemplate, "{ITERATIONS}", iterations)
	userTemplate = strings.ReplaceAll(userTemplate, "{USER_PROMPTS}", userPrompts)
	userTemplate = strings.ReplaceAll(userTemplate, "{ITERATION_TOOLS}", iterTools)
	userTemplate = strings.ReplaceAll(userTemplate, "{CONTEXT}", contextText)
	userTemplate = strings.ReplaceAll(userTemplate, "{SUSPECT_CONTENT}", suspectContent)
	return userTemplate
}

// buildJudgeContext gathers the environment context passed to the judge model
// so its exit_strategy can reference real files, directories and tools (FIX-322).
// Content:
//   - current working directory
//   - workspace root (a.workspacePath when set)
//   - recent file/directory paths referenced in the message history
//   - the available tool list (when toolCallEnabled)
func (a *Agent) buildJudgeContext() string {
	var sb strings.Builder

	cwd, _ := os.Getwd()
	sb.WriteString("cwd: ")
	sb.WriteString(cwd)
	sb.WriteString("\n")

	if a.workspacePath != "" {
		sb.WriteString("workspace: ")
		sb.WriteString(a.workspacePath)
		sb.WriteString("\n")
	}

	// Recent file/dir paths seen in the last few user/assistant messages.
	seen := make(map[string]bool)
	var paths []string
	a.mu.Lock()
	start := len(a.messages) - 8
	if start < 0 {
		start = 0
	}
	for i := start; i < len(a.messages); i++ {
		content := strings.TrimSpace(a.messages[i].CombineContentParts())
		if content == "" {
			content = strings.TrimSpace(a.messages[i].Content)
		}
		for _, tok := range strings.Fields(content) {
			// Heuristic: token that looks like a file path (contains / or \, or
			// has a common source extension).
			if (strings.Contains(tok, "/") || strings.Contains(tok, "\\") ||
				strings.HasSuffix(tok, ".go") || strings.HasSuffix(tok, ".md") ||
				strings.HasSuffix(tok, ".json") || strings.HasSuffix(tok, ".py")) &&
				!strings.HasPrefix(tok, "http://") && !strings.HasPrefix(tok, "https://") {
				clean := strings.Trim(tok, `"'(),;:`)
				if clean != "" && !seen[clean] {
					seen[clean] = true
					paths = append(paths, clean)
					if len(paths) >= 12 {
						break
					}
				}
			}
		}
		if len(paths) >= 12 {
			break
		}
	}
	a.mu.Unlock()

	if len(paths) > 0 {
		sb.WriteString("recent_paths:\n")
		for _, p := range paths {
			sb.WriteString("  - ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	// Available tools (OpenAI mode tools list; XML mode uses buildToolsInternal).
	// Guard with a.mcpMgr nil check: buildToolsInternal panics if the MCP
	// manager is absent (e.g. unit tests or judge path in a partially
	// initialized agent).
	if a.toolCallEnabled {
		var toolNames []string
		if a.mcpMgr != nil {
			for _, t := range a.buildToolsInternal() {
				toolNames = append(toolNames, t.Name)
			}
		}
		if len(toolNames) > 0 {
			sb.WriteString("available_tools: ")
			sb.WriteString(strings.Join(toolNames, ", "))
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
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
