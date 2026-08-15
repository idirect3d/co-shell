// Author: L.Shuang
// Created: 2026-08-15
// Last Modified: 2026-08-15
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
	"testing"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// newAutoLoopAgent builds an Agent for loop_intervention="auto" tests:
// judge disabled, retry-count limit effectively off (100), one plain user
// message with the given retried count.
func newAutoLoopAgent(retry int, threshold int) *Agent {
	a := newRetryLimitAgent(retry, 100, buildRetryUser("指令", retry))
	a.cfg.LLM.LoopIntervention = "auto"
	a.cfg.LLM.LoopJudgeEnabled = false
	a.cfg.LLM.LoopAutoReorganizeThreshold = threshold
	return a
}

// TestLoopAutoThreshold_Fallback verifies that an unset/invalid threshold
// falls back to the default of 5.
func TestLoopAutoThreshold_Fallback(t *testing.T) {
	a := newAutoLoopAgent(0, 0)
	if got := a.loopAutoThreshold(); got != 5 {
		t.Errorf("threshold 0 should fall back to 5, got %d", got)
	}
	a.cfg.LLM.LoopAutoReorganizeThreshold = -3
	if got := a.loopAutoThreshold(); got != 5 {
		t.Errorf("negative threshold should fall back to 5, got %d", got)
	}
	a.cfg.LLM.LoopAutoReorganizeThreshold = 7
	if got := a.loopAutoThreshold(); got != 7 {
		t.Errorf("threshold 7 should be honored, got %d", got)
	}
}

// TestAutoEscalateToReorganize verifies the escalation boundary: the next
// intervention escalates exactly when chain retried_count+1 reaches the
// threshold.
func TestAutoEscalateToReorganize(t *testing.T) {
	// count=3, threshold=5: next count would be 4 < 5 → no escalation.
	a := newAutoLoopAgent(3, 5)
	if a.autoEscalateToReorganize() {
		t.Error("count=3 with threshold=5 should NOT escalate")
	}

	// count=4, threshold=5: next count would be 5 >= 5 → escalate.
	a = newAutoLoopAgent(4, 5)
	if !a.autoEscalateToReorganize() {
		t.Error("count=4 with threshold=5 should escalate")
	}

	// count=9, threshold=5: stays escalated until the chain is collapsed.
	a = newAutoLoopAgent(9, 5)
	if !a.autoEscalateToReorganize() {
		t.Error("count=9 with threshold=5 should stay escalated")
	}
}

// TestAutoIntervention_BelowThreshold verifies that auto mode behaves like
// prompt mode (judge suggestion / generic feedback) before the threshold.
func TestAutoIntervention_BelowThreshold(t *testing.T) {
	a := newAutoLoopAgent(0, 5)
	a.SetIO(&retryLimitTestIO{})

	err := a.applyLoopIntervention(&LoopEvent{
		Type:       LoopEventToolCallRepeat,
		Detector:   "tool call loop detector",
		Reason:     "tool called twice consecutively",
		Suggestion: "换个思路",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// A new feedback chain message is appended with the suggestion text.
	last := a.messages[len(a.messages)-1]
	if got := last.ContentParts[0].Text; got != "换个思路" {
		t.Errorf("below threshold, feedback should be the suggestion, got %q", got)
	}
	if got := retryEnv(msgEnv(&last)); got != 1 {
		t.Errorf("feedback chain retried count should be 1, got %d", got)
	}
	if last.ContentParts[0].Text == i18n.T(i18n.KeyLoopAutoReorganize) {
		t.Error("below threshold, feedback must not be the reorganize directive")
	}
}

// TestAutoIntervention_EscalatesAtThreshold verifies that once the chain's
// retried count reaches the threshold, the feedback message is replaced by
// the mandatory reorganize_context directive.
func TestAutoIntervention_EscalatesAtThreshold(t *testing.T) {
	a := newAutoLoopAgent(0, 3)
	a.SetIO(&retryLimitTestIO{})

	event := &LoopEvent{
		Type:       LoopEventToolCallRepeat,
		Detector:   "tool call loop detector",
		Reason:     "tool called twice consecutively",
		Suggestion: "换个思路",
	}

	// Interventions 1 and 2: prompt-style feedback, chain count 1 and 2.
	for i := 1; i <= 2; i++ {
		if err := a.applyLoopIntervention(event); err != nil {
			t.Fatalf("intervention %d: expected nil error, got %v", i, err)
		}
		last := a.messages[len(a.messages)-1]
		if got := last.ContentParts[0].Text; got != "换个思路" {
			t.Fatalf("intervention %d: feedback should be the suggestion, got %q", i, got)
		}
		if got := retryEnv(msgEnv(&last)); got != i {
			t.Fatalf("intervention %d: chain retried count should be %d, got %d", i, i, got)
		}
	}

	// Intervention 3: count 2+1 reaches threshold 3 → escalate to directive.
	if err := a.applyLoopIntervention(event); err != nil {
		t.Fatalf("intervention 3: expected nil error, got %v", err)
	}
	last := a.messages[len(a.messages)-1]
	if want := i18n.T(i18n.KeyLoopAutoReorganize); last.ContentParts[0].Text != want {
		t.Errorf("intervention 3: feedback should be the reorganize directive, got %q", last.ContentParts[0].Text)
	}
	if got := retryEnv(msgEnv(&last)); got != 3 {
		t.Errorf("intervention 3: chain retried count should be 3, got %d", got)
	}

	// The chain is updated in place — no extra feedback messages appended.
	feedbackMsgs := 0
	for _, m := range a.messages {
		if m.Role == "user" && isLoopFeedbackMessage(&m) {
			feedbackMsgs++
		}
	}
	if feedbackMsgs != 1 {
		t.Errorf("expected exactly 1 feedback message (updated in place), got %d", feedbackMsgs)
	}
}

// TestAutoIntervention_NoMessages verifies the escalation helper does not
// panic and reports no escalation when there is no user/tool message.
func TestAutoIntervention_NoMessages(t *testing.T) {
	a := &Agent{cfg: newAutoLoopAgent(0, 5).cfg}
	a.messages = []llm.Message{{Role: "system", Content: "system"}}
	if a.autoEscalateToReorganize() {
		t.Error("no user/tool message: should not escalate")
	}
}
