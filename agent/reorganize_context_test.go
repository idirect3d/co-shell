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

// TestCollapseAfterReorganize verifies that after reorganize_context is used,
// the message history collapses to [system, user(summary)] with no orphaned
// tool/assistant messages (FIX-318).
func TestCollapseAfterReorganize(t *testing.T) {
	tests := []struct {
		name          string
		messages      []llm.Message
		cacheSummary  string
		wantRoles     []string // expected roles after collapse
		wantPointer   int
		wantFlagReset bool
	}{
		{
			name: "OpenAI mode: assistant(tool_calls)+tool flushed summary",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "user", Content: "u1"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "reorganize_context"}}},
				{Role: "tool", Content: "done", ToolCallID: "t1"},
				{Role: "user", Content: "summary+env"},
			},
			cacheSummary:  "",
			wantRoles:     []string{"system", "user"},
			wantPointer:   1,
			wantFlagReset: true,
		},
		{
			name: "XML mode: tool result as user message",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "user", Content: "u1"},
				{Role: "assistant", Content: "<cs:reorganize_context>"},
				{Role: "user", Content: "tool result + summary + env"},
			},
			cacheSummary:  "",
			wantRoles:     []string{"system", "user"},
			wantPointer:   1,
			wantFlagReset: true,
		},
		{
			name: "non-streaming Run path: summary still in cache",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "user", Content: "u1"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "reorganize_context"}}},
				{Role: "tool", Content: "done", ToolCallID: "t1"},
			},
			cacheSummary:  "cached summary",
			wantRoles:     []string{"system", "user"},
			wantPointer:   1,
			wantFlagReset: true,
		},
		{
			name: "no user message: collapse to system only",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "reorganize_context"}}},
				{Role: "tool", Content: "done", ToolCallID: "t1"},
			},
			cacheSummary:  "",
			wantRoles:     []string{"system"},
			wantPointer:   1,
			wantFlagReset: true,
		},
		{
			name: "flag not set: no change",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "user", Content: "u1"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "execute_command"}}},
				{Role: "tool", Content: "out", ToolCallID: "t1"},
			},
			cacheSummary:  "",
			wantRoles:     []string{"system", "user", "assistant", "tool"},
			wantPointer:   0,
			wantFlagReset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Agent{}
			a.messages = append(a.messages, tt.messages...)
			a.reorganizeContextUsed = true
			if tt.cacheSummary != "" {
				a.taskInstructionCache.WriteString(tt.cacheSummary)
			}

			if tt.name == "flag not set: no change" {
				a.reorganizeContextUsed = false
			}

			a.collapseAfterReorganize()

			// Verify roles
			if len(a.messages) != len(tt.wantRoles) {
				t.Fatalf("got %d messages, want %d (roles=%v)", len(a.messages), len(tt.wantRoles), tt.wantRoles)
			}
			for i, want := range tt.wantRoles {
				if a.messages[i].Role != want {
					t.Errorf("message[%d] role = %q, want %q", i, a.messages[i].Role, want)
				}
			}

			// Verify no orphaned tool messages without preceding assistant tool_calls.
			for i, m := range a.messages {
				if m.Role == "tool" {
					preceded := false
					for j := i - 1; j >= 0; j-- {
						if a.messages[j].Role == "assistant" && len(a.messages[j].ToolCalls) > 0 {
							preceded = true
							break
						}
					}
					if !preceded {
						t.Errorf("message[%d] is an orphaned tool message (no preceding assistant tool_calls)", i)
					}
				}
			}

			if a.messagePointer != tt.wantPointer {
				t.Errorf("messagePointer = %d, want %d", a.messagePointer, tt.wantPointer)
			}
			// After collapse, the flag must always be reset (consumed) so a stale
			// flag never triggers a second collapse on a later iteration.
			if a.reorganizeContextUsed {
				t.Errorf("reorganizeContextUsed = %v, want false after collapse", a.reorganizeContextUsed)
			}

			// Cache must be cleared when flag was consumed.
			if tt.wantFlagReset && tt.cacheSummary != "" {
				if a.taskInstructionCache.Len() != 0 {
					t.Errorf("taskInstructionCache not cleared, still has %q", a.taskInstructionCache.String())
				}
			}
		})
	}
}

// TestCollapseAfterReorganize_UserContent verifies the surviving user message
// carries the summary (from cache in non-streaming path, or from the flushed
// last user message in streaming path).
func TestCollapseAfterReorganize_UserContent(t *testing.T) {
	tests := []struct {
		name        string
		messages    []llm.Message
		cacheString string
		wantUserSub string
	}{
		{
			name: "streaming path keeps flushed summary in last user message",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "reorganize_context"}}},
				{Role: "tool", Content: "done", ToolCallID: "t1"},
				{Role: "user", Content: "flushed summary + env"},
			},
			cacheString: "",
			wantUserSub: "flushed summary",
		},
		{
			name: "non-streaming path builds user from cache",
			messages: []llm.Message{
				{Role: "system", Content: "sys"},
				{Role: "assistant", Content: "a", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "reorganize_context"}}},
				{Role: "tool", Content: "done", ToolCallID: "t1"},
			},
			cacheString: "cached summary text",
			wantUserSub: "cached summary text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Agent{}
			a.messages = append(a.messages, tt.messages...)
			a.reorganizeContextUsed = true
			if tt.cacheString != "" {
				a.taskInstructionCache.WriteString(tt.cacheString)
			}

			a.collapseAfterReorganize()

			if len(a.messages) == 0 {
				t.Fatal("no messages after collapse")
			}
			last := a.messages[len(a.messages)-1]
			content := last.Content
			if len(last.ContentParts) > 0 {
				content = last.CombineContentParts()
			}
			if !strings.Contains(content, tt.wantUserSub) {
				t.Errorf("last user content = %q, want contains %q", content, tt.wantUserSub)
			}
		})
	}
}
