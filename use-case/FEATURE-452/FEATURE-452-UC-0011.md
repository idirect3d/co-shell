# FEATURE-452-UC-0011 系统提示词明确 track_task_progress 定计划 + meta.progress 更新

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 意图暴露已启用（intent-exposure-enabled，默认 true）

## 操作步骤
1. 查看系统提示词中关于任务进度跟踪的说明
2. 确认是否明确"用 track_task_progress 设定初始计划，用其他工具调用的 meta.progress 更新跟踪任务执行状态"

## 预期结果
- 系统提示词（`KeySystemPromptToolUsageTaskProgress` 或 meta 对象说明）明确：
  - 初始计划用 `track_task_progress` 建立
  - 执行中用其他工具调用的 `meta.progress` 增量更新任务执行状态
- 中英文（zh/en）提示词均包含此说明

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
