# co-shell

> Intelligent Command-Line Shell — Interact with AI Agent via natural language to intelligently orchestrate and execute system commands.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-00075-blue)]()

[![中文](https://img.shields.io/badge/README-中文-blue)](../zh-CN/README.md)


---

## Introduction

**co-shell** is a universal command — simple yet powerful.

At just ~10MB with zero external dependencies, it can orchestrate virtually every capability of your computer with a single instruction. To accomplish a task, it can call upon almost any command it knows, with virtually no limits. It's safe — every command execution requires your consent; it's transparent — every command is displayed before execution.

And remember, it's just a command itself — it can call itself. Infinite possibilities?

> **Your imagination is its fuel — the command is everything.**

No more memorizing complex command parameters. Just say what you want, and co-shell will automatically understand, orchestrate, and execute the appropriate system commands, programs, or scripts.

```bash
# Traditional way
$ find . -type f -name "*.go" | xargs wc -l | tail -1

# co-shell way
❯ Count all lines of Go source code in this project
```

### ⚠️ Disclaimer

co-shell is an intelligent command-line tool powered by Large Language Models (LLM). AI models may generate and execute dangerous commands including:

- Deleting files or directories (e.g., `rm -rf /`)
- Formatting disks (e.g., `mkfs`, `format`)
- Modifying critical system configurations
- Shutting down or rebooting the system
- Downloading and executing untrusted programs
- Leaking sensitive information (e.g., API keys, passwords, credentials)

By using this software, you acknowledge that you fully understand the above risks and agree to assume all responsibility for any loss or damage resulting from its use. The developers and publishers assume no liability.

> The full disclaimer is shown on first launch and requires confirmation. It will not be shown again after acceptance.

### Key Features

- 🗣️ **Natural Language Interaction** — Describe tasks in plain English
- 🤖 **AI Agent Driven** — Intelligent intent understanding, command orchestration, execution, and result analysis
- 🔧 **MCP Protocol Support** — Access a rich ecosystem of MCP tools (filesystem, GitHub, databases, etc.)
- 💾 **Persistent Memory** — Agent remembers your preferences and historical context
- 📜 **Session History** — Browse past commands with arrow keys
- ⚡ **Streaming Output** — Real-time display of AI thinking and command execution results
- 🔌 **Extensible** — Custom rules, MCP Servers, multi-model switching

---

## Quick Start

### Installation

#### Option 1: Download Binary (Recommended)

Download the executable for your platform from the [Releases](https://github.com/idirect3d/co-shell/releases) page:

| OS | Architecture | File |
|---|---|---|
| macOS | Intel | `co-shell-darwin-amd64` |
| macOS | Apple Silicon | `co-shell-darwin-arm64` |
| Linux | x86_64 | `co-shell-linux-amd64` |
| Linux | ARM64 | `co-shell-linux-arm64` |
| Windows | x86_64 | `co-shell-windows-amd64.exe` |
| Windows | ARM64 | `co-shell-windows-arm64.exe` |

```bash
# Example: macOS Apple Silicon
chmod +x co-shell-darwin-arm64
./co-shell-darwin-arm64
```

#### Option 2: Build from Source

```bash
git clone https://github.com/idirect3d/co-shell.git
cd co-shell
go build -o co-shell .
./co-shell
```

### Configure API

The setup wizard will launch automatically on first startup. You can also configure manually:

```bash
❯ .settings api-key sk-your-api-key-here
❯ .settings endpoint https://api.deepseek.com/v1
❯ .settings model deepseek-chat
```

### Start Using

```bash
❯ List all files in the current directory
❯ Find all files larger than 100MB
❯ Check how much disk space is left
❯ Create a new Go project
```

---

## Command-Line Options

```bash
co-shell [options]                    Start interactive REPL
co-shell [options] <instruction>      Execute single instruction and exit

Options:
  -c, --config <path>    Specify config file path (default: ~/.co-shell/config.json)
  -m, --model <name>     Temporarily override model name
  -e, --endpoint <url>   Temporarily override API endpoint
  -k, --api-key <key>    Temporarily override API Key
      --log on|off       Temporarily toggle logging
      --max-iterations   Max iterations (-1 for unlimited, default 10)
  -v, --version          Show version info
  -h, --help             Show help
```

---

## Built-in Commands

All built-in commands start with `.` and support Tab completion.

| Command | Description |
|---|---|
| `.settings` | LLM API parameter management (api-key / endpoint / model / temperature / max-tokens / max-iterations / show-thinking / show-command / show-output / log) |
| `.mcp` | MCP Server management (add / remove / list / enable / disable) |
| `.rule` | Global rule management (add / remove / clear) |
| `.memory` | Persistent memory management (save / get / search / delete / clear) |
| `.context` | Context management (show / reset / set) |

---

## Tech Stack

- **Language**: Go 1.22+
- **REPL**: [go-prompt](https://github.com/c-bata/go-prompt)
- **LLM**: [go-openai](https://github.com/sashabaranov/go-openai) (OpenAI / DeepSeek / compatible models)
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **Storage**: [bbolt](https://go.etcd.io/bbolt) (embedded KV database)

---

## Version History

### v0.1.0 — Alpha (Current)

> **BUILD**: 00075 | **Release Date**: 2026-04-26


First Alpha preview with core functionality.

**Implemented Features:**

- REPL interactive interface (go-prompt, Tab completion)
- LLM client abstraction (OpenAI-compatible API, streaming support)
- Agent core loop (LLM call → tool execution → iteration)
- Built-in command system (.set / .mcp / .rule / .memory / .context / .list / .last / .first / .wizard)
- Persistent storage (bbolt for memory/context)
- MCP client manager (multi-server connection)
- System command execution (timeout control, command confirmation)
- Configuration management (JSON persistence to ~/.co-shell/, multi-location loading)
- API initial setup with setup wizard (multi-provider support)
- Direct system command execution (bypass LLM for native commands)
- Logging system (file log, runtime toggle)
- API Key masking (show first 4 + last 4 chars)
- Command-line flags (--help / --version / --model / --endpoint / --api-key / --log / --max-iterations / --lang)
- Session history (arrow key navigation, cross-session persistence, .list/.last/.first commands)
- Internationalization (i18n) support: Chinese and English, --lang flag, auto-detect system language
- Multi-provider support (DeepSeek v4 / Alibaba Qwen / OpenAI-compatible fallback)
- Result processing modes (minimal / explain / analyze / free)
- Configurable timeouts for all operations
- Cross-platform support (macOS / Linux / Windows)
- Streaming output with thinking process display

---

## Project Structure

```
co-shell/
├── main.go              # Entry point, module initialization
├── config/              # Configuration management (LLM/MCP/Rules)
├── repl/                # REPL interaction layer
├── agent/               # Agent core loop
├── llm/                 # LLM client abstraction
├── mcp/                 # MCP client manager
├── store/               # Persistent storage (bbolt)
├── cmd/                 # Built-in command handlers
├── log/                 # Logging system
├── i18n/                # Internationalization
├── wizard/              # Setup wizard
├── USAGE.md             # Detailed usage guide
├── README.zh.md         # Chinese documentation
└── ROADMAP.md           # Version plan and roadmap
```

---

## License

[MIT](LICENSE) © 2026 L.Shuang

---

## Author

- **L.Shuang** — [GitHub](https://github.com/idirect3d)
