// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-06-04
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleSafetySetting handles safety and confirmation settings: confirm-tool,
// error-max-single-count, error-max-type-count, loop-detect-*, dedup-*.
func (h *SettingsHandler) handleSafetySetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "confirm-tool":
		if len(args) < 2 {
			return h.showToolModes(), nil
		}
		toolName := args[1]
		switch toolName {
		case "on", "1", "true", "yes":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "confirm"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", "confirm")
			log.Info("Confirm tool set to on (confirm)")
			return fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmEnabled), i18n.T(i18n.KeyOn)), nil
		case "off", "0", "false", "no":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "auto"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", "auto")
			log.Info("Confirm tool set to off (auto)")
			return fmt.Sprintf("%s\n%s", fmt.Sprintf(i18n.T(i18n.KeyCmdConfirmDisabled), i18n.T(i18n.KeyOff)), i18n.T(i18n.KeyCmdConfirmDisableWarn)), nil
		case "reset":
			// Reset all tool mode settings: clear config and re-sync from defaults.
			h.cfg.LLM.ToolModes = make(map[string]string)
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SyncToolModes(h.cfg)
			log.Info("Confirm tool modes reset to defaults")
			return i18n.T(i18n.KeySettingCmd_183), nil

		case "confirm", "auto", "disabled":
			// Global override: set "default" key. SyncToolModes will apply this
			// value to ALL tools regardless of their individual settings.
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = toolName
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SetToolMode("", toolName)
			log.Info("Confirm tool global default set to %s", toolName)
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_184), toolName), nil
		case "custom":
			// "custom" means no global override — each tool uses its own setting.
			// Save "default": "custom" to config so SyncToolModes knows not to apply
			// a global override, and the value persists across restarts.
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "custom"
			if err := h.cfg.Save(); err != nil {
				return "", err
			}
			h.agent.SyncToolModes(h.cfg)
			log.Info("Confirm tool default cleared, per-tool mode restored")
			return i18n.T(i18n.KeySettingCmd_185), nil
		}
		if len(args) < 3 {
			mode := "confirm"
			if v, ok := h.cfg.LLM.ToolModes[toolName]; ok {
				mode = v
			} else if v, ok := h.cfg.LLM.ToolModes["default"]; ok {
				// If global default is active, individual tool shows the global value
				mode = v
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_186), toolName, mode), nil
		}
		switch args[2] {
		case "on", "1", "true", "yes":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = "confirm"
		case "off", "0", "false", "no":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = "auto"
		case "confirm", "auto", "disabled":
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes[toolName] = args[2]
		default:
			return "", errors.New(i18n.T(i18n.KeySettingCmd_187))
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetToolMode(toolName, h.cfg.LLM.ToolModes[toolName])
		mode := h.cfg.LLM.ToolModes[toolName]
		log.Info("Confirm tool %s set to %s", toolName, mode)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_188), toolName, mode), nil

	case "error-max-single-count":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_189), h.cfg.LLM.ErrorMaxSingleCount), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_191))
		}
		h.cfg.LLM.ErrorMaxSingleCount = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Error max single count set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_192), n), nil

	case "error-max-type-count":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_193), h.cfg.LLM.ErrorMaxTypeCount), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_194))
		}
		h.cfg.LLM.ErrorMaxTypeCount = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Error max type count set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_195), n), nil

	case "loop-intervention":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_196), h.cfg.LLM.LoopIntervention), nil
		}
		switch args[1] {
		case "off", "retry", "prompt", "reorganize", "temperature", "random":
			h.cfg.LLM.LoopIntervention = args[1]
		default:
			return "", errors.New(i18n.T(i18n.KeySettingCmd_197))
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		// Sync the Agent's config pointer to ensure runtime reads the same value.
		// Agent's a.cfg may have been replaced by rebuildSystemPrompt() in earlier sessions,
		// causing a.cfg.LLM.LoopIntervention to be empty even when h.cfg has the correct value.
		h.agent.SetConfig(h.cfg)
		log.Info("Loop intervention set to %s, agent config synced", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_198), args[1]), nil

	case "loop-detect-threshold":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_199), h.cfg.LLM.LoopDetectThreshold), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if n < 1 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_200))
		}
		h.cfg.LLM.LoopDetectThreshold = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop detect threshold set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_201), n), nil

	// loop-temp-enabled is removed. Temperature adjustment is now controlled
	// by loop-intervention.

	case "loop-temp-step-up":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_202), h.cfg.LLM.LoopTempStepUp), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if v <= 0 || v > 1.0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_203))
		}
		h.cfg.LLM.LoopTempStepUp = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp step up set to %.2f", v)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_204), v), nil

	case "loop-temp-step-down":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_205), h.cfg.LLM.LoopTempStepDown), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if v <= 0 || v > 1.0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_206))
		}
		h.cfg.LLM.LoopTempStepDown = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp step down set to %.2f", v)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_207), v), nil

	case "loop-temp-max":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_208), h.cfg.LLM.LoopTempMax), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if v <= h.cfg.LLM.LoopTempMin || v > 2.0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_209))
		}
		h.cfg.LLM.LoopTempMax = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp max set to %.2f", v)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_210), v), nil

	case "loop-temp-min":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_211), h.cfg.LLM.LoopTempMin), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		if v >= h.cfg.LLM.LoopTempMax || v < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_212))
		}
		h.cfg.LLM.LoopTempMin = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop temp min set to %.2f", v)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_213), v), nil

	case "loop-judge-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.LoopJudgeEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_214), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.LoopJudgeEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.LoopJudgeEnabled = false
		default:
			return "", errors.New(i18n.T(i18n.KeySettingCmd_215))
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.LoopJudgeEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Loop judge enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_216), status), nil

	case "duplicate-content-threshold":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_217), h.cfg.LLM.DuplicateContentThreshold), nil
		}
		v, err := strconv.ParseFloat(args[1], 64)
		if err != nil || v < 0 || v > 1.0 {
			return "", fmt.Errorf(i18n.TF(i18n.KeySettingCmd_218, args[1]))
		}
		h.cfg.LLM.DuplicateContentThreshold = v
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Duplicate content threshold set to %.2f", v)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_219), v), nil

	// loop-reorganize-enabled is removed. Context reorganization is now controlled
	// by loop-intervention.

	case "loop-judge-timeout":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.TF(i18n.KeySettingCmd_220, h.cfg.LLM.LoopJudgeTimeout)), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid timeout %q, must be a non-negative integer", args[1])
		}
		h.cfg.LLM.LoopJudgeTimeout = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop judge timeout set to %d seconds", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_221), n), nil

	case "loop-single-line-length":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_222), h.cfg.LLM.LoopSingleLineLength), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		h.cfg.LLM.LoopSingleLineLength = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop single line length set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_223), n), nil

	case "loop-single-line-window":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_224), h.cfg.LLM.LoopSingleLineWindow), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		h.cfg.LLM.LoopSingleLineWindow = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop single line window set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_225), n), nil

	case "loop-single-line-block-limit":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_226), h.cfg.LLM.LoopSingleLineBlockLimit), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_190))
		}
		h.cfg.LLM.LoopSingleLineBlockLimit = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop single line block limit set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_227), n), nil

	case "loop-long-output-threshold":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_228), h.cfg.LLM.LoopLongOutputThreshold), nil
		}
		threshold, err := strconv.Atoi(args[1])
		if err != nil || threshold < 0 {
			return "", fmt.Errorf("invalid threshold %q, must be a non-negative integer", args[1])
		}
		h.cfg.LLM.LoopLongOutputThreshold = threshold
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Loop long output threshold set to %d", threshold)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_229), threshold), nil

	default:
		return "", fmt.Errorf("unknown safety setting: %s", subcommand)
	}
}

// showToolModes displays the current tool mode configuration.
func (h *SettingsHandler) showToolModes() string {
	var sb strings.Builder

	modeName := h.cfg.LLM.WorkMode
	if modeName == "" || modeName == "default" {
		modeName = "act"
	}
	sb.WriteString(fmt.Sprintf(i18n.TF(i18n.KeySettingCmd_230, modeName)))
	sb.WriteString(i18n.T(i18n.KeySettingCmd_231))

	// Use agent's effective toolModes if available (already computed by SyncToolModes)
	// Otherwise compute them the same way as SyncToolModes does.
	var effectiveModes map[string]string
	if h.agent != nil {
		// Access internal toolModes safely - for display only
		if tm := h.agent.ToolModes(); tm != nil {
			effectiveModes = tm
		}
	}
	if effectiveModes == nil {
		effectiveModes = agent.DefaultToolModes()
	}

	defaultMode := effectiveModes["default"]
	if defaultMode == "" {
		defaultMode = "confirm"
	}
	sb.WriteString(fmt.Sprintf(i18n.TF(i18n.KeySettingCmd_232, defaultMode)))

	allTools := make([]string, 0, len(agent.DefaultToolModes()))
	for name := range agent.DefaultToolModes() {
		if name != "default" {
			allTools = append(allTools, name)
		}
	}
	sort.Strings(allTools)

	for _, toolName := range allTools {
		mode := effectiveModes[toolName]
		if mode == "" {
			mode = defaultMode
		}
		sb.WriteString(fmt.Sprintf("  %-35s %s\n", toolName, mode))
	}

	return sb.String()
}
