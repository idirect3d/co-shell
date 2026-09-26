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

// Network observation built on the Network domain: requestWillBeSent /
// responseReceived / loadingFailed are folded into one bounded list of
// request entries, so a tool can report what a page actually loaded (and what
// failed) without opening DevTools.
//
// The Network domain is enabled on demand only: plain browsing issues no
// Network.enable call (FEATURE-556 Phase 3). Unlike the console domain, CDP
// does not replay network events, so observation must be enabled before the
// traffic of interest happens.

package browser

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// networkBufferSize bounds the observed requests per session; the oldest
	// entries are dropped first.
	networkBufferSize = 300
	// NetworkDefaultLimit is used when a caller asks for no explicit limit.
	NetworkDefaultLimit = 20
	// NetworkMaxLimit caps how many entries one query may return.
	NetworkMaxLimit = 100
	// NetworkSummaryEntries and NetworkSummaryChars limit how many failures a
	// single tool result lists and how long each line may be.
	NetworkSummaryEntries = 3
	NetworkSummaryChars   = 200
	// networkEnableTimeout bounds the Network.enable call.
	networkEnableTimeout = 5 * time.Second
)

// NetworkEntry describes one observed request and its outcome.
type NetworkEntry struct {
	RequestID string
	URL       string
	Method    string
	Type      string // resource type: Document/Script/Image/XHR/...
	Status    int
	MIMEType  string
	FromCache bool
	Failed    bool
	ErrorText string
	At        time.Time
}

// NetworkCollector collects the network activity of one CDP session.
type NetworkCollector struct {
	session eventSession
	cancels []func()

	mu       sync.Mutex
	entries  []NetworkEntry
	received int
}

// NewNetworkCollector enables Network reporting on session and starts
// collecting requestWillBeSent, responseReceived and loadingFailed. The
// collector is always usable; a non-nil error only reports that the domain
// could not be enabled.
func NewNetworkCollector(ctx context.Context, session eventSession) (*NetworkCollector, error) {
	collector := &NetworkCollector{session: session}

	// Subscribe before enabling so no event of the enabled domain is missed.
	for _, method := range []string{"Network.requestWillBeSent", "Network.responseReceived", "Network.loadingFailed"} {
		events, cancel := session.Watch(method)
		collector.cancels = append(collector.cancels, cancel)
		go collector.loop(events)
	}

	enableCtx, cancel := context.WithTimeout(ctx, networkEnableTimeout)
	defer cancel()

	if _, err := session.Call(enableCtx, "Network.enable", map[string]interface{}{}); err != nil {
		return collector, fmt.Errorf("cannot enable network observation: %w", err)
	}
	return collector, nil
}

// Close stops collecting network activity.
func (c *NetworkCollector) Close() {
	if c == nil {
		return
	}
	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = nil
}

// Entries returns a snapshot of the collected requests (oldest first), keeping
// only failures when onlyFailures is set and at most limit entries. A limit
// <= 0 means NetworkDefaultLimit; the result never exceeds NetworkMaxLimit.
func (c *NetworkCollector) Entries(onlyFailures bool, limit int) []NetworkEntry {
	if c == nil {
		return nil
	}
	if limit <= 0 {
		limit = NetworkDefaultLimit
	}
	if limit > NetworkMaxLimit {
		limit = NetworkMaxLimit
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]NetworkEntry, 0, limit)
	for _, entry := range c.entries {
		if onlyFailures && !entry.Failed {
			continue
		}
		out = append(out, entry)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Count returns the number of buffered entries.
func (c *NetworkCollector) Count() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Received returns the total number of observed requests, including entries
// already dropped from the bounded buffer.
func (c *NetworkCollector) Received() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.received
}

// loop records events until the subscription closes.
func (c *NetworkCollector) loop(events <-chan Event) {
	for ev := range events {
		c.record(ev)
	}
}

// record folds one CDP network event into the entry list.
func (c *NetworkCollector) record(ev Event) {
	requestID := eventString(ev.Params, "requestId")
	if requestID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch ev.Method {
	case "Network.requestWillBeSent":
		request, _ := ev.Params["request"].(map[string]interface{})
		entry := c.ensureLocked(requestID)
		if url := eventString(request, "url"); url != "" {
			entry.URL = url
		}
		if method := eventString(request, "method"); method != "" {
			entry.Method = method
		}
		if resourceType := eventString(ev.Params, "type"); resourceType != "" {
			entry.Type = resourceType
		}

	case "Network.responseReceived":
		response, _ := ev.Params["response"].(map[string]interface{})
		entry := c.ensureLocked(requestID)
		if url := eventString(response, "url"); url != "" {
			entry.URL = url
		}
		entry.Status = int(jsonFloat(response["status"]))
		entry.MIMEType = eventString(response, "mimeType")
		entry.FromCache = boolField(response, "fromDiskCache") ||
			boolField(response, "fromServiceWorker") ||
			boolField(response, "fromPrefetchCache")
		if resourceType := eventString(ev.Params, "type"); resourceType != "" {
			entry.Type = resourceType
		}

	case "Network.loadingFailed":
		entry := c.ensureLocked(requestID)
		entry.Failed = true
		reason := eventString(ev.Params, "errorText")
		if blocked := eventString(ev.Params, "blockedReason"); blocked != "" {
			if reason == "" {
				reason = "blocked: " + blocked
			} else {
				reason += " (blocked: " + blocked + ")"
			}
		}
		if reason == "" {
			reason = "unknown error"
		}
		entry.ErrorText = reason
		if boolField(ev.Params, "canceled") && reason == "unknown error" {
			entry.ErrorText = "canceled"
		}
		if resourceType := eventString(ev.Params, "type"); resourceType != "" {
			entry.Type = resourceType
		}
	}
}

// ensureLocked returns the entry of requestID, creating it when the request was
// not seen before (a failure may be reported without a preceding request event
// when observation started mid-flight).
func (c *NetworkCollector) ensureLocked(requestID string) *NetworkEntry {
	for i := range c.entries {
		if c.entries[i].RequestID == requestID {
			return &c.entries[i]
		}
	}

	c.received++
	c.entries = append(c.entries, NetworkEntry{RequestID: requestID, At: time.Now()})
	if len(c.entries) > networkBufferSize {
		c.entries = c.entries[len(c.entries)-networkBufferSize:]
	}
	return &c.entries[len(c.entries)-1]
}

// NetworkSummary renders at most maxEntries entries as single lines, each
// truncated to maxChars.
func NetworkSummary(entries []NetworkEntry, maxEntries, maxChars int) []string {
	if maxEntries <= 0 || maxChars <= 0 {
		return nil
	}
	lines := make([]string, 0, len(entries))
	for i, entry := range entries {
		if i >= maxEntries {
			break
		}
		lines = append(lines, truncateString(networkEntryLine(entry), maxChars))
	}
	return lines
}

// NetworkFailureLines renders at most maxEntries failures (in observation
// order, each truncated to maxChars) and reports how many further failures were
// not listed.
func NetworkFailureLines(entries []NetworkEntry, maxEntries, maxChars int) ([]string, int) {
	if maxEntries <= 0 || maxChars <= 0 {
		return nil, 0
	}

	var lines []string
	total := 0
	for _, entry := range entries {
		if !entry.Failed {
			continue
		}
		total++
		if len(lines) < maxEntries {
			lines = append(lines, truncateString(networkEntryLine(entry), maxChars))
		}
	}
	if len(lines) == 0 {
		return nil, 0
	}
	return lines, total - len(lines)
}

// networkEntryLine renders one entry as a single line.
func networkEntryLine(entry NetworkEntry) string {
	method := entry.Method
	if method == "" {
		method = "GET"
	}
	line := fmt.Sprintf("%s %s", method, entry.URL)
	switch {
	case entry.Failed:
		line += " -> FAILED: " + entry.ErrorText
	case entry.Status > 0:
		line += fmt.Sprintf(" -> %d", entry.Status)
		if entry.MIMEType != "" {
			line += " " + entry.MIMEType
		}
	}
	if entry.Type != "" {
		line += " (" + entry.Type + ")"
	}
	if entry.FromCache {
		line += " [cache]"
	}
	return line
}

// boolField reads a boolean field from a CDP payload.
func boolField(params map[string]interface{}, key string) bool {
	value, _ := params[key].(bool)
	return value
}
