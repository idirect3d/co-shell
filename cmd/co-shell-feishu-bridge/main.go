// Author: L.Shuang
// Created: 2026-05-04
// Last Modified: 2026-05-04
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

// co-shell-feishu-bridge connects Feishu (Lark) to co-shell via WebSocket.
//
// Usage:
//
//	co-shell-feishu-bridge --app-id <APP_ID> --app-secret <APP_SECRET>
//
// Flags:
//
//	--app-id          Feishu App ID (required)
//	--app-secret      Feishu App Secret (required)
//	--co-shell-path   Path to co-shell executable (default: search PATH)
//	--workspace       Workspace path (default: current directory)
//	--config          Config file path (default: {workspace}/config.json)
//	--mode            Execution mode: sync/pool/preempt (default: sync)
//	--log-level       Log level: debug/info/warn/error/off (default: info)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/idirect3d/co-shell/bridge"
	"github.com/idirect3d/co-shell/feishu"
)

const version = "0.1.0"

// cliFlags holds parsed command-line flags.
type cliFlags struct {
	appID       string
	appSecret   string
	coShellPath string
	workspace   string
	configPath  string
	mode        string
	logLevel    string
	showHelp    bool
	showVersion bool
}

func parseFlags() cliFlags {
	var f cliFlags

	flag.StringVar(&f.appID, "app-id", "", "Feishu App ID (required)")
	flag.StringVar(&f.appSecret, "app-secret", "", "Feishu App Secret (required)")
	flag.StringVar(&f.coShellPath, "co-shell-path", "", "Path to co-shell executable (default: search PATH)")
	flag.StringVar(&f.workspace, "workspace", "", "co-shell workspace path (default: current directory)")
	flag.StringVar(&f.workspace, "w", "", "co-shell workspace path (short)")
	flag.StringVar(&f.configPath, "config", "", "co-shell config file path (default: {workspace}/config.json)")
	flag.StringVar(&f.configPath, "c", "", "co-shell config file path (short)")
	flag.StringVar(&f.mode, "mode", "sync", "Execution mode: sync / pool / preempt")
	flag.StringVar(&f.logLevel, "log-level", "info", "Log level: debug/info/warn/error/off")
	flag.BoolVar(&f.showHelp, "help", false, "Show help")
	flag.BoolVar(&f.showHelp, "h", false, "Show help (short)")
	flag.BoolVar(&f.showVersion, "version", false, "Show version")
	flag.BoolVar(&f.showVersion, "v", false, "Show version (short)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `co-shell-feishu-bridge v%s - Feishu Bridge

Connects a Feishu bot to co-shell via a WebSocket long connection,
receives Feishu messages, calls co-shell to process and reply.

Usage:
  co-shell-feishu-bridge [flags]

Required:
  --app-id <ID>        Feishu App ID
  --app-secret <KEY>   Feishu App Secret

Options:
  --co-shell-path <PATH>  Path to co-shell executable (default: search PATH)
  --workspace, -w <PATH>  co-shell workspace path (default: current directory)
  --config, -c <PATH>     co-shell config file path (default: {workspace}/config.json)
  --mode <MODE>           Execution mode: sync/pool/preempt (default: sync)
  --log-level <LEVEL>     Log level: debug/info/warn/error/off (default: info)
  --help, -h              Show help
  --version, -v           Show version

Execution modes:
  sync     Sync mode (default): execute one by one, next starts after current finishes
  pool     Pool mode: when current task finishes, merge all queued messages for batch processing
  preempt  Preempt mode: new message interrupts current process, executes new task immediately

Examples:
  co-shell-feishu-bridge --app-id cli_xxx --app-secret xxx
  co-shell-feishu-bridge --app-id cli_xxx --app-secret xxx --mode pool
  co-shell-feishu-bridge --app-id cli_xxx --app-secret xxx -w ./my-workspace
`, version)
	}

	flag.Parse()

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
		fmt.Printf("co-shell-feishu-bridge v%s\n", version)
		os.Exit(0)
	}

	// Load or create configuration
	cfg := loadConfig(flags)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  %v\n", err)
		fmt.Println()
		fmt.Println("Provide via command-line flags, or edit the config file manually:")
		fmt.Printf("  %s\n", cfg.BridgeConfigPath())
		fmt.Println()
		runSetupWizard(cfg)
	}

	// Resolve co-shell path
	coShellPath, err := bridge.ResolveCoShellPath(cfg.CoShellPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		fmt.Println("Specify the co-shell path via the --co-shell-path flag.")
		os.Exit(1)
	}
	log.Printf("Using co-shell: %s", coShellPath)

	// Ensure workspace exists
	if err := os.MkdirAll(cfg.Workspace, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create workspace directory: %v\n", err)
		os.Exit(1)
	}

	// Create executor
	executor := &bridge.Executor{
		CoShellPath: coShellPath,
		Workspace:   cfg.Workspace,
		ConfigPath:  cfg.CoShellCfgPath,
		Timeout:     120 * time.Second,
	}

	// Parse mode
	mode, ok := bridge.ParseMode(cfg.Mode)
	if !ok {
		fmt.Fprintf(os.Stderr, "⚠️  Invalid execution mode: %s, falling back to sync\n", cfg.Mode)
		mode = bridge.ModeSync
	}

	// Create global context for cancellation (Ctrl+C propagates to all subprocesses)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create scheduler with global context for cancellation
	scheduler := bridge.NewScheduler(ctx, mode, executor)

	// Create Feishu SDK client (for API calls like sending messages)
	larkClient := lark.NewClient(cfg.AppID, cfg.AppSecret)

	// Create handler
	handler := feishu.NewHandler(larkClient, scheduler, cfg.Workspace)

	// Create bridge (uses SDK's larkws.NewClient internally)
	feishuBridge := feishu.NewBridge(cfg, handler)

	// Print startup info
	fmt.Printf("🚀 co-shell-feishu-bridge v%s starting...\n", version)
	fmt.Printf("   Workspace: %s\n", cfg.Workspace)
	fmt.Printf("   Mode: %s\n", mode)
	fmt.Printf("   co-shell: %s\n", coShellPath)
	fmt.Println()

	// Start the bridge
	if err := feishuBridge.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Connected to Feishu, waiting for messages...")
	fmt.Println("   Press Ctrl+C to exit")
	fmt.Println()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println()
	fmt.Println("Shutting down...")

	feishuBridge.Stop()
	cancel()

	fmt.Println("✅ Exited safely")
}

// loadConfig loads the configuration from file or command-line flags.
func loadConfig(flags cliFlags) *feishu.Config {
	cfg := feishu.DefaultConfig()

	// Determine bridge config file path
	// Priority: --config flag > default ({workspace}/feishu-bridge.json)
	bridgeConfigPath := cfg.BridgeConfigPath()
	if flags.configPath != "" {
		if absPath, err := filepath.Abs(flags.configPath); err == nil {
			bridgeConfigPath = absPath
		} else {
			bridgeConfigPath = flags.configPath
		}
	}

	// Try to load existing bridge config
	if err := bridge.LoadConfig(bridgeConfigPath, cfg); err == nil {
		log.Printf("Loaded config from: %s", bridgeConfigPath)
	}

	// Apply CLI overrides
	if flags.appID != "" {
		cfg.AppID = flags.appID
	}
	if flags.appSecret != "" {
		cfg.AppSecret = flags.appSecret
	}
	if flags.coShellPath != "" {
		cfg.CoShellPath = flags.coShellPath
	}
	if flags.workspace != "" {
		cfg.Workspace = flags.workspace
	}
	if flags.configPath != "" {
		// --config flag specifies bridge config path, not co-shell config path
		// co-shell config path should be set in the bridge config file
	}
	if flags.mode != "" {
		cfg.Mode = flags.mode
	}
	if flags.logLevel != "" {
		cfg.LogLevel = flags.logLevel
	}

	// Resolve workspace to absolute path
	if absPath, err := filepath.Abs(cfg.Workspace); err == nil {
		cfg.Workspace = absPath
	}

	return cfg
}

// runSetupWizard prompts the user for missing configuration.
func runSetupWizard(cfg *feishu.Config) {
	fmt.Println("📌 Please complete the following configuration:")
	fmt.Println()

	// Prompt for App ID
	if cfg.AppID == "" {
		fmt.Print("Enter Feishu App ID: ")
		var input string
		fmt.Scanln(&input)
		cfg.AppID = strings.TrimSpace(input)
	}

	// Prompt for App Secret
	if cfg.AppSecret == "" {
		fmt.Print("Enter Feishu App Secret: ")
		var input string
		fmt.Scanln(&input)
		cfg.AppSecret = strings.TrimSpace(input)
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		log.Printf("Warning: cannot save config: %v", err)
	} else {
		fmt.Printf("✅ Configuration saved to: %s\n", cfg.BridgeConfigPath())
	}

	fmt.Println()
}
