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
	"strings"

	"go.etcd.io/bbolt"
)

// eventStreamBucket is the bbolt bucket holding the per-session UI event stream
// (FEATURE-507). Events are persisted so the browser can replay history after a
// refresh without the backend keeping any history in memory.
const eventStreamBucket = "eventstream"

// maxEventStreamEntries caps the number of persisted events per session. When
// exceeded, the oldest events are dropped so a long-running session cannot grow
// the database without bound.
const maxEventStreamEntries = 5000

// eventStreamSep separates the session ID from the sequence number in a key.
// It cannot appear in a session ID (sess-YYYYMMDDhhmmss-xxxxxxxx).
const eventStreamSep = "\x00"

// EventStreamEntry is one persisted UI event together with its sequence number.
type EventStreamEntry struct {
	Seq  int             `json:"seq"`  // monotonically increasing per session
	Data json.RawMessage `json:"data"` // the StreamEvent JSON as sent to the browser
}

// eventStreamKey builds the bbolt key for a session event. The sequence number
// is zero-padded to 8 digits so bbolt's byte-ordered B+tree yields natural
// numeric ordering (same trick as formatMemoryKey).
func eventStreamKey(sessionID string, seq int) []byte {
	return []byte(fmt.Sprintf("%s%s%08d", sessionID, eventStreamSep, seq))
}

// eventStreamPrefix returns the key prefix shared by all events of a session.
func eventStreamPrefix(sessionID string) []byte {
	return []byte(sessionID + eventStreamSep)
}

// AppendEvent persists one UI event for the given session (FEATURE-507).
// The event is stored as-is (the same JSON the browser receives) so replay can
// feed it straight back into the frontend renderer. Nothing is kept in memory.
//
// When the session exceeds maxEventStreamEntries, the oldest entries are
// removed in the same transaction.
func (s *Store) AppendEvent(sessionID string, eventJSON []byte) error {
	if sessionID == "" || len(eventJSON) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(eventStreamBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", eventStreamBucket)
		}
		prefix := eventStreamPrefix(sessionID)

		// Determine the next sequence number from the last key of this session.
		// Seek to the first key past the session prefix, then step back once:
		// that lands on the session's highest sequence number.
		nextSeq := 1
		cursor := bucket.Cursor()
		last, _ := cursor.Seek(append(append([]byte(nil), prefix...), 0xFF))
		if last == nil {
			last, _ = cursor.Last()
		}
		if last != nil && strings.HasPrefix(string(last), string(prefix)) {
			if seq, err := parseEventSeq(last, prefix); err == nil {
				nextSeq = seq + 1
			}
		}

		if err := bucket.Put(eventStreamKey(sessionID, nextSeq), eventJSON); err != nil {
			return err
		}

		// Trim the oldest entries when the cap is exceeded.
		return trimEventStream(bucket, prefix)
	})
}

// parseEventSeq extracts the sequence number from a session event key.
func parseEventSeq(key, prefix []byte) (int, error) {
	rest := string(key[len(prefix):])
	var seq int
	if _, err := fmt.Sscanf(rest, "%d", &seq); err != nil {
		return 0, err
	}
	return seq, nil
}

// trimEventStream deletes the oldest events of a session until the count is
// within maxEventStreamEntries. It must be called inside an Update transaction.
func trimEventStream(bucket *bbolt.Bucket, prefix []byte) error {
	cursor := bucket.Cursor()
	var keys [][]byte
	for k, _ := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = cursor.Next() {
		keys = append(keys, append([]byte(nil), k...))
	}
	if len(keys) <= maxEventStreamEntries {
		return nil
	}
	for _, k := range keys[:len(keys)-maxEventStreamEntries] {
		if err := bucket.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// LoadEvents returns up to limit events of a session, newest-last (chronological
// order), reading directly from bbolt. When beforeSeq > 0 only events with a
// sequence number strictly lower than beforeSeq are considered (paging cursor).
//
// The returned entries are read one by one and never cached, so the backend
// keeps no history in memory.
func (s *Store) LoadEvents(sessionID string, limit, beforeSeq int) ([]EventStreamEntry, bool, error) {
	if sessionID == "" {
		return nil, false, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var entries []EventStreamEntry
	hasMore := false
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(eventStreamBucket))
		if bucket == nil {
			return nil
		}
		prefix := eventStreamPrefix(sessionID)
		cursor := bucket.Cursor()

		// Collect the session's keys in order, then take the tail window.
		var keys [][]byte
		for k, _ := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = cursor.Next() {
			if beforeSeq > 0 {
				if seq, err := parseEventSeq(k, prefix); err == nil && seq >= beforeSeq {
					break
				}
			}
			keys = append(keys, append([]byte(nil), k...))
		}
		if len(keys) > limit {
			hasMore = true
			keys = keys[len(keys)-limit:]
		}
		for _, k := range keys {
			v := bucket.Get(k)
			if v == nil {
				continue
			}
			seq, err := parseEventSeq(k, prefix)
			if err != nil {
				continue
			}
			entries = append(entries, EventStreamEntry{
				Seq:  seq,
				Data: append(json.RawMessage(nil), v...),
			})
		}
		return nil
	})
	return entries, hasMore, err
}

// DeleteEventsAfter removes all events of a session whose sequence number is
// greater than the given message index (FEATURE-507). It keeps the persisted
// event stream consistent with :session pop, which truncates the conversation
// back to a message index.
//
// Events carry their message index in meta.msg_index; the caller passes the
// target index and every event belonging to a later message is dropped.
func (s *Store) DeleteEventsAfter(sessionID string, msgIndex int) error {
	if sessionID == "" {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(eventStreamBucket))
		if bucket == nil {
			return nil
		}
		prefix := eventStreamPrefix(sessionID)
		cursor := bucket.Cursor()
		var doomed [][]byte
		for k, v := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = cursor.Next() {
			idx, ok := eventMsgIndex(v)
			if !ok {
				continue
			}
			if idx > msgIndex {
				doomed = append(doomed, append([]byte(nil), k...))
			}
		}
		for _, k := range doomed {
			if err := bucket.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// eventMsgIndex extracts meta.msg_index from a persisted event JSON.
func eventMsgIndex(eventJSON []byte) (int, bool) {
	var ev struct {
		Meta map[string]string `json:"meta"`
	}
	if err := json.Unmarshal(eventJSON, &ev); err != nil {
		return 0, false
	}
	raw, ok := ev.Meta["msg_index"]
	if !ok || raw == "" {
		return 0, false
	}
	var idx int
	if _, err := fmt.Sscanf(raw, "%d", &idx); err != nil {
		return 0, false
	}
	return idx, true
}

// ClearEventStream removes every persisted event of a session (FEATURE-507).
// Called when a session is deleted or reset.
func (s *Store) ClearEventStream(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(eventStreamBucket))
		if bucket == nil {
			return nil
		}
		prefix := eventStreamPrefix(sessionID)
		cursor := bucket.Cursor()
		var doomed [][]byte
		for k, _ := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = cursor.Next() {
			doomed = append(doomed, append([]byte(nil), k...))
		}
		for _, k := range doomed {
			if err := bucket.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
