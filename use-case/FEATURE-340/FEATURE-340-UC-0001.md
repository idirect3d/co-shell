# FEATURE-340: :context full 在 openai 模式显示工具调用方法声明

## 需求

`:context full` 在 openai 工具调用模式下，除现有消息记录和 `<environment_details>` 外，还需显示 LLM 请求中 `tools` 参数的工具函数声明（每个工具的 name、description、parameters JSON Schema）。

## 方案

1. `agent` 包新增公开方法 `ToolDeclarations() []llm.Tool`：
   - 复用 `buildToolsInternal()`（内置工具 + MCP 工具完整列表）
   - 带 `mcpMgr == nil` guard（避免单元测试场景 panic，参照 loop.go 已有先例）
   - 仅 openai 模式返回非空（xml 模式不通过 tools 参数下发，返回空）
   - 返回拷贝，不暴露内部状态

2. `cmd/context.go` 的 `showContext(full)`：
   - 仅当 `full == true` **且** `ToolCallMode() == "openai"` 时，在所有消息块之后追加 tool declarations 区块
   - 每个工具渲染：工具名 + Description + parameters JSON（复用 `formatToolArguments` 的 JSON 格式化逻辑）
   - 缩进风格与现有 tool_calls 区块一致（6 空格）

3. i18n：新增工具声明标题 key（zh/en 双存在）

## 测试用例（循环模式）

### UC-0001：默认（非 full）不显示工具声明

| # | 场景 | 步骤 | 预期结果 |
|---|---|---|---|
| 1 | openai 模式 + 默认 :context | 构造含消息历史的 handler，调用 `showContext(false)` | 输出不包含工具声明区块标题 |
| 2 | openai 模式 + :context show | 调用 `Handle(["show"])` | 输出不包含工具声明区块标题 |
| 3 | openai 模式 + :context 无参数 | 调用 `Handle([])` | 输出不包含工具声明区块标题 |

### UC-0002：full + openai 模式显示工具声明

| # | 场景 | 步骤 | 预期结果 |
|---|---|---|---|
| 1 | openai 模式 + :context full | 调用 `Handle(["full"])` | 输出包含工具声明区块标题 |
| 2 | 工具名显示 | 检查输出中的工具列表 | 每个工具名以 `- <name>` 格式出现（如 `- read_file`） |
| 3 | Description 显示 | 检查 read_file 工具 | 输出包含 read_file 的 Description 文本 |
| 4 | parameters JSON 显示 | 检查 read_file 工具 | 输出包含格式化后的 parameters JSON（如 `"type": "object"`、`"path"`） |
| 5 | 分隔符 | 检查区块位置 | 工具声明区块在所有消息块之后，区块前有 `contextSeparator` 分隔线 |

### UC-0003：full + xml 模式不显示工具声明

| # | 场景 | 步骤 | 预期结果 |
|---|---|---|---|
| 1 | xml 模式 + :context full | 设置 ToolCallMode("xml") 后调用 `Handle(["full"])` | 输出**不包含**工具声明区块标题 |
| 2 | xml 模式 + 消息仍正常显示 | 检查输出 | 消息记录和 `<environment_details>` 仍正常显示 |

### UC-0004：mcpMgr nil 不 panic

| # | 场景 | 步骤 | 预期结果 |
|---|---|---|---|
| 1 | agent 未初始化 mcpMgr（测试环境） | 用 `agent.New(nil, nil, ds, "")` 构造 handler 调用 `showContext(true)` | 不 panic，输出正常 |
| 2 | 工具声明仍显示 | 检查输出 | openai full 模式下仍显示内置工具声明（MCP 工具不在列） |

### UC-0005：i18n 双语言

| # | 场景 | 步骤 | 预期结果 |
|---|---|---|---|
| 1 | zh 语言 | `i18n.SetLang("zh")` 显示 | 标题为中文（如 `[工具声明]`） |
| 2 | en 语言 | `i18n.SetLang("en")` 显示 | 标题为英文（如 `[Tool Declarations]`） |

## 测试命令

```bash
go build ./...
go vet ./cmd/ ./agent/
go test ./cmd/ ./agent/ ./i18n/
bin/output_audit.sh   # Hardcoded Chinese = 0
```
