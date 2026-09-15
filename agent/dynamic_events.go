// Author: L.Shuang
// Created: 2026-09-04
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

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DynamicEventKind enumerates the user-action event categories tracked by the
// dynamic perception queue (FEATURE-471).
type DynamicEventKind string

const (
	// DynamicClipObject: a pasted screenshot / clipboard image uploaded to the
	// workspace (e.g. input/clip-*.png).
	DynamicClipObject DynamicEventKind = "clip_object"
	// DynamicUploadFile: a regular file uploaded to the workspace.
	DynamicUploadFile DynamicEventKind = "upload_file"
	// DynamicUserMessage: a message the user typed while a task was running.
	DynamicUserMessage DynamicEventKind = "user_message"
	// DynamicOpenFile: a file the user opened / revealed from the Web UI.
	DynamicOpenFile DynamicEventKind = "open_file"
	// DynamicViewFile: a file the user single-clicked to preview in-page.
	DynamicViewFile DynamicEventKind = "view_file"
	// DynamicBoardRequest: a new help request appeared on the hub bulletin
	// board (FEATURE-490). Text carries the request summary JSON.
	DynamicBoardRequest DynamicEventKind = "board_request"
	// DynamicBoardDM: a direct message arrived on a board thread (FEATURE-490).
	DynamicBoardDM DynamicEventKind = "board_dm"
	// DynamicBoardNotify: a board state change notification (claimed/result)
	// (FEATURE-490).
	DynamicBoardNotify DynamicEventKind = "board_notify"

	// DynamicUIAction: a component interaction (form submit, button, chart
	// drill-down) reported while a turn is running (FEATURE-524 window mode).
	// It is injected into the running turn instead of starting a new one, so the
	// LLM sees the user's action right before its next call.
	DynamicUIAction DynamicEventKind = "ui_action"
)

// DynamicEvent is one user-action event buffered in the dynamic perception
// queue. Path events (clip/upload/open) carry a workspace-relative path whose
// size and mtime are resolved at injection time; message events carry the raw
// text the user typed.
type DynamicEvent struct {
	Kind DynamicEventKind
	// Path is the workspace-relative file path for clip/upload/open events.
	Path string
	// Text is the user-typed message for user_message events.
	Text string
	// Time is when the event occurred (RFC3339).
	Time time.Time
}

// dynamicEventQueue is the thread-safe FIFO queue of pending dynamic events.
// Events are consumed (drained) each time an <environment_details> block is
// injected into a user or tool message, so every event is seen by the LLM
// exactly once (FEATURE-471).
type dynamicEventQueue struct {
	mu     sync.Mutex
	events []DynamicEvent
	// maxSize caps the queue; when full the oldest event is dropped.
	maxSize int
}

// newDynamicEventQueue creates a queue with the given capacity (>=1).
func newDynamicEventQueue(maxSize int) *dynamicEventQueue {
	if maxSize < 1 {
		maxSize = 1
	}
	return &dynamicEventQueue{maxSize: maxSize}
}

// add appends an event, dropping the oldest when the queue is full.
// Duplicate path events (same kind + path) are coalesced by refreshing their
// timestamp instead of appending a second entry.
func (q *dynamicEventQueue) add(ev DynamicEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	// Coalesce duplicate path events (clip/upload/open) by path.
	if ev.Kind != DynamicUserMessage {
		for i := range q.events {
			if q.events[i].Kind == ev.Kind && q.events[i].Path == ev.Path {
				q.events[i].Time = ev.Time
				return
			}
		}
	}
	if len(q.events) >= q.maxSize {
		q.events = q.events[1:]
	}
	q.events = append(q.events, ev)
}

// drain removes and returns all buffered events in FIFO order.
func (q *dynamicEventQueue) drain() []DynamicEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return nil
	}
	out := q.events
	q.events = nil
	return out
}

// pendingUserMessages returns (without consuming) the buffered user_message
// events, oldest first. Used to backfill the Web UI input box when a task ends
// with unconsumed user messages (FEATURE-471).
func (q *dynamicEventQueue) pendingUserMessages() []DynamicEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []DynamicEvent
	for _, ev := range q.events {
		if ev.Kind == DynamicUserMessage {
			out = append(out, ev)
		}
	}
	return out
}

// clearUserMessages removes all buffered user_message events. Called after the
// pending messages have been handed back to the Web UI input box.
func (q *dynamicEventQueue) clearUserMessages() {
	q.mu.Lock()
	defer q.mu.Unlock()
	kept := q.events[:0]
	for _, ev := range q.events {
		if ev.Kind != DynamicUserMessage {
			kept = append(kept, ev)
		}
	}
	q.events = kept
}

// dynamicEventQueue returns the agent's dynamic event queue, creating it on
// first use with the configured capacity.
func (a *Agent) dynamicEventQueue() *dynamicEventQueue {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dynEvents == nil {
		a.dynEvents = newDynamicEventQueue(a.dynamicEventQueueCapacity())
	}
	return a.dynEvents
}

// dynamicEventQueueCapacity returns the configured queue capacity (default 100).
func (a *Agent) dynamicEventQueueCapacity() int {
	if a.cfg != nil && a.cfg.DynamicEventQueueSize > 0 {
		return a.cfg.DynamicEventQueueSize
	}
	return 100
}

// AddDynamicEvent enqueues a user-action event into the dynamic perception
// queue. Path events (clip/upload/open) take a workspace-relative path; message
// events take the raw text. Thread-safe; safe to call while a task is running.
func (a *Agent) AddDynamicEvent(kind DynamicEventKind, pathOrText string) {
	ev := DynamicEvent{Kind: kind, Time: time.Now()}
	switch kind {
	case DynamicUserMessage, DynamicBoardRequest, DynamicBoardDM, DynamicBoardNotify, DynamicUIAction:
		// Message-like events carry raw text (user messages, board events and
		// rendered-component actions).
		ev.Text = pathOrText
	default:
		ev.Path = pathOrText
	}
	a.dynamicEventQueue().add(ev)
}

// consumeDynamicEvents drains the queue and renders the <user_dynamic_events>
// block. includeUserMessages controls whether buffered user_message events are
// included (tool messages only, per FEATURE-471). Each event is rendered as a
// single flat tag in queue (time) order:
//
//	<open_file>2026-09-04 12:33:43.321 ./work/test.md 20.3KB</open_file>
//	<user_message>2026-09-04 12:35:00.000 补充消息</user_message>
//
// File size/mtime are resolved live via os.Stat at injection time.
func (a *Agent) consumeDynamicEvents(includeUserMessages bool) string {
	q := a.dynamicEventQueue()
	events := q.drain()
	if len(events) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<user_dynamic_events>\n")
	for _, ev := range events {
		switch ev.Kind {
		case DynamicUserMessage:
			if includeUserMessages {
				sb.WriteString("  <user_message>" + formatEventTime(ev.Time) + " " + ev.Text + "</user_message>\n")
			}
		case DynamicUIAction:
			// FEATURE-524 window mode: a component interaction that happened while
			// the agent was working. The text already carries the ui id, the action
			// id and the structured payload, so it is injected verbatim.
			sb.WriteString("  <ui_action>" + formatEventTime(ev.Time) + " " + ev.Text + "</ui_action>\n")
		case DynamicClipObject:
			sb.WriteString("  <clip_object>" + formatFileEvent(ev) + "</clip_object>\n")
		case DynamicUploadFile:
			sb.WriteString("  <upload_file>" + formatFileEvent(ev) + "</upload_file>\n")
		case DynamicOpenFile:
			sb.WriteString("  <open_file>" + formatFileEvent(ev) + "</open_file>\n")
		case DynamicViewFile:
			sb.WriteString("  <view_file>" + formatFileEvent(ev) + "</view_file>\n")
		case DynamicBoardRequest, DynamicBoardDM, DynamicBoardNotify:
			// FEATURE-490: board events carry a JSON summary in Text. Render as
			// a flat tag so the LLM can see the board activity.
			sb.WriteString("  <" + string(ev.Kind) + ">" + formatEventTime(ev.Time) + " " + ev.Text + "</" + string(ev.Kind) + ">\n")
		}
	}
	sb.WriteString("</user_dynamic_events>")
	return sb.String()
}

// formatFileEvent renders one path event's inner text as:
//
//	<time> <./path> <human-size>
//
// e.g. "2026-09-04 12:33:43.321 ./work/test.md 20.3KB". The path is prefixed
// with "./" and the size is human-readable. The event's own timestamp is used
// for the time (the action time), while the file's mtime is not shown.
func formatFileEvent(ev DynamicEvent) string {
	size, _ := statFile(ev.Path)
	return formatEventTime(ev.Time) + " ./" + ev.Path + " " + humanSize(size)
}

// formatEventTime renders a timestamp with millisecond precision.
func formatEventTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

// humanSize renders a byte count in a human-readable form (B/KB/MB/GB).
func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	f := float64(n)
	for _, u := range units {
		f /= 1024
		if f < 1024 {
			return fmt.Sprintf("%.1f%s", f, u)
		}
	}
	return fmt.Sprintf("%.1fPB", f/1024)
}

// statFile resolves the size and modification time of a workspace-relative path
// (relative to cwd). Missing files yield size 0 and an empty mtime.
func statFile(rel string) (int64, string) {
	if rel == "" {
		return 0, ""
	}
	info, err := os.Stat(rel)
	if err != nil {
		// Try resolving against cwd explicitly (rel may already be absolute).
		if abs, aerr := filepath.Abs(rel); aerr == nil {
			info, err = os.Stat(abs)
		}
	}
	if err != nil || info == nil {
		return 0, ""
	}
	return info.Size(), info.ModTime().Format(time.RFC3339)
}

// PendingUserMessages returns the buffered user_message texts (oldest first)
// without consuming them, and clears them from the queue. The Web UI calls this
// when a task ends to backfill the input box (FEATURE-471).
func (a *Agent) PendingUserMessages() []string {
	q := a.dynamicEventQueue()
	evs := q.pendingUserMessages()
	if len(evs) == 0 {
		return nil
	}
	q.clearUserMessages()
	out := make([]string, 0, len(evs))
	for _, ev := range evs {
		out = append(out, ev.Text)
	}
	return out
}
