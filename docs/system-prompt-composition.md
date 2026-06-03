# System Prompt Composition Matrix

## 拼装逻辑

系统提示词由 `buildSystemPromptWithMode()` 拼装，各节之间用 `\n\n====\n\n` 分隔。

```
identity + sep + toolUsageSection + sep + resultModeText + sep + capabilities + sep + rulesText + sep + envText + sep + objectiveText
```

其中 **toolUsageSection** 的选取逻辑：
1. 如果调用方传入了非空 `toolUsageText`（即 `BuildToolUsagePrompt` 的返回值），则完全覆盖
2. 否则按 `shellEnabled` 选取基础 key（`KeySystemPromptToolUsage` / `KeySystemPromptToolUsageShell`）

`BuildToolUsagePrompt()` 仅在 XML 模式下被调用（`rebuildSystemPrompt`、`SetResultMode` 中判断模式后调用），OpenAI 模式不调用。

## 使用场景矩阵（按实际拼入顺序）

| 序号 | 节标题 | OpenAI + !Shell | OpenAI + Shell | XML + !Shell | XML + Shell |
|----|-------|----------------|----------------|-------------|-------------|
| 1 | (Identity) | `KeySystemPromptIdentity` | ← 同左 | ← 同左 | ← 同左 |
| 2 | **TOOL USE** | `KeySystemPromptToolUsage` | `KeySystemPromptToolUsageShell` | `KeySystemPromptToolUsageXML` + per-tool keys(`KeyToolUsage*`) | `KeySystemPromptToolUsageXMLShell` + per-tool keys(`KeyToolUsage*`) |
| 3 | # 工具使用示例 / # Tool Use Examples | — | — | `KeySystemPromptXMLExamples` | ← 同左 |
| 4 | **UPDATING TASK PROGRESS** | — | — | `KeySystemPromptXMLTaskProgress` | ← 同左 |
| 5 | **EDITING FILES** | — | — | `KeySystemPromptEditingFiles` | ← 同左 |
| 6 | **RESULT MODE** | `KeySystemPromptResultMode` | ← 同左 | ← 同左 | ← 同左 |
| 7 | **CAPABILITIES** | `KeySystemPromptCapabilities` | `KeySystemPromptCapabilitiesShell` | `KeySystemPromptCapabilities` | `KeySystemPromptCapabilitiesShell` |
| 8 | **RULES** | `KeySystemPromptRules` | `KeySystemPromptRulesShell` | `KeySystemPromptRules` | `KeySystemPromptRulesShell` |
| 9 | **SYSTEM INFORMATION** | `KeySystemPromptEnvironment` | ← 同左 | ← 同左 | ← 同左 |
| 10 | **OBJECTIVE** | `KeySystemPromptObjective` | ← 同左 | ← 同左 | ← 同左 |

> 注：序号 3/4/5 当前在代码中组装在 TOOL USE 节内，用 `\n====\n` 分隔，但语义上是独立节。OpenAI 模式下已有对应的未接入资源（`KeySystemPromptToolUsageExamples`、`KeySystemPromptToolUsageTaskProgress`）。

## 系统中涉及的所有 i18n Key

### 系统提示词节（7 个顶层节，10 个语义节）

| 序号 | Key | 用途 | 使用位置 |
|----|-----|------|---------|
| 1 | `KeySystemPromptIdentity` | 身份描述（agentName / agentDescription / agentPrinciples） | `system_prompt.go` |
| 1a | `KeyDefaultAgentDescription` | Identity 中 agentDescription 的默认值 | `system_prompt.go:76` |
| 1b | `KeyDefaultAgentPrinciples` | Identity 中 agentPrinciples 的默认值 | `system_prompt.go:79` |
| 2 | `KeySystemPromptToolUsage` | TOOL USE 节内容（OpenAI/非Shell） | `system_prompt.go:90` |
| 2s | `KeySystemPromptToolUsageShell` | TOOL USE 节内容（OpenAI/Shell） | `system_prompt.go:90` |
| 2x | `KeySystemPromptToolUsageXML` | TOOL USE 节内容（XML/非Shell，含 `# Tool Use Formatting`） | `toolcall_mode.go:1172` |
| 2xs | `KeySystemPromptToolUsageXMLShell` | TOOL USE 节内容（XML/Shell，含 `# Tool Use Formatting`） | （已定义但当前 XML 模式未区分 Shell 版本） |
| 2t | `KeyToolUsage*`（多个 per-tool key） | 每个工具的详细 XML 描述 | `toolcall_mode.go:1178-1181` |
| 3 | `KeySystemPromptXMLExamples` | # 工具使用示例 + # Tool Use Guidelines | `toolcall_mode.go:1185` |
| 3o | `KeySystemPromptToolUsageExamples` | OpenAI 模式工具示例（**未接入**） | `BuildToolUsagePrompt` 中定义 |
| 4 | `KeySystemPromptXMLTaskProgress` | UPDATING TASK PROGRESS | `toolcall_mode.go:1187` |
| 4o | `KeySystemPromptToolUsageTaskProgress` | OpenAI 模式任务进度管理（**未接入**） | `BuildToolUsagePrompt` 中定义 |
| 5 | `KeySystemPromptEditingFiles` | EDITING FILES（通用版本） | `toolcall_mode.go:1189` |
| 6 | `KeySystemPromptResultMode` | RESULT MODE 节内容 | `system_prompt.go:96` |
| 7 | `KeySystemPromptCapabilities` | CAPABILITIES 节（非Shell） | `system_prompt.go:105` |
| 7s | `KeySystemPromptCapabilitiesShell` | CAPABILITIES 节（Shell） | `system_prompt.go:105` |
| 8 | `KeySystemPromptRules` | RULES 节（非Shell） | `system_prompt.go:115` |
| 8s | `KeySystemPromptRulesShell` | RULES 节（Shell） | `system_prompt.go:115` |
| 9 | `KeySystemPromptEnvironment` | SYSTEM INFORMATION 节内容，含 `{CWD}` / `{OS}` 等 | `system_prompt.go:131` |
| 9a | `KeyAnonymousUser` | 用户显示名称默认值（Environment 节） | `system_prompt.go:142` |
| 10 | `KeySystemPromptObjective` | OBJECTIVE 节内容，含 `{TASK}` / `{TASK_TRACKING}` | `system_prompt.go:124` |

### 工具结果模板（不属于系统提示词，但用于构建 LLM 消息上下文）

| Key | 用途 | 使用位置 |
|-----|------|---------|
| `KeyXMLToolResultTemplate` | XML 模式工具结果模板，含 `{TOOL_CALL}` / `{TOOL_RESULT}` / `{TASK_TRACKING}` | `agent.go:462` |
| `KeyToolResultNoPlan` | 无任务计划时的下一步提示 | `agent.go:492` |
| `KeyToolResultWithPlan` | 有任务计划时的下一步提示，含 `{TASK_PLAN}` | `agent.go:489` |
| `KeyUserMessageTemplate` | 用户消息模板，含 `{INSTRUCTION}` / `{TASK_TRACKING}` | `agent.go:472` |

### 已清理的删除 key（不再使用）

| Key | 原因 |
|-----|------|
| `KeySystemPromptToolExamples` | legacy 未使用 |
| `KeySystemPromptToolGuidelines` | legacy 未使用 |
| `KeySystemPromptUpdatingProgress` | legacy 未使用 |
| `KeySystemPromptXMLEditingFiles` | 重命名为 `KeySystemPromptEditingFiles` |
| `KeySystemPromptXMLGuidelines` | 未使用（内容合并到了 Examples 中） |