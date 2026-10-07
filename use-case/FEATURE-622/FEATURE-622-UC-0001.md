# FEATURE-622 UC-0001：env_details 历史裁剪收敛为「唯一全量信封」

> 分支：**FEATURE-622**　版本：**v0.105.0**　BUILD：**待定（编码后回填）**
> 需求（用户 2026-10-07 22:35~22:36 确认）：
> ① 方向 = 方案 C——**最新信封自足全量**，历史一律只剩 `<time>`+`<message_no>`；
> ② 配置精简 = 去掉「完整信息保留条数 / 简要信息保留条数」两个数字，只保留 `env_history_prune` 一个开关（开 = 历史裁剪，关 = 完全按原策略）；
> ③ **取消 L1**：除最后一条外全部只剩 `<time>`+`<message_no>`；
> ④ `<task_plan>` **不做特殊回退**（接受最新信封跳过计划的那一轮看不到计划）；
> ⑤ `<board_agents>` **每轮实发明细**（id|name|status）——一个上下文中只有最后一个 env_details 才有明细；
> ⑥ 裁剪开启时最后信封「完整自足」——`env_dir_change_detect` / `env_board_roster_brief` / `env_runtime_info_once` **三个瘦身子开关被旁路**，仅在裁剪关闭时生效。
>
> 现状（缺陷）：生成端变化检测发出的 `<unchanged since msg#N/>` 是**跨消息引用**，而发送端降级把 msg#N 的信封裁成只剩 `<time>`+`<message_no>` → 引用悬空。实测：连续多轮请求中 `<current_dir>`/`<tools>`/`<research>` 恒为 `unchanged since msg#1`（仅每 20 个信封强制全量一次才可见），`<board_agents rev="771"><unchanged/>` 则**没有任何兜底路径**，长会话中名册明细事实上长期不可见。

## 术语

- **唯一全量信封** = 一次请求中，**只有最后一条带 `<environment_details>` 的 user/tool 消息**保持完整信封，其余消息的信封一律只保留 `<time>` + `<message_no>`。
- **自足** = 该全量信封内不出现任何跨消息引用（`<unchanged since msg#N/>`、`<unchanged/>`），所有块均为实发内容。
- **全量载体** = 从消息数组末尾向前查找的第一条带信封的 user/tool 消息（不再写死 `msgs[n-1]`）。
- **裁剪开 / 关** = 配置 `env_history_prune` 为 true / false。
- **旧行为** = `env_history_prune=false` 时的现有逻辑：不做任何裁剪，生成端三个瘦身子开关照旧生效。
- `(R)` = 离线可断言（单元测试 / Go 层直接构造消息后检查信封文本）；`(M)` = 需 `--serve` 隔离实例 + 调试浏览器或真实会话实测。
> 运行时循环：先跑 `(R)` 全绿，再对每个 `(M)` 用例逐项实测并汇总，最后回填 BUILD。

## 变更点（计划）

| 位置 | 变更 |
|---|---|
| `config/config.go` | 删除 `EnvHistoryFullKeep` / `EnvHistoryLiteKeep` 及默认值；`EnvHistoryPrune` 注释改写为唯一开关 |
| `config.json` | 删除 `env_history_full_keep` / `env_history_lite_keep` 两键 |
| `cmd/settings.go` / `cmd/settings_agent.go` / `cmd/settings_web.go` | 删除两项的设置行、解析分支与 Web 表单项 |
| `i18n/keys.go` / `i18n/zh.go` / `i18n/en.go` | 删除两项文案；改写 `env_history_prune` 说明（开 = 历史只留时间+序号、最后一个全量） |
| `agent/env_prune.go` | 降级逻辑改写为「唯一全量信封」；删除 `task_plan` 回退；全量载体由结尾向前查找；常量与注释同步 |
| `agent/env_slim.go` | 裁剪开启时三列表 / 名册 / runtime_info 一律实发（不产占位符）；裁剪关闭时保持原变化检测 |
| 测试 | 更新 `env_prune_test.go`（分级与回退断言作废）、`cmd/settings_token_opt_test.go`、`agent/feature611_test.go`；新增本任务单测 |

## 用例与结果

### A. 裁剪开启（默认）——唯一全量信封

- **UC-01（R）** 4 条带信封的消息，裁剪开 → 仅最后一条含 `<current_dir>`/`<tools>`/`<research>`/`<board_agents>`/`<task_plan>`/`<opened_resources>` 等全量块；其余 3 条**只有** `<time>`+`<message_no>`。
- **UC-02（R）** 取消 L1：第 2 条（原 L1 位）不再含 `<cwd>`/`<context_window>`/`<current_mode>`/`<result_mode>`/`<no_tool_action>`/`<task_plan>`/`<opened_resources>`（与旧断言相反，属预期变化）。
- **UC-03（R）** 全量载体按尾部查找：消息数组末尾是 assistant（无信封）时，最后一条带信封的 user/tool 消息仍保持全量，不被降级。
- **UC-04（R）** `task_plan` 回退已移除：最新信封无 `<task_plan>` 时，历史消息中任何一条都不再因此被提升（旧行为会提升最近一条到 L1）。
- **UC-05（R）** 持久历史不被改动：裁剪只作用于发给 provider 的副本；`a.messages` 中每条仍是全量信封，副本与持久历史不共享 ContentParts 底层数组。

### B. 生成端自足（裁剪开启时三个瘦身子开关被旁路）

- **UC-06（R）** `env_dir_change_detect=true` 且裁剪开：连续两次构建信封（列表内容未变），第二次仍实发 `<current_dir>`/`<tools>`/`<research>` 全量，信封文本中**不含** `<unchanged since msg#`。
- **UC-07（R）** `env_board_roster_brief=true` 且裁剪开：名册 rev 未变时仍实发 `<agent …>` 明细（id|name|status），**不含** `<unchanged/>`。
- **UC-08（R）** `env_runtime_info_once=true` 且裁剪开：runtime_info 未变化时仍每轮输出 `<runtime_info>` 块。
- **UC-09（M）** 端到端：连续 ≥3 轮工具调用后，检查实际发往模型的 messages——除最后一条外全部信封均为 `<time>`+`<message_no>`，最后一条含全量三列表 + 名册明细 + runtime_info，且**全文无 `<unchanged`**。

### C. 裁剪关闭（回归旧行为）

- **UC-10（R）** `env_history_prune=false`：每条消息的信封与裁剪前逐字节相同（不裁剪、不改写）。
- **UC-11（R）** 裁剪关闭时三个瘦身子开关恢复生效：三列表内容未变 → 第二次信封出现 `<unchanged since msg#N/>`；名册 rev 未变 → `<unchanged/>`；runtime_info 值未变 → 不出现。
- **UC-12（R）** 裁剪关闭且 `env_dir_change_detect=false` 时：三列表每轮实发（原语义不变）。

### D. 配置与界面（精简为单一开关）

- **UC-13（R）** 配置项精简：结构体、`config.json`、`:set` 清单、Web 设置页、i18n 文案中均不再出现 `env_history_full_keep` / `env_history_lite_keep`；旧 `config.json` 残留这两个键时报错为「忽略未知字段」可接受（加载不失败）。
- **UC-14（R）** `:set env-history-prune on|off` 仍可切换并写回 `config.json`；旧的 `:set env-history-full-keep` 报「未知配置项」而不是静默接受。
- **UC-15（M）** Web 设置页「环境信息」分组只剩这一个开关，切换保存后重启仍生效。

### E. 回归（其它机制不受影响）

- **UC-16（R）** 同轮单份信封（plan B）：一个迭代内非末条 tool 结果仍为 `<time>`+`<message_no>`，末条为全量（`env_tool_batch_single` 行为不变）。
- **UC-17（R）** `<skills>` 仍每请求注入到最新信封，历史信封内不出现（`injectSkillsEnvToLast` 不受影响）。
- **UC-18（R）** 最新信封仍完整包含动态块：`<user_dynamic_events>`、`<board_replies>`、`<sub_agents>`、`<opened_resources>`、`<session_title>`、`<user_presence>`、`<no_tool_action>`、`<context_window>`、`<cwd>`、`<current_mode>`、`<result_mode>`；未确认事件重放（`[unacked]`）机制不变。
- **UC-19（M）** 真实会话实测：连续 ≥3 轮对话后，每轮请求中「最后一个 env_details 完整自足、历史仅时间+序号、全文无 `<unchanged`」；并核对 `attempt_completion`、`:context full`、Web UI 回放（持久历史）仍为全量。
- **UC-20（R/M）** token 对比：同一段 ≥10 条消息的会话，记录裁剪开（新）与裁剪关（旧）两种设置下的请求体字节数，裁剪开启时历史部分显著小于裁剪关闭、且最后一条为全量，给出实测数字。

## 验收标准

- `go build ./... && go vet ./...` 通过；
- `(R)` 用例（UC-01~08、10~14、16~18、20）全部通过，单测覆盖「唯一全量信封 / 自足无占位符 / 配置精简 / 尾部查找载体」；
- `(M)` 用例（UC-09、15、19、20）在 `--serve` 隔离实例实测通过，并记录可复核证据（请求体片段、设置页截图、字节数）；
- 裁剪开启时，真实会话中不再出现 `<unchanged since msg#` 与 `<board_agents>` 的 `<unchanged/>`；
- 裁剪关闭时行为与改动前一致（回归）；
- 持久历史、`:context full`、Web UI 回放、`attempt_completion` 的 `task_message_no` 语义均不回归。

## 预期变化（与旧测试的差异，供确认）

- 旧 `TestEnvPruneLevels`（L2/L2/L1/L0）与 `TestEnvPruneEdgeCases`（`full_keep=0` / `lite_keep=0`）断言作废：分级只剩「最后一条全量 + 其余最小」。
- 旧 `TestEnvPruneTaskPlanFallback` 作废：回退机制按用户决策移除。
- 旧 `TestEnvStaticBlocksChangeDetect` / `TestBoardRosterBrief` 改为双向覆盖（裁剪开 = 实发；裁剪关 = 占位）。
- `cmd/settings_token_opt_test.go` 中两个条数项的断言删除，改为断言「已不存在」。
