# LLM 调用架构说明

## 概述

co-shell 使用 OpenAI 兼容的 API 协议调用大语言模型（LLM）。所有 LLM 调用都通过 `llm.Client` 接口进行，该接口定义在 `llm/client.go` 中。

## LLM Client 接口

```go
type Client interface {
    Chat(ctx, messages, tools)           // 非流式聊天补全
    ChatStream(ctx, messages, tools)     // 流式聊天补全
    ListModels(ctx)                      // 获取可用模型列表
    TestVisionSupport(ctx)               // 测试视觉支持
    TestTextSupport(ctx)                 // 测试文本支持
    TestToolCallSupport(ctx)             // 测试工具调用支持
    TestThinkingSupport(ctx)             // 测试思考模式支持
    SetThinkingEnabled(bool)             // 启用/禁用思考模式
    SetReasoningEffort(string)           // 设置推理努力程度
    SetTopP(float64)                     // 设置 top-p 采样
    SetTopK(int)                         // 设置 top-k 采样
    SetRepetitionPenalty(float64)        // 设置重复惩罚
    SetTokenUsage(string)                // 设置 token 用量显示模式
    SetBodyAdditions(map[string]string)  // 设置自定义请求体属性
    RemoveBodyAddition(key)              // 移除自定义请求体属性
    GetBodyAdditions() map[string]string // 获取自定义请求体属性
    Close() error                        // 清理资源
}
```

## LLM 调用点总览

项目中共有 **10 处** `llm.NewClient` 调用（创建 LLM 客户端），**3 处** `.Chat()` 调用（非流式），**1 处** `.ChatStream()` 调用（流式）。

---

## 一、模型选择架构

### 1. 三类模型的定义与选择逻辑

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

### 2. 动态模型切换机制

Agent 主循环中的 LLM 调用，**每次调用前**根据当前调用条件动态判断使用哪个模型：

```
Agent.RunStream() / Agent.Run()
    │
    ├── 判断当前上下文是否包含图片（imagePaths 非空）
    │   ├── 是 → 使用 default-vision-model（需具备 Vision 能力）
    │   └── 否 → 使用 default-tool-model（需具备 ToolCall 能力）
    │
    ├── 从 ModelManager 获取对应模型配置
    │   ├── 获取模型的 Endpoint、APIKey、Model 名
    │   └── 临时创建或复用 LLM Client
    │
    └── 发起 LLM 调用（Chat / ChatStream）
```

**当前实现状态：**
- `main.go:718-731`：启动时根据 `--image` 标志自动选择视觉模型（已实现）
- `cmd/settings.go:1131-1170`：`.set` 界面动态显示三类模型的 ID（已实现）
- Agent 主循环中每次调用前动态判断模型：**待实现**

### 3. `.set` 界面中的模型显示

在 `.set` 命令的显示界面中，三类模型以只读方式显示其当前选中的模型 ID：

```
default-tool-model:    deepseek-official-deepseek-v4-flash    (default tool model)
default-vision-model:  qwen-official-qwen-vl-max              (default vision model)
default-problem-model: deepseek-official-deepseek-v4-flash    (default problem-solving model)
```

用户可以通过 `.model` 命令管理模型配置（添加、删除、启用/禁用），系统自动根据能力标签和优先级重新计算三类默认模型。

---

## 二、LLM Client 创建点（10 处）

### 1. `main.go:760` — 主程序初始化

**用途：** 程序启动时创建主 LLM 客户端。

**参数来源：**

| 参数 | 来源 | 默认值 |
|------|------|--------|
| `Endpoint` | `cfg.LLM.Endpoint` (config.json) | `https://api.deepseek.com` |
| `APIKey` | `cfg.LLM.APIKey` (config.json) | `""` |
| `Model` | `cfg.LLM.Model` (config.json) | `deepseek-v4-flash` |
| `Temperature` | `cfg.LLM.Temperature` (config.json) | `0.7` |
| `MaxTokens` | `cfg.LLM.MaxTokens` (config.json) | `-1` (不发送) |
| `LLMTimeout` | `cfg.LLM.LLMTimeout` (config.json) | `0` (无超时) |

**后续设置（从 config.json 读取）：**

| 设置方法 | 来源 | 默认值 |
|----------|------|--------|
| `SetThinkingEnabled` | `cfg.LLM.ThinkingEnabled` | `false` |
| `SetReasoningEffort` | `cfg.LLM.ReasoningEffort` | `"low"` |
| `SetTopP` | `cfg.LLM.TopP` | `0.9` |
| `SetTopK` | `cfg.LLM.TopK` | `20` |
| `SetRepetitionPenalty` | `cfg.LLM.RepetitionPenalty` | `1.0` |
| `SetTokenUsage` | `cfg.LLM.TokenUsage` | `"on"` |
| `SetBodyAdditions` | `cfg.LLM.BodyAdditions` | `nil` |

---

### 2. `cmd/settings.go:56` — `.settings` 命令重建客户端

**用途：** 当用户通过 `.set` 命令修改 LLM 相关参数时，重建客户端使设置立即生效。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | `h.cfg.LLM.Endpoint` |
| `APIKey` | `h.cfg.LLM.APIKey` |
| `Model` | `h.cfg.LLM.Model` |
| `Temperature` | `h.cfg.LLM.Temperature` |
| `MaxTokens` | `h.cfg.LLM.MaxTokens` |
| `LLMTimeout` | **未传递**（使用默认 60s） |

**后续设置：** `SetTopP`, `SetTopK`, `SetRepetitionPenalty`, `SetThinkingEnabled`, `SetReasoningEffort` — 全部来自 `h.cfg.LLM`。

> **注意：** 此处的 `rebuildLLMClient()` 未传递 `LLMTimeout` 参数，也未设置 `SetTokenUsage` 和 `SetBodyAdditions`。

---

### 3. `repl/repl.go:514` — 向导完成后重建客户端

**用途：** 设置向导（wizard）完成后重建 LLM 客户端。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | `r.cfg.LLM.Endpoint` |
| `APIKey` | `r.cfg.LLM.APIKey` |
| `Model` | `r.cfg.LLM.Model` |
| `Temperature` | `r.cfg.LLM.Temperature` |
| `MaxTokens` | `r.cfg.LLM.MaxTokens` |
| `LLMTimeout` | `r.cfg.LLM.LLMTimeout` |

**后续设置：** `SetThinkingEnabled`, `SetReasoningEffort` — 来自 `r.cfg.LLM`。

> **注意：** 此处的 `rebuildLLMClient()` 未设置 `SetTopP`, `SetTopK`, `SetRepetitionPenalty`, `SetTokenUsage`, `SetBodyAdditions`。

---

### 4. `agent/settings_tools.go:833` — LLM 工具调用重建客户端

**用途：** 当 LLM 通过 `update_setting` 工具修改设置时，重建客户端使设置立即生效。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | `a.cfg.LLM.Endpoint` |
| `APIKey` | `a.cfg.LLM.APIKey` |
| `Model` | `a.cfg.LLM.Model` |
| `Temperature` | `a.cfg.LLM.Temperature` |
| `MaxTokens` | `a.cfg.LLM.MaxTokens` |
| `LLMTimeout` | `a.cfg.LLM.LLMTimeout` |

**后续设置：** `SetThinkingEnabled`, `SetReasoningEffort`, `SetTopP`, `SetTopK`, `SetRepetitionPenalty` — 来自 `a.cfg.LLM`。

> **注意：** 此处的 `rebuildLLMClient()` 未设置 `SetTokenUsage` 和 `SetBodyAdditions`。

---

### 5. `cmd/model.go:332` — 模型向导测试端点连通性

**用途：** 在 `.model add` 向导中测试端点连通性。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | 用户输入的端点 |
| `APIKey` | `""` (空字符串) |
| `Model` | `"test"` (固定值) |
| `Temperature` | `0` |
| `MaxTokens` | `0` |
| `LLMTimeout` | `10` (固定 10s) |

**用途：** 仅调用 `ListModels()` 测试连通性，不进行对话。

---

### 6. `cmd/model.go:376` — 模型向导获取模型列表

**用途：** 在 `.model add` 向导中获取可用模型列表。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | 用户输入的端点 |
| `APIKey` | 用户输入的 API Key |
| `Model` | `"test"` (固定值) |
| `Temperature` | `0` |
| `MaxTokens` | `0` |
| `LLMTimeout` | `15` (固定 15s) |

**用途：** 仅调用 `ListModels()` 获取模型列表，不进行对话。

---

### 7. `cmd/model.go:631` — 模型能力检测

**用途：** 在 `.model add` 向导中检测模型能力（视觉、工具调用、思考模式）。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | 用户输入的端点 |
| `APIKey` | 用户输入的 API Key |
| `Model` | 用户选择的模型名 |
| `Temperature` | `0` |
| `MaxTokens` | `0` |
| `LLMTimeout` | `30` (固定 30s) |

**用途：** 调用 `TestVisionSupport()`, `TestToolCallSupport()`, `TestThinkingSupport()` 进行能力检测。

---

### 8. `wizard/wizard.go:332` — 设置向导获取模型列表

**用途：** 在首次设置向导中获取可用模型列表。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | 用户选择的预设端点 |
| `APIKey` | 用户输入的 API Key |
| `Model` | `""` (空字符串) |
| `Temperature` | `0` |
| `MaxTokens` | `0` |
| `LLMTimeout` | 未传递 (默认 60s) |

**用途：** 仅调用 `ListModels()`。

---

### 9. `wizard/wizard.go:390` — 设置向导检测视觉支持

**用途：** 在首次设置向导中检测模型是否支持视觉识别。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | `cfg.LLM.Endpoint` |
| `APIKey` | `cfg.LLM.APIKey` |
| `Model` | 用户选择的模型 ID |
| `Temperature` | `cfg.LLM.Temperature` |
| `MaxTokens` | `cfg.LLM.MaxTokens` |
| `LLMTimeout` | `10` (固定 10s) |

**用途：** 调用 `TestVisionSupport()`。

---

### 10. `wizard/wizard.go:430` — 设置向导检测工具调用支持

**用途：** 在首次设置向导中检测模型是否支持工具调用。

**参数来源：**

| 参数 | 来源 |
|------|------|
| `Endpoint` | `cfg.LLM.Endpoint` |
| `APIKey` | `cfg.LLM.APIKey` |
| `Model` | 用户选择的模型 ID |
| `Temperature` | `cfg.LLM.Temperature` |
| `MaxTokens` | `cfg.LLM.MaxTokens` |
| `LLMTimeout` | `15` (固定 15s) |

**用途：** 调用 `TestToolCallSupport()`。

---

## 三、LLM 实际调用点（Chat / ChatStream）

### 1. `agent/loop.go:162` — Agent.Run 非流式调用

```go
resp, err := a.llmClient.Chat(ctx, a.messages, tools)
```

**参数来源：**
- `ctx`：从外部传入的 context
- `a.messages`：Agent 维护的完整对话历史（包含 system prompt + 用户消息 + assistant 回复 + tool 结果）
- `tools`：由 `a.buildTools()` 构建的可用工具列表

**调用链路：** `main.go` → `repl/repl.go` (非流式模式) → `Agent.Run()` → `llmClient.Chat()`

**发送到 API 的请求体参数：**
- `model`：创建 client 时设置的 `c.model`
- `messages`：`a.messages`（完整历史）
- `temperature`：`c.temperature`（>=0 时发送）
- `max_tokens`：`c.maxTokens`（>=0 时发送）
- `top_p`：`c.topP`（>=0 时发送）
- `top_k`：`c.topK`（>=0 时发送）
- `repetition_penalty`：`c.repetitionPenalty`（>=0 时发送）
- `tools`：`buildTools(tools)`（非空时发送）
- `thinking`：`c.thinkingEnabled && isThinkingModel(c.model)` 时发送
- `reasoning_effort`：thinking 模式下发送 `c.reasoningEffort`
- 自定义属性：`c.bodyAdditions` 合并到请求体

---

### 2. `agent/loop.go:740` — Agent.RunStream 流式调用

```go
eventCh, err := a.llmClient.ChatStream(ctx, contextMsgs, tools)
```

**参数来源：**
- `ctx`：从外部传入的 context
- `contextMsgs`：由 `a.buildContextMessages()` 构建的**截断后**的对话历史
  - 根据 `cfg.LLM.ContextLimit` 截断历史消息
  - 根据 `a.messagePointer` 跳过指定位置之前的消息
  - 每条消息内容前添加索引前缀（如 `"123: 2026-05-01 12:09:24 - ..."`）
- `tools`：由 `a.buildTools()` 构建的可用工具列表

**调用链路：** `main.go` → `repl/repl.go` (流式模式) → `Agent.RunStream()` → `streamLLMResponse()` → `llmClient.ChatStream()`

**发送到 API 的请求体参数：** 与 Chat 相同，额外包含：
- `stream: true`
- `stream_options.include_usage`：根据 `c.tokenUsage` 决定（`"none"` 时不发送）

---

### 3. `agent/loop.go:873` — nonStreamingFallback 非流式回退

```go
resp, err := a.llmClient.Chat(ctx, contextMsgs, tools)
```

**用途：** 当流式调用失败时的回退方案。

**参数来源：**
- `ctx`：从外部传入的 context
- `contextMsgs`：由 `a.buildContextMessages()` 构建的截断后对话历史
- `tools`：由 `a.buildTools()` 构建的可用工具列表

---

## 四、LLM 能力检测调用点

### 1. `llm/client.go:1179` — TestVisionSupport

通过 `testChat()` 发送包含 1x1 像素 base64 图片的多模态请求，检测模型是否支持视觉输入。

**临时设置：** `temperature = 0`（测试后恢复）

### 2. `llm/client.go:1212` — TestTextSupport

通过 `testChat()` 发送简单文本请求 "Hi"，检测模型是否支持基本文本对话。

**临时设置：** `temperature = 0`（测试后恢复）

### 3. `llm/client.go:1230` — TestToolCallSupport

通过 `testChat()` 发送包含 `test_tool` 工具定义的请求，检测模型是否支持工具调用。

**临时设置：** `temperature = 0`（测试后恢复）

### 4. `llm/client.go:1273` — TestThinkingSupport

通过 `testChat()` 发送启用 thinking 模式的请求，检测模型是否支持思考模式。

**临时设置：** `thinkingEnabled = true`, `reasoningEffort = "low"`（测试后恢复）

---

## 五、参数传递完整链路图

```
config.json (持久化)
    │
    ├── config.LoadFromFile() → config.Config
    │       │
    │       ├── main.go → llm.NewClient(cfg.LLM.*) → llmClient
    │       │       │
    │       │       ├── agent.New(llmClient, ...) → Agent
    │       │       │       │
    │       │       │       ├── Agent.Run() → llmClient.Chat(ctx, a.messages, tools)
    │       │       │       │       └── 使用完整对话历史 a.messages
    │       │       │       │
    │       │       │       └── Agent.RunStream() → streamLLMResponse()
    │       │       │               ├── buildContextMessages() → 截断历史
    │       │       │               └── llmClient.ChatStream(ctx, contextMsgs, tools)
    │       │       │
    │       │       └── 后续设置:
    │       │           ├── SetThinkingEnabled(cfg.LLM.ThinkingEnabled)
    │       │           ├── SetReasoningEffort(cfg.LLM.ReasoningEffort)
    │       │           ├── SetTopP(cfg.LLM.TopP)
    │       │           ├── SetTopK(cfg.LLM.TopK)
    │       │           ├── SetRepetitionPenalty(cfg.LLM.RepetitionPenalty)
    │       │           ├── SetTokenUsage(cfg.LLM.TokenUsage)
    │       │           └── SetBodyAdditions(cfg.LLM.BodyAdditions)
    │       │
    │       ├── cmd/settings.go → rebuildLLMClient() (运行时重建)
    │       │       └── 未传递: LLMTimeout, TokenUsage, BodyAdditions
    │       │
    │       ├── repl/repl.go → rebuildLLMClient() (向导后重建)
    │       │       └── 未传递: TopP, TopK, RepetitionPenalty, TokenUsage, BodyAdditions
    │       │
    │       └── agent/settings_tools.go → rebuildLLMClient() (LLM工具调用重建)
    │               └── 未传递: TokenUsage, BodyAdditions
    │
    ├── cmd/model.go (模型管理向导)
    │       ├── 测试连通性: llm.NewClient(endpoint, "", "test", 0, 0, 10)
    │       ├── 获取模型列表: llm.NewClient(endpoint, apiKey, "test", 0, 0, 15)
    │       └── 能力检测: llm.NewClient(endpoint, apiKey, modelName, 0, 0, 30)
    │
    └── wizard/wizard.go (首次设置向导)
            ├── 获取模型列表: llm.NewClient(endpoint, apiKey, "", 0, 0)
            ├── 检测视觉: llm.NewClient(cfg.LLM.*, 10)
            └── 检测工具调用: llm.NewClient(cfg.LLM.*, 15)
```

## 六、配置默认值

所有 LLM 参数的默认值定义在 `config/config.go` 的 `DefaultConfig()` 函数中：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `Provider` | `"deepseek"` | 供应商 |
| `Endpoint` | `"https://api.deepseek.com"` | API 端点 |
| `Model` | `"deepseek-v4-flash"` | 模型名 |
| `Temperature` | `0.7` | 温度 (-1=不发送) |
| `MaxTokens` | `-1` | 最大 token 数 (-1=不发送) |
| `MaxIterations` | `1000` | 最大迭代次数 |
| `TopP` | `0.9` | Top-p 采样 (-1=不发送) |
| `TopK` | `20` | Top-k 采样 (-1=不发送) |
| `RepetitionPenalty` | `1.0` | 重复惩罚 (-1=不发送) |
| `ThinkingEnabled` | `false` | 思考模式 |
| `ReasoningEffort` | `"low"` | 推理努力程度 |
| `TokenUsage` | `"on"` | Token 用量显示 |
| `LLMTimeout` | `0` | 非流式请求超时 (0=无超时) |
| `ToolTimeout` | `0` | 工具调用超时 (0=无超时) |
| `CommandTimeout` | `0` | 命令执行超时 (0=无超时) |

## 七、请求体 JSON 结构

最终发送到 API 的请求体结构（`chatRequestJSON`）：

```json
{
    "model": "deepseek-v4-flash",
    "messages": [
        {"role": "system", "content": "..."},
        {"role": "user", "content": "..."},
        {"role": "assistant", "content": "...", "tool_calls": [...]},
        {"role": "tool", "content": "...", "tool_call_id": "..."}
    ],
    "temperature": 0.7,
    "max_tokens": -1,
    "top_p": 0.9,
    "top_k": 20,
    "repetition_penalty": 1.0,
    "tools": [...],
    "stream": true,
    "stream_options": {"include_usage": true},
    "thinking": {"type": "enabled"},
    "reasoning_effort": "low",
    // 自定义属性 (BodyAdditions) 在此合并
}
```

> **注意：** 参数值为 `-1` 时表示"不发送该参数到 API"，在序列化时会被省略。

## 八、关键发现

1. **三个 `rebuildLLMClient()` 实现不完全一致：**
   - `cmd/settings.go` 的 `rebuildLLMClient()` 未传递 `LLMTimeout`，未设置 `SetTokenUsage` 和 `SetBodyAdditions`
   - `repl/repl.go` 的 `rebuildLLMClient()` 未设置 `SetTopP`、`SetTopK`、`SetRepetitionPenalty`、`SetTokenUsage`、`SetBodyAdditions`
   - `agent/settings_tools.go` 的 `rebuildLLMClient()` 未设置 `SetTokenUsage` 和 `SetBodyAdditions`

2. **流式 vs 非流式的消息差异：**
   - `Agent.Run()`（非流式）使用 `a.messages`（完整历史）
   - `Agent.RunStream()`（流式）使用 `buildContextMessages()` 构建的截断历史

3. **能力检测使用独立客户端：** 所有能力检测（视觉、工具调用、思考模式）都创建临时客户端，使用 `temperature=0` 确保确定性结果，测试后恢复原始设置。

4. **模型选择架构（新设计）：**
   - 系统通过 `.model list` 管理多个模型，每个模型有独立的能力标签（Vision、ToolCall、Thinking）和优先级
   - 三类默认模型（default-tool-model、default-vision-model、default-problem-model）根据能力标签和优先级自动计算
   - `.set` 界面动态显示三类模型的当前选中 ID
   - Agent 主循环中每次调用前动态判断使用哪个模型（待实现完整动态切换）
