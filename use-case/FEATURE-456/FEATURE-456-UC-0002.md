# FEATURE-456-UC-0002 三个介入点（A/B/C）开关

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 已启用（`supervisor.enabled = true`）

## 操作步骤
1. 查看配置中三个介入点开关：`supervisor.entryA`（attempt_completion）、`supervisor.entryB`（未调用工具自动退出）、`supervisor.entryC`（任务进度标记完成）
2. 确认默认值：A 默认开、B 默认开、C 默认关
3. **介入点 A**：让主 LLM 调用 `attempt_completion`，观察是否触发监督审查
4. **介入点 B**：让主 LLM 不调用任何工具直接输出最终答案（`noToolAction=exit` 或 attempt_completion 不可用），观察是否触发监督审查
5. **介入点 C**：让主 LLM 调用 `track_task_progress` 且将某个步骤标记为 completed，观察是否触发监督审查（默认 C 关闭，应不触发）
6. 将 `supervisor.entryC` 设为 true，重复步骤 5，观察是否触发监督审查

## 预期结果
- A 默认开：调用 `attempt_completion` 触发监督审查
- B 默认开：未调用工具自动退出触发监督审查
- C 默认关：`track_task_progress` 标记完成**不**触发监督审查
- C 设为 true 后：`track_task_progress` 标记完成触发监督审查

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
