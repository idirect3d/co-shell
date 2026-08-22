# FEATURE-405 改进主消息录入框高度

## 背景

FEATURE-404 缩小 padding 后滚动条仍出现。用户发现 autoGrow 自动增长高度（`input.style.height = Math.min(input.scrollHeight, 120)`），但文本框高度仍不足。建议适当增加累积单位高度/文本框高度。

## 验收标准

1. `autoGrow` 高度上限从 120px 增加到 150px。
2. `autoGrow` 给 scrollHeight 加增量（+4px）。
3. CSS `#input` 的 `max-height` 从 120px 增加到 150px。
4. 主消息录入框在无内容/单行/多行未超限时不出现滚动条。
5. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 autoGrow 高度上限增加
- 前置：Web UI 已加载
- 步骤：检查 autoGrow 函数
- 期望：高度上限从 120px 增加到 150px

### UC-002 autoGrow 加增量
- 前置：Web UI 已加载
- 步骤：检查 autoGrow 函数
- 期望：scrollHeight 加增量（+4px）

### UC-003 CSS max-height 增加
- 前置：Web UI 已加载
- 步骤：检查 #input 样式
- 期望：max-height 从 120px 增加到 150px

### UC-004 无内容/单行时不出现滚动条
- 前置：Web UI 已加载
- 步骤：观察录入框
- 期望：无内容/单行时不出现滚动条

### UC-005 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
