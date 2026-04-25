# co-shell

> 智能命令行 Shell — 通过自然语言与 AI Agent 交互，智能编排和执行系统命令。

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-00029-blue)]()

---

## 简介

**co-shell** 是一个革命性的命令行工具，它让你可以用**自然语言**与操作系统交互。不再需要记忆复杂的命令参数，只需说出你想做什么，AI Agent 会自动理解、编排并执行相应的系统命令。

```bash
# 传统方式
$ find . -type f -name "*.go" | xargs wc -l | tail -1

# co-shell 方式
❯ 统计项目中所有 Go 文件的代码行数
```

### 核心特性

- 🗣️ **自然语言交互** — 用中文或英文直接描述任务
- 🤖 **AI Agent 驱动** — 智能理解意图、编排命令、执行并分析结果
- 🔧 **MCP 协议支持** — 接入丰富的 MCP 工具生态（文件系统、GitHub、数据库等）
- 💾 **持久化记忆** — Agent 可以记住你的偏好和历史上下文
- 📜 **会话历史** — 支持上下键翻页浏览历史命令
- ⚡ **流式输出** — 实时显示 AI 思考过程和命令执行结果
- 🔌 **可扩展** — 支持自定义规则、MCP Server、多模型切换

---

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/idirect3d/co-shell.git
cd co-shell

# 编译
go build -o co-shell .

# 运行
./co-shell
```

### 配置 API

首次启动会自动进入设置向导，或手动配置：

```bash
❯ .settings api-key sk-your-api-key-here
❯ .settings endpoint https://api.deepseek.com/v1
❯ .settings model deepseek-chat
```

### 开始使用

```bash
❯ 列出当前目录下所有文件
❯ 查找所有大于 100MB 的文件
❯ 看看磁盘还剩多少空间
❯ 帮我创建一个新的 Go 项目
```

---

## 命令行选项

```bash
co-shell [选项]                    启动交互式 REPL
co-shell [选项] <指令>             执行单条指令后退出

选项:
  -c, --config <path>    指定配置文件路径（默认: ~/.co-shell/config.json）
  -m, --model <name>     临时指定模型名称
  -e, --endpoint <url>   临时指定 API 端点
  -k, --api-key <key>    临时指定 API Key
      --log on|off       临时指定日志开关
      --max-iterations   最大迭代次数（-1 为不限制，默认 10）
  -v, --version          显示版本信息
  -h, --help             显示帮助信息
```

---

## 内置命令

所有内置命令以 `.` 开头，支持 Tab 自动补全。

| 命令 | 功能 |
|---|---|
| `.settings` | LLM API 参数管理（api-key / endpoint / model / temperature / max-tokens / max-iterations / show-thinking / show-command / show-output / log） |
| `.mcp` | MCP Server 管理（add / remove / list / enable / disable） |
| `.rule` | 全局规则管理（add / remove / clear） |
| `.memory` | 持久化记忆管理（save / get / search / delete / clear） |
| `.context` | 上下文管理（show / reset / set） |

---

## 技术栈

- **语言**: Go 1.22+
- **REPL**: [go-prompt](https://github.com/c-bata/go-prompt)
- **LLM**: [go-openai](https://github.com/sashabaranov/go-openai)（兼容 OpenAI / DeepSeek / 国产模型）
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **存储**: [bbolt](https://go.etcd.io/bbolt)（嵌入式 KV 数据库）

---

## 版本历史

### v0.1.0 — Alpha（当前版本）

> **BUILD**: 00033 | **最后更新**: 2026-04-25

首个 Alpha 预览版，核心功能可运行。

**已实现功能：**

- REPL 交互界面（go-prompt，Tab 补全）
- LLM 客户端抽象（OpenAI 兼容 API，流式输出支持）
- Agent 核心循环（LLM 调用 → 工具执行 → 迭代）
- 内置命令系统（.settings / .mcp / .rule / .memory / .context）
- 持久化存储（bbolt 记忆/上下文）
- MCP 客户端管理器（多 Server 连接）
- 系统命令执行（超时控制）
- 配置管理（JSON 持久化到 ~/.co-shell/）
- API 初始设置与设置向导
- 系统命令直接运行
- 日志系统（文件日志，支持运行时开关）
- API Key 脱敏显示
- 命令行参数支持（--help / --version / --model / --endpoint / --api-key / --log / --max-iterations / --lang）
- 会话历史管理（上下键翻页，跨会话持久化）
- 基础错误处理和用户提示
- 最大迭代次数可配置
- 国际化（i18n）支持中文/英文，--lang 参数，自动检测系统语言
- 多供应商支持（DeepSeek v4 / 阿里千问 / OpenAI 兼容兜底）

---

## 项目结构

```
co-shell/
├── main.go              # 程序入口，初始化所有模块
├── config/              # 配置管理（LLM/MCP/Rules）
├── repl/                # REPL 交互层
├── agent/               # Agent 核心循环
├── llm/                 # LLM 客户端抽象
├── mcp/                 # MCP 客户端管理器
├── store/               # 持久化存储（bbolt）
├── cmd/                 # 内置命令处理器
├── log/                 # 日志系统
├── USAGE.md             # 详细使用说明
└── ROADMAP.md           # 版本计划与路线图
```

---

## 许可证

[MIT](LICENSE) © 2026 L.Shuang

---

## 作者

- **L.Shuang** — [GitHub](https://github.com/idirect3d)