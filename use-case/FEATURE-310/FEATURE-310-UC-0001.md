# FEATURE-310 工具调用意图+个性化摘要显示测试用例

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-310 |
| 任务名称 | 工具调用意图+个性化摘要显示 |
| 类型 | 特性（工具调用显示改进） |
| 版本 | v0.7.0 |
| 依赖 | FEATURE-252（intent 参数）、FEATURE-184（confirm-tool 确认机制）、FEATURE-302（StreamRenderer 渲染管线） |

**目标**：① 新增 `buildToolSummary(toolName, args)` 统一构建"工具调用摘要"（友好工具名 + intent + 主要参数，长内容截断）；② 个性化话术通过 i18n 模板配置（zh/en），不同工具不同话术；③ `showTool` 开启时（执行前与确认提示）显示摘要替代原始工具名/JSON；④ 摘要显示与 show-tool 共享显示控制。

**验收基线**：`go test ./agent/...` 全绿；`go vet ./agent/...` 0 告警；`go build ./...` 通过；新增 i18n key 全部 zh/en 双存在；edit_i18n 审计通过。

---

## 一、编译与回归

- **UC-0001**: `go build ./...` 退出码 0（含 cmd/co-shell-feishu-bridge、cmd/co-shell-hub）。
- **UC-0002**: `go test ./agent/...` 无 FAIL（含既有测试 + 本任务新增测试）。
- **UC-0003**: `go vet ./agent/...` 0 告警（无参 `fmt.Errorf/i18n.TF(key)` 必须改用 `errors.New`；含 % 占位符的保留）。

## 二、buildToolSummary 核心函数（循环模式）

- **UC-0004**: 通用 fallback——无个性化模板的工具名（如 `evaluate_expression`）输出 `[🔧 工具名] 目的: <intent>`；intent 缺失时输出仅含工具名（table-driven：具意图 / 缺意图 / 空参数 3 子用例）。
- **UC-0005**: 文本类工具参数提取（循环：execute_command→command；read_file→path+start_line+end_line；write_to_file→path+mode；search_files→path+regex；list_files→path+recursive；list_code_definition_names→path）。
- **UC-0006**: 长内容截断（循环：write_to_file content 超长截断、replace_in_file replacements 只显示数量、excel_edit values 只显示行列数、excel_format what 数组显示数量、visual_analysis paths 显示数量，每个断言不包含原始长内容、包含 [N 字符/项] 摘要标记）。
- **UC-0007**: shell 类工具（shell_send→command、shell_window_content→仅 intent、shell_get_output→无参、shell_reset→仅 intent）摘要正确。
- **UC-0008**: 文件编辑类（replace_in_file→path+replacements 数量；rename_file→old+new；delete_file→path）摘要正确。
- **UC-0009**: session/文档类（excel_open→path+mode；word_open→path+mode；word_continue→session_id+content 截断）摘要正确。
- **UC-0010**: 零参数工具（view_task_plan、list_settings、browser_screenshot 等仅 intent）摘要只显示工具名+目的。

## 三、i18n 模板配置（循环模式）

- **UC-0011**: 新增 key 双存在断言（循环：每个新增 `KeyToolCallSummary*` key 断言 zh.go + en.go 均存在；audit i18n=1 不增）。
- **UC-0012**: 模板含 `%s` 占位符数量与参数数量一致断言（循环：每个个性化模板逐个格式化不 panic、输出非 key 原文）。
- **UC-0013**: zh/en 模板结构一致——同一工具两个语言模板占位符顺序一致（循环逐个工具断言，防止 zh/en 错位）。

## 四、集成显示行为（模拟 LLM 调用，循环模式）

- **UC-0014**: `showTool` 开启时，`executeToolCall` 确认前显示的 `displayStr` 为 buildToolSummary 摘要（包含 intent），不再为原始 JSON 或裸工具名（模拟 execute_command/read_file/write_to_file 三工具各 1 子用例）。
- **UC-0015**: `showTool=false` 时，确认提示与执行前均不显示摘要（回归 show-tool 关闭行为）。
- **UC-0016**: `run_stream.go` 中 `showTool` 且 `showToolInput` 场景输出为摘要而非原始 JSON（历史行为替换断言）。
- **UC-0017**: `.simulate` 命令模拟 LLM 返回工具调用时，摘要显示管线生效（与正常 LLM 调用同一 executeToolCall 路径）。

## 五、审计收尾

- **UC-0018**: git diff 限 agent/tool_summary.go（新增）+ agent/run_stream.go + agent/tools.go + i18n keys.go/zh.go/en.go + 测试文件；无越界文件。
- **UC-0019**: 全量 `go test ./...` 通过（新增摘要逻辑不破坏既有渲染 golden 测试：render_tui.golden / render_single_cmd.golden）。

## 循环模式

UC-0004（3）+ UC-0005（6）+ UC-0006（5）+ UC-0007（4）+ UC-0008（3）+ UC-0009（3）+ UC-0010（1）+ UC-0011（新增 key ≥10）+ UC-0012（个性化模板 ≥8）+ UC-0013（≥8）+ UC-0014（3）≥ 54 子用例。