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

package llm

import (
	"context"
	"testing"
	"time"
)

// FIX-353: mlx-vlm (and some other OpenAI-compatible servers) emit a complete
// tool call — id, name and full arguments — in a single delta chunk, instead
// of the OpenAI/vLLM style of streaming id+name first and argument fragments
// in later chunks. The accumulator must keep the arguments carried by the
// very first chunk; previously they were silently dropped, producing a final
// ToolCall with empty arguments ("arguments is empty").
//
// mockSSEServer / sseDone are shared with feature347_test.go (same package).

// drainToolCalls consumes the stream and returns the final accumulated
// tool calls in order.
func drainToolCalls(t *testing.T, client Client) []ToolCall {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	var calls []ToolCall
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventToolCall:
			if ev.ToolCall != nil {
				calls = append(calls, *ev.ToolCall)
			}
		case StreamEventDone:
			return calls
		}
	}
	return calls
}

// Single-chunk complete tool call (mlx-vlm style): the final accumulated
// ToolCall must carry the arguments from the first (and only) chunk.
func TestFIX353_SingleChunkToolCallArgumentsKept(t *testing.T) {
	args := `{"result": "done", "session_title": "问候"}`
	frame := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"attempt_completion","arguments":"{\"result\": \"done\", \"session_title\": \"问候\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n"
	srv := mockSSEServer(t, frame, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	calls := drainToolCalls(t, client)
	if len(calls) != 1 {
		t.Fatalf("want 1 tool call, got %d: %+v", len(calls), calls)
	}
	tc := calls[0]
	if tc.ID != "call_1" || tc.Name != "attempt_completion" {
		t.Errorf("tool call id/name = %q/%q, want call_1/attempt_completion", tc.ID, tc.Name)
	}
	if tc.Arguments != args {
		t.Errorf("arguments = %q, want %q", tc.Arguments, args)
	}
}

// Regression for the classic OpenAI/vLLM style: id+name first (no
// arguments), argument fragments appended by later chunks.
func TestFIX353_FragmentedToolCallArgumentsStillAccumulate(t *testing.T) {
	head := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_file","arguments":""}}]}}]}` + "\n\n"
	frag1 := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]}}]}` + "\n\n"
	frag2 := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.go\"}"}}]}}]}` + "\n\n"
	srv := mockSSEServer(t, head, frag1, frag2, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	calls := drainToolCalls(t, client)
	if len(calls) != 1 {
		t.Fatalf("want 1 tool call, got %d: %+v", len(calls), calls)
	}
	want := `{"path":"a.go"}`
	if calls[0].Arguments != want {
		t.Errorf("arguments = %q, want %q", calls[0].Arguments, want)
	}
}
