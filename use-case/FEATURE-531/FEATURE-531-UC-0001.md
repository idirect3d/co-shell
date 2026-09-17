# FEATURE-531 系统提示词组成策略调整 — 测试用例

> 分支：FEATURE-531
> 版本：v0.64.0
> 范围：全局节列表（8 节固定顺序，不再按 mode 定义）+ ResultMode 由各 mode 策略动态拼装（跨 mode 文本一致）+ 移除 `--unload-mode`、新增 `--init`（导出 IDENTITY/CAPABILITIES/RULES 与 mode/{MODE}.md）+ 新会话实时优先读取磁盘文件否则回退系统默认值 + 废弃旧机制（mode/{mode}/*.md、WORKMODE.md、work_modes[].sections）

## 设计与约定（用例依据）

- **全局节列表（唯一真相源）**：`Identity → ToolUsage → ResultMode → Capabilities → Rules → Skills → ExternalTools → Environment`，所有 mode（内置与自定义）共用，不再有 `WorkMode.Sections` 参与拼装。
- **磁盘覆盖（仅三节）**：`./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md` 存在且非空白时覆盖对应节；其余节不支持磁盘覆盖。
- **mode 策略文件**：`mode/{MODE NAME 大写}.md`（如 `mode/ACT.md`），供 ResultMode 拼装；缺失或空白时回退 i18n（`KeyWorkModeAct/Plan/Research`）或自定义 mode 的 `Description`。
- **ResultMode 拼装**：标题 `{MODE1} V.S. {MODE2} V.S. ...`（各 mode 名大写 + ` MODE`）+ 引导句 + 每个 mode 一段（`# {MODE} MODE` + 策略正文）+ 末尾 `--init` 说明；**与当前 mode 无关**。
- **旧机制**：`mode/{mode}/{SECTION}.md`、`mode/{mode}/RESULT_MODE.md`、`./WORKMODE.md` 一律不再读取；`work_modes[].sections` 字段不再参与拼装。
- **`--init` 导出**：一次写 `./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md` 与 `mode/{MODE}.md`（全部已配置 mode，含自定义）；目标文件已存在则先备份为 `{文件名}.bak` 再覆盖；不创建 `mode/{mode}/` 子目录。
- **实时性**：每次构建系统提示词（新会话/新回合重建）都重新读盘，磁盘文件改动下一回合生效。

## 用例清单（概览）

| 组 | 用例 | 覆盖点 |
|---|---|---|
| A 全局节列表与跨模式一致性 | UC-0001 ~ UC-0006 | 统一节列表与顺序、跨 mode 提示词一致、旧配置字段失效、回归修复 |
| B ResultMode 静态拼装 | UC-0007 ~ UC-0013 | 标题/引导句、跨 mode 文本一致、mode 文件优先、回退与边界、末尾说明 |
| C 根目录节文件优先 | UC-0014 ~ UC-0019 | 三节覆盖生效、其余节不覆盖、空文件回退、缓存残留文件失效 |
| D `--init` 导出 | UC-0020 ~ UC-0025 | 导出清单、占位符原文、.bak 备份、自定义 mode、子目录与 `--unload-mode` 移除 |
| E 实时性、i18n 与回归 | UC-0026 ~ UC-0032 | 实时读盘、双语、分隔符与占位符、无文件基线、编译测试 |

---

## A 全局节列表与跨模式一致性

### UC-0001 三个内置 mode 的节列表来自全局定义且顺序固定
- **类型**：单元测试（`config` / `agent` 包）
- **前置**：`cfg.WorkModes` 为空（使用系统默认）。
- **步骤**：分别以 `act` / `plan` / `research` 构建系统提示词，提取各节出现顺序。
- **预期**：三者均为 `Identity → ToolUsage → ResultMode → Capabilities → Rules → Skills → ExternalTools → Environment`，逐项相同；不存在任何 mode 专属节（如 plan 专属 Objective）。

### UC-0002 旧配置 `work_modes[].sections` 不再影响节列表
- **类型**：单元测试
- **前置**：配置中 `work_modes` 保留历史字段，例如 `act.sections = [Identity, ToolUsage, Capabilities, Rules, ExternalTools, Environment]`（无 ResultMode，8 节白名单外的旧快照）。
- **步骤**：以该配置构建 act 模式系统提示词。
- **预期**：仍按全局 8 节顺序输出，`RESULT MODE`（ResultMode）节**必须出现**；缺失/多余的 `sections` 声明被忽略，构建不报错。

### UC-0003 回归：act 模式不再丢失 ResultMode 节（FEATURE-531 的起因）
- **类型**：单元测试（复现 FEATURE-472 后的实际症状）
- **前置**：使用与用户本机一致的旧配置快照（act/research 6 节、plan 8 节）。
- **步骤**：构建 act 模式提示词，检查是否包含 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE` 标题与三段 mode 描述。
- **预期**：**包含**（修复前实际为缺失）；plan/research 同样包含，且三者的该节内容完全一致。

### UC-0004 切换 mode 时提示词仅在“当前 mode 标识”上不同
- **类型**：端到端（serve 模式，切换 `:mode` 前后抓取系统提示词）
- **前置**：无任何磁盘覆盖文件（无 `IDENTITY.md`/`CAPABILITIES.md`/`RULES.md`/`mode/*.md`）。
- **步骤**：分别在 act / plan / research 下抓取系统提示词全文；先把 `environment_details` 中的 `current_mode` 标识剔除，再逐字节对比。
- **预期**：三份文本**完全一致**（唯一差异是 `current_mode` 与工具集描述等运行时注入信息，不在系统提示词静态部分）；ResultMode 节均列出全部模式。

### UC-0005 自定义 mode 与内置 mode 共用同一节列表
- **类型**：单元测试
- **前置**：配置新增自定义 mode `audit`（含 Description）。
- **步骤**：以 `audit` 为当前模式构建提示词。
- **预期**：节列表与顺序与内置 mode 相同（同为全局 8 节）；`audit` 出现在 ResultMode 的模式罗列中。

### UC-0006 配置层不再存在按 mode 的节列表来源
- **类型**：代码审查 + 单元测试
- **前置**：无。
- **步骤**：检索 `DefaultActSections` / `DefaultPlanSections` / `DefaultResearchSections` / `WorkMode.Sections` 的使用点。
- **预期**：提示词拼装路径只引用全局节列表（如 `config.DefaultSections()`）；旧的按 mode 节列表定义与 `getWorkModeSectionNames` 的按 mode 分支已删除；`work_modes[].sections` 字段已从类型中移除（旧配置文件中的该字段被安全忽略）。

---

## B ResultMode 静态拼装

### UC-0007 标题、引导句与分段格式
- **类型**：单元测试
- **前置**：内置三模式 + 1 个自定义 mode `audit`。
- **步骤**：构建 ResultMode 节文本。
- **预期**：首行为 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE V.S. AUDIT MODE`（顺序 = 内置在前、自定义随后，` V.S. ` 分隔）；随后为引导句（含模式数量，中英各自语言）；随后依次为 `# ACT MODE` / `# PLAN MODE` / `# RESEARCH MODE` / `# AUDIT MODE` 四段，每段正文非空。

### UC-0008 切换 mode 时 ResultMode 文本逐字节一致
- **类型**：单元测试 + 端到端
- **前置**：无磁盘策略文件。
- **步骤**：分别以 act / plan / research 构建 ResultMode 节并比对。
- **预期**：三者**完全一致**（同一字符串，且 md5 相同）；不因当前 mode 不同而只输出该 mode 的描述。

### UC-0009 `mode/{MODE}.md` 优先于 i18n 默认
- **类型**：单元测试
- **前置**：写入 `mode/PLAN.md`，内容为自定义文本（含占位符 `{AGENT_NAME}`）。
- **步骤**：构建 ResultMode 节。
- **预期**：`# PLAN MODE` 段正文为用户文件内容（占位符按规则替换）；`# ACT MODE`、`# RESEARCH MODE` 段仍为 i18n 默认内容；其他 mode 的策略文件互不影响。

### UC-0010 策略文件缺失或空白时回退系统默认
- **类型**：单元测试
- **前置**：分别构造三种情况：① `mode/ACT.md` 不存在；② 存在但内容为空；③ 存在但仅空白字符。
- **步骤**：构建 ResultMode 节。
- **预期**：三种情况均回退为 i18n `KeyWorkModeAct` 内容；不出现空段落、空标题或多余分隔符。

### UC-0011 自定义 mode 的策略来源与回退
- **类型**：单元测试
- **前置**：自定义 mode `audit`，`Description="审计模式 - 仅核查不修改"`。
- **步骤**：① 无 `mode/AUDIT.md` 时构建 ResultMode；② 写入 `mode/AUDIT.md` 后再次构建。
- **预期**：① `# AUDIT MODE` 段正文 = 该 mode 的 `Description`；② 正文 = 文件内容。

### UC-0012 策略文件命名与大小写规则
- **类型**：单元测试（含负例）
- **前置**：分别写入 `mode/ACT.md`（正确）与 `mode/act.md`（错误大小写）两种文件。
- **步骤**：构建 ResultMode 节。
- **预期**：仅 `mode/{NAME 大写}.md` 生效；`mode/act.md`（小写）被忽略，对应段仍为 i18n 默认；路径不落在 `mode/{mode}/` 子目录中。

### UC-0013 ResultMode 末尾说明改为 `--init`
- **类型**：单元测试
- **前置**：无。
- **步骤**：构建 ResultMode 节，检查末尾说明段（`KeySystemPromptResultModeNote`）。
- **预期**：说明文字指向 `--init`（中英双语均已更新），不再出现 `--unload-mode` 字样。

---

## C 根目录节文件优先

### UC-0014 `./IDENTITY.md` 覆盖 Identity 节
- **类型**：端到端（serve，工作区根写入文件后开新回合）
- **前置**：工作区根写入 `IDENTITY.md`，内容含 `{AGENT_NAME}` 占位符。
- **步骤**：构建系统提示词，定位 Identity 节。
- **预期**：Identity 节为文件内容，占位符已替换为实际 agent 名；删除文件后下一回合恢复 i18n 默认。

### UC-0015 `./CAPABILITIES.md` 覆盖 Capabilities 节
- **类型**：端到端
- **前置**：写入 `CAPABILITIES.md`；`meta-capability-enabled=true`。
- **步骤**：构建系统提示词。
- **预期**：Capabilities 节 = 文件内容 + 追加的 `META-CAPABILITIES` 索引（追加逻辑不受覆盖影响）。

### UC-0016 `./RULES.md` 覆盖 Rules 节且占位符/自定义规则拼接保持
- **类型**：端到端
- **前置**：写入 `RULES.md`（含 `{CONTEXT_REORGANIZE_THRESHOLD}` 等占位符）；工作区存在 `.rules/` 目录。
- **步骤**：构建系统提示词。
- **预期**：Rules 节 = 文件内容（占位符已替换）；`.rules/` 的加载与追加行为与覆盖前一致，不重复、不丢失。

### UC-0017 其余节不支持根目录覆盖（旧行为废弃）
- **类型**：单元测试（负例）
- **前置**：写入 `./TOOL_USAGE.md`、`./ENVIRONMENT.md`、`./SKILLS.md`（内容为显眼占位文本）。
- **步骤**：构建系统提示词。
- **预期**：这些节仍为 i18n 默认内容，文件中文本**不出现在**提示词中。

### UC-0018 旧 mode 子目录与 WORKMODE.md 全部失效
- **类型**：单元测试（负例）
- **前置**：写入 `mode/act/RESULT_MODE.md`、`mode/act/RULES.md`、`mode/plan/IDENTITY.md`、`./WORKMODE.md`（均为显眼占位文本）。
- **步骤**：分别以 act / plan 构建系统提示词。
- **预期**：以上文件内容一律**不出现在**提示词任何节中；ResultMode 仍由新机制（`mode/{MODE}.md` + i18n）生成。

### UC-0019 覆盖文件为空时不产生空节
- **类型**：单元测试
- **前置**：`IDENTITY.md` 存在但内容为空白。
- **步骤**：构建系统提示词。
- **预期**：Identity 节回退 i18n 默认（非空），不出现空节或被跳过的节。

---

## D `--init` 导出

### UC-0020 `--init` 一次导出全部目标文件
- **类型**：CLI（`co-shell --init`，工作区为空目录）
- **前置**：配置含内置三模式 + 1 个自定义 mode `audit`；工作区根无 `IDENTITY.md`/`CAPABILITIES.md`/`RULES.md`，无 `mode/` 目录。
- **步骤**：在干净目录执行 `co-shell --init`，列出生成的文件。
- **预期**：生成 `./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md`、`mode/ACT.md`、`mode/PLAN.md`、`mode/RESEARCH.md`、`mode/AUDIT.md`；命令输出告知写入的文件数量与路径；命令执行后退出（不进入 REPL/不启动 serve）。

### UC-0021 导出内容为“占位符原文”，可直接编辑
- **类型**：CLI
- **前置**：同上。
- **步骤**：查看导出的 `IDENTITY.md` 与 `mode/ACT.md` 内容。
- **预期**：含 `{AGENT_NAME}`、`{WORKSPACE}` 等占位符**原文**（未替换）；`mode/ACT.md` 内容 = `KeyWorkModeAct` 原文；文件为可读 utf-8 文本。

### UC-0022 已存在文件先备份 `.bak` 再覆盖
- **类型**：CLI
- **前置**：已存在 `./IDENTITY.md`（用户手工修改过）与 `mode/PLAN.md`。
- **步骤**：执行 `co-shell --init`。
- **预期**：生成 `IDENTITY.md.bak`、`mode/PLAN.md.bak`，其内容等于覆盖前的用户版本；目标文件内容被最新系统默认值覆盖；命令输出提示发生了备份。

### UC-0023 不创建子目录、不产生历史遗留路径
- **类型**：CLI
- **前置**：干净目录。
- **步骤**：执行 `co-shell --init` 后检查目录结构。
- **预期**：不存在 `mode/act/`、`mode/plan/` 等子目录，也不生成 `WORKMODE.md`、`RESULT_MODE.md`；仅产生 `mode/` 目录及其下的 `{MODE}.md` 文件。

### UC-0024 `--unload-mode` 已彻底移除
- **类型**：CLI（负例）
- **前置**：无。
- **步骤**：执行 `co-shell --unload-mode act`；再执行 `co-shell --help` 查看参数列表。
- **预期**：`--unload-mode` 不再被识别（报未知参数并给出非零退出码或明确提示，不得静默忽略）；`--help` 中不出现该参数，出现 `--init` 及其说明；代码中 `UnloadModeSections`、`KeyUnloadMode*` 等已删除。

### UC-0025 导出文件可驱动提示词覆盖（导出→编辑→生效闭环）
- **类型**：端到端
- **前置**：执行过 `--init`。
- **步骤**：编辑 `./RULES.md` 与 `mode/RESEARCH.md` 加入自定义标记文本，然后启动 serve 并分别构建 act / research 的提示词。
- **预期**：Rules 节含自定义标记；ResultMode 的 `# RESEARCH MODE` 段含自定义标记；且 act / research 的 ResultMode 文本仍一致（同一文件驱动）。

---

## E 实时性、i18n 与回归

### UC-0026 每次构建实时读盘（新回合生效）
- **类型**：端到端（serve）
- **前置**：serve 运行中，无相关文件。
- **步骤**：回合 1 抓取提示词（Identity 为默认）→ 写入 `./IDENTITY.md` → 回合 2 抓取提示词 → 删除文件 → 回合 3 抓取。
- **预期**：回合 2 立即使用文件内容（无需重启）；回合 3 回到默认值。

### UC-0027 中英双语下列表与标题一致、正文按语言切换
- **类型**：单元测试
- **前置**：分别 `i18n.Init("zh")` 与 `i18n.Init("en")`。
- **步骤**：构建系统提示词与 ResultMode 节。
- **预期**：节列表与顺序在两种语言下相同；`ACT MODE V.S. ...` 标题与 `# XX MODE` 段落标题一致（英文大写模式名）；引导句与 mode 正文按语言不同；两语言下 mode 描述均非空且不含未替换占位符。

### UC-0028 节分隔符与占位符替换在全景下仍正确
- **类型**：单元测试
- **前置**：无。
- **步骤**：构建完整提示词，检查节间分隔与占位符。
- **预期**：各节之间以 `KeySectionSeparator` 分隔，节首尾无多余空行（TrimSpace 语义保持）；`{OS}`、`{ARCH}`、`{SHELL}`、`{HOME}`、`{COMMAND}`、`{WORKSPACE}`、`{LANG}`、`{CONTEXT_REORGANIZE_THRESHOLD}` 等全部替换，无残留 `{...}`。

### UC-0029 无任何磁盘文件时的基线（回归 FEATURE-531 起因）
- **类型**：端到端
- **前置**：删除 `IDENTITY.md`/`CAPABILITIES.md`/`RULES.md`/`mode/*.md`。
- **步骤**：分别以 act / plan / research 构建提示词。
- **预期**：三者均含 8 节且顺序一致；均含 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE` 节；该节三段分别等于 i18n `KeyWorkModeAct/Plan/Research`。

### UC-0030 旧配置（含 sections 字段）加载不报错、不影响新行为
- **类型**：单元测试
- **前置**：`config.json` 保留 `work_modes` 及其 `sections` 字段（含与全局不一致的历史值）。
- **步骤**：加载配置并构建提示词。
- **预期**：加载与构建均无错误/警告失败；节列表仍为全局 8 节；其余 mode 参数（temperature、model_id 等）继续生效。

### UC-0031 编译与静态检查
- **类型**：命令
- **前置**：无。
- **步骤**：`go build ./... && go vet ./...`。
- **预期**：均无错误、无 vet 告警；受影响应用（co-shell 与 co-shell-hub）可正常编译出 work/ 产物。

### UC-0032 回归：既有测试全绿
- **类型**：命令
- **前置**：无。
- **步骤**：`go test ./...`（含 `agent`、`config`、`i18n` 包）。
- **预期**：全部通过；原 `TestDefaultSectionsStaticAcrossModes` / `TestBuildResultModeSection_*` 等用例已按新设计更新并保留等价断言（跨 mode 一致、ResultMode 必含）。

---

## 验证记录（执行时填写）

| 用例 | 类型 | 结果 | 证据/命令 | 备注 |
|---|---|---|---|---|
| UC-0001 ~ UC-0006 | 单测/审查 | 待执行 | | |
| UC-0007 ~ UC-0013 | 单测/E2E | 待执行 | | |
| UC-0014 ~ UC-0019 | E2E/负例 | 待执行 | | |
| UC-0020 ~ UC-0025 | CLI/E2E | 待执行 | | |
| UC-0026 ~ UC-0032 | E2E/回归 | 待执行 | | |

## 完成判定（全部满足方可交付）

1. 三个内置 mode 与自定义 mode 的节列表完全一致，且顺序为 Identity → ToolUsage → ResultMode → Capabilities → Rules → Skills → ExternalTools → Environment。
2. ResultMode 节在任意 mode 下内容完全一致，且由 mode/{MODE}.md（优先）或 i18n 默认值拼装；自定义 mode 亦参与。
3. `--init` 可一次导出 `./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md` 与 `mode/{MODE}.md`（含 `.bak` 备份），`--unload-mode` 已移除。
4. 新会话构建提示词时实时优先读取上述文件，缺失/空白回退系统默认；旧机制（mode/{mode}/*.md、WORKMODE.md、work_modes[].sections）不再生效。
5. `go build ./...`、`go vet ./...`、`go test ./...` 全部通过。
