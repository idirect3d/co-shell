# Changelog

All notable changes to co-shell will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.2.0] - 2026-04-27

### Added
- Agent built-in file operation tools: `read_file`, `search_files`, `list_code_definition_names`, `replace_in_file`, `write_to_file`
- Workspace architecture: `--workspace` flag, auto-created `bin/`, `db/`, `log/`, `output/`, `tmp/` subdirectories
- Agent system prompt multi-language support (auto-switch Chinese/English based on i18n)
- Sub-agent support: launch child co-shell processes as sub-agents, shared I/O, result collection
- Scheduled task execution: crontab-like scheduling for sub-agents
- `-c`/`--config` flag for custom config file path, `config.LoadFromFile()` method
- `--name`/`-n` flag for custom agent name, `Said()` method with timestamp and name
- `.list`/`.last`/`.first` help documentation in `.help` command

### Fixed
- Sub-agent command argument conflict with `-c` flag causing config path loss

## [0.1.0] - 2026-04-26

### Added
- REPL interactive interface (go-prompt, Tab completion)
- LLM client abstraction (OpenAI-compatible API, streaming support)
- Agent core loop (LLM call → tool execution → iteration)
- Built-in command system (`.set` / `.mcp` / `.rule` / `.memory` / `.context` / `.list` / `.last` / `.first` / `.wizard`)
- Persistent storage (bbolt for memory/context)
- MCP client manager (multi-server connection)
- System command execution (timeout control, command confirmation)
- Configuration management (JSON persistence to `~/.co-shell/`, multi-location loading)
- API initial setup with setup wizard (multi-provider support)
- Direct system command execution (bypass LLM for native commands)
- Logging system (file log, runtime toggle)
- API Key masking (show first 4 + last 4 chars)
- Command-line flags (`--help` / `--version` / `--model` / `--endpoint` / `--api-key` / `--log` / `--max-iterations` / `--lang`)
- Single-command mode (`-c`/`--cmd` flag)
- Session history (arrow key navigation, cross-session persistence, `.list`/`.last`/`.first` commands)
- Internationalization (i18n) support: Chinese and English, `--lang` flag, auto-detect system language
- Multi-provider support (DeepSeek v4 / Alibaba Qwen / OpenAI-compatible fallback)
- Result processing modes (minimal / explain / analyze / free)
- Configurable timeouts for all operations
- Cross-platform support (macOS / Linux / Windows)
- Streaming output with thinking process display
- Setup wizard enhancements: Tab completion, arrow key selection, ESC exit, connection test
- Setup wizard auto-test: endpoint connectivity check, auto-fetch model list on API key input

### Fixed
- DeepSeek thinking mode: `reasoning_content` not passed back causing API 400 error
- `max_iterations=0` in config.json causing Agent to use internal default of 10
- Stream completion causing non-streaming API call and halving iteration count
- Message history incomplete (assistant with tool_calls but missing tool messages) causing API 400 error

