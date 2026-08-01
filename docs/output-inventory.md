# co-shell 输出内容及方式摸底清单

> 状态：已完成（2026-08-01 全量调查）
> 关联文档：[docs/output-architecture.md](./output-architecture.md)（架构规划）
>
> 本文档记录 co-shell 全部输出内容与输出方式的**现状**，是审计与迁移的依据。
> 调查方式：对全部 Go 源码（排除测试/mobile/hub 子模块）做 6 种输出签名全量正则扫描 + 按包确认 + 核心文件精读 + 循环验证补查。

---

## 一、调查方法与范围

| 项 | 说明 |
|----|------|
| 扫描签名 | `fmt.Print/Printf/Println/Fprint/Fprintf/Fprintln`、`io.Print/Printf/Println`、`h.io().X()`、`a.defaultIO().X()`、`cb("event", ...)`、`os.Stdout/Stderr.Write`、`log.Print*` |
| 覆盖包 | repl / agent / cmd / main / config / store / shell / log / i18n / feishu / bridge / hub / subagent / taskplan / memory / mcp / browser / scheduler |
| 排除项 | `*_test.go`、`mobile/`、`hub/go.mod` 子模块、`bin/` 脚本（非 Go） |
| 验证轮次 | 8 轮（全局扫描 → 按包定位 → 核心文件精读 → 计数 → 遗漏补查 stderr/log 直写 → 各包零输出确认） |
| 结果 | **约 850+ 处输出调用** |

### 通道统计（按调用与涉及包）

| 通道 | 调用形式 | 典型位置 | 数量级 |
|------|---------|---------|--------|
| C1 事件回调 | `cb("event", content)` | agent/run_stream.go、loop.go、stream_response.go | ~70 |
| C2 UserIO | `io.Print/Printf/Println` | repl/userio.go、cmd/*（h.io()）、agent（a.defaultIO()） | ~300+ |
| C3 直接 fmt | `fmt.Print/Printf/Println` | repl/repl.go、cmd/mode.go、main.go、feishu、bridge | ~450+ |
| C4 stdout/子进程 tee | `os.Stdout/Stderr`、`io.MultiWriter` | agent/command_tools.go、shell/session.go、bridge/executor.go | ~20 |

### 输出调用 Top 文件

| 文件 | 调用数 | 归类 |
|------|-------:|------|
| cmd/mode.go | 161 | B |
| cmd/model.go | 113 | B |
| cmd/config.go | 104 | B |
| repl/repl.go | 81 | A/C |
| cmd/settings_db.go | 48 | B |
| main.go | 36 | C |
| cmd/session.go | 30 | B |
| cmd/co-shell-feishu-bridge/main.go | 29 | D |
| agent/settings_tools.go | 26 | B |
| agent/tools.go | 23 | B |
| repl/enhanced_input.go | 16 | C |
| repl/userio.go | 13 | 基础设施 |
| agent/run_stream.go | 13 | A |
| repl/vault.go | 10 | B |

---

## 二、A 类：LLM 迭代输出

**说明**：事件由 Agent 发送（`cb`），REPL（`repl/repl.go streamCallback`）与单命令模式（`main.go executeSingleCommand`）分别渲染。
**⚠️ 两处渲染逻辑重复**（DRY 违背）。

### A-1 事件发送端 → 接收端映射

| 事件 | 发送端（agent） | 接收端（repl） | 接收端（main） | 控制开关 | i18n |
|------|----------------|---------------|---------------|---------|------|
| content_chunk | stream_response.go:198 | repl.go:669 | main.go:1349 | ShowLlmContent | 透传（不 i18n） |
| thinking_chunk | stream_response.go:261 | repl.go:671 | main.go:1351 | ShowLlmThinking | 透传 |
| content（非流式） | loop.go:526（thinking）、run_stream 无 | repl.go:673 | — | ShowLlmContent | 透传 |
| thinking（非流式） | loop.go:526 | repl.go:677 | — | ShowLlmThinking | 透传 |
| command | run_stream.go:983 | repl.go:681 | main.go:1353 | ShowCommand | 透传 |
| output | run_stream.go（execute_command 产物） | repl.go:686（标题+分隔线框） | main.go:1355 | ShowCommandOutput | 透传 |
| tool_call（名/参数） | run_stream.go:1006 | repl.go:696 | main.go:1361 | ShowTool / ShowToolInput | 透传 |
| tool_call（Result） | run_stream.go:1053 | repl.go:696 | — | ShowToolOutput | 透传 |
| token_iter | run_stream.go:749/816/1128/1174 | repl.go:701 | main.go:1363 | TokenUsage | ✅ KeyTokenUsageDisplay/Timing |
| token_task | run_stream.go:757/822/1135 | repl.go:718 | — | TokenUsage | ❌ 硬编码（"本次任务 Token 总计"） |
| info | run_stream.go 10+ 处、loop.go 10+ 处 | repl.go:725 | — | 无条件 | ❌ 硬编码 |
| warning | run_stream.go:926 | repl.go:727 | — | 无条件 | ❌ |
| error | run_stream.go:541/578/621/1037 | repl.go:731 | main.go:1379 | 无条件 | ❌ |
| done | run_stream.go:542/579/717/760/824/1037/1137 等 | repl.go:735 | main.go:1381 | 无条件 | — |

### A-2 OpenAI 模式实时工具占用（遗漏路径提醒）

`stream_response.go:307/309`：OpenAI 模式下当 `showTool` 开启时，用 `cb("content_chunk", "[🔧 工具名(参数)]")` 实时显示工具占用。**⚠️ 该路径将工具信息混入 LLM 内容通道**，新架构中应改为 ChannelTool。

### A-3 A 类硬编码中文清单（需 i18n 化）

| 位置 | 内容 | 建议 key |
|------|------|---------|
| run_stream.go:209 | 已取消本次操作 | KeyOutputCancelled |
| run_stream.go:219 | 已暂停接收 LLM 返回的数据 | KeyOutputPaused |
| run_stream.go:220 | 是否确认取消？[C]…[Enter]… | KeyOutputCancelPrompt |
| run_stream.go:234/237 | 调试模式已开启/关闭 | KeyDebugModeOn/Off |
| run_stream.go:241/266 | 继续接收 LLM 返回数据 | KeyOutputResume |
| run_stream.go:246/271 | 重新接收数据失败 | KeyOutputRetryFailed |
| run_stream.go:247/257/272 | 已取消本次响应 | KeyOutputCancelled2 |
| run_stream.go:434 | 检测到循环输出（策略: %s） | KeyLoopDetectedSummary |
| run_stream.go:435/689 | 处理方式: %s | KeyLoopHandling |
| run_stream.go:437/971 | 发送给 LLM 的提示 | KeyLoopFeedbackSent |
| run_stream.go:439/973 | （无反馈，仅重发上下文） | KeyLoopNoFeedback |
| run_stream.go:441/690/975 | ──────── 分隔线 | 统一 UI 组件 |
| run_stream.go:469/472 | 相同错误已出现 %d 次 / 不同错误类型已达 %d 种 | KeyErrorCountPrompt |
| run_stream.go:499 | 用户取消了操作 | KeyOutputUserCancel |
| run_stream.go:541 | LLM 调用出错（parse-error-action=exit） | KeyLLMErrorExit |
| run_stream.go:549/572 | LLM 调用出错…正在重试 | KeyLLMErrorRetry |
| run_stream.go:578 | LLM 调用出错…请检查输入 | KeyLLMErrorCheckInput |
| run_stream.go:621 | 方法调用解析错误 | KeyXMLParseErrorExit |
| run_stream.go:688 | 检测到XML解析错误 | KeyXMLParseErrorSummary |
| run_stream.go:926 | 上下文超限…已跳过此轮工具执行 | KeyContextOverLimit |
| run_stream.go:1037 | 工具 %s 执行失败 | KeyToolExecFailed |
| run_stream.go:1059 | （工具调用无输出） | KeyToolNoOutput |
| run_stream.go:1211 | 你必须马上进行上下文整理… | KeyReorganizeUrgent |
| loop.go:669/779 | 发送给判定模型的完整提示词/完全返回 | KeyLoopJudgePrompt/Response |
| loop.go:841 | 检测到循环 (%s) | KeyLoopDetectEvent |
| loop.go:858/894/1032-1036 | 判定模型认为非循环/循环介入已禁用/返回… | KeyLoopJudgeResult |
| repl.go:722 | 本次任务 Token 总计: 输入=%d… | KeyTokenTaskDisplay |

**⚠️ 以上约 30 处全部为硬编码中文**，未走 i18n（`i18n.T`）。

---

## 三、B 类：交互向导/菜单输出（无开关）

**说明**：B 类全部为交互式 UI，无输出开关控制；通道在 `h.io()`（UserIO）与直接 fmt 之间混用。

### B-1 cmd 处理器输出统计

| 处理器 | 文件 | 调用数 | 通道 | i18n | 样式 |
|--------|------|-------:|------|------|------|
| 模型向导（7 步） | cmd/model.go | 113 | ✅ 统一 h.io() | ❌ 大量硬编码 | ✅/❌/⚠️ emoji + 手动步骤 |
| 工作模式管理 | cmd/mode.go | 161 | ⚠️ 部分 h.io() 部分 fmt | ❌ | 手绘框线 `──────────` |
| :config 向导 | cmd/config.go | 104 | ✅ h.io() | ⚠️ 部分 | `──` 分隔线 + `[n]` |
| 数据库配置向导 | cmd/settings_db.go | 48 | ✅ h.io() | ❌ | emoji + 步骤 |
| 会话管理 | cmd/session.go | 30 | ❌ 直接 fmt（showSession） | ❌ | 编号列表 + `*` 标记 |

### B-2 确认/对话框类输出

| 场景 | 位置 | 通道 | 说明 |
|------|------|------|------|
| 工具调用确认 | agent/command_tools.go:307 | ✅ defaultIO() | Enter/C/A/G/D/N + 风险警告 |
| 自动批准提示 | agent/tools.go（3 处） | ✅ defaultIO() | "已自动批准 %s（剩余 %d 次）" 硬编码 |
| 永久禁用工具 | agent/tools.go | ✅ defaultIO() | i18n KeyCmdConfirmDisableToolD |
| :settings 确认框 | agent/settings_tools.go | ✅ defaultIO() | `════` 标题框 + 旧/新值 + 原因 |
| 保险库向导 | repl/vault.go | ❌ fmt + readPassword | 创建/解锁/添加条目 |
| 调试模式拦截 | agent/agent.go debugIntercept | ✅ a.io | 标题 + `>` |
| 重置会话确认 | cmd/reset.go | ❌ fmt | 单行 |

**⚠️ B 类问题：**
- cmd/mode.go 161 处调用中大量 **fmt 直写**，raw 模式下无 `\r\n` 转换；
- 分隔线样式 ≥4 种（`────`/`════`/`──────────`/`────────────────────────────`）；
- 向导中大量中文硬编码未走 i18n。

---

## 四、C 类：REPL/CLI 基础设施输出（无开关）

| 内容 | 位置 | 通道 | i18n | 备注 |
|------|------|------|------|------|
| 欢迎页 | repl.go:740 printWelcome | fmt | ⚠️ 部分 | 版本/版权/logo |
| :help | repl.go:752 printHelp | fmt | ✅ | 全 i18n |
| DB 自动同步状态 | repl.go:165 syncDB | fmt | ❌ | 4 条中文 |
| `.` 前缀提示确认 | repl.go:256 | fmt | ❌ | 3 条中文 |
| :new 新会话 | repl.go:403 | fmt | ❌ | 中文 |
| :session pop 编辑 | repl.go:435 | fmt | ❌ | 中文 |
| 历史重执行 | repl.go:483 | fmt | ⚠️ 部分 | |
| 退出清理 cleanup | repl.go:791 | fmt | ⚠️ | `" MCP error: %v"` 前导空格拼接脆弱 |
| 编辑回显/光标控制 | repl/enhanced_input.go | fmt + `\033[` | — | 终端控制与内容混在一起 |
| 启动参数校验警告 | main.go:473-879 | io.ErrPrintf（stderr） | ❌ | **约 30 条** `Warning: invalid --xxx` |
| 生成 CAPABILITIES/RULES | main.go:415-439 | io | ⚠️ | 中文 |
| 免责声明 | main.go:1279-1300 | io | ✅ | |
| 单命令模式 | main.go:1330-1386 | io | ⚠️ | 事件渲染副逻辑 |
| 迁移统计 | store/pgstore.go | fmt | ❌ | ✅ history/memory |
| Sub-agent 启动 | subagent/subagent.go | fmt | ❌ | |
| Sub-agent 通讯 | agent/subagent_tools.go | fmt | ❌ | |

**⚠️ C 类问题：**
- main.go 大量警告走 stderr（`io.ErrPrintf`），与 REPL 内 emoji 前缀风格不一致；
- enhanced_input.go 的终端控制序列与业务输出混同；
- cleanup() 错误拼接格式脆弱。

---

## 五、D 类：外部集成入口输出（独立程序）

| 程序 | 文件 | 调用数 | 通道 | 角色前缀 |
|------|------|-------:|------|---------|
| feishu-bridge | cmd/co-shell-feishu-bridge/main.go | 29 | fmt | 🚀/✅/❌/📌 |
| bridge 调度 | bridge/scheduler.go、executor.go | 12 | fmt | ⚙️/✅ |
| feishu handler | feishu/handler.go | 10 | fmt | 📩/📤/⏳ |
| hub CLI | cmd/co-shell-hub/main.go | 9 | fmt + log | ===/⚠️ |
| subagent | subagent/subagent.go | 1 | fmt | 🔧 |

**⚠️ D 类问题：** 角色前缀体系与主程序（`[👤]/[🐚]/[⚙️]/[🔴]`）完全不同，无法复用 EmojiPrefixes 语义。

---

## 六、事件魔法字符串清单

全部 eventType 取值及出现位置（A 类通道的字符串字面量）：

| 事件字符串 | 发送端 | 接收端（switch-case） |
|-----------|--------|---------------------|
| `content_chunk` | stream_response.go:198/307/309 | repl.go:669、main.go:1349 |
| `thinking_chunk` | stream_response.go:261 | repl.go:671、main.go:1351 |
| `content` | （非流式走 content_chunk） | repl.go:673 |
| `thinking` | loop.go:526 | repl.go:677 |
| `command` | run_stream.go:983 | repl.go:681、main.go:1353 |
| `output` | run_stream.go（execute_command 产物） | repl.go:686、main.go:1355 |
| `tool_call` | run_stream.go:1006/1053 | repl.go:696、main.go:1361 |
| `token_iter` | run_stream.go:749/816/1128/1174 | repl.go:701、main.go:1363 |
| `token_task` | run_stream.go:757/822/1135 | repl.go:718 |
| `info` | run_stream.go 10+ 处、loop.go 669/779/841/858/894/969-975/1032-1036 | repl.go:725 |
| `warning` | run_stream.go:926 | repl.go:727 |
| `error` | run_stream.go:541/578/621/1037 | repl.go:731、main.go:1379 |
| `done` | run_stream.go:542/579/622/717/760/824/1038/1137 | repl.go:735、main.go:1381 |

---

## 七、样式不统一清单

| 样式元素 | 出现位置 | 不一致点 |
|---------|---------|---------|
| 分隔线（横线） | repl.go output 事件 | `ep.OutputSep`（`────`） |
| 分隔线（标题框） | agent/settings_tools.go | `══════════` |
| 分隔线（长） | run_stream.go:441/690/975、loop.go:975 | `────────────────────────────`（不等长） |
| 分隔线（菜单框） | cmd/mode.go | `──────────` / `──────────────────────` |
| 菜单编号 | cmd/config.go | `[n]` |
| 菜单编号 | cmd/session.go、cmd/model.go | `[n]` 但缩进/对齐不同 |
| 步骤编号 | cmd/model.go | `[1/7]`/`[2/7]` 与 `步骤: ` 混用 |
| 状态 emoji | 各文件 | ✅/❌/⚠️/ℹ️ 语义相同但调用方式不同（有的 `ep.`，有的硬编码） |

---

## 八、输入路径现状（双模式支持依据）

> 本节记录输入侧三套路径的现状，是 `docs/output-architecture.md` 3.6 节与 P2.5 阶段的改造依据。
> 响应式 = 单一按键监测（含方向键等控制字符）；标准流式 = 标准输入输出。

### 8.1 三套输入路径

| 输入路径 | 文件 | 现状 |
|---------|------|------|
| 标准流式 | `repl/userio.go` `StdioIO` | bufio.Scanner 行读，`--input-mode stdio` |
| 按键级 | `repl/userio.go` `EnhancedIO` | raw 模式 + `ReadKey` 解析 ESC 序列（方向键/退格/历史） |
| 后台监控 | `repl/repl_esc_posix.go` `startESCMonitor` | 独立 goroutine，unix.Poll 100ms 轮询，只认 `0x1b`/`0x03` |
| 窗口 | `repl/repl_esc_windows.go` | **no-op**（无 unix.Poll），Windows 上 ESC/Ctrl+C 监控不存在 |

### 8.2 关键实现细节

- `EnhancedIO.ReadKey`（repl/userio.go:203-254）：raw 模式单字节读，处理 ESC 序列（`ESC + [`/`ESC + O`），吞掉方向键等完整转义序列后继续等待；
- `EnhancedInput`（repl/enhanced_input.go）：行编辑器，方向键/退格/历史/清屏；
- `startESCMonitor`（repl/repl_esc_posix.go:20-98）：`IsReading()` + `IsCommandRunning()` 两个标志规避与 ReadLine/ReadKey 及子进程的 stdin 竞争。

### 8.3 同步阻塞输入点清单（响应式迁移障碍）

以下 API 为同步阻塞读，P2.5 后将逐步迁移为消费 `InputSource` 事件流：

| 位置 | API | 用途 |
|------|-----|------|
| agent/command_tools.go:316 | `io.ReadLine()` | 工具调用确认（Enter/C/A/G/D/N） |
| agent/run_stream.go:226 | `io.ReadLine()` | ESC 中断确认 |
| agent/run_stream.go:493 | `io.ReadLine()` | 错误反复出现确认 |
| agent/vault.go（多处） | `io.ReadLine()` / readPassword | 保险库向导 |
| cmd/model.go（wizardPrompt*） | `io.ReadLine()` | 模型配置向导 |
| cmd/mode.go | `io.ReadLine()` | 模式管理向导 |
| cmd/config.go | `io.ReadLine()` | :config 向导 |
| cmd/session.go | `io.ReadLine()` | 会话管理 |
| main.go:1287 | `io.ReadLine()` | 免责声明确认 |
| agent/simulate.go | `io.ReadLine()` | 模拟输入 |

---

## 九、基础设施现状

| 设施 | 现状 | 缺口 |
|------|------|------|
| UserIO | 4 方法输出 5 方法输入，4 实现 | 无通道/级别参数 |
| EmojiPrefixes | 15 个角色前缀（emoji/纯文本双模式） | 只覆盖 A 类部分角色；B/C/D 大量绕过 |
| 输出开关 | 7 个 show 开关 + token-usage | 只覆盖 A 类 |
| 事件回调 | `(string, string)` 魔法字符串 | 无类型枚举、无结构体 |
| i18n | 139 keys（UI 消息在 zh.go/en.go） | A 类约 30 处、B 类大量、C 类约 40 处、D 类全部硬编码；**目标 100% i18n**（P4.5 归零，见架构文档 1.2/四） |
| UI 样式组件 | 无 | ≥4 种分隔线、无菜单/面板组件 |

---

## 十、输入路径补充说明

- **标准流式输入**（`StdioSource` 提案）：行级事件 + EOF 感知，非 raw 模式，可 pipe；
- **响应式输入**（`RawKeySource` 提案）：按键级事件（`arrow_up/down/left/right`、`esc`、`ctrl_c`…），复用现有 EnhancedInput 的 ESC 序列解析；
- **ESC 监控改造**：从独立 poll goroutine 改为统一事件流消费者，消除 stdin 竞争并补齐 Windows；
- **核心目标**：统一 `--input-mode` 参数（`stdio`/`tui`/`web`）一键切换整套 I/O 通道——通过 `SessionIO` 管道（InputSource+EventSink 配对）实现，Agent 核心零改动；`--input-mode`（整体形态）与 show-xx 开关（类别启停）正交，详见 `docs/output-architecture.md` 3.6.5/3.6.6。

### i18n 豁免项（100% i18n 的边界）

> 同步自 `docs/output-architecture.md` 六章，审计脚本将始终对这些类别豁免。

| 类别 | 说明 |
|------|------|
| LLM 透传内容 | 流式输出、工具返回数据——内容由 LLM/命令产生，非 co-shell 文案 |
| 数据内容 | 命令输出、文件内容、会话导出、日志文件——非 UI 文案 |
| 终端控制序列 | `\033[2J` 等 ESC 转义——非可见文本 |
| 单字节提示 | `[Enter] [C] [A]` 等按键提示可走 i18n，但单字符回显（按键本身）不 i18n |

**验收口径**：`bin/output_audit.sh` 第 3 项「Hardcoded Chinese」在上表豁免之外应最终归零（P4.5）。

---

## 十一、循环验证记录

| 轮次 | 验证方式 | 结果 |
|------|---------|------|
| 1 | `fmt.Xxx` 全库扫描 | 292 处匹配，按包分布 |
| 2 | `io.Xxx` / `h.io().Xxx` 扫描 | cmd/model、mode、config、settings_db 为主 |
| 3 | 核心文件精读（repl.go/run_stream.go/command_tools.go/tools.go/subagent.go） | 确认渲染逻辑与确认流程 |
| 4 | 计数脚本（各文件调用数排序） | Top 文件确认，~850 总量 |
| 5 | `cb("event")` 全量 grep | 70 处事件发送，含 loop.go 判定模型路径 |
| 6 | `os.Stdout/Stderr` + `log.Print*` 补查 | 确认 C4 通道只存在于 command_tools/shell/bridge |
| 7 | 各包零输出确认（mcp/scheduler/browser/taskplan/memory/mode） | 0 输出调用，纯逻辑包 |
| 8 | i18n 文件规模确认 | 139 keys；zh/en.go 各 14 条 UI 翻译（其余为系统提示词） |

**调查中发现的遗漏路径（重要）**：`stream_response.go:307/309` OpenAI 模式实时工具占用会输出 `[🔧 工具名]` 到 content_chunk——此路径在旧 ENHANCEMENT-126 清单中未记录，本次已补充。