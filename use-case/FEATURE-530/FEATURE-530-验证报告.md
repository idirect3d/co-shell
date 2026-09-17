# FEATURE-530 公告板 v2（消息模型）— 运行时验证报告

> 分支：FEATURE-530 ｜ 版本：v0.63.0 ｜ BUILD：1059
> 用例：`use-case/FEATURE-530/FEATURE-530-UC-0001.md`（UC-0001 ~ UC-0033，A~F 六组）
> 设计：`research/agent-board-design-v2.md`
> 验证日期：2026-09-16 ｜ 方式：真机 E2E（独立 hub + 两个 co-shell 实例）+ 单测 + 静态核对

---

## 1. 结论摘要

| 项 | 结果 |
|---|---|
| 用例总数 | 33（A~F 六组） |
| 通过 | **33 / 33** |
| 未通过 | 0 |
| 本轮发现并修复的真实缺陷 | **3**（见第 3 节） |
| E2E 驱动断言 | **23 checks / 0 failed**（首轮为 23 checks / 6 failed） |
| UC-0006 / UC-0026 专项校验 | **2/2 passed**（`tmp/fix530-e2e3/uc0006_check.py`） |
| `go build ./... && go vet ./...` | 全绿 |
| `go test ./...` | agent、hub/gateway、web 全绿；cmd 包 2 例失败为既有基线失败（见第 6 节） |

---

## 2. 验证方法与证据来源

- **E2E 环境**：`tmp/fix530-e2e3/`（hub tcp 29231 / web 29232，registry + bbolt `hub-board.db`；agent-a 29241、agent-b 29242）。
  - 驱动：`bash tmp/fix530-e2e3/run.sh` → `python3 driver.py`（TCP 注入 + REST 断言，逐条打印 `[EV] <UC> PASS/FAIL <证据>`），并在结尾重启 hub 做持久化对比。
  - 专项校验：`python3 tmp/fix530-e2e3/uc0006_check.py`（UC-0006 掉线、UC-0026 重启后扫描）。
  - hub 侧证据：`GET /api/board/conversations`、`.../{key}/messages`、`tmp/fix530-e2e3/logs/*.log`。
  - co-shell 侧证据：`tmp/fix530-e2e3/agent-{a,b}/log/co-shell-2026-09-16.log`。
- **单测**：`agent/board_test.go`（10 例）、`hub/gateway/board_test.go`（9 例）、新增回归 `hub/gateway/board_frame_test.go`、`web/board_frame_test.go`。hub 为独立 module，需 `cd hub && go test ./...`。
- **静态核对**：工具列表、旧报文处理分支、开关默认值、表单标签（用于 UC-0029~UC-0032）。

---

## 3. 本轮发现并修复的真实缺陷

### 缺陷 1（阻断级）：hub → agent 的 board 帧形态与 co-shell 不一致

- **现象**：UC-0001~0004 注册与 roster 全线失败（roster 空、rev=0）、UC-0007 广播 `deliveries` 为空、UC-0019 无超时事件；hub 日志 `board: agent agent-a sent a bad board_register payload`。
- **根因**：co-shell 按**扁平帧**收发（`{"type":"board_register","name":…}`），hub 却用 `marshalEnvelope` 发 `{type,payload}` 并读 `env.Payload`。
- **修复**：`hub/gateway/board.go` 新增 `marshalFrame` 并替换 `relay()`/`notify()`；`hub/gateway/proxy.go` 新增 `agentBoardFrame` 解析扁平帧，`sendBoardMessage` 重构为接收 `boardSendRequest`（客户端 envelope 路径不变）。
- **验证**：driver 由 6 failed 转为 0 failed。

### 缺陷 2：co-shell 侧 `board_message` 帧的消息 id 未解析

- **现象**：`session: board messages delivered (batch=4, busy=false, ids=,,,)`、`injected while busy (id= from=agent-a)`。
- **根因**：hub 的帧字段是 `id`，而 `handleBoardDelivery` 读的是 `MessageID`（json tag `message_id`，仅用于 `board_sent`/`board_timeout`）。
- **影响**：注入文本缺消息 id；去重表键恒空，决策 6「同一 message_id 不重复注入」失效。
- **修复**：`web/session_board.go` 改用 `msg.ID`。
- **验证**：`ids=m-32,m-33,m-34,m-35`、`injected while busy (id=m-37 from=agent-a)`。

### 缺陷 3（UC-0006 根因）：hub 链接重连后 co-shell 不重发 `board_register`

- **现象**：hub 重启后 agent 的 WS 自动重连（hub 日志 `gateway: connected agent agent-a`），但 registrar 只在会话启动时发送一次，导致 hub 侧 roster 长期保持 `online=false`、`roster_rev` 冻结，UC-0006 无法观测。
- **根因**：`initBoard` 只做一次性 `go s.registerBoard(name, duty)`；hub 采用「主动拨号」，重连对 agent 而言是一次全新的客户端连接，但没有任何回调触发重注册。
- **修复**：`web/server.go` 新增 `onConnect` 回调（`SetConnectHook`，在 `setConn` 中于新客户端接入时触发）；`web/session_board.go` 的 `initBoard` 在安装该钩子的同时保留启动时的一次注册（幂等，hub 侧会刷新 rev 并广播 roster）。
- **验证**：修复后 hub 重启，agent-a/agent-b 均重新注册（hub 日志 `board: agent agent-a registered (duty source=hub)`、`board: agent agent-b registered (duty source=agent)`），`post-restart rev` 明显递增；UC-0006 专项校验 PASS（`rev 30->31 roster={'agent-a': True, 'agent-b': False}`）。

---

## 4. 逐用例结果

### A 注册与 roster

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0001 连接后自动注册 | PASS | `[EV] UC-0001 PASS roster_ids=['agent-a','agent-b']`；hub `board: agent agent-a registered`；agent `board registration sent (name="agent-a")`（无 LLM 回合） |
| UC-0002 职责来源=agent | PASS | `[EV] UC-0002 PASS b duty='B 职责：测试与验证（agent 上报）' source='agent'` |
| UC-0003 hub 录入优先 | PASS | `[EV] UC-0003 PASS a duty='Hub录入A职责（hub优先）' source='hub'` |
| UC-0004 注册触发 roster 广播 | PASS | `[EV] UC-0004 PASS roster_rev=28`（同轮内 rev 连续递增，注册/掉线/重连均产生新 rev） |
| UC-0005 roster 注入 environment_details | PASS | 单测 `agent/board_test.go:TestBoardRosterBlock`（`<board_agents rev="N">`；空 roster 不产标签） |
| UC-0006 掉线时 roster 缩减 | PASS | 专项校验：`[EV] UC-0006 offline shrinks roster PASS rev 30->31 roster={'agent-a': True, 'agent-b': False}`（杀掉 agent-b 后 rev 递增、online 翻为 false） |

### B 发送与投递

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0007 广播排除自己 | PASS | `deliveries=[{"to":"agent-b","ok":true}]`，`conversation="*"`，发送者未收到 |
| UC-0008 定向私信 | PASS | `conversation="agent-a+agent-b"`，仅 agent-b `ok=true` |
| UC-0009 多收件人 | PASS | 多收件人 ack `deliveries` 正常；掉线收件人 `ok=false`+原因由单测 `TestBoardUnknownRecipient` 覆盖 |
| UC-0010 发送立即回执 | PASS | ack 秒级返回，含 `message_id=m-N`/`conversation`/`deliveries`/`timeout_seconds`，不等待收件人处理 |
| UC-0011 timeout_seconds 必填 | PASS | `error: timeout_seconds out of range (5..86400)`（缺省与越界同一路径）；工具层文案由 `TestBoardSendToolValidation` 覆盖 |
| UC-0012 范围校验 | PASS | `lo`/`hi` 均报错并回显区间 `5..86400` |
| UC-0013 回复 state 校验 | PASS | 缺失/非法均报 `state must be one of handled/declined/need_info when replying` |

### C 三态与超时

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0014 handled | PASS | `m-74` handled 回复投递给 `agent-a` |
| UC-0015 declined | PASS | `m-75` declined 回复投递给 `agent-a`（会話 `*`） |
| UC-0016 need_info | PASS | `m-76` need_info 回复投递给 `agent-a` |
| UC-0017 结局回复归并 | PASS | `replies_to_m-72=3 states=['handled','need_info','handled']` |
| UC-0018 no_reply 不计时 | PASS | `no_reply_msg=m-78`，`pending` 列表不含该消息 |
| UC-0019 超时未响应事件 | PASS | `msg=m-79 pending_before=1 pending_after=0`；co-shell 侧转 `board_timeout` 动态事件 |

### D 注入行为

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0020 空闲开新回合 | PASS | `board messages delivered (batch=4, busy=false, ids=m-32,m-33,m-34,m-35)`，随后 `Agent.RunStream: user input: [board-message m-32 from agent-b …]` |
| UC-0021 忙碌动态感知不打断 | PASS | `session: board message injected while busy (id=m-37 from=agent-a)`；driver `UC-0021 busy ack PASS` |
| UC-0022 多条合并一条注入 | PASS | driver `UC-0022 burst sent PASS 3 messages sent within the aggregate window`；单测 `TestBoardAggregatorMergesMessages` |
| UC-0023 同 id 不重复注入 | PASS | 单测 `TestBoardSeenDedupRing`；修复缺陷 2 后真机 `ids` 非空，去重键恢复有效 |
| UC-0024 去重表容量滚动 | PASS | 单测 `TestBoardSeenDedupRing`（环形缓冲淘汰语义） |

### E 持久化与 UI

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0025 bbolt 重启恢复 | PASS | 重启前后快照一致（`convs=3`、`msgs` 不变），`message_id` 序号与 `roster_rev` 继续递增（修复缺陷 3 后 rev 因重注册继续增长） |
| UC-0026 重启后未响应判定 | PASS | 专项校验：`[EV] UC-0026 awaiting after restart pre=6`、`[EV] UC-0026 post-restart sweep PASS pending 6->0`（重启前落库的 awaiting 在重启后仍按时被清扫，无丢失/重复） |
| UC-0027 Chat 栏会話聚合与公告唯一 | PASS | `keys=['*','agent-a+agent-b','agent-a']`，广播会話 `*` 唯一，未因多条广播产生重复会話 |
| UC-0028 Chat 栏消息流与徽标 | PASS | `count=12`，字段齐全 `['content','conversation','from','id','no_reply','reply_to','seq','state','timeout','to','ts']` |
| UC-0029 hub 配置项改名 | PASS | 静态：`hub/gateway/webui_static.go` 三个表单（新建/编辑/抽屉）标签均为「职责描述」；`hub-agents.json` 字段名仍为 `name` |

### F 清理与回归

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0030 旧工具彻底删除 | PASS | 静态：`board_post`/`board_list`/`board_claim`/`board_dm`/`board_confirm`/`board_result` 在 `agent/`、`hub/`、`web/`、`cmd/` 中已无实现引用（仅 `hub/gateway/agent.go:35` 一处历史注释）；工具注册仅剩 `board_send` |
| UC-0031 旧报文不再被识别 | PASS | 静态：`Proxy.HandleMessage` 仅分派 `switch_agent`/`list_agents`/`board_send`，客户端发 `board_register` 回 `board_register is agent-only`；`routeAgentMessage` 仅识别 `board_register`/`board_send` |
| UC-0032 开关默认关闭 | PASS | 静态：`config.BoardEnabled` 为 `bool`（零值 false）；未开启时 `registerBoard`、`handleBoardFrame`、`board_send` 注册三处均提前返回（本轮也实测到：agent-b 未开开关时不注册，开启后立即注册，行为符合预期） |
| UC-0033 编译、静态检查与单测 | PASS | `go build ./... && go vet ./...` 全绿；`agent`、`web`、`hub/gateway` 单测全绿；`cmd` 包 2 例为既有基线失败（见第 6 节） |

---

## 5. 验证过程中修正的环境问题（非产品缺陷）

| 现象 | 原因 | 处理 |
|---|---|---|
| agent-b 全程未注册、roster 中 `online=false` | `tmp/fix530-e2e3/agent-b/config.json` 缺少 `board_enabled`（开关默认关闭，属 UC-0032 的预期行为） | 补 `"board_enabled": true` 后正常注册 |
| UC-0002 `b duty=''` | agent-b 配置中的 `llm.agent_duty` 丢失（上报值为空） | 补回 `llm.agent_name/agent_duty` 后 `duty='B 职责：测试与验证（agent 上报）' source='agent'` |

两项均为**测试环境配置**问题（产品侧的开关默认关闭、空职责上报都是符合用例预期的行为），不是代码缺陷；在此记录以免后续重跑时重复踩坑。

---

## 6. 验收标准逐条核对

| # | 标准 | 结果 |
|---|---|---|
| 1 | 用例全通过或明确记录未通过项与原因 | ✅ **33/33 通过**（首轮 6 项失败已全部修复并复测） |
| 2 | `go build ./... && go vet ./...` 全绿 | ✅ 通过 |
| 3 | `go test ./...` 通过 | ⚠️ agent、hub/gateway、web 全绿；`cmd` 包 2 例失败属既有基线失败（`main` worktree 同样失败，本任务未改 `cmd/` 测试） |
| 4 | 旧 6 工具与工单模型删除，仅剩 `board_send` | ✅ 静态核对通过 |
| 5 | hub Chat 栏公告会話唯一置顶 + 定向会話 + 三态徽标 | ✅ REST 层面核对通过（`*` 唯一、定向会話按参与者命名、字段含 `state`/`reply_to`）；未逐像素走查 UI |
| 6 | hub 重启后 bbolt 恢复会話历史与 roster_rev | ✅ 快照一致；且修复缺陷 3 后 agent 重连会重新注册，rev 继续递增 |
| 7 | 两个入口版本号/build 一致且 ROADMAP 标注 | ✅ `main.go` 与 `cmd/co-shell-hub/main.go` 同为 v0.63.0 / BUILD 1059；ROADMAP 已标注 |

### 既有失败（与本任务无关，已确认基线一致）

| 测试 | 现象 | 基线核对 |
|---|---|---|
| `cmd.TestWebWizardModelNameStep` | `cmd/model.go:968` 空指针 panic（模型列表获取失败路径） | 将本轮改动 stash 后重跑仍失败 |
| `cmd.TestSettingsJSONFillsDefaults` | `setting "logo" (group "外观与显示") has empty Default` | 在 `main` worktree 中同样失败 |

---

## 7. 复验方式

```bash
# 1) 编译
go build ./... && go vet ./... && go build -o work/co-shell-0.63.0.darwin.arm64 .
(cd hub && go test ./...)   # hub 为独立 module
# 2) 单测
go test ./agent/ ./web/ -run Board -count=1
# 3) 真机 E2E（33 用例主体 + 重启持久化）
bash tmp/fix530-e2e3/run.sh          # 输出 [EV] 行与 driver summary
# 4) UC-0006 / UC-0026 专项（需在 run.sh 之后执行）
python3 tmp/fix530-e2e3/uc0006_check.py
```

## 8. 后续建议

1. `cmd` 包既有的 2 例失败建议单独登记修复任务（会长期污染 `go test ./...` 信号）。
2. `tmp/fix530-e2e3/uc0006_check.py` 建议并入 `run.sh` 作为常规阶段，避免掉线/重启后超时两类用例回归时无人发现。
3. 可考虑为 `SetConnectHook` 增加单测（连接建立时确实触发 board 重注册），使缺陷 3 的回归保护不依赖真机环境。

---

## 补充验证（2026-09-17）：公告板「双方互相显示 offline」根因修复 [BUILD-1060]

### 现象（用户报告）
两个 agent 互相把对方看成 offline：`co-shell-source` 侧注入的 roster 为
`<board_agents rev="5"><agent id="co-shell-source" status="online"/><agent id="co-shell-work-test" status="offline"/></board_agents>`，
而 `co-shell-work-test` 侧只看到自己在线。hub REST 权威 roster（rev=7）则是**两个 agent 都 online=false**，
但 hub Web UI 的 `/api/agents` 显示两者 `connected=true`。

### 根因（代码 + 运行时证据，非推测）
1. co-shell agent 的 `serve` 是**单客户端**模型：`web/server.go:setConn` 在新客户端接入时会关闭旧连接。
2. hub 的公告板每个 agent 只持有**一条** link（`hub/gateway/proxy.go` 的 `p.agents[id]`，仅在启动拨号时创建一次、从不替换），
   roster 广播与消息都从这条 socket 下推（`board.go:notify` → `link(id)`）。
3. 因此当**任何其它客户端**（hub Web UI 转发的会话、浏览器）连上该 agent 的 serve 端口时，agent 会关闭上一条连接——
   正是公告板的 link socket。于是：
   - `AgentConn.readLoop` 读失败 → `Close()` → `handleAgentGone` → `Board.SetOffline` → agent 被标记 offline（rev++ 并广播）；
   - 之后所有 `notify()` 都写向已关闭的 socket（`board: notify board_roster -> agent-a failed: use of closed network connection`），
     该 agent 再也收不到 roster 广播，其本地缓存**冻结在“自己注册那一刻的快照”**（自己 online、别人 offline）；
   - 而 agent 只在新客户端接入时才重发 `board_register`，hub 也从不重拨，状态永不收敛。

复现日志（修复前二进制，`tmp/fix530-e2e3/uc0034_check.py`）：

```
board: agent agent-a registered (duty source=hub)
board: notify board_roster -> agent-a failed: write tcp ...: use of closed network connection
board: agent agent-a went offline
[EV] UC-0034 roster survives takeover FAIL (rev 36->37, agent-a online=False, agent-b online=True)
```

注：`ps`/`lsof` 显示 hub 到同一 agent 存在多条 WS（本次实测 hub→agent-a 3 条、→agent-b 2 条），
正是这些“接管”连接不断顶掉公告板 link，与上述分析一致。

### 修复（hub 自愈 + agent 心跳兜底）
- `hub/gateway/agent.go`：新增 `Done()`，供守护循环感知链路断开。
- `hub/gateway/proxy.go`：
  - 把“一次性拨号”改为**每 agent 一个守护循环**（`superviseAgent`）：链路断开后按 2s→30s 退避重拨，重拨成功即替换 link；
  - 新增 `installAgentLink`：连接建立即 `Board.MarkOnline` + `BroadcastRoster()`（新链路上立刻下发最新 roster）；
  - `dropAgentLink`：**只有当前 link 关闭才**置 offline（被更新的连接顶掉的旧 socket 不再误判），并从 link 表移除死连接；
  - `RemoveAgent` 先标记 `removed` 再在锁外 `Close()`（避免 close 钩子重入 `p.mu` 造成死锁）。
- `hub/gateway/board.go`：
  - 新增 `MarkOnline(id)`（连接建立即置 online、rev 递增、触发刷新）；
  - `touchLocked` 返回是否发生 offline→online 翻转，`Send` 据此递增 rev——agent 的任何活动都会传播为可广播的状态变化；
  - `Register` 幂等：只有 roster 真正变化才递增 rev（避免心跳/重连造成的无意义 rev 抖动与提示词抖动）。
- `web/session_board.go`：新增 60s 周期心跳（`boardHeartbeat`）重发 `board_register` 作为兜底，保证状态最终收敛。

### 验证结果
| 用例/检查 | 结果 | 证据 |
| --- | --- | --- |
| UC-0034 接管后 roster 收敛（新增） | **PASS** | 修复前 FAIL（rev 36→37, agent-a offline）；修复后 PASS（rev 39→41，两个 agent online=true） |
| UC-0006 掉线缩减 roster | PASS | `rev 45->46 roster={'agent-a': True, 'agent-b': False}` |
| UC-0025 bbolt 重启持久化 | PASS | pre rev=43 → post rev=45，convs/msgs 一致 |
| UC-0026 重启后超时清扫 | PASS | `pending 6->0` |
| hub 单测（含新增回归） | PASS | `cd hub && go test ./...` → ok gateway；新增 4 个 board 单测全绿 |
| 编译/静态检查 | PASS | `go build ./... && go vet ./...`（root 与 hub 模块） |

新增回归单测（`hub/gateway/board_online_test.go`）：
`TestBoardRosterConvergesAfterLinkLoss`、`TestMarkOnlineIgnoresUnknownAgent`、
`TestRegisterDoesNotChurnRevision`、`TestSendTouchesAgentBackOnline`。

### 复跑命令
```bash
cd /Users/direct3d/github/co-shell
go build ./... && go vet ./... && (cd hub && go vet ./... && go test ./...)
python3 tmp/fix530-e2e3/uc0034_check.py          # 接管场景：期望 PASS
bash tmp/fix530-e2e3/run.sh                      # 协议/持久化/UC-0006/0026 回归
python3 tmp/fix530-e2e3/uc0006_check.py
```

### 遗留说明
- 修复后，hub 会在公告板 link 被顶掉后**重拨**（2s 起、最长 30s 间隔）以夺回链路；这会把“顶掉它的那个客户端”
  从 agent 的单客户端槽位上挤下来（serve 模式的既有语义）。若后续要彻底避免这种互相顶替，可考虑让 agent 支持
  多客户端或让 hub 的 Web UI 转反复用公告板连接（不在本次范围内）。
- 两个 agent 进程需重启到 BUILD-1060 才会带上 agent 侧心跳兜底；hub 侧的修复对旧 agent 也生效（重连后旧 agent 由
  `onConnect` 钩子重发注册）。

---

## 追加修复（BUILD-1062）：hub 网关连接独立槽位，消除「启动 agent 后秒变未连接」

### 问题（用户 2026-09-17 反馈）
在 hub Web UI 对 stopped 的 agent 点「启动」→ 短暂显示已连接 → 很快（且周期性）变成未连接；手动重启 agent 进程后可长时间保持连接。
取证：用户运行的 hub 是 BUILD-1061（`work/co-shell-hub-0.63.0.darwin.arm64`，二进制字节数与 1061 产物一致），故该现象发生在 1061 上。

### 根因
- agent serve 模式只接受一个 WS 客户端（`web/server.go:setConn` 新客户端接入即 `old.Close()`）。
- hub 需要一条自己到 agent 的 AgentConn（公告板 roster/消息 + 代理帧）。
- hub Web UI 又把 agent 自身 UI 以 iframe 内嵌（`/agent/{id}/`，FEATURE-484），浏览器因此是第二个客户端。
- 两者互抢同一槽位：浏览器顶掉 hub link → `dropAgentLink` 把该 agent 移出 link 列表 → `/api/agents` 的 `connected` 立刻转 false → hub 2s 后重拨又把浏览器连接顶掉 → 浏览器重连再顶掉 hub……周期性拉锯。
- 为什么现在才暴露：BUILD-1061 之前 `connected` 依赖的 link 列表在掉线时不清理（“假在线”）；1061 引入 supervise/drop 后语义变准，抖动遂可见。

### 证据（对照实验，`tmp/fix530-e2e3/uc0035_check.py`）
| 构建 | phase A 无竞争 | phase B 单个客户端占位 | phase C 客户端持续抢占 |
| --- | --- | --- | --- |
| BUILD-1061（修复前） | True x7.8s | False x1.6s → True x9.9s | **9/18 采样未连接**（hub 日志每 2s：`read error: EOF` → `went offline` → 重连） |
| BUILD-1062（本修复） | True x7.8s | True x12.0s | **0/18 未连接**，全程 True |
| BUILD-1059（更早） | True | True | 0/18，但为假在线（agent 实际已被顶掉） |

### 修复（用户选定方案 A：agent 侧网关独立槽位）
- `hub/gateway/agent.go`：新增 `gatewayURL()`，hub 拨号时给 agent URL 加 `role=hub`（managed/external 的 registry 配置无需改动）。
- `web/server.go`：新增 `gwConn` 网关槽位；`handleWS` 按 `?role=hub` 分流（网关连接不推 state）；`setGatewayConn`/`clearGatewayConn` 只操作网关槽位（浏览器接入、网关断线都不触发 `failAll`）；`sendRaw`（board 帧）改为「网关 + 浏览器」双发，两者皆无连接时仍返回错误以保留注册重试语义。
- 语义保持：浏览器之间仍是单客户端接管（新标签页顶掉旧标签页）。

### 验证结果
| 用例/检查 | 结果 | 证据 |
| --- | --- | --- |
| UC-0035 启动后连接稳定性（新增） | **PASS** | 修复前 9/18 未连接、phase B 掉线 1.6s；修复后 0/18，phase A/B/C 全 True |
| web 单测（新增） | PASS | `TestGatewayLinkCoexistsWithBrowser`、`TestBrowserRoleStaysInBrowserSlot` |
| hub 单测（新增） | PASS | `TestGatewayURLTagsRole`；`cd hub && go test ./...` 全绿 |
| UC-0034 接管后 roster（断言按新语义更新） | PASS | rev 54→54、两 agent online=true（网关槽位后接管不再断 hub link，故不再要求 rev 递增） |
| UC-0001~0028 / UC-0025（run.sh） | PASS | driver 全部 PASS；UC-0025 pre rev=56 → post rev=58，convs/msgs 一致 |
| 编译/静态检查 | PASS | `go build ./... && go vet ./...`（root 与 hub 模块） |

### 复跑命令
```bash
cd /Users/direct3d/github/co-shell
mkdir -p tmp/fix530-e2e3/bin-gw
go build -o tmp/fix530-e2e3/bin-gw/co-shell . && go build -o tmp/fix530-e2e3/bin-gw/co-shell-hub ./cmd/co-shell-hub
python3 tmp/fix530-e2e3/uc0035_check.py --label gw \
  --agent-binary tmp/fix530-e2e3/bin-gw/co-shell --hub-binary tmp/fix530-e2e3/bin-gw/co-shell-hub
python3 tmp/fix530-e2e3/uc0034_check.py \
  --agent-binary tmp/fix530-e2e3/bin-gw/co-shell --hub-binary tmp/fix530-e2e3/bin-gw/co-shell-hub
bash tmp/fix530-e2e3/run-gw.sh
```

### 兼容性说明
- agent 与 hub 需同时升级到 BUILD-1062 才启用网关独立槽位；旧 hub（拨号不带 `role`）访问新 agent 时仍走浏览器单客户端语义，`sendRaw` 有回退，board 功能不受影响。
