# Agent 公告板协作机制 — 设计方案

> 状态：架构讨论稿（待用户确认后转 ROADMAP 任务）
> 日期：2026-09-08
> 范围：hub 中新增 agent 公告板，让接入 hub 的多个 co-shell 实例互相协作

## 1. 目标与场景

让接入 hub 的多个 co-shell 实例能互相协作：

1. 一个 co-shell 发出"请求协助"信息（公告板广播）
2. 其他 co-shell 实例看到后，若与自身职责相关则响应（认领）
3. 条件不足时，双方通过公告板私信（DM）讨论，确定目标后再执行
4. 执行方完成任务后返回结果给请求方

## 2. 已确认的方向性决策

| 决策点 | 结论 |
|--------|------|
| 通信通道 | **WS 原生双向通道为主** + MCP 作为可选外部接入面 |
| 下发感知 | **广播用巡检感知 + 定向任务用主动注入**（方案 C） |
| 公告板中枢 | 放 hub（天然汇聚点） |

## 3. 现状调研结论（关键约束）

### 3.1 hub 架构
- hub 是 **WebSocket 聚合网关**：`Proxy` 维护 `agents map[string]agentLink`，每个 agent 一个 `AgentConn`（hub 作为 WS 客户端连 co-shell 的 `--serve` 端口）。
- hub **只做代理转发，不解释业务逻辑**：`Proxy.HandleMessage` 只处理 `switch_agent`/`list_agents`，其余消息原样转发到当前 agent。
- **最大缺口**：`AgentConn.readLoop` 只把 agent 消息广播给订阅的 Web UI 客户端，**hub 没有向 agent 主动推送业务消息的通道**（`Send` 仅转发客户端消息）。

### 3.2 co-shell 动态感知机制（FEATURE-471）
- `agent/dynamic_events.go`：`dynamicEventQueue`，`AddDynamicEvent(kind, pathOrText)` 入队，`consumeDynamicEvents()` 在注入 `<environment_details>` 时排空渲染成 `<user_dynamic_events>` 块。
- 现有 kind：`clip_object`/`upload_file`/`user_message`/`open_file`/`view_file`。
- **本质是"被动感知"**：事件在下一轮 user/tool 消息注入时被 LLM 看到，**不主动触发 agent 执行**。

### 3.3 co-shell serve 消息处理
- `web/session.go` `handleMessage`：`input` 类型 → `s.inputCh`（agent 运行循环消费，触发执行）；`dynamic_event` → `AddDynamicEvent`（被动感知）。
- 即：**要让 co-shell 主动执行任务，本质是向 inputCh 注入一条"输入"**。

## 4. 核心设计：两种下发语义分离

| 消息类型 | 语义 | 通道 | 机制 |
|---------|------|------|------|
| **感知类**（新请求广播、私信到达、状态变更） | 让 LLM"知道" | 通道1 | 复用动态感知 `dynamic_event` |
| **执行类**（认领后执行任务） | 让 agent"主动去做"并回传 | 通道2 | 新增 `board_task` → 注入 inputCh |

### 4.1 通道1：感知通道（复用动态感知机制）
hub 向 co-shell 发送 `{type:"dynamic_event", kind:"board_*", value:"..."}`。
- co-shell dispatch 已支持 `dynamic_event` → `AddDynamicEvent`。
- 需**扩展 `DynamicEventKind`**：新增 `board_request`/`board_dm`/`board_notify` 等，并在 `consumeDynamicEvents` 增加渲染分支。
- 适用：广播新请求、私信到达、任务状态变更等"让 agent 知道"的消息。
- 局限：agent 空闲时不主动触发（靠巡检唤醒）。

### 4.2 通道2：执行通道（新增）
hub 向 co-shell 发送新的上行消息类型 `{type:"board_task", task_id, instruction, requester_id, ...}`。
- co-shell dispatch 识别后，**作为一条 input 注入 agent 运行循环**（复用 `s.inputCh` 路径）。
- 这条 input 需**带上下文标记**（如 `[board-task #123 from agent-X]`），让 agent 知道是协作任务而非真人指令，在系统提示词层面约束行为。
- 执行结果通过**新增工具 `board_result`** 回传 hub（agent 主动调用，内部走 WS 上行）。

## 5. 空闲 agent 唤醒机制（方案 C 落地）

广播用巡检感知 + 定向任务用主动注入：

- **巡检感知**：co-shell 用 scheduler 定时（如每 N 秒）主动查一次公告板（`board_list`），有新请求就触发自己判断是否响应。适合"抢单式"自主协作。
- **主动注入**：hub 直接向目标 agent 注入 `board_task` 触发执行。适合"请求方指定"的定向协作。

## 6. 公告板实体与状态机

### 6.1 实体
```
Request(请求)          DM(私信)              Task(任务)
├─ id                  ├─ thread_id          ├─ id
├─ requester_id        ├─ from/to            ├─ request_id
├─ title/desc          ├─ content            ├─ assignee_id
├─ required_role        ├─ ts                 ├─ status(negotiating/
├─ status:             └─ ...                 │   executing/done/
│   open/claimed/                              │   failed/cancelled)
│   negotiating/                              ├─ result
│   executing/done/                           └─ ...
│   failed/cancelled
└─ ...
```

### 6.2 协作流程状态机
```
请求方A                          hub Board                    响应方B
  │  post_request(role,desc)      │                              │
  │──────────────────────────────►│ 创建 Request(open)           │
  │                               │ 广播 board_notify(通道1)      │
  │                               │─────────────────────────────►│ 巡检感知到
  │                               │                              │ 判断职责匹配
  │                               │◄──── claim(request_id) ──────│
  │                               │ 状态→claimed, 记录B          │
  │◄──── 通知"B已认领" ────────────│                              │
  │  (可选) 私信澄清需求            │                              │
  │◄──── board_dm ────────────────│◄──── board_dm ───────────────│
  │──── board_dm ────────────────►│──── board_dm ───────────────►│
  │  (达成一致)                    │                              │
  │  confirm(request_id)          │ 状态→negotiating→executing   │
  │──────────────────────────────►│ 创建 Task, 指派给B           │
  │                               │ 下行 board_task(通道2)        │
  │                               │─────────────────────────────►│ agent 执行
  │                               │◄──── board_result(task_id) ──│
  │◄──── 收到结果 ────────────────│ 状态→done, 存结果             │
```

## 7. 模块改动清单

### 7.1 hub 侧（公告板中枢）
- **新增 `hub/gateway/board.go`**：Board 业务层，维护 Request/DM/Task 状态机、职责注册表、消息路由。
- **`AgentConn` 增加下行写通道**：goroutine + channel，让 Board 能向指定 agent 主动推送 `board_notify`/`board_dm`/`board_task`。需保证与现有 `Send` 并发安全（统一走写队列）。
- **`Proxy.HandleMessage` 识别公告板消息类型**：`board_post`/`board_claim`/`board_dm`/`board_confirm`/`board_result` 等交给 Board，而非转发给当前 agent。
- **职责注册表**：agent 接入时声明 role，Board 维护 `role registry`。
- **可选 MCP Server**：暴露公告板为 MCP server（stdio/SSE），工具 `board_post`/`board_list`/`board_claim`/`board_dm`/`board_respond`/`board_result`，供外部客户端接入。

### 7.2 co-shell 侧（协作 Agent）
- **扩展 `DynamicEventKind`**：新增 `board_request`/`board_dm`/`board_notify`，`consumeDynamicEvents` 增加渲染分支（通道1）。
- **serve dispatch 新增 `board_task` 处理**：识别后注入 inputCh 触发执行（通道2）。
- **新增工具 `board_result`**：agent 执行完主动回传结果。
- **巡检调度**：scheduler 定时 `board_list` 检查新请求（可选，配合通道1感知）。
- **安全边界**：默认不自动执行外部任务，需配置开启（如 `--board-agent` + 职责声明 + 是否需人工确认）。

## 8. 安全与权限

- co-shell 不应无条件执行外部指令，需配置"是否参与协作"、"可接受的职责范围"、"是否需人工确认"。
- 公告板消息需校验来源（仅接受来自 hub 的合法连接）。
- 产物文件通过共享工作区或 hub 文件中转传递。

## 9. 待确认问题

1. 执行任务的隔离性：当前会话执行 vs 隔离子会话/子任务执行（推荐隔离，不干扰当前工作）。
2. 安全边界默认值：是否默认关闭自动执行，需显式开启。
3. 本次范围：最小可用闭环 vs 含 Web UI 可视化公告板界面。
4. 版本归属与任务编号（FEATURE-XXX）。
