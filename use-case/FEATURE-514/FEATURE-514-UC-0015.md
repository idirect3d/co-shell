# FEATURE-514-UC-0015 监督员逐条核对验收标准并在打回时给出问题项

## 前置条件
- `supervisor_enabled = true`
- 任务计划含验收标准 A、B；交付物只满足 A

## 操作步骤
1. 主 LLM 调用 `attempt_completion` 声明完成
2. 观察监督员返回的 `submit_review`（结论/理由/建议）
3. 观察打回后主 LLM 收到的反馈内容

## 预期结果
- 监督员结论为打回（approved=false）
- 反馈中明确指出未达标的验收标准条目（B 未满足）及原因
- 主 LLM 收到该反馈后继续修复；修好后再次 attempt_completion 可放行
- 单元测试 `TestSupervisorReview_CriteriaCheck` 覆盖（含 criteria_check 解析与格式化）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
