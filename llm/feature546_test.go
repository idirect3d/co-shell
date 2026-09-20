// FEATURE-546 unit tests: KV cache hit-rate parsing (group A) and the vLLM
// MTP acceptance-rate scrape (group C).
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// UC-0001: usage.prompt_tokens_details.cached_tokens decodes and is readable
// through the nil-safe accessor.
func TestFEATURE546UsageParsesCachedTokens(t *testing.T) {
	var u usageJSON
	body := `{"prompt_tokens":12000,"completion_tokens":300,"total_tokens":12300,` +
		`"prompt_tokens_details":{"cached_tokens":9600}}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u.cached() != 9600 {
		t.Errorf("cached() = %d, want 9600", u.cached())
	}
	if u.Details == nil {
		t.Fatal("Details = nil, want non-nil so CachedReported can be set")
	}
}

// UC-0002: an engine without prefix caching sends no prompt_tokens_details at
// all; cached() must be 0 and Details nil, which is what separates "not
// reported" from a genuine 0% hit rate.
func TestFEATURE546UsageWithoutDetails(t *testing.T) {
	var u usageJSON
	body := `{"prompt_tokens":12000,"completion_tokens":300,"total_tokens":12300}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u.Details != nil {
		t.Errorf("Details = %+v, want nil", u.Details)
	}
	if u.cached() != 0 {
		t.Errorf("cached() = %d, want 0", u.cached())
	}
}

// UC-0003: cached() is nil-receiver safe, since usage is optional on stream
// chunks and error responses.
func TestFEATURE546CachedNilSafe(t *testing.T) {
	var u *usageJSON
	if got := u.cached(); got != 0 {
		t.Errorf("cached() on nil = %d, want 0", got)
	}
}

// UC-0010: the acceptance rate is accepted/draft summed over every labelled
// series of both counters.
func TestFEATURE546ParseMTPStatusSumsLabelledSeries(t *testing.T) {
	body := "# HELP vllm:spec_decode_num_accepted_tokens_total Accepted\n" +
		"# TYPE vllm:spec_decode_num_accepted_tokens_total counter\n" +
		`vllm:spec_decode_num_accepted_tokens_total{model_name="qwen",finished="true"} 600` + "\n" +
		`vllm:spec_decode_num_accepted_tokens_total{model_name="qwen",finished="false"} 400` + "\n" +
		`vllm:spec_decode_num_draft_tokens_total{model_name="qwen"} 2000` + "\n"
	got := ParseMTPStatus(body)
	if !got.Available {
		t.Fatalf("Available = false (reason %q), want true", got.Reason)
	}
	if got.Rate < 0.4999 || got.Rate > 0.5001 {
		t.Errorf("Rate = %v, want 0.5", got.Rate)
	}
}

// UC-0011: metrics without any spec_decode counters means the engine was
// started without --speculative-config, which must be reported as such.
func TestFEATURE546ParseMTPStatusNoSpecMetrics(t *testing.T) {
	body := "vllm:num_requests_running{model_name=\"qwen\"} 3\n" +
		"vllm:prompt_tokens_total{model_name=\"qwen\"} 12345\n"
	got := ParseMTPStatus(body)
	if got.Available {
		t.Fatal("Available = true, want false")
	}
	if got.Reason != MTPReasonNoSpecMetrics {
		t.Errorf("Reason = %q, want %q", got.Reason, MTPReasonNoSpecMetrics)
	}
}

// UC-0012: counters present but nothing drafted yet leaves the ratio
// undefined; it must not be reported as 0%.
func TestFEATURE546ParseMTPStatusNoDraftTokens(t *testing.T) {
	body := "vllm:spec_decode_num_accepted_tokens_total 0\n" +
		"vllm:spec_decode_num_draft_tokens_total 0\n"
	got := ParseMTPStatus(body)
	if got.Available {
		t.Fatal("Available = true, want false")
	}
	if got.Reason != MTPReasonNoDraftTokens {
		t.Errorf("Reason = %q, want %q", got.Reason, MTPReasonNoDraftTokens)
	}
}

// UC-0013: only one of the two counters present is still "no spec metrics",
// because the ratio cannot be formed.
func TestFEATURE546ParseMTPStatusPartialMetrics(t *testing.T) {
	body := "vllm:spec_decode_num_draft_tokens_total 500\n"
	got := ParseMTPStatus(body)
	if got.Available || got.Reason != MTPReasonNoSpecMetrics {
		t.Errorf("got %+v, want unavailable with reason %q", got, MTPReasonNoSpecMetrics)
	}
}

// UC-0014: an empty metrics_endpoint short-circuits to "not configured"
// without any network access.
func TestFEATURE546FetchMTPRateNotConfigured(t *testing.T) {
	for _, url := range []string{"", "   "} {
		got := FetchMTPRate(url, 0)
		if got.Available || got.Reason != MTPReasonNotConfigured {
			t.Errorf("url %q: got %+v, want reason %q", url, got, MTPReasonNotConfigured)
		}
	}
}

// UC-0015: a reachable endpoint is scraped and parsed end to end.
func TestFEATURE546FetchMTPRateOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("vllm:spec_decode_num_accepted_tokens_total 75\n" +
			"vllm:spec_decode_num_draft_tokens_total 100\n"))
	}))
	defer srv.Close()

	got := FetchMTPRate(srv.URL+"/metrics", 5)
	if !got.Available {
		t.Fatalf("Available = false (reason %q detail %q), want true", got.Reason, got.Detail)
	}
	if got.Rate < 0.7499 || got.Rate > 0.7501 {
		t.Errorf("Rate = %v, want 0.75", got.Rate)
	}
}

// UC-0016: an endpoint answering a non-200 status is reported as a fetch
// failure with the status kept for diagnostics.
func TestFEATURE546FetchMTPRateHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	got := FetchMTPRate(srv.URL, 5)
	if got.Available || got.Reason != MTPReasonFetchFailed {
		t.Errorf("got %+v, want unavailable with reason %q", got, MTPReasonFetchFailed)
	}
	if got.Detail == "" {
		t.Error("Detail is empty, want the HTTP status for diagnosis")
	}
}

// A rate above 1 (an engine over-counting accepted tokens) is clamped so the
// UI can never show "120%".
func TestFEATURE546ParseMTPStatusClampsAboveOne(t *testing.T) {
	body := "vllm:spec_decode_num_accepted_tokens_total 300\n" +
		"vllm:spec_decode_num_draft_tokens_total 100\n"
	got := ParseMTPStatus(body)
	if !got.Available || got.Rate != 1 {
		t.Errorf("got %+v, want available with rate clamped to 1", got)
	}
}
