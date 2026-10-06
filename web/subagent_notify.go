// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package web

import (
	"github.com/idirect3d/co-shell/log"
)

// StartSubAgentCompletionTurn implements agent.SubAgentCompletionStarter: it
// delivers the completion notice of one finished sub-agent as a formal user
// message, which starts a fresh agent turn (FEATURE-566). It reports false when
// this IO is not bound to a session or the session refuses the notice, so the
// agent falls back to the dynamic-event path.
func (w *WebIO) StartSubAgentCompletionTurn(sessionID, notice string) bool {
	if w.startSubAgentTurn == nil {
		return false
	}
	return w.startSubAgentTurn(sessionID, notice)
}

// startSubAgentCompletionTurn queues a completion notice as this session's next
// user input. A notice for another session is refused: the turn would otherwise
// run in the current session with a foreign context. The wait is bounded so the
// sub-agent reaper that calls it can never be parked by a stalled browser.
func (s *WebSession) startSubAgentCompletionTurn(sessionID, notice string) bool {
	if sessionID == "" || sessionID != s.ag.CurrentSessionID() {
		log.Warn("session: sub-agent completion notice refused (session=%q current=%q)",
			sessionID, s.ag.CurrentSessionID())
		return false
	}
	// FEATURE-542: the explicit FIFO queue never blocks, so the notice can no
	// longer be dropped because the channel happened to be busy.
	s.inputQ.push(clientMessage{Type: "input", Text: notice})
	log.Info("session: sub-agent completion notice starts a new turn (session=%s)", sessionID)
	return true
}
