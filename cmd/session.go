// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
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
// It displays information about the current conversation session.
func (h *SessionHandler) Handle(args []string) (string, error) {
	return h.showSession()
}

func (h *SessionHandler) showSession() (string, error) {
	messages := h.agent.Messages()
	total := len(messages)

	var sb strings.Builder
	sb.WriteString("📋 " + i18n.T(i18n.KeySessionTitle) + "\n")
	sb.WriteString(fmt.Sprintf("  %s: %d\n", i18n.T(i18n.KeySessionTotalMessages), total))

	if total > 0 {
		// Count by role
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

		// Show context limit info
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

		// Show model info
		if h.cfg != nil {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionModel), h.cfg.LLM.Model))
			sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionProvider), h.cfg.LLM.Provider))
		}

		// Show agent name
		sb.WriteString(fmt.Sprintf("  %s: %s\n", i18n.T(i18n.KeySessionAgentName), h.agent.Name()))

		// Show last few messages as summary
		sb.WriteString("\n" + i18n.T(i18n.KeySessionRecentMessages) + "\n")
		start := 0
		if total > 10 {
			start = total - 10
		}
		for i := start; i < total; i++ {
			msg := messages[i]
			content := msg.Content
			// Truncate long content
			if len(content) > 80 {
				content = content[:77] + "..."
			}
			// Replace newlines with spaces for display
			content = strings.ReplaceAll(content, "\n", " ")
			sb.WriteString(fmt.Sprintf("  %4d  [%-9s] %s\n", i+1, msg.Role, content))
		}
	}

	return sb.String(), nil
}
