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
//	--config       Config file path (default: ./hub.json)
//	--port         UDP port to listen on (default: 8080)
//	--co-shell-path Path to co-shell executable
//	--workspace    Base workspace directory
//	--help         Show help
//	--version      Show version
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
	port := flag.Int("port", 0, "UDP port to listen on (default: 8080)")
	coShellPath := flag.String("co-shell-path", "", "path to co-shell executable")
	workspace := flag.String("workspace", "", "base workspace directory")
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

	// Load config
	cfg, err := loadConfig(*configPath, port, coShellPath, workspace)
	if err != nil {
		log.Printf("Warning: %v", err)
		log.Println("Using default configuration")
	}

	// Create and run hub
	h, err := hub.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create hub: %v", err)
	}

	h.Run()
}

func loadConfig(path string, port *int, coShellPath *string, workspace *string) (*hub.HubConfig, error) {
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
	if *workspace != "" {
		cfg.Workspace = *workspace
	}

	return cfg, nil
}

func printUsage() {
	fmt.Println(`co-shell-hub v0.1.0 - 多 Agent 管理服务端

Usage:
  co-shell-hub [flags]

Flags:
  --config PATH        Config file path (default: ./hub.json)
  --port NUM           UDP port to listen on (default: 8080)
  --co-shell-path PATH Path to co-shell executable
  --workspace PATH     Base workspace directory
  --help               Show help
  --version            Show version

Config file (JSON):
  {
    "port": 8080,
    "co_shell_path": "co-shell",
    "workspace": ".",
    "agents": [
      {"id": "default", "name": "默认助手"},
      {"id": "research", "name": "研究助手"}
    ]
  }`)
}
