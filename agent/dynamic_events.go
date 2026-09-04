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
	if kind == DynamicUserMessage {
		ev.Text = pathOrText
	} else {
		ev.Path = pathOrText
	}
	a.dynamicEventQueue().add(ev)
}

// consumeDynamicEvents drains the queue and renders the <user_dynamic_events>
// block. includeUserMessages controls whether buffered user_message events are
// included (tool messages only, per FEATURE-471). File size/mtime are resolved
// live via os.Stat at injection time.
func (a *Agent) consumeDynamicEvents(includeUserMessages bool) string {
	q := a.dynamicEventQueue()
	events := q.drain()
	if len(events) == 0 {
		return ""
	}

	var clips, uploads, msgs, opens []DynamicEvent
	for _, ev := range events {
		switch ev.Kind {
		case DynamicClipObject:
			clips = append(clips, ev)
		case DynamicUploadFile:
			uploads = append(uploads, ev)
		case DynamicUserMessage:
			if includeUserMessages {
				msgs = append(msgs, ev)
			}
		case DynamicOpenFile:
			opens = append(opens, ev)
		}
	}

	var sb strings.Builder
	sb.WriteString("<user_dynamic_events>\n")
	if len(clips) > 0 {
		sb.WriteString("  <clip_objects>\n")
		for _, ev := range clips {
			sb.WriteString("    " + formatFileEvent(ev) + "\n")
		}
		sb.WriteString("  </clip_objects>\n")
	}
	if len(uploads) > 0 {
		sb.WriteString("  <upload_files>\n")
		for _, ev := range uploads {
			sb.WriteString("    " + formatFileEvent(ev) + "\n")
		}
		sb.WriteString("  </upload_files>\n")
	}
	if len(msgs) > 0 {
		sb.WriteString("  <user_messages>\n")
		for _, ev := range msgs {
			sb.WriteString("    <message time=\"" + ev.Time.Format(time.RFC3339) + "\">" + ev.Text + "</message>\n")
		}
		sb.WriteString("  </user_messages>\n")
	}
	if len(opens) > 0 {
		sb.WriteString("  <open_files>\n")
		for _, ev := range opens {
			sb.WriteString("    " + formatFileEvent(ev) + "\n")
		}
		sb.WriteString("  </open_files>\n")
	}
	sb.WriteString("</user_dynamic_events>")
	return sb.String()
}

// formatFileEvent renders one path event as a self-closing element carrying the
// workspace-relative path, live size and mtime:
//
//	<file path="input/clip-1.png" size="36739" mtime="2026-09-04T19:32:00+08:00"/>
func formatFileEvent(ev DynamicEvent) string {
	size, mtime := statFile(ev.Path)
	return fmt.Sprintf("<file path=\"%s\" size=\"%d\" mtime=\"%s\"/>", ev.Path, size, mtime)
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
