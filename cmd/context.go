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

// contextSeparator is a long horizontal line placed between consecutive
// messages so the reader can visually separate each message block.
const contextSeparator = "──────────────────────────────────────────────"

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
//	.context full               - show full context including <environment_details>
//	.context reset              - reset context (clear conversation history)
//	.context set <key> <value>  - set a context variable
func (h *ContextHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showContext(false)
	}

	subcommand := args[0]
	switch subcommand {
	case "show":
		return h.showContext(false)

	case "full":
		return h.showContext(true)

	case "reset":
		return h.resetContext()

	case "set":
		return h.setContext(args[1:])

	default:
		return "", fmt.Errorf("unknown subcommand: %s\n\nAvailable commands:\n  show              - Show current context\n  full              - Show current context with full <environment_details>\n  reset             - Reset context\n  set <key> <value> - Set a context variable", subcommand)
	}
}

func (h *ContextHandler) showContext(full bool) (string, error) {
	// Rebuild the system prompt first so external file edits (PRINCIPLES.md,
	// .rules/, etc.) are reflected in the displayed context (message 0).
	h.agent.RebuildSystemPrompt()
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
		// Keep whitespace/control characters as-is (no newline flattening) so
		// multi-line content, indentation, tabs and blank lines are preserved
		// for the user. <environment_details> envelopes are hidden unless full.
		cleanContent := stripEnvBlocks(content)

		envText := agent.MessageEnv(&msg)

		marker := " "
		if i == pointerIdx {
			marker = "*"
		}

		// Header: [marker][index] [role] time ♾️retried_count
		headTime := extractTimeTag(envText)
		retryN := agent.RetriedCountOf(&msg)
		var retrySuffix string
		if retryN > 0 {
			retrySuffix = fmt.Sprintf(" ♾️%d", retryN)
		}
		sb.WriteString(fmt.Sprintf("  %s%3d  [%-9s] %s%s\n", marker, i, msg.Role, headTime, retrySuffix))

		// Message content block indented to align with the role label '[' above
		// (2 spaces + marker + 3-digit index + 2 spaces = column 8), control
		// characters preserved.
		if cleanContent != "" {
			sb.WriteString(indentText(cleanContent, "        "))
		}

		// tool_calls sub-block shares the same message index (no new index),
		// so displayed sequence numbers always match the real message array
		// index used by <message_no>. The label is left-aligned with the
		// role labels ([user     ], [assistant], ...) above.
		if len(msg.ToolCalls) > 0 {
			sb.WriteString("        [tool_calls]\n")
			for _, tc := range msg.ToolCalls {
				sb.WriteString(fmt.Sprintf("        - %s\n", tc.Name))
				argsText := formatToolArguments(tc.Arguments)
				if argsText != "" {
					sb.WriteString(indentText(argsText, "            "))
				}
			}
		}

		// Full mode: append the complete <environment_details> block, aligned
		// with the role label so all content shares the same left margin.
		if full && envText != "" {
			sb.WriteString(indentText(envText, "        "))
		}

		sb.WriteString("\n")
		sb.WriteString(contextSeparator)
		sb.WriteString("\n")
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

// stripEnvBlocks removes all <environment_details>...</environment_details>
// blocks from a message content so the default :context output stays readable.
// The blocks are restored by :context full (shown separately below the header).
func stripEnvBlocks(s string) string {
	const openTag = "<environment_details>"
	const closeTag = "</environment_details>"
	for {
		start := strings.Index(s, openTag)
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], closeTag)
		if end < 0 {
			// Unterminated block: drop from start to end of string.
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+len(closeTag):]
	}
	return strings.TrimSpace(s)
}

// extractTimeTag returns the content of the <time>...</time> tag in the env
// text, or "" when absent.
func extractTimeTag(envText string) string {
	const openTag = "<time>"
	const closeTag = "</time>"
	start := strings.Index(envText, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(envText[start:], closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(envText[start : start+end])
}

// indentText prefixes every line of s with indent. Trailing newline is kept.
func indentText(s, indent string) string {
	if s == "" {
		return ""
	}
	// Normalize \r\n to \n so each line is indented exactly once.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(indent)
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatToolArguments renders a ToolCall's Arguments JSON string in an
// indented, human-readable form. Parses when possible; falls back to the raw
// string when the JSON is invalid, and returns "" when empty.
func formatToolArguments(args string) string {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return ""
	}
	var pretty interface{}
	if err := json.Unmarshal([]byte(trimmed), &pretty); err == nil {
		if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			return string(b)
		}
	}
	return trimmed
}

// Help returns the help text for the context command.
func (h *ContextHandler) Help() string {
	return `Context Management (.context)

Usage:
  .context                  Show current conversation context
  .context show             Show detailed context
  .context full             Show full context including <environment_details>
  .context reset            Reset context (clear conversation history)
  .context set <k> <v>      Set a context variable

Examples:
  .context show
  .context full
  .context set mode expert
  .context reset`
}
