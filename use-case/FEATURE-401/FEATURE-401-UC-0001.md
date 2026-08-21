# FEATURE-401 左下角 logo 与消息框之间增加"+"号按钮（创建新会话）

## 背景

Web UI 左下角 co-shell logo 和主消息框之间需要增加一个大一点的"+"号按钮（与 logo 和消息框都留 5-10 像素间距），点击创建新会话。

## 验收标准

1. 左下角 co-shell logo 和主消息框之间有一个"+"号按钮。
2. "+"号按钮与 logo 和消息框都留 5-10 像素间距。
3. 点击"+"号按钮，发送 `{ type: "session_new" }` WebSocket 消息。
4. 后端收到 session_new 消息后，创建新会话（复用 `:new` 命令逻辑）。
5. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 + 号按钮位置正确
- 前置：Web UI 已加载
- 步骤：检查底部布局
- 期望：左下角 logo 和主消息框之间有"+"号按钮

### UC-002 + 号按钮间距正确
- 前置：Web UI 已加载
- 步骤：检查"+"号按钮与 logo、消息框的间距
- 期望：与 logo 和消息框都留 5-10 像素间距

### UC-003 点击发送 session_new 消息
- 前置：Web UI 已加载
- 步骤：点击"+"号按钮
- 期望：发送 `{ type: "session_new" }` WebSocket 消息

### UC-004 后端创建新会话
- 前置：Web UI 已连接
- 步骤：发送 `{ type: "session_new" }` 消息
- 期望：后端创建新会话（生成新 sessionID + 创建 SessionEntry + 切换当前会话）

### UC-005 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
