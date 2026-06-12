// Author: L.Shuang
// Created: 2026-06-04
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// ConfigSubCommand defines a nested subcommand within a command entry.
type ConfigSubCommand struct {
	Name   string
	Desc   string
	Args   string // example args shown to the user
	Action func(args []string)
}

// ConfigGroup defines a group of configuration parameters.
type ConfigGroup struct {
	Name   string
	Params []ConfigParam
}

// ConfigParam defines a single configurable parameter or command entry.
// For command entries with sub-commands: Action is nil, SubCommands is set.
// For command entries without sub-commands: Action is set.
type ConfigParam struct {
	Name         string
	Options      []string
	CurrentValue func() string
	SetValue     func(value string) (string, error)
	ResetValue   func() string
	Action       func(args []string) // for command entries; args may be empty
	Desc         string
	SubCommands  []ConfigSubCommand // sub-commands shown as numbered list
}

// ConfigHandler handles the .config built-in command for the guided configuration wizard.
type ConfigHandler struct {
	cfg             *config.Config
	agent           *agent.Agent
	scanner         *bufio.Scanner
	mcpHandler      *MCPHandler
	ruleHandler     *RuleHandler
	memoryHandler   *MemoryHandler
	contextHandler  *ContextHandler
	listHandler     *ListHandler
	imageHandler    *ImageHandler
	planHandler     *PlanHandler
	sessionHandler  *SessionHandler
	modelHandler    *ModelHandler
	sectionHandler  *SectionHandler
	modeHandler     *ModeHandler
	settingsHandler *SettingsHandler
	simulateHandler *SimulateHandler
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cfg *config.Config, ag *agent.Agent) *ConfigHandler {
	return &ConfigHandler{
		cfg:             cfg,
		agent:           ag,
		simulateHandler: NewSimulateHandler(ag, cfg),
	}
}

// SetHandlers sets all command handlers for action entries.
func (h *ConfigHandler) SetHandlers(mcp *MCPHandler, rule *RuleHandler, mem *MemoryHandler,
	ctx *ContextHandler, lst *ListHandler, img *ImageHandler, plan *PlanHandler,
	sess *SessionHandler, mdl *ModelHandler, sec *SectionHandler, mode *ModeHandler,
	set *SettingsHandler) {
	h.mcpHandler = mcp
	h.ruleHandler = rule
	h.memoryHandler = mem
	h.contextHandler = ctx
	h.listHandler = lst
	h.imageHandler = img
	h.planHandler = plan
	h.sessionHandler = sess
	h.modelHandler = mdl
	h.sectionHandler = sec
	h.modeHandler = mode
	h.settingsHandler = set
}

// SetScanner sets a shared stdin scanner from REPL.
func (h *ConfigHandler) SetScanner(s *bufio.Scanner) {
	h.scanner = s
}

// Handle processes the .config command.
func (h *ConfigHandler) Handle(args []string) (string, error) {
	h.runWizard()
	return "", nil
}

// io returns the UserIO from the agent, falling back to DefaultUserIO.
func (h *ConfigHandler) io() agent.UserIO {
	return agent.GetIO(h.agent)
}

// readLine reads a line from stdin via UserIO or shared scanner.
func (h *ConfigHandler) readLine() string {
	if h.scanner != nil {
		if h.scanner.Scan() {
			return strings.TrimSpace(h.scanner.Text())
		}
		return ""
	}
	line, err := h.io().ReadLine()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// configGroups builds the full list of configuration groups.
func (h *ConfigHandler) configGroups() []ConfigGroup {
	return []ConfigGroup{
		{
			Name:   i18n.T(i18n.KeySettingsGroupIdentity),
			Params: h.identityParams(),
		},
		{
			Name:   i18n.T(i18n.KeySettingsGroupModel),
			Params: h.agentParams(),
		},
		{
			Name:   i18n.T(i18n.KeySettingsGroupDisplay),
			Params: h.displayParams(),
		},
		{
			Name:   i18n.T(i18n.KeySettingsGroupSafety),
			Params: h.safetyParams(),
		},
		{
			Name: i18n.T(i18n.KeySettingsGroupMemory),
			Params: append(h.memoryParams(),
				cmdEntry(".memory", i18n.T(i18n.KeyHelpMemory), "", h.memoryHandler.Handle, []ConfigSubCommand{
					{Name: "save", Desc: "保存一条记忆", Args: "<key> <value>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"save"}, a...)) }},
					{Name: "get", Desc: "获取一条记忆", Args: "<key>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"get"}, a...)) }},
					{Name: "search", Desc: "按前缀搜索记忆", Args: "<prefix>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"search"}, a...)) }},
					{Name: "delete", Desc: "删除一条记忆", Args: "<key>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"delete"}, a...)) }},
					{Name: "list", Desc: "列出所有记忆", Action: func(a []string) { runHandler(h.memoryHandler.Handle, []string{"list"}) }},
					{Name: "clear", Desc: "清空所有记忆", Action: func(a []string) { runHandler(h.memoryHandler.Handle, []string{"clear"}) }},
				}),
				cmdEntry(".context", i18n.T(i18n.KeyHelpContext), "", h.contextHandler.Handle, []ConfigSubCommand{
					{Name: "show", Desc: "显示当前上下文", Action: func(a []string) { runHandler(h.contextHandler.Handle, []string{"show"}) }},
					{Name: "reset", Desc: "重置上下文", Action: func(a []string) { runHandler(h.contextHandler.Handle, []string{"reset"}) }},
				}),
				cmdEntry(".session", i18n.T(i18n.KeyHelpSession), "", h.sessionHandler.Handle, []ConfigSubCommand{
					{Name: "info", Desc: "显示会话概要", Action: func(a []string) { runHandler(h.sessionHandler.Handle, []string{}) }},
				}),
				cmdEntry(".new", i18n.T(i18n.KeyHelpNew), "", func(args []string) (string, error) {
					h.agent.Reset()
					return "", nil
				}, nil),
				cmdEntry(".plan", i18n.T(i18n.KeyHelpPlan), "", h.planHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "查看当前任务计划", Action: func(a []string) { runHandler(h.planHandler.Handle, []string{}) }},
					{Name: "create", Desc: "创建新任务计划", Args: "<title>", Action: func(a []string) { runHandler(h.planHandler.Handle, append([]string{"create"}, a...)) }},
				}),
				cmdEntry(".db", i18n.T(i18n.KeyDBSubCmdDesc), "", h.settingsHandler.HandleDB, []ConfigSubCommand{
					{Name: "info", Desc: "查看数据库配置", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, []string{}) }},
					{Name: "host", Desc: "设置数据库地址", Args: "<host>", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, append([]string{"host"}, a...)) }},
					{Name: "port", Desc: "设置数据库端口", Args: "<port>", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, append([]string{"port"}, a...)) }},
					{Name: "migrate", Desc: "从本地 bbolt 迁移到 PostgreSQL", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, []string{"migrate"}) }},
				}),
				cmdEntry(".history", "查看用户输入命令历史", "", h.listHandler.HandleHistory, []ConfigSubCommand{
					{Name: "last", Desc: "查看最近 N 条历史", Args: "[N]", Action: func(a []string) { runHandler(h.listHandler.HandleHistory, append([]string{"last"}, a...)) }},
					{Name: "first", Desc: "查看最早 N 条历史", Args: "[N]", Action: func(a []string) { runHandler(h.listHandler.HandleHistory, append([]string{"first"}, a...)) }},
				}),
			),
		},
		{
			Name:   i18n.T(i18n.KeyWizardGroupDevTools),
			Params: h.devToolParams(),
		},
		{
			Name:   i18n.T(i18n.KeyWizardGroupModelMgr),
			Params: h.modelMgrParams(),
		},
		{
			Name:   i18n.T(i18n.KeyWizardGroupWorkMode),
			Params: h.workModeParams(),
		},
		{
			Name:   i18n.T(i18n.KeyWizardGroupMultimodal),
			Params: h.multimodalParams(),
		},
	}
}

// cmdEntry creates a ConfigParam for a command entry with optional sub-commands.
func cmdEntry(name, desc, usage string, handler func([]string) (string, error), subs []ConfigSubCommand) ConfigParam {
	p := ConfigParam{Name: name, Desc: desc}
	if len(subs) > 0 {
		// Has sub-commands: store them and set a simple action to show menu
		p.SubCommands = subs
		p.Action = func(args []string) {
			if len(args) > 0 {
				runHandler(handler, args)
			}
		}
	} else {
		// Simple action: call handler directly or show usage
		p.Action = func(args []string) {
			io := agent.DefaultIO()
			if handler != nil {
				result, err := handler(args)
				if err != nil {
					io.Printf("  ❌ %v\n", err)
					return
				}
				if result != "" {
					io.Println(result)
				}
			} else if usage != "" {
				io.Printf("  用法: %s %s\n", name, usage)
			}
		}
	}
	return p
}

// runHandler runs a handler function and displays result/error.
func runHandler(handler func([]string) (string, error), args []string) {
	io := agent.DefaultIO()
	result, err := handler(args)
	if err != nil {
		io.Printf("  ❌ %v\n", err)
		return
	}
	if result != "" {
		io.Println(result)
	}
}

// identityParams returns identity parameters.
func (h *ConfigHandler) identityParams() []ConfigParam {
	return []ConfigParam{
		{Name: "name", CurrentValue: func() string {
			n := h.cfg.LLM.AgentName
			if n == "" {
				return "co-shell"
			}
			return n
		}, SetValue: func(v string) (string, error) {
			h.cfg.LLM.AgentName = v
			h.agent.SetName(v)
			return i18n.TF(i18n.KeySettingsUpdated, "name", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.AgentName = ""
			h.agent.SetName("co-shell")
			return i18n.TF(i18n.KeySettingsUpdated, "name", "co-shell")
		}},
		{Name: "description", CurrentValue: func() string {
			d := h.cfg.LLM.AgentDescription
			if d == "" {
				return i18n.T(i18n.KeyDefault)
			}
			if len(d) > 50 {
				return d[:50] + "..."
			}
			return d
		}, SetValue: func(v string) (string, error) {
			h.cfg.LLM.AgentDescription = v
			return i18n.TF(i18n.KeySettingsUpdated, "description", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.AgentDescription = ""
			return i18n.TF(i18n.KeySettingsUpdated, "description", i18n.T(i18n.KeyDefault))
		}},
	}
}

// agentParams returns agent setting parameters.
func (h *ConfigHandler) agentParams() []ConfigParam {
	return []ConfigParam{
		{Name: "max-iterations", CurrentValue: func() string {
			n := h.cfg.LLM.MaxIterations
			if n <= 0 {
				return "1000"
			}
			return strconv.Itoa(n)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("请输入非负整数")
			}
			h.cfg.LLM.MaxIterations = n
			h.agent.SetMaxIterations(n)
			return i18n.TF(i18n.KeySettingsUpdated, "max-iterations", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.MaxIterations = 1000
			h.agent.SetMaxIterations(1000)
			return i18n.TF(i18n.KeySettingsUpdated, "max-iterations", "1000")
		}},
		{Name: "vision", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.VisionSupport), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.VisionSupport, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "vision", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.VisionSupport = false
			return i18n.TF(i18n.KeySettingsUpdated, "vision", "off")
		}},
		{Name: "thinking-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.ThinkingEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.ThinkingEnabled, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "thinking-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ThinkingEnabled = false
			return i18n.TF(i18n.KeySettingsUpdated, "thinking-enabled", "off")
		}},
		syncedOnOffParam(&h.cfg.LLM.ToolCallEnabled, "toolcall-enabled", func(v bool) { h.agent.SetToolCallEnabled(v) }),
		{Name: "toolcall-mode", Options: []string{"openai", "xml"}, CurrentValue: func() string {
			m := h.cfg.LLM.ToolCallMode
			if m == "" {
				return "openai"
			}
			return m
		}, SetValue: func(v string) (string, error) {
			switch v {
			case "openai", "xml":
				h.cfg.LLM.ToolCallMode = v
				h.agent.SetToolCallMode(v)
			default:
				return "", fmt.Errorf("请输入 openai 或 xml")
			}
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-mode", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ToolCallMode = "openai"
			h.agent.SetToolCallMode("openai")
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-mode", "openai")
		}},
		syncedOnOffParam(&h.cfg.LLM.PlanEnabled, "plan-enabled", func(v bool) { h.agent.SetPlanEnabled(v) }),
		syncedOnOffParam(&h.cfg.LLM.SubAgentEnabled, "subagent-enabled", func(v bool) { h.agent.SetSubAgentEnabled(v) }),
		{Name: "result-mode", Options: []string{"minimal", "explain", "analyze", "free"}, CurrentValue: func() string {
			return config.ResultModeString(config.ResultMode(h.cfg.LLM.ResultMode))
		}, SetValue: func(v string) (string, error) {
			if mode, ok := config.ParseResultMode(v); ok {
				h.cfg.LLM.ResultMode = int(mode)
				h.agent.SetResultMode(mode)
				return i18n.TF(i18n.KeySettingsUpdated, "result-mode", v), nil
			}
			return "", fmt.Errorf("%s", i18n.T(i18n.KeyConfigValMinExplAnFree))
		}, ResetValue: func() string {
			h.cfg.LLM.ResultMode = int(config.ResultModeMinimal)
			h.agent.SetResultMode(config.ResultModeMinimal)
			return i18n.TF(i18n.KeySettingsUpdated, "result-mode", "minimal")
		}},
		{Name: "shell-session-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.ShellSessionEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.ShellSessionEnabled, v); err != nil {
				return "", err
			}
			// Sync to agent: start or stop the VT session immediately
			h.agent.SetShellEnabled(h.cfg.LLM.ShellSessionEnabled)
			return i18n.TF(i18n.KeySettingsUpdated, "shell-session-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ShellSessionEnabled = false
			h.agent.SetShellEnabled(false)
			return i18n.TF(i18n.KeySettingsUpdated, "shell-session-enabled", "off")
		}},
		{Name: "shell-session-timeout", CurrentValue: func() string {
			n := h.cfg.LLM.ShellSessionTimeout
			if n <= 0 {
				return i18n.T(i18n.KeyUnlimited)
			}
			return strconv.Itoa(n)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("请输入非负整数")
			}
			h.cfg.LLM.ShellSessionTimeout = n
			return i18n.TF(i18n.KeySettingsUpdated, "shell-session-timeout", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ShellSessionTimeout = 0
			return i18n.TF(i18n.KeySettingsUpdated, "shell-session-timeout", i18n.T(i18n.KeyUnlimited))
		}},
		{Name: "shell-vt-rows", CurrentValue: func() string {
			n := h.cfg.LLM.ShellVTRows
			if n <= 0 {
				return "24"
			}
			return strconv.Itoa(n)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 5 || n > 200 {
				return "", fmt.Errorf("请输入 5-200 的整数")
			}
			h.cfg.LLM.ShellVTRows = n
			return i18n.TF(i18n.KeySettingsUpdated, "shell-vt-rows", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ShellVTRows = 24
			return i18n.TF(i18n.KeySettingsUpdated, "shell-vt-rows", "24")
		}},
		{Name: "shell-vt-cols", CurrentValue: func() string {
			n := h.cfg.LLM.ShellVTCols
			if n <= 0 {
				return "80"
			}
			return strconv.Itoa(n)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 20 || n > 500 {
				return "", fmt.Errorf("请输入 20-500 的整数")
			}
			h.cfg.LLM.ShellVTCols = n
			return i18n.TF(i18n.KeySettingsUpdated, "shell-vt-cols", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ShellVTCols = 80
			return i18n.TF(i18n.KeySettingsUpdated, "shell-vt-cols", "80")
		}},
		{Name: "input-mode", Options: []string{"enhanced", "stdio"}, CurrentValue: func() string {
			m := h.cfg.LLM.InputMode
			if m == "" {
				return "enhanced"
			}
			return m
		}, SetValue: func(v string) (string, error) {
			switch v {
			case "enhanced", "stdio":
				h.cfg.LLM.InputMode = v
			default:
				return "", fmt.Errorf("请输入 enhanced 或 stdio")
			}
			return i18n.TF(i18n.KeySettingsUpdated, "input-mode", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.InputMode = "enhanced"
			return i18n.TF(i18n.KeySettingsUpdated, "input-mode", "enhanced")
		}},
		// Browser settings
		syncedOnOffParam(&h.cfg.LLM.BrowserEnabled, "browser-enabled", func(v bool) { h.agent.SetBrowserEnabled(v) }),
		{Name: "browser-port", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.BrowserPort)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 65535 {
				return "", fmt.Errorf("请输入 1-65535 的整数")
			}
			h.cfg.LLM.BrowserPort = n
			return i18n.TF(i18n.KeySettingsUpdated, "browser-port", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.BrowserPort = 9222
			return i18n.TF(i18n.KeySettingsUpdated, "browser-port", "9222")
		}},
		{Name: "browser-headless", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.BrowserHeadless), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.BrowserHeadless, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "browser-headless", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.BrowserHeadless = false
			return i18n.TF(i18n.KeySettingsUpdated, "browser-headless", "off")
		}},
		// Rule management
		cmdEntry(".rule", "管理 AI 全局规则", "", h.ruleHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出所有规则", Action: func(a []string) { runHandler(h.ruleHandler.Handle, []string{}) }},
			{Name: "add", Desc: "添加新规则", Args: "<rule>", Action: func(a []string) { runHandler(h.ruleHandler.Handle, append([]string{"add"}, a...)) }},
			{Name: "remove", Desc: "删除规则", Args: "<index>", Action: func(a []string) { runHandler(h.ruleHandler.Handle, append([]string{"remove"}, a...)) }},
		}),
		// Search settings
		{Name: "search-max-line-length", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.SearchMaxLineLength)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.SearchMaxLineLength = n
			return i18n.TF(i18n.KeySettingsUpdated, "search-max-line-length", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.SearchMaxLineLength = 8192
			return i18n.TF(i18n.KeySettingsUpdated, "search-max-line-length", "8192")
		}},
		{Name: "search-max-result-bytes", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.SearchMaxResultBytes)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.SearchMaxResultBytes = n
			return i18n.TF(i18n.KeySettingsUpdated, "search-max-result-bytes", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.SearchMaxResultBytes = 65536
			return i18n.TF(i18n.KeySettingsUpdated, "search-max-result-bytes", "65536")
		}},
		{Name: "search-context-lines", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.SearchContextLines)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.SearchContextLines = n
			return i18n.TF(i18n.KeySettingsUpdated, "search-context-lines", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.SearchContextLines = 5
			return i18n.TF(i18n.KeySettingsUpdated, "search-context-lines", "5")
		}},
	}
}

// displayParams returns display parameters.
// Each on/off parameter's SetValue also syncs to the agent for immediate effect.
func (h *ConfigHandler) displayParams() []ConfigParam {
	return []ConfigParam{
		syncedOnOffParam(&h.cfg.LLM.EmojiEnabled, "emoji-enabled", h.agent.SetEmojiEnabled),
		syncedOnOffParam(&h.cfg.LLM.ShowLlmThinking, "show-llm-thinking", func(v bool) { h.agent.SetShowLlmThinking(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowLlmContent, "show-llm-content", func(v bool) { h.agent.SetShowLlmContent(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowTool, "show-tool", func(v bool) { h.agent.SetShowTool(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowToolInput, "show-tool-input", func(v bool) { h.agent.SetShowToolInput(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowToolOutput, "show-tool-output", func(v bool) { h.agent.SetShowToolOutput(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowCommand, "show-command", func(v bool) { h.agent.SetShowCommand(v) }),
		syncedOnOffParam(&h.cfg.LLM.ShowCommandOutput, "show-command-output", func(v bool) { h.agent.SetShowCommandOutput(v) }),
	}
}

// syncedOnOffParam creates a ConfigParam for an on/off style parameter that
// also propagates the change to the agent immediately via the syncFn callback.
// This ensures .config changes take effect instantly, just like .set.
func syncedOnOffParam(b *bool, name string, syncFn func(bool)) ConfigParam {
	return ConfigParam{
		Name: name, Options: []string{"on", "off"},
		CurrentValue: onOffFunc(b),
		SetValue: func(v string) (string, error) {
			if err := setBoolPtr(b, v); err != nil {
				return "", err
			}
			syncFn(*b)
			return i18n.TF(i18n.KeySettingsUpdated, name, v), nil
		},
		ResetValue: func() string {
			*b = true
			syncFn(*b)
			return i18n.TF(i18n.KeySettingsUpdated, name, "on")
		},
	}
}

// safetyParams returns safety parameters.
func (h *ConfigHandler) safetyParams() []ConfigParam {
	return []ConfigParam{
		{Name: "confirm-tool", Action: func(args []string) {
			h.confirmToolWizard()
		}},
		{Name: "tool-timeout", CurrentValue: func() string {
			n := h.cfg.LLM.ToolTimeout
			if n <= 0 {
				return i18n.T(i18n.KeyUnlimited)
			}
			return strconv.Itoa(n) + "s"
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("请输入非负整数（秒数）")
			}
			h.cfg.LLM.ToolTimeout = n
			return i18n.TF(i18n.KeySettingsUpdated, "tool-timeout", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ToolTimeout = 0
			return i18n.TF(i18n.KeySettingsUpdated, "tool-timeout", i18n.T(i18n.KeyUnlimited))
		}},
		{Name: "cmd-timeout", CurrentValue: func() string {
			n := h.cfg.LLM.CommandTimeout
			if n <= 0 {
				return i18n.T(i18n.KeyUnlimited)
			}
			return strconv.Itoa(n) + "s"
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("请输入非负整数（秒数）")
			}
			h.cfg.LLM.CommandTimeout = n
			return i18n.TF(i18n.KeySettingsUpdated, "cmd-timeout", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.CommandTimeout = 0
			return i18n.TF(i18n.KeySettingsUpdated, "cmd-timeout", i18n.T(i18n.KeyUnlimited))
		}},
		{Name: "llm-timeout", CurrentValue: func() string {
			n := h.cfg.LLM.LLMTimeout
			if n <= 0 {
				return i18n.T(i18n.KeyUnlimited)
			}
			return strconv.Itoa(n) + "s"
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("请输入非负整数（秒数）")
			}
			h.cfg.LLM.LLMTimeout = n
			return i18n.TF(i18n.KeySettingsUpdated, "llm-timeout", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.LLMTimeout = 0
			return i18n.TF(i18n.KeySettingsUpdated, "llm-timeout", i18n.T(i18n.KeyUnlimited))
		}},
		{Name: "error-max-single-count", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.ErrorMaxSingleCount)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.ErrorMaxSingleCount = n
			return i18n.TF(i18n.KeySettingsUpdated, "error-max-single-count", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ErrorMaxSingleCount = 10
			return i18n.TF(i18n.KeySettingsUpdated, "error-max-single-count", "10")
		}},
		{Name: "error-max-type-count", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.ErrorMaxTypeCount)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.ErrorMaxTypeCount = n
			return i18n.TF(i18n.KeySettingsUpdated, "error-max-type-count", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ErrorMaxTypeCount = 100
			return i18n.TF(i18n.KeySettingsUpdated, "error-max-type-count", "100")
		}},
		onOffParam(&h.cfg.LLM.LoopDetectEnabled, "loop-detect-enabled"),
		onOffParam(&h.cfg.LLM.DedupEnabled, "dedup-enabled"),
	}
}

// memoryParams returns memory parameters.
func (h *ConfigHandler) memoryParams() []ConfigParam {
	return []ConfigParam{
		syncedOnOffParam(&h.cfg.LLM.MemoryEnabled, "memory-enabled", func(v bool) { h.agent.SetMemoryEnabled(v) }),
		{Name: "context-limit", CurrentValue: func() string {
			n := h.cfg.LLM.ContextLimit
			if n == 0 {
				return i18n.T(i18n.KeyOff)
			} else if n == -1 {
				return i18n.T(i18n.KeyUnlimited)
			}
			return strconv.Itoa(n)
		}, SetValue: func(v string) (string, error) {
			if v == "off" || v == "0" {
				h.cfg.LLM.ContextLimit = 0
				return i18n.TF(i18n.KeySettingsUpdated, "context-limit", "0"), nil
			} else if v == "unlimited" || v == "-1" {
				h.cfg.LLM.ContextLimit = -1
				return i18n.TF(i18n.KeySettingsUpdated, "context-limit", "-1"), nil
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return "", fmt.Errorf("%s", i18n.T(i18n.KeyConfigValCtxLimit))
			}
			h.cfg.LLM.ContextLimit = n
			return i18n.TF(i18n.KeySettingsUpdated, "context-limit", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ContextLimit = 0
			return i18n.TF(i18n.KeySettingsUpdated, "context-limit", "0")
		}},
		{Name: "context-start", Options: []string{"window", "task", "smart"}, CurrentValue: func() string {
			return h.cfg.LLM.ContextStartMode
		}, SetValue: func(v string) (string, error) {
			switch v {
			case "window", "task", "smart":
				h.cfg.LLM.ContextStartMode = v
			default:
				return "", fmt.Errorf("%s", i18n.T(i18n.KeyConfigValCtxStart))
			}
			return i18n.TF(i18n.KeySettingsUpdated, "context-start", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ContextStartMode = "task"
			return i18n.TF(i18n.KeySettingsUpdated, "context-start", "task")
		}},
		{Name: "memory-search-max-content-len", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.MemorySearchMaxContentLen)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.MemorySearchMaxContentLen = n
			return i18n.TF(i18n.KeySettingsUpdated, "memory-search-max-content-len", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.MemorySearchMaxContentLen = 512
			return i18n.TF(i18n.KeySettingsUpdated, "memory-search-max-content-len", "512")
		}},
		{Name: "memory-search-max-results", CurrentValue: func() string {
			return strconv.Itoa(h.cfg.LLM.MemorySearchMaxResults)
		}, SetValue: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return "", fmt.Errorf("请输入正整数")
			}
			h.cfg.LLM.MemorySearchMaxResults = n
			return i18n.TF(i18n.KeySettingsUpdated, "memory-search-max-results", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.MemorySearchMaxResults = 100
			return i18n.TF(i18n.KeySettingsUpdated, "memory-search-max-results", "100")
		}},
	}
}

// searchParams returns search parameters (moved to agentParams by request).
func (h *ConfigHandler) searchParams() []ConfigParam {
	return []ConfigParam{}
}

// logParam returns the log parameter (used in dev tools).
func (h *ConfigHandler) logParam() ConfigParam {
	return ConfigParam{Name: "log", Options: []string{"debug", "info", "warn", "error", "off"}, CurrentValue: func() string {
		return log.LogLevelString(log.GetLevel())
	}, SetValue: func(v string) (string, error) {
		if level, ok := log.ParseLogLevel(v); ok {
			log.SetLevel(level)
			return i18n.TF(i18n.KeySettingsUpdated, "log", v), nil
		}
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyConfigValDebugOff))
	}, ResetValue: func() string { log.SetLevel(log.LogLevelInfo); return i18n.TF(i18n.KeySettingsUpdated, "log", "info") }}
}

// devToolParams returns developer tool parameters.
func (h *ConfigHandler) devToolParams() []ConfigParam {
	return []ConfigParam{
		h.logParam(),
		cmdEntry(".body-add", "向 LLM 请求体添加自定义 JSON 属性", "key=value", nil, nil),
		cmdEntry(".body-remove", "从 LLM 请求体删除自定义 JSON 属性", "key", nil, nil),
		cmdEntry(".body-display", "显示 LLM 请求体中的自定义 JSON 属性", "", func(args []string) (string, error) {
			if len(h.cfg.LLM.BodyAdditions) == 0 {
				return "  没有自定义属性", nil
			}
			var sb strings.Builder
			for k, v := range h.cfg.LLM.BodyAdditions {
				sb.WriteString(fmt.Sprintf("    %s = %s\n", k, v))
			}
			return sb.String(), nil
		}, nil),
		ConfigParam{
			Name: ".simulate",
			Desc: "模拟 LLM 方法调用，进行解析和执行测试",
			Action: func(args []string) {
				io := agent.GetIO(h.agent)
				io.Println()
				io.Printf("  %s\n", i18n.T(i18n.KeySimulatePromptInput))
				input := h.readLine()
				if input == "" {
					io.Printf("  ❌ %s\n", i18n.T(i18n.KeySimulateNoContent))
					return
				}
				result, err := h.simulateHandler.Handle([]string{input})
				if err != nil {
					io.Printf("  ❌ %v\n", err)
					return
				}
				if result != "" {
					io.Println(result)
				}
			},
		},
	}
}

// modelMgrParams returns model management parameters.
func (h *ConfigHandler) modelMgrParams() []ConfigParam {
	return []ConfigParam{
		cmdEntry(".model", "模型管理（add/list/remove/switch/info）", "", h.modelHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出所有已配置模型", Action: func(a []string) { runHandler(h.modelHandler.Handle, []string{"list"}) }},
			{Name: "add", Desc: "添加新模型（启动设置向导）", Action: func(a []string) { runHandler(h.modelHandler.Handle, []string{"add"}) }},
			{Name: "switch", Desc: "切换当前使用的模型", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"switch"}, a...)) }},
			{Name: "remove", Desc: "删除一个模型配置", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"remove"}, a...)) }},
			{Name: "info", Desc: "查看指定模型的详细配置", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"info"}, a...)) }},
		}),
	}
}

// mcpRuleParams is kept for backwards compatibility but no longer used as a group.
func (h *ConfigHandler) mcpRuleParams() []ConfigParam { return []ConfigParam{} }

// workModeParams returns work mode and section parameters.
func (h *ConfigHandler) workModeParams() []ConfigParam {
	return []ConfigParam{
		cmdEntry(".mode", "管理工作模式", "", h.modeHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出所有工作模式", Action: func(a []string) { runHandler(h.modeHandler.Handle, []string{}) }},
			{Name: "switch", Desc: "切换工作模式", Args: "<name>", Action: func(a []string) { runHandler(h.modeHandler.Handle, append([]string{"switch"}, a...)) }},
			{Name: "create", Desc: "创建工作模式", Args: "<name>", Action: func(a []string) { runHandler(h.modeHandler.Handle, append([]string{"create"}, a...)) }},
			{Name: "edit", Desc: "交互式编辑当前模式", Action: func(a []string) { runHandler(h.modeHandler.Handle, []string{"edit"}) }},
		}),
		cmdEntry(".section", "管理自定义提示词节", "", h.sectionHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出所有自定义节", Action: func(a []string) { runHandler(h.sectionHandler.Handle, []string{}) }},
			{Name: "add", Desc: "添加节", Args: "<name>", Action: func(a []string) { runHandler(h.sectionHandler.Handle, append([]string{"add"}, a...)) }},
			{Name: "remove", Desc: "删除节", Args: "<name>", Action: func(a []string) { runHandler(h.sectionHandler.Handle, append([]string{"remove"}, a...)) }},
		}),
	}
}

// multimodalParams returns multimodal and MCP parameters.
func (h *ConfigHandler) multimodalParams() []ConfigParam {
	return []ConfigParam{
		cmdEntry(".image", "管理多模态图片缓存", "", h.imageHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出缓存的图片", Action: func(a []string) { runHandler(h.imageHandler.Handle, []string{}) }},
			{Name: "add", Desc: "添加图片到缓存", Args: "<path>", Action: func(a []string) { runHandler(h.imageHandler.Handle, append([]string{"add"}, a...)) }},
			{Name: "remove", Desc: "从缓存移除图片", Args: "<index>", Action: func(a []string) { runHandler(h.imageHandler.Handle, append([]string{"remove"}, a...)) }},
			{Name: "clear", Desc: "清空图片缓存", Action: func(a []string) { runHandler(h.imageHandler.Handle, []string{"clear"}) }},
		}),
		cmdEntry(".mcp", "管理 MCP 服务器连接", "", h.mcpHandler.Handle, []ConfigSubCommand{
			{Name: "list", Desc: "列出所有 MCP 服务器", Action: func(a []string) { runHandler(h.mcpHandler.Handle, []string{}) }},
			{Name: "add", Desc: "添加 MCP 服务器", Args: "<name> <command> [args...]", Action: func(a []string) {
				if len(a) < 2 {
					agent.DefaultIO().Println("  用法: .mcp add <name> <command> [args...]")
					return
				}
				runHandler(h.mcpHandler.Handle, append([]string{"add"}, a...))
			}},
			{Name: "remove", Desc: "移除 MCP 服务器", Args: "<name>", Action: func(a []string) { runHandler(h.mcpHandler.Handle, append([]string{"remove"}, a...)) }},
		}),
	}
}

// onOffFunc returns localized on/off string for a bool pointer.
func onOffFunc(b *bool) func() string {
	return func() string {
		if *b {
			return i18n.T(i18n.KeyOn)
		}
		return i18n.T(i18n.KeyOff)
	}
}

// setBoolPtr parses a string and sets a boolean pointer value.
func setBoolPtr(b *bool, v string) error {
	switch strings.ToLower(v) {
	case "on", "1", "true", "yes":
		*b = true
	case "off", "0", "false", "no":
		*b = false
	default:
		return fmt.Errorf("%s", i18n.T(i18n.KeyConfigValOnOff))
	}
	return nil
}

// onOffParam creates a ConfigParam for an on/off style parameter with reset.
func onOffParam(b *bool, name string) ConfigParam {
	return ConfigParam{
		Name: name, Options: []string{"on", "off"},
		CurrentValue: onOffFunc(b),
		SetValue: func(v string) (string, error) {
			if err := setBoolPtr(b, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, name, v), nil
		},
		ResetValue: func() string {
			*b = true
			return i18n.TF(i18n.KeySettingsUpdated, name, "on")
		},
	}
}

// confirmToolWizard provides an interactive wizard for confirm-tool settings.
// Shows all tools with their current modes and global options.
func (h *ConfigHandler) confirmToolWizard() {
	io := h.io()
	for {
		allTools := make([]string, 0, len(agent.DefaultToolModes()))
		for name := range agent.DefaultToolModes() {
			if name != "default" {
				allTools = append(allTools, name)
			}
		}
		sort.Strings(allTools)

		// Build effective modes
		effectiveModes := agent.DefaultToolModes()
		globalDefault := ""
		for k, v := range h.cfg.LLM.ToolModes {
			if k == "default" {
				globalDefault = v
			} else {
				effectiveModes[k] = v
			}
		}

		// Calculate global default string
		globalStr := "custom"
		if globalDefault != "" && globalDefault != "custom" {
			globalStr = globalDefault
		}

		// Fixed 2-digit width for numbering
		numWidth := 2
		// Format: "%s[%2d] ..." where mark is either "  " (2 spaces) or " >" (arrow+space)
		fmtStr := fmt.Sprintf("%%s[%%%dd] %%-35s %%s\n", numWidth)
		globalFmt := fmt.Sprintf("%%s[%%%dd] %%-12s %%s\n", numWidth)

		io.Println("── confirm-tool ──────────────────────────────")
		io.Println()
		io.Println("  工具调用:")
		io.Println()

		// Show all tools with numbers
		toolCount := len(allTools)
		for i, name := range allTools {
			mode := effectiveModes[name]
			if globalDefault == "confirm" || globalDefault == "auto" || globalDefault == "disabled" {
				if _, hasOwn := h.cfg.LLM.ToolModes[name]; !hasOwn {
					mode = globalDefault
				}
			}
			io.Printf(fmtStr, "  ", i+1, name, mode)
		}

		// Global options
		globalStart := toolCount + 1
		globalOptions := []string{"confirm", "auto", "disabled", "custom"}
		modeDesc := map[string]string{
			"confirm":  i18n.T(i18n.KeyModeConfirmDesc),
			"auto":     i18n.T(i18n.KeyModeAutoDesc),
			"disabled": i18n.T(i18n.KeyModeDisabledDesc),
			"custom":   i18n.T(i18n.KeyModeCustomDesc),
		}
		io.Println()
		io.Printf("  全局确认模式: %s          %s\n", globalStr, modeDesc[globalStr])
		io.Println()
		for i, opt := range globalOptions {
			mark := "  "
			if globalStr == opt {
				mark = " >"
			}
			desc := modeDesc[opt]
			io.Printf(globalFmt, mark, globalStart+i, opt, desc)
		}

		io.Println()
		io.Printf("  请选择 [1-%d] [B] 返回上一步 [Q] 完全退出: ", globalStart+3)

		input := h.readLine()
		if strings.EqualFold(input, "b") || strings.EqualFold(input, "B") {
			io.Println()
			return
		}
		if strings.EqualFold(input, "q") || strings.EqualFold(input, "Q") {
			io.Println()
			io.Println(i18n.T(i18n.KeyConfigExited))
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > globalStart+3 {
			io.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			io.Println()
			continue
		}

		if idx >= globalStart {
			// Global option selected
			opt := globalOptions[idx-globalStart]
			switch opt {
			case "confirm", "auto", "disabled":
				if h.cfg.LLM.ToolModes == nil {
					h.cfg.LLM.ToolModes = make(map[string]string)
				}
				h.cfg.LLM.ToolModes["default"] = opt
				h.agent.SetToolMode("", opt)
			case "custom":
				if h.cfg.LLM.ToolModes == nil {
					h.cfg.LLM.ToolModes = make(map[string]string)
				}
				h.cfg.LLM.ToolModes["default"] = "custom"
				h.agent.SyncToolModes(h.cfg)
			}
			if err := h.cfg.Save(); err != nil {
				log.Warn("Failed to save config: %v", err)
			}
			io.Printf("  全局确认模式已设置为: %s\n", opt)
			io.Println()
			continue
		}

		// Specific tool selected - ask for mode
		toolName := allTools[idx-1]
		io.Printf("  设置工具 %s 的确认模式:\n", toolName)
		io.Println()
		modeOptions := []string{"confirm", "auto", "disabled"}
		currentMode := effectiveModes[toolName]
		for i, opt := range modeOptions {
			mark := "  "
			if opt == currentMode {
				mark = "> "
			}
			desc := modeDesc[opt]
			io.Printf("%s[%d] %-12s %s\n", mark, i+1, opt, desc)
		}
		io.Println()
		io.Print("  选择 [1-3], [B] 返回上一步 [Q] 完全退出: ")

		modeInput := h.readLine()
		if strings.EqualFold(modeInput, "b") || strings.EqualFold(modeInput, "B") {
			io.Println()
			continue
		}
		if strings.EqualFold(modeInput, "q") || strings.EqualFold(modeInput, "Q") {
			io.Println()
			io.Println(i18n.T(i18n.KeyConfigExited))
			return
		}

		modeIdx, err := strconv.Atoi(modeInput)
		if err != nil || modeIdx < 1 || modeIdx > 3 {
			io.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			io.Println()
			continue
		}

		mode := modeOptions[modeIdx-1]
		if h.cfg.LLM.ToolModes == nil {
			h.cfg.LLM.ToolModes = make(map[string]string)
		}
		h.cfg.LLM.ToolModes[toolName] = mode
		h.agent.SetToolMode(toolName, mode)
		if err := h.cfg.Save(); err != nil {
			log.Warn("Failed to save config: %v", err)
		}
		io.Printf("  工具 %s 已设置为: %s\n", toolName, mode)
		io.Println()
	}
}

// runWizard runs the interactive configuration wizard.
func (h *ConfigHandler) runWizard() {
	io := h.io()
	groups := h.configGroups()
	io.Println()
	io.Println(i18n.T(i18n.KeyConfigWizardTitle))
	io.Println()
	io.Println(i18n.T(i18n.KeyConfigWizardIntro))
	io.Println()
	h.showGroupMenu(groups)
}

// showGroupMenu displays the top-level group selection menu.
func (h *ConfigHandler) showGroupMenu(groups []ConfigGroup) {
	io := h.io()
	numWidth := len(strconv.Itoa(len(groups)))
	fmtStr := fmt.Sprintf("  [%%%dd] %%s\n", numWidth)
	for {
		io.Println(i18n.T(i18n.KeyConfigGroupTitle))
		for i, g := range groups {
			io.Printf(fmtStr, i+1, g.Name)
		}
		io.Println()
		io.Printf("  选择分组 [1-%d]: [B] 返回上一步 [Q] 完全退出: ", len(groups))

		input := h.readLine()
		if strings.EqualFold(input, "b") || strings.EqualFold(input, "B") {
			io.Println()
			return
		}
		if strings.EqualFold(input, "q") || strings.EqualFold(input, "Q") {
			io.Println()
			io.Println(i18n.T(i18n.KeyConfigExited))
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(groups) {
			io.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			io.Println()
			continue
		}
		io.Println()
		h.showParamMenu(groups[idx-1])
		io.Println()
	}
}

// showParamMenu shows params in a group.
func (h *ConfigHandler) showParamMenu(group ConfigGroup) {
	io := h.io()
	for {
		io.Printf("── %s ────────────────────────────────\n", group.Name)
		io.Println()
		for i, p := range group.Params {
			if len(p.SubCommands) > 0 {
				io.Printf("  [%d] %s...\n", i+1, p.Name)
			} else if p.Desc != "" {
				io.Printf("  [%d] %s  → %s\n", i+1, p.Name, p.Desc)
			} else if p.CurrentValue != nil {
				val := p.CurrentValue()
				io.Printf("  [%d] %s\n", i+1, p.Name)
				io.Printf("         → 当前值: %s\n", val)
			} else {
				io.Printf("  [%d] %s\n", i+1, p.Name)
			}
		}
		io.Println()
		io.Printf("  选择 [1-%d] [B] 返回上一步 [Q] 完全退出: ", len(group.Params))

		input := h.readLine()
		if strings.EqualFold(input, "b") || strings.EqualFold(input, "B") {
			io.Println()
			return
		}
		if strings.EqualFold(input, "q") || strings.EqualFold(input, "Q") {
			io.Println()
			io.Println(i18n.T(i18n.KeyConfigExited))
			io.Println()
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(group.Params) {
			io.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			io.Println()
			continue
		}

		p := group.Params[idx-1]

		if len(p.SubCommands) > 0 {
			h.showSubCommandMenu(p)
			io.Println()
			continue
		}

		if p.Action != nil {
			p.Action(nil)
			io.Println()
			continue
		}

		h.showValueInput(p)
		io.Println()
	}
}

// showSubCommandMenu shows sub-commands as a numbered list.
func (h *ConfigHandler) showSubCommandMenu(param ConfigParam) {
	io := h.io()
	numWidth := len(strconv.Itoa(len(param.SubCommands)))
	cmdFmt := fmt.Sprintf("  [%%%dd] %%s  → %%s\n", numWidth)
	cmdArgsFmt := fmt.Sprintf("  [%%%dd] %%s %%s  → %%s\n", numWidth)
	for {
		io.Printf("── %s ────────────────────────────────\n", param.Name)
		io.Println()
		for i, sc := range param.SubCommands {
			if sc.Args != "" {
				io.Printf(cmdArgsFmt, i+1, sc.Name, sc.Args, sc.Desc)
			} else {
				io.Printf(cmdFmt, i+1, sc.Name, sc.Desc)
			}
		}
		io.Println()
		io.Printf("  选择子命令 [1-%d] [B] 返回上一步 [Q] 完全退出: ", len(param.SubCommands))

		input := h.readLine()
		if strings.EqualFold(input, "b") || strings.EqualFold(input, "B") {
			io.Println()
			return
		}
		if strings.EqualFold(input, "q") || strings.EqualFold(input, "Q") {
			io.Println()
			io.Println(i18n.T(i18n.KeyConfigExited))
			io.Println()
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(param.SubCommands) {
			io.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			io.Println()
			continue
		}

		sc := param.SubCommands[idx-1]
		io.Println()

		// If sub-command needs args and has Args field, prompt for input
		if sc.Args != "" {
			io.Printf("  用法: %s %s\n", sc.Name, sc.Args)
			if !strings.HasPrefix(sc.Args, "[") {
				// Required args: prompt for input
				io.Print("  请输入参数: ")
				argInput := h.readLine()
				if argInput != "" {
					sc.Action(strings.Fields(argInput))
				}
				io.Println()
				continue
			}
		}
		// No args needed or optional args: just run
		sc.Action(nil)
		io.Println()
	}
}

// showValueInput prompts the user to enter a value for a parameter.
func (h *ConfigHandler) showValueInput(param ConfigParam) {
	io := h.io()
	numWidth := 2
	if len(param.Options) > 0 {
		numWidth = len(strconv.Itoa(len(param.Options)))
	}
	optFmt := fmt.Sprintf("  %%s[%%%dd] %%s\n", numWidth)
	for {
		current := param.CurrentValue()
		io.Printf(i18n.T(i18n.KeyConfigValueLabParam)+"\n", param.Name)
		io.Printf(i18n.T(i18n.KeyConfigValueLabCurrent)+"\n", current)

		if len(param.Options) > 0 {
			io.Println()
			for i, opt := range param.Options {
				mark := "  "
				if opt == current {
					mark = "> "
				}
				io.Printf(optFmt, mark, i+1, opt)
			}
			io.Println()
			io.Print("  输入编号或直接输入值（[D] 默认值，[B] 返回上一步 [Q] 完全退出）: ")
		} else {
			io.Print("  输入新值（[D] 默认值，[B] 返回上一步 [Q] 完全退出）: ")
		}

		input := h.readLine()
		if input == "" {
			io.Println(i18n.T(i18n.KeyConfigValueUnchanged))
			return
		}
		if strings.EqualFold(input, "b") || strings.EqualFold(input, "B") {
			return
		}
		if strings.EqualFold(input, "q") || strings.EqualFold(input, "Q") {
			io.Println(i18n.T(i18n.KeyConfigExited))
			return
		}
		if strings.EqualFold(input, "D") && param.ResetValue != nil {
			msg := param.ResetValue()
			if saveErr := h.cfg.Save(); saveErr != nil {
				log.Warn("Failed to save config: %v", saveErr)
			}
			io.Printf("  ✅ %s\n", msg)
			return
		}

		if len(param.Options) > 0 {
			if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(param.Options) {
				input = param.Options[idx-1]
			}
		}

		msg, err := param.SetValue(input)
		if err != nil {
			io.Printf("  ❌ %v\n", err)
			continue
		}
		if saveErr := h.cfg.Save(); saveErr != nil {
			log.Warn("Failed to save config: %v", saveErr)
		}
		io.Printf("  ✅ %s\n", msg)
		return
	}
}
