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
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Ensure schema exists (create if not exists)
	if err := store.ensureSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot ensure PostgreSQL schema: %w", err)
	}

	// Create tables
	if err := store.ensureTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot create PostgreSQL tables: %w", err)
	}

	return store, nil
}

// ensureSchema creates the schema if it doesn't exist and grants usage/creation
// privileges to the current user. This is called before ensureTables to ensure
// the target schema is available.
func (s *PGStore) ensureSchema() error {
	if s.schema == "public" {
		return nil
	}
	// Create schema if not exists
	if _, err := s.db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, s.schema)); err != nil {
		return fmt.Errorf("cannot create schema %s: %w", s.schema, err)
	}
	// Grant usage and create privileges on the schema to the current user
	if _, err := s.db.Exec(fmt.Sprintf(`GRANT USAGE, CREATE ON SCHEMA %s TO CURRENT_USER`, s.schema)); err != nil {
		return fmt.Errorf("cannot grant privileges on schema %s: %w", s.schema, err)
	}
	return nil
}

// ensureTables creates all required tables if they don't exist.
func (s *PGStore) ensureTables() error {
	tables := []string{
		// History table
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.history (
			id TEXT PRIMARY KEY,
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
	// Generate a timestamp-based key matching bbolt's format
	key := fmt.Sprintf("%020d", time.Now().UnixNano())
	query := fmt.Sprintf(`INSERT INTO %s.history (id, input) VALUES ($1, $2)`, s.schema)
	_, err := s.db.Exec(query, key, input)
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
// For each table, it first checks the maximum key/ID already in PostgreSQL,
// then only migrates entries with keys greater than that maximum.
// This allows the migration to be safely re-run to pick up new data.
// Tables are migrated in parallel using goroutines for maximum throughput.
func (s *PGStore) MigrateFromBolt(boltStore *Store) error {
	type migrateResult struct {
		name string
		err  error
	}

	results := make(chan migrateResult, 7)

	// Launch all migrations in parallel
	go func() { results <- migrateResult{"history", s.migrateHistory(boltStore)} }()
	go func() { results <- migrateResult{"context", s.migrateContext(boltStore)} }()
	go func() { results <- migrateResult{"schedules", s.migrateSchedules(boltStore)} }()
	go func() { results <- migrateResult{"taskplans", s.migrateTaskPlans(boltStore)} }()
	go func() { results <- migrateResult{"memory", s.migrateMemory(boltStore)} }()
	go func() { results <- migrateResult{"token_usage", s.migrateTokenUsage(boltStore)} }()
	go func() { results <- migrateResult{"session", s.migrateSession(boltStore)} }()

	// Collect results
	var firstErr error
	for i := 0; i < 7; i++ {
		r := <-results
		if r.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", r.name, r.err)
		}
	}

	return firstErr
}

// migrateHistory migrates history entries that are not yet in PostgreSQL.
// History keys in bbolt are timestamp strings (%020d = nanoseconds).
// PG uses the same key as the primary key (TEXT), so we compare by key string.
func (s *PGStore) migrateHistory(boltStore *Store) error {
	// Find the maximum key already in PG
	var maxKey string
	query := fmt.Sprintf(`SELECT COALESCE(MAX(id), '') FROM %s.history`, s.schema)
	s.db.QueryRow(query).Scan(&maxKey)

	entries, err := boltStore.ListHistoryWithKeys()
	if err != nil {
		return fmt.Errorf("cannot load history from bolt: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("✅ history: 无数据")
		return nil
	}

	// Filter entries with key > maxKey
	var toMigrate []HistoryEntryWithKey
	for _, entry := range entries {
		if entry.Key > maxKey {
			toMigrate = append(toMigrate, entry)
		}
	}

	if len(toMigrate) == 0 {
		fmt.Println("✅ history: 无新增数据")
		return nil
	}

	fmt.Printf("✅ history: 迁移 %d 条记录\n", len(toMigrate))

	// Batch insert using multi-row VALUES for maximum throughput
	batchSize := 500
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

		// Build multi-row INSERT: VALUES ($1,$2,$3),($4,$5,$6),...
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s.history (id, input, created_at) VALUES `, s.schema))
		args := make([]interface{}, 0, len(batch)*3)
		for j, entry := range batch {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 3
			sb.WriteString(fmt.Sprintf("($%d,$%d,$%d)", base+1, base+2, base+3))
			args = append(args, entry.Key, entry.Input, entry.Timestamp)
		}
		sb.WriteString(" ON CONFLICT (id) DO NOTHING")

		if _, err := s.db.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("cannot batch insert history: %w", err)
		}
	}

	return nil
}

// migrateContext migrates context entries (full table replace).
func (s *PGStore) migrateContext(boltStore *Store) error {
	// Clear existing data
	if _, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s.context`, s.schema)); err != nil {
		return fmt.Errorf("cannot clear context: %w", err)
	}

	contextKeys := []string{"current_context", "system_prompt", "user_preferences"}
	migrated := 0
	for _, key := range contextKeys {
		data, found, err := boltStore.GetContext(key)
		if err != nil {
			return fmt.Errorf("cannot load context %s from bolt: %w", key, err)
		}
		if found {
			if err := s.SaveContext(key, data); err != nil {
				return fmt.Errorf("cannot save context %s to pg: %w", key, err)
			}
			migrated++
		}
	}
	if migrated > 0 {
		fmt.Printf("✅ context: 迁移 %d 条记录\n", migrated)
	} else {
		fmt.Println("✅ context: 无数据")
	}
	return nil
}

// migrateSchedules migrates schedule entries (full table replace).
func (s *PGStore) migrateSchedules(boltStore *Store) error {
	// Clear existing data
	if _, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s.schedules`, s.schema)); err != nil {
		return fmt.Errorf("cannot clear schedules: %w", err)
	}

	schedules, err := boltStore.LoadSchedules()
	if err != nil {
		return fmt.Errorf("cannot load schedules from bolt: %w", err)
	}

	if len(schedules) == 0 {
		fmt.Println("✅ schedules: 无数据")
		return nil
	}

	// Collect all entries
	var toMigrate []struct {
		id   int
		data []byte
	}
	for id, data := range schedules {
		toMigrate = append(toMigrate, struct {
			id   int
			data []byte
		}{id, data})
	}

	fmt.Printf("✅ schedules: 迁移 %d 条记录\n", len(toMigrate))

	// Batch insert using multi-row VALUES
	batchSize := 500
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s.schedules (id, data) VALUES `, s.schema))
		args := make([]interface{}, 0, len(batch)*2)
		for j, entry := range batch {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 2
			sb.WriteString(fmt.Sprintf("($%d,$%d)", base+1, base+2))
			args = append(args, entry.id, entry.data)
		}
		sb.WriteString(" ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data")

		if _, err := s.db.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("cannot batch insert schedules: %w", err)
		}
	}
	return nil
}

// migrateTaskPlans migrates task plan entries (full table replace).
func (s *PGStore) migrateTaskPlans(boltStore *Store) error {
	// Clear existing data
	if _, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s.taskplans`, s.schema)); err != nil {
		return fmt.Errorf("cannot clear taskplans: %w", err)
	}

	taskPlans, err := boltStore.ListTaskPlans()
	if err != nil {
		return fmt.Errorf("cannot load task plans from bolt: %w", err)
	}

	if len(taskPlans) == 0 {
		fmt.Println("✅ taskplans: 无数据")
		return nil
	}

	var toMigrate []struct {
		id   int
		data []byte
	}
	for id, data := range taskPlans {
		toMigrate = append(toMigrate, struct {
			id   int
			data []byte
		}{id, data})
	}

	fmt.Printf("✅ taskplans: 迁移 %d 条记录\n", len(toMigrate))

	batchSize := 500
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s.taskplans (id, data) VALUES `, s.schema))
		args := make([]interface{}, 0, len(batch)*2)
		for j, entry := range batch {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 2
			sb.WriteString(fmt.Sprintf("($%d,$%d)", base+1, base+2))
			args = append(args, entry.id, entry.data)
		}
		sb.WriteString(" ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data")

		if _, err := s.db.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("cannot batch insert taskplans: %w", err)
		}
	}
	return nil
}

// migrateMemory migrates memory entries with keys greater than the max in PG.
func (s *PGStore) migrateMemory(boltStore *Store) error {
	var maxKey string
	query := fmt.Sprintf(`SELECT COALESCE(MAX(key), '') FROM %s.memory`, s.schema)
	s.db.QueryRow(query).Scan(&maxKey)

	memoryEntries, err := boltStore.SearchMemory("")
	if err != nil {
		return fmt.Errorf("cannot load memory from bolt: %w", err)
	}
	if len(memoryEntries) == 0 {
		fmt.Println("✅ memory: 无数据")
		return nil
	}

	// Filter entries with key > maxKey
	var toMigrate []MemoryEntry
	for _, entry := range memoryEntries {
		if entry.Key > maxKey {
			toMigrate = append(toMigrate, entry)
		}
	}

	if len(toMigrate) == 0 {
		fmt.Println("✅ memory: 无新增数据")
		return nil
	}

	fmt.Printf("✅ memory: 迁移 %d 条记录\n", len(toMigrate))

	batchSize := 500
	for i := 0; i < len(memoryEntries); i += batchSize {
		end := i + batchSize
		if end > len(memoryEntries) {
			end = len(memoryEntries)
		}
		batch := memoryEntries[i:end]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s.memory (key, value) VALUES `, s.schema))
		args := make([]interface{}, 0, len(batch)*2)
		for j, entry := range batch {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 2
			sb.WriteString(fmt.Sprintf("($%d,$%d)", base+1, base+2))
			args = append(args, entry.Key, entry.Value)
		}
		sb.WriteString(" ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value")

		if _, err := s.db.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("cannot batch insert memory: %w", err)
		}
	}

	return nil
}

// migrateTokenUsage migrates token usage entries with IDs greater than the max in PG.
func (s *PGStore) migrateTokenUsage(boltStore *Store) error {
	var maxID string
	query := fmt.Sprintf(`SELECT COALESCE(MAX(id), '') FROM %s.token_usage`, s.schema)
	s.db.QueryRow(query).Scan(&maxID)

	tokenUsage, err := boltStore.ListTokenUsage()
	if err != nil {
		return fmt.Errorf("cannot load token usage from bolt: %w", err)
	}

	var toMigrate []TokenUsageEntry
	for _, entry := range tokenUsage {
		if entry.ID > maxID {
			toMigrate = append(toMigrate, entry)
		}
	}

	if len(toMigrate) == 0 {
		fmt.Println("✅ token_usage: 无新增数据")
		return nil
	}

	fmt.Printf("✅ token_usage: 迁移 %d 条记录\n", len(toMigrate))

	batchSize := 500
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s.token_usage (id, prompt_tokens, completion_tokens, total_tokens, recorded_at) VALUES `, s.schema))
		args := make([]interface{}, 0, len(batch)*5)
		for j, entry := range batch {
			if j > 0 {
				sb.WriteString(",")
			}
			base := j * 5
			sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5))
			args = append(args, entry.ID, entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens, entry.Timestamp)
		}
		sb.WriteString(" ON CONFLICT (id) DO NOTHING")

		if _, err := s.db.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("cannot batch insert token_usage: %w", err)
		}
	}
	return nil
}

// migrateSession migrates the session entry (fixed key "current", UPSERT).
func (s *PGStore) migrateSession(boltStore *Store) error {
	sessionData, found, err := boltStore.LoadSession()
	if err != nil {
		return fmt.Errorf("cannot load session from bolt: %w", err)
	}
	if !found {
		fmt.Println("✅ session: 无数据")
		return nil
	}

	var sd SessionData
	if err := json.Unmarshal(sessionData, &sd); err != nil {
		return fmt.Errorf("cannot unmarshal session data: %w", err)
	}
	if err := s.SaveSession(sd.Messages); err != nil {
		return fmt.Errorf("cannot save session to pg: %w", err)
	}
	fmt.Println("✅ session: 迁移 1 条记录")
	return nil
}

// DropTables drops all tables in the current schema.
func (s *PGStore) DropTables() error {
	tables := []string{"history", "context", "schedules", "taskplans", "memory", "token_usage", "sessions"}
	for _, table := range tables {
		query := fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s CASCADE`, s.schema, table)
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("cannot drop table %s: %w", table, err)
		}
	}
	return nil
}

// RecreateTables recreates all tables (calls ensureTables).
func (s *PGStore) RecreateTables() error {
	return s.ensureTables()
}

// DSN returns the connection string (with password masked) for display purposes.
func (s *PGStore) DSN() string {
	return fmt.Sprintf("postgresql://%s:%d/%s (schema: %s)", "localhost", 5432, "coshell_db", s.schema)
}

// BackupToCSV exports all tables to CSV files in the specified directory.
// Each table gets its own CSV file named <table>.csv.
func (s *PGStore) BackupToCSV(dir string) error {
	tables := []string{"history", "context", "schedules", "taskplans", "memory", "token_usage", "sessions"}
	for _, table := range tables {
		query := fmt.Sprintf(`SELECT * FROM %s.%s`, s.schema, table)
		rows, err := s.db.Query(query)
		if err != nil {
			return fmt.Errorf("cannot query %s: %w", table, err)
		}

		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return fmt.Errorf("cannot get columns for %s: %w", table, err)
		}

		filePath := filepath.Join(dir, table+".csv")
		f, err := os.Create(filePath)
		if err != nil {
			rows.Close()
			return fmt.Errorf("cannot create %s: %w", filePath, err)
		}

		// Write CSV header
		if _, err := fmt.Fprintln(f, strings.Join(columns, ",")); err != nil {
			f.Close()
			rows.Close()
			return fmt.Errorf("cannot write header for %s: %w", table, err)
		}

		// Write rows
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		rowCount := 0
		for rows.Next() {
			if err := rows.Scan(valuePtrs...); err != nil {
				f.Close()
				rows.Close()
				return fmt.Errorf("cannot scan row in %s: %w", table, err)
			}

			// Format values as CSV
			line := make([]string, len(columns))
			for i, v := range values {
				if v == nil {
					line[i] = ""
				} else {
					switch val := v.(type) {
					case []byte:
						// Escape special characters for CSV
						str := string(val)
						if strings.ContainsAny(str, ",\"\n") {
							str = `"` + strings.ReplaceAll(str, `"`, `""`) + `"`
						}
						line[i] = str
					case string:
						if strings.ContainsAny(val, ",\"\n") {
							val = `"` + strings.ReplaceAll(val, `"`, `""`) + `"`
						}
						line[i] = val
					default:
						line[i] = fmt.Sprintf("%v", val)
					}
				}
			}
			if _, err := fmt.Fprintln(f, strings.Join(line, ",")); err != nil {
				f.Close()
				rows.Close()
				return fmt.Errorf("cannot write row in %s: %w", table, err)
			}
			rowCount++
		}
		f.Close()
		rows.Close()

		if err := rows.Err(); err != nil {
			return fmt.Errorf("rows iteration error in %s: %w", table, err)
		}

		fmt.Printf("✅ %s: %d 条记录\n", table, rowCount)
	}
	return nil
}

// RestoreFromCSV imports data from CSV files in the specified directory.
// It clears each table before importing.
func (s *PGStore) RestoreFromCSV(dir string) error {
	tables := []string{"history", "context", "schedules", "taskplans", "memory", "token_usage", "sessions"}
	for _, table := range tables {
		filePath := filepath.Join(dir, table+".csv")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Printf("✅ %s: 无备份文件，跳过\n", table)
			continue
		}

		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", filePath, err)
		}

		// Read header
		reader := bufio.NewReader(f)
		headerLine, err := reader.ReadString('\n')
		if err != nil {
			f.Close()
			return fmt.Errorf("cannot read header from %s: %w", filePath, err)
		}
		columns := strings.Split(strings.TrimRight(headerLine, "\r\n"), ",")

		// Clear existing data
		if _, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s.%s`, s.schema, table)); err != nil {
			f.Close()
			return fmt.Errorf("cannot clear %s: %w", table, err)
		}

		// Build INSERT statement with placeholders
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		insertSQL := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s)`,
			s.schema, table,
			strings.Join(columns, ","),
			strings.Join(placeholders, ","))

		// Read and insert rows
		rowCount := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break // EOF
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				continue
			}

			// Parse CSV line (simple parser, handles quoted fields)
			values := parseCSVLine(line)
			if len(values) != len(columns) {
				f.Close()
				return fmt.Errorf("column count mismatch in %s: expected %d, got %d", filePath, len(columns), len(values))
			}

			args := make([]interface{}, len(values))
			for i, v := range values {
				args[i] = v
			}

			if _, err := s.db.Exec(insertSQL, args...); err != nil {
				f.Close()
				return fmt.Errorf("cannot insert row into %s: %w", table, err)
			}
			rowCount++
		}
		f.Close()

		fmt.Printf("✅ %s: %d 条记录\n", table, rowCount)
	}
	return nil
}

// parseCSVLine parses a single CSV line, handling quoted fields.
func parseCSVLine(line string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuotes {
			if c == '"' {
				if i+1 < len(line) && line[i+1] == '"' {
					current.WriteByte('"')
					i++
				} else {
					inQuotes = false
				}
			} else {
				current.WriteByte(c)
			}
		} else {
			if c == '"' {
				inQuotes = true
			} else if c == ',' {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteByte(c)
			}
		}
	}
	result = append(result, current.String())
	return result
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
