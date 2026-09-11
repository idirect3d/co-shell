// Author: L.Shuang
// Created: 2026-09-11
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/idirect3d/co-shell/workspace"
)

// newTestStore opens a Store backed by a temporary workspace.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	ws, err := workspace.New(dir)
	if err != nil {
		t.Fatalf("workspace.New: %v", err)
	}
	s, err := NewStore(ws)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// eventJSON builds a minimal persisted event with a message index.
func eventJSON(msgIndex int, text string) []byte {
	ev := map[string]interface{}{
		"type": "content",
		"chan": "llm",
		"text": text,
		"meta": map[string]string{"msg_index": fmt.Sprint(msgIndex)},
	}
	data, _ := json.Marshal(ev)
	return data
}

// TestAppendAndLoadEvents verifies events are persisted and read back in order.
func TestAppendAndLoadEvents(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-test-1"
	for i := 1; i <= 5; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, fmt.Sprintf("msg-%d", i))); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}
	entries, hasMore, err := s.LoadEvents(sid, 3, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if !hasMore {
		t.Fatalf("expected hasMore=true when older events exist")
	}
	// The tail window must be the newest three, in chronological order.
	if entries[0].Seq != 3 || entries[2].Seq != 5 {
		t.Fatalf("unexpected seq window: %d..%d", entries[0].Seq, entries[2].Seq)
	}
}

// TestLoadEventsBeforeCursor verifies the paging cursor returns older events
// without overlapping the previous page.
func TestLoadEventsBeforeCursor(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-test-2"
	for i := 1; i <= 10; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, fmt.Sprintf("msg-%d", i))); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}
	page1, _, err := s.LoadEvents(sid, 3, 0)
	if err != nil {
		t.Fatalf("LoadEvents page1: %v", err)
	}
	if page1[0].Seq != 8 {
		t.Fatalf("page1 should start at seq 8, got %d", page1[0].Seq)
	}
	page2, _, err := s.LoadEvents(sid, 3, page1[0].Seq)
	if err != nil {
		t.Fatalf("LoadEvents page2: %v", err)
	}
	if len(page2) != 3 {
		t.Fatalf("expected 3 entries on page2, got %d", len(page2))
	}
	if page2[2].Seq >= page1[0].Seq {
		t.Fatalf("page2 must not overlap page1: page2 last=%d page1 first=%d", page2[2].Seq, page1[0].Seq)
	}
}

// TestLoadEventsEmptySession verifies an unknown session yields an empty page.
func TestLoadEventsEmptySession(t *testing.T) {
	s := newTestStore(t)
	entries, hasMore, err := s.LoadEvents("sess-does-not-exist", 20, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) != 0 || hasMore {
		t.Fatalf("expected empty page, got %d entries hasMore=%v", len(entries), hasMore)
	}
}

// TestDeleteEventsAfter verifies pop-style truncation drops later messages.
func TestDeleteEventsAfter(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-test-3"
	for i := 1; i <= 6; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, fmt.Sprintf("msg-%d", i))); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}
	if err := s.DeleteEventsAfter(sid, 3); err != nil {
		t.Fatalf("DeleteEventsAfter: %v", err)
	}
	entries, _, err := s.LoadEvents(sid, 100, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 remaining events, got %d", len(entries))
	}
	for _, e := range entries {
		idx, ok := eventMsgIndex(e.Data)
		if !ok || idx > 3 {
			t.Fatalf("event with msg_index %d survived truncation", idx)
		}
	}
}

// TestClearEventStream verifies a session's events can be removed entirely.
func TestClearEventStream(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-test-4"
	for i := 1; i <= 4; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, "x")); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
	if err := s.ClearEventStream(sid); err != nil {
		t.Fatalf("ClearEventStream: %v", err)
	}
	entries, _, err := s.LoadEvents(sid, 100, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no events after clear, got %d", len(entries))
	}
}

// TestEventStreamSessionIsolation verifies sessions do not see each other.
func TestEventStreamSessionIsolation(t *testing.T) {
	s := newTestStore(t)
	if err := s.AppendEvent("sess-a", eventJSON(1, "a")); err != nil {
		t.Fatalf("AppendEvent a: %v", err)
	}
	if err := s.AppendEvent("sess-b", eventJSON(1, "b")); err != nil {
		t.Fatalf("AppendEvent b: %v", err)
	}
	a, _, err := s.LoadEvents("sess-a", 10, 0)
	if err != nil {
		t.Fatalf("LoadEvents a: %v", err)
	}
	if len(a) != 1 {
		t.Fatalf("session a should have 1 event, got %d", len(a))
	}
	if err := s.ClearEventStream("sess-a"); err != nil {
		t.Fatalf("ClearEventStream a: %v", err)
	}
	b, _, err := s.LoadEvents("sess-b", 10, 0)
	if err != nil {
		t.Fatalf("LoadEvents b: %v", err)
	}
	if len(b) != 1 {
		t.Fatalf("clearing session a must not affect session b, got %d", len(b))
	}
}

// TestEventStreamCapTrimsOldest verifies the per-session cap drops old events.
func TestEventStreamCapTrimsOldest(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-test-5"
	total := maxEventStreamEntries + 10
	for i := 1; i <= total; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, "x")); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}
	entries, _, err := s.LoadEvents(sid, total+100, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) != maxEventStreamEntries {
		t.Fatalf("expected cap of %d events, got %d", maxEventStreamEntries, len(entries))
	}
	// The newest event must survive the trim.
	if entries[len(entries)-1].Seq != total {
		t.Fatalf("newest event should be seq %d, got %d", total, entries[len(entries)-1].Seq)
	}
}

// TestEventStreamPersistsAcrossReopen verifies events survive a store restart,
// which is what makes refresh-replay work without in-memory history.
func TestEventStreamPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	ws, err := workspace.New(dir)
	if err != nil {
		t.Fatalf("workspace.New: %v", err)
	}
	s1, err := NewStore(ws)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	const sid = "sess-test-6"
	if err := s1.AppendEvent(sid, eventJSON(1, "persisted")); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := NewStore(ws)
	if err != nil {
		t.Fatalf("reopen NewStore: %v", err)
	}
	defer func() { _ = s2.Close() }()
	entries, _, err := s2.LoadEvents(sid, 10, 0)
	if err != nil {
		t.Fatalf("LoadEvents after reopen: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 persisted event after reopen, got %d", len(entries))
	}
	if filepath.Base(ws.DBPath()) == "" {
		t.Fatalf("unexpected empty db path")
	}
}
