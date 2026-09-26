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
	"os"
	"testing"
	"time"
)

// TestIntegrationDialogPolicyFromConfig covers B-018: the dialog policy taken
// from ChromeConfig (the browser-dialog-policy setting) decides how a page's
// JavaScript dialogs are answered, and a dialog never blocks later CDP work.
//
// auto    -> alert accepted, confirm answered false (page shows "cancelled")
// accept  -> alert accepted, confirm answered true (page shows "confirmed")
// dismiss -> alert dismissed, confirm answered false (page shows "cancelled")
func TestIntegrationDialogPolicyFromConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	cases := []struct {
		policy          string
		wantConfirmText string
	}{
		{policy: "auto", wantConfirmText: "cancelled"},
		{policy: "accept", wantConfirmText: "confirmed"},
		{policy: "dismiss", wantConfirmText: "cancelled"},
	}

	for _, tc := range cases {
		t.Run(tc.policy, func(t *testing.T) {
			manager, client := uc556PolicySession(t, tc.policy)
			server := uc556StartServer(t).URL

			// confirm.html writes the answer into #out, so the page itself
			// reports which branch the dialog manager chose.
			uc556Navigate(t, client, server+"/confirm.html")
			if got := uc556Eval(t, client, "document.querySelector('#out').textContent"); got != tc.wantConfirmText {
				t.Fatalf("dialog-policy=%s: confirm answered %q, want %q", tc.policy, got, tc.wantConfirmText)
			}

			// alert.html fires an alert while the document is still parsing: the
			// dialog must be answered automatically, otherwise this navigate and
			// the evaluate below would hang.
			uc556Navigate(t, client, server+"/alert.html")
			if got := uc556Eval(t, client, "document.querySelector('#ready').textContent"); got != "ready" {
				t.Fatalf("dialog-policy=%s: page blocked by alert (ready=%q)", tc.policy, got)
			}
			if got := uc556Eval(t, client, "String(document.readyState)"); got != "complete" {
				t.Fatalf("dialog-policy=%s: readyState=%q, want complete", tc.policy, got)
			}

			// TakeDialogMessages drains the queue (Descriptions() does the same),
			// so it is the only reader here.
			handled := manager.TakeDialogMessages()
			if len(handled) != 2 {
				t.Fatalf("dialog-policy=%s: handled %d dialogs, want 2 (confirm + alert)", tc.policy, len(handled))
			}
			t.Logf("dialog-policy=%s handled: %v", tc.policy, handled)
		})
	}
}

// uc556PolicySession starts a dedicated headless Chrome configured with the
// requested dialog policy, which is the path the agent uses (config ->
// ChromeConfig -> attachSession -> DialogManager).
func uc556PolicySession(t *testing.T, policy string) (*ChromeManager, *CDPClient) {
	t.Helper()

	profile, err := os.MkdirTemp("", "co-shell-uc556-dialog-profile-")
	if err != nil {
		t.Fatalf("cannot create temp profile: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(profile) })

	port, err := uc556FreePort()
	if err != nil {
		t.Fatalf("cannot reserve debug port: %v", err)
	}

	cfg := DefaultChromeConfig()
	cfg.ProfileDir = profile
	cfg.DialogPolicy = policy

	manager := NewChromeManagerWithConfig(port, true, cfg)
	t.Cleanup(func() { manager.Stop() })

	if _, err := manager.Start(); err != nil {
		t.Fatalf("cannot start Chrome: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := manager.EnsurePageConnected(ctx); err != nil {
		t.Fatalf("cannot connect to Chrome: %v", err)
	}

	client := manager.Client()
	if client == nil {
		t.Fatal("Chrome manager has no CDP client")
	}
	return manager, client
}
