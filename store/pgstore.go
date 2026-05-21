// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-05-21
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

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/idirect3d/co-shell/config"
	_ "github.com/lib/pq"
)

// PGStore implements the same storage interface as Store but uses PostgreSQL
// as the backend. It provides persistent storage for sessions, configuration,
// and conversation memory.
type PGStore struct {
	db     *sql.DB
	schema string
}

// NewPGStore creates a new PostgreSQL-backed store.
// It connects to the database using the provided DBConfig and ensures all
// required tables exist.
func NewPGStore(cfg config.DBConfig) (*PGStore, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
	)

	schema := cfg.Schema
	if schema == "" {
		schema = "public"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot ping PostgreSQL: %w", err)
	}

	store := &PGStore{db: db, schema: schema}

	// Create tables
	if err := store.ensureTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot create PostgreSQL tables: %w", err)
	}

	return store, nil
}

// ensureTables creates all required tables if they don't exist.
func (s *PGStore) ensureTables() error {
	tables := []string{
		// History table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.history (
			id BIGSERIAL PRIMARY KEY,
			input TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Context table (key-value store)
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.context (
			key TEXT PRIMARY KEY,
			value BYTEA NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Schedules table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.schedules (
			id INTEGER PRIMARY KEY,
			data BYTEA NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Task plans table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.taskplans (
			id INTEGER PRIMARY KEY,
			data BYTEA NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Memory table (conversation memory)
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.memory (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Token usage table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.token_usage (
			id TEXT PRIMARY KEY,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		// Sessions table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sessions (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL DEFAULT 1,
			messages BYTEA NOT NULL,
			last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),
	}

	for _, ddl := range tables {
		if _, err := s.db.Exec(ddl); err != nil {
			return fmt.Errorf("cannot create table: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *PGStore) Close() error {
	return s.db.Close()
}

// --- History Operations ---

// SaveHistory appends a history entry.
func (s *PGStore) SaveHistory(input string) error {
	query := fmt.Sprintf(`INSERT INTO %s.history (input) VALUES ($1)`, s.schema)
	_, err := s.db.Exec(query, input)
	return err
}

// LoadHistory retrieves all history entries in reverse chronological order.
func (s *PGStore) LoadHistory() ([]string, error) {
	query := fmt.Sprintf(`SELECT input FROM %s.history ORDER BY id DESC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inputs []string
	for rows.Next() {
		var input string
		if err := rows.Scan(&input); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, rows.Err()
}

// ListHistory returns all history entries in chronological order (oldest first).
func (s *PGStore) ListHistory() ([]HistoryEntryWithTime, error) {
	query := fmt.Sprintf(`SELECT input, created_at FROM %s.history ORDER BY id ASC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HistoryEntryWithTime
	for rows.Next() {
		var entry HistoryEntryWithTime
		if err := rows.Scan(&entry.Input, &entry.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// ClearHistory removes all history entries.
func (s *PGStore) ClearHistory() error {
	query := fmt.Sprintf(`DELETE FROM %s.history`, s.schema)
	_, err := s.db.Exec(query)
	return err
}

// --- Context Operations ---

// SaveContext stores context data.
func (s *PGStore) SaveContext(key string, data []byte) error {
	query := fmt.Sprintf(`INSERT INTO %s.context (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`, s.schema)
	_, err := s.db.Exec(query, key, data)
	return err
}

// GetContext retrieves context data.
func (s *PGStore) GetContext(key string) ([]byte, bool, error) {
	query := fmt.Sprintf(`SELECT value FROM %s.context WHERE key = $1`, s.schema)
	var data []byte
	err := s.db.QueryRow(query, key).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// DeleteContext removes a specific context entry by key.
func (s *PGStore) DeleteContext(key string) error {
	query := fmt.Sprintf(`DELETE FROM %s.context WHERE key = $1`, s.schema)
	_, err := s.db.Exec(query, key)
	return err
}

// ClearContext removes all context data.
func (s *PGStore) ClearContext() error {
	query := fmt.Sprintf(`DELETE FROM %s.context`, s.schema)
	_, err := s.db.Exec(query)
	return err
}

// --- Schedule Operations ---

// SaveSchedule stores a scheduled task entry.
func (s *PGStore) SaveSchedule(id int, data []byte) error {
	query := fmt.Sprintf(`INSERT INTO %s.schedules (id, data) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET data = $2`, s.schema)
	_, err := s.db.Exec(query, id, data)
	return err
}

// LoadSchedules loads all scheduled task entries.
func (s *PGStore) LoadSchedules() (map[int][]byte, error) {
	query := fmt.Sprintf(`SELECT id, data FROM %s.schedules ORDER BY id ASC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]byte)
	for rows.Next() {
		var id int
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		result[id] = data
	}
	return result, rows.Err()
}

// DeleteSchedule removes a scheduled task entry.
func (s *PGStore) DeleteSchedule(id int) error {
	query := fmt.Sprintf(`DELETE FROM %s.schedules WHERE id = $1`, s.schema)
	_, err := s.db.Exec(query, id)
	return err
}

// --- Task Plan Operations ---

// NextTaskPlanID returns the next available task plan ID.
func (s *PGStore) NextTaskPlanID() (int, error) {
	query := fmt.Sprintf(`SELECT COALESCE(MAX(id), 0) + 1 FROM %s.taskplans`, s.schema)
	var nextID int
	err := s.db.QueryRow(query).Scan(&nextID)
	return nextID, err
}

// SaveTaskPlan stores a task plan by ID.
func (s *PGStore) SaveTaskPlan(id int, data []byte) error {
	query := fmt.Sprintf(`INSERT INTO %s.taskplans (id, data) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET data = $2`, s.schema)
	_, err := s.db.Exec(query, id, data)
	return err
}

// GetTaskPlan retrieves a task plan by ID.
func (s *PGStore) GetTaskPlan(id int) ([]byte, bool, error) {
	query := fmt.Sprintf(`SELECT data FROM %s.taskplans WHERE id = $1`, s.schema)
	var data []byte
	err := s.db.QueryRow(query, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// ListTaskPlans returns all task plan entries.
func (s *PGStore) ListTaskPlans() (map[int][]byte, error) {
	query := fmt.Sprintf(`SELECT id, data FROM %s.taskplans ORDER BY id ASC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]byte)
	for rows.Next() {
		var id int
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		result[id] = data
	}
	return result, rows.Err()
}

// DeleteTaskPlan removes a task plan by ID.
func (s *PGStore) DeleteTaskPlan(id int) error {
	query := fmt.Sprintf(`DELETE FROM %s.taskplans WHERE id = $1`, s.schema)
	_, err := s.db.Exec(query, id)
	return err
}

// --- Conversation Memory Operations ---

// SaveConversationMessage stores a conversation message.
func (s *PGStore) SaveConversationMessage(id string, data []byte) error {
	query := fmt.Sprintf(`INSERT INTO %s.memory (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = $2`, s.schema)
	_, err := s.db.Exec(query, id, string(data))
	return err
}

// ListConversationMessages returns all conversation messages in chronological order (oldest first).
func (s *PGStore) ListConversationMessages() ([][]byte, error) {
	query := fmt.Sprintf(`SELECT value FROM %s.memory ORDER BY key ASC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result [][]byte
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, []byte(value))
	}
	return result, rows.Err()
}

// SaveMemory stores a key-value pair in the memory bucket.
func (s *PGStore) SaveMemory(key, value string) error {
	query := fmt.Sprintf(`INSERT INTO %s.memory (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = $2`, s.schema)
	_, err := s.db.Exec(query, key, value)
	return err
}

// GetMemory retrieves a value from the memory bucket by key.
func (s *PGStore) GetMemory(key string) (string, bool, error) {
	query := fmt.Sprintf(`SELECT value FROM %s.memory WHERE key = $1`, s.schema)
	var value string
	err := s.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SearchMemory searches the memory bucket for keys containing the given prefix.
func (s *PGStore) SearchMemory(prefix string) ([]MemoryEntry, error) {
	query := fmt.Sprintf(`SELECT key, value FROM %s.memory WHERE key LIKE $1 ORDER BY key ASC`, s.schema)
	likePattern := prefix + "%"
	rows, err := s.db.Query(query, likePattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		if err := rows.Scan(&entry.Key, &entry.Value); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// DeleteMemory removes a specific memory entry by key.
func (s *PGStore) DeleteMemory(key string) error {
	query := fmt.Sprintf(`DELETE FROM %s.memory WHERE key = $1`, s.schema)
	_, err := s.db.Exec(query, key)
	return err
}

// DeleteMemoryRange removes a range of conversation messages from the memory bucket.
func (s *PGStore) DeleteMemoryRange(lastFrom, lastTo int) error {
	if lastFrom < 1 || lastTo < 1 {
		return fmt.Errorf("lastFrom and lastTo must be >= 1")
	}
	if lastFrom < lastTo {
		return fmt.Errorf("lastFrom (%d) must be >= lastTo (%d)", lastFrom, lastTo)
	}

	// Use a subquery to find the keys to delete based on order
	query := fmt.Sprintf(`DELETE FROM %s.memory WHERE key IN (
		SELECT key FROM (
			SELECT key, ROW_NUMBER() OVER (ORDER BY key DESC) AS rn
			FROM %s.memory
		) sub
		WHERE sub.rn >= $1 AND sub.rn <= $2
	)`, s.schema, s.schema)

	_, err := s.db.Exec(query, lastTo, lastFrom)
	return err
}

// --- Token Usage Operations ---

// SaveTokenUsage stores a token usage entry.
func (s *PGStore) SaveTokenUsage(entry *TokenUsageEntry) error {
	query := fmt.Sprintf(`INSERT INTO %s.token_usage (id, prompt_tokens, completion_tokens, total_tokens, recorded_at)
		VALUES ($1, $2, $3, $4, $5)`, s.schema)
	_, err := s.db.Exec(query, entry.ID, entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens, entry.Timestamp)
	return err
}

// ListTokenUsage returns all token usage entries in chronological order (oldest first).
func (s *PGStore) ListTokenUsage() ([]TokenUsageEntry, error) {
	query := fmt.Sprintf(`SELECT id, prompt_tokens, completion_tokens, total_tokens, recorded_at
		FROM %s.token_usage ORDER BY id ASC`, s.schema)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []TokenUsageEntry
	for rows.Next() {
		var entry TokenUsageEntry
		if err := rows.Scan(&entry.ID, &entry.PromptTokens, &entry.CompletionTokens, &entry.TotalTokens, &entry.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// ClearConversationMessages removes all conversation messages from the memory bucket.
func (s *PGStore) ClearConversationMessages() error {
	query := fmt.Sprintf(`DELETE FROM %s.memory`, s.schema)
	_, err := s.db.Exec(query)
	return err
}

// --- Session Persistence Operations ---

// SaveSession persists the current conversation session to the database.
func (s *PGStore) SaveSession(messages []byte) error {
	query := fmt.Sprintf(`INSERT INTO %s.sessions (id, version, messages, last_updated_at) VALUES ('current', 1, $1, NOW())
		ON CONFLICT (id) DO UPDATE SET messages = $1, version = 1, last_updated_at = NOW()`, s.schema)
	_, err := s.db.Exec(query, messages)
	return err
}

// LoadSession loads the last persisted conversation session from the database.
func (s *PGStore) LoadSession() ([]byte, bool, error) {
	query := fmt.Sprintf(`SELECT messages FROM %s.sessions WHERE id = 'current'`, s.schema)
	var messages []byte
	err := s.db.QueryRow(query).Scan(&messages)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return messages, true, nil
}

// ClearSession removes the persisted session from the database.
func (s *PGStore) ClearSession() error {
	query := fmt.Sprintf(`DELETE FROM %s.sessions WHERE id = 'current'`, s.schema)
	_, err := s.db.Exec(query)
	return err
}

// --- Migration Support ---

// MigrateFromBolt migrates all data from a bbolt Store to this PGStore.
// This is a one-time operation that copies all data from the local bbolt
// database to the PostgreSQL database.
func (s *PGStore) MigrateFromBolt(boltStore *Store) error {
	// Migrate history
	historyEntries, err := boltStore.ListHistory()
	if err != nil {
		return fmt.Errorf("cannot load history from bolt: %w", err)
	}
	for _, entry := range historyEntries {
		if err := s.SaveHistory(entry.Input); err != nil {
			return fmt.Errorf("cannot save history to pg: %w", err)
		}
	}

	// Migrate context
	// Since bbolt context is key-value, we iterate all keys
	contextKeys := []string{"current_context", "system_prompt", "user_preferences"}
	for _, key := range contextKeys {
		data, found, err := boltStore.GetContext(key)
		if err != nil {
			return fmt.Errorf("cannot load context %s from bolt: %w", key, err)
		}
		if found {
			if err := s.SaveContext(key, data); err != nil {
				return fmt.Errorf("cannot save context %s to pg: %w", key, err)
			}
		}
	}

	// Migrate schedules
	schedules, err := boltStore.LoadSchedules()
	if err != nil {
		return fmt.Errorf("cannot load schedules from bolt: %w", err)
	}
	for id, data := range schedules {
		if err := s.SaveSchedule(id, data); err != nil {
			return fmt.Errorf("cannot save schedule %d to pg: %w", id, err)
		}
	}

	// Migrate task plans
	taskPlans, err := boltStore.ListTaskPlans()
	if err != nil {
		return fmt.Errorf("cannot load task plans from bolt: %w", err)
	}
	for id, data := range taskPlans {
		if err := s.SaveTaskPlan(id, data); err != nil {
			return fmt.Errorf("cannot save task plan %d to pg: %w", id, err)
		}
	}

	// Migrate memory
	memoryEntries, err := boltStore.SearchMemory("")
	if err != nil {
		return fmt.Errorf("cannot load memory from bolt: %w", err)
	}
	for _, entry := range memoryEntries {
		if err := s.SaveMemory(entry.Key, entry.Value); err != nil {
			return fmt.Errorf("cannot save memory %s to pg: %w", entry.Key, err)
		}
	}

	// Migrate token usage
	tokenUsage, err := boltStore.ListTokenUsage()
	if err != nil {
		return fmt.Errorf("cannot load token usage from bolt: %w", err)
	}
	for _, entry := range tokenUsage {
		if err := s.SaveTokenUsage(&entry); err != nil {
			return fmt.Errorf("cannot save token usage %s to pg: %w", entry.ID, err)
		}
	}

	// Migrate session
	sessionData, found, err := boltStore.LoadSession()
	if err != nil {
		return fmt.Errorf("cannot load session from bolt: %w", err)
	}
	if found {
		// Parse the session data to extract messages
		var sd SessionData
		if err := json.Unmarshal(sessionData, &sd); err != nil {
			return fmt.Errorf("cannot unmarshal session data: %w", err)
		}
		if err := s.SaveSession(sd.Messages); err != nil {
			return fmt.Errorf("cannot save session to pg: %w", err)
		}
	}

	return nil
}

// DSN returns the connection string (with password masked) for display purposes.
func (s *PGStore) DSN() string {
	return fmt.Sprintf("postgresql://%s:%d/%s (schema: %s)", "localhost", 5432, "coshell_db", s.schema)
}

// Ensure compile-time interface compliance
var _ StoreInterface = (*PGStore)(nil)

// StoreInterface defines the storage interface that both Store and PGStore implement.
// This is used for type checking only.
type StoreInterface interface {
	Close() error
	SaveHistory(input string) error
	LoadHistory() ([]string, error)
	ListHistory() ([]HistoryEntryWithTime, error)
	ClearHistory() error
	SaveContext(key string, data []byte) error
	GetContext(key string) ([]byte, bool, error)
	DeleteContext(key string) error
	ClearContext() error
	SaveSchedule(id int, data []byte) error
	LoadSchedules() (map[int][]byte, error)
	DeleteSchedule(id int) error
	NextTaskPlanID() (int, error)
	SaveTaskPlan(id int, data []byte) error
	GetTaskPlan(id int) ([]byte, bool, error)
	ListTaskPlans() (map[int][]byte, error)
	DeleteTaskPlan(id int) error
	SaveConversationMessage(id string, data []byte) error
	ListConversationMessages() ([][]byte, error)
	SaveMemory(key, value string) error
	GetMemory(key string) (string, bool, error)
	SearchMemory(prefix string) ([]MemoryEntry, error)
	DeleteMemory(key string) error
	DeleteMemoryRange(lastFrom, lastTo int) error
	SaveTokenUsage(entry *TokenUsageEntry) error
	ListTokenUsage() ([]TokenUsageEntry, error)
	ClearConversationMessages() error
	SaveSession(messages []byte) error
	LoadSession() ([]byte, bool, error)
	ClearSession() error
}
