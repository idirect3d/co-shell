# LLM 组件化输出协议（FEATURE-524）

> 本文是 `render_ui` 协议的实现契约与安全模型说明，覆盖 v0.59.0 引入的 10 个组件、
> 下行/上行事件、上下文裁剪开关与沙箱隔离。代码落点：
> `agent/uitree.go`（校验）、`agent/ui_tools.go`（工具）、`agent/events.go`（事件）、
> `agent/loop.go` + `agent/ui_prune.go`（上下文裁剪）、`web/static/ui*.js`（渲染）。

## 1. 一句话模型

LLM 用**声明式组件树 JSON**（而非 Markdown 文本）表达结果；后端校验后把树放进
`ui_render` 事件下发，Web UI 在主 DOM 里画成卡片/表格/图表；只有 `html` 逃生舱
进沙箱 iframe。终端与飞书忽略该事件，继续显示 LLM 的纯文本回复，因此**任何前端
都不会因为 LLM 使用本工具而失效**。

## 2. 工具签名

```
render_ui(meta, tree, update?, waiting?)
```

| 参数 | 必填 | 语义 |
| --- | --- | --- |
| `meta` | 是 | 透明化元数据（intent/risk/risk_reason/affected_objects/progress） |
| `tree` | 是 | 组件树根节点 |
| `update` | 否 | 要原地更新的**已渲染**组件 id；缺省表示渲染一棵新树 |
| `waiting` | 否 | `false`（默认）立即返回；`true` 挂起本次调用，直到用户在组件上操作，并把该操作作为工具结果返回 |

**回执只有一行摘要**（如「已渲染 card（id=ui-7，3 个子节点）。」）。树**不回传**给
LLM——LLM 自己刚写过，回传只会重复占用上下文。

## 3. 组件树节点

```json
{ "type": "card", "id": "可选", "props": { }, "children": [ ], "actions": [ ] }
```

- `type` 必须是白名单内的组件类型（见 §5），否则整棵树被拒。
- `id` 可选，用于 `update` 定位与前端 `data-ui-id`。
- `children` 仅容器类组件可用；叶子组件带 `children` 会被拒。
- `actions`：`{ "on": "click|select|submit|change", "id": "动作标识", "payload": 任意 }`，
  缺 `id` 会被拒。

### 3.1 校验上限（超出即整棵树被拒，错误信息 i18n）

| 限制 | 值 | 常量 |
| --- | --- | --- |
| 最大深度 | 6 | `agent.UIMaxTreeDepth` |
| 最大节点数 | 200 | `agent.UIMaxTreeNodes` |
| 单节点 props 序列化大小 | 8 KiB | `agent.UIMaxNodeProps` |
| `html` 组件内容上限 | 256 KiB | `ui-sandbox.html` 的 `MAX_HTML` |
| 前端递归深度兜底 | 8 | `web/static/ui.js` 的 `MAX_DEPTH` |

## 4. 事件流

| 方向 | 事件 | 载荷 |
| --- | --- | --- |
| 下行 | `ui_render` | `meta.ui_id` = 树 id，`meta.ui_tree` = 树 JSON |
| 下行 | `ui_update` | `meta.ui_id` = 目标 id，`meta.ui_patch` = 替换节点 JSON |
| 上行 | `ui_action` | `{type:"ui_action", ui_id, action_id, payload}` |

- 两个下行事件走 `ChannelSystem`，终端渲染器忽略。
- `ui_update` **原地替换**：`.ui-card` 数量、父节点与前兄弟、索引均不变
  （仅目标块内部重画），因此不会打断用户正在看的其它块。
- `ui_action` 打开**新一轮 agent 回合**：后端把动作渲染成一条用户消息
  （`agent.UIActionMessage`），把结构化取值原样交给 LLM。
- `waiting=true` 时，该动作同时作为**当次工具调用的结果**返回，同一回合内继续；
  已有一次等待在挂起时，第二次 `waiting` 调用降级为普通回执，避免争抢同一动作
  （`agent/beginUIWait`）。

## 5. 组件目录（13 个）

| type | 关键 props | 说明 |
| --- | --- | --- |
| `card` | `title?`, `subtitle?`, `icon?` | 容器；承载标题栏与子节点 |
| `row` | `gap?`（间距 px） | **布局容器**；子节点横向排列，窄屏自动堆叠为单列，可嵌套 |
| `col` | `flex?`（占比权重，默认均分） | **布局容器**；row 内的一列，用于在单列里纵向堆叠多个组件 |
| `kv` | `items: [{k, v}]` | 键值明细（适合"要点/参数"紧凑展示） |
| `callout` | `variant: info\|warn\|success\|error`, `title?`, `text` | 提示块；用于结论与告警 |
| `progress` | `value`, `max?`, `label?` | 进度条 |
| `table` | `columns: [{key, label?, align?, width?}]`, `rows: [{列key: 值}]` | 明细表格 |
| `chart` | `kind: bar\|line\|pie`, `unit?`, `series: [{name, data:[{label, value}]}]` | 手写 SVG 图表，零依赖 |
| `steps` | `items: [{title, desc?, status: done\|active\|pending}]` | 步骤/时间线 |
| `file` | `path`, `name?`, `size?` | 文件卡片；`/api/open`、`/api/reveal` 动作 |
| `form` | `title?`, `fields: [{name, label, type, options?, value?}]`, `submit` | 表单；提交后开启新一轮回合 |
| `html` | `content` | **逃生舱**，沙箱 iframe 内运行（见 §6） |
| `figure` | `xmin/xmax/ymin/ymax?`, `view?`, `axes?`, `grid?`, `download?`；子节点为图元 | **矢量绘图**；2D/3D 逻辑坐标几何示意图，手写 SVG（见 §9） |

**两栅版式例**（左流程 / 右上图标 + 右下表单）：

```json
{"type":"row","props":{"gap":12},"children":[
  {"type":"col","props":{"flex":2},"children":[{"type":"steps","props":{"items":[…]}}]},
  {"type":"col","children":[
    {"type":"card","props":{"icon":"👤","title":"用户信息"}},
    {"type":"form","props":{"title":"登记","fields":[…]},"actions":[{"on":"submit","id":"save"}]}
  ]}
]}
```

窄屏（每列不足 220px）时 row 自动变为上下堆叠，无需额外声明。

新增组件 = 在 `web/static/ui.js`（或 `ui-chart.js`/`ui-form.js`/`ui-layout.js`）里
`UI.register("name", {render(node, ctx)})`。协议、Go 校验与 agent 主循环**无需改动**；
未注册类型在前端优雅降级为原始 JSON 文本块。

## 6. 安全模型

### 6.1 声明式组件：不做 HTML 拼接

`ui.js` / `ui-chart.js` / `ui-form.js` / `ui-layout.js` 与 `md.js` 遵守同一条铁律：**永不赋值 innerHTML，
永不用字符串拼 HTML**。所有元素用 `document.createElement` 创建，所有动态字符串经
`textContent` 落地。因此 LLM 写出 `<img onerror=...>` 时，它在页面上只是**字面文本**，
不会产生任何 DOM。属性（如 `href`、`style`）都由组件实现自行赋值，不接受 LLM 传来的
原始 HTML 片段。

`file` 组件的 `path` 必须落在当前工作区内，否则只显示纯文本路径，不给出「打开/定位」按钮
（路径逃逸防护）。图片类渲染仅允许 `data:` / blob / 同源。

### 6.2 html 逃生舱：不透明源沙箱

`html` 组件经 `<iframe sandbox="allow-scripts">` 加载 `/api/ui-sandbox`：

- **没有 `allow-same-origin`** → 帧运行在不透明源（opaque origin）：拿不到主页面 DOM、
  cookie、localStorage、WebSocket；`iframe.contentDocument` 对父页面为 `null`（实测报
  `SecurityError`）。
- 帧内是**服务端固定的驱动代码**（不是 LLM 输出），负责接收 HTML、重建 `<script>` 并执行；
  LLM 内容通过 `postMessage` 交进去，从不进主 DOM。
- 帧 CSP（`web/server.go:uiSandboxCSP`）：
  `default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:;
  font-src data:; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none';
  frame-ancestors 'self'`。
  即：**零网络**（含 `connect-src 'none'`，无法把数据外发）、仅允许内联脚本与内联样式、
  只接受 `data:` 图片/字体。
- 主页面 CSP（`web/server.go:indexCSP`）为 `frame-src 'self'`，只有同源沙箱页可被嵌入。

### 6.3 帧与父页面的唯一通道：白名单消息

| 方向 | 消息 | 触发 |
| --- | --- | --- |
| 父 → 子 | `{type:"ui-html-render", html}` | 帧 load 后投递内容（超 256 KiB 截断） |
| 父 → 子 | `{type:"ui-html-measure"}` | 帧拿到真实布局盒子时请求重测高度 |
| 子 → 父 | `{type:"ui-html-height", height}` | 内容尺寸稳定后上报 |

父侧只接受 `e.source === frame.contentWindow` 且 `type === "ui-html-height"` 的消息，
高度被夹在 60–1200px；子侧只接受 `e.source === window.parent` 的消息。

**自动高度**（Stage 4 修复）：帧在 `render()` 后 0/80/400ms 采样、`ResizeObserver` 观察
`documentElement`/`body`/`#root`。仅此还不够——当块先被渲染进 `display:none` 容器（折叠块、
静默显示模式）时，帧内测量恒为 0，且**帧重新可见时帧内 RO 不会触发**（实测确认）。因此
父侧额外用 `ResizeObserver` 观察 iframe 元素，一旦它有真实高度就发 `ui-html-measure` 请帧内
重测（不重渲染、不重跑脚本）。实测：可见场景与「隐藏→显示」场景均得到 `frame.style.height =
525px`。

### 6.4 上下文裁剪

`render_ui` 的树会作为工具调用参数留在消息历史里，每次 LLM 调用都要重发。配置项
`UIContextPrune`（JSON：`ui_context_prune`，默认 `true`）在 `buildContextMessages()`
把历史交给 provider **之前**，把树替换为一行摘要（`agent/ui_prune.go`）：

- 两种历史形态都处理：OpenAI 形态的 `ToolCalls[].Arguments` 与 XML 形态的
  `<tree>…</tree>` 内容。
- 只替换树本身，`meta`/`update`/`waiting` 等兄弟参数保持原样；解析失败或非 UI 树一律不动。
- 替换发生在**每次请求的副本**上，持久化历史与 Web UI 刷新回放**仍保留完整树**
  （刷新后组件依然完整渲染）。
- 关闭开关即完全恢复原行为（`UIContextPrune=false`）。

单测：`agent/ui_prune_test.go`（UC-40~UC-44）。

### 6.5 其它治理项

- `UIEnabled`（JSON：`ui_enabled`）：`render_ui` 工具总开关，关闭后工具不出现在工具列表。
- 所有错误/提示文案走 i18n（`i18n/keys.go` 的 `KeyUI*`），中英双语。
- 树校验在 Go 侧完成，前端另有深度兜底，双层防护。

## 7. 相关测试与用例

| 位置 | 覆盖 |
| --- | --- |
| `agent/uitree_test.go` | 树校验：合法/非法/超限/叶子带 children |
| `agent/ui_tools_test.go` | 工具回执与开关 |
| `agent/events_test.go` | `ui_render`/`ui_update` 事件构造 |
| `agent/ui_wait_test.go` | `waiting` 挂起/释放、原地更新（UC-28~UC-32） |
| `agent/ui_prune_test.go` | 上下文裁剪开关与两种历史形态（UC-40~UC-44） |
| `use-case/FEATURE-524/FEATURE-524-UC-0001.md` | 全量用例（UC-01~UC-49），含浏览器端到端实测 |

## 8. 窗口模式：`ui_window`（BUILD-1041）

让 LLM 弹出一个常驻浮层窗口，并在"后台"继续工作时把中间结果持续推回同一窗口，而不是只能把结果追加到对话流里。

### 8.1 两个工具的分工

| 工具 | 职责 |
|---|---|
| `ui_window(meta, action="open"\|"close", title?)` | 只管窗口的开关与标题：**不携带内容** |
| `render_ui(meta, tree, target="window", update?)` | 窗口内容：`target="window"` 渲染进窗口，`update="<id>"` 原地刷新 |

窗口只有**一个实例**：重复 `ui_window(open)` 不会叠加浮层，而是复用同一窗口并更新标题。窗口**不阻塞**回合：除用户操作需要等待外，agent 可以边跑边多次推送更新。

### 8.2 事件与 Meta

| 事件 | 通道 | Meta |
|---|---|---|
| `ui_window` | `ChannelSystem` | `ui_window_action` = `open`/`close`，`ui_window_title`（仅 open） |
| `ui_render` / `ui_update` | `ChannelSystem` | 原有 `ui_id` + `ui_tree`/`ui_patch`；`target="window"` 时额外带 `ui_target` |

`ui_target` 只在非默认（窗口）时出现，因此对话流渲染的事件形状与既有用例（UC-08/UC-09）完全一致。终端等无窗口出口没有 `ui_window` 事件的分支，只输出降级文本，不会报错。

### 8.3 用户操作的两条路径

| 场景 | 行为 |
|---|---|
| 存在 `render_ui(waiting=true)` 挂起 | 动作作为该调用的**工具结果**返回，同一回合继续（沿用 UC-32/UC-48） |
| 回合运行中、动作未声明 `blocking` | 作为 `<ui_action>` **动态事件**在当前回合的下一次 LLM 调用前注入（Q3-A），回合不被打断 |
| 无活跃回合 | 退化为既有的"开启新一轮"行为 |

`actions:[{on, id, payload, blocking?}]` 中的 `blocking:true` 由前端随 `ui_action` 上行（`clientMessage.blocking`）：它声明"这个动作期望被一个挂起的 `waiting=true` 调用消费"。若此时没有挂起的等待，则退化为非阻塞路径并记 warning，而不是空等到超时。

回合结束与注入的竞态：注入走的是动态感知队列（FEATURE-471），队列在下一次消息构造时才被 drain。因此即使动作恰好在回合结束瞬间到达，也会在下一回合的第一次 LLM 调用前被注入，不会丢失。

### 8.4 前端窗口（本期最小可用）

- 单实例 `#uiWindow`（`web/static/index.html`），样式 `.ui-window`：固定右下角、标题栏 + 关闭按钮 + body。
- `UI.openWindow(title)` / `UI.closeWindow()` / `UI.isWindowOpen()`（`web/static/ui.js`）；`app.js` 消费 `ui_window` 事件，并把 `ui_target="window"` 的树渲染进 `#uiWindowTree`。
- 窗口内节点同样带 `data-ui-id` / `data-ui-tree-id`，因此既有的 `ui_update` 全局寻址（`findByUIID`）与动作上行（`uiTreeIDOf`）对窗口内容**无需改动即可工作**。
- 关闭窗口会清空 body：此后指向窗口内 id 的 `ui_update` 找不到目标，落到既有告警分支（`console.warn`），不会复活游离 DOM。
- 本期不做：多窗口、拖动/缩放/最小化、位置尺寸自定义、刷新或切会话后的恢复。刷新与切会话都会丢弃窗口（`app.js` 在 `session_switch` 前调用 `UI.closeWindow()`）。
  **（v0.85.0 起本段限制由 §10 取代：单窗口 → 多窗口，切会话丢弃 → 按会话恢复；§8.1~8.3 的事件/动作/waiting 协议不变。）**

### 8.5 相关测试

- Go 单测：`agent/ui_window_test.go`（UC-50 打开并 park、UC-51 `target=window` 与事件 Meta、UC-52 `blocking` 往返与 `<ui_action>` 注入、UC-53 等待通道、UC-54 生命周期事件、UC-55 单窗口与标题限长）。
- 浏览器实测用例：`use-case/FEATURE-524/FEATURE-524-UC-0001.md` I 组 UC-50~UC-56。

---

## 9. 矢量绘图：`figure`（FEATURE-570 / BUILD-1154）

### 9.1 一句话模型

`figure` 是一块**逻辑坐标画布**：LLM 用数学坐标描述图元（点/线段/多边形/椭圆/坐标轴/标注/折线/函数曲线），前端把它们投影成一张 SVG 画出来。坐标一律以 **xyz** 为基准——只写 `[x,y]` 时按 2D 直出；任一图元带 z 分量时，整张图按**固定等轴测投影**绘制（视角由 `view` 预设或 `yaw`/`pitch` 指定）。图元是 `figure` 的 **children**，因此天然复用节点的 `id`/`update` 原地刷新与 `actions` 交互。

代码落点：`agent/uitree.go`（白名单与校验）、`web/static/ui-figure.js`（渲染器）、`web/static/ui-expr.js`（函数表达式解析器）、`web/static/style.css`（`.ui-figure` 样式块）。

### 9.2 画布 props

| prop | 类型 | 默认 | 语义 |
| --- | --- | --- | --- |
| `title` | string | — | 图纸标题（左上角） |
| `xmin` / `xmax` | number | `0` / `10` | x 轴显示范围（逻辑坐标） |
| `ymin` / `ymax` | number | `0` / `10` | y 轴显示范围 |
| `zmin` / `zmax` | number | `0` / `10` | z 轴范围（仅 3D 生效） |
| `view` | `front\|top\|side\|iso` | `iso` | 3D 预设视角（仅 3D 生效） |
| `yaw` / `pitch` | number（度） | 预设值 | 自定义视角，覆盖 `view` |
| `axes` | bool | `true` | 是否画默认坐标轴（仍可用 `axes` 图元单独绘制） |
| `grid` | bool | `true` | 是否画网格 |
| `unit` | string | — | 刻度单位后缀 |
| `width` / `height` | number | `720` / `420` | 逻辑画布尺寸（CSS 等比自适应容器） |
| `download` | bool | `true` | 是否显示「下载 SVG」按钮 |

### 9.3 图元目录（8 种，只能作为 `figure` 的 children）

| type | props | 说明 |
| --- | --- | --- |
| `point` | `x`, `y`, `z?`, `label?`, `labelPos?`, `r?`, `filled?` | 点（可带标注） |
| `segment` | `x1`,`y1`,`z1?`, `x2`,`y2`,`z2?`, `arrow?: false\|end\|start\|both`, `label?` | 线段 / 向量（可带箭头） |
| `polygon` | `points: [[x,y,z?], …]`, `closed?`（默认 true）, `fill?`, `opacity?` | 多边形；`closed:false` 即折线 |
| `ellipse` | `cx`, `cy`, `z?`, `rx`, `ry`, `rotation?`, `fill?` | 椭圆；`rx == ry` 即圆 |
| `axes` | `x?`,`y?`,`z?`, `grid?`, `ticks?`, `step?`, `xlabel?`,`ylabel?`,`zlabel?`, `origin?` | 坐标轴；可移到任意原点，与 `figure.axes` 互补 |
| `label` | `x`,`y`,`z?`, `text`, `anchor?: start\|middle\|end`, `dx?`, `dy?`, `size?` | 文字标注 |
| `path` | `points: [[x,y,z?], …]`, `smooth?`, `dash?`, `arrow?` | 折线 / 轮廓 |
| `function` | `expr`, `var?`（默认 `x`）, `from`, `to`, `z?`, `samples?`（≤400）, `exprZ?` | 函数曲线；3D 可给参数方程 |

### 9.4 统一样式 props（所有图元通用）

| prop | 取值 | 说明 |
| --- | --- | --- |
| `stroke` / `fill` | `accent` `info` `warn` `success` `error` `dim` `none` | **只接受语义色名**，映射为 CSS 类，四套主题自动一致（不接受任意 hex） |
| `strokeWidth` | `1..6` | 线宽 |
| `dash` | `solid` `dashed` `dotted` | 线型 |
| `opacity` | `0..1` | 填充/描边透明度 |

### 9.5 坐标系与投影

- **2D**：`xmin/xmax/ymin/ymax` 等比映射到画布（**y 轴向上**，符合数学惯例），四周留内边距；图元超出声明范围时**自动扩边**（`viewBox` 放大），不裁剪、不丢失图形。
- **2D/3D 判定**：整棵 `figure` 子树中只要有一个坐标带 z 分量，就走 3D。
- **3D 投影**：`p' = Rz(yaw) · Rx(pitch) · p`，取 `x'`/`y'` 投影到画布；预设视角 `front(0,0)` / `top(0,90)` / `side(90,0)` / `iso(35,25)`。
- **3D 顺序**：本期**不做深度排序 / 隐藏线 / 光照**，按 children 的声明顺序绘制（后声明的覆盖先声明的），LLM 用声明顺序表达遮挡关系。

### 9.6 函数曲线与安全

`function` 的 `expr` 由 `web/static/ui-expr.js` 求值——**手写词法 + 递归下降**，与项目"不引第三方库"一致：

- **禁用** `eval` / `new Function` / 字符串 `setTimeout`；变量只由调用方注入（`x`、`t` 等），**不暴露任何宿主对象**（`window`/`document`/`constructor` 等一律解析失败）。
- 文法：`expr := term (('+'|'-') term)*` → `term := factor (('*'|'/'|'%') factor)*` → `factor := unary ('^' factor)?` → `unary := ('-'|'+')? primary` → `primary := number | ident '(' args ')' | ident | '(' expr ')'`。
- 函数白名单：`sin cos tan asin acos atan sinh cosh tanh sqrt abs log ln log10 exp floor ceil round min max pow sign atan2`；常量 `pi` `e`。
- 上限：`expr` ≤ 500 字符、采样点 ≤ 400、递归深度 ≤ 32；**解析失败在图内降级为一行提示**（不废掉整张图），采样值非有限则跳过该点。

### 9.7 交互与导出

- 图元可声明 `actions`（`click` / `select`，`payload: point`），走既有 `ui_action` 上行通道；配合 `render_ui(waiting=true)` 可在同一回合内继续。
- 「下载 SVG」：`XMLSerializer` + `Blob` + `a[download]`，纯前端、无新增接口；导出前用 `getComputedStyle` 把主题色**内联为实际颜色值**，脱离 CSS 也能打开。

### 9.8 校验（后端）

- 白名单新增 `figure` 与 8 种图元；**图元类型只在 `figure` 子树内合法**（`validateUINode` 增加父类型上下文），散落在 `card` 等处会被拒。
- 数值必须有限（拒 NaN/Inf）；`rx`/`ry` ≥ 0；`opacity ∈ [0,1]`；`strokeWidth ∈ (0,6]`。
- 规模：图元节点 ≤ 200（复用 `UIMaxTreeNodes`）、单图元点数 ≤ 400；错误信息 i18n 中英同步、可自纠。

### 9.9 降级

| 出口 | 行为 |
| --- | --- |
| 未注册 `figure` 的前端 bundle | `ui.js` 的未注册类型降级路径：原样显示 JSON 文本块 |
| 终端 / 飞书 | 忽略 `ui_render` 事件，继续显示 LLM 的纯文本回复 |

### 9.10 示例

```json
{"type":"figure","props":{"title":"单位圆与正弦","xmin":-1.5,"xmax":7,"ymin":-1.5,"ymax":1.5,"axes":true,"grid":true},
 "children":[
   {"type":"axes","props":{"origin":[0,0],"xlabel":"x","ylabel":"y"}},
   {"type":"ellipse","props":{"cx":0,"cy":0,"rx":1,"ry":1,"stroke":"accent"}},
   {"type":"point","props":{"x":1,"y":0,"label":"A(1,0)","filled":true}},
   {"type":"segment","props":{"x1":0,"y1":0,"x2":1,"y2":0,"arrow":"end","stroke":"info"}},
   {"type":"function","props":{"expr":"sin(x)","from":0,"to":6.28,"samples":160,"stroke":"warn"}}
 ]}
```

3D 只需给坐标补上 z：`{"type":"point","props":{"x":1,"y":1,"z":1,"label":"P"}}`。

### 9.11 相关测试与用例

- 用例：`use-case/FEATURE-570/FEATURE-570-UC-XXXX.md`（基础渲染 / 坐标系 / 3D / 函数曲线 / 安全鲁棒 / 集成兼容 六组）。
- 单测：`agent/uitree_test.go`（白名单、图元上下文、数值与规模上限）。

---

## 10. 主信息区覆盖面板与多窗口（FEATURE-571 / v0.85.0）

> 本节把 §8 的「单实例右下浮窗」扩展为**多窗口 + 面板形态**：窗口不再只有一个，且可以落在**覆盖主信息区**的面板上（与文件预览同一区域、层级在其下）。§8.1~8.3 的事件/动作/waiting 协议继续有效；§8.4「本期不做」中关于单窗口与切会话丢弃的限制由本节取代。

### 10.1 三种呈现面（target / surface）

| 呈现面 | 触发 | 位置与行为 |
| --- | --- | --- |
| 对话流内联块 | `render_ui(tree)`（默认） | 作为消息流里的一块，随内容上卷 |
| 右下角浮窗 | `render_ui(target="window")` 或窗口 `surface=float` | 固定右下角，不遮挡主信息区（§8 既有形态，样式复用 `.ui-window`） |
| 主信息区面板 | `render_ui(target="panel")` 或窗口 `surface=panel` | **铺满主消息区**（覆盖层，与 `.file-viewer` 同区域），位置固定、不随内容上卷 |

- `target` 与窗口的 `surface` 是同一概念的两种入口：`target` 是「这次渲染往哪个面去」，`surface` 是「这个窗口本身是什么形态」。二者不一致时以本次调用为准（渲染会顺带把窗口切到对应形态）。
- **与文件/图片预览并存**：面板 `z-index` **低于** `.file-viewer`。预览打开时临时盖住面板，关掉预览后露出面板（面板本身不被关闭）；每个窗口/层各自有独立关闭按钮。

### 10.2 多窗口模型与寻址

窗口用**名字**寻址，缺省名为 `default`。

| 操作 | 工具 | 说明 |
| --- | --- | --- |
| 创建 / 命名 / 改形态 | `ui_window(meta, action="open", title?, window?, surface?)` | `window` 缺省 `default`；同名窗口**不叠加**，而是激活并更新标题/形态 |
| 关闭 | `ui_window(meta, action="close", window?)` | `window` 缺省关闭**当前活动窗口** |
| 写内容 | `render_ui(meta, tree, target="panel"\|"window", window?, update?)` | `window` 缺省写入**当前活动窗口**；窗口不存在时自动按 `target` 建一个并激活 |

- 窗口记录：`Name` / `Title` / `Surface`（`panel`\|`float`）/ `Active` / 内容摘要（最近一次渲染的根节点类型与 id）/ 打开时间。
- **上限 8 个窗口**（`agent.UIMaxWindows`）：超出时 `ui_window(open)` 被拒并返回 i18n 错误，不静默丢弃。
- 窗口**不阻塞**回合：除 `waiting=true` 外，agent 可边跑边推送。
- 窗口名约束：长度 1~32 个字符；禁止控制字符与 `<` `>` `&` `"` `'` `\`；首尾不允许空白。合法示例：`default`、`行情看板`、`build-log`。

### 10.3 面板结构与交互

面板 DOM 位于 `#stream-wrap` 内，与 `.file-viewer` 同级：

```html
<div id="uiPanel" class="ui-panel">
  <div class="ui-panel-head">
    <div id="uiPanelTabs" class="ui-panel-tabs"><!-- 每个窗口一个 tab --></div>
    <button id="uiPanelFloat">收成浮窗</button>   <!-- 切换 surface: panel ↔ float -->
    <button id="uiPanelClose">✕</button>          <!-- 关闭活动窗口 -->
  </div>
  <div id="uiPanelBody" class="ui-panel-body"></div>
</div>
```

| 交互 | 行为 |
| --- | --- |
| 点击 tab | 切换活动窗口（只切显示，不改各自内容与滚动位置） |
| 左右滑动（指针/触摸横向拖拽） | 同上；窄屏（≤720px）隐藏 tabs 后仍保留此切换入口 |
| 窄屏 ≤720px | 隐藏 tabs 条，头部保留 **≡ 窗口列表按钮** 作为显式切换入口 |
| 收成浮窗 | 活动窗口 `surface: panel → float`：面板隐藏、该窗口出现在右下角；再次点击可切回面板 |
| ✕ | 关闭**该窗口自己**（面板与浮窗各自有独立关闭按钮） |
| `Esc` | 关闭**当前活动窗口**（面板打开时优先于其它 Esc 行为） |
| 多窗口 | 多个面板/浮窗可同时存在；同一时刻有且仅有一个「活动窗口」（tabs 高亮、Esc 与 ✕ 的作用对象） |

- 面板**一律铺满主消息区**（不做 2/3、全屏分档）。
- 本期**不做**：窗口拖动/缩放、位置尺寸自定义、后端持久化窗口。

### 10.4 事件与 Meta 扩展

| 事件 | 通道 | Meta |
| --- | --- | --- |
| `ui_window` | `ChannelSystem` | `ui_window_action` = `open`/`close`/**`activate`**；`ui_window_name`；`ui_window_surface`（`panel`/`float`，仅 open）；`ui_window_title`（仅 open） |
| `ui_render` / `ui_update` | `ChannelSystem` | 原有 `ui_id` + `ui_tree`/`ui_patch`；`ui_target`（**非 stream 时才有**）+ `ui_window_name`（非默认窗口时才有） |

- 旧行为保持不变：`target` 缺省（stream）时事件形状与既有用例完全一致；终端等无窗口出口只输出降级文本，不报错。

### 10.5 `<environment_details>` 的 `<ui_windows>` 清单

`agent/envelope.go` 的 `buildOpenedResources()` 在 `<opened_resources>` 内追加：

```xml
<ui_windows>none</ui_windows>
<!-- 或 -->
<ui_windows>
  - default | panel | active | 行情看板 | card(ui-3)
  - log | float | | 构建日志 | table(ui-9)
</ui_windows>
```

- 每行：`名字 | 形态 | 是否活动 | 标题 | 内容摘要`；无窗口时为 `none`。
- `ui_window` 与 `render_ui` 的工具说明中写明：**先读该清单**再决定「新建窗口」还是「更新已有窗口」，避免重复 `open` 出多个同类窗口。

### 10.6 跨会话恢复（localStorage）

- 按**会话 id** 分键存储：窗口清单 + 每个窗口**最后渲染的整棵树**。
- 切换会话：不再无条件 `UI.closeWindow()`；先按新会话 id 还原窗口清单与各窗内容，再与 `history` 重放出的 `ui_render`/`ui_window` 事件**按窗口名合并**（重放优先，它是更权威的时序）。
- 刷新页面：连接建立后按会话 id 还原。
- 后端**不持久化窗口**——「窗口」是前端的展示态，后端只维护会话级窗口清单用于生成 `<ui_windows>`。

### 10.7 前后端清单同步

前端在任何改变清单的动作后上行一条客户端消息：

```json
{"type":"ui_window_state","windows":[{"name":"default","title":"…","surface":"panel","active":true}],"closed":["log"]}
```

| 触发 | 同步内容 |
| --- | --- |
| ✕ / `Esc` 关闭 | 关闭的窗口名（`closed`） |
| 切换 tab / 左右滑动 | 新的活动窗口名（`windows[].active`） |
| 收成浮窗 / 展开为面板 | 该窗口的 `surface` |
| 连接建立 / 切换会话 | **全量**清单 |

后端据此维护会话级清单（`SetUIWindowClosed` / `SetUIWindowActive` / `SetUIWindowSurface`），保证 `<ui_windows>` 与用户所见一致。

### 10.8 校验与错误（i18n）

| 场景 | 结果 |
| --- | --- |
| `target` 非法 | 拒绝，`KeyUIErrTarget` |
| `surface` 非法 | 拒绝，新增错误键（仅允许 `panel`/`float`） |
| `window` 名非法（长度/字符集） | 拒绝，新增错误键 |
| `close`/`render_ui` 指向不存在的窗口（且非自动创建） | 拒绝，新增错误键（窗口不存在） |
| 新建窗口数超过 8 | 拒绝，新增错误键（超上限） |

### 10.9 相关测试与用例

- 单测：`agent/ui_tools_test.go`、`agent/ui_window_test.go`（新增：`target=panel`、窗口名解析与非法值、`surface` 校验、`<ui_windows>` 注入、超上限拒绝、清单状态同步），并保持既有 `ui_prune` / `ui_render_emit` / `ui_wait` 全绿。
- 浏览器实测用例：`use-case/FEATURE-571/FEATURE-571-UC-XXXX.md`。
