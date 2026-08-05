# FEATURE-305: i18n 归零冲刺（P4.5，100% i18n 达成）

## 背景

P3（FEATURE-303 [BUILD-342]）+ P4（FEATURE-304 [BUILD-343]）已完成 B 类向导与 D 类外部入口的 i18n 迁移（audit 1555→771）。当前 HEAD（FIX-329 [BUILD-370]）实测 audit：

- 第 3 项 **Hardcoded Chinese = 782**（P4.5 需归零）
- 第 5 项 **i18n keys missing zh/en = 1**（`KeyToolUsageShellSend`：en_system.go 有、zh_system.go 缺）

782 处按文件分类，主战场：
| 批 | 文件（处数） | 类别 |
|---|------------|------|
| 1 | main.go（87）+ repl/repl.go（28） | C 类基础设施（含约 30 条 `Warning: invalid --xxx` stderr 警告） |
| 2 | cmd/settings_agent（83）+ settings_safety（58）+ settings_llm（38）+ settings.go（24）+ settings_search（21）+ settings_display（19）+ settings_log（5） | B 类 :set 向导家族（共 248） |
| 3 | agent/settings_tools.go（96）+ tools.go（28）+ vault.go（5） | 代理设置/确认框 |
| 4 | agent/excel_tools（48）+ docx_tools（15） | 文档编辑工具返回给 LLM 的 result 文案 |
| 5 | agent/browser_tools（19）+ file_tools（13）+ image_tools（8）+ taskplan_tools（5）+ shell_tools（5）+ toolcall_renderop（5）+ toolcall_mode（23） | 其他工具提示 |
| 6 | taskplan/taskplan.go（27）+ cmd/plan.go（19）+ config/model_template（18）等 30+ 个 <5 处文件 | 逻辑包零星中文 |

**用户已确认的豁免项**（不计入 audit 归零目标，需在文档/审计备注写明）：
1. `docx/html.go` 的 10 处 —— Word 导出 HTML 的静态模板，属「数据内容」豁免
2. `bridge/executor.go` 的 5 处 —— `inputPromptPatterns` 子进程输出匹配 pattern，不产生输出
3. `i18n 豁免项`（LLM 透传/终端控制序列/单字符回显）—— 沿用架构文档 1.2 边界

**用户已确认的取舍**：工具 result 文案（excel/docx/browser 等返回给 LLM 的描述）i18n 后随 `--lang` 变化。

## 实现方案

1. 按上表 6 批逐批迁移：所有硬编码中文 → `i18n.T()/TF()`，新增 key 同步 `i18n/keys.go` + `zh.go`/`en.go`（及对应 `*_loop.go`/`*_system.go`）
2. 修复 `KeyToolUsageShellSend` 缺 zh 翻译（补 zh_system.go）
3. 每批跑 `go build ./...` + `go vet ./...` + `bin/output_audit.sh`，要求第 3/5 项只降不升
4. 迁移过程保持输出格式不变（emoji 前缀、分隔线、stderr 通道）——**纯文案替换，零行为变化**

## 用例列表

### UC-0001 audit 归零检查（静态）
**前置条件**：全部批次迁移完成，分支 FEATURE-305 代码就绪。
**步骤**：运行 `bin/output_audit.sh`。
**预期**：
- 第 3 项 Hardcoded Chinese = **0**
- 第 5 项 i18n keys missing zh/en = **0**
- 第 1 项 Direct fmt 不高于基线（202±10）
- 第 2 项 Magic string = 0、第 4 项 Sync-blocking = 20（均不反弹）

### UC-0002 KeyToolUsageShellSend 修复（静态）
**前置条件**：迁移进行中或完成。
**步骤**：运行 `bin/output_audit.sh --list` 与 `grep -c 'KeyToolUsageShellSend' i18n/zh_system.go i18n/en_system.go`。
**预期**：audit 不再报告该 key 缺失；zh_system.go 与 en_system.go 均含 `KeyToolUsageShellSend` 翻译。

### UC-0003 新增 key zh/en 双存在（静态，可脚本化）
**前置条件**：每批迁移后。
**步骤**：对每个新增的 `KeyXxx`，在 `i18n/keys.go` 中定义，并分别在 `zh.go`/`en.go`（或 `*_loop.go`/`*_system.go`）中提供翻译。
**预期**：`bin/output_audit.sh --list` 无 "missing zh/en" 报告；`i18n/i18n_test.go` 全部通过。

### UC-0004 运行时：startup 警告 stderr 中英文（运行时）
**前置条件**：`./co-shell --bad-flag 123`（构造非法参数），分别 `--lang zh` 与 `--lang en` 运行。
**步骤**：观察 stderr。
**预期**：
- zh：`Warning: 无效的 --bad-flag`（或对应中文翻译），输出至 stderr
- en：`Warning: invalid --bad-flag`，输出至 stderr
- 两语言文案不通硬化（audit 归零证明），原格式（前缀/换行）不变

### UC-0005 运行时：:set 向导中英文显示（运行时）
**前置条件**：分别 `--lang zh` 与 `--lang en` 进入 REPL。
**步骤**：输入 `:set`，观察分组标题、参数说明、当前值提示；再进入 `:set output-categories` 子配置。
**预期**：两语言下所有可见文案均走翻译（无中文硬编码残留）；选择编号右对齐、快捷键提示（[B]/[Q]）格式不变。

### UC-0006 运行时：工具 result 文案随语言（运行时）
**前置条件**：分别 `--lang zh` 与 `--lang en`，toolcall-mode=openai，show-tool=true。
**步骤**：LLM 调用 excel_open / browse_navigate / write_to_file 各一次，观察 `[⚙️]< 工具摘要` 与工具返回给 LLM 的 result。
**预期**：zh 下摘要为中文话术、en 下为英文话术；工具 result 内嵌文案（如 `[返回结果]`/`[意图]` 标签）随语言切换；无中文硬编码残留。

### UC-0007 运行时：main.go 其他 C 类输出中英文（运行时）
**前置条件**：分别 `--lang zh`/`--lang en` 正常启动与退出。
**步骤**：观察欢迎页、DB 自动同步状态、cleanup 提示。
**预期**：两语言下均走 i18n，无中文硬编码残留。

### UC-0008 回归：向导功能与渲染基线不变（单测）
**前置条件**：全部迁移完成。
**步骤**：运行 `go test ./cmd/... ./agent/... ./repl/... .` 与 `go vet ./...`。
**预期**：
- 全部测试通过（含 `settings_mode_dirs_test.go`、`repl/render_test.go` golden、`render_test.go` single_cmd golden、`i18n/i18n_test.go`）
- `go vet ./...` 0 警告
- 行为零变化（纯文案替换，审核 diff 只改字符串字面量）

## 验收标准

1. `bin/output_audit.sh` 第 3 项 Hardcoded Chinese = **0**、第 5 项 i18n keys missing zh/en = **0**（硬目标）
2. `go build ./...` 通过；`go vet ./...` 0 警告
3. `go test ./...` 全绿（含 golden 基线回归，渲染零变化）
4. 手动验证 UC-0004~UC-0007 运行时场景（zh/en 双语）
5. ROADMAP FEATURE-305 标 ✅ + BUILD 编号；DEV-HANDOVER 更新 audit 计数与豁免备注