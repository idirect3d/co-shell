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

	"github.com/idirect3d/co-shell/log"
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

// StepInput represents a step input from the LLM via track_task_progress.
// The Description field can contain multi-line text:
//   - The first line is the step title/summary.
//   - Subsequent lines provide detailed content for the step.
//
// The Status field accepts status icons or raw strings:
//   - "[ ]" / "pending"      — todo
//   - "[=]" / "in_progress"   — in progress
//   - "[X]" / "completed"     — completed
//   - "[C]" / "cancelled"     — cancelled
//   - "[F]" / "failed"        — failed
type StepInput struct {
	Description string `json:"description"`
	Status      string `json:"status"`
}

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
	store         *store.DualStore
	memoryMgr     *memory.Manager
	planCounter   int    // monotonically increasing counter for plan IDs
	memoryEnabled bool   // whether memory archival is enabled
	agentName     string // name of the agent for memory archival
}

// NewManager creates a new TaskPlan manager.
func NewManager(s *store.DualStore) *Manager {
	mgr := &Manager{
		store:         s,
		memoryMgr:     memory.NewManager(s),
		memoryEnabled: false, // disabled by default, agent will enable it
	}
	// Load the current plan to determine the counter
	plan, err := mgr.loadCurrent()
	if err == nil && plan != nil {
		mgr.planCounter = plan.ID
	}
	return mgr
}

// SetMemoryEnabled enables or disables memory archival for task plans.
func (m *Manager) SetMemoryEnabled(enabled bool) {
	m.memoryEnabled = enabled
}

// SetAgentName sets the agent name used for memory archival.
func (m *Manager) SetAgentName(name string) {
	m.agentName = name
}

// HasUnfinished returns true if there is a current plan with unfinished steps.
// Unfinished steps are those with status pending, in_progress, or failed.
// Steps with status completed or cancelled are considered finished.
func (m *Manager) HasUnfinished() bool {
	plan, err := m.loadCurrent()
	if err != nil || plan == nil {
		return false
	}
	for _, step := range plan.Steps {
		if step.Status != StatusCompleted && step.Status != StatusCancelled {
			return true
		}
	}
	return false
}

// GetCurrent returns the current task plan, or nil if none exists.
func (m *Manager) GetCurrent() (*TaskPlan, error) {
	return m.loadCurrent()
}

// UpdateSteps is the unified method that:
//   - If no plan exists, creates a new one with the given title/description/steps.
//   - If a plan exists, replaces its steps entirely with the new steps array.
//   - If steps is empty, archives the current plan (if any) and deletes it.
//
// The title and description parameters set the plan-level fields. For a detailed plan,
// description should contain the full plan context, background, constraints, etc.
//
// Each StepInput.Description can contain multi-line text:
//   - The first line is the step title/summary.
//   - Subsequent lines provide detailed content for the step.
//
// Each StepInput.Status supports icons: "[ ]" / "[=]" / "[X]" / "[C]" / "[F]"
// or raw strings: "pending" / "in_progress" / "completed" / "cancelled" / "failed".
func (m *Manager) UpdateSteps(title, description string, steps []StepInput) (*TaskPlan, error) {
	if len(steps) == 0 || title == "" {
		// Empty steps or empty title: archive and delete current plan
		existing, err := m.loadCurrent()
		if err != nil {
			return nil, fmt.Errorf("无法加载当前任务计划: %w", err)
		}
		if existing != nil {
			if err := m.archiveToMemory(existing, true); err != nil {
				log.Warn("Failed to archive cancelled plan: %v", err)
			}
			if err := m.store.DeleteContext(currentPlanKey); err != nil {
				return nil, fmt.Errorf("无法删除空任务计划: %w", err)
			}
		}
		return nil, nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Load existing plan
	existing, err := m.loadCurrent()
	if err != nil {
		return nil, fmt.Errorf("无法加载当前任务计划: %w", err)
	}

	if existing == nil {
		// No existing plan — create new one
		m.planCounter++
		plan := &TaskPlan{
			ID:          m.planCounter,
			Title:       title,
			Description: description,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		for i, s := range steps {
			status, err := ParseStatus(s.Status)
			if err != nil {
				return nil, fmt.Errorf("步骤 #%d 状态无效: %w", i+1, err)
			}
			plan.Steps = append(plan.Steps, TaskStep{
				ID:          i + 1,
				Description: s.Description,
				Status:      status,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}

		if err := m.saveCurrent(plan); err != nil {
			return nil, fmt.Errorf("无法保存任务计划: %w", err)
		}

		return plan, nil
	}

	// Existing plan — replace entirely
	if title != "" {
		existing.Title = title
	}
	if description != "" {
		existing.Description = description
	}

	newSteps := make([]TaskStep, 0, len(steps))
	for i, s := range steps {
		status, err := ParseStatus(s.Status)
		if err != nil {
			return nil, fmt.Errorf("步骤 #%d 状态无效: %w", i+1, err)
		}
		newSteps = append(newSteps, TaskStep{
			ID:          i + 1,
			Description: s.Description,
			Status:      status,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	existing.Steps = newSteps
	existing.UpdatedAt = now

	if err := m.saveCurrent(existing); err != nil {
		return nil, fmt.Errorf("无法保存任务计划: %w", err)
	}

	return existing, nil
}

// archiveToMemory archives the given task plan to conversation memory.
// The plan is formatted as a human-readable string and saved as a memory entry.
// If memory is not enabled, this is a no-op.
// If cancelled is true, the plan is marked as cancelled instead of completed.
func (m *Manager) archiveToMemory(plan *TaskPlan, cancelled bool) error {
	if !m.memoryEnabled {
		return nil
	}

	statusLabel := "已完成"
	if cancelled {
		statusLabel = "已取消"
	}
	content := fmt.Sprintf("📋 %s任务计划 #%d: %s\n", statusLabel, plan.ID, plan.Title)
	if plan.Description != "" {
		content += fmt.Sprintf("   描述: %s\n", plan.Description)
	}
	content += fmt.Sprintf("   创建时间: %s\n", plan.CreatedAt)
	content += fmt.Sprintf("   %s时间: %s\n", statusLabel, time.Now().Format("2006-01-02 15:04:05"))

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

	name := m.agentName
	if name == "" {
		name = "system"
	}
	return m.memoryMgr.AddMessage(name, content, time.Now())
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

// ParseStatus maps a status string to TaskStatus.
// Accepts both status icons and raw strings for user-friendliness.
//   - "[ ]" / "pending"      → StatusPending
//   - "[=]" / "in_progress"  → StatusInProgress
//   - "[X]" / "completed"    → StatusCompleted
//   - "[C]" / "cancelled"    → StatusCancelled
//   - "[F]" / "failed"       → StatusFailed
func ParseStatus(status string) (TaskStatus, error) {
	switch status {
	case "pending", "[ ]":
		return StatusPending, nil
	case "in_progress", "[=]":
		return StatusInProgress, nil
	case "completed", "[X]":
		return StatusCompleted, nil
	case "cancelled", "[C]":
		return StatusCancelled, nil
	case "failed", "[F]":
		return StatusFailed, nil
	default:
		return "", fmt.Errorf("无效的状态 '%s'：可选状态为 pending/[ ]、in_progress/[=]、completed/[X]、failed/[F]、cancelled/[C]", status)
	}
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
