// Author: L.Shuang
// Created: 2026-05-17
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

// co-shell-hub is a service that manages multiple co-shell agent instances
// and handles UDP communication with mobile clients.
//
// Usage:
//
//	co-shell-hub [flags]
//
// Flags:
//
//	--config           Config file path (default: ./hub.json)
//	--port             UDP port to listen on (default: 12800)
//	--co-shell-path    Path to co-shell executable
//	--hub-workspace    Hub workspace directory (default: current directory)
//	--lazy-mode        Start agents on demand (default: true)
//	--start-all        Start all agents on hub startup
//	--gen-key          Generate a new Ed25519 key pair and exit
//	--help             Show help
//	--version          Show version
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/idirect3d/co-shell/hub"
)

const version = "0.1.0"

func main() {
	configPath := flag.String("config", "", "config file path (default: ./hub.json)")
	port := flag.Int("port", 0, "UDP port to listen on (default: 12800)")
	coShellPath := flag.String("co-shell-path", "", "path to co-shell executable")
	hubWorkspace := flag.String("hub-workspace", "", "hub workspace directory (default: current directory)")
	lazyMode := flag.Bool("lazy-mode", true, "start agents on demand when message received")
	startAll := flag.Bool("start-all", false, "start all agents on hub startup")
	genKey := flag.Bool("gen-key", false, "generate a new Ed25519 key pair and exit")
	showVersion := flag.Bool("version", false, "show version")
	showHelp := flag.Bool("help", false, "show help")
	flag.Parse()

	if *showVersion {
		fmt.Printf("co-shell-hub v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// Generate key pair and exit
	if *genKey {
		keyPair, err := hub.GenerateKeyPair()
		if err != nil {
			log.Fatalf("Failed to generate key pair: %v", err)
		}
		fmt.Println("=== Ed25519 Key Pair ===")
		fmt.Printf("Private key: %x\n", keyPair.PrivateKey)
		fmt.Printf("Public key:  %x\n", keyPair.PublicKey)
		fmt.Println("\nAdd to hub.json:")
		fmt.Printf(`"auth": {
  "hub_private_key": "%x",
  "mobile_public_key": ""
}`, keyPair.PrivateKey)
		os.Exit(0)
	}

	// Determine config path
	if *configPath == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Printf("Warning: cannot determine executable path: %v", err)
			*configPath = "./hub.json"
		} else {
			*configPath = filepath.Join(filepath.Dir(exe), "hub.json")
		}
	}

	// Load or generate auth
	auth, err := hub.LoadOrGenerateAuth(*configPath)
	if err != nil {
		log.Printf("Warning: cannot load auth config: %v", err)
		auth = &hub.AuthConfig{}
	}

	// Load config
	cfg, err := loadConfig(*configPath, port, coShellPath, hubWorkspace, lazyMode, startAll)
	if err != nil {
		log.Printf("Warning: %v", err)
		log.Println("Using default configuration")
		cfg = hub.DefaultConfig()
	}

	// Apply CLI overrides to default config as well
	if *port != 0 {
		cfg.Port = *port
	}
	if *coShellPath != "" {
		cfg.CoShellPath = *coShellPath
	}
	if *hubWorkspace != "" {
		cfg.Workspace = *hubWorkspace
	}
	if !*lazyMode {
		cfg.LazyMode = false
	}
	if *startAll {
		cfg.LazyMode = false
		for i := range cfg.Agents {
			cfg.Agents[i].AutoStart = true
		}
	}

	// Auto-discover agents from workspace subdirectories
	if len(cfg.Agents) == 0 {
		discovered := hub.DiscoverAgents(cfg.Workspace)
		if len(discovered) > 0 {
			log.Printf("Discovered %d agents from workspace", len(discovered))
			cfg.Agents = discovered
		}
	}

	// Save auth to config file first
	if err := auth.SaveAuth(*configPath); err != nil {
		log.Printf("Warning: cannot save auth config: %v", err)
	}

	// Save full config to file
	if err := hub.SaveConfig(*configPath, cfg); err != nil {
		log.Printf("Warning: cannot save config: %v", err)
	}

	// Create and run hub
	h, err := hub.New(cfg, auth)
	if err != nil {
		log.Fatalf("Failed to create hub: %v", err)
	}

	h.Run()
}

func loadConfig(path string, port *int, coShellPath *string, hubWorkspace *string, lazyMode *bool, startAll *bool) (*hub.HubConfig, error) {
	cfg, err := hub.LoadConfig(path)
	if err != nil {
		return nil, err
	}

	// Apply CLI overrides
	if *port != 0 {
		cfg.Port = *port
	}
	if *coShellPath != "" {
		cfg.CoShellPath = *coShellPath
	}
	if *hubWorkspace != "" {
		cfg.Workspace = *hubWorkspace
	}
	if !*lazyMode {
		cfg.LazyMode = false
	}
	if *startAll {
		cfg.LazyMode = false
		for i := range cfg.Agents {
			cfg.Agents[i].AutoStart = true
		}
	}

	return cfg, nil
}

func printUsage() {
	fmt.Println(`co-shell-hub v0.1.0 - 多 Agent 管理服务端

Usage:
  co-shell-hub [flags]

Flags:
  --config PATH           Config file path (default: ./hub.json)
  --port NUM              UDP port to listen on (default: 12800)
  --co-shell-path PATH    Path to co-shell executable
  --hub-workspace PATH    Hub workspace directory (default: current directory)
  --lazy-mode             Start agents on demand when message received (default: true)
  --start-all             Start all agents on hub startup
  --gen-key               Generate a new Ed25519 key pair and exit
  --help                  Show help
  --version               Show version

Config file (JSON):
  {
    "port": 12800,
    "co_shell_path": "co-shell",
    "workspace": ".",
    "lazy_mode": true,
    "auth": {
      "hub_private_key": "base64_encoded_private_key",
      "mobile_public_key": "base64_encoded_public_key"
    },
    "agents": [
      {"id": "default", "name": "默认助手"},
      {"id": "research", "name": "研究助手"}
    ]
  }`)
}
