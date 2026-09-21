// Author: L.Shuang
// Created: 2026-09-21
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
	"fmt"
	"testing"
)

// FEATURE-548 store-level tests: the persisted event stream is paged and
// trimmed by whole message groups, so a message is never returned or kept
// half-complete (UC-0001 ~ UC-0004).

// uc548AppendGroup appends n events belonging to one message group.
func uc548AppendGroup(t *testing.T, s *Store, sessionID string, msgIndex, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := s.AppendEvent(sessionID, eventJSON(msgIndex, fmt.Sprintf("m%d-%d", msgIndex, i))); err != nil {
			t.Fatalf("AppendEvent(m%d,%d): %v", msgIndex, i, err)
		}
	}
}

// uc548GroupOrder returns the message groups in first-seen order together with
// the number of events seen for each of them.
func uc548GroupOrder(t *testing.T, entries []EventStreamEntry) ([]int, map[int]int) {
	t.Helper()
	var order []int
	counts := map[int]int{}
	for _, e := range entries {
		idx, ok := eventMsgIndex(e.Data)
		if !ok {
			t.Fatalf("event without msg_index: %s", e.Data)
		}
		if _, seen := counts[idx]; !seen {
			order = append(order, idx)
		}
		counts[idx]++
	}
	return order, counts
}

// UC-0001: paging returns whole message groups, never a half message.
func TestFeature548UC0001GroupsAreWhole(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-548-uc0001"
	for g := 0; g < 3; g++ {
		uc548AppendGroup(t, s, sid, g, 5)
	}

	entries, hasMore, oldestSeq, err := s.LoadEventsByGroups(sid, 2, 0)
	if err != nil {
		t.Fatalf("LoadEventsByGroups: %v", err)
	}
	if len(entries) != 10 {
		t.Fatalf("expected 10 events (2 groups x 5), got %d", len(entries))
	}
	if !hasMore {
		t.Fatalf("expected hasMore=true: group 0 is older than the returned page")
	}
	order, counts := uc548GroupOrder(t, entries)
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected groups [1 2] (newest two), got %v", order)
	}
	for _, g := range order {
		if counts[g] != 5 {
			t.Fatalf("group %d returned %d events, want 5 (half message)", g, counts[g])
		}
	}
	if oldestSeq != entries[0].Seq {
		t.Fatalf("cursor = %d, want %d (first event of the oldest returned group)", oldestSeq, entries[0].Seq)
	}
}

// UC-0002: a group bigger than the old raw-event page budget is still returned
// whole.
func TestFeature548UC0002OversizedGroupIsWhole(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-548-uc0002"
	const bigGroup = 120
	uc548AppendGroup(t, s, sid, 0, bigGroup)
	uc548AppendGroup(t, s, sid, 1, 3)

	entries, hasMore, cursor, err := s.LoadEventsByGroups(sid, 1, 0)
	if err != nil {
		t.Fatalf("LoadEventsByGroups(newest): %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("newest page: got %d events, want 3 (group 1 only)", len(entries))
	}
	if order, counts := uc548GroupOrder(t, entries); len(order) != 1 || order[0] != 1 || counts[1] != 3 {
		t.Fatalf("newest page: groups %v counts %v, want only group 1 with 3 events", order, counts)
	}
	if !hasMore {
		t.Fatalf("newest page: expected hasMore=true (the 120-event group is older)")
	}

	older, hasMoreOlder, _, err := s.LoadEventsByGroups(sid, 1, cursor)
	if err != nil {
		t.Fatalf("LoadEventsByGroups(older): %v", err)
	}
	if len(older) != bigGroup {
		t.Fatalf("older page: got %d events, want all %d of group 0", len(older), bigGroup)
	}
	order, counts := uc548GroupOrder(t, older)
	if len(order) != 1 || order[0] != 0 || counts[0] != bigGroup {
		t.Fatalf("older page: groups %v counts %v, want only group 0 with %d events", order, counts, bigGroup)
	}
	if hasMoreOlder {
		t.Fatalf("older page: hasMore=true, want false (no group left)")
	}
}

// UC-0003: repeated paging neither duplicates nor drops events and keeps the
// global order monotonic.
func TestFeature548UC0003PagingIsComplete(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-548-uc0003"
	const groups = 25
	const perGroup = 7
	for g := 0; g < groups; g++ {
		uc548AppendGroup(t, s, sid, g, perGroup)
	}

	var pages [][]EventStreamEntry
	before := 0
	for {
		entries, hasMore, oldestSeq, err := s.LoadEventsByGroups(sid, 5, before)
		if err != nil {
			t.Fatalf("LoadEventsByGroups(before=%d): %v", before, err)
		}
		if len(entries) == 0 {
			break
		}
		pages = append(pages, entries)
		if !hasMore {
			break
		}
		if before > 0 && oldestSeq >= before {
			t.Fatalf("cursor did not move towards older events: %d -> %d", before, oldestSeq)
		}
		before = oldestSeq
		if len(pages) > groups {
			t.Fatal("paging loop did not terminate")
		}
	}

	// Pages are returned newest-first; concatenate oldest-first to replay.
	var all []EventStreamEntry
	for i := len(pages) - 1; i >= 0; i-- {
		all = append(all, pages[i]...)
	}
	if len(all) != groups*perGroup {
		t.Fatalf("paged %d events, want %d (duplicates or gaps)", len(all), groups*perGroup)
	}
	order, counts := uc548GroupOrder(t, all)
	if len(order) != groups {
		t.Fatalf("paged %d groups, want %d", len(order), groups)
	}
	for i, g := range order {
		if g != i {
			t.Fatalf("group order not monotonic at position %d: %v", i, order)
		}
		if counts[g] != perGroup {
			t.Fatalf("group %d has %d events, want %d", g, counts[g], perGroup)
		}
	}
}

// UC-0004: trimming the oldest events never cuts a message group in half.
func TestFeature548UC0004TrimKeepsGroupsWhole(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-548-uc0004"
	const groupSize = 3
	// Two groups beyond the cap so trimming certainly runs several times.
	const groupCount = maxEventStreamEntries/groupSize + 2
	for g := 0; g < groupCount; g++ {
		uc548AppendGroup(t, s, sid, g, groupSize)
	}

	entries, _, err := s.LoadEvents(sid, maxEventStreamEntries*2, 0)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no events survived trimming")
	}
	if len(entries) < maxEventStreamEntries {
		t.Fatalf("only %d events survived, want at least %d (trimming dropped a whole group too many)",
			len(entries), maxEventStreamEntries)
	}
	order, counts := uc548GroupOrder(t, entries)
	for _, g := range order {
		if counts[g] != groupSize {
			t.Fatalf("group %d has %d events, want %d (trim cut a message in half)", g, counts[g], groupSize)
		}
	}
	if counts[groupCount-1] != groupSize {
		t.Fatalf("newest group %d has %d events, want %d", groupCount-1, counts[groupCount-1], groupSize)
	}
}
