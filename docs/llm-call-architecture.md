# LLM 调用架构说明

> 本文档详细说明 co-shell 中所有调用大模型（LLM）的地方，以及这些调用所使用的参数来源和传递链路。

---

## 1. 概述

co-shell 的 LLM 调用架构分为三层：

```
┌─────────────────────────────────────────────────────────────┐
│                    调用层 (Callers)                          │
│  agent/loop.go  │  cmd/model.go  │  cmd/settings.go        │
│  agent/settings_tools.go  │  repl/repl.go  │  main.go       │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   客户端层 (Client)                          │
│  llm/client.go  —  openAIClient                             │
│  Chat() / ChatStream() / ListModels() / Test*()             │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   配置层 (Config)                            │
│  config/config.go  —  LLMConfig / ModelConfig               │
│  config/model_template.go  —  ModelManager / ModelTemplate  │
└─────────────────────────────────────────────────────────────┘
```

- **配置层**: 定义所有 LLM 相关参数的结构体和默认值
- **客户端层**: 实现与 LLM API 的 HTTP 通信，构建请求体
- **调用层**: 实际发起 LLM 调用的各个模块

---

## 2. LLM 客户端接口

文件: `llm/client.go`

### 2.1 Client 接口

```go
type Client interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)
    ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error)
    ListModels(ctx context.Context) ([]ModelInfo, error)
    TestVisionSupport(ctx context.Context) bool
    TestTextSupport(ctx context.Context) bool
    TestToolCallSupport(ctx context.Context) bool
    TestThinkingSupport(ctx context.Context) bool
    SetThinkingEnabled(enabled bool)
    SetReasoningEffort(effort string)
    SetTopP(topP float64)
    SetTopK(topK int)
    SetRepetitionPenalty(penalty float64)
    SetTokenUsage(mode string)
    SetBodyAdditions(additions map[string]string)
    RemoveBodyAddition(key string)
    GetBodyAdditions() map[string]string
    Close() error
}
```

### 2.2 实现类: openAIClient

`openAIClient` 是 `Client` 接口的唯一实现，内部维护以下状态：

| 字段 | 类型 | 说明 |
|---|---|---|
| `baseURL` | string | API 端点地址 |
| `apiKey` | string | API 密钥 |
| `model` | string | 模型名称 |
| `temperature` | float64 | 温度参数 (-1=不发送) |
| `maxTokens` | int | 最大输出令牌数 (-1=不发送) |
| `topP` | float64 | Top-P 采样 (-1=不发送) |
| `topK` | int | Top-K 采样 (-1=不发送) |
| `repetitionPenalty` | float64 | 重复惩罚 (-1=不发送) |
| `thinkingEnabled` | bool | 思考模式开关 |
| `reasoningEffort` | string | 推理努力程度 |
| `tokenUsage` | string | Token 用量显示模式 |
| `bodyAdditions` | map[string]string | 自定义请求体属性 |

### 2.3 请求体构建

`Chat()` 和 `ChatStream()` 方法构建的请求体 JSON 结构：

```json
{
  "model": "<model>",
  "messages": [...],
  "temperature": <float>,        // 仅当 >= 0 时发送
  "max_tokens": <int>,           // 仅当 >= 0 时发送
  "top_p": <float>,              // 仅当 >= 0 时发送
  "top_k": <int>,                // 仅当 >= 0 时发送
  "repetition_penalty": <float>, // 仅当 >= 0 时发送
  "tools": [...],                // 仅当有工具定义时发送
  "stream": <bool>,              // ChatStream 时为 true
  "stream_options": {
    "include_usage": <bool>      // tokenUsage != "none" 时发送
  },
  "thinking": {                  // 仅 thinkingEnabled && isThinkingModel 时发送
    "type": "enabled"
  },
  "reasoning_effort": "<string>",// 同上
  // ... bodyAdditions 合并的自定义属性
}
```

---

## 3. 参数来源与传递链路

### 3.1 参数层级

LLM 调用参数有三个层级，优先级从高到低：

```
模型级参数 (ModelConfig)  ← 最高优先级
    ↓ 回退
全局级参数 (LLMConfig)    ← 中间优先级
    ↓ 回退
默认值 (DefaultConfig)    ← 最低优先级
```

### 3.2 参数定义位置

#### 全局参数 (config.LLMConfig)

文件: `config/config.go`

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Temperature` | float64 | 0.0 | 温度参数 |
| `MaxTokens` | int | 8192 | 最大输出令牌数 |
| `TopP` | float64 | -1 | Top-P (-1=不发送) |
| `TopK` | int | -1 | Top-K (-1=不发送) |
| `RepetitionPenalty` | float64 | -1 | 重复惩罚 (-1=不发送) |
| `ThinkingEnabled` | bool | false | 思考模式 |
| `ReasoningEffort` | string | "low" | 推理努力程度 |
| `TokenUsage` | string | "off" | Token 用量显示模式 |
| `LLMTimeout` | int | 60 | LLM 请求超时秒数 |
| `BodyAdditions` | map[string]string | nil | 自定义请求体属性 |

#### 模型级参数 (config.ModelConfig)

文件: `config/config.go`

| 字段 | 类型 | 说明 |
|---|---|---|
| `Temperature` | *float64 | nil=使用全局值 |
| `MaxTokens` | *int | nil=使用全局值 |
| `TopP` | *float64 | nil=使用全局值 |
| `TopK` | *int | nil=使用全局值 |
| `RepetitionPenalty` | *float64 | nil=使用全局值 |
| `ThinkingEnabled` | *bool | nil=使用全局值 |
| `ReasoningEffort` | *string | nil=使用全局值 |
| `CustomParams` | map[string]interface{} | 自定义参数，合并到 bodyAdditions |

### 3.3 参数传递链路

```
config.json
    │
    ▼
config.LoadWithPath() / config.LoadFromFile()
    │
    ▼
config.Config  ← 包含 LLMConfig 和 []*ModelConfig
    │
    ├── main.go: 选择最高优先级模型 → 解析模型级参数 → llm.NewClient()
    │       │
    │       ▼
    │   llm.Client 实例 (openAIClient)
    │       │
    │       ├── agent.New(client, ...) → Agent.llm
    │       │       │
    │       │       ├── agent.Run() → llm.Chat()
    │       │       └── agent.RunStream() → llm.ChatStream()
    │       │
    │       ├── cmd/model.go: 能力检测 → llm.TestVisionSupport() / TestToolCallSupport() / TestThinkingSupport()
    │       │
    │       └── cmd/settings.go: 重建客户端 → llm.NewClient()
    │
    ├── agent/loop.go: 动态模型切换
    │       │
    │       ├── selectModelForCall() → 根据任务选择模型
    │       └── switchToModel() → 重建 llm.Client
    │
    └── agent/settings_tools.go: LLM 工具调用 → 重建客户端
```

---

## 4. 所有 LLM 调用点详细说明

### 4.1 Agent 核心循环 — agent/loop.go

这是最主要的 LLM 调用点，负责处理用户输入并生成回复。

#### 4.1.1 Run() — 非流式调用

```go
func (a *Agent) Run(ctx context.Context, userInput string) (string, error)
```

- **调用位置**: `agent/loop.go`
- **调用方法**: `a.llm.Chat(ctx, messages, tools)`
- **参数来源**:
  - `model`、`temperature`、`maxTokens` 等来自 `a.llm` (openAIClient 实例)
  - `messages` 来自 Agent 维护的对话上下文 (`a.messages`)
  - `tools` 来自 Agent 注册的工具列表 (`a.tools`)
- **调用时机**: 当 `output-mode` 为非流式模式时使用
- **调用频率**: 每次用户输入一次，可能多次迭代

#### 4.1.2 RunStream() — 流式调用

```go
func (a *Agent) RunStream(ctx context.Context, userInput string, callback func(string, string)) (string, error)
```

- **调用位置**: `agent/loop.go`
- **调用方法**: `a.llm.ChatStream(ctx, messages, tools)`
- **参数来源**: 同 `Run()`
- **调用时机**: 默认模式，流式输出 LLM 回复
- **调用频率**: 每次用户输入一次，可能多次迭代

#### 4.1.3 streamLLMResponse() — 流式响应处理

```go
func (a *Agent) streamLLMResponse(ctx context.Context, messages []llm.Message, tools []llm.Tool) (<-chan llm.StreamEvent, error)
```

- **调用位置**: `agent/loop.go`
- **调用方法**: `a.llm.ChatStream(ctx, messages, tools)`
- **参数来源**: 同 `RunStream()`
- **特殊逻辑**: 调用前会通过 `selectModelForCall()` 动态选择模型

#### 4.1.4 动态模型切换

```go
func (a *Agent) selectModelForCall(messages []llm.Message, tools []llm.Tool) *config.ModelConfig
func (a *Agent) switchToModel(model *config.ModelConfig)
```

- **位置**: `agent/agent.go`
- **逻辑**:
  1. `selectModelForCall()` 根据当前消息和工具判断是否需要切换模型
     - 如果消息包含图片 → 选择支持 Vision 的最高优先级模型
     - 如果启用了 thinking → 选择支持 Thinking 的最高优先级模型
     - 否则返回 nil (使用当前模型)
  2. `switchToModel()` 使用目标模型的参数重建 `llm.Client`
     - 模型级参数优先，未设置时回退到全局 `cfg.LLM`

### 4.2 模型管理向导 — cmd/model.go

#### 4.2.1 端点连通性测试

```go
client := llm.NewClient(endpoint, "", "test", 0, 0, 10)
models, err := client.ListModels(ctx)
```

- **参数来源**: 用户输入的 `endpoint`，硬编码的 `apiKey=""`、`model="test"`、`temperature=0`、`maxTokens=0`、`timeout=10`
- **调用时机**: 用户添加模型时输入端点后

#### 4.2.2 获取模型列表

```go
client = llm.NewClient(endpoint, apiKey, "test", 0, 0, 15)
models, err = client.ListModels(ctx)
```

- **参数来源**: 用户输入的 `endpoint` 和 `apiKey`，硬编码的 `model="test"`、`temperature=0`、`maxTokens=0`、`timeout=15`
- **调用时机**: 用户输入 API Key 后

#### 4.2.3 能力检测

```go
client := llm.NewClient(endpoint, testKey, modelName, 0, 0, 30)
vision := client.TestVisionSupport(ctx)
toolCall := client.TestToolCallSupport(ctx)
thinking := client.TestThinkingSupport(ctx)
```

- **参数来源**: 用户输入的 `endpoint`、`apiKey`、`modelName`，硬编码的 `temperature=0`、`maxTokens=0`、`timeout=30`
- **调用时机**: 用户选择模型后自动检测

### 4.3 设置命令 — cmd/settings.go

#### 4.3.1 重建 LLM 客户端

```go
client := llm.NewClient(
    h.cfg.LLM.Endpoint,
    h.cfg.LLM.APIKey,
    h.cfg.LLM.Model,
    h.cfg.LLM.Temperature,
    h.cfg.LLM.MaxTokens,
)
client.SetTopP(h.cfg.LLM.TopP)
client.SetTopK(h.cfg.LLM.TopK)
client.SetRepetitionPenalty(h.cfg.LLM.RepetitionPenalty)
client.SetThinkingEnabled(h.cfg.LLM.ThinkingEnabled)
client.SetReasoningEffort(h.cfg.LLM.ReasoningEffort)
client.SetTokenUsage(h.cfg.LLM.TokenUsage)
```

- **参数来源**: `h.cfg.LLM` (全局配置)
- **调用时机**: 用户执行 `.set` 修改 LLM 相关参数后

### 4.4 模型切换 — cmd/model.go switchModel

```go
client := llm.NewClient(
    h.cfg.LLM.Endpoint,
    h.cfg.LLM.APIKey,
    h.cfg.LLM.Model,
    h.cfg.LLM.Temperature,
    h.cfg.LLM.MaxTokens,
)
client.SetTopP(h.cfg.LLM.TopP)
// ... 其他参数设置
h.agent.SetLLMClient(client)
```

- **参数来源**: `h.cfg.LLM` (全局配置，已从目标模型同步)
- **调用时机**: 用户执行 `.model switch <id>` 切换模型后

### 4.5 LLM 工具调用重建客户端 — agent/settings_tools.go

```go
client := llm.NewClient(
    cfg.LLM.Endpoint,
    cfg.LLM.APIKey,
    cfg.LLM.Model,
    cfg.LLM.Temperature,
    cfg.LLM.MaxTokens,
)
// ... 设置其他参数
ag.SetLLMClient(client)
```

- **参数来源**: `cfg.LLM` (全局配置)
- **调用时机**: LLM 通过工具调用修改 LLM 相关参数后

### 4.6 REPL 向导后重建 — repl/repl.go

```go
client := llm.NewClient(
    cfg.LLM.Endpoint,
    cfg.LLM.APIKey,
    cfg.LLM.Model,
    cfg.LLM.Temperature,
    cfg.LLM.MaxTokens,
)
// ... 设置其他参数
ag.SetLLMClient(client)
```

- **参数来源**: `cfg.LLM` (全局配置)
- **调用时机**: 首次设置向导完成后

### 4.7 主程序初始化 — main.go

```go
// 使用最高优先级模型的参数
activeModel := modelMgr.GetActiveModel(false)
if activeModel != nil && activeModel.APIKey != "" {
    temperature := cfg.LLM.Temperature
    if activeModel.Temperature != nil {
        temperature = *activeModel.Temperature
    }
    // ... 类似处理其他参数
    llmClient = llm.NewClient(
        activeModel.Endpoint,
        activeModel.APIKey,
        activeModel.Model,
        temperature,
        maxTokens,
        cfg.LLM.LLMTimeout,
    )
    llmClient.SetThinkingEnabled(thinkingEnabled)
    // ... 设置其他参数
}
```

- **参数来源**: 模型级参数优先，回退到全局 `cfg.LLM`
- **调用时机**: 程序启动时

---

## 5. 模型选择架构

### 5.1 三类模型的定义与选择逻辑

系统利用 `.model list` 中配置的多个模型，根据能力标签（ToolCall、Vision、Thinking）和优先级，动态选择最适合当前任务的模型。

| 模型类型 | 选择逻辑 | 用途 |
|----------|----------|------|
| **default-tool-model** | 具备 `ToolCall` 能力的**最高优先级**已启用模型 | Agent 主循环默认调用的模型 |
| **default-vision-model** | 具备 `Vision` 能力的**最高优先级**已启用模型 | 当上下文中添加了图片文件（`.image add`）时调用 |
| **default-problem-model** | 具备 `ToolCall` 能力的**第二优先级**已启用模型 | 当主循环模型调用出错时，用于诊断和恢复的辅助模型（预留，暂未实现） |

**选择规则：**
- 从 `ModelManager.GetAllModels()` 获取所有已配置模型，按优先级降序排列
- 遍历模型列表，找到第一个满足能力条件的已启用模型
- 如果没有任何模型满足条件，对应类型的模型 ID 显示为 `"-"`（表示 none）
- `default-problem-model` 当前逻辑与 `default-tool-model` 相同（取同一模型），后续可独立配置

### 5.2 动态模型切换机制

Agent 主循环中的 LLM 调用，**每次调用前**根据当前调用条件动态判断使用哪个模型：

```
Agent.RunStream() / Agent.Run()
    │
    ├── selectModelForCall()
    │   ├── 判断当前上下文是否包含图片（imagePaths 非空）
    │   │   ├── 是 → 选择 default-vision-model（需具备 Vision 能力）
    │   │   └── 否 → 选择 default-tool-model（需具备 ToolCall 能力）
    │   │
    │   └── 从 ModelManager 获取对应模型配置
    │       ├── 获取模型的 Endpoint、APIKey、Model 名
    │       └── 临时创建或复用 LLM Client
    │
    └── switchToModel() → 重建 llm.Client
        ├── 模型级参数优先
        └── 未设置时回退到全局 cfg.LLM
```

### 5.3 模型管理器 (ModelManager)

文件: `config/model_template.go`

```go
type ModelManager struct {
    templates []*ModelTemplate
    models    []*ModelConfig
}
```

核心方法:

| 方法 | 说明 |
|---|---|
| `GetActiveModel(visionRequired bool)` | 获取最高优先级的已启用模型，可选要求支持 Vision |
| `GetModel(id string)` | 按 ID 获取模型 |
| `GetAllModels()` | 获取所有模型 |
| `AddModel(m *ModelConfig)` | 添加模型 |
| `RemoveModel(id string)` | 移除模型 |

### 5.4 模型参数模板

文件: `config/model_template.go`

内置模板定义了各供应商的默认参数:

| 模板 ID | 供应商 | 默认端点 | 默认参数 |
|---|---|---|---|
| `deepseek` | DeepSeek | https://api.deepseek.com | thinking={"type":"enabled"} |
| `qwen` | 阿里千问 | https://dashscope.aliyuncs.com/compatible-mode/v1 | extra_body={} |
| `openai` | OpenAI | https://api.openai.com/v1 | 无 |
| `glm` | 智谱 | https://open.bigmodel.cn/api/paas/v4 | 无 |
| `xai` | xAI | https://api.x.ai/v1 | 无 |
| `xiaomi` | 小米 | https://api.minimax.chat/v1 | 无 |
| `ollama` | Ollama | http://localhost:11434/v1 | 无 |
| `openai-compatible` | 通用 | (用户自定义) | 无 |

---

## 6. 参数汇总

### 6.1 发送给 LLM 的请求参数

| 参数 | 来源 | 优先级 |
|---|---|---|
| `model` | ModelConfig.Model | 模型级 |
| `temperature` | ModelConfig.Temperature → LLMConfig.Temperature → 0.0 | 模型级 > 全局 > 默认 |
| `max_tokens` | ModelConfig.MaxTokens → LLMConfig.MaxTokens → 8192 | 模型级 > 全局 > 默认 |
| `top_p` | ModelConfig.TopP → LLMConfig.TopP → -1 (不发送) | 模型级 > 全局 > 默认 |
| `top_k` | ModelConfig.TopK → LLMConfig.TopK → -1 (不发送) | 模型级 > 全局 > 默认 |
| `repetition_penalty` | ModelConfig.RepetitionPenalty → LLMConfig.RepetitionPenalty → -1 (不发送) | 模型级 > 全局 > 默认 |
| `thinking` | ModelConfig.ThinkingEnabled → LLMConfig.ThinkingEnabled → false | 模型级 > 全局 > 默认 |
| `reasoning_effort` | ModelConfig.ReasoningEffort → LLMConfig.ReasoningEffort → "low" | 模型级 > 全局 > 默认 |
| `stream_options.include_usage` | LLMConfig.TokenUsage → "off" | 全局 > 默认 |
| `tools` | Agent 注册的工具列表 (受 ToolCallEnabled 控制) | 运行时 |
| `bodyAdditions` | ModelConfig.CustomParams + LLMConfig.BodyAdditions | 模型级 + 全局合并 |

### 6.2 参数配置方式

| 方式 | 示例 | 优先级 |
|---|---|---|
| 命令行参数 | `--temperature 0.7 --max-tokens 4096` | 最高 |
| REPL 命令 | `.set temperature 0.7` | 中 |
| 配置文件 | `config.json` 中的 `llm` 字段 | 低 |
| 默认值 | `config.DefaultConfig()` | 最低 |

### 6.3 模型级参数配置方式

| 方式 | 示例 | 说明 |
|---|---|---|
| `.model add` 向导 | 交互式设置 | 自动检测能力 |
| `.model from-tpl` | `.model from-tpl deepseek deepseek-chat --api-key xxx` | 从模板创建 |
| `.model set-param` | `.model set-param my-model thinking {"type":"enabled"}` | 设置自定义参数 |
| `.model set-priority` | `.model set-priority my-model 50` | 设置优先级 |
| `.model switch` | `.model switch my-model` | 切换并启用 |

---

## 7. 调用流程图

### 7.1 正常对话流程

```
用户输入
    │
    ▼
REPL (repl/repl.go)
    │
    ▼
Agent.RunStream() (agent/loop.go)
    │
    ├── selectModelForCall()  ← 动态模型选择
    │   └── 如需切换 → switchToModel() → 重建 llm.Client
    │
    ├── 构建 messages (含上下文、系统提示词)
    │
    ├── 构建 tools (工具定义列表)
    │
    ├── streamLLMResponse()
    │   └── llm.ChatStream()  ← 发送 HTTP 请求到 LLM API
    │       └── openAIClient.ChatStream()
    │           ├── 构建 chatRequestJSON
    │           ├── 合并 bodyAdditions
    │           ├── POST /chat/completions
    │           └── 解析 SSE 流 → 返回 StreamEvent 通道
    │
    ├── 处理流式事件 (content/thinking/tool_call/done)
    │
    ├── 如果 LLM 返回 tool_calls
    │   ├── 执行工具 → 获取结果
    │   ├── 将结果加入 messages
    │   └── 再次调用 LLM (迭代)
    │
    └── 迭代完成 → 返回最终结果
```

### 7.2 模型切换流程

```
用户执行 .model switch <id>
    │
    ▼
cmd/model.go switchModel()
    │
    ├── 将目标模型移到列表首位
    ├── 重新编码优先级
    ├── 同步参数到 cfg.LLM
    │
    ├── 重建 llm.Client
    │   ├── llm.NewClient(activeModel.Endpoint, activeModel.APIKey, ...)
    │   ├── client.SetTopP(...)
    │   ├── client.SetTopK(...)
    │   └── ...
    │
    └── agent.SetLLMClient(client)
```

### 7.3 参数修改生效流程

```
用户执行 .set temperature 0.7
    │
    ▼
cmd/settings.go Handle()
    │
    ├── 修改 cfg.LLM.Temperature = 0.7
    ├── 保存 config.json
    │
    ├── 重建 llm.Client
    │   └── agent.SetLLMClient(newClient)
    │
    └── 立即生效 (无需重启)
```

---

## 8. 关键代码位置索引

| 功能 | 文件 | 说明 |
|---|---|---|
| Client 接口定义 | `llm/client.go` | Client 接口 |
| openAIClient 结构体 | `llm/client.go` | openAIClient 结构体 |
| NewClient 构造函数 | `llm/client.go` | NewClient 函数 |
| Chat() 非流式调用 | `llm/client.go` | Chat 方法 |
| ChatStream() 流式调用 | `llm/client.go` | ChatStream 方法 |
| 请求体构建 (chatRequestJSON) | `llm/client.go` | chatRequestJSON 结构体 |
| 参数合并 (mergeBodyAdditions) | `llm/client.go` | mergeBodyAdditions 函数 |
| LLMConfig 结构体 | `config/config.go` | LLMConfig 字段 |
| ModelConfig 结构体 | `config/config.go` | ModelConfig 字段 |
| ModelManager | `config/model_template.go` | GetActiveModel 等方法 |
| Agent.Run() | `agent/loop.go` | Run 方法 |
| Agent.RunStream() | `agent/loop.go` | RunStream 方法 |
| streamLLMResponse() | `agent/loop.go` | streamLLMResponse 方法 |
| selectModelForCall() | `agent/agent.go` | selectModelForCall 方法 |
| switchToModel() | `agent/agent.go` | switchToModel 方法 |
| 主程序初始化 | `main.go` | LLM Client 初始化部分 |
| 设置命令重建客户端 | `cmd/settings.go` | Handle 方法 |
| 模型切换重建客户端 | `cmd/model.go` | switchModel 方法 |
| LLM 工具重建客户端 | `agent/settings_tools.go` | HandleSetLLMConfig 等方法 |
| REPL 向导后重建 | `repl/repl.go` | Run 方法 |
| 模型管理向导 | `cmd/model.go` | wizardEnterModelParams 等方法 |
