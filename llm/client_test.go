package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTruncateBody covers the truncateBody diagnostic helper.
func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantTail string // substring that must appear in result
		wantLen  int    // upper bound for result length when truncated
	}{
		{
			name:     "short body unchanged",
			body:     `{"model":"test"}`,
			wantTail: `{"model":"test"}`,
		},
		{
			name:     "exactly at limit unchanged",
			body:     strings.Repeat("a", 1000),
			wantTail: strings.Repeat("a", 1000),
		},
		{
			name:     "long body truncated with marker",
			body:     strings.Repeat("x", 5000),
			wantTail: "[truncated 4000 bytes]",
			wantLen:  1000 + 50, // 1000 bytes + truncation marker text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody([]byte(tt.body))
			if !strings.Contains(got, tt.wantTail) {
				t.Errorf("truncateBody(%q) = %q, want contains %q", tt.body, got, tt.wantTail)
			}
			if tt.wantLen > 0 && len(got) > tt.wantLen {
				t.Errorf("truncateBody result length %d exceeds limit %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestRequestSummary covers the requestSummary compact error-message helper.
func TestRequestSummary(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantSubs []string // substrings that must all appear in result
	}{
		{
			name:     "full request",
			body:     `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function"}],"stream":true,"extra_body":{}}`,
			wantSubs: []string{"model=gpt-4o", "messages=1", "tools=1", "stream=true", "extra_body=present"},
		},
		{
			name:     "minimal request",
			body:     `{"model":"m","messages":[]}`,
			wantSubs: []string{"model=m", "messages=0"},
		},
		{
			name:     "no stream field",
			body:     `{"model":"m","messages":[{}]}`,
			wantSubs: []string{"model=m", "messages=1"},
		},
		{
			name:     "unparseable body",
			body:     `not json`,
			wantSubs: []string{"unparseable body"},
		},
		{
			name:     "empty body",
			body:     ``,
			wantSubs: []string{"unparseable body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := requestSummary([]byte(tt.body))
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("requestSummary(%q) = %q, want contains %q", tt.body, got, sub)
				}
			}
			// The summary must behave a valid single-line string: no raw newlines
			// (it is embedded in error messages fed back to the LLM).
			if strings.ContainsAny(got, "\n\r") {
				t.Errorf("requestSummary(%q) = %q, must not contain newline", tt.body, got)
			}
		})
	}
}

// TestRequestSummaryDoesNotEchoContent guards against accidentally leaking full
// message contents into the error summary sent back to the LLM.
func TestRequestSummaryDoesNotEchoContent(t *testing.T) {
	secret := "SENSITIVE_SYSTEM_PROMPT_12345"
	body, err := json.Marshal(map[string]interface{}{
		"model": "m",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": secret},
		},
	})
	if err != nil {
		t.Fatalf("cannot marshal test body: %v", err)
	}
	got := requestSummary(body)
	if strings.Contains(got, secret) {
		t.Errorf("requestSummary leaked message content: %q", got)
	}
}
