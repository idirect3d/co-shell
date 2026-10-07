// Package web - FIX-619 structural assertions. The fix spans three layers (iOS
// shell, page, server) whose runtime behaviour cannot be exercised from Go, so
// the wiring that matters is asserted against the source text — the same
// approach FIX-608 used for the iOS download handling.
//
// Author: L.Shuang
// Created: 2026-10-07
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"os"
	"strings"
	"testing"
)

// fix619Read loads a source file relative to this package.
func fix619Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// fix619AssertAll checks that every wanted snippet is present in src.
func fix619AssertAll(t *testing.T, src, label string, wants []string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(src, w) {
			t.Errorf("%s: missing %q", label, w)
		}
	}
}

// TestFix619FrontendHeartbeat asserts the page owns a liveness signal: it pings
// the server and can tell a zombie socket from a live one (B).
func TestFix619FrontendHeartbeat(t *testing.T) {
	appJS := fix619Read(t, "static/app.js")
	fix619AssertAll(t, appJS, "static/app.js", []string{
		"WS_HEARTBEAT_MS = 25000", // ping cadence
		"WS_WATCHDOG_MS = 45000",  // silence that marks the socket dead
		`type: "ping"`,            // the heartbeat frame itself
		"function wsMarkActivity()",
		"function wsSilentTooLong()",
		"function startHeartbeat()",
		"function stopHeartbeat()",
		"function forceReconnect()",
		"function ensureConnected()",
	})
}

// TestFix619FrontendLifecycleSignals asserts the page re-checks the connection
// when it comes back to the foreground or the network returns.
func TestFix619FrontendLifecycleSignals(t *testing.T) {
	appJS := fix619Read(t, "static/app.js")
	fix619AssertAll(t, appJS, "static/app.js", []string{
		`addEventListener("visibilitychange"`,
		`addEventListener("pageshow"`,
		`addEventListener("online"`,
		`addEventListener("focus"`,
		"ensureConnected",
	})
}

// TestFix619FrontendGuardUsesActivity asserts the wsConnect guard no longer
// trusts an OPEN socket blindly (the zombie-socket dead end).
func TestFix619FrontendGuardUsesActivity(t *testing.T) {
	appJS := fix619Read(t, "static/app.js")
	guard := appJS[strings.Index(appJS, "function wsConnect()"):]
	guard = guard[:strings.Index(guard, "const sock = new WebSocket")]
	fix619AssertAll(t, guard, "wsConnect guard", []string{
		"wsSilentTooLong()",
		"stale.close()",
	})
}

// TestFix619FrontendOnmessageMarksActivity asserts every received frame counts
// as activity (otherwise the watchdog would kill a healthy connection).
func TestFix619FrontendOnmessageMarksActivity(t *testing.T) {
	appJS := fix619Read(t, "static/app.js")
	fix619AssertAll(t, appJS, "static/app.js", []string{
		"sock.onmessage = (m) => {",
		"wsMarkActivity();",
		"startHeartbeat();",
		"stopHeartbeat();",
	})
}

// TestFix619ServerPingHandler asserts the server answers the application-level
// heartbeat, which is what lets the page classify a connection as dead.
func TestFix619ServerPingHandler(t *testing.T) {
	sessionGo := fix619Read(t, "session.go")
	fix619AssertAll(t, sessionGo, "web/session.go", []string{
		`case "ping":`,
		`serverMessage{Kind: "pong"}`,
	})
}

// TestFix619ServerHeartbeatWiring asserts the browser WS connection gets a ping
// timer and a read deadline (C).
func TestFix619ServerHeartbeatWiring(t *testing.T) {
	serverGo := fix619Read(t, "server.go")
	fix619AssertAll(t, serverGo, "web/server.go", []string{
		"wsPingInterval = 30 * time.Second",
		"wsReadTimeout  = 90 * time.Second",
		"c.SetReadTimeout(wsReadTimeout)",
		"stopPing := c.StartPing(wsPingInterval)",
	})
	wsGo := fix619Read(t, "ws.go")
	fix619AssertAll(t, wsGo, "web/ws.go", []string{
		"func (c *wsConn) SetReadTimeout(",
		"func (c *wsConn) refreshReadDeadline()",
		"func (c *wsConn) WritePing()",
		"func (c *wsConn) StartPing(",
		"c.refreshReadDeadline()",
	})
}

// TestFix619IOSRefreshIsThorough asserts the iOS refresh rebuilds the connection
// instead of merely reloading the document, and that it keeps the injected
// access_key cookie alive (A + E).
func TestFix619IOSRefreshIsThorough(t *testing.T) {
	swift := fix619Read(t, "../mobile/ios/Runner/WebViewController.swift")
	fix619AssertAll(t, swift, "WebViewController.swift", []string{
		"func startFreshLoad()",
		"webView?.stopLoading()",
		"URLCache.shared.removeAllCachedResponses()",
		"WKWebsiteDataTypeMemoryCache",
		"WKWebsiteDataTypeDiskCache",
		"cachePolicy: .reloadIgnoringLocalCacheData",
		"private static let retryDelays: [TimeInterval] = [1, 3, 5]",
		"private static let refreshTimeout: TimeInterval = 12",
		"func rebuildWebView()",
		"webView?.removeFromSuperview()",
		"retryLoad(error)",
		// E: the load lifecycle is logged so a failure can be classified later.
		`Logger(subsystem: "com.coshell.mobile", category: "webview")`,
		"func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!)",
		"logLoadFailure(error, phase:",
		`"access_key"`,
	})
	// The cached *content* must go, the credentials must not.
	if strings.Contains(swift, "WKWebsiteDataTypeCookies") {
		t.Error("WebViewController.swift must not clear cookies (the access_key would be lost)")
	}
}
