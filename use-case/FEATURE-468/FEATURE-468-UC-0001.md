# FEATURE-468 Responses API 支持 — 测试用例

> 分支：FEATURE-468
> 版本：v0.31.0
> 目标：co-shell 支持 OpenAI Responses API（/v1/responses），通过 `reasoning: {effort: "none"}` 控制思考开关，支持按模型选择 API 类型（chat 默认 / responses）

## 测试环境

- 三个目标端点（均已实测支持 /responses）：
  - LM Studio qwen3.6（http://127.0.0.1:11234/v1）
  - DeepSeek 官方（https://api.deepseek.com）
  - 本地代理 deepseek-0731（http://localhost:11535/deepseek/v1）

## UC-0001: config 增加 api_type 字段

- **前置**：无
- **步骤**：
  1. 查看 `config/model_template.go` 的 ModelConfig 结构
  2. 确认新增 `APIType string \`json:"api_type,omitempty"\`` 字段
  3. 确认默认值为空（表示 chat）
- **预期**：ModelConfig 有 APIType 字段，默认空值

## UC-0002: NewClient 根据 api_type 分发

- **前置**：config 有 api_type 字段
- **步骤**：
  1. 调用 `NewClient(endpoint, apiKey, model, temp, maxTokens, apiType)`
  2. apiType="chat" 或空 → 返回 openAIClient
  3. apiType="responses" → 返回 responsesClient
- **预期**：根据 api_type 返回正确的 client 类型

## UC-0003: responsesClient 请求转换（Message → input）

- **前置**：responsesClient 已创建
- **步骤**：
  1. 构造 `[]llm.Message`（system/user/assistant/tool 四种角色）
  2. 调用 Chat/ChatStream
  3. 检查请求体 input 数组格式
- **预期**：
  - system/user/assistant → `{type: "message", role, content: [{type: "input_text", text}]}`
  - assistant 带 ToolCalls → `{type: "function_call", call_id, name, arguments}`
  - tool → `{type: "function_call_output", call_id, output}`

## UC-0004: responsesClient 工具定义转换

- **前置**：responsesClient 已创建
- **步骤**：
  1. 构造 `[]llm.Tool`
  2. 调用 Chat/ChatStream
  3. 检查请求体 tools 数组格式
- **预期**：`tools: [{type: "function", name, description, parameters}]`（扁平结构）

## UC-0005: responsesClient 响应解析（output[] → LLMResponse）

- **前置**：mock Responses API 响应
- **步骤**：
  1. 构造 Responses API 响应（含 message/function_call/reasoning）
  2. 调用 Chat
  3. 检查 LLMResponse
- **预期**：
  - Content 从 output[] 中 type=message 的 content[].text 提取
  - ReasoningContent 从 type=reasoning 提取
  - ToolCalls 从 type=function_call 提取（call_id→ID, name, arguments）
  - Usage 从 usage.input_tokens/output_tokens 映射

## UC-0006: responsesClient 流式解析（事件 → StreamEvent）

- **前置**：mock Responses API 流式响应
- **步骤**：
  1. 构造流式事件序列（response.created → output_text.delta → completed）
  2. 调用 ChatStream
  3. 检查 StreamEvent 序列
- **预期**：
  - output_text.delta → StreamEventContent
  - reasoning_text.delta → StreamEventReasoning
  - function_call_arguments.delta → StreamEventToolCallDelta
  - completed → StreamEventDone

## UC-0007: responsesClient 思考控制（reasoning.effort）

- **前置**：responsesClient 已创建
- **步骤**：
  1. SetThinkingEnabled(true) + SetReasoningEffort("none")
  2. 调用 Chat/ChatStream
  3. 检查请求体 reasoning 字段
- **预期**：请求体含 `reasoning: {effort: "none"}`（关闭思考）

## UC-0008: 模型向导支持 api_type 选择

- **前置**：Web UI 模型设置向导
- **步骤**：
  1. 打开模型设置向导
  2. 选择模板后，出现 API 类型选择（chat/responses）
  3. 选择 responses 并保存
- **预期**：模型配置保存 api_type=responses

## UC-0009: 端到端验证（LM Studio qwen3.6 关闭思考）

- **前置**：LM Studio 运行，qwen3.6 模型配置 api_type=responses
- **步骤**：
  1. 配置 qwen3.6 模型 api_type=responses，thinking_enabled=true, reasoning_effort=none
  2. 通过 co-shell 发送请求
  3. 检查响应
- **预期**：响应无 reasoning_content（思考关闭）

## UC-0010: 端到端验证（DeepSeek 官方关闭思考）

- **前置**：DeepSeek API key 有效
- **步骤**：
  1. 配置 deepseek-v4-flash 模型 api_type=responses，reasoning_effort=none
  2. 通过 co-shell 发送请求
  3. 检查响应
- **预期**：响应无 reasoning_content（思考关闭）

## UC-0011: 向后兼容（默认 chat）

- **前置**：现有模型配置无 api_type 字段
- **步骤**：
  1. 不设置 api_type，使用现有配置
  2. 通过 co-shell 发送请求
- **预期**：使用 Chat Completions API，行为与之前一致
