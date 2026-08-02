# FEATURE-302 Out + RenderCommand 抽象 + 渲染合并（P2）测试用例

## 基本信息

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-302 |
| 任务名称 | Out 接口落地 + RenderCommand 抽象 + 渲染合并（P2） |
| 类型 | 新特性（抽象重构，行为零变化） |
| 版本 | v0.7.0 |
| 架构文档 | docs/output-architecture.md（3.2/3.7/3.4/6.2） |

**目标**：统一输出通道（Out 接口）+ 语义化渲染指令（RenderCommand）；REPL/main 渲染合并为单一入口；`--input-mode enhanced→tui` 别名。
**验收基线**：P1 后 audit = fmt=206 / 魔法事件=0 / 中文=1555 / 同步输入=19 / i18n=1；两份 golden 已就绪。
**回归原则**：渲染结果零变化（golden 逐字节一致），仅内部通道与渲染器实现变化。

---

## 一、编译与回归

### UC-0001: 编译通过
`go build ./...` 退出码 0。

### UC-0002: 全量测试通过
`go test ./...` 无 FAIL（既有 file_tools 失败除外，与 FEATURE-265 相关）。

### UC-0003: Out 接口与 ChannelID/Level 枚举完整（table-driven）
ChannelLLM/ChannelTool/ChannelCommand/ChannelDebug/ChannelWizard/ChannelSystem 等常量非空且互不重复；Level Info/Success/Warning/Error/Debug 递增有序；Out 接口含 Emit + 5 个便捷方法（Info/Success/Warning/Error/Debug）。

### UC-0004: RenderCommand/RenderKind 常量完整（table-driven）
RenderText/RenderTitle/RenderBox/RenderMenu/RenderStep/RenderSep/RenderDialog/RenderProgress 非空且互不重复；RenderCommand 结构含 Kind/Content/Level/Region/Meta 字段。

---

## 二、渲染行为不变（golden）

### UC-0005: REPL 渲染 golden 一致
P1 的 `repl/testdata/render_tui.golden` 在合并后仍逐字节一致（repl/render_test.go 继续通过）。

### UC-0006: 单命令渲染 golden 一致
P1 的 `testdata/render_single_cmd.golden` 仍逐字节一致（render_test.go 继续通过）。

### UC-0007: 合并后单一入口渲染与 REPL 输出一致
LineRenderer 消费同一事件序列的输出 == 合并前 repl.streamCallback 的输出（新 renderer 快照 vs P1 golden，**循环模式**：13 个事件逐一断言）。

### UC-0008: TerminalOut 便捷方法快照（循环模式）
TerminalOut.Info/Success/Warning/Error/Debug 对固定 Channel 的渲染输出逐字节匹配 EmojiPrefixes 前缀 + i18n 文案组合快照（table-driven 循环）。

---

## 三、行为一致性（功能回归）

### UC-0009: `--input-mode` parseInputMode 兼容（table-driven）
`enhanced`→`tui`、`tui`→`tui`、`stdio`→`stdio`、未知值→报错并提示合法值；config.json 旧值 `enhanced` 加载归一化为 `tui` 不回写。

### UC-0010: REPL 冒烟（手动）
`--input-mode tui` 启动 REPL 执行简单指令；`:set input-mode <tui|stdio>` 切换后提示符/流式/工具输出正常；ESC 中断与 Ctrl+C 取消功能回归。需 LLM API Key。

### UC-0011: stdio 管道无 ANSI 污染
`--input-mode stdio` 下 `co-shell "简单问题"` 的 stdout 不含 `\033[` 转义序列（grep 验证）；A 类输出前缀（emoji/text）正常。

---

## 四、审计收尾

### UC-0012: audit 全项回归对比
audit 5 项相比 P1：fmt=**206→204** / 魔法事件=0 / 中文=1555 / 同步输入=19 / i18n=1。
- **fmt 206→204 为合规改进**：渲染合并后 repl.streamCallback 与 main.renderSingleCmdEvent 从直接 fmt 委托到统一 StreamRenderer（走 UserIO），净消除 2 处直接 fmt 输出——这正是 P2「统一输出通道」目标，非新增不合规。
- 其余 4 项与 P1 完全一致，无新增不合规。
- git diff 范围仅限 out.go/command.go/stream_renderer.go（新）+ config.go + repl/repl.go + main.go + 测试文件。

---

## 循环模式说明

- UC-0007/UC-0008 采用 table-driven 循环：同一渲染器对 13 个事件 / 5 个便捷方法的快照逐一断言，任一事件输出变化即失败
- UC-0009 对 5 种输入模式取值循环断言 parseInputMode 返回值/错误
- 运行时用例数 ≥ 26（13 事件 + 5 便捷方法 + 5 输入模式 + 3 golden/audit 收尾），满足循环模式质量目标