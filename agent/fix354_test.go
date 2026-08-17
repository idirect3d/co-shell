// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
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

// FIX-354: refreshLastUserEnvelope must not reach back across tool messages
// to rewrite a mid-history user message's <time> on every iteration — doing
// so breaks prefix caching for the whole suffix after that message (observed
// in production: cache hit frozen at the system prompt, ~55K tokens
// recomputed every request).
//
// loopRetryTestMsg / sampleEnv are shared with loop_retry_test.go (same
// package).

func lastPartText(m llm.Message) string {
	if len(m.ContentParts) == 0 {
		return ""
	}
	return m.ContentParts[len(m.ContentParts)-1].Text
}

// User message still at the tail (only assistant messages after it): the
// time refresh keeps working.
func TestFIX354_RefreshWhenUserMessageAtTail(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			{Role: "assistant", Content: "好的"},
		},
	}

	a.refreshLastUserEnvelope()

	got := lastPartText(a.messages[1])
	if strings.Contains(got, "<time>2026-08-03 10:00:00 Monday</time>") {
		t.Errorf("time was not refreshed: %q", got)
	}
	if !strings.Contains(got, "<time>") {
		t.Errorf("time tag missing after refresh: %q", got)
	}
}

// Tool messages after the last user message (tool-call iterations underway):
// the user message is mid-history and must be left untouched.
func TestFIX354_NoRefreshAcrossToolMessages(t *testing.T) {
	tb := loopRetryTestMsg{}
	a := &Agent{
		messages: []llm.Message{
			{Role: "system", Content: "system"},
			tb.buildUser("请帮我调研"),
			{Role: "assistant", Content: "好的"},
			{Role: "tool", Content: "tool result"},
			{Role: "assistant", Content: "继续"},
			{Role: "tool", Content: "tool result 2"},
		},
	}

	a.refreshLastUserEnvelope()

	got := lastPartText(a.messages[1])
	if !strings.Contains(got, "<time>2026-08-03 10:00:00 Monday</time>") {
		t.Errorf("mid-history user message time was rewritten: %q", got)
	}
}
