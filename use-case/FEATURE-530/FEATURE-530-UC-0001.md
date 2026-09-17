# FEATURE-530 公告板 v2（消息模型）— 测试用例

> 分支：FEATURE-530
> 版本：v0.63.0
> 设计：`research/agent-board-design-v2.md`
> 范围：hub 消息板（注册/roster/消息/三态/超时/会話/bbolt/ Chat 栏）+ co-shell 侧注册、`board_send`、忙闲分派注入；删除旧工单模型

## 用例清单（概览）

| 组 | 用例 | 覆盖点 |
|---|---|---|
| A 注册与 roster | UC-0001 ~ UC-0006 | 自动注册、职责描述优先级、roster 广播与注入、掉线 |
| B 发送与投递 | UC-0007 ~ UC-0013 | 广播、定向、排除自己、多收件人、回执内容、参数校验 |
| C 三态与超时 | UC-0014 ~ UC-0019 | handled/declined/need_info、no_reply、未响应事件 |
| D 注入行为 | UC-0020 ~ UC-0024 | 空闲开回合、忙碌动态感知、合并、去重 |
| E 持久化与 UI | UC-0025 ~ UC-0029 | bbolt 重启恢复、Chat 栏会話聚合与徽标、改名 |
| F 清理与回归 | UC-0030 ~ UC-0033 | 旧工具删除、旧报文失效、开关默认关闭、编译与测试 |

---

## A 注册与 roster

### UC-0001 连接后自动注册（非 LLM）
- **前置**：co-shell 实例 `board_enabled=true`，hub 已配置该实例且未连接。
- **步骤**：hub 启动并连入该实例；观察 hub 日志与注册表。
- **预期**：WS 连接建立后**无需任何 LLM 回合**，hub 立即收到 `board_register`，注册表中出现该 agent（id/name/duty/online=true），并回 `board_registered`。

### UC-0002 职责描述来源：agent 上报
- **前置**：该实例 hub 侧未手工录入"职责描述"；实例的 `CAPABILITIES.md`/身份与个性有内容。
- **步骤**：连接后查看 hub 侧该 agent 的 `duty` 与 `duty_source`。
- **预期**：`duty` = agent 上报值（CAPABILITIES.md 优先，空则 AgentDescription），截断 ≤200 字；`duty_source="agent"`。

### UC-0003 职责描述来源：hub 手工录入优先
- **前置**：hub 配置该 agent 的"职责描述"填写为"负责财务对账"。
- **步骤**：连接后查看 `duty` 与 `duty_source`。
- **预期**：`duty="负责对账..."`（hub 值），`duty_source="hub"`；agent 上报值被覆盖。

### UC-0004 注册变化触发 roster 广播
- **前置**：A 已注册，B 未启动。
- **步骤**：启动 B（board_enabled=true）；观察 A 收到的 `board_roster`。
- **预期**：hub `roster_rev` 递增，向**所有**已连接 agent 广播全量 roster（含 A、B 两条），A 侧缓存更新为新 rev。

### UC-0005 roster 注入 environment_details
- **前置**：A、B 均已注册，A 侧 roster 已更新。
- **步骤**：在 A 上发起任意一个需要 LLM 的回合，查看注入的上下文。
- **预期**：`<environment_details>` 中出现 `<board_agents rev="N">`，包含 B 的 `id/name/职责描述`；roster 为空时该块整体不出现（不产生空标签噪声）。

### UC-0006 agent 掉线时 roster 缩减
- **前置**：A、B 均已注册。
- **步骤**：停止 B（或断开其连接）；观察 A 侧 roster。
- **预期**：hub 将 B 标记 `online=false`（或从清单移除）并广播新 rev；A 侧 `<board_agents>` 随之更新。

---

## B 发送与投递

### UC-0007 广播消息（`to=*`）排除自己
- **前置**：A、B、C 三实例已注册。
- **步骤**：A 调用 `board_send(to="*", content="...", timeout_seconds=120)`。
- **预期**：B、C 各收到一条 `board_message`（from=A）；**A 自己不收到**；hub 立即回 `board_sent`。

### UC-0008 定向私信（单收件人）
- **前置**：A、B 已注册。
- **步骤**：A 调用 `board_send(to="agent-b", ...)`。
- **预期**：仅 B 收到；会話 key = `agent-a+agent-b`（参与者排序后 `+` 连接）；C 不收到。

### UC-0009 多收件人投递
- **前置**：A、B、C 已注册。
- **步骤**：A 调用 `board_send(to="agent-b,agent-c", ...)`。
- **预期**：B、C 均收到；若 C 掉线，`board_sent.deliveries` 中 C 项 `ok=false` 并带原因，B 项 `ok=true`。

### UC-0010 发送立即回执（异步，不等待处理）
- **前置**：B 处于忙碌状态（正在执行任务）。
- **步骤**：A 发送 `to=agent-b` 的消息，测量工具返回耗时。
- **预期**：工具在**秒级**返回，含 `message_id=m-N`、`conversation`、`deliveries`、等待窗口提示；**不等待** B 处理完成。

### UC-0011 `timeout_seconds` 必填
- **前置**：A 已注册且工具可用。
- **步骤**：调用 `board_send` 但**不带** `timeout_seconds`。
- **预期**：工具返回错误，明确提示 `timeout_seconds is required`；hub 未创建任何消息。

### UC-0012 `timeout_seconds` 范围校验
- **步骤**：分别用 `0`、`4`、`86401` 调用 `board_send`。
- **预期**：三种情况均报错并回显合法区间 `5..86400`；用 `5` 与 `86400` 成功。

### UC-0013 回复参数校验（`state`）
- **前置**：B 收到 A 的消息 `m-12`。
- **步骤**：B 先调用 `board_send(reply_to="m-12", content="...")`（不带 state）→ 再带 `state="whatever"` → 最后带 `state="handled"`。
- **预期**：前两次报错（回复必须带三态且取值合法）；第三次成功，hub 将该回复归入原会話并投递给 A。

---

## C 三态与超时

### UC-0014 `handled`：受理
- **前置**：B 收到 A 的 `m-12`，内容落在 B 的职责描述范围内。
- **步骤**：B 调用 `board_send(to="agent-a", reply_to="m-12", state="handled", content="我来处理", timeout_seconds=600)`。
- **预期**：hub 记录 B 已受理并终止该收件人的未响应计时；A 收到该回复（动态感知/新回合）；Chat 栏该消息显示"已受理"徽标。

### UC-0015 `declined`：不受理
- **前置**：同 UC-0014，但内容与 B 职责无关。
- **步骤**：B 回 `state="declined"`。
- **预期**：A 收到"已拒绝"；A 可据此继续等其他 agent 或自行处理；不会产生"未响应"事件。

### UC-0016 `need_info`：需要补充信息
- **步骤**：B 回 `state="need_info", content="请提供范围与验收标准"`。
- **预期**：A 收到"待补充"；不计入未响应；A 补充后 B 再回 `handled` 或 `declined`，链条仍在同一会話（`reply_to` 指向原消息）。

### UC-0017 结局回复（`handled` + 结果）
- **步骤**：B 完成任务后调用 `board_send(reply_to="m-12", state="handled", content="已完成：…")`。
- **预期**：A 收到结果；会話内两条 B 的回复都归属 `m-12`；Chat 栏显示进展与结果。

### UC-0018 `no_reply=true`：不应答、不计时
- **步骤**：A 调用 `board_send(to="*", content="知会：今晚发布", no_reply=true, timeout_seconds=60)`。
- **预期**：B、C 收到消息；hub **不为**其建未响应计时；B、C 侧提示词约定不应答；60s 后 A **不**收到未响应事件。

### UC-0019 超时生成"未响应"事件
- **前置**：B 在线但不回应。
- **步骤**：A 广播 `timeout_seconds=120`；等待 120s（扫描 tick 5s，允许 ±10s 误差）。
- **预期**：A 收到 `board_timeout` 事件，含 `message_id`、未响应名单（B、C…）、deadline；Chat 栏该消息徽标变"未响应"。

---

## D 注入行为

### UC-0020 空闲时收到消息 → 开新回合处理
- **前置**：B 空闲（无进行中回合）。
- **步骤**：A 发送定向消息给 B。
- **预期**：B 在聚合窗口（约 400ms）后以一条带标记的 input 开启新回合；注入文本包含 `[board-message <id> from <A>]` 与正文；B 的 LLM 能看到并据此决策。

### UC-0021 忙碌时收到消息 → 注入动态感知，不打断
- **前置**：B 正在执行长任务（`IsBusy()=true`）。
- **步骤**：A 发送消息给 B。
- **预期**：B **不中断**当前回合、不新开回合；消息进入动态感知队列，B 本轮下一次注入时可在 `<user_dynamic_events>` 中看到 `<board_message>`；当前回合结束后不重复触发。

### UC-0022 多条消息合并为一条注入
- **前置**：B 空闲。
- **步骤**：在 < 400ms 内连续投递 3 条消息（m-21/m-22/m-23）给 B。
- **预期**：B 只开启**一个**回合，且该回合的注入中包含 3 条消息（合并在同一块），而非 3 个回合。

### UC-0023 同一 message_id 不重复注入
- **前置**：B 已注入过 `m-30`。
- **步骤**：hub 重发同一 `m-30`（模拟重连/重投）；隔一段时间再发一次。
- **预期**：B 侧去重表命中 → 不再注入（既不进 inputCh 也不进动态事件）；LLM 上下文中 `m-30` 只出现一次。

### UC-0024 去重表容量滚动
- **步骤**：连续注入超过去重表容量（默认 1000 条）的消息，再重发最早那条。
- **预期**：超出容量的最早条目被淘汰后可再次注入（行为符合环形缓冲语义，不报错、不崩溃）。

---

## E 持久化与 UI

### UC-0025 消息与会話持久化（bbolt）
- **步骤**：A、B 完成若干消息与三态往来；停止 hub；重启 hub。
- **预期**：hub 重启后 `GET /api/board/conversations` 仍返回相同会話列表与消息，`message_id` 序号不回退（继续递增），`roster_rev` 连续。

### UC-0026 重启后未响应判定仍正确
- **前置**：存在未到期（或未到期前的）`Awaiting` 记录。
- **步骤**：重启 hub；等待到 deadline。
- **预期**：仍能正确判超时并发出 `board_timeout` 事件（不因重启丢失或重复）。

### UC-0027 Chat 栏会话聚合与公告会話唯一
- **步骤**：产生广播消息 + 两两定向消息；打开 hub Web UI 的 Chat 栏。
- **预期**：会話列表包含**唯一**的公告会話（参与者显示"全部"，固定置顶）与按参与者命名的定向会話（如 `agent-a+agent-b`）；同参与者集合的多条消息归入同一会話。

### UC-0028 Chat 栏消息流与徽标
- **步骤**：查看某个会話的消息流。
- **预期**：每条显示 from/to/content/时间；回复按 `reply_to` 缩进或关联；三态徽标（已受理/已拒绝/待补充）与"未响应"正确；等待窗口剩余时间显示正确。

### UC-0029 hub 配置项改名
- **步骤**：打开 hub 新建本地 agent / 添加外部 agent / 编辑抽屉三个表单。
- **预期**：原"备注"标签均显示为"职责描述"；提交与回显正常；`hub-agents.json` 字段名仍为 `name`（未变）。

---

## F 清理与回归

### UC-0030 旧工具彻底删除
- **步骤**：开启 `board_enabled`，查看 LLM 可用工具列表。
- **预期**：只存在 `board_send`（及可选的只读查询工具）；`board_post`/`board_list`/`board_claim`/`board_dm`/`board_confirm`/`board_result` **全部不存在**。

### UC-0031 旧报文不再被识别
- **步骤**：向 hub 发送 `board_post` / `board_claim` / `board_confirm` / `board_result`。
- **预期**：hub 不再按旧协议处理（无 Request/Task 产生）；按新协议应报错或忽略，且不导致 panic。

### UC-0032 开关默认关闭
- **前置**：默认配置（未设置 `board_enabled`）。
- **步骤**：启动实例并接入 hub。
- **预期**：不发生注册、不出现 `<board_agents>`、不注册 `board_send`；hub 下发 board 消息时 co-shell 忽略且无副作用。

### UC-0033 编译、静态检查与单测
- **步骤**：`go build ./... && go vet ./...`；`go test ./...`。
- **预期**：全部通过；新增单测覆盖三态校验、去重注入、忙闲分派、合并窗口、超时事件、会話 key 生成、bbolt 读写；既有失败与本任务无关（基线一致）。

---

## 运行时验证方式建议

| 组 | 验证手段 |
|---|---|
| A / B / C | 独立 hub 实例 + 两个 co-shell 实例（独立 workspace/端口/registry），用 TCP 驱动脚本注入 `board_*` 报文与观察回执；co-shell 侧用日志与注入内容断言 |
| D | co-shell 侧单测（注入分派、聚合窗口、去重表）+ 真机观察日志时序 |
| E | hub 重启前后对比 API 返回；浏览器实测 Chat 栏（`browser_*` 工具） |
| F | 工具列表快照 + `go build/vet/test` |
