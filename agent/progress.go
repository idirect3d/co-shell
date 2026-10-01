// Author: L.Shuang
// Created: 2026-08-28
// Last Modified: 2026-08-28
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
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/taskplan"
)

// MissingProgressStep identifies a step that was still in progress in the task
// plan but was left out of a tool call's progress report (FIX-574).
type MissingProgressStep struct {
	Index  int
	Title  string
	Status string
}

// ProgressReportError reports a rejected meta.progress report (FIX-574). It
// carries the in-progress steps the report failed to cover, so the error text
// can list them instead of falling back to the generic "missing required
// parameters" template (which only talks about the tool's own arguments).
type ProgressReportError struct {
	Missing []MissingProgressStep
}

// Error implements the error interface with the missing steps spelled out, so
// the LLM can add exactly those steps to meta.progress and re-send the call.
func (e *ProgressReportError) Error() string {
	parts := make([]string, 0, len(e.Missing))
	for _, m := range e.Missing {
		parts = append(parts, fmt.Sprintf("[%d] %q (status: %s)", m.Index, m.Title, m.Status))
	}
	return fmt.Sprintf(
		"progress report must include the latest status of every in-progress step; missing step(s): %s",
		strings.Join(parts, ", "),
	)
}

// parseProgressSteps extracts the LLM's progress report from the tool call's
// meta object (FEATURE-447). The meta.progress field is an array of objects
// each with index/description/status. An empty or absent progress array yields
// nil.
func parseProgressSteps(args map[string]interface{}) ([]ProgressStep, error) {
	meta := metaObject(args)
	v, ok := meta["progress"]
	if !ok {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("progress argument must be an array of objects")
	}
	steps := make([]ProgressStep, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("progress[%d] must be an object with index/description/status", i)
		}
		idx, ok := m["index"].(float64)
		if !ok {
			return nil, fmt.Errorf("progress[%d].index is required and must be a number", i)
		}
		desc, _ := m["description"].(string)
		status, _ := m["status"].(string)
		if desc == "" {
			return nil, fmt.Errorf("progress[%d].description is required", i)
		}
		if status == "" {
			return nil, fmt.Errorf("progress[%d].status is required", i)
		}
		steps = append(steps, ProgressStep{
			Index:       int(idx),
			Description: desc,
			Status:      status,
		})
	}
	return steps, nil
}

// hasNonEmptyProgress reports whether the meta object carries a non-empty
// progress array (FEATURE-447). An absent, empty, or non-array progress is
// treated as "no progress report" so the plan is left untouched.
func hasNonEmptyProgress(meta map[string]interface{}) bool {
	v, ok := meta["progress"]
	if !ok {
		return false
	}
	arr, ok := v.([]interface{})
	if !ok {
		return false
	}
	return len(arr) > 0
}

// applyProgressReport applies the LLM's progress report to the current task
// plan (FEATURE-447). It returns the updated plan (nil when no plan exists and
// the report is empty) and an error when the report is invalid (e.g. an index
// beyond the appendable range, which the caller treats as a tool call failure).
func (a *Agent) applyProgressReport(args map[string]interface{}) (*taskplan.TaskPlan, error) {
	steps, err := parseProgressSteps(args)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return a.taskPlanMgr.GetCurrent()
	}

	// FEATURE-452: every step that was in_progress before this update must be
	// reflected in this progress report, otherwise an in-progress step could be
	// forgotten. Check the current plan's in_progress steps against the reported
	// indices and reject the report if any in-progress step is not covered.
	if plan, gerr := a.taskPlanMgr.GetCurrent(); gerr == nil && plan != nil {
		reported := make(map[int]bool, len(steps))
		for _, s := range steps {
			reported[s.Index] = true
		}
		var forgotten []MissingProgressStep
		for i, st := range plan.Steps {
			if st.Status == taskplan.StatusInProgress && !reported[i] {
				forgotten = append(forgotten, MissingProgressStep{
					Index:  i,
					Title:  st.Description,
					Status: string(st.Status),
				})
			}
		}
		if len(forgotten) > 0 {
			// FIX-574: carry the missing steps so the caller can render an
			// actionable error (still a hard failure: the call is rejected).
			return nil, &ProgressReportError{Missing: forgotten}
		}
	}

	inputs := make([]taskplan.ProgressInput, 0, len(steps))
	for _, s := range steps {
		inputs = append(inputs, taskplan.ProgressInput{
			Index:       s.Index,
			Description: s.Description,
			Status:      s.Status,
		})
	}
	return a.taskPlanMgr.ApplyProgress(inputs)
}
