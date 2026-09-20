// FEATURE-546: reads the speculative-decoding (MTP) acceptance rate from a
// vLLM-style Prometheus /metrics endpoint.
//
// The rate is the cumulative ratio of accepted to drafted tokens:
//
//	vllm:spec_decode_num_accepted_tokens_total / vllm:spec_decode_num_draft_tokens_total
//
// Both are counters that may carry labels (model_name, ...), so every series of
// a metric name is summed. Unavailability is reported as a stable reason key
// rather than a translated sentence, so the Web UI can localise the message and
// the suggestion itself (a 0% or blank readout would be misleading).
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

package llm

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Reason keys for an unavailable MTP metric. The front end maps these to a
// localized message plus a suggestion.
const (
	// MTPReasonNotConfigured: metrics_endpoint is empty in the model config.
	MTPReasonNotConfigured = "not_configured"
	// MTPReasonFetchFailed: the endpoint could not be reached or returned an error.
	MTPReasonFetchFailed = "fetch_failed"
	// MTPReasonNoSpecMetrics: metrics were fetched but contain no spec_decode
	// counters, i.e. the engine was started without --speculative-config.
	MTPReasonNoSpecMetrics = "no_spec_metrics"
	// MTPReasonNoDraftTokens: the counters exist but nothing has been drafted
	// yet, so the ratio is undefined (engine idle since it started).
	MTPReasonNoDraftTokens = "no_draft_tokens"
)

// MTPMetricNames are the two vLLM counters the acceptance rate is built from.
const (
	mtpAcceptedMetric = "vllm:spec_decode_num_accepted_tokens_total"
	mtpDraftMetric    = "vllm:spec_decode_num_draft_tokens_total"
)

// MTPStatus is the outcome of one MTP probe, ready to be serialised to the
// browser. Rate is meaningful only when Available is true.
type MTPStatus struct {
	Available bool    `json:"available"`
	Rate      float64 `json:"rate"`
	Reason    string  `json:"reason,omitempty"`
	// Detail carries the raw technical cause (HTTP status, transport error).
	// It is diagnostic text, not user-facing prose.
	Detail string `json:"detail,omitempty"`
}

// mtpUnavailable builds an unavailable status for the given reason key.
func mtpUnavailable(reason, detail string) MTPStatus {
	return MTPStatus{Available: false, Reason: reason, Detail: detail}
}

// FetchMTPRate probes metricsURL and computes the acceptance rate. An empty
// metricsURL short-circuits without any network access. timeoutSecs <= 0 falls
// back to 3 seconds so a dead host never stalls the model list.
func FetchMTPRate(metricsURL string, timeoutSecs int) MTPStatus {
	if strings.TrimSpace(metricsURL) == "" {
		return mtpUnavailable(MTPReasonNotConfigured, "")
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return mtpUnavailable(MTPReasonFetchFailed, err.Error())
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return mtpUnavailable(MTPReasonFetchFailed, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return mtpUnavailable(MTPReasonFetchFailed, "HTTP "+resp.Status)
	}
	// A vLLM metrics page is a few hundred KB at most; cap the read so a
	// misconfigured URL pointing at a large download cannot exhaust memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return mtpUnavailable(MTPReasonFetchFailed, err.Error())
	}
	return ParseMTPStatus(string(body))
}

// ParseMTPStatus computes the acceptance rate from Prometheus exposition text.
// It is separated from the HTTP layer so it can be unit tested directly.
func ParseMTPStatus(body string) MTPStatus {
	accepted, hasAccepted := sumMetric(body, mtpAcceptedMetric)
	draft, hasDraft := sumMetric(body, mtpDraftMetric)
	if !hasAccepted || !hasDraft {
		return mtpUnavailable(MTPReasonNoSpecMetrics, "")
	}
	if draft <= 0 {
		return mtpUnavailable(MTPReasonNoDraftTokens, "")
	}
	rate := accepted / draft
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return MTPStatus{Available: true, Rate: rate}
}

// sumMetric adds up the sample values of every series of the given metric
// name. The second return reports whether at least one series was found.
func sumMetric(body, name string) (float64, bool) {
	var total float64
	found := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// A sample line is "<identifier> <value>", where the identifier may
		// carry a {label="..."} block. Splitting on the last space keeps label
		// values that themselves contain spaces intact.
		sp := strings.LastIndexByte(line, ' ')
		if sp < 0 {
			continue
		}
		ident, valueStr := line[:sp], line[sp+1:]
		if brace := strings.IndexByte(ident, '{'); brace >= 0 {
			ident = ident[:brace]
		}
		if ident != name {
			continue
		}
		v, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}
		total += v
		found = true
	}
	return total, found
}
