// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
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

// Request mocking built on the Fetch domain: in-memory URL rules decide whether
// a request is fulfilled with a canned response, failed, or passed through.
//
// Fetch interception is enabled only once a rule exists, so browsing issues no
// Fetch.enable call while no mock is active (FEATURE-556 Phase 3). Every paused
// request is answered — a timer continues it when the rule's call does not
// return in time — because a paused request that is never answered would hang
// the page.

package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/log"
)

const (
	// MockActionFulfill answers the request with a canned response.
	MockActionFulfill = "fulfill"
	// MockActionFail fails the request.
	MockActionFail = "fail"
	// MockActionContinue lets the request reach the network untouched.
	MockActionContinue = "continue"

	// mockPauseTimeout is the safety net for a paused request.
	mockPauseTimeout = 3 * time.Second
	// mockCallTimeout bounds a single Fetch response call.
	mockCallTimeout = 5 * time.Second
)

// mockErrorReasons lists the Fetch.failRequest error reasons Chrome accepts.
var mockErrorReasons = []string{
	"Failed", "Aborted", "TimedOut", "AccessDenied", "ConnectionClosed",
	"ConnectionReset", "ConnectionRefused", "ConnectionAborted", "ConnectionFailed",
	"NameNotResolved", "InternetDisconnected", "AddressUnreachable",
	"BlockedByClient", "BlockedByResponse",
}

// MockRule is one in-memory request mocking rule.
type MockRule struct {
	Match       string            // URL substring, or glob pattern when Glob is set
	Glob        bool              // Match is a glob ('*' and '?') instead of a substring
	Action      string            // fulfill/fail/continue
	Status      int               // response code for fulfill
	Headers     map[string]string // response headers for fulfill
	Body        string            // response body for fulfill
	Base64Body  bool              // Body is already base64 encoded
	ErrorReason string            // Fetch error reason for fail
}

// Validate reports why a rule cannot be used.
func (r MockRule) Validate() error {
	if strings.TrimSpace(r.Match) == "" {
		return fmt.Errorf("match must not be empty")
	}
	switch r.Action {
	case MockActionFulfill:
		if r.Status < 100 || r.Status > 599 {
			return fmt.Errorf("status must be between 100 and 599")
		}
	case MockActionFail:
		if !isValidMockErrorReason(r.ErrorReason) {
			return fmt.Errorf("unknown errorReason %q (valid: %s)", r.ErrorReason, strings.Join(mockErrorReasons, ", "))
		}
	case MockActionContinue:
	default:
		return fmt.Errorf("unknown action %q (valid: %s, %s, %s)", r.Action, MockActionFulfill, MockActionFail, MockActionContinue)
	}
	return nil
}

// Matches reports whether the rule applies to url. Substring rules look for
// Match anywhere in the URL; glob rules match the whole URL with '*' matching
// any run of characters (including '/') and '?' matching one character.
// Matching is case sensitive.
func (r MockRule) Matches(url string) bool {
	if r.Glob {
		return matchGlob(r.Match, url)
	}
	return strings.Contains(url, r.Match)
}

// Describe renders the rule for tool output.
func (r MockRule) Describe() string {
	kind := "substring"
	if r.Glob {
		kind = "glob"
	}
	switch r.Action {
	case MockActionFulfill:
		return fmt.Sprintf("%s %q -> fulfill %d (%d header(s), %d body byte(s))",
			kind, r.Match, r.Status, len(r.Headers), len(r.Body))
	case MockActionFail:
		return fmt.Sprintf("%s %q -> fail %s", kind, r.Match, r.ErrorReason)
	default:
		return fmt.Sprintf("%s %q -> continue", kind, r.Match)
	}
}

// FetchMock answers the paused requests of one CDP session according to a list
// of in-memory rules.
type FetchMock struct {
	session eventSession
	// pauseTimeout is the fallback delay before a paused request is continued;
	// tests may shorten it.
	pauseTimeout time.Duration

	mu      sync.Mutex
	rules   []MockRule
	enabled bool
	cancel  func()
}

// NewFetchMock creates an idle mock: no Fetch.enable is sent until a rule is
// added, so a session without mocks behaves exactly as before.
func NewFetchMock(session eventSession) *FetchMock {
	return &FetchMock{session: session, pauseTimeout: mockPauseTimeout}
}

// AddRule validates and appends a rule, enabling interception for the first one.
func (m *FetchMock) AddRule(ctx context.Context, rule MockRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	m.rules = append(m.rules, rule)
	needEnable := !m.enabled
	m.mu.Unlock()

	if needEnable {
		return m.enable(ctx)
	}
	return nil
}

// Rules returns a copy of the active rules.
func (m *FetchMock) Rules() []MockRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.rules) == 0 {
		return nil
	}
	out := make([]MockRule, len(m.rules))
	copy(out, m.rules)
	return out
}

// Enabled reports whether request interception is active.
func (m *FetchMock) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// Clear drops every rule and disables interception when it was active. Calling
// it twice is a no-op.
func (m *FetchMock) Clear(ctx context.Context) error {
	m.mu.Lock()
	m.rules = nil
	m.mu.Unlock()
	return m.disable(ctx)
}

// Off is an alias of Clear: it restores the browser's normal request behaviour.
func (m *FetchMock) Off(ctx context.Context) error {
	return m.Clear(ctx)
}

// Close stops answering requests without touching Chrome (used when the session
// goes away).
func (m *FetchMock) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.enabled = false
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// enable turns on Fetch interception and starts answering paused requests.
func (m *FetchMock) enable(ctx context.Context) error {
	// Subscribe before enabling so no paused request is missed.
	events, cancel := m.session.Watch("Fetch.requestPaused")

	callCtx, cancelCall := context.WithTimeout(ctx, mockCallTimeout)
	defer cancelCall()
	if _, err := m.session.Call(callCtx, "Fetch.enable", map[string]interface{}{
		"patterns": []map[string]interface{}{{"urlPattern": "*", "requestStage": "Request"}},
	}); err != nil {
		cancel()
		return fmt.Errorf("cannot enable request mocking: %w", err)
	}

	m.mu.Lock()
	m.cancel = cancel
	m.enabled = true
	m.mu.Unlock()

	go m.loop(events)
	return nil
}

// disable turns off Fetch interception when it was active.
func (m *FetchMock) disable(ctx context.Context) error {
	m.mu.Lock()
	cancel := m.cancel
	enabled := m.enabled
	m.cancel = nil
	m.enabled = false
	m.mu.Unlock()

	if !enabled {
		return nil
	}
	if cancel != nil {
		cancel()
	}

	callCtx, cancelCall := context.WithTimeout(ctx, mockCallTimeout)
	defer cancelCall()
	if _, err := m.session.Call(callCtx, "Fetch.disable", map[string]interface{}{}); err != nil {
		return fmt.Errorf("cannot disable request mocking: %w", err)
	}
	return nil
}

// loop answers paused requests until the subscription closes.
func (m *FetchMock) loop(events <-chan Event) {
	for ev := range events {
		m.handlePaused(ev)
	}
}

// handlePaused answers one Fetch.requestPaused event. A timer continues the
// request when the rule's call has not succeeded in time, so a paused request
// can never stall the page.
func (m *FetchMock) handlePaused(ev Event) {
	requestID := eventString(ev.Params, "requestId")
	if requestID == "" {
		return
	}
	url := ""
	if request, ok := ev.Params["request"].(map[string]interface{}); ok {
		url = eventString(request, "url")
	}

	rule, matched := m.match(url)
	method, params := mockAnswer(requestID, rule, matched)

	var mu sync.Mutex
	answered := false
	respond := func(method string, params map[string]interface{}) {
		mu.Lock()
		if answered {
			mu.Unlock()
			return
		}
		mu.Unlock()

		callCtx, cancel := context.WithTimeout(context.Background(), mockCallTimeout)
		defer cancel()
		if _, err := m.session.Call(callCtx, method, params); err != nil {
			log.Debug("mock %s failed for %s: %v", method, url, err)
			return
		}

		mu.Lock()
		answered = true
		mu.Unlock()
	}

	timer := time.AfterFunc(m.pauseTimeout, func() {
		respond("Fetch.continueRequest", map[string]interface{}{"requestId": requestID})
	})
	defer timer.Stop()

	respond(method, params)
}

// match returns the first rule matching url.
func (m *FetchMock) match(url string) (MockRule, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rule := range m.rules {
		if rule.Matches(url) {
			return rule, true
		}
	}
	return MockRule{}, false
}

// mockAnswer returns the Fetch method and parameters for a paused request.
func mockAnswer(requestID string, rule MockRule, matched bool) (string, map[string]interface{}) {
	if !matched {
		return "Fetch.continueRequest", map[string]interface{}{"requestId": requestID}
	}

	switch rule.Action {
	case MockActionFulfill:
		params := map[string]interface{}{
			"requestId":    requestID,
			"responseCode": rule.Status,
		}
		if len(rule.Headers) > 0 {
			params["responseHeaders"] = mockResponseHeaders(rule.Headers)
		}
		if rule.Body != "" {
			if rule.Base64Body {
				params["body"] = rule.Body
			} else {
				params["body"] = base64.StdEncoding.EncodeToString([]byte(rule.Body))
			}
		}
		return "Fetch.fulfillRequest", params

	case MockActionFail:
		return "Fetch.failRequest", map[string]interface{}{
			"requestId":   requestID,
			"errorReason": rule.ErrorReason,
		}

	default:
		return "Fetch.continueRequest", map[string]interface{}{"requestId": requestID}
	}
}

// mockResponseHeaders renders headers as the CDP name/value array, sorted by
// name so the result is deterministic.
func mockResponseHeaders(headers map[string]string) []map[string]interface{} {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]interface{}{"name": name, "value": headers[name]})
	}
	return out
}

// isValidMockErrorReason reports whether reason is accepted by Fetch.failRequest.
func isValidMockErrorReason(reason string) bool {
	for _, valid := range mockErrorReasons {
		if valid == reason {
			return true
		}
	}
	return false
}

// matchGlob reports whether pattern matches value, where '*' matches any run of
// characters (including '/') and '?' matches exactly one character. Matching is
// case sensitive.
func matchGlob(pattern, value string) bool {
	patternIndex, valueIndex := 0, 0
	starIndex, starValue := -1, 0

	for valueIndex < len(value) {
		switch {
		case patternIndex < len(pattern) && (pattern[patternIndex] == '?' || pattern[patternIndex] == value[valueIndex]):
			patternIndex++
			valueIndex++
		case patternIndex < len(pattern) && pattern[patternIndex] == '*':
			starIndex = patternIndex
			starValue = valueIndex
			patternIndex++
		case starIndex >= 0:
			patternIndex = starIndex + 1
			starValue++
			valueIndex = starValue
		default:
			return false
		}
	}

	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}
