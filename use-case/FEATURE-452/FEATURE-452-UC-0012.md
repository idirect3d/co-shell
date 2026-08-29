# FEATURE-452-UC-0012 工具调用示例补全 meta 参数

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 意图暴露已启用（intent-exposure-enabled，默认 true）

## 操作步骤
1. 查看系统提示词中所有工具调用示例（OpenAI 模式 + XML 模式）
2. 确认每个工具的示例是否都携带完整的 meta 参数（intent/risk/risk_reason/affected_objects/progress）

## 预期结果
- 所有要求 meta 的工具，其调用示例都携带完整的 meta 参数
- `attempt_completion` 的示例携带 meta 参数（本任务新增）
- `track_task_progress` 的示例不携带 meta 参数（保持 FEATURE-450 决策）
- 中英文（zh/en）示例均一致

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
