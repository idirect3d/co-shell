// Author: L.Shuang
// Created: 2026-05-07
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ModelCapability defines the capabilities a model supports.
type ModelCapability struct {
	Vision     bool `json:"vision"`
	ToolCall   bool `json:"tool_call"`
	Thinking   bool `json:"thinking"`
	Multimodal bool `json:"multimodal"`
}

// ModelTemplate defines a reusable template for model configuration.
type ModelTemplate struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Provider     string          `json:"provider"`
	Endpoint     string          `json:"endpoint"`
	DefaultModel string          `json:"default_model"`
	Models       []string        `json:"models"`
	APIKeyURL    string          `json:"api_key_url,omitempty"`
	Capabilities ModelCapability `json:"capabilities"`
	Priority     int             `json:"priority"`
	Description  string          `json:"description,omitempty"`
	// Fields that can be overridden by user configuration
	CustomEndpoint       string   `json:"custom_endpoint,omitempty"`
	CustomModels         []string `json:"custom_models,omitempty"`
	OverrideCapabilities *bool    `json:"override_capabilities,omitempty"`
	// DefaultMaxModelLen is the default maximum context length (in tokens) for models
	// created from this template. Used as a fallback when the API doesn't return this value.
	// 0 means unknown.
	DefaultMaxModelLen int `json:"default_max_model_len,omitempty"`

	// DefaultParams defines the default custom parameters for models created from this template.
	// These are provider-specific parameters that get injected into the LLM request body.
	// The value can be any JSON value. A string value of "None" (case-sensitive) means
	// the parameter should NOT be sent in the request body (removed if present).
	// For example:
	//   DeepSeek: {"thinking": {"type": "enabled"}}
	//   Qwen:     {"extra_body": {"chat_template_kwargs": {"enable_thinking": true}}}
	//   OpenAI:   {"reasoning_effort": "high"}
	DefaultParams map[string]interface{} `json:"default_params,omitempty"`
}

// ModelConfig represents a configured model instance with its settings.
type ModelConfig struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Provider     string                 `json:"provider"`
	Endpoint     string                 `json:"endpoint"`
	Model        string                 `json:"model"`
	APIKey       string                 `json:"api_key,omitempty"`
	Priority     int                    `json:"priority"`
	Capabilities ModelCapability        `json:"capabilities"`
	Enabled      bool                   `json:"enabled"`
	TemplateID   string                 `json:"template_id,omitempty"`
	CustomParams map[string]interface{} `json:"custom_params,omitempty"`

	// MaxModelLen is the maximum context length (in tokens) supported by the model.
	// This value is automatically detected from the API when listing models.
	// A value of 0 means unknown or not yet detected.
	MaxModelLen int `json:"max_model_len,omitempty"`

	// Model-level LLM parameters (override global cfg.LLM settings when set)
	// A value of nil/0 means "use global default from cfg.LLM"
	Temperature       *float64 `json:"temperature,omitempty"`
	MaxTokens         *int     `json:"max_tokens,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	TopK              *int     `json:"top_k,omitempty"`
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
	ThinkingEnabled   *bool    `json:"thinking_enabled,omitempty"`
	ReasoningEffort   *string  `json:"reasoning_effort,omitempty"`
}

// ModelManager manages model templates and configurations.
type ModelManager struct {
	templates   map[string]*ModelTemplate
	models      []*ModelConfig
	templatesMu sync.RWMutex
	modelsMu    sync.RWMutex
}

var (
	defaultManager *ModelManager
	once           sync.Once
)

// GetDefaultModelManager returns the singleton ModelManager instance.
func GetDefaultModelManager() *ModelManager {
	once.Do(func() {
		defaultManager = &ModelManager{
			templates: make(map[string]*ModelTemplate),
			models:    make([]*ModelConfig, 0),
		}
		defaultManager.initBuiltInTemplates()
	})
	return defaultManager
}

// initBuiltInTemplates initializes the built-in model templates.
func (m *ModelManager) initBuiltInTemplates() {
	templates := []ModelTemplate{
		{
			ID:           "deepseek-official",
			Name:         "DeepSeek 官方",
			Provider:     "deepseek",
			Endpoint:     "https://api.deepseek.com",
			DefaultModel: "deepseek-v4-flash",
			Models:       []string{"deepseek-v4-flash", "deepseek-v4-pro"},
			APIKeyURL:    "https://platform.deepseek.com/api_keys",
			Priority:     100,
			Description:  "DeepSeek 官方 API，支持 thinking 模式",
			Capabilities: ModelCapability{Vision: false, ToolCall: true, Thinking: true, Multimodal: false},
			DefaultParams: map[string]interface{}{
				"thinking":              map[string]interface{}{"type": "enabled"},
				"reasoning_effort":      "high",
				"max_completion_tokens": 8192,
				"frequency_penalty":     float64(0),
				"presence_penalty":      float64(0),
			},
		},
		{
			ID:           "qwen-official",
			Name:         "阿里千问（通义千问）",
			Provider:     "qwen",
			Endpoint:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			DefaultModel: "qwen-plus",
			Models:       []string{"qwen-plus", "qwen-max", "qwen-turbo", "qwen-vl-max"},
			APIKeyURL:    "https://bailian.console.aliyun.com/?apiKey=1#/api-key",
			Priority:     90,
			Description:  "阿里通义千问 API，支持多模态模型",
			Capabilities: ModelCapability{Vision: true, ToolCall: true, Thinking: false, Multimodal: true},
			DefaultParams: map[string]interface{}{
				"extra_body": map[string]interface{}{
					"chat_template_kwargs": map[string]interface{}{
						"enable_thinking": false,
					},
				},
				"frequency_penalty": float64(0),
				"presence_penalty":  float64(0),
			},
		},
		{
			ID:           "xiaomi-mimo",
			Name:         "小米 MiMo 大模型",
			Provider:     "xiaomi",
			Endpoint:     "https://api.xiaomimimo.com/v1",
			DefaultModel: "mimo-v2.5-pro",
			Models:       []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni", "mimo-v2-flash"},
			APIKeyURL:    "https://platform.xiaomimimo.com/#/console/api-keys",
			Priority:     80,
			Description:  "小米 MiMo 大模型 API",
			Capabilities: ModelCapability{Vision: false, ToolCall: true, Thinking: false, Multimodal: false},
			DefaultParams: map[string]interface{}{
				"frequency_penalty": float64(0),
				"presence_penalty":  float64(0),
			},
		},
		{
			ID:           "zhipu-glm",
			Name:         "智谱 AI（GLM）",
			Provider:     "zhipu",
			Endpoint:     "https://open.bigmodel.cn/api/paas/v4/",
			DefaultModel: "glm-4-plus",
			Models:       []string{"glm-4-plus", "glm-4-0520", "glm-4-air", "glm-4-flash", "glm-4v-plus"},
			APIKeyURL:    "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
			Priority:     75,
			Description:  "智谱 GLM 系列模型，支持视觉模型",
			Capabilities: ModelCapability{Vision: true, ToolCall: true, Thinking: false, Multimodal: true},
			DefaultParams: map[string]interface{}{
				"frequency_penalty": float64(0),
				"presence_penalty":  float64(0),
			},
		},
		{
			ID:           "openai-official",
			Name:         "OpenAI 官方",
			Provider:     "openai",
			Endpoint:     "https://api.openai.com/v1",
			DefaultModel: "gpt-4o",
			Models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
			APIKeyURL:    "https://platform.openai.com/api-keys",
			Priority:     95,
			Description:  "OpenAI 官方 API，GPT-4o 支持多模态",
			Capabilities: ModelCapability{Vision: true, ToolCall: true, Thinking: false, Multimodal: true},
			DefaultParams: map[string]interface{}{
				"reasoning_effort":  "medium",
				"frequency_penalty": float64(0),
				"presence_penalty":  float64(0),
			},
		},
		{
			ID:           "lmstudio-local",
			Name:         "LM Studio 本地部署",
			Provider:     "lmstudio",
			Endpoint:     "http://localhost:1234/v1",
			DefaultModel: "",
			Models:       []string{},
			APIKeyURL:    "",
			Priority:     55,
			Description:  "本地部署的 LM Studio 服务，模型需自行加载",
			Capabilities: ModelCapability{Vision: false, ToolCall: false, Thinking: false, Multimodal: false},
		},
		{
			ID:           "ollama-local",
			Name:         "Ollama 本地部署",
			Provider:     "ollama",
			Endpoint:     "http://localhost:11434/v1",
			DefaultModel: "",
			Models:       []string{},
			APIKeyURL:    "",
			Priority:     50,
			Description:  "本地部署的 Ollama 服务，模型需自行拉取",
			Capabilities: ModelCapability{Vision: false, ToolCall: false, Thinking: false, Multimodal: false},
		},
		{
			ID:           "custom-openai-compatible",
			Name:         "OpenAI 兼容（自定义）",
			Provider:     "openai-compatible",
			Endpoint:     "",
			DefaultModel: "",
			Models:       []string{},
			APIKeyURL:    "",
			Priority:     60,
			Description:  "任何兼容 OpenAI API 的服务",
			Capabilities: ModelCapability{Vision: false, ToolCall: false, Thinking: false, Multimodal: false},
		},
	}

	for i := range templates {
		m.templates[templates[i].ID] = &templates[i]
	}
}

// GetTemplate returns a template by ID.
func (m *ModelManager) GetTemplate(id string) *ModelTemplate {
	m.templatesMu.RLock()
	defer m.templatesMu.RUnlock()
	return m.templates[id]
}

// GetAllTemplates returns all built-in templates in a fixed order.
func (m *ModelManager) GetAllTemplates() []*ModelTemplate {
	m.templatesMu.RLock()
	defer m.templatesMu.RUnlock()

	// Fixed order for built-in templates
	order := []string{
		"deepseek-official",
		"qwen-official",
		"xiaomi-mimo",
		"zhipu-glm",
		"openai-official",
		"lmstudio-local",
		"ollama-local",
		"custom-openai-compatible",
	}

	result := make([]*ModelTemplate, 0, len(m.templates))
	for _, id := range order {
		if t, ok := m.templates[id]; ok {
			result = append(result, t)
		}
	}
	// Append any custom templates not in the fixed order
	for _, t := range m.templates {
		found := false
		for _, id := range order {
			if t.ID == id {
				found = true
				break
			}
		}
		if !found {
			result = append(result, t)
		}
	}
	return result
}

// ListTemplates returns all built-in template IDs.
func (m *ModelManager) ListTemplates() []string {
	m.templatesMu.RLock()
	defer m.templatesMu.RUnlock()
	result := make([]string, 0, len(m.templates))
	for id := range m.templates {
		result = append(result, id)
	}
	return result
}

// AddModel adds a new model configuration.
func (m *ModelManager) AddModel(model *ModelConfig) error {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()

	for _, existing := range m.models {
		if existing.ID == model.ID {
			return fmt.Errorf("model ID %s already exists", model.ID)
		}
	}

	m.models = append(m.models, model)
	return nil
}

// RemoveModel removes a model by ID.
func (m *ModelManager) RemoveModel(id string) error {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()

	for i, model := range m.models {
		if model.ID == id {
			m.models = append(m.models[:i], m.models[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("model %s not found", id)
}

// GetModel returns a model by ID.
func (m *ModelManager) GetModel(id string) *ModelConfig {
	m.modelsMu.RLock()
	defer m.modelsMu.RUnlock()

	for _, model := range m.models {
		if model.ID == id {
			return model
		}
	}
	return nil
}

// GetAllModels returns all configured models, sorted by priority (descending).
func (m *ModelManager) GetAllModels() []*ModelConfig {
	m.modelsMu.RLock()
	defer m.modelsMu.RUnlock()

	result := make([]*ModelConfig, len(m.models))
	copy(result, m.models)

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority > result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetActiveModel returns the highest priority enabled model that matches the given capabilities.
func (m *ModelManager) GetActiveModel(visionRequired bool) *ModelConfig {
	m.modelsMu.RLock()
	defer m.modelsMu.RUnlock()

	var bestModel *ModelConfig
	bestPriority := -1

	for _, model := range m.models {
		if !model.Enabled {
			continue
		}

		if visionRequired && !model.Capabilities.Vision {
			continue
		}

		if model.Priority > bestPriority {
			bestPriority = model.Priority
			bestModel = model
		}
	}

	return bestModel
}

// GetActiveModelFromConfig returns the highest priority enabled model from a Config's Models slice.
// This is a convenience function for code that has access to *Config but not *ModelManager.
// Returns nil if no enabled models are found.
func GetActiveModelFromConfig(cfg *Config) *ModelConfig {
	if cfg == nil {
		return nil
	}
	var best *ModelConfig
	bestPriority := -1
	for _, m := range cfg.Models {
		if m.Enabled && m.Priority > bestPriority {
			bestPriority = m.Priority
			best = m
		}
	}
	return best
}

// GetModelsWithCapability returns all enabled models that have the specified capability.
func (m *ModelManager) GetModelsWithCapability(vision, toolCall, thinking bool) []*ModelConfig {
	m.modelsMu.RLock()
	defer m.modelsMu.RUnlock()

	var result []*ModelConfig
	for _, model := range m.models {
		if !model.Enabled {
			continue
		}

		match := true
		if vision && !model.Capabilities.Vision {
			match = false
		}
		if toolCall && !model.Capabilities.ToolCall {
			match = false
		}
		if thinking && !model.Capabilities.Thinking {
			match = false
		}

		if match {
			result = append(result, model)
		}
	}

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority > result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// SwitchToModel switches the active model to the one with the given ID.
func (m *ModelManager) SwitchToModel(id string) error {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()

	for _, model := range m.models {
		if model.ID == id {
			model.Enabled = true
			return nil
		}
	}
	return fmt.Errorf("model %s not found", id)
}

// EnableModel disables all models, then enables only the specified one.
func (m *ModelManager) EnableModel(id string) error {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()

	found := false
	for _, model := range m.models {
		if model.ID == id {
			model.Enabled = true
			found = true
		} else {
			model.Enabled = false
		}
	}

	if !found {
		return fmt.Errorf("model %s not found", id)
	}
	return nil
}

// SaveModels saves all model configurations to a JSON file.
func (m *ModelManager) SaveModels(path string) error {
	m.modelsMu.RLock()
	defer m.modelsMu.RUnlock()

	data, err := json.MarshalIndent(m.models, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal models: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write models file: %w", err)
	}
	return nil
}

// LoadModels loads model configurations from a JSON file.
func (m *ModelManager) LoadModels(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read models file: %w", err)
	}

	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()

	var loaded []*ModelConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("cannot parse models file: %w", err)
	}

	m.models = loaded
	return nil
}

// ExportTemplatesAsJSON returns all built-in templates as JSON string.
func (m *ModelManager) ExportTemplatesAsJSON() (string, error) {
	m.templatesMu.RLock()
	defer m.templatesMu.RUnlock()

	templates := make([]ModelTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, *t)
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportTemplate adds a custom template from JSON data.
func (m *ModelManager) ImportTemplate(jsonData string) error {
	var template ModelTemplate
	if err := json.Unmarshal([]byte(jsonData), &template); err != nil {
		return fmt.Errorf("cannot parse template JSON: %w", err)
	}

	if template.ID == "" || template.Name == "" {
		return fmt.Errorf("template must have ID and Name")
	}

	m.templatesMu.Lock()
	defer m.templatesMu.Unlock()

	m.templates[template.ID] = &template
	return nil
}

// CreateModelFromTemplate creates a new model config based on a template.
func (m *ModelManager) CreateModelFromTemplate(templateID string, modelID string, name string, apiKey string) (*ModelConfig, error) {
	m.templatesMu.RLock()
	template, ok := m.templates[templateID]
	m.templatesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	configID := fmt.Sprintf("%s-%s", templateID, strings.ReplaceAll(modelID, "/", "-"))
	if name == "" {
		name = fmt.Sprintf("%s (%s)", template.Name, modelID)
	}

	// Inherit default params from template
	var customParams map[string]interface{}
	if len(template.DefaultParams) > 0 {
		customParams = deepCopyMap(template.DefaultParams)
	}

	modelConfig := &ModelConfig{
		ID:           configID,
		Name:         name,
		Provider:     template.Provider,
		Endpoint:     template.Endpoint,
		Model:        modelID,
		APIKey:       apiKey,
		Priority:     template.Priority,
		Capabilities: template.Capabilities,
		Enabled:      false,
		TemplateID:   templateID,
		CustomParams: customParams,
	}

	if err := m.AddModel(modelConfig); err != nil {
		return nil, err
	}

	return modelConfig, nil
}

// SaveModelsToWorkspace saves models to the workspace models.json file.
func SaveModelsToWorkspace(wsRoot string, models []*ModelConfig) error {
	dir := filepath.Join(wsRoot, "db")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create db directory: %w", err)
	}

	path := filepath.Join(dir, "models.json")
	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal models: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// LoadModelsFromWorkspace loads models from the workspace models.json file.
func LoadModelsFromWorkspace(wsRoot string) ([]*ModelConfig, error) {
	path := filepath.Join(wsRoot, "db", "models.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read models file: %w", err)
	}

	var models []*ModelConfig
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("cannot parse models file: %w", err)
	}

	return models, nil
}

// deepCopyMap performs a deep copy of a map[string]interface{}.
// This is needed to avoid sharing references between template DefaultParams and model CustomParams.
func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[k] = deepCopyMap(val)
		case []interface{}:
			dst[k] = deepCopySlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

// deepCopySlice performs a deep copy of a []interface{}.
func deepCopySlice(src []interface{}) []interface{} {
	dst := make([]interface{}, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[i] = deepCopyMap(val)
		case []interface{}:
			dst[i] = deepCopySlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
}
