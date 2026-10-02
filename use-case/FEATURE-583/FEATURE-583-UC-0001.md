# FEATURE-583-UC-0001 skill 加载/卸载机制 · 单元与集成用例

> 分支：`FEATURE-583` ｜ 版本：`v0.91.0` ｜ 编号：FEATURE-583
> 方案依据：`docs/skill-load-unload-design.md`（决策已定稿版，D-A~D-F 经用户确认）
> 现状与缺陷背景：`docs/skills-mechanism.md`
> 本文覆盖**代码层（单元 / 集成）**用例；运行时端到端条目见 `FEATURE-583-UC-0002.md`。

---

## 一、设计与约定（用例依据）

### 1. 工具（D-A）

| 工具 | 参数 | 授权 | 语义 |
|---|---|---|---|
| `skill_load` | `name` | **auto** | 校验（存在 / 未被禁用）→ 读全文 → 插入成对 pinned 消息 → 置状态 → `rebuildSystemPrompt()` → 发 `skill_state` 事件 → 返回含元信息与正文的结果 |
| `skill_unload` | `name` | **auto** | 校验已加载 → 状态移除 + 取消 pin → `rebuildSystemPrompt()` → 发事件 → 返回"将在下一次重整/裁剪时移除" |

命名遵循项目工具族惯例 `<命名空间>_<动词>`（`memory_search` / `vault_add` / `shell_send` / `word_read` / `browser_click` / `board_send`）。

### 2. 状态（`agent/skill_state.go`）

```
LoadedSkill{ Name, Path, Dir, Bytes, SHA256, Source("workspace"|"global"), LoadedAt }
Agent.loadedSkills map[string]LoadedSkill   // a.mu 保护
LoadedSkills() / LoadSkill(name) / UnloadSkill(name) / SkillLoaded(name)
```

### 3. pin 载体（D-B）

- pin 的对象是**成对消息**：`assistant`（含 `tool_calls`）+ `tool`（结果，承载 SKILL.md 全文），二者 `Pinned = true`。
- `llm.Message` 新增 `Pinned bool \`json:"pinned,omitempty"\``；**向 LLM 发起请求前必须剥离该字段**。
- 正文以 `----- SKILL.md BEGIN -----` / `----- SKILL.md END -----` 包裹，中间为逐字节原文（不摘要、不截断），便于断言。

### 4. 五处豁免点（R3 实现要点）

| # | 位置 | 期望行为 |
|---|---|---|
| 1 | `agent/loop.go:566-620` `buildContextMessages()` | pinned 无视 `messagePointer` 恒纳入；结果 = `[system] + pinned（原顺序） + messages[pointer:]`，不重复 |
| 2 | `agent/reorganize_context.go:132/148/150` `collapseAfterReorganize()` | 坍缩为 `[system] + pinned… + [user(summary)]` |
| 3 | `agent/reorganize_context.go:217-224` window 裁剪 | 保留 pinned |
| 4 | `agent/run_stream.go:1590/1605/1636`、`agent/agent.go:2321/2345/2349` | 回滚 / retry-from 截断时保留 pinned |
| 5 | `agent/agent.go:851/897/2111-2126` `SetHistory` | pin 标记随 JSON 持久化保留 |

### 5. 卸载语义（用户确认）

`skill_unload` **不物理删除内容**：仅状态移除 + `Pinned=false`；内容在下一次 `reorganize_context` 坍缩或 window 裁剪时被丢弃。工具结果文案须写明该时间差。

### 6. 配置与设置页（D-D）

| 配置 | JSON 键 | 默认 | 语义 |
|---|---|---|---|
| `SkillsDefaultEnabled` | `skills_default_enabled` | **true** | 新增 / 未配置过的 skill 的默认启用状态 |
| `SkillsEnabled` | `skills_enabled,omitempty` | 空 | 用户显式选择 `name → enabled` |
| 告警阈值 | 复用既有配置风格 | 单文件 32 KB / 累计 64 KB | 超阈值**仅告警，不拒绝**（D-E） |

判定：`SkillEnabled(name) = SkillsEnabled[name] 若存在，否则 SkillsDefaultEnabled`。

### 7. 持久化（D-C）

`store.SessionEntry` 新增 `LoadedSkills []string`（`loaded_skills,omitempty`；旧记录缺失=空集）。
**不变式**：加载集合中的每个 name，都能在消息里找到内容完整的 pinned 对应项；异常时自动重新加载。

### 8. 事件（R2）

`skill_state`：`{type, action: load|unload|config, name, bytes, total_bytes, loaded:[{name, source, bytes, sha256, loaded_at}], warn?}`。

---

## 二、单元与集成用例

### A 类：状态与工具

#### UC-0001 `skill_load` 成功加载
- **类型**：单元（`agent`）
- **前置**：临时工作区含 `skills/demo/SKILL.md`（正文含唯一标记串）；Agent 状态为空
- **步骤**：调用 `skill_load{name:"demo"}`；检查状态表与消息序列
- **预期**：① 状态表出现 `demo`（Bytes/SHA256/Source 正确）；② 消息尾部为 `assistant(tool_calls)` + `tool(结果)` 且**二者 `Pinned=true`**；③ 结果含 BEGIN/END 包裹，中间正文与磁盘**逐字节一致**；④ 结果含来源、字节数、估算 token、目录内资源清单

#### UC-0002 `skill_load` 异常路径
- **类型**：单元
- **步骤**：分别以不存在的 name、被禁用的 name 调用
- **预期**：均返回明确错误（不 panic、不改变状态表）；禁用场景的错误提示指向设置页开启

#### UC-0003 `skill_load` 重复调用 = 刷新
- **类型**：单元
- **前置**：已加载 `demo`
- **步骤**：修改 `SKILL.md` 内容后再次 `skill_load{name:"demo"}`
- **预期**：① 旧消息对 `Pinned=false`；② 新增一对 pinned 消息，正文为新内容；③ 状态表 SHA256 更新；④ 不会长期存在两份**同时 pinned** 的正文

#### UC-0004 `skill_unload` 语义
- **类型**：单元
- **前置**：已加载 `demo`
- **步骤**：调用 `skill_unload{name:"demo"}`；检查状态表、消息 pin 标记与正文
- **预期**：① 状态表移除；② 对应消息 `Pinned=false`；③ **正文仍存在于消息中**（未立即删除）；④ 结果为"已解除保护，将在下一次上下文重整/裁剪时移除"语义

#### UC-0005 `skill_unload` 异常路径
- **类型**：单元
- **步骤**：对未加载的 name 调用 `skill_unload`
- **预期**：返回明确错误，状态表不变

### B 类：pin 豁免（R3 核心）

#### UC-0006 指针前移后 pinned 仍在（豁免点 1）
- **类型**：单元
- **前置**：加载 `demo` 后，插入若干消息并把 `messagePointer` 移到 pinned 之后
- **步骤**：调用 `buildContextMessages()`
- **预期**：`[0]=system`；pinned 对紧随其后；其后才是 `messages[pointer:]`；正文逐字节完整

#### UC-0007 压缩坍缩后 pinned 仍在（豁免点 2）
- **类型**：单元
- **前言**：加载 `demo` → 设置 `reorganizeContextUsed` → 构造 summary 场景
- **步骤**：调用 `collapseAfterReorganize()`
- **预期**：消息序列为 `[system, pinned(assistant), pinned(tool), user(summary)]`（或不存在 summary 时的对应形态）；**pinned 正文完整**；`messagePointer` 指向 summary 消息

#### UC-0008 window 裁剪后 pinned 仍在（豁免点 3）
- **类型**：单元
- **前置**：加载 `demo`；`ContextPolicy=window` 且 `ContextLimit=-1`
- **步骤**：触发 window 裁剪路径
- **预期**：仅普通历史被裁剪；pinned 对保留且完整

#### UC-0009 回滚与 retry-from 截断保护（豁免点 4）
- **类型**：单元
- **步骤**：加载 `demo` 后分别触发：解析失败回滚、retry-from 截断（截断点晚于 pinned）
- **预期**：两种场景下 pinned 都在；若截断点越过 pinned，pinned 被重排到 system 之后并保持顺序与完整性

#### UC-0010 多技能与顺序稳定
- **类型**：单元
- **步骤**：依次加载 `a`、`b`；再次 `buildContextMessages()` 多次
- **预期**：pinned 顺序稳定、无重复；卸载 `a` 后 `b` 仍在且 `a` 的 pin 已取消

### C 类：提示词与事件（R2）

#### UC-0011 SKILLS 节静态化 + env_details 清单（缓存友好，2026-10-02 修订）
- **类型**：单元
- **步骤**：① 渲染系统提示词；② 渲染请求中最后一个 `<environment_details>`
- **预期**：① 系统提示词 SKILLS 节**只含静态规则**（skill_load/skill_unload 用法 + 卸载语义 + 指向 env_details），不含任何技能清单或已加载标记，**加载/卸载前后逐字节不变**；② 可用技能清单（名称 + 简介 + `[已加载]` 标记、过滤禁用项、不含路径）出现在**请求中最后一个** env_details 的 `<skills>` 元素里；③ 加载/卸载只改变请求尾部，pinned 消息保持原位置（前缀缓存保留）

#### UC-0020 缓存友好性（新增，2026-10-02）
- **类型**：单元
- **步骤**：构造含 env 的多轮历史 → 记录 `buildContextMessages()` → 加载 skill → 再次记录
- **预期**：① pinned 对追加在请求**尾部**，消息顺序不变、系统提示词不含技能清单；② `<skills>` 块在整个请求中只出现一次，且位于最后一个 env 里

#### UC-0012 `skill_state` 事件
- **类型**：单元
- **步骤**：分别触发 load / unload / 配置变更
- **预期**：事件 `type=skill_state`，`action` 正确，`loaded[]` 为全量快照（name/source/bytes/sha256/loaded_at），含 `total_bytes`；超阈值时带 `warn`

### D 类：配置与禁用

#### UC-0013 启用/禁用判定
- **类型**：单元（`config` + `agent`）
- **步骤**：构造三种情形：① 显式 `true`；② 显式 `false`；③ 未配置 + 默认 `true`；④ 未配置 + 默认 `false`
- **预期**：判定结果分别为 启用/禁用/启用/禁用；被禁用者**不出现在 SKILLS 节索引**，且 `skill_load` 拒绝

#### UC-0014 禁用已加载技能 → 自动卸载
- **类型**：单元
- **步骤**：加载 `demo` 后在配置中将其禁用并应用
- **预期**：状态表移除、pin 取消、发 `skill_state`（action=config）；索引不再出现该技能

#### UC-0015 超阈值仅告警（D-E）
- **类型**：单元
- **步骤**：加载一个超过单文件阈值（或累计超阈值）的 SKILL.md
- **预期**：**加载成功**（不拒绝）；结果、事件、告警文案含 `warn` 与估算 token；内容仍完整

### E 类：持久化与恢复

#### UC-0016 会话保存 / 恢复一致性
- **类型**：集成（`store` + `agent`）
- **步骤**：加载 `demo` → 保存会话（`SessionEntry` 写入 `LoadedSkills`）→ 新 Agent 恢复（`SetHistory` + 状态校正）
- **预期**：加载集合恢复为 `[demo]`；`messages` 中 pinned 内容完整；不变式成立（状态 name 都能找到 pinned 正文）

#### UC-0017 磁盘变更与删除
- **类型**：集成
- **步骤**：① 加载后修改 SKILL.md（sha256 变化）再恢复会话；② 加载后删除 SKILL.md 再恢复/校验
- **预期**：① 状态保留，标记"内容已变更"并提示可用 `:skill reload`；② 自动卸载 + 告警，不残留残缺正文

#### UC-0018 旧会话兼容
- **类型**：集成
- **步骤**：读取 `loaded_skills` 缺失的旧会话记录
- **预期**：视为空集，正常加载，无报错

### F 类：请求构造

#### UC-0019 请求体剥离 `pinned`
- **类型**：单元（`llm`）
- **步骤**：构造含 `Pinned=true` 的消息，走请求序列化路径
- **预期**：序列化结果不含 `pinned` 字段；消息 JSON 往返（会话保存/读取）仍保留 `pinned`

---

## 三、质量门（全部满足方可进入运行时验收）

1. `gofmt` 对改动文件干净；
2. `go build ./... && go vet ./...` 通过；
3. 本文件 UC-0001 ~ UC-0019 对应的新增单测全绿；
4. `go test ./agent/ ./config/ ./cmd/ ./llm/ ./store/ -short -count=1` 的失败集与 `main` 基线**逐项一致**（差集为空）；
5. i18n 中英文案齐备，无键名泄漏（`i18n` 相关既有测试不新增失败）。

## 四、验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0001 ~ UC-0019 | 待执行 | — |
