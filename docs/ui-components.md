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

## 5. 组件目录（10 个）

| type | 关键 props | 说明 |
| --- | --- | --- |
| `card` | `title?`, `subtitle?`, `icon?` | 容器；承载标题栏与子节点 |
| `kv` | `items: [{k, v}]` | 键值明细（适合"要点/参数"紧凑展示） |
| `callout` | `variant: info\|warn\|success\|error`, `title?`, `text` | 提示块；用于结论与告警 |
| `progress` | `value`, `max?`, `label?` | 进度条 |
| `table` | `columns: [{key, label?, align?, width?}]`, `rows: [{列key: 值}]` | 明细表格 |
| `chart` | `kind: bar\|line\|pie`, `unit?`, `series: [{name, data:[{label, value}]}]` | 手写 SVG 图表，零依赖 |
| `steps` | `items: [{title, desc?, status: done\|active\|pending}]` | 步骤/时间线 |
| `file` | `path`, `name?`, `size?` | 文件卡片；`/api/open`、`/api/reveal` 动作 |
| `form` | `title?`, `fields: [{name, label, type, options?, value?}]`, `submit` | 表单；提交后开启新一轮回合 |
| `html` | `content` | **逃生舱**，沙箱 iframe 内运行（见 §6） |

新增组件 = 在 `web/static/ui.js`（或 `ui-chart.js`/`ui-form.js`）里
`UI.register("name", {render(node, ctx)})`。协议、Go 校验与 agent 主循环**无需改动**；
未注册类型在前端优雅降级为原始 JSON 文本块。

## 6. 安全模型

### 6.1 声明式组件：不做 HTML 拼接

`ui.js` / `ui-chart.js` / `ui-form.js` 与 `md.js` 遵守同一条铁律：**永不赋值 innerHTML，
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
