// Author: L.Shuang
// Created: 2026-09-13
//
// FEATURE-514 unit tests: multi-line uniform-length loop detection and the
// per-turn context-remove limit.

package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// uniformLines builds n complete lines, each exactly lineLen characters long,
// with different content per line (so the periodic detector must not fire).
func uniformLines(n, lineLen int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(uniformLine(lineLen, i))
	}
	return sb.String()
}

// uniformLine builds one complete line of exactly lineLen characters (plus the
// trailing newline); the index keeps the content unique so the periodic
// detector cannot fire.
func uniformLine(lineLen, i int) string {
	digits := fmt.Sprintf("%d", i)
	pad := lineLen - 1 - len(digits)
	if pad < 0 {
		pad = 0
	}
	line := "L" + strings.Repeat("0", pad) + digits
	if len(line) > lineLen {
		line = line[:lineLen]
	}
	return line + "\n"
}

func TestUniformLine_ExactThresholdTriggers(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(100)

	err := ld.AddChunk(uniformLines(100, 16), time.Now())
	if err == nil {
		t.Fatal("100 consecutive same-length lines should trigger uniform_line_length")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	if ldErr.LoopType != "uniform_line_length" {
		t.Errorf("expected LoopType=uniform_line_length, got %q", ldErr.LoopType)
	}
}

func TestUniformLine_JustBelowThreshold(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(100)

	if err := ld.AddChunk(uniformLines(99, 16), time.Now()); err != nil {
		t.Fatalf("99 same-length lines must not trigger (threshold 100), got: %v", err)
	}
	// The 100th line completes the window and must trigger.
	if err := ld.AddChunk(uniformLine(16, 99), time.Now()); err == nil {
		t.Fatal("the 100th same-length line should trigger")
	}
}

func TestUniformLine_MixedLengthNoTrigger(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(100)

	lines := uniformLines(100, 16)
	// Make one line longer than the others.
	longer := "L" + strings.Repeat("0", 15) + "X\n"
	lines = strings.Replace(lines, uniformLine(16, 0), longer, 1)
	if err := ld.AddChunk(lines, time.Now()); err != nil {
		t.Fatalf("mixed line lengths must not trigger, got: %v", err)
	}
}

func TestUniformLine_CustomThreshold(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(5)

	if err := ld.AddChunk(uniformLines(4, 16), time.Now()); err != nil {
		t.Fatalf("4 lines below a threshold of 5 must not trigger, got: %v", err)
	}
	if err := ld.AddChunk(uniformLine(16, 4), time.Now()); err == nil {
		t.Fatal("5 same-length lines should trigger with threshold 5")
	}
}

func TestUniformLine_DisabledNoTrigger(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(0)

	if err := ld.AddChunk(uniformLines(200, 16), time.Now()); err != nil {
		t.Fatalf("threshold 0 disables the check, got: %v", err)
	}
}

func TestUniformLine_PeriodicTakesPrecedence(t *testing.T) {
	ld := NewLoopDetectorWithBlockLimit(5, 200)
	ld.SetUniformLineThreshold(100)

	// ABAB... (200 lines, both lines same length): the periodic detector runs
	// first and must classify this as multi_line, not uniform_line_length.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("AAAAAAAAAAAAAAAA\n")
		sb.WriteString("BBBBBBBBBBBBBBBB\n")
	}
	err := ld.AddChunk(sb.String(), time.Now())
	if err == nil {
		t.Fatal("ABAB repetition should trigger the periodic detector")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	if ldErr.LoopType != "multi_line" {
		t.Errorf("expected LoopType=multi_line (periodic precedence), got %q", ldErr.LoopType)
	}
}

func TestUniformLine_LoopTypeKeyAndI18n(t *testing.T) {
	if got := loopTypeKey("uniform_line_length"); got != i18n.KeyLoopTypeUniformLine {
		t.Errorf("loopTypeKey(uniform_line_length) = %q, want %q", got, i18n.KeyLoopTypeUniformLine)
	}
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLang(lang)
		if got := i18n.T(i18n.KeyLoopTypeUniformLine); got == "" || got == i18n.KeyLoopTypeUniformLine {
			t.Errorf("missing i18n label for %s: %q", lang, got)
		}
	}
	i18n.SetLang("zh")
}

func TestContextRemoveLimit_ZeroUnlimited(t *testing.T) {
	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{ContextRemoveLimit: 0}}}

	for i := 0; i < 10; i++ {
		if a.noteContextRemovalAndCheckLimit() {
			t.Fatalf("limit 0 must be unlimited (iteration %d)", i)
		}
	}
}

func TestContextRemoveLimit_Exceeded(t *testing.T) {
	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{ContextRemoveLimit: 1}}}

	if a.noteContextRemovalAndCheckLimit() {
		t.Fatal("the first removal must be allowed with limit 1")
	}
	if !a.noteContextRemovalAndCheckLimit() {
		t.Fatal("the second removal must exceed limit 1")
	}
}

func TestContextRemoveLimit_NilConfigUnlimited(t *testing.T) {
	a := &Agent{}
	if a.contextRemoveLimit() != 0 {
		t.Errorf("nil config must report limit 0, got %d", a.contextRemoveLimit())
	}
	if a.noteContextRemovalAndCheckLimit() {
		t.Fatal("nil config must never exceed the limit")
	}
}
// FEATURE-514: acceptance criteria + criteria_check tests.

func TestParseAcceptanceCriteria_ArrayAndString(t *testing.T) {
	arr, ok := parseAcceptanceCriteria([]interface{}{"a", " b ", ""})
	if !ok || len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Fatalf("array parsing failed: %v ok=%v", arr, ok)
	}

	str, ok := parseAcceptanceCriteria("- build passes\n- UI visible")
	if !ok || len(str) != 2 || str[0] != "build passes" || str[1] != "UI visible" {
		t.Fatalf("string parsing failed: %v ok=%v", str, ok)
	}

	if _, ok := parseAcceptanceCriteria(nil); ok {
		t.Fatal("nil input must report ok=false (keep existing criteria)")
	}
	if _, ok := parseAcceptanceCriteria(42); ok {
		t.Fatal("unsupported type must report ok=false")
	}
}

func TestFormatReviewAsText_IncludesCriteriaCheck(t *testing.T) {
	r := &SupervisorReview{
		Approved:   true,
		Reason:     "ok",
		Suggestion: "done",
		CriteriaCheck: []CriteriaCheckItem{
			{Criterion: "build passes", Met: true, Evidence: "go build ok"},
		},
	}
	out := formatReviewAsText(r)
	if !strings.Contains(out, "build passes") {
		t.Fatalf("criteria check missing in review text: %s", out)
	}
	if !strings.Contains(out, "go build ok") {
		t.Fatalf("evidence missing in review text: %s", out)
	}
}

func TestFormatReviewFeedback_ListsUnmetCriteria(t *testing.T) {
	r := &SupervisorReview{
		Approved:   false,
		Reason:     "missing tests",
		Suggestion: "add tests",
		CriteriaCheck: []CriteriaCheckItem{
			{Criterion: "build passes", Met: true},
			{Criterion: "UI shows criteria", Met: false, Evidence: "not implemented"},
		},
	}
	out := formatReviewFeedback(r)
	if !strings.Contains(out, "UI shows criteria") {
		t.Fatalf("unmet criterion missing in feedback: %s", out)
	}
	if strings.Contains(out, "build passes") {
		t.Fatalf("met criterion must not be listed as unmet: %s", out)
	}
}
