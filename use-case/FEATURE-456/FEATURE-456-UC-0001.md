# FEATURE-456-UC-0001 监督 LLM 启用/禁用开关

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 配置文件中监督 LLM 开关 `supervisor.enabled` 存在

## 操作步骤
1. 查看配置中监督 LLM 的启用/禁用开关（`supervisor.enabled`）
2. 确认默认值是否为**启用**（true）
3. 保持启用状态，让主 LLM 完成一个任务并调用 `attempt_completion`
4. 观察是否触发监督 LLM 审查
5. 将 `supervisor.enabled` 设为 false（禁用）
6. 再次让主 LLM 完成一个任务并调用 `attempt_completion`
7. 观察是否触发监督 LLM 审查

## 预期结果
- `supervisor.enabled` 默认值为 true（启用）
- 启用时，主 LLM 调用 `attempt_completion` 会触发监督 LLM 审查
- 禁用时，主 LLM 调用 `attempt_completion` 不触发监督 LLM 审查，直接走原有流程

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
