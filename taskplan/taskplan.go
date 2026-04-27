// Author: L.Shuang
// Created: 2026-04-28
// Last Modified: 2026-04-28
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

// Package taskplan provides task plan management for co-shell.
// It allows the LLM to create, track, adjust, and view multi-step task plans.
package taskplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/store"
)

// TaskStatus represents the status of a task step.
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
	StatusCancelled  TaskStatus = "cancelled"
)

// TaskStep represents a single step in a task plan.
type TaskStep struct {
	ID          int        `json:"id"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Note        string     `json:"note,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// TaskPlan represents a complete task plan with multiple steps.
type TaskPlan struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Steps       []TaskStep `json:"steps"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// Manager handles task plan CRUD operations.
type Manager struct {
	store *store.Store
}

// NewManager creates a new TaskPlan manager.
func NewManager(s *store.Store) *Manager {
	return &Manager{store: s}
}

// Create creates a new task plan and returns it.
func (m *Manager) Create(title, description string, steps []string) (*TaskPlan, error) {
	now := time.Now().Format("2006-01-02 15:04:05")

	plan := &TaskPlan{
		Title:       title,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	for i, stepDesc := range steps {
		plan.Steps = append(plan.Steps, TaskStep{
			ID:          i + 1,
			Description: stepDesc,
			Status:      StatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Get next ID
	nextID, err := m.store.NextTaskPlanID()
	if err != nil {
		return nil, fmt.Errorf("cannot get next task plan ID: %w", err)
	}
	plan.ID = nextID

	// Save to store
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal task plan: %w", err)
	}

	if err := m.store.SaveTaskPlan(nextID, data); err != nil {
		return nil, fmt.Errorf("cannot save task plan: %w", err)
	}

	return plan, nil
}

// Get retrieves a task plan by ID.
func (m *Manager) Get(id int) (*TaskPlan, error) {
	data, found, err := m.store.GetTaskPlan(id)
	if err != nil {
		return nil, fmt.Errorf("cannot get task plan: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("task plan #%d not found", id)
	}

	var plan TaskPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("cannot unmarshal task plan: %w", err)
	}
	return &plan, nil
}

// UpdateStepStatus updates the status of a specific step in a task plan.
func (m *Manager) UpdateStepStatus(planID, stepID int, status TaskStatus, note string) (*TaskPlan, error) {
	plan, err := m.Get(planID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	found := false
	for i := range plan.Steps {
		if plan.Steps[i].ID == stepID {
			plan.Steps[i].Status = status
			plan.Steps[i].UpdatedAt = now
			if note != "" {
				plan.Steps[i].Note = note
			}
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("step #%d not found in task plan #%d", stepID, planID)
	}

	plan.UpdatedAt = now

	// Persist
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal task plan: %w", err)
	}
	if err := m.store.SaveTaskPlan(planID, data); err != nil {
		return nil, fmt.Errorf("cannot save task plan: %w", err)
	}

	return plan, nil
}

// RemoveSteps removes steps from a task plan by step ID range (from, to inclusive).
// Steps before and including from-1 are preserved. Steps from `from` to `to` are removed.
// Steps after `to` are renumbered.
// IMPORTANT: no completed steps can be removed.
// Returns an error if any step in the range is completed.
func (m *Manager) RemoveSteps(planID, from, to int) (*TaskPlan, error) {
	plan, err := m.Get(planID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Validate range
	if from < 1 || to < from || to > len(plan.Steps) {
		return nil, fmt.Errorf("invalid step range [%d, %d] in task plan #%d (total steps: %d)", from, to, planID, len(plan.Steps))
	}

	// Validate that no completed steps are being removed
	for i := from - 1; i < to; i++ {
		if plan.Steps[i].Status == StatusCompleted {
			return nil, fmt.Errorf("cannot remove completed step #%d (%q) in task plan #%d: completed steps cannot be modified",
				plan.Steps[i].ID, plan.Steps[i].Description, planID)
		}
	}

	// Remove steps [from-1, to)
	plan.Steps = append(plan.Steps[:from-1], plan.Steps[to:]...)

	// Re-assign sequential IDs
	for i := range plan.Steps {
		plan.Steps[i].ID = i + 1
	}

	plan.UpdatedAt = now

	// Persist
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal task plan: %w", err)
	}
	if err := m.store.SaveTaskPlan(planID, data); err != nil {
		return nil, fmt.Errorf("cannot save task plan: %w", err)
	}

	return plan, nil
}

// InsertStepsAfter inserts new steps after the specified step ID in a task plan.
// The step ID refers to the step after which new steps will be inserted.
// If afterStepID is 0, new steps are inserted at the beginning.
// IMPORTANT: there must be no completed steps after the insertion point.
// Returns an error if any completed step exists after the insertion point.
func (m *Manager) InsertStepsAfter(planID, afterStepID int, newStepDescriptions []string) (*TaskPlan, error) {
	plan, err := m.Get(planID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Find the insertion index
	insertIndex := 0 // default: insert at beginning
	if afterStepID > 0 {
		found := false
		for i, s := range plan.Steps {
			if s.ID == afterStepID {
				insertIndex = i + 1 // insert after this step
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("step #%d not found in task plan #%d", afterStepID, planID)
		}
	}

	// Validate that there are no completed steps after the insertion point
	for i := insertIndex; i < len(plan.Steps); i++ {
		if plan.Steps[i].Status == StatusCompleted {
			return nil, fmt.Errorf("cannot insert steps after step #%d in task plan #%d: step #%d (%q) after the insertion point is already completed. Completed steps cannot be reordered",
				afterStepID, planID, plan.Steps[i].ID, plan.Steps[i].Description)
		}
	}

	// Create new step entries
	var newSteps []TaskStep
	for _, desc := range newStepDescriptions {
		newSteps = append(newSteps, TaskStep{
			Description: desc,
			Status:      StatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Insert new steps at the insertion point
	before := make([]TaskStep, insertIndex)
	copy(before, plan.Steps[:insertIndex])
	after := make([]TaskStep, len(plan.Steps)-insertIndex)
	copy(after, plan.Steps[insertIndex:])

	plan.Steps = append(before, append(newSteps, after...)...)

	// Re-assign sequential IDs
	for i := range plan.Steps {
		plan.Steps[i].ID = i + 1
	}

	plan.UpdatedAt = now

	// Persist
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal task plan: %w", err)
	}
	if err := m.store.SaveTaskPlan(planID, data); err != nil {
		return nil, fmt.Errorf("cannot save task plan: %w", err)
	}

	return plan, nil
}

// List returns all task plans.
func (m *Manager) List() ([]*TaskPlan, error) {
	entries, err := m.store.ListTaskPlans()
	if err != nil {
		return nil, fmt.Errorf("cannot list task plans: %w", err)
	}

	var plans []*TaskPlan
	for _, data := range entries {
		var plan TaskPlan
		if err := json.Unmarshal(data, &plan); err != nil {
			continue // skip corrupted entries
		}
		plans = append(plans, &plan)
	}
	return plans, nil
}

// FormatPlan formats a task plan as a human-readable string.
func FormatPlan(plan *TaskPlan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Task Plan #%d: %s\n", plan.ID, plan.Title))
	if plan.Description != "" {
		sb.WriteString(fmt.Sprintf("   Description: %s\n", plan.Description))
	}
	sb.WriteString(fmt.Sprintf("   Created: %s\n", plan.CreatedAt))
	sb.WriteString(fmt.Sprintf("   Updated: %s\n", plan.UpdatedAt))

	// Calculate progress
	total := len(plan.Steps)
	completed := 0
	for _, s := range plan.Steps {
		if s.Status == StatusCompleted {
			completed++
		}
	}
	progress := 0
	if total > 0 {
		progress = completed * 100 / total
	}
	sb.WriteString(fmt.Sprintf("   Progress: %d/%d (%d%%)\n\n", completed, total, progress))

	if total == 0 {
		sb.WriteString("   (No steps)\n")
		return sb.String()
	}

	for _, step := range plan.Steps {
		statusIcon := statusIcon(step.Status)
		sb.WriteString(fmt.Sprintf("  %s Step #%d: %s\n", statusIcon, step.ID, step.Description))
		if step.Note != "" {
			sb.WriteString(fmt.Sprintf("       Note: %s\n", step.Note))
		}
	}

	return sb.String()
}

// FormatPlanList formats a list of task plans as a human-readable string.
func FormatPlanList(plans []*TaskPlan) string {
	if len(plans) == 0 {
		return "📋 No task plans yet."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Task Plans (%d total):\n\n", len(plans)))
	for _, plan := range plans {
		total := len(plan.Steps)
		completed := 0
		for _, s := range plan.Steps {
			if s.Status == StatusCompleted {
				completed++
			}
		}
		progress := 0
		if total > 0 {
			progress = completed * 100 / total
		}
		sb.WriteString(fmt.Sprintf("  #%d [%d%%] %s (%d/%d steps)\n",
			plan.ID, progress, plan.Title, completed, total))
	}
	return sb.String()
}

// statusIcon returns an icon for the given task status.
func statusIcon(status TaskStatus) string {
	switch status {
	case StatusPending:
		return "⬜"
	case StatusInProgress:
		return "🔄"
	case StatusCompleted:
		return "✅"
	case StatusFailed:
		return "❌"
	case StatusCancelled:
		return "🚫"
	default:
		return "⬜"
	}
}
