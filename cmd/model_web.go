// Web UI model management support (FEATURE-422): ModelWebJSON returns the
// configured models and built-in templates as structured JSON so the browser
// can render a graphical model manager. Non-interactive operations (switch /
// enable / disable / remove / set-priority) delegate to the existing
// ModelHandler methods. The add/edit wizards run through the agent's UserIO
// (WebIO in a web session), so the browser drives them via the standard
// ask/interaction round-trips.
//
// Author: L.Shuang
// Created: 2026-08-23
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"github.com/idirect3d/co-shell/config"
)

// WebModel is one model exposed to the Web UI. The API key is masked.
type WebModel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Endpoint     string `json:"endpoint"`
	Model        string `json:"model"`
	APIKey       string `json:"api_key"`
	Priority     int    `json:"priority"`
	Enabled      bool   `json:"enabled"`
	Vision       bool   `json:"vision"`
	ToolCall     bool   `json:"tool_call"`
	Thinking     bool   `json:"thinking"`
	MaxModelLen  int    `json:"max_model_len"`
	TemplateID   string `json:"template_id,omitempty"`
}

// WebTemplate is one built-in template exposed to the Web UI.
type WebTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Endpoint     string   `json:"endpoint"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	Vision       bool     `json:"vision"`
	ToolCall     bool     `json:"tool_call"`
	Thinking     bool     `json:"thinking"`
}

// ModelWebJSON returns the configured models and built-in templates for the
// Web UI model manager.
func (h *ModelHandler) ModelWebJSON() (models []WebModel, templates []WebTemplate) {
	for _, m := range h.cfg.Models {
		models = append(models, WebModel{
			ID:          m.ID,
			Name:        m.Name,
			Provider:    m.Provider,
			Endpoint:    m.Endpoint,
			Model:       m.Model,
			APIKey:      maskKey(m.APIKey),
			Priority:    m.Priority,
			Enabled:     m.Enabled,
			Vision:      m.Capabilities.Vision,
			ToolCall:    m.Capabilities.ToolCall,
			Thinking:    m.Capabilities.Thinking,
			MaxModelLen: m.MaxModelLen,
			TemplateID:  m.TemplateID,
		})
	}
	manager := config.GetDefaultModelManager()
	for _, t := range manager.GetAllTemplates() {
		templates = append(templates, WebTemplate{
			ID:           t.ID,
			Name:         t.Name,
			Provider:     t.Provider,
			Endpoint:     t.Endpoint,
			DefaultModel: t.DefaultModel,
			Models:       t.Models,
			Vision:       t.Capabilities.Vision,
			ToolCall:     t.Capabilities.ToolCall,
			Thinking:     t.Capabilities.Thinking,
		})
	}
	return models, templates
}

// ModelSwitch switches to the given model (delegates to switchModel).
func (h *ModelHandler) ModelSwitch(id string) (string, error) {
	return h.switchModel([]string{id})
}

// ModelEnable enables the given model (delegates to enableModel).
func (h *ModelHandler) ModelEnable(id string) (string, error) {
	return h.enableModel([]string{id})
}

// ModelDisable disables the given model (delegates to disableModel).
func (h *ModelHandler) ModelDisable(id string) (string, error) {
	return h.disableModel([]string{id})
}

// ModelRemove removes the given model (delegates to removeModel).
func (h *ModelHandler) ModelRemove(id string) (string, error) {
	return h.removeModel([]string{id})
}

// ModelSetPriority sets the priority of the given model (delegates to
// setPriorityForModel).
func (h *ModelHandler) ModelSetPriority(id string, priority int) (string, error) {
	return h.setPriorityForModel(id, priority)
}

// StartAddWizard launches the interactive add-model wizard. In a web session
// the wizard's prompts and input requests flow through the agent's WebIO, so
// the browser drives it via ask/interaction round-trips.
func (h *ModelHandler) StartAddWizard() (string, error) {
	return h.AddModelWizard()
}

// StartEditWizard launches the interactive edit-model wizard for the given
// model ID.
func (h *ModelHandler) StartEditWizard(id string) (string, error) {
	return h.editModelWizard([]string{id})
}
