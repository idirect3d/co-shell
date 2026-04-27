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
		if len(args) < 2 {
			return "", fmt.Errorf("用法: .plan view <plan_id>")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 plan_id: %s", args[1])
		}
		return h.handleView(id)

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
		if len(args) < 4 {
			return "", fmt.Errorf("用法: .plan insert <plan_id> <after_step_id> <step1> | <step2> | ...")
		}
		planID, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 plan_id: %s", args[1])
		}
		afterStepID, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("无效的 after_step_id: %s", args[2])
		}
		stepsStr := strings.Join(args[3:], " ")
		steps := strings.Split(stepsStr, "|")
		for i := range steps {
			steps[i] = strings.TrimSpace(steps[i])
		}
		return h.handleInsert(planID, afterStepID, steps)

	case "remove":
		if len(args) < 4 {
			return "", fmt.Errorf("用法: .plan remove <plan_id> <from> <to>")
		}
		planID, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 plan_id: %s", args[1])
		}
		from, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("无效的 from: %s", args[2])
		}
		to, err := strconv.Atoi(args[3])
		if err != nil {
			return "", fmt.Errorf("无效的 to: %s", args[3])
		}
		return h.handleRemove(planID, from, to)

	case "update":
		if len(args) < 4 {
			return "", fmt.Errorf("用法: .plan update <plan_id> <step_id> <status> [note]")
		}
		planID, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的 plan_id: %s", args[1])
		}
		stepID, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("无效的 step_id: %s", args[2])
		}
		status := taskplan.TaskStatus(args[3])
		note := ""
		if len(args) > 4 {
			note = strings.Join(args[4:], " ")
		}
		return h.handleUpdate(planID, stepID, status, note)

	default:
		return "", fmt.Errorf("未知的 .plan 子命令: %s（可用: list, view, create, insert, remove, update）", subcommand)
	}
}

func (h *PlanHandler) handleList() (string, error) {
	plans, err := h.planMgr.List()
	if err != nil {
		return "", fmt.Errorf("无法获取任务计划列表: %w", err)
	}
	return taskplan.FormatPlanList(plans), nil
}

func (h *PlanHandler) handleView(id int) (string, error) {
	plan, err := h.planMgr.Get(id)
	if err != nil {
		return "", fmt.Errorf("无法获取任务计划: %w", err)
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

func (h *PlanHandler) handleInsert(planID, afterStepID int, steps []string) (string, error) {
	plan, err := h.planMgr.InsertStepsAfter(planID, afterStepID, steps)
	if err != nil {
		return "", fmt.Errorf("无法插入步骤: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleRemove(planID, from, to int) (string, error) {
	plan, err := h.planMgr.RemoveSteps(planID, from, to)
	if err != nil {
		return "", fmt.Errorf("无法删除步骤: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

func (h *PlanHandler) handleUpdate(planID, stepID int, status taskplan.TaskStatus, note string) (string, error) {
	plan, err := h.planMgr.UpdateStepStatus(planID, stepID, status, note)
	if err != nil {
		return "", fmt.Errorf("无法更新步骤状态: %w", err)
	}
	return taskplan.FormatPlan(plan), nil
}

// showPlanHelp displays the .plan command usage.
func showPlanHelp() string {
	return `📋 任务计划管理 (.plan)

  .plan list                              列出所有任务计划
  .plan view <plan_id>                    查看任务计划详情
  .plan create <title> <step1> | <step2>  创建新任务计划（步骤用 | 分隔）
  .plan insert <plan_id> <after_step_id> <step1> | <step2>  在指定步骤后插入新步骤
  .plan remove <plan_id> <from> <to>      删除指定范围内的步骤
  .plan update <plan_id> <step_id> <status> [note]  更新步骤状态

状态值: pending, in_progress, completed, failed, cancelled

示例:
  .plan list
  .plan view 1
  .plan create "我的计划" 步骤一 | 步骤二 | 步骤三
  .plan insert 1 1 新步骤A | 新步骤B
  .plan remove 1 3 4
  .plan update 1 2 completed 已完成`
}
