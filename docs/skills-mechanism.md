# co-shell Skill 机制：管理模式与调用链路

> **文档性质**：现状梳理（as-is）+ 缺陷登记（D-001 ~ D-008）。仅描述**当前实现**，不含任何改动。
> **调查基准**：`main` 分支，co-shell `0.90.0`（`2026-10-02`），全部结论来自源码只读阅读与运行时证据（见 §9.2）。
> **相关历史任务**：FEATURE-453（v0.23.0 引入 skill 支持，`ROADMAP.md:3603-3618`）、FEATURE-505（SKILLS 节追加配置说明 + 改为始终输出，`ROADMAP.md:2573`）、FEATURE-501（启动自动创建 `skills/` 等内置目录，`main.go:498-505`）。

---

## 1. 概述

**skill = 一个目录 + 一个 `SKILL.md`**（Agent Skills 开放标准，`agent/skill.go:36-37`）。

co-shell 对 skill 的处理模式是 **「索引注入 + 模型按需读取」**：

1. **发现**：扫描工作区 `./skills/` 与全局 `~/.co-shell/skills/`，合并去重；
2. **注入**：系统提示词 `SKILLS` 节**只列索引**（名称 + 简介 + SKILL.md 绝对路径），**不注入正文**；
3. **使用**：模型判断需要某 skill 时，自行 `read_file` 该 SKILL.md 取正文，再用 `execute_command` 等工具执行其中的步骤/脚本；
4. **管理**：REPL 提供 `:skill list / show / add / remove` 四个子命令（**仅 REPL**）。

系统提示词 SKILLS 节原文（`i18n/zh_system.go:1434-1440`）已经把这套约定直接写给模型：

> 以下 skills 可用。每个 skill 是一个包含 SKILL.md 文件的目录。此处仅列出 skill 索引（名称、简介、路径）。当你需要使用某个 skill 时，用 read_file 读取其 SKILL.md 文件获取完整说明。

**关键特征（与相邻机制的区别）**：

- 没有 skill 专用工具（对比 meta-capability 有 `introspect_capability`）；
- 没有触发词解析、没有自动匹配、没有强制加载（是否使用完全取决于模型的自觉）；
- 没有搜索 / 下载 / 更新 / 版本 / 来源校验（对比 `notes/TODO.md:42` 仍列的待办）。

---

## 2. 数据模型

`agent/skill.go:38-49`：

| 字段 | 含义 | 来源 |
|---|---|---|
| `Name` | skill 名 | frontmatter `name:`，缺失时用**目录名** |
| `Description` | 简介 | frontmatter `description:`，缺失时用 SKILL.md 首个非空非标题行 |
| `Path` | SKILL.md 的（绝对）路径 | `<skills 目录>/<子目录名>/SKILL.md` |
| `Dir` | skill 目录路径 | `<skills 目录>/<子目录名>` |

---

## 3. 全链路（磁盘 → 模型）

```
<workspace>/skills/<dir>/SKILL.md      ← 工作区级（优先级高）
~/.co-shell/skills/<dir>/SKILL.md      ← 全局级（优先级低）
        │
        │ ① scanSkillsDir(dir)          agent/skill.go:55-86   仅一层子目录 + 精确文件名 SKILL.md
        ▼
      []Skill
        │
        │ ② scanSkills(ws, home)        agent/skill.go:91-117  工作区先扫、全局后扫；按 Name 去重（先到者胜）；按名排序
        ▼
      []Skill（合并去重后）
        │
        │ ③ buildSkillsIndex(skills)    agent/skill.go:170-185 生成 "- name: desc (path)" 行
        ▼
   索引文本
        │
        │ ④ 系统提示词 SKILLS 节        agent/system_prompt.go:664-684  节位置由 config/config.go:990-1001 固定
        ▼
   LLM 上下文（仅索引）
        │
        │ ⑤ 模型按需 read_file(<path>)  取 SKILL.md 正文
        ▼
   模型按正文执行（execute_command / 其它工具）
```

### 3.1 提示词重建时机（决定索引何时刷新）

SKILLS 节在**每次构建系统提示词时实时重扫磁盘**（`agent/system_prompt.go:667` 调用 `scanSkills(env.cwd, env.homeDir)`）。`rebuildSystemPrompt()` 的全仓调用点：

- `agent/run.go:83`、`agent/run_stream.go:298` —— 每次任务运行开始前；
- `agent/settings_tools.go:784/791/798` —— 设置变更后；
- `agent/agent.go:559/1176/1246/2158` —— 配置 / 工作区 / 模式等相关变更路径。

**推论**：在 `./skills/` 中新增或删除 skill，下一次任务运行即会反映到上下文，无需重启进程。

**例外**：会话恢复路径（`main.go:1470-1474`、`web/session.go:886-893`、`cmd/session.go:335/523`）会**沿用会话中已保存的 system 消息**，直到下一次 `rebuildSystemPrompt()` 才刷新——即"切到旧会话时，看到的可能仍是旧索引"。

### 3.2 节位置固定，无独立开关

`config/config.go:990-1001` 的 `DefaultSections()` 是全局唯一节列表来源（FEATURE-531）：

```
Identity → ToolUsage → ResultMode → Objective → Capabilities → Rules → Skills → ExternalTools → Environment
```

`agent/system_prompt.go:820-835` 只遍历这一个列表；**没有** `skills-enabled` 之类的配置项，Skills 节无法单独关闭（见 D-008）。

---

## 4. 发现与解析规则

### 4.1 目录扫描（`scanSkillsDir`，`agent/skill.go:55-86`）

- 只遍历传入目录的**一层**子项：`e.IsDir()` 的子目录才参与；
- 候选文件固定为 `<子目录>/SKILL.md`（**大小写敏感、文件名必须精确**）；
- `os.ReadFile` 失败（无 SKILL.md / 不可读）→ `continue`，**静默跳过，无任何日志或提示**；
- 目录不存在或读取失败 → 返回 `nil`。

**边界**：`skills/foo/bar/SKILL.md`（两层）不会被发现；`skill.md`、`SKILL.MD` 不识别；只有 README 的目录被忽略。

### 4.2 合并与优先级（`scanSkills`，`agent/skill.go:91-117`）

| 顺序 | 目录 | 说明 |
|---|---|---|
| 1 | `<workspacePath>/skills` | 工作区级，先入表 |
| 2 | `<homeDir>/.co-shell/skills` | 全局级，后入表 |

- 去重键是 **`Skill.Name`**（frontmatter 的 `name:`，不是目录名）：已存在同名即丢弃后到者 → **工作区级优先**；
- 最终按 `Name` 字典序排序（`sort.Strings`，`agent/skill.go:111`）；
- 导出包装 `ScanSkills`（`agent/skill.go:121-123`）供 `:skill` 命令复用同一套发现逻辑（保证命令与提示词看到的集合一致）。

### 4.3 frontmatter 解析（`parseSkillFrontmatter`，`agent/skill.go:129-152`）

- 仅当文件（去掉 BOM/空白后）以 `---` 开头才解析；取到下一个 `\n---` 之间的文本作为 frontmatter；
- **逐行**匹配，只识别两个字段：`name:` 与 `description:`，取**同一行** `:` 之后的内容并去掉首尾引号；
- 其它字段（如 README 中提到的"触发词"）**一概不解析**；
- 不支持 YAML 多行块标量（`>-` / `|`）——详见缺陷 **D-001**。

**兜底规则**：

| 情况 | 结果 | 代码 |
|---|---|---|
| 无 frontmatter / 无 `name:` | 用目录名 | `agent/skill.go:72-74` |
| 无 `description:` 或解析为空 | `firstNonEmptyLine()`：首个非空、非 `#` 开头、非 `---` 开头的行 | `agent/skill.go:75-77`、`156-165` |

### 4.4 索引格式（`buildSkillsIndex`，`agent/skill.go:170-185`）

每行一条，无 skill 时返回空串：

```
- {Name}: {Description} ({Path})
```

例如（运行时实际渲染，见 §9.2 证据）：`- image-generate: >- (/Users/.../skills/image-generate/SKILL.md)`。

### 4.5 注入位置与"始终输出"（`agent/system_prompt.go:664-684`）

- 有索引时：把索引插到 SKILLS 文本**末尾括号说明之前**（兼容中文 `\n\n（` 与英文 `\n\n(`），保证"（可以通过向 ./skills/ …）"说明留在最后；
- 无索引时：仍返回头部 + 说明（FEATURE-505，`agent/skills_note_test.go:86-106` 覆盖）；
- 中英文案分别在 `i18n/zh_system.go:1434-1440`、`i18n/en_system.go:1449-1454`，键为 `i18n/keys.go:2488`。

---

## 5. 管理命令（`:skill`）

实现：`cmd/skill.go`；入口：`repl/repl.go:483`（`case ":skill": r.skillHandler.Handle(args)`），处理器在 REPL 构造时注册（`repl/repl.go:134`）；文案 `i18n/zh.go:833-838`、`i18n/en.go:851-856`；`README.md:288` 亦登记该命令。

| 子命令 | 行为 | 代码位置 |
|---|---|---|
| `list` | `agent.ScanSkills(ws, home)` 合并列出（名称 / 简介 / 路径）；无 skill 时提示 | `cmd/skill.go:104-122` |
| `show <name>` | 按 `Name` 精确匹配后打印该 SKILL.md **原文**；未找到报错 | `cmd/skill.go:124-140` |
| `add <path> [--global]` | 递归复制源目录到 `./skills/<源目录名>`（带 `--global` 则到 `~/.co-shell/skills/<源目录名>`） | `cmd/skill.go:142-173`、`193-219` |
| `remove <name>` | 按 `Name` 匹配后 `os.RemoveAll(s.Dir)` | `cmd/skill.go:175-190` |

### 5.1 目录解析

`skillsDir(global)`（`cmd/skill.go:97-102`）：

- 工作区级：`filepath.Join(workspacePath(), "skills")`；`workspacePath()` 优先取 `agent.WorkspacePath()`，为空时回退 `os.Getwd()`（`cmd/skill.go:73-84`）；
- 全局级：`filepath.Join(os.UserHomeDir(), ".co-shell", "skills")`。

注意：目标目录**不是**由 `skillsDir` 创建，而是由 `copyDir` 内的 `os.MkdirAll` 顺带创建（`cmd/skill.go:198`）。

### 5.2 `add` 的复制语义（`copyDir` / `copyFile`，`cmd/skill.go:193-237`）

- 只校验"源路径存在且是目录"（`cmd/skill.go:161-164`），**不校验是否含 SKILL.md**、不校验来源；
- 目标目录名 = **源目录 basename**（`cmd/skill.go:167`），与 frontmatter 的 `name` 无关；
- 同名文件用 `os.Create` 直接覆盖（`cmd/skill.go:228`），**目标目录中已不存在于源的多余文件不会被删除**（增量覆盖，非镜像同步）；
- 无确认提示、无 `.gitignore`/忽略清单、无回滚。

### 5.3 `remove` 的删除语义

按 `ScanSkills` 返回的顺序匹配第一个同名项（工作区先）→ 删的是**胜出那份**。因此当全局存在同名 skill 且工作区也有一份时，命令**只能删掉工作区那份**；被遮蔽的全局 skill 无法通过该命令删除（见 D-003）。

---

## 6. 调用机制（模型如何真正用上 skill）

### 6.1 唯一通道：模型自行 `read_file`

1. 模型在系统提示词 SKILLS 节看到 `- {name}: {description} ({SKILL.md 路径})`；
2. 需要时调用 `read_file` 读取该路径的 SKILL.md 全文；
3. 按正文指引调用其它工具（`execute_command` 跑 `scripts/*`、`write_to_file` 等）完成任务。

**没有** skill 专用工具。对照 `DefaultToolModes()` 的全量工具清单（`agent/agent.go:297`）可见：不存在任何 `skill_*` / `use_skill` 类工具；SKILLS 节也只是"告知有这些技能 + 路径"，不含强制或自动动作。

### 6.2 无触发词、无自动匹配

- `parseSkillFrontmatter` 只取 `name` / `description`（`agent/skill.go:141-150`），**不解析触发词、关键词、允许工具等任何扩展字段**；
- 是否命中"该不该用这个 skill"，完全靠模型读 `description` 自行判断。因此现实中 `description` 往往被写成"当用户提出 XX / YY 请求时使用"（见 `skills/image-generate/SKILL.md:1-8`、`skills/agent-benchmark/SKILL.md:1-6`）；
- 结合缺陷 D-001（块标量解析失效），这类"触发说明"目前**根本进不了索引**，命中率只能依赖 skill 名称匹不匹配。

### 6.3 技能脚本与资源的执行

skill 目录内的 `scripts/`、`templates/` 等资源没有任何注册机制，由模型在 SKILL.md 指引下用通用工具访问（例如 `python <skill>/scripts/xxx.py`）。`<skill>` 需模型自行从索引中的 `SKILL.md` 路径推导（索引只给 SKILL.md 路径，不给目录字段）。

### 6.4 子 Agent 与其它渠道

- **子 Agent**：`subagent` 会以**独立 co-shell 进程**运行，工作目录为 `{parent-workspace}/sub-agents/{session}/{id}`（`subagent/launch.go:199-210`，命令行含 `-w pr.dir`，`cmd.Dir = pr.dir`）→ 子 Agent 扫描的是**它自己的工作区** `./skills/` 与全局 `~/.co-shell/skills/`，**看不到父工作区的 `./skills/`**（除非装到全局，或子 Agent 按 SKILL.md 里的绝对路径去读父工作区文件）。
- **渠道**：SKILLS 节属于系统提示词，Web UI / 飞书 / 移动端等渠道共享同一套提示词构建逻辑，因此"索引 + 按需读取"在全部渠道有效；但 `:skill` 管理命令**仅 REPL 可用**（见 D-005）。

---

## 7. 与相邻机制的对照

| 机制 | 注入提示词的内容 | 正文加载方式 | 专用查询工具 | 管理手段 | 代码位置 |
|---|---|---|---|---|---|
| `.rules/` **根目录** `.md` | **全文**（按文件名排序，带 `====` 标题） | 已注入 | — | 手工编辑文件 | `agent/system_prompt.go:69-108` |
| `.rules/` **子文件夹** | 仅子目录名 + 其 `.md` 完整路径（作为"可用规则类型"） | 模型按需 `read_file` | 无 | 手工 | `agent/system_prompt.go:109-`（FEATURE-417，见项目规范） |
| **`skills/`** | 仅 `name` + `description` + SKILL.md 路径 | 模型按需 `read_file` SKILL.md | **无** | `:skill list/show/add/remove`（仅 REPL） | `agent/skill.go`、`cmd/skill.go` |
| META-CAPABILITIES | 索引（受 `cfg.LLM.MetaCapabilityEnabled` 控制） | 工具按需取详情 | **有** `introspect_capability` | 代码内置 | `agent/system_prompt.go:611-632` |
| EXTERNAL TOOLS | 一段固定说明（`bin/` 下 Python 工具） | 模型自行 `list_files` / `read_file` | 无 | 手工 | `i18n/zh_system.go:1429-1431` |

**一句话对比**：`.rules/` 是"根文件全量注入、子目录按需"；`skills/` 是"全部只给索引、正文一律按需"；meta-capability 多了专用检索工具，skill 没有。

---

## 8. 当前缺陷清单（D-001 ~ D-008）

> 每条包含：现象 / 证据 / 根因位置 / 影响 / 建议方向。均为只读调查所得，未做改动。

### D-001 `description` 多行块标量解析失效（简介丢失）——**信息量最严重**

- **现象**：SKILL.md 中使用 YAML 块标量写简介时（`description: >-` 或 `description: |`），索引里的简介被写成字面量 `>-` / `|`，多行简介内容全部丢失。
- **证据（运行时可核验）**：
  - 会话系统提示词 SKILLS 节实际渲染为：
    ```
    - agent-benchmark: >- (/Users/direct3d/github/co-shell/skills/agent-benchmark/SKILL.md)
    - image-generate: >- (/Users/direct3d/github/co-shell/skills/image-generate/SKILL.md)
    ```
  - 日志：`log/co-shell-2026-09-25.log:3948` → `- image-generate: >- (/Users/direct3d/.co-shell/skills/image-generate/SKILL.md)`
  - 对照源文件：`skills/image-generate/SKILL.md:2-8`、`skills/agent-benchmark/SKILL.md:2-6` 均使用 `description: >-` 多行写法。
- **根因**：`parseSkillFrontmatter`（`agent/skill.go:141-150`）逐行取 `description:` 之后的**同一行**文本，未识别块标量语义，也未做后续行拼接。
- **影响**：索引退化为"名称 + 路径"，模型失去"什么时候该用这个 skill"的关键线索；技能命中率完全依赖名字。
- **建议方向**：支持 `>-`（折叠）/ `|`（字面）块标量：取后续缩进行拼接；并考虑"多行简介只取首句"以控制索引长度。

### D-002 无 skill 专用工具、无自动匹配

- **现象**：只能用 `read_file` 读 SKILL.md；没有 `use_skill` 之类工具，也没有基于名称/触发词的自动匹配与强制加载。
- **证据**：`DefaultToolModes()` 工具清单（`agent/agent.go:297` 起）无任何 skill 相关工具；`agent/skill.go` 无触发词解析；SKILLS 节文案（`i18n/zh_system.go:1437`）只写"用 read_file 读取"。
- **影响**：模型忽略 SKILLS 节时不触发；无法约束/降低 token 消耗；技能使用完全不可观测（无使用日志）。
- **建议方向**：新增 `use_skill <name>`（或 `introspect_skill`）工具，返回 SKILL.md 正文 + 目录内资源清单；可顺带记录使用轨迹。

### D-003 同名冲突静默 + `remove` 只能删"胜出份"

- **现象**：工作区与全局同名（按 frontmatter `name` 判定）时，全局那份被静默丢弃、无任何提示；`:skill remove <name>` 删的是胜出（工作区）那份，导致被遮蔽的全局 skill **无法通过命令删除**。
- **证据**：去重逻辑 `agent/skill.go:95-102`（`if _, exists := byName[s.Name]; !exists`）；删除逻辑 `cmd/skill.go:180-189` 遍历 `ScanSkills` 结果取第一个匹配。
- **建议方向**：`list` 标注来源（workspace / global）与被遮蔽项；给 `remove` 增加 `--global`（或 `--source`）以定向删除。

### D-004 管理命令前缀文案与实际不符（`.skill` vs `:skill`）

- **现象**：帮助文案写的是 `.skill`（`i18n/zh.go:833`：`Skill 管理 (.skill)`；`i18n/en.go:851` 同），而 REPL 只识别 `:` 前缀。
- **证据**：`.` 开头的输入在 `repl/repl.go:362-391` 会被当作"当前目录下是否是可执行文件"来探测，不是可执行则提示"内置命令应使用 `:` 前缀"；`:skill` 在 `repl/repl.go:483` 分派。
- **影响**：按帮助文案输入 `.skill list` 不会执行管理命令，用户得到的是"前缀用错"的提示与一次是否发给 LLM 的询问。
- **建议方向**：统一文案为 `:skill`（并顺带排查其它 `.xxx` 文案）；或在 REPL 增加 `.` → `:` 的兼容映射。

### D-005 `:skill` 命令仅在 REPL 可用

- **现象**：Web UI / 飞书 / 移动端等渠道没有 skill 管理入口。
- **证据**：全仓检索 `SkillHandler` 只命中 `cmd/skill.go` 与 `repl/repl.go`（构造 + 分派）；`web/`、`feishu/`、`mobile/`、`bridge/` 无引用。
- **影响**：非 REPL 用户只能靠手工把目录放进 `./skills/` 或 `~/.co-shell/skills/`。
- **建议方向**：明确取舍——要么在 Web UI 提供只读的 skill 列表/查看，要么在文档中说明该命令的渠道边界。

### D-006 子 Agent 看不到父工作区技能

- **现象**：子 Agent 是独立进程 + 独立工作区，只扫自己的 `./skills/` 与全局目录，父工作区 `./skills/` 中的技能对其不可见。
- **证据**：`subagent/launch.go:199-210`（`args` 含 `-w pr.dir`、`cmd.Dir = pr.dir`）；子 Agent 工作区为 `{parent-workspace}/sub-agents/{session}/{id}`（`subagent/subagent.go:30-31`、`331-332`）。
- **影响**：需要专业技能的委派任务，子 Agent 拿不到技能；需重复安装到全局，或由父 Agent 在指令中显式贴出 SKILL.md 路径/正文。
- **建议方向**：在子 Agent 启动参数或指令中注入父工作区技能索引（或提供"技能继承"开关）。

### D-007 无搜索 / 下载 / 更新 / 来源校验；`add` 为增量覆盖

- **现象**：仅支持"从本地路径复制"，没有远程检索、安装、升级、卸载清单，也不做来源与内容校验。
- **证据**：`cmd/skill.go:142-173` 只做 `os.Stat` + `copyDir`；`copyDir`/`copyFile`（`193-237`）用 `os.Create` 覆盖同名文件、**不清理目标中多余的旧文件**；`notes/TODO.md:42` 仍列"增加对 skill 的支持（包括：搜索、下载、加入、移除等）"为待办；FEATURE-453 原始范围仅"扫描 + 索引 + 命令"（`ROADMAP.md:3608`）。
- **影响**：技能升级后可能残留陈旧脚本；无法评估技能来源可信度。
- **建议方向**：`add` 增加"覆盖前确认 / 先清空目标目录 / `--force`"语义；后续再考虑远程索引与校验和。

### D-008 无 Skill 开关；Skills 节位置固定

- **现象**：不能按需关闭 Skills 节（例如给不需要技能的轻量 Agent 省 token），也没有"仅启用指定 skills"的配置项。
- **证据**：节列表来自唯一的 `config.DefaultSections()`（`config/config.go:990-1001`），`agent/system_prompt.go:820-835` 只遍历该列表，无 skills 相关配置分支。
- **附带发现（与本缺陷相关但独立）**：`cfg.PromptSections`（`:section` 命令维护的自定义节）在非测试 Go 代码中**仅被 `cmd/section.go` 引用**，未参与提示词拼装（全仓 grep 结果）——即自定义节当前"只存在于配置与管理界面"。
- **建议方向**：如需开关，新增配置项（如 `skills-enabled` / `skills-mode`）并在 Skills 分支短路返回空节；自定义节拼装问题另立任务评估。

---

## 9. 附录

### 9.1 关键代码索引

| 位置 | 职责 |
|---|---|
| `agent/skill.go:38-49` | `Skill` 数据结构 |
| `agent/skill.go:55-86` | `scanSkillsDir`：单目录一层扫描 |
| `agent/skill.go:91-117` | `scanSkills`：工作区 + 全局合并、按名去重、排序 |
| `agent/skill.go:121-123` | `ScanSkills`：供 `:skill` 命令复用的导出包装 |
| `agent/skill.go:129-152` | `parseSkillFrontmatter`：仅 `name` / `description` 单行解析（D-001 根因） |
| `agent/skill.go:156-165` | `firstNonEmptyLine`：简介兜底 |
| `agent/skill.go:170-185` | `buildSkillsIndex`：索引行生成 |
| `agent/system_prompt.go:664-684` | SKILLS 节渲染（实时扫描 + 索引插入位置 + 始终输出） |
| `agent/system_prompt.go:820-835` | 全局节列表拼装（Skills 位置由此固定） |
| `config/config.go:990-1001` | `DefaultSections()`：节列表唯一真相源 |
| `cmd/skill.go:40-102` | `SkillHandler`、子命令分派、目录解析 |
| `cmd/skill.go:104-190` | `list` / `show` / `add` / `remove` 实现 |
| `cmd/skill.go:193-237` | `copyDir` / `copyFile`：递归复制（增量覆盖语义） |
| `repl/repl.go:134`、`483` | 处理器注册与 `:skill` 分派（仅 REPL） |
| `repl/repl.go:362-396` | `.` / `:` 前缀判定（D-004 相关） |
| `i18n/zh_system.go:1434-1440`、`i18n/en_system.go:1449-1454` | SKILLS 节中英文案（含末尾配置说明） |
| `i18n/keys.go:2488`、`1076-1083` | `KeySystemPromptSkills` 与 `:skill` 命令文案键 |
| `i18n/zh.go:833-838`、`i18n/en.go:851-856` | `:skill` 命令文案（D-004） |
| `subagent/launch.go:199-210` | 子 Agent 启动参数（独立工作区，D-006） |
| `main.go:498-505` | 启动时自动创建 `skills/` 等内置目录（FEATURE-501） |

### 9.2 证据来源

1. **会话运行时渲染**（2026-10-02）：系统提示词 SKILLS 节实际内容包含
   `- agent-benchmark: >- (/Users/direct3d/github/co-shell/skills/agent-benchmark/SKILL.md)`、
   `- image-generate: >- (/Users/direct3d/github/co-shell/skills/image-generate/SKILL.md)`。
2. **日志**：`log/co-shell-2026-09-25.log:3948` 记录同一渲染结果（全局技能路径形态）。
3. **源码**：本文所有 `文件:行号` 引用均来自 `main` 分支 `0.90.0` 当前工作树（只读阅读）。
4. **规范与历史**：`notes/TODO.md:42`（待办）、`ROADMAP.md:2573`（FEATURE-505）、`ROADMAP.md:3603-3618`（FEATURE-453）、`README.md:288`（命令登记）。

### 9.3 变更记录

| 日期 | 内容 |
|---|---|
| 2026-10-02 | 初版：现状梳理 + D-001 ~ D-008 缺陷登记（只读调查，无代码改动） |
