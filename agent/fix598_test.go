// Package agent - FIX-598 tests: an LLM call failure must be handled according
// to its cause, and only request-shape problems may edit the conversation.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// timeoutErr mimics the network-level failures Go's http client reports.
type timeoutErr struct{ msg string }

func (e timeoutErr) Error() string   { return e.msg }
func (e timeoutErr) Timeout() bool   { return true }
func (e timeoutErr) Temporary() bool { return true }

// TestClassifyLLMError pins the taxonomy the agent loop now reacts to.
func TestClassifyLLMError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want LLMErrKind
	}{
		{"nil", nil, LLMErrUnknown},
		{
			"dial timeout (url.Error)",
			&url.Error{Op: "Post", URL: "https://api.deepseek.com/chat/completions", Err: timeoutErr{"dial tcp 119.188.175.46:443: i/o timeout"}},
			LLMErrNetwork,
		},
		{"deadline exceeded", context.DeadlineExceeded, LLMErrNetwork},
		{"cancelled context is not retryable", context.Canceled, LLMErrUnknown},
		{"connection reset text", errors.New("read tcp 192.168.3.47:65208->119.188.175.46:443: read: connection reset by peer"), LLMErrNetwork},
		{"auth 401", &llm.OpenAIError{StatusCode: 401, Message: "invalid api key"}, LLMErrAuth},
		{"model not found 404", &llm.OpenAIError{StatusCode: 404, Message: "model does not exist"}, LLMErrAuth},
		{"rate limit 429", &llm.OpenAIError{StatusCode: 429, Message: "rate limit reached"}, LLMErrRateLimit},
		{"server 503", &llm.OpenAIError{StatusCode: 503, Message: "service unavailable"}, LLMErrServer},
		{
			"tool_calls format 400",
			&llm.OpenAIError{StatusCode: 400, Message: "An assistant message with 'tool_calls' must be followed by tool messages responding to each tool_call_id"},
			LLMErrContextFormat,
		},
		{"context too long 400", &llm.OpenAIError{StatusCode: 400, Message: "maximum context length is 65536 tokens"}, LLMErrContextTooLong},
		{"unclassifiable 400", &llm.OpenAIError{StatusCode: 400, Message: "vendor specific complaint"}, LLMErrUnknown},
	}
	for _, c := range cases {
		if got := ClassifyLLMError(c.err); got != c.want {
			t.Errorf("%s: ClassifyLLMError = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestOnlyRequestShapeErrorsRemoveContext is the FIX-598 acceptance criterion:
// network / auth / rate-limit / server / unknown failures must never drop the
// last assistant tool-call message from the conversation.
func TestOnlyRequestShapeErrorsRemoveContext(t *testing.T) {
	for _, k := range []LLMErrKind{LLMErrNetwork, LLMErrAuth, LLMErrRateLimit, LLMErrServer, LLMErrUnknown} {
		if k.AllowsContextRemoval() {
			t.Errorf("%q must not remove context", k)
		}
	}
	for _, k := range []LLMErrKind{LLMErrContextFormat, LLMErrContextTooLong} {
		if !k.AllowsContextRemoval() {
			t.Errorf("%q must keep the legacy removal behaviour", k)
		}
	}
	if LLMErrAuth.Retryable() || LLMErrUnknown.Retryable() || LLMErrContextFormat.Retryable() {
		t.Error("only network/rate-limit/server failures may be retried in place")
	}
	for _, k := range []LLMErrKind{LLMErrNetwork, LLMErrRateLimit, LLMErrServer} {
		if !k.Retryable() {
			t.Errorf("%q must be retryable", k)
		}
	}
}

// TestLLMErrRetryBudgetIsBounded: transient retries are capped per turn and
// restored by a successful call.
func TestLLMErrRetryBudgetIsBounded(t *testing.T) {
	a := &Agent{}
	for i := 1; i <= llmRetryLimit; i++ {
		if !a.noteLLMErrRetryAndCheckLimit() {
			t.Fatalf("retry %d of %d must be allowed", i, llmRetryLimit)
		}
	}
	if a.noteLLMErrRetryAndCheckLimit() {
		t.Fatalf("retry %d must be refused", llmRetryLimit+1)
	}
	a.resetLLMErrRetries()
	if a.llmErrRetryAttempt() != 0 {
		t.Fatalf("resetLLMErrRetries left attempt=%d", a.llmErrRetryAttempt())
	}
	if !a.noteLLMErrRetryAndCheckLimit() {
		t.Fatal("a successful call must restore the retry budget")
	}
}

// TestLLMErrRetryDelayBacksOffAndCaps: 1s, 2s, 4s, then the 8s cap.
func TestLLMErrRetryDelayBacksOffAndCaps(t *testing.T) {
	a := &Agent{}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second}
	for i, w := range want {
		a.noteLLMErrRetryAndCheckLimit()
		if got := a.llmErrRetryDelay(); got != w {
			t.Fatalf("attempt %d: delay = %v, want %v", i+1, got, w)
		}
	}
}

// TestSleepCtxStopsOnCancellation: a cancelled turn must not sleep through the
// backoff.
func TestSleepCtxStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, time.Hour) {
		t.Fatal("sleepCtx must return false once the context is done")
	}
	if !sleepCtx(context.Background(), time.Millisecond) {
		t.Fatal("sleepCtx must return true when the timer fires first")
	}
}
