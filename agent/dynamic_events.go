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
	// DynamicBoardMessage: a message delivered by the hub message board while
	// the agent was busy (FEATURE-530). Text carries a ready-rendered
	// <board_message> tag.
	DynamicBoardMessage DynamicEventKind = "board_message"
	// DynamicBoardTimeout: the hub reports that a recipient never answered a
	// message that expected a reply (FEATURE-530). Text carries a rendered
	// <board_timeout> tag.
	DynamicBoardTimeout DynamicEventKind = "board_timeout"

	// DynamicUIAction: a component interaction (form submit, button, chart
	// drill-down) reported while a turn is running (FEATURE-524 window mode).
	// It is injected into the running turn instead of starting a new one, so the
	// LLM sees the user's action right before its next call.
	DynamicUIAction DynamicEventKind = "ui_action"

	// DynamicSubAgentDone: a sub-agent reached a terminal state (completed or
	// failed) while the agent was mid-turn (FEATURE-566). Text carries a
	// ready-rendered <sub_agent_done> tag.
	DynamicSubAgentDone DynamicEventKind = "sub_agent_done"
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
	// ID is the stable unique identifier assigned on enqueue (FEATURE-611),
	// e.g. "ev-7". A retainable event keeps its ID across replays, and the LLM
	// acknowledges it by listing that ID in the tool call's meta.ack.
	ID string
}

// dynamicEventQueue is the thread-safe FIFO queue of pending dynamic events.
// Events are consumed (drained) each time an <environment_details> block is
// injected into a user or tool message, so every event is seen by the LLM
// exactly once (FEATURE-471).
type dynamicEventQueue struct {
	mu     sync.Mutex
	events []DynamicEvent
	// retained holds events that were already injected but not yet
	// acknowledged (FEATURE-611). They are replayed in every following
	// envelope until the LLM acks them via meta.ack. Only retainable kinds
	// (see isRetainableKind) ever enter this list: a transient file event is
	// dropped right after its single injection.
	retained []DynamicEvent
	// seq assigns each enqueued event a stable unique id ("ev-<n>").
	seq int
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
	// Coalesce duplicate path events (clip/upload/open/view) by path. Text
	// events (user message, board message, ui action, sub-agent notice) carry
	// their payload in Text and leave Path empty, so they must never coalesce:
	// two notices of the same kind are two distinct events (FEATURE-566).
	if ev.Path != "" {
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
	q.seq++
	ev.ID = fmt.Sprintf("ev-%d", q.seq)
	q.events = append(q.events, ev)
}

// snapshot returns copies of the pending and retained event lists without
// touching either one (FEATURE-612).
func (q *dynamicEventQueue) snapshot() (pending, retained []DynamicEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	pending = append([]DynamicEvent(nil), q.events...)
	retained = append([]DynamicEvent(nil), q.retained...)
	return pending, retained
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

// isRetainableKind reports whether an event of this kind is kept for replay
// until the LLM acknowledges it (FEATURE-611). Transient path events are
// excluded: a file the user just dropped or opened is actionable immediately,
// so replaying it would only waste context.
func isRetainableKind(kind DynamicEventKind) bool {
	switch kind {
	case DynamicUserMessage, DynamicUIAction, DynamicBoardMessage,
		DynamicBoardTimeout, DynamicSubAgentDone:
		return true
	}
	return false
}

// takeForInjection removes every pending event and returns them as fresh
// (first-time) events, together with the previously injected but still
// unacknowledged events that must be replayed. Newly injected retainable
// events move into the retained list; transient events are dropped after their
// single injection. retainMax <= 0 disables retention altogether, restoring
// the pre-FEATURE-611 one-shot behaviour.
func (q *dynamicEventQueue) takeForInjection(retainMax int) (fresh, replayed []DynamicEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	fresh = q.events
	q.events = nil
	if retainMax <= 0 {
		q.retained = nil
		return fresh, nil
	}
	replayed = append([]DynamicEvent(nil), q.retained...)
	for _, ev := range fresh {
		if isRetainableKind(ev.Kind) {
			q.retained = append(q.retained, ev)
		}
	}
	// Cap the retained set by dropping its oldest entries.
	if len(q.retained) > retainMax {
		q.retained = q.retained[len(q.retained)-retainMax:]
	}
	return fresh, replayed
}

// ack drops the given event ids from the retained (and, defensively, the
// pending) set so they stop being replayed (FEATURE-611). Unknown or already
// acknowledged ids are ignored, which makes acknowledgement idempotent. It
// returns how many events were acknowledged.
func (q *dynamicEventQueue) ack(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	keptRetained := q.retained[:0]
	for _, ev := range q.retained {
		if want[ev.ID] {
			n++
			continue
		}
		keptRetained = append(keptRetained, ev)
	}
	q.retained = keptRetained
	keptPending := q.events[:0]
	for _, ev := range q.events {
		if want[ev.ID] {
			n++
			continue
		}
		keptPending = append(keptPending, ev)
	}
	q.events = keptPending
	return n
}

// pendingUserMessages returns (without consuming) the buffered user_message
// events, oldest first. Both the pending queue and the retained set are
// scanned (FEATURE-611): a user message that was already injected but never
// acknowledged is still unconsumed from the user's point of view. Used to
// backfill the Web UI input box when a task ends with unconsumed user messages
// (FEATURE-471).
func (q *dynamicEventQueue) pendingUserMessages() []DynamicEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []DynamicEvent
	for _, ev := range q.retained {
		if ev.Kind == DynamicUserMessage {
			out = append(out, ev)
		}
	}
	for _, ev := range q.events {
		if ev.Kind == DynamicUserMessage {
			out = append(out, ev)
		}
	}
	return out
}

// clearUserMessages removes all buffered user_message events (pending and
// retained). Called after the pending messages have been handed back to the
// Web UI input box.
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
	keptRetained := q.retained[:0]
	for _, ev := range q.retained {
		if ev.Kind != DynamicUserMessage {
			keptRetained = append(keptRetained, ev)
		}
	}
	q.retained = keptRetained
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

// Defaults for the FEATURE-611 dynamic-event retention settings.
const (
	// defaultDynamicEventRetainMax is the default cap on injected-but-unacked
	// retainable events kept for replay; 0 disables replay altogether.
	defaultDynamicEventRetainMax = 100
	// UserMessageModeDynamic delivers a supplemental user message through
	// <user_dynamic_events>; UserMessageModeFull delivers it as a complete user
	// message (a regular user turn).
	UserMessageModeDynamic = "dynamic"
	UserMessageModeFull    = "full"
)

// dynamicEventQueueCapacity returns the configured queue capacity (default 100).
func (a *Agent) dynamicEventQueueCapacity() int {
	if a.cfg != nil && a.cfg.DynamicEventQueueSize > 0 {
		return a.cfg.DynamicEventQueueSize
	}
	return 100
}

// dynamicEventRetainMax returns the configured cap on injected-but-unacked
// events retained for replay (FEATURE-611). 0 (or a negative value) disables
// the mechanism, restoring one-shot injection.
func (a *Agent) dynamicEventRetainMax() int {
	if a.cfg != nil {
		return a.cfg.DynamicEventRetainMax
	}
	return defaultDynamicEventRetainMax
}

// AckDynamicEvents acknowledges the given dynamic-event ids (FEATURE-611): the
// events stop being replayed in <user_dynamic_events>. Unknown or already
// acknowledged ids are ignored, so acknowledgement is idempotent. It returns
// how many events were acknowledged. Safe to call while a task is running.
func (a *Agent) AckDynamicEvents(ids []string) int {
	return a.dynamicEventQueue().ack(ids)
}

// UserMessageMode returns how a message the user types while a task is running
// is delivered (FEATURE-611): UserMessageModeDynamic (default) injects it into
// <user_dynamic_events>, while UserMessageModeFull turns it into a complete
// user message.
func (a *Agent) UserMessageMode() string {
	if a.cfg != nil && a.cfg.UserMessageMode != "" {
		return a.cfg.UserMessageMode
	}
	return UserMessageModeDynamic
}

// AddDynamicEvent enqueues a user-action event into the dynamic perception
// queue. Path events (clip/upload/open) take a workspace-relative path; message
// events take the raw text. Thread-safe; safe to call while a task is running.
func (a *Agent) AddDynamicEvent(kind DynamicEventKind, pathOrText string) {
	ev := DynamicEvent{Kind: kind, Time: time.Now()}
	switch kind {
	case DynamicUserMessage, DynamicUIAction, DynamicBoardMessage, DynamicBoardTimeout, DynamicSubAgentDone:
		// Message-like events carry raw text (user messages, board events,
		// rendered-component actions and sub-agent completion notices).
		ev.Text = pathOrText
	default:
		ev.Path = pathOrText
	}
	a.dynamicEventQueue().add(ev)
}

// consumeDynamicEvents returns the rendered <user_dynamic_events> block: the
// freshly queued events plus every retainable event that was already injected
// but not yet acknowledged (FEATURE-611). A replayed event carries
// unacked="true" and a "[unacked] " marker so the LLM knows it is a repeat;
// a retainable event is acknowledged by listing its ack_id in a tool call's
// meta.ack, after which it stops being replayed. includeUserMessages controls
// whether user_message events are included (tool messages only, per
// FEATURE-471). Each event is rendered as a single flat tag in queue (time)
// order:
//
//	<open_file>2026-09-04 12:33:43.321 ./work/test.md 20.3KB</open_file>
//	<user_message ack_id="ev-12">2026-09-04 12:35:00.000 补充消息</user_message>
//
// File size/mtime are resolved live via os.Stat at injection time.
func (a *Agent) consumeDynamicEvents(includeUserMessages bool) string {
	q := a.dynamicEventQueue()
	fresh, replayed := q.takeForInjection(a.dynamicEventRetainMax())
	if len(fresh) == 0 && len(replayed) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<user_dynamic_events>\n")
	for _, ev := range fresh {
		if ev.Kind == DynamicUserMessage && !includeUserMessages {
			continue
		}
		sb.WriteString(renderDynamicEvent(ev, false))
	}
	for _, ev := range replayed {
		if ev.Kind == DynamicUserMessage && !includeUserMessages {
			continue
		}
		sb.WriteString(renderDynamicEvent(ev, true))
	}
	sb.WriteString("</user_dynamic_events>")
	return sb.String()
}

// renderDynamicEvent renders one event as a flat tag line (two-space indent and
// trailing newline). Retainable events carry an ack_id attribute so the LLM can
// acknowledge them via meta.ack; a replay (already injected once, not yet
// acknowledged) additionally carries unacked="true" and a "[unacked] " marker
// (FEATURE-611).
func renderDynamicEvent(ev DynamicEvent, replayed bool) string {
	attrs := ""
	marker := ""
	if isRetainableKind(ev.Kind) && ev.ID != "" {
		attrs = ` ack_id="` + ev.ID + `"`
	}
	if replayed {
		attrs += ` unacked="true"`
		marker = "[unacked] "
	}
	switch ev.Kind {
	case DynamicUserMessage:
		return "  <user_message" + attrs + ">" + marker + formatEventTime(ev.Time) + " " + ev.Text + "</user_message>\n"
	case DynamicUIAction:
		// FEATURE-524 window mode: a component interaction that happened while
		// the agent was working. The text carries the ui id, the action id and
		// the structured payload.
		return "  <ui_action" + attrs + ">" + marker + formatEventTime(ev.Time) + " " + ev.Text + "</ui_action>\n"
	case DynamicClipObject:
		return "  <clip_object>" + formatFileEvent(ev) + "</clip_object>\n"
	case DynamicUploadFile:
		return "  <upload_file>" + formatFileEvent(ev) + "</upload_file>\n"
	case DynamicOpenFile:
		return "  <open_file>" + formatFileEvent(ev) + "</open_file>\n"
	case DynamicViewFile:
		return "  <view_file>" + formatFileEvent(ev) + "</view_file>\n"
	case DynamicBoardMessage, DynamicBoardTimeout, DynamicSubAgentDone:
		// FEATURE-530/FEATURE-566: board events and sub-agent completion notices
		// arrive as an already-rendered tag in Text, so the ack id and the
		// replay marker are injected into that tag.
		return "  " + injectTagAttrs(ev.Text, attrs, marker) + "\n"
	}
	return ""
}

// injectTagAttrs inserts attrs before the closing bracket of an already-rendered
// tag and marker right after it (FEATURE-611). A self-closing tag keeps its
// "/>" well-formed; text that is not a tag gets the attrs appended so nothing
// is silently dropped.
func injectTagAttrs(tag, attrs, marker string) string {
	if tag == "" {
		return tag
	}
	gt := strings.Index(tag, ">")
	if gt < 0 {
		return tag + attrs + marker
	}
	at := gt
	if at > 0 && tag[at-1] == '/' {
		at-- // self-closing tag: insert before "/>"
	}
	return tag[:at] + attrs + tag[at:gt] + ">" + marker + tag[gt+1:]
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

// DynEventView is the browser-facing view of one dynamic perception event
// (FEATURE-612). It feeds the Web UI's dynamic-info panel.
type DynEventView struct {
	// ID is the stable event id ("ev-<n>"), the very value meta.ack uses.
	ID string `json:"id"`
	// Kind is the DynamicEventKind (clip_object, user_message, board_message…).
	Kind string `json:"kind"`
	// Path is the workspace-relative path of a path event.
	Path string `json:"path,omitempty"`
	// Text is the payload of a text event (user message, board notice…).
	Text string `json:"text,omitempty"`
	// Ts is the event time in Unix milliseconds.
	Ts int64 `json:"ts"`
	// Pending marks an event that is still queued, i.e. never injected into an
	// <environment_details> envelope yet.
	Pending bool `json:"pending,omitempty"`
	// Unacked marks an injected event the LLM has not confirmed through
	// meta.ack yet (FEATURE-611); such events are replayed until acknowledged.
	Unacked bool `json:"unacked,omitempty"`
}

// DynEventsSnapshot returns the events the panel may still show, newest first:
// the ones queued for their first injection (pending) plus the ones already
// injected but not yet acknowledged (unacked, FEATURE-611). Nothing else — an
// event leaves the snapshot for good once it is consumed: a transient file
// event right after its single injection, a retainable one once the LLM acks
// it ("read once, then gone"). Read-only, nothing is consumed.
//
// The two lists are merged by walking both backwards instead of sorting: every
// queued event is newer than every retained one, because an event only reaches
// the retained set by being taken out of the queue during an injection.
func (a *Agent) DynEventsSnapshot() []DynEventView {
	q := a.dynamicEventQueue()
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]DynEventView, 0, len(q.events)+len(q.retained))
	for i := len(q.events) - 1; i >= 0; i-- {
		out = append(out, dynEventView(q.events[i], true, false))
	}
	for i := len(q.retained) - 1; i >= 0; i-- {
		out = append(out, dynEventView(q.retained[i], false, true))
	}
	return out
}

// dynEventView projects one queued event onto its browser-facing view, tagged
// with the set it currently lives in.
func dynEventView(ev DynamicEvent, pending, unacked bool) DynEventView {
	return DynEventView{
		ID:      ev.ID,
		Kind:    string(ev.Kind),
		Path:    ev.Path,
		Text:    ev.Text,
		Ts:      ev.Time.UnixMilli(),
		Pending: pending,
		Unacked: unacked,
	}
}
