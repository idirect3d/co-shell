// Author: L.Shuang
// Created: 2026-08-04
// Last Modified: 2026-08-04
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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// retryLimitTestIO is a configurable UserIO mock for retry-limit tests.
type retryLimitTestIO struct {
	buf       bytes.Buffer
	readVal   string // value returned by ReadLine
	readCalls int    // number of ReadLine calls
}

func (b *retryLimitTestIO) Print(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
}

func (b *retryLimitTestIO) Printf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *retryLimitTestIO) Println(args ...interface{}) {
	for _, a := range args {
		b.buf.WriteString(fmt.Sprint(a))
	}
	b.buf.WriteString("\n")
}

func (b *retryLimitTestIO) ErrPrintf(format string, args ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, args...))
}

func (b *retryLimitTestIO) ReadLine() (string, error) {
	b.readCalls++
	return b.readVal, nil
}

func (b *retryLimitTestIO) ReadKey() (byte, error) { return 0, nil }
func (b *retryLimitTestIO) IsReading() bool        { return false }

// buildRetryUser creates a user message whose env carries the given retried count.
func buildRetryUser(instruction string, retry int) llm.Message {
	env := sampleEnv("")
	env = setRetriedCountInText(env, retry)
	return llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: instruction},
			{Type: llm.ContentPartText, Text: env},
		},
	}
}

// newRetryLimitAgent builds an Agent with the given config and message history.
func newRetryLimitAgent(retry int, maxSingle int, userMsg llm.Message) *Agent {
	a := &Agent{
		cfg: &config.Config{},
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			userMsg,
		},
	}
	a.cfg.LLM.ErrorMaxSingleCount = maxSingle
	return a
}

// TestRetryCountLimit_BelowThreshold verifies UC-0001:
// checkRetryCountLimit does not prompt when count < maxSingle.
func TestRetryCountLimit_BelowThreshold(t *testing.T) {
	a := newRetryLimitAgent(2, 3, buildRetryUser("指令", 2))
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	ok, err := a.checkRetryCountLimit()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true (below threshold), got false")
	}
	if io.readCalls != 0 {
		t.Fatalf("expected no ReadLine call, got %d", io.readCalls)
	}
	// retried count must stay unchanged
	if got := retryEnv(msgEnv(&a.messages[1])); got != 2 {
		t.Errorf("retried count should stay 2, got %d", got)
	}
}

// TestRetryCountLimit_EnterResets verifies UC-0002:
// at threshold, Enter resets retried count to 1 and continues.
func TestRetryCountLimit_EnterResets(t *testing.T) {
	a := newRetryLimitAgent(3, 3, buildRetryUser("指令", 3))
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	ok, err := a.checkRetryCountLimit()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true (Enter continues), got false")
	}
	if io.readCalls != 1 {
		t.Fatalf("expected 1 ReadLine call, got %d", io.readCalls)
	}
	// retried count reset to 1
	if got := retryEnv(msgEnv(&a.messages[1])); got != 1 {
		t.Errorf("retried count should reset to 1, got %d", got)
	}
	// user saw the warning
	if !strings.Contains(io.buf.String(), "提示") && !strings.Contains(io.buf.String(), "警告") {
		t.Errorf("expected warning text in output, got: %q", io.buf.String())
	}
}

// TestRetryCountLimit_Cancel verifies UC-0003:
// at threshold, C cancels and returns an error.
func TestRetryCountLimit_Cancel(t *testing.T) {
	a := newRetryLimitAgent(3, 3, buildRetryUser("指令", 3))
	io := &retryLimitTestIO{readVal: "C"}
	a.SetIO(io)

	ok, err := a.checkRetryCountLimit()
	if err == nil {
		t.Fatalf("expected cancel error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false (cancel), got true")
	}
	var cancelErr *retryCountCancelError
	if !errorsAs(err, &cancelErr) {
		t.Errorf("expected retryCountCancelError, got %T", err)
	}
	// retried count unchanged on cancel
	if got := retryEnv(msgEnv(&a.messages[1])); got != 3 {
		t.Errorf("retried count should stay 3 on cancel, got %d", got)
	}
}

// TestRetryCountLimit_IgnoreAll verifies UC-0004:
// at threshold, A sets errorApproveAll and suppresses future prompts.
func TestRetryCountLimit_IgnoreAll(t *testing.T) {
	a := newRetryLimitAgent(3, 3, buildRetryUser("指令", 3))
	io := &retryLimitTestIO{readVal: "A"}
	a.SetIO(io)

	ok, err := a.checkRetryCountLimit()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true (ignore all), got false")
	}
	if !a.errorApproveAll {
		t.Fatalf("expected errorApproveAll=true, got false")
	}

	// Bump retried count above threshold again — no further prompt.
	a.applyLoopFeedback("")
	a.applyLoopFeedback("")
	if got := retryEnv(msgEnv(&a.messages[1])); got < 3 {
		t.Fatalf("retried count should exceed threshold, got %d", got)
	}
	io.readCalls = 0
	ok, err = a.checkRetryCountLimit()
	if err != nil {
		t.Fatalf("expected nil error (suppressed), got %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true (suppressed), got false")
	}
	if io.readCalls != 0 {
		t.Fatalf("expected no ReadLine call after ignore-all, got %d", io.readCalls)
	}
}

// TestRetryCountLimit_DefaultThreshold verifies UC-0005:
// config <= 0 falls back to default threshold 10.
func TestRetryCountLimit_DefaultThreshold(t *testing.T) {
	a := newRetryLimitAgent(9, 0, buildRetryUser("指令", 9))
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	// 9 < 10 (default) → no prompt
	ok, err := a.checkRetryCountLimit()
	if err != nil || !ok {
		t.Fatalf("expected (true, nil) for retry=9 < default 10, got (%v, %v)", ok, err)
	}
	if io.readCalls != 0 {
		t.Fatalf("expected no ReadLine call, got %d", io.readCalls)
	}

	// Bump to 10 → prompt triggers
	a.applyLoopFeedback("")
	io.readCalls = 0
	ok, err = a.checkRetryCountLimit()
	if err != nil {
		t.Fatalf("expected nil error (Enter), got %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	if io.readCalls != 1 {
		t.Fatalf("expected 1 ReadLine call at threshold 10, got %d", io.readCalls)
	}
}

// TestRetryCountLimit_NoUserMessage verifies UC-0006:
// no user message → no prompt.
func TestRetryCountLimit_NoUserMessage(t *testing.T) {
	a := &Agent{
		cfg:      &config.Config{},
		messages: []llm.Message{{Role: "system", Content: "system"}},
	}
	a.cfg.LLM.ErrorMaxSingleCount = 3
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	ok, err := a.checkRetryCountLimit()
	if err != nil || !ok {
		t.Fatalf("expected (true, nil) for no user message, got (%v, %v)", ok, err)
	}
	if io.readCalls != 0 {
		t.Fatalf("expected no ReadLine call, got %d", io.readCalls)
	}
}

// TestRetryCountLimit_ResetsThenReaccumulates verifies UC-0011:
// after Enter reset to 1, reaccumulating to the threshold triggers again.
func TestRetryCountLimit_ResetsThenReaccumulates(t *testing.T) {
	a := newRetryLimitAgent(3, 3, buildRetryUser("指令", 3))
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	// First trigger: Enter resets to 1.
	ok, err := a.checkRetryCountLimit()
	if err != nil || !ok {
		t.Fatalf("expected (true, nil) on first trigger, got (%v, %v)", ok, err)
	}
	if got := retryEnv(msgEnv(&a.messages[1])); got != 1 {
		t.Fatalf("retried count should reset to 1, got %d", got)
	}

	// Reaccumulate to 3 via applyLoopFeedback("").
	a.applyLoopFeedback("")
	a.applyLoopFeedback("")
	if got := retryEnv(msgEnv(&a.messages[1])); got != 3 {
		t.Fatalf("retried count should reach 3, got %d", got)
	}

	// Second trigger should prompt again.
	io.readCalls = 0
	ok, err = a.checkRetryCountLimit()
	if err != nil || !ok {
		t.Fatalf("expected (true, nil) on second trigger, got (%v, %v)", ok, err)
	}
	if io.readCalls != 1 {
		t.Fatalf("expected 1 ReadLine call on second trigger, got %d", io.readCalls)
	}
	if got := retryEnv(msgEnv(&a.messages[1])); got != 1 {
		t.Errorf("retried count should reset to 1 again, got %d", got)
	}
}

// TestRetryCountLimit_InterventionEnter verifies UC-0009:
// applyLoopIntervention reaches the retry limit and Enter continues (returns nil).
func TestRetryCountLimit_InterventionEnter(t *testing.T) {
	a := newRetryLimitAgent(2, 3, buildRetryUser("指令", 2))
	a.cfg.LLM.LoopIntervention = "retry"
	a.cfg.LLM.LoopJudgeEnabled = false
	io := &retryLimitTestIO{readVal: ""}
	a.SetIO(io)

	// applyLoopFeedback("") bumps retried count 2 → 3 (threshold).
	// Then checkRetryCountLimit prompts; Enter resets to 1 and continues.
	err := a.applyLoopIntervention(&LoopEvent{
		Type:     LoopEventToolCallRepeat,
		Detector: "tool call loop detector",
		Reason:   "tool called twice consecutively",
	})
	if err != nil {
		t.Fatalf("expected nil error (Enter continues), got %v", err)
	}
	if io.readCalls != 1 {
		t.Fatalf("expected 1 ReadLine call, got %d", io.readCalls)
	}
	// retried count reset to 1 after Enter.
	if got := retryEnv(msgEnv(&a.messages[1])); got != 1 {
		t.Errorf("retried count should reset to 1, got %d", got)
	}
}

// TestRetryCountLimit_InterventionCancel verifies UC-0010:
// applyLoopIntervention reaches the retry limit and C cancels (returns error).
func TestRetryCountLimit_InterventionCancel(t *testing.T) {
	a := newRetryLimitAgent(2, 3, buildRetryUser("指令", 2))
	a.cfg.LLM.LoopIntervention = "retry"
	a.cfg.LLM.LoopJudgeEnabled = false
	io := &retryLimitTestIO{readVal: "C"}
	a.SetIO(io)

	err := a.applyLoopIntervention(&LoopEvent{
		Type:     LoopEventToolCallRepeat,
		Detector: "tool call loop detector",
		Reason:   "tool called twice consecutively",
	})
	if err == nil {
		t.Fatalf("expected error (C cancels), got nil")
	}
	// retried count stays at 3 on cancel.
	if got := retryEnv(msgEnv(&a.messages[1])); got != 3 {
		t.Errorf("retried count should stay 3 on cancel, got %d", got)
	}
}

// errorsAs is a small helper matching errors.As semantics without importing
// the errors package under a conflicting name in tests.
func errorsAs(err error, target interface{}) bool {
	type causer interface{ Unwrap() error }
	for err != nil {
		if t, ok := target.(interface {
			error
			As(e interface{}) bool
		}); ok {
			if t.As(err) {
				return true
			}
		}
		switch e := err.(type) {
		case *retryCountCancelError:
			switch t := target.(type) {
			case **retryCountCancelError:
				*t = e
				return true
			}
		}
		c, ok := err.(causer)
		if !ok {
			return false
		}
		err = c.Unwrap()
	}
	return false
}
