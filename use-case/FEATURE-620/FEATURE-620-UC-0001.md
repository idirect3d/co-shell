# FEATURE-620 验收测试用例（UC-0001）：P0 —— 思考字段参数化 + auto union 兜底

> 权威依据：`ROADMAP.md` v0.103.0 / FEATURE-620；分支 **FEATURE-620**，版本 **v0.103.0**（FEATURE 走 minor 递增）。
> 设计文档：`/Users/direct3d/.co-shell/agents/admin-llm/output/co-shell-model-wizard-design.md`（admin-llm，v1 草案）
> 相关代码：`llm/client.go` —— 响应体 `responseMessageJSON`(:439)、SSE `parseSSELine` delta(:1311) + 赋值(:1341)、`parseResponseChoices`(:612)、**请求侧 `chatMessageJSON`(:332，用于 L536-563 回传)**。

> 问题（用户实测，2026-10-07）：本地 vLLM 模型 `deepseek-v4-flash-vision-exp @192.168.3.39:8899` **思考不显示**。
> 根因：vLLM 官方已将思考字段由 `reasoning_content` 改名为 `reasoning`（官方文档：「reasoning used to be called reasoning_content. To migrate, directly replace reasoning_content with reasoning.」），并警告客户端会「silently read an empty reasoning_content, even when reasoning is populated」；而 co-shell 的 `llm/client.go` **只解析 `reasoning_content`**，思考被静默丢弃、UI 无思考块。

> 已确认决策（用户确认，不可改动）：
> ① 版本归属 **v0.103.0**，任务号 **FEATURE-620**，分支 **FEATURE-620** 单分支一支到底，**P0 为第一步**；
> ② P0 范围 = **响应侧 3 处 + 请求侧第 4 处**（`chatMessageJSON.ReasoningContent`，admin-llm 设计文档漏列，本会话修正 1）；
> ③ 默认策略 = **显式声明优先、auto 兜底**；`ReasoningField` 为空即 auto，**严格保持现状行为**。

> 本次涉及的参数（可调但需在用例中保持一致）：auto 兜底时取值顺序固定 **`reasoning_content` 优先**；同一响应两者皆有 → 去重，保留 `reasoning_content`。

## 用例清单

| 编号 | 名称 | 类型 | 层面 |
|---|---|---|---|
| P0-001 | `ReasoningField=""`(auto) + 响应体仅有 `reasoning` → 思考被解析出来（止血主用例） | 运行时（Go, mock HTTP） | 非流式 |
| P0-002 | `ReasoningField=""`(auto) + 响应体仅有 `reasoning_content` → 行为与改动前逐字一致 | 运行时回归 | 非流式 |
| P0-003 | `ReasoningField=""`(auto) + 响应体两者皆有 → 取 `reasoning_content`，不拼接、不重复 | 运行时 | 非流式 |
| P0-004 | `ReasoningField="reasoning"` 显式 → 只取 `reasoning`，即使 `reasoning_content` 有值也忽略 | 运行时 | 非流式 |
| P0-005 | `ReasoningField="reasoning_content"` 显式 → 只取 `reasoning_content`，忽略 `reasoning` | 运行时 | 非流式 |
| P0-006 | `ReasoningField="none"` → 不产出任何思考（即使两端都有值） | 运行时 | 非流式 |
| P0-007 | 流式 delta 仅有 `reasoning` → 发出 `StreamEventReasoning`（**改前必失败，为本次止血核心**） | 运行时（Go, mock SSE） | 流式 |
| P0-008 | 流式 delta 仅有 `reasoning_content` → 仍发 `StreamEventReasoning`（回归） | 运行时回归 | 流式 |
| P0-009 | 流式 `ReasoningField="reasoning"` 显式 → 只发 `reasoning` 事件，不掺入 `reasoning_content` | 运行时（Go, mock SSE） | 流式 |
| P0-010 | 请求侧：`ReasoningField="reasoning"` 时 assistant 历史消息回传字段名为 `reasoning` | 运行时（Go, mock HTTP 断言请求体） | 请求侧 |
| P0-011 | 请求侧：`ReasoningField=""/auto` 时回传字段名仍为 `reasoning_content`（零兼容风险） | 运行时回归 | 请求侧 |
| P0-012 | 请求侧：`ReasoningField="none"` 时不回传思考字段 | 运行时 | 请求侧 |
| P0-013 | 真实端点：本地 vLLM 显示思考块（端到端） | 手动/端到端 | E2E |
| P0-014 | `go build ./... && go vet ./...` 一次通过；`go test ./llm/ -run P0` 全绿 | 构建/测试 | 收尾 |

## 实现要点（预期，供编码时对齐）

1. 解析入口集中化：新增「按 `ReasoningField` 取值」的单一函数（如 `pickReasoning(field, reasoningContent, reasoning string) string`），**union 兜底写在这一处**，避免三处各自实现导致行为漂移；
   - `field == ""`(auto)：`reasoningContent != "" ? reasoningContent : reasoning`
   - `field == "reasoning_content"`：取 `reasoningContent`；`field == "reasoning"`：取 `reasoning`；`field == "none"`：返回空
2. 结构体加字段：`responseMessageJSON`、`streamEventJSON`、`parseSSELine` 内匿名 delta 结构体均增加 `Reasoning string \`json:"reasoning,omitempty"\``；
3. 请求侧：`chatMessageJSON` 的思考回传字段名需可切换（`reasoning_content` / `reasoning`），由客户端持有的 `ReasoningField` 决定；auto 时保持 `reasoning_content`；
4. `parseResponseChoices` 改为接收 `ReasoningField`（或改为方法）以完成取值；
5. **不引入新依赖**；`llm/client.go` 已达 1778 行，本次只做最小外科修改，**不顺带重构**（文件超 500 行属既有状况）。

## 用例详情

> 工程约定：全部用 `httptest.NewServer` 起 mock 端点（沿用 `llm/feature347_test.go:102`、`llm/responses_client_test.go:233` 的既有模式），
> 非流式返回 `choices[0].message`，流式返回 `text/event-stream` 的 `data: {...}` 分片，最后 `data: [DONE]`。

### 非流式组（P0-001 ~ P0-006）

- **P0-001（止血主用例，改前必失败）**
  - 给定：mock 返回 `{"choices":[{"message":{"role":"assistant","content":"hi","reasoning":"let me think"}}]}`；模型配置 `ReasoningField=""`。
  - 断言：返回的 reasoning 为 `let me think`（改前为 `""` → 失败）。
  - 说明：这是本次排障现象的**最小复现**，P0 完成前必须红。
- **P0-002（兼容性回归）**
  - 给定：mock 返回 `reasoning_content":"let me think"`，无 `reasoning`；`ReasoningField=""`。
  - 断言：reasoning 为 `let me think`，且与改动前 `git stash` 对照完全一致。
- **P0-003（两者皆有的去重）**
  - 给定：mock 同时返回 `reasoning_content":"A"` 与 `reasoning":"B"`；`ReasoningField=""`。
  - 断言：结果为 `A`（`reasoning_content` 优先），且**不是** `AB`/`BA`/`A\nB`。
- **P0-004 / P0-005（显式优先）**
  - 给定：同一响应两端都有值（`reasoning_content="A"`、`reasoning="B"`）。
  - 断言：`ReasoningField="reasoning"` → `B`；`ReasoningField="reasoning_content"` → `A`。
  - 说明：验证「显式声明优先于 auto 嗅探」这一已确认决策。
- **P0-006（none 关闭）**
  - 给定：两端都有值；`ReasoningField="none"`。
  - 断言：reasoning 为空字符串，且不产出思考事件。

### 流式组（P0-007 ~ P0-009）

- **P0-007（止血核心，改前必失败）**
  - 给定：SSE 分片依次为 `{"choices":[{"delta":{"reasoning":"step1"}}]}`、`{"choices":[{"delta":{"reasoning":"step2"}}]}`、`{"choices":[{"delta":{"content":"ans"}}]}`、`[DONE]`；`ReasoningField=""`。
  - 断言：事件流中出现 **2 个** `StreamEventReasoning`，内容依次为 `step1`、`step2`；随后 1 个 `StreamEventContent`（`ans`）。
  - 说明：改前 reasoning 事件数为 0 → 必须红。
- **P0-008（流式回归）**
  - 给定：同上但分片用 `reasoning_content`；`ReasoningField=""`。
  - 断言：同样得到 2 个 `StreamEventReasoning`（回归不变）。
- **P0-009（流式显式 + 混合分片）**
  - 给定：分片 `delta.reasoning_content="X"` 与 `delta.reasoning="Y"` 交替出现；`ReasoningField="reasoning"`。
  - 断言：仅 `Y` 系列出现；`X` 系列被忽略，且不产生重复/交错错乱。

### 请求侧组（P0-010 ~ P0-012）

> 做法：mock 服务端读取请求体 JSON，断言 `messages[]` 中 assistant 消息的**键名**。

- **P0-010**
  - 给定：带 reasoning 的 assistant 历史消息 + `ReasoningField="reasoning"`。
  - 断言：请求体该消息含 `"reasoning"` 键、**不含** `"reasoning_content"` 键。
- **P0-011（零兼容风险）**
  - 给定：同上但 `ReasoningField=""`（auto）。
  - 断言：请求体含 `"reasoning_content"` 键 —— 与改动前完全一致。
- **P0-012**
  - 给定：同上但 `ReasoningField="none"`。
  - 断言：请求体两个键都不出现（`omitempty` 生效）。

### 端到端（P0-013 ~ P0-014）

- **P0-013（用户可感知的验收）**
  - 步骤：在隔离实例（`--serve`，独立 workdir）配置本地 vLLM 端点 `192.168.3.39:8899` + 模型 `deepseek-v4-flash-vision-exp`，发一条需要思考的问题。
  - 断言：Web UI 出现思考块且内容非空。
  - 说明：**这是本任务面向用户的最终验收点**；若该端点不可达，需用户提供替代可达端点或改用 mock 证据说明，不得静默跳过。
- **P0-014（收尾）**
  - 断言：`go build ./... && go vet ./...` 一次通过（禁止分开执行）；`go test ./llm/ -short` 全绿；`go test ./... -short` 失败集与 main 基线逐项一致（无本次引入的新失败）。
