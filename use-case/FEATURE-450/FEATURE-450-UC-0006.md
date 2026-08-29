# FEATURE-450-UC-0006 方法声明与系统提示词示例同步更新

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）

## 操作步骤
1. 检查 `agent/tools.go` 中 `track_task_progress` 和 `attempt_completion` 的工具定义
2. 检查 `i18n/en_system.go` / `i18n/zh_system.go` 中 `KeyToolUsageTrackTaskProgress` / `KeyToolUsageAttemptCompletion` 的参数说明
3. 检查 `KeySystemPromptToolUsageMetaOpenAI` / `KeySystemPromptToolUsageMetaXML` 中 meta.progress 的说明
4. 检查 `agent/meta_param_test.go` 的 `TestInjectMetaParamAllTools` 是否已适配（不再要求这两个工具含 meta）

## 预期结果
- `track_task_progress` / `attempt_completion` 工具定义中不再声明 meta 参数，必需清单不含 meta
- i18n 工具使用说明（OpenAI + XML）中不再声明 meta 参数
- meta.progress 说明已更新为"至少提供 1 条当前状态记录（即便状态没变也要提供）"
- `TestInjectMetaParamAllTools` 已适配：这两个工具不再要求 meta 在必需清单中，其他工具仍要求
- `go test ./...` 全绿

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败
