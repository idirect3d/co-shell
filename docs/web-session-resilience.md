# Web 会话韧性设计：事件队列为单一事实源

> 状态：设计已确认，待实施（编号 FEATURE-542；原拟编号 FEATURE-541 已让位给同期的 FIX-541 配置保存修复）
> 作者：L.Shuang / co-shell developer
> 创建：2026-09-19
> 关联：FEATURE-458（前端不自动重连，本设计撤销其决策）、FEATURE-507/508（事件持久化与历史回放，本设计复用）、
> FEATURE-388（Interaction 交互模型）、FEATURE-369（await_input/turn_start 边界）、FEATURE-530（hub 网关与公告板）、
> FIX-475（WS 写超时与异步发送队列）
> 更新：2026-10-06 追加 §9「用户在线感知与在线/离线策略」（评审后追加，本次先落设计、暂不编码）

---

## 1. 背景与问题

### 1.1 用户提出的问题

> 现在如果浏览器或者移动客户端断开网络，后台运行的 co-shell 似乎会因为 web socket 断开，导致工作异常且无法继续（需要先证实），这样的异常终止不是我所期望的。有没有什么改进优化的方案？

### 1.2 用户期望的目标语义（原文归纳）

1. **不论有没有前端在监控**，只要指令发出去，除非向用户提问，就应该**一直运行完**。
2. 整个过程都应该**保存下来**（持久化）。
3. 前端取数都**从持久化后的队列中取最新的信息**；用户重连之后，按缓存召回策略**只提取最后一部分重建前端会话**即可，本质上与正常连续取没有区别。
4. 如果最后是**提问**，也要正常**弹出对话框**。
5. 这样也能够**支持多端同时连接**（都是从持久化的信息取数据）。

---

## 2. 现象证实（代码证据链）

### 2.1 断连如何被发现

| 环节 | 代码位置 | 行为 |
| --- | --- | --- |
| 写侧探活 | `web/ws.go:42` `wsWriteTimeout = 10 * time.Second`；`web/ws.go:152-165` `writeLoop` 写失败即 `c.Close()` | 断网后**最迟 10 秒**发现（前提是发生过一次推送，才有写动作触发） |
| 读侧探活 | `web/server.go:666-672` `handleWS` 的 `c.ReadMessage()` 返回 err 即 `return` | 对端 FIN/RST 到达即退出；**服务端不主动发 ping、无 `SetReadDeadline`**，纯静默的半开连接可能长期不被判死 |
| 断连钩子 | `web/server.go:663-664` `defer c.Close()` + `defer s.clearConn(c)`；`web/server.go:521-530` `clearConn` 内调用 `onDisconnect()` | 浏览器槽位断开会触发钩子 |
| 网关槽位差异 | `web/server.go:654-659` 网关分支走 `clearGatewayConn`；`web/server.go:517-519` 该函数**只摘除连接、不调用 `onDisconnect`** | hub 链路断开不会触发 `failAll` |
| 钩子内容 | `web/session.go:141` `srv.SetDisconnectHook(sess.wio.failAll)` | **断连 = 立即 `failAll()`** |

### 2.2 不会发生的事（先排除误判）

- **进程不会退出**：`WebSession.closed` 只在 `WebSession.Close()`（`web/session.go:1094-1101`）时关闭，断连不关闭它。
- **事件不会丢失**：`WebRenderer.Render`（`web/session.go:1324-1354`）是**先 `AppendEvent` 落库、再 `sendEvent` 推 WebSocket**，顺序是：
  1. `coalescer.Add(ev)` 把流式碎片合并为完整块；
  2. `r.s.ag.Store().AppendEvent(r.s.ag.CurrentSessionID(), data)` 持久化；
  3. `r.s.srv.sendEvent(ev)` 推送（失败仅意味着这一帧没送达，不影响第 2 步）。
  所以断线期间产生的事件**完整保留**，重连后可恢复（配合 FEATURE-507 的历史回放）。
- **REPL 主循环等待输入时是安全的**：`WebSession.ReadLine`（`web/session.go:1058-1081`）只 `select` 两个分支——`s.inputCh` 与 `s.closed`。断连不关 `s.closed`，因此它会一直等下去，不会返回错误。
- **写侧不会阻塞 agent**：FIX-475 已实现有界异步发送队列（`web/ws.go:44-49` `wsOutQueueSize = 512`），生产者只入队、队列满则丢弃并返回 nil，绝不因网络阻塞 agent 循环。

### 2.3 会发生的事（这就是"工作异常"的真实来源）

1. **挂起的人工确认/提问被立即判失败**
   `WebIO.failAll`（`web/session.go:1269-1277`）在断连瞬间把所有 pending ask 通道塞入 `askResult{err: errNoWebClient}`。
   受影响范围：工具执行确认、`ask_user` 提问、模型配置向导等**所有需要人类输入的挂起点**。
   后果：agent 侧看到的是"工具调用失败 / 未收到答案"，LLM 可能因此**重复执行、换路径、或直接放弃任务**——这正是用户观察到的"异常"。

2. **断连期间新发起的交互同样立即失败**
   `WebIO.Ask`（`web/session.go:1209-1211`）与 `WebIO.ask`（`web/session.go:1236-1238`）在 `sendInteraction` / `sendAsk` 返回 false 时直接返回 `errNoWebClient`；
   而 `Server.sendJSON`（`web/server.go:536-547`）在 `s.conn == nil` 时返回 false。
   后果：断网期间 agent **每碰到一次需要确认的工具，就吃一次错误**。

3. **一条极窄的安全边界（潜在隐患）**
   `repl/repl.go:329-339`：
   ```go
   input, err := r.session.ReadLine(prompt)
   if err != nil {
       if err.Error() == "interrupt" { ... os.Exit(0) }
       break          // ← 任何非 nil error 都会结束主循环
   }
   ```
   目前 web 侧 `ReadLine` 只在 `s.closed` 时返回 `io.EOF`，所以断连不触发退出。但**任何未来在断连路径上给 ReadLine 增加错误返回，都会导致整个 serve 进程退出**。

4. **前端不会自动重连**
   `web/static/app.js:457-458`：
   > "When disconnected, we do NOT auto-reconnect — the user must click the control to reconnect (avoids two browsers fighting over the single connection)."
   这是 FEATURE-458 的有意决策。断网恢复后界面停在断开态，用户看到的就是"卡住/无法继续"。

### 2.4 根因定性

不是"WebSocket 断开把 agent 弄崩了"，而是**架构上把"客户端暂时不可达"当成了"用户永久不可达"**：
断连瞬间就把所有等待人类输入的挂起点判死刑。

这在单客户端局域网时代是合理的（断连 ≈ 用户走了），但在**移动客户端切后台、地铁断网、Wi-Fi 漫游**这类秒级到分钟级可恢复的断连场景下，代价就是任务被污染。

---
## 3. 需求（用户逐条确认）

| 编号 | 需求 | 用户原话 / 决策 | 验收方式 |
| --- | --- | --- | --- |
| R1 | 指令发出即跑完，与前端在否无关 | "不论有没有前端在监控，只要指令发出去，除非向用户提问，就应该一直运行完" | 断网 30 分钟，agent 仍跑完；日志无中断记录 |
| R2 | 全过程持久化，前端只从队列取数 | "整个过程，都应该保存下来" | 事件队列是唯一数据源，前端不持有独立权威状态 |
| R3 | 重连按尾部召回重建，与连续取无差别 | "前端取数都是从持久化后的队列中取最新的信息……按照缓存召回策略，只要提取最后的一部份重建前端会话即可，本质上与正常连续取没有区别" | 重连后按 `lastSeq` 增量补齐，界面与未断线时一致 |
| R4 | 最后是提问也要重连弹框 | "如果最后是提问，也要正常弹出对话框" | 回放尾部存在未 resolved 的交互 → 重新弹出对话框 |
| R5 | 多端同时连接，任意端可发指令 | 决策："任意端都可发指令，按 seq 入队串行执行，事件里记录来源端标识" | 两个浏览器 tab 同时收流；指令按 seq 串行；事件带来源端标识 |
| R6 | agent 忙时发指令走现有动态感知注入 | 决策补充："如果 agent 正忙，都可以通过动态感知注入方式'通知'ag​ent，与现在语义相同" | 复用 `Agent.AddDynamicEvent`，不新增排队 UI |
| R7 | 挂起交互永久等待，不超时 | 决策："不超时：永久挂起，直到重连或用户手动取消" | 无超时计时器；挂起状态可跨断连存活 |
| R8 | 取消用现有暂停/终止键，任意端可取消 | 决策："取消使用现在的暂停/终止键就可以了吧？哪个端都可以取消" | 任意端触发 interrupt → 挂起交互被解锁 |
| R9 | hub 链路与多端共存互不顶替 | 决策 + 补充："本次一并保证：hub 链路（gwConn）与新 conns 列表共存、互不顶替——如果是 hub 与前端断掉，也应该是同样的保证 co-shell 能够持续执行。agent 间公告板私信都应该正常工作，只是没法发给前端，待前端重连后，应该能读取现在持久化的会话" | hub 断连不影响 agent 执行；公告板私信照常；前端重连后可读持久化会话 |

---

## 4. 现状与需求的差距：三个必须一并修的缺口

设计评审时通过读代码发现了三个**硬缺口**。它们不是可选的优化，而是 R4、R8 成立的前提，必须与主改造一起完成。

### GAP-1：交互请求完全未持久化（本任务核心工作量）

`Server.sendAsk` / `Server.sendInteraction`（`web/server.go:549-557` 区域）走的是 `s.sendJSON(...)` **直接下发**，**从不经过 `AppendEvent`**。

后果：
- 事件流里只有 LLM 输出、工具调用等事件，**没有"agent 正在提问"这件事**；
- 因此"重连后重新弹出对话框"（R4）**目前没有任何数据支撑**——不是"改一下前端"就能实现的；
- 必须新增一类持久化事件来承载交互请求，否则 R4 无法成立。

### GAP-2：暂停/终止键无法唤醒挂起的交互

`web/session.go:226-229`：
```go
case "interrupt":
    s.ag.Interrupt()
    // FEATURE-524: ESC also ends a parked render_ui(waiting=true) instead of
    s.ag.ReleaseUIWait()
```

而 `WebIO` 的 pending 通道**只被四个路径唤醒**：`resolve`（同进程回答）、`resolveInteraction`（结构化回答）、`failAll`（断连）、`sessionClosed`（会话关闭）。

后果：
- 现状下 `failAll` 恰好充当了"断连即解锁"的角色，**掩盖了这个问题**；
- 一旦按 R7 去掉 `failAll` 的判失败行为，**挂起的提问将无人能解救**——用户按暂停键也解不开；
- 所以用户选的"用现有暂停/终止键取消"（R8）**必须配合新增一条唤醒路径**才能成立。

### GAP-3：`inputCh` 无缓冲，多端并发发送时顺序不可控

`web/session.go:93` 声明 `inputCh chan clientMessage`，`web/session.go:117` 以 `make(chan clientMessage)` 创建——**无缓冲**。
`web/session.go:159-161` 等处用 `select { case s.inputCh <- msg: ... }` 阻塞发送。

后果：
- 单客户端时只有一条发送方，行为确定；
- **多端共存后**，每个连接的读循环都是独立的发送方。当多个发送方同时就绪且接收方（`ReadLine`）准备接收时，Go 运行时对 `select` 的发送方选择是**伪随机**的；
- 这会导致 R5 要求的"按 seq 入队串行执行"在实现上**不可保证**；
- 必须改为**显式 FIFO 队列**（带 seq 与来源端标识），而不是依赖 channel 竞争。

---

## 5. 设计

### 5.1 核心思路

把 **ask / interaction 提升为事件队列里的一等公民**——与 LLM 输出块、工具调用事件同级，同样落进持久化事件流、同样带 seq、同样可被任意前端重放。

这样用户提出的四条目标语义**一次性同时成立**：
- 断线无关（agent 不依赖连接状态推进）；
- 重连弹框（未 resolved 的请求在事件流里可被回放发现）；
- 多端一致（所有端读同一份持久化队列）；
- 召回无差别（尾部重建与连续取走同一条路径）。

### 5.2 新增的两类持久化事件

| 事件类型 | 载荷 | 产生时机 | 用途 |
| --- | --- | --- | --- |
| `interaction_request` | `{id, kind, payload, source}` | agent 发起提问/确认时 | 前端据此渲染对话框；重连后据此恢复未答对话框 |
| `interaction_resolved` | `{id, answer, resolved_by}` | 任一端回答后 | 唤醒 agent；同时通知其余端关闭对应对话框 |

两个事件都经 `AppendEvent` 落库（复用 `store/eventstream.go` 的按 session 单调 seq 机制），因此天然满足 R2、R3、R4。

### 5.3 七点改造

| # | 文件 | 改造内容 |
| --- | --- | --- |
| 1 | `web/server.go` | `s.conn` 单槽位 → `conns []clientConn{id, *wsConn}` 列表；`sendJSON` 改为 fan-out 广播到所有浏览器端；**去掉 `setConn` 中踢掉旧连接的行为**；`gwConn` 保持独立槽位（R9） |
| 2 | `web/server.go` | 断连钩子只从列表摘除该连接，**不再调用 `failAll`** |
| 3 | `web/session.go` | `WebIO.ask` / `WebIO.Ask`：先落 `interaction_request` 事件，再 fan-out 投递；**无客户端时不报错**，永久阻塞等待（R7） |
| 4 | `web/session.go` | `resolve` / `resolveInteraction`：先落 `interaction_resolved`，再唤醒 agent，并广播给**其余端**（R4/R5） |
| 5 | `agent`（Interrupt 路径） | 新增"唤醒所有挂起交互"路径，向 LLM 回传"用户已取消"语义（GAP-2、R8） |
| 6 | `store/eventstream.go` | 新增 `LoadEventsAfter(sessionID, afterSeq, limit)`，支持**按游标向后增量取数**（现有 `LoadEvents` 只支持 `beforeSeq` 向前翻页） |
| 7 | `web/static/app.js` | 自动重连（指数退避 1s/2s/5s…上限 30s）；携带 `lastSeq` 增量取数；回放尾部若发现未 resolved 的 `interaction_request` 则弹出对话框；指令携带 `client_id`；来源端标识渲染进 YOU 块 |

### 5.4 明确不做的事（防止过度改造）

- **不动** `WebRenderer` 的"先落库再推送"机制（`web/session.go:1324-1354`）——它已经正确，是本设计的基础。
- **不动** `LoadEvents` 的向前翻页能力（FEATURE-507/508）——只新增向后增量接口，不替换。
- **不动** board / hub 协议本身（FEATURE-530）——只保证网关槽位与浏览器多端列表共存。
- **不动** REPL 主循环逻辑（`repl/repl.go`）——`WebSession.ReadLine` 本来就永久等待，天然满足 R1。
- **不动** `WebSession.ReadLine` 的 select 结构。

### 5.5 关于撤销 FEATURE-458 的说明

`web/static/app.js:457-458` 的"不自动重连"是 **FEATURE-458 的有意决策**，其唯一理由是"避免两个浏览器互抢单连接"（`avoids two browsers fighting over the single connection`）。

本设计实施多端共存后，该前提**彻底消失**：不再是"抢一条连接"，而是"多端各自持有连接、读同一份持久化队列"。
因此可以安全撤销该决策。**实施时必须在代码注释与 ROADMAP 决策记录中写明撤销理由**，避免后续维护者误以为是改错了老决策。

---
## 6. 风险（已在评审阶段向用户声明）

### 6.1 永久挂起 = session 永久 busy

用户选择 R7"不超时：永久挂起"。这意味着：

- 用户关掉浏览器且**再也不回来**时，该 turn 会一直挂着；
- 该 session 会**长期处于 busy 状态**（`agent.IsBusy()` 为 true，状态条呼吸灯、会话标题高亮等指示持续）；
- **只有暂停/终止键（interrupt）能解锁**（依赖 GAP-2 的修复）。

这是用户明确选择的语义（"永久挂起，直到重连或用户手动取消"），实施时**照此执行**，但必须记录在 ROADMAP 决策记录中，避免后续被当作 bug 处理。

### 6.2 多端并发指令的语义交织

R5 允许多端各自发指令，R6 又要求 agent 忙时走动态感知注入。两者的组合意味着：

- 指令**按 seq 串行入队**（GAP-3 修复后可保证顺序）；
- 但"两个端几乎同时各发一条"在语义上仍会**交织进同一段对话**，等价于"一个人快速连发两条"；
- 用户已确认**不做端间互斥**（不做主控端/旁观端区分），因此这是**预期行为**，不是缺陷。

### 6.3 潜在隐患（低概率，建议顺带加固）

`repl/repl.go:330-339` 中，`ReadLine` 返回任何非 nil error 都会 `break` 主循环（除 `interrupt` 特例会 `os.Exit(0)`）。
本设计不改变这条路径，但建议顺带加固：**web 模式下 `ReadLine` 的 `io.EOF` 不应导致 serve 进程退出**，以防未来误伤。

---

## 7. 验收标准

| # | 标准 | 验证方式 |
| --- | --- | --- |
| 1 | 断网 30 分钟后重连，agent 已跑完且事件完整，无丢失 | 断开网络 → 观察日志 agent 继续执行 → 恢复网络 → 界面补齐 |
| 2 | 断连时挂起的提问在重连后重新弹框，回答后 agent 从原处继续，无重复工具调用 | 在确认框弹出时断网 → 重连 → 对话框重现 → 回答 → 观察仅执行一次 |
| 3 | 两个浏览器 tab 同时连接均能收到完整事件流，互不顶替 | 打开两个 tab，确认后开的**不踢掉**先开的 |
| 4 | 任意端发指令按 seq 串行入队，事件记录来源端标识 | 两端同时发指令 → 检查事件流顺序与来源标记 |
| 5 | agent 忙时发指令走动态感知注入，与现有多端语义一致 | 在 agent 运行中从另一端发指令 → 检查 `AddDynamicEvent` 路径 |
| 6 | 暂停/终止键能解锁挂起的交互并回"用户已取消" | 确认框弹出时按暂停键 → 检查 agent 收到取消语义而非报错 |
| 7 | hub 链路（gwConn）与浏览器多端共存互不顶替，hub 断连不影响 agent 执行 | 断开 hub 连接 → agent 继续执行；公告板私信正常 |
| 8 | `LoadEventsAfter` 支持按游标增量取数 | 单元测试覆盖 |
| 9 | `go build ./... && go vet ./... && go test ./...` 全绿 | CI/本地验证 |

---

## 8. 执行序列

1. 合并 FIX-540（squash merge 回 main，打 tag `v0.68.2`）——已完成
2. 新建分支 **FEATURE-542**（编号接续；因同期插入 FIX-541，原拟编号 FEATURE-541 顺延），版本 `0.68.3 → 0.69.0`（minor +1）
3. 编写测试用例 `use-case/FEATURE-542/`，**经用户确认后**才进入编码
4. 按 §5.3 的七点实施；`go build ./... && go vet ./...` + `go test ./...`
5. 编译 co-shell / co-shell-hub（含 build 计数更新），提交当前分支，**等用户实测通过后**再合并

---

## 9. 追加设计：用户在线感知与在线/离线策略

> 追加时间：2026-10-06（FEATURE-542 评审后由用户追加；**本次先落设计，暂不编码**）
> 动机：FEATURE-542 解决了「断连后任务被污染」，但即使 ask 改为永久挂起（R7），离线期间 agent 仍会卡在等待中空转。若 agent 能提前知道「用户不在场」，就可选择「自行决策继续」而非「发起提问等待」，从源头降低阻塞概率。

### 9.1 感知形式：独立状态标签

在 `<environment_details>` 中新增**独立状态标签**（**不是**一次性动态事件——「在线」是持续状态）：

```xml
<user_presence>online</user_presence>
```

- 取值：`online` | `offline`
- 由 agent 构建每个 envelope 时**实时读取** provider，与 `<result_mode>` / `<no_tool_action>` 同属「每轮注入的运行时状态」
- 与 `<user_dynamic_events>` 的分工：后者报「用户做了什么」（事件，一次性消费），前者报「用户在不在」（状态，每轮刷新）

### 9.2 判定标准（本次实现范围）

| 场景 | 判定 |
| --- | --- |
| 浏览器直连（本次：`s.conn != nil`；多端改造后：`len(conns) > 0`） | `online` |
| 仅 hub 链路（`gwConn != nil`） | **不算在线**——hub 只是中转，其后方可能没有任何前端 |
| 无任何客户端 | `offline` |
| stdio / enhanced（REPL）模式 | `online`（用户就在终端前，无「客户端」概念） |

**遗留（明确不做）**：hub 后端的「有人订阅」需 hub↔agent 协议扩展（hub 上报当前订阅客户端数），留待多端改造（§9.4）一并处理。

### 9.3 系统提示词中的在线/离线策略

**位置**：
1. 【环境感知】节（`KeyCapEnvAwareness`）补一条「用户在场感知」——告知 `<user_presence>` 的含义与来源；
2. RULES 节补一条**策略规则**（总注入，不受开关影响）——给出两档差异化行为。

**策略**：

| 状态 | 行为 |
| --- | --- |
| **online** | 正常交互：需要澄清/确认时照常调用 `ask_user`、发起交互；按常规节奏等待用户反馈 |
| **offline** | **避免发起需要人工输入的阻塞调用**（`ask_user`、需人工确认的工具调用、结构化交互）：<br>① 能做出合理默认决策的 → 直接决策并继续；<br>② 无法决策但可绕过的 → 跳过该分支、继续其他可做部分；<br>③ 把「需人工确认项」累计到最终结果中**一次性汇总**，等用户回来处理 |

**安全边界（不可放宽）**：offline 策略只改变「是否发起阻塞式交互」，**不改变任务目标与安全边界**——高风险操作（删除/覆盖/越界写入/涉及凭据）在 offline 下**不得自行批准**，应记录待确认或改走更保守的可逆路径。

### 9.4 与 FEATURE-542 主体改造的接口

| 项 | 本次设计 | 多端改造后 |
| --- | --- | --- |
| 在线判定源 | `s.conn != nil` | `len(conns) > 0`（任意一端在线即 online） |
| hub 侧 | 不处理（gwConn 不计入） | hub 上报订阅客户端数（协议扩展），agent 据此判定 |
| 与 R7（永久挂起）的关系 | 互补：ask 不再被判死，但 LLM 已倾向不发起 | 同 |
| 与自动重连的关系 | 无 | 重连即回到 online，下一轮 envelope 立即反映 |
| stdio / REPL | `online` | `online` |

### 9.5 实施要点（供后续编码参考，本次不编码）

| # | 文件 | 改造内容 |
| --- | --- | --- |
| 1 | `agent`（loop.go / agent.go） | 新增 `userPresenceProvider func() bool` 字段 + `SetUserPresenceProvider(fn)`；`buildFullEnvironmentDetails` 注入 `<user_presence>online|offline</user_presence>`（provider 为 nil 时：stdio/REPL → online，serve 且未注册 → offline） |
| 2 | `web/server.go` | 新增 `HasClient() bool`（`s.conn != nil`；多端改造后改 `len(conns) > 0`） |
| 3 | `web/session.go` | 注册 `srv.SetUserPresenceProvider(sess.srv.HasClient)` → agent（与 `SetBusyProvider` 同构） |
| 4 | `agent/env_prune.go` | `user_presence` 加入 `envLiteTags`（它决定行为策略，轻量 envelope 也必须携带） |
| 5 | `i18n`（zh/en） | `KeyCapEnvAwareness` 补「用户在场感知」条目；RULES 节补在线/离线策略规则 |
| 6 | 测试 | ① provider nil → stdio/REPL 视为 online；② provider=false → 标签为 offline；③ 标签随 provider 实时变化 |

### 9.6 验收标准（补充）

| # | 标准 | 验证方式 |
| --- | --- | --- |
| S1 | serve 模式下无浏览器连接时，envelope 出现 `<user_presence>offline</user_presence>` | 断开浏览器后查看 envelope |
| S2 | 浏览器连上后，下一轮 envelope 变为 `online` | 重连并观察 |
| S3 | offline 时 LLM 不再调用 `ask_user`，改为自主决策并在结果中汇总待确认项 | 断连后下发需澄清的任务，观察是否阻塞 |
| S4 | stdio / REPL 模式恒为 `online` | 终端运行观察 |
| S5 | 高风险操作在 offline 下不被自行批准 | 构造删除类任务，观察是否记录而非执行 |

---

## 附录 A：hub 共享配置目录创建行为调查（同时期插入调查）

**问题**：若 `~/.co-shell/` 不存在，在 hub 配置 agent 时勾选"共享配置"，会自动生成该文件夹吗？

**结论：不会。** 既不会创建 `~/.co-shell/` 目录，也不会生成 `~/.co-shell/config.json`。

证据链：

| # | 位置 | 事实 |
| --- | --- | --- |
| 1 | `hub/gateway/manager.go:249` | 只执行 `os.MkdirAll(workspace, 0755)`——只建 workspace 本身 |
| 2 | `hub/gateway/manager.go:268-278` | 注释明确 `Shared config needs no file`；`if !useSharedConfig { ... }` 整段被跳过，仅非共享时才写 `{workspace}/config.json` |
| 3 | `hub/gateway/manager.go:443-448` | `buildArgs` 只把 `-c ~/.co-shell/config.json` 拼进命令行，**不碰文件系统** |
| 4 | `config/config.go:1158-1167` | `LoadFromFile` 遇 `os.IsNotExist` 时**静默返回 `DefaultConfig()`**，不建目录、不写文件，且返回的 path 为空串 |
| 5 | 全仓库 `MkdirAll` 审计 | 唯一涉及 `~/.co-shell` 的是 `hub/gateway/settings.go:96`，那是 hub gateway **自身**的自签 TLS 证书目录（`settings.go:85` 注释），与 agent 共享配置无关 |

**实际表现 = 静默降级**：agent 正常启动，但使用 co-shell 内置默认配置（无模型等），且**没有任何提示**——因为 `configPath` 返回空串，`main.go:621` 的 `log.Info("Config loaded from: ...")` 都不打印。用户只会觉得"agent 起来了但配置是空的"。

## 附录 B：配置保存路径回退缺陷（已转 FIX-541 处理）

调查附录 A 时顺带发现的问题：

`LoadFromFile` 走 `IsNotExist` 分支时**没有设置 `cfg.configPath`**（对比：只有成功解析时才会在 `config/config.go:1192` 赋值 `cfg.configPath = path`）。

而 `Config.Save()`（`config/config.go:1221-1238`）：
```go
path := c.configPath
if path == "" {
    if c.ws == nil { return fmt.Errorf("workspace not set, cannot save config") }
    path = c.ws.ConfigPath()        // ← 回退到 {workspace}/config.json
}
...
if err := os.WriteFile(path, data, 0644); err != nil {   // ← 没有 MkdirAll
    return fmt.Errorf("cannot write config: %w", err)
}
```

推论：以 `-c ~/.co-shell/config.json` 启动而该文件不存在时，用户在 UI/REPL 中修改配置 → 配置会写入 **`{workspace}/config.json`** 而非共享路径；下次重启仍读 `~/.co-shell/config.json`（仍不存在）→ 又回默认配置，表现为"配置保存了却不生效"。

另：`Save()` 直接 `WriteFile` 而无 `MkdirAll`，若 path 确实指向 `~/.co-shell/config.json` 且目录不存在，会以 ENOENT 失败。目前因为回退到 workspace（hub 已建该目录）而侥幸不报错——这侧面说明共享配置路径下的保存链路**从未被真正走通过**。

`cfg.Save()` 在 `cmd/settings_display.go`、`cmd/config.go` 等有 20 余处调用点，均复用同一 cfg 实例，推断成立度较高，但**未实测验证**。

**用户决策**：另开 **FIX-541** 修复——"在 co-shell 保存配置时，如果保存路径中的文件夹或配置文件不存在，则自动重建文件夹和文件"。

---

## 附录 C：关键代码位置速查

| 事项 | 位置 |
| --- | --- |
| WS 写超时常量 | `web/ws.go:42` |
| WS 异步发送队列与 writer goroutine | `web/ws.go:44-49`、`web/ws.go:152-165` |
| WS 连接生命周期/读循环 | `web/server.go:647-673` |
| 断连钩子注册 | `web/session.go:141` |
| `clearConn` / `clearGatewayConn` | `web/server.go:521-530` / `web/server.go:517-519` |
| `WebIO.failAll` | `web/session.go:1269-1277` |
| `WebIO.ask` / `WebIO.Ask` | `web/session.go:1221-1245` / `web/session.go:1190-1218` |
| `WebSession.ReadLine` | `web/session.go:1058-1081` |
| `WebRenderer.Render`（先落库再推送） | `web/session.go:1324-1354` |
| REPL 主循环 ReadLine 错误分支 | `repl/repl.go:329-339` |
| 事件持久化（seq 键、按 session 隔离） | `store/eventstream.go:50-60`、`store/eventstream.go:73-118` |
| `LoadEvents`（向前翻页） | `store/eventstream.go:153-207` |
| 前端不自动重连（FEATURE-458） | `web/static/app.js:457-458` |
| 前端历史回放 | `web/static/app.js:477-490`、`web/static/app.js:704-720` |
| 动态感知注入 | `agent/dynamic_events.go:189`（`AddDynamicEvent`）、`agent/dynamic_events.go:211`（`consumeDynamicEvents`） |
| interact 消息处理 | `web/session.go:226-229` |
| hub 共享配置判定与建目录 | `hub/gateway/manager.go:249`、`hub/gateway/manager.go:268-278`、`hub/gateway/manager.go:443-448` |
| 配置加载与保存 | `config/config.go:1158-1194`、`config/config.go:1221-1239` |
