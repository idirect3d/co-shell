# FEATURE-531 增量用例 · UC-0002：个性化内容只落 md 文件（不再写 config.json）

> 分支：FEATURE-531 ｜ 版本：v0.64.0 ｜ 关联：[BUILD-1064] 主体 + 本次增量（BUILD-1065）
> 目标：身份 / 原则 / 规则 / 能力 / mode 策略等「agent 个性化」内容只存放在工作区 md 文件中，
> 不再写入 `~/.co-shell/config.json`，使多个 agent 共享同一 config.json 时不会互相同步个性化。

## 一、设计约定（实现依据）

### 1. 文件与唯一性

| 文件 | 承载内容 | 对应 config.json 字段（本次废弃） |
|---|---|---|
| `./IDENTITY.md` | agent-name、agent-description、user-name、user-description（结构化）+ 自由文本 | `LLM.AgentName`、`LLM.AgentDescription`、`LLM.ModeDescriptions` |
| `./PRINCIPLES.md` | 原则全文 | `LLM.AgentPrinciples` |
| `./RULES.md` | 规则全文 | `Rules` |
| `./CAPABILITIES.md` | 能力全文 | （FEATURE-531 前已文件化） |
| `./mode/{MODE}.md` | 该 mode 的策略（ResultMode 段落） | （FEATURE-531 前已文件化） |

- 上述 config 字段：**保存时不再写入、读取时完全忽略**（旧值失效）。
- `:set defaults` 不再重置这些个性化字段，也不删除上述 md 文件。

### 2. IDENTITY.md 结构化格式（markdown 分节键值）

```markdown
## agent-name
work

## agent-description
你是一个严谨、务实、目标驱动的通用智能体。

## user-name
匿名

## user-description
（可选，无内容时省略）

（此处任意自由文本会原样追加到身份节末尾）
```

解析规则：`## <key>` 开始一个字段段，段内容为到下一个 `## ` 标题为止的文本；键不区分大小写；
未识别的键与首个已识别键之前的文本一并作为「自由文本」保留。

### 3. 渲染与回退

- 身份节 = i18n 模板 `KeySystemPromptIdentity`，占位符替换：`{AGENT_NAME}`（文件 → 默认 `co-shell`）、
  `{AGENT_DESCRIPTION}`（文件 → i18n `KeyAgentDefaultDescription`）、`{AGENT_PRINCIPLES}`（PRINCIPLES.md → i18n
  `KeyAgentDefaultPrinciples`）、`{USER_NAME}`（文件 → 系统用户名 → i18n `KeyAnonymousUser`）、
  `{USER_DESCRIPTION}`（文件，无内容则替换为空）；末尾追加 IDENTITY.md 的自由文本。
- 规则节 = RULES.md → i18n 模板（其 `{CUSTOM_RULES}` 由 `.rules/` 目录树填充）。
- 三 mode（act/plan/research）除当前模式标识外仍完全一致（FEATURE-531 验收 1 不回归）。

## 二、用例清单

### A 组：IDENTITY.md 读写

| 编号 | 前置 | 步骤 | 期望结果 |
|---|---|---|---|
| UC2-A1 | 工作区无 IDENTITY.md | 构建系统提示词 | 名字为 `co-shell`，描述/原则取 i18n 默认 |
| UC2-A2 | IDENTITY.md 含 agent-name=`work`、agent-description=`自定义描述` | 构建系统提示词 | 身份节使用文件值，非 i18n 默认 |
| UC2-A3 | 同上 | 经 `SaveIdentity("name","new")` 保存名字 | IDENTITY.md 的 `## agent-name` 段变为 `new`；config.json 内容不变 |
| UC2-A4 | 同上 | 保存描述（`description`） | IDENTITY.md 的 `## agent-description` 段更新；config.json 不变 |
| UC2-A5 | IDENTITY.md 含自由文本段落 | 保存 agent-name | 自由文本与其它字段原样保留（不被覆盖丢失） |
| UC2-A6 | IDENTITY.md 含 user-name=`张三` | 构建提示词 | `{USER_NAME}` 解析为 `张三` |
| UC2-A7 | IDENTITY.md 只有 agent-name | 构建提示词 | 描述回退 i18n 默认（不报错、不为空） |
| UC2-A8 | 无 IDENTITY.md | `:config agent-description xxx` | 自动创建 IDENTITY.md 并写入 `## agent-description`，config.json 不新增键 |

### B 组：PRINCIPLES.md / RULES.md

| 编号 | 前置 | 步骤 | 期望结果 |
|---|---|---|---|
| UC2-B1 | 无 PRINCIPLES.md | 保存 principles | 生成 `./PRINCIPLES.md` 且内容为所存文本；config.json 无 `agent_principles` |
| UC2-B2 | 有 PRINCIPLES.md | 构建提示词 | 身份节的 `{AGENT_PRINCIPLES}` 使用文件内容 |
| UC2-B3 | 无 PRINCIPLES.md | 构建提示词 | 回退 i18n `KeyAgentDefaultPrinciples` |
| UC2-B4 | 无 RULES.md | 保存 rules | 生成 `./RULES.md`；config.json 无 `rules` 键 |
| UC2-B5 | 有 RULES.md | 构建提示词 | 规则节使用文件内容，忽略 config 旧值 |
| UC2-B6 | 无 RULES.md、`.rules/` 下有 md | 构建提示词 | 规则节 = i18n 模板 + `.rules/` 目录树 |

### C 组：config.json 旧值失效与 `:set defaults`

| 编号 | 前置 | 步骤 | 期望结果 |
|---|---|---|---|
| UC2-C1 | 手工在 config.json 写入 `agent_name/agent_principles/rules/mode_descriptions` | 构建提示词 | 全部不生效（与未写入时构建结果一致） |
| UC2-C2 | 任意 | 保存配置（任意字段） | 输出的 config.json 不含 `agent_name`、`agent_description`、`mode_descriptions`、`agent_principles`、`rules` 键 |
| UC2-C3 | 存在 IDENTITY.md / PRINCIPLES.md / RULES.md / mode/*.md | 执行 `:set defaults` | 8 节结构与三 mode 一致性保持；上述文件内容不被删除，提示词仍使用文件内容 |

### D 组：--init 导出

| 编号 | 前置 | 步骤 | 期望结果 |
|---|---|---|---|
| UC2-D1 | 空工作区 | 执行 `--init` | 生成结构化 `./IDENTITY.md`（含 agent-name/agent-description/user-name）、`./PRINCIPLES.md`、`./CAPABILITIES.md`、`./RULES.md`、`./mode/{ACT,PLAN,RESEARCH}.md`；不创建子目录 |
| UC2-D2 | 上述文件已存在 | 再次执行 `--init` | 旧文件先备份为 `{name}.bak` 再覆盖 |
| UC2-D3 | 空工作区 | 执行 `--init` 后立即构建提示词 | 文件存在时构建结果与 i18n 默认一致（导出即当前生效内容） |

### E 组：入口一致性与显示名

| 编号 | 前置 | 步骤 | 期望结果 |
|---|---|---|---|
| UC2-E1 | Web UI 身份表单 | 保存 name / description / principles / rules | 分别写入对应 md 文件，config.json 不变 |
| UC2-E2 | Web UI agent 表单 | 保存 name / description / principles | 同上 |
| UC2-E3 | LLM 设置工具 | 设置 agent-name / agent-description / agent-principles | 同上（写 md 文件） |
| UC2-E4 | IDENTITY.md 的 agent-name=`work` | 查看 `:session` 显示名 / 设置页显示名 | 显示 `work`，与提示词一致 |
| UC2-E5 | 保存后新开会话 | 构建提示词 | 立即反映最新文件内容（实时读盘） |

### F 组：回归

| 编号 | 步骤 | 期望结果 |
|---|---|---|
| UC2-F1 | 三 mode 构建提示词并对比 | 除当前模式标识外完全一致 |
| UC2-F2 | `go build ./... && go vet ./...` | 无输出、退出码 0 |
| UC2-F3 | `go test ./...` | 除 cmd 包既有失败 `TestWebWizardModelNameStep`（本改动无关）外全绿 |

## 三、验收判定

- A–F 组全部用例通过（可用单元测试 + CLI/Web 运行时验证）。
- `use-case/FEATURE-531/FEATURE-531-UC-0001.md`（主体）用例不回归。
