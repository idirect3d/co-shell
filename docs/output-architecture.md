# co-shell 输出架构规划

> 状态：规划中（草案）
> 创建日期：2026-08-01
> 作者：L.Shuang
> 关联文档：[docs/output-inventory.md](./output-inventory.md)（现状摸底清单）、[repl/output-control-inventory.md](../repl/output-control-inventory.md)（ENHANCEMENT-126 历史清单）
>
> 本文档是输出统一化改造的**架构蓝图**，为后续多个分支的迭代开发提供设计与验收依据。
> 实施时需遵循项目规则：输出走 `i18n.T()`、用户交互走 `UserIO`、逐个阶段可独立验证。

---

## 一、背景与目标

### 1.1 为什么要改

现有输出体系存在 4 个结构性缺陷，阻碍 UI 规范化和扩展性：

| # | 缺陷 | 具体表现 |
|---|------|---------|
| 1 | **通道混用** | 同一 REPL 界面同时使用「事件回调」「UserIO」「直接 fmt」三种通道，`repl.go` 走 UserIO（做 `\r\n` 转换）、`main.go` 走 fmt（不做） |
| 2 | **开关覆盖不全** | 7 个 show 开关只覆盖 LLM 迭代输出（A 类）；交互向导（B 类）、REPL 基础设施（C 类）、外部入口（D 类）完全无开关 |
| 3 | **样式不统一** | 至少 4 种手写分隔线（`────`/`════`/`──────────`/`────────────────────────────`）；菜单、向导、框线各写各的 |
| 4 | **i18n 覆盖不全** | A 类约 20 处、B 类向导文案、C 类 main.go 约 50 处、D 类入口存在大量硬编码中文 |

### 1.2 目标

1. **近期**：统一输出通道抽象，让所有输出（A/B/C/D）都能被分类、分级、开关控制；
2. **中期**：支持**区域化渲染**（UI 中不同信息输出到不同区域），为 Web 界面打基础；
3. **远期**：通过结构化事件 + 渲染器分离，实现 JSON/Web 渲染器；
4. **贯穿全程（硬性目标）**：**100% i18n 化**——所有用户可见文本必须走 `i18n.T()/TF()`（除下方「i18n 豁免项」），P5 结束时硬编码中文计数归零。

### 1.3 非目标（明确不做）

- 不改变 LLM 返回内容本身（流式内容直接透传，不 i18n 包装）；
- 不要求一次性迁移所有调用点（按阶段推进，每阶段可独立合入）。

---

## 二、现状摘要

完整清单见 [docs/output-inventory.md](./output-inventory.md)，要点如下：

- **输出调用总量**：约 850+ 处（Go 源码，排除测试/mobile/hub 子模块）；
- **4 种底层通道**：
  - `C1 事件回调` — `cb("event", content)`，StreamCallback 类型（`agent/loop.go:58`）；
  - `C2 UserIO` — `io.Print/Printf/Println`，4 种实现（StdioIO/EnhancedIO/DefaultUserIO/fmtIO）；
  - `C3 直接 fmt` — `fmt.Print/Printf/Println`，遍布各包；
  - `C4 stdout/stderr 直写` — 子进程 tee（`agent/command_tools.go`）、shell session（`shell/session.go`）、bridge（`bridge/executor.go`）；
- **4 类输出内容**：A=LLM 迭代、B=向导/菜单、C=REPL/CLI 基础设施、D=外部集成入口；
- **10 个问题点**：A 类 3 个（渲染重复/事件硬编码中文/魔法字符串）、B 类 3 个（fmt 直写/样式不一/硬编码）、C 类 3 个（stderr 不一致/终端控制混入/拼接脆弱）、D 类 1 个（角色体系不同）。

---

## 三、目标架构

### 3.1 分层模型

```
┌─────────────────────────────────────────────────┐
│  调用方（agent.run_stream / cmd handlers / repl） │
└──────────────┬──────────────────────────────────┘
               │ 调用 Out 接口（带 Channel + Level）
┌──────────────▼──────────────────────────────────┐
│  Out 接口层（分类×级别过滤 → 路由）                │
│  ┌─────────────┐  ┌─────────────┐               │
│  │ OutputFilter│  │ Router      │               │
│  └─────────────┘  └──────┬──────┘               │
└──────────────────────────┼──────────────────────┘
                           │ StreamEvent/内容
              ┌────────────▼────────────┐
              │  渲染器（可插拔）        │
              │  Terminal | JSON | Web  │
              └────────────┬────────────┘
                           ▼
                    stdout / WebSocket / DOM
```

### 3.2 Out 接口设计（草案）

定义于新文件 `agent/out.go`（或 `repl/out.go`，取决于归属）。核心思想：**所有输出统一走一个入口，带「分类」「级别」两个维度，由过滤器决定是否真正渲染。**

```go
// ChannelID 指明输出的业务分类，用于过滤与区域化路由。
type ChannelID string

const (
    ChannelLLM      ChannelID = "llm"      // LLM 内容/思考
    ChannelTool     ChannelID = "tool"     // 工具调用、参数、返回
    ChannelCommand  ChannelID = "command"  // 系统命令、命令输出
    ChannelDebug    ChannelID = "debug"    // 调试/循环检测/内部诊断
    ChannelWizard   ChannelID = "wizard"   // 交互向导/菜单
    ChannelSystem   ChannelID = "system"   // 欢迎/帮助/状态/清理
    ChannelMCP      ChannelID = "mcp"      // MCP 服务器状态
    ChannelMemory   ChannelID = "memory"   // 记忆管理
    ChannelTaskPlan ChannelID = "taskplan" // 任务计划
    ChannelDB       ChannelID = "db"       // 数据库/同步/迁移
    ChannelBridge   ChannelID = "bridge"   // feishu/bridge/hub 入口
    ChannelSubAgent ChannelID = "subagent" // 子代理
)

// Level 指明输出的重要级别，用于样式（颜色/前缀）与可选过滤。
type Level int

const (
    LevelInfo Level = iota
    LevelSuccess
    LevelWarning
    LevelError
    LevelDebug
)

// Out 是所有面向用户输出的统一入口。
// 实现需满足：
//   - 按 Channel 过滤（对应 show-xx 开关）
//   - 按 Level 渲染（emoji 前缀/颜色/粗细）
//   - 支持 Area 区域化（阶段三）
type Out interface {
    Emit(ch ChannelID, lv Level, format string, args ...interface{})
    // 便捷方法（保留可读性）
    Info(ch ChannelID, format string, args ...interface{})
    Success(ch ChannelID, format string, args ...interface{})
    Warning(ch ChannelID, format string, args ...interface{})
    Error(ch ChannelID, format string, args ...interface{})
    Debug(ch ChannelID, format string, args ...interface{})
}
```

**与现有 7 开关的映射**（阶段二落地时实施）：

| 现有开关 | 新通道 | 说明 |
|---------|--------|------|
| show-llm-thinking | ChannelLLM + LevelInfo | 思考内容 |
| show-llm-content | ChannelLLM + LevelInfo | 主内容 |
| show-tool | ChannelTool + LevelInfo | 工具名 |
| show-tool-input | ChannelTool + LevelDebug | 工具参数 |
| show-tool-output | ChannelTool + LevelInfo | 工具返回 |
| show-command | ChannelCommand + LevelInfo | 命令行 |
| show-command-output | ChannelCommand + LevelInfo | 命令输出 |

### 3.3 事件结构化（StreamEvent）

现状 `StreamCallback` 是 `(eventType string, content string)`，事件类型是魔法字符串。阶段一将其升级为常量 + 结构体（**保持调用签名兼容**，先加常量，再逐步迁移结构体）：

```go
// 事件类型常量（agent/events.go）
const (
    EventContentChunk  = "content_chunk"   // LLM 流式内容
    EventThinkingChunk = "thinking_chunk"  // LLM 流式思考
    EventContent       = "content"         // LLM 非流式内容
    EventThinking      = "thinking"        // LLM 非流式思考
    EventCommand       = "command"         // 系统命令
    EventOutput        = "output"          // 命令输出
    EventToolCall      = "tool_call"       // 工具调用（名/参数/返回）
    EventTokenIter     = "token_iter"      // 迭代 token
    EventTokenTask     = "token_task"      // 任务 token
    EventInfo          = "info"            // 信息
    EventWarning       = "warning"         // 警告
    EventError         = "error"           // 错误
    EventDone          = "done"            // 完成标记
)

// StreamEvent 结构化负载（阶段二/三使用）
type StreamEvent struct {
    Type    ChannelID // 事件类型
    Channel ChannelID // 业务分类
    Level   Level     // 级别
    Content string
    Meta    map[string]string // 预留：工具名、耗时、token 等
}
```

### 3.4 渲染器分离

- **TerminalRenderer**：现有 emoji 前缀（`config.GetEmojiPrefixes`）+ 手动分隔线 + raw `\r\n` 转换。合并 `repl.go streamCallback` 与 `main.go executeSingleCommand` 的重复逻辑。
- **JSONRenderer**（远期）：事件序列化为 JSON 行，供 web 前端/CI 消费。
- **WebRenderer**（远期）：映射到 DOM 区域。

### 3.5 区域（Region）模型（阶段三）

| 区域 | 内容 | 终端表现 | Web 表现 |
|------|------|---------|---------|
| `RegionPrompt` | 输入行 | `[👤]>` | 输入框 |
| `RegionMain` | LLM 内容（不含前缀） | 默认文本 | 主内容区 |
| `RegionMeta` | 命令/工具/token 元信息 | `[🔴]/[⚙️]/Token` | 折叠面板 |
| `RegionStatus` | 进度/警告 | `[⚠️]` | 顶部状态条 |
| `RegionWizard` | 向导/菜单 | 框线+编号 | 独立弹层 |

**设计要点**：`RegionMain` 输出**不带前缀**，避免污染纯文本，便于后续 pipe 集成与 Web 渲染。

### 3.6 输入模式架构（InputSource / InputEvent）

> 本节是「标准流式 / 响应式」双输入模式的核心设计。**响应式 = 单一按键监测（含上下左右等控制字符）；标准流式 = 标准输入输出**。

#### 3.6.1 现状：三套输入路径竞争

| 输入路径 | 文件 | 现状 |
|---------|------|------|
| 标准流式 | `repl/userio.go` `StdioIO` | bufio.Scanner 行读（`--input-mode stdio`） |
| 按键级 | `repl/userio.go` `EnhancedIO` | raw 模式 + `ReadKey` 解析 ESC 序列（方向键/退格/历史） |
| 后台监控 | `repl/repl_esc_posix.go` `startESCMonitor` | **独立 goroutine**，unix.Poll 100ms 轮询，只认 `0x1b`/`0x03` |

**三个结构性缺陷（响应式必须解决）：**
1. **竞争**：ESC 监控 goroutine 与 ReadKey/ReadLine 同时读 stdin，靠 `IsReading()` + `IsCommandRunning()` 两个标志 + 100ms 轮询规避，脆弱且非实时；
2. **不可扩展**：增强/stdio 是配置级切换，非统一输入流，无法接入第三种输入源（Web 事件/远程推送）；
3. **平台缺失**：`repl/repl_esc_windows.go` 的 `startESCMonitor` 是 no-op（无 unix.Poll），**Windows 上 ESC/Ctrl+C 监控不存在**。

**资产**：`EnhancedIO`/`EnhancedInput` 已具备方向键等控制字符的完整解析，响应式底层已存在，缺的是抽象成可复用输入事件流并消除竞争。

#### 3.6.2 目标：统一 InputSource / InputEvent

```
stdin / WS / UI事件
   ↓
单一 Reader 循环（唯一读取者）
   ↓ 产出统一 InputEvent 流
InputEvent{ Kind, Data }
   ├── 后台消费者：ESC / Ctrl+C 监控（永不绝收输入）
   ├── ReadLine 消费者（行编辑器）
   └── ReadKey 消费者（等待下一个按键/控制字符）
```

```go
// InputKind 统一输入事件类型
type InputKind string

const (
    InputLine     InputKind = "line"      // 整行输入（标准流式）
    InputKey      InputKind = "key"       // 单键（响应式）
    InputArrowUp  InputKind = "arrow_up"  // 方向键（控制字符解析后）
    InputArrowDn  InputKind = "arrow_dn"
    InputArrowLt  InputKind = "arrow_lt"
    InputArrowRt  InputKind = "arrow_rt"
    InputEsc      InputKind = "esc"       // ESC（中断）
    InputCtrlC    InputKind = "ctrl_c"    // Ctrl+C（取消）
    InputTab      InputKind = "tab"
    InputBackspace InputKind = "backspace"
    InputEnter    InputKind = "enter"
    InputEOF      InputKind = "eof"       // 标准流式 EOF
)

// InputEvent 由统一 Reader 循环产出
type InputEvent struct {
    Kind InputKind
    Data string // line 内容 / key 对应字符等
}

// InputSource 是跨模式输入抽象。
type InputSource interface {
    NextEvent(ctx context.Context) (InputEvent, error)
}
```

- **标准流式** = `StdioSource`：`InputLine`/`InputEOF`，非 raw 模式，可 pipe；
- **响应式** = `RawKeySource`：`InputKey`/`InputArrow*`/`InputEsc`/…，复用现有 EnhancedInput 解析；
- **消除竞争**：ESCMonitor 从独立轮询 goroutine 改为**同一事件流的消费者**，不再抢读 stdin，顺带修复 Windows 缺失；
- **未来扩展**：`WSSource`（Web 前端 WebSocket）实现同一接口，输入侧即对接 Web UI。

#### 3.6.3 `UserIO` 定位（重要）

- `UserIO` 仍是 **Terminal 专属适配器**（raw 模式、`\r\n` 输出、ReadLine 编辑）；
- `InputSource` 是**跨模式通用层**；`UserIO.ReadLine/ReadKey` 在 P2.5 后逐步迁移为消费 `InputSource` 事件。

#### 3.6.4 标准流式 / 响应式 模式矩阵

| 层 | 标准流式 | 响应式 |
|----|---------|--------|
| 输入 | `StdioSource`（行/EOF，pipe 友好） | `RawKeySource`（单键/控制字符） |
| 输出 | Terminal 纯净文本 / JSON-Lines（无 ANSI 污染） | StreamEvent 增量推送（chat/tool/status 分区） |
| 交互请求 | `AskUserBlocking`（同步适配器，行为不变） | AskUser 事件对（UI 自主响应，Agent 挂起） |
| ESC/Ctrl+C | 不适用（脚本环境） | 统一事件流消费者（含 Windows 补齐） |

#### 3.6.5 统一 `--input-mode`：一条参数切换整套 I/O 通道（核心目标）

> **目标**：通过单一 CLI 参数 `--input-mode <mode>`，在三种 I/O 组合之间切换，**核心 Agent 零改动**；不新增松散的全局面板开关。

| `--input-mode` | 输入源 InputSource | 输出端 EventSink | 渲染器 | 适用场景 |
|----------------|-------------------|-----------------|--------|---------|
| `stdio`（默认） | `StdinSource`（行/EOF，可 pipe） | `StdoutSink`（JSON-Lines / 纯净文本） | `StreamRenderer` | 脚本 / CI / 管道 / 标准终端 |
| `tui` | `RawKeySource`（含方向键/ESC 等控制字符） | Terminal 分区域渲染 | `RegionRenderer` | 交互式终端（现有 Enhanced） |
| `web` | `WSSource`（WebSocket 消息） | `WSSink`（分区推送 / JSON 事件） | `WebRenderer` | Web 服务界面（远期） |

**单一事实来源**：InputSource 与 EventSink 通过**同一个 `SessionIO` 管道（Pairing）** 配对，Agent/SessionRunner 只面向该管道编程。切换 `--input-mode` 只替换管道两端实现：

```go
// 统一会话管道：一条管道同时承载上行输入与下行输出。
// 切换 --input-mode 即替换 Pipe() 返回的 输入源+输出端 对。
type SessionIO interface {
    Input()  InputSource
    Output() EventSink
}

var sessionFactories = map[string]func(cfg *config.Config) (SessionIO, error){
    "stdio": newStdioSession, // StdinSource → StdoutSink(JSON-Lines)
    "tui":   newTUISession,   // RawKeySource → TerminalRegionSink
    "web":   newWebSession,   // WSSource → WSSink
}
```

- **API 一致性**：三种模式均以「同一套结构化事件进出 Agent」为契约，仅外层 I/O 载体不同；
- **web 模式实现低成本**：复用 `web` 管道 = 起一个内嵌 HTTP 服务 + WSSource/WSSink + WebRenderer，无需改动 Agent 核心；
- **未来扩展**：新增模式只需在 `sessionFactories` 注册，`--input-mode` 自动支持。

#### 3.6.6 分类开关（局部启停）与 `--input-mode`（全局模式）的关系

- `--input-mode`：**整体 I/O 形态**（一次切换全部通道，默认 `stdio`）；
- show-xx 开关：**单一类别启停**（在选定模式下按需显示/隐藏 LLM/工具/命令等）；
- 二者正交：分类开关在任意模式下都生效，模式决定「如何承载」、开关决定「显示什么」。

### 3.7 RenderCommand 渲染指令抽象（核心升级）

> 决策依据：tui 需同时支持「行模式」与「全屏重绘」两种形态——UI 组件必须与渲染方式解耦。
> 原则：**各 UI 组件（Menu/Box/Step/Dialog）只发布语义化渲染指令，不关心谁消费**；ANSI/颜色/坐标全部收敛到渲染器内部，业务层零感知。

```
UI 组件（Menu / Box / Step / Dialog / Progress）
   ↓ 产出 RenderCommand 流（语义化，不含 ANSI / 字符布局）
RenderCommand{ Kind, Content, Level, Region, Meta }
   ├── LineRenderer      （行模式：前缀+分隔线，tui v1）
   ├── FullScreenRenderer（全屏重绘：坐标/窗口管理，tui v2，可选分支）
   ├── StreamRenderer    （stdio：JSON-Lines / 纯净文本）
   └── WebRenderer       （web：DOM 分区推送）
```

```go
// RenderKind 语义化渲染指令类型
type RenderKind string

const (
    RenderText    RenderKind = "text"     // 纯文本信息
    RenderTitle   RenderKind = "title"    // 标题
    RenderBox     RenderKind = "box"      // 框/面板（Box(title)）
    RenderMenu    RenderKind = "menu"     // 菜单（Menu(items)）
    RenderStep    RenderKind = "step"     // 向导步骤（Step(n, name)）
    RenderSep     RenderKind = "sep"      // 分隔线
    RenderDialog  RenderKind = "dialog"   // 对话框/确认框
    RenderProgress RenderKind = "progress" // 进度提示
)

// RenderCommand 由 UI 组件产出，被各渲染器消费。
type RenderCommand struct {
    Kind    RenderKind
    Content string
    Level   Level       // 用于颜色/权重（渲染器内部应用）
    Region  Region      // 区域路由（阶段三）
    Meta    map[string]string // 预留：菜单项、步骤号等
}

// Renderer 是渲染指令的消费者。
type Renderer interface {
    Render(cmd RenderCommand) error
}
```

**核心收益**：
- **tui v1（LineRenderer）与 tui v2（FullScreenRenderer）** 消费同一 RenderCommand，仅渲染实现不同；
- **P3（UI 组件）与 P5（渲染器）完全解耦**——组件层零感知模式差异；
- **新增模式 = 新增 Renderer**，组件层零改动，正是「不同模式只是输出方式不同」。

### 3.8 零新增依赖约束（硬性规则）

> 项目规范：尽力降低第三方依赖、严禁来源不明/过时库。本方案所有功能**必须坚持 Go 原生实现，零新增依赖**。

| 能力 | 使用的现有组件 | 新增 |
|------|---------------|:---:|
| WebSocket（web 模式） | `github.com/gorilla/websocket`（已在 go.mod v1.5.3） | 无 |
| HTTP 服务（web 静态页） | 标准库 `net/http` | 无 |
| JSON 事件/渲染 | 标准库 `encoding/json` | 无 |
| 终端 raw 模式 / SIGWINCH | `golang.org/x/sys`（已在）POSIX；`syscall`（Windows） | 无 |
| 单键 / ESC 序列解析 | 现有 `repl/enhanced_input.go` 逻辑 | 无 |
| 全屏重绘（tui v2） | **原生字节缓冲 + ANSI 光标定位**（`\033[H`/`\033[{r}c`），**禁用 tview/tcell** | 无 |

**禁止引入**：tview/tcell（全屏 TUI）、cobra/pflag（CLI）、任何新的 UI/输入库。

---

## 四、分阶段实施计划

> 每个阶段一个独立分支，可单独合入并验收。

### P1：事件枚举重构（纯重构，零行为变化）

**目标**：消除魔法字符串，为输出与输入事件结构体打基础（**双枚举**）。

- 新增 `agent/events.go`：输出事件类型常量（`EventContentChunk/EventCommand/...`）+ `StreamEvent` 结构体；
- 新增 `agent/input.go`（或 `repl/input.go`）：`InputKind` 常量 + `InputEvent` 结构体（`InputLine/InputKey/InputArrow*/InputEsc/InputCtrlC/InputEOF...`）；
- 将 `agent/loop.go / run_stream.go / stream_response.go` 中所有 `cb("xxx", ...)` 字符串替换为输出常量；
- `repl/repl.go`、`main.go` 的 `streamCallback` switch-case 同步用常量；
- **验收**：`go build ./... && go test ./agent/...` 通过；行为无变化（可对照 git diff 只改字符串字面量）。

### P2：Out 接口落地 + RenderCommand 抽象 + 渲染合并

**目标**：统一输出通道，引入 RenderCommand 渲染指令（3.7），消除 REPL/main 双渲染。

- 新增 `agent/out.go`：`Out` 接口 + `TerminalOut` 实现（基于 EmojiPrefixes + raw 转换）；
- 新增 `agent/command.go`：`RenderCommand`/`RenderKind` 结构（3.7 草案落地）；UI 组件层（`Box/Menu/Step/Sep/Dialog`）从此只产出 RenderCommand；
- `repl/repl.go streamCallback` 与 `main.go executeSingleCommand` 合并为「事件→渲染器」单一入口（先按 LineRenderer 语义合并，ANSI 收敛到渲染器）；
- 事件处理器/向导的 `h.io()` 返回带 Out 的包装，内部将输出转为 RenderCommand；
- **验收**：`:set` 的开关注入仍生效；A 类输出渲染结果与改动前一致（终端截图对比 + 3.7 RenderCommand 流水线单测）；纯文本路径（`--input-mode stdio`）输出无 ANSI 污染。

### P2.5：输入统一（响应式核心奠基）

**目标**：合并三套输入路径为单一 Reader + InputSource，消除 ESC 监控竞争，补齐 Windows。

- 新增 `InputSource` 接口及 `StdioSource`/`RawKeySource` 实现（RawKeySource 复用现有 EnhancedInput 的 ESC 序列解析）；
- 重构 REPL 主循环：单一 Reader goroutine 消费 stdin，产出 `InputEvent` 分发到「ESC/Ctrl+C 监控」「行编辑器」「单键等待」三类消费者；
- ESC 监控从独立 poll goroutine 改为事件流消费者（`repl_esc_posix.go` 改造，`repl_esc_windows.go` 补齐）；
- `UserIO.ReadLine/ReadKey` 逐步改为访问 `InputSource`（保持行为兼容，先保留原接口）；
- **验收**：增强模式下方向键/退格/历史功能回归；ESC 中断与 Ctrl+C 取消在 POSIX 与 Windows 均生效；`--input-mode stdio` 管道模式行为不变。

### P3：向导迁移（B 类）+ i18n 归零第一部分

**目标**：cmd 5 大 handler（model/mode/config/settings_db/session）迁移到 Out，消除 fmt 直写与硬编码。

- 定义常用 UI 组件：`Out.Box(title)`、`Out.Menu(items)`、`Out.Step(n, name)`、`Out.Sep`；
- 逐个迁移 `cmd/model.go（113 处）→ mode.go（161）→ config.go（104）→ settings_db.go（48）→ session.go（30）`；
- **i18n**：同步将 B 类向导全部硬编码中文迁移到 i18n keys（占总量约 1/3）；
- **验收**：各向导功能回归（可用现有 `cmd/settings_mode_dirs_test.go` 等测试 + 手动走一遍向导）；审计脚本 Hardcoded Chinese 计数较基线下降约 1/3。

### P4：外部入口迁移 + 分类开关 + i18n 归零第二部分（D 类）

- `feishu/bridge/hub/subagent` 入口迁移到 Out（或定义独立 ChannelBridge/ChannelSubAgent 渲染策略）；
- 新增「分类开关」配置（`config.LLM.OutputCategories`），`:set` 显示/修改，`--help` 同步；
- **i18n**：D 类入口全部硬编码中文迁移（总量占比小）；同时启动 A 类约 30 处 + C 类约 40 处（含 main.go 约 30 条 `Warning: invalid --xxx`）的迁移；
- **验收**：D 类入口输出可通过开关关闭；`co-shell-feishu-bridge` 独立运行不受影响；审计 Hardcoded Chinese 计数持续下降。

### P4.5：i18n 归零冲刺（100% i18n 达成）

**目标**：清空剩余硬编码中文，达成 100% i18n 硬性目标。

- 按 `docs/output-inventory.md` A-3 硬编码清单逐条迁移（A 类约 30 处：run_stream/loop 的 info/warning/error、token_task 等）；
- 同步补齐 `i18n/keys.go` + `zh.go`/`en.go`（及 loop/system 翻译文件）翻译；
- **验收**：`bin/output_audit.sh` 第 3 项 **Hardcoded Chinese = 0**；新增 i18n key 的覆盖率检查（新增 key 必须同时存在于 zh/en）。

### P5：区域化起步（Region）+ web 模式原型

- `TerminalRenderer` 增加 `Area()` 方法，先落地 `RegionMain` 纯净输出（终端分区域渲染的第一版）；同时确立 `StreamRenderer`（JSON-Lines）为 `stdio` 模式的标准输出端；
- 新增 `SessionIO` 管道抽象（3.6.5），把现有 REPL 主循环改造成"单一管道"驱动；注册 `sessionFactories["stdio"]`（StdinSource → StreamRenderer）与 `sessionFactories["tui"]`（RawKeySource → RegionRenderer，即现有 Enhanced 体验）；
- **web 模式原型**：注册 `sessionFactories["web"]` —— 内嵌 HTTP 服务提供静态页 + WebSocket（WSSource/WSSink），事件以 JSON 分区推送；`--input-mode web` 一键启动；
- 主要 I/O 调用点（工具确认、ESC 中断、向导读取）迁移到管道事件（AskUser 事件对 + AskUserBlocking 同步适配器）；
- **验收**：
  - `--input-mode stdio`：`co-shell "question"` 的 stdout 为合法 JSON 事件流；`--input-mode tui`：终端分区域渲染 + 方向键/ESC 回归；
  - `--input-mode web`：浏览器打开内嵌页面，多分区实时展示 LLM 内容/工具/状态，可发送指令并接收按键级交互；
  - 三种模式跑同一指令，Agent 结果一致（仅 I/O 载体不同）。

---

## 五、迁移指引（开发者操作手册）

### 5.1 新增一条输出（决策树）

```
要输出什么？
├─ LLM 内容/思考 → ChannelLLM，走事件回调（cb）
├─ 工具相关 → ChannelTool / ChannelCommand，走事件回调
├─ 向导/菜单 → ChannelWizard，走 Out（h.io().Out()）
├─ 系统状态（欢迎/帮助/清理）→ ChannelSystem，走 Out
├─ 数据库/同步 → ChannelDB，走 Out
├─ 调试/诊断 → ChannelDebug，走 Out.Debug
└─ 其他 → 评估是否新增 Channel（先讨论，勿擅自加）
每个输出：
1. 文本必须走 i18n.T() / i18n.TF()（新增 key 需同步 zh/en）
2. 交互输入必须走 UserIO.ReadLine() / ReadKey()（P2.5 后逐步改为消费 InputSource）
3. 禁止新增 fmt.Print / fmt.Printf / fmt.Println（用户交互场景）
```

### 5.2 新增一次输入读取（决策树）

```
要等待什么输入？
├─ 整行文本（标准流式/表单）→ InputSource.NextEvent 等待 InputLine
├─ 单键确认（Enter/C/A/G...）→ 等待 InputKey 并校验白名单
├─ 控制字符（方向键/ESC/Tab）→ 等待 InputArrow*/InputEsc 等（响应式专属）
└─ 中断/取消（ESC/Ctrl+C）→ 由统一事件流消费者处理，业务代码不直接读 stdin
注意：
- 禁止新增独立 goroutine 直接 read(os.Stdin, ...)（会造成 stdin 竞争）
- 键盘输入解析（ESC 序列）统一在 RawKeySource，业务层只消费语义化 InputKind
```

### 5.3 迁移现有输出（checklist）

- [ ] 确认该输出属于哪个 Channel/Level
- [ ] 确认该输出是否受开关控制
- [ ] 将 `fmt.X` 替换为 `out.X`（或 `io.Out().X`）
- [ ] 将硬编码中文替换为 i18n key（参考 `docs/output-inventory.md` 硬编码清单）
- [ ] 运行 `bin/output_audit.sh`（P2 起提供）确认无新增不合规输出

---

## 六、版本计划、兼容与设计约束

### 6.1 兼容映射：`--input-mode enhanced` → `tui` 别名（决策 1 方案 A）

现状 `--input-mode` 合法值为 `enhanced`/`stdio`（repl.go `inputMode` switch）。新增 `tui` 后：

| 旧值 | 新值 | 处理 |
|------|------|------|
| `enhanced` | `tui` | **保留为 `tui` 别名**，老配置/脚本无缝迁移，不报错 |
| `stdio` | `stdio` | 不变 |
| （新） | `tui` / `web` | 新增 |

- 配置解析统一走 `parseInputMode(string)`：`enhanced`→`tui`，未知值报错并提示合法值；
- config.json 旧值 `"input_mode": "enhanced"` 自动归一化为 `tui`（加载时归一化，不回写）。

### 6.2 配置三通道（项目规范：CLI / REPL / 配置文件）

| 通道 | 实现 |
|------|------|
| CLI | `--input-mode <stdio\|tui\|web>`（`enhanced` 为 `tui` 别名），`--help` 说明 |
| REPL | `:set input-mode <stdio\|tui\|web>`（向导式：列出当前值 + 枚举选项，参照 `:set` 其他枚举参数实现） |
| config.json | `LLM.InputMode` 字段（默认 `tui`，保持现有默认交互体验；`enhanced` 自动归一化为 `tui`） |

优先级：CLI 参数 > 配置文件（与现有 `show-xx` 等一致）。

### 6.3 P1 回归基线（零行为变化验证手段）

P1 修改 ~70 处 `cb("...")`，需可重复的基线对比：

- **golden-file 测试**：为 `repl.go streamCallback` 与 `main.go executeSingleCommand` 的事件渲染新增 golden 测试（输入同一事件序列，输出快照对比）；
- **实时基线**：P1 合入前录制固定 LLM prompt 的 stdout 快照（`script`/`tee`），合入后同条件重跑比对；
- 验收以「golden 测试全绿 + 快照无差异」为准，而非仅 git diff。

### 6.4 测试计划（项目规范：关键逻辑必须有单元测试）

| 测试 | 位置 | 形态 |
|------|------|------|
| RawKeySource / ESC 序列解析 | `repl/input_test.go`（新增） | table-driven：方向键/ESC/Ctrl+C/Tab/退格/Enter 各序列 |
| RenderCommand 渲染器 | `agent/render_test.go`（新增） | 快照：LineRenderer/StreamRenderer 对同一 RenderCommand 的渲染输出 |
| SessionIO 管道切换 | `repl/session_test.go`（新增） | stdio↔tui 管道配对 + `parseInputMode` 兼容测试 |
| i18n key zh/en 一致性 | `i18n/i18n_keys_test.go`（新增） | 遍历 keys.go 所有 key，断言 zh/en 均有翻译（对应审计第 5 项） |
| 现有功能回归 | 现有 `*_test.go` | `go test ./...` 全绿 |

### 6.5 颜色方案（B1）

| Level | 颜色 | 说明 |
|-------|------|------|
| LevelInfo | 默认 | 不加色 |
| LevelSuccess | 绿 | |
| LevelWarning | 黄 | |
| LevelError | 红 | |
| LevelDebug | 灰 | |

- **ANSI 只出现在 LineRenderer / FullScreenRenderer**；StreamRenderer（stdio JSON-Lines/纯净文本）与 WebRenderer **强制剥离 ANSI**；
- 支持 `NO_COLOR` 环境变量（遵循 https://no-color.org 约定）与 `--no-color` 参数。

### 6.6 web 模式安全边界（B3）

- 原型阶段默认**只绑定 `127.0.0.1`**，无认证；
- 文档明确：生产化（远程访问）前必须补认证（token/回环校验），本次不做。

### 6.7 日志边界（B4）

- **用户可见输出** → `Out` / `RenderCommand` / 事件回调；
- **诊断调试** → `log` 包（文件输出，`./log/`）；
- 禁止混用：不得将用户可见文案写入 log，也不得将诊断信息通过 Out 发给用户（除 `LevelDebug` 受开关控制外）。

### 6.8 风险与依赖

**风险矩阵：**

| 风险 | 等级 | 缓解 |
|------|:---:|------|
| P2.5 stdin 竞争重构（三路径合并） | ★★★ | 独立版本（0.7.1），先补 golden 回归再改 |
| P1 核心文件大 diff（~70 处） | ★★ | golden-file 测试 + stdout 快照基线（6.3） |
| i18n 大规模迁移遗漏 | ★★ | 审计脚本第 3/5 项持续跟踪 |
| web 模式安全 | ★ | 默认只绑 127.0.0.1 |

**阶段依赖 DAG：**

```
P1 ──→ P2 ──→ P2.5 ──→ P5
        │        │
        ├──────→ P3（仅依赖 P2 的 Out/RenderCommand）
        │        │
        └──────→ P4（仅依赖 P2）
P3 与 P4 可并行；P4.5（i18n 归零）依赖 P3+P4 完成。
```

### 6.9 UI 设计语言衔接（项目规范快捷键）

P3 的 `Out.Box/Menu/Step` 必须绑定规范快捷键，禁止各向导自定：

| 快捷键 | 语义 |
|--------|------|
| `[B]` | 退回上一步 |
| `[C]` | 创建 |
| `[E]` | 编辑 |
| `[D]` | 删除 |
| `[Q]` | 退出到命令提示符 |
| `[数字]` | 选择枚举值（选择前列出可选项） |

### 6.10 版本计划（0.7.x 系列，当前基线 0.5.1）

| 版本 | 对应阶段 | 核心交付物 | 验收锚点 |
|------|---------|-----------|---------|
| **0.7.0**（地基） | P1+P2+P3+P4+P4.5 | 事件双枚举 + Out/RenderCommand 抽象 + 渲染单一化；`--input-mode enhanced→tui` 别名；配置三通道；**100% i18n 归零** | `output_audit.sh` 全项 clean（Hardcoded Chinese=0）；老配置无缝运行 |
| **0.7.1**（输入统一） | P2.5 | `InputSource` 统一事件流；ESC 监控改消费者 + **Windows 补齐**；i18n key 覆盖率测试 | 方向键/ESC/Ctrl+C 双平台通过；`--input-mode stdio` 管道行为不变 |
| **0.7.2**（tui v1+web） | P5 | LineRenderer + StreamRenderer(JSON-Lines) + WebRenderer 原型；`stdio/tui/web` 三态齐备 | 三模式同指令结果一致；`--input-mode web` 浏览器分区实况 |
| **0.7.3**（可选分支） | tui v2 | FullScreenRenderer（全屏重绘，原生 ANSI 缓冲，不用 tview） | 独立分支交付，不影响主线稳定性 |

---

## 七、验收标准（全局）

1. **开关完整**：A/B/C/D 四类输出均可通过配置/`:set`/CLI 开关控制；
2. **i18n 100%**：`bin/output_audit.sh` 第 3 项 **Hardcoded Chinese = 0**；新增 i18n key 必须同时有 zh/en 翻译；
3. **通道统一**：用户交互路径无新增 `fmt.Print` 直写（审计脚本兜底）；
4. **渲染单一**：事件渲染逻辑唯一（REPL/main 共用渲染器）；
5. **回归安全**：每个阶段独立合入，`go build ./...` + 相关单测通过。

### i18n 豁免项（明确不做 i18n 的边界）

| 类别 | 说明 |
|------|------|
| LLM 透传内容 | 流式输出、工具返回数据——内容由 LLM/命令产生，非 co-shell 文案 |
| 数据内容 | 命令输出、文件内容、会话导出、日志文件——非 UI 文案 |
| 终端控制序列 | `\033[2J` 等 ESC 转义——非可见文本 |
| 单字节提示 | `[Enter] [C] [A]` 等按键提示可走 i18n，但单字符回显（按键本身）不 i18n |

---

## 八、相关历史文档

- `repl/output-control-inventory.md`：ENHANCEMENT-126（7 开关改造）的历史清单，本文档 P1/P2 是其延续；
- `docs/output-inventory.md`：现状完整摸底（通道统计/A-D 四类逐项/硬编码清单/问题点编号）。