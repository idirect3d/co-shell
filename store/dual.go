// Author: L.Shuang
// Created: 2026-06-08
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package store

import "fmt"

// DualStore wraps a bbolt Store and optionally a PGStore.
// All operations go through bbolt as the primary store.
// Memory and history operations are additionally written to PG when available.
// Other data (context, schedules, taskplans, sessions, token_usage) stay local only.
type DualStore struct {
	Bolt     *Store   // primary bbolt store
	pg       *PGStore // nil when PostgreSQL is not connected
	autoSync bool     // when true, memory/history reads come from PG (if connected)
}

// NewDualStore creates a new DualStore with bbolt as primary and PG as secondary.
// If pgStore is nil, it behaves exactly like a regular bbolt Store.
func NewDualStore(boltStore *Store, pgStore *PGStore) *DualStore {
	return &DualStore{
		Bolt: boltStore,
		pg:   pgStore,
	}
}

// NewDualStoreWithSync creates a DualStore with an initial autoSync setting.
func NewDualStoreWithSync(boltStore *Store, pgStore *PGStore, autoSync bool) *DualStore {
	return &DualStore{
		Bolt:     boltStore,
		pg:       pgStore,
		autoSync: autoSync,
	}
}

// SetAutoSync sets whether auto-sync is enabled. When true and PG is connected,
// memory and history reads will be served from PG. When false, reads always from bbolt.
func (d *DualStore) SetAutoSync(enabled bool) {
	d.autoSync = enabled
}

// readFromPG returns true if reads should go to PG (PG connected + autoSync enabled).
func (d *DualStore) readFromPG() bool {
	return d.pg != nil && d.autoSync
}

// PG returns the PGStore, or nil if not connected.
func (d *DualStore) PG() *PGStore {
	if d == nil {
		return nil
	}
	return d.pg
}

// Close closes both stores.
func (d *DualStore) Close() error {
	if d.pg != nil {
		d.pg.Close()
	}
	return d.Bolt.Close()
}

// --- History Operations (dual-write, dual-read when autoSync is on) ---

func (d *DualStore) SaveHistory(input string) error {
	if err := d.Bolt.SaveHistory(input); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.SaveHistory(input); err != nil {
			logWarn("Failed to save history to PG: %v", err)
		}
	}
	return nil
}

func (d *DualStore) LoadHistory() ([]string, error) {
	if d.readFromPG() {
		return d.pg.LoadHistory()
	}
	return d.Bolt.LoadHistory()
}

func (d *DualStore) ListHistory() ([]HistoryEntryWithTime, error) {
	if d.readFromPG() {
		return d.pg.ListHistory()
	}
	return d.Bolt.ListHistory()
}

func (d *DualStore) ClearHistory() error {
	if err := d.Bolt.ClearHistory(); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.ClearHistory(); err != nil {
			logWarn("Failed to clear history on PG: %v", err)
		}
	}
	return nil
}

// --- Context Operations (local only) ---

func (d *DualStore) SaveContext(key string, data []byte) error {
	return d.Bolt.SaveContext(key, data)
}

func (d *DualStore) GetContext(key string) ([]byte, bool, error) {
	return d.Bolt.GetContext(key)
}

func (d *DualStore) DeleteContext(key string) error {
	return d.Bolt.DeleteContext(key)
}

func (d *DualStore) ClearContext() error {
	return d.Bolt.ClearContext()
}

// --- Schedule Operations (local only) ---

func (d *DualStore) SaveSchedule(id int, data []byte) error {
	return d.Bolt.SaveSchedule(id, data)
}

func (d *DualStore) LoadSchedules() (map[int][]byte, error) {
	return d.Bolt.LoadSchedules()
}

func (d *DualStore) DeleteSchedule(id int) error {
	return d.Bolt.DeleteSchedule(id)
}

// --- Task Plan Operations (local only) ---

func (d *DualStore) NextTaskPlanID() (int, error) {
	return d.Bolt.NextTaskPlanID()
}

func (d *DualStore) SaveTaskPlan(id int, data []byte) error {
	return d.Bolt.SaveTaskPlan(id, data)
}

func (d *DualStore) GetTaskPlan(id int) ([]byte, bool, error) {
	return d.Bolt.GetTaskPlan(id)
}

func (d *DualStore) ListTaskPlans() (map[int][]byte, error) {
	return d.Bolt.ListTaskPlans()
}

func (d *DualStore) DeleteTaskPlan(id int) error {
	return d.Bolt.DeleteTaskPlan(id)
}

// --- Conversation Memory Operations (dual-write, dual-read when autoSync is on) ---

func (d *DualStore) SaveConversationMessage(id string, data []byte) error {
	if err := d.Bolt.SaveConversationMessage(id, data); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.SaveConversationMessage(id, data); err != nil {
			logWarn("Failed to save conversation message to PG: %v", err)
		}
	}
	return nil
}

func (d *DualStore) ListConversationMessages() ([][]byte, error) {
	if d.readFromPG() {
		return d.pg.ListConversationMessages()
	}
	return d.Bolt.ListConversationMessages()
}

func (d *DualStore) SaveMemory(key, value string) error {
	if err := d.Bolt.SaveMemory(key, value); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.SaveMemory(key, value); err != nil {
			logWarn("Failed to save memory to PG: %v", err)
		}
	}
	return nil
}

func (d *DualStore) GetMemory(key string) (string, bool, error) {
	if d.readFromPG() {
		return d.pg.GetMemory(key)
	}
	return d.Bolt.GetMemory(key)
}

func (d *DualStore) SearchMemory(prefix string) ([]MemoryEntry, error) {
	if d.readFromPG() {
		return d.pg.SearchMemory(prefix)
	}
	return d.Bolt.SearchMemory(prefix)
}

func (d *DualStore) DeleteMemory(key string) error {
	if err := d.Bolt.DeleteMemory(key); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.DeleteMemory(key); err != nil {
			logWarn("Failed to delete memory on PG: %v", err)
		}
	}
	return nil
}

func (d *DualStore) DeleteMemoryRange(lastFrom, lastTo int) error {
	if err := d.Bolt.DeleteMemoryRange(lastFrom, lastTo); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.DeleteMemoryRange(lastFrom, lastTo); err != nil {
			logWarn("Failed to delete memory range on PG: %v", err)
		}
	}
	return nil
}

func (d *DualStore) ClearConversationMessages() error {
	if err := d.Bolt.ClearConversationMessages(); err != nil {
		return err
	}
	if d.pg != nil {
		if err := d.pg.ClearConversationMessages(); err != nil {
			logWarn("Failed to clear conversation messages on PG: %v", err)
		}
	}
	return nil
}

// --- Token Usage Operations (local only) ---

func (d *DualStore) SaveTokenUsage(entry *TokenUsageEntry) error {
	return d.Bolt.SaveTokenUsage(entry)
}

func (d *DualStore) ListTokenUsage() ([]TokenUsageEntry, error) {
	return d.Bolt.ListTokenUsage()
}

// --- Session Operations (local only) ---

func (d *DualStore) SaveSession(messages []byte) error {
	return d.Bolt.SaveSession(messages)
}

func (d *DualStore) LoadSession() ([]byte, bool, error) {
	return d.Bolt.LoadSession()
}

func (d *DualStore) ClearSession() error {
	return d.Bolt.ClearSession()
}

func (d *DualStore) SaveCurrentSessionID(id string) error {
	return d.Bolt.SaveCurrentSessionID(id)
}

func (d *DualStore) LoadCurrentSessionID() (string, bool, error) {
	return d.Bolt.LoadCurrentSessionID()
}

func (d *DualStore) UpdateNamedSession(id string, entry *SessionEntry) error {
	return d.Bolt.UpdateNamedSession(id, entry)
}

func (d *DualStore) SaveNamedSession(entry *SessionEntry) error {
	return d.Bolt.SaveNamedSession(entry)
}

func (d *DualStore) ListNamedSessions() ([]SessionEntry, error) {
	return d.Bolt.ListNamedSessions()
}

func (d *DualStore) LoadNamedSession(id string) (*SessionEntry, bool, error) {
	return d.Bolt.LoadNamedSession(id)
}

func (d *DualStore) DeleteNamedSession(id string) error {
	return d.Bolt.DeleteNamedSession(id)
}

// Vault returns a VaultStore using the underlying bbolt database.
func (d *DualStore) Vault() *VaultStore {
	return d.Bolt.Vault()
}

// logWarn is a non-fatal warning logger.
func logWarn(format string, args ...interface{}) {
	_ = fmt.Sprintf(format, args...)
}
