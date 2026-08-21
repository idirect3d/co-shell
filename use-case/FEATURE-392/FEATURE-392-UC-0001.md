# FEATURE-392 修正 Web UI 设置面板分类与顺序

## 背景

FEATURE-391 的 Web UI 设置面板分类与 TUI `:set` 输出（`showSettingsHelp()`）不一致。本任务重写 `cmd/settings_web.go` 的 `SettingsJSON()`，使其分组和顺序与 `showSettingsHelp()` 完全一致，但去掉 Group 1 [身份与个性] 分组。

## 验收标准

1. `SettingsJSON()` 返回 5 组，分组标题与顺序为：
   - ① [智能体设置]（`KeySettingsGroupModel`）
   - ② [显示与输出]（`KeySettingsGroupDisplay`）
   - ③ [安全与确认]（`KeySettingsGroupSafety`）
   - ④ [记忆与上下文]（`KeySettingsGroupMemory`）
   - ⑤ [开发者]（`KeySettingsGroupSearchDebug`）
2. 每组内设置项的顺序与 `showSettingsHelp()` 完全一致。
3. 不包含 [身份与个性] 分组（name/description/principles/mode/capabilities/principles-file/rules）。
4. 去掉不在 TUI 输出中的多余项：top-p、top-k、repetition-penalty、max-model-len、max-retries、read-file-max-size、duplicate-content-threshold、loop-temp-enabled。
5. 不包含只读展示项 current-tool-model/current-vision-model/current-problem-model 和特殊项 output-categories/db/llm-log。
6. 删除不再使用的 `agentNameValue` 辅助函数。
7. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 分组数量与标题
- 前置：无
- 步骤：调用 `SettingsJSON()`
- 期望：返回 5 组，标题依次为 [智能体设置]、[显示与输出]、[安全与确认]、[记忆与上下文]、[开发者]

### UC-002 智能体设置组顺序
- 前置：无
- 步骤：检查第 1 组 items 的 key 顺序
- 期望：temperature, max-tokens, max-iterations, vision, vision-context-mode, thinking-enabled, reasoning-effort, toolcall-enabled, toolcall-mode, xml-tag-prefix, xml-stream-validate, plan-enabled, subagent-enabled, result-mode, shell-session-enabled, shell-session-timeout, shell-vt-rows, shell-vt-cols, browser-enabled, browser-port, browser-headless, browser-max-html-size, excel-max-sessions, excel-max-cells, docx-max-sessions, docx-max-read-paras, visual-analysis-max-images, search-max-line-length, search-max-result-bytes, search-context-lines, no-tool-action, parse-error-action

### UC-003 显示与输出组顺序
- 前置：无
- 步骤：检查第 2 组 items 的 key 顺序
- 期望：emoji-enabled, show-llm-thinking, show-llm-content, show-tool, show-tool-input, show-tool-output, show-command, show-command-output, show-loop-detection, show-parse-error-raw, token-usage

### UC-004 安全与确认组顺序
- 前置：无
- 步骤：检查第 3 组 items 的 key 顺序
- 期望：confirm-tool, tool-timeout, cmd-timeout, llm-timeout, error-max-single-count, error-max-type-count, loop-intervention, loop-auto-reorganize-threshold, loop-detect-threshold, loop-temp-step-up, loop-temp-step-down, loop-temp-max, loop-temp-min, loop-judge-enabled, loop-judge-timeout, loop-long-output-threshold, loop-single-line-length, loop-single-line-window, loop-single-line-block-limit, problem-solver-enabled, default-problem-model, default-tool-model

### UC-005 记忆与上下文组顺序
- 前置：无
- 步骤：检查第 4 组 items 的 key 顺序
- 期望：memory-enabled, context-limit, context-policy, context-reorganize-threshold, memory-search-max-content-len, memory-search-max-results

### UC-006 开发者组顺序
- 前置：无
- 步骤：检查第 5 组 items 的 key 顺序
- 期望：debug, log

### UC-007 不包含身份与个性分组
- 前置：无
- 步骤：遍历所有组的 items
- 期望：不包含 name/description/principles/mode/capabilities/principles-file/rules 任一 key

### UC-008 不包含多余项
- 前置：无
- 步骤：遍历所有组的 items
- 期望：不包含 top-p/top-k/repetition-penalty/max-model-len/max-retries/read-file-max-size/duplicate-content-threshold/loop-temp-enabled 任一 key

### UC-009 不包含只读/特殊项
- 前置：无
- 步骤：遍历所有组的 items
- 期望：不包含 current-tool-model/current-vision-model/current-problem-model/output-categories/db/llm-log 任一 key

### UC-010 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警
