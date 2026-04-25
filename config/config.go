// Author: L.Shuang
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	Provider      string  `json:"provider"`
	APIKey        string  `json:"api_key"`
	Endpoint      string  `json:"endpoint"`
	Model         string  `json:"model"`
	Temperature   float64 `json:"temperature"`
	MaxTokens     int     `json:"max_tokens"`
	MaxIterations int     `json:"max_iterations"`
	ShowThinking  bool    `json:"show_thinking"`
	ShowCommand   bool    `json:"show_command"`
	ShowOutput    bool    `json:"show_output"`
}

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Servers []MCPServerConfig `json:"servers"`
}

// MCPServerConfig defines a single MCP server.
type MCPServerConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled bool     `json:"enabled"`
}

// Config is the top-level configuration structure.
type Config struct {
	LLM        LLMConfig `json:"llm"`
	MCP        MCPConfig `json:"mcp"`
	Rules      []string  `json:"rules"`
	LogEnabled bool      `json:"log_enabled"`
}

// DefaultConfig returns a Config with sensible defaults (DeepSeek, key empty).
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:     "deepseek",
			Endpoint:     "https://api.deepseek.com",
			Model:        "deepseek-v4-flash",
			Temperature:  0.7,
			MaxTokens:    393216,
			ShowThinking: true,
			ShowCommand:  true,
			ShowOutput:   true,
		},
		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
		Rules:      []string{},
		LogEnabled: true,
	}
}

// configPaths returns a list of config file paths to search, in priority order.
// Priority: 1. CLI-specified path  2. ./config.json  3. ~/.co-shell/config.json
func configPaths(cliPath string) []string {
	var paths []string

	// 1. CLI-specified path (highest priority)
	if cliPath != "" {
		paths = append(paths, cliPath)
	}

	// 2. Current directory config.json
	paths = append(paths, filepath.Join(".", "config.json"))

	// 3. Home directory ~/.co-shell/config.json
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".co-shell", "config.json"))
	}

	return paths
}

// defaultConfigPath returns the default config path (~/.co-shell/config.json).
// This is used for saving config and ensuring the directory exists.
func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	dir := filepath.Join(home, ".co-shell")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config from disk, searching multiple locations in priority order.
// Priority: CLI-specified path > ./config.json > ~/.co-shell/config.json
// If cliPath is empty, it will be skipped.
// Returns the loaded config and the path it was loaded from.
func LoadWithPath(cliPath string) (*Config, string, error) {
	paths := configPaths(cliPath)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("cannot read config %s: %w", path, err)
		}

		cfg := DefaultConfig()
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, "", fmt.Errorf("cannot parse config %s: %w", path, err)
		}
		return cfg, path, nil
	}

	// No config file found, return defaults
	return DefaultConfig(), "", nil
}

// Load reads the config from disk using default search paths.
// Equivalent to LoadWithPath("").
func Load() (*Config, error) {
	cfg, _, err := LoadWithPath("")
	return cfg, err
}

// Save writes the config to the default location (~/.co-shell/config.json).
func (c *Config) Save() error {
	path, err := defaultConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}
	return nil
}

// Show returns a human-readable representation of the config.
func (c *Config) Show() string {
	thinkingStatus := "on"
	if !c.LLM.ShowThinking {
		thinkingStatus = "off"
	}
	commandStatus := "on"
	if !c.LLM.ShowCommand {
		commandStatus = "off"
	}
	outputStatus := "on"
	if !c.LLM.ShowOutput {
		outputStatus = "off"
	}
	logStatus := "on"
	if !c.LogEnabled {
		logStatus = "off"
	}
	maxIterStr := fmt.Sprintf("%d", c.LLM.MaxIterations)
	if c.LLM.MaxIterations == -1 {
		maxIterStr = "unlimited"
	} else if c.LLM.MaxIterations == 0 {
		maxIterStr = "10 (default)"
	}
	providerName := c.LLM.Provider
	if providerName == "" {
		providerName = "custom"
	}
	return fmt.Sprintf(`LLM Configuration:
  Provider:        %s
  Endpoint:        %s
  Model:           %s
  Temperature:     %.1f
  Max Tokens:     %d
  Max Iterations:  %s
  Show Thinking:   %s
  Show Command:    %s
  Show Output:     %s

Logging: %s

MCP Servers: %d
Rules: %d`,
		providerName,
		c.LLM.Endpoint, c.LLM.Model, c.LLM.Temperature, c.LLM.MaxTokens,
		maxIterStr,
		thinkingStatus, commandStatus, outputStatus,
		logStatus,
		len(c.MCP.Servers), len(c.Rules))
}
