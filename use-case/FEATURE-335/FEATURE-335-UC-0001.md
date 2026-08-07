# FEATURE-335: :context 显示增强（tool_calls 块 + 保留控制字符 + retried_count + full 模式）

## 背景

`:context` 命令目前只能输出 assistant、user、tool 三种消息，没有返回 tool_calls 消息；且第 96 行 `strings.ReplaceAll(content, "\n", " ")` 将多行内容拍平成单行，空格、tab、回车、换行等控制字符全部丢失，用户难以阅读和理解。

## 修改内容

1. **新增 tool_calls 块**：assistant 消息含 `ToolCalls` 时，在同一消息下标下方输出 `[tool_calls]` 子块，每个 ToolCall 显示工具名 + 格式化缩进的参数 JSON（解析失败则原样输出）。tool_calls **不占独立序号**，与 `<message_no>`（消息数组绝对下标）严格对齐。
2. **保留控制字符**：移除 `ReplaceAll("\n", " ")` 拍平逻辑，各消息内容按原始格式多行输出、4 空格缩进、块间空行分隔。
3. **retried_count 显示**：消息头最右侧显示 `♾️N`（从消息最后一个 ContentPart 的 `<environment_details>` 中解析 `<retried_count>`，无则省略）。需要 agent 导出公开 helper（如 `RetriedCountOf`）。
4. **full 模式**：`:context full` 显示所有消息的完整 `<environment_details>`；`:context`（默认）隐藏 env 块。两个命令独立（子命令）实现，**不新增全局配置参数**。

## 用例（循环式运行时确认）

### UC-0001: 序号与 `<message_no>` 严格对齐
- 构建含 user(env message_no=1)、assistant(Content+ToolCalls)、tool(env message_no=3)、user(env message_no=4) 的消息序列
- `:context` 输出应显示 4 行消息头，序号依次 0/1/2/3，其中 assistant 消息的 tool_calls 作为子块**不占独立序号**
- 每条消息头序号 == 该消息 env 内 `<message_no>` 值（tool 消息序号 3 == env 中的 message_no 3）

### UC-0002: tool_calls 块显示工具名 + 缩进参数
- assistant 消息含 `ToolCalls: [{Name: "read_file", Arguments: '{"path":"cmd/context.go","intent":"查看"}'}]`
- `:context` 输出 `[assistant]` 块下出现 `tool_calls:` 子块，工具名 `read_file`，参数 JSON 格式化缩进、包含原字段
- 参数 JSON 非法时原样输出不崩溃

### UC-0003: 保留控制字符（多行/空格/tab 原样显示）
- 构造带多行文本、内部缩进（空格/tab）、空行的 user 消息
- `:context` 输出中这些控制字符**原样保留**，不再被替换为单个空格

### UC-0004: retried_count 显示在消息头最右侧
- user 消息最后一个 ContentPart 含 `<environment_details>` 且 `<retried_count>3</retried_count>`
- `:context` 消息头最右侧出现 `♾️3`
- 无 retried_count 的普通消息头不显示 `♾️`
- 无 ContentParts 的 plain-Content 消息不显示 `♾️`

### UC-0005: full 模式显示 environment_details
- `:context full` 输出中每个含 env 的消息下方显示完整 `<environment_details>` 块（含 `<cwd>`、`<message_no>`、`<retried_count>` 等）
- 默认 `:context` 隐藏 env 块

### UC-0006: 指针标记与总消息数不受影响
- 消息指针位置显示 `*` 标记，未变
- `总消息数: N` 正确反映实际消息数量（不因 tool_calls 子块增加而虚增）
- system 消息（无 env）正常显示其内容

### UC-0007: PRINCIPLES.md 实时重建回归
- 修改 PRINCIPLES.md 后 `:context` 反映新内容（复用 cmd/context_rebuild_test.go 行为）

### UC-0008: 构建与测试全绿
- `go build ./...`、`go test ./cmd/ ./agent/`、`go vet ./cmd/ ./agent/` 全绿
- 新增 `cmd/context_show_test.go` 单测覆盖：tool_calls 块 / 多行内容 / retried_count / full 开关 / 序号对齐

## 验收

1. 上述 UC-0001~0008 全部通过
2. 所有可见序号与 `<message_no>` 严格一致，tool_calls 不产生伪下标
3. 无新增全局配置参数；`:context` 与 `:context full` 各自独立可用