package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// FEATURE-620 P2 acceptance tests: response-side decoupling and inline
// chain-of-thought stripping (use-case/FEATURE-620/FEATURE-620-UC-0003.md).

// --- P2-001..P2-004: ThinkingProtocol overrides the provider name -------------

func TestFEATURE620ThinkingProtocolOverridesProvider(t *testing.T) {
	enabled := ThinkingConfig{Mode: ThinkingModeEnabled}

	t.Run("P2-001 empty protocol keeps the provider default", func(t *testing.T) {
		for _, provider := range []string{"deepseek", "qwen", "kimi", "zhipu", "openai", "openai-compatible"} {
			got := GetThinkingAdapterFor("", provider).BuildAdditions(enabled)
			want := GetThinkingAdapter(provider).BuildAdditions(enabled)
			if !sameAdditions(got, want) {
				t.Errorf("provider %q: got %v, want %v", provider, got, want)
			}
		}
	})

	t.Run("P2-002 explicit protocol wins over a generic provider", func(t *testing.T) {
		// A self-hosted engine reports the provider name "openai-compatible",
		// which cannot describe the model behind it.
		got := GetThinkingAdapterFor("qwen", "openai-compatible").BuildAdditions(enabled)
		if len(got) == 0 {
			t.Fatalf("qwen protocol produced no additions: %v", got)
		}
		fallback := GetThinkingAdapter("openai-compatible").BuildAdditions(enabled)
		if sameAdditions(got, fallback) {
			t.Errorf("explicit protocol was ignored, got the fallback additions: %v", got)
		}
		qwenDirect := GetThinkingAdapter("qwen").BuildAdditions(enabled)
		if !sameAdditions(got, qwenDirect) {
			t.Errorf("got %v, want the qwen adapter output %v", got, qwenDirect)
		}
	})

	t.Run("P2-003 unknown protocol degrades to the provider", func(t *testing.T) {
		got := GetThinkingAdapterFor("not-a-protocol", "deepseek").BuildAdditions(enabled)
		want := GetThinkingAdapter("deepseek").BuildAdditions(enabled)
		if !sameAdditions(got, want) {
			t.Errorf("got %v, want the provider fallback %v", got, want)
		}
	})

	t.Run("P2-004 vendor request shapes are unchanged", func(t *testing.T) {
		// Guards the shapes vendor templates rely on.
		if got := GetThinkingAdapterFor("deepseek", "deepseek").BuildAdditions(enabled); !strings.Contains(got["thinking"], "enabled") {
			t.Errorf("deepseek thinking = %q, want an enabled object", got["thinking"])
		}
		qwen := GetThinkingAdapterFor("qwen", "qwen").BuildAdditions(enabled)
		if !strings.Contains(qwen["extra_body"], "enable_thinking") {
			t.Errorf("qwen extra_body = %q, want chat_template_kwargs.enable_thinking", qwen["extra_body"])
		}
		openai := GetThinkingAdapterFor("openai", "openai").BuildAdditions(enabled)
		if _, ok := openai["reasoning_effort"]; !ok {
			t.Errorf("openai additions = %v, want reasoning_effort", openai)
		}
	})
}

// sameAdditions compares two adapter addition maps.
func sameAdditions(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// ThinkingProtocolCandidates must offer "auto" first and the adapter keys after.
func TestFEATURE620ThinkingProtocolCandidates(t *testing.T) {
	candidates := ThinkingProtocolCandidates()
	if len(candidates) == 0 || candidates[0] != "" {
		t.Fatalf("candidates = %v, want an empty auto entry first", candidates)
	}
	for _, want := range thinkingProtocolProbeOrder {
		found := false
		for _, c := range candidates {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("candidate %q missing from %v", want, candidates)
		}
	}
}

// --- P2-008..P2-011: inline chain-of-thought stripping -----------------------

// inlineClient builds a client configured for inline chain-of-thought.
func inlineClient(t *testing.T, endpoint string) Client {
	t.Helper()
	c := NewClient(endpoint, "k", "m", 0, 0, 15)
	c.SetReasoningField(ReasoningFieldInline)
	c.SetReasoningTags([]string{"thinking"})
	return c
}

// P2-008: the tags are removed from the answer and reported as thinking.
func TestFEATURE620InlineStrippingNonStreaming(t *testing.T) {
	srv := chatMockServer(t, `{"choices":[{"message":{"role":"assistant","content":"<thinking>推理过程</thinking>答案"}}]}`)
	client := inlineClient(t, srv.URL)
	defer client.Close()

	resp, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ReasoningContent != "推理过程" {
		t.Errorf("ReasoningContent = %q, want %q", resp.ReasoningContent, "推理过程")
	}
	if resp.Content != "答案" {
		t.Errorf("Content = %q, want %q", resp.Content, "答案")
	}
}

// P2-009: without an explicit inline field the content is untouched.
func TestFEATURE620InlineStrippingIsOffByDefault(t *testing.T) {
	const body = `{"choices":[{"message":{"role":"assistant","content":"<thinking>推理过程</thinking>答案"}}]}`
	srv := chatMockServer(t, body)
	client := NewClient(srv.URL, "k", "m", 0, 0, 15)
	defer client.Close()

	resp, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "<thinking>推理过程</thinking>答案" {
		t.Errorf("Content = %q, want the untouched text", resp.Content)
	}
	if resp.ReasoningContent != "" {
		t.Errorf("ReasoningContent = %q, want empty", resp.ReasoningContent)
	}
}

// P2-010: tags split across stream chunks must not leak into the answer.
func TestFEATURE620InlineStrippingAcrossStreamChunks(t *testing.T) {
	srv := sseMockServer(t,
		`{"choices":[{"delta":{"content":"<thin"}}]}`,
		`{"choices":[{"delta":{"content":"king>推理"}}]}`,
		`{"choices":[{"delta":{"content":"过程</thin"}}]}`,
		`{"choices":[{"delta":{"content":"king>答案"}}]}`,
	)
	client := inlineClient(t, srv.URL)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ch, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var answer, thinking string
	for ev := range ch {
		switch ev.Type {
		case StreamEventContent:
			answer += ev.Content
		case StreamEventReasoning:
			thinking += ev.Content
		}
	}

	if strings.Contains(answer, "thin") || strings.Contains(answer, "<") || strings.Contains(answer, ">") {
		t.Errorf("answer leaked tag text: %q", answer)
	}
	if answer != "答案" {
		t.Errorf("answer = %q, want %q", answer, "答案")
	}
	if thinking != "推理过程" {
		t.Errorf("thinking = %q, want %q", thinking, "推理过程")
	}
}

// P2-011: a stream cut off inside a tag must not lose text or leak a half tag.
func TestFEATURE620InlineStrippingUnclosedTagFallback(t *testing.T) {
	srv := sseMockServer(t, `{"choices":[{"delta":{"content":"<thinking>推理"}}]}`)
	client := inlineClient(t, srv.URL)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ch, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var answer, thinking string
	for ev := range ch {
		switch ev.Type {
		case StreamEventContent:
			answer += ev.Content
		case StreamEventReasoning:
			thinking += ev.Content
		}
	}

	if !strings.Contains(answer+thinking, "推理") {
		t.Errorf("the buffered text was lost: answer=%q thinking=%q", answer, thinking)
	}
	if strings.Contains(answer, "<thinking>") {
		t.Errorf("a half tag leaked into the answer: %q", answer)
	}
}

// The stripper itself: buffering, minimum churn and flush semantics.
func TestFEATURE620InlineCoTStripper(t *testing.T) {
	t.Run("plain text passes through", func(t *testing.T) {
		s := NewInlineCoTStripper([]string{"thinking"})
		answer, thinking := s.Write("hello")
		answer += flushAnswer(s, &thinking)
		if answer != "hello" || thinking != "" {
			t.Errorf("answer=%q thinking=%q, want hello/\"\"", answer, thinking)
		}
	})

	t.Run("no tags configured means pass-through", func(t *testing.T) {
		s := NewInlineCoTStripper(nil)
		answer, thinking := s.Write("<thinking>x</thinking>")
		if answer != "<thinking>x</thinking>" || thinking != "" {
			t.Errorf("answer=%q thinking=%q, want the untouched text", answer, thinking)
		}
	})

	t.Run("a partial tag is withheld until it resolves", func(t *testing.T) {
		s := NewInlineCoTStripper([]string{"thinking"})
		answer, _ := s.Write("ok<thi")
		if strings.Contains(answer, "<thi") {
			t.Errorf("a partial tag leaked: %q", answer)
		}
		answer2, thinking2 := s.Write("nking>why</thinking>done")
		if answer+answer2 != "okdone" {
			t.Errorf("answer = %q, want %q", answer+answer2, "okdone")
		}
		if thinking2 != "why" {
			t.Errorf("thinking = %q, want %q", thinking2, "why")
		}
	})
}

// flushAnswer flushes the stripper and appends any thinking it reports.
func flushAnswer(s *InlineCoTStripper, thinking *string) string {
	a, th := s.Flush()
	*thinking += th
	return a
}

// The tool-call probe must not be affected by inline configuration.
func TestFEATURE620InlineDoesNotAffectToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"plain\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	client := inlineClient(t, srv.URL)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ch, err := client.ChatStream(ctx, []Message{{Role: "user", Content: "q"}}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var answer string
	for ev := range ch {
		if ev.Type == StreamEventContent {
			answer += ev.Content
		}
	}
	if answer != "plain" {
		t.Errorf("answer = %q, want %q", answer, "plain")
	}
}
