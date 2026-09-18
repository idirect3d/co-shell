# FEATURE-535 用例：系统提示词身份注入与 RULES 语义显式化（分支 FEATURE-535 / v0.65.0）

> 需求（用户原话）：
> 1、现在新增加的用户名和用户描述，是不是还没有注入系统提示词？2、如果想加入系统提示词，给个建议放在哪部份？
> 3、RULES 配置，我想完全暴露系统默认值，用户可完全修改，而不是像现在这样追加的方式（但实际似乎是替换系统默认值），这个方案怎么样？
>
> 决策（用户拍板）：
> ① 身份注入 Environment 节 `<system_info>`（结构化字段 `<user>` / `<user_description>`），用户名从 channel 拆出、`<channel>` 只保留频道名；
> ② user_description 为空时不输出该标签行；
> ③ 做 `promptIdentity` 结构体重构（根治「加字段忘传参」）；
> ④ RULES 采用显式两层（`./RULES.md` 整节替换 + `.rules/` 在 `{CUSTOM_RULES}` 处追加），语义写清、删占位符时告警；
> ⑤ 新增 `{DEFAULT_RULES}`，展开边界为整个默认段（含说明与 `{CUSTOM_RULES}`）；
> ⑥ Settings「规则」框无文件时显示默认文本（口径 = `{DEFAULT_RULES}` 已展开、`{CUSTOM_RULES}` 保留字面量 = `--init` 导出原文）；
> ⑦ `--init` 维持「备份 .bak 后覆盖」；
> ⑧ 视觉识别轮天然不含 Environment 节，不注入 user 信息。

## 实现（改动面）

- `i18n/zh_system.go` / `i18n/en_system.go`：Environment 模板 `<channel>` 收窄为仅频道名，新增 `<user>{USER_NAME}</user>` 与 `<user_description>{USER_DESCRIPTION}</user_description>`；RULES 节说明文案补充 `{DEFAULT_RULES}` 语义。
- `agent/system_prompt.go`：新增 `promptIdentity` 结构体并收敛 `buildSystemPromptWithMode` 参数（12 → 6 左右）；贯通 `env.userDescription`；`env.channelInfo` 语义收窄；Rules 分支展开 `{DEFAULT_RULES}` 并在缺 `{CUSTOM_RULES}` 致 `.rules/` 未注入时告警；空 `user_description` 不输出标签行。
- `agent/agent.go`：两处提示词构建点传入 `ResolveUserDescription(dir)`。
- `cmd/settings_identity.go`：`IdentityJSON()` 的 rules 字段在无 `./RULES.md` 时回退默认文本。
- 用例：本文件；单测：`agent/system_prompt_feature535_test.go`、`cmd/settings_identity_test.go`。

---

## UC-0001（单元）Environment 节注入 `<user>` 与 `<user_description>`，`<channel>` 仅频道名

**前置**：临时 workspace 内 `IDENTITY.md` 含 `## user-name = 梁爽`、`## user-description = 后端工程师`；配置 `channel = web`。

**操作**：构建系统提示词（`buildVisionIdentityPrompt` 与 `buildSystemPromptWithMode` 均可）。

**预期**：
- 输出含 `<user>梁爽</user>` 与 `<user_description>后端工程师</user_description>`；
- 输出含 `<channel>web</channel>`，且**不含** `梁爽 @ web` 这类拼接形式；
- 注入位置在 Environment 节（`SYSTEM INFORMATION` / `<system_info>` 块内）。

**验证方式**：`go test ./agent/ -run TestFeature535IdentityInEnvironment`。

---

## UC-0002（单元）空用户描述不输出标签行

**前置**：`IDENTITY.md` 只有 `## user-name = 梁爽`（`user-description` 为空/缺省）。

**操作**：构建系统提示词。

**预期**：
- 输出含 `<user>梁爽</user>`；
- 输出**不含** `<user_description>` 与 `</user_description>`（不出现空标签、不出现空行残留）。

**验证方式**：`go test ./agent/ -run TestFeature535EmptyUserDescriptionOmitted`。

---

## UC-0003（单元）修改用户描述后提示词随之变化（不再空转）

**前置**：`IDENTITY.md` 的 `user-description` 初始为「后端工程师」。

**操作**：`agent.UpdateIdentityField(dir, "user-description", "运维负责人")` 后重建提示词。

**预期**：
- 重建后的提示词含 `<user_description>运维负责人</user_description>`；
- 旧值「后端工程师」不再出现在该字段中（证明而非死路径）。

**验证方式**：`go test ./agent/ -run TestFeature535UserDescriptionRebuild`。

---

## UC-0004（单元）无 RULES.md：默认模板 + `.rules/` 在 `{CUSTOM_RULES}` 处注入

**前置**：workspace 无 `./RULES.md`；`.rules/team.md` 内容为 `team rule`。

**操作**：构建系统提示词。

**预期**：
- RULES 节保留内置默认内容（含 4 条默认规则）；
- `.rules/team.md` 内容出现在默认段末尾的 `{CUSTOM_RULES}` 位置（`====\n\nteam\n\nteam rule`）；
- 输出中不含未解析的 `{DEFAULT_RULES}`、`{CUSTOM_RULES}`、`{CONTEXT_REORGANIZE_THRESHOLD}`（阈值已替换为配置值，默认 80）。

**验证方式**：`go test ./agent/ -run TestFeature535RulesDefaultPlusCustom`。

---

## UC-0005（单元）RULES.md 含 `{DEFAULT_RULES}`：展开为完整默认段并继续注入 `.rules/`

**前置**：`./RULES.md` 内容为 `我的规则\n\n{DEFAULT_RULES}`；`.rules/team.md` 内容为 `team rule`。

**操作**：构建系统提示词。

**预期**：
- 输出含「我的规则」；
- `{DEFAULT_RULES}` 被替换为整个内置默认段（含默认 4 条规则与原有说明文字）；
- 由于展开出的默认段内含 `{CUSTOM_RULES}`，`.rules/team.md` 的内容**仍然被注入**（顺序替换，无递归残留）。

**验证方式**：`go test ./agent/ -run TestFeature535DefaultRulesPlaceholder`。

---

## UC-0006（单元）RULES.md 缺 `{CUSTOM_RULES}` 且存在 `.rules/` → 告警且内容未注入

**前置**：`./RULES.md` 内容为 `只按我的规则来`（无任何占位符）；`.rules/team.md` 存在。

**操作**：构建系统提示词。

**预期**：
- 提示词中**不含** `team rule`（.rules/ 未注入，语义明确）；
- 输出一条告警日志（包含「未注入 / {CUSTOM_RULES}」等可检索关键词与 `.rules/` 路径）。

**验证方式**：`go test ./agent/ -run TestFeature535MissingCustomRulesWarns`。

---

## UC-0007（单元）Settings「规则」框回退默认文本

**前置**：临时 workspace 无 `./RULES.md`；随后另建一个含 `./RULES.md`（内容 `我的规则`）的 workspace。

**操作**：分别调用 `SettingsHandler.IdentityJSON()` 取 `rules` 字段。

**预期**：
- 无文件时：`rules` 非空，且等于 `i18n.T(KeySystemPromptRules)`（与 `--init` 导出原文一致，含 `{CUSTOM_RULES}` 字面量）；
- 有文件时：`rules` = 文件内容原文（`我的规则`）。

**验证方式**：`go test ./cmd/ -run TestFeature535IdentityRulesFallback`。

---

## UC-0008（单元）视觉识别轮不注入用户信息

**前置**：`IDENTITY.md` 含 user-name / user-description。

**操作**：调用 `buildVisionIdentityPrompt(cfg, dir)`。

**预期**：
- 输出中不含 `<user>` / `<user_description>` / `<system_info>`（该方法只构建 Identity 节）。

**验证方式**：`go test ./agent/ -run TestFeature535VisionPromptUnaffected`。

---

## UC-0009（运行时）真实会话中系统提示词与设置界面一致

**前置**：以 `--serve` 启动带本次改动的构建产物（不自动开浏览器）；工作区无 `./RULES.md`，`.rules/` 下有规则文件。

**操作**：① 在设置中把「用户描述」改为一段文本并保存；② 用 `:context show`（或等价命令）查看当前系统提示词；③ 打开「身份与个性 → 规则」标签。

**预期**：
- 提示词的 Environment 节出现 `<user>` 与本步保存的 `<user_description>` 文本，`<channel>` 只有频道名；
- 「规则」文本框中显示默认规则全文（非空），文本内容与 `--init` 导出的 `RULES.md` 一致；
- 修改用户描述后提示词立即刷新（下一回合生效），无需重启。

**验证方式**：人工按上述步骤核对（必要时配合 `grep` 已有上下文命令输出）。

---

## UC-0010（运行时）缺占位符告警在实际运行中可见

**前置**：以 `--serve` 启动带本次改动的产物；工作区存在 `./RULES.md`（内容 `只按我的规则来`）与 `.rules/team.md`。

**操作**：触发一次系统提示词构建（发起一轮对话或让它重建提示词），查看日志文件（`log/` 下当日日志）。

**预期**：
- 日志出现本次新增的告警条目，内容包含 `.rules/` 未注入的原因（缺 `{CUSTOM_RULES}`）与路径信息；
- 提示词 RULES 节确实不含 `.rules/team.md` 的内容。

**验证方式**：人工查看日志 + `:context show` 对照。

---

## UC-0011（运行时）从 UI 保存默认文本后 `.rules/` 仍生效（零漂移）

**前置**：工作区无 `./RULES.md`，`.rules/team.md` 内容为 `team rule`；以 `--serve` 启动。

**操作**：① 打开「身份与个性 → 规则」，不做任何编辑直接点击保存（此时写入的即 UI 显示的默认文本）；② 再发起一轮对话并查看系统提示词；③ 对比 `--init` 导出的 `./RULES.md`。

**预期**：
- 保存后 `./RULES.md` 内容 = UI 中显示的默认文本（含 `{CUSTOM_RULES}` 字面量，含说明文字）；
- 新提示词中 `.rules/team.md` 的内容**仍被注入**（说明保存在 RULES.md 里的 `{CUSTOM_RULES}` 字面量在运行期照常展开）；
- `--init` 导出的 `./RULES.md` 与本文件内容一致（口径统一，无版本漂移）。

**验证方式**：人工核对文件内容 + 提示词 + `--init` 导出结果。
