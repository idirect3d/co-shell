// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/wizard"
	"github.com/idirect3d/co-shell/workspace"
)

const version = "0.4.0"
const build = "148"

// cliFlags holds parsed command-line flags.
type cliFlags struct {
	workspacePath string
	configPath    string
	model         string
	endpoint      string
	apiKey        string
	log           string
	command       string
	maxIterations int
	showHelp      bool
	showVersion   bool
	lang          string
	agentName     string
	imagePaths    string // comma-separated image file paths for multimodal input

	// LLM behavior parameters
	temperature     float64
	maxTokens       int
	showLlmThinking string // "on"/"off"

	showLlmContent    string // "on"/"off"
	showTool          string // "on"/"off"
	showToolInput     string // "on"/"off"
	showToolOutput    string // "on"/"off"
	showCommand       string // "on"/"off"
	showCommandOutput string // "on"/"off"
	confirmCommand    string // "on"/"off"
	resultMode        string // minimal/explain/analyze/free

	// Agent identity parameters
	description string
	principles  string

	// Vision support
	vision string // "on"/"off"

	// Memory enabled
	memoryEnabled string // "on"/"off"

	// Plan enabled
	planEnabled string // "on"/"off"

	// SubAgent enabled
	subAgentEnabled string // "on"/"off"

	// Timeout parameters
	toolTimeout int
	cmdTimeout  int
	llmTimeout  int

	// Output mode
	outputMode string // compact/normal/debug

	// Memory search config
	memorySearchMaxContentLen int
	memorySearchMaxResults    int

	// Error tracking config
	errorMaxSingleCount int
	errorMaxTypeCount   int

	// Log level
	logLevel string // debug/info/warn/error/off

	// Emoji enabled
	emojiEnabled string // "on"/"off"

	// External config file generation
	initCapabilities bool
	initRules        bool
}

func parseFlags() cliFlags {
	var f cliFlags

	// Define flags
	flag.StringVar(&f.workspacePath, "workspace", "", "指定工作区路径（默认：当前目录）")
	flag.StringVar(&f.workspacePath, "w", "", "指定工作区路径（简写）")
	flag.StringVar(&f.configPath, "config", "", "指定配置文件路径（默认：{workspace}/config.json）")
	flag.StringVar(&f.configPath, "c", "", "指定配置文件路径（简写）")
	flag.StringVar(&f.model, "model", "", "临时指定模型名称（覆盖配置文件）")
	flag.StringVar(&f.model, "m", "", "临时指定模型名称（简写）")
	flag.StringVar(&f.endpoint, "endpoint", "", "临时指定 API 端点（覆盖配置文件）")
	flag.StringVar(&f.endpoint, "e", "", "临时指定 API 端点（简写）")
	flag.StringVar(&f.apiKey, "api-key", "", "临时指定 API Key（覆盖配置文件）")
	flag.StringVar(&f.apiKey, "k", "", "临时指定 API Key（简写）")
	flag.StringVar(&f.log, "log", "", "临时指定日志开关（on/off，覆盖配置文件）")
	flag.IntVar(&f.maxIterations, "max-iterations", -1, "最大迭代次数（-1 为不限制，默认 1000）")
	flag.StringVar(&f.agentName, "name", "", "指定 agent 名称（默认：co-shell，用于标识日志、sub-agent workspace 等）")
	flag.StringVar(&f.agentName, "n", "", "指定 agent 名称（简写）")
	flag.StringVar(&f.lang, "lang", "", "设置语言（zh/en，默认自动检测）")
	flag.StringVar(&f.imagePaths, "image", "", "图片文件路径（多张图片用逗号分隔），用于多模态输入")
	flag.StringVar(&f.imagePaths, "i", "", "图片文件路径（简写）")
	flag.BoolVar(&f.showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&f.showHelp, "h", false, "显示帮助信息（简写）")
	flag.BoolVar(&f.showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&f.showVersion, "v", false, "显示版本信息（简写）")

	// LLM behavior parameters
	flag.Float64Var(&f.temperature, "temperature", -1, "温度参数（0.0 ~ 2.0，覆盖配置文件）")
	flag.IntVar(&f.maxTokens, "max-tokens", -1, "最大输出令牌数（覆盖配置文件）")
	flag.StringVar(&f.showLlmThinking, "show-llm-thinking", "", "显示 LLM 返回的思考内容（on/off，覆盖配置文件）")

	flag.StringVar(&f.showCommand, "show-command", "", "显示执行的系统命令（on/off，覆盖配置文件）")
	flag.StringVar(&f.showLlmContent, "show-llm-content", "", "显示 LLM 返回的主要内容（on/off，覆盖配置文件）")
	flag.StringVar(&f.showTool, "show-tool", "", "显示工具调用名称（on/off，覆盖配置文件）")
	flag.StringVar(&f.showToolInput, "show-tool-input", "", "显示工具调用输入参数（on/off，覆盖配置文件）")
	flag.StringVar(&f.showToolOutput, "show-tool-output", "", "显示工具调用返回数据（on/off，覆盖配置文件）")
	flag.StringVar(&f.showCommandOutput, "show-command-output", "", "显示命令返回数据（on/off，覆盖配置文件）")

	flag.StringVar(&f.confirmCommand, "confirm-command", "", "执行命令前需确认（on/off，覆盖配置文件）")
	flag.StringVar(&f.resultMode, "result-mode", "", "结果处理模式（minimal/explain/analyze/free，覆盖配置文件）")

	// Agent identity parameters
	flag.StringVar(&f.description, "description", "", "指定 agent 描述/专长（覆盖配置文件）")
	flag.StringVar(&f.principles, "principles", "", "指定 agent 核心原则（覆盖配置文件）")

	// Vision support
	flag.StringVar(&f.vision, "vision", "", "视觉识别能力（on/off，覆盖配置文件）")

	// Memory enabled
	flag.StringVar(&f.memoryEnabled, "memory-enabled", "", "启用持久化记忆功能（覆盖配置文件）")
	flag.StringVar(&f.memoryEnabled, "memory-disabled", "", "禁用持久化记忆功能（覆盖配置文件）")

	// Plan enabled
	flag.StringVar(&f.planEnabled, "plan-enabled", "", "启用任务计划功能（覆盖配置文件）")
	flag.StringVar(&f.planEnabled, "plan-disabled", "", "禁用任务计划功能（覆盖配置文件）")

	// SubAgent enabled
	flag.StringVar(&f.subAgentEnabled, "subagent-enabled", "", "启用子代理功能（覆盖配置文件）")
	flag.StringVar(&f.subAgentEnabled, "subagent-disabled", "", "禁用子代理功能（覆盖配置文件）")

	// Timeout parameters
	flag.IntVar(&f.toolTimeout, "tool-timeout", -1, "工具调用超时秒数（0=不限，覆盖配置文件）")
	flag.IntVar(&f.cmdTimeout, "cmd-timeout", -1, "系统命令执行超时秒数（0=不限，覆盖配置文件）")
	flag.IntVar(&f.llmTimeout, "llm-timeout", -1, "LLM API 请求超时秒数（0=不限，覆盖配置文件）")

	// Output mode
	flag.StringVar(&f.outputMode, "output-mode", "", "LLM 前端输出模式（compact/normal/debug，覆盖配置文件）")

	// Memory search config
	flag.IntVar(&f.memorySearchMaxContentLen, "memory-search-max-content-len", -1, "记忆搜索内容最大字符长度（默认 512，覆盖配置文件）")
	flag.IntVar(&f.memorySearchMaxResults, "memory-search-max-results", -1, "记忆搜索最大结果数（默认 100，覆盖配置文件）")

	// Error tracking config
	flag.IntVar(&f.errorMaxSingleCount, "error-max-single-count", -1, "相同错误最大出现次数（默认 10，覆盖配置文件）")
	flag.IntVar(&f.errorMaxTypeCount, "error-max-type-count", -1, "不同错误类型最大数量（默认 100，覆盖配置文件）")

	// Log level
	flag.StringVar(&f.logLevel, "log-level", "", "日志输出级别（debug/info/warn/error/off，覆盖配置文件）")

	// Emoji enabled
	flag.StringVar(&f.emojiEnabled, "emoji-enabled", "", "启用表情符号前缀（on/off，覆盖配置文件）")

	// External config file generation
	flag.BoolVar(&f.initCapabilities, "init-capabilities", false, "在工作区生成默认 CAPABILITIES.md 文件并退出")
	flag.BoolVar(&f.initRules, "init-rules", false, "在工作区生成默认 RULES.md 文件并退出")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, buildUsage(version))
	}

	flag.Parse()

	// If there are non-flag arguments and no explicit -c/--cmd, treat them as the command
	if f.command == "" && flag.NArg() > 0 {
		f.command = strings.Join(flag.Args(), " ")
	}

	return f
}

func main() {
	flags := parseFlags()

	// Initialize i18n before any user-facing output
	i18n.Init(flags.lang)

	// Handle --help
	if flags.showHelp {
		flag.Usage()
		os.Exit(0)
	}

	// Handle --version
	if flags.showVersion {
		// Try to load config to check vision support (without creating workspace dirs)
		visionIndicator := ""
		configPath := flags.configPath
		if configPath == "" {
			root := flags.workspacePath
			if root == "" {
				wd, err := os.Getwd()
				if err == nil {
					root = wd
				}
			}
			if root != "" {
				if absRoot, err := filepath.Abs(root); err == nil {
					configPath = filepath.Join(absRoot, "config.json")
				}
			}
		}
		if configPath != "" {
			cfg, _, err := config.LoadFromFile(configPath, nil)
			if err == nil && cfg.LLM.VisionSupport {
				visionIndicator = " 👀"
			}
		}
		fmt.Printf("co-shell v%s [BUILD-%s]%s\n", version, build, visionIndicator)
		os.Exit(0)
	}

	// Initialize workspace
	ws, err := workspace.New(flags.workspacePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot initialize workspace: %v\n", err)
		os.Exit(1)
	}

	// Handle --init-capabilities: generate default CAPABILITIES.md in workspace root
	if flags.initCapabilities {
		ep := config.GetEmojiPrefixes(true) // default to enabled for CLI output
		capPath := filepath.Join(ws.Root(), "CAPABILITIES.md")
		if _, err := os.Stat(capPath); err == nil {
			fmt.Printf("%s %s 已存在，跳过生成。\n", ep.Warning, capPath)
			os.Exit(0)
		}
		content := i18n.T(i18n.KeySystemPromptCapabilities)
		if err := os.WriteFile(capPath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", capPath, err)
			os.Exit(1)
		}
		fmt.Printf("%s 已生成默认 CAPABILITIES.md: %s\n", ep.Success, capPath)
		os.Exit(0)
	}

	// Handle --init-rules: generate default RULES.md in workspace root
	if flags.initRules {
		ep := config.GetEmojiPrefixes(true) // default to enabled for CLI output
		rulesPath := filepath.Join(ws.Root(), "RULES.md")
		if _, err := os.Stat(rulesPath); err == nil {
			fmt.Printf("%s %s 已存在，跳过生成。\n", ep.Warning, rulesPath)
			os.Exit(0)
		}
		content := i18n.T(i18n.KeySystemPromptRules)
		if err := os.WriteFile(rulesPath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", rulesPath, err)
			os.Exit(1)
		}
		fmt.Printf("%s 已生成默认 RULES.md: %s\n", ep.Success, rulesPath)
		os.Exit(0)
	}

	// Load configuration with priority:
	// 1. -c/--config <path> (highest priority)
	// 2. CO_SHELL_CONFIG_PATH environment variable (inherited from parent agent)
	// 3. {workspace}/config.json (default)
	var cfg *config.Config
	var configPath string
	if flags.configPath != "" {
		cfg, configPath, err = config.LoadFromFile(flags.configPath, ws)
	} else if envConfigPath := os.Getenv("CO_SHELL_CONFIG_PATH"); envConfigPath != "" {
		cfg, configPath, err = config.LoadFromFile(envConfigPath, ws)
	} else {
		cfg, configPath, err = config.LoadWithPath(ws)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot load config: %v\n", err)
		cfg = config.DefaultConfig()
	}
	if configPath != "" {
		log.Info("Config loaded from: %s", configPath)
		// Set environment variable so sub-agent processes inherit the config path
		os.Setenv("CO_SHELL_CONFIG_PATH", configPath)
	}

	// Apply CLI overrides
	if flags.model != "" {
		cfg.LLM.Model = flags.model
	}
	if flags.endpoint != "" {
		cfg.LLM.Endpoint = flags.endpoint
	}
	if flags.apiKey != "" {
		cfg.LLM.APIKey = flags.apiKey
	}
	if flags.log != "" {
		switch flags.log {
		case "on", "1", "true", "yes":
			cfg.LogEnabled = true
		case "off", "0", "false", "no":
			cfg.LogEnabled = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --log value %q, use on|off\n", flags.log)
		}
	}

	// Apply LLM behavior CLI overrides
	if flags.temperature >= 0 {
		cfg.LLM.Temperature = flags.temperature
	}
	if flags.maxTokens >= 0 {
		cfg.LLM.MaxTokens = flags.maxTokens
	}
	if flags.showLlmThinking != "" {
		switch flags.showLlmThinking {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowLlmThinking = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowLlmThinking = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-llm-thinking value %q, use on|off\n", flags.showLlmThinking)
		}
	}

	if flags.showCommand != "" {
		switch flags.showCommand {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowCommand = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowCommand = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-command value %q, use on|off\n", flags.showCommand)
		}
	}
	if flags.showLlmContent != "" {
		switch flags.showLlmContent {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowLlmContent = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowLlmContent = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-llm-content value %q, use on|off\n", flags.showLlmContent)
		}
	}
	if flags.showTool != "" {
		switch flags.showTool {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowTool = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowTool = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-tool value %q, use on|off\n", flags.showTool)
		}
	}
	if flags.showToolInput != "" {
		switch flags.showToolInput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowToolInput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowToolInput = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-tool-input value %q, use on|off\n", flags.showToolInput)
		}
	}
	if flags.showToolOutput != "" {
		switch flags.showToolOutput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowToolOutput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowToolOutput = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-tool-output value %q, use on|off\n", flags.showToolOutput)
		}
	}
	if flags.showCommandOutput != "" {
		switch flags.showCommandOutput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowCommandOutput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowCommandOutput = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --show-command-output value %q, use on|off\n", flags.showCommandOutput)
		}
	}

	if flags.confirmCommand != "" {
		switch flags.confirmCommand {
		case "on", "1", "true", "yes":
			cfg.LLM.ConfirmCommand = true
		case "off", "0", "false", "no":
			cfg.LLM.ConfirmCommand = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --confirm-command value %q, use on|off\n", flags.confirmCommand)
		}
	}
	if flags.resultMode != "" {
		if mode, ok := config.ParseResultMode(flags.resultMode); ok {
			cfg.LLM.ResultMode = int(mode)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid --result-mode value %q, use minimal/explain/analyze/free\n", flags.resultMode)
		}
	}

	// Apply agent identity CLI overrides
	if flags.description != "" {
		cfg.LLM.AgentDescription = flags.description
	}
	if flags.principles != "" {
		cfg.LLM.AgentPrinciples = flags.principles
	}

	// Apply vision CLI override
	if flags.vision != "" {
		switch flags.vision {
		case "on", "1", "true", "yes":
			cfg.LLM.VisionSupport = true
		case "off", "0", "false", "no":
			cfg.LLM.VisionSupport = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --vision value %q, use on|off\n", flags.vision)
		}
	}

	// Apply memory-enabled CLI override
	if flags.memoryEnabled != "" {
		switch flags.memoryEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.MemoryEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.MemoryEnabled = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --memory-enabled value %q, use on|off\n", flags.memoryEnabled)
		}
	}

	// Apply plan-enabled CLI override
	if flags.planEnabled != "" {
		switch flags.planEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.PlanEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.PlanEnabled = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --plan-enabled value %q, use on|off\n", flags.planEnabled)
		}
	}

	// Apply subagent-enabled CLI override
	if flags.subAgentEnabled != "" {
		switch flags.subAgentEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.SubAgentEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.SubAgentEnabled = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --subagent-enabled value %q, use on|off\n", flags.subAgentEnabled)
		}
	}

	// Apply timeout CLI overrides
	if flags.toolTimeout >= 0 {
		cfg.LLM.ToolTimeout = flags.toolTimeout
	}
	if flags.cmdTimeout >= 0 {
		cfg.LLM.CommandTimeout = flags.cmdTimeout
	}
	if flags.llmTimeout >= 0 {
		cfg.LLM.LLMTimeout = flags.llmTimeout
	}

	// Apply memory search config CLI overrides

	if flags.memorySearchMaxContentLen >= 0 {
		cfg.LLM.MemorySearchMaxContentLen = flags.memorySearchMaxContentLen
	}
	if flags.memorySearchMaxResults >= 0 {
		cfg.LLM.MemorySearchMaxResults = flags.memorySearchMaxResults
	}

	// Apply emoji-enabled CLI override
	if flags.emojiEnabled != "" {
		switch flags.emojiEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.EmojiEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.EmojiEnabled = false
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid --emoji-enabled value %q, use on|off\n", flags.emojiEnabled)
		}
	}

	// Apply error tracking config CLI overrides
	if flags.errorMaxSingleCount >= 0 {
		cfg.LLM.ErrorMaxSingleCount = flags.errorMaxSingleCount
	}
	if flags.errorMaxTypeCount >= 0 {
		cfg.LLM.ErrorMaxTypeCount = flags.errorMaxTypeCount
	}

	// Initialize logger with workspace
	if err := log.Init(cfg.LogEnabled, ws); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot initialize logger: %v\n", err)
	}
	defer log.Close()

	// Apply log level: CLI flag overrides config, config overrides default
	if flags.logLevel != "" {
		if level, ok := log.ParseLogLevel(flags.logLevel); ok {
			log.SetLevel(level)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid --log-level value %q, use debug/info/warn/error/off\n", flags.logLevel)
		}
	} else if cfg.LogLevel != "" {
		if level, ok := log.ParseLogLevel(cfg.LogLevel); ok {
			log.SetLevel(level)
		}
	}

	log.Info("co-shell v%s started (workspace: %s)", version, ws.Root())
	if flags.model != "" || flags.endpoint != "" || flags.apiKey != "" {
		log.Info("CLI overrides applied: model=%s endpoint=%s api-key=%s",
			flags.model, flags.endpoint, maskKey(flags.apiKey))
	}

	// Show disclaimer on first run
	if !cfg.DisclaimerAccepted {
		showDisclaimer(cfg, ws)
	}

	// Initialize persistent store with workspace
	s, err := store.NewStore(ws)
	if err != nil {
		log.Error("Cannot initialize store: %v", err)
		fmt.Fprintf(os.Stderr, "Error: cannot initialize store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Initialize MCP manager
	mcpMgr := mcp.NewManager()
	defer mcpMgr.Close()

	// Connect to enabled MCP servers from config
	for _, serverCfg := range cfg.MCP.Servers {
		if serverCfg.Enabled {
			if err := mcpMgr.AddServer(serverCfg.Name, serverCfg.Command, serverCfg.Args); err != nil {
				log.Warn("Cannot connect to MCP server %q: %v", serverCfg.Name, err)
				fmt.Fprintf(os.Stderr, "Warning: cannot connect to MCP server %q: %v\n", serverCfg.Name, err)
			} else {
				log.Info("Connected to MCP server: %s", serverCfg.Name)
			}
		}
	}

	// Run API setup wizard if configuration is incomplete
	if !isLLMConfigComplete(cfg) {
		log.Info("Running API setup wizard")
		if !wizard.RunSetupWizard(cfg) {
			fmt.Println(i18n.T(i18n.KeySetupCancelled))
			os.Exit(1)
		}
	}

	// Initialize LLM client
	var llmClient llm.Client
	if cfg.LLM.APIKey != "" {
		llmClient = llm.NewClient(
			cfg.LLM.Endpoint,
			cfg.LLM.APIKey,
			cfg.LLM.Model,
			cfg.LLM.Temperature,
			cfg.LLM.MaxTokens,
			cfg.LLM.LLMTimeout,
		)
		// Apply thinking/reasoning configuration
		llmClient.SetThinkingEnabled(cfg.LLM.ThinkingEnabled)
		llmClient.SetReasoningEffort(cfg.LLM.ReasoningEffort)
		log.Info("LLM client initialized: endpoint=%s model=%s llm_timeout=%ds thinking=%v reasoning_effort=%s",
			cfg.LLM.Endpoint, cfg.LLM.Model, cfg.LLM.LLMTimeout, cfg.LLM.ThinkingEnabled, cfg.LLM.ReasoningEffort)
	} else {
		// Create a no-op client that warns about missing API key
		llmClient = &noopClient{}
		log.Warn("No API key configured, using no-op LLM client")
	}

	// Build rules string
	rules := ""
	for _, rule := range cfg.Rules {
		rules += rule + "\n"
	}

	// Initialize agent
	ag := agent.New(llmClient, mcpMgr, s, rules)
	ag.SetWorkspacePath(ws.Root())

	// Initialize scheduler
	sch := scheduler.New(func(entry *scheduler.CronEntry) {
		ag.OnScheduledTaskTriggered(entry)
	})
	// Load persisted scheduler entries from store
	if entries, err := loadSchedulerEntries(s); err != nil {
		log.Warn("Cannot load scheduler entries: %v", err)
	} else {
		sch.LoadEntries(entries)
	}
	sch.Start()
	defer sch.Stop()

	ag.SetScheduler(sch)

	// Apply agent name: CLI flag overrides config
	if flags.agentName != "" {
		cfg.LLM.AgentName = flags.agentName
	}
	ag.SetName(cfg.LLM.AgentName)
	ag.SetShowLlmThinking(cfg.LLM.ShowLlmThinking)
	ag.SetShowLlmContent(cfg.LLM.ShowLlmContent)
	ag.SetShowTool(cfg.LLM.ShowTool)
	ag.SetShowToolInput(cfg.LLM.ShowToolInput)
	ag.SetShowToolOutput(cfg.LLM.ShowToolOutput)
	ag.SetShowCommand(cfg.LLM.ShowCommand)
	ag.SetShowCommandOutput(cfg.LLM.ShowCommandOutput)

	// Apply max iterations: CLI flag overrides config, config overrides default
	if flags.maxIterations >= 0 {
		ag.SetMaxIterations(flags.maxIterations)
	} else if cfg.LLM.MaxIterations > 0 {
		ag.SetMaxIterations(cfg.LLM.MaxIterations)
	} else {
		// Config has MaxIterations == 0 (e.g., loaded from old config.json without this field),
		// use the default value from DefaultConfig()
		ag.SetMaxIterations(config.DefaultConfig().LLM.MaxIterations)
	}

	// Apply command confirmation setting
	ag.SetConfirmCommand(cfg.LLM.ConfirmCommand)

	// Apply emoji enabled setting
	ag.SetEmojiEnabled(cfg.LLM.EmojiEnabled)

	// Pass config to agent for timeout settings
	ag.SetConfig(cfg)

	// Apply memory enabled setting
	ag.SetMemoryEnabled(cfg.LLM.MemoryEnabled)

	// Apply plan enabled setting
	ag.SetPlanEnabled(cfg.LLM.PlanEnabled)

	// Sync memory enabled to task plan manager
	ag.TaskPlanManager().SetMemoryEnabled(cfg.LLM.MemoryEnabled)

	// Sync agent name to task plan manager for memory archival
	ag.TaskPlanManager().SetAgentName(cfg.LLM.AgentName)

	// Apply subagent enabled setting
	ag.SetSubAgentEnabled(cfg.LLM.SubAgentEnabled)

	// Apply result mode
	ag.SetResultMode(config.ResultMode(cfg.LLM.ResultMode))

	// Set image paths for multimodal input if provided

	if flags.imagePaths != "" {
		// Check if the current model supports vision
		if !cfg.LLM.VisionSupport {
			ep := config.GetEmojiPrefixes(cfg.LLM.EmojiEnabled)
			fmt.Fprintf(os.Stderr, "%s 错误: 当前模型不支持视觉识别能力（VisionSupport=off），无法处理图片输入。\n", ep.Error)
			fmt.Fprintf(os.Stderr, "   请去掉-image参数或使用支持多模态的模型。\n")
			os.Exit(1)
		}
		paths := strings.Split(flags.imagePaths, ",")
		// Trim whitespace from each path
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
		ag.SetImagePaths(paths)
		log.Info("Image paths set for multimodal input: %v", paths)
	}

	log.Info("Agent initialized with %d rules", len(cfg.Rules))

	// If --command flag is provided, execute the single command and exit
	if flags.command != "" {
		executeSingleCommand(ag, cfg, flags.command)
		return
	}

	// Start REPL (interactive mode)
	r := repl.New(cfg, s, mcpMgr, ag)
	r.SetVersion(version, build)
	log.Info("REPL started")
	if err := r.Run(); err != nil {
		log.Error("REPL error: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// showDisclaimer displays the risk disclaimer and prompts the user to accept.
// If accepted, it saves the config with DisclaimerAccepted=true.
// If declined, it exits the program.
func showDisclaimer(cfg *config.Config, ws *workspace.Workspace) {
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyDisclaimerTitle))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyDisclaimerBody))
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(i18n.T(i18n.KeyDisclaimerPrompt))
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == i18n.T(i18n.KeyDisclaimerYes) || response == "yes" {
			cfg.DisclaimerAccepted = true
			if err := cfg.Save(); err != nil {
				log.Warn("Cannot save disclaimer acceptance: %v", err)
			}
			fmt.Println()
			return
		}

		if response == i18n.T(i18n.KeyDisclaimerNo) || response == "no" {
			fmt.Println(i18n.T(i18n.KeyDisclaimerRefused))
			os.Exit(0)
		}

		// Invalid input, prompt again
	}
}

// isDirectCommand checks if the input looks like a system command that can be
// executed directly. Delegates to repl package.
func isDirectCommand(input string) bool {
	_, ok := repl.IsDirectCommand(input)
	return ok
}

// executeSingleCommand executes a single command (natural language or system command)
// and prints the result, then exits.
func executeSingleCommand(ag *agent.Agent, cfg *config.Config, input string) {
	log.Info("Single command mode: %s", input)

	ep := config.GetEmojiPrefixes(cfg.LLM.EmojiEnabled)

	// Check if it's a direct system command
	if isDirectCommand(input) {
		// Direct system command
		if cfg.LLM.ShowCommand {
			fmt.Printf("$ %s\n", input)
		}
		output, err := ag.ExecuteCommandDirectly(input)
		if err != nil {
			fmt.Print(output)
			fmt.Printf("%s Error: %v\n", ep.Error, err)
			os.Exit(1)
		}
		if output != "" {
			fmt.Println(output)
		}
		return
	}

	// Natural language input - use agent with streaming output
	ctx := context.Background()
	_, err := ag.RunStream(ctx, input, func(eventType string, content string) {
		switch eventType {
		case "content_chunk":
			fmt.Print(content)
		case "thinking_chunk":
			fmt.Print(content)
		case "command":
			fmt.Printf("%s%s\n", ep.CommandInput, content)
		case "output":
			fmt.Println()
			fmt.Println(ep.OutputTitle)
			fmt.Println(ep.OutputSep)
			fmt.Println(content)
			fmt.Println(ep.OutputSep)
		case "tool_call":
			fmt.Printf("%s%s\n", ep.ToolCallInput, content)
		case "error":
			fmt.Printf("%s%s\n", ep.Error, content)
		case "done":
			fmt.Println()
		}
	})

	if err != nil {
		fmt.Printf("%s Error: %v\n", ep.Error, err)
		os.Exit(1)
	}
}

// isLLMConfigComplete checks whether the LLM configuration has all required fields.
func isLLMConfigComplete(cfg *config.Config) bool {
	return cfg.LLM.APIKey != "" &&
		cfg.LLM.Endpoint != "" &&
		cfg.LLM.Model != ""
}

// noopClient is a placeholder LLM client used when no API key is configured.
type noopClient struct{}

func (c *noopClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return nil, fmt.Errorf("%s", i18n.T(i18n.KeyNoopClientError))
}

func (c *noopClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	return nil, fmt.Errorf("%s", i18n.T(i18n.KeyNoopClientError))
}

func (c *noopClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}

func (c *noopClient) TestVisionSupport(ctx context.Context) bool {
	return false
}

func (c *noopClient) TestTextSupport(ctx context.Context) bool {
	return false
}

func (c *noopClient) SetThinkingEnabled(enabled bool) {}

func (c *noopClient) SetReasoningEffort(effort string) {}

func (c *noopClient) Close() error {
	return nil
}

// loadSchedulerEntries loads persisted scheduler entries from the store.
func loadSchedulerEntries(s *store.Store) ([]*scheduler.CronEntry, error) {
	entries, err := s.LoadSchedules()
	if err != nil {
		return nil, fmt.Errorf("cannot load schedules: %w", err)
	}

	var result []*scheduler.CronEntry
	for _, data := range entries {
		var entry scheduler.CronEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			log.Warn("Cannot unmarshal scheduler entry: %v", err)
			continue
		}
		result = append(result, &entry)
	}
	return result, nil
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
