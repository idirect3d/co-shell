// Package store - FIX-509 regression test.
//
// Author: L.Shuang
// Created: 2026-09-12
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

// TestAppendEventSeqContinuesWithLaterSessionPresent reproduces FIX-509: when
// another session's keys sort AFTER this session's prefix, AppendEvent must
// still continue this session's own sequence.
//
// Before the fix the prefix seek returned the *other* session's key, the prefix
// check failed, nextSeq fell back to 1, and every subsequent event overwrote
// seq=1. The oldest event was silently destroyed and the replayed history (read
// back in seq order) no longer matched what the browser had shown live.
func TestAppendEventSeqContinuesWithLaterSessionPresent(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-a"
	const later = "sess-z"

	// The later-sorting session exists first: its keys make the session's own
	// keys no longer the tail of the bucket.
	if err := s.AppendEvent(later, eventJSON(1, "z1")); err != nil {
		t.Fatalf("AppendEvent(%s): %v", later, err)
	}

	for i := 1; i <= 3; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, fmt.Sprintf("a%d", i))); err != nil {
			t.Fatalf("AppendEvent(%s, %d): %v", sid, i, err)
		}
	}

	entries, hasMore, err := s.LoadEvents(sid, 10, 0)
	if err != nil {
		t.Fatalf("LoadEvents(%s): %v", sid, err)
	}
	if hasMore {
		t.Errorf("hasMore = true, want false")
	}
	if len(entries) != 3 {
		t.Fatalf("session %s kept %d events, want 3 (new events overwrote seq=1)", sid, len(entries))
	}
	for i, e := range entries {
		if e.Seq != i+1 {
			t.Fatalf("entries[%d].Seq = %d, want %d", i, e.Seq, i+1)
		}
	}
}

// TestAppendEventSeqContinuesWhenSessionIsLastInBucket guards the other half of
// the same logic: when the session's keys really are the tail of the bucket,
// the sequence must keep growing as well.
func TestAppendEventSeqContinuesWhenSessionIsLastInBucket(t *testing.T) {
	s := newTestStore(t)
	const sid = "sess-z"

	for i := 1; i <= 3; i++ {
		if err := s.AppendEvent(sid, eventJSON(i, fmt.Sprintf("z%d", i))); err != nil {
			t.Fatalf("AppendEvent(%s, %d): %v", sid, i, err)
		}
	}

	entries, _, err := s.LoadEvents(sid, 10, 0)
	if err != nil {
		t.Fatalf("LoadEvents(%s): %v", sid, err)
	}
	if len(entries) != 3 {
		t.Fatalf("session %s kept %d events, want 3", sid, len(entries))
	}
	for i, e := range entries {
		if e.Seq != i+1 {
			t.Fatalf("entries[%d].Seq = %d, want %d", i, e.Seq, i+1)
		}
	}
}
