# FEATURE-452-UC-0005 attempt_completion 弹框 - 用户选 next_step 传回 LLM 继续执行

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 弹框开关开启（默认）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具，携带 `next_steps` 参数（如 ["补充单元测试", "更新文档"]）
2. 弹出"下一步建议"交互选择框，显示 next_steps（[1] 补充单元测试、[2] 更新文档）+ 三个固定选项
3. 用户输入 `1`（选择"补充单元测试"）
4. 观察任务是否继续执行（不退出）

## 预期结果
- 选择框显示 LLM 的 next_steps 选项（数字编号 1..N）
- 用户选择某个 next_step 后，任务**不退出**，把用户选择传回 LLM，LLM 继续执行该步骤
- 任务循环继续，LLM 响应后执行"补充单元测试"

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
