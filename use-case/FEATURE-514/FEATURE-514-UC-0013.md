# FEATURE-514-UC-0013 任务计划支持结构化验收标准

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）

## 操作步骤
1. 让主 LLM 调用 `track_task_progress`，在参数中传入 `acceptance_criteria: ["构建通过", "Web UI 显示验收标准"]`
2. 查看返回的任务计划 JSON 与前端任务进展展示

## 预期结果
- 任务计划对象包含 `acceptance_criteria` 数组，内容与传入一致
- 终端 `.list tasks`（或等价的展示路径）与 Web UI 任务进展均能看到验收标准
- 不传 `acceptance_criteria` 时字段为空且不报错（向后兼容）
- 单元测试 `TestTaskPlan_AcceptanceCriteria` 覆盖

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
