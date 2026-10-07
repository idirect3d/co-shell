# FEATURE-620 验收测试用例（UC-0002）：P1 —— 探测能力 + 「探测与能力」步骤

> 权威依据：`ROADMAP.md` v0.103.0 / FEATURE-620；分支 **FEATURE-620**，版本 **v0.103.0**。
> 设计文档：`/Users/direct3d/.co-shell/agents/admin-llm/output/co-shell-model-wizard-design.md` §3–§5。
> 相关代码：`config/model_template.go`（`ModelConfig`:80 / `ModelTemplate`:48 / `ModelCapability`:40）、`cmd/model.go`（`detectModelCapabilities`:1383、`wizardSelectCapabilities`:1467）、`cmd/model_web_wizard.go`（`WebWizardData`:93）、`web/static/app.js`（向导渲染 :8313 起）、`i18n/keys.go` + `i18n/zh.go` + `i18n/en.go`。

> 现状缺口：向导 `capabilities` 步骤只探 3 个布尔（vision / tool_call / thinking），**探不出「思考放在哪个字段」**；`ModelConfig` 也没有存「响应侧字段映射」的位置 → 探测结果无处落盘。

> 已确认决策（用户确认，不可改动）：
> ① 探测**放在向导内**（合并进 `capabilities` 步骤，**步骤序列不变长**）；模型列表页独立「探测」按钮留后续，本轮不做；
> ② 本轮采纳**修正 2**：P1 探测采用**多协议遍历**（deepseek/qwen/zhipu/minimax/kimi/openai-compatible 的 enable 参数依次试，命中即记协议 key），并把命中的 key **直接写入 `ThinkingProtocol`**（新增字段，零兼容风险）；P2 的「解耦」只剩 adapter 选择逻辑；
> ③ `ProbeLevel` 默认 **light**；
> ④ 采纳**修正 3**：`inline` 剥离**探测不自动开启**。

> 本次涉及的参数（可调但需在用例中保持一致）：单项探测超时 **15s**；`ProbeLevel` = `light`（仅 1 次请求：探字段名）/ `full`（全项）/ `off`（不发请求）；探测请求**强制写 LLM 交互日志**。

## 用例清单

| 编号 | 名称 | 类型 | 层面 |
|---|---|---|---|
| P1-001 | `ModelConfig`/`ModelTemplate` 支持 4 新字段的 JSON 往返（含 `omitempty`） | 运行时（Go） | 数据模型 |
| P1-002 | 旧配置文件（无新字段）加载 → 零值，行为与改动前一致（兼容性回归） | 运行时回归 | 数据模型 |
| P1-003 | 探测字段名：mock delta 只出现 `reasoning` → `ReasoningField="reasoning"` | 运行时（Go, mock SSE） | 探测 |
| P1-004 | 探测字段名：mock delta 只出现 `reasoning_content` → `ReasoningField="reasoning_content"` | 运行时（Go, mock SSE） | 探测 |
| P1-005 | 探测字段名：mock delta 两者皆无 → `ReasoningField="none"` | 运行时（Go, mock SSE） | 探测 |
| P1-006 | 探测「思考默认开关」：**不传**任何 thinking 参数仍返回思考 → `ReasoningDefault="on"` | 运行时（Go, mock SSE） | 探测 |
| P1-007 | 探测「思考默认开关」：不传参数且无思考 → `ReasoningDefault="off"` | 运行时（Go, mock SSE） | 探测 |
| P1-008 | 多协议遍历：provider=`openai-compatible` 但仅 qwen 协议能开思考 → `ThinkingProtocol="qwen"`（修正 2 核心） | 运行时（Go, mock SSE） | 探测 |
| P1-009 | 多协议遍历：遍历顺序固定且命中即停（请求次数 = 命中前的协议数 + 1） | 运行时（Go） | 探测 |
| P1-010 | 单项探测超时（>15s）只降级该项，不阻断后续项与整个向导 | 运行时（Go, 慢 mock） | 探测健壮性 |
| P1-011 | `ProbeLevel=off` → 零探测请求；`light` → 恰好 1 次请求；`full` → 请求数 > 1 | 运行时（Go, 计数 mock） | 探测分档 |
| P1-012 | 探测请求写入 LLM 交互日志（`log.SetLLMInteractionEnabled` 生效，含原始分片） | 运行时（Go） | 可观测性 |
| P1-013 | 探测结果**不回写** `inline`（`ReasoningTags` 不被自动置位，修正 3） | 运行时 | 探测约束 |
| P1-014 | `WebWizardData` 新字段往返存活；**用户覆写值优先于探测值** | 运行时（Go） | 向导状态 |
| P1-015 | 向导步骤序列不变长（`capabilities` → 「探测与能力」，仍是 9 步） | 静态断言 | 向导结构 |
| P1-016 | Web 前端渲染「探测与能力」表格：探测值列 + 可覆写下拉列 + 重新探测按钮 | 运行时（浏览器） | 前端 UI |
| P1-017 | 前端「重新探测」按钮触发重探并刷新表格（复用既有 `Detect` 标志） | 运行时（浏览器） | 前端 UI |
| P1-018 | i18n：新增文案 key 在 zh/en **两侧都有**且非空（对齐检查） | 静态/运行时（Go） | i18n |
| P1-019 | 覆写后的 `ReasoningField` 随模型保存并落盘，重载后仍生效 | 运行时（Go） | 持久化 |
| P1-020 | `go build ./... && go vet ./...` 一次通过；`go test ./... -short` 无本次引入的新失败 | 构建/测试 | 收尾 |

## 实现要点（预期，供编码时对齐）

1. **新文件 `llm/reasoning_probe.go`**：探测逻辑自包含在此，避免 `cmd/model.go`（已 2302 行）与 `llm/client.go`（已 1778 行）继续膨胀；
   - 对外暴露单一入口（如 `ProbeReasoning(ctx, endpoint, apiKey, model, opts) *DetectedProfile`）与 `DetectedProfile` 结构（`ReasoningField` / `ReasoningDefault` / `ToolCallStyle` / `ReasoningTags` / `MaxModelLen` / `Engine` / `ThinkingProtocol`）；
   - 流式探测**只读前 N 个 chunk** 即主动取消（`light` 档只发 1 次请求，应对慢/并发差的端点）；
2. `config/model_template.go`：`ModelConfig` 与 `ModelTemplate` **同步**新增 4 字段（`ThinkingProtocol` / `ReasoningField` / `ToolCallStyle` / `ReasoningTags`），全部 `omitempty`；
3. `cmd/model.go`：`detectModelCapabilities` 扩展为「探测与能力」，把 vision/tool_call/thinking 与新增项一并产出；`wizardSelectCapabilities` 改为结构化展示 + 可覆写（REPL 版走编号菜单，Web 版走表格）；
4. `cmd/model_web_wizard.go`：`WebWizardData` 增加 §3.2 的 6 个字段；覆写值在 `Submit` 时写入 `ModelConfig`；
5. `web/static/app.js` + `i18n/*`：表格渲染与中英文案；
6. **不新增长度依赖**；`go.mod` 不变。

## 用例详情

> 探测相关用例统一用 `httptest.NewServer` 起 mock 端点，mock 端可按「第 N 次请求」返回不同行为，并统计请求数与请求体（用于断言协议遍历顺序）。

### 数据模型组（P1-001 ~ P1-002）

- **P1-001**
  - 步骤：构造带 4 个新字段的 `ModelConfig` / `ModelTemplate` → `json.Marshal` → `json.Unmarshal`。
  - 断言：4 字段逐字段相等；全零值时 marshal 结果**不含**这 4 个键（`omitempty` 生效）。
- **P1-002（兼容性回归，最高优先）**
  - 步骤：用**改动前**的 `config.json` 副本（`git show main:config.json`）加载。
  - 断言：加载成功、无报错；新字段为零值；`Capabilities` 等既有字段与改动前逐字段一致。

### 探测组（P1-003 ~ P1-013）

- **P1-003 / P1-004 / P1-005（字段名探测）**
  - 步骤：mock 返回开思考后的流式分片，分别只在 `delta.reasoning` / `delta.reasoning_content` / 都不在 出现思考内容；调用探测。
  - 断言：`DetectedProfile.ReasoningField` 依次为 `reasoning` / `reasoning_content` / `none`。
- **P1-006 / P1-007（默认开关探测）**
  - 步骤：mock 记录请求体；**不携带任何 thinking 参数**发起探测。
  - 断言：返回思考 → `ReasoningDefault="on"`；不返回思考 → `"off"`。且两种情况下请求体**均不含** thinking / reasoning_effort 类参数。
- **P1-008（修正 2 核心：多协议遍历）**
  - 步骤：mock 仅当请求体含 `chat_template_kwargs.enable_thinking=true`（qwen 协议）时才返回思考；配置的 `provider="openai-compatible"`。
  - 断言：`ThinkingProtocol="qwen"`；后续 P2 阶段该值即用于 adapter 选择。
  - 说明：**这是修正 2 的验收点** —— 若不做多协议遍历，此处会误判为「不支持思考」。
- **P1-009（遍历顺序与提前终止）**
  - 步骤：mock 在第 2 个协议命中。
  - 断言：请求次数 = 2；第 1 个请求用 deepseek 协议参数、第 2 个用 qwen 协议参数（顺序固定、命中即停，不做多余请求）。
- **P1-010（超时降级）**
  - 步骤：mock 对「字段名探测」sleep 20s，对后续项正常响应。
  - 断言：该单项在 ~15s 内被判失败（`ReasoningField` 保持空/unknown），**后续项仍被探测**，向导正常走完不报错。
- **P1-011（ProbeLevel 分档）**
  - 断言：`off` → 请求数 0；`light` → 请求数 1；`full` → 请求数 > 1。默认档为 `light`。
- **P1-012（可观测性）**
  - 步骤：开启 LLM 交互日志后执行探测，读取日志文件。
  - 断言：日志含探测请求的 URL/请求体与响应原始分片；**不含 API Key 明文**。
- **P1-013（修正 3：不自动开 inline）**
  - 步骤：mock 返回 `ReasoningField=none` 且 `content` 含 `<thinking>…</thinking>`。
  - 断言：探测产出**建议**（UI 可展示命中标签），但写回配置的 `ReasoningTags` **为空/未启用** —— 不自动开启 inline 剥离。

### 向导与前端组（P1-014 ~ P1-018）

- **P1-014**：`WebWizardData` 序列化往返后 6 个新字段存活；当用户覆写 `ReasoningField="reasoning"` 而探测值为 `reasoning_content` 时，**以用户值为准**。
- **P1-015**：断言向导步骤序列仍为 `template → endpoint → api_key → model_name → [探测与能力] → model_id → priority → max_model_len → enabled`（**9 步，未加长**）。
- **P1-016 / P1-017（浏览器运行时）**
  - 步骤：`--serve` 起隔离实例 → 打开向导走到「探测与能力」步骤。
  - 断言：表格含设计文档 §3.3 的 8 行；「思考字段名」行有 `[auto / reasoning_content / reasoning / inline / none]` 下拉；每行有「重新探测」按钮；点击后发起重探并**原地刷新**表格值（不跳步、不丢已填字段）。
- **P1-018**：断言新增的 i18n key 在 `zh.go` 与 `en.go` 中同时存在且翻译非空（防漏译）。

### 持久化（P1-019）

- 步骤：向导中把 `ReasoningField` 覆写为 `reasoning` → 保存 → 重新加载配置。
- 断言：`config.json` 中出现 `"reasoning_field": "reasoning"`；重载后该值仍在；随后一次 LLM 调用按该字段解析。

### 收尾（P1-020）

- `go build ./... && go vet ./...` 一次通过（禁止分开执行）；`go test ./... -short` 的失败集与 main 基线**逐项一致**（不引入新失败）。
