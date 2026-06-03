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
	"os"
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
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cfg *config.Config, ag *agent.Agent) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, agent: ag}
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

// readLine reads a line from stdin.
func (h *ConfigHandler) readLine() string {
	if h.scanner != nil {
		if h.scanner.Scan() {
			return strings.TrimSpace(h.scanner.Text())
		}
		return ""
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
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
				cmdEntry(".memory", "管理记忆和持久知识", "", h.memoryHandler.Handle, []ConfigSubCommand{
					{Name: "save", Desc: "保存一条记忆", Args: "<key> <value>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"save"}, a...)) }},
					{Name: "get", Desc: "获取一条记忆", Args: "<key>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"get"}, a...)) }},
					{Name: "search", Desc: "按前缀搜索记忆", Args: "<prefix>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"search"}, a...)) }},
					{Name: "delete", Desc: "删除一条记忆", Args: "<key>", Action: func(a []string) { runHandler(h.memoryHandler.Handle, append([]string{"delete"}, a...)) }},
					{Name: "list", Desc: "列出所有记忆", Action: func(a []string) { runHandler(h.memoryHandler.Handle, []string{"list"}) }},
					{Name: "clear", Desc: "清空所有记忆", Action: func(a []string) { runHandler(h.memoryHandler.Handle, []string{"clear"}) }},
				}),
				cmdEntry(".context", "管理对话上下文", "", h.contextHandler.Handle, []ConfigSubCommand{
					{Name: "show", Desc: "显示当前上下文", Action: func(a []string) { runHandler(h.contextHandler.Handle, []string{"show"}) }},
					{Name: "reset", Desc: "重置上下文", Action: func(a []string) { runHandler(h.contextHandler.Handle, []string{"reset"}) }},
				}),
				cmdEntry(".session", "查看当前会话信息", "", h.sessionHandler.Handle, []ConfigSubCommand{
					{Name: "info", Desc: "显示会话概要", Action: func(a []string) { runHandler(h.sessionHandler.Handle, []string{}) }},
				}),
				cmdEntry(".new", "清空当前会话，开始全新对话", "", func(args []string) (string, error) {
					h.agent.Reset()
					return "", nil
				}, nil),
			),
		},
		{
			Name: i18n.T(i18n.KeySettingsGroupSearchDebug),
			Params: append(h.searchParams(),
				cmdEntry(".history", "查看用户输入命令历史", "", h.listHandler.HandleHistory, []ConfigSubCommand{
					{Name: "last", Desc: "查看最近 N 条历史", Args: "[N]", Action: func(a []string) { runHandler(h.listHandler.HandleHistory, append([]string{"last"}, a...)) }},
					{Name: "first", Desc: "查看最早 N 条历史", Args: "[N]", Action: func(a []string) { runHandler(h.listHandler.HandleHistory, append([]string{"first"}, a...)) }},
				}),
			),
		},
		{
			Name: "[ 模型管理 ]",
			Params: []ConfigParam{
				cmdEntry(".model", "模型管理（add/list/remove/switch/info）", "", h.modelHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "列出所有已配置模型", Action: func(a []string) { runHandler(h.modelHandler.Handle, []string{"list"}) }},
					{Name: "add", Desc: "添加新模型（启动设置向导）", Action: func(a []string) { runHandler(h.modelHandler.Handle, []string{"add"}) }},
					{Name: "switch", Desc: "切换当前使用的模型", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"switch"}, a...)) }},
					{Name: "remove", Desc: "删除一个模型配置", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"remove"}, a...)) }},
					{Name: "info", Desc: "查看指定模型的详细配置", Args: "[id]", Action: func(a []string) { runHandler(h.modelHandler.Handle, append([]string{"info"}, a...)) }},
				}),
			},
		},
		{
			Name: "[ MCP 与规则 ]",
			Params: []ConfigParam{
				cmdEntry(".mcp", "管理 MCP 服务器连接", "", h.mcpHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "列出所有 MCP 服务器", Action: func(a []string) { runHandler(h.mcpHandler.Handle, []string{}) }},
					{Name: "add", Desc: "添加 MCP 服务器", Args: "<name> <command> [args...]", Action: func(a []string) {
						if len(a) < 2 {
							fmt.Println("  用法: .mcp add <name> <command> [args...]")
							return
						}
						runHandler(h.mcpHandler.Handle, append([]string{"add"}, a...))
					}},
					{Name: "remove", Desc: "移除 MCP 服务器", Args: "<name>", Action: func(a []string) { runHandler(h.mcpHandler.Handle, append([]string{"remove"}, a...)) }},
				}),
				cmdEntry(".rule", "管理 AI 全局规则", "", h.ruleHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "列出所有规则", Action: func(a []string) { runHandler(h.ruleHandler.Handle, []string{}) }},
					{Name: "add", Desc: "添加新规则", Args: "<rule>", Action: func(a []string) { runHandler(h.ruleHandler.Handle, append([]string{"add"}, a...)) }},
					{Name: "remove", Desc: "删除规则", Args: "<index>", Action: func(a []string) { runHandler(h.ruleHandler.Handle, append([]string{"remove"}, a...)) }},
				}),
			},
		},
		{
			Name: "[ 工作模式与节管理 ]",
			Params: []ConfigParam{
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
			},
		},
		{
			Name: "[ 图片与任务计划 ]",
			Params: []ConfigParam{
				cmdEntry(".image", "管理多模态图片缓存", "", h.imageHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "列出缓存的图片", Action: func(a []string) { runHandler(h.imageHandler.Handle, []string{}) }},
					{Name: "add", Desc: "添加图片到缓存", Args: "<path>", Action: func(a []string) { runHandler(h.imageHandler.Handle, append([]string{"add"}, a...)) }},
					{Name: "remove", Desc: "从缓存移除图片", Args: "<index>", Action: func(a []string) { runHandler(h.imageHandler.Handle, append([]string{"remove"}, a...)) }},
					{Name: "clear", Desc: "清空图片缓存", Action: func(a []string) { runHandler(h.imageHandler.Handle, []string{"clear"}) }},
				}),
				cmdEntry(".plan", "管理任务计划", "", h.planHandler.Handle, []ConfigSubCommand{
					{Name: "list", Desc: "查看当前任务计划", Action: func(a []string) { runHandler(h.planHandler.Handle, []string{}) }},
					{Name: "create", Desc: "创建新任务计划", Args: "<title>", Action: func(a []string) { runHandler(h.planHandler.Handle, append([]string{"create"}, a...)) }},
				}),
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
			},
		},
		{
			Name: "[ 数据库 ]",
			Params: []ConfigParam{
				cmdEntry(".db", "查看/配置 PostgreSQL 数据库连接", "", h.settingsHandler.HandleDB, []ConfigSubCommand{
					{Name: "info", Desc: "查看数据库配置", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, []string{}) }},
					{Name: "host", Desc: "设置数据库地址", Args: "<host>", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, append([]string{"host"}, a...)) }},
					{Name: "port", Desc: "设置数据库端口", Args: "<port>", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, append([]string{"port"}, a...)) }},
					{Name: "migrate", Desc: "从本地 bbolt 迁移到 PostgreSQL", Action: func(a []string) { runHandler(h.settingsHandler.HandleDB, []string{"migrate"}) }},
				}),
			},
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
			if handler != nil {
				result, err := handler(args)
				if err != nil {
					fmt.Printf("  ❌ %v\n", err)
					return
				}
				if result != "" {
					fmt.Println(result)
				}
			} else if usage != "" {
				fmt.Printf("  用法: %s %s\n", name, usage)
			}
		}
	}
	return p
}

// runHandler runs a handler function and displays result/error.
func runHandler(handler func([]string) (string, error), args []string) {
	result, err := handler(args)
	if err != nil {
		fmt.Printf("  ❌ %v\n", err)
		return
	}
	if result != "" {
		fmt.Println(result)
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
		{Name: "toolcall-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.ToolCallEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.ToolCallEnabled, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ToolCallEnabled = true
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-enabled", "on")
		}},
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
			default:
				return "", fmt.Errorf("请输入 openai 或 xml")
			}
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-mode", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ToolCallMode = "openai"
			return i18n.TF(i18n.KeySettingsUpdated, "toolcall-mode", "openai")
		}},
		{Name: "plan-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.PlanEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.PlanEnabled, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "plan-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.PlanEnabled = true
			return i18n.TF(i18n.KeySettingsUpdated, "plan-enabled", "on")
		}},
		{Name: "subagent-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.SubAgentEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.SubAgentEnabled, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "subagent-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.SubAgentEnabled = true
			return i18n.TF(i18n.KeySettingsUpdated, "subagent-enabled", "on")
		}},
		{Name: "result-mode", Options: []string{"minimal", "explain", "analyze", "free"}, CurrentValue: func() string {
			return config.ResultModeString(config.ResultMode(h.cfg.LLM.ResultMode))
		}, SetValue: func(v string) (string, error) {
			if mode, ok := config.ParseResultMode(v); ok {
				h.cfg.LLM.ResultMode = int(mode)
				return i18n.TF(i18n.KeySettingsUpdated, "result-mode", v), nil
			}
			return "", fmt.Errorf(i18n.T(i18n.KeyConfigValMinExplAnFree))
		}, ResetValue: func() string {
			h.cfg.LLM.ResultMode = int(config.ResultModeMinimal)
			return i18n.TF(i18n.KeySettingsUpdated, "result-mode", "minimal")
		}},
		{Name: "shell-session-enabled", Options: []string{"on", "off"}, CurrentValue: onOffFunc(&h.cfg.LLM.ShellSessionEnabled), SetValue: func(v string) (string, error) {
			if err := setBoolPtr(&h.cfg.LLM.ShellSessionEnabled, v); err != nil {
				return "", err
			}
			return i18n.TF(i18n.KeySettingsUpdated, "shell-session-enabled", v), nil
		}, ResetValue: func() string {
			h.cfg.LLM.ShellSessionEnabled = false
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
	}
}

// displayParams returns display parameters.
func (h *ConfigHandler) displayParams() []ConfigParam {
	return []ConfigParam{
		onOffParam(&h.cfg.LLM.EmojiEnabled, "emoji-enabled"),
		onOffParam(&h.cfg.LLM.ShowLlmThinking, "show-llm-thinking"),
		onOffParam(&h.cfg.LLM.ShowLlmContent, "show-llm-content"),
		onOffParam(&h.cfg.LLM.ShowTool, "show-tool"),
		onOffParam(&h.cfg.LLM.ShowToolInput, "show-tool-input"),
		onOffParam(&h.cfg.LLM.ShowToolOutput, "show-tool-output"),
		onOffParam(&h.cfg.LLM.ShowCommand, "show-command"),
		onOffParam(&h.cfg.LLM.ShowCommandOutput, "show-command-output"),
	}
}

// safetyParams returns safety parameters.
func (h *ConfigHandler) safetyParams() []ConfigParam {
	return []ConfigParam{
		{Name: "confirm-tool", Options: []string{"confirm", "auto"}, CurrentValue: func() string {
			if v, ok := h.cfg.LLM.ToolModes["default"]; ok {
				return v
			}
			return "confirm"
		}, SetValue: func(v string) (string, error) {
			switch strings.ToLower(v) {
			case "on", "confirm":
				if h.cfg.LLM.ToolModes == nil {
					h.cfg.LLM.ToolModes = make(map[string]string)
				}
				h.cfg.LLM.ToolModes["default"] = "confirm"
			case "off", "auto":
				if h.cfg.LLM.ToolModes == nil {
					h.cfg.LLM.ToolModes = make(map[string]string)
				}
				h.cfg.LLM.ToolModes["default"] = "auto"
			default:
				return "", fmt.Errorf("请输入 on/confirm 或 off/auto")
			}
			h.agent.SyncToolModes(h.cfg)
			return i18n.TF(i18n.KeySettingsUpdated, "confirm-tool", v), nil
		}, ResetValue: func() string {
			if h.cfg.LLM.ToolModes == nil {
				h.cfg.LLM.ToolModes = make(map[string]string)
			}
			h.cfg.LLM.ToolModes["default"] = "confirm"
			h.agent.SyncToolModes(h.cfg)
			return i18n.TF(i18n.KeySettingsUpdated, "confirm-tool", "confirm")
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
		onOffParam(&h.cfg.LLM.MemoryEnabled, "memory-enabled"),
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
				return "", fmt.Errorf(i18n.T(i18n.KeyConfigValCtxLimit))
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
				return "", fmt.Errorf(i18n.T(i18n.KeyConfigValCtxStart))
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

// searchParams returns search parameters.
func (h *ConfigHandler) searchParams() []ConfigParam {
	return []ConfigParam{
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
		{Name: "log", Options: []string{"debug", "info", "warn", "error", "off"}, CurrentValue: func() string {
			return log.LogLevelString(log.GetLevel())
		}, SetValue: func(v string) (string, error) {
			if level, ok := log.ParseLogLevel(v); ok {
				log.SetLevel(level)
				return i18n.TF(i18n.KeySettingsUpdated, "log", v), nil
			}
			return "", fmt.Errorf(i18n.T(i18n.KeyConfigValDebugOff))
		}, ResetValue: func() string { log.SetLevel(log.LogLevelInfo); return i18n.TF(i18n.KeySettingsUpdated, "log", "info") }},
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
		return fmt.Errorf(i18n.T(i18n.KeyConfigValOnOff))
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

// runWizard runs the interactive configuration wizard.
func (h *ConfigHandler) runWizard() {
	groups := h.configGroups()
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyConfigWizardTitle))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyConfigWizardIntro))
	fmt.Println()
	h.showGroupMenu(groups)
}

// showGroupMenu displays the top-level group selection menu.
func (h *ConfigHandler) showGroupMenu(groups []ConfigGroup) {
	for {
		fmt.Println(i18n.T(i18n.KeyConfigGroupTitle))
		for i, g := range groups {
			fmt.Printf("  [%d] %s\n", i+1, g.Name)
		}
		fmt.Println()
		fmt.Printf(i18n.T(i18n.KeyConfigGroupPrompt), len(groups))

		input := h.readLine()
		if strings.EqualFold(input, "P") || strings.EqualFold(input, "Q") {
			fmt.Println()
			fmt.Println(i18n.T(i18n.KeyConfigExited))
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(groups) {
			fmt.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			fmt.Println()
			continue
		}
		fmt.Println()
		h.showParamMenu(groups[idx-1])
		fmt.Println()
	}
}

// showParamMenu shows params in a group. If a command has SubCommands, shows them as numbered list.
func (h *ConfigHandler) showParamMenu(group ConfigGroup) {
	for {
		fmt.Printf("── %s ────────────────────────────────\n", group.Name)
		fmt.Println()
		for i, p := range group.Params {
			if p.CurrentValue != nil {
				fmt.Printf("  [%d] %s = %s\n", i+1, p.Name, p.CurrentValue())
			} else if p.Desc != "" {
				fmt.Printf("  [%d] %s  → %s\n", i+1, p.Name, p.Desc)
			} else {
				fmt.Printf("  [%d] %s\n", i+1, p.Name)
			}
		}
		fmt.Println()
		fmt.Printf(i18n.T(i18n.KeyConfigParamPrompt), len(group.Params))

		input := h.readLine()
		if strings.EqualFold(input, "P") {
			fmt.Println()
			return
		}
		if strings.EqualFold(input, "Q") {
			fmt.Println()
			fmt.Println(i18n.T(i18n.KeyConfigExited))
			fmt.Println()
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(group.Params) {
			fmt.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			fmt.Println()
			continue
		}

		p := group.Params[idx-1]

		// If it has sub-commands, show sub-command menu
		if len(p.SubCommands) > 0 {
			h.showSubCommandMenu(p)
			fmt.Println()
			continue
		}

		// If it's an action entry without sub-commands, run it
		if p.Action != nil {
			p.Action(nil)
			fmt.Println()
			continue
		}

		// Otherwise, show value input
		h.showValueInput(p)
		fmt.Println()
	}
}

// showSubCommandMenu shows sub-commands as a numbered list.
func (h *ConfigHandler) showSubCommandMenu(param ConfigParam) {
	for {
		fmt.Printf("── %s ────────────────────────────────\n", param.Name)
		fmt.Println()
		for i, sc := range param.SubCommands {
			if sc.Args != "" {
				fmt.Printf("  [%d] %s %s  → %s\n", i+1, sc.Name, sc.Args, sc.Desc)
			} else {
				fmt.Printf("  [%d] %s  → %s\n", i+1, sc.Name, sc.Desc)
			}
		}
		fmt.Println()
		fmt.Print("  选择子命令 [1-" + strconv.Itoa(len(param.SubCommands)) + "], [P] 返回, [Q] 退出: ")

		input := h.readLine()
		if strings.EqualFold(input, "P") {
			fmt.Println()
			return
		}
		if strings.EqualFold(input, "Q") {
			fmt.Println()
			fmt.Println(i18n.T(i18n.KeyConfigExited))
			fmt.Println()
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(param.SubCommands) {
			fmt.Println(i18n.T(i18n.KeyConfigInvalidChoice))
			fmt.Println()
			continue
		}

		sc := param.SubCommands[idx-1]
		fmt.Println()

		// If sub-command needs args and has Args field, prompt for input
		if sc.Args != "" {
			fmt.Printf("  用法: %s %s\n", sc.Name, sc.Args)
			if !strings.HasPrefix(sc.Args, "[") {
				// Required args: prompt for input
				fmt.Print("  请输入参数: ")
				argInput := h.readLine()
				if argInput != "" {
					sc.Action(strings.Fields(argInput))
				}
				fmt.Println()
				continue
			}
		}
		// No args needed or optional args: just run
		sc.Action(nil)
		fmt.Println()
	}
}

// showValueInput prompts the user to enter a value for a parameter.
func (h *ConfigHandler) showValueInput(param ConfigParam) {
	for {
		current := param.CurrentValue()
		fmt.Printf(i18n.T(i18n.KeyConfigValueLabParam)+"\n", param.Name)
		fmt.Printf(i18n.T(i18n.KeyConfigValueLabCurrent)+"\n", current)

		if len(param.Options) > 0 {
			fmt.Println()
			for i, opt := range param.Options {
				mark := "  "
				if opt == current {
					mark = "> "
				}
				fmt.Printf("  %s[%d] %s\n", mark, i+1, opt)
			}
			fmt.Println()
			fmt.Print("  输入编号或直接输入值（[D] 默认值，[P] 返回，[Q] 退出）: ")
		} else {
			fmt.Print("  输入新值（[D] 默认值，[P] 返回，[Q] 退出）: ")
		}

		input := h.readLine()
		if input == "" {
			fmt.Println(i18n.T(i18n.KeyConfigValueUnchanged))
			return
		}
		if strings.EqualFold(input, "P") {
			return
		}
		if strings.EqualFold(input, "Q") {
			fmt.Println(i18n.T(i18n.KeyConfigExited))
			return
		}
		if strings.EqualFold(input, "D") && param.ResetValue != nil {
			msg := param.ResetValue()
			if saveErr := h.cfg.Save(); saveErr != nil {
				log.Warn("Failed to save config: %v", saveErr)
			}
			fmt.Printf("  ✅ %s\n", msg)
			return
		}

		if len(param.Options) > 0 {
			if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(param.Options) {
				input = param.Options[idx-1]
			}
		}

		msg, err := param.SetValue(input)
		if err != nil {
			fmt.Printf("  ❌ %v\n", err)
			continue
		}
		if saveErr := h.cfg.Save(); saveErr != nil {
			log.Warn("Failed to save config: %v", saveErr)
		}
		fmt.Printf("  ✅ %s\n", msg)
		return
	}
}
