# FEATURE-346: browser_screenshot 视觉识别与 FEATURE-343 一致化

## 背景

1. FEATURE-343 的 minimal 识别轮机制（上下文折叠为 `[system(仅 Identity), user(intent+图片)]`、识别轮 tools 清空、识别结果回填工具返回）目前**仅 `visual_analysis` 走**；`browser_screenshot` 截图后只设置 `imagePaths`、未设置 `visionPendingIntent`，识别在完整上下文内联进行。FEATURE-343 用例 UC-0019 当时**有意排除** browser_screenshot，导致两者行为不一致。
2. 调研发现潜在 bug：OpenAI 工具调用模式下，工具执行后 `a.messages` 最后一条为 `tool` 消息；`buildContextMessages` 的图片注入逻辑依赖最后一条为 `user` 消息（`if lastMsg.Role == "user"`），否则图片未注入就被 `imagePaths=nil` 清空。识别轮折叠时 `mediaParts` 为空 → **视觉模型收不到图片**。visual_analysis 与 browser_screenshot 均受影响。本次一并修复，使两者行为一致且识别轮真正携带图片。

## 目标

minimal 模式（`vision-context-mode=minimal`）下，`browser_screenshot` 与 `visual_analysis` 走完全相同的识别轮机制：

- `browserScreenshotTool` 视觉支持时设置 `visionPendingIntent`（取工具 `intent` 参数，缺失时用本地化默认指令兜底）+ `imagePaths`
- 下一轮迭代折叠为 `[system(仅 Identity), user(intent + 图片)]`，不写主会话上下文
- 识别轮 tools 清空（OpenAI 传空 tools；XML system prompt 精简为仅 Identity）
- 识别结果回填为 `browser_screenshot` 工具返回（OpenAI `tool` 消息 / XML `user` 消息），不新增独立 assistant 消息
- 识别轮失败时回填错误标记，上下文保持合法
- **OpenAI 模式识别轮图片不再丢失**（图片注入不依赖最后一条消息角色）
- full 模式（`vision-context-mode=full`）行为保持不变

> 注：本特性将**取代** FEATURE-343 用例 UC-0019 的预期（browser_screenshot 不再"不触发识别轮"）。

## 架构

```
RunStream iteration 0（主模型轮次）：
  user("看看当前页面") → 进 a.messages
  assistant(tool_calls: browser_screenshot, intent) → 进 a.messages
  执行 browser_screenshot：
    * minimal + vision：设置 visionPendingIntent=intent + imagePaths=[截图]；记录 ToolCallID/工具名
    * 返回 "Screenshot saved..." 消息（UI 与历史均显示，与 visual_analysis 占位结果一致）
RunStream iteration 1（识别轮，额外轮次）：
  检测：minimal && len(imagePaths)>0 && visionPendingIntent!=""
  buildContextMessages() 折叠：
    msgs = [system(仅 Identity), user(intent + 截图 base64)]
    ↑ 图片直接从 imagePaths 编码注入，不依赖最后一条历史是否 user（修复 OpenAI 模式图片丢失）
  tools = []（OpenAI 清空；XML system prompt 仅 Identity）
  视觉模型输出 finalContent：
    * 不追加 assistant 消息
    * OpenAI：追加 {Role:"tool", ToolCallID:<browser_screenshot ID>, Content:finalContent}
    * XML：追加 user 消息（工具名 = browser_screenshot）
    * 失败/空：回填错误标记
    * 清空 visionPendingIntent
  主模型下一轮看到：user → assistant(call browser_screenshot) → tool/user(识别结果)
```

## 用例

<!--FEATURE23_PART2-->
### M1 minimal 模式 browser_screenshot 识别流程（OpenAI 工具调用模式）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | browser_screenshot 调用记录进历史 | minimal + toolcall-mode=openai，用户输入"看看当前页面"，LLM 调用 browser_screenshot(intent="查看页面当前状态") | a.messages 包含 user 消息 + assistant(带 ToolCalls browser_screenshot 原始参数)；工具执行后 `imagePaths=[截图路径]` 且 `visionPendingIntent=="查看页面当前状态"` |
| UC-0002 | 识别轮折叠为 [system(仅Identity), user(intent+图片)] | 上一轮调 browser_screenshot 后，下一轮迭代开始 | buildContextMessages() 返回 2 条：msgs[0] 仅含 Identity 节（无 ToolUsage/Capabilities/Rules/Environment 等节，且含 agent 身份描述，与主提示词同源解析），msgs[1] 为 user(intent 文本 + 截图 base64 ContentPart)；**即使最后一条历史是 tool 消息（OpenAI）图片也不丢失** |
| UC-0003 | 识别轮 tools 清空 | 识别轮调用 ChatStream | tools 参数为 `[]`（空列表），LLM 请求体中无 tools 字段 |
| UC-0004 | 识别结果回填 tool 消息（OpenAI） | 视觉模型返回"页面标题为 co-shell，导航栏包含 GitHub 链接" | a.messages 新增 `{Role:"tool", ToolCallID:<browser_screenshot 的 ID>, Content:"页面标题为…"}`；**不新增** assistant 消息 |
| UC-0005 | 识别结果后主模型可继续对话 | 回填完成后用户再输入"点击导航栏的 GitHub 链接" | 主模型上下文包含：user → assistant(call browser_screenshot) → tool(识别结果)；识别轮的 user(intent/图片 base64) **不可见** |
| UC-0006 | visionPendingIntent 清理 | 识别轮结束后 | visionPendingIntent == ""；后续正常轮次不再触发 minimal 折叠 |
| UC-0007 | imagePaths 一次性清空 | 识别轮发送后 | imagePaths == nil（现有 one-shot 行为保留） |
| UC-0008 | 识别轮 UI 显示 | 视觉模型流式输出识别内容 | 用户界面实时透传显示识别内容（流式渲染）；不显示工具调用确认 |

### M2 minimal 模式识别轮失败处理

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0009 | 识别轮 LLM 调用失败 | 视觉模型 API 返回 500 | a.messages 追加 tool 消息（OpenAI）含错误标记（如 `视觉分析失败: <错误>`）；不新增 assistant 消息；上下文协议合法 |
| UC-0010 | 识别轮用户 ESC 中断 | 用户按 ESC 取消识别轮输出 | 视觉模型输出丢弃，a.messages 追加错误标记 tool 消息（OpenAI）或 user 错误消息（XML），上下文保持合法 |
| UC-0011 | 识别轮模型返回空内容 | 视觉模型输出为空串 | 回填为错误标记（`视觉分析失败: 空结果`），上下文保持合法 |

### M3 XML 工具调用模式

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0012 | XML 模式识别结果作为 user 消息 | minimal + toolcall-mode=xml，browser_screenshot 后识别轮返回"文字内容：Hello" | a.messages 新增 user 消息（识别结果并入 tool result 模板，**工具名为 browser_screenshot**：`<TOOL_CALL>browser_screenshot</...>`）；**不新增** assistant 消息 |
| UC-0013 | XML 模式识别轮 system prompt 仅 Identity | XML 模式识别轮 | msgs[0] system 仅含 Identity 节，无 XML 工具说明（无 # Tools 列表、无 XML 格式示例） |
| UC-0014 | XML 模式识别失败回填 | XML 模式视觉模型报错 | a.messages 追加 user 错误消息（识别结果模板含错误标记），上下文保持合法 |

### M4 full 模式回归（保持现状）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0015 | full 模式 browser_screenshot 图片进完整上下文 | vision-context-mode=full，browser_screenshot 后下一轮迭代 | 不折叠；图片注入最后一条 user 消息；完整 system prompt + 全部 tools 保留（现状不变） |
| UC-0016 | full 模式无工具栏清空 | full 模式识别轮 | tools 参数保持完整工具列表；XML system prompt 不精简 |
| UC-0017 | full 模式 visual_analysis 行为不变 | full 模式调 visual_analysis | "文件加载成功"占位结果与 intent 照旧写入历史（taskInstructionCache flush 行为不变） |

<!--FEATURE23_PART3-->
### M5 边界与防误触发

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0018 | 不支持视觉时不触发识别轮 | vision 关闭，browser_screenshot 截图 | 不设置 visionPendingIntent；不注入图片；返回"当前模型不支持视觉识别"提示（KeySettingCmd_690）；不触发折叠 |
| UC-0019 | browser_screenshot 无 intent 参数（向后兼容） | LLM 调用 browser_screenshot 时未提供 intent | 使用本地化默认识别指令（KeyBrowserScreenshotVisionIntentDefault）作为 visionPendingIntent，识别轮正常触发 |
| UC-0020 | browser_screenshot 后紧跟 visual_analysis | 同一轮内连续调用两者 | 最后一次调用覆盖 visionPendingIntent / imagePaths / ToolCallID；识别轮只触发一次；回填定位最后一次调用 |
| UC-0021 | 识别轮后再次 browser_screenshot | 识别完成后再次截图 | 新一轮识别独立执行；前一轮记录保持完整；visionPendingIntent 重新设置 |

### M6 上下文合法性验证

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0022 | 识别轮后下一轮主模型请求合法（OpenAI） | 回填完成后再让主模型迭代 | 请求消息序列无 orphan tool 消息（每个 tool 紧跟对应 assistant tool_calls）；无 400 协议错误 |
| UC-0023 | 识别轮后下一轮主模型请求合法（XML） | XML 模式回填完成后再迭代 | 消息序列符合 XML 模式结构（user 工具结果在 assistant content 工具调用后） |
| UC-0024 | 识别轮前后 :context 输出检查 | 识别轮完成后执行 `:context` | 历史中可见 browser_screenshot 工具调用与识别结果回填；**不可见**识别轮的 user(intent+图片 base64) 与单独 assistant 识别消息 |
| UC-0025 | 会话持久化恢复后上下文一致 | 识别完成后退出重进 co-shell（session 恢复） | 恢复的上下文包含 browser_screenshot 调用 + 识别结果，无识别轮污染记录 |

### M7 单元测试覆盖

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0026 | 识别轮图片注入不依赖最后消息角色 | 最后一条历史为 tool 消息（OpenAI 模式），imagePaths + visionPendingIntent 已设 | buildContextMessages() 折叠为 [system(Identity), user(intent+图片)]；图片以 base64 ContentPart 存在于 user 消息（文件不可读时跳过并告警，折叠仍发生） |
| UC-0027 | executeToolCall 记录 browser_screenshot 的 ToolCallID 与工具名 | browser_screenshot 工具调用被执行 | `lastVisionToolCallID == tc.ID` 且 `lastVisionToolCallName == "browser_screenshot"` |
| UC-0028 | XML 回填使用被记录的工具名 | browser_screenshot 触发识别轮，XML 模式 | 回填的 user 消息工具名为 `browser_screenshot`（非硬编码 visual_analysis） |
| UC-0029 | 新增 i18n 默认指令 key 存在 | 检查 KeyBrowserScreenshotVisionIntentDefault | zh/en 均有翻译，非空且非 key 原文 |
| UC-0030 | 识别轮 system prompt 含 agent 身份描述 | 未显式设置 AgentDescription（默认 act 模式） | buildVisionIdentityPrompt() 输出包含模式专属默认描述（如"你是一个严谨、务实、目标驱动的编程助手"）；主提示词与识别轮描述同源（resolveAgentDescription 同一解析链） |
| UC-0031 | browser_screenshot 工具说明引导 intent 编写 | 检查工具 schema 与 i18n 系统提示（zh/en） | 工具 Description 与 intent 参数说明明确：intent 将作为截图视觉识别指令，要求写出具体分析目标（提取/核对哪些信息），禁止"查看页面"式模糊表述；Usage 示例为指示性指令 |

## 验证方式

1. **单元测试**：`agent/` 下新增 `fix23_test.go`（识别轮图片注入不依赖最后消息角色 / 记录字段 / XML 工具名 / i18n key）
2. **运行命令**：`go build ./... && go test ./agent/ && go vet ./agent/`
3. **人工验证**：按 UC-0001~0031 场景逐一走查（可用 `.simulate` 模拟 LLM 返回进行隔离验证）

## 验收标准

1. 全部单测通过，构建/vet 全绿
2. M1：minimal + OpenAI browser_screenshot 识别流程完整（调用进历史 / 折叠仅 Identity / tools 清空 / 结果回填 / 无 assistant 残留 / intent 清理 / **图片不丢失**）
3. M2：识别失败三场景（API 报错 / ESC / 空结果）回填错误标记，上下文合法
4. M3：XML 模式正确回填，工具名为 browser_screenshot，system prompt 仅 Identity
5. M4：full 模式 3 项回归全部保持现状
6. M5：无 intent 兜底 / 连续调用 / 再次截图边界正常；不支持视觉不触发
7. M6：识别轮前后上下文协议合法，:context 与 session 恢复无污染
8. M7：识别轮 system prompt 含 agent 身份描述（UC-0030）；browser_screenshot 工具说明引导指导性 intent（UC-0031）


