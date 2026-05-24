# co-shell

> Intelligent Command-Line Shell — Interact with AI Agent via natural language to intelligently orchestrate and execute system commands.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-00190-blue)]()

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
| macOS | Intel | [co-shell-v0.5.0-Beta2-darwin-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-darwin-amd64.zip) |
| macOS | Apple Silicon | [co-shell-v0.5.0-Beta2-darwin-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-darwin-arm64.zip) |
| Linux | x86_64 | [co-shell-v0.5.0-Beta2-linux-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-linux-amd64.zip) |
| Linux | ARM64 | [co-shell-v0.5.0-Beta2-linux-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-linux-arm64.zip) |
| Windows | x86_64 | [co-shell-v0.5.0-Beta2-windows-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-windows-amd64.zip) |
| Windows | ARM64 | [co-shell-v0.5.0-Beta2-windows-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-windows-arm64.zip) |
| 工具 | 跨平台 | [md2docx.py](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/md2docx.py) — Markdown 转 Word 文档转换器 |
| **Bridge** | macOS Intel | [co-shell-feishu-bridge-v0.1.0-darwin-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-darwin-amd64.zip) |
| **Bridge** | macOS Apple Silicon | [co-shell-feishu-bridge-v0.1.0-darwin-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-darwin-arm64.zip) |
| **Bridge** | Linux x86_64 | [co-shell-feishu-bridge-v0.1.0-linux-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-linux-amd64.zip) |
| **Bridge** | Linux ARM64 | [co-shell-feishu-bridge-v0.1.0-linux-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-linux-arm64.zip) |
| **Bridge** | Windows x86_64 | [co-shell-feishu-bridge-v0.1.0-windows-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-windows-amd64.zip) |
| **Bridge** | Windows ARM64 | [co-shell-feishu-bridge-v0.1.0-windows-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-feishu-bridge-v0.1.0-windows-arm64.zip) |

**macOS / Linux:**
```bash
# curl
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-darwin-arm64.zip
unzip co-shell.zip && rm co-shell.zip
chmod +x co-shell
./co-shell

# or wget
wget https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-darwin-arm64.zip
unzip co-shell-v0.5.0-Beta2-darwin-arm64.zip && rm co-shell-v0.5.0-Beta2-darwin-arm64.zip
chmod +x co-shell
./co-shell
```

**Windows (PowerShell):**
```powershell
# PowerShell
Invoke-WebRequest -Uri https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-windows-amd64.zip -OutFile co-shell.zip
Expand-Archive -Path co-shell.zip -DestinationPath .
.\co-shell.exe
```

**Windows (CMD):**
```cmd
:: CMD
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-v0.5.0-Beta2-windows-amd64.zip
tar -xf co-shell.zip
del co-shell.zip
co-shell.exe
```

> **⚠️ Windows 安全提示**: co-shell 使用 Go 语言编译，是标准的原生可执行程序。由于未进行数字签名，Windows Defender 或部分杀毒软件可能会误报病毒。这是 Go 编译的 CLI 工具的常见情况（如 Hugo、Terraform 等均会遇到）。请放心，本项目完全开源，您可自行从源码编译验证。如遇到拦截，点击"更多信息"→"仍要运行"即可。

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
      --toolcall-enabled  Enable tool calling (on/off)
      --toolcall-mode     Tool call mode (openai/xml)
      --confirm-tool      Confirm before tool calls (on/off)
      --memory-enabled    Enable persistent memory (on/off)
      --plan-enabled      Enable task plan (on/off)
      --subagent-enabled  Enable sub-agent (on/off)
      --emoji-enabled     Enable emoji prefixes (on/off)
      --show-logo         Show startup logo (on/off)
      --token-usage       Token usage display (on/off/none)
      --context-start     Context start mode (window/task/smart)
      --loop-detect-enabled  Enable loop detection (on/off)
      --dedup-enabled     Enable message deduplication (on/off)
      --log-level         Log level (debug/info/warn/error/off)
      --output-mode       Output mode (compact/normal/debug)
      --top-p N           Top-P sampling (0.0~1.0)
      --top-k N           Top-K sampling
      --repetition-penalty N  Repetition penalty (0.0~2.0)
      --body-add          Custom JSON body properties (key=value)
  -v, --version           Show version info
  -h, --help              Show help
```

---

## Configuration Reference

The following table lists all configurable parameters, their CLI flags, REPL `.set` commands, JSON config keys, default values, and descriptions.

| Parameter | CLI Flag | REPL `.set` | Config Key | Default | Description |
|---|---|---|---|---|---|
| **API & Model** | | | | | |
| API Key | `-k, --api-key` | `api-key` | `api_key` | `""` | LLM provider API key |
| Endpoint | `-e, --endpoint` | `endpoint` | `endpoint` | `https://api.deepseek.com` | LLM API endpoint URL |
| Model | `-m, --model` | `model` | `model` | `deepseek-v4-flash` | LLM model name |
| Provider | — | — | `provider` | `deepseek` | LLM provider preset name |
| Temperature | `--temperature` | `temperature` | `temperature` | `0.7` | LLM temperature (0.0–2.0) |
| Max Tokens | `--max-tokens` | `max-tokens` | `max_tokens` | `393216` | Max output tokens |
| Max Iterations | `--max-iterations` | `max-iterations` | `max_iterations` | `1000` | Max agent loop iterations (-1 = unlimited) |
| Max Retries | — | `max-retries` | `max_retries` | `3` | Max retries for transient LLM errors |
| Vision Support | `--vision` | `vision` | `vision_support` | `off` | Enable multimodal image input |
| Thinking Mode | — | `thinking-enabled` | `thinking_enabled` | `off` | Enable LLM reasoning/thinking mode |
| Reasoning Effort | — | `reasoning-effort` | `reasoning_effort` | `low` | Reasoning depth (low/medium/high) |
| ToolCall Enabled | `--toolcall-enabled` | `toolcall-enabled` | `tool_call_enabled` | `on` | Enable tool/function calling |
| ToolCall Mode | `--toolcall-mode` | `tool mode` | `tool_call_mode` | `openai` | Tool call mode (openai/xml) |
| Top-P | `--top-p` | `top-p` | `top_p` | `-1` (none) | Top-P sampling (0.0~1.0, -1=don't send) |
| Top-K | `--top-k` | `top-k` | `top_k` | `-1` (none) | Top-K sampling (-1=don't send) |
| Repetition Penalty | `--repetition-penalty` | `repetition-penalty` | `repetition_penalty` | `-1` (none) | Repetition penalty (0.0~2.0, -1=don't send) |
| Body Additions | `--body-add` | — | `body_additions` | `{}` | Custom JSON properties for LLM request body |
| **Display & Output** | | | | | |
| Show Thinking | `--show-thinking` | `show-thinking` | `show_thinking` | `off` | Display AI thinking process |
| Show Command | `--show-command` | `show-command` | `show_command` | `on` | Display executed system commands |
| Show Output | `--show-output` | `show-output` | `show_output` | `on` | Display command execution output |
| Result Mode | `--result-mode` | `result-mode` | `result_mode` | `free` | Result presentation (minimal/explain/analyze/free) |
| Output Mode | `--output-mode` | `output-mode` | `output_mode` | `normal` | LLM front-end output (compact/normal/debug) |
| Token Usage | `--token-usage` | `token-usage` | `token_usage` | `on` | Token usage display (on/off/none) |
| Emoji Enabled | `--emoji-enabled` | `emoji-enabled` | `emoji_enabled` | `on` | Enable emoji role prefixes |
| Show Logo | `--show-logo` | `show-logo` | `show_logo` | `on` | Show startup ASCII logo |
| **Safety & Timeout** | | | | | |
| Confirm Command | `--confirm-command` | `confirm-command` | `confirm_command` | `on` | Require confirmation before executing commands |
| Confirm Tool | `--confirm-tool` | `confirm-tool` | `confirm_tool` | `on` | Require confirmation before tool calls |
| Tool Timeout | `--tool-timeout` | `tool-timeout` | `tool_timeout` | `0` (unlimited) | Tool call timeout in seconds |
| Command Timeout | `--cmd-timeout` | `cmd-timeout` | `cmd_timeout` | `0` (unlimited) | System command timeout in seconds |
| LLM Timeout | `--llm-timeout` | `llm-timeout` | `llm_timeout` | `0` (unlimited) | LLM API request timeout in seconds |
| Error Max Single | — | `error-max-single-count` | `error_max_single_count` | `10` | Max occurrences of same error before prompt |
| Error Max Types | — | `error-max-type-count` | `error_max_type_count` | `100` | Max distinct error types before prompt |
| **Memory & Context** | | | | | |
| Memory Enabled | `--memory-enabled` | `memory-enabled` | `memory_enabled` | `on` | Enable persistent memory tools |
| Context Limit | — | `context-limit` | `context_limit` | `-1` (all) | Recent messages in LLM context (0=off, N=last N) |
| Context Start Mode | `--context-start` | `context-start` | `context_start_mode` | `task` | Context start mode (window/task/smart) |
| Memory Search Max Content | — | `memory-search-max-content-len` | `memory_search_max_content_len` | `512` | Max content length in memory search results |
| Memory Search Max Results | — | `memory-search-max-results` | `memory_search_max_results` | `100` | Max results returned by memory search |
| **Tasks & Sub-Agents** | | | | | |
| Plan Enabled | `--plan-enabled` | `plan-enabled` | `plan_enabled` | `on` | Enable task plan tools |
| SubAgent Enabled | `--subagent-enabled` | `subagent-enabled` | `sub_agent_enabled` | `on` | Enable sub-agent tools |
| **Loop Detection & Dedup** | | | | | |
| Loop Detect Enabled | `--loop-detect-enabled` | `loop-detect-enabled` | `loop_detect_enabled` | `on` | Enable LLM output loop detection |
| Dedup Enabled | `--dedup-enabled` | `dedup-enabled` | `dedup_enabled` | `on` | Enable message deduplication |
| **Search & Debug** | | | | | |
| Search Max Line Length | — | `search-max-line-length` | `search_max_line_length` | `8192` | Max chars per line in search results |
| Search Max Result Bytes | — | `search-max-result-bytes` | `search_max_result_bytes` | `65536` | Max total bytes for search results |
| Search Context Lines | — | `search-context-lines` | `search_context_lines` | `5` | Context lines before/after each match |
| Log Enabled | `--log` | `log` | `log_enabled` | `on` | Enable file logging |
| Log Level | `--log-level` | `log-level` | `log_level` | `info` | Log level (debug/info/warn/error/off) |
| **Agent Identity** | | | | | |
| Agent Name | `-n, --name` | `name` | `agent_name` | `co-shell` | Agent name for identification |
| Agent Description | `--description` | `description` | `agent_description` | `""` | Agent expertise description |
| Agent Principles | `--principles` | `principles` | `agent_principles` | `""` | Agent core principles |
| **Workspace** | | | | | |
| Workspace Path | `-w, --workspace` | — | — | current dir | Workspace root directory |
| Config Path | `-c, --config` | — | — | `{workspace}/config.json` | Config file path |
| Language | `--lang` | — | — | auto-detect | Interface language (zh/en) |
| Image Input | `-i, --image` | — | — | `""` | Image path(s) for multimodal input |

---

## Built-in Commands

All built-in commands start with `.` and support Tab completion.

| Command | Description |
|---|---|
| `.set` | LLM API parameter management (see Configuration Reference above) |
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

co-shell can autonomously conduct in-depth research by searching the web, collecting data, and generating comprehensive reports.

<video src="https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell.mp4" controls width="800"></video>

*Demo: co-shell autonomously researching and generating a comprehensive report.*

Here are some real-world examples produced entirely by co-shell:

### 1. Arctic Ocean Brand Research (北冰洋品牌深度调研报告)

A comprehensive brand research report on the iconic Chinese soda brand "Arctic Ocean" (北冰洋), tracing its history from 1936 to the present. The report covers brand evolution, product development, market competition, and strategic recommendations.

> **Report**: [arctic-ocean-brand-research-report.md](samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.md) (14 pages) | [DOCX](samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.docx)

### 2. Beijing Zoo Panda Latest News (北京动物园大熊猫最新动态研究报告)

A timely research report on the latest developments of giant pandas at Beijing Zoo, covering facility upgrades, new arrivals, and public events from 2023 to 2026.

> **Report**: [beijing-zoo-panda-latest-report.md](samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.md) (13 pages) | [DOCX](samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.docx)

### 3. China Reinsurance Market 2026-2027 (中国再保险市场调研报告)

A professional market research report analyzing the impact of the 2026 Iran war on China's reinsurance market, covering geopolitical risks, energy market turmoil, and strategic recommendations.

> **Report**: [china-reinsurance-market-report-2026-2027.md](samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.md) (26 pages) | [DOCX](samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.docx)

### 4. Y-10 Aircraft Research (中国运-10飞机深度调研报告)

An in-depth research report on China's first domestically developed jet airliner — the Y-10 (运-10), covering its development history, technical specifications, and historical significance.

> **Report**: [Y-10-in-depth-research-report.md](samples/research/yun10/Y-10-in-depth-research-report.md) (13 pages) | [DOCX](samples/research/yun10/Y-10-in-depth-research-report.docx)

---

## Tech Stack

- **Language**: Go 1.22+
- **REPL**: [go-prompt](https://github.com/c-bata/go-prompt)
- **LLM**: [go-openai](https://github.com/sashabaranov/go-openai) (OpenAI / DeepSeek / compatible models)
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **Storage**: [bbolt](https://go.etcd.io/bbolt) (embedded KV database)

---

## Version History

### v0.5.0 — Beta2 (Current)

> **BUILD**: 00190 | **Release Date**: 2026-05-22

Beta2 release — feature complete, stable and usable.

**版本亮点**: 可配置工具调用模式（OpenAI/XML）、XML 模式支持非 function calling 模型、PostgreSQL 持久化存储、会话自动恢复、LLM 输出循环检测与消息去重、工具调用确认机制扩展、模型管理交互体验优化、数据库配置子命令模式。

**Implemented Features:**

- **ToolCall mode** — configurable tool call mechanism supporting OpenAI standard API and XML embedded mode. XML mode enables tool calling for models without native function calling support, using `<tool_call>` XML tags in content. Configurable via `.set tool mode openai|xml`, `--toolcall-mode` CLI flag, and `config.json`
- **PostgreSQL storage** — persistent storage via PostgreSQL as an alternative to bbolt. Configurable via `.set db` sub-commands (host/port/dbname/user/password/enabled), with connection test and bbolt data migration support
- **Session persistence** — automatic conversation context restoration after program restart. Session data is saved after each LLM request and restored on startup
- **Loop detection** — monitors LLM streaming output for repetitive patterns, automatically stops and sends correction prompts when detected. Configurable via `.set loop-detect-enabled`, `--loop-detect-enabled`
- **Message deduplication** — feature-based duplicate message detection with Jaccard similarity comparison. Configurable via `.set dedup-enabled` and related parameters
- **Tool call confirmation** — all tool calls (not just execute_command) require user confirmation. Each tool has independent confirm-tool control. Added "G" option (agree and disable confirmation for this method). Number-based approval counter is per-method, reset on task completion
- **Model management UX** — `.model switch/remove/enable/disable/info/set-priority/set-param` commands show model list with number selection when no model ID is provided
- **DB sub-command mode** — `.set db enabled/host/port/dbname/user/password` sub-commands for database configuration. Interactive setup wizard on first `.set db` run
- **Default agent name** — program uses current working directory name (last segment) as default agent name
- **XML mode `<item>` tag** — unified array parameter tag naming in XML tool call mode, all array elements use `<item>` tag
- **Conversation timestamp format** — improved readability: changed from "2026-05-12 10:15:30 - " to "在 2026-05-12 10:15:30 说："
- **write_to_file tool enhancement** — added reminder to prefer replace_in_file over rewriting entire files to avoid introducing new issues
- **Model switch fix** — fixed `.model add/switch` not taking effect due to ModelManager and cfg.Models desynchronization
- **Qwen 3.6 infinite loop fix** — fixed infinite loop when writing large files with Qwen 3.6 model

### v0.4.0 — RC2

> **BUILD**: 00151 | **Release Date**: 2026-05-04

Release Candidate 2 — feature complete, stable and usable.

**版本亮点**: LLM 自主设置系统参数、表情符号角色区分输出、日志级别控制、文件工具增强、超时时间智能预判。

**Implemented Features:**

- **LLM settings tool** — LLM can modify system parameters via tool calls (equivalent to `.set`), with user confirmation for each change
- **Emoji role indicators** — distinct emoji prefixes for different output roles: 👤 user input, 🐚 LLM response, ⚙️ tool calls/results, 🔴 command execution. Configurable via `.set emoji-enabled`, `--emoji-enabled`, and `config.json`
- **Log level control** — `.set log debug/info/warn/error/off`, `--log-level` CLI flag, config.json persistence
- **Enhanced file tools** — improved read_file and write_file for better source code manipulation
- **Intelligent timeout** — LLM can pass `timeout_seconds` in tool calls, system takes max of user-configured minimum and LLM-predicted timeout
- **Agent identity defaults** — multi-language default agent descriptions for consistent behavior
- **Output mode refactoring** — fine-grained control: show-llm-thinking, show-llm-content, show-tool, show-tool-input, show-tool-output, show-command, show-command-output
- **Session display improvement** — message list numbering starts from 0 (system message = 0, user message 1 = 1, etc.)

### v0.3.0 — RC1

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

## Extended Features

### 🚀 co-shell-feishu-bridge — 飞书机器人集成

`co-shell-feishu-bridge` 是一个独立的网关程序，将飞书（Lark）机器人连接到 co-shell。通过飞书 WebSocket 长连接，用户可以在飞书聊天中直接向 co-shell 发送自然语言指令并接收 AI 处理结果。

**安全特性**：采用我方主动连接飞书 WebSocket 长链接的方式，无需暴露任何公网端口，无需配置反向代理或防火墙规则。

#### 快速启动

```bash
# 编译桥接器
go build -o work/co-shell-feishu-bridge ./cmd/co-shell-feishu-bridge/

# 启动（需先创建飞书应用并获取 App ID / App Secret）
./work/co-shell-feishu-bridge \
  --app-id cli_a5b3c4d5e6f7g8h9 \
  --app-secret a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

#### 三种工作模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `sync`（默认） | 逐条执行，前一条完成后才处理下一条 | 文件操作、状态依赖的任务 |
| `pool` | 当前任务完成后，合并队列中所有消息批量处理 | 快速连续提问 |
| `preempt` | 新消息到达时中断当前任务，立即处理新消息 | 需要快速响应的场景 |

> 详细文档：[docs/bridge/feishu.md](docs/bridge/feishu.md)

### 🗄️ PostgreSQL 持久化存储

co-shell 支持使用 PostgreSQL 作为持久化存储后端，替代默认的嵌入式 bbolt 数据库。适用于需要多进程共享数据、数据备份恢复、或已有 PostgreSQL 基础设施的场景。

#### 安装 PostgreSQL

**推荐方式：让 co-shell 自己安装！** 只需告诉它：

```bash
❯ 帮我安装 PostgreSQL 数据库
```

co-shell 会自动检测你的操作系统并执行相应的安装命令（macOS 使用 `brew install postgresql`，Ubuntu/Debian 使用 `apt install postgresql`，CentOS/RHEL 使用 `yum install postgresql-server`，Windows 使用 `winget install PostgreSQL.PostgreSQL`），安装完成后还会启动数据库服务并提示你进行后续配置。

**手动安装参考：**

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Ubuntu/Debian
sudo apt update && sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql

# Windows (PowerShell 管理员)
winget install PostgreSQL.PostgreSQL
```

#### 配置方式

```bash
# 交互式配置向导（推荐）
❯ .db config

# 或直接设置参数
❯ .set db host localhost
❯ .set db port 5432
❯ .set db name co_shell
❯ .set db user postgres
❯ .set db password your-password
❯ .set db enabled on
```

#### 数据管理

```bash
❯ .db init      # 初始化数据库（重建所有表）
❯ .db migrate   # 从本地 bbolt 迁移数据到 PostgreSQL
❯ .db backup    # 备份所有表到 CSV 文件
❯ .db restore   # 从备份恢复数据
```

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
├── bin/                 # Utility scripts (md2docx, md2wechat)
├── docs/                # Multi-language documentation
│   ├── en/              # English documentation
│   └── zh-CN/           # Chinese documentation
├── CHANGELOG.md         # Changelog
├── USAGE.md             # Detailed usage guide
└── ROADMAP.md           # Version plan and roadmap
```

---

## License

[MIT](LICENSE) © 2026 L.Shuang

---

## Author

- **L.Shuang** — [GitHub](https://github.com/idirect3d)
