// Author: L.Shuang
// Created: 2026-07-11
// Last Modified: 2026-07-11
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
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
)

// ResetHandler handles the :reset built-in command.
type ResetHandler struct {
	agent *agent.Agent
}

// NewResetHandler creates a new ResetHandler.
func NewResetHandler(ag *agent.Agent) *ResetHandler {
	return &ResetHandler{agent: ag}
}

// Handle processes the :reset command.
// It clears the current session messages without saving.
func (h *ResetHandler) Handle(args []string) (string, error) {
	// Require confirmation
	fmt.Print(i18n.T(i18n.KeySessionResetConfirm))
	response := ""
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyCancelled))
	}

	h.agent.Reset()

	// Also clear the session entry in BoltDB
	if sessionID := h.agent.CurrentSessionID(); sessionID != "" {
		if existing, found, _ := h.agent.Store().LoadNamedSession(sessionID); found && existing != nil {
			existing.Messages = []byte("[]")
			existing.MessageCount = 0
			existing.UpdatedAt = time.Now()
			if err := h.agent.Store().UpdateNamedSession(sessionID, existing); err != nil {
				return "", fmt.Errorf("重置会话失败: %v", err)
			}
		}
	}

	return i18n.T(i18n.KeySessionResetDone), nil
}
