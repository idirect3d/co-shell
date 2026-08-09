// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-05-13
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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/repl"
	"github.com/idirect3d/co-shell/scheduler"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

const version = "0.7.3"

const build = "383"

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
	temperature       float64
	maxTokens         int
	topP              float64
	topK              int
	repetitionPenalty float64
	showLlmThinking   string // "on"/"off"

	showLlmContent string // "on"/"off"
	showTool       string // "on"/"off"
	showToolInput  string // "on"/"off"
	showToolOutput string // "on"/"off"
	// showParseErrorRaw: "on"/"off" — display raw LLM message on tool call parse errors (FEATURE-336)
	showParseErrorRaw string
	showCommand       string // "on"/"off"
	showCommandOutput string // "on"/"off"
	confirmTool       string // "on"/"off" for default
	resultMode        string // minimal/explain/analyze/free
	outputCategories  string // "cat=on;cat2=off" CLI override for OutputCategories

	// Agent identity parameters
	description string
	// Vision support
	vision string // "on"/"off"
	// VisionContextMode: how much context to send to the vision model (FEATURE-319)
	visionContextMode string // "minimal"/"full"

	// Memory enabled
	memoryEnabled string // "on"/"off"

	// Plan enabled
	planEnabled string // "on"/"off"

	// SubAgent enabled
	subAgentEnabled string // "on"/"off"

	// ToolCall enabled
	toolCallEnabled string // "on"/"off"

	// ToolCall mode (FEATURE-182)
	toolCallMode string // "openai"/"xml"

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

	// Token usage display mode
	tokenUsage string // "on"/"off"/"none"

	// Show logo on startup
	showLogo string // "on"/"off"

	// Session ID
	sessionID string

	// Context start mode
	contextPolicy string // "window"/"task"/"smart"/"reorganize"

	// External config file generation
	initCapabilities bool
	initRules        bool
	unloadPrinciples bool
	unloadCapsAlias  bool
	unloadRulesAlias bool

	// Loop intervention (FEATURE-267)
	loopIntervention string // off/retry/prompt/reorganize/temperature/random

	// Loop temperature adjustment (FEATURE-230)
	loopTempEnabled  string  // "on"/"off"
	loopTempStepUp   float64 // temperature increase step (default: -1, use config)
	loopTempStepDown float64 // temperature decrease step (default: -1, use config)
	loopTempMax      float64 // temperature max bound (default: -1, use config)
	loopTempMin      float64 // temperature min bound (default: -1, use config)

	// Body additions: custom JSON properties to add to the LLM request body
	bodyAdd string // format: key=value, can be specified multiple times

	// Input mode
	inputMode string

	// Unload mode (FEATURE-245)
	unloadMode string // mode name to unload sections to disk

	// Debug mode
	debugMode string // "on"/"off"

	// Work mode (FEATURE-288)
	workMode string // act/plan/research

	// Thinking enabled
	thinkingEnabled string // "on"/"off"/"default"

	// Reasoning effort
	reasoningEffort string // "low"/"medium"/"high"/"max"/"none"/"default"

	// Max retries
	maxRetries int

	// Context limit
	contextLimit int

	// Shell session enabled
	shellSessionEnabled string // "on"/"off"

	// Browser enabled
	browserEnabled string // "on"/"off"
}

func parseFlags() cliFlags {
	var f cliFlags

	// Define flags
	flag.StringVar(&f.workspacePath, "workspace", "", "Set workspace path (default: current directory)")
	flag.StringVar(&f.workspacePath, "w", "", "Set workspace path (short)")
	flag.StringVar(&f.configPath, "config", "", "Set config file path (default: {workspace}/config.json)")
	flag.StringVar(&f.configPath, "c", "", "Set config file path (short)")
	flag.StringVar(&f.model, "model", "", "Temporarily specify model name (overrides config file)")
	flag.StringVar(&f.model, "m", "", "Temporarily specify model name (short)")
	flag.StringVar(&f.endpoint, "endpoint", "", "Temporarily specify API endpoint (overrides config file)")
	flag.StringVar(&f.endpoint, "e", "", "Temporarily specify API endpoint (short)")
	flag.StringVar(&f.apiKey, "api-key", "", "Temporarily specify API Key (overrides config file)")
	flag.StringVar(&f.apiKey, "k", "", "Temporarily specify API Key (short)")
	flag.StringVar(&f.log, "log", "", "Temporarily set log switch (on/off, overrides config file)")
	flag.IntVar(&f.maxIterations, "max-iterations", -1, "Max iterations (-1 unlimited, default 1000)")
	flag.StringVar(&f.agentName, "name", "", "Set agent name (default: co-shell; used for logs, sub-agent workspace, etc.)")
	flag.StringVar(&f.agentName, "n", "", "Set agent name (short)")
	flag.StringVar(&f.lang, "lang", "", "Set language (zh/en, default auto-detect)")
	flag.StringVar(&f.imagePaths, "image", "", "Image file paths (comma-separated), for multimodal input")
	flag.StringVar(&f.imagePaths, "i", "", "Image file paths (short)")
	flag.BoolVar(&f.showHelp, "help", false, "Show help")
	flag.BoolVar(&f.showHelp, "h", false, "Show help (short)")
	flag.BoolVar(&f.showVersion, "version", false, "Show version")
	flag.BoolVar(&f.showVersion, "v", false, "Show version (short)")

	// LLM behavior parameters
	flag.Float64Var(&f.temperature, "temperature", -1, "Temperature parameter (0.0 ~ 2.0, overrides config file)")
	flag.IntVar(&f.maxTokens, "max-tokens", -1, "Max output tokens (overrides config file)")
	flag.Float64Var(&f.topP, "top-p", -1, "Top-P sampling parameter (0.0 ~ 1.0, -1 not sent, overrides config file)")
	flag.IntVar(&f.topK, "top-k", -1, "Top-K sampling parameter (>= 1, -1 not sent, overrides config file)")
	flag.Float64Var(&f.repetitionPenalty, "repetition-penalty", -1, "Repetition penalty (0.0 ~ 2.0, -1 not sent, overrides config file)")
	flag.StringVar(&f.showLlmThinking, "show-llm-thinking", "", "Show LLM thinking content (on/off, overrides config file)")

	flag.StringVar(&f.showCommand, "show-command", "", "Show executed system command (on/off, overrides config file)")
	flag.StringVar(&f.showLlmContent, "show-llm-content", "", "Show LLM main content (on/off, overrides config file)")
	flag.StringVar(&f.showTool, "show-tool", "", "Show tool call names (on/off, overrides config file)")
	flag.StringVar(&f.showToolInput, "show-tool-input", "", "Show tool call input parameters (on/off, overrides config file)")
	flag.StringVar(&f.showToolOutput, "show-tool-output", "", "Show tool call return data (on/off, overrides config file)")
	flag.StringVar(&f.showParseErrorRaw, "show-parse-error-raw", "", "Show raw LLM message on tool call parse errors (on/off, overrides config file)")
	flag.StringVar(&f.showCommandOutput, "show-command-output", "", "Show command return data (on/off, overrides config file)")
	flag.StringVar(&f.outputCategories, "output-categories", "", "Output category switches (format: cat=on;cat2=off, e.g. bridge=off;subagent=off, overrides config)")

	flag.StringVar(&f.confirmTool, "confirm-tool", "", "Require confirmation before tool calls (on/off, overrides config file)")
	flag.StringVar(&f.resultMode, "result-mode", "", "Result processing mode (minimal/explain/analyze/free, overrides config file)")

	// Agent identity parameters
	flag.StringVar(&f.description, "description", "", "Set agent description/expertise (overrides config file)")

	// Vision support
	flag.StringVar(&f.vision, "vision", "", "Vision capability (on/off, overrides config file)")
	flag.StringVar(&f.visionContextMode, "vision-context-mode", "", "Vision model context mode (minimal/full, default minimal; minimal sends only system prompt + recognition instruction to avoid vision model context overflow)")

	// Memory enabled
	flag.StringVar(&f.memoryEnabled, "memory-enabled", "", "Enable persistent memory (overrides config file)")
	flag.StringVar(&f.memoryEnabled, "memory-disabled", "", "Disable persistent memory (overrides config file)")

	// Plan enabled
	flag.StringVar(&f.planEnabled, "plan-enabled", "", "Enable task plan (overrides config file)")
	flag.StringVar(&f.planEnabled, "plan-disabled", "", "Disable task plan (overrides config file)")

	// SubAgent enabled
	flag.StringVar(&f.subAgentEnabled, "subagent-enabled", "", "Enable sub-agent (overrides config file)")
	flag.StringVar(&f.subAgentEnabled, "subagent-disabled", "", "Disable sub-agent (overrides config file)")

	// ToolCall enabled
	flag.StringVar(&f.toolCallEnabled, "toolcall-enabled", "", "Enable tool calls (on/off, overrides config file)")
	flag.StringVar(&f.toolCallEnabled, "toolcall-disabled", "", "Disable tool calls (overrides config file)")

	// ToolCall mode (FEATURE-182)
	flag.StringVar(&f.toolCallMode, "toolcall-mode", "", "Tool call mode (openai/xml, overrides config file)")

	// Timeout parameters
	flag.IntVar(&f.toolTimeout, "tool-timeout", -1, "Tool call timeout seconds (0=unlimited, overrides config file)")
	flag.IntVar(&f.cmdTimeout, "cmd-timeout", -1, "System command timeout seconds (0=unlimited, overrides config file)")
	flag.IntVar(&f.llmTimeout, "llm-timeout", -1, "LLM API request timeout seconds (0=unlimited, overrides config file)")

	// Output mode
	flag.StringVar(&f.outputMode, "output-mode", "", "LLM frontend output mode (compact/normal/debug, overrides config file)")

	// Memory search config
	flag.IntVar(&f.memorySearchMaxContentLen, "memory-search-max-content-len", -1, "Memory search max content length (default 512, overrides config file)")
	flag.IntVar(&f.memorySearchMaxResults, "memory-search-max-results", -1, "Memory search max results (default 100, overrides config file)")

	// Error tracking config
	flag.IntVar(&f.errorMaxSingleCount, "error-max-single-count", -1, "Max occurrences of same error (default 10, overrides config file)")
	flag.IntVar(&f.errorMaxTypeCount, "error-max-type-count", -1, "Max distinct error types (default 100, overrides config file)")

	// Log level
	flag.StringVar(&f.logLevel, "log-level", "", "Log level (debug/info/warn/error/off, overrides config file)")

	// Emoji enabled
	flag.StringVar(&f.emojiEnabled, "emoji-enabled", "", "Enable emoji prefixes (on/off, overrides config file)")

	// Token usage display mode
	flag.StringVar(&f.tokenUsage, "token-usage", "", "Token usage display mode (on/off/none, overrides config file)")

	// Show logo on startup
	flag.StringVar(&f.showLogo, "show-logo", "", "Show startup logo (on/off, overrides config file)")

	// Session ID
	flag.StringVar(&f.sessionID, "session-id", "", "Set session ID, load existing session or create new (short -s)")
	flag.StringVar(&f.sessionID, "s", "", "Set session ID (short)")

	// Context start mode
	flag.StringVar(&f.contextPolicy, "context-policy", "", "Context policy (window/task/smart/reorganize, overrides config file)")

	// External config file generation
	flag.BoolVar(&f.initCapabilities, "unload-capabilities", false, "Export current system capabilities to CAPABILITIES.md in workspace root and exit")
	flag.BoolVar(&f.initRules, "unload-rules", false, "Export current system rules to RULES.md in workspace root and exit")
	flag.BoolVar(&f.unloadPrinciples, "unload-principles", false, "Export current system principles to PRINCIPLES.md in workspace root and exit")
	// Backward-compatible aliases (deprecated)
	flag.BoolVar(&f.unloadCapsAlias, "init-capabilities", false, "Deprecated, use --unload-capabilities")
	flag.BoolVar(&f.unloadRulesAlias, "init-rules", false, "Deprecated, use --unload-rules")

	// Loop intervention (FEATURE-267)
	flag.StringVar(&f.loopIntervention, "loop-intervention", "", "Loop intervention strategy (off/retry/prompt/reorganize/temperature/random, overrides config file)")

	// Loop temperature adjustment CLI overrides (FEATURE-230)
	flag.StringVar(&f.loopTempEnabled, "loop-temp-enabled", "", "Enable loop temperature auto-adjustment (on/off, overrides config file)")
	flag.Float64Var(&f.loopTempStepUp, "loop-temp-step-up", -1, "Loop temperature up step (0.01~1.0, overrides config file)")
	flag.Float64Var(&f.loopTempStepDown, "loop-temp-step-down", -1, "Loop temperature down step (0.01~1.0, overrides config file)")
	flag.Float64Var(&f.loopTempMax, "loop-temp-max", -1, "Loop temperature max (overrides config file)")
	flag.Float64Var(&f.loopTempMin, "loop-temp-min", -1, "Loop temperature min (overrides config file)")

	// Body additions: custom JSON properties to add to the LLM request body
	flag.StringVar(&f.bodyAdd, "body-add", "", "Add custom JSON property to LLM request body (format: key=value, repeatable, comma-separated)")

	// Input mode (FEATURE-198)
	flag.StringVar(&f.inputMode, "input-mode", "", "REPL input mode (enhanced=interactive/stdio=standard input, overrides config file)")

	// Unload mode (FEATURE-245)
	flag.StringVar(&f.unloadMode, "unload-mode", "", "Unload current mode sections to mode/<name>/ .md files")

	// Debug mode
	flag.StringVar(&f.debugMode, "debug", "", "Enable debug mode (preview and edit messages before sending to LLM)")

	// Work mode (FEATURE-288)
	flag.StringVar(&f.workMode, "mode", "", "Startup work mode (act/plan/research, overrides config file)")

	// Thinking enabled (FEATURE-288)
	flag.StringVar(&f.thinkingEnabled, "thinking-enabled", "", "Enable AI thinking (on/off/default, overrides config file)")

	// Reasoning effort (FEATURE-288)
	flag.StringVar(&f.reasoningEffort, "reasoning-effort", "", "Reasoning effort (low/medium/high/max/none/default, overrides config file)")

	// Max retries (FEATURE-288)
	flag.IntVar(&f.maxRetries, "max-retries", -1, "LLM transient error retries (default 3, overrides config file)")

	// Context limit (FEATURE-288)
	flag.IntVar(&f.contextLimit, "context-limit", -1, "Conversation context limit (-1=unlimited/0=no history/N=last N messages, overrides config file)")

	// Shell session enabled (FEATURE-288)
	flag.StringVar(&f.shellSessionEnabled, "shell-session-enabled", "", "Enable persistent shell session (on/off, overrides config file)")

	// Browser enabled (FEATURE-288)
	flag.StringVar(&f.browserEnabled, "browser-enabled", "", "Enable browser automation (on/off, overrides config file)")

	// Custom usage message
	flag.Usage = func() {
		agent.NewDefaultUserIO().ErrPrintf("%s", buildUsage(version, build))
	}

	flag.Parse()

	// Merge deprecated alias flags into the canonical fields
	if f.unloadCapsAlias {
		f.initCapabilities = true
	}
	if f.unloadRulesAlias {
		f.initRules = true
	}

	// If there are non-flag arguments and no explicit -c/--cmd, treat them as the command
	if f.command == "" && flag.NArg() > 0 {
		f.command = strings.Join(flag.Args(), " ")
	}

	return f
}

func main() {
	flags := parseFlags()
	io := agent.NewDefaultUserIO()

	// Initialize i18n before any user-facing output
	i18n.Init(flags.lang)

	// Handle --help
	if flags.showHelp {
		flag.Usage()
		os.Exit(0)
	}

	// Handle --version
	if flags.showVersion {
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
		io.Printf("co-shell v%s [BUILD-%s]%s\n", version, build, visionIndicator)
		os.Exit(0)
	}

	ws, err := workspace.New(flags.workspacePath)
	if err != nil {
		io.ErrPrintf("Error: cannot initialize workspace: %v\n", err)
		os.Exit(1)
	}

	// Change working directory to the workspace root so that all relative
	// path operations (command execution, file I/O, etc.) are scoped to the
	// workspace regardless of how the application was launched.
	if err := os.Chdir(ws.Root()); err != nil {
		io.ErrPrintf("Warning: cannot change to workspace directory: %v\n", err)
	}

	if flags.initCapabilities {
		ep := config.GetEmojiPrefixes(true)
		capPath := filepath.Join(ws.Root(), "CAPABILITIES.md")
		if _, err := os.Stat(capPath); err == nil {
			io.Printf("%s %s %s\n", ep.Warning, capPath, i18n.T(i18n.KeyFileExistsSkip))
			os.Exit(0)
		}
		content := i18n.T(i18n.KeySystemPromptCapabilities)
		if err := os.WriteFile(capPath, []byte(content), 0644); err != nil {
			io.ErrPrintf("Error: cannot write %s: %v\n", capPath, err)
			os.Exit(1)
		}
		io.Printf("%s %s %s\n", ep.Success, i18n.T(i18n.KeyGeneratedDefaultCAPABILITIES), capPath)
		os.Exit(0)
	}

	if flags.initRules {
		ep := config.GetEmojiPrefixes(true)
		rulesPath := filepath.Join(ws.Root(), "RULES.md")
		if _, err := os.Stat(rulesPath); err == nil {
			io.Printf("%s %s %s\n", ep.Warning, rulesPath, i18n.T(i18n.KeyFileExistsSkip))
			os.Exit(0)
		}
		content := i18n.T(i18n.KeySystemPromptRules)
		if err := os.WriteFile(rulesPath, []byte(content), 0644); err != nil {
			io.ErrPrintf("Error: cannot write %s: %v\n", rulesPath, err)
			os.Exit(1)
		}
		io.Printf("%s %s %s\n", ep.Success, i18n.T(i18n.KeyGeneratedDefaultRULES), rulesPath)
		os.Exit(0)
	}

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
		io.ErrPrintf("Warning: cannot load config: %v\n", err)
		cfg = config.DefaultConfig()
	}
	if configPath != "" {
		log.Info("Config loaded from: %s", configPath)
		// Set environment variable so sub-agent processes inherit the config path
		os.Setenv("CO_SHELL_CONFIG_PATH", configPath)
	}

	// Apply CLI overrides for model connection parameters.
	// These override the active model's fields if a model exists,
	// or will be used when creating the default model entry below.
	// The actual application happens when creating/updating the model entry.
	if flags.log != "" {
		switch flags.log {
		case "on", "1", "true", "yes":
			cfg.LogEnabled = true
		case "off", "0", "false", "no":
			cfg.LogEnabled = false
		default:
			io.ErrPrintf("Warning: invalid --log value %q, use on|off\n", flags.log)
		}
	}

	// Apply LLM behavior CLI overrides
	if flags.temperature >= 0 {
		cfg.LLM.Temperature = flags.temperature
	}
	if flags.maxTokens >= 0 {
		cfg.LLM.MaxTokens = flags.maxTokens
	}

	if flags.topP >= 0 {
		cfg.LLM.TopP = flags.topP
	}
	if flags.topK >= 0 {
		cfg.LLM.TopK = flags.topK
	}
	if flags.repetitionPenalty >= 0 {
		cfg.LLM.RepetitionPenalty = flags.repetitionPenalty
	}
	if flags.outputCategories != "" {
		if err := applyOutputCategoriesCLI(cfg, flags.outputCategories, io); err != nil {
			// Warning already printed; keep the rest of config as-is.
			_ = err
		}
	}
	if flags.showLlmThinking != "" {
		switch flags.showLlmThinking {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowLlmThinking = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowLlmThinking = false
		default:
			io.ErrPrintf("Warning: invalid --show-llm-thinking value %q, use on|off\n", flags.showLlmThinking)
		}
	}

	if flags.showCommand != "" {
		switch flags.showCommand {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowCommand = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowCommand = false
		default:
			io.ErrPrintf("Warning: invalid --show-command value %q, use on|off\n", flags.showCommand)
		}
	}
	if flags.showLlmContent != "" {
		switch flags.showLlmContent {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowLlmContent = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowLlmContent = false
		default:
			io.ErrPrintf("Warning: invalid --show-llm-content value %q, use on|off\n", flags.showLlmContent)
		}
	}
	if flags.showTool != "" {
		switch flags.showTool {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowTool = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowTool = false
		default:
			io.ErrPrintf("Warning: invalid --show-tool value %q, use on|off\n", flags.showTool)
		}
	}
	if flags.showToolInput != "" {
		switch flags.showToolInput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowToolInput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowToolInput = false
		default:
			io.ErrPrintf("Warning: invalid --show-tool-input value %q, use on|off\n", flags.showToolInput)
		}
	}
	if flags.showParseErrorRaw != "" {
		switch flags.showParseErrorRaw {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowParseErrorRaw = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowParseErrorRaw = false
		default:
			io.ErrPrintf("Warning: invalid --show-parse-error-raw value %q, use on|off\n", flags.showParseErrorRaw)
		}
	}
	if flags.showToolOutput != "" {
		switch flags.showToolOutput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowToolOutput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowToolOutput = false
		default:
			io.ErrPrintf("Warning: invalid --show-tool-output value %q, use on|off\n", flags.showToolOutput)
		}
	}
	if flags.showCommandOutput != "" {
		switch flags.showCommandOutput {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowCommandOutput = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowCommandOutput = false
		default:
			io.ErrPrintf("Warning: invalid --show-command-output value %q, use on|off\n", flags.showCommandOutput)
		}
	}

	if flags.confirmTool != "" {
		if cfg.LLM.ToolModes == nil {
			cfg.LLM.ToolModes = make(map[string]string)
		}
		switch flags.confirmTool {
		case "on", "1", "true", "yes":
			cfg.LLM.ToolModes["default"] = "confirm"
		case "off", "0", "false", "no":
			cfg.LLM.ToolModes["default"] = "auto"
		default:
			io.ErrPrintf("Warning: invalid --confirm-tool value %q, use on|off\n", flags.confirmTool)
		}
	}
	if flags.resultMode != "" {
		if mode, ok := config.ParseResultMode(flags.resultMode); ok {
			cfg.LLM.ResultMode = int(mode)
		} else {
			io.ErrPrintf("Warning: invalid --result-mode value %q, use minimal/explain/analyze/free\n", flags.resultMode)
		}
	}

	// Apply agent identity CLI overrides
	if flags.description != "" {
		cfg.LLM.AgentDescription = flags.description
	}

	// Apply vision CLI override
	if flags.vision != "" {
		switch flags.vision {
		case "on", "1", "true", "yes":
			cfg.LLM.VisionSupport = true
		case "off", "0", "false", "no":
			cfg.LLM.VisionSupport = false
		default:
			io.ErrPrintf("Warning: invalid --vision value %q, use on|off\n", flags.vision)
		}
	}

	// Apply vision-context-mode CLI override (FEATURE-319)
	if flags.visionContextMode != "" {
		switch flags.visionContextMode {
		case "minimal", "full":
			cfg.LLM.VisionContextMode = flags.visionContextMode
		default:
			io.ErrPrintf("Warning: invalid --vision-context-mode value %q, use minimal|full\n", flags.visionContextMode)
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
			io.ErrPrintf("Warning: invalid --memory-enabled value %q, use on|off\n", flags.memoryEnabled)
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
			io.ErrPrintf("Warning: invalid --plan-enabled value %q, use on|off\n", flags.planEnabled)
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
			io.ErrPrintf("Warning: invalid --subagent-enabled value %q, use on|off\n", flags.subAgentEnabled)
		}
	}

	// Apply toolcall-enabled CLI override
	if flags.toolCallEnabled != "" {
		switch flags.toolCallEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.ToolCallEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.ToolCallEnabled = false
		default:
			io.ErrPrintf("Warning: invalid --toolcall-enabled value %q, use on|off\n", flags.toolCallEnabled)
		}
	}

	// Apply toolcall-mode CLI override (FEATURE-182)
	if flags.toolCallMode != "" {
		switch flags.toolCallMode {
		case "openai", "xml":
			cfg.LLM.ToolCallMode = flags.toolCallMode
		default:
			io.ErrPrintf("Warning: invalid --toolcall-mode value %q, use openai|xml\n", flags.toolCallMode)
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
			io.ErrPrintf("Warning: invalid --emoji-enabled value %q, use on|off\n", flags.emojiEnabled)
		}
	}

	// Apply token-usage CLI override
	if flags.tokenUsage != "" {
		switch flags.tokenUsage {
		case "on", "off", "none":
			cfg.LLM.TokenUsage = flags.tokenUsage
		default:
			io.ErrPrintf("Warning: invalid --token-usage value %q, use on|off|none\n", flags.tokenUsage)
		}
	}

	// Apply body-add CLI override
	if flags.bodyAdd != "" {
		if cfg.LLM.BodyAdditions == nil {
			cfg.LLM.BodyAdditions = make(map[string]string)
		}
		// Parse comma-separated key=value pairs
		pairs := strings.Split(flags.bodyAdd, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				io.ErrPrintf("Warning: invalid --body-add format %q, use key=value\n", pair)
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key == "" {
				continue
			}
			cfg.LLM.BodyAdditions[key] = value
		}
	}

	// Apply show-logo CLI override
	if flags.showLogo != "" {
		switch flags.showLogo {
		case "on", "1", "true", "yes":
			cfg.LLM.ShowLogo = true
		case "off", "0", "false", "no":
			cfg.LLM.ShowLogo = false
		default:
			io.ErrPrintf("Warning: invalid --show-logo value %q, use on|off\n", flags.showLogo)
		}
	} else if flags.command != "" {
		// In single command mode, hide logo by default unless explicitly enabled
		cfg.LLM.ShowLogo = false
	}

	// Apply error tracking config CLI overrides
	if flags.errorMaxSingleCount >= 0 {
		cfg.LLM.ErrorMaxSingleCount = flags.errorMaxSingleCount
	}
	if flags.errorMaxTypeCount >= 0 {
		cfg.LLM.ErrorMaxTypeCount = flags.errorMaxTypeCount
	}

	// Apply context-policy CLI override
	if flags.contextPolicy != "" {
		switch flags.contextPolicy {
		case "window", "task", "smart", "reorganize":
			cfg.LLM.ContextPolicy = flags.contextPolicy
		default:
			io.ErrPrintf("Warning: invalid --context-policy value %q, use window/task/smart/reorganize\n", flags.contextPolicy)
		}
	}

	// Apply work-mode CLI override (FEATURE-288)
	if flags.workMode != "" {
		switch flags.workMode {
		case "act", "plan", "research":
			cfg.LLM.WorkMode = flags.workMode
		default:
			io.ErrPrintf("Warning: invalid --mode value %q, use act/plan/research\n", flags.workMode)
		}
	}

	// Apply thinking-enabled CLI override (FEATURE-288)
	if flags.thinkingEnabled != "" {
		switch flags.thinkingEnabled {
		case "on", "off", "default":
			cfg.LLM.ThinkingEnabled = flags.thinkingEnabled
		default:
			io.ErrPrintf("Warning: invalid --thinking-enabled value %q, use on/off/default\n", flags.thinkingEnabled)
		}
	}

	// Apply reasoning-effort CLI override (FEATURE-288)
	if flags.reasoningEffort != "" {
		switch flags.reasoningEffort {
		case "low", "medium", "high", "max", "none", "default":
			cfg.LLM.ReasoningEffort = flags.reasoningEffort
		default:
			io.ErrPrintf("Warning: invalid --reasoning-effort value %q, use low/medium/high/max/none/default\n", flags.reasoningEffort)
		}
	}

	// Apply max-retries CLI override (FEATURE-288)
	if flags.maxRetries >= 0 {
		cfg.LLM.MaxRetries = flags.maxRetries
	}

	// Apply context-limit CLI override (FEATURE-288)
	if flags.contextLimit >= -1 {
		cfg.LLM.ContextLimit = flags.contextLimit
	}

	// Apply shell-session-enabled CLI override (FEATURE-288)
	if flags.shellSessionEnabled != "" {
		switch flags.shellSessionEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.ShellSessionEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.ShellSessionEnabled = false
		default:
			io.ErrPrintf("Warning: invalid --shell-session-enabled value %q, use on|off\n", flags.shellSessionEnabled)
		}
	}

	// Apply browser-enabled CLI override (FEATURE-288)
	if flags.browserEnabled != "" {
		switch flags.browserEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.BrowserEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.BrowserEnabled = false
		default:
			io.ErrPrintf("Warning: invalid --browser-enabled value %q, use on|off\n", flags.browserEnabled)
		}
	}

	// Apply loop-intervention CLI override
	if flags.loopIntervention != "" {
		switch flags.loopIntervention {
		case "off", "retry", "prompt", "reorganize", "temperature", "random":
			cfg.LLM.LoopIntervention = flags.loopIntervention
		default:
			io.ErrPrintf("Warning: invalid --loop-intervention value %q, use off|retry|prompt|reorganize|temperature|random\n", flags.loopIntervention)
		}
	}

	// Apply loop temperature CLI overrides (FEATURE-230)
	if flags.loopTempEnabled != "" {
		switch flags.loopTempEnabled {
		case "on", "1", "true", "yes":
			cfg.LLM.LoopTempEnabled = true
		case "off", "0", "false", "no":
			cfg.LLM.LoopTempEnabled = false
		default:
			io.ErrPrintf("Warning: invalid --loop-temp-enabled value %q, use on|off\n", flags.loopTempEnabled)
		}
	}
	if flags.loopTempStepUp >= 0 {
		cfg.LLM.LoopTempStepUp = flags.loopTempStepUp
	}
	if flags.loopTempStepDown >= 0 {
		cfg.LLM.LoopTempStepDown = flags.loopTempStepDown
	}
	if flags.loopTempMax >= 0 {
		cfg.LLM.LoopTempMax = flags.loopTempMax
	}
	if flags.loopTempMin >= 0 {
		cfg.LLM.LoopTempMin = flags.loopTempMin
	}

	// Initialize logger with workspace
	if err := log.Init(cfg.LogEnabled, ws); err != nil {
		io.ErrPrintf("Warning: cannot initialize logger: %v\n", err)
	}
	defer log.Close()

	// Initialize LLM interaction log (always create file writer, enabled state from config)
	if err := log.InitLLMInteractionLog(ws); err != nil {
		io.ErrPrintf("Warning: cannot initialize LLM interaction log: %v\n", err)
	}
	log.SetLLMInteractionEnabled(cfg.LLM.LLMInteractionLog)
	defer log.CloseLLMInteractionLog()

	// Apply log level: CLI flag overrides config, config overrides default
	if flags.logLevel != "" {
		if level, ok := log.ParseLogLevel(flags.logLevel); ok {
			log.SetLevel(level)
		} else {
			io.ErrPrintf("Warning: invalid --log-level value %q, use debug/info/warn/error/off\n", flags.logLevel)
		}
	} else if cfg.LogLevel != "" {
		if level, ok := log.ParseLogLevel(cfg.LogLevel); ok {
			log.SetLevel(level)
		}
	}

	// Handle unload mode (FEATURE-245): export mode sections to disk and exit
	if flags.unloadMode != "" {
		if err := agent.UnloadModeSections(cfg, flags.unloadMode); err != nil {
			io.Print(i18n.TF(i18n.KeyUnloadModeFailed, err) + "\n")
			os.Exit(1)
		}
		// Count written files
		sectionNames := config.DefaultBuiltInSections()
		if cfg != nil {
			for _, wm := range cfg.WorkModes {
				if wm.Name == flags.unloadMode && len(wm.Sections) > 0 {
					sectionNames = wm.Sections
					break
				}
			}
		}
		if len(sectionNames) == 0 {
			// Check built-in modes
			for _, wm := range config.DefaultWorkModes() {
				if wm.Name == flags.unloadMode && len(wm.Sections) > 0 {
					sectionNames = wm.Sections
					break
				}
			}
		}
		io.Print(i18n.TF(i18n.KeyUnloadModeDone, flags.unloadMode, len(sectionNames)) + "\n")
		os.Exit(0)
	}

	// Handle --unload-principles (FEATURE-330): export the resolved system
	// principles to PRINCIPLES.md in the workspace root and exit.
	if flags.unloadPrinciples {
		ep := config.GetEmojiPrefixes(true)
		principlesPath := filepath.Join(ws.Root(), "PRINCIPLES.md")
		if _, err := os.Stat(principlesPath); err == nil {
			io.Printf("%s %s %s\n", ep.Warning, principlesPath, i18n.T(i18n.KeyFileExistsSkip))
			os.Exit(0)
		}
		content := agent.ResolveAgentPrinciples(cfg, ws.Root())
		if err := os.WriteFile(principlesPath, []byte(content), 0644); err != nil {
			io.ErrPrintf("Error: cannot write %s: %v\n", principlesPath, err)
			os.Exit(1)
		}
		io.Printf("%s %s %s\n", ep.Success, i18n.T(i18n.KeyGeneratedDefaultPrinciples), principlesPath)
		os.Exit(0)
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

	// Initialize persistent store (DualStore: bbolt + optional PG)
	s, err := store.NewStoreFromConfig(cfg, ws)
	if err != nil {
		log.Error("Cannot initialize store: %v", err)
		io.ErrPrintf("Error: cannot initialize store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Initialize model manager
	modelMgr := config.GetDefaultModelManager()

	// Sync cfg.Models to modelMgr so that selectModelForCall / GetActiveModel works
	for _, m := range cfg.Models {
		_ = modelMgr.AddModel(m) // ignore duplicate errors
	}

	// Check if we need to auto-select a vision-capable model based on --image flag
	if visionRequired := flags.imagePaths != ""; visionRequired {
		if activeModel := modelMgr.GetActiveModel(true); activeModel != nil {
			if activeModel.Capabilities.Vision {
				cfg.LLM.VisionSupport = true
				log.Info("Auto-selected vision model: %s", activeModel.ID)
			}
		}
	}

	mcpMgr := mcp.NewManager()
	defer mcpMgr.Close()

	// Connect to enabled MCP servers from config
	for _, serverCfg := range cfg.MCP.Servers {
		if serverCfg.Enabled {
			if err := mcpMgr.AddServer(serverCfg.Name, serverCfg.Command, serverCfg.Args); err != nil {
				log.Warn("Cannot connect to MCP server %q: %v", serverCfg.Name, err)
				io.ErrPrintf("Warning: cannot connect to MCP server %q: %v\n", serverCfg.Name, err)
			} else {
				log.Info("Connected to MCP server: %s", serverCfg.Name)
			}
		}
	}

	// Run model setup wizard if no models are configured
	wasModelsEmpty := len(cfg.Models) == 0
	if wasModelsEmpty {
		log.Info("No models configured, running model setup wizard")
		modelHandler := cmd.NewModelHandler(cfg, nil)
		if _, err := modelHandler.AddModelWizard(); err != nil {
			io.Println(i18n.T(i18n.KeySetupCancelled))
			os.Exit(1)
		}
	}

	// Sync cfg.Models to modelMgr so that selectModelForCall / GetActiveModel works
	// This must happen AFTER the setup wizard, as the wizard adds models to cfg.Models.
	for _, m := range cfg.Models {
		// Check if model already exists in modelMgr to avoid duplicate errors
		existing := modelMgr.GetModel(m.ID)
		if existing == nil {
			if err := modelMgr.AddModel(m); err != nil {
				log.Warn("Failed to add model %s to model manager: %v", m.ID, err)
			}
		}
	}

	log.Info("Model manager sync: cfg.Models count=%d, modelMgr models count=%d",
		len(cfg.Models), len(modelMgr.GetAllModels()))

	// Initialize LLM client using the highest priority enabled model's parameters.
	// This ensures the initial client uses the correct model-level settings
	// (endpoint, api_key, model, temperature, etc.) rather than the legacy
	// global cfg.LLM fields which may be stale or inconsistent.
	var llmClient llm.Client
	activeModel := modelMgr.GetActiveModel(false)
	log.Info("Model manager: %d models loaded, GetActiveModel returned: %v", len(modelMgr.GetAllModels()), activeModel != nil)
	if activeModel != nil {
		log.Info("Active model details: id=%s, enabled=%v, api_key='%s', endpoint=%s, model=%s",
			activeModel.ID, activeModel.Enabled, activeModel.APIKey, activeModel.Endpoint, activeModel.Model)
	}
	log.Info("cfg.Models count: %d", len(cfg.Models))
	for i, m := range cfg.Models {
		log.Info("  cfg.Models[%d]: id=%s, enabled=%v, api_key='%s'", i, m.ID, m.Enabled, m.APIKey)
	}
	if activeModel != nil && activeModel.APIKey != "" {
		// Resolve parameters: model-level takes precedence, fall back to global cfg.LLM
		temperature := cfg.LLM.Temperature
		if activeModel.Temperature != nil {
			temperature = *activeModel.Temperature
		}
		maxTokens := cfg.LLM.MaxTokens
		if activeModel.MaxTokens != nil {
			maxTokens = *activeModel.MaxTokens
		}
		thinkingEnabled := cfg.LLM.ThinkingEnabled == "on"
		reasoningEffort := cfg.LLM.ReasoningEffort
		if activeModel.ReasoningEffort != nil {
			reasoningEffort = *activeModel.ReasoningEffort
		}
		topP := cfg.LLM.TopP
		if activeModel.TopP != nil {
			topP = *activeModel.TopP
		}
		topK := cfg.LLM.TopK
		if activeModel.TopK != nil {
			topK = *activeModel.TopK
		}
		repetitionPenalty := cfg.LLM.RepetitionPenalty
		if activeModel.RepetitionPenalty != nil {
			repetitionPenalty = *activeModel.RepetitionPenalty
		}

		llmClient = llm.NewClient(
			activeModel.Endpoint,
			activeModel.APIKey,
			activeModel.Model,
			temperature,
			maxTokens,
			cfg.LLM.LLMTimeout,
		)
		llmClient.SetTopP(topP)
		llmClient.SetTopK(topK)
		llmClient.SetRepetitionPenalty(repetitionPenalty)
		llmClient.SetTokenUsage(cfg.LLM.TokenUsage)

		// Build body additions: cfg.BodyAdditions + thinking adapter + model custom params
		additions := make(map[string]string)
		if len(cfg.LLM.BodyAdditions) > 0 {
			for k, v := range cfg.LLM.BodyAdditions {
				additions[k] = v
			}
		}
		adapter := llm.GetThinkingAdapter(activeModel.Provider)
		thinkingAdditions := adapter.BuildAdditions(llm.ThinkingConfig{
			Mode:            llm.ThinkingModeFromString(cfg.LLM.ThinkingEnabled),
			ReasoningEffort: reasoningEffort,
		})
		for k, v := range thinkingAdditions {
			additions[k] = v
		}
		if len(activeModel.CustomParams) > 0 {
			for k, v := range activeModel.CustomParams {
				jsonBytes, err := json.Marshal(v)
				if err == nil {
					additions[k] = string(jsonBytes)
				}
			}
		}
		if len(additions) > 0 {
			llmClient.SetBodyAdditions(additions)
		}
		log.Info("LLM client initialized from model %s: endpoint=%s model=%s llm_timeout=%ds thinking=%v reasoning_effort=%s",
			activeModel.ID, activeModel.Endpoint, activeModel.Model, cfg.LLM.LLMTimeout, thinkingEnabled, reasoningEffort)
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
	ag.SetVaultStore(s.Vault())
	ag.SetModelManager(modelMgr)

	// Apply result mode BEFORE restoring session, because SetResultMode
	// resets a.messages to [{system}] which would destroy restored messages.
	ag.SetResultMode(config.ResultMode(cfg.LLM.ResultMode))

	// Restore previous session if available
	if ag.RestoreSession() {
		log.Info("Previous session restored from storage")
	}

	// Apply --session-id override: load existing named session or create a new one.
	if flags.sessionID != "" {
		entry, entryFound, err := s.LoadNamedSession(flags.sessionID)
		if err != nil {
			log.Warn("SessionID: LoadNamedSession(%q) error: %v", flags.sessionID, err)
		} else if entryFound && entry != nil && len(entry.Messages) > 0 {
			// Existing session found: load its messages
			var msgs []llm.Message
			if err := json.Unmarshal(entry.Messages, &msgs); err == nil && len(msgs) > 0 {
				systemPrompt := ""
				if len(msgs) > 0 && msgs[0].Role == "system" {
					systemPrompt = msgs[0].Content
				}
				ag.SetHistory(append([]llm.Message{{Role: "system", Content: systemPrompt}}, msgs...))
				log.Info("SessionID: loaded %d messages from session %q (%s)", len(msgs), flags.sessionID, entry.Title)
			}
		} else {
			// Session ID not found: start fresh, will create on first Persist
			ag.Reset()
			log.Info("SessionID: session %q not found, starting fresh", flags.sessionID)
		}
		ag.SetCurrentSessionID(flags.sessionID)
		if err := s.SaveCurrentSessionID(flags.sessionID); err != nil {
			log.Warn("SessionID: SaveCurrentSessionID error: %v", err)
		} else {
			log.Info("SessionID: current session set to %q", flags.sessionID)
		}
	}

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

	// Apply agent name: default to current working directory name if not set in config
	if cfg.LLM.AgentName == "" {
		cwd, _ := os.Getwd()
		if cwd != "" {
			cfg.LLM.AgentName = filepath.Base(cwd)
		}
	}
	// CLI flag overrides everything
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
	ag.SetBrowserEnabled(cfg.LLM.BrowserEnabled)

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

	// Apply tool mode settings from config
	ag.SyncToolModes(cfg)

	// Apply debug mode setting
	ag.SetDebugMode(cfg.LLM.DebugMode)

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

	// Apply persistent shell session enabled setting
	ag.SetShellEnabled(cfg.LLM.ShellSessionEnabled)

	// Apply tool call enabled setting
	ag.SetToolCallEnabled(cfg.LLM.ToolCallEnabled)

	// Apply tool call mode (FEATURE-182)
	toolCallMode := cfg.LLM.ToolCallMode
	if toolCallMode == "" {
		toolCallMode = "openai"
	}
	ag.SetToolCallMode(toolCallMode)

	// Set image paths for multimodal input if provided

	if flags.imagePaths != "" {
		// Check if the current model supports vision
		if !cfg.LLM.VisionSupport {
			ep := config.GetEmojiPrefixes(cfg.LLM.EmojiEnabled)
			io.ErrPrintf("%s %s\n", ep.Error, i18n.T(i18n.KeyVisionNotSupported))
			io.ErrPrintf("   %s\n", i18n.T(i18n.KeyVisionUseMultimodalModel))
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
	// Apply input mode setting
	// On Windows, always use stdio mode since raw terminal is not available.
	inputMode := "tui" // default interactive mode (P2: "enhanced" → "tui")
	if runtime.GOOS == "windows" {
		inputMode = "stdio"
	} else {
		if cfg.LLM.InputMode != "" {
			inputMode = config.NormalizeInputMode(cfg.LLM.InputMode)
		}
		if flags.inputMode != "" {
			inputMode = config.NormalizeInputMode(flags.inputMode)
		}
	}
	r.SetInputMode(inputMode)
	log.Info("REPL started (input mode: %s)", inputMode)
	if err := r.Run(); err != nil {
		log.Error("REPL error: %v", err)
		io.ErrPrintf("Error: %v\n", err)
		os.Exit(1)
	}
}

// showDisclaimer displays the risk disclaimer and prompts the user to accept.
// If accepted, it saves the config with DisclaimerAccepted=true.
// If declined, it exits the program.
func showDisclaimer(cfg *config.Config, ws *workspace.Workspace) {
	io := agent.NewDefaultUserIO()
	io.Println()
	io.Println(i18n.T(i18n.KeyDisclaimerTitle))
	io.Println()
	io.Println(i18n.T(i18n.KeyDisclaimerBody))
	io.Println()

	for {
		io.Print(i18n.T(i18n.KeyDisclaimerPrompt))
		response, _ := io.ReadLine()
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == i18n.T(i18n.KeyDisclaimerYes) || response == "yes" {
			cfg.DisclaimerAccepted = true
			if err := cfg.Save(); err != nil {
				log.Warn("Cannot save disclaimer acceptance: %v", err)
			}
			io.Println()
			return
		}

		if response == i18n.T(i18n.KeyDisclaimerNo) || response == "no" {
			io.Println(i18n.T(i18n.KeyDisclaimerRefused))
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

// renderSingleCmdEvent renders a single stream event for single-command mode.
// It is the exact body previously inlined in executeSingleCommand's callback,
// extracted verbatim for testability (golden baseline, UC-0006).
func renderSingleCmdEvent(io agent.UserIO, ep config.EmojiPrefixes, eventType string, content string) {
	// Delegate to the unified stream renderer (P2 merge). The signature is
	// preserved for the golden test baseline (render_single_cmd.golden).
	renderer := agent.NewStreamRenderer(io, ep, agent.StreamModeSingleCmd)
	renderer.Render(eventType, content)
}

// executeSingleCommand executes a single command (natural language or system command)
// and prints the result, then exits.
func executeSingleCommand(ag *agent.Agent, cfg *config.Config, input string) {
	log.Info("Single command mode: %s", input)

	ep := config.GetEmojiPrefixes(cfg.LLM.EmojiEnabled)
	io := ag.IO()
	if io == nil {
		io = agent.NewDefaultUserIO()
	}

	// Check if it's a direct system command
	if isDirectCommand(input) {
		// Direct system command
		if cfg.LLM.ShowCommand {
			io.Printf("$ %s\n", input)
		}
		output, err := ag.ExecuteCommandDirectly(input)
		if err != nil {
			io.Print(output)
			io.Printf("%s Error: %v\n", ep.Error, err)
			os.Exit(1)
		}
		if output != "" {
			io.Println(output)
		}
		return
	}

	// Natural language input - use agent with streaming output
	ctx := context.Background()
	_, err := ag.RunStream(ctx, input, func(eventType string, content string) {
		renderSingleCmdEvent(io, ep, eventType, content)
	})

	if err != nil {
		io.Printf("%s Error: %v\n", ep.Error, err)
		os.Exit(1)
	}
}

// isLLMConfigComplete checks whether the LLM configuration has all required fields.
// It checks if there is at least one enabled model with API key, endpoint, and model name.
func isLLMConfigComplete(cfg *config.Config) bool {
	activeModel := config.GetActiveModelFromConfig(cfg)
	if activeModel == nil {
		return false
	}
	return activeModel.APIKey != "" &&
		activeModel.Endpoint != "" &&
		activeModel.Model != ""
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

func (c *noopClient) TestToolCallSupport(ctx context.Context) bool {
	return false
}

func (c *noopClient) TestThinkingSupport(ctx context.Context) bool {
	return false
}

func (c *noopClient) SetThinkingEnabled(enabled bool) {}

func (c *noopClient) SetReasoningEffort(effort string) {}

func (c *noopClient) SetTopP(topP float64) {}

func (c *noopClient) SetTopK(topK int) {}

func (c *noopClient) SetRepetitionPenalty(penalty float64) {}

func (c *noopClient) SetTokenUsage(mode string) {}

func (c *noopClient) SetTemperature(temp float64)                  {}
func (c *noopClient) SetBodyAdditions(additions map[string]string) {}

func (c *noopClient) RemoveBodyAddition(key string) {}

func (c *noopClient) GetBodyAdditions() map[string]string { return nil }

func (c *noopClient) Close() error {
	return nil
}

// loadSchedulerEntries loads persisted scheduler entries from the store.
func loadSchedulerEntries(s *store.DualStore) ([]*scheduler.CronEntry, error) {
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

// applyOutputCategoriesCLI applies --output-categories CLI override.
// Format: "cat=on;cat2=off" (e.g. "bridge=off;subagent=off").
// Invalid categories/values print a warning to io and return an error.
func applyOutputCategoriesCLI(cfg *config.Config, spec string, io agent.UserIO) error {
	if cfg.LLM.OutputCategories == nil {
		cfg.LLM.OutputCategories = map[string]bool{}
		for _, c := range config.DefaultOutputCategories() {
			cfg.LLM.OutputCategories[c] = true
		}
	}
	valid := map[string]bool{}
	for _, c := range config.DefaultOutputCategories() {
		valid[c] = true
	}
	var firstErr error
	for _, pair := range strings.Split(spec, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			io.ErrPrintf("Warning: invalid --output-categories item %q, use cat=on|off\n", pair)
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid --output-categories item %q", pair)
			}
			continue
		}
		cat := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if !valid[cat] {
			io.ErrPrintf("Warning: unknown output category %q in --output-categories\n", cat)
			if firstErr == nil {
				firstErr = fmt.Errorf("unknown output category %q", cat)
			}
			continue
		}
		switch val {
		case "on", "1", "true", "yes":
			cfg.LLM.OutputCategories[cat] = true
		case "off", "0", "false", "no":
			cfg.LLM.OutputCategories[cat] = false
		default:
			io.ErrPrintf("Warning: invalid --output-categories value %q for %q, use on|off\n", val, cat)
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid --output-categories value %q for %q", val, cat)
			}
		}
	}
	return firstErr
}

// maskKey masks the API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
