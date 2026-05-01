# co-shell

> 智能命令行 Shell — 通过自然语言与 AI Agent 交互，智能编排和执行系统命令。

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-00135-blue)]()
[![English](https://img.shields.io/badge/README-English-blue)](../en/README.md)




---

## 简介

**co-shell** 是一个万能命令——它既简单又复杂。

仅 10MB 左右的体积，不依赖其他环境和组件，却能够将电脑上几乎所有的功能编排进来，而你只需要一个指令。为了实现一个任务目标，它可以几乎不受限制地调用它所知道的一切命令。它是安全的——每次执行命令前都会征求你的同意；它是透明的——所有被执行的命令在执行前都会完整显示。

别忘了，它本身也只是一个命令，自己也可以调用自己——是不是拥有了无限可能？

> **你的想象力就是它的动力——指令即一切。**

不再需要记忆复杂的命令参数，只需说出你想做什么，co-shell会自动理解、编排并执行相应的系统命令、程序或脚本。


```bash
# 传统方式
$ find . -type f -name "*.go" | xargs wc -l | tail -1

# co-shell 方式
❯ 统计项目中所有 Go 文件的代码行数
```

### ⚠️ 风险声明

co-shell 是一个由大语言模型（LLM）驱动的智能命令行工具。AI 模型可能会生成并执行以下类型的危险命令：

- 删除文件或目录（如 `rm -rf /` 等）
- 格式化磁盘（如 `mkfs`、`format` 等）
- 修改系统关键配置
- 关闭或重启系统
- 下载并执行未知来源的程序
- 泄露敏感信息（如 API Key、密码、密钥等）

继续使用本程序即表示您已充分了解上述风险，并同意自行承担所有因使用本程序可能导致的任何损失或损害。开发者和发布者不承担任何责任。

> 首次启动时会显示完整声明并要求确认，确认后不再显示。

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

#### 方式一：直接下载二进制文件（推荐）

从 [Releases](https://github.com/idirect3d/co-shell/releases) 页面下载对应系统的压缩包，解压后即可运行：

| 操作系统 | 架构 | 下载 |
|---|---|---|
| macOS | Intel | [co-shell-v0.3.0-darwin-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-amd64.zip) |
| macOS | Apple Silicon | [co-shell-v0.3.0-darwin-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip) |
| Linux | x86_64 | [co-shell-v0.3.0-linux-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-amd64.zip) |
| Linux | ARM64 | [co-shell-v0.3.0-linux-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-arm64.zip) |
| Windows | x86_64 | [co-shell-v0.3.0-windows-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip) |
| Windows | ARM64 | [co-shell-v0.3.0-windows-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-arm64.zip) |

**macOS / Linux：**
```bash
# curl
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip
unzip co-shell.zip && rm co-shell.zip
chmod +x co-shell
./co-shell

# 或 wget
wget https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip
unzip co-shell-v0.3.0-darwin-arm64.zip && rm co-shell-v0.3.0-darwin-arm64.zip
chmod +x co-shell
./co-shell
```

**Windows（PowerShell）：**
```powershell
# PowerShell
Invoke-WebRequest -Uri https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip -OutFile co-shell.zip
Expand-Archive -Path co-shell.zip -DestinationPath .
.\co-shell.exe
```

**Windows（CMD）：**
```cmd
:: CMD
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip
tar -xf co-shell.zip
del co-shell.zip
co-shell.exe
```


#### 方式二：从源码编译

```bash
git clone https://github.com/idirect3d/co-shell.git
cd co-shell
go build -o co-shell .
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
  -w, --workspace <path>  工作区路径（默认：当前目录）
  -c, --config <path>     配置文件路径（默认：{workspace}/config.json）
  -m, --model <name>      临时指定模型名称
  -e, --endpoint <url>    临时指定 API 端点
  -k, --api-key <key>     临时指定 API Key
  -n, --name <name>       设置 Agent 名称（默认：co-shell）
  -i, --image <path>      图片文件路径，用于多模态输入
      --lang <code>       设置语言（zh/en，默认自动检测）
      --log on|off        临时指定日志开关
      --max-iterations N  最大迭代次数（-1 为不限制，默认 1000）
      --temperature N     温度参数（0.0 ~ 2.0）
      --max-tokens N      最大输出令牌数
      --show-thinking     显示 AI 思考过程（on/off）
      --show-command      显示执行的命令（on/off）
      --show-output       显示命令输出（on/off）
      --confirm-command   执行命令前需确认（on/off）
      --result-mode       结果处理模式（minimal/explain/analyze/free）
      --description       Agent 描述/专长
      --principles        Agent 核心原则
      --tool-timeout N    工具调用超时秒数（0=不限）
      --cmd-timeout N     命令执行超时秒数（0=不限）
      --llm-timeout N     LLM 请求超时秒数（0=不限）
      --vision            视觉识别能力（on/off）
  -v, --version           显示版本信息
  -h, --help              显示帮助信息
```

---

## 内置命令

所有内置命令以 `.` 开头，支持 Tab 自动补全。

| 命令 | 功能 |
|---|---|
| `.set` | LLM API 参数管理（api-key / endpoint / model / temperature / max-tokens / max-iterations / show-thinking / show-command / show-output / log / result-mode / name / description / principles / vision / tool-timeout / cmd-timeout / llm-timeout） |
| `.mcp` | MCP Server 管理（add / remove / list / enable / disable） |
| `.rule` | 全局规则管理（add / remove / clear） |
| `.memory` | 持久化记忆管理（save / get / search / delete / clear） |
| `.context` | 上下文管理（show / reset / set） |
| `.image` | 多模态图片缓存管理（add / remove / clear / list） |
| `.plan` | 任务计划管理（list / view / create / insert / remove / update） |
| `.wizard` | 重新启动 API 设置向导 |
| `.list` | 查看历史任务列表 |
| `.last` | 查看最近的历史任务 |
| `.first` | 查看最早的历史任务 |
| `.help` | 显示帮助信息 |
| `.exit` | 退出 co-shell |

---

## 成果样例

co-shell 可以自主进行深度调研，通过搜索网络、收集数据并生成综合报告。以下是 co-shell 完全自主生成的真实案例：

### 1. 北冰洋品牌深度调研报告

对中华老字号汽水品牌"北冰洋"的全面品牌调研报告，追溯其从 1936 年至今的发展历程，涵盖品牌沿革、产品体系演进、市场竞争态势及战略建议。

> **报告**: [arctic-ocean-brand-research-report.md](../samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.md)（267页）| [DOCX](../samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.docx)

### 2. 北京动物园大熊猫最新动态研究报告

关于北京动物园大熊猫群体最新动态的及时研究报告，涵盖场馆升级改造、新成员加入及 2023 年至 2026 年的公共事件。

> **报告**: [beijing-zoo-panda-latest-report.md](../samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.md)（196页）| [DOCX](../samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.docx)

### 3. 中国再保险市场 2026-2027 年调研报告

专业市场调研报告，分析 2026 年伊朗战争对中国再保险市场的影响，涵盖地缘政治风险、能源市场动荡及战略建议。

> **报告**: [china-reinsurance-market-report-2026-2027.md](../samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.md)（442页）| [DOCX](../samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.docx)

### 4. 中国运-10 飞机深度调研报告

关于中国首架自主研制喷气式客机运-10（Y-10）的深度调研报告，涵盖研发历史、技术参数及历史意义。

> **报告**: [Y-10-in-depth-research-report.md](../samples/research/yun10/Y-10-in-depth-research-report.md)（290页）| [DOCX](../samples/research/yun10/Y-10-in-depth-research-report.docx)

---

## 技术栈

- **语言**: Go 1.22+
- **REPL**: [go-prompt](https://github.com/c-bata/go-prompt)
- **LLM**: [go-openai](https://github.com/sashabaranov/go-openai)（兼容 OpenAI / DeepSeek / 国产模型）
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **存储**: [bbolt](https://go.etcd.io/bbolt)（嵌入式 KV 数据库）

---

## 版本历史

### v0.3.0 — RC1（当前版本）

> **BUILD**: 00135 | **发布日期**: 2026-05-01

发布候选版，功能完整，可预览。本版本引入了全面的上下文与记忆管理、输出模式切换、思考过程开关以及广泛的模型支持。

**已实现功能：**

- 多模态模型支持（图片输入、视觉理解），👀 标识
- 图片缓存管理（.image 命令，add_images/remove_images/clear_images 工具）
- Agent 身份自定义（name/description/principles 通过 .set 设置）
- 任务计划管理（.plan 命令，create_task_plan/update_task_step/insert_task_steps/remove_task_steps/view_task_plan/list_task_plans 工具）
- 任务计划单例模式——同一时间只能有一个活跃计划，完成后自动归档
- 批量命令执行，"本次都批准"选项可继承给子 agent
- 12 个新 CLI 参数（--temperature/--max-tokens/--show-thinking/--show-command/--show-output/--confirm-command/--result-mode/--description/--principles/--tool-timeout/--cmd-timeout/--llm-timeout）
- 视觉识别能力自动检测（通过模型 API）
- 增强的帮助文档，完整参数描述
- **对话上下文限制**（.set context-limit）——控制发送给 LLM 的历史消息数量
- **持久化记忆管理**（.memory 命令，get_memory_slice/memory_search 工具）
- **记忆功能开关**（.set memory-enabled，--memory-enabled/--memory-disabled）
- **LLM 输出模式**（.set output-mode）——精简 / 标准 / 调试三种模式
- **Sub-agent 开关**（.set subagent-enabled）——控制子 agent 工具可用性
- **思考过程开关**（.set thinking-enabled，--thinking-enabled/--thinking-disabled）——控制 AI 推理过程显示
- **Token 用量统计**——通过 Agent.TokenUsage() 累计追踪
- **对话重置**（.new 命令）——无需重启即可清空所有历史
- **错误重试限制**——可配置单错误和错误类型最大次数，超限提示用户
- **数字批准机制**——输入数字自动批准后续 N 次命令执行
- **search_files 增强**——忽略二进制文件、内容长度保护、可配置限制
- **即时生效**——.set 修改参数后无需重启
- **新增模型支持**——小米（Mi）、GLM（Z.ai）最新模型
- **设置向导增强**——内置供应商跳过地址设置、增强风险警示
- **成果样例**——新增真实使用案例

### v0.2.0 — Beta

> **BUILD**: 00087 | **发布日期**: 2026-04-27

Beta 测试版，功能完善，可日常使用。

**已实现功能：**

- Agent 内置文件操作工具（read_file、search_files、list_code_definition_names、replace_in_file、write_to_file）
- Workspace 架构（--workspace 参数，自动创建子目录）
- Agent 系统提示词多语言支持（根据 i18n 自动切换中英文）
- Sub-agent 支持（启动子进程作为影分身，共享 I/O，收集成果）
- 定时任务执行（类 crontab 表达式调度 sub-agent）
- 自定义配置文件路径（-c/--config 参数）
- 自定义 Agent 名称（--name/-n 参数）
- 多轮对话上下文管理
- 系统命令执行安全沙箱
- 危险操作命令确认机制
- 配置文件热重载
- 增强的错误处理和用户提示

### v0.1.0 — Alpha

> **BUILD**: 00074 | **发布日期**: 2026-04-26

首个 Alpha 预览版，核心功能可运行。

**已实现功能：**

- REPL 交互界面（go-prompt，Tab 补全）
- LLM 客户端抽象（OpenAI 兼容 API，流式输出支持）
- Agent 核心循环（LLM 调用 → 工具执行 → 迭代）
- 内置命令系统（.set / .mcp / .rule / .memory / .context / .list / .last / .first / .wizard）
- 持久化存储（bbolt 记忆/上下文）
- MCP 客户端管理器（多 Server 连接）
- 系统命令执行（超时控制，命令确认机制）
- 配置管理（JSON 持久化到 ~/.co-shell/，多位置加载）
- API 初始设置与设置向导（多供应商支持）
- 系统命令直接运行
- 日志系统（文件日志，支持运行时开关）
- API Key 脱敏显示
- 命令行参数支持（--help / --version / --model / --endpoint / --api-key / --log / --max-iterations / --lang）
- 会话历史管理（上下键翻页，跨会话持久化，.list/.last/.first 命令）
- 国际化（i18n）支持中文/英文，--lang 参数，自动检测系统语言
- 多供应商支持（DeepSeek v4 / 阿里千问 / OpenAI 兼容兜底）
- 结果处理模式（minimal / explain / analyze / free）
- 超时时间参数化配置
- 跨平台支持（macOS / Linux / Windows）
- 流式输出与思考过程显示


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
├── i18n/                # 国际化
├── wizard/              # 设置向导
├── scheduler/           # 定时任务调度器
├── subagent/            # Sub-agent 管理
├── taskplan/            # 任务计划管理
├── memory/              # 持久化记忆管理
├── workspace/           # 工作区管理
├── samples/             # 成果样例
├── bin/                 # 二进制输出目录
├── output/              # 输出目录
├── CHANGELOG.md         # 变更日志
├── USAGE.md             # 详细使用说明
├── docs/                # 多语言文档
│   └── en/              # 英文文档
└── ROADMAP.md           # 版本计划与路线图
```

---

## 许可证

[MIT](LICENSE) © 2026 L.Shuang

---

## 作者

- **L.Shuang** — [GitHub](https://github.com/idirect3d)