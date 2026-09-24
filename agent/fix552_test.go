// Package agent - FIX-552 tests: no-tool-action=prompt keeps the last
// assistant output in history (simulate a Q&A) before appending the
// continue prompt; no-tool-action=retry still discards it.
//
// Author: L.Shuang
// Created: 2026-09-24
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// fix552Step is one scripted LLM response.
type fix552Step struct {
	content string // plain content reply (no tool call)
	tool    string // tool name to call (e.g. "attempt_completion"); empty = content reply
}

// fix552Client replays a fixed script of LLM responses, one per ChatStream call.
type fix552Client struct {
	calls  int
	script []fix552Step
}

func (f *fix552Client) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Content: "ok"}, nil
}

func (f *fix552Client) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	i := f.calls
	f.calls++
	ch := make(chan llm.StreamEvent, 3)
	if i < len(f.script) {
		step := f.script[i]
		if step.tool != "" {
			ch <- llm.StreamEvent{
				Type: llm.StreamEventToolCall,
				ToolCall: &llm.ToolCall{
					ID:        "call-1",
					Name:      step.tool,
					Arguments: `{"meta":{"intent":"complete","risk":"low","risk_reason":"done","affected_objects":[],"progress":[]},"result":"done","session_title":"t","session_keywords":"k"}`,
				},
			}
			ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "tool_calls"}
		} else {
			ch <- llm.StreamEvent{Type: llm.StreamEventContent, Content: step.content}
			ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "stop"}
		}
	} else {
		ch <- llm.StreamEvent{Type: llm.StreamEventContent, Content: "done"}
		ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "stop"}
	}
	close(ch)
	return ch, nil
}

func (f *fix552Client) ListModels(ctx context.Context) ([]llm.ModelInfo, error) { return nil, nil }
func (f *fix552Client) TestVisionSupport(ctx context.Context) bool              { return false }
func (f *fix552Client) TestTextSupport(ctx context.Context) bool                { return true }
func (f *fix552Client) TestToolCallSupport(ctx context.Context) bool            { return true }
func (f *fix552Client) TestThinkingSupport(ctx context.Context) bool            { return false }
func (f *fix552Client) SetThinkingEnabled(enabled bool)                         {}
func (f *fix552Client) SetReasoningEffort(effort string)                        {}
func (f *fix552Client) SetTopP(topP float64)                                    {}
func (f *fix552Client) SetTopK(topK int)                                        {}
func (f *fix552Client) SetRepetitionPenalty(penalty float64)                    {}
func (f *fix552Client) SetTokenUsage(mode string)                               {}
func (f *fix552Client) SetTemperature(temp float64)                             {}
func (f *fix552Client) SetBodyAdditions(additions map[string]string)            {}
func (f *fix552Client) RemoveBodyAddition(key string)                           {}
func (f *fix552Client) GetBodyAdditions() map[string]string                     { return nil }
func (f *fix552Client) Close() error                                            { return nil }

// newFix552Agent builds an agent with tool calls enabled and the given
// no-tool-action strategy.
func newFix552Agent(t *testing.T, client llm.Client, noToolAction string) *Agent {
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
	cfg := &config.Config{}
	cfg.LLM.NoToolAction = noToolAction
	ag.SetConfig(cfg)
	ag.SetToolCallEnabled(true)
	ag.SetToolCallMode("openai")
	ag.SetPlanEnabled(true)
	ag.SyncToolModes(&config.Config{})
	return ag
}

// TestNoToolActionPromptKeepsAssistant verifies UC-0001-01: with
// no-tool-action=prompt, a 0-tool-call iteration keeps the assistant output in
// a.messages and appends the continue prompt (simulate a Q&A).
func TestNoToolActionPromptKeepsAssistant(t *testing.T) {
	client := &fix552Client{script: []fix552Step{
		{content: "first answer"}, // 0 tool calls -> prompt branch
		{tool: "attempt_completion"},
	}}
	ag := newFix552Agent(t, client, "prompt")

	if _, err := ag.RunStream(context.Background(), "do something", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream: %v", err)
	}

	// The assistant "first answer" must be kept in history.
	foundAssistant := false
	foundContinue := false
	for _, m := range ag.messages {
		if m.Role == "assistant" && strings.Contains(m.Content, "first answer") {
			foundAssistant = true
		}
		if m.Role == "user" && strings.Contains(m.CombineContentParts(), i18n.T(i18n.KeyContinuePrompt)) {
			foundContinue = true
		}
	}
	if !foundAssistant {
		t.Errorf("prompt mode: assistant 'first answer' was NOT kept in a.messages")
	}
	if !foundContinue {
		t.Errorf("prompt mode: continue prompt was NOT appended as a user message")
	}
}

// TestNoToolActionRetryDiscardsAssistant verifies UC-0001-02: with
// no-tool-action=retry, a 0-tool-call iteration discards the assistant output.
func TestNoToolActionRetryDiscardsAssistant(t *testing.T) {
	client := &fix552Client{script: []fix552Step{
		{content: "first answer"}, // 0 tool calls -> retry branch (discard)
		{tool: "attempt_completion"},
	}}
	ag := newFix552Agent(t, client, "retry")

	if _, err := ag.RunStream(context.Background(), "do something", func(ev StreamEvent) {}); err != nil {
		t.Fatalf("RunStream: %v", err)
	}

	for _, m := range ag.messages {
		if m.Role == "assistant" && strings.Contains(m.Content, "first answer") {
			t.Errorf("retry mode: assistant 'first answer' should have been discarded, but it is in a.messages")
		}
	}
}
