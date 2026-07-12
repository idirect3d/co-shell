// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-06-01
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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
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

// New creates a new Agent instance.
func New(llmClient llm.Client, mcpMgr *mcp.Manager, s *store.DualStore, rules string) *Agent {
	systemPrompt := buildSystemPromptWithMode(nil, rules, config.ResultModeMinimal, false, "", "", "", "", "", "", "", i18n.T(i18n.KeySystemPromptToolUsage))

	return &Agent{
		llmClient:       llmClient,
		mcpMgr:          mcpMgr,
		store:           s,
		memoryManager:   memory.NewManager(s),
		systemPrompt:    systemPrompt,
		maxIterations:   config.DefaultConfig().LLM.MaxIterations,
		rules:           rules,
		subAgentMgr:     subagent.NewManager(),
		taskPlanMgr:     taskplan.NewManager(s),
		name:            "co-shell",
		modelManager:    config.GetDefaultModelManager(),
		toolCallModeMgr: NewToolCallModeManager(),
		excelSessionMgr: newExcelSessionManager(),
		docxSessionMgr:  newDocxSessionManager(),
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// SetIO sets the UserIO implementation used by this agent for user interaction.
// Must be called before RunStream if enhanced input is desired.
func (a *Agent) SetIO(io UserIO) {
	a.io = io
}

// IO returns the current UserIO implementation (may be nil).
func (a *Agent) IO() UserIO {
	return a.io
}

// defaultIO returns the UserIO for output operations that happen before SetIO is called.
// When io is nil, falls back to direct fmt.Print.
func (a *Agent) defaultIO() UserIO {
	if a.io != nil {
		return a.io
	}
	return defaultIO
}

func (a *Agent) Messages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

func (a *Agent) SetName(name string) {
	if name == "" {
		name = "co-shell"
	}
	a.name = name
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Said() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	return i18n.TF(i18n.KeyAgentSaid, now, a.name)
}

func (a *Agent) SetShowLlmThinking(show bool)   { a.showLlmThinking = show }
func (a *Agent) SetShowLlmContent(show bool)    { a.showLlmContent = show }
func (a *Agent) SetShowTool(show bool)          { a.showTool = show }
func (a *Agent) SetShowToolInput(show bool)     { a.showToolInput = show }
func (a *Agent) SetShowToolOutput(show bool)    { a.showToolOutput = show }
func (a *Agent) SetShowCommand(show bool)       { a.showCommand = show }
func (a *Agent) SetShowCommandOutput(show bool) { a.showCommandOutput = show }

func (a *Agent) SetMaxIterations(n int) {
	if n <= 0 {
		a.maxIterations = -1
	} else {
		a.maxIterations = n
	}
}

func (a *Agent) SetToolMode(toolName string, mode string) {
	if a.toolModes == nil {
		a.toolModes = make(map[string]string)
	}
	if toolName == "" {
		a.toolModes["default"] = mode
	} else {
		a.toolModes[toolName] = mode
	}
}

// ToolModes returns the current tool mode settings (for display purposes only).
func (a *Agent) ToolModes() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.toolModes == nil {
		return nil
	}
	result := make(map[string]string, len(a.toolModes))
	for k, v := range a.toolModes {
		result[k] = v
	}
	return result
}

func DefaultToolModes() map[string]string {
	return map[string]string{
		"execute_command":            "confirm",
		"read_file":                  "confirm",
		"write_to_file":              "confirm",
		"replace_in_file":            "confirm",
		"search_files":               "confirm",
		"list_files":                 "auto",
		"list_code_definition_names": "auto",
		"visual_analysis":            "auto",
		"update_settings":            "confirm",
		"list_settings":              "auto",
		"ask_followup_question":      "auto",
		"launch_sub_agent":           "confirm",
		"schedule_task":              "confirm",
		"track_task_progress":        "auto",
		"view_task_plan":             "auto",
		"get_memory_slice":           "auto",
		"memory_search":              "auto",
		"delete_memory":              "confirm",
		"shell_send":                 "confirm",
		"shell_get_output":           "auto",
		"shell_window_content":       "auto",
		"shell_reset":                "auto",
		"attempt_completion":         "auto",
		"evaluate_expression":        "auto",
		"reorganize_context":         "auto",
		// Vault tools (FEATURE-274) - list is auto, add/remove require confirmation
		"vault_list":   "auto",
		"vault_add":    "confirm",
		"vault_remove": "confirm",
		// Word tools (FEATURE-121) - continue/write operations need confirm, read-only are auto
		"word_open":          "auto",
		"word_close":         "auto",
		"word_save":          "auto",
		"word_overview":      "auto",
		"word_read":          "auto",
		"word_table_read":    "auto",
		"word_continue":      "confirm",
		"word_erase":         "confirm",
		"word_inspect_style": "auto",
		"word_format":        "confirm",
		// Excel tools (FEATURE-120) - edit/paste/insert/delete are confirm, rest are auto
		"excel_open":     "auto",
		"excel_close":    "auto",
		"excel_save":     "auto",
		"excel_overview": "auto",
		"excel_read":     "auto",
		"excel_edit":     "confirm",
		"excel_copy":     "auto",
		"excel_paste":    "confirm",
		"excel_insert":   "confirm",
		"excel_delete":   "confirm",
		"excel_sheet":    "auto",
		// Browser tools (FEATURE-200) - all auto since screenshots are non-destructive
		"browser_navigate":                 "auto",
		"browser_screenshot":               "auto",
		"browser_click":                    "auto",
		"browser_type":                     "auto",
		"browser_evaluate":                 "auto",
		"browser_get_html":                 "auto",
		"browser_scroll":                   "auto",
		"browser_get_interactive_elements": "auto",
		"browser_go_back":                  "auto",
		"browser_go_forward":               "auto",
		"browser_close":                    "auto",
	}
}

// SyncToolModes synchronizes tool mode settings from config to agent.
// It applies per-tool overrides, global defaults, and mode-specific restrictions.
func (a *Agent) SyncToolModes(cfg *config.Config) {
	modes := DefaultToolModes()

	// Check if the current WorkMode has its own ToolModes.
	// If a WorkMode has explicit ToolModes, use them as the COMPLETE base —
	// the work mode's ToolModes represent the full intention for that mode,
	// and global cfg.LLM.ToolModes should NOT override mode-specific restrictions.
	workModeName := cfg.LLM.WorkMode
	if workModeName != "" {
		hasModeToolModes := false
		// Search user-defined modes first
		for _, wm := range cfg.WorkModes {
			if wm.Name == workModeName && wm.ToolModes != nil && len(wm.ToolModes) > 0 {
				modes = cloneToolModes(wm.ToolModes)
				hasModeToolModes = true
				break
			}
		}
		// Fall back to built-in modes (act, plan)
		if !hasModeToolModes {
			for _, wm := range config.DefaultWorkModes() {
				if wm.Name == workModeName && wm.ToolModes != nil && len(wm.ToolModes) > 0 {
					modes = cloneToolModes(wm.ToolModes)
					hasModeToolModes = true
					break
				}
			}
		}

		if hasModeToolModes {
			// Mode has its own ToolModes — apply per-tool overrides from config,
			// but ONLY for tools that already have an explicit setting in the mode.
			// The mode's "default" setting is respected; global default does NOT override it.
			if cfg.LLM.ToolModes != nil {
				for k, v := range cfg.LLM.ToolModes {
					if k == "default" {
						continue
					}
					// Only override if there's an explicit setting in the mode
					if _, hasExplicit := modes[k]; hasExplicit {
						modes[k] = v
					}
				}
			}
			a.toolModes = modes
			return
		}
	}

	// No mode-specific ToolModes: use defaults with global overrides.
	// Apply per-tool overrides from config.LLM.ToolModes (runtime overrides).
	if cfg.LLM.ToolModes != nil {
		for k, v := range cfg.LLM.ToolModes {
			if k == "default" {
				continue
			}
			if _, exists := modes[k]; exists {
				modes[k] = v
			}
		}
	}

	// Apply global default override if set to confirm/auto/disabled.
	if globalDefault, ok := cfg.LLM.ToolModes["default"]; ok && globalDefault != "" && globalDefault != "custom" {
		modes["default"] = globalDefault
		for k := range modes {
			if k != "default" {
				modes[k] = globalDefault
			}
		}
	}

	a.toolModes = modes
}

// cloneToolModes returns a copy of a tool modes map.
func cloneToolModes(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (a *Agent) SetMemoryEnabled(enabled bool)   { a.memoryEnabled = enabled }
func (a *Agent) SetEmojiEnabled(enabled bool)    { a.emojiEnabled = enabled }
func (a *Agent) SetToolCallEnabled(enabled bool) { a.toolCallEnabled = enabled }

func (a *Agent) SetDebugMode(enabled bool) { a.debugMode = enabled }
func (a *Agent) IsDebugMode() bool         { return a.debugMode }

// debugIntercept displays the next messages to be sent to the LLM and allows
// the user to review/modify the last user message before sending.
// Returns true if the messages were modified, false if sent as-is.
func (a *Agent) debugIntercept() bool {
	if !a.debugMode || a.io == nil {
		return false
	}

	// Show the last user message for preview.
	// User messages now use ContentParts (array format) with the instruction as the
	// first text part. Use CombineContentParts() to get the full text representation.
	a.mu.Lock()
	userMsg := ""
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "user" {
			if len(a.messages[i].ContentParts) > 0 {
				userMsg = a.messages[i].CombineContentParts()
			} else {
				userMsg = a.messages[i].Content
			}
			break
		}
	}
	a.mu.Unlock()

	if userMsg == "" {
		return false
	}

	a.io.Println()
	a.io.Println(i18n.T(i18n.KeyDebugPromptHeader))
	// Try to extract just the user's actual message (before environment_details)
	cleanMsg := userMsg
	if idx := strings.Index(cleanMsg, "<environment_details>"); idx > 0 {
		cleanMsg = strings.TrimSpace(cleanMsg[:idx])
	}
	a.io.Println(cleanMsg)
	a.io.Println()
	a.io.Println(i18n.T(i18n.KeyDebugPromptFooter))
	a.io.Printf("> ")

	input, err := a.io.ReadLine()
	if err != nil {
		return false
	}

	// Handle debug mode toggle without sending as modified content
	if strings.HasPrefix(input, ":debug ") {
		switch strings.TrimSpace(input[7:]) {
		case "on":
			a.SetDebugMode(true)
			a.io.Println("调试模式已开启")
		case "off":
			a.SetDebugMode(false)
			a.io.Println("调试模式已关闭")
		}
		// Send the message as-is regardless
		return false
	}

	if input == "" {
		// No modification, send as-is
		log.Debug("debugIntercept: user pressed Enter, sending unmodified")
		return false
	}

	// User modified the content - replace the last user message.
	// Since user messages now use ContentParts, update the first text part.
	a.mu.Lock()
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "user" {
			if len(a.messages[i].ContentParts) > 0 {
				// Update the first text part with the modified input.
				// The first part is always the user instruction.
				// Preserve the ContentParts structure for environment_details parts.
				a.messages[i].ContentParts[0].Text = input
			} else {
				a.messages[i].Content = input
			}
			break
		}
	}
	a.mu.Unlock()

	log.Debug("debugIntercept: user modified the message")
	return true
}

func (a *Agent) SetToolCallMode(mode string) {
	if a.toolCallModeMgr == nil {
		a.toolCallModeMgr = NewToolCallModeManager()
	}
	a.toolCallModeMgr.SetCurrentByString(mode)
	a.rebuildSystemPrompt()
	log.Info("Tool call mode set to %s", mode)
}

func (a *Agent) ToolCallMode() string {
	if a.toolCallModeMgr == nil {
		return string(ToolCallModeOpenAI)
	}
	mode := a.toolCallModeMgr.Current()
	if mode == nil {
		return string(ToolCallModeOpenAI)
	}
	return string(mode.Type)
}

func (a *Agent) SetStore(s *store.DualStore) { a.store = s }

// Store returns the agent's DualStore instance (may be nil).
func (a *Agent) Store() *store.DualStore {
	return a.store
}

// SetCurrentSessionID sets the current session's ID for tracking.
func (a *Agent) SetCurrentSessionID(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.currentSessionID = id
}

// CurrentSessionID returns the current session's ID.
func (a *Agent) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentSessionID
}

// UpdateCurrentSession writes current messages back to the current session entry and
// updates the title/keywords. Does NOT create a new entry.
func (a *Agent) UpdateCurrentSession(title, keywords string) error {
	if a.store == nil {
		return fmt.Errorf("store not available")
	}
	a.mu.Lock()
	sessionID := a.currentSessionID
	msgs := a.messages
	systemPrompt := ""
	if len(msgs) > 0 && msgs[0].Role == "system" {
		msgs = msgs[1:]
	}
	if len(msgs) > 0 && a.messages[0].Role == "system" {
		systemPrompt = a.messages[0].Content
	}
	msgData, err := json.Marshal(msgs)
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("cannot marshal messages: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}

	now := time.Now()
	if sessionID == "" {
		// No current session: create one
		randBytes := make([]byte, 4)
		randBytes[0] = byte(now.Nanosecond() & 0xFF)
		randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
		randBytes[2] = byte(now.Second() & 0xFF)
		randBytes[3] = byte(now.Minute() & 0xFF)
		sessionID = fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
		a.SetCurrentSessionID(sessionID)
	}

	// Try to load existing to preserve CreatedAt
	existing, found, _ := a.store.LoadNamedSession(sessionID)
	createdAt := now
	if found && existing != nil {
		createdAt = existing.CreatedAt
	}

	entry := &store.SessionEntry{
		ID:           sessionID,
		Title:        title,
		Keywords:     keywords,
		SystemPrompt: systemPrompt,
		Messages:     msgData,
		MessageCount: len(msgs),
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}
	return a.store.UpdateNamedSession(sessionID, entry)
}

// FlushCurrentSession writes the current agent messages to the current session entry
// without changing title/keywords. Used when switching sessions.
func (a *Agent) FlushCurrentSession() error {
	a.mu.Lock()
	sessionID := a.currentSessionID
	msgs := a.messages
	if len(msgs) > 0 && msgs[0].Role == "system" {
		msgs = msgs[1:]
	}
	msgData, err := json.Marshal(msgs)
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("cannot marshal messages: %w", err)
	}
	if sessionID == "" || len(msgs) == 0 {
		return nil
	}

	existing, found, _ := a.store.LoadNamedSession(sessionID)
	if !found || existing == nil {
		return nil
	}
	entry := &store.SessionEntry{
		ID:           existing.ID,
		Title:        existing.Title,
		Keywords:     existing.Keywords,
		SystemPrompt: existing.SystemPrompt,
		Messages:     msgData,
		MessageCount: len(msgs),
		CreatedAt:    existing.CreatedAt,
		UpdatedAt:    time.Now(),
	}
	return a.store.UpdateNamedSession(sessionID, entry)
}

// SaveCurrentSession is kept for backward compatibility but should use UpdateCurrentSession instead.
func (a *Agent) SaveCurrentSession(title, keywords string) error {
	if a.store == nil {
		return fmt.Errorf("store not available")
	}
	a.mu.Lock()
	msgs := a.messages
	systemPrompt := ""
	if len(msgs) > 0 && msgs[0].Role == "system" {
		msgs = msgs[1:]
	}
	if len(msgs) > 0 && a.messages[0].Role == "system" {
		systemPrompt = a.messages[0].Content
	}
	msgData, err := json.Marshal(msgs)
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("cannot marshal messages: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}
	now := time.Now()
	randBytes := make([]byte, 4)
	// crypto/rand is not imported, use time-based seed
	randBytes[0] = byte(now.Nanosecond() & 0xFF)
	randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
	randBytes[2] = byte(now.Second() & 0xFF)
	randBytes[3] = byte(now.Minute() & 0xFF)
	id := fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
	entry := &store.SessionEntry{
		ID:           id,
		Title:        title,
		Keywords:     keywords,
		SystemPrompt: systemPrompt,
		Messages:     msgData,
		MessageCount: len(msgs),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return a.store.SaveNamedSession(entry)
}

func (a *Agent) RestoreSession() bool {
	if a.store == nil {
		log.Debug("RestoreSession: store is nil, skipping")
		return false
	}

	// First try to load the current session ID
	sessionID, idFound, err := a.store.LoadCurrentSessionID()
	if err != nil {
		log.Warn("RestoreSession: LoadCurrentSessionID error: %v", err)
	}
	if idFound && sessionID != "" {
		// Try to load the named session by ID
		entry, entryFound, err := a.store.LoadNamedSession(sessionID)
		if err != nil {
			log.Warn("RestoreSession: LoadNamedSession(%q) error: %v", sessionID, err)
		} else if entryFound && entry != nil && len(entry.Messages) > 0 {
			var msgs []llm.Message
			if err := json.Unmarshal(entry.Messages, &msgs); err == nil && len(msgs) > 0 {
				a.mu.Lock()
				a.messages = append([]llm.Message{{Role: "system", Content: a.systemPrompt}}, msgs...)
				a.currentSessionID = sessionID
				a.mu.Unlock()
				log.Info("RestoreSession: restored %d messages from session %q (%s)", len(msgs), sessionID, entry.Title)
				return true
			}
		}
		// Entry not found or empty: ID is registered but no content yet
		a.mu.Lock()
		a.currentSessionID = sessionID
		a.mu.Unlock()
		log.Info("RestoreSession: session ID %q registered, no stored messages", sessionID)
		return true
	}

	// Legacy fallback: try old SaveSession format (SessionData as JSON in "current" key)
	a.mu.Lock()
	defer a.mu.Unlock()
	data, found, err := a.store.LoadSession()
	if err != nil {
		log.Warn("RestoreSession: LoadSession error: %v", err)
		return false
	}
	if !found {
		log.Debug("RestoreSession: no session data found in store")
		return false
	}
	log.Debug("RestoreSession: loaded %d bytes from store", len(data))

	var session store.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		log.Warn("RestoreSession: failed to unmarshal SessionData: %v, raw data: %s", err, string(data[:min(len(data), 200)]))
		return false
	}
	if len(session.Messages) == 0 {
		log.Warn("RestoreSession: SessionData has empty Messages field")
		return false
	}

	var nonSystemMessages []llm.Message
	if err := json.Unmarshal(session.Messages, &nonSystemMessages); err != nil {
		log.Warn("RestoreSession: failed to unmarshal messages array: %v", err)
		return false
	}
	log.Info("RestoreSession: legacy restore of %d non-system messages", len(nonSystemMessages))
	a.messages = append([]llm.Message{{Role: "system", Content: a.systemPrompt}}, nonSystemMessages...)
	return true
}

func (a *Agent) PersistSession() error {
	if a.store == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	msgs, err := json.Marshal(a.messages)
	if err != nil {
		return fmt.Errorf("cannot serialize messages: %w", err)
	}
	entry, err := json.Marshal(store.SessionData{
		Version:       1,
		Messages:      msgs,
		LastUpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("cannot marshal session data: %w", err)
	}
	return a.store.SaveSession(entry)
}

// PersistSessionNonSystem persists all non-system messages to storage.
// System message is excluded so the current system prompt is always fresh on restore.
func (a *Agent) PersistSessionNonSystem() error {
	if a.store == nil {
		return nil
	}
	if len(a.messages) == 0 {
		return nil
	}
	a.mu.Lock()
	// Exclude the first message if it's the system prompt
	msgs := a.messages
	if msgs[0].Role == "system" {
		msgs = msgs[1:]
	}
	if len(msgs) == 0 {
		a.mu.Unlock()
		log.Debug("PersistSessionNonSystem: only system message found, nothing to persist")
		return nil
	}
	data, err := json.Marshal(msgs)
	sessionID := a.currentSessionID
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("cannot serialize non-system messages: %w", err)
	}

	// Write to current named session entry instead of "current" key
	if sessionID == "" {
		// No current session: create one
		now := time.Now()
		randBytes := make([]byte, 4)
		randBytes[0] = byte(now.Nanosecond() & 0xFF)
		randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
		randBytes[2] = byte(now.Second() & 0xFF)
		randBytes[3] = byte(now.Minute() & 0xFF)
		sessionID = fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
		a.SetCurrentSessionID(sessionID)
	}

	// Preserve existing title/keywords/CreatedAt
	existing, found, _ := a.store.LoadNamedSession(sessionID)
	entry := &store.SessionEntry{
		ID:           sessionID,
		Title:        "",
		Keywords:     "",
		Messages:     data,
		MessageCount: len(msgs),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if found && existing != nil {
		entry.Title = existing.Title
		entry.Keywords = existing.Keywords
		entry.CreatedAt = existing.CreatedAt
	}
	if err := a.store.UpdateNamedSession(sessionID, entry); err != nil {
		log.Warn("PersistSessionNonSystem: UpdateNamedSession failed: %v", err)
	}
	// Also update "current" pointer
	if err := a.store.SaveCurrentSessionID(sessionID); err != nil {
		log.Warn("PersistSessionNonSystem: SaveCurrentSessionID failed: %v", err)
	}

	log.Debug("PersistSessionNonSystem: saved %d msgs to session %q", len(msgs), sessionID)
	return nil
}

func (a *Agent) MessagePointer() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messagePointer
}

func (a *Agent) SetPlanEnabled(enabled bool) {
	a.planEnabled = enabled
}

func (a *Agent) SetSubAgentEnabled(enabled bool) {
	a.subAgentEnabled = enabled
}

// SetShellEnabled enables or disables shell session mode.
// When enabled, it auto-starts a shell session.
// When disabled, it auto-stops any active shell session.
func (a *Agent) SetShellEnabled(enabled bool) {
	a.mu.Lock()
	a.shellEnabled = enabled
	a.mu.Unlock()

	if enabled {
		// Auto-start shell session
		if a.shellSession == nil || !a.shellSession.IsRunning() {
			sess := &shell.Session{}
			if a.cfg != nil && a.cfg.LLM.ShellVTRows > 0 && a.cfg.LLM.ShellVTCols > 0 {
				sess.SetVT(a.cfg.LLM.ShellVTRows, a.cfg.LLM.ShellVTCols)
			}
			if _, err := sess.Start(); err != nil {
				log.Warn("Failed to auto-start shell session: %v", err)
				return
			}
			a.mu.Lock()
			a.shellSession = sess
			a.mu.Unlock()
			log.Info("Shell session auto-started (shell-session-enabled=on)")
		}
	} else {
		// Auto-stop shell session
		a.CloseShellSession()
	}
}

// IsShellEnabled returns whether the shell session mode is enabled.
func (a *Agent) IsShellEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.shellEnabled
}

// CloseShellSession closes the active shell session if one exists.
func (a *Agent) CloseShellSession() {
	a.mu.Lock()
	sess := a.shellSession
	a.shellSession = nil
	a.mu.Unlock()

	if sess != nil {
		sess.Close()
		log.Info("Shell session closed (shell-session-enabled=off)")
	}
}

// EnsureShellSession starts a shell session if one is not already running.
// This is called on startup when shell-session-enabled=on.
func (a *Agent) EnsureShellSession() {
	if !a.shellEnabled {
		return
	}
	a.mu.Lock()
	hasSession := a.shellSession != nil && a.shellSession.IsRunning()
	a.mu.Unlock()
	if !hasSession {
		a.SetShellEnabled(true)
	}
}

func (a *Agent) SetConfig(cfg *config.Config) {
	a.cfg = cfg
	a.rebuildSystemPrompt()
	// Configure Excel session manager with max sessions
	if a.excelSessionMgr != nil {
		a.excelSessionMgr.Configure(0, cfg.LLM.ExcelMaxSessions)
	}
	// Configure DOCX session manager with max sessions
	if a.docxSessionMgr != nil {
		a.docxSessionMgr.Configure(cfg.LLM.DocxMaxSessions)
	}
}

func (a *Agent) SetLLMClient(client llm.Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.llmClient != nil {
		a.llmClient.Close()
	}
	a.llmClient = client
	log.Info("LLM client replaced at runtime")
}

// VaultStore returns the vault store instance (may be nil).
func (a *Agent) VaultStore() *store.VaultStore {
	return a.vaultStore
}

// SetVaultStore sets the vault store instance.
func (a *Agent) SetVaultStore(vs *store.VaultStore) {
	a.vaultStore = vs
}

func (a *Agent) GetLLMClient() llm.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.llmClient
}

func (a *Agent) rebuildSystemPrompt() {
	// Reload config from disk to ensure system prompt always uses the latest
	// configuration (WorkModes, PromptSections, agent identity, etc.).
	// mode/*.md files are already read from disk each time by loadSectionText.
	//
	// NOTE: We do NOT replace a.cfg with freshCfg here because SettingsHandler
	// and Agent share the same config pointer. Replacing it would break
	// runtime config synchronization — settings changed via :set would not
	// be visible to the Agent's loop detection and other runtime paths that
	// read from a.cfg directly. Only the system prompt sections are rebuilt.
	if a.cfg != nil {
		if cfgPath := a.cfg.ConfigPath(); cfgPath != "" {
			if freshCfg, _, err := config.LoadFromFile(cfgPath, nil); err == nil {
				// Copy WorkModes from freshCfg to keep
				// system prompt sections up to date without replacing the
				// shared config pointer.
				a.cfg.WorkModes = freshCfg.WorkModes
				// Copy LoopIntervention from disk config only if the fresh
				// config has a non-empty value. When a.cfg is initialized
				// via SetConfig() with the correct value from memory, but
				// disk config.json has no "loop_intervention" field (empty
				// default), we must NOT overwrite the cached value with "".
				// Otherwise rebuildSystemPrompt() called after SetConfig()
				// and at the start of every RunStream iteration would erase
				// the "retry" that was correctly set from the in-memory
				// SettingsHandler config pointer.
				if freshCfg.LLM.LoopIntervention != "" {
					a.cfg.LLM.LoopIntervention = freshCfg.LLM.LoopIntervention
				}
			}
		}
	}

	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	userName := ""
	channel := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		// Resolve description with priority:
		// 1. Mode-specific description from ModeDescriptions
		// 2. Global AgentDescription
		// 3. Mode-specific i18n default (act/plan/research)
		// 4. Global i18n default
		workMode := a.cfg.LLM.WorkMode
		if workMode == "" {
			workMode = "act"
		}
		// Try mode-specific description first
		if a.cfg.LLM.ModeDescriptions != nil {
			if md, ok := a.cfg.LLM.ModeDescriptions[workMode]; ok && md != "" {
				agentDesc = md
			}
		}
		// Fall back to global description
		if agentDesc == "" {
			agentDesc = a.cfg.LLM.AgentDescription
		}
		// Fall back to mode-specific i18n default
		if agentDesc == "" {
			switch workMode {
			case "plan":
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionPlan)
			case "research":
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionResearch)
			default:
				agentDesc = i18n.T(i18n.KeyAgentDefaultDescriptionAct)
			}
		}
		// Fall back to global i18n default
		if agentDesc == "" {
			agentDesc = i18n.T(i18n.KeyAgentDefaultDescription)
		}
		agentPrinciples = a.cfg.LLM.AgentPrinciples
		userName = a.cfg.LLM.UserName
		channel = a.cfg.LLM.Channel
	}

	toolUsageText := ""
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Mode()
		if mode == ToolCallModeXML {
			tools := a.buildToolsInternal()
			lang := string(i18n.GetLang())
			workMode := ""
			if a.cfg != nil {
				workMode = a.cfg.LLM.WorkMode
			}
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang, workMode)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.getCurrentTaskDescription()

	a.systemPrompt = buildSystemPromptWithMode(a.cfg, a.rules, a.resultMode, a.shellEnabled, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)
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

func (a *Agent) SetWorkspacePath(path string)            { a.workspacePath = path }
func (a *Agent) SetImagePaths(paths []string)            { a.imagePaths = paths }
func (a *Agent) SetModelManager(mm *config.ModelManager) { a.modelManager = mm }

// selectModelForCall selects the appropriate model based on vision requirements
// and the current work mode's model bindings.
func (a *Agent) selectModelForCall() *config.ModelConfig {
	if a.modelManager == nil {
		return nil
	}

	// Determine which model ID to use based on current work mode
	modelID := a.getModelIDForCall()
	if modelID != "" {
		// Look up the model by ID in cfg.Models
		if a.cfg != nil {
			for _, m := range a.cfg.Models {
				if m.ID == modelID && m.Enabled {
					return m
				}
			}
		}
		// Fallback: try ModelManager
		if m := a.modelManager.GetModel(modelID); m != nil && m.Enabled {
			return m
		}
	}

	// No mode-specific model: use global priority
	visionRequired := len(a.imagePaths) > 0
	return a.modelManager.GetActiveModel(visionRequired)
}

// getModelIDForCall returns the model ID to use based on the current work mode.
// Returns the VisionModelID if vision is needed and set, otherwise ModelID.
// Returns empty string if neither is set (use global).
func (a *Agent) getModelIDForCall() string {
	if a.cfg == nil {
		return ""
	}
	workModeName := a.cfg.LLM.WorkMode
	if workModeName == "" {
		workModeName = "act"
	}

	// Search user-defined modes first, then built-in defaults
	var mode *config.WorkMode
	for i := range a.cfg.WorkModes {
		if a.cfg.WorkModes[i].Name == workModeName {
			mode = &a.cfg.WorkModes[i]
			break
		}
	}
	if mode == nil {
		for _, m := range config.DefaultWorkModes() {
			if m.Name == workModeName {
				mode = &m // note: this is a copy, but we only read ModelID/VisionModelID
				break
			}
		}
	}
	if mode == nil {
		return ""
	}

	visionRequired := len(a.imagePaths) > 0
	if visionRequired && mode.VisionModelID != nil {
		return *mode.VisionModelID
	}
	if mode.ModelID != nil {
		return *mode.ModelID
	}
	return ""
}

// ApplyWorkModeConfig creates a new LLM client using the current work mode's
// model binding and parameter overrides. Parameter priority:
//  1. WorkMode overrides (highest)
//  2. ModelConfig overrides (model-level)
//  3. Global cfg.LLM defaults (lowest)
//
// Call this when switching modes or when RunStream needs to establish the client.
func (a *Agent) ApplyWorkModeConfig() {
	if a.cfg == nil {
		return
	}

	// Step 1: Select the model
	var mode *config.WorkMode
	workModeName := a.cfg.LLM.WorkMode
	if workModeName == "" {
		workModeName = "act"
	}
	for i := range a.cfg.WorkModes {
		if a.cfg.WorkModes[i].Name == workModeName {
			mode = &a.cfg.WorkModes[i]
			break
		}
	}
	if mode == nil {
		for i, m := range config.DefaultWorkModes() {
			if m.Name == workModeName {
				mode = &config.DefaultWorkModes()[i]
				break
			}
		}
	}

	modelID := a.getModelIDForCall()
	var modelCfg *config.ModelConfig
	if modelID != "" {
		for _, m := range a.cfg.Models {
			if m.ID == modelID && m.Enabled {
				modelCfg = m
				break
			}
		}
	}
	if modelCfg == nil {
		visionRequired := len(a.imagePaths) > 0
		if a.modelManager != nil {
			modelCfg = a.modelManager.GetActiveModel(visionRequired)
		}
		if modelCfg == nil {
			modelCfg = config.GetActiveModelFromConfig(a.cfg)
		}
	}
	if modelCfg == nil {
		log.Warn("applyWorkModeConfig: no model config found, cannot switch")
		return
	}

	// Step 2: Merge parameters (mode > model config > global)
	temperature := a.cfg.LLM.Temperature
	if modelCfg.Temperature != nil {
		temperature = *modelCfg.Temperature
	}
	if mode != nil && mode.Temperature != nil {
		temperature = *mode.Temperature
	}

	maxTokens := a.cfg.LLM.MaxTokens
	if modelCfg.MaxTokens != nil {
		maxTokens = *modelCfg.MaxTokens
	}
	if mode != nil && mode.MaxTokens != nil {
		maxTokens = *mode.MaxTokens
	}

	thinkingEnabled := a.cfg.LLM.ThinkingEnabled == "on"
	if a.cfg.LLM.ThinkingEnabled == "default" {
		if modelCfg.ThinkingEnabled != nil {
			thinkingEnabled = *modelCfg.ThinkingEnabled
		} else {
			thinkingEnabled = false
		}
	}
	if mode != nil && mode.ThinkingEnabled != nil {
		thinkingEnabled = *mode.ThinkingEnabled
	}

	reasoningEffort := a.cfg.LLM.ReasoningEffort
	if modelCfg.ReasoningEffort != nil {
		reasoningEffort = *modelCfg.ReasoningEffort
	}
	if mode != nil && mode.ReasoningEffort != nil {
		reasoningEffort = *mode.ReasoningEffort
	}

	topP := a.cfg.LLM.TopP
	if modelCfg.TopP != nil {
		topP = *modelCfg.TopP
	}
	if mode != nil && mode.TopP != nil {
		topP = *mode.TopP
	}

	topK := a.cfg.LLM.TopK
	if modelCfg.TopK != nil {
		topK = *modelCfg.TopK
	}
	if mode != nil && mode.TopK != nil {
		topK = *mode.TopK
	}

	repetitionPenalty := a.cfg.LLM.RepetitionPenalty
	if modelCfg.RepetitionPenalty != nil {
		repetitionPenalty = *modelCfg.RepetitionPenalty
	}
	if mode != nil && mode.RepetitionPenalty != nil {
		repetitionPenalty = *mode.RepetitionPenalty
	}

	// Create the LLM client
	newClient := llm.NewClient(
		modelCfg.Endpoint, modelCfg.APIKey, modelCfg.Model,
		temperature, maxTokens, a.cfg.LLM.LLMTimeout,
	)
	newClient.SetTopP(topP)
	newClient.SetTopK(topK)
	newClient.SetRepetitionPenalty(repetitionPenalty)
	newClient.SetTokenUsage(a.cfg.LLM.TokenUsage)

	// Merge body additions: cfg body additions + thinking adapter + model custom params
	mergedAdditions := make(map[string]string)
	if len(a.cfg.LLM.BodyAdditions) > 0 {
		for k, v := range a.cfg.LLM.BodyAdditions {
			mergedAdditions[k] = v
		}
	}
	adapter := llm.GetThinkingAdapter(modelCfg.Provider)
	thinkingMode := llm.ThinkingModeDisabled
	if thinkingEnabled {
		thinkingMode = llm.ThinkingModeEnabled
	}
	thinkingAdditions := adapter.BuildAdditions(llm.ThinkingConfig{
		Mode:            thinkingMode,
		ReasoningEffort: reasoningEffort,
	})
	for k, v := range thinkingAdditions {
		mergedAdditions[k] = v
	}
	if len(modelCfg.CustomParams) > 0 {
		for k, v := range modelCfg.CustomParams {
			if strVal, ok := v.(string); ok && strVal == "None" {
				delete(mergedAdditions, k)
				continue
			}
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				log.Warn("Failed to marshal CustomParam %s: %v", k, err)
				continue
			}
			mergedAdditions[k] = string(jsonBytes)
		}
	}
	if len(mergedAdditions) > 0 {
		newClient.SetBodyAdditions(mergedAdditions)
	}

	// Update mode-level config settings that affect agent behavior
	if mode != nil && mode.MaxIterations != nil {
		a.SetMaxIterations(*mode.MaxIterations)
	}
	if mode != nil && mode.ContextLimit != nil {
		if a.cfg != nil {
			a.cfg.LLM.ContextLimit = *mode.ContextLimit
		}
	}
	if mode != nil && mode.ToolCallMode != nil {
		a.SetToolCallMode(*mode.ToolCallMode)
	}

	a.SetLLMClient(newClient)
	log.Info("applyWorkModeConfig: switched to model=%s, temperature=%.2f, maxTokens=%d, vision=%v (mode=%s)",
		modelCfg.Model, temperature, maxTokens, modelCfg.Capabilities.Vision, workModeName)
}

func (a *Agent) getTaskPlanText() string {
	if a.taskPlanMgr == nil {
		return ""
	}
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil || plan == nil {
		return ""
	}
	if !a.taskPlanMgr.HasUnfinished() {
		return ""
	}
	return taskplan.FormatPlan(plan)
}

func (a *Agent) formatXMLToolResult(toolName, toolArgs, toolResult string, messageNo int) string {
	template := i18n.T(i18n.KeyXMLToolResultTemplate)
	result := strings.ReplaceAll(template, "{TOOL_CALL}", toolName)
	result = strings.ReplaceAll(result, "{TOOL_CALL_PARAMETERS}", toolArgs)
	result = strings.ReplaceAll(result, "{TOOL_RESULT}", toolResult)
	result = strings.ReplaceAll(result, "{TASK_TRACKING}", a.getTaskPlanPrompt())
	result = strings.ReplaceAll(result, "{MESSAGE_NO}", strconv.Itoa(messageNo))
	result = strings.ReplaceAll(result, "{CURRENT_TIME}", time.Now().Format("2006-01-02 15:04:05 Monday"))
	return result
}

// buildUserMessage creates a structured user Message with ContentParts.
// This ensures all user messages use the array format for content:
//
//	[{"type":"text","text":"instruction"}, {"type":"text","text":"<environment_details>..."}]
//
// Part 0: user instruction (wrapped in <task> tags for XML mode, raw text for OpenAI mode)
// Part N: environment_details will be appended by injectEnvelopeToLastUser as ContentPart
//
// Regardless of tool call mode, using ContentParts separates the user's instruction
// from environment context, making it clearer for the LLM.
func (a *Agent) buildUserMessage(instruction string) llm.Message {
	msg := llm.Message{Role: "user"}
	text := instruction
	if a.isXMLMode() {
		text = fmt.Sprintf("<task>\n%s\n</task>", instruction)
	}
	msg.ContentParts = []llm.ContentPart{
		{Type: llm.ContentPartText, Text: text},
	}
	return msg
}

// formatUserMessage is kept for backward compatibility but is no longer used
// for building new messages. All user messages now use buildUserMessage which
// produces structured ContentParts.
func (a *Agent) formatUserMessage(instruction string, messageNo int) string {
	template := i18n.T(i18n.KeyUserMessageTemplate)
	result := strings.ReplaceAll(template, "{INSTRUCTION}", instruction)
	return result
}

// buildXMLToolResultMessage creates a structured user Message with ContentParts for XML mode
// tool results. Each tool result is a separate text part.
// Part 0: tool result text
// Part N: environment_details will be appended by injectTimeAndMessageNo as ContentPart
func (a *Agent) buildXMLToolResultMessage(toolName, toolArgs, toolResult string, messageNo int) llm.Message {
	msg := llm.Message{Role: "user"}
	template := i18n.T(i18n.KeyXMLToolResultTemplate)
	formatted := strings.ReplaceAll(template, "{TOOL_CALL}", toolName)
	formatted = strings.ReplaceAll(formatted, "{TOOL_CALL_PARAMETERS}", toolArgs)
	formatted = strings.ReplaceAll(formatted, "{TOOL_RESULT}", toolResult)
	formatted = strings.ReplaceAll(formatted, "{TASK_TRACKING}", a.getTaskPlanPrompt())
	formatted = strings.ReplaceAll(formatted, "{MESSAGE_NO}", strconv.Itoa(messageNo))
	formatted = strings.ReplaceAll(formatted, "{CURRENT_TIME}", time.Now().Format("2006-01-02 15:04:05 Monday"))
	msg.EnsureContentParts()
	msg.ContentParts[0].Text = formatted
	return msg
}

// getCurrentTaskDescription returns the current task description for {TASK} in the
// system prompt. Priority:
// 1. Active task plan title (if one exists with unfinished steps)
// 2. The first user message at or after the messagePointer (context start)
// Returns empty string if neither is available.
// getProblemModelID returns the problem-solving model ID for the current work mode.
// Priority: ProblemModelID > ModelID > "" (use global fallback).
func (a *Agent) getProblemModelID() string {
	if a.cfg == nil {
		return ""
	}
	workModeName := a.cfg.LLM.WorkMode
	if workModeName == "" {
		workModeName = "act"
	}
	var mode *config.WorkMode
	for i := range a.cfg.WorkModes {
		if a.cfg.WorkModes[i].Name == workModeName {
			mode = &a.cfg.WorkModes[i]
			break
		}
	}
	if mode == nil {
		for _, m := range config.DefaultWorkModes() {
			if m.Name == workModeName {
				mode = &m
				break
			}
		}
	}
	if mode == nil {
		return ""
	}
	if mode.ProblemModelID != nil {
		return *mode.ProblemModelID
	}
	if mode.ModelID != nil {
		return *mode.ModelID
	}
	return ""
}

func (a *Agent) getCurrentTaskDescription() string {
	// Priority 1: active task plan with unfinished steps
	if a.taskPlanMgr != nil && a.taskPlanMgr.HasUnfinished() {
		plan, err := a.taskPlanMgr.GetCurrent()
		if err == nil && plan != nil && plan.Title != "" {
			return plan.Title
		}
	}
	// Priority 2: first user message at/after messagePointer
	// User messages now use ContentParts (array format). Use CombineContentParts()
	// to get the full text, and strip environment_details for a clean task description.
	a.mu.Lock()
	defer a.mu.Unlock()
	startIdx := 1 // skip system prompt (index 0)
	if a.messagePointer > 0 && a.messagePointer < len(a.messages) {
		startIdx = a.messagePointer
	}
	for i := startIdx; i < len(a.messages); i++ {
		if a.messages[i].Role != "user" {
			continue
		}
		var content string
		if len(a.messages[i].ContentParts) > 0 {
			content = a.messages[i].CombineContentParts()
		} else {
			content = a.messages[i].Content
		}
		if content == "" {
			continue
		}
		// Strip <environment_details> if present for cleaner display
		if envStart := strings.Index(content, "<environment_details>"); envStart > 0 {
			content = strings.TrimSpace(content[:envStart])
		}
		// Truncate to reasonable length
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		return content
	}
	return ""
}

func (a *Agent) getTaskPlanPrompt() string {
	if a.taskPlanMgr == nil {
		return ""
	}
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil || plan == nil {
		return ""
	}
	if a.taskPlanMgr.HasUnfinished() {
		planText := taskplan.FormatPlan(plan)
		template := i18n.T(i18n.KeyToolResultWithPlan)
		return strings.ReplaceAll(template, "{TASK_PLAN}", planText)
	}
	return i18n.T(i18n.KeyToolResultNoPlan)
}

// Interrupt signals the agent to stop receiving LLM stream data (ESC key).
// Multiple calls are safe; subsequent signals are no-ops until ResetInterrupt.
func (a *Agent) Interrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	select {
	case a.interruptCh <- struct{}{}:
	default:
	}
}

// InterruptChan returns the interrupt channel for select-based listening.
func (a *Agent) InterruptChan() <-chan struct{} {
	return a.interruptCh
}

// ResetInterrupt re-creates the interrupt channel for a new request.
func (a *Agent) ResetInterrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.interruptCh = make(chan struct{}, 1)
}

// Cancel signals the agent to immediately abort the current task (Ctrl+C).
// Unlike Interrupt, this causes an immediate exit to the REPL prompt
// without any confirmation prompt. Multiple calls are safe; subsequent
// signals are no-ops until ResetCancel (FEATURE-239).
func (a *Agent) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	select {
	case a.cancelCh <- struct{}{}:
	default:
	}
}

// CancelChan returns the cancel channel for select-based listening.
func (a *Agent) CancelChan() <-chan struct{} {
	return a.cancelCh
}

// ResetCancel re-creates the cancel channel for a new request.
func (a *Agent) ResetCancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelCh = make(chan struct{}, 1)
}

func (a *Agent) TaskPlanManager() *taskplan.Manager  { return a.taskPlanMgr }
func (a *Agent) SetScheduler(s *scheduler.Scheduler) { a.scheduler = s }
func (a *Agent) Scheduler() *scheduler.Scheduler     { return a.scheduler }

func (a *Agent) SetResultMode(mode config.ResultMode) {
	// Build system prompt outside the lock to avoid deadlock:
	// getCurrentTaskDescription() acquires a.mu internally.
	a.mu.Lock()
	a.resultMode = mode
	a.mu.Unlock()

	agentName := ""
	agentDesc := ""
	agentPrinciples := ""
	userName := ""
	channel := ""
	if a.cfg != nil {
		agentName = a.cfg.LLM.AgentName
		agentDesc = a.cfg.LLM.AgentDescription
		agentPrinciples = a.cfg.LLM.AgentPrinciples
		userName = a.cfg.LLM.UserName
		channel = a.cfg.LLM.Channel
	}

	toolUsageText := ""
	if a.toolCallModeMgr != nil {
		mode := a.toolCallModeMgr.Mode()
		if mode == ToolCallModeXML {
			tools := a.buildToolsInternal()
			lang := string(i18n.GetLang())
			workMode := ""
			if a.cfg != nil {
				workMode = a.cfg.LLM.WorkMode
			}
			toolUsageText = BuildToolUsagePrompt(ToolCallModeXML, tools, lang, workMode)
		}
	}

	taskPlanText := a.getTaskPlanText()
	taskDesc := a.getCurrentTaskDescription()

	a.systemPrompt = buildSystemPromptWithMode(a.cfg, a.rules, mode, a.shellEnabled, agentName, agentDesc, agentPrinciples, userName, channel, taskDesc, taskPlanText, toolUsageText)

	a.mu.Lock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	a.mu.Unlock()
	log.Info("Result mode set to %s, system prompt rebuilt", config.ResultModeString(mode))
}

func (a *Agent) getToolTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.ToolTimeout > 0 {
		return time.Duration(a.cfg.LLM.ToolTimeout) * time.Second
	}
	return 0
}

func (a *Agent) getCommandTimeout() time.Duration {
	if a.cfg != nil && a.cfg.LLM.CommandTimeout > 0 {
		return time.Duration(a.cfg.LLM.CommandTimeout) * time.Second
	}
	return 0
}

func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	log.Info("Agent history reset")
}

func (a *Agent) GetHistory() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messages
}

func (a *Agent) SetHistory(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = messages
}

func (a *Agent) GetMessages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

func (a *Agent) adjustMessagePointer() {
	for a.messagePointer > 0 && a.messages[a.messagePointer].Role == "tool" {
		a.messagePointer--
	}
}

// SetBrowserEnabled enables or disables browser tools.
func (a *Agent) SetBrowserEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.browserEnabled = enabled
	if enabled && a.chromeMgr == nil {
		// Browser manager will be initialized when first tool call is made
		log.Info("Browser tools enabled (will auto-start Chrome on first use)")
	} else if !enabled && a.chromeMgr != nil {
		a.chromeMgr.Stop()
		a.chromeMgr = nil
		log.Info("Browser tools disabled, Chrome stopped")
	}
	// Rebuild system prompt to include/exclude browser tool descriptions
	// Run in goroutine to avoid deadlock with mu
	go a.rebuildSystemPrompt()
}

// EnsureBrowser prepares Chrome for the agent if browser is enabled.
// Called during initialization to pre-launch Chrome when configured.
// IsBrowserEnabled returns whether browser tools are enabled.
func (a *Agent) IsBrowserEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.browserEnabled
}

// EnsureBrowserStarted ensures a Chrome browser instance is available.
// It first tries to connect to an already-running Chrome on the configured
// remote debugging port. Only falls back to starting a new Chrome instance
// if no existing instance is detected. This prevents creating duplicate
// browser windows when co-shell restarts or when Chrome is already running.
func (a *Agent) EnsureBrowserStarted() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.chromeMgr != nil && a.chromeMgr.IsRunning() {
		return nil
	}

	if a.cfg == nil {
		return fmt.Errorf("config not set")
	}

	port := a.cfg.LLM.BrowserPort
	if port <= 0 {
		port = 9222
	}

	// Use a persistent browser data directory under the workspace, so
	// Chrome state (cookies, sessions, downloads) survives co-shell restarts.
	// This also makes it possible to trace back issues from browser data.
	browserDataDir := filepath.Join(a.workspacePath, "browser-data")

	// Step 1: Try to reuse an already-running Chrome instance on the same port.
	// This avoids creating a new browser window when co-shell restarts or when
	// Chrome was left running from a previous session.
	debugURL := fmt.Sprintf("http://localhost:%d", port)
	if browser.IsEndpointAvailable(debugURL) {
		// Existing Chrome detected — create a ChromeManager without starting a new process.
		log.Info("Browser detected on port %d, reusing existing instance", port)
		mgr := browser.NewChromeManager(port, a.cfg.LLM.BrowserHeadless, browserDataDir)
		mgr.SetStarted() // Mark as started so Start() won't launch a new process
		a.chromeMgr = mgr
		return nil
	}

	// Step 2: No existing Chrome — start a new one.
	mgr := browser.NewChromeManager(port, a.cfg.LLM.BrowserHeadless, browserDataDir)
	if _, err := mgr.Start(); err != nil {
		return fmt.Errorf("cannot start browser: %w", err)
	}

	a.chromeMgr = mgr
	log.Info("Browser started (port=%d, headless=%v)", port, a.cfg.LLM.BrowserHeadless)
	return nil
}

// CloseBrowser stops the Chrome browser if running.
func (a *Agent) CloseBrowser() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.chromeMgr != nil {
		a.chromeMgr.Stop()
		a.chromeMgr = nil
		a.browserScreenshotData = ""
		log.Info("Browser closed")
	}
}

func (a *Agent) removeLastAssistantWithToolCalls() string {
	lastAssistantIdx := -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "assistant" && len(a.messages[i].ToolCalls) > 0 {
			lastAssistantIdx = i
			break
		}
	}
	if lastAssistantIdx < 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- 已移除的消息 (从索引 %d 开始) ---\n", lastAssistantIdx))
	for i := lastAssistantIdx; i < len(a.messages); i++ {
		msg := a.messages[i]
		sb.WriteString(fmt.Sprintf("[%d] role=%s", i, msg.Role))
		if msg.Content != "" {
			sb.WriteString(fmt.Sprintf(", content=%q", msg.Content))
		}
		if len(msg.ToolCalls) > 0 {
			sb.WriteString(fmt.Sprintf(", tool_calls=%d", len(msg.ToolCalls)))
			for j, tc := range msg.ToolCalls {
				sb.WriteString(fmt.Sprintf("\n    tool_call[%d]: name=%q, id=%q, args=%q", j, tc.Name, tc.ID, tc.Arguments))
			}
		}
		if msg.ToolCallID != "" {
			sb.WriteString(fmt.Sprintf(", tool_call_id=%q", msg.ToolCallID))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("--- 结束 ---")
	a.messages = a.messages[:lastAssistantIdx]
	return sb.String()
}
