# FEATURE-542 验收测试用例（UC-0001）：Web 会话韧性 + 多端一致性

> 权威依据：`docs/web-session-resilience.md`（v0.99.0 版，§1~§8）。分支 **FEATURE-542**，版本 **v0.99.0**。
> 本轮用例对应设计文档：**§3 需求 R1~R9** / **§4 缺口 GAP-1/2/3** / **§5.2 两类事件** / **§5.3 七点改造** / **§5.6 四项一致性约束** / **§7 九项验收标准**。
> 在线感知（§9、S1~S5）见同目录 **FEATURE-542-UC-0002.md**。

> 根因（设计文档 §2.4）：架构把「客户端暂时不可达」当成「用户永久不可达」——`WebIO.failAll` 在断连瞬间把所有 pending 人工交互通道置 `errNoWebClient`。

> 改动范围（预期实现，对应 §5.3 七点 + §9.5）：
> `web/server.go`（`conns []clientConn` 列表 + fan-out + 不踢旧连接 + 断连不 `failAll` + `HasClient()`）、
> `web/session.go`（`Ask`/`ask` 落 `interaction_request`；`resolve`/`resolveInteraction` 落 `interaction_resolved`；两条路径统一 id 空间/pending 表；无客户端永久阻塞；FIFO 队列带 seq 与来源端）、
> `store/eventstream.go`（新增 `LoadEventsAfter`）、
> `agent`（Interrupt 唤醒挂起交互 + `SetUserPresenceProvider` + `buildFullEnvironmentDetails` 注入 `<user_presence>`）、
> `agent/env_prune.go`（`user_presence` ∈ `envLiteTags`）、
> `web/static/app.js`（自动重连 + `lastSeq` 增量 + 事件驱动弹窗/关闭 + `client_id`）、
> `i18n`（zh/en 环境感知与 RULES 策略）、
> hub（`gwConn` 与多端共存）。

> 已确认决策（不可改动）：
> ① 多端**共享 agent 同一当前会话**是预期行为（任一端 `session_switch` 影响所有端，不做端间隔离，§5.6.1）；
> ② 前端弹出/关闭**必须由事件流驱动**（§5.6.2）；
> ③ 并发回答**先到先得 + 后到静默忽略**（§5.6.3）；
> ④ 两条交互路径**必须统一**（一个 id 空间 + 一套事件 + 一份 pending 表，§5.6.4）；
> ⑤ 撤销 FEATURE-458「不自动重连」，实施时须在代码注释与 ROADMAP 决策记录写明撤销理由（§5.5）；
> ⑥ 挂起交互**不超时**（永久挂起，直到重连或 interrupt，R7/§6.1）。

## 用例清单

| 编号 | 名称 | 类型 | 对应设计条目 |
|------|------|------|--------------|
| A-001 | `sendJSON` fan-out：多连接时每个端都收到 | 单测 | §5.3#1 |
| A-002 | 第二个连接接入**不踢掉**第一个连接 | 单测 | §5.3#1、R5、验收 3 |
| A-003 | 摘除一个连接后，其余连接仍能收到推送 | 单测 | §5.3#1/2 |
| A-004 | 断连钩子**不再** `failAll`：pending 交互不被判失败 | 单测 | §5.3#2、R7、GAP-2 |
| A-005 | `HasClient()`：无浏览器连接为 false，有连接为 true | 单测 | §9.5#2 |
| A-006 | FIFO 队列：多发送方按到达顺序出队，不丢不乱 | 单测 | GAP-3、R5 |
| A-007 | 队列消息携带来源端标识（`client_id`）与 seq | 单测 | GAP-3、R5、§5.3#7 |
| A-008 | `WebIO.Ask` 先落 `interaction_request` 事件再投递 | 单测 | GAP-1、§5.2、§5.3#3 |
| A-009 | `WebIO.ask`（工具确认路径）同样落 `interaction_request`，与 Ask 同 id 空间 | 单测 | §5.6.4、§5.3#3 |
| A-010 | 无客户端时 `Ask`/`ask` 不返回 `errNoWebClient`，而是阻塞等待 | 单测 | R7、GAP-1、§5.3#3 |
| A-011 | `resolve` 先落 `interaction_resolved`（含 `resolved_by`）再唤醒 agent | 单测 | §5.2、§5.3#4 |
| A-012 | `resolveInteraction` 与 `resolve` 走同一事件 + 同一 pending 表 | 单测 | §5.6.4、§5.3#4 |
| A-013 | 并发回答竞态：先到先得，后到静默忽略（只 1 个 resolved） | 单测 | §5.6.3 |
| A-014 | 断连期间新发起的交互不失败（不产生 errNoWebClient） | 单测 | §2.3-2、R7 |
| A-015 | `LoadEventsAfter`：按游标向后增量取数（含边界） | 单测 | §5.3#6、验收 8 |
| A-016 | `LoadEvents`（向前翻页）行为未被破坏（回归） | 单测 | §5.4 |
| A-017 | interrupt 唤醒所有挂起交互，回传「用户已取消」语义 | 单测 | GAP-2、R8、§5.3#5 |
| A-018 | 唤醒后 agent 侧收到取消语义**而非**错误（无 errNoWebClient 泄漏） | 单测 | GAP-2、验收 6 |
| A-019 | `gwConn` 与浏览器 `conns` 共存互不顶替 | 单测 | R9、验收 7 |
| A-020 | 交互事件经 `AppendEvent` 落库，重连可回放读到（R2/R4 数据基础） | 单测 | GAP-1、R2、R4 |
| A-021 | `app.js` 静态断言：事件驱动弹窗/关闭 + `lastSeq` + `client_id` + 自动重连 | 单测（静态） | §5.3#7、§5.5、§5.6.2 |
| A-022 | 撤销 FEATURE-458 的实施注记（注释说明撤销理由） | 单测（静态） | §5.5 |
| A-023 | 版本与构建号一致（co-shell / co-shell-hub） | 单测 | PROJECT STANDARDS |
| B-001 | 断网 30 分钟，agent 仍跑完，无中断记录 | 运行时 | R1、验收 1 |
| B-002 | 断连时挂起的提问，重连后重新弹框；回答后从原处继续，无重复工具调用 | 运行时 | R4、验收 2 |
| B-003 | 两个浏览器 tab 同时连接，均收完整事件流，互不顶替 | 运行时 | R5、验收 3 |
| B-004 | 任意端发指令按 seq 串行执行，事件记录来源端标识 | 运行时 | R5、验收 4 |
| B-005 | agent 忙时另一端发指令 → 走动态感知注入 | 运行时 | R6、验收 5 |
| B-006 | 暂停/终止键解锁挂起交互并回「用户已取消」 | 运行时 | R8、验收 6 |
| B-007 | hub 链路共存：hub 断连不影响 agent 执行，公告板私信正常 | 运行时 | R9、验收 7 |
| B-008 | 一端回答，其余端自动关闭对应对话框 | 运行时 | §5.6.2、R4 |
| B-009 | 多端共享同一当前会话：任一端 `session_switch` 影响所有端 | 运行时 | §5.6.1 |
| B-010 | 两端几乎同时回答同一提问 → 先到先得，另一端静默关闭 | 运行时 | §5.6.3 |
| B-011 | `ask_user` 与工具确认两条路径在多端下表现一致 | 运行时 | §5.6.4 |
| B-012 | 永久挂起语义：无客户端提问不超时，session 保持 busy，interrupt 可解锁 | 运行时 | R7、§6.1 |
| B-013 | 重连按 `lastSeq` 增量补齐，界面与未断线时一致 | 运行时 | R3、验收 1 |
| B-014 | 断连→重连期间事件不丢、顺序不变 | 运行时 | R2、验收 1 |
| B-015 | `go build ./... && go vet ./... && go test ./...` 全绿 | 运行时 | 验收 9 |

---

## A 组：单元测试（Go）

- **A-001**：构造 `Server` 并注册 3 个 `wsConn`（可注入假连接/内存管道）→ 调 `sendJSON(serverMessage{...})` → 断言 3 个连接各自的写出通道均收到同一帧，且返回成功。
- **A-002**：注册 conn1 → 注册 conn2 → 断言 conn1 **未被关闭**、仍留在列表中，且 conn1 随后仍能收到 `sendJSON` 广播（回归 FEATURE-458 时代的「踢旧连接」`setConn` 行为必须消失）。
- **A-003**：注册 conn1/conn2 → 摘除 conn1（模拟断连）→ 断言 `sendJSON` 仍送达 conn2 且返回成功；列表长度由 2 → 1。
- **A-004**：注册 conn → 通过 `WebIO.ask` 发起一次交互（pending 中）→ 触发断连钩子 → 断言 pending 通道**未被**写入 `errNoWebClient`，交互仍处于等待状态（可继续被 `resolve` 唤醒）。
- **A-005**：无连接时 `HasClient()` == false；注册一个浏览器连接后 == true；摘除后回到 false；**仅** `gwConn` 存在（无浏览器连接）时仍为 false。
- **A-006**：向队列并发投递 N 条消息（多 goroutine，模拟多端）→ 依次出队 → 断言出队顺序与投递到的 seq 严格递增一致、条数未丢（FIFO 显式队列语义，不依赖 channel 竞争）。
- **A-007**：投递带 `client_id` 的指令 → 出队消息保留 `client_id`；断言 seq 单调 +1 且来源端标识不丢失。
- **A-008**：调用 `WebIO.Ask(ctx, in)` → 断言在 fan-out 之前已通过 `AppendEvent` 落一条 `interaction_request`（含 `id`/`kind`/`payload`/`source`），且事件 seq 存在。
- **A-009**：调用 `WebIO.ask(mode)`（工具确认/向导路径）→ 断言同样落 `interaction_request`，且其 `id` 与 `Ask` 路径处于**同一 id 空间**（同一 pending 表可被 `resolve`/`resolveInteraction` 命中）。
- **A-010**：无任何浏览器连接时调用 `Ask`/`ask` → 断言**不返回** `errNoWebClient`，返回前一直阻塞（goroutine 处于等待态；用超时探测确认「仍在等待」而非「已失败」）。
- **A-011**：pending 交互存在时调 `resolve(id, value)` → 断言：① 先落 `interaction_resolved`（含 `answer` 与 `resolved_by`）；② 随后 pending 通道被唤醒；③ fan-out 广播该 resolved 事件。
- **A-012**：对同一 pending 交互，分别用 `resolveInteraction(id, res)` 与 `resolve(id, value)` 路径验证：落到同一 pending 表项、产生同一形态的 `interaction_resolved` 事件（**一个 id 空间 + 一套事件 + 一份 pending 表**）。
- **A-013**：两端并发调用 `resolve` 同一 id（可用 barrier 保证几乎同时）→ 断言：恰好产生 **1** 个 `interaction_resolved` 事件；`resolved_by` 记录先到者；后到调用**静默返回**（无错误、无第二个事件、不改写答案）。
- **A-014**：断连（无客户端）状态下发起新的交互 → 断言不产生 `errNoWebClient` 失败路径；交互进入 pending 等待（这是 §2.3-2「断网期间每碰到确认就吃一次错误」的正面回归）。
- **A-015**：写入若干事件（seq 1..10）→ `LoadEventsAfter(sessionID, 3, 100)` 断言返回 seq 4..10 且升序；边界：`afterSeq=0` 返回全部；`afterSeq=10` 返回空；`limit=3` 只返回 seq 4..6 且不越界；跨 session 隔离（另一 session 事件不混入）。
- **A-016**：既有 `LoadEvents(sessionID, limit, beforeSeq)` 向前翻页用例保持通过（不改语义、不替换接口，§5.4）。
- **A-017**：agent 侧构造挂起交互（等待中）→ 调 `Interrupt()` → 断言所有挂起交互被唤醒，且回传给 LLM 的语义为「用户已取消」。
- **A-018**：承 A-017，断言 agent 侧观察到的是取消语义对象（如专用 `InteractionResult`/错误类型），**不是** `errNoWebClient`；且工具确认框等所有挂起点均被覆盖（不遗漏 `ask` 与 `Ask` 两条路径）。
- **A-019**：注册浏览器连接 conn1 → 再注册 `gwConn` → 断言两者独立存在（`setGatewayConn` 不清 `conns`，`clearGatewayConn` 不影响 `conns`，反之亦然）；hub 报文经 `gwConn` 正常收发。
- **A-020**：`WebIO.Ask` 落库后，用 `LoadEvents` 反查该 session → 能从持久化事件流读出 `interaction_request`；未 resolved 时重连回放可发现该请求（R4 数据基础）。
- **A-021**（静态断言，作用于 `web/static/app.js`）：① 存在自动重连逻辑且退避序列为 1s/2s/5s…上限 30s；② 重连请求携带 `lastSeq` 走增量取数；③ 存在 `interaction_request` → 弹出、`interaction_resolved` → 按 id 关闭的处理分支；④ 指令体携带 `client_id`；⑤ 来源端标识渲染进 YOU 块。
- **A-022**（静态断言）：`app.js` 中关于自动重连的注释写明**撤销 FEATURE-458 决策的理由**（多端共存后「两个浏览器互抢单连接」的前提已消失）。
- **A-023**：读取 `main.go` 的 `build` 与 `cmd/co-shell-hub/main.go` 的 `hubBuild`，断言两者数值一致（不受本次改动破坏）；版本号均为 `0.99.0`。

---

## B 组：运行时验收（人工/端到端）

- **B-001（R1、验收 1）**：`--serve` 启动 → 浏览器下发长任务 → **断开网络 30 分钟** → 观察日志 agent 持续执行至完成、无中断记录 → 恢复网络 → 界面补齐全部事件。
- **B-002（R4、验收 2）**：任务中途弹出确认框时断网 → 恢复网络重连 → 对话框**自动重现**（由回放的 `interaction_request` 驱动）→ 回答 → agent 从原处继续，日志确认工具**仅执行一次**（无重复调用）。
- **B-003（R5、验收 3）**：打开 tab1 → 再打开 tab2 → 断言 tab1 **未被踢掉**，两端均持续收到完整事件流（对照旧行为：tab2 接入会踢掉 tab1）。
- **B-004（R5、验收 4）**：两端几乎同时各发一条指令 → 检查事件流：按 seq 串行执行、顺序稳定可复现；每条指令事件带来源端标识（YOU 块显示来源端）。
- **B-005（R6、验收 5）**：agent 运行中从另一端发指令 → 断言走 `Agent.AddDynamicEvent` 动态感知注入路径（与现有多端语义一致），不新增排队 UI。
- **B-006（R8、验收 6）**：确认框弹出时按暂停/终止键（任意端）→ agent 收到「用户已取消」语义，交互被解锁，**不报错**；session 从 busy 恢复。
- **B-007（R9、验收 7）**：浏览器端在线 + hub 链路同时连接 → 断开 hub 连接 → 断言 agent 继续执行不受影响；公告板/私信功能照常工作（只是不能推给前端）；前端重连后可读持久化会话。
- **B-008（§5.6.2）**：tab1 弹出问答框 → 在 tab2 回答 → 断言 tab1 的问答框**自动关闭**（由 `interaction_resolved` 事件驱动，而非本地 `hideAsk()`）。
- **B-009（§5.6.1）**：tab1 执行 `session_switch` 切到会话 B → 断言 tab2 也切到会话 B（多端共享同一当前会话，UI 不暗示会话独立）。
- **B-010（§5.6.3）**：两端几乎同时提交同一提问的答案 → 断言 agent 只采纳先到者、只产生一个 resolved 事件；后答端界面因 resolved 事件静默关闭，无报错弹窗。
- **B-011（§5.6.4）**：分别验证两条路径的多端一致：① `ask_user` 提问；② 工具执行确认框（`WebIO.ask`）。两者均须：多端同时弹出、一端回答他端自动关闭、重连后可回放重现。
- **B-012（R7、§6.1）**：无任何浏览器连接时 agent 发起提问 → 断言**不超时**、永久挂起；session 保持 busy（状态指示持续）；此时任意端按暂停键可解锁（依赖 B-006 路径）。
- **B-013（R3、验收 1）**：断线期间产生大量事件 → 重连 → 前端按 `lastSeq` 增量补齐 → 断言界面呈现与「未曾断线」时一致（无重复、无缺段、顺序正确）。
- **B-014（R2）**：断连→重连全过程录制事件序 → 断言事件完整、顺序不变、无丢失（事件队列为唯一数据源，前端不持有独立权威状态）。
- **B-015（验收 9）**：一次性执行 `go build ./... && go vet ./... && go test ./...` → 全部通过；并按 PROJECT STANDARDS 编译 `work/co-shell-0.99.0.*` 与 `work/co-shell-hub-0.99.0.*` 产物。

---

## 覆盖矩阵（R / GAP / §5.6 / §7）

| 设计条目 | 覆盖用例 |
| --- | --- |
| R1 指令发出即跑完 | B-001、B-014 |
| R2 全过程持久化 | A-020、B-013、B-014 |
| R3 重连尾部召回无差别 | B-013、A-015 |
| R4 提问也要重连弹框 | A-008、A-020、B-002、B-008 |
| R5 多端同时连接/串行/来源端标识 | A-002、A-006、A-007、B-003、B-004 |
| R6 忙时走动态感知注入 | B-005 |
| R7 挂起交互永久等待不超时 | A-010、A-014、B-012 |
| R8 任意端暂停键可取消 | A-017、A-018、B-006 |
| R9 hub 链路与多端共存 | A-019、B-007 |
| GAP-1 交互请求未持久化 | A-008、A-009、A-020 |
| GAP-2 暂停键无法唤醒挂起交互 | A-004、A-017、A-018、B-006 |
| GAP-3 `inputCh` 无缓冲顺序不可控 | A-006、A-007 |
| §5.2 两类新事件 | A-008、A-011、A-020 |
| §5.3 #1/#2 连接列表、fan-out、不踢旧、断连不 failAll | A-001、A-002、A-003、A-004 |
| §5.3 #3/#4 交互落库 + 无客户端阻塞 + resolved 广播 | A-008、A-009、A-010、A-011、A-012 |
| §5.3 #5 Interrupt 唤醒挂起交互 | A-017、A-018 |
| §5.3 #6 `LoadEventsAfter` | A-015、A-016 |
| §5.3 #7 前端重连/增量/事件驱动弹窗 | A-021、B-002、B-008、B-013 |
| §5.6.1 多端共享同一会话 | B-009 |
| §5.6.2 弹出/关闭由事件流驱动 | A-021、B-002、B-008 |
| §5.6.3 先到先得 + 后到静默忽略 | A-013、B-010 |
| §5.6.4 两条交互路径统一 | A-009、A-012、B-011 |
| §5.5 撤销 FEATURE-458 | A-021、A-022 |
| §7 验收 1~9 | B-001~B-009、A-015、B-015 |
