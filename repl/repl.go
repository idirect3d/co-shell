// Author: L.Shuang
package repl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	prompt "github.com/c-bata/go-prompt"
	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
)

// commandPattern matches inputs that look like system commands:
// - Starts with a known command word (alphanumeric, hyphens, underscores, dots, slashes, tildes)
// - Optionally followed by arguments (anything)
// - May contain shell operators (|, >, <, &&, ||, ;)
var commandPattern = regexp.MustCompile(`^[a-zA-Z0-9._/~-]+(\s+.*)?$`)

// isDirectCommand checks if the input looks like a system command that can be
// executed directly. It extracts the first word and checks if it exists in PATH.
func isDirectCommand(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false
	}

	// Must match the basic command pattern
	if !commandPattern.MatchString(trimmed) {
		return "", false
	}

	// Extract the first word as the command name
	firstWord := strings.Fields(trimmed)[0]

	// Check if the command exists in PATH
	_, err := exec.LookPath(firstWord)
	if err != nil {
		return "", false
	}

	return trimmed, true
}

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

	// Load persistent history from store
	history := r.loadHistory()

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
		prompt.OptionHistory(history),
		prompt.OptionMaxSuggestion(10),
	)

	p.Run()
	return nil
}

// loadHistory loads persistent history from the store.
// Returns history entries in chronological order (oldest first) for go-prompt.
func (r *REPL) loadHistory() []string {
	entries, err := r.store.LoadHistory()
	if err != nil {
		log.Warn("Cannot load history: %v", err)
		return []string{}
	}

	// Reverse to chronological order (oldest first) for go-prompt
	// LoadHistory returns newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	log.Debug("Loaded %d history entries", len(entries))
	return entries
}

// saveHistory saves a single input to the persistent history store.
func (r *REPL) saveHistory(input string) {
	if err := r.store.SaveHistory(input); err != nil {
		log.Warn("Cannot save history: %v", err)
	}
}

// executor handles each line of input.
func (r *REPL) executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Save to persistent history
	r.saveHistory(input)

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

	// Handle direct system commands (bypass LLM)
	if cmd, ok := isDirectCommand(input); ok {
		r.handleSystemCommand(cmd)
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
		fmt.Printf("❌ 未知命令: %s\n输入 .help 查看可用命令列表\n", command)
		return
	}

	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}
	fmt.Println(result)

	// Update agent settings after handling settings command that may have changed them
	if command == ".settings" {
		r.agent.SetShowThinking(r.cfg.LLM.ShowThinking)
		r.agent.SetShowCommand(r.cfg.LLM.ShowCommand)
		r.agent.SetShowOutput(r.cfg.LLM.ShowOutput)
	}
}

// handleSystemCommand executes a system command directly and displays the output.
func (r *REPL) handleSystemCommand(command string) {
	// Show command if enabled
	if r.cfg.LLM.ShowCommand {
		fmt.Printf("$ %s\n", command)
	}

	output, err := r.agent.ExecuteCommandDirectly(command)
	if err != nil {
		// output may contain partial stdout before the error occurred
		if output != "" {
			fmt.Print(output)
		}
		fmt.Printf("❌ 命令执行失败: %v\n", err)
		return
	}

	if output != "" {
		fmt.Println(output)
	}
}

// handleAgentInput sends natural language input to the agent with streaming output.
func (r *REPL) handleAgentInput(input string) {
	ctx := context.Background()

	// Use streaming version
	_, err := r.agent.RunStream(ctx, input, r.streamCallback)
	if err != nil {
		fmt.Printf("❌ 处理失败: %v\n", err)
		fmt.Println("💡 提示: 请检查 API 配置是否正确，输入 .settings 查看当前配置")
		return
	}
}

// streamCallback handles streaming events from the agent.
func (r *REPL) streamCallback(eventType string, content string) {
	switch eventType {
	case "content_chunk":
		// Stream output content in real-time
		fmt.Print(content)

	case "thinking_chunk":
		// Thinking content is displayed dimmed/grayed
		fmt.Print(content)

	case "content":
		fmt.Print(content)
		fmt.Println()

	case "thinking":
		fmt.Print(content)
		fmt.Println()

	case "command":
		// Show the command that will be executed
		fmt.Printf("⚡ %s\n", content)

	case "output":
		// Show the full command output (stdout + stderr) before LLM analysis
		fmt.Println()
		fmt.Println("📋 命令输出:")
		fmt.Println("────────────────────────────────────────────")
		fmt.Println(content)
		fmt.Println("────────────────────────────────────────────")
		fmt.Println()

	case "tool_call":
		fmt.Println(content)

	case "error":
		fmt.Printf("❌ %s\n", content)

	case "done":
		fmt.Println()
	}
}

// completer provides tab completion suggestions.
// Only shows suggestions when input starts with "." (built-in commands).
// Press Tab to show, Esc to hide.
func (r *REPL) completer(d prompt.Document) []prompt.Suggest {
	// Only show suggestions for built-in commands (starting with .)
	if strings.HasPrefix(d.Text, ".") {
		return r.builtinCompleter(d)
	}

	// Return empty list for natural language input to avoid auto-popup
	return []prompt.Suggest{}
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
			{Text: "show-thinking", Description: "Show/hide LLM thinking process (on|off)"},
			{Text: "show-command", Description: "Show/hide commands before execution (on|off)"},
			{Text: "show-output", Description: "Show/hide command output before LLM analysis (on|off)"},
			{Text: "log", Description: "Enable/disable file logging (on|off)"},
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
	fmt.Print(`
╔══════════════════════════════════════╗
║         co-shell v0.1.0              ║
║   Intelligent Command-Line Shell     ║
╚══════════════════════════════════════╝

Type .help for available commands, or just type in natural language!
`)
}

// printHelp displays the help information.
func (r *REPL) printHelp() {
	fmt.Print(`
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
