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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// waitNetwork polls the collector until the entry of requestID satisfies ready.
func waitNetwork(t *testing.T, collector *NetworkCollector, requestID string, ready func(NetworkEntry) bool) NetworkEntry {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, entry := range collector.Entries(false, NetworkMaxLimit) {
			if entry.RequestID == requestID && ready(entry) {
				return entry
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("network entry %s did not reach the expected state", requestID)
	return NetworkEntry{}
}

// waitNetworkCount polls the collector until it holds want entries.
func waitNetworkCount(t *testing.T, collector *NetworkCollector, want int) []NetworkEntry {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries := collector.Entries(false, NetworkMaxLimit)
		if len(entries) >= want {
			return entries
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("observed %d entries, want %d", len(collector.Entries(false, NetworkMaxLimit)), want)
	return nil
}

// request id are folded into one entry.
func TestNetworkRequestResponseMerge(t *testing.T) {
	session := newMockSession()
	collector, err := NewNetworkCollector(context.Background(), session)
	if err != nil {
		t.Fatalf("NewNetworkCollector: %v", err)
	}
	defer collector.Close()

	session.emit("Network.requestWillBeSent", map[string]interface{}{
		"requestId": "R1",
		"type":      "Image",
		"request":   map[string]interface{}{"url": "http://example.test/ok.png", "method": "GET"},
	})
	session.emit("Network.responseReceived", map[string]interface{}{
		"requestId": "R1",
		"type":      "Image",
		"response": map[string]interface{}{
			"status":        float64(200),
			"mimeType":      "image/png",
			"fromDiskCache": true,
		},
	})

	entry := waitNetwork(t, collector, "R1", func(entry NetworkEntry) bool { return entry.Status == 200 })
	if entry.MIMEType != "image/png" || entry.Type != "Image" {
		t.Errorf("mimeType/type = %q/%q, want image/png/Image", entry.MIMEType, entry.Type)
	}
	if entry.URL != "http://example.test/ok.png" || entry.Method != "GET" {
		t.Errorf("url/method = %q/%q", entry.URL, entry.Method)
	}
	if !entry.FromCache {
		t.Error("fromDiskCache must be reflected")
	}
	if entry.Failed {
		t.Error("a 200 response must not be marked as failed")
	}
	if entries := collector.Entries(false, NetworkMaxLimit); len(entries) != 1 {
		t.Errorf("request and response must be merged into one entry, got %d", len(entries))
	}
	if !session.called("Network.enable") {
		t.Errorf("network observation must enable the domain: %v", session.methods())
	}
}

// TestNetworkLoadingFailed covers A-027: failures keep url/type and carry a reason.
func TestNetworkLoadingFailed(t *testing.T) {
	session := newMockSession()
	collector, err := NewNetworkCollector(context.Background(), session)
	if err != nil {
		t.Fatalf("NewNetworkCollector: %v", err)
	}
	defer collector.Close()

	session.emit("Network.requestWillBeSent", map[string]interface{}{
		"requestId": "R2",
		"type":      "Script",
		"request":   map[string]interface{}{"url": "http://missing.test/app.js", "method": "GET"},
	})
	session.emit("Network.loadingFailed", map[string]interface{}{
		"requestId": "R2",
		"type":      "Script",
		"errorText": "net::ERR_NAME_NOT_RESOLVED",
		"canceled":  true,
	})

	entry := waitNetwork(t, collector, "R2", func(entry NetworkEntry) bool { return entry.Failed })
	if !strings.Contains(entry.ErrorText, "net::ERR_NAME_NOT_RESOLVED") {
		t.Errorf("errorText = %q, want the CDP failure reason", entry.ErrorText)
	}
	if entry.URL != "http://missing.test/app.js" || entry.Type != "Script" {
		t.Errorf("url/type = %q/%q, want them preserved", entry.URL, entry.Type)
	}
}

// TestNetworkFilterAndLimit covers A-028: bounded buffer, failure filter, limit.
func TestNetworkFilterAndLimit(t *testing.T) {
	session := newMockSession()
	collector, err := NewNetworkCollector(context.Background(), session)
	if err != nil {
		t.Fatalf("NewNetworkCollector: %v", err)
	}
	defer collector.Close()

	for i := 0; i < networkBufferSize+50; i++ {
		session.emit("Network.requestWillBeSent", map[string]interface{}{
			"requestId": fmt.Sprintf("R%d", i),
			"type":      "Document",
			"request":   map[string]interface{}{"url": fmt.Sprintf("http://example.test/%d", i), "method": "GET"},
		})
	}

	deadline := time.Now().Add(2 * time.Second)
	for collector.Received() < networkBufferSize+50 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := collector.Received(); got != networkBufferSize+50 {
		t.Fatalf("observed %d requests, want %d", got, networkBufferSize+50)
	}
	if buffered := collector.Count(); buffered != networkBufferSize {
		t.Fatalf("buffer holds %d entries, want %d", buffered, networkBufferSize)
	}

	entries := collector.Entries(false, NetworkMaxLimit)
	if len(entries) == 0 || entries[0].RequestID != "R50" {
		t.Errorf("oldest buffered entry = %v, want R50 (older entries dropped)", entries)
	}

	// A second collector isolates filter and limit behaviour.
	limitedSession := newMockSession()
	limited, err := NewNetworkCollector(context.Background(), limitedSession)
	if err != nil {
		t.Fatalf("NewNetworkCollector: %v", err)
	}
	defer limited.Close()

	for i := 0; i < 3; i++ {
		limitedSession.emit("Network.requestWillBeSent", map[string]interface{}{
			"requestId": fmt.Sprintf("L%d", i),
			"request":   map[string]interface{}{"url": fmt.Sprintf("http://example.test/l%d", i), "method": "GET"},
		})
	}
	limitedSession.emit("Network.loadingFailed", map[string]interface{}{"requestId": "L0", "errorText": "net::ERR_FAILED"})
	limitedSession.emit("Network.loadingFailed", map[string]interface{}{"requestId": "L2", "errorText": "net::ERR_ABORTED"})

	waitNetworkCount(t, limited, 3)
	failuresDeadline := time.Now().Add(2 * time.Second)
	for len(limited.Entries(true, 0)) < 2 && time.Now().Before(failuresDeadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if failures := limited.Entries(true, 0); len(failures) != 2 {
		t.Errorf("only_failures returned %d entries, want 2", len(failures))
	}
	if one := limited.Entries(false, 1); len(one) != 1 {
		t.Errorf("limit 1 returned %d entries, want 1", len(one))
	}
	if many := limited.Entries(false, 500); len(many) != 3 {
		t.Errorf("entries = %d, want 3 (limit capped, not exceeded)", len(many))
	}
}

// TestNetworkFailureLines covers A-029: at most three failure lines are listed.
func TestNetworkFailureLines(t *testing.T) {
	session := newMockSession()
	collector, err := NewNetworkCollector(context.Background(), session)
	if err != nil {
		t.Fatalf("NewNetworkCollector: %v", err)
	}
	defer collector.Close()

	for i := 0; i < 6; i++ {
		session.emit("Network.requestWillBeSent", map[string]interface{}{
			"requestId": fmt.Sprintf("F%d", i),
			"type":      "Image",
			"request":   map[string]interface{}{"url": fmt.Sprintf("http://example.test/f%d.png", i), "method": "GET"},
		})
		session.emit("Network.loadingFailed", map[string]interface{}{
			"requestId": fmt.Sprintf("F%d", i),
			"errorText": "net::ERR_FAILED",
		})
	}

	entries := waitNetworkReady(t, collector, 6, func(entry NetworkEntry) bool {
		return entry.Failed && entry.URL != ""
	})
	lines, omitted := NetworkFailureLines(entries, NetworkSummaryEntries, NetworkSummaryChars)
	if len(lines) != NetworkSummaryEntries {
		t.Fatalf("listed %d failure lines, want %d", len(lines), NetworkSummaryEntries)
	}
	if omitted != 3 {
		t.Errorf("omitted = %d, want 3", omitted)
	}
	if !strings.Contains(lines[0], "f0.png") || !strings.Contains(lines[0], "net::ERR_FAILED") {
		t.Errorf("first line = %q, want url and reason", lines[0])
	}
}

// TestTargetParsingAndCurrent covers A-030: only pages are listed and the current
// page is marked once.
func TestTargetParsingAndCurrent(t *testing.T) {
	session := newMockSession()
	session.results["Target.getTargets"] = map[string]interface{}{
		"targetInfos": []interface{}{
			map[string]interface{}{"targetId": "T1", "type": "page", "title": "One", "url": "http://one.test/"},
			map[string]interface{}{"targetId": "T2", "type": "page", "title": "Two", "url": "http://two.test/"},
			map[string]interface{}{"targetId": "S1", "type": "service_worker", "title": "", "url": "http://one.test/sw.js"},
		},
	}

	pages, err := ListPageTargets(context.Background(), session)
	if err != nil {
		t.Fatalf("ListPageTargets: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("listed %d page targets, want 2", len(pages))
	}
	if pages[0].TargetID != "T1" || pages[0].URL != "http://one.test/" || pages[0].Title != "One" {
		t.Errorf("first page = %+v", pages[0])
	}

	methods := session.methods()
	if len(methods) != 2 || methods[0] != "Target.setDiscoverTargets" || methods[1] != "Target.getTargets" {
		t.Errorf("calls = %v, want discovery then listing", methods)
	}
	if params := session.paramsOf("Target.setDiscoverTargets"); params["discover"] != true {
		t.Errorf("discover = %v, want true", params["discover"])
	}

	if index := CurrentTargetIndex(pages, "http://two.test/", ""); index != 1 {
		t.Errorf("current index = %d, want 1", index)
	}
	if index := CurrentTargetIndex(pages, "http://one.test/", ""); index != 0 {
		t.Errorf("current index = %d, want the first match (0)", index)
	}
	if index := CurrentTargetIndex(pages, "http://missing.test/", "One"); index != 0 {
		t.Errorf("title fallback index = %d, want 0", index)
	}
	if index := CurrentTargetIndex(pages, "", ""); index != -1 {
		t.Errorf("index = %d, want -1 when nothing matches", index)
	}
}

// TestSelectTargetErrors covers A-031: clear errors describe what is available.
func TestSelectTargetErrors(t *testing.T) {
	pages := []TargetInfo{
		{TargetID: "T1", Type: "page", URL: "http://one.test/"},
		{TargetID: "T2", Type: "page", URL: "http://two.test/"},
	}

	if _, err := SelectTarget(pages, 5, ""); err == nil || !strings.Contains(err.Error(), "1..2") {
		t.Errorf("out of range error = %v, want the available range", err)
	}
	if _, err := SelectTarget(pages, 0, "nope"); err == nil || !strings.Contains(err.Error(), "T1") {
		t.Errorf("unknown id error = %v, want the available ids", err)
	}
	if _, err := SelectTarget(pages, 0, ""); err == nil {
		t.Error("a missing index and target id must fail")
	}
	if target, err := SelectTarget(pages, 2, ""); err != nil || target.TargetID != "T2" {
		t.Errorf("1-based index selection failed: %+v, %v", target, err)
	}
	if target, err := SelectTarget(pages, 0, "T1"); err != nil || target.URL != "http://one.test/" {
		t.Errorf("target id selection failed: %+v, %v", target, err)
	}
}

// TestSwitchTargetURL covers A-032: switching needs a WebSocket debugger URL.
func TestSwitchTargetURL(t *testing.T) {
	url, err := SwitchTargetURL(TargetInfo{TargetID: "T2", WSURL: "ws://127.0.0.1/devtools/page/T2"})
	if err != nil || url != "ws://127.0.0.1/devtools/page/T2" {
		t.Fatalf("SwitchTargetURL = %q, %v", url, err)
	}
	if _, err := SwitchTargetURL(TargetInfo{TargetID: "T3"}); err == nil || !strings.Contains(err.Error(), "webSocketDebuggerUrl") {
		t.Errorf("error = %v, want a missing debugger URL error", err)
	}
}

// TestDebugTargetsAndAttach covers the /json/list lookup used to switch pages.
func TestDebugTargetsAndAttach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"T2","type":"page","title":"Two","url":"http://two.test/","webSocketDebuggerUrl":"ws://127.0.0.1:9/devtools/page/T2"}]`)
	}))
	defer server.Close()

	debugTargets, err := DebugTargets(server.URL)
	if err != nil {
		t.Fatalf("DebugTargets: %v", err)
	}
	if len(debugTargets) != 1 || debugTargets[0].WSURL == "" {
		t.Fatalf("debug targets = %+v, want one target with a WS URL", debugTargets)
	}

	pages := AttachDebugURLs([]TargetInfo{{TargetID: "T1"}, {TargetID: "T2"}}, debugTargets)
	if pages[0].WSURL != "" {
		t.Errorf("unmatched target gained %q", pages[0].WSURL)
	}
	if pages[1].WSURL != "ws://127.0.0.1:9/devtools/page/T2" {
		t.Errorf("matched target WS URL = %q", pages[1].WSURL)
	}

	if _, err := DebugTargets("http://127.0.0.1:1"); err == nil {
		t.Error("an unreachable debug endpoint must fail")
	}
}

// waitNetworkReady polls the collector until every observed entry satisfies
// ready: events of different methods are handled by independent goroutines, so
// a snapshot may briefly hold an entry whose request data is still pending.
func waitNetworkReady(t *testing.T, collector *NetworkCollector, want int, ready func(NetworkEntry) bool) []NetworkEntry {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries := collector.Entries(false, NetworkMaxLimit)
		if len(entries) >= want {
			complete := true
			for _, entry := range entries {
				if !ready(entry) {
					complete = false
					break
				}
			}
			if complete {
				return entries
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("network entries did not reach the expected state: %v", collector.Entries(false, NetworkMaxLimit))
	return nil
}
