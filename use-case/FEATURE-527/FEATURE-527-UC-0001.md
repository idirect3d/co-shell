# FEATURE-527 测试用例：hub「使用最新版本」co-shell 选项（启动时自动选版）

## 背景

co-shell-hub 的 Agent 配置里，co-shell 可执行程序下拉目前只能从**当次扫描结果**中挑选一个**固定路径**（或选「默认（沿用 hub 配置）」）。用户编译出新版本后，必须逐个 Agent 手工改选，否则一直跑旧版本。

**本次目标**：下拉新增「使用最新版本」选项；选中后，**每次启动该 Agent 时**都重新扫描搜索路径，动态选出最高版本的可执行程序再启动。

**代码定位（现状）**：

| 环节 | 位置 | 现状 |
|---|---|---|
| 候选扫描 | `hub/gateway/detect.go` `DetectCoShells()` | 按 cwd → PATH → hub 同目录扫描，逐候选执行 `--version` 校验（无缓存） |
| 下拉（新建） | `hub/gateway/webui_static.go` `loadDefaults()` + `fillCoShellOptions` 的姊妹逻辑 | 首项 `默认（沿用 hub 配置）`(value=`""`)，其余为候选路径 |
| 下拉（修改） | `hub/gateway/webui_static.go` `fillCoShellOptions()` | 同上；已配置但未探测到的路径追加「（当前值）」 |
| 启动 | `hub/gateway/manager.go` `Manager.Start()` | `spec.CoShell == ""` → 用 `m.coShell`（hub 自身默认）；否则用该固定路径 |
| 列表版本 | `hub/gateway/webui.go` `handleListAgents()` | 对 managed agent 一律显示 `coShellVersion(m.CoShellPath())` |

## 需求（用户确认）

1. 新建 Agent 与 Agent 设置（修改）两个表单的 co-shell 下拉，都新增「使用最新版本」选项。
2. 选中该选项后，**每次启动**该 Agent 都重新搜索一遍，定位最高版本可执行程序后运行（不缓存、不固定路径）。
3. 选版规则：**hub 同目录优先**——hub 同目录存在可用 co-shell 时取其中版本最大者；否则在「当前目录 + PATH 各目录」的全部候选中取**全局最大版本**。
4. 所有搜索路径都搜不到可用 co-shell 时：启动失败并**如实报错**（不静默回退）。
5. 版本 `0.61.0`（FEATURE，minor+1）；任务号 `FEATURE-527`；分支 `FEATURE-527`。

## 实现契约

| 项 | 约定 |
|---|---|
| 标记值 | `latest`（后端常量 `CoShellLatest`），持久化在 `AgentSpec.CoShell` |
| 下拉文案 | `使用最新版本（启动时自动搜索）` |
| 下拉位置 | 新建表单：候选之前（首位，即使未扫到候选也显示）；修改表单：紧跟 `默认（沿用 hub 配置）` |
| 解析函数 | `resolveLatestCoShell(hubDir, cwd string, pathDirs []string) (string, error)`（可注入路径，便于单测）+ 环境包装 `ResolveLatestCoShell()` |
| 选版规则 | ① hubDir 有可用候选 → 取版本最大者；② 否则 cwd + pathDirs 全部候选取全局最大；③ 都没有 → error |
| 版本比较 | 按 `major.minor.patch` **数值**比较（`0.9.0 < 0.10.0`），非字符串比较 |
| 启动接入 | `Manager.Start()`：`spec.CoShell == CoShellLatest` → 实时 `ResolveLatestCoShell()`；失败 → 返回错误（agent 不启动） |
| 每次启动重扫 | 解析结果不缓存；运行中重复 Start 仍为 no-op（需停后再启才重新解析） |
| 列表展示 | `co_shell` 原样回传 `latest`；版本列对 latest agent 显示**当前解析到的**版本，解析失败留空 |
| 修改表单版本提示 | 选中 latest 时直接显示「启动时自动搜索最新版本」，不请求 `/api/agent-version?path=latest` |
| 涉及文件 | `hub/gateway/detect.go`、`hub/gateway/manager.go`、`hub/gateway/webui.go`、`hub/gateway/webui_static.go`、`hub/gateway/detect_test.go`、`hub/gateway/manager_test.go`；版本号 `main.go`、`cmd/co-shell-hub/main.go` |
| 非目标 | 不做候选缓存；不新增 API 端点；不改 legacy hub（`hub/hub.go`）；不改 co-shell 主程序行为 |

## 验收前置（独立实例，不影响生产）

```bash
rm -rf /tmp/feat527 && mkdir -p /tmp/feat527/hub /tmp/feat527/cwd /tmp/feat527/p1 /tmp/feat527/p2
cd /Users/direct3d/github/co-shell
go build -o /tmp/feat527/hub/co-shell-hub ./cmd/co-shell-hub

# 造 4 个「假 co-shell」脚本（--version 输出符合 banner 正则），代表不同版本
mk(){ printf '#!/bin/sh\necho "co-shell v%s [BUILD-%s]"\n' "$2" "$3" > "$1"; chmod +x "$1"; }
mk /tmp/feat527/hub/co-shell-0.50.0.darwin.arm64 0.50.0 900      # hub 同目录（低版本）
mk /tmp/feat527/cwd/co-shell-0.58.0.darwin.arm64 0.58.0 901      # CWD
mk /tmp/feat527/p1/co-shell-0.61.0.darwin.arm64  0.61.0 902      # PATH 第 1 段（最高）
mk /tmp/feat527/p2/co-shell-0.9.0.darwin.arm64   0.9.0  903      # PATH 第 2 段（字符串比较会误判为最大）

# 启动 hub（CWD = /tmp/feat527/cwd，PATH 前置 p1:p2）
cd /tmp/feat527/cwd && PATH=/tmp/feat527/p1:/tmp/feat527/p2:$PATH \
  /tmp/feat527/hub/co-shell-hub --serve --port 12931 &
```

> 打开 `http://127.0.0.1:12931` 操作；下文「新建表单」/「修改表单」指令分别指 Enter Agent 管理后的新建面板与点卡片箭头进入的修改面板。

## 用例

### UC-01 新建表单出现「使用最新版本」选项（含无候选场景）

1. 打开新建 Agent 面板，展开 co-shell 下拉。
2. 断言：存在选项文本 `使用最新版本（启动时自动搜索）`，值为 `latest`，且位于所有候选之前。
3. 把 hub 同目录 / CWD / PATH 中的假 co-shell 全部临时改名，重新进入新建面板。
4. 断言：下拉**仍**包含该选项（即使当前扫不到任何候选）。

### UC-02 修改表单出现该选项并正确回显

1. 选中某 managed Agent，进入修改面板，展开 co-shell 下拉。
2. 断言：`默认（沿用 hub 配置）` 之后即为 `使用最新版本（启动时自动搜索）`。
3. 选它 → 保存 → 重新进入该 Agent 修改面板。
4. 断言：下拉仍选中该项（不是「默认」，也不是某个具体路径）。

### UC-03 hub 同目录优先于其它路径（核心规则）

1. 前置：hub 同目录有 `0.50.0`，PATH 的 p1 有 `0.61.0`，CWD 有 `0.58.0`。
2. 把某 Agent 的 co-shell 设为「使用最新版本」，保存后启动它。
3. 断言：选中的是 `/tmp/feat527/hub/co-shell-0.50.0.darwin.arm64`（hub 同目录），**不是** p1 的 0.61.0。
4. 校验方式：hub 日志中该 agent 启动可行；或前端 Agent 列表该行版本显示 `v0.50.0`。

### UC-04 hub 同目录多候选取最大版本

1. 在同一 hub 同目录再放入 `co-shell-0.61.0.darwin.arm64`（与 0.50.0 共存）。
2. 停止该 Agent 后重新启动。
3. 断言：本次使用 `co-shell-0.61.0.darwin.arm64`（同目录内取最大）。

### UC-05 hub 同目录无候选时取全局最大

1. 移除 hub 同目录全部假 co-shell（只剩 `co-shell-hub` 自身，其 `--version` 不匹配 banner，应被过滤）。
2. 停止并重新启动该 Agent。
3. 断言：使用 p1 的 `co-shell-0.61.0.darwin.arm64`（cwd 0.58.0 与 p2 0.9.0 均更低）。

### UC-06 版本比较按数值而非字符串

1. 保持 p2 的 `0.9.0`，在 p2 内再放 `co-shell-0.10.0.darwin.arm64`；清空 hub 同目录与 cwd 候选。
2. 重启该 Agent。
3. 断言：选中 `0.10.0`（字符串比较会错误选 `0.9.0`）。

### UC-07 无效候选被过滤

1. 在 p1 放入同名前缀但非 co-shell 的可执行文件 `co-shell-decoy`（输出 `hi`）与不可执行文件 `co-shell-0.99.0.darwin.arm64`（无 x 位）。
2. 重启该 Agent。
3. 断言：二者均不被选中；仍选真实最高版本（0.61.0）。

### UC-08 每次启动都重新搜索（不缓存）

1. 设 p1 为最高（0.61.0），启动 Agent → 记为 A。
2. 停止 Agent；在 p1 放入 `co-shell-0.70.0.darwin.arm64`。
3. 重新启动 Agent。
4. 断言：本次使用 0.70.0（说明启动时重新扫描，而非沿用首次结果）。
5. 反向校验：把 0.70.0 删除后**不重启**，仅再次点击启动（运行中为 no-op）→ 运行进程不变（符合「启动时」语义）。

### UC-09 全部路径搜不到 → 启动失败并如实报错

1. 把三个目录的假 co-shell 全部改名，且确保 PATH 中无真实 co-shell。
2. 启动该 Agent。
3. 断言：Agent **未启动**（开关仍为关）；前端弹出/显示错误提示，内容明确指出未找到可用 co-shell（不静默回退到 hub 默认，也不假装成功）。

### UC-10 修改表单的版本提示

1. 修改表单中把 co-shell 选为「使用最新版本」。
2. 断言：版本提示区显示「启动时自动搜索最新版本」之类的说明，且**不**发起 `/api/agent-version?kind=local&path=latest` 请求（浏览器 Network 面板确认无该请求）。
3. 切换到某个具体路径候选 → 提示恢复为 `co-shell vX.Y.Z [BUILD-N]`（原行为不回归）。

### UC-11 Agent 列表的版本显示

1. 设 hub 同目录存在可用 `0.61.0`，某 Agent 使用「使用最新版本」。
2. 刷新 Agent 列表。
3. 断言：该行版本显示解析出的 `v0.61.0`（而非空白）；把候选全部移除并刷新 → 版本列留空但 Agent 配置不丢失。
4. 断言：重新进入修改面板，下拉仍为「使用最新版本」。

### UC-12 单元测试（resolveLatestCoShell 注入目录）

`cd hub/gateway && go test ./... -run 'TestResolveLatestCoShell|TestCompareVersion' -v` 全部 PASS，覆盖：
- 同目录优先（同目录低版本 vs 其它路径高版本 → 同目录）
- 同目录多版本取最大
- 同目录无候选 → 全局最大（跨 cwd/PATH）
- 数值比较（0.9.0 vs 0.10.0）
- 无效候选（非 banner 输出、无执行位）被过滤
- 全路径无候选 → 返回 error

### UC-13 回归

1. `cd /Users/direct3d/github/co-shell && go build ./... && go vet ./...` 一次执行通过。
2. `go test ./hub/gateway/ -v` 全绿（含既有 FIX-521/FIX-525 用例）。
3. 既有行为不回归：选「默认（沿用 hub 配置）」→ 仍用 hub 自身默认可执行程序启动；选具体路径候选 → 仍用该固定路径。
