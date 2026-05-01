// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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

// createTaskPlanTool creates a new task plan with title, description, and steps.
func (a *Agent) createTaskPlanTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("createTaskPlanTool called: args=%v", args)
	title, ok := args["title"].(string)
	if !ok {
		return "", fmt.Errorf("title argument is required")
	}

	description, _ := args["description"].(string)

	stepsRaw, ok := args["steps"].([]interface{})
	if !ok {
		return "", fmt.Errorf("steps argument is required and must be an array of strings")
	}

	steps := make([]string, 0, len(stepsRaw))
	for _, s := range stepsRaw {
		stepStr, ok := s.(string)
		if !ok {
			return "", fmt.Errorf("each step must be a string")
		}
		steps = append(steps, stepStr)
	}

	plan, err := a.taskPlanMgr.Create(title, description, steps)
	if err != nil {
		return "", fmt.Errorf("cannot create task plan: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// updateTaskStepTool updates the status of a specific step in the current task plan.
func (a *Agent) updateTaskStepTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("updateTaskStepTool called: args=%v", args)
	stepID, ok := args["step_id"].(float64)
	if !ok {
		return "", fmt.Errorf("step_id argument is required")
	}

	statusStr, ok := args["status"].(string)
	if !ok {
		return "", fmt.Errorf("status argument is required")
	}

	note, _ := args["note"].(string)

	status := taskplan.TaskStatus(statusStr)
	plan, err := a.taskPlanMgr.UpdateStepStatus(int(stepID), status, note)
	if err != nil {
		return "", fmt.Errorf("cannot update step status: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)
	return formatted, nil
}

// insertTaskStepsTool inserts new steps after a specified step in the current task plan.
func (a *Agent) insertTaskStepsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("insertTaskStepsTool called: args=%v", args)
	afterStep, ok := args["after_step_id"].(float64)
	if !ok {
		return "", fmt.Errorf("after_step_id argument is required")
	}

	newStepsRaw, ok := args["steps"].([]interface{})
	if !ok {
		return "", fmt.Errorf("steps argument is required")
	}

	newSteps := make([]string, 0, len(newStepsRaw))
	for _, s := range newStepsRaw {
		stepStr, ok := s.(string)
		if !ok {
			return "", fmt.Errorf("each new step must be a string")
		}
		newSteps = append(newSteps, stepStr)
	}

	plan, err := a.taskPlanMgr.InsertStepsAfter(int(afterStep), newSteps)
	if err != nil {
		return "", fmt.Errorf("cannot insert task steps: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// removeTaskStepsTool removes unfinished steps from the current task plan by range.
func (a *Agent) removeTaskStepsTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("removeTaskStepsTool called: args=%v", args)
	fromStep, ok := args["from"].(float64)
	if !ok {
		return "", fmt.Errorf("from argument is required")
	}

	toStep, ok := args["to"].(float64)
	if !ok {
		return "", fmt.Errorf("to argument is required")
	}

	plan, err := a.taskPlanMgr.RemoveSteps(int(fromStep), int(toStep))
	if err != nil {
		return "", fmt.Errorf("cannot remove task steps: %w", err)
	}

	formatted := taskplan.FormatPlan(plan)
	fmt.Println(formatted)

	// Set flag so agent loop adjusts messagePointer after tool messages are appended
	a.mu.Lock()
	a.needAdjustPointer = true
	a.mu.Unlock()

	return formatted, nil
}

// listTaskPlansTool lists all task plans.
func (a *Agent) listTaskPlansTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("listTaskPlansTool called: args=%v", args)
	plan, err := a.taskPlanMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("cannot get current plan: %w", err)
	}

	if plan == nil {
		return "当前没有任务计划。", nil
	}

	return taskplan.FormatPlan(plan), nil
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
		Workspace:         filepath.Join(a.workspacePath, "sub-agents", fmt.Sprintf("scheduled-%d", entry.ID)),
		Instruction:       entry.Instruction,
		TimeoutSeconds:    1800,
		Purpose:           fmt.Sprintf("Scheduled task #%d: %s", entry.ID, entry.Instruction),
		ConfirmCommandOff: true,
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
