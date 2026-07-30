# FEATURE-299 :set default 命令 — 重置非关键配置到系统默认值

## 基本信息

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-299 |
| 任务名称 | :set default 命令 |
| 类型 | 新特性 (Feature) |
| 版本 | v0.6.0 |
| 作者 | L.Shuang |

---

## 测试环境

- Go 1.22+
- REPL 环境（:set / :settings 命令）

---

## 测试用例

### UC-0001: :set 输出末尾显示 `:set default` 提示

**描述**：执行 `:set` 或 `:settings` 命令时，在输出末尾显示一行提示，告知用户可用 `:set default` 重置非关键配置。

**预期结果**：
- `:set` 命令输出末尾显示类似 `💡 提示：使用 :set default 可将除 LLM、记忆与上下文、数据库以外的所有配置重置为系统默认值。` 的提示
- 提示文本应支持中英文切换

---

### UC-0002: :set default 重置 LLM 以外的配置

**描述**：执行 `:set default` 后，LLM & Generation（temperature, max-tokens, thinking-enabled 等）配置应保持不变。

**前提条件**：
- 已修改 temperature 为 1.5，max-tokens 为 100000
- 已修改 display.theme 为 "dark"

**预期结果**：
- temperature = 1.5（保持不变）
- max-tokens = 100000（保持不变）
- display.theme → "default"（重置为默认值）
- display.language → "zh"（保持之前用户设定的值或默认值）

---

### UC-0003: :set default 重置 Memory & Context 以外的配置

**描述**：执行 `:set default` 后，Memory & Context（memory-enabled, context-limit 等）配置应保持不变。

**前提条件**：
- 已修改 memory-enabled = false，context-limit = 0

**预期结果**：
- memory-enabled = false（保持不变）
- context-limit = 0（保持不变）

---

### UC-0004: :set default 重置 Database 以外的配置

**描述**：执行 `:set default` 后，Database 相关配置保持不变。

**前提条件**：
- 已修改 db.host = "10.0.0.1"，db.enabled = true

**预期结果**：
- db.host = "10.0.0.1"（保持不变）
- db.enabled = true（保持不变）

---

### UC-0005: :set default 重置 Safety/Tools/Search/Display/Log 配置

**描述**：执行 `:set default` 后，Safety、Tools、Search、Display（除 LLM 外）、Log 等所有非关键配置应重置为系统默认值。

**前提条件**：
- 已修改 confirm-tool = "auto"，command-timeout = 10
- 已修改 log-level = "debug"

**预期结果**：
- confirm-tool → "custom"（恢复默认）
- command-timeout → 0（unlimited）（恢复默认）
- log-level → "info"（恢复默认）

---

### UC-0006: :set default 成功后显示确认消息

**描述**：执行 `:set default` 成功后，应显示一条确认消息。

**预期结果**：
- 显示类似 `✅ 配置已成功重置为系统默认值（保留了 LLM、记忆与上下文和数据库配置）。` 的消息
- 消息应支持中英文切换

---

### UC-0007: :set default 配置自动持久化

**描述**：执行 `:set default` 后，配置应自动保存到 config.json。

**预期结果**：
- config.json 中的对应字段已更新为默认值
- 重新加载 co-shell 后，重置的配置保持为默认值