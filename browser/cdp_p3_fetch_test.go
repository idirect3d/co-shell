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

package browser

import (
	"context"
	"encoding/base64"
	"testing"
	"time"
)

// hangingSession records calls like mockSession but blocks one method until the
// test releases it, which exercises the fallback that continues a paused request
// while the rule's call is still in flight.
type hangingSession struct {
	*mockSession
	hangMethod string
	release    chan struct{}
}

// Call delegates to the recording session, blocking the configured method.
func (h *hangingSession) Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	if method == h.hangMethod {
		_, _ = h.mockSession.Call(ctx, method, params)
		<-h.release
		return map[string]interface{}{}, nil
	}
	return h.mockSession.Call(ctx, method, params)
}

// countCalls counts how often method was recorded.
func countCalls(session *mockSession, method string) int {
	count := 0
	for _, name := range session.methods() {
		if name == method {
			count++
		}
	}
	return count
}

// waitCallParams waits until method was called and returns its params.
func waitCallParams(t *testing.T, session *mockSession, method string) map[string]interface{} {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if params := session.paramsOf(method); params != nil {
			return params
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s was not called: %v", method, session.methods())
	return nil
}

// TestMockRuleMatching covers A-034: substring and glob matching semantics.
func TestMockRuleMatching(t *testing.T) {
	substring := MockRule{Match: "api/user"}
	if !substring.Matches("http://example.test/api/user?id=1") {
		t.Error("a substring rule must match anywhere in the URL, query string included")
	}
	if substring.Matches("http://example.test/api/order") {
		t.Error("a URL without the substring must not match")
	}
	if substring.Matches("http://example.test/API/USER") {
		t.Error("matching is case sensitive")
	}

	glob := MockRule{Match: "*/mock-*.png", Glob: true}
	if !glob.Matches("http://127.0.0.1:8080/mock-target.png") {
		t.Error("'*' must match across '/' in a glob rule")
	}
	if glob.Matches("http://127.0.0.1:8080/real-target.png") {
		t.Error("a non-matching glob must not match")
	}

	question := MockRule{Match: "*/mock-?.png", Glob: true}
	if !question.Matches("http://example.test/mock-1.png") || question.Matches("http://example.test/mock-12.png") {
		t.Error("'?' must match exactly one character")
	}

	exact := MockRule{Match: "http://example.test/api/user"}
	if !exact.Matches("http://example.test/api/user") {
		t.Error("a full URL rule must match the URL itself")
	}
	if !exact.Matches("http://example.test/api/user/2") {
		t.Error("substring semantics mean a longer URL containing the pattern also matches")
	}
}

// TestMockRuleValidation covers A-037: invalid rules are rejected up front.
func TestMockRuleValidation(t *testing.T) {
	valid := []MockRule{
		{Match: "api", Action: MockActionContinue},
		{Match: "api", Action: MockActionFulfill, Status: 200},
		{Match: "api", Action: MockActionFail, ErrorReason: "Failed"},
	}
	for _, rule := range valid {
		if err := rule.Validate(); err != nil {
			t.Errorf("rule %+v must be valid: %v", rule, err)
		}
	}

	invalid := []MockRule{
		{Action: MockActionContinue},
		{Match: "api", Action: MockActionFulfill, Status: 42},
		{Match: "api", Action: MockActionFail, ErrorReason: "NotAReason"},
		{Match: "api", Action: "explode"},
	}
	for _, rule := range invalid {
		if err := rule.Validate(); err == nil {
			t.Errorf("rule %+v must be rejected", rule)
		}
	}

	description := MockRule{Match: "*/mock-*.png", Glob: true, Action: MockActionFulfill, Status: 201, Body: "x"}.Describe()
	if description == "" {
		t.Error("Describe must render a rule for tool output")
	}
}

// TestFetchMockLifecycle covers A-038: interception only exists while a rule is
// active, and disabling it twice stays idempotent.
func TestFetchMockLifecycle(t *testing.T) {
	session := newMockSession()
	mock := NewFetchMock(session)

	if mock.Enabled() {
		t.Error("a fresh mock must be idle")
	}
	if session.called("Fetch.enable") {
		t.Errorf("no Fetch.enable without rules: %v", session.methods())
	}
	if rules := mock.Rules(); rules != nil {
		t.Errorf("a fresh mock must have no rules: %v", rules)
	}

	rule := MockRule{Match: "*/mock-target.png", Glob: true, Action: MockActionFulfill, Status: 200, Body: "MOCK"}
	if err := mock.AddRule(context.Background(), rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if !mock.Enabled() || !session.called("Fetch.enable") {
		t.Error("the first rule must enable interception")
	}
	if params := session.paramsOf("Fetch.enable"); params["patterns"] == nil {
		t.Errorf("Fetch.enable must carry interception patterns: %v", params)
	}
	if len(mock.Rules()) != 1 {
		t.Errorf("rules = %d, want 1", len(mock.Rules()))
	}

	if err := mock.Off(context.Background()); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if mock.Enabled() || len(mock.Rules()) != 0 {
		t.Error("off must clear the rules and disable interception")
	}
	if calls := countCalls(session, "Fetch.disable"); calls != 1 {
		t.Errorf("Fetch.disable called %d times, want 1", calls)
	}

	if err := mock.Off(context.Background()); err != nil {
		t.Fatalf("second Off: %v", err)
	}
	if calls := countCalls(session, "Fetch.disable"); calls != 1 {
		t.Errorf("a repeated off must not call Fetch.disable again, got %d", calls)
	}
}

// TestFetchMockAnswers covers A-035, A-036 and A-037: the request is answered
// according to the matching rule, or continued when nothing matches.
func TestFetchMockAnswers(t *testing.T) {
	cases := []struct {
		name       string
		rule       MockRule
		url        string
		wantMethod string
		check      func(t *testing.T, params map[string]interface{})
	}{
		{
			name:       "unmatched request continues",
			rule:       MockRule{Match: "*/other.png", Glob: true, Action: MockActionFulfill, Status: 200},
			url:        "http://example.test/mock-target.png",
			wantMethod: "Fetch.continueRequest",
			check: func(t *testing.T, params map[string]interface{}) {
				if len(params) != 1 || params["requestId"] != "P1" {
					t.Errorf("continue params = %v, want only the request id", params)
				}
			},
		},
		{
			name:       "continue rule",
			rule:       MockRule{Match: "mock-target", Action: MockActionContinue},
			url:        "http://example.test/mock-target.png",
			wantMethod: "Fetch.continueRequest",
			check: func(t *testing.T, params map[string]interface{}) {
				if len(params) != 1 || params["requestId"] != "P1" {
					t.Errorf("continue params = %v, want only the request id", params)
				}
			},
		},
		{
			name: "fulfill rule",
			rule: MockRule{
				Match:   "mock-target",
				Action:  MockActionFulfill,
				Status:  201,
				Headers: map[string]string{"X-Mock": "1"},
				Body:    "hello",
			},
			url:        "http://example.test/mock-target.png",
			wantMethod: "Fetch.fulfillRequest",
			check: func(t *testing.T, params map[string]interface{}) {
				if params["responseCode"] != 201 {
					t.Errorf("responseCode = %v, want 201", params["responseCode"])
				}
				headers, ok := params["responseHeaders"].([]map[string]interface{})
				if !ok || len(headers) != 1 || headers[0]["name"] != "X-Mock" || headers[0]["value"] != "1" {
					t.Errorf("responseHeaders = %v, want [{name:X-Mock value:1}]", params["responseHeaders"])
				}
				want := base64.StdEncoding.EncodeToString([]byte("hello"))
				if params["body"] != want {
					t.Errorf("body = %v, want base64 of %q", params["body"], "hello")
				}
			},
		},
		{
			name:       "fail rule",
			rule:       MockRule{Match: "mock-target", Action: MockActionFail, ErrorReason: "Aborted"},
			url:        "http://example.test/mock-target.png",
			wantMethod: "Fetch.failRequest",
			check: func(t *testing.T, params map[string]interface{}) {
				if params["errorReason"] != "Aborted" {
					t.Errorf("errorReason = %v, want Aborted", params["errorReason"])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := newMockSession()
			mock := NewFetchMock(session)
			defer mock.Close()

			if err := mock.AddRule(context.Background(), tc.rule); err != nil {
				t.Fatalf("AddRule: %v", err)
			}

			session.emit("Fetch.requestPaused", map[string]interface{}{
				"requestId": "P1",
				"request":   map[string]interface{}{"url": tc.url},
			})

			tc.check(t, waitCallParams(t, session, tc.wantMethod))
		})
	}
}

// TestFetchMockPauseFallback covers the safety net: a paused request is
// continued even when answering it fails.
func TestFetchMockPauseFallback(t *testing.T) {
	session := newMockSession()
	hanging := &hangingSession{
		mockSession: session,
		hangMethod:  "Fetch.fulfillRequest",
		release:     make(chan struct{}),
	}

	mock := NewFetchMock(hanging)
	mock.pauseTimeout = 50 * time.Millisecond

	rule := MockRule{Match: "mock-target", Action: MockActionFulfill, Status: 200, Body: "x"}
	if err := mock.AddRule(context.Background(), rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	hanging.emit("Fetch.requestPaused", map[string]interface{}{
		"requestId": "P9",
		"request":   map[string]interface{}{"url": "http://example.test/mock-target.png"},
	})

	// The rule's call is still in flight, yet the request must not stay paused.
	params := waitCallParams(t, session, "Fetch.continueRequest")
	if params["requestId"] != "P9" {
		t.Errorf("fallback continue params = %v, want request id P9", params)
	}

	close(hanging.release)
	mock.Close()
}
