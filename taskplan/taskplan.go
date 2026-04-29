// Author: L.Shuang
// Created: 2026-04-28
// Last Modified: 2026-04-30
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
// It allows the LLM to create, track, adjust, and view a single current task plan (checklist).
// Only one active plan exists at a time. When a new plan is created, the old plan is
// automatically archived to conversation memory.
package taskplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/memory"
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

// currentPlanKey is the fixed key used to store the single current task plan.
const currentPlanKey = "current"

// Manager handles task plan CRUD operations.
// Only one active plan exists at a time.
type Manager struct {
	store       *store.Store
	memoryMgr   *memory.Manager
	planCounter int // monotonically increasing counter for plan IDs
}

// NewManager creates a new TaskPlan manager.
func NewManager(s *store.Store) *Manager {
	mgr := &Manager{
		store:     s,
		memoryMgr: memory.NewManager(s),
	}
	// Load the current plan to determine the counter
	plan, err := mgr.loadCurrent()
	if err == nil && plan != nil {
		mgr.planCounter = plan.ID
	}
	return mgr
}

// HasUnfinished returns true if there is a current plan with unfinished steps.
// Unfinished steps are those with status pending, in_progress, failed, or cancelled.
func (m *Manager) HasUnfinished() bool {
	plan, err := m.loadCurrent()
	if err != nil || plan == nil {
		return false
	}
	for _, step := range plan.Steps {
		if step.Status != StatusCompleted {
			return true
		}
	}
	return false
}

// GetCurrent returns the current task plan, or nil if none exists.
func (m *Manager) GetCurrent() (*TaskPlan, error) {
	return m.loadCurrent()
}

// Create creates a new task plan. If there is an existing plan with unfinished steps,
// it returns an error. If there is an existing plan (all completed), it is archived
// to memory before creating the new one.
func (m *Manager) Create(title, description string, steps []string) (*TaskPlan, error) {
	// Check for unfinished plan
	if m.HasUnfinished() {
		return nil, fmt.Errorf("当前还有未完成的任务计划，请先完成所有步骤或调整现有计划后再创建新计划")
	}

	// Archive existing plan to memory if it exists
	existing, err := m.loadCurrent()
	if err == nil && existing != nil && len(existing.Steps) > 0 {
		if err := m.archiveToMemory(existing); err != nil {
			return nil, fmt.Errorf("无法归档旧任务计划: %w", err)
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	m.planCounter++

	plan := &TaskPlan{
		ID:          m.planCounter,
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

	// Save to store
	if err := m.saveCurrent(plan); err != nil {
		return nil, fmt.Errorf("无法保存任务计划: %w", err)
	}

	return plan, nil
}

// UpdateStepStatus updates the status of a specific step in the current task plan.
func (m *Manager) UpdateStepStatus(stepID int, status TaskStatus, note string) (*TaskPlan, error) {
	plan, err := m.loadCurrent()
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("当前没有任务计划")
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
		return nil, fmt.Errorf("步骤 #%d 在当前任务计划中未找到", stepID)
	}

	plan.UpdatedAt = now

	// Persist
	if err := m.saveCurrent(plan); err != nil {
		return nil, fmt.Errorf("无法保存任务计划: %w", err)
	}

	return plan, nil
}

// RemoveSteps removes steps from the current task plan by step ID range (from, to inclusive).
// Steps before and including from-1 are preserved. Steps from `from` to `to` are removed.
// Steps after `to` are renumbered.
// IMPORTANT: no completed steps can be removed.
// Returns an error if any step in the range is completed.
func (m *Manager) RemoveSteps(from, to int) (*TaskPlan, error) {
	plan, err := m.loadCurrent()
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("当前没有任务计划")
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Validate range
	if from < 1 || to < from || to > len(plan.Steps) {
		return nil, fmt.Errorf("无效的步骤范围 [%d, %d]（总步骤数: %d）", from, to, len(plan.Steps))
	}

	// Validate that no completed steps are being removed
	for i := from - 1; i < to; i++ {
		if plan.Steps[i].Status == StatusCompleted {
			return nil, fmt.Errorf("无法删除已完成的步骤 #%d (%q)：已完成步骤不可修改",
				plan.Steps[i].ID, plan.Steps[i].Description)
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
	if err := m.saveCurrent(plan); err != nil {
		return nil, fmt.Errorf("无法保存任务计划: %w", err)
	}

	return plan, nil
}

// InsertStepsAfter inserts new steps after the specified step ID in the current task plan.
// The step ID refers to the step after which new steps will be inserted.
// If afterStepID is 0, new steps are inserted at the beginning.
// IMPORTANT: there must be no completed steps after the insertion point.
// Returns an error if any completed step exists after the insertion point.
func (m *Manager) InsertStepsAfter(afterStepID int, newStepDescriptions []string) (*TaskPlan, error) {
	plan, err := m.loadCurrent()
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("当前没有任务计划")
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
			return nil, fmt.Errorf("步骤 #%d 在当前任务计划中未找到", afterStepID)
		}
	}

	// Validate that there are no completed steps after the insertion point
	for i := insertIndex; i < len(plan.Steps); i++ {
		if plan.Steps[i].Status == StatusCompleted {
			return nil, fmt.Errorf("无法在步骤 #%d 后插入新步骤：插入点之后的步骤 #%d (%q) 已完成，已完成步骤不可重新排序",
				afterStepID, plan.Steps[i].ID, plan.Steps[i].Description)
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
	if err := m.saveCurrent(plan); err != nil {
		return nil, fmt.Errorf("无法保存任务计划: %w", err)
	}

	return plan, nil
}

// archiveToMemory archives the given task plan to conversation memory.
// The plan is formatted as a human-readable string and saved as a memory entry.
func (m *Manager) archiveToMemory(plan *TaskPlan) error {
	content := fmt.Sprintf("📋 已完成任务计划 #%d: %s\n", plan.ID, plan.Title)
	if plan.Description != "" {
		content += fmt.Sprintf("   描述: %s\n", plan.Description)
	}
	content += fmt.Sprintf("   创建时间: %s\n", plan.CreatedAt)
	content += fmt.Sprintf("   完成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	total := len(plan.Steps)
	completed := 0
	for _, s := range plan.Steps {
		if s.Status == StatusCompleted {
			completed++
		}
	}
	content += fmt.Sprintf("   进度: %d/%d\n\n", completed, total)

	for _, step := range plan.Steps {
		statusIcon := statusIcon(step.Status)
		content += fmt.Sprintf("  %s Step #%d: %s\n", statusIcon, step.ID, step.Description)
		if step.Note != "" {
			content += fmt.Sprintf("       Note: %s\n", step.Note)
		}
	}

	return m.memoryMgr.AddMessage("system", content, time.Now())
}

// loadCurrent loads the current task plan from the store.
func (m *Manager) loadCurrent() (*TaskPlan, error) {
	data, found, err := m.store.GetContext(currentPlanKey)
	if err != nil {
		return nil, fmt.Errorf("无法加载任务计划: %w", err)
	}
	if !found {
		return nil, nil
	}

	var plan TaskPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("无法解析任务计划: %w", err)
	}
	return &plan, nil
}

// saveCurrent saves the current task plan to the store.
func (m *Manager) saveCurrent(plan *TaskPlan) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("无法序列化任务计划: %w", err)
	}
	return m.store.SaveContext(currentPlanKey, data)
}

// FormatPlan formats a task plan as a human-readable string.
func FormatPlan(plan *TaskPlan) string {
	if plan == nil {
		return "📋 当前没有任务计划。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 任务计划 #%d: %s\n", plan.ID, plan.Title))
	if plan.Description != "" {
		sb.WriteString(fmt.Sprintf("   描述: %s\n", plan.Description))
	}
	sb.WriteString(fmt.Sprintf("   创建时间: %s\n", plan.CreatedAt))
	sb.WriteString(fmt.Sprintf("   更新时间: %s\n", plan.UpdatedAt))

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
	sb.WriteString(fmt.Sprintf("   进度: %d/%d (%d%%)\n\n", completed, total, progress))

	if total == 0 {
		sb.WriteString("   （无步骤）\n")
		return sb.String()
	}

	for _, step := range plan.Steps {
		statusIcon := statusIcon(step.Status)
		sb.WriteString(fmt.Sprintf("  %s 步骤 #%d: %s\n", statusIcon, step.ID, step.Description))
		if step.Note != "" {
			sb.WriteString(fmt.Sprintf("       备注: %s\n", step.Note))
		}
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
