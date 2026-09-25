package agent

// FIX-545: token output-performance statistics.
//
// Two defects are pinned here:
//  1. A round whose output is only tool_calls never recorded a first-token time,
//     so every rate and duration collapsed to "-" ("↓1,266 (-, -)").
//  2. The output duration was not shipped to the frontend, which had to derive it
//     from the rounded tokens-per-second rate (and duplicated the input duration).

import (
	"testing"
	"time"

	"github.com/idirect3d/co-shell/llm"
)

// seedTiming lays out a synthetic LLM round on the Agent the way
// streamLLMResponse would: call start, first token, stream end, plus the usage
// counters GetLLMTiming reads.
func seedTiming(a *Agent, prompt, completion int, ttft, genWindow time.Duration) {
	start := time.Now().Add(-(ttft + genWindow))
	a.mu.Lock()
	defer a.mu.Unlock()
	a.llmCallStartTime = start
	a.firstTokenTime = start.Add(ttft)
	a.llmStreamEndTime = start.Add(ttft + genWindow)
	// FIX-555: the rates divide THIS call's usage by THIS call's window, so the
	// fixture seeds the per-call fields (and the reported flag) instead of the
	// session accumulators the old implementation differenced.
	a.iterPromptTokens = prompt
	a.iterCompletionTokens = completion
	a.iterTokens = prompt + completion
	a.callUsageReported = true
}

// assertCompleteTiming requires all four display fields to carry a real value.
func assertCompleteTiming(t *testing.T, got LLMTiming, wantFT, wantOutDur string) {
	t.Helper()
	for name, val := range map[string]string{
		"FirstTokenLatency": got.FirstTokenLatency,
		"InputTPS":          got.InputTPS,
		"OutputTPS":         got.OutputTPS,
		"OutputDuration":    got.OutputDuration,
	} {
		if val == "" || val == "-" {
			t.Errorf("%s = %q, want a real value", name, val)
		}
	}
	if got.FirstTokenLatency != wantFT {
		t.Errorf("FirstTokenLatency = %q, want %q", got.FirstTokenLatency, wantFT)
	}
	if got.OutputDuration != wantOutDur {
		t.Errorf("OutputDuration = %q, want %q", got.OutputDuration, wantOutDur)
	}
}

// UC-0001: a round that emits only tool-call events must still measure rates.
// This is the exact regression the user reported as "↓xxxx (-, -)".
func TestGetLLMTimingToolCallOnlyRound(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 12000, 300, 1500*time.Millisecond, 6*time.Second)

	assertCompleteTiming(t, a.GetLLMTiming(), "1.5s", "6.0s")
}

// UC-0002/UC-0003: content-only and reasoning-only rounds keep working.
func TestGetLLMTimingTextRounds(t *testing.T) {
	for _, evType := range []llm.StreamEventType{llm.StreamEventContent, llm.StreamEventReasoning} {
		if !isFirstTokenEvent(evType) {
			t.Errorf("isFirstTokenEvent(%v) = false, want true", evType)
		}
	}
	a := &Agent{}
	seedTiming(a, 5000, 100, 500*time.Millisecond, 2*time.Second)
	assertCompleteTiming(t, a.GetLLMTiming(), "0.5s", "2.0s")
}

// Done/Error are not output: they must not anchor the first token, otherwise the
// generation window would be measured against the wrong instant.
func TestIsFirstTokenEventExcludesNonOutput(t *testing.T) {
	for _, evType := range []llm.StreamEventType{llm.StreamEventDone, llm.StreamEventError} {
		if isFirstTokenEvent(evType) {
			t.Errorf("isFirstTokenEvent(%v) = true, want false", evType)
		}
	}
}

// UC-0004: with no fresh tokens there is nothing to rate — return empty rather
// than inventing 0 / Inf / NaN.
func TestGetLLMTimingNoNewTokens(t *testing.T) {
	a := &Agent{}
	a.mu.Lock()
	a.llmCallStartTime = time.Now().Add(-time.Second)
	a.firstTokenTime = time.Now()
	a.llmStreamEndTime = time.Now()
	a.mu.Unlock()

	if got := a.GetLLMTiming(); got != (LLMTiming{}) {
		t.Errorf("GetLLMTiming() = %+v, want the zero value", got)
	}
}

// UC-0005: reading the timing resets the stamps so the next round starts clean.
func TestGetLLMTimingResetsAfterRead(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 1000, 50, time.Second, time.Second)
	if got := a.GetLLMTiming(); got.OutputDuration == "-" {
		t.Fatalf("first read returned %+v, want populated", got)
	}

	a.mu.Lock()
	started := !a.llmCallStartTime.IsZero()
	first := !a.firstTokenTime.IsZero()
	end := !a.llmStreamEndTime.IsZero()
	a.mu.Unlock()
	if started || first || end {
		t.Error("timing stamps were not reset after reading")
	}
	if got := a.GetLLMTiming(); got != (LLMTiming{}) {
		t.Errorf("second read = %+v, want the zero value", got)
	}
}

// Durations at or above a minute switch to the "m" unit, matching the frontend.
func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		sec  float64
		want string
	}{
		{0.04, "0.0s"},
		{1.53, "1.5s"},
		{59.9, "59.9s"},
		{60, "1.0m"},
		{150, "2.5m"},
	} {
		if got := formatDuration(tc.sec); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.sec, got, tc.want)
		}
	}
}

// UC-0006: emitTokenIter must carry the measured output duration as out_dur so
// the frontend never has to derive it.
func TestEmitTokenIterCarriesOutDur(t *testing.T) {
	a := &Agent{}
	var evs []StreamEvent
	cb := func(ev StreamEvent) { evs = append(evs, ev) }

	timing := LLMTiming{FirstTokenLatency: "1.5s", InputTPS: "8000", OutputTPS: "45", OutputDuration: "15.1s"}
	a.emitTokenIter(cb, 12000, 678, 12678, 1048576, timing)

	if len(evs) != 1 {
		t.Fatalf("callback fired %d times, want 1", len(evs))
	}
	got := evs[0].Meta
	for k, want := range map[string]string{
		MetaKeyFT:     "1.5s",
		MetaKeyInTPS:  "8000",
		MetaKeyOutTPS: "45",
		MetaKeyOutDur: "15.1s",
		MetaKeyPrompt: "12000",
		MetaKeyTotal:  "12678",
		MetaKeyMax:    "1048576",
		"ctx_index":   "-1",
	} {
		if got[k] != want {
			t.Errorf("Meta[%q] = %q, want %q", k, got[k], want)
		}
	}
}

// UC-0007: TokenIterEvent keeps its 7-argument signature so existing renderer
// tests across four packages compile untouched.
func TestTokenIterEventSignatureUnchanged(t *testing.T) {
	ev := TokenIterEvent(100, 50, 150, 200, "1.0s", "10", "20")
	if ev.Meta[MetaKeyOutDur] != "" {
		t.Errorf("Meta[%q] = %q, want absent (out_dur is attached by emitTokenIter)", MetaKeyOutDur, ev.Meta[MetaKeyOutDur])
	}
	if ev.Meta[MetaKeyFT] != "1.0s" || ev.Meta[MetaKeyInTPS] != "10" || ev.Meta[MetaKeyOutTPS] != "20" {
		t.Errorf("unexpected meta %+v", ev.Meta)
	}
}
