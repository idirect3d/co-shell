# co-shell 多轮迭代 Token 优化设计方案

- 版本：draft-1（待用户确认）
- 日期：2026-10-01
- 作者：co-shell developer（L.Shuang 指导）
- 状态：**设计方案 + 基线测量阶段，尚未开发**

---

## 1. 背景与问题

多轮工具迭代时，上下文窗口被"与任务无关的固定包袱"快速吃掉，导致：

- 同样窗口内可完成的迭代轮数偏少；
- 过早触发 `reorganize_context` 上下文重整，打断任务连续性。

用户的判断（经代码核实成立）：**每轮注入的 `<environment_details>` 内容偏多，且历史消息里的 env 会逐条累积。**

## 2. 实测与代码事实

| 项 | 事实 | 依据 |
|---|---|---|
| env 注入时机 | 每条 user 消息、**每个** tool result 各注入一份完整 env，生成后冻结在消息上 | `agent/envelope.go:117`、`agent/run_stream.go:1707/1766` |
| 历史 env 是否裁剪 | **不裁剪**。`buildContextMessages` 只在 `window` 模式按条数截断，从不修改历史 env | `agent/loop.go:543-605` |
| 同一轮多工具 | 每个 tool result 各注入一份完整 env（故一轮 3 工具 = 3 份） | `agent/run_stream.go:1660-1767`（循环体内 1766 行） |
| 单份 env 组成 | `time/message_no/context_window/cwd/current_mode/no_tool_action/session_title/runtime_info/current_dir(70+ 条目)/tools(37 条)/research(10 条)/board_agents(14 agent 全描述)/board_replies/sub_agents/task_plan/opened_resources(+ui_windows)/user_dynamic_events` | `agent/envelope.go:156-254` |
| 单份 env 体积 | 约 **8–9KB**（≈2.2–2.5K tokens），其中 `current_dir+tools+research+board_agents` 占 ~70% | 本仓库 cwd 实测 |
| 工具定义体积 | `tool_call_mode=openai`，**62 个工具**全量 JSON schema 每轮发送；其中每个工具 `meta` 参数的 description 是同一段 ~330 字符长文本，重复 60+ 次 ≈ **20KB（≈5K tokens）/轮** | `grep -c 'Callback: ' agent/tools.go` = 62；`agent/tools.go` 各工具定义 |
| 工具组已有条件注入 | browser 按 `browserEnabled`、vault 按 `vaultStore!=nil`、subagent/plan/memory/ui/board 按开关；**excel/word 为 "always available"（无条件全量）** | `agent/tools.go:81/417/515/596/687/1019/1089/1093/1839/1844` |
| 当前配置 | `context_limit=-1`、`context_policy=reorganize`、`context_reorganize_threshold=80`、`tool_result_max_size=65536`、**`read_file_max_size=0`（无上限）**、`env_include_*=true`、`ui_context_prune=true` | `config.json` |

**推论（量级估算）**：按 50 轮工具迭代计算，仅历史 env 一项就占用 `(50-1)×8.5KB ≈ 416KB ≈ 120K tokens`，占 512K 窗口约 **23%**；若把每轮多工具、工具 schema 重复描述计算在内，固定包袱占比更高。

## 3. 目标与验收标准

**目标**：同等窗口内支持更多轮迭代，推迟/减少上下文重整触发。

**验收标准（可量化）**
1. 50 轮迭代场景下，窗口内"纯 env + 工具定义固定开销"占比从 ~23%+ 降到 **≤5%**；
2. 单份 env（L0）体积从 ~8.5KB 降到 **≤3KB**；
3. 每轮 62 个工具的 schema 体积下降 **≥30%**（首期目标）；
4. 任务正确性不回归：文件定位、任务计划追踪、loop 重试、`attempt_completion` 的 `task_message_no` 语义、Web UI 回放、`:context full` 全部保持现状；
5. 对比基线（见 `research/token-baseline/`）在同一标准化 10 轮任务下，**上下文增长斜率显著低于基线**。

## 4. 方案 A：历史 env 三级降级（最高收益）

### 4.1 分档定义

| 档 | 作用范围（从尾部数） | 保留内容 | 约体积 |
|---|---|---|---|
| **L0 完整** | 最新 `env_history_full_keep` 条 | 现状全量 | ~8.5KB |
| **L1 精简** | 其后 `env_history_lite_keep` 条 | `time` / `message_no` / `context_window` / `cwd` / `current_mode` / `no_tool_action` / `session_title` / `task_plan` / `opened_resources`(含 ui_windows) / `sub_agents` / `board_replies` | ~1KB |
| **L2 极简** | 其余全部 | `time` + `message_no` | ~80B |

- **默认**：`env_history_full_keep=1`、`env_history_lite_keep=1`（即 1 完整 + 1 精简 + 其余极简）。
- **参数语义（用户确认）**：`0` 表示"不限/全部"——`full_keep=0` → 所有消息都按 L0（等于关闭降级）；`lite_keep=0` → 除 L0 外全部按 L1（无 L2 层）。**完整优先于精简**：一条消息先判 L0，再判 L1，否则 L2。
- L1/L2 **不包含**：`current_dir` / `tools` / `research` / `board_agents` 全量 / `runtime_info` / `user_dynamic_events`（这些要么静态、要么一次性）。

### 4.2 裁剪算法（`pruneEnvContext`）

```
func (a *Agent) pruneEnvContext(msgs []llm.Message) []llm.Message:
    if !cfg.EnvHistoryPrune: return msgs          # 开关
    n := len(msgs)
    for i, msg := range msgs:
        if msg 无 env ContentPart: continue
        dist := n-1 - i                            # 距尾部距离：0=最新
        switch:
          cfg.FullKeep == 0 || dist < cfg.FullKeep       -> L0（不动）
          cfg.LiteKeep == 0 || dist < cfg.FullKeep+cfg.LiteKeep -> L1（重建精简块）
          else                                            -> L2（重建极简块）
```

**重建（而非正则替换）**：L1/L2 由 `buildEnvBlock(level, msg)` 从**该消息已有 env 中抽取**对应字段重新拼装（time/message_no 从原文取，task_plan 等如需保留则从原文对应块截取），避免依赖脆弱的字符串正则。

**兜底规则（必须实现）**
1. **最后一条消息永不降级**（`dist==0` 恒为 L0）。
2. 读取消息 env 的**内部逻辑**（loop retry 的 `retried_count` / `loop_feedback`、`isLoopFeedbackMessage`）读的是 `a.messages`（持久历史），不受影响；但实现时仍禁止修改持久历史。
3. **task_plan 可见性兜底**：若最新条因 `skipTaskPlan`（`track_task_progress`/`view_task_plan`/`attempt_completion` 的结果）不含 `task_plan`，则向前找到最近一条含 `task_plan` 的消息，将其至少保持在 L1（保证计划始终可见）。
4. **ContentParts 深拷贝**：`msgs` 的元素与 `a.messages` 共享 `ContentParts` 底层数组，重建前必须先克隆该消息的 `ContentParts` slice（与 `pruneUIContext` 同款处理，见 `agent/ui_prune.go:29-31`）。

### 4.3 接入点

- 新增文件：`agent/env_prune.go`（`pruneEnvContext`、`buildEnvBlock`、`envLevelOf`）。
- 调用点：`agent/loop.go` 的 `buildContextMessages()` 末尾，紧跟已有 `msgs = a.pruneUIContext(msgs)`（`agent/loop.go:681`）之后。
- 配置：`config.LLM` 新增 `EnvHistoryPrune bool` / `EnvHistoryFullKeep int` / `EnvHistoryLiteKeep int`；同步 `cmd/settings_*`（agent / web / CLI）。

### 4.4 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| LLM 引用历史某轮的文件列表 | 低 | L2 只丢弃静态列表；需要时可用 `list_files` 获取；C 的变更检测也会保留"变化时"的全量 |
| 污染持久历史 / Web UI 回放 | 中 | 只改副本 + 深拷贝 ContentParts；用例断言 `a.messages` 不变 |
| 裁剪掉 retry 标记 | 中 | 最后一条永不裁剪；且 retry 逻辑读持久历史 |
| XML 模式下 user 消息即工具结果 | 中 | 两种模式统一走 `pruneEnvContext`（按 ContentPart 判定，不区分角色） |

## 5. 方案 B：同轮多工具只注入一份完整 env

**现状**：工具循环内每个 tool result 都调用 `injectTimeAndMessageNoToLast()`（`agent/run_stream.go:1766`）。

**设计**
1. 循环前预扫描本轮 `toolCalls`，算出**最后一个会真正落库的消息下标**（排除被 `shouldDeferVisionToolResult` 延迟的视觉工具，`continue` 分支不落库）。
2. 循环中：当前工具是"最后一个" → 注入 L0；否则 → 注入 L2（`time`+`message_no`）。
3. XML 模式下 tool result 是 user 消息，规则相同。

**收益**：一轮 N 个工具从 `N×8.5KB` 降到 `8.5KB + (N-1)×0.1KB`。

**风险**：低。若最后一个 tool call 被延迟（视觉识别轮），需保证仍有消息拿到 L0（该分支由 `abortVisionRecognitionRound`/backfill 路径注入，需在用例中覆盖）。

## 6. 方案 C：env 内部瘦身

| 子项 | 现状 | 改法 | 配置 |
|---|---|---|---|
| `board_agents` | 14 个 agent 全职责描述（~2.2KB） | 只输出 `id | name | online/offline`（~0.7KB），职责按需用工具/系统提示词查；roster `rev` 未变时可输出 `<unchanged/>` | `env_board_roster_brief`(默认 true) |
| `current_dir`/`tools`/`research` | 每次全量列表（~5KB） | **变更检测**：与上次输出的 hash 比较，未变则输出 `<unchanged since msg#N/>`；变化或会话首条输出全量 | `env_dir_change_detect`(默认 true) |
| `runtime_info` | 每份都含 pid/version/build/port/bind（~0.5KB） | 仅在会话首条或字段值变化时输出 | `env_runtime_info_once`(默认 true) |

**注意**：变更检测的"上次输出"状态放在 Agent 上（会话级），需与**发送副本裁剪**解耦——变更检测作用于 **L0/L1 的生成源**（即 `buildFullEnvironmentDetails` 产出），因此一旦某条 env 生成为"未变"，它在历史 L0 中也只是 `<unchanged/>`。为保留可回溯性，建议**每 N 次迭代强制输出一次全量**（如 20）或当 LLM 显式调用 `list_files` 时重置。

**收益**：L0 从 ~8.5KB → ~2–3KB。

## 7. 方案 D：工具定义瘦身与分组懒加载

### 7.1 D1（低风险，建议先做，与懒加载解耦）

1. **`meta` 参数描述压缩**：60+ 个工具重复同一段 ~330 字符说明 → 统一为一行（~80 字符），并把完整说明保留在系统提示词一处。预计省 **~15KB（≈4K tokens）/轮**，零行为变化。
2. **工具描述去重**：4 个 subagent 工具互相重复解释 `<sub_agents>` 块、多处重复解释 `<environment_details>`/`meta` → 收敛为各自一句 + 系统提示词统一说明。
3. **`read_file_max_size`：`0`（无上限）→ `65536`**，与 `tool_result_max_size` 对齐，避免单次大文件读取吃满窗口。
4. XML 模式的工具声明走同一份 `[]llm.Tool`（`ToolDeclarations()`，`agent/toolcall_mode.go:2032`），自动同享收益。

### 7.2 D2（分组懒加载，两种方案，**待用户选定**）

**方案 D2-a：入口工具常驻 + 兄弟懒加载（推荐，行为语义不变）**

- 每个"可选组"保留 1 个入口工具常驻，其余成员在组被激活后才注入：
  | 组 | 常驻入口 | 全量注入条件 |
  |---|---|---|
  | browser（12+6 个） | `browser_navigate`、`browser_close` | 浏览器已启动（`chromeMgr.IsRunning()`）或本次会话调用过入口 |
  | excel（14 个） | `excel_open` | 存在打开的 excel 会话（`excelSessionMgr.listSessions()` 非空） |
  | word（13 个） | `word_open` | 存在打开的 word 会话 |
  | mcp（按 server） | — | 该 server 被首次使用/显式启用 |
  | ui / board / subagent / vault | 保持现有开关条件 | 同现状 |
- 实现：在 `buildToolsInternal()` 现有条件处（`agent/tools.go:81/417/…/1093/1839/1844`）增加组激活判定；新增 `agent/tool_groups.go` 维护组→成员与激活判据。
- LLM 侧**无新增概念**：它仍能看到 `excel_open`，调用后下一轮即看到全部 `excel_*`，无"鸡生蛋"问题。
- 预计每轮省 **6–10K tokens**（browser/excel/word/mcp 在未使用时合计占大头）。

**方案 D2-b：enable_tools + 工具目录（激进）**

- 未激活组**完全不注入**；系统提示词加"可用工具组目录（每组一行）"，另加 `enable_tools(group)` 工具供 LLM 主动启用。
- 优点：节省更多；缺点：**改变 LLM 行为语义**，模型可能永远不会去 enable，需要大量用例回归。风险高，建议仅作为后续演进（MCP 工具多时再考虑）。

### 7.3 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| 描述压缩导致模型误用工具 | 低 | 系统提示词保留完整 meta 说明；用例回归高频工具 |
| D2-a 组激活判定遗漏 | 中 | 用例覆盖"打开 excel→excel_* 出现→关闭→消失"；保留 `--enable-all-tools` 逃生开关 |
| D2-b 语义变化 | 高 | 本期不默认启用，仅设计保留 |

## 8. 新增配置项汇总

| 配置项 | 类型 | 默认 | 作用 |
|---|---|---|---|
| `env_history_prune` | bool | true | A：历史 env 三级降级总开关 |
| `env_history_full_keep` | int | 1 | A：尾部保留 L0 的条数；0=全部按 L0（关闭降级） |
| `env_history_lite_keep` | int | 1 | A：L0 之后保留 L1 的条数；0=其余全部按 L1 |
| `env_tool_batch_single` | bool | true | B：同轮多工具仅最后一个注入 L0 |
| `env_board_roster_brief` | bool | true | C：board_agents 精简为一行 |
| `env_dir_change_detect` | bool | true | C：目录/工具/研究列表变更检测 |
| `env_runtime_info_once` | bool | true | C：runtime_info 仅首条/变化时输出 |
| `tool_desc_compact` | bool | true | D1：工具描述去重与 meta 压缩 |
| `tool_lazy_load` | bool | true | D2-a：分组懒加载（入口常驻） |
| `read_file_max_size` | int | 0 → **65536** | D1：单次读取上限 |

> 所有开关关闭时，行为与当前版本**完全一致**（向后兼容）。

## 9. 代码接入点清单

| 模块 | 文件 | 改动 |
|---|---|---|
| A | `agent/env_prune.go`（新增） | `pruneEnvContext` / `buildEnvBlock` / `envLevelOf` |
| A | `agent/loop.go:681` 附近 | 调用 `pruneEnvContext` |
| A/B/C | `agent/envelope.go` | 拆分 `buildFullEnvironmentDetails`，支持按档位生成；C 的变更检测状态 |
| B | `agent/run_stream.go:1660-1767` | 预计算最后一个落库 tool call，分级注入 |
| C | `agent/board_state.go:247` | `BoardRosterBlock` 精简模式 |
| C | `agent/file_tools.go:480` | 列表输出支持"未变"占位 |
| D1 | `agent/tools.go`（60+ 处 meta 描述）、`agent/*_tools*.go` | 描述压缩、去重 |
| D2-a | `agent/tools.go`、`agent/tool_groups.go`（新增） | 组激活判定与注入过滤 |
| 配置 | `config/config.go`、`cmd/settings_agent.go`、`cmd/settings_web.go`、`cmd/settings.go` | 新增字段与设置项 |
| 国际化 | `i18n/zh.go`、`i18n/en.go` | 新增设置项描述文案 |

## 10. 测试与验收

- **基线对比**：见 `research/token-baseline/基线方案.md`（标准化 10 轮任务 × 3 次运行），开发后跑同一任务，比较上下文增长曲线。
- **单元/集成用例（use-case 阶段产出）**：
  1. A：L0/L1/L2 分档正确；`full_keep=0` 退化为全量；`lite_keep=0` 无 L2；开关关闭时输出与现状一致。
  2. A：持久历史 `a.messages` 不变、`:context full` 仍显示完整 env、Web UI 回放不变。
  3. A：loop 重试计数（`retried_count`）与 `loop_feedback` 仍生效，且最后一条消息恒为 L0。
  4. A：`task_plan` 兜底（最新条为 `track_task_progress` 结果时计划仍可见）。
  5. B：一轮 3 工具 = 1 份 L0 + 2 份 L2；视觉延迟工具路径仍保证有 L0。
  6. C：目录未变输出 `<unchanged/>`，变更后恢复全量；`board_agents` 精简后仍能正确选择 agent。
  7. D1：62 个工具 schema 总体积下降 ≥30%，工具调用成功率无回归。
  8. D2-a：excel/word/browser/mcp 组的激活/失效切换正确；`tool_lazy_load=false` 时与现状一致。
- **回归**：`go build ./... && go vet ./...`、`go test ./... -short` 全绿。

## 11. 后续可选（本期不做）

- **E-1 旧工具结果渐进降级**：对距尾部超过 N 条的 tool 消息，把结果替换为"首 512B + 摘要 + 全文路径"。收益大但**对任务连续性冲击较大**（用户判断），本期仅记录，待 A/B/C/D 落地评估后再议。
- **E-2 鼓励批量工具调用**：在系统提示词中明确"一次响应可并行调用多个工具"，依赖 B 先落地。
- **prompt caching**：若服务端支持，可降低费用，但不减少窗口占用，不作为本期手段。

## 12. 待用户确认的取舍点

1. **D2 懒加载方案**：选 **D2-a（入口常驻 + 兄弟懒加载，行为不变，推荐）** 还是 **D2-b（enable_tools + 工具目录，激进）**？
2. **C 的目录列表**：变更检测保留 `<unchanged/>` 占位，是否需要"每 N 次强制全量"？N 取值（建议 20）？
3. **D1 的 `read_file_max_size`**：直接改为 65536，还是保留更宽松的 131072？
4. 配置项命名与默认值是否符合预期（见 §8）。
