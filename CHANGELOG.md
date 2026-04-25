# Changelog

All notable changes to co-shell will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Internationalization (i18n) support: Chinese and English, `--lang` flag, auto-detect system language
- `--lang` and `--max-iterations` flags in `--help` examples

### Changed
- `--help` output is now internationalized (switches between Chinese/English)

---

## [0.1.0] - 2026-04-25

### Added
- REPL interactive interface (go-prompt, Tab completion)
- LLM client abstraction (OpenAI-compatible API, streaming support)
- Agent core loop (LLM call → tool execution → iteration)
- Built-in command system (`.settings` / `.mcp` / `.rule` / `.memory` / `.context`)
- Persistent storage (bbolt for memory/context)
- MCP client manager (multi-server connection)
- System command execution (timeout control)
- Configuration management (JSON persistence to `~/.co-shell/`)
- API initial setup with setup wizard
- Direct system command execution (bypass LLM for native commands)
- Logging system (file log, runtime toggle)
- API Key masking (show first 4 + last 4 chars)
- Command-line flags (`--help` / `--version` / `--model` / `--endpoint` / `--api-key` / `--log` / `--max-iterations`)
- Single-command mode (`-c`/`--cmd` flag)
- Session history (arrow key navigation, cross-session persistence)
- Basic error handling and user prompts
- Configurable max iterations
- Multi-config location support (CLI flag > local `config.json` > `~/.co-shell/config.json`)
- Multi-provider support (DeepSeek v4 / Alibaba Qwen / OpenAI-compatible fallback)
- Setup wizard enhancements: Tab completion, arrow key selection, ESC exit, connection test
- Setup wizard auto-test: endpoint connectivity check, auto-fetch model list on API key input

---

## [0.0.1] - 2026-04-25

### Added
- Project initialization
- Basic project structure
