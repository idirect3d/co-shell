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

	flag.StringVar(&f.appID, "app-id", "", "飞书应用 App ID（必填）")
	flag.StringVar(&f.appSecret, "app-secret", "", "飞书应用 App Secret（必填）")
	flag.StringVar(&f.coShellPath, "co-shell-path", "", "co-shell 可执行文件路径（默认：从 PATH 查找）")
	flag.StringVar(&f.workspace, "workspace", "", "co-shell 工作空间路径（默认：当前目录）")
	flag.StringVar(&f.workspace, "w", "", "co-shell 工作空间路径（简写）")
	flag.StringVar(&f.configPath, "config", "", "co-shell 配置文件路径（默认：{workspace}/config.json）")
	flag.StringVar(&f.configPath, "c", "", "co-shell 配置文件路径（简写）")
	flag.StringVar(&f.mode, "mode", "sync", "工作模式：sync（同步）/ pool（队列）/ preempt（抢占）")
	flag.StringVar(&f.logLevel, "log-level", "info", "日志级别：debug/info/warn/error/off")
	flag.BoolVar(&f.showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&f.showHelp, "h", false, "显示帮助信息（简写）")
	flag.BoolVar(&f.showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&f.showVersion, "v", false, "显示版本信息（简写）")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `co-shell-feishu-bridge v%s - 飞书桥接器

将飞书机器人连接到 co-shell，通过 WebSocket 长连接接收飞书消息，
调用 co-shell 处理并回复。

使用方式:
  co-shell-feishu-bridge [flags]

必需参数:
  --app-id <ID>        飞书应用 App ID
  --app-secret <KEY>   飞书应用 App Secret

可选参数:
  --co-shell-path <PATH>  co-shell 可执行文件路径（默认：从 PATH 查找）
  --workspace, -w <PATH>  co-shell 工作空间路径（默认：当前目录）
  --config, -c <PATH>     co-shell 配置文件路径（默认：{workspace}/config.json）
  --mode <MODE>           工作模式：sync/pool/preempt（默认：sync）
  --log-level <LEVEL>     日志级别：debug/info/warn/error/off（默认：info）
  --help, -h              显示帮助信息
  --version, -v           显示版本信息

工作模式说明:
  sync     同步模式（默认）：逐条执行，前一条完成后才执行下一条
  pool     队列模式：当前任务完成后，将队列中所有消息合并批量处理
  preempt  抢占模式：新消息中断当前进程，立即执行新任务

示例:
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
		fmt.Println("请通过命令行参数提供，或手动编辑配置文件：")
		fmt.Printf("  %s\n", cfg.BridgeConfigPath())
		fmt.Println()
		runSetupWizard(cfg)
	}

	// Resolve co-shell path
	coShellPath, err := bridge.ResolveCoShellPath(cfg.CoShellPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		fmt.Println("请通过 --co-shell-path 参数指定 co-shell 路径。")
		os.Exit(1)
	}
	log.Printf("Using co-shell: %s", coShellPath)

	// Ensure workspace exists
	if err := os.MkdirAll(cfg.Workspace, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 无法创建工作空间目录: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "⚠️  无效的工作模式: %s，使用默认模式 sync\n", cfg.Mode)
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
	fmt.Printf("🚀 co-shell-feishu-bridge v%s 启动中...\n", version)
	fmt.Printf("   工作空间: %s\n", cfg.Workspace)
	fmt.Printf("   工作模式: %s\n", mode)
	fmt.Printf("   co-shell: %s\n", coShellPath)
	fmt.Println()

	// Start the bridge
	if err := feishuBridge.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 已连接到飞书，等待消息...")
	fmt.Println("   按 Ctrl+C 退出")
	fmt.Println()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println()
	fmt.Println("正在关闭...")

	feishuBridge.Stop()
	cancel()

	fmt.Println("✅ 已安全退出")
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
	fmt.Println("📌 请完成以下配置：")
	fmt.Println()

	// Prompt for App ID
	if cfg.AppID == "" {
		fmt.Print("请输入飞书 App ID: ")
		var input string
		fmt.Scanln(&input)
		cfg.AppID = strings.TrimSpace(input)
	}

	// Prompt for App Secret
	if cfg.AppSecret == "" {
		fmt.Print("请输入飞书 App Secret: ")
		var input string
		fmt.Scanln(&input)
		cfg.AppSecret = strings.TrimSpace(input)
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		log.Printf("Warning: cannot save config: %v", err)
	} else {
		fmt.Printf("✅ 配置已保存到: %s\n", cfg.BridgeConfigPath())
	}

	fmt.Println()
}
