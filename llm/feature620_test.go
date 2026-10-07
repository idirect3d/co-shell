package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// FEATURE-620 P0 acceptance tests (use-case/FEATURE-620/FEATURE-620-UC-0001.md).
//
// Root cause under test: vLLM renamed the thinking response field from
// "reasoning_content" to "reasoning" and warns that a client still reading the
// old name silently gets an empty string. Before this change co-shell read only
// "reasoning_content", so thinking was dropped and the UI showed no thinking
// block.

// P0-001..P0-006: field selection and the auto union fallback.
func TestFEATURE620PickReasoning(t *testing.T) {
	tests := []struct {
		name             string
		field            string
		reasoningContent string
		reasoning        string
		want             string
	}{
		{"P0-001 auto reads reasoning only", ReasoningFieldAuto, "", "let me think", "let me think"},
		{"P0-002 auto keeps legacy field", ReasoningFieldAuto, "let me think", "", "let me think"},
		{"P0-003 auto prefers legacy when both set", ReasoningFieldAuto, "A", "B", "A"},
		{"P0-004 explicit reasoning wins", ReasoningFieldReasoning, "A", "B", "B"},
		{"P0-005 explicit content wins", ReasoningFieldContent, "A", "B", "A"},
		{"P0-006 none disables extraction", ReasoningFieldNone, "A", "B", ""},
		{"inline never reads a dedicated field", ReasoningFieldInline, "A", "B", ""},
		{"unknown value degrades to auto", "bogus", "A", "B", "A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PickReasoning(tc.field, tc.reasoningContent, tc.reasoning); got != tc.want {
				t.Errorf("PickReasoning(%q, %q, %q) = %q, want %q",
					tc.field, tc.reasoningContent, tc.reasoning, got, tc.want)
			}
		})
	}
}

// P0-010..P0-012: the JSON key used to echo thinking back in a request.
func TestFEATURE620ReasoningEchoField(t *testing.T) {
	tests := []struct {
		field string
		want  string
	}{
		{ReasoningFieldAuto, ReasoningFieldContent},
		{ReasoningFieldContent, ReasoningFieldContent},
		{ReasoningFieldReasoning, ReasoningFieldReasoning},
		{ReasoningFieldInline, ReasoningFieldContent},
		{ReasoningFieldNone, ReasoningFieldContent},
		{"bogus", ReasoningFieldContent},
	}
	for _, tc := range tests {
		if got := ReasoningEchoField(tc.field); got != tc.want {
			t.Errorf("ReasoningEchoField(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

// chatMockServer answers every request with the same canned JSON body.
func chatMockServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("write canned response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sseMockServer replies with an SSE stream built from the given data payloads.
func sseMockServer(t *testing.T, payloads ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, p := range payloads {
			if _, err := io.WriteString(w, "data: "+p+"\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, endpoint, reasoningField string) Client {
	t.Helper()
	c := NewClient(endpoint, "test-key", "test-model", 0, 0, 15)
	c.SetReasoningField(reasoningField)
	return c
}

// P0-001..P0-006 end to end through Chat: the configured field decides which
// response field the thinking is read from.
func TestFEATURE620ChatReadsConfiguredReasoningField(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		field     string
		wantThink string
	}{
		{
			name:      "P0-001 auto picks up the renamed field (regression that was broken)",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning":"let me think"}}]}`,
			field:     ReasoningFieldAuto,
			wantThink: "let me think",
		},
		{
			name:      "P0-002 auto still reads the legacy field",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning_content":"let me think"}}]}`,
			field:     ReasoningFieldAuto,
			wantThink: "let me think",
		},
		{
			name:      "P0-003 both present: legacy field wins, no concatenation",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning_content":"A","reasoning":"B"}}]}`,
			field:     ReasoningFieldAuto,
			wantThink: "A",
		},
		{
			name:      "P0-004 explicit reasoning ignores the legacy value",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning_content":"A","reasoning":"B"}}]}`,
			field:     ReasoningFieldReasoning,
			wantThink: "B",
		},
		{
			name:      "P0-005 explicit content ignores the new value",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning_content":"A","reasoning":"B"}}]}`,
			field:     ReasoningFieldContent,
			wantThink: "A",
		},
		{
			name:      "P0-006 none yields no thinking",
			response:  `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning_content":"A","reasoning":"B"}}]}`,
			field:     ReasoningFieldNone,
			wantThink: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := chatMockServer(t, tc.response)
			client := newTestClient(t, srv.URL, tc.field)
			defer client.Close()

			resp, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "q"}}, nil)
			if err != nil {
				t.Fatalf("Chat: %v", err)
			}
			if resp.ReasoningContent != tc.wantThink {
				t.Errorf("ReasoningContent = %q, want %q", resp.ReasoningContent, tc.wantThink)
			}
			if resp.Content != "hi" {
				t.Errorf("Content = %q, want %q", resp.Content, "hi")
			}
		})
	}
}

// reasoningEvents runs one streamed exchange and returns the content of every
// StreamEventReasoning in order.
func reasoningEvents(t *testing.T, endpoint, reasoningField string, payloads ...string) []string {
	t.Helper()
	srv := sseMockServer(t, payloads...)
	client := newTestClient(t, srv.URL, reasoningField)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ch, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var got []string
	for ev := range ch {
		if ev.Type == StreamEventReasoning {
			got = append(got, ev.Content)
		}
	}
	return got
}

// P0-007: the streaming path — the one the UI watches — must surface thinking
// sent under the new field name. Before the fix this returned nothing.
func TestFEATURE620ChatStreamEmitsRenamedReasoningField(t *testing.T) {
	got := reasoningEvents(t, "", ReasoningFieldAuto,
		`{"choices":[{"delta":{"reasoning":"step1"}}]}`,
		`{"choices":[{"delta":{"reasoning":"step2"}}]}`,
		`{"choices":[{"delta":{"content":"ans"}}]}`,
	)
	want := []string{"step1", "step2"}
	if len(got) != len(want) {
		t.Fatalf("reasoning events = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reasoning event %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// P0-008: the legacy field keeps working (regression guard).
func TestFEATURE620ChatStreamKeepsLegacyReasoningField(t *testing.T) {
	got := reasoningEvents(t, "", ReasoningFieldAuto,
		`{"choices":[{"delta":{"reasoning_content":"step1"}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"step2"}}]}`,
		`{"choices":[{"delta":{"content":"ans"}}]}`,
	)
	want := []string{"step1", "step2"}
	if len(got) != len(want) {
		t.Fatalf("reasoning events = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reasoning event %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// P0-009: an explicit field selection ignores the other field in stream chunks.
func TestFEATURE620ChatStreamHonoursExplicitReasoningField(t *testing.T) {
	got := reasoningEvents(t, "", ReasoningFieldReasoning,
		`{"choices":[{"delta":{"reasoning_content":"X1"}}]}`,
		`{"choices":[{"delta":{"reasoning":"Y1"}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"X2"}}]}`,
		`{"choices":[{"delta":{"reasoning":"Y2"}}]}`,
	)
	want := []string{"Y1", "Y2"}
	if len(got) != len(want) {
		t.Fatalf("reasoning events = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reasoning event %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// capturedMessageKeys runs one request against a recording mock and returns the
// JSON keys of the first assistant message in the request body.
func capturedMessageKeys(t *testing.T, reasoningField string) map[string]bool {
	t.Helper()
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, reasoningField)
	defer client.Close()

	msgs := []Message{
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "a", ReasoningContent: "because"},
	}
	if _, err := client.Chat(context.Background(), msgs, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	rawMsgs, ok := captured["messages"].([]interface{})
	if !ok || len(rawMsgs) < 2 {
		t.Fatalf("captured request has no messages: %#v", captured)
	}
	assistant, ok := rawMsgs[1].(map[string]interface{})
	if !ok {
		t.Fatalf("second message is not an object: %#v", rawMsgs[1])
	}
	keys := make(map[string]bool, len(assistant))
	for k := range assistant {
		keys[k] = true
	}
	return keys
}

// P0-010: an explicit "reasoning" switches the echo key to the new name.
func TestFEATURE620RequestEchoesReasoningUnderNewKey(t *testing.T) {
	keys := capturedMessageKeys(t, ReasoningFieldReasoning)
	if !keys["reasoning"] {
		t.Errorf("request is missing the %q key: %v", "reasoning", keys)
	}
	if keys["reasoning_content"] {
		t.Errorf("request must not carry %q when the field is %q: %v",
			"reasoning_content", ReasoningFieldReasoning, keys)
	}
}

// P0-011: without configuration the request is byte-for-byte what it was before.
func TestFEATURE620RequestKeepsLegacyEchoKeyByDefault(t *testing.T) {
	keys := capturedMessageKeys(t, ReasoningFieldAuto)
	if !keys["reasoning_content"] {
		t.Errorf("request is missing the legacy %q key: %v", "reasoning_content", keys)
	}
	if keys["reasoning"] {
		t.Errorf("request must not carry %q when the field is auto: %v", "reasoning", keys)
	}
}

// P0-012: "none" sends no thinking back at all.
func TestFEATURE620RequestOmitsReasoningWhenDisabled(t *testing.T) {
	keys := capturedMessageKeys(t, ReasoningFieldNone)
	if keys["reasoning"] || keys["reasoning_content"] {
		t.Errorf("request must carry no thinking key when disabled: %v", keys)
	}
}

// P0-013: integration check against a real OpenAI-compatible endpoint.
//
// Skipped in -short mode and unless CO_SHELL_TEST_LLM_ENDPOINT is set, so the
// default `go test ./...` run never touches the network. Example:
//
//	CO_SHELL_TEST_LLM_ENDPOINT=http://192.168.3.39:8899/v1 \
//	CO_SHELL_TEST_LLM_MODEL=deepseek-v4-flash-vision-exp \
//	go test ./llm/ -run FEATURE620RealEndpoint -v
//
// This is the end-to-end proof for the reported symptom: a vLLM endpoint that
// reports thinking under "reasoning" must now produce reasoning events.
func TestFEATURE620RealEndpointSurfacesReasoning(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: skipped in -short mode")
	}
	endpoint := os.Getenv("CO_SHELL_TEST_LLM_ENDPOINT")
	if endpoint == "" {
		t.Skip("set CO_SHELL_TEST_LLM_ENDPOINT to run this integration test")
	}
	model := os.Getenv("CO_SHELL_TEST_LLM_MODEL")
	if model == "" {
		t.Skip("set CO_SHELL_TEST_LLM_MODEL to run this integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// No explicit field: the auto union fallback must cover both namings.
	client := NewClient(endpoint, "", model, 0, 256, 60)
	defer client.Close()
	client.SetReasoningField(ReasoningFieldAuto)

	ch, err := client.ChatStream(ctx, []Message{
		{Role: "user", Content: "Count from 1 to 5, explaining your reasoning briefly."},
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var reasoningLen, contentLen int
	var samples []string
	for ev := range ch {
		switch ev.Type {
		case StreamEventReasoning:
			reasoningLen += len(ev.Content)
			if len(samples) < 3 && ev.Content != "" {
				samples = append(samples, ev.Content)
			}
		case StreamEventContent:
			contentLen += len(ev.Content)
		}
	}

	t.Logf("endpoint=%s model=%s reasoning_bytes=%d content_bytes=%d samples=%q",
		endpoint, model, reasoningLen, contentLen, samples)

	if reasoningLen == 0 {
		t.Fatalf("endpoint %s reported no thinking through the client (content_bytes=%d)", endpoint, contentLen)
	}
}
