# co-shell Web UI 设计文档

> 读者对象：后续接手 web 界面（`co-shell serve`）新功能开发或功能改进的工程师。
> 本文档自包含——不需要再查阅其他资料即可开展工作。文中所有 `文件:行号` 引用以
> BUILD-426（v0.7.7）为准；行号会随后续改动漂移，以符号名为准。

---

## 1. 概述

Web UI 是 co-shell 的第三种交互界面（与终端 TUI、stdio 并列），通过
`co-shell serve` 启动：内嵌 HTTP 服务器（仅监听 127.0.0.1）+ 浏览器页面，
页面与后端之间用一条 WebSocket 交换结构化事件。Go 侧零第三方依赖
（WebSocket 协议为手写的 RFC 6455 实现），前端零构建步骤（原生
HTML/CSS/JS，无框架、无打包器）。

演进历史（细节见 ROADMAP.md 对应条目）：

| 里程碑 | BUILD | 内容 |
| --- | --- | --- |
| FEATURE-307a | 413 | 事件语义化：`StreamEvent` 结构 + `LineRenderer`，事件不再含装饰字符 |
| FEATURE-307b | 415 | `SessionIO` 抽象 + REPL 解耦 + `--output-format json`（JSON-Lines） |
| FEATURE-307c | 417 | `co-shell serve` + web 界面全量交付（本系统的主体） |
| FEATURE-359 | 418 | 布局重构（侧栏贯通到底）、贝壳 logo 马赛克、拖放上传简化 |
| FEATURE-360 | 419 | logo 穹顶形状微调 |
| FIX-361 | 420 | 修复事件流被 flex 压缩成细条、无法滚动（`.ev` 加 `flex-shrink:0`） |
| FEATURE-362 | 421 | 工具调用合并单块（`Meta["phase"]`）+ Markdown 实时渲染（`md.js`） |
| FIX-363 | 422 | 主题兜底：无 `matchMedia` 时默认深色 |
| FEATURE-364 | 424 | 本文档 |
| FEATURE-365 | 425 | 底部通栏（logo 入底栏左端）、去附件按钮、右上角下拉菜单（面板开关+系统设置）、主题三态 auto |
| FEATURE-366 | 426 | 计划面板标题/条目 accent 高亮、菜单与主题按钮对调、logo 换 7×14 新图样 + favicon.svg、标签标题=工作区路径 |

---

## 2. 总体架构

```
┌────────────────────────── co-shell 进程 ───────────────────────────┐
│                                                                    │
│  main.go                                                           │
│   │  解析 "serve" 子命令 → web.NewServer → srv.Listen(port)        │
│   │  repl.RegisterSessionFactory("web", srv.SessionFactory())      │
│   │  inputMode = "web" → REPL 主循环（不变）                        │
│   ▼                                                                │
│  repl.REPL ── SessionIO 接口 ──► web.WebSession (session.go)       │
│   │                                │  ├─ WebIO (agent.UserIO)      │
│   │                                │  │   Print* → ui_text 事件    │
│   │                                │  │   ReadLine/ReadKey → ask   │
│   │                                │  └─ WebRenderer               │
│   │                                │      (agent.EventRenderer)    │
│   ▼                                ▼                               │
│  agent.Agent ──StreamEvent──► web.Server (server.go)               │
│   ▲                                │  单客户端 WebSocket hub       │
│   └──── input/answer/interrupt ────┘  + HTTP API (tree/upload/...) │
│                                    │                               │
│         web/ws.go：手写 RFC 6455 WebSocket 帧编解码                 │
└────────────────────────────────────┼───────────────────────────────┘
                                     │ HTTP + WebSocket (127.0.0.1)
┌────────────────────────────────────┼───────────────────────────────┐
│  浏览器                             ▼                               │
│   index.html  布局骨架（CSS Grid：侧栏/事件流/任务面板/底部输入）     │
│   style.css   双主题 CSS 变量 + 全部样式                             │
│   app.js      WS 客户端、事件渲染、目录树、上传、任务面板、ask、输入   │
│   md.js       Markdown 子集渲染器（手写、DOM 构建、防 XSS）           │
│   favicon.svg 像素风标签图标（FEATURE-366，与 logo 同一 7×14 图样）    │
└───────────────────────────────────────────────────────────────────┘
```

关键设计决策：

1. **REPL 主循环不变**。web 只是又一种 input mode。`repl.SessionIO`
   （`repl/session.go:28`）是唯一的接入点；web 包通过
   `repl.RegisterSessionFactory("web", ...)` 注册自己，避免
   `repl ↔ web` 循环导入（web 引用 repl，repl 永不引用 web）。
2. **事件即协议**。`agent.StreamEvent`（`agent/events.go:46`）是唯一
   下行数据单元，web 前端、JSON-Lines、终端 LineRenderer 消费同一事件流。
3. **单客户端**。hub 只保留一条浏览器连接，新连接顶替旧连接
   （`server.go:185` setConn）。这是单机工具，不是多用户服务。
4. **零依赖红线**。不得引入任何新的第三方 Go 模块或前端库/CDN。
   WebSocket 手写、Markdown 渲染器手写，都是这一红线的直接结果。

---

## 3. 启动链路

### 3.1 `serve` 子命令解析（main.go:216-242）

stdlib `flag` 会把 `serve` 当成位置参数混进单命令字符串，所以
`parseFlags` 在 flag 解析**之前**手工扫描 `os.Args`：

- 跳过每个 flag 的值（bool flag 白名单除外），避免 `-w serve`（工作区
  恰好叫 serve）被劫持；
- 遇到第一个位置参数若是 `serve`，置 `f.serve = true` 并把它从
  `os.Args` 中剔除，然后正常 `flag.Parse()`。

### 3.2 冲突检查（main.go:479-493）

serve 与以下选项互斥，冲突即报错退出（i18n key `serve_conflict`）：

- 单命令模式（`--command` / 位置参数）
- `--input-mode`
- `--output-format`

### 3.3 服务启动（main.go:1425-1443）

```go
srv := web.NewServer(ws.Root(), web.ServerOptions{Lang, Version, Build})
repl.RegisterSessionFactory("web", srv.SessionFactory())
addr, err := srv.Listen(flags.port)   // 127.0.0.1，端口占用自动递增，最多 10 个
web.OpenBrowser("http://" + addr)     // 失败仅警告（serve_browser_failed）
inputMode = "web"
```

之后 `r.SetInputMode("web")`、REPL 照常 `r.Run()`。REPL 通过
`sessionFactories["web"]` 拿到 `WebSession`，主循环完全不感知 web 的
存在。i18n key：`serve_started` / `serve_no_port`。

### 3.4 降级链背景

非 web 的链路：默认 `tui`；raw terminal 不可用时 REPL 内自动降级
`stdio`（`repl/repl.go:269`）。`--output-format json` 隐含 stdio。
（"裸启动自动进 serve"的 auto 链在讨论中，尚未实现——实现时见第 10
节的注意事项。）

---

## 4. Go 后端：web 包

包结构（`web/`，共 ~1500 行）：

| 文件 | 职责 |
| --- | --- |
| `server.go` | HTTP 路由、单客户端 WS hub、工作区文件 API |
| `ws.go` | 手写 RFC 6455 WebSocket（握手/帧/分片/ping-pong/close） |
| `session.go` | REPL 接入：WebSession / WebIO / WebRenderer |
| `open.go` | 打开浏览器/文件/文件管理器（按 GOOS 分发，变量可注入以便测试） |
| `static/` | 前端五件套（embed.FS 内嵌进二进制） |
| `*_test.go` | 单测（WS 握手与帧、API、session 行为） |

### 4.1 HTTP 路由（server.go:118-126）

| 路由 | 说明 |
| --- | --- |
| `GET /` | `static/index.html`（从 embed.FS 读） |
| `GET /static/*` | 前端静态资源（`http.FileServer(http.FS(staticFS))`） |
| `GET /ws` | WebSocket 升级端点 |
| `GET /api/bootstrap` | `{lang, version, build, workspace}`，前端启动时拉取 |
| `GET /api/tree` | 工作区目录树 JSON（见 4.3） |
| `POST /api/upload?dir=<rel>` | multipart 上传，单文件 ≤100MiB |
| `POST /api/open` | `{path}` → 用 OS 默认程序打开文件 |
| `POST /api/reveal` | `{path}` → 在文件管理器中定位 |
| `GET /api/file?path=<rel>` | 读取文件内容（前端图片预览用） |

### 4.2 路径安全（server.go:304-311）

所有涉及文件系统的 API 都过 `resolvePath`：`filepath.Join(root, rel)`
后做 `filepath.Rel` 校验，任何 `..` 逃逸一律 `errPathOutside`（403）。
上传另做 `filepath.Base` 防文件名注入。**新增文件 API 必须复用
`resolvePath`**，这是唯一的安全闸。

### 4.3 目录树（server.go:349-387）

- 递归构建，深度上限 8 层，每层最多 500 个条目；
- `treeExcludedNames` 隐藏噪声目录：`.git` / `node_modules` / `db` /
  `log` / `tmp`（`output/` 刻意保留）；
- 排序：目录在前，名称升序；
- 节点结构：`{name, path(工作区相对), dir, children?}`。

### 4.4 单客户端 WS hub（server.go:185-296）

- `setConn`：新连接顶替旧连接；旧连接关闭并触发 `onDisconnect` 钩子
  （让挂起的 ask 立即失败，见 4.6）。
- `sendJSON` / `sendEvent` / `sendAsk`：无客户端时返回 false（调用方
  据此报错而不是傻等）。
- `sendState`：仅在连接建立时推送一次任务计划快照（`planFn` 由
  WebSession 提供）；运行中的计划更新走 `task_plan` 事件。
- `handleWS`：升级 → setConn → sendState → 循环 `ReadMessage` →
  `dispatch` 到 `handler`（由 WebSession 注册）。

### 4.5 ws.go：手写 WebSocket

- 握手：`Sec-WebSocket-Accept = base64(sha1(key + GUID))`，
  `http.Hijacker` 夺取连接后手写 101 响应；
- 帧解码：支持 16/64 位扩展长度、client mask 去掩码（mask 与否都接受，
  因为仅 loopback）、单帧/单消息上限均 4MiB；
- 分片重组（continuation）、ping 自动回 pong、close 回 close 帧后
  返回 `io.EOF`；
- 写帧用 `wmu` 互斥（事件推送和 ask 来自不同 goroutine）。

**不要**把它替换成 gorilla/websocket——零依赖红线（docs/DEV-HANDOVER.md
第六节第 8 条）。

### 4.6 session.go：REPL 接入三件套

**WebSession**（`SessionIO` 实现）：

- `newWebSession` 把 `WebIO` 装到 agent 上并**保持整个会话生命周期**
  （这样 REPL/builtin 的 `io.Print*` 输出也能到达浏览器，而不是只覆盖
  单次运行）；向 server 注册消息 handler、断连钩子、计划快照提供者。
- `ReadLine(prompt)`：阻塞等浏览器 `input` 消息；附件路径先过
  `resolvePath` 再 `ag.SetImagePaths`（与 CLI `--image` 同路径）。
- `Acquire(ag)`：返回 `WebRenderer` + 空 release（WebIO 已常驻）。
- `Interactive() = false`：页面自带 UI，终端装饰（欢迎横幅、prompt、
  Said 行）全部抑制。
- `handleMessage`：`input` → inputCh；`answer` → WebIO.resolve；
  `interrupt` → `ag.Interrupt()`（等价于终端的 ESC）。

**WebIO**（`agent.UserIO` 实现）：

- `Print*` → 包成 `ui_text` 事件（system 频道）推送；
- `ReadLine/ReadKey` → 发 `ask` 消息（mode `line`/`key`），带自增 id
  阻塞等匹配 answer；空 answer 的 ReadKey 映射为 Enter；
- 无客户端时立即返回 `errNoWebClient`（否则 agent 会永远卡在等待
  回答上）；断连钩子 `failAll` 让所有挂起 ask 失败。

**WebRenderer**（`agent.EventRenderer` 实现）：`Render(ev)` 原样推送，
不做任何分拣——分块/渲染策略全部在前端（这是有意设计：Go 侧重发，
前端重展示策略，改 UI 不碰 Go）。

### 4.7 open.go：OS 集成

按 `runtime.GOOS` 分发（不需要 build tag）：

| 功能 | macOS | Windows | Linux |
| --- | --- | --- | --- |
| 浏览器 | `open <url>` | `rundll32 url.dll,FileProtocolHandler` | `xdg-open` |
| 打开文件 | `open <path>` | 同上 | `xdg-open` |
| 定位文件 | `open -R` | `explorer /select,` | `xdg-open <所在目录>`（无可移植 reveal） |

三个函数是包级变量（`openBrowserFunc` 等），测试注入 fake，不会真的
起进程。新增 OS 集成沿用这个模式。

---

## 5. 通信协议

### 5.1 下行（server → 浏览器），三种 kind

```jsonc
// 事件流（主体）
{"kind":"event","event":{"type":"content_chunk","chan":"llm","text":"...",
                         "level":"warning","meta":{"phase":"input"}}}
// 交互请求（agent 需要用户输入时）
{"kind":"ask","id":"ask-7","mode":"line"}   // mode: "line" | "key"
// 连接建立时的状态快照（仅一次）
{"kind":"state","plan":{...}}               // plan 为 null 表示无计划
```

`eventJSON` 字段规则（与 JSON-Lines StreamRenderer 一致）：`level` 仅
非 info 时出现，`chan` 为空时省略，`text`/`meta` 为空省略。

### 5.2 上行（浏览器 → server），三种 type

```jsonc
{"type":"input","text":"用户指令","attachments":["相对/路径.png"]}
{"type":"answer","id":"ask-7","value":"用户回答"}
{"type":"interrupt"}                        // 等价终端 ESC
```

注：`attachments` 字段协议保留（服务端照常处理），但 FEATURE-365 起
官方前端已移除附件入口，实际只发送 `text`。

### 5.3 StreamEvent 类型速查（agent/events.go）

| type | 含义 | 前端处理（app.js renderEvent） |
| --- | --- | --- |
| `content_chunk` / `content` | LLM 正文（流式/整段） | 累积进 LLM 块，MD 渲染 |
| `thinking_chunk` / `thinking` | LLM 思考 | 累积进 THINK 块，MD 渲染 |
| `tool_call_stream` | 工具参数流式片段 | 累积进当前 TOOL 块（纯文本） |
| `tool_call` | 工具调用摘要/结果 | 按 `meta.phase` 合并（见下） |
| `command` / `output` | 命令回显/输出 | 独立 CMD 块，等宽纯文本 |
| `token_iter` / `token_task` | token 用量 | 一行 meta 小字 |
| `task_plan` | 任务计划快照（`meta.plan` 为 JSON） | 更新右侧面板，不进事件流 |
| `info`/`warning`/`error` | 通知 | 普通块，level 驱动配色 |
| `ui_text` | WebIO Print* 输出 | 普通 SYS 块 |
| `done` | 一轮结束 | 复位所有流式累加器 |

**tool_call 的 phase 标记**（FEATURE-362）：一次工具调用按序发出
`phase=input`（执行前摘要）→ `phase=result`（执行后结果）。前端规则：
流式片段累积 → input 替换之（片段是半截 JSON/XML，无保留价值）→
result 追加进同一块。error 频道为 tool 的 `error` 事件同样追加进当前
TOOL 块并标红。

---

## 6. 前端

五个文件，无构建步骤，直接以 ES5+ 原生语法书写（不依赖任何框架/库，
不引入 CDN）。

### 6.1 index.html：布局骨架

CSS Grid 双行三列（`style.css` `#layout`）：

```
┌──────────┬────────────────────┬──────────┐
│ sidebar  │  #stream-wrap      │ plan-    │
│ (工作区   │  (事件流，grid 主区) │ panel    │
│  目录树)  │                    │ (任务进展)│
├──────────┴────────────────────┴──────────┤
│ #bottom（通栏）：logo + ask 区 + 输入行    │
└──────────────────────────────────────────┘
```

- 侧栏只占第一行（`grid-row: 1`）；底部 `#bottom` 通栏
  （`grid-column: 1/-1`），其上边界即工作区清单的下边界（FEATURE-365）；
- logo 马赛克在 `#bottom` 内最左端、与录入框同一外框（右缘细分隔线），
  贝壳图案手工绘制于 7×14 网格（`app.js` LOGO_ART，FEATURE-366 起换用
  穹顶+流苏中缝+下碗的新图样），`#` 格渲染 accent 色，高度动态约束
  不超过输入行；同一图样的像素版即 `favicon.svg`（64 个 1×1 rect、
  `shape-rendering:crispEdges`、accent 青），改 LOGO_ART 时应同步重生成；
- 面板可见性 class 驱动：`#layout.no-plan` 第三列归零、
  `#layout.no-ws` 第一列归零（两者可叠加，见 style.css 组合规则）；
- 顶栏右侧：连接状态 → 明暗主题按钮 → ☰ 下拉菜单（悬停展开：工作区/
  任务进展开关 + 系统设置，FEATURE-366 起菜单在最右端）。

### 6.2 style.css：双主题

`:root`/`[data-theme="dark"]` 与 `[data-theme="light"]` 两套 CSS 变量
（`--bg/--bg-panel/--bg-elev/--mono-bg/--fg/--fg-dim/--fg-faint/--accent/
--border/--ok/--warn/--err/--glow`）。**新增样式一律用变量**，明暗主题
自动适配。`.ev-body.md` 下有整套 Markdown 元素样式。

### 6.3 app.js：模块地图（按注释分节）

1. **i18n**：`I18N.zh/en` 字典 + `applyI18n()`（`data-i18n` /
   `data-i18n-ph` 属性驱动）；语言由 `/api/bootstrap` 下发。
2. **主题**（FEATURE-365 三态）：localStorage `co-shell-theme` =
   `auto`（默认，跟随 OS，`matchMedia change` 实时响应）/ `dark` /
   `light`；无 `matchMedia` 时 auto 兜底深色（FIX-363）。`setTheme`
   只应用不落盘，避免 resolved 值覆盖 `auto`；顶栏 ☾/☀ 按钮写入
   手动值，系统设置弹层里的下拉可切回 auto。
3. **WebSocket**：`wsConnect()` 自动重连（2s）；`onmessage` 按 kind
   分发 event/ask/state。
4. **事件渲染**（核心，见 6.4）。
5. **任务面板**：`renderPlan(plan)` 缓存 `lastPlan` 并交给
   `applyPanels()` 统一判定可见性，状态图标 ○◐●✕✗。FEATURE-366 起
   计划标题与条目文字同字号、仅靠 accent 高亮 + 间距区分层级。
6. **面板开关**（FEATURE-365）：`panelPrefs`（localStorage
   `co-shell-panels` = `{ws, plan}`，缺省为显示）+ `applyPanels()`；
   菜单项点击翻转偏好并持久化，勾选态（`.mi-check.on`）随动。
7. **ask 区**：`showAsk/hideAsk/answerAsk`；key 模式渲染按键按钮组
   （Enter/c/a/g/d/n），line 模式渲染输入框。
8. **输入行**：Enter 发送 / Shift+Enter 换行 / ↑↓ 历史 / 自动增高。
   FEATURE-365 起不再支持附件（📎 按钮与 chips 已移除），input 消息只
   带 `text`。历史导航（FIX-367）：`histPos === history.length` 是
   "未发送草稿"哨兵；↑ 仅在光标位于首行、↓ 仅在末行时才翻历史（多行
   草稿内垂直移光标不受影响），唤回条目的光标停在末尾，翻到最新一条
   后再按 ↓ 恢复草稿；历史为空时 ↑↓ 完全不碰输入内容。
9. **目录树**：`loadTree/treeNode`，目录点击展开/折叠；文件点击：
   图片 → 浮层预览（`/api/file`），其他 → `/api/open`；悬停 ⌖ 按钮
   → `/api/reveal`。
10. **上传/拖放**（FEATURE-359）：唯一上传入口是拖放——拖到目录行
    上传进该目录（`stopPropagation` 防冒泡），拖到侧栏其他位置上传进
    根目录；侧栏整体弱高亮 + 目标行高亮。
11. **系统设置弹层**（FEATURE-365）：`#settings` modal，目前含主题
    三态下拉；新增客户端设置项往这里加。
12. **boot**：拉 `/api/bootstrap` → 用 workspace 设置
    `document.title`（浏览器标签 = favicon + 工作区路径，FEATURE-366）
    → applyI18n → applyPanels → loadTree → wsConnect。

### 6.4 事件渲染机制（app.js renderEvent，FEATURE-362 重构）

**流式累加器**：`curLLM / curThinking / curTool` 三个指针，结构
`{body, raw, raf, hasResult}`。`raw` 累积未装饰原文；任何非本流事件
到达时对应指针复位；`done` 全部复位。

**Markdown 渲染**：`scheduleMd(b)` 用 `requestAnimationFrame` 节流，
每帧对 `raw` **整体重解析**（不是增量）——半截语法（未闭合围栏、
 dangling `**`）自然呈现为中间态，下一片段到达自愈。长文档全量重解析
 的性能足够（KB 级文本）。

**TOOL 块合并**：见 5.3。判定新块的规则：`phase=input` 到达时若
 `curTool` 不存在或已有结果（`hasResult`）→ 新开块，否则替换当前块
 内容（流式片段 → 干净摘要）。

**FIX-361 的教训**：`.stream` 是 flex 纵向容器，子元素 `.ev` 必须
 `flex-shrink: 0`，否则内容超高时 flex 会等比压缩所有子块而不是溢出
 滚动。新增直接子元素时注意同样处理。

### 6.5 md.js：Markdown 子集渲染器（209 行）

- **支持**：围栏代码块（```/~~~，含语言 class）、ATX 标题（h1 渲染为
  h3 以保持流内尺度，原级别存 `data-level`）、管道表格、引用（递归
  解析）、hr、无序/有序列表（单级）、段落（行间 `<br>`）、行内
  code/粗体/斜体/删除线/链接。
- **安全**：全程 `createElement`/`createTextNode` 构建 DOM，原始输入
  永不进 `innerHTML`；链接仅放行 `http(s):`/`mailto:` scheme，其余按
  字面文本。
- **已知边界**：`_emphasis_` 要求非单词字符相邻（保护 snake_case）；
  不支持嵌套列表、图片、Setext 标题。扩展新语法时遵循同一模式：
  块级在 `mdBlocks` 主循环加分支（**每个分支必须推进 `i`**——标题
  分支曾漏 `i++` 造成死循环），行内在 `MD_INLINE_RE` 加 alternation
  并在 `mdInline` 加对应分支（注意 alternation 顺序即优先级，code
  span 必须在最前）。

---

## 7. 背景知识：事件流与 REPL 体系

改动 web 之前需要理解的三个既有抽象（详见
docs/output-architecture.md，本文只给最小必要集）：

1. **`agent.StreamEvent`**：agent 主循环的唯一输出口。`Type`（枚举
   常量）、`Chan`（业务频道）、`Level`（重要级）、`Text`（纯语义，
   无装饰）、`Meta`（结构化数据，渲染器可忽略）。
2. **`agent.EventRenderer`**：事件的消费端。三个实现：
   `LineRenderer`（终端装饰行）、`StreamRenderer`（JSON-Lines）、
   `WebRenderer`（WS 推送）。
3. **`repl.SessionIO`**：REPL 的输入/装配抽象，三个实现：stdio /
   tui / web。REPL 主循环面向接口编程，不分支模式。

另有两条硬约束：

- **audit 基线**（`bin/output_audit.sh --strict`）：存量 143 处裸
  fmt 输出 / 0 魔法字符串事件 / 2 处硬编码中文 / 18 处同步阻塞读 /
  0 i18n 缺失。exit 1 是存量固有，**你的改动不得让任何一项数字变大**；
  新增用户可见字符串必须走 i18n（`i18n/keys.go` + zh/en 双语）。
- **golden 文件**（`repl/testdata/render_tui.golden`、
  `testdata/render_single_cmd.golden`）：终端渲染基线，绝不使用
  `-update` 重新生成；web 改动本不应触碰它们（碰到了说明改错了层）。

---

## 8. 测试方法论

### 8.1 Go 侧（web 包已有单测，新功能必须补）

- `ws_test.go`：握手、帧编解码（含 mask、扩展长度、分片、大小上限）；
- `server_test.go`：API 端点（树、上传、路径逃逸 403、open/reveal
  用注入的 fake launcher）；
- `session_test.go`：WebSession/WebIO 行为（ask 往返、断连 failAll、
  无客户端报错）。
- 跑法：`go test ./web/`；全量 `go build ./... && go vet ./... &&
  go test ./...` 必须全绿。

### 8.2 前端逻辑：Node + 最小 DOM shim（无浏览器断言）

headless Chrome 在本机不稳定时，用 Node 加载**真实的** md.js/app.js
跑断言（FEATURE-362 验证过 17 项）：

```js
// 要点：实现 El/TextNode 两个类（appendChild/classList/textContent/
// dataset/style）、document.createElement、requestAnimationFrame 收
// 集成队列手动 flush（同步执行的 shim 会失真——rAF 回调里 b.raf=null
// 发生在赋值返回之前）。然后 eval 两个文件（它们以 "use strict" 开头，
// 声明不外泄，需要在 eval 串尾加 globalThis.X = X 导出入口）。
```

驱动假事件过 `renderEvent`，断言块数量、合并行为、MD 产物结构。

### 8.3 前端视觉效果：headless Chrome 截图

```bash
# 静态 harness 页（引用真实 style.css/md.js/app.js + 内联驱动脚本），
# python3 -m http.server 起本地服务，然后：
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --headless --disable-gpu --user-data-dir=<临时目录> \
  --screenshot=/tmp/shot.png --window-size=1000,1200 <url>
# 进程退出可能挂住，但截图文件会先写盘——等文件出现后 pkill 即可。
```

### 8.4 真机冒烟

`go build -o work/co-shell .` 后 `work/co-shell serve`，浏览器实际跑
一条指令，观察流式渲染、工具块合并、ask 应答、打断按钮。

---

## 9. 开发规范（与全项目一致）

1. **分支**：`FIX-xxx` / `FEATURE-xxx`（编号接续 ROADMAP 最大号），
   `--no-ff` 合回 main，删分支，不 push（除非用户明确要求）。
2. **BUILD 号**：每次合并 `main.go` 的 `const build` 加一，commit
   message 带 `[BUILD-xxx]`；前端版本号经 `/api/bootstrap` 自动下发。
3. **ROADMAP**：条目写在对应版本章节（背景/目标/实现/测试四段式），
   顶部任务表只保留 FEATURE-301~308。
4. **零依赖**：不加 Go 模块、不加前端库/CDN/npm。
5. **注释英文、提交信息与文档中文**；i18n 所有用户可见字符串。
6. 编译产物：`go build -o work/co-shell .`（仓库根，已被 .gitignore）。
7. 改完跑：`go build/vet/test` + `bash bin/output_audit.sh --strict`
   + `git diff go.mod go.sum` 必须为空。

---

## 10. 扩展食谱

### 10.1 新增一个 HTTP API

1. `server.go` `NewServer` 里注册路由；
2. handler 涉及文件系统必须过 `resolvePath`；返回统一 `writeJSON`；
3. `server_test.go` 补用例（含路径逃逸用例）；
4. 前端 `fetch` 调用，错误走 `console.error` + i18n 文案。

### 10.2 新增一种下行消息（kind）

1. `serverMessage` 加字段（omitempty）+ `Server` 加 `sendXxx` 方法；
2. 发送方（通常是 WebSession/WebRenderer）调用；
3. `app.js` `ws.onmessage` 加分支；
4. 注意与单客户端语义兼容（无连接时 sendJSON 返回 false）。

### 10.3 新增一种事件的渲染

1. 若新事件类型：先在 `agent/events.go` 定义常量（勿用魔法字符串，
   audit 会抓）；
2. `app.js` `renderEvent` 加分支——决定它进事件流（新块/并入流式块）
   还是进侧栏（参考 `task_plan` 双通道：事件驱动更新 + state 快照兜底）；
3. `eventClass` 加映射以继承既有配色，或新增 `.ev.xxx` CSS。

### 10.4 新增 UI 面板/区块

1. `index.html` 加骨架（grid 区域规划先看 6.1 的布局约束）；
2. `style.css` 用主题变量写样式；
3. `app.js` 加渲染函数；可见性切换参考 `#layout.no-plan` 的 class
   驱动方式。

### 10.5 实现"裸启动自动进 serve"（已讨论，待做）

决策链：显式 serve/--input-mode/config input_mode 都未指定 →
stdin/stdout 均为 TTY → 配置就绪（config.json 存在 + 有启用模型 +
免责声明已接受）→ 有 GUI 桌面（macOS 非 SSH；Linux 有
DISPLAY/WAYLAND_DISPLAY；Windows 恒真）→ 起 serve；任一环失败降级
tui（raw mode 失败再降 stdio，已有）。Listen/OpenBrowser 失败同样
降级 tui。每次降级必须 i18n 打印原因。注意：首次运行向导
（cmd/config.go 的 stdin scanner）在 web 模式不可用，配置未就绪时
绝不能进 web。

### 10.6 前端新交互

- 新 WS 上行消息：`clientMessage` 加字段 + `WebSession.handleMessage`
  加分支；
- 纯前端交互（如新的按钮）只动 `index.html/style.css/app.js` 三件套，
  改完 `node --check` + Node shim 或截图验证。

---

## 11. 已知限制与后续方向

1. **REPL 层 143 处裸 fmt.Print**（audit 存量）：在 web 模式下这些
   输出落在 serve 的终端而不是浏览器。builtin 命令的主要输出已通过
   常驻 WebIO 到达浏览器，但长尾仍在。收编需逐个改走 UserIO/事件，
   属渐进式工作。
2. **配置向导在 web 模式不可用**（cmd/config.go 旁路 stdin scanner）。
   首次配置必须在终端完成；这也是 auto-serve 必须检查"配置就绪"的
   原因。
3. **单客户端**：第二个 tab 会顶掉第一个（有意设计）。若未来要多
   客户端，hub 要改成广播 + 每连接状态。
4. **FEATURE-308**（全屏 TUI v2）是渲染器三态中唯一未做的可选分支，
   与 web 无关但共享同一事件协议。
5. **图片附件**：协议字段 `attachments` 保留且服务端照常解析
   （`WebSession.ReadLine`），但官方前端自 FEATURE-365 起移除了附件
   入口（📎 按钮与 chips），没有粘贴/截图直传。
