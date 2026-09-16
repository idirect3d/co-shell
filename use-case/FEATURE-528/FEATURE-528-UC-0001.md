# FEATURE-528 测试用例：CLI `--yolo` 启动开关 + hub Agent YOLO 滑动开关

## 背景

co-shell 已有 YOLO（You Only Live Once）运行时能力：`agent.SetYOLO/IsYOLO`（FEATURE-439）、REPL `:YOLO` 命令、co-shell Web UI 状态栏的 YOLO 开关。但：

1. **命令行启动参数里没有 YOLO 开关**——`main.go` 只有 `--confirm-tool on|off`（改 `LLM.ToolModes["default"]`），无法在启动时直接打开 YOLO 主开关。
2. **hub 配置 Agent 时没有 YOLO 选项**——`hub/gateway/AgentSpec` 有 `Workspace/Port/ConfigPath/ExtraArgs`，Web UI 有「共享配置」这类滑动开关，但没有 YOLO；用户只能手工往「补充运行参数」里填字符串，既不显式也无风险提示。

**本次目标**：① co-shell 新增 `--yolo` 启动开关；② hub 新建/修改 Agent 表单显式提供 YOLO 模式滑动开关（默认关闭、开启提示风险、关闭说明含义），开启后启动该 Agent 时追加 `--yolo`。

## 代码定位（现状）

| 环节 | 位置 | 现状 |
|---|---|---|
| CLI 参数定义 | `main.go` `cliFlags` + `parseFlags()` | 无 YOLO 相关标志；`--confirm-tool` 用 `on/off` 字符串 |
| CLI 帮助 | `usage.go` `buildUsage()` + `i18n/{zh,en}.go` `KeyCLIHelp*` | 无 YOLO 条目 |
| YOLO 运行时 | `agent/agent.go` `SetYOLO/IsYOLO`、`agent/tools.go` `needsConfirm := mode == "confirm" && !a.yoloMode` | 仅由 REPL `:YOLO` 与 co-shell Web UI 的 `yolo_set` 修改 |
| hub 持久化 | `hub/gateway/manager.go` `AgentSpec`（JSON 文件 `hub-agents.json`） | 无 YOLO 字段 |
| hub 启动参数 | `hub/gateway/manager.go` `Manager.Start()` | 组装 `--serve --port ... --bind ... -w ...` + `ExtraArgs` |
| hub API | `hub/gateway/webui.go` `handleCreateAgent` / `handleUpdateAgent` | 请求体含 `extra_args` 等，无 `yolo` |
| hub UI | `hub/gateway/webui_static.go` | 新建表单 `m-*`、详情/编辑表单 `d-*`，已有 `.switch/.slider` 滑动开关样式 |

## 需求（用户确认）

1. 命令行新增 YOLO 模式开关（若无）：`--yolo` 为**纯布尔开关**，出现即开启，默认关闭。
2. hub 配置 Agent 时显式增加 YOLO 模式选项（**滑动开关**），默认关闭，需用户明确打开。
3. 开启时在开关下方显示**简单风险信息**；关闭时显示 **YOLO 的含义说明**。
4. 版本 `0.62.0`（FEATURE，minor+1）；任务号 `FEATURE-528`；分支 `FEATURE-528`。

## 实现契约

| 项 | 约定 |
|---|---|
| CLI 标志 | `--yolo`（`flag.BoolVar`，默认 `false`），**出现即开启**；不提供 `--yolo on|off` |
| CLI 生效方式 | 启动时对已构建的 agent 调用 `SetYOLO(true)`；**不写回 config.json**（YOLO 为运行时状态，沿用 FEATURE-439 的"不持久化"约定） |
| CLI 帮助 | `usage.go` 增加一行 `KeyCLIHelpYOLO`，zh/en 双语 |
| hub 字段 | `AgentSpec.YOLO bool \`json:"yolo,omitempty"\``，默认 `false`（缺省即关闭，向后兼容旧 `hub-agents.json`） |
| hub 启动参数 | `spec.YOLO == true` → 在系统参数之后、`ExtraArgs` 之前追加 `--yolo`；为 `false` 时不追加任何参数 |
| hub API | create/patch 请求体新增 `yolo`；patch 用 `*bool`，未传（`nil`）表示不改动（兼容旧前端） |
| hub UI 控件 | 复用现有 `.switch` 滑动开关；新建表单 id `m-yolo`，编辑表单 id `d-yolo`；默认关闭 |
| hub UI 文案 | 关闭态显示含义说明（YOLO = You Only Live Once，开启后所有工具调用自动批准、无需逐个确认）；开启态显示风险提示（工具调用将不再确认，可能执行高风险命令/改动文件，请仅在受控环境使用） |
| hub 文案双语 | 跟随现有前端 i18n 机制（`i18nT(key, fallback)` + 语言表），zh/en 均齐备 |
| 涉及文件 | `main.go`、`usage.go`、`i18n/keys.go`、`i18n/zh.go`、`i18n/en.go`、`hub/gateway/manager.go`、`hub/gateway/webui.go`、`hub/gateway/webui_static.go`、`cmd/co-shell-hub/main.go`（版本/BUILD）、`ROADMAP.md` |
| 非目标 | 不改 legacy hub（`hub/hub.go`）；不改 REPL `:YOLO` 与 co-shell Web UI 既有 YOLO 开关；不把 YOLO 持久化进 config.json；不改 `--confirm-tool` 语义 |

## 验收前置（独立实例，不影响生产）

```bash
# A. CLI 侧
cd /Users/direct3d/github/co-shell
go build -o /tmp/feat528/co-shell ./            # 或直接使用 work/ 下 0.62.0 产物
mkdir -p /tmp/feat528/ws && cd /tmp/feat528/ws

# B. hub 侧（隔离实例，独立端口）
rm -rf /tmp/feat528/hub && mkdir -p /tmp/feat528/hub
cd /Users/direct3d/github/co-shell
go build -o /tmp/feat528/hub/co-shell-hub ./cmd/co-shell-hub
cd /tmp/feat528/hub && /tmp/feat528/hub/co-shell-hub --serve --port 12941 &
```

> hub 页面：`http://127.0.0.1:12941`；下文「新建表单」= Agent 管理中的新建面板，「编辑表单」= 点 Agent 卡片箭头进入的修改面板。

## 用例

### UC-01 CLI 帮助列出 `--yolo`（默认关闭语义可读）

1. 执行 `/tmp/feat528/co-shell --help`。
2. 断言：输出中存在 `--yolo`，描述说明其为 YOLO 模式开关（开启后工具调用自动批准）、默认关闭。
3. 切换 `--lang en` 再执行一次，断言出现对应英文描述。
4. 断言：帮助文本中**不出现** `--yolo on|off`（本次为纯布尔开关）。

### UC-02 缺省（不带 `--yolo`）保持关闭：工具调用仍需确认

1. 在 `/tmp/feat528/ws` 下执行 `co-shell --prompt "执行 echo hi"`（不带 `--yolo`）。
2. 断言：出现工具确认交互（要求确认 execute_command），未自动执行。
3. 断言：不产生副作用（无自动批准）。

### UC-03 带 `--yolo` 出现即开启：工具调用自动批准

1. 在 `/tmp/feat528/ws` 下执行 `co-shell --yolo --prompt "执行 echo hi"`。
2. 断言：**未出现任何工具确认交互**，命令直接执行并返回结果。
3. 断言：`--yolo` 后面不跟参数（纯布尔）时行为与 `--yolo=true` 一致，均开启。
4. 断言：进程退出后重新以不带 `--yolo` 启动，确认恢复为"需确认"（YOLO 不持久化）。

### UC-04 新建表单：YOLO 开关默认关闭且显示含义说明

1. 打开 hub 新建 Agent 表单。
2. 断言：存在「YOLO 模式」滑动开关（id `m-yolo`），初始为**关闭**状态。
3. 断言：开关下方显示 YOLO **含义说明**（You Only Live Once / 开启后所有工具调用自动批准、无需逐个确认），且**不显示**风险提示。
4. 不打开开关直接创建 Agent → 断言 API 返回与列表回显 `yolo` 为 false/缺省。

### UC-05 新建表单：打开开关显示风险提示并持久化

1. 在新建表单中打开 YOLO 开关。
2. 断言：开关下方文案切换为**风险提示**（工具调用不再确认、可能执行高风险操作、请谨慎使用），含义说明隐藏。
3. 创建 Agent → 断言 `hub-agents.json` 中该 Agent 记录 `yolo: true`；页面/接口回显开关为打开。

### UC-06 编辑表单：回显与双向切换

1. 进入 UC-05 创建 Agent 的编辑表单。
2. 断言：`d-yolo` 开关为**打开**状态，并显示风险提示。
3. 关闭该开关并保存 → 重新进入编辑表单，断言开关为关闭且显示含义说明；`hub-agents.json` 中 `yolo` 为 false/缺省。
4. 再次打开并保存 → 断言回显打开（双向切换均可保存生效）。

### UC-07 启动 Agent 时追加 `--yolo`（开启态）

1. 对 UC-05 的 Agent 点击启动。
2. 断言：该 co-shell 子进程命令行**包含 `--yolo`**（`ps -o args= -p <pid>` 或以 hub 日志中记录的启动命令为准）。
3. 断言：启动后该 Agent 内工具调用自动批准（与 UC-03 行为一致）。

### UC-08 启动 Agent 不追加 `--yolo`（关闭态，回归）

1. 对 YOLO 关闭的 Agent 点击启动。
2. 断言：子进程命令行**不含 `--yolo`**（不残留该参数）。
3. 断言：原有 `--serve --port --bind -w` 与「补充运行参数」仍完整保留，顺序未被破坏。

### UC-09 API 兼容：未传 `yolo` 的修改不误清空

1. 对 YOLO 已开启的 Agent，用**不含 `yolo` 字段**的 PATCH 请求修改备注（模拟旧前端）。
2. 断言：该 Agent 的 `yolo` 仍为 true（`*bool` 为 nil 时不改动），未被误重置为 false。

### UC-10 中英文文案齐备

1. 切换到英文界面，重开新建/编辑表单。
2. 断言：开关标题、关闭态含义说明、开启态风险提示均有英文文案且非空（不是中文兜底、不是空白）。
3. 切回中文，断言三处文案为中文且语义与英文一致。
