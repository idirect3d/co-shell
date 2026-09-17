# FIX-532 用例：`:set defaults` 重置 `work_modes`（分支 FEATURE-531 / v0.64.0）

> 背景：`~/.co-shell/config.json` 的 `work_modes` 中残留从内置模式物化而来的 `description`
> （"行动模式 - 可执行系统命令…"等，即 i18n `KeyWorkModeActDesc/PlanDesc/ResearchDesc`）。
> `modeDescription()`（`agent/system_prompt.go:376-395`）优先级为
> 「`mode/{MODE}.md` → `cfg.WorkModes` 同名 Description → 内置详细 i18n 键 → `m.Description`」，
> 因此 ResultMode 节长期显示简短简介而非详细策略；`:set defaults` 又不重置 `WorkModes`，重置后问题依旧。

## 需求（用户确认）

1. `:set defaults` 重置 `WorkModes`（**全部清空**）。
2. 清空后若当前模式 `cfg.LLM.WorkMode` 指向被清掉的自定义模式 → 回退为 `act`。
3. 最小改动：不调整 `renameModeRetainedDirs` 的调用顺序（自定义模式目录 `mode/{自定义模式名}/` 不再纳入重命名）。

## 实现（预期改动面）

- `cmd/settings.go` `handleSetDefault()`：在 `Save()` 之前追加
  - `h.cfg.WorkModes = nil`
  - 当前模式非内置（非 act/plan/research）时 `h.cfg.LLM.WorkMode = "act"`
- `ROADMAP.md`：FIX-532 任务登记（已加）

---

## UC-0001（单元）`:set defaults` 清空 WorkModes 并落盘

**前置**：临时 cfg（`config.DefaultConfig()`）注入 2 条自定义模式（如 `review`、`expert`）与 1 条覆盖内置的模式（如 `act`，带自定义 Description）；`cfg.LLM.WorkMode = "review"`。

**操作**：调用 `handleSetDefault()`。

**预期**：
- 返回消息非空且非错误（`KeySettingsResetSuccess`）。
- `cfg.WorkModes` 为空（nil 或 len==0）。
- 重新从磁盘 `LoadFromFile` → `WorkModes` 为空 / `work_modes` 字段不存在。
- `cfg.LLM.WorkMode == "act"`（原指向被清掉的 `review`）。

**验证方式**：单元测试断言（表驱动）。

---

## UC-0002（单元）清空后 ResultMode 节回落详细策略

**前置**：`cfg.WorkModes` 含 `{Name:"act", Description:"行动模式 - 可执行系统命令、修改文件、操作浏览器等所有操作"}`（复现用户现象）。

**操作**：
1. 先断言 `buildResultModeSection(cwd, cfg)` 含该简短 Description（复现现状）；
2. 调用 `handleSetDefault()` 后再调用 `buildResultModeSection(cwd, cfg)`。

**预期**：
- 重置后 ResultMode 节 **不含** "行动模式 - 可执行系统命令"、"规划模式 - 仅分析和规划"、"调研模式 - 搜索、查阅资料"；
- 重置后 ACT/PLAN/RESEARCH 三节内容分别来自 `i18n.T(KeyWorkModeAct/Plan/Research)`（含 "attempt_completion"、"ask_user"、"研究报告" 等关键词）；
- 节内条目顺序仍为 ACT → PLAN → RESEARCH。

**验证方式**：单元测试断言（中英各跑一次）。

---

## UC-0003（单元）当前模式回退规则

| 用例 | 初始 `LLM.WorkMode` | 期望重置后 |
|------|--------------------|-----------|
| a | `review`（自定义） | `act` |
| b | `act` | `act`（不变） |
| c | `plan` | `plan`（不变） |
| d | `research` | `research`（不变） |
| e | `""`（空） | `act`（默认入口语义不变，按空值→act 处理） |

**验证方式**：表驱动单元测试。

---

## UC-0004（单元）既有保留项不受影响（回归）

**前置**：设置 `LLM.MemoryEnabled=false`、`LLM.ContextLimit=7`、`LLM.ContextPolicy="x"`、`LLM.ContextReorganizeThreshold=0.5`、`LLM.MemorySearchMaxContentLen/Results`、`DB.Path` 为特殊值；同时设置 `LLM.ResultMode`/`ShowLogo`/`ToolModes` 等为非默认值。

**操作**：调用 `handleSetDefault()`。

**预期**：
- Memory/Context 相关 6 个保留字段值不变；`DB` 整体不变。
- `ResultMode`、显示类开关等被重置为 `DefaultConfig()` 值（既有行为不变）。
- `cfg.WorkModes` 清空。

**验证方式**：单元测试断言。

---

## UC-0005（单元）幂等与边界

**前置**：`cfg.WorkModes` 为 nil / 空切片；`cfg.LLM.WorkMode = "act"`。

**操作**：连续调用 `handleSetDefault()` 两次。

**预期**：两次均返回成功消息、无 panic/无错误；`WorkModes` 保持为空；`WorkMode` 保持 `act`。

**验证方式**：单元测试断言。

---

## UC-0006（运行时 / CLI）真实配置上的端到端重置

**前置**：`~/.co-shell/config.json` 存在 `work_modes`（含 research/act/plan 三条带 `description` 的物化条目）；备份该文件。

**操作**：
1. `work/co-shell-0.64.0.darwin.arm64 run --workdir <tmp> --prompt ":set defaults"`（或交互 REPL 输入 `:set defaults`，需在提示确认时输入 y）；
2. 检查 `~/.co-shell/config.json`；
3. 再开新会话，检查系统提示词 ResultMode 节。

**预期**：
- 输出成功提示（`KeySettingsResetSuccess`）；
- `work_modes` 字段消失（或为空数组）；
- 新会话系统提示词 ResultMode 节三段为详细策略文本（含 `attempt_completion` / `ask_user` / 研究报告等要点），**不含** "行动模式 - 可执行系统命令"、"规划模式 - 仅分析和规划"、"调研模式 - 搜索、查阅资料"；
- 其余既有行为不变（MCP/Log 重置，工作区 md 个性化文件不被删除）。

**验证方式**：命令行执行 + `grep work_modes ~/.co-shell/config.json` + 系统提示词预览（Web UI 或日志）。

---

## UC-0007（运行时 / Web UI）界面执行与结果可见

**前置**：`--serve` 启动隔离实例（独立端口与 workdir）；配置中有 `work_modes`。

**操作**：
1. Web UI 输入 `:set defaults` 并确认；
2. 观察系统消息/提示；
3. 打开系统设置面板查看模式列表相关项（或新开会话触发系统提示词重建）。

**预期**：
- 界面显示重置成功提示，无报错；
- 新会话的 ResultMode 节显示详细策略文本；
- 设置面板中不再出现配置里物化的自定义模式条目。

**验证方式**：浏览器实测 + 截图/DOM 断言。

---

## UC-0008（运行时）自定义模式场景

**前置**：通过 `:mode add expert "专家模式"` 新增自定义模式，`work_modes` 中出现 expert；`:mode switch expert` 切到该模式。

**操作**：执行 `:set defaults`（确认 y），随后查看当前模式与新会话系统提示词。

**预期**：
- `work_modes` 清空（expert 消失）；
- 当前模式回退为 `act`（`cfg.LLM.WorkMode == "act"`）；
- ResultMode 节恢复为三段详细策略（不再出现 expert 条目）。

**验证方式**：REPL 命令 + 配置文件检查 + 系统提示词检查。

---

## UC-0009（运行时 / 约束验证）自定义模式目录不被重命名

**前置**：在 `{cwd}/mode/` 下创建自定义模式目录 `expert/`（可含任意文件），同时保留一个内置模式目录 `act/`（若场景需要）。

**操作**：执行 `:set defaults`。

**预期（按用户决策的最小改动）**：
- 清空 `WorkModes` 后，`knownModeNames` 仅含 act/plan/research，`mode/expert/` **不**被重命名为 `expert.YYYYMMDD`；
- 若存在 `mode/act/`（内置模式名目录），仍进入重命名清单（确认 y 后改名为 `act.YYYYMMDD`）。

**验证方式**：目录列表比对（`ls mode/`）。

---

## UC-0010（回归）既有 `:set defaults` 行为不回退

**前置**：设置若干非默认值：`MCP` 配置、`LogEnabled=true`、`LogLevel="debug"`、`LLM.ShowLogo=false`、`LLM.ResultMode=free`。

**操作**：执行 `:set defaults`。

**预期**：
- `MCP` 重置为默认、`LogEnabled/LogLevel` 重置为默认、显示类开关与 `ResultMode` 重置为默认；
- LLM 保留项（Memory/Context 系列）与 `DB` 不变；
- `WorkModes` 唯一新增的清空行为生效；
- 工作区 `./IDENTITY.md`、`./PRINCIPLES.md`、`./RULES.md`、`./CAPABILITIES.md`、`./mode/*.md` 文件均不被删除或修改。

**验证方式**：单元测试 + 文件列表比对。

---

## 验收标准

1. `go build ./... && go vet ./...` 全绿；
2. `go test ./cmd/ -run "SetDefault|Defaults" -short` 通过（新增用例）；`go test ./... -short` 无新增失败（既有失败项不属本任务）；
3. UC-0001 ~ UC-0005 单元用例通过；
4. UC-0006 ~ UC-0010 运行时用例按上述预期通过，并有实测记录（配置文件片段 / 系统提示词片段 / 目录列表）。
