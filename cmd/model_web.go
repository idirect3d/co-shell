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
	"sync"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// WebModel is one model exposed to the Web UI. The API key is masked.
type WebModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Model           string `json:"model"`
	APIKey          string `json:"api_key"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
	Vision          bool   `json:"vision"`
	ToolCall        bool   `json:"tool_call"`
	Thinking        bool   `json:"thinking"`

	// FEATURE-526: the model-level thinking switch (a nil config value means
	// "not explicitly disabled") and the reasoning depth. The Web UI renders a
	// thinking-depth bulb from these two values; Thinking above stays the
	// capability mark ("the model supports thinking").
	ThinkingEnabled bool   `json:"thinking_enabled"`
	ReasoningEffort string `json:"reasoning_effort"`

	MaxModelLen int    `json:"max_model_len"`
	TemplateID  string `json:"template_id,omitempty"`

	// Available reports whether the model was reachable and present in its
	// endpoint's /models list at the time the list was built (FEATURE-496).
	// The front end greys out unavailable models.
	Available bool `json:"available"`

	// MetricsEndpoint is the configured Prometheus URL of the engine serving
	// this model, echoed so the front end can tell "not configured" from
	// "configured but unreachable" (FEATURE-546).
	MetricsEndpoint string `json:"metrics_endpoint,omitempty"`

	// MTP is the speculative-decoding acceptance rate probed while the model
	// list was built. It is always present: when unavailable it carries a
	// reason key the front end localises (FEATURE-546).
	MTP llm.MTPStatus `json:"mtp"`
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
		// FEATURE-496: report whether the model is currently reachable and
		// present in its endpoint's /models list so the front end can grey out
		// unavailable models. Only models with an endpoint and API key are
		// checked; others default to available=true (no info to judge).
		available := true
		if m.Endpoint != "" && m.APIKey != "" && m.Model != "" {
			if ok, _ := llm.CheckModelAvailable(m.Endpoint, m.APIKey, m.Model, 5); !ok {
				available = false
			}
		}
		// FEATURE-526: a nil switch means "not explicitly disabled" (true); the
		// Web UI combines it with the capability mark and the reasoning depth
		// to pick the thinking-depth bulb it shows.
		thinkingEnabled := m.ThinkingEnabled == nil || *m.ThinkingEnabled
		effort := ""
		if m.ReasoningEffort != nil {
			effort = *m.ReasoningEffort
		}
		models = append(models, WebModel{
			ID:              m.ID,
			Name:            m.Name,
			Provider:        m.Provider,
			Endpoint:        m.Endpoint,
			Model:           m.Model,
			APIKey:          maskKey(m.APIKey),
			Priority:        m.Priority,
			Enabled:         m.Enabled,
			Vision:          m.Capabilities.Vision,
			ToolCall:        m.Capabilities.ToolCall,
			Thinking:        m.Capabilities.Thinking,
			ThinkingEnabled: thinkingEnabled,
			ReasoningEffort: effort,
			MaxModelLen:     m.MaxModelLen,
			TemplateID:      m.TemplateID,
			Available:       available,
			MetricsEndpoint: m.MetricsEndpoint,
		})
	}
	probeMTP(models)
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

// ModelAutoSelect reports whether the automatic selection (degrade-on-failure)
// switch is on (FEATURE-602).
func (h *ModelHandler) ModelAutoSelect() bool {
	return h.cfg.LLM.AutoSelectEnabled()
}

// ModelSetAutoSelect persists the automatic selection switch (FEATURE-602) and
// returns the localized confirmation.
func (h *ModelHandler) ModelSetAutoSelect(on bool) (string, error) {
	h.cfg.LLM.AutoModelSelect = on
	if err := h.cfg.Save(); err != nil {
		return "", err
	}
	return i18n.TF(i18n.KeySettingsUpdated, "auto-model-select", boolStr(on)), nil
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

// probeMTP fills in the MTP status of every model in the list by scraping its
// configured metrics_endpoint (FEATURE-546). Probes run concurrently because
// each one is an HTTP round-trip to a possibly remote inference host: doing
// them serially would make the model list take N timeouts to appear when a
// host is down. Models without a metrics_endpoint get the "not configured"
// reason without any network access.
func probeMTP(models []WebModel) {
	var wg sync.WaitGroup
	for i := range models {
		if models[i].MetricsEndpoint == "" {
			models[i].MTP = llm.MTPStatus{
				Available: false,
				Reason:    llm.MTPReasonNotConfigured,
			}
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			models[idx].MTP = llm.FetchMTPRate(models[idx].MetricsEndpoint, 0)
		}(i)
	}
	wg.Wait()
}
