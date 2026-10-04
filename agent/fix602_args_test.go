// Package agent - FEATURE-602: a tool call whose `arguments` is not valid JSON
// must never stay in the conversation. Strict OpenAI-compatible servers (vLLM)
// re-parse every assistant tool_call's arguments on each request and reject the
// whole body with 400 once one of them is malformed, so a surviving bad call
// poisons the rest of the session. This pins the removal under the default
// parse-error-action ("prompt"), where the old code kept the message.
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// fix602ArgsClient replays one tool call with malformed arguments, then a valid
// attempt_completion so the turn can finish.
type fix602ArgsClient struct {
	*fix552Client
	calls int
}

func (c *fix602ArgsClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	i := c.calls
	c.calls++
	ch := make(chan llm.StreamEvent, 3)
	if i == 0 {
		// `"path"::` is not valid JSON: the arguments cannot be parsed.
		ch <- llm.StreamEvent{
			Type:     llm.StreamEventToolCall,
			ToolCall: &llm.ToolCall{ID: "call-bad", Name: "read_file", Arguments: `{"path":: "/tmp/x"}`},
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "tool_calls"}
	} else {
		ch <- llm.StreamEvent{
			Type: llm.StreamEventToolCall,
			ToolCall: &llm.ToolCall{
				ID:   "call-done",
				Name: "attempt_completion",
				Arguments: `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},` +
					`"result":"done","session_title":"t","session_keywords":"k"}`,
			},
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "tool_calls"}
	}
	close(ch)
	return ch, nil
}

// TestFeature602MalformedToolArgumentsAreDroppedFromHistory is the FEATURE-602
// acceptance test: with the default parse-error-action "prompt" the malformed
// assistant message (and its result) must be gone from a.messages afterwards.
func TestFeature602MalformedToolArgumentsAreDroppedFromHistory(t *testing.T) {
	client := &fix602ArgsClient{fix552Client: &fix552Client{}}
	ag := newFix552Agent(t, client, "prompt")
	// The user's config runs parse-error-action=prompt; the old code only removed
	// the malformed call when the action was "retry" in OpenAI mode.
	ag.cfg.LLM.ParseErrorAction = "prompt"

	if _, err := ag.RunStream(context.Background(), "do something", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream: %v", err)
	}

	for i, m := range ag.messages {
		for j, tc := range m.ToolCalls {
			if strings.Contains(tc.Arguments, `"path"::`) {
				t.Fatalf("malformed tool call survived at messages[%d].tool_calls[%d]: %s", i, j, tc.Arguments)
			}
			if tc.Name == "read_file" {
				t.Fatalf("the failing tool call must not remain: messages[%d].tool_calls[%d]", i, j)
			}
		}
	}
}
