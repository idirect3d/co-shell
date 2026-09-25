// FIX-555: token timing/rate tests.
//
// The reported windows follow one LLM request (user's metric): the input window
// is "request submitted → first output character", the output window is "first
// output character → end of this round's output (thinking + answer + every
// tool-call argument)". Tool execution happens after the stream ends and is
// never part of either window.
//
// Two defects are pinned here:
//  1. the rate numerators used to be session accumulators (totalTokens -
//     prevTotalTokens), which spanned every call since the previous read while
//     the windows covered only the last one — every iteration that ended through
//     a `continue` path inflated the next rate;
//  2. the stamps were cleared only when GetLLMTiming read them, so a call that
//     ended without a first-token (or Done) event inherited the previous call's
//     stamps and printed a negative TTFT next to a multi-hour window.

package agent

import (
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
)

// UC-024: the rate divides this call's tokens by this call's window.
func TestFIX555TimingUsesOneCall(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 1000, 100, time.Second, 4*time.Second)

	got := a.GetLLMTiming()
	if got.FirstTokenLatency != "1.0s" {
		t.Errorf("FirstTokenLatency = %q, want %q", got.FirstTokenLatency, "1.0s")
	}
	if got.OutputDuration != "4.0s" {
		t.Errorf("OutputDuration = %q, want %q", got.OutputDuration, "4.0s")
	}
	if got.InputTPS != "1000" {
		t.Errorf("InputTPS = %q, want %q (1000 tokens / 1s)", got.InputTPS, "1000")
	}
	if got.OutputTPS != "25" {
		t.Errorf("OutputTPS = %q, want %q (100 tokens / 4s)", got.OutputTPS, "25")
	}
}

// UC-024: session totals from earlier calls must not leak into the rate. This is
// the exact regression that made a one-call window carry several calls' tokens.
func TestFIX555TimingIgnoresSessionTotals(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 1000, 100, time.Second, 4*time.Second)

	a.mu.Lock()
	a.totalPromptTokens = 500000
	a.totalCompletionTokens = 90000
	a.totalTokens = 590000
	a.mu.Unlock()

	got := a.GetLLMTiming()
	if got.InputTPS != "1000" {
		t.Errorf("InputTPS = %q, want %q — session accumulators must not be used", got.InputTPS, "1000")
	}
	if got.OutputTPS != "25" {
		t.Errorf("OutputTPS = %q, want %q — session accumulators must not be used", got.OutputTPS, "25")
	}
}

// UC-024: a request whose engine reported no usage yields an empty result, and
// the window is still consumed so its stamps cannot leak into the next call.
func TestFIX555NoUsageClearsStamps(t *testing.T) {
	a := &Agent{}
	a.mu.Lock()
	a.llmCallStartTime = time.Now().Add(-3 * time.Second)
	a.firstTokenTime = time.Now().Add(-2 * time.Second)
	a.llmStreamEndTime = time.Now()
	a.mu.Unlock()

	if got := a.GetLLMTiming(); got != (LLMTiming{}) {
		t.Errorf("GetLLMTiming() = %+v, want the zero value when no usage was reported", got)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.llmCallStartTime.IsZero() || !a.firstTokenTime.IsZero() || !a.llmStreamEndTime.IsZero() {
		t.Error("stamps were not cleared on the no-usage path (they would leak into the next call)")
	}
}

// UC-025: a fresh call starts from a clean record.
func TestFIX555BeginLLMCallClearsStamps(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 1000, 100, time.Second, time.Second)

	a.beginLLMCall(false)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.llmCallStartTime.IsZero() {
		t.Error("llmCallStartTime not set")
	}
	if !a.firstTokenTime.IsZero() || !a.llmStreamEndTime.IsZero() {
		t.Error("previous call's first-token/end stamps survived into the new call")
	}
	if a.callUsageReported {
		t.Error("callUsageReported not reset")
	}
}

// UC-025: a retry keeps the interrupted attempt's start so the window covers the
// whole wait, while the stale stamps of that attempt are dropped.
func TestFIX555BeginLLMCallResumeKeepsStart(t *testing.T) {
	a := &Agent{}
	seedTiming(a, 1000, 100, 2*time.Second, 3*time.Second)

	a.mu.Lock()
	originalStart := a.llmCallStartTime
	a.mu.Unlock()

	a.beginLLMCall(true)

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.llmCallStartTime.Equal(originalStart) {
		t.Errorf("resumed start = %v, want the interrupted attempt's start %v", a.llmCallStartTime, originalStart)
	}
	if !a.firstTokenTime.IsZero() || !a.llmStreamEndTime.IsZero() {
		t.Error("aborted attempt's window survived into the retry")
	}
}

// UC-026: a non-streaming call fills the input window (whole wall clock) and
// deliberately leaves the output window unmeasured — the first output character
// is not observable on that path, so reporting a generated window would be an
// invention.
func TestFIX555RecordNonStreamingCall(t *testing.T) {
	a := &Agent{}
	start := time.Now().Add(-2 * time.Second)
	a.recordNonStreamingCall(start, true)

	a.mu.Lock()
	a.iterPromptTokens = 500
	a.iterCompletionTokens = 50
	a.iterTokens = 550
	a.mu.Unlock()

	got := a.GetLLMTiming()
	if got.FirstTokenLatency == "-" || got.FirstTokenLatency == "" {
		t.Errorf("FirstTokenLatency = %q, want the wall-clock input window", got.FirstTokenLatency)
	}
	if got.InputTPS == "-" || got.InputTPS == "" {
		t.Errorf("InputTPS = %q, want a rate over the wall clock", got.InputTPS)
	}
	if got.OutputDuration != "-" || got.OutputTPS != "-" {
		t.Errorf("output window = (%q, %q), want \"-\" — it cannot be measured without stream events",
			got.OutputDuration, got.OutputTPS)
	}
}

// UC-027: the token line is emitted only when the mode is on and the call
// reported tokens.
func TestFIX555EmitIterTokenStatsGate(t *testing.T) {
	count := func(a *Agent, cb StreamCallback) int {
		n := 0
		a.emitIterTokenStats(func(StreamEvent) { n++ })
		return n
	}

	on := &Agent{cfg: config.DefaultConfig()}
	on.cfg.LLM.TokenUsage = "on"
	off := &Agent{cfg: config.DefaultConfig()}
	off.cfg.LLM.TokenUsage = "off"

	// Mode off: nothing, even with tokens present.
	off.mu.Lock()
	off.iterTokens = 100
	off.mu.Unlock()
	if n := count(off, nil); n != 0 {
		t.Errorf("token-usage=off emitted %d events, want 0", n)
	}

	// Mode on but no tokens reported for this call: nothing.
	if n := count(on, nil); n != 0 {
		t.Errorf("zero-token call emitted %d events, want 0", n)
	}

	// Mode on with tokens: exactly one event.
	seedTiming(on, 1000, 100, time.Second, 4*time.Second)
	if n := count(on, nil); n != 1 {
		t.Errorf("populated call emitted %d events, want 1", n)
	}
}
