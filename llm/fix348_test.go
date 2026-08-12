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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// FIX-348 tests: SSE cross-line data frame reassembly (an upstream such as
// vLLM can emit tool-call arguments whose JSON string values contain real
// newlines, which makes one SSE data frame span multiple physical lines that
// StreamReader splits) plus non-blocking event sending so a slow consumer
// cannot stall the read loop / raw-chunk logging.
//
// mockSSEServer / sseDone / feat347WS / logSliceStart / logSince are shared
// with feature347_test.go (same package).

// drainToolCallDeltas consumes the stream and returns the accumulated
// ToolCallDelta argument fragments in order.
func drainToolCallDeltas(t *testing.T, client Client) (frags []string, gotError bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventToolCallDelta:
			if ev.ToolCallDelta != nil {
				frags = append(frags, ev.ToolCallDelta.Arguments)
			}
		case StreamEventError:
			gotError = true
		case StreamEventDone:
			return frags, gotError
		}
	}
	return frags, gotError
}

// UC-0001/0002/0006: an SSE data frame whose outer JSON is pretty-printed
// across multiple physical lines (real newlines between JSON tokens, which is
// valid) is split by StreamReader; the reassembly logic must rejoin the
// continuation lines so the event is parsed exactly once with complete tool
// arguments, and no error event is produced.
func TestFIX348_CrossLineDataFrameReassembly(t *testing.T) {
	frame := "data: {\"choices\":[{\n  \"index\":0,\n  \"delta\":{\n    \"tool_calls\":[{\n      \"index\":0,\n      \"id\":\"c1\",\n      \"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"path\\\":\\\"a\\\"}\"}\n    }]\n  }\n}]}\n\n"
	srv := mockSSEServer(t, frame, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	frags, gotError := drainToolCallDeltas(t, client)
	if gotError {
		t.Errorf("UC-0002: got an error event")
	}
	if len(frags) != 1 {
		t.Fatalf("UC-0001: want exactly 1 ToolCallDelta fragment, got %d: %v", len(frags), frags)
	}
	want := "{\"path\":\"a\"}"
	if frags[0] != want {
		t.Errorf("UC-0001: arguments = %q, want %q", frags[0], want)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(frags[0]), &m); err != nil {
		t.Errorf("UC-0001: reassembled arguments not valid JSON: %v", err)
	}
}

// UC-0003: an upstream (e.g. vLLM) can emit a bare continuation line with NO
// "data: " prefix after a complete tool-call arguments frame (the co-flow
// 13:39 log shows exactly this). The bare line must be appended to the
// accumulated tool-call arguments instead of being silently dropped.
// UC-0003 (revised): a bare line with NO "data: " prefix and no unfinished
// frame pending is ignored. The earlier "append bare lines to the current
// tool call's arguments" fallback was removed together with the StreamReader
// root fix: those bare lines were never sent by the upstream — they were the
// surviving tails of large frames whose heads StreamReader had discarded, and
// appending them fed SSE frame trailers into the arguments, which broke the
// downstream streaming parser ("unbalanced closing brace/square bracket").
// Frames that genuinely span lines (real newline inside the JSON) are
// reassembled by the pendingData path instead.
func TestFIX348_BareLineIgnored(t *testing.T) {
	frame1 := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"replace_in_file\",\"arguments\":\"{\\\"intent\\\": \\\"x\\\", \\\"path\\\": \\\"a.go\\\"}\"}}]}}]}\n\n"
	// Bare line (no "data: " prefix) arriving between complete frames.
	bareLine := "\\\\n\\\\tby, bm, bd := b.Date()" + "\n"
	finish := "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"
	srv := mockSSEServer(t, frame1, bareLine, finish, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	frags, gotError := drainToolCallDeltas(t, client)
	if gotError {
		t.Errorf("bare line produced an error event")
	}
	joined := strings.Join(frags, "")
	if strings.Contains(joined, "by, bm, bd") {
		t.Errorf("bare line leaked into arguments; joined = %q", joined)
	}
	if !strings.Contains(joined, "\"path\": \"a.go\"") {
		t.Errorf("complete frame arguments lost; joined = %q", joined)
	}
}

// UC-0004: single-line frames behave exactly as before (regression).
func TestFIX348_SingleLineRegression(t *testing.T) {
	srv := mockSSEServer(t, sseContentChunk, sseToolChunk, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	var contentEvents, deltaEvents int
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventContent:
			contentEvents++
		case StreamEventToolCallDelta:
			deltaEvents++
		case StreamEventDone:
			if contentEvents != 1 || deltaEvents != 1 {
				t.Errorf("UC-0004: content=%d (want 1) delta=%d (want 1)", contentEvents, deltaEvents)
			}
			return
		}
	}
}

// UC-0005/0008: a malformed "data: " line that never completes (followed by
// an empty SSE boundary line) is dropped without producing an error event,
// and subsequent frames are unaffected.
func TestFIX348_BadDataDropped(t *testing.T) {
	bad := "data: {bad json\n\n"
	srv := mockSSEServer(t, bad, sseContentChunk, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	var contentEvents int
	gotError := false
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventContent:
			contentEvents++
		case StreamEventError:
			gotError = true
		case StreamEventDone:
			if gotError {
				t.Errorf("UC-0005: malformed data produced an error event")
			}
			if contentEvents != 1 {
				t.Errorf("UC-0008: content after malformed line = %d, want 1", contentEvents)
			}
			return
		}
	}
}

// UC-0010/0011: with a slow consumer the non-blocking sendEvent must not lose
// events and must preserve their order (FIFO).
func TestFIX348_EventsNotLostUnderSlowConsumer(t *testing.T) {
	var frames []string
	for i := 0; i < 30; i++ {
		frames = append(frames, fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"c%d\"}}]}\n\n", i))
	}
	frames = append(frames, sseDone)
	srv := mockSSEServer(t, frames...)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	var mu sync.Mutex
	var received []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx := context.Background()
		eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
		if err != nil {
			return
		}
		for ev := range eventCh {
			if ev.Type == StreamEventContent {
				mu.Lock()
				received = append(received, ev.Content)
				mu.Unlock()
				time.Sleep(5 * time.Millisecond) // slow consumer
			}
			if ev.Type == StreamEventDone {
				return
			}
		}
	}()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 30 {
		t.Errorf("UC-0010: events lost: got %d, want 30", len(received))
	}
	for i := 0; i < len(received) && i < 30; i++ {
		if received[i] != fmt.Sprintf("c%d", i) {
			t.Errorf("UC-0011: order wrong at %d: got %q", i, received[i])
		}
	}
}

// UC-0009: even when the consumer is slow enough to fill the event channel,
// the raw-chunk log lines must all be written (the read loop is never stalled
// by event delivery) and no frame is lost from the log.
func TestFIX348_LogCompleteUnderSlowConsumer(t *testing.T) {
	// This test asserts on raw-chunk log lines, so force debug level
	// (other tests in the package may have switched it to info).
	log.SetLevel(log.LogLevelDebug)

	var frames []string
	for i := 0; i < 20; i++ {
		frames = append(frames, fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"c%d\"}}]}\n\n", i))
	}
	frames = append(frames, sseDone)
	srv := mockSSEServer(t, frames...)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx := context.Background()
		eventCh, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
		if err != nil {
			close(done)
			return
		}
		for ev := range eventCh {
			if ev.Type == StreamEventContent {
				time.Sleep(5 * time.Millisecond) // slow consumer
			}
			if ev.Type == StreamEventDone {
				return
			}
		}
	}()

	path, start := logSliceStart(t)
	<-done
	got := logSince(t, path, start)

	wantLines := 21 // 20 content frames + [DONE]
	if n := strings.Count(got, "LLM ChatStream raw chunk:"); n != wantLines {
		t.Errorf("UC-0009: raw chunk lines = %d, want %d", n, wantLines)
	}
}

// Plan B (interaction-log decoupling): a bare continuation line (no "data: "
// prefix) inside a tool-call arguments stream must be appended to the
// [RESP][tool_calls interaction log in real time, even though it is malformed
// and would fail JSON parsing — nothing the LLM returns should be swallowed.
func TestFIX348_BareLineLoggedToInteractionLog(t *testing.T) {
	log.SetLevel(log.LogLevelDebug)
	if err := log.InitLLMInteractionLog(feat347WS); err != nil {
		t.Fatalf("cannot init interaction log: %v", err)
	}
	log.SetLLMInteractionEnabled(true)

	date := time.Now().Format("2006-01-02")
	logPath := feat347WS.LLMInteractionLogFilePath(date)
	st, _ := os.Stat(logPath)
	start := int64(0)
	if st != nil {
		start = st.Size()
	}

	nameFrame := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"write_to_file\"}}]}}]}\n\n"
	argsFrame := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"intent\\\":\\\"x\\\",\\\"content\\\":\\\"\"}}]}}]}\n\n"
	bareLine := "平台\\\\n\\\\n**现状与问题**" + "\n"
	finish := "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"
	srv := mockSSEServer(t, nameFrame, argsFrame, bareLine, finish, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	drainToolCallDeltas(t, client)

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("cannot open interaction log: %v", err)
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		t.Fatalf("seek interaction log: %v", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read interaction log: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, "RESP][tool_calls") {
		t.Errorf("planB: RESP][tool_calls header missing\n---log---\n%s", got)
	}
	if !strings.Contains(got, "平台\\\\n\\\\n**现状与问题**") {
		t.Errorf("planB: bare line not logged to interaction log\n---log---\n%s", got)
	}
}

// --- Root-cause fix: StreamReader must not discard partial lines ---

// dribbleReader returns at most max bytes per Read, simulating socket-level
// segmentation of a large SSE frame across multiple reads.
type dribbleReader struct {
	data []byte
	max  int
}

func (d *dribbleReader) Read(p []byte) (int, error) {
	if len(d.data) == 0 {
		return 0, io.EOF
	}
	n := d.max
	if n > len(d.data) {
		n = len(d.data)
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, d.data[:n])
	d.data = d.data[n:]
	return n, nil
}

// Root fix regression: a single SSE line much larger than one socket read
// (e.g. vLLM emitting a whole write_to_file arguments payload as one ~7KB
// frame) must be reassembled byte-for-byte regardless of how the underlying
// reader segments it. The pre-fix StreamReader discarded every 4096-byte
// segment except the last, losing the frame head mid-UTF-8-character.
func TestFIX348_StreamReaderLargeFrameFragmented(t *testing.T) {
	bigLine := "data: " + strings.Repeat("汉", 2400) // ~7.2KB, no newline inside
	stream := bigLine + "\n" + "data: tail\n"
	for _, seg := range []int{1, 1000, 4096, 65536} {
		sr := NewStreamReader(&dribbleReader{data: []byte(stream), max: seg})
		line1, err := sr.Read()
		if err != nil {
			t.Fatalf("seg=%d: first Read error: %v", seg, err)
		}
		if string(line1) != bigLine {
			t.Errorf("seg=%d: first line len=%d, want %d (head lost)", seg, len(line1), len(bigLine))
		}
		line2, err := sr.Read()
		if err != nil {
			t.Fatalf("seg=%d: second Read error: %v", seg, err)
		}
		if string(line2) != "data: tail" {
			t.Errorf("seg=%d: second line = %q", seg, line2)
		}
		if _, err := sr.Read(); err != io.EOF {
			t.Errorf("seg=%d: third Read err = %v, want io.EOF", seg, err)
		}
	}
}

// End-to-end: a >4KB tool-call arguments payload delivered as one single-line
// SSE frame is accumulated complete and unmodified.
func TestFIX348_LargeSingleFrameArguments(t *testing.T) {
	args := `{"path":"f.md","content":"` + strings.Repeat("多行内容。", 1000) + `"}`
	argsJSON, _ := json.Marshal(args) // proper JSON string escaping
	frame := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"write_to_file\",\"arguments\":" + string(argsJSON) + "}}]}}]}\n\n"
	srv := mockSSEServer(t, frame, sseDone)
	defer srv.Close()
	client := NewClient(srv.URL, "sk", "m", 0, -1)

	frags, gotError := drainToolCallDeltas(t, client)
	if gotError {
		t.Fatalf("got an error event")
	}
	joined := strings.Join(frags, "")
	if joined != args {
		t.Fatalf("arguments len=%d, want %d", len(joined), len(args))
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(joined), &m); err != nil {
		t.Errorf("accumulated arguments not valid JSON: %v", err)
	}
}
