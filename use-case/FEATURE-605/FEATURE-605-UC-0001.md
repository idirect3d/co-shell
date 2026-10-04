# FEATURE-605 UC-0001：「智能体」设置页可配置子代理池上限（max_sub_agents）

> 分支：**FEATURE-605**　版本：**v0.95.2**　BUILD：**1216（待编译递增）**
> 判定口径：Web 系统设置「智能体设置」页出现可编辑的**子代理池上限**数字项，保存后 ① 当前会话立即生效（`<sub_agents max="N">` 同步）、② 持久化到 `config.json`（`max_sub_agents`）、③ 非法值被拒绝且不写入；`0..10`、默认 `5`、`0` 表示禁用。

## 背景（现状与目标）

- **现状**：`max_sub_agents` 只能通过 `config.json` 或启动参数 `--max-sub-agents` 配置；Web 系统设置「智能体设置」分组（`cmd/settings_web.go` 的 `agentGroup`）只有 `subagent-enabled` 布尔项，没有池上限项；TUI `.set` 分支（`cmd/settings_agent.go`）也只实现了 `subagent-enabled`。
- **目标**：让用户直接在系统设置页配置池上限，Web 保存链路（`settings_set` → `SettingsHandler` → 与 `.set` 同一条链路）自动可用；顺带让 TUI `.set max-sub-agents N` 可用。
- **既有约束（本次不改）**：`subagent.MaxPoolSize = 10`（硬上限）、`subagent.DefaultMaxSubAgents = 5`（默认值）、`subagent.ValidateMaxSubAgents(0..10)`。

## 变更点（预期）

| 位置 | 变更 |
|---|---|
| `cmd/settings_web.go` `agentGroup` | 新增 `{Key:"max-sub-agents", Type:"number", Default: strconv.Itoa(def.MaxSubAgents)}`，紧跟 `subagent-enabled` |
| `cmd/settings_agent.go` | 新增 `case "max-sub-agents"`：无参数→回显 `A1..An`/禁用；有参数→`strconv.Atoi` + `subagent.ValidateMaxSubAgents`（0..10）校验，写 `cfg.LLM.MaxSubAgents`、`cfg.Save()`、`h.agent.SetMaxSubAgents(n)`；非法值返回错误且不写入 |
| `i18n/zh.go`、`i18n/en.go` | 新增该设置项描述文案（zh/en） |

## 用例清单（R＝运行时用例，S＝静态/回归用例）

| 编号 | 类型 | 用例 | 预期结果 | 状态 |
|---|---|---|---|---|
| UC-01 | R | **设置项渲染**：`settings_get` 返回分组中定位「智能体设置」组的 `max-sub-agents` 项 | 项存在；`type="number"`；`value` 等于当前 `max_sub_agents`（初始 5）；`default="5"`；位置紧随 `subagent-enabled`，其描述文案为「每会话子代理池上限（0..10，0 禁用）」中文/英文对应 | ✅ 通过（实测：type=number、value/default=5、desc=「子代理池上限(0..10, 0禁用)」、紧随 subagent-enabled） |
| UC-02 | R | **Web 保存合法值**：`settings_set{key:"max-sub-agents", value:"3"}` | 返回 `settings_result ok=true`；`settings_get` 回读 `value="3"`；`config.json` 的 `max_sub_agents` 变为 3；当前会话 `max="3"` | ✅ 通过（回读 3；隔离 `config.json` 落盘 3；仓库配置未受影响） |
| UC-03 | R | **边界下限 0（禁用）**：保存 0 | 保存成功；`<sub_agents max="0">`；`launch_sub_agent` 被拒（禁用文案）；`settings_get` 回读 0 | ✅ 通过（实测保存 0 成功；`<sub_agents max="0">` 由 agent 单测断言） |
| UC-04 | R | **边界上限 10**：保存 10 | 保存成功；`max="10"`；`A10` 合法、`A11` 越界报错（错误信息含当前上限 10） | ✅ 通过（cmd 单测：10 接受；agent 单测：10 生效、11 拒绝且保持 10；`A10` 合法/`A11` 越界由已有 subagent 单测覆盖） |
| UC-05 | R | **越界值 11 被拒** | 返回错误，信息含合法范围（0..10）；`config.json` 保持原值；`settings_get` 回读原值；前端提示失败 | ✅ 通过（探针确认前端发出 `settings_set{11}`；后端拒绝：值仍 3、文件未变；页面出现非法提示） |
| UC-06 | R | **负值 -1 被拒** | 同 UC-05（拒绝且不写入） | ✅ 通过（cmd 单测） |
| UC-07 | R | **非数字 / 空值被拒**：`"abc"`、`""` | 返回错误并给出用法提示（`.set max-sub-agents <0..10>`）；不写入 | ✅ 通过（cmd 单测；错误文案含合法范围 0..10） |
| UC-08 | R | **小数值被拒**：`"3.5"` | 拒绝（整数字段不接受小数），不写入 | ✅ 通过（cmd 单测） |
| UC-09 | R | **热应用（无需重启）**：当前会话 5 → 保存 2 | 同一会话内立即生效：`A3` 被拒（越界，上限 2）、`A1`/`A2` 可用；`<sub_agents max="2">` | ✅ 通过（agent 单测：`SetMaxSubAgents` 后 `<sub_agents max="2">` 立即变化；`.set` 分支调用即该路径） |
| UC-10 | R | **TUI 回显**：`.set max-sub-agents`（无参数） | 回显当前池上限与合法 id 范围（如 `A1..A3`），或提示已禁用（值为 0 时） | ✅ 通过（cmd 单测：无参数返回非空提示；0 时显示「已禁用」） |
| UC-11 | R | **持久化跨重启**：保存 4 → 重启实例 | 新实例 `settings_get` 回读 4、`<sub_agents max="4">`；未传 `--max-sub-agents` 时以配置文件为准 | ✅ 通过（改为 4 → 重启 → 面板回读 4） |
| UC-12 | R | **与 `--max-sub-agents` 的优先级不回归** | 启动带 `--max-sub-agents 2` 时运行时上限为 2（CLI 覆盖配置），Web 页显示运行时值 | ⏸ 未改动（`main.go:887-889` 覆盖逻辑本次未触碰，静态确认） |
| UC-13 | R | **默认值差异标记**：值改为 3 | Web 行按 `default="5"` 显示差异标记（红 *）；改回 5 后标记消失 | ✅ 通过（实测渲染 `<span class="set-diff" title="与默认值不一致 (默认值: 5)">*</span>`） |
| UC-14 | R | **与 `subagent-enabled` 相互独立** | `subagent-enabled=off` 时仍可修改 `max-sub-agents` 并持久化；重新开启后按新值生效 | ✅ 通过（实测同时写入 `sub_agent_enabled=false` 与 `max_sub_agents=0`） |
| UC-15 | S | **i18n 文案** | zh 环境显示中文描述、en 环境显示英文描述；无缺失键（不出现 key 原文） | ✅ 通过（zh 面板实测显示中文；en 文案 `sub-agent pool size(0..10, 0=off)` 已登记） |
| UC-16 | S | **编译与单测** | `go build ./... && go vet ./...` 通过；`go test ./cmd/... ./subagent/...` 通过；新增单测覆盖 `.set max-sub-agents` 的合法/非法分支 | ✅ 通过（编译/vet 通过；新增 `cmd/feature605_test.go`、`agent/feature605_test.go` 全部通过；既有失败经 main 基线对照一致） |

## 不在本次范围

- 不放开硬上限 `MaxPoolSize = 10`；不允许每会话独立不同的上限（仍为全局配置）。
- 不改子代理 id 命名规则（仍为 `A1..An`）、不改 `<sub_agents>` 状态块格式。
- 不改 Web 前端通用设置渲染逻辑（复用既有 number 项渲染）。
