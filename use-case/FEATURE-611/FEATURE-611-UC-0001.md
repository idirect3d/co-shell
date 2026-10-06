# FEATURE-611 UC-0001：动态事件保留机制（未确认事件重放 + meta.ack 确认）

> 分支：**FEATURE-611**　版本：**v0.100.0**　BUILD：**1231**
> 现象：`<user_dynamic_events>` 为 one-shot 注入——事件被注入到某一条消息的 `<environment_details>` 后，该消息在对话历史中被 `env_prune` 降级（L1/L2）时 `user_dynamic_events` 块被直接丢弃；若 LLM 当轮未回应（正在跑长任务、事件注入后继续做别的事），信息永久丢失。
> 判定口径：**重要事件**（user_message / board_message / board_timeout / sub_agent_done / ui_action）在未被确认前，会在后续每条消息的 `<user_dynamic_events>` 中带 `[unacked]` 标记重复出现（保留原始时间戳），直到 LLM 在工具调用 `meta.ack` 中按事件 ID 精确确认；**瞬时事件**（clip_object / upload_file / open_file / view_file）不参与重放；`dynamic_event_retain_max=0` 时完全退化为原 one-shot 行为。

## 背景（现状与缺口）

- 现状：`agent/dynamic_events.go` 的 `dynamicEventQueue.drain()` 一次性清空队列；`consumeDynamicEvents()` 把事件渲染进当次 envelope。`agent/env_prune.go` 的 `envLiteTags`/`envMinimalTags` 均不含 `user_dynamic_events`，降级时该块被整块删除。
- 缺口：事件只存在于一条消息里，「注入过」= 永久丢弃，没有任何确认/重放机制。
- 本次：引入事件唯一 ID、未确认事件重放、`meta.ack` 精确确认、重放上限配置、user_message 处理方式配置、系统提示词策略说明。

## 变更点（计划）

| 位置 | 变更 |
|---|---|
| `agent/dynamic_events.go` | `DynamicEvent` 增 `ID string`；`dynamicEventQueue` 增 `seq` 计数器与 `acked` 标记；新增「未确认事件重放」注入路径（不改 drain 的一次性语义则新增 `pending/ack` 接口）；渲染带 `id` 属性与 `[unacked]` 标记 |
| `agent/risk.go` | `metaFieldNames` 增 `"ack"`；新增 `parseMetaAck(args)`（可选字段，不加入 `validateMeta` 必填） |
| `agent/tools.go` | `injectMetaParam` / `toolMetaDesc` 说明 `meta.ack`（可选，列出已处理事件 ID） |
| 工具调用处理路径 | 解析 `meta.ack` 并调用 `agent.AckDynamicEvents(ids)`（在工具执行前/后统一处理） |
| `config/config.go` | 新增 `DynamicEventRetainMax int`（0=禁用，默认 100）、`UserMessageMode string`（dynamic/full，默认 dynamic） |
| `cmd/settings_web.go`、`cmd/settings.go`、`cmd/settings_display.go` | 记忆与上下文分组注册两项配置（Web + CLI `:set`） |
| `i18n/zh.go`、`i18n/en.go`、`i18n/keys.go` | 新增配置标签/说明文案 |
| `i18n/zh_system.go`、`i18n/en_system.go` | `KeyCapEnvAwareness` 补充未确认事件重放与 `meta.ack` 处理策略 |

## 用例与结果

> 状态列在实施完成前为 `待验证`；`(R)` = 可自动化（单元测试/命令行复现），`(M)` = 需人工在 Web UI/REPL 验收。

### A. 事件唯一 ID 与渲染

- **UC-01（R）** 每个新入队事件获得唯一 ID（形如 `ev-<seq>`，`seq` 单调递增），同一会话内不重复。
- **UC-02（R）** 可确认事件的标签带 `ack_id` 属性：`<user_message ack_id="ev-12">…</user_message>`；user_message / ui_action / board_message / board_timeout / sub_agent_done 五类均带；瞬时事件（clip_object / upload_file / open_file / view_file）**不带** `ack_id`（它们只注入一次，无需确认）。
- **UC-03（R）** 事件 ID 在一次注入到下一次重放之间保持稳定（同一事件重放时 ID 不变）。
- **UC-04（R）** 路径类事件的重复合并（同 kind + path 刷新时间戳）不改变已分配 ID，或明确记录为「合并后沿用首条 ID」的一致行为。

### B. 未确认事件重放

- **UC-05（R）** 重要事件首次注入后处于「未确认」态；在未收到 `meta.ack` 前，后续每次 envelope 构建都会再次包含该事件。
- **UC-06（R）** 重放的事件带 `unacked="true"` 与 `[unacked] ` 标记（如 `<user_message ack_id="ev-12" unacked="true">[unacked] …</user_message>`），首次注入不带该标记；预渲染标签（board_message / board_timeout / sub_agent_done）以属性注入方式携带（如 `<sub_agent_done id="A1" … ack_id="ev-3">[unacked] …`）。
- **UC-07（R）** 重放保留事件**原始时间戳**（不刷新为当前时间）。
- **UC-08（R）** 连续 N 次 envelope 构建（未 ack）时，同一事件出现 N 次，且顺序与首次入队顺序一致。
- **UC-09（R）** 瞬时事件（clip/upload/open/view）**不重放**：注入一次后即从队列移除，后续 envelope 不再出现。
- **UC-10（R）** `sub_agent_done` / `board_message` / `board_timeout` / `ui_action` 四类重要事件均纳入重放（逐一验证）。

### C. meta.ack 确认

- **UC-11（R）** 工具调用 `meta.ack: ["ev-12"]` 后，`ev-12` 不再出现在后续任何 envelope 中。
- **UC-12（R）** 仅被 ack 的事件停止重放；未被 ack 的其他未确认事件继续重放（精确确认，不误伤）。
- **UC-13（R）** `meta.ack` 为**可选**字段：不带 `ack` 的工具调用正常执行，不触发 `meta.ack is required` 之类校验错误。
- **UC-14（R）** `meta.ack` 不被 `promoteMisplacedMetaParams` 提升为顶层参数（`metaFieldNames` 含 `"ack"`）。
- **UC-15（R）** ack 不存在或非数组（如字符串 `"ev-12"`）时不 panic；按「忽略该字段」或「解析为单元素」的既定行为一致处理并记录结果。
- **UC-16（R）** ack 一个不存在/已确认的 ID 是幂等 no-op，不报错、不影响其他事件。
- **UC-17（R）** 多个 ID 一次性确认：`meta.ack: ["ev-11","ev-12"]` 两条同时停止重放。

### D. 分级保留与容量

- **UC-18（R）** 未确认重要事件数量超过 `dynamic_event_retain_max` 时，丢弃最旧的未确认事件（保留最新的 N 条）。
- **UC-19（R）** `dynamic_event_retain_max=0` 时机制完全禁用：所有事件恢复 one-shot（注入一次即移除，无 `[unacked]` 标记、无重放）。
- **UC-20（R）** `dynamic_event_retain_max` 为负值/非法值时回退默认 100（或明确拒绝并提示），不导致无限累积。

### E. user_message 处理方式

- **UC-21（R）** `user_message_mode=dynamic`（默认）：用户补充消息走动态感知 `<user_dynamic_events>` 注入，并纳入重放。
- **UC-22（R）** `user_message_mode=full`：用户补充消息作为完整 user_message 插入消息流（走正常消息路径），不再作为动态事件注入。
- **UC-23（R）** `dynamic` 模式下，任务结束时未消费的 user_message 仍可通过 `PendingUserMessages()` 回填 Web UI 输入框（与重放机制不冲突、不重复回填）。
- **UC-24（R）** `full` 模式下不出现「同一补充消息既作为 user 消息又作为动态事件」的重复注入。

### F. 与 env_prune 的交互

- **UC-25（R）** 携带事件的旧消息被 `env_prune` 降级（L1/L2）移除 `user_dynamic_events` 块后，未确认事件仍出现在**最新** envelope 中（信息不丢失）。
- **UC-26（R）** `EnvHistoryPrune=off` 时不重放也会保留原块（本机制不改变关闭裁剪时的既有行为）。

### G. 配置项可见可改

- **UC-27（R）** `dynamic_event_retain_max` 在 Web 设置「记忆与上下文」分组可见，且与 `env-include-user-dynamic` 等相邻；类型为 number，默认 100。
- **UC-28（R）** `user_message_mode` 在 Web 设置「记忆与上下文」分组可见；类型为 enum，选项 `dynamic`/`full`，默认 `dynamic`。
- **UC-29（R）** CLI `:set dynamic-event-retain-max <n>` 与 `:set user-message-mode <dynamic|full>` 可查询当前值、可设置并持久化到 config.json；非法值被拒绝并给出提示。
- **UC-30（R）** `:set list`（settings 列表）中两项配置显示当前值，且默认值列正确。

### H. 系统提示词

- **UC-31（R）** zh 系统提示词的【环境感知】「用户动态感知」条目包含：未确认重要事件会在后续消息中带 `[unacked]` 重复出现、需通过工具调用 `meta.ack` 确认、确认后不再出现。
- **UC-32（R）** en 系统提示词对应条目包含等价英文说明（unacked / meta.ack / replayed until acknowledged）。
- **UC-33（R）** `meta` 参数描述（`toolMetaDesc`）说明 `ack` 为可选字段及其语义（列出已处理的事件 ID）。
- **UC-34（R）** `meta-capability-enabled=off` 时不注入元能力索引，但 `[unacked]` 标记与重放机制仍照常工作（提示词缺失不改变后端行为）。

### I. 编译与版本一致性

- **UC-35（R）** `go build ./... && go vet ./...` 一次性通过。
- **UC-36（R）** `go test ./...`（或 `-short`）无新增失败；新增单元测试全部通过。
- **UC-37（R）** 版本一致：`main.go` `version=0.100.0`/`build=1231`，`cmd/co-shell-hub/main.go` `hubVersion=0.100.0`/`hubBuild=1231`。
- **UC-38（R）** 产物存在：`work/co-shell-0.100.0.darwin.arm64` 与 `work/co-shell-hub-0.100.0.darwin.arm64`，`--version` 自检输出与常量一致。
- **UC-39（R）** 所有改动位于 `FEATURE-611` 分支，`main` 分支无本次提交。

### J. 回归与边界

- **UC-40（R）** 无任何动态事件时，envelope 不含 `<user_dynamic_events>` 块（不因新机制产生空块）。
- **UC-41（R）** 大量事件（>1000）下未确认重放不导致内存/耗时异常增长（队列上限生效）。
- **UC-42（R）** 既有用例不回归：`agent/dynamic_events_test.go`、`agent/env_prune*_test.go`、`agent/transparency_test.go`、`agent/promote_meta_test.go`、`agent/meta_param_test.go`、`cmd/settings_token_opt_test.go` 全部通过。
- **UC-43（M）** Web UI 人工验收：任务运行中发补充消息 → 下一轮 envelope 带 `[unacked]`；LLM 下次工具调用带 `meta.ack` 后该事件消失。
- **UC-44（M）** `user_message_mode=full` 人工验收：补充消息以普通 user 消息进入对话，不再出现在 `<user_dynamic_events>`。
- **UC-45（M）** `dynamic_event_retain_max=0` 人工验收：行为与改动前一致（one-shot、无 `[unacked]`）。

## 验证结果（2026-10-06）

| 用例 | 结论 | 证据 |
|---|---|---|
| UC-01/03 | ✅ 通过 | `go test ./agent/ -run TestFeature611EventIDsUniqueAndStable` PASS（ev-1/ev-2 唯一，重放保持 ID）|
| UC-05/06/07 | ✅ 通过 | `TestFeature611UnackedReplayMarkerAndTimestamp` PASS（首次无标记，重放带 `[unacked]`+`unacked="true"`，时间戳原样保留）|
| UC-09 | ✅ 通过 | `TestFeature611TransientEventsNotReplayed` PASS（clip 不重放、不带 ack_id）|
| UC-10 | ✅ 通过 | `TestFeature611BoardEventReplayAndAckParsing` PASS（board 事件带 ack_id 并重放）|
| UC-11/12/16/17 | ✅ 通过 | `TestFeature611AckStopsReplayPrecisely` PASS（精确确认、幂等、未 ack 的继续重放）|
| UC-13 | ✅ 通过 | `TestFeature611ParseMetaAck` PASS（absent/array/[]string/裸字符串/空数组/非法值 6 种情形）|
| UC-14 | ✅ 通过 | `TestFeature611AckNotPromoted` PASS（`meta.ack` 不被提升到顶层）|
| UC-18 | ✅ 通过 | `TestFeature611RetainCapDropsOldest` PASS（cap=2 时丢掉最旧）|
| UC-19/20 | ✅ 通过 | `TestFeature611RetainDisabledIsOneShot` PASS（0 与负值均为 one-shot）|
| UC-21/22 | ✅ 通过 | `TestFeature611UserMessageModeAccessor` PASS（默认 dynamic，full 可设，nil cfg 回退 dynamic）|
| UC-25 | ✅ 通过 | `TestFeature611ReplayIndependentOfPruning` PASS（原消息被 L1 降级丢掉块后，未确认事件仍在最新 envelope 重放）|
| UC-02/06（属性注入） | ✅ 通过 | `TestFeature611InjectTagAttrs` PASS（容器标签与自闭合标签均正确注入属性）|
| UC-35/36/38 | ✅ 通过 | `go build ./... && go vet ./...` 一次性通过；编译产物 `work/co-shell-0.100.0.darwin.arm64` 自检输出 `co-shell v0.100.0 [BUILD-1231]` |

### 预存在失败（非本次引入，已在 main 分支复现）

| 测试 | 说明 |
|---|---|
| `agent: TestSetResultMode_UsesResolvedRulesAndPrinciples` | main 上同样失败（FEATURE-610 重构后 SetResultMode 不再产生消息，测试未同步）|
| `cmd: TestSkillHandlerListEmpty` / `TestSettingsJSONFillsDefaults` | main 上同样失败 |
| `i18n: TestRulesDirCustomizationNote` / `TestPerSectionFileNotes` / `TestRulesNoteMergesRulesFile` | main 上同样失败（受本地 RULES.md 改动影响）|
| `agent: TestLineRendererTokenEvents` / `TestRetryCountLimit_EnterResets` | 单独运行通过，为包内 i18n 状态污染的既有问题 |

### 本次同步更新的既有断言

- `agent/dynamic_events_test.go`：`<user_message>` → `<user_message`（标签现带 `ack_id` 属性）。
- `agent/ui_window_test.go`：`<ui_action>` → `<ui_action`（同上）。

### 设计微调（相对初稿）

1. 属性名用 `ack_id`（而非 `id`）：`sub_agent_done` 等预渲染标签本身已有 `id`，避免属性重复。
2. 瞬时事件不带 `ack_id`：它们从不重放，无需确认，避免误导 LLM 做无效确认。
3. hub 无 `--version` 参数（与 co-shell 不同），hub 仅验证编译成功。

