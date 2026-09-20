// FEATURE-546: GET /api/usage aggregates the persisted per-call token records
// (store.TokenUsageEntry) into the history overview the status-bar popover
// renders. Everything is derived on request from the token_usage bucket, so
// there is no second copy of the statistics to keep in sync.
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"net/http"
	"sort"
	"time"

	"github.com/idirect3d/co-shell/store"
)

// maxDayBuckets / maxHourBuckets cap the trend series so a long-lived database
// cannot produce an unbounded payload. Only the most recent buckets are kept.
const (
	maxDayBuckets  = 60
	maxHourBuckets = 72
)

// usageSummary is the totals block of the response.
type usageSummary struct {
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	// CacheAvailable is false when no recorded call carried prompt_tokens_details,
	// i.e. the engine runs without prefix caching. The front end must then show
	// the reason instead of a 0% hit rate.
	CacheAvailable bool    `json:"cache_available"`
	CacheRate      float64 `json:"cache_rate"`
	FirstSeen      string  `json:"first_seen,omitempty"`
	LastSeen       string  `json:"last_seen,omitempty"`
}

// usageBucket is one point of a token trend series.
type usageBucket struct {
	Label     string `json:"label"`
	Prompt    int    `json:"prompt"`
	Completion int   `json:"completion"`
}

// usageRateBucket is one point of the speed trend series. Counts report how
// many calls contributed to the average, so sparse buckets stay visible.
type usageRateBucket struct {
	Label   string  `json:"label"`
	InTPS   float64 `json:"in_tps"`
	OutTPS  float64 `json:"out_tps"`
	Count   int     `json:"count"`
}

// usageResponse is the GET /api/usage payload.
type usageResponse struct {
	Summary   usageSummary      `json:"summary"`
	ByDay     []usageBucket     `json:"by_day"`
	ByHour    []usageBucket     `json:"by_hour"`
	RateByDay []usageRateBucket `json:"rate_by_day"`
}

// bucketOrder keeps buckets in first-seen order. Entries arrive chronological,
// so insertion order is already the chronological order of the series.
type bucketOrder struct {
	labels []string
	index  map[string]int
}

func newBucketOrder() *bucketOrder {
	return &bucketOrder{index: map[string]int{}}
}

// pos returns the slot of a label, appending it on first sight.
func (b *bucketOrder) pos(label string) int {
	if i, ok := b.index[label]; ok {
		return i
	}
	i := len(b.labels)
	b.index[label] = i
	b.labels = append(b.labels, label)
	return i
}

// handleUsage serves the aggregated token history for the status-bar popover.
// An empty database answers 200 with zeroed totals and empty series so the
// front end can distinguish "no history" from a failed request.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	resp := usageResponse{
		ByDay:     []usageBucket{},
		ByHour:    []usageBucket{},
		RateByDay: []usageRateBucket{},
	}
	if s.usageFn == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	entries, err := s.usageFn()
	if err != nil {
		// Generic reason only: the driver error may carry filesystem paths.
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "usage: list failed"})
		return
	}
	writeJSON(w, http.StatusOK, aggregateUsage(entries))
}

// aggregateUsage folds raw per-call entries into the response. It is separated
// from the HTTP layer so it can be unit tested directly.
func aggregateUsage(entries []store.TokenUsageEntry) usageResponse {
	resp := usageResponse{
		ByDay:     []usageBucket{},
		ByHour:    []usageBucket{},
		RateByDay: []usageRateBucket{},
	}
	if len(entries) == 0 {
		return resp
	}

	dayOrder, hourOrder := newBucketOrder(), newBucketOrder()
	days := map[string]*usageBucket{}
	hours := map[string]*usageBucket{}
	// Per-day sums of tokens and seconds, so the daily speed is computed as
	// sum(tokens)/sum(seconds) — a token-weighted mean that a single long call
	// cannot skew the way a per-call average would.
	type secAcc struct {
		prompt, completion int
		inSec, outSec      float64
		count              int
	}
	rates := map[string]*secAcc{}
	rateOrder := newBucketOrder()

	var cachePrompt, cacheCached int
	var firstTS, lastTS time.Time
	firstSeen := false

	for i := range entries {
		e := &entries[i]
		resp.Summary.Requests++
		resp.Summary.PromptTokens += e.PromptTokens
		resp.Summary.CompletionTokens += e.CompletionTokens
		resp.Summary.TotalTokens += e.TotalTokens

		if !firstSeen {
			firstTS, lastTS, firstSeen = e.Timestamp, e.Timestamp, true
		}
		if e.Timestamp.Before(firstTS) {
			firstTS = e.Timestamp
		}
		if e.Timestamp.After(lastTS) {
			lastTS = e.Timestamp
		}

		// Only calls that actually reported prompt_tokens_details may feed the
		// hit rate; otherwise an engine without prefix caching would pull the
		// historical figure to 0%.
		if e.CachedReported && e.PromptTokens > 0 {
			cachePrompt += e.PromptTokens
			cacheCached += e.CachedTokens
			resp.Summary.CacheAvailable = true
		}

		dayLabel := e.Timestamp.Format("01-02")
		hourLabel := e.Timestamp.Format("01-02 15:00")

		db := days[dayLabel]
		if db == nil {
			db = &usageBucket{Label: dayLabel}
			days[dayLabel] = db
			dayOrder.pos(dayLabel)
		}
		db.Prompt += e.PromptTokens
		db.Completion += e.CompletionTokens

		hb := hours[hourLabel]
		if hb == nil {
			hb = &usageBucket{Label: hourLabel}
			hours[hourLabel] = hb
			hourOrder.pos(hourLabel)
		}
		hb.Prompt += e.PromptTokens
		hb.Completion += e.CompletionTokens

		if e.FirstTokenSec > 0 || e.OutputSec > 0 {
			ra := rates[dayLabel]
			if ra == nil {
				ra = &secAcc{}
				rates[dayLabel] = ra
				rateOrder.pos(dayLabel)
			}
			ra.count++
			if e.FirstTokenSec > 0 {
				ra.prompt += e.PromptTokens
				ra.inSec += e.FirstTokenSec
			}
			if e.OutputSec > 0 {
				ra.completion += e.CompletionTokens
				ra.outSec += e.OutputSec
			}
		}
	}

	if resp.Summary.CacheAvailable && cachePrompt > 0 {
		resp.Summary.CacheRate = clampRate(float64(cacheCached) / float64(cachePrompt))
	}
	resp.Summary.CachedTokens = cacheCached
	resp.Summary.FirstSeen = firstTS.Format(time.RFC3339)
	resp.Summary.LastSeen = lastTS.Format(time.RFC3339)

	resp.ByDay = trimBuckets(dayOrder, days, maxDayBuckets)
	resp.ByHour = trimBuckets(hourOrder, hours, maxHourBuckets)
	for _, label := range trimLabels(rateOrder.labels, maxDayBuckets) {
		ra := rates[label]
		rb := usageRateBucket{Label: label, Count: ra.count}
		if ra.inSec > 0 {
			rb.InTPS = float64(ra.prompt) / ra.inSec
		}
		if ra.outSec > 0 {
			rb.OutTPS = float64(ra.completion) / ra.outSec
		}
		resp.RateByDay = append(resp.RateByDay, rb)
	}
	return resp
}

// clampRate keeps a computed ratio inside [0, 1] even if an engine reports more
// cached tokens than prompt tokens.
func clampRate(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// trimBuckets turns an accumulated bucket map into a series, keeping only the
// most recent max entries in chronological order.
func trimBuckets(order *bucketOrder, buckets map[string]*usageBucket, max int) []usageBucket {
	labels := trimLabels(order.labels, max)
	out := make([]usageBucket, 0, len(labels))
	for _, l := range labels {
		out = append(out, *buckets[l])
	}
	return out
}

// trimLabels keeps the tail of a first-seen-ordered label list, then restores
// chronological order (the labels sort lexicographically for the formats used).
func trimLabels(labels []string, max int) []string {
	if len(labels) > max {
		labels = labels[len(labels)-max:]
	}
	out := make([]string, len(labels))
	copy(out, labels)
	sort.Strings(out)
	return out
}


