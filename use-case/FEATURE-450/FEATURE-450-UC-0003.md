# FEATURE-450-UC-0003 track_task_progress 去掉 meta 参数（XML 模式）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 XML 工具调用模式（`--tool-call-mode xml`）
- 任务计划功能已启用（plan enabled）

## 操作步骤
1. 让 LLM 以 XML 方式调用 `track_task_progress`，**不携带** `<meta>` 标签，仅携带 `<title>`/`<description>`/`<steps>`
2. 观察 XML 解析与工具执行是否成功

## 预期结果
- XML 模式工具使用说明（`KeyToolUsageTrackTaskProgress`）中不再声明 meta 参数
- XML 必需清单校验（`toolcall_mode.go`）不再要求 meta 参数
- 不携带 `<meta>` 标签时工具正常执行，不报"缺少必需参数 meta"错误
- 任务计划成功创建

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
