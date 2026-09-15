// Tests for the Web UI model JSON payload (FEATURE-526).
package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestModelWebJSONThinkingFields verifies the thinking-depth fields added in
// FEATURE-526: thinking_enabled follows "nil means not explicitly disabled"
// and reasoning_effort is dereferenced to a plain string. The JSON key names
// are asserted as well, since the Web UI reads them verbatim (UC-0011).
func TestModelWebJSONThinkingFields(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	strPtr := func(v string) *string { return &v }

	cfg := &config.Config{Models: []*config.ModelConfig{
		{ID: "unset", Capabilities: config.ModelCapability{Thinking: true}},
		{
			ID:              "off",
			Capabilities:    config.ModelCapability{Thinking: true},
			ThinkingEnabled: boolPtr(false),
			ReasoningEffort: strPtr("high"),
		},
		{
			ID:              "on",
			Capabilities:    config.ModelCapability{Thinking: true},
			ThinkingEnabled: boolPtr(true),
			ReasoningEffort: strPtr("xhigh"),
		},
		{ID: "noreason", Capabilities: config.ModelCapability{Thinking: false}, ThinkingEnabled: boolPtr(true)},
	}}

	models, _ := (&ModelHandler{cfg: cfg}).ModelWebJSON()
	if len(models) != len(cfg.Models) {
		t.Fatalf("models = %d, want %d", len(models), len(cfg.Models))
	}
	byID := make(map[string]WebModel, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}

	cases := []struct {
		id          string
		wantEnabled bool
		wantEffort  string
		wantCapable bool
	}{
		{"unset", true, "", true},
		{"off", false, "high", true},
		{"on", true, "xhigh", true},
		{"noreason", true, "", false},
	}
	for _, c := range cases {
		m, ok := byID[c.id]
		if !ok {
			t.Fatalf("model %q missing from ModelWebJSON output", c.id)
		}
		if m.ThinkingEnabled != c.wantEnabled {
			t.Errorf("%s: thinking_enabled = %v, want %v", c.id, m.ThinkingEnabled, c.wantEnabled)
		}
		if m.ReasoningEffort != c.wantEffort {
			t.Errorf("%s: reasoning_effort = %q, want %q", c.id, m.ReasoningEffort, c.wantEffort)
		}
		// The capability mark keeps its own meaning (FEATURE-516).
		if m.Thinking != c.wantCapable {
			t.Errorf("%s: thinking = %v, want %v", c.id, m.Thinking, c.wantCapable)
		}
	}

	raw, err := json.Marshal(byID["on"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(raw)
	for _, want := range []string{`"thinking_enabled":true`, `"reasoning_effort":"xhigh"`} {
		if !strings.Contains(js, want) {
			t.Errorf("JSON %s does not contain %s", js, want)
		}
	}
}
