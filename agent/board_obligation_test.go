// Author: L.Shuang
// Created: 2026-09-28
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

package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
)

// newMustReplyTestAgent builds an agent with the board on and the reply-duty
// gate set as requested (FEATURE-567).
func newMustReplyTestAgent(t *testing.T, mustReply bool) *Agent {
	t.Helper()
	cfg := &config.Config{}
	cfg.BoardEnabled = true
	cfg.BoardMustReply = mustReply
	return newTestAgentWithCfg(cfg)
}

// TestBoardObligationLifecycle covers registration, rendering and clearing of a
// reply duty (FEATURE-567 UC-0001/UC-0002).
func TestBoardObligationLifecycle(t *testing.T) {
	a := newMustReplyTestAgent(t, true)
	now := time.Now().Unix()

	// no_reply messages never create a duty.
	a.AddBoardObligation(BoardMessage{ID: "m-fyi", From: "agent-a", NoReply: true, Timeout: 60, TS: now})
	if got := a.PendingBoardObligations(); len(got) != 0 {
		t.Fatalf("no_reply message created a duty: %+v", got)
	}
	if block := a.BoardObligationBlock(); block != "" {
		t.Fatalf("block should be omitted while nothing is pending, got %q", block)
	}

	a.AddBoardObligation(BoardMessage{ID: "m-7", From: "agent-a", Timeout: 60, TS: now})
	pending := a.PendingBoardObligations()
	if len(pending) != 1 || pending[0].MessageID != "m-7" || pending[0].From != "agent-a" {
		t.Fatalf("pending = %+v, want one duty for m-7", pending)
	}

	block := a.BoardObligationBlock()
	if !strings.Contains(block, `<board_replies pending="1">`) ||
		!strings.Contains(block, `message_id="m-7"`) ||
		!strings.Contains(block, `from="agent-a"`) ||
		!strings.Contains(block, "remaining=") {
		t.Fatalf("unexpected <board_replies> block: %s", block)
	}
	if strings.Contains(block, "expired") {
		t.Fatalf("fresh duty should have a positive remaining window: %s", block)
	}

	// A repeated delivery refreshes the duty instead of duplicating it.
	a.AddBoardObligation(BoardMessage{ID: "m-7", From: "agent-a", Timeout: 30, TS: now})
	if got := a.PendingBoardObligations(); len(got) != 1 {
		t.Fatalf("re-delivery duplicated the duty: %+v", got)
	}

	if !a.ResolveBoardObligation("m-7") {
		t.Fatal("ResolveBoardObligation should report the duty as pending")
	}
	if a.ResolveBoardObligation("m-7") {
		t.Fatal("second resolve must report no pending duty")
	}
	if block := a.BoardObligationBlock(); block != "" {
		t.Fatalf("block should be omitted after answering, got %q", block)
	}
}

// TestBoardObligationBlockNeedsBoard: the duty list is part of the board
// feature, so it disappears when the board is switched off.
func TestBoardObligationBlockNeedsBoard(t *testing.T) {
	cfg := &config.Config{}
	cfg.BoardEnabled = false
	cfg.BoardMustReply = true
	a := newTestAgentWithCfg(cfg)
	a.AddBoardObligation(BoardMessage{ID: "m-7", From: "agent-a", Timeout: 60, TS: time.Now().Unix()})
	if block := a.BoardObligationBlock(); block != "" {
		t.Fatalf("board switch is off, block should be empty, got %q", block)
	}
}

// TestBoardCompletionGateRefusesTraces covers the hard gate: the refusal names
// the message, its sender and the remaining window, and the same duty is
// released after BoardObligationMaxBlocked refusals (FEATURE-567 UC-0003).
func TestBoardCompletionGateRefusesTraces(t *testing.T) {
	a := newMustReplyTestAgent(t, true)
	a.AddBoardObligation(BoardMessage{ID: "m-7", From: "agent-a", Timeout: 300, TS: time.Now().Unix()})

	for i := 1; i <= BoardObligationMaxBlocked; i++ {
		msg, blocked := a.BoardCompletionGate()
		if !blocked {
			t.Fatalf("refusal %d: gate should block attempt_completion", i)
		}
		for _, want := range []string{"m-7", "agent-a", "remaining=", "attempt_completion", "已拒绝"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("refusal %d missing %q: %s", i, want, msg)
			}
		}
	}
	// The (MaxBlocked+1)-th call is released so a forgotten message can never
	// deadlock the agent.
	if _, blocked := a.BoardCompletionGate(); blocked {
		t.Fatalf("gate should release after %d refusals", BoardObligationMaxBlocked)
	}
	// The counter is cleared, so the next cycle refuses again.
	if _, blocked := a.BoardCompletionGate(); !blocked {
		t.Fatal("gate should refuse again after the counter was reset")
	}
}

// TestBoardCompletionGateDisabled: with board-must-reply off the duty is still
// tracked (soft reminder) but never blocks completion.
func TestBoardCompletionGateDisabled(t *testing.T) {
	a := newMustReplyTestAgent(t, false)
	a.AddBoardObligation(BoardMessage{ID: "m-7", From: "agent-a", Timeout: 60, TS: time.Now().Unix()})
	if msg, blocked := a.BoardCompletionGate(); blocked {
		t.Fatalf("gate must not block while board-must-reply is off: %s", msg)
	}
	if block := a.BoardObligationBlock(); block == "" {
		t.Fatal("the soft reminder should still be rendered")
	}
}

// TestBoardTimeoutXMLRedelivered checks the hub timeout event rendering with and
// without the re-delivery counter (FEATURE-567 UC-0004).
func TestBoardTimeoutXMLRedelivered(t *testing.T) {
	plain := BoardTimeoutXML("m-7", "agent-b", 1700000000, 0)
	if strings.Contains(plain, "redelivered") {
		t.Fatalf("no re-delivery yet, attribute should be omitted: %s", plain)
	}
	twice := BoardTimeoutXML("m-7", "agent-b", 1700000000, 1)
	if !strings.Contains(twice, `redelivered="1"`) {
		t.Fatalf("re-delivery counter missing: %s", twice)
	}
}
