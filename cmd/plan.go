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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/taskplan"
)

// PlanHandler handles the .plan built-in command.
type PlanHandler struct {
	planMgr *taskplan.Manager
}

// NewPlanHandler creates a new PlanHandler.
func NewPlanHandler(planMgr *taskplan.Manager) *PlanHandler {
	return &PlanHandler{planMgr: planMgr}
}

// Handle processes .plan commands.
func (h *PlanHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return showPlanHelp(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.handleList()

	case "view":
		return h.handleView()

	case "create":
		if len(args) < 3 {
			return "", fmt.Errorf("用法: .plan create <title> <step1> | <step2> | ...")
		}
		title := args[1]
		stepsStr := strings.Join(args[2:], " ")
		steps := strings.Split(stepsStr, "|")
		// Trim spaces from each step
		for i := range steps {
			steps[i] = strings.TrimSpace(steps[i])
		}
		return h.handleCreate(title, steps)

	case "insert":
		if len(args) < 3 {
			return "", fmt.Errorf("用法: .plan insert <after_step_id> <step1> | <step2> | ...")
		}
		afterStepID, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 after_step_id: %s", args[1])
		}
		stepsStr := strings.Join(args[2:], " ")
		steps := strings.Split(stepsStr, "|")
		for i := range steps {
			steps[i] = strings.TrimSpace(steps[i])
		}
		return h.handleInsert(afterStepID, steps)

	case "remove":
		if len(args) < 3 {
			return "", fmt.Errorf("用法: .plan remove <from> <to>")
		}
		from, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 from: %s", args[1])
		}
		to, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("无效的 to: %s", args[2])
		}
		return h.handleRemove(from, to)

	case "update":
		if len(args) < 3 {
			return "", fmt.Errorf("用法: .plan update <step_id> <status> [note]")
		}
		stepID, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 step_id: %s", args[1])
		}
		status := taskplan.TaskStatus(args[2])
		note := ""
		if len(args) > 3 {
			note = strings.Join(args[3:], " ")
		}
		return h.handleUpdate(stepID, status, note)

	default:
		return "", fmt.Errorf("未知的 .plan 子命令: %s（可用: list, view, create, insert, remove, update）", subcommand)
	}
}

func (h *PlanHandler) handleList() (string, error) {
	plan, err := h.planMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("无法获取当前任务计划: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleView() (string, error) {
	plan, err := h.planMgr.GetCurrent()
	if err != nil {
		return "", fmt.Errorf("无法获取当前任务计划: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleCreate(title string, steps []string) (string, error) {
	plan, err := h.planMgr.Create(title, "", steps)
	if err != nil {
		return "", fmt.Errorf("无法创建任务计划: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleInsert(afterStepID int, steps []string) (string, error) {
	plan, err := h.planMgr.InsertStepsAfter(afterStepID, steps)
	if err != nil {
		return "", fmt.Errorf("无法插入步骤: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleRemove(from, to int) (string, error) {
	plan, err := h.planMgr.RemoveSteps(from, to)
	if err != nil {
		return "", fmt.Errorf("无法删除步骤: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleUpdate(stepID int, status taskplan.TaskStatus, note string) (string, error) {
	plan, err := h.planMgr.UpdateStepStatus(stepID, status, note)
	if err != nil {
		return "", fmt.Errorf("无法更新步骤状态: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

// showPlanHelp displays the .plan command usage.
func showPlanHelp() string {
	return `📋 任务计划管理 (.plan)

  .plan list                              查看当前任务计划
  .plan view                              查看当前任务计划详情
  .plan create <title> <step1> | <step2>  创建新任务计划（步骤用 | 分隔）
  .plan insert <after_step_id> <step1> | <step2>  在指定步骤后插入新步骤
  .plan remove <from> <to>                删除指定范围内的步骤
  .plan update <step_id> <status> [note]  更新步骤状态

注意：同一时间只能有一个任务计划。创建新计划时，如果当前计划已完成，旧计划会自动归档到记忆。
      如果当前计划有未完成的步骤，必须先完成或调整后才能创建新计划。

状态值: pending, in_progress, completed, failed, cancelled

示例:
  .plan list
  .plan view
  .plan create "我的计划" 步骤一 | 步骤二 | 步骤三
  .plan insert 1 新步骤A | 新步骤B
  .plan remove 3 4
  .plan update 2 completed 已完成`
}
