// Author: L.Shuang
// Created: 2026-04-26
// Last Modified: 2026-04-26
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

// Package scheduler provides a cron-like task scheduler for co-shell.
//
// It supports simple 5-field cron expressions (minute hour day month weekday)
// and manages scheduled tasks that launch sub-agents at specified times.
// Tasks are persisted in the bbolt store and survive restarts.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// CronEntry represents a single scheduled task.
type CronEntry struct {
	// ID is the unique identifier for this task.
	ID int `json:"id"`

	// Name is a human-readable name for this task.
	Name string `json:"name"`

	// CronExpr is the 5-field cron expression (minute hour day month weekday).
	CronExpr string `json:"cron_expr"`

	// Instruction is the instruction to pass to the sub-agent when triggered.
	Instruction string `json:"instruction"`

	// NextRun is the next scheduled run time.
	NextRun time.Time `json:"next_run"`

	// LastRun is the last time this task was triggered.
	LastRun time.Time `json:"last_run"`

	// Running indicates whether the task is currently executing.
	Running bool `json:"running"`

	// Paused indicates whether the task is paused.
	Paused bool `json:"paused"`

	// CreatedAt is when this task was created.
	CreatedAt time.Time `json:"created_at"`

	// RunCount is the number of times this task has been triggered.
	RunCount int `json:"run_count"`
}

// TaskCallback is called when a scheduled task is triggered.
// The callback receives the task entry and should return when execution completes.
type TaskCallback func(entry *CronEntry)

// Scheduler manages cron-like scheduled tasks.
type Scheduler struct {
	mu       sync.RWMutex
	entries  map[int]*CronEntry
	nextID   int
	running  bool
	stopCh   chan struct{}
	callback TaskCallback
}

// New creates a new Scheduler.
func New(callback TaskCallback) *Scheduler {
	return &Scheduler{
		entries:  make(map[int]*CronEntry),
		nextID:   1,
		stopCh:   make(chan struct{}),
		callback: callback,
	}
}

// Start begins the scheduler loop in a background goroutine.
// It checks every second for tasks that need to be triggered.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.loop()
	log.Info("Scheduler started")
}

// Stop stops the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	log.Info("Scheduler stopped")
}

// Add adds a new scheduled task and returns its ID.
// The cron expression uses 5 fields: minute hour day month weekday.
// Each field supports * (any) or a specific number.
func (s *Scheduler) Add(name, cronExpr, instruction string) (int, error) {
	// Validate cron expression
	if err := validateCron(cronExpr); err != nil {
		return 0, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}

	// Calculate next run time
	nextRun, err := nextRunTime(cronExpr, time.Now())
	if err != nil {
		return 0, fmt.Errorf("cannot calculate next run time: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	entry := &CronEntry{
		ID:          id,
		Name:        name,
		CronExpr:    cronExpr,
		Instruction: instruction,
		NextRun:     nextRun,
		CreatedAt:   time.Now(),
	}

	s.entries[id] = entry
	log.Info("Scheduled task #%d (%s): cron=%s, nextRun=%s", id, name, cronExpr, nextRun.Format(time.RFC3339))
	return id, nil
}

// Remove removes a scheduled task by ID.
func (s *Scheduler) Remove(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.entries[id]
	if exists {
		delete(s.entries, id)
		log.Info("Removed scheduled task #%d", id)
	}
	return exists
}

// Pause pauses a scheduled task.
func (s *Scheduler) Pause(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[id]
	if exists {
		entry.Paused = true
		log.Info("Paused scheduled task #%d (%s)", id, entry.Name)
	}
	return exists
}

// Resume resumes a paused scheduled task.
func (s *Scheduler) Resume(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[id]
	if exists {
		entry.Paused = false
		// Recalculate next run time
		entry.NextRun, _ = nextRunTime(entry.CronExpr, time.Now())
		log.Info("Resumed scheduled task #%d (%s)", id, entry.Name)
	}
	return exists
}

// List returns all scheduled tasks.
func (s *Scheduler) List() []*CronEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CronEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		// Return a copy to avoid race conditions
		e := *entry
		result = append(result, &e)
	}
	return result
}

// Get returns a specific task by ID.
func (s *Scheduler) Get(id int) *CronEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[id]
	if !exists {
		return nil
	}
	e := *entry
	return &e
}

// LoadEntries replaces all entries (used when restoring from persistent storage).
func (s *Scheduler) LoadEntries(entries []*CronEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make(map[int]*CronEntry)
	maxID := 0
	for _, entry := range entries {
		s.entries[entry.ID] = entry
		if entry.ID > maxID {
			maxID = entry.ID
		}
	}
	s.nextID = maxID + 1
	log.Info("Loaded %d scheduled tasks from storage", len(entries))
}

// GetEntriesForStorage returns all entries for persistent storage.
func (s *Scheduler) GetEntriesForStorage() []*CronEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CronEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		e := *entry
		result = append(result, &e)
	}
	return result
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// loop is the main scheduler loop that runs in a background goroutine.
func (s *Scheduler) loop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkAndTrigger(now)
		}
	}
}

// checkAndTrigger checks all tasks and triggers any that are due.
func (s *Scheduler) checkAndTrigger(now time.Time) {
	s.mu.RLock()
	// Collect entries that need triggering
	var toTrigger []*CronEntry
	for _, entry := range s.entries {
		if entry.Paused {
			continue
		}
		if entry.Running {
			// Skip if already running - don't start a new instance
			continue
		}
		if now.After(entry.NextRun) || now.Equal(entry.NextRun) {
			e := *entry // copy
			toTrigger = append(toTrigger, &e)
		}
	}
	s.mu.RUnlock()

	// Trigger each due task
	for _, entry := range toTrigger {
		s.triggerTask(entry)
	}
}

// triggerTask marks a task as running and calls the callback.
func (s *Scheduler) triggerTask(entry *CronEntry) {
	s.mu.Lock()
	// Double-check the entry still exists and is not already running
	current, exists := s.entries[entry.ID]
	if !exists || current.Running || current.Paused {
		s.mu.Unlock()
		return
	}
	current.Running = true
	current.LastRun = time.Now()
	current.RunCount++
	// Calculate next run time
	current.NextRun, _ = nextRunTime(current.CronExpr, current.LastRun)
	s.mu.Unlock()

	log.Info("Triggering scheduled task #%d (%s)", entry.ID, entry.Name)

	// Call the callback (this blocks until sub-agent completes)
	if s.callback != nil {
		s.callback(current)
	}

	// Mark as not running
	s.mu.Lock()
	if current, exists := s.entries[entry.ID]; exists {
		current.Running = false
	}
	s.mu.Unlock()

	log.Info("Scheduled task #%d (%s) completed", entry.ID, entry.Name)
}

// validateCron validates a 5-field cron expression.
func validateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("expected 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	validators := []struct {
		name  string
		min   int
		max   int
		value string
	}{
		{"minute", 0, 59, fields[0]},
		{"hour", 0, 23, fields[1]},
		{"day", 1, 31, fields[2]},
		{"month", 1, 12, fields[3]},
		{"weekday", 0, 6, fields[4]},
	}

	for _, v := range validators {
		if v.value == "*" {
			continue
		}
		num, err := strconv.Atoi(v.value)
		if err != nil {
			return fmt.Errorf("invalid %s value %q: not a number or *", v.name, v.value)
		}
		if num < v.min || num > v.max {
			return fmt.Errorf("%s value %d out of range [%d, %d]", v.name, num, v.min, v.max)
		}
	}

	return nil
}

// nextRunTime calculates the next time a cron expression should run after a given time.
// It uses a simple algorithm: try each minute up to 60*24*31 minutes ahead (about a month).
func nextRunTime(expr string, after time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("expected 5 fields")
	}

	cronMin := fields[0]
	cronHour := fields[1]
	cronDay := fields[2]
	cronMonth := fields[3]
	cronWeekday := fields[4]

	// Start checking from the next minute
	t := after.Truncate(time.Minute).Add(1 * time.Minute)

	// Check up to 60*24*31 minutes ahead (about a month)
	for i := 0; i < 60*24*31; i++ {
		if matchesCron(t, cronMin, cronHour, cronDay, cronMonth, cronWeekday) {
			return t, nil
		}
		t = t.Add(1 * time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within the next month for expression %q", expr)
}

// matchesCron checks if a given time matches the cron fields.
func matchesCron(t time.Time, min, hour, day, month, weekday string) bool {
	if !matchField(min, t.Minute()) {
		return false
	}
	if !matchField(hour, t.Hour()) {
		return false
	}
	if !matchField(day, t.Day()) {
		return false
	}
	if !matchField(month, int(t.Month())) {
		return false
	}
	if !matchField(weekday, int(t.Weekday())) {
		return false
	}
	return true
}

// matchField checks if a value matches a cron field (supports * or exact number).
func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}
	num, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	return num == value
}

// FormatNextRun returns a human-readable string for the next run time.
func FormatNextRun(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	now := time.Now()
	diff := t.Sub(now)
	if diff < 0 {
		return "已过期"
	}
	if diff < 1*time.Hour {
		return fmt.Sprintf("%.0f 分钟后", diff.Minutes())
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%.0f 小时后", diff.Hours())
	}
	return t.Format("01-02 15:04")
}
