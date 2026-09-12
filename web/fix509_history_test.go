// Package web - FIX-509 regression tests: pushHistory must keep paging until it
// has collected `count` message groups or the store reports no older events.
//
// Before FIX-509 hasMore was derived from len(order) > count, so a page that
// happened to contain fewer than `count` groups (common when a single message
// spans many events) reported "no more history" and the browser stopped paging
// after the first request.
//
// The tests drive the store directly (rather than the WebSocket) so they stay
// independent of the session's event push plumbing.
//
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
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// historyEvent builds a persisted event JSON carrying the given message index.
func historyEvent(msgIndex int) []byte {
	ev := agent.StreamEvent{
		Type: agent.EventContent,
		Chan: agent.ChannelLLM,
		Text: fmt.Sprintf("msg-%d", msgIndex),
		Meta: map[string]string{"msg_index": fmt.Sprint(msgIndex)},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	return data
}

// seedHistory appends `groups` message groups, each with `eventsPerGroup`
// events, to the session's persisted event stream.
func seedHistory(t *testing.T, st *store.DualStore, sessionID string, groups, eventsPerGroup int) {
	t.Helper()
	for g := 0; g < groups; g++ {
		for e := 0; e < eventsPerGroup; e++ {
			if err := st.AppendEvent(sessionID, historyEvent(g)); err != nil {
				t.Fatalf("AppendEvent(group=%d, event=%d): %v", g, e, err)
			}
		}
	}
}

// collectHistoryPage drives the real paging function (loadHistoryPage) and
// returns the group keys of the resulting page plus the hasMore flag and the
// cursor. Calling production code here matters: an earlier version of this
// helper duplicated the loop, so it kept passing while the real one regressed.
func collectHistoryPage(t *testing.T, st *store.DualStore, sessionID string, count, beforeSeq int) (groups []string, hasMore bool, oldestSeq int) {
	t.Helper()
	events, hasMore, oldestSeq := loadHistoryPage(st, sessionID, count, beforeSeq)
	seen := map[string]bool{}
	for _, data := range events {
		key := eventGroupKey(data)
		if !seen[key] {
			seen[key] = true
			groups = append(groups, key)
		}
	}
	return groups, hasMore, oldestSeq
}

// newHistoryStore builds a bbolt-backed store over a temp workspace.
func newHistoryStore(t *testing.T) *store.DualStore {
	t.Helper()
	ws, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	return store.NewDualStore(boltStore, nil)
}

// TestPushHistoryFillsRequestedGroupCount verifies that the paging loop returns
// `count` groups even when each group spans many events, i.e. when one
// LoadEvents page cannot cover `count` groups on its own.
func TestPushHistoryFillsRequestedGroupCount(t *testing.T) {
	st := newHistoryStore(t)
	const sessionID = "sess-fix509-fill"

	// 30 groups x 60 events = 1800 raw events. One LoadEvents page is capped at
	// count*maxEventsPerMessage = 20*50 = 1000 raw events, which covers only
	// ~16 groups, so the loop must issue a second request.
	seedHistory(t, st, sessionID, 30, 60)

	groups, hasMore, _ := collectHistoryPage(t, st, sessionID, 20, 0)

	if len(groups) != 20 {
		t.Fatalf("page contains %d message groups, want 20", len(groups))
	}
	if !hasMore {
		t.Fatal("hasMore = false, want true (older groups still exist)")
	}
}

// TestPushHistoryHasMoreFalseAtOldest verifies that hasMore becomes false once
// the page reaches the oldest persisted event.
func TestPushHistoryHasMoreFalseAtOldest(t *testing.T) {
	st := newHistoryStore(t)
	const sessionID = "sess-fix509-oldest"

	// Only 5 groups exist, fewer than the requested 20.
	seedHistory(t, st, sessionID, 5, 3)

	groups, hasMore, oldestSeq := collectHistoryPage(t, st, sessionID, 20, 0)

	if len(groups) != 5 {
		t.Fatalf("page contains %d message groups, want 5", len(groups))
	}
	if hasMore {
		t.Fatal("hasMore = true, want false (no older events exist)")
	}
	if oldestSeq == 0 {
		t.Fatal("oldestSeq = 0, want the sequence of the oldest event")
	}
}

// TestPushHistoryBeforeCursorPagesBackwards verifies that the cursor returned by
// one page can be fed back as `before` to fetch strictly older groups without
// overlap.
func TestPushHistoryBeforeCursorPagesBackwards(t *testing.T) {
	st := newHistoryStore(t)
	const sessionID = "sess-fix509-cursor"

	seedHistory(t, st, sessionID, 10, 2)

	first, _, cursor := collectHistoryPage(t, st, sessionID, 4, 0)
	if cursor == 0 {
		t.Fatal("first page cursor = 0, want a cursor")
	}
	if len(first) != 4 {
		t.Fatalf("first page contains %d groups, want 4", len(first))
	}

	second, _, _ := collectHistoryPage(t, st, sessionID, 4, cursor)
	if len(second) == 0 {
		t.Fatal("second page is empty, want older groups")
	}

	// The two pages must not overlap.
	firstSet := map[string]bool{}
	for _, g := range first {
		firstSet[g] = true
	}
	for _, g := range second {
		if firstSet[g] {
			t.Fatalf("group %q appears in both pages; the cursor did not advance", g)
		}
	}
}

// TestPushHistoryEmptyStream verifies the empty-session case is safe.
func TestPushHistoryEmptyStream(t *testing.T) {
	st := newHistoryStore(t)
	const sessionID = "sess-fix509-empty"

	groups, hasMore, oldestSeq := collectHistoryPage(t, st, sessionID, 20, 0)

	if len(groups) != 0 {
		t.Fatalf("empty stream returned %d groups, want 0", len(groups))
	}
	if hasMore {
		t.Fatal("hasMore = true for an empty stream, want false")
	}
	if oldestSeq != 0 {
		t.Fatalf("oldestSeq = %d for an empty stream, want 0", oldestSeq)
	}
}

// TestHistoryMessageAlwaysCarriesPagingFields is the FIX-509 regression guard for
// the wire format: has_more and oldest_seq must be present in every history
// message, even when they hold their zero values.
//
// They used to be tagged `omitempty`, so a page with has_more=false or
// oldest_seq=0 serialized without those keys at all. The browser then read
// undefined, stored oldest_seq as 0, and loadOlderHistory bailed out on its
// `!historyOldestSeq` guard — paging stopped after the very first page.
func TestHistoryMessageAlwaysCarriesPagingFields(t *testing.T) {
	cases := []struct {
		name      string
		hasMore   bool
		oldestSeq int
	}{
		{"zero values", false, 0},
		{"has more", true, 42},
		{"no more but cursor set", false, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(serverMessage{
				Kind:      "history",
				HasMore:   tc.hasMore,
				OldestSeq: tc.oldestSeq,
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, ok := decoded["has_more"]; !ok {
				t.Errorf("has_more missing from %s", raw)
			}
			if _, ok := decoded["oldest_seq"]; !ok {
				t.Errorf("oldest_seq missing from %s", raw)
			}
			var got struct {
				HasMore   bool `json:"has_more"`
				OldestSeq int  `json:"oldest_seq"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.HasMore != tc.hasMore || got.OldestSeq != tc.oldestSeq {
				t.Errorf("round-trip = (%v, %d), want (%v, %d)",
					got.HasMore, got.OldestSeq, tc.hasMore, tc.oldestSeq)
			}
		})
	}
}

// TestTopSentinelIsNotSticky is the FIX-509 regression guard for the stream's
// top sentinel style.
//
// The sentinel is watched by an IntersectionObserver to load older history
// pages. It must stay in normal flow: a `position: sticky` marker is pinned to
// the scroll container's top edge at every scroll offset, so it never leaves
// the root and the observer sees no further enter/leave transitions. Paging
// then stops after the very first page — the exact symptom users reported.
func TestTopSentinelIsNotSticky(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	// Isolate the .stream-top-sentinel rule body.
	marker := ".stream-top-sentinel {"
	start := strings.Index(string(css), marker)
	if start < 0 {
		t.Fatalf("%q rule not found in style.css", marker)
	}
	end := strings.Index(string(css)[start:], "}")
	if end < 0 {
		t.Fatalf("unterminated %q rule", marker)
	}
	body := string(css)[start : start+end]
	if strings.Contains(body, "position: sticky") || strings.Contains(body, "position:sticky") {
		t.Errorf("top sentinel must not be position:sticky (observer would never re-fire); rule body:\n%s", body)
	}
}

// historyEventTagged builds an event whose text encodes both its group index and
// its position inside the group, so a test can verify the exact replay order.
func historyEventTagged(group, inGroup int) []byte {
	ev := agent.StreamEvent{
		Type: agent.EventContent,
		Chan: agent.ChannelLLM,
		Text: fmt.Sprintf("g%d-%d", group, inGroup),
		Meta: map[string]string{"msg_index": fmt.Sprint(group)},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	return data
}

// TestHistoryPageIsChronologicalAcrossWindows is the FIX-509 regression guard for
// replay ordering. LoadEvents returns the NEWEST window below the cursor, so a
// second call returns an older window. Appending pages in call order therefore
// emitted newer events before older ones, and a group split across two windows
// ended up internally reversed — the replayed conversation came back with LLM and
// TOOL blocks clumped instead of alternating. The page must be globally
// chronological and must end on the newest group.
func TestHistoryPageIsChronologicalAcrossWindows(t *testing.T) {
	const sessionID = "sess-fix509-order"
	st := newHistoryStore(t)

	// 40 groups x 60 events = 2400 events. One 1000-event window covers only
	// ~16 groups, so requesting 20 forces a second, older window to be read.
	const totalGroups = 40
	const eventsPerGroup = 60
	for g := 0; g < totalGroups; g++ {
		for e := 0; e < eventsPerGroup; e++ {
			if err := st.AppendEvent(sessionID, historyEventTagged(g, e)); err != nil {
				t.Fatalf("AppendEvent(group=%d, event=%d): %v", g, e, err)
			}
		}
	}

	events, hasMore, oldestSeq := loadHistoryPage(st, sessionID, 20, 0)
	if len(events) == 0 {
		t.Fatal("loadHistoryPage returned no events")
	}
	if !hasMore {
		t.Error("hasMore = false, want true (older groups still exist)")
	}
	if oldestSeq == 0 {
		t.Error("oldestSeq = 0, want a usable cursor")
	}

	firstGroup, lastGroup := -1, -1
	groupCount := 0
	prevGroup, prevInGroup := -1, -1
	for i, data := range events {
		var ev struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			t.Fatalf("unmarshal event %d: %v", i, err)
		}
		var g, e int
		if _, err := fmt.Sscanf(ev.Text, "g%d-%d", &g, &e); err != nil {
			t.Fatalf("event %d has unexpected text %q: %v", i, ev.Text, err)
		}
		if g < prevGroup {
			t.Fatalf("event %d: group went backwards (%d after %d) — page is not chronological", i, g, prevGroup)
		}
		if g == prevGroup {
			if e != prevInGroup+1 {
				t.Fatalf("event %d: group %d out of order (event %d after %d)", i, g, e, prevInGroup)
			}
		} else {
			if prevGroup >= 0 && e != 0 {
				t.Fatalf("group %d starts at event %d, want 0", g, e)
			}
			groupCount++
			if firstGroup < 0 {
				firstGroup = g
			}
			lastGroup = g
		}
		prevGroup, prevInGroup = g, e
	}

	if groupCount != 20 {
		t.Errorf("group count = %d, want 20", groupCount)
	}
	if firstGroup != totalGroups-20 {
		t.Errorf("first group = %d, want %d", firstGroup, totalGroups-20)
	}
	if lastGroup != totalGroups-1 {
		t.Errorf("last group = %d, want %d (the newest group must come last)", lastGroup, totalGroups-1)
	}
}

// TestStaticAssetsCarryAnETag is the FIX-509 regression guard for UI delivery.
//
// The UI assets are embedded in the binary and have no modification time, so
// http.FileServer used to emit neither Last-Modified nor ETag. Browsers fell
// back to heuristic caching and kept serving a stale app.js after a rebuild,
// which made UI fixes appear to have no effect. Every static response must now
// carry a validator and a Cache-Control that forces revalidation.
func TestStaticAssetsCarryAnETag(t *testing.T) {
	handler := serveStatic()

	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing — the browser cannot revalidate the asset")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}

	// A repeat request with the same validator must be answered with 304 so the
	// revalidation stays cheap.
	req2 := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for a matching validator", rec2.Code)
	}
}
