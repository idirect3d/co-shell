// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-09-27
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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/subagent"
)

// subAgentLogTailBytes caps how much of a sub-agent's JSON-Lines log tail is
// handed back to the parent LLM by sub_agent_status.
const subAgentLogTailBytes = 2000

// errI18n builds an error from an i18n template (kept as a helper so vet's
// printf checker never sees a non-constant format string).
func errI18n(key string, args ...interface{}) error {
	return errors.New(i18n.TF(key, args...))
}

// poolErrorI18n renders a pool validation failure (FEATURE-565) as a localized
// error that names the current limit and the legal range, so the model can retry
// with a usable id instead of guessing.
func poolErrorI18n(pe *subagent.PoolError) error {
	switch pe.Kind {
	case subagent.PoolErrDisabled:
		return errI18n(i18n.KeySubAgentPoolDisabled)
	case subagent.PoolErrIDRequired:
		return errI18n(i18n.KeySubAgentIDRequired, pe.Max, pe.Max)
	case subagent.PoolErrIDFormat:
		return errI18n(i18n.KeySubAgentIDFormat, pe.ID, subagent.MaxPoolSize)
	case subagent.PoolErrIDOutOfRange:
		return errI18n(i18n.KeySubAgentIDOutOfRange, pe.ID, pe.Max, pe.Max)
	case subagent.PoolErrIDBusy:
		return errI18n(i18n.KeySubAgentIDBusy, pe.ID, pe.PID, pe.Run, pe.Max)
	case subagent.PoolErrIDDuplicate:
		return errI18n(i18n.KeySubAgentIDDuplicate, pe.ID, pe.Max)
	}
	return errors.New(pe.Error())
}

// subAgentBlock renders the <sub_agents> block injected into
// <environment_details> (FEATURE-563/565): the pool limit followed by the
// sub-agents of the current session that are running right now. The block is
// omitted only while sub-agent tools are off for this run (it would just cost
// context then).
func (a *Agent) subAgentBlock() string {
	if a.subAgentMgr == nil || !a.subAgentEnabled {
		return ""
	}
	return a.subAgentMgr.StatusBlock()
}

// launchSubAgentTool starts one or more sub-agent processes in a single call
// (FEATURE-563).
//
// Every task carries a caller-chosen id that names the sub-agent workspace
// ({workspace}/sub-agents/{id}) and is used afterwards by sub_agent_status and
// kill_sub_agent. The call waits at most timeout_seconds; sub-agents still
// running when that window expires keep running in the background.
func (a *Agent) launchSubAgentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("launchSubAgentTool called: args=%v", args)

	tasks, err := parseSubAgentTasks(args["tasks"])
	if err != nil {
		return "", err
	}
	timeoutSeconds, ok := subAgentIntArg(args["timeout_seconds"])
	if !ok {
		return "", errI18n(i18n.KeySubAgentTimeoutRequired, subagent.MinWaitSeconds, subagent.MaxWaitSeconds)
	}
	if err := subagent.ValidateWaitSeconds(timeoutSeconds); err != nil {
		return "", errI18n(i18n.KeySubAgentTimeoutRange, subagent.MinWaitSeconds, subagent.MaxWaitSeconds)
	}

	procs, err := a.subAgentMgr.Start(ctx, tasks, time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		if len(procs) == 0 {
			if pe, ok := subagent.AsPoolError(err); ok {
				return "", poolErrorI18n(pe)
			}
			return "", err
		}
		log.Warn("launch_sub_agent: partially started (%d process(es)): %v", len(procs), err)
	}

	if a.cfg == nil || a.cfg.OutputCategoryShown("subagent") {
		a.defaultIO().Printf(i18n.TF(i18n.KeySubAgentLaunched), len(procs))
	}

	// Refresh the Web UI sub-agent list right after the launch (FEATURE-564).
	a.pushSubAgents()

	var sb strings.Builder
	sb.WriteString(i18n.TF(i18n.KeySubAgentLaunched, len(procs)))
	a.appendSubAgentProcLines(&sb, procs, false)
	if err != nil {
		sb.WriteString("\n")
		sb.WriteString(i18n.TF(i18n.KeySubAgentError, err))
	}
	return sb.String(), nil
}

// subAgentStatusTool reports the process table rows (FEATURE-563): status,
// run, pid/exit code, elapsed time, the final answer of the sub-agent's last
// LLM turn, its output files and the tail of its log. wait_seconds > 0 blocks
// until the selected sub-agents finish — it never kills them.
func (a *Agent) subAgentStatusTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("subAgentStatusTool called: args=%v", args)

	ids, err := parseSubAgentIDs(args["ids"], false)
	if err != nil {
		return "", err
	}
	waitSeconds := 0
	if raw, ok := args["wait_seconds"]; ok && raw != nil {
		n, ok := subAgentIntArg(raw)
		if !ok || n < 0 || n > subagent.MaxWaitSeconds {
			return "", errI18n(i18n.KeySubAgentWaitRange, subagent.MaxWaitSeconds)
		}
		waitSeconds = n
	}

	var procs []*subagent.Proc
	if waitSeconds > 0 {
		procs = a.subAgentMgr.Wait(ids, time.Duration(waitSeconds)*time.Second)
	} else {
		procs = a.subAgentMgr.List(ids)
	}
	if len(procs) == 0 {
		if len(ids) > 0 {
			return "", errI18n(i18n.KeySubAgentUnknownID, strings.Join(ids, ", "))
		}
		return i18n.T(i18n.KeySubAgentNone), nil
	}

	// Refresh the Web UI sub-agent list (FEATURE-564).
	a.pushSubAgents()

	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeySubAgentStatusHeader))
	a.appendSubAgentProcLines(&sb, procs, true)
	return sb.String(), nil
}

// killSubAgentTool terminates the given sub-agents and their whole process
// group (FEATURE-563): SIGTERM first, SIGKILL after a short grace period.
func (a *Agent) killSubAgentTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("killSubAgentTool called: args=%v", args)

	ids, err := parseSubAgentIDs(args["ids"], true)
	if err != nil {
		return "", err
	}
	procs, err := a.subAgentMgr.Kill(ids)
	if err != nil {
		return "", err
	}

	if a.cfg == nil || a.cfg.OutputCategoryShown("subagent") {
		a.defaultIO().Printf(i18n.TF(i18n.KeySubAgentKilled), len(procs))
	}

	// Refresh the Web UI sub-agent list (FEATURE-564).
	a.pushSubAgents()

	var sb strings.Builder
	sb.WriteString(i18n.TF(i18n.KeySubAgentKilled, len(procs)))
	a.appendSubAgentProcLines(&sb, procs, false)
	return sb.String(), nil
}

// appendSubAgentProcLines renders one line per process; with details=true it
// also reports the tool-call trail, output files, final result and log tail.
func (a *Agent) appendSubAgentProcLines(sb *strings.Builder, procs []*subagent.Proc, details bool) {
	for _, p := range procs {
		sb.WriteString("\n")
		sb.WriteString(formatSubAgentProcLine(p))
		if p.WaitTimedOut {
			sb.WriteString("\n  ")
			sb.WriteString(i18n.T(i18n.KeySubAgentWaitTimedOut))
		}
		if !details {
			continue
		}
		if len(p.ToolCalls) > 0 {
			sb.WriteString("\n  ")
			sb.WriteString(i18n.TF(i18n.KeySubAgentToolCalls, strings.Join(p.ToolCalls, ", ")))
		}
		if a.subAgentMgr != nil {
			if files := a.subAgentMgr.OutputFiles(p.ID); len(files) > 0 {
				sb.WriteString("\n  ")
				sb.WriteString(i18n.TF(i18n.KeySubAgentOutputList, strings.Join(files, ", ")))
			}
		}
		sb.WriteString("\n  ")
		sb.WriteString(i18n.T(i18n.KeySubAgentResultHeader))
		sb.WriteString("\n")
		if strings.TrimSpace(p.Result) == "" {
			sb.WriteString("  ")
			sb.WriteString(i18n.T(i18n.KeySubAgentResultNone))
			continue
		}
		sb.WriteString(subAgentIndent(p.Result))
	}
}

// formatSubAgentProcLine renders the one-line summary of a process row.
func formatSubAgentProcLine(p *subagent.Proc) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "- id=%s status=%s run=%d", p.ID, p.Status, p.Run)
	if p.Status == subagent.StatusRunning {
		fmt.Fprintf(&sb, " pid=%d", p.PID)
	} else {
		fmt.Fprintf(&sb, " exit_code=%d", p.ExitCode)
	}
	fmt.Fprintf(&sb, " elapsed=%s", p.Elapsed().Round(time.Second))
	if p.Workspace != "" {
		fmt.Fprintf(&sb, " workspace=%s", p.Workspace)
	}
	return sb.String()
}

// subAgentIndent indents a multi-line block so it stays readable inside the
// tool result.
func subAgentIndent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

// parseSubAgentTasks decodes the launch_sub_agent "tasks" argument.
func parseSubAgentTasks(raw interface{}) ([]subagent.Task, error) {
	var items []interface{}
	switch t := raw.(type) {
	case []interface{}:
		items = t
	case []map[string]interface{}:
		items = make([]interface{}, 0, len(t))
		for _, m := range t {
			items = append(items, m)
		}
	}
	if len(items) == 0 {
		return nil, errI18n(i18n.KeySubAgentTasksRequired)
	}

	tasks := make([]subagent.Task, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, errI18n(i18n.KeySubAgentTaskShape, i)
		}
		id, _ := m["id"].(string)
		id = strings.TrimSpace(id)
		if err := subagent.ValidateID(id); err != nil {
			return nil, errI18n(i18n.KeySubAgentInvalidID, id)
		}
		instruction, _ := m["instruction"].(string)
		if strings.TrimSpace(instruction) == "" {
			return nil, errI18n(i18n.KeySubAgentInstructionRequired, id)
		}
		tasks = append(tasks, subagent.Task{ID: id, Instruction: instruction})
	}
	return tasks, nil
}

// parseSubAgentIDs decodes an optional/required array of sub-agent ids. A bare
// string is accepted as a single-element list for robustness.
func parseSubAgentIDs(raw interface{}, required bool) ([]string, error) {
	var items []interface{}
	switch t := raw.(type) {
	case nil:
	case []interface{}:
		items = t
	case []string:
		for _, s := range t {
			items = append(items, s)
		}
	case string:
		items = []interface{}{t}
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if err := subagent.ValidateID(s); err != nil {
			return nil, errI18n(i18n.KeySubAgentInvalidID, s)
		}
		ids = append(ids, s)
	}
	if required && len(ids) == 0 {
		return nil, errI18n(i18n.KeySubAgentIDsRequired)
	}
	return ids, nil
}

// subAgentIntArg coerces a tool argument to int (JSON numbers arrive as
// float64; plain strings are accepted too).
func subAgentIntArg(raw interface{}) (int, bool) {
	switch n := raw.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		if v, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// subAgentToolDefs returns the managed sub-agent tool definitions
// (FEATURE-563): batch launch, status/result lookup and termination. They are
// registered by buildTools only while sub-agent support is enabled.
func (a *Agent) subAgentToolDefs() []llm.Tool {
	return []llm.Tool{
		{
			Name: "launch_sub_agent",
			Description: "Launch one or more sub-agent processes in a single call. Every sub-agent takes a slot from the current " +
				"session's fixed id pool A1..An (n = the max sub-agents of this run, shown as max=… in the <sub_agents> block of " +
				"<environment_details>): read that block first and pick ids it does NOT report as running. Each sub-agent runs its " +
				"own co-shell in unattended mode (workspace <workspace>/sub-agents/<session>/<id>) and reports the final answer of " +
				"its last LLM turn (thinking excluded). The call waits at most timeout_seconds; sub-agents still running then keep " +
				"running in the background — their status is shown in the <sub_agents> block and can be read with sub_agent_status. " +
				"Re-using the id of a finished sub-agent restarts it in the same workspace with an incremented run (history logs " +
				"kept); re-using the id of a running sub-agent, an id outside A1..An, a duplicate id inside the batch, or launching " +
				"while the pool is disabled (max 0) or exhausted all fail with an error.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"tasks": map[string]interface{}{
						"type":        "array",
						"description": "Required: one entry per sub-agent to launch, each {id, instruction}. Ids must come from this session's pool A1..An and be unique within the batch.",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id": map[string]interface{}{
									"type":        "string",
									"description": "Required: the slot to use, A1..An of this session's pool (n = max sub-agents). Pick an id that the <sub_agents> block of <environment_details> does not report as running.",
								},
								"instruction": map[string]interface{}{
									"type":        "string",
									"description": "One-shot instruction the sub-agent executes autonomously.",
								},
							},
							"required": []string{"id", "instruction"},
						},
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Required: how long this call waits for the batch, 1..600 seconds. Sub-agents that exceed it keep running in the background.",
					},
				},
				"required": []string{"meta", "tasks", "timeout_seconds"},
			},
			Callback: a.launchSubAgentTool,
		},
		{
			Name: "sub_agent_status",
			Description: "Report the status of launched sub-agents: status/running pid or exit code, run number, elapsed time, " +
				"the final answer of their last LLM turn, their output file list and a log tail. The <sub_agents> block of " +
				"<environment_details> lists the same sub-agents of the current session, but only while they are running, so check " +
				"that block first and only call this tool when you need the detailed result of a running sub-agent. wait_seconds > 0 " +
				"blocks until the selected sub-agents finish (it never kills them); omit it to get the current snapshot.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"ids": map[string]interface{}{
						"type":        "array",
						"description": "Optional: sub-agent ids to report. Omit to report every tracked sub-agent.",
						"items":       map[string]interface{}{"type": "string"},
					},
					"wait_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional: wait up to this many seconds (0..600) for the selected sub-agents to finish before reporting. Never kills them.",
					},
				},
				"required": []string{"meta"},
			},
			Callback: a.subAgentStatusTool,
		},
		{
			Name: "kill_sub_agent",
			Description: "Terminate sub-agent processes and their whole process group (SIGTERM, then SIGKILL after a short grace " +
				"period). Their status becomes killed. Use the <sub_agents> block of <environment_details> to see which " +
				"sub-agents are still running before killing them.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type":        "object",
						"description": "Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure. progress must cover every step that is still in_progress plus the current step (index/description/status).",
					},
					"ids": map[string]interface{}{
						"type":        "array",
						"description": "Required: ids of the sub-agents to terminate.",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"meta", "ids"},
			},
			Callback: a.killSubAgentTool,
		},
	}
}
