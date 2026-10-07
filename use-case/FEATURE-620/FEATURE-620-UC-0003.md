# FEATURE-620 验收测试用例（UC-0003）：P2 —— 协议解耦 + vllm-self-hosted 模板 + 内嵌 CoT 剥离

> 权威依据：`ROADMAP.md` v0.103.0 / FEATURE-620；分支 **FEATURE-620**，版本 **v0.103.0**。
> 设计文档：`/Users/direct3d/.co-shell/agents/admin-llm/output/co-shell-model-wizard-design.md` §2、§6、§7、§8。
> 相关代码：`llm/thinking_adapter.go`（`GetThinkingAdapter`:36、各 provider adapter:77-215）、`llm/client.go`（解析与请求侧字段）、`config/model_template.go`（模板表 :151-317、模板 ID 列表 :346-348、`GetTemplate`/`CreateFromTemplate` 附近 :664）。

> 已确认决策（用户确认，不可改动）：
> ① **新增 `vllm-self-hosted` 模板**（采纳 admin-llm 建议；**不动** `custom-openai-compatible` / `lmstudio-local` / `ollama-local`，它们复用同一套探测）；
> ② **厂商模板保持不动**（deepseek-official / qwen-official / qwen3.8 / xiaomi-mimo / kimi-official / zhipu-glm / openai-official 行为与内容不变，向后兼容）；
> ③ `ThinkingProtocol` 与 `provider` **解耦**：`ThinkingProtocol` 字段已在 P1 落地并可由探测写入，P2 完成 **adapter 选择逻辑**（优先级：`ThinkingProtocol` > `provider` 默认）；
> ④ **`inline` 剥离默认关闭**（修正 3）：仅当用户显式启用 `ReasoningTags` 时才剥离，**探测不自动开启**。

> 本次涉及的参数（可调但需在用例中保持一致）：`ReasoningField` 取值集合 `""(auto) / reasoning_content / reasoning / inline / none`；`ToolCallStyle` 取值集合 `native / text / none`；默认内置标签集建议 `["thinking", "reasoning"]`。

## 用例清单

| 编号 | 名称 | 类型 | 层面 |
|---|---|---|---|
| P2-001 | `ThinkingProtocol=""` → 沿用 provider 默认 adapter（**回归**） | 运行时（Go） | 协议解耦 |
| P2-002 | `ThinkingProtocol="qwen"` 且 `provider="openai-compatible"` → 用 qwen adapter（解耦核心） | 运行时（Go） | 协议解耦 |
| P2-003 | 非法/未知 `ThinkingProtocol` → 降级为 auto（provider 默认），不报错、不致请求失败 | 运行时（Go） | 协议解耦健壮性 |
| P2-004 | 解耦后厂商模型请求体逐字节不变（deepseek/qwen/kimi/glm 回归） | 运行时回归 | 兼容性 |
| P2-005 | `vllm-self-hosted` 模板存在且字段符合设计（`Provider=openai-compatible`、`Endpoint` 空、`Capabilities` 全 false） | 运行时（Go） | 模板 |
| P2-006 | 从 `vllm-self-hosted` 模板创建模型：不回填 endpoint 时向导给出提示，不静默生成指向空地址的配置 | 运行时（Go/UI） | 模板 |
| P2-007 | 现有 7 个厂商模板 + 3 个兼容模板的内容逐字段不变（**模板回归**） | 运行时回归 | 兼容性 |
| P2-008 | `ReasoningField="inline"` + `ReasoningTags=["thinking"]` → content 中 `<thinking>…</thinking>` 被剥离并作为思考输出 | 运行时（Go） | inline |
| P2-009 | 默认（`ReasoningField≠inline`）→ **不剥离**，`content` 原样保留 | 运行时回归 | inline |
| P2-010 | inline 剥离：标签跨多个流式分片时不丢内容、不误切正常文本 | 运行时（Go, mock SSE） | inline 健壮性 |
| P2-011 | inline 剥离：未闭合标签（流被截断）→ 兜底不丢内容（原样输出，不吞正文） | 运行时（Go, mock SSE） | inline 健壮性 |
| P2-012 | `ToolCallStyle` 探测结果可落盘并被读取（native/text/none） | 运行时（Go） | 探测落盘 |
| P2-013 | 端到端：本地 vLLM 走「探测 → 落盘 → 显示思考」全链路（P0+P1+P2 联合验收） | 手动/端到端 | E2E |
| P2-014 | `go build ./... && go vet ./...` 一次通过；`go test ./... -short` 无本次引入的新失败 | 构建/测试 | 收尾 |

## 实现要点（预期，供编码时对齐）

1. **adapter 选择优先级**：新增单一入口（如 `GetThinkingAdapterFor(protocol, provider string)`）：`protocol` 非空 → 按 registry key 取；为空 → 退回现有 `GetThinkingAdapter(provider)` 逻辑。**保留** `GetThinkingAdapter` 原签名与行为，避免影响既有调用点；
2. **inline 剥离放在解析层**（`llm/client.go`），与 union 兜底同一处收敛，避免分散实现；剥离结果走既有的 `StreamEventReasoning` 事件，正文走 `StreamEventContent`；
   - 跨分片需**状态机**（记录是否处于标签内），不能对单个 chunk 做正则替换；
   - 未闭合标签必须兜底：把已缓冲内容当作正文输出，**绝不吞正文**；
3. **`vllm-self-hosted` 模板**：`ID="vllm-self-hosted"`、`Provider="openai-compatible"`、`Endpoint=""`（用户必填）、`Capabilities` 全 false、描述写明「需服务端按模型配置 `--reasoning-parser` / `--tool-call-parser`」；需同步加到模板顺序列表（`config/model_template.go:346` 附近）与 i18n 文案；
4. **不新增长度依赖**；`go.mod` 不变。

## 用例详情

### 协议解耦组（P2-001 ~ P2-004）

- **P2-001（回归，最高优先）**
  - 步骤：`ThinkingProtocol=""`，provider 分别取 `deepseek` / `qwen` / `kimi` / `zhipu` 等。
  - 断言：`BuildAdditions` 输出与改动前（`git stash` 对照）**逐键逐值一致**。
- **P2-002（解耦核心）**
  - 步骤：`ThinkingProtocol="qwen"`、`provider="openai-compatible"`，请求 thinking enabled。
  - 断言：请求体含 `extra_body.chat_template_kwargs.enable_thinking=true`（qwen 协议产物），**而非** fallback adapter 的产物。
  - 说明：这正是 vLLM 上「一台机器跑任意模型、provider 名不可靠」场景的验收点。
- **P2-003**：`ThinkingProtocol="not-a-protocol"` → 断言不 panic、不返回错误、行为等同 auto（fallback），并记录一条 warn 日志。
- **P2-004（厂商模型回归）**
  - 步骤：对 7 个厂商模板各建一个 `ModelConfig`，生成完整请求体。
  - 断言：请求体 JSON 与改动前**逐字节一致**（键顺序与内容均不变）。

### 模板组（P2-005 ~ P2-007）

- **P2-005**：断言 `vllm-self-hosted` 存在；`Provider=="openai-compatible"`；`Endpoint==""`；`Capabilities == {false,false,false,false}`；`Description` 含 `reasoning-parser` 与 `tool-call-parser` 字样；出现在模板顺序列表中。
- **P2-006**：从该模板启动 Web 向导 → 断言端点步骤为空且必填（不能以空 endpoint 直接提交，也不静默写入空地址）。
- **P2-007（模板回归）**
  - 断言：`deepseek-official` / `qwen-official` / `qwen3.8` / `xiaomi-mimo` / `kimi-official` / `zhipu-glm` / `openai-official` / `lmstudio-local` / `ollama-local` / `custom-openai-compatible` 共 10 个模板的关键字段（ID/Provider/Endpoint/DefaultModel/Capabilities/Priority/DefaultParams）与改动前逐字段一致。

### inline 剥离组（P2-008 ~ P2-011）

- **P2-008**
  - 步骤：非流式响应 `content = "<thinking>推理过程</thinking>答案"`；`ReasoningField="inline"`、`ReasoningTags=["thinking"]`。
  - 断言：思考部分为 `推理过程`，正文为 `答案`（标签与内容都不残留在正文里）。
- **P2-009（默认关闭，回归）**
  - 给定：相同响应但 `ReasoningField=""`(auto)。
  - 断言：正文**原样**为 `<thinking>推理过程</thinking>答案`，不剥离、不丢字符。
- **P2-010（跨分片状态机）**
  - 步骤：SSE 分片切成 `<thin` + `king>推理` + `过程</thin` + `king>答案`（标签与内容均被切断）。
  - 断言：思考为 `推理过程`，正文为 `答案`；不存在把 `<thin` 当正文输出的情况。
- **P2-011（未闭合兜底）**
  - 步骤：分片为 `<thinking>推理` 后流结束（无闭合标签）。
  - 断言：正文包含 `推理`（**不吞正文**）；行为有明确兜底且不 panic。
  - 说明：对应设计文档 §8「inline 剥离有误切风险」的防护验收。

### 落盘与端到端（P2-012 ~ P2-014）

- **P2-012**：`ToolCallStyle` 探测为 `native` / `text` / `none` 三值分别写入 `ModelConfig` 并重载后读回一致（`omitempty` 下空值不落盘）。
- **P2-013（联合验收，最终交付点）**
  - 步骤：隔离实例（`--serve`，独立 workdir）→ 向导新增本地 vLLM 端点 → 走到「探测与能力」→ 确认探测值（思考字段名应为 `reasoning`）→ 保存 → 发起一次需要思考的提问。
  - 断言：① 探测表格正确显示字段名；② 保存后 `config.json` 含 `reasoning_field`；③ Web UI 出现非空思考块。
  - 说明：等价于 P0-013 的完整链路版本；**这是本任务面向用户的最终验收点**。
- **P2-014**：`go build ./... && go vet ./...` 一次通过（禁止分开执行）；`go test ./... -short` 的失败集与 main 基线**逐项一致**。
