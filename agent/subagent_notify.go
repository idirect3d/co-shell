// Author: L.Shuang
// Created: 2026-09-27
//
// MIT License
//
// Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"strconv"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/subagent"
)

// SubAgentCompletionStarter is implemented by UserIO implementations that can
// start a fresh agent turn themselves (the Web UI). Returning true means the
// notice was delivered as a formal user message that starts a new turn.
type SubAgentCompletionStarter interface {
	StartSubAgentCompletionTurn(sessionID, notice string) bool
}

// onSubAgentCompleted is the sub-agent manager's completion notifier
// (FEATURE-566): every sub-agent that reaches a natural terminal state — a
// success or a failure — is reported to the agent that launched it, whether that
// agent is mid-turn or idle. A notice for another session is dropped (the turn
// would otherwise run with a foreign context), and a killed process is never
// reported: the caller asked for the termination and got the result from the
// kill tool call itself.
func (a *Agent) onSubAgentCompleted(p subagent.Proc) {
	if a.subAgentMgr == nil {
		return
	}
	if p.SessionID == "" || p.SessionID != a.CurrentSessionID() {
		log.Info("subagent: completion notice dropped (session=%q current=%q id=%s)",
			p.SessionID, a.CurrentSessionID(), p.ID)
		return
	}
	if p.Status != subagent.StatusCompleted && p.Status != subagent.StatusFailed {
		return
	}

	notice := a.subAgentDoneNotice(p)

	if a.IsBusy() {
		// Mid-turn: the running iteration picks the notice up as a dynamic
		// event before its next LLM call, exactly like a board message.
		a.AddDynamicEvent(DynamicSubAgentDone, notice)
		return
	}
	if starter, ok := a.io.(SubAgentCompletionStarter); ok && starter.StartSubAgentCompletionTurn(p.SessionID, notice) {
		return
	}
	// No surface able to start a turn (terminal REPL / single command): print
	// the notice and buffer it, so the next turn still sees it.
	if a.io != nil {
		a.io.Printf("%s\n", notice)
	}
	a.AddDynamicEvent(DynamicSubAgentDone, notice)
}

// subAgentDoneNotice renders the notice for one finished sub-agent. It carries
// the status facts and the pointer to the status tool only — the result body and
// the logs stay in the manager, so the notice stays small (FEATURE-566).
func (a *Agent) subAgentDoneNotice(p subagent.Proc) string {
	elapsed := p.EndedAt.Sub(p.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	elapsedStr := elapsed.Round(time.Second).String()

	status := "completed"
	key := i18n.KeySubAgentNoticeCompleted
	if p.Status == subagent.StatusFailed {
		status = "failed"
		key = i18n.KeySubAgentNoticeFailed
	}
	text := i18n.TF(key, p.ID, p.Run, p.ExitCode, elapsedStr)

	return fmt.Sprintf("<sub_agent_done id=%q run=%q status=%q exit_code=%q elapsed=%q>%s</sub_agent_done>",
		p.ID, strconv.Itoa(p.Run), status, strconv.Itoa(p.ExitCode), elapsedStr, text)
}
