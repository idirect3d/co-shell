# FEATURE-524 测试用例：LLM 组件化输出协议（co-shell Web UI 富组件渲染）

## 背景

当前 Web UI 的输出上限是「一段排版过的文本」：

| 位置 | 现状 | 问题 |
|---|---|---|
| `agent/events.go:19` | 唯一下行单元 `StreamEvent{Type,Level,Chan,Text,Meta}` | 只能承载文本，没有结构化富内容通道 |
| `web/static/md.js` | 手写 Markdown 子集渲染器，注释明确 *never feeds raw input to innerHTML* | 不支持卡片/图表/交互控件 |
| `web/static/app.js:1497` | `renderEvent()` 按 `ev.type` 分发 | 未知事件类型落到 `default` 分支，组件事件无处渲染 |
| `web/server.go` | 无任何 CSP 响应头 | 开放 HTML 输出前必须补齐 |

因此 LLM 无法把「结果」画出来。本任务引入 `render_ui` 工具 + 组件树协议 + 前端 registry。

## 需求（用户确认）

1. 目标用户：非技术用户直接用 co-shell 完成任务并看结果（办公 / 数据分析 / 资料整理）。
2. 表达载体：**工具调用为主**——新增 `render_ui` 工具，参数为组件树 JSON。
3. 渲染归属：声明式组件走**主 DOM**（复用现有 CSS 变量，四套主题自动一致）；仅 `html` 逃生舱进 sandbox iframe。
4. 图表/地图：**手写 SVG**（柱/折线/饼），零依赖红线不破；本期不做地图。
5. 交互语义：组件动作开启新一轮 agent 回合（续作型），支持图表数据点钻取。
6. 阻塞策略：由工具参数 `waiting` 控制，**默认不阻塞**。
7. 更新机制：`ui_update` 按 id 原地更新，作用域 = **当前回合内任意组件**。
8. 上下文裁剪：本期就做，配置开关控制，**默认开**。
9. 跨端：只做 Web UI，终端/飞书降级为纯文本或省略。
10. 归属版本 **v0.59.0**（FEATURE，minor+1）；任务号 **FEATURE-524**；分支 `FEATURE-524`。

## 实现契约

| 项 | 约定 |
|---|---|
| 组件树节点 | `{type, id?, props?, children?, actions?}` |
| 动作声明 | `{on: click\|select\|submit\|change, id, payload: node\|value\|row\|point\|form}` |
| 工具签名 | `render_ui(tree: object, waiting: bool=false, intent: string)` |
| 工具安全级别 | **只读，免确认放行**（不触发 confirm 交互） |
| 工具返回值 | 仅简短回执（形如 `已渲染 card/ui-7（3 个子节点）`），**不回传树本身** |
| 下行事件 | `ui_render`（`Meta{ui_id, ui_tree}`）、`ui_update`（`Meta{ui_id, ui_patch}`） |
| 上行消息 | `{"type":"ui_action","ui_id","action_id","payload"}` |
| 渲染容器 | 事件流内新块类型 `.ev.ui-block`，复用现有折叠/定位/复制机制 |
| 安全铁律 | 声明式组件一律 `createElement` + `textContent`，**禁止 innerHTML** |
| 沙箱 | iframe `sandbox="allow-scripts"`，**不加 `allow-same-origin`** |
| 校验上限 | 组件类型白名单；树深度 ≤ 6；节点数 ≤ 200；单 props 序列化 ≤ 8 KiB |
| MVP 组件 | `card` / `kv` / `table` / `chart` / `steps` / `callout` / `progress` / `file` / `form` / `html` |
| 非目标（明确不做） | ① 地图组件；② 引入任何第三方图表库/前端框架；③ 终端与飞书的组件原生渲染（仅要求降级不报错）；④ 跨回合的组件原地更新；⑤ 用户自定义 JS 注册组件（仅内置 registry） |

## 验收前置

```bash
cd /Users/direct3d/github/co-shell

# 1) 编译到 work/（规范：先编 work/ 再原子替换 ~/bin/）
go build -o work/co-shell . && go build -o work/co-shell-hub ./cmd/co-shell-hub

# 2) 启动独立测试实例（不使用生产实例 28256，独立端口 + 独立 workdir）
rm -rf /tmp/feat524 && mkdir -p /tmp/feat524
cd /tmp/feat524 && /Users/direct3d/github/co-shell/work/co-shell --serve --dev --port 28260 --workdir /tmp/feat524
```

> `--dev` 使前端静态资源从磁盘热加载，改 CSS/JS 不必重新编译。
> Go 侧改动仍需重新编译并重启实例。

浏览器判定统一在 `browser_evaluate` 中执行（`querySelector` / `getComputedStyle` / `getAttribute`），并配合 `browser_screenshot` 视觉复核。

---

## A 组：组件树校验（Go 单测，`agent/uitree_test.go`）

### UC-01 合法最小树通过校验

- **输入**：`{"type":"card","props":{"title":"标题"}}`
- **预期**：`ValidateTree` 返回 `nil` error；节点计数 = 1。

### UC-02 未注册组件类型被拒绝

- **输入**：`{"type":"iframe_hack"}`
- **预期**：返回 error；错误信息**包含可用类型列表**（便于 LLM 自我纠正）。

### UC-03 树深度超限被拒绝

- **输入**：构造 7 层嵌套的 card（深度 7 > 上限 6）
- **预期**：返回 error，指明超限的是「深度」。

### UC-04 节点数超限被拒绝

- **输入**：构造 201 个节点的 card 树
- **预期**：返回 error，指明超限的是「节点数」。

### UC-05 props 体积超限被拒绝

- **输入**：单个节点 props 序列化后 > 8 KiB
- **预期**：返回 error，指明超限的是「props 体积」。

### UC-06 非法 actions 被拒绝

- **输入**：① `actions:[{"on":"onmouseover","id":"a"}]`（未知 on）② `actions:[{"on":"click"}]`（缺 id）
- **预期**：两种情况均返回 error。

---

## B 组：工具与事件（Go 单测，`agent/ui_tools_test.go`、`agent/events_test.go`）

### UC-07 render_ui 返回简短回执

- **输入**：`render_ui`，树为 card 含 3 个子节点
- **预期**：返回字符串包含组件类型与子节点数（如 `card` + `3`），长度 < 200 字节，**不含完整树 JSON**（断言返回串里不出现子节点的 props 明文）。

### UC-08 ui_render 事件构造正确

- **预期**：`ev.Type == "ui_render"`；`ev.Meta["ui_id"]` 非空；`ev.Meta["ui_tree"]` 可 `json.Unmarshal` 回原树且深度相等。

### UC-09 ui_update 事件构造正确

- **预期**：`ev.Type == "ui_update"`；`ev.Meta["ui_id"]` 与目标一致；`ev.Meta["ui_patch"]` 可解析。

### UC-10 render_ui 免确认放行

- **预期**：工具定义中 risk 为只读；执行 `render_ui` 不产生 `Interaction`（不出现 confirm 交互请求）。

---

## C 组：前端基础渲染与安全（浏览器实测）

### UC-11 card 渲染顺序正确

- **输入**：card（title + 2 个 children）
- **预期**：存在 `.ui-card`；标题文本正确；2 个子节点按数组顺序排列（DOM 顺序断言）。

### UC-12 kv 键值列表

- **输入**：`kv` 含 3 组键值
- **预期**：渲染 3 行；键与值文本均正确；长值不溢出容器（`scrollWidth <= clientWidth + 1`）。

### UC-13 callout 四种变体

- **输入**：`callout` 的 `variant` 分别为 `info` / `warn` / `success` / `error`
- **预期**：四种变体各自的颜色变量不同（`getComputedStyle` 的 border/图标颜色两两不等）。

### UC-14 progress 进度条

- **输入**：`progress` 值 0 / 50 / 100
- **预期**：填充条宽度比例分别约 0% / 50% / 100%（容差 ±2%）；数值越界（-10、150）被夹紧到 0/100。

### UC-15 未注册组件优雅降级

- **输入**：`{"type":"nonexistent_widget"}`
- **预期**：渲染占位块显示组件名；原始 JSON 包在可折叠区域内；**页面不抛异常、后续事件仍正常渲染**。

### UC-16 XSS 防护（核心用例）

- **输入**：props 中注入 `"<img src=x onerror=window.__xss=1>"`、`"<script>window.__xss=1</script>"`、`"</div><b>x</b>"`
- **预期**：① `window.__xss` 为 `undefined`；② 容器内 `querySelectorAll("img,script,b")` 长度为 0；③ 注入串以**文本**形式可见（`textContent` 包含原串）。

---
## D 组：数据展示组件（浏览器实测）

### UC-17 table 渲染正确

- **输入**：`table` 含表头 3 列 + 4 行数据
- **预期**：`thead` 1 个、`tbody tr` 4 个、每个 `tr` 3 个单元格；表头与首行文本正确。

### UC-18 table 空数据空态

- **输入**：`table` 的 `rows` 为空数组
- **预期**：不渲染空 `tbody`；显示明确的空态文案（来自 i18n）；不抛异常。

### UC-19 table 列对齐与状态色

- **输入**：列声明 `align:"right"` 与 `status:"warn"` 的单元格
- **预期**：对齐列 `text-align` 为 `right`；status 单元格颜色与普通单元格不同。

### UC-20 chart 柱状图

- **输入**：`chart` kind=`bar`，3 个分类
- **预期**：容器内存在 `<svg>`；柱子元素数 = 3；存在坐标轴文本；柱高比例与数值比例一致（容差 ±3%）。

### UC-21 chart 折线图

- **输入**：`chart` kind=`line`，5 个数据点
- **预期**：存在 `<polyline>` 或等价的 `<path>`；数据点标记数 = 5；存在 x/y 轴刻度标签。

### UC-22 chart 饼图

- **输入**：`chart` kind=`pie`，4 个扇区
- **预期**：扇区 `<path>` 数 = 4；各扇区角度占比与数值占比一致（累积起始/结束角容差 ±2°）。

### UC-23 chart 边界值

- **输入**：① 全 0 数据；② 负值；③ 单个数据点；④ 超长分类名
- **预期**：四种情况均不抛异常；全 0 显示零基线而非除零 NaN；超长分类名被截断或换行且不撑破容器（`scrollWidth <= clientWidth + 1`）。

### UC-24 steps 步骤时间线

- **输入**：`steps` 含 3 个步骤，状态分别为 `done` / `active` / `pending`
- **预期**：渲染 3 项；三种状态样式不同（颜色/图标）；完成后有连接线。

### UC-25 file 文件卡片

- **输入**：`file` 指向工作区内真实存在的文件（相对路径）
- **预期**：显示文件名与大小；提供打开/定位入口且**复用现有 `/api/open`、`/api/reveal`**；路径逃逸（`../../etc/passwd`）被拒绝并降级为纯文本展示。

---

## E 组：交互与原地更新（浏览器实测）

### UC-26 form 提交发出 ui_action

- **输入**：`form` 含一个输入框 + 提交按钮，动作声明 `{on:"submit", id:"go", payload:"form"}`
- **预期**：填写并提交后，前端 WebSocket 发出的消息形如 `{"type":"ui_action","ui_id":"...","action_id":"go","payload":{...}}`（可在 `browser_evaluate` 中拦截 `wsSend` 或读取后端日志断言）。

### UC-27 图表数据点点击回传结构化载荷

- **输入**：`chart` 带 `actions:[{on:"select", id:"drill", payload:"point"}]`，点击第 3 个数据点
- **预期**：上行的 `payload` 包含该点的**分类标签与数值**（非仅有索引），使 LLM 无需猜测。

### UC-28 动作触发新一轮 agent 回合

- **预期**：`ui_action` 到达后端后，agent 开启新回合（日志出现 `turn_start`）；前端出现新的用户输入回显或流式内容块；`render_ui` 工具**不在**该回合中重新调用即可继续。

### UC-29 ui_update 原地更新

- **输入**：同一回合内，先 `render_ui`（card id=`n1`，title=旧），再触发 `ui_update`（patch 为 `n1` 的 props.title=新）
- **预期**：**DOM 中 `.ui-card` 数量不增加**；原节点标题文本变为「新」；节点在流中的位置不变。

### UC-30 ui_update 目标不存在

- **输入**：`ui_update` 指向不存在的 `ui_id`
- **预期**：不抛异常；输出一条可诊断的警告（前端 console 或后端日志）；流中其余内容正常。

### UC-31 waiting=false 不阻塞

- **输入**：`render_ui` 默认参数（waiting 未传）
- **预期**：工具**立即**返回回执（无挂起）；agent 继续同回合执行后续步骤。可用时间断言：工具调用到下一事件间隔 < 2s。

### UC-32 waiting=true 阻塞与释放

- **输入**：`render_ui(waiting=true)`
- **预期**：① 工具挂起等待（同回合不继续）；② 用户点击组件内动作 → 动作作为**工具返回值**返回，agent 在同回合内继续；③ 用户改为直接发消息 → 阻塞被释放，工具返回「用户未操作」，新消息正常作为新一轮处理。

---
## F 组：安全（浏览器实测 + 代码审查）

### UC-33 html 逃生舱沙箱属性正确

- **预期**：渲染出的 iframe 满足：① `sandbox` 属性存在且**含** `allow-scripts`；② `sandbox` **不含** `allow-same-origin`；③ 无 `allow-top-navigation`、`allow-popups`。

### UC-34 沙箱内无法访问父页面

- **输入**：`html` 组件内容为 `<script>window.parent.document.title</script>`
- **预期**：iframe 内脚本访问 `parent.document` 抛 `SecurityError`（不透明源）；父页面 `document.title` 未被修改；父页面 `localStorage` 未被读取。

### UC-35 伪造 postMessage 被拒绝

- **预期**：从页面其它来源（非该 iframe）向 window 发送同结构消息，前端不产生任何 `ui_action` 上行；控制台有可诊断日志。

### UC-36 主页面 CSP 头存在且有效

- **预期**：`GET /` 响应含 `Content-Security-Policy` 头；在开启 CSP 后，A~E 组全部用例仍通过（**无回归**是此用例的核心）；`/api/ui-sandbox` 响应有独立且更宽松的 CSP，仅作用于该文档。

### UC-37 html 内容非可信输入

- **输入**：`html` 组件 `content` 字段由服务端经 `/api/ui-sandbox` 返回（而非拼接进 srcdoc）
- **预期**：父页面 HTML 中不出现该内容明文；渲染通过 iframe 加载完成。

---

## G 组：治理与一致性（浏览器实测 + Go 单测）

### UC-38 上下文裁剪默认开启

- **预期**：`render_ui` 工具结果**本身**即为简短回执；历史压缩后，组件树在上下文中的表示变成摘要（如 `已渲染 chart:销售趋势 [ui-7]`），**不出现**完整树 JSON。

### UC-39 裁剪开关可关闭

- **预期**：配置关闭后，历史保留原树 JSON；开启时恢复压缩。开关默认值为「开」。

### UC-40 系统提示词含组件目录（zh / en）

- **预期**：`i18n/zh_system.go` 与 `en_system.go` 均含组件目录分节；生成的系统提示词中包含 10 个组件名与 `render_ui` 的调用说明；语言切换后目录文案随之切换。

### UC-41 系统提示词分节可被 .rules 覆盖

- **预期**：在工作区 `.rules/` 下放置同名分节文件后，生成结果使用外部文件内容（与现有分节机制一致）。

### UC-42 四套主题观感一致

- **输入**：分别在 `dark` / `dark-muted` / `light` / `eyecare` 下渲染同一套组件
- **预期**：组件颜色**全部来自 CSS 变量**（断言组件样式中不出现硬编码 `#rrggbb` 之外的固定色，或抽查 3 处关键颜色随主题变化）；四套主题下文字与背景对比度均 ≥ 4.5:1（正文）；截图人工复核无「白底黑字/黑底黑字」错配。

### UC-43 无第三方依赖（构建约束）

- **预期**：`go.mod` 未新增依赖；`web/static/` 未新增第三方 JS/CSS 文件；页面加载的 `<script>`/`<link>` 全部同源、无 CDN 引用。

---

## H 组：端到端场景（浏览器实测，真实 LLM 驱动）

### UC-44 生成可视化分析报告

- **步骤**：在测试实例中对 agent 下达任务：「用 render_ui 给我一份本月销售分析报告，包含结论卡片、关键指标、明细表格和柱状图」。
- **预期**：① agent 调用 `render_ui` 且参数通过校验；② 页面渲染出卡片 + 指标 + 表格 + 柱状图；③ 无控制台错误；④ 截图人工复核：图形化、直观、无文本堆砌。

### UC-45 图表数据点钻取

- **步骤**：承接 UC-44，点击柱状图某个柱子。
- **预期**：① 触发新一轮 agent 回合；② LLM 收到的载荷**包含该柱子的分类名与数值**；③ LLM 给出该分类的细化分析（可能再次 `render_ui`）；④ 全过程无需用户手动重新描述上下文。

### UC-46 会话刷新后组件重建

- **步骤**：完成 UC-44 后刷新页面（或切换会话再切回）。
- **预期**：历史重放中组件**被正确重建**（而非显示为「未知事件」或空白）；组件数量与刷新前一致；交互动作在重建后的组件上仍可用。

### UC-47 非 Web 出口降级不报错

- **步骤**：在终端 TUI 模式下让 agent 调用 `render_ui`。
- **预期**：终端不抛异常、不卡死；输出可读的降级文本（组件类型 + 文本摘要）；`--output-format json` 下事件仍能正常序列化。

### UC-48 与既有交互机制共存

- **步骤**：在一次 `render_ui`（waiting=true）挂起期间，触发一个需要确认放行的工具调用（如 `execute_command`）。
- **预期**：两种交互不互相吞掉；用户操作分别路由到正确目标；无死锁。**这是 Stage 3 的专项风险点**。

### UC-49 长会话不劣化

- **步骤**：连续渲染 5 个组件树后继续对话。
- **预期**：① 上下文占用增长**显著低于**不裁剪时的完整树体积；② 流渲染性能可接受（无肉眼可见卡顿）；③ 页面 DOM 节点数不失控。

---

## 通过标准

进入下一阶段（合并）的硬性条件：

1. **A/B/E/G（Go 单测部分）全部通过**：`go test ./agent/... ./web/... -run '524'` 全绿。
2. **C/D/F 组浏览器实测全部通过**：逐条在 `browser_evaluate` 中执行断言并留存结果。
3. **H 组端到端全部通过**：UC-44 / UC-45 / UC-46 为核心必过项（其余可降级为「已知限制」并记录）。
4. **编译与静态检查**：`go build ./... && go vet ./...` 全绿；`node --check web/static/ui.js` 等新增 JS 全部通过。
5. **无回归**：既有 Web UI 功能（事件流渲染、任务计划面板、ask 交互、文件预览）不受影响。
6. **依赖红线**：`go.mod` 无新增依赖，`git diff go.mod go.sum` 为空。

## 已知限制（本期接受，记录在案）

| 限制 | 说明 |
|---|---|
| 不做地图组件 | 真实地图需瓦片服务与坐标系，与零依赖红线冲突，后续版本单独立项 |
| `ui_update` 仅限当前回合 | 跨回合更新需处理已折叠/已滚出视口的块，复杂度高，本期不做 |
| 图表仅 3 种 | 柱/折线/饼；散点、热力、桑基等后续按需扩展 |
| 终端/飞书无原生渲染 | 仅降级为文本，不保证观感 |
| 组件集固定 | 仅内置 registry，不支持用户自定义 JS 注册组件（避免任意代码执行面） |
