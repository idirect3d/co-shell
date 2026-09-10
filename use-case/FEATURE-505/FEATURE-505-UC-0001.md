# FEATURE-505 SKILLS 节末尾追加 skill 配置机制说明

## 背景

系统提示词的 SKILLS 节列出了可用 skill 索引，但 LLM 自身不知道用户可以通过向 `./skills/`（工作空间级）或 `~/.co-shell/skills/`（全局级）下放 skill 目录的方式定制 skill，也不知道同名时工作空间级优先、可用 `:skill` 命令管理，因此无法主动告知用户如何定制 skill。此外 SKILLS 段在无 skill 时整段不输出，导致说明也随之消失。

## 方案（用户确认）

1. 在 SKILLS 段最末尾（动态索引列表之后）追加一句括号说明：
   > （可以通过向 ./skills/（工作空间级）或 ~/.co-shell/skills/（全局级）下放 skill 目录的方式定制 skill，每个 skill 是一个包含 SKILL.md 的目录，同名时工作空间级优先，可用 :skill list/show/add/remove 命令管理）
2. SKILLS 段改为**始终输出**（即使无 skill 也输出头部 + 说明）。

## 验收标准

1. `i18n/zh_system.go` / `i18n/en_system.go` 的 `KeySystemPromptSkills` 包含该括号说明
2. `agent/system_prompt.go` 的 Skills 分支：无 skill 时也输出（头部 + 说明）；有 skill 时说明位于索引列表之后
3. `go build ./... && go vet ./...` 通过
4. 相关单元测试通过（含无 skill 场景）

---

## 测试用例

### UC-0001 中文 SKILLS 文本包含 skill 配置说明

- 前置：`i18n.Init("zh")`
- 步骤：读取 `i18n.T(i18n.KeySystemPromptSkills)`
- 期望：包含「可以通过向 ./skills/」「~/.co-shell/skills/」「同名时工作空间级优先」「:skill list/show/add/remove」

### UC-0002 英文 SKILLS 文本包含 skill 配置说明

- 前置：`i18n.Init("en")`
- 步骤：读取 `i18n.T(i18n.KeySystemPromptSkills)`
- 期望：包含 "placing skill directories under ./skills/"、"~/.co-shell/skills/"、"workspace-level takes precedence"、":skill list/show/add/remove"

### UC-0003 说明以括号包裹且位于文本末尾

- 前置：`i18n.Init("zh")` / `i18n.Init("en")`
- 步骤：读取 RULES 文本，TrimSpace 后检查结尾
- 期望：中文以「）」结尾、英文以 ")" 结尾；说明为最后一个非空段落

### UC-0004 说明提及 skills 目录

- 步骤：定位说明起始标记（中文「（可以通过向」/ 英文 "(You can customize skills"）
- 期望：说明段落中包含 "skills/"

### UC-0005 无 skill 时 SKILLS 段仍输出

- 前置：空工作空间 + 空 home 目录（无任何 skill）
- 步骤：`buildSkillsIndex(scanSkills(empty, empty))` 返回空；按 Skills 分支逻辑取 header
- 期望：header 非空，且包含 "skills/"（说明仍在）

### UC-0006 有 skill 时索引位于说明之前

- 前置：`i18n.Init("zh")`，构造一个 skill 索引条目
- 步骤：按 Skills 分支逻辑将索引插入 header
- 期望：索引位置 < 说明位置；说明仍为最后一个段落（以「）」结尾）

### UC-0007 编译与静态检查通过

- 步骤：`go build ./... && go vet ./...`
- 期望：无错误输出

### UC-0008 单元测试通过

- 步骤：`go test ./agent/ -run 'TestSkillsSection|TestSkillsIndex' -v`
- 期望：全部子用例 PASS

### UC-0009 端到端渲染验证（中英 × 有无 skill 四种组合）

- 步骤：分别以 zh/en 渲染 SKILLS 段，各覆盖无 skill 与有 skill 两种场景
- 期望：
  - 无 skill：输出「SKILLS + 说明」
  - 有 skill：输出「SKILLS + 索引列表 + 说明」
  - 说明始终位于最末尾
