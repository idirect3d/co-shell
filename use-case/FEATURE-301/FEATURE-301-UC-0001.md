# FEATURE-301 输出/输入事件双枚举重构（P1）测试用例

## 基本信息

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-301 |
| 任务名称 | 输出/输入事件双枚举重构（P1） |
| 类型 | 新特性（纯重构） |
| 版本 | v0.7.0 |
| 架构文档 | docs/output-architecture.md（3.3/3.6/3.7） |

**目标**：事件魔法字符串 → 常量引用；**验收**：行为零变化。
**基线**：`bin/output_audit.sh` 第 2 项魔法事件 = **63**。

---

## 一、编译与回归

### UC-0001: 编译通过
`go build ./...` 退出码 0。

### UC-0002: 全量测试通过
`go test ./...` 无 FAIL。

### UC-0003: 事件常量值等于原字符串（table-driven）
13 个常量 EventContentChunk/EventThinkingChunk/EventContent/EventThinking/EventCommand/EventOutput/EventToolCall/EventTokenIter/EventTokenTask/EventInfo/EventWarning/EventError/EventDone 分别等于 content_chunk/thinking_chunk/content/thinking/command/output/tool_call/token_iter/token_task/info/warning/error/done。

### UC-0004: InputKind 常量完整
InputLine/InputKey/InputArrowUp/InputArrowDn/InputArrowLt/InputArrowRt/InputEsc/InputCtrlC/InputTab/InputBackspace/InputEnter/InputEOF 非空且互不重复。

---

## 二、渲染行为不变（golden）

### UC-0005: REPL 渲染 golden 一致
提取可测渲染函数，固定事件序列渲染与 repl/testdata/render_tui.golden 逐字节一致。开发第一步先录基线 golden 再重构。

### UC-0006: 单命令渲染 golden 一致
executeSingleCommand 渲染与 main/render_single_cmd.golden 一致。

### UC-0007: cb 调用点全部常量化
audit 第 2 项 63 降至 0；特殊残留须白名单说明。

### UC-0008: 接收端 switch 用常量
repl.go 与 main.go 的 switch eventType case 全改 agent.Event 常量；go vet 通过。

---

## 三、行为一致性

### UC-0009: REPL 与单命令渲染一致
二者 golden 输出相同（P2 渲染合并前置验证）。

### UC-0010: 真实 REPL 冒烟（手动）
增强模式启动，执行简单指令，流式/工具/命令输出正常，无乱码重复。需 LLM API Key。

---

## 四、审计收尾

### UC-0011: audit 全项回归对比
仅第 2 项变化（63→0）；fmt=206、中文=1555、同步输入=19、i18n=1 全不变。

### UC-0012: git diff 范围核查
改动仅限 events.go/input.go（新）+ loop/run_stream/stream_response/repl/main + 测试文件。