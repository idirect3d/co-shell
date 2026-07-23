# FEATURE-287 方法调用解析错误统一处理参数 parse-error-action

## 背景

目前 co-shell 在处理方法调用解析错误时，XML 解析错误、流式 tool call 增量校验失败、工具执行失败等场景各自使用不同的处理逻辑，且缺乏统一的配置参数来控制行为。这导致用户无法根据自己的需求调整错误处理策略。

## 目标

新增 `parse-error-action` 配置参数，统一所有方法调用解析错误的处理策略，支持三种模式：

- **exit**：退出迭代循环，向用户报告错误
- **retry**（默认）：无反馈，直接重发上下文
- **prompt**：发送结构化错误反馈（含参考格式）给 LLM，让 LLM 自行修正

## 影响范围

### 四个需要统一的错误路径

| 路径 | 文件 | 行号范围（估算） | 当前行为 |
|------|------|-------------------|----------|
| **A**: XML 解析错误 | `agent/run_stream.go` | 562-672 | 使用 `LoopIntervention` 策略 |
| **B**: 流式错误 + 移除 assistant 消息后 | `agent/run_stream.go` | 526-559 | 硬编码错误反馈追加 |
| **C**: 工具执行失败 | `agent/run_stream.go` | 983-997 | 使用 `formatToolError()` |
| **D**: 流式 tool call 全部无效 | `agent/stream_response.go` | 417-433 | 返回 error → 走路径 B |

### 非流式模式

| 路径 | 文件 | 行号范围 | 当前行为 |
|------|------|----------|----------|
| **E**: 工具执行失败（非流式） | `agent/run.go` | 225-228 | `fmt.Sprintf("Error: %v", err)` |

### 配置文件与 UI

- `config/config.go`：`LLMConfig` 结构体新增 `ParseErrorAction` 字段
- `cmd/settings_agent.go`：新增 `parse-error-action` 子命令
- `cmd/settings.go`：注册到 Agent 设置分组并显示

## 测试用例

### TC-001：参数配置正确性

**步骤**：
1. 启动 co-shell
2. 输入 `:set parse-error-action` 查看当前值
3. 输入 `:set parse-error-action exit`
4. 输入 `:set parse-error-action` 确认已变更为 `exit`
5. 输入 `:set parse-error-action retry`
6. 输入 `:set parse-error-action prompt`

**预期结果**：
- 默认值为 `retry`
- 支持 `exit`、`retry`、`prompt` 三个值
- 输入非法值时提示错误并列出可选值

**验证方法**：`go build ./... && go vet ./...` 编译通过，REPL 中交互测试

### TC-002：`:set` 命令显示

**步骤**：
1. 输入 `:set`

**预期结果**：
- Agent 分组中显示 `parse-error-action: retry 方法调用解析错误处理方式(exit/retry/prompt)`

**验证方法**：人工观察输出

### TC-003：XML 解析错误 - prompt 策略

**步骤**：
1. 设置 `:set loop-intervention off`（避免干扰）
2. 设置 `:set parse-error-action prompt`
3. 使用 `.simulate` 命令模拟一个未知方法标签的 LLM 返回，如 `<unknown_tool>data</unknown_tool>`

**预期结果**：
- 系统检测到未知方法标签，生成 `_xml_parse_error`
- 错误信息写入 `taskInstructionCache`
- 生成 `<task>` 纠错提示并追加到消息中
- 提示内容包含方法名检查和参考格式

**验证方法**：`.simulate` 测试，观察日志输出

### TC-004：XML 解析错误 - retry 策略（默认）

**步骤**：
1. 设置 `:set parse-error-action retry`
2. 使用 `.simulate` 命令模拟未知方法标签

**预期结果**：
- 不追加任何反馈消息
- 直接 continue 到下一轮迭代

**验证方法**：`.simulate` 测试，观察日志确认无纠错提示追加

### TC-005：XML 解析错误 - exit 策略

**步骤**：
1. 设置 `:set parse-error-action exit`
2. 使用 `.simulate` 命令模拟未知方法标签

**预期结果**：
- 退出迭代循环
- 向用户报告无法处理的错误信息

**验证方法**：`.simulate` 测试，确认退出循环并显示错误

### TC-006：工具执行失败测试（executeToolCall 路径）

**步骤**：
1. 设置 `:set parse-error-action prompt`
2. 使用 `.simulate` 命令执行一个参数缺失的 read_file 调用，如 `<read_file>...</read_file>`（不包含 `path` 参数）

**预期结果**：
- `formatToolError()` 生成结构化错误信息
- 错误信息包含缺失参数提示
- 结果作为 tool 结果消息返回给 LLM

**验证方法**：确认错误信息中包含了必要参数的说明

### TC-007：`:config` 配置向导显示

**步骤**：
1. 输入 `:config`
2. 进入开发者设置或智能体设置分组

**预期结果**：
- 列表中包含 `parse-error-action` 参数及其当前值和说明
- 可通过向导修改

**验证方法**：人工观察

### TC-008：编译和代码质量

**步骤**：
1. 执行 `go build ./...`
2. 执行 `go vet ./...`

**预期结果**：
- 编译无错误
- vet 无警告