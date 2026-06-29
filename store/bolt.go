// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-28
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
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/idirect3d/co-shell/workspace"
	"go.etcd.io/bbolt"
)

// memoryKeyNumRe matches the trailing numeric part of a memory key (e.g., "sub_agent:123").
var memoryKeyNumRe = regexp.MustCompile(`\d+$`)

// formatMemoryKey zero-pads the trailing numeric part of a key to 8 digits
// for natural sort order in BoltDB's B+tree.
// Example: "sub_agent:1" -> "sub_agent:00000001"
func formatMemoryKey(key string) string {
	loc := memoryKeyNumRe.FindStringIndex(key)
	if loc == nil {
		return key
	}
	numStr := key[loc[0]:loc[1]]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return key
	}
	return key[:loc[0]] + fmt.Sprintf("%08d", num)
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

// Store provides persistent storage for sessions, configuration, and conversation memory.
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
		for _, bucket := range []string{"sessions", "context", "history", "schedules", "taskplans", "memory"} {
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

// HistoryEntryWithKey represents a history entry with its bbolt key.
type HistoryEntryWithKey struct {
	Key       string
	Input     string
	Timestamp time.Time
}

// ListHistoryWithKeys returns all history entries with their bbolt keys in chronological order.
func (s *Store) ListHistoryWithKeys() ([]HistoryEntryWithKey, error) {
	var entries []HistoryEntryWithKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("history"))
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry HistoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}
			entries = append(entries, HistoryEntryWithKey{
				Key:       string(k),
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

// DeleteContext removes a specific context entry by key.
func (s *Store) DeleteContext(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("context")).Delete([]byte(key))
	})
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

// --- Task Plan Operations ---

// NextTaskPlanID returns the next available task plan ID.
func (s *Store) NextTaskPlanID() (int, error) {
	var maxID int
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("taskplans"))
		cursor := bucket.Cursor()
		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			var id int
			if _, err := fmt.Sscanf(string(k), "%010d", &id); err != nil {
				continue
			}
			if id > maxID {
				maxID = id
			}
		}
		return nil
	})
	return maxID + 1, err
}

// SaveTaskPlan stores a task plan by ID.
func (s *Store) SaveTaskPlan(id int, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("taskplans"))
		key := fmt.Sprintf("%010d", id)
		return bucket.Put([]byte(key), data)
	})
}

// GetTaskPlan retrieves a task plan by ID.
func (s *Store) GetTaskPlan(id int) ([]byte, bool, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("taskplans"))
		key := fmt.Sprintf("%010d", id)
		data = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return data, data != nil, nil
}

// ListTaskPlans returns all task plan entries.
func (s *Store) ListTaskPlans() (map[int][]byte, error) {
	result := make(map[int][]byte)
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("taskplans"))
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

// DeleteTaskPlan removes a task plan by ID.
func (s *Store) DeleteTaskPlan(id int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("taskplans"))
		key := fmt.Sprintf("%010d", id)
		return bucket.Delete([]byte(key))
	})
}

// --- Conversation Memory Operations ---

// SaveConversationMessage stores a conversation message in the memory bucket.
func (s *Store) SaveConversationMessage(id string, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		return bucket.Put([]byte(id), data)
	})
}

// ListConversationMessages returns all conversation messages in chronological order (oldest first).
func (s *Store) ListConversationMessages() ([][]byte, error) {
	var result [][]byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			// Make a copy of v since it's only valid within the transaction
			data := make([]byte, len(v))
			copy(data, v)
			result = append(result, data)
		}
		return nil
	})
	return result, err
}

// SaveMemory stores a key-value pair in the memory bucket.
// The key's trailing numeric part is zero-padded to 8 digits for natural sort order.
func (s *Store) SaveMemory(key, value string) error {
	formattedKey := formatMemoryKey(key)
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		return bucket.Put([]byte(formattedKey), []byte(value))
	})
}

// GetMemory retrieves a value from the memory bucket by key.
// Tries both the original key and the zero-padded format for backward compatibility.
func (s *Store) GetMemory(key string) (string, bool, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		// Try the key as-is first (for backward compatibility with old data)
		data = bucket.Get([]byte(key))
		if data == nil {
			// Try zero-padded format
			data = bucket.Get([]byte(formatMemoryKey(key)))
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	if data == nil {
		return "", false, nil
	}
	return string(data), true, nil
}

// SearchMemory searches the memory bucket for keys containing the given prefix.
// The prefix is also matched against zero-padded keys for backward compatibility.
type MemoryEntry struct {
	Key   string
	Value string
}

func (s *Store) SearchMemory(prefix string) ([]MemoryEntry, error) {
	var entries []MemoryEntry
	seen := make(map[string]bool) // deduplicate keys
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		cursor := bucket.Cursor()

		// Search with original prefix
		prefixBytes := []byte(prefix)
		for k, v := cursor.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, v = cursor.Next() {
			key := string(k)
			if !seen[key] {
				seen[key] = true
				entries = append(entries, MemoryEntry{
					Key:   key,
					Value: string(v),
				})
			}
		}

		// Also search with zero-padded prefix for backward compatibility
		// (old data stored with unpadded keys may not match the padded prefix)
		paddedPrefix := formatMemoryKey(prefix)
		if paddedPrefix != prefix {
			paddedBytes := []byte(paddedPrefix)
			for k, v := cursor.Seek(paddedBytes); k != nil && bytes.HasPrefix(k, paddedBytes); k, v = cursor.Next() {
				key := string(k)
				if !seen[key] {
					seen[key] = true
					entries = append(entries, MemoryEntry{
						Key:   key,
						Value: string(v),
					})
				}
			}
		}

		return nil
	})
	return entries, err
}

// DeleteMemory removes a specific memory entry by key.
func (s *Store) DeleteMemory(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		// Try the key as-is first (for backward compatibility with old data)
		if err := bucket.Delete([]byte(key)); err != nil {
			return err
		}
		// Also try zero-padded format
		if err := bucket.Delete([]byte(formatMemoryKey(key))); err != nil {
			return err
		}
		return nil
	})
}

// DeleteMemoryRange removes a range of conversation messages from the memory bucket.
// Parameters:
//   - lastFrom: starting position from the end (inclusive). 1 = most recent message.
//   - lastTo: ending position from the end (inclusive). 1 = most recent message.
//
// Example: lastFrom=5, lastTo=1 deletes the 5 most recent messages.
func (s *Store) DeleteMemoryRange(lastFrom, lastTo int) error {
	if lastFrom < 1 || lastTo < 1 {
		return fmt.Errorf("lastFrom and lastTo must be >= 1")
	}
	if lastFrom < lastTo {
		return fmt.Errorf("lastFrom (%d) must be >= lastTo (%d)", lastFrom, lastTo)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		cursor := bucket.Cursor()

		// Collect all keys in order
		var keys [][]byte
		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			keys = append(keys, keyCopy)
		}

		total := len(keys)
		if total == 0 {
			return nil
		}

		// Calculate slice boundaries (from end)
		startIdx := total - lastFrom
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := total - lastTo + 1
		if endIdx > total {
			endIdx = total
		}
		if startIdx >= endIdx {
			return nil
		}

		// Delete keys in the range
		for i := startIdx; i < endIdx; i++ {
			if err := bucket.Delete(keys[i]); err != nil {
				return fmt.Errorf("cannot delete memory key %q: %w", string(keys[i]), err)
			}
		}
		return nil
	})
}

// --- Token Usage Operations ---

// TokenUsageEntry records token usage for a single LLM API call.
type TokenUsageEntry struct {
	ID               string    `json:"id"`                // unique ID (timestamp-based)
	PromptTokens     int       `json:"prompt_tokens"`     // input tokens
	CompletionTokens int       `json:"completion_tokens"` // output tokens
	TotalTokens      int       `json:"total_tokens"`      // total tokens
	Timestamp        time.Time `json:"timestamp"`         // when the API call was made
}

// SaveTokenUsage stores a token usage entry in the token_usage bucket.
func (s *Store) SaveTokenUsage(entry *TokenUsageEntry) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("token_usage"))
		if err != nil {
			return fmt.Errorf("cannot create token_usage bucket: %w", err)
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("cannot marshal token usage entry: %w", err)
		}
		return bucket.Put([]byte(entry.ID), data)
	})
}

// ListTokenUsage returns all token usage entries in chronological order (oldest first).
func (s *Store) ListTokenUsage() ([]TokenUsageEntry, error) {
	var entries []TokenUsageEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("token_usage"))
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry TokenUsageEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue // skip corrupted entries
			}
			entries = append(entries, entry)
		}
		return nil
	})
	return entries, err
}

// ClearConversationMessages removes all conversation messages from the memory bucket.
func (s *Store) ClearConversationMessages() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("memory"))
		return bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
	})
}

// --- Session Persistence Operations ---

// SessionData represents a persistable conversation session.
type SessionData struct {
	// Version is the schema version of the session data (for future compatibility).
	Version int `json:"version"`
	// Messages is the list of conversation messages in order.
	// Uses json.RawMessage so the inner JSON array is stored as raw JSON, not base64-encoded.
	Messages json.RawMessage `json:"messages"` // JSON-encoded []llm.Message
	// LastUpdatedAt is the timestamp when this session was last persisted.
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// SaveSession persists the current conversation session to the database.
// It stores the session in the "sessions" bucket with key "current".
// Expects a complete, self-contained JSON blob (e.g., full SessionData serialization).
func (s *Store) SaveSession(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("sessions"))
		return bucket.Put([]byte("current"), data)
	})
}

// LoadSession loads the last persisted conversation session from the database.
// Returns the raw stored bytes, whether a session was found, and any error.
// IMPORTANT: Copies the data inside the transaction since bbolt's Get returns
// a reference to the mmap that may be invalidated after the transaction ends.
func (s *Store) LoadSession() ([]byte, bool, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("sessions"))
		if bucket == nil {
			return nil
		}
		ref := bucket.Get([]byte("current"))
		if ref != nil {
			data = make([]byte, len(ref))
			copy(data, ref)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return data, data != nil, nil
}

// ClearSession removes the persisted session from the database.
func (s *Store) ClearSession() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("sessions"))
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte("current"))
	})
}
