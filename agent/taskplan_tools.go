// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-06-17
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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/subagent"
	"github.com/idirect3d/co-shell/taskplan"
)

// scheduleTaskTool schedules a recurring task using a cron expression.
func (a *Agent) scheduleTaskTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("scheduleTaskTool called: args=%v", args)
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("name argument is required")
	}

	cron, ok := args["cron"].(string)
	if !ok {
		return "", fmt.Errorf("cron argument is required")
	}

	instruction, ok := args["instruction"].(string)
	if !ok {
		return "", fmt.Errorf("instruction argument is required")
	}

	if a.scheduler == nil {
		return "", fmt.Errorf("scheduler is not initialized")
	}

	id, err := a.scheduler.Add(name, cron, instruction)
	if err != nil {
		return "", fmt.Errorf("cannot schedule task: %w", err)
	}

	// Persist to store
	if err := a.persistSchedulerEntries(); err != nil {
		log.Warn("Cannot persist scheduler entries: %v", err)
	}

	entry := a.scheduler.Get(id)
	nextRun := ""
	if entry != nil {
		nextRun = entry.NextRun.Format("2006-01-02 15:04:05")
	}

	return fmt.Sprintf("✅ 定时任务 #%d (%s) 已创建\n  Cron: %s\n  指令: %s\n  下次执行: %s",
		id, name, cron, instruction, nextRun), nil
}

// trackTaskProgressTool is the unified LLM tool for recording and tracking task progress.
// It accepts a title, description, and complete steps array (each step has description + status).
// If no plan exists, it creates one. If a plan exists, it replaces the steps entirely.
// If steps is empty, the current plan is archived and deleted.
func (a *Agent) trackTaskProgressTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("trackTaskProgressTool called: args=%v", args)

	title, _ := args["title"].(string)
	description, _ := args["description"].(string)

	stepsRaw, ok := args["steps"].([]interface{})
	if !ok {
		return "", fmt.Errorf("steps argument is required and must be an array of objects")
	}

	steps := make([]taskplan.StepInput, 0, len(stepsRaw))
	for i, s := range stepsRaw {
		stepMap, ok := s.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("步骤 #%d: 每个步骤必须是一个对象，包含 description 和 status 字段", i+1)
		}

		desc, _ := stepMap["description"].(string)
		status, _ := stepMap["status"].(string)

		if desc == "" {
			return "", fmt.Errorf("步骤 #%d: description 字段不能为空", i+1)
		}
		if status == "" {
			status = "[ ]" // default to pending
		}

		steps = append(steps, taskplan.StepInput{
			Description: desc,
			Status:      status,
		})
	}

	plan, err := a.taskPlanMgr.UpdateSteps(title, description, steps)
	if err != nil {
		return "", fmt.Errorf("cannot update task progress: %w", err)
	}

	if plan == nil {
		// Empty steps resulted in plan being archived and deleted
		return "✅ 当前任务计划已归档并删除。", nil
	}

	formatted := taskplan.FormatPlan(plan)
	a.defaultIO().Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// viewTaskPlanTool views the current active task plan.
func (a *Agent) viewTaskPlanTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("viewTaskPlanTool called: args=%v", args)
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("cannot get current plan: %w", err)
	}

	if plan == nil {
		return "当前没有活跃的任务计划。", nil
	}

	return taskplan.FormatPlan(plan), nil
}

// OnScheduledTaskTriggered is called when a scheduled task is triggered.
func (a *Agent) OnScheduledTaskTriggered(entry *scheduler.CronEntry) {
	log.Info("Scheduled task #%d triggered: %s", entry.ID, entry.Instruction)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cfg := subagent.SubAgentConfig{
		Workspace:      filepath.Join(a.workspacePath, "sub-agents", fmt.Sprintf("scheduled-%d", entry.ID)),
		Instruction:    entry.Instruction,
		TimeoutSeconds: 1800,
		Purpose:        fmt.Sprintf("Scheduled task #%d: %s", entry.ID, entry.Instruction),
		ConfirmToolOff: true,
	}

	result, err := a.subAgentMgr.LaunchSubAgent(ctx, cfg)
	if err != nil {
		log.Error("Scheduled task #%d failed: %v", entry.ID, err)
		return
	}

	log.Info("Scheduled task #%d completed: duration=%s, exitCode=%d",
		entry.ID, result.Duration, result.ExitCode)
}

// persistSchedulerEntries saves all scheduler entries to memory for persistence across restarts.
func (a *Agent) persistSchedulerEntries() error {
	if a.scheduler == nil {
		return nil
	}

	entries := a.scheduler.GetEntriesForStorage()
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("cannot marshal scheduler entries: %w", err)
	}

	return a.store.SaveMemory("scheduler:entries", string(data))
}
