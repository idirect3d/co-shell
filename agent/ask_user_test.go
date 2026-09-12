// Package agent - tests for the ask_user multi-question tool (FEATURE-512).
//
// Author: L.Shuang
// Created: 2026-09-13
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// TestParseQuestionsXMLShape verifies the XML shape produced by
// parseXMLChildrenToJSON (repeated <item> tags) is normalised into questions
// with their options and multi flag (UC-0001).
func TestParseQuestionsXMLShape(t *testing.T) {
	// XML arguments are parsed with the configured tag prefix (the same prefix
	// the system prompt advertises), so set it explicitly for this test.
	prevPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(prevPrefix)

	xmlContent := `<cs:questions>` +
		`<cs:item><cs:title>用哪个数据库？</cs:title><cs:options><cs:item>MySQL</cs:item><cs:item>PostgreSQL</cs:item></cs:options><cs:required>true</cs:required></cs:item>` +
		`<cs:item><cs:title>需要哪些能力？</cs:title><cs:options><cs:item>读写分离</cs:item><cs:item>自动备份</cs:item></cs:options><cs:multi>true</cs:multi></cs:item>` +
		`</cs:questions>`

	jsonStr, parseErrors := parseXMLChildrenToJSON(xmlContent)
	if len(parseErrors) > 0 {
		t.Fatalf("parseXMLChildrenToJSON errors: %v", parseErrors)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		t.Fatalf("unmarshal XML args %q: %v", jsonStr, err)
	}

	questions := normalizeQuestions(args)
	if len(questions) != 2 {
		t.Fatalf("questions = %d, want 2 (args: %s)", len(questions), jsonStr)
	}
	if questions[0].Title != "用哪个数据库？" {
		t.Errorf("Q1 title = %q, want 用哪个数据库？", questions[0].Title)
	}
	if len(questions[0].Options) != 2 || questions[0].Options[0] != "MySQL" || questions[0].Options[1] != "PostgreSQL" {
		t.Errorf("Q1 options = %v, want [MySQL PostgreSQL]", questions[0].Options)
	}
	if questions[1].Title != "需要哪些能力？" {
		t.Errorf("Q2 title = %q, want 需要哪些能力？", questions[1].Title)
	}
	if !questions[1].Multi {
		t.Errorf("Q2 multi = false, want true")
	}
	if !questions[0].AllowNote {
		t.Errorf("allow_note should default to true")
	}
	if !questions[0].Required {
		t.Errorf("Q1 required = false, want true (XML <required>)")
	}
	if questions[1].Required {
		t.Errorf("Q2 required = true, want default false")
	}
}

// TestNormalizeQuestionsJSONShape verifies the JSON/OpenAI shape (array of
// objects) with explicit flags (UC-0002/UC-0004).
func TestNormalizeQuestionsJSONShape(t *testing.T) {
	args := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"title":    "用哪个数据库？",
				"options":  []interface{}{"MySQL", "PostgreSQL"},
				"required": true,
			},
			map[string]interface{}{
				"title":      "需要哪些能力？",
				"options":    []interface{}{"读写分离", "自动备份"},
				"multi":      true,
				"allow_note": false,
			},
		},
	}
	questions := normalizeQuestions(args)
	if len(questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(questions))
	}
	if questions[0].Multi {
		t.Errorf("Q1 multi = true, want default false")
	}
	if !questions[0].AllowNote {
		t.Errorf("Q1 allow_note = false, want default true")
	}
	if !questions[1].Multi {
		t.Errorf("Q2 multi = false, want true")
	}
	if questions[1].AllowNote {
		t.Errorf("Q2 allow_note = true, want explicit false")
	}
	if !questions[0].Required {
		t.Errorf("Q1 required = false, want explicit true (JSON)")
	}
	if questions[1].Required {
		t.Errorf("Q2 required = true, want default false")
	}
}

// TestAskUserInteractionShape verifies the interaction handed to the manager
// (UC-0005): kind, questions and the fixed exit key.
func TestAskUserInteractionShape(t *testing.T) {
	mgr := &captureInteractionManager{}
	a := &Agent{
		io:                   &mockUserIO{},
		taskInstructionCache: bytes.Buffer{},
		interactionMgr:       mgr,
	}
	_, err := a.askUserTool(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{"title": "Q1", "options": []interface{}{"A", "B"}},
			map[string]interface{}{"title": "Q2", "options": []interface{}{"C"}, "multi": true, "allow_note": false},
		},
	})
	if err != nil {
		t.Fatalf("askUserTool error: %v", err)
	}

	in := mgr.captured
	if in.Kind != InteractionQuestions {
		t.Fatalf("kind = %q, want %q", in.Kind, InteractionQuestions)
	}
	if len(in.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(in.Questions))
	}
	if !in.Questions[1].Multi || in.Questions[1].AllowNote {
		t.Errorf("Q2 flags = multi:%v allow_note:%v, want true/false", in.Questions[1].Multi, in.Questions[1].AllowNote)
	}
	exitKey := false
	for _, k := range in.Keys {
		if k.Key == "-" && k.Value == "think_exit" {
			exitKey = true
		}
	}
	if !exitKey {
		t.Errorf("fixed exit key missing: %+v", in.Keys)
	}
}

// TestAskUserToolDeclaration verifies the registered tool name and schema
// (UC-0008).
func TestAskUserToolDeclaration(t *testing.T) {
	a := &Agent{}
	tool := a.buildAskUserTool()
	if tool.Name != "ask_user" {
		t.Errorf("tool name = %q, want ask_user", tool.Name)
	}
	if tool.Callback == nil {
		t.Error("callback missing")
	}
	props, _ := tool.Parameters["properties"].(map[string]interface{})
	if _, ok := props["questions"]; !ok {
		t.Errorf("questions parameter missing: %+v", props)
	}
	if _, ok := props["meta"]; !ok {
		t.Errorf("meta parameter missing: %+v", props)
	}
	qProps, _ := props["questions"].(map[string]interface{})
	items, _ := qProps["items"].(map[string]interface{})
	itemProps, _ := items["properties"].(map[string]interface{})
	if _, ok := itemProps["required"]; !ok {
		t.Errorf("questions[].required schema property missing: %+v", itemProps)
	}
}

// TestQuestionsInteractionJSONRoundTrip verifies the interaction and its answers
// survive a JSON round trip (UC-0012).
func TestQuestionsInteractionJSONRoundTrip(t *testing.T) {
	in := Interaction{
		Kind: InteractionQuestions,
		Questions: []Question{
			{Title: "Q1", Options: []string{"A", "B"}, AllowNote: true},
			{Title: "Q2", Options: []string{"X", "Y"}, Multi: true},
		},
	}
	blob, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal interaction: %v", err)
	}
	var back Interaction
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal interaction: %v", err)
	}
	if back.Kind != InteractionQuestions || len(back.Questions) != 2 {
		t.Fatalf("round trip lost questions: %+v", back)
	}
	if !back.Questions[0].AllowNote || !back.Questions[1].Multi {
		t.Errorf("round trip lost flags: %+v", back.Questions)
	}

	res := InteractionResult{
		Action: ActionSubmit,
		Answers: []QuestionAnswer{{
			Question: "Q1",
			Selected: []string{"A"},
			Notes:    []AnswerNote{{Option: "A", Note: "细节"}},
		}},
	}
	blob, err = json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var resBack InteractionResult
	if err := json.Unmarshal(blob, &resBack); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if resBack.Action != ActionSubmit || len(resBack.Answers) != 1 {
		t.Fatalf("round trip lost answers: %+v", resBack)
	}
	if len(resBack.Answers[0].Selected) != 1 || resBack.Answers[0].Selected[0] != "A" {
		t.Errorf("selected = %v, want [A]", resBack.Answers[0].Selected)
	}
	if len(resBack.Answers[0].Notes) != 1 || resBack.Answers[0].Notes[0].Option != "A" {
		t.Errorf("notes = %+v, want one note on A", resBack.Answers[0].Notes)
	}
}

// TestSplitOptionNumbers covers the terminal multi-choice parsing helper
// (UC-0010).
func TestSplitOptionNumbers(t *testing.T) {
	cases := []struct {
		in       string
		wantNums []int
		wantNote string
	}{
		{"1", []int{1}, ""},
		{"1,3", []int{1, 3}, ""},
		{"1,3 每天凌晨执行", []int{1, 3}, "每天凌晨执行"},
		{"2 需要兼容 PG15", []int{2}, "需要兼容 PG15"},
		{"任意内容", nil, "任意内容"},
	}
	for _, c := range cases {
		nums, note := splitOptionNumbers(c.in)
		if len(nums) != len(c.wantNums) {
			t.Errorf("splitOptionNumbers(%q) nums = %v, want %v", c.in, nums, c.wantNums)
			continue
		}
		for i := range nums {
			if nums[i] != c.wantNums[i] {
				t.Errorf("splitOptionNumbers(%q) nums = %v, want %v", c.in, nums, c.wantNums)
				break
			}
		}
		if note != c.wantNote {
			t.Errorf("splitOptionNumbers(%q) note = %q, want %q", c.in, note, c.wantNote)
		}
	}
}

// TestFormatQuestionAnswersCompact verifies the compact answer text shared with
// the web UI: one line per question, no repeated question text, per-option
// notes rendered as （备注: A→xxx） and unanswered questions marked (FEATURE-512).
func TestFormatQuestionAnswersCompact(t *testing.T) {
	got := formatQuestionAnswers([]QuestionAnswer{
		{Question: "用哪个数据库？", Selected: []string{"MySQL", "PostgreSQL"}, Notes: []AnswerNote{{Option: "MySQL", Note: "带只读实例"}}},
		{Question: "还有其他说明吗？", Text: "请保留旧表"},
		{Question: "需要哪些能力？", Selected: []string{"读写分离"}},
		{Question: "部署环境？"},
	})
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("formatQuestionAnswers lines = %d (%q), want 4", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "Q1: MySQL + PostgreSQL") {
		t.Errorf("line 1 = %q, want it to start with Q1: MySQL + PostgreSQL", lines[0])
	}
	if !strings.Contains(lines[0], "备注: MySQL→带只读实例") {
		t.Errorf("line 1 = %q, want the note marker 备注: MySQL→带只读实例", lines[0])
	}
	if lines[1] != "Q2: 请保留旧表" {
		t.Errorf("line 2 = %q, want Q2: 请保留旧表", lines[1])
	}
	if lines[2] != "Q3: 读写分离" {
		t.Errorf("line 3 = %q, want Q3: 读写分离", lines[2])
	}
	if lines[3] != "Q4: "+i18n.T(i18n.KeyAskUserNoAnswer) {
		t.Errorf("line 4 = %q, want the not-answered marker", lines[3])
	}
	if strings.Contains(got, "用哪个数据库？") {
		t.Errorf("answer text should not repeat the question text: %q", got)
	}
}
