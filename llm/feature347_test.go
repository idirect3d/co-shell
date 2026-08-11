// Author: L.Shuang
// Created: 2026-08-11
// Last Modified: 2026-08-11
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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/workspace"
)

// FEATURE-347: verify that at log level=debug the original (raw) LLM chunks are
// written to the main log BEFORE any parsing, and that they are suppressed at
// non-debug levels. The unit tests exercise mock HTTP SSE / JSON servers and
// assert against the real co-shell log file.

var feat347WS *workspace.Workspace

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "co-shell-feat347-*")
	if err != nil {
		panic(fmt.Sprintf("cannot create temp workspace: %v", err))
	}
	feat347WS, err = workspace.New(root)
	if err != nil {
		panic(fmt.Sprintf("cannot init temp workspace: %v", err))
	}
	if err := log.Init(true, feat347WS); err != nil {
		panic(fmt.Sprintf("cannot init logger: %v", err))
	}
	log.SetLevel(log.LogLevelDebug)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

// logSliceStart records the current end-of-file offset of the main log file so
// the test can later assert only on the newly appended portion.
func logSliceStart(t *testing.T) (string, int64) {
	t.Helper()
	path := feat347WS.LogFilePath(time.Now().Format("2006-01-02"))
	st, err := os.Stat(path)
	if err != nil {
		return path, 0
	}
	return path, st.Size()
}

// logSince reads the main log file from the given offset to the end.
func logSince(t *testing.T, path string, start int64) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log file %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		t.Fatalf("seek log file: %v", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	return string(b)
}

// mockSSEServer streams the given raw SSE chunks and closes the connection.
func mockSSEServer(t *testing.T, chunks ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fl, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for _, c := range chunks {
			fmt.Fprint(w, c)
			fl.Flush()
		}
	}))
}

// drainChatStream consumes the ChatStream event channel until Done or close.
func drainChatStream(t *testing.T, client Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	for ev := range eventCh {
		if ev.Type == StreamEventDone {
			return
		}
	}
}

const sseContentChunk = `data: {"choices":[{"delta":{"content":"你好"}}]}` + "\n\n"
const sseToolChunk = `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read_file","arguments":"{\"path\":\"a\"}"}}]}}]}` + "\n\n"
const sseDone = "data: [DONE]\n\n"
const badSSEChunk = "data: {bad json\n\n"

func TestFEATURE347_StreamRawChunkLogging(t *testing.T) {
	tests := []struct {
		name            string
		level           log.LogLevel
		chunks          []string
		wantContains    []string
		wantNotContains []string
		wantRawCount    int // expected number of "LLM ChatStream raw chunk:" lines
	}{
		{
			name:         "debug_emits_raw_data_chunk_verbatim",
			level:        log.LogLevelDebug,
			chunks:       []string{sseContentChunk, sseDone},
			wantContains: []string{"LLM ChatStream raw chunk: data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}"},
			wantRawCount: 2,
		},
		{
			name:            "info_suppresses_raw_chunk",
			level:           log.LogLevelInfo,
			chunks:          []string{sseContentChunk, sseDone},
			wantNotContains: []string{"LLM ChatStream raw chunk:"},
			wantRawCount:    0,
		},
		{
			name:         "done_signal_logged_with_data_prefix",
			level:        log.LogLevelDebug,
			chunks:       []string{sseContentChunk, sseDone},
			wantContains: []string{"LLM ChatStream raw chunk: data: [DONE]"},
			wantRawCount: 2,
		},
		{
			name:         "empty_separator_lines_skipped",
			level:        log.LogLevelDebug,
			chunks:       []string{sseContentChunk, "\n\n", sseDone},
			wantContains: []string{"LLM ChatStream raw chunk: data: {\"choices\""},
			wantRawCount: 2, // content + [DONE]; the blank separator is skipped
		},
		{
			name:         "parse_failing_line_still_logged",
			level:        log.LogLevelDebug,
			chunks:       []string{badSSEChunk, sseContentChunk, sseDone},
			wantContains: []string{"LLM ChatStream raw chunk: data: {bad json"},
			wantRawCount: 3,
		},
		{
			name:   "mixed_content_and_tool_calls_preserved_in_order",
			level:  log.LogLevelDebug,
			chunks: []string{sseContentChunk, sseToolChunk, sseDone},
			wantContains: []string{
				"LLM ChatStream raw chunk: data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}",
				"LLM ChatStream raw chunk: data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0",
				"LLM ChatStream raw chunk: data: [DONE]",
			},
			wantRawCount: 3,
		},
		{
			name:         "raw_lines_keep_data_prefix_no_trailing_newline",
			level:        log.LogLevelDebug,
			chunks:       []string{sseContentChunk, sseDone},
			wantContains: []string{"LLM ChatStream raw chunk: data: "},
			wantRawCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := mockSSEServer(t, tt.chunks...)
			defer srv.Close()
			client := NewClient(srv.URL, "sk-SECRET123", "m", 0, -1)

			log.SetLevel(tt.level)
			path, start := logSliceStart(t)
			drainChatStream(t, client)
			got := logSince(t, path, start)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q\n---log---\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("log should not contain %q\n---log---\n%s", notWant, got)
				}
			}
			if gotCount := strings.Count(got, "LLM ChatStream raw chunk:"); gotCount != tt.wantRawCount {
				t.Errorf("raw chunk lines = %d, want %d\n---log---\n%s", gotCount, tt.wantRawCount, got)
			}
			// UC-0006: raw lines must keep the "data: " prefix and carry no
			// embedded newline (StreamReader strips the trailing \n).
			if strings.Contains(got, "raw chunk: \n") || strings.Contains(got, "raw chunk:\n") {
				t.Errorf("raw chunk line contains embedded newline\n---log---\n%s", got)
			}
			// UC-0013: no bare content echo (legacy log.Raw) interleaved between
			// raw chunk lines — the content only appears inside the data line.
			if strings.Contains(got, "你好[") || strings.Contains(got, "\n你好\n") {
				t.Errorf("log contains bare content echo between raw chunk lines\n---log---\n%s", got)
			}
		})
	}
}

func TestFEATURE347_StreamRawChunkFrameOrder(t *testing.T) {
	srv := mockSSEServer(t, sseContentChunk, sseToolChunk, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk-SECRET123", "m", 0, -1)

	log.SetLevel(log.LogLevelDebug)
	path, start := logSliceStart(t)
	drainChatStream(t, client)
	got := logSince(t, path, start)

	contentIdx := strings.Index(got, `raw chunk: data: {"choices":[{"delta":{"content"`)
	toolIdx := strings.Index(got, `raw chunk: data: {"choices":[{"delta":{"tool_calls"`)
	doneIdx := strings.Index(got, "raw chunk: data: [DONE]")
	if contentIdx < 0 || toolIdx < 0 || doneIdx < 0 {
		t.Fatalf("missing raw chunk lines (content=%d tool=%d done=%d)\n---log---\n%s", contentIdx, toolIdx, doneIdx, got)
	}
	if !(contentIdx < toolIdx && toolIdx < doneIdx) {
		t.Errorf("raw chunk frame order wrong: content=%d tool=%d done=%d", contentIdx, toolIdx, doneIdx)
	}
}

func TestFEATURE347_StreamRawChunkNoAPIKeyLeak(t *testing.T) {
	srv := mockSSEServer(t, sseContentChunk, sseToolChunk, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk-SECRET123", "m", 0, -1)

	log.SetLevel(log.LogLevelDebug)
	path, start := logSliceStart(t)
	drainChatStream(t, client)
	got := logSince(t, path, start)

	if strings.Contains(got, "sk-SECRET123") {
		t.Errorf("raw chunk logging leaked the API key\n---log---\n%s", got)
	}
}

func TestFEATURE347_ChatRawResponseLogging(t *testing.T) {
	body := `{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	client := NewClient(srv.URL, "sk-SECRET123", "m", 0, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tests := []struct {
		name            string
		level           log.LogLevel
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "debug_emits_raw_response_body",
			level:        log.LogLevelDebug,
			wantContains: []string{"LLM Chat raw response: " + body},
		},
		{
			name:            "info_suppresses_raw_response_body",
			level:           log.LogLevelInfo,
			wantNotContains: []string{"LLM Chat raw response:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.SetLevel(tt.level)
			path, start := logSliceStart(t)
			if _, err := client.Chat(ctx, []Message{{Role: "user", Content: "hi"}}, nil); err != nil {
				t.Fatalf("Chat failed: %v", err)
			}
			got := logSince(t, path, start)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q\n---log---\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("log should not contain %q\n---log---\n%s", notWant, got)
				}
			}
		})
	}
}

func TestFEATURE347_SetLevelRuntimeSwitch(t *testing.T) {
	srv := mockSSEServer(t, sseContentChunk, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk-SECRET123", "m", 0, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	drain := func() {
		eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
		if err != nil {
			t.Fatalf("ChatStream failed: %v", err)
		}
		for ev := range eventCh {
			if ev.Type == StreamEventDone {
				return
			}
		}
	}

	// debug: raw chunks emitted
	log.SetLevel(log.LogLevelDebug)
	path, start := logSliceStart(t)
	drain()
	first := logSince(t, path, start)
	if !strings.Contains(first, "LLM ChatStream raw chunk:") {
		t.Errorf("debug level should emit raw chunks\n---log---\n%s", first)
	}

	// info: raw chunks suppressed
	log.SetLevel(log.LogLevelInfo)
	_, start2 := logSliceStart(t)
	drain()
	second := logSince(t, path, start2)
	if strings.Contains(second, "LLM ChatStream raw chunk:") {
		t.Errorf("info level should suppress raw chunks\n---log---\n%s", second)
	}
}
