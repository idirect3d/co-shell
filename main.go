// Author: L.Shuang
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
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
	flag.BoolVar(&f.showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&f.showHelp, "h", false, "显示帮助信息（简写）")
	flag.BoolVar(&f.showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&f.showVersion, "v", false, "显示版本信息（简写）")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `co-shell v%s - 智能命令行 Shell

用法:
  co-shell [选项]                    启动交互式 REPL
  co-shell [选项] <指令>             执行单条指令后退出

选项:
  -c, --config <path>    指定配置文件路径（默认: ~/.co-shell/config.json）
  -m, --model <name>     临时指定模型名称（覆盖配置文件）
  -e, --endpoint <url>   临时指定 API 端点（覆盖配置文件）
  -k, --api-key <key>    临时指定 API Key（覆盖配置文件）
      --log on|off       临时指定日志开关（覆盖配置文件）
      --max-iterations   最大迭代次数（-1 为不限制，默认 10）
  -v, --version          显示版本信息
  -h, --help             显示帮助信息

示例:
  co-shell                             启动交互式 REPL
  co-shell 列出当前目录的文件           执行自然语言指令
  co-shell "cat ~/.co-shell/config.json"  执行系统命令
  co-shell -m deepseek-chat 你好       指定模型并执行指令
  co-shell -k sk-xxxx --log off        临时指定 API Key 并关闭日志
`, version)
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
			fmt.Println("❌ 设置未完成，程序退出。")
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

// executeSingleCommand executes a single command (natural language or system command)
// and prints the result, then exits.
func executeSingleCommand(ag *agent.Agent, cfg *config.Config, input string) {
	log.Info("Single command mode: %s", input)

	// Check if it's a direct system command
	trimmed := strings.TrimSpace(input)
	firstWord := strings.Fields(trimmed)[0]
	if _, err := exec.LookPath(firstWord); err == nil {
		// Direct system command
		if cfg.LLM.ShowCommand {
			fmt.Printf("$ %s\n", trimmed)
		}
		output, err := ag.ExecuteCommandDirectly(trimmed)
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
			fmt.Println("📋 命令输出:")
			fmt.Println("────────────────────────────────────────────")
			fmt.Println(content)
			fmt.Println("────────────────────────────────────────────")
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
	return nil, fmt.Errorf("LLM not configured. Set your API key with: .settings api-key <your-key>")
}

func (c *noopClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	return nil, fmt.Errorf("LLM not configured. Set your API key with: .settings api-key <your-key>")
}

func (c *noopClient) ListModels(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("LLM not configured. Set your API key with: .settings api-key <your-key>")
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
