package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	Endpoint    string  `json:"endpoint"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// MCPConfig holds MCP server configuration.
type MCPConfig struct {
	Servers []MCPServerConfig `json:"servers"`
}

// MCPServerConfig defines a single MCP server.
type MCPServerConfig struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Args     []string `json:"args"`
	Enabled  bool   `json:"enabled"`
}

// Config is the top-level configuration structure.
type Config struct {
	LLM    LLMConfig  `json:"llm"`
	MCP    MCPConfig  `json:"mcp"`
	Rules  []string   `json:"rules"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Endpoint:    "https://api.openai.com/v1",
			Model:       "gpt-4o",
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
		Rules: []string{},
	}
}

// configPath returns the path to the config file.
func configPath() (string, error) {
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

// Load reads the config from disk, returning defaults if not found.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	path, err := configPath()
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
	return fmt.Sprintf(`LLM Configuration:
  Endpoint:    %s
  Model:       %s
  Temperature: %.1f
  Max Tokens:  %d

MCP Servers: %d
Rules: %d`,
		c.LLM.Endpoint, c.LLM.Model, c.LLM.Temperature, c.LLM.MaxTokens,
		len(c.MCP.Servers), len(c.Rules))
}
