# FEATURE-546 测试用例（UC-0001 ~ UC-0020）

> 主题：状态栏汇总/单轮用量可点击弹出向上浮层 + 接入 KV cache 命中率与 MTP 命中率
> 版本：v0.69.0　分支：FEATURE-546（基于 main，v0.68.7 已合并打 tag）
> 验收口径：两段用量点得开、浮层在图标**上方**；汇总浮层含历史总 KV cache 命中率与三张分析图；单轮浮层含 MTP 与 KV cache 命中率；数据缺失时**明确提示原因**而非静默空值

---

## 一、编码前调研结论（用例设计依据，全部为硬事实）

| 项 | 现状 | 位置 |
|---|---|---|
| LLM usage 解析 | 只有 `prompt_tokens`/`completion_tokens`/`total_tokens`，**未解析** `prompt_tokens_details` | `llm/client.go:147` `TokenUsage`、`:421` `usageJSON` |
| 历史用量表 | 只有 `ID/PromptTokens/CompletionTokens/TotalTokens/Timestamp`，无 cached、无计时 | `store/bolt.go:566` `TokenUsageEntry` |
| KV cache / MTP 关键字 | 全仓 Go 代码 **0 命中**（唯一命中是 i18n 英文文案里的 "speculative" 一词，误匹配） | — |
| 历史用量查询 API | **不存在** | `web/server.go:304-327` 端点清单 |
| 状态栏两段用量 | 裸 `<span class="sb-item">`，**无点击行为** | `web/static/index.html:498-499` |
| 向上弹出浮层基础设施 | ✅ 已有：`.sb-sessions{position:relative}` + `.session-menu{position:absolute;bottom:100%;right:0}` | `web/static/style.css:1930-1936` |
| 点击开合基础设施 | ✅ 已有：`bindClickToggle(el, menu, onOpen)` | `web/static/app.js:7542` |
| 折线图渲染能力 | ✅ 已有：`UI.__renderAxesChart(box, series, unit, "line")`，手写 SVG、无第三方库、颜色走 CSS class 故四主题自适应 | `web/static/ui-chart.js:264` |
| usage 持久化点 | **3 处**：流式 `agent/stream_response.go:658`（能拿到 timing）、非流式 `agent/loop.go:709`、`agent/run.go:110`（拿不到 timing） | — |

## 二、数据来源与实现约定（经 ask_user 确认）

- **KV cache 命中率** = `usage.prompt_tokens_details.cached_tokens / prompt_tokens`。需 vLLM 开 `--enable-prefix-caching` 才返回该字段；未返回时 `cached_tokens=0`。
- **MTP 命中率** = vLLM `/metrics` 的 `vllm:spec_decode_num_accepted_tokens_total / vllm:spec_decode_num_draft_tokens_total`。
- **`/metrics` 不可用时必须提示用户并给出建议**（不得静默显示空值/0%）。
- **汇总浮层维度**（用户勾选三项 + 已明确要求的命中率）：① 总量概览（历史总 prompt/completion/total、累计请求次数）② 历史总 KV cache 命中率 ③ 按天/按小时 token 用量趋势折线 ④ in/out tokens/s 历史均值趋势折线。
- **图表复用** `UI.__renderAxesChart`，不新写图表代码、不引入第三方库。
- **旧数据兼容**：`TokenUsageEntry` 新增字段带 `json` 标签，bbolt 既有条目反序列化后新字段自动为 0，无需迁移。

### 待用户确认的实现前提（本用例文档随附提问）

- **P1 `/metrics` 地址来源**：拟**从现有模型 endpoint 自动派生**（去掉 `/v1` 后缀 + `/metrics`），零配置；派生地址不可达时按约定提示用户。若希望改为显式配置项，请在确认时说明。
- **P2 速率趋势的数据来源**：`TokenUsageEntry` 需新增 `FirstTokenSec`/`OutputSec` 两个浮点字段才能算历史 tokens/s。**非流式的 2 处持久化点拿不到 timing**，拟只在流式路径写入（非流式留 0，聚合时跳过 0 值）。

---

## 三、用例清单

### A 组：后端 usage 解析（KV cache 数据链路）

#### UC-0001 非流式响应解析 cached_tokens（核心）
- **前置**：httptest 桩返回 `"usage":{"prompt_tokens":1000,"completion_tokens":50,"total_tokens":1050,"prompt_tokens_details":{"cached_tokens":800}}`
- **步骤**：`go test ./llm/ -run TestFEATURE546 -v`
- **预期**：`resp.Usage.CachedTokens == 800`，其余三字段不回归。
- **失败判据**：`CachedTokens` 为 0 或字段未被解析。

#### UC-0002 响应无 prompt_tokens_details 时不报错
- **预期**：`CachedTokens == 0`，`PromptTokens` 等正常，无 panic、无 error。

#### UC-0003 流式响应解析 cached_tokens
- **前置**：SSE 流最后一个 chunk 带 `usage.prompt_tokens_details.cached_tokens`
- **预期**：`StreamEventDone` 的 `Usage.CachedTokens` 有值。

#### UC-0004 `cached_tokens > prompt_tokens` 时命中率不越界
- **预期**：聚合命中率计算结果 clamp 在 `[0, 1]`，不出现 >100% 或 NaN/Inf。

### B 组：后端持久化与聚合

#### UC-0005 流式持久化写入 cached 与计时
- **步骤**：`go test ./agent/ -run TestFEATURE546 -v`
- **预期**：落盘条目含 `CachedTokens`、`FirstTokenSec`、`OutputSec` 且非零。

#### UC-0006 旧格式条目可读（向后兼容）
- **前置**：手工写入一条只含 5 个旧字段的 JSON 到 `token_usage` bucket
- **预期**：`ListTokenUsage()` 正常返回该条，新字段为 0，不跳过、不报错。

#### UC-0007 `GET /api/usage` 总量概览正确
- **前置**：写入 3 条已知用量
- **预期**：返回 `total_prompt`/`total_completion`/`total`/`requests` 与手算一致。

#### UC-0008 `GET /api/usage` 历史总 KV cache 命中率
- **前置**：3 条条目 cached 之和 / prompt 之和 = 已知比值
- **预期**：`hit_rate` 等于该比值（**按 token 加权，不是各条命中率的算术平均**）。

#### UC-0009 按天 / 按小时分桶正确
- **预期**：跨 2 天、3 小时的条目分别归入正确桶；`by_day` 与 `by_hour` 标签格式稳定（`MM-DD` / `HH:00`），时间升序。

#### UC-0010 速率趋势跳过零值计时
- **前置**：混合含 `OutputSec==0`（非流式）与 `>0`（流式）的条目
- **预期**：速率折线只统计 `>0` 的条目；全为 0 时该图为空态而不是 0 线。

#### UC-0011 空库时 `/api/usage` 不报错
- **预期**：HTTP 200，各计数为 0，`hit_rate` 为 0，数组为空；前端渲染空态。

### C 组：MTP 命中率抓取

#### UC-0012 Prometheus 文本解析出 MTP 命中率
- **前置**：httptest 桩返回含 `vllm:spec_decode_num_accepted_tokens_total 300` 与 `vllm:spec_decode_num_draft_tokens_total 500` 的 /metrics 文本
- **步骤**：`go test ./web/ -run TestFEATURE546 -v`
- **预期**：MTP 命中率 == 0.6。

#### UC-0013 同名指标多 label 时求和
- **前置**：`vllm:spec_decode_num_draft_tokens_total{model_name="a"} 100` 与 `{model_name="b"} 100`
- **预期**：分母取 200（对所有 label 序列求和），不重复计数、不只取第一条。

#### UC-0014 注释行 / 帮助文本不被误解析
- **前置**：/metrics 文本含 `# HELP vllm:spec_decode_num_draft_tokens_total ...` 与 `# TYPE ...`
- **预期**：`#` 开头的行被跳过，命中率不被 HELP 行里的数字污染。

#### UC-0015 /metrics 不可用时的降级提示（用户明确要求）
- **前置**：endpoint 不可达 / 返回 404 / 返回非 Prometheus 文本 / 缺 spec_decode 指标
- **预期**：接口返回**明确的不可用原因**（区分「连不上」与「无该指标」），前端显示提示文案与建议（如「需 vLLM 启动时开启 `--speculative-config`，并确认 /metrics 可访问」），**不显示 0%、不显示空白**。

#### UC-0016 /metrics 地址派生正确
- **预期**：endpoint `http://10.0.0.5:8000/v1` → 派生 `http://10.0.0.5:8000/metrics`；无 `/v1` 后缀时同样正确；派生结果不含重复斜杠。

### D 组：前端浮层（运行时用例，`--serve` 启动后浏览器验证）

#### UC-0017 点击汇总用量在图标**上方**弹出浮层
- **步骤**：点 `#sbSession` → 截图比对
- **预期**：浮层出现在图标上方（`bottom:100%` 语义），含总量概览、历史总 KV cache 命中率、两张趋势折线；再点或点页面其它处关闭。

#### UC-0018 点击单轮用量弹出单轮指标浮层
- **预期**：浮层含本轮 ↑/↓ token、↑/↓ 速率、↑/↓ 用时（与状态栏一致，不回归 FIX-545），并新增 **KV cache 命中率**与 **MTP 命中率**两项。

#### UC-0019 浮层不遮挡输入、四主题与窄屏可用
- **预期**：dark / light / light-tp / paper 四主题下浮层与折线配色正常；窄屏（`body.narrow`，`#sbSession` 本就 `display:none`）下 `#sbLast` 浮层不溢出视口、不遮挡输入框。

#### UC-0020 数据缺失时前端提示而非空值
- **预期**：vLLM 未开 prefix caching 时 KV cache 命中率显示「未启用」类提示；`/metrics` 不可用时 MTP 显示 UC-0015 的提示与建议；两者均不显示 `0%` 或空白。

---

## 四、通过标准

- A/B/C 组全部为可自动化 Go 单测，`go test ./llm/ ./store/ ./agent/ ./web/ -run TestFEATURE546 -v` 全绿。
- D 组为运行时用例，`--serve` 启动后用浏览器逐条验证并截图。
- `go build ./... && go vet ./...` 全绿；全量 `go test ./... -short` **不新增**失败项（已知 5 项既有失败见 ROADMAP v0.68.7 记录）。

## 五、预计改动面（供确认范围）

| 文件 | 改动 |
|---|---|
| `llm/client.go` | `TokenUsage`/`usageJSON` 新增 `CachedTokens`（含 `prompt_tokens_details` 嵌套解析），流式与非流式两处赋值点 |
| `store/bolt.go` | `TokenUsageEntry` 新增 `CachedTokens`/`FirstTokenSec`/`OutputSec` |
| `agent/stream_response.go` | 持久化点写入 cached 与计时 |
| `agent/loop.go` / `agent/run.go` | 非流式持久化点补 cached（计时留 0） |
| `agent/events.go` | `token_iter` 新增 `cached` / `cache_rate` meta 键 |
| `web/server.go` | 新增 `GET /api/usage`（历史聚合）与 MTP 抓取（Prometheus 解析 + 降级原因） |
| `web/static/index.html` | `#sbSession`/`#sbLast` 各包一层 wrap + 两个浮层容器 |
| `web/static/style.css` | 两个浮层样式（复用 `.session-menu` 的 `bottom:100%` 语义） |
| `web/static/app.js` | 点击开合 + 拉取 `/api/usage` + 复用 `UI.__renderAxesChart` 渲染折线 |
| `i18n/` | 浮层所有文案中英双语 |
