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
	"fmt"
	"os"
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
		info.textID = "(无可用模型)"
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
		info.visionID = "(无可用视觉模型)"
	}

	return info
}

// runWizard runs the interactive mode management wizard.
func (h *ModeHandler) runWizard() {
	io := h.io()
	for {
		h.showModeOverview()
		io.Print("请选择 (输入编号或命令): ")
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
			selected, err := h.selectModeByNumber("选择要切换到的模式:")
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
			selected, err := h.selectModeByNumber("选择要编辑的模式:")
			if err == nil {
				h.showModeDetail(selected.Name)
			}
			continue
		}

		// Try number: select mode for detail view
		num, err := strconv.Atoi(input)
		if err == nil && num >= 1 {
			modes := h.getAllModes()
			if num <= len(modes) {
				h.showModeDetail(modes[num-1].Name)
				continue
			}
		}
		io.Println("  无效输入")
	}
}

// showModeOverview displays the first-level menu listing all modes.
func (h *ModeHandler) showModeOverview() {
	io := h.io()
	io.Println()
	io.Println("────────── 工作模式管理 ──────────")
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
			io.Printf("     文本模型: %s [%s]%s\n", modelInfo.textID, modelInfo.textProvider, capTxt)
		}
		// Vision model line
		visTxt := modelInfo.visionID
		if modelInfo.sameAsText {
			visTxt = "(同文本模型)"
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
			io.Printf("     视觉模型: %s%s%s\n", visTxt, provTxt, capTxt)
		}
		io.Println()
	}

	io.Println("──────────────────────────────────────")
	io.Println("  [S] 切换模式    [C] 创建模式")
	io.Println("  [E] 编辑模式    [D] 删除模式")
	io.Println("  [Q] 退出")
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
		io.Printf("────────── 编辑模式: %s ──────────\n", modeName)
		io.Println()

		// Option 1: Prompt sections
		io.Printf("  [1] 提示词节 (%d 节)\n", len(mode.Sections))
		// Option 2: Tool modes
		toolModeCount := 0
		if mode.ToolModes != nil {
			toolModeCount = len(mode.ToolModes)
		}
		io.Printf("  [2] 工具限制 (%d 项设置)\n", toolModeCount)

		// Option 3: Model bindings
		modelInfo := h.resolveModeModel(mode)
		io.Println("  [3] 模型绑定")
		if modelInfo.textID != "" {
			io.Printf("       文本: %s [%s]%s\n", modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		}
		visID := modelInfo.visionID
		if modelInfo.sameAsText {
			visID = "(同文本模型)"
		}
		io.Printf("       视觉: %s\n", visID)

		// Option 4: Parameter overrides
		io.Println("  [4] 参数覆盖")
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
			io.Printf("      (%d 项覆盖)\n", paramCount)
		} else {
			io.Println("      (全部使用全局值)")
		}

		io.Println()
		io.Println("──────────────────────────────────────")
		io.Printf("  [S] 切换到此模式\n")
		io.Printf("  [D] 删除此模式\n")
		io.Println("  [B] 返回上级  [Q] 退出")
		io.Println("──────────────────────────────────────")
		io.Print("请选择: ")

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
			io.Println("  无效输入")
		}
	}
}

// doSwitch switches to a mode and applies its config.
func (h *ModeHandler) doSwitch(name string) {
	h.cfg.LLM.WorkMode = name
	if err := h.cfg.Save(); err != nil {
		h.io().Printf("  ❌ 保存配置失败: %v\n", err)
		return
	}
	if h.ag != nil {
		h.ag.SyncToolModes(h.cfg)
		h.ag.SetConfig(h.cfg)
		h.ag.ApplyWorkModeConfig()
	}
	h.io().Printf("  ✅ 已切换到模式: %s\n", name)
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
		h.io().Printf("  ❌ 模式 %s 不存在\n", name)
		return
	}
	io := h.io()
	io.Printf("  确定要删除模式 '%s'? (y/N): ", name)
	confirm := strings.TrimSpace(strings.ToLower(h.readLine()))
	if confirm != "y" && confirm != "yes" {
		io.Println("  已取消")
		return
	}
	if h.cfg.LLM.WorkMode == name {
		h.cfg.LLM.WorkMode = "act"
	}
	h.cfg.WorkModes = append(h.cfg.WorkModes[:idx], h.cfg.WorkModes[idx+1:]...)
	if err := h.cfg.Save(); err != nil {
		io.Printf("  ❌ 保存失败: %v\n", err)
		return
	}
	io.Printf("  ✅ 已删除模式: %s\n", name)
}

// showToolModesWizard shows and manages tool modes for a mode.
func (h *ModeHandler) showToolModesWizard(modeName string) {
	// Reuse handleModeTools with empty args to display list
	result, err := h.handleModeTools(modeName, nil)
	if err == nil && result != "" {
		h.io().Print(result)
	}
	h.io().Println("\n回车返回上级菜单")
	h.readLine()
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
		io.Printf("────────── 模型绑定: %s ──────────\n", modeName)
		io.Println()

		// Resolve and display current model info
		modelInfo := h.resolveModeModel(mode)
		if mode.ModelID != nil {
			io.Printf("  文本模型: %s [%s]%s\n", modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		} else {
			io.Printf("  文本模型: %s [%s]%s (全局)\n", modelInfo.textID, modelInfo.textProvider, modelInfo.textCaps)
		}
		visDesc := modelInfo.visionID
		if modelInfo.sameAsText {
			visDesc = "(同文本模型)"
		}
		io.Printf("  视觉模型: %s\n", visDesc)
		io.Println()

		io.Println("  [1] 设置文本模型")
		if mode.ModelID != nil {
			io.Println("  [2] 解除文本模型绑定")
		}
		io.Println("  [3] 设置视觉模型")
		if mode.VisionModelID != nil {
			io.Println("  [4] 解除视觉模型绑定")
		}

		io.Println()
		io.Println("  [B] 返回  [Q] 退出")
		io.Print("请选择: ")

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
		default:
			io.Println("  无效输入")
		}
	}
}

// selectModelInteractive shows a numbered list of available models for the user to choose.
func (h *ModeHandler) selectModelInteractive(modeName, bindType string) {
	io := h.io()
	if len(h.cfg.Models) == 0 {
		io.Println("  未配置任何模型。请先使用 .model add 添加模型。")
		io.Print("\n按回车继续...")
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
	io.Printf("  选择 %s 模型 (输入编号):\n\n", bindType)
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
	io.Print("  请输入编号 (0 取消): ")

	input := strings.TrimSpace(h.readLine())
	if input == "" || input == "0" {
		return
	}

	// Try to interpret as priority-based number
	num, err := strconv.Atoi(input)
	if err != nil || num < 0 {
		io.Println("  无效输入")
		return
	}

	// sorted is already sorted by priority from above
	idx := num - 1
	if idx < 0 || idx >= len(sorted) {
		io.Println("  无效编号")
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
		io.Printf("────────── 参数覆盖: %s ──────────\n", modeName)
		io.Println()

		fmtT := func(name string, v *float64) string {
			if v != nil {
				return fmt.Sprintf("  %s: %.2f", name, *v)
			}
			return fmt.Sprintf("  %s: (未覆盖)", name)
		}
		fmtI := func(name string, v *int) string {
			if v != nil {
				return fmt.Sprintf("  %s: %d", name, *v)
			}
			return fmt.Sprintf("  %s: (未覆盖)", name)
		}
		fmtS := func(name string, v *string) string {
			if v != nil {
				return fmt.Sprintf("  %s: %s", name, *v)
			}
			return fmt.Sprintf("  %s: (未覆盖)", name)
		}
		fmtB := func(name string, v *bool) string {
			if v != nil {
				val := "off"
				if *v {
					val = "on"
				}
				return fmt.Sprintf("  %s: %s", name, val)
			}
			return fmt.Sprintf("  %s: (未覆盖)", name)
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
		io.Println("  [R] 重置全部  [B] 返回  [Q] 退出")
		io.Print("请选择编号设置参数 (或 R/B/Q): ")

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
			io.Println("  无效输入")
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
			io.Printf("  当前已设置%s。输入 'r' 重置为全局默认，或输入新值: ", paramKeys[input])
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
			io.Printf("  请输入%s的值: ", paramKeys[input])
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
		io.Print("\n  新建工作模式名称 (输入 Q 取消): ")
		name := h.readLine()
		if strings.ToUpper(name) == "Q" || strings.ToUpper(name) == "QUIT" || name == ".." {
			io.Println("  已取消")
			return "", nil
		}
		if name == "" {
			io.Println("  名称不能为空")
			continue
		}
		if name == "default" {
			io.Println("  不能创建名为 'default' 的模式")
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
			io.Printf("  模式 '%s' 已存在\n", name)
			continue
		}
		// Name is valid - proceed
		io.Print("  模式描述 (可选，输入 Q 取消): ")
		desc := h.readLine()
		if strings.ToUpper(desc) == "Q" || strings.ToUpper(desc) == "QUIT" || desc == ".." {
			io.Println("  已取消")
			return "", nil
		}

		sections := h.interactiveSelectSections("选择此模式要包含的节 (输入编号切换，空行继续，输入 Q 取消):")

		newMode := config.WorkMode{
			Name:        name,
			Description: desc,
			Sections:    sections,
		}
		h.cfg.WorkModes = append(h.cfg.WorkModes, newMode)
		if err := h.cfg.Save(); err != nil {
			io.Printf("  ❌ 保存失败: %v\n", err)
			return "", nil
		}

		io.Printf("  ✅ 已创建模式: %s\n", name)

		// Ask if user wants to configure model/params now
		io.Print("  是否现在配置模型和参数? (Y/n): ")
		confirm := strings.TrimSpace(strings.ToLower(h.readLine()))
		if confirm != "n" && confirm != "no" {
			h.showModeDetail(name)
		}
		return "", nil
	}
}

// interactiveRemoveWizard interactively selects and removes a mode.
func (h *ModeHandler) interactiveRemoveWizard() {
	selected, err := h.selectModeByNumber("选择要删除的模式:")
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
			sb.WriteString(fmt.Sprintf("      节数: %d\n", len(m.Sections)))
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
	io.Print("\n  请选择 (输入编号): ")

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
	selected, err := h.selectModeByNumber("选择要切换的工作模式:")
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

	io.Print("\n  新建工作模式名称: ")
	name := h.readLine()
	if name == "" {
		return "", fmt.Errorf("名称不能为空")
	}

	// Check duplicates
	for _, m := range h.cfg.WorkModes {
		if m.Name == name {
			return "", fmt.Errorf("%s", i18n.T(i18n.KeyModeExists))
		}
	}
	if name == "default" {
		return "", fmt.Errorf("不能创建名为 'default' 的模式")
	}

	io.Print("  模式描述 (可选): ")
	desc := h.readLine()

	// Select sections
	sections := h.interactiveSelectSections("选择此模式要包含的节 (输入编号切换，空行继续):")

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
		io.Println("  操作说明:")
		io.Println("    [编号]   - 切换选择/取消该节")
		io.Println("    +<编号>  - 将节上移一位 (如 +3)")
		io.Println("    -<编号>  - 将节下移一位 (如 -3)")
		io.Println("    <回车>   - 完成选择")
		io.Print("\n  请输入: ")

		input := h.readLine()
		if input == "" {
			break
		}

		// Handle move up: +N
		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(allSections) {
				io.Println("  无效编号")
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
				io.Println("  无效编号")
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
			io.Println("  无效输入")
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
		selected, err := h.selectModeByNumber("选择要编辑的工作模式:")
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
		io.Printf("\n  编辑模式: %s\n", mode.Name)
		io.Println("  当前节顺序:")
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
			io.Println("\n  备选节:")
			for avNum, ae := range availList {
				io.Printf("    [%d] %s\n", avNum+1, ae.name)
			}
		}
		io.Println()
		io.Println("  操作说明:")
		io.Println("    +<序号>  - 上移 (如 +2)")
		io.Println("    -<序号>  - 下移 (如 -3)")
		io.Println("    a<编号>  - 添加未包含的节 (如 a5)")
		io.Println("    d<序号>  - 移除此节 (如 d2)")
		io.Println("    v<序号>  - 查看节内容 (如 v3)")
		io.Println("    p        - 预览最终完整提示词")
		io.Println("    完成    - 保存并退出")
		io.Print("\n  请输入: ")

		input := h.readLine()
		if input == "" || input == "完成" {
			break
		}

		if strings.HasPrefix(input, "+") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println("  无效序号")
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
				io.Println("  无效序号")
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
				io.Println("  无效编号")
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
				io.Println("  无效序号")
				continue
			}
			pos := num - 1
			currentIndices = append(currentIndices[:pos], currentIndices[pos+1:]...)
			continue
		}

		if strings.HasPrefix(input, "v") {
			num, err := strconv.Atoi(input[1:])
			if err != nil || num < 1 || num > len(currentIndices) {
				io.Println("  无效序号")
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
			io.Println("\n  ==== 完整提示词预览 ====")
			io.Print(h.previewFullPrompt(currentIndices, allSections))
			io.Println("\n  =======================")
			continue
		}

		io.Println("  无效输入")
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

	return fmt.Sprintf("已更新模式 %s 的节顺序 (%d 节)", mode.Name, len(mode.Sections)), nil
}

// interactiveRemove removes a mode interactively or by name.
func (h *ModeHandler) interactiveRemove(args []string) (string, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		selected, err := h.selectModeByNumber("选择要删除的工作模式:")
		if err != nil {
			return "", err
		}
		name = selected.Name
	}

	// Confirm deletion
	h.io().Printf("  确定要删除工作模式 '%s'? (y/N): ", name)
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
			content = content[:500] + "...(截断)"
		}
		return content
	}
	// Check custom prompt sections
	for _, ps := range h.cfg.PromptSections {
		if ps.Name == name && ps.Content != "" {
			return ps.Content
		}
	}
	return "(内容来自 i18n 内置资源，共 0 字符)"
}

// previewFullPrompt concatenates all current sections in order.
func (h *ModeHandler) previewFullPrompt(indices []int, allSections []string) string {
	var sb strings.Builder
	for i, idx := range indices {
		name := allSections[idx]
		sb.WriteString(fmt.Sprintf("\n==== [%d] %s ====\n", i+1, name))
		content := h.previewSection(name)
		if len(content) > 300 {
			content = content[:300] + "...(截断)"
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
		return "", fmt.Errorf("未找到工作模式: %s", modeName)
	}

	// No args: list tools
	if len(args) == 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("模式 %s 的工具限制:\n", modeName))
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
		sb.WriteString(fmt.Sprintf("  默认: %s\n\n", defaultMode))
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
		return fmt.Sprintf("已重置模式 %s 的工具设置为默认", modeName), nil
	}

	// Set specific tool mode: <method> <auto|confirm|disabled>
	if len(args) >= 2 {
		method := args[0]
		value := args[1]
		if value != "auto" && value != "confirm" && value != "disabled" {
			return "", fmt.Errorf("无效的工具模式 %q，请使用 auto/confirm/disabled", value)
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
		return fmt.Sprintf("已设置模式 %s 的工具 %s → %s", modeName, method, value), nil
	}

	return "", fmt.Errorf("用法: .mode %s tools [<方法名> <auto|confirm|disabled>]", modeName)
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
		return "", fmt.Errorf("未找到模式: %s", modeName)
	}

	// No args: show current model bindings
	if len(args) == 0 || args[0] == "show" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("模式 %s 的模型绑定:\n", modeName))
		if mode.ModelID != nil {
			sb.WriteString(fmt.Sprintf("  文本模型: %s\n", *mode.ModelID))
		} else {
			sb.WriteString("  文本模型: (使用全局最高优先级)\n")
		}
		if mode.VisionModelID != nil {
			sb.WriteString(fmt.Sprintf("  视觉模型: %s\n", *mode.VisionModelID))
		} else {
			sb.WriteString("  视觉模型: (使用文本模型或全局)\n")
		}
		return sb.String(), nil
	}

	// list: show available models
	if args[0] == "list" {
		if len(h.cfg.Models) == 0 {
			return "未配置任何模型。使用 .model add 添加模型。", nil
		}
		var sb strings.Builder
		sb.WriteString("可用的模型:\n\n")
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
				sb.WriteString(fmt.Sprintf("    能力: %s | 优先级: %d\n", caps, m.Priority))
			} else {
				sb.WriteString(fmt.Sprintf("    优先级: %d\n", m.Priority))
			}
		}
		sb.WriteString("\n使用 .mode <名称> model text <模型ID> 绑定文本模型\n")
		sb.WriteString("使用 .mode <名称> model vision <模型ID> 绑定视觉模型\n")
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
			default:
				return "", fmt.Errorf("无效的绑定类型: %s (使用 text 或 vision)", target)
			}
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("cannot save config: %w", err)
			}
			if h.ag != nil {
				h.ag.ApplyWorkModeConfig()
			}
			return fmt.Sprintf("已解除模式 %s 的 %s 模型绑定", modeName, target), nil
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
			return "", fmt.Errorf("模型 %s 不存在。使用 .mode %s model list 查看可用模型", value, modeName)
		}

		switch target {
		case "text":
			mode.ModelID = &value
		case "vision":
			mode.VisionModelID = &value
		default:
			return "", fmt.Errorf("无效的绑定类型: %s (使用 text 或 vision)", target)
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil {
			h.ag.ApplyWorkModeConfig()
		}
		return fmt.Sprintf("已设置模式 %s 的 %s 模型为: %s", modeName, target, value), nil
	}

	return "", fmt.Errorf("用法: .mode %s model [text|vision <模型ID>|none|list]", modeName)
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
		return "", fmt.Errorf("未找到模式: %s", modeName)
	}

	// No args: show current parameter overrides
	if len(args) == 0 || args[0] == "show" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("模式 %s 的参数覆盖:\n", modeName))
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
		if sb.Len() == len(fmt.Sprintf("模式 %s 的参数覆盖:\n", modeName)) {
			sb.WriteString("  (无覆盖，全部使用全局默认值)\n")
		}
		sb.WriteString("\n支持设置的参数: temperature, max_tokens, top_p, top_k, repetition_penalty,\n")
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
		return fmt.Sprintf("已重置模式 %s 的所有参数覆盖", modeName), nil
	}

	// reset <key>: reset one parameter
	if args[0] == "reset" {
		if len(args) < 2 {
			return "", fmt.Errorf("用法: .mode %s param reset <key>", modeName)
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
			return "", fmt.Errorf("未知参数: %s", key)
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil && modeName == h.cfg.LLM.WorkMode {
			h.ag.ApplyWorkModeConfig()
		}
		return fmt.Sprintf("已重置模式 %s 的参数 %s", modeName, key), nil
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
				return "", fmt.Errorf("无效的 temperature 值: %s", value)
			}
			mode.Temperature = &v
			saved = true
		case "max_tokens":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf("无效的 max_tokens 值: %s", value)
			}
			mode.MaxTokens = &v
			saved = true
		case "top_p":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", fmt.Errorf("无效的 top_p 值: %s", value)
			}
			mode.TopP = &v
			saved = true
		case "top_k":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf("无效的 top_k 值: %s", value)
			}
			mode.TopK = &v
			saved = true
		case "repetition_penalty":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", fmt.Errorf("无效的 repetition_penalty 值: %s", value)
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
				return "", fmt.Errorf("无效的 thinking 值: %s (使用 on/off)", value)
			}
			saved = true
		case "reasoning_effort":
			if value != "low" && value != "medium" && value != "high" {
				return "", fmt.Errorf("无效的 reasoning_effort 值: %s (使用 low/medium/high)", value)
			}
			mode.ReasoningEffort = &value
			saved = true
		case "max_iterations":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf("无效的 max_iterations 值: %s", value)
			}
			mode.MaxIterations = &v
			saved = true
		case "context_limit":
			v, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf("无效的 context_limit 值: %s", value)
			}
			mode.ContextLimit = &v
			saved = true
		case "tool_call_mode":
			if value != "openai" && value != "xml" {
				return "", fmt.Errorf("无效的 tool_call_mode 值: %s (使用 openai/xml)", value)
			}
			mode.ToolCallMode = &value
			saved = true
		default:
			return "", fmt.Errorf("未知参数: %s (支持: temperature, max_tokens, top_p, top_k, repetition_penalty, thinking, reasoning_effort, max_iterations, context_limit, tool_call_mode)", key)
		}

		if !saved {
			return "", fmt.Errorf("无法设置参数 %s", key)
		}

		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("cannot save config: %w", err)
		}
		if h.ag != nil && modeName == h.cfg.LLM.WorkMode {
			h.ag.ApplyWorkModeConfig()
		}

		return fmt.Sprintf("已设置模式 %s 的参数 %s = %s", modeName, key, value), nil
	}

	return "", fmt.Errorf("用法: .mode %s param <参数名> <参数值>", modeName)
}
