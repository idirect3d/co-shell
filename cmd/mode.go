// Author: L.Shuang
// Created: 2026-06-03
// Last Modified: 2026-06-06
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
// copies or portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
)

// ModeHandler handles the .mode built-in command.
type ModeHandler struct {
	cfg *config.Config
	ag  *agent.Agent
}

// NewModeHandler creates a new ModeHandler.
func NewModeHandler(cfg *config.Config, ag *agent.Agent) *ModeHandler {
	return &ModeHandler{cfg: cfg, ag: ag}
}

// io returns the UserIO from the agent, falling back to DefaultUserIO.
func (h *ModeHandler) io() agent.UserIO {
	return agent.GetIO(h.ag)
}

// Handle processes .mode commands.
// When called without arguments, it enters the interactive wizard.
// Subcommands remain available as shortcuts.
func (h *ModeHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		h.runWizard()
		return "", nil
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return h.listModes(), nil
	case "switch":
		return h.interactiveSwitch(args[1:])
	case "create":
		return h.interactiveCreate()
	case "edit":
		return h.interactiveEdit(args[1:])
	case "remove", "rm":
		return h.interactiveRemove(args[1:])
	default:
		// Check for .mode <name> tools [<method> <value>]
		if len(args) >= 2 && args[1] == "tools" {
			return h.handleModeTools(subcommand, args[2:])
		}
		// Check for .mode <name> model [subcommand]
		if len(args) >= 2 && args[1] == "model" {
			return h.handleModeModel(subcommand, args[2:])
		}
		// Check for .mode <name> param [subcommand]
		if len(args) >= 2 && args[1] == "param" {
			return h.handleModeParam(subcommand, args[2:])
		}
		return "", fmt.Errorf("unknown mode subcommand: %s", subcommand)
	}
}

// readLine reads a line from UserIO.
func (h *ModeHandler) readLine() string {
	line, err := h.io().ReadLine()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// modeModelInfo holds the resolved model display info for a work mode.
type modeModelInfo struct {
	textID         string
	textProvider   string
	textCaps       string // 👁🔧💭
	visionID       string
	visionProvider string
	visionCaps     string
	sameAsText     bool // vision uses same model as text
	problemID      string
	sameAsTextPro  bool // problem uses same model as text
	problemCaps    string
}

// resolveModeModel resolves the actual model(s) a mode would use at runtime.
// It applies the same logic as agent.getModelIDForCall() but for display purposes.
// Returns empty strings if no model is available.
func (h *ModeHandler) resolveModeModel(mode *config.WorkMode) modeModelInfo {
	info := modeModelInfo{}

	// Resolve text model
	textModelID := ""
	if mode.ModelID != nil {
		textModelID = *mode.ModelID
	}
	// Find the actual ModelConfig
	var textCfg *config.ModelConfig
	for _, m := range h.cfg.Models {
		if textModelID != "" && m.ID == textModelID && m.Enabled {
			textCfg = m
			break
		}
	}
	// If mode has no ModelID or it's not found/enabled, use global highest priority
	if textCfg == nil {
		for _, m := range h.cfg.Models {
			if m.Enabled && (textCfg == nil || m.Priority > textCfg.Priority) {
				textCfg = m
			}
		}
	}
	if textCfg != nil {
		info.textID = textCfg.ID
		info.textProvider = textCfg.Provider
		if textCfg.Capabilities.Vision {
			info.textCaps += "👁"
		}
		if textCfg.Capabilities.ToolCall {
			info.textCaps += "🔧"
		}
		if textCfg.Capabilities.Thinking {
			info.textCaps += "💭"
		}
	} else {
		info.textID = i18n.T(i18n.KeyCmdMig_166)
		return info
	}

	// Resolve vision model
	visionModelID := ""
	if mode.VisionModelID != nil {
		visionModelID = *mode.VisionModelID
	}
	var visionCfg *config.ModelConfig
	if visionModelID != "" {
		for _, m := range h.cfg.Models {
			if m.ID == visionModelID && m.Enabled {
				visionCfg = m
				break
			}
		}
	}
	if visionCfg != nil {
		info.visionID = visionCfg.ID
		info.visionProvider = visionCfg.Provider
		if visionCfg.Capabilities.Vision {
			info.visionCaps += "👁"
		}
		if visionCfg.Capabilities.ToolCall {
			info.visionCaps += "🔧"
		}
		if visionCfg.Capabilities.Thinking {
			info.visionCaps += "💭"
		}
	} else if textCfg.Capabilities.Vision {
		info.sameAsText = true
		info.visionID = textCfg.ID
		info.visionProvider = textCfg.Provider
		info.visionCaps = info.textCaps
	} else {
		// Try to find any enabled model with vision capability
		for _, m := range h.cfg.Models {
			if m.Enabled && m.Capabilities.Vision {
				info.visionID = m.ID
				info.visionProvider = m.Provider
				if m.Capabilities.Vision {
					info.visionCaps += "👁"
				}
				if m.Capabilities.ToolCall {
					info.visionCaps += "🔧"
				}
				if m.Capabilities.Thinking {
					info.visionCaps += "💭"
				}
				break
			}
		}
	}
	if info.visionID == "" {
		info.visionID = i18n.T(i18n.KeyCmdMig_167)
	}

	// Resolve problem-solving model
	problemModelID := ""
	if mode.ProblemModelID != nil {
		problemModelID = *mode.ProblemModelID
	}
	if problemModelID != "" {
		// Find by ID
		for _, m := range h.cfg.Models {
			if m.ID == problemModelID && m.Enabled {
				info.problemID = m.ID
				if m.Capabilities.Vision {
					info.problemCaps += "👁"
				}
				if m.Capabilities.ToolCall {
					info.problemCaps += "🔧"
				}
				if m.Capabilities.Thinking {
					info.problemCaps += "💭"
				}
				break
			}
		}
	}
	if info.problemID == "" && problemModelID == "" {
		info.sameAsTextPro = true
		info.problemID = textCfg.ID
		info.problemCaps = info.textCaps
	}

	return info
}

// runWizard runs the interactive mode management wizard.
func (h *ModeHandler) runWizard() {
	io := h.io()
	for {
		h.showModeOverview()
		io.Print(i18n.T(i18n.KeyCmdMig_356))
		input := strings.ToUpper(strings.TrimSpace(h.readLine()))

		if input == "Q" || input == "QUIT" || input == ".." {
			return
		}
		if input == "B" || input == "BACK" {
			return
		}
		if input == "C" || input == "CREATE" {
			h.interactiveCreateWizard()
			continue
		}
		if input == "S" || input == "SWITCH" {
			// Interactive switch: select a mode by number
			selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_369))
			if err == nil {
				h.doSwitch(selected.Name)
			}
			continue
		}
		if input == "D" || input == "DELETE" {
			h.interactiveRemoveWizard()
			continue
		}
		if input == "E" || input == "EDIT" {
			selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_374))
			if err == nil {
				h.showModeDetail(selected.Name)
			}
			continue
		}

		// Try number: switch to that mode directly
		num, err := strconv.Atoi(input)
		if err == nil && num >= 1 {
			modes := h.getAllModes()
			if num <= len(modes) {
				h.doSwitch(modes[num-1].Name)
				return
			}
		}
		io.Println(i18n.T(i18n.KeyCmdMig_107))
	}
}

// showModeOverview displays the first-level menu listing all modes.
func (h *ModeHandler) showModeOverview() {
	io := h.io()
	io.Println()
	io.Println(i18n.T(i18n.KeyCmdMig_208))
	io.Println()

	modes := h.getAllModes()
	current := h.cfg.LLM.WorkMode
	if current == "" || current == "default" {
		current = "act"
	}

	for i, m := range modes {
		marker := " "
		if m.Name == current {
			marker = "*"
		}
		// Show mode name + description on first line
		desc := m.Description
		if desc == "" {
			desc = "-"
		}
		io.Printf("  %s [%d] %s: %s\n", marker, i+1, m.Name, desc)

		// Show resolved model info on second and third lines
		modelInfo := h.resolveModeModel(&m)
		// Text model line
		if modelInfo.textID != "" {
			capTxt := ""
			if modelInfo.textCaps != "" {
				capTxt = " " + modelInfo.textCaps
			}
			io.Printf(i18n.T(i18n.KeyCmdMig_009), modelInfo.textID, modelInfo.textProvider, capTxt)
		}
		// Vision model line
		visTxt := modelInfo.visionID
		if modelInfo.sameAsText {
			visTxt = i18n.T(i18n.KeyCmdMig_165)
		}
		if modelInfo.visionID != "" {
			capTxt := ""
			if modelInfo.visionCaps != "" && !modelInfo.sameAsText {
				capTxt = " " + modelInfo.visionCaps
			} else if !modelInfo.sameAsText {
				// still show provider even if no extra caps
			}
			provTxt := ""
			if !modelInfo.sameAsText && modelInfo.visionProvider != "" {
				provTxt = " [" + modelInfo.visionProvider + "]"
			}
			io.Printf(i18n.T(i18n.KeyCmdMig_011), visTxt, provTxt, capTxt)
		}
		io.Println()
	}

	io.Println("──────────────────────────────────────")
	io.Println(i18n.T(i18n.KeyCmdMig_059))
	io.Println(i18n.T(i18n.KeyCmdMig_060))
	io.Println("──────────────────────────────────────")
}

// showModeDetail displays the second-level menu for a specific mode.
func (h *ModeHandler) showModeDetail(modeName string) {
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return
	}

	io := h.io()
	for {
		io.Println()
		io.Printf(i18n.T(i18n.KeyCmdMig_211), modeName)
		io.Println()

		// Option 1: Prompt sections
		io.Printf(i18n.T(i18n.KeyCmdMig_044), len(mode.Sections))
		// Option 2: Tool modes
		toolModeCount := 0
		if mode.ToolModes != nil {
			toolModeCount = len(mode.ToolModes)
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_047), toolModeCount)

		// Option 3: Model bindings
		modelInfo := h.resolveModeModel(mode)
		io.Println(i18n.T(i18n.KeyCmdMig_050))
		if modelInfo.textID != "" {
			io.Printf(i18n.T(i18n.KeyCmdMig_002), modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		}
		visID := modelInfo.visionID
		if modelInfo.sameAsText {
			visID = i18n.T(i18n.KeyCmdMig_165)
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_003), visID)
		probID := modelInfo.problemID
		if modelInfo.sameAsTextPro {
			probID = i18n.T(i18n.KeyCmdMig_165)
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_004), probID)

		// Option 4: Parameter overrides
		io.Println(i18n.T(i18n.KeyCmdMig_053))
		paramCount := 0
		if mode.Temperature != nil {
			paramCount++
		}
		if mode.MaxTokens != nil {
			paramCount++
		}
		if mode.TopP != nil {
			paramCount++
		}
		if mode.TopK != nil {
			paramCount++
		}
		if mode.RepetitionPenalty != nil {
			paramCount++
		}
		if mode.ThinkingEnabled != nil {
			paramCount++
		}
		if mode.ReasoningEffort != nil {
			paramCount++
		}
		if mode.MaxIterations != nil {
			paramCount++
		}
		if mode.ContextLimit != nil {
			paramCount++
		}
		if mode.ToolCallMode != nil {
			paramCount++
		}
		if paramCount > 0 {
			io.Printf(i18n.T(i18n.KeyCmdMig_005), paramCount)
		} else {
			io.Println(i18n.T(i18n.KeyCmdMig_006))
		}

		io.Println()
		io.Println("──────────────────────────────────────")
		io.Printf(i18n.T(i18n.KeyCmdMig_063))
		io.Printf(i18n.T(i18n.KeyCmdMig_061))
		io.Println(i18n.T(i18n.KeyCmdMig_058))
		io.Println("──────────────────────────────────────")
		io.Print(i18n.T(i18n.KeyCmdMig_357))

		input := strings.ToUpper(strings.TrimSpace(h.readLine()))
		if input == "Q" || input == "QUIT" || input == ".." {
			return
		}
		if input == "B" || input == "BACK" {
			return
		}
		if input == "S" || input == "SWITCH" {
			h.doSwitch(modeName)
			continue
		}
		if input == "D" || input == "DELETE" {
			h.doRemove(modeName)
			return
		}

		switch input {
		case "1":
			h.interactiveEdit([]string{modeName})
		case "2":
			h.showToolModesWizard(modeName)
		case "3":
			h.showModelBindingsWizard(modeName)
		case "4":
			h.showParamWizard(modeName)
		default:
			io.Println(i18n.T(i18n.KeyCmdMig_107))
		}
	}
}

// doSwitch switches to a mode and applies its config.
func (h *ModeHandler) doSwitch(name string) {
	h.cfg.LLM.WorkMode = name
	if err := h.cfg.Save(); err != nil {
		h.io().Printf(i18n.T(i18n.KeyCmdMig_077), err)
		return
	}
	if h.ag != nil {
		h.ag.SyncToolModes(h.cfg)
		h.ag.SetConfig(h.cfg)
		h.ag.ApplyWorkModeConfig()
	}
	h.io().Printf(i18n.T(i18n.KeyCmdMig_071), name)
}

// doRemove removes a mode.
func (h *ModeHandler) doRemove(name string) {
	idx := -1
	for i, m := range h.cfg.WorkModes {
		if m.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.io().Printf(i18n.T(i18n.KeyCmdMig_079), name)
		return
	}
	io := h.io()
	io.Printf(i18n.T(i18n.KeyCmdMig_127), name)
	confirm := strings.TrimSpace(strings.ToLower(h.readLine()))
	if confirm != "y" && confirm != "yes" {
		io.Println(i18n.T(i18n.KeyCmdMig_095))
		return
	}
	if h.cfg.LLM.WorkMode == name {
		h.cfg.LLM.WorkMode = "act"
	}
	h.cfg.WorkModes = append(h.cfg.WorkModes[:idx], h.cfg.WorkModes[idx+1:]...)
	if err := h.cfg.Save(); err != nil {
		io.Printf(i18n.T(i18n.KeyCmdMig_076), err)
		return
	}
	io.Printf(i18n.T(i18n.KeyCmdMig_073), name)
}

// showToolModesWizard interactively shows and manages tool modes for a mode.
// Each tool is numbered; selecting a number cycles its mode:
// auto → confirm → disabled → auto
func (h *ModeHandler) showToolModesWizard(modeName string) {
	io := h.io()
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return
	}

	// Get sorted list of all known tools (excluding "default")
	defaultModes := agent.DefaultToolModes()
	toolNames := make([]string, 0, len(defaultModes))
	for name := range defaultModes {
		if name != "default" {
			toolNames = append(toolNames, name)
		}
	}
	sort.Strings(toolNames)

	// Tool mode cycle order
	cycleOrder := []string{"auto", "confirm", "disabled"}

	for {
		io.Println()
		io.Printf(i18n.T(i18n.KeyCmdMig_209), modeName)
		io.Println()

		// Determine current tool modes from mode config
		toolModes := mode.ToolModes
		if toolModes == nil {
			if modeName == "plan" {
				toolModes = config.DefaultPlanToolModes()
			} else {
				toolModes = defaultModes
			}
		}
		defaultMode := toolModes["default"]
		if defaultMode == "" {
			defaultMode = "confirm"
		}

		io.Printf(i18n.T(i18n.KeyCmdMig_153), defaultMode)

		for i, name := range toolNames {
			m := toolModes[name]
			if m == "" {
				m = defaultMode
			}
			io.Printf("  [%d] %-30s %s\n", i+1, name, m)
		}

		io.Println()
		io.Println(i18n.T(i18n.KeyCmdMig_065))
		io.Println(i18n.T(i18n.KeyCmdMig_064))
		io.Print(i18n.T(i18n.KeyCmdMig_205))

		input := strings.TrimSpace(h.readLine())
		if input == "" {
			return
		}

		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(toolNames) {
			io.Println(i18n.T(i18n.KeyCmdMig_107))
			continue
		}

		selectedTool := toolNames[num-1]

		// Get current mode for this tool
		currentMode := toolModes[selectedTool]
		if currentMode == "" {
			currentMode = defaultMode
		}

		// Cycle to next mode
		nextMode := cycleOrder[0]
		for j, cm := range cycleOrder {
			if cm == currentMode {
				nextIdx := (j + 1) % len(cycleOrder)
				nextMode = cycleOrder[nextIdx]
				break
			}
		}

		// Ensure ToolModes map exists and is populated with all defaults
		if mode.ToolModes == nil {
			mode.ToolModes = make(map[string]string)
			// Copy all default tool modes as base
			for k, v := range defaultModes {
				mode.ToolModes[k] = v
			}
		}
		mode.ToolModes[selectedTool] = nextMode

		if err := h.cfg.Save(); err != nil {
			io.Printf(i18n.T(i18n.KeyCmdMig_076), err)
			continue
		}

		if modeName == h.cfg.LLM.WorkMode || (h.cfg.LLM.WorkMode == "" && modeName == "act") {
			if h.ag != nil {
				h.ag.SyncToolModes(h.cfg)
			}
		}

		io.Printf(i18n.T(i18n.KeyCmdMig_070), selectedTool, nextMode)
	}
}

// showModelBindingsWizard shows and manages model bindings for a mode.
func (h *ModeHandler) showModelBindingsWizard(modeName string) {
	io := h.io()
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return
	}

	for {
		io.Println()
		io.Printf(i18n.T(i18n.KeyCmdMig_210), modeName)
		io.Println()

		// Resolve and display current model info
		modelInfo := h.resolveModeModel(mode)
		if mode.ModelID != nil {
			io.Printf(i18n.T(i18n.KeyCmdMig_102), modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		} else {
			io.Printf(i18n.T(i18n.KeyCmdMig_101), modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		}
		visDesc := modelInfo.visionID
		if modelInfo.sameAsText {
			visDesc = i18n.T(i18n.KeyCmdMig_165)
		}
		io.Printf(i18n.T(i18n.KeyCmdMig_134), visDesc)
		io.Println()

		io.Println(i18n.T(i18n.KeyCmdMig_045))
		if mode.ModelID != nil {
			io.Println(i18n.T(i18n.KeyCmdMig_048))
		}
		io.Println(i18n.T(i18n.KeyCmdMig_051))
		if mode.VisionModelID != nil {
			io.Println(i18n.T(i18n.KeyCmdMig_054))
		}
		io.Println(i18n.T(i18n.KeyCmdMig_055))
		if mode.ProblemModelID != nil {
			io.Println(i18n.T(i18n.KeyCmdMig_056))
		}

		io.Println()
		io.Println(i18n.T(i18n.KeyCmdMig_057))
		io.Print(i18n.T(i18n.KeyCmdMig_357))

		input := strings.ToUpper(strings.TrimSpace(h.readLine()))
		if input == "Q" || input == "QUIT" || input == ".." {
			return
		}
		if input == "B" || input == "BACK" {
			return
		}

		switch input {
		case "1":
			h.selectModelInteractive(modeName, "text")
		case "2":
			if mode.ModelID != nil {
				h.handleModeModel(modeName, []string{"text", "none"})
			}
		case "3":
			h.selectModelInteractive(modeName, "vision")
		case "4":
			if mode.VisionModelID != nil {
				h.handleModeModel(modeName, []string{"vision", "none"})
			}
		case "5":
			h.selectModelInteractive(modeName, "problem")
		case "6":
			if mode.ProblemModelID != nil {
				h.handleModeModel(modeName, []string{"problem", "none"})
			}
		default:
			io.Println(i18n.T(i18n.KeyCmdMig_107))
		}
	}
}

// selectModelInteractive shows a numbered list of available models for the user to choose.
func (h *ModeHandler) selectModelInteractive(modeName, bindType string) {
	io := h.io()
	if len(h.cfg.Models) == 0 {
		io.Println(i18n.T(i18n.KeyCmdMig_114))
		io.Print(i18n.T(i18n.KeyCmdMig_202))
		h.readLine()
		return
	}

	// Sort by priority descending for sequential display
	sorted := make([]*config.ModelConfig, len(h.cfg.Models))
	copy(sorted, h.cfg.Models)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Show available models with sequential numbers
	io.Println()
	io.Printf(i18n.T(i18n.KeyCmdMig_148), bindType)
	for idx, m := range sorted {
		status := "⬜"
		if m.Enabled {
			status = "✅"
		}
		caps := ""
		if m.Capabilities.Vision {
			caps += "👁"
		}
		if m.Capabilities.ToolCall {
			caps += "🔧"
		}
		if m.Capabilities.Thinking {
			caps += "💭"
		}
		capStr := ""
		if caps != "" {
			capStr = " [" + caps + "]"
		}
		io.Printf("  [%d] %s %s [%s] %s%s\n", idx+1, status, m.ID, m.Provider, m.Model, capStr)
	}
	io.Println()
	io.Print(i18n.T(i18n.KeyCmdMig_139))

	input := strings.TrimSpace(h.readLine())
	if input == "" || input == "0" {
		return
	}

	// Try to interpret as priority-based number
	num, err := strconv.Atoi(input)
	if err != nil || num < 0 {
		io.Println(i18n.T(i18n.KeyCmdMig_107))
		return
	}

	// sorted is already sorted by priority from above
	idx := num - 1
	if idx < 0 || idx >= len(sorted) {
		io.Println(i18n.T(i18n.KeyCmdMig_106))
		return
	}

	selectedModel := sorted[idx].ID
	result, err := h.handleModeModel(modeName, []string{bindType, selectedModel})
	if err != nil {
		io.Printf("  ❌ %v\n", err)
	} else {
		io.Println(result)
	}
}

// showParamWizard shows and manages parameter overrides for a mode.
func (h *ModeHandler) showParamWizard(modeName string) {
	io := h.io()
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return
	}

	for {
		io.Println()
		io.Printf(i18n.T(i18n.KeyCmdMig_207), modeName)
		io.Println()

		fmtT := func(name string, v *float64) string {
			if v != nil {
				return fmt.Sprintf("  %s: %.2f", name, *v)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_026), name)
		}
		fmtI := func(name string, v *int) string {
			if v != nil {
				return fmt.Sprintf("  %s: %d", name, *v)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_026), name)
		}
		fmtS := func(name string, v *string) string {
			if v != nil {
				return fmt.Sprintf("  %s: %s", name, *v)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_026), name)
		}
		fmtB := func(name string, v *bool) string {
			if v != nil {
				val := "off"
				if *v {
					val = "on"
				}
				return fmt.Sprintf("  %s: %s", name, val)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_026), name)
		}

		io.Println("  [1] " + fmtT("temperature", mode.Temperature))
		io.Println("  [2] " + fmtI("max_tokens", mode.MaxTokens))
		io.Println("  [3] " + fmtT("top_p", mode.TopP))
		io.Println("  [4] " + fmtI("top_k", mode.TopK))
		io.Println("  [5] " + fmtT("repetition_penalty", mode.RepetitionPenalty))
		io.Println("  [6] " + fmtB("thinking", mode.ThinkingEnabled))
		io.Println("  [7] " + fmtS("reasoning_effort", mode.ReasoningEffort))
		io.Println("  [8] " + fmtI("max_iterations", mode.MaxIterations))
		io.Println("  [9] " + fmtI("context_limit", mode.ContextLimit))
		io.Println(" [10] " + fmtS("tool_call_mode", mode.ToolCallMode))

		io.Println()
		io.Println(i18n.T(i18n.KeyCmdMig_062))
		io.Print(i18n.T(i18n.KeyCmdMig_359))

		input := strings.ToUpper(strings.TrimSpace(h.readLine()))
		if input == "Q" || input == "QUIT" || input == ".." {
			return
		}
		if input == "B" || input == "BACK" {
			return
		}
		if input == "R" || input == "RESET" || input == "RESET-ALL" {
			h.handleModeParam(modeName, []string{"reset-all"})
			continue
		}

		paramKeys := map[string]string{
			"1":  "temperature",
			"2":  "max_tokens",
			"3":  "top_p",
			"4":  "top_k",
			"5":  "repetition_penalty",
			"6":  "thinking",
			"7":  "reasoning_effort",
			"8":  "max_iterations",
			"9":  "context_limit",
			"10": "tool_call_mode",
		}

		key, ok := paramKeys[input]
		if !ok {
			io.Println(i18n.T(i18n.KeyCmdMig_107))
			continue
		}

		// Check if currently set → offer reset
		isSet := false
		switch input {
		case "1":
			isSet = mode.Temperature != nil
		case "2":
			isSet = mode.MaxTokens != nil
		case "3":
			isSet = mode.TopP != nil
		case "4":
			isSet = mode.TopK != nil
		case "5":
			isSet = mode.RepetitionPenalty != nil
		case "6":
			isSet = mode.ThinkingEnabled != nil
		case "7":
			isSet = mode.ReasoningEffort != nil
		case "8":
			isSet = mode.MaxIterations != nil
		case "9":
			isSet = mode.ContextLimit != nil
		case "10":
			isSet = mode.ToolCallMode != nil
		}

		if isSet {
			io.Printf(i18n.T(i18n.KeyCmdMig_096), paramKeys[input])
			val := strings.TrimSpace(h.readLine())
			if strings.ToUpper(val) == "R" || strings.ToUpper(val) == "RESET" {
				h.handleModeParam(modeName, []string{"reset", key})
				continue
			}
			if val == "" {
				continue
			}
			h.handleModeParam(modeName, []string{key, val})
		} else {
			io.Printf(i18n.T(i18n.KeyCmdMig_137), paramKeys[input])
			val := strings.TrimSpace(h.readLine())
			if val == "" {
				continue
			}
			h.handleModeParam(modeName, []string{key, val})
		}
	}
}

// interactiveCreateWizard guides the user through creating a new mode.
func (h *ModeHandler) interactiveCreateWizard() (string, error) {
	io := h.io()

	// Prompt for name with cancel support
	for {
		io.Print(i18n.T(i18n.KeyCmdMig_177))
		name := h.readLine()
		if strings.ToUpper(name) == "Q" || strings.ToUpper(name) == "QUIT" || name == ".." {
			io.Println(i18n.T(i18n.KeyCmdMig_095))
			return "", nil
		}
		if name == "" {
			io.Println(i18n.T(i18n.KeyCmdMig_090))
			continue
		}
		if name == "default" {
			io.Println(i18n.T(i18n.KeyCmdMig_080))
			continue
		}
		duplicate := false
		for _, m := range h.cfg.WorkModes {
			if m.Name == name {
				duplicate = true
				break
			}
		}
		if duplicate {
			io.Printf(i18n.T(i18n.KeyCmdMig_117), name)
			continue
		}
		// Name is valid - proceed
		io.Print(i18n.T(i18n.KeyCmdMig_119))
		desc := h.readLine()
		if strings.ToUpper(desc) == "Q" || strings.ToUpper(desc) == "QUIT" || desc == ".." {
			io.Println(i18n.T(i18n.KeyCmdMig_095))
			return "", nil
		}

		sections := h.interactiveSelectSections(i18n.T(i18n.KeyCmdMig_368))

		newMode := config.WorkMode{
			Name:        name,
			Description: desc,
			Sections:    sections,
		}
		h.cfg.WorkModes = append(h.cfg.WorkModes, newMode)
		if err := h.cfg.Save(); err != nil {
			io.Printf(i18n.T(i18n.KeyCmdMig_076), err)
			return "", nil
		}

		io.Printf(i18n.T(i18n.KeyCmdMig_072), name)

		// Ask if user wants to configure model/params now
		io.Print(i18n.T(i18n.KeyCmdMig_110))
		confirm := strings.TrimSpace(strings.ToLower(h.readLine()))
		if confirm != "n" && confirm != "no" {
			h.showModeDetail(name)
		}
		return "", nil
	}
}

// interactiveRemoveWizard interactively selects and removes a mode.
func (h *ModeHandler) interactiveRemoveWizard() {
	selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_372))
	if err != nil {
		return
	}
	h.doRemove(selected.Name)
}

func (h *ModeHandler) listModes() string {
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyModeList))
	sb.WriteString("\n")

	modes := h.getAllModes()
	current := h.cfg.LLM.WorkMode
	if current == "" || current == "default" {
		current = "act"
	}

	for i, m := range modes {
		marker := " "
		if m.Name == current {
			marker = "*"
		}
		desc := m.Description
		if desc == "" {
			desc = "-"
		}
		sb.WriteString(fmt.Sprintf("  %s [%d] %s: %s\n", marker, i+1, m.Name, desc))
		if len(m.Sections) > 0 {
			sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_008), len(m.Sections)))
		}
	}
	return sb.String()
}

// getAllAvailableSections returns all available section names (built-in + custom).
func (h *ModeHandler) getAllAvailableSections() []string {
	var sections []string
	sections = append(sections, config.DefaultBuiltInSections()...)
	for _, s := range h.cfg.PromptSections {
		sections = append(sections, s.Name)
	}
	return sections
}

// getAllModes returns all available modes (config + built-in), ensuring no duplicates.
// "default" in config is treated as equivalent to built-in "act" mode.
func (h *ModeHandler) getAllModes() []config.WorkMode {
	builtIn := config.DefaultWorkModes()
	modeMap := make(map[string]bool)
	modes := make([]config.WorkMode, 0, len(builtIn)+len(h.cfg.WorkModes))
	for _, m := range h.cfg.WorkModes {
		name := m.Name
		if name == "default" {
			name = "act"
		}
		if !modeMap[name] {
			m.Name = name
			modes = append(modes, m)
			modeMap[name] = true
		}
	}
	for _, m := range builtIn {
		if !modeMap[m.Name] {
			modes = append(modes, m)
			modeMap[m.Name] = true
		}
	}
	// Sort by name for stable ordering across invocations.
	sort.Slice(modes, func(i, j int) bool {
		return modes[i].Name < modes[j].Name
	})
	return modes
}

// selectModeByNumber interactively selects a mode by number.
func (h *ModeHandler) selectModeByNumber(prompt string) (*config.WorkMode, error) {
	modes := h.getAllModes()

	io := h.io()
	io.Println()
	io.Println(prompt)
	io.Println()
	for i, m := range modes {
		io.Printf("  [%d] %s", i+1, m.Name)
		if m.Description != "" {
			io.Printf(" - %s", m.Description)
		}
		io.Println()
	}
	io.Print(i18n.T(i18n.KeyCmdMig_190))

	input := h.readLine()
	if input == "" {
		return nil, fmt.Errorf("%s", i18n.T(i18n.KeyCancelled))
	}
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(modes) {
		return nil, fmt.Errorf("%s", i18n.T(i18n.KeyInvalidChoice))
	}
	return &modes[num-1], nil
}

// interactiveSwitch switches to a mode interactively or by name.
func (h *ModeHandler) interactiveSwitch(args []string) (string, error) {
	if len(args) > 0 {
		// Direct name provided
		name := args[0]
		// Backward compatibility: "default" maps to "act"
		if name == "default" {
			name = "act"
		}
		if !h.modeExists(name) && name != "act" {
			return "", fmt.Errorf("%s", i18n.T(i18n.KeyModeNotFound))
		}
		h.cfg.LLM.WorkMode = name
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil {
			h.ag.SyncToolModes(h.cfg) // must come BEFORE SetConfig/rebuildSystemPrompt
			h.ag.SetConfig(h.cfg)
			h.ag.ApplyWorkModeConfig() // apply mode-specific model/param overrides
		}
		return fmt.Sprintf(i18n.T(i18n.KeyModeSwitched), name), nil
	}

	// Interactive selection
	selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_370))
	if err != nil {
		return "", err
	}
	h.cfg.LLM.WorkMode = selected.Name
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SyncToolModes(h.cfg) // must come BEFORE SetConfig/rebuildSystemPrompt
		h.ag.SetConfig(h.cfg)
		h.ag.ApplyWorkModeConfig() // apply mode-specific model/param overrides
	}
	return fmt.Sprintf(i18n.T(i18n.KeyModeSwitched), selected.Name), nil
}

// interactiveCreate creates a new work mode interactively.
func (h *ModeHandler) interactiveCreate() (string, error) {
	io := h.io()

	io.Print(i18n.T(i18n.KeyCmdMig_178))
	name := h.readLine()
	if name == "" {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_254))
	}

	// Check duplicates
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return "", fmt.Errorf("%s", i18n.T(i18n.KeyModeExists))
		}
	}
	if name == "default" {
		return "", errors.New(i18n.T(i18n.KeyCmdMig_230))
	}

	io.Print(i18n.T(i18n.KeyCmdMig_118))
	desc := h.readLine()

	// Select sections
	sections := h.interactiveSelectSections(i18n.T(i18n.KeyCmdMig_367))

	newMode := config.WorkMode{
		Name:        name,
		Description: desc,
		Sections:    sections,
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes, newMode)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyModeAdded), name), nil
}

// interactiveSelectSections presents all available sections with numbers,
// allowing the user to toggle inclusion and reorder.
func (h *ModeHandler) interactiveSelectSections(prompt string) []string {
	allSections := h.getAllAvailableSections()
	selected := make(map[int]bool)
	var order []int

	// Initialize: all sections selected by default
	for i := range allSections {
		selected[i] = true
		order = append(order, i)
	}

	io := h.io()

	for {
		io.Println()
		io.Println(prompt)
		io.Println()
		for i, name := range allSections {
			marker := " "
			if selected[i] {
				marker = "✓"
			}
			io.Printf("  [%d] [%s] %s\n", i+1, marker, name)
		}
		io.Println()
		io.Println(i18n.T(i18n.KeyCmdMig_100))
		io.Println(i18n.T(i18n.KeyCmdMig_018))
		io.Println(i18n.T(i18n.KeyCmdMig_014))
		io.Println(i18n.T(i18n.KeyCmdMig_016))
		io.Println(i18n.T(i18n.KeyCmdMig_017))
		io.Print(i18n.T(i18n.KeyCmdMig_188))

		input := h.readLine()
		if input == "" {
			break
		}

		// Handle move up: +N
		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				io.Println(i18n.T(i18n.KeyCmdMig_106))
				continue
			}
			idx := num - 1
			// Find position in order
			for pos, v := range order {
				if v == idx && pos > 0 {
					order[pos], order[pos-1] = order[pos-1], order[pos]
					break
				}
			}
			continue
		}

		// Handle move down: -N
		if strings.HasPrefix(input, "-") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				io.Println(i18n.T(i18n.KeyCmdMig_106))
				continue
			}
			idx := num - 1
			for pos, v := range order {
				if v == idx && pos < len(order)-1 {
					order[pos], order[pos+1] = order[pos+1], order[pos]
					break
				}
			}
			continue
		}

		// Handle toggle
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(allSections) {
			io.Println(i18n.T(i18n.KeyCmdMig_107))
			continue
		}
		idx := num - 1
		selected[idx] = !selected[idx]
	}

	// Build result
	var result []string
	for _, v := range order {
		if selected[v] {
			result = append(result, allSections[v])
		}
	}
	if len(result) == 0 {
		result = config.DefaultBuiltInSections()
	}
	return result
}

// interactiveEdit allows interactive reordering of sections for a mode.
func (h *ModeHandler) interactiveEdit(args []string) (string, error) {
	var modeName string
	if len(args) > 0 {
		modeName = args[0]
	} else {
		selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_373))
		if err != nil {
			return "", err
		}
		modeName = selected.Name
	}
	// Use findOrCreateMode to ensure the mode exists in config
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyModeNotFound))
	}

	// Show current sections and allow reordering
	allSections := h.getAllAvailableSections()

	// Build current section index mapping
	currentIndices := make([]int, 0, len(mode.Sections))
	for _, s := range mode.Sections {
		for i, a := range allSections {
			if a == s {
				currentIndices = append(currentIndices, i)
				break
			}
		}
	}

	io := h.io()
	for {
		io.Printf(i18n.T(i18n.KeyCmdMig_187), mode.Name)
		io.Println(i18n.T(i18n.KeyCmdMig_098))
		for pos, idx := range currentIndices {
			io.Printf("    [%d] %s\n", pos+1, allSections[idx])
		}
		// Show available sections not yet in the list (independently numbered from 1)
		inCurrent := make(map[int]bool)
		for _, idx := range currentIndices {
			inCurrent[idx] = true
		}
		type availEntry struct {
			globalIdx int
			name      string
		}
		var availList []availEntry
		for i, name := range allSections {
			if !inCurrent[i] {
				availList = append(availList, availEntry{globalIdx: i, name: name})
			}
		}
		if len(availList) > 0 {
			io.Println(i18n.T(i18n.KeyCmdMig_175))
			for avNum, ae := range availList {
				io.Printf("    [%d] %s\n", avNum+1, ae.name)
			}
		}
		io.Println()
		io.Println(i18n.T(i18n.KeyCmdMig_100))
		io.Println(i18n.T(i18n.KeyCmdMig_013))
		io.Println(i18n.T(i18n.KeyCmdMig_015))
		io.Println(i18n.T(i18n.KeyCmdMig_019))
		io.Println(i18n.T(i18n.KeyCmdMig_020))
		io.Println(i18n.T(i18n.KeyCmdMig_022))
		io.Println(i18n.T(i18n.KeyCmdMig_021))
		io.Println(i18n.T(i18n.KeyCmdMig_024))
		io.Print(i18n.T(i18n.KeyCmdMig_188))

		input := h.readLine()
		if input == "" || input == i18n.T(i18n.KeyCmdMig_258) {
			break
		}

		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println(i18n.T(i18n.KeyCmdMig_105))
				continue
			}
			pos := num - 1
			if pos > 0 {
				currentIndices[pos], currentIndices[pos-1] = currentIndices[pos-1], currentIndices[pos]
			}
			continue
		}

		if strings.HasPrefix(input, "-") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println(i18n.T(i18n.KeyCmdMig_105))
				continue
			}
			pos := num - 1
			if pos < len(currentIndices)-1 {
				currentIndices[pos], currentIndices[pos+1] = currentIndices[pos+1], currentIndices[pos]
			}
			continue
		}

		if strings.HasPrefix(input, "a") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(availList) {
				io.Println(i18n.T(i18n.KeyCmdMig_106))
				continue
			}
			// Map to global index via availList
			globalIdx := availList[num-1].globalIdx
			currentIndices = append(currentIndices, globalIdx)
			continue
		}

		if strings.HasPrefix(input, "d") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println(i18n.T(i18n.KeyCmdMig_105))
				continue
			}
			pos := num - 1
			currentIndices = append(currentIndices[:pos], currentIndices[pos+1:]...)
			continue
		}

		if strings.HasPrefix(input, "v") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println(i18n.T(i18n.KeyCmdMig_105))
				continue
			}
			globalIdx := currentIndices[num-1]
			secName := allSections[globalIdx]
			// Build and show the section content using the same logic as buildNamedSection
			io.Printf("\n  ==== [%s] ====\n", secName)
			io.Println(h.previewSection(secName))
			io.Println("  ================")
			continue
		}

		if input == "p" {
			io.Println(i18n.T(i18n.KeyCmdMig_169))
			io.Print(h.previewFullPrompt(currentIndices, allSections))
			io.Println("\n  =======================")
			continue
		}

		io.Println(i18n.T(i18n.KeyCmdMig_107))
	}

	// Build updated sections
	mode.Sections = make([]string, 0, len(currentIndices))
	for _, idx := range currentIndices {
		mode.Sections = append(mode.Sections, allSections[idx])
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_260), mode.Name, len(mode.Sections)), nil
}

// interactiveRemove removes a mode interactively or by name.
func (h *ModeHandler) interactiveRemove(args []string) (string, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		selected, err := h.selectModeByNumber(i18n.T(i18n.KeyCmdMig_371))
		if err != nil {
			return "", err
		}
		name = selected.Name
	}

	// Confirm deletion
	h.io().Printf(i18n.T(i18n.KeyCmdMig_126), name)
	confirm := strings.TrimSpace(strings.ToLower(h.readLine()))
	if confirm != "y" && confirm != "yes" {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyCancelled))
	}

	idx := -1
	for i, m := range h.cfg.WorkModes {
		if m.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyModeNotFound))
	}

	if h.cfg.LLM.WorkMode == name {
		h.cfg.LLM.WorkMode = "act"
	}

	h.cfg.WorkModes = append(h.cfg.WorkModes[:idx], h.cfg.WorkModes[idx+1:]...)
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("cannot save config: %w", err)
	}
	if h.ag != nil {
		h.ag.SetConfig(h.cfg)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyModeRemoved), name), nil
}

// previewSection loads and returns the content of a single section.
// Uses the same loading logic as agent.buildNamedSection.
func (h *ModeHandler) previewSection(name string) string {
	cwd, _ := os.Getwd()
	// Check if there's a mode-specific file
	modeName := h.cfg.LLM.WorkMode
	if modeName != "" {
		filePath := fmt.Sprintf("%s/mode/%s/%s.md", cwd, modeName, name)
		if data, err := os.ReadFile(filePath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	// Fallback: return the i18n key name as placeholder
	i18nKey := "system_prompt_" + strings.ToLower(name)
	content := i18n.T(i18nKey)
	if content != "" && content != i18nKey {
		// Try to replace common placeholders with static values
		content = strings.ReplaceAll(content, "{AGENT_NAME}", h.cfg.LLM.AgentName)
		content = strings.ReplaceAll(content, "{CWD}", cwd)
		content = strings.ReplaceAll(content, "{CUSTOM_RULES}", "")
		if len(content) > 500 {
			content = content[:500] + i18n.T(i18n.KeyCmdMig_168)
		}
		return content
	}
	// Check custom prompt sections
	for _, ps := range h.cfg.PromptSections {
		if ps.Name == name && ps.Content != "" {
			return ps.Content
		}
	}
	return i18n.T(i18n.KeyCmdMig_164)
}

// previewFullPrompt concatenates all current sections in order.
func (h *ModeHandler) previewFullPrompt(indices []int, allSections []string) string {
	var sb strings.Builder
	for i, idx := range indices {
		name := allSections[idx]
		sb.WriteString(fmt.Sprintf("\n==== [%d] %s ====\n", i+1, name))
		content := h.previewSection(name)
		if len(content) > 300 {
			content = content[:300] + i18n.T(i18n.KeyCmdMig_168)
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (h *ModeHandler) modeExists(name string) bool {
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return true
		}
	}
	return false
}

// handleModeTools manages per-tool modes for a named work mode.
// Syntax: .mode <modeName> tools              — list tools
//
//	.mode <modeName> tools <method> <value> — set tool mode
//	.mode <modeName> tools reset         — reset to default
func (h *ModeHandler) handleModeTools(modeName string, args []string) (string, error) {
	// Ensure the named mode exists in config (import from built-in if needed)
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_290), modeName)
	}

	// No args: list tools
	if len(args) == 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_309), modeName))
		toolModes := mode.ToolModes
		if toolModes == nil {
			if modeName == "plan" {
				toolModes = config.DefaultPlanToolModes()
			} else {
				toolModes = agent.DefaultToolModes()
			}
		}
		defaultMode := toolModes["default"]
		if defaultMode == "" {
			defaultMode = "confirm"
		}
		sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_153), defaultMode))
		for name := range agent.DefaultToolModes() {
			if name == "default" {
				continue
			}
			m := toolModes[name]
			if m == "" {
				m = defaultMode
			}
			sb.WriteString(fmt.Sprintf("  %-30s %s\n", name, m))
		}
		return sb.String(), nil
	}

	// reset: reset to mode-specific defaults
	if args[0] == "reset" {
		if modeName == "plan" {
			mode.ToolModes = config.DefaultPlanToolModes()
		} else {
			mode.ToolModes = nil
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if modeName == h.cfg.LLM.WorkMode || (h.cfg.LLM.WorkMode == "" && modeName == "act") {
			if h.ag != nil {
				h.ag.SyncToolModes(h.cfg)
			}
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_266), modeName), nil
	}

	// Set specific tool mode: <method> <auto|confirm|disabled>
	if len(args) >= 2 {
		method := args[0]
		value := args[1]
		if value != "auto" && value != "confirm" && value != "disabled" {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_280), value)
		}
		if mode.ToolModes == nil {
			mode.ToolModes = make(map[string]string)
		}
		mode.ToolModes[method] = value
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if modeName == h.cfg.LLM.WorkMode || (h.cfg.LLM.WorkMode == "" && modeName == "act") {
			if h.ag != nil {
				h.ag.SyncToolModes(h.cfg)
			}
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_264), modeName, method, value), nil
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_323), modeName)
}

// findOrCreateMode finds a mode by name in config, importing from built-in if needed.
// Returns nil if the mode doesn't exist and has no built-in default.
func (h *ModeHandler) findOrCreateMode(name string) *config.WorkMode {
	// Check existing
	for i := range h.cfg.WorkModes {
		if h.cfg.WorkModes[i].Name == name {
			return &h.cfg.WorkModes[i]
		}
	}
	// Check built-in defaults
	for _, m := range config.DefaultWorkModes() {
		if m.Name == name {
			h.cfg.WorkModes = append(h.cfg.WorkModes, m)
			return &h.cfg.WorkModes[len(h.cfg.WorkModes)-1]
		}
	}
	return nil
}

// handleModeModel manages model bindings for a named work mode.
// Syntax:
//
//	.mode <name> model                    — show current model bindings
//	.mode <name> model text <modelID>     — bind text model
//	.mode <name> model vision <modelID>   — bind vision model
//	.mode <name> model text none|remove   — unbind text model
//	.mode <name> model vision none|remove — unbind vision model
//	.mode <name> model list               — list available models
func (h *ModeHandler) handleModeModel(modeName string, args []string) (string, error) {
	// Ensure the mode exists
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_291), modeName)
	}

	// No args: show current model bindings
	if len(args) == 0 || args[0] == "show" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_310), modeName))
		if mode.ModelID != nil {
			sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_103), *mode.ModelID))
		} else {
			sb.WriteString(i18n.T(i18n.KeyCmdMig_104))
		}
		if mode.VisionModelID != nil {
			sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_134), *mode.VisionModelID))
		} else {
			sb.WriteString(i18n.T(i18n.KeyCmdMig_135))
		}
		return sb.String(), nil
	}

	// list: show available models
	if args[0] == "list" {
		if len(h.cfg.Models) == 0 {
			return i18n.T(i18n.KeyCmdMig_295), nil
		}
		var sb strings.Builder
		sb.WriteString(i18n.T(i18n.KeyCmdMig_253))
		for _, m := range h.cfg.Models {
			status := "⬜"
			if m.Enabled {
				status = "✅"
			}
			caps := ""
			if m.Capabilities.Vision {
				caps += "👁"
			}
			if m.Capabilities.ToolCall {
				caps += "🔧"
			}
			sb.WriteString(fmt.Sprintf("  %s %s [%s] %s\n", status, m.ID, m.Provider, m.Model))
			if caps != "" {
				sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_025), caps, m.Priority))
			} else {
				sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_023), m.Priority))
			}
		}
		sb.WriteString(i18n.T(i18n.KeyCmdMig_201))
		sb.WriteString(i18n.T(i18n.KeyCmdMig_235))
		return sb.String(), nil
	}

	// text or vision binding
	if len(args) >= 2 {
		target := args[0] // "text" or "vision"
		value := args[1]

		// Check for unbind
		if value == "none" || value == "remove" {
			switch target {
			case "text":
				mode.ModelID = nil
			case "vision":
				mode.VisionModelID = nil
			case "problem":
				mode.ProblemModelID = nil
			default:
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_281), target)
			}
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("cannot save config: %w", err)
			}
			if h.ag != nil {
				h.ag.ApplyWorkModeConfig()
			}
			return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_261), modeName, target), nil
		}

		// Validate model ID exists
		validID := false
		for _, m := range h.cfg.Models {
			if m.ID == value {
				validID = true
				break
			}
		}
		if !validID {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_304), value, modeName)
		}

		switch target {
		case "text":
			mode.ModelID = &value
		case "vision":
			mode.VisionModelID = &value
		case "problem":
			mode.ProblemModelID = &value
		default:
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_281), target)
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil {
			h.ag.ApplyWorkModeConfig()
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_262), modeName, target, value), nil
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_320), modeName)
}

// handleModeParam manages parameter overrides for a named work mode.
// Syntax:
//
//	.mode <name> param                          — show all overrides
//	.mode <name> param temperature <value>     — set parameter
//	.mode <name> param reset <key>             — reset one parameter
//	.mode <name> param reset-all               — reset all parameters
func (h *ModeHandler) handleModeParam(modeName string, args []string) (string, error) {
	// Ensure the mode exists
	mode := h.findOrCreateMode(modeName)
	if mode == nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_291), modeName)
	}

	// No args: show current parameter overrides
	if len(args) == 0 || args[0] == "show" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_308), modeName))
		if mode.Temperature != nil {
			sb.WriteString(fmt.Sprintf("  temperature:        %.2f\n", *mode.Temperature))
		}
		if mode.MaxTokens != nil {
			sb.WriteString(fmt.Sprintf("  max_tokens:         %d\n", *mode.MaxTokens))
		}
		if mode.TopP != nil {
			sb.WriteString(fmt.Sprintf("  top_p:              %.2f\n", *mode.TopP))
		}
		if mode.TopK != nil {
			sb.WriteString(fmt.Sprintf("  top_k:              %d\n", *mode.TopK))
		}
		if mode.RepetitionPenalty != nil {
			sb.WriteString(fmt.Sprintf("  repetition_penalty:  %.2f\n", *mode.RepetitionPenalty))
		}
		if mode.ThinkingEnabled != nil {
			if *mode.ThinkingEnabled {
				sb.WriteString("  thinking:           on\n")
			} else {
				sb.WriteString("  thinking:           off\n")
			}
		}
		if mode.ReasoningEffort != nil {
			sb.WriteString(fmt.Sprintf("  reasoning_effort:   %s\n", *mode.ReasoningEffort))
		}
		if mode.MaxIterations != nil {
			sb.WriteString(fmt.Sprintf("  max_iterations:     %d\n", *mode.MaxIterations))
		}
		if mode.ContextLimit != nil {
			sb.WriteString(fmt.Sprintf("  context_limit:      %d\n", *mode.ContextLimit))
		}
		if mode.ToolCallMode != nil {
			sb.WriteString(fmt.Sprintf("  tool_call_mode:     %s\n", *mode.ToolCallMode))
		}
		if sb.Len() == len(fmt.Sprintf(i18n.T(i18n.KeyCmdMig_308), modeName)) {
			sb.WriteString(i18n.T(i18n.KeyCmdMig_027))
		}
		sb.WriteString(i18n.T(i18n.KeyCmdMig_203))
		sb.WriteString("  thinking, reasoning_effort, max_iterations, context_limit, tool_call_mode\n")
		return sb.String(), nil
	}

	// reset-all: clear all overrides
	if args[0] == "reset-all" {
		mode.Temperature = nil
		mode.MaxTokens = nil
		mode.TopP = nil
		mode.TopK = nil
		mode.RepetitionPenalty = nil
		mode.ThinkingEnabled = nil
		mode.ReasoningEffort = nil
		mode.MaxIterations = nil
		mode.ContextLimit = nil
		mode.ToolCallMode = nil
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil && modeName == h.cfg.LLM.WorkMode {
			h.ag.ApplyWorkModeConfig()
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_267), modeName), nil
	}

	// reset <key>: reset one parameter
	if args[0] == "reset" {
		if len(args) < 2 {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_322), modeName)
		}
		key := args[1]
		switch key {
		case "temperature":
			mode.Temperature = nil
		case "max_tokens":
			mode.MaxTokens = nil
		case "top_p":
			mode.TopP = nil
		case "top_k":
			mode.TopK = nil
		case "repetition_penalty":
			mode.RepetitionPenalty = nil
		case "thinking":
			mode.ThinkingEnabled = nil
		case "reasoning_effort":
			mode.ReasoningEffort = nil
		case "max_iterations":
			mode.MaxIterations = nil
		case "context_limit":
			mode.ContextLimit = nil
		case "tool_call_mode":
			mode.ToolCallMode = nil
		default:
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_292), key)
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil && modeName == h.cfg.LLM.WorkMode {
			h.ag.ApplyWorkModeConfig()
		}
		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_265), modeName, key), nil
	}

	// Set parameter: <key> <value>
	if len(args) >= 2 {
		key := args[0]
		value := strings.Join(args[1:], " ")
		var saved bool

		switch key {
		case "temperature":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_274), value)
			}
			mode.Temperature = &v
			saved = true
		case "max_tokens":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_271), value)
			}
			mode.MaxTokens = &v
			saved = true
		case "top_p":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_278), value)
			}
			mode.TopP = &v
			saved = true
		case "top_k":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_277), value)
			}
			mode.TopK = &v
			saved = true
		case "repetition_penalty":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_273), value)
			}
			mode.RepetitionPenalty = &v
			saved = true
		case "thinking":
			switch value {
			case "on", "true", "1":
				v := true
				mode.ThinkingEnabled = &v
			case "off", "false", "0":
				v := false
				mode.ThinkingEnabled = &v
			default:
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_275), value)
			}
			saved = true
		case "reasoning_effort":
			if value != "low" && value != "medium" && value != "high" {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_272), value)
			}
			mode.ReasoningEffort = &value
			saved = true
		case "max_iterations":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_270), value)
			}
			mode.MaxIterations = &v
			saved = true
		case "context_limit":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_269), value)
			}
			mode.ContextLimit = &v
			saved = true
		case "tool_call_mode":
			if value != "openai" && value != "xml" {
				return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_276), value)
			}
			mode.ToolCallMode = &value
			saved = true
		default:
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_293), key)
		}

		if !saved {
			return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_283), key)
		}

		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil && modeName == h.cfg.LLM.WorkMode {
			h.ag.ApplyWorkModeConfig()
		}

		return fmt.Sprintf(i18n.T(i18n.KeyCmdMig_263), modeName, key, value), nil
	}

	return "", fmt.Errorf(i18n.T(i18n.KeyCmdMig_321), modeName)
}
