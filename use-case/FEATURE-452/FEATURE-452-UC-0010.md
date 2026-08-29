# FEATURE-452-UC-0010 ask_followup_question 的 - 传回 LLM 让 LLM 自己退出

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `ask_followup_question` 工具，携带 `question` 和 `options`
2. 弹出选择框
3. 用户输入 `-`（对应"我要再想想，先退出"）
4. 观察任务行为

## 预期结果
- 用户输入 `-` 后，任务**不直接退出**，把用户选择"我要再想想，先退出"传回 LLM
- LLM 收到后自行决定退出（如调用 attempt_completion 或结束）
- 与 attempt_completion 的 `-`（完成退出，直接退出）行为不同

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
