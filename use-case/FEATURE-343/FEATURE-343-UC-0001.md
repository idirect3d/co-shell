# FEATURE-343: 视觉识别上下文隔离（minimal 识别轮独立调用 + 结果回填工具返回）

## 背景

当前 minimal 模式（`vision-context-mode=minimal`）仅折叠历史消息为 `[system, user(intent+图片)]`，但存在三个问题：
1. 识别轮输出以独立 assistant 消息留在主上下文历史中，语义割裂（"assistant 说了一个识别结论"，而非"visual_analysis 工具返回了识别内容"）
2. `visual_analysis` 工具执行后生成的"文件加载成功"占位结果混入历史，且 intent 文本也通过 `taskInstructionCache` 无条件写入历史
3. OpenAI 模式识别轮仍携带全部 tools 参数，视觉模型可能尝试继续调用工具而不是专注识别

## 目标

minimal 模式下，识别轮作为**额外独立轮次**执行，不污染主会话上下文：
- visual_analysis 工具调用记录（含输入参数）正常进历史
- 不生成"文件加载成功"占位结果进历史（仅 UI 显示）
- 识别轮输入只含 [system(仅 Identity 节), user(intent + 图片)]，不写主会话上下文
- 识别轮 tools 清空（OpenAI 传空 tools；XML system prompt 精简为仅 Identity）
- 识别结果回填为 visual_analysis 工具调用的返回，不新增 assistant 消息
- 识别轮失败时返回错误标记到工具结果，上下文保持合法

full 模式（`vision-context-mode=full`）保持现状不变。

## 架构

```
RunStream iteration 0（主模型轮次）：
  user("帮我看看这张图") → 进 a.messages
  assistant(tool_calls: visual_analysis, paths+intent) → 进 a.messages ✅
  执行 visual_analysis：
    * minimal：设置 visionPendingIntent + imagePaths；不写 taskInstructionCache；不追加占位结果
    * UI 仍显示"文件加载成功"给用户（不进历史）
    * 记录 ToolCallID（OpenAI 模式）供回填定位

RunStream iteration 1（识别轮，额外轮次）：
  检测：minimal && len(imagePaths)>0 && visionPendingIntent!=""
  buildContextMessages() 折叠：
    msgs = [system(仅 Identity 节), user(visionPendingIntent + 图片 base64)]
    ↑ 只发给视觉模型，不写 a.messages
  tools = []（OpenAI 模式清空 tools 参数）
  视觉模型输出 finalContent：
    * 不追加 assistant 消息到 a.messages
    * OpenAI：追加 {Role:"tool", ToolCallID:记录ID, Content:finalContent}
    * XML：追加 user 消息（识别结果并入 tool result 模板）
    * 失败时：追加错误标记 tool/user 消息
    * 清空 visionPendingIntent
  循环结束/继续，主模型下一轮（如有）看到完整序列：
    user(输入) → assistant(call visual_analysis) → tool/user(识别结果) ✅
```

## 用例

### M1 minimal 模式识别流程（OpenAI 工具调用模式）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | visual_analysis 调用记录进历史 | `vision-context-mode=minimal`，toolcall-mode=openai，用户输入"识别这张发票金额"，LLM 调用 visual_analysis(paths=[invoice.png], intent="识别发票金额") | a.messages 包含 user 消息 + assistant(带 ToolCalls visual_analysis 原始参数) 两条记录；执行后**无**"文件加载成功" tool 消息 |
| UC-0002 | 识别轮折叠为 [system(仅Identity), user(intent+图片)] | 上一轮 LLM 调 visual_analysis 后，下一轮迭代开始 | buildContextMessages() 返回 2 条：msgs[0] 仅含 Identity 节（无 ToolUsage/Capabilities/Rules/Environment 等节），msgs[1] 为 user(intent 文本 + 图片 base64 ContentPart) |
| UC-0003 | 识别轮 tools 清空 | 识别轮调用 `ChatStream` | tools 参数为 `[]`（空列表），LLM 请求体中无 tools 字段 |
| UC-0004 | 识别结果回填 tool 消息（OpenAI） | 视觉模型返回"发票总金额：¥12,345.67" | a.messages 新增 `{Role:"tool", ToolCallID:<visual_analysis 的 ID>, Content:"发票总金额：¥12,345.67"}`；**不新增** assistant 消息 |
| UC-0005 | 识别结果后主模型可继续对话 | 回填完成后用户再输入"那税额呢" | 主模型上下文包含：user(帮我看看这张图) → assistant(call visual_analysis) → tool(发票总金额：¥12,345.67) → user(那税额呢)；视觉模型识别的 user/intent/图片 base64 **不可见** |
| UC-0006 | visionPendingIntent 清理 | 识别轮结束后 | visionPendingIntent == ""；后续正常轮次不再触发 minimal 折叠 |
| UC-0007 | imagePaths 一次性清空 | 识别轮发送后 | imagePaths == nil（现有 one-shot 行为保留） |
| UC-0008 | 识别轮 UI 显示 | 视觉模型流式输出识别内容 | 用户界面实时透传显示识别内容（流式渲染）；不显示工具调用确认 |

### M2 minimal 模式识别轮失败处理

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0009 | 识别轮 LLM 调用失败 | 视觉模型 API 返回 500 | a.messages 追加 tool 消息（OpenAI）含错误标记（如 `视觉分析失败: <错误>`）；不新增 assistant 消息；上下文协议合法（tool 跟在 assistant tool_calls 之后） |
| UC-0010 | 识别轮用户 ESC 中断 | 用户按 ESC 取消识别轮输出 | 视觉模型输出丢弃，a.messages 追加错误标记 tool 消息（OpenAI）或 user 错误消息（XML），上下文保持合法 |
| UC-0011 | 识别轮模型返回空内容 | 视觉模型输出为空串 | 回填为错误标记（`视觉分析失败: 空结果`），上下文保持合法 |

### M3 minimal 模式识别结果回填（XML 工具调用模式）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0012 | XML 模式识别结果作为 user 消息 | `vision-context-mode=minimal`，toolcall-mode=xml，visual_analysis 后识别轮返回"文字内容：Hello" | a.messages 新增 user 消息（识别结果并入 tool result 模板：`<TOOL_CALL>visual_analysis</...><TOOL_RESULT>文字内容：Hello</...>`）；**不新增** assistant 消息 |
| UC-0013 | XML 模式识别轮 system prompt 仅 Identity | XML 模式识别轮 | msgs[0] system 仅含 Identity 节，无 XML 工具说明（无 # Tools 列表、无 XML 格式示例） |
| UC-0014 | XML 模式识别失败回填 | XML 模式视觉模型报错 | a.messages 追加 user 错误消息（识别结果模板含错误标记），上下文保持合法 |

### M4 full 模式回归（保持现状）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0015 | full 模式识别输入输出正常进历史 | `vision-context-mode=full`，用户输入"看这张图" | 识别轮的 user(intent+图片) 与 assistant(识别结果) 均正常进 a.messages；工具说明照常携带（OpenAI tools 参数非空 / XML system prompt 完整） |
| UC-0016 | full 模式 visual_analysis 占位结果保留 | full 模式调用 visual_analysis | "文件加载成功"占位结果照旧写入 a.messages（taskInstructionCache flush 行为不变）；intent 文本照旧写入历史 |
| UC-0017 | full 模式无工具栏清空 | full 模式识别轮 | tools 参数保持完整工具列表；XML system prompt 不精简 |
| UC-0018 | default 模式（未设置 vision-context-mode） | 不设置 vision-context-mode | 默认 minimal 行为生效（与 FEATURE-319 默认值一致，折叠 + 识别轮独立） |

### M5 边界与防误触发

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0019 | browser_screenshot 不误触发识别轮 | minimal 模式，browser_screenshot 截图后自动加载到 imagePaths，但**无** visionPendingIntent | 不触发 recognition 轮；下一轮迭代照常走主模型（不折叠、不替换 system、不清空 tools） |
| UC-0020 | 无图片时正常轮次 | minimal 模式，普通文本对话（无 visual_analysis） | 不触发识别轮；buildContextMessages 正常返回完整历史；tools 完整 |
| UC-0021 | visionPendingIntent 存在但无图片 | minimal 模式，visionPendingIntent 已设但 imagePaths 为空 | 不触发识别轮（len(imagePaths)==0 守卫），走正常轮次 |
| UC-0022 | 连续两次 visual_analysis | 一条消息内 LLM 连续调用两次 visual_analysis（不同 paths） | 第二次调用覆盖 visionPendingIntent 与 imagePaths（合并累积）；识别轮只触发一次；回填定位最近一次 visual_analysis 的 ToolCallID |
| UC-0023 | 识别轮后再次调用 visual_analysis | 识别完成后再对另一张图调用 visual_analysis | 新一轮识别独立执行；前一轮记录保持完整；visionPendingIntent 重新设置 |

### M6 上下文合法性验证

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0024 | 识别轮后下一轮主模型请求合法（OpenAI） | 回填完成后再让主模型迭代 | 请求消息序列无 orphan tool 消息（每个 tool 紧跟对应 assistant tool_calls）；无 400 协议错误 |
| UC-0025 | 识别轮后下一轮主模型请求合法（XML） | XML 模式回填完成后再迭代 | 消息序列符合 XML 模式结构（user 工具结果在 assistant content 工具调用后） |
| UC-0026 | 识别轮前后 :context 输出检查 | 识别轮完成后执行 `:context` | 历史中可见 visual_analysis 工具调用与识别结果回填；**不可见**识别轮的 user(intent+图片 base64) 与单独 assistant 识别消息 |
| UC-0027 | 会话持久化恢复后上下文一致 | 识别完成后退出重进 co-shell（session 恢复） | 恢复的上下文包含 visual_analysis 调用 + 识别结果，无识别轮污染记录 |

## 验证方式

1. **单元测试**：`agent/` 下新增测试（识别轮检测条件 / 识别轮输出处理 / tools 清空 / 折叠 system 仅 Identity / 回填定位 / 失败兜底 / visionPendingIntent 清理）
2. **运行命令**：`go build ./... && go test ./agent/ && go vet ./agent/`
3. **人工验证**：按 UC-0001~0027 场景逐一走查（可用 `.simulate` 模拟 LLM 返回进行隔离验证）

## 验收标准

1. 全部单测通过，构建/vet 全绿
2. M1：minimal + OpenAI 识别流程完整（调用进历史 / 不生成占位 / 折叠仅 Identity / tools 清空 / 结果回填 / 无 assistant 残留 / intent 清理）
3. M2：识别失败三场景（API 报错 / ESC / 空结果）回填错误标记，上下文合法
4. M3：XML 模式同样正确处理，system prompt 仅 Identity
5. M4：full 模式 4 项回归全部保持现状
6. M5：browser_screenshot 不误触发；连续调用正常
7. M6：识别轮前后上下文协议合法，:context 与 session 恢复无污染