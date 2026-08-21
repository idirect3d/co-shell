# FEATURE-398 右上角菜单增加"重启后台"菜单项

## 背景

Web UI 右上角菜单中，系统设置下面需要增加"重启后台"菜单项，与系统设置之间增加分隔线。功能：发送重启信号通知外部 supervisor 重启进程。

## 验收标准

1. 右上角菜单中，系统设置下面增加"重启后台"菜单项。
2. "重启后台"与"系统设置"之间增加分隔线。
3. 点击"重启后台"菜单项，发送 `{ type: "restart" }` WebSocket 消息。
4. 后端收到 restart 消息后，向当前进程发送 SIGHUP 信号（由外部 supervisor 捕获后重启进程）。
5. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 菜单项位置正确
- 前置：Web UI 已加载，打开右上角下拉菜单
- 步骤：检查菜单结构
- 期望：系统设置下面有"重启后台"菜单项，两者之间有分隔线

### UC-002 点击发送 restart 消息
- 前置：Web UI 已加载，打开右上角下拉菜单
- 步骤：点击"重启后台"菜单项
- 期望：发送 `{ type: "restart" }` WebSocket 消息

### UC-003 后端处理 restart 消息
- 前置：Web UI 已连接
- 步骤：发送 `{ type: "restart" }` 消息
- 期望：后端向当前进程发送 SIGHUP 信号

### UC-004 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
