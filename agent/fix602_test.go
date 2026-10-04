// FEATURE-602 unit tests: automatic model degradation (agent + config level).
//
// Author: L.Shuang
// Created: 2026-10-04
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// TestFeature602FallbackableMatrix pins the trigger set agreed with the user:
// connection (network), service (5xx / 429) and model-internal (auth,
// context-too-long) failures may be fixed by another model; a request-shape
// problem (context-format) and an unclassifiable failure may not.
func TestFeature602FallbackableMatrix(t *testing.T) {
	cases := []struct {
		kind LLMErrKind
		want bool
	}{
		{LLMErrNetwork, true},
		{LLMErrServer, true},
		{LLMErrRateLimit, true},
		{LLMErrAuth, true},
		{LLMErrContextTooLong, true},
		{LLMErrContextFormat, false},
		{LLMErrUnknown, false},
	}
	for _, c := range cases {
		if got := c.kind.Fallbackable(); got != c.want {
			t.Errorf("%q.Fallbackable() = %v, want %v", c.kind, got, c.want)
		}
	}
}

// TestFeature602ClassifiedErrorsAreFallbackable checks the taxonomy end-to-end
// for the API-level failures the user listed (5xx, quota/permission/model-not-
// found, rate limit, context too long).
func TestFeature602ClassifiedErrorsAreFallbackable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want LLMErrKind
	}{
		{"server 503", &llm.OpenAIError{StatusCode: 503, Message: "service unavailable"}, LLMErrServer},
		{"rate limit 429", &llm.OpenAIError{StatusCode: 429, Message: "too many requests"}, LLMErrRateLimit},
		{"model not found 404", &llm.OpenAIError{StatusCode: 404, Message: "model does not exist"}, LLMErrAuth},
		{"insufficient quota", &llm.OpenAIError{StatusCode: 400, Message: "insufficient quota"}, LLMErrAuth},
		{"permission denied", errors.New("permission denied"), LLMErrAuth},
		{"max context length", &llm.OpenAIError{StatusCode: 400, Message: "maximum context length is 65536 tokens"}, LLMErrContextTooLong},
	}
	for _, c := range cases {
		kind := ClassifyLLMError(c.err)
		if kind != c.want {
			t.Errorf("%s: ClassifyLLMError = %q, want %q", c.name, kind, c.want)
			continue
		}
		if !kind.Fallbackable() {
			t.Errorf("%s: %q must be fallbackable", c.name, kind)
		}
	}
}

// TestFeature602ToolCallParseErrorIsContextFormat locks the classification the
// user reported on 2026-10-04 12:44: a malformed tool call produced by the model
// ("unbalanced closing brace/square bracket") is a request-shape problem — it
// must remove the offending assistant message (FIX-146), not stay "unknown"
// (which left the context untouched) and not degrade to another model.
func TestFeature602ToolCallParseErrorIsContextFormat(t *testing.T) {
	err := errors.New("tool call stream parse error (tool=replace_in_file): unbalanced closing brace/square bracket")
	kind := ClassifyLLMError(err)
	if kind != LLMErrContextFormat {
		t.Fatalf("ClassifyLLMError = %q, want %q", kind, LLMErrContextFormat)
	}
	if !kind.AllowsContextRemoval() {
		t.Error("a tool-call parse error must allow context removal (FIX-146 path)")
	}
	if kind.Fallbackable() {
		t.Error("a tool-call parse error must not trigger model degradation")
	}
	// The XML flavour ("unknown tool tag <x>") is the same family.
	if got := ClassifyLLMError(errors.New("tool call stream parse error: unknown tool tag <nope>")); got != LLMErrContextFormat {
		t.Errorf("unknown tool tag: ClassifyLLMError = %q, want %q", got, LLMErrContextFormat)
	}
}

// buildFallbackAgent wires a model manager with the given models.
func buildFallbackAgent(t *testing.T, cfg *config.Config) *Agent {
	t.Helper()
	mm := &config.ModelManager{}
	for _, m := range cfg.Models {
		if err := mm.AddModel(m); err != nil {
			t.Fatalf("AddModel(%s): %v", m.ID, err)
		}
	}
	return &Agent{cfg: cfg, modelManager: mm}
}

func model(id string, priority int, enabled, vision bool) *config.ModelConfig {
	return &config.ModelConfig{
		ID: id, Priority: priority, Enabled: enabled,
		Capabilities: config.ModelCapability{Vision: vision},
	}
}

// TestFeature602DegradesDownPriorityOrder walks A(P90) -> B(P80) -> C(P70) and
// then exhausts: the strategy only ever moves to a lower-ranked model, never
// re-tries a model and reports exhaustion as nil.
func TestFeature602DegradesDownPriorityOrder(t *testing.T) {
	cfg := &config.Config{Models: []*config.ModelConfig{
		model("A", 90, true, false), model("B", 80, true, false), model("C", 70, true, false),
	}}
	a := buildFallbackAgent(t, cfg)

	for _, want := range []string{"B", "C"} {
		next := a.nextFallbackModel()
		if next == nil {
			t.Fatalf("nextFallbackModel() = nil, want %s", want)
		}
		if next.ID != want {
			t.Fatalf("nextFallbackModel() = %s, want %s", next.ID, want)
		}
		a.applyFallbackModel(next)
	}
	if next := a.nextFallbackModel(); next != nil {
		t.Fatalf("nextFallbackModel() = %s, want nil after C", next.ID)
	}
}

// TestFeature602NeverJumpsBackUp pins the "degrade only" rule: when the turn is
// bound to B (P80), the next candidate is C (P70), not the higher-ranked A.
func TestFeature602NeverJumpsBackUp(t *testing.T) {
	bID := "B"
	cfg := &config.Config{
		LLM:    config.LLMConfig{WorkMode: "act"},
		Models: []*config.ModelConfig{model("A", 90, true, false), model("B", 80, true, false), model("C", 70, true, false)},
		WorkModes: []config.WorkMode{
			{Name: "act", ModelID: &bID},
		},
	}
	a := buildFallbackAgent(t, cfg)
	next := a.nextFallbackModel()
	if next == nil || next.ID != "C" {
		t.Fatalf("nextFallbackModel() = %v, want C (never back up to A)", next)
	}
}

// TestFeature602SkipsDisabledAndNonVision covers the candidate filters: a
// disabled model and (in a vision turn) a non-vision model are never chosen.
func TestFeature602SkipsDisabledAndNonVision(t *testing.T) {
	visionCfg := &config.Config{Models: []*config.ModelConfig{
		model("A", 90, true, true), model("B", 80, false, true), model("C", 70, true, true),
	}}
	a := buildFallbackAgent(t, visionCfg)
	a.imagePaths = []string{"/tmp/fake.png"}
	if next := a.nextFallbackModel(); next == nil || next.ID != "C" {
		t.Fatalf("vision: nextFallbackModel() = %v, want C (B disabled)", next)
	}

	mixedCfg := &config.Config{Models: []*config.ModelConfig{
		model("A", 90, true, true), model("B", 80, true, false), model("C", 70, true, true),
	}}
	b := buildFallbackAgent(t, mixedCfg)
	b.imagePaths = []string{"/tmp/fake.png"}
	if next := b.nextFallbackModel(); next == nil || next.ID != "C" {
		t.Fatalf("vision: nextFallbackModel() = %v, want C (B has no vision)", next)
	}
}

// TestFeature602FallbackLastsTheTurn verifies the degraded model is used for the
// rest of the turn and cleared when a new turn starts.
func TestFeature602FallbackLastsTheTurn(t *testing.T) {
	cfg := &config.Config{Models: []*config.ModelConfig{
		model("A", 90, true, false), model("B", 80, true, false),
	}}
	a := buildFallbackAgent(t, cfg)
	if got := a.getModelIDForCall(); got != "" {
		t.Fatalf("getModelIDForCall() = %q, want empty (global selection)", got)
	}
	if got := a.selectModelForCall().ID; got != "A" {
		t.Fatalf("initial selection = %q, want A", got)
	}
	a.applyFallbackModel(cfg.Models[1])
	if got := a.getModelIDForCall(); got != "B" {
		t.Fatalf("after degrade getModelIDForCall() = %q, want B", got)
	}
	if got := a.selectModelForCall().ID; got != "B" {
		t.Fatalf("after degrade selection = %q, want B", got)
	}
	a.resetModelFallback()
	if got := a.selectModelForCall().ID; got != "A" {
		t.Fatalf("after reset selection = %q, want A", got)
	}
}

// TestFeature602RetryLimitIsConfigurable covers UC-0004: the in-place retry
// budget follows cfg.LLM.LLMErrRetryLimit (default 3 when unset).
func TestFeature602RetryLimitIsConfigurable(t *testing.T) {
	if got := (&config.LLMConfig{}).LLMErrRetryLimitOrDefault(); got != 3 {
		t.Errorf("default retry limit = %d, want 3", got)
	}
	if got := config.DefaultConfig().LLM.LLMErrRetryLimit; got != 3 {
		t.Errorf("DefaultConfig retry limit = %d, want 3", got)
	}

	a := &Agent{cfg: &config.Config{LLM: config.LLMConfig{LLMErrRetryLimit: 1}}}
	if !a.noteLLMErrRetryAndCheckLimit() {
		t.Fatal("retry 1 must be allowed with a limit of 1")
	}
	if a.noteLLMErrRetryAndCheckLimit() {
		t.Fatal("retry 2 must be refused with a limit of 1")
	}
	if got := a.effectiveLLMRetryLimit(); got != 1 {
		t.Errorf("effectiveLLMRetryLimit() = %d, want 1", got)
	}

	fresh := &Agent{cfg: &config.Config{}}
	if got := fresh.effectiveLLMRetryLimit(); got != 3 {
		t.Errorf("unset limit must fall back to 3, got %d", got)
	}
}

// TestFeature602AutoSelectDefaultsOffAndPersists covers UC-0001: the switch is
// off by default (including for an old config without the key) and an explicit
// "on" survives a save/reload round-trip.
func TestFeature602AutoSelectDefaultsOffAndPersists(t *testing.T) {
	if config.DefaultConfig().LLM.AutoSelectEnabled() {
		t.Error("auto-model-select must default off")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"llm":{"temperature":0.5}}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.LLM.AutoSelectEnabled() {
		t.Error("a config without the key must keep the switch off")
	}

	cfg.LLM.AutoModelSelect = true
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, _, err := config.LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.LLM.AutoSelectEnabled() {
		t.Error("auto_model_select=true did not survive the round-trip")
	}
}
