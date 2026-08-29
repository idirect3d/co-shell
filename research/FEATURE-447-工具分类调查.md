# FEATURE-447 工具调用透明化 — 工具分类调查报告

> 调查时间：2026-08-28
> 调查范围：所有注册的工具调用方法（共 61 个）
> 调查内容：哪些工具引入了新的进展报告、文件影响、风险报告机制

---

## 一、机制总览

透明化机制分三层，作用于**所有 61 个工具**：

| 机制 | 函数 | 作用范围 |
|------|------|---------|
| **风险报告** | `assessRisk`（agent/risk.go） | 所有工具（规则引擎 + LLM 自评取较高者） |
| **文件影响** | `affectedFiles`（agent/files.go） | 所有工具（确定性从参数 / 预测性从 LLM 字段） |
| **进展报告** | `applyProgressReport`（agent/progress.go） | 所有工具（可携带 progress 参数） |

---

## 二、风险等级分类（agent/risk.go）

### 16 个只读工具 → low
`read_file`、`search_files`、`list_files`、`list_code_definition_names`、`visual_analysis`、`get_memory_slice`、`memory_search`、`view_task_plan`、`list_settings`、`vault_list`、`excel_read`、`excel_overview`、`word_read`、`word_overview`、`word_table_read`、`word_inspect_style`

### 17 个写工具 → medium
`write_to_file`、`replace_in_file`、`excel_edit`、`excel_save`、`excel_copy`、`excel_paste`、`excel_insert`、`excel_delete`、`excel_sheet`、`excel_format`、`word_continue`、`word_erase`、`word_format`、`word_save`、`update_settings`、`schedule_task`、`track_task_progress`

### 11 个执行工具 → high
`execute_command`、`shell_send`、`shell_reset`、`launch_sub_agent`、`delete_memory`、`vault_remove`、`browser_navigate`、`browser_click`、`browser_type`、`browser_evaluate`、`browser_close`

### 敏感路径检测（强制 high）
任何工具触及 `.ssh`/`.aws`/`.config` 等用户私有目录或 `/etc`/`/usr` 等系统目录 → 强制 high

---

## 三、文件影响分类（agent/files.go）

### 27 个确定性工具（从 path/paths 参数解析，前端 accent 色高亮）
`read_file`、`write_to_file`、`replace_in_file`、`search_files`、`list_files`、`list_code_definition_names`、`visual_analysis`、`excel_open`、`word_open`、`excel_read`、`excel_overview`、`excel_edit`、`excel_save`、`excel_copy`、`excel_paste`、`excel_insert`、`excel_delete`、`excel_sheet`、`excel_format`、`word_read`、`word_overview`、`word_table_read`、`word_inspect_style`、`word_continue`、`word_erase`、`word_format`、`word_save`

### 34 个预测性工具（从 LLM 的 files 字段解析，前端蓝色高亮）
`execute_command`、`shell_send`、`shell_reset`、`shell_get_output`、`shell_window_content`、`browser_navigate`、`browser_click`、`browser_type`、`browser_evaluate`、`browser_close`、`browser_screenshot`、`browser_scroll`、`browser_go_back`、`browser_go_forward`、`browser_get_interactive_elements`、`browser_get_rendered_html`、`evaluate_expression`、`get_memory_slice`、`memory_search`、`delete_memory`、`vault_list`、`vault_add`、`vault_remove`、`schedule_task`、`track_task_progress`、`view_task_plan`、`update_settings`、`list_settings`、`launch_sub_agent`、`ask_followup_question`、`attempt_completion`、`reorganize_context`、`excel_close`、`word_close`

---

## 四、⚠️ 发现的分类缺口（17 个未分类工具）

以下工具**未在 risk.go 中分类**，走默认 medium 风险，但其中部分应调整：

| 工具 | 当前 | 建议 | 理由 |
|------|------|------|------|
| excel_open / word_open | medium | **low** | 打开文件是读操作 |
| excel_close / word_close | medium | **low** | 关闭会话是读操作 |
| browser_get_interactive_elements / get_rendered_html / go_back / go_forward / screenshot / scroll | medium | **high** | 浏览器操作涉及外部环境 |
| vault_add | medium | **high** | 涉及敏感凭据存储 |
| evaluate_expression | medium | **low** | 纯计算，无副作用 |
| shell_get_output / shell_window_content | medium | **high** | 读取 shell 会话状态 |
| ask_followup_question / attempt_completion / reorganize_context | medium | **low** | 交互/内部操作，无文件副作用 |

---

## 五、结论

- **风险报告**：44 个工具已明确分类（16 low + 17 medium + 11 high），17 个未分类走默认 medium
- **文件影响**：27 个确定性 + 34 个预测性，全部覆盖
- **进展报告**：所有工具均可携带 progress 参数

**待决策**：是否补齐这 17 个未分类工具的风险等级？其中 `excel_open`/`word_open`/`evaluate_expression` 等应降为 low，`browser_*`/`vault_add`/`shell_*` 等应升为 high。
