// FEATURE-546 unit tests: aggregation behind GET /api/usage (group B).
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/store"
)

// errBoom stands in for a store failure when checking that internal error text
// never reaches the browser.
var errBoom = errors.New("boom")

// ts builds a timestamp in the local zone, which is what the day/hour buckets
// are keyed on.
func ts(day, hour int) time.Time {
	return time.Date(2026, 9, day, hour, 30, 0, 0, time.Local)
}

// UC-0004: totals add up and the hit rate is token-weighted over the reported
// entries only — an entry without prompt_tokens_details must not drag the
// figure toward 0%.
func TestFEATURE546AggregateTotalsAndWeightedHitRate(t *testing.T) {
	entries := []store.TokenUsageEntry{
		{PromptTokens: 1000, CompletionTokens: 100, TotalTokens: 1100, Timestamp: ts(1, 9), CachedTokens: 800, CachedReported: true},
		{PromptTokens: 3000, CompletionTokens: 200, TotalTokens: 3200, Timestamp: ts(1, 10), CachedTokens: 600, CachedReported: true},
		// Not reported: excluded from the ratio but still counted in totals.
		{PromptTokens: 5000, CompletionTokens: 50, TotalTokens: 5050, Timestamp: ts(2, 9)},
	}
	got := aggregateUsage(entries)

	if got.Summary.Requests != 3 {
		t.Errorf("Requests = %d, want 3", got.Summary.Requests)
	}
	if got.Summary.PromptTokens != 9000 || got.Summary.CompletionTokens != 350 || got.Summary.TotalTokens != 9350 {
		t.Errorf("totals = %+v, want prompt 9000 / completion 350 / total 9350", got.Summary)
	}
	if !got.Summary.CacheAvailable {
		t.Error("CacheAvailable = false, want true")
	}
	// (800+600) / (1000+3000) = 0.35, not 1400/9000.
	if got.Summary.CacheRate < 0.3499 || got.Summary.CacheRate > 0.3501 {
		t.Errorf("CacheRate = %v, want 0.35", got.Summary.CacheRate)
	}
	if got.Summary.CachedTokens != 1400 {
		t.Errorf("CachedTokens = %d, want 1400", got.Summary.CachedTokens)
	}
}

// UC-0005: when no entry reported cached_tokens the aggregate must say so
// instead of presenting a 0% hit rate.
func TestFEATURE546AggregateCacheUnavailable(t *testing.T) {
	got := aggregateUsage([]store.TokenUsageEntry{
		{PromptTokens: 1000, CompletionTokens: 10, TotalTokens: 1010, Timestamp: ts(1, 9)},
		{PromptTokens: 2000, CompletionTokens: 20, TotalTokens: 2020, Timestamp: ts(1, 10), CachedTokens: 0},
	})
	if got.Summary.CacheAvailable {
		t.Error("CacheAvailable = true, want false")
	}
	if got.Summary.CacheRate != 0 {
		t.Errorf("CacheRate = %v, want 0 alongside CacheAvailable=false", got.Summary.CacheRate)
	}
}

// UC-0006: day and hour buckets group by local calendar day and hour, in
// ascending order.
func TestFEATURE546AggregateBuckets(t *testing.T) {
	entries := []store.TokenUsageEntry{
		{PromptTokens: 100, CompletionTokens: 10, Timestamp: ts(1, 9)},
		{PromptTokens: 200, CompletionTokens: 20, Timestamp: ts(1, 9)},
		{PromptTokens: 300, CompletionTokens: 30, Timestamp: ts(1, 14)},
		{PromptTokens: 400, CompletionTokens: 40, Timestamp: ts(2, 9)},
	}
	got := aggregateUsage(entries)

	if len(got.ByDay) != 2 {
		t.Fatalf("len(ByDay) = %d, want 2 (%+v)", len(got.ByDay), got.ByDay)
	}
	if got.ByDay[0].Prompt != 600 || got.ByDay[0].Completion != 60 {
		t.Errorf("ByDay[0] = %+v, want prompt 600 / completion 60", got.ByDay[0])
	}
	if got.ByDay[1].Prompt != 400 {
		t.Errorf("ByDay[1] = %+v, want prompt 400", got.ByDay[1])
	}

	if len(got.ByHour) != 3 {
		t.Fatalf("len(ByHour) = %d, want 3 (%+v)", len(got.ByHour), got.ByHour)
	}
	if got.ByHour[0].Prompt != 300 {
		t.Errorf("ByHour[0] = %+v, want prompt 300", got.ByHour[0])
	}
}

// UC-0008: the daily speed is sum(tokens)/sum(seconds) and entries with no
// recorded timing are skipped entirely.
func TestFEATURE546AggregateRateBuckets(t *testing.T) {
	entries := []store.TokenUsageEntry{
		{PromptTokens: 1000, CompletionTokens: 100, Timestamp: ts(1, 9), FirstTokenSec: 2, OutputSec: 4},
		{PromptTokens: 3000, CompletionTokens: 200, Timestamp: ts(1, 10), FirstTokenSec: 6, OutputSec: 6},
		// No timing recorded: contributes to totals but not to the speed series.
		{PromptTokens: 9000, CompletionTokens: 900, Timestamp: ts(1, 11)},
	}
	got := aggregateUsage(entries)
	if len(got.RateByDay) != 1 {
		t.Fatalf("len(RateByDay) = %d, want 1 (%+v)", len(got.RateByDay), got.RateByDay)
	}
	rb := got.RateByDay[0]
	if rb.Count != 2 {
		t.Errorf("Count = %d, want 2 (untimed entry must be skipped)", rb.Count)
	}
	// in: (1000+3000)/(2+6) = 500; out: (100+200)/(4+6) = 30.
	if rb.InTPS != 500 {
		t.Errorf("InTPS = %v, want 500", rb.InTPS)
	}
	if rb.OutTPS != 30 {
		t.Errorf("OutTPS = %v, want 30", rb.OutTPS)
	}
}

// UC-0009: an empty database answers 200 with zeroed totals and empty (not
// null) series, so the browser can tell "no history" from a failed request.
func TestFEATURE546UsageEndpointEmpty(t *testing.T) {
	s := &Server{usageFn: func() ([]store.TokenUsageEntry, error) { return nil, nil }}
	rec := httptest.NewRecorder()
	s.handleUsage(rec, httptest.NewRequest(http.MethodGet, "/api/usage", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got usageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	if got.Summary.Requests != 0 || got.Summary.CacheAvailable {
		t.Errorf("summary = %+v, want zeroed and cache unavailable", got.Summary)
	}
	if got.ByDay == nil || got.ByHour == nil || got.RateByDay == nil {
		t.Errorf("series must serialise as [] not null: %+v", got)
	}
}

// A store failure must not leak the driver error; the browser gets a generic
// reason and a non-200 status.
func TestFEATURE546UsageEndpointError(t *testing.T) {
	s := &Server{usageFn: func() ([]store.TokenUsageEntry, error) {
		return nil, errBoom
	}}
	rec := httptest.NewRecorder()
	s.handleUsage(rec, httptest.NewRequest(http.MethodGet, "/api/usage", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); body == "" || len(body) > 200 {
		t.Errorf("unexpected error body: %s", body)
	}
}

// Without a wired store the endpoint still answers with the empty shape rather
// than 404 or a panic.
func TestFEATURE546UsageEndpointNoStore(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleUsage(rec, httptest.NewRequest(http.MethodGet, "/api/usage", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
