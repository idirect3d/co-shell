# FEATURE-396 去掉暂停终止时提示框内无用的 [1]-[9] 批准次数提示

## 背景

Web UI 中，ESC 暂停终止（中断确认）时提示框内多了一个 [1]-[9] 批准次数提示，但该场景没有 approve_count（批准N次）功能，提示无用。根源：前端 `renderVirtualKeyboard` 对所有非 select 交互都无条件把数字键 0-9 映射为 approve-count 并渲染 [1]-[9] 项，而 ESC 中断确认（run_stream.go 的 InteractionConfirm）只包含 Enter 继续 / c 取消两个键，没有 approve_count 相关键。

## 验收标准

1. ESC 暂停终止（中断确认）时，提示框内不再显示 [1]-[9] 批准次数提示。
2. 工具确认放行（promptToolConfirmation，含 approve_count 功能）时，仍显示 [1]-[9] 批准次数提示。
3. 提问选择（select）交互时，数字键仍映射到选项（1..N）。
4. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 暂停终止不显示 1-9 提示
- 前置：Web UI 已加载，LLM 输出中按 ESC 触发中断确认
- 步骤：观察提示框
- 期望：提示框内不显示 [1]-[9] 批准次数提示，仅显示 Enter 继续、c 取消

### UC-002 工具确认放行仍显示 1-9 提示
- 前置：Web UI 已加载，触发工具确认放行（promptToolConfirmation）
- 步骤：观察提示框
- 期望：提示框内仍显示 [1]-[9] 批准次数提示（该场景有 approve_count 功能）

### UC-003 提问选择数字键映射选项
- 前置：Web UI 已加载，触发提问选择（select）交互
- 步骤：观察提示框
- 期望：数字键 1..N 映射到选项，点击数字键选择对应选项

### UC-004 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
