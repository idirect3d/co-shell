// Package agent - FIX-598: classify LLM call failures by cause.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// LLMErrKind classifies why an LLM call failed so the agent loop can react to
// the cause instead of inferring it from the shape of the conversation.
//
// Before FIX-598 every streaming-call failure was treated as "the context holds
// a malformed tool call": the loop deleted the last assistant message with
// tool_calls plus everything after it and retried. A plain network timeout
// therefore silently dropped legitimate history — the model lost the record of
// the tool it had just run (and could run side effects again) — and the useless
// delete/retry cycle only stopped at the context-remove limit.
type LLMErrKind string

const (
	// LLMErrNetwork: the request never reached the API or the response never
	// came back — DNS, dial, TLS, reset, timeout. Transient; the context is fine.
	LLMErrNetwork LLMErrKind = "network"
	// LLMErrRateLimit: the API answered 429 / "too many requests".
	LLMErrRateLimit LLMErrKind = "rate-limit"
	// LLMErrServer: the API answered 5xx.
	LLMErrServer LLMErrKind = "server"
	// LLMErrAuth: credentials, model id or quota — retrying cannot help.
	LLMErrAuth LLMErrKind = "auth"
	// LLMErrContextFormat: the request was rejected because the message list is
	// malformed (a tool result without its tool_calls, wrong roles, bad args).
	LLMErrContextFormat LLMErrKind = "context-format"
	// LLMErrContextTooLong: the message list no longer fits the model.
	LLMErrContextTooLong LLMErrKind = "context-too-long"
	// LLMErrUnknown: not classifiable; handled conservatively (no removal).
	LLMErrUnknown LLMErrKind = "unknown"
)

// llmRetryLimit is how many times a transient failure (network / 5xx / 429) is
// re-issued unchanged before the turn is aborted. Editing the history cannot
// fix those, so the retries are bounded and the context is left alone.
const llmRetryLimit = 3

// llmRetryMaxDelay caps the exponential backoff (1s, 2s, 4s … 8s).
const llmRetryMaxDelay = 8 * time.Second

// AllowsContextRemoval reports whether dropping the last assistant tool-call
// message (and everything after it) is a sensible reaction to this failure.
// Only request-shape problems qualify: a network outage, an auth failure or a
// rate limit is not fixed by editing the conversation.
func (k LLMErrKind) AllowsContextRemoval() bool {
	return k == LLMErrContextFormat || k == LLMErrContextTooLong
}

// Retryable reports whether re-issuing the same request may succeed later.
func (k LLMErrKind) Retryable() bool {
	switch k {
	case LLMErrNetwork, LLMErrRateLimit, LLMErrServer:
		return true
	default:
		return false
	}
}

var (
	// Transport-level phrases seen in the wild when the request never got an
	// HTTP answer (Go's http client wraps most of them in *url.Error already).
	networkNeedles = []string{
		"i/o timeout", "connection reset", "connection refused", "connection closed",
		"no such host", "tls handshake", "broken pipe", "unexpected eof",
		"network is unreachable", "proxyconnect", "eof",
	}
	rateLimitNeedles = []string{"rate limit", "too many requests", "429"}
	authNeedles      = []string{
		"invalid api key", "unauthorized", "authentication", "api key",
		"model not found", "model does not exist", "insufficient quota",
		"insufficient_quota", "billing", "permission denied",
	}
	contextFormatNeedles = []string{
		"tool_call_id", "tool_calls", "messages with role", "must be a response to",
		"function_call", "invalid_request_error", "invalid type for",
		"invalid value for", "unexpected role",
	}
	contextTooLongNeedles = []string{
		"maximum context length", "context length", "too many tokens",
		"reduce the length", "string too long",
	}
)

// ClassifyLLMError maps a streaming-call failure to the reaction the agent loop
// should take. It is pure, so it is unit-tested directly.
//
// Note: this is deliberately separate from classifyConnectionError
// (problem_solver.go), which only answers "is this a sufficient condition to
// skip the problem model?".
func ClassifyLLMError(err error) LLMErrKind {
	if err == nil {
		return LLMErrUnknown
	}
	// A cancelled parent context is not a retryable API problem: the caller is
	// shutting the turn down.
	if errors.Is(err, context.Canceled) {
		return LLMErrUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return LLMErrNetwork
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return LLMErrNetwork
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		return LLMErrNetwork
	}
	var apiErr *llm.OpenAIError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch {
		case apiErr.StatusCode == 429:
			return LLMErrRateLimit
		case apiErr.StatusCode == 401, apiErr.StatusCode == 403, apiErr.StatusCode == 404:
			return LLMErrAuth
		case apiErr.StatusCode >= 500:
			return LLMErrServer
		case apiErr.StatusCode == 400, apiErr.StatusCode == 422:
			return classifyTextCategory(apiErr.Message)
		}
	}
	return classifyTextCategory(err.Error())
}

// classifyTextCategory maps the error text onto a category. Order matters:
// request-shape problems are checked before the generic network phrases so a
// "400 invalid_request_error" is never mistaken for a transport failure.
func classifyTextCategory(msg string) LLMErrKind {
	low := strings.ToLower(msg)
	if containsAny(low, contextFormatNeedles) {
		return LLMErrContextFormat
	}
	if containsAny(low, contextTooLongNeedles) {
		return LLMErrContextTooLong
	}
	if containsAny(low, authNeedles) {
		return LLMErrAuth
	}
	if containsAny(low, rateLimitNeedles) {
		return LLMErrRateLimit
	}
	if containsAny(low, networkNeedles) {
		return LLMErrNetwork
	}
	return LLMErrUnknown
}

func containsAny(low string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(low, n) {
			return true
		}
	}
	return false
}

// noteLLMErrRetryAndCheckLimit records one in-place retry of a transient LLM
// failure and reports whether another one is still allowed.
func (a *Agent) noteLLMErrRetryAndCheckLimit() bool {
	a.mu.Lock()
	a.llmErrRetryCount++
	n := a.llmErrRetryCount
	a.mu.Unlock()
	return n <= llmRetryLimit
}

// llmErrRetryAttempt returns how many in-place retries have been recorded.
func (a *Agent) llmErrRetryAttempt() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.llmErrRetryCount
}

// resetLLMErrRetries clears the retry budget; called after a call succeeds so a
// later, unrelated failure starts with a fresh one.
func (a *Agent) resetLLMErrRetries() {
	a.mu.Lock()
	a.llmErrRetryCount = 0
	a.mu.Unlock()
}

// llmErrRetryDelay returns the backoff before the recorded retry: 1s, 2s, 4s,
// capped at llmRetryMaxDelay.
func (a *Agent) llmErrRetryDelay() time.Duration {
	n := a.llmErrRetryAttempt()
	if n < 1 {
		n = 1
	}
	d := time.Second << uint(n-1)
	if d > llmRetryMaxDelay {
		d = llmRetryMaxDelay
	}
	return d
}

// sleepCtx waits for d or until ctx is done (false when it finished first).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
