// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-07-11
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
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// SessionHandler handles the .session built-in command.
type SessionHandler struct {
	agent *agent.Agent
	cfg   *config.Config
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(ag *agent.Agent, cfg *config.Config) *SessionHandler {
	return &SessionHandler{
		agent: ag,
		cfg:   cfg,
	}
}

// Handle processes the .session command.
func (h *SessionHandler) Handle(args []string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return h.showInteractive()
	}

	switch args[0] {
	case "?":
		return h.showHelp()
	case "help":
		return h.showHelp()
	case "export":
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		return h.handleExport(path)
	case "import":
		if len(args) < 2 {
			return "", fmt.Errorf(i18n.T(i18n.KeySessionMigUseImport))
		}
		return h.handleImport(args[1])
	case "save":
		title := ""
		if len(args) > 1 {
			title = strings.Join(args[1:], " ")
		}
		return h.handleSave(title)
	case "list":
		return h.handleList()
	case "switch":
		if len(args) < 2 {
			return "", fmt.Errorf(i18n.T(i18n.KeySessionMigUseSwitch))
		}
		return h.handleSwitch(args[1])
	case "delete":
		if len(args) < 2 {
			return "", fmt.Errorf(i18n.T(i18n.KeySessionMigUseDelete))
		}
		return h.handleDelete(args[1])
	case "pop":
		if len(args) > 2 && args[1] == "to" {
			v, err := strconv.Atoi(args[2])
			if err != nil || v < 0 {
				return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNonNeg), args[2])
			}
			return h.popTo(v)
		}
		n := 1
		if len(args) > 1 {
			v, err := strconv.Atoi(args[1])
			if err != nil || v <= 0 {
				return "", fmt.Errorf(i18n.T(i18n.KeySessionMigPositive), args[1])
			}
			n = v
		}
		return h.popMessages(n)
	default:
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigUnknownSub), args[0])
	}
}

func (h *SessionHandler) showInteractive() (string, error) {
	for {
		// First show current session stats
		currentInfo, _ := h.showSession()
		fmt.Println(currentInfo)

		// Then show session list (dynamic message count, marks current with *)
		fmt.Println()
		if err := h.showListInteractive(); err != nil {
			return "", err
		}
		fmt.Println()

		// Show wizard-style menu
		fmt.Println(i18n.T(i18n.KeySessionMigOpsTitle))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsNum))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsExport))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsImport))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsDelete))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsPop))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsNew))
		fmt.Println(i18n.T(i18n.KeySessionMigOpsQuit))
		fmt.Print(i18n.T(i18n.KeySessionMigOpsChoose))

		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle single-letter commands
		switch strings.ToLower(input) {
		case "q":
			return "", nil
		case "e":
			return h.handleExport("")
		case "i":
			fmt.Print(i18n.T(i18n.KeySessionMigFilePrompt))
			var path string
			fmt.Scanln(&path)
			path = strings.TrimSpace(path)
			if path == "" {
				return "", fmt.Errorf(i18n.T(i18n.KeySessionMigCancelled))
			}
			return h.handleImport(path)
		case "d":
			return h.handleDeleteWithList()
		case "p":
			return h.popMessages(1)
		case "n":
			nextN := h.nextSessionNumber()
			title := fmt.Sprintf(i18n.T(i18n.KeySessionMigTitle), nextN)
			h.agent.Reset()
			now := time.Now()
			randBytes := make([]byte, 4)
			randBytes[0] = byte(now.Nanosecond() & 0xFF)
			randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
			randBytes[2] = byte(now.Second() & 0xFF)
			randBytes[3] = byte(now.Minute() & 0xFF)
			sessionID := fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
			entry := &store.SessionEntry{
				ID:           sessionID,
				Title:        title,
				Keywords:     "",
				SystemPrompt: "",
				Messages:     []byte("[]"),
				MessageCount: 0,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err2 := h.agent.Store().SaveNamedSession(entry); err2 != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeySessionMigSaveFail), err2)
			}
			h.agent.SetCurrentSessionID(sessionID)
			h.agent.Store().SaveCurrentSessionID(sessionID)
			return fmt.Sprintf(i18n.T(i18n.KeySessionMigCreated), title), nil
		default:
			// Try as a number (switch)
			if n, err := strconv.Atoi(input); err == nil && n > 0 {
				return h.handleSwitch(strconv.Itoa(n))
			}
			fmt.Printf(i18n.T(i18n.KeySessionMigUnknownOp), input)
			continue
		}
	}
}

func (h *SessionHandler) handleDeleteWithList() (string, error) {
	entries, err := h.agent.Store().ListNamedSessions()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return i18n.T(i18n.KeySessionMigNoneDelete), nil
	}

	fmt.Print(i18n.T(i18n.KeySessionMigDelPrompt))
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)
	if input == "" {
		return i18n.T(i18n.KeySessionMigCancelled), nil
	}
	return h.handleDelete(input)
}

func (h *SessionHandler) nextSessionNumber() int {
	if entries, err := h.agent.Store().ListNamedSessions(); err == nil {
		maxN := 0
		for _, e := range entries {
			var suffix int
			if _, err := fmt.Sscanf(e.Title, i18n.T(i18n.KeySessionMigTitle), &suffix); err == nil && suffix > maxN {
				maxN = suffix
			}
		}
		return maxN + 1
	}
	return 1
}

func (h *SessionHandler) showListInteractive() error {
	entries, err := h.agent.Store().ListNamedSessions()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println(i18n.T(i18n.KeySessionListEmpty))
		return nil
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	currentID := h.agent.CurrentSessionID()
	fmt.Println(i18n.T(i18n.KeySessionListTitle))
	fmt.Printf("  %s  %-30s %-24s %5s  %s\n", i18n.T(i18n.KeySessionNumber), i18n.T(i18n.KeySessionTitleLabel), i18n.T(i18n.KeySessionKeywords), i18n.T(i18n.KeySessionMessageCount), i18n.T(i18n.KeySessionCreatedAt))

	for i, entry := range entries {
		// Dynamically compute message count
		var msgs []llm.Message
		msgCount := 0
		if err := json.Unmarshal(entry.Messages, &msgs); err == nil {
			msgCount = len(msgs)
		}
		title := entry.Title
		if title == "" {
			title = "(unnamed)"
		}
		keywords := entry.Keywords
		if keywords == "" {
			keywords = "-"
		}
		marker := " "
		if entry.ID == currentID {
			marker = "*"
		}
		fmt.Printf("  %s%3d  %-30s %-24s %5d  %s\n",
			marker, i+1, title, keywords, msgCount,
			entry.CreatedAt.Format("2006-01-02 15:04"))
	}
	return nil
}

func (h *SessionHandler) showHelp() (string, error) {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpTitle))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpInfo))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpPop))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpPopTo))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpExport))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpImport))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpSave))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpList))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpSwitch))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpDelete))
	sb.WriteString(i18n.T(i18n.KeySessionMigHelpHelp))
	return sb.String(), nil
}

func (h *SessionHandler) handleExport(filePath string) (string, error) {
	if filePath == "" {
		filePath = fmt.Sprintf("session-%s.cosh-session.json", time.Now().Format("20060102-150405"))
		fmt.Println(i18n.TF(i18n.KeySessionExportDefaultPath, filePath))
	} else {
		// If filePath doesn't end with .json, append the extension
		if !strings.HasSuffix(strings.ToLower(filePath), ".cosh-session.json") {
			filePath = filePath + ".cosh-session.json"
		}
	}

	messages := h.agent.Messages()
	var nonSystem []llm.Message
	systemPrompt := ""
	for i, msg := range messages {
		if i == 0 && msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		nonSystem = append(nonSystem, msg)
	}

	export := store.SessionExport{
		Version:      1,
		ExportedAt:   time.Now(),
		Title:        "",
		Keywords:     "",
		SystemPrompt: systemPrompt,
		MessageCount: len(nonSystem),
		Messages:     nonSystem,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionExportFailed, err))
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionExportFailed, err))
	}

	return i18n.TF(i18n.KeySessionExportDone, filePath), nil
}

func (h *SessionHandler) handleImport(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionImportFailed, err))
	}

	var export store.SessionExport
	if err := json.Unmarshal(data, &export); err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionImportFailed, err))
	}

	if len(export.Messages) == 0 {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionImportFailed, "empty messages"))
	}

	fmt.Printf("%s %s\n", i18n.T(i18n.KeySessionImport), filePath)
	if export.Title != "" {
		fmt.Printf("  %s: %s\n", i18n.T(i18n.KeySessionTitleLabel), export.Title)
		if export.Keywords != "" {
			fmt.Printf("  %s: %s\n", i18n.T(i18n.KeySessionKeywords), export.Keywords)
		}
	}
	fmt.Printf("  %s: %d\n", i18n.T(i18n.KeySessionMessageCount), len(export.Messages))
	fmt.Printf("  %s: %s\n", i18n.T(i18n.KeySessionCreatedAt), export.ExportedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n%s / %s\n", i18n.T(i18n.KeySessionConfirmReplace), i18n.T(i18n.KeySessionConfirmAppend))
	fmt.Printf(i18n.T(i18n.KeySessionMigReplaceAp))

	response := ""
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "a" {
		// Append: keep current messages, add imported ones
		current := h.agent.Messages()
		current = append(current, export.Messages...)
		h.agent.SetHistory(current)
	} else {
		// Replace: keep system prompt, replace rest with imported messages
		current := h.agent.Messages()
		systemPrompt := ""
		if len(current) > 0 && current[0].Role == "system" {
			systemPrompt = current[0].Content
		}
		newMsgs := []llm.Message{{Role: "system", Content: systemPrompt}}
		newMsgs = append(newMsgs, export.Messages...)
		h.agent.SetHistory(newMsgs)
	}

	return i18n.TF(i18n.KeySessionImportDone, filePath, len(export.Messages)), nil
}

func (h *SessionHandler) handleSave(title string) (string, error) {
	if title == "" {
		title = fmt.Sprintf(i18n.T(i18n.KeySessionMigIDFmt), time.Now().Format("2006-01-02 15:04:05"))
	}

	if err := h.agent.SaveCurrentSession(title, ""); err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionSaveFailed, err))
	}

	return i18n.TF(i18n.KeySessionSaveDone, title), nil
}

func (h *SessionHandler) handleList() (string, error) {
	entries, err := h.agent.Store().ListNamedSessions()
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigListFail), err)
	}

	if len(entries) == 0 {
		return i18n.T(i18n.KeySessionListEmpty), nil
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySessionListTitle) + "\n")

	currentID := h.agent.CurrentSessionID()
	for i, entry := range entries {
		// Dynamically compute message count
		var msgs []llm.Message
		msgCount := 0
		if err2 := json.Unmarshal(entry.Messages, &msgs); err2 == nil {
			msgCount = len(msgs)
		}
		title := entry.Title
		if title == "" {
			title = "(unnamed)"
		}
		keywords := entry.Keywords
		if keywords == "" {
			keywords = "-"
		}
		marker := " "
		if entry.ID == currentID {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  %s%3d  %-30s %-24s %5d  %s\n",
			marker, i+1, title, keywords, msgCount,
			entry.CreatedAt.Format("2006-01-02 15:04")))
	}

	sb.WriteString(i18n.T(i18n.KeySessionMigSwHint))
	sb.WriteString(i18n.T(i18n.KeySessionMigDelHint))
	return sb.String(), nil
}

func (h *SessionHandler) handleSwitch(idStr string) (string, error) {
	entries, err := h.agent.Store().ListNamedSessions()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionSwitchFailed, err))
	}

	// Find session by number or ID
	var target *store.SessionEntry
	if n, err := strconv.Atoi(idStr); err == nil && n > 0 && n <= len(entries) {
		// Sort by CreatedAt for consistent numbering
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})
		target = &entries[n-1]
	} else {
		for i := range entries {
			if entries[i].ID == idStr {
				target = &entries[i]
				break
			}
		}
	}

	if target == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNotFound), idStr)
	}

	// Flush current session messages back to DB before switching
	if err := h.agent.FlushCurrentSession(); err != nil {
		// Non-fatal, just log
		log.Debug("FlushCurrentSession before switch: %v", err)
	}

	// Load messages from target session
	var messages []llm.Message
	if err := json.Unmarshal(target.Messages, &messages); err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionSwitchFailed, err))
	}

	// Build full history with fresh system prompt
	current := h.agent.Messages()
	systemPrompt := ""
	if len(current) > 0 && current[0].Role == "system" {
		systemPrompt = current[0].Content
	}
	newMsgs := []llm.Message{{Role: "system", Content: systemPrompt}}
	newMsgs = append(newMsgs, messages...)
	h.agent.SetHistory(newMsgs)

	// Update the current session ID to the target session
	h.agent.SetCurrentSessionID(target.ID)
	if err := h.agent.Store().SaveCurrentSessionID(target.ID); err != nil {
		log.Warn("SaveCurrentSessionID after switch: %v", err)
	}

	return i18n.TF(i18n.KeySessionSwitchDone, target.Title), nil
}

func (h *SessionHandler) handleDelete(idStr string) (string, error) {
	entries, err := h.agent.Store().ListNamedSessions()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionDeleteFailed, err))
	}

	// Find session by number or ID
	var targetID string
	if n, err := strconv.Atoi(idStr); err == nil && n > 0 && n <= len(entries) {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})
		targetID = entries[n-1].ID
	} else {
		for _, e := range entries {
			if e.ID == idStr {
				targetID = e.ID
				break
			}
		}
	}

	if targetID == "" {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNotFound), idStr)
	}

	// Confirm
	fmt.Print(i18n.TF(i18n.KeySessionDeleteConfirm, idStr))
	response := ""
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyCancelled))
	}

	if err := h.agent.Store().DeleteNamedSession(targetID); err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeySessionDeleteFailed, err))
	}

	return i18n.TF(i18n.KeySessionDeleteDone, targetID), nil
}

// popMessages removes up to n non-system messages from the tail of the history.
func (h *SessionHandler) popMessages(n int) (string, error) {
	a := h.agent
	aMsg := a.Messages()
	if len(aMsg) <= 1 {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNoDelSys))
	}

	var popIdx []int
	for i := len(aMsg) - 1; i > 0 && len(popIdx) < n; i-- {
		if aMsg[i].Role != "system" {
			popIdx = append(popIdx, i)
		}
	}

	if len(popIdx) == 0 {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNoDelMsg))
	}

	lastContent := aMsg[popIdx[0]].Content
	if lastContent == "" && len(aMsg[popIdx[0]].ContentParts) > 0 {
		lastContent = aMsg[popIdx[0]].CombineContentParts()
	}
	if lastContent == "" {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNoDelMsg))
	}

	cutIdx := popIdx[len(popIdx)-1]
	a.SetHistory(aMsg[:cutIdx])

	dropped := len(popIdx) - 1
	if dropped > 0 {
		fmt.Printf(i18n.T(i18n.KeySessionPopDropped)+"\n", len(popIdx), dropped)
	}

	return fmt.Sprintf("POP:%s", lastContent), nil
}

// popTo removes all messages after index n (keeps [0..n]).
func (h *SessionHandler) popTo(n int) (string, error) {
	a := h.agent
	aMsg := a.Messages()
	if len(aMsg) <= 1 {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigNoDelSys))
	}

	if n >= len(aMsg)-1 {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigLastNoDel), n)
	}
	if n < 0 {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigInvalidNo), n)
	}

	if aMsg[n].Role == "system" {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigCannotDelSys))
	}

	lastContent := aMsg[n].Content
	if lastContent == "" && len(aMsg[n].ContentParts) > 0 {
		lastContent = aMsg[n].CombineContentParts()
	}
	if lastContent == "" {
		return "", fmt.Errorf(i18n.T(i18n.KeySessionMigGetMsgFail), n)
	}

	a.SetHistory(aMsg[:n+1])

	dropped := len(aMsg) - (n + 1)
	if dropped > 0 {
		fmt.Printf(i18n.T(i18n.KeySessionPopDropped)+"\n", dropped, dropped)
	}

	return fmt.Sprintf("POP:%s", lastContent), nil
}

// showSession displays information about the current conversation session.
func (h *SessionHandler) showSession() (string, error) {
	messages := h.agent.Messages()
	total := len(messages)

	var sb strings.Builder
	sb.WriteString("📋 " + i18n.T(i18n.KeySessionTitle) + "\n")
	sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionTotalMessages), total))

	if total > 0 {
		systemCount := 0
		userCount := 0
		assistantCount := 0
		toolCount := 0
		for _, msg := range messages {
			switch msg.Role {
			case "system":
				systemCount++
			case "user":
				userCount++
			case "assistant":
				assistantCount++
			case "tool":
				toolCount++
			}
		}
		sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionRoleSystem), systemCount))
		sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionRoleUser), userCount))
		sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionRoleAssistant), assistantCount))
		sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionRoleTool), toolCount))

		contextLimit := -1
		if h.cfg != nil {
			contextLimit = h.cfg.LLM.ContextLimit
		}
		limitStr := i18n.T(i18n.KeyUnlimited)
		if contextLimit == 0 {
			limitStr = i18n.T(i18n.KeySessionNoHistory)
		} else if contextLimit > 0 {
			limitStr = fmt.Sprintf("%d", contextLimit)
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionContextLimit), limitStr))

		if h.cfg != nil {
			activeModel := config.GetActiveModelFromConfig(h.cfg)
			modelName := "(not set)"
			providerName := "(not set)"
			if activeModel != nil {
				modelName = activeModel.Model
				providerName = activeModel.Provider
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionModel), modelName))
			sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionProvider), providerName))
		}

		sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionAgentName), h.agent.Name()))
	}

	return sb.String(), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
