# Skill 加载/卸载机制设计方案（决策已定稿版）

> **文档性质**：设计方案（to-be），尚未实现。
> **配套文档**：`docs/skills-mechanism.md`（现状梳理 + D-001~D-008 缺陷登记）。
> **基准**：`main` 分支，co-shell `0.90.0`（2026-10-02）。
> **决策来源**：2026-10-02 经用户逐项确认（见 §0）。

---

## 0. 决策记录（已定稿）

| 编号 | 决策项 | **结论** | 影响 |
|---|---|---|---|
| **D-A** | 工具形态与授权 | **两个工具 `skill_load` / `skill_unload`，两者默认 `auto`**（不打断流程，靠事件与提示告知） | 命名依据见 §3.2；`DefaultToolModes()` 增加两项 |
| **D-B** | 注入载体 | **C 方案：历史消息 pin** —— 加载时插入真实消息并标记 `pinned`，压缩/裁剪时豁免；**卸载后不立即物理删除，仅在下一次上下文重整/裁剪时移除** | §3.3、§3.4；需在 5 处裁剪点做豁免 |
| **D-C** | 状态持久化 | **会话级**：随会话保存，切回会话仍加载，重启后按"状态 + 磁盘"重建（`SessionEntry` 加字段） | §3.8 |
| **D-D** | 用户可见性 | **提示词层标注 + REPL 加载/卸载行 + Web UI 新增"已加载技能"区**（复用子代理区样式）；**并在系统设置中新增 "Skills" 配置页面**：列出当前 skill 清单、滑动开关逐项启用/禁用、页面下方提供"默认启用"配置（对新增或未配置过的 skill 的默认状态） | §3.6、§3.7 |
| **D-E** | 上限策略 | **不设硬上限**，仅在超过阈值时告警（完全交给 LLM / 用户判断） | §3.9 |
| **D-F** | 与 D-001 的关系 | **分开**：先做加载/卸载机制；`description` 块标量解析（D-001）另开 FIX | §6 |

---

## 1. 目标与硬要求

| 编号 | 要求 | 验收要点 |
|---|---|---|
| **R1** | LLM 可自主加载/卸载 skill 详细内容 | 有专用工具；加载后正文进入上下文；卸载后解除保护 |
| **R2** | 加载动作与加载集合可被 co-shell 与用户感知（显示） | 提示词层标注；REPL 行；Web UI"已加载技能"区；设置页可管启用/禁用；命令可查询/干预 |
| **R3** | 已加载内容在卸载前始终完整、不随压缩或重整丢失/退化 | `reorganize_context`、window 裁剪、指针前移后，正文逐字节完整（处于 pin 豁免位置） |

非目标（本期不做）：远程搜索/下载/安装（D-007）、按章节部分加载、跨会话全局常驻、自动语义匹配、`description` 解析修复（D-001，另开 FIX）。

---

## 2. 现状机制调查结论（方案依据）

### 2.1 所有会"动"历史的代码位置（pin 豁免点清单）

| 位置 | 行为 | 代码 |
|---|---|---|
| 上下文组装 | `msgs = [system] + messages[pointer:]`，system 恒为第 0 条 | `agent/loop.go:566-620`；调用点 `agent/stream_response.go:104`、`agent/loop.go:753` |
| 压缩坍缩 | `reorganize_context` 后坍缩为 `[system, user(summary)]`，其余全部丢弃 | `agent/reorganize_context.go:49-108`（工具）、`:132/148/150`（坍缩三处赋值） |
| window 裁剪 | 裁剪为 `[system, lastUser]` | `agent/reorganize_context.go:217-224` |
| 解析失败回滚 | 截断到最后一条 assistant 消息之前 | `agent/run_stream.go:1590/1605/1636` |
| retry-from / 回退 | 截断到指定消息之前 | `agent/agent.go:2321/2345/2349` |
| 历史替换 | `SetHistory`（会话加载/导入/重置） | `agent/agent.go:851/897/2111-2126` |
| 指针调整 | 任务计划变更、`attempt_completion.task_message_no` 前移指针（跳过历史） | `agent/run.go:296-312`、`agent/run_stream.go:1849-1865`、`agent/agent.go:2665-2675` |
| loop 历史修正 | 只 **读取** 最近 N 条构造反馈，不裁剪消息 | `agent/loop.go:1348-1360` |

**结论**：pin 机制需要在上述前 6 类位置做豁免（其中"指针调整"由"上下文组装"统一覆盖）。

### 2.2 已有"豁免"先例（可对照，不要混淆）

`contextOverflowExemptTools`（`agent/reorganize_context.go:157-196`，FIX-510）是**工具执行豁免**（上下文超阈值时仍允许执行 `reorganize_context`/`attempt_completion`），与"消息豁免"不是一回事，但说明代码库接受"豁免名单"这一设计取向。

### 2.3 消息结构与会话持久化

- `llm.Message{Role, Content, ContentParts, ToolCallID, ToolCalls, ReasoningContent}`（`llm/client.go:71-78`）——**无 pin 字段**，需新增。
- `store.SessionEntry{ID, Title, Keywords, SystemPrompt, Messages, MessageCount, CreatedAt, UpdatedAt}`（`store/bolt.go:61-70`）——会话保存 system + 消息；加载集合需新增字段。
- 消息以 JSON 序列化保存（`Messages []byte`），新增字段天然随会话持久化。

### 2.4 感知与显示通道

| 层次 | 现状 | 位置 |
|---|---|---|
| 事件 | `StreamCallback` + `*Event()` 构造函数 | `agent/events.go:108/176-346` |
| 前端事件分派 | `app.js` 按 `ev.type` 处理（`task_plan`/`sub_agents`/`token_iter`…） | `web/static/app.js:1844-2400` |
| 前端状态区 | "任务进展"面板、"子代理"标题+列表 | `web/static/index.html:471/476-477`、`web/static/app.js:3275 renderSubAgents` |
| 设置页 | 设置项类型仅 `bool`(54)/`number`(49)/`string`(17)/`enum`(17)/`logo`(1)，**无列表型控件** | `cmd/settings_web.go`（`Type:` 声明） |
| 配置中的映射型字段先例 | `OutputCategories map[string]bool`、`PromptSections []PromptSection`、`WorkModes []WorkMode` | `config/config.go:142/1073/1075` |

### 2.5 新工具接线点

| 事项 | 位置 |
|---|---|
| 工具定义 | `agent/tools.go:47 buildTools()`、`:68 buildToolsInternal()` |
| 工具分发 | `agent/tools.go:2235 executeToolCall()` |
| 权限默认 | `agent/agent.go:297 DefaultToolModes()` |
| XML 模式用法文案 | `agent/toolcall_mode.go`（如 `:121` 附近映射）+ `i18n/{zh,en}_system.go` |
| 结果摘要/错误文案 | `agent/tool_summary.go`、`agent/tool_error.go`（可选） |

### 2.6 系统提示词与 SKILLS 节

- SKILLS 节渲染：`agent/system_prompt.go:664-684`（索引 + 末尾说明，始终输出）。
- `rebuildSystemPrompt()`：`agent/agent.go:1372-1440`（原地替换 `messages[0]`）。
- 先例：任务计划文本 `taskPlanText` 是"状态型内容注入提示词"的既有范式（`agent/agent.go:1756/1432/2072`、`{TASK_PLAN}` 替换 `agent/agent.go:1944`）。

---

## 3. 方案设计（pin 消息）

### 3.1 状态模型

```go
// agent/skill_state.go（新增）
type LoadedSkill struct {
    Name     string    // frontmatter name（索引中的名字，唯一）
    Path     string    // SKILL.md 绝对路径
    Dir      string    // skill 目录（用于列 scripts/ 等资源）
    Bytes    int       // 正文字节数
    SHA256   string    // 加载时快照指纹（用于"文件已变更"提示与去重）
    Source   string    // "workspace" | "global"
    LoadedAt time.Time
}
```

- `Agent` 持有 `loadedSkills map[string]LoadedSkill`（受 `a.mu` 保护），暴露 `LoadedSkills() / LoadSkill(name) / UnloadSkill(name) / LoadedSkillExists(name)`，供 **提示词构建、命令、事件、设置页**共用同一份状态。
- 与 `pinned` 消息的关系：状态表是"权威索引"，pin 消息是"内容载体"；两者由工具实现同时维护，恢复时以状态表为准做一致性校验（§3.8）。

### 3.2 工具（R1）

| 工具 | 参数 | 授权 | 行为 |
|---|---|---|---|
| `skill_load` | `name` | **auto** | 校验（存在/未被禁用）→ 读盘全文 → 插入 pinned 消息对 → 置状态 → `rebuildSystemPrompt()` → 发 `skill_state` 事件 → 返回简短结果 |
| `skill_unload` | `name` | **auto** | 校验已加载 → 置状态移除 + 取消 pin → `rebuildSystemPrompt()` → 发事件 → 返回简短结果 |

**命名依据（回答 D-A 备注）**：项目内"工具族"的既有命名主流是 `<命名空间>_<动词>`：`memory_search`、`vault_add`、`shell_send`、`word_read`、`excel_edit`、`browser_click`、`board_send`、`sub_agent_status`。因此本族取 **`skill_load` / `skill_unload`**：

- `load_skill` 倒装，偏离族命名惯例；
- `use_skill` 语义模糊（"使用"不等于"加载进上下文"，且与卸载不对称）；
- 若后续需要列出/刷新，可平滑扩展为 `skill_list` / `skill_reload`。

**工具结果（`skill_load` 返回值）**：

```text
SKILL LOADED: image-generate
source: workspace | file: /…/skills/image-generate/SKILL.md | 12,431 B | sha256: ab12cd…
The full content below is now pinned in this conversation and stays available until you call skill_unload.
Bundled resources: scripts/generate_image.py, README.md

----- SKILL.md BEGIN -----
<SKILL.md 全文，逐字节原样，不摘要、不截断>
----- SKILL.md END -----
```

- 正文承载在**工具结果消息**里 → 天然出现在 Web UI 聊天流与 `:context` 中（用户/co-shell 直接可见，R2 的天然组成部分）。
- `skill_unload` 返回：`SKILL UNLOADED: image-generate — 保护已解除；内容将在下一次上下文重整（reorganize）或裁剪时从上下文中移除。`
- 重复 `skill_load` 同名 = **刷新**：先把旧消息取消 pin，再插入新的 pinned 消息（避免两份正文长期并存；旧的将在下次重整时消失）。
- 幂等与报错：未找到 → 明确错误；已加载 → 刷新语义（结果说明）；被禁用 → 明确错误并提示在设置页开启。

### 3.3 pin 的载体与实现要点（D-B 核心）

**pin 的对象是"成对消息"**，不是单条：

| 消息 | 角色 | 说明 |
|---|---|---|
| ① `assistant`（含 `tool_calls`） | 模型发起的 `skill_load` 调用 | 必须保留，否则 ② 成为孤儿 tool 消息（OpenAI 会 400；项目内同类约束见 `agent/reorganize_context.go:167-170`） |
| ② `tool`（结果） | 承载 SKILL.md 全文 | 内容载体，`Pinned = true` |

实现要点：

1. **消息字段**：`llm.Message`（`llm/client.go:71-78`）新增 `Pinned bool \`json:"pinned,omitempty"\``；随会话 JSON 持久化（`store.SessionEntry.Messages`）。
2. **发送前剥离**：向 LLM 发起请求的序列化路径需剔除 `pinned`（避免未知字段影响第三方兼容性）——实现时复核 `llm` 包请求构造处。
3. **豁免点（必须全部覆盖，缺一处即可能丢内容）**：

| # | 位置 | 豁免做法 |
|---|---|---|
| 1 | `agent/loop.go:566-620` `buildContextMessages()`（调用点 `agent/stream_response.go:104`、`agent/loop.go:753`） | pinned 消息**无视 messagePointer 恒纳入**；结果 = `[system] + pinned（原相对顺序） + messages[pointer:]`，去重且保持顺序 |
| 2 | `agent/reorganize_context.go:132/148/150` `collapseAfterReorganize()` | 坍缩为 `[system] + pinned… + [user(summary)]` |
| 3 | `agent/reorganize_context.go:217-224`（window 模式裁剪） | 保留 pinned，仅对非 pinned 历史裁剪 |
| 4 | `agent/run_stream.go:1590/1605/1636`（解析失败回滚）、`agent/agent.go:2321/2345/2349`（retry-from 截断） | 截断时保留 pinned；若截断点之后存在 pinned，重排到 system 之后 |
| 5 | `agent/agent.go:851/897/2111-2126`（`SetHistory`） | pinned 标记随消息序列化保留，无需额外处理（复核导入路径） |

4. **指针语义**：`messagePointer` 只影响非 pinned 历史；`attempt_completion.task_message_no` 与任务计划造成的指针前移不会影响已加载内容。
5. **提示词层标注**：SKILLS 节（`agent/system_prompt.go:664-684`）对已加载项加前缀并在节末补一句说明，例如
   `- [已加载] image-generate: …（正文已在上文工具结果中，**不要重复 read_file**；如需卸载请调用 skill_unload）`。
   实现上需把加载集合传入提示词构建（`promptEnv` 增加字段），不改节列表、不加新节。

### 3.4 卸载语义（按用户备注）

- `skill_unload` **不物理删除**消息内容，只做两件事：① 从加载集合移除；② 把对应消息 `Pinned=false`。
- 之后：SKILLS 索引去掉 `[已加载]` 标记；Web UI"已加载技能"区移除该项；`skill_state` 事件通知前端。
- 内容将在**下一次上下文重整或裁剪**时随普通历史被丢弃（`collapseAfterReorganize` / window 裁剪天然完成，无需额外代码）。
- 文案必须写明这一时间差，避免用户误以为"立即消失"。
- 可选优化（本期不做，登记为后备）：卸载时把工具结果正文替换为一行 `[SKILL UNLOADED: name]` 占位，立即回收 token。

### 3.5 抗压缩保证链（R3 逐环节）

1. **加载**：`skill_load` 插入 pinned 消息 → 内容进入真实历史（可见、可查、可持久化）。
2. **每轮发送**：`buildContextMessages()` 把 pinned 消息无条件纳入（豁免点 1）→ 无论指针在哪，正文都在。
3. **压缩**：`reorganize_context` 坍缩历史时保留 pinned（豁免点 2）→ 内容**不参与摘要**，因此不会因摘要质量而退化。
4. **裁剪**：window 模式裁剪（豁免点 3）、解析失败回滚与 retry-from 截断（豁免点 4）均保留 pinned。
5. **会话恢复**：pinned 消息随会话保存/恢复（§3.8）→ 重启后仍在。
6. **卸载**：取消 pin 后，内容按 §3.4 在下次重整时消失——此后不再受任何保护，符合"未卸载前保持完整"的要求边界。

**快照语义（必须在文档与提示中讲清）**：pin 的内容是**加载时刻的快照**，压缩不会使它退化；但**磁盘文件后续变更不会自动同步**。刷新手段：再次 `skill_load`（= 刷新），或 `:skill reload <name>`；若开启"文件变更提示"，则在提示词/REPL 中提示"技能文件已变更（sha256 不同），可 reload"。

### 3.6 感知与显示（R2）

| 受众 | 手段 | 具体做法 |
|---|---|---|
| **LLM / co-shell** | 提示词层 | SKILLS 节索引对已加载项加 `[已加载]`（§3.3-5）；`skill_load` 结果本身即在上下文中可见 |
| **用户（REPL）** | 即时行 + 命令 | 加载/卸载时输出一行（如 `🔧 skill 已加载：image-generate（12.4 KB，累计 12.4 KB）`），并随工具调用行展示；`:skill list` 增加 `[loaded]`、来源、大小、sha256 前 8 位；新增 `:skill load｜unload｜reload <name>` 供用户干预（与 LLM 共享同一状态） |
| **用户（Web UI）** | 常驻状态区 | 新增事件 `skill_state`（构造参照 `SubAgentsEvent`，`agent/events.go:108`）→ `app.js` 在事件分派（`web/static/app.js:1844` 起）新增分支 → 渲染到"任务进展/子代理"同区域的**已加载技能**列表（容器样式复用 `web/static/index.html:471-477`，渲染参考 `renderSubAgents` `app.js:3275`） |
| **用户（设置页）** | 启用/禁用管理 | 见 §3.7 |

事件载荷建议（一次全量，前端整体重绘，符合项目"状态变化即刷新"规范）：

```json
{"type":"skill_state","action":"load|unload|config",
 "name":"image-generate","bytes":12431,"total_bytes":12431,
 "loaded":[{"name":"image-generate","source":"workspace","bytes":12431,"sha256":"ab12cd34","loaded_at":"2026-10-02T16:31:02+08:00"}]}
```

### 3.7 启用/禁用配置与设置页（D-D 新增要求）

**配置结构**（`config/config.go`，命名对齐既有风格）：

| 字段 | JSON 键 | `:set` 键 | 默认 | 语义 |
|---|---|---|---|---|
| `SkillsDefaultEnabled bool` | `skills_default_enabled` | `skills-default-enabled` | **true（启用）** | 对**新增或用户未配置过**的 skill 的默认状态 |
| `SkillsEnabled map[string]bool` | `skills_enabled,omitempty` | （由设置页维护） | 空 | 用户显式选择：`name → enabled`（先例：`OutputCategories map[string]bool`，`config/config.go:142`） |

判定：`SkillEnabled(name) = SkillsEnabled[name] if present else SkillsDefaultEnabled`。

**语义**：

- 禁用 → ① 不出现在 SKILLS 节索引；② `skill_load` 拒绝并提示"该技能已在设置中被禁用"；③ 若当前已加载则自动卸载（取消 pin + 发 `skill_state`）。
- 默认启用关闭（`skills-default-enabled=false`）→ 新出现的 skill 默认禁用，LLM 看不到也无法加载，需用户在设置页显式开启（适合"白名单"用法）。
- 任何配置变更 → `rebuildSystemPrompt()`（先例 `agent/settings_tools.go:784/791/798`）+ 发 `skill_state`（`action=config`）。

**设置页（Web UI 系统设置新增 "Skills" 页）**：

- 上半部分：当前 skill 清单（名称、来源、SKILL.md 大小、描述、是否已加载）+ 每项一个**滑动开关**（启用/禁用）；未配置项显示为"跟随默认"（视觉上与显式值区分）。
- 下半部分：**"默认启用"**开关（对应 `skills_default_enabled`），并附说明"对新增或未配置过的 skill 生效"。
- 控件实现遵循 `.rules/前端控件使用规范/`（开关控件、容器与布局规范）；需要时用 `read_file` 读取该规则目录下的分文件。
- **接线注意**：现有通用设置项类型只有 `bool/number/string/enum/logo`（`cmd/settings_web.go`，无列表型控件）→ 该页建议做成**专用页面 + 专用端点**（如 `GET/POST /api/skills/config`），而不是硬塞进通用设置项数组；Web 侧遵循既有的 API/前端分层。

### 3.8 状态持久化与恢复（D-C）

- **存储**：`store.SessionEntry`（`store/bolt.go:61-70`）新增 `LoadedSkills []string`（json `loaded_skills,omitempty`；旧记录缺失即空集，向后兼容）；pin 标记随 `Messages` 一并持久化。
- **保存路径**：会话持久化处写入当前加载集合名字列表（`agent/agent.go:986-989 PersistSessionNonSystem` 及 `web/session.go`、`cmd/session.go` 的保存分支）。
- **恢复路径**（三处入口：`main.go` run 会话加载、`web/session.go`、`cmd/session.go`）：读回 messages（含 pinned 内容）→ 以 `LoadedSkills` 校正状态表 → 一致性校验：

| 情况 | 处理 |
|---|---|
| pinned 消息在场 | 直接沿用（内容为**加载时快照**） |
| 文件 sha256 与快照不同 | 标记"内容已变更"，REPL/`:skill list` 提示可用 `:skill reload` 刷新 |
| 技能已删除或被禁用 | 自动取消 pin + 告警，状态表移除 |
| 状态有名字但找不到 pinned 消息 | 自动重新 load，维持"已加载 ⇒ 内容在场"强一致 |

- **不变式（须有测试）**：加载集合中的每个 name，都能在消息里找到一条内容完整的 pinned 对应项。

### 3.9 成本与告警（D-E：不设硬上限）

- `skill_load` 结果与 `skill_state` 事件均报告：本文件字节数、**估算** token（字节/4 粗估，明确标注为估算）、累计已加载字节。
- 阈值告警（可配置，默认值）：单文件 > **32 KB** 或累计 > **64 KB** 时，在工具结果 + REPL 行 + 事件中带 `warn`（**不拒绝加载**）。
- 缓存成本：pin 内容作为历史消息参与请求，新增/移除会使其后位置的提示词缓存失效（项目已在观测缓存命中率 `MetaKeyCacheRate`，FEATURE-546）；建议在加载提示中给出"本次加载约增加每轮 N tokens"的估算。

### 3.10 边界与风险

- **子 Agent**：独立进程/独立工作区（`subagent/launch.go:199-210`），不继承父加载集合；其可见技能集合边界见 `docs/skills-mechanism.md` D-006。
- **安全**：SKILL.md 属外部内容，pin 后进入历史高权重区。缓解：设置页可禁用；结果与事件标注 `source=workspace|global`；（可选）增加"仅允许 workspace 来源被加载"的配置。
- **与提示词静态化取向（FEATURE-472）**：本方案**不改系统提示词正文**（仅索引加标记）→ 无冲突。
- **名称冲突（D-003）**：加载按 `ScanSkills` 结果解析（工作区优先）；如需精确来源，可扩展 `source` 参数（本期不做）。
- **卸载残留窗口**：unload 到下一次重整之间，模型仍可能看到已卸载内容（用户已确认该语义，文案中明确说明）。
- **明确不做**：不解决"模型完全忽略 SKILLS 节"的情形；不做语义自动匹配；不做正文截断（保证完整性）。

---

## 4. 实现要点清单（按文件）

| 文件 | 改动 | 测试点 |
|---|---|---|
| `agent/skill_state.go`（新） | `LoadedSkill`、加载集合读写、上限/告警计算、事件发射 | 状态增删幂等；禁用判定；告警阈值 |
| `agent/skills_tools.go`（新） | `skill_load` / `skill_unload` 处理函数（读盘、pin 消息对、rebuild、事件） | 加载后 pinned 消息在场；卸载后 pin 取消；重复 load=刷新；禁用/未找到报错 |
| `agent/tools.go` | `buildToolsInternal()`（`:68`）注册两工具；`executeToolCall()`（`:2235`）分发 | 工具可被调用并返回约定格式 |
| `agent/agent.go` | `DefaultToolModes()`（`:297`）加两项 `auto`；`Agent` 初始化加载集合；`PersistSessionNonSystem`（`:986`）写入 `LoadedSkills` | 默认授权为 auto；会话保存含加载集合 |
| `agent/loop.go` | `buildContextMessages()`（`:566-620`）豁免 pinned（无视 pointer、保持顺序） | 指针前移后 pinned 仍在；顺序稳定 |
| `agent/reorganize_context.go` | 坍缩（`:132/148/150`）与 window 裁剪（`:217-224`）保留 pinned | 压缩后正文逐字节完整 |
| `agent/run_stream.go` / `agent/agent.go` | 解析失败回滚（`run_stream.go:1590/1605/1636`）、retry-from 截断（`agent.go:2321/2345/2349`）保留 pinned | 回滚/回退后 pinned 仍在 |
| `agent/system_prompt.go` | Skills 分支（`:664-684`）加 `[已加载]` 标记；`promptEnv` 增加加载集合 | 标记与状态一致 |
| `llm/client.go` | `Message` 增加 `Pinned bool`（`:71-78`）；请求序列化前剥离该字段 | JSON 往返保留；请求体不含 `pinned` |
| `agent/events.go` | 新增 `SkillStateEvent(...)`（参照 `:108`） | 事件字段完整 |
| `agent/toolcall_mode.go`、`i18n/zh_system.go`、`i18n/en_system.go`、`i18n/keys.go` | 工具描述/用法示例、提示词标记说明、结果与告警文案（**中英双语**） | 中英均无键名泄漏 |
| `cmd/skill.go` | `:skill list` 标注 loaded/来源/大小；新增 `load｜unload｜reload` 子命令 | 命令与状态一致 |
| `repl/repl.go` | 加载/卸载行渲染（事件 → 行） | REPL 可见 |
| `web/static/app.js`、`web/static/index.html` | `skill_state` 事件分支 + "已加载技能"区渲染；设置页 Skills 页（清单 + 滑动开关 + 默认开关） | 前端显示与后端一致 |
| `cmd/settings_web.go`、`web/`（API） | Skills 专用配置端点（清单/开关/默认开关） | 开关持久化、生效（索引/加载同步） |
| `config/config.go` | `SkillsDefaultEnabled`、`SkillsEnabled`、告警阈值项 + `:set` 接线 + 默认值 | 默认值与非配置项判定 |
| `store/bolt.go` | `SessionEntry.LoadedSkills`（向后兼容） | 旧会话可读 |

---

## 5. 验收标准（UC 草案）

1. **R1-1**：`skill_load` 后，命中消息中存在 pinned 的 tool 结果，且其正文与磁盘 SKILL.md **逐字节一致**（断言 sha256）。
2. **R1-2**：`skill_unload` 后状态表移除、pin 取消、索引标记消失；再次 `skill_load` 可重新加载（刷新语义正确，不出现两份长期并存）。
3. **R2-1**：REPL 加载/卸载各输出一行提示；`:skill list` 显示 `[loaded]`、来源与大小。
4. **R2-2**：Web UI 收到 `skill_state` 后"已加载技能"区出现/移除条目；设置页开关可改变索引与可加载性。
5. **R3-1**：加载后调用 `reorganize_context` → 坍缩为 `[system, pinned…, user(summary)]`，正文完整（逐字节比对）。
6. **R3-2**：加载后执行 window 裁剪 → 正文仍在且完整。
7. **R3-3**：加载后前移 `messagePointer`（任务计划/`task_message_no`）→ 正文仍在。
8. **R3-4**：加载后触发解析失败回滚 / retry-from 截断 → 正文仍在。
9. **R3-5**：会话保存 → 重启 → 恢复：加载集合与 pinned 内容按 §3.8 一致；文件变更时给出 reload 提示。
10. **配置**：`skills-default-enabled=false` 时新 skill 默认禁用（不进索引、不可 load）；设置页开关生效且持久化；禁用已加载技能会自动卸载。
11. **告警**：超过阈值仅告警不拒绝；事件/结果/REPL 三处一致。
12. **质量门**：`gofmt` 干净、`go build ./... && go vet ./...` 通过、新增单测全绿、既有失败集与基线逐项一致。

---

## 6. 后续（本期之外）

| 项 | 说明 |
|---|---|
| **D-001 另开 FIX** | `description: >-` 块标量解析失效（`agent/skill.go:141-150`）——用户决定分开处理；修好后可提高技能选择的准确率 |
| D-002 增强 | 语义自动匹配 / 触发词解析（如需，可在设置页加"自动加载"开关） |
| D-003 | 同名冲突与 `remove` 只能删胜出份；后续可加来源标注与 `--global` |
| D-007 | 远程搜索/下载/更新/校验（`notes/TODO.md:42`） |
| 可选优化 | 卸载时立即以占位替换正文回收 token；`skill_reload` 作为独立工具 |

## 7. 变更记录

| 日期 | 内容 |
|---|---|
| 2026-10-02 | 初版：现状约束 + 三载体对比（A 提示词分段 / B 投影注入 / C pin 消息） |
| 2026-10-02 | 按用户决策定稿：工具 `skill_load`/`skill_unload`（auto）、载体取 **C pin 消息**（卸载仅在重整时清理）、会话级持久化、提示词+REPL+Web 三处可见、新增设置页 Skills 启用/禁用与"默认启用"、不设硬上限改为告警、D-001 另开 FIX |

---

## 8. 修订记录（2026-10-02，缓存友好化改造）

**触发**：把 SKILLS 索引放在系统提示词里，会让每次 skill 加载/卸载都改写提示词前缀，导致整段 prompt-prefix 缓存失效，代价过大。用户据此要求调整感知机制（并确认了两项选择）。

| 项 | 原方案 | **现方案** |
|---|---|---|
| 系统提示词 SKILLS 节 | 动态索引 + `[已加载]` 标记 | **纯静态规则**（skill_load/skill_unload 用法、卸载语义、可用清单在 env_details）——加载/卸载不再改写提示词 |
| 可用技能清单 | 提示词索引（名称 + 简介 + 路径） | **最新 `<environment_details>` 中的 `<skills>` 元素**：名称 + 简介 + `[已加载]` 标记，过滤禁用项、不含路径；只出现在请求里**最后一个** env_details |
| pinned 消息在请求中的位置 | `[system] + pinned + messages[pointer:]`（紧跟 system） | **保持原始相对位置**，只把被指针切掉的 pinned 消息补回 → 加载/卸载只改变请求尾部 |

**R1/R2/R3 不变**：工具与授权不变；可见性由 env_details 清单 + `skill_state` 事件 + REPL 行 + Web 区 + 设置页共同承担；正文完整性仍由「pin 闭包在五处历史改写点豁免 + skill 工具结果不截断」保证。

**实现要点**：`agent/skill.go`（`buildSkillsEnvBlock` / `injectSkillsEnvToLast` / `injectEnvBlock`，注入时复制 ContentParts 以免污染冻结历史）、`agent/system_prompt.go`（Skills 节静态化；删除 promptEnv/promptIdentity 的 `loadedSkills` 字段）、`agent/loop.go`（`buildContextMessages` 的 `emitRange` 按原序输出 + 请求期注入 env 清单）、`i18n/{zh,en}_system.go`（新静态文案；移除 `system_prompt_skills_loaded_note` 键）。
