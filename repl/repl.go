package repl

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	prompt "github.com/c-bata/go-prompt"
	"github.com/liangshuang/co-shell/agent"
	"github.com/liangshuang/co-shell/cmd"
	"github.com/liangshuang/co-shell/config"
	"github.com/liangshuang/co-shell/mcp"
	"github.com/liangshuang/co-shell/store"
)

// BuiltinHandler defines the interface for built-in command handlers.
type BuiltinHandler interface {
	Handle(args []string) (string, error)
}

// REPL represents the interactive shell loop.
type REPL struct {
	cfg             *config.Config
	store           *store.Store
	mcpMgr          *mcp.Manager
	agent           *agent.Agent
	settingsHandler *cmd.SettingsHandler
	mcpHandler      *cmd.MCPHandler
	ruleHandler     *cmd.RuleHandler
	memoryHandler   *cmd.MemoryHandler
	contextHandler  *cmd.ContextHandler
}

// New creates a new REPL instance.
func New(cfg *config.Config, s *store.Store, mcpMgr *mcp.Manager, ag *agent.Agent) *REPL {
	return &REPL{
		cfg:             cfg,
		store:           s,
		mcpMgr:          mcpMgr,
		agent:           ag,
		settingsHandler: cmd.NewSettingsHandler(cfg),
		mcpHandler:      cmd.NewMCPHandler(cfg, mcpMgr),
		ruleHandler:     cmd.NewRuleHandler(cfg),
		memoryHandler:   cmd.NewMemoryHandler(s),
		contextHandler:  cmd.NewContextHandler(s),
	}
}

// Run starts the REPL loop.
func (r *REPL) Run() error {
	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n👋 Goodbye!")
		r.cleanup()
		os.Exit(0)
	}()

	// Print welcome message
	r.printWelcome()

	// Start the prompt
	p := prompt.New(
		r.executor,
		r.completer,
		prompt.OptionTitle("co-shell"),
		prompt.OptionPrefix("❯ "),
		prompt.OptionInputTextColor(prompt.Cyan),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionHistory([]string{}),
		prompt.OptionMaxSuggestion(10),
	)

	p.Run()
	return nil
}

// executor handles each line of input.
func (r *REPL) executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle exit commands
	if input == "exit" || input == "quit" || input == ".exit" || input == ".quit" {
		fmt.Println("👋 Goodbye!")
		r.cleanup()
		os.Exit(0)
	}

	// Handle help
	if input == "help" || input == ".help" || input == "?" {
		r.printHelp()
		return
	}

	// Handle built-in commands (start with .)
	if strings.HasPrefix(input, ".") {
		r.handleBuiltin(input)
		return
	}

	// Handle natural language input via agent
	r.handleAgentInput(input)
}

// handleBuiltin processes built-in commands.
func (r *REPL) handleBuiltin(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	args := parts[1:]

	var result string
	var err error

	switch command {
	case ".settings":
		result, err = r.settingsHandler.Handle(args)
	case ".mcp":
		result, err = r.mcpHandler.Handle(args)
	case ".rule":
		result, err = r.ruleHandler.Handle(args)
	case ".memory":
		result, err = r.memoryHandler.Handle(args)
	case ".context":
		result, err = r.contextHandler.Handle(args)
	default:
		fmt.Printf("Unknown command: %s\nType .help for available commands\n", command)
		return
	}

	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// handleAgentInput sends natural language input to the agent.
func (r *REPL) handleAgentInput(input string) {
	fmt.Println("🤔 Thinking...")

	ctx := context.Background()
	response, err := r.agent.Run(ctx, input)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println(response)
}

// completer provides tab completion suggestions.
func (r *REPL) completer(d prompt.Document) []prompt.Suggest {
	text := d.GetWordBeforeCursor()

	// If we're typing a builtin command
	if strings.HasPrefix(d.Text, ".") {
		return r.builtinCompleter(d)
	}

	return []prompt.Suggest{
		{Text: ".settings", Description: "Manage LLM settings"},
		{Text: ".mcp", Description: "Manage MCP servers"},
		{Text: ".rule", Description: "Manage global rules"},
		{Text: ".memory", Description: "Manage memory"},
		{Text: ".context", Description: "Manage context"},
		{Text: ".help", Description: "Show help"},
		{Text: ".exit", Description: "Exit co-shell"},
	}
}

// builtinCompleter provides completion for built-in commands.
func (r *REPL) builtinCompleter(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()

	// Complete the command name
	if !strings.Contains(text, " ") {
		return []prompt.Suggest{
			{Text: ".settings", Description: "Manage LLM API settings"},
			{Text: ".mcp", Description: "Manage MCP server connections"},
			{Text: ".rule", Description: "Manage global rules"},
			{Text: ".memory", Description: "Manage memory and knowledge"},
			{Text: ".context", Description: "Manage conversation context"},
			{Text: ".help", Description: "Show help information"},
			{Text: ".exit", Description: "Exit co-shell"},
		}
	}

	// Complete subcommands
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return nil
	}

	command := parts[0]
	subPrefix := parts[len(parts)-1]

	switch command {
	case ".settings":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "api-key", Description: "Set API key"},
			{Text: "endpoint", Description: "Set API endpoint URL"},
			{Text: "model", Description: "Set model name"},
			{Text: "temperature", Description: "Set temperature (0.0-2.0)"},
			{Text: "max-tokens", Description: "Set max tokens"},
		}, subPrefix, true)
	case ".mcp":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "add", Description: "Add a new MCP server"},
			{Text: "remove", Description: "Remove an MCP server"},
			{Text: "list", Description: "List all MCP servers"},
			{Text: "enable", Description: "Enable an MCP server"},
			{Text: "disable", Description: "Disable an MCP server"},
		}, subPrefix, true)
	case ".rule":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "add", Description: "Add a new rule"},
			{Text: "remove", Description: "Remove a rule by index"},
			{Text: "clear", Description: "Clear all rules"},
		}, subPrefix, true)
	case ".memory":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "save", Description: "Save a memory entry"},
			{Text: "get", Description: "Get a memory entry"},
			{Text: "search", Description: "Search memory entries"},
			{Text: "delete", Description: "Delete a memory entry"},
			{Text: "clear", Description: "Clear all memory"},
		}, subPrefix, true)
	case ".context":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "show", Description: "Show current context"},
			{Text: "reset", Description: "Reset context"},
			{Text: "set", Description: "Set a context variable"},
		}, subPrefix, true)
	}

	return nil
}

// printWelcome displays the welcome message.
func (r *REPL) printWelcome() {
	fmt.Println(`
╔══════════════════════════════════════╗
║         co-shell v0.1.0              ║
║   Intelligent Command-Line Shell     ║
╚══════════════════════════════════════╝

Type .help for available commands, or just type in natural language!
`)
}

// printHelp displays the help information.
func (r *REPL) printHelp() {
	fmt.Println(`
Available Commands:

  Natural Language:
    Just type your request in natural language, and I'll help you execute it.

  Built-in Commands (start with .):
    .settings     - Manage LLM API settings (key, model, endpoint, etc.)
    .mcp          - Manage MCP server connections
    .rule         - Manage global rules for the AI
    .memory       - Manage memory and persistent knowledge
    .context      - Manage conversation context
    .help         - Show this help message
    .exit         - Exit co-shell

  Examples:
    ❯ List all files in the current directory
    ❯ Find all large files over 100MB
    ❯ .settings model gpt-4o
    ❯ .mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp
    ❯ .rule add "Always confirm before deleting files"
`)
}

// cleanup performs cleanup operations before exit.
func (r *REPL) cleanup() {
	fmt.Print("Cleaning up...")
	if err := r.mcpMgr.Close(); err != nil {
		fmt.Printf(" MCP error: %v", err)
	}
	if err := r.store.Close(); err != nil {
		fmt.Printf(" DB error: %v", err)
	}
	fmt.Println(" Done.")
}
