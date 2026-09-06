# co-shell-hub WebSocket 网关架构设计（FEATURE-484）

> 分支：FEATURE-484 ｜ 版本：v0.38.0
> 状态：设计中
> 目标：将 co-shell-hub 从"UDP + stdin/stdout 管道"架构升级为 **WebSocket 聚合网关**架构。

## 1. 背景与目标

原 hub（FEATURE-128/183）通过 UDP 与移动端通讯，通过 stdin/stdout 管道管理 co-shell agent 进程。用户提出架构演进：hub 通过 co-shell 的 `--serve` WebSocket 端口与多个 co-shell 通讯，甚至直接提供 Web UI 聚合多 agent。

**hub 基本目标**（用户确认）：
1. 提供通讯服务（传统 TCP + API Key 认证）
2. 原则上不处理 co-shell 业务逻辑，仅做代理转发
3. 能够同时与多个 agent 通讯，并负责提供多 agent 切换
4. 提供 Web UI 界面，主要负责转发 co-shell 返回的信息，实现 agent 切换和数据缓存（保持每次切换到同一 co-shell 实例与上一次显示内容一致，不因切换丢失会话内容）

**架构简化决策**（用户确认）：
- 放弃 UDP 加密通道，改用**传统 TCP** 通讯
- 安全方面仅实现 **API Key 认证**：hub 只对带有正确 API Key 的请求进行响应
- 通讯加密通过 **VPN 等成熟方案**实现，与 hub 无关（hub 不负责加密）

## 2. co-shell WebSocket 协议调研结论

### 2.1 客户端→服务端消息（web/session.go handleMessage）

| 消息类型 | 用途 |
|---------|------|
| input | 发送用户输入给 agent（触发任务） |
| session_list / session_switch / session_delete / session_rename / session_new / session_pop | 会话管理 |
| settings_get / settings_set | 系统设置 |
| mcp_get / mcp_add / mcp_update / mcp_remove | MCP 服务器管理 |
| mode_get / mode_switch | 工作模式切换 |
| model_* | 模型管理 |
| answer / interaction_answer | 交互应答（ask/选择） |
| interrupt | 中断当前任务 |
| dynamic_event | 动态感知事件上报 |
| yolo_set / yolo_get | YOLO 模式 |

### 2.2 服务端→客户端

- 事件流（stream events）：通过 WebRenderer 推送（content_chunk/tool_call/token_iter/done 等）
- 会话列表推送（session_list）
- 任务计划推送（taskplan 事件）

### 2.3 关键约束

- **单 WebSocket 客户端连接**：co-shell serve 端口同一时刻仅允许一个 WebSocket 客户端（两浏览器互抢）。因此 hub 作为客户端独占每个 co-shell 连接后，浏览器无法直连，需由 hub 对外提供 Web UI。
- **会话（session）机制**：每个 co-shell 实例内部管理多个会话，`session_switch` 切换会话且不丢失内容。hub 网关可复用此机制实现"切换 agent 不丢失会话"。

## 3. 目标架构

```
┌─────────────┐   UDP加密    ┌──────────────────────────────┐
│  移动端 App  │◄────────────►│          co-shell-hub        │
└─────────────┘              │  ┌────────────────────────┐  │
                             │  │ 安全通讯层 (UDP加密)     │  │
┌─────────────┐   WebSocket  │  └───────────┬────────────┘  │
│  Web UI     │◄────────────►│              │ 代理转发       │
│ (浏览器)     │              │  ┌───────────▼────────────┐  │
└─────────────┘              │  │ Agent 连接管理器        │  │
                             │  │  (每 agent 一个 WS 客户端)│  │
                             │  └───────────┬────────────┘  │
                             │              │               │
                             │  ┌───────────▼────────────┐  │
                             │  │ 多 agent 切换 + 数据缓存 │  │
                             │  └────────────────────────┘  │
                             └──────────────────────────────┘
                                        │ WebSocket (每 agent 一个 serve 端口)
                              ┌─────────┼─────────┐
                              ▼         ▼         ▼
                        ┌─────────┐ ┌─────────┐ ┌─────────┐
                        │co-shell │ │co-shell │ │co-shell │
                        │ agent 1 │ │ agent 2 │ │ agent N │
                        └─────────┘ └─────────┘ └─────────┘
```

## 4. 核心设计决策

### 4.1 hub 作为 WebSocket 客户端连接每个 co-shell

- 每个 co-shell agent 以 `--serve` 启动，暴露一个 WebSocket 端口
- hub 为每个 agent 维护一个 WebSocket 客户端连接（gorilla/websocket）
- hub 转发客户端消息（input/session_switch 等）到对应 agent，把 agent 事件流转发回前端/移动端

### 4.2 API Key 认证层（TCP）

- 移动端/Web UI 通过 TCP 连接 hub，请求需携带正确 API Key
- hub 只对带正确 API Key 的请求响应，否则拒绝
- 通讯加密由 VPN 等外部方案提供，hub 不负责加密（简化）

### 4.3 代理转发（不处理业务逻辑）

- hub 原则上不解析 co-shell 业务消息，仅做透明转发
- 消息格式：`{agent_id, payload}`，hub 根据 agent_id 路由到对应 agent 的 WebSocket 连接

### 4.4 多 agent 切换与数据缓存

- 复用 co-shell 的 session 机制：每个 agent 内部管理会话
- hub 维护"当前 agent + 当前会话"状态，切换 agent 时向目标 agent 发送 session_switch
- 数据缓存：hub 缓存每个 agent 最近的事件流/会话列表，切换回来时恢复显示

## 5. 待确认/待细化

- [x] hub Web UI 是复用 co-shell 前端静态资源还是新建（待步骤6细化）
- [x] 移动端与 hub 的 TCP 协议如何承载 WebSocket 转发（消息封装格式：长度前缀 JSON 帧，见 §7）
- [ ] 单客户端限制下 hub 与 co-shell 的连接管理（重连、心跳）
- [ ] 数据缓存的具体粒度（事件流/会话/任务计划）

## 6. 代码组织决策（用户确认：方案 B）

在 `hub/` 下新建**独立 gateway 包**（`hub/gateway/`），旧 UDP 代码（`hub/hub.go` 等）**保留不动**，新架构代码独立演进。gateway 包作为 hub module 的子包（`github.com/idirect3d/co-shell/hub/gateway`），零外部依赖（纯标准库），后续步骤4引入 gorilla/websocket 时再按需添加。

## 7. gateway 包实现（步骤3：TCP 服务 + API Key 认证层）

已实现文件：

- `config.go` — `Config`（ListenAddr/APIKey/MaxFrameSize/AuthTimeout）+ `DefaultConfig`
- `protocol.go` — 长度前缀 JSON 帧（4 字节大端长度头 + JSON 载荷），`Envelope{Type,Payload}` 信封
- `auth.go` — `Authenticator`，API Key 常量时间比较（`crypto/subtle`）
- `server.go` — `Server`（TCP 监听/连接管理/认证握手/读循环）+ `Conn`（并发安全 Send）+ `Handler` 接口（OnConnect/HandleMessage/OnDisconnect，供步骤4挂接 WebSocket 转发）
- `server_test.go` — 单元测试（认证成功/失败/未认证拒绝/业务消息转发/ping-pong）

**协议**：客户端连接后先发 `{"type":"auth","payload":{"api_key":"..."}}`；服务端校验通过回 `auth_ack`，失败回 `error` 并断开。认证通过后客户端可发任意业务消息（`input`/`session_switch` 等），gateway 不解释业务逻辑，经 `Handler` 透明转发（步骤4实现）。支持 `ping`/`pong` 心跳。

## 8. 实施步骤（对应任务计划）

1. 架构设计（本文档）
2. 修复 main.dart 编译错误
3. hub TCP 服务 + API Key 认证层（gateway 包，已完成）
4. hub WebSocket 客户端连接多个 co-shell agent（代理转发）
5. hub 多 agent 切换与数据缓存
6. hub Web UI 界面
7. 移动端适配 hub 网关
8. 编译验证
