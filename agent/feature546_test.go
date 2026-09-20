// FEATURE-546 unit tests: the cache_rate meta key carried by token_iter.
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import "testing"

// UC-0017: when the engine reported prompt_tokens_details, the event carries
// the per-call hit rate as a percentage string.
func TestFEATURE546EmitTokenIterCarriesCacheRate(t *testing.T) {
	a := &Agent{iterCachedTokens: 3000, iterCachedReported: true}
	var evs []StreamEvent
	a.emitTokenIter(func(ev StreamEvent) { evs = append(evs, ev) }, 12000, 678, 12678, 1048576,
		LLMTiming{FirstTokenLatency: "1.5s", InputTPS: "8000", OutputTPS: "45"})

	if len(evs) != 1 {
		t.Fatalf("callback fired %d times, want 1", len(evs))
	}
	if got := evs[0].Meta[MetaKeyCacheRate]; got != "25.0%" {
		t.Errorf("Meta[%q] = %q, want %q", MetaKeyCacheRate, got, "25.0%")
	}
}

// UC-0018: an engine that never sent prompt_tokens_details must not produce a
// cache_rate key at all — the browser then shows the reason instead of 0%.
func TestFEATURE546EmitTokenIterOmitsCacheRateWhenUnreported(t *testing.T) {
	a := &Agent{iterCachedTokens: 0, iterCachedReported: false}
	var evs []StreamEvent
	a.emitTokenIter(func(ev StreamEvent) { evs = append(evs, ev) }, 12000, 678, 12678, 1048576,
		LLMTiming{FirstTokenLatency: "1.5s"})

	if _, ok := evs[0].Meta[MetaKeyCacheRate]; ok {
		t.Errorf("Meta[%q] = %q, want absent", MetaKeyCacheRate, evs[0].Meta[MetaKeyCacheRate])
	}
}

// A reported genuine 0% hit rate still has to be emitted, so "prefix caching on
// but nothing reused" stays distinguishable from "not reported".
func TestFEATURE546EmitTokenIterEmitsZeroRateWhenReported(t *testing.T) {
	a := &Agent{iterCachedTokens: 0, iterCachedReported: true}
	var evs []StreamEvent
	a.emitTokenIter(func(ev StreamEvent) { evs = append(evs, ev) }, 12000, 678, 12678, 1048576,
		LLMTiming{FirstTokenLatency: "1.5s"})

	if got := evs[0].Meta[MetaKeyCacheRate]; got != "0.0%" {
		t.Errorf("Meta[%q] = %q, want %q", MetaKeyCacheRate, got, "0.0%")
	}
}

// A buggy engine reporting more cached than prompt tokens must not surface a
// rate above 100%.
func TestFEATURE546EmitTokenIterClampsCacheRate(t *testing.T) {
	a := &Agent{iterCachedTokens: 20000, iterCachedReported: true}
	var evs []StreamEvent
	a.emitTokenIter(func(ev StreamEvent) { evs = append(evs, ev) }, 12000, 678, 12678, 1048576,
		LLMTiming{FirstTokenLatency: "1.5s"})

	if got := evs[0].Meta[MetaKeyCacheRate]; got != "100.0%" {
		t.Errorf("Meta[%q] = %q, want %q", MetaKeyCacheRate, got, "100.0%")
	}
}
