// Package agent - Interaction model and TerminalInteractionManager tests
// (FEATURE-388, UC-0001 ~ UC-0015).
//
// Author: L.Shuang
// Created: 2026-08-21
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockUserIO is a scripted UserIO for testing TerminalInteractionManager.
type mockUserIO struct {
	inputs []string // queued line inputs
	idx    int
	out    strings.Builder
}

func (m *mockUserIO) Print(args ...interface{}) {
	m.out.WriteString(strings.TrimSpace(fmtSprint(args...)) + " ")
}
func (m *mockUserIO) Printf(format string, args ...interface{}) {
	m.out.WriteString(strings.TrimSpace(fmt.Sprintf(format, args...)) + " ")
}
func (m *mockUserIO) Println(args ...interface{}) {
	m.out.WriteString(strings.TrimSpace(fmtSprint(args...)) + "\n")
}
func (m *mockUserIO) ErrPrintf(format string, args ...interface{}) {}
func (m *mockUserIO) ReadLine() (string, error) {
	if m.idx >= len(m.inputs) {
		return "", nil
	}
	v := m.inputs[m.idx]
	m.idx++
	return v, nil
}
func (m *mockUserIO) ReadKey() (byte, error) { return 0, nil }
func (m *mockUserIO) IsReading() bool        { return false }

func fmtSprint(args ...interface{}) string {
	var sb strings.Builder
	for _, a := range args {
		sb.WriteString(a.(string))
	}
	return sb.String()
}

// TestInteractionKindEnum verifies the InteractionKind constants (UC-0002).
func TestInteractionKindEnum(t *testing.T) {
	if InteractionConfirm != "confirm" {
		t.Errorf("InteractionConfirm = %q, want confirm", InteractionConfirm)
	}
	if InteractionSelect != "select" {
		t.Errorf("InteractionSelect = %q, want select", InteractionSelect)
	}
	if InteractionInput != "input" {
		t.Errorf("InteractionInput = %q, want input", InteractionInput)
	}
	if InteractionKey != "key" {
		t.Errorf("InteractionKey = %q, want key", InteractionKey)
	}
}

// TestInteractionStructFields verifies Interaction/InteractionResult fields
// exist and are typed correctly (UC-0001).
func TestInteractionStructFields(t *testing.T) {
	in := Interaction{
		Kind:      InteractionConfirm,
		Title:     "title",
		Body:      "body",
		Options:   []string{"A", "B"},
		Keys:      []KeyOption{{Label: "Approve", Key: "", Value: "approve"}},
		Default:   "approve",
		AllowFree: true,
		Presets:   []string{"3", "10", "50"},
	}
	if in.Kind != InteractionConfirm || in.Title != "title" || in.Body != "body" {
		t.Errorf("Interaction fields wrong: %+v", in)
	}
	if len(in.Options) != 2 || len(in.Keys) != 1 || len(in.Presets) != 3 {
		t.Errorf("Interaction slices wrong: %+v", in)
	}
	if !in.AllowFree || in.Default != "approve" {
		t.Errorf("Interaction flags wrong: %+v", in)
	}

	res := InteractionResult{Action: ActionApprove, Value: "3", Raw: "3"}
	if res.Action != ActionApprove || res.Value != "3" || res.Raw != "3" {
		t.Errorf("InteractionResult fields wrong: %+v", res)
	}
}

// TestInteractionManagerInterface verifies TerminalInteractionManager
// implements InteractionManager (UC-0003).
func TestInteractionManagerInterface(t *testing.T) {
	var _ InteractionManager = NewTerminalInteractionManager(&mockUserIO{})
}

// TestTerminalConfirmApprove verifies Enter approves (UC-0004).
func TestTerminalConfirmApprove(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{""}})
	res, err := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm, Title: "t"})
	if err != nil {
		t.Fatalf("Ask error: %v", err)
	}
	if res.Action != ActionApprove {
		t.Errorf("Action = %q, want approve", res.Action)
	}
}

// TestTerminalConfirmCancel verifies c cancels (UC-0005).
func TestTerminalConfirmCancel(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"c"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionCancel {
		t.Errorf("Action = %q, want cancel", res.Action)
	}
}

// TestTerminalConfirmApproveAll verifies a approves all (UC-0006).
func TestTerminalConfirmApproveAll(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"a"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionApproveAll {
		t.Errorf("Action = %q, want approve_all", res.Action)
	}
}

// TestTerminalConfirmApproveG verifies g disables confirmation (UC-0007).
func TestTerminalConfirmApproveG(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"g"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionApproveG {
		t.Errorf("Action = %q, want approve_g", res.Action)
	}
}

// TestTerminalConfirmApproveD verifies d permanently disables (UC-0008).
func TestTerminalConfirmApproveD(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"d"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionApproveD {
		t.Errorf("Action = %q, want approve_d", res.Action)
	}
}

// TestTerminalConfirmApproveCount verifies N approves N times (UC-0009).
func TestTerminalConfirmApproveCount(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"3"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionApproveCount || res.Value != "3" {
		t.Errorf("Action/Value = %q/%q, want approve_count/3", res.Action, res.Value)
	}
}

// TestTerminalConfirmModify verifies other input is supplementary (UC-0010).
func TestTerminalConfirmModify(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"请先检查文件"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionConfirm})
	if res.Action != ActionModify || res.Value != "请先检查文件" {
		t.Errorf("Action/Value = %q/%q, want modify/请先检查文件", res.Action, res.Value)
	}
}

// TestTerminalSelectOption verifies selecting an option by number (UC-0011).
func TestTerminalSelectOption(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"1"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"立即执行", "稍后执行"}})
	if res.Action != ActionSelect || res.Value != "立即执行" {
		t.Errorf("Action/Value = %q/%q, want select/立即执行", res.Action, res.Value)
	}
}

// TestTerminalSelectCancel verifies the cancel option (UC-0012).
func TestTerminalSelectCancel(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"3"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"A", "B"}})
	if res.Action != ActionCancel {
		t.Errorf("Action = %q, want cancel", res.Action)
	}
}

// TestTerminalSelectOptionWithNote verifies option + supplementary note (UC-0013).
func TestTerminalSelectOptionWithNote(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"1 请补充细节"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"立即执行", "稍后执行"}})
	if res.Action != ActionSelect || res.Value != "立即执行" {
		t.Errorf("Action/Value = %q/%q, want select/立即执行", res.Action, res.Value)
	}
	if !strings.Contains(res.Raw, "请补充细节") {
		t.Errorf("Raw = %q, want to contain 请补充细节", res.Raw)
	}
}

// TestTerminalInputFree verifies free-form input (UC-0014).
func TestTerminalInputFree(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"任意内容"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionInput})
	if res.Action != ActionInput || res.Value != "任意内容" {
		t.Errorf("Action/Value = %q/%q, want input/任意内容", res.Action, res.Value)
	}
}

// TestTerminalSelectInvalidRetry verifies invalid option retries (UC-0015).
func TestTerminalSelectInvalidRetry(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"9", "1"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"A", "B"}})
	if res.Action != ActionSelect || res.Value != "A" {
		t.Errorf("Action/Value = %q/%q, want select/A", res.Action, res.Value)
	}
}

// TestPromptToolConfirmationMigration verifies promptToolConfirmation maps
// Interaction results back to CmdConfirmResult unchanged after migration
// (UC-0016).
func TestPromptToolConfirmationMigration(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  CmdConfirmResult
	}{
		{"enter approve", "", CmdConfirmApprove},
		{"c cancel", "c", CmdConfirmCancel},
		{"a approve all", "a", CmdConfirmApproveAll},
		{"g approve g", "g", CmdConfirmApproveG},
		{"d approve d", "d", CmdConfirmApproveD},
		{"3 approve count", "3", CmdConfirmApproveCount},
		{"modify", "请先检查", CmdConfirmModify},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			io := &mockUserIO{inputs: []string{tc.input}}
			result, _ := promptToolConfirmation("execute_command", "summary", NewTerminalInteractionManager(io))
			if result != tc.want {
				t.Errorf("promptToolConfirmation(%q) = %v, want %v", tc.input, result, tc.want)
			}
		})
	}
}

// TestPromptToolConfirmationCountValue verifies the approve-count value is
// carried through (UC-0016).
func TestPromptToolConfirmationCountValue(t *testing.T) {
	io := &mockUserIO{inputs: []string{"5"}}
	result, val := promptToolConfirmation("execute_command", "summary", NewTerminalInteractionManager(io))
	if result != CmdConfirmApproveCount || val != "5" {
		t.Errorf("result/val = %v/%q, want CmdConfirmApproveCount/5", result, val)
	}
}

// TestPromptToolConfirmationModifyValue verifies the modify value is carried
// through (UC-0016).
func TestPromptToolConfirmationModifyValue(t *testing.T) {
	io := &mockUserIO{inputs: []string{"请先检查文件"}}
	result, val := promptToolConfirmation("execute_command", "summary", NewTerminalInteractionManager(io))
	if result != CmdConfirmModify || val != "请先检查文件" {
		t.Errorf("result/val = %v/%q, want CmdConfirmModify/请先检查文件", result, val)
	}
}

// captureInteractionManager captures the Interaction passed to Ask so tests can
// verify the structured fields (Keys/AllowFree/Presets) that drive the Web UI.
// askResult (optional) overrides the default approve result.
type captureInteractionManager struct {
	captured  Interaction
	askResult InteractionResult
}

func (m *captureInteractionManager) Ask(ctx context.Context, in Interaction) (InteractionResult, error) {
	m.captured = in
	if m.askResult.Action != "" {
		return m.askResult, nil
	}
	return InteractionResult{Action: ActionApprove}, nil
}

// TestPromptToolConfirmationInputHolds verifies that supplementary input
// (ActionInput) holds execution and maps to CmdConfirmModify, NOT approve
// (FEATURE-388 fix).
func TestPromptToolConfirmationInputHolds(t *testing.T) {
	mgr := &captureInteractionManager{}
	mgr.askResult = InteractionResult{Action: ActionInput, Value: "请先检查文件"}
	result, val := promptToolConfirmation("execute_command", "summary", mgr)
	if result != CmdConfirmModify || val != "请先检查文件" {
		t.Errorf("result/val = %v/%q, want CmdConfirmModify/请先检查文件", result, val)
	}
}

// TestPromptToolConfirmationInteractionFields verifies promptToolConfirmation
// builds a confirm Interaction with Keys, AllowFree and Presets so the Web UI
// can render a button group (FEATURE-388 fix).
func TestPromptToolConfirmationInteractionFields(t *testing.T) {
	mgr := &captureInteractionManager{}
	promptToolConfirmation("execute_command", "summary", mgr)

	in := mgr.captured
	if in.Kind != InteractionConfirm {
		t.Errorf("Kind = %q, want confirm", in.Kind)
	}
	if len(in.Keys) == 0 {
		t.Error("Keys should be non-empty so the Web UI renders a button group")
	}
	if !in.AllowFree {
		t.Error("AllowFree should be true so the user can type supplementary instructions")
	}
	if len(in.Presets) == 0 {
		t.Error("Presets should be non-empty for approve-N buttons")
	}
	// Verify the key values map to the expected actions.
	foundApprove := false
	for _, k := range in.Keys {
		if k.Value == string(ActionApprove) {
			foundApprove = true
		}
	}
	if !foundApprove {
		t.Errorf("Keys missing approve action: %+v", in.Keys)
	}
}

// TestPromptErrorConfirmationInteractionFields verifies promptErrorConfirmation
// builds a confirm Interaction with continue/cancel/ignore keys so the Web UI
// can render the error-handling button group (FEATURE-388).
func TestPromptErrorConfirmationInteractionFields(t *testing.T) {
	mgr := &captureInteractionManager{}
	promptErrorConfirmation(mgr, "title", "body")

	in := mgr.captured
	if in.Kind != InteractionConfirm {
		t.Errorf("Kind = %q, want confirm", in.Kind)
	}
	if len(in.Keys) != 3 {
		t.Errorf("Keys should have 3 options (continue/cancel/ignore), got %d", len(in.Keys))
	}
	// Verify the three actions are present.
	actions := map[string]bool{}
	for _, k := range in.Keys {
		actions[k.Value] = true
	}
	if !actions[string(ActionApprove)] || !actions[string(ActionCancel)] || !actions[string(ActionApproveAll)] {
		t.Errorf("Keys missing expected actions: %+v", in.Keys)
	}
}

// newAskFollowupAgent builds a minimal Agent for askFollowupQuestionTool tests.
func newAskFollowupAgent(io UserIO) *Agent {
	return &Agent{
		io:                   io,
		taskInstructionCache: bytes.Buffer{},
	}
}

// TestAskFollowupQuestionSelect verifies askFollowupQuestionTool stores the
// selected option in the task instruction cache after migration (UC-0017).
func TestAskFollowupQuestionSelect(t *testing.T) {
	io := &mockUserIO{inputs: []string{"1"}}
	a := newAskFollowupAgent(io)
	res, err := a.askFollowupQuestionTool(context.Background(), map[string]interface{}{
		"question": "请选择处理方式",
		"options":  []interface{}{"立即执行", "稍后执行"},
	})
	if err != nil {
		t.Fatalf("askFollowupQuestionTool error: %v", err)
	}
	if res == "" {
		t.Error("expected non-empty result")
	}
	if got := a.taskInstructionCache.String(); got != "立即执行" {
		t.Errorf("taskInstructionCache = %q, want 立即执行", got)
	}
}

// TestAskFollowupQuestionSelectWithNote verifies option + supplementary note is
// stored (UC-0017).
func TestAskFollowupQuestionSelectWithNote(t *testing.T) {
	io := &mockUserIO{inputs: []string{"1 请补充细节"}}
	a := newAskFollowupAgent(io)
	_, err := a.askFollowupQuestionTool(context.Background(), map[string]interface{}{
		"question": "请选择处理方式",
		"options":  []interface{}{"立即执行", "稍后执行"},
	})
	if err != nil {
		t.Fatalf("askFollowupQuestionTool error: %v", err)
	}
	got := a.taskInstructionCache.String()
	if !strings.Contains(got, "立即执行") || !strings.Contains(got, "请补充细节") {
		t.Errorf("taskInstructionCache = %q, want to contain 立即执行 and 请补充细节", got)
	}
}

// TestAskFollowupQuestionFreeInput verifies free-form input is stored (UC-0017).
func TestAskFollowupQuestionFreeInput(t *testing.T) {
	io := &mockUserIO{inputs: []string{"任意内容"}}
	a := newAskFollowupAgent(io)
	_, err := a.askFollowupQuestionTool(context.Background(), map[string]interface{}{
		"question": "请补充说明",
	})
	if err != nil {
		t.Fatalf("askFollowupQuestionTool error: %v", err)
	}
	if got := a.taskInstructionCache.String(); got != "任意内容" {
		t.Errorf("taskInstructionCache = %q, want 任意内容", got)
	}
}

// TestAskFollowupQuestionCancel verifies cancel returns CANCEL_AGENT (UC-0017).
func TestAskFollowupQuestionCancel(t *testing.T) {
	io := &mockUserIO{inputs: []string{"3"}}
	a := newAskFollowupAgent(io)
	_, err := a.askFollowupQuestionTool(context.Background(), map[string]interface{}{
		"question": "请选择处理方式",
		"options":  []interface{}{"立即执行", "稍后执行"},
	})
	if err == nil || !strings.Contains(err.Error(), "CANCEL_AGENT") {
		t.Errorf("expected CANCEL_AGENT error, got %v", err)
	}
}
