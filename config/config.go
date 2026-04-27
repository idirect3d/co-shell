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
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/workspace"
)

// ResultMode defines how command execution results are presented to the user.
type ResultMode int

const (
	// ResultModeMinimal: return raw command output directly to the user, no LLM processing.
	ResultModeMinimal ResultMode = iota
	// ResultModeExplain: LLM explains the command output briefly.
	ResultModeExplain
	// ResultModeAnalyze: LLM performs deep analysis of the command output.
	ResultModeAnalyze
	// ResultModeFree: no specific instruction, LLM decides how to present results.
	ResultModeFree
)

// ResultModeString returns the string representation of a ResultMode.
func ResultModeString(m ResultMode) string {
	switch m {
	case ResultModeMinimal:
		return "minimal"
	case ResultModeExplain:
		return "explain"
	case ResultModeAnalyze:
		return "analyze"
	case ResultModeFree:
		return "free"
	default:
		return "minimal"
	}
}

// ParseResultMode parses a string into a ResultMode.
func ParseResultMode(s string) (ResultMode, bool) {
	switch s {
	case "minimal":
		return ResultModeMinimal, true
	case "explain":
		return ResultModeExplain, true
	case "analyze":
		return ResultModeAnalyze, true
	case "free":
		return ResultModeFree, true
	default:
		return ResultModeMinimal, false
	}
}

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	Provider       string  `json:"provider"`
	APIKey         string  `json:"api_key"`
	Endpoint       string  `json:"endpoint"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
	MaxIterations  int     `json:"max_iterations"`
	ShowThinking   bool    `json:"show_thinking"`
	ShowCommand    bool    `json:"show_command"`
	ShowOutput     bool    `json:"show_output"`
	ConfirmCommand bool    `json:"confirm_command"`
	ResultMode     int     `json:"result_mode"` // 0=minimal, 1=explain, 2=analyze, 3=free

	// Agent identity
	AgentName        string `json:"agent_name"`        // Agent name (default: co-shell)
	AgentDescription string `json:"agent_description"` // Agent expertise description
	AgentPrinciples  string `json:"agent_principles"`  // Agent core principles

	// Retry settings
	MaxRetries int `json:"max_retries"` // Max retries for transient LLM errors (default: 3)

	// Timeout settings (in seconds, 0 means no timeout)
	ToolTimeout         int `json:"tool_timeout"`          // Tool call timeout (default: 0 = no timeout)
	CommandTimeout      int `json:"command_timeout"`       // System command execution timeout (default: 0 = no timeout)
	LLMTimeout          int `json:"llm_timeout"`           // LLM API non-streaming request timeout (default: 0 = no timeout)
	EndpointTestTimeout int `json:"endpoint_test_timeout"` // Endpoint connectivity test timeout (default: 0 = no timeout)
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
	LLM                LLMConfig `json:"llm"`
	MCP                MCPConfig `json:"mcp"`
	Rules              []string  `json:"rules"`
	LogEnabled         bool      `json:"log_enabled"`
	DisclaimerAccepted bool      `json:"disclaimer_accepted"`

	ws         *workspace.Workspace // workspace reference for Save()
	configPath string               // actual config file path loaded from (may differ from ws.ConfigPath())
}

// DefaultConfig returns a Config with sensible defaults (DeepSeek, key empty).
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:       "deepseek",
			Endpoint:       "https://api.deepseek.com",
			Model:          "deepseek-v4-flash",
			Temperature:    0.7,
			MaxTokens:      393216,
			MaxIterations:  1000,
			ShowThinking:   true,
			ShowCommand:    true,
			ShowOutput:     true,
			ConfirmCommand: true,
			ResultMode:     int(ResultModeFree),
		},

		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
		Rules:      []string{},
		LogEnabled: true,
	}
}

// LoadWithPath reads the config from the workspace config.json.
// Returns the loaded config and the path it was loaded from.
func LoadWithPath(ws *workspace.Workspace) (*Config, string, error) {
	return LoadFromFile(ws.ConfigPath(), ws)
}

// LoadFromFile reads the config from a specific file path.
// If the file does not exist, returns a default config.
// Returns the loaded config and the path it was loaded from.
func LoadFromFile(path string, ws *workspace.Workspace) (*Config, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.ws = ws
			return cfg, "", nil
		}
		return nil, "", fmt.Errorf("cannot read config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	cfg.ws = ws
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, "", fmt.Errorf("cannot parse config %s: %w", path, err)
	}
	cfg.configPath = path
	return cfg, path, nil
}

// Load reads the config from disk using default search paths.
// Deprecated: Use LoadWithPath with a workspace instead.
func Load() (*Config, error) {
	return DefaultConfig(), nil
}

// Save writes the config to disk.
// If the config was loaded from a specific path (via -c/--config), it saves there.
// Otherwise, it saves to the workspace config.json.
func (c *Config) Save() error {
	path := c.configPath
	if path == "" {
		if c.ws == nil {
			return fmt.Errorf("workspace not set, cannot save config")
		}
		path = c.ws.ConfigPath()
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
// Two-column layout: parameter name (left) | value with label and range (right)
func (c *Config) Show() string {
	thinkingStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowThinking {
		thinkingStatus = i18n.T(i18n.KeyOff)
	}
	commandStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowCommand {
		commandStatus = i18n.T(i18n.KeyOff)
	}
	outputStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowOutput {
		outputStatus = i18n.T(i18n.KeyOff)
	}
	confirmStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ConfirmCommand {
		confirmStatus = i18n.T(i18n.KeyOff)
	}
	logStatus := i18n.T(i18n.KeyOn)
	if !c.LogEnabled {
		logStatus = i18n.T(i18n.KeyOff)
	}
	maxIterStr := fmt.Sprintf("%d", c.LLM.MaxIterations)
	if c.LLM.MaxIterations == -1 {
		maxIterStr = i18n.T(i18n.KeyUnlimited)
	} else if c.LLM.MaxIterations == 0 {
		maxIterStr = "1000 (" + i18n.T(i18n.KeyDefault) + ")"
	}

	providerName := c.LLM.Provider
	if providerName == "" {
		providerName = i18n.T(i18n.KeyCustom)
	}

	// Format timeout values
	toolTimeoutStr := fmt.Sprintf("%ds", c.LLM.ToolTimeout)
	if c.LLM.ToolTimeout <= 0 {
		toolTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	cmdTimeoutStr := fmt.Sprintf("%ds", c.LLM.CommandTimeout)
	if c.LLM.CommandTimeout <= 0 {
		cmdTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	llmTimeoutStr := fmt.Sprintf("%ds", c.LLM.LLMTimeout)
	if c.LLM.LLMTimeout <= 0 {
		llmTimeoutStr = i18n.T(i18n.KeyUnlimited)
	}

	// Mask API key
	maskedKey := "********"
	if c.LLM.APIKey != "" {
		if len(c.LLM.APIKey) <= 8 {
			maskedKey = "****"
		} else {
			maskedKey = c.LLM.APIKey[:4] + "****" + c.LLM.APIKey[len(c.LLM.APIKey)-4:]
		}
	}

	// Build three columns: param name | current value | (label, options/range)
	col3Provider := i18n.T(i18n.KeyCol3Provider)
	col3Endpoint := i18n.T(i18n.KeyCol3Endpoint)
	col3Model := i18n.T(i18n.KeyCol3Model)
	col3Temp := i18n.T(i18n.KeyCol3Temperature)
	col3MaxTokens := i18n.T(i18n.KeyCol3MaxTokens)
	col3MaxIter := i18n.T(i18n.KeyCol3MaxIter)
	col3Thinking := i18n.T(i18n.KeyCol3Thinking)
	col3Command := i18n.T(i18n.KeyCol3Command)
	col3Output := i18n.T(i18n.KeyCol3Output)
	col3Confirm := i18n.T(i18n.KeyCol3Confirm)
	col3ToolTimeout := i18n.T(i18n.KeyCol3ToolTimeout)
	col3CmdTimeout := i18n.T(i18n.KeyCol3CmdTimeout)
	col3LLMTimeout := i18n.T(i18n.KeyCol3LLMTimeout)
	col3Log := i18n.T(i18n.KeyCol3Log)
	col3ResultMode := i18n.T(i18n.KeyCol3ResultMode)
	col3MCP := ""
	col3Rules := ""
	col3APIKey := i18n.T(i18n.KeyCol3APIKey)
	col3Name := i18n.T(i18n.KeyCol3Name)
	col3Desc := i18n.T(i18n.KeyCol3Desc)
	col3Principles := i18n.T(i18n.KeyCol3Principles)

	resultModeStr := ResultModeString(ResultMode(c.LLM.ResultMode))

	agentName := c.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	agentDesc := c.LLM.AgentDescription
	if agentDesc == "" {
		agentDesc = "（未设置）"
	}
	agentPrinciples := c.LLM.AgentPrinciples
	if agentPrinciples == "" {
		agentPrinciples = "（未设置）"
	}

	return fmt.Sprintf(i18n.T(i18n.KeyConfigFormat),
		"provider:", providerName, col3Provider,
		"endpoint:", c.LLM.Endpoint, col3Endpoint,
		"model:", c.LLM.Model, col3Model,
		"temperature:", fmt.Sprintf("%.1f", c.LLM.Temperature), col3Temp,
		"max-tokens:", fmt.Sprintf("%d", c.LLM.MaxTokens), col3MaxTokens,
		"max-iterations:", maxIterStr, col3MaxIter,
		"show-thinking:", thinkingStatus, col3Thinking,
		"show-command:", commandStatus, col3Command,
		"show-output:", outputStatus, col3Output,
		"confirm-command:", confirmStatus, col3Confirm,
		"result-mode:", resultModeStr, col3ResultMode,
		"tool-timeout:", toolTimeoutStr, col3ToolTimeout,
		"cmd-timeout:", cmdTimeoutStr, col3CmdTimeout,
		"llm-timeout:", llmTimeoutStr, col3LLMTimeout,
		"log:", logStatus, col3Log,
		"name:", agentName, col3Name,
		"description:", agentDesc, col3Desc,
		"principles:", agentPrinciples, col3Principles,
		"MCP 服务器:", len(c.MCP.Servers), col3MCP,
		"规则:", len(c.Rules), col3Rules,
		"api-key:", maskedKey, col3APIKey)

}
