# FEATURE-514-UC-0014 监督审查 prompt 包含验收标准段

## 前置条件
- `supervisor_enabled = true`、`supervisor_entry_object = true`
- 当前任务计划含 2 条验收标准

## 操作步骤
1. 触发主 LLM 调用 `attempt_completion`
2. 打开 `show-sup-prompt`，查看送给监督员的用户 prompt（SUP·监督 块的 prompt 段）
3. 或直接单测 `TestBuildSupervisorUserPrompt_IncludesAcceptanceCriteria` 断言

## 预期结果
- prompt 中出现独立的「# 验收标准」段，逐条列出任务计划的验收标准（编号或列表形式）
- 任务计划无验收标准时该段显示占位文本（如「（未定义验收标准）」），不报错

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
