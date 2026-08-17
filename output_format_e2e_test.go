// Package main - end-to-end output format consistency test (FEATURE-307b).
// Runs the same agent on the same input twice — once through LineRenderer
// (text) and once through StreamRenderer (JSON-Lines) — and asserts both
// runs observe the identical event sequence and content.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// fakeLLMClient is a scriptable llm.Client for end-to-end renderer tests.
// ChatStream replays a fixed content stream; every other method is a no-op
// stub satisfying the interface.
type fakeLLMClient struct {
	chunks []string
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Content: strings.Join(f.chunks, "")}, nil
}

func (f *fakeLLMClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, len(f.chunks)+1)
	for _, c := range f.chunks {
		ch <- llm.StreamEvent{Type: llm.StreamEventContent, Content: c}
	}
	ch <- llm.StreamEvent{Type: llm.StreamEventDone, Done: true, FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (f *fakeLLMClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}
func (f *fakeLLMClient) TestVisionSupport(ctx context.Context) bool   { return false }
func (f *fakeLLMClient) TestTextSupport(ctx context.Context) bool     { return true }
func (f *fakeLLMClient) TestToolCallSupport(ctx context.Context) bool { return false }
func (f *fakeLLMClient) TestThinkingSupport(ctx context.Context) bool { return false }
func (f *fakeLLMClient) SetThinkingEnabled(enabled bool)              {}
func (f *fakeLLMClient) SetReasoningEffort(effort string)             {}
func (f *fakeLLMClient) SetTopP(topP float64)                         {}
func (f *fakeLLMClient) SetTopK(topK int)                             {}
func (f *fakeLLMClient) SetRepetitionPenalty(penalty float64)         {}
func (f *fakeLLMClient) SetTokenUsage(mode string)                    {}
func (f *fakeLLMClient) SetTemperature(temp float64)                  {}
func (f *fakeLLMClient) SetBodyAdditions(additions map[string]string) {}
func (f *fakeLLMClient) RemoveBodyAddition(key string)                {}
func (f *fakeLLMClient) GetBodyAdditions() map[string]string          { return nil }
func (f *fakeLLMClient) Close() error                                 { return nil }

// recordedRun captures the RunStream result, the event type sequence and the
// concatenated content_chunk payloads of one agent run.
type recordedRun struct {
	result     string
	eventTypes []string
	chunkText  strings.Builder
}

// runWithRenderer runs one input through the agent, recording every event
// while forwarding it to the given renderer.
func runWithRenderer(t *testing.T, ag *agent.Agent, input string, renderer agent.EventRenderer) recordedRun {
	t.Helper()
	var run recordedRun
	cb := func(ev agent.StreamEvent) {
		run.eventTypes = append(run.eventTypes, ev.Type)
		if ev.Type == agent.EventContentChunk {
			run.chunkText.WriteString(ev.Text)
		}
		renderer.Render(ev)
	}
	result, err := ag.RunStream(context.Background(), input, cb)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	run.result = result
	return run
}

// TestOutputFormatEndToEndConsistency runs the same agent on the same input
// with a LineRenderer (text) and a StreamRenderer (JSON-Lines), then asserts:
//
//	(a) both RunStream calls return the same result;
//	(b) every JSON line is a valid JSON object and its type sequence matches
//	    the text-mode event sequence exactly;
//	(c) the concatenated content_chunk payloads are identical in both modes.
func TestOutputFormatEndToEndConsistency(t *testing.T) {
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })

	fake := &fakeLLMClient{chunks: []string{"你好", "世界"}}
	ag := agent.New(fake, nil, store.NewDualStore(boltStore, nil), "")
	ag.SetWorkspacePath(wsDir)
	ag.SetConfig(&config.Config{})
	ag.SetShowLlmContent(true)

	// Run 1: text mode through LineRenderer into a buffer UserIO.
	ep := config.GetEmojiPrefixes(true)
	textIO := &cmdBufferIO{}
	textRun := runWithRenderer(t, ag, "打个招呼",
		agent.NewLineRenderer(textIO, ep, agent.StreamModeREPL))

	// Run 2: JSON-Lines mode through StreamRenderer into a buffer.
	var jsonBuf bytes.Buffer
	jsonRun := runWithRenderer(t, ag, "打个招呼",
		agent.NewStreamRenderer(&jsonBuf))
	jsonOut := jsonBuf.String()

	// (a) Same result from both runs.
	if textRun.result != jsonRun.result {
		t.Errorf("RunStream results differ: text=%q json=%q", textRun.result, jsonRun.result)
	}
	if textRun.result == "" {
		t.Errorf("RunStream returned empty result")
	}

	// (b) Same event type sequence; each JSON line is a valid object.
	if len(textRun.eventTypes) == 0 {
		t.Fatalf("no stream events recorded")
	}
	if len(textRun.eventTypes) != len(jsonRun.eventTypes) {
		t.Fatalf("event count differs: text=%d json=%d", len(textRun.eventTypes), len(jsonRun.eventTypes))
	}
	jsonLines := strings.Split(strings.TrimSuffix(jsonOut, "\n"), "\n")
	if len(jsonLines) != len(textRun.eventTypes) {
		t.Fatalf("JSON line count %d != event count %d", len(jsonLines), len(textRun.eventTypes))
	}
	for i, line := range jsonLines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d is not valid JSON: %q: %v", i, line, err)
		}
		if m["type"] != textRun.eventTypes[i] {
			t.Errorf("event %d: json type %v != text type %q", i, m["type"], textRun.eventTypes[i])
		}
		if textRun.eventTypes[i] != jsonRun.eventTypes[i] {
			t.Errorf("event %d: text mode saw %q, json mode saw %q",
				i, textRun.eventTypes[i], jsonRun.eventTypes[i])
		}
	}

	// (c) Concatenated content_chunk payloads match in both modes.
	if textRun.chunkText.String() != jsonRun.chunkText.String() {
		t.Errorf("content_chunk payloads differ: text=%q json=%q",
			textRun.chunkText.String(), jsonRun.chunkText.String())
	}
	if textRun.chunkText.String() != "你好世界" {
		t.Errorf("concatenated chunks = %q, want %q", textRun.chunkText.String(), "你好世界")
	}
}
