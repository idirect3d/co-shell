# FEATURE-504 RULES 节末尾追加 .rules/ 定制说明

## 背景

系统提示词的 RULES 节列出了核心规则，但 LLM 自身不知道用户可以通过向 `.rules/` 下放规则文件的方式定制规则/规范，也不知道文件名会被当作各节标题、子文件夹会被列出作为索引，因此无法主动告知用户如何定制规则。

## 方案（用户确认）

在 RULES 节（i18n 的 `KeySystemPromptRules` 文本）末尾、`{CUSTOM_RULES}` 占位符之前，追加一句括号说明：

> （可以通过向 .rules/ 下放规则文件的方式，在以下位置定制规则/规范，文件名将被当作各节标题，子文件夹将被列出（作为索引），但不会再遍历子文件夹，需要时可自取）

英文对应：

> (You can customize rules/specifications by placing rule files under .rules/; the file names are used as section titles, and subfolders are listed as an index but are not traversed further — read them on demand.)

## 验收标准

1. `i18n/zh_system.go` 的 `KeySystemPromptRules` 文本末尾包含该中文括号说明
2. `i18n/en_system.go` 的 `KeySystemPromptRules` 文本末尾包含对应英文说明
3. 说明位于 `{CUSTOM_RULES}` 占位符之前
4. `go build ./... && go vet ./...` 通过
5. 相关单元测试通过

---

## 测试用例

### UC-0001 中文 RULES 文本包含 .rules/ 定制说明

- 前置：`i18n.Init("zh")`
- 步骤：读取 `i18n.T(i18n.KeySystemPromptRules)`
- 期望：
  - 包含「可以通过向 .rules/ 下放规则文件的方式」
  - 包含「文件名将被当作各节标题」
  - 包含「子文件夹将被列出（作为索引），但不会再遍历子文件夹」
  - 说明以「（」开头、以「）」结尾（括号包裹）

### UC-0002 英文 RULES 文本包含 .rules/ 定制说明

- 前置：`i18n.Init("en")`
- 步骤：读取 `i18n.T(i18n.KeySystemPromptRules)`
- 期望：
  - 包含 "placing rule files under .rules/"
  - 包含 "file names are used as section titles"
  - 包含 "subfolders are listed as an index but are not traversed further"
  - 说明以 "(" 开头、以 ")" 结尾（括号包裹）

### UC-0003 说明位于 {CUSTOM_RULES} 占位符之前

- 前置：`i18n.Init("zh")` / `i18n.Init("en")`
- 步骤：读取 RULES 文本，定位说明与 `{CUSTOM_RULES}` 的下标
- 期望：说明的下标 < `{CUSTOM_RULES}` 的下标（说明在自定义规则内容之前）

### UC-0004 说明位于 RULES 节末尾（最后一条规则之后）

- 前置：`i18n.Init("zh")` / `i18n.Init("en")`
- 步骤：读取 RULES 文本，定位「管理上下文窗口」规则与说明的下标
- 期望：说明的下标 > 「管理上下文窗口」规则的下标（说明在最后一条规则之后）

### UC-0005 中英文说明均存在且非空

- 前置：分别 `i18n.Init("zh")` / `i18n.Init("en")`
- 步骤：读取两种语言的 RULES 文本
- 期望：两种语言均包含 `.rules/` 字样，且说明段落非空

### UC-0006 编译与静态检查通过

- 步骤：`go build ./... && go vet ./...`
- 期望：无错误输出

### UC-0007 单元测试通过

- 步骤：`go test ./i18n/ -run TestRulesDirCustomizationNote -v`
- 期望：全部子用例 PASS

### UC-0008 系统提示词端到端渲染包含说明

- 前置：构建完整系统提示词（zh/en）
- 步骤：调用系统提示词构建流程
- 期望：渲染结果中 RULES 节包含该说明，且位于 `{CUSTOM_RULES}` 替换内容之前
