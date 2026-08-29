# FEATURE-452-UC-0003 attempt_completion 弹框开关（默认开启，config 可关闭）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 默认配置下，让 LLM 调用 `attempt_completion` 工具（携带 result/session_title/session_keywords）
2. 观察是否弹出"下一步建议"交互选择框
3. 在 config 中关闭弹框开关（如 `attempt_completion_confirm: false`）
4. 再次让 LLM 调用 `attempt_completion`，观察是否直接退出（不弹框）

## 预期结果
- 默认配置（开关开启）下，`attempt_completion` 弹出"下一步建议"交互选择框
- 关闭开关后，`attempt_completion` 直接退出（保持 FEATURE-450 之前的行为），不弹框
- 开关可通过 config 配置

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
