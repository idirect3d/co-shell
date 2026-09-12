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

	"github.com/idirect3d/co-shell/i18n"
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

// TestTerminalSelectCancel verifies the cancel option (UC-0012). With 2
// options, the supplementary option is [3] and cancel is [4] (FEATURE-438).
func TestTerminalSelectCancel(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"4"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"A", "B"}})
	if res.Action != ActionCancel {
		t.Errorf("Action = %q, want cancel", res.Action)
	}
}

// TestTerminalSelectSupplementary verifies selecting the fixed supplementary
// option (len(options)+1) enters free input mode (FEATURE-438).
func TestTerminalSelectSupplementary(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"3", "请补充细节"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"A", "B"}})
	if res.Action != ActionInput || res.Value != "请补充细节" {
		t.Errorf("Action/Value = %q/%q, want input/请补充细节", res.Action, res.Value)
	}
}

// TestTerminalSelectSpaceInput verifies typing a leading space enters
// supplementary-info input directly (FEATURE-438).
func TestTerminalSelectSpaceInput(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{" 请补充细节"}})
	res, _ := m.Ask(context.Background(), Interaction{Kind: InteractionSelect, Options: []string{"A", "B"}})
	if res.Action != ActionInput || res.Value != "请补充细节" {
		t.Errorf("Action/Value = %q/%q, want input/请补充细节", res.Action, res.Value)
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

// newAskUserAgent builds a minimal Agent for ask_user tool tests (FEATURE-512).
func newAskUserAgent(io UserIO) *Agent {
	return &Agent{
		io:                   io,
		taskInstructionCache: bytes.Buffer{},
	}
}

// TestAskUserLegacySingleQuestion verifies the legacy question/options shape is
// still accepted and treated as a single-question form (UC-0003).
func TestAskUserLegacySingleQuestion(t *testing.T) {
	io := &mockUserIO{inputs: []string{"1"}}
	a := newAskUserAgent(io)
	res, err := a.askUserTool(context.Background(), map[string]interface{}{
		"question": "请选择处理方式",
		"options":  []interface{}{"立即执行", "稍后执行"},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}
	if res == "" {
		t.Error("expected non-empty result")
	}
	if got := a.taskInstructionCache.String(); !strings.Contains(got, "立即执行") {
		t.Errorf("taskInstructionCache = %q, want to contain 立即执行", got)
	}
}

// TestAskUserMultipleQuestions verifies several questions are collected in one
// round: single choice first, then multi choice (UC-0009/UC-0010).
func TestAskUserMultipleQuestions(t *testing.T) {
	io := &mockUserIO{inputs: []string{"1", "1,2"}}
	a := newAskUserAgent(io)
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"title":   "用哪个数据库？",
				"options": []interface{}{"MySQL", "PostgreSQL"},
			},
			map[string]interface{}{
				"title":   "需要哪些能力？",
				"options": []interface{}{"读写分离", "自动备份"},
				"multi":   true,
			},
		},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}
	got := a.taskInstructionCache.String()
	// The compact format keeps only the question number and the answer; the
	// question text itself is no longer repeated (FEATURE-512).
	if strings.Contains(got, "用哪个数据库？") || strings.Contains(got, "需要哪些能力？") {
		t.Errorf("taskInstructionCache = %q, should not repeat the question text", got)
	}
	for _, want := range []string{"Q1: MySQL", "Q2: 读写分离 + 自动备份"} {
		if !strings.Contains(got, want) {
			t.Errorf("taskInstructionCache = %q, want to contain %q", got, want)
		}
	}
}

// TestAskUserFreeTextQuestion verifies a question without options takes
// free-form text (UC-0013).
func TestAskUserFreeTextQuestion(t *testing.T) {
	io := &mockUserIO{inputs: []string{"请保留旧表"}}
	a := newAskUserAgent(io)
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{"title": "还有其他要说明的吗？"},
		},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}
	if got := a.taskInstructionCache.String(); !strings.Contains(got, "请保留旧表") {
		t.Errorf("taskInstructionCache = %q, want to contain 请保留旧表", got)
	}
}

// TestAskUserEmptyAnswer verifies an unanswered question is reported back with
// the "not answered" marker instead of being dropped (UC-0006/UC-0017).
func TestAskUserEmptyAnswer(t *testing.T) {
	io := &mockUserIO{inputs: []string{""}}
	a := newAskUserAgent(io)
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{"title": "需要哪些能力？", "options": []interface{}{"A", "B"}},
		},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}
	if got := a.taskInstructionCache.String(); !strings.Contains(got, i18n.T(i18n.KeyAskUserNoAnswer)) {
		t.Errorf("taskInstructionCache = %q, want the not-answered marker", got)
	}
}

// TestAskUserOptionNote verifies a note typed after the option number is kept
// with that option (UC-0011).
func TestAskUserOptionNote(t *testing.T) {
	io := &mockUserIO{inputs: []string{"2 每天凌晨执行"}}
	a := newAskUserAgent(io)
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{"title": "需要哪些能力？", "options": []interface{}{"读写分离", "自动备份"}},
		},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}
	got := a.taskInstructionCache.String()
	if !strings.Contains(got, "自动备份") || !strings.Contains(got, "每天凌晨执行") {
		t.Errorf("taskInstructionCache = %q, want to contain 自动备份 and 每天凌晨执行", got)
	}
}

// TestAskUserCancel verifies the cancel action aborts the tool (UC-0007).
func TestAskUserCancel(t *testing.T) {
	a := &Agent{
		io:                   &mockUserIO{},
		taskInstructionCache: bytes.Buffer{},
		interactionMgr:       &captureInteractionManager{askResult: InteractionResult{Action: ActionCancel}},
	}
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"question": "请选择处理方式",
		"options":  []interface{}{"立即执行", "稍后执行"},
	})
	if err == nil || !strings.Contains(err.Error(), "CANCEL_AGENT") {
		t.Errorf("expected CANCEL_AGENT error, got %v", err)
	}
}

// TestTerminalSelectKeyOption verifies askSelect matches a fixed key option
// (e.g. "-" or "+") and returns its Value (FEATURE-452).
func TestTerminalSelectKeyOption(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"-"}})
	res, err := m.Ask(context.Background(), Interaction{
		Kind:    InteractionSelect,
		Options: []string{"A", "B"},
		Keys: []KeyOption{
			{Label: "exit", Key: "-", Value: "exit"},
			{Label: "more", Key: "+", Value: "more"},
		},
	})
	if err != nil {
		t.Fatalf("Ask error: %v", err)
	}
	if res.Action != ActionSelect || res.Value != "exit" {
		t.Errorf("Action/Value = %q/%q, want select/exit", res.Action, res.Value)
	}
}

// TestTerminalSelectKeyOptionPlus verifies the "+" key option (FEATURE-452).
func TestTerminalSelectKeyOptionPlus(t *testing.T) {
	m := NewTerminalInteractionManager(&mockUserIO{inputs: []string{"+"}})
	res, err := m.Ask(context.Background(), Interaction{
		Kind:    InteractionSelect,
		Options: []string{"A", "B"},
		Keys: []KeyOption{
			{Label: "exit", Key: "-", Value: "exit"},
			{Label: "more", Key: "+", Value: "more"},
		},
	})
	if err != nil {
		t.Fatalf("Ask error: %v", err)
	}
	if res.Action != ActionSelect || res.Value != "more" {
		t.Errorf("Action/Value = %q/%q, want select/more", res.Action, res.Value)
	}
}
