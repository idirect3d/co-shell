# FEATURE-450-UC-0002 attempt_completion 去掉 meta 参数（OpenAI 模式）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具，**不携带** `meta` 参数，仅携带 `result`/`session_title`/`session_keywords`
2. 观察工具调用是否成功执行并完成任务

## 预期结果
- `attempt_completion` 工具定义中不再声明 `meta` 参数
- `attempt_completion` 的必需清单（required）中不再包含 `meta`
- 不携带 meta 参数时工具正常执行，不报"meta object is required"错误
- 任务正常完成

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
