// Package agent - task_plan stream event tests (FEATURE-307c): constructor
// shape, LineRenderer silence (terminal output must not change), and the
// emission point after a successful track_task_progress tool call
// (including the archived-plan empty snapshot).
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/taskplan"
	"github.com/idirect3d/co-shell/workspace"
)

// TestTaskPlanEventConstructor locks the wire shape of the task_plan event:
// type/channel and the plan JSON carried in Meta.
func TestTaskPlanEventConstructor(t *testing.T) {
	ev := TaskPlanEvent(`{"title":"T"}`)
	if ev.Type != EventTaskPlan {
		t.Errorf("Type = %q, want %q", ev.Type, EventTaskPlan)
	}
	if ev.Chan != ChannelTaskPlan {
		t.Errorf("Chan = %q, want %q", ev.Chan, ChannelTaskPlan)
	}
	if ev.Meta[MetaKeyPlan] != `{"title":"T"}` {
		t.Errorf("Meta[plan] = %q", ev.Meta[MetaKeyPlan])
	}
	if ev.Text != "" {
		t.Errorf("Text = %q, want empty", ev.Text)
	}
	// Archived plan: empty snapshot.
	if got := TaskPlanEvent("").Meta[MetaKeyPlan]; got != "" {
		t.Errorf("archived plan snapshot = %q, want empty", got)
	}
}

// TestLineRendererIgnoresTaskPlan asserts the terminal renderers produce
// zero output for task_plan events (the outer switch has no case for the
// new type), keeping tui/stdio behavior unchanged.
func TestLineRendererIgnoresTaskPlan(t *testing.T) {
	for _, mode := range []StreamMode{StreamModeREPL, StreamModeSingleCmd} {
		if got := renderOne(mode, TaskPlanEvent(`{"title":"T","steps":[]}`)); got != "" {
			t.Errorf("mode %v: LineRenderer emitted %q for task_plan, want silence", mode, got)
		}
	}
}

// scriptStep is one scripted tool call the fake client emits.
type scriptStep struct {
	name string // tool name ("track_task_progress" / "attempt_completion")
	args string // arguments JSON
}

// scriptedPlanClient replays a fixed script of tool calls, one per LLM
// call; a plain content reply is never a final answer while
// attempt_completion is available, so every script ends with
// attempt_completion.
type scriptedPlanClient struct {
	calls  int
	script []scriptStep
}

func (f *scriptedPlanClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Content: "ok"}, nil
}

func (f *scriptedPlanClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	i := f.calls
	f.calls++
	ch := make(chan llm.StreamEvent, 3)
	if i < len(f.script) {
		ch <- llm.StreamEvent{
			Type: llm.StreamEventToolCall,
			ToolCall: &llm.ToolCall{
				ID:        "call-1",
				Name:      f.script[i].name,
				Arguments: f.script[i].args,
			},
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "tool_calls"}
	} else {
		ch <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "done"}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "stop"}
	}
	close(ch)
	return ch, nil
}

func (f *scriptedPlanClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}
func (f *scriptedPlanClient) TestVisionSupport(ctx context.Context) bool   { return false }
func (f *scriptedPlanClient) TestTextSupport(ctx context.Context) bool     { return true }
func (f *scriptedPlanClient) TestToolCallSupport(ctx context.Context) bool { return true }
func (f *scriptedPlanClient) TestThinkingSupport(ctx context.Context) bool { return false }
func (f *scriptedPlanClient) SetThinkingEnabled(enabled bool)              {}
func (f *scriptedPlanClient) SetReasoningEffort(effort string)             {}
func (f *scriptedPlanClient) SetTopP(topP float64)                         {}
func (f *scriptedPlanClient) SetTopK(topK int)                             {}
func (f *scriptedPlanClient) SetRepetitionPenalty(penalty float64)         {}
func (f *scriptedPlanClient) SetTokenUsage(mode string)                    {}
func (f *scriptedPlanClient) SetTemperature(temp float64)                  {}
func (f *scriptedPlanClient) SetBodyAdditions(additions map[string]string) {}
func (f *scriptedPlanClient) RemoveBodyAddition(key string)                {}
func (f *scriptedPlanClient) GetBodyAdditions() map[string]string          { return nil }
func (f *scriptedPlanClient) Close() error                                 { return nil }

// newPlanEventAgent builds an agent backed by a temp-workspace store with
// tool calls enabled (openai mode).
func newPlanEventAgent(t *testing.T, client llm.Client) *Agent {
	t.Helper()
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ag := New(client, mcp.NewManager(), store.NewDualStore(boltStore, nil), "")
	ag.SetWorkspacePath(wsDir)
	ag.SetConfig(&config.Config{})
	ag.SetToolCallEnabled(true)
	ag.SetToolCallMode("openai")
	// The task-plan tools are only built when the plan feature is enabled.
	ag.SetPlanEnabled(true)
	// Install the default tool modes so track_task_progress and
	// attempt_completion run in "auto" mode (no confirmation prompt).
	ag.SyncToolModes(&config.Config{})
	return ag
}

// collectTaskPlanEvents runs one input and returns every task_plan event's
// plan snapshot.
func collectTaskPlanEvents(t *testing.T, ag *Agent, input string) []string {
	t.Helper()
	var plans []string
	_, err := ag.RunStream(context.Background(), input, func(ev StreamEvent) {
		if ev.Type == EventTaskPlan {
			plans = append(plans, ev.Meta[MetaKeyPlan])
		}
	})
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	return plans
}

// TestTaskPlanEventEmittedAfterTrackProgress verifies a successful
// track_task_progress tool call emits one task_plan event carrying the full
// plan JSON.
func TestTaskPlanEventEmittedAfterTrackProgress(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"track_task_progress", `{"meta":{"intent":"make a plan","risk":"medium","risk_reason":"planning","affected_objects":[],"progress":[]},"title":"Build it","description":"D","steps":[{"description":"step one","status":"pending"}]}`},
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)

	plans := collectTaskPlanEvents(t, ag, "make a plan")
	if len(plans) != 1 {
		t.Fatalf("task_plan events = %d, want 1", len(plans))
	}
	var plan taskplan.TaskPlan
	if err := json.Unmarshal([]byte(plans[0]), &plan); err != nil {
		t.Fatalf("plan snapshot is not valid JSON: %v", err)
	}
	if plan.Title != "Build it" || len(plan.Steps) != 1 || plan.Steps[0].Description != "step one" {
		t.Errorf("plan snapshot = %+v, want title \"Build it\" with step \"step one\"", plan)
	}
}

// TestTaskPlanEventEmptyOnArchive verifies archiving the plan (empty steps)
// emits a task_plan event with an empty snapshot so consumers hide the panel.
func TestTaskPlanEventEmptyOnArchive(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"track_task_progress", `{"meta":{"intent":"make a plan","risk":"medium","risk_reason":"planning","affected_objects":[],"progress":[]},"title":"Build it","description":"D","steps":[{"description":"step one","status":"pending"}]}`},
		{"track_task_progress", `{"meta":{"intent":"archive plan","risk":"medium","risk_reason":"archiving","affected_objects":[],"progress":[]},"title":"","description":"","steps":[]}`},
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)

	plans := collectTaskPlanEvents(t, ag, "plan then archive")
	if len(plans) != 2 {
		t.Fatalf("task_plan events = %d, want 2 (create + archive)", len(plans))
	}
	if plans[0] == "" {
		t.Errorf("create snapshot must be non-empty")
	}
	if plans[1] != "" {
		t.Errorf("archived plan snapshot = %q, want empty", plans[1])
	}
	if plan, _ := ag.taskPlanMgr.GetCurrent(); plan != nil {
		t.Errorf("current plan = %+v, want nil after archive", plan)
	}
}

// TestTaskPlanEventNotEmittedForOtherTools verifies plain content runs emit
// no task_plan events.
func TestTaskPlanEventNotEmittedForOtherTools(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)
	if plans := collectTaskPlanEvents(t, ag, "hello"); len(plans) != 0 {
		t.Errorf("task_plan events = %v, want none", plans)
	}
}

// Guard: the plan snapshot must not contain decoration (emoji/ANSI).
func TestTaskPlanEventSnapshotUndecorated(t *testing.T) {
	ev := TaskPlanEvent(`{"title":"T"}`)
	if strings.ContainsAny(ev.Meta[MetaKeyPlan], "✅⚠️❌\x1b") {
		t.Errorf("plan snapshot carries decoration: %q", ev.Meta[MetaKeyPlan])
	}
}

// TestApplyProgressReportFromMeta verifies the LLM's progress report is read
// from the meta object (meta.progress), not the top-level arguments
// (FEATURE-447). A non-track_task_progress tool carrying meta.progress must
// update the current plan's step status.
func TestApplyProgressReportFromMeta(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"track_task_progress", `{"meta":{"intent":"make a plan","risk":"medium","risk_reason":"planning","affected_objects":[],"progress":[]},"title":"Build it","description":"D","steps":[{"description":"step one","status":"pending"}]}`},
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)

	// Create the plan first.
	if _, err := ag.RunStream(context.Background(), "make a plan", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream(create): %v", err)
	}

	// Apply a progress report carried in the meta object of a generic tool
	// call (not track_task_progress). The progress lives at meta.progress.
	args := map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":          "update progress",
			"risk":            "low",
			"risk_reason":     "updating",
			"affected_objects": []interface{}{},
			"progress": []interface{}{
				map[string]interface{}{"index": float64(0), "description": "step one", "status": "in_progress"},
			},
		},
	}
	plan, err := ag.applyProgressReport(args)
	if err != nil {
		t.Fatalf("applyProgressReport: %v", err)
	}
	if plan == nil || len(plan.Steps) != 1 {
		t.Fatalf("plan = %+v, want 1 step", plan)
	}
	if plan.Steps[0].Status != "in_progress" {
		t.Errorf("step status = %q, want in_progress", plan.Steps[0].Status)
	}
}

// TestApplyProgressReportForgottenInProgress verifies that when the current
// plan has an in_progress step that is NOT covered by the new progress report,
// the report is rejected (FEATURE-452).
func TestApplyProgressReportForgottenInProgress(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"track_task_progress", `{"meta":{"intent":"make a plan","risk":"medium","risk_reason":"planning","affected_objects":[],"progress":[]},"title":"Build it","description":"D","steps":[{"description":"step one","status":"pending"},{"description":"step two","status":"pending"}]}`},
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)
	if _, err := ag.RunStream(context.Background(), "make a plan", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream(create): %v", err)
	}

	// Mark step 0 as in_progress.
	args := map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":           "update progress",
			"risk":             "low",
			"risk_reason":      "updating",
			"affected_objects": []interface{}{},
			"progress": []interface{}{
				map[string]interface{}{"index": float64(0), "description": "step one", "status": "in_progress"},
			},
		},
	}
	if _, err := ag.applyProgressReport(args); err != nil {
		t.Fatalf("applyProgressReport(mark in_progress): %v", err)
	}

	// Now report only step 1, leaving in_progress step 0 uncovered → must fail.
	args2 := map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":           "update progress",
			"risk":             "low",
			"risk_reason":      "updating",
			"affected_objects": []interface{}{},
			"progress": []interface{}{
				map[string]interface{}{"index": float64(1), "description": "step two", "status": "completed"},
			},
		},
	}
	if _, err := ag.applyProgressReport(args2); err == nil {
		t.Fatal("applyProgressReport should fail when an in_progress step is forgotten")
	}
}

// TestApplyProgressReportCoversInProgress verifies that when the current plan
// has an in_progress step that IS covered by the new progress report, the
// report is accepted (FEATURE-452).
func TestApplyProgressReportCoversInProgress(t *testing.T) {
	client := &scriptedPlanClient{script: []scriptStep{
		{"track_task_progress", `{"meta":{"intent":"make a plan","risk":"medium","risk_reason":"planning","affected_objects":[],"progress":[]},"title":"Build it","description":"D","steps":[{"description":"step one","status":"pending"},{"description":"step two","status":"pending"}]}`},
		{"attempt_completion", `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`},
	}}
	ag := newPlanEventAgent(t, client)
	if _, err := ag.RunStream(context.Background(), "make a plan", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream(create): %v", err)
	}

	// Mark step 0 as in_progress.
	args := map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":           "update progress",
			"risk":             "low",
			"risk_reason":      "updating",
			"affected_objects": []interface{}{},
			"progress": []interface{}{
				map[string]interface{}{"index": float64(0), "description": "step one", "status": "in_progress"},
			},
		},
	}
	if _, err := ag.applyProgressReport(args); err != nil {
		t.Fatalf("applyProgressReport(mark in_progress): %v", err)
	}

	// Report step 0 (the in_progress one) as completed → must succeed.
	args2 := map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":           "update progress",
			"risk":             "low",
			"risk_reason":      "updating",
			"affected_objects": []interface{}{},
			"progress": []interface{}{
				map[string]interface{}{"index": float64(0), "description": "step one", "status": "completed"},
			},
		},
	}
	if _, err := ag.applyProgressReport(args2); err != nil {
		t.Fatalf("applyProgressReport(cover in_progress): %v", err)
	}
}
