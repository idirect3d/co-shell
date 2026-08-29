# FEATURE-452-UC-0004 attempt_completion 弹框 - 用户选"完成退出"（-）直接退出

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 弹框开关开启（默认）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `attempt_completion` 工具（携带 result/session_title/session_keywords，可选 next_steps）
2. 弹出"下一步建议"交互选择框，显示 LLM 的 next_steps（如有）+ 三个固定选项
3. 用户输入 `-`（对应"完成退出"）
4. 观察任务是否直接退出

## 预期结果
- 选择框显示三个固定选项："给出下一步的建议"、"任务尚未达到目标"（+）、"完成退出"（-）
- 用户输入 `-` 后，任务**直接退出**（SetCompleted），与 FEATURE-450 之前的行为一致
- 会话正常保存（session_title/session_keywords 生效）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
