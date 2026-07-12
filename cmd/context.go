// Author: L.Shuang
// Created: 2026-04-25
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
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/store"
)

// ContextHandler handles the .context built-in command.
// Shows current conversation context (messages).
type ContextHandler struct {
	agent *agent.Agent
	store *store.DualStore
}

// NewContextHandler creates a new ContextHandler.
func NewContextHandler(ag *agent.Agent, s *store.DualStore) *ContextHandler {
	return &ContextHandler{agent: ag, store: s}
}

// Handle processes .context commands.
// Syntax:
//
//	.context                    - show current conversation context (messages)
//	.context show               - show detailed context
//	.context reset              - reset context (clear conversation history)
//	.context set <key> <value>  - set a context variable
func (h *ContextHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showContext()
	}

	subcommand := args[0]
	switch subcommand {
	case "show":
		return h.showContext()

	case "reset":
		return h.resetContext()

	case "set":
		return h.setContext(args[1:])

	default:
		return "", fmt.Errorf("unknown subcommand: %s\n\nAvailable commands:\n  show              - Show current context\n  reset             - Reset context\n  set <key> <value> - Set a context variable", subcommand)
	}
}

func (h *ContextHandler) showContext() (string, error) {
	messages := h.agent.Messages()
	if len(messages) == 0 {
		return "Context is empty.\n\nStart a conversation to build context.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 " + "当前上下文" + "\n")
	sb.WriteString(fmt.Sprintf("  总消息数: %d\n", len(messages)))

	pointerIdx := h.agent.MessagePointer()
	for i, msg := range messages {
		content := msg.Content
		if content == "" && len(msg.ContentParts) > 0 {
			content = msg.CombineContentParts()
		}
		content = strings.ReplaceAll(content, "\n", " ")
		marker := " "
		if i == pointerIdx {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  %s%3d  [%-9s] %s\n", marker, i, msg.Role, content))
	}

	return sb.String(), nil
}

func (h *ContextHandler) resetContext() (string, error) {
	h.agent.Reset()
	if err := h.store.ClearContext(); err != nil {
		return "", fmt.Errorf("failed to reset context: %w", err)
	}
	return "✅ Context reset. Conversation history cleared.", nil
}

func (h *ContextHandler) setContext(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: .context set <key> <value>")
	}

	key := args[0]
	value := strings.Join(args[1:], " ")

	data, found, err := h.store.GetContext("current")
	if err != nil {
		return "", fmt.Errorf("failed to read context: %w", err)
	}

	ctx := make(map[string]interface{})
	if found && len(data) > 0 {
		if err := json.Unmarshal(data, &ctx); err != nil {
			return "", fmt.Errorf("failed to parse context: %w", err)
		}
	}

	ctx[key] = value
	newData, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := h.store.SaveContext("current", newData); err != nil {
		return "", fmt.Errorf("failed to save context: %w", err)
	}

	return fmt.Sprintf("✅ Context set: %s = %s", key, value), nil
}

func truncateStringForContext(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// Help returns the help text for the context command.
func (h *ContextHandler) Help() string {
	return `Context Management (.context)

Usage:
  .context                  Show current conversation context
  .context show             Show detailed context
  .context reset            Reset context (clear conversation history)
  .context set <k> <v>      Set a context variable

Examples:
  .context show
  .context set mode expert
  .context reset`
}
