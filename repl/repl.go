// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/wizard"
)

// version is the current co-shell version displayed in the welcome message.
const version = "0.3.0"

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

// IsDirectCommand checks if the input looks like a system command that can be
// executed directly. It extracts the first word and checks if it exists in PATH.
func IsDirectCommand(input string) (string, bool) {

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
	listHandler     *cmd.ListHandler
	imageHandler    *cmd.ImageHandler

	history    []string
	historyPos int
}

// New creates a new REPL instance.
func New(cfg *config.Config, s *store.Store, mcpMgr *mcp.Manager, ag *agent.Agent) *REPL {
	return &REPL{
		cfg:             cfg,
		store:           s,
		mcpMgr:          mcpMgr,
		agent:           ag,
		settingsHandler: cmd.NewSettingsHandler(cfg, ag),

		mcpHandler:     cmd.NewMCPHandler(cfg, mcpMgr),
		ruleHandler:    cmd.NewRuleHandler(cfg),
		memoryHandler:  cmd.NewMemoryHandler(s),
		contextHandler: cmd.NewContextHandler(s),
		listHandler:    cmd.NewListHandler(s),
		imageHandler:   cmd.NewImageHandler(ag),
	}

}

// Run starts the REPL loop using standard library input/output.
// No go-prompt, no raw terminal mode, no complex terminal control.
func (r *REPL) Run() error {
	// Print welcome message
	r.printWelcome()

	// Load persistent history from store
	r.loadHistory()

	// Set up signal handling for Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal main loop to exit
	done := make(chan struct{})

	// Handle signals in a goroutine
	go func() {
		select {
		case <-sigCh:
			fmt.Println("\n" + i18n.T(i18n.KeyGoodbye))
			r.cleanup()
			os.Exit(0)
		case <-done:
			return
		}
	}()

	// Main input loop using bufio.Scanner (standard line-buffered input)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		prompt := "❯ "
		if r.cfg.LLM.VisionSupport {
			prompt = "👀 "
		}
		fmt.Print(prompt)

		if !scanner.Scan() {
			// EOF (Ctrl+D) or error
			break
		}

		input := scanner.Text()
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		// Save to persistent history
		r.saveHistory(input)

		// Handle exit commands
		if input == "exit" || input == "quit" || input == ".exit" || input == ".quit" {
			break
		}

		// Handle help
		if input == "help" || input == ".help" || input == "?" {
			r.printHelp()
			continue
		}

		// Handle built-in commands (start with .)
		if strings.HasPrefix(input, ".") {
			r.handleBuiltin(input)
			continue
		}

		// Handle numeric input: re-execute a history entry by number
		if num, err := strconv.Atoi(input); err == nil && num > 0 {
			r.handleHistoryReExecute(num)
			continue
		}

		// Handle direct system commands (bypass LLM)
		if cmd, ok := IsDirectCommand(input); ok {

			r.handleSystemCommand(cmd)
			continue
		}

		// Handle natural language input via agent
		r.handleAgentInput(input)
	}

	close(done)
	r.cleanup()
	fmt.Println(i18n.T(i18n.KeyGoodbye))
	return nil
}

// loadHistory loads persistent history from the store.
func (r *REPL) loadHistory() {
	entries, err := r.store.LoadHistory()
	if err != nil {
		log.Warn("Cannot load history: %v", err)
		r.history = []string{}
		return
	}

	// Reverse to chronological order (oldest first)
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	r.history = entries
	r.historyPos = len(r.history)
	log.Debug("Loaded %d history entries", len(entries))
}

// saveHistory saves a single input to the persistent history store.
func (r *REPL) saveHistory(input string) {
	if err := r.store.SaveHistory(input); err != nil {
		log.Warn("Cannot save history: %v", err)
	}
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
	case ".settings", ".set":
		result, err = r.settingsHandler.Handle(args)

	case ".mcp":
		result, err = r.mcpHandler.Handle(args)
	case ".rule":
		result, err = r.ruleHandler.Handle(args)
	case ".memory":
		result, err = r.memoryHandler.Handle(args)
	case ".context":
		result, err = r.contextHandler.Handle(args)
	case ".wizard":
		r.handleWizard()
		return
	case ".list":
		result, err = r.listHandler.HandleList(args)
	case ".last":
		result, err = r.listHandler.HandleLast(args)
	case ".first":
		result, err = r.listHandler.HandleFirst(args)
	case ".image":
		result, err = r.imageHandler.Handle(args)
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
	if command == ".settings" || command == ".set" {

		r.agent.SetShowThinking(r.cfg.LLM.ShowThinking)
		r.agent.SetShowCommand(r.cfg.LLM.ShowCommand)
		r.agent.SetShowOutput(r.cfg.LLM.ShowOutput)
	}
}

// handleHistoryReExecute re-executes a history entry by its 1-based index.
func (r *REPL) handleHistoryReExecute(num int) {
	entries, err := r.store.ListHistory()
	if err != nil {
		fmt.Printf("❌ %s: %v\n", i18n.T(i18n.KeyError), err)
		return
	}

	if num < 1 || num > len(entries) {
		fmt.Println(i18n.TF(i18n.KeyListInvalid, len(entries)))
		return
	}

	input := entries[num-1].Input
	fmt.Printf("🔄 %s\n", input)

	// Check if it's a built-in command
	if strings.HasPrefix(input, ".") {
		r.handleBuiltin(input)
		return
	}

	// Check if it's a direct system command
	if cmd, ok := IsDirectCommand(input); ok {
		r.handleSystemCommand(cmd)
		return
	}

	// Otherwise, send to agent
	r.handleAgentInput(input)
}

// handleWizard runs the API setup wizard.
func (r *REPL) handleWizard() {
	fmt.Print(i18n.T(i18n.KeyWizardCmdRunning))

	if wizard.RunSetupWizard(r.cfg) {
		fmt.Print(i18n.T(i18n.KeyWizardCmdDone))
	} else {
		fmt.Println(i18n.T(i18n.KeySetupCancelled))
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

	// Print agent name with timestamp before streaming response
	fmt.Println()
	fmt.Println(r.agent.Said())

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
		fmt.Print(content)

	case "thinking_chunk":
		fmt.Print(content)

	case "content":
		fmt.Print(content)
		fmt.Println()

	case "thinking":
		fmt.Print(content)
		fmt.Println()

	case "command":
		fmt.Printf("⚡ %s\n", content)

	case "output":
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

// printWelcome displays the welcome message in a compact format
// similar to traditional Unix tools (e.g., zip, tar).
func (r *REPL) printWelcome() {
	visionIndicator := ""
	if r.cfg.LLM.VisionSupport {
		visionIndicator = " 👀"
	}
	fmt.Printf("co-shell v%s%s\n", version, visionIndicator)

	fmt.Println("Copyright (c) 2026 L.Shuang - Type '.help' for usage.")
	fmt.Println()
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
	fmt.Println(i18n.T(i18n.KeyHelpList))
	fmt.Println(i18n.T(i18n.KeyHelpLast))
	fmt.Println(i18n.T(i18n.KeyHelpFirst))
	fmt.Println(i18n.T(i18n.KeyHelpWizard))
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
