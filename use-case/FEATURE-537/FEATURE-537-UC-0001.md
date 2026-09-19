# FEATURE-537 用例：hub Chat 面板微信式会话列表 / 详情 / 用户发言 / 移动端适配（分支 FEATURE-537 / v0.67.0）

> 需求（用户原话）：
> 1、Chat 抽屉内的内容，可以模仿微信会话列表的操作方式，第一级是会话列表，点击之后向右扩展会话的具体对话内容，也可以仿照微信的方式，显示谁在什么时间说了什么。
> 2、界面应该支持移动端（窄屏），也是模仿微信移动端，点击会话后整体转入目标会话，点击返回可以退出到会话列表。

> 用户答复确认（2026-09-18）：
> Q1 交互范围 = **既要界面，也要能在 hub 里向会话发言（需新增后端发送接口与输入框）**
> Q2 宽屏布局 = **覆盖式：点击会话后详情整屏滑入覆盖列表，带返回（与移动端一致）**
> Q3 详情呈现 = **微信式气泡：按发送者左右对齐，头像/名字 + 时间 + 内容**
> Q4 窄屏抽屉 = **全屏铺满（100vw）：列表占一屏，点击会话后详情占一屏，返回回列表**
> Q5 版本/编号 = **v0.67.0，任务 FEATURE-537，分支 FEATURE-537**

## 现状（改造前）

- hub Web UI 是内嵌单页（`hub/gateway/webui_static.go` 的 `webIndexHTML`），左侧抽屉含 Agents / Chat 两个可折叠区块。
- Chat 区块为**上下同屏**：`.board-convs`（会话列表，max-height 40%）+ `.board-msgs`（消息流）；点击会话只切换下方消息，列表始终可见，无返回概念；消息为**卡片**（`from → to` + 时间 + 内容），非气泡。
- 数据只读：`GET /api/board/conversations`、`GET /api/board/conversations/{key}/messages`；无任何写入接口。

## 实现（改动面）

- `hub/gateway/board.go`：`SendRequest` 增加可选 `Conv string` 字段；非空时 `Send` 直接使用该会话 key（不再由 `from+to` 推导），使人类发言归入既有会话而非新开 `a+b+user` 会话。
- `hub/gateway/webui.go`：新增路由 `POST /api/board/conversations/{key}/messages`。
- `hub/gateway/webui_board.go`：新增发送 handler（校验 key、content；映射错误码；调用 `Board.Send`）。
- `hub/gateway/webui_static.go`：Chat 区块改为两级视图（列表视图 / 详情视图 + 返回按钮 + 底部输入框）；CSS 增加气泡、滑入动画、窄屏 100vw；JS 增加视图切换、气泡渲染、发言提交与刷新。

## 设计口径（已与用户确认）

1. **人类发言者身份**：hub 侧以固定的发送者 id `user` 落库；前端显示为「我」，气泡靠右；agent 侧收到 `from: "user"` 的 `board_message`。
2. **投递目标**：广播会话（key `*`）→ `to=["*"]`（hub 展开为所有已注册 agent，不含发送者）；普通会话（如 `a+b`）→ `to` 为该会话除 `user` 外的参与者，并以 `Conv=a+b` 显式归属，保证**不新建会话**。
3. **不强制回复**：用户发言 `NoReply=true`（不产生「等待 Ns」窗口，不触发超时通知），发言即入会话。
4. **尺寸**：宽屏抽屉宽度由 340px 调整为 **480px**（`--panel-w: 480px`，用户确认）；窄屏（`max-width: 640px`）抽屉改为 `width:100vw` 铺满；宽窄屏均为「详情滑入覆盖列表 + 返回」。
5. **未读/头像**：数据源无未读计数，本次不做未读角标；头像用 id 首字母色块（无图片资源）。

---

## UC-0001（静态）Chat 区块具备两级视图结构与输入框

**前置**：`hub/gateway/webui_static.go` 为当前分支版本。

**操作**：检查 `webIndexHTML` 中 Chat 区块（`#chatBody`）的 DOM 结构。

**预期**：
- 存在会话列表视图容器（如 `#chatListView`）与其列表挂载点（`#boardConvs`）；
- 存在会话详情视图容器（如 `#chatDetailView`），含返回按钮（如 `#chatBack`）、标题（如 `#chatTitle`）、消息流挂载点（`#boardMsgs`）；
- 存在发言表单（如 `#chatForm` + 输入框 `#chatInput` + 发送按钮）；
- 详情视图默认隐藏，列表视图默认可见（`showChatView('list')` 语义）。

**验证方式**：`grep` 上述 id 与 `showChatView` 关键字；浏览器 DOM 核对。

---

## UC-0002（静态）样式覆盖气泡 / 滑入动画 / 窄屏铺满

**前置**：同上。

**操作**：检查 `<style>` 中的 Chat 相关规则。

**预期**：
- 存在消息气泡规则（如 `.chat-row` / `.chat-row.me` / `.chat-bubble`），且 `.me` 通过 `flex-direction: row-reverse` 或 `margin-left:auto` 使自己的消息靠右；
- 存在视图切换动画（`slide-in`/`slide-out` 或 transform 过渡）；
- 存在窄屏媒体查询使抽屉铺满：`@media (max-width:640px)` 内 `#agentPanel { width:100vw; }`；
- 原有 `.board-msg`（旧卡片样式）已随详情视图改造一并移除或不再被引用。

**验证方式**：`grep` CSS 规则；浏览器开发者工具核对计算样式。

---

## UC-0003（单测）`Board.Send` 支持显式会话归属，人类发言不新开会话

**前置**：`hub/gateway/board_test.go`（沿用 `newTestBoard` 与 `fakeAgentLink`）。

**操作**：构造含 agent `a`、`b` 的 Board，先由 `a` 向 `["b"]` 发一条消息（生成会话 `a+b`）；再以 `From:"user"`、`To:["a","b"]`、`Conv:"a+b"`、`NoReply:true` 调用 `Send`。

**预期**：
- 第二次 `Send` 无错误，返回的 `msg.Conv == "a+b"`（而非推导出的 `a+b+user`）；
- `Conversations()` 中会话 key 集合仍为 `{a+b}`，不出现 `a+b+user`；
- `Messages("a+b", 0, 0)` 返回 2 条，末条 `From=="user"`；
- `fakeAgentLink` 收到的 `board_message` 帧含该条消息（`from=user`）。

**验证方式**：新增 Go 单测并在 `hub/` 目录运行 `go test ./gateway/ -run TestBoardSendExplicitConv`。

---

## UC-0004（单测）发送接口的错误码与语义

**前置**：`hub/gateway/webui_board.go` 新增 handler 后。

**操作**：用 `httptest` 分别请求：
1. 合法 body（`{"content":"hello"}`）→ 200，返回 `{"message":{...}}`；
2. 空 content / 纯空白 → 400；
3. 超过 `MaxContentBytes`（8KB）→ 400；
4. 未知会话 key（`Messages()` 为空且 board 中无此会话）→ 404；
5. `board == nil`（未挂载公告板）→ 503。

**预期**：状态码与上述一致；错误响应为 JSON 且含 `error` 字段（不泄露内部实现细节）。

**验证方式**：新增 Go 单测（`httptest` + `newTestBoard`），`go test ./gateway/ -run TestBoardPostMessage`。

---

## UC-0005（运行时）宽屏：对话列表 → 详情（覆盖滑入 + 返回 + 气泡）

**前置**：以 hub（`co-shell-hub`，`--serve` 等价能力）启动 Web UI；调试浏览器打开 hub 首页；抽屉已打开并切到 Chat 区块；窗口 ≥900px。

**操作**：打开 Chat 区块 → 观察第一级列表 → 点击某个会话 → 观察详情视图 → 点击返回。

**预期**：
- 第一级为**会话列表**：每项显示会话标题（`全部（公告）` 或 `a + b`）、最后一条消息摘要、时间（`MM-DD HH:MM`）；广播会话（`*`）置顶；
- 点击会话后**列表不再可见**，详情视图整屏滑入覆盖（含返回按钮 + 会话标题）；
- 详情内消息为**气泡**：`from` 名 + 时间 + 内容；`from == "user"`（自己）的气泡靠右并高亮；他人的气泡靠左；
- 原有状态徽标（已受理/已拒绝/待补充/等待 Ns/免回复）仍正确显示；
- 点击返回 → 回到列表，且列表无横向/纵向溢出。

**验证方式**：浏览器截图 + `browser_evaluate` 读取 `.chat-row.me` 数量、`getBoundingClientRect()`、视图可见性。

---

## UC-0006（运行时）用户在 hub 内发言：消息落库、上屏、列表摘要更新、agent 收帧

**前置**：存在至少 1 个已注册（在线）agent；已进入某会话详情。

**操作**：在底部输入框输入文本 → 点击发送（或回车）。

**预期**：
- 输入框被清空；新消息以「我」的右侧气泡出现在详情末尾，时间正确；
- 发送后列表项的最后消息摘要与时间更新为刚发送内容；
- 返回列表再进入该会话，消息仍存在（已落库）；
- agent 侧收到 `board_message` 帧（`from=user`、`to` 为该会话参与者；广播会话 `to=["*"]` 展开）；
- 空内容不发送（按钮禁用或提示），超长（>8KB）返回错误提示且不清空用户输入。

**验证方式**：浏览器操作 + hub 日志/`curl` 复核 `GET /api/board/conversations/{key}/messages`；agent 收帧以 UC-0003 单测为准。

---

## UC-0007（运行时）窄屏（移动端）：抽屉铺满、进入详情、返回列表

**前置**：浏览器视口设为 375×812（或 `--serve` 后用设备模拟）。

**操作**：点击左侧边缘/徽标打开抽屉 → 抽屉占满宽度 → 打开 Chat → 点击会话 → 点击返回。

**预期**：
- 抽屉宽度 = 视口宽度（100vw），无横向滚动条；
- 第一级列表占一屏；点击会话后**整体转入**详情视图（列表不可见），与微信移动端一致；
- 返回按钮可见且可点击，返回后回到列表；
- 输入框在窄屏下可见、可输入、发送可用（不被底部安全区/键盘遮挡到不可用的程度）。

**验证方式**：`browser_evaluate` 读取 `#agentPanel.getBoundingClientRect().width` 与 `window.innerWidth`，比较相等；截图核对。

---

## UC-0008（运行时/回归）Agents 区块与抽屉交互不受影响

**前置**：同上。

**操作**：切换 Agents / Chat 区块；点击 Agent 卡片切换 iframe；测试抽屉开合（含 pin 钉住）。

**预期**：
- Agents 区块的列表、状态灯、开关、新建/设置弹窗行为与改造前一致；
- Chat 区块折叠/展开正常，展开时按 UC-0005 展示；
- 抽屉在窄屏下铺满不影响 Agents 区块使用（列表可滚动、底部按钮可见）。

**验证方式**：浏览器逐项操作核对；`go build ./... && go vet ./...` 与 `go test ./gateway/` 全绿。

---

## 实测记录（已回填，2026-09-18 / BUILD-1076）

| 用例 | 结果 | 证据 |
|------|------|------|
| UC-0001 静态结构 | ✅ 通过 | `webui_static.go`：`#chatListView`（含 `#boardConvs`）与 `#chatDetailView`（`#chatBack`/`#chatTitle`/`#chatSub`/`#boardMsgs`/`#chatErr`/`#chatForm`+`#chatInput`+`#chatSend`）齐备；`renderConvs` 行 `onclick → openConv()`（L825-827）、`openConv` 调 `showChatView('detail')`+`loadMessages`（L830-841）、`chatBack.onclick → showChatView('list')`（L896）、`chatForm.onsubmit → POST`（L881-895） |
| UC-0002 静态样式 | ✅ 通过 | CSS 含 `--panel-w:480px`、`.chat-view` 的 `hidden/slide-in/slide-out`、`.chat-row.me{flex-direction:row-reverse}`、`.chat-bubble`、`.chat-compose`、`.chat-compose .btn.primary{flex:none;width:auto}`，及 `@media (max-width:640px){#agentPanel{width:100vw}}` |
| UC-0003 单测：显式会话归属 | ✅ 通过 | `go test ./hub/gateway/`（含 `board_user_post_test.go`）全绿：`SendRequest.Conv` 非空时直接使用该会话 key，不新建 `a+b+user`；广播 `to=["*"]` |
| UC-0004 单测：接口错误码 | ✅ 通过 | `go test ./hub/gateway/`（含 `webui_board_post_test.go`）全绿：503（board 未挂载）/404（会话不存在）/400（空内容、超长、非法 JSON）/200（`{"message":{...}}`） |
| UC-0005 宽屏两级 + 气泡 | ✅ 通过（stub 数据运行时） | 临时 hub（`127.0.0.1:28290`）+ 注入伪造 board 数据：面板宽 480px；列表 2 行（「全部（公告）」「agent-a · agent-b」）含标题/摘要/时间/头像首字，广播会话置顶；点击后 `listHidden=true`、`detailVisible=true`、标题正确、`#chatSub=共 3 条`；3 条气泡（agent-a 左、**我 右 + `row-reverse` + 青色底**、agent-b 左带「已受理」徽标）；无横向溢出；点返回后回到列表 |
| UC-0006 用户发言 | ⚠️ 部分通过（后续项需用户实测） | 已验证：输入框提交路径打通（`chatForm.onsubmit` → `POST /api/board/conversations/{key}/messages`，200 后清空输入、刷新列表与详情）；宽屏下输入框 409px / 发送按钮 48px 不挤压；空内容不提交；错误码由 UC-0004 单测覆盖。**未验证：真实 agent 是否收到 `board_message` 帧并回复**（无真实 agent 环境，建议用户实测） |
| UC-0007 窄屏整屏切换 | ✅ 通过（375px 视口） | `narrowMatch=true`、`#agentPanel` 宽 375 = `innerWidth`（100vw，无横向滚动）；进入会话后列表不可见、返回后 `backToList=true`；输入框与发送按钮仍可用 |
| UC-0008 回归 | ✅ 通过 | `hub/` 与根目录 `go build ./... && go vet ./...` 通过；`go test ./hub/gateway/` 通过；Agents 区块与抽屉开合路径未改动；过程中修掉 2 个真实 bug：`badge(state)` 与顶部 `var badge` 同名冲突（改名 `stateBadge`）、发送按钮 `width:100%` 挤压输入框 |

## 已确认口径（用户答复）

1. 人类发言者显示名为「我」（id=`user`），且**不强制 agent 回复**（`NoReply=true`，不产生等待窗口）——✅ 已确认。
2. 宽屏抽屉宽度由 340px 调整为 **480px**（`--panel-w: 480px`）——✅ 已确认（已实现）。
3. 本次不做未读角标（数据源无未读计数）——✅ 已确认。
4. 切换方式：**覆盖式**（详情滑入覆盖列表 + 返回按钮），宽窄屏一致——✅ 已确认。
5. 窄屏（`max-width: 640px`）抽屉铺满 `100vw`，点击会话整体转入详情——✅ 已确认。

## 未验证项（需用户实测）

1. **真实 agent 收帧与回复**：本次自测为「单测 + 注入 stub 数据的运行时界面验证」，没有真实在线 agent 环境，因此「用户在 hub 发言 → agent 收到 `board_message` → agent 回复上屏」的全链路未能端到端验证，建议用户用真实 agent 实测一次。
2. 宽屏下同一会话内「等待 Ns / 免回复」徽标与真实 pending 数据的联动（当前仅用 stub 数据核对过渲染分支）。

