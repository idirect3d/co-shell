// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/wizard"
)

const version = "0.1.0"

// cliFlags holds parsed command-line flags.
type cliFlags struct {
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
}

func parseFlags() cliFlags {
	var f cliFlags

	// Define flags
	flag.StringVar(&f.configPath, "config", "", "指定配置文件路径")
	flag.StringVar(&f.configPath, "c", "", "指定配置文件路径（简写）")
	flag.StringVar(&f.model, "model", "", "临时指定模型名称（覆盖配置文件）")
	flag.StringVar(&f.model, "m", "", "临时指定模型名称（简写）")
	flag.StringVar(&f.endpoint, "endpoint", "", "临时指定 API 端点（覆盖配置文件）")
	flag.StringVar(&f.endpoint, "e", "", "临时指定 API 端点（简写）")
	flag.StringVar(&f.apiKey, "api-key", "", "临时指定 API Key（覆盖配置文件）")
	flag.StringVar(&f.apiKey, "k", "", "临时指定 API Key（简写）")
	flag.StringVar(&f.log, "log", "", "临时指定日志开关（on/off，覆盖配置文件）")
	flag.IntVar(&f.maxIterations, "max-iterations", -1, "最大迭代次数（-1 为不限制，默认 10）")
	flag.StringVar(&f.lang, "lang", "", "设置语言（zh/en，默认自动检测）")
	flag.BoolVar(&f.showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&f.showHelp, "h", false, "显示帮助信息（简写）")
	flag.BoolVar(&f.showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&f.showVersion, "v", false, "显示版本信息（简写）")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s

%s

  %s
  %s

%s

  %s
  %s
  %s
  %s
  %s
  %s
  %s
  %s
  %s

%s

  %s
  %s
  %s
  %s
  %s
  %s
  %s
`,
			i18n.TF(i18n.KeyCLIHelpTitle, version),
			i18n.T(i18n.KeyCLIHelpUsage),
			i18n.T(i18n.KeyCLIHelpUsageREPL),
			i18n.T(i18n.KeyCLIHelpUsageCmd),
			i18n.T(i18n.KeyCLIHelpOptions),
			i18n.T(i18n.KeyCLIHelpConfig),
			i18n.T(i18n.KeyCLIHelpModel),
			i18n.T(i18n.KeyCLIHelpEndpoint),
			i18n.T(i18n.KeyCLIHelpAPIKey),
			i18n.T(i18n.KeyCLIHelpLang),
			i18n.T(i18n.KeyCLIHelpLog),
			i18n.T(i18n.KeyCLIHelpMaxIter),
			i18n.T(i18n.KeyCLIHelpVersion),
			i18n.T(i18n.KeyCLIHelpHelp),
			i18n.T(i18n.KeyCLIHelpExamples),
			i18n.T(i18n.KeyCLIHelpEx1),
			i18n.T(i18n.KeyCLIHelpEx2),
			i18n.T(i18n.KeyCLIHelpEx3),
			i18n.T(i18n.KeyCLIHelpEx4),
			i18n.T(i18n.KeyCLIHelpEx5),
			i18n.T(i18n.KeyCLIHelpEx6),
			i18n.T(i18n.KeyCLIHelpEx7),
		)
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
		fmt.Printf("co-shell v%s\n", version)
		os.Exit(0)
	}

	// Load configuration from multiple locations in priority order:
	// 1. CLI-specified path (--config / -c)
	// 2. ./config.json (current directory)
	// 3. ~/.co-shell/config.json (home directory)
	cfg, configPath, err := config.LoadWithPath(flags.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot load config: %v\n", err)
		cfg = config.DefaultConfig()
	}
	if configPath != "" {
		log.Info("Config loaded from: %s", configPath)
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

	// Initialize logger
	if err := log.Init(cfg.LogEnabled); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot initialize logger: %v\n", err)
	}
	defer log.Close()

	log.Info("co-shell v%s started", version)
	if flags.model != "" || flags.endpoint != "" || flags.apiKey != "" {
		log.Info("CLI overrides applied: model=%s endpoint=%s api-key=%s",
			flags.model, flags.endpoint, maskKey(flags.apiKey))
	}

	// Show disclaimer on first run
	if !cfg.DisclaimerAccepted {
		showDisclaimer(cfg)
	}

	// Initialize persistent store
	s, err := store.NewStore()
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
		)
		log.Info("LLM client initialized: endpoint=%s model=%s", cfg.LLM.Endpoint, cfg.LLM.Model)
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
	ag.SetShowThinking(cfg.LLM.ShowThinking)
	ag.SetShowCommand(cfg.LLM.ShowCommand)
	ag.SetShowOutput(cfg.LLM.ShowOutput)

	// Apply max iterations: CLI flag overrides config, config overrides default
	if flags.maxIterations >= 0 {
		ag.SetMaxIterations(flags.maxIterations)
	} else if cfg.LLM.MaxIterations > 0 {
		ag.SetMaxIterations(cfg.LLM.MaxIterations)
	}

	// Apply command confirmation setting
	ag.SetConfirmCommand(cfg.LLM.ConfirmCommand)

	log.Info("Agent initialized with %d rules", len(cfg.Rules))

	// If --command flag is provided, execute the single command and exit
	if flags.command != "" {
		executeSingleCommand(ag, cfg, flags.command)
		return
	}

	// Start REPL (interactive mode)
	r := repl.New(cfg, s, mcpMgr, ag)
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
func showDisclaimer(cfg *config.Config) {
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

// windowsBuiltins is a set of cmd.exe built-in commands that are not found by exec.LookPath.
var windowsBuiltins = map[string]bool{
	"dir": true, "copy": true, "del": true, "erase": true, "move": true,
	"ren": true, "rename": true, "type": true, "cd": true, "chdir": true,
	"md": true, "mkdir": true, "rd": true, "rmdir": true, "cls": true,
	"echo": true, "set": true, "path": true, "prompt": true, "title": true,
	"date": true, "time": true, "ver": true, "vol": true, "label": true,
	"pushd": true, "popd": true, "where": true, "find": true, "findstr": true,
	"more": true, "sort": true, "pause": true, "color": true, "help": true,
	"break": true, "call": true, "exit": true, "for": true, "goto": true,
	"if": true, "rem": true, "shift": true, "start": true,
	"assoc": true, "ftype": true, "dpath": true, "subst": true,
}

// isDirectCommand checks if the input looks like a system command that can be
// executed directly. It extracts the first word and checks if it exists in PATH.
func isDirectCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}

	// Extract the first word as the command name
	firstWord := strings.Fields(trimmed)[0]

	// Check if the command exists in PATH
	_, err := exec.LookPath(firstWord)
	if err == nil {
		return true
	}

	// On Windows, also check for cmd.exe built-in commands
	if runtime.GOOS == "windows" && windowsBuiltins[strings.ToLower(firstWord)] {
		return true
	}

	return false
}

// executeSingleCommand executes a single command (natural language or system command)
// and prints the result, then exits.
func executeSingleCommand(ag *agent.Agent, cfg *config.Config, input string) {
	log.Info("Single command mode: %s", input)

	// Check if it's a direct system command
	if isDirectCommand(input) {
		// Direct system command
		if cfg.LLM.ShowCommand {
			fmt.Printf("$ %s\n", input)
		}
		output, err := ag.ExecuteCommandDirectly(input)
		if err != nil {
			fmt.Print(output)
			fmt.Printf("❌ Error: %v\n", err)
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
			fmt.Printf("⚡ %s\n", content)
		case "output":
			fmt.Println()
			fmt.Println(i18n.T(i18n.KeyOutputTitle))
			fmt.Println(i18n.T(i18n.KeyOutputSep))
			fmt.Println(content)
			fmt.Println(i18n.T(i18n.KeyOutputSep))
		case "tool_call":
			fmt.Println(content)
		case "error":
			fmt.Printf("❌ %s\n", content)
		case "done":
			fmt.Println()
		}
	})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
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
	return nil, fmt.Errorf(i18n.T(i18n.KeyNoopClientError))
}

func (c *noopClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	return nil, fmt.Errorf(i18n.T(i18n.KeyNoopClientError))
}

func (c *noopClient) ListModels(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf(i18n.T(i18n.KeyNoopClientError))
}

func (c *noopClient) Close() error {
	return nil
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
