# Web 消息持久化与恢复设计说明书

> **文档定位**：Web UI 会话消息「怎么存、怎么取、怎么恢复、怎么渲染」的单一事实源（Single Source of Truth）。
> **适用对象**：co-shell 维护者；修改 `web/`（含 `web/static/app.js`）、`store/eventstream.go`、`agent` 事件发射逻辑前必读。
> **关联文档**：`docs/web-session-resilience.md`（会话韧性 / 交互请求持久化）、`docs/output-architecture.md`（输出架构）、`docs/web-ui-design.md`（Web UI 设计）。
> **本次修订**：FIX-575 —— 修复「重启/刷新恢复会话时工具调用消息随机丢失」，并把「块边界必须显式可判定」上升为设计不变量。

---

## 1. 目标与不变量（设计契约）

### 1.1 目标

| 编号 | 目标 | 说明 |
|------|------|------|
| G1 | **恢复等价** | 刷新页面 / 重连 / 重启进程后回放出的消息流，必须与实时流式渲染出的消息流**结构等价**（块数量、顺序、类型、标题语义一致） |
| G2 | **无界会话可用** | 后端不在内存中保留历史；浏览器可对任意长度的会话分页回看 |
| G3 | **失败不阻塞** | 持久化失败绝不能阻塞或中断实时流 |
| G4 | **有界存储** | 单会话事件流有上限，防止数据库无限增长 |
| G5 | **概览不失真** | 顶部「消息可视化条」显示的块颜色/数量必须与正文消息流中的块一致 |

### 1.2 不变量（Invariants，改动前必须保持）

| 编号 | 不变量 | 违背后的现象 |
|------|--------|--------------|
| **I1** | 一个消息组（`meta.msg_index` 相同的连续事件）绝不跨页拆分 | 消息半截显示、重复出现 |
| **I2** | **回放与实时的块结构必须由同一套规则决定** | 恢复后块数量/顺序与实时不一致（本 bug） |
| **I3** | **块起点必须显式可判定**，禁止依赖渲染游标状态（如 `curTool.raw`/`hasResult`）来推断 | 状态不同则判定不同 → "随机"丢失 |
| **I4** | 每个渲染出的块在「消息可视化条」上有且只有一条线 | 概览与正文不一致 |
| **I5** | 事件写入后 `seq` 单调递增且不冲突；事件不因写入逻辑被覆盖 | 历史出现空洞 |
| **I6** | `meta.msg_index` 只增长，同一消息组在键空间中连续 | 分页/裁剪错位 |

---

## 2. 角色与总体架构

```
                       ┌─────────────────────────── 浏览器 (web/static/app.js) ───────────────────────────┐
                       │  renderEvent()  块游标: curLLM/curThinking/curTool/curSUP/curREPL/lastBlock     │
                       │  msgViz*        消息可视化条（1 线 = 1 块）                                     │
                       │  renderHistory() 回放页；loadOlderHistory() 上滚加载更早页                       │
                       └───────────────▲───────────────────────────────────────────┬──────────────────────┘
                                       │ WebSocket: event / history              │ history_get(count,before)
                                       │                                         ▼
┌── agent 事件发射 ──┐   ┌────────────────────────── web/ ──────────────────────────────┐
│ StreamEvent        │──▶│ WebRenderer.Render()                                        │
│ (EventContentChunk │   │  ① 注入 meta.msg_index                                      │
│  EventToolCallStr… │   │  ② persistPendingUserEcho()  ← 本轮用户消息回执             │
│  EventToolCall     │   │  ③ eventCoalescer.Add(ev) → 合并成"可回放"的完整块事件       │
│  EventTokenIter    │   │  ④ marshalEvent → Store.AppendEvent(sessionID, JSON)       │
│  EventDone …)      │   │  ⑤ srv.sendEvent(ev)  ← 实时发给浏览器（原样，不合并）      │
└────────────────────┘   └───────────────┬─────────────────────────────────────────────┘
                                         ▼
                      ┌──────────── store/eventstream.go (bbolt) ─────────────┐
                      │ bucket "eventstream"                                  │
                      │ key = sessionID + 0x00 + %08d(seq)                    │
                      │ value = eventJSON（与发给浏览器同构）                  │
                      │ AppendEvent / LoadEvents / LoadEventsByGroups /       │
                      │ trimEventStream(≤5000) / DeleteEventsAfter            │
                      └───────────────────────────────────────────────────────┘
```

**关键设计取舍**：实时通道（⑤）与持久化通道（③④）**是两条不同的数据形态**——
- 实时通道是**增量分片**（`content_chunk`/`thinking_chunk`/`tool_call_stream`），依赖浏览器端的流式游标拼接；
- 持久化通道是**合并后的完整块**（`content`/`thinking`/`tool_call`），因为回放时浏览器没有流式游标。

> ⚠️ 这就是 I2 的由来：两条通道的**块结构必须严格等价**，否则"刷新前后不一样"。
> FIX-575 修复的正是这个等价性被破坏的问题（见 §7）。

---

## 3. 数据结构

### 3.1 事件流存储（`store/eventstream.go`）

| 项 | 值 / 说明 |
|----|-----------|
| bucket | `eventstream` |
| key | `sessionID + "\x00" + fmt.Sprintf("%08d", seq)`；8 位零填充使 bbolt 的字节序即自然数字序 |
| 分隔符 | `eventStreamSep = "\x00"`（session ID 为 `sess-YYYYMMDDhhmmss-xxxxxxxx`，不含该字符） |
| 上限 | `maxEventStreamEntries = 5000`（每会话事件数，超出时从最旧开始按**消息组边界**裁剪） |
| seq 语义 | 会话内单调递增，从 1 开始；用于分页游标 `before` |

### 3.2 事件线格式（`eventJSON`，`web/session.go` / `web/eventcoalesce.go`）

```json
{ "type":"tool_call", "text":"…", "chan":"tool", "level":"warning", "ts":1759312345678,
  "meta": { "msg_index":"42", "phase":"input", "tool_summary":"{…}", "diff_lines":"[…]" } }
```

- `type`：见 `agent/events.go`（`content`/`thinking`/`tool_call`/`token_iter`/`token_task`/`done`/`error`/`ui_text`…）。
- `ts`：块**开始**时刻（合并块取首片时间；`eventTimeKey="ts"` 是内部键，序列化后从 `meta` 中剔除）。
- `meta`：结构化数据。

### 3.3 `meta` 关键键（回放解释器依赖，改动需同步评审）

| 键 | 取值 | 写入方 | 回放用途 |
|----|------|--------|----------|
| `msg_index` | 消息数组下标（字符串） | `WebRenderer.Render` / `persistPendingUserEcho` | **消息组键**：分页、裁剪、retry-from、可视化 |
| `phase` | `input` \| `result` \| **`stream`（FIX-575 新增）** | `agent.withPhase` / `eventCoalescer.flush` | **TOOL 块边界判定** |
| `tool_summary` | ToolSummary JSON | `agent/run_stream.go` | 标题 action + intent、风险徽标、影响文件高亮 |
| `diff_lines` | 逐行 diff JSON | `agent`（`tool_call_diff`） | 写文件类工具的彩色 diff |
| `sup_part` | `prompt` \| `content` \| `tool` | `agent/sup_stream.go` | SUP 块三段式路由 |
| `prompt`/`completion`/`total`/`max`/… | 数值字符串 | `TokenIterEvent` | 状态栏统计 + **可视化条 flush 时机** |

### 3.4 两个层次的概念（务必区分）

| 概念 | 定义 | 载体 |
|------|------|------|
| **块（block）** | 正文流中一个独立卡片（`.ev.llm` / `.ev.tool` / …） | DOM 节点，可视化条 1 条线 |
| **消息组（message group）** | `meta.msg_index` 相同的连续事件序列（一次 LLM 迭代产生的所有块） | 分页与裁剪的最小单位 |

> 一个消息组通常包含 **多个块**。因此"按组分页"不等于"按块分页"。

---

## 4. 写入策略（持久化）

### 4.1 入口：`WebRenderer.Render(ev)`（`web/session.go`）

```
Render(ev)
 ├─ 1. ev.Meta["msg_index"] = strconv.Itoa(msgIndexForRetry())   // len(agent.Messages())-1
 ├─ 2. persistPendingUserEcho()      // 本轮用户消息回执，只入库、不发浏览器
 ├─ 3. for _, pe := range coalescer.Add(ev):                    // 合并 → 完整块事件
 │        data, _ := marshalEvent(pe)
 │        Store.AppendEvent(CurrentSessionID(), data)           // 失败仅 log.Warn
 └─ 4. srv.sendEvent(ev)            // 实时事件原样下发（含分片）
```

**顺序保证**：`Add()` 内部"先 flush 上一个未完成块，再追加当前事件"，因此持久化流的顺序严格等于实时顺序。

**为什么用户消息回执在 `Render` 里写？** 只有此刻才知道用户消息在 `agent.Messages()` 中的下标（`agent.RunStream` 内部 append）。它复用**同一 `msg_index`**，因此与本轮第一个块同组，分页/`:session pop` 时同生共死。

### 4.2 合并策略：`eventCoalescer`（`web/eventcoalesce.go`）

| 输入事件类型 | 处理 | 输出 |
|--------------|------|------|
| `content_chunk` | 累积到 `pending{kind:"content"}` | 块结束时发 `content` |
| `thinking_chunk` | 累积到 `pending{kind:"thinking"}` | 块结束时发 `thinking` |
| `tool_call_stream` | 累积到 `pending{kind:"tool"}`；**出现 `⚙️ <tool>` 表头则先 flush 再开新块** | 块结束时发 `tool_call` + `meta.phase=stream` |
| 其他任意类型 | **先 flush**（把累积块落库），再原样输出该事件 | 原事件 |

- `flush()` 丢弃 `text == ""` 的累积块（空块无渲染意义）。
- **每个合并块携带"块起点"语义**（FIX-575 起）：`meta[phase] = "stream"`，与 `agent.withPhase` 产出的 `input`/`result` 并列。
- 合并块的 `meta` 取自**首片**（`cloneMeta`），保证 `msg_index`、上下文用量归属正确。

### 4.3 序列号分配与裁剪（`Store.AppendEvent`）

```
tx = db.Update
  ├─ 定位 nextSeq：Seek(sessionID+"\x00"+0xFF)
  │     ├─ 命中 → cursor.Prev() 取本会话最大 seq
  │     └─ 未命中 → cursor.Last()（需再校验前缀，否则说明本会话尚无事件 → seq=1）
  ├─ Put(key(sessionID, nextSeq), eventJSON)
  └─ trimEventStream(bucket, prefix)     // len > 5000 时从最旧开始删
        cut = len - 5000
        while cut > 0 && group(keys[cut-1]) == group(keys[cut]): cut--   // 对齐组边界（I1）
        delete keys[:cut]
```

- **前缀校验必不可少**：FIX-509 的教训——缺少 `Prev()`/前缀校验时，跨会话键会被误认，`nextSeq` 回退为 1，**覆盖 seq=1 把最旧事件冲掉**（历史出现空洞）。
- 裁剪以**整组**为界：宁可暂时多留几条，也不产生半截消息。
- 修改 seq 定位逻辑时必须跑 `store/eventstream_test.go`、`web/fix509_history_test.go`。

### 4.4 失败策略

`AppendEvent` 失败只 `log.Warn("append event stream: %v")`，**不影响**实时结果：历史少一条，但当前回合继续。这是 G3 的体现。

---

## 5. 读取与恢复策略

### 5.1 触发时机

| 场景 | 触发 | 行为 |
|------|------|------|
| 页面连接 / 刷新 | WebSocket `open` | `resetStreamView()` → `history_get{count: page_buffer_size}` |
| 进程重启后重连 | 同上（会话 ID 从 DB 恢复） | 同上 |
| 切换会话 | `session_switch` 回执 | `resetStreamView()` → `history_get{count, value: sessionID}` |
| 上滚到顶 | 顶部哨兵 `IntersectionObserver` | `history_get{count, before: historyOldestSeq}`（插入到顶部） |

### 5.2 分页

- 页大小：`page_buffer_size`（默认 **20**，配置项 `llm.page_buffer_size`）。
- 后端 `web/session.go:loadHistoryPage` → `Store.LoadEventsByGroups(sessionID, groups, beforeSeq)`：
  1. 收集游标以下的全部键；
  2. 按 `EventGroupKey`（即 `meta.msg_index`，缺失时退化为事件自身）切分为**连续段**；
  3. 取最后 `groups` 段，整段返回 → **保证 I1**；
  4. 返回 `hasMore` 与 `oldestSeq`（下一页游标）。
- 早期实现按"`count × maxEventsPerMessage` 事件窗口"截断，会把长消息切半——FEATURE-548 已修，勿回退。

### 5.3 恢复完备性（**这是"看起来丢了历史"的常见误解来源**）

> **初始页只回放最近 `page_buffer_size` 个消息组**；更早的内容必须由用户上滚触发懒加载，才会被渲染出来（正文与可视化条同时补齐）。

因此：
- "刷新后只有最近一段，上滚能补回来" → **正常的分页行为，不是 bug**；
- "刷新后缺失的块上滚补不回来、且散落在中间" → **真 bug**（FIX-575，见 §7）。

### 5.4 DOM 滑窗（`enforceStreamWindow`，FEATURE-508）

- 阈值：`stream-window-max-blocks`（默认 300）/ `stream-window-max-nodes`（默认 30000）。
- 每次 `makeBlock` 后检查，超出则从最旧的**整组**开始淘汰（不拆组），最新组永不淘汰。
- 淘汰只影响 DOM，不回写数据库；用户上滚可重新加载。

### 5.5 回放渲染状态机（`app.js:renderEvent`）

回放与实时共用 `renderEvent`，但**实时有分片事件、回放只有合并事件**，因此两边的游标状态不同：

| 游标 | 实时写入者 | 回放写入者 |
|------|-----------|-----------|
| `curLLM` | `content_chunk` | `content` |
| `curThinking` | `thinking_chunk` | `thinking` |
| `curTool` | `tool_call_stream`（齿轮表头开新块）、`tool_call` | `tool_call`（`phase=stream` 开新块） |
| `curREPL` | `ui_text(chan=repl)` | 同左 |
| `toolBlockByName` / `iterToolBlocks` | 由流式块登记，供 `phase=input` 复用 | **FIX-575 起同步登记**（否则 input 事件找不到块） |

**TOOL 块边界规则（FIX-575 后，两端一致）**：

| 事件 | phase | 处理 |
|------|-------|------|
| 合并的流式参数块（回放）/ `tool_call_stream` 齿轮表头（实时） | `stream`（实时无此事件类型） | **无条件开新 TOOL 块**，剥表头、登记 `toolBlockByName`/`iterToolBlocks`、参数进 `params.raw`、`hasResult=false` |
| 工具调用前的意图摘要 | `input` | `fresh` 时按名字复用流式块，否则新建；写标题 + 风险徽标（`hasResult=false`） |
| 执行结果 / 工具错误 | `result`（或 `error`+`chan=tool`） | 追加到当前块，`hasResult=true` |
| 旧库数据：无 `phase` 且文本含 `⚙️` | —（兼容路径） | 与 `stream` 同样处理 |

### 5.6 消息可视化条（`#msgViz`，FEATURE-482）

- `makeBlock()` 每创建一个块 → `msgVizPending.push({cls, box})`（只排队，不画线）。
- **仅在 `token_iter` 事件到达时** `msgVizFlush()` 才真正生成线（颜色由块 class 决定：tool=黄、llm=青蓝、error=红…），红点高度 = 当时上下文占用。
- 结论：**线数天然等于已渲染块数**（I4）。若概览与正文不符，说明块的创建规则不一致。

---

## 6. 流程逻辑总览（端到端时序）

### 6.1 一次工具调用的实时路径

```
LLM 流式返回
 ├─ (可选) content_chunk ×N            → 浏览器: LLM 块；入库: 合并为 content
 ├─ tool_call_stream ×N（首片含 ⚙️ 表头）→ 浏览器: 新建/追加 TOOL 块(参数区)；入库: 合并为 tool_call(phase=stream)
 ├─ tool_call(phase=input, tool_summary)→ 浏览器: 复用该 TOOL 块写标题/intent；入库: 原样
 ├─ [工具执行]
 ├─ tool_call(phase=result) / error     → 浏览器: 追加结果到该块；入库: 原样
 └─ token_iter                          → 浏览器: 追加统计行 + msgVizFlush()；入库: 原样
```

### 6.2 恢复（刷新/重启）路径

```
连接建立
 └─ resetStreamView()                // 清空 DOM 与游标（含 msgVizTrack、msgVizPending）
     └─ history_get{count:20}
         └─ LoadEventsByGroups → 最近 20 个消息组（整组）
             └─ renderHistory(events) → 逐条 renderEvent() 重建块
                 ├─ 每个块 → msgVizPending 排队
                 └─ token_iter → msgVizFlush() 落线
上滚到顶 → history_get{before:oldestSeq} → prependingHistory=true → 块的线与块一起插到最左侧
```

---

## 7. 本次缺陷与修复（FIX-575）

### 7.1 现象

重启（或刷新）恢复会话后，正文消息流与顶部可视化条中**部分工具调用块随机缺失**：
- 位置：**散落在中间**，前后都有，成块缺失；
- 上滚到底**也补不回来**（数据在库中，属于已回放窗口，是渲染丢失而非分页）；
- 缺失位置每次不同（"随机"）。

### 7.2 根因（违反 I2/I3）

| | 实时路径 | 回放路径（修复前） |
|---|---|---|
| 块起点信号 | `tool_call_stream` 的 `⚙️ <tool>` 表头（`app.js:2026-2048`：`if (!curTool \|\| isNewTool)` **无条件开新块**） | 已被合并掉，只剩**无 `phase`** 的 `tool_call` |
| 判定条件 | 显式信号 | **状态启发式**：`(!phase && type==="tool_call" && !fresh && !curTool.raw)`，`fresh = !curTool \|\| curTool.hasResult` |
| 辅助表 | `toolBlockByName` / `iterToolBlocks` 已登记 | 为空 → `phase=input` 无法复用，行为不可预期 |

两种失效形态（是否触发取决于上一个工具调用是否留下结果事件，而后者受工具种类 / `showToolOutput` / 结果是否为空影响，故表现为随机）：

- **(a) 丢块**：上一个 TOOL 块 `raw` 非空 → 判定为假 → 落入 `else`（结果分支）→ `if (!curTool)` 不成立 → **本次调用的参数被并入上一块，自己不再成块**；
- **(b) 吞块**：`curTool.raw` 为空且 `hasResult=false` → 走 `input` 分支但 `fresh=false` → **不新建块且文本被直接丢弃**。

数据侧无问题（`AppendEvent` 正常），纯粹是**回放解释器状态机与实时不一致**。

### 7.3 修复设计

1. **后端显式化块起点**（`web/eventcoalesce.go`）：合并出的工具块携带 `meta.phase = "stream"`。
   —— 从此块起点是**数据**而不是**推断**（满足 I3）。
2. **前端回放与实时同构**（`web/static/app.js`）：`phase === "stream"`（或旧数据的齿轮表头）→ **无条件开新 TOOL 块**，并完整登记 `toolBlockByName`/`iterToolBlocks`，使后续 `phase=input` 能按名字复用、`phase=result` 追加到同一块（满足 I2）。
3. **不再静默丢文本**：`input` 分支在 `fresh === false` 时也要把事件文本落到参数区/`raw`。
4. **向后兼容**：升级前已入库的旧事件（无 `phase=stream`）走"齿轮表头"兼容路径，历史同样能正确重建。

### 7.4 影响面与取舍

- 仅影响 Web 回放的 TOOL 块重建；终端渲染（`LineRenderer`）、JSON 流（`StreamRenderer`）不受影响。
- 不改变数据库 schema（只增加一个 `meta` 键），旧库可直接使用，无需迁移。
- 旧库事件流**不会**自动获得 `phase=stream` 标记（历史数据写入时无法追溯），兼容路径长期保留。

---

## 8. 验证与回归清单

### 8.1 自动化

| 层级 | 命令 / 用例 |
|------|-------------|
| 编译与静态检查 | `go build ./... && go vet ./...`（必须一次执行） |
| 事件流存储 | `go test ./store/ -run 'Event|FIX509|FEATURE548' -v` |
| 合并与回放 | `go test ./web/ -run 'FIX575|FIX549|FIX506|FEATURE548|Coalesce' -v` |
| 全量 | `go test ./...`（与 main 基线比对，确认无新增失败） |

### 8.2 运行时用例（人工/半自动，存于 `use-case/FIX-575/`）

| 用例 | 步骤 | 期望 |
|------|------|------|
| UC-01 单轮多次调用 | 一轮内连续 3 次 `read_file`，随后刷新页面 | 刷新前后 `.ev.tool` 数量/顺序/标题完全一致 |
| UC-02 无结果事件的工具 | 触发 `attempt_completion` / 空结果工具后紧跟其他工具调用，刷新 | 后续工具调用仍各自成块，未被并入上一块或吞掉 |
| UC-03 写文件类工具 | 含 `tool_call_diff` 的 `write_to_file`/`replace_in_file`，刷新 | diff 块与参数区完整，块数一致 |
| UC-04 旧库兼容 | 用升级前落库的会话回放（无 `phase=stream`） | 与实时结构一致 |
| UC-05 切会话 / 重连 | 会话 A↔B 切换、断线重连后各自回放 | 无串会话、无重复块、无缺失块 |
| UC-06 概览一致性 | 任一会话刷新前后 | `#msgVizTrack` 黄线数 == `.ev.tool` 数 |

---

## 9. 关键代码位置速查

| 关注点 | 位置 |
|--------|------|
| 事件类型常量 | `agent/events.go`（`EventToolCall="tool_call"`、`EventToolCallStream="tool_call_stream"`、`PhaseInput/PhaseResult/MetaKeyPhase`） |
| 工具调用事件发射 | `agent/run_stream.go:1481-1693`（input 摘要 / 结果 / 工具错误） |
| 渲染器入口与持久化 | `web/session.go` `WebRenderer.Render` / `persistPendingUserEcho` / `msgIndexForRetry` |
| 合并器 | `web/eventcoalesce.go`（`Add` / `appendFragment` / `flush` / `coalescedType` / `marshalEvent`） |
| 分页读取 | `web/session.go:loadHistoryPage`、`pushHistory`；`store/eventstream.go:LoadEventsByGroups` |
| 事件写入与裁剪 | `store/eventstream.go:AppendEvent` / `trimEventStream` / `EventGroupKey` / `DeleteEventsAfter` |
| 前端回放 | `web/static/app.js:renderHistory` / `resetStreamView` / `renderEvent`（TOOL 分支 ~2104-2244） |
| 可视化条 | `web/static/app.js` `msgVizColor` / `msgVizFlush` / `msgVizApplyPan`；`index.html:#msgViz` |
| DOM 滑窗与懒加载 | `web/static/app.js:enforceStreamWindow` / `loadOlderHistory` / `ensureTopSentinel` |

---

## 10. 变更记录

| 日期 | 任务 | 内容 |
|------|------|------|
| 2026-10-01 | FIX-575 | 首版：固化持久化/恢复策略与流程；新增不变量 I1–I6；修复「恢复后工具调用块随机丢失」（回放与实时建块规则不一致），引入 `meta.phase=stream` 显式块起点标记 |
