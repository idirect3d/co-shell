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

package cmd

import (
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// ModelHandler handles the .model built-in command for multi-model management.
type ModelHandler struct {
	cfg         *config.Config
	agent       *agent.Agent
	wizardStack []string // stack of wizard steps to return to
}

// NewModelHandler creates a new ModelHandler.
func NewModelHandler(cfg *config.Config, ag *agent.Agent) *ModelHandler {
	return &ModelHandler{
		cfg:         cfg,
		agent:       ag,
		wizardStack: make([]string, 0),
	}
}

// io returns the UserIO from the agent, falling back to DefaultUserIO.
func (h *ModelHandler) io() agent.UserIO {
	return agent.GetIO(h.agent)
}

// syncModelsToManager synchronizes all models from h.cfg.Models to the singleton ModelManager.
// This ensures that selectModelForCall() in the agent loop can find the latest models.
// The ModelManager's internal list is fully replaced with the current cfg.Models.
// Must be called after any mutation to h.cfg.Models (add/remove/switch/enable/disable/priority).
func (h *ModelHandler) syncModelsToManager() {
	manager := config.GetDefaultModelManager()
	allModels := manager.GetAllModels()
	for _, m := range allModels {
		_ = manager.RemoveModel(m.ID)
	}
	for _, m := range h.cfg.Models {
		_ = manager.AddModel(m)
	}
	log.Debug("Synced %d models from cfg.Models to ModelManager", len(h.cfg.Models))
}

// Handle processes .model commands.
func (h *ModelHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showHelp()
	}

	subcommand := args[0]
	switch subcommand {
	case "list", "ls":
		return h.listModels()
	case "add":
		return h.AddModelWizard()
	case "edit":
		return h.editModelWizard(args[1:])
	case "remove", "rm":
		return h.removeModel(args[1:])
	case "switch", "use":
		return h.switchModel(args[1:])
	case "enable":
		return h.enableModel(args[1:])
	case "disable":
		return h.disableModel(args[1:])
	case "info":
		return h.modelInfo(args[1:])
	case "templates", "tpl":
		return h.listTemplates()
	case "add-from-template", "from-tpl":
		return h.addFromTemplate(args[1:])
	case "set-priority", "prio":
		return h.setPriority(args[1:])
	case "set-param", "param":
		return h.setParam(args[1:])
	default:
		return "", fmt.Errorf("unknown model subcommand: %s (use .model for help)", subcommand)
	}
}

// showHelp returns the help message for .model command.
func (h *ModelHandler) showHelp() (string, error) {
	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_157))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_034))
	result.WriteString(i18n.T(i18n.KeyCmdMig_033))
	result.WriteString(i18n.T(i18n.KeyCmdMig_028))
	result.WriteString(i18n.T(i18n.KeyCmdMig_030))
	result.WriteString(i18n.T(i18n.KeyCmdMig_032))
	result.WriteString(i18n.T(i18n.KeyCmdMig_035))
	result.WriteString(i18n.T(i18n.KeyCmdMig_038))
	result.WriteString(i18n.T(i18n.KeyCmdMig_031))
	result.WriteString(i18n.T(i18n.KeyCmdMig_029))
	result.WriteString(i18n.T(i18n.KeyCmdMig_037))
	result.WriteString(i18n.T(i18n.KeyCmdMig_036))
	result.WriteString(i18n.T(i18n.KeyCmdMig_039))
	result.WriteString(i18n.T(i18n.KeyCmdMig_082))
	result.WriteString(i18n.T(i18n.KeyCmdMig_068))
	result.WriteString(i18n.T(i18n.KeyCmdMig_067))
	result.WriteString(i18n.T(i18n.KeyCmdMig_066))
	return result.String(), nil
}

// listModels shows all configured models in a compact two-line format.
// Line 1: <No>.[<id>][<provider>][<endpoint>:<model>][<max_model_len>][<capabilities>]
// Line 2: model parameters (temperature/top-k/top-p/etc.)
func (h *ModelHandler) listModels() (string, error) {
	models := h.cfg.Models
	if len(models) == 0 {
		return i18n.T(i18n.KeyCmdMig_296), nil
	}

	sorted := make([]*config.ModelConfig, len(models))
	copy(sorted, models)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_158))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	activeCount := 0
	for idx, m := range sorted {
		status := "⬜"
		if m.Enabled {
			status = "✅"
			activeCount++
		}

		// Build capabilities string
		capStr := []string{}
		if m.Capabilities.Vision {
			capStr = append(capStr, "👁")
		}
		if m.Capabilities.ToolCall {
			capStr = append(capStr, "🔧")
		}
		if m.Capabilities.Thinking {
			capStr = append(capStr, "💭")
		}
		capsDisplay := strings.Join(capStr, "")

		// Build max_model_len display
		maxModelLenDisplay := ""
		if m.MaxModelLen > 0 {
			maxModelLenDisplay = fmt.Sprintf("%d", m.MaxModelLen)
		}

		// Line 1: <No>.[<id>][<provider>][<endpoint>:<model>][<max_model_len>][<capabilities>]
		no := idx + 1
		result.WriteString(fmt.Sprintf("  %s %d.[%s][%s][%s:%s]",
			status, no, m.ID, m.Provider, m.Endpoint, m.Model))
		if maxModelLenDisplay != "" {
			result.WriteString(fmt.Sprintf("[%s]", maxModelLenDisplay))
		}
		if capsDisplay != "" {
			result.WriteString(fmt.Sprintf("[%s]", capsDisplay))
		}
		result.WriteString("\n")

		// Line 2: model parameters
		params := []string{}
		if m.Temperature != nil {
			params = append(params, fmt.Sprintf("temperature=%.1f", *m.Temperature))
		}
		if m.MaxTokens != nil {
			params = append(params, fmt.Sprintf("max_tokens=%d", *m.MaxTokens))
		}
		if m.TopP != nil {
			params = append(params, fmt.Sprintf("top_p=%.2f", *m.TopP))
		}
		if m.TopK != nil {
			params = append(params, fmt.Sprintf("top_k=%d", *m.TopK))
		}
		if m.RepetitionPenalty != nil {
			params = append(params, fmt.Sprintf("repetition_penalty=%.1f", *m.RepetitionPenalty))
		}
		if m.ThinkingEnabled != nil {
			if *m.ThinkingEnabled {
				params = append(params, "thinking=on")
			} else {
				params = append(params, "thinking=off")
			}
		}
		if m.ReasoningEffort != nil {
			params = append(params, fmt.Sprintf("reasoning_effort=%s", *m.ReasoningEffort))
		}
		if len(params) > 0 {
			result.WriteString(fmt.Sprintf("      %s\n", strings.Join(params, " | ")))
		}

		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_007), m.Priority))
		result.WriteString("\n")
	}

	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_130), activeCount, len(sorted)))
	return result.String(), nil
}

// modelInfo shows detailed information about a specific model.
func (h *ModelHandler) modelInfo(args []string) (string, error) {
	var modelID string
	if len(args) == 0 {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_362))
		if err != nil {
			return "", err
		}
	} else {
		modelID = args[0]
	}
	var model *config.ModelConfig
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			model = m
			break
		}
	}

	if model == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
	}

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_159))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")
	result.WriteString(fmt.Sprintf("  ID: %s\n", model.ID))
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_089), model.Name))
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_086), model.Provider))
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_129), model.Endpoint))
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_116), model.Model))
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_081), model.Priority))
	if model.Enabled {
		result.WriteString(i18n.T(i18n.KeyCmdMig_122))
	} else {
		result.WriteString(i18n.T(i18n.KeyCmdMig_123))
	}
	if model.TemplateID != "" {
		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_120), model.TemplateID))
	}
	if model.MaxModelLen > 0 {
		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_112), model.MaxModelLen))
	} else {
		result.WriteString(i18n.T(i18n.KeyCmdMig_113))
	}

	capStr := []string{}
	if model.Capabilities.Vision {
		capStr = append(capStr, i18n.T(i18n.KeyCmdMig_376))
	}
	if model.Capabilities.ToolCall {
		capStr = append(capStr, i18n.T(i18n.KeyCmdMig_380))
	}
	if model.Capabilities.Thinking {
		capStr = append(capStr, i18n.T(i18n.KeyCmdMig_378))
	}
	if len(capStr) > 0 {
		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_131), strings.Join(capStr, "、")))
	} else {
		result.WriteString(i18n.T(i18n.KeyCmdMig_132))
	}

	// Show custom params
	if len(model.CustomParams) > 0 {
		result.WriteString(i18n.T(i18n.KeyCmdMig_133))
		for k, v := range model.CustomParams {
			result.WriteString(fmt.Sprintf("    %s: %v\n", k, v))
		}
	}

	return result.String(), nil
}

// modelWizardState accumulates state across wizard steps to support back navigation.
type modelWizardState struct {
	Template     *config.ModelTemplate
	Endpoint     string
	APIKey       string
	ModelName    string
	ModelID      string
	Priority     int
	Capabilities config.ModelCapability
	Enabled      bool
	MaxModelLen  int             // max context length from API model list
	APIModels    []llm.ModelInfo // model info from API for max_model_len lookup
}

// AddModelWizard starts the interactive wizard to add a new model.
// This is a public method so it can be called from main.go for first-time setup.
func (h *ModelHandler) AddModelWizard() (string, error) {
	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_160))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	state := modelWizardState{}

	for {
		// Step 1: Select template (or re-select after back navigation)
		if state.Template == nil {
			template, err := h.wizardSelectTemplate()
			if err != nil || template == nil {
				return result.String(), err
			}
			state.Template = template
			// Reset all model-specific fields when template changes
			state.Endpoint = ""
			state.APIKey = ""
			state.ModelName = ""
			state.ModelID = ""
			state.Priority = 0
			state.Capabilities = config.ModelCapability{}
			state.Enabled = true
		}

		// Step 2: Enter endpoint
		{
			io := h.io()
			// Use previously entered value as default if available, otherwise template default
			defaultEndpoint := state.Template.Endpoint
			if state.Endpoint != "" {
				defaultEndpoint = state.Endpoint
			}
			io.Println(i18n.T(i18n.KeyCmdMig_180))
			endpoint := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_344), defaultEndpoint, "q")
			if endpoint == "__BACK__" {
				state.Template = nil
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			if strings.ToUpper(endpoint) == "Q" || strings.ToUpper(endpoint) == "QUIT" {
				return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
			}
			// Auto-complete endpoint if needed
			if endpoint != "" {
				completedEndpoint, success := autoCompleteEndpoint(endpoint)
				if success {
					endpoint = completedEndpoint
					if endpoint != defaultEndpoint {
						h.io().Printf(i18n.T(i18n.KeyCmdMig_194), defaultEndpoint, endpoint)
					}
				}
			}
			// Test endpoint connectivity
			io.Print(i18n.T(i18n.KeyCmdMig_196))
			h.testEndpointConnectivity(endpoint)
			state.Endpoint = endpoint
		}

		// Step 3: Enter API key
		if state.APIKey == "" {
			io := h.io()
			defaultAPIKey := ""
			for _, m := range h.cfg.Models {
				if m.TemplateID == state.Template.ID && m.APIKey != "" {
					defaultAPIKey = m.APIKey
					break
				}
			}
			io.Println(i18n.T(i18n.KeyCmdMig_179))
			apiKey := h.wizardPromptSecret(i18n.T(i18n.KeyCmdMig_343), defaultAPIKey)
			if apiKey == "__BACK__" {
				// Go back to endpoint step — keep the endpoint value as it is
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			if strings.ToUpper(apiKey) == "Q" || strings.ToUpper(apiKey) == "QUIT" {
				return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
			}
			state.APIKey = apiKey
		}

		// Step 4: Select model name
		if state.ModelName == "" {
			io := h.io()
			io.Println(i18n.T(i18n.KeyCmdMig_185))
			var modelSuggestions []string
			modelSuggestions, state.Endpoint, state.APIModels = h.fetchModelSuggestions(state.Endpoint, state.APIKey, state.Template)
			modelName := h.wizardPromptString(i18n.T(i18n.KeyCmdMig_358), modelSuggestions, "q")
			if modelName == "__BACK__" {
				// Go back to API key step — keep the API key value
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			if modelName == "" || strings.ToUpper(modelName) == "Q" || strings.ToUpper(modelName) == "QUIT" {
				return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
			}
			state.ModelName = modelName
		}

		// Step 5: Choose capabilities
		if state.Capabilities == (config.ModelCapability{}) {
			io := h.io()
			io.Println(i18n.T(i18n.KeyCmdMig_183))
			detectedCaps := h.detectModelCapabilities(state.Endpoint, state.APIKey, state.ModelName)
			capabilities, goBack := h.wizardSelectCapabilities(detectedCaps)
			if goBack {
				state.ModelName = ""
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			state.Capabilities = capabilities
		}

		// Step 6: Enter model ID
		if state.ModelID == "" {
			io := h.io()
			defaultModelID := fmt.Sprintf("%s-%s", state.Template.ID, strings.ReplaceAll(state.ModelName, "/", "-"))
			if h.modelIDExists(defaultModelID) {
				suffix := 2
				for {
					candidate := fmt.Sprintf("%s-%d", defaultModelID, suffix)
					if !h.modelIDExists(candidate) {
						defaultModelID = candidate
						break
					}
					suffix++
				}
			}
			io.Println(i18n.T(i18n.KeyCmdMig_184))
			modelID := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_350), defaultModelID, "q")
			if modelID == "__BACK__" {
				state.Capabilities = config.ModelCapability{}
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			if strings.ToUpper(modelID) == "Q" || strings.ToUpper(modelID) == "QUIT" {
				return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
			}
			// Verify the model ID doesn't already exist — if it does, prompt again
			for h.modelIDExists(modelID) {
				h.io().Printf(i18n.T(i18n.KeyCmdMig_078), modelID)
				modelID = h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_350), defaultModelID, "q")
				if modelID == "__BACK__" {
					state.Capabilities = config.ModelCapability{}
					h.io().Println(i18n.T(i18n.KeyCmdMig_191))
					continue
				}
				if strings.ToUpper(modelID) == "Q" || strings.ToUpper(modelID) == "QUIT" {
					return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
				}
			}
			state.ModelID = modelID
		}

		// Step 7: Set priority
		if state.Priority == 0 {
			io := h.io()
			newPriority := (len(h.cfg.Models) + 1) * 10
			io.Println(i18n.T(i18n.KeyCmdMig_181))
			priorityStr := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_338)+fmt.Sprintf("%d", newPriority)+")", fmt.Sprintf("%d", newPriority), "q")
			if priorityStr == "__BACK__" {
				state.ModelID = ""
				h.io().Println(i18n.T(i18n.KeyCmdMig_191))
				continue
			}
			if strings.ToUpper(priorityStr) == "Q" || strings.ToUpper(priorityStr) == "QUIT" {
				return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
			}
			priority, err := strconv.Atoi(priorityStr)
			if err != nil {
				priority = newPriority
			}
			state.Priority = priority
		}

		// Step 8: Max model length — detect from API first
		if state.MaxModelLen == 0 {
			io := h.io()
			io.Print(i18n.T(i18n.KeyCmdMig_192))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			detectClient := llm.NewClient(state.Endpoint, state.APIKey, state.ModelName, 0, 0, 10)
			detectModels, err := detectClient.ListModels(ctx)
			cancel()
			if err != nil {
				io.Printf(i18n.T(i18n.KeyCmdMig_213), err)
				io.Println(i18n.T(i18n.KeyCmdMig_128))
			} else {
				for _, m := range detectModels {
					if m.ID == state.ModelName {
						state.MaxModelLen = m.MaxModelLen
						if state.MaxModelLen > 0 {
							io.Printf(i18n.T(i18n.KeyCmdMig_224), state.MaxModelLen)
						} else {
							io.Printf(i18n.T(i18n.KeyCmdMig_212))
						}
						break
					}
				}
			}
		}
		// Fall back to built-in known model lengths if still 0
		if state.MaxModelLen == 0 {
			state.MaxModelLen = knownMaxModelLen(state.ModelName)
		}
		// Fall back to template default if still 0
		if state.MaxModelLen == 0 && state.Template.DefaultMaxModelLen > 0 {
			state.MaxModelLen = state.Template.DefaultMaxModelLen
		}
		io := h.io()
		io.Println(i18n.T(i18n.KeyCmdMig_186))
		maxModelStr := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_352), fmt.Sprintf("%d", state.MaxModelLen), "q")
		if strings.ToUpper(maxModelStr) == "Q" || strings.ToUpper(maxModelStr) == "QUIT" {
			return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_256))
		}
		if mm, err := parseTokenCount(maxModelStr); err == nil && mm >= 0 {
			state.MaxModelLen = mm
		}

		// Step 9: Enable model
		io.Println(i18n.T(i18n.KeyCmdMig_182))
		enabled := h.wizardPromptBool(i18n.T(i18n.KeyCmdMig_286), state.Enabled)
		if !enabled {
			enabled = false
		}

		// Build and save model config
		modelConfig := &config.ModelConfig{
			ID:           state.ModelID,
			Name:         fmt.Sprintf("%s (%s)", state.Template.Name, state.ModelName),
			Provider:     state.Template.Provider,
			Endpoint:     state.Endpoint,
			Model:        state.ModelName,
			APIKey:       state.APIKey,
			Priority:     state.Priority,
			Enabled:      enabled,
			TemplateID:   state.Template.ID,
			Capabilities: state.Capabilities,
			MaxModelLen:  state.MaxModelLen,
		}

		if err := h.saveModel(modelConfig); err != nil {
			return result.String(), err
		}

		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_199), modelConfig.ID, modelConfig.Model))
		log.Info("Added model via wizard: %s (template=%s, model=%s)", modelConfig.ID, state.Template.ID, state.ModelName)
		return result.String(), nil
	}
}

// wizardSelectTemplate displays template list and lets user select one.
func (h *ModelHandler) wizardSelectTemplate() (*config.ModelTemplate, error) {
	manager := config.GetDefaultModelManager()
	templates := manager.GetAllTemplates()

	io := h.io()
	for {
		io.Print(i18n.T(i18n.KeyCmdMig_206))
		io.Printf(i18n.T(i18n.KeyCmdMig_042))

		for i, t := range templates {
			io.Printf("  [%d] %-20s %s\n", i+1, t.ID, t.Name)
			io.Printf("     %s\n", t.Description)
			if len(t.Models) > 0 {
				io.Printf(i18n.T(i18n.KeyCmdMig_012), strings.Join(t.Models, ", "))
			}
			capStr := []string{}
			if t.Capabilities.Vision {
				capStr = append(capStr, i18n.T(i18n.KeyCmdMig_377))
			}
			if t.Capabilities.ToolCall {
				capStr = append(capStr, i18n.T(i18n.KeyCmdMig_381))
			}
			if t.Capabilities.Thinking {
				capStr = append(capStr, i18n.T(i18n.KeyCmdMig_379))
			}
			if len(capStr) > 0 {
				io.Printf(i18n.T(i18n.KeyCmdMig_010), strings.Join(capStr, " "))
			}
			io.Println()
		}

		io.Print(i18n.T(i18n.KeyCmdMig_142))
		input := h.readLine()

		if input == "0" || strings.ToUpper(input) == "Q" || strings.ToUpper(input) == "QUIT" || strings.ToUpper(input) == "BACK" || strings.ToUpper(input) == ".." {
			io.Println(i18n.T(i18n.KeyCmdMig_147))
			return nil, nil
		}

		if strings.ToUpper(input) == "Q" || strings.ToUpper(input) == "QUIT" {
			return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(templates) {
			io.Println(i18n.T(i18n.KeyCmdMig_109))
			continue
		}

		selected := templates[idx-1]
		io.Printf(i18n.T(i18n.KeyCmdMig_074), selected.ID, selected.Name)
		return selected, nil
	}
}

// isDomain checks if a string looks like a domain name.
func isDomain(s string) bool {
	// Simple check: contains a dot and doesn't look like an IP
	return strings.Contains(s, ".") && !strings.Contains(s, ":") && !isIPv4(s)
}

// isIPv4 checks if a string looks like an IPv4 address (optionally with port).
func isIPv4(s string) bool {
	// Remove port if present
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// hasVNSuffix checks if the endpoint URL already contains a /vN suffix (e.g. /v1, /v2).
func hasVNSuffix(s string) bool {
	clean := strings.TrimRight(s, "/")
	return regexp.MustCompile(`/v\d+$`).MatchString(clean)
}

// extractHTTPStatusCode extracts HTTP status code from ListModels error message.
// The error format is "API returned status NNN: ..." or a network error.
// Returns 0 if the error is not an HTTP error or if err is nil.
func extractHTTPStatusCode(err error) int {
	if err == nil {
		return 0
	}
	errStr := err.Error()
	var status int
	_, scanErr := fmt.Sscanf(errStr, "API returned status %d:", &status)
	if scanErr != nil {
		return 0
	}
	return status
}

// autoCompleteEndpoint tries to fix an endpoint URL based on user input.
// It tries multiple prefix strategies and returns the first working endpoint, or the
// original endpoint if all attempts fail.
//
// Strategy:
//  1. For each prefix candidate (domain: https→http, IP: http→https):
//     a. Try the base URL (no /vN suffix). If 200/401/403 → success, return it.
//     b. If 404 and URL doesn't already have /vN → try +/v1.
//     If +/v1 returns 200/401/403 → success, return it.
//     c. If +/v1 is also 404 → skip, try next prefix.
//     d. Network error or other non-404 → skip, try next prefix.
//  2. All prefixes failed → return original input unchanged.
//
// Returns the tested endpoint and whether it succeeded.
func autoCompleteEndpoint(rawEndpoint string) (string, bool) {
	// Determine the base prefix strategies based on input type
	var baseStrategies []string

	if isIPv4(rawEndpoint) || (strings.Contains(rawEndpoint, ":") && !strings.Contains(rawEndpoint, ".") || (strings.Contains(rawEndpoint, ":") && isIPv4(strings.Split(rawEndpoint, ":")[0]))) {
		// Looks like IP address or IP:port
		baseStrategies = []string{"http://", "https://"}
	} else {
		// Looks like a domain
		baseStrategies = []string{"https://", "http://"}
	}

	// For endpoints already starting with http/https, don't add prefix again
	if strings.HasPrefix(rawEndpoint, "http://") || strings.HasPrefix(rawEndpoint, "https://") {
		baseStrategies = []string{""}
	}

	// Check if the URL already has a /vN suffix — if so, never try to add /v1
	alreadyHasVNSuffix := hasVNSuffix(rawEndpoint)

	// Try each prefix: test base URL first, then +/v1 only if base returns 404
	for _, base := range baseStrategies {
		baseURL := base + rawEndpoint

		// Step a: Try the base URL (no /vN suffix added)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client := llm.NewClient(baseURL, "", "test", 0, 0, 5)
		_, err := client.ListModels(ctx)
		cancel()

		status := extractHTTPStatusCode(err)

		if err == nil || status == http.StatusUnauthorized || status == http.StatusForbidden {
			// 200 = connected, 401/403 = endpoint exists but needs auth
			log.Info("Auto-completed endpoint: %s -> %s", rawEndpoint, baseURL)
			return baseURL, true
		}

		if status == http.StatusNotFound && !alreadyHasVNSuffix {
			// Step b: 404 on base URL, try +/v1 as a fallback
			v1URL := strings.TrimRight(baseURL, "/") + "/v1"
			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			client2 := llm.NewClient(v1URL, "", "test", 0, 0, 5)
			_, err2 := client2.ListModels(ctx2)
			cancel2()

			v1Status := extractHTTPStatusCode(err2)

			if err2 == nil || v1Status == http.StatusUnauthorized || v1Status == http.StatusForbidden {
				// +/v1 worked (200) or endpoint exists (401/403)
				log.Info("Auto-completed endpoint (with /v1 fallback): %s -> %s", rawEndpoint, v1URL)
				return v1URL, true
			}
			// Step c: +/v1 also 404 or network error → continue to next prefix
		}
		// Step d: Network error or other non-404 → continue to next prefix
	}

	log.Warn("Auto-completion failed for endpoint: %s", rawEndpoint)
	return rawEndpoint, false
}

// testEndpointConnectivity tests an endpoint and prints the result.
func (h *ModelHandler) testEndpointConnectivity(endpoint string) {
	io := h.io()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client := llm.NewClient(endpoint, "", "test", 0, 0, 10)
	models, err := client.ListModels(ctx)
	cancel()
	if err != nil {
		if !strings.Contains(err.Error(), "status") && !strings.Contains(err.Error(), "HTTP") {
			io.Printf(i18n.T(i18n.KeyCmdMig_229), err)
			io.Print(i18n.T(i18n.KeyCmdMig_111))
			retry := strings.TrimSpace(strings.ToLower(h.readLine()))
			if retry != "y" && retry != "yes" {
				return
			}
		}
	} else {
		io.Printf(i18n.T(i18n.KeyCmdMig_227), len(models))
	}
}

// fetchModelSuggestions fetches available models from the API and combines with template defaults.
// Returns the model suggestions list, the (possibly updated) endpoint URL, and the raw model info list
// (for max_model_len lookup and other downstream use).
// If the initial ListModels call fails and the endpoint doesn't have a /vN suffix,
// retries with +/v1 suffix as a fallback and updates the endpoint on success.
func (h *ModelHandler) fetchModelSuggestions(endpoint, apiKey string, template *config.ModelTemplate) ([]string, string, []llm.ModelInfo) {
	io := h.io()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client := llm.NewClient(endpoint, apiKey, "test", 0, 0, 15)
	models, err := client.ListModels(ctx)
	cancel()

	suggestions := make([]string, 0)
	seen := make(map[string]bool)

	if err != nil && !hasVNSuffix(endpoint) {
		// Stage 2: ListModels failed with API key — try +/v1 as a fallback
		retryEndpoint := strings.TrimRight(endpoint, "/") + "/v1"
		ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
		client2 := llm.NewClient(retryEndpoint, apiKey, "test", 0, 0, 15)
		models2, err2 := client2.ListModels(ctx2)
		cancel2()

		if err2 == nil {
			// +/v1 worked! Update endpoint
			io.Printf(i18n.T(i18n.KeyCmdMig_225), len(models2), retryEndpoint)
			for _, m := range models2 {
				if !seen[m.ID] {
					suggestions = append(suggestions, m.ID)
					seen[m.ID] = true
				}
			}
			// Add template defaults
			for _, m := range template.Models {
				if !seen[m] {
					suggestions = append(suggestions, m)
					seen[m] = true
				}
			}
			return suggestions, retryEndpoint, models2
		}
		// Both original and +/v1 failed, fall through to template defaults
		io.Printf(i18n.T(i18n.KeyCmdMig_214), err)
		io.Println(i18n.T(i18n.KeyCmdMig_092))
	} else if err != nil {
		io.Printf(i18n.T(i18n.KeyCmdMig_214), err)
		io.Println(i18n.T(i18n.KeyCmdMig_092))
	} else {
		io.Printf(i18n.T(i18n.KeyCmdMig_226), len(models))
		for _, m := range models {
			if !seen[m.ID] {
				suggestions = append(suggestions, m.ID)
				seen[m.ID] = true
			}
		}
	}
	for _, m := range template.Models {
		if !seen[m] {
			suggestions = append(suggestions, m)
			seen[m] = true
		}
	}
	return suggestions, endpoint, nil
}

// wizardEnterModelParams prompts user to enter model-specific parameters.
func (h *ModelHandler) wizardEnterModelParams(template *config.ModelTemplate) (*config.ModelConfig, error) {
	var result strings.Builder
	result.WriteString("\n")

	// Step 1: Enter endpoint (optional, default from template)
	defaultEndpoint := template.Endpoint
	endpoint := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_344), defaultEndpoint, "q")
	if endpoint == "__BACK__" {
		return nil, fmt.Errorf("__BACK__")
	}
	if strings.ToUpper(endpoint) == "Q" || strings.ToUpper(endpoint) == "QUIT" {
		return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
	}

	// FEATURE-172: Auto-complete endpoint if needed
	originalEndpoint := endpoint
	if endpoint != "" {
		completedEndpoint, success := autoCompleteEndpoint(endpoint)
		if success {
			endpoint = completedEndpoint
			if endpoint != originalEndpoint {
				h.io().Printf(i18n.T(i18n.KeyCmdMig_194), originalEndpoint, endpoint)
			}
		}
	}

	// Test endpoint connectivity
	io := h.io()
	io.Print(i18n.T(i18n.KeyCmdMig_196))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client := llm.NewClient(endpoint, "", "test", 0, 0, 10)
	models, err := client.ListModels(ctx)
	cancel()
	if err != nil {
		// HTTP error means connectivity is OK, no need to prompt
		if !strings.Contains(err.Error(), "status") && !strings.Contains(err.Error(), "HTTP") {
			io.Printf(i18n.T(i18n.KeyCmdMig_229), err)
			io.Print(i18n.T(i18n.KeyCmdMig_111))
			retry := strings.TrimSpace(strings.ToLower(h.readLine()))
			if retry != "y" && retry != "yes" {
				return nil, errors.New(i18n.T(i18n.KeyCmdMig_328))
			}
		}
	} else {
		io.Printf(i18n.T(i18n.KeyCmdMig_227), len(models))
	}

	// Step 2: Enter API key
	// Find existing API key from models with the same template ID,
	// fall back to config default API key
	defaultAPIKey := ""
	for _, m := range h.cfg.Models {
		if m.TemplateID == template.ID && m.APIKey != "" {
			defaultAPIKey = m.APIKey
			break
		}
	}
	// No fallback to global cfg.LLM.APIKey since it has been removed.
	// If no existing model with the same template has an API key, defaultAPIKey stays empty.
	apiKey := h.wizardPromptSecret(i18n.T(i18n.KeyCmdMig_343), defaultAPIKey)
	if apiKey == "__BACK__" {
		return nil, fmt.Errorf("__BACK__")
	}
	if strings.ToUpper(apiKey) == "Q" || strings.ToUpper(apiKey) == "QUIT" {
		return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
	}

	// Step 3: Fetch available models from API and let user select
	io.Print(i18n.T(i18n.KeyCmdMig_197))
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	client = llm.NewClient(endpoint, apiKey, "test", 0, 0, 15)
	models, err = client.ListModels(ctx)
	cancel()
	if err != nil {
		io.Printf(i18n.T(i18n.KeyCmdMig_214), err)
		io.Println(i18n.T(i18n.KeyCmdMig_092))
		models = nil
	} else {
		io.Printf(i18n.T(i18n.KeyCmdMig_226), len(models))
	}

	// Build model name suggestions: API models first, then template defaults
	modelSuggestions := make([]string, 0)
	seen := make(map[string]bool)
	if models != nil {
		for _, m := range models {
			if !seen[m.ID] {
				modelSuggestions = append(modelSuggestions, m.ID)
				seen[m.ID] = true
			}
		}
	}
	for _, m := range template.Models {
		if !seen[m] {
			modelSuggestions = append(modelSuggestions, m)
			seen[m] = true
		}
	}

	modelName := h.wizardPromptString(i18n.T(i18n.KeyCmdMig_358), modelSuggestions, "q")
	if modelName == "__BACK__" {
		return nil, fmt.Errorf("__BACK__")
	}
	if modelName == "" || strings.ToUpper(modelName) == "Q" || strings.ToUpper(modelName) == "QUIT" {
		return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
	}

	// Step 4: Look up max_model_len from the API model list
	maxModelLen := 0
	if models != nil {
		for _, m := range models {
			if m.ID == modelName {
				maxModelLen = m.MaxModelLen
				break
			}
		}
	}

	// Step 5: Auto-detect capabilities by sending test requests
	io.Print(i18n.T(i18n.KeyCmdMig_195))
	detectedCaps := h.detectModelCapabilities(endpoint, apiKey, modelName)

	// Step 6: Choose capabilities (pre-populated with detected results)
	capabilities, goBack := h.wizardSelectCapabilities(detectedCaps)
	if goBack {
		return nil, fmt.Errorf("__BACK__")
	}

	// Step 7: Enter model ID (customizable, default: templateID-modelName)
	defaultModelID := fmt.Sprintf("%s-%s", template.ID, strings.ReplaceAll(modelName, "/", "-"))
	// If default ID already exists, append a suffix number
	if h.modelIDExists(defaultModelID) {
		suffix := 2
		for {
			candidate := fmt.Sprintf("%s-%d", defaultModelID, suffix)
			if !h.modelIDExists(candidate) {
				defaultModelID = candidate
				break
			}
			suffix++
		}
	}
	modelID := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_350), defaultModelID, "q")
	if modelID == "__BACK__" {
		return nil, fmt.Errorf("__BACK__")
	}
	if strings.ToUpper(modelID) == "Q" || strings.ToUpper(modelID) == "QUIT" {
		return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
	}

	// Step 7: Set priority - default to highest priority + 10
	newPriority := (len(h.cfg.Models) + 1) * 10
	priorityStr := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_338)+fmt.Sprintf("%d", newPriority)+")", fmt.Sprintf("%d", newPriority), "q")
	if priorityStr == "__BACK__" {
		return nil, fmt.Errorf("__BACK__")
	}
	if strings.ToUpper(priorityStr) == "Q" || strings.ToUpper(priorityStr) == "QUIT" {
		return nil, errors.New(i18n.T(i18n.KeyCmdMig_256))
	}
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		priority = template.Priority
	}

	// Step 8: Enable model?
	enabled := h.wizardPromptBool(i18n.T(i18n.KeyCmdMig_286), true)
	if !enabled {
		enabled = false
	}

	return &config.ModelConfig{
		ID:           modelID,
		Name:         fmt.Sprintf("%s (%s)", template.Name, modelName),
		Provider:     template.Provider,
		Endpoint:     endpoint,
		Model:        modelName,
		APIKey:       apiKey,
		Priority:     priority,
		Enabled:      enabled,
		TemplateID:   template.ID,
		Capabilities: capabilities,
		MaxModelLen:  maxModelLen,
	}, nil
}

// modelIDExists checks if a model ID already exists in the configuration.
func (h *ModelHandler) modelIDExists(id string) bool {
	for _, m := range h.cfg.Models {
		if m.ID == id {
			return true
		}
	}
	return false
}

// readLine reads a line from UserIO.
func (h *ModelHandler) readLine() string {
	line, err := h.io().ReadLine()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// wizardPromptString prompts for a string value with template suggestions.
// Supports both numeric selection and direct text input.
// Supports back navigation: returns "__BACK__" when user enters 0/back/..
// Default value: first suggestion if available, otherwise empty.
func (h *ModelHandler) wizardPromptString(prompt string, suggestions []string, cancelKeys string) string {
	io := h.io()
	defaultVal := ""
	if len(suggestions) > 0 {
		defaultVal = suggestions[0]
	}

	for {
		if len(suggestions) > 0 {
			io.Printf("\n%s:\n", prompt)
			for i, s := range suggestions {
				io.Printf("  [%d] %s\n", i+1, s)
			}
			io.Printf(i18n.T(i18n.KeyCmdMig_143), defaultVal)
		} else {
			io.Printf("\n%s: ", prompt)
		}

		input := h.readLine()
		if input == "" && defaultVal == "" {
			io.Println(i18n.T(i18n.KeyCmdMig_144))
			continue
		}

		// Check back/cancel keys
		upper := strings.ToUpper(input)
		if input != "" && (upper == "Q" || upper == "QUIT") {
			return ""
		}
		if input == "0" || upper == "BACK" || input == ".." {
			return "__BACK__"
		}

		// Empty input: use default
		if input == "" && defaultVal != "" {
			io.Printf(i18n.T(i18n.KeyCmdMig_085), defaultVal)
			return defaultVal
		}

		// Check if user selected a suggestion by number
		if len(suggestions) > 0 {
			if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(suggestions) {
				return suggestions[idx-1]
			}
		}

		// Direct text input
		return input
	}
}

// wizardPromptStringWithDefault prompts for a string value with a default.
// Supports back navigation: returns "__BACK__" when user enters 0/back/.., and "Q"/"QUIT" for cancel.
func (h *ModelHandler) wizardPromptStringWithDefault(prompt string, defaultValue string, cancelKeys string) string {
	io := h.io()
	for {
		io.Printf(i18n.T(i18n.KeyCmdMig_198), prompt, defaultValue)
		input := h.readLine()

		// Check back/cancel keys
		upper := strings.ToUpper(input)
		if input != "" && (upper == "Q" || upper == "QUIT") {
			return "Q"
		}
		if input == "0" || upper == "BACK" || input == ".." {
			return "__BACK__"
		}

		if input == "" {
			io.Printf(i18n.T(i18n.KeyCmdMig_085), defaultValue)
			return defaultValue
		}

		return input
	}
}

// wizardPromptSecret prompts for a secret value (API key).
// If defaultVal is provided, it will be shown as masked default.
// Supports back navigation: returns "__BACK__" when user enters 0/back/..
func (h *ModelHandler) wizardPromptSecret(prompt string, defaultVal string) string {
	io := h.io()
	for {
		if defaultVal != "" {
			// Mask the API key: show first 4 chars + **** + last 4 chars
			// If the key is shorter than 8 chars, just show ****
			var masked string
			if len(defaultVal) >= 8 {
				masked = defaultVal[:4] + "****" + defaultVal[len(defaultVal)-4:]
			} else {
				masked = "****"
			}
			io.Printf(i18n.T(i18n.KeyCmdMig_198), prompt, masked)
		} else {
			io.Printf("\n%s: ", prompt)
		}

		input := h.readLine()

		// Check back/cancel keys
		upper := strings.ToUpper(input)
		if input != "" && (upper == "Q" || upper == "QUIT") {
			return "Q"
		}
		if input == "0" || upper == "BACK" || input == ".." {
			return "__BACK__"
		}

		if input == "" {
			if defaultVal != "" {
				io.Println(i18n.T(i18n.KeyCmdMig_084))
				return defaultVal
			}
			io.Println(i18n.T(i18n.KeyCmdMig_040))
			return ""
		}

		return input
	}
}

// wizardPromptBool prompts for a yes/no answer.
func (h *ModelHandler) wizardPromptBool(prompt string, defaultVal bool) bool {
	io := h.io()
	for {
		defaultStr := "y"
		if !defaultVal {
			defaultStr = "n"
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_198), prompt, defaultStr)

		input := strings.TrimSpace(strings.ToLower(h.readLine()))

		if input == "" {
			io.Printf(i18n.T(i18n.KeyCmdMig_085), defaultStr)
			return defaultVal
		}

		switch input {
		case "y", "yes", i18n.T(i18n.KeyCmdMig_284), "yep", "yeah":
			return true
		case "n", "no", i18n.T(i18n.KeyCmdMig_257), "nope":
			return false
		default:
			io.Println(i18n.T(i18n.KeyCmdMig_108))
		}
	}
}

// detectModelCapabilities auto-detects model capabilities by sending test requests.
// Tests vision, tool call, and thinking support.
func (h *ModelHandler) detectModelCapabilities(endpoint, apiKey, modelName string) config.ModelCapability {
	io := h.io()
	caps := config.ModelCapability{}

	testKey := apiKey

	client := llm.NewClient(endpoint, testKey, modelName, 0, 0, 30)
	defer client.Close()

	// Force-enable LLM interaction logging for capability tests
	wasOn := log.IsLLMInteractionEnabled()
	log.SetLLMInteractionEnabled(true)
	defer log.SetLLMInteractionEnabled(wasOn)

	// Determine provider from existing model configs for provider-specific thinking params
	testProvider := ""
	for _, m := range h.cfg.Models {
		if m.Model == modelName {
			testProvider = m.Provider
			break
		}
	}
	testAdapter := llm.GetThinkingAdapter(testProvider)

	// Test 1-3: disable thinking using provider-specific format
	disableAdditions := testAdapter.BuildAdditions(llm.ThinkingConfig{
		Mode: llm.ThinkingModeDisabled,
	})
	if len(disableAdditions) > 0 {
		client.SetBodyAdditions(disableAdditions)
	}

	// Test vision support
	io.Print(i18n.T(i18n.KeyCmdMig_154))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	vision := client.TestVisionSupport(ctx)
	cancel()
	if vision {
		io.Println(i18n.T(i18n.KeyCmdMig_223))
		caps.Vision = true
	} else {
		io.Println(i18n.T(i18n.KeyCmdMig_228))
	}

	// Test tool call support
	io.Print(i18n.T(i18n.KeyCmdMig_162))
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	toolCall := client.TestToolCallSupport(ctx)
	cancel()
	if toolCall {
		io.Println(i18n.T(i18n.KeyCmdMig_223))
		caps.ToolCall = true
	} else {
		io.Println(i18n.T(i18n.KeyCmdMig_228))
	}

	// Test thinking support: switch to thinking-enabled params
	io.Print(i18n.T(i18n.KeyCmdMig_155))
	enableAdditions := testAdapter.BuildAdditions(llm.ThinkingConfig{
		Mode:            llm.ThinkingModeEnabled,
		ReasoningEffort: "low",
	})
	if len(enableAdditions) > 0 {
		client.SetBodyAdditions(enableAdditions)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	thinking := client.TestThinkingSupport(ctx)
	cancel()
	if thinking {
		io.Println(i18n.T(i18n.KeyCmdMig_223))
		caps.Thinking = true
	} else {
		io.Println(i18n.T(i18n.KeyCmdMig_228))
	}

	return caps
}

// wizardSelectCapabilities lets user review and adjust model capabilities.
// Shows detected capabilities and allows toggling.
// Returns capabilities and whether user chose to go back.
func (h *ModelHandler) wizardSelectCapabilities(base config.ModelCapability) (config.ModelCapability, bool) {
	io := h.io()
	caps := config.ModelCapability{
		Vision:   base.Vision,
		ToolCall: base.ToolCall,
		Thinking: base.Thinking,
	}

	for {
		io.Println(i18n.T(i18n.KeyCmdMig_204))
		io.Println(i18n.T(i18n.KeyCmdMig_046))
		io.Println(i18n.T(i18n.KeyCmdMig_049))
		io.Println(i18n.T(i18n.KeyCmdMig_052))
		io.Printf(i18n.T(i18n.KeyCmdMig_176))
		if caps.Vision {
			io.Print("👁 ")
		}
		if caps.ToolCall {
			io.Print("🔧 ")
		}
		if caps.Thinking {
			io.Print("💭 ")
		}
		io.Println()
		io.Print(i18n.T(i18n.KeyCmdMig_140))

		input := h.readLine()

		if input == "" {
			return caps, false
		}

		if input == "0" || strings.ToUpper(input) == "BACK" || strings.ToUpper(input) == ".." {
			return caps, true
		}

		switch input {
		case "1":
			caps.Vision = !caps.Vision
		case "2":
			caps.ToolCall = !caps.ToolCall
		case "3":
			caps.Thinking = !caps.Thinking
		default:
			io.Println(i18n.T(i18n.KeyCmdMig_107))
		}
	}
}

// saveModel adds and saves a model configuration.
// If priority conflicts with an existing model, the new model is inserted before it.
// After insertion, all model priorities are re-encoded: lowest = 10, each step +10.
// If the added model has vision capability, automatically enable global vision support.
func (h *ModelHandler) saveModel(model *config.ModelConfig) error {
	// Check for duplicate ID
	for _, m := range h.cfg.Models {
		if m.ID == model.ID {
			return fmt.Errorf(i18n.T(i18n.KeyCmdMig_306), model.ID)
		}
	}

	// Insert model at the correct position based on priority
	inserted := false
	for i, m := range h.cfg.Models {
		if model.Priority >= m.Priority {
			// Insert before this model (same priority = insert before)
			h.cfg.Models = append(h.cfg.Models[:i], append([]*config.ModelConfig{model}, h.cfg.Models[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		// Append at the end (lowest priority)
		h.cfg.Models = append(h.cfg.Models, model)
	}

	// Re-encode priorities: lowest = 10, each step +10
	h.reencodePriorities()

	if err := h.cfg.Save(); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
	}

	// FEATURE-171: If the added model has vision capability, automatically enable
	// global vision support to improve user experience.
	if model.Capabilities.Vision && !h.cfg.LLM.VisionSupport {
		h.cfg.LLM.VisionSupport = true
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to enable global vision support: %v", err)
		} else {
			log.Info("Auto-enabled global vision support due to model with vision capability: %s", model.ID)
		}
	}

	// FIX-183: Sync to ModelManager so selectModelForCall() can find the new model
	h.syncModelsToManager()

	return nil
}

// reencodePriorities re-encodes all model priorities so that the lowest = 10,
// and each higher priority model gets +10.
func (h *ModelHandler) reencodePriorities() {
	// Sort by priority descending
	for i := 0; i < len(h.cfg.Models); i++ {
		for j := i + 1; j < len(h.cfg.Models); j++ {
			if h.cfg.Models[j].Priority > h.cfg.Models[i].Priority {
				h.cfg.Models[i], h.cfg.Models[j] = h.cfg.Models[j], h.cfg.Models[i]
			}
		}
	}

	// Re-encode: lowest = 10, each step +10
	n := len(h.cfg.Models)
	for i := 0; i < n; i++ {
		h.cfg.Models[n-1-i].Priority = (i + 1) * 10
	}
}

// editModelWizard starts an interactive wizard to edit an existing model's basic parameters.
// User selects a model, then goes through similar steps as adding (endpoint, api_key, model name, priority, capabilities)
// but with existing values pre-filled as defaults.
func (h *ModelHandler) editModelWizard(args []string) (string, error) {
	var modelID string
	if len(args) > 0 {
		modelID = args[0]
	} else {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_365))
		if err != nil {
			return "", err
		}
	}

	// Find existing model
	var model *config.ModelConfig
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			model = m
			break
		}
	}
	if model == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
	}

	io := h.io()
	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_161), modelID))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	// Step 0: Test API connectivity and detect max_model_len
	io.Println(i18n.T(i18n.KeyCmdMig_193))
	detectedMaxModelLen := 0
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client := llm.NewClient(model.Endpoint, model.APIKey, model.Model, 0, 0, 15)
	models, err := client.ListModels(ctx)
	cancel()
	if err != nil {
		io.Printf(i18n.T(i18n.KeyCmdMig_069), err)
		io.Println(i18n.T(i18n.KeyCmdMig_091))
	} else {
		io.Printf(i18n.T(i18n.KeyCmdMig_075), len(models))
		for _, m := range models {
			if m.ID == model.Model {
				detectedMaxModelLen = m.MaxModelLen
				if detectedMaxModelLen > 0 {
					io.Printf(i18n.T(i18n.KeyCmdMig_115), m.ID, detectedMaxModelLen)
				}
				break
			}
		}
		if detectedMaxModelLen == 0 {
			io.Println(i18n.T(i18n.KeyCmdMig_097))
		}
	}

	// Step 1: Endpoint (default = current value)
	io.Println(i18n.T(i18n.KeyCmdMig_043))
	endpoint := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_344), model.Endpoint, "q")
	if strings.ToUpper(endpoint) == "Q" || strings.ToUpper(endpoint) == "QUIT" {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}

	// Step 2: API key (default = current, masked)
	io.Println("\n  [2/7] API Key")
	apiKey := h.wizardPromptSecret(i18n.T(i18n.KeyCmdMig_343), model.APIKey)
	if strings.ToUpper(apiKey) == "Q" || strings.ToUpper(apiKey) == "QUIT" {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}

	// Step 3: Model name (default = current)
	io.Println(i18n.T(i18n.KeyCmdMig_170))
	modelName := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_351), model.Model, "q")
	if strings.ToUpper(modelName) == "Q" || strings.ToUpper(modelName) == "QUIT" {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}

	// Step 4: Priority (default = current)
	io.Println(i18n.T(i18n.KeyCmdMig_171))
	priorityStr := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_337), fmt.Sprintf("%d", model.Priority), "q")
	if strings.ToUpper(priorityStr) == "Q" || strings.ToUpper(priorityStr) == "QUIT" {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		priority = model.Priority
	}

	// Step 5: Max model length — detect from API with current parameters
	io.Println(i18n.T(i18n.KeyCmdMig_172))
	io.Print(i18n.T(i18n.KeyCmdMig_192))
	detectCtx, detectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	detectClient := llm.NewClient(endpoint, apiKey, modelName, 0, 0, 10)
	detectModels, detectErr := detectClient.ListModels(detectCtx)
	detectCancel()
	newDetectedMax := 0
	if detectErr != nil {
		io.Printf(i18n.T(i18n.KeyCmdMig_213), detectErr)
		io.Println(i18n.T(i18n.KeyCmdMig_128))
	} else {
		for _, m := range detectModels {
			if m.ID == modelName {
				newDetectedMax = m.MaxModelLen
				if newDetectedMax > 0 {
					io.Printf(i18n.T(i18n.KeyCmdMig_224), newDetectedMax)
				} else {
					io.Printf(i18n.T(i18n.KeyCmdMig_212))
				}
				break
			}
		}
	}
	maxModelLenDefault := model.MaxModelLen
	if maxModelLenDefault == 0 && newDetectedMax > 0 {
		maxModelLenDefault = newDetectedMax
	}
	if maxModelLenDefault == 0 {
		// Try built-in known model lengths
		maxModelLenDefault = knownMaxModelLen(modelName)
	}
	if maxModelLenDefault == 0 {
		// Try template default
		if tpl := config.GetDefaultModelManager().GetTemplate(model.TemplateID); tpl != nil && tpl.DefaultMaxModelLen > 0 {
			maxModelLenDefault = tpl.DefaultMaxModelLen
		}
	}
	maxModelLenStr := h.wizardPromptStringWithDefault(i18n.T(i18n.KeyCmdMig_352), fmt.Sprintf("%d", maxModelLenDefault), "q")
	if strings.ToUpper(maxModelLenStr) == "Q" || strings.ToUpper(maxModelLenStr) == "QUIT" {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}
	maxModelLen := 0
	if mm, er := parseTokenCount(maxModelLenStr); er == nil && mm >= 0 {
		maxModelLen = mm
	}

	// Step 6: Capabilities (default = current)
	io.Println(i18n.T(i18n.KeyCmdMig_173))
	capabilities, goBack := h.wizardSelectCapabilities(model.Capabilities)
	if goBack {
		return result.String(), errors.New(i18n.T(i18n.KeyCmdMig_259))
	}

	// Step 7: Enabled state (default = current)
	io.Println(i18n.T(i18n.KeyCmdMig_174))
	enabled := h.wizardPromptBool(i18n.T(i18n.KeyCmdMig_285), model.Enabled)

	// Apply changes
	model.Endpoint = endpoint
	model.APIKey = apiKey
	model.Model = modelName
	model.Priority = priority
	model.MaxModelLen = maxModelLen
	model.Capabilities = capabilities
	model.Enabled = enabled

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
	}

	// Sync to ModelManager
	h.syncModelsToManager()

	// Rebuild LLM client if this is the active model
	if h.agent != nil {
		h.agent.ApplyWorkModeConfig()
	}

	log.Info("Edited model: %s (endpoint=%s, model=%s)", modelID, endpoint, modelName)
	result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_200), modelID))
	return result.String(), nil
}

// addFromTemplate adds a model from a built-in template.
func (h *ModelHandler) addFromTemplate(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_324))
	}

	templateID := args[0]
	modelName := args[1]

	var apiKey string
	for i := 2; i < len(args); i++ {
		if args[i] == "--api-key" && i+1 < len(args) {
			apiKey = args[i+1]
			i++
		}
	}

	manager := config.GetDefaultModelManager()
	template := manager.GetTemplate(templateID)
	if template == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_312), templateID)
	}

	modelID := fmt.Sprintf("%s-%s", templateID, strings.ReplaceAll(modelName, "/", "-"))

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_305), modelID)
		}
	}

	newModel := &config.ModelConfig{
		ID:           modelID,
		Name:         fmt.Sprintf("%s (%s)", template.Name, modelName),
		Provider:     template.Provider,
		Endpoint:     template.Endpoint,
		Model:        modelName,
		APIKey:       apiKey,
		Priority:     template.Priority,
		Enabled:      false,
		TemplateID:   templateID,
		Capabilities: template.Capabilities,
	}

	h.cfg.Models = append(h.cfg.Models, newModel)

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
	}

	// FIX-183: Sync to ModelManager so selectModelForCall() can find the new model
	h.syncModelsToManager()

	log.Info("Added model from template: %s (template=%s, model=%s)", modelID, templateID, modelName)
	return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_215), template.Name, modelID, modelName), nil
}

// selectModelByNumber displays a numbered list of models and prompts the user to select one.
// Returns the selected model ID, or an error if cancelled.
// If there are no models configured, returns an error.
func (h *ModelHandler) selectModelByNumber(prompt string) (string, error) {
	io := h.io()
	models := h.cfg.Models
	if len(models) == 0 {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_294))
	}

	// Sort by priority descending
	sorted := make([]*config.ModelConfig, len(models))
	copy(sorted, models)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	io.Printf("\n%s:\n\n", prompt)
	for i, m := range sorted {
		status := "⬜"
		if m.Enabled {
			status = "✅"
		}
		capStr := []string{}
		if m.Capabilities.Vision {
			capStr = append(capStr, "👁")
		}
		if m.Capabilities.ToolCall {
			capStr = append(capStr, "🔧")
		}
		if m.Capabilities.Thinking {
			capStr = append(capStr, "💭")
		}
		capsDisplay := strings.Join(capStr, "")
		io.Printf("  [%d] %s %s [%s][%s:%s]",
			i+1, status, m.ID, m.Provider, m.Endpoint, m.Model)
		if capsDisplay != "" {
			io.Printf("[%s]", capsDisplay)
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_163), m.Priority)
	}
	io.Print(i18n.T(i18n.KeyCmdMig_189))

	input := h.readLine()
	if input == "" || input == "0" || strings.ToUpper(input) == "Q" || strings.ToUpper(input) == "QUIT" {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_259))
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(sorted) {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_282))
	}

	return sorted[idx-1].ID, nil
}

// removeModel removes a model configuration.
// If the removed model had vision capability and no remaining models have vision capability,
// automatically disable global vision support.
func (h *ModelHandler) removeModel(args []string) (string, error) {
	var modelID string
	if len(args) == 0 {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_364))
		if err != nil {
			return "", err
		}
	} else {
		modelID = args[0]
	}

	for i, m := range h.cfg.Models {
		if m.ID == modelID {
			// Check if the removed model had vision capability
			hadVision := m.Capabilities.Vision

			h.cfg.Models = append(h.cfg.Models[:i], h.cfg.Models[i+1:]...)
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
			}

			// FEATURE-171: If the removed model had vision capability, check if
			// any remaining models still have vision capability. If not, disable
			// global vision support to keep config consistent.
			if hadVision && h.cfg.LLM.VisionSupport {
				hasRemainingVision := false
				for _, rm := range h.cfg.Models {
					if rm.Capabilities.Vision {
						hasRemainingVision = true
						break
					}
				}
				if !hasRemainingVision {
					h.cfg.LLM.VisionSupport = false
					if err := h.cfg.Save(); err != nil {
						log.Warn("Failed to disable global vision support: %v", err)
					} else {
						log.Info("Disabled global vision support: no remaining models with vision capability after removing %s", modelID)
					}
				}
			}

			// FIX-183: Sync to ModelManager so selectModelForCall() no longer uses the removed model
			h.syncModelsToManager()

			log.Info("Removed model: %s", modelID)
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_221), modelID), nil
		}
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
}

// switchModel switches to a specific model by moving it to the front of the queue
// (highest priority) and re-encoding all priorities.
func (h *ModelHandler) switchModel(args []string) (string, error) {
	var modelID string
	if len(args) == 0 {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_360))
		if err != nil {
			return "", err
		}
	} else {
		modelID = args[0]
	}

	// Find the target model index
	targetIdx := -1
	for i, m := range h.cfg.Models {
		if m.ID == modelID {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
	}

	// Move the target model to the front of the slice
	target := h.cfg.Models[targetIdx]
	h.cfg.Models = append([]*config.ModelConfig{target}, append(h.cfg.Models[:targetIdx], h.cfg.Models[targetIdx+1:]...)...)

	// Assign priorities: target gets highest, others get lower values
	// This avoids reencodePriorities which would re-sort and lose the target's position
	n := len(h.cfg.Models)
	for i := 0; i < n; i++ {
		h.cfg.Models[n-1-i].Priority = (i + 1) * 10
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
	}

	// Sync to ModelManager so applyWorkModeConfig() can find the switched model
	h.syncModelsToManager()

	// Rebuild LLM client using ApplyWorkModeConfig for consistent parameter resolution
	// (mode overrides > model config > global defaults)
	if h.agent != nil {
		h.agent.ApplyWorkModeConfig()
	}

	log.Info("Switched to model: %s (priority=%d)", modelID, h.cfg.Models[0].Priority)
	return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_216), modelID, h.cfg.Models[0].Priority), nil
}

// enableModel enables a specific model.
func (h *ModelHandler) enableModel(args []string) (string, error) {
	var modelID string
	if len(args) == 0 {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_361))
		if err != nil {
			return "", err
		}
	} else {
		modelID = args[0]
	}

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Enabled = true
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
			}

			// FIX-183: Sync to ModelManager so selectModelForCall() reflects the enabled state
			h.syncModelsToManager()

			log.Info("Enabled model: %s", modelID)
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_217), modelID), nil
		}
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
}

// disableModel disables a specific model.
func (h *ModelHandler) disableModel(args []string) (string, error) {
	var modelID string
	if len(args) == 0 {
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_363))
		if err != nil {
			return "", err
		}
	} else {
		modelID = args[0]
	}

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Enabled = false
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
			}

			// FIX-183: Sync to ModelManager so selectModelForCall() reflects the disabled state
			h.syncModelsToManager()

			log.Info("Disabled model: %s", modelID)
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_219), modelID), nil
		}
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
}

// setPriority sets the priority of a model.
func (h *ModelHandler) setPriority(args []string) (string, error) {
	if len(args) < 1 {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_326))
	}

	var modelID string
	if len(args) >= 1 {
		modelID = args[0]
	}
	if len(args) < 2 {
		// Only model ID provided, need priority value too
		// Try interactive selection for model ID
		var err error
		modelID, err = h.selectModelByNumber(i18n.T(i18n.KeyCmdMig_366))
		if err != nil {
			return "", err
		}
		// Prompt for priority value
		h.io().Print(i18n.T(i18n.KeyCmdMig_347))
		priorityStr := h.readLine()
		if priorityStr == "" {
			return "", errors.New(i18n.T(i18n.KeyCmdMig_259))
		}
		priority, err := strconv.Atoi(priorityStr)
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_279), priorityStr)
		}
		return h.setPriorityForModel(modelID, priority)
	}
	priority, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_279), args[1])
	}

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Priority = priority
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
			}

			// FIX-183: Sync to ModelManager so selectModelForCall() reflects the new priority
			h.syncModelsToManager()

			log.Info("Set priority for model %s to %d", modelID, priority)
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_218), modelID, priority), nil
		}
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
}

// setPriorityForModel sets the priority of a model and saves the configuration.
func (h *ModelHandler) setPriorityForModel(modelID string, priority int) (string, error) {
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Priority = priority
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
			}

			// FIX-183: Sync to ModelManager so selectModelForCall() reflects the new priority
			h.syncModelsToManager()

			log.Info("Set priority for model %s to %d", modelID, priority)
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_218), modelID, priority), nil
		}
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
}

// listTemplates lists all built-in templates.
func (h *ModelHandler) listTemplates() (string, error) {
	manager := config.GetDefaultModelManager()
	templates := manager.GetAllTemplates()

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString(i18n.T(i18n.KeyCmdMig_156))
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	for _, t := range templates {
		capStr := []string{}
		if t.Capabilities.Vision {
			capStr = append(capStr, "👁")
		}
		if t.Capabilities.ToolCall {
			capStr = append(capStr, "🔧")
		}
		if t.Capabilities.Thinking {
			capStr = append(capStr, "💭")
		}

		result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_041), t.ID, t.Name, t.Priority))
		result.WriteString(fmt.Sprintf("     %s\n", t.Description))
		if len(t.Models) > 0 {
			result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_012), strings.Join(t.Models, ", ")))
		}
		if len(capStr) > 0 {
			result.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_010), strings.Join(capStr, " ")))
		}
		result.WriteString("\n")
	}

	result.WriteString(i18n.T(i18n.KeyCmdMig_083))
	result.WriteString(i18n.T(i18n.KeyCmdMig_099))
	return result.String(), nil
}

// knownMaxModelLen returns a known default max context length for well-known model IDs.
// Used as fallback when the API does not return max_model_len.
func knownMaxModelLen(modelID string) int {
	known := map[string]int{
		"deepseek-v4-flash": 1048576,
		"deepseek-v4-pro":   1048576,
		"deepseek-v4":       1048576,
		"deepseek":          262144,
		"deepseek-chat":     262144,
		"deepseek-r1":       65536,
		"deepseek-v3":       65536,
		"gpt-4o":            128000,
		"gpt-4o-mini":       128000,
		"gpt-4-turbo":       128000,
		"gpt-3.5-turbo":     16385,
		"qwen-plus":         131072,
		"qwen-max":          131072,
		"qwen-turbo":        1000000,
		"qwen-vl-max":       131072,
		"glm-4":             131072,
		"mimo-v2":           65536,
	}
	lower := strings.ToLower(modelID)
	if v, ok := known[lower]; ok {
		return v
	}
	// Prefix match: longest prefix wins
	matchLen := 0
	matchVal := 0
	for prefix, v := range known {
		if strings.HasPrefix(lower, prefix) && len(prefix) > matchLen {
			matchLen = len(prefix)
			matchVal = v
		}
	}
	return matchVal
}

// parseTokenCount parses a token count string that may include unit suffixes.
// Supported formats: "65536", "64K", "1M", "128k", "1m"
// Returns the integer count and nil error on success.
func parseTokenCount(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	lower := strings.ToLower(s)
	multiplier := 1
	if strings.HasSuffix(lower, "k") {
		multiplier = 1000
		s = strings.TrimSuffix(s, "k")
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(lower, "m") {
		multiplier = 1000000
		s = strings.TrimSuffix(s, "m")
		s = strings.TrimSuffix(s, "M")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}

// setParam sets a custom parameter for a model.
// Usage: .model set-param <modelID> <key> <value>
// If value is "None", the parameter is removed (not sent to LLM).
// If value is a valid JSON string, it is stored as a parsed JSON value.
// Otherwise, it is stored as a plain string.
func (h *ModelHandler) setParam(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_325))
	}

	modelID := args[0]
	key := args[1]
	value := strings.Join(args[2:], " ")

	// Find the model
	var model *config.ModelConfig
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			model = m
			break
		}
	}

	if model == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_303), modelID)
	}

	// Initialize CustomParams map if nil
	if model.CustomParams == nil {
		model.CustomParams = make(map[string]interface{})
	}

	// Handle "None" value: remove the parameter
	if value == "None" {
		delete(model.CustomParams, key)
		if len(model.CustomParams) == 0 {
			model.CustomParams = nil
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
		}
		log.Info("Removed custom param %s from model %s", key, modelID)
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_220), modelID, key), nil
	}

	// Try to parse as JSON first
	var parsedValue interface{}
	if err := json.Unmarshal([]byte(value), &parsedValue); err == nil {
		model.CustomParams[key] = parsedValue
	} else {
		// Store as plain string
		model.CustomParams[key] = value
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_237), err)
	}

	log.Info("Set custom param %s=%v for model %s", key, model.CustomParams[key], modelID)
	return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_222), modelID, key, model.CustomParams[key]), nil
}
