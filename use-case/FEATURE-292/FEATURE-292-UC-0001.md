# FEATURE-292-UC-0001: 移除 `<task>` 标签包裹策略

## 测试目的

验证移除所有 `<task>` 标签包裹后，用户指令、补充输入、循环反馈等内容以纯文本形式传递给 LLM，不产生额外的注意力扭曲。

## 涉及修改点

| 位置 | 文件 | 当前行为 | 修改后 |
|------|------|----------|--------|
| `buildUserMessage()` | `agent/agent.go` | XML 模式下用户指令被 `<task>指令</task>` 包裹 | 直接发送原始 `instruction` 文本 |
| flush taskInstructionCache (tool result) | `agent/run_stream.go` | `\<task\>内容\</task\>` 写入 ContentPart | 直接写入 `内容` (纯文本) |
| flush taskInstructionCache (after tool calls) | `agent/run_stream.go` | 同上 | 同上 |
| flush taskInstructionCache (no tool + exit) | `agent/run_stream.go` | 同上 | 同上 |
| applyLoopIntervention | `agent/loop.go` | `\<task\>反馈\</task\>` 包裹 feedback | 直接写 `反馈` |
| handleLoopDetection | `agent/loop.go` | 同步路径 feedback `<task>` 包裹 | 同上 |
| reorganize pending msg | `agent/run_stream.go` | `\<task\>必须马上整理上下文\</task\>` | 直接写纯文本 |
| getFirstUserCommand | `agent/loop.go` | 提取 `<task>` 标签内容 | 直接取第一条 user 消息内容 |
| getLastUserCommand | `agent/loop.go` | 提取最后一个 `<task>` 标签内容 | 取最后一条 user 消息内容 |

## 测试场景

### 场景 1：用户指令不再被 `<task>` 包裹

**输入：** 用户输入 "帮我查看当前目录的文件"
**期望：** 生成的消息 Content 中不包含 `<task>` 标签

### 场景 2：ask_followup 补充输入不再被 `<task>` 包裹

**输入：** LLM ask_followup_question → 用户回复 "选择第二个选项"
**期望：** flush 到工具结果消息的补充内容不包含 `<task>` 标签

### 场景 3：循环检测反馈不再被 `<task>` 包裹

**输入：** 循环检测触发 → applyLoopIntervention
**期望：** 追加的 user 消息内容不包含 `<task>` 标签

### 场景 4：判模模型仍能获取原始任务指令

**输入：** getFirstUserCommand/getLastUserCommand
**期望：** 直接返回 user 消息的纯文本内容，无需解析 `<task>` 标签

### 场景 5：上下文超限重组提示不再被 `<task>` 包裹

**输入：** 上下文超限 → reorganizePending 消息
**期望：** 生成的提醒消息不包含 `<task>` 标签