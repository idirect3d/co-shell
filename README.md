# co-shell

> Intelligent Command-Line Shell — Interact with AI Agent via natural language to intelligently orchestrate and execute system commands.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-00135-blue)]()

[![中文](https://img.shields.io/badge/README-中文-blue)](docs/zh-CN/README.md)

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

Download the zip archive for your platform from the [Releases](https://github.com/idirect3d/co-shell/releases) page:

| OS | Architecture | Download |
|---|---|---|
| macOS | Intel | [co-shell-v0.3.0-darwin-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-amd64.zip) |
| macOS | Apple Silicon | [co-shell-v0.3.0-darwin-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip) |
| Linux | x86_64 | [co-shell-v0.3.0-linux-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-amd64.zip) |
| Linux | ARM64 | [co-shell-v0.3.0-linux-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-arm64.zip) |
| Windows | x86_64 | [co-shell-v0.3.0-windows-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip) |
| Windows | ARM64 | [co-shell-v0.3.0-windows-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-arm64.zip) |
| 工具 | 跨平台 | [md2docx.py](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/md2docx.py) — Markdown 转 Word 文档转换器 |

**macOS / Linux:**
```bash
# curl
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip
unzip co-shell.zip && rm co-shell.zip
chmod +x co-shell
./co-shell

# or wget
wget https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip
unzip co-shell-v0.3.0-darwin-arm64.zip && rm co-shell-v0.3.0-darwin-arm64.zip
chmod +x co-shell
./co-shell
```

**Windows (PowerShell):**
```powershell
# PowerShell
Invoke-WebRequest -Uri https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip -OutFile co-shell.zip
Expand-Archive -Path co-shell.zip -DestinationPath .
.\co-shell.exe
```

**Windows (CMD):**
```cmd
:: CMD
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip
tar -xf co-shell.zip
del co-shell.zip
co-shell.exe
```


> **💡 md2docx.py 工具**: 用于将 Markdown 文件转换为精美 Word 文档（支持公文格式 GB/T 9704-2012）。下载后放入 co-shell 运行目录下的 `bin/` 文件夹即可，co-shell 会自动安装依赖并调用它生成 DOCX 报告。

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
  -w, --workspace <path>  Workspace path (default: current directory)
  -c, --config <path>     Config file path (default: {workspace}/config.json)
  -m, --model <name>      Temporarily override model name
  -e, --endpoint <url>    Temporarily override API endpoint
  -k, --api-key <key>     Temporarily override API Key
  -n, --name <name>       Set agent name (default: co-shell)
  -i, --image <path>      Image file path(s) for multimodal input
      --lang <code>       Set language (zh/en, auto-detect by default)
      --log on|off        Temporarily toggle logging
      --max-iterations N  Max iterations (-1 for unlimited, default 1000)
      --temperature N     Temperature (0.0 ~ 2.0)
      --max-tokens N      Max output tokens
      --show-thinking     Show AI thinking process (on/off)
      --show-command      Show executed commands (on/off)
      --show-output       Show command output (on/off)
      --confirm-command   Confirm before executing commands (on/off)
      --result-mode       Result mode (minimal/explain/analyze/free)
      --description       Agent description/expertise
      --principles        Agent core principles
      --tool-timeout N    Tool call timeout in seconds (0=unlimited)
      --cmd-timeout N     Command timeout in seconds (0=unlimited)
      --llm-timeout N     LLM request timeout in seconds (0=unlimited)
      --vision            Vision support (on/off)
  -v, --version           Show version info
  -h, --help              Show help
```

---

## Built-in Commands

All built-in commands start with `.` and support Tab completion.

| Command | Description |
|---|---|
| `.set` | LLM API parameter management (api-key / endpoint / model / temperature / max-tokens / max-iterations / show-thinking / show-command / show-output / log / result-mode / name / description / principles / vision / tool-timeout / cmd-timeout / llm-timeout) |
| `.mcp` | MCP Server management (add / remove / list / enable / disable) |
| `.rule` | Global rule management (add / remove / clear) |
| `.memory` | Persistent memory management (save / get / search / delete / clear) |
| `.context` | Context management (show / reset / set) |
| `.image` | Multimodal image cache management (add / remove / clear / list) |
| `.plan` | Task plan management (list / view / create / insert / remove / update) |
| `.wizard` | Restart the API setup wizard |
| `.list` | View history task list |
| `.last` | View recent history tasks |
| `.first` | View earliest history tasks |
| `.help` | Show this help message |
| `.exit` | Exit co-shell |

---

## Sample Research Reports

co-shell can autonomously conduct in-depth research by searching the web, collecting data, and generating comprehensive reports. Here are some real-world examples produced entirely by co-shell:

### 1. Arctic Ocean Brand Research (北冰洋品牌深度调研报告)

A comprehensive brand research report on the iconic Chinese soda brand "Arctic Ocean" (北冰洋), tracing its history from 1936 to the present. The report covers brand evolution, product development, market competition, and strategic recommendations.

> **Report**: [arctic-ocean-brand-research-report.md](samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.md) (267 pages) | [DOCX](samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.docx)

### 2. Beijing Zoo Panda Latest News (北京动物园大熊猫最新动态研究报告)

A timely research report on the latest developments of giant pandas at Beijing Zoo, covering facility upgrades, new arrivals, and public events from 2023 to 2026.

> **Report**: [beijing-zoo-panda-latest-report.md](samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.md) (196 pages) | [DOCX](samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.docx)

### 3. China Reinsurance Market 2026-2027 (中国再保险市场调研报告)

A professional market research report analyzing the impact of the 2026 Iran war on China's reinsurance market, covering geopolitical risks, energy market turmoil, and strategic recommendations.

> **Report**: [china-reinsurance-market-report-2026-2027.md](samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.md) (442 pages) | [DOCX](samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.docx)

### 4. Y-10 Aircraft Research (中国运-10飞机深度调研报告)

An in-depth research report on China's first domestically developed jet airliner — the Y-10 (运-10), covering its development history, technical specifications, and historical significance.

> **Report**: [Y-10-in-depth-research-report.md](samples/research/yun10/Y-10-in-depth-research-report.md) (290 pages) | [DOCX](samples/research/yun10/Y-10-in-depth-research-report.docx)

---

## Tech Stack

- **Language**: Go 1.22+
- **REPL**: [go-prompt](https://github.com/c-bata/go-prompt)
- **LLM**: [go-openai](https://github.com/sashabaranov/go-openai) (OpenAI / DeepSeek / compatible models)
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **Storage**: [bbolt](https://go.etcd.io/bbolt) (embedded KV database)

---

## Version History

### v0.3.0 — RC1 (Current)

> **BUILD**: 00135 | **Release Date**: 2026-05-01

Release Candidate 1 — feature complete, ready for preview.

**版本亮点**: 全面对话上下文管理、持久化记忆与检索、LLM 输出模式切换、思考过程开关、Token 用量统计、错误重试智能限制、数字批量批准、小米/GLM 最新模型支持、**报告自动生成公文格式 DOCX**（GB/T 9704-2012 标准）。

**Implemented Features:**

- Multimodal model support (image input, visual understanding) with 👀 indicator
- Image cache management (.image command, add_images/remove_images/clear_images tools)
- Agent identity customization (name/description/principles via .set)
- Task plan management (.plan command, create_task_plan/update_task_step/insert_task_steps/remove_task_steps/view_task_plan/list_task_plans tools)
- Task plan singleton mode — only one active plan at a time, auto-archive on completion
- Batch command execution with "Approve All" inheritance for sub-agents
- 12 new CLI flags (--temperature/--max-tokens/--show-thinking/--show-command/--show-output/--confirm-command/--result-mode/--description/--principles/--tool-timeout/--cmd-timeout/--llm-timeout)
- Vision support auto-detection via model API
- Enhanced help documentation with complete parameter descriptions
- **Conversation context limit** (.set context-limit) — control how many history messages sent to LLM
- **Persistent memory management** (.memory command, get_memory_slice/memory_search tools)
- **Memory toggle** (.set memory-enabled, --memory-enabled/--memory-disabled)
- **LLM output mode** (.set output-mode) — compact / normal / debug modes
- **Sub-agent toggle** (.set subagent-enabled) — control sub-agent tool availability
- **Thinking toggle** (.set thinking-enabled, --thinking-enabled/--thinking-disabled) — control AI reasoning process display
- **Reasoning effort** (.set reasoning-effort) — control AI reasoning depth (low/medium/high)
- **Token usage statistics** — cumulative token tracking via Agent.TokenUsage()
- **Conversation reset** (.new command) — clear all history without restart
- **Error retry limit** — configurable single-error and type-error max counts with user prompt
- **Number-based approval** — enter a number to auto-approve N subsequent command executions
- **search_files enhancement** — binary file ignore, content length protection, configurable limits
- **Instant .set effect** — no restart needed after parameter changes
- **New model support** — Xiaomi (Mi), GLM (Z.ai) latest models
- **Setup wizard enhancement** — skip endpoint for built-in providers, enhanced risk warnings
- **Sample research reports** — added real-world usage examples
- **Official document format output** — research reports auto-generated as GB/T 9704-2012 compliant DOCX (小标宋 title, 黑体/楷体/仿宋 headings, fixed 28pt line spacing, 2-char first-line indent). [Download samples](#sample-research-reports) to see the effect.

### v0.2.0 — Beta

> **BUILD**: 00087 | **Release Date**: 2026-04-27

Beta release with enhanced capabilities for daily use.

**Implemented Features:**

- Agent built-in file operation tools (read_file, search_files, list_code_definition_names, replace_in_file, write_to_file)
- Workspace architecture with --workspace flag and auto-created subdirectories
- Agent system prompt multi-language support (auto-switch Chinese/English)
- Sub-agent support: launch child co-shell processes, shared I/O, result collection
- Scheduled task execution with crontab-like scheduling
- Custom config file path via -c/--config flag
- Custom agent name via --name/-n flag
- Multi-turn conversation context management
- System command execution sandbox
- Command confirmation for dangerous operations
- Config file hot-reload
- Enhanced error handling and user prompts

### v0.1.0 — Alpha

> **BUILD**: 00074 | **Release Date**: 2026-04-26

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
├── scheduler/           # Scheduled task scheduler
├── subagent/            # Sub-agent management
├── taskplan/            # Task plan management
├── memory/              # Persistent memory management
├── workspace/           # Workspace management
├── samples/             # Sample research reports
├── bin/                 # Binary output directory
├── output/              # Output directory
├── CHANGELOG.md         # Changelog
├── USAGE.md             # Detailed usage guide
├── docs/                # Multi-language documentation
│   └── zh-CN/           # Chinese documentation
└── ROADMAP.md           # Version plan and roadmap
```

---

## License

[MIT](LICENSE) © 2026 L.Shuang

---

## Author

- **L.Shuang** — [GitHub](https://github.com/idirect3d)
