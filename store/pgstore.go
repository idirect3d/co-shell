// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-06-08
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/config"
	_ "github.com/lib/pq"
)

// PGStore implements memory and history storage using PostgreSQL.
// Only used for dual-write of memory and history data alongside bbolt.
type PGStore struct {
	db     *sql.DB
	schema string
}

// NewPGStore creates a new PostgreSQL-backed store.
// It connects to the database using the provided DBConfig and ensures all
// required tables exist (memory and history only).
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

	// Create tables (memory and history only)
	if err := store.ensureTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot create PostgreSQL tables: %w", err)
	}

	return store, nil
}

// ensureSchema creates the schema if it doesn't exist and grants usage/creation
// privileges to the current user.
func (s *PGStore) ensureSchema() error {
	if s.schema == "public" {
		return nil
	}
	if _, err := s.db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, s.schema)); err != nil {
		return fmt.Errorf("cannot create schema %s: %w", s.schema, err)
	}
	if _, err := s.db.Exec(fmt.Sprintf(`GRANT USAGE, CREATE ON SCHEMA %s TO CURRENT_USER`, s.schema)); err != nil {
		return fmt.Errorf("cannot grant privileges on schema %s: %w", s.schema, err)
	}
	return nil
}

// ensureTables creates only memory and history tables.
func (s *PGStore) ensureTables() error {
	tables := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.history (
			id TEXT PRIMARY KEY,
			input TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, s.schema),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.memory (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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

// ClearConversationMessages removes all conversation messages from the memory bucket.
func (s *PGStore) ClearConversationMessages() error {
	query := fmt.Sprintf(`DELETE FROM %s.memory`, s.schema)
	_, err := s.db.Exec(query)
	return err
}

// --- Migration Support ---

// MigrateFromBolt migrates memory and history data from a bbolt Store to this PGStore.
// Uses incremental migration: only entries with keys greater than the max in PG are migrated.
func (s *PGStore) MigrateFromBolt(boltStore *Store) error {
	type migrateResult struct {
		name string
		err  error
	}

	results := make(chan migrateResult, 2)

	go func() { results <- migrateResult{"history", s.migrateHistory(boltStore)} }()
	go func() { results <- migrateResult{"memory", s.migrateMemory(boltStore)} }()

	var firstErr error
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", r.name, r.err)
		}
	}

	return firstErr
}

// migrateHistory migrates history entries that are not yet in PostgreSQL.
func (s *PGStore) migrateHistory(boltStore *Store) error {
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

	batchSize := 500
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

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
	for i := 0; i < len(toMigrate); i += batchSize {
		end := i + batchSize
		if end > len(toMigrate) {
			end = len(toMigrate)
		}
		batch := toMigrate[i:end]

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

// DropTables drops only memory and history tables.
func (s *PGStore) DropTables() error {
	tables := []string{"history", "memory"}
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

// BackupToCSV exports memory and history tables to CSV files in the specified directory.
func (s *PGStore) BackupToCSV(dir string) error {
	tables := []string{"history", "memory"}
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

		if _, err := fmt.Fprintln(f, strings.Join(columns, ",")); err != nil {
			f.Close()
			rows.Close()
			return fmt.Errorf("cannot write header for %s: %w", table, err)
		}

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

			var csvRow []string
			for _, val := range values {
				switch v := val.(type) {
				case []byte:
					csvRow = append(csvRow, string(v))
				case string:
					csvRow = append(csvRow, v)
				case int64:
					csvRow = append(csvRow, fmt.Sprintf("%d", v))
				case float64:
					csvRow = append(csvRow, fmt.Sprintf("%v", v))
				case bool:
					csvRow = append(csvRow, fmt.Sprintf("%t", v))
				case time.Time:
					csvRow = append(csvRow, v.Format(time.RFC3339))
				case nil:
					csvRow = append(csvRow, "")
				default:
					csvRow = append(csvRow, fmt.Sprintf("%v", v))
				}
			}

			// Escape CSV values (quote if contains comma, quote, or newline)
			for i, v := range csvRow {
				if strings.ContainsAny(v, ",\"\n") {
					csvRow[i] = `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
				}
			}

			if _, err := fmt.Fprintln(f, strings.Join(csvRow, ",")); err != nil {
				f.Close()
				rows.Close()
				return fmt.Errorf("cannot write row to %s: %w", table, err)
			}
			rowCount++
		}
		rows.Close()
		f.Close()

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error reading rows from %s: %w", table, err)
		}
	}

	return nil
}

// RestoreFromCSV restores memory and history data from CSV files in the specified directory.
func (s *PGStore) RestoreFromCSV(dir string) error {
	tables := []string{"history", "memory"}
	for _, table := range tables {
		filePath := filepath.Join(dir, table+".csv")
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", filePath, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)

		// Read header line
		if !scanner.Scan() {
			return fmt.Errorf("empty CSV file: %s", filePath)
		}
		header := scanner.Text()
		columns := strings.Split(header, ",")

		// Build INSERT statement
		colNames := make([]string, len(columns))
		placeholders := make([]string, len(columns))
		for i, col := range columns {
			colNames[i] = strings.TrimSpace(col)
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		insertQuery := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s) ON CONFLICT DO NOTHING`,
			s.schema, table, strings.Join(colNames, ","), strings.Join(placeholders, ","))

		// Parse CSV rows and insert
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("cannot begin transaction for %s: %w", table, err)
		}

		stmt, err := tx.Prepare(insertQuery)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("cannot prepare insert for %s: %w", table, err)
		}

		rowCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			values := parseCSVLine(line, len(columns))
			if values == nil {
				continue
			}

			args := make([]interface{}, len(values))
			for i, v := range values {
				args[i] = v
			}

			if _, err := stmt.Exec(args...); err != nil {
				tx.Rollback()
				return fmt.Errorf("cannot insert row into %s: %w (line: %s)", table, err, line)
			}
			rowCount++
		}
		defer stmt.Close()

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("cannot commit transaction for %s: %w", table, err)
		}
	}

	return nil
}

// parseCSVLine parses a single CSV line into values.
func parseCSVLine(line string, expectedCols int) []string {
	var values []string
	current := ""
	inQuote := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuote && i+1 < len(line) && line[i+1] == '"' {
				current += `"`
				i++
			} else {
				inQuote = !inQuote
			}
		} else if ch == ',' && !inQuote {
			values = append(values, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	values = append(values, current)

	if len(values) != expectedCols {
		return nil
	}
	return values
}
