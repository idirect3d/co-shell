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

// Package bridge provides shared utilities for co-shell bridge programs.
package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds common configuration for bridge programs.
type Config struct {
	CoShellPath string `json:"co_shell_path"` // Path to co-shell executable; empty means find in PATH
	Workspace   string `json:"workspace"`     // Workspace path; empty means current directory
	ConfigPath  string `json:"config_path"`   // Config file path; empty means {workspace}/config.json
	LogLevel    string `json:"log_level"`     // Log level: debug/info/warn/error/off
}

// SaveConfig saves the bridge configuration to a JSON file.
func SaveConfig(path string, cfg interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}
	return nil
}

// LoadConfig loads the bridge configuration from a JSON file.
func LoadConfig(path string, cfg interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("cannot unmarshal config: %w", err)
	}
	return nil
}

// ResolveCoShellPath returns the path to the co-shell executable.
// If path is empty, it searches the PATH.
func ResolveCoShellPath(path string) (string, error) {
	if path != "" {
		// Verify the specified path exists
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("co-shell path %q does not exist: %w", path, err)
		}
		return path, nil
	}

	// Search PATH
	execPath, err := os.Executable()
	if err == nil {
		// Check if co-shell is in the same directory as the bridge
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, "co-shell")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fall back to PATH lookup
	candidate, err := findInPath("co-shell")
	if err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("co-shell not found in PATH and --co-shell-path not specified")
}

// findInPath searches for an executable in the PATH.
func findInPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH is empty")
	}

	dirs := filepath.SplitList(pathEnv)
	for _, dir := range dirs {
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() && (fi.Mode()&0111) != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}
