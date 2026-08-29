# FEATURE-450-UC-0005 meta.progress 至少提供 1 条当前状态记录（校验规则）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 已通过 `track_task_progress` 创建任务计划（含多个步骤）

## 操作步骤
1. 让 LLM 调用一个普通工具（如 `read_file`），携带完整 meta 参数，但 `meta.progress` 为**空数组** `[]`
2. 观察工具调用是否报错
3. 让 LLM 调用普通工具，携带 `meta.progress` 为**至少 1 条当前状态记录**（即便状态没变也提供，如 `[{"index":0,"description":"step one","status":"in_progress"}]`）
4. 观察工具调用是否成功

## 预期结果
- `meta.progress` 为空数组时，工具调用报错（提示至少提供 1 条当前状态记录）
- `meta.progress` 提供至少 1 条当前状态记录时，工具调用成功，任务计划状态被更新
- 系统提示词 meta 对象说明（OpenAI + XML）已更新为"至少提供 1 条当前状态记录（即便状态没变也要提供）"

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
