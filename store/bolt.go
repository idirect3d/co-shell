// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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
	"time"

	"github.com/idirect3d/co-shell/workspace"
	"go.etcd.io/bbolt"
)

// MemoryEntry represents a single memory entry.
type MemoryEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionEntry represents a conversation session entry.
type SessionEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []byte    `json:"messages"` // JSON-encoded messages
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HistoryEntry represents a single history entry.
type HistoryEntry struct {
	Input     string    `json:"input"`
	Timestamp time.Time `json:"timestamp"`
}

// Store provides persistent storage for memory, sessions, and configuration.
type Store struct {
	db *bbolt.DB
}

// NewStore creates or opens the bolt database in the workspace db/ directory.
func NewStore(ws *workspace.Workspace) (*Store, error) {
	dbPath := ws.DBPath()
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	// Create buckets
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range []string{"memory", "sessions", "context", "history", "schedules"} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("cannot create bucket %s: %w", bucket, err)
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- History Operations ---

const maxHistoryEntries = 1000

// SaveHistory appends a history entry to the history bucket.
func (s *Store) SaveHistory(input string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("history"))

		// Generate a sequential key using timestamp + counter
		key := fmt.Sprintf("%020d", time.Now().UnixNano())

		entry := HistoryEntry{
			Input:     input,
			Timestamp: time.Now(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), data)
	})
}

// LoadHistory retrieves all history entries in reverse chronological order.
func (s *Store) LoadHistory() ([]string, error) {
	var inputs []string
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("history"))
		cursor := bucket.Cursor()

		// Iterate in reverse order (newest first)
		for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
			var entry HistoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue // skip corrupted entries
			}
			inputs = append(inputs, entry.Input)
		}
		return nil
	})
	return inputs, err
}

// HistoryEntryWithTime represents a history entry with its timestamp.
type HistoryEntryWithTime struct {
	Input     string    `json:"input"`
	Timestamp time.Time `json:"timestamp"`
}

// ListHistory returns all history entries in chronological order (oldest first).
func (s *Store) ListHistory() ([]HistoryEntryWithTime, error) {
	var entries []HistoryEntryWithTime
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("history"))
		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry HistoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue // skip corrupted entries
			}
			entries = append(entries, HistoryEntryWithTime{
				Input:     entry.Input,
				Timestamp: entry.Timestamp,
			})
		}
		return nil
	})
	return entries, err
}

// ClearHistory removes all history entries.
func (s *Store) ClearHistory() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("history"))
		return bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
	})
}

// --- Memory Operations ---

// SaveMemory stores a key-value pair in the memory bucket.
func (s *Store) SaveMemory(key, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		entry := MemoryEntry{
			Key:       key,
			Value:     value,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), data)
	})
}

// GetMemory retrieves a value by key from the memory bucket.
func (s *Store) GetMemory(key string) (string, bool, error) {
	var value string
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		data := bucket.Get([]byte(key))
		if data == nil {
			return nil
		}
		var entry MemoryEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}
		value = entry.Value
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return value, value != "", nil
}

// SearchMemory searches memory entries by key prefix.
func (s *Store) SearchMemory(prefix string) ([]MemoryEntry, error) {
	var entries []MemoryEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		cursor := bucket.Cursor()
		prefixBytes := []byte(prefix)
		for k, v := cursor.Seek(prefixBytes); k != nil && len(k) >= len(prefixBytes) && string(k[:len(prefixBytes)]) == prefix; k, v = cursor.Next() {
			var entry MemoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
		}
		return nil
	})
	return entries, err
}

// DeleteMemory removes a memory entry.
func (s *Store) DeleteMemory(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("memory")).Delete([]byte(key))
	})
}

// ListMemory returns all memory entries.
func (s *Store) ListMemory() ([]MemoryEntry, error) {
	var entries []MemoryEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		return bucket.ForEach(func(k, v []byte) error {
			var entry MemoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
			return nil
		})
	})
	return entries, err
}

// ClearMemory removes all memory entries.
func (s *Store) ClearMemory() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		return bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
	})
}

// --- Context Operations ---

// SaveContext stores context data.
func (s *Store) SaveContext(key string, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("context")).Put([]byte(key), data)
	})
}

// GetContext retrieves context data.
func (s *Store) GetContext(key string) ([]byte, bool, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		data = tx.Bucket([]byte("context")).Get([]byte(key))
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return data, data != nil, nil
}

// ClearContext removes all context data.
func (s *Store) ClearContext() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("context"))
		return bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
	})
}

// --- Schedule Operations ---

// ScheduleEntry represents a persisted scheduled task entry.
type ScheduleEntry struct {
	Data []byte `json:"data"`
}

// SaveSchedule stores a scheduled task entry.
func (s *Store) SaveSchedule(id int, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("schedules"))
		key := fmt.Sprintf("%010d", id)
		return bucket.Put([]byte(key), data)
	})
}

// LoadSchedules loads all scheduled task entries.
func (s *Store) LoadSchedules() (map[int][]byte, error) {
	result := make(map[int][]byte)
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("schedules"))
		return bucket.ForEach(func(k, v []byte) error {
			var id int
			if _, err := fmt.Sscanf(string(k), "%010d", &id); err != nil {
				return nil // skip corrupted keys
			}
			result[id] = v
			return nil
		})
	})
	return result, err
}

// DeleteSchedule removes a scheduled task entry.
func (s *Store) DeleteSchedule(id int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("schedules"))
		key := fmt.Sprintf("%010d", id)
		return bucket.Delete([]byte(key))
	})
}
