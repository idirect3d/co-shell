# FEATURE-447 工具调用透明化：meta 参数改造设计文档

> **版本**: v0.20.0（BUILD-689）
> **分支**: FEATURE-447
> **日期**: 2026-08-29
> **状态**: ✅ 已完成

---

## 1. 背景与目标

### 1.1 背景

在 co-shell 的 Agent 执行过程中，LLM 每次调用工具时，用户无法直观看到：

- 本次调用的**意图**（要做什么）
- 本次操作的**风险等级**（低/中/高）
- 本次操作**影响哪些对象**（文件、数据库记录、API 资源等）
- 当前**任务进展**（正在执行任务计划的哪一步）

这些信息对提升用户体验与意图暴露至关重要。为此，FEATURE-447 引入**工具调用透明化**机制：让 LLM 每次调用工具时，通过统一的 **meta 对象** 主动报告这些元数据。

### 1.2 目标

1. **统一 meta 对象参数**：所有工具调用必须携带一个必填的 `meta` 对象参数（放最前），包含 `intent`/`risk`/`risk_reason`/`affected_objects`/`progress`。
2. **移除顶层 intent 参数**：工具定义中不再有顶层 `intent` 参数，统一用 `meta.intent`。
3. **视觉工具增加 instruct 参数**：`visual_analysis` / `browser_screenshot` 额外带 `instruct` 参数（给视觉模型的指令），`meta.intent` 给用户。
4. **meta 对象结构说明放系统提示词**：节约上下文，避免每个工具定义重复描述。
5. **meta.progress 与 track_task_progress 保持一致**：progress 用于更新任务执行状态，index 必须准确、状态定义一致。

---

## 2. meta 对象结构设计

### 2.1 结构定义

每个工具调用都必须携带一个**必需**的 `meta` 对象参数，作为第一个参数。其结构如下：

| 字段 | 必需 | 说明 |
|------|------|------|
| `intent` | ✅ | 本次调用的意图说明——展示给用户 |
| `risk` | ✅ | 风险等级自评：`low`/`medium`/`high`，缺失或无效将导致工具调用错误 |
| `risk_reason` | 可选 | 风险评估的简要理由 |
| `affected_objects` | 可选 | 本次操作影响的对象（文件/文件夹等），绝对路径数组，最多 3 个 |
| `progress` | 可选 | 任务进展报告——对象数组，每个含 `index`/`description`/`status` |

### 2.2 risk 等级定义

| 等级 | 含义 |
|------|------|
| `low` | 只读操作（读取文件、搜索、列出） |
| `medium` | 在工作区内新建或修改文件 |
| `high` | 删除/覆盖文件、写入工作区之外、执行系统命令，或涉及敏感位置（用户主目录私有目录如 `.ssh`/`.aws`/`.config`，或系统目录如 `/etc`/`/usr`） |

同时要考虑操作是否可能涉及用户的敏感信息（读取用户主目录、系统文件夹、凭据等）。

### 2.3 progress 与 track_task_progress 的一致性约定

`meta.progress` 用于更新 `track_task_progress` 制订的任务执行状态，**必须与 `track_task_progress` 保持一致**：

- **index 必须准确**：每个 `index` 必须与任务计划中对应步骤的索引完全一致。
- **状态定义一致**：每个 `status` 必须取 `track_task_progress` 使用的同一组状态值：

| 状态值 | 显示符号 | 含义 |
|--------|----------|------|
| `pending` | `[ ]` | 待执行 |
| `in_progress` | `[=]` | 执行中 |
| `completed` | `[X]` | 已完成 |
| `cancelled` | `[C]` | 已取消 |
| `failed` | `[F]` | 失败 |

- 只报告状态有变化的步骤以及当前正在执行的步骤。
- `index` 等于当前步骤数表示追加新步骤；超出该范围则报错。

### 2.4 视觉工具 instruct 参数

视觉工具（`visual_analysis` / `browser_screenshot`）额外带一个 **`instruct`** 参数——给视觉模型的明确指令，描述要从图像中分析/提取什么。这与 `meta.intent`（展示给用户的意图）不同。

---

## 3. 实现过程

### 3.1 后端解析

#### 3.1.1 `agent/risk.go` — 风险等级解析

`assessRisk` 从 `meta.risk` 解析风险等级，`metaObject` 辅助函数提取 meta 对象：

```go
// metaObject extracts the meta object from tool args.
func metaObject(args map[string]interface{}) map[string]interface{} {
    if m, ok := args["meta"].(map[string]interface{}); ok {
        return m
    }
    return nil
}

// assessRisk parses the risk level from meta.risk.
func assessRisk(args map[string]interface{}) (string, string, error) {
    meta := metaObject(args)
    risk, _ := meta["risk"].(string)
    // ... 校验 risk 取值，缺失或无效返回错误
}
```

#### 3.1.2 `agent/files.go` — 影响对象解析

`affectedFiles` 从 `meta.affected_objects` 解析影响对象（绝对路径数组，最多 3 个），每个条目携带 `Sensitive` 标志（是否触及用户私有或系统关键位置，仅用于前端展示，不强制风险等级）：

```go
// extractPredictedFiles collects the LLM-reported affected files from the
// meta.affected_objects field. At most maxPredictedFiles are kept.
func extractPredictedFiles(args map[string]interface{}) []string {
    meta := metaObject(args)
    var files []string
    if v, ok := meta["affected_objects"]; ok {
        // ... 解析字符串数组，最多 maxPredictedFiles 个
    }
    return files
}
```

#### 3.1.3 `agent/progress.go` — 任务进展解析

`parseProgressSteps` 从 `meta.progress` 解析任务进展步骤，校验 index 与状态：

```go
// parseProgressSteps parses the progress report from meta.progress.
func parseProgressSteps(args map[string]interface{}) ([]ProgressStep, error) {
    meta := metaObject(args)
    // ... 解析 progress 数组，校验 index 与 status
}
```

#### 3.1.4 `agent/tool_summary.go` — 意图解析

`buildToolSummary` 从 `meta.intent` 解析工具调用意图，用于前端展示。

### 3.2 工具定义源码直接声明

`agent/tools.go` 的 `buildToolsInternal` 中，每个工具定义**直接声明** meta 对象参数（放最前，移除顶层 intent），视觉工具直接声明 instruct 参数：

```go
{
    Name: "read_file",
    Description: "Read the contents of a file...",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "meta": map[string]interface{}{
                "type": "object",
                "description": "**REQUIRED**: Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.",
                "properties": map[string]interface{}{ /* ... */ },
                "required": []string{"intent", "risk"},
            },
            "path": map[string]interface{}{ /* ... */ },
            // ...
        },
        "required": []string{"meta", "path", "start_line", "end_line"},
    },
},
```

**关键改动**：移除对所有工具的 `injectMetaParam` 运行时注入循环（仅保留对 MCP 工具注入，因其 InputSchema 来自 MCP 服务器）。新增 `agent/meta_param_test.go` 验证所有工具定义均含 meta 参数。

### 3.3 视觉工具

- `agent/image_tools.go`：`visual_analysis` 从 `args["instruct"]` 读取视觉指令。
- `agent/browser_tools.go`：`browser_screenshot` 从 `args["instruct"]` 读取视觉指令。

### 3.4 系统提示词

`i18n/en_system.go` / `i18n/zh_system.go` 中：

1. **meta 对象结构说明**：在 `KeySystemPromptToolUsage` 和 `KeySystemPromptToolUsageXML` 中写入 meta 对象结构说明（intent/risk/risk_reason/affected_objects/progress）+ instruct 说明 + XML 示例。
2. **meta.progress 约定**：明确写入 progress 与 track_task_progress 的一致性约定（index 必须准确、状态定义一致）。
3. **合并资源**：将原 `KeySystemPromptToolTransparency` 内容合并到 `KeySystemPromptToolUsage` 和 `KeySystemPromptToolUsageXML`，删除独立的 `KeySystemPromptToolTransparency` 资源（`i18n/keys.go` 常量 + `agent/toolcall_mode.go` 单独拼接逻辑）。

### 3.5 测试

- `agent/meta_param_test.go`：验证所有工具定义均含 meta 参数。
- `agent/transparency_test.go`：验证 risk/affected_objects/progress 解析。
- `i18n/i18n_test.go`：删除已失效的 `TestTEmptyTranslationReturnsEmpty`（KeySystemPromptToolUsage 不再为空）。

---

## 4. 验证结果

- `go build ./... && go vet ./... && go build -o work/co-shell .` 全部通过
- `go test ./... -short` 全部通过
- build 计数更新至 689

---

## 5. 后续待确认需求

用户提出：**meta 中的所有属性/参数都改为必需（包括 affected_objects），如果方法不影响任何实体对象，则 affected_objects 填写 `nothing` 明确表示没有影响任何对象**。此需求尚未实现，待确认后继续。
