// Web UI structured model wizard (FEATURE-429): a stateless, structured
// alternative to the TUI AddModelWizard/editModelWizard. The browser owns the
// accumulated field data (WebWizardData) and asks the backend for the form of
// each step (WebWizardStepData). The backend reuses the existing low-level
// logic (saveModel / fetchModelSuggestions / detectModelCapabilities /
// autoCompleteEndpoint / modelIDExists / knownMaxModelLen / parseTokenCount)
// but returns structured form descriptions instead of driving a text prompt
// loop through UserIO. The TUI wizard is left untouched.
//
// Author: L.Shuang
// Created: 2026-08-24
// MIT License - Copyright (c) 2026 L.Shuang

package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// WebWizardStep identifies a step in the add/edit model wizard.
type WebWizardStep string

const (
	WebWizardTemplate     WebWizardStep = "template"
	WebWizardEndpoint     WebWizardStep = "endpoint"
	WebWizardAPIKey       WebWizardStep = "api_key"
	WebWizardModelName    WebWizardStep = "model_name"
	WebWizardCapabilities WebWizardStep = "capabilities"
	WebWizardModelID      WebWizardStep = "model_id"
	WebWizardPriority     WebWizardStep = "priority"
	WebWizardMaxModelLen  WebWizardStep = "max_model_len"
	WebWizardEnabled      WebWizardStep = "enabled"
)

// webWizardSteps is the ordered list of wizard steps.
var webWizardSteps = []WebWizardStep{
	WebWizardTemplate,
	WebWizardEndpoint,
	WebWizardAPIKey,
	WebWizardModelName,
	WebWizardCapabilities,
	WebWizardModelID,
	WebWizardPriority,
	WebWizardMaxModelLen,
	WebWizardEnabled,
}

// WebWizardField describes one form field in a step.
type WebWizardField struct {
	Key      string   `json:"key"`                // field key, e.g. "template", "endpoint"
	Type     string   `json:"type"`               // "select" | "text" | "password" | "number" | "checkbox" | "switch"
	Label    string   `json:"label"`              // human-readable label
	Value    string   `json:"value"`              // current/default value
	Options  []string `json:"options,omitempty"`  // for select
	Required bool     `json:"required"`
	Hint     string   `json:"hint,omitempty"`
}

// WebWizardStepData describes one step's form for the frontend to render.
type WebWizardStepData struct {
	Step    WebWizardStep    `json:"step"`
	Title   string           `json:"title"`
	Fields  []WebWizardField `json:"fields"`
	IsFirst bool             `json:"is_first"`
	IsLast  bool             `json:"is_last"`
	// ModelMaxLens maps a model ID to its max context length (in tokens) as
	// reported by the API. Populated on the model_name step so the frontend can
	// record the selected model's max length for later steps (FEATURE-429).
	ModelMaxLens map[string]int `json:"model_max_lens,omitempty"`
	// Message carries an informational/error message to show the user (e.g. why
	// the model list refresh failed). Empty when there is nothing to report.
	Message string `json:"message,omitempty"`
	// TemplateJSON carries the selected template's raw JSON (pretty-printed) so
	// the frontend can show it in a collapsible viewer on the template step
	// (FEATURE-467). Empty when no template is selected.
	TemplateJSON string `json:"template_json,omitempty"`
	// ReasoningEffortOptions lists the reasoning_effort choices for the selected
	// template's provider (FEATURE-467). Empty when the provider has no
	// reasoning_effort concept.
	ReasoningEffortOptions []string `json:"reasoning_effort_options,omitempty"`
}

// WebWizardData holds all filled fields across steps. It is owned by the
// browser and sent back on each next/prev/submit call, so the backend stays
// stateless.
type WebWizardData struct {
	Mode        string `json:"mode"` // "add" | "edit"
	EditID      string `json:"edit_id,omitempty"`
	TemplateID  string `json:"template_id,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	ModelName   string `json:"model_name,omitempty"`
	Vision      bool   `json:"vision"`
	ToolCall    bool   `json:"tool_call"`
	Thinking    bool   `json:"thinking"`
	// ReasoningEffort is the reasoning depth for the model (FEATURE-467).
	// Empty means "use the provider/template default".
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	ModelID         string `json:"model_id,omitempty"`
	Priority        int    `json:"priority"`
	MaxModelLen     int    `json:"max_model_len"`
	// ModelMaxLen is the max context length (in tokens) reported by the API for
	// the currently selected model. Recorded on the model_name step and used to
	// pre-fill / hint the max_model_len step (FEATURE-429).
	ModelMaxLen int  `json:"model_max_len,omitempty"`
	Enabled     bool `json:"enabled"`
}

// WebWizardStart begins a new add/edit wizard and returns the first step's
// form. For edit mode the existing model's values are pre-filled.
func (h *ModelHandler) WebWizardStart(mode, id string) (*WebWizardStepData, *WebWizardData, error) {
	data := &WebWizardData{Mode: mode, EditID: id, Enabled: true}
	if mode == "edit" {
		model := h.findModel(id)
		if model == nil {
			return nil, nil, fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), id)
		}
		data.TemplateID = model.TemplateID
		data.Endpoint = model.Endpoint
		data.APIKey = model.APIKey
		data.ModelName = model.Model
		data.Vision = model.Capabilities.Vision
		data.ToolCall = model.Capabilities.ToolCall
		data.Thinking = model.Capabilities.Thinking
		// FEATURE-467: pre-fill the model-level thinking settings on edit.
		if model.ThinkingEnabled != nil {
			data.Thinking = *model.ThinkingEnabled
		}
		if model.ReasoningEffort != nil {
			data.ReasoningEffort = *model.ReasoningEffort
		}
		data.ModelID = model.ID
		data.Priority = model.Priority
		data.MaxModelLen = model.MaxModelLen
		data.Enabled = model.Enabled
	}
	step, err := h.webWizardStepData(data, WebWizardTemplate)
	return step, data, err
}

// WebWizardNext validates the current step's values and returns the next
// step's form. step is the step the browser is currently on; data holds all
// accumulated values (including the just-filled current step).
func (h *ModelHandler) WebWizardNext(data *WebWizardData, step WebWizardStep) (*WebWizardStepData, error) {
	idx := webWizardStepIndex(step)
	if idx < 0 || idx >= len(webWizardSteps)-1 {
		return nil, fmt.Errorf("invalid step %q", step)
	}
	next := webWizardSteps[idx+1]
	return h.webWizardStepData(data, next)
}

// WebWizardPrev returns the previous step's form.
func (h *ModelHandler) WebWizardPrev(data *WebWizardData, step WebWizardStep) (*WebWizardStepData, error) {
	idx := webWizardStepIndex(step)
	if idx <= 0 {
		return nil, fmt.Errorf("already at first step")
	}
	prev := webWizardSteps[idx-1]
	return h.webWizardStepData(data, prev)
}

// WebWizardRefresh re-renders the current step's form. It is used by the
// "refresh model list" button on the model_name step to re-fetch the model
// suggestions without advancing to the next step (FEATURE-429).
func (h *ModelHandler) WebWizardRefresh(data *WebWizardData, step WebWizardStep) (*WebWizardStepData, error) {
	return h.webWizardStepData(data, step)
}

// WebWizardSubmit builds a ModelConfig from the accumulated data and saves it.
// For add mode it calls saveModel; for edit mode it updates the existing model.
func (h *ModelHandler) WebWizardSubmit(data *WebWizardData) (string, error) {
	if data.Mode == "edit" {
		model := h.findModel(data.EditID)
		if model == nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), data.EditID)
		}
		model.Endpoint = data.Endpoint
		model.APIKey = data.APIKey
		model.Model = data.ModelName
		model.Priority = data.Priority
		model.MaxModelLen = data.MaxModelLen
		model.Capabilities = config.ModelCapability{
			Vision:   data.Vision,
			ToolCall: data.ToolCall,
			Thinking: data.Thinking,
		}
		// FEATURE-467: persist the model-level thinking settings.
		thinking := data.Thinking
		model.ThinkingEnabled = &thinking
		if data.ReasoningEffort != "" {
			effort := data.ReasoningEffort
			model.ReasoningEffort = &effort
		} else {
			model.ReasoningEffort = nil
		}
		model.Enabled = data.Enabled
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
		}
		h.syncModelsToManager()
		if h.agent != nil {
			h.agent.ApplyWorkModeConfig()
		}
		log.Info("Edited model via web wizard: %s", data.EditID)
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_200), data.EditID), nil
	}

	if data.TemplateID == "" {
		return "", fmt.Errorf("template not selected")
	}
	manager := config.GetDefaultModelManager()
	template := manager.GetTemplate(data.TemplateID)
	if template == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_312), data.TemplateID)
	}

	modelConfig := &config.ModelConfig{
		ID:           data.ModelID,
		Name:         fmt.Sprintf("%s (%s)", template.Name, data.ModelName),
		Provider:     template.Provider,
		Endpoint:     data.Endpoint,
		Model:        data.ModelName,
		APIKey:       data.APIKey,
		Priority:     data.Priority,
		Enabled:      data.Enabled,
		TemplateID:   template.ID,
		Capabilities: config.ModelCapability{
			Vision:   data.Vision,
			ToolCall: data.ToolCall,
			Thinking: data.Thinking,
		},
		MaxModelLen: data.MaxModelLen,
	}
	// FEATURE-467: persist the model-level thinking settings chosen on the
	// template step.
	thinking := data.Thinking
	modelConfig.ThinkingEnabled = &thinking
	if data.ReasoningEffort != "" {
		effort := data.ReasoningEffort
		modelConfig.ReasoningEffort = &effort
	}
	if err := h.saveModel(modelConfig); err != nil {
		return "", err
	}
	log.Info("Added model via web wizard: %s (template=%s, model=%s)", modelConfig.ID, template.ID, data.ModelName)
	return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_199), modelConfig.ID, modelConfig.Model), nil
}

// webWizardStepData builds the form description for the given step based on the
// accumulated data. Network-triggering steps (model name suggestions, capability
// detection) run their detection here.
func (h *ModelHandler) webWizardStepData(data *WebWizardData, step WebWizardStep) (*WebWizardStepData, error) {
	idx := webWizardStepIndex(step)
	if idx < 0 {
		return nil, fmt.Errorf("invalid step %q", step)
	}
	sd := &WebWizardStepData{
		Step:    step,
		IsFirst: idx == 0,
		IsLast:  idx == len(webWizardSteps)-1,
	}

	switch step {
	case WebWizardTemplate:
		sd.Title = i18n.T(i18n.KeyCmdMig_160)
		manager := config.GetDefaultModelManager()
		field := WebWizardField{Key: "template_id", Type: "select", Label: i18n.T(i18n.KeyCmdMig_206), Required: true}
		for _, t := range manager.GetAllTemplates() {
			field.Options = append(field.Options, t.ID)
		}
		field.Value = data.TemplateID
		// FEATURE-467: expose the selected template's thinking switch and, when
		// thinking is on, the provider-specific reasoning_effort choices. The
		// template's raw JSON is also returned so the frontend can show it in a
		// collapsible viewer (transparency).
		sd.Fields = []WebWizardField{field}
		if t := h.template(data.TemplateID); t != nil {
			// Thinking switch default: the template's declared thinking capability.
			thinking := t.Capabilities.Thinking
			if data.Thinking {
				thinking = true
			}
			sd.Fields = append(sd.Fields, WebWizardField{
				Key: "thinking", Type: "switch", Label: i18n.T(i18n.KeyCmdMig_378),
				Value: webWizardBoolStr(thinking), Required: false,
			})
			// reasoning_effort select (shown only when thinking is on). The default
			// is the empty "not set" option so the user decides whether to send it
			// (FEATURE-467). On edit, the model's saved value is pre-filled.
			opts := reasoningEffortOptions(t.Provider)
			if len(opts) > 0 {
				sd.ReasoningEffortOptions = opts
				effort := data.ReasoningEffort
				sd.Fields = append(sd.Fields, WebWizardField{
					Key: "reasoning_effort", Type: "select", Label: i18n.T(i18n.KeyCmdMig_383),
					Value: effort, Options: opts, Required: false,
				})
			}
			// Template raw JSON for the collapsible transparency viewer.
			if j, err := json.MarshalIndent(t, "", "  "); err == nil {
				sd.TemplateJSON = string(j)
			}
		}

	case WebWizardEndpoint:
		sd.Title = i18n.T(i18n.KeyCmdMig_180)
		defaultEndpoint := ""
		if t := h.template(data.TemplateID); t != nil {
			defaultEndpoint = t.Endpoint
		}
		if data.Endpoint != "" {
			defaultEndpoint = data.Endpoint
		}
		sd.Fields = []WebWizardField{{
			Key: "endpoint", Type: "text", Label: i18n.T(i18n.KeyCmdMig_344),
			Value: defaultEndpoint, Required: true,
		}}

	case WebWizardAPIKey:
		sd.Title = i18n.T(i18n.KeyCmdMig_179)
		sd.Fields = []WebWizardField{{
			Key: "api_key", Type: "password", Label: i18n.T(i18n.KeyCmdMig_343),
			Value: data.APIKey, Required: false,
		}}

	case WebWizardModelName:
		sd.Title = i18n.T(i18n.KeyCmdMig_382)
		field := WebWizardField{Key: "model_name", Type: "select", Label: i18n.T(i18n.KeyCmdMig_358), Required: true}
		suggestions, _, modelInfos, fetchErr := h.fetchModelSuggestions(data.Endpoint, data.APIKey, h.template(data.TemplateID))
		field.Options = suggestions
		field.Value = data.ModelName
		sd.Fields = []WebWizardField{field}
		// Record each model's max context length (as reported by the API) so the
		// frontend can remember the selected model's max length for later steps.
		sd.ModelMaxLens = make(map[string]int, len(modelInfos))
		for _, mi := range modelInfos {
			if mi.MaxModelLen > 0 {
				sd.ModelMaxLens[mi.ID] = mi.MaxModelLen
			}
		}
		// Surface the fetch error (if any) so the user knows the model list may
		// be incomplete (only template defaults shown).
		if fetchErr != nil {
			sd.Message = fmt.Sprintf("获取模型列表失败：%v（已显示模板默认模型）", fetchErr)
		}

	case WebWizardCapabilities:
		sd.Title = i18n.T(i18n.KeyCmdMig_183)
		caps := config.ModelCapability{Vision: data.Vision, ToolCall: data.ToolCall, Thinking: data.Thinking}
		if caps == (config.ModelCapability{}) {
			caps = h.detectModelCapabilities(data.Endpoint, data.APIKey, data.ModelName)
		}
		sd.Fields = []WebWizardField{
			{Key: "vision", Type: "checkbox", Label: i18n.T(i18n.KeyCmdMig_376), Value: webWizardBoolStr(caps.Vision)},
			{Key: "tool_call", Type: "checkbox", Label: i18n.T(i18n.KeyCmdMig_380), Value: webWizardBoolStr(caps.ToolCall)},
			{Key: "thinking", Type: "checkbox", Label: i18n.T(i18n.KeyCmdMig_378), Value: webWizardBoolStr(caps.Thinking)},
		}

	case WebWizardModelID:
		sd.Title = i18n.T(i18n.KeyCmdMig_184)
		defaultID := data.ModelID
		if defaultID == "" {
			defaultID = fmt.Sprintf("%s-%s", data.TemplateID, strings.ReplaceAll(data.ModelName, "/", "-"))
			if h.modelIDExists(defaultID) {
				suffix := 2
				for {
					candidate := fmt.Sprintf("%s-%d", defaultID, suffix)
					if !h.modelIDExists(candidate) {
						defaultID = candidate
						break
					}
					suffix++
				}
			}
		}
		sd.Fields = []WebWizardField{{
			Key: "model_id", Type: "text", Label: i18n.T(i18n.KeyCmdMig_350),
			Value: defaultID, Required: true,
		}}

	case WebWizardPriority:
		sd.Title = i18n.T(i18n.KeyCmdMig_181)
		priority := data.Priority
		if priority == 0 {
			priority = (len(h.cfg.Models) + 1) * 10
		}
		sd.Fields = []WebWizardField{{
			Key: "priority", Type: "number", Label: i18n.T(i18n.KeyCmdMig_338),
			Value: strconv.Itoa(priority), Required: true,
		}}

	case WebWizardMaxModelLen:
		sd.Title = i18n.T(i18n.KeyCmdMig_186)
		maxLen := data.MaxModelLen
		if maxLen == 0 {
			// Prefer the model's max context length reported by the API (recorded
			// on the model_name step), then fall back to known/template defaults.
			maxLen = data.ModelMaxLen
		}
		if maxLen == 0 {
			maxLen = knownMaxModelLen(data.ModelName)
		}
		if maxLen == 0 {
			if t := h.template(data.TemplateID); t != nil && t.DefaultMaxModelLen > 0 {
				maxLen = t.DefaultMaxModelLen
			}
		}
		field := WebWizardField{
			Key: "max_model_len", Type: "number", Label: i18n.T(i18n.KeyCmdMig_352),
			Value: strconv.Itoa(maxLen), Required: false,
		}
		// Hint the user with the selected model's max context length as reported
		// by the API (recorded on the model_name step).
		if data.ModelMaxLen > 0 {
			field.Hint = fmt.Sprintf("当前模型 %s 的最大上下文长度为 %d tokens", data.ModelName, data.ModelMaxLen)
		}
		sd.Fields = []WebWizardField{field}

	case WebWizardEnabled:
		sd.Title = i18n.T(i18n.KeyCmdMig_182)
		sd.Fields = []WebWizardField{{
			Key: "enabled", Type: "switch", Label: i18n.T(i18n.KeyCmdMig_286),
			Value: webWizardBoolStr(data.Enabled), Required: false,
		}}
	}

	return sd, nil
}

// findModel returns the configured model with the given ID, or nil.
func (h *ModelHandler) findModel(id string) *config.ModelConfig {
	for _, m := range h.cfg.Models {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// template returns the built-in template with the given ID, or nil.
func (h *ModelHandler) template(id string) *config.ModelTemplate {
	if id == "" {
		return nil
	}
	return config.GetDefaultModelManager().GetTemplate(id)
}

// webWizardStepIndex returns the index of a step in webWizardSteps, or -1.
func webWizardStepIndex(step WebWizardStep) int {
	for i, s := range webWizardSteps {
		if s == step {
			return i
		}
	}
	return -1
}

// webWizardBoolStr renders a bool as "true"/"false" for a form field value.
func webWizardBoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// reasoningEffortOptions returns the reasoning_effort choices for a provider
// (FEATURE-467). The first (empty) entry is the "not set" option meaning the
// reasoning_effort parameter is not sent, letting the user decide per model
// (e.g. Qwen3.6 does not support reasoning_effort while Qwen3.8 does).
// qwen includes xhigh because Qwen3.8 uses xhigh/medium/low.
func reasoningEffortOptions(provider string) []string {
	switch provider {
	case "qwen":
		return []string{"", "low", "medium", "high", "xhigh"}
	case "deepseek", "zhipu", "moonshot", "kimi", "openai", "openai-compatible":
		return []string{"", "low", "medium", "high"}
	default:
		// Providers without a reasoning_effort concept still expose the "not set"
		// option so the wizard stays uniform and the user decides.
		return []string{""}
	}
}

