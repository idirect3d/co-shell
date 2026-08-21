# FEATURE-397 Web UI 拦截 ESC 按键触发暂停

## 背景

Web UI 中暂停（interrupt）只能通过点击 ⏸ 按钮触发（发送 `{ type: "interrupt" }` 消息），ESC 键未被拦截。希望按 ESC 也能触发暂停，与终端行为一致。

## 验收标准

1. Web UI 中，按 ESC 键且当前正在运行（running）时，发送 `{ type: "interrupt" }` 消息，等价于点击 ⏸ 按钮。
2. 交互 pending 时（如工具确认、暂停确认弹窗），按 ESC 不触发暂停（避免与虚拟键盘冲突）。
3. 输入框聚焦时按 ESC 也触发暂停（全局 ESC 都暂停）。
4. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 运行中按 ESC 触发暂停
- 前置：Web UI 已加载，agent 正在运行（running = true）
- 步骤：按 ESC 键
- 期望：发送 `{ type: "interrupt" }` 消息，agent 暂停

### UC-002 空闲时按 ESC 不触发
- 前置：Web UI 已加载，agent 空闲（running = false）
- 步骤：按 ESC 键
- 期望：不发送 interrupt 消息

### UC-003 交互 pending 时按 ESC 不触发暂停
- 前置：Web UI 已加载，有 pendingInteraction（如工具确认弹窗）
- 步骤：按 ESC 键
- 期望：不发送 interrupt 消息（保持虚拟键盘的 ESC 处理）

### UC-004 输入框聚焦时按 ESC 触发暂停
- 前置：Web UI 已加载，agent 正在运行，焦点在主输入框
- 步骤：按 ESC 键
- 期望：发送 `{ type: "interrupt" }` 消息，agent 暂停

### UC-005 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
