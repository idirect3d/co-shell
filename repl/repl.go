// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
package repl

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	prompt "github.com/c-bata/go-prompt"
	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
)

// commandPattern matches inputs that look like system commands.
// On Unix: starts with alphanumeric, dots, underscores, hyphens, slashes, tildes
// On Windows: also allows backslashes, colons (drive letters)
var commandPattern = regexp.MustCompile(commandPatternString())

// commandPatternString returns the appropriate regex pattern for the current platform.
func commandPatternString() string {
	if runtime.GOOS == "windows" {
		return `^[a-zA-Z0-9._~\\:/-]+(\s+.*)?$`
	}
	return `^[a-zA-Z0-9._/~-]+(\s+.*)?$`
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
	if err == nil {
		return trimmed, true
	}

	// On Windows, also check for cmd.exe built-in commands
	if runtime.GOOS == "windows" && windowsBuiltins[strings.ToLower(firstWord)] {
		return trimmed, true
	}

	return "", false
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
		prompt.OptionSetExitCheckerOnInput(func(in string, breakline bool) bool {
			// Check if the input is an exit command
			trimmed := strings.TrimSpace(in)
			if trimmed == "exit" || trimmed == "quit" || trimmed == ".exit" || trimmed == ".quit" {
				return true
			}
			return false
		}),
	)

	// Note: go-prompt handles SIGINT (Ctrl+C) internally:
	//   1. Calls p.tearDown() to restore terminal from raw mode
	//   2. Calls os.Exit(0)
	// So we do NOT register our own signal handler here, as it would
	// conflict with go-prompt's handler and cause terminal state corruption.
	p.Run()

	// After go-prompt exits cleanly (via ExitChecker), perform cleanup
	r.cleanup()
	fmt.Println(i18n.T(i18n.KeyGoodbye))
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

	// Handle exit commands - ExitChecker handles the actual exit,
	// but we still need to handle it here for the case where
	// ExitChecker is not set (e.g., tests).
	if input == "exit" || input == "quit" || input == ".exit" || input == ".quit" {
		return
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
		fmt.Printf(i18n.T(i18n.KeyUnknownCommand)+"\n", command)
		return
	}

	if err != nil {
		fmt.Printf("❌ %s: %v\n", i18n.T(i18n.KeyError), err)
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
		fmt.Printf("❌ %s: %v\n", i18n.T(i18n.KeyCmdFailed), err)
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
		fmt.Printf("❌ %s: %v\n", i18n.T(i18n.KeyProcessFailed), err)
		fmt.Println(i18n.T(i18n.KeyCheckConfig))
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
		fmt.Println(i18n.T(i18n.KeyOutputTitle))
		fmt.Println(i18n.T(i18n.KeyOutputSep))
		fmt.Println(content)
		fmt.Println(i18n.T(i18n.KeyOutputSep))
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

` + i18n.T(i18n.KeyWelcomeTip) + `
`)
}

// printHelp displays the help information.
func (r *REPL) printHelp() {
	fmt.Println(i18n.T(i18n.KeyHelpTitle))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpNLTitle))
	fmt.Println(i18n.T(i18n.KeyHelpNLDesc))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpBuiltinTitle))
	fmt.Println(i18n.T(i18n.KeyHelpSettings))
	fmt.Println(i18n.T(i18n.KeyHelpMCP))
	fmt.Println(i18n.T(i18n.KeyHelpRule))
	fmt.Println(i18n.T(i18n.KeyHelpMemory))
	fmt.Println(i18n.T(i18n.KeyHelpContext))
	fmt.Println(i18n.T(i18n.KeyHelpHelp))
	fmt.Println(i18n.T(i18n.KeyHelpExit))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpExampleTitle))
	fmt.Println(i18n.T(i18n.KeyHelpExample1))
	fmt.Println(i18n.T(i18n.KeyHelpExample2))
	fmt.Println(i18n.T(i18n.KeyHelpExample3))
	fmt.Println(i18n.T(i18n.KeyHelpExample4))
	fmt.Println(i18n.T(i18n.KeyHelpExample5))
}

// cleanup performs cleanup operations before exit.
func (r *REPL) cleanup() {
	fmt.Print(i18n.T(i18n.KeyCleaningUp))
	if err := r.mcpMgr.Close(); err != nil {
		fmt.Printf(" MCP error: %v", err)
	}
	if err := r.store.Close(); err != nil {
		fmt.Printf(" DB error: %v", err)
	}
	fmt.Println(i18n.T(i18n.KeyDone))
}
