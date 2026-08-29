# FEATURE-452-UC-0002 attempt_completion 增加 meta 参数（XML 模式）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 XML 工具调用模式（`--tool-call-mode xml`）
- 意图暴露已启用（intent-exposure-enabled，默认 true）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具，**携带** `meta` 参数（含 intent/risk/risk_reason/affected_objects/progress）
2. 观察 XML 工具使用模板（`KeyToolUsageAttemptCompletion`）中是否声明了 meta 参数
3. 观察 XML 示例中是否包含 `<meta>` 对象

## 预期结果
- XML 工具使用模板中 `attempt_completion` 声明了 meta 参数（放最前）
- XML 示例中包含 `<meta>` 对象（含 intent/risk/risk_reason/affected_objects/progress）
- 携带 meta 参数时工具正常执行
- `track_task_progress` 的 XML 模板**不**声明 meta 参数

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
