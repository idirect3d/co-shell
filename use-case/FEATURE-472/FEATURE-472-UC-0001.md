# FEATURE-472 ResultMode 节静态化 — 测试用例

> 分支：FEATURE-472 ｜ 版本：v0.35.0
> 目标：将系统提示词 RESULT MODE 节从"只描述当前模式"改为静态列出所有已配置模式（标题 + 引导句 + 各模式介绍），让 LLM 同时了解各模式差异。

## UC-0001 默认三模式生成静态 ResultMode 节

**前置**：`i18n.Init("en")`；cfg 使用 `config.DefaultConfig()`，`cfg.WorkModes` 为空（无用户自定义），`cfg.LLM.WorkMode = "act"`。

**操作**：调用 `buildNamedSection("ResultMode", env, cfg, false, nil, false)`。

**预期**：
- 返回非空字符串。
- 标题包含 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE`。
- 引导句包含 `There are 3 modes:`。
- 依次包含 ACT MODE / PLAN MODE / RESEARCH MODE 三个模式的介绍（来自 KeyWorkModeAct/Plan/Research 资源，非空）。
- 三个模式介绍顺序为 act → plan → research。

**验证方式**：单测断言字符串包含上述子串。

## UC-0002 用户自定义模式被纳入

**前置**：`i18n.Init("en")`；cfg 的 `cfg.WorkModes` 含一个自定义模式 `{Name:"review", Description:"Review mode - review code"}`，`cfg.LLM.WorkMode = "act"`。

**操作**：调用 `buildNamedSection("ResultMode", env, cfg, false, nil, false)`。

**预期**：
- 标题包含 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE V.S. REVIEW MODE`。
- 引导句包含 `There are 4 modes:`。
- 自定义模式 review 的介绍来自其 `Description`（"Review mode - review code"）。

**验证方式**：单测断言标题含 REVIEW MODE、引导句含 4、介绍含 review 的 Description。

## UC-0003 用户自定义覆盖内置同名模式

**前置**：`i18n.Init("en")`；cfg 的 `cfg.WorkModes` 含 `{Name:"act", Description:"Custom act override"}`（覆盖内置 act），`cfg.LLM.WorkMode = "act"`。

**操作**：调用 `buildNamedSection("ResultMode", env, cfg, false, nil, false)`。

**预期**：
- 标题仍为 `ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE`（act 不重复）。
- act 的介绍使用自定义 Description（"Custom act override"），而非内置 KeyWorkModeAct。

**验证方式**：单测断言标题不含重复 ACT MODE、介绍含 "Custom act override"。

## UC-0004 模式介绍为空时回退到 WorkMode.Description

**前置**：`i18n.Init("en")`；cfg 的 `cfg.WorkModes` 含 `{Name:"custom", Description:"Custom mode desc"}`；内置 act/plan/research 的 KeyWorkModeAct/Plan/Research 资源为空（模拟未填充场景）。

**操作**：调用 `buildNamedSection("ResultMode", env, cfg, false, nil, false)`。

**预期**：
- 每个模式的介绍非空：内置模式回退到其 WorkMode.Description（KeyWorkModeActDesc 等），自定义模式用其 Description。
- 无空介绍条目。

**验证方式**：单测断言每个模式条目后都有非空介绍文本。

## UC-0005 中英双语资源填充

**前置**：分别 `i18n.Init("zh")` 和 `i18n.Init("en")`。

**操作**：读取 `i18n.T(i18n.KeyWorkModeAct)`、`KeyWorkModePlan`、`KeyWorkModeResearch`。

**预期**：
- zh 与 en 下三个键均非空。
- 内容为 cline 风格的模式介绍（参考 notes/cline.json 的 ACT MODE V.S. PLAN MODE 段落），act/plan 参考 cline 原文，research 为拟写初稿。

**验证方式**：单测断言三个键在 zh/en 下均非空且长度合理。

## UC-0006 三个内置模式都包含 ResultMode 节

**前置**：`i18n.Init("en")`；cfg 使用 `config.DefaultConfig()`。

**操作**：分别设置 `cfg.LLM.WorkMode` 为 act/plan/research，调用 `buildSystemPromptWithMode` 生成系统提示词。

**预期**：
- 三种模式下系统提示词均包含 ResultMode 节内容（标题 ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE）。
- 三种模式下 ResultMode 节内容一致（静态化，不随当前模式变化）。

**验证方式**：单测断言三种模式生成的系统提示词中 ResultMode 节文本相同。

## UC-0007 运行时用例（REPL 验证）

**前置**：启动 co-shell（`go run . run`），当前模式 act。

**操作**：输入一条简单指令（如 `:mode` 查看模式），观察系统提示词（可用 `:context` 或日志查看）。

**预期**：
- 系统提示词 RESULT MODE 节列出所有已配置模式（ACT/PLAN/RESEARCH MODE）及各自介绍。
- 切换到 plan 模式后，RESULT MODE 节内容不变（仍列出所有模式）。

**验证方式**：人工确认系统提示词输出。
