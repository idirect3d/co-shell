// Integration tests for FEATURE-556 Phase 3 tab targets and request mocking
// against a real headless Chrome.
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0020.md (group B, B-032..B-035).
// Gated by testing.Short() and skipped when no Chrome is installed.

package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestIntegrationP3Tabs covers B-032/B-033: the session page is listed with a
// usable debugger URL, opening and closing a tab changes the list, switching
// reconnects the session, and invalid selectors report what is available.
func TestIntegrationP3Tabs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	manager, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	uc556Navigate(t, client, fixtures.server.URL+"/plain.html")

	// B-032: the attached page is listed, with the debugger URL needed to
	// switch to it.
	pages, err := manager.PageTargets(ctx)
	if err != nil {
		t.Fatalf("PageTargets: %v", err)
	}
	if len(pages) == 0 {
		t.Fatalf("PageTargets returned no page target")
	}
	for _, page := range pages {
		if page.TargetID == "" {
			t.Fatalf("page target without id: %+v", page)
		}
		if _, err := SwitchTargetURL(page); err != nil {
			t.Fatalf("page target %s cannot be switched to: %v", page.TargetID, err)
		}
	}

	targetID, err := CreateTarget(ctx, client, fixtures.server.URL+"/plain.html")
	if err != nil {
		t.Fatalf("CreateTarget: %v", err)
	}
	opened, err := manager.PageTargets(ctx)
	if err != nil {
		t.Fatalf("PageTargets after create: %v", err)
	}
	if len(opened) != len(pages)+1 {
		t.Fatalf("pages after create=%d, want %d", len(opened), len(pages)+1)
	}

	// B-033: switching reconnects this session to the chosen tab.
	selected, err := SelectTarget(opened, 0, targetID)
	if err != nil {
		t.Fatalf("SelectTarget(target_id): %v", err)
	}
	wsURL, err := SwitchTargetURL(selected)
	if err != nil {
		t.Fatalf("SwitchTargetURL: %v", err)
	}
	if err := manager.SelectTarget(wsURL); err != nil {
		t.Fatalf("SelectTarget(ws): %v", err)
	}
	switched := manager.Client()
	if switched == nil {
		t.Fatalf("no CDP client after switching tabs")
	}
	if got := uc556P3EvalEventually(t, switched, "document.querySelector('#out').textContent", "plain", 15*time.Second); got != "plain" {
		t.Fatalf("switched page content=%q, want plain", got)
	}
	if err := ActivateTarget(ctx, switched, targetID); err != nil {
		t.Fatalf("ActivateTarget: %v", err)
	}

	// Closing the page the session is attached to must not wedge the browser:
	// the dead connection reports an error instead of hanging.
	closed, err := CloseTarget(ctx, switched, targetID)
	if err != nil {
		t.Fatalf("CloseTarget: %v", err)
	}
	if !closed {
		t.Fatalf("CloseTarget reported success=false")
	}
	evalCtx, evalCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer evalCancel()
	if _, err := switched.Evaluate(evalCtx, "1 + 1"); err == nil {
		t.Fatalf("evaluate on the closed page succeeded, want an error")
	}

	manager.DropConnection()

	if _, err := manager.EnsurePageConnected(ctx); err != nil {
		t.Fatalf("EnsurePageConnected after closing the current tab: %v", err)
	}
	remaining, err := manager.PageTargets(ctx)
	if err != nil {
		t.Fatalf("PageTargets after close: %v", err)
	}
	if len(remaining) == 0 {
		t.Fatalf("no page target left after closing the extra tab")
	}
	for _, page := range remaining {
		if page.TargetID == targetID {
			t.Fatalf("closed target %s is still listed", targetID)
		}
	}

	// B-033: bad selectors name what is available instead of failing silently.
	if _, err := SelectTarget(remaining, len(remaining)+1, ""); err == nil {
		t.Fatalf("SelectTarget(out of range) returned no error")
	} else if !containsAll(err.Error(), "out of range") {
		t.Fatalf("out-of-range error does not describe the range: %v", err)
	}
	if _, err := SelectTarget(remaining, 0, "0123456789abcdef"); err == nil {
		t.Fatalf("SelectTarget(unknown target id) returned no error")
	} else if !containsAll(err.Error(), "0123456789abcdef") {
		t.Fatalf("unknown-id error does not name the id: %v", err)
	}
}

// TestIntegrationP3RequestMock covers B-034/B-035: a fulfill rule replaces the
// response, a fail rule breaks the request, Off restores the real resource, and
// interception stays off until the first rule is added.
func TestIntegrationP3RequestMock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	manager, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	mock, err := manager.Mock()
	if err != nil {
		t.Fatalf("Mock: %v", err)
	}
	t.Cleanup(func() { mock.Close() })
	if mock.Enabled() {
		t.Fatalf("interception enabled before any rule was added")
	}

	// Baseline: /real.png is a 3x3 image.
	uc556Navigate(t, client, fixtures.server.URL+"/mock.html?asset=baseline")
	if got := uc556P3EvalEventually(t, client, "document.querySelector('#pic').naturalWidth", "3", 15*time.Second); got != "3" {
		t.Fatalf("real image width=%q, want 3", got)
	}

	// B-034: the fulfill rule answers with the canned 1x1 image.
	rule := MockRule{
		Match:      "/real.png",
		Action:     MockActionFulfill,
		Status:     200,
		Headers:    map[string]string{"Content-Type": "image/png"},
		Body:       base64.StdEncoding.EncodeToString(uc556P3PNG(t, 1)),
		Base64Body: true,
	}
	if err := mock.AddRule(ctx, rule); err != nil {
		t.Fatalf("AddRule(fulfill): %v", err)
	}
	if !mock.Enabled() {
		t.Fatalf("interception not enabled after adding a rule")
	}
	if got := len(mock.Rules()); got != 1 {
		t.Fatalf("Rules()=%d, want 1", got)
	}

	uc556Navigate(t, client, fixtures.server.URL+"/mock.html?asset=fulfilled")
	if got := uc556P3EvalEventually(t, client, "document.querySelector('#pic').naturalWidth", "1", 15*time.Second); got != "1" {
		t.Fatalf("mocked image width=%q, want 1 (fulfill rule not applied)", got)
	}

	// Rules that do not match keep working through to the network: the document
	// and other subresources above were continued untouched.

	// B-035: the fail rule breaks the request instead of returning content.
	if err := mock.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if mock.Enabled() {
		t.Fatalf("interception still enabled after Clear")
	}
	if err := mock.AddRule(ctx, MockRule{Match: "/real.png", Action: MockActionFail, ErrorReason: "Failed"}); err != nil {
		t.Fatalf("AddRule(fail): %v", err)
	}
	uc556Navigate(t, client, fixtures.server.URL+"/mock.html?asset=failed")
	if got := uc556P3EvalEventually(t, client, "document.querySelector('#pic').naturalWidth", "0", 15*time.Second); got != "0" {
		t.Fatalf("failed image width=%q, want 0", got)
	}

	// Off restores the browser's normal request behaviour.
	if err := mock.Off(ctx); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if mock.Enabled() {
		t.Fatalf("interception still enabled after Off")
	}
	if got := len(mock.Rules()); got != 0 {
		t.Fatalf("Rules()=%d after Off, want 0", got)
	}
	uc556Navigate(t, client, fixtures.server.URL+"/mock.html?asset=restored")
	if got := uc556P3EvalEventually(t, client, "document.querySelector('#pic').naturalWidth", "3", 15*time.Second); got != "3" {
		t.Fatalf("image width=%q after Off, want the real 3", got)
	}
}

// uc556P3EvalEventually evaluates expression until it returns want, tolerating
// the transient errors of a target that is still loading.
func uc556P3EvalEventually(t *testing.T, client *CDPClient, expression, want string, timeout time.Duration) string {
	t.Helper()

	last := ""
	uc556P3Eventually(t, timeout, fmt.Sprintf("%s == %q", expression, want), func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		value, err := client.Evaluate(ctx, expression)
		if err != nil {
			return false
		}
		last = fmt.Sprintf("%v", value)
		return last == want
	})
	return last
}

// containsAll reports whether every part occurs in value.
func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
