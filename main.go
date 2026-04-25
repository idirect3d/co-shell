package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/store"
)

// readLine reads a line from stdin, trimming whitespace and carriage returns.
func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimRight(scanner.Text(), "\r\n ")
	}
	return ""
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Initialize persistent store
	s, err := store.NewStore()
	if err != nil {
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
				fmt.Fprintf(os.Stderr, "Warning: cannot connect to MCP server %q: %v\n", serverCfg.Name, err)
			}
		}
	}

	// Run API setup wizard if configuration is incomplete
	if !isLLMConfigComplete(cfg) {
		runSetupWizard(cfg)
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
	} else {
		// Create a no-op client that warns about missing API key
		llmClient = &noopClient{}
	}

	// Build rules string
	rules := ""
	for _, rule := range cfg.Rules {
		rules += rule + "\n"
	}

	// Initialize agent
	ag := agent.New(llmClient, mcpMgr, s, rules)

	// Start REPL
	r := repl.New(cfg, s, mcpMgr, ag)
	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// isLLMConfigComplete checks whether the LLM configuration has all required fields.
func isLLMConfigComplete(cfg *config.Config) bool {
	return cfg.LLM.APIKey != "" &&
		cfg.LLM.Endpoint != "" &&
		cfg.LLM.Model != ""
}

// runSetupWizard guides the user through configuring the LLM API settings interactively.
func runSetupWizard(cfg *config.Config) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║        🔧 co-shell API 设置向导                           ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  您需要先完成大模型 API 的配置，才能开始使用 co-shell。    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: API Endpoint
	fmt.Printf("📌 API 端点 [%s]: ", cfg.LLM.Endpoint)
	input := readLine()
	if input != "" {
		cfg.LLM.Endpoint = input
	}

	// Step 2: Model name
	fmt.Printf("📌 模型名称 [%s]: ", cfg.LLM.Model)
	input = readLine()
	if input != "" {
		cfg.LLM.Model = input
	}

	// Step 3: API Key (required, loop until test passes)
	for {
		fmt.Print("📌 API Key (必填): ")
		input = readLine()
		if input == "" {
			fmt.Println("⚠️  API Key 不能为空，请重新输入。")
			continue
		}
		cfg.LLM.APIKey = input

		// Test the connection
		fmt.Print("🔄 正在测试 API 连接...")
		if err := testAPIConnection(cfg); err != nil {
			fmt.Printf("\n❌ 连接测试失败: %v\n", err)
			fmt.Println("请检查 API Key 是否正确，或重新输入。")
			cfg.LLM.APIKey = "" // reset so wizard continues
			continue
		}
		fmt.Println(" ✅ 连接成功！")
		break
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		fmt.Printf("⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Println("✅ 配置已保存到 ~/.co-shell/config.json")
	}
	fmt.Println()
}

// testAPIConnection sends a simple chat completion request to verify the configuration.
func testAPIConnection(cfg *config.Config) error {
	client := llm.NewClient(cfg.LLM.Endpoint, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Temperature, cfg.LLM.MaxTokens)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15)
	defer cancel()

	_, err := client.Chat(ctx, []llm.Message{
		{Role: "user", Content: "Respond with exactly: OK"},
	}, nil)
	return err
}

// noopClient is a placeholder LLM client used when no API key is configured.
type noopClient struct{}

func (c *noopClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.LLMResponse, error) {
	return nil, fmt.Errorf("LLM not configured. Set your API key with: .settings api-key <your-key>")
}

func (c *noopClient) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error) {
	return nil, fmt.Errorf("LLM not configured. Set your API key with: .settings api-key <your-key>")
}

func (c *noopClient) Close() error {
	return nil
}
