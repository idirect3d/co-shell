# FEATURE-388 工具调用交互标准化 — 测试用例

> 版本：v0.1
> 日期：2026-08-21
> 分支：FEATURE-388
> 说明：本文件按阶段组织测试用例，覆盖 Interaction 模型、TerminalInteractionManager（TUI）、WebInteractionManager（Web）、ToolSummary 结构化。

---

## 阶段一：Interaction 模型 + TerminalInteractionManager（TUI）

### UC-0001 Interaction 结构体字段完整性
- **前置**：无
- **操作**：构造一个 `Interaction{Kind: confirm}` 对象
- **预期**：`Interaction` 包含 `Kind`、`Title`、`Body`、`Options`、`Keys`、`Default`、`AllowFree` 字段；`InteractionResult` 包含 `Action`、`Value`、`Raw` 字段
- **验证**：单元测试断言字段存在且类型正确

### UC-0002 InteractionKind 枚举
- **前置**：无
- **操作**：检查 `InteractionKind` 常量
- **预期**：包含 `confirm`、`select`、`input`、`key` 四种类型
- **验证**：单元测试断言常量值

### UC-0003 InteractionManager 接口定义
- **前置**：无
- **操作**：检查 `InteractionManager` 接口
- **预期**：包含 `Ask(ctx, in Interaction) (InteractionResult, error)` 方法
- **验证**：单元测试断言接口可被 `TerminalInteractionManager` 实现

### UC-0004 TerminalInteractionManager 确认放行（Enter 批准）
- **前置**：构造 `TerminalInteractionManager`，注入 mock UserIO
- **操作**：调用 `Ask` 传入 `Interaction{Kind: confirm}`，mock 输入 Enter（空行）
- **预期**：返回 `InteractionResult{Action: "approve"}`
- **验证**：单元测试断言返回结果

### UC-0005 TerminalInteractionManager 确认放行（c 取消）
- **前置**：同上
- **操作**：mock 输入 `c`
- **预期**：返回 `InteractionResult{Action: "cancel"}`
- **验证**：单元测试断言返回结果

### UC-0006 TerminalInteractionManager 确认放行（a 全部批准）
- **前置**：同上
- **操作**：mock 输入 `a`
- **预期**：返回 `InteractionResult{Action: "approve_all"}`
- **验证**：单元测试断言返回结果

### UC-0007 TerminalInteractionManager 确认放行（g 禁用确认）
- **前置**：同上
- **操作**：mock 输入 `g`
- **预期**：返回 `InteractionResult{Action: "approve_g"}`
- **验证**：单元测试断言返回结果

### UC-0008 TerminalInteractionManager 确认放行（d 永久禁用）
- **前置**：同上
- **操作**：mock 输入 `d`
- **预期**：返回 `InteractionResult{Action: "approve_d"}`
- **验证**：单元测试断言返回结果

### UC-0009 TerminalInteractionManager 确认放行（N 批准N次）
- **前置**：同上
- **操作**：mock 输入 `3`
- **预期**：返回 `InteractionResult{Action: "approve_count", Value: "3"}`
- **验证**：单元测试断言返回结果

### UC-0010 TerminalInteractionManager 确认放行（其他输入 = 补充指令）
- **前置**：同上
- **操作**：mock 输入 `请先检查文件`
- **预期**：返回 `InteractionResult{Action: "modify", Value: "请先检查文件"}`
- **验证**：单元测试断言返回结果

### UC-0011 TerminalInteractionManager 提问/选择（选项编号）
- **前置**：构造 `Interaction{Kind: select, Options: ["立即执行", "稍后执行"]}`
- **操作**：mock 输入 `1`
- **预期**：返回 `InteractionResult{Action: "select", Value: "立即执行"}`
- **验证**：单元测试断言返回结果

### UC-0012 TerminalInteractionManager 提问/选择（取消选项）
- **前置**：同上，Options 长度 2，cancelIdx = 3
- **操作**：mock 输入 `3`
- **预期**：返回 `InteractionResult{Action: "cancel"}`
- **验证**：单元测试断言返回结果

### UC-0013 TerminalInteractionManager 提问/选择（选项编号 + 补充输入）
- **前置**：同上
- **操作**：mock 输入 `1 请补充细节`
- **预期**：返回 `InteractionResult{Action: "select", Value: "立即执行", Raw: "1 请补充细节"}`
- **验证**：单元测试断言返回结果

### UC-0014 TerminalInteractionManager 自由输入（无选项）
- **前置**：构造 `Interaction{Kind: input}`
- **操作**：mock 输入 `任意内容`
- **预期**：返回 `InteractionResult{Action: "input", Value: "任意内容"}`
- **验证**：单元测试断言返回结果

### UC-0015 TerminalInteractionManager 无效输入重试
- **前置**：构造 `Interaction{Kind: select, Options: ["A", "B"]}`
- **操作**：mock 依次输入 `9`（越界）、`1`
- **预期**：第一次提示重选，第二次返回 `InteractionResult{Action: "select", Value: "A"}`
- **验证**：单元测试断言返回结果

---

## 阶段二：迁移 promptToolConfirmation 和 askFollowupQuestionTool

### UC-0016 promptToolConfirmation 迁移后行为不变
- **前置**：`promptToolConfirmation` 改用 `InteractionManager`
- **操作**：mock 输入 Enter/c/a/g/d/N/其他
- **预期**：返回的 `CmdConfirmResult` 与迁移前完全一致
- **验证**：单元测试断言各输入对应的 `CmdConfirmResult`

### UC-0017 askFollowupQuestionTool 迁移后行为不变
- **前置**：`askFollowupQuestionTool` 改用 `InteractionManager`
- **操作**：mock 输入选项编号/自由文本
- **预期**：返回结果与迁移前一致（写入 taskInstructionCache）
- **验证**：单元测试断言返回结果和 taskInstructionCache 内容

### UC-0018 工具回调不再直接调用 io.Println/ReadLine
- **前置**：检查 `promptToolConfirmation` 和 `askFollowupQuestionTool` 源码
- **操作**：静态检查
- **预期**：两个函数不再直接调用 `io.Println/Printf/ReadLine`，改为通过 `InteractionManager`
- **验证**：代码审查

---

## 阶段三：WebInteractionManager + WebSocket 协议

### UC-0019 WebInteractionManager 推送结构化 interaction 消息
- **前置**：构造 `WebInteractionManager`，注入 mock WebIO
- **操作**：调用 `Ask` 传入 `Interaction{Kind: confirm}`
- **预期**：通过 WebSocket 推送 `{kind: "interaction", id, interaction: {...}}` 消息，包含完整 Interaction 数据
- **验证**：单元测试断言推送的消息结构

### UC-0020 WebInteractionManager 接收 interaction_answer 并返回结果
- **前置**：同上
- **操作**：mock 浏览器回传 `{type: "interaction_answer", id, result: {action: "approve"}}`
- **预期**：`Ask` 返回 `InteractionResult{Action: "approve"}`
- **验证**：单元测试断言返回结果

### UC-0021 WebInteractionManager 批准N次预设按钮
- **前置**：构造 `Interaction{Kind: confirm}`，Keys 中 approve_count 携带预设值 [3,10,50]
- **操作**：检查推送的 interaction 数据
- **预期**：`approve_count` 键位携带 `preset: [3, 10, 50]`
- **验证**：单元测试断言推送数据

### UC-0022 前端渲染确认放行按钮组
- **前置**：Node DOM shim 加载真实 app.js
- **操作**：驱动 `interaction` 消息（kind=confirm）
- **预期**：渲染批准/取消/全部批准/禁用确认/永久禁用按钮 + 批准N次预设按钮（3/10/50）+ 自由输入框
- **验证**：Node 断言按钮数量和文本

### UC-0023 前端渲染选项列表
- **前置**：Node DOM shim 加载真实 app.js
- **操作**：驱动 `interaction` 消息（kind=select，Options=["A","B"]）
- **预期**：渲染单选按钮 A/B + 取消
- **验证**：Node 断言选项数量和文本

### UC-0024 前端不再硬编码确认键位
- **前置**：检查 app.js 源码
- **操作**：静态检查
- **预期**：`showAsk` 不再硬编码 `[["Enter",""],["c","c"],...]`，改为从 `interaction.keys` 动态渲染
- **验证**：代码审查

### UC-0025 前端交互回传 interaction_answer
- **前置**：Node DOM shim 加载真实 app.js
- **操作**：点击"批准"按钮
- **预期**：发送 `{type: "interaction_answer", id, result: {action: "approve"}}`
- **验证**：Node 断言发送的消息

---

## 阶段四：ToolSummary 结构化

### UC-0026 ToolSummary 结构体定义
- **前置**：无
- **操作**：检查 `ToolSummary` 结构体
- **预期**：包含 `ToolName`、`Intent`、`Params` 字段；`SummaryParam` 包含 `Name`、`Value`、`Kind`
- **验证**：单元测试断言字段存在

### UC-0027 buildToolSummary 返回结构化数据（高频工具）
- **前置**：调用 `buildToolSummary("execute_command", args)`
- **操作**：检查返回值
- **预期**：返回 `ToolSummary{ToolName: "execute_command", Intent: "列出文件", Params: [{Name: "command", Value: "ls -la"}]}`
- **验证**：单元测试断言结构化字段

### UC-0028 buildToolSummary 返回结构化数据（低频工具 fallback）
- **前置**：调用 `buildToolSummary("list_settings", args)`
- **操作**：检查返回值
- **预期**：返回 `ToolSummary{ToolName: "list_settings", Intent: "查看配置"}`，Params 为空或含全部参数
- **验证**：单元测试断言结构化字段

### UC-0029 ToolSummary 同时保留 Text（i18n 整句模板）
- **前置**：调用 `buildToolSummary`
- **操作**：检查返回值
- **预期**：`ToolSummary` 同时包含 `Text`（现有 i18n 整句模板生成的文本）和结构化字段
- **验证**：单元测试断言 Text 非空

### UC-0030 EventToolCall 携带结构化摘要
- **前置**：触发一次工具调用
- **操作**：检查 `EventToolCall` 事件的 Meta
- **预期**：`Meta[MetaKeyToolSummary]` 携带 `ToolSummary` 的 JSON
- **验证**：单元测试断言 Meta 内容

### UC-0031 TUI 渲染 ToolSummary 用 Text
- **前置**：LineRenderer 处理 `EventToolCall`
- **操作**：检查渲染输出
- **预期**：TUI 用 `ToolSummary.Text` 渲染（与现状一致）
- **验证**：单元测试断言渲染文本

### UC-0032 Web 渲染 ToolSummary 用结构化字段
- **前置**：Node DOM shim 加载真实 app.js
- **操作**：驱动 `EventToolCall` 事件（携带结构化摘要）
- **预期**：渲染为结构化卡片（工具名 + 意图 + 参数列表）
- **验证**：Node 断言卡片内容

---

## 回归测试

### UC-0033 TUI 确认放行回归
- **前置**：完整 TUI 环境
- **操作**：触发一个需要确认的工具调用，依次输入 Enter/c/a/g/d/N/其他
- **预期**：行为与迁移前完全一致
- **验证**：手动测试

### UC-0034 Web 确认放行回归
- **前置**：完整 Web 环境
- **操作**：触发一个需要确认的工具调用，点击各按钮
- **预期**：按钮式确认正常工作，放行次数预设按钮生效
- **验证**：浏览器手动测试

### UC-0035 编译与测试全绿
- **前置**：无
- **操作**：`go build ./... && go vet ./... && go test ./...`
- **预期**：全部通过
- **验证**：命令执行
