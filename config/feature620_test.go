package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// FEATURE-620 P2 acceptance tests for the template changes
// (use-case/FEATURE-620/FEATURE-620-UC-0003.md).

// P2-005: the new self-hosted template exists with the fields the wizard needs.
func TestFEATURE620VLLMSelfHostedTemplate(t *testing.T) {
	tmpl := GetDefaultModelManager().GetTemplate("vllm-self-hosted")
	if tmpl == nil {
		t.Fatal("template vllm-self-hosted not found")
	}
	if tmpl.Provider != "openai-compatible" {
		t.Errorf("Provider = %q, want %q", tmpl.Provider, "openai-compatible")
	}
	if tmpl.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty so the user must supply one", tmpl.Endpoint)
	}
	if tmpl.Capabilities != (ModelCapability{}) {
		t.Errorf("Capabilities = %+v, want all false (probed instead of assumed)", tmpl.Capabilities)
	}
	for _, want := range []string{"--reasoning-parser", "--tool-call-parser"} {
		if !strings.Contains(tmpl.Description, want) {
			t.Errorf("Description %q does not mention %q", tmpl.Description, want)
		}
	}
	if tmpl.Name == "" {
		t.Error("Name is empty")
	}

	// It must be offered by the wizard, which renders GetAllTemplates().
	found := false
	for _, t2 := range GetDefaultModelManager().GetAllTemplates() {
		if t2.ID == "vllm-self-hosted" {
			found = true
			break
		}
	}
	if !found {
		t.Error("vllm-self-hosted is missing from GetAllTemplates()")
	}
}

// P2-007: adding the new template must not disturb the existing ones.
func TestFEATURE620ExistingTemplatesUnchanged(t *testing.T) {
	// Key fields of every pre-FEATURE-620 template.
	want := map[string]struct {
		provider string
		endpoint string
		priority int
	}{
		"deepseek-official":        {"deepseek", "https://api.deepseek.com", 100},
		"qwen-official":            {"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1", 90},
		"qwen3.8":                  {"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1", 89},
		"xiaomi-mimo":              {"xiaomi", "https://api.xiaomimimo.com/v1", 80},
		"kimi-official":            {"kimi", "https://api.moonshot.cn", 85},
		"zhipu-glm":                {"zhipu", "https://open.bigmodel.cn/api/paas/v4/", 75},
		"openai-official":          {"openai", "https://api.openai.com/v1", 95},
		"lmstudio-local":           {"lmstudio", "http://localhost:1234/v1", 55},
		"ollama-local":             {"ollama", "http://localhost:11434/v1", 50},
		"custom-openai-compatible": {"openai-compatible", "", 60},
	}

	manager := GetDefaultModelManager()
	for id, tc := range want {
		tmpl := manager.GetTemplate(id)
		if tmpl == nil {
			t.Errorf("template %q disappeared", id)
			continue
		}
		if tmpl.Provider != tc.provider {
			t.Errorf("%s: Provider = %q, want %q", id, tmpl.Provider, tc.provider)
		}
		if tmpl.Endpoint != tc.endpoint {
			t.Errorf("%s: Endpoint = %q, want %q", id, tmpl.Endpoint, tc.endpoint)
		}
		if tmpl.Priority != tc.priority {
			t.Errorf("%s: Priority = %d, want %d", id, tmpl.Priority, tc.priority)
		}
		// FEATURE-620 hints must stay empty on the vendor templates: their
		// behaviour is provider-defined and must not change.
		if tmpl.ThinkingProtocol != "" || tmpl.ReasoningField != "" ||
			tmpl.ToolCallStyle != "" || len(tmpl.ReasoningTags) != 0 {
			t.Errorf("%s: FEATURE-620 hints must be empty, got %+v", id, tmpl)
		}
	}
}

// The template's engine-behaviour hints must reach a model created from it.
func TestFEATURE620TemplateHintsCarryToModel(t *testing.T) {
	manager := GetDefaultModelManager()
	tmpl := manager.GetTemplate("custom-openai-compatible")
	if tmpl == nil {
		t.Fatal("custom-openai-compatible template not found")
	}
	original := *tmpl

	tmpl.ThinkingProtocol = "qwen"
	tmpl.ReasoningField = "reasoning"
	tmpl.ToolCallStyle = "native"
	tmpl.ReasoningTags = []string{"thinking"}
	t.Cleanup(func() {
		// Restore the shared built-in template for the other tests in this
		// package, which all read the same manager.
		restored := original
		*tmpl = restored
	})

	model, err := manager.CreateModelFromTemplate("custom-openai-compatible", "test-hints", "test", "")
	if err != nil {
		t.Fatalf("CreateModelFromTemplate: %v", err)
	}
	defer func() {
		_ = manager.RemoveModel(model.ID)
	}()

	if model.ThinkingProtocol != "qwen" {
		t.Errorf("ThinkingProtocol = %q, want %q", model.ThinkingProtocol, "qwen")
	}
	if model.ReasoningField != "reasoning" {
		t.Errorf("ReasoningField = %q, want %q", model.ReasoningField, "reasoning")
	}
	if model.ToolCallStyle != "native" {
		t.Errorf("ToolCallStyle = %q, want %q", model.ToolCallStyle, "native")
	}
	if len(model.ReasoningTags) != 1 || model.ReasoningTags[0] != "thinking" {
		t.Errorf("ReasoningTags = %v, want [thinking]", model.ReasoningTags)
	}
}

// P2-001: a model without FEATURE-620 hints keeps the legacy behaviour.
func TestFEATURE620ModelConfigNewFieldsAreOptional(t *testing.T) {
	model := &ModelConfig{
		ID:       "m1",
		Provider: "deepseek",
		Model:    "deepseek-chat",
	}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"thinking_protocol", "reasoning_field", "tool_call_style", "reasoning_tags"} {
		if strings.Contains(string(data), key) {
			t.Errorf("empty %q must be omitted from JSON, got %s", key, data)
		}
	}
}
