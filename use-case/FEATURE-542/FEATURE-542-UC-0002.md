# FEATURE-542 验收测试用例（UC-0002）：用户在线感知与在线/离线策略

> 权威依据：`docs/web-session-resilience.md` **§9（含 §9.5 实施要点、§9.6 验收标准 S1~S5）**。分支 **FEATURE-542**，版本 **v0.99.0**。
> 会话韧性主体（R1~R9、GAP-1/2/3、§5.6、§7）见同目录 **FEATURE-542-UC-0001.md**。

> 动机（§9 引言）：FEATURE-542 解决了「断连后任务被污染」，但即使 ask 改为永久挂起（R7），离线期间 agent 仍会卡在等待中空转。若 agent 能提前知道「用户不在场」，就可选择「自行决策继续」而非「发起提问等待」，从源头降低阻塞概率。

> 已确认决策（不可改动）：
> ① 在线呈现＝**独立状态标签每轮注入**（`<user_presence>online|offline</user_presence>`，是持续状态而非一次性动态事件，§9.1）；
> ② 判定口径（**本次实现范围**）：**仅「浏览器直连客户端」算在线**——`s.conn != nil`（多端改造后 `len(conns) > 0`）；**hub 链路 `gwConn` 不算在线**；`stdio`/`enhanced(REPL)` 恒 `online`；无任何客户端 → `offline`（§9.2）；
> ③ 离线策略＝**避免 `ask_user`、自主决策继续、汇总「需人工确认项」**（§9.3）；
> ④ **安全边界不放宽**：offline 下高风险操作（删除/覆盖/越界写入/涉及凭据）**不得自行批准**（§9.3）；
> ⑤ hub 上报订阅客户端数**留待多端改造**，本次不做（§9.2 遗留 / §9.4）。

> 改动范围（预期实现，§9.5）：`agent`（`userPresenceProvider` + `SetUserPresenceProvider`；`buildFullEnvironmentDetails` 注入 `<user_presence>`）、`web/server.go`（`HasClient()`）、`web/session.go`（注册 provider）、`agent/env_prune.go`（`user_presence` ∈ `envLiteTags`）、`i18n` zh/en（`KeyCapEnvAwareness` 补条目 + RULES 补策略规则）。

## 用例清单

| 编号 | 名称 | 类型 | 对应设计条目 |
|------|------|------|--------------|
| A-001 | provider 为 nil 且 stdio/REPL 模式 → `online` | 单测 | §9.5#1、§9.2、S4 |
| A-002 | provider 为 nil 且 serve 模式未注册 → `offline` | 单测 | §9.5#1 |
| A-003 | provider 返回 false → 标签为 `offline` | 单测 | §9.5#6②、S1 |
| A-004 | provider 返回 true → 标签为 `online` | 单测 | §9.5#6②、S2 |
| A-005 | 标签随 provider 实时变化（同一会话内 online↔offline） | 单测 | §9.5#6③、S2 |
| A-006 | `user_presence` ∈ `envLiteTags`：L1/L2 轻量 envelope 也携带该标签 | 单测 | §9.5#4 |
| A-007 | `HasClient()` 与在线判定联动：仅 gwConn → false；有浏览器 → true | 单测 | §9.2、§9.5#2/#3 |
| A-008 | 标签位于 `<environment_details>` 内，与既有标签同节且格式正确 | 单测 | §9.1 |
| A-009 | i18n：`KeyCapEnvAwareness` zh/en 均含「用户在场感知」条目，键集合一致 | 单测 | §9.5#5、§9.3 位置 1 |
| A-010 | i18n：RULES 节含在线/离线策略规则（两档行为 + 安全边界），zh/en 齐备 | 单测 | §9.5#5、§9.3 位置 2 |
| A-011 | `SetUserPresenceProvider` 注册/替换/置 nil 幂等，不 panic、无数据竞争 | 单测 | §9.5#1 |
| B-001 | S1：serve 模式无浏览器连接时，envelope 出现 `<user_presence>offline</user_presence>` | 运行时 | S1 |
| B-002 | S2：浏览器连上后，下一轮 envelope 变为 `online` | 运行时 | S2 |
| B-003 | S3：offline 时 LLM 不再调用 `ask_user`，改为自主决策并在结果中汇总待确认项 | 运行时 | S3、§9.3 |
| B-004 | S4：stdio / REPL（enhanced）模式恒为 `online` | 运行时 | S4、§9.2 |
| B-005 | S5：高风险操作在 offline 下不被自行批准 | 运行时 | S5、§9.3 安全边界 |
| B-006 | 重连即回到 online，下一轮 envelope 立即反映 | 运行时 | §9.4 |
| B-007 | hub 链路存在但无浏览器连接 → 仍判 `offline`（gwConn 不计入） | 运行时 | §9.2 |
| B-008 | 在线/离线切换只影响交互策略，不改变任务目标与安全边界（回归） | 运行时 | §9.3、§9.4 |

---

## A 组：单元测试（Go）

- **A-001**：构造 agent，`userPresenceProvider` 保持 nil，运行模式为 `stdio`（及 `enhanced` REPL）→ 调 `buildFullEnvironmentDetails(...)` → 断言输出含 `<user_presence>online</user_presence>`。
- **A-002**：provider 为 nil，运行模式为 `serve` → 断言输出为 `<user_presence>offline</user_presence>`（未注册 provider 不得默认 online，避免 serve 下误判用户在场）。
- **A-003**：`SetUserPresenceProvider(func() bool { return false })` → envelope 含 `offline`。
- **A-004**：`SetUserPresenceProvider(func() bool { return true })` → envelope 含 `online`。
- **A-005**：用一个可变开关的 provider（初值 false → 生成 envelope 记 offline；改为 true → 再生成 envelope 记 online）→ 断言**每轮实时读取**，非一次性快照、非缓存。
- **A-006**：分别以 L0（完整）/L1（lite，`envLiteTags`）/L2（minimal，`envMinimalTags`）三档 envelope 生成策略产出文本 → 断言三档**均含** `user_presence` 标签（因其决定行为策略，轻量 envelope 也必须携带）；对照既有 `env_prune_test.go` 的裁剪断言风格。
- **A-007**：`Server.HasClient()`：无连接 false；仅 `gwConn`（gateway 分支接入）存在 → **false**；浏览器连接接入 → true；摘除后 → false。
- **A-008**：断言 `<user_presence>` 出现在 `<environment_details>` 块内、作为独立标签而非嵌在其它标签里；取值只能是 `online` 或 `offline`（无第三态、无空值）。
- **A-009**：读取 zh/en 的 `KeyCapEnvAwareness` 文案 → 断言均含「用户在场感知 / user presence」条目，且说明「`<user_presence>` 含义与来源」；zh/en 结构条目数一致（键集合一致）。
- **A-010**：读取 zh/en 系统提示词的 RULES 节 → 断言含在线/离线两档差异化行为（online 正常交互；offline 避免阻塞式交互、自主决策、汇总待确认项）与**安全边界不放宽**条款（高风险操作 offline 不得自行批准）；zh/en 齐备。
- **A-011**：连续调用 `SetUserPresenceProvider(fn1)` → `SetUserPresenceProvider(fn2)` → `SetUserPresenceProvider(nil)` → 断言无 panic、后续 envelope 生成不异常；并发场景（读写 provider 同时进行）下 `-race` 无告警。

---

## B 组：运行时验收（人工/端到端）

- **B-001（S1）**：`--serve` 启动，**不打开任何浏览器**（确保无浏览器直连）→ 下发一条指令 → 查看发给 LLM 的 envelope（日志或调试输出）→ 断言含 `<user_presence>offline</user_presence>`。
- **B-002（S2）**：承接 B-001，打开浏览器连接 → 下发下一条指令 → 断言**下一轮** envelope 已变为 `online`（无需重启、无需重新注册 provider）。
- **B-003（S3、§9.3）**：保持 offline → 下发一条**需要澄清**的任务（如信息不完整的需求）→ 断言：agent **不调用 `ask_user`**、不阻塞；自主选择合理默认决策继续执行；在最终结果中**一次性汇总**「需人工确认项」。对照 online 时同样任务会正常发起提问。
- **B-004（S4）**：终端 stdio 模式（及 `enhanced` REPL）运行 → 断言 envelope 恒为 `online`（无「客户端」概念；断开浏览器不影响）。
- **B-005（S5、安全边界）**：offline 状态下构造**高风险操作**任务（如删除文件/覆盖既有文件/越界写入/涉及凭据）→ 断言 agent **不自行批准执行**：应记录为待确认项或改走更保守的可逆路径；**不得**出现实际删除/覆盖的副作用。
- **B-006（§9.4）**：断开浏览器（offline）→ 重连 → 断言下一轮 envelope 立即回到 `online`；与自动重连（UC-0001 A-021/B-013）无冲突、无额外延迟要求。
- **B-007（§9.2）**：在**仅 hub 链路**（`gwConn != nil`）而无浏览器直连的情况下运行 → 断言仍判 `offline`（hub 只是中转，其后方可能没有任何前端；本次不做 hub 上报订阅数）。
- **B-008（回归）**：分别在 online / offline 下运行同一任务的**非交互部分** → 断言任务目标、工具可用范围、安全边界完全一致；差异**仅**在于是否发起阻塞式交互与是否汇总待确认项。

---

## 覆盖矩阵（§9 条目）

| 设计条目 | 覆盖用例 |
| --- | --- |
| §9.1 感知形式：独立状态标签每轮注入 | A-001~A-005、A-008 |
| §9.2 判定标准（浏览器直连=online；gwConn 不算；stdio/REPL 恒 online；无客户端=offline） | A-001、A-002、A-007、B-004、B-007 |
| §9.3 策略（online 正常交互 / offline 避免阻塞+自主决策+汇总待确认） | A-010、B-003、B-008 |
| §9.3 安全边界不放宽（高风险 offline 不得自行批准） | A-010、B-005 |
| §9.4 与主体改造接口（`HasClient`、重连回 online、与 R7 互补） | A-007、B-002、B-006 |
| §9.5#1 provider + 注入（nil 判定规则） | A-001~A-005、A-011 |
| §9.5#2/#3 `HasClient()` 与注册 | A-007、B-001、B-002 |
| §9.5#4 `envLiteTags` 含 `user_presence` | A-006 |
| §9.5#5 i18n（环境感知条目 + RULES 策略规则） | A-009、A-010 |
| §9.5#6 测试要点（① nil→stdio/REPL online ② provider=false→offline ③ 实时变化） | A-001、A-003、A-005 |
| §9.6 S1 无浏览器连接 → offline | A-003、B-001 |
| §9.6 S2 连上后下一轮 → online | A-004、A-005、B-002、B-006 |
| §9.6 S3 offline 不调用 `ask_user`，自主决策并汇总 | A-010、B-003 |
| §9.6 S4 stdio/REPL 恒 online | A-001、B-004 |
| §9.6 S5 高风险操作 offline 不被自行批准 | A-010、B-005 |
| §9.2 遗留（hub 上报订阅数本次不做） | B-007 |
