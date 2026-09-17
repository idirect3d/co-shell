# 公告板 v2 设计（消息模型）— FEATURE-530

> 状态：设计已冻结（用户逐条确认）
> 日期：2026-09-16
> 分支：FEATURE-530　版本：v0.63.0
> 取代：`research/agent-board-design.md`（FEATURE-490 工单型设计，已废弃）

## 1. 目标

让接入同一 hub 的多个 co-shell 实例像"同事"一样互相通信：

1. hub 提供公告板（消息中枢 + 注册表）；
2. co-shell 启动/启用公告板时**自动注册**（非 LLM 调用），注册信息为**名称 + 职责描述**；
3. 注册信息变化时 hub 广播当前 agent 清单，co-shell 注入 `environment_details`，让 LLM 知道有哪些可用 agent；
4. agent 通过**唯一工具** `board_send` 发消息：hub 广播/转发；收件方**空闲则用当前会话处理**（开新回合），**忙碌则注入动态感知**；回复必须带"是否处理"标记（三态），让提问方知道是否由该 agent 处理；
5. 支持定向私信（收件人可为多个或 `*`）；
6. hub 分配消息 ID、异步转发、**立即回执**（不等收件人处理完）；
7. hub Web UI 的 **Chat 栏**按参与者聚合展示会話；公告会話全局唯一；
8. 消息与会話**持久化到 bbolt**。

## 2. 决策汇总（用户确认）

| # | 决策 |
|---|---|
| 1 | hub 主动拨号连 co-shell（co-shell 无 hub 地址）；co-shell 改设置需重启，注册只在连接建立后自动上行一次 |
| 2 | 回复三态：`handled` / `declined` / `need_info`；`no_reply` 仅用于发送侧声明"本条无需回复" |
| 3 | `to=["*"]` 广播**排除发送者自己** |
| 4 | 要求回复的消息超时 → hub 生成"未响应"系统事件给发送方（超时时长由发送方指定） |
| 5 | 忙碌时多条消息**合并成一条**注入 |
| 6 | roster **每轮**注入；同一 `message_id` **绝不重复注入** |
| 7 | 旧 6 个 board 工具与工单状态机**直接删除**（不留兼容层） |
| 8 | "能力"概念改为**职责描述**；hub 配置 agent 的录入项"备注"改名"职责描述"（只改显示标签，不动字段名） |
| 9 | 职责描述优先级：**hub 手工录入 > agent 上报**，生效来源写入 `duty_source` |
| 10 | `timeout_seconds` **一律必填**（含 `no_reply=true`；后者 hub 忽略超时计时），范围 5..86400 |
| 11 | 工具等待 hub 发送回执的超时固定默认 5s，并作为**系统参数**暴露（`:set` + Web 设置面板） |
| 12 | 消息与会話持久化到 bbolt |

## 3. 概念模型与数据结构

```
Message        一条消息（含回复）；hub 分配 ID；带 reply_to 链
Conversation   参与者集合 → 一个会話；公告会話 key = "*" 全局唯一
AgentInfo      注册表项：id / name / duty / duty_source / online / last_seen
```

不再有 Request / Task / 认领 / 确认 / 结果状态机。

```go
// Message is one board message.
type Message struct {
    ID             string   `json:"id"`              // "m-<n>"，hub 分配
    ReplyTo        string   `json:"reply_to"`        // "" = 新话题；"0" 等价于空
    From           string   `json:"from"`            // agent id，取自连接身份
    To             []string `json:"to"`              // ["*"] = 广播
    Content        string   `json:"content"`
    NoReply        bool     `json:"no_reply"`        // 发送侧：本条无需回复
    State          string   `json:"state"`           // 回复侧：handled|declined|need_info
    TimeoutSeconds int      `json:"timeout_seconds"` // 发送侧指定；no_reply=true 时仍必填但 hub 不计时
    Conversation   string   `json:"conversation"`    // "*" 或 "a+b"
    TS             int64    `json:"ts"`
}

// AgentInfo is one registered agent.
type AgentInfo struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    Duty       string `json:"duty"`        // 职责描述
    DutySource string `json:"duty_source"` // "hub" | "agent"
    Online     bool   `json:"online"`
    LastSeen   int64  `json:"last_seen"`
}

// Awaiting tracks one pending "reply expected" obligation.
type Awaiting struct {
    MessageID string
    AgentID   string
    Deadline  int64
    Status    string // pending | answered | timeout
}
```

**会話 key 规则**：广播 → `*`；定向 → 参与者 id 排序后以 `+` 连接。
**回复继承会話**：回复消息的 `conversation` 取自被回复消息，不重新推导（避免协商中参与者变化导致会話分裂）。

## 4. 线协议（复用现有 WS，不新增通道）

| 方向 | type | 字段 | 语义 |
|---|---|---|---|
| ↑ | `board_register` | `{name, duty}` | 连接建立后**自动**发送（非 LLM） |
| ↑ | `board_send` | `{to[], content, reply_to, no_reply, state, timeout_seconds, req_id}` | 发消息 / 回复；`req_id` 供工具同步等回执 |
| ↓ | `board_registered` | `{ok, agent_id, roster_rev, duty_source}` | 注册确认 |
| ↓ | `board_roster` | `{rev, agents:[AgentInfo]}` | 注册表变化即全量广播 |
| ↓ | `board_sent` | `{req_id, message_id, conversation, deliveries:[{to, ok, error}]}` | **立即回执** |
| ↓ | `board_message` | `Message` | 投递给收件人 |
| ↓ | `board_timeout` | `{message_id, agents:[未响应名单], deadline}` | 超时系统事件 |

身份规则：`from` 由 hub 从连接身份推导（`Proxy.current[conn]`），**不信任报文自报值**。

### 4.1 `board_send` 工具参数（LLM 可见）

| 参数 | 必填 | 说明 |
|---|---|---|
| `to` | ✅ | `*`（广播）或逗号分隔的 agent id 列表 |
| `content` | ✅ | 消息正文（上限 8KB，超出报错） |
| `reply_to` | ❌ | 回复哪条消息；空 / `0` 表示新话题 |
| `no_reply` | ❌ | true = 本条只是知会，不需要任何人回复 |
| `state` | 回复时✅ | `handled` / `declined` / `need_info`（`reply_to` 非空时必填） |
| `timeout_seconds` | ✅ | **一律必填**，5..86400；LLM 按任务性质自行估时 |

**返回值**：成功 `message_id=m-12; conversation=*; delivered=3/4 (agent-c: offline); 等待应答窗口=300s`；失败给出明确原因（缺参 / 越界 / 未连 hub / 开关关闭 / 未知收件人 / hub 未确认）。

### 4.2 两个"等待"必须区分

| | 谁等谁 | 时长 | 处理 |
|---|---|---|---|
| A. 工具等 hub 发送回执 | co-shell 工具 → hub | 默认 5s，**系统参数可配**（决策 11），不由 LLM 指定 | 网络级确权；超时提示"消息可能未发出" |
| B. hub 等收件人应答 | hub → 收件人 | **发送方 `timeout_seconds`** | 超时生成 `board_timeout` 事件 |

工具在拿到 A 的回执后**立即返回**（不阻塞等待 B）。

## 5. co-shell 侧行为

### 5.1 自动注册（决策 1）
- 时机：hub 连入 serve 端口、WS 会话建立完成后**立即**上行 `board_register`（与 LLM 无关，不占回合）。
- 内容：`name` = `cfg.LLM.AgentName`（空则 "co-shell"）；`duty` = 职责描述（见 5.2）。
- 生效边界：注册内容为**启动快照**；运行中修改身份与个性不热更，重启后生效（文档中写明）。
- 前置：`board_enabled=false` 时不注册、不接收任何 board 消息、不注册工具（沿用现有开关，默认 off）。

### 5.2 职责描述来源（决策 8/9）
- agent 上报值：优先 `CAPABILITIES.md`（身份与个性 → 能力），为空回退 `cfg.LLM.AgentDescription`；过长截断（每个 agent ≤ 200 字）。
- hub 手工录入值（hub 配置 agent 的"职责描述"录入项）：**优先级更高**，有则覆盖 agent 上报值。
- 生效值来源记录在 `AgentInfo.DutySource`（`hub` / `agent`），并随 `board_registered` 回给 co-shell。

### 5.3 roster 注入（决策 6）
每轮在 `Agent.buildFullEnvironmentDetails` 中渲染（roster 为空则整块省略）：

```
<board_agents rev="7">
  <agent id="agent-b" name="审查员">负责 Go 并发与 SQL 审查</agent>
</board_agents>
```

### 5.4 消息聚合与分派（决策 5/6）
```
收到 board_message：
  1) 查去重表（message_id 环形缓冲，默认 1000 条）→ 命中则丢弃
  2) 记入去重表，推入聚合缓冲
聚合窗口（debounce 400ms）到期：
  3) 窗口内 N 条合并为一个块（每条渲染为 <board_message id=… from=… reply_to=… timeout=…>正文</board_message>）
  4) 此刻 !IsBusy() → 合成一条 input 注入 inputCh（开新回合）
     此刻 IsBusy()  → AddDynamicEvent(board_message)，本轮 LLM 即可见（不打断）
```
说明：去重表重启后丢失（可接受，文档标注）；同一 `message_id` 无论走哪条通道都只注入一次。

### 5.5 `board_send` 工具
- 仅 `BoardEnabled()` 时注册；参数与返回值见 §4.1。
- 实现：生成 `req_id` → 注册 `pending map[req_id]chan boardSent` → 上行 → 等待（默认 5s，可配）→ 回填工具返回值。
- 服务端尚未回执即超时：返回"hub 未确认，消息可能已发出"（不重试）。

### 5.6 系统提示词约定（i18n 中英）
- 收到 board 消息 → 先判断是否落在自己的职责描述范围内；
- **无论处理与否都要用 `board_send` 回三态**（`reply_to` = 收到的 message_id）：
  `handled` 由我处理（进展与结果继续用 handled + content 回复）／`declined` 不由我处理（不相关/无能力）／`need_info` 需要补充信息；
- `no_reply=true` 的消息（或本条本身不需应答）不要回复；
- 同一消息不重复回复；
- 超时估算参考：仅判断是否接手 30–60s；需读代码/查资料 300–600s；需实际执行改造 1800–7200s；纯知会使用 `no_reply=true`。

### 5.7 回执超时可配置（决策 11）
新增系统参数 `board-ack-timeout`（秒，默认 5），暴露于 `:set`、`:config` 与 Web 设置面板；用于 §4.2 的 A 类等待。

## 6. hub 侧行为

### 6.1 注册表与 roster 广播
`board_register` → 写 `AgentInfo`（duty 生效值见 5.2）→ `roster_rev++` → 向所有已连接 agent 广播 `board_roster`（全量）+ 推送 Web UI。
agent 断开 → 标记 `online=false` 并广播新 rev；重连注册 → 恢复。

### 6.2 发送 / 转发 / 回执
1. 校验：收件人合法、`content` 非空且 ≤ 8KB、`timeout_seconds` 必填且 5..86400、`state` 取值合法（仅回复时）；
2. 分配 `message_id`（`m-<seq>`，seq 持久化）→ 写会話 → **立即回 `board_sent`**（含每收件人投递状态）；
3. 异步逐收件人投递 `board_message`（广播排除发送者自己）；
4. 对"要求回复"的消息（`no_reply=false`），为每个投递成功的收件人建 `Awaiting{deadline = TS + timeout}`。

### 6.3 回执入账与超时
- 收到带 `reply_to` 的消息 → 落会話 → 对应 `Awaiting` 置 `answered` → 把该回复投递给原消息发送方（同一 `board_message` 通道）；
- 后台 tick（5s）扫描到期 `Awaiting` → 置 `timeout` → 生成 `board_timeout` 事件给发送方（附未响应名单与 deadline）；
- `no_reply=true` 的消息不建 `Awaiting`（不判定未响应）。

### 6.4 持久化（bbolt）
库文件：hub 独立文件（与 co-shell 的 `db/co-shell.db` 分开），buckets：

| bucket | key → value |
|---|---|
| `agents` | agentID → AgentInfo |
| `messages` | messageID（零填充序号便于有序扫描）→ Message |
| `conversations` | convKey → {participants, last_msg, updated_at} |
| `conv_msgs` | convKey + 单调序号 → messageID |
| `awaiting` | messageID + agentID → Awaiting |
| `meta` | `seq`、`roster_rev` |

单写事务；重启后 Chat 栏历史与未响应判定可恢复。

### 6.5 Web UI Chat 栏
- 位置：既有 `#chatHead` / `#chatBody` 面板（当前为占位符"Chat 功能规划中，暂不可用"）；
- 内容：会話列表（**"公告"会話固定置顶**，key=`*`，参与者显示"全部"；定向会話按 `a+b` 展示）+ 消息流（from / to / content / 时间 / `reply_to` 缩进 / 状态徽标 `已受理`｜`已拒绝`｜`待补充`｜`未响应` / 等待窗口剩余）；
- 接口：`GET /api/board/conversations`、`GET /api/board/conversations/{key}/messages?before=<seq>&limit=<n>`；刷新沿用现有 3s 轮询。

### 6.6 配置项改名（决策 8/10）
`hub/gateway/webui_static.go` 三处表单标签 `备注` → `职责描述`：L364（新建本地 agent）、L376（添加外部 agent）、L390（编辑抽屉）。
**仅改显示标签**：`AgentSpec.Name`、`agentView.Name`、JSON 字段 `name`、`hub-agents.json` 均不变（零兼容风险）。

## 7. 三态语义

| state | 含义 | 发送方如何理解 | hub 行为 |
|---|---|---|---|
| `handled` | 由我处理（受理；进展与结果继续用该态 + content 回复） | 已有人接单，不必再等别人 | 标"已受理"，终止该收件人计时 |
| `declined` | 不由我处理（不相关/无能力/不能承担）——必须显式回复，不能沉默 | 排除该 agent，继续等其他应答 | 标"已拒绝"，终止计时 |
| `need_info` | 可能是我，但需补充信息才能判断/开始 | 需要补充材料；不算接单也不算拒绝 | 标"待补充"，不计入未响应 |

配套规则：
- `no_reply=true`：收件方不应答，hub 不启动计时；
- 默认每条都必回三态；广播时 N 个收件人各回一条（同一 `reply_to`），发送方据此汇总；
- "沉默"≠`declined`：沉默由超时转化为"未响应"事件，二者语义不同。

## 8. 删除清单（决策 7）

**hub**：`BoardRequest` / `BoardTask` / `BoardDM` / 状态机常量 / `roles` 注册表；`MsgTypeBoardPost|List|Claim|DM|Confirm|Result`；`handleBoard` 六分支；`routeAgentMessage` 仅处理 `board_result` 的写法。
**co-shell**：`agent/board_tools.go` 六个工具与 `BoardSender` 旧用法；`agent/tools.go` 中六工具注册块；`agent/toolcall_mode.go` 六条 `KeyToolUsageBoard*` 映射；`i18n` 对应键；`agent/dynamic_events.go` 的 `board_request` / `board_dm` / `board_notify` 三类 kind；`web/session.go` 的 `board_task` 分支；`web/server.go` 的 `TaskID` / `Requester` / `Instruction` 字段。
**顺带修复**：hub 下发 `dynamic_event` 的 `value` 为对象、而 co-shell `clientMessage.Value` 为 string 导致 `dispatch` 静默丢弃的缺陷——新协议改用专用字段（`json.RawMessage`），不再复用 `Value string`。
**保留**：`board_enabled` 开关（默认 off）。

## 9. 改动文件清单（实施指引）

**hub 侧**
| 文件 | 改动 |
|---|---|
| `hub/gateway/board.go` | 重写为消息板（注册表 / Message / Conversation / Awaiting / 三态 / 超时判定） |
| `hub/gateway/board_store.go`（新） | bbolt 持久化层（§6.4 buckets） |
| `hub/gateway/proxy.go` | 新协议路由：`routeAgentMessage` 处理全部上行；`handleBoard` 改为 `board_register` / `board_send` |
| `hub/gateway/agent.go` | 连接建立/断开时触发注册回调与 roster 广播 |
| `hub/gateway/webui.go` | Chat 栏 API（会話列表 / 消息流） |
| `hub/gateway/webui_static.go` | Chat 面板实现 + 三处"备注"→"职责描述" |
| `cmd/co-shell-hub/main.go` | 挂载消息板与存储（初始化 bbolt 库文件） |

**co-shell 侧**
| 文件 | 改动 |
|---|---|
| `agent/board_state.go`（新） | roster 缓存、去重环形表、pending 回执表、聚合缓冲 |
| `agent/board_tools.go` | 删除 6 工具，改为 `board_send`（含 `timeout_seconds` 必填校验） |
| `agent/tools.go` | 工具注册块替换为 `board_send`（仍受 `BoardEnabled()` 约束） |
| `agent/envelope.go` | 渲染 `<board_agents>` |
| `agent/dynamic_events.go` | 新增 `board_message` kind 与渲染 |
| `web/session.go` | 注册握手、`board_roster` / `board_message` / `board_sent` / `board_timeout` 处理与忙闲分派 |
| `web/server.go` | `clientMessage` 增加 `json.RawMessage` 载荷字段，移除 `TaskID/Requester/Instruction` |
| `agent/toolcall_mode.go`、`i18n/*` | 工具用法、系统提示词协作约定、设置项文案（中英） |
| `config/config.go`、`cmd/settings*.go` | `board-ack-timeout` 设置项（默认 5s） |

**删除**：`board_task` 报文与旧 3 类 board 动态事件、旧工单相关全部代码与 i18n 键。

## 10. 验收标准

1. 两实例接入 hub → hub 收到两条 `board_register`，`roster_rev` 递增，`duty_source` 正确；
2. co-shell 的 `environment_details` 出现 `<board_agents>`，含对方 name 与职责描述；
3. A 广播（`to=*`）→ hub 立即回执（message_id + 投递清单 + 等待窗口）；B 空闲时起新回合，回 `handled`；A 下一轮能看到该回复；
4. B 忙碌时收到消息 → 不打断当前回合，本轮 LLM 可见（动态感知）；多条合并为一个块；
5. 同一 `message_id` 重复投递 → 只注入一次；
6. A 以 `timeout_seconds=120` 广播且无人应答 → 120s 后 A 收到"未响应"事件；（B 在窗口内回 `handled` 则不产生该事件）
7. 定向私信 `to=agent-b` → 只投递给 B，会話 key = `agent-a+agent-b`；
8. `no_reply=true` 的消息收件方不应答、不产生未响应事件；
9. `timeout_seconds` 缺失 / 越界 → 工具报错并回显合法区间；`state` 非法或缺省（回复时）→ 报错；
10. hub Chat 栏：公告会話唯一且置顶，定向会話按参与者分组，消息带三态徽标与等待窗口；
11. hub 重启后 Chat 栏历史、`roster_rev`、未响应判定仍正确（bbolt）；
12. 旧 6 个 board 工具在工具列表中彻底消失；`board_task` 报文不再被识别；
13. `go build ./... && go vet ./...` 全绿；新增单测覆盖：三态校验、去重注入、忙闲分派、合并窗口、超时事件、会話 key 生成、持久化读写。

## 11. 风险与注意

- **serve 单 WS 客户端槽位**：hub 接入后浏览器不能直连该实例的 Web UI（现状既有限制，文档写明）；
- **token 成本**：roster 每轮注入 + 消息每轮注入持续占用上下文 → 单条消息 ≤ 8KB、职责描述 ≤ 200 字/agent；roster 为空时整块省略；
- **广播风暴**：N 个收件人各回一条 → 靠 `no_reply` 与"三态必须显式回"两条规则平衡；
- **bbolt 独占**：hub 新库文件需与 `db/co-shell.db` 分开，单进程独占；
- **去重表易失**：co-shell 重启后去重表清空，理论上可能重复注入重启前已注入的消息（可接受，已在文档标注）。

## 12. 与旧实现（FEATURE-490）的关系

| | 旧（FEATURE-490） | 新（FEATURE-530） |
|---|---|---|
| 模型 | Request/Task 工单状态机 | Message + 回复链 + 会話 |
| 注册 | 无（roles 表恒空） | 连接后自动注册，hub 广播 roster |
| 感知 | `board_request/dm/notify`（因 roles 空而不发出） | `board_message`，忙闲分派 + 合并 + 去重 |
| 工具 | 6 个（仅 `board_result` 真生效） | 1 个 `board_send` |
| 回执 | 无 | 立即回执 + 三态应答 + 未响应事件 |
| 私信 | 依附工单线程 | 一等公民（收件人列表） |
| UI | 无 | Chat 栏（会話 + 徽标） |
| 持久化 | 无（内存） | bbolt |
