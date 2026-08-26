# FEATURE-439 测试用例

## 概述

FEATURE-439 引入 YOLO（You Only Live Once）模式总开关：在工具调用确认入口做总开关，开启后跳过所有人为判断直接执行所有工具调用。CLI 通过 `:YOLO`（必须大写）命令 toggle，Web UI 通过右下角古典拨动开关控制。启动默认关，状态不持久化。

## 测试环境

- 分支：FEATURE-439
- 版本：v0.18.0
- 编译：`go build ./... && go vet ./... && go build -o work/co-shell .`

---

## UC-0001 后端 YOLO 开关默认关闭

**前置**：新建 Agent 实例
**操作**：调用 `agent.IsYOLO()`
**预期**：返回 `false`（默认关闭）

## UC-0002 后端 SetYOLO/IsYOLO 设置与读取

**前置**：新建 Agent 实例
**操作**：`agent.SetYOLO(true)` 后调用 `agent.IsYOLO()`
**预期**：返回 `true`
**操作**：`agent.SetYOLO(false)` 后调用 `agent.IsYOLO()`
**预期**：返回 `false`

## UC-0003 YOLO 开启后 confirm 工具跳过确认直接执行

**前置**：Agent 的 execute_command 工具模式为 confirm（默认），YOLO 开启
**操作**：触发 execute_command 工具调用
**预期**：不弹出确认提示，直接执行

## UC-0004 YOLO 关闭时 confirm 工具仍需确认

**前置**：Agent 的 execute_command 工具模式为 confirm（默认），YOLO 关闭
**操作**：触发 execute_command 工具调用
**预期**：弹出确认提示，等待用户确认

## UC-0005 YOLO 开启不影响 disabled 工具

**前置**：某工具（如 execute_command）被设为 disabled，YOLO 开启
**操作**：检查该工具是否出现在 LLM 工具列表中
**预期**：disabled 工具不出现在 LLM 工具列表中（LLM 看不到，不会调用），不受 YOLO 影响

## UC-0006 CLI :YOLO 命令 toggle 并显示状态

**前置**：REPL 环境，YOLO 默认关闭
**操作**：输入 `:YOLO`
**预期**：YOLO 开启，显示"YOLO 模式已开启（所有工具调用自动批准）"
**操作**：再次输入 `:YOLO`
**预期**：YOLO 关闭，显示"YOLO 模式已关闭（工具调用需确认）"

## UC-0007 CLI :yolo（小写）不识别

**前置**：REPL 环境
**操作**：输入 `:yolo`（小写）
**预期**：显示未知命令（:YOLO 必须大写）

## UC-0008 Web UI 拨动开关默认 OFF

**前置**：启动 Web UI，连接 WebSocket
**操作**：页面加载后发送 `yolo_get`
**预期**：返回 `yolo: false`，开关显示 OFF（拨杆向下）

## UC-0009 Web UI 点击开关开启 YOLO

**前置**：Web UI 已连接
**操作**：点击 YOLO 拨动开关
**预期**：发送 `yolo_set: true`，后端返回 `yolo: true`，开关显示 ON（拨杆向上，橙红色警示色）

## UC-0010 Web UI 点击开关关闭 YOLO

**前置**：Web UI 已连接，YOLO 已开启
**操作**：再次点击 YOLO 拨动开关
**预期**：发送 `yolo_set: false`，后端返回 `yolo: false`，开关显示 OFF（拨杆向下）

## UC-0011 YOLO 状态不持久化

**前置**：YOLO 已开启
**操作**：重启程序
**预期**：YOLO 恢复为关闭（默认关，状态不持久化）

---

## 单元测试清单

| 用例 | 测试文件 | 测试函数 |
|------|---------|---------|
| UC-0001/0002 | agent/yolo_test.go | TestYOLODefaultOff / TestYOLOSetGet |
| UC-0003/0004 | agent/yolo_test.go | TestYOLOConfirmSkip |
| UC-0005 | agent/yolo_test.go | TestYOLODisabledUnaffected |
| UC-0006/0007 | repl/repl_test.go | TestYOLOCommand |
