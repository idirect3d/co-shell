# FEATURE-602 UC-0004：原地重试次数可配置

> 分支：**FEATURE-602**　版本：**v0.94.0**　BUILD：**1211**
> 判定口径：FIX-598 的"原地重试"上限（原为写死常量 `llmRetryLimit = 3`）改为系统参数 `llm-err-retry-limit`，默认 3；可通过 `.set llm-err-retry-limit <n>` 修改并持久化；Web 设置页可见可改。

## 背景

FIX-598 对 network/5xx/429 失败会**原地重试**（不改上下文、指数退避 1s/2s/4s…，上限 8s），次数写死为常量 3（`agent/llm_error_class.go` 的 `llmRetryLimit`）。用户提问（2026-10-04 12:39 备注）希望该次数可通过系统参数调整。本次一并支持，并作为"原地重试耗尽后再降级"的判据。

## 变更点（预计）

| 位置 | 变更 |
|---|---|
| `config/config.go` | `LLMConfig` 增加 `LLMErrRetryLimit int`（json `llm_err_retry_limit,omitempty`）；`DefaultConfig` 设 3；≤0 时按 3 处理（沿用 `error_max_single_count` 的"零值回落默认"惯例） |
| `agent/llm_error_class.go` | 常量 `llmRetryLimit` 保留为默认值；新增 `func (a *Agent) llmRetryLimit() int` 读取 cfg，`noteLLMErrRetryAndCheckLimit` 使用它 |
| `cmd/settings.go` | 分发 + `showSettingsHelp` 行 |
| `cmd/settings_safety.go` | `.set llm-err-retry-limit <n>`：校验 n≥1、保存 config |
| `cmd/settings_web.go` | Web 设置页新增 `{Key:"llm-err-retry-limit", Type:"number"}` |
| `agent/settings_tools.go` | LLM 工具 get/set 支持该键 |
| `i18n` | 参数描述中英词条 |

## 用例与结果

- **UC-01（S）默认值** → `config.DefaultConfig().LLM.LLMErrRetryLimit == 3`；`a.llmRetryLimit() == 3`（cfg 未设时）。
- **UC-02（S）可修改并持久化** → `.set llm-err-retry-limit 5` → `cfg.LLM.LLMErrRetryLimit == 5` 且 config.json 含 `"llm_err_retry_limit": 5`，重载后仍为 5。
- **UC-03（S）生效** → 设为 5 时，`noteLLMErrRetryAndCheckLimit()` 允许第 5 次、拒绝第 6 次；设为 1 时允许第 1 次、拒绝第 2 次。
- **UC-04（S）非法值拒绝** → `0` / 负数 / 非数字被拒绝，返回错误且不改配置。
- **UC-05（S）旧配置兼容** → 不含该键的旧 config.json 加载后取默认 3（零值回落）。
- **UC-06（R）Web 设置页** → 设置页出现 `llm-err-retry-limit`（number 类型，默认 3），修改后保存成功并回显新值。
- **UC-07（S）构建** → `go build ./... && go vet ./...` 通过；`go test ./config/ ./cmd/ ./agent/ -short -run TestFeature602` 通过。

## 不在本次范围

- 不改动退避算法（1s/2s/4s…，上限 8s）。
- 不为 `llm-err-retry-limit` 增加 CLI 启动参数（仅 `.set` / Web 设置页 / 配置文件）。
