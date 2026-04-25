package main

import (
	"context"
	"fmt"
	"os"

	"github.com/liangshuang/co-shell/agent"
	"github.com/liangshuang/co-shell/config"
	"github.com/liangshuang/co-shell/llm"
	"github.com/liangshuang/co-shell/mcp"
	"github.com/liangshuang/co-shell/repl"
	"github.com/liangshuang/co-shell/store"
)

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
