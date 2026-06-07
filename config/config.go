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
	"strings"

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
// In the multi-model architecture, the actual model connection parameters
// (endpoint, api_key, model name) are stored in ModelConfig entries.
// The fields here serve as global overrides for model-level parameters.
type LLMConfig struct {
	Temperature   float64 `json:"temperature"`
	MaxTokens     int     `json:"max_tokens"`
	MaxIterations int     `json:"max_iterations"`
	// ToolModes stores per-tool mode settings.
	// Key is the tool name (e.g., "execute_command", "read_file", "write_to_file").
	// Value is one of: "disabled" (not sent to LLM), "confirm" (enabled, requires user confirmation),
	// "auto" (enabled, auto-approved without confirmation).
	// Use "default" key to set the default for all tools.
	// If a tool is not in the map, the default mode is "confirm".
	ToolModes  map[string]string `json:"tool_modes"`
	ResultMode int               `json:"result_mode"` // 0=minimal, 1=explain, 2=analyze, 3=free

	// Output control switches (ENHANCEMENT-126)
	ShowLlmThinking   bool `json:"show_llm_thinking"`   // Show LLM thinking content (default: true)
	ShowLlmContent    bool `json:"show_llm_content"`    // Show LLM main content (default: true)
	ShowTool          bool `json:"show_tool"`           // Show tool call name (default: true)
	ShowToolInput     bool `json:"show_tool_input"`     // Show tool call input parameters (default: false)
	ShowToolOutput    bool `json:"show_tool_output"`    // Show tool call return data (default: false)
	ShowCommand       bool `json:"show_command"`        // Show system command (default: true)
	ShowCommandOutput bool `json:"show_command_output"` // Show command return data (default: true)

	// Agent identity
	AgentName        string `json:"agent_name"`        // Agent name (default: co-shell)
	AgentDescription string `json:"agent_description"` // Agent expertise description
	AgentPrinciples  string `json:"agent_principles"`  // Agent core principles

	// User identity
	UserName string `json:"user_name"` // User name for LLM to identify different users (default: OS username)
	Channel  string `json:"channel"`   // Communication channel: co-shell, feishu, co-tor, agent (default: co-shell)

	// Vision support
	VisionSupport bool `json:"vision_support"` // Whether the model supports vision/multimodal input

	// Sampling parameters (ENHANCEMENT-140)
	// -1 means don't send the parameter to the API
	TopP              float64 `json:"top_p"`              // Top-p sampling (default: -1, don't send)
	TopK              int     `json:"top_k"`              // Top-k sampling (default: -1, don't send)
	RepetitionPenalty float64 `json:"repetition_penalty"` // Repetition penalty (default: -1, don't send)

	// Retry settings
	MaxRetries int `json:"max_retries"` // Max retries for transient LLM errors (default: 3)

	// Timeout settings (in seconds, 0 means no timeout)
	ToolTimeout         int `json:"tool_timeout"`          // Tool call timeout (default: 0 = no timeout)
	CommandTimeout      int `json:"command_timeout"`       // System command execution timeout (default: 0 = no timeout)
	LLMTimeout          int `json:"llm_timeout"`           // LLM API non-streaming request timeout (default: 0 = no timeout)
	EndpointTestTimeout int `json:"endpoint_test_timeout"` // Endpoint connectivity test timeout (default: 0 = no timeout)

	// ContextLimit: number of recent conversation messages to include in LLM context
	// 0 = no history auto-included, -1 = all messages, N = last N messages
	ContextLimit int `json:"context_limit"`

	// MemoryEnabled: whether persistent memory (get_history_slice, memory_search) is enabled
	MemoryEnabled bool `json:"memory_enabled"`

	// PlanEnabled: whether task plan tools (create_task_plan, etc.) are enabled
	PlanEnabled bool `json:"plan_enabled"`

	// SubAgentEnabled: whether sub-agent tools (launch_sub_agent) are enabled
	SubAgentEnabled bool `json:"sub_agent_enabled"`

	// ShellSessionEnabled: whether persistent shell session tools (shell_start, shell_exec, shell_stop) are enabled
	ShellSessionEnabled bool `json:"shell_session_enabled"`

	// ShellSessionTimeout: persistent shell command execution timeout in seconds (0 = no timeout)
	ShellSessionTimeout int `json:"shell_session_timeout"`

	// ShellVTRows: virtual terminal window rows (character height). Default: 24
	ShellVTRows int `json:"shell_vt_rows"`
	// ShellVTCols: virtual terminal window columns (character width). Default: 80
	ShellVTCols int `json:"shell_vt_cols"`

	// ListMaxItems: maximum number of items to return in list_files results.
	// Default: 100
	ListMaxItems int `json:"list_max_items"`

	// SearchMaxLineLength: maximum character length for a single line in search results
	// Lines longer than this will be truncated. Default: 8192
	SearchMaxLineLength int `json:"search_max_line_length"`

	// SearchMaxResultBytes: maximum total bytes for search results
	// Results exceeding this will be truncated. Default: 65536
	SearchMaxResultBytes int `json:"search_max_result_bytes"`

	// SearchContextLines: number of context lines before and after each match in search results
	// Default: 5
	SearchContextLines int `json:"search_context_lines"`

	// MemorySearchMaxContentLen: maximum character length for content in memory search results.
	// Content longer than this will be truncated with "...". Default: 512
	MemorySearchMaxContentLen int `json:"memory_search_max_content_len"`

	// MemorySearchMaxResults: maximum number of results returned by memory search. Default: 100
	MemorySearchMaxResults int `json:"memory_search_max_results"`

	// ErrorMaxSingleCount: maximum count for a single error type before prompting user.
	// When the same error message appears this many times, the user is prompted.
	// Default: 10
	ErrorMaxSingleCount int `json:"error_max_single_count"`

	// ErrorMaxTypeCount: maximum number of distinct error types before prompting user.
	// When the number of unique error messages exceeds this, the user is prompted.
	// Default: 100
	ErrorMaxTypeCount int `json:"error_max_type_count"`

	// LoopDetectEnabled: whether to enable LLM output loop detection.
	// When enabled, the agent monitors LLM output for repeating patterns
	// and intervenes if a loop is detected.
	// Default: true
	LoopDetectEnabled bool `json:"loop_detect_enabled"`

	// LoopDetectThreshold: the maximum number of times a similar content block
	// can repeat before triggering loop detection intervention.
	// When the same (or similar) content repeats this many times consecutively,
	// the agent will pause and prompt for intervention.
	// Default: 5
	LoopDetectThreshold int `json:"loop_detect_threshold"`

	// LoopDetectMaxWindow: the sliding window size (in content chunks) to check
	// for repetition patterns. The detector looks at the last N chunks to find
	// repeating patterns.
	// Default: 20
	LoopDetectMaxWindow int `json:"loop_detect_max_window"`

	// DedupEnabled: whether to enable message deduplication checking.
	// When enabled, before adding a message to the session, the system
	// extracts keywords and searches existing messages for duplicates.
	// If a highly similar message is found, it counts as a duplicate.
	// Default: true
	DedupEnabled bool `json:"dedup_enabled"`

	// DedupFeatureRatio: the ratio of words to extract as features from each message.
	// For example, 0.2 means 20% of the words in the message will be used as features.
	// Default: 0.2
	DedupFeatureRatio float64 `json:"dedup_feature_ratio"`

	// DedupMatchRatio: the minimum ratio of feature words that must match
	// in order to proceed to full similarity calculation.
	// Range: 0.0 ~ 1.0
	// Default: 0.6
	DedupMatchRatio float64 `json:"dedup_match_ratio"`

	// DedupSimilarityThreshold: the minimum Jaccard similarity score (0-100)
	// to consider two messages as duplicates.
	// Default: 85
	DedupSimilarityThreshold int `json:"dedup_similarity_threshold"`

	// DedupMaxHistory: maximum number of recent messages to check against
	// for deduplication. Limiting this prevents performance issues with
	// very long conversations.
	// Default: 50
	DedupMaxHistory int `json:"dedup_max_history"`

	// DedupRepeatLimit: the maximum number of duplicate messages allowed
	// before triggering an intervention warning.
	// Default: 3
	DedupRepeatLimit int `json:"dedup_repeat_limit"`

	// ThinkingEnabled: whether to enable LLM thinking/reasoning mode.
	// When enabled, the LLM API request includes thinking configuration
	// (e.g., DeepSeek thinking mode, OpenAI reasoning_effort).
	// Default: true
	ThinkingEnabled bool `json:"thinking_enabled"`

	// ReasoningEffort: the reasoning effort level for models that support it.
	// Valid values: "low", "medium", "high" (model-dependent).
	// Default: "high"
	ReasoningEffort string `json:"reasoning_effort"`

	// EmojiEnabled: whether to use emoji prefixes to distinguish different roles' output.
	// When enabled, user input is prefixed with 👤>, LLM output with 💬>,
	// tool call input/output with ⚙️</⚙️>, command input/output with 🔴</🔴>.
	// Default: true
	EmojiEnabled bool `json:"emoji_enabled"`

	// ShowLogo: whether to display the ASCII art logo on startup.
	// Default: true
	ShowLogo bool `json:"show_logo"`

	// ToolCallEnabled: whether tool/function calling is enabled for the LLM.
	// When disabled, the LLM operates in pure text mode without tool definitions.
	// Default: true
	ToolCallEnabled bool `json:"tool_call_enabled"`

	// MaxModelLen: the maximum context length (in tokens) supported by the model.
	// This value is automatically detected from the API when listing models.
	// A value of 0 means unknown or not yet detected.
	MaxModelLen int `json:"max_model_len"`

	// TokenUsage: controls token usage display and stream_options.include_usage.
	// "on" = display token usage and send include_usage=true
	// "off" = don't display token usage but still send include_usage=true
	// "none" = don't display token usage and don't send include_usage
	TokenUsage string `json:"token_usage"`

	// BodyAdditions: custom JSON properties to add to the LLM request body.
	// Each entry is a key-value pair where the key is the property name and
	// the value is a JSON string that will be merged into the request body.
	BodyAdditions map[string]string `json:"body_additions"`

	// ContextStartMode: controls how the context start position is managed.
	// "window" = fixed window: context is the last N messages (N = context_limit)
	// "task" = task mode: context start pointer follows task boundaries automatically
	// "smart" = smart mode: allow LLM to adjust context start via adjust_context_start tool
	// Default: "task" (backward compatible with current behavior)
	ContextStartMode string `json:"context_start_mode"`

	// ToolCallMode: the tool call mode to use.
	// "openai" = standard OpenAI API tool call mechanism (send tools as JSON array)
	// "xml" = custom XML format embedded in system prompt (no tools parameter sent)
	// Default: "xml"
	ToolCallMode string `json:"tool_call_mode"`

	// ToolCallModeSystemPrompts stores custom system prompt sections per tool call mode.
	// Key is the tool call mode type (e.g., "openai", "xml").
	// Value is the custom system prompt text for that mode's tool usage section.
	// If empty, the built-in i18n prompt is used.
	ToolCallModeSystemPrompts map[string]string `json:"tool_call_mode_system_prompts"`

	// WorkMode: the name of the currently active work mode.
	WorkMode string `json:"work_mode"`

	// InputMode: the REPL input mode.
	// "enhanced" = raw terminal mode with history navigation and proper multi-byte backspace (default)
	// "stdio" = standard line-buffered input via bufio.Scanner
	InputMode string `json:"input_mode"`

	// BrowserEnabled: whether browser automation tools are enabled.
	BrowserEnabled bool `json:"browser_enabled"`
	// BrowserPort: Chrome DevTools Protocol debug port.
	BrowserPort int `json:"browser_port"`
	// BrowserHeadless: whether to run Chrome in headless mode.
	BrowserHeadless bool `json:"browser_headless"`

	// BrowserMaxHTMLSize: maximum HTML content size (in bytes) before saving to file.
	// When browser_get_html returns HTML larger than this, the content is saved to
	// ./download/html/ and the file path is returned instead.
	// Default: 10240 (10KB)
	BrowserMaxHTMLSize int `json:"browser_max_html_size"`

	// ReadFileMaxSize: maximum total bytes returned by read_file.
	// When the output content exceeds this limit, it is truncated and a notice
	// is prepended. 0 means no limit.
	// Default: 81920 (80KB)
	ReadFileMaxSize int `json:"read_file_max_size"`
}

// EmojiPrefixes defines the emoji prefixes for different output roles.
// When emoji is enabled, uses emoji symbols; when disabled, uses i18n text labels.
type EmojiPrefixes struct {
	UserInput       string // 👤 >  or i18n key "emoji_prefix_user"
	VisionUserInput string // 👀 >  or i18n key "emoji_prefix_vision_user" (replaces UserInput when vision is supported)
	LlmOutput       string // 🐚 >  or i18n key "emoji_prefix_assistant"
	ToolCallInput   string // ⚙️ <  or i18n key "emoji_prefix_tool_input"
	ToolCallOutput  string // ⚙️ >  or i18n key "emoji_prefix_tool_output"
	CommandInput    string // 🔴 <  or i18n key "emoji_prefix_cmd_input"
	CommandOutput   string // 🔴 >  or i18n key "emoji_prefix_cmd_output"
	Info            string // ℹ️   or i18n key "emoji_prefix_info"
	Error           string // ❌   or i18n key "emoji_prefix_error"
	Warning         string // ⚠️   or i18n key "emoji_prefix_warning"
	Success         string // ✅   or i18n key "emoji_prefix_success"
	Thinking        string // 💬   or i18n key "emoji_prefix_thinking"
	OutputTitle     string // 📋   or i18n key "emoji_prefix_output_title"
	OutputSep       string // ───  or i18n key "emoji_prefix_output_sep"
}

// GetEmojiPrefixes returns the emoji prefixes based on whether emoji is enabled.
// When enabled, returns emoji symbols directly.
// When disabled, returns plain text labels (e.g., [user], [assistant], etc.).
func GetEmojiPrefixes(enabled bool) EmojiPrefixes {
	if !enabled {
		return EmojiPrefixes{
			UserInput:       "[user]> ",
			VisionUserInput: "[vision]> ",
			LlmOutput:       "[assistant]> ",
			ToolCallInput:   "[tool]< ",
			ToolCallOutput:  "[tool]> ",
			CommandInput:    "[cmd]< ",
			CommandOutput:   "[cmd]> ",
			Info:            "[info] ",
			Error:           "[error] ",
			Warning:         "[warn] ",
			Success:         "[ok] ",
			Thinking:        "[think] ",
			OutputTitle:     "[output] Command Output:",
			OutputSep:       "────────────────────────────────────────────",
		}
	}
	return EmojiPrefixes{
		UserInput:       "[👤]> ",
		VisionUserInput: "[👀]> ",
		LlmOutput:       "[🐚]> ",
		ToolCallInput:   "[⚙️]< ",
		ToolCallOutput:  "[⚙️]> ",
		CommandInput:    "[🔴]< ",
		CommandOutput:   "[🔴]> ",
		Info:            "[ℹ️] ",
		Error:           "[❌] ",
		Warning:         "[⚠️] ",
		Success:         "[✅] ",
		Thinking:        "[💬] ",
		OutputTitle:     "[📋] Command Output:",
		OutputSep:       "────────────────────────────────────────────",
	}
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

// DBConfig holds PostgreSQL database connection configuration.
type DBConfig struct {
	// Enabled enables PostgreSQL storage. When enabled, co-shell will attempt
	// to connect to the PostgreSQL database. If the connection fails, it will
	// fall back to local bbolt storage with a warning.
	Enabled bool `json:"enabled"`

	// Host is the PostgreSQL server hostname or IP address.
	// Default: "localhost"
	Host string `json:"host"`

	// Port is the PostgreSQL server port.
	// Default: 5432 (standard PostgreSQL port)
	Port int `json:"port"`

	// DBName is the PostgreSQL database name.
	// Default: "coshell_db"
	DBName string `json:"db_name"`

	// Schema is the PostgreSQL schema to use.
	// Default: "public"
	Schema string `json:"schema"`

	// User is the PostgreSQL user for authentication.
	// Default: "postgres"
	User string `json:"user"`

	// Password is the PostgreSQL password for authentication.
	Password string `json:"password"`
}

// DefaultDBConfig returns a DBConfig with sensible defaults.
func DefaultDBConfig() DBConfig {
	return DBConfig{
		Enabled: false,
		Host:    "localhost",
		Port:    5432,
		DBName:  "coshell_db",
		Schema:  "public",
		User:    "postgres",
	}
}

// PromptSection defines a named section of the system prompt that can be
// customized by the user. Each section has a unique name and content source.
// The content is loaded from an external .md file in the workspace root
// (named {Name}.md) if it exists, or from the Content string if non-empty,
// or falls back to the built-in i18n resource for built-in section names.
type PromptSection struct {
	// Name is the unique identifier for this section (e.g., "Identity", "CustomRules").
	Name string `json:"name"`
	// Content is the inline content override. If empty, the section tries to load
	// from {Name}.md in the workspace root, then falls back to i18n.
	Content string `json:"content,omitempty"`
	// BuiltIn indicates this is a system-defined section (not user-created).
	BuiltIn bool `json:"built_in"`
}

// WorkMode defines a named work mode that specifies which prompt sections
// to include and in what order.
type WorkMode struct {
	// Name is the unique identifier for this work mode (e.g., "default", "coding").
	Name string `json:"name"`
	// Description provides a human-readable summary of this mode.
	Description string `json:"description,omitempty"`
	// Sections lists the names of PromptSection entries to assemble, in order.
	Sections []string `json:"sections"`
}

// DefaultBuiltInSections returns the default list of built-in prompt section names
// in their standard assembly order.
func DefaultBuiltInSections() []string {
	return []string{
		"Identity",
		"ToolUsage",
		"ResultMode",
		"Capabilities",
		"Rules",
		"ExternalTools",
		"Environment",
		"Objective",
	}
}

// DefaultWorkModes returns a map of default work modes with standard configuration.
func DefaultWorkModes() []WorkMode {
	return []WorkMode{
		{
			Name:        "default",
			Description: "默认工作模式，包含所有标准提示词节",
			Sections:    DefaultBuiltInSections(),
		},
	}
}

// Config is the top-level configuration structure.
type Config struct {
	LLM                LLMConfig `json:"llm"`
	MCP                MCPConfig `json:"mcp"`
	DB                 DBConfig  `json:"db"`
	Rules              []string  `json:"rules"`
	LogEnabled         bool      `json:"log_enabled"`
	LogLevel           string    `json:"log_level"` // debug/info/warn/error/off
	DisclaimerAccepted bool      `json:"disclaimer_accepted"`
	// Models stores multiple model configurations for switching.
	Models []*ModelConfig `json:"models,omitempty"`

	// PromptSections stores user-defined and built-in prompt sections.
	PromptSections []PromptSection `json:"prompt_sections,omitempty"`
	// WorkModes stores user-defined work modes.
	WorkModes []WorkMode `json:"work_modes,omitempty"`

	ws         *workspace.Workspace // workspace reference for Save()
	configPath string               // actual config file path loaded from (may differ from ws.ConfigPath())
}

// DefaultConfig returns a Config with sensible defaults (DeepSeek, key empty).
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Temperature:               0.5,
			MaxTokens:                 -1,
			MaxIterations:             1000,
			ShowLlmThinking:           true,
			ShowLlmContent:            true,
			ShowTool:                  true,
			ShowToolInput:             false,
			ShowToolOutput:            false,
			ShowCommand:               true,
			ShowCommandOutput:         true,
			ToolModes:                 nil, // nil means "custom" mode: each tool uses its own default from DefaultToolModes()
			ResultMode:                int(ResultModeFree),
			ContextLimit:              -1, // -1 = 所有消息；0 = 不自动包含历史消息，LLM 需通过记忆工具获取；N = 最近 N 条
			MemoryEnabled:             true,
			PlanEnabled:               true,
			SubAgentEnabled:           true,
			ShellSessionEnabled:       true,
			ShellSessionTimeout:       0,
			ShellVTRows:               24,
			ShellVTCols:               80,
			ListMaxItems:              256,
			SearchMaxLineLength:       8192,
			SearchMaxResultBytes:      65536,
			SearchContextLines:        5,
			MemorySearchMaxContentLen: 512,
			MemorySearchMaxResults:    100,
			ErrorMaxSingleCount:       10,
			ErrorMaxTypeCount:         100,
			LoopDetectEnabled:         true,
			LoopDetectThreshold:       5,
			LoopDetectMaxWindow:       256,
			DedupEnabled:              true,
			DedupFeatureRatio:         0.2,
			DedupMatchRatio:           0.6,
			DedupSimilarityThreshold:  85,
			DedupMaxHistory:           50,
			DedupRepeatLimit:          3,
			TopP:                      -1,
			TopK:                      -1,
			RepetitionPenalty:         -1,
			ThinkingEnabled:           false,
			ReasoningEffort:           "low",
			EmojiEnabled:              true,
			ShowLogo:                  true,
			ToolCallEnabled:           true,
			ToolCallMode:              "xml",
			TokenUsage:                "on",
			InputMode:                 "enhanced",
			BrowserEnabled:            false,
			BrowserPort:               9222,
			BrowserHeadless:           false,
		},

		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
		Rules:      []string{},
		LogEnabled: true,
		LogLevel:   "info",
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
	llmThinkingStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowLlmThinking {
		llmThinkingStatus = i18n.T(i18n.KeyOff)
	}
	llmContentStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowLlmContent {
		llmContentStatus = i18n.T(i18n.KeyOff)
	}
	toolStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowTool {
		toolStatus = i18n.T(i18n.KeyOff)
	}
	toolInputStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowToolInput {
		toolInputStatus = i18n.T(i18n.KeyOff)
	}
	toolOutputStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowToolOutput {
		toolOutputStatus = i18n.T(i18n.KeyOff)
	}
	commandStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowCommand {
		commandStatus = i18n.T(i18n.KeyOff)
	}
	commandOutputStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ShowCommandOutput {
		commandOutputStatus = i18n.T(i18n.KeyOff)
	}
	thinkingEnabledStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ThinkingEnabled {
		thinkingEnabledStatus = i18n.T(i18n.KeyOff)
	}
	toolCallEnabledStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.ToolCallEnabled {
		toolCallEnabledStatus = i18n.T(i18n.KeyOff)
	}
	reasoningEffortStr := c.LLM.ReasoningEffort
	if reasoningEffortStr == "" {
		reasoningEffortStr = "low"
	}
	confirmDefault := "custom"
	if c.LLM.ToolModes != nil {
		if v, ok := c.LLM.ToolModes["default"]; ok {
			confirmDefault = v
		} else {
			confirmDefault = "custom"
		}
	}
	confirmDefaultStatus := confirmDefault
	logLevel := c.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	logStatus := logLevel
	if !c.LogEnabled {
		logStatus = "off"
	}
	maxIterStr := fmt.Sprintf("%d", c.LLM.MaxIterations)
	if c.LLM.MaxIterations == -1 {
		maxIterStr = i18n.T(i18n.KeyUnlimited)
	} else if c.LLM.MaxIterations == 0 {
		maxIterStr = "1000 (" + i18n.T(i18n.KeyDefault) + ")"
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

	// Build three columns: param name | current value | (label, options/range)
	col3Temp := i18n.T(i18n.KeyCol3Temperature)
	col3MaxTokens := i18n.T(i18n.KeyCol3MaxTokens)
	col3MaxIter := i18n.T(i18n.KeyCol3MaxIter)
	col3MaxRetries := i18n.T(i18n.KeyCol3MaxRetries)
	col3LlmThinking := i18n.T(i18n.KeyCol3LlmThinking)
	col3LlmContent := i18n.T(i18n.KeyCol3LlmContent)
	col3Tool := i18n.T(i18n.KeyCol3Tool)
	col3ToolInput := i18n.T(i18n.KeyCol3ToolInput)
	col3ToolOutput := i18n.T(i18n.KeyCol3ToolOutput)
	col3Command := i18n.T(i18n.KeyCol3Command)
	col3CommandOutput := i18n.T(i18n.KeyCol3CommandOutput)
	col3Confirm := i18n.T(i18n.KeyCol3Confirm)
	col3ToolTimeout := i18n.T(i18n.KeyCol3ToolTimeout)
	col3CmdTimeout := i18n.T(i18n.KeyCol3CmdTimeout)
	col3LLMTimeout := i18n.T(i18n.KeyCol3LLMTimeout)
	col3Log := i18n.T(i18n.KeyCol3Log)
	col3ResultMode := i18n.T(i18n.KeyCol3ResultMode)
	col3Name := i18n.T(i18n.KeyCol3Name)
	col3Desc := i18n.T(i18n.KeyCol3Desc)
	col3Vision := i18n.T(i18n.KeyCol3Vision)
	col3ContextLimit := i18n.T(i18n.KeyCol3ContextLimit)
	col3MemoryEnabled := i18n.T(i18n.KeyCol3MemoryEnabled)
	col3PlanEnabled := i18n.T(i18n.KeyCol3PlanEnabled)
	col3SubAgentEnabled := i18n.T(i18n.KeyCol3SubAgentEnabled)
	col3SearchMaxLineLength := i18n.T(i18n.KeyCol3SearchMaxLineLength)
	col3SearchMaxResultBytes := i18n.T(i18n.KeyCol3SearchMaxResultBytes)
	col3SearchContextLines := i18n.T(i18n.KeyCol3SearchContextLines)
	col3MemorySearchMaxContentLen := i18n.T(i18n.KeyCol3MemorySearchMaxContentLen)
	col3MemorySearchMaxResults := i18n.T(i18n.KeyCol3MemorySearchMaxResults)
	col3ThinkingEnabled := i18n.T(i18n.KeyCol3ThinkingEnabled)
	col3ReasoningEffort := i18n.T(i18n.KeyCol3ReasoningEffort)
	col3ToolCallEnabled := i18n.T(i18n.KeyCol3ToolCallEnabled)
	col3MaxModelLen := i18n.T(i18n.KeyCol3MaxModelLen)
	col3TopP := i18n.T(i18n.KeyCol3TopP)
	col3TopK := i18n.T(i18n.KeyCol3TopK)
	col3RepetitionPenalty := i18n.T(i18n.KeyCol3RepetitionPenalty)
	col3TokenUsage := i18n.T(i18n.KeyCol3TokenUsage)

	resultModeStr := ResultModeString(ResultMode(c.LLM.ResultMode))

	visionStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.VisionSupport {
		visionStatus = i18n.T(i18n.KeyOff)
	}

	memoryEnabledStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.MemoryEnabled {
		memoryEnabledStatus = i18n.T(i18n.KeyOff)
	}

	planEnabledStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.PlanEnabled {
		planEnabledStatus = i18n.T(i18n.KeyOff)
	}

	subAgentEnabledStatus := i18n.T(i18n.KeyOn)
	if !c.LLM.SubAgentEnabled {
		subAgentEnabledStatus = i18n.T(i18n.KeyOff)
	}

	// Format context limit
	contextLimitStr := fmt.Sprintf("%d", c.LLM.ContextLimit)
	if c.LLM.ContextLimit == 0 {
		contextLimitStr = i18n.T(i18n.KeyOff)
	} else if c.LLM.ContextLimit == -1 {
		contextLimitStr = i18n.T(i18n.KeyUnlimited)
	}

	agentName := c.LLM.AgentName
	if agentName == "" {
		agentName = "co-shell"
	}
	// Build description from Identity i18n content (with agent name populated)
	identityContent := strings.ReplaceAll(i18n.T(i18n.KeySystemPromptIdentity), "{AGENT_NAME}", agentName)
	// Truncate long description for display
	agentDescDisplay := identityContent
	if len(agentDescDisplay) > 120 {
		agentDescDisplay = agentDescDisplay[:120] + "..."
	}

	return fmt.Sprintf(i18n.T(i18n.KeyConfigFormat),
		"temperature:", fmt.Sprintf("%.1f", c.LLM.Temperature), col3Temp,
		"max-tokens:", fmt.Sprintf("%d", c.LLM.MaxTokens), col3MaxTokens,
		"max-iterations:", maxIterStr, col3MaxIter,
		"max-retries:", fmt.Sprintf("%d", c.LLM.MaxRetries), col3MaxRetries,
		"show-llm-thinking:", llmThinkingStatus, col3LlmThinking,
		"show-llm-content:", llmContentStatus, col3LlmContent,
		"show-tool:", toolStatus, col3Tool,
		"show-tool-input:", toolInputStatus, col3ToolInput,
		"show-tool-output:", toolOutputStatus, col3ToolOutput,
		"show-command:", commandStatus, col3Command,
		"show-command-output:", commandOutputStatus, col3CommandOutput,
		"confirm-tool:", confirmDefaultStatus, col3Confirm,
		"result-mode:", resultModeStr, col3ResultMode,
		"tool-timeout:", toolTimeoutStr, col3ToolTimeout,
		"cmd-timeout:", cmdTimeoutStr, col3CmdTimeout,
		"llm-timeout:", llmTimeoutStr, col3LLMTimeout,
		"log:", logStatus, col3Log,
		"name:", agentName, col3Name,
		"description:", agentDescDisplay, col3Desc,
		"vision:", visionStatus, col3Vision,
		"context-limit:", contextLimitStr, col3ContextLimit,
		"memory-enabled:", memoryEnabledStatus, col3MemoryEnabled,
		"plan-enabled:", planEnabledStatus, col3PlanEnabled,
		"subagent-enabled:", subAgentEnabledStatus, col3SubAgentEnabled,
		"search-max-line-length:", fmt.Sprintf("%d", c.LLM.SearchMaxLineLength), col3SearchMaxLineLength,
		"search-max-result-bytes:", fmt.Sprintf("%d", c.LLM.SearchMaxResultBytes), col3SearchMaxResultBytes,
		"search-context-lines:", fmt.Sprintf("%d", c.LLM.SearchContextLines), col3SearchContextLines,
		"memory-search-max-content-len:", fmt.Sprintf("%d", c.LLM.MemorySearchMaxContentLen), col3MemorySearchMaxContentLen,
		"memory-search-max-results:", fmt.Sprintf("%d", c.LLM.MemorySearchMaxResults), col3MemorySearchMaxResults,
		"thinking-enabled:", thinkingEnabledStatus, col3ThinkingEnabled,
		"reasoning-effort:", reasoningEffortStr, col3ReasoningEffort,
		"toolcall-enabled:", toolCallEnabledStatus, col3ToolCallEnabled,
		"max-model-len:", fmt.Sprintf("%d", c.LLM.MaxModelLen), col3MaxModelLen,
		"top-p:", fmt.Sprintf("%.1f", c.LLM.TopP), col3TopP,
		"top-k:", fmt.Sprintf("%d", c.LLM.TopK), col3TopK,
		"repetition-penalty:", fmt.Sprintf("%.1f", c.LLM.RepetitionPenalty), col3RepetitionPenalty,
		"token-usage:", c.LLM.TokenUsage, col3TokenUsage)

}
