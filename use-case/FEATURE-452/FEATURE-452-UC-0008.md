# FEATURE-452-UC-0008 attempt_completion 无 next_steps 时只有三个固定选项

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 弹框开关开启（默认）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具，**不携带** `next_steps` 参数
2. 弹出"下一步建议"交互选择框
3. 观察选择框的选项列表

## 预期结果
- 选择框**不显示** next_steps 选项（因为 LLM 未提供）
- 选择框只显示三个固定选项："给出下一步的建议"、"任务尚未达到目标"（+）、"完成退出"（-）
- 用户仍可通过三个固定选项进行选择

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
