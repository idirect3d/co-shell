# System Prompt 组成说明

> 本文档详细说明 co-shell 发送给 LLM 的 system message（系统提示词）的组成结构、各部分的来源、构建逻辑以及动态变化机制。
>
> **用途**: 为 FIX-181（System Prompt 重构）提供基础分析材料。

---

## 1. 概述

System Prompt 是发送给 LLM 的第一条消息（role=system），用于定义 AI 助手的身份、能力、行为规则和环境信息。co-shell 的 system prompt 由 `agent/system_prompt.go` 中的 `buildSystemPromptWithMode()` 函数构建，支持多语言（zh/en）和外部文件覆盖。

### 1.1 构建入口

```
buildSystemPrompt(rules string)                          → 默认模式（ResultModeMinimal）
    └── buildSystemPromptWithMode(rules, mode, name, desc, principles, userName, channel)
```

调用链路：

```
Agent.New() / Agent.SetConfig() / Agent.SetResultMode() / Agent.rebuildSystemPrompt()
    └── buildSystemPromptWithMode(rules, mode, agentName, agentDescription, agentPrinciples, userName, channel)
```

### 1.2 最终消息结构

发送给 LLM 的 messages 数组布局：

```
[0] system  →  System Prompt（本文档描述的内容）
[1..n-2]    →  历史对话消息（受 ContextLimit 和 messagePointer 控制）
[n-1]       →  当前用户输入
```

---

## 2. System Prompt 的组成结构

System Prompt 由以下 **8 个部分**按顺序拼接而成。每个部分在最终发送的 system message 中都有对应的**章节标题**（如 `IDENTITY`、`TOOL USE`、`CAPABILITIES`、`RULES`、`OBJECTIVE` 等），这些标题同时也是 i18n key 中定义的固定文本。

### 2.0 整体布局概览

最终发送给 LLM 的 system message 的完整文本结构如下（以中文版为例）。各大部分之间以 `\n====\n` 作为分隔符：

```
IDENTITY
# 你的身份
...（Part 1: 身份信息）
TOOL USE
# 工具使用指南
...（Part 2: 工具使用指南 - 每个工具的详细说明、示例）
RESULT MODE
结果处理模式:
...（Part 3: 结果处理模式）
CAPABILITIES
你拥有以下能力:
...（Part 4: 能力描述）
RULES
## 重要规则:
...（Part 5: 行为规则）
OBJECTIVE
...（Part 6: 任务执行方法论 + Prompt Injection 防护）
当前环境:
...（Part 7: 静态环境信息）
自定义:
...（Part 8: 自定义规则，可选）
## 动态环境信息
...（Part 9: 动态环境信息）
```

### 2.1 各部分与 Cline 概念的映射

co-shell 的 system prompt 设计参考了 Cline 等类似 AI 编码助手的 prompt 结构。以下是各部分与 Cline 概念的对应关系：

| Cline 概念 | co-shell 对应部分 | 说明 |
|------------|-------------------|------|
| **IDENTITY** | Part 1: 身份信息 | Agent 名称、描述、核心原则 |
| **TOOL USE** | Part 2: 工具使用指南 | 工具列表、用途说明、使用示例 |
| **Tool Use Examples** | Part 2 中每个工具的 `例如：/ Example:` 块 | 每个工具定义后附带的 JSON 调用示例 |
| **Tool Use Guidelines** | Part 2 开头的 `# Tool Use Formatting` | 工具调用的通用规则（独立操作可并行、依赖操作需顺序等） |
| **UPDATING TASK PROGRESS** | Part 5 中的 `# Task Planning & Tracking (Checklist System)` | 任务规划、步骤跟踪、动态调整的完整规则 |
| **EDITING FILES** | Part 5 中的文件操作规则 + Part 2 中 replace_in_file/write_to_file 的详细说明 | 文件修改的优先级（replace_in_file 优先于 write_to_file）、SEARCH/REPLACE 格式要求 |
| **CAPABILITIES** | Part 4: 能力描述 | 系统能力清单（执行命令、调用工具、读写文件等） |
| **RULES** | Part 5: 行为规则 | 通用行为约束和最佳实践 |
| **OBJECTIVE** | Part 6: 任务执行方法论 | 迭代式任务分解和执行的完整方法论 |
| **ENVIRONMENT** | Part 7: 静态环境信息 | 当前环境上下文（平台、Shell、目录等） |

### 2.2 各部分详细说明


```
┌─────────────────────────────────────────────────────────────┐
│  Part 1: 身份信息 (Identity)                                 │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ ## 你的身份 / ## Your Identity                        │   │
│  │                                                       │   │
│  │ 你是 <agentName>，一个智能命令行助手...                │   │
│  │                                                       │   │
│  │ <agentDescription>                                    │   │
│  │                                                       │   │
│  │ <agentPrinciples>                                     │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 2: 工具使用指南 (Tool Usage Guide)                       │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ ## 工具使用指南 / ## Tool Usage Guide                 │   │
│  │                                                       │   │
│  │ 你通过以下工具与系统交互。每个工具都有特定的用途       │   │
│  │ 和使用方式：                                          │   │
│  │                                                       │   │
│  │ ### execute_command — 执行系统命令                    │   │
│  │ - 用于运行 Shell 命令、脚本或任何 CLI 工具            │   │
│  │ - ...                                                │   │
│  │                                                       │   │
│  │ ### read_file — 读取文件内容                          │   │
│  │ - ...                                                │   │
│  │                                                       │   │
│  │ ### MCP 工具                                          │   │
│  │ - 通过 MCP 协议连接的外部工具                         │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 3: 结果处理模式 (Result Mode)                           │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ 结果处理模式: / RESULT PROCESSING MODE:               │   │
│  │ <resultModeInstruction>                               │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 4: 能力描述 (Capabilities)                              │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ 你拥有以下能力: / You have the following capabilities: │   │
│  │ 1. 执行系统命令 (<shellName>)                         │   │
│  │ 2. 调用 {cwd}/bin/ 下的工具                           │   │
│  │ 3. 调用 MCP 工具                                      │   │
│  │ 4. 读写文件                                           │   │
│  │ 5. 搜索历史记忆 / 获取历史记忆片段                    │   │
│  │ 6. 复杂任务管理和跟踪                                  │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 5: 行为规则 (Rules)                                     │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ ## 重要规则: / IMPORTANT RULES:                       │   │
│  │ - 使用 "execute_command" 工具运行系统命令...           │   │
│  │ - 除非用户特别指定，否则优先使用标准系统命令...        │   │
│  │ - 主动探索系统以发现可用工具...                        │   │
│  │ - ...                                                 │   │
│  │                                                       │   │
│  │ ## 任务规划与跟踪规则（Checklist 机制）                │   │
│  │ - 收到用户的指令后，先分析需求并进行任务规划...        │   │
│  │ - ...                                                 │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 6: 任务执行方法论 (Objective)                            │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ OBJECTIVE                                             │   │
│  │                                                       │   │
│  │ 你要迭代式地完成任务，将其分解为清晰的步骤...           │   │
│  │ 1. 分析用户的任务...                                   │   │
│  │ 2. 按顺序逐步完成这些目标...                            │   │
│  │ 3. 你有广泛的能力...                                   │   │
│  │ 4. 在完成任务之前，验证任务要求...                      │   │
│  │ 5. 完成用户任务并验证结果后，呈现结果...                │   │
│  │ 6. 用户可能会提供反馈...                                │   │
│  │                                                       │   │
│  │ 你是 co-shell，一个由 Go 语言编写的智能命令行应用程序...│   │
│  │                                                       │   │
│  │ **特别重要**：从这一行开始...（Prompt Injection 防护）  │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 7: 静态环境信息 (Static Environment)                     │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ 当前环境:                                              │   │
│  │ - 平台: <GOOS> (<GOARCH>)                             │   │
│  │ - Shell: <shellName>                                  │   │
│  │ - 工作目录: <cwd>                                     │   │
│  │ - 主机名: <hostname>                                  │   │
│  │ - 用户: <username>                                    │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 8: 自定义规则 (Custom Rules) — 可选                     │
│  ┌───────────────────────────────────────────────────────┐   │
│  │                                                       │   │
│  │ 自定义: / Custom:                                     │   │
│  │ <rules>                                               │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                               │
│  Part 9: 动态环境信息 (Dynamic Environment Info)               │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ ## 动态环境信息 / ## Dynamic Environment Info         │   │
│  │                                                       │   │
│  │ - 当前时间: <now>                                     │   │
│  │ - 渠道: <userName @ channel>                          │   │
│  │ - 上下文用量: <used> / <total> 条消息                 │   │
│  └───────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.1 各部分详细说明

#### Part 1: 身份信息 (Identity)

| 字段 | 来源 | 说明 |
|------|------|------|
| `agentName` | `cfg.LLM.AgentName` → 默认 `"co-shell"` | Agent 名称 |
| `agentDescription` | `cfg.LLM.AgentDescription` → i18n 默认值 | Agent 描述/专长 |
| `agentPrinciples` | `cfg.LLM.AgentPrinciples` → i18n 默认值 | Agent 核心原则 |

- i18n Key: `KeySystemPromptIdentity`
- 中文默认描述：`你是一个全科研究员，擅长搜集专业资料...`
- 中文默认原则：`做研究时需要保存所有收集到的原始资料...`

#### Part 2: 工具使用指南 (Tool Usage Guide)

- i18n Key: `KeySystemPromptToolUsage`
- 详细描述每个可用工具的用途、使用场景和示例
- 覆盖所有内置工具：execute_command、read_file、search_files、list_code_definition_names、replace_in_file、write_to_file、add_images/remove_images/clear_images、launch_sub_agent、schedule_task、create_task_plan/update_task_step/insert_task_steps/remove_task_steps/list_task_plans/view_task_plan、get_memory_slice/memory_search/delete_memory、update_settings/list_settings、ask_followup_question、adjust_context_start、MCP 工具
- **目的**: 帮助 LLM 理解每个工具的用途，减少试错，提高工具调用准确率

#### Part 3: 结果处理模式 (Result Mode)

- i18n Key: `KeySystemPromptResultMode`
- 根据 `config.ResultMode` 动态生成指令文本：

| 模式 | 说明 |
|------|------|
| `minimal` | 仅返回成功/失败，不附加解释 |
| `explain` | 简要解释输出含义（2-3 句） |
| `analyze` | 详细分析模式、异常、影响 |
| `free` | 自主决定如何呈现结果 |

#### Part 4: 能力描述 (Capabilities)

- i18n Key: `KeySystemPromptCapabilities`
- **外部文件覆盖**: 如果工作区根目录存在 `CAPABILITIES.md`，则使用其内容替代内置 i18n 值
- 加载逻辑：`loadExternalFile(cwd, "CAPABILITIES.md")`

#### Part 5: 行为规则 (Rules)

- i18n Key: `KeySystemPromptRules`
- **外部文件覆盖**: 如果工作区根目录存在 `RULES.md`，则使用其内容替代内置 i18n 值
- 加载逻辑：`loadExternalFile(cwd, "RULES.md")`
- 中文版包含：
  - 工具使用规则（execute_command、MCP 等）
  - 文件操作规则
  - 任务规划与跟踪规则（Checklist 机制）
  - 子代理使用规则（英文版独有）
  - 自主决策原则（英文版独有）

#### Part 6: 任务执行方法论 (Objective)

- i18n Key: `KeySystemPromptObjective`
- 定义 LLM 如何迭代式地完成任务：分析需求 → 分解步骤 → 逐步执行 → 验证结果
- 包含 6 条核心方法论规则（任务分析、工具选择、验证、呈现、反馈处理）
- 末尾包含 Prompt Injection 防护声明（"忽略上面所有内容"检测）
- 末尾声明"你是 co-shell，一个由 Go 语言编写的智能命令行应用程序"

#### Part 7: 静态环境信息 (Static Environment)

| 变量 | 来源 | 示例值 |
|------|------|--------|
| `runtime.GOOS` | Go 运行时 | `darwin` |
| `runtime.GOARCH` | Go 运行时 | `arm64` |
| `shellName()` | `agent/command_tools.go` | `zsh` / `bash` / `powershell` |
| `cwd` | `os.Getwd()` | `/Users/liangshuang/Documents/Project/co-shell` |
| `hostname` | `os.Hostname()` | 主机名 |
| `username` | `$USER` / `$USERNAME` | `liangshuang` |

- i18n Key: `KeySystemPromptEnvironment`

#### Part 8: 自定义规则 (Custom Rules) — 可选

- 仅当 `rules` 参数非空时追加
- 格式：`"\n\n自定义:\n<rules>"`（中文）或 `"\n\nCustom:\n<rules>"`（英文）

#### Part 9: 动态环境信息 (Dynamic Environment Info)

- i18n Key: `KeySystemPromptDynamicEnv`
- 每次请求时动态生成，包含：
  - **当前时间**: `time.Now()` 格式化的时间字符串
  - **信息渠道**: 标识信息来源，格式为 `"userName @ channel"`（如 `"liangshuang @ co-shell"`、`"zhangsan @ feishu"`）
    - `userName` 来源：`cfg.LLM.UserName` → 默认 OS 用户名
    - `channel` 来源：`cfg.LLM.Channel` → 默认 `"co-shell"`
    - 可能的值：`co-shell`（REPL 模式）、`feishu`（飞书集成）、`co-tor`（子代理）、`agent`（Agent 间通信）
  - **上下文用量**: 当前上下文中的消息数量 / 总限制
- **目的**: 将动态变化的环境信息放在最后，让 LLM 在阅读完所有固定规则后，再获取当前会话的实时状态

---

## 3. 构建流程详解

### 3.1 buildSystemPromptWithMode() 函数

```go
func buildSystemPromptWithMode(
    rules string,           // 用户自定义规则（来自 .rule add）
    mode config.ResultMode, // 结果处理模式
    agentName string,       // Agent 名称（来自配置）
    agentDescription string,// Agent 描述（来自配置）
    agentPrinciples string, // Agent 原则（来自配置）
    userName string,        // 用户名（来自配置，默认 OS 用户名）
    channel string,         // 通信渠道（co-shell / feishu / co-tor / agent）
) string
```

**执行步骤**:

1. **收集环境信息**: GOOS、GOARCH、shell、时间、工作目录、主机名、用户名
2. **构建身份信息**: `i18n.TF(KeySystemPromptIdentity, name, desc, principles)` → Part 1
3. **构建工具使用指南**: `i18n.T(KeySystemPromptToolUsage)` → Part 2
4. **构建结果模式指令**: `resultModeInstruction(mode)` → Part 3
5. **加载能力描述**: 尝试加载 `CAPABILITIES.md`，失败则使用 i18n → Part 4
6. **加载行为规则**: 尝试加载 `RULES.md`，失败则使用 i18n → Part 5
7. **构建任务执行方法论**: `i18n.T(KeySystemPromptObjective)` → Part 6
8. **构建静态环境信息**: `i18n.TF(KeySystemPromptEnvironment, GOOS, GOARCH, shell, cwd, hostname, username)` → Part 7
9. **拼接 Parts 1-7**: 各大部分之间以 `\n====\n` 分隔：`identity + sep + toolUsage + sep + resultMode + sep + capabilities + sep + rulesText + sep + objectiveText + sep + envText`
10. **追加自定义规则**: 如果 `rules != ""`，以 `sep` 分隔追加 Part 8（格式：`"\n====\n自定义:\n<rules>"`）
11. **构建渠道信息**: 根据 `userName` 和 `channel` 参数拼接渠道标识（格式：`"userName @ channel"`），默认 `"Anonymous @ co-shell"`
12. **构建动态环境信息**: 以 `sep` 分隔追加 Part 9：`i18n.TF(KeySystemPromptDynamicEnv, now, channelInfo)`

### 3.2 外部文件覆盖机制

```go
func loadExternalFile(workspacePath, filename string) string
```

- 从 `os.Getwd()` 获取的工作区根目录加载文件
- 文件不存在或读取失败时返回空字符串
- 支持的文件：`CAPABILITIES.md`（覆盖 Part 3）、`RULES.md`（覆盖 Part 4）

### 3.3 触发重建的时机

| 触发点 | 调用函数 | 说明 |
|--------|----------|------|
| `Agent.New()` | `buildSystemPrompt(rules)` | 首次创建 Agent |
| `Agent.SetConfig(cfg)` | `rebuildSystemPrompt()` | 配置变更（含身份信息） |
| `Agent.SetResultMode(mode)` | `buildSystemPromptWithMode(...)` | 结果模式变更，同时重置对话历史 |
| `rebuildSystemPrompt()` | `buildSystemPromptWithMode(...)` | 内部调用，保留对话历史 |

---

## 4. 多语言支持

### 4.1 语言选择

- 优先级：`--lang` CLI 参数 > `LANG`/`LC_ALL` 环境变量 > 默认中文
- 所有 system prompt 内容通过 `i18n.T()` / `i18n.TF()` 获取翻译

### 4.2 中英文差异

| 部分 | 中文 (zh) | 英文 (en) |
|------|-----------|-----------|
| 身份标题 | `## 你的身份` | `## Your Identity` |
| 工具使用指南标题 | `## 工具使用指南` | `## Tool Usage Guide` |
| 结果模式标题 | `结果处理模式:` | `RESULT PROCESSING MODE:` |
| 能力标题 | `你拥有以下能力:` | `You have the following capabilities:` |
| 规则标题 | `## 重要规则:` | `IMPORTANT RULES:` |
| 环境标题 | `你是 co-shell，一个由 Go 语言编写的智能命令行应用程序...` | `You are co-shell, a Go-powered intelligent command-line application...` |
| 动态环境信息标题 | `## 动态环境信息` | `## Dynamic Environment Info` |
| 自定义标题 | `自定义:` | `Custom:` |
| 英文版独有内容 | 无 | `## Autonomy Principles`、`## Sub-agent Creation Principles` |

---

## 5. 与 messages 中其他消息的关系

### 5.1 消息索引前缀

`buildContextMessages()` 在发送给 LLM 前，会为每条非 system 消息添加索引前缀：

```
格式: "<index>: <content>"
示例: "123: 在 2026-05-01 12:09:24 说：现在来更新主报告。"
```

- System 消息（index 0）不加前缀
- 带 tool_calls 的 assistant 消息不加前缀
- 索引帮助 LLM 理解对话顺序（尤其在上下文截断时）

### 5.2 上下文截断

`buildContextMessages()` 根据 `ContextLimit` 和 `messagePointer` 控制发送给 LLM 的消息数量：

| ContextLimit | 行为 |
|---|---|
| `-1` | 发送所有消息（不截断） |
| `0` | 仅发送 system prompt + 当前用户输入 |
| `N > 0` | 发送 system prompt + 最近 N 条历史 + 当前用户输入 |

### 5.3 messagePointer 机制

- `messagePointer` 标记上下文起始位置
- 当 task plan 被创建/更新时，pointer 移动到最新位置，忽略之前的对话
- 在 `smart` 模式下，LLM 可通过 `adjust_context_start` 工具动态调整 pointer

---

## 6. 当前问题分析（FIX-181 背景）

### 6.1 结构性问题

1. **身份信息与环境信息分离**: Part 1（身份）在 Part 2（环境）之前，但 Part 2 开头又重复声明"你是 co-shell"，造成身份信息分散在两处
2. **中英文内容不一致**: 英文版有 `## Autonomy Principles` 和 `## Sub-agent Creation Principles`，中文版没有
3. **能力描述硬编码**: 能力列表在 i18n 中硬编码，与实际的工具开关（memoryEnabled、planEnabled、subAgentEnabled）不同步
4. **规则内容膨胀**: 行为规则部分包含了 Checklist 机制、子代理使用等大量说明性内容

### 6.2 动态性问题

1. **工具使用指南与 tools 参数双重传递**: Part 2 在 system prompt 中详细描述了每个工具的用途，同时 tools 参数又通过 function calling 传递了工具定义。两者内容重叠但格式不同，可能导致 LLM 混淆
2. **外部文件覆盖粒度粗**: `CAPABILITIES.md` 和 `RULES.md` 是整体替换，不能部分覆盖
3. **身份信息变更需重建**: 修改 Agent 名称/描述/原则需要重建整个 system prompt

### 6.3 构建流程问题

1. **拼接顺序固定**: 身份信息始终在最前面，无法根据场景调整
2. **无缓存机制**: 每次 `rebuildSystemPrompt()` 都重新拼接所有字符串
3. **无版本控制**: system prompt 内容变更没有版本标识，难以追踪 LLM 行为变化

---

## 7. 关键代码位置索引

| 功能 | 文件 | 行号 | 说明 |
|------|------|------|------|
| `buildSystemPrompt()` | `agent/system_prompt.go` | 42-44 | 默认模式入口 |
| `buildSystemPromptWithMode()` | `agent/system_prompt.go` | 71-150 | 核心构建函数 |
| `loadExternalFile()` | `agent/system_prompt.go` | 48-58 | 外部文件加载 |
| `resultModeInstruction()` | `agent/system_prompt.go` | 153-167 | 结果模式指令生成 |
| `rebuildSystemPrompt()` | `agent/agent.go` | 275-299 | 触发重建 |
| `buildContextMessages()` | `agent/loop.go` | 771-815 | 上下文消息构建 |
| `addIndexPrefixToMessages()` | `agent/loop.go` | 825-854 | 索引前缀添加 |
| `shellName()` | `agent/command_tools.go` | - | Shell 名称检测 |
| i18n 系统提示词 Key | `i18n/i18n.go` | 456-477 | Key 常量定义 |
| 中文翻译 | `i18n/zh.go` | 505-557 | 中文 system prompt 内容 |
| 英文翻译 | `i18n/en.go` | 502-563 | 英文 system prompt 内容 |
| `KeySystemPromptToolUsage` | `i18n/i18n.go` | 471 | 工具使用指南 Key |
| `KeySystemPromptDynamicEnv` | `i18n/i18n.go` | 473 | 动态环境信息 Key |
| 工具使用指南中文内容 | `i18n/zh.go` | 558-850 | Part 2 中文翻译 |
| 工具使用指南英文内容 | `i18n/en.go` | 564-793 | Part 2 英文翻译 |
| 动态环境信息中文内容 | `i18n/zh.go` | 852-857 | Part 9 中文翻译 |
| 动态环境信息英文内容 | `i18n/en.go` | 795-800 | Part 9 英文翻译 |
| `Agent.New()` | `agent/agent.go` | 48-67 | 首次构建 |
| `Agent.SetConfig()` | `agent/agent.go` | 246-250 | 配置变更触发重建 |
| `Agent.SetResultMode()` | `agent/agent.go` | 451-474 | 结果模式变更触发重建 |
