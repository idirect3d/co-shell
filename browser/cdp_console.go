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

// Console capture built on the Runtime and Log domains: JavaScript console
// output, uncaught exceptions and browser log entries are buffered per session
// and consumed by the browser_console tool.
//
// Capture is enabled on demand: nothing is sent to Chrome until a tool asks for
// console output, so plain browsing issues no Runtime.enable/Log.enable call
// (FEATURE-556 Phase 3). Chrome replays the messages a page produced before the
// domains were enabled — the same behaviour that lets DevTools show console
// output of an already loaded page — so the first request still sees them.

package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// consoleBufferSize bounds the collected messages per session; the oldest
	// entries are dropped first.
	consoleBufferSize = 200
	// consoleEnableTimeout bounds the Runtime.enable / Log.enable calls.
	consoleEnableTimeout = 5 * time.Second
	// ConsoleSummaryEntries and ConsoleSummaryChars limit how much console
	// output a single tool result carries.
	ConsoleSummaryEntries = 5
	ConsoleSummaryChars   = 200
)

// ConsoleMessage is one collected console entry.
type ConsoleMessage struct {
	Level  string // log/info/warning/error, as reported by CDP
	Source string // console-api/exception/network/...
	Text   string
	URL    string
	Line   int // 1-based line, 0 when unknown
	At     time.Time
}

// eventSession is the subset of *CDPClient an event collector needs: Call to
// enable a domain and Watch to subscribe to its events.
type eventSession interface {
	Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error)
	Watch(method string) (<-chan Event, func())
}

// ConsoleManager collects the console messages of one CDP session.
type ConsoleManager struct {
	session eventSession
	cancels []func()

	mu       sync.Mutex
	entries  []ConsoleMessage
	received int
}

// NewConsoleManager enables console reporting on session and starts collecting
// Runtime.consoleAPICalled, Runtime.exceptionThrown and Log.entryAdded.
// The manager is always usable; a non-nil error only reports which domains
// could not be enabled.
func NewConsoleManager(ctx context.Context, session eventSession) (*ConsoleManager, error) {
	manager := &ConsoleManager{session: session}

	// Subscribe before enabling: CDP never replays events that happened before
	// a subscription exists.
	for _, method := range []string{"Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Log.entryAdded"} {
		events, cancel := session.Watch(method)
		manager.cancels = append(manager.cancels, cancel)
		go manager.loop(events)
	}

	enableCtx, cancel := context.WithTimeout(ctx, consoleEnableTimeout)
	defer cancel()

	var failed []string
	for _, domain := range []string{"Runtime.enable", "Log.enable"} {
		if _, err := session.Call(enableCtx, domain, map[string]interface{}{}); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", domain, err))
		}
	}
	if len(failed) > 0 {
		return manager, fmt.Errorf("cannot enable console capture (%s)", strings.Join(failed, "; "))
	}
	return manager, nil
}

// Close stops collecting console messages.
func (m *ConsoleManager) Close() {
	if m == nil {
		return
	}
	for _, cancel := range m.cancels {
		cancel()
	}
	m.cancels = nil
}

// TakeMessages returns the messages collected so far and clears the buffer, so
// a message is only reported once.
func (m *ConsoleManager) TakeMessages() []ConsoleMessage {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.entries) == 0 {
		return nil
	}
	out := m.entries
	m.entries = nil
	return out
}

// Len returns the number of buffered messages.
func (m *ConsoleManager) Len() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// Received returns the total number of messages recorded, including the ones
// already dropped from the bounded buffer.
func (m *ConsoleManager) Received() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.received
}

// loop records events until the subscription closes.
func (m *ConsoleManager) loop(events <-chan Event) {
	for ev := range events {
		m.record(ev)
	}
}

// record converts one CDP event into a console message.
func (m *ConsoleManager) record(ev Event) {
	var message ConsoleMessage
	switch ev.Method {
	case "Runtime.consoleAPICalled":
		message = consoleMessageFromAPICall(ev.Params)
	case "Runtime.exceptionThrown":
		message = consoleMessageFromException(ev.Params)
	case "Log.entryAdded":
		message = consoleMessageFromLogEntry(ev.Params)
	default:
		return
	}
	message.At = time.Now()
	m.append(message)
}

// append adds a message, keeping only the newest consoleBufferSize entries.
func (m *ConsoleManager) append(message ConsoleMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.received++
	m.entries = append(m.entries, message)
	if len(m.entries) > consoleBufferSize {
		m.entries = m.entries[len(m.entries)-consoleBufferSize:]
	}
}

// ConsoleSummary renders at most maxEntries messages as single lines, each
// truncated to maxChars (with an ellipsis marker when truncated).
func ConsoleSummary(entries []ConsoleMessage, maxEntries, maxChars int) []string {
	if maxEntries <= 0 || maxChars <= 0 {
		return nil
	}
	lines := make([]string, 0, len(entries))
	for i, entry := range entries {
		if i >= maxEntries {
			break
		}
		source := entry.Source
		if source == "" {
			source = "log"
		}
		line := fmt.Sprintf("[%s/%s] %s", entry.Level, source, entry.Text)
		if entry.URL != "" {
			line += fmt.Sprintf(" (%s:%d)", entry.URL, entry.Line)
		}
		lines = append(lines, truncateString(strings.TrimSpace(line), maxChars))
	}
	return lines
}

// consoleMessageFromAPICall converts Runtime.consoleAPICalled params.
func consoleMessageFromAPICall(params map[string]interface{}) ConsoleMessage {
	level := eventString(params, "type")
	if level == "" {
		level = "log"
	}

	text := ""
	if args, ok := params["args"].([]interface{}); ok {
		parts := make([]string, 0, len(args))
		for _, arg := range args {
			parts = append(parts, consoleArgText(arg))
		}
		text = strings.Join(parts, " ")
	}

	url, line := consoleLocation(params["stackTrace"])
	return ConsoleMessage{Level: level, Source: "console-api", Text: text, URL: url, Line: line}
}

// consoleMessageFromException converts Runtime.exceptionThrown params.
func consoleMessageFromException(params map[string]interface{}) ConsoleMessage {
	details, _ := params["exceptionDetails"].(map[string]interface{})
	if details == nil {
		details = map[string]interface{}{}
	}

	text := eventString(details, "text")
	if exception, ok := details["exception"].(map[string]interface{}); ok {
		if description := eventString(exception, "description"); description != "" {
			if text == "" {
				text = firstLine(description)
			} else {
				text = text + ": " + firstLine(description)
			}
		}
	}
	if text == "" {
		text = "uncaught exception"
	}

	return ConsoleMessage{
		Level:  "error",
		Source: "exception",
		Text:   text,
		URL:    eventString(details, "url"),
		Line:   consoleLineNumber(details["lineNumber"]),
	}
}

// consoleMessageFromLogEntry converts Log.entryAdded params.
func consoleMessageFromLogEntry(params map[string]interface{}) ConsoleMessage {
	entry, _ := params["entry"].(map[string]interface{})
	if entry == nil {
		return ConsoleMessage{Level: "info", Source: "log"}
	}

	level := eventString(entry, "level")
	if level == "" {
		level = "info"
	}
	source := eventString(entry, "source")
	if source == "" {
		source = "log"
	}

	return ConsoleMessage{
		Level:  level,
		Source: source,
		Text:   eventString(entry, "text"),
		URL:    eventString(entry, "url"),
		Line:   consoleLineNumber(entry["lineNumber"]),
	}
}

// consoleLocation reads the first call frame of a stack trace.
func consoleLocation(stack interface{}) (string, int) {
	trace, ok := stack.(map[string]interface{})
	if !ok {
		return "", 0
	}
	frames, ok := trace["callFrames"].([]interface{})
	if !ok || len(frames) == 0 {
		return "", 0
	}
	frame, ok := frames[0].(map[string]interface{})
	if !ok {
		return "", 0
	}
	return eventString(frame, "url"), consoleLineNumber(frame["lineNumber"])
}

// consoleLineNumber converts CDP's 0-based line number into a 1-based one
// (0 means "unknown").
func consoleLineNumber(value interface{}) int {
	line := int(jsonFloat(value)) + 1
	if line < 1 {
		return 0
	}
	return line
}

// consoleArgText renders one argument of a console call as text: primitives
// verbatim, remote objects via their description (falling back to their type).
func consoleArgText(arg interface{}) string {
	object, ok := arg.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", arg)
	}
	if value, exists := object["value"]; exists {
		if value == nil {
			return "null"
		}
		return fmt.Sprintf("%v", value)
	}
	if subtype, _ := object["subtype"].(string); subtype == "null" {
		return "null"
	}
	if raw, _ := object["unserializableValue"].(string); raw != "" {
		return raw
	}
	if description, _ := object["description"].(string); description != "" {
		return firstLine(description)
	}
	if kind, _ := object["type"].(string); kind != "" {
		return "<" + kind + ">"
	}
	return "?"
}

// firstLine returns the first line of s, trimmed.
func firstLine(s string) string {
	if index := strings.IndexByte(s, '\n'); index >= 0 {
		s = s[:index]
	}
	return strings.TrimSpace(s)
}
