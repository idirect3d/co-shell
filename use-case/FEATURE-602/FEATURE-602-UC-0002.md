# FEATURE-602 UC-0002：降级触发条件矩阵

> 分支：**FEATURE-602**　版本：**v0.94.0**　BUILD：**1211**
> 判定口径：LLM 调用失败后，只有落在「连接类 + 服务类 + 模型内部类」的错误才触发降级；请求格式类（context-format）与无法归类（unknown）不降级，维持 FIX-598 的原有处理。

## 背景

FIX-598 已把 LLM 失败分类为 `LLMErrKind`：`network / rate-limit / server / auth / context-format / context-too-long / unknown`（`agent/llm_error_class.go`）。用户确认（2026-10-04 12:39）本次降级触发集合为：

| 错误分类 | 含义 | 触发降级 |
|---|---|---|
| `network` | 网络/超时（DNS/dial/TLS/reset/timeout） | ✅ |
| `server` | 服务端 5xx | ✅ |
| `rate-limit` | 429 限流 | ✅ |
| `auth` | 凭据/找不到模型/限额不足/需充值/无权限（401/403/404 及文本） | ✅ |
| `context-too-long` | 最大长度超限 | ✅ |
| `context-format` | 消息格式非法（tool_calls 结构错） | ❌ |
| `unknown` | 无法归类 | ❌ |

## 变更点（预计）

| 位置 | 变更 |
|---|---|
| `agent/llm_error_class.go` | 新增 `func (k LLMErrKind) Fallbackable() bool`：仅上述 5 类返回 true |
| `agent/fix602_test.go` | table-driven 用例覆盖 7 类 → 期望 true/false |

## 用例与结果

- **UC-01（S）network 触发** → `Fallbackable()==true`。
- **UC-02（S）server 触发** → `Fallbackable()==true`；`&llm.OpenAIError{StatusCode:503}` 经 `ClassifyLLMError` 得 server。
- **UC-03（S）rate-limit 触发** → `Fallbackable()==true`；429 → rate-limit。
- **UC-04（S）auth 触发** → `Fallbackable()==true`；401/403/404 与 "model not found" / "insufficient quota" / "permission denied" 文本 → auth。
- **UC-05（S）context-too-long 触发** → `Fallbackable()==true`；"maximum context length is …" → context-too-long。
- **UC-06（S）context-format 不触发** → `Fallbackable()==false`（仍走 FIX-598 的"移除最后一条带 tool_calls 的 assistant 消息"路径）。
- **UC-07（S）unknown 不触发** → `Fallbackable()==false`（保守处理，不改模型、不删上下文）。
- **UC-08（S）开关关闭时不降级** → `AutoSelectEnabled()==false` 时，即使分类为 network 也不进入降级分支（见 UC-0003/UC-0004 的集成用例）。
- **UC-09（S）构建** → `go test ./agent/ -short -run TestFeature602` 通过。

## 不在本次范围

- 不修改 FIX-598 的分类规则与 needle 列表。
- 不改变 `Retryable()` / `AllowsContextRemoval()` 的既有语义。
