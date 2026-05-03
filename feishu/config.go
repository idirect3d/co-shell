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

// Package feishu implements the Feishu (Lark) bridge for co-shell.
package feishu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/bridge"
)

// Config holds the Feishu bridge configuration.
type Config struct {
	AppID          string `json:"app_id"`
	AppSecret      string `json:"app_secret"`
	CoShellPath    string `json:"co_shell_path"`
	Workspace      string `json:"workspace"`
	CoShellCfgPath string `json:"co_shell_config_path"` // Path to co-shell's config.json
	Mode           string `json:"mode"`                 // sync / pool / preempt
	LogLevel       string `json:"log_level"`            // debug/info/warn/error/off
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	wd, _ := os.Getwd()
	return &Config{
		Workspace: wd,
		Mode:      "sync",
		LogLevel:  "info",
	}
}

// BridgeConfigPath returns the path to the bridge config file.
func (c *Config) BridgeConfigPath() string {
	return filepath.Join(c.Workspace, "feishu-bridge.json")
}

// Validate checks if the required fields are set.
func (c *Config) Validate() error {
	var missing []string
	if c.AppID == "" {
		missing = append(missing, "app-id")
	}
	if c.AppSecret == "" {
		missing = append(missing, "app-secret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required parameters: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Save persists the configuration to the bridge config file.
func (c *Config) Save() error {
	return bridge.SaveConfig(c.BridgeConfigPath(), c)
}

// Load loads the configuration from the bridge config file.
func (c *Config) Load() error {
	return bridge.LoadConfig(c.BridgeConfigPath(), c)
}

// MaskSecret returns a masked version of the app secret for display.
func (c *Config) MaskSecret() string {
	if len(c.AppSecret) <= 8 {
		return "****"
	}
	return c.AppSecret[:4] + "****" + c.AppSecret[len(c.AppSecret)-4:]
}
