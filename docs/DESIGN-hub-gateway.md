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

### 5.1 hub Web UI 访问控制（用户确认，步骤6要求）

hub Web UI 需与 co-shell 一致支持**访问白名单**，默认仅本机访问：

- 复用 co-shell 的访问控制模式（FEATURE-430/431）：`--bind`（默认 127.0.0.1）+ `--whitelist`（逗号分隔 IP/网段，空=仅本机访问）
- **无白名单时强制本机访问**（忽略 `--bind`，绑定 127.0.0.1），避免意外暴露到网络
- 白名单校验支持精确 IP 与 CIDR 网段（复用 `web/server.go` 的 `parseWhitelist`/`ipAllowed` 逻辑）
- 该要求纳入步骤6（hub Web UI 界面）实现

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

## 9. agent 生命周期管理（用户确认，扩展 hub 核心职责）

hub 应具备**管理 co-shell agent 的能力**（核心职责之一）：在界面中创建 agent 实例（创建/指定 co-shell workspace 位置，可新建或复用），并提供 co-shell `--serve` 服务进程管理界面（启动/停止指定 agent）。

### 9.1 设计决策（用户确认）

1. **co-shell 可执行文件路径**：`--co-shell-path` 参数指定（默认取 hub 同目录的 co-shell）。
2. **端口分配**：hub 自动分配（从起始端口递增），**用户可修改**（创建后可编辑端口）。
3. **workspace 创建**：hub 自动创建 workspace 目录；**config.json 是可选项**，且只在指定 workspace 下创建**空文件**（代表一个 co-shell 实例仅使用一个专用 config.json，不使用默认路径配置）。
4. **进程管理**：hub 作为 agent 进程的父进程，负责启动/停止/监控退出。
5. **不受控 agent**：支持添加**不受控的 co-shell 服务地址**（外部已运行的 agent，hub 只连接访问，不负责启停）。

### 9.2 agent 类型

- **受控 agent（managed）**：hub 创建 workspace + 启动 co-shell --serve 子进程，hub 管理其生命周期（启动/停止/监控）。
- **不受控 agent（external）**：用户提供 WS 地址，hub 只连接访问，不负责启停。

### 9.3 agent 注册表（持久化）

hub 维护 agent 配置列表，持久化到配置文件（如 hub-gateway.json）：

```json
{
  "agents": [
    {"id": "a1", "name": "Agent A", "type": "managed",
     "workspace": "/path/ws/a1", "port": 8390, "config_file": "config.json"},
    {"id": "ext1", "name": "External", "type": "external",
     "ws_url": "ws://192.168.1.5:8399/ws"}
  ]
}
```

### 9.4 Web UI 管理界面

- agent 列表显示：id/名称/类型/workspace/端口/运行状态
- 操作：创建 agent（输入 workspace 路径，可新建或复用）、启动、停止、删除、编辑端口
- 创建时可选：是否创建空 config.json
- 支持添加不受控 agent（输入 WS 地址）

### 9.5 进程管理

- hub 启动受控 agent：`co-shell --serve --port <port> --bind 127.0.0.1 -w <workspace> [-c <config>]`
- hub 监控进程退出，agent 崩溃时自动重启（可选）
- 停止 agent：终止子进程，断开 WS 连接

### 9.6 实现状态（步骤7-8 已完成）

- **Manager（hub/gateway/manager.go）**：agent 注册表持久化到独立 JSON 文件（默认 `./hub-agents.json`，`--registry` 指定）；`CreateManaged` 自动创建 workspace + 可选空 config.json + 自动分配端口（`--base-port` 起始，默认 12810）；`AddExternal` 注册不受控 agent；`Start` 启动 co-shell `--serve` 子进程并监控退出；`Stop`/`Remove`/`StopAll` 终止进程。co-shell 路径由 `--co-shell-path` 指定（默认取 hub 同目录 co-shell，解析为绝对路径）。
- **入口集成（cmd/co-shell-hub/main.go）**：创建 Manager 加载注册表，启动时自动连接注册表中 external agent（及已运行的 managed agent）；`--agent ID=WSURL` 作为外部 agent 幂等加入注册表。
- **Web UI 管理端点（hub/gateway/webui.go）**：`GET /api/agents`（列表含运行/连接状态）、`POST /api/agents`（创建受控）、`POST /api/agents/external`（添加不受控）、`POST /api/agents/{id}/start|stop`、`DELETE /api/agents/{id}`。启动受控 agent 后带重试连接其 WS 端口。
- **前端（hub/gateway/webui_static.go）**：右侧管理面板含创建受控 agent 表单（workspace + 可选 config.json）、添加不受控 agent 表单、agent 列表（类型/运行状态徽标 + 启动/停止/删除按钮），每 3 秒自动刷新。
- **说明**：受控 agent 的 workspace 若 config.json 为空（`{}`），co-shell 启动会进入模型配置向导并因无 TTY 退出——需在 workspace 提供含已配置模型的 config.json 才能正常 `--serve`。

## 10. Web UI 架构演进：iframe 多页外壳 + 反向代理（用户确认，FEATURE-484 延续）

> 本节记录 hub Web UI 的一次**架构方向调整**。经与用户讨论确认：hub 不再"聚合转发 co-shell 业务消息 + 自绘简化聊天界面"，而是转向 **iframe 多页外壳 + 反向代理**——hub 完整透传每个 co-shell 实例的 Web UI，只维护外壳（agent 切换 + 管理 + 移动端自适应）。

### 10.1 背景与动机

原实现（§3-§8）中 hub 作为 WebSocket 客户端连接多个 co-shell agent，把 agent 的 serverMessage 包进 `agent_event` 信封转发给浏览器，hub 前端（webui_static.go）自绘一个简化聊天界面。该方案存在明显局限：

- hub 前端只渲染了 co-shell 消息流的**一小部分**（content_chunk/text/done），thinking/tool_call/command/ask/interaction/plan/session/settings 等结构化内容无法完整呈现。
- 上行交互弱（只支持发文本，无法回答 ask、处理 interaction、切换 session 等）。
- hub 需要长期维护一套"简化版"前端，且永远追不上 co-shell 原生前端的演进。

用户确认新方向：**hub 不维护 co-shell 界面，完整透传 co-shell 的 Web UI（含界面），只注入/维护 hub 自己的增强部分（agent 切换、管理入口）**。

### 10.2 目标架构

```
浏览器（单用户，后连独占，先连自动断开不重连）
   │
   ▼
hub 外壳页面（hub 自己 serve：agent 切换栏 + 管理菜单，移动端自适应）
   │
   ├── iframe #1 ──▶ hub 反向代理 ──▶ co-shell agent A (127.0.0.1:portA)
   ├── iframe #2 ──▶ hub 反向代理 ──▶ co-shell agent B (127.0.0.1:portB)
   └── ...（每个 agent 一个 iframe，各自独立完整 co-shell Web UI）
```

- **每个 co-shell 实例的完整 Web UI**（index.html + app.js + style.css + /ws + /api/...）经 hub 反向代理原样透传，浏览器看到的就是 co-shell 原生界面，功能完整。
- **hub 外壳**只负责：agent 切换（显示/隐藏对应 iframe）、agent 管理（创建/启停/删除，复用 §9 Manager）、移动端自适应。
- **co-shell 实例界面内部状态**（session/plan/settings 等）由各实例自己负责，hub 不碰、不缓存。

### 10.3 关键设计决策（用户确认）

1. **单用户独占**：hub 外壳只允许一个浏览器客户端。后连接的独占，先连接的自动断开且不自动重连（与 co-shell 自身 single-client 模型一致）。
2. **hub 负责跨源代理**：每个 iframe 加载 hub 自己的 URL（`http://hub:port/agent/{id}/`），hub 后端反向代理到对应 co-shell 实例的 `http://127.0.0.1:{port}/`。浏览器只与 hub 一个源打交道，规避跨域/同源问题。
3. **hub 解决端口转发**：co-shell 实例监听 127.0.0.1，远程浏览器无法直连。hub 反向代理把 `hub/agent/{id}/...` 转发到 `127.0.0.1:{port}/...`（含 HTTP 静态资源 + WebSocket /ws + /api 文件接口）。
4. **数据缓存降级为可选优化**：状态由各 co-shell 实例自己管理，hub 不做业务缓存。切换 agent 时 iframe 保留（或按需销毁重建），切回时状态仍在（co-shell 实例自己持久化 session）。
5. **移动端适配**：外壳布局自适应，窄屏时 agent 切换栏/管理菜单自动收起为缩略图标（汉堡菜单/抽屉），点开才展开；iframe 全屏占满内容区。

### 10.4 单端口 + 子路径前缀方案（用户确认）

co-shell 前端硬编码大量**根路径绝对路径**（见 §10.6 清单），且 WS 用 `location.host + "/ws"`。为让多个 agent 的 iframe 在 hub 单端口下共存，采用**子路径前缀**区分：

- 每个 agent 的 iframe 加载 `http://hub:port/agent/{id}/`（注意以 `/` 结尾）。
- co-shell 前端改用**相对路径**（方式乙，见 §10.5），相对路径在 `/agent/{id}/` 下解析为 `/agent/{id}/static/...`、`/agent/{id}/api/...`、`/agent/{id}/ws`。
- hub 反向代理把 `/agent/{id}` 前缀**剥掉**，转发到对应 co-shell 实例的根路径（`/static/...`、`/api/...`、`/ws`）。co-shell server 路由无需改动（仍部署在根路径）。

### 10.5 co-shell 前端改相对路径（方式乙，单一代码源）

**决策**：直接修改 co-shell 自身前端（`web/static/`），把绝对路径改为相对路径，让 co-shell Web UI 支持子路径部署。hub 不复制前端、不维护第二套代码。

- **优点**：单一前端代码源（co-shell 自己维护）；hub 只做透明反向代理（剥前缀），不深度改写内容；符合"hub 不维护 co-shell 界面"。
- **对 co-shell 自身无影响**：co-shell 默认部署在根路径，相对路径在根路径下与绝对路径等价。
- **前提**：iframe 加载 URL 必须以 `/` 结尾，相对路径才能正确解析到 `/agent/{id}/` 前缀下。

### 10.6 co-shell 前端需改的绝对路径清单（调研结果）

**index.html**（4 处）：
- `/static/app.js`、`/static/favicon.png`、`/static/md.js`、`/static/style.css`

**app.js**（字符串字面量，需改为相对路径）：
- `/api/bootstrap`、`/api/download`、`/api/file`、`/api/get-model-max-len`、`/api/gitdiff`、`/api/logo`、`/api/open`、`/api/reveal`、`/api/test-api-key`、`/api/test-endpoint`、`/api/tree`、`/api/upload`
- `/logos/`（品牌 logo，`/logos/{theme}`）
- `/static/logos/`（模型 logo）
- `/ws`（WS 地址，`new WebSocket("ws://" + location.host + "/ws")` → 需改为相对 `ws` 或基于 `location.pathname` 前缀）

**注意**：`/ws` 的 WS 地址是 `location.host + "/ws"`，改相对路径时需处理为基于当前路径前缀的 WS 地址（如 `(location.protocol==='https:'?'wss://':'ws://') + location.host + location.pathname + 'ws'`，其中 pathname 以 `/agent/{id}/` 结尾）。

### 10.7 hub 新增模块

- **反向代理模块**（hub/gateway/ 下新增，如 `proxyhttp.go`）：解析 `/agent/{id}/...` 路径，剥掉前缀后转发到对应 co-shell 实例的 `http://127.0.0.1:{port}/...`。支持：
  - HTTP 静态资源与 /api 接口（用 `httputil.ReverseProxy` 或手写转发）。
  - WebSocket /ws 升级后双向转发（用 gorilla/websocket 或 httputil 的 WS 支持）。
- **外壳页面**（替换 webui_static.go 的独立聊天界面）：HTML/CSS/JS 实现 agent 切换栏 + 管理菜单 + iframe 布局 + 移动端自适应。
- **单用户独占**：外壳 WS/连接管理，后连独占、先连断开。

### 10.8 与现有代码的关系

- **替换**：`hub/gateway/webui_static.go` 的独立聊天界面被外壳页面替换；`proxy.go` 的 WS 客户端代理转发逻辑不再需要（每个 agent 由 iframe 内 co-shell 前端经 hub 反向代理直连）。
- **保留**：`manager.go`（agent 生命周期管理，外壳管理菜单调用）；`server.go`/`auth.go`（TCP + API Key 服务，供移动端）；`wsserver.go`（若移动端仍需 WS 网关）。
- **新增**：反向代理模块 + 外壳页面。

### 10.9 实施步骤（分步）

1. **co-shell 前端改相对路径**：修改 `web/static/index.html` + `app.js`，把 §10.6 清单的绝对路径改相对路径；验证 co-shell 自身（根路径部署）功能不受影响。
2. **hub 反向代理模块**：新增 `/agent/{id}/` 前缀的反向代理（HTTP + WS），转发到对应 co-shell 实例。
3. **hub 外壳页面**：替换独立聊天界面，实现 agent 切换 + 管理 + iframe 布局 + 移动端自适应。
4. **单用户独占**：外壳连接管理。
5. **编译验证 + 浏览器实测**（多 agent 切换、完整 co-shell UI 显示、移动端布局）。
6. **提交到分支 FEATURE-484**。

### 10.10 实现状态（步骤12-13 已完成）

- **co-shell 前端改相对路径**：`web/static/index.html` 的 4 处静态资源（favicon/style.css/md.js/app.js）与 `web/static/app.js` 的全部 `/api/...`、`/logos/`、`/static/logos/` 改为相对路径；WS 地址改为基于 `location.pathname` 前缀的相对地址（`(location.protocol==='https:'?'wss://':'ws://') + location.host + location.pathname + 'ws'`）。co-shell 根路径部署时相对路径与绝对路径等价，功能不受影响。
- **hub 反向代理模块（hub/gateway/proxyhttp.go）**：`httpReverseProxy` 解析 `/agent/{id}/...`，剥掉前缀后经 `httputil.ReverseProxy` 转发到对应 co-shell 实例的 `http://127.0.0.1:{port}/...`（managed 用 Port，external 从 WSURL 推导 HTTP base）。`httputil.ReverseProxy` 同时处理 HTTP 静态资源 + /api + WebSocket /ws 升级（Go 1.12+ 自动隧道升级连接）。
- **hub 外壳页面（webui_static.go 重写）**：agent 切换栏（含运行状态点 + 关闭按钮）+ 管理抽屉（创建受控/添加不受控/启停删）+ iframe 布局（每 agent 一个 iframe，切换显示/隐藏，保留状态）+ 移动端自适应（`@media (max-width:640px)` 标签栏换行到第二行）。
- **webui.go 集成**：注册 `/agent/` 前缀到反向代理；移除旧 `/ws` 聊天桥接（外壳不再需要 hub WS，各 agent 的 co-shell UI 经反向代理直连自身 /ws）。
- **单用户独占**：iframe 架构下由各 co-shell 实例自身的 single-client WS 模型天然强制（第二个浏览器对同一 agent 的 iframe 连接会被 co-shell 拒绝/断开），hub 外壳本身为无状态 HTTP，无需额外 hub 级 WS 独占。
- **浏览器实测通过**：hub 外壳加载两个 agent（ws-a/ws-b），各自 iframe 完整显示 co-shell UI（工作区侧栏 + 聊天区），均显示"已连接"（WS 经反向代理隧道成功），点击标签可切换 agent 且各自状态保留。
- **说明**：`proxy.go` 的 WS 客户端代理转发逻辑（步骤4-6 产物）在新架构下不再被 Web UI 使用，但保留（供 TCP 服务端 / 移动端网关复用）。
