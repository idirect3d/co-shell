# 参数修改后未立即生效清单

> 分析日期: 2026-05-04
> 分析范围: cmd/settings.go (REPL .set 命令), agent/settings_tools.go (LLM update_settings 工具), agent/agent.go (Agent Setter 方法), agent/loop.go (Agent 运行时使用)

## 分类标准

根据参数在 Agent 运行时的使用方式，分为三类：

- **A类 - Agent 独立字段**: Agent 结构体中有独立字段（如 `a.showLlmThinking`），需通过 Setter 方法同步。**修改 cfg 不会自动生效**。
- **B类 - 通过 a.cfg 指针读取**: Agent 运行时从 `a.cfg.LLM.xxx` 读取。由于 `h.cfg` 和 `a.cfg` 是同一指针，**修改 cfg 后立即生效**。
- **C类 - LLM 客户端内部状态**: 参数存储在 `llm.Client` 实现中（如 `openAIClient` 的字段），需通过 `rebuildLLMClient()` 重建客户端才能生效。

---

## 一、A类：Agent 独立字段 — 必须显式同步

### 1. `show-llm-content` ⚠️ 未同步

| 维度 | 说明 |
|------|------|
| **Agent 字段** | `a.showLlmContent`（`loop.go:83`） |
| **REPL 路径** | `cmd/settings.go:321-345` — 只修改 cfg 并保存，**缺少 `h.agent.SetShowLlmContent()` 调用** |
| **LLM 工具路径** | `agent/settings_tools.go:398-407` — ✅ 正确调用了 `a.SetShowLlmContent(b)` |
| **影响** | 修改后 `a.showLlmContent` 仍为旧值，影响 `streamLLMResponse` 中 thinking 内容的显示判断 |
| **修复** | 在 `cmd/settings.go` 的 `show-llm-content` case 中增加 `h.agent.SetShowLlmContent()` 调用 |

---

## 二、B类：通过 a.cfg 指针读取 — 已自动生效

以下参数 Agent 运行时从 `a.cfg.LLM.xxx` 读取，由于 `h.cfg` 和 `a.cfg` 是同一指针，修改 cfg 后立即生效：

| 参数 | Agent 使用位置 | 说明 |
|------|---------------|------|
| `context-limit` | `loop.go:579-623` `buildContextMessages()` | ✅ 已生效 |
| `tool-timeout` | `loop.go:273-278` `getToolTimeout()` | ✅ 已生效 |
| `cmd-timeout` | `loop.go:282-287` `getCommandTimeout()` | ✅ 已生效 |
| `search-max-line-length` | `file_tools.go` 搜索工具 | ✅ 已生效 |
| `search-max-result-bytes` | `file_tools.go` 搜索工具 | ✅ 已生效 |
| `search-context-lines` | `file_tools.go` 搜索工具 | ✅ 已生效 |
| `memory-search-max-content-len` | `memory_tools.go` 记忆工具 | ✅ 已生效 |
| `memory-search-max-results` | `memory_tools.go` 记忆工具 | ✅ 已生效 |
| `error-max-single-count` | `loop.go:312-314` 错误计数 | ✅ 已生效 |
| `error-max-type-count` | `loop.go:316-318` 错误计数 | ✅ 已生效 |
| `max-retries` | 当前 Agent 未直接使用此字段 | ✅ 无影响 |

---

## 三、C类：LLM 客户端内部状态 — 需 rebuildLLMClient()

以下参数存储在 `openAIClient` 结构体中，修改后必须重建 LLM 客户端才能生效。

### 已正确调用 rebuildLLMClient() 的参数

| 参数 | REPL 路径 | LLM 工具路径 |
|------|-----------|-------------|
| `api-key` | ✅ `cmd/settings.go:83` | ✅ `agent/settings_tools.go:308` |
| `endpoint` | ✅ `cmd/settings.go:96` | ✅ `agent/settings_tools.go:316` |
| `model` | ✅ `cmd/settings.go:109` | ✅ `agent/settings_tools.go:324` |
| `temperature` | ✅ `cmd/settings.go:129` | ✅ `agent/settings_tools.go:339` |
| `max-tokens` | ✅ `cmd/settings.go:149` | ✅ `agent/settings_tools.go:354` |
| `vision` | ✅ `cmd/settings.go:517` | ✅ `agent/settings_tools.go:502` |
| `thinking-enabled` | ✅ `cmd/settings.go:780` | ✅ `agent/settings_tools.go:514` |
| `reasoning-effort` | ✅ `cmd/settings.go:803` | ✅ `agent/settings_tools.go:527` |

### ⚠️ 两个 rebuildLLMClient() 实现不一致

**`cmd/settings.go:54-64`（REPL 路径）：**
```go
func (h *SettingsHandler) rebuildLLMClient() {
    client := llm.NewClient(
        h.cfg.LLM.Endpoint,
        h.cfg.LLM.APIKey,
        h.cfg.LLM.Model,
        h.cfg.LLM.Temperature,
        h.cfg.LLM.MaxTokens,
        // ❌ 缺少 LLMTimeout 参数
    )
    h.agent.SetLLMClient(client)
    // ❌ 缺少 SetThinkingEnabled() 和 SetReasoningEffort()
}
```

**`agent/settings_tools.go:781-793`（LLM 工具路径）：**
```go
func (a *Agent) rebuildLLMClient() {
    client := llm.NewClient(
        a.cfg.LLM.Endpoint,
        a.cfg.LLM.APIKey,
        a.cfg.LLM.Model,
        a.cfg.LLM.Temperature,
        a.cfg.LLM.MaxTokens,
        a.cfg.LLM.LLMTimeout,  // ✅ 传了 LLMTimeout
    )
    client.SetThinkingEnabled(a.cfg.LLM.ThinkingEnabled)   // ✅
    client.SetReasoningEffort(a.cfg.LLM.ReasoningEffort)   // ✅
    a.SetLLMClient(client)
}
```

**影响：** 通过 REPL 修改 `api-key`/`endpoint`/`model`/`temperature`/`max-tokens`/`vision`/`thinking-enabled`/`reasoning-effort` 后，虽然客户端被重建，但：
1. `LLMTimeout` 丢失（使用默认 60s）
2. `ThinkingEnabled` 丢失（重置为 false）
3. `ReasoningEffort` 丢失（重置为 "low"）

---

## 四、REPL 路径完全缺失的参数

以下参数在 `agent/settings_tools.go`（LLM 工具路径）中有完整处理，但在 `cmd/settings.go`（REPL 路径）中**完全没有 case 分支**：

| 参数 | 类型 | 影响 |
|------|------|------|
| `tool-timeout` | B类（通过 a.cfg 读取） | 用户无法通过 `.set tool-timeout` 设置 |
| `cmd-timeout` | B类（通过 a.cfg 读取） | 用户无法通过 `.set cmd-timeout` 设置 |
| `llm-timeout` | C类（需 rebuildLLMClient） | 用户无法通过 `.set llm-timeout` 设置；且需 rebuildLLMClient 才生效 |

---

## 五、Wizard 路径 — 完全缺失同步

### 问题描述

通过 `.wizard` 命令（`repl/repl.go:387-396`）修改配置后，wizard 只修改了 `r.cfg` 并调用 `cfg.Save()`，但**完全没有将修改同步到 Agent**。

对比 `.set` 命令（`repl/repl.go:340-348`）：
```go
// .set 命令在 Handle 返回后，会同步以下参数到 agent
if command == ".settings" || command == ".set" {
    r.agent.SetShowLlmThinking(r.cfg.LLM.ShowLlmThinking)
    r.agent.SetShowLlmContent(r.cfg.LLM.ShowLlmContent)
    r.agent.SetShowTool(r.cfg.LLM.ShowTool)
    r.agent.SetShowToolInput(r.cfg.LLM.ShowToolInput)
    r.agent.SetShowToolOutput(r.cfg.LLM.ShowToolOutput)
    r.agent.SetShowCommand(r.cfg.LLM.ShowCommand)
    r.agent.SetShowCommandOutput(r.cfg.LLM.ShowCommandOutput)
}
```

而 `.wizard` 命令（`repl/repl.go:387-396`）：
```go
func (r *REPL) handleWizard() {
    if wizard.RunSetupWizard(r.cfg) {
        fmt.Print(i18n.T(i18n.KeyWizardCmdDone))
    }
    // ❌ 没有任何同步到 agent 的操作
    // ❌ 没有 rebuildLLMClient()
    // ❌ 没有同步 show-llm-thinking 等显示参数
}
```

### Wizard 修改的参数清单

| 参数 | 类型 | 当前效果 | 说明 |
|------|------|---------|------|
| `endpoint` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |
| `api-key` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |
| `model` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |
| `temperature` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |
| `max-tokens` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |
| `vision` | C类（LLM 客户端） | ❌ 不生效 | 需要 `rebuildLLMClient()` |

### 影响

通过 `.wizard` 修改 LLM API 配置后，Agent 仍然使用旧的 LLM 客户端（旧的 endpoint/api-key/model），导致：
1. 修改 endpoint 后仍连接旧地址
2. 修改 api-key 后仍使用旧 key
3. 修改 model 后仍使用旧模型
4. 修改 temperature/max-tokens 后仍使用旧值

**用户必须重启 co-shell 才能让 wizard 的修改生效。**

---

## 六、总结：需要修复的问题

| 优先级 | 参数 | 问题 | 修复方式 |
|--------|------|------|---------|
| **最高** | wizard 修改的所有 LLM 参数 | `.wizard` 命令后没有 `rebuildLLMClient()` | 在 `handleWizard()` 成功后调用 `rebuildLLMClient()` |
| **高** | `show-llm-content` | REPL 路径缺少 `SetShowLlmContent()` 调用 | 增加一行同步调用 |
| **高** | `rebuildLLMClient()` 在 `cmd/settings.go` 中 | 缺少 `LLMTimeout`、`SetThinkingEnabled()`、`SetReasoningEffort()` | 对齐到 `agent/settings_tools.go` 版本 |
| **中** | `tool-timeout` | REPL 路径缺少 case 分支 | 新增 case 分支 |
| **中** | `cmd-timeout` | REPL 路径缺少 case 分支 | 新增 case 分支 |
| **中** | `llm-timeout` | REPL 路径缺少 case 分支 | 新增 case 分支，调用 rebuildLLMClient() |
