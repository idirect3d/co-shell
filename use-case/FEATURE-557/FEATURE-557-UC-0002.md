# FEATURE-557 验收测试用例（UC-0002）：公告板在线状态改事件驱动 + hub 三态呈现

> 需求（用户提出）：
> 1. 现在的 60s 心跳意义不大（roster 只增不删、注册过就长期显示在线，除非删 agent），希望**改为事件/消息触发的状态变更**：
>    - ① agent 启动且 `board-enabled` 启用 → 发注册消息 → 置【在线】；
>    - ② agent **实时**变更 `board-enabled` 状态 → 发注册/注销消息 → 置【在线】/【未注册】；
>    - ③ hub 关停 agent 时，若【在线】则置【离线】。
> 2. hub Agents 清单中该 agent 呈现**三态**：【未注册】不显示图标 / 【在线】亮色会话气泡 / 【离线】灰色会话气泡。
>
> 交互决策（经 ask_user 两轮确认，共 9 项）：
> - ① **未注册 = 不删除条目**，仅置状态位；roster 条目与该 agent 的历史会话/消息一律保留（否则 agent 将无法访问）；
> - ② **完全去掉 60s 心跳**，仅靠 WS 断开判定离线；已在线者 socket 断开即置【离线】。**已知局限**：agent 进程僵死而 socket 未断时会持续显示【在线】（用户已明确接受）；
> - ③ hub 主动 stop managed agent → **立即**置【离线】；hub 重启 → **仅已注册的置【离线】**，未注册的保持【未注册】；
> - ④ UI 三态：在线=亮色气泡 / 离线=灰色气泡（`--fg-faint`）/ 未注册=不显示；**三态均带 tooltip**；
> - ⑤ 保留 `board_identity` 推送（IDENTITY.md 4s 轮询 + 设置界面直推），与在线状态无关；
> - ⑥ 沿用 **FEATURE-557 分支与 v0.76.0**，仅递增 BUILD（不新建编号、不改版本号）；
> - ⑦ **链路每次建立都无条件校正**一次状态（开关开→register，关→unregister），使 hub 重启/重连后状态自愈；
> - ⑧ 升级后旧 roster 数据（无 `registered` 字段）视为【未注册】，agent 重连后自然恢复（宁可不显示也不错显示）；
> - ⑨ 会话运行中 off→on 时，把公告板运行时（聚合器/连接钩子/身份推送）**一并拉起**，打开即完整可用。
>
> 三态模型（hub 数据 → UI）：
>
> | 状态 | hub 数据 | UI |
> |------|----------|----|
> | 未注册 | `Registered=false` | 不显示图标 |
> | 在线 | `Registered=true && Online=true` | 亮色气泡 |
> | 离线 | `Registered=true && Online=false` | 灰色气泡（`--fg-faint`） |
>
> 状态转移：
> `board_register` 帧 →【在线】；`board_unregister` 帧（新增）→【未注册】；WS 断开（`dropAgentLink`）→ 仅在已注册时【离线】；hub 主动 stop agent → 立即【离线】；hub 重启 → 已注册的置【离线】，未注册的保持【未注册】。
>
> 改动范围（预期实现）：`hub/gateway/board.go`（`AgentInfo.Registered` + `SetUnregistered` + 三态门控）、`hub/gateway/proxy.go`（`board_unregister` 帧）、`hub/gateway/webui.go`（stop agent 置离线 + `board_state`）、`hub/gateway/webui_static.go`（三态渲染）、`web/session_board.go`（去心跳 + `announceBoardState` + `watchBoardSwitch`）。

## 用例清单

| 编号 | 名称 | 类型 |
|------|------|------|
| A-001 | `Register` 置 `registered=true` 且 `online=true` | 单测 |
| A-002 | 重复 `Register` 幂等（不增 rosterRev） | 单测 |
| A-003 | `SetUnregistered` 置未注册（`registered=false`、`online=false`）且**条目保留** | 单测 |
| A-004 | `SetUnregistered` **不清理**该 agent 的历史会话与消息 | 单测 |
| A-005 | `SetUnregistered` 对未注册条目幂等（no-op，不增 rosterRev） | 单测 |
| A-006 | `SetOffline` 对未注册条目 no-op（【未注册】不被改写成【离线】） | 单测 |
| A-007 | `MarkOnline` 对未注册条目 no-op（链路重建不点亮） | 单测 |
| A-008 | 任意 board 帧（`touchLocked`）不把未注册条目翻在线 | 单测 |
| A-009 | 未注册条目再次 `Register` → 回到【在线】 | 单测 |
| A-010 | `NewBoard` 恢复旧数据（无 `registered` 字段）→ `registered=false`、`online=false` | 单测 |
| A-011 | `NewBoard` 恢复新数据（`registered=true`）→ `registered=true`、`online=false` | 单测 |
| A-012 | `GET /api/agents` 的 `board_state` 三态取值正确，`board_enabled` 兼容 | 单测 |
| A-013 | `handleStopAgent` 后该 agent 立即变为 offline | 单测 |
| A-014 | 静态资源三态渲染（亮色/灰色/不渲染）与 tooltip、灰色 CSS 存在 | 单测 |
| A-015 | agent 侧 `announceBoardState` 帧选择正确（开→`board_register` 带 name/duty；关→`board_unregister`） | 单测 |
| A-016 | 源码中 60s 心跳已彻底移除（无 `boardHeartbeat` / `boardHeartbeatInterval` 残留） | 单测（静态断言） |
| B-001 | 启动即注册：`board-enabled` 开的 agent 在 hub 清单显示**亮色气泡** | 运行时 |
| B-002 | `board-enabled` 关的 agent **不显示图标** | 运行时 |
| B-003 | 运行中关闭 `board-enabled` → 数秒内图标**消失**（转未注册） | 运行时 |
| B-004 | 运行中打开 `board-enabled` → 数秒内变**亮色气泡**，且公告板功能立即可用（无需重启会话） | 运行时 |
| B-005 | agent 进程退出（WS 断开）→ **灰色气泡**（离线） | 运行时 |
| B-006 | 离线 agent 重新启动/重连 → 自动恢复**亮色**（在线） | 运行时 |
| B-007 | hub 主动 stop managed agent → **立即**灰色（离线），不等 socket 超时 | 运行时 |
| B-008 | hub 重启 → 已注册的 agent 显示**灰色**（离线），未注册的仍**不显示图标** | 运行时 |
| B-009 | 未注册 agent 的条目仍可访问（Chat 面板仍在、历史消息可读，条目未被删除） | 运行时 |
| B-010 | 三态 tooltip 文案正确（未注册/在线/离线语义可区分） | 运行时 |
| B-011 | 心跳确实消失：连续观察 2 分钟，无周期性 `board_register` 帧 | 运行时 |
| B-012 | 升级迁移：用旧版 hub 数据启动新 hub → 图标不显示（未注册），agent 重连后恢复 | 运行时（回归） |
| B-013 | 版本号与 BUILD 同步（CLI/Web 显示 `0.76.0 [BUILD-1128]`） | 运行时（回归） |

---

## A 组：单元测试（Go）

> 归属：hub 侧新增于 `hub/gateway/`（沿用 `feature557_test.go` 或新增同目录 `feature557_state_test.go`）；agent 侧新增于 `web/`。

### A-001 `Register` 置 `registered=true` 且 `online=true`

- 前置：空 roster。
- 步骤：`b.Register("a1", "n1", "d1")`。
- 预期：`b.Agent("a1")` 返回 `registered == true`、`online == true`、`last_seen > 0`。

### A-002 重复 `Register` 幂等（不增 rosterRev）

- 前置：`a1` 已注册且在线。
- 步骤：记录 `RosterRev()` → 再次 `Register("a1", "n1", "d1")`。
- 预期：`RosterRev()` 不变（重复注册不制造 roster 变更）。

### A-003 `SetUnregistered` 置未注册且**条目保留**

- 前置：`a1` 已注册且在线。
- 步骤：`b.SetUnregistered("a1")`。
- 预期：
  1. `Agent("a1")` 仍返回 `ok == true`（**条目未被删除**）；
  2. `registered == false` 且 `online == false`；
  3. 返回值 `changed == true`，`RosterRev()` 递增。

### A-004 `SetUnregistered` **不清理**历史会话与消息

- 前置：`a1` 与 `b1` 之间已有一轮 `Send` 往返（存在会话与消息记录）。
- 步骤：`SetUnregistered("a1")` → 读取会话列表与历史消息。
- 预期：该会话仍在、消息条数与内容不变（注销只动状态位，不动数据）。

### A-005 `SetUnregistered` 对未注册条目幂等（no-op，不增 rosterRev）

- 前置：`a1` 已是未注册态。
- 步骤：记录 `RosterRev()` → 再次 `SetUnregistered("a1")`。
- 预期：`changed == false`，`RosterRev()` 不变。

### A-006 `SetOffline` 对未注册条目 no-op

- 前置：`a1` 为未注册态（`registered=false`）。
- 步骤：`b.SetOffline("a1")`。
- 预期：`changed == false`；`Agent("a1")` 仍是 `registered == false && online == false`（**不会被改写成【离线】**——这是三态不被退化的关键用例）。

### A-007 `MarkOnline` 对未注册条目 no-op

- 前置：`a1` 为未注册态。
- 步骤：`b.MarkOnline("a1")`（模拟链路建立时 `installAgentLink` 的行为）。
- 预期：`changed == false`；`online` 仍为 `false`（未注册 agent 一连上不会被点亮）。

### A-008 任意 board 帧（`touchLocked`）不把未注册条目翻在线

- 前置：`a1` 为未注册态。
- 步骤：投递一个 board 帧（等价于 `touchLocked("a1")` 的调用路径）。
- 预期：`online` 仍为 `false`。

### A-009 未注册条目再次 `Register` → 回到【在线】

- 前置：`a1` 已被 `SetUnregistered`。
- 步骤：`b.Register("a1", "n1", "d1")`。
- 预期：`registered == true && online == true`，`changed == true`（开关重新打开的路径）。

### A-010 `NewBoard` 恢复旧数据（无 `registered` 字段）→ 未注册

- 前置：BoardStore 中存在旧格式条目（JSON 无 `registered` 字段，仅有 `online:true`）。
- 步骤：`NewBoard(store)` 后读 roster。
- 预期：该条目 `registered == false` 且 `online == false`（旧数据按未注册处理）。

### A-011 `NewBoard` 恢复新数据（`registered=true`）→ 已注册但离线

- 前置：BoardStore 中存在新格式条目 `{"registered":true,"online":true}`。
- 步骤：`NewBoard(store)` 后读 roster。
- 预期：`registered == true` 且 `online == false`（hub 重启语义：已注册的全部置离线，未注册的保持未注册）。

### A-012 `GET /api/agents` 的 `board_state` 三态取值正确，`board_enabled` 兼容

- 前置：registry 中有三个 agent：`a1` 已注册且在线、`a2` 已注册但离线（或 `SetUnregistered` 后未注册）、`a3` 从未注册。
- 步骤：请求 `GET /api/agents`。
- 预期：
  1. `board_state` 分别取 `"online"` / `"offline"` / `"unregistered"`；
  2. `board_enabled` 兼容保留：`board_state != "unregistered"` 时为 `true`，否则 `false`；
  3. `WebUI` 未挂载公告板（`w.board == nil`）时全部为 `"unregistered"` 且不 panic。

### A-013 `handleStopAgent` 后该 agent 立即变为 offline

- 前置：managed agent `a1` 已注册且在线。
- 步骤：调用停用路径（`handleStopAgent` 等价的内部置下线调用）。
- 预期：`Agent("a1").online == false` 且 `registered == true`（立即离线，不等 socket 断开）；roster 已广播。

### A-014 静态资源三态渲染与 tooltip、灰色 CSS 存在

- 步骤：检查内嵌页面常量 `webIndexHTML`。
- 预期：
  1. `renderList()` 中 `board_state == "online"` 时渲染气泡图标；`"offline"` 时渲染带灰色类（如 `.bmark.off`）的同一图标；`"unregistered"` 时**不渲染**任何图标；
  2. 三种状态均有 `title`（tooltip）文案；
  3. CSS 存在灰色态定义（使用 `--fg-faint` 变量）；
  4. 图标 symbol `i-board` 几何未被改变（仍与 co-shell `i-sessions` 一致）。

### A-015 agent 侧 `announceBoardState` 帧选择正确

- 前置：可注入的 fake server（记录 `sendRaw` 载荷）。
- 步骤：① 开关开时调用 `announceBoardState()`；② 开关关时调用。
- 预期：
  1. 开关开 → 发送帧 `{"type":"board_register","name":...,"duty":...}`；
  2. 开关关 → 发送帧 `{"type":"board_unregister"}`（不含 name/duty）；
  3. 链路未就绪时按既有重试策略重试，不 panic。

### A-016 源码中 60s 心跳已彻底移除

- 步骤：静态断言 `web/session_board.go`。
- 预期：
  1. **不含** `boardHeartbeat` / `boardHeartbeatInterval` / `60 * time.Second` 相关心跳定义；
  2. `initBoard()` 中不再有基于 `BoardEnabled()` 的早退（运行时无条件建立）；
  3. 保留 `watchBoardIdentity`（`board_identity` 推送不删除）。

---

## B 组：运行时验收（浏览器实测 + 日志）

> 通用前置：hub 以 `--serve` 启动；浏览器打开 hub UI；至少 1 个 managed agent（co-shell serve 模式）。observation 时长统一为「不超过 15 秒」（开关变更走 2s 轮询）。

### B-001 启动即注册：开关开的 agent 显示亮色气泡

- 前置：agent 的 `board-enabled=on`，hub 已连接该 agent。
- 步骤：打开 Agents 抽屉，观察该 agent 卡片名称右侧。
- 预期：显示**亮色**聊天气泡图标；tooltip 为「已启用公告板·在线」类语义。

### B-002 开关关的 agent 不显示图标

- 前置：agent 的 `board-enabled=off`。
- 步骤：观察其卡片。
- 预期：名称右侧**没有任何图标**（既无亮色也无灰色）；其余卡片元素（状态灯/开关/箭头）不受影响。

### B-003 运行中关闭 `board-enabled` → 图标消失

- 前置：B-001 状态（亮色气泡）。
- 步骤：在该 agent 内执行 `:config` 关闭 `board-enabled`（或设置界面切换）。
- 预期：**15 秒内**气泡图标消失（转为【未注册】）；hub 日志出现注销相关记录；无重复刷帧。

### B-004 运行中打开 `board-enabled` → 变亮色且功能立即可用

- 前置：B-002 状态（无图标），且该会话此前开关为 off。
- 步骤：在该 agent 内打开 `board-enabled`；随后让该 agent 发一条公告板消息（或观察其公告板工具可用性）。
- 预期：
  1. **15 秒内**出现**亮色**气泡（无需重启 agent 会话）；
  2. 公告板聚合器与工具**立即可用**（消息能正常发送/接收，不出现「聚合器未初始化」类错误）。

### B-005 agent 进程退出（WS 断开）→ 灰色气泡

- 前置：B-001 状态（亮色气泡）。
- 步骤：正常退出 / kill 该 agent 进程。
- 预期：图标变为**灰色**（【离线】），条目仍留在清单中（未被删除）；hub 日志出现 `went offline` 类记录。

### B-006 离线 agent 重新启动/重连 → 恢复亮色

- 前置：B-005 状态（灰色气泡）。
- 步骤：重新启动该 agent（`board-enabled=on`）并等待 hub 连接。
- 预期：图标恢复**亮色**（在线）。

### B-007 hub 主动 stop managed agent → 立即灰色

- 前置：managed agent 显示亮色气泡。
- 步骤：在 hub UI 中关闭该 agent 的开关（`POST /api/agents/{id}/stop`）。
- 预期：图标**立即**（不等 socket 超时）变为**灰色**；再次启动该 agent 后恢复亮色。

### B-008 hub 重启 → 已注册的灰色、未注册的不显示

- 前置：agent A 开关 on（亮色）、agent B 开关 off（无图标）。
- 步骤：重启 hub（保持两 agent 进程不变）。
- 预期：
  1. hub 起来后，A 显示**灰色**（【离线】）——已注册项置离线；
  2. B 仍**不显示图标**（未被错误置为【离线】）；
  3. 待 hub 与 A 重新建立链路后，A 自动恢复**亮色**（自愈，无需人工干预）。

### B-009 未注册 agent 的条目仍可访问

- 前置：agent A 已注册过、且曾在 hub 抽屉 Chat 面板产生过消息；随后关闭其 `board-enabled`。
- 步骤：在 hub 抽屉 Chat 面板查找该 agent 的历史会话与消息。
- 预期：该 agent **仍出现在 Chat 面板**、历史消息仍可读（条目与消息未被删除）。

### B-010 三态 tooltip 文案正确

- 步骤：依次悬停三态图标（若某态无图标则悬停其离线/在线态）。
- 预期：
  1. 在线态 tooltip 表明「已启用公告板·在线」；
  2. 离线态 tooltip 表明「已启用公告板·离线」；
  3. 未注册态不显示图标因而无 tooltip（若产品需要在卡片上区分「未注册」语义，可另行提出）。

### B-011 心跳确实消失

- 前置：agent 开关 on 且已在线；打开 hub 日志（或 agent 日志）。
- 步骤：连续观察 **2 分钟以上**。
- 预期：**没有**每 60 秒一次的周期性 `board_register` 记录；期间除开关变更/身份变更外无注册帧。

---

## 回归用例

### B-012 升级迁移：旧 hub 数据启动新 hub

- 前置：使用**上一版 hub** 产生的 `hub-board.db`（条目无 `registered` 字段）。
- 步骤：用新 hub 启动，观察 Agents 清单图标。
- 预期：原先的 agent **不显示图标**（按未注册处理）；待 agent 重连并注册后自动恢复亮色（可接受一次性"图标迟到数秒"）。

### B-013 版本号与 BUILD 同步（CLI/Web 显示）

- 步骤：
  1. `work/co-shell-0.76.0.darwin.arm64 --version`；
  2. 启动 `work/co-shell-hub-0.76.0.darwin.arm64` 并查看徽标版本号。
- 预期：co-shell 与 co-shell-hub 均显示 `0.76.0`，BUILD 号一致且为 `[BUILD-1128]`；`main.go` 与 `cmd/co-shell-hub/main.go` 两处同步。

---

## 验收判定

- **必须通过（自动）**：A-001 ~ A-016。
- **必须通过（运行时）**：B-001、B-002、B-003、B-004、B-005、B-006、B-007、B-008、B-009、B-010、B-011。
- **回归确认**：B-012、B-013。
- **已接受的局限（不作为失败项）**：agent 进程僵死而 socket 未断时，状态会持续显示【在线】（去掉心跳的既定代价）。
- 全部通过后由用户确认合入 main（版本 v0.76.0，BUILD-1128）。
