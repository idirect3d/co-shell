# FEATURE-452-UC-0001 attempt_completion 增加 meta 参数（OpenAI 模式）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）
- 意图暴露已启用（intent-exposure-enabled，默认 true）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具，**携带** `meta` 参数（含 intent/risk/risk_reason/affected_objects/progress）
2. 观察工具定义中是否声明了 `meta` 参数
3. 观察 `attempt_completion` 的必需清单（required）中是否包含 `meta`

## 预期结果
- `attempt_completion` 工具定义中声明了 `meta` 参数（放最前）
- `attempt_completion` 的必需清单（required）中包含 `meta`
- 携带 meta 参数时工具正常执行
- `track_task_progress` 工具**不**声明 meta 参数（保持 FEATURE-450 决策）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
