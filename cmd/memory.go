// Author: L.Shuang
// Created: 2026-04-26
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

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
)

// formatMemoryValue formats a memory value for display.
// If the value is valid JSON, it formats it as indented fields.
// String values are unescaped so that special characters (e.g., \n, \t, \")
// are displayed in their raw form rather than escaped.
// Otherwise, it returns the raw value.
func formatMemoryValue(value string) string {
	var parsed interface{}
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return value
	}
	formatted, err := json.MarshalIndent(parsed, "    ", "  ")
	if err != nil {
		return value
	}
	// Unescape string values in the formatted JSON so that special characters
	// (like \n, \t, \") are displayed in their raw form.
	result := unescapeJSONString(string(formatted))
	return result
}

// unescapeJSONString unescapes JSON string escape sequences in the formatted output.
// It handles common escape sequences: \n, \t, \", \\, \r.
func unescapeJSONString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				sb.WriteByte('\n')
				i += 2
				continue
			case 't':
				sb.WriteByte('\t')
				i += 2
				continue
			case 'r':
				sb.WriteByte('\r')
				i += 2
				continue
			case '\\':
				sb.WriteByte('\\')
				i += 2
				continue
			case '"':
				sb.WriteByte('"')
				i += 2
				continue
			default:
				sb.WriteByte(s[i])
				i++
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}

// MemoryHandler handles the .memory built-in command.
type MemoryHandler struct {
	store *store.Store
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(s *store.Store) *MemoryHandler {
	return &MemoryHandler{store: s}
}

// Handle processes .memory commands.
func (h *MemoryHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.Help(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "save":
		return h.saveMemory(args[1:])
	case "get":
		return h.getMemory(args[1:])
	case "search":
		return h.searchMemory(args[1:])
	case "delete":
		return h.deleteMemory(args[1:])
	case "clear":
		return h.clearMemory()
	case "list":
		return h.listMemory()
	default:
		return "", fmt.Errorf("unknown memory subcommand: %s", subcommand)
	}
}

// saveMemory saves a key-value pair to memory.
func (h *MemoryHandler) saveMemory(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: .memory save <key> <value>")
	}
	key := args[0]
	value := strings.Join(args[1:], " ")
	if err := h.store.SaveMemory(key, value); err != nil {
		return "", fmt.Errorf("cannot save memory: %w", err)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyMemorySaved), key, value), nil
}

// getMemory retrieves a value from memory by key.
func (h *MemoryHandler) getMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory get <key>")
	}
	key := args[0]
	value, found, err := h.store.GetMemory(key)
	if err != nil {
		return "", fmt.Errorf("cannot get memory: %w", err)
	}
	if !found {
		return fmt.Sprintf("Memory key %q not found", key), nil
	}
	return fmt.Sprintf(i18n.T(i18n.KeyMemoryGet), key, value), nil
}

// searchMemory searches memory for keys containing the given prefix.
func (h *MemoryHandler) searchMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory search <prefix>")
	}
	prefix := args[0]
	entries, err := h.store.SearchMemory(prefix)
	if err != nil {
		return "", fmt.Errorf("cannot search memory: %w", err)
	}
	if len(entries) == 0 {
		return i18n.T(i18n.KeyMemoryEmpty), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d memory entries:\n", len(entries)))
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("  %s:\n", entry.Key))
		sb.WriteString(fmt.Sprintf("    %s\n", formatMemoryValue(entry.Value)))
	}
	return sb.String(), nil
}

// deleteMemory deletes a memory entry by key.
func (h *MemoryHandler) deleteMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory delete <key>")
	}
	key := args[0]
	// Use SaveMemory with empty value to delete
	if err := h.store.SaveMemory(key, ""); err != nil {
		return "", fmt.Errorf("cannot delete memory: %w", err)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyMemoryDeleted), key), nil
}

// clearMemory clears all memory entries.
func (h *MemoryHandler) clearMemory() (string, error) {
	if err := h.store.ClearConversationMessages(); err != nil {
		return "", fmt.Errorf("cannot clear memory: %w", err)
	}
	return i18n.T(i18n.KeyMemoryCleared), nil
}

// listMemory lists all memory entries.
func (h *MemoryHandler) listMemory() (string, error) {
	entries, err := h.store.SearchMemory("")
	if err != nil {
		return "", fmt.Errorf("cannot list memory: %w", err)
	}
	if len(entries) == 0 {
		return i18n.T(i18n.KeyMemoryEmpty), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Memory entries (%d):\n", len(entries)))
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("  %s:\n", entry.Key))
		sb.WriteString(fmt.Sprintf("    %s\n", formatMemoryValue(entry.Value)))
	}
	return sb.String(), nil
}

// Help returns the help text for the memory command.
func (h *MemoryHandler) Help() string {
	return `Usage: .memory <subcommand> [args]

Subcommands:
  save <key> <value>   - Save a value to memory
  get <key>            - Get a value from memory
  search <prefix>      - Search memory by key prefix
  delete <key>         - Delete a memory entry
  clear                - Clear all memory entries
  list                 - List all memory entries`
}
