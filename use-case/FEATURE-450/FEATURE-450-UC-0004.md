# FEATURE-450-UC-0004 统一合法性校验以必需清单为准（track_task_progress 不要求 meta）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 意图暴露开关开启（intent-exposure-enabled，默认 true）

## 操作步骤
1. 让 LLM 调用 `track_task_progress`，**不携带** `meta` 参数
2. 观察 `run_stream.go` 中统一合法性校验（`assessRisk`→`validateMeta`）是否对 `track_task_progress` 跳过 meta 校验
3. 同时让 LLM 调用一个普通工具（如 `read_file`），**携带**完整 meta 参数

## 预期结果
- `track_task_progress` 不携带 meta 时，统一合法性校验不报错（因其必需清单不含 meta）
- 普通工具（如 `read_file`）仍要求完整 meta 参数（其必需清单含 meta），缺失时报错
- 统一合法性校验以各工具必需清单为准，而非对所有工具一律要求 meta

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
