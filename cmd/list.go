// Author: L.Shuang
// Created: 2026-04-26
// Last Modified: 2026-04-26
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
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
)

// errUsage is a sentinel error for usage errors.
type errUsage string

func (e errUsage) Error() string { return string(e) }

// ListHandler handles the .history (also .list), .last, and .first built-in commands.
type ListHandler struct {
	store *store.Store
}

// NewListHandler creates a new ListHandler.
func NewListHandler(s *store.Store) *ListHandler {
	return &ListHandler{store: s}
}

// HandleHistory handles the .history command.
// Usage:
//
//	.history              — show subcommand usage
//	.history [start] [end] — show history range
//	.history last [N]      — show last N entries (default 10)
//	.history first [N]     — show first N entries (default 10)
func (h *ListHandler) HandleHistory(args []string) (string, error) {
	if len(args) == 0 {
		return i18n.T(i18n.KeyHistoryUsage), nil
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "last":
		return h.HandleLast(subArgs)
	case "first":
		return h.HandleFirst(subArgs)
	default:
		// Treat as range: .history [start] [end]
		return h.HandleList(args)
	}
}

// HandleList handles the .list command (compatible alias for .history).
// Usage: .list [start] [end]
// If no arguments, shows all history entries.
// If one argument, shows from that index to the end.
// If two arguments, shows the range [start, end].
func (h *ListHandler) HandleList(args []string) (string, error) {
	entries, err := h.store.ListHistory()
	if err != nil {
		return "", fmt.Errorf("failed to load history: %w", err)
	}

	if len(entries) == 0 {
		return i18n.T(i18n.KeyListEmpty), nil
	}

	total := len(entries)
	start, end := 1, total

	if len(args) >= 1 {
		s, err := strconv.Atoi(args[0])
		if err != nil || s < 1 || s > total {
			return "", errUsage(i18n.TF(i18n.KeyListInvalid, total))
		}

		start = s
	}
	if len(args) >= 2 {
		e, err := strconv.Atoi(args[1])
		if err != nil || e < 1 || e > total {
			return "", errUsage(i18n.TF(i18n.KeyListInvalid, total))
		}

		end = e
	}

	if start > end {
		start, end = end, start
	}

	return formatHistoryList(entries, start, end), nil
}

// HandleLast handles the .last command.
// Usage: .last [N] — shows the last N entries (default 10).
func (h *ListHandler) HandleLast(args []string) (string, error) {
	entries, err := h.store.ListHistory()
	if err != nil {
		return "", fmt.Errorf("failed to load history: %w", err)
	}

	if len(entries) == 0 {
		return i18n.T(i18n.KeyListEmpty), nil
	}

	n := 10
	if len(args) >= 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 1 {
			return "", errUsage(i18n.T(i18n.KeyLastUsage))
		}

		n = v
	}

	total := len(entries)
	if n > total {
		n = total
	}

	start := total - n + 1
	return formatHistoryList(entries, start, total), nil
}

// HandleFirst handles the .first command.
// Usage: .first [N] — shows the first N entries (default 10).
func (h *ListHandler) HandleFirst(args []string) (string, error) {
	entries, err := h.store.ListHistory()
	if err != nil {
		return "", fmt.Errorf("failed to load history: %w", err)
	}

	if len(entries) == 0 {
		return i18n.T(i18n.KeyListEmpty), nil
	}

	n := 10
	if len(args) >= 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 1 {
			return "", errUsage(i18n.T(i18n.KeyFirstUsage))
		}

		n = v
	}

	total := len(entries)
	if n > total {
		n = total
	}

	return formatHistoryList(entries, 1, n), nil
}

// formatHistoryList formats a list of history entries with line numbers.
func formatHistoryList(entries []store.HistoryEntryWithTime, start, end int) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyListTitle) + "\n")
	for i := start - 1; i < end; i++ {
		entry := entries[i]
		timeStr := entry.Timestamp.Format("01-02 15:04")
		sb.WriteString(fmt.Sprintf("  %4d  [%s] %s\n", i+1, timeStr, entry.Input))
	}
	sb.WriteString("\n" + i18n.T(i18n.KeyListReExecute))
	return sb.String()
}
