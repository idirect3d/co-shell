// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/store"
)

// MemoryHandler handles the .memory built-in command.
type MemoryHandler struct {
	store *store.Store
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(s *store.Store) *MemoryHandler {
	return &MemoryHandler{store: s}
}

// Handle processes .memory commands.
// Syntax:
//
//	.memory                    - list all memory entries
//	.memory save <k> <v>      - save a memory entry
//	.memory get <key>         - get a memory entry
//	.memory search <query>    - search memory entries
//	.memory delete <key>      - delete a memory entry
//	.memory clear             - clear all memory
func (h *MemoryHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.listMemory()
	}

	subcommand := args[0]
	switch subcommand {
	case "save":
		return h.saveMemory(args[1:])

	case "get":
		return h.getMemory(args[1:])

	case "search":
		return h.searchMemory(args[1:])

	case "delete", "del", "rm":
		return h.deleteMemory(args[1:])

	case "clear":
		return h.clearMemory()

	default:
		return "", fmt.Errorf("unknown subcommand: %s\n\nAvailable commands:\n  save <key> <value>  - Save a memory entry\n  get <key>           - Get a memory entry\n  search <query>      - Search memory entries\n  delete <key>        - Delete a memory entry\n  clear               - Clear all memory", subcommand)
	}
}

func (h *MemoryHandler) saveMemory(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: .memory save <key> <value>")
	}

	key := args[0]
	value := strings.Join(args[1:], " ")

	if err := h.store.SaveMemory(key, value); err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}

	return fmt.Sprintf("✅ Memory saved: %s = %s", key, value), nil
}

func (h *MemoryHandler) getMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory get <key>")
	}

	key := args[0]
	value, found, err := h.store.GetMemory(key)
	if err != nil {
		return "", fmt.Errorf("failed to get memory: %w", err)
	}

	if !found {
		return fmt.Sprintf("No memory found for key: %s", key), nil
	}

	return fmt.Sprintf("%s = %s", key, value), nil
}

func (h *MemoryHandler) searchMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory search <query>")
	}

	query := args[0]
	entries, err := h.store.SearchMemory(query)
	if err != nil {
		return "", fmt.Errorf("failed to search memory: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Sprintf("No memory entries found matching: %s", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Memory entries matching %q:\n", query))
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", entry.Key, entry.Value))
	}
	return sb.String(), nil
}

func (h *MemoryHandler) deleteMemory(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: .memory delete <key>")
	}

	key := args[0]
	if err := h.store.DeleteMemory(key); err != nil {
		return "", fmt.Errorf("failed to delete memory: %w", err)
	}

	return fmt.Sprintf("✅ Memory deleted: %s", key), nil
}

func (h *MemoryHandler) clearMemory() (string, error) {
	if err := h.store.ClearMemory(); err != nil {
		return "", fmt.Errorf("failed to clear memory: %w", err)
	}

	return "✅ All memory cleared", nil
}

func (h *MemoryHandler) listMemory() (string, error) {
	entries, err := h.store.ListMemory()
	if err != nil {
		return "", fmt.Errorf("failed to list memory: %w", err)
	}

	if len(entries) == 0 {
		return "No memory entries.\n\nSave one with: .memory save <key> <value>", nil
	}

	var sb strings.Builder
	sb.WriteString("Memory:\n")
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", entry.Key, entry.Value))
	}
	return sb.String(), nil
}

// Help returns the help text for the memory command.
func (h *MemoryHandler) Help() string {
	return `Memory Management (.memory)

Usage:
  .memory                    List all memory entries
  .memory save <key> <value> Save a memory entry
  .memory get <key>          Get a memory entry
  .memory search <query>     Search memory entries by key prefix
  .memory delete <key>       Delete a memory entry
  .memory clear              Clear all memory

Examples:
  .memory save language zh-CN
  .memory save preference "Always use verbose output"
  .memory search language
  .memory get preference`
}
