// Author: L.Shuang
// Created: 2026-08-03
// Last Modified: 2026-08-03
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
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// loopRetryTestMsg is a minimal service for building test messages.
type loopRetryTestMsg struct{}

// buildUser creates a real user message: ContentParts [instruction, env].
func (loopRetryTestMsg) buildUser(instruction string) llm.Message {
	return llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: instruction},
			{Type: llm.ContentPartText, Text: sampleEnv("")},
		},
	}
}

// buildPlainUser creates a plain-Content user message (no env, no ContentParts).
func (loopRetryTestMsg) buildPlainUser(instruction string) llm.Message {
	return llm.Message{Role: "user", Content: instruction}
}

// buildFeedback creates a loop feedback message with the given counter.
func (loopRetryTestMsg) buildFeedback(tip string, retry int) llm.Message {
	env := sampleEnv("")
	env = setLoopFeedbackInText(env, true)
	env = setRetriedCountInText(env, retry)
	return llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: tip},
			{Type: llm.ContentPartText, Text: env},
		},
	}
}

// buildAssistant creates a minimal assistant message (loop content).
func (loopRetryTestMsg) buildAssistant(content string) llm.Message {
	return llm.Message{Role: "assistant", Content: content}
}

// sampleEnv returns a minimal <environment_details> block for testing.
func sampleEnv(extra string) string {
	return "<environment_details>\n" +
		"<time>2026-08-03 10:00:00 Monday</time>\n" +
		"<cwd>/test</cwd>\n" +
		extra +
		"</environment_details>"
}

// lastUserMsg returns the last user message in a.messages.
func lastUserMsg(a *Agent) llm.Message {
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "user" {
			return a.messages[i]
		}
	}
	return llm.Message{}
}

// msgEnv returns the env text of the last ContentPart of msg ("" when absent).
func msgEnv(m *llm.Message) string {
	return lastEnvText(m)
}

// msgFirstText returns the first text part (instruction / feedback) of msg.
func msgFirstText(m *llm.Message) string {
	if m == nil || len(m.ContentParts) == 0 {
		return m.Content
	}
	return m.ContentParts[0].Text
}

// retryEnv returns the <retried_count> value in the env text.
func retryEnv(env string) int {
	return getRetriedCountFromText(env)
}

// TestLoopRetry_PromptFirstTrigger verifies UC-0001:
// prompt first trigger appends a new feedback message with env + tags.
func TestLoopRetry_PromptFirstTrigger(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("请换个思路")

	if count != 1 {
		t.Fatalf("first prompt intervention should record retried_count=1, got %d", count)
	}
	if len(a.messages) != before+1 {
		t.Fatalf("expected 1 new message (%d -> %d), got %d -> %d", before, before+1, len(a.messages), len(a.messages))
	}

	last := a.messages[len(a.messages)-1]
	if last.Role != "user" {
		t.Fatalf("new message should be user role, got %q", last.Role)
	}
	if len(last.ContentParts) != 2 {
		t.Fatalf("new feedback message should have 2 ContentParts, got %d", len(last.ContentParts))
	}
	if got := msgFirstText(&last); got != "请换个思路" {
		t.Errorf("feedback text mismatch: got %q", got)
	}
	env := msgEnv(&last)
	if !isLoopFeedbackText(env) {
		t.Errorf("feedback env should carry <loop_feedback>true</loop_feedback>, got:\n%s", env)
	}
	if got := retryEnv(env); got != 1 {
		t.Errorf("feedback env should carry <retried_count>1</retried_count>, got %d", got)
	}

	// The original user message must be untouched.
	orig := a.messages[1]
	if got := msgFirstText(&orig); got != "请帮我调研" {
		t.Errorf("original user instruction was modified: %q", got)
	}
	if origEnv := msgEnv(&orig); strings.Contains(origEnv, "<loop_feedback>") || strings.Contains(origEnv, "<retried_count>") {
		t.Errorf("original user env was modified:\n%s", origEnv)
	}
}

// TestLoopRetry_PromptUpdateInPlace verifies UC-0002:
// subsequent prompt interventions update the existing feedback message in place.
func TestLoopRetry_PromptUpdateInPlace(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
			tb.buildFeedback("请换个思路", 1),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("换一个更具体的思路")

	if count != 2 {
		t.Fatalf("second prompt intervention should record retried_count=2, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("no new message should be added on update (%d)", len(a.messages))
	}

	last := a.messages[len(a.messages)-1]
	if got := msgFirstText(&last); got != "换一个更具体的思路" {
		t.Errorf("feedback text should be replaced, got %q", got)
	}
	env := msgEnv(&last)
	if !isLoopFeedbackText(env) {
		t.Errorf("<loop_feedback>true</loop_feedback> should be preserved")
	}
	if got := retryEnv(env); got != 2 {
		t.Errorf("retried_count should be 2, got %d", got)
	}
}

// TestLoopRetry_PromptCountOnUserMsg verifies UC-0003:
// prompt intervention when the count lives on a real user message appends a NEW
// feedback message and never overwrites the user message.
func TestLoopRetry_PromptCountOnUserMsg(t *testing.T) {
	tb := loopRetryTestMsg{}
	// The last user message carries retried_count=2 (added by a previous retry),
	// but is NOT a loop feedback message.
	userEnv := setRetriedCountInText(sampleEnv(""), 2)
	userMsg := llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: "请帮我调研"},
			{Type: llm.ContentPartText, Text: userEnv},
		},
	}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			userMsg,
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("请换个思路")

	if count != 1 {
		t.Fatalf("new feedback chain should start at retried_count=1, got %d", count)
	}
	if len(a.messages) != before+1 {
		t.Fatalf("expected 1 new message, got %d -> %d", before, len(a.messages))
	}

	last := a.messages[len(a.messages)-1]
	env := msgEnv(&last)
	if !isLoopFeedbackText(env) {
		t.Errorf("new message should be a loop feedback message")
	}
	if got := retryEnv(env); got != 1 {
		t.Errorf("new feedback retried_count should be 1, got %d", got)
	}

	// The original user message and its retried_count=2 must be preserved.
	orig := a.messages[1]
	if got := msgFirstText(&orig); got != "请帮我调研" {
		t.Errorf("original user instruction was modified: %q", got)
	}
	if got := retryEnv(msgEnv(&orig)); got != 2 {
		t.Errorf("original user retried_count should stay 2, got %d", got)
	}
	if isLoopFeedbackText(msgEnv(&orig)) {
		t.Errorf("original user message should NOT gain loop_feedback marker")
	}
}

// TestLoopRetry_RetryFirstTrigger verifies UC-0004:
// retry does not append a message; it increments the last user message env.
func TestLoopRetry_RetryFirstTrigger(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("")

	if count != 1 {
		t.Fatalf("retry first trigger should record retried_count=1, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("retry should NOT append a message (%d -> %d)", before, len(a.messages))
	}

	last := lastUserMsg(a)
	if got := msgFirstText(&last); got != "请帮我调研" {
		t.Errorf("user instruction should be unchanged, got %q", got)
	}
	env := msgEnv(&last)
	if isLoopFeedbackText(env) {
		t.Errorf("retry should NOT add loop_feedback marker")
	}
	if got := retryEnv(env); got != 1 {
		t.Errorf("retried_count should be 1, got %d", got)
	}
}

// TestLoopRetry_RetryIncrement verifies UC-0005:
// subsequent retries increment the counter on the user message.
func TestLoopRetry_RetryIncrement(t *testing.T) {
	tb := loopRetryTestMsg{}
	userEnv := setRetriedCountInText(sampleEnv(""), 1)
	userMsg := llm.Message{
		Role: "user",
		ContentParts: []llm.ContentPart{
			{Type: llm.ContentPartText, Text: "请帮我调研"},
			{Type: llm.ContentPartText, Text: userEnv},
		},
	}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			userMsg,
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("")

	if count != 2 {
		t.Fatalf("second retry should record retried_count=2, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("retry should NOT append a message")
	}
	last := lastUserMsg(a)
	if got := msgFirstText(&last); got != "请帮我调研" {
		t.Errorf("user instruction should be unchanged, got %q", got)
	}
	if got := retryEnv(msgEnv(&last)); got != 2 {
		t.Errorf("retried_count should be 2, got %d", got)
	}
}

// TestLoopRetry_RetryOnFeedback verifies UC-0006:
// retry on a feedback message increments the counter and keeps the feedback text.
func TestLoopRetry_RetryOnFeedback(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
			tb.buildFeedback("请换个思路", 1),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("")

	if count != 2 {
		t.Fatalf("retry on feedback should record retried_count=2, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("retry should NOT append a message")
	}
	last := a.messages[len(a.messages)-1]
	if got := msgFirstText(&last); got != "请换个思路" {
		t.Errorf("feedback text should be preserved on retry, got %q", got)
	}
	env := msgEnv(&last)
	if !isLoopFeedbackText(env) {
		t.Errorf("loop_feedback marker should be preserved")
	}
	if got := retryEnv(env); got != 2 {
		t.Errorf("retried_count should be 2, got %d", got)
	}
}

// TestLoopRetry_TemperatureLikeRetry verifies UC-0007 subset:
// temperature strategy passes empty feedback to applyLoopFeedback, so the message
// behavior must be identical to retry. The temperature adjustment itself belongs
// to FEATURE-230 and is out of scope here.
func TestLoopRetry_TemperatureLikeRetry(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("")

	if count != 1 {
		t.Fatalf("temperature-like feedback should record retried_count=1, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("temperature-like feedback should NOT append a message")
	}
	last := lastUserMsg(a)
	if got := retryEnv(msgEnv(&last)); got != 1 {
		t.Errorf("retried_count should be 1, got %d", got)
	}
	if isLoopFeedbackText(msgEnv(&last)) {
		t.Errorf("temperature-like feedback should NOT add loop_feedback marker")
	}
}

// TestLoopRetry_RetryOnPlainMessage verifies UC-0008:
// retry on a plain-Content user message (no env) builds a full env block first.
func TestLoopRetry_RetryOnPlainMessage(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildPlainUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
		},
	}

	before := len(a.messages)
	count := a.applyLoopFeedback("")

	if count != 1 {
		t.Fatalf("retry on plain message should record retried_count=1, got %d", count)
	}
	if len(a.messages) != before {
		t.Fatalf("retry should NOT append a message")
	}
	last := lastUserMsg(a)
	if got := msgFirstText(&last); got != "请帮我调研" {
		t.Errorf("original plain text should be preserved, got %q", got)
	}
	env := msgEnv(&last)
	if env == "" {
		t.Fatal("plain message should gain a full env block")
	}
	if !strings.Contains(env, "<cwd>") || !strings.Contains(env, "<environment_details>") {
		t.Errorf("env block should be complete, got:\n%s", env)
	}
	if got := retryEnv(env); got != 1 {
		t.Errorf("retried_count should be 1, got %d", got)
	}
}

// TestLoopRetry_FeedbackEnvComplete verifies UC-0009:
// the feedback message env carries the full standard environment blocks.
func TestLoopRetry_FeedbackEnvComplete(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			tb.buildAssistant("循环内容"),
		},
	}

	a.applyLoopFeedback("请换个思路")
	last := a.messages[len(a.messages)-1]
	env := msgEnv(&last)

	for _, block := range []string{
		"<time>", "<message_no>", "<context_window>", "<cwd>",
		"<files>", "<opened_resources>", "<loop_feedback>", "<retried_count>",
	} {
		if !strings.Contains(env, block) {
			t.Errorf("feedback env should contain %q, got:\n%s", block, env)
		}
	}
}

// TestLoopRetry_MultiChain verifies UC-0010:
// a loop chain keeps exactly one feedback message with an increasing counter;
// a fresh user request starts a new independent chain.
func TestLoopRetry_MultiChain(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
		},
	}

	// Chain 1: three prompt interventions with assistant loop content in between.
	// NOTE: In the real agent loop the interrupted assistant content is NOT
	// appended to a.messages (loop detection fires during streaming), but the
	// test still appends assistant messages to prove that applyLoopFeedback
	// locates the last USER message regardless of trailing assistant messages.
	for i := 1; i <= 3; i++ {
		if i > 1 {
			a.messages = append(a.messages, tb.buildAssistant("循环内容"))
		}
		a.applyLoopFeedback("请换个思路")
		last := lastUserMsg(a)
		env := msgEnv(&last)
		if !isLoopFeedbackText(env) {
			t.Fatalf("intervention %d should still be a feedback message (last user msg)", i)
		}
		if got := retryEnv(env); got != i {
			t.Fatalf("intervention %d should record retried_count=%d, got %d", i, i, got)
		}
	}

	// Count feedback messages in chain 1: exactly one.
	feedbackCount := 0
	for _, m := range a.messages {
		if m.Role == "user" && isLoopFeedbackMessage(&m) {
			feedbackCount++
		}
	}
	if feedbackCount != 1 {
		t.Fatalf("chain 1 should have exactly 1 feedback message, got %d", feedbackCount)
	}

	// New user request appends a fresh real user message.
	a.messages = append(a.messages, tb.buildUser("新的任务"))
	// New chain: prompt intervention after the new user message.
	a.messages = append(a.messages, tb.buildAssistant("循环内容"))
	a.applyLoopFeedback("新链反馈")

	// The new feedback chain must be independent (retried_count=1).
	last := lastUserMsg(a)
	if got := retryEnv(msgEnv(&last)); got != 1 {
		t.Fatalf("new chain should start at retried_count=1, got %d", got)
	}
	if got := msgFirstText(&last); got != "新链反馈" {
		t.Fatalf("new chain feedback text mismatch: %q", got)
	}
	feedbackCount = 0
	for _, m := range a.messages {
		if m.Role == "user" && isLoopFeedbackMessage(&m) {
			feedbackCount++
		}
	}
	if feedbackCount != 2 {
		t.Fatalf("two independent chains should yield 2 feedback messages, got %d", feedbackCount)
	}
}

// TestLoopRetry_NoUserMessage verifies the guard when history has no user message.
func TestLoopRetry_NoUserMessage(t *testing.T) {
	a := &Agent{messages: []llm.Message{{Role: "system", Content: "system"}}}
	if got := a.applyLoopFeedback("请换个思路"); got != 0 {
		t.Fatalf("no user message should return 0, got %d", got)
	}
	if got := a.applyLoopFeedback(""); got != 0 {
		t.Fatalf("no user message (retry) should return 0, got %d", got)
	}
	if len(a.messages) != 1 {
		t.Fatalf("no message should be appended, got %d", len(a.messages))
	}
}
