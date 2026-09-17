# FEATURE-533 用例：可编辑文件说明按节归位（分支 FEATURE-531 / v0.64.0）

> 背景：系统提示词 MODE 节（标题 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE`）末尾的注记
> （`i18n.KeySystemPromptResultModeNote`，`agent/system_prompt.go:451` 追加）同时罗列了
> `./mode/{MODE}.md`、`./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md` 四个文件，
> 而另外三个文件各自有对应的节（Identity / CAPABILITIES / RULES），说明位置与内容错位。

## 需求（用户确认）

1. 把 `./mode/{MODE}.md` 的说明留在 MODE 节末尾，**删除**该句中的 `./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md`。
2. `./IDENTITY.md` 的说明移到 Identity 节（`KeySystemPromptIdentity`）末尾。
3. `./CAPABILITIES.md` 的说明放到 CAPABILITIES 正文末尾，即 `4. 目标驱动` 之后、META-CAPABILITIES 清单之前（只改 i18n 文本，**零代码改动**）。
4. `./RULES.md` 的说明合并进 RULES 节已有的 `.rules/` 注记中（不新增第二条注记）。
5. 中英双语一致。

## 实现（预期改动面）

- `i18n/zh_system.go` / `i18n/en_system.go`：改动上述 4 处文本（Identity / Capabilities / Rules / ResultModeNote）。
- `i18n/section_notes_test.go`（新增）：断言 4 处注记的落位、语言、合并。
- `ROADMAP.md`：FEATURE-533 任务登记。

**不改动**：`agent/system_prompt.go`（注记挂载点不变）、`.rules/` 与 META-CAPABILITIES 的生成逻辑、Skills 节注记。

---

## UC-0001（单元）四处注记各自落在对应节

**操作**：分别对 zh/en，读取 `i18n.T(KeySystemPromptIdentity)`、`KeySystemPromptCapabilities`、`KeySystemPromptRules`、`KeySystemPromptResultModeNote`。

**预期**：
- Identity 节文本末尾含 `./IDENTITY.md` 且含 `--init`，位于 `{AGENT_PRINCIPLES}` 之后；
- CAPABILITIES 节文本末尾含 `./CAPABILITIES.md` 且含 `--init`，位于 `4. **目标驱动**` / `4. **Goal-Driven**` 之后；
- RULES 节文本含 `./RULES.md` 与 `--init`，且位于最后一条规则（`管理上下文窗口` / `Managing the context window`）之后、`{CUSTOM_RULES}` 之前；
- ResultModeNote **不含** `./IDENTITY.md`、`./CAPABILITIES.md`、`./RULES.md`，仅含 `./mode/{MODE}.md` 与 `--init`。

**验证方式**：`go test ./i18n/ -run 'TestPerSectionFileNotes|TestResultModeNoteOnlyModeFile'`（表驱动，中英各一轮）。

---

## UC-0002（单元）RULES 注记是合并而非新增

**操作**：读取 zh/en 的 RULES 节文本。

**预期**：
- 原有 `.rules/` 说明的原文片段保持不变：
  - zh：`（可以通过向 .rules/ 下放规则文件的方式`、`文件名将被当作各节标题`、`子文件夹将被列出（作为索引），但不会再遍历子文件夹`
  - en：`(You can customize rules/specifications by placing rule files under .rules/`、`file names are used as section titles`、`subfolders are listed as an index but are not traversed further`
- 同一 `strings.Count` 下该注记只出现 **1 次**（无重复段落）；
- 该注记内同时含 `./RULES.md` 与 `--init`。

**验证方式**：`go test ./i18n/ -run TestRulesNoteMergesRulesFile` + 既有 `TestRulesDirCustomizationNote` 仍通过。

---

## UC-0003（回归）MODE 节既有注记测试与导出行为

**操作**：
1. `go test ./agent/ -run TestBuildResultModeSection_TrailingNote`；
2. 阅读 `--init` 导出内容（`ExportSections` 写 `CAPABILITIES.md` / `RULES.md` 取自 i18n 文本）。

**预期**：
- MODE 节注记仍位于所有模式描述之后、括号包裹、含 `--init`（既有断言不破坏）；
- `--init` 导出的 `CAPABILITIES.md` / `RULES.md` 文件内容自动带上对应注记（因为取自同一 i18n 文本），无需额外代码；
- `go build ./... && go vet ./...` 全绿。

**验证方式**：`go build ./... && go vet ./... && go test ./i18n/ ./agent/ ./... `（全量）。
