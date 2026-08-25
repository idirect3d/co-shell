// Tests for the structured web model wizard (FEATURE-429).
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// newWebWizardHandler builds a ModelHandler with a workspace-backed config so
// saveModel can persist, plus a fresh agent on a temp bbolt store.
func newWebWizardHandler(t *testing.T) *ModelHandler {
	t.Helper()
	ws, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	cfg, _, err := config.LoadFromFile(filepath.Join(ws.Root(), "config.json"), ws)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	boltStore, err := store.NewStore(ws)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = boltStore.Close() })
	ag := agent.New(nil, nil, store.NewDualStore(boltStore, nil), "")
	ag.SetIO(&wizardTestIO{})
	return NewModelHandler(cfg, ag)
}

// TestWebWizardStartAdd verifies WebWizardStart("add") returns the template
// step with the built-in template options (UC-001).
func TestWebWizardStartAdd(t *testing.T) {
	h := newWebWizardHandler(t)
	step, data, err := h.WebWizardStart("add", "")
	if err != nil {
		t.Fatalf("WebWizardStart: %v", err)
	}
	if step.Step != WebWizardTemplate {
		t.Fatalf("first step = %q, want %q", step.Step, WebWizardTemplate)
	}
	if !step.IsFirst {
		t.Errorf("first step should be IsFirst")
	}
	if len(step.Fields) != 1 || step.Fields[0].Key != "template_id" {
		t.Fatalf("template step fields = %+v, want one template_id field", step.Fields)
	}
	if len(step.Fields[0].Options) == 0 {
		t.Errorf("template step should have template options")
	}
	if data.Mode != "add" {
		t.Errorf("data.Mode = %q, want add", data.Mode)
	}
}

// TestWebWizardStartEdit verifies WebWizardStart("edit") pre-fills the existing
// model's values (UC-002).
func TestWebWizardStartEdit(t *testing.T) {
	h := newWebWizardHandler(t)
	// Seed an existing model.
	model := &config.ModelConfig{
		ID: "tpl-m1", Name: "Tpl (m1)", Provider: "tpl", Endpoint: "https://x",
		Model: "m1", APIKey: "k", Priority: 10, Enabled: true, TemplateID: "tpl",
		Capabilities: config.ModelCapability{Vision: true, ToolCall: true},
		MaxModelLen:  65536,
	}
	h.cfg.Models = append(h.cfg.Models, model)

	step, data, err := h.WebWizardStart("edit", "tpl-m1")
	if err != nil {
		t.Fatalf("WebWizardStart(edit): %v", err)
	}
	if step.Step != WebWizardTemplate {
		t.Fatalf("first step = %q, want template", step.Step)
	}
	if data.Mode != "edit" || data.EditID != "tpl-m1" {
		t.Errorf("data mode/edit_id = %q/%q, want edit/tpl-m1", data.Mode, data.EditID)
	}
	if data.Endpoint != "https://x" || data.ModelName != "m1" || data.Priority != 10 {
		t.Errorf("prefill mismatch: endpoint=%q model=%q priority=%d", data.Endpoint, data.ModelName, data.Priority)
	}
	if !data.Vision || !data.ToolCall || data.Thinking {
		t.Errorf("prefill caps mismatch: vision=%v toolcall=%v thinking=%v", data.Vision, data.ToolCall, data.Thinking)
	}
}

// TestWebWizardNext verifies advancing from template to endpoint (UC-003).
func TestWebWizardNext(t *testing.T) {
	h := newWebWizardHandler(t)
	_, data, err := h.WebWizardStart("add", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	data.TemplateID = "deepseek"
	next, err := h.WebWizardNext(data, WebWizardTemplate)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next.Step != WebWizardEndpoint {
		t.Fatalf("next step = %q, want endpoint", next.Step)
	}
	if len(next.Fields) != 1 || next.Fields[0].Key != "endpoint" {
		t.Fatalf("endpoint fields = %+v", next.Fields)
	}
}

// TestWebWizardPrev verifies going back from endpoint to template (UC-004).
func TestWebWizardPrev(t *testing.T) {
	h := newWebWizardHandler(t)
	_, data, err := h.WebWizardStart("add", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	prev, err := h.WebWizardPrev(data, WebWizardEndpoint)
	if err != nil {
		t.Fatalf("prev: %v", err)
	}
	if prev.Step != WebWizardTemplate {
		t.Fatalf("prev step = %q, want template", prev.Step)
	}
}

// TestWebWizardSubmitAdd verifies submitting a completed add wizard saves the
// model (UC-005).
func TestWebWizardSubmitAdd(t *testing.T) {
	h := newWebWizardHandler(t)
	data := &WebWizardData{
		Mode: "add", TemplateID: "deepseek-official", Endpoint: "https://api.deepseek.com",
		APIKey: "k", ModelName: "deepseek-chat", ModelID: "deepseek-chat",
		Priority: 10, MaxModelLen: 65536, Enabled: true,
		Vision: true, ToolCall: true,
	}
	result, err := h.WebWizardSubmit(data)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result == "" {
		t.Errorf("submit result should be non-empty")
	}
	found := false
	for _, m := range h.cfg.Models {
		if m.ID == "deepseek-chat" {
			found = true
			if m.Endpoint != "https://api.deepseek.com" || m.Model != "deepseek-chat" {
				t.Errorf("saved model mismatch: %+v", m)
			}
		}
	}
	if !found {
		t.Errorf("model deepseek-chat not saved")
	}
}

// TestWebWizardSubmitEdit verifies submitting an edit wizard updates the model
// (UC-006).
func TestWebWizardSubmitEdit(t *testing.T) {
	h := newWebWizardHandler(t)
	model := &config.ModelConfig{
		ID: "tpl-m1", Name: "Tpl (m1)", Provider: "tpl", Endpoint: "https://old",
		Model: "m1", APIKey: "k", Priority: 10, Enabled: true, TemplateID: "tpl",
	}
	h.cfg.Models = append(h.cfg.Models, model)

	data := &WebWizardData{
		Mode: "edit", EditID: "tpl-m1", TemplateID: "deepseek-official", Endpoint: "https://new",
		APIKey: "k2", ModelName: "m2", ModelID: "tpl-m1", Priority: 20,
		MaxModelLen: 128000, Enabled: false, Thinking: true,
	}
	if _, err := h.WebWizardSubmit(data); err != nil {
		t.Fatalf("submit edit: %v", err)
	}
	if model.Endpoint != "https://new" || model.Model != "m2" || model.Priority != 20 {
		t.Errorf("edit not applied: %+v", model)
	}
	if model.Enabled || !model.Capabilities.Thinking {
		t.Errorf("edit enabled/thinking not applied: %+v", model)
	}
}

// TestWebWizardModelNameStep verifies the model-name step returns suggestions
// (UC-007). Network fetch may fail in CI, so it is gated by -short.
func TestWebWizardModelNameStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent test in -short mode")
	}
	h := newWebWizardHandler(t)
	data := &WebWizardData{
		Mode: "add", TemplateID: "deepseek", Endpoint: "https://api.deepseek.com",
		APIKey: "k", ModelName: "",
	}
	step, err := h.webWizardStepData(data, WebWizardModelName)
	if err != nil {
		t.Fatalf("model name step: %v", err)
	}
	if step.Step != WebWizardModelName {
		t.Fatalf("step = %q, want model_name", step.Step)
	}
	if len(step.Fields) != 1 || step.Fields[0].Key != "model_name" {
		t.Fatalf("model name fields = %+v", step.Fields)
	}
}

// TestWebWizardCapabilitiesStep verifies the capabilities step returns the
// three capability checkboxes (UC-008). Network detection may fail in CI, so it
// is gated by -short.
func TestWebWizardCapabilitiesStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent test in -short mode")
	}
	h := newWebWizardHandler(t)
	data := &WebWizardData{
		Mode: "add", TemplateID: "deepseek", Endpoint: "https://api.deepseek.com",
		APIKey: "k", ModelName: "deepseek-chat",
	}
	step, err := h.webWizardStepData(data, WebWizardCapabilities)
	if err != nil {
		t.Fatalf("capabilities step: %v", err)
	}
	if step.Step != WebWizardCapabilities {
		t.Fatalf("step = %q, want capabilities", step.Step)
	}
	if len(step.Fields) != 3 {
		t.Fatalf("capabilities fields = %d, want 3", len(step.Fields))
	}
	keys := map[string]bool{}
	for _, f := range step.Fields {
		keys[f.Key] = true
	}
	for _, k := range []string{"vision", "tool_call", "thinking"} {
		if !keys[k] {
			t.Errorf("capabilities field %q missing", k)
		}
	}
}
