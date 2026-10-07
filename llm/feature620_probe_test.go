package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// FEATURE-620 P1 acceptance tests for the capability probe
// (use-case/FEATURE-620/FEATURE-620-UC-0002.md).

// probeRecorder captures the bodies the probe sent.
type probeRecorder struct {
	mu     sync.Mutex
	bodies []map[string]interface{}
}

func (r *probeRecorder) add(body map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodies = append(r.bodies, body)
}

func (r *probeRecorder) requests() []map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]interface{}(nil), r.bodies...)
}

// probeServer serves probe traffic and records every request body.
func probeServer(t *testing.T, respond func(body map[string]interface{}, w http.ResponseWriter)) (*httptest.Server, *probeRecorder) {
	t.Helper()
	rec := &probeRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read probe body: %v", err)
			return
		}
		var body map[string]interface{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode probe body: %v", err)
			return
		}
		rec.add(body)
		respond(body, w)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// writeSSEDeltas writes the given raw data payloads as an SSE stream.
func writeSSEDeltas(w http.ResponseWriter, deltas ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	for _, d := range deltas {
		if _, err := io.WriteString(w, "data: "+d+"\n\n"); err != nil {
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
}

// writeJSON writes a non-streaming chat response.
func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func isStreaming(body map[string]interface{}) bool {
	stream, _ := body["stream"].(bool)
	return stream
}

// P1-003 / P1-004 / P1-005: the probe names the field that actually carried
// thinking, and reports "none" when nothing did.
func TestFEATURE620ProbeReasoningFieldName(t *testing.T) {
	tests := []struct {
		name    string
		delta   string
		want    string
		wantAny bool
	}{
		{"P1-003 renamed vLLM field", `{"choices":[{"delta":{"reasoning":"think"}}]}`, ReasoningFieldReasoning, true},
		{"P1-004 legacy field", `{"choices":[{"delta":{"reasoning_content":"think"}}]}`, ReasoningFieldContent, true},
		{"P1-005 neither field", `{"choices":[{"delta":{"content":"answer"}}]}`, ReasoningFieldNone, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
				if isStreaming(body) {
					writeSSEDeltas(w, tc.delta)
					return
				}
				writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
			})

			profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeLight)
			if profile.ReasoningField != tc.want {
				t.Errorf("ReasoningField = %q, want %q", profile.ReasoningField, tc.want)
			}
			wantDefault := reasoningDefaultOff
			if tc.wantAny {
				wantDefault = reasoningDefaultOn
			}
			if profile.ReasoningDefault != wantDefault {
				t.Errorf("ReasoningDefault = %q, want %q", profile.ReasoningDefault, wantDefault)
			}
		})
	}
}

// P1-006 / P1-007: the default switch is read with NO thinking parameter sent.
func TestFEATURE620ProbeReasoningDefaultSwitch(t *testing.T) {
	t.Run("P1-006 thinking on by default", func(t *testing.T) {
		srv, rec := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
			writeSSEDeltas(w, `{"choices":[{"delta":{"reasoning":"think"}}]}`)
		})

		profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeLight)
		if profile.ReasoningDefault != reasoningDefaultOn {
			t.Errorf("ReasoningDefault = %q, want %q", profile.ReasoningDefault, reasoningDefaultOn)
		}
		// The switch must be observed without pushing any thinking parameter.
		for i, body := range rec.requests() {
			if _, ok := body["thinking"]; ok {
				t.Errorf("request %d sent a thinking parameter: %v", i, body)
			}
			if _, ok := body["extra_body"]; ok {
				t.Errorf("request %d sent extra_body: %v", i, body)
			}
		}
	})

	t.Run("P1-007 thinking off by default", func(t *testing.T) {
		srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
			writeSSEDeltas(w, `{"choices":[{"delta":{"content":"answer"}}]}`)
		})

		profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeLight)
		if profile.ReasoningDefault != reasoningDefaultOff {
			t.Errorf("ReasoningDefault = %q, want %q", profile.ReasoningDefault, reasoningDefaultOff)
		}
	})
}

// P1-008 / P1-009: with the provider name useless (openai-compatible), the
// probe walks the request-side protocols in a fixed order and stops at the
// first one that works.
func TestFEATURE620ProbeWalksThinkingProtocols(t *testing.T) {
	srv, rec := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
		if !isStreaming(body) {
			writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
			return
		}
		// Only the qwen protocol (extra_body.chat_template_kwargs) turns
		// thinking on for this fake engine.
		if _, ok := body["extra_body"]; ok {
			writeSSEDeltas(w, `{"choices":[{"delta":{"reasoning":"think"}}]}`)
			return
		}
		writeSSEDeltas(w, `{"choices":[{"delta":{"content":"answer"}}]}`)
	})

	profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeFull)

	if profile.ThinkingProtocol != "qwen" {
		t.Errorf("ThinkingProtocol = %q, want %q (provider name must not decide this)",
			profile.ThinkingProtocol, "qwen")
	}
	if profile.ReasoningField != ReasoningFieldReasoning {
		t.Errorf("ReasoningField = %q, want %q", profile.ReasoningField, ReasoningFieldReasoning)
	}

	// Fixed order + stop at the first hit: baseline request, then deepseek,
	// then qwen — and nothing after the hit.
	streams := 0
	var sawDeepseek, sawQwenAfterDeepseek bool
	for _, body := range rec.requests() {
		if !isStreaming(body) {
			continue
		}
		streams++
		switch streams {
		case 2:
			if _, ok := body["thinking"]; ok {
				sawDeepseek = true
			}
		case 3:
			if _, ok := body["extra_body"]; ok && sawDeepseek {
				sawQwenAfterDeepseek = true
			}
		}
	}
	if !sawDeepseek {
		t.Error("the deepseek protocol was not tried first")
	}
	if !sawQwenAfterDeepseek {
		t.Error("the qwen protocol was not tried after deepseek")
	}
	if streams != 3 {
		t.Errorf("stream requests = %d, want 3 (baseline + deepseek + qwen, then stop)", streams)
	}
}

// P1-011: ProbeLevel decides the request budget.
func TestFEATURE620ProbeLevelRequestBudget(t *testing.T) {
	countStreams := func(level ProbeLevel) int {
		srv, rec := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
			if isStreaming(body) {
				writeSSEDeltas(w, `{"choices":[{"delta":{"content":"answer"}}]}`)
				return
			}
			writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
		})
		ProbeReasoning(context.Background(), srv.URL, "", "m", level)

		n := 0
		for _, body := range rec.requests() {
			if isStreaming(body) {
				n++
			}
		}
		return n
	}

	if got := countStreams(ProbeOff); got != 0 {
		t.Errorf("ProbeOff made %d requests, want 0", got)
	}
	if got := countStreams(ProbeLight); got != 1 {
		t.Errorf("ProbeLight made %d requests, want exactly 1", got)
	}
	if got := countStreams(ProbeFull); got <= 1 {
		t.Errorf("ProbeFull made %d requests, want more than 1", got)
	}
}

// P1-010: a slow item degrades on its own and does not stop the rest of the
// probe.
func TestFEATURE620ProbeSlowItemDegrades(t *testing.T) {
	restore := probeItemTimeout
	probeItemTimeout = 150 * time.Millisecond
	t.Cleanup(func() { probeItemTimeout = restore })

	srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
		if isStreaming(body) {
			// Never answers within the (shortened) item timeout.
			time.Sleep(time.Second)
			writeSSEDeltas(w, `{"choices":[{"delta":{"reasoning":"late"}}]}`)
			return
		}
		if _, ok := body["tools"]; ok {
			writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"probe_noop","arguments":"{}"}}]}}]}`)
			return
		}
		writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	})

	start := time.Now()
	profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeFull)
	elapsed := time.Since(start)

	if profile.ReasoningField == ReasoningFieldReasoning || profile.ReasoningField == ReasoningFieldContent {
		t.Errorf("timed-out item reported a detected field: %q", profile.ReasoningField)
	}
	if profile.ToolCallStyle != ToolCallStyleNative {
		t.Errorf("ToolCallStyle = %q, want %q: a failed item must not stop the others",
			profile.ToolCallStyle, ToolCallStyleNative)
	}
	if elapsed > 5*time.Second {
		t.Errorf("probe took %v: the item timeout did not bound it", elapsed)
	}
}

// P1-012: probe requests are always logged, and the setting is restored.
func TestFEATURE620ProbeEnablesInteractionLogging(t *testing.T) {
	wasEnabled := log.IsLLMInteractionEnabled()
	t.Cleanup(func() { log.SetLLMInteractionEnabled(wasEnabled) })
	log.SetLLMInteractionEnabled(false)

	var sawEnabled bool
	srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
		if log.IsLLMInteractionEnabled() {
			sawEnabled = true
		}
		writeSSEDeltas(w, `{"choices":[{"delta":{"reasoning":"think"}}]}`)
	})

	ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeLight)

	if !sawEnabled {
		t.Error("the probe did not enable LLM interaction logging")
	}
	if log.IsLLMInteractionEnabled() {
		t.Error("the probe did not restore the LLM interaction logging setting")
	}
}

// P1-013: inline chain-of-thought is reported as a hint only — the probe never
// selects inline stripping on its own (the risk of cutting real answer text
// stays with the user).
func TestFEATURE620ProbeInlineTagsAreOnlyAHint(t *testing.T) {
	srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
		if isStreaming(body) {
			writeSSEDeltas(w, `{"choices":[{"delta":{"content":"answer"}}]}`)
			return
		}
		if _, ok := body["tools"]; ok {
			writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"no tool"}}]}`)
			return
		}
		writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"<thinking>reasoning</thinking>answer"}}]}`)
	})

	profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeFull)

	if len(profile.ReasoningTags) != 1 || profile.ReasoningTags[0] != "thinking" {
		t.Errorf("ReasoningTags = %v, want [thinking]", profile.ReasoningTags)
	}
	if profile.ReasoningField == ReasoningFieldInline {
		t.Error("ReasoningField must stay \"none\": inline stripping is opt-in, not auto-detected")
	}
}

// P1-012 helper path: tool-call style detection covers all three outcomes.
func TestFEATURE620ProbeToolCallStyle(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "native",
			body: `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"probe_noop","arguments":"{}"}}]}}]}`,
			want: ToolCallStyleNative,
		},
		{
			name: "text",
			body: `{"choices":[{"message":{"role":"assistant","content":"I would call probe_noop now"}}]}`,
			want: ToolCallStyleText,
		},
		{
			name: "none",
			body: `{"choices":[{"message":{"role":"assistant","content":"plain answer"}}]}`,
			want: ToolCallStyleNone,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
				if isStreaming(body) {
					writeSSEDeltas(w, `{"choices":[{"delta":{"content":"answer"}}]}`)
					return
				}
				if _, ok := body["tools"]; ok {
					writeJSON(w, tc.body)
					return
				}
				writeJSON(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
			})

			profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeFull)
			if profile.ToolCallStyle != tc.want {
				t.Errorf("ToolCallStyle = %q, want %q", profile.ToolCallStyle, tc.want)
			}
		})
	}
}

// The engine is recognised from system_fingerprint when the stream carries one.
func TestFEATURE620ProbeRecognisesEngine(t *testing.T) {
	srv, _ := probeServer(t, func(body map[string]interface{}, w http.ResponseWriter) {
		writeSSEDeltas(w,
			`{"choices":[{"delta":{"reasoning":"think"}}],"system_fingerprint":"vllm-0.25.2.dev0+g752a3a504"}`,
		)
	})

	profile := ProbeReasoning(context.Background(), srv.URL, "", "m", ProbeLight)
	if profile.Engine != EngineVLLM {
		t.Errorf("Engine = %q, want %q", profile.Engine, EngineVLLM)
	}
}
