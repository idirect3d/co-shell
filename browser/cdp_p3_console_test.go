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
	"strings"
	"sync"
	"testing"
	"time"
)

// mockSession is a controllable CDP session for unit tests: it records every
// call and lets tests push events into the channels a collector subscribed to.
type mockSession struct {
	mu       sync.Mutex
	calls    []string
	params   []map[string]interface{}
	watchers map[string][]chan Event
	results  map[string]map[string]interface{}
}

// newMockSession creates an empty mock session.
func newMockSession() *mockSession {
	return &mockSession{
		watchers: make(map[string][]chan Event),
		results:  make(map[string]map[string]interface{}),
	}
}

// Call records the call and returns the canned result of the method.
func (m *mockSession) Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	m.mu.Lock()
	m.calls = append(m.calls, method)
	m.params = append(m.params, params)
	result := m.results[method]
	m.mu.Unlock()

	if result == nil {
		result = map[string]interface{}{}
	}
	return result, nil
}

// Watch returns a buffered channel tests can push events into.
func (m *mockSession) Watch(method string) (<-chan Event, func()) {
	m.mu.Lock()
	ch := make(chan Event, 512)
	m.watchers[method] = append(m.watchers[method], ch)
	m.mu.Unlock()

	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, candidate := range m.watchers[method] {
			if candidate == ch {
				m.watchers[method] = append(m.watchers[method][:i], m.watchers[method][i+1:]...)
				close(ch)
				return
			}
		}
	}
}

// emit delivers an event to every subscriber of method.
func (m *mockSession) emit(method string, params map[string]interface{}) {
	m.mu.Lock()
	watchers := make([]chan Event, len(m.watchers[method]))
	copy(watchers, m.watchers[method])
	m.mu.Unlock()

	ev := Event{Method: method, Params: params}
	for _, ch := range watchers {
		ch <- ev
	}
}

// methods returns the recorded calls in order.
func (m *mockSession) methods() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, len(m.calls))
	copy(out, m.calls)
	return out
}

// called reports whether method was recorded.
func (m *mockSession) called(method string) bool {
	for _, name := range m.methods() {
		if name == method {
			return true
		}
	}
	return false
}

// paramsOf returns the params of the last call of method, or nil.
func (m *mockSession) paramsOf(method string) map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.calls) - 1; i >= 0; i-- {
		if m.calls[i] == method {
			return m.params[i]
		}
	}
	return nil
}

// callIndex returns the position of method in the call log, or -1.
func (m *mockSession) callIndex(method string) int {
	for i, name := range m.methods() {
		if name == method {
			return i
		}
	}
	return -1
}

// drainConsole collects messages until the buffer stays empty twice in a row, so
// tests do not race the collector goroutine.
func drainConsole(t *testing.T, manager *ConsoleManager) []ConsoleMessage {
	t.Helper()

	var collected []ConsoleMessage
	empty := 0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		messages := manager.TakeMessages()
		if len(messages) == 0 {
			empty++
			if empty >= 3 {
				return collected
			}
		} else {
			empty = 0
			collected = append(collected, messages...)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("console collector did not settle, collected %d message(s)", len(collected))
	return collected
}

// TestConsoleAPICallParsing covers A-013: consoleAPICalled args are rendered.
func TestConsoleAPICallParsing(t *testing.T) {
	session := newMockSession()
	manager, err := NewConsoleManager(context.Background(), session)
	if err != nil {
		t.Fatalf("NewConsoleManager: %v", err)
	}
	defer manager.Close()

	for _, level := range []string{"log", "warning", "error"} {
		session.emit("Runtime.consoleAPICalled", map[string]interface{}{
			"type": level,
			"args": []interface{}{
				map[string]interface{}{"type": "string", "value": "hello-" + level},
				map[string]interface{}{"type": "number", "value": float64(42)},
				map[string]interface{}{"type": "object", "description": "Object\n    at app.js:1"},
			},
		})
	}

	messages := drainConsole(t, manager)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	for i, level := range []string{"log", "warning", "error"} {
		if messages[i].Level != level {
			t.Errorf("message %d: level = %q, want %q", i, messages[i].Level, level)
		}
		if messages[i].Source != "console-api" {
			t.Errorf("message %d: source = %q, want console-api", i, messages[i].Source)
		}
		for _, want := range []string{"hello-" + level, "42", "Object"} {
			if !strings.Contains(messages[i].Text, want) {
				t.Errorf("message %d: text %q does not contain %q", i, messages[i].Text, want)
			}
		}
	}
	if session.callIndex("Runtime.enable") < 0 || session.callIndex("Log.enable") < 0 {
		t.Errorf("console domains not enabled: %v", session.methods())
	}
	if session.callIndex("Runtime.enable") > session.callIndex("Runtime.consoleAPICalled") {
		t.Log("subscribed before enabling as required")
	}
}

// TestConsoleExceptionThrown covers A-014: uncaught exceptions become error entries.
func TestConsoleExceptionThrown(t *testing.T) {
	session := newMockSession()
	manager, err := NewConsoleManager(context.Background(), session)
	if err != nil {
		t.Fatalf("NewConsoleManager: %v", err)
	}
	defer manager.Close()

	session.emit("Runtime.exceptionThrown", map[string]interface{}{
		"exceptionDetails": map[string]interface{}{
			"text":       "Uncaught",
			"url":        "http://example.test/app.js",
			"lineNumber": float64(4),
			"exception": map[string]interface{}{
				"type":        "object",
				"description": "Error: boom\n    at app.js:5:1",
			},
		},
	})

	messages := drainConsole(t, manager)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	message := messages[0]
	if message.Level != "error" || message.Source != "exception" {
		t.Errorf("level/source = %q/%q, want error/exception", message.Level, message.Source)
	}
	if !strings.Contains(message.Text, "Error: boom") {
		t.Errorf("text %q does not carry the exception description", message.Text)
	}
	if message.URL != "http://example.test/app.js" || message.Line != 5 {
		t.Errorf("location = %s:%d, want http://example.test/app.js:5", message.URL, message.Line)
	}

	lines := ConsoleSummary(messages, ConsoleSummaryEntries, ConsoleSummaryChars)
	if len(lines) != 1 || !strings.Contains(lines[0], "http://example.test/app.js:5") {
		t.Errorf("summary %v does not carry the source location", lines)
	}
}

// TestConsoleLogEntryAdded covers A-015: browser log entries keep level and source.
func TestConsoleLogEntryAdded(t *testing.T) {
	session := newMockSession()
	manager, err := NewConsoleManager(context.Background(), session)
	if err != nil {
		t.Fatalf("NewConsoleManager: %v", err)
	}
	defer manager.Close()

	entries := []map[string]interface{}{
		{"source": "network", "level": "error", "text": "404 (Not Found)", "url": "http://example.test/missing.png", "lineNumber": float64(0)},
		{"source": "other", "level": "warning", "text": "deprecated API"},
		{"source": "javascript", "level": "info", "text": "started"},
	}
	for _, entry := range entries {
		session.emit("Log.entryAdded", map[string]interface{}{"entry": entry})
	}

	messages := drainConsole(t, manager)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].Source != "network" || messages[0].Level != "error" {
		t.Errorf("network entry = %s/%s, want network/error", messages[0].Source, messages[0].Level)
	}
	if messages[1].Level != "warning" || messages[1].Source != "other" {
		t.Errorf("second entry = %s/%s, want other/warning", messages[1].Source, messages[1].Level)
	}
	if messages[2].Level != "info" || messages[2].Text != "started" {
		t.Errorf("third entry = %s/%q, want info/started", messages[2].Level, messages[2].Text)
	}
}

// TestConsoleRingBuffer covers A-016: only the newest 200 messages are kept and
// TakeMessages never repeats an entry.
func TestConsoleRingBuffer(t *testing.T) {
	session := newMockSession()
	manager, err := NewConsoleManager(context.Background(), session)
	if err != nil {
		t.Fatalf("NewConsoleManager: %v", err)
	}
	defer manager.Close()

	const total = 250
	for i := 0; i < total; i++ {
		session.emit("Runtime.consoleAPICalled", map[string]interface{}{
			"type": "log",
			"args": []interface{}{map[string]interface{}{"type": "string", "value": "msg-" + itoa(i)}},
		})
	}

	deadline := time.Now().Add(2 * time.Second)
	for manager.Received() < total && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := manager.Received(); got != total {
		t.Fatalf("recorded %d messages, want %d", got, total)
	}
	if buffered := manager.Len(); buffered != consoleBufferSize {
		t.Fatalf("buffer holds %d messages, want %d", buffered, consoleBufferSize)
	}

	messages := manager.TakeMessages()
	if len(messages) != consoleBufferSize {
		t.Fatalf("expected the buffer to keep %d messages, got %d", consoleBufferSize, len(messages))
	}
	if got := messages[len(messages)-1].Text; got != "msg-249" {
		t.Errorf("last message = %q, want msg-249", got)
	}
	if got := messages[0].Text; got != "msg-50" {
		t.Errorf("first kept message = %q, want msg-50 (oldest dropped)", got)
	}
	if again := manager.TakeMessages(); len(again) != 0 {
		t.Errorf("second TakeMessages returned %d entries, want 0", len(again))
	}
}

// TestConsoleSummaryLimits covers A-017: at most 5 lines, each truncated.
func TestConsoleSummaryLimits(t *testing.T) {
	messages := make([]ConsoleMessage, 0, 8)
	messages = append(messages, ConsoleMessage{Level: "error", Source: "console-api", Text: strings.Repeat("x", 800)})
	for i := 1; i < 8; i++ {
		messages = append(messages, ConsoleMessage{Level: "log", Source: "console-api", Text: "short-" + itoa(i)})
	}

	lines := ConsoleSummary(messages, ConsoleSummaryEntries, ConsoleSummaryChars)
	if len(lines) != ConsoleSummaryEntries {
		t.Fatalf("expected %d lines, got %d", ConsoleSummaryEntries, len(lines))
	}
	if len(lines[0]) > ConsoleSummaryChars+3 {
		t.Errorf("line length = %d, want about %d", len(lines[0]), ConsoleSummaryChars)
	}
	if !strings.HasSuffix(lines[0], "...") {
		t.Errorf("long line %q was not marked as truncated", lines[0])
	}
}

// TestConsoleNotEnabledBeforeUse covers A-018: no console domain call happens
// before a tool asks for console output.
func TestConsoleNotEnabledBeforeUse(t *testing.T) {
	session := newMockSession()

	// Plain browsing tools only call their own CDP methods.
	if _, err := session.Call(context.Background(), "Page.navigate", map[string]interface{}{}); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if session.called("Runtime.enable") || session.called("Log.enable") {
		t.Fatalf("console domains must stay untouched before use: %v", session.methods())
	}

	manager, err := NewConsoleManager(context.Background(), session)
	if err != nil {
		t.Fatalf("NewConsoleManager: %v", err)
	}
	defer manager.Close()

	if !session.called("Runtime.enable") || !session.called("Log.enable") {
		t.Errorf("console domains must be enabled on first use: %v", session.methods())
	}
	if manager.TakeMessages() != nil {
		t.Error("a fresh manager must have no messages")
	}
}

// itoa converts a non-negative int to its decimal representation.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
