// Author: L.Shuang
// Created: 2026-04-28
// Last Modified: 2026-04-28
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

// Package memory provides conversation memory management for co-shell.
// It stores and retrieves conversation messages with metadata (name, content, datetime),
// supporting slice-based history retrieval and keyword-based search.
package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/store"
)

// MessageEntry represents a single conversation message stored in memory.
type MessageEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`       // speaker name: user/agent name or alias
	Content   string    `json:"content"`    // message content
	Datetime  time.Time `json:"datetime"`   // message timestamp
	CreatedAt time.Time `json:"created_at"` // record creation time
}

// Manager handles conversation memory operations.
type Manager struct {
	store *store.Store
}

// NewManager creates a new memory Manager.
func NewManager(s *store.Store) *Manager {
	return &Manager{store: s}
}

// AddMessage adds a new conversation message to memory.
// This is called automatically by the program when user input or agent response occurs.
// It is NOT exposed to the LLM or internal commands.
func (m *Manager) AddMessage(name, content string, datetime time.Time) error {
	entry := MessageEntry{
		ID:        fmt.Sprintf("%020d", time.Now().UnixNano()),
		Name:      name,
		Content:   content,
		Datetime:  datetime,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cannot marshal message entry: %w", err)
	}

	return m.store.SaveConversationMessage(entry.ID, data)
}

// loadAllEntries loads all message entries from the store in chronological order.
func (m *Manager) loadAllEntries() ([]MessageEntry, error) {
	rawEntries, err := m.store.ListConversationMessages()
	if err != nil {
		return nil, fmt.Errorf("cannot list conversation messages: %w", err)
	}

	var entries []MessageEntry
	for _, raw := range rawEntries {
		var entry MessageEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue // skip corrupted entries
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// GetHistorySlice retrieves a slice of conversation history in chronological order.
// Parameters:
//   - lastfrom: starting position from the end (inclusive). 1 = most recent message.
//   - lastto: ending position from the end (inclusive). 1 = most recent message.
//
// Example: lastfrom=5, lastto=1 returns the 5 most recent messages in chronological order.
// Returns messages sorted by time ascending (oldest first within the slice).
func (m *Manager) GetHistorySlice(lastfrom, lastto int) ([]MessageEntry, error) {
	if lastfrom < 1 || lastto < 1 {
		return nil, fmt.Errorf("lastfrom and lastto must be >= 1")
	}
	if lastfrom < lastto {
		return nil, fmt.Errorf("lastfrom (%d) must be >= lastto (%d)", lastfrom, lastto)
	}

	allEntries, err := m.loadAllEntries()
	if err != nil {
		return nil, err
	}

	total := len(allEntries)
	if total == 0 {
		return []MessageEntry{}, nil
	}

	// Calculate slice boundaries (from end)
	// allEntries is in chronological order (oldest first)
	// We need to get from the end: lastfrom and lastto are 1-based from the end
	startIdx := total - lastfrom
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := total - lastto + 1
	if endIdx > total {
		endIdx = total
	}
	if startIdx >= endIdx {
		return []MessageEntry{}, nil
	}

	// Return in chronological order (already sorted)
	result := make([]MessageEntry, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		result = append(result, allEntries[i])
	}

	return result, nil
}

// SearchResult represents a single search result from memory search.
type SearchResult struct {
	Entry     MessageEntry `json:"entry"`
	MatchOn   string       `json:"match_on"`  // which field matched ("name" or "content")
	Relevance float64      `json:"relevance"` // simple relevance score (0.0-1.0)
}

// SearchParams defines parameters for memory search.
type SearchParams struct {
	Keywords      []string  // keywords to search for (AND logic: all must match)
	Since         time.Time // only return messages after this time (zero value = no filter)
	Name          string    // filter by speaker name (empty = no filter)
	MaxResults    int       // maximum number of results to return (0 = no limit)
	MaxContentLen int       // maximum character length for content in results (0 = no truncation)
}

// Search searches conversation memory for messages matching the given criteria.
// Returns results sorted by datetime descending (newest first).
func (m *Manager) Search(params SearchParams) ([]SearchResult, error) {
	allEntries, err := m.loadAllEntries()
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	for _, entry := range allEntries {
		// Filter by time
		if !params.Since.IsZero() && entry.Datetime.Before(params.Since) {
			continue
		}

		// Filter by name
		if params.Name != "" && !strings.EqualFold(entry.Name, params.Name) {
			continue
		}

		// If no keywords, return all matching (time/name filter only)
		if len(params.Keywords) == 0 {
			results = append(results, SearchResult{
				Entry:     entry,
				MatchOn:   "all",
				Relevance: 1.0,
			})
			continue
		}

		// Check keywords (AND logic)
		allMatch := true
		matchedFields := make(map[string]bool)
		contentLower := strings.ToLower(entry.Content)
		nameLower := strings.ToLower(entry.Name)

		for _, kw := range params.Keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(contentLower, kwLower) {
				matchedFields["content"] = true
			} else if strings.Contains(nameLower, kwLower) {
				matchedFields["name"] = true
			} else {
				allMatch = false
				break
			}
		}

		if allMatch && len(matchedFields) > 0 {
			matchOn := "content"
			if matchedFields["name"] && !matchedFields["content"] {
				matchOn = "name"
			} else if matchedFields["name"] && matchedFields["content"] {
				matchOn = "name,content"
			}

			// Simple relevance: more keywords = higher relevance
			relevance := float64(len(matchedFields)) / float64(len(params.Keywords))
			if relevance > 1.0 {
				relevance = 1.0
			}

			results = append(results, SearchResult{
				Entry:     entry,
				MatchOn:   matchOn,
				Relevance: relevance,
			})
		}
	}

	// Sort by datetime descending (newest first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Entry.Datetime.After(results[i].Entry.Datetime) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results if MaxResults is set
	if params.MaxResults > 0 && len(results) > params.MaxResults {
		results = results[:params.MaxResults]
	}

	return results, nil
}

// FormatHistorySlice formats a slice of message entries as a human-readable string.
func FormatHistorySlice(entries []MessageEntry) string {
	if len(entries) == 0 {
		return "（无历史对话记录）"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 历史对话记录（共 %d 条）:\n\n", len(entries)))
	for i, entry := range entries {
		timeStr := entry.Datetime.Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("[%d] %s | %s:\n", i+1, timeStr, entry.Name))
		sb.WriteString(fmt.Sprintf("    %s\n", entry.Content))
	}
	return sb.String()
}

// FormatSearchResults formats search results as a human-readable string.
// If maxContentLen > 0, content longer than this will be truncated with "...".
func FormatSearchResults(results []SearchResult, maxContentLen int) string {
	if len(results) == 0 {
		return "（未找到匹配的记忆）"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 记忆搜索结果（共 %d 条）:\n\n", len(results)))
	for i, r := range results {
		timeStr := r.Entry.Datetime.Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("[%d] %s | %s (匹配: %s):\n", i+1, timeStr, r.Entry.Name, r.MatchOn))
		content := r.Entry.Content
		if maxContentLen > 0 && len(content) > maxContentLen {
			content = content[:maxContentLen] + "..."
		}
		sb.WriteString(fmt.Sprintf("    %s\n", content))
	}
	return sb.String()
}
