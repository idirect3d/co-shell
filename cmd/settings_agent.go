// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-05-21
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
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// handleAgentSetting handles agent identity and behavior settings: name, description,
// principles, max-iterations, max-retries, memory-enabled, plan-enabled, subagent-enabled,
// context-limit, context-start.
func (h *SettingsHandler) handleAgentSetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "name":
		if len(args) < 2 {
			name := h.cfg.LLM.AgentName
			if name == "" {
				name = "co-shell"
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_101), name), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentName = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetName(value)
		log.Info("Agent name set to %s", value)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_102), value), nil

	case "description":
		workMode := h.cfg.LLM.WorkMode
		if workMode == "" {
			workMode = "act"
		}
		if len(args) < 2 {
			// Display current description for this mode
			desc := ""
			if h.cfg.LLM.ModeDescriptions != nil {
				if md, ok := h.cfg.LLM.ModeDescriptions[workMode]; ok && md != "" {
					desc = md
				}
			}
			if desc == "" {
				desc = h.cfg.LLM.AgentDescription
			}
			if desc == "" {
				desc = i18n.T(i18n.KeyAgentDefaultDescription)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_103), workMode, desc), nil
		}
		value := strings.Join(args[1:], " ")
		// Set mode-specific description
		if h.cfg.LLM.ModeDescriptions == nil {
			h.cfg.LLM.ModeDescriptions = make(map[string]string)
		}
		h.cfg.LLM.ModeDescriptions[workMode] = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Agent description set for mode %s: %s", workMode, value)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_104), workMode, value), nil

	case "principles":
		if len(args) < 2 {
			principles := h.cfg.LLM.AgentPrinciples
			if principles == "" {
				principles = ""
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_105), principles), nil
		}
		value := strings.Join(args[1:], " ")
		h.cfg.LLM.AgentPrinciples = value
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Agent principles set to %s", value)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_106), value), nil

	case "max-iterations":
		if len(args) < 2 {
			maxIterStr := fmt.Sprintf("%d", h.cfg.LLM.MaxIterations)
			if h.cfg.LLM.MaxIterations <= 0 {
				maxIterStr = i18n.T(i18n.KeySettingCmd_107)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_108), maxIterStr), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_109), args[1])
		}
		if n < -1 || n == 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_110))
		}
		h.cfg.LLM.MaxIterations = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetMaxIterations(n)
		log.Info("Max iterations set to %d", n)
		maxIterStr := fmt.Sprintf("%d", n)
		if n == -1 {
			maxIterStr = i18n.T(i18n.KeySettingCmd_111)
		}
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_112), maxIterStr), nil

	case "max-retries":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_113), h.cfg.LLM.MaxRetries), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_114), args[1])
		}
		if n < 0 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_115))
		}
		h.cfg.LLM.MaxRetries = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Max retries set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_116), n), nil

	case "memory-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.MemoryEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_117), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.MemoryEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.MemoryEnabled = false
		default:
			return "", fmt.Errorf("usage: .set memory-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetMemoryEnabled(h.cfg.LLM.MemoryEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.MemoryEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Memory enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_118), status), nil

	case "plan-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.PlanEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_119), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.PlanEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.PlanEnabled = false
		default:
			return "", fmt.Errorf("usage: .set plan-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetPlanEnabled(h.cfg.LLM.PlanEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.PlanEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Plan enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_120), status), nil

	case "shell-session-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.ShellSessionEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_121), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShellSessionEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShellSessionEnabled = false
		default:
			return "", fmt.Errorf("usage: .set shell-session-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetShellEnabled(h.cfg.LLM.ShellSessionEnabled)
		if !h.cfg.LLM.ShellSessionEnabled {
			h.agent.CloseShellSession()
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShellSessionEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Shell session enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_122), status), nil

	case "shell-session-timeout":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_123), h.cfg.LLM.ShellSessionTimeout), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_124), args[1])
		}
		h.cfg.LLM.ShellSessionTimeout = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell session timeout set to %d", n)
		if n == 0 {
			return i18n.T(i18n.KeySettingCmd_125), nil
		}
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_126), n), nil

	case "subagent-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOn)
			if !h.cfg.LLM.SubAgentEnabled {
				status = i18n.T(i18n.KeyOff)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_127), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.SubAgentEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.SubAgentEnabled = false
		default:
			return "", fmt.Errorf("usage: .set subagent-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetSubAgentEnabled(h.cfg.LLM.SubAgentEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.SubAgentEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("SubAgent enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_128), status), nil

	case "context-limit":
		if len(args) < 2 {
			limitStr := fmt.Sprintf("%d", h.cfg.LLM.ContextLimit)
			if h.cfg.LLM.ContextLimit == 0 {
				limitStr = i18n.T(i18n.KeyOff)
			} else if h.cfg.LLM.ContextLimit == -1 {
				limitStr = i18n.T(i18n.KeyUnlimited)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_129), limitStr), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_130), args[1])
		}
		if n < -1 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_131))
		}
		h.cfg.LLM.ContextLimit = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Context limit set to %d", n)
		return i18n.TF(i18n.KeyContextLimitUpdated, n, n), nil

	case "shell-vt-rows":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_132), h.cfg.LLM.ShellVTRows), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 5 || n > 200 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_133), args[1])
		}
		h.cfg.LLM.ShellVTRows = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell VT rows set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_134), n), nil

	case "shell-vt-cols":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_135), h.cfg.LLM.ShellVTCols), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 20 || n > 500 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_136), args[1])
		}
		h.cfg.LLM.ShellVTCols = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Shell VT cols set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_137), n), nil

	case "context-reorganize-threshold":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_138), h.cfg.LLM.ContextReorganizeThreshold), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 || n > 100 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_139), args[1])
		}
		h.cfg.LLM.ContextReorganizeThreshold = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Context reorganize threshold set to %d", n)
		if n == 0 {
			return i18n.T(i18n.KeySettingCmd_140), nil
		}
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_141), n), nil

	case "context-policy":
		if len(args) < 2 {
			mode := i18n.T(i18n.KeyContextPolicyTask)
			if h.cfg.LLM.ContextPolicy == "window" {
				mode = i18n.T(i18n.KeyContextPolicyWindow)
			} else if h.cfg.LLM.ContextPolicy == "smart" {
				mode = i18n.T(i18n.KeyContextPolicySmart)
			} else if h.cfg.LLM.ContextPolicy == "reorganize" {
				mode = i18n.T(i18n.KeyContextPolicyReorganize)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_142), mode), nil
		}
		switch args[1] {
		case "window", "task", "smart", "reorganize":
			h.cfg.LLM.ContextPolicy = args[1]
		default:
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_143), args[1])
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		modeDesc := i18n.T(i18n.KeyContextPolicyTask)
		switch args[1] {
		case "window":
			modeDesc = i18n.T(i18n.KeyContextPolicyWindow)
		case "smart":
			modeDesc = i18n.T(i18n.KeyContextPolicySmart)
		case "reorganize":
			modeDesc = i18n.T(i18n.KeyContextPolicyReorganize)
		}
		log.Info("Context policy set to %s (%s)", args[1], modeDesc)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_144), args[1], modeDesc), nil

	case "no-tool-action":
		if len(args) < 2 {
			val := h.cfg.LLM.NoToolAction
			if val == "" {
				val = "retry"
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_145), val), nil
		}
		validValues := map[string]bool{"exit": true, "retry": true, "prompt": true}
		val := args[1]
		if !validValues[val] {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_146), val)
		}
		h.cfg.LLM.NoToolAction = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("No-tool-action set to %s", val)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_147), val), nil

	case "parse-error-action":
		if len(args) < 2 {
			val := h.cfg.LLM.ParseErrorAction
			if val == "" {
				val = "retry"
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_148), val), nil
		}
		validValues := map[string]bool{"exit": true, "retry": true, "prompt": true}
		val := args[1]
		if !validValues[val] {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_149), val)
		}
		h.cfg.LLM.ParseErrorAction = val
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Parse-error-action set to %s", val)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_150), val), nil

	case "show-parse-error-raw":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.ShowParseErrorRaw {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_337), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.ShowParseErrorRaw = true
		case "off", "0", "false", "no":
			h.cfg.LLM.ShowParseErrorRaw = false
		default:
			return "", fmt.Errorf("usage: .set show-parse-error-raw on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.ShowParseErrorRaw {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Show parse error raw set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_337), status), nil

	case "browser-enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.BrowserEnabled {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_151), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.BrowserEnabled = true
		case "off", "0", "false", "no":
			h.cfg.LLM.BrowserEnabled = false
		default:
			return "", fmt.Errorf("usage: .set browser-enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetBrowserEnabled(h.cfg.LLM.BrowserEnabled)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.BrowserEnabled {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Browser enabled set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_152), status), nil

	case "browser-port":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_153), h.cfg.LLM.BrowserPort), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_154), args[1])
		}
		if n < 1 || n > 65535 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_155))
		}
		h.cfg.LLM.BrowserPort = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Browser port set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_156), n), nil

	case "browser-headless":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.BrowserHeadless {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_157), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.BrowserHeadless = true
		case "off", "0", "false", "no":
			h.cfg.LLM.BrowserHeadless = false
		default:
			return "", fmt.Errorf("usage: .set browser-headless on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.BrowserHeadless {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Browser headless set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_158), status), nil

	case "read-file-max-size":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_159), h.cfg.LLM.ReadFileMaxSize, h.cfg.LLM.ReadFileMaxSize/1024), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1024 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_160))
		}
		h.cfg.LLM.ReadFileMaxSize = n
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to save config: %v", err)
		}
		log.Info("Read file max size set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_161), n, n/1024), nil

	case "browser-max-html-size":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_162), h.cfg.LLM.BrowserMaxHTMLSize, h.cfg.LLM.BrowserMaxHTMLSize/1024), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1024 {
			return "", errors.New(i18n.T(i18n.KeySettingCmd_163))
		}
		h.cfg.LLM.BrowserMaxHTMLSize = n
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to save config: %v", err)
		}
		log.Info("Browser max HTML size set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_164), n, n/1024), nil

	case "input-mode":
		if len(args) < 2 {
			mode := h.cfg.LLM.InputMode
			if mode == "" {
				mode = "enhanced"
			}
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_165), mode), nil
		}
		switch args[1] {
		case "enhanced", "stdio":
			h.cfg.LLM.InputMode = args[1]
		default:
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_166), args[1])
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Input mode set to %s", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_167), args[1]), nil

	case "excel-max-sessions":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_168), h.cfg.LLM.ExcelMaxSessions), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 50 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_169), args[1])
		}
		h.cfg.LLM.ExcelMaxSessions = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Excel max sessions set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_170), n), nil

	case "excel-max-cells":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_171), h.cfg.LLM.ExcelMaxCells), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 10 || n > 100000 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_172), args[1])
		}
		h.cfg.LLM.ExcelMaxCells = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Excel max cells set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_173), n), nil

	case "docx-max-sessions":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_174), h.cfg.LLM.DocxMaxSessions), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 50 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_169), args[1])
		}
		h.cfg.LLM.DocxMaxSessions = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetConfig(h.cfg)
		log.Info("Docx max sessions set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_175), n), nil

	case "docx-max-read-paras":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_176), h.cfg.LLM.DocxMaxReadParas), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 10 || n > 5000 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_177), args[1])
		}
		h.cfg.LLM.DocxMaxReadParas = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Docx max read paras set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_178), n), nil

	case "visual-analysis-max-images":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_179), h.cfg.LLM.VisualAnalysisMaxImages), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 20 {
			return "", fmt.Errorf(i18n.T(i18n.KeySettingCmd_180), args[1])
		}
		h.cfg.LLM.VisualAnalysisMaxImages = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("Visual analysis max images set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_181), n), nil

	case "debug":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.LLM.DebugMode {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyDebugMode)+": %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.LLM.DebugMode = true
		case "off", "0", "false", "no":
			h.cfg.LLM.DebugMode = false
		default:
			return "", fmt.Errorf("usage: .set agent debug on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		h.agent.SetDebugMode(h.cfg.LLM.DebugMode)
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.LLM.DebugMode {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("Debug mode set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeySettingCmd_182), status), nil

	default:
		return "", fmt.Errorf("unknown agent setting: %s", subcommand)
	}
}
