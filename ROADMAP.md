# co-shell 版本计划

> 版本号格式：`v{major}.{minor}.{patch}`

---

## v0.59.0 — 开发中

> **版本**: v0.59.0

> **状态**: 🚧 开发中
> **里程碑**: LLM 组件化输出协议——LLM 用结构化组件树（而非纯文本/Markdown）表达结果，Web UI 渲染为现代、直观、可交互的富组件
> **说明**: 当前 Web UI 的瓶颈不是排版，而是 LLM 没有「画东西」的表达通道：唯一下行单元是 `agent.StreamEvent`（`agent/events.go:19`），而前端 `web/static/md.js` 只是手写 Markdown 子集渲染器（注释明确 *never feeds raw input to innerHTML*），LLM 输出的上限就是「一段排版过的文本」，没有卡片/图表/交互控件。本版本新增 `render_ui` 工具，LLM 传入结构化组件树 JSON，前端 registry 递归渲染为主 DOM 富组件；并提供 `html` 逃生舱（sandbox iframe）承载长尾表达需求。硬约束：零第三方依赖、无前端构建步骤、`embed.FS` 单二进制打包。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-524 | 0.59.0 | P1 | LLM 组件化输出协议：`render_ui` 工具 + 组件树协议 + 前端 registry + 10 个基础组件 + 交互回传 + html 沙箱 + 上下文裁剪 |

> 当前 BUILD: 1025
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-524 LLM 组件化输出协议（co-shell Web UI 富组件渲染）**
  - 需求（用户确认，6 轮讨论）：
    1. 目标用户：非技术用户直接用 co-shell 完成任务并看结果（办公/数据分析/资料整理）。
    2. 表达载体：**工具调用为主**——新增 `render_ui` 工具，参数为组件树 JSON。
    3. 渲染归属：声明式组件走**主 DOM**（复用现有 CSS 变量，四套主题自动一致）；仅 `html` 逃生舱进 sandbox iframe。
    4. 图表/地图：**手写 SVG**（柱/折线/饼），零依赖红线不破；本期不做地图。
    5. 交互语义：组件动作开启新一轮 agent 回合（续作型），支持图表数据点钻取。
    6. 阻塞策略：由工具参数 `waiting` 控制，**默认不阻塞**。
    7. 更新机制：`ui_update` 按 id 原地更新，作用域=当前回合内任意组件。
    8. 上下文裁剪：本期就做，配置开关控制，**默认开**。
    9. 跨端：只做 Web UI，终端/飞书降级为纯文本或省略。
    10. 归属版本 **v0.59.0**（FEATURE，minor+1）；任务号 **FEATURE-524**；分支 `FEATURE-524`。
  - 协议契约：
    - 组件树节点 `{type, id?, props?, children?, actions?}`；动作声明 `{on: click|select|submit|change, id, payload: node|value|row|point|form}`。
    - 工具签名 `render_ui(tree: object, waiting: bool=false, intent: string)`；**只读安全免确认**；返回值仅简短回执（如「已渲染 card/ui-7（3 个子节点）」），不回传树本身。
    - 下行事件 `ui_render`（`Meta{ui_id, ui_tree}`）与 `ui_update`（`Meta{ui_id, ui_patch}`）。
    - 上行消息 `{"type":"ui_action","ui_id","action_id","payload"}`。
    - 阻塞语义：`waiting=false`（默认）→ 立即返回回执，用户动作作为用户输入注入并**开启新一轮**；`waiting=true` → 阻塞等待，用户动作作为**工具返回值**在同一回合内继续；用户直接打字 / ESC 打断 / 超时 → 释放阻塞并返回「用户未操作」。
  - MVP 组件（10 个）：`card` / `kv` / `table` / `chart` / `steps` / `callout` / `progress` / `file` / `form` / `html`。
  - 实施阶段（4 个 Stage）：
    1. **Stage 1 骨架**：`agent/uitree.go`（Node/Action 类型 + 校验）、`agent/ui_tools.go`（render_ui）、`agent/events.go`（新事件）、`agent/tools.go`（注册）、`web/static/ui.js`（registry + 递归渲染）、`renderEvent` 接入；首批组件 card/kv/callout/progress。
    2. **Stage 2 数据展示**：`web/static/ui-chart.js`（SVG 柱/折线/饼）+ table + steps + file。
    3. **Stage 3 交互与原地更新**：form + 动作回传（`web/session.go` 新增 `ui_action` 分支）+ `ui_update` + `waiting` 阻塞与释放。
    4. **Stage 4 逃生舱与治理**：html 沙箱（`/api/ui-sandbox` 独立端点 + 独立 CSP）+ 主页面 CSP + 组件目录进系统提示词（i18n zh/en）+ 上下文裁剪开关 + 文档与前端控件规范。
  - 安全约束：声明式组件一律 `createElement` + `textContent`（**禁止 innerHTML**，沿用 md.js 铁律）；html 逃生舱 iframe `sandbox="allow-scripts"`（**不加 `allow-same-origin`**，不透明源，无法访问父页面 DOM/存储）；`postMessage` 校验 `event.source === iframe.contentWindow` + 消息结构白名单。
  - 测试用例：`use-case/FEATURE-524/FEATURE-524-UC-0001.md`（UC-01~UC-49，A~H 共 8 组，已获用户确认）。
  - 进度：🚧 Stage 1（骨架：组件树协议 + 校验 + render_ui 工具 + ui_render 事件 + 前端 registry）已完成 [BUILD-1026]。
    - 已完成：`agent/uitree.go`（组件白名单/深度≤6/节点≤200/props≤8KB + ParseUITree/ValidateUITree/UISummary/MarshalUITree）、`agent/ui_tools.go`（render_ui：校验→暂存→简短回执，不回调树）、`agent/events.go`（`ui_render`/`ui_update` 事件 + Meta key + 构造器，走 ChannelSystem 以避开 show-* 过滤）、`agent/run_stream.go` 发射点、`agent/tools.go` 注册、`config` 的 `ui_enabled`/`ui_context_prune`（均默认开）、`i18n` 中英文案（`zh_ui.go`/`en_ui.go`，含系统提示词分节）、`web/static/ui.js`（registry + 递归渲染 + card/kv/callout/progress + 未注册降级）、`app.js`/`index.html`/`style.css` 接入。
    - 已通过：A/B 组单测（UC-01~UC-10）`go test ./agent/`；浏览器实测（UC-11~UC-16，28260 实例）——card/kv/callout(warn)/progress(75%) 渲染正确、`steps` 优雅降级、XSS 载荷零元素（`img/script/iframe/svg` 均为 0，`window.__xss` 未被置位）；`go build ./... && go vet ./...` 全绿。
    - 已知非本次引入的失败：`cmd` 包 `TestSettingsJSONFillsDefaults` 报 `setting "logo" has empty Default`（`cmd/settings_web.go:114` 定义处即无 Default），与 FEATURE-524 无关，待单独修复。
    - 待办：Stage 2 数据展示（table/chart/steps/file）、Stage 3 交互与原地更新（form/ui_action/waiting）、Stage 4 逃生舱与治理（html 沙箱/CSP/上下文裁剪落地）。

---

## v0.58.1 — 开发中

> **版本**: v0.58.1

> **状态**: ✅ 已完成（2026-09-15 合并 main，v0.58.1 BUILD-1025）
> **里程碑**: 修复「身份与个性」配置弹窗在矮窗口下显示不全的问题（高度自适应 + 内部滚动）
> **说明**: 点击左下角 logo 图标弹出菜单中的「身份与个性」弹窗（`.identity-box`）此前没有高度上限，弹窗高度完全由内容撑开；窗口高度较矮时弹窗上下溢出视口且无滚动条，底部字段与保存按钮无法访问。本次为 `.identity-box` 增加 `max-height: 90vh`（与既有系统设置 / 模型管理弹窗一致），并把 `.modal-body` 改为可伸缩 + 内部滚动，使弹窗最大高度不超过页面高度、内容超出时自动出现滚动条。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-523 | 0.58.1 | P1 | 「身份与个性」弹窗高度自适应：max-height 90vh + 内容区内部滚动，矮窗口下不再被裁切 |

> 当前 BUILD: 1025
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-523 「身份与个性」弹窗高度自适应（矮窗口不再裁切）** [BUILD-1025]
  - 需求（用户确认）：
    1. 点击左下角 logo 图标 → 弹出菜单 →「身份与个性」，进入该配置界面。
    2. 当前页面（窗口）比较矮时该界面显示不全，要求**最大高度不超过页面高度**，并**自动出现滚动条**。
    3. 归属版本 v0.58.1（FIX，patch+1）；最大高度取 **90vh**（与现有系统设置 / 模型管理弹窗一致）。
  - 根因：
    - `web/static/style.css:588` `.identity-box { width: 560px; max-width: 92vw; }` **无高度上限**，弹窗高度完全由表单内容撑开。
    - `web/static/style.css:2295` `.modal-body { padding: 14px; }` **无滚动容器**。
    - `.modal` 为 `position: fixed; inset: 0` 居中弹层，内容超高时向视口上下溢出且无滚动条，底部字段与「保存」按钮不可达。
  - 方案（实际实施，仅改 `web/static/style.css` 两条规则）：
    - `.identity-box` 增加 `max-height: 90vh`、`display: flex`、`flex-direction: column`。
    - 新增 `.identity-box .modal-body { min-height: 0; overflow-y: auto; }`（`min-height: 0` 是滚动生效的关键：flex 子项默认 `min-height: auto` 不会收缩到内容高度以下）。
    - 未改动 `.modal` / `.modal-box` / `.modal-body` 的公共规则，也未改动 JS 与其他弹窗样式。
  - 用例：`use-case/FIX-523/FIX-523-UC-0001.md`（UC-01~UC-16，用户已确认）
  - 校验（独立实例 v0.58.1 BUILD-1025 @28258，workdir `/tmp/fix523`，未影响生产实例 28256）：
    1. **缺陷复现（修复前）**：800×500 视口下弹窗 `top=-207.6` / `bottom=707.6`（上下各溢出 207.6px），高度 **915.3px**（>90vh=450px），`max-height` 计算值 `none`、`overflow-y: visible`、`scrollHeight == clientHeight == 870`（无滚动条），最后一个保存按钮 `bottom=692.6 > 500` 且 `scrollTop` 恒为 0，标题栏本身 `top=-206.6`（已在视口外）——UC-02/03/04/05/06 均失败，缺陷确认。
    2. **修复后主用例**：800×500 → `top=25 / bottom=475`（不溢出）、高度 **450 = 90vh**、`overflow-y: auto`（`clientH 405` / `scrollH 870`）、滚动后最后一个保存按钮 `top=423.3 / bottom=459.8`（完全可见）、标题栏滚动前后 `top` 均为 26（固定）。
    3. **高度自适应（无回归）**：800×1400 → 高度 **915.3 < 90vh(1260)**，`scrollHeight == clientHeight`（无多余滚动条），上下居中（top 242.4 / bottom 1157.6）。
    4. **极矮窗口**：800×360 → 高度 324 = 90vh，`top=18 / bottom=342`，滚动条存在，保存按钮滚动后可见（bottom 326.8）。
    5. **动态自适应**：500 → 1400 → 360 → 1280×800 连续改变视口高度（不重载页面），每次都满足 `top≥0`、`bottom≤innerHeight`、`height≤90vh`。
    6. **宽度与窄屏**：桌面 1280×800 宽度仍为 **560px**；窄屏 390×640（`body.narrow` 为真）宽度 **358.8 = 92vw**、高度 576 = 90vh、`top=32 / bottom=608` 不溢出、滚动条存在、保存按钮可达。
    7. **回归（其他弹窗无变化）**：1280×800 下 `.settings-box` 仍为 600×720（max-height 90vh 生效）、`.settings-nav`/`.settings-pane` 仍 `overflow-y: auto`、`.models-body` max-height 480px（60vh）、`.preview-box` max-width 1024px（80vw）；窄屏下系统设置仍按 FEATURE-487 全屏（390×640 / 800×800 均为整视口）。
    8. **静态与构建**：`node --check web/static/app.js` 通过（未改动 JS）；`go build ./... && go vet ./...` 在根模块与 `cmd/co-shell-hub` 模块均全绿；`--version` = `0.58.1 [BUILD-1025]`。
    9. **视觉**：800×500 截图对照，修复前弹窗被硬裁剪、底部保存按钮缺失且无滚动条；修复后标题栏/关闭按钮固定在顶部、内容区出现纵向滚动条、底部字段与保存按钮可达。
  - 版本与构建：`main.go` version 0.58.0 → 0.58.1、build 1024 → **1025**，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 进度：✅ 已完成（2026-09-15 用户确认测试通过，合并 main 并打标签 v0.58.1）[BUILD-1025]

---

## v0.58.0 — 开发中

> **版本**: v0.58.0

> **状态**: ✅ 已完成（2026-09-15 合并 main，v0.58.0 BUILD-1024）
> **里程碑**: 新增 dark-muted（暗色低对比）主题——主消息区信息块正文字体适度调暗
> **说明**: dark 主题下，主消息区信息块（`.ev`）背景为近黑深灰（`--bg-panel` #10141d、代码块 `--mono-bg` #0d1119），块正文使用 `--fg` #d5dbe7，对比度约 **13.5:1**，长时间阅读偏刺眼。本版本基于 dark 配色新增一套 `dark-muted` 色调，**仅**把信息块正文（`.ev-body`，含正文/代码/思考/引用）的文字颜色调暗（正文约 8.7:1、次级文字约 4.9:1），工作区与标题栏保持 dark 原配色，块标题与警示色（工具标题、错误红、监督块亮白、警告黄）保持现状。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-522 | 0.58.0 | P1 | 基于 dark 新增 `dark-muted` 色调：仅调暗主消息区信息块正文字色（`.ev-body` 及 md 引用/次级标题），工作区与标题栏配色不变；接入主题循环切换与系统设置下拉；logo 支持独立色调槽 |

> 当前 BUILD: 1024
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-522 dark-muted 暗色低对比主题（主消息区正文字体调暗）** [BUILD-1024]
  - 需求（用户确认）：
    1. 基于当前 dark 主题样式，增加一套主题风格；主要目的是在**背景为黑或接近黑色的深灰区域**，避免使用亮白色或其他对比度过高的亮色作为文字颜色，**适当调暗字体颜色**。
    2. **工作区、标题栏不用调**，主要是主消息区的信息块。
    3. 主题标识（`data-theme` 值）为 `dark-muted`，显示名「暗色低对比」；归属版本 v0.58.0。
    4. 调暗范围：**仅信息块正文**（`.ev-body` 的正文/代码/思考/工具输出文字）；**块标题与警示色亮度保持现状**。
    5. 接入方式：**加入主题循环切换按钮 + 系统设置下拉**（与现有 4 个色调并列）。
  - 方案（实际实施）：
    - `web/static/style.css`：在 dark 基础变量块新增 `--ev-body-fg` / `--ev-body-fg-dim`（默认分别为 `var(--fg)` / `var(--fg-dim)`，其余主题行为不变）；`.ev-body` 显式使用 `--ev-body-fg`，`.ev.thinking .ev-body`、`.ev-body.md blockquote`、`.ev-body.md h5/h6` 改用 `--ev-body-fg-dim`；新增 `[data-theme="dark-muted"]` 块仅覆盖上述两个变量（正文 #a9b1c1 ≈8.7:1、次级 #7b8395 ≈4.9:1，dark 原值分别约 13.5:1 / 6.1:1）。配色取自 `:root`（即 dark），故工作区/标题栏与 dark 完全一致。
    - `web/static/app.js`：`themeIcon()` 新增映射到 `i-theme-muted`；`themeMode()` / `applyTheme()` 接受 `dark-muted`；`themeToggle` 循环改为 dark → dark-muted → light → light-tp → paper → dark；设置下拉 `opts` 与 logo 区块色调标签同步。
    - `web/static/index.html`：图标 sprite 新增 `i-theme-muted`（半明半暗圆，表意「低对比」）。
    - `cmd/settings_web.go`：`theme-mode` 的 `Options` 加入 `dark-muted`。
    - `web/server.go`：`logoThemeFromParam` 白名单加入 `dark-muted`（及其错误提示文案）。
  - 用例：`use-case/FEATURE-522/FEATURE-522-UC-0001.md`（UC-0001~0014，14 条，用户已确认）
  - 校验（独立实例 v0.58.0 BUILD-1023 @28257，workdir `/tmp/fe522`，未影响生产实例 28256）：
    1. 静态与构建：`go build ./... && go vet ./...`（root 与 hub 两模块）全绿；`node --check web/static/app.js` 通过；`--version` = `0.58.0 [BUILD-1023]`。
    2. 核心效果（实测计算值）：`.ev.llm .ev-body` 文字色 dark `rgb(213,219,231)`（#d5dbe7，对 `--mono-bg` #0d1119 对比度 **13.6:1**）→ dark-muted `rgb(169,177,193)`（#a9b1c1，**8.77:1**，对比度降 35.5%）；思考/引用次级文字 `rgb(139,147,165)`（5.98:1）→ `rgb(123,131,149)`（**4.85:1**）；代码块/行内代码文字随正文一并调暗，代码块底色仍为 #0d1119。
    3. 反向用例（保持现状）：块标题 `.ev-head`、工具标题 #fbbf24、错误 #FF0000（文字与指示点）、警告 #facc15、监督块 #ffffff、user-msg 标题 accent —— 在 dark 与 dark-muted 下逐项完全相同。
    4. 工作区与标题栏：`#topbar` 底色/底边框、`.brand-name`、`.stream-head`、`.stream-title`、`.tree-row.dir > .name` 在两主题下逐项完全相同（dark-muted 未重定义任何调色板变量，直接继承 `:root`）。
    5. 回归：dark/light/light-tp/paper 下 `.ev-body` 色仍分别等于各自 `--fg`（#d5dbe7 / #1d2433 / #1d2433 / #4a3b2a），即变量默认回落正确。
    6. 交互：`themeToggle` 循环实测 `dark → dark-muted → light → light-tp → paper → dark`；图标切换为新增 sprite `i-theme-muted`（16×16）；设置→外观→主题下拉选项为 `auto, dark, dark-muted, light, light-tp, paper`，选中后即时生效、`localStorage` 持久化且刷新后仍为 dark-muted。
    7. 接口：`GET /logos/dark-muted` 未配置时 404、上传/删除接口接受 `theme=dark-muted`（落盘 `logos/logo-dark-muted.png`）、非法主题仍 400 且提示文案已更新。
    8. 视觉：dark 与 dark-muted 同载具截图对照，后者正文/代码/引用明显更柔和，警示色与标题栏/工作区外观无变化（需用户主观确认“调暗适度”）。
  - 第二轮（用户反馈，BUILD-1024）：**再调暗一档 + 录入框 / 会话标题栏**
    - 需求（用户确认）：① 刚才变动的颜色再调暗（选定档位「中」）；② 录入框与会话标题栏也一起调，范围为「文字色 + 录入框背景/边框一并压暗」。
    - 实施：
      1. `--ev-body-fg`：`#a9b1c1` → **`#8f97a9`**（6.29:1，上一轮 8.70:1，再降 28%）；`--ev-body-fg-dim`：`#7b8395` → **`#767e90`**（4.52:1）。**次级未严格按比例降**：按比例会落到 ≈3.7:1（低于 WCAG AA 4.5:1，且 `.ev.thinking` 另有 `opacity:.75` 叠加），故取 AA 下限附近的 4.52:1。
      2. 新增 4 个变量（默认回落原值，其余主题零变化）：`--input-fg`/`--input-bg`/`--input-border`（`#input` 改用）与 `--stream-head-fg`（`.stream-title`、窄屏 `.stc-face` 改用）。dark-muted 下：录入框文字 `#8f97a9`、底 `#090c12`（原 `#0d1119`）、边框 `#1a2030`（原 `#232a3a`）；会话标题文字 `#8f97a9`（标题栏背景不变）。
      3. 非目标：`#input::placeholder`（占位符）、`.ask-line input` 等次要输入框、状态条、工作区与顶栏、块标题与警示色——均未改动。`#input:focus` 仍为 accent 边框。
    - 校验（BUILD-1024，独立实例 @28257）：正文 `rgb(143,151,169)` / 对比度 **6.29:1**（dark 13.26:1）；次级 `rgb(118,126,144)` / **4.52:1** ≥ AA；录入框 dark `rgb(213,219,231)`+`rgb(13,17,25)`+`rgb(35,42,58)` → muted `rgb(143,151,169)`+`rgb(9,12,18)`+`rgb(26,32,48)`；会话标题 `rgb(213,219,231)` → `rgb(143,151,169)`、标题栏背景两主题均 `rgb(16,20,29)`；light/light-tp/paper 下 `#input` 与标题文字仍等于各自 `--fg`/`--mono-bg`/`--border`；反向用例（顶栏/工作区/警示色/占位符）两主题逐项相同；dark/dark-muted 截图对照确认录入框与会话标题同步变柔。
  - 版本与构建：`main.go` version 0.57.1 → 0.58.0、build 1022 → **1024**（第二轮 +1），`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 进度：✅ 已完成（2026-09-15 用户确认后合并 main，打标签 v0.58.0；co-shell 与 co-shell-hub 已编译并原子替换至 `~/bin/`）[BUILD-1024]

---

## v0.57.1 — 已完成

> **版本**: v0.57.1

> **状态**: ✅ 已完成（2026-09-15 合并 main，v0.57.1 BUILD-1022）
> **里程碑**: hub 探测 co-shell 增加「hub 自身目录」并标注候选来源
> **说明**: `DetectCoShells()` 只扫描 hub 进程的当前工作目录与 PATH，**不扫描 hub 可执行文件自身所在目录**；而启动 managed agent 时用的 `defaultCoShellPath()` 恰恰取 hub 同目录。两者语义不一致，导致「co-shell 与 hub 放同一目录却在下拉里搜不到」——实测双击启动 hub 时 CWD=$HOME，仅因为 `~/bin` 恰好位于 PATH 首位才被搜到。本版本让探测覆盖 hub 自身目录，并让下拉候选显示来源，便于区分同名/陈旧文件。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-521 | 0.57.1 | P2 | `DetectCoShells()` 新增扫描 hub 可执行文件所在目录（`os.Executable()` + 软链解析），来源标注为可辨识值；hub Web UI 的 co-shell 下拉对每个候选显示来源（当前目录 / PATH / hub 同目录） |

> 当前 BUILD: 1022
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-521 hub 探测 co-shell 增加「hub 自身目录」并标注候选来源** [BUILD-1022]
  - 需求（用户确认）：
    1. `DetectCoShells()` 在现有「进程 CWD」+「PATH 各目录」之外，新增扫描 **hub 可执行文件所在目录**，与 `defaultCoShellPath()` 的启动语义对齐；取目录时解析软链（macOS 上 `os.Executable()` 不保证解引用）。
    2. 候选列表的 `source` 能区分三类来源，前端下拉在选项中显示来源（当前目录 / PATH / hub 同目录），便于区分同名或版本陈旧的二进制（实测环境 `~/bin` 下存在被命名为 `.exe` 的旧 0.7.8 macOS 二进制）。
    3. **不改名、不删除**任何既有文件（用户明确：不动 `co-shell-b455.exe`）。
  - 方案（实际实施）：
    - 后端 `hub/gateway/detect.go`：`DetectCoShells()` 在 CWD → PATH **之后追加**扫描 hub 可执行文件所在目录（用户确认「对现有候选顺序零影响」）；新增 `hubDir()`（`os.Executable()` + `filepath.EvalSymlinks`）；候选 `source` 新增 `"hubdir"`。
    - 前端 `hub/gateway/webui_static.go`：新增 `coShellSourceLabel()` / `coShellOptionText()`，新建与修改表单的 co-shell 下拉均显示「<路径>（hub 同目录 / 当前目录 / PATH）  (vX.Y.Z)」，`value` 仍为纯绝对路径；「未找到」提示补上 hub 同目录。
    - 单测 `hub/gateway/detect_test.go`（新增）：`TestHubDirResolvesExecutableDir`、`TestDetectCoShellsHubDir`（含同前缀但 `--version` 无效的反例过滤断言）。
  - 用例：`use-case/FIX-521/FIX-521-UC-0001.md`（UC-0001~0011，11 条，用户已确认）
  - 校验（BUILD-1022）：
    1. root 与 hub 两模块 `go build ./... && go vet ./...` 全绿；`go test ./gateway/` 通过；前端 JS 经 `node --check` 通过且 `webui_static.go` 反引号计数 = 2（未破坏 Go 原始字符串）。
    2. 隔离实例实测（`/tmp/fix521`，独立端口 12901/12900 与注册表）：候选顺序为 `cwd → path → path → hubdir`；`co-shell-fake`（`--version` 无效）与 `not-co-shell`（前缀不符）均未被列入；`/api/co-shell-locations` 与 `/api/agent-defaults` 的 `co_shells` 一致；下拉 4 项均带来源标注（含旧版 `co-shell-b455.exe（PATH） (v0.7.8)` 可辨识）；`/api/hub-info` = 0.57.1/1022。
    3. UC-0004（同一路径双路命中的来源归属）与 UC-0006（hub 目录无 co-shell 时）未单独构造场景实测；去重沿用既有 `seen` map（本次未改动）。
  - 进度：✅ 已确认并合并（2026-09-15，main 41637f7，v0.57.1 BUILD-1022，标签 v0.57.1）

---

## v0.57.0 — 已完成

> **版本**: v0.57.0

> **状态**: ✅ 已完成（2026-09-14 合并 main，v0.57.0 BUILD-1021）
> **里程碑**: Hub Agent 信息与启动参数支持修改——把只读详情页替换为与新建表单一致的可编辑界面
> **说明**: co-shell-hub 的 Agent 列表卡片右侧箭头目前打开一个**只读**详情页（`viewDetail`，字段以 `<div class="val">` 展示），添加 Agent 后无法再调整其信息与启动参数。本版本把该只读页替换为**可编辑的修改表单**，字段与「新建 Agent」界面保持一致；后端相应新增 agent 修改接口与持久化逻辑。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-520 | 0.57.0 | P2 | Hub Agent 信息与启动参数支持修改：只读详情页 → 可编辑表单（字段与新建一致）；新增 `PUT /api/agents/{id}` 与 `Manager.Update`；ID 只读；运行中保存后提示需重启生效；远程 Agent 与本地字段集一致、本地专有字段置灰 |

> 当前 BUILD: 1021
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-520 Hub Agent 信息与启动参数支持修改** [BUILD-1021]
  - 需求（用户确认）：
    1. 把 Agent 列表卡片右侧箭头打开的**只读详情页**替换为**修改界面**，控件与「新建 Agent」表单同款。字段集统一为：ID（只读）/ 备注 / 类型（只读）/ Workspace 路径 / 主机地址 / 端口号 / co-shell 可执行程序 / 共享配置 / 补充运行参数。
    2. **ID 只读展示、不可修改**（ID 是注册表唯一键与连接标识，改名需迁移注册表与代理连接，本次不做）。
    3. 对**运行中**的 Agent 保存修改后，仅持久化并提示「需重启 agent 后生效」，**不自动重启**（不打断正在执行的任务）。
    4. **远程（external）Agent 与本地使用同一套字段集**，不适用的字段置灰不可编辑：本地 Agent 的「主机地址」置灰；远程 Agent 的 Workspace / co-shell / 共享配置 / 补充运行参数置灰。「端口号」两者均可编辑（本地 = serve 端口，远程 = WS 端口），远程保存后按「主机地址 + 端口号」重建 ws_url（主机地址字段即为此而设）。
  - 方案：
    - 后端 `manager.go`：新增 `Update(id, patch)`——校验 agent 存在、端口合法性/占用、workspace 目录，处理 {workspace}/config.json 创建，更新注册表并持久化。
    - 后端 `webui.go`：新增 `PUT /api/agents/{id}` handler（含请求体校验与错误码）。
    - 前端 `webui_static.go`：`viewDetail` 由只读 `<div class="val">` 改为可编辑控件（与新建表单同款 `.field`/`.switch` 结构），新增「保存 / 取消」按钮，远程 Agent 隐藏本地专有字段区域。
  - 用例：`use-case/FEATURE-520/FEATURE-520-UC-0001.md`（UC-0001~0014）
  - 实施（BUILD-1021）：
    1. `hub/gateway/manager.go`：新增 `AgentPatch`（指针字段，区分“未提供”与“置空”）与 `Manager.Update(id, patch)`——仅更新携带的字段；ID/Type 不可变；校验 ws_url/workspace 非空、端口范围 1–65535，且仅当端口变化时才检查占用；维护 {workspace}/config.json；`save()` 落盘。新增哨兵错 `ErrAgentNotFound`（404）与 `ErrInvalidAgent`（400），端口占用返回 409。
    2. `hub/gateway/webui.go`：新增 `PUT /api/agents/{id}` 与 `handleUpdateAgent`（指针型请求体）；按错误类型映射状态码；远程 agent 修改后重建代理连接（`RemoveAgent` + `AddAgent`）使新 ws_url 立即生效。
    3. `hub/gateway/webui_static.go`：删除只读详情页（`viewDetail` 的 `.val` 字段 + 对应 CSS）与 `showAgentDetail`，改为可编辑表单（`showAgentEdit` / `saveAgentEdit` / `wsURLParts` / `setFieldEnabled` / `fillCoShellOptions` / `checkEditVersion`）；新增主机地址字段与「保存修改」按钮；不同类型置灰相应字段；co-shell 下拉保留“已配置但未检测到”的当前值，避免静默改写。
    4. `hub/gateway/manager_test.go`（新增）：`Manager.Update` 的 table-driven 单测（8 组）——未知 agent、全量字段更新与落盘、部分补丁不扰其他字段、空备注回退 ID、自身端口可重复提交、端口占用拒绝且不落库、非法输入（端口越界/空 workspace/空 ws_url）、远程 ws_url 与字段隔离。
  - 校验（BUILD-1021）：
    1. `go build ./... && go vet ./...`（root 与 hub 模块）全绿；`go test ./gateway/ -run TestManagerUpdate` 全通过；hub JS 经 `node --check` 语法校验通过（反引号计数=2，未破坏 Go 原始字符串）。
    2. HTTP 端到端（独立测试实例 `--web-addr 127.0.0.1:12901 --registry /tmp/fe520-hub-agents.json`，脚本 `tmp/fe520_verify.sh`）：29/29 断言通过——字段回填、改名/改 workspace/端口/co-shell/共享配置/参数落盘、workspace 与 config.json 自动创建、端口占用 409 且不落库、未知 agent 404、非法 JSON 与越界端口 400、远程 ws_url 可改、`/api/hub-info` 版本 0.57.0 / BUILD-1021。
    3. 浏览器 DOM（UC-0008~0013）：箭头打开的是可编辑表单（9 个控件、`.val` 只读字段=0）；ID 只读；本地 agent 的「主机地址」置灰（disabled）；修改界面提示“运行中（修改需重启后生效）”，保存后提示“已保存，需重启该 agent 后生效。”；UI 保存使注册表出现新备注且受管进程 PID 未变（未自动重启）；保存后列表即时刷新、启动/停止/删除无回归。
    4. 反引号、点：截图留待最终交付确认（旧只读页已无 `.val` 残留）。
  - 进度：✅ 已确认并合并（2026-09-14，main 6c82eb7，v0.57.0 BUILD-1021，标签 v0.57.0）

---

## v0.56.2 — 已完成

> **版本**: v0.56.2

> **状态**: ✅ 已完成（2026-09-14 合并 main，v0.56.2 BUILD-1020）
> **里程碑**: 修复 Web UI 工作区文件树中文件行误显示「展开/收起」三角箭头
> **说明**: 工作区文件列表里，文件（非目录）行左侧也渲染了只有目录才该有的 ▸ 展开/收起三角箭头。根因在 `web/static/app.js` 的 `treeNode()`：创建 `.tw`（twisty）span 后无条件执行 `setIcon(tw, "i-tri")`，因此文件行也被填入箭头；目录分支随后会按 open 状态重新设置 `i-tri`/`i-tri-down`，所以该初始赋值对目录冗余、对文件则是错误箭头来源。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-519 | 0.56.2 | P2 | 删除 `treeNode()` 中对所有节点生效的初始 `setIcon(tw, "i-tri")`：`.tw` span 仍为每行创建（CSS `width:12px` 占位不变，文件/目录名称左对齐不受影响），但文件行保持为空→不再显示箭头；目录行由 dir 分支正常设置展开/收起箭头 |

> 当前 BUILD: 1020
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [X] **FIX-519 工作区文件列表文件行误显示三角箭头** [BUILD-1020]
  - 根因：`web/static/app.js` `treeNode()` 为每个节点（含文件）无条件 `setIcon(tw, "i-tri")`。
  - 方案：删除该行初始赋值（外科手术式单行删除）；`.tw` 仍保留作为 12px 左对齐占位，文件行为空、目录行由 dir 分支按 open 状态设置 `i-tri`/`i-tri-down`。`light-tp` 主题的文件夹字形作用于 `.tree-row.dir .tw`，不受影响。
  - 测试用例：`use-case/FIX-519/FIX-519-UC-0001.md`（UC-0001~0007：静态校验、文件行无箭头、对齐不偏移、展开收起功能、light-tp 主题、构建全绿、版本同步）。
  - 版本与构建：`main.go` version 0.56.1 → 0.56.2、build 1018 → 1020，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步。
  - 进度：✅ 已完成（2026-09-14 合并 main、打 tag v0.56.2；已编译部署至 `~/bin/` 并重启实例生效，浏览器实测 179 个文件行箭头数=0、16 个目录行缺箭头数=0、同层名称左偏移均 39px）

---

## v0.56.1 — 开发中

> **版本**: v0.56.1

> **状态**: 🚧 开发中
> **里程碑**: 修复 Web UI 刷新后 10–20 秒高 CPU / 输入卡顿——历史回放期间的工作区刷新风暴
> **说明**: 刷新页面（或 WebSocket 重连）时前端回放一页持久化事件，而 `renderEvent()` 在 `token_iter`、`done`、工具调用结束三类事件上都会立即调用 `refreshBranch()+loadTree()`。`loadTree()` 会 `fetch("api/tree")` 并清空重建整棵工作区树 DOM；该接口每次都递归遍历整个工作区（深度上限 8 层、每目录最多 500 项，跳过 `.git/node_modules/db/log/tmp`）并执行一次 git status。本仓库工作区实测 **4065 目录 / 16753 文件**（其中 `work/` 占 3441 目录 + 13511 文件），/api/tree 单次响应 **6.3 MB**、服务端耗时 155 ms。一页历史含数十条上述事件，于是刷新后触发几十至上百次「6.3 MB 树请求 + JSON 解析 + 整树 DOM 重建」，主线程被占满十余秒，输入框随之卡顿。用户反馈“已有几个版本”均存在，与 FEATURE-517 无关。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-518 | 0.56.1 | P2 | 工作区刷新风暴治理：回放期间抑制 `refreshBranch/loadTree`（结束只刷一次）；其余场景 300ms 去抖合并；文件树目录子节点按需渲染（方案 B，默认折叠、首次展开才构建） |

> 当前 BUILD: 1018
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [X] **FIX-518 修复刷新后高 CPU / 输入卡顿（工作区刷新风暴）** [BUILD-1018]
  - 现象：刷新页面后 10–20 秒内 CPU 占用极高，焦点在主输入框时操作极慢、卡顿，随后自行恢复。
  - 根因：见上方「说明」（历史回放 × 每次事件触发 6.3 MB 文件树重建）。
  - 修复（`web/static/app.js`）：
    1. 新增 `scheduleWorkspaceRefresh(immediate)`：合并工作区刷新请求；`historyReplaying` 期间仅置 `workspaceRefreshPending`，其余场景 300 ms 去抖（`WORKSPACE_REFRESH_DEBOUNCE_MS`）。
    2. 三类高频事件（`token_iter` / `done` / 工具调用结束）由「立即 refreshBranch+loadTree」改为 `scheduleWorkspaceRefresh()`。
    3. `renderHistory()` 全程持有 `historyReplaying`，结束时 `endHistoryReplay()` 释放，并仅在有待处理请求时刷新一次。
    4. 保留用户主动动作（上传、revealInTree、session 截断、首屏加载）的直接刷新。
  - 用例：`use-case/FIX-518/FIX-518-UC-0001.md`
  - 实施（BUILD-1017）：
    1. 新增 `scheduleWorkspaceRefresh(immediate)`：`historyReplaying` 期间仅置 `workspaceRefreshPending`（不发起请求）；其余场景 300 ms 去抖（`WORKSPACE_REFRESH_DEBOUNCE_MS`）；`immediate` 跳过去抖立即执行。
    2. `renderEvent()` 的三类高频事件（`token_iter` / `done` / 工具调用结束）改调 `scheduleWorkspaceRefresh()`，不再直接 `refreshBranch()+loadTree()`。
    3. `renderHistory()` 全程持有 `historyReplaying`；`endHistoryReplay()` 释放标志，并仅在有待处理请求（`workspaceRefreshPending`）时刷新一次；分页追加（`prepend`）分支同样适用。
    4. 保留用户主动动作（上传、`revealInTree`、session 截断、首屏加载）的直接刷新。
    5. 版本与构建：`main.go` version 0.56.0 → 0.56.1、build 1016 → 1017，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 校验（BUILD-1017）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；独立实例（v0.56.1 BUILD-1017 @28384）在页面内包裹 `fetch` 计数实测：
    1. 模拟一页含 **30 个 `token_iter`** 的历史回放：`api/tree` 调用 **1 次**（改前同场景 **30 次**）、`api/bootstrap` **1 次**（改前 30 次）——降幅 30×。
    2. 运行时去抖：连续 3 次 `scheduleWorkspaceRefresh()` 只产生 **1 次** 刷新（tree 与 bootstrap 各 1）。
    3. 回放结束但无待处理请求时：**0 次**刷新（不产生多余请求）。
    4. 旧行为对照：直接循环 30 次 `refreshBranch()+loadTree()` 实测产生 30 次请求、耗时 39.6 ms（小工作区）；真实工作区单次响应 6.3 MB / 服务端 155 ms，故改前 30 次约需 9 s 以上的主线程占用，与用户观察到的 10–20 秒卡顿量级吻合。
  - 实施（BUILD-1018，方案 B 文件树按需渲染）：
    1. `treeNode(node)` 目录分支不再无条件递归构建全部子节点：折叠目录只保留一个空 `<ul>`，仅当该目录在 `expandedDirs` 中时才调用新增的 `fillDirChildren(ul, node)` 构建子项。
    2. 新增 `fillDirChildren(ul, node)`：以 `ul.dataset.filled` 作幂等标记（同一目录不重复构建），用 `DocumentFragment` 一次性插入子行。
    3. 修复由此暴露的真实缺陷：`revealInTree(path)` 原本只把祖先目录加入 `expandedDirs`、**漏了工作区根 `""`**（`highlightAffectedFiles` 有这一行）。旧代码整棵树都在 DOM 里所以只是「看不见」，懒渲染后根未填充会导致目标行根本不存在——已补 `expandedDirs.add("")`。
    4. 服务端 `/api/tree` 与可见内容**未改动**（实测 `JSON.parse` 仅 6 ms，非瓶颈，无需改服务端）。
    5. 版本与构建：`main.go` build 1017 → 1018、`cmd/co-shell-hub/main.go` hubBuild 同步 1018（version 0.56.1 不变，FIX 类不动版本位）。
  - 校验（BUILD-1018）：在诊断实例（`--serve --port 28385 -w /tmp/fe518-big`，合成大树 15006 文件 / 6034 目录 = 21040 条目）实测：
    1. 首屏 `.tree-row` **21034 → 1**；`document.getElementsByTagName('*').length` **237953 → 560**。
    2. 单次 `await loadTree()` **498.9 ms → 134.2 ms**（基线 498.9 ms 为 HEAD 真实单次成本；此前记录的 4618.8 ms 是重建脚本测量值，对应 LoAF 多次调用累计，非单次成本）。
    3. 展开根目录 **0.4 ms**（渲染 14 行）；折叠后再展开耗时 0 ms、行数不变（14）——**不重复渲染**；深层展开 190 → 340 行，折叠 340，再展开仍 340（noDup）。
    4. 功能回归全部通过：`revealInTree` 目标行存在且可见（rows 190 / expanded 5）；刷新后 `highlightTreeFile` → `fv-selected` 生效；`highlightAffectedFiles` → `.name.aff-pred` 存在且可见；懒渲染出的目录行 `onclick/ondrop/ondragover` handler 齐全（拖拽上传可用）；点击文件行预览器打开且 `fv-selected` 生效。
    5. `go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过。
  - 遗留（仅记录，未修）：`web/server.go` 的 `buildTree` 每个目录上限 500 项且**静默丢弃**超出部分（`work/` 这类目录在树中看不全），建议另开任务处理。
  - 附带（规范）：`.rules/PROJECT STANDARDS.md`「编译可执行码」新增第 3 条「部署到内网 Linux 主机」——参数为 `3` 编译后自动把 Linux arm64 的 `co-shell` / `co-shell-hub` 复制到 `liangshuang@192.168.3.39:~/bin/`（先传 `.new` 再远端 `mv`，避开 Linux `Text file busy`；仅复制不重启远端服务；失败仅告警不阻断；参数 `release` 不部署）。
  - 进度：✅ 已完成（v0.56.1 / BUILD-1018）——BUILD-1017 刷新风暴修复（回放期间抑制 + 300ms 去抖）与 BUILD-1018 文件树目录子节点按需渲染（方案 B）均已实测验证，用户已确认合并到 main。

---

## v0.56.0 — 已完成

> **版本**: v0.56.0

> **状态**: ✅ 已完成（2026-09-13 合并 main，tag v0.56.0，v0.56.0 BUILD-1016）
> **里程碑**: ask_user 多问题表单（qs-card）焦点切换动效——焦点在题目间移动时提供可见的过渡反馈（淡入淡出 + 平滑滚动），消除瞬移带来的眩晕感
> **说明**: Web UI 的 ask_user 多问题表单在切换焦点题目时为「瞬时跳变」：`setActive()` 直接调用 `card.scrollIntoView({block:"start"})`（无动画），Qn 徽标的 `.hot` 类、1-9 选项键帽与提示行仅切换 `hidden` 类（无过渡）。用户在长表单中按回车前进或按 `+` 返回时看不到焦点移动轨迹，容易迷失与眩晕。本次在 CSS 层为徽标 / 键帽 / 提示行加入淡出淡入过渡，在 JS 层将滚动改为平滑滚动，并保持既有键盘交互不变。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-517 | 0.56.0 | P2 | ask_user 多问题表单焦点切换动效：Qn 徽标 / 选项键帽 / 提示行淡出淡入 + 焦点题目平滑滚动（约 300ms）；鼠标点选保持现状（不自动前进） |

> 当前 BUILD: 1016
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-517 ask_user 多问题表单焦点切换动效**
  - 需求（用户确认）：
    1. 焦点在题目间切换时（Enter / 选项键帽选择后前进、`+` 返回上一题）要有可见的移动动效，Q1 → Q2 徽标的焦点转移需明显可辨。
    2. 焦点消失与显现要有淡出 / 淡入效果（含 Qn 徽标、1-9 选项键帽、提示行）。
    3. 题目切换的滚动要平滑，不能瞬移，避免用户眩晕。
    4. 动效强度（用户确认）：轻量——仅淡入淡出 + 平滑滚动。
    5. 时长：**450ms**（初版 300ms，用户实机体验后要求「焦点移动的动态效果用时再延长 50%」；滚动同步降速以保持同一节奏）。
    6. 鼠标点击选项行保持现状（仅勾选，不自动前进）。
  - 用例：`use-case/FEATURE-517/FEATURE-517-UC-0001.md`
  - 实施（BUILD-1015）：
    1. `web/static/style.css`：`.qs-index` 加入 `transition: background-color/color/border-color 0.3s`（徽标焦点交叉淡变）；`.qs-hotkey` 与 `.qs-hint` 加入 `transition: opacity 0.3s`，并新增 `.qs-hotkey.qs-fading, .qs-hint.qs-fading { opacity: 0; pointer-events: none; }`。
    2. `web/static/app.js`（`renderQuestions`）：新增 `QS_FADE_MS = 300`、`qFadeTimers`、`fadeToggle(el, on, instant)`。因 `.hidden` 是 `display:none` 无法过渡，改为「先加 `.qs-fading` 淡出 → 300ms 后加 `.hidden`」的两阶段；淡入则先恢复显示、置 `.qs-fading` 后强制 reflow 再移除，从而真正播放过渡。隐藏/显示终态与原实现（`display:none`）完全一致，静止布局不变。
    3. 新增 `qsScrollTo(card, instant)`：每帧重算目标（`card` 顶部对齐 `#askArea` 顶部）并用指数缓出推进 `scrollTop`，上限 `QS_FADE_MS * 2`；`instant` 用于首次激活（表单出现时瞬时就位、不播动画）且不影响初次定位旧行为。
    4. 首次激活（`qsFirstActivation`）跳过淡入淡出，避免表单刚渲染就整片动画。
    5. 版本与构建：`main.go` version 0.55.0 → 0.56.0、build 1013 → 1015，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 校验（BUILD-1015）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；独立实例（v0.56.0 BUILD-1015 @28381，workdir `/tmp/fe517-site`）DOM 实测（6 题表单）：
    1. 样式：`.qs-index` transition-duration `0.3s, 0.3s, 0.3s`（background-color/color/border-color）；`.qs-hotkey` 与 `.qs-hint` 各 `0.3s`（opacity）——落在需求确认的 280–350ms 区间。
    2. 徽标交叉淡变：切换瞬间 t60ms 时 Q1/Q2 背景 alpha 分别为 0.718 / 0.282（t150ms 为 0.196 / 0.804），终态与旧样式一致——证明为交叉过渡而非瞬时切换。
    3. 键帽/提示行淡入淡出：t60ms Q1/Q2 键帽 opacity 0.717 / 0.283；提示行 t0 为 `qs-fading`、t320ms 后为 `hidden`；终态键帽 `hidden` 数量回到 `3,0,3,3,3,3`（仅焦点题显示 3 枚）与旧实现一致。
    4. 平滑滚动且终态精确：前进时 `scrollTop` 依次 53 → 114.5 → 281 → 299 → 222（全程有中间值，非瞬移）；焦点卡片相对 `#askArea` 顶部偏差终态 **0.9px**（前版用 `scrollIntoView({behavior:"smooth"})` 时为 **-76px**，因淡出结束后旧题卡片变矮、预计算目标落空；改为每帧跟随重算后修正）。
    5. 交互回归：`+` 回退（ofs 由 0.9 → -0.9，中途 -18.9 有过程）；Q1 再按 `+` 不越界；数字键 `2` 选择后 Q1 选项变 `qs-option on` 并自动前进；鼠标点击选项行仅勾选、`scrollTop` 位移 0（不前进）；空格打开题目备注输入框（`qs-free qs-free-note` 获焦）；长按数字键打开选项备注框（测试中未派发 keyup 时 450ms 后自动弹出）；`-` 取消表单后 `#askArea` 隐藏。
    6. 初次渲染：表单刚渲染时 Q1 键帽 `qs-fading` 数 0、可见 2 枚、提示行可见且无 fading、opacity 1——无突兀动画。
    7. 截图视觉复核：焦点题徽标高亮 + 键帽 1/2/3 + 提示行齐备；非焦点题键帽与提示行完全隐藏、无残留空行；无错位/重叠/文字裁切。
  - 进度：✅ 已完成（BUILD-1015 实现 + 独立实例用例验证）
  - 第二轮调整（BUILD-1016）：用户实机体验后要求焦点移动时长「再延长 50%」，仅调参数、不改结构：`QS_FADE_MS` 300 → **450**，`.qs-index` / `.qs-hotkey` / `.qs-hint` 的 transition 由 `0.3s` → **`0.45s`**，滚动的每帧收敛系数由 `0.25` → **`0.18`**（使滚动走完时间与淡入淡出同步拉长；仍用指数跟随而非定长补间，以平滑吸收旧题塌陷带来的目标位移）。
  - 校验（BUILD-1016）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；独立实例（v0.56.0 BUILD-1016 @28382）DOM 实测：
    1. `transition-duration` = `0.45s, 0.45s, 0.45s`（徽标）/ `0.45s`（键帽）/ `0.45s`（提示行）——已同步放大。
    2. 时间线拉长可量化：前进时 t120ms 徽标高亮为 **0.573 / 0.427**、t250ms 为 **0.150 / 0.850**、t400ms 为 **0.008 / 0.992**、t520ms 回到终态（对比 300ms 版 t60ms 已达 0.718 / 0.282）；键帽透明度同步为 0.571/0.429 → 0.148/0.852。
    3. 滚动仍为渐进且终态精确：`scrollTop` 97.5 → 287.5 → 298.5（布局塌陷后跟随）→ 221.5，焦点题相对容器顶部偏差 **1.4px**；滑动过程中有中间值，非瞬移。
    4. 回归无影响：淡出结束后键帽/提示行仍回落到 `hidden` 终态（键帽 hidden 数 `3,0,3,3,3,3`），静止布局与旧版一致。
  - 进度：✅ 已完成（BUILD-1016 450ms 节奏实测通过；用户确认后已 squash 合并 main，tag v0.56.0）

---

## v0.55.0 — 已完成

> **版本**: v0.55.0

> **状态**: ✅ 已完成（2026-09-13 合并 main，v0.55.0 BUILD-1013）
> **里程碑**: Web UI 图标矢量化——4 个主题图标（暗色/亮色/tp/护眼）与暂停键、主模型、视觉模型图标由系统字体/emoji 字符改为矢量图标（内联 SVG sprite + currentColor），消除跨系统、跨浏览器的渲染差异
> **说明**: 现有图标多为 Unicode 字符与 emoji（`☾ ☀ ♤ ☕︎ ⏸ 🧠 👀`），其字形由操作系统与浏览器字体决定：Windows / macOS / Linux 以及 Chrome / Safari / Firefox 下大小、粗细、基线、配色可能明显不同，且彩色 emoji 无法随主题文字色着色。改为内联 SVG sprite（`<symbol>` + `<use>`，`fill`/`stroke` 用 `currentColor`）后，图标几何由路径定义、颜色继承主题文字色，可保证各系统一致并随 4 个主题自动变色。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-516 | 0.55.0 | P2 | 图标矢量化：4 主题图标（暗色月亮/亮色太阳/tp 黑桃/护眼叶片）+ 暂停（双竖线）+ 主模型（大脑）+ 视觉模型（眼睛），内联 SVG sprite，先出预览页确认后集成 |

> 当前 BUILD: 1013
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-516 图标矢量化（4 主题 + 暂停 + 主模型 + 视觉模型）**
  - 需求：
    1. 以矢量（SVG）替换现有系统字体 / emoji 图标，保证跨系统、跨浏览器渲染一致。
    2. 本次范围：4 个主题图标（暗色、亮色、tp、护眼）+ 暂停键（双竖线）+ 主模型（大脑）+ 视觉模型（眼睛）。
    3. 技术形态：内联 SVG sprite（`<symbol>` + `<use>`），`fill`/`stroke` 用 `currentColor`，随主题文字色自动变色。
    4. 图标语义（用户确认）：暗色=月亮、亮色=太阳、tp=黑桃矢量重绘（延续现有识别符号）、护眼=叶片/植物、暂停=双竖线、主模型=大脑、视觉模型=眼睛。
    5. 交付方式（用户确认）：先出预览页（SVG 源文件 + 总览 HTML，含 4 主题呈现）由用户确认，确认后再集成到 Web UI。
  - 用例：`use-case/FEATURE-516/FEATURE-516-UC-0001.md`
  - 实施（BUILD-1004）：
    1. `web/static/index.html`：`<body>` 后内联 8 个图标的 SVG sprite（`i-theme-dark` / `i-theme-light` / `i-theme-tp` / `i-theme-eyecare` / `i-send` / `i-pause` / `i-model-text` / `i-model-vision`）；`#themeToggle`、`#miModels .mi-ico`、`#sendBtn` 由字符改为 `<svg><use></svg>`。
    2. `web/static/app.js`：新增 `iconHTML(id, cls)` 与 `setIcon(host, id)`；`themeIcon()` 由返回字形字符改为返回 symbol id；`setTheme()` / `setRunning()` 改为替换 `<use href>`；状态栏主模型 / 视觉模型图标改为 `iconHTML("i-model-text"/"i-model-vision")`。
    3. `web/static/style.css`：新增 `.ico`（1em 随字号、继承 `currentColor`、基线对齐）与 `.icon-sprite`，并为主题按钮 16px / 发送按钮 15px / 菜单图标 15px / 状态栏图标 14px 设定尺寸。
    4. 图标几何：月亮、太阳、黑桃、叶片、双竖线为自绘；大脑（`i-model-text`）采用 Feather/Lucide `brain` 几何（ISC 许可，代码注释已标注来源）；四主题图标外接框已统一为 34–36px@48px。
    5. 版本与构建：`main.go` version 0.54.0 → 0.55.0、build 1003 → 1004，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 校验（BUILD-1004）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；独立实例（v0.55.0 BUILD-1004 @28260）DOM 实测：sprite 8 个 symbol 齐备、页面 `<use>` 引用零缺失；主题切换 paper → `#i-theme-eyecare`、light-tp → `#i-theme-tp`、light → `#i-theme-light`、dark → `#i-theme-dark` 均正确；主题按钮图标 16×16、发送按钮 15×15；界面已无 `☾/☀/♤/☕/⏸/🧠/👀` 字符（`app.js:5133` 的工具分类图标属本次范围外，未改）。
  - 第二阶段（BUILD-1006）：状态栏三图标 + 系统设置齿轮 + 设置界面分类图标矢量化
    1. 新增 9 个 symbol：`i-sessions`（对话气泡，原 💬）、`i-sum`（Σ）、`i-timer`（秒表，原 ⏱️）、`i-settings`（齿轮，原 ⚙️；Feather settings 几何）、`i-display`（显示器，原 🖥️）、`i-shield`（盾牌+对勾，原 🛡️）、`i-memory`（打开的书，原 📚）、`i-mcp`（插头，原 🔌）、`i-devtools`（终端窗口，原 🔧）；累计 17 个 symbol。
    2. `app.js`：`settingsGroupIcon()` 由返回 emoji 改为返回 symbol id；`renderSettingsNav()` 改用 `iconHTML()`；状态栏 `sbSessions`/`sbSession`/`sbLast` 三处改用矢量；事件流内任务级 token 汇总行（`Σ2,087,596 (↑… ↓…)`）的 Σ 一并改为矢量以保持同一符号全 UI 一致；移除仅供图标使用的 i18n 常量 `sbSession`/`sbLast`（zh/en 各一处）。
    3. `index.html`：sprite 追加 9 个 symbol；`.mi-ico`（系统设置菜单）改为矢量；设置搜索框 placeholder 去掉 `🔍`（placeholder 无法内嵌 SVG，改为纯文字）。
    4. `style.css`：`.settings-nav-icon` 改 flex 居中并给内部图标 15px；新增 `.sb-last-ico .ico`（13px）与 `.ico-inline`（12px）。
    5. 版本与构建：build 1004 → 1006，co-shell 与 co-shell-hub 已同步编译并原子替换至 `~/bin/`。
  - 校验（BUILD-1006）：`go build ./... && go vet ./...` 全绿；`node --check app.js` 通过；独立实例（v0.55.0 BUILD-1006 @28262）DOM 实测：17 个 symbol 齐备、引用零缺失；设置分类 7 种标题映射依次为 `i-model-text`/`i-display`/`i-shield`/`i-memory`/`i-mcp`/`i-devtools`/`i-settings`；状态栏三项与菜单齿轮均为 `<use>` 矢量；截图确认无豆腐块/错位。
  - 第三阶段（BUILD-1009）：状态栏 hover/间距 + hub 图标矢量化与布局优化
    1. co-shell 状态栏：`.sb-item .ico` 与 `.sb-last-ico` 加 `margin-right:5px`（`.sb-last-ico .ico` 归零避免叠加），并移除图标后紧跟的空格字符（改为 CSS 控制间距，5 处一致）；新增分组 hover 规则：`.sb-sessions`/`.sb-model-wrap`/`.statusbar > .sb-item` 悬停时整组（图标+文字+`b`/`.sb-unit`）同变 accent，悬停目标读作一个整体。
    2. hub 前端（`hub/gateway/webui_static.go`）：新增 9 个 symbol 的内联 SVG sprite（`i-hub-mark`/`i-pin`/`i-chev`/`i-back`/`i-close`/`i-plus`/`i-gear`/`i-send`/`i-check`），替换全部 emoji/字符图标（▸ 📌 ▾ ‹ ✕ ＋ ⚙ 💬 ➤ ✓）。
    3. hub logo 栏：抽屉打开时 `#hubBadge.open` 宽度对齐抽屉（`--panel-w` 340px）、取消右下圆角、抽屉打开期间不淡出；按钉从 Agents 标题移到 logo 栏并右对齐（实测图标 18×18、右留白 15px 与左侧 14px 对称、垂直中心 22 = 栏高 44/2）；图钉改为填充剪影，pinned 时旋转 45°。
    4. hub 抽屉标题：Agents / Chat 去掉左侧 `.mark` 图标，文字靠左对齐。
    5. hub 详情页：去掉右上角关闭图标（`#detailClose` 元素与其 JS 绑定一并移除，仅保留左侧返回）。
  - 校验（BUILD-1009）：`go build ./... && go vet ./...` 全绿；`node --check` 通过（co-shell app.js + hub 内嵌 JS 646 行）；独立实例实测：co-shell 状态栏 5 处图标 `margin-right` 均为 5px、hover 规则已生效、图标后无多余空格；hub 9 个 symbol 齐备、引用零缺失、页面 emoji 计数为 0、logo 栏展开宽度 340 = 抽屉宽、pin 在 logo 栏内右对齐、Agents/Chat 无 `.mark`、`#detailClose` 不存在。
  - 第四阶段（BUILD-1010）：状态栏悬停呼吸效果 + 任务进展面板限高
    1. 状态栏悬停改为「图标呼吸闪烁」：悬停组内图标时图标变 accent 并做 `sbIcoBreath` 透明度脉动（1.1s 循环，1 → 0.2 → 1）；同组文字与数值颜色不再变化（移除了上一阶段的整组变色规则）。
    2. 任务进展面板限高：`renderPlan()` 把任务标题与概要包进 `.plan-head-scroll`（`max-height:200px; overflow-y:auto`），验收标准 `.plan-ac` 同样限高 200px；子任务列表结构不变（仍直接挂在 `.plan-body` 下），样式与行为不受影响。
    3. 版本与构建：build 1009 → 1010，co-shell 与 co-shell-hub 已同步编译并替换到 `~/bin/`。
  - 校验（BUILD-1010）：`go build ./... && go vet ./...` 全绿；`node --check app.js` 通过；独立实例（BUILD-1010 @28264）注入 61 行概要 + 40 条验收标准的测试 plan 实测：概要区 maxHeight=200px / overflowY=auto / clientHeight=200 / scrollHeight=1141，验收标准区 maxHeight=200px / clientHeight=200 / scrollHeight=755（均可内部滚动）；`.plan-step` 仍为 2 个且父元素为 `.plan-body`（结构未变）；hover 规则实测为 `animation: 1.1s ease-in-out infinite` 且仅作用于 `.ico`。
  - 第五阶段（BUILD-1011）：全量图标矢量化（co-shell Web UI 零字符图标）+ 护眼叶片重绘
    1. sprite 扩充：`index.html` 本阶段新增 30 个 symbol——（前 21 个）`i-tri`/`i-tri-down`/`i-menu`/`i-close`/`i-xmark`/`i-clip`/`i-plus`/`i-trash`/`i-expand`/`i-collapse`/`i-image`/`i-file`/`i-download`/`i-dot`/`i-circle`/`i-half`/`i-checkbox-on`/`i-checkbox-off`/`i-radio-on`/`i-tool`，（本阶段新增 9 个）`i-think`（灯泡，替换 💭）、`i-check`、`i-refresh`、`i-retry`、`i-arrow-up`、`i-arrow-down`、`i-tri-up`、`i-copy`、`i-reveal`；累计 46 个 symbol。
    2. `app.js`：新增 `mkIcon(id, cls)`（以 DOM 节点建图标，配合 textContent 避免 HTML 注入）与 `setResultIcon(host, id, text)`；会话列表当前/删除标记、块操作（复制 ⧉ / 重试 ⏪）、在目录中定位（⌖）、模板 JSON 折叠三角（▸/▾）、任务进展状态图标（○/◐/●/✕/✗ → `i-circle`/`i-half`/`i-dot`/`i-close`/`i-xmark`）、交互选项勾选（☑/◉/☐/○）、附件相关（📎/📄/🖼/⬇/✕）、目录变更计数（●）、模型能力标记（👁/🔧/💭）、模型菜单「＋ 新增模型」（两处）、模型连通/API Key/最大长度校验结果（✅/❌）等全部改为内联 SVG。
    3. `index.html`：顶栏 brand-mark（▸）、菜单按钮（☰）、工作区刷新（⟳）、会话运行高亮点（●）、菜单勾选（✓×5）、块导航箭头（↑/↓）、置顶按钮（▲）全部改为 `<svg><use></svg>`。
    4. `style.css`：设置分组标题的 `content:"▸ "`/`"▾ "` 字符三角改为纯 CSS 矢量三角（border 技巧 + open 态 `rotate(90deg)`），不再依赖字体字形。
    5. 护眼叶片重绘（`i-theme-eyecare`）：改为单片饱满叶型（`M4.8 19.2A12 12 0 0 1 19.4 4.6 14 14 0 0 1 4.8 19.2Z`）+ 主脉（`M6.6 17.4 17.6 6.4`），主轴保持斜向 45°；已对比确认不再呈窄长梭形/羽毛感。
    6. 保留项（非图标，未改）：`⚙️ <tool>` 为后端协议标记（`app.js` 用它解析工具块），i18n 文案中的 `＋`、状态栏 token 行的 `↑/↓/Σ` 属文本内容。
    7. 版本与构建：build 1010 → 1011，co-shell 与 co-shell-hub 已同步编译并原子替换到 `~/bin/`。
  - 校验（BUILD-1011）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；独立实例（v0.55.0 BUILD-1011 @28361，workdir `/tmp/fe516-domsite`）DOM 实测：46 个 symbol 齐备、页面 `<use>` 引用零缺失（缺失集为空）、正文与 DOM 中的字符图标残留计数为 0、关键可见图标尺寸正常（brand-mark 14×14、主题按钮 16×16、菜单 14×14、菜单勾选 13×13）；截图视觉确认叶片为「单片饱满叶 + 单条主脉 + 主轴 45°」；验证实例已关闭（未影响主实例 28256）。
  - 第六阶段（BUILD-1012）：图标尺寸与对齐微调（用户反馈）
    1. 标题栏 logo 左侧三角缩小 50%：新增 `.brand-mark .ico { width: 7px; height: 7px; }`（原为 1em = 14px），使文字标记更突出。
    2. 工作区文件树目录三角缩小 20%：新增 `.tree-row .tw .ico { width: 0.8em; height: 0.8em; }`（`.tw` 字号 10px → 8px）；用 em 而非固定 px，使 light-tp 主题下 `font-size:0`（用文件夹图形替代箭头）的行为保持不变。
    3. 「工作区」标签与刷新图标中心对齐：删除 `#treeRefresh` 上为旧字体字形保留的 `transform: translateY(-2.5px)`（该光学微调会让矢量图标整体偏上 2.5px），改由 `.ws-title` 的 flex `align-items:center` 居中。
  - 校验（BUILD-1012）：`go build ./... && go vet ./...` 全绿；独立实例（v0.55.0 BUILD-1012 @28362，workdir `/tmp/fe516-domsite2`）DOM 实测：`.brand-mark .ico` 7×7px（原 14×14，缩小 50%）、`.tree-row .tw .ico` 8×8px（13 个目录节点一致，原 10×10，缩小 20%）、`#treeRefresh` 图标中心 Y=61.2 与「工作区」文字（行盒 18.6px）中心 Y=61.3 相差 0.1px（< 1px 达标）、`transform` 计算值为 none；验证实例已关闭（未影响主实例 28256）。注：截图复核因 CDP `Page.captureScreenshot` 超时未完成，结论以上述 DOM 量化测量为准。
  - 第七阶段（BUILD-1013）：品牌三角与关闭按钮居中（用户反馈）
    1. logo 三角与「co-shell」文字中心对齐：`.brand-mark` 改为 `display:inline-flex; align-items:center`，使三角在标记盒内居中、不再受共享 `.ico` 规则中 `vertical-align:-0.14em` 的基线偏移影响（.brand 的 align-items:center 再将其与文字行盒中心对齐）。
    2. 弹窗关闭按钮 X 居中：`.icon-btn` 改为 `display:inline-flex; align-items:center; justify-content:center; padding:0`——原先按钮沿用 UA 默认 padding（水平 6px）导致 22px 按钮内内容盒被挤窄，叠加 `.ico` 的 vertical-align 偏移，X 看起来未居中。
  - 校验（BUILD-1013）：`go build ./... && go vet ./...` 全绿；独立实例（v0.55.0 BUILD-1013 @28363，workdir `/tmp/fe516-domsite3`）DOM 实测：`.brand-mark .ico` 中心 Y=21.5 与 `.brand-name` 中心 Y=21.5 **差值 0px**（`.brand-mark` display=flex）；弹窗头部关闭按钮 22×22 中心 (928.5, 414) 与内部图标 12×12 中心 (928.5, 414) **dx=0 / dy=0**（按钮 display=flex、padding=0px）；回归检查 `#treeRefresh` 图标与「工作区」文字中心 Y 均为 61.3px（差值 0，第六阶段效果保持）。注：本轮及上一轮截图复核均因 CDP `Page.captureScreenshot` 超时未完成（调试浏览器通道问题，非页面问题），结论以上述 DOM 量化测量为准。
  - 进度：✅ 已完成（第七阶段验收通过，2026-09-13 合并 main，v0.55.0 BUILD-1013）

---

## v0.54.0 — 开发中

> **版本**: v0.54.0

> **状态**: ✅ 已合并到 main（tag v0.54.0）
> **里程碑**: Web UI 交互细化——THINK 块固定高度（200px）+ 可展开；工作区文件列表长名称悬停 1 秒后才自适应加宽；hub 左侧热区悬停 1 秒后才展开 Agents 抽屉；hub 抽屉顶端下移至标题栏之下、改为 Agents/Chat 双分栏可折叠结构、新增专属矢量 favicon
> **说明**: 三处交互优化均针对「鼠标一触即变」带来的干扰：① THINK 块内容全量展开会把事件流撑得很长，按既有 TOOL 输入参数块（`.tool-params-body` 限高 200px + 内部滚动 + 展开按钮）的范式统一；② 工作区侧栏在鼠标**掠过**长文件名时立即加宽（FEATURE-383），导致快速移动鼠标时布局反复抖动；③ hub 左侧热区一碰即弹出 Agents 抽屉，鼠标偶然掠过左边缘会误触发。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-515 | 0.54.0 | P2 | THINK 块固定高度（200px）+ 超高内部滚动 + 常驻展开/收起按钮；文件列长名称悬停 >1s 才自适应加宽；hub 左边缘悬停 >1s 才展开抽屉 |

> 当前 BUILD: 1003
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-515 Web UI 交互优化（THINK 块限高 / 文件列悬停延时 / hub 边栏悬停延时 / hub 抽屉与 favicon）** [BUILD-1003]
  - 需求：
    1. co-shell：THINK 块默认固定高度，内容超高自动滚动（参考其他块中的限制高度内容），用户可展开内容（用户确认：高度 200px、展开按钮始终显示）。
    2. co-shell：鼠标划过工作区文件列表中超长文件/文件夹名并停留超过 1 秒后才扩展工作区为自适应宽度（当前是立即扩展）。
    3. co-shell-hub：鼠标划过左边框附近并停留超过 1 秒，才展开 Agents 边栏。
  - 用例：`use-case/FEATURE-515/FEATURE-515-UC-0001.md`（15 条运行时用例）。
  - 实施（BUILD-986）：
    1. THINK 块限高（`web/static/style.css` + `web/static/app.js`）：`.ev.thinking .ev-body { max-height:200px; overflow-y:auto }`，`.ev.thinking.think-expanded .ev-body { max-height:none }`；`makeBlock` 在 `cls==="thinking"` 时为标题栏 `.ev-actions` 追加常驻展开按钮（`addThinkToggle`，⤢/⤡）；`scheduleMd` 在重渲前记录 `thinkPinned` 并在重渲后把仍在底部的流式块钉到底部，用户上滑时不干扰（新增 `thinkPinned` 辅助函数）。
    2. 文件列悬停延时（`web/static/app.js`）：`treeNode` 的 `row.mouseenter` 不再立即加 `sidebar-auto`，改为 1s 定时器（`treeAutoTimer`），`mouseleave` 清除定时器；仅当 `name.scrollWidth > name.clientWidth`（名称被截断）时启动。
    3. hub 左侧热区延时（`hub/gateway/webui_static.go`）：`edge.mouseenter` 改为 1s 定时器（`edgeTimer`）后调 `openPanel`，`mouseleave` 清除定时器；徽标点击开合与钉住逻辑不变。
    4. 版本与构建：`main.go` version 0.53.0 → 0.54.0、build 985 → 986，`cmd/co-shell-hub/main.go` hubVersion/hubBuild 同步；co-shell 与 co-shell-hub 已编译到 `work/` 并原子替换至 `~/bin/`。
  - 校验（BUILD-986）：`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 与 hub 内嵌 JS 语法检查通过；浏览器实测（co-shell v0.54.0 BUILD-986 @28260、hub @12902）：THINK 长内容 body 200px / scrollHeight 904（可滚动）、按钮 ⤢/⤡ 切换后 maxHeight none/200px、短内容不溢出且按钮仍在、底部跟随只在上滑前生效；文件列短悬停（400ms 离开）不加宽、持续悬停 500ms 仍未加宽、1500ms 已加宽；hub 热区掠过（300ms 离开）不展开、持续悬停 500ms 未展开、1300ms 已展开。
  - hub UI 优化（BUILD-988）：
    1. 抽屉位置：`#agentPanel` 由 `top:0` 改为 `top:var(--topbar-h)`（新增变量 44px），抽屉紧贴标题栏底部且不再遮挡标题栏。
    2. 双分栏抽屉：左边栏拆为 Agents / Chat 两个可折叠区块（`.pane-head` + `.pane-body`）。两块 body 以 `flex:1 1 0` 分配剩余高度，收起时 `flex-grow:0`（`transition:flex-grow .28s` 动效），因此 Agents 展开时 Chat 停靠在抽屉最下方；点 Chat 后 Agents 收起为仅标题、Chat 紧贴 Agents 标题条下方展开并展开自身内容。默认 Agents 展开（关闭/重开恢复默认），进入新建/详情/设置视图会自动展开 Agents；Chat 内容暂为占位 UI（输入框样式 + “功能规划中”提示）。
    3. hub favicon：新增 `hub/gateway/static/favicon.svg`（矢量，主用）与 `favicon.png`（64px 兜底），由 `webui_favicon.go` 以 go:embed 内置，路由 `/favicon.svg`、`/favicon.png`、`/favicon.ico`，页面 head 以 link 标签引用。构图：co-shell 标志居左上、白色对话气泡（深色描边 + 青色三点）居右下约 1/4 区域。
  - 校验（BUILD-988）：`go build ./... && go vet ./...` 全绿；hub 内嵌 JS 语法检查通过；浏览器实测：抽屉 top=44px、默认 Agents 展开（body 581px）/ Chat 收起（body 0px）且 Chat 标题停靠在抽屉最底部、点击 Chat 后 Agents body=0 且 Chat body=581px、Chat 标题与 Agents 标题底间距 0px；favicon 三个路径均 200 且 content-type 正确（image/svg+xml / image/png）。
  - hub 初始化页 logo（BUILD-1000）：空状态 `#empty` 中间的 `▸` 字符改为矢量 logo `<img class="logo" id="emptyRun" src="/favicon.svg">`（96×96、hover 放大 1.08），点击仍打开“新建 Agent”配置面板；点击绑定统一为 `#empty` 容器级事件委托（移除元素级 `onclick`，避免重复绑定，重渲染后依然生效）。
  - hub favicon 矢量图修复（BUILD-1001）：逐像素比对 `favicon.png`（用户确认为准）与 `favicon.svg` 发现：SVG 的气泡（build 996–998 手写的弧线 path）实际渲染已塌缩为右下角约 4.7×6.7 的残片（白区包围盒 (27.3,25.3)-(31.9,31.9)，而 PNG 为 (16.1,17.0)-(30.1,26.9)），根因是弧线两端点与半径不自洽（端点落在外沿椭圆、半径却取中心线椭圆），渲染器选到另一个圆心。改为按 PNG 实测几何重建：`<ellipse cx=23.06 cy=21.97 rx=7.74 ry=5.7 stroke=#9aa3b2 stroke-width=1.47>` + 白色分片 + 尾巴描边（尖端 30.24,28.00），避免弧线圆心歧义。校验：白区/描边包围盒与各角度轮廓半径 Δ≤0.15，尾巴凸起峰值两侧均为 Δ2.69@40°。
  - hub favicon 尾巴交界处修正（BUILD-1002）：用户反馈尾巴与气泡融合处能看到主气泡的细边框把两者隔开。原实现用「三角形」白块覆盖交界处，只盖住了尾巴区域，椭圆描边带靠圆心一侧（θ≈40°）未被覆盖。改为「四边形」白块（θ=48°/25° 两条径向边上各取描边带内、外两点：26.27,25.53 / 28.01,27.46 / 30.24,28.00 / 30.51,25.44 / 28.15,24.34），盖住整条描边带；尾巴描边路径不变。校验：各角度轮廓半径与上一版一致（尾巴凸起峰值 Δ2.64@40°，与 PNG 的 2.64 相同）；尾巴口内的灰色像素经核对均来自尾巴自身两条边线。
  - 进度：✅ 已完成；用户验收通过后已合并到 main（tag v0.54.0，BUILD-1003）。
## v0.53.0 — 已合并

> **版本**: v0.53.0

> **状态**: ✅ 已合并到 main（tag v0.53.0）
> **里程碑**: 循环检测新增“多行等长”维度（触发二次判定）；“移除有问题的上下文”尝试次数上限（默认 0 = 不限制）；监督员按结构化验收标准逐条核对交付物
> **说明**: 代码检视发现三处能力缺口：① 循环检测只有基于“内容重复”的检测（周期重复 / 单行重复 / 单行超长 / 字符级周期 / 跨迭代重复），缺少基于“行长度”维度的检测——等长但内容不同的多行输出（如逐行等宽的排版型/列表型输出）永不触发，也就进不了二次判定；③ `agent/run_stream.go` 中两处“移除有问题的上下文”（LLM 调用出错重试、问题解决模型 delete_last_msg）均无计数与上限，可能无限循环；⑤ 监督员链路已通但缺少结构化“验收标准”维度，无法逐条核对交付物是否达标。（② SUP 流式暴露与 ④ Plan 模式需求挖掘提示词经检视已实现，本次不改。）

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-514 | 0.53.0 | P1 | 循环检测新增多行等长检测；移除上下文尝试次数上限（默认 0=不限制，到达即终止）；监督员按结构化验收标准逐条核对并反馈 |

> 当前 BUILD: 985
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-514 循环检测多行等长 + 移除上下文次数上限 + 监督员验收标准核对**
  - 背景（代码检视结论）：
    1. 循环检测无“行长度”维度：`agent/loop_detector.go` 只有周期重复（内容相同）、p=1 单行重复（数量门限）、单行超长（>2048 字符）、字符级周期、跨迭代内容重复、工具调用重复；等长但内容不同的多行输出不会被任何检测器命中。
    2. `agent/run_stream.go` 两处移除上下文（LLM 调用出错移除最后 assistant+tool_calls 后重试；问题解决模型建议 delete_last_msg）无计数、无上限。
    3. 监督员（FEATURE-456）已覆盖终极目标核对，但 `taskplan` 无验收标准结构字段，监督提示词也未要求逐条核对、未规定“未达标必须打回”。
  - 实施（已编码完成，待用户验证）[BUILD-984]：
    1. 多行等长：`LoopDetector` 增加行长环形缓冲，满足“连续 N 行（默认 100）字符长度完全一致且不构成周期重复”时产生新事件（LoopType=`uniform_line_length`），经 `applyLoopIntervention` 进入二次判定；新增配置 `loop-uniform-line-threshold`；
    2. 移除上下文次数上限：新增配置 `context-remove-limit`（默认 0 = 不限制），两处移除路径计数，达到上限即终止并向用户报告；
    3. 验收标准：`taskplan` 新增 `acceptance_criteria`，`buildSupervisorUserPrompt` 增“验收标准”段，监督系统提示词要求逐条核对、任一条不满足必须打回，`submit_review` 增加 `criteria_check`。  - 校验（BUILD-984）：`go build ./... && go vet ./...` 全绿；新增 `agent/feature514_test.go` 全绿（等长检测 6 例 + 移除上下文上限 3 例 + 验收标准解析/格式化 3 例）；`node --check web/static/app.js` 通过；`go test ./taskplan/ ./i18n/` 通过；agent 包仅剩既有失败 TestAutoIntervention_* / TestStreamSupReply（与本次改动无关，FIX-513 记录中同样存在）；co-shell / co-shell-hub 已编译至 ~/bin（v0.53.0 BUILD-984）。
  - 进度：✅ 已完成并合并到 main（tag v0.53.0）[BUILD-985]。用户验收通过。
  - 附带修复（用户要求查清并修复 3 个既有失败用例，BUILD-985）：
    1. `TestAutoIntervention_BelowThreshold` / `TestAutoIntervention_EscalatesAtThreshold`（`agent/loop_retry_limit_test.go` 夹具 + `agent/loop_auto_test.go` 断言）：根因是测试夹具用 `&config.Config{}` **零值**构造 Agent，而 FEATURE-471 的 `<environment_details>` 开关（`EnvIncludeDetails` 等 bool）在零值下为 false，导致 `buildFullEnvironmentDetails` 返回空串——新增的反馈消息不带 env，`<retried_count>` 永远读到 0。**真实运行不受影响**：`LoadFromFile` 先 `DefaultConfig()` 再 `json.Unmarshal`，旧配置缺字段时保留默认 true。修复：夹具改用 `config.DefaultConfig()` 作为基线（贴近真实运行时配置）。
    2. `TestStreamSupReply`（`agent/sup_stream_test.go`）：根因是**测试期望过时**——FEATURE-461 之后 `streamSupReply` 除转发 content 块外，还会通过 `emitSupTool` 把工具调用输入作为 SUP 块底部段转发（共 3 个事件）。修复：断言改为 3 个事件，并分别校验 `sup_part=content`（part1/part2）与 `sup_part=tool`（`submit_review` 段文本），断言强度不降反升。
    校验：`go build ./... && go vet ./...` 全绿；`go test ./agent/ ./taskplan/ ./i18n/ -short` 全绿（agent 包此前 3 个失败已全部消除，无新增回归）。

## v0.52.1 — 已合并

> **版本**: v0.52.1

> **状态**: ✅ 已合并到 main（tag v0.52.1）
> **里程碑**: 修复 v0.52.0 已知缺陷——Web UI 下 `ask_user` 的答案未送达 LLM（FIX-513）
> **说明**: v0.52.0（FEATURE-512）上线 `ask_user` 多题收集后，Web UI 场景下用户提交的答案**从未到达 LLM**：浏览器发出的 `result.answers` 被 `web/server.go` 中手工维护的 wire DTO `interactionResultJSON`（仅 Action/Value/Raw）静默丢弃，`web/session.go` 组装 `InteractionResult` 时也没有 Answers；`agent/ask_user.go` 的 `ActionSubmit` 分支在 `len(res.Answers)==0` 时直接 `return "", nil`，既不回传答案也不调用 `storeUserReply`。前端「YOU · 回答」只是 `app.js` 本地渲染，掩盖了后端实际未收到答案的事实。本次修复同时删除手工镜像 DTO（根除“新增字段漏加”类回归）并补齐 web 层回归测试。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-513 | 0.52.1 | P1 | Web UI 下 `ask_user` 答案未送达 LLM：wire 层丢弃 Answers + agent 侧空答案兜底 |

> 当前 BUILD: 983
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-513 Web UI 下 ask_user 答案未送达 LLM（修复 v0.52.0 已知缺陷）** [BUILD-983]
  - 现象（用户报告 + 实测确认）：Web UI 上完成多题表单并提交后，下一轮 LLM 看不到任何答案；工具结果为空（渲染为「（工具调用无输出）」）。
  - 根因（三处叠加）：
    1. `web/server.go`：`clientMessage.Result` 使用手工镜像 DTO `interactionResultJSON`（字段仅 Action/Value/Raw），浏览器发出的 `answers` 在反序列化阶段即被丢弃。
    2. `web/session.go`：`interaction_answer` 分支组装 `agent.InteractionResult` 时未传 Answers。
    3. `agent/ask_user.go`：`ActionSubmit` 在 `len(res.Answers)==0` 时 `return "", nil` —— 空工具结果且不调用 `storeUserReply`，LLM 完全看不到回答。
    4. 误判来源：前端「YOU · 回答」是 `app.js` 本地渲染，与后端是否收到答案无关；终端路径正常，agent 单测缺少 web wire 覆盖，故漏检。
  - 修复：
    - wire 层（A2，根除镜像 DTO 类回归）：删除 `interactionResultJSON`，`clientMessage.Result` 改为 `json.RawMessage`，`session.go` 直接 `json.Unmarshal` 到 `agent.InteractionResult`（解析失败记 `log.Warn` 后忽略）。
    - agent 兜底：`ActionSubmit` 有 Answers 时按原逻辑格式化入库；Answers 为空但 `Value` 非空时用 `Value` 作为用户回复；两者皆空时返回新增 i18n 文案（`KeyToolAskUserNoAnswer`，zh「未收到答案」/ en「No answer received」）而不再返回空串；`default` 分支同样返回该文案。
  - 校验（BUILD-983）：`web/interaction_answers_test.go` 新增 `TestWebIOAskInteractionAnswers`（含 selected/notes/text 的 answers 完整往返）与 `TestInteractionAnswerWireCarriesAnswers`（wire JSON 编解码断言）；`agent/ask_user_test.go` 新增 `TestAskUserSubmitFallsBackToValue` / `TestAskUserSubmitNoAnswers` / `TestAskUserUnknownAction`；`go build ./... && go vet ./...` 全绿；`go test ./agent/ ./web/` 通过（agent 包仅既有失败 TestAutoIntervention_* / TestStreamSupReply，与被改动代码无关）；co-shell / co-shell-hub 编译至 ~/bin。
  - 运行时验收（A/B 对照，用户实测）：旧构建 v0.52.0 BUILD-982 下提交同一道三题表单，LLM 侧收到「（工具调用无输出）」；重启到 v0.52.1 BUILD-983 后重新提交，LLM 侧完整收到三题答案（含多选备注与自由文本），验收通过。
  - 进度：✅ 已完成并合并到 main（tag v0.52.1）[BUILD-983]。

## v0.52.0 — 已合并

> **版本**: v0.52.0

> **状态**: ✅ 已合并到 main（tag v0.52.0）
> **里程碑**: 提问工具升级为「一次收集多题答案」（每题单选/多选 + 选项补充说明），并更名为 `ask_user`（FEATURE-512）
> **说明**: 当前内置提问工具 `ask_followup_question` 只支持「一个问题 + 单选 + 一条补充说明」，而 LLM 经常需要一次性确认多个决策点（如「用哪个数据库 + 是否要迁移 + 是否保留旧表」），只能把多个问题硬塞进一个问题的选项里，导致选项语义混乱、用户难以逐题作答。本次将该能力升级为结构化问卷：一次下发多道题目，每题独立支持单选或多选，选项可附加补充说明，Web UI 上每题以独立卡片呈现并做视觉分隔，终端逐题作答；同时方法更名为 `ask_user`，为将来承载图片/文件等更多交互形态留出语义空间。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-512 | 0.52.0 | P1 | `ask_user` 支持多题收集（单选/多选/选项补充说明），Web 端分题卡片渲染、终端逐题作答 |

> 当前 BUILD: 982
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-512 ask_user 多问题收集能力升级**
  - 需求（用户提出）：LLM 经常会一次问多个问题以提高效率，而现有内置提问工具只支持单个单选问题，导致 LLM 只能把多个问题塞进同一个问题的选项里，选项不足以覆盖全部待确定项。需要升级为：一次收集多个问题的答案；每题可单选可多选；每个选项可在选择基础上补充说明；界面上用某种方式分隔不同的题目。同时讨论方法命名。
  - 命名决策（用户确认）：更名为 `ask_user`（最简洁，为将来支持图片/文件等交互留语义空间），旧名 `ask_followup_question` 不再注册。
  - 现状（调研结论）：工具注册于 `agent/tools.go`（`ask_followup_question`，参数 `question` 必需 / `options` 可选），回调 `askFollowupQuestionTool` 构造统一交互模型 `agent.Interaction`（`kind=select` 或 `input`，固定 `-`/`+` 键）→ `InteractionManager.Ask`；Web 端由 `web/session.go` 的 `WebIO.Ask` 下发 `interaction` 消息、`web/static/app.js` 的 `showInteraction()` + `renderVirtualKeyboard()` 渲染；终端由 `agent/interaction.go` 的 `TerminalInteractionManager.askSelect` 渲染。当前模型只有单一 `Options []string`，无法表达多选题。
  - 设计：
    - 工具层：`ask_user` 参数改为 `questions`（数组，1..N，每题 `title` 必需、`options` 可选、`multi` 可选默认 false、`allow_note` 可选默认 true）；兼容旧的 `question`/`options` 写法（归一化为单题），避免历史提示词与外部调用失效。
    - 交互模型：`agent/interaction.go` 新增 `Question` 结构与 `Interaction.Questions` 字段、新增 `kind=questions`；`InteractionResult` 新增 `Answers []QuestionAnswer`（每题含 `selected[]` / `note` / `text`）。现有 `confirm`/`select`/`input`/`key` 语义保持不变。
    - Web UI：每题一个独立卡片（序号 Q1/Q2 + 题面 markdown + 选项区），题目之间以分隔线/留白视觉分隔；单选渲染为 radio 样式、多选渲染为 checkbox 样式；每个选项提供「补充说明」入口（展开为输入框，附加到该选项）；底部一个「提交」按钮一次性回传全部答案。
    - 终端：逐题渲染（Q1/N 题面 + 选项），多选以逗号分隔作答（如 `1,3`），补充说明沿用现有「空格进入补充」方式，全部答完后一次性回传。
    - 回传：把每题答案格式化为可读文本（题面 + 所选项 + 补充说明）写入 task instruction cache 并作为工具结果返回，保证 LLM 能准确对应题与答。
  - 验收：① LLM 一次调用可下发多道题，Web 端每题独立卡片且视觉分隔清晰；② 单选/多选均能正确收集，选项补充说明能附加到所选选项；③ 终端可逐题作答、多选逗号分隔；④ 旧的 `question`+`options` 调用仍可用；⑤ 答案完整回传 LLM 上下文。
  - 实现（BUILD-974）：
    - 新增 `agent/ask_user.go`（工具声明 + `questions` 归一化 + 回调 + 摘要文本）与 `agent/interaction_questions.go`（终端逐题渲染、选项解析、答案格式化）；`agent/interaction.go` 新增 `Question`/`AnswerNote`/`QuestionAnswer` 结构、`Interaction.Questions`、`InteractionResult.Answers`、`kind="questions"` 与 `ActionSubmit`。
    - 工具改名：`agent/tools.go` 注册改为 `buildAskUserTool`（回调 `askUserTool`），`agent/agent.go`、`config/config.go`、`agent/toolcall_mode.go`、`agent/tool_summary.go`、`agent/tool_error.go` 与 i18n 文案/提示词全量同步；旧名 `ask_followup_question` 不再注册。
    - Web：`web/static/app.js` 新增 `renderQuestions()`（每题卡片 + 单选/多选标记 + 每选项补充说明 + 统一提交按钮，题间虚线分隔，提交按钮显示已答进度），`web/static/style.css` 新增 `.qs-*` 样式；答案以 `{action:"submit", answers:[...]}` 回传。
    - 终端：`askQuestions` 逐题渲染，多选以逗号分隔（如 `1,3`），`-` 退出整份表单，空格开头或「序号 + 空格 + 文本」作为选项补充说明；无选项题直接接收文本。
  - 验证（BUILD-974）：
    - 新增 `agent/ask_user_test.go`（XML/JSON 解析、交互结构、工具声明、JSON 往返、选项解析）并重写 `agent/interaction_test.go` 中的 ask_user 用例；`go test ./agent/ -run 'AskUser|Questions|SplitOption|Interaction|TerminalSelect'` 全部通过。
    - `go build ./... && go vet ./...` 全绿；co-shell / co-shell-hub 已编译至 ~/bin（v0.52.0 BUILD-974）。
    - 已知基线失败（与本次改动无关）：`agent` 包 `TestAutoIntervention_BelowThreshold` / `TestAutoIntervention_EscalatesAtThreshold` / `TestStreamSupReply`。
  - 增量改进（用户实测后提出，BUILD-975）：
    - 答案消息化：`ask_user` 的作答内容在前端以用户消息块渲染，标题为「YOU · 回答」（`renderUserEcho(text, {answer:true})`），以区别于用户键入的命令；答案文本与后端回传格式一致（题面 + 选择 + 回答 + 选项补充 + （未作答））。
    - 多题快捷键：仅当前题显示 1-9 选项角标；数字键选中（单选选中后自动跳下一题，多选可反复勾选）；`-` 下一题、`+` 上一题；空格打开该题补充说明输入；底部显示快捷键提示行。
  - 运行时验证（BUILD-975，独立实例 127.0.0.1:28260，不影响会话实例）：
    - 渲染：注入 3 题表单 → 3 张卡片、仅当前题显示 1-3 角标、当前题高亮、快捷键提示行文案正确。
    - 快捷键：`2`→A2 选中（多选停留本题）、`3`→A2+A3 复选、`-`→切到第 2 题（角标随之切换）、`1`→B1 选中并自动跳到第 3 题、`+`→回到第 2 题、空格→打开该题补充说明输入。
    - 答案消息化：提交后事件流新增标题为「YOU · 回答」的用户消息，内容为「Q1 题面/选择: A2 + A3；Q2 选择: B1 + 回答: 补充；Q3 回答: 自由文本」，表单正常收起。
  - 第三轮优化（用户实测后提出，BUILD-976）：
    - 快捷键：`-`（下一题）改为 **Enter**（末题 Enter 直接提交）；`+` 仍为上一题；数字键短按选择、**长按 ≥450ms** 打开该选项的备注输入框（回车失焦即恢复快捷键监听）。
    - 焦点标识改为 **Qn 徽标高亮**（`qs-index.hot`），快捷键提示行只显示在**当前焦点题**下方。
    - 切换焦点题时自动 `scrollIntoView({block:"start"})` 将当前题滚动到可视区顶部；无选项的自由输入题在获得焦点时自动聚焦其输入框，Enter 进入下一题。
    - 答案文本（界面与后端回传模型一致）简化为每题一行：`Q1: A + C（备注: A→xxx）`、`Q2: 文本`、`Q3: （未作答）`，不再重复题面；新增 `TestFormatQuestionAnswersCompact` 覆盖该格式。
    - 浏览器实测（BUILD-976）：长按数字弹出该项备注框 → 填写后 Enter 失焦；Enter 逐题推进；单选选中即跳题；自由题自动聚焦且 Enter 提交末题；提交后「YOU · 回答」正文为 `Q1: A2（备注: A2→备注内容）/ Q2: （未作答）/ Q3: 自由文本答案`。
  - 第四轮优化（用户实测后提出，BUILD-977）：
    - 焦点题 Qn 徽标改为**反色实心高对比**（`background: var(--accent); color: var(--bg); font-weight:700`），非焦点题为描边镂空。
  - 第五轮优化（用户实测后提出，BUILD-978）：
    - 必填题：`questions[].required`（默认 false）→ `Question.Required`；Web 端必填徽标 `qs-req`，提交前校验未作答的必填题（阻止提交 + Qn 徽标高亮 `qs-missing` + 滚动到首个缺失题 + 按钮旁提示「必填题未作答 Q1, Q3」）；工具 schema 与中英文系统提示词用法同步补充 required 说明。
    - 提示行动态生成 + 键位方框：按当前题生成 `[1]-[N] 选择 · 长按[1]-[N] 填备注 · [回车] 下一题/确定 · [+] 上一题 · [空格] 备注`（无选项题省略数字键部分），数字范围取当前题选项数，末题 Enter 标注为「确定」；新增 `kbdSpan()` 与 `.qs-kbd` 键帽样式，替换原固定文案 `qsHotkeyHint`。
    - 底部按钮键位：提交按钮文案改为 `[回车] 确定 (n/N)` 且仅在最后一题显示（其他题 Enter 语义为「下一题」）；取消按钮前置 `[-]` 键位方框。
    - 校验：`agent/ask_user_test.go` 新增 required 解析断言（XML `<required>true</required>` 与 JSON `"required": true`）与 schema `questions[].required` 断言；`node --check web/static/app.js` 通过；`go build ./... && go vet ./...` 全绿；`go test ./agent/ -run 'AskUser|Questions|SplitOption|Interaction|TerminalSelect|Format'` 通过；co-shell / co-shell-hub 已编译至 ~/bin（v0.52.0 BUILD-978）。
  - 第六轮优化（用户实测后提出，BUILD-979）：
    - 取消更顺手：每题提示行末尾新增 `[-] 取消`（与底部取消按钮同一动作 think_exit）。
    - 提示行键帽改为**可点击的大键帽**（`button.qs-kbd`，accent 描边 + 悬停反色）：点数字选选项（单选自动跳题）、点「长按」组数字开该项备注、点 [回车] 下一题/提交、[+] 上一题、[空格] 本题备注、[-] 取消；点击后自动 blur，保证键盘快捷键仍可用。
    - 「必填」徽标移到**题面文本末尾**（同行行尾；mdRender 输出块级元素，因此徽标挂到最后一个块元素内）。
    - 修复 BUILD-978 引入的运行时缺陷：`qsWarn` 未声明导致 `renderQuestions` 收尾与提交校验分支抛 ReferenceError（表现为点提交无反应、焦点不回跳）；现声明为标准交互状态变量。
    - 必填校验回跳：末题提交时若存在未作答必填题，阻止提交并高亮/滚动/**聚焦到第一个未作答的必填题**，如此往复直至无缺失项才提交成功。
    - 验证（独立实例 127.0.0.1:28260 + 临时工作区，不影响会话实例）：注入 4 题表单实测 —— 点击数字键帽选中并自动跳题；末题提示行无数字键段且显示 `[回车] 确定 (n/N)`；未答必填时提交被拦截（Q2/Q4 高亮 + 「必填题未作答 Q2, Q4」+ 焦点回到 Q2）；补齐后提交成功，答案文本为紧凑格式 `Q1: 正确 / Q2: 数字选择 / Q3: （未作答）/ Q4: 自由文本答案`。
    - 校验：`node --check`、`go build ./... && go vet ./...`、`go test ./agent/ -run 'AskUser|Questions|SplitOption|Interaction|TerminalSelect|Format'` 全绿；co-shell / co-shell-hub 已编译至 ~/bin（v0.52.0 BUILD-979）。
  - 第七轮优化（用户实测后提出，BUILD-980）：
    - 键盘 `-` 可直接取消整份表单（此前只监听了回车/数字/+/空格，键盘 `-` 无响应，只有渲染出的 `[-]` 键帽与底部按钮可用）。
    - 选项左侧的数字键帽改为**老版尺寸**（34×34，与 `ask_followup_question` 的 `.opt-key-btn` 一致）并**可点击**：单击选中该选项（单选自动跳题），长按 ≥450ms 打开该选项备注框。
    - 提示行简化：去掉开头的 1-N 数字键帽与「长按」处的数字键帽，改为纯文字 `数字键选择 · 长按选项数字键填备注`，仅保留 `[回车]`/`[+]`/`[空格]`/`[-]` 四个可点击键帽。
    - Qn 徽标与题面文本**基线对齐**（`.qs-head` 改 `align-items: baseline`，并清除 markdown 段落的默认边距——此前题面被 `<p>` 的默认 margin 顶到下一行）。
    - 验证（独立实例 127.0.0.1:28260 + 临时工作区）：实测键盘 `-` 触发 `select:think_exit`；提示行仅 4 个可点击键帽且文案正确；选项键帽 34×34，单击即选中并跳题、长按 450ms 打开并聚焦备注框；Qn 徽标与题面 top 差 0px。
    - 校验：`node --check`、`go build ./... && go vet ./...` 全绿；co-shell / co-shell-hub 已编译至 ~/bin（v0.52.0 BUILD-980）。
  - 第八轮优化（用户实测后提出，BUILD-981）：
    - 提示行内的快捷键键帽（`回车` / `空格` / `-` / `+`）放大到与选项键帽同级：`min-width/height: 34px`、`font-size: 13px`、实心 `--bg-elev` 底 + accent 描边，悬停反色，与 `.qs-hotkey`/`.opt-key-btn` 视觉统一，便于鼠标点击。
    - 底部提交与取消按钮改为 `inline-flex` 对齐，保证按钮内“键帽 + 文字”垂直居中且间距一致。
    - 验证（独立实例 127.0.0.1:28260）：DOM 实测提示行键帽 `回车` 为 44×34、选项键帽 34×34（高度一致），底部取消按钮内键帽 34×34；`node --check`、`go build ./... && go vet ./...` 全绿；co-shell / co-shell-hub 已编译至 ~/bin（v0.52.0 BUILD-981）。
  - 进度：✅ 已完成并合并到 main（tag v0.52.0），共九轮增量（BUILD-974 → BUILD-982）；前端为 embed 资源，重启服务后生效。

---

## v0.51.2 — 开发中

> **版本**: v0.51.2

> **状态**: 🚧 开发中（FIX-511）
> **里程碑**: 修复 Web UI 页面刷新 / 重新连接后运行态控件与后端不一致，并加固“断开连接”后的消息接收语义（FIX-511）
> **说明**: 运行态的界面表现（logo 呼吸、发送/中断按钮的红色 ⏸ 态、会话标题高亮点）完全由前端 `setRunning()` 驱动，而它只接收实时 WS 事件 `turn_start` / `await_input`；这两条事件由 `WebSession.ReadLine` 直接 `sendEvent`，不经过渲染器，因此不会被持久化到事件流，页面刷新后的历史回放里没有它们。同时前端在 WS 连接建立时只请求 mode/session/yolo/history，从不查询当前运行态，导致刷新后界面停留在空闲态（后端其实一直在跑）。本次复用连接时已有的 `kind:"state"` 状态快照通道携带 `busy`，让刷新与重连都能立即恢复运行态；同时按“断开即停止接收、重连即重新同步”的语义加固套接字生命周期（在途消息丢弃、旧套接字回调不污染新连接、重连重建事件流避免重复回放）。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-511 | 0.51.2 | P1 | 连接时 state 快照携带 busy 以恢复运行态外观；断开后忽略在途消息与旧套接字回调；重连重建事件流避免重复追加 |

> 当前 BUILD: 973
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FIX-511 运行态同步与断连语义（刷新/重连后显示运行态控件与后端不一致）**
  - 现象（用户报告）：任务正在运行时刷新页面，logo 失去呼吸效果、运行/暂停按钮显示为 ▶ 而非红色边框的 ⏸，运行态与后端不一致。
  - 根因 1（状态无来源）：前端 `running` 仅由实时 WS 事件 `turn_start` → `setRunning(true)` / `await_input` → `setRunning(false)` 驱动（`web/static/app.js:1304-1305`）；这两条事件由 `WebSession.ReadLine` 直接 `srv.sendEvent` 发出（`web/session.go:1007`），**不经过渲染器**，因此不会走 `r.coalescer.Add(ev)` → `AppendEvent` 的持久化路径（`web/session.go:1285-1294`）→ 历史回放中不存在 turn 边界事件。
  - 根因 2（连接时不查询）：`ws.onopen`（`web/static/app.js:383-399`）只发送 `mode_get` / `session_list` / `yolo_get` / `history_get`，从不查询当前运行态，于是 `boot()` 的 `setRunning(false)`（`app.js:6049`）一直生效。
  - 根因 3（后端已有状态却未下发）：`agent.IsBusy()`（`agent/loop.go:419`，`RunStream` 进入 `SetBusy(true)`、退出 `SetBusy(false)`）已通过 FEATURE-499 注入服务端 `busyFn`（`web/session.go:137`），并暴露为 `GET /api/status`（`web/server.go:618`）；而连接时已有的状态快照通道 `handleWS` → `sendState`（`web/server.go:540`、`500-515`）只携带任务计划，未携带 `busy`。
  - 根因 4（断开后仍接收）：`ws.onmessage`（`app.js:409`）没有任何连接态/套接字归属校验，点击“断开连接”后已在途（浏览器任务队列或 CLOSING 握手期间的）消息仍会被 `renderEvent` 渲染；且 `ws.onclose` 未校验套接字归属，快速“断开 → 重连”时旧套接字的回调会把新连接的 `wsReady` 置 false 并将状态重置为空闲，污染刚同步好的状态。
  - 根因 5（重连重复回放）：`renderHistory` 的非 prepend 分支只追加不清空（`app.js:1833-1841`），而无游标的 `history_get` 只在 `ws.onopen` 发出（`app.js:395`）→ 重连（未刷新页面）时会把最近一页消息在已有事件流后重复追加。
  - 方案（用户确认，方案 C）：复用连接时既有的 `kind:"state"` 快照通道下发 `busy`，前端在 `state` 分支调用 `setRunning(busy)`；服务端在每次连接建立（含重连）时推送该快照，天然满足“重连先同步状态”。
  - 验收：任务运行时刷新 → logo 呼吸、按钮为红色边框 ⏸、会话标题高亮点呼吸；断开后事件流不再新增内容；重连后恢复运行态外观且不出现重复消息。
  - 实现（BUILD-973）：
    - `web/server.go`：`serverMessage` 新增 `Busy *bool`（指针 + omitempty，保证 `false` 仍会序列化，前端可区分“无该字段”与“非运行”）；`sendState` 同时读取 `planFn` 与 `busyFn`（即 `agent.IsBusy()`）并写入快照。
    - `web/static/app.js`：`wsConnect` 为每个连接绑定独立套接字（`const sock`），`onopen`/`onmessage`/`onclose` 均校验收属（`sock !== ws` 时直接返回），避免旧连接回调污染新连接（快速断开→重连场景）；`onmessage` 增加 `!wsReady` 守卫，断开后不再处理任何在途消息；`state` 分支读取 `msg.busy` 并调用 `setRunning(busy)`；新增 `resetStreamView()`，在每次建立连接、请求历史前清空事件流并重置渲染簿记（在途块引用/工具块映射/消息索引/分页状态/令牌统计），使重连表现为“按后端持久化历史重建”而非重复追加。
  - 验证（BUILD-973）：
    - 单元测试：新增 `web/fix511_test.go`（UC-0010/0011）—— `sendState` 在 busyFn 为 true/false/nil 三态下均正确输出 `busy`（nil → false），无 provider 时 `plan` 为 null 且可正常序列化，plan 透传不受影响；`go test ./web/` 全部通过。
    - 浏览器实测（独立实例 v0.51.2 BUILD-972，workspace /tmp/fix511，serve 端口 28260）：
      - UC-0001：长任务运行中刷新 → `logo=true / streamActive=true / 按钮=⏸+run / running=true`，同时刻后端 `/api/status` 为 `busy=true`（修复前必为 false）。
      - UC-0002：空闲时刷新 → 保持空闲（不误显运行态）。
      - UC-0004：运行中点“断开连接” → 断开前 `.ev` 17 个、断开 7s 后仍为 17 且 `running=false`（在途消息被完全丢弃，事件流冻结）。
      - UC-0003/UC-0005/UC-0006：重新连接 5s 后 → `logo=true / active=true / 按钮=⏸+run / running=true` 且 `.ev` 仍为 17（未被翻倍，证明重建而非追加重放）；旧套接字回调未污染新连接。
      - UC-0007：重连后发送新输入可正常执行并渲染（实测“回复 ok”任务成功）。
      - UC-0008/UC-0009：未单独构造交互场景，其语义由 `busy`（`RunStream` 全程为真，含等待交互）与 UC-0010 单元测试覆盖。
    - `go build ./... && go vet ./...` 全绿；`go test ./...` 中 `agent` 包的 3 个失败（TestAutoIntervention_BelowThreshold / _EscalatesAtThreshold / TestStreamSupReply）与 `cmd` 包 TestWebWizardModelNameStep 的 panic（`fetchModelSuggestions` 网络探测失败导致空指针）均为 main 基线既有/环境相关，本次改动未触及这两个包的代码（仅版本常量）。
  - 进度：已在分支 FIX-511 完成开发、自测与编译（co-shell / co-shell-hub 已更新至 ~/bin）[BUILD-973]，等待用户确认后合并。

---

## v0.51.1 — 已合并

> **版本**: v0.51.1

> **状态**: ✅ 已合并到 main（tag v0.51.1）
> **里程碑**: 修复上下文超限时 reorganize_context / attempt_completion 被跳过，以及 reorganize_context 必需 meta 导致调用失败（FIX-510）
> **说明**: 上下文占用超过 context-reorganize-threshold 时，原逻辑跳过本轮全部工具执行，仅豁免 reorganize_context；attempt_completion（报告完成）被一并跳过；而 reorganize_context 自身还因 required 含 meta 而可能被 meta 校验判失败（skipExec），导致超限时首次调用几乎无法成功。本次把二者纳入超限豁免集合，并移除 reorganize_context 的必需 meta。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-510 | 0.51.1 | P1 | 超限跳过工具执行时豁免 reorganize_context / attempt_completion；移除 reorganize_context 的 required meta |

> 当前 BUILD: 972
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-510 超限时豁免 reorganize_context/attempt_completion 并移除其必需 meta**
  - 根因：`agent/run_stream.go:1265-1323` 在 `usagePct >= ContextReorganizeThreshold` 时置 `reorganizePending=true` 跳过本轮全部工具执行，仅当本轮 toolCalls 含 `reorganize_context` 时豁免（`hasReorganizeCall`）；`attempt_completion` 未豁免，报告完成被一并跳过。
  - 叠加缺陷：`agent/tools.go:809` 中 `reorganize_context` 的 required 含 `"meta"`，`risk.go` 的 `toolRequiresMeta` 据此执行 `assessRisk` 强校验；meta 缺失或 risk 非法 → `skipExec`，超限场景下首次重组高概率失败。
  - 方案（用户确认）：① 豁免集合扩展为 `reorganize_context` + `attempt_completion`；② 移除 `reorganize_context` 的 required `"meta"`（properties 保留为可选）；③ 修正 `risk.go` 注释与代码的不一致。
  - 实现（BUILD-970）：
    - `agent/reorganize_context.go`：新增豁免集合 `contextOverflowExemptTools`（`reorganize_context` + `attempt_completion`）与判定函数 `hasContextOverflowExemptTool()`；`agent/run_stream.go` 的超限跳过分支改用它判定（保持“整轮豁免”语义，避免部分 tool_call 无结果导致 API 400），紧急重整指令的触发条件同步改名。
    - `agent/tools.go`：`reorganize_context` 的 `required` 去掉 `"meta"`（properties 保留 → meta 变为可选），不再受 `assessRisk` 校验。
    - `agent/toolcall_mode.go`：新增 `toolRequiresMetaIn` / `toolsWithoutMetaNames` / `applyNoMetaToolsHint`；`buildXMLToolPrompt` 在 XML 模式的 meta 说明中注入 `{NO_META_TOOLS}`（运行时按工具 schema 生成“不需要 meta 的工具”清单，与 required 不漂移）；`risk.go` 的 `Agent.toolRequiresMeta` 改为复用纯函数，并修正与代码不符的注释。
    - i18n：`KeySystemPromptToolUsageMetaXML`（中/英）新增 `{NO_META_TOOLS}` 占位符与 `KeySystemPromptMetaExemptTools` 模板；`reorganize_context` 的工具用法中 meta 标注为可选（中文原先未提 meta，一并补齐）。
    - 说明：OpenAI 模式不发文本清单（工具以 JSON schema 下发，`required` 对模型可见），因此 `KeySystemPromptToolUsageMetaOpenAI` 不含占位符。
    - 另发现（已记录）：`buildToolsInternal` 仅在 `intent-exposure` 开启时保留 meta（关闭时统一剥离），因此“移除必需 meta”实际作用于意图透明化开启的场景。
  - 验证（BUILD-970）：`go build ./... && go vet ./...` 全绿；新增 `agent/fix510_test.go`（豁免判定 6 例、reorganize_context 不再要求 meta、免 meta 清单生成、无 meta 调用成功）与 `i18n/fix510_test.go`（占位符/模板）全部通过；`go test ./i18n/` 通过；`agent` 包仅剩 3 个**既有**失败（TestAutoIntervention_BelowThreshold / _EscalatesAtThreshold / TestStreamSupReply），已用 main 基线 worktree 复核确认改动前即失败。`meta_param_test.go` 按新行为补充 reorganize_context 例外分支；测试中不再调用 `i18n.Init`（避免污染同进程语言状态）。
  - 追加修复（用户实测反馈，BUILD-971）：超限时工具已豁免放行并执行成功，界面仍出现 SYS 警告块「上下文超限 (26.0% > 25%)，已跳过此轮工具执行」。根因：`run_stream.go` 中该警告（及 FEATURE-345 问题模型咨询/可能的终止任务）位于 `usagePct >= threshold` 分支内，而豁免判定 `hasContextOverflowExemptTool()` 在其后才计算 → 告警条件与跳过条件不一致。修复：新增 `contextOverflowSkipsTools(usagePct, threshold, toolCalls)`（`agent/reorganize_context.go`），并将「是否跳过」与「是否告警/咨询问题模型」都改为由它决定；超限且携带豁免工具时仅写 info 日志，界面完全静默（不咨询问题模型，避免其建议 stop 而终止任务）。新增 `TestContextOverflowSkipsTools`（6 例）与用例 UC-0013（含“普通工具仍应告警”对照场景）。
  - 进度：已完成并合并到 main（tag v0.51.1）[BUILD-972]

---

## v0.51.0 — 已合并

> **版本**: v0.51.0

> **状态**: ✅ 已合并到 main（tag v0.51.0）
> **里程碑**: hub agent 公告板协作机制（FEATURE-490 合并落地）
> **说明**: 把 2026-09-08 基于 v0.42.0 开发的 hub agent 公告板协作能力合并到当前基线：hub 侧公告板中枢（广播感知 / 认领 / 私信 / 确认 / 主动下发执行 / 结果回传）+ co-shell 侧 6 个 board 工具与 3 类动态感知事件 + `board_task` 主动执行注入；同时补齐当年缺失的 i18n、工具使用说明映射、配置暴露（:set 与 Web 面板）与 co-shell 侧单测。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-490 | 0.51.0 | P1 | hub agent 公告板协作机制合并落地：合并分支 FEATURE-490 + 解决 3 处冲突 + 补齐 i18n / 工具使用说明映射 / 配置暴露 / co-shell 侧测试 |

> 当前 BUILD: 969
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-490 hub agent 公告板协作机制（合并落地）**
  - 背景：FEATURE-490 于 2026-09-08 在 v0.42.0 基线上完成（单提交 `631c1f3`，+1264/−4，17 文件），但一直未合并进 main。实测确认 main 中没有任何 board 相关代码（`BoardEnabled` / `board_enabled` / `board_task` / `hub/gateway/board.go` 均不存在）。
  - 实测合并预演：`git merge-tree main FEATURE-490` 仅 3 个文件冲突（`ROADMAP.md`、`main.go`、`web/server.go`），其余 14 个文件自动合并；hub 侧 `proxy.go`/`agent.go` 自 merge-base 起 main 一行未改（零漂移）。
  - 本次补齐的规范缺口：① board 6 个工具的 Description/参数描述零 i18n；② `agent/toolcall_mode.go` 缺 board 使用说明映射；③ `BoardEnabled` 未接入 `:set` 与 Web 设置面板；④ co-shell 侧无任何测试。
  - 分支：**FEATURE-490-merge**（基于 main 新建，与原分支 `FEATURE-490` 解耦）；版本：**v0.51.0**。
  - 原实现记录（原分支 v0.42.0，提交 `631c1f3`）：
    - 背景：接入 hub 的多个 co-shell 实例无法互相协作。需要公告板机制：一个 co-shell 发出协助请求，其他实例按职责匹配响应，条件不足时私信讨论，执行后返回结果。
    - 方案（用户确认）：① 通信通道：WS 原生双向通道为主 + MCP 作为可选外部接入面；② 下发感知：广播用巡检感知 + 定向任务用主动注入；③ 执行方式：当前会话执行；④ 安全边界：co-shell 增加公告板开关，默认关闭；⑤ 本次范围：最小可用闭环（发请求→认领→私信→执行→回结果）。
    - 原实施：`hub/gateway/`（board.go + AgentConn 下行通道 + Proxy 识别）+ co-shell（DynamicEventKind 扩展 + serve dispatch board_task + board_result 工具 + 公告板开关）
    - 原验证：go build+vet 全绿；hub 6 个公告板单测通过 [BUILD-912]
  - 本次落地（BUILD-967）：在 FEATURE-490-merge 分支合并完成，冲突仅 3 处且已解决（ROADMAP.md 保留新版本段并收拢原实现记录；main.go 保留 0.51.0/967；web/server.go 同时保留 FEATURE-507 分页字段与 FEATURE-490 board_task 字段，sendRaw 自动合并）。
  - 本次补齐规范缺口：① 6 个 board 工具新增 XML 模式用法示例（`KeyToolUsageBoard*` 中文/英文 + `toolUsageKeyMap` 映射）；② `BoardEnabled` 接入 `:set board-enabled on|off`、Web 设置面板、`:config` 参数（重置语义为 off）与中英文案；③ 新增 co-shell 侧单测 `agent/board_fix490_test.go`（工具注册开关、6 个工具线协议报文、三类 board 动态事件渲染、无 sender 时失败行为）。
  - 本次验证：`go build ./... && go vet ./...` 全绿；`go test ./agent/ -run TestBoard` 4/4 通过；`go test ./web/ ./i18n/` 通过；hub 模块 `hub/gateway` 6 个公告板单测通过。既有失败与本次无关：`agent` 的 TestAutoIntervention_* / TestStreamSupReply、`cmd` 的 TestWebWizardModelNameStep（合并前后基线一致）。
  - 合并 main 前同步（BUILD-969）：先把 main 的 FIX-509（v0.50.1）合入本分支，冲突 3 处已解决（`main.go` / `cmd/co-shell-hub/main.go` 统一为版本 0.51.0 + BUILD 969；`ROADMAP.md` 同时保留 v0.51.0 与 v0.50.1 两个版本段）。
  - 双实例联调：hub + 两个 co-shell 实例的公告板全链路已实测闭环（post → list → claim → dm → confirm → board_task → board_result → done），报告见 `tmp/fix490-e2e/LIAN-TIAO-REPORT.md`。
  - 遗留缺口（已记录，建议另立 FIX 任务）：① hub 的 `routeAgentMessage` 只处理 `board_result`，agent 上行的其余 5 个 `board_*` 报文被丢弃；② co-shell 侧未处理 board 响应（`board_list` / `board_claim` 等），「巡检感知」缺失一半。
  - 进度：已完成并合并到 main（tag v0.51.0）[BUILD-969]

---

## v0.50.1 — 已合并

> **版本**: v0.50.1

> **状态**: ✅ 已合并到 main（tag v0.50.1）
> **里程碑**: 修复历史分页加载中断（FIX-509）
> **说明**: 修复 Web UI 向上滚动只能加载一次（约 20 条）后无法继续加载更早历史的问题。根因是 `pushHistory` 的 `hasMore` 判断基于「本次加载的组数是否 > count」，而非「是否还有更早事件」；当单次 1000 条原始事件分组后 ≤ 20 组时（消息块较大时常见）误报无更多历史。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-509 | 0.50.1 | P1 | 修复历史分页加载中断：pushHistory 改为循环加载直到凑够 count 组或没有更早事件；hasMore 语义修正为「是否还有更早事件」；风险标签去重 + 收紧块复用匹配 |

> 当前 BUILD: 968
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-509 修复历史分页加载中断（已合并 v0.50.1）**
  - 背景：FEATURE-508 交付后，用户反馈向上滚动只能加载约 20 条，之后无法继续加载更早历史。
  - 根因：`web/session.go` 的 `pushHistory` 中，`hasMore` 基于 `len(order) > count` 判断。单次 `LoadEvents` 上限为 `count*maxEventsPerMessage = 1000` 条原始事件，而一个 TOOL 块可含 50+ 事件，因此 1000 条原始事件分组后常不足 20 组，导致 `hasMore=false`，前端 `loadOlderHistory` 因 `!historyHasMore` 直接 return。
  - 方案（用户确认）：后端循环加载，直到凑够 count 组或 store 报告没有更早事件为止；`hasMore` 语义修正为「是否还有更早事件」；`oldestSeq` 改为取最旧组的首个事件 seq（原实现取 `entries[0].Seq`，在循环加载下不再正确）。
  - 实施：`web/session.go`（pushHistory 重写为循环加载）+ `web/static/app.js`（风险标签去重 + 收紧块复用匹配）+ `web/fix509_history_test.go`（4 个回归测试）+ `main.go`/`cmd/co-shell-hub/main.go`（版本 0.50.1）+ ROADMAP.md
  - 测试：`go test ./web/ -run TestPushHistory` 4 个用例全部通过（覆盖：凑够 count 组、到达最旧一条、游标向后翻页不重叠、空流）
  - [BUILD-966] 用户反馈「回放与实时不一致（顺序+内容）」后重新实测：直接读隔离实例 bbolt 副本，发现事件流 seq 连续无缺口（2584 条、gaps=0），但同一 session 内 msg_index 出现 7 次回落、8 段拼接，且 seq=1 存的是最新事件（真正的最旧事件已丢失）——证明写入侧本身已损坏，与此前前端修复无关。
  - [BUILD-966] 根因（复现测试钉死）：`store/eventstream.go` 的 `AppendEvent` 注释写着「seek 到前缀之后、再回退一步」，代码却缺少回退。当存在排序在本会话之后的另一个会话（如新建会话 `sess-20260912…` 与旧会话 `sess-20260911…` 共存）时，`Seek(prefix+0xFF)` 返回的是**别的会话的 key**，前缀校验失败使 `nextSeq` 回落为 1，于是此后每条新事件都覆盖 `seq=1`。
  - [BUILD-966] 修复：`Seek` 后显式 `Prev()`（无后继时仍回退 `Last()`），确保取到本会话最高 seq。新增 `store/eventstream_fix509_test.go`（2 个回归测试，修复前第一个用例失败：`kept 1 events, want 3`）。
  - [BUILD-966] 待办：`loadHistoryPage` 按 `msg_index` 分组假设了 msg_index 会话内唯一，但 retry（`popTo` 截断）与 `reorganize_context` 折叠会使其回退/重置为 1（且 `ClearEventStream` 在生产代码中无调用方），跨世代分组会重排页内事件——待用户确认是否一并修复。
  - [BUILD-966] 验证：`go test ./store/` 全部通过。
  - 附带修复（FIX-508 回归）：① 风险标签插入无去重，叠加块复用导致标签累积（截图见 10 个）；② 块复用 fallback 过于宽松（`iterToolBlocks.find(b => !b._intentFilled && b.params)`），使后续无关工具调用（含 attempt_completion）误用前一个调用的块，标题与参数描述不同调用。已改为仅按 tool_name 精确命中。
  - 补充修复（BUILD 959，用户实测反馈）：① `web/server.go` 的 `HasMore`/`OldestSeq` 带 `omitempty`，零值（false/0）时字段被整个从 JSON 中删除，前端读到 `undefined` → `historyOldestSeq=0` → `loadOlderHistory` 因 `!historyOldestSeq` 直接 return，分页彻底停摆；已去掉 `omitempty` 并新增 `TestHistoryMessageAlwaysCarriesPagingFields` 回归测试。② `web/static/app.js` 的 `wasAtTop = prevTop < 5` 阈值过小，而 IntersectionObserver 的 `rootMargin` 为 120px，触发时 `prevTop≈120` 导致走 `else` 分支把视口向下推入内容高度（实测 scrollTop 0→583→9671）；阈值放宽到 200 以覆盖 rootMargin。
  - 补充修复 2（BUILD 960，用户实测反馈「到达滑动窗口边界后无法继续加载」）：`web/static/style.css` 的哨兵被改为 `position: sticky; top: 0`（BUILD 958 引入），导致哨兵在**任意滚动位置**都钉在滚动容器顶边（实测 scrollTop 0/500/2000/10000 时 relativeTop 恒为 0）。IntersectionObserver 只在**进入/离开**时触发，哨兵永不离开视口 → 首次触发后再无新事件 → 分页停摆。已回退为普通流内元素，并新增 `TestTopSentinelIsNotSticky` 回归测试。同时 `web/static/app.js` 新增 `scheduleChainLoad()`：prepend 后若用户仍在顶部，主动链式加载下一页，不再单纯依赖 observer 重新触发。
  - 新增配置项（BUILD 961，用户需求）：将历史分页「一次取 20 条」改为可配置参数 `page-buffer-size`（Go: `LLM.PageBufferSize` / JSON: `page_buffer_size`），范围 10-100，默认 20。接入点：`config/config.go`（字段 + 默认值）、`cmd/settings_web.go`（Web 设置面板）、`cmd/settings_display.go`（CLI `:set` 读写与校验）、`cmd/settings.go`（CLI 白名单 + 列表展示）、`i18n/{keys,zh,en}.go`（中英文描述）、`web/static/app.js`（`pageBufferSize` 变量 + `cacheStreamWindowSettings` 读取 + 两处 `history_get` 请求改用该值）。后端 `pushHistory` 的 `defaultHistoryMessages` 保留为兜底（前端未传 count 时使用）。
  - 补充修复 3（BUILD 962，用户实测反馈）：prepend 历史页时，① 每条消息的 token 用量行被追加到主消息区**最底端**堆叠（实测 8 个 `.ev.meta` 成了 `streamB` 的直接子节点，而非嵌套在各自 `.ev` 块内）；② 消息指示器（msgViz）的新线被加到**右端**，导致时间轴反向。根因：`insertAnchor` 只作用于 `makeBlock` 创建块的瞬间，而 token 行与指示器线是在**后续的 `token_iter` 事件**里创建的，仍无条件 `appendChild`。修复：新增 `prependingHistory` 标志（`renderHistory` 的 prepend 循环内置位），token 行改为 `streamB.insertBefore(line, insertAnchor)`，指示器线改为 `msgVizTrack.insertBefore(line, msgVizTrack.firstChild)`。
  - 合并（BUILD-968）：按用户确认合并到 main（快进合并），并打版本标签 `v0.50.1`。合并前复核：`go build ./... && go vet ./...` 全绿；`store` 包全部单测通过；`cmd` 的 `TestWebWizardModelNameStep` 为既有失败（与本次无关）。
  - 补充修复 4（BUILD 963，用户实测反馈「向上滚动会连续加载很多页」）：BUILD 962 引入的 `wasAtTop` 分支在 prepend 后强制 `scrollTop = 0`，使哨兵始终留在 IntersectionObserver 的 120px 边距内，`scheduleChainLoad()` 于是立即再次触发，一路加载到最旧事件。已改为**统一位置补偿**：prepend 后一律 `scrollTop = prevTop + delta`，把新内容插到当前视口**上方**、视口位置保持不变；哨兵随之移出视口，分页自然停止，直到用户再次向上滚动。同时删除已无用的 `scheduleChainLoad()`。
  - 补充修复 5（BUILD 964，用户实测反馈「历史内容缺少记录或顺序不对，LLM 与 TOOL 块连成一片」）：`web/session.go` 的分页循环依赖「单页内有序」而非「全局有序」。`LoadEvents` 每次返回的是**最新**窗口（seq < cursor），第二次调用返回的是**更旧**的窗口，但代码把后读到的页 append 到 `order` 之后，于是 `order` 变成「较新组..., 较旧组...」；同时 `groups[key]` 跨页累积会把跨页分组的事件前后颠倒；裁剪 `order[len-count:]` 后顺序彻底错乱。已把分页逻辑提取为 `loadHistoryPage()` 并**先按 seq 全局排序再分组**，保证输出严格按时间顺序、最后一个是最新组。测试侧：`collectHistoryPage` 原先复制了生产逻辑（所以从未捕获该 bug），已改为调用真实 `loadHistoryPage`；新增 `TestHistoryPageIsChronologicalAcrossWindows`（40 组 × 60 事件，强制跨窗口），已验证该用例在旧实现下失败（`group went backwards (6 after 39)`）、修复后通过。
  - 补充修复 6（BUILD 965，定位「修复后仍复现」的关键原因）：`web/server.go` 的 `GET /static/` 直接用了 `http.FileServer`，而 `embed.FS` 中的文件**没有修改时间**，导致响应既无 `Last-Modified` 也无 `ETag`（实测响应头仅有 `Content-Type`/`Content-Length`）。浏览器因此退化为**启发式缓存**、无法校验，重启服务后仍继续使用旧 build 的 `app.js` —— 这正是前几轮修复在用户侧「看起来没生效」的原因。已改为 `serveStatic()`：按资源内容哈希生成 `ETag` 并设置 `Cache-Control: no-cache`，匹配 `If-None-Match` 时返回 304。新增 `TestStaticAssetsCarryAnETag` 回归测试。
  - 进度：已合并到 main 并打版本标签 v0.50.1（合并前复核：`go build ./... && go vet ./...` 全绿，`store` 包单测通过）[BUILD-968]

---

## v0.50.0 — 开发中

> **版本**: v0.50.0

> **状态**: 🚧 开发中
> **里程碑**: 前端滑动窗口缓存渲染（FEATURE-508）
> **说明**: 在 v0.49.0（事件流持久化 + 分页回放 + 刷新回填）基础上，实现前端滑动窗口缓存渲染：顶部哨兵加载更早历史、底部超限回收旧块、滚动锚点保持、联动功能适配。双阈值（块数 + DOM 节点数）做成两个系统参数，可通过系统设置配置。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-508 | 0.50.0 | P2 | 前端滑动窗口缓存渲染（P2）：顶部哨兵加载 + 底部超限回收 + 滚动锚定 + 联动适配；双阈值做成系统参数 |

> 当前 BUILD: 955
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-508 前端滑动窗口缓存渲染（P2）**
  - 背景：P1 已解决“刷新后无历史”与“事件流持久化”，但前端块仍然**只增不减**（`makeBlock()` 永久 appendChild），长会话下 DOM 节点与内存无上限增长。
  - 方案（用户确认）：① 顶部哨兵加载（IntersectionObserver + `history_get` 的 `before` 游标）；② 底部超限回收（双阈值：块数 + DOM 节点数，从最旧端按 message index 组整组回收）；③ 滚动锚点保持（插入前记 scrollHeight，插入后补偿 scrollTop）；④ 联动适配（msgViz / blockNav / token 统计）。**双阈值做成两个系统参数，可通过系统设置配置。**
  - 实施：`web/static/app.js`（哨兵 + 回收 + 锚点）+ `web/static/style.css` + `config/`（两个新参数）+ `cmd/settings_*.go`（设置项）+ `i18n/`（中英文案）+ ROADMAP.md
  - 测试：见 use-case/FEATURE-508/
  - 进度：已完成并合并（运行时实测：哨兵加载 27→29 块、超限回收 32→9 块、锚点补偿 scrollTop 增量==scrollHeight 增量、联动功能正常）[BUILD-955]

---

## v0.49.0 — 开发中

> **版本**: v0.49.0

> **状态**: 🚧 开发中
> **里程碑**: 前端消息滑动窗口缓存渲染（FEATURE-507）
> **说明**: 解决“大模型返回内容不断增多导致浏览器内存/CPU 持续增长”的问题。P1 阶段：后端将已下发的事件流按会话**持久化**（bbolt，不驻留内存，内存中仅保留当前上下文），新增按游标分页读取历史事件的接口，前端刷新/重建会话时从后端读取最后 N 条（默认 20）消息并渲染为多个消息块、滚动到最后一条。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-507 | 0.49.0 | P1 | 前端消息滑动窗口缓存渲染（P1）：后端事件流持久化 + 分页回放接口 + 刷新回填最后 20 条并滚到底 |

> 当前 BUILD: 948
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-507 前端消息滑动窗口缓存渲染（P1：持久化事件流 + 分页回放 + 刷新回填）**
  - 背景：随着大模型返回内容不断增多，浏览器占用内存与 CPU 持续增长——前端消息块只追加不回收（`makeBlock()` → `streamB.appendChild`，app.js:883/909），静默模式仅隐藏显示、DOM 与 raw 文本照旧保留；且后端目前**无历史回放**（`handleWS` 建连仅 `sendState()`），刷新浏览器后消息区为空。
  - 方案（用户确认）：选 B —— 持久化“已下发的事件流”按游标分页回放；窗口按 message index 组；双阈值（块数 + DOM 节点数/文本字节数）。**关键约束：后端历史不得放内存，必须持久化；前端需要时直接从持久化读取；后端内存中仅有当前上下文内容。**
  - 实施：`store/`（新增事件流持久化读写）+ `web/session.go`（下发事件时同步落盘 + 分页回放接口）+ `web/static/app.js`（刷新回填最后 20 条并滚到底）+ `web/server.go`（消息类型）+ ROADMAP.md
  - 测试：见 use-case/FEATURE-507/
  - 进度：开发完成——`store/eventstream.go` 新增 `eventstream` bucket 与 `AppendEvent`/`LoadEvents`/`DeleteEventsAfter`/`ClearEventStream`（key = `{sessionID}\x00{seq:08d}`，value = StreamEvent JSON，单会话上限 5000 条自动裁剪最旧）；`store/dual.go` 补 4 个转发方法；`web/session.go` 在 `WebRenderer.Render` 落盘（失败仅告警不阻断）、新增 `pushHistory`（按 `meta.msg_index` 分组取最后 N 组）与 `popTo` 同步裁剪；`web/server.go` 新增 `history_get` 入站字段与 `history` 出站字段；`web/static/app.js` 新增 `renderHistory` 并在 `ws.onopen` 请求最后 20 条、渲染后滚到底。新增 `store/eventstream_test.go` 8 个单测全部通过（含分页游标、上限裁剪、重启后仍可回放）；`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；`go test ./store/` 全绿（`agent`/`cmd` 包 4 个失败已在干净 main 上复现，与本次修改无关）[BUILD-948]

---

## v0.48.0 — 开发中

> **版本**: v0.48.0

> **状态**: 🚧 开发中
> **里程碑**: 启动时自动创建系统内置文件夹（FEATURE-501）+ PLAN/RESEARCH 模式描述优化（FEATURE-502）+ RESULT MODE 节追加 --unload-mode 说明（FEATURE-503）+ RULES 节追加 .rules/ 定制说明（FEATURE-504） + SKILLS 节追加 skill 配置机制说明（FEATURE-505）+ 修复“从此处重新执行”（FIX-506）
> **说明**: 0.48.0 系列包含三项：① 内置能力可见性——co-shell 启动时自动在 workspace 根目录创建 12 个系统内置文件夹（.rules/skills/research/input/output/mode/bin/tmp/log/db/download/logos），让用户通过文件夹名字即可大致了解内置能力；② 工作模式描述优化——强化 PLAN MODE 挖掘需求/反复确认模糊点、RESEARCH MODE 结论须有高置信度证据支撑的行为纪律；③ RESULT MODE 节末尾追加 --unload-mode 配置说明，让 LLM 自己知道各模式策略可导出到 ./mode/ 下实时调整。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-501 | 0.48.0 | P1 | 启动时自动创建系统内置文件夹：.rules/skills/research/input/output/mode/bin/tmp/log/db/download/logos，体现内置能力 |
| FEATURE-502 | 0.48.0 | P1 | PLAN/RESEARCH 模式描述优化：PLAN MODE 强调挖掘需求、模糊处反复确认；RESEARCH MODE 强调结论须有高置信度证据支撑 |
| FEATURE-503 | 0.48.0 | P1 | RESULT MODE 节末尾追加 --unload-mode 配置说明：让 LLM 知道各模式策略可导出到 ./mode/ 下编辑实时调整 |
| FEATURE-504 | 0.48.0 | P1 | RULES 节末尾追加 .rules/ 定制说明：告知可通过向 .rules/ 下放规则文件定制规则/规范，文件名作为各节标题，子文件夹被列出（作为索引）但不再遍历 |
| FEATURE-505 | 0.48.0 | P1 | SKILLS 节末尾追加 skill 配置机制说明：告知可通过向 ./skills/ 或 ~/.co-shell/skills/ 下放 skill 目录定制 skill，同名工作空间级优先，可用 :skill 命令管理；SKILLS 段改为始终输出 |
| FIX-506 | 0.48.0 | P1 | 修复 Web UI“从此处重新执行”三个问题：① 回退数变成退至序号（msg_index 注入的是轮次计数器而非消息数组下标，导致上百条上下文只剩 5 条）；② 点击后整页刷新（location.reload）；③ 图标 ↻ 改为 ⏪ |

> 当前 BUILD: 945
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-501 启动时自动创建系统内置文件夹**
  - 背景：co-shell 的多个内置能力各自对应 workspace 下的一个系统文件夹（.rules 规则、skills 技能、research 调研、input 输入附件、output 输出产物、mode 工作模式、bin 自定义工具脚本、tmp 临时文件、log 日志、db 数据库、download 下载、logos 自定义 logo），但这些文件夹只在对应功能首次使用时才按需创建，新用户无法从目录结构直观发现 co-shell 有哪些内置能力。
  - 方案（用户确认）：co-shell 启动时统一自动创建 12 个系统内置文件夹（方案B），使用户通过文件夹名大致了解内置能力；已存在则不覆盖、不报错；创建失败仅告警不阻断启动。
  - 实施：main.go（定义内置文件夹清单 + 启动时创建）+ main.go/cmd/co-shell-hub/main.go（版本号 0.48.0 + build 计数）
  - 测试：见 use-case/FEATURE-501/
  - 进度：开发完成——main.go 新增 builtinDirs（12 个内置文件夹）与纯函数 ensureBuiltinDirs（MkdirAll 幂等创建，返回失败项不 panic），在 os.Chdir(ws.Root()) 后调用并以 log.Warn 记录失败（不阻断启动）；新增 main_test.go 5 个单元测试（首次创建/保留已有内容/补齐缺失/路径被文件占用不崩溃/幂等）全部通过；全新目录端到端启动验证 12 个文件夹全部创建；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-937]

- [x] **FEATURE-502 PLAN/RESEARCH 模式描述优化**
  - 背景：PLAN MODE 与 RESEARCH MODE 的系统提示词行为指导文案未突出各自的核心纪律：PLAN MODE 的价值在于挖掘用户真实需求、对模糊之处反复确认，而不是自行猜测后替用户做决定；RESEARCH MODE 的价值在于结论有据可查，所有结论必须有高置信度证据支撑，不能凭空想像。
  - 方案（用户确认）：仅优化系统提示词中的模式行为指导（i18n/zh_system.go、i18n/en_system.go 的 KeyWorkModePlan / KeyWorkModeResearch）；UI 设置界面的简短标签保持不变。本任务与 FEATURE-501 一并在 v0.48.0 发布（追加在 FEATURE-501 分支）。
  - 实施：i18n/zh_system.go + i18n/en_system.go + ROADMAP.md
  - 测试：见 use-case/FEATURE-502/
  - 进度：开发完成——i18n/zh_system.go 与 i18n/en_system.go 重写 KeyWorkModePlan（强调挖掘真实需求、模糊处必须反复用 ask_followup_question 确认、不凭猜测替用户决定）；重写 KeyWorkModeResearch（所有结论须有高置信度证据、严禁凭空想像、证据不足须如实说明不确定性，调研手段补充“浏览器与curl在互联网开展调研”）；新增 i18n/mode_desc_test.go 3 个测试（中/英要点断言、非空渲染）全部通过；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-938]

- [x] **FEATURE-503 RESULT MODE 节末尾追加 --unload-mode 配置说明**
  - 背景：系统提示词的 RESULT MODE 节（标题 ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE）列出了各工作模式的行为策略，但 LLM 自身不知道这些策略可以通过 `--unload-mode {mode}` 导出到 `./mode/` 下编辑并实时生效，因此无法主动告知用户如何调整模式策略。
  - 方案（用户确认）：在该节末尾追加一句括号说明——（以上内容可以通过 --unload-mode {mode}，将各模式的策略导出到 ./mode/ 下，可以通过编辑这些文件进行实时调整）。
  - 实施：i18n/keys.go（新增 KeySystemPromptResultModeNote）+ i18n/zh_system.go / en_system.go（中英双语说明文本）+ agent/system_prompt.go（buildResultModeSection 末尾追加说明）+ ROADMAP.md
  - 测试：见 use-case/FEATURE-503/
  - 进度：开发完成——i18n/keys.go 新增 KeySystemPromptResultModeNote；zh_system.go/en_system.go 填充中英双语括号说明；agent/system_prompt.go 的 buildResultModeSection 在模式描述循环后追加该说明；新增单测 TestBuildResultModeSection_TrailingNote（zh/en 两子用例，断言说明存在、位于模式描述之后、括号包裹）通过；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-939]

- [x] **FEATURE-504 RULES 节末尾追加 .rules/ 定制说明**
  - 背景：系统提示词的 RULES 节列出了核心规则，但 LLM 自身不知道用户可以通过向 `.rules/` 下放规则文件的方式定制规则/规范，也不知道文件名会被当作各节标题、子文件夹会被列出作为索引，因此无法主动告知用户如何定制规则。
  - 方案（用户确认）：在 RULES 节（i18n 的 KeySystemPromptRules 文本）末尾、`{CUSTOM_RULES}` 占位符之前，追加一句括号说明——（可以通过向 .rules/ 下放规则文件的方式，在以下位置定制规则/规范，文件名将被当作各节标题，子文件夹将被列出（作为索引），但不会再遍历子文件夹，需要时可自取）。
  - 实施：i18n/zh_system.go + i18n/en_system.go（中英双语括号说明）+ ROADMAP.md
  - 测试：见 use-case/FEATURE-504/
  - 进度：开发完成——i18n/zh_system.go 与 i18n/en_system.go 的 KeySystemPromptRules 文本末尾（{CUSTOM_RULES} 之前）各追加一句括号说明；新增 i18n/rules_note_test.go 单测 TestRulesDirCustomizationNote（zh/en 两子用例，断言说明存在、括号包裹、位于最后一条规则之后且位于 {CUSTOM_RULES} 之前）通过；端到端渲染验证说明位于 RULES 节末尾；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-940]

- [x] **FEATURE-505 SKILLS 节末尾追加 skill 配置机制说明**
  - 背景：系统提示词的 SKILLS 节列出了可用 skill 索引，但 LLM 自身不知道用户可以通过向 `./skills/`（工作空间级）或 `~/.co-shell/skills/`（全局级）下放 skill 目录的方式定制 skill，也不知道同名时工作空间级优先、可用 `:skill` 命令管理，因此无法主动告知用户如何定制 skill。此外 SKILLS 段在无 skill 时整段不输出，导致说明也随之消失。
  - 方案（用户确认）：① 在 SKILLS 段最末尾（动态索引列表之后）追加一句括号说明——（可以通过向 ./skills/（工作空间级）或 ~/.co-shell/skills/（全局级）下放 skill 目录的方式定制 skill，每个 skill 是一个包含 SKILL.md 的目录，同名时工作空间级优先，可用 :skill list/show/add/remove 命令管理）；② SKILLS 段改为始终输出（即使无 skill 也输出头部 + 说明）。
  - 实施：i18n/zh_system.go + i18n/en_system.go（中英双语括号说明）+ agent/system_prompt.go（SKILLS 段始终输出 + 索引插入到说明之前）+ ROADMAP.md
  - 测试：见 use-case/FEATURE-505/
  - 进度：开发完成——i18n/zh_system.go 与 i18n/en_system.go 的 KeySystemPromptSkills 文本末尾各追加一句括号说明；agent/system_prompt.go 的 Skills 分支改为始终输出（无 skill 时返回 header），并在有 skill 时将索引插入到末尾括号说明之前（兼容中文全角「（」与英文半角「(」）；新增 agent/skills_note_test.go 3 个单测（中英说明断言、无 skill 仍输出、索引位于说明之前）全部通过；端到端渲染验证中英双语在有无 skill 两种场景下均正确；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-941]

- [x] **FIX-506 修复 Web UI“从此处重新执行”回退语义错位 + 页面刷新 + 图标** ✅ 已完成 [BUILD-943]
  - 背景：Web UI 的“从此处重新执行”（retry-from，块标题栏 ↻ 按钮）存在三个问题：① 点击倒数第二个用户消息，上百条消息的上下文只剩 5 条（回退数变成了退至序号）；② 点击后整页刷新（无必要）；③ 图标 ↻ 应改为两个向左的三角（古典倒带）。
  - 根因：前端 `app.js` 的 ↻ 按钮发送 `session_pop` 时传 `box.dataset.msgIndex`，该值来自后端 `WebRenderer.Render` 注入的 `msg_index`——它是 `WebSession.msgIndex`，**每次用户输入 +1 的轮次计数器**（1、2、3…），并非 `a.messages` 数组下标；而后端 `cmd/session.go` 的 `popTo(n)` 按 `a.Messages()` 的**数组下标**截断（`a.SetHistory(aMsg[:n+1])`）。两者语义错位导致截断位置错误。
  - 方案（用户确认）：① 后端 `WebRenderer` 注入的 `msg_index` 改为真实消息数组下标（`len(a.Messages())-1`），使前端传值与 `popTo` 语义一致；轮次计数保留用于 token 统计显示；② 前端 `pop_result` 成功后不再 `location.reload()`，改为本地截断事件流（移除目标块及其后所有块）并刷新分支/文件树；③ 图标 ↻ → ⏪。
  - 实施：`web/session.go`（新增 `msgIndexForRetry()` 返回真实数组下标，`Render` 用它注入 `msg_index`；`msgIndex` 字段注释明确为轮次计数器）+ `web/static/app.js`（新增 `popTargetIndex` 变量与 `truncateStreamFrom()` 函数；`pop_result` 分支改为调用它；↻ 改 ⏪）+ `web/fix506_test.go`（3 个单测）[BUILD-943]
  - 测试：见 use-case/FIX-506/；单元测试 `web/fix506_test.go`（TestMsgIndexForRetryIsArrayIndex / TestRenderAttachesArrayIndex / TestMsgIndexForRetryEmptyHistory）全部通过；`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；`go test ./web/` 全绿（`cmd` 包 TestWebWizardModelNameStep 为既有网络依赖失败，与本次修改无关，已在干净 main 上复现）

- [x] **FIX-506（续修）修正 YOU 块回退参数口径 + 移除其他块回退按钮** ✅ 已完成 [BUILD-944]
  - 背景：首轮修复后用户实测仍失败——依次发送 hello 1/2/3，点击第 2 个 YOU 块的 ⏪，预期保留 hello 1 + hello 2，实际只剩 hello 1。
  - 根因：首轮把 `msg_index` 改为 `len(a.Messages())-1`（**渲染时刻**的最后一条消息下标），方向正确但仍有缺陷：① 前端 `renderUserEcho` 在用户回车时**立即**渲染 YOU 块，此时后端尚未把该用户消息追加进 `a.messages`（追加发生在 `agent/run_stream.go` 的 `RunStream` 内），它用的是 `lastMsgIndex`——**上一轮最后一个事件**的下标；② 同一轮内 user→assistant→tool 消息依次追加，每个事件渲染时 `len-1` 持续增长，导致 YOU/LLM/TOOL 块各携带不同下标。结果点 hello 2 实际回退到第 1 轮末尾，第 2 轮被整体截断。
  - 关键事实：一轮中**第一个**流事件到达时，`a.messages` 末尾正是该用户消息（assistant 消息要等流式结束才追加），因此**该轮首个事件的 `msg_index` 就是该用户消息的真实下标**。
  - 方案（用户确认）：① 语义 = 点击 YOU 块 ⏪ 保留到该消息为止（含），丢弃其后所有内容；② **移除其他所有类型块（LLM/THINK/TOOL/REPL/SYSTEM）的回退按钮**，只保留 YOU 块；③ 重点保证前后端参数口径一致。
  - 实施：`web/static/app.js`（新增 `pendingUserBlock` 变量；`renderUserEcho` 创建 YOU 块时不再用陈旧的 `lastMsgIndex`，改为标记 `data-msg-index-pending` 待定；`renderEvent` 在该轮首个带 `msg_index` 的事件到达时回填 YOU 块的真实下标；`addBlockActions` 仅当 `cls === "user-msg"` 时才添加 ⏪；`truncateStreamFrom` 清理 `pendingUserBlock`）+ `web/session.go`（补充 `msgIndexForRetry` 语义注释）+ `web/fix506_retry_test.go`（2 个单测）[BUILD-944]
  - 测试：见 use-case/FIX-506/FIX-506-UC-0002.md；单元测试 `web/fix506_retry_test.go`（TestFirstEventOfTurnCarriesUserMessageIndex / TestPopToKeepsUpToUserMessage）与既有 `web/fix506_test.go` 3 个单测全部通过；`go build ./... && go vet ./...` 全绿；`node --check web/static/app.js` 通过；`go test ./web/` 全绿；co-shell/co-shell-hub 编译到 ~/bin/

- [x] **FIX-506（二次续修）修复 YOU 块 ⏪ 回退按钮点击无反应** ✅ 已完成 [BUILD-945]
  - 背景：二次续修后用户实测仍失败——点击 YOU 块的 ⏪ 按钮完全无反应，前端控制台也无任何报错。
  - 根因：`data-msg-index` 属性被写到了错误的 DOM 元素上。`makeBlock()` 把 `data-msg-index` 设置在 `.ev`（box）上并返回 `.ev-body`（body），而 `renderUserEcho` 把返回值 body 赋给了 `pendingUserBlock`，于是 `renderEvent` 的回填写到了 body 上；但 ⏪ 按钮读的是 `box.dataset.msgIndex`、`truncateStreamFrom` 查的是 `.ev[data-msg-index]`——两者都拿不到值。因果链：box 上始终无 `data-msg-index` → 点击发送 `value:""` → 后端 `Atoi("")` 失败 → 返回 `pop_result OK:false` → 前端 `if (msg.ok)` 不成立 → 静默无反应。
  - 方案（用户确认）：① 让 `pendingUserBlock` 指向 box（`.ev`），使回填与 `makeBlock`/⏪按钮/`truncateStreamFrom` 三处口径一致；② 顺带修复静默失败——`pop_result` 失败时在消息流中给出可见提示，且按钮在索引缺失时直接提示而不发无效请求。
  - 实施：`web/static/app.js`（`renderUserEcho` 改用 `body.parentElement` 作为 `pendingUserBlock`；新增 `showRetryFromError()` 提示函数；`pop_result` 失败分支调用它；⏪ 按钮在 `popTargetIndex` 为空时提前提示；i18n 中英新增 `retryFromFailed` 文案）+ `main.go` / `cmd/co-shell-hub/main.go`（版本 0.48.0→0.48.1，build 944→945）[BUILD-945]
  - 测试：单元测试 `web/fix506_retry_test.go` 与 `web/fix506_test.go` 共 5 个单测全部通过；`go build ./... && go vet ./...` 全绿；co-shell/co-shell-hub 编译到 ~/bin/（`co-shell v0.48.1 [BUILD-945]`）

- [x] **FIX-506（三次续修）回退后自动执行 :continue** ✅ 已完成 [BUILD-946]
  - 背景：二次续修后 ⏪ 按钮已能正常回退，但回退后停在那里等用户手动输入 `:continue`，用户期望点击后直接自动继续运行。
  - 方案（用户确认）：点击 YOU 块 ⏪ → 截断消息流（保留该用户消息，丢弃其后内容）→ **自动触发 `:continue`** 继续运行，无需确认。
  - 实施：`web/static/app.js`（`pop_result` 成功分支在 `truncateStreamFrom(popTargetIndex)` 之后追加 `wsSend({ type: "input", text: "" })`——空输入即 `:continue` 的等价通道，Agent 追加 continue 提示词后继续运行）+ `main.go` / `cmd/co-shell-hub/main.go`（build 945→946）[BUILD-946]
  - 测试：`node --check web/static/app.js` 通过；`go build ./... && go vet ./...` 全绿；co-shell/co-shell-hub 编译到 ~/bin/（`co-shell v0.48.1 [BUILD-946]`）

- [x] **FIX-506（四次续修）修正自动 continue 通道：空输入被主循环丢弃** ✅ 已完成 [BUILD-947]
  - 背景：三次续修后用户实测——回退位置准确了，但**并未自动继续执行**。
  - 根因：前端发的是空输入 `wsSend({type:"input",text:""})`，而 REPL 主循环对空行直接跳过（`repl/repl.go`: `if input == "" { continue }`），所以什么都没发生。`:continue` 之所以有效，是因为它走 `handleBuiltin` 分支（`repl.go` case ":continue"）**直接调 `handleAgentInput("")`，绕过了主循环的空输入过滤**。
  - 方案：前端改发字面量 `:continue` 内置命令（`wsSend({type:"input",text:":continue"})`），走既有成熟路径；`resumeReader()` 对 Web 会话为 no-op（仅对 tuiSession 生效），安全。
  - 实施：`web/static/app.js`（`pop_result` 成功分支改发 `:continue`）+ `main.go` / `cmd/co-shell-hub/main.go`（build 946→947）[BUILD-947]
  - 测试：`node --check web/static/app.js` 通过；`go build ./... && go vet ./...` 全绿；co-shell/co-shell-hub 编译到 ~/bin/（`co-shell v0.48.1 [BUILD-947]`）

---

## v0.47.0 — 已发布

> **状态**: 🚧 开发中
> **里程碑**: Web UI 会话标题锁定优化（FEATURE-500）
> **说明**: 0.47.0 系列为 Web UI 会话标题管理优化版本：用户手动修改会话标题后自动加 `$` 前缀表示锁定，任务完成时不再被 LLM 自动生成的标题覆盖；清空标题则恢复默认占位标题并解除锁定，下次任务完成时重新自动生成。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-500 | 0.47.0 | P1 | Web UI 会话标题锁定优化：手动改标题自动加 `$` 前缀锁定；清空标题恢复默认占位标题解除锁定；`$` 开头标题任务完成时不再自动更新 |

> 当前 BUILD: 935
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-500 Web UI 会话标题锁定优化**
  - 背景：Web UI 会话标题当前在任务完成时会被 LLM 自动生成的标题覆盖，用户手动修改的标题无法保留。需要引入标题锁定机制：用户手动修改标题后自动加 `$` 前缀表示锁定，任务完成时不再覆盖；清空标题则恢复默认占位标题并解除锁定。
  - 方案（用户确认）：① 用户修改标题（非空、有变化）生效时，后端保存为 `$` + 标题（锁定）；② 用户清空标题生效时，后端恢复默认占位标题（如"新会话N"，与新建会话逻辑相同）并解除锁定，下次任务完成时 LLM 自动生成新标题；③ 会话标题以 `$` 开头时，任务完成（attempt_completion）只更新 keywords，保留原标题。
  - 实施：web/session.go（renameSession 加 `$` 前缀/清空恢复默认标题）+ agent/tools.go（attemptCompletionTool 检测 `$` 前缀跳过标题更新）+ web/static/app.js（前端清空标题时发送 session_rename 空值）+ main.go + cmd/co-shell-hub/main.go（版本号 0.47.0 + build 计数）
  - 测试：见 use-case/FEATURE-500/
  - 进度：开发完成——web/session.go renameSession 非空标题加 `$` 前缀锁定（去重已有 `$`）、空标题恢复默认占位标题解锁（提取 nextDefaultSessionTitle 复用 newSession 编号逻辑）；agent/tools.go attemptCompletionTool 检测当前会话标题 `$` 前缀时保留原标题只更新 keywords；web/static/app.js commitTitle 清空标题时发送空值触发解锁。新增 web/session_rename_test.go 4 个单元测试全部通过；go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-936]

---

## v0.46.0 — 开发中

> **版本**: v0.46.0

> **状态**: 🚧 开发中
> **里程碑**: Hub Agents 列表优化——点击外部收起 + 三态状态指示灯（FEATURE-499）
> **说明**: 0.46.0 系列为 hub 界面优化版本：① 恢复 Agents 列表展开后点击列表之外自动收起；② Agents 列表每个 Agent 前的指示灯改为三态实时监测——关闭灰色、已打开空闲绿色、正在执行任务红色呼吸灯。为支持第三态，需让 hub 能感知每个已连接 agent 是否正在执行任务（新增 busy 状态，hub 向 agent 查询）。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-499 | 0.46.0 | P1 | Hub Agents 列表优化：点击外部自动收起 + 三态指示灯（关闭灰/空闲绿/执行中红呼吸灯），后端新增 agent busy 状态供 hub 查询 |

> 当前 BUILD: 933
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-499 Hub Agents 列表优化：点击外部收起 + 三态状态指示灯**
  - 背景：hub 界面（hub/gateway/webui_static.go 内嵌 HTML）的 Agents 列表存在两处体验问题：① FEATURE-492 移除了点击列表之外自动收起（scrim 点击仅 stopPropagation），用户希望恢复；② 列表每个 Agent 前的指示灯当前仅两态（on=绿 running/connected、off=红），用户希望改为三态实时监测：关闭=灰、已打开空闲=绿、正在执行任务=红呼吸灯。第三态需要 hub 感知 agent 是否正在执行任务，当前 /api/agents 仅返回 running/connected，无 busy 信号。
  - 方案（用户确认）：① 前端恢复点击 scrim/列表外自动收起（保留 pin 钉住功能）；② 后端新增 agent busy 状态——co-shell agent 暴露是否正在执行任务状态，hub 向每个已连接 agent 查询并在 /api/agents 返回 busy 字段；③ 前端指示灯三态：关闭灰、空闲绿、执行中红呼吸灯（CSS 动画）。
  - 实施：hub/gateway/webui_static.go + hub/gateway/webui.go + hub/gateway/agent.go/proxy.go + co-shell agent 端（web/ 或 agent/）+ main.go + cmd/co-shell-hub/main.go（版本号 0.46.0 + build 计数）
  - 测试：见 use-case/FEATURE-499/
  - 进度：完成——前端恢复点击 scrim/列表外自动收起（保留 pin 钉住）；指示灯三态 CSS（灰/绿/红呼吸灯 stBreath 动画）+ renderList 按 running/connected/busy 三态渲染；后端新增 agent busy 状态（agent/loop.go busyCount+SetBusy/IsBusy，RunStream 入口/出口设置 busy；web/server.go /api/status 端点+SetBusyProvider；web/session.go 注册 busy provider；hub/gateway/detect.go agentBusy helper；hub/gateway/webui.go agentView.Busy + handleListAgents 查询各 agent /api/status）。go build+vet 全绿，co-shell/co-shell-hub 编译到 ~/bin/，合并 main 打标签 v0.46.0 [BUILD-935]

---

## v0.45.0 — 开发中

> **版本**: v0.45.0

> **状态**: 🚧 开发中
> **里程碑**: MCP server 支持 SSE URL 连接（FEATURE-498）
> **说明**: 0.45.0 系列为 MCP server 配置界面增加通过 URL（SSE 传输）连接远程 MCP server 的能力。当前 MCP server 仅支持 stdio（本地命令）方式，本次新增 SSE URL 连接：配置结构增加 URL 字段，连接层根据 URL 选择 SSE 客户端，Web UI 配置界面增加 URL 输入框。暂不支持鉴权。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-498 | 0.45.0 | P1 | MCP server 支持 SSE URL 连接：MCPServerConfig 增加 URL 字段 + mcp/client.go AddServer 支持 SSE 客户端 + Web UI 配置界面增加 URL 输入框 + WebSocket 消息透传 URL |

> 当前 BUILD: 923
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-498 MCP server 支持 SSE URL 连接**
  - 背景：co-shell 的 MCP server 配置界面（Web UI 系统设置 MCP Server 区块 + REPL :mcp）目前只支持 stdio（本地命令）方式连接，无法通过 URL 连接远程 MCP server。底层依赖库 mark3labs/mcp-go v0.8.3 已内置 SSE 客户端（client.NewSSEMCPClient），具备 URL 连接能力但未被使用。
  - 方案（用户确认）：① 先支持 SSE 传输，暂不考虑鉴权；② config.MCPServerConfig 增加 URL 字段（非空时走 SSE 连接，为空走 stdio）；③ mcp/client.go AddServer 根据 URL 选择 NewSSEMCPClient 或 NewStdioMCPClient；④ cmd/mcp.go WebMCPServer 透传 URL；⑤ Web UI 配置界面（web/static/app.js）表单增加 URL 输入框，卡片展示 URL；⑥ web/session.go/web/server.go WebSocket 消息透传 URL；⑦ i18n 文案。
  - 实施：config/config.go + mcp/client.go + cmd/mcp.go + web/static/app.js + web/session.go + web/server.go + i18n/ + main.go + cmd/co-shell-hub/main.go（版本号 0.45.0 + build 计数）
  - 测试：见 use-case/FEATURE-498/
  - 进度：核心实现完成——config.MCPServerConfig 增加 URL 字段（omitempty）；mcp/client.go AddServer 增加 url 参数（非空走 NewSSEMCPClient，为空走 NewStdioMCPClient）；cmd/mcp.go addServerWithURL 支持 url + WebMCPServer/MCPServersJSON/AddServerJSON/UpdateServerJSON 透传 url；web/server.go clientMessage 增加 URL 字段；web/session.go handleMCPAdd/handleMCPUpdate 透传 url；web/static/app.js 表单增加 URL 输入框（命令/URL 至少填一）+ 卡片展示 URL + toggle 传 url；main.go 启动连接传 serverCfg.URL。go build+vet 全绿，node --check app.js 通过，co-shell/co-shell-hub 编译到 ~/bin/ [BUILD-925]

---

## v0.44.1 — 开发中

> **版本**: v0.44.1

> **状态**: 🚧 开发中
> **里程碑**: Windows 下 co-shell 可执行文件 .exe 后缀处理（FIX-497）
> **说明**: 0.44.1 系列为修复版本：① hub 在 Windows 上查找 co-shell 可执行文件时未考虑 .exe 后缀；② hub 检测 co-shell 可执行文件时仅精确匹配文件名，无法识别带版本号的文件（如 co-shell-0.44.0.darwin.arm64）。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-497 | 0.44.1 | P1 | Windows 下 co-shell 可执行文件 .exe 后缀未处理 + hub 检测带版本号文件名：defaultCoShellPath / ResolveCoShellPath 处理 .exe；DetectCoShells 扫描所有 co-shell 开头可执行文件并验证 --version |

> 当前 BUILD: 923
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-497 Windows 下 co-shell 可执行文件 .exe 后缀未处理 + 带版本号检测**
  - 背景：hub 在 Windows 上执行时，系统找不到 PATH 路径下以及当前文件夹下的 co-shell.exe。原因是查找 co-shell 可执行文件时未考虑 Windows 上可执行文件带 .exe 后缀；且 hub 检测 co-shell 可执行文件时仅精确匹配文件名，无法识别带版本号的文件。
  - 方案：① cmd/co-shell-hub/main.go 的 defaultCoShellPath() 在 Windows 上返回 {dir}/co-shell.exe；② bridge/config.go 的 ResolveCoShellPath() 在 Windows 上检查 {dir}/co-shell.exe 并在 PATH 中查找 co-shell.exe；③ hub/hub.go 的 DefaultConfig() 默认 CoShellPath 在 Windows 上为 co-shell.exe；④ hub/gateway/detect.go DetectCoShells 扫描当前目录/PATH 中所有以 co-shell 开头的可执行文件（含版本号），引用前先执行 --version 确认是 co-shell 才放入下拉列表。
  - 实施：cmd/co-shell-hub/main.go + bridge/config.go + hub/hub.go + hub/gateway/detect.go + hub/gateway/detect_test.go（按 runtime.GOOS 判断 .exe 后缀 + 前缀扫描 + --version 验证）
  - 测试：见 use-case/FIX-497/
  - 进度：完成——defaultCoShellPath/ResolveCoShellPath/DefaultConfig 处理 .exe 后缀；DetectCoShells 扫描所有 co-shell 开头可执行文件并验证 --version；新增 coShellCandidates + 单元测试。go build+vet 全绿，GOOS=windows 交叉编译通过，hub 模块测试通过 [BUILD-923]

---

## v0.44.0 — 开发中

> **版本**: v0.44.0

> **状态**: 🚧 开发中
> **里程碑**: 模型联通性检测（FEATURE-496）
> **说明**: 0.44.0 系列为模型联通性检测：通过调用各模型 endpoint 的 /models 接口检查目标模型是否可用。用于 ① 用户提交指令前（可配置 on_submit/on_send/off，默认 on_submit）若主模型不可用则终止任务并报告；② 前端模型列表（状态条主/视觉模型列表、模型管理列表）将不可用模型字体变灰显示。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-496 | 0.44.0 | P1 | 模型联通性检测：model_connectivity_check 配置（off/on_submit/on_send，默认 on_submit）+ 后端检测函数（调用 /models 检查目标模型）+ agent 提交前检查终止任务 + 前端模型列表不可用模型字体变灰 |
| FIX-497 | 0.44.0 | P1 | Windows 下 co-shell 可执行文件 .exe 后缀未处理：defaultCoShellPath / ResolveCoShellPath 硬编码无后缀，Windows 上无法找到 co-shell.exe |

> 当前 BUILD: 918
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-496 模型联通性检测**
  - 背景：当某个已配置模型的 endpoint 不可达、API Key 失效或模型名不在服务商 /models 列表中时，用户提交指令或 agent 发送消息会失败。希望在发送前检测模型联通性，避免无效请求；并在前端模型列表中直观标识不可用模型。
  - 方案（用户确认）：① 新增配置 model_connectivity_check（off=关闭 / on_submit=仅用户提交指令时检测 / on_send=每次 agent 发消息前检测），默认 on_submit；② 后端新增联通性检测函数：对某 ModelConfig 调用其 endpoint 的 /models（复用 llm.Client.ListModels），若返回列表含该模型 model 名则可用，接口失败或列表不含则不可用；③ 检测时机：用户提交指令时（on_submit/on_send）或 agent 每次发消息前（on_send），若当前主模型不可用则终止任务并报告用户；④ 前端：后端推送 models 消息时为每个模型附带 available 字段，状态条主/视觉模型列表与模型管理列表据此将不可用模型字体变灰。
  - 实施：config/config.go（model_connectivity_check）+ llm/（联通性检测函数）+ agent/（提交/发送前检查接入）+ cmd/model_web.go（WebModel 增加 available）+ web/session.go（models 消息附带可用性）+ web/static/app.js + style.css（不可用模型变灰）+ i18n/（文案）+ main.go + cmd/co-shell-hub/main.go（版本号 0.44.0 + build 计数）
  - 测试：见 use-case/FEATURE-496/
  - 进度：核心实现完成——config 新增 model_connectivity_check（默认 on_submit）+ llm/connectivity.go CheckModelAvailable（调用 /models 检查目标模型）+ agent/connectivity.go checkModelConnectivity + run_stream.go 接入（on_submit 主循环前检测、on_send 每次迭代检测，不可用则终止任务并报告）+ cmd/model_web.go WebModel 增加 available（ModelWebJSON 实时检测填充）+ 前端 app.js/style.css 不可用模型字体变灰 + i18n zh/en 文案 + 版本号 0.44.0。go build+vet 全绿，node --check app.js 通过 [BUILD-919]

---

- [ ] **FIX-497 Windows 下 co-shell 可执行文件 .exe 后缀未处理**
  - 背景：hub 在 Windows 上执行时，系统找不到 PATH 路径下以及当前文件夹下的 co-shell.exe。原因是查找 co-shell 可执行文件时未考虑 Windows 上可执行文件带 .exe 后缀。
  - 方案：① cmd/co-shell-hub/main.go 的 defaultCoShellPath() 在 Windows 上返回 {dir}/co-shell.exe；② bridge/config.go 的 ResolveCoShellPath() 在 Windows 上检查 {dir}/co-shell.exe 并在 PATH 中查找 co-shell.exe；③ hub/hub.go 的 DefaultConfig() 默认 CoShellPath 在 Windows 上为 co-shell.exe（裸名，exec.Command 会自动补后缀，但显式指定更稳妥）。
  - 实施：cmd/co-shell-hub/main.go + bridge/config.go + hub/hub.go（按 runtime.GOOS 判断 .exe 后缀）
  - 测试：见 use-case/FIX-497/
  - 进度：完成——cmd/co-shell-hub/main.go defaultCoShellPath 在 Windows 返回 co-shell.exe；bridge/config.go 新增 coShellBinName() 并在同目录/PATH 查找 .exe；hub/hub.go DefaultConfig 在 Windows 默认 co-shell.exe。go build+vet 全绿，GOOS=windows 交叉编译通过 [BUILD-921]
  - 进度（增强）：hub/gateway/detect.go DetectCoShells 改为扫描当前目录/PATH 中所有以 co-shell 开头的可执行文件（含版本号如 co-shell-0.44.0.darwin.arm64），引用前先执行 --version 确认是 co-shell 才放入下拉列表；新增 coShellCandidates 辅助函数 + 单元测试 detect_test.go。go build+vet 全绿，GOOS=windows 交叉编译通过，hub 模块测试通过 [BUILD-922]

---

## v0.43.0 — 开发中

> **版本**: v0.43.0

> **状态**: 🚧 开发中
> **里程碑**: 工具结果上下文长度限制（FEATURE-491）
> **说明**: 0.43.0 系列为工具结果进入上下文增加通用长度限制：当工具返回内容超过 tool_result_max_size（默认 64KB）时，只返回限定值以内的头部内容，追加说明（文件位置/大小/行数），并将完整内容保存为文本文件供 LLM 评估是否补充读取。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-491 | 0.43.0 | P1 | 工具结果上下文长度限制：tool_result_max_size 配置 + 通用截断 + 保存完整内容到 tmp/tool-result/ + 告知 LLM 文件位置/大小/行数 |
| FEATURE-492 | 0.43.0 | P1 | hub 界面 2 项小优化：左上角 logo/版本信息框 3 秒后渐变透明（hover 恢复）+ 实例列表收起图标改图钉（可钉住不收起） |
| FEATURE-493 | 0.43.0 | P1 | Web UI 2 项小改进：状态栏数字单位（%/t/s/s）亮色显示（深色纯白/亮色纯黑）+ 会话标题栏去加粗 |
| FEATURE-494 | 0.43.0 | P1 | 文件查看器打开文件后定位到该文件最近一次提交的第一个修改位置（滚动到可视区上 1/3 处） |

> 当前 BUILD: 918
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-491 工具结果上下文长度限制**
  - 背景：当某个工具方法返回的文本内容太大时，可能导致上下文超限。这是无法避免的问题，需要兜底解决。
  - 方案（用户确认）：① 新增配置 tool_result_max_size（默认 64KB，0=不限制）；② 工具结果进入上下文前统一拦截，超过限制只返回限定值以内的头部内容；③ 追加说明（完整内容保存的文件位置/总大小/总行数/截断字节数）；④ 完整内容保存为文本文件到 tmp/tool-result/，供 LLM 评估是否补充读取；⑤ 版本 v0.43.0（minor 递增）。
  - 实施：config/config.go（tool_result_max_size）+ agent/（通用截断函数 + run.go/run_stream.go 接入）+ i18n/（截断说明文案）+ main.go（版本号 0.43.0 + build 计数）
  - 测试：见 use-case/FEATURE-491/
  - 进度：核心实现完成——config 新增 tool_result_max_size（默认 64KB）+ agent/tool_result_limit.go 通用截断函数（截断头部 + 保存完整内容到 tmp/tool-result/ + 追加说明含文件路径/大小/行数）+ run.go/run_stream.go 两条路径接入（含视觉识别回填）+ i18n zh/en 文案。go build+vet 全绿，10 个单元测试通过 [BUILD-913]

- [ ] **FEATURE-492 hub 界面 2 项小优化**
  - 背景：co-shell-hub Web UI 左上角 logo/版本信息框常驻遮挡 co-shell 界面；实例列表抽屉右上角收起图标交互不够直观。
  - 方案（用户确认）：① 左上角 co-shell-hub logo/版本信息框（#hubBadge）：页面初始化后先显示 3 秒，然后渐变为完全透明（但仍盖住下方内容、可接收点击），鼠标划过恢复不透明、移走变透明；② co-shell 实例列表从左侧弹出框（#agentPanel）右上角收起图标（#panelClose）改为图钉图标，用户可钉住该区域不自动收起。
  - 实施：hub/gateway/webui_static.go（内嵌 HTML/CSS/JS：badge 自动透明 + hover 恢复；图钉图标 + 钉住状态）+ cmd/co-shell-hub/main.go（hub 版本号/build 计数）
  - 测试：见 use-case/FEATURE-492/
  - 进度：核心实现完成——#hubBadge 初始化显示 3 秒后渐变透明（opacity 0，仍盖住下方、可点击），hover 恢复不透明、移走变透明；#panelClose 改为图钉图标（📌），点击钉住后 mouseleave 不自动收起、再次点击取消钉住恢复自动收起。go build+vet 全绿，浏览器验证 10 个用例全部通过 [BUILD-879]

- [ ] **FEATURE-493 Web UI 2 项小改进（状态栏单位亮色 + 标题去加粗）**
  - 背景：① 状态栏数字信息单位（%、t/s、s）颜色偏暗，希望显示为"亮"色（深色主题纯白、亮色主题纯黑）更醒目；② 会话标题栏字体加粗，希望去掉加粗更简洁。
  - 方案（用户确认）：① 状态栏单位字符（%、t/s、s）包进 `<span class="sb-unit">`，CSS 用 `var(--fg)`（深色=纯白 #d5dbe7、亮色=纯黑 #1d2433）；② 会话标题栏 `.stream-title` 与窄屏循环标题 `.stc-face` 的 `font-weight: 600` 改为 `400`。仅改底部状态栏单位，不含消息流 token 行。
  - 实施：web/static/app.js（updateStatus 中单位包 span）+ web/static/style.css（.sb-unit 亮色 + 标题去加粗）
  - 测试：见 use-case/FEATURE-493/
  - 进度：核心实现完成——状态栏单位（%/t/s/s）包进 .sb-unit span 用 var(--fg) 亮色显示（深色纯白/亮色纯黑），会话标题栏 .stream-title/.stc-face 去加粗（600→400）。go build+vet 全绿，web 包测试通过，浏览器验证标题去加粗生效 [BUILD-916]

- [ ] **FEATURE-494 文件查看器定位最近提交首个修改位置**
  - 背景：打开一个文本文件后，希望直接定位到该文件最近一次被修改的提交中的第一个修改位置，方便快速查看最近改动。
  - 方案（用户确认）：① 后端新增接口（如 /api/gitfirstchange）：`git log -1 -- <file>` 取该文件最近一次被修改的提交 C，再 `git diff C^ C -- <file>` 解析第一个 hunk 的起始行（新侧行号）作为首个修改位置；② 前端 openFilePreview 打开文件后调用该接口，拿到首个修改行号后滚动文件查看器（#fvBody）使该行位于可视区上 1/3 处（scrollTop = 目标行 offsetTop - 可视区高度/3）；③ 无 git 仓库/文件未跟踪/无父提交时返回空，前端不滚动。
  - 实施：web/server.go（新增 handleGitFirstChange + 路由）+ web/static/app.js（openFilePreview 后调用 + 滚动定位函数）
  - 测试：见 use-case/FEATURE-494/
  - 进度：核心实现完成——后端 /api/gitfirstchange 接口（git log -1 取最近提交 C，git diff C^ C 解析首个 hunk 起始行）+ 前端 openFilePreview 打开文件后调用并滚动到可视区上 1/3。修复 git rev-parse 缺 cmd.Dir 导致根提交回退误判的 bug（现正确返回首个修改行）。go build+vet 全绿，web 包 TestGitFirstChange 系列测试通过（修改第 3 行返回 line=3），node --check app.js 通过 [BUILD-917]

- [ ] **FEATURE-495 提交指令后焦点自动移出**
  - 背景：用户在主消息框提交指令后，焦点仍停留在输入框，光标闪烁干扰查看输出，且方向键/空格等按键会误入输入框。希望提交后焦点自动移出；因 document 级 keydown 监听已能在用户敲击普通字符键时自动把焦点带回输入框，故不影响后续录入。
  - 方案（用户确认）：在 sendInput() 中，当用户真正提交了内容（发送新指令 / 发送动态消息 / 上传附件发送 / 回答交互补充）后调用 input.blur() 让焦点移出输入框。
  - 实施：web/static/app.js（sendInput 各提交分支末尾 input.blur()）
  - 测试：见 use-case/FEATURE-495/
  - 进度：核心实现完成——sendInput 各提交分支（发送新指令 / 动态消息 / 上传附件 uploadAndSend / 回答交互补充）末尾调用 input.blur() 让焦点移出输入框；因 document 级 keydown 监听已能在敲击普通字符键时自动回焦，不影响后续录入。go build+vet 全绿，node --check app.js 通过 [BUILD-918]

---

## v0.41.0 — 开发中

> **版本**: v0.41.0

> **状态**: 🚧 开发中
> **里程碑**: 会话标题自动生成优化（FEATURE-488）
> **说明**: 0.41.0 系列承接 v0.40.0 发布后的新功能。FEATURE-488 优化会话标题自动生成：在 environment_details 注入当前会话标题作为 LLM 记忆锚点，优化 attempt_completion 的 session_title 提示词引导 LLM 结合当前标题做会话级概括（用关键字/排比高度浓缩历史主题、尽量不丢失历史上做过的不同的事），并将 session_title 长度限制从 30 放宽到 60 字符。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-488 | 0.41.0 | P1 | 会话标题自动生成优化：environment_details 注入会话标题 + session_title 提示词优化 + 长度放宽到 60 |
| FIX-489 | 0.41.0 | P1 | msgviz 消息可视化控件 hover 实线改进：默认虚线、hover 实线、移开恢复虚线 |

> 当前 BUILD: 908
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-488 会话标题自动生成优化**
  - 背景：co-shell 会话标题由 LLM 在 attempt_completion 时自动生成，但只关注最近一次任务目标，对整个会话的概括性不足（后一次任务会整体覆盖前一次标题）。
  - 方案（用户确认）：① 在 environment_details 注入当前会话标题（含默认标题处理），作为 LLM 记忆锚点；② 优化 attempt_completion 的 session_title 提示词，引导 LLM 结合当前标题做会话级概括，用关键字/排比高度浓缩历史主题、尽量不丢失历史上做过的不同的事；③ 将 session_title 长度限制从 30 放宽到 60 字符；④ Agent 内存缓存当前标题避免每次构建 environment_details 查库。
  - 实施：agent/envelope.go（注入会话标题）+ agent/agent.go（内存缓存当前标题）+ agent/tools.go（长度限制 60）+ i18n/zh_system.go + i18n/en_system.go（提示词优化）+ main.go（版本号 0.41.0 + build 计数）
  - 测试：见 use-case/FEATURE-488/
  - 进度：待开发

- [x] **FIX-489 msgviz 消息可视化控件 hover 实线改进** ✅ 已完成 [BUILD-911]
  - 背景：msgviz 消息可视化控件线段当前为 1px 虚线（repeating-linear-gradient），hover 仅加宽到 3px 但仍为虚线，视觉区分度不足。
  - 方案（用户确认）：① app.js 将线段颜色改为 CSS 变量 --line-color（line.style.setProperty）；② style.css 默认虚线（repeating-linear-gradient + var(--line-color)），:hover 实线（background: var(--line-color)）+ 3px 宽 + 100% 不透明；③ 移开鼠标自动恢复虚线（浏览器原生 :hover 行为）。
  - 实施：web/static/app.js（--line-color 变量）+ web/static/style.css（默认虚线、:hover 实线）+ main.go（build 计数 911）
  - 测试：浏览器验证默认虚线（repeating-linear-gradient 1px 点+1px 间隙）、hover 实线（纯色 var(--line-color)）、移开恢复虚线均正常。go build+vet 全绿，co-shell 编译到 ~/bin/ [BUILD-911]

---

## v0.40.0 — 开发中

> **版本**: v0.40.0

> **状态**: 🚧 开发中
> **里程碑**: co-shell web ui 窄屏适配（FEATURE-487）
> **说明**: 0.40.0 系列承接 v0.39.0 发布后的新功能。FEATURE-487 从 v0.39.0 移入本版本（v0.39.0 已随 FEATURE-486 发布打标签），完成 co-shell web ui 的窄屏（移动端）适配优化。

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-487 | 0.40.0 | P1 | co-shell web ui 窄屏适配：会话标题循环切换任务进展(翻牌)、消息指示器 hover 无过渡动画等 |

> 当前 BUILD: 908
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-487 co-shell web ui 窄屏适配** ✅ 已完成 [BUILD-908]
  - 背景：co-shell web ui 在窄屏（移动端）下布局与交互不适配。
  - 方案（用户确认）：FEATURE-487 归入 v0.40.0（v0.39.0 已发布）。
  - 实施：web/static/（index.html + style.css + app.js）+ main.go（build 计数）
  - 测试：见 use-case/FEATURE-487/
  - 进度：见 v0.39.0 段原 FEATURE-487 进度（①~㉒），含窄屏布局微调、三段开关改胶囊、录入框16px、modal全屏、单一断点、全宽sheet、会话标题循环切换(翻牌)、消息指示器hover无过渡动画等。go build+vet 全绿，co-shell 编译到 ~/bin/ [BUILD-908]

---

## v0.39.0 — 已完成

> **版本**: v0.39.0

> **状态**: 🚧 开发中
> **里程碑**: hub https 远程安全访问 + 移动端浏览器内核化（FEATURE-485）
> **说明**: 0.39.0 系列为移动端浏览器内核化做准备，改造 co-shell-hub 使其可通过 https 远程安全访问：增加系统设置界面（上传 SSL 证书/自签名生成、配置白名单、设置访问验证 KEY、全部访问需 key 开关），hub 配置证书后仅用 https 监听；对不在白名单的主机访问需提供访问 key（HTTP 头）。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-485 | 0.39.0 | P1 | hub https 远程安全访问：系统设置界面（SSL 证书/白名单/访问 KEY）、https 监听、访问 key 校验、移动端 mobile-legacy 复制 |
| FEATURE-486 | 0.39.0 | P1 | 移动端浏览器内核化：mobile/ 放弃 Flutter 改原生 iOS，内嵌本地代理注入访问 KEY，WKWebView 渲染 hub web ui，系统设置页（服务端地址 + 访问 KEY） |

> 当前 BUILD: 884
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-485 hub https 远程安全访问 + 移动端浏览器内核化准备**
  - 背景：移动端将改为直接访问 co-shell-hub 的 Web UI（浏览器内核方式），需 hub 支持 https 远程安全访问。原 mobile/（UDP 方式）复制为 mobile-legacy/ 保留待以后实现。
  - 方案（用户确认）：① 版本 v0.39.0（minor 递增）；② SSL 证书支持自签名自动生成（也支持上传 PEM）；③ 访问验证 KEY 加到 HTTP 头中（如 X-Access-Key），WEB 访问不提供则先提示输入；④ 配置证书后仅用 https（替换 http）；⑤ 本次范围：hub 端改造（设置界面 + https + 访问 key）+ 移动端 mobile-legacy 复制。
  - 实施：cmd/co-shell-hub/ + hub/gateway/ + mobile/
  - 测试：见 use-case/FEATURE-485/
  - 进度：① hubBadge 窄屏适配：新增 updateBadgeResponsive JS（loadHubInfo 回调 + resize 时调用），当视口宽度 < 徽标完整宽度×2 时隐藏版本号（.ver display:none），徽标自动收缩到只显示 ▸ co-shell-hub（280px→135px）；② 底部按钮上移+间距：.foot padding 8px→8px 8px 18px（底部 +10px，覆盖 agent 列表 ＋新建/⚙设置 和设置页 保存设置），settingsBtn margin-top 6px→16px（两按钮间距 +10px）。端到端验证：iOS 模拟器窄屏徽标版本号隐藏、浏览器 drawer 底部按钮上移+间距生效。go build+vet 全绿，co-shell-hub 编译到 ~/bin/ [BUILD-884]hub 端改造完成（设置界面 + https + 访问 key）+ 移动端 mobile-legacy 复制。实现：① gateway/settings.go 新增 Settings 结构（TLS 开关/证书路径/白名单/访问 KEY/全部需 key 开关），持久化到 hub-settings.json，支持自签名证书自动生成（ECDSA P256）；② webui.go WebUI 持有 settings，Serve() 支持 TLS（配置证书后仅 https），accessControl 中间件实现访问控制（白名单内放行、白名单外需 X-Access-Key 头否则 401、RequireKey 强制全部需 key），新增 GET/PUT /api/settings；③ webui_static.go 设置界面（+新建 下加 ⚙设置 按钮，新增 viewSettings：HTTPS 开关/证书路径/生成自签名/白名单/访问 KEY/全部需 key 开关）；④ cmd/co-shell-hub 加载 settings 并传给 NewWebUI；⑤ 复制 mobile/ → mobile-legacy/（保留 UDP 方式）。编译全绿 [BUILD-880]
  - 进度（迭代调整）：① 设置按钮宽度与 +新建 一致；② 生成自签名证书后把证书路径填入输入框（GET /api/settings 返回 settings_dir）；③ 证书默认放 ~/.co-shell/（hub-cert.pem/hub-key.pem）；④ 访问 KEY 下加"重新生成安全 KEY"按钮（crypto 生成 64 位 hex）；⑤ 修复生成证书后无法访问——TLSConfig 在证书文件不存在时自动生成（不再因加载不存在的相对路径失败）；⑥ 新增 --serve 参数（不自动打开浏览器），自动打开浏览器时按 TLS 启用用 https://；⑦ accessControl 语义：白名单为空时默认仅本机(loopback)免 KEY，其他主机需 KEY（命令行 --whitelist 会覆盖 settings 白名单，需用设置界面管理时勿传 --whitelist）。编译全绿 [BUILD-881]

- [ ] **FEATURE-486 移动端浏览器内核化：原生 iOS + Cookie 注入访问 KEY**
  - 背景：mobile/ 原为 Flutter UDP 客户端（已复制为 mobile-legacy/ 保留）。本次在 mobile/ 中直接改造：放弃 Flutter，先实现原生 iOS 版，通过浏览器控件（WKWebView）渲染 hub 的 web ui 页面。
  - 方案（用户最终确认）：① 在 mobile/ 中直接修改，删除 mobile/ 全部内容重建纯原生 iOS 工程（mobile-legacy/ 保留完整 Flutter UDP 版勿动）；② 放弃 Flutter，纯原生 iOS（Swift + UIKit + WKWebView）；③ 访问 KEY 通过 Cookie 传递——hub 端 accessControl 已支持 access_key Cookie 校验（见 FEATURE-485 迭代：webui.go 新增 accessCookie 常量 + accessKeyFromRequest 函数，同时检查 X-Access-Key 头和 access_key Cookie），key 在移动端原生系统设置页输入（存 Keychain），WKWebView 加载 hub 页面前注入 access_key Cookie，用户无需在网页输入 key；④ 系统设置页（原生 UI）输入服务端地址（如 https://192.168.3.19:23311）+ 访问 KEY，存 Keychain；⑤ 已配置→显示 WKWebView 加载 hub 页面（加载前注入 access_key Cookie）；未配置→显示原生设置页；提供设置入口可随时回设置页；处理自签名 https 证书信任（ATS 例外 + WKWebView 证书校验放行）；⑥ 新任务号 FEATURE-486，归入 v0.39.0。
  - 实施：mobile/（原生 iOS 工程）
  - 测试：见 use-case/FEATURE-486/
  - 进度：① hub 端 accessControl 支持 access_key Cookie（webui.go accessCookie + accessKeyFromRequest，编译通过）；② mobile/ 清空重建，从 Flutter 工程改造成纯原生 iOS 工程：重写 project.pbxproj（去 Flutter/CocoaPods/RunnerTests，单 target 纯 Swift）、Info.plist（去 FLUTTER_BUILD_NAME 占位符，加 ATS 例外 NSAllowsArbitraryLoads + 本地网络权限）、LaunchScreen.storyboard（去 LaunchImage 引用）、AppDelegate/SceneDelegate 改纯 UIKit；③ 新增 Swift 源码：SettingsStore（Keychain 存服务端地址+key）、SettingsViewController（原生设置页，输入服务端地址+key，校验 URL 格式）、WebViewController（WKWebView 壳，加载前注入 access_key Cookie，didReceive 放行自签名证书，target=_blank 同页打开）、RootViewController（导航，未配置→设置页，已配置→WebView，设置保存后切换/重载）；④ xcodebuild 编译到 iPhone 17 Pro 模拟器 BUILD SUCCEEDED，安装启动验证设置页正常渲染（标题/输入框/保存按钮齐全无崩溃）；⑤ 新增 mobile/README.md 说明目录用途；⑥ hub webui.go 新增 requestLog 中间件，向标准输出打印每个请求的时间/源地址/URI（含 X-Forwarded-For 支持），便于追踪移动端远程访问；⑦ 修复设置页保存无反应：模拟器上 Keychain 对未签名 app 不可用（SecItemAdd 返回 errSecMissingEntitlement -34018），SettingsStore 改为 Keychain + UserDefaults 双后端（Keychain 失败自动回退 UserDefaults，真机用 Keychain），并新增 Runner.entitlements（keychain-access-groups）。端到端验证通过：模拟器输入地址+key 保存后成功切换到 WKWebView 并加载 hub web UI（Cookie 注入通过认证、自签名证书正常）。go build+vet 全绿，co-shell-hub 编译到 ~/bin/ [BUILD-883]

- [ ] **FEATURE-487 hub web ui 移动端适配**
  - 背景：移动端（原生 iOS WKWebView）渲染 hub web ui 时，窄屏下 hub 徽标（logo+版本框）过宽遮挡内容，抽屉底部按钮贴底不便触控。
  - 方案（用户确认）：① 当界面宽度小于徽标（logo+版本框）宽度的两倍时，自动隐藏版本信息，徽标外框自动收缩适应剩余 logo 长度；② 各页面底部按钮（Agent 抽屉的 ＋新建/⚙设置、设置页的 保存设置）整体上移 10px（增加与底边间距），并增加 ＋新建 与 ⚙设置 两按钮之间间距 10px。
  - 实施：hub/gateway/webui_static.go
  - 测试：见 use-case/FEATURE-487/
  - 进度：① 窄屏隐藏徽标版本号、底部按钮上移+间距（webui_static.go）；② 窄屏布局微调：主输入栏移到工具图标行上方、发送按钮移到工具行最右并减宽20%、YOLO开关降高15%、主模型/视觉模型/会话清单弹出菜单保证在屏幕内+超长省略号（web/static app.js+style.css）；③ 移动端输入框聚焦自动放大修复（窄屏输入控件字号≥16px）；④ 窄屏优化：静默-极简-正常/act-plan-research 三段开关改为连续点击顺序切换的单个胶囊按钮（降低宽度）、静默-极简-正常开关移到标题栏连接状态按钮左边、运行按钮与YOLO同尺寸、隐藏状态条token总计栏保留迭代用量、隐藏主/视觉模型名只留图标、消息可视化(msgviz)放回会话名右边各占50%；⑤ 窄屏优化：主消息录入框加高20%、主/视觉模型图标移到 act-plan-research 三段开关右边、所有界面录入框字体统一16px；⑥ 优化：所有录入框（含模型配置/系统配置界面）强制16px防移动端自动放大、窄屏模型管理/向导界面宽度不超视口+模型名/ID省略号；⑦ 窄屏时系统设置/模型管理/模型向导 modal 全屏充满页面、配置页标题栏文字右对齐（与关闭图标保持间距）；⑧ 窄屏时模型配置向导界面隐藏左侧步骤页签栏，仅显示右侧配置信息（全宽）；⑨ 修复窄屏系统设置分类页签点击无作用：settings-body 缺 id=settingsBody 导致 settings-drilled 类永不生效，补 id 并打开设置时重置到分类列表；⑩ 新增更窄断点 body.narrower（视口<品牌宽度×1.5）：工具行图标右对齐、仅 act-plan-research 控件左对齐、隐藏状态栏迭代 token 用量(#sbLast)；⑪ 统一屏幕宽度标准为单一断点：视口≥品牌宽度×3 为宽屏（正常）、视口<品牌宽度×3 为窄屏，移除 body.narrower 更窄断点，将其布局效果（工具行图标右对齐、隐藏迭代 token 用量）合并到 body.narrow；⑫ 窄屏时主模型/视觉模型/会话三个弹出菜单改为全宽 sheet（position:fixed 铺满视口宽度，bottom 对齐工具行顶部向上展开，JS 设 --menu-bottom）；⑬ 窄屏时状态条迭代 token 用量(#sbLast)保留文字、仅隐藏 ⏱️ 图标（图标包进 .sb-last-ico span）；⑭ web ui 三项优化：① 系统菜单增加会话标题/消息可视化两个显示开关（状态条开关上方，localStorage 持久化）；② 失败消息颜色由橙红改纯红 #FF0000（指示灯圆点/可视化线段/高风险标签/报错文字，不含文本工具被替换删掉的文本）；③ 消息可视化控件：线段改点虚线(1px点+1px间隔+75%透明)、token用量黑点扩为3x3且中心对齐、hover线段变3px宽+100%不透明、tip显示消息类型/工具意图/上下文占比；⑮ iPhone X+ 圆角屏适配：viewport 加 viewport-fit=cover 让页面充满屏幕，底部 #bottom 与窄屏全屏 modal 加 env(safe-area-inset-bottom) padding 避开 Home Indicator；⑯ 窄屏时最下方状态栏 token 用量(#sbLast)居中显示（statusbar justify-content:center）；⑰ 窄屏工具行优化：运行按钮(#sendBtn)宽度调成与 YOLO 一致(34px)以便与其他图标同行；⑱ 窄屏工具行恢复右对齐：act-plan-research 开关(modeSeg)靠左(margin-right:auto)，从主模型开始向右的所有图标右对齐，使开关长度变化不影响其他图标位置；⑲ 去掉右上角系统菜单中的"重启后台"选项（移除 miRestart 菜单项及 JS 逻辑）；⑳ 消息可视化控件优化：hover 线段变宽时 margin-right 归零(1px+2px=3px+0px)避免后续线段抖动、token 用量黑点改 3x2。go build+vet 全绿，co-shell 编译到 ~/bin/ [BUILD-906]；㉑ 窄屏时会话标题栏的会话标题处改为只读循环切换控件：每2秒上下翻牌切换 会话标题(不可修改)→目标:{任务进展描述}→正在执行:{进行中步骤}→进展情况:已完成(完成数/总数)，无任务计划时静态显示会话标题，超长省略号。go build+vet 全绿，co-shell 编译到 ~/bin/ [BUILD-907]；㉒ 消息指示器控件(msgviz)鼠标划过焦点线段变粗及恢复时移除 transition 过渡动画，直接同时切换线段宽度与间距。go build+vet 全绿，co-shell 编译到 ~/bin/ [BUILD-908]

---

## v0.38.0 — 开发中

> **版本**: v0.38.0

> **状态**: 🚧 开发中
> **里程碑**: hub WebSocket 聚合网关架构（FEATURE-484）
> **说明**: 0.38.0 系列将 co-shell-hub 从"UDP + stdin/stdout 管道"升级为 **WebSocket 聚合网关**架构：hub 提供传统 TCP 服务 + API Key 认证（加密交给 VPN），作为 WebSocket 客户端独占连接多个 co-shell agent（每 agent 一个 --serve 端口），代理转发不处理业务逻辑，支持多 agent 切换与数据缓存，对外提供 Web UI。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-484 | 0.38.0 | P1 | hub WebSocket 网关架构：TCP+API Key 认证层、WebSocket 客户端代理转发、多 agent 切换与缓存、Web UI、移动端适配 |

> 当前 BUILD: 856
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-484 hub WebSocket 网关架构**
  - 背景：原 hub（FEATURE-128/183）通过 UDP 与移动端通讯、stdin/stdout 管道管理 co-shell agent。用户确认架构演进：hub 通过 co-shell `--serve` WebSocket 端口与多个 co-shell 通讯，对外提供 TCP + API Key 服务与 Web UI 聚合多 agent。
  - 方案（已确认简化）：① 传统 TCP 服务 + API Key 认证（加密交给 VPN）；② 原则上不处理 co-shell 业务逻辑，仅代理转发；③ 同时与多个 agent 通讯，负责多 agent 切换；④ Web UI 转发 co-shell 返回信息，实现 agent 切换与数据缓存（切换不丢失会话）。
  - 代码组织（用户确认方案 B）：在 hub/ 下新建独立 gateway 包，旧 UDP 代码保留不动，新代码独立演进。
  - Web UI 访问控制（用户确认，步骤6要求）：hub Web UI 与 co-shell 一致支持访问白名单，默认仅本机访问——复用 co-shell 模式（FEATURE-430/431）：`--bind`（默认 127.0.0.1）+ `--whitelist`（逗号分隔 IP/网段，空=仅本机访问）；无白名单时强制本机访问（忽略 --bind，绑定 127.0.0.1）；白名单校验支持精确 IP 与 CIDR 网段（复用 web/server.go 的 parseWhitelist/ipAllowed 逻辑）。
  - 实施：hub/gateway/（新）+ cmd/co-shell-hub/ + mobile/
  - 测试：见 use-case/FEATURE-484/
  - 进度：步骤3-6（TCP+API Key 认证、WS 客户端代理转发、多 agent 切换与缓存、Web UI）已完成 [BUILD-857]；步骤7-8（agent 生命周期管理：Manager 注册表持久化 + co-shell --serve 子进程启停 + Web UI 管理界面创建/启动/停止/删除/添加不受控 agent）已完成，编译全绿、管理 API 与 Web UI 浏览器验证通过 [BUILD-861]
  - Web UI 架构演进（用户确认，架构讨论后）：hub 从"聚合转发业务消息 + 自绘简化聊天界面"转向 **iframe 多页外壳 + 反向代理**——hub 完整透传每个 co-shell 实例的 Web UI（含界面），只维护外壳（agent 切换 + 管理 + 移动端自适应）。单用户独占；hub 反向代理解决跨源/端口转发；数据缓存降级为可选优化。方案：单端口 + 子路径前缀（/agent/{id}/），co-shell 自身前端改相对路径（方式乙，单一代码源），hub 反向代理剥前缀转发。设计见 docs/DESIGN-hub-gateway.md §10。
  - 进度（新方向）：设计文档 §10 已完成（iframe 外壳 + 反向代理 + co-shell 前端改相对路径方案）；步骤12-13（完整实现）已完成：① co-shell 前端改相对路径（web/static/index.html + app.js 全部 /api、/logos、/static/logos、WS 地址改相对）；② hub 反向代理模块（hub/gateway/proxyhttp.go，/agent/{id}/ 前缀剥除后 httputil.ReverseProxy 转发 HTTP+WS）；③ hub 外壳页面（webui_static.go 重写为 agent 切换栏 + 管理抽屉 + iframe 布局 + 移动端自适应）；④ webui.go 注册 /agent/ 反向代理并移除旧 /ws 聊天桥接。编译全绿（hub 模块 + 主模块），浏览器实测通过：两个 agent（ws-a/ws-b）各自 iframe 完整显示 co-shell UI 且均"已连接"（WS 经反向代理隧道成功），点击标签可切换 agent 且状态保留 [BUILD-862]
  - 进度（Web UI 管理界面迭代）：卡片交互与详情字段修复（点击 agent 卡片切换主界面、卡片最右侧 > 符号点击查看配置分离两功能；本地/远程 agent 详情显示不同字段，远程隐藏 workspace/co-shell/共享配置、端口行显示 WS 地址；创建确认按钮移到页面最下方 foot 与 +新建 设计语言一致）[BUILD-875]；修复点击卡片切换主界面失效——根因 ensureFrame 在 frame 已存在时直接 return 不调用 showFrame 切换 active iframe，导致点击卡片只更新卡片高亮但主界面 iframe 不切换；修复：点击卡片切换时在 ensureFrame 后显式调用 showFrame(current)，浏览器实测双向切换（agent-1↔agent-2）主界面 iframe 同步切换正常 [BUILD-876]
  - 进度（连接状态指示 + serve/license 优化）：① hub 外壳页面左上角 co-shell-hub logo 左边的小三角（.mark ▸）通过颜色显示与 hub 的 API 轮询连接状态——连接正常=accent 色，断开=灰色；检测到断开后停止自动轮询（避免多浏览器互抢单客户端连接），点击三角或刷新页面触发重连（startPolling/stopPolling/setConn 控制）[BUILD-877]；② co-shell serve 模式运行时忽略模型配置向导（跳过 AddModelWizard，用户通过 Web UI 配置模型）；③ 新增 --accept-license 参数表示同意用户使用协议，跳过 disclaimer 交互提示并持久化 DisclaimerAccepted=true 到 config [BUILD-877]
  - 进度（agent 补充运行参数界面）：agent 配置界面（新增/查看）新增"补充运行参数"字段，运行 co-shell agent 时除系统自动传递的参数（workspace/config/port 等）外附加用户自定义参数。实现：① AgentSpec 增加 ExtraArgs 字段（manager.go，持久化 extra_args）；② Start 启动命令构造时按空白拆分附加用户参数，字段为空时默认补 --accept-license（--serve 已由系统传递不重复）；③ webui.go agentView/createAgentRequest 增加 ExtraArgs 字段并透传 CreateManaged；④ webui_static.go 新增配置界面输入框（默认值 --accept-license）、详情界面展示字段（远程 agent 隐藏）、createLocalAgent POST body 带 extra_args。编译全绿 [BUILD-879]
  - 进度（启动体验 + 配置搜索 + 开关冲突修复）：① hub 服务启动后默认自动打开浏览器（web.OpenBrowser + browserURL 将 0.0.0.0 监听地址转换为 loopback URL）[BUILD-878]；② hub-agents.json 注册表默认搜索路径与 hub-gateway.json 一致：命令行指定 > ./ > ~/.co-shell/（firstExisting/homeDir 辅助函数）[BUILD-878]；③ 修复 agent 卡片启动/停止开关点击冲突——点击开关时 click 事件冒泡到卡片触发切换逻辑并 renderList 重建 DOM，导致开关 change 事件丢失（开关状态不变）；修复：开关 label 添加 click 事件 stopPropagation 分离开关点击与卡片切换，浏览器实测点击开关正常触发启停且开关状态正确变化 [BUILD-878]

---

## v0.37.1 — 开发中

> **版本**: v0.37.1

> **状态**: 🚧 开发中
> **里程碑**: msgviz 红点样式修复
> **说明**: 0.37.1 系列修复消息可视化控件（msgviz）上下文占用红点的显示样式：将纯红色 1px 红点改为控件底色（使所在线段看起来断开），并加高到 2px。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-483 | 0.37.1 | P1 | msgviz 上下文占用红点改为控件底色（线段断开效果）并加高到 2px；线段颜色固定为 dark 色调不随主题改变 |

> 当前 BUILD: 851
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FIX-483 msgviz 上下文占用红点改为控件底色并加高到 2px**
  - 背景：FEATURE-482 消息可视化控件中，代表上下文占用的红点当前是纯红色（#ff0000）1px×1px，叠加在彩色线段上。用户希望红点改为控件的底色（stream-head 背景色 var(--bg-panel)），使红点位置看起来像线段在该处断开（被挖空），并加高到 2px，让上下文占用位置更清晰可辨。
  - 方案（已确认）：① style.css `.msgviz-dot` 背景色从 `#ff0000` 改为 `var(--bg-panel)`（控件底色），高度从 1px 改为 2px；② app.js `msgVizFlush` 中 `dotH` 从 1 改为 2，使红点垂直位置计算（range = h - dotH）与 2px 高度匹配。
  - 实施：`web/static/style.css`（.msgviz-dot 背景色 + 高度）+ `web/static/app.js`（dotH 常量）
  - 测试：见 use-case/FIX-483/

---

## v0.37.0 — 开发中

> **版本**: v0.37.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 消息可视化
> **说明**: 0.37.0 系列在 Web UI 会话标题栏新增消息可视化图表控件，集中显示各类消息状态、数量与上下文占用。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-482 | 0.37.0 | P1 | 会话标题栏新增消息可视化图表控件：1px 色线表示每条消息（颜色=标题栏指示灯色），红点高度表示上下文占用，点击定位，无限追加+拖拽滑动 |

> 当前 BUILD: 845
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-482 会话标题栏消息可视化图表控件** [BUILD-848]
  - 背景：Web UI 会话标题栏（stream-head）中，会话标题输入框与显示模式三段开关之间有大片空白。希望新增一个消息可视化图表控件，集中通过不同颜色简洁显示各类输入、输出、错误处理等消息状态和数量，以及上下文占用情况，并能点击色块快速定位到实际上下文中消息所在位置。
  - 方案（已确认）：在 stream-head 中 #streamTitle 固定 200px 后插入可视化控件（flex:1 占满剩余），再是 #streamMode。控件以 1px 宽纵向色线表示每条消息（颜色=该消息块标题栏指示灯颜色），每条色线中间一个纯红点，红点高度表示该消息产生时的累计上下文占用比例（占满=主模型最大上下文）。点击色块定位到对应 .ev 块。线段无限追加（浏览器刷新才清空），超出控件宽度时拖拽左右滑动。
  - 实施：`web/static/index.html`（stream-head 布局 + 控件容器）+ `web/static/style.css`（控件样式：色线/红点/拖拽）+ `web/static/app.js`（makeBlock 挂钩记录消息 + token_iter 补记上下文占用 + 渲染色线红点 + 点击定位 + 拖拽滑动）
  - 测试：见 use-case/FEATURE-482/

---

## v0.36.0 — 开发中

> **版本**: v0.36.0

> **状态**: 🚧 开发中
> **里程碑**: 环境感知增强
> **说明**: 0.36.0 系列增强 co-shell 的环境感知能力，让 co-shell 自己知道运行环境和启动配置。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-481 | 0.36.0 | P1 | 系统信息增加运行时环境：进程号、版本号、build号、服务模式（stdio/enhanced/serve），serve 模式含端口号、白名单（如有）、bind 地址 |

> 当前 BUILD: 841
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-481 系统信息增加运行时环境感知** [BUILD-845]
  - 背景：co-shell 需要让 LLM 自己知道当前运行环境和启动配置，以便更好地理解自身运行状态。当前 `<environment_details>` 只包含时间、消息号、上下文窗口、cwd、文件列表等，缺少进程号、版本号、build号、服务模式等运行时信息。
  - 方案（已确认）：在 agent 上新增运行时环境信息注入机制，在 `<environment_details>` 中输出 `<runtime_info>` 块，包含进程号、版本号、build号、服务模式（stdio/enhanced/serve）；serve 模式额外输出端口号、白名单（如有）、bind 地址。
  - 实施：`agent/agent.go`（RuntimeInfo 结构体 + SetRuntimeInfo/RuntimeInfo setter）+ `agent/loop.go`（Agent 结构体 runtimeInfo 字段）+ `agent/envelope.go`（buildRuntimeInfo 方法 + buildFullEnvironmentDetails 输出 runtime_info）+ `main.go`（agent 创建后注入 pid/version/build；stdio 单命令注入 serviceMode=stdio；startWebUI 注入 serve 端口/bind/白名单；REPL 启动前注入 enhanced）+ `agent/runtime_info_test.go`（5 个单元测试）
  - 扩展（本次会话）：① `<runtime_info>` 增加 `<model_name>`（当前消息发送给模型的 API model name）——`agent/agent.go` RuntimeInfo 新增 ModelName 字段 + `agent/envelope.go` buildRuntimeInfo 在 `<build>` 后渲染 `<model_name>`（非空才输出）+ `main.go` SetRuntimeInfo 从 activeModel.Model 注入；② META-CAPABILITIES 新增【环境感知】静态段落（服务模式感知/用户动态感知/模型参数感知）——`i18n/keys.go` 新增 KeyCapEnvAwareness + `zh_system.go`/`en_system.go` 双语内容 + `agent/capability.go` buildMetaCapabilityIndex 末尾追加（修正：最初误加到 formatCapabilityList，后移到 buildMetaCapabilityIndex）+ `agent/capability_test.go` TestMetaCapabilityIndex_EnvAwareness
  - 测试：见 use-case/FEATURE-481/

---

## v0.35.3 — 开发中

> **版本**: v0.35.3

> **状态**: 🚧 开发中
> **里程碑**: simple 完成模式选项显示修复
> **说明**: 0.35.3 系列修复 attempt_completion simple 完成模式完成提示框无选项的问题。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-480 | 0.35.3 | P1 | 修复 simple 完成模式完成提示框无选项：simple 分支 Interaction 只设置 Keys 未设置 Options，前端 renderVirtualKeyboard 因 options 为空不渲染任何选项按钮 |

> 当前 BUILD: 841
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-480 修复 simple 完成模式完成提示框无选项** [BUILD-841]
  - 背景：attempt_completion 设为 simple 模式时，完成提示框里没有任何选项。simple 分支构造的 Interaction 只设置了 Keys（两个 KeyOption），未设置 Options。前端 showInteraction 中 `if (it.kind === "select" && it.options && it.options.length)` 才调用 renderVirtualKeyboard 渲染选项按钮，simple 模式 options 为空导致整个虚拟键盘（含 keys 按钮）都不渲染。
  - 方案（已确认）：simple 分支保持仅 Keys（两个固定按键："-" 确认任务完成 / "+" 继续输入需求），不设 Options（与 TUI askSelect 的 keys-only select 模型一致）。修复前端：① showInteraction 门控放宽为 options 或 keys 任一非空即渲染；② renderVirtualKeyboard 新增 keys-only select 分支，仅渲染两个 [Key] Label 按钮（不渲染编号选项、不渲染额外补充按钮），点击 exit 发送 {action:select,value:exit} 使后端完成，点击 continue 发送 value 使后端继续。
  - 实施：`agent/tools.go`（simple 分支保持仅 Keys）+ `web/static/app.js`（showInteraction 门控 + renderVirtualKeyboard keys-only select 分支 + keyMap 注册）
  - 测试：见 use-case/FIX-480/

---

## v0.35.2 — 开发中

> **版本**: v0.35.2

> **状态**: 🚧 开发中
> **里程碑**: attempt_completion 完成模式三态化
> **说明**: 0.35.2 系列将 attempt_completion 的完成确认行为从布尔开关扩展为三态完成模式（主动/简单/退出），并新增 completion-mode 系统参数放到[安全与确认]设置页。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-479 | 0.35.2 | P1 | attempt_completion 完成模式三态化：将 AttemptCompletionConfirm 布尔开关扩展为 completion-mode 三态（active 主动=显示完整确认框含 LLM next_steps/简单 simple=只显示确认完成+继续输入两个固定选项/退出 exit=直接完成不显示确认框），新增系统参数放[安全与确认]设置页 |

> 当前 BUILD: 838
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-479 attempt_completion 完成模式三态化** [BUILD-838]
  - 背景：attempt_completion 的完成确认行为目前只有一个布尔开关 AttemptCompletionConfirm（默认 true），且未暴露到系统设置页。用户希望将完成确认行为扩展为三种模式：① 主动模式（active）：显示 LLM 收集的选项 + 下一步建议 + 取消 + 补充信息 + 完成（同当前 confirm=true 做法）；② 简单模式（simple）：只给用户提供【确认任务完成】和【继续输入需求】两个固定选项，不再显示 LLM 希望收集的问题；③ 退出模式（exit）：直接退出，不显示任何附加完成提示框（同当前 confirm=false 做法）。
  - 方案（已确认）：① 将 config.LLMConfig.AttemptCompletionConfirm 布尔字段扩展为 CompletionMode 三态字符串字段（active/simple/exit，默认 active），保留 AttemptCompletionConfirm 兼容（active 对应 true、exit 对应 false）；② agent/tools.go attemptCompletionTool 按 CompletionMode 分支：active=现有完整确认框、simple=只显示【确认任务完成】+【继续输入需求】两个固定选项（继续输入把用户输入发回 LLM 继续循环）、exit=直接完成不弹框；③ 新增 completion-mode 系统参数，放[安全与确认]设置页（REPL .set + Web UI 设置 + LLM 工具设置）。
  - 实施：`config/config.go`（LLMConfig 新增 CompletionMode 字段 + 默认 active）+ `agent/tools.go`（attemptCompletionTool 按 CompletionMode 三态分支）+ `cmd/settings_safety.go`/`cmd/settings_web.go`（completion-mode 设置项 + safetyGroup 显示）+ `agent/settings_tools.go`（getSettingValue/applySetting）+ `i18n/`（KeyCol3CompletionMode 等文案）+ `agent/attempt_completion_test.go`（三态测试）
  - 测试：见 use-case/FEATURE-479/

---

## v0.35.1 — 开发中

> **版本**: v0.35.1

> **状态**: 🚧 开发中
> **里程碑**: light-tp/paper 色调图标放大
> **说明**: 0.35.1 系列为小改进版本，调整 FEATURE-477 新增的 light-tp/paper 两个色调在 themeToggle 按钮上的图标尺寸。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-478 | 0.35.1 | P1 | light-tp/paper 色调图标放大：themeToggle 按钮在 light-tp（♤）与 paper（☕︎）色调下图标比 ☾/☀ 视觉偏小，按比例放大——light-tp +10%（14px→15px）、paper +30%（14px→18px） |

> 当前 BUILD: 831
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-478 light-tp/paper 色调图标放大 + IME 回车误触发修复**
  - 背景：① FEATURE-477 新增 light-tp（♤）与 paper（☕︎）两个色调后，themeToggle 按钮上这两个色调的图标（♤/☕︎）比 dark/light 的 ☾/☀ 视觉上偏小，需要按比例放大以保持视觉一致；② 输入框监听键盘按键实现快捷键，回车是发送键，但使用输入法（IME）时按回车选词会误触发发送，体验不佳。
  - 方案（已确认）：① 在 `web/static/style.css` 中为 `[data-theme="light-tp"] #themeToggle` 与 `[data-theme="paper"] #themeToggle` 分别设置放大后的 font-size——light-tp 放大 10%（14px→15px）、paper 放大 30%（14px→18px）；dark/light 保持原尺寸；② 用 `KeyboardEvent.isComposing` + document 级 `compositionstart`/`compositionend` 维护 `window.__composing` 兜底标志，新增 `imeComposing(e)` 辅助函数，主输入框及各处 Enter 触发逻辑在 IME 组合期间不响应回车（选词），输入完成后正常响应。
  - 实施：`web/static/style.css`（新增 `[data-theme="light-tp"] #themeToggle { font-size: 15px; }` + `[data-theme="paper"] #themeToggle { font-size: 18px; }`）+ `web/static/app.js`（新增 `window.__composing` 标志 + document compositionstart/compositionend 监听 + `imeComposing(e)` 辅助函数；主输入框回车发送、askInput、askInteraction inp、模型向导 inp、会话标题 streamTitle 共 5 处 Enter 触发逻辑加 `!imeComposing(e)` 判断）[BUILD-832]
  - 测试：见 use-case/FEATURE-478/

---

## v0.35.0 — 开发中

> **版本**: v0.35.0

> **状态**: 🚧 开发中
> **里程碑**: ResultMode 节静态化
> **说明**: 0.35.0 系列将系统提示词的 RESULT MODE 节从"只描述当前模式"改为静态列出所有已配置模式（标题如 ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE + 引导句 + 各模式介绍），让 LLM 同时了解各模式差异，避免模式切换导致系统提示词内容变化。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-472 | 0.35.0 | P1 | ResultMode 节静态化：遍历所有已配置模式生成标题与各模式介绍，填充 KeyWorkModeAct/Plan/Research 中英双语资源 |
| FEATURE-473 | 0.35.0 | P1 | track_task_progress 说明优化：强调该方法主要用于初始化任务计划，之后的任务执行跟踪通过工具调用透明化中的 meta 对象（meta.progress）更新，不用重复调用 |
| FIX-474 | 0.35.0 | P1 | 修复 Web UI TOOL 块标题栏意图错位：iterToolBlocks/curTool 只在 done 事件清空、不在每次 LLM 迭代（token_iter）边界清空，导致跨迭代孤儿块污染意图回填定位，使某工具块的意图被错误填到另一个工具块上 |
| FIX-475 | 0.35.0 | P1 | 修复 Web UI 任务执行卡住不输出：WebSocket 事件推送是同步无超时的 TCP 写且跑在 agent 主循环 goroutine 上，浏览器接收缓冲满时 conn.Write 无限阻塞冻结整个 agent 循环；改为写超时 + 异步推送解耦 |
| FEATURE-476 | 0.35.0 | P1 | Web UI 亮色主题色调调整：按样本界面蓝灰配色，仅左侧工作区面板——工作区标题栏深蓝背景、分支独立浅蓝半透明小块、文件列表浅灰背景；主标题栏底边深蓝加粗边框 |
| FEATURE-477 | 0.35.0 | P1 | 系统 logo 配置功能：在系统设置"外观与显示"中按实际主题(dark/light)各配置一个系统 logo，存工作区 logos/ 目录；配置后 logo 显示在 topbar 左上角"▸ co-shell"左边，缩放为标题栏高度保持宽高比，co-shell 文字右移不盖住 logo；未配置主题保持现状 |

> 当前 BUILD: 811
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-472 ResultMode 节静态化** [BUILD-806]
  - 背景：系统提示词 RESULT MODE 节当前只根据当前 modeName 注入当前模式的描述（KeyWorkModeAct/Plan/Research 填充 KeySystemPromptResultMode 的 %s），且这三个资源内容为空。LLM 无法同时了解各模式区别，不利于具体思考。
  - 方案（已确认）：① 遍历所有已配置模式（config.DefaultWorkModes + 用户自定义 cfg.WorkModes 去重），将模式名大写后接 MODE，用 " V.S. " 连接作为 RESULT MODE 节标题（如 ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE）；② 接引导句 "In each user message, the environment_details will specify the current mode. There are %s modes:"；③ 按顺序分别填入各模式介绍（KeyWorkModeAct/KeyWorkModePlan/KeyWorkModeResearch）；④ 参考 notes/cline.json 的 ACT MODE V.S. PLAN MODE 段落填充 KeyWorkModeAct/Plan 资源，并拟写 KeyWorkModeResearch 初稿；⑤ 中英双语维护（zh_system.go + en_system.go）；⑥ 三个内置模式都包含 ResultMode 节。
  - 实施：`agent/system_prompt.go`（buildNamedSection/getRawSectionText 的 ResultMode case 改为遍历所有模式生成静态节）+ `i18n/zh_system.go`/`en_system.go`（填充 KeyWorkModeAct/Plan/Research）+ `i18n/keys.go`（如需新增引导句/标题模板键）
  - 测试：见 use-case/FEATURE-472/

- [ ] **FEATURE-473 track_task_progress 说明优化（强调初始化计划 + meta.progress 更新）** [BUILD-811]
  - 背景：`track_task_progress` 的说明（工具定义 Description、KeyToolUsageTrackTaskProgress、KeySystemPromptToolUsageTaskProgress）未明确强调该方法主要用于**初始化任务计划**，导致 LLM 在执行过程中反复调用 track_task_progress 更新计划，而不是通过其他工具调用的 meta.progress 增量更新，造成冗余调用与上下文浪费。
  - 方案（已确认）：在 `track_task_progress` 相关说明中强调：该方法**主要用于初始化任务计划**（一次性传递完整 steps 数组建立计划）；之后的任务执行跟踪**通过工具调用透明化中的 meta 对象（meta.progress）更新**，**不用重复调用 track_task_progress**。
  - 实施：`agent/tools.go`（track_task_progress 工具定义 Description 强调初始化计划 + meta.progress 更新，不重复调用）+ `i18n/en_system.go`/`zh_system.go`（KeyToolUsageTrackTaskProgress + KeySystemPromptToolUsageTaskProgress 同步强调）[BUILD-811]
  - 测试：见 use-case/FEATURE-473/

- [x] **FIX-474 修复 Web UI TOOL 块标题栏意图错位（跨迭代孤儿块污染意图回填）**
  - 背景：Web UI 中 TOOL 块标题栏的意图(intent)信息有时显示错位（张冠李戴、间歇性出现）——某个 TOOL 块标题栏显示的意图是另一个工具调用的意图。
  - 根因：`web/static/app.js` 的意图回填定位逻辑（FIX-462 引入的 iterToolBlocks 机制）中，`iterToolBlocks`/`curTool`/`toolBlockByName` 只在 `done` 事件时清空（1058 行），不在每次 LLM 迭代（`token_iter`，1027 行只重置 curLLM/curThinking/curSup）边界清空。因此一个任务内多次迭代产生的 TOOL 块持续累积在 `iterToolBlocks` 里；当其中存在孤儿块（在 tool_call_stream 中因 ⚙️ 头部创建了块、但对应 tool_call input 事件从未到达导致 _intentFilled 一直为 false，如 track_task_progress/attempt_completion/view_task_plan 等不要求 meta 的工具——后端 run_stream.go 1356-1379 整段跳过 input 事件发射，但 ⚙️ 头部仍照常发射创建块）时，下一次工具调用的意图回填优先按 iterToolBlocks 创建顺序取第一个未填充块，被错误填到孤儿块上，造成张冠李戴。
  - 方案（已确认）：在 `web/static/app.js` 的 `token_iter`（每次 LLM 迭代结束）分支中，同时重置工具块跟踪状态：`curTool = null; toolBlockByName = {}; iterToolBlocks = [];`（与 done 分支一致），使每次迭代从干净状态开始，避免跨迭代孤儿块污染。
  - 实施：`web/static/app.js` ① token_iter 分支重置 curTool/toolBlockByName/iterToolBlocks（与 done 分支一致）；② 意图回填优先按工具名匹配块（iterToolBlocks.find 加 b.toolName === summary.tool_name 条件），跳过不同名的孤儿块，同名工具仍按创建顺序解析 [BUILD-812]
  - 测试：见 use-case/FIX-474/

- [x] **FIX-475 修复 Web UI 任务执行卡住不输出（WebSocket 同步写阻塞冻结 agent 循环）**
  - 背景：Web UI 任务执行时（不太常见）卡住不输出，运行按钮保持"正在运行"，按什么键都不管用；有时连接状态"已连接→已断开"后又继续收到数据；有时报浏览器 WebSocket 无法接收数据错误。
  - 根因：事件推送链路 `agent.RunStream → cb → WebRenderer.Render → sendEvent → sendJSON → wsConn.WriteMessage → net.Conn.Write` 全部同步、无缓冲、无写超时，且跑在 agent 主循环 goroutine 上。当浏览器端 TCP 接收缓冲满（标签页后台节流/前端处理不过来/网络拥塞）时 `conn.Write` 无限阻塞，冻结整个 agent 循环——不再产生事件、不再返回 await_input/done，运行按钮无法复位，按键/打断均无效。
  - 方案（已确认）：修复方向1——① 给 WebSocket 写加超时（SetWriteDeadline，超时即关闭连接返回错误）；② 事件推送改为异步队列 + 独立写 goroutine 解耦，agent 永不因网络阻塞（channel 满时丢弃最旧事件或降级，避免 agent 冻结）。
  - 实施：`web/ws.go`（wsConn 增加有界 outCh 队列 + 独立 writeLoop goroutine 消费，WriteMessage 改为非阻塞入队（队列满丢弃不阻塞生产者），writeFrame 加 SetWriteDeadline 写超时（10s，超时关闭连接），Close 用 closeOnce 停止 writer goroutine）+ `web/server.go`（handleWS 读循环结束 defer c.Close() 停止 writer goroutine 防泄漏）+ `web/ws_test.go`（新增 TestWSWriteDoesNotBlockOnStalledClient 验证客户端停读时生产者不阻塞）[BUILD-813]
  - 测试：见 use-case/FIX-475/

- [x] **FEATURE-476 Web UI 亮色主题色调调整（样本蓝灰配色）** [BUILD-815]
  - 背景：用户希望按样本界面（企业级蓝灰配色）对 co-shell Web UI 的亮色主题（[data-theme="light"]）做色调调整，色调版本不变（仍是亮色），仅调整各区域背景/文字颜色，引入"深蓝标题栏 + 浅蓝半透明次级条 + 浅灰内容区"的分层配色。
  - 方案（已确认）：仅左侧工作区面板——① 工作区标题栏（#sidebar .panel-head）背景改深蓝（约 #1F4E79），文字改白色；② 分支（#wsBranch）做成独立浅蓝半透明小块（约 #E8F0FE 半透明），与深蓝标题栏区分；③ 文件列表（#tree）背景改浅灰（约 #F2F2F2）；④ co-shell 主标题栏（#topbar）底边用深蓝画比现在粗一点的边框（参考样例左上 logo 栏底边）。
  - 实施：`web/static/style.css`（[data-theme="light"] 变量块新增深蓝/浅蓝/浅灰变量 + 针对 #sidebar .panel-head / #wsBranch / #tree / #topbar 的亮色覆盖规则，用 [data-theme="light"] 前缀限定不影响暗色主题）
  - 测试：见 use-case/FEATURE-476/

- [x] **FEATURE-477 系统 logo 配置功能** [BUILD-827]
  - 背景：用户希望 co-shell Web UI 支持配置系统 logo，按实际主题(dark/light)各配一个，配置后显示在 topbar 左上角"▸ co-shell v0.35.0"左边，替代/前置默认的 ▸ 标记，co-shell 文字右移不盖住 logo。
  - 方案（已确认）：① logo 按实际主题(dark/light)各配置一个（auto 时跟随当前解析出的主题），存工作区 `logos/` 目录（logo-dark.png / logo-light.png）；② 用 HTTP 接口上传/读取——POST /api/logo（multipart 或 base64，参数 theme=dark/light）+ GET /logos/{theme}（读取 logo 图片）；③ 设置 UI"外观与显示"分组增加 logo 配置项（dark/light 各一个上传按钮，支持剪贴板粘贴上传）；④ topbar .brand 最左边插入 <img> 显示当前主题 logo，缩放高度=标题栏高度(44px)保持宽高比，co-shell 文字右移留空隙；未配置主题保持现状（显示 ▸ co-shell）。
  - 实施：`web/server.go`（新增 /api/logo 上传 + /logos/{theme} 读取路由，logo 存工作区 logos/）+ `cmd/settings_web.go`（displayGroup 增加 logo 配置项）+ `web/static/app.js`（渲染 logo 上传控件 + topbar logo 显示与缩放）+ `web/static/index.html`/`style.css`（.brand 结构 + logo 样式）
  - 测试：见 use-case/FEATURE-477/

---

## v0.34.0 — 开发中

> **版本**: v0.34.0

> **状态**: 🚧 开发中
> **里程碑**: 动态感知机制改进
> **说明**: 0.34.0 系列改进动态感知机制：建立动态信息队列，实时监控用户动作（粘贴截图/上传文件/任务执行期间补充消息/点击打开文件），在用户提交命令或每次工具返回结果时，将队列中的动态事件注入 `<environment_details>` 的 `<user_dynamic_events>` 标签（含 clip_objects/upload_files/user_messages/open_files 子标签），替代现有 `<<<DYNAMIC>>>` 机制。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-471 | 0.34.0 | P1 | 动态感知机制改进：动态信息队列 + user_dynamic_events 注入 + WebSocket dynamic_event 通道 |

> 当前 BUILD: 789
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-471 动态感知机制改进**
  - 背景：现有 `<<<DYNAMIC>>>` 机制只在用户提交消息那一刻生成附件信息块，任务执行期间用户的新动作（粘贴截图/上传文件/补充消息/打开文件）无法被 LLM 感知。
  - 方案（已确认）：① 建立动态信息队列 Q，实时监控用户动作（粘贴截图/上传文件/任务执行期间补充消息/点击打开文件），动作发生时将动作及内容加入队列；② 用户提交命令或每次工具返回结果给 LLM 时，从队列提取所有缓存的动态事件，放入 `<environment_details>` 的 `<user_dynamic_events>` 标签（含 `<clip_objects>`/`<upload_files>`/`<user_messages>`/`<open_files>` 子标签，均含文件大小与时间），替代 `<<<DYNAMIC>>>` 机制；③ 根据时机，clip_objects/upload_files/open_files 可放入 tool 和 user 消息，user_messages 只能放入 tool 消息；④ 队列容量默认 100 可经系统参数配置；⑤ user_messages 任务结束未消费时，将未消费消息内容中间加换行插入到消息录入框内容头部，供用户直接提交；⑥ 新增 WebSocket dynamic_event 消息通道，前端在任务执行期间也能上报动作事件；⑦ CLI 端事件不入队列。
  - 实施：`agent/dynamic_events.go`（动态事件队列：AddDynamicEvent/消费/去重/容量上限/PendingUserMessages）+ `agent/envelope.go`（buildFullEnvironmentDetails 注入 user_dynamic_events，user_messages 仅 tool 消息）+ `agent/loop.go`（dynEvents 字段）+ `web/server.go`（clientMessage.Kind + serverMessage.Backfill）+ `web/session.go`（dynamic_event 分支 + done 事件回填）+ `web/static/app.js`（上传/打开/补充消息上报 dynamic_event + backfillInput 回填 + 移除 composeDynamicText）+ `config/config.go`/`cmd/settings.go`/`cmd/settings_web.go`/`i18n/`（dynamic-event-queue-size 参数，默认100）
  - 测试：`agent/dynamic_events_test.go`（7 个单测：入队/排空/容量/去重/user vs tool/文件 stat/回填）；见 use-case/FEATURE-471/

---

## v0.33.0 — 开发中

> **版本**: v0.33.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 系统设置界面默认值优化
> **说明**: 0.33.0 系列为 Web UI 系统设置界面增加默认值展示与差异标记：所有参数的值和选项的 tips 信息里显示参数的默认值；当参数当前值不等于默认值时，在值的右边显示红色 * 标记，让用户快速知道哪些值当前是修改过默认值的。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-470 | 0.33.0 | P1 | Web UI 系统设置界面默认值优化：tips 显示默认值 + 当前值≠默认值时红色 * 标记 |

> 当前 BUILD: 781
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-470 Web UI 系统设置界面默认值优化** [BUILD-789]
  - 背景：Web UI 系统设置界面（settings_get 返回的分组设置项）只显示参数当前值，用户无法知道系统默认值是什么，也无法快速识别哪些参数被修改过默认值。
  - 方案（已确认）：① 后端 `WebSettingItem` 新增 `Default` 字段（默认值），`SettingsJSON()` 为每个设置项填充默认值（来自 `config.DefaultConfig()` 与 normalize 函数）；② 前端 `renderSettingItem` 在 label 的 title（tips）中追加"默认值: xxx"；③ 当 `it.value !== it.default` 时，在控件右侧显示红色 * 标记。
  - 实施：`cmd/settings_web.go`（WebSettingItem 加 Default 字段 + SettingsJSON 填充默认值）+ `web/static/app.js`（renderSettingItem 读取 it.default：tips 追加默认值 + 值≠默认值时控件右侧加红色 * 标记）+ `web/static/style.css`（.set-diff 红色 * 标记样式）
  - 测试：见 use-case/FEATURE-470/

---

## v0.32.0 — 开发中

> **版本**: v0.32.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 消息附件（剪贴板图片粘贴 + 文件上传）
> **说明**: 0.32.0 系列为 Web UI 主消息录入框增加附件能力：支持从剪贴板粘贴图片（Ctrl+V）、普通文件上传，提交前以缩略图/文件图标形式平铺预览并可删除/放大预览；提交时将文件上传到工作区 input 目录（默认位置可经系统参数设置），图片经既有 attachments 通道注入视觉模型。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-469 | 0.32.0 | P1 | Web UI 消息附件：主录入框 Ctrl+V 粘贴剪贴板图片生成缩略图（置于录入框下附件区），支持文件选择上传与文件图标平铺、提交前删除/放大预览；提交时批量上传到工作区 input/ 目录（默认位置可用系统参数设置），图片附件走既有 attachments→SetImagePaths 视觉通道 |

> 当前 BUILD: 778
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-469 Web UI 消息附件（剪贴板图片粘贴 + 文件上传）**
  - 背景：Web UI 主消息录入框不支持粘贴/上传图片与文件。协议层已预留（input 消息 attachments 字段 → ReadLine → agent.SetImagePaths → 视觉模型），历史版本曾实现 📎 附件按钮后被简化移除；本任务以剪贴板图片粘贴为入口重新提供更完整的附件体验。
  - 方案（已确认）：① 录入框 Ctrl+V：剪贴板含图片时生成缩略图加入录入框下方附件区；② 📎 文件选择按钮支持多选普通文件，与图片缩略图并排平铺（文件显示图标+文件名）；③ 每个待发附件提供删除（✕）与点击放大预览（图片走本地预览弹层，文件显示基本信息）；④ 发送消息时先批量 POST /api/upload?dir=<inputDir> 上传全部待发附件到工作区 input/ 目录（默认 input，可经系统参数 web-input-dir 修改，配置变更即时生效），再以图片路径作为 input 消息 attachments 发出（既有视觉通道）；⑤ 普通文件仅上传到工作区 input/ 目录并在附件区/消息回显可见，不进入视觉通道。
  - 实施：`config/config.go`（Config.WebInputDir 字段）+ `cmd/settings.go`（:set web-input-dir 注册，非法路径拒绝）+ `cmd/settings_web.go`（开发者组设置项 + webInputDirValue 默认 input）+ `i18n/keys.go`/`en.go`/`zh.go`（KeyCol3WebInputDir）+ `web/server.go`（handleUpload 目标目录不存在时自动创建）+ `web/static/index.html`（录入框 📎 按钮 + 下方附件托盘 #attachBar/#attachList/#attachClear/#attachFile）+ `web/static/style.css`（.attach-* 缩略图/文件图标/删除/清空 + .user-dyn 动态标签 chips）+ `web/static/app.js`（paste 剪贴板图片、📎 多选、拖放加入附件托盘；缩略图与📄文件图标平铺；单项删除/清空/点击放大预览；发送时 FormData 批量上传至 web-input-dir → 图片走既有 attachments→SetImagePaths 视觉通道、全部文件以 <<<DYNAMIC>>> 动态感知标签追加 user 消息末尾；renderUserBody 将动态块渲染为气泡内标签区，回显即见；历史回看会话时 user 消息回放渲染沿用现有路径（原始动态块文本可读），与回显同渲染列为后续小任务）[BUILD-779]；补充：图片附件按主模型视觉能力门控——`agent/agent.go` 新增 MainModelSupportsVision()（GetActiveModel(false) 判断主模型 Capabilities.Vision），`web/session.go` ReadLine 仅当主模型支持视觉才 SetImagePaths 注入图片字节，否则只保留动态信息文本告知（避免把图片发给无视觉主模型）[BUILD-780]
  - 测试：见 use-case/FEATURE-469/（UC-0001~0004 运行时用例）；单测：`cmd/web_input_dir_test.go`（默认值 input/非法路径拒绝/设置组含 web-input-dir）+ `web/server_test.go` TestUploadAutoCreatesDir（上传目录自动创建）

---

## v0.31.0 — 开发中

> **版本**: v0.31.0

> **状态**: 🚧 开发中
> **里程碑**: Responses API 支持
> **说明**: 0.31.0 系列引入 Responses API 支持：让 co-shell 支持 OpenAI Responses API（/v1/responses），通过 `reasoning: {effort: "none"}` 控制思考开关（解决 qwen3.6 等模型在 Chat Completions 下无法关闭思考的问题），并支持按模型选择 API 类型。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-468 | 0.31.0 | P1 | Responses API 支持：新增 llm/responsesClient 实现 Client 接口（支持 LM Studio/DeepSeek 官方/本地代理三个端点）+ config 增加 api_type 字段让用户按模型选择 API（chat 默认 / responses）+ 模型向导支持 api_type 选择 |

- [x] **FEATURE-468 Responses API 支持（已合并，v0.31.0）**
  - 背景：qwen3.6-35b (uncensored) 等模型在 Chat Completions API 下无法通过任何请求参数关闭思考（enable_thinking/reasoning_effort 均无效），但 Responses API（/v1/responses）的 `reasoning: {effort: "none"}` 能完全关闭思考。已实测验证 LM Studio（127.0.0.1:11234）、DeepSeek 官方（api.deepseek.com）、本地代理（localhost:11535）三个端点均支持 /responses 端点且格式基本一致。
  - 方案（已确认）：新增 `llm/responses_client.go` 实现 `llm.Client` 接口（请求/响应/流式/工具调用转换，支持 `reasoning: {effort}` 思考控制）+ config `ModelConfig` 增加 `api_type` 字段（"chat" 默认 / "responses"）+ `NewClient` 根据 api_type 分发 + 模型向导支持 api_type 选择。
  - 实施：`config/model_template.go`（ModelConfig 增加 APIType 字段）+ `llm/responses_client.go`（responsesClient 实现 Client 接口：请求转换 Message→input、工具定义转换、响应解析 output[]、流式事件解析、reasoning 思考控制）+ `llm/client.go`（NewClient 根据 api_type 分发）+ `main.go`/`cmd/settings.go`/`agent/agent.go`（NewClient 调用处传入 api_type）+ `cmd/model_web_wizard.go`/`web/static/app.js`（模型向导支持 api_type 选择）+ `llm/responses_client_test.go`（请求/响应/流式/工具调用转换测试）
  - 测试：见 use-case/FEATURE-468/（开发中已验证：llm/responses_client_test.go 9 项 + cmd/model_web_wizard_test.go api_type 用例全绿；TestStreamSupReply 为存量失败（HEAD 亦失败），与本次改动无关）
  - 状态：核心实现完成（responses_client + 分发 + 向导），端到端验证（UC-0009/0010）待真实端点
  - 进度：[BUILD-772]；[BUILD-773] api_type 下拉优化；[BUILD-774] 错误诊断落盘；[BUILD-775] 修复 input_text 缺 text 键 invalid_union（ContentParts 合并 + text 键常出）；[BUILD-776] 修复 assistant 消息 part 需用 output_text（实测 LM Studio input_text 400 / output_text 200）；[BUILD-777] 修复流式工具调用 arguments 丢失——LM Studio 不发 arguments delta、完整参数在 output_item.done.item.arguments，改为按 output_index 累积 + output_item.done 权威补全；同时 reasoning.effort 值域归一（max→xhigh、default→未设，适配 LM Studio 枚举 none/minimal/low/medium/high/xhigh）

> 当前 BUILD: 777
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

---

## v0.30.0 — 开发中

> **版本**: v0.30.0

> **状态**: 🚧 开发中
> **里程碑**: 元能力感知（Meta-Capability Awareness）
> **说明**: 0.30.0 系列引入元能力感知机制：让 co-shell 感知自身隐藏的元能力（自我改造、模型调度、问题解决策略、分身协作、上下文管理等），通过内置知识库 + introspect_capability 工具 + CAPABILITIES 索引节 + 元能力感知开关实现。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-466 | 0.30.0 | P1 | 元能力感知：内置元能力知识库（i18n 多语言资源，每个能力有稳定唯一 ID/分类/名称/简介/完整说明）+ introspect_capability 工具（按 ID 精确查询 / 按关键字数组模糊搜索 / 返回完整索引）+ CAPABILITIES 索引节（开关控制是否注入元能力清单）+ 元能力感知开关 meta-capability-enabled |
| FEATURE-467 | 0.30.0 | P1 | Web UI 模型设置向导第一步（选择模板）改进：实时显示不同模板的思考相关开关选项并可设置（先显示 thinking 开关，开启后按 provider 显示对应 reasoning_effort 选项）+ 空白处显示模板原始 JSON（默认可收起隐藏，需要时展开）+ 补充 reasoning_effort 模板设置 |

- [ ] **FEATURE-466 元能力感知（Meta-Capability Awareness）**
  - 背景：co-shell 主要依靠策略文件注入上下文的方式感知世界和自身能力，但元能力（自我改造 .rules/、模型调度、问题解决策略、分身协作、上下文管理等）没有以能力形式出现在上下文中，LLM 不知道自己可以这么做，导致有些事能做却因不知而走弯路。
  - 方案（已确认）：内置元能力知识库（i18n 多语言资源，每个能力有稳定唯一 ID 不随语言变化、分类、名称、简介、完整说明）+ 新增 introspect_capability 工具（按 ID 精确查询 / 按关键字数组多条件模糊搜索 / 返回完整索引）+ CAPABILITIES 增加元能力索引节（开关控制是否注入）+ 新增元能力感知开关 meta-capability-enabled（便于观察功能效果）。
  - 实施：`config/config.go`（LLMConfig 加 MetaCapabilityEnabled 字段 + DefaultConfig 默认值）+ `agent/loop.go`（Agent 加 metaCapabilityEnabled 字段 + Setter）+ `main.go`（初始化开关 + 版本号 0.30.0）+ `agent/tools.go`（buildToolsInternal 注册 introspect_capability 工具）+ `agent/capability.go`（元能力知识库查询逻辑：按 ID 查询 / 关键字模糊搜索 / 返回索引 + introspectCapabilityTool 回调 + argStringSlice 辅助）+ `agent/system_prompt.go`（Capabilities case 开关开启时注入元能力索引节）+ `i18n/keys.go`（新增元能力资源 key）+ `i18n/en_system.go`/`i18n/zh_system.go`（元能力知识库多语言资源）+ `cmd/config.go`/`cmd/settings_agent.go`/`agent/settings_tools.go`（meta-capability-enabled 参数支持）+ `agent/capability_test.go`（元能力索引/ID 查询/关键字搜索/工具回调/注入测试）[BUILD-769]
  - 测试：见 use-case/FEATURE-466/

- [ ] **FEATURE-467 Web UI 模型设置向导思考设置改进**
  - 背景：Web UI 模型设置向导第一步（选择模板）目前只显示模板下拉框，无法在选模板时查看/设置该模板的思考相关开关（thinking、reasoning_effort）。不同 provider 的思考参数不同（qwen 用 enable_thinking、deepseek 用 thinking+reasoning_effort 等），且模板的 DefaultParams 中 reasoning_effort 未在向导中暴露。
  - 方案（已确认）：向导第一步选择模板时，实时显示该模板的思考相关开关选项并可设置：① 先显示 thinking 开关；② 开启后按 provider 显示对应的 reasoning_effort 选项；③ 在空白处显示模板原始 JSON 内容（默认可收起隐藏，需要时展开，保持透明）；④ 补充之前没处理的 reasoning_effort 模板设置。
  - 实施：`cmd/model_web_wizard.go`（WebWizardData 增加 Thinking/ReasoningEffort 字段 + template 步骤返回思考字段与模板 JSON + submit 保存到模型级 ThinkingEnabled/ReasoningEffort）+ `web/static/app.js`（template 步骤渲染 thinking 开关 + reasoning_effort 下拉 + 模板 JSON 展示）+ `web/static/style.css`（模板 JSON 展示样式）+ `i18n/keys.go`/`en.go`/`zh.go`（新增 reasoning_effort/模板 JSON 标签 key）+ `cmd/model_web_wizard_test.go`（template 思考字段/无 reasoning_effort/submit 保存测试）[BUILD-770]
  - 补充（BUILD-771）：新增 qwen3.8 模板（`config/model_template.go`，与 qwen 并列，ID=qwen3.8，模型 qwen3.8-27b，thinking 默认开）+ reasoning_effort 下拉对所有模板显示并含"不设置"（空值）选项（默认不设置，由用户决定）+ qwenThinkingAdapter 处理 ReasoningEffort（enable_thinking=true 且非空才传 reasoning_effort 顶层字段）+ 前端空值 option 显示"不设置"标签 + 测试更新（qwen reasoning_effort 含 xhigh / qwen3.8 模板测试）
  - 测试：见 use-case/FEATURE-467/

> 当前 BUILD: 768
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

---

## v0.29.0 — 开发中

> **版本**: v0.29.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 系统设置改进
> **说明**: 0.29.0 系列改进 Web UI 系统设置功能：修复右侧属性/值设置区高度基准问题，新增 MCP Server 设置区块。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-464 | 0.29.0 | P1 | Web UI 系统设置改进：① 右侧属性/值设置区高度以系统设置窗口高度为基准（而非浏览器窗口高度）；② 在"记忆与上下文"和"开发者"之间增加 MCP Server 设置区块，支持列表展示、追加、修改、删除 MCP server，可命名并设置地址参数等 |
| FIX-465 | 0.29.0 | P1 | 修复视觉模型选择 bug：getModelIDForCall() 在视觉识别时，当当前工作模式未绑定 VisionModelID 时直接返回该模式的 ModelID（可能不支持视觉），未检查视觉能力也未回退到全局视觉模型，导致视觉识别错误使用不支持视觉的模型 |

- [ ] **FIX-465 修复视觉模型选择 bug**
  - 背景：视觉识别时，当当前工作模式（如 act）未绑定 VisionModelID 时，getModelIDForCall() 直接返回该模式的 ModelID（可能不支持视觉），未检查视觉能力也未回退到全局视觉模型，导致视觉识别把图片发给不支持视觉的模型（如 deepseek-v4-flash），API 报 400。
  - 方案（已确认）：getModelIDForCall() 在 visionRequired 且模式未绑定 VisionModelID 时，检查模式 ModelID 是否支持视觉（modelSupportsVision）；若不支持则返回空字符串，让 selectModelForCall() 走全局回退路径（GetActiveModel(true) 正确选择全局最高优先级视觉模型）。
  - 实施：`agent/agent.go`（getModelIDForCall 增加视觉能力检查 + 新增 modelSupportsVision 辅助函数）+ `agent/fix465_test.go`（TestGetModelIDForCallVisionFallback / TestSelectModelForCallVisionFallback）[BUILD-764]；补充：`web/static/style.css`（settings-body 覆盖定义加 max-height:none，重置基础 max-height:60vh，修复缩小窗口后右半边底部结构性空白）[BUILD-765]；补充：`agent/tools.go`（injectMetaParam 修复 MCP 工具 required 字段类型断言 bug——MCP 工具 required 为 []string，原用 .([]interface{}) 断言失败导致必填字段丢失；改为 switch 处理 []interface{}/[]string 两种类型 + 去重 meta/instruct；buildTools 深拷贝 MCP 工具 InputSchema 避免修改共享引用导致重复 meta；新增 deepCopyMap/deepCopyValue 辅助函数）[BUILD-767]

> 当前 BUILD: 761
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-464 Web UI 系统设置改进**
  - 背景：Web UI 系统设置存在两个问题：① 右侧属性/值设置区高度以浏览器窗口高度为基准，会随浏览器大小变化；② 缺少 MCP Server 设置入口，用户无法在 Web UI 中管理 MCP server。
  - 方案（已确认）：① 修复右侧属性/值设置区高度基准，使其以系统设置窗口高度为基准（flex 子项加 min-height:0 使滚动生效）；② 在"记忆与上下文"和"开发者"之间新增 MCP Server 设置区块，默认显示 MCP server 列表，支持追加、修改、删除，可命名并设置地址参数等。
  - 实施：`web/static/style.css`（settings-pane 加 min-height:0 修复高度基准 + MCP manager 样式）+ `web/static/app.js`（MCP manager 渲染/增删改 + i18nT 辅助函数 + mcp/mcp_result 消息处理）+ `cmd/settings_web.go`（WebSettingGroup 加 Kind 字段 + 新增 MCP Server 组）+ `cmd/mcp.go`（MCPServersJSON/AddServerJSON/UpdateServerJSON/RemoveServerJSON）+ `web/session.go`（mcp 字段 + mcp_get/add/update/remove 处理）+ `web/server.go`（clientMessage/serverMessage 加 MCP 字段）+ `repl/session.go` + `repl/repl.go`（SessionDeps 加 MCPHandler）+ `i18n/`（KeyMCPUpdated/KeySettingsGroupMCP）[BUILD-762]；修复：MCP 列表为空——renderMCPServerManager 未发送 mcp_get 请求后端列表，且 renderMCPServers 回调会与 renderMCPServerManager 形成 mcp_get 无限循环；改为 renderMCPServerManager(fetch) 默认发送 mcp_get、renderMCPServers 以 fetch=false 调用避免循环 [BUILD-763]；改进：MCP 卡片界面——① 启用/禁用改为滑动开关（mcp-toggle），与标题、删除按钮在一行；② 命令单占一整行；③ 编辑框内去掉启用/禁用 checkbox；④ 去掉编辑按钮，改为点击卡片标题或命令文字进入编辑状态 [BUILD-766]
  - 测试：见 use-case/FEATURE-464/

---

## v0.28.1 — 开发中

> **版本**: v0.28.1

> **状态**: 🚧 开发中
> **里程碑**: SUP 块显示优化
> **说明**: 0.28.1 系列优化 SUP 块显示，为 SUP 块每部分内容（提示词/流式内容/工具输入）限制最高高度，参考 TOOL 块输入参数的做法，提供限高文本输出区域 + 展开 + RAW 选项。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-462 | 0.28.1 | P1 | SUP 块显示限高+展开+RAW：SUP 块每部分内容（提示词/流式内容/工具输入）限制最高高度，参考 TOOL 块输入参数的做法，提供限高文本输出区域 + 展开 + RAW 选项 |
| FIX-463 | 0.28.1 | P1 | gitStatusMap 加超时保护：git status 命令卡住时不再阻塞 /api/tree 文件树 API，超时返回 nil（不显示 git 状态但文件树正常显示） |

> 当前 BUILD: 755
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FIX-462 SUP 块显示限高+展开+RAW**
  - 背景：SUP 块（提示词/流式内容/工具输入三部分）每部分内容没有限定最高高度，可能显示过多内容。需要参考 TOOL 块输入参数的做法，提供限制高度的文本输出区域 + 展开 + RAW 选项。
  - 方案（已确认）：① SUP 块三部分（提示词/流式内容/工具输入）每部分标题栏增加 Raw 小胶囊开关 + 展开/收起按钮；② 每部分 body 限制最高高度（max-height 200px）可滚动，展开后取消限高；③ Raw 开关控制 md 渲染/原始文本切换；④ 流式内容部分（主 ev-body）同样限高 + 展开 + Raw。
  - 实施：`web/static/app.js` newSupBlock 改造为三部分均用 makeSupPart 创建（标题栏含 Raw + 展开按钮 + 限高 body），新增 makeSupPart/renderSupPart/addSupContentControls 辅助函数，content_chunk supervisor 分支 prompt/tool 部分改用 renderSupPart 渲染、content 部分支持 contentRawMode；`web/static/style.css` 新增 .sup-part-head flex 布局 + .sup-part-raw/.sup-part-toggle 按钮样式 + .sup-part-body 限高（200px）+ .sup-prompt.expanded/.sup-tool.expanded 展开 + .sup-content-body 限高 + .ev.sup-content-expanded 展开 + .sup-part-body.md markdown 样式 [BUILD-753]
  - 测试：见 use-case/FIX-462/

- [ ] **FIX-462 风险标签中文化 + 多方法意图回填**
  - 背景：风险标签 LOW/MEDIUM/HIGH 改为中文（低风险/中风险/高风险）并支持多语言；一个迭代调用两个方法时意图只写在后一个 TOOL 块，应倒序一个一个回填。
  - 实施：`web/static/app.js` 新增 riskLabel() 辅助函数 + iterToolBlocks 数组；tool_call_stream 新建工具块时 push 到 iterToolBlocks；tool_call 处理中意图回填优先按 iterToolBlocks 创建顺序匹配下一个未填充块（解决同名工具如 execute_command 时 toolBlockByName 被覆盖导致第一个块缺意图/风险标签），风险标签改用 riskLabel() [BUILD-758]
  - 测试：见 use-case/FIX-462/

- [ ] **FIX-462 修复报告确认框快捷键失效**
  - 背景：FIX-462 新增的"点击输入框取消快捷键监控"导致输入框获得焦点时立即设置 supplementMode=true，从而禁用所有快捷键（1-9/空格/回车等）。因输入框在交互弹框出现时常已获得焦点，快捷键全部失效。
  - 实施： 将 input 的 focus 监听改为 click 单击才取消快捷键监控（进入补充输入模式），自动聚焦/Tab 聚焦不触发，仅显式鼠标单击才禁用快捷键；同步更新 zh/en supplementHint 文案为"单击输入框" [BUILD-757]
  - 测试：见 use-case/FIX-462/

- [ ] **FIX-463 gitStatusMap 加超时保护**
  - 背景：mcp-sample 是 git 仓库，co-shell 的 gitStatusMap()（web/server.go:593-632）在 handleTree 中执行 `git status --porcelain -z`，但该命令在 mcp-sample 下卡住不返回（git 在 refresh_index 阶段对某个已跟踪文件执行 mmap 时挂起），且 cmd.Output() 没有超时保护，导致整个 /api/tree 请求永久挂起，Web UI 文件树为空。
  - 方案（已确认）：给 gitStatusMap 中的 git 命令设置超时（3 秒），超时则返回 nil（不显示 git 状态，但文件树正常显示）。这是健壮性缺陷——git 命令卡住不应阻塞文件树 API。
  - 实施：`web/server.go` gitStatusMap 用 context.WithTimeout 包裹 git 命令，超时返回 nil；新增 `web/gitstatus_timeout_test.go` 测试超时场景 [BUILD-761]
  - 测试：见 use-case/FIX-463/

---

## v0.28.0 — 开发中

> **版本**: v0.28.0

> **状态**: 🚧 开发中
> **里程碑**: 问题解决/循环判定/监督 LLM 交互流式暴露
> **说明**: 0.28.0 系列将问题解决、循环判定、监督三个场景与 LLM 的交互内容（发送的 prompt + 流式回复）以流式方式暴露给前端，使用独立的 SUP 块显示，并新增两个开关分别控制是否输出 prompt 与流式回复。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-460 | 0.28.0 | P1 | 问题解决/循环判定/监督 LLM 交互流式暴露：三个场景（问题解决 problem_solver / 循环判定 loop_detector / 监督 supervisor）与 LLM 的交互内容（发送的 prompt + 流式回复）以流式方式暴露给前端，使用独立的 SUP 块显示（SUP·问题解决 / SUP·循环判定 / SUP·监督）；新增两个开关 show-sup-prompt（是否显示发送给 LLM 的 prompt）与 show-sup-stream（是否流式显示 LLM 回复）分别控制是否输出；三个场景从非流式 Chat() 改为流式 ChatStream()，基于流式累积结果做结构化解析（report_problem / submit_review） |

> 当前 BUILD: 747
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-460 问题解决/循环判定/监督 LLM 交互流式暴露**
  - 背景：问题解决、循环判定、监督三个场景与 LLM 的交互目前是同步阻塞调用（非流式），用户无法在前端看到 LLM 的思考/输出过程，只能看到最终结果。希望将这三个场景的 LLM 交互内容（发送的 prompt + 流式回复）以流式方式暴露给前端，使用独立的 SUP 块显示，并新增两个开关分别控制是否输出 prompt 与流式回复。
  - 方案（已确认）：① 新增两个开关 show-sup-prompt（是否显示发送给 LLM 的 prompt）与 show-sup-stream（是否流式显示 LLM 回复），默认关闭，放在 [安全与确认] 组，支持 REPL `.set` + Web UI 设置 + LLM 工具设置；② 将 callProblemSolver（问题解决/循环判定共用）与 callSupervisor（监督）从非流式 Chat() 改为流式 ChatStream()，累积流式内容，同时基于累积结果做结构化解析（report_problem / submit_review）；③ 三个场景分别用 SUP·问题解决 / SUP·循环判定 / SUP·监督 标题的 SUP 块显示；④ 前端新增流式 SUP 块渲染（复用现有 content_chunk 流式机制，归到 supervisor 通道并带场景标题）。
  - 实施：`config/config.go` SupervisorConfig 新增 ShowSupPrompt/ShowSupStream 两个开关（默认 false）+ 默认配置；`agent/sup_stream.go` 新增 SupScenario 类型（problem_solver/loop_judge/supervisor）+ supTitle 标题映射（SUP·问题解决/SUP·循环判定/SUP·监督）+ supPromptEnabled/supStreamEnabled 开关读取 + emitSupPrompt（show-sup-prompt 开启时发 content_chunk 事件到 supervisor 通道带 sup_scenario meta）+ streamSupReply（show-sup-stream 开启时流式转发 content_chunk 到前端，累积内容与工具调用供结构化解析）；`agent/events.go` 新增 MetaKeySupScenario 常量；`agent/problem_solver.go` callProblemSolver 改为流式 ChatStream（新增 scenario 参数，emitSupPrompt + streamSupReply 累积后解析 report_problem），solveProblem 传 SupScenarioProblemSolver；`agent/loop.go` judgeLoop 传 SupScenarioLoopJudge；`agent/supervisor.go` callSupervisor 改为流式 ChatStream（emitSupPrompt + streamSupReply 累积后解析 submit_review）；设置入口：`cmd/settings.go` 分发 + showSettingsHelp safetyGroup 显示 + `cmd/settings_safety.go` handleSafetySetting/showSupervisorBoolSetting 处理（show-sup-prompt/show-sup-stream）+ `cmd/settings_web.go` safetyGroup 显示（Web UI 设置）+ `agent/settings_tools.go` getSettingValue/applySetting（LLM 工具设置）；i18n 新增 KeyCol3ShowSupPrompt/KeyCol3ShowSupStream（zh/en）；前端 `web/static/app.js` 新增 curSup 流式 SUP 块变量 + content_chunk 处理 supervisor 通道分支（按 sup_scenario 标题创建 SUP 块并流式累积）+ isStreamingBody 包含 curSup + 三处重置点重置 curSup；`web/static/style.css` 复用现有 .ev.supervisor 样式；单元测试：`agent/sup_stream_test.go`（6 个用例：supTitle 标题映射、supPromptEnabled/supStreamEnabled 开关、emitSupPrompt 事件/开关/空 prompt、streamSupReply 累积+转发+工具调用、streamSupReply 不转发、streamSupReply 错误传播）+ `agent/supervisor_test.go` 更新 TestGetSettingValue_Supervisor/TestApplySetting_SupervisorBooleans 覆盖两个新开关 [BUILD-749]
  - 测试：见 use-case/FEATURE-460/

- [ ] **FEATURE-461 attempt_completion 完成报告信息框改进**
  - 背景：attempt_completion 的 completion-confirm 对话框（完成报告信息框）布局与交互需要改进：报告标题应作为加粗标题移到内容框外、补充信息录入框、选项+补充信息组合发送等。
  - 方案（已确认）：① "任务结果已就绪，请选择下一步"标题保持在选项按钮框外/上方；② 【任务完成报告】提取为加粗标题"任务完成报告"（去括号）渲染在内容块外上方；③ 【监督 LLM 审查】改为"审查报告"标题渲染在内容块外；④ "根据合理推理给出下一步建议"选项改为"给出下一步建议"；⑤ 用户选择区下方新增"补充信息："录入框，点击后取消快捷键监控；⑥ 用户输入补充信息后可点击选项按钮，以【选项内容 + "，" + 补充信息】作为提示词发给 LLM（"我已确认完成（退出）"直接退出不发送，"补充信息"按钮直接发送用户输入）；⑦ 报告提示框最大高度提升到 UI 的 80%。
  - 实施：`web/static/app.js` splitReportSections 改为返回 {title, content}（【任务完成报告】→"任务完成报告"、【监督 LLM 审查】→"审查报告"）+ showInteraction 渲染 report-title 加粗标题 + 新增 supplementInput 变量与 getSupplement/sendSupplement/answerSelectWithSupplement 辅助函数 + 用户选择区下方新增 .interaction-supplement 补充信息录入框（聚焦取消快捷键监控）+ 选项按钮点击附加补充信息（answerSelectWithSupplement）+ exit 键直接 hideAsk 退出不发送 + 补充信息按钮 sendSupplement 直接发送；`web/static/style.css` 新增 .report-title/.interaction-supplement 样式 + .report-block max-height 40%→80%；`i18n/zh.go`/`i18n/en.go` KeyAttemptCompletionSuggestNext 改为"给出下一步建议"/"Give next-step suggestions" [BUILD-750]
  - 测试：见 use-case/FEATURE-461/

---

## v0.27.0 — 开发中

> **版本**: v0.27.0

> **状态**: 🚧 开发中
> **里程碑**: attempt_completion 对话框显示监督信息
> **说明**: 0.27.0 系列实现 attempt_completion 的 completion-confirm 对话框显示监督 LLM 的结论/理由/建议，为人工审核提供强有力支持。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-459 | 0.27.0 | P1 | attempt_completion 对话框显示监督信息：调用 attempt_completion 时，连同监督 LLM 返回的（如果有）结论、理由、建议，都在信息提示选择框的信息提示部分全部显示出来，为人工审核提供强有力支持 |

> 当前 BUILD: 741
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-459 attempt_completion 对话框显示监督信息**
  - 背景：任务完成时，attempt_completion 的 completion-confirm 对话框只显示标题和选项，人工判断选择的那块信息过于简单，用户无法方便地看到信息的全局。监督 LLM 返回的结论/理由/建议（如果有）没有在对话框中显示，人工审核缺乏支持。
  - 方案（已确认）：① attempt_completion 的 completion-confirm 对话框（Interaction）的 Body 字段中，显示监督 LLM 返回的结论/理由/建议（即 report 内容）；② 若监督 LLM 未启用或未返回 report，则不显示该部分；③ 信息提示部分（interaction-body）渲染为 markdown，为人工审核提供强有力支持。
  - 实施：`agent/tools.go` attemptCompletionTool 中，将监督 LLM 放行时返回的 report（formatReviewAsText 生成的结论/理由/建议）设置到 completion-confirm 对话框 Interaction 的 Body 字段（report 为空时不显示，符合 omitempty）[BUILD-742]；增强：对话框 Body 显示主 LLM 调用 attempt_completion 时的主要内容（result，前缀【任务完成报告】）+ 监督 LLM 审查内容（report，前缀【监督 LLM 审查】）；`agent/supervisor.go` runSupervisorReview 监督调用失败时返回失败信息作为 report（结论：审查失败 ⚠️ + 原因 + 错误），降级放行但向用户报告失败 [BUILD-743]；前端优化：`web/static/app.js` showInteraction 新增 splitReportSections 函数（按【任务完成报告】/【监督 LLM 审查】标记拆分 Body 为多个独立信息块）+ 键盘事件处理新增长按检测（按住快捷键 >=500ms 时把选项内容填入主信息录入框并进入补充模式，松开前正常选择）+ hideAsk 清理 __vkKeyup 监听器；`web/static/style.css` 新增 .report-block 样式（max-height:40% + overflow-y:auto + 边框圆角背景）[BUILD-744]；对话框交互优化：点击【+：任务尚未达到目标】改为把“任务尚未达到目标：”填入主消息框并退出对话框（保持 interaction pending，用户补充后发送回后端继续循环），`web/static/app.js` 新增 fillInputAndExit 函数并同步处理物理 + 键点击/长按/keyup；i18n 文本调整：KeyAttemptCompletionSuggestNext 改为“根据合理推理给出下一步建议”（en: Give next-step suggestions based on reasonable reasoning）、KeyAttemptCompletionExit 改为“我已确认完成（退出）”（en: I have confirmed completion (exit)）[BUILD-746]
  - 测试：见 use-case/FEATURE-459/（9 个用例：监督放行时对话框显示监督结论/理由/建议、监督未启用/未返回 report 时不显示、信息提示部分渲染为 markdown、对话框选项仍正常显示、用户选择完成退出后任务正常完成、选择其他选项后继续循环）；单元测试：`agent/attempt_completion_test.go` 新增 TestAttemptCompletionBodyShowsSupervisorReport（监督强制放行时 Body 包含主 LLM 报告 + 监督信息）+ TestAttemptCompletionBodyShowsMainReportWithoutSupervisor（监督未启用时 Body 显示主 LLM 报告但不含监督信息）[BUILD-742]；`agent/supervisor_test.go` 更新 TestRunSupervisorReview_CallFailure（监督调用失败时 report 包含失败信息）[BUILD-743]

---

## v0.26.0 — 开发中

> **版本**: v0.26.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 连接状态开关
> **说明**: 0.26.0 系列实现 Web UI 右上角连接状态显示改为连接/断开开关，通过指示灯圆点指示连接状态，并增加手动重连策略（断开后不自动连接，等用户手动点击再连接），避免两个浏览器访问一个服务互抢连接。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-458 | 0.26.0 | P1 | Web UI 连接状态开关：右上角连接状态显示改为连接/断开动作的开关（点击可连接/断开），通过已连接/已断开左边的指示灯圆点指示当前连接状态；增加连接策略：断开后不自动连接，等用户手动点击再连接，避免两个浏览器访问一个服务互抢连接 |

> 当前 BUILD: 739
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-458 Web UI 连接状态开关**
  - 背景：Web UI 右上角的连接状态目前只是纯显示（已连接/已断开），无法主动控制连接。且断开后会自动重连（setTimeout 2 秒），当两个浏览器同时访问一个服务时，会互抢 WebSocket 单客户端连接，导致连接不稳定。
  - 方案（已确认）：① 右上角连接状态显示改为可点击的开关（点击可连接/断开）；② 通过已连接/已断开左边的指示灯圆点（connDot）指示当前连接状态（已连接高亮、已断开灰色）；③ 增加连接策略：断开后不自动连接，等用户手动点击再连接，避免两个浏览器互抢连接。
  - 实施：`web/static/app.js` 修改 wsConnect 逻辑（移除 onclose 中的 setTimeout 自动重连，改为手动重连 + 防重入检查）+ conn 元素添加 onclick 事件（已连接时点击断开、已断开时点击连接）+ 新增 wsDisconnect 函数（主动关闭 WebSocket）+ 初始化时自动连接一次；`web/static/index.html` conn 元素添加 role="button" + tabindex="0"（可点击、可键盘操作）；`web/static/style.css` conn 添加外框（高度 28px 与明暗按钮一致、宽度自适应容纳指示灯+文字、边框、圆角、cursor:pointer、hover 效果）[BUILD-740]
  - 测试：见 use-case/FEATURE-458/（13 个用例：连接状态可点击开关、指示灯高亮/灰色、点击断开、断开不自动重连、点击重连、异常断开不自动重连、两浏览器互抢缓解、断开后输入框不可用、重连后功能恢复、外框存在、外框宽度自适应、外框与明暗按钮对齐）

---

## v0.25.0 — 开发中

> **版本**: v0.25.0

> **状态**: 🚧 开发中
> **里程碑**: 专职监督 LLM 交付复核
> **说明**: 0.25.0 系列实现专职监督 LLM，对主 LLM 的交付物进行独立复核，自动打回继续或重做，实现全自动化交付门禁。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-456 | 0.25.0 | P1 | 专职监督 LLM 交付复核：新增独立监督 LLM（复用问题解决 LLM 模型），以审查主 LLM 交付物是否达到用户终极目标为核心目标；监督 LLM 上下文与会话绑定独立累积（可配置清空）；三个介入点（A attempt_completion / B 未调用工具自动退出 / C 任务进度标记完成）由三个开关控制（A/B 默认开、C 默认关）；监督 LLM 只允许低风险工具（显式白名单可配置）；判定时给监督 LLM 用户历次消息增量 + taskplan description + 主 LLM 最后报告 + 场景入口信息 + 任务进度清单；完成审查工具收集是否放行/理由/建议（必填）；不放行则理由+建议作为 user 消息进主 LLM 上下文重跑，放行则理由+建议向用户报告并进记忆；防死循环最大打回 20 次（可配），超限交人工判定并报告次数 |
| FEATURE-457 | 0.25.0 | P2 | Web UI 设置页分组折叠+搜索：设置页 5 大分组（LLM/Agent/显示输出/安全确认/记忆上下文）支持折叠/展开（默认折叠，点击组标题展开），设置页顶部加搜索框实时过滤（输入关键词只显示参数名/描述匹配的设置项，匹配的分组自动展开），减少设置项过多带来的视觉噪音，提升用户体验 |

> 当前 BUILD: 738
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-456 专职监督 LLM 交付复核**
  - 背景：长任务完成后经常出现遗漏，需人工检查确认，但人工实际做不了什么。用同一套上下文（同一个人）自检容易漏，需要一个专职监督 LLM 专门做交付复核，自动打回继续或重做，实现全自动化。
  - 方案（已确认）：① 新增监督 LLM（复用问题解决 LLM 模型），启用/禁用开关默认启用；② 监督 LLM 上下文与会话绑定独立累积（清空开关放[安全与确认]）；③ 三个介入点 A/B/C 由三个开关控制（A 默认开、B 默认开、C 默认关）；④ 监督 LLM 只允许低风险工具（显式白名单可配置：read_file/execute_command/memory_search 等）；⑤ 判定输入：用户历次消息增量 + taskplan description + 主 LLM 最后报告 + 场景入口信息 + 任务进度清单；⑥ 完成审查工具收集是否放行/理由/建议（必填）；⑦ 不放行则理由+建议作为 user 消息进主 LLM 上下文重跑，放行则理由+建议向用户报告并进记忆；⑧ 防死循环最大打回 20 次（可配），超限交人工判定并报告次数；⑨ 同步阻塞式审查。
  - 实施：新增 `agent/supervisor.go`（监督 LLM 核心：supervisorContext 会话绑定独立累积上下文 + supervisorState 运行时状态 + SupervisorReview 审查结果结构 + submitReviewTool 完成审查工具（approved/reason/suggestion 三字段必填）+ callSupervisor 多轮工具调用循环（白名单方案 B：白名单内工具执行、白名单外自动拒绝）+ buildSupervisorTools/findToolDefinition 工具集构建 + runSupervisorReview 同步审查入口（放行/打回/防死循环 maxRetries 默认 20 超限强制放行并报告次数 + 放行时理由+建议进记忆））；`config/config.go` 新增 `SupervisorConfig`（Enabled 默认 true / EntryObject 默认 true / EntryExit 默认 true / EntryTask 默认 false / ClearContext 默认 false / MaxRetries 默认 20 / AllowedTools 白名单）+ LLMConfig.Supervisor 字段 + 默认配置；`agent/loop.go` Agent 结构体新增 supervisorState 字段 + `agent/agent.go` New() 初始化；三个介入点挂载：A 在 `agent/tools.go` attemptCompletionTool（completion-confirm 前，打回则 feedback 作为 user 消息返回重跑）、B 在 `agent/run_stream.go` Rule 3（attempt_completion 不可用直接退出）和 Rule 2 noToolAction=exit（打回则 feedback 作为 user 消息 continue）、C 在 `agent/taskplan_tools.go` trackTaskProgressTool（hasCompletedStep 检测有步骤标记完成时触发）；`i18n/` 新增 KeySupervisorSystemPrompt（en/zh）+ keys.go 定义 [BUILD-725]
  - 测试：见 use-case/FEATURE-456/；单元测试：`agent/supervisor_test.go`（20 个用例：parseSupervisorReview 放行/打回/JSON 容错、supervisorToolAllowed 白名单默认/自定义、hasCompletedStep 完成检测、formatReviewAsText/formatReviewFeedback 格式化、supervisorMaxRetries 默认 20/可配、supervisorEntryEnabled 三开关默认值、runSupervisorReview 禁用/入口关闭/超限强制放行/调用失败降级、getSettingValue 读取 supervisor 参数开关、applySetting 设置 supervisor 布尔开关/max-retries 校验/allowed-tools 逗号分隔/非法布尔拒绝）[BUILD-728]；设置入口：`cmd/settings.go` handleSafetySetting 分发 + showSettingsHelp safetyGroup 显示 + `cmd/settings_safety.go` handleSafetySetting 处理（supervisor-enabled/entry-object/entry-exit/entry-task/clear-context/max-retries/allowed-tools）+ `cmd/settings_web.go` safetyGroup 显示（Web UI 设置）+ `agent/settings_tools.go` getSettingValue/applySetting（LLM 工具设置）[BUILD-730]

- [ ] **FEATURE-457 Web UI 设置页分组折叠+搜索**
  - 背景：系统设置项非常多（LLM/Agent/显示输出/安全确认/记忆上下文 5 大组，每组几十个参数），Web UI 设置页一次性全部平铺展示，视觉噪音大，用户难以快速找到目标参数。
  - 方案（已确认）：① Web UI 设置页 5 大分组支持折叠/展开（默认折叠，点击组标题展开）；② 设置页顶部加搜索框，输入关键词实时过滤，只显示参数名/描述匹配的设置项，匹配的分组自动展开；③ 本次只做 Web UI 端，REPL `.set` 端不做。
  - 实施：`web/static/index.html` 设置页顶部添加搜索框（settingsSearch）；`web/static/app.js` 重写 renderSettings 实现分组折叠（每个组包在 .set-group 容器中，组标题可点击展开/折叠，默认折叠，箭头 ▸/▾ 指示）+ 新增 applySettingsFilter 搜索过滤（匹配 key/desc/组标题，匹配分组自动展开）+ 声明 settingsSearch 变量 + settingsSearch input 事件监听（清空恢复默认折叠）；`web/static/style.css` 添加 .set-group/.set-group-body/.set-search 折叠与搜索样式 [BUILD-731]；iPad 风格布局：`web/static/index.html` settings modal 重构为左右两栏（左侧 settingsNav 分类导航栏 + 右侧 settings-pane 内容区，modal 固定 600x800）；`web/static/app.js` 重写 renderSettings 为 iPad 风格（settingsGroups/settingsActiveGroup 状态 + settingsGroupIcon 分类图标映射 + renderSettingsNav 渲染左侧分类导航（每个分类配图标）+ selectSettingsGroup 点击切换 + renderSettingsPane 渲染右侧选中分类设置项 + applySettingsFilter 在选中分类内过滤）+ 声明 settingsNav 变量；`web/static/style.css` 添加 .settings-box（加宽）/ .settings-nav / .settings-nav-item（图标+文字，选中高亮）/ .settings-pane 左右两栏布局样式 [BUILD-733]；窗口固定 600x800：`web/static/style.css` .settings-box 改为 width:600px/height:800px（max-width:92vw/max-height:90vh 小屏保护），.settings-body 固定高度 calc(100%-41px) + overflow:hidden，.settings-nav/.settings-pane overflow-y:auto 自动滚动条 [BUILD-734]；分组调整：`i18n/zh.go`/`i18n/en.go` 所有分组标题去掉 [] 方括号 + KeySettingsGroupDisplay 改名（zh: 显示与输出→外观与显示，en: Display & Output→Appearance & Display）；`cmd/settings_web.go` displayGroup 添加 theme-mode 设置项（前端本地主题，Type enum auto/dark/light）；`web/static/index.html` 移除静态外观组（主题选择器）；`web/static/app.js` 删除 setThemeMode 声明与 onchange 块 + renderSettingItem 对 theme-mode 特殊处理（渲染为主题选择器，onchange 保存 localStorage + applyTheme）[BUILD-735]；分组高亮调整：`web/static/style.css` .settings-nav-item 添加 border:1px solid transparent，.settings-nav-item.active 由绿色实心背景+白字改为高亮边框（border-color:var(--accent)）+ 低反差背景（background:var(--accent-dim)），参考模型设置 .model-row.selected 高亮方式，背景与字体颜色反差小、主要靠高亮边框 [BUILD-736]；监督输出独立通道：`agent/out.go` 新增 ChannelSupervisor 通道常量；`agent/supervisor.go` 新增 emitSupervisorReport 方法（report 非空时经 streamCb 发 InfoEvent(ChannelSupervisor, report)，否则回退 defaultIO().ErrPrintf）；三个介入点 A（`agent/tools.go` attemptCompletionTool）/ B（`agent/run_stream.go` no-tool exit）/ C（`agent/taskplan_tools.go` trackTaskProgressTool）统一改为调用 emitSupervisorReport；`web/static/app.js` CHAN_LABEL 新增 supervisor:SUP + eventClass 对 ui_text chan=supervisor 返回 supervisor 类 + renderEvent 新增 supervisor 分支（SUP 块，按结论打回/放行加 supervisor-reject/supervisor-pass 类）；`web/static/style.css` 新增 .ev.supervisor 纯亮白字体 + 放行绿/打回红指示灯样式 [BUILD-737]；SUP 标题栏字体与指示灯颜色改为 #ff58f3，并显式区分 YOU 标题行（.ev.user-msg .ev-head 用主题 accent 色，与 SUP 的 #ff58f3 区分）[BUILD-738]
  - 测试：见 use-case/FEATURE-457/

---

## v0.24.0 — 开发中

> **版本**: v0.24.0

> **状态**: 🚧 开发中
> **里程碑**: 远程访问工作区文件下载功能
> **说明**: 0.24.0 系列实现远程访问 Web UI 时工作区文件列表的下载功能。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-455 | 0.24.0 | P1 | 远程访问时工作区文件列表定位功能变为下载图标：当用户通过远程访问 web ui（监听地址非 127.0.0.1/localhost）时，文件列表的"定位到文件夹"图标受控自动变为"下载"图标（文件夹的定位功能消失），点击下载图标可通过浏览器下载目标文件；下载功能通过命令行参数 --download-enabled 控制启用/禁用，禁用时完全不提供此功能，需防止非授权下载（复用 whitelist IP 授权 + 路径穿越校验） |

> 当前 BUILD: 724
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-455 远程访问时工作区文件列表定位功能变为下载图标** ✅ 已完成
  - 背景：Web UI 工作区文件列表的"定位到文件夹"（reveal）功能通过 OS 文件管理器在服务器本地定位文件。当用户通过远程访问（监听地址非 127.0.0.1/localhost，如 0.0.0.0 或局域网 IP）时，该功能无意义（无法在远程浏览器上打开服务器本地文件管理器）。希望远程访问时该图标受控自动变为"下载"图标，用户点击可通过浏览器下载目标文件。
  - 方案：① 新增 `--download-enabled` 命令行参数（默认禁用，安全优先），传入 `ServerOptions.DownloadEnabled`；② 后端新增 `/api/download` 路由，复用 `resolvePath` 路径穿越校验 + whitelist 中间件 IP 授权，仅当远程访问且下载启用时提供；③ bootstrap 下发 `remote`（是否远程访问）与 `downloadEnabled` 标志给前端；④ 前端：远程访问且下载启用时，文件列表 reveal 图标变为下载图标（文件夹的定位图标消失），点击触发浏览器下载。
  - 实施：`main.go` 新增 `--download-enabled` 参数（默认 false）+ `cliFlags.downloadEnabled` + `ServerOptions.DownloadEnabled` 传入；`web/server.go` 新增 `ServerOptions.DownloadEnabled` 字段、`isRemote()`（Bind 非 loopback 判定）、`downloadEnabled()`（DownloadEnabled && isRemote）、`handleDownload`（路径穿越校验 + 仅文件 + Content-Disposition attachment）、`/api/download` 路由、bootstrap 下发 `remote`/`downloadEnabled`；`web/static/app.js` 新增 `remoteAccess`/`downloadEnabled` 全局变量（boot 读取）、treeNode 渲染逻辑（远程+下载启用时文件变下载图标 ⬇、文件夹定位图标消失）、`downloadFile()` 函数、`T.downloadFile` 中英文文本；`usage.go` + `i18n`（zh/en/keys）新增 `--download-enabled` 帮助文本；`web/server_test.go` 新增 `TestIsRemote`/`TestDownload` 单元测试 + 修复 `TestBootstrap` 解码类型 [BUILD-722]；⑪ 修复：远程访问时定位功能始终禁用（无论是否开启下载）——`web/static/app.js` treeNode 渲染改为 `if (remoteAccess)` 分支（下载启用时文件显示下载图标，否则文件/文件夹均不显示任何图标），`web/server.go` `handleOpen`/`handleReveal` 在 `isRemote()` 时返回 403（防止直接调用 API 触发远程主机本地应用/文件管理器），`web/server_test.go` TestDownload 补充远程 open/reveal 403 验证，测试用例 UC-002 更新 [BUILD-723]
  - 测试：见 use-case/FEATURE-455/

---

## v0.23.1 — 开发中

> **版本**: v0.23.1

> **状态**: 🚧 开发中
> **里程碑**: Web UI attempt_completion 弹框 Keys 选项修复
> **说明**: 0.23.1 系列修复 Web UI 端 attempt_completion 完成确认弹框缺少 Keys 选项（"+ 任务尚未达到目标" / "- 完成退出"）的 bug。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-454 | 0.23.1 | P1 | Web UI attempt_completion 弹框缺 Keys 选项：web/static/app.js 的 renderVirtualKeyboard select 分支只渲染 it.options（next_steps + 给出下一步的建议）和固定补充信息选项，未渲染 it.keys（"+ 任务尚未达到目标" / "- 完成退出"），导致 Web UI 上 attempt_completion 弹框缺少这两个选项；TUI 端正确渲染 Keys |

> 当前 BUILD: 721
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FIX-454 Web UI attempt_completion 弹框缺 Keys 选项**
  - 背景：Web UI 端 attempt_completion 完成确认弹框只显示"1: 给出下一步建议"和"空格：补充信息"，缺少"+ 任务尚未达到目标"和"- 完成退出"两个 Keys 选项。
  - 根因：web/static/app.js 的 renderVirtualKeyboard select 分支只渲染 it.options 和固定补充信息选项，未渲染 it.keys。
  - 方案：在 renderVirtualKeyboard select 分支中渲染 it.keys（Keys 快捷键选项），并让物理按键（+/-）也能触发。 [BUILD-721]

---

## v0.23.0 — 开发中

> **版本**: v0.23.0

> **状态**: 🚧 开发中
> **里程碑**: skill 支持（Agent Skills 开放标准）
> **说明**: 0.23.0 系列专注 skill 支持：采用 Agent Skills 开放标准（SKILL.md + 目录，兼容 Claude Code/Cursor 生态），工作空间级 ./skills/ + 全局 ~/.co-shell/skills/ 合并展示，系统提示词只列 skill 索引按需加载，命令 :skill list/show/add/remove。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-453 | 0.23.0 | P1 | skill 支持：Agent Skills 开放标准（SKILL.md + 目录），工作空间级 ./skills/ + 全局 ~/.co-shell/skills/ 合并展示，系统提示词只列 skill 索引（name+description+路径）按需加载（LLM 用 read_file 读取 SKILL.md），命令 :skill list/show/add/remove（add 从本地路径复制，默认到工作空间 ./skills/，可指定 --global） |

> 当前 BUILD: 720
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-453 skill 支持（Agent Skills 开放标准）**
  - 背景：co-shell 缺少对 skill（可复用能力包）的支持，无法按需加载特定领域的操作指导。
  - 方案（已确认）：见 use-case/FEATURE-453/ [BUILD-720]

---

## v0.22.0 — 开发中

> **版本**: v0.22.0

> **状态**: 🚧 开发中
> **里程碑**: attempt_completion 下一步建议交互 + 交互选项统一
> **说明**: 0.22.0 系列专注任务收尾交互优化：attempt_completion 增加"下一步建议"交互选项（可配置开关），ask_followup_question 增加两个固定选项，统一 -/+ 快捷键语义，补全 meta 参数。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-452 | 0.22.0 | P1 | attempt_completion 增加"下一步建议"交互选项 + ask_followup_question 增加两个固定选项 + meta 参数补全：① 系统提示词明确 track_task_progress 定计划 + meta.progress 更新；② 补全工具调用示例 meta 参数；③ attempt_completion 加 meta 参数（track_task_progress 不加）；④ attempt_completion 增加"下一步建议"交互（可配置开关默认弹框、next_steps 可选、三个固定选项：给出下一步的建议/任务尚未达到目标(+)/完成退出(-)，前两个传回 LLM 继续循环，完成退出直接退出）；⑤ ask_followup_question 增加两个固定选项：我要再想想先退出(-，传回 LLM 让 LLM 自己退出)/还有其他选项或组合吗(+，传回 LLM) |

> 当前 BUILD: 715
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-452 attempt_completion 下一步建议交互 + 交互选项统一** [BUILD-716]
  - 背景：任务收尾时 attempt_completion 直接退出，用户无法在收尾时引导 LLM 继续或确认是否真正完成；ask_followup_question 的选项交互缺少"先退出"和"更多选项"的固定入口。
  - 方案（已确认）：见 use-case/FEATURE-452/
  - 实施：① `agent/tools.go` attempt_completion 工具定义加回 meta 参数（放最前，required 加 meta）+ 新增 next_steps 可选参数；② `agent/meta_param_test.go` 更新：attempt_completion 现在要求 meta，track_task_progress 仍不要求；③ `i18n/en_system.go`/`zh_system.go` KeyToolUsageAttemptCompletion 加 meta 参数声明与 XML 示例（含 next_steps）；④ `i18n/en_system.go`/`zh_system.go` KeySystemPromptToolUsageTaskProgress 明确"用 track_task_progress 建立初始计划，执行中用其他工具调用的 meta.progress 增量更新"；⑤ `config/config.go` LLMConfig 新增 AttemptCompletionConfirm 开关（默认 true）；⑥ `agent/interaction.go` askSelect 支持 Keys 快捷键解析（渲染 [Key] Label，输入匹配 Key 返回 ActionSelect+Value）；⑦ `agent/tools.go` attemptCompletionTool 重构：开关开启时弹 InteractionSelect（选项=next_steps+给出下一步的建议，Keys=[任务尚未达到目标(+), 完成退出(-)]），用户选"完成退出"才 SetCompleted，其他选择 storeUserReply 传回 LLM 继续循环（不 SetCompleted），开关关闭时直接退出；⑧ `agent/tools.go` askFollowupQuestionTool 增加两个固定选项（- 我要再想想先退出 / + 还有其他选项或组合吗），映射回用户可读文本传回 LLM；⑨ `i18n/keys.go`/`en.go`/`zh.go` 新增 KeyAttemptCompletion* 和 KeyAskFollowup* 键；⑩ 新增 `agent/attempt_completion_test.go` 测试弹框逻辑（exit/continue/not_done/disabled），`agent/interaction_test.go` 新增 askSelect Keys 快捷键与 ask_followup_question 固定选项测试 [BUILD-716]；⑪ meta.progress 校验规则：`agent/progress.go` applyProgressReport 增加校验——本次更新之前所有状态为 in_progress 的步骤必须在本次 progress 报告中体现最新状态，若遗漏任何 in_progress 步骤则报错拒绝（防止正在运行的任务被遗忘），`i18n/en_system.go`/`zh_system.go` OpenAI+XML 两处 meta 说明的 progress 段落强调该规则，`agent/taskplan_event_test.go` 新增 TestApplyProgressReportForgottenInProgress/TestApplyProgressReportCoversInProgress 验证 [BUILD-717]；⑫ 工具调用示例 meta.progress 补全为两个待办记录：`i18n/en_system.go`/`zh_system.go` KeySystemPromptToolUsageMetaXML 的 meta 示例 progress 由单个 item 改为两个 item（index 0 completed 上一条已完成 + index 1 in_progress 正在执行），使示例符合 in_progress 覆盖规则 [BUILD-718]
  - 测试：见 use-case/FEATURE-452/

---

## v0.21.0 — 开发中

> **版本**: v0.21.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 优化（受影响对象高亮 + 模型管理界面）
> **说明**: 0.21.0 系列专注 Web UI 优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-448 | 0.20.1 | P1 | 明确 meta.progress 中 index 的取值规范：系统提示词未明确 index 是 0-based 还是 1-based，导致 LLM 难以给出准确值（代码逻辑为 0-based，index 从 0 开始，index == 当前步骤数时追加新步骤）；在系统提示词 meta 对象说明中明确 index 为 0-based（从 0 开始计数） |
| FEATURE-449 | 0.21.0 | P1 | Web UI 优化：① 受影响对象为根路径时也标记高亮；② 修复受影响对象在文件列表未展开文件夹时高亮不生效；③ 受影响对象高亮清理策略改为用户录入正式指令时清空（回答问题/补充信息不算），工具调用时只清空背景保留字体高亮；④ 模型管理界面优化（显示 url、toggle 开关、图标化按钮、点击模型 ID 进入修改向导） |
| FEATURE-450 | 0.21.0 | P1 | 任务计划相关改进：① track_task_progress 和 attempt_completion 去掉 meta 参数（从 xml/openai 方法声明及必需清单中移除），统一合法性校验以必需清单为准；② 对提供 meta 的方法，meta.progress 检查规则调整为至少提供 1 条当前状态记录（即便状态没变也要提供），并更新方法声明（xml/openai）及相关示例 |

> 当前 BUILD: 705
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-448 明确 meta.progress 中 index 的取值规范** ✅ 已完成 [BUILD-706]
  - 背景：meta.progress 中每个对象的 index 字段，其取值规范（0-based 还是 1-based）在系统提示词中没有明确说明。虽然代码逻辑是 0-based（`taskplan/taskplan.go` ApplyProgress：`s.Index < 0` 报错、`s.Index == len(plan.Steps)` 追加新步骤、`plan.Steps[s.Index]` 更新），但提示词只写了"index 必须与任务计划步骤索引一致"和"index 等于当前步骤数表示追加新步骤"，没有明确 index 从 0 开始，导致 LLM 难以给出准确值（可能用 1-based 导致 index 超出范围报错）。
  - 方案（已确认）：在系统提示词 meta 对象说明（`i18n/en_system.go`/`i18n/zh_system.go` 的 OpenAI + XML 两处 `KeySystemPromptToolUsageMetaOpenAI`/`KeySystemPromptToolUsageMetaXML`）中明确 index 是 **0-based（从 0 开始计数）**：第一个步骤 index 为 0，第 N 个步骤 index 为 N-1，index 等于当前步骤数时追加新步骤。
  - 实施：`i18n/en_system.go`/`i18n/zh_system.go` 的 OpenAI + XML 两处 meta 说明的 progress 段落补充 index 为 0-based 的说明 [BUILD-705]；进一步强调 meta.progress 的作用是精准调整变动的部分——只报告状态有变化的步骤及当前正在执行的步骤，未变化的步骤可以不传（无需包含在 progress 数组中）[BUILD-707]；修复多工具调用时意图/风险只显示在最后一个块：`web/static/app.js` 新增 `toolBlockByName` 映射（toolName→块对象），`tool_call_stream` 事件解析 `⚙️ <tool>` 头部记录工具名到映射，`tool_call` input 事件根据 summary.tool_name 切换 `curTool` 到正确块（使每个工具的标题/风险/结果按调用顺序落在各自块上），`done` 事件清空映射 [BUILD-708]；修复主模型上下文超长计算用错模型：`agent/loop.go` `GetMaxModelLen()` 原用 `GetActiveModel(false)`（全局默认模型，排第一位）作为分母，当用户在当前模式绑定了其他（非第一位）模型时造成错位，改为 `resolveModelForInfo(false)`（当前模式绑定模型优先，再回退全局默认）；`agent/envelope.go` context_window 显示同步改用 `GetMaxModelLen()` [BUILD-709]
  - 测试：见 use-case/FIX-448/

- [x] **FEATURE-449 Web UI 优化（受影响对象高亮 + 模型管理界面）** ✅ 已完成 [BUILD-710]
  - 背景：Web UI 存在若干交互细节问题：① 受影响对象为根路径（文件列表第一个目录）时未标记高亮；② 受影响对象在文件列表未展开文件夹时高亮不生效；③ 受影响对象高亮清理策略不合理（回答问题/补充信息时也清空）；④ 模型管理界面信息与操作不够直观。
  - 方案（已确认）：见 use-case/FEATURE-449/
  - 实施：① 受影响对象为根路径时也标记高亮：`web/static/app.js` `applyAffectedHighlight` 将根目录空路径 `""` 归一化为 `"."` 以匹配后端返回的根路径，使根目录行也能被高亮；② 修复未展开文件夹时高亮不生效：`web/static/app.js` `highlightAffectedFiles` 展开受影响对象所有祖先目录时同时把工作区根目录 `""` 加入 `expandedDirs`（此前根目录默认折叠，导致受影响对象所在的一级目录虽在 expandedDirs 中却因根 ul 折叠而不可见）；③ 高亮清理策略调整：`web/static/app.js` 新增 `clearAffectedHighlight`（完全清空背景+字体），`sendInput` 正式指令分支（pendingInteraction 为 false）调用它清空上一次高亮，回答问题/补充信息（pendingInteraction 为 true 的 answerInteraction 分支）不清空；工具调用时 `highlightAffectedFiles` 把上一次的 `aff-pred` 降级为新增的 `aff-pred-fg`（仅保留字体高亮、清空背景），`web/static/style.css` 新增 `.aff-pred-fg` 样式；④ 模型管理界面优化：`web/static/app.js` `renderModelsBody` 在 model-meta 下新增一行显示 endpoint URL（`.model-url`），禁用/启用改为 toggle 滑动开关（`.model-toggle`），切换改为向上箭头图标按钮（▲），删除改为垃圾桶图标按钮（🗑），去掉编辑按钮，点击模型 ID 打开编辑向导；`web/static/style.css` 新增 `.model-url`/`.model-icon`/`.model-del`/`.model-toggle`/`.model-toggle-slider` 样式 [BUILD-710]；修复降级高亮在 loadTree 重建后丢失：`web/static/app.js` 引入 `prevAffectedFiles` 变量记录上一次受影响对象，`highlightAffectedFiles` 保存上一次 affectedFiles 到 prevAffectedFiles，`applyAffectedHighlight` 在 loadTree 重建后对 prevAffectedFiles 应用 `aff-pred-fg`（仅字体）、对当前 affectedFiles 应用 `aff-pred`（完整高亮），`clearAffectedHighlight` 同时清空 prevAffectedFiles [BUILD-711]；模型管理操作模式调整：去掉每个模型行内的切换（▲）和删除（🗑）图标，移到工具栏（`web/static/index.html` 新增 `modelPinBtn` 置顶按钮在 +新增模型 左边、`modelDelBtn` 删除按钮在 +新增模型 右边且为红色 danger 样式），操作模式改为点选一个模型（`web/static/app.js` 新增 `selectedModelID` 变量和 `selectModel` 函数，点击模型行高亮边框表示选中，`web/static/style.css` 新增 `.model-row.selected` 高亮边框样式），置顶按钮对选中模型发送 `model_switch`，删除按钮对选中模型调用 `confirmDeleteModel` 显示警告确认对话框，模型行内仅保留 toggle 开关 [BUILD-712]；工具栏三个按钮（置顶/新增/删除）改为右对齐显示：`web/static/style.css` `.models-toolbar` 由 `justify-content: space-between` 改为 `justify-content: flex-end` 并加 gap [BUILD-713]
  - 测试：见 use-case/FEATURE-449/

- [ ] **FEATURE-450 任务计划相关改进（meta 参数与 progress 检查规则调整）** [BUILD-714]
  - 背景：① `track_task_progress` 和 `attempt_completion` 两个工具当前仍要求必填 `meta` 参数（xml/openai 方法声明及必需清单中均含 meta），但这两个工具本身是任务计划/完成报告工具，要求 meta 冗余且与统一合法性校验冲突；② 对提供 meta 的方法，`meta.progress` 当前规则允许"只报告状态变化的步骤，未变化可不传"，导致 LLM 可能完全不传 progress 或传空数组，无法反映当前执行状态。
  - 方案（已确认）：① 从 `track_task_progress`/`attempt_completion` 的 xml/openai 方法声明及必需清单中移除 meta 参数，统一合法性校验（`agent/run_stream.go` 的 `assessRisk`→`validateMeta`）改为以必需清单为准——工具必需清单不含 meta 时跳过 meta 校验；② `meta.progress` 检查规则调整为至少提供 1 条当前状态记录（即便状态没变也要提供），并更新方法声明（xml/openai）及相关示例。
  - 实施：① `agent/tools.go` 移除 `track_task_progress`/`attempt_completion` 的 meta 参数声明及必需清单项（track_task_progress required 改为 `[title, description, steps]`，attempt_completion required 改为 `[result, session_title, session_keywords]`）；② `agent/risk.go` 新增 `toolRequiresMeta(name)` 辅助方法（以必需清单为准判断工具是否要求 meta），`validateMeta` 要求 `meta.progress` 为非空数组（至少 1 条当前状态记录）；③ `agent/run_stream.go` 统一合法性校验（`assessRisk`）改为仅对必需清单含 meta 的工具调用；④ `i18n/en_system.go`/`zh_system.go` 从 `KeyToolUsageTrackTaskProgress`/`KeyToolUsageAttemptCompletion` 移除 meta 参数声明，OpenAI+XML 两处 meta 对象说明的 progress 规则更新为"至少提供 1 条当前状态记录（即便状态没变也要提供）"；⑤ `agent/meta_param_test.go` `TestInjectMetaParamAllTools` 排除这两个工具（断言 meta 不在其声明与必需清单中）[BUILD-714]
  - 测试：见 use-case/FEATURE-450/

- [ ] **FIX-451 修复 path 参数缺失 bug（LLM 将参数误嵌套进 meta 对象）** [BUILD-715]
  - 背景：search_files 等带 path 参数的工具，即便 LLM 传了 path 参数，也会报 "path argument is required"。日志显示失败调用中 path、regex 等参数被解析到了 meta 对象内部（`args=map[file_pattern:*.go meta:map[... path:/... regex:...] ...]`），顶层没有 path，导致 `searchFilesTool` 的 `args["path"].(string)` 断言失败。相邻成功调用中 path 在顶层，说明是 LLM 生成参数时把工具参数误嵌套进 meta 对象（OpenAI 模式 JSON 或 XML 模式）的不稳定格式错误。
  - 方案（已确认）：在 `executeToolCall` 解析 args 后做容错——meta 对象只合法持有 intent/risk/risk_reason/affected_objects/progress，若 meta 内部出现其他键（如 path、regex、command 等误放的工具参数），自动提升到顶层，使工具能正常运行。
  - 实施：① `agent/risk.go` 新增 `metaFieldNames` 常量集合与 `promoteMisplacedMetaParams(args)` 函数（遍历 meta 内部键，非 meta 字段且顶层不存在时提升到顶层，顶层已存在时以顶层为准）；② `agent/tools.go` `executeToolCall` 在 `json.Unmarshal` 解析 args 后调用 `promoteMisplacedMetaParams(args)`；③ `agent/promote_meta_test.go` 新增 `TestPromoteMisplacedMetaParams` 单元测试（验证 path/regex 提升、合法 meta 字段保留、顶层优先、无 meta 时 no-op）[BUILD-715]
  - 测试：见 use-case/FIX-451/

---

## v0.20.0 — 开发中

> **版本**: v0.20.0

> **状态**: 🚧 开发中
> **里程碑**: 工具调用透明化
> **说明**: 0.20.0 系列专注工具调用透明化：LLM 每次调用工具时报告任务进展、影响文件、风险等级，前端用颜色标识风险、动态高亮工作区文件，提升用户体验与意图暴露。细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-447 | 0.20.0 | P1 | 工具调用透明化：LLM 每次调用工具时报告任务进展（progress 数组）、影响文件（确定性从参数解析/预测性从 LLM 字段）、风险等级（LLM 自评+规则引擎兜底），前端工具卡片显示风险颜色标识、工作区文件树动态高亮（确定性=accent 色文字变色无背景，预测性=蓝色） |

> 当前 BUILD: 704
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-447 工具调用透明化（风险+影响文件+进展报告）**
  - 背景：LLM 调用工具时，用户无法直观看到当前任务进展、操作影响哪些文件、操作风险等级。需要让 LLM 每次调用工具时主动报告这些信息，前端用颜色标识风险、动态高亮工作区文件，提升用户体验与意图暴露。
  - 方案（已确认）：① 扩展 ToolSummary 增加 Risk/RiskReason/Files/Progress 字段；② 风险等级=LLM 自评+规则引擎兜底（读=low、新建/改=medium、删/覆盖/写 workspace 外/执行命令=high、涉及敏感路径=high），最终取较高者；③ 影响文件分方法：确定性工具（read_file/replace_in_file/write_to_file 等）从参数解析，预测性工具（execute_command 等）从 LLM 预测的 files 字段解析（最多 3 个）；④ 进展报告：LLM 在参数中附带 progress 数组（index/description/status），后端校验（index 超出范围=新增，超出可新增范畴=报错），更新任务计划并推送 EventTaskPlan；⑤ 前端工具卡片显示风险等级颜色标识，文件树高亮（确定性=accent 色文字变色无背景，预测性=蓝色）。
  - 实施：① `agent/tool_summary.go` 扩展 ToolSummary 增加 Risk/RiskReason/Files/Progress 字段 + 新增 ProgressStep 结构；② `agent/risk.go` 新增风险等级计算（规则引擎 readTools/writeTools/execTools + 敏感路径检测 + LLM 自评取较高者）；③ `agent/files.go` 新增影响文件提取（确定性工具从 path/paths 参数解析，预测性工具从 files 字段解析最多 3 个，AffectedFile 携带 deterministic 标志）；④ `agent/progress.go` 新增进展报告解析与校验（parseProgressSteps + applyProgressReport）；⑤ `taskplan/taskplan.go` 新增 ProgressInput 结构 + ApplyProgress 方法（index 更新/追加/超范围报错）；⑥ `agent/run_stream.go` 工具执行循环填充风险/影响文件元数据 + 应用进展报告 + 推送 EventTaskPlan；⑦ `web/static/app.js` 工具卡片风险标识 + 文件树受影响文件高亮（highlightAffectedFiles/applyAffectedHighlight）；⑧ `web/static/style.css` 新增 risk-badge 风险颜色 + aff-det/aff-pred 文件高亮样式；⑨ `i18n/` 新增 KeySystemPromptToolTransparency（zh/en）+ KeyTaskPlanDefaultTitle；⑩ `agent/toolcall_mode.go` buildXMLToolPrompt 注入透明化说明 [BUILD-683]；调整：风险等级改为必填项全部由 LLM 评估（移除规则引擎，缺失按解析错误处理），敏感路径检测保留仅用于信息收集，文件影响全部交给 LLM（移除 pathParamTools 分类，统一从 files 字段解析，前端高亮统一蓝色）[BUILD-684]；统一 meta 对象参数：所有工具增加必填 meta 对象参数（放最前，含 intent/risk/risk_reason/files/progress），移除工具定义顶层 intent 参数（统一用 meta.intent），视觉工具（visual_analysis/browser_screenshot）增加必填 instruct 参数（给视觉模型的指令，meta.intent 给用户），meta 对象结构说明放系统提示词节约上下文（`agent/tools.go` injectMetaParam 统一注入 + `agent/risk.go` metaObject 辅助 + `agent/risk.go`/`agent/files.go`/`agent/progress.go`/`agent/tool_summary.go` 从 meta 对象解析 + `agent/image_tools.go`/`agent/browser_tools.go` 用 instruct 作为视觉指令 + `i18n/` 更新 KeySystemPromptToolTransparency 说明 meta 对象结构）[BUILD-685]；meta 对象参数改为源码直接声明：`agent/tools.go` buildToolsInternal 中每个工具定义直接声明 meta 对象参数（放最前，移除顶层 intent），视觉工具直接声明 instruct 参数，移除对所有工具的 injectMetaParam 运行时注入循环（仅保留对 MCP 工具注入，因其 InputSchema 来自 MCP 服务器），新增 `agent/meta_param_test.go` 验证所有工具定义均含 meta 参数 [BUILD-686]；更新 XML 工具使用模板：`i18n/en_system.go`/`i18n/zh_system.go` 所有 KeyToolUsage* 模板的 intent 参数改为 meta 对象（放最前），XML 示例改为 <meta> 对象（含 intent/risk），视觉工具模板加 instruct 参数，KeySystemPromptToolUsageXML/KeySystemPromptXMLExamples 的 XML 示例同步更新为 meta 对象 [BUILD-687]；合并 meta 对象说明到 ToolUsage 资源：将 KeySystemPromptToolTransparency 的 meta 对象结构说明合并到 KeySystemPromptToolUsage 和 KeySystemPromptToolUsageXML（不再单开资源），补充 meta.progress 与 track_task_progress 的一致性约定（index 必须与任务计划步骤索引一致、status 取值与 track_task_progress 一致 pending/in_progress/completed/cancelled/failed 或 [ ]/[=]/[X]/[C]/[F]），删除 KeySystemPromptToolTransparency 常量及 buildXMLToolPrompt 中单独拼接逻辑（`i18n/en_system.go`/`i18n/zh_system.go`/`i18n/keys.go`/`agent/toolcall_mode.go`）[BUILD-688]；meta.files 改名为 affected_objects：meta 对象中影响对象字段由 files 改为 affected_objects（更通用，不仅限于文件），同步更新后端解析（`agent/files.go` meta["affected_objects"]）、工具定义 meta 参数描述（`agent/tools.go`）、系统提示词 meta 对象说明与 XML 示例（`i18n/en_system.go`/`i18n/zh_system.go`）、测试（`agent/transparency_test.go`）[BUILD-689]；增加意图暴露开关 intent-exposure-enabled：控制整个 meta 透明化（intent/risk/affected_objects/progress），关闭时从工具定义移除 meta 参数要求（`agent/tools.go` buildToolsInternal 剥离 meta）、不注入系统提示词 meta 说明（`agent/toolcall_mode.go` stripMetaDescription）、后端不解析 meta，命令行（`main.go` --intent-exposure-enabled）、REPL（`cmd/settings_agent.go` .set intent-exposure-enabled）、Web UI（`cmd/settings_web.go`/`cmd/settings_tools.go`/`agent/settings_tools.go`）均增加参数，配置 `config/config.go` LLM.IntentExposureEnabled（默认 true）[BUILD-690]；meta 说明改为占位符方案：将 meta 对象说明从 KeySystemPromptToolUsage/KeySystemPromptToolUsageXML 中拆出为单独资源（`KeySystemPromptToolUsageMetaOpenAI`/`KeySystemPromptToolUsageMetaXML`，en/zh），ToolUsage 中用 `{META_DESCRIPTION}` 占位符，构建系统提示词时根据 intentExposureEnabled 决定是否注入 meta 说明（`agent/toolcall_mode.go` buildXMLToolPrompt、`agent/system_prompt.go` buildNamedSection），移除 stripMetaDescription 字符串剥离逻辑 [BUILD-691]；修复禁用意图暴露时 required 清单残留 meta 的问题：`agent/tools.go` 剥离逻辑同时处理 `[]string` 和 `[]interface{}` 两种 required 类型，确保 meta 从 required 清单中一并移除 [BUILD-692]；修复必填参数校验 bug + 去掉 OpenAI 模式 REQUIRED 前缀：`agent/toolcall_mode.go` 必填参数校验同时处理 `[]string` 和 `[]interface{}` 两种 required 类型（此前 `[]interface{}` 断言导致校验失效），`agent/tools.go` 去掉所有参数说明的 `**REQUIRED**: ` 前缀（OpenAI 模式，XML 方式说明保留）[BUILD-693]；去掉 OpenAI 模式 Optional 前缀：`agent/tools.go` 去掉所有参数说明的 `Optional: ` 前缀（OpenAI 模式，XML 方式说明保留）[BUILD-694]；清理 affected_objects 路径尾随冒号：LLM 可能在路径后追加冒号导致前端精确匹配失败、文件高亮不生效，`agent/files.go` 新增 cleanAffectedPath 去掉尾随冒号/逗号/分号及空白，新增 `agent/transparency_test.go` TestAffectedFilesTrailingColon 验证 [BUILD-695]；受影响文件高亮自动展开目录并滚动：`web/static/app.js` highlightAffectedFiles 清除旧高亮、展开受影响文件所在目录（加入 expandedDirs）、重建文件树后高亮并滚动到第一个受影响文件（scrollToAffected），高亮只反映当前工具调用（下次调用自动恢复）[BUILD-696]；增强受影响文件高亮效果：`web/static/style.css` aff-pred 由纯蓝色文字改为蓝色文字+加粗+淡蓝背景+圆角，使受影响文件在文件树中更醒目 [BUILD-697]；受影响文件路径转换为相对路径：后端 `agent/files.go` affectedFiles 改为方法并用 workspacePath 把工作区内的绝对路径转换为相对路径（relativizeAffectedPath），使前端能匹配文件树的相对路径 dataset.path，修复高亮和目录展开不生效，新增 `agent/transparency_test.go` TestAffectedFilesRelativize 验证 [BUILD-698]；meta 对象所有属性统一必填校验：`agent/risk.go` 新增 validateMeta 校验 intent/risk/risk_reason/affected_objects/progress 全部必填（缺失报错），assessRisk 先调用 validateMeta，系统提示词 meta 对象说明（`i18n/en_system.go`/`i18n/zh_system.go` OpenAI+XML 模式）所有属性改为 REQUIRED/必需，更新 `agent/transparency_test.go` TestValidateMeta 和 `agent/taskplan_event_test.go` 测试 [BUILD-699]；修复模型设置 endpoint 自动补全结果未持久化：Web UI 模型向导「测试连通性」成功后只显示自动补全后的 endpoint 但未写回输入框和 wizardData，导致用户输入不完整 URL 时虽测试通过但保存的仍是未补全的 URL、实际使用失败，`web/static/app.js` test-endpoint 成功时若 body.endpoint 与用户输入不同则写回 ctl.value 和 wizardData.endpoint（CLI 流程 cmd/model.go 已正确持久化，无需修改）[BUILD-700]；修复 meta.progress 未驱动任务进展展示：`agent/run_stream.go` 工具执行循环检查 progress 时误用顶层 `argsMap["progress"]`（progress 实际在 meta 对象内 `meta.progress`），导致 applyProgressReport 从未被调用、任务进展不更新，改为 `hasNonEmptyProgress(metaObject(argsMap))` 检查 meta 对象内非空 progress 数组，新增 `agent/progress.go` hasNonEmptyProgress 辅助函数，新增 `agent/taskplan_event_test.go` TestApplyProgressReportFromMeta 验证 meta.progress 正确解析应用 [BUILD-701]；视觉识别 token usage 回传主 LLM：`agent/run_stream.go` 视觉识别轮次（FEATURE-343 afterESC 块）在识别结果确定后，通过 IterTokenDelta 获取视觉识别 LLM 调用的 token usage（输入/输出/总计），通过 GetMaxModelLen 获取视觉模型最大上下文长度，计算百分比并附加到返回给主 LLM 的识别结果中（`i18n/` 新增 KeyVisionRecognitionTokenUsage en/zh，格式：输入/输出/总计 + 占视觉模型上下文百分比）[BUILD-702]；修复视觉识别百分比用错模型上下文：`agent/run_stream.go` 视觉识别轮次计算 token 消耗百分比时误用 GetMaxModelLen（返回主文本模型上下文 524288），改为 resolveModelForInfo(true) 获取视觉模型最大上下文（262144），使百分比准确反映占视觉模型上下文的比例 [BUILD-703]；提交合并到 main 并打版本标签 v0.20.0 [BUILD-704]
  - 测试：见 use-case/FEATURE-447/

---

## v0.19.0 — 开发中

> **版本**: v0.19.0

> **状态**: 🚧 开发中
> **里程碑**: Web UI 文件预览/图片查看改进
> **说明**: 0.19.0 系列专注 Web UI 文件预览与图片查看的交互与信息展示改进，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-444 | 0.19.0 | P1 | Web UI 文件预览/图片查看 3 项改进：① 文件预览底部标题栏增加文件大小数字（格式 1,234,567B），标题栏边框改用与文件名一致的高亮色（accent），透明度规则不变；② 图片预览弹窗增加标题栏（关闭图标放标题栏右侧，显示名称/大小/修改日期/分辨率），标题栏不折行，小图时窗口被标题栏内容撑起；③ 图形查看由双击改为单击点选，双击改为用操作系统默认程序打开 |

> 当前 BUILD: 677
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-444 Web UI 文件预览/图片查看 3 项改进** ✅ 已完成 [BUILD-678]
  - 背景：Web UI 文件预览与图片查看存在 3 处可改进点：① 文件预览底部标题栏未显示文件大小，且标题栏边框用普通边框色而非高亮色；② 图片预览弹窗无标题栏，小图时关闭图标与图片混在一起难以辨认；③ 图形查看需双击才打开，与文本文件单击预览的交互不一致。
  - 方案（已确认）：① 后端 treeNode 增加 Size 字段，前端文件预览标题栏增加文件大小显示（千分位格式 1,234,567B），标题栏边框改用 accent 高亮色，透明度规则不变；② 图片预览弹窗增加标题栏（关闭图标放标题栏右侧，显示名称/大小/修改日期/分辨率），标题栏不折行，小图时窗口被标题栏内容撑起；③ 图形文件单击打开图片预览，双击用系统默认程序打开。
  - 实施：① `web/server.go` treeNode 增加 `Size int64` 字段 + buildTree 填充 `info.Size()`；② `web/static/index.html` 文件预览标题栏增加 size 元素、图片预览弹窗增加标题栏结构；③ `web/static/app.js` 文件预览填充 size、图片预览填充标题栏信息（名称/大小/修改日期/分辨率，分辨率从图片 naturalWidth/naturalHeight 读取）、图形单击预览/双击系统打开；④ `web/static/style.css` 文件预览标题栏边框改 accent、图片预览标题栏样式（不折行、小图撑起窗口）；⑤ 测试用例 + `web/filepreview_test.go` 新增 TestTreeSizeField 验证 tree 节点 size 字段
  - 测试：见 use-case/FEATURE-444/

- [x] **FEATURE-445 Web UI 自动分区滚动改为跟随滚动** ✅ 已完成 [BUILD-679]
  - 背景：Web UI 当前使用自动分区滚动（FEATURE-416）：用户向上滚动查看历史时，流被分成静态区 B + 动态区 A，新输出进 A。用户希望改为更简单的跟随滚动机制：不再自动分区，而是当滚动条位置在距离内容末尾 100 像素内时自动跟随新输出；超出 100 像素则不跟随，方便查看历史；并提供底部悬浮向下箭头按钮一键回到末尾恢复跟随。
  - 方案（已确认）：① 移除自动分区机制（splitStream/mergeStream/splitActive 及 streamA/mergeBtn 相关）；② 改为跟随滚动：`scrollStream()` 判断滚动条是否在距末尾 100px 内，是则滚动到底部；③ 滚动位置超过 100px 后，底部悬浮显示向下箭头按钮（下三角矢量图标、扁平效果、胶囊边框、半透明、hover 高亮），点击滚动到底部并恢复自动跟随。
  - 实施：① `web/static/app.js` 移除 splitStream/mergeStream/splitActive/streamA/mergeBtn 相关逻辑，`scrollStream()` 改为 100px 阈值跟随判断（followOutput 状态），新增 updateFollowState/jumpToBottom 与底部箭头按钮的显示/隐藏/点击逻辑；② `web/static/index.html` 移除 streamA 元素，新增底部向下箭头按钮（SVG 下三角矢量图标）；③ `web/static/style.css` 移除 stream-a/stream-merge 样式，新增 scroll-down 按钮样式（胶囊/半透明/hover 高亮）；④ 测试用例
  - 测试：见 use-case/FEATURE-445/

- [x] **FEATURE-446 Web UI 区块边界导航箭头位置调整 + token 统计行格式调整 + 状态条间距调整** ✅ 已完成 [BUILD-682]
  - 背景：① 主消息区的区块边界导航小箭头（blockNavTop/blockNavBottom，FEATURE-419）当前位于顶部/底部居中位置，用户上下切换定位区块边界时需要移动较大距离，希望移到靠近右边滚动条、且靠近垂直方向中间的位置；② LLM 输出后的 token 统计行格式需调整，从 `↑335,499 ↓1,266 Σ336,765/524,288 4.0s 68` 改为 `↑335,499 10.0s 33,549t/s ↓1,266 4.0s 68t/s`（输入/输出分别显示耗时与速率，移除总 token/上下文窗口）；③ Web UI 下方状态条右侧显示项（图标引导的几部分信息）之间间距需调小到 8px。
  - 方案（已确认）：① 将 `.block-nav.top` 从 `top:10px; left:50%` 改为 `right:8px; top:50%; transform:translateY(-115%)`（中间偏上），`.block-nav.bottom` 从 `bottom:10px; left:50%` 改为 `right:8px; bottom:50%; transform:translateY(115%)`（中间偏下）；② token 统计行改为：`↑prompt [输入耗时 输入速率t/s] ↓completion [ft] [输出速率t/s]`，输入耗时=prompt/in_tps，输入速率=in_tps，输出耗时=ft，输出速率=out_tps，移除 Σtotal/max；③ `.statusbar` 的 `gap: 4px 18px` 改为 `gap: 4px 8px`（列间距 18px→8px）。
  - 实施：① `web/static/style.css` 修改 `.block-nav.top`/`.block-nav.bottom` 定位（right:8px + 垂直中间偏上/偏下）；② `web/static/app.js` token_iter 统计行格式改为输入/输出分别显示耗时与速率（复用 fmtDur，输入耗时=prompt/in_tps，输出耗时=ft，速率用 fmtNum+t/s），移除 Σtotal/max；③ `web/static/style.css` `.statusbar` gap 列间距 18px→8px；④ 测试用例
  - 测试：见 use-case/FEATURE-446/

---

## v0.18.0 — 开发中

> **版本**: v0.18.0

> **状态**: 🚧 开发中（YOLO 模式总开关）
> **里程碑**: YOLO 模式总开关
> **说明**: 0.18.0 系列专注 YOLO 模式（全自动执行模式）总开关，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-439 | 0.18.0 | P1 | YOLO 模式总开关：在工具调用确认入口做总开关，开启后跳过所有人为判断直接执行所有工具调用（disabled 工具 LLM 看不到不受影响）；CLI `:YOLO`（必须大写）命令 toggle 并显示当前状态；Web UI 右下角运行/暂停按钮旁新增古典上下拨动开关（默认关拨杆向下 OFF，点击拨杆向上橙红色警示 ON，悬停显示 YOLO模式）；启动默认关，状态不持久化 |
| FEATURE-440 | 0.18.0 | P1 | 新增 2 个 shell 启动脚本（与 co-shell 可执行程序同目录）：run-cli.sh 以 enhanced 模式启动 CLI（--input-mode enhanced）；run-web.sh 以白名单形式启动对其他主机的 Web UI 服务（--serve --bind 0.0.0.0 --whitelist，白名单通过参数或 COSHELL_WHITELIST 环境变量指定） |
| FEATURE-441 | 0.18.0 | P1 | run-web.sh 白名单支持读取默认名称文件（WHITELIST）：白名单来源优先级 命令参数 > 环境变量 COSHELL_WHITELIST > 默认文件 WHITELIST；若三种来源均未提供白名单，则提示用户提供并退出（不再默认 127.0.0.1） |
| FEATURE-442 | 0.18.0 | P1 | show-tool-input 配置默认值改为 true（默认打开）：修改 config/config.go DefaultConfig() 中 ShowToolInput 默认值 false→true，并更新字段注释 |
| FEATURE-443 | 0.18.0 | P1 | 修复 Web UI md 渲染器（md.js）有序列表编号问题：有序列表项被其他内容（如子列表、段落）打断时被分成多个 <ol>，每个从 1 开始编号，导致原文 1、2 渲染后都变 1；修复为使用 <ol start=N> 从原文序号开始编号 |
| FEATURE-444 | 0.19.0 | P1 | Web UI 文件预览/图片查看 3 项改进：① 文件预览底部标题栏增加文件大小数字（格式 1,234,567B），标题栏边框改用与文件名一致的高亮色（accent），透明度规则不变；② 图片预览弹窗增加标题栏（关闭图标放标题栏右侧，显示名称/大小/修改日期/分辨率），标题栏不折行，小图时窗口被标题栏内容撑起；③ 图形查看由双击改为单击点选，双击改为用操作系统默认程序打开 |
| FEATURE-445 | 0.19.0 | P1 | Web UI 自动分区滚动改为跟随滚动：不再自动分区（移除 splitStream/mergeStream 机制），改为当滚动条位置在距离内容末尾 100 像素内时，有新动态内容输出则自动滚动到内容末尾（自动跟随输出）；滚动位置超出 100 像素则不跟随，方便查看历史；滚动位置超过 100 像素后，底部悬浮显示向下箭头按钮（下三角矢量图标、扁平效果、胶囊边框、半透明、鼠标滑过恢复高亮），点击直达最后内容（恢复自动输出跟踪） |
| FEATURE-446 | 0.19.0 | P1 | Web UI 区块边界导航箭头位置调整：将主消息区顶部/底部的区块边界导航小箭头（blockNavTop/blockNavBottom）从顶部/底部居中位置移到靠近右边滚动条、且靠近垂直方向中间的位置（一个在中间偏上、一个在中间偏下），使用户上下切换定位区块边界时无需挪动很大位置 |

> 当前 BUILD: 668
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-439 YOLO 模式总开关** ✅ 已完成
  - 背景：agent 工具中常见的 YOLO 模式（You Only Live Once）指自动批准所有工具调用、不逐个询问用户。co-shell 现有 confirm-tool 支持逐工具/全局 auto 模式，但缺少一个独立的、临时的总开关，用户希望一键开启/关闭全自动执行，且不改变 confirm-tool 配置。
  - 方案（已确认）：在工具调用确认入口（agent/tools.go executeToolCall）加 YOLO 总开关判断，开启后 `needsConfirm=false` 跳过所有确认直接执行。disabled 工具 LLM 本来就看不到、不会调用，不受 YOLO 影响。CLI 新增 `:YOLO`（必须大写）命令 toggle 并显示当前状态。Web UI 右下角运行/暂停按钮旁新增古典上下拨动开关。启动默认关，状态不持久化。
  - 需求：① Agent 新增 `yoloMode` 字段 + `SetYOLO`/`IsYOLO` 方法；② `agent/tools.go` 确认入口加 YOLO 判断（`needsConfirm = mode=="confirm" && !a.yoloMode`）；③ CLI `repl/repl.go` handleBuiltin 新增 `case ":YOLO":` toggle 并显示状态；④ Web `session.go` handleMessage 新增 `yolo_set`/`yolo_get` 消息；⑤ Web UI `index.html` input-row sendBtn 旁新增古典拨动开关 + `app.js` 交互 + `style.css` 样式；⑥ i18n 中英文案；⑦ 测试用例。
  - 实施：① `agent/loop.go` Agent 结构体新增 `yoloMode` 字段；② `agent/agent.go` 新增 `SetYOLO`/`IsYOLO` 方法；③ `agent/tools.go` 确认入口 `needsConfirm = mode=="confirm" && !a.yoloMode`；④ `repl/repl.go` handleBuiltin 新增 `case ":YOLO":` + `handleYOLOCommand` 方法；⑤ `web/server.go` clientMessage/serverMessage 新增 `YOLO` 字段；⑥ `web/session.go` handleMessage 新增 `yolo_set`/`yolo_get` + `handleYOLOSet`/`handleYOLOGet` 方法；⑦ `web/static/index.html` input-row sendBtn 旁新增古典拨动开关；⑧ `web/static/app.js` 新增 `setYOLO`/`yoloSwitch.onclick`/`yolo` 消息处理/`yolo_get` 初始化；⑨ `web/static/style.css` 新增 `.yolo-switch` 系列样式；⑩ `i18n/keys.go`/`en.go`/`zh.go` 新增 `KeyYOLOOn`/`KeyYOLOOff` 中英文案；⑪ 测试：`agent/yolo_test.go`（4 个）、`web/session_test.go`（2 个）、`repl/yolo_test.go`（1 个）[BUILD-668]；⑫ 补充：Web UI 前端调整——sendBtn 高度调为 30px 与录入框一致、yolo-switch 高度调为 30px、扳手（.yolo-thumb）写小字 YOLO 且边框加粗为 2px [BUILD-669]；⑬ 补充：Web UI 前端调整——sendBtn/yolo-switch 高度调为 32px 与录入框对齐、YOLO 开关 hover 只高亮 YOLO 文字不高亮边框 [BUILD-670]；⑭ 补充：Web UI 前端调整——sendBtn/yolo-switch 高度调为 33px 与录入框完全对齐 [BUILD-671]；⑮ 补充：Web UI 前端调整——sendBtn/yolo-switch 高度调为 34px 与录入框完全对齐 [BUILD-672]；⑯ 补充：Web UI 前端调整——YOLO 开关 hover 提示信息解释 YOLO 含义（data-i18n-title=yoloTitle，中英文案）[BUILD-673]；⑰ 合并前 build 计数更新 [BUILD-674]
  - 测试：见 use-case/FEATURE-439/

- [x] **FEATURE-440 新增 2 个 shell 启动脚本** ✅ 已完成
  - 背景：用户希望提供便捷的启动脚本，与 co-shell 可执行程序放在一起，一键以 enhanced 模式启动 CLI，或以白名单形式启动对其他主机的 Web UI 服务。
  - 方案（已确认）：在 work/ 目录（与 co-shell 可执行程序同目录）新增 2 个 shell 脚本：`run-cli.sh` 以 `--input-mode enhanced` 启动增强交互 REPL；`run-web.sh` 以 `--serve --bind 0.0.0.0 --whitelist` 启动 Web UI 服务（白名单通过第一个参数或 `COSHELL_WHITELIST` 环境变量指定，未指定时默认仅本机回环）。
  - 实施：① `work/run-cli.sh`——定位脚本同目录 co-shell，`exec "$BIN" --input-mode enhanced "$@"` 透传参数；② `work/run-web.sh`——定位脚本同目录 co-shell，解析白名单（参数优先，其次环境变量，默认 127.0.0.1），`exec "$BIN" --serve --bind 0.0.0.0 --whitelist "$WHITELIST" "$@"`；③ 两个脚本 `chmod +x` 可执行；④ 验证：run-cli.sh 正确调用 co-shell 并传递 enhanced 参数、run-web.sh 正确启动 Web UI 服务（绑定 0.0.0.0，白名单生效）
  - 测试：见 use-case/FEATURE-440/

- [x] **FEATURE-441 run-web.sh 白名单文件支持** ✅ 已完成 [BUILD-675]
  - 背景：run-web.sh 以白名单形式启动 Web UI 服务，但当前未指定白名单时默认仅本机回环（127.0.0.1）。用户希望白名单支持读取默认名称文件（如 WHITELIST），且若用户未提供任何白名单信息（命令参数、环境变量、白名单文件），则提示用户提供而不要继续。
  - 方案（已确认）：run-web.sh 白名单来源优先级：命令参数 > 环境变量 COSHELL_WHITELIST > 默认文件 WHITELIST（脚本同目录）。若三种来源均未提供白名单，则打印提示并退出（exit 1），不再默认 127.0.0.1。
  - 实施：① `work/run-web.sh` 白名单解析改为：参数优先，其次环境变量，再次读取同目录 WHITELIST 文件（存在且非空时读取内容作为白名单）；② 三种来源均无时打印提示（说明三种提供方式）并 `exit 1`；③ 更新脚本头部注释说明；④ 验证：参数/环境变量/文件/无来源四种情况
  - 测试：见 use-case/FEATURE-441/

- [x] **FEATURE-442 show-tool-input 默认打开** ✅ 已完成 [BUILD-676]
  - 背景：show-tool-input 配置默认值为 false（不显示工具调用输入参数），用户希望默认打开。
  - 方案（已确认）：将 config/config.go DefaultConfig() 中 ShowToolInput 默认值从 false 改为 true，并更新字段注释。
  - 实施：① `config/config.go` DefaultConfig() 中 `ShowToolInput: false` → `true`；② 字段注释 `(default: false)` → `(default: true)`；③ 验证：无测试依赖该默认值，编译通过

- [x] **FEATURE-443 修复 md.js 有序列表编号问题** ✅ 已完成 [BUILD-677]
  - 背景：Web UI md 渲染器（md.js）渲染有序列表时，当列表项被其他内容（如子列表、段落）打断，会被分成多个独立的 `<ol>`，每个都从 1 开始编号，导致原文中 `1.`、`2.` 的列表项渲染后都变成 `1.`。
  - 方案（已确认）：有序列表渲染时解析首个列表项在原文中的序号，设置 `<ol start=N>` 属性，让每个 `<ol>` 从原文序号开始编号，即使被其他内容打断也能保持正确序号。
  - 实施：① `web/static/md.js` 有序列表分支：`ordered` 时解析 `m[1]` 中的数字并设置 `list.start`；② 验证：原文 `1.`、`2.`（中间含 `•` 子列表）渲染后 `<ol>` 的 start 分别为 1、2，浏览器正确显示 1、2

---

## v0.17.0 — 开发中

> **版本**: v0.17.0

> **状态**: 🚧 开发中（死循环历史污染修正）
> **里程碑**: 死循环历史污染修正
> **说明**: 0.17.0 系列专注死循环二次判定后的历史污染修正，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-438 | 0.17.0 | P1 | 死循环历史污染修正：二次判定确认死循环后，判定模型额外返回 history_fixes 列表（message_index + search + replace + reason），程序执行历史修正（消息序号定位 + 原文匹配对 search→replace 精确替换 + 相邻消息兜底 + 只替换首次出现 + 仅 assistant 消息 + 与原有处理叠加），范围最近 3 条 assistant 消息，新增配置开关 loop-history-fix-enabled 默认 on，写日志 + 用户可见提示，先修正后 delete |

> 当前 BUILD: 658
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-438 死循环历史污染修正**
  - 背景：死循环二次判定确认后，当前处理主要是丢弃循环导致未完成的消息（delete_last_msg）或追加纠正信息（prompt_feedback），但发生死循环往往意味着历史消息可能已被导致死循环的文字污染。污染源不清理，下一轮 LLM 仍可能被带偏。
  - 方案（已确认）：二次判定确认死循环后，判定模型在 report_problem 中额外返回 history_fixes 列表（每项含 message_index 消息序号 + search 被替换原文 + replace 替换新内容 + reason 原因），程序执行历史修正：用消息序号定位消息，原文匹配对 search→replace 精确替换，相邻消息（±1）兜底查找，只替换首次出现，仅允许 assistant 消息，与原有处理（prompt_feedback/delete_last_msg 等）叠加执行。范围：最近 3 条 assistant 消息。新增配置开关 loop-history-fix-enabled 默认 on。写日志 + 用户可见提示。先执行历史修正再执行 delete_last_msg。
  - 需求：① `ProblemReport` 新增 `HistoryFixes []HistoryFix` 字段（HistoryFix: MessageIndex int + Search string + Replace string + Reason string）；② `reportProblemTool` schema 增加 `history_fixes` 数组属性；③ 新增 `applyHistoryFixes` 执行逻辑（消息序号定位 + 原文匹配替换 + 相邻兜底 + 仅 assistant + 首次出现）；④ 接入 run_stream.go 二次判定处理流程，与原有处理叠加，先修正后 delete；⑤ 新增配置开关 `loop-history-fix-enabled` 默认 on；⑥ i18n 文案 + 用户可见提示。
  - 实施：① `agent/problem_solver.go` `ProblemReport` 新增 `HistoryFixes []HistoryFix` 字段 + 定义 `HistoryFix` 结构体（MessageIndex/Search/Replace/Reason）+ `reportProblemTool` schema 增加 `history_fixes` 数组属性；② `agent/loop_detector.go` `LoopJudgeResult` 新增 `HistoryFixes` 字段；③ `agent/loop.go` `judgeLoop` 从 report 传递 `HistoryFixes`、Agent 结构体新增 `loopHistoryFixes` 字段、`handleLoopDetection` 确认循环时保存/非循环时清空、新增 `applyHistoryFixes` 方法（消息序号定位 + 原文匹配替换 + 相邻兜底 + 仅 assistant + 只替换首次出现 + 仅最近 3 条 + 空 search/search==replace 忽略）+ `applyLoopIntervention` 确认循环时应用历史修正；④ `agent/run_stream.go` 迭代开始重置 `loopHistoryFixes`、`loopDetectCrit` 块中先执行历史修正再 strip/反馈；⑤ `config/config.go` 新增 `LoopHistoryFixEnabled` 字段默认 true；⑥ `cmd/settings.go`/`settings_safety.go`/`settings_web.go` 接入 `loop-history-fix-enabled` 设置项；⑦ i18n 新增 `KeyLoopHistoryFixApplied`/`KeyCol3LoopHistoryFixEnabled` 中英文案；⑧ `agent/loop_history_fix_test.go` 新增 10 个单元测试（定位替换/首次出现/非assistant跳过/相邻兜底/无匹配跳过/仅最近3条/空search/search==replace/ContentParts/空列表）[BUILD-659]；⑨ 补充：`agent/loop.go` 新增 `getRecentAssistantHistory` 方法（最近 3 条 assistant 消息按 `[真实索引] 内容` 格式返回，排除 tool_calls 消息）、`buildLoopJudgeUserPrompt` 填充 `{HISTORY}` 占位符；`i18n/zh_loop.go`/`en_loop.go` `KeyLoopJudgeUserPrompt` 新增 `{HISTORY}` 段并说明 history_fixes 用法（type=loop 时返回 history_fixes，message_index 用 [序号]，search 逐字匹配，仅修正 assistant 消息）；`agent/loop_history_fix_test.go` 新增 3 个单元测试（getRecentAssistantHistory 索引格式/排除tool_calls/buildLoopJudgeUserPrompt 含 HISTORY）[BUILD-660]；⑩ 补充：`applyHistoryFixes` 每条修复详情（消息序号 + search→replace + reason）通过 `a.streamCb` 输出到 `ChannelDebug`（dbg 块，锁外输出避免死锁），结果提示（已修正 N 处）保留 `ChannelSystem`；i18n 新增 `KeyLoopHistoryFixDetail` 中英文案 [BUILD-661]；⑪ 补充：去掉 `{ITERATIONS}` 段（提示词模板 zh/en + `buildLoopJudgeUserPrompt` 填充 + 删除 `getRecentIterations` 方法），避免与 `{HISTORY}` 内容重复；新增配置项 `loop-history-fix-max-messages`（最近 N 条，默认 5，范围 1-50）替代硬编码 `loopHistoryFixMaxMessages=3`，`getRecentAssistantHistory`/`applyHistoryFixes` 通过 `historyFixMaxMessages()` 读取配置，接入 `:set`/Web 设置 + i18n `KeyCol3LoopHistoryFixMaxMsgs` [BUILD-662]；⑫ 补充：`ask_followup_question` 选项列表末尾固定显示"补充信息"选项（`[len(options)+1]`，取消移到 `[len(options)+2]`），用户选它后进入自由输入；用户输入以空格开头的内容直接作为补充信息送给 LLM；i18n 新增 `KeySettingCmd_776`/`KeySettingCmd_777` 中英文案；`agent/interaction_test.go` 更新 `TestTerminalSelectCancel`/`TestAskFollowupQuestionCancel` 并新增补充信息选项/空格输入测试 [BUILD-663]；⑬ 补充：Web UI 端 `web/static/app.js` `renderVirtualKeyboard` 的 select 分支（ask_followup_question 多选项）末尾固定渲染"补充信息"选项按钮（`空格/Ins/0`，点击后 `enterSupplementMode()`），`__vkHandler` 中 select 交互也支持空格/Insert/0 进入补充输入模式（原仅 `isToolConfirm` 时显示）；补充输入模式下主输入框输入内容按 Enter 发送 `{action:"input"}` 给后端，后端 `askFollowupQuestionTool` 的 ActionInput 分支处理 [BUILD-664]；⑭ 补充：Web UI 端 `web/static/style.css` `.option-buttons` 由 `flex-wrap: wrap`（一排自适应换行）改为 `flex-direction: column`（一个选项渲染一行），提升多选项可读性 [BUILD-665]；⑮ 补充：Web UI 端 `web/static/style.css` `.ask-area`（信息提示框）设置 `max-height: 50vh` + `overflow-y: auto`，内容超过界面高度 1/2 时自动显示滚动条，避免信息过大超出显示范围 [BUILD-666]；⑯ 合并前 build 计数更新 [BUILD-667]
  - 测试：见 use-case/FEATURE-438/

---

## v0.16.0 — 开发中

> **版本**: v0.16.0

> **状态**: 🚧 开发中（Web UI 优化）
> **里程碑**: Web UI 优化
> **说明**: 0.16.0 系列专注 Web UI 优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-428 | 0.16.0 | P1 | 会话清单显示消息数：状态条会话清单每项增加消息数显示，消息数靠右右对齐，鼠标滑过提示"消息计数" |
| FEATURE-429 | 0.16.0 | P1 | 新增模型向导 Web UI 分步表单化：后端新增结构化向导接口（不再走文本流），前端渲染真正的分步表单（模板下拉/endpoint输入/API key密码框/模型名下拉/能力勾选/ID/优先级/启用开关），底部上一步/下一步/完成按钮，保留 TUI 向导不变 |
| FEATURE-430 | 0.16.0 | P1 | Web UI 绑定地址可配置：新增 --bind 命令行参数设定 Web UI 监听地址（默认 127.0.0.1），支持局域网访问 |
| FEATURE-431 | 0.16.0 | P1 | Web 服务访问白名单：新增 --whitelist 命令行参数或系统设置指定白名单（支持 IP/网段），无白名单时强制本机访问 |
| FEATURE-432 | 0.16.0 | P1 | Web UI 文件预览悬浮标题栏：文件预览顶部显示半透明悬浮标题栏（文件名全路径/最后修改时间/关闭图标），平时 25% 透明度，鼠标放上变清晰，点击路径自动定位展开工作区文件夹 |
| FEATURE-433 | 0.16.0 | P1 | Web UI 模型管理向导"2. 接口地址"步骤增加连通性测试按钮：点击测试 /models 端点连通性，按钮旁显示结果，复用 URL 自动补全 |
| FEATURE-434 | 0.16.0 | P1 | 重新设计 --help 示例：从 12 个精简为 3 个突出重点（直接执行指令 / 绑定IP+白名单的 web ui 服务模式 / 工作空间+会话ID+指令的纯 agent 调用） |
| FEATURE-435 | 0.16.0 | P1 | 文件预览标题栏改进：标题栏移到文件显示区域下方（与底边留空间）、关闭图标用高亮颜色、鼠标滑过文件显示区域下方100px区域标题栏即高亮、md 文件时在标题栏同水平位置增加 Raw 按钮开关（切换原始文本/解析渲染，默认为关） |
| FEATURE-436 | 0.16.0 | P1 | 四项 UI 改进：Raw 按钮按下后切换其他文件时保持状态、MD 文件解析后内容显示时增加行号列、自动分屏融合按钮高亮背景形状改为横向胶囊（图标横向纵向居中）、底部状态栏右侧预装/输出速度（XXXt/s）以千分位格式显示 |
| FEATURE-437 | 0.16.0 | P1 | Web UI 模型配置向导改进：① API Key 步骤增加测试按钮；② 模型名步骤改为"选择模型"并增加刷新模型列表按钮、记录模型最大长度；③ 最大上下文长度步骤提示当前模型最大长度；④ 上下文长度步骤增加"获得模型最大上下文长度"按钮（获取失败提示原因） |

> 当前 BUILD: 620
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-428 会话清单显示消息数** [BUILD-621]
  - 背景：状态条会话清单每项只显示标题，用户无法直观看到每个会话的消息数量。
  - 方案（已确认）：后端 `sessionInfo` 增加 `message_count` 字段（复用 `SessionEntry.MessageCount`），前端会话清单每项在标题右侧靠右显示消息数，鼠标滑过提示"消息计数"。
  - 需求：`web/server.go` `sessionInfo` 增加 `MessageCount` 字段；`web/session.go` `pushSessionList` 填充 `MessageCount`；`web/static/app.js` `renderSessionMenu` 渲染消息数（右对齐）+ title 提示；`web/static/style.css` 新增消息数样式；`web/static/app.js` i18n 增加"消息计数"文案。
  - 实施：`web/server.go` `sessionInfo` 增加 `MessageCount` 字段；`web/session.go` `pushSessionList` 填充 `MessageCount`；`web/static/app.js` `renderSessionMenu` 在标题后添加右对齐消息数 span（`session-count`）+ title 提示（`T.sessionCount`），i18n zh/en 增加 `sessionCount` 文案；`web/static/style.css` 新增 `.session-count` 样式（`margin-left:auto` 靠右 + `flex-shrink:0`）[BUILD-621]
  - 测试：见 use-case/FEATURE-428/

- [ ] **FEATURE-429 新增模型向导 Web UI 分步表单化** [BUILD-622]
  - 背景：FEATURE-422 的 Web 模型向导本质是"把 TUI 文本向导翻译成 Web 弹窗里的文本流 + 通用输入框"，不是真正的分步表单，用户体验差。
  - 方案（已确认）：新增一套独立的 Web 专用结构化向导接口（与现有 TUI 向导并行共存，不破坏 REPL 体验），后端复用现有逻辑（saveModel/fetchModelSuggestions/detectModelCapabilities/autoCompleteEndpoint 等），前端渲染真正的分步表单。
  - 需求：① 后端 `cmd/model_web_wizard.go` 新增结构化向导状态机（WebWizardStep 枚举 + WebWizardState + WebWizardStepData 表单描述 + WebWizardStart/Next/Prev/Submit 方法），复用现有底层逻辑；② `web/session.go` 新增 model_wizard_start/next/prev 消息；③ 前端向导弹窗改为分步表单（左侧步骤导航 + 右侧表单控件 + 底部上一步/下一步/完成按钮）；④ 保留 TUI 向导不变。
  - 实施：① `cmd/model_web_wizard.go` 新增结构化向导（WebWizardStep 枚举 + WebWizardData 前端持有 + WebWizardStepData 表单描述 + WebWizardStart/Next/Prev/Submit 方法，复用 saveModel/fetchModelSuggestions/detectModelCapabilities/autoCompleteEndpoint/modelIDExists/knownMaxModelLen，edit 分支不依赖模板存在）；② `web/server.go` clientMessage 增加 Step/WizardData 字段、serverMessage 增加 WizardStep/WizardData 字段；③ `web/session.go` 新增 model_wizard_start/next/prev/submit 消息处理（handleModelWizardStart/Next/Prev/Submit + sendWizardStep + decodeWizardData）；④ `web/static/index.html` 向导弹窗改为分步表单结构（左侧导航 + 右侧表单 + 底部按钮）；⑤ `web/static/app.js` 重写向导逻辑（wizardData/wizardStep 状态 + renderWizardStep/renderWizardField/collectWizardFields + 上一步/下一步/完成按钮 + 兼容占位函数）；⑥ `web/static/style.css` 新增分步表单样式（wizard-layout/nav/field/actions 等）；⑦ `cmd/model_web_wizard_test.go` 新增 8 个单元测试（启动/推进/回退/提交/模型拉取/能力检测）[BUILD-622]；⑧ 改进：能力勾选（vision/tool_call/thinking）和启用开关（enabled）改为胶囊开关（复用 .tool-params-raw 样式，renderWizardField 渲染胶囊按钮 + collectWizardFields 读取 .on 状态），进入能力步骤时前端显示"正在检测模型能力..." loading（modelWizardNext 在 model_name 步骤点击时先显示占位再发送 model_wizard_next），style.css 新增 .wizard-field .tool-params-raw 适配样式 [BUILD-623]；⑨ 改进：能力/启用开关改为"能拨动的开关"（toggle switch，iOS 风格滑块：圆角轨道 + 圆形滑块，checked 时滑块右移 + 轨道变色），布局横排（标签左、开关右，如 👁 视觉识别 [开关]），renderWizardField 渲染 .wizard-toggle 滑块开关 + 字段加 .toggle 类横排，collectWizardFields 读取 checkbox checked，style.css 新增 .wizard-toggle/.wizard-toggle-slider 滑块开关样式 [BUILD-624]；⑩ 优化：自动分区后悬浮接续按钮图标放大一倍（.stream-merge .merge-arrow font-size 18px→36px）；模型列表删除改用自定义确认模态框（参考会话列表，新增 delModel 模态框 + confirmDeleteModel/closeModelDeleteModal + i18n modelDeleteConfirm，renderModelsBody 删除按钮改用 confirmDeleteModel）[BUILD-625]；⑪ 优化：状态条模型列表图标显示为 18px（.model-menu-item .model-logo 32px→18px）；模型管理列表模型左边加 32px 图标（renderModelsBody 每行左侧加 .model-logo img）+ 去掉每行高亮框（.model-row 去掉 border、.model-row.enabled 去掉 accent 边框，新增 .model-row .model-logo 32px 样式）[BUILD-626]；⑫ 优化：模型管理列表模型主要信息列改为左对齐（.model-info 加 flex:1 + text-align:left）；去掉每行白点（renderModelsBody 中 id 前的 ●/○ 状态指示点去掉，因已有图标）[BUILD-627]；⑬ 优化：正在动态输出的 LLM/TOOL 块动态效果"..."改为标题行开头原点呼吸灯（addBlockActions 中 .ev-streaming textContent 从 "..." 改为 "●" 并 head.prepend 放到标题行开头，style.css .ev-streaming 改为 breathe 呼吸灯动画 + accent 色原点）[BUILD-628]；⑭ 优化：呼吸灯复用块左上角已有圆点（去掉新增 .ev-streaming 元素，markStreaming/unmarkStreaming 改为给 .ev-head 加/移除 streaming 类，style.css .ev-head.streaming::before 加 breathe 动画，两处隐藏逻辑改为移除 .ev-head.streaming 类）[BUILD-630]；⑮ 优化：命令行启动文字 logo 换成 logo.txt 内容（repl/logo.md 从 22 行 =+*#% 字符图案改为 12 行 logo.txt 贝壳图案），缩小更精致，且 # 换成白色实心矩形字符 █ 替代 [BUILD-631]；⑯ 优化：命令行 logo 用行间无间隙的色块字符渲染（repl/logo.md 用 ▀ 上半块/▄ 下半块/█ 全块组合，每 2 行像素合并为 1 个字符，12 行压缩为 6 行，色块占满整个字符高度、行间无间隙）[BUILD-632]；⑰ 优化：命令行 logo 所有空格和块都变成两个相同字符，横向扩宽 1 倍（repl/logo.md 每个字符重复 2 次）[BUILD-633]；⑱ 优化：命令行 logo 照着 web UI 左下角 logo（LOGO_ART 12x12 贝壳图案）用色块字符 █ 拼凑（repl/logo.md 用 █ 替换 #，横向扩宽 1 倍，去上下空行）[BUILD-634]；⑲ 修复：模型管理点删除后删除确认模态框被模型管理窗口覆盖在底下（delModel 和 models 都是 .modal z-index 70，delModel 在 DOM 中靠前被覆盖；style.css 给 #delModel 设 z-index:80 使其显示在 models 之上）[BUILD-635]；⑳ 优化：新建模型向导 URL 自动补全机制改进（cmd/model.go autoCompleteEndpoint 原逻辑仅在 base URL 返回 404 时才尝试 +/v1，当 base 返回网络错误（连接失败/超时/DNS 失败）时不会尝试 +/v1；改为只要 base URL 测试失败（无论 404 还是网络错误）且无 /vN 后缀就尝试 +/v1，实现 http://、https://、/v1 前后缀的多次尝试）[BUILD-636]
  - 测试：见 use-case/FEATURE-428/

- [ ] **FEATURE-430 Web UI 绑定地址可配置**
  - 背景：Web UI 的 `web.Server.Listen` 硬编码绑定 `127.0.0.1`（loopback），只能本机访问，无法被局域网内其他设备访问。
  - 方案（已确认）：新增 `--bind` 命令行参数设定 Web UI 监听地址（默认 `127.0.0.1`），支持 `0.0.0.0` 或局域网 IP 实现局域网访问。
  - 需求：`main.go` 新增 `--bind` 参数（默认 `127.0.0.1`）；`web.Server.Listen` 接受绑定地址参数（或通过 `ServerOptions` 传入）；`main.go` `startWebUI` 调用 `srv.Listen` 时传入绑定地址。
  - 实施：`main.go` 新增 `--bind` 参数（默认 `127.0.0.1`）；`web/server.go` `Listen` 方法改为接受绑定地址（`net.Listen("tcp", bind+":"+port)`）；`main.go` `startWebUI` 传入 `flags.bind`；`web/server_test.go` 新增 `TestListenBind` 单元测试（验证 127.0.0.1/0.0.0.0/默认三种绑定地址）[BUILD-638]
  - 测试：见 use-case/FEATURE-430/

- [ ] **FEATURE-431 Web 服务访问白名单**
  - 背景：Web UI 绑定地址可配置（FEATURE-430）后，若绑定 `0.0.0.0` 则局域网内任何设备都能访问，缺乏访问控制。
  - 方案（已确认）：新增 Web 服务访问白名单，支持 IP/网段（如 `192.168.1.0/24`），可通过 `--whitelist` 命令行临时指定或系统设置永久指定（config.json）。无白名单时强制本机访问（忽略 `--bind`）。
  - 需求：`config` 新增 `web_whitelist` 配置项；`main.go` 新增 `--whitelist` 参数并合并配置；无白名单时强制 `bind=127.0.0.1`；`web.Server` 增加白名单校验（支持网段）。
  - 实施：`config/config.go` `Config` 新增 `WebWhitelist []string`（`web_whitelist`）；`main.go` 新增 `--whitelist` 参数（逗号分隔 IP/网段），`startWebUI` 合并命令行与配置白名单，无白名单时强制 `bind=127.0.0.1`；`web/server.go` `ServerOptions` 新增 `Whitelist`，`NewServer` 用 `whitelistMiddleware` 包装 mux（`parseWhitelist`/`ipAllowed` 支持精确 IP 与 CIDR 网段）；`cmd/settings_web.go` `SettingsJSON` 新增 `web-whitelist` 设置项，`cmd/settings.go` `Handle` 分发 `web-whitelist` 到新增 `handleWebSetting`；i18n 新增 `KeyCol3WebWhitelist`；`web/server_test.go` 新增 `TestIPAllowed` 单元测试 [BUILD-640]；⑳ 优化：`--help` 补充 `--bind`/`--whitelist` 参数说明（`usage.go` `buildUsage` 添加 `KeyCLIHelpBind`/`KeyCLIHelpWhitelist` 行，i18n 新增对应文案）[BUILD-641]
  - 测试：见 use-case/FEATURE-431/

- [ ] **FEATURE-432 Web UI 文件预览悬浮标题栏**
  - 背景：文件预览（FEATURE-425）没有标题栏，用户无法直观看到当前查看的文件名和最后修改时间，也无法快速关闭。
  - 方案（已确认）：文件预览启动后，在文件内容顶部显示悬浮半透明标题栏，显示文件名（全路径）、最后修改时间、关闭图标。平时透明度 25%，鼠标放上后变清晰。点击路径自动定位到工作区文件所在位置（自动展开路径中的文件夹）。
  - 需求：后端 `treeNode` 增加 `Mtime` 字段；前端 `fileViewer` 内增加悬浮标题栏；`openFilePreview` 填充标题栏；点击路径展开工作区树到该文件。
  - 实施：`web/server.go` `treeNode` 增加 `Mtime` 字段，`buildTree` 为文件节点获取 `ModTime`；`web/static/index.html` `fileViewer` 内增加悬浮标题栏（`fv-titlebar`/`fvPath`/`fvMtime`/`fvClose`）；`web/static/app.js` `openFilePreview` 填充标题栏（路径/mtime），新增 `formatMtime` 辅助函数和 `revealInTree` 函数（点击路径展开工作区树到该文件），绑定关闭按钮；`web/static/style.css` 新增 `.fv-titlebar` 半透明悬浮样式（默认 25% 透明度，hover 变清晰）[BUILD-642]；⑳ 优化：标题栏改为灵动岛形式（.fv-titlebar 两端圆形胶囊形，position:absolute 悬浮在正文上部，平时背景透明+边框文字 50% 透明，hover 背景变 50% 半透明+字体边框完全清晰，最大程度节约空间不干扰浏览）[BUILD-643]；修复点击文件标题定位工作区时丢失高亮（revealInTree 改为 async，await loadTree() 后再 highlightTreeFile，确保树渲染完成后高亮）[BUILD-644]
  - 测试：见 use-case/FEATURE-432/
- [ ] **FEATURE-433 Web UI 模型管理向导 endpoint 连通性测试按钮**
  - 背景：模型管理向导"2. 接口地址"步骤只能手动输入 endpoint，无法快速验证其连通性。
  - 方案（已确认）：在 endpoint 步骤增加"测试连通性"按钮，点击后测试 /models 端点连通性，按钮旁显示结果，复用 URL 自动补全（autoCompleteEndpoint）。
  - 需求：后端新增测试 API；前端 endpoint 步骤增加按钮和结果展示。
  - 实施：`cmd/model.go` 新增导出函数 `TestEndpointConnectivity`（复用 `autoCompleteEndpoint` 自动补全）；`web/server.go` 新增 `POST /api/test-endpoint` 路由和 `handleTestEndpoint` handler；`web/static/app.js` `renderWizardField` 在 endpoint 字段旁增加"测试连通性"按钮（点击调用 API 显示结果）；`web/static/style.css` 新增 `.wizard-endpoint-test`/`.wizard-test-btn`/`.wizard-test-result` 样式 [BUILD-645]；⑳ 优化：测试按钮配色与其他按钮一致（`.wizard-test-btn` 改为 accent-dim 背景 + accent 文字 + accent 边框，hover 变 accent 背景，深色风格适配）[BUILD-646]
  - 测试：见 use-case/FEATURE-433/

- [ ] **FEATURE-434 重新设计 --help 示例**
  - 背景：`--help` 下示例过多（12 个），信息冗余，重点不突出，用户难以快速找到常用用法。
  - 方案（已确认）：精简为 3 个重点示例：① 直接执行指令；② 启动配置绑定 IP 和网络白名单的对其他主机提供 web ui 的服务模式；③ 同时指定工作空间、会话 ID、指令（用于其他程序调用 co-shell 的纯 agent 应用）。
  - 需求：`usage.go` `buildUsage` 示例部分从 12 个精简为 3 个；`i18n`（zh/en）更新 `KeyCLIHelpEx1`/`Ex2`/`Ex3` 文案，移除 `Ex4`-`Ex12` 引用。
  - 实施：`usage.go` `buildUsage` 示例部分改为只输出 3 个示例（`KeyCLIHelpEx1`/`Ex2`/`Ex3`）；`i18n/zh.go`/`en.go` 更新 `KeyCLIHelpEx1`（直接执行指令）、`KeyCLIHelpEx2`（--serve --bind --whitelist web ui 服务模式）、`KeyCLIHelpEx3`（-w -s 指令纯 agent 调用）文案 [BUILD-648]
  - 测试：见 use-case/FEATURE-434/

- [ ] **FEATURE-435 文件预览标题栏改进**
  - 背景：文件预览（FEATURE-425/432）的悬浮标题栏位于文件显示区域顶部，用户需精确滑过标题栏才能高亮（区域太小不易操作）；关闭图标颜色不突出；md 文件缺少 Raw 开关切换原始文本/解析渲染。
  - 方案（已确认）：① 标题栏移到文件显示区域下方，与底边留空间；② 关闭图标用高亮颜色；③ 鼠标滑过文件显示区域下方 100px 区域标题栏即高亮（不必精确滑到标题栏）；④ md 文件时在标题栏同水平位置增加与标题栏风格类似的 Raw 按钮开关（切换原始文本/解析渲染），默认为关。
  - 需求：`web/static/index.html` `fileViewer` 内标题栏移到下方 + 增加 Raw 按钮；`web/static/app.js` 标题栏高亮触发区域改为文件显示区域下方 100px、Raw 开关控制 md 渲染；`web/static/style.css` 标题栏移到下方、关闭图标高亮色、Raw 按钮样式。
  - 实施：`web/static/index.html` `fileViewer` 内标题栏移到 `fv-body` 之后（底部）+ 增加 Raw 按钮；`web/static/style.css` `.fv-titlebar` 改为 `bottom:12px`（与底边留空间）、`.fv-close` 改高亮色 `var(--accent)`、新增 `.fv-titlebar.active` 高亮状态 + `.fv-raw` Raw 按钮样式；`web/static/app.js` 新增 `fvRaw` 状态（默认 false）、`loadFileChunk` md 判断改为 `isMdFile(path) && !fvRaw`、`openFilePreview` 按是否 md 文件显示/隐藏 Raw 按钮、`fvRawEl.onclick` 切换 Raw 并重载文件、`fileViewer` mousemove 监听底部 100px 区域控制标题栏 `.active` 高亮、`closeFileViewer` 重置 fvRaw [BUILD-650]
  - 测试：见 use-case/FEATURE-435/

- [x] **FEATURE-436 四项 UI 改进**
  - 背景：① Raw 按钮按下后切换其他文件时状态被重置（FEATURE-435 每次打开文件都重置 fvRaw=false）；② MD 文件解析后的内容显示时没有行号列；③ 自动分屏融合按钮高亮背景形状与图标不对称；④ 底部状态栏右侧预装/输出速度（XXXt/s）未用千分位格式。
  - 方案（已确认）：① Raw 状态用 localStorage 持久化，切换文件时保持；② MD 解析渲染时也显示行号列；③ 融合按钮高亮背景形状改为横向胶囊，图标横向纵向居中；④ 状态栏速度以千分位格式显示。
  - 需求：`web/static/app.js` Raw 状态持久化 + md 行号 + 状态栏千分位；`web/static/style.css` 融合按钮胶囊形状 + md 行号样式。
  - 实施：`web/static/app.js` Raw 状态用 localStorage 持久化（`openFilePreview` 读取 `localStorage.getItem("fvRaw")`、`fvRawEl.onclick` 保存）、`renderFileBody` 每个 md 块包裹 flex 行 + 行号列（`fv-md-row`/`fv-md-no`）、状态栏速度 `fmtNum(liTPS)`/`fmtNum(loTPS)` 千分位；`web/static/md.js` `mdBlocks` 每个块设置 `data-line` 起始行号；`web/static/style.css` `.stream-merge` 高亮背景改横向胶囊（`justify-content:center` + `padding:2px 14px`）、新增 `.fv-md-row`/`.fv-md-no` md 行号样式 [BUILD-652]；⑳ 优化：md 行号列仿照 RAW 模式，`.fv-md-no` 添加 `border-right` 分割线 + `margin-right` 间隙 + `padding-right`（宽度 3em→4.5em、颜色 `--fg-dim`→`--fg-faint`）[BUILD-653]；㉑ 规范修订：`.rules/PROJECT STANDARDS.md` 编译可执行码章节——无参数编译命名不变（co-shell）且需递增 build no；带参数编译生成文件命名规范 `co-shell-{version}-{os}-{arch}[.exe]` 且不用递增 build no [BUILD-654]
  - 测试：见 use-case/FEATURE-436/

- [x] **FEATURE-437 Web UI 模型配置向导改进** ✅ 已完成
  - 背景：模型配置向导（FEATURE-429）需要更便捷的配置体验：① API Key 步骤无法手动验证 key 有效性；② 模型名步骤无法刷新模型列表，且选择模型后无法记录其最大上下文长度；③ 最大上下文长度步骤未提示当前模型的最大长度；④ 上下文长度步骤无法一键获取模型最大长度。
  - 方案（已确认）：① API Key 步骤增加"测试 API Key"按钮（后端 `TestAPIKey` + `/api/test-api-key`）；② 模型名步骤改为"选择模型"并增加"刷新模型列表"按钮（新增 `model_wizard_refresh` 消息 + `WebWizardRefresh`），`WebWizardStepData.ModelMaxLens` 记录各模型最大长度，前端选择模型时记入 `wizardData.model_max_len`；③ max_model_len 步骤用 `data.ModelMaxLen` 作为默认值并显示 hint；④ 上下文长度步骤增加"获得模型最大上下文长度"按钮（后端 `GetModelMaxLen` + `/api/get-model-max-len`，获取失败提示原因）。
  - 需求：`cmd/model.go` `TestAPIKey`/`GetModelMaxLen`；`cmd/model_web_wizard.go` `WebWizardRefresh`/`ModelMaxLens`/`ModelMaxLen`/`Message`；`web/server.go` `/api/test-api-key`/`/api/get-model-max-len`；`web/session.go` `model_wizard_refresh`；`web/static/app.js` 按钮与渲染；`web/static/style.css` 样式；`i18n` 新增 `KeyCmdMig_382`。
  - 实施：`cmd/model.go` 新增 `TestAPIKey`/`GetModelMaxLen`、`fetchModelSuggestions` 增加 error 返回值；`cmd/model_web_wizard.go` 新增 `WebWizardRefresh`、`WebWizardStepData.ModelMaxLens`/`Message`、`WebWizardData.ModelMaxLen`、max_model_len 默认值优先用 `ModelMaxLen` 并显示 hint；`web/server.go` 新增 `/api/test-api-key`/`/api/get-model-max-len`；`web/session.go` 新增 `model_wizard_refresh` 处理；`web/static/app.js` 步骤标题改"选择模型"、API Key 测试按钮、刷新模型列表按钮、记录模型 max len、max_model_len 获取按钮与错误提示；`web/static/style.css` `.wizard-step-message` 样式；`i18n` 新增 `KeyCmdMig_382`（选择模型）[BUILD-658]
  - 测试：见 use-case/FEATURE-437/

---

## v0.15.0 — 开发中

> **版本**: v0.15.0

> **状态**: 🚧 开发中（启动模式设计语言重构）
> **里程碑**: 启动模式设计语言重构
> **说明**: 0.15.0 系列重构启动模式设计语言，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-427 | 0.15.0 | P1 | 重构启动模式设计语言：默认 web UI 模式（自动弹浏览器，无浏览器降级 enhanced→stdio），--serve 启动 web UI 不弹浏览器，指令参数强制 stdio，移除 serve 子命令 |

> 当前 BUILD: 620
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-427 启动模式设计语言重构** [BUILD-620]
  - 背景：当前启动模式默认 enhanced REPL，serve 为子命令且自动弹浏览器，模式优先级与设计语言不一致。
  - 方案（已确认）：重构为三种工作模式——模式 A（`--input-mode enhanced`，REPL 增强模式，终端最佳体验）、模式 B（`--input-mode stdio`，REPL 标准输入输出，最大兼容性，输入指令参数时自动使用）、模式 C（`--serve`，web UI 模式，不自动弹浏览器）。默认优先级调整为：默认 web UI 模式（不能录入指令）→ 自动弹浏览器 → 无浏览器降级 enhanced → 环境不支持 enhanced 降级 stdio。移除 `serve` 子命令。指令参数强制 stdio，若同时指定其他模式参数则报错。
  - 需求：修改 `main.go` 启动逻辑与 `usage.go` --help 说明。
  - 测试：见 use-case/FEATURE-427/

---

## v0.14.1 — 开发中

> **版本**: v0.14.1

> **状态**: 🚧 开发中（极简模式收起逻辑修复）
> **里程碑**: 极简模式收起逻辑修复
> **说明**: 0.14.1 系列专注极简模式收起逻辑修复，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-426 | 0.14.1 | P1 | 修复极简模式收起逻辑：当有块开始动态输出时收起其他未收起的块（用户块除外），用户块总是展开显示原始指令 |

> 当前 BUILD: 613
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FIX-426 极简模式收起逻辑修复**
  - 背景：极简模式下，`unmarkStreaming` 在块停止流式输出时无条件收起该块（含用户块），且未考虑"当前最后一块"——当块 A 停止输出且无后续块时，块 A 被收起导致主消息区无展开块；用户块也被收起，用户看不到原始指令。
  - 方案（已确认）：极简模式下，当有块开始动态输出内容时，把其他未收起的块收起（用户块除外）；用户块总是展开显示原始指令。
  - 需求：修改 `web/static/app.js` 的 `unmarkStreaming`/`applyBlockDisplayMode`，极简模式下收起非用户块、非当前输出块，用户块（user-msg）始终展开。
  - 实施：`web/static/app.js` ① `markStreaming` 极简模式下当有块开始流式输出时收起其他非用户块（用户块始终展开）；② `unmarkStreaming` 极简模式下仅当有其他块仍在流式输出时才收起当前块（最后一个块保持展开），用户块/结果块不收起；③ `applyBlockDisplayMode`/`applyDisplayMode` 极简模式下用户块始终展开 [BUILD-606]；④ 修复 TOOL 块不被自动收起：`.ev.collapsed` 原只隐藏 `.ev-body`，但 TOOL 块的输入参数子块 `.tool-params` 是 `.ev` 直接子元素不在 `.ev-body` 内，导致收起时输入参数仍显示。修改 `.ev.collapsed .tool-params` 也 `display:none`，使 TOOL 块被新输出块触发收起时输入参数和结果都隐藏只剩标题 [BUILD-607]；⑤ 修复静默模式最终结果块不显示：`applyBlockDisplayMode`/`applyDisplayMode` 静默模式下最终结果块（ev-result）不仅显示且移除 `collapsed` 展开，确保用户看到任务完成报告；⑥ 运行开始时会话标题栏高亮点和 co-shell logo 呼吸效果：`setRunning(true)` 给 `.stream-active` 和 `#logo` 添加 `breathing` class（CSS `breathe` 动画明暗交替），`setRunning(false)` 移除恢复正常 [BUILD-608]；⑦ 修复极简模式最终结果块被收起：`markStreaming` 极简模式下收起其他块时排除 `ev-result`，使最后一个 TOOL: 完成任务 块保持展开 [BUILD-609]；⑧ 主消息区滚动自动分区后的悬浮融合按钮图标改为 `⎶`（移除原 ↓+横线组合，图标放大到 18px 与按钮比例协调）[BUILD-610]；⑨ 修复"完成任务"TOOL 块 ev-result 标记：`tool_call` 事件处理中，当 TOOL 块标题更新为 "TOOL: 完成任务" 时标记 `ev-result` 并移除 `collapsed`（因 `makeBlock` 创建时 label="TOOL" 无法判断是否完成任务），确保静默模式下最终结果块显示且展开 [BUILD-611]
  - 测试：见 use-case/FIX-426/

---

## v0.14.0 — 开发中

> **版本**: v0.14.0

> **状态**: 🚧 开发中（文本文件只读预览）
> **里程碑**: 文本文件只读预览
> **说明**: 0.14.0 系列专注 Web UI 文本文件只读预览，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-425 | 0.14.0 | P1 | 文本文件只读预览：工作区单击受支持文本文件（源代码/txt/md/csv/shell脚本/配置文件等）在弹窗中显示（双击仍用系统打开），单文件快速切换，弹窗覆盖主消息区90%以上且不覆盖工作区/任务进展/录入框并自动缩放，>100K 文件按需预读，Go 语法高亮，git 未提交修改按 diff 高亮 |

> 当前 BUILD: 595
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-425 文本文件只读预览** ✅ 已完成
  - 背景：Web UI 工作区文件目前只能双击用系统程序打开，无法在界面内直接查看文本文件内容。需要支持在弹窗中只读预览文本文件，带语法高亮和 git diff 高亮。
  - 方案（已确认）：① 工作区文件行单击预览文本文件（双击仍用系统打开）；② 单文件显示，点击新文件立即替换，重复点选当前文件无反应；③ 弹窗绝对定位覆盖主消息区（#stream-wrap）内部，不覆盖工作区/任务进展/录入框，窗口变化自动缩放；④ >100K 文件后端按行范围读取、前端滚动按需预读；⑤ 前端 JS 正则实现 Go 语法高亮（关键字/方法/字符串/变量），参考 vscode 配色；⑥ git 版本控制下后端返回未提交修改行级数据，前端按 diff 高亮。
  - 需求：修改 `web/server.go`（新增按行范围读取 + git diff 行级数据 API）、`web/static/index.html`（文本查看器弹窗 DOM）、`web/static/app.js`（单击预览、单文件切换、滚动加载、语法高亮、diff 高亮）、`web/static/style.css`（弹窗布局 + 高亮配色）。
  - 实施：① `web/server.go` 新增 `serveFileLines`（按行范围读取，bufio.Scanner 流式读不整载入内存，返回 `{total, lines}`）、`handleGitDiff`（`git diff -- <path>` 解析未提交修改行级数据）、`parseGitDiff`（解析 unified diff hunk，返回 add/del 行号）、`gitDiffLine` 结构，注册 `/api/gitdiff` 路由，`handleFile` 支持 start/end 参数；② `web/static/index.html` 在 `#stream-wrap` 内部新增 `#fileViewer` 文本查看器弹窗（绝对定位覆盖主消息区，不覆盖工作区/任务进展/录入框）；③ `web/static/app.js` 新增 `openFilePreview`（单击预览，单文件切换，重复点选无反应）、`loadFileChunk`（按行范围滚动加载）、`loadFileDiff`（git diff 高亮，只保留 add 状态）、`highlightCode`（Go 语法高亮正则分词）、`renderFileLine`（行号 + 高亮 + diff 着色），文件行单击预览、双击系统打开；④ `web/static/style.css` 新增 `.file-viewer`（inset:6px 绝对定位自动缩放）、`.fv-line`/`.fv-no`/`.fv-code`、`.fv-add`（绿）/`.fv-del`（红）、Go 语法高亮配色（tok-kw/tok-str/tok-num/tok-com/tok-fn/tok-type/tok-var/tok-pkg）；⑤ `web/filepreview_test.go` 新增按行读取 + git diff 解析单元测试 [BUILD-596]；⑥ 6 点改进：预览窗口完全覆盖主消息区（inset:0 无边框圆角，标题栏与工作区/任务进展栏同高度对齐）、标题栏增加文件内搜索框（fvSearch 高亮匹配行）、高亮工作区当前选择的文件（fv-selected）、md 文件增加 Raw 胶囊开关（默认关自动渲染 md）、JSON 属性/值/{}[] 两种颜色（tok-prop/tok-brace）+ java/python/go 注释统一绿色、浅色主题下变量名和数字改深色 [BUILD-597]；⑦ 3 点新改进：修复大 md 文件（如 ROADMAP.md）无法渲染（md 渲染改为按需加载 + 累积重渲染，md.js 每次重新解析累积文本保证当前视口完整）、Raw 模式持久化（localStorage 记住选择，打开下一个文件保持状态）、不认识后缀名的文件先按文本打开出现控制字符则按 HEX 显示（后端新增 `serveFileHex` 按字节范围读取返回 hex 行，前端 `loadFileHex`/`renderHexRow` 按需加载）[BUILD-598]；⑧ 3 点新改进：md 表格渲染后加边框（`.fv-body.md table` 边框样式）、工作区 git 修改状态字母与文件名水平对齐（`.git-status` line-height 调整为 19px）、HEX 方式根据窗口宽度自适应显示 8/16/32/64/128 字符一行（后端 `serveFileHex` 支持 width 参数，前端 `fitHexWidth` 计算 + resize 监听重新加载）[BUILD-599]；⑨ 2 点新改进：git 状态字母与文件名分毫不差对齐（git-status 从 li 绝对定位改为 tree-row flex 子元素，align-items:center 精确对齐，centerDiff=0）、文件预览标题行高度与工作区/任务进展栏一致（`.fv-head` 固定 height:36px + box-sizing:border-box，搜索框 height:22px 不撑高标题行）[BUILD-600]；⑩ 3 点新改进：文件预览标题栏再缩小 1 像素（`.fv-head` height 36→35px）、git 状态字位置再向上调 1 像素（`.git-status` transform:translateY(-1px)）、HEX 自适应宽度修正（`fitHexWidth` 准确计算每行总宽度=偏移+hex+ascii，以内容不超出显示区域且单行字节数最大的方式显示，960px→16字节/行、300px→8字节/行）[BUILD-601]；⑪ 4 点新改进：再次点选同一文件自动关闭预览（`openFilePreview` 检测 fvPath===node.path 时调用 `closeFileViewer`）、主消息区增加标题栏（`.stream-head` 高度 35px 与工作区/任务进展栏对齐，`.file-viewer` 从标题栏下方开始）、主消息区标题为当前会话标题可直接修改（后端新增 `session_rename` 消息 + `renameSession` 方法调用 `UpdateNamedSession`，前端 `streamTitle` 输入框 Enter/blur 提交）、三段模式选择胶囊（静默/极简/正常，`displayMode` localStorage 持久化，静默只显示用户指令/错误/结果块，极简块完成后自动收起只剩标题，正常与现在一样）[BUILD-602]；⑫ 2 点新改进：主消息区标题栏模式选择放到右侧并改用与 act/plan/research 切换一样的 `.mode-seg` 胶囊控件（带滑动高亮滑块，`.stream-mode` 设 `margin-left:auto` 靠右，`initStreamMode` 适配滑块移动）、会话标题靠左放置（`.stream-title` text-align:left），前端控件使用规范补充 `.mode-seg` 复用场景说明 [BUILD-603]；⑬ 2 点新改进：去掉文件预览的标题行（移除 `.fv-head` 及 fvTitle/fvSearch/fvRaw/fvClose 控件和相关 JS/CSS，md 文件始终自动渲染）、主消息区标题栏标题前加高亮的点（`.stream-active` ● 用 accent 色，同会话列表当前会话点图标）[BUILD-604]；合并到 main 并打版本标签 v0.14.0 [BUILD-605]
  - 测试：见 use-case/FEATURE-425/

---

## v0.13.0 — 开发中

> **版本**: v0.13.0

> **状态**: 🚧 开发中（文件 diff 显示）
> **里程碑**: 文件 diff 显示
> **说明**: 0.13.0 系列专注文件 diff 显示优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-424 | 0.13.0 | P1 | 文件写入/覆盖 diff 显示：所有工作模式（write_to_file 新建/覆盖/追加、replace_in_file 指定行号/不指定行号）统一带行号 + 状态标记（+/-/空格），前后端新增协议区分新增/删除内容，Web UI 新增内容绿色、删除内容红色 |

> 当前 BUILD: 582
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-424 文件写入/覆盖 diff 显示**
  - 背景：当前前端显示的文件修改内容，哪些是增加、哪些是删除/覆盖看不清楚。需要统一所有工作模式（write_to_file 新建/覆盖/追加、replace_in_file 指定行号/不指定行号）的 diff 显示，带行号 + 状态标记，Web UI 用颜色区分新增/删除。
  - 方案（已确认）：① 后端统一所有工作模式的渲染格式：左对齐、开头空 1 格、5 位右对齐行号、`+`/`-`/` ` 三态（新增/删除/不变），后跟空格再跟内容；② `replace_in_file` 对 search/replace 做逐行 diff，相同行标 ` `（不变）；③ 前后端新增协议：每行新增一个状态字段（默认可为空，不特殊处理），后端输出时携带，前端据此渲染；④ Web UI 各信息块新增内容绿色、删除内容红色。
  - 需求：修改 `agent/toolcall_renderop.go`（统一渲染格式 + 逐行 diff + 状态字段）、`web/session.go`（协议携带状态字段）、`web/static/app.js`（解析状态字段渲染颜色）、`web/static/style.css`（新增/删除颜色样式）。
  - 实施：① `agent/toolcall_renderop.go` 新增 `buildDiffText` 逐行 diff（LCS 对齐，相同行标 ` `、仅 search 标 `-`、仅 replace 标 `+`，统一格式 `{1空格}{5位右对齐行号}{状态}{空格}{内容}`），`replaceSearchAccum`/`replaceReplaceAccum` 累积完整 search/replace 内容，`finaliseParameter` 在 replace 结束时计算 diff 存入 `diffText`，`emitToolEnd` 通过 `diffEmit` 回调发送；② `agent/events.go` 新增 `EventToolCallDiff` 事件类型；③ `agent/stream_response.go` 创建渲染器时设置 `diffEmit` 回调发送 `tool_call_diff` 事件；④ `web/static/app.js` 新增 `tool_call_diff` 事件处理（用 diff 文本替换 `params.raw` 并设置 `params.diff`），`renderParams` 增加 diff 渲染分支，新增 `renderDiff` 逐行解析状态标记着色；⑤ `web/static/style.css` 新增 `.diff-row`/`.diff-add`（绿）/`.diff-del`（红）/`.diff-ctx` 样式；⑥ `agent/toolcall_diff_test.go` 新增 diff 单元测试 [BUILD-583]；⑦ write_to_file 意图渲染统一为 `(<intent>)` 格式（与 replace_in_file 一致），`writeIntent` 累积意图值，content 参数到达时在 content 前显示 `(<intent>)`，新增 `agent/toolcall_intent_test.go` 验证 write_to_file/replace_in_file 意图渲染 [BUILD-584]；⑧ 意图渲染统一为通用方案：所有工具（read_file/write_to_file/execute_command 等）的 intent 参数统一显示为 `(<intent>)`，`intent` 字段累积意图值，`finaliseParameter` 在 intent 参数结束时显示 `(<intent>)`，不再显示为 `intent:` 参数行 [BUILD-585]；⑨ 验证前端生效：重新编译（BUILD-586）并重启 serve 服务，确认前端 JS/CSS 正确加载（`renderDiff`/`renderParams` 函数已定义、`.diff-add`/`.diff-del`/`.diff-ctx` 样式存在），TOOL 块标题显示 `TOOL: <action> - <intent>`，RAW 模式显示输入参数，diff 新增绿色/删除红色计算样式正确 [BUILD-586]；⑩ 多工具调用分块：一次迭代 LLM 调用多个工具时，后端 `tool_call_stream` 事件流中每个工具以 `⚙️ <tool>\n` 标记开头，前端 `tool_call_stream` 处理检测该标记，遇到就新建 TOOL 块（避免多个工具参数累积到第一个块导致覆盖），并用 `indexOf` 剥离 `⚙️` 标题行（正则对 emoji U+2699+U+FE0F 不生效，改用 indexOf），Node.js 模拟验证 2 工具调用正确分块且无 `⚙️` 泄漏 [BUILD-588]；⑪ 修复 replace_in_file 分块：replace_in_file 的 `⚙️` 标记原延迟到 replacements 参数才发送（path/intent 先到导致前端误分块），改为 `OpToolStart` 时立即发送 `⚙️ replace_in_file\n`（不含 path），path 作为普通参数行显示，`flushReplaceHeader` 只发送一次 intent 行（emit 后清空 `replaceIntent`），更新 `TestToolCallStream_XMLReplaceNoLineNo` 测试，完整 agent 测试套件通过 [BUILD-589]；⑫ write_to_file 内容颜色区分：write_to_file 的 content 行累积到 `diffText`，`emitToolEnd` 时也发送 `tool_call_diff` 事件（内容全为新增→绿色），前端 `renderDiff` 兼容两种格式（replace_in_file 的 `{1空格}{5位行号}{状态} {内容}` 状态在 index 6，write_to_file 的 `{5空格}{5位行号}+ {内容}` 状态在 index 10），新增 `TestToolCallDiff_WriteToFile` 测试，Node.js 模拟验证 write_to_file 全绿、replace_in_file 三态正确 [BUILD-590]；⑬ 按原方案重构为结构化状态字段：原方案要求"每行新增一个状态字段"，但 BUILD-590 实际实现是"发送带状态标记的纯文本，前端解析字符位置"（依赖 `charAt(6)`/`charAt(10)`，脆弱且未按方案）。重构为：`agent/toolcall_renderop.go` 新增 `ToolDiffLine{Line, Status}` 结构，`diffEmit` 类型改为 `func([]ToolDiffLine)`，`buildDiffText` 返回 `[]ToolDiffLine`（每行携带 `add`/`del`/`ctx` 状态），`diffLines` 累积结构化数据，`emitToolEnd` 传 `[]ToolDiffLine`；`agent/events.go` 新增 `MetaKeyDiffLines` 常量；`agent/stream_response.go` 的 `diffEmit` 回调把 `[]ToolDiffLine` 序列化为 JSON 放入 `tool_call_diff` 事件的 `Meta[MetaKeyDiffLines]`；`web/static/app.js` 的 `renderDiff` 改为直接读取每行 `status` 字段着色（不再解析字符位置），`tool_call_diff` 事件处理解析 `ev.meta.diff_lines` 存入 `params.diffLines`；更新 `toolcall_diff_test.go` 适配新类型（用查找方式检查每行状态，不依赖 LCS 对齐顺序）。完整测试套件通过 [BUILD-591]；⑭ 修复 ToolDiffLine 缺少 json tag：`ToolDiffLine{Line, Status}` 无 json tag，`json.Marshal` 输出大写字段名 `{Line, Status}`，而前端 `renderDiff` 读取小写 `item.line`/`item.status`，导致 status 读取失败（全变默认色）且行文本为空。给 `ToolDiffLine` 添加 `json:"line"`/`json:"status"` tag，浏览器实测 diff 行正确着色 [BUILD-592]；⑮ 修复 JSON 模式 tool_call_diff 事件不发送：JSON parser 不产生 `OpToolEnd`，`emitToolEnd` 不被调用，`tool_call_diff` 事件不发送。在 `stream_response.go` 的 `StreamEventToolCall` 事件处理中触发 `emitToolEnd` [BUILD-593]；⑯ 修复 JSON 模式 start_line 数字值没被解析：JSON parser 对数字值（如 `start_line: 5`）走 `default` 分支累积到 buffer 但不产生 `OpValueFragment`，导致 `replaceStartLine` 为 0 行号总是 1。在 `,` 分隔符时处理 buffer 产生 `OpValueFragment` 和 `OpParamEnd`；同时将 `buildDiffText` 调用延迟到 `emitToolEnd`（`replace` 参数结束时 `start_line` 还没解析），确保行号正确。新增 `TestToolCallDiff_JSONReplace` 测试验证删除行/新增行/行号 [BUILD-594]；⑰ 工作区文件打开改为双击：`web/static/app.js` 的 `treeNode` 文件行 `row.onclick` 改为 `row.ondblclick`，避免单击误触打开系统程序，目录行单击展开/折叠保持不变 [BUILD-595]
  - 测试：见 use-case/FEATURE-424/

---

## v0.12.1 — 开发中

> **版本**: v0.12.1

> **状态**: 🚧 开发中（Web UI 优化）
> **里程碑**: Web UI 优化
> **说明**: 0.12.1 系列专注 Web UI 优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-423 | 0.12.1 | P1 | Web UI 主消息区自动分割后自动融合：自动分割后，当滚动条下滚（页面上滚）B 区已显示到底、且 A 区高度未达最高限（上下内容刚好接上）时，触发自动融合（相当于自动点浮动融合按钮） |

> 当前 BUILD: 585
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-423 Web UI 主消息区自动分割后自动融合** ✅ 已完成
  - 背景：FEATURE-416 实现了主消息区自动分割（用户上滚时 B 区静态、A 区动态）。当前分割后需手动点击浮动融合按钮才能合并。希望当用户下滚到 B 区底部、且 A 区内容未填满（上下内容刚好接上）时，自动触发融合，减少手动操作。
  - 方案（已确认）：在 `streamB` 的 scroll 事件中，当 `splitActive` 为 true 时，检测 B 区是否滚动到底部（`scrollTop + clientHeight >= scrollHeight - 4`），满足则自动调用 `mergeStream()`。只判断上半区（B 区）是否到底，不关心下半区（A 区）状态，以保证实际效果可靠且一致。
  - 需求：修改 `web/static/app.js` `streamB` scroll 事件处理，增加自动融合逻辑。
  - 实施：`web/static/app.js` `streamB` scroll 事件在 `splitActive` 时检测 B 区到底（`scrollTop + clientHeight >= scrollHeight - 4`），满足则自动调用 `mergeStream()` [BUILD-578]；状态栏 token 单次用量统计图标从循环符号 `🔄` 改为计时器符号 `⏱️`（`sbLast`，zh/en 两处）[BUILD-579]；自动融合机制简化为只判断 B 区到底（去掉 A 区未填满条件），保证效果可靠一致 [BUILD-580]；模型选择清单标题右边增加当前模式胶囊（`buildModelMenuTitle` 辅助函数，标题行 flex 布局 + `.model-menu-mode` 胶囊高亮样式，主模型/视觉模型两个菜单均显示）[BUILD-581]
  - 测试：见 use-case/FEATURE-423/

---

## v0.12.0 — 开发中

> **版本**: v0.12.0

> **状态**: 🚧 开发中（Web UI 优化）
> **里程碑**: Web UI 优化
> **说明**: 0.12.0 系列专注 Web UI 优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-419 | 0.12.0 | P1 | Web UI 三项优化：会话列表展开时默认滚动到当前会话、工作区状态每次迭代（显示块完成后去掉"..."时）刷新、快捷键收集阶段没收全部按键（不再漏给录入框） |
| FIX-420 | 0.12.0 | P1 | 修复问题判定模型（problem solver）调用失败：thinking 模型（deepseek-v4-flash）在 thinking 模式下不支持 tool_choice，SetThinkingEnabled(false) 无效（Chat 不读取该字段），需通过 thinking adapter 注入 disabled 参数 |
| FEATURE-421 | 0.12.0 | P1 | Web UI 信息块标题行文字增加约 2px 阴影，解决部分字在某些场景下不明显的问题 |
| FEATURE-422 | 0.12.0 | P1 | Web UI 模型配置界面：状态条模型选择器（hover 展开主模型/视觉模型列表，可切换，含"默认"选项和"+"新增入口）+ 模型管理弹窗 + 专用向导 UI（复用 REPL 向导状态机，通过 WebIO 驱动，新增 __CANCEL__ 哨兵值实现任意步骤退出兜底） |

> 当前 BUILD: 552
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-419 Web UI 三项优化**
  - 背景：Web UI 存在三个体验问题：① 状态条会话列表展开时未滚动到当前会话，用户需手动查找；② 工作区文件/分支状态只在任务全部结束后刷新，未在每次迭代（显示块完成后去掉"..."时）更新；③ 快捷键收集阶段只没收了数字和字母，其他符号（如 !@#$%^&*() 等）会漏给录入框，用户需反复删除无用内容。
  - 方案（已确认）：① `renderSessionMenu` 渲染会话列表后，菜单展开时滚动到当前会话项；② `token_iter`/`token_task` 事件（去掉"..."处）增加 `refreshBranch()` + `loadTree()` 刷新工作区状态；③ 快捷键收集阶段（interaction pending 且非 supplement 模式）没收全部按键，无论是否触发下一步操作都不再返还给页面。
  - 需求：修改 `web/static/app.js`：① `renderSessionMenu` 增加滚动到当前会话逻辑；② `token_iter`/`token_task` 分支增加工作区刷新；③ `__vkHandler` 与 input keydown 处理改为没收全部按键。
  - 实施：`web/static/app.js` ① `renderSessionMenu` 渲染后 `scrollIntoView` 滚动到当前会话项；② `token_iter`/`token_task` 分支（去掉"..."处）增加 `refreshBranch()` + `loadTree()`；③ `__vkHandler` 与 input keydown 在 interaction pending 且非 supplement 模式时对所有按键 `preventDefault()` 没收 [BUILD-553]；④ token 统计信息追加到当前迭代最后一个块底部（含迭代序号、时间、千分位，新增 `iterCount`/`fmtTime`）；⑤ `renderUserEcho` 用 `requestAnimationFrame(scrollStream)` 确保滚动到底部，避免输入回车误触发自动分区 [BUILD-554]；⑥ token 统计信息改为追加到本次迭代产生的每个块（LLM/THINK/TOOL/REPL）底部，新增 `iterBlocks` 列表追踪本次迭代所有块，`token_iter` 遍历列表把 token 行追加到每个块底部（而非仅最后一个块）[BUILD-556]；⑦ token 行改为放到消息块外面（`.ev` box 之后作为兄弟节点，`b.parentElement.after(line)`），而非块内部 [BUILD-557]；⑧ token_task（任务汇总）改为追加到任务最后一个信息块后面（新增 `lastBlock` 追踪任务最后一个块），格式改为 `Σtotal (↑prompt ↓completion)` [BUILD-558]；⑨ token_task 汇总行改为追加到 stream 容器末尾（`lastBlock.parentElement.parentElement.appendChild`），确保顺序为 块→迭代token行→汇总行（先迭代再汇总）[BUILD-559]；⑩ token_iter 迭代序号改用 context 消息序号（`msgIndex`，后端附加），而非前端自增 `iterCount`（`msgIndex` 不存在时回退 `iterCount`）[BUILD-560]；⑪ token_iter 迭代序号改用 `:context` 命令的消息序号（`messages` 数组索引），后端新增 `emitTokenIter` 辅助方法在事件中附加 `ctx_index`（`len(a.messages)-1`），前端使用 `ctx_index` 作为序号 [BUILD-561]；⑫ 块边界悬浮导航图标：当一个块高度大于屏幕高度时，块标题行不可见则在主数据区顶部显示悬浮向上图标（点击直达块标题行），块底部不可见则在主数据区底部显示悬浮向下图标（点击直达块底部），新增 `updateBlockNav`/`bindBlockNav` 监听 `streamB`/`streamA` 滚动确定当前块 [BUILD-562]；合并 [BUILD-563]
  - 测试：见 use-case/FEATURE-419/

- [ ] **FIX-420 修复问题判定模型（problem solver）调用失败**
  - 背景：日志显示 `judgeLoop: problem solver call failed: Thinking mode does not support this tool_choice`。问题判定模型 `deepseek-v4-flash` 是 thinking 模型（默认开启思考），DeepSeek API 在 thinking 模式下不支持 `tool_choice` 参数。`callProblemSolver` 中 `SetThinkingEnabled(false)` 无效（`Chat` 方法不读取 `thinkingEnabled` 字段），且 `SetBodyAdditions` 只注入了 `tool_choice`，未注入 `{"thinking":{"type":"disabled"}}`，导致请求体同时带 thinking 和 tool_choice → 400 错误 → judgeLoop 返回 nil → 回退到直接循环反馈。
  - 方案（已确认）：`agent/problem_solver.go` `callProblemSolver` 移除无效的 `SetThinkingEnabled(false)`，改用 `llm.GetThinkingAdapter(modelCfg.Provider)` + `BuildAdditions(ThinkingModeDisabled)` 生成 thinking disabled 参数，与 `tool_choice` 合并到同一个 `SetBodyAdditions`。
  - 实施：`agent/problem_solver.go` `callProblemSolver` 用 thinking adapter 注入 `{"thinking":{"type":"disabled"}}` 与 `tool_choice` 合并 [BUILD-555]
  - 测试：见 use-case/FIX-420/

- [ ] **FEATURE-421 Web UI 信息块标题行文字增加阴影**
  - 背景：Web UI 信息块（LLM/THINK/TOOL/REPL 等）标题行 `.ev-head` 文字颜色为 `var(--fg-faint)`（较淡），部分字在某些场景下（如浅色背景、小字号）不够明显，影响可读性。
  - 方案（已确认）：给 `.ev-head` 增加约 2px 的 `text-shadow`，提升标题文字对比度与可读性，不影响布局。
  - 需求：修改 `web/static/style.css` `.ev-head` 增加 `text-shadow`（约 2px 阴影）。
  - 实施：`web/static/style.css` `.ev-head` 增加 `text-shadow: 0 1px 2px rgba(0,0,0,0.35)`（约 2px 阴影）提升标题文字可读性 [BUILD-564]；合并 [BUILD-565]
  - 测试：见 use-case/FEATURE-421/

- [x] **FEATURE-422 Web UI 模型配置界面**
  - 背景：Web UI 目前没有模型配置界面，用户需在录入框发 `:model add` 等命令，体验不佳。希望为 Web UI 设计一套专门的模型设置 UI，复用 REPL 向导状态机。
  - 方案（已确认）：① 状态条模型选择器：hover 状态条主模型/视觉模型图标展开可选择模型列表（当前模型高亮），直接切换当前模式使用的模型，含"默认"选项（清空当前模式模型绑定，恢复全局默认）和"+"新增入口；② 模型管理弹窗：模型列表 + 操作（切换/启用/禁用/编辑/删除/优先级）；③ 专用向导 UI：大弹窗，左侧步骤导航 + 右侧字段设置，上一步/下一步/取消。后端复用 ModelHandler 向导，通过 WebIO 驱动（Print→ui_text、ReadLine/ReadKey→ask、Ask→interaction）；新增 __CANCEL__ 哨兵值实现任意步骤退出兜底（readLine 错误时返回 __CANCEL__，所有 wizardPrompt* 识别并传播，AddModelWizard/editModelWizard 主循环识别退出）；给能力选择、启用等步骤增加 Q 退出；model_wizard_cancel 消息调用 wio.failAll() 让向导退出。
  - 实施：① `cmd/model.go` readLine() 错误时返回 __CANCEL__，所有 wizardPrompt*/wizardSelectCapabilities/wizardSelectTemplate/testEndpointConnectivity 识别并传播 __CANCEL__，AddModelWizard/editModelWizard 主循环识别退出，给能力选择、启用等步骤增加 Q 退出；② `cmd/model_web.go` 新增 ModelWebJSON()（模型+模板列表）、ModelSwitch/Enable/Disable/Remove/SetPriority（复用现有逻辑）、StartAddWizard/StartEditWizard；③ `web/session.go` 注入 model *cmd.ModelHandler，新增 model_get/model_switch/model_enable/model_disable/model_remove/model_set_priority/model_add/model_edit/model_wizard_cancel 消息，model_wizard_cancel 调用 wio.failAll()；④ `web/static/index.html` 新增模型管理弹窗 + 向导弹窗 + 状态条模型选择器 DOM，菜单栏新增模型管理入口；⑤ `web/static/app.js` renderModels/renderModelMenu/renderModelsBody 渲染模型列表和状态条选择器，openModelWizard/closeModelWizard/showWizardAsk/appendWizardText 驱动向导，wizardActive 标志路由 ui_text/ask/interaction 到向导弹窗；⑥ `web/static/style.css` 模型管理弹窗、向导弹窗、状态条选择器样式；⑦ `cmd/model_wizard_cancel_test.go` 新增向导取消测试（Q 退出 + ReadLine 错误退出）[BUILD-567]；⑧ 状态条模型选择器拆分为主模型/视觉模型两个独立清单（hover 🧠/👀 各自展开），视觉模型仅列 vision:true 模型 [BUILD-568]；⑨ 清单增强："默认"选项（显示当前默认模型图标+ID+上下文+半透明）、上下文统一 K 单位（1K=1024）、内置模板官方 logo（DeepSeek/Qwen/Xiaomi/Kimi/Zhipu/OpenAI/LM Studio/Ollama，按 32x32 显示）、选择模型后状态栏刷新 [BUILD-569]；⑩ DeepSeek 图标上色（#4D6BFE）、选择模型改为设置当前 mode 模型绑定（model_bind 消息，而非全局优先级切换）、默认选项显示当前默认模型 [BUILD-571]；⑪ 默认选项透明度 50%、状态栏按 mode 绑定解析（resolveModelForInfo：当前 mode 绑定 > 全局默认）[BUILD-572]；⑫ 模式切换刷新状态栏、模型列表高亮当前在用模型 [BUILD-573]；⑬ 当前 mode 未绑定模型时高亮"默认"选项（ModelInfo 增加 ModeTextModelID/ModeVisionModelID，bootstrap 返回）[BUILD-574]；⑭ 模型高亮改为模式切换控件配色（--accent-dim 底色 + --accent 边框）[BUILD-575]；⑮ 会话切换清单高亮统一为模型列表配色 [BUILD-576]；⑯ 高亮方案融合进前端控件使用规范（.rules/前端控件使用规范/通用约定.md 新增"选中/高亮状态"章节）[BUILD-577]
  - 测试：见 use-case/FEATURE-422/

---

## v0.11.1 — 已完成

> **版本**: v0.11.1

> **状态**: ✅ 已完成（系统提示词优化）
> **里程碑**: 系统提示词优化
> **说明**: 0.11.1 系列专注系统提示词优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-417 | 0.11.1 | P1 | .rules/ 按需加载机制：子文件夹名+完整路径加载到系统提示词（作为可用规则类型提示），子文件夹内容不加载，LLM 需要时按路径主动读取 |
| FEATURE-418 | 0.11.1 | P1 | .rules/ 按需加载层级显示：子文件夹及内部规则按 md 层级输出到系统提示词（子文件夹名→# 标题，内部 .md 文件→## 标题+路径），支持递归遍历子文件夹 |

> 当前 BUILD: 552
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-417 .rules/ 按需加载机制** ✅ 已完成
  - 背景：`.rules/` 目录下所有 `.md` 文件被无条件加载到系统提示词 RULES 节。加入前端控件使用规范（594 行）后，RULES 节膨胀约 3.5 倍，大量与当前任务无关的领域特定规范稀释了注意力，可能导致核心执行纪律要求（如使用 track_task_progress）被忽略。
  - 方案（已确认）：`.rules/` 下可以建子文件夹，子文件夹名 + 完整路径会被加载到系统提示词（作为"可用规则类型"提示），但子文件夹里的文件内容不会加载。LLM 看到路径后可直接用 read_file 读取，无需探索。
  - 需求：修改 `agent/system_prompt.go` 的 `loadRulesDir` 函数，在加载根目录 `.md` 文件后，扫描子文件夹，将子文件夹名 + 完整路径格式化为"可用规则类型（按需加载，需要时用 read_file 读取）"提示追加到结果末尾。将 `前端控件使用规范.md` 移到 `.rules/前端控件使用规范/` 子文件夹。
  - 实施：`agent/system_prompt.go` `loadRulesDir` 在加载根目录 .md 文件后扫描子文件夹，将子文件夹名 + 完整路径格式化为"可用规则类型（按需加载，需要时用 read_file 读取对应路径）"提示追加到结果末尾；`前端控件使用规范.md` 移到 `.rules/前端控件使用规范/` 子文件夹；`.rules/PROJECT STANDARDS.md` 补充按需加载机制说明；`agent/rules_test.go` 新增 `TestLoadRulesDir_SubdirOnDemand` 测试 [BUILD-549]；合并 [BUILD-550]
  - 测试：见 use-case/FEATURE-417/

- [x] **FEATURE-418 .rules/ 按需加载层级显示** ✅ 已完成
  - 背景：FEATURE-417 实现了 .rules/ 按需加载，但子文件夹只显示文件夹名 + 路径，未展示子文件夹内部的规则文件。希望按 md 层级显示：子文件夹名→# 标题，内部 .md 文件→## 标题+路径，支持递归遍历子文件夹。
  - 方案（已确认）：修改 `loadRulesDir`，递归遍历子文件夹，按 md 层级输出：子文件夹名作为 `# 标题`，子文件夹内的 .md 文件作为 `## 文件名: 路径`，如果还有子文件夹继续递归（层级加深）。
  - 需求：修改 `agent/system_prompt.go` 的 `loadRulesDir`，将子文件夹遍历改为递归，按 md 层级输出子文件夹名（# 标题）和内部 .md 文件（## 文件名: 路径）。将 `前端控件使用规范.md` 拆分为多个文件（通用约定、基础按钮控件、容器及布局控件）放到子文件夹下。
  - 实施：`agent/system_prompt.go` `loadRulesDir` 子文件夹遍历改为递归（新增 `appendRulesTree` 辅助函数），按 md 层级输出子文件夹名（# 标题）和内部 .md 文件（## 文件名: 路径），支持递归多层；`前端控件使用规范.md` 拆分为 3 个文件（通用约定/基础按钮控件/容器及布局控件）放到子文件夹；`agent/rules_test.go` 更新 `TestLoadRulesDir_SubdirOnDemand` + 新增 `TestLoadRulesDir_SubdirRecursive` [BUILD-551]；合并 [BUILD-552]
  - 测试：见 use-case/FEATURE-418/

---

## v0.11.0 — 开发中

> **版本**: v0.11.0

> **状态**: ✅ 已完成（Web UI 交互体验优化）
> **里程碑**: Web UI 交互体验优化
> **说明**: 0.11.0 系列专注 Web UI 交互体验优化，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-416 | 0.11.0 | P1 | LLM 输出时主内容区滚动锁定优化：用户向上翻页时生成动态输出区域 A（最大高度为主输出区 1/4），原输出区变为静态区域 B，B 区下方总是显示向下继续图标，点击后合并 A 区内容恢复整体输出 |

> 当前 BUILD: 546
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-416 LLM 输出时主内容区滚动锁定优化** ✅ 已完成
  - 背景：LLM 输出内容时，主内容区的滚动条几乎被锁定在最新一行内容处，用户无法专注查看历史信息。需要设计一种机制提升用户查看历史内容的体验。
  - 方案（已确认）：LLM 正在输出时（运行按钮为红色"暂停"状态），若用户向上翻页（滚动条/滚轮/PgUp/PgDn），立即在主输出框下方生成高度自适应、最大高度为主输出区 1/4 的动态输出区域 A，将最后一个输出块挪到 A 区继续动态输出，后续输出块也在 A 区累积；原输出区变为静态区域 B，用户可持久专注查看历史内容。只要区域 A 存在，就在 B 区下方总是显示一个悬浮的向下箭头图标（指向横线，表示可与下方内容对接），用户点击后将 A 区最新内容合并到 B 区下面，删除 A 区，恢复为整体输出区。
  - 实施：`web/static/index.html` `#stream` 内增加区域 B（`#streamB` 静态区）+ 区域 A（`#streamA` 动态区）；`web/static/style.css` `.stream` 改 flex 列布局 + `.stream-b`/`.stream-a`/`.stream-merge` 样式；`web/static/app.js` 新增 `splitActive` 状态 + `splitStream`/`mergeStream` 函数 + `#streamB` 滚动监听（用户向上翻页且运行时拆分）+ `setRunning` 停止时自动合并 + `scrollStream`/`makeBlock` 按拆分状态归属输出块 [BUILD-547]；合并 [BUILD-548]
  - 测试：见 use-case/FEATURE-416/

---

## v0.10.1 — 开发中

> **版本**: v0.10.1

> **状态**: ✅ 已完成（Web UI 修复）
> **里程碑**: Web UI 修复
> **说明**: 0.10.1 系列专注 Web UI 修复，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-415 | 0.10.1 | P1 | 修复 Web UI TOOL 块输入参数子块 Raw 开关默认状态反了：默认应打开 Raw 模式（按原始内容显示），但当前开关显示为关闭状态 |

> 当前 BUILD: 544
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-415 修复 Web UI TOOL 块输入参数子块 Raw 开关默认状态反了** ✅ 已完成
  - 背景：FEATURE-412 增加"Raw"小胶囊开关控制输入参数子块是否进行 md 渲染，默认关闭（rawMode=true 显示原始内容）。但开关的视觉状态与逻辑状态不一致：默认显示原始内容（rawMode=true）时，Raw 开关应显示为打开（高亮 .on），但当前显示为关闭状态。
  - 根因：`ensureToolParams` 创建 rawPill 时未默认添加 `.on` class，导致默认显示原始内容但开关视觉状态为关闭。
  - 方案（已确认）：在 `ensureToolParams` 创建 rawPill 时默认添加 `.on` class，使开关视觉状态与 rawMode=true（显示原始内容）的默认逻辑状态一致。
  - 实施：`web/static/app.js` `ensureToolParams` 默认 `rawMode` 从 `true` 改为 `false`（默认进行 md 渲染），使 Raw 开关默认关闭（无 .on）与逻辑状态一致 [BUILD-545]；合并 [BUILD-546]
  - 测试：见 use-case/FIX-415/

---

## v0.10.0 — 已完成

> **版本**: v0.10.0

> **状态**: ✅ 已完成（Web UI 优化）
> **里程碑**: Web UI 优化
> **说明**: 0.10.0 系列专注 Web UI 优化，细分任务：

> 当前 BUILD: 529
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [ ] **FEATURE-409 Web UI 四项优化**
  - 背景：Web UI 若干细节优化。
  - 方案（已确认）：① 新建会话+号按钮边框默认透明，hover 才显示；② 文件右侧定位到文件夹图标放大一倍；③ 工作区刷新图标缩小1/3，中文提示改为"刷新"；④ TOOL 块子块动态累积数据时自动滚动到最后一行。
  - 实施：`web/static/style.css` + `web/static/app.js` + `web/static/index.html` [BUILD-516]；追加：定位图标缩小1/4（22px->16.5px）、提问确认栏 body 改为 md 解析 [BUILD-517]；修复：question（it.title）与选项（it.options）也做 md 解析 [BUILD-519]；追加：TOOL 输入参数子块 md 解析、问题提示窗口表格加边框 [BUILD-520]；追加：问题提示框内 ask_followup_question 内容边框颜色统一为 accent [BUILD-521]；追加：快捷键响应后吞掉按键避免漏进输入框 [BUILD-522]；追加：显示块标题栏三个图标（复制/收起/重试自） [BUILD-523]；追加：正在输出的块不被收起 + 标题旁动态省略号 [BUILD-524]；修复：TOOL 标题更新保留图标（避免靠右对齐）、YOU 块增加复制/重试图标 [BUILD-525]；追加：块结束后若同类块被收起且有其他块在输出则自动收起 [BUILD-526]；追加：块结束输出后立即隐藏该块的动态省略号 [BUILD-527]；追加：右上角菜单项文字对齐，无图标项留出图标空位 [BUILD-528]；追加：token_iter 事件时隐藏动态省略号 [BUILD-529]
  - 测试：见 use-case/FEATURE-409/

- [x] **FEATURE-410 Web UI 增加模式切换胶囊控件** ✅ 已完成
  - 背景：Web UI 缺少工作模式切换入口，用户需在 TUI 中用 `:mode switch` 切换。
  - 方案（已确认）：在状态条左边（左下角 logo 下面，左对齐）增加一个横向长胶囊分段控件，分为 1-N 段对应 1-N 个模式选项，每段有文字，高亮段代表选中，用户直接选择一段，高亮效果滑动到用户选择的段上。
  - 实施：`web/static/index.html` + `web/static/app.js` + `web/static/style.css` + `web/session.go` + `web/server.go` + `cmd/mode.go` [BUILD-532]；追加：胶囊控件移到状态条靠左并降低高度 [BUILD-533]；合并 [BUILD-534]
  - 测试：见 use-case/FEATURE-410/

- [x] **FIX-411 修复 Web UI REPL 输出块不新建** ✅ 已完成
  - 背景：在 Web UI 执行 REPL 命令时，系统输出内容都追加到同一个 REPL 输出块，即使有新的 REPL 命令并显示了新的 YOU 模块，REPL 内容仍加到前一个 REPL 块而不新建。导致多个用户消息堆叠，输出在最前面，用户消息多时看不到 REPL 输出。
  - 根因：`renderUserEcho`（创建 YOU 块）没有重置 `curREPL`，导致新 REPL 命令的 ui_text repl 事件继续追加到前一个 REPL 块。
  - 方案（已确认）：在 `renderUserEcho` 开头重置 `curREPL = null`，使每次用户输入（新命令/确认/选择）开启新的 REPL 输出块。
  - 实施：`web/static/app.js` `renderUserEcho` 重置 `curREPL` [BUILD-535]；合并 [BUILD-536]
  - 测试：见 use-case/FIX-411/

- [x] **FEATURE-412 Web UI TOOL 块输入参数子块增加"原始内容"小胶囊开关** ✅ 已完成
  - 背景：TOOL 块输入参数子块（FEATURE-400）当前始终进行 md 内容解析和渲染（FEATURE-409），但部分参数内容（如 JSON、XML 等结构化数据）经 md 渲染后可能丢失原始格式，用户希望可切换查看原始内容。
  - 方案（已确认）：在输入参数子块标题栏、收起展开图标的左边，增加一个名为"原始内容"的小胶囊开关，控制是否进行 md 内容解析和渲染，默认关闭（即默认显示原始内容，不做 md 渲染）。
  - 需求：`web/static/app.js` 的 `ensureToolParams` 在标题栏收起展开图标左边增加"原始内容"小胶囊开关（默认关闭）；开关关闭时参数子块以原始文本（textContent）显示，开关打开时以 md 渲染（mdRender）。`web/static/style.css` 新增小胶囊开关样式。
  - 实施：`web/static/app.js` `ensureToolParams` 标题栏增加"原始内容"小胶囊开关（默认关闭，rawMode=true）+ 新增 `renderParams` 按开关状态渲染（rawMode 时 textContent 显示原始文本，否则 mdRender）；`tool_call_stream` 改用 `renderParams`；`web/static/style.css` 新增 `.tool-params-raw` 小胶囊开关样式 + `.tool-params-right` 右侧容器 [BUILD-537]；修复：rawPill onclick 未切换 rawMode 导致开关无效 [BUILD-538]；追加：开关文字"原始内容"改为"Raw" [BUILD-539]；合并 [BUILD-540]
  - 测试：见 use-case/FEATURE-412/

- [x] **FEATURE-413 整理前端控件使用规范** ✅ 已完成
  - 背景：Web UI 经过多轮迭代（FEATURE-362/365/378/383/387/388/391/393/399/400/401/409/410/412 等），积累了丰富的 HTML 控件（按钮、图标按钮、模态框、下拉菜单、分段控件、小胶囊开关、虚拟键盘选项、事件块、输入参数子块、文件树、状态栏等）。这些控件分散在 `web/static/index.html`、`app.js`、`style.css` 中，缺乏统一的使用规范，未来新增界面控件时可能样式不一致。
  - 方案（已确认）：整理一份前端控件使用手册，放到 `.rules/` 目录，作为未来界面控件的使用准则。手册按控件分类，每个控件说明其用途、HTML 结构、CSS 类名、JS 用法、样式要点、使用注意事项，确保控件以统一的方式和样式被使用。
  - 需求：在 `.rules/` 下新建 `前端控件使用规范.md`，系统整理现有前端控件（按钮、图标按钮、键盘按钮、虚拟键盘选项键、小胶囊开关、分段控件、开关/输入/下拉、事件块、输入参数子块、模态框、下拉菜单、悬浮菜单、会话菜单、图片预览、提问区域、交互组件、选项按钮组、任务计划、文件树、状态栏、输入行等），每个控件说明用途、HTML 结构、CSS 类名、JS 用法、样式要点、注意事项。
  - 实施：`.rules/前端控件使用规范.md` 系统整理现有前端控件（基础按钮控件 7 类 + 容器/布局控件 13 类 + 通用状态类 + 使用准则），每个控件说明用途、HTML 结构、CSS 类名、JS 用法、样式要点、注意事项 [BUILD-541]；合并 [BUILD-542]
  - 测试：见 use-case/FEATURE-413/

- [x] **FEATURE-414 项目规范增加版本计划规定** ✅ 已完成
  - 背景：当前规范（`.rules/PROJECT STANDARDS.md`）在新建分支时，仅当用户未指定版本时才提示制定版本计划，且打版本标签的时机不明确（仅提交到其他分支时也可能打标签）。需要明确：新建任何分支前（含 feature/fix）都需让用户确认成果归属版本，合并到 main 时按计划打版本标签，仅提交到其他分支时不打版本标签。
  - 方案（已确认）：修改 `.rules/PROJECT STANDARDS.md` 的「新建开发任务」和「代码提交及合并」章节：① 新建任何分支前（含 feature/fix）需让用户确认新建分支的成果将放到哪个版本；② 用户明确后先更新 ROADMAP 中的版本计划下对应的内容；③ 待合并到 main 分支时，按计划打版本标签；④ 仅提交到其他分支时不打版本标签。
  - 需求：`.rules/PROJECT STANDARDS.md` 的「新建开发任务」步骤 1 改为：新建任何分支前（含 feature/fix）需让用户确认成果归属版本，用户明确后先更新 ROADMAP 版本计划下对应内容；「代码提交及合并」步骤 4 改为：合并到 main 分支时按计划打版本标签，仅提交到其他分支时不打版本标签。
  - 实施：`.rules/PROJECT STANDARDS.md` 「新建开发任务」步骤 1 改为"确认版本归属（新建任何分支前必做）"（新建任何分支前含 feature/fix 需让用户确认成果归属版本，明确后先更新 ROADMAP 版本计划下对应内容）；「代码提交及合并」步骤 4 改为"打版本标签（仅合并到 main 时）"（合并到 main 时按计划打版本标签，仅提交到其他分支时不打版本标签） [BUILD-543]；合并 [BUILD-544]
  - 测试：见 use-case/FEATURE-414/

---

## v0.9.1 — 开发中（已完成）

> **版本**: v0.9.1

> **状态**: ✅ 已完成
> **里程碑**: Web UI 会话切换上下文修复
> **说明**: 0.9.1 系列专注 Web UI 会话切换上下文修复，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FIX-407 | 0.9.1 | P1 | 修复 Web UI 会话切换不切换上下文（switchSession 补充 FlushCurrentSession + 加载目标会话消息 + SetHistory） |
| FEATURE-408 | 0.9.1 | P1 | 优化分支规范：一个会话一个分支（非每个指令都建分支，任务编号会话开始时确定一次，用户完整确认后自动合并） |

> 当前 BUILD: 515
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FIX-407 修复 Web UI 会话切换不切换上下文** ✅ 已完成
  - 背景：通过 Web UI 状态栏会话图标弹出的会话列表进行会话切换时，只切换了任务进展中的任务计划，但 `:context` 显示的上下文没有切换。根因：`WebSession.switchSession` 只调用 `SetCurrentSessionID` + `SaveCurrentSessionID`，没有像 REPL `:session switch` 命令（`cmd/session.go handleSwitch`）那样执行 `FlushCurrentSession` + 加载目标会话消息 + `SetHistory`，导致 agent 内存中的 `a.messages`（真正的 LLM 上下文）从未被替换。
  - 方案（已确认）：参照 `cmd/session.go handleSwitch` 的正确模式，在 `web/session.go switchSession` 中补充：① `FlushCurrentSession()` 将当前会话消息写回 DB；② `LoadNamedSession(id)` 加载目标会话；③ `json.Unmarshal` 目标会话 Messages；④ 用当前 system prompt + 目标消息构建新 history 并 `SetHistory`；⑤ `SetCurrentSessionID` + `SaveCurrentSessionID`
  - 实施：`web/session.go` `switchSession` 补充 `FlushCurrentSession` + `LoadNamedSession` + `json.Unmarshal` + `SetHistory`（新增 `llm` 导入）；`web/session_test.go` 新增 `TestSessionSwitchSwitchesContext`（验证切换后 agent 上下文被替换 + 原会话消息被刷新到 DB）[BUILD-514]
  - 测试：见 use-case/FIX-407/

- [x] **FEATURE-408 优化分支规范：一个会话一个分支** ✅ 已完成
  - 背景：原规范要求每个用户修改指令都建任务编号并创建分支，但实际中一个会话通常已在一个功能分支上，重复建分支繁琐。
  - 方案（已确认）：改为"一个会话一个分支"——仅当当前会话中尚未创建分支时才自动创建分支；任务编号在会话开始时确定一次，后续指令归入该任务编号；每次编码完成/编译/提交/测试仍更新 BUILD 编号；直到用户完整确认后自动打版本标签、编译、提交、合并代码。
  - 实施：修改 `.rules/PROJECT STANDARDS.md` 的分支策略、新建开发任务、开发流程、代码提交及合并四个章节，落实"一个会话一个分支"原则 [BUILD-515]
  - 测试：见 use-case/FEATURE-408/

---

## v0.9.0 — 开发中（已完成）

> **版本**: v0.9.0

> **状态**: ✅ 已完成
> **里程碑**: 工具调用交互标准化
> **说明**: 0.9.0 系列专注工具调用交互标准化（Interaction 模型 + 意图字段结构化 + TUI/Web 统一输出），细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-388 | 0.9.0 | P1 | 工具调用交互标准化（Interaction 模型 + 意图字段结构化 + TUI/Web 统一输出） |
| FEATURE-389 | 0.9.0 | P1 | 优化分支规范：强制分支检查 + 小优化也必须建分支 + 禁止 main 提交提升为最高红线 |
| FEATURE-390 | 0.9.0 | P1 | 规范增加版本号默认递增规则：FEATURE→minor递增，FIX→patch递增 |
| FEATURE-391 | 0.9.0 | P1 | 将 :set 功能迁移到 Web UI（settings_get/settings_set 结构化消息） |
| FEATURE-392 | 0.9.0 | P1 | 修正 Web UI 设置面板分类与顺序（与 TUI :set 一致，去掉身份与个性分组） |
| FEATURE-393 | 0.9.0 | P1 | Web UI 身份与个性设置菜单（logo 悬停弹出，name/description/principles/capabilities/rules 大表单） |
| FEATURE-394 | 0.9.0 | P1 | Web UI 系统设置面板对齐优化（配置项名称右对齐、值左对齐，靠向中线显示） |
| FEATURE-395 | 0.9.0 | P1 | 去掉右上角下拉菜单中工作区/任务进展/状态条三个菜单项的图标（保留文字和功能） |
| FEATURE-396 | 0.9.0 | P1 | 去掉暂停终止（ESC 中断确认）时提示框内无用的 [1]-[9] 批准次数提示 |
| FEATURE-397 | 0.9.0 | P1 | Web UI 拦截 ESC 按键触发暂停（等价于点击 ⏸ 按钮） |
| FEATURE-398 | 0.9.0 | P1 | 右上角菜单增加"重启后台"菜单项（发送重启信号通知外部 supervisor 重启进程） |
| FEATURE-399 | 0.9.0 | P1 | 优化询问用户（ask_followup_question）交互：选项用虚拟键盘风格快捷按钮 + 去掉无用 [1-9] + 空格补充说明 |
| FEATURE-400 | 0.9.0 | P1 | TOOL 输入参数子块：动态输出参数放入可滚动子块（子标题栏"输入参数" + 展开/固定高度切换按钮） |
| FEATURE-401 | 0.9.0 | P1 | 左下角 logo 与消息框之间增加"+"号按钮（创建新会话） |
| FEATURE-402 | 0.9.0 | P1 | 调整"+"号按钮位置到录入框左边 + 去掉对话条背景色 |
| FEATURE-403 | 0.9.0 | P1 | "+"号按钮基准位置向右上方 x/y 都整体移动 3 像素 |
| FEATURE-404 | 0.9.0 | P1 | 改进主消息录入框滚动条：缩小内容上下间距避免无内容/单行时出现滚动条 |
| FEATURE-405 | 0.9.0 | P1 | 改进主消息录入框高度：autoGrow 增加上限 + scrollHeight 加增量 |
| FEATURE-406 | 0.9.0 | P1 | 修复主消息录入框默认状态下光标垂直方向不居中（增加上下 padding） |

> 当前 BUILD: 468
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

### 任务详情

- [x] **FEATURE-388 工具调用交互标准化** ✅ 已完成
  - 背景：当前三类交互（意图字段、提问/选择、确认放行）都是通过 `UserIO` 底层原语（`Print*` + `ReadLine/ReadKey`）拼装出来的，交互逻辑散落在各工具回调里。Web UI 只能看到 `ui_text` 文本 + 通用输入框，无法结构化渲染，前端还硬编码了确认键位。
  - 方案（已确认）：引入统一 `Interaction` 交互模型，把交互从底层原语拼装提升为结构化声明。
    - 决策 1：`InteractionManager` 采用独立接口（方案 B），组合复用 `UserIO` 底层能力，对现有 StdioIO/EnhancedIO/WebIO 零侵入
    - 决策 2：确认放行键位保持固定（Enter/c/a/g/d/N），不支持用户自定义，但不同场景用不同键位集
    - 决策 3：`ToolSummary` 结构化摘要覆盖所有工具（高频工具定制化参数展示，低频工具通用 fallback）
    - 决策 4：Web 端"批准N次"提供预设按钮 3/10/50 + 自由数字输入
    - 决策 5：i18n 模板分两步走（第一步 `ToolSummary` 同时保留 `Text` 和结构化字段，TUI 用 `Text`、Web 用结构化字段；第二步逐步迁移为字段标签）
  - 需求：① 定义 `Interaction`/`InteractionResult`/`InteractionManager` 接口；② 实现 `TerminalInteractionManager`（TUI）；③ 迁移 `promptToolConfirmation` 和 `askFollowupQuestionTool` 到新模型；④ 实现 `WebInteractionManager` + WebSocket `interaction` 协议；⑤ 前端渲染交互组件（按钮组/选项列表/输入框），移除硬编码键位；⑥ `buildToolSummary` 返回 `ToolSummary` 结构体，`EventToolCall` 携带结构化摘要
  - 测试：`agent/interaction_test.go`（Interaction 模型 + TerminalInteractionManager + 迁移后行为不变 + promptToolConfirmation/promptErrorConfirmation 构造的 Interaction 含 Keys + ActionInput 映射为 CmdConfirmModify 不批准执行）、`web/session_test.go`（WebIO.Ask 推送结构化 interaction + 接收 interaction_answer）、`tmp/feature388_interaction_test.js`（前端选项项渲染 + 键按钮只显示字母/按键名 + 说明写在按钮旁边 + 数字键合并 1-9 + 空格补录模式 + 数字选择模式 + 快捷键暂停 + 上下结构 + TOOL 标题栏显示动作和意图，25 断言全过）、`agent/tool_summary_test.go`（ToolSummary 结构化 + EventToolCall 携带摘要）；`go build/vet/test ./...` 全绿；浏览器验证确认放行/错误处理/ESC中断/提问选择统一为选项项（每个选项一个虚拟键盘风格方形键按钮只显示字母/按键名如 A/Enter，说明文字写在按钮旁边如"全部批准"，横向排列，点击按钮直接响应）；上下结构（上面提示信息，下面一排键按钮+按键说明）；[空格]进入补充信息补录模式，主消息框录入且不再监听快捷键（避免被劫持），但仍可点击按钮；补充信息输入后取消执行工具调用（ActionInput 映射为 CmdConfirmModify，不批准执行）；TOOL 调用显示框标题栏显示"TOOL <动作> - <意图>"（工具名翻译成动作），消息框内部不再显示意图参数 [BUILD-481]
  - 测试：见 use-case/FEATURE-388/

- [ ] **FEATURE-389 优化分支规范（防止 LLM 忘记建分支）**
  - 背景：LLM 经常忘记按规范创建新分支，小优化（标题栏优化、颜色优化、冒号优化等）直接在 main 分支上提交，违反"禁止直接在 main 上提交"规范。根因：①规范是流程步骤而非强制约束，小优化时 LLM 跳过"新建分支"步骤；②缺少提交前的强制检查；③小优化没有任务编号，LLM 默认在 main 上继续。
  - 方案（已确认）：采用方案 1+2+3。
    - 方案 1：在"开发流程"第 1 步"开始编码"前、"代码提交及合并"第 3 步"提交合并代码"前，增加强制分支检查（`git branch --show-current` 确认在功能分支，若在 main 必须先建分支）
    - 方案 2：明确任何代码修改（包括小优化）都必须先建任务编号并创建分支
    - 方案 3：把"禁止直接在 main 提交"提升为最高优先级红线
    - 方案 4（长期增强，本次不实施）：在 execute_command 执行 git 提交时检查当前分支，若在 main 则拒绝
  - 需求：修改 `.rules/PROJECT STANDARDS.md`，落实方案 1+2+3
  - 实施：①分支策略章节新增"🚨 最高优先级红线"，明确禁止直接在 main 提交任何代码修改（含小优化），违反视为严重违规；②新建开发任务第 2 步新增"任何代码修改（包括小优化）都必须建任务编号"；③开发流程新增第 0 步"强制分支检查（编码前必做）"，用 `git branch --show-current` 确认在功能分支；④代码提交及合并第 3 步新增"强制分支确认（提交前必做）" [BUILD-484]
  - 测试：见 use-case/FEATURE-389/

- [ ] **FEATURE-390 规范增加版本号默认递增规则**
  - 背景：当前版本计划由用户手动选择[小/中/大/不新建]，容易不一致。希望版本号递增与分支类型自动绑定，减少人为决策。
  - 方案（已确认）：作为"新建版本计划"的默认值。
    - FEATURE 分支（新功能）→ 版本号默认第二位（minor）递增
    - FIX 分支（Bug 修复）→ 版本号默认第三位（patch）递增
    - 用户可覆盖默认值；major（第一位）递增仍由用户手动决定（重大变更需人工判断）
    - 与 ROADMAP 版本命名规范一致（patch=修复、minor=新功能）
  - 需求：修改 `.rules/PROJECT STANDARDS.md` 新建开发任务第 1 步，补充版本号默认递增规则
  - 实施：新建开发任务第 1 步新增"版本号默认递增规则"——FEATURE 分支（新功能）→ 版本号第二位（minor）递增，FIX 分支（Bug 修复）→ 版本号第三位（patch）递增；作为默认值用户可覆盖，major（第一位）递增仍由用户手动决定 [BUILD-485]
  - 测试：见 use-case/FEATURE-390/

- [ ] **FEATURE-391 将 :set 功能迁移到 Web UI**
  - 背景：REPL 的 `:set` 命令功能强大（几十个设置项），但 Web UI 只能通过输入框输入 `:set` 命令，缺少图形化设置界面。前端已有"系统设置"弹窗，但只含主题设置（前端本地 localStorage）。
  - 方案（已确认）：方案 B，新增 `settings_get`/`settings_set` WebSocket 消息。
    - 后端：`SessionDeps` 新增 `SettingsHandler` 字段；新增 `settings_get`（读取当前设置，返回结构化 JSON 按组组织）；新增 `settings_set`（构造 `[key,value]` 调用 `SettingsHandler.Handle`）
    - 前端：设置弹窗动态渲染设置项（bool→开关、number→数字输入、enum→下拉、string→文本），修改后发送 `settings_set`；保留现有主题设置
  - 需求：① `repl/session.go` SessionDeps 新增 SettingsHandler 字段，`repl/repl.go` 传入；② `web/server.go` clientMessage/serverMessage 新增 settings 类型；③ `web/session.go` 新增 settings_get/settings_set 处理；④ 前端 app.js/index.html/style.css 设置弹窗动态渲染
  - 实施：① `repl/session.go` SessionDeps 新增 `SettingsHandler *cmd.SettingsHandler` 字段，`repl/repl.go` 传入 `r.settingsHandler`；② `cmd/settings_web.go` 新增 `SettingsJSON()` 返回按组组织的结构化设置项（6 组覆盖全部 :set 设置项，含 type/options）；③ `web/server.go` clientMessage 新增 `settings_get`/`settings_set` 类型（settings_set 携带 key/value），serverMessage 新增 `settings`/`settings_result` kind；④ `web/session.go` WebSession 新增 settings 字段，handleMessage 新增 settings_get/settings_set 处理（settings_get 返回 SettingsJSON，settings_set 调用 SettingsHandler.Handle）；⑤ 前端 index.html 设置弹窗新增 settingsBody 容器，app.js 打开弹窗发送 settings_get、按类型渲染设置项（bool→开关、number→数字输入、enum→下拉、string→文本）、修改后发送 settings_set（改哪个发哪个，文本框焦点移出/回车触发、开关/下拉立即触发）、显示设置结果，style.css 新增设置面板样式 [BUILD-487]
  - 测试：见 use-case/FEATURE-391/

- [ ] **FEATURE-392 修正 Web UI 设置面板分类与顺序（与 TUI :set 一致）**
  - 背景：FEATURE-391 的 Web UI 设置面板分类与 TUI `:set` 输出（`showSettingsHelp()`）不一致：当前把 temperature/max-tokens 等放"模型组"（用 KeySettingsGroupModel 标题），但 TUI 中这些属于 Group 2 [智能体设置]；当前把 name/memory-enabled 等放"Agent 组"（用 KeySettingsGroupIdentity 标题），但 TUI 中 name 属于 Group 1、memory-enabled 属于 Group 5；顺序也不同；还包含 top-p/top-k/repetition-penalty/max-model-len/max-retries/read-file-max-size/duplicate-content-threshold/loop-temp-enabled 等不在 TUI 输出中的多余项。
  - 方案（已确认）：重写 `cmd/settings_web.go` 的 `SettingsJSON()`，使其分组和顺序与 `showSettingsHelp()` 完全一致，但去掉 Group 1 [身份与个性]（该分组未来放到其他位置）。输出 5 组：①[智能体设置]（KeySettingsGroupModel）temperature→parse-error-action；②[显示与输出]（KeySettingsGroupDisplay）emoji-enabled→token-usage；③[安全与确认]（KeySettingsGroupSafety）confirm-tool→default-tool-model；④[记忆与上下文]（KeySettingsGroupMemory）memory-enabled→memory-search-max-results；⑤[开发者]（KeySettingsGroupSearchDebug）debug、log。
  - 需求：① 去掉 top-p/top-k/repetition-penalty/max-model-len/max-retries/read-file-max-size/duplicate-content-threshold/loop-temp-enabled（不在 TUI 输出中）；② current-tool-model/current-vision-model/current-problem-model 是只读展示项（Web 设置面板无意义，不包含）；③ output-categories/db/llm-log 是特殊项（复合项/子命令/开发者日志开关，不包含）；④ 删除不再使用的 agentNameValue 辅助函数（name 属于 Group 1 已去掉）；⑤ 保留 config 导入（result-mode 用到 config.ResultModeString）
  - 实施：重写 `cmd/settings_web.go` 的 `SettingsJSON()` 为 5 组，分组和顺序与 `showSettingsHelp()` 完全一致（去掉 Group 1 身份与个性），删除 agentNameValue 辅助函数 [BUILD-488]
  - 测试：见 use-case/FEATURE-392/

- [ ] **FEATURE-393 Web UI 身份与个性设置菜单（logo 悬停弹出）**
  - 背景：FEATURE-392 已把 [身份与个性] 分组从设置面板去掉（该分组未来放到其他位置）。现在需要把身份与个性设置放到 Web UI 左下角 co-shell logo 的悬停菜单中，在一个大表单中同时设置 name/description/principles/capabilities/rules。
  - 方案（已确认）：鼠标移向左下角 co-shell logo 时向上弹出菜单，含 [身份与个性] 入口；点击后打开大表单弹窗，同时展示 name（单行）、description/principles/capabilities/rules（多行文本），每项（含名字）旁有独立保存按钮，点击后单独保存。
  - 需求：① 后端新增身份与个性读写接口（name/description/principles 存 config.json，capabilities 存外部文件 CAPABILITIES.md，rules 存 cfg.Rules）；② WebSocket 新增 identity_get/identity_set 消息；③ 前端 logo 悬停弹出菜单 + 身份与个性大表单 + 每项独立保存按钮
  - 实施：① `cmd/settings_identity.go` 新增 `IdentityJSON()`（返回 name/description/principles/capabilities/rules 当前值）和 `SaveIdentity(key,value)`（name/description/principles 写 config.json，capabilities 写 CAPABILITIES.md，rules 写 cfg.Rules）；② `web/server.go` clientMessage 新增 identity_get/identity_set 类型，serverMessage 新增 identity/identity_result kind；③ `web/session.go` 新增 identity_get/identity_set 处理；④ 前端 index.html 新增 logo 悬停菜单 + 身份弹窗，app.js 渲染大表单（name 单行 + 其余多行 textarea）+ 每项独立保存按钮，style.css 新增样式；修复：description/principles/capabilities 显示默认值、字段标签本地化（多语言）+ 首字母大写；去掉身份菜单人像图标 [BUILD-491]
  - 测试：见 use-case/FEATURE-393/

- [ ] **FEATURE-394 Web UI 系统设置面板对齐优化**
  - 背景：Web UI 系统设置面板中，配置项名称（.set-label）默认左对齐，值控件靠右，视觉上名称与值分离。希望名称右对齐、值左对齐，两者都靠向中线显示，更紧凑美观。
  - 方案（已确认）：将每行名称-值显示区域分为左右两个等宽块（各 50%）占满整行，名称右对齐、值左对齐，两者靠向中线。
  - 需求：① `.set-label` 改为 `flex: 0 0 50%` + `text-align: right` + 右 padding；② `.set-input`/`.set-row select` 改为 `flex: 1` + `width: auto` 占满右半块；③ `.set-toggle` 保持小尺寸靠右半块左对齐；④ 去掉 `.set-row` 的 `justify-content: space-between`（避免 checkbox 被推到最右）
  - 实施：`web/static/style.css` 设置面板改为等宽双列布局（label 50% 右对齐，值控件 50% 左对齐）；`index.html` 主题上方加「外观」节标题 + 主题 label 加 set-label 类（主题行也等宽双列对齐）；`app.js` i18n 加 appearance 键；`style.css` `.set-group-title` 居中显示 [BUILD-495]；后续优化：外观节标题加方括号 `[ 外观 ]`（i18n appearance 键值同步）、新增 `settingsDynamic` 容器把动态设置项渲染到独立容器（与静态外观/主题节分离）、调整 `.set-row` gap 与 `.set-label` padding [BUILD-497]
  - 测试：见 use-case/FEATURE-394/

- [ ] **FEATURE-395 去掉右上角下拉菜单中三个菜单项的图标**
  - 背景：Web UI 右上角下拉菜单中，工作区（🗂）、任务进展（📋）、状态条（📊）三个菜单项带有 emoji 图标，视觉上较杂乱。希望去掉这三个菜单项的图标，只保留文字，功能不变；系统设置（⚙️）图标保留。
  - 方案（已确认）：仅去掉工作区/任务进展/状态条三个菜单项的图标（emoji），保留文字和功能；系统设置图标保留。
  - 需求：`web/static/index.html` 右上角菜单中，去掉 `miWs`/`miPlan`/`miStatus` 三个菜单项的 `<span class="mi-ico">` 图标元素，保留文字和功能。
  - 实施：`web/static/index.html` 去掉 `miWs`/`miPlan`/`miStatus` 三个菜单项的 `.mi-ico` 图标 span，保留 `.mi-label` 文字和原有功能 [BUILD-498]
  - 测试：见 use-case/FEATURE-395/

- [ ] **FEATURE-396 去掉暂停终止时提示框内无用的 [1]-[9] 批准次数提示**
  - 背景：Web UI 中，ESC 暂停终止（中断确认）时提示框内多了一个 [1]-[9] 批准次数提示，但该场景没有 approve_count（批准N次）功能，提示无用。根源：前端 `renderVirtualKeyboard` 对所有非 select 交互都无条件把数字键 0-9 映射为 approve-count 并渲染 [1]-[9] 项，而 ESC 中断确认（run_stream.go 的 InteractionConfirm）只包含 Enter 继续 / c 取消两个键，没有 approve_count 相关键。
  - 方案（已确认）：仅当交互的 Keys 中包含 approve_count 相关键（或 Presets 非空）时才渲染 [1]-[9] 批准次数提示；ESC 暂停终止场景不渲染。
  - 需求：`web/static/app.js` 的 `renderVirtualKeyboard` 中，数字键 approve-count 映射与 [1]-[9] 项渲染改为按需（仅当交互需要 approve_count 时），避免暂停终止等无 approve_count 场景出现无用提示。
  - 实施：`web/static/app.js` `renderVirtualKeyboard` 增加 approve-count 能力判断（仅当 `it.presets` 非空或 keys 含 approve_count 相关键时启用数字键映射与 [1]-[9] 项），ESC 暂停终止场景（无 presets）不再渲染 [1]-[9] 提示 [BUILD-499]
  - 测试：见 use-case/FEATURE-396/

- [ ] **FEATURE-397 Web UI 拦截 ESC 按键触发暂停**
  - 背景：Web UI 中暂停（interrupt）只能通过点击 ⏸ 按钮触发（发送 `{ type: "interrupt" }` 消息），ESC 键未被拦截。希望按 ESC 也能触发暂停，与终端行为一致。
  - 方案（已确认）：在全局 keydown 监听中，当 `e.key === "Escape"` 且当前正在运行（running）时发送 `{ type: "interrupt" }` 消息，等价于点击 ⏸ 按钮。交互 pending 时不触发（避免与虚拟键盘冲突），输入框聚焦时也触发（全局 ESC 都暂停）。
  - 需求：`web/static/app.js` 全局 keydown 监听中，增加 ESC 键处理——当 `e.key === "Escape"` 且 `running` 为 true 且无 pendingInteraction 时，发送 interrupt 消息。
  - 实施：`web/static/app.js` 全局 keydown 监听（document.addEventListener）增加 ESC 分支——`e.key === "Escape"` 且 `running` 且 `!pendingInteraction` 时 `wsSend({ type: "interrupt" })` 并 `e.preventDefault()` [BUILD-500]
  - 测试：见 use-case/FEATURE-397/

- [ ] **FEATURE-398 右上角菜单增加"重启后台"菜单项**
  - 背景：Web UI 右上角菜单中，系统设置下面需要增加"重启后台"菜单项，与系统设置之间增加分隔线。功能：发送重启信号通知外部 supervisor 重启进程。
  - 方案（已确认）：前端右上角菜单系统设置下面增加"重启后台"菜单项（与系统设置之间加分隔线），点击后发送 `{ type: "restart" }` WebSocket 消息；后端收到后向当前进程发送 SIGHUP 信号，由外部 supervisor（如 launchd/systemd）捕获后重启进程。
  - 需求：① `web/static/index.html` 右上角菜单系统设置下面增加"重启后台"菜单项 + 分隔线；② `web/static/app.js` 增加菜单项点击处理（发送 restart 消息）+ i18n 键；③ `web/server.go` clientMessage 新增 restart 类型；④ `web/session.go` handleMessage 新增 restart 处理（发送 SIGHUP 信号给当前进程）。
  - 实施：`web/static/index.html` 右上角菜单系统设置下面增加"重启后台"菜单项（`miRestart`）+ 分隔线；`web/static/app.js` 增加 `miRestart.onclick` 发送 `{ type: "restart" }` + i18n restart 键；`web/server.go` clientMessage 注释补充 restart 类型；`web/session.go` handleMessage 新增 `case "restart"` 调用 `syscall.Kill(os.Getpid(), syscall.SIGHUP)` 发送重启信号 [BUILD-501]
  - 测试：见 use-case/FEATURE-398/

- [ ] **FEATURE-399 优化询问用户（ask_followup_question）交互**
  - 背景：Web UI 中"询问用户"（select）场景渲染了无用的 [1]-[9] 批准次数按钮（标签"批准N次"，对 select 场景无用），且选项按钮是普通样式（`1. 选项`），没有快捷按钮。希望选项用虚拟键盘风格渲染——每个选项一个方形键按钮（数字 1..N）+ 选项文字，点击按钮或按物理数字键即选择对应选项；按空格可录入补充说明。
  - 方案（已确认）：select 场景用虚拟键盘风格渲染每个选项（数字 1..N 方形键按钮 + 选项文字），去掉 [1]-[9] 批准次数按钮和 Enter 批准按钮，保留 Space 补充说明按钮。
  - 需求：`web/static/app.js` 的 `showInteraction` 和 `renderVirtualKeyboard` 中，select 场景改为虚拟键盘风格渲染选项（每个选项一个数字方形键按钮 + 选项文字），去掉 [1]-[9] 批准次数按钮和 Enter 批准按钮，保留 Space 补充说明。
  - 实施：`web/static/app.js` `showInteraction` 的 select 分支去掉 radio-style 选项列表，改为直接调用 `renderVirtualKeyboard(it, true)`；`renderVirtualKeyboard` 的 select 分支渲染每个选项为数字方形键按钮（1..N）+ 选项文字，不渲染 [1]-[9] 批准次数按钮和 Enter 批准按钮，保留 Space 补充说明 [BUILD-502]
  - 测试：见 use-case/FEATURE-399/

- [ ] **FEATURE-400 TOOL 输入参数子块**
  - 背景：TOOL 调用时，动态输出的输入参数（tool_call_stream 流式片段）累积显示，之后被执行前摘要（phase=input 参数摘要）替换冲掉。希望将动态输出的输入参数内容输出到当前 TOOL 块中的一个子块：子块有滚动条、子标题栏（"输入参数"）、标题栏右上角有完全展开/固定高度切换按钮，固定高度时可滚动。
  - 方案（已确认）：子块显示流式参数，执行前摘要不再替换（子块就是参数容器），执行结果追加到 TOOL 块。
  - 需求：`web/static/app.js` 的 TOOL 块渲染中，流式参数（tool_call_stream）累积进一个子块（`tool-params`），子块有子标题栏（"输入参数"）+ 右上角展开/固定高度切换按钮 + 可滚动内容区；执行前摘要（phase=input）不再替换流式参数；执行结果（phase=result）追加到 TOOL 块 body。`web/static/style.css` 新增子块样式。
  - 实施：`web/static/app.js` TOOL 块渲染增加输入参数子块（`tool-params`：子标题栏 + 展开/固定高度切换按钮 + 可滚动内容区），流式参数累积进子块，执行前摘要不再替换；`web/static/style.css` 新增子块样式（固定高度 + 滚动条 + 展开切换） [BUILD-503]
  - 测试：见 use-case/FEATURE-400/

- [ ] **FEATURE-401 左下角 logo 与消息框之间增加"+"号按钮（创建新会话）**
  - 背景：Web UI 左下角 co-shell logo 和主消息框之间需要增加一个大一点的"+"号按钮（与 logo 和消息框都留 5-10 像素间距），点击创建新会话。
  - 方案（已确认）：在 `#logoWrap` 和 `.bottom-main` 之间增加一个"+"号按钮（`#newSessionBtn`），点击发送 `{ type: "session_new" }` WebSocket 消息；后端收到后创建新会话（复用 `:new` 命令逻辑）。
  - 需求：① `web/static/index.html` 在 `#logoWrap` 和 `.bottom-main` 之间增加"+"号按钮；② `web/static/app.js` 增加按钮点击处理（发送 session_new 消息）；③ `web/static/style.css` 按钮样式（大一点，与 logo 和消息框留 5-10 像素间距）；④ `web/server.go` clientMessage 新增 session_new 类型；⑤ `web/session.go` handleMessage 新增 session_new 处理（创建新会话）。
  - 实施：`web/static/index.html` 在 `#logoWrap` 和 `.bottom-main` 之间增加"+"号按钮（`#newSessionBtn`）；`web/static/app.js` 增加 `newSessionBtn.onclick` 发送 `{ type: "session_new" }`；`web/static/style.css` 按钮样式（大号 + 与 logo/消息框 5-10px 间距）；`web/server.go` clientMessage 注释补充 session_new 类型；`web/session.go` handleMessage 新增 `case "session_new"` 创建新会话（复用 `:new` 逻辑：刷新当前会话 + 生成新 sessionID + 创建 SessionEntry + SetCurrentSessionID） [BUILD-504]
  - 测试：见 use-case/FEATURE-401/

- [ ] **FEATURE-402 调整"+"号按钮位置到录入框左边 + 去掉对话条背景色**
  - 背景：FEATURE-401 将"+"号按钮放在 logo 和消息框之间，但用户希望调整："+"号嵌入到录入框（input-row）的左边，留好和边框和录入区域的边距；logo 位置、对齐方式保持现状；去掉对话条（#bottom）背景色。
  - 方案（已确认）：将"+"号按钮从 `#logoWrap` 和 `.bottom-main` 之间移到 `.input-row` 的左边（`#input` 前面），嵌入到录入框内；logo 位置、对齐方式保持现状；去掉 `#bottom` 的背景色。
  - 需求：① `web/static/index.html` 将"+"号按钮从 `#logoWrap` 和 `.bottom-main` 之间移到 `.input-row` 的左边；② `web/static/style.css` 调整按钮样式（嵌入录入框左边，留好边距）+ 去掉 `#bottom` 背景色。
  - 实施：`web/static/index.html` 将 `#newSessionBtn` 从 `#logoWrap` 和 `.bottom-main` 之间移到 `.input-row` 内 `#input` 前面；`web/static/style.css` 调整 `.new-session-btn` 样式（嵌入录入框左边，留好和边框/录入区域边距）+ 去掉 `#bottom` 的 `background: var(--bg-panel)` [BUILD-505]
  - 测试：见 use-case/FEATURE-402/

- [ ] **FEATURE-403 "+"号按钮基准位置向右上方 x/y 都整体移动 3 像素**
  - 背景：FEATURE-402 将"+"号按钮嵌入到录入框左边，但用户希望"+"号按钮的基准位置再向右上方 x/y 都整体移动 3 像素。
  - 方案（已确认）：给 `.new-session-btn` 加 `transform: translate(3px, -3px)`（向右 3px、向上 3px）。
  - 需求：`web/static/style.css` 的 `.new-session-btn` 增加 `transform: translate(3px, -3px)`。
  - 实施：`web/static/style.css` 的 `.new-session-btn` 增加 `transform: translate(3px, -3px)` [BUILD-506]
  - 测试：见 use-case/FEATURE-403/

- [ ] **FEATURE-404 改进主消息录入框滚动条**
  - 背景：主消息录入框（`#input` textarea）在没有内容、有单行内容或有多行内容但没有超过最大显示行数时，都会自动出现滚动条。怀疑是输入框默认高度、内容上下间距（padding）设置稍大导致。
  - 方案（已确认）：缩小 `#input` 的内容上下间距（padding），使单行/多行但未超限时内容能完整容纳，不出现滚动条。
  - 需求：`web/static/style.css` 的 `#input` 缩小上下 padding（当前 `padding: 8px 12px`，上下 8px 较大）。
  - 实施：`web/static/style.css` 的 `#input` 上下 padding 从 8px 缩小到 4px（`padding: 4px 12px`），使单行/多行但未超限时不出现滚动条 [BUILD-508]
  - 测试：见 use-case/FEATURE-404/

- [ ] **FEATURE-405 改进主消息录入框高度**
  - 背景：FEATURE-404 缩小 padding 后滚动条仍出现。用户发现 autoGrow 自动增长高度（`input.style.height = Math.min(input.scrollHeight, 120)`），但文本框高度仍不足。建议适当增加累积单位高度/文本框高度。
  - 方案（已确认）：两者都做——① `autoGrow` 高度上限从 120px 增加到 150px；② `autoGrow` 给 scrollHeight 加增量（+4px）；③ CSS `#input` 的 `max-height` 从 120px 增加到 150px。
  - 需求：`web/static/app.js` 的 `autoGrow` 增加上限 + 加增量；`web/static/style.css` 的 `#input` 增加 `max-height`。
  - 实施：`web/static/app.js` `autoGrow` 改为 `Math.min(input.scrollHeight + 4, 150)`；`web/static/style.css` `#input` 的 `max-height` 从 120px 改为 150px [BUILD-509]
  - 测试：见 use-case/FEATURE-405/

- [ ] **FEATURE-406 修复主消息录入框默认状态下光标垂直方向不居中**
  - 背景：FEATURE-405 后录入框高度正常，但默认状态下光标和文字内容偏靠上，垂直方向不居中，上下间距不一样。用户建议增加上下 padding（4px → 6px）纠正。
  - 方案（已确认）：将 `#input` 的上下 padding 从 4px 增加到 6px，使光标和文字内容垂直居中。
  - 需求：`web/static/style.css` 的 `#input` 上下 padding 从 4px 增加到 6px。
  - 实施：`web/static/style.css` 的 `#input` 上下 padding 从 4px 改为 6px（`padding: 6px 12px`） [BUILD-512]
  - 测试：见 use-case/FEATURE-406/

---

## v0.8.0 — 开发中（已完成）

> **版本**: v0.8.0

> **状态**: ✅ 已完成
> **里程碑**: web UI 状态栏会话菜单
> **说明**: 0.8.0 系列专注 web UI 状态栏会话管理，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-387 | 0.8.0 | P1 | ✅ 已完成 web UI 状态栏会话菜单（💬 图标 + 会话数 + 悬停展开会话列表 + 切换/删除会话） |

> 当前 BUILD: 468
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

---

## v0.7.9 — 开发中（已完成）

> **版本**: v0.7.9

> **状态**: ✅ 已完成
> **里程碑**: 任务计划（taskplan）与会话（session）绑定
> **说明**: 0.7.9 系列专注任务计划与会话绑定，细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-386 | 0.7.9 | P1 | 任务计划与会话绑定（存储 key 加 session 前缀 + 切换会话加载目标会话计划 + 旧数据迁移） |

> 当前 BUILD: 459
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

---

## v0.7.8 — 开发中（已完成）

> **版本**: v0.7.8

> **状态**: ✅ 已完成
> **里程碑**: Web UI 文件列表标注 git 修改状态
> **说明**: 0.7.x 系列专注输出架构重构（见 docs/output-architecture.md），细分任务：

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-301 | 0.7.0 | P1 | ✅ 已完成（事件双枚举 + 回归基线 [BUILD-340]） |
| FEATURE-302 | 0.7.0 | P2 | ✅ 已完成（Out/RenderCommand + 渲染合并 [BUILD-341]） |
| FEATURE-303 | 0.7.0 | P3 | ✅ 已完成（向导迁移（B 类）+ i18n 归零第一步 [BUILD-342]） |
| FEATURE-304 | 0.7.0 | P4 | ✅ 已完成（外部入口迁移 + 分类开关 [BUILD-343]） |
| FEATURE-305 | 0.7.0 | P4.5 | i18n 归零冲刺（100% 达成） |
| FEATURE-306 | 0.7.7 | P2.5 | ✅ 已完成（输入统一 A1 全量事件流化 + Windows 补齐 + 系列回归修复 [BUILD-408]） |
| FEATURE-307 | 0.7.7 | P5 | ✅ 已完成（LineRenderer + StreamRenderer + WebRenderer + `serve` Web 界面 [BUILD-413/415/417]） |
| FEATURE-308 | 0.7.8 | tui v2 | FullScreenRenderer（可选分支） |

> 当前 BUILD: 433
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

---

## v0.7.8 — Web UI 文件列表 git 状态标注

> **状态**: 🚧 开发中

- [x] **FEATURE-375 web 文件列表标注 git 修改状态** ✅ 已完成 [BUILD-440]
  - 背景：Web UI 侧栏文件树需要直观显示每个文件在当前分支下的 git 修改状态（M/A/D/U/R），目录需汇总显示内部变更文件数；LLM 迭代完成后自动刷新
  - 实现：① `web/server.go` 新增 `gitStatusMap(root)` 执行 `git status --porcelain -z`（参数数组、无 shell）解析为 路径→状态码 映射，非 git 仓库返回空；`treeNode` 增加 `status` 字段（文件状态码）与 `changes` 字段（目录内变更文件数汇总）；`buildTree` 构建时填充；② `app.js` `treeNode()` 渲染文件字母徽标（M 蓝/A 绿/D 红/U 绿/R 紫）与目录计数徽标；`done` 事件触发 `loadTree()` 自动刷新；③ `style.css` 徽标样式；④ 文件树每层缩进统一为 1 个字符宽度（tw 箭头绝对定位不占宽度）；⑤ 刷新文件列表时保持目录展开/折叠状态（expandedDirs 集合）
  - 测试：`TestGitStatusMap`（修改/新增/删除/未跟踪/重命名/非仓库 子测试）+ `TestTreeStatus`（/api/tree 节点 status/changes 字段）；headless 浏览器验证徽标渲染、done 刷新、缩进 1 字符、展开状态保持；`go vet/test ./web/`、`node --check` 全绿

- [x] **FIX-376 提问工具（ask_followup_question）问题内容被截断** ✅ 已完成 [BUILD-443]
  - 背景：LLM 调用 ask_followup_question 提问时，显示的问题被截断（如"询问用户: 模型文件正在后台传输中（deepseek模型156G，预计2-3小时），ds...（截断）"），用户看不到完整问题，失去提问意义
  - 根因：`agent/tool_summary.go` 中 `ask_followup_question` 的 question 参数被 `truncate()` 截断到 `maxSummaryParamLen=80` 字符
  - 修复：① `ask_followup_question` 的 question 是用户必须阅读并回答的核心内容，改为不截断、显示完整问题（showTool 显示与确认提示两个场景均受益）；② 确认/提问后用户输入（确认选择、选项、录入内容）回显为 web UI 的 YOU 消息块（`answerAsk` 调用 `renderUserEcho`）
  - 测试：新增 `TestBuildToolSummaryAskQuestionFull` 验证长问题完整显示且无截断标记；headless 浏览器验证提问/确认提交答案后 YOU 消息块正确显示；`go test ./agent/`、`node --check` 全绿

- [x] **FEATURE-377 文件列表每次方法调用返回后自动刷新** ✅ 已完成 [BUILD-445]
  - 背景：文件列表自动刷新策略当前只在 `done` 事件（LLM 迭代完成）时刷新，一次迭代内多次工具调用不会实时刷新；希望每次方法调用（工具调用）返回后自动刷新文件列表（含分支状态）
  - 实现：`web/static/app.js` `renderEvent` 中 `tool_call` 事件 result 分支（工具执行完成）末尾调用 `refreshBranch()` + `loadTree()`，使每次工具调用返回后文件列表与分支状态实时刷新
  - 测试：headless 浏览器验证工具调用返回（tool_call result 事件）后文件列表自动刷新包含新文件；`node --check` 全绿

- [x] **FEATURE-378 web 底部状态条（上下文 token 统计）** ✅ 已完成 [BUILD-451]
  - 背景：用户消息框下方（窗口最底部）需要一条状态条，实时显示当前上下文长度统计；通过右上角下拉菜单新增开关控制是否显示
  - 状态条显示（三段，整体靠右）：`🧠{主模型原名}(89% of 1M)/👀{视觉模型原名}(50% of 200K)`（模型上下文占用，分子=最后一轮输入+输出）+ `会话 15000（↑14500 ↓500）`（会话总 token + 会话总输入/输出）+ `最后一轮 ↑4500（2250t/s, 2s) ↓500 (20t/s, 25s)`（最后一轮输入/输出 + 速度 + 用时）
  - 实现：`agent/loop.go` 新增 `ModelInfo()` 返回主/视觉模型原名（`ModelConfig.Model`）与 MaxModelLen；`web/server.go` 新增 `modelInfoFn` + `SetModelInfoProvider`，`handleBootstrap` 返回 textModel/textMaxLen/visionModel/visionMaxLen；`web/session.go` WebSession 注册 `sess.ag.ModelInfo`；`index.html` 状态条新增 sbModel 段；`app.js` bootstrap 读取模型信息，`updateStatus` 计算占用%（分子=最后一轮输入+输出 token/模型 MaxLen）并显示 🧠/👀；`style.css` 状态条样式（`#bottom` 加 `flex-wrap:wrap` + `.statusbar` 加 `flex-basis:100%` 独占一行占满全宽 + `justify-content:flex-end` 靠右）
  - 测试：Node DOM shim 加载真实 app.js 驱动假事件，34 项断言全过（默认显示、token_iter 更新会话/最后一轮两段、速度/用时计算、模型上下文占用 🧠/👀 用最后一轮分子、多次迭代累积、菜单开关切换与持久化、刷新后保持）；bootstrap API 实测返回模型原名 textModel/textMaxLen/visionModel/visionMaxLen；`node --check`、`go build/vet/test ./agent/ ./web/` 全绿

- [x] **FEATURE-380 文件列表 git 状态字母移到最左列** ✅ 已完成 [BUILD-454]
  - 背景：文件列表的 git 状态字母徽标（M/A/D/U/R）当前显示在文件名右侧，希望移到每个文件（不是文件夹）的最左边一列，紧靠文件夹列表左边框，跨过文件夹收起/打开箭头继续向左，且不能影响文件夹和文件的显示位置（不占用横向有效空间）；文件夹的修改文件数徽标（●N）保持现状
  - 实现：`app.js` `treeNode()` 将 `.git-status` 状态字母作为 `li`（.tree-node）的**直接子元素**（非 row 内），文件显示状态字母（M/A/D/R/U），文件夹留空；`style.css` 给 `.tree` 容器加 `position:relative`，`.git-status` 设为**绝对定位**（`left:2px`）**相对于 `.tree` 容器**，因此紧贴文件列表左边框（而非文件名）；`.tree-row` 增加 `padding-left:16px` 补偿状态字母列，使文件名横向位置不受影响、不占用额外有效空间；文件夹修改文件数徽标（●N）保留在右侧
  - 测试：Node DOM shim 加载真实 app.js 驱动 treeNode，13 项断言全过（文件行状态字母是 li 直接子元素且先于 row、文件夹行状态列留空、文件夹修改文件数徽标在右侧、文件行无 git-badge）；`node --check`、`go build/vet/test ./web/` 全绿

- [x] **FEATURE-383 web 文件列表三项 UI 优化** ✅ 已完成 [BUILD-457]
  - 背景：① 文件夹 git 修改状态图标及合计数应靠右显示（右对齐）紧贴 reveal 图标；② reveal 图标提示信息应改为定位到文件夹；③ 鼠标滑过超长文件名时左边栏（工作区）自动变为自适应宽度保证最长文件名显示完整，鼠标离开左边栏区域后 2-3 秒自动回归原固定宽度
  - 实现：① `app.js` `treeNode()` 将文件夹 git 徽标与 reveal 按钮放入 `.row-actions` 容器，`style.css` `.row-actions` 加 `margin-left:auto` 靠右、`.git-badge` 改 `margin-right:6px` 紧贴 reveal；② `reveal.title` 改用 i18n `T.revealDir`（zh 定位到文件夹 / en Reveal in folder）；③ `treeNode()` 给 `.tree-row` 加 `mouseenter` 监听，超长文件名（`scrollWidth>clientWidth`）时给 `#layout` 加 `sidebar-auto` class，`style.css` 新增 `#layout.sidebar-auto` 变体（grid 第一列 `auto` 自适应），boot 中 `#sidebar` 加 `mouseleave` 监听 2.5 秒后移除 `sidebar-auto` 回归固定宽度
  - 测试：Node DOM shim 加载真实 app.js 驱动 treeNode，16 项断言全过（含 git-badge 在 row-actions 内且先于 reveal、reveal title 本地化）；`node --check`、`go build/vet/test ./web/` 全绿

- [x] **FEATURE-384 状态栏三项 UI 优化** ✅ 已完成 [BUILD-458]
  - 背景：① 去掉主模型和视觉模型之间的 `/` 改为一个空格的固定宽度；② `会话` 文字改为 Σ 汇总图标；③ `最后一轮` 文字改为 🔄 适配图标（用户确认组合 B）
  - 实现：`app.js` i18n `sbSession` 值改为 `Σ`、`sbLast` 值改为 `🔄`（zh/en 通用图标）；`updateStatus()` 模型间分隔从 `/👀` 改为 ` 👀`（一个空格固定宽度）
  - 测试：Node DOM shim 加载真实 app.js 驱动假事件，37 项断言全过（含 sbSession 用 Σ 图标、sbLast 用 🔄 图标、模型分隔为空格非 /）；`node --check`、`go build/vet/test ./web/` 全绿

- [x] **FIX-385 web 人工确认选项不显示（约 50:50 无规律）** ✅ 已完成 [BUILD-459]
  - 背景：有些方法调用需要人工确认时，输出区域可能没有显示具体选项（比例约 50:50，无规律），用户不知道该怎么确认
  - 根因：确认选项文本通过 `io.Println`/`io.Printf`（WebIO.pushText → ui_text 事件）渲染到 `#stream` 事件流的 REPL 块；随后 `io.ReadLine`（WebIO.ask → ask 消息）触发前端 `showAsk` 显示底部 askArea 输入框。askArea 显示会撑高 `#bottom`、压缩 `#stream` 高度，但 `showAsk` 未重新滚动 `#stream`——当 `#stream` 内容较多已滚动时，选项文本被压缩出视野（50:50 取决于 `#stream` 是否已滚动）
  - 修复：`app.js` `showAsk()` 末尾（`askInput.focus()` 之后）追加 `scrollStream()`，askArea 显示后重新滚动 `#stream` 到底部，确保确认选项文本可见
  - 测试：`node --check`、`go build/vet/test ./web/` 全绿

- [x] **FEATURE-386 任务计划与会话绑定** ✅ 已完成 [BUILD-460]
  - 背景：任务计划（taskplan，即 `track_task_progress` 维护的待办任务清单）当前是全局单例，用固定 key `"current"` 存储单个全局任务计划，无 session 维度。切换会话（`handleSwitch`）只切 `currentSessionID` 和消息历史，不触碰 taskplan，导致所有会话共享同一份任务计划，切换后任务清单互相污染、混乱
  - 方案（已确认 1A 2A 3A）：① 存储 key 加 session 前缀（`current:{sessionID}`），每个会话一份独立计划；② 切换会话时自动加载目标会话绑定的任务计划（在 `Agent.SetCurrentSessionID` 中统一通知 `taskPlanMgr.SetSessionID`，覆盖所有切换路径）；③ 升级时把现有全局 `"current"` 计划迁移到当前会话名下
  - 实现：`taskplan/taskplan.go` 增加 `sessionID` 字段、`SetSessionID`、`planKey`、`migrateLegacyPlan`；`loadCurrent`/`saveCurrent`/`DeleteContext` 改用 `planKey`；`agent/agent.go` `SetCurrentSessionID` 中通知 `taskPlanMgr.SetSessionID`；planCounter 保持全局递增；sessionID 为空时回退全局 `"current"` key
  - 测试：`taskplan/taskplan_test.go` 新增 6 个单元测试（planKey 按 session 隔离、各会话计划隔离、会话内更新不影响其他会话、旧数据迁移、迁移保留已有会话计划、归档仅作用于当前会话）全过；`go build/vet/test ./...` 全绿

- [x] **FEATURE-387 web UI 状态栏会话菜单** ✅ 已完成
  - 背景：web UI 状态栏需要增加会话管理入口，方便用户查看/切换/删除会话
  - 需求：① 状态栏增加 💬 图标 + 会话数；② 鼠标移到该区域时向上展开会话菜单；③ 菜单列出所有会话标题；④ 鼠标悬停会话项时提示关键字 + 时间；⑤ 点击会话项切换到目标会话；⑥ 每个会话左侧（左对齐）设置删除符号，点击删除会话
  - 实现：`agent/io.go` 新增 `SessionListPusher` 接口；`agent/agent.go` `SetCurrentSessionID` 在锁外推送会话列表（覆盖 `:new` 路径，修复新建会话后会话数不刷新）；`web/session.go` `WebIO` 实现 `PushSessionList`（回调到 `WebSession.pushSessionList`），移除 `switchSession` 冗余推送；前端 `index.html`/`app.js`/`style.css` 增加删除确认 modal（复用 `.modal` 样式），删除前弹确认框，确认后发送 `session_delete` 并刷新会话数
  - 测试：`web/session_test.go` 新增 `TestSessionListPushedOnSetCurrent`，修正 `TestSessionSwitch`/`TestSessionDelete` 消费 `SetCurrentSessionID` 推送的额外 sessions 消息；`go build/vet/test ./...` 全绿；浏览器验证 `:new` 后会话数自动 +1、删除前弹确认框、取消不删、确认后会话数 -1；当前会话显示 ● 活动指示符（不可删除）而非 ✕ 删除按钮 [BUILD-468]
  - 测试：见 use-case/FEATURE-387/

---

## v0.7.7 — 输入统一

> **状态**: 🚧 开发中
> **里程碑**: 输入统一（InputSource）+ Windows 补齐

### 功能清单

- [x] **FEATURE-306 输入统一（InputSource）+ Windows 补齐（P2.5）** ✅ 已完成 [BUILD-408]
  - 背景：v0.7.5 完成（视觉隔离 + 问题判定异常场景接入），FEATURE-306 原顺延至 v0.7.6；v0.7.6 已收尾，按用户决策新建小版本 v0.7.7 承接
  - 目标：统一 REPL/单次执行/子命令的输入源抽象（InputSource），补齐 Windows 平台输入兼容
  - 方案（A1 全量事件流化，用户已确认）：
    - InputSource 接口 + InputEvent 结构（agent/input.go 扩展，P1 已有 InputKind 枚举）
    - RawKeySource（tui）：单一 Reader goroutine 唯一读 stdin，复用 readCSI/readSS3 解析 ESC 序列 → 语义化 InputEvent，事件广播给订阅消费者
    - StdioSource（stdio）：同步行读 → InputLine/EOF，行为与现状 bufio.Scanner 完全一致
    - ESC/Ctrl+C 监控从独立 unix.Poll 轮询 goroutine 改为事件流消费者（repl_esc_posix.go / repl_esc_windows.go 合并，Windows no-op 消除）
    - EnhancedInput 全量事件流化：ReadLine 从事件流消费，buffer/cursor/history/渲染逻辑原样保留
    - EnhancedIO.ReadLine/ReadKey 改为消费事件流；系统命令（sudo/passwd）执行期间 Reader 暂停让出 stdin
    - cmd 内置命令向导回归修复：向导执行期间 Pause InputReader（停止抢读 + 恢复 cooked 模式），readLine 增加 paused 回退（bufio.Scanner 直接读行），:continue 先 Resume 再进 agent——解决向导显示乱码（无 \r）与输入无反应（Reader 抢读）
    - raw 模式输出换行修复：A1 后 raw 模式常驻，REPL 主循环阶段的直接 fmt 输出（命令输出/LLM 回显/help/清理等）只有 \n 没有 \r → 光标不归位；新增 REPL rawPrint/rawPrintf/rawPrintln（raw 激活时 \n→\r\n 转换，含行首 \r），替换 repl.go 中所有用户可见输出（审计 Direct fmt 200→143）
    - replace_in_file 字面转义修复：LLM 在 XML 模式把 search/replace 换行写成字面 \n/\t 转义导致显示与执行不一致——方案 A（zh/en 系统提示新增规则：search/replace 必须用真实换行、字面反斜杠用 \\）+ 方案 B（agent/toolcall_mode.go 新增 decodeXMLEscapes：jsonValue 字符串分支解码 \n/\t/\r/\\/\"，未知转义保留反斜杠，与 JSON 模式 decodeJSONEscape 对齐）；feedLined 加 debug 日志（内容含反斜杠时记录）供定位显示层反斜杠；测试 agent/xml_escape_test.go（decode 8 子用例 + 字面/真实换行集成）
    - 工具调用前导换行修复：流式工具头（EventToolCallStream）此前直接 emit "⚙️ write_to_file"，与前面 LLM 内容粘连；ToolCallRenderer 新增 leadingEmitted 状态，首个工具头前加一次 \n（emitToolHeader），与 EventToolCall 的 REPL 前导换行一致，后续工具头不重复空行；测试 TestToolCallStream_LeadingNewlineBeforeHeader
    - main.go 放开 Windows 强制 stdio，支持 --input-mode tui
    - stdio 管道修复：REPL 主循环改用持久 StdioSource（原每次 new bufio.Scanner 会吞掉缓冲中的后续管道行）+ EOF 返回 io.EOF 正常退出（原无限循环打提示符）
  - 验收：方向键/ESC/Ctrl+C 双平台通过（Windows 交叉编译 + pty 模拟 tui 真机验证）；--input-mode stdio 管道行为不变（`:help`+`exit` 管道正常退出）
  - 验证：`go build ./...`、`go vet ./...`、`go test ./...` 全绿；Windows 交叉编译通过；`bin/output_audit.sh --strict`（Hardcoded Chinese=0 / i18n missing=0）；use-case/FEATURE-306/FEATURE-306-UC-0001.md 共 18 用例（UC-0001~0018，含 cmd 向导回归/raw 换行/字面转义/前导换行）

- [ ] **FIX-350 judge 产出雷同 guidance 的对策（失败策略哨兵分隔格式 + 禁止字面雷同措辞强化）**
  - 背景：FEATURE-349 上线后实测，循环跑到一定程度后 judge 多轮返回的 guidance 趋于雷同甚至几乎完全一样——judge 与主 LLM 同源（deepseek-v4-flash），自身也陷入重复产出；原失败策略列表以 `1. ...` 纯编号行渲染，与 guidance 内容中的列表样式容易混淆，且"禁止重复"的措辞约束力不足
  - 目标：让 judge 清晰区分"历史意见列表的结构"与"意见内容"，并以最硬措辞禁止新 guidance 与上一次雷同
  - 实现：
    - 失败策略列表改为带序号的哨兵包裹格式 `[FAILED-STRATEGY #N BEGIN]` / `[FAILED-STRATEGY #N END]`（agent/loop.go `buildFailedStrategiesText`）：verbose ASCII 哨兵独占一行，与 guidance 内容不可能冲突
    - judge 用户提示段标题强化（zh/en）：新 guidance 绝对不得与列表中任何一条相同或近似雷同——尤其是最近一条；主LLM仍在同一处循环时必须彻底更换措辞与切入角度
    - judge 系统提示第 6 条同步强化（zh/en）：新 guidance 与最近一条不得有任何字面级别的雷同；条目中说明哨兵格式便于 judge 解析
  - 测试：agent/feature349_test.go 更新编号列表用例为哨兵格式断言 + 新增对抗性用例（策略内容含 `1. ` 与 `[FAILED-STRATEGY` 片段时结构仍清晰）；go build/vet/test 全绿 [BUILD-397]

- [x] **FIX-353 兼容单 chunk 完整下发的工具调用（首个 delta 不再丢 arguments）** ✅ 已完成 [BUILD-409]
  - 背景：接入本地 mlx-vlm 服务（Qwen3.8-27B）后所有工具调用报"arguments is empty"。排查发现 mlx-vlm 把 id+name+完整 arguments 放在一个 chunk 一次性下发，而 llm/client.go 的累积逻辑只在条目已存在时追加 Arguments，首 chunk 携带的参数被丢弃；OpenAI/vLLM 风格（首 chunk 仅 id+name、参数分片随后）则一直正常
  - 实现：llm/client.go 流式累积 `!exists` 分支初始化 ToolCall 时保留 `tc.Arguments`，两种下发风格均兼容
  - 测试：llm/fix353_test.go（单 chunk 完整下发 + 分片追加回归，已验证修复前 FAIL/修复后 PASS）

- [x] **FIX-354 跨 tool 消息不再刷新 user 消息时间（前缀缓存大面积失效修复）** ✅ 已完成 [BUILD-410]
  - 背景：远程 deepseek 多轮工具调用后期 prefill 急剧变慢。日志分析发现 prompt_cache_hit 冻结在 65280、miss 每轮增长：refreshLastUserEnvelope 每个 iteration 无条件回扫改写最后一条 user 消息的 `<time>`，该消息位于历史中部时其后全部内容无法命中前缀缓存（实测每轮重算 55K+ token）。FEATURE-327 已将 retried_count 定位扩展为 user||tool，时间刷新未同步该原则
  - 实现：agent/envelope.go 回扫时先遇到 tool 消息即放弃刷新——只在 user 消息仍处于历史尾部（其后最多 assistant 消息）时刷新时间
  - 测试：agent/fix354_test.go（尾部刷新保留 + 跨 tool 不刷新，修复前 FAIL/修复后 PASS）

- [x] **FEATURE-355 execute_command 超时参数必填化 + on_timeout kill/detach** ✅ 已完成 [BUILD-411]
  - 背景：超时的"停止等待"与"终止进程"两个语义被混淆；起服务/长构建场景超时即杀全组且部分输出全丢。另外调查发现 `A && nohup B > log 2>&1 &` 写法（&& 列表整体后台）子 shell 持有输出管道导致 Wait 阻塞，是"后台服务总是被超时杀掉"的实际成因
  - 目标：timeout_seconds 必填（0=无限等待）；新增必填 on_timeout：`kill` 维持原行为，`detach` 超时后停止等待、返回 PID+部分输出+日志文件路径，进程后台继续运行，后续可 tail 日志或 kill
  - 实现：agent/tools.go schema 两参数必填；agent/command_tools.go detach 模式输出写临时日志文件（避免返回后内存 buffer 无限增长）且不连接 stdin（避免与 REPL 竞争），后台 reaper 协程完成 Wait/关文件/Release；tool_error.go 提示与三处 README 同步
  - 测试：agent/feature355_test.go 7 用例（必填校验×3/0 超时无限等待/kill 超时杀进程组/detach 存活+部分输出+PID 与日志路径）；既有 FIX-320 测试按新接口适配

- [x] **FIX-356 XML 工具说明补齐 execute_command 新必填参数** ✅ 已完成 [BUILD-412]
  - 背景：FEATURE-355 后 XML 调用方式的工具说明（i18n en/zh）仍将 timeout_seconds 描述为可选且无 on_timeout，XML 模式下 LLM 不知道新参数
  - 实现：i18n/en_system.go / zh_system.go 的 KeyToolUsageExecuteCommand 文案与示例同步；确认 XML 参数经 jsonValue 自动类型转换（数字→JSON number），解析链路无需改动

- [x] **FIX-357 修复 LLM 输出期间按 ESC 无效** ✅ 已完成 [BUILD-414]
  - 背景：FEATURE-306（BUILD-408）统一 InputSource 后，`RawKeySource.parseByte` 收到 0x1b 时为区分方向键等 ANSI 序列会调用 `readEscapeSequence` 无限期阻塞等待第二个字节；单独按 ESC 无后续字节，`InputEsc` 事件永远发不出，打断信号到不了 agent。日志佐证：work/log 中 `ESC detected` 8/14 还有 27 次，8/15 起归零。既有测试用立即 EOF 的假 reader 未覆盖真实终端的阻塞行为
  - 目标：LLM 流式输出期间按 ESC 可打断（恢复 FEATURE-201 行为），方向键等转义序列不误判
  - 实现：repl/raw_key_source.go ESC 序列读取加 50ms 超时窗口（escSeqTimeout，真实转义序列后续字节立即到达，超时即独立 ESC 键）；Pause/Close 引发的父 context 取消向上传播错误，不再误报幻键 ESC；Windows 侧复用既有 cancelIoEx 机制无需改平台代码
  - 测试：repl/raw_key_source_test.go 新增 4 用例（阻塞式假 reader：独立 ESC 超时检出 / ESC 后迟到的普通键不丢失 / 方向键正确识别 / 父取消不误报 ESC）；已验证 2 个关键用例修复前 FAIL、修复后 PASS；go test ./... 全绿；audit 与基线持平（143/0/2/18/0）

- [x] **FIX-358 修复全新 workspace 首运行配置向导 EOF 空转刷爆 stdout** ✅ 已完成 [BUILD-416]
  - 背景：全新 workspace 首次以管道/非终端方式启动时，无模型配置触发 AddModelWizard，向导循环 `readLine()` 吞掉 ReadLine 错误返回 ""；而 `DefaultUserIO.ReadLine` 在干净 EOF 时 `bufio.Scanner.Err()` 为 nil，EOF 与空行不可区分，于是向导无限重印提示——307b 冒烟实测几秒内灌出 1.75GB stdout
  - 目标：非终端 stdin 下配置向导快速失败并给出可操作提示；EOF 在 IO 层可区分；消除 read-ahead 丢行隐患
  - 实现：agent/io.go DefaultUserIO.ReadLine 干净 EOF 返回 io.EOF + scanner 持久化（新建 scanner 每次丢预读字节）；cmd/model.go AddModelWizard 入口 tty 守卫（osStdinIsTerminal，可注入便于测试）+ 新增 i18n key KeySetupNonInteractive（zh/en）；main.go 向导失败时 stderr 输出错误明细
  - 测试：agent/io_test.go 3 用例（EOF 可区分且粘滞 / 两行一次到达不丢 / 空行≠EOF）；cmd/model_wizard_test.go 非终端守卫快速返回；复现场景回归（空 workspace 管道启动：无刷屏、stderr 明确报错、exit=1）；go test ./... 全绿；audit 与基线持平

- [x] **FEATURE-359 web 界面布局重构 + 贝壳 logo 马赛克 + 拖放简化** ✅ 已完成 [BUILD-418]
  - 背景：307c web 原型基础上的一轮 UI 打磨——底部通栏截断侧栏、logo 缺失、专用拖放区冗余
  - 目标：左下角放入贝壳 logo 马赛克；工作区侧栏贯通到底；拖放直接落在目录树
  - 实现（仅 web/static/ 三文件，Go 零改动）：① 布局改两行网格——侧栏 grid-row 1/-1 贯通到底，底部 ask+输入区 grid-column 2/-1 左缘紧贴侧栏右缘；② logo 马赛克按贝壳语义手工绘制（草帽形上壳 3 行 + 大眼睛 2 行 + 连续浅碗下壳 2 行，7×16 格子），CSS grid 色块按字符强度映射 accent 透明度、明暗双主题自适应，高度动态约束不超过输入框；③ 删除专用拖放区——文件夹节点 drop 上传到该目录（stopPropagation 防冒泡），面板空白/文件节点 drop 默认根目录，侧栏整体弱高亮 + 节点高亮反馈；附件按钮移至工作区标题栏（保留附件 chip 引用流）
  - 测试：headless Chrome 截图明暗双主题验证布局/logo/贯通；upload API 根目录与子目录落盘 curl 验证；node --check 通过

- [x] **FEATURE-360 贝壳 logo 穹顶加圆微调** ✅ 已完成 [BUILD-419]
  - 背景：FEATURE-359 马赛克 logo 的用户迭代收尾——穹顶两侧再向外鼓一点，帽形更圆
  - 实现（仅 web/static/app.js 的 LOGO_ART 两行）：第 1 行 `      ####` → `     ######`（加宽居中），第 2 行 `    ##    ##` → `   ###    ###`（两侧外鼓一格）
  - 测试：headless Chrome 截图放大验证轮廓（上壳饱满、眼睛与下壳不变）

- [x] **FIX-361 web 事件流消息被压缩成细条、无法滚动** ✅ 已完成 [BUILD-420]
  - 背景：307c web 界面长会话时，事件区每条消息被挤成一条细带、无滚动条、内容完全不可读（用户截图反馈）
  - 根因：`.stream` 是 `display:flex; flex-direction:column` 容器，子元素 `.ev` 默认 `flex-shrink:1`——内容总高超过容器时，flex 布局优先等比压缩子元素去填满容器，溢出根本不发生，`overflow-y:auto` 永远不会出现滚动条
  - 修复（仅 web/static/style.css 一处）：`.ev` 增加 `flex-shrink: 0`，子元素保持自然高度、容器正常溢出滚动
  - 测试：headless Chrome 加载真实 style.css 的测试页（60 条事件、容器 300px 高）断言 `scrollHeight > clientHeight` 且首条事件保持自然高度、`scrollTop` 可滚到底

- [x] **FEATURE-362 web 事件流：工具调用合并单块 + Markdown 实时渲染** ✅ 已完成 [BUILD-421]
  - 背景：307c web 界面的两个体验问题（用户截图反馈）——一次工具调用的流式参数片段各自成块（一个 bash 命令碎成十几条 TOOL 块）；LLM/工具输出的 Markdown 全部按字面纯文本显示（`**粗体**`、```围栏、表格均可读性差）
  - 目标：一次工具调用 = 一个块（流式片段累积 → 输入摘要替换 → 结果追加）；LLM 与 TOOL 块内容按 Markdown 实时渲染
  - 实现：① Go 侧 `agent/events.go` 新增 `MetaKeyPhase`/`PhaseInput`/`PhaseResult` 与 `withPhase()`，`run_stream.go` 四处 `EventToolCall` 发射点打上 input/result 标记（Meta 被 LineRenderer 忽略，终端行为与 golden 测试零影响）；② 新增 `web/static/md.js`——手写 Markdown 子集渲染器（标题/粗斜体/删除线/行内代码/围栏代码块/管道表格/列表/引用/分隔线/链接），纯 DOM API 构建节点、不喂 innerHTML，天然防 XSS；`_emphasis_` 要求非单词字符相邻，snake_case 标识符不误伤；③ `app.js` 流式块改为 `{body, raw, raf}` 累加器，每片段整体重解析（rAF 节流），半截语法（未闭合围栏等）随下一片段自愈；`tool_call_stream` 片段累积进当前 TOOL 块，phase=input 替换为干净摘要，phase=result 追加并 MD 渲染，同块展示；command/output 终端回显保持等宽纯文本；④ `style.css` 补 `.ev-body.md` 下 MD 元素样式（双主题沿用 CSS 变量）
  - 测试：Node + 最小 DOM shim 加载真实 md.js/app.js、驱动假事件断言 17 项全过（块数、合并、MD 结构、snake_case 保护）；headless Chrome 截图目视确认（过程中借此发现并修复标题分支漏 `i++` 的死循环）；`go build/vet/test` 全绿；audit 持平基线；`git diff go.mod go.sum` 为空

- [x] **FIX-363 web 主题兜底改为深色** ✅ 已完成 [BUILD-422]
  - 背景：web 界面主题初始化优先级为 localStorage 手动选择 → 跟随 OS（prefers-color-scheme）→ 兜底；原兜底为浅色，与产品深色主基调不一致
  - 修复（仅 app.js initTheme 一行）：`window.matchMedia` 不存在（浏览器无法上报系统色彩方案）时兜底深色；index.html 的 `data-theme="dark"` 首屏防抖属性本就一致
  - 测试：Node DOM shim 模拟无 matchMedia 环境加载真实 app.js，断言初始化后 `data-theme="dark"`

- [x] **FEATURE-364 Web UI 完整设计文档** ✅ 已完成 [BUILD-424]
  - 背景：web 界面（307c 起，历经 FEATURE-359/360/362、FIX-361/363）已具规模，需要一份自包含文档让接手者不借助其他资料即可开发新功能或改进
  - 实现：新增 `docs/web-ui-design.md`——概述与 BUILD 演进索引、总体架构图、启动链路（serve 子命令解析/冲突检查/降级链）、web 包四文件逐一解剖（server/ws/session/open）、WS 通信协议全表（含 StreamEvent 类型速查与 phase 语义）、前端四文件模块地图（含流式累加器/rAF 节流/flex-shrink 教训/md.js 扩展边界）、事件流与 REPL 背景最小集、测试方法论（Go 单测/Node DOM shim/headless 截图/真机冒烟）、开发规范、扩展食谱（加 API/加消息/加事件渲染/加面板/auto-serve 决策链）、已知限制
  - 测试：纯文档；文中文件/符号/行号引用已对照 BUILD-423 代码逐一核实

- [x] **FEATURE-365 web 界面三项改进：底部通栏 + 面板菜单 + 系统设置** ✅ 已完成 [BUILD-425]
  - 背景：用户提出的三点 UI 改进——录入区延伸到页面最左端；去掉上传图标只留拖拽；右上角加下拉菜单控制面板可见性并提供系统设置入口
  - 实现（仅 web/static 三文件，Go 零改动）：① 布局改为底部通栏——`#bottom` 跨满 `grid-column:1/-1`，logo 移入底栏最左端与录入框同一外框（右缘细分隔线），底栏上边界即工作区清单下边界；侧栏收缩为只占首行；② 删除 📎 附件按钮、fileInput 与附件 chips 全链路（input 消息只带 text；WS 协议 attachments 字段保留，服务端行为不变）；③ 顶栏新增 ☰ 悬停下拉菜单——🗂 工作区 / 📋 任务进展 开关（带 ✓ 状态，偏好持久化 localStorage `co-shell-panels`，`applyPanels()` 统一判定，`no-ws`/`no-plan` class 驱动 grid 列归零）+ ⚙️ 系统设置弹层（主题三态：跟随系统/深色/浅色——`co-shell-theme` 新增 auto 值，`matchMedia change` 实时跟随，`setTheme` 改为只应用不落盘避免覆盖 auto；顺带解决了此前"手动选择无法解除"的已知限制）；菜单 hover 桥接用 wrapper 纵向 padding 消除间隙
  - 测试：Node DOM shim 断言 21 项全过（主题三态切换/OS 变更跟随/手动钉死、面板开关与持久化、无附件发送）；headless Chrome 截图三张（默认布局+菜单展开+计划面板、隐藏工作区、浅色主题+设置弹层）；`node --check` 通过；`go build` 通过

- [x] **FEATURE-366 web 界面四项改进：计划面板高亮 + 按钮对调 + 新 logo/favicon + 工作区路径标题** ✅ 已完成 [BUILD-426]
  - 背景：365 之后的又一轮 UI 打磨——任务进展标题与条目需要更统一的视觉层级；菜单/主题按钮顺序不顺手；logo 换用用户新绘的 7×14 图样并需要浏览器标签图标；多标签页难以区分各自的工作区
  - 实现：① 计划面板样式（style.css）——`.plan-title` 取消加粗、字号与条目一致（12.5px），仅靠 accent 高亮 + 加大下间距区分；`.plan-step .desc` 条目文字同步 accent 高亮；② index.html 顶栏 themeToggle 与 ☰ 菜单位置对调（菜单移到最右端）；③ `LOGO_ART` 整体替换为用户手绘的 7 行 × 14 列新图样（穹顶上壳 3 行 + 流苏中缝 1 行 + 下碗 3 行），并据此生成 `web/static/favicon.svg`（64 个 1×1 rect、`shape-rendering:crispEdges`、accent 青 #3fd6ef、随 embed.FS 内嵌），index.html 加 `<link rel="icon">`；④ `boot()` 用 `/api/bootstrap` 返回的 workspace 绝对路径设置 `document.title`——浏览器标签即"图标 + 工作区路径"，多实例一目了然（bootstrap 本已返回 workspace，Go 零改动）
  - 测试：headless Chrome 截图验证（计划面板高亮与间距、按钮顺序、底栏新 logo）；favicon.svg 经 `<img>` 放大渲染目视确认轮廓与 7×14 图样逐格一致；harness 页将 `document.title` 写入 DOM 截图确认标题生效；`go vet`、`go test ./web/ ./agent/` 全绿

- [x] **FIX-367 修复 web 输入框 ↑↓ 无法翻历史、↓ 误清草稿** ✅ 已完成 [BUILD-427]
  - 背景：web 输入栏按 ↑↓ 不能翻历史消息，按 ↓ 反而清空当前输入（用户反馈）
  - 根因（web/static/app.js 输入区三处叠加）：① `histPos` 初值 -1 与"历史为空"叠加时 `-1 === history.length - 1` 成立，↓ 直接把草稿替换成空 `histDraft`；② ↑ 要求 `selectionStart === 0`，而程序设值后光标停在条目末尾，第二次 ↑ 永远不触发；③ ↓ 无任何光标位置/状态守卫，随时可能动草稿
  - 修复：`histPos` 改为 `history.length` 作为"未发送草稿"哨兵（初值 0）；↑ 仅在光标位于首行、↓ 仅在末行时才翻历史（多行草稿内正常移光标）；新增 `recallHistory()` 统一设值并把光标停到条目末尾；历史为空时 ↑↓ 直接返回不碰输入；仅在实际翻页时 preventDefault
  - 测试：headless Chrome harness 驱动真实 app.js + 真实 KeyboardEvent，11 项断言全过（空历史 ↓ 不清草稿、连续 ↑ 翻旧、到顶停住、↓ 翻新、越过最新恢复草稿、多行首行/末行判定）；同一 harness 对旧代码复跑，1/3/4/5 项 FAIL 作回归对照；`node --check` 通过

- [x] **FEATURE-368 贝壳 logo 图样调整（第二轮）** ✅ 已完成 [BUILD-428]
  - 背景：用户新绘 7×14 图样替换 366 版——穹顶次行收窄为 10 格、第 3/6 行改为 4+3+3 分段、流苏中缝两端填满（`##`…`###`）、下碗各行同步微调
  - 实现：`app.js` LOGO_ART 七行整体替换；`favicon.svg` 按新网格重新生成（63 格）
  - 测试：favicon 经 `<img>` 放大渲染逐格目视比对与新图样一致；`node --check` 通过

- [x] **FEATURE-369 web 界面三项基础改进：面板白色高亮 + logo 等距 + 播放/暂停合并按钮** ✅ 已完成 [BUILD-429]
  - 背景：366 的 accent 青色高亮在任务面板上过于花哨；logo 与输入框之间的分隔线多余；发送/打断两个按钮占空间且语义互斥，可合并
  - 实现：① 计划面板高亮改纯前景色（白）——`.plan-title` 去 glow 用 `--fg`；条目仅首行高亮（renderPlan 按首个 `\n` 拆分，首行包 `.hl` span），续行回退 `--fg-dim`；② `.logo` 去掉右缘分隔线，padding 对称 16px + `.bottom-main` 左 padding 归零，图标两侧等距；③ 发送/打断合并为单按钮 ▶/⏸——`running` 状态前端在 sendInput 立即置位，精确边界由 Go 侧 WebSession.ReadLine 在阻塞前/取到输入后直发 `await_input`/`turn_start` 事件（web 本地常量，不经 renderer，TUI/stdio 无感）；运行中点击发 interrupt、空闲点击发送；WS 断开复位；运行态按钮转 error 红色样式
  - 测试：headless Chrome harness 驱动真实 app.js + 可捕获 WS stub，13 项断言全过（按钮状态机五态转换、interrupt 消息发出、空输入不发送、条目首行/续行配色、logo 无边线与底栏零左 padding）；`go vet/test ./web/` 全绿；`node --check` 通过

- [x] **FEATURE-370 logo 第三版 + 标签标题 👀 前缀** ✅ 已完成 [BUILD-430]
  - 背景：用户提供 logo.txt 第三版图样（穹顶第 3 行改 12 格实心、第 6 行改 3+2+3 窄分段）；希望浏览器标签标题带 co-shell 的 👀 标识
  - 实现：`LOGO_ART` 七行按 logo.txt 整体替换；`favicon.svg` 同步重生成（63 格）；`boot()` 的 `document.title` 改为 `"👀 " + workspace`
  - 测试：favicon 放大渲染逐格比对一致；harness 断言 `document.title === "👀 /tmp/ws"` 通过；底栏 logo 截图目视确认新轮廓；`node --check`、`go build` 通过

- [x] **FEATURE-371 logo 12×12 近方形版 + favicon 改 24×24 PNG** ✅ 已完成 [BUILD-431]
  - 背景：用户更新 logo.txt 为 12×12 带上下留白的新图样；favicon 要求按字符版光栅化——一个字符占 4 像素正方形（2×2px）、空格透明、最终 24×24 单图标
  - 实现：`LOGO_ART` 换 12×12 网格（含空行留白）；`favicon.png` 由 logo.txt 逐格光栅化（2×2px/字符、accent 青 RGBA、纯 Python 标准库手写 PNG 编码，130 字节）；index.html 图标链接 svg→png，`favicon.svg` 删除
  - 测试：favicon.png 原尺寸与放大目视逐格确认与 logo.txt 一致（透明留白正确）；headless 截图确认底栏 12×12 logo 新构图与等距；`node --check`、`go build` 通过

- [x] **FIX-372 标签标题去掉 👀 前缀** ✅ 已完成 [BUILD-432]
  - 背景：370 加的 👀 前缀经实际使用被认为多余，标题回归纯工作区路径
  - 修复（仅 app.js 一行）：`document.title = b.workspace`
  - 测试：harness 断言 `document.title === "/tmp/ws"`（裸路径、无前缀）通过

- [x] **FEATURE-373 web 工作区标题栏显示分支名 + UI 改进** ✅ 已完成 [BUILD-434]
  - 背景：工作区标题栏（侧栏 panel-head）需要显示当前 git 分支名，便于多分支开发时一眼识别；分支名应在 LLM 迭代完成时自动刷新；标题栏 UI 需优化（刷新按钮去边框放大紧贴工作区、分支名靠右截断）
  - 实现：① 服务端 `web/server.go` 新增 `gitBranch(root)` 读取 `.git/HEAD` 解析分支名，`handleBootstrap` 返回 `branch` 字段；② 前端 `app.js` 新增 `refreshBranch()`，页面加载与 `done` 事件（LLM 迭代完成）时重新获取并更新 `#wsBranch`；③ `index.html` 调整标题栏结构（刷新按钮移入 `.ws-title` 组紧贴工作区、分支名单独靠右）；④ `style.css` 刷新按钮去边框放大一倍（24px）、分支名靠右 + 超长截断（`text-overflow: ellipsis`）、刷新按钮像素级垂直对齐（`translateY` 微调）
  - 测试：`TestGitBranch`（分支 ref/无仓库/detached HEAD 三子测试）+ `TestBootstrap` 验证 branch 字段；headless 浏览器验证分支名显示、`done` 事件触发刷新、超长分支名截断、刷新按钮像素级对齐；`go vet/test ./web/`、`node --check` 全绿

- [x] **FEATURE-374 web UI 执行 REPL 命令输出传到浏览器** ✅ 已完成 [BUILD-435]
  - 背景：web UI 执行 `:set` 等 REPL 命令时，输出内容显示在终端窗口（服务端），没有传到浏览器；希望输出以 REPL 块（平行于 TOOL/LLM）显示在浏览器事件流，人工输入/选择内容传回后台
  - 实现：① `agent/out.go` 新增 `ChannelREPL` channel；② `web/session.go` `WebIO.pushText` 的 `ui_text` 事件 channel 改为 `ChannelREPL`；③ `repl/repl.go` `handleBuiltin` 中 `fmt.Print*`（7 处）改为通过 `agent.GetIO(r.agent)` 输出（web 模式走 WebIO → 浏览器，终端模式走 DefaultUserIO → 终端）；④ `app.js` `CHAN_LABEL` 添加 `repl: "REPL"`、`eventClass` 对 repl channel 的 `ui_text` 返回 `repl` class、`renderEvent` 显示 REPL 标签；⑤ `style.css` 为 `.ev.repl` 添加样式
  - 测试：headless 浏览器通过 WebSocket 发送 `:set` 命令，事件流出现 REPL 块（标签 REPL、内容为设置帮助）；`go vet/test ./repl/ ./web/ ./agent/`、`node --check` 全绿

---

## v0.7.6 — 输入统一（已完成）

> **状态**: ✅ 已完成
> **目标日期**: 2026-08-12
> **里程碑**: browser_screenshot 视觉识别一致化 + 循环介入 auto 策略

### 功能清单

- [x] FEATURE-346 browser_screenshot 视觉识别与 FEATURE-343 一致化（minimal 识别轮接入）：[BUILD-390]
  - 背景：FEATURE-343 的 minimal 识别轮（上下文折叠 [Identity-only, intent+图片]、tools 清空、结果回填工具返回）仅 visual_analysis 走；browser_screenshot 截图后只注入 imagePaths、未设置 visionPendingIntent，识别在完整上下文内联进行（UC-0019 有意排除），与 visual_analysis 行为不一致
  - 目标：browser_screenshot 视觉支持时也设置 visionPendingIntent（取 intent 参数，缺失用本地化默认指令兜底），记录 ToolCallID/工具名并回填识别结果，minimal 模式下与 FEATURE-343 完全一致；full 模式行为不变
  - 实现：
    - agent/browser_tools.go：视觉支持时设置 visionPendingIntent
    - agent/tools.go：recordVisionToolCall 记录 visual_analysis/browser_screenshot 的 ToolCallID 与工具名
    - agent/loop.go + agent/image_tools.go：重构图片注入（encodeMediaContentPart），修复 OpenAI 模式识别轮图片丢失（不依赖最后消息角色）
    - agent/run_stream.go：识别轮结果回填使用记录的工具名（XML 模式）
    - agent/system_prompt.go + agent.go：resolveAgentDescription 同源解析，识别轮 system prompt 含 agent 身份描述
    - i18n keys/zh/en + zh_system/en_system：默认识别指令 + browser_screenshot intent 引导说明（指导性指令）
  - 编号说明：FEATURE-23 与历史 ENHANCEMENT-23 冲突，改用未使用的最小编号 FEATURE-346
  - 测试：agent/fix346_test.go 6 单测 + use-case/FEATURE-346/FEATURE-346-UC-0001.md（31 用例）；go build/vet/test 全绿；audit Hardcoded Chinese=0 / i18n keys missing=0；cc 编译 co-shell v0.7.6 [BUILD-390]
- [x] FEATURE-347 log=debug 时第一时间输出原始 LLM chunk
  - 背景：ChatStream 读循环（llm/client.go:922-976）中 reader.Read() 拿到的原始 SSE 行在 parseSSELine 之前无任何日志输出；解析失败的行、[DONE]、心跳/空行被静默丢弃（continue）。llm-interaction 日志的 RESP][assistant / RESP][tool_calls 是按解析后字段重组的内容流，不是绝对原始 chunk。排查"工具参数被截断"（如 deepseek-v4-flash 在高上下文下于 `"replacements":` 处停笔，co-flow 08-11 单日 132 次 unexpected end of JSON input）时无法在日志层直接确认原始帧
  - 目标：log level=debug 时，在 ChatStream 读循环中、parseSSELine 之前逐行输出每条原始 SSE chunk（含 `data: ` 前缀与换行剥离后的完整行），覆盖正常 data 行 / [DONE] / 解析失败行；空行（SSE 事件分隔符）跳过避免刷屏。原则上输出所有 LLM 返回的原始内容
  - 实现：
    - llm/client.go：ChatStream 读循环在 parseSSELine(line) 之前加 `log.Debug("LLM ChatStream raw chunk: %s", string(line))`，空行跳过
    - llm/client.go：非流式 Chat 加 `log.Debug("LLM Chat raw response: %s", string(respBytes))`
    - llm/client.go：移除 legacy `log.Raw("%s", event.Content)` content 裸输出——raw chunk 已含完整 data 行（含 content），避免无前缀文本插入 raw chunk 行之间造成"凑在一起"观感（诊断：慢速 SSE 实测每帧到达 ~10ms 内逐行写入，co-shell 层实时；co-flow 真实日志粘连由 log.Raw 引起）
  - 编号说明：FEATURE/FIX 系列最小编号 24 与历史 ENHANCEMENT-24 冲突；217/268/282 与本地历史分支同名冲突，故顺延使用 FEATURE-347（符合 FEATURE-346 先例）
  - 测试：llm/feature347_test.go 12 用例（debug 逐字节输出/info 不输出/[DONE] 输出/空行跳过/解析失败行仍输出/混合帧保真/帧序/API key 不泄露/非流式响应/SetLevel 随动/无裸 content 粘连）+ use-case/FEATURE-347/FEATURE-347-UC-0001.md（13 用例）；go build/vet/test 全绿 [BUILD-393]
- [x] FIX-348 SSE 大帧被切分丢弃导致工具参数截断（unexpected end of JSON input / unbalanced closing brace）
  - 背景：co-flow 08-11 单日 132 次 `cannot parse tool arguments: unexpected end of JSON input`，原始报文截断在 `"replacements": `；work 端 08-11 全部 write_to_file 大 content 调用失败。初判为"arguments 内含真实换行导致帧跨行"，08-11 晚 wire 级日志（wireLogReader，收到即记录原始字节）实测修正：**vLLM（deepseek-v4-flash-0731）把整个工具参数攒成单个 6.5~7.7KB 巨帧一次性发出**（工具名 chunk 后静默 ~38s，然后 3ms 内整帧到达），帧内**没有**真实换行；真正的数据丢失点是 `StreamReader.Read()`（BUILD-32 起从未改）——`bytes.Buffer.ReadBytes('\n')` 在缓冲区无换行时把已读出的部分行随 io.EOF 一起返回，而代码丢弃了它继续读，导致 >4096 字节的帧除最后一个分片外全部静默丢失（幸存者从 UTF-8 字符中间开始，即日志中的 `??` 前缀；帧尾 `"}}]},"logprobs":null,...` 结构完整证明帧在网络上是完整的）
  - 目标：任意大小/任意分片的 SSE 帧完整重组，工具参数不截断；raw chunk 日志"收到即记录"不被 eventCh 发送阻塞拖累
  - 实现：
    - 方案A（根治数据丢失，已落地）：重写 `StreamReader.Read`——仅在缓冲区存在完整行（含 `\n`）时才返回，部分行保留在 buffer 中跨 read 累积，绝不丢弃；读缓冲 4096→32KB
    - 方案A 配套（保留）：ChatStream 读循环 pendingData 跨行帧拼接——`data:` 行解析失败（不完整 JSON）时暂存，把后续无 `data:` 前缀的续行按真实换行拼回原帧，直到 JSON 完整或遇空行/新 data 帧（覆盖"帧内真实换行"场景）
    - 方案A 扩展（裸行容错，已移除）：原先把裸行追加到当前工具调用 arguments 的兜底已删除——实测证明裸行并非上游行为，而是 StreamReader 丢帧头后的幸存尾巴；该兜底会把 SSE 帧尾垃圾喂给 FEATURE-235 流式解析器，报 `unbalanced closing brace/square bracket` 并中断流（work 端 20:08 后的报错形态）。裸行恢复为忽略
    - 方案B（日志实时加固）：eventCh 发送改非阻塞 `sendEvent()`（先补发 FIFO pending 队列，select 非阻塞发送，满则入队）+ 缓冲 100→1024，保证读循环不被消费者慢拖住
    - 方案B 扩展（interaction 日志与解析解耦）：读循环新增 `inToolCallStream` 状态（首个 tool_calls delta 置 true，finish_reason 置 false），在 tool_calls 流内**收到每个物理行**（data 帧/裸行/畸形/跨行片段）立即追加到 `RESP][tool_calls` interaction 日志（去 data 前缀），**与解析结果解耦**——不管 LLM 返回对错都实时记录；handleEvent 移除重复的 RESP][tool_calls 记录
    - 诊断（保留）：wireLogReader 包装 resp.Body，每次底层 Read 返回即以 DEBUG 输出 `LLM ChatStream wire read: N bytes: %q`，记录真实到达字节流（%q 转义使换行/截断的 UTF-8 清晰可见）
  - 测试：llm/fix348_test.go——跨行帧拼接、裸行忽略（原"裸行追加"用例改写）、单行回归、坏数据丢弃、慢消费者不丢事件/日志完整、裸行写 interaction 日志、**StreamReader 大帧分片重组（1/1000/4096/65536 字节分片各验证一遍，旧实现必挂）**、>4KB 单帧 arguments 端到端完整；go build/vet/test 全绿；use-case/FIX-348/FIX-348-UC-0001.md
- [x] FEATURE-349 循环二次判定记忆化（judge 回喂已失败策略 + 系统提示升级/抢救有效分析引导）
  - 背景：co-flow 08-12 全天 50 次 loop detected 全部干预无效——judge 判定**无记忆**，同输入必出同策略，第 1 次与第 50 次干预完全等价；且 temp 0 / 0.2 + repetition-penalty 1.05 均无法打破循环（实测），因为根因是"有效产出零沉淀（FEATURE-17 方案C 丢弃无工具调用轮次的 assistant 内容）+ 上下文恒定 → 确定性重演"，sampling 参数只扰动生成过程、不改变生成起点
  - 目标：同一任务的多次循环判定形成记忆链，judge 知道"上次这么劝没用"，强制实质性升级；引导 judge 把中止点前的有效分析浓缩进 exit_strategy（不写死结构化字段，由 judge 判断是否有必要）
  - 实现：
    - Agent 新增 `loopFailedStrategies`（agent/loop.go）：judge 确认循环时记录其 exit_strategy（`recordFailedLoopStrategyLocked`，连续重复合并、上限 3 条）；新用户任务开始 RunStream 时清空（.continue 模式保留链）
    - judge 用户提示新增 `{FAILED_STRATEGIES}` 占位符（buildLoopJudgeUserPrompt，经 buildProblemSolverPrompt 进入 problem-solver 判定）：编号列出已失败策略，首次判定显示"无"
    - judge 系统提示新增规则（zh/en 同步，KeyProblemSolverSystemPrompt 第 6/7/8 条）：① 禁止重复/换措辞已失败策略，必须实质性升级；失败 ≥2 条时认真考虑建议放弃当前路线（给阶段结论/向用户提问/attempt_completion 收尾）；② 疑似循环内容中"重复开始前"的分析如有价值，浓缩进 guidance 让主模型在其基础上继续（替代原 P1 结构化 PRESERVE_ANALYSIS 方案——按用户决策不写死，由 judge 自行判断）；③ **预防性禁用**（对 loop 类必做）：reason 中明确预测主LLM下一轮将复读的具体内容（引用重复样本），guidance 中显式禁用该模式——点名禁止意图陈述/分析性前缀，要求响应直接以工具调用开始（report_problem schema 的 reason 描述同步要求预测复读内容）
    - 判定结果收集统一为 report_problem 工具调用（用户决策，移除自由文本 JSON 路径）：judgeLoop 删除 classic JSON 兜底（judgeClient/JSON 截取解析/REQ-RESP][judgeLoop 段），problem solver 失败即返回 nil 走直接反馈兜底；callProblemSolver 按 tool-call mode 配置选择传输——OpenAI 模式 tools+tool_choice 强制调用，XML 模式系统提示内嵌 report_problem 用法（BuildToolUsagePrompt）并解析内容中的 XML 标签；删除 callProblemSolver 内"纯 JSON 内容"兜底解析；i18n 清理 KeyLoopJudgeSystemPrompt/KeyLoopJudgePrompt/KeyLoopJudgeResponse 及 judge 用户提示的"输出格式"段
    - 删除 FEATURE-241 异步判定死代码（loopJudgeInflight/loopJudgePendingResult/loopJudgeResultCh/loopJudgeTriggered 仅声明从未使用）及 run_stream.go 中 checkLoopJudgeResult 死注释
  - 测试：agent/feature349_test.go 9 用例（记录/空值跳过/连续重复合并/上限截断/none 文案/编号列表/提示词占位符替换/首次判定/双语模板占位符齐备）；go build/vet/test 全绿 [BUILD-396]

- [x] FEATURE-352 循环介入新增 auto 策略（纠错提示 + 达到阈值自动升级强制重整上下文）：[BUILD-402]
  - 背景：loop-intervention=prompt 的纠错提示在循环根因（上下文恒定 → 确定性重演，见 FEATURE-349）下多次无效；reorganize 策略又缺少"先尝试纠错、无效再重整"的渐进路径。auto 模式自动管理"纠错提示 → 强制重整"的升级链
  - 目标：auto 模式在同一反馈链（<retried_count> 信封计数）上先按 prompt 方式发送纠错提示；当链上确认循环次数达到 loop-auto-reorganize-threshold（默认 5）后，自动升级为强制 reorganize_context 指令
  - 实现：
    - config/config.go：LLMConfig 新增 LoopAutoReorganizeThreshold 配置项（json: loop_auto_reorganize_threshold，默认 5，<=0 回退默认）；DefaultConfig() 与 LoadFromFile 兜底；LoopIntervention 默认值 prompt → auto
    - agent/loop.go：applyLoopIntervention 新增 auto case（阈值前与 prompt 一致，阈值到达后反馈替换为 KeyLoopAutoReorganize 强制指令）；新增 loopAutoThreshold()（未设置/非法回退 5）与 autoEscalateToReorganize()（锁内从最后 user/tool 消息 env 读取 retried_count，count+1 >= 阈值即升级；重整后反馈链 collapse 自然重启计数）
    - agent/run_stream.go：流式同步循环分支新增 auto case（优先 loopJudgeExitStrategy，否则 LoopPromptTemplate 模板反馈，阈值到达时升级）；fallback 默认 prompt → auto
    - cmd/settings.go + cmd/settings_safety.go：:set loop-auto-reorganize-threshold 支持查询/设置（>=1 校验，同步 Agent 配置指针），:set 列表与 :set defaults 接入
    - main.go：--loop-intervention 合法值新增 auto（CLI help 同步）
    - i18n：keys/zh/en 新增 KeyStrategyAutoReorganize / KeyLoopAutoReorganize / KeyCol3LoopAutoReorgThresh / KeySettingCmd_773~775 双语；zh_loop.go / en_loop.go 同步
  - 测试：agent/loop_auto_test.go 5 个单测（阈值回退默认/升级边界/阈值下与 prompt 一致且计数+1/阈值到达升级且反馈消息原地更新不追加/无 user/tool 消息不 panic）；go build/vet/test 全绿 [BUILD-402]

---

## v0.7.5 — 视觉识别上下文隔离

> **状态**: ✅ 已完成（2026-08-10）
> **目标日期**: 2026-08-10
> **里程碑**: 视觉识别 minimal/full 上下文模式改造（识别轮独立调用 + 结果回填工具返回 + 系统上下文隔离）+ 问题判定异常场景接入

### 功能清单

- [x] FEATURE-343 视觉识别上下文隔离（minimal 识别轮独立调用 + 结果回填工具返回）：[BUILD-386]
  - 背景：当前 minimal 模式仅折叠历史消息为 [system, user(intent+图片)]，但识别轮输出以独立 assistant 消息留在主上下文历史中（语义割裂）；"文件加载成功"占位结果混入历史；OpenAI 模式识别轮仍携带全部 tools 参数
  - minimal 模式目标：
    - visual_analysis 工具调用记录正常进历史（含输入参数）；不生成"文件加载成功"占位结果进历史（仅 UI 显示）
    - 下一轮为独立识别轮：上下文折叠为 [system(仅 Identity 节), user(intent + 图片)]，不写主会话上下文
    - 识别轮 tools 清空（OpenAI 模式传空 tools；XML 模式 system prompt 精简为仅 Identity）
    - 识别结果回填为 visual_analysis 工具调用的返回（OpenAI: tool 消息 / XML: user 消息），不新增 assistant 消息
    - 识别轮失败时返回错误标记到工具结果，上下文保持合法
  - full 模式：保持现状（识别输入/输出正常进历史、带工具、visual_analysis 照旧写占位结果）
  - 涉及文件：agent/run_stream.go（识别轮检测/输出处理）、agent/image_tools.go（视觉分析工具分流）、agent/stream_response.go（识别轮 tools 清空）、agent/system_prompt.go（仅 Identity 辅助函数）
  - 测试：单元测试 + use-case/FEATURE-343/FEATURE-343-UC-*.md
  - 验收：`go build ./...`、`go vet ./agent/`、`go test ./agent/` 全绿
- [x] FIX-344 智能体设置组模型摘要改名 + 修复 :set default-* 的 %!d 显示 bug：[BUILD-387]
  - 背景：[智能体设置] 分组中 default-tool-model/default-problem-model/default-vision-model 是**只读显示项**（显示当前生效模型），与可配置项 default-tool-model 同名造成歧义；且 :set default-tool-model 查询时误用 KeySettingCmd_229（%d 模板）传入字符串 "auto"，输出 `%!d(string=auto)`
  - 修改：cmd/settings.go 只读摘要改名 current-tool-model/current-problem-model/current-vision-model；cmd/settings_safety.go default-tool-model/default-problem-model 查询/设置/解绑改用专用 KeyDefaultModelCurrent/KeyDefaultModelSet 模板；problem-solver-enabled 同源 %!d bug 一并修复
  - i18n：keys/zh/en 新增 KeyDefaultModelCurrent/KeyDefaultModelSet 双语
  - 验收：`go build ./...`、`go vet ./cmd/ ./i18n/` 全绿；编译产物 [BUILD-387]（work/co-shell）
- [x] FEATURE-345 异常场景接入问题判定（tool_format_error / context_overflow / llm_connection_error 走 report_problem）：[BUILD-389]
  - 背景：FEATURE-342 仅循环场景接入 report_problem（judgeLoop → callProblemSolver），三类异常仍走各自硬编码路径；config.go 承诺"疑似信号（工具格式错误/上下文溢出/连接异常）发送给问题模型"尚未兑现
  - 方案：agent/problem_solver.go 新增 solveProblem(ctx, hint, detail) 通用入口（复用 callProblemSolver 强制 report_problem 单工具）+ applyProblemAction(report) 动作分发（prompt_feedback/delete_last_msg/compact_context/notify_user/retry/continue）；agent/run_stream.go 三处接入（streamErr 连接错误、XML 工具格式错误、context 超限）
  - 降级保护：问题模型调用失败时回退到现有硬编码路径；HTTP 401/403/404/429/5xx 充分条件不调用问题模型；parse-error-action=exit 仍优先
  - 补充修复：:set default-problem-model/default-tool-model 支持 auto 值清空恢复自动计算（原先仅 none/-）
  - 测试：agent/problem_solver_test.go 新增 TestApplyProblemAction/TestSolveProblem_Gated/TestBuildProblemSolverUserPrompt；use-case/FEATURE-345/FEATURE-345-UC-0001.md（10 用例）
  - 验收：`go build ./...`、`go vet ./agent/ ./cmd/ ./i18n/`、`go test ./agent/ ./i18n/ ./cmd/` 全绿；编译产物 [BUILD-389]（work/co-shell）

---

## v0.7.4 — 问题判定优化

> **状态**: 🚧 开发中
> **目标日期**: 2026-08-08
> **里程碑**: 统一问题判定机制（report_problem 工具 + 问题模型优先级链 + 硬编码错误分类 + 降级保护）

### 功能清单

- [x] FEATURE-342 问题判定优化（统一问题判定机制）：[BUILD-385]
  - 验收：`go build ./...`、`go vet ./agent/ ./cmd/ ./i18n/`、`go test ./agent/ ./i18n/` 全绿；problem_solver_test.go 8 个单测通过；用例 FEATURE-342-UC-0001~0033 设计完成；编译产物 [BUILD-385]（work/co-shell）
  - 遗留：动作执行层（suggested_action 分发到 run_stream/loop 的 delete/compact/notify 路径）与降级保护降级链在 M3 之后继续实施（当前 judgeLoop 已接入报告路径，纯文本回退保留）
  - 问题模型优先级链：模式绑定的 ProblemModelID > 全局 default-problem-model > 全局 default-tool-model > 当前活跃模型
  - 新增全局配置 default-problem-model / default-tool-model（可手动设置，未设置时自动计算：ToolCall 第二高/最高优先），:set 从只读变为可配置
  - 新增 report_problem 工具（单工具强制 tool_choice）：类型枚举 [no_anomaly, loop, tool_format_error, context_overflow, llm_connection_error, unknown]，含 error_detail/reason/guidance/suggested_action
  - 新增 problem-solver-enabled 开关（默认开启）；循环路径受 problem-solver-enabled AND loop-judge-enabled 双门控
  - guidance 写作规范：自包含（不引用被删消息）、直接指示下一步、重申主目标、可执行性
  - 硬编码错误分类：HTTP 401/403/404/429/5xx 充分条件直接提示用户，非充分条件才调用问题模型
  - 降级保护：问题模型调用失败依次降级，降级后仍失败则停止并报告用户
  - 动作执行层：continue / prompt_feedback / delete_last_msg / compact_context / notify_user / retry
   - 单元测试 + i18n 中英文翻译

---

## v0.7.3 — :context 显示增强

> **状态**: 🚧 开发中
> **目标日期**: 2026-08-07
> **里程碑**: :context 命令显示增强（tool_calls 块 + 保留控制字符 + retried_count + full 模式）

### 功能清单

- [x] FEATURE-335 :context 显示增强：[BUILD-377]
  - 新增 tool_calls 块：assistant 消息含 `ToolCalls` 时，在同一消息下标下方输出 `[tool_calls]` 子块（6 空格缩进，与正文内容同列），每个 ToolCall 显示工具名 + 格式化缩进的参数 JSON（解析失败则原样输出）；与 `<message_no>` 下标严格对齐，tool_calls 不占独立序号
  - 保留控制字符：移除 `showContext` 中 `strings.ReplaceAll(content, "\n", " ")` 拍平逻辑，各消息内容按原始格式多行输出、6 空格缩进（正文在 `[role]` 左括号左侧 2 格）、块间空行分隔
  - 消息间长分割线：每个 message 块之间输出 `──────────────────────────────` 长分割线，便于视觉分隔
  - retried_count 显示：消息头最右侧显示 `♾️N`（从消息最后一个 ContentPart 的 `<environment_details>` 中解析 `<retried_count>`，无则省略）；新增 agent 公开 helper `RetriedCountOf`/`MessageEnv` 供 cmd 包解析
  - full 模式：`:context full` 显示所有消息的完整 `<environment_details>`；`:context`（默认）隐藏 env 块；两个命令独立实现，不新增全局配置参数
  - 新增 `cmd/context_show_test.go` 单测 10 个（含消息分割线）+ `cmd/context_rebuild_test.go` 调用适配（showContext 增加 full 参数）
  - 验收：`go build ./...`、`go test ./cmd/ ./agent/`、`go vet ./cmd/ ./agent/` 全绿；用例 FEATURE-335-UC-0001~0008 通过；编译产物 [BUILD-377]（work/co-shell）
- [x] FEATURE-336 工具调用解析错误显示原报文：[BUILD-378]
  - 新增 show-parse-error-raw 开关控制：开启后工具调用解析失败时在用户界面显示原始报文，便于诊断
  - 按反馈将 show-parse-error-raw 移到显示与输出分组
  - 验收：`go build ./...` 全绿；用例 FEATURE-336-UC-0001 通过；编译产物 [BUILD-378]（work/co-shell）
- [x] FIX-337 修复 search_files 结果匹配数错误显示 0 处：[BUILD-379]
  - 根因：`searchFilesTool` 在 walk 回调中边匹配边输出——`writeHeader()` 在文件内容前被调用，此时 `matchCount` 尚未累加本文件，首个（或唯一）匹配文件的 header 显示"找到 0 处匹配"但下方内容正常输出；且多文件场景下简单提前累加也只能显示首个文件计数，非总数
  - 修复：文件内容块（文件头 + 上下文行 + 分隔空行）统一暂存到 `filesOut` builder；walk 结束后 `matchCount` 为完整总数时才写 header（复用原 `writeHeader()` 延迟兜底），再追加 `filesOut` 内容到 result。同时顺带修复 `truncatedLineCount` 同类顺序问题（首个文件超长行也能计入 header 截断提示）
  - 测试：`TestSearchFilesTool` 开头固定 `i18n.SetLang("zh")`；既有用例 "search all files for hello" 增加断言 `找到 3 处`；新增 "search single file with multiple matches"（断言 `找到 2 处` + `file1.go:1-4:`）、"search single file with single match"（断言 `找到 1 处`）；"search with no matches" 增加断言 `未找到匹配`
  - 验收：红→绿确认（stash 实现后 3 个新断言全部复现 "找到 0 处" 失败，恢复后全部通过）；`go build ./...`、`go vet ./agent/`、`go test ./agent/` 全绿；用例 FIX-337-UC-0001~0006 通过；编译产物 [BUILD-379]（work/co-shell）
- [x] FEATURE-338 write_to_file 流式渲染行号右对齐优化：[BUILD-381]（迭代改进：3 位 → 5 位固定右对齐）
  - write_to_file 内容流式渲染时，行号前缀以固定 5 位占位右对齐（`%5d`），保证各行 `+` 号在同一位置垂直对齐
  - 场景：1 位数行号前补 4 空格、2 位数补 3 空格、3 位数补 2 空格、4 位数补 1 空格、5 位数不补；同一次渲染内所有行号宽度统一
  - 影响函数：`agent/toolcall_renderop.go` 的 `linePrefix()`（write_to_file 分支 indent="     "、marker="+"、colon=false）
  - 不改变 replace_in_file 的 `5-: ` / `5+: ` 格式（冒号模式无对齐需求）
  - 测试：新增 `agent/toolcall_stream_test.go` 6 个单测用例（覆盖 1~9 行 / 10~99 行 / 9→10 边界 / 100~999 行 / replace 冒号模式不受影响 / 门控回归）
  - 顺带修复 3 个既有测试（TestParseXMLToolCalls_* 语言顺序依赖缺陷，main 分支同样失败）
  - 验收：`go build ./...`、`go vet ./agent/`、`go test ./agent/ ./repl/ ./i18n/` 全绿；渲染 golden 回归不变；用例 FEATURE-338-UC-0001~0009 通过
- [x] FEATURE-339 :context 标题行体验优化：[BUILD-382]
  - 去掉标题行重复的 📋 表情符号（cmd/context.go:96 硬编码 "📋 " 前缀 + KeyContextTitleLine 值含 📋 → 输出 "📋 📋 当前上下文"）
  - 总消息数与标题合并到同一行，用括号分隔（如 `📋 当前上下文（总消息数: 28）`），不再独占一行
  - 影响：cmd/context.go、i18n/zh.go、i18n/en.go（KeyContextTotalCount 改为无换行的括号格式）、cmd/context_show_test.go 断言同步
  - 验收：`go build ./...`、`go test ./cmd/` 全绿；用例 FEATURE-339-UC-0001 通过
- [x] FEATURE-340 :context full 在 openai 模式显示工具调用方法声明：[BUILD-383]
  - 需求：`:context full` 在 openai 工具调用模式下，除消息记录和 `<environment_details>` 外，还需显示 LLM 请求中 `tools` 参数的工具函数声明（每个工具的 name、description、parameters JSON Schema）
  - 方案：agent 包新增公开方法 `ToolDeclarations() []llm.Tool`（复用 `buildToolsInternal()`，带 `mcpMgr == nil` guard，仅 openai 模式返回非空）；cmd/context.go `showContext(full)` 在 full 且 openai 模式下于消息末尾追加 `[tool declarations]` 区块（工具名 + Description + parameters JSON，沿用现有 6 空格缩进风格）；i18n 三文件新增标题 key（KeyContextToolDeclarations zh/en 双语）
  - 测试：cmd/context_show_test.go 新增 4 个测试 + 1 个 openai helper（openai full 显示 工具名/Description/parameters JSON、非 full 不显示、xml 不显示、mcpMgr nil 不 panic）
  - 验收：`go build ./...`、`go vet ./cmd/ ./agent/`、`go test ./cmd/ ./agent/ ./i18n/` 全绿；audit Hardcoded Chinese=0 / i18n keys missing=0；用例 FEATURE-340-UC-0001~0005 通过；编译产物 [BUILD-383]（work/co-shell）
- [x] FIX-341 show-parse-error-raw 未覆盖 OpenAI 模式 JSON 解析错误路径：[BUILD-384]
  - 根因：`show-parse-error-raw` 开关（FEATURE-336）的原始报文显示逻辑只加在 `run_stream.go` 的 XML 解析错误分支（taskInstructionCache 的 raw 字段，第 644/720 行），未覆盖 OpenAI（非 XML）模式下工具参数 JSON 解析失败分支——`executeToolCall` 返回 `cannot parse tool arguments: %w` 时走 `run_stream.go` 第 1074 行 FIX-314/317 分支，只输出 `KeyToolExecRetry` 摘要（`tc.Name` + `execErr`），未打印原始参数 `tc.Arguments`，导致开关开了也不显示原始报文
  - 修复：提取 `emitParseErrorRaw(cb, rawDetail)` 辅助方法（`ShowParseErrorRaw && rawDetail != ""` 时输出 `KeyXMLParseErrorRaw`），五处调用点统一复用——XML 分支 exit action、XML 分支 retry/prompt、OpenAI JSON 解析错误分支（`cannot parse tool arguments`，`rawDetail = tc.Arguments`）、工具执行失败汇聚点（非 XML 模式，覆盖 missing required / 坏路径 / SEARCH 不匹配等非语法错误）、parse-error-action=exit 分支
  - 新增 `agent/fix341_test.go` 单测 4 个（开关开启输出原始报文/开关关闭不输出/raw 空串不输出/nil cfg 不 panic）
  - 验收：`go build ./...`、`go vet ./agent/`、`go test ./agent/ ./repl/ ./i18n/` 全绿；用例 FIX-341-UC-0001~0006 通过；编译产物 [BUILD-384]（work/co-shell）

---

## v0.7.2 — 工作区配置外部化

> **状态**: 🚧 开发中
> **目标日期**: 2026-08-06
> **里程碑**: 系统提示词外部化（PRINCIPLES.md 覆盖 + .rules/ 目录规则 + .rule 生效修复）

### 功能清单

- [x] FEATURE-330 工作区配置外部化：系统提示词支持 PRINCIPLES.md 与 .rules/ 目录 [BUILD-375]
  - PRINCIPLES.md：存在时替代 config.json 的 `llm.agent_principles`（查找顺序 workspacePath → cwd），每次重建系统提示词时读取，修改后立即生效
  - `.rules/*.md`：按文件名排序合并到 RULES 节 `{CUSTOM_RULES}`，每个文件以 `====\n标题(去.md后缀)\n\n内容` 格式标明规则领域，追加在 config rules 之后；去掉 `{CUSTOM_RULES}` 前的 `# 自定义规则` 标题
   - 修复 `.rule` 命令修改不生效：`a.rules` 快照改为每次从 `cfg.Rules` 现场重建（`strings.Join`）；随后按反馈移除 REPL `:rule` 命令及相关子命令（cmd/rule.go 删除），规则改由 `.rules/` 目录管理
   - 新增 `--unload-principles` 命令行参数：将当前系统解析后的 principles 导出到工作区根目录 PRINCIPLES.md 并退出（文件已存在则跳过）；`--init-capabilities`/`--init-rules` 改名 `--unload-capabilities`/`--unload-rules`（旧名保留为弃用别名）
   - `:set` 实时体现外部文件状态：新增 agent 公开方法 `ResolveAgentPrinciples`/`ResolveRules`/`ExternalFile`/`RulesDirFiles`，showSettingsHelp 方法化注入 capabilities/rules 状态行
   - `:context` 实时反映 PRINCIPLES.md 修改：新增 `agent.RebuildSystemPrompt()` 公开方法，showContext 开头触发重建
   - 新增 `agent/rules_test.go`、`cmd/settings_external_test.go`、`cmd/context_rebuild_test.go` 单元测试
   - 验收：`go build ./...`、`go test ./...`、`go vet ./...` 全绿；用例 FEATURE-330-UC-0001~0025 通过；编译产物 [BUILD-375]（work/co-shell）

---

## v0.7.1 — 循环检测优化

> **状态**: 🚧 开发中
> **目标日期**: 2026-08-05
> **里程碑**: 循环检测误判修复 + judge 提示词升级 + 报警类型区分

### 功能清单

- [x] FIX-329 循环检测误判修复与循环报警类型区分：[BUILD-370]
  - 修复行匹配误判：LoopDetector 逐行比较时不再 TrimSpace，保留行首尾缩进（只去 `\r`），修复 `  }` 与 `}` trim 后视为同一行导致 threshold=2 即误报的问题
  - 单行重复数量块限制：新增 `loop-single-line-block-limit` 配置（默认 200），p=1 单行重复需同时满足 `单行字符数 × 重复次数 > 限制` 才判定循环，短行（如 `}` × 2 = 2 字符）不再误报；接入 config.go / DefaultConfig / `:set` / `:set defaults` / `:config` 向导
  - judge 提示词升级：`KeyLoopJudgeUserPrompt` 新增 `{USER_PROMPTS}`（所有真实用户输入，过滤 XML 工具结果/continue prompt/循环反馈消息）与 `{ITERATION_TOOLS}`（迭代工具调用序列），使判模对"是否有进展"有更全面的判断
  - judge 提示词终极目标锚定：系统提示词新增第 7 条"先锚定终极目标"（提炼主目标/最终交付物作为执行锚点，防止偏离主线）与第 8 条"再明确阶段目标与优先级顺序"（按优先级列执行步骤且每步可溯源到终极目标），输出格式要求同步；新增样例 6（多层级目标先锚定终极目标）
  - 循环报警类型区分：`LoopDetectedError` 增加类型标识，`handleLoopDetection` 提示区分 6 种触发场景——单行重复/多行周期/单行超长/字符周期/长输出/工具调用重复；zh/en 双语
- [x] FIX-330 系统命令交互卡死修复与 OpenAI 模式工具示例清理
  - 根因：enhanced（raw terminal）模式下 `handleAgentInput` 启动时 `EnhancedIO.startRaw()` 将终端置于 raw mode（关闭 ECHO/ICRNL/ICANON/ISIG）且整个 RunStream 持续保持，需要用户输入的系统命令（sudo/passwd 等）通过 `cmd.Stdin=os.Stdin` 读输入时无回显、回车产生 `\r` 而非 `\n`、Ctrl+C 失效，表现为卡死；FIX-209 仅解决 ESC monitor 竞争未解决 raw mode 本身
  - `agent/loop.go` 新增 `CommandHooks`（BeforeCommand/AfterCommand）+ `SetCommandHooks`/`onCommandStart`/`onCommandEnd`（锁内取函数指针、锁外调用，允许钩子内安全回调 agent）
  - `agent/command_tools.go`：`executeSystemCommand` 与 `ExecuteCommandDirectly` 命令启动前 `onCommandStart()`（恢复 cooked mode）、`cmd.Wait()` 后 `onCommandEnd()`（重进 raw mode），含 `cmd.Start()` 失败分支对称释放
  - `repl/repl.go`：enhanced 模式 `startRaw()` 成功后注册钩子（Before→`eio.stopRaw()`/After→`eio.startRaw()`），`RunStream` 返回后先清除钩子再 `stopRaw()`；stdio 模式不注册（终端本在 cooked mode）
  - i18n：`KeySystemPromptToolUsageExamples` zh/en 均置空字符串（`i18n.T` 对"key 存在值为空"返回空串，合法"无内容"值），OpenAI 模式 ToolExamples 节输出为空，减少上下文占用
  - 用例：use-case/FIX-330/FIX-330-UC-0001.md（UC-0001~0006）
  - 验收：`go build ./...`、`go test ./agent/ ./repl/ ./i18n/` 全绿；编译三平台产物（cc 规则参数 `3`）
- [x] FIX-331 XML 工具调用流式解析跨 chunk 边界前缀破坏修复
  - 根因：`XMLToolCallParser.Feed` `xmlStateOutside` 分支处理 `<` 时，若该 `<` 是 chunk 末尾字符或标签前缀片段（`<`、`<c`、`<cs`、`<cs:`、`</`、`</c`…）落在 chunk 边界，前缀完整匹配检查失败，`<` 被当普通文本输出且不缓存到 partial；下一 chunk 的 `cs:read_file>` 因不以 `<` 开头而无法识别为标签，整个工具调用 XML 结构破坏——表现为工具名粘在普通文本末、参数键值交叉错乱（`end_line:`/`intent:` 无值、`,` `:` 残留、`start_line1` 粘连）、`[⚙️ ]< read_file` 残渣泄漏到普通内容
  - 修复：`xmlStateOutside` 在 `<` 认定为字面文本前增加两个缓存判断——开标签前缀部分匹配 `strings.HasPrefix(p.prefix, content[i+1:])`（含空串）与闭标签 `content[i+1]=='/' && strings.HasPrefix(p.prefix, content[i+2:])`，匹配时整段缓存到 `p.partial` 待下一 chunk 续扫；普通非前缀 `<`（`a < b`、`</div>`）保持字面路径不受影响
  - 用例：use-case/FIX-331/FIX-331-UC-0001.md（UC-0001~0003）；新增 TestToolCallStream_XMLChunkBoundaryLoneLessThan / _PartialPrefix 单测
  - 验收：`go test ./agent/ -run TestToolCallStream` 13 用例全绿（含 2 新 + 11 既有）；`go build ./...`、`go vet ./agent/...`、`go test ./agent/ ./repl/ ./i18n/` 全绿
- [x] FIX-332 i18n 转义多字节字符改为直接书写
  - 背景：i18n/en.go 与 i18n/zh.go 各 381 处（共 762）以 `\uXXXX` 转义书写中文/符号（如 `\u5f53\u524d\u503c` = "当前值"），可读性差
  - 新增 bin/decode_u_escapes.py：按 Go 词法逐字符解析，仅转换双引号字符串内的单反斜杠 `\uXXXX` → UTF-8 字符；保护反引号 raw string、行/块注释、`\\u` 字面量、其他转义（\n\t\" 等）不受影响
  - 效果：en.go/zh.go `\uXXXX` 转义清零；运行时字符串值不变（Go 规范保证双引号内 `\uXXXX` 与直接 UTF-8 字节等价）
  - 用例：use-case/FIX-332/FIX-332-UC-0001.md（UC-0001~0004）
  - 验收：`go build ./...`、`go vet ./i18n/...`、`go test ./i18n/` 全绿；两文件 `\uXXXX` 数量为 0
- [x] FIX-333 英文资源中文英文化
  - 背景：i18n/en.go 英文资源大量条目误存中文（复制自 zh.go），KeyCmdMig_* 381 个 key 值在 en/zh 完全一致均为中文；另有 KeySettingCmd_613/614/615、KeyNoActiveTaskPlan、KeyNoRecentIterations 5 处含中文样例/全角符号
  - 新增 bin/translate_en_cmdkeys.py：内置 381 条 KeyCmdMig_* 中→英映射，按行正则替换（保留 %s/%d/\n 等转义与前导缩进），执行 gofmt 规范缩进
  - 手工修正 5 处：KeySettingCmd_613 图片识别示例英文化、614 '微软雅黑'→'Microsoft YaHei'、615 word_continue content 示例英文化、KeyNoActiveTaskPlan/KeyNoRecentIterations 去全角双语改纯英文
  - 效果：en.go 资源值中 `[\u4e00-\u9fff]` 数量为 0；zh.go 未改动
  - 用例：use-case/FIX-333/FIX-333-UC-0001.md（UC-0001~0004）
  - 验收：`go build ./...`、`go vet ./i18n/...`、`go test ./i18n/` 全绿；en.go 无中文残留
- [x] FIX-334 修复 KeyCmdMig_* 中文资源双重转义换行
  - 根因：zh.go 的 `KeyCmdMig_*` 资源值使用 `\\n`（源码双反斜杠+n），Go 双引号解码后得到**字面 `\n` 两字符**而非换行；`:model switch` 等中文界面输出 `(优先级: 20)\n  [2]` 显示字面 `\n`。en.go 在 FIX-333 翻译时已是单转义（正确），中文界面暴露 zh.go 缺陷
  - 修复：将 zh.go 中 `KeyCmdMig_*` 行的 `\\n` 还原为 `\n`（真实换行），共 143 行；`KeySettingCmd_615` 教学示例中的 `\\n` 是展示给 LLM 的字面 JSON 参数格式，有意保留
  - 用例：use-case/FIX-334/FIX-334-UC-0001.md（UC-0001~0003）
  - 验收：`go build ./...`、`go test ./i18n/` 全绿；zh.go `KeyCmdMig_*` 行无 `\\n` 残留

---

## v0.7.0 — 输出架构重构

> **状态**: 🚧 开发中
> **目标日期**: 2026-08-01
> **里程碑**: 输出统一化（Out/RenderCommand + InputSource + 100% i18n + stdio/tui/web）
> **架构文档**: docs/output-architecture.md

### 功能清单

- [x] **FEATURE-235 工具调用流式识别与实时渲染（P1）**：LLM 流式输出过程中（XML 与 OpenAI 两种模式）实时解析工具调用串。XML 模式通过 FSA 逐字符解析 content 中的 `<cs:tool>` 标签；OpenAI 模式通过流式 JSON Tokenizer 逐字符解析 arguments 增量（llm/client.go 增加 StreamEventToolCallDelta 实时通道，并在 llm-interaction 日志记录 [RESP][tool_calls 逐 chunk 到达粒度）。两套解析器各自独立错误检测，输出同构 RenderOp 渲染事件流，由统一 ToolCallRenderer 渲染（新增 EventToolCallStream 事件）。渲染规则：方法名首行由 show-tool 门控，动态参数/内容/diff 由 show-tool-input 门控完整展开；content 通道常驻 XMLToolCallParser（不依赖 tool-call-mode，普通文本产出 OpPlainText 归还普通输出，`<cs:` 转入工具渲染并靠 PendingToolCall 防止整段工具调用回落原文）；解析错误立即 streamCancel 中止，走现有 parse-error-action。循环检测（FIX-179/LoopLongOutputThreshold/loopDetectSyncErr）上移到 StreamEventContent 分支最前，确保工具渲染 continue 不会绕过。新增 toolcall_renderop.go / toolcall_parser_xml.go / toolcall_parser_json.go / toolcall_stream_test.go（9 个单测），修改 llm/client.go / events.go / stream_response.go / stream_renderer.go / main.go（BUILD 364→365）[BUILD-365]

- [x] **FEATURE-236 输出前导提示表情符号后统一加空格（P1）**：`[⚙️]` → `[⚙️ ]`。调研发现运行时前缀实际硬编码于 config.GetEmojiPrefixes() 而非 i18n 资源，故同步修改：1) i18n/zh.go + i18n/en.go 的 KeyEmojiPrefix* 系列 12 项 zh/en 双语；2) config/config.go GetEmojiPrefixes() emoji enabled 分支全部 13 项（含 VisionUserInput/Loop，禁用分支纯文本标签未动）；3) 同步更新 repl/render_tui.golden 与 testdata/render_single_cmd.golden 前缀基线；4) 顺带修复 pre-existing TestRenderSingleCmdGolden 失配——单命令模式 token 渲染死代码引用已废弃空串 KeyTokenUsageTiming 产生 `%!(EXTRA...)` 乱码行，删除后重新生成 golden。go build/vet/test 全量通过（10 包全绿）[BUILD-368]

- [x] **FEATURE-301 输出/输入事件双枚举重构（P1）**：[BUILD-340]
  - 新增 agent/events.go（13 输出事件常量）+ agent/input.go（12 InputKind 常量）
  - agent/loop.go / run_stream.go / stream_response.go 的 63 处 cb 魔法字符串替换为常量；repl.go/main.go switch 同步用 agent.Event 常量
  - 回归基线：agent/events_test.go（UC-0003/0004）+ repl/render_test.go（render_tui.golden）+ render_test.go（render_single_cmd.golden）逐字节一致
  - 验收：go build；audit 魔法事件 63→0（其余 4 项 206/1555/19/1 不变）；行为零变化

- [x] **FEATURE-302 Out + RenderCommand 抽象 + 渲染合并（P2）**：[BUILD-341]
  - 新增 agent/out.go（Out 接口 + ChannelID/Level + TerminalOut）+ agent/command.go（RenderCommand/RenderKind）+ agent/stream_renderer.go（单一事件渲染管线）
  - repl.go streamCallback 与 main.go renderSingleCmdEvent 均委托 StreamRenderer（StreamModeREPL/SingleCmd），消除双渲染实现
  - config.NormalizeInputMode：--input-mode enhanced 兼容别名 → tui；config.json 旧值加载归一化为 tui 不回写；REPL readLine/handleAgentInput 支持 tui
  - 回归验证：P1 两份 golden 逐字节一致（渲染零变化）；agent/out_test.go 新增 UC-0003/0004/0008/0009（ChannelID/Level/RenderKind/TerminalOut/parseInputMode table 测试）
  - audit：fmt 206→204（渲染合并消除 2 处直接 fmt，合规改进），魔法事件=0 / 中文=1555 / 同步输入=19 / i18n=1 与 P1 一致

- [x] **FEATURE-303 向导迁移（B 类）+ i18n 归零第一步（P3）** [BUILD-342]：
  - UI 组件 Out.Box/Menu/Step/Sep（绑定规范快捷键 [B]/[C]/[E]/[D]/[Q]/[数字]）
  - 迁移 cmd/model.go → mode.go → config.go → settings_db.go → session.go
  - i18n：B 类硬编码中文迁移（约 1/3）
  - 验收：向导回归；audit Hardcoded Chinese 降 1/3

- [x] **FEATURE-304 外部入口迁移 + 分类开关（P4）** [BUILD-343]：
  - 新增分类开关 OutputCategories（:set output-categories / --output-categories CLI / config.json 持久化）
  - feishu/bridge/subagent/feishu-bridge 运行时输出迁移（D 类 i18n/英文化 + ChannelSubAgent 开关控制）
  - i18n：A 类 47+16 key 迁移（run_stream ESC/循环策略/错误段 + loop.go 全部）zh/en 双存在
  - 验收：audit 901→772（-129）；D 类可开关控制；go vet 0；build 绿

- [x] **FEATURE-305 i18n 归零冲刺（P4.5）**：
  - 按 inventory A-3 清单逐条迁移；补齐 keys.go + zh/en 翻译
  - 修复审计发现的 KeyToolUsageShellSend 缺 zh 翻译 bug
  - 第 3~6 批完成：settings_tools/tools/vault/excel/docx/browser/file/taskplan/plan/model_template + repl vet
  - audit 420→133（清零 287 处）；i18n keys missing 0；go test ./... 全绿
  - 验收：audit 第 3 项 Hardcoded Chinese = 0 [BUILD-371]
  - 追加批次（P4.5 续）：修复 audit 脚本口径（统一 build_file_list 排除 work/hub/work/ 与行尾注释剥离、豁免 docx/html + bridge/executor + xlsx/styles 注释）；迁移 XML 解析/流式校验（toolcall_mode 13 + xml_stream_validator 4）、agent 工具（image/memory/run/taskplan/shell/agent/session/loop/stream_renderer/system_prompt）、cmd 系列（context/image/memory/section/reset/settings/tool/plan）、config（provider/config）、llm、memory、scheduler、shell、store/pgstore、repl/vault、co-shell-hub；新增 100+ i18n key 并 zh/en 双存在
  - 完成状态：audit 第 3 项 Hardcoded Chinese 123 → **0**（100%），i18n keys missing 0，go build ./... 通过 [BUILD-380]

- [ ] **FEATURE-306 输入统一 InputSource（P2.5，v0.7.4）**：
  - InputSource 接口 + StdioSource/RawKeySource；单一 Reader goroutine
  - ESC 监控改事件流消费者；Windows 补齐（repl_esc_windows.go）
  - 验收：方向键/ESC/Ctrl+C 双平台通过；stdio 管道行为不变

- [x] **FEATURE-307 渲染器三态 + web 原型（P5，v0.7.5）**：
  - SessionIO 管道 + sessionFactories（stdio/tui/web）
  - LineRenderer + StreamRenderer(JSON-Lines) + WebRenderer（HTTP+WebSocket，绑 127.0.0.1）
  - 验收：三模式同指令结果一致；web 浏览器分区实况
  - [x] **307a 事件语义化（StreamEvent + LineRenderer）**：[BUILD-413]
    - 背景：agent 事件流是 `(eventType string, content string)` 半渲染文本——emoji 前缀、装饰换行、token 统计键值串全混在载荷里（81 处 `cb(...)` 调用点内嵌 `ep.` 装饰，token 事件靠 `fmt.Sscanf` 反解析），无法直接作为 JSON-Lines / WebRenderer 的事件协议
    - 目标：迁移为结构化 `StreamEvent{Type, Level, Chan, Text, Meta}`（agent/events.go），Text 纯语义（无 emoji/分隔线/装饰换行），token 统计入 Meta；emoji 前缀与换行布局全部下沉到渲染器；`StreamRenderer` 更名 `LineRenderer`（stream_renderer.go → line_renderer.go），StreamRenderer 之名留给 307b 的 JSON-Lines 渲染器
    - 实现：`StreamCallback` 签名改 `func(ev StreamEvent)`；81 处生产点按 8 类模式迁移（纯文本 Info / 内嵌错误/警告/成功装饰 → ErrEvent/WarnEvent/OKEvent / loop 前缀 → LevelDebug / 分隔线保留 / token → TokenIterEvent/TokenTaskEvent / 内容工具流带 Channel）；6 个 i18n key（KeyLLMErrorRetry/KeyLLMErrorFixRetry/KeyContextOverLimit/KeyToolExecRetry/KeyOutputResume/KeyOutputRetryFailed）去 emoji `%s` 槽位（zh/en 双语）；消费者 repl.streamCallback 与 main.renderSingleCmdEvent 改接 `LineRenderer.Render(ev)`
    - 有意的呈现归一（仅 4 类）：ESC 重试失败的取消消息补齐前置换行；EventToolCall 载荷尾部 `\n` 与渲染器补 `\n` 的双空行归一为单空行（4 处）；上下文超限警告双 ⚠️ 修复为单 ⚠️（原 EventWarning 类型前缀与模板内嵌 emoji 叠加）；i18n 模板尾部 `\n` 移除改由渲染器统一补
    - 测试：新增 agent/line_renderer_test.go 锁定 Level 布局表 / 双 StreamMode 差异 / token Meta 渲染；两份 golden（render_tui / render_single_cmd）不带 -update 通过；audit 三项不退化（magic events=0、Hardcoded Chinese=2 持平、fmt=143 持平）；`cb(` 调用点 grep 无 `ep.` 参数；零依赖变更（go.mod/go.sum 无 diff）
    - 验收：`go build ./...`、`go vet ./...`、`go test ./...` 全绿；编译产物 [BUILD-413]（仓库根 co-shell）
  - [x] **307b SessionIO 管道 + REPL 解耦 + JSON-Lines**：[BUILD-415]
    - 背景：REPL 主循环按 inputMode 分支读写（tui 建 InputReader / stdio 建 StdioSource），每次 Agent 运行的 IO 装配（UserIO 安装、ESC 消费者、CommandHooks、LineRenderer 构造）散落在 handleAgentInput 的模式 switch 里；事件流只能渲染为终端行，无法被脚本/CI 机器消费
    - 目标：SessionIO 接口 + stdio/tui 两实现收编输入读取与每次运行的 IO/渲染器装配；REPL 主循环面向 SessionIO 编程；新增 `--output-format text|json`（CLI-only）使事件流可以 JSON-Lines 输出；tui 模式行为与 BUILD-414 逐字节一致（纯结构移动）
    - 实现：agent 包新增 `EventRenderer` 接口（events.go）、`Level.String()`（out.go）与 `StreamRenderer` JSON-Lines 渲染器（stream_renderer.go，字段规则：type 必有、level 非 info 才出、chan/text/meta 非空才出，无 ts/ANSI/emoji）；新文件 `repl/session.go` 定义 `SessionIO`（ReadLine/Acquire/Interactive/Close）+ `sessionFactories{stdio,tui}`，ESC 消费者与 CommandHooks 逐字搬入 tuiSession.Acquire/release（repl_esc.go 改为 tuiSession 方法）；repl.go 的 reader/stdioSrc/userIO 字段收敛为 `session`+`renderer`，主循环改 `session.ReadLine`，handleAgentInput 改 `Acquire/release`；json 模式抑制 welcome/prompt/Said/syncDB 装饰（stdout 只承载 JSON 事件行，运行期错误走 stderr）；main.go 新增 `--output-format` flag 与互斥校验（json + 显式 `--input-mode tui` 报错退出，未指定 input-mode 时 json 隐含 stdio），单指令模式按 format 选择渲染器；i18n 新增 KeyCLIHelpOutputFormat/KeyOutputFormatInvalid（zh/en 双存在）
    - 测试：agent/stream_renderer_test.go（14 类事件 JSON 行逐字段断言、meta 序列化、level/chan 省略规则、无装饰）；agent/out_test.go Level.String() 表测试；repl/session_test.go（工厂配对、tui 回退 stdio、Acquire/release 配对含 ESC 订阅计数）；output_format_e2e_test.go 用 fake llm.Client 端到端一致性（同一 agent 同一输入跑两遍，RunStream 结果一致、JSON 每行可解析且 type 序列与 text 模式一致、content_chunk 拼接一致）；两份既有 golden 不带 -update 通过；audit 143/0/2/18/0 持平（SessionIO 边界调用加入 check 4 豁免，注释注明理由）
    - 验收：`go build ./...`、`go vet ./...`、`go test ./...` 全绿；`bash bin/output_audit.sh --strict` 与基线持平；`git diff go.mod go.sum` 为空；`printf ':exit\n' | ./co-shell --input-mode stdio --output-format json` stdout 无装饰混入；编译产物 [BUILD-415]（仓库根 co-shell）
    - 范围说明：`serve` 子命令（web 界面）整体移至 307c——用户决策以 `co-shell serve` 替代原计划的 `--input-mode web`；builtin 处理器、printHelp、cmd/config.go 旁路 scanner、bridge/feishu、UserIO 阻塞交互本阶段不动
  - [x] **307c `co-shell serve` + Web 界面（P5 收官）**：[BUILD-417]
    - 背景：307b 落地后 SessionIO 管道只剩 web 形态未接；事件流已结构化（307a）、REPL 主循环已面向 SessionIO（307b），web 模式只差「一个 SessionIO 实现 + 内嵌 HTTP/WS 服务 + 页面」。依赖政策禁止新增第三方库，gorilla/websocket 不可用
    - 目标：`co-shell serve [--port 8399]` 启动单会话 web 服务（绑死 127.0.0.1、无认证、端口占用自动递增至多 10 个），自动打开默认浏览器；页面含事件流分区（LLM/工具/命令/系统）、任务进展区（track_task_progress 实时更新）、工作区目录树、拖拽上传、OS 打开/定位文件、ask 应答区、打断按钮；明/暗双主题（暗色默认跟随 prefers-color-scheme，localStorage 记忆）
    - 实现：新包 `web/`——`ws.go` 标准库手写 WebSocket（SHA1+base64 握手、帧编解码、客户端掩码、continuation 重组、ping→pong、close、帧/消息 4MiB 上限）；`server.go` HTTP 路由（embed 静态页 + `/ws` + `/api/tree|upload|open|reveal|file|bootstrap`）与单客户端集线器（新连接顶替旧连接，断开时挂起 ask 全部失败返回）；`session.go` WebSession（repl.SessionIO 实现：input→ReadLine、interrupt→ag.Interrupt、attachments→ag.SetImagePaths）+ WebIO（agent.UserIO：Print*→ui_text 事件，ReadLine/ReadKey→ask/answer 阻塞闭环，无客户端即报错避免永久阻塞）+ WebRenderer（事件原样推送）；`open.go` OS 打开浏览器/文件/定位（runtime.GOOS 分发，launcher 可注入）。repl 包导出 `RegisterSessionFactory`（避免 repl↔web 循环依赖），SessionDeps 新增 `Ag *agent.Agent`（仅 web 使用）；agent 新增 `EventTaskPlan` 事件（Meta["plan"] 全量计划 JSON，归档清空推空串），发射点在 run_stream.go 工具回显分支后（track_task_progress 成功即推，不受 showTool 门控）；LineRenderer 外层 switch 无 case 自动静默，TUI 零影响。main.go 在 flag.Parse 前拦截 `os.Args[1]=="serve"`，新增 `--port`，serve 与 `--input-mode`/`--command`/位置参数/`--output-format` 互斥（报错 exit 1）；serve 路径复用 main 的 config/store/agent 初始化，仅末端注册 web 工厂 + `SetInputMode("web")` + `r.Run()`。前端三件套原生 JS 无构建（embed.FS 内嵌）：CSS 变量双主题、单强调色、细边框、等宽内容区；输入历史（内存数组+上下键）、连接指示灯、图片预览走 /api/file。所有文件路径 API 强制 workspace 内校验（filepath.Rel 防 ../ 穿越）；上传单文件上限 100MB；目录树排除 .git/node_modules/db/log/tmp、保留 output/
    - 测试：`web/ws_test.go`（RFC 6455 握手已知向量、掩码/16 位/64 位长度帧往返、continuation 重组、ping→pong、close、超上限拒绝，stdlib 手写最小客户端）；`web/server_test.go`（tree 排除清单、upload 落盘+../ 拒绝、open/reveal 注入假 launcher 校验路径、file 读取边界）；`web/session_test.go`（内存 WS 对：input→ReadLine、Print→ui_text、ask/answer 闭环、ReadKey 单键、interrupt→InterruptChan、attachments→ImagePaths、WebRenderer 透传、非交互断言）；`agent/taskplan_event_test.go`（构造器形状、LineRenderer 双 StreamMode 零输出断言、track_task_progress 成功发射全量计划、归档推空快照、其他工具不发射、快照无装饰）；两份既有 golden 不带 -update 通过；audit 与基线 143/0/2/18/0 持平；`git diff go.mod go.sum` 为空
    - i18n：新增 7 个 key（zh/en 双存在）——KeyCLIHelpPort/KeyServeStarted/KeyServeNoPort/KeyServeConflict/KeyServeBrowserFailed/KeyWebOpenFailed/KeyWebRevealFailed
    - 冒烟：临时 workspace 预置 dummy config（disclaimer_accepted=true + 死地址模型），`./co-shell -w <tmp> serve --port 18399` 后台启动，curl 验证 `/` 返回 HTML、`/api/tree` 返回 JSON 且排除清单生效、`/api/file?path=../` 与 `/api/upload?dir=../` 返回 403；WS 握手由 web/ws_test.go 覆盖
    - 验收：`go build ./...`、`go vet ./...`、`go test ./...` 全绿；audit 持平；依赖零变更；浏览器人工验证（发指令→流式输出、ask 闭环、打断、任务面板实时更新）留给用户；编译产物 [BUILD-417]（仓库根 co-shell）
    - 范围说明：builtin 命令结果文本仍走 fmt 到 serve 终端（REPL 层 rawPrint/fmt.Println 不在 UserIO 通路上），agent 交互与流式输出全部在浏览器可见；cmd/config.go 旁路 scanner 在 web 模式不可用（:config 请用 :set 替代）；open/reveal 依赖 127.0.0.1 回环 + workspace 路径白名单两道防线，远程访问前必须补认证（见 docs/output-architecture.md 6.6）

- [ ] **FEATURE-308 全屏 TUI v2（tui v2，v0.7.6 可选分支）**：
  - FullScreenRenderer：原生 ANSI 缓冲，禁用 tview/tcell；SIGWINCH 重绘
  - 验收：独立分支交付，不影响主线

- [x] **FIX-309 修复 stripCodeBlockXML 闭合行处理缺陷**：[BUILD-344]
  - 根因1：`stripCodeBlockXML()` 删除代码块闭合 ``` 时使用 `IndexByte('\n')` 定位行尾，当闭合 ``` 与 `</cs:参数>` 同一行时（如 `` ```</cs:replace>`` ），会连带删除 `</cs:replace>` 闭合标签，导致 `parseXMLChildrenToJSON` 报告"参数缺少闭合标签"
  - 根因2：`stripCodeBlockXML()` 全局剥离所有代码块，当代码块是工具参数真实内容（如 `<cs:replace>` 中的 bash 脚本）时被误删
  - 修复：`stripCodeBlockXML()` 改为跟踪 `cs:` 前缀标签嵌套——标签外的示例代码块剥离（FIX-291 行为），标签内的真实参数代码块逐字保留（FIX-309）；闭合 ``` 只跳过反引号本身，保留同行后续内容
  - 新增回归测试：`TestStripCodeBlockXML_ClosingFenceWithCloseTagSameLine`（单元）+ `TestParseXMLToolCallsWithTools_CodeBlockCloseTagSameLine`（集成）
  - 验收：全部 stripCodeBlockXML 相关测试通过；`go test ./agent/`、`go vet ./agent/`、`go build ./...` 全绿；用例 FIX-309-UC-0001 通过

- [x] **FEATURE-310 工具调用意图+个性化摘要显示**：[BUILD-345]
  - 新增 `agent/tool_summary.go`：`buildToolSummary(toolName, args)` 统一构建工具调用摘要（友好工具名 + intent + 关键参数，长内容自动截断）
  - i18n 新增 19 个 `KeyToolCallSummary*` key（zh/en 双存在），为 execute_command/read_file/write_to_file/replace_in_file/search_files/list_files/list_code_definition_names/shell_send/visual_analysis/excel_open/word_open/update_settings/ask_followup_question/launch_sub_agent 配置个性化话术
  - `agent/run_stream.go`：showTool 开启时，工具执行前显示摘要替代原始工具名/JSON
  - `agent/tools.go`：确认提示的 `displayStr` 改为摘要（含 intent），便于用户确认前掌握调用影响
  - 新增 `agent/tool_summary_test.go`：7 个测试函数覆盖 fallback/文本工具/长内容截断/shell/文档/零参数/i18n key 双存在
  - 验收：`go build ./...`、`go test ./...`（含渲染 golden 回归）、`go vet ./agent/...` 全绿；用例 FEATURE-310-UC-0001 通过

- [x] **FIX-311 show-loop-detection 默认值修正为 false（P1）**：config.go DefaultConfig() 中 ShowLoopDetection 实际为 true 与字段注释 `Default: false` 矛盾。将 DefaultConfig() 默认值改为 false，与字段注释及 FEATURE-241 设计意图（默认不显示完整判定详情，避免刷屏）一致。只改 config/config.go 一处，go build + go vet + agent 测试通过 [BUILD-363]

- [x] **FIX-312 OpenAI 模式系统提示词出现 "==== system_prompt_tool_usage"（P1）**：i18n.lookup() 改为返回 (string,bool) 区分「空值翻译」与「缺失 key」，T() 对空值返回 ""（合法「无内容」语义）；修正 zh/en_system.go 中 KeySystemPromptToolUsage 值从含换行 raw string 改为真正空字符串 ""；新增 i18n 单元测试 [BUILD-346]

- [x] **FIX-313 OpenAI 模式 ToolExamples 节误用 XML 格式示例（P1）**：buildNamedSection/getRawSectionText 的 ToolExamples case 按 cfg.LLM.ToolCallMode 分支——OpenAI 用 KeySystemPromptToolUsageExamples（JSON 格式），其他用 XML 格式；补全 zh/en 示例 intent 必填字段；孤儿 key 恢复使用 [BUILD-347]

- [x] **FIX-314 OpenAI 模式 parse-error-action=retry 时方法调用及错误结果不应进入上下文（P1）**：executeToolCall 失败（JSON 解析错误/缺参数）后回滚 assistant(tool_calls) 消息并 continue iterationLoop 干净重发；新增 KeyToolExecRetry i18n key（zh/en）向用户显示 UI 错误提示（不进 LLM 上下文）[BUILD-348]

- [x] **FIX-315 :rule 添加的自定义规则未进入系统提示词（P1）**：KeySystemPromptRules 翻译补 {CUSTOM_RULES} 占位符（zh/en），使 :rule 规则经 buildSectionWithPlaceholders 注入；英文 Rules 对齐中文 4 条平铺规则（补 XML 转义规则，去 MUST/Recommendations 分层）[BUILD-349]

- [x] **FIX-316 工具执行后必显输出（P1）**：attempt_completion 显示 ✅ 任务完成+result 全量、track_task_progress 显示完整计划、read/write/replace/search/exec 显示动作概要（含实际命令+intent/行数/替换数/匹配数）；统一走 StreamRenderer EventToolCall 通道，受 showTool 控制，带工具图标引导；trackTaskProgressTool 去裸 Println [BUILD-350]

- [x] **FIX-317 OpenAI 模式 replace_in_file 执行失败死循环（P1）**：FIX-314 retry 静默丢弃错误使 LLM 无法修正。现按错误分流：`cannot parse tool arguments`（JSON 语法错）→保留静默重发自纠；path 不存在/SEARCH 不匹配/缺参等其他错误→回灌 formatToolError 结构化反馈（ERROR DETAILS+CORRECTION INSTRUCTIONS）让 LLM 修正 [BUILD-351]

- [x] **FIX-318 reorganize_context 触发 OpenAI 400（P1）**：工具执行阶段清空 a.messages 导致 tool 结果消息成为孤儿，违反"tool 必须跟在带 tool_calls 的 assistant 之后"。修复：reorganizeContextTool 仅置 reorganizeContextUsed 标志不再清空历史；run_stream.go / run.go 在工具结果全部追加后调用 collapseAfterReorganize() 折叠为 [system, user(summary)]；新增单元测试覆盖 OpenAI/XML/非流式/无 user/标志未置五场景 [BUILD-352]

- [x] **FEATURE-319 视觉模型上下文控制（P2）**：主模型上下文限制大于视觉模型时，发送给视觉模型的内容可只含系统提示词+识别指令，避免上下文超限。新增 vision-context-mode 参数（minimal/full，默认 minimal），minimal 模式 buildContextMessages 折叠为 [system, user(intent 识别指令+图片)]；visual_analysis 的 intent 参数作为识别指令；支持 config.json/--vision-context-mode CLI/:set vision-context-mode 三通道，默认值重置、i18n 帮助 [BUILD-353]

- [x] **FIX-320 execute_command 超时 goroutine 误杀后台任务（P1）**：executeSystemCommand 中命令提前结束时超时 goroutine 仍残留，到点无条件 kill(-pid)，可能误杀仍留在原进程组的后台子进程（如 `sleep 300 &`）；且 REPL 路径 ExecuteCommandDirectly 未设置进程组，管道孙进程超时后泄漏。修复：LLM 路径引入 done channel 在 Wait 返回后通知超时 goroutine 放弃 kill + Process.Release 释放 PID；REPL 路径弃用 CommandContext 统一为 exec.Command + setProcessGroupAttr + 手动超时 goroutine。新增 command_tools_test.go 5 个单元测试（后台任务不被误杀/超时正常触发/管道孙进程连锅端/无超时自然运行），use-case/FIX-320 6 个运行时用例；运行时验证 UC-0001（后台 sleep 300 超时窗口后仍存活）与 UC-0003（管道孙进程超时连锅端）通过 [BUILD-354]

- [x] **FIX-321 循环反馈消息补全 environment_details + retry_count 重试计数机制（P1）**：循环干预（sync 循环分支与 applyLoopIntervention）产生的 user 消息仅含纯文本 feedback，缺少其他 user 消息都具备的 `<environment_details>` 块。修复：envelope.go 新增 applyLoopFeedback 统一入口 + tag helpers（loop_feedback/retry_count 读写）；feedback 非空（prompt/reorganize）创建带完整 env 的 feedback 消息、已存在时原地替换文本递增计数；feedback 为空（retry/temperature）不新增消息，仅在最后一个 user 消息 env 递增 retry_count（无 env 先补完整 env）；计数只存在于消息上下文中，不设 Agent 状态字段、无归零。新增 loop_retry_test.go 11 个单测覆盖三种状态×两种处理方式判定矩阵、temperature 一致性、无 env 消息、完整 env 校验、多轮循环链 [BUILD-355]

- [x] **FIX-322 循环二次判定 exit_strategy 质量升级（P1）**：judgeLoop 仅凭 SUSPECT_CONTENT 判定，无法感知 cwd/可用工具/文件线索，导致给 LLM 的下一步指令空泛。修复：A) 系统提示词加入 exit_strategy 编写六要求——动作性动词开头/明确作用对象/可落地下一步/信息不足时指明提问/禁止空泛话术/场景分类指导，扩展 5 类样例、is_loop=true 必须非空约束；B) user 模板新增 {CONTEXT} 块，buildJudgeContext 填充 cwd/workspace/最近文件路径/可用工具列表（含 mcpMgr nil 保护）；D) 服务端空值兜底——is_loop=true 且 exit_strategy 为空时回退 KeyLoopJudgeFallback 可操作指令。新增 loop_judge_test.go 3 个单测（context 构建/user 模板填充/兜底 key），zh/en 双语同步 [BUILD-356]

- [x] **FEATURE-323 工具执行结果回显优化（P1）**：普通工具执行成功后，事件流中连续输出"执行前摘要"与"执行后结果回执"两行（`[⚙️]< 读取文件...` 与 `[⚙️]< 读取 xx 文件...`），内容重复。修复：run_stream.go 工具执行循环中，普通工具执行成功时不再 emit FIX-316 的 buildToolOutcome 回执（与执行前摘要重复）；仅当 execErr != nil（执行失败）时 emit EventError 显示错误原因（与回灌 LLM 的结构化 result 一致）；attempt_completion/track_task_progress/view_task_plan 三类工具仍显示完整 result。go vet + agent 全量测试通过 [BUILD-357]

- [x] **FEATURE-324 工具返回结果标记优化（P1）**：工具为空 result 时，返回消息中 `[意图] xxx` 成为唯一内容，易被 LLM 误认为结果是意图文本。修复：intent 拼接改为三段式——`[返回结果][实际结果或空]\n\n[意图] xxx`，明确区分"返回结果"与"意图"；[返回结果]/[意图] 标签 i18n key 化（zh/en 双语），OpenAI 与 XML 模式共用同一条 intent 拼接（均走 executeToolCall 返回）。go vet + agent/i18n 测试通过 [BUILD-358]

- [x] **FEATURE-325 工具显示去重（P1）**：流式实时预览 `[🔧 工具名]`（stream_response.go，showTool 控制）与执行前摘要 `[⚙️]< 工具名摘要`（run_stream.go，showTool 控制）都由 showTool 控制且内容重复。修复：删除 stream_response.go 中 StreamEventToolCall 分支的实时预览块（`[🔧 工具名]`），仅保留执行前摘要，用户看到每个工具只显示一次 `[⚙️]< ...`。go vet + agent 全量测试通过 [BUILD-359]

- [x] **FEATURE-326 默认参数调整（P1）**：show-tool-input/show-tool-output/show-command/show-command-output 四项显示开关默认值统一改为 false（off）；toolcall-mode 默认值从 xml 改为 openai。修复位置 config/config.go DefaultConfig() [BUILD-360]

- [x] **FEATURE-327 循环重试次数熔断提示（P1）**：复用上下文中 `<retried_count>` 标签（原 retry_count，改名后语义为"已完成重试次数"）与 error-max-single-count 参数。envelope.go 新增 checkRetryCountLimit：循环介入递增 retried_count 后若达到阈值则提示用户处理（回车继续并重置 retried_count=1 / C 取消 / A 忽略全部抑制后续提示）；定位从"最后 user 消息"扩展为"最后 user 或 tool 消息"，OpenAI 模式下计数挂在被删消息前一条 tool 上体现重试起点；feedback 文本仍只追加为 user 消息不覆盖 tool 结果。三处接入：run_stream.go sync 循环分支、loop.go applyLoopIntervention、run_stream.go 工具调用循环（原忽略返回值改为检查）。不改变现有 errorCounter 机制。新增 loop_retry_limit_test.go 9 个单测 [BUILD-362]

- [x] **FEATURE-328 文件写入/修改流式渲染用户体验优化（P1）**：write_to_file 内容按行号增量显示（git diff 风格 `N+ 内容`），replace_in_file search/replace 逐行实时显示（search 行 `-`、replace 行 `+`），replace 块指定 start_line 时显示真实行号、未指定则不显示；OpenAI 模式 JSON arguments 值片段增加转义解码（`\n`→换行等）使换行渲染与 XML 模式一致；ToolCallRenderer 重构为行式状态机，JSON 解析器仅影响渲染值不影响原始累积；新增 toolcall_stream_test.go 单测 11 个（含 FEATURE-328 UC-0001~0006）[BUILD-367]

---

## v0.6.0 — Beta3

> **状态**: 🚧 开发中
> **目标日期**: 2026-06-01
> **里程碑**: 功能完善，稳定可用

### 功能清单

- [x] **FEATURE-269 思考模式三态控制+Provider专用适配器+测试按Provider禁用思考** [BUILD-283]
  - ThinkingEnabled 从 bool 改为 string 三态（on/off/default），支持模型级覆盖
  - reasoning-effort 扩展 max/none/default 选项
  - 7 个 Provider 思考适配器（DeepSeek/Qwen/OpenAI/Xiaomi/Zhipu/Mimo/fallback）
  - 所有 LLM client 创建路径（main.go/startup、cmd/settings.go/rebuild、agent/agent.go/ApplyWorkModeConfig）均通过适配器注入 BodyAdditions
  - 模型测试时按 Provider 正确禁用/启用思考格式（Qwen→extra_body, DeepSeek→thinking:{type})
  - 模型向导修复 ID 重复导致失败的问题
  - i18n 循环检测内容拆分到独立文件 zh_loop.go/en_loop.go，清理重复 key
  - 交互日志修复：测试时强制开启、.set llm-log 同步到 log 模块
  - `.set` 界面显示 thinking-enabled 和 reasoning-effort 及完整选项说明

- [x] FIX-271 恢复被 FEATURE-270 覆盖的 FEATURE-267/268 修改 [BUILD-286]
  - 恢复 agent/loop.go 中 getFirstUserCommand()/getRecentIterations() 等循环判定提示构建
  - 恢复 agent/run_stream.go 中 LoopIntervention/LoopPromptTemplate 循环介入策略
  - 恢复 config/config.go 中 LoopIntervention/LoopPromptTemplate 字段
  - 恢复 cmd/settings.go 的二维数组分组显示（allGroups [][]settingLine）
  - 修复 TokenUsageDisplay 模板被覆盖导致显示乱码的问题

- [x] FEATURE-208 外部工具（bin/）梳理与优化：[BUILD-213]
  - [x] 新增 pdf2png 工具：将 PDF 拆分为分页 PNG 图片，支持 LLM 多模态 PDF 内容解析
  - [x] 新增 docx2pdf 工具：将 .docx 转换为 PDF，WPS 优先（Linux wps2pdf / macOS AppleScript / LibreOffice 兜底）
  - [x] 新增 doc2pdf 工具：将老式 .doc 转换为 PDF，WPS 优先，LibreOffice 兜底
  - [x] 新增 wps2pdf 工具：将 .wps（WPS Office Writer）转换为 PDF
  - [x] 为每个外部工具创建同名 .md 参数说明文件
  - [x] 系统提示词中新增 ExternalTools 节（中英文），指导 LLM 调用 bin/ 工具
- [x] FEATURE-192 持续交互 Shell 环境：新增 shell/session.go 包，使用 PTY 维护长期运行的 shell 进程，支持命令发送和输出捕获。提供 shell_start / shell_exec / shell_stop / shell_get_output 四个 LLM 工具，让 LLM 在同一个 shell 进程中连续执行命令（如 cd 保持路径、Python REPL 等），支持超时控制。新增 shell-session-enabled / shell-session-timeout 配置项。[BUILD-196]
- [x] ENHANCEMENT-193 Shell Session 机制优化：shell_send 替代 shell_exec，纯 idle timeout 观察（不再追加 \n 和无 marker），新增 unescapeCommand() 支持控制字符（\n、\x03 等），stripLogANSI 日志和 LLM 输出控制字符剥离，0 工具调用时需 attempt_completion 才退出，XML 解析已知工具优先检查避免 HTML 标签误报，parseXMLChildrenToJSON 不再 trim 保留全部空格，defaultToolModes() 默认工具确认模式配置，.set confirm-tool reset 恢复出厂设置。[BUILD-197]
- [x] FEATURE-194 虚拟终端（Virtual Terminal）功能：为 Shell Session 增加纯 Go 实现的虚拟终端，支持 ANSI 控制序列解析和字符网格渲染，让 LLM 能像人一样查看终端画面。[BUILD-198]
  - 新增 `shell/vt.go`：虚拟终端核心实现，无外部依赖，支持光标移动（CUP/HVP/CHA/CUU/CUD/CUF/CUB）、清屏清行（ED/EL）、字符输入（CR/LF/BS/TAB）、屏幕滚动、SGR 解析
  - 新增 VT 窗口尺寸配置：`shell-vt-rows`（默认 24）、`shell-vt-cols`（默认 80），支持通过 `.set` REPL、命令行参数、配置文件设置
  - `shell_send` 改为返回完整的 VT 窗口文本（rows 行），而非增量文本
  - 新增 `shell_window_content` 工具，返回 VT 窗口当前内容
  - 完善 shell session 相关方法的使用说明和用例
  - `shell-session-enabled` 为 on 时自动屏蔽 `execute_command`；为 off 时自动屏蔽所有 shell session 工具
- [x] FEATURE-195 系统提示词外部化：所有系统提示词节（Identity/ToolUsage/ResultMode/Capabilities/Rules/Environment/Objective 及 XML 模式的 Examples/TaskProgress/EditingFiles）均支持通过外部 Markdown 文件覆盖，文件放在 workspace 根目录，启动时优先读取外部文件，不存在时回退到 i18n 内置资源。[BUILD-199]
- [x] FEATURE-196 工作模式配置系统：
  - 新增 PromptSection 和 WorkMode 数据结构，config.json 持久化
  - 新增 `.section` 命令管理自定义节（add/list/remove）
  - 新增 `.mode` 命令管理工作模式（list/create/edit/switch）
  - 新增 `--mode` 命令行参数和 `.set mode` REPL 命令切换
  - 内置默认工作模式，含所有内置节的默认拼接顺序
  - 修改 `buildSystemPromptWithMode` 按当前工作模式拼装
  - 交互式编辑：+/-上下移动、a/d添加删除、v查看内容、p预览完整提示词 [BUILD-199]
- [x] FEATURE-197 REPL 配置向导模式：新增 .config 内置命令，提供逐级菜单式配置向导，第一层按 .set 命令的配置分类显示，P 返回上一步，Q 完全退出 [BUILD-200]
- [x] FEATURE-198 shell-session-enabled=on 时 REPL 直接命令通过 VT 会话执行：当 shell-session-enabled 为开时，用户输入的 shell 命令（非以 . 开头的输入内容）直接发送到 VT 去执行，并返回执行结果（使用 shell_get_output，获得完整的输出）[BUILD-202]
- [x] FIX-199 修复 .config 设置 shell-session-enabled=on 后未自动初始化 VT session 及 VT 中文显示问题：shell-session-enabled SetValue 缺少 agent.SetShellEnabled() 同步；VT 逐字节处理 UTF-8 导致中文文件名乱码；VT \n 空 lineBuf 时 flushLine 导致 ls 多空行 [BUILD-203]
- [x] FIX-204 修复 execute_command / ExecuteCommandDirectly 执行交互式命令（如 sudo）时 stdin 未连接导致用户无法输入的问题 [BUILD-209]
- [x] FEATURE-205 UserIO 规范合规整改：检查全代码库中所有直接使用 fmt.Print/fmt.Printf/fmt.Println/os.Stdin/bufio.Scanner 进行用户交互的地方，统一替换为 agent.UserIO 接口方法。整改完成：agent/io.go 新增 DefaultUserIO + GetIO + DefaultIO + ErrPrintf；cmd/config.go、cmd/mode.go、cmd/model.go、cmd/settings_db.go、main.go 全部改用 io 方法；删除 cmd/model.go 废弃的 bufio.Scanner 字段。同时修正 i18n/zh_system.go 中 BROWSER USAGE 示例错误（browser_xxx 是工具调用而非系统命令）。[BUILD-210]
- [x] FIX-206 补全浏览器工具 XML 模式逐方法调用说明：在 i18n/keys.go 中新增 11 个浏览器工具键，在 i18n/zh_system.go 和 i18n/en_system.go 中为每个浏览器工具添加完整的 Description/Parameters/Usage XML 说明，在 agent/toolcall_mode.go 的 toolUsageKeyMap 中建立 browser_xxx 到 i18n 键的映射，确保 XML 模式下 LLM 能获取到每个浏览器工具的完整调用格式。[BUILD-211]
- [x] FEATURE-200 CDP 浏览器支持：新增 browser/ 包，通过 Chrome DevTools Protocol (CDP) 直接启动并控制 Chrome 浏览器，提供一组浏览器操作 LLM 工具（browser_navigate、browser_screenshot、browser_click、browser_type、browser_evaluate、browser_get_html、browser_scroll、browser_get_interactive_elements、browser_go_back、browser_go_forward、browser_close），配合截图视觉分析实现 "观察→评估→操作" 的 SREA 闭环。新增 browser-enabled / browser-port / browser-headless 配置项，截图自动注入到多模态上下文供 VLM 分析。[BUILD-211]
- [x] FEATURE-223 browser_get_html 更名为 browser_get_rendered_html：明确名称和文档，强调该工具返回的是经过所有 JS 渲染后的 DOM HTML（来自 Chrome 实时 DOM 树），而非原始静态源码，LLM 无需再单独下载 JS/JSON 等资源。同步更新所有 i18n 系统提示词中的工具描述、SREA 步骤和工具表格。HTML 无论大小始终保存到本地文件以确保数据完整性。新增页面数据收集方式对比章节（截图/交互元素/渲染后 DOM HTML 三种方式优劣分析）。[BUILD-227]
- [x] FIX-224 修复 .simulate 缺少父标签闭合时错误提示混淆：XML 解析器 findAnyCloseTag 回退逻辑会盲目使用不匹配的子标签闭合标签来闭合父标签，导致 LLM 看到"参数缺少闭合标签"的错误提示而非"父标签缺少闭合标签"。修复为检测名称不匹配时直接报清晰错误，错误消息末尾附带正确的方法调用格式示例。[BUILD-228]
- [x] FIX-225 修复当前会话历史记录未出现在上下键导航中的问题：REPL 的 `saveHistory()` 只将输入持久化到数据库，但从未更新内存中的 `r.history` 切片，导致每次 `readLine()` 创建新 `EnhancedInput` 时传入的都是启动时加载的旧历史。修复为在数据库写入后同步更新 `r.history` 和 `r.historyPos`。[BUILD-229]
- [x] FIX-226 修复上下键导航时残留旧行字符的问题：`clearLine()` 使用 `\033[J`（从光标清除到屏幕末尾）只能清除光标之后的字符，当从长命令切换到短命令时，旧行尾部字符残留在光标之前。修复为 `\r\033[2K`（先回车到行首，再擦除整行），确保整行被完全清除。[BUILD-230]
- [x] FEATURE-207 系统提示词规则增强：新增五条系统提示词规则——1) 获取 Web 页面优先使用浏览器工具（ToolUsage 节）；2) curl/wget 下载先保存到本地再用 read_file（ToolUsage + RULES 节）；3) 阶段性任务完成时推荐移动上下文指针（RULES 节）；4) 修改程序优先 replace_in_file 分多次而非重写整个文件（RULES 节）；5) 研究报告用 Markdown 整理后转 Word 并呈现给用户（RULES 节）。五条规则均与现有规则互补不冲突。[BUILD-212]
- [x] FEATURE-202 表达式计算器工具：新增 `evaluate_expression` LLM 工具，提供表达式计算能力。使用递归下降解析器实现，支持四则运算（+、-、*、/、%）、指数运算（^）、三角函数（sin、cos、tan、asin、acos、atan）、对数（log、ln）、开方（sqrt）、绝对值（abs）、取整（ceil、floor、round）以及常数 pi、e。工具接收表达式字符串，解析计算后返回精确数值结果，让 LLM 在进行数学计算时无需依赖 Python 或外部命令。[BUILD-207]
- [x] FEATURE-201 ESC 中断 LLM 输出功能：用户在增强输入模式下按 ESC 键可中断 LLM 流式输出。系统先暂停接收 LLM 返回数据，提示用户确认取消或继续；若确认取消则丢弃不完整消息并返回命令提示符；若选择继续则重新尝试接收 LLM 返回数据，失败时同取消处理。[BUILD-206]
- [x] FEATURE-243 attempt_completion 新增 task_message_no 参数、系统提示词"管理上下文窗口"节、工作模式描述区系统提示词接入、EN 版缺失节标题补齐、attempt_completion task_message_no 改为必需字段、track_task_progress 空标题清空所有计划记录 [BUILD-253]
- [x] FEATURE-248 XML 模式 User Message ContentParts 结构化：将 XML 模式下 user 消息从纯文本字符串改为 ContentParts 数组结构，使用户指令、工具返回结果、`<environment_details>` 分开为独立片段。每次构建上下文时不再剥离旧的 `<environment_details>`，各部分以数组形式保留在上下文历史中。OpenAI 模式保持原有逻辑不变。[BUILD-259]
- [ ] FEATURE-93 日历与待办事项管理：提供日历功能，支持记录和管理待办事项（todo）。提供 .calendar 内置命令（add/list/remove/update）管理待办事项；提供 add_todo / list_todos / update_todo / remove_todo 四个 LLM 工具，让大模型能操作待办事项；数据持久化到 bbolt。如果系统有日历应用（如 macOS 日历），提供选项帮助用户将待办事项同步到系统日历。
- [x] FEATURE-203 .config 配置向导增强：1) 补齐 browser-enabled/browser-port/browser-headless 参数到"智能体设置"分组；2) 快捷键改进：B 退回上一步、Q 完全退出，每一步都显示快捷键提示；3) 显示格式改进：所有选项先显示说明再显示当前值，.Xxxx 命令显示"name..."表示进入子配置；4) 选择编号按最长数字右对齐；5) 去掉发送LLM上下文时的序号前缀；6) 运行失败的命令也加入history。[BUILD-208]
- [x] FIX-212 修复浏览器 CDP 功能异常：[BUILD-218]
  - CDP 调用超时保护（ensureTimeoutContext，默认30s超时兜底），防止 context.Background() 无 deadline 导致永久阻塞
  - WaitForPageLoad 轮询 document.readyState=="complete"，解决 Page.navigate 异步返回后页面未加载完成的问题
  - 过滤 Chrome 扩展背景页面（chrome-extension://），优先 type=="page" 的真实标签页；全为扩展时自动新建标签页，解决导航到扩展页而非目标 URL 的问题
  - browserCloseTool 不设 browserEnabled=false，解决第二次调用浏览器工具时 "tool not found" 的问题
  - browserCloseTool 不杀 Chrome 进程，仅断开 WebSocket；下一个工具调用时 EnsurePageConnected 自动创建新标签页重连，不会产生多余的空白窗口
  - 重启时检测并复用已有 Chrome 实例（IsEndpointAvailable+SetStarted），避免重复启动浏览器
  - 浏览器数据目录持久化到 {workspace}/browser-data/，替代 /tmp 临时目录，浏览器状态（Cookie/Session/下载）跨重启保留，可追溯
  - 自闭合标签（<tag />）解析为工具调用：checkSelfClosing 支持 <tag/> 和 <tag /> 两种格式
  - HTML 内容导致 parseXMLChildrenToJSON 递归解析失败时回退纯文本，解决 write_to_file 含 HTML/CSS 内容时解析退出
  - createNewPage 增加 HTTP 状态码检查，返回非 JSON 响应时提供完整错误体诊断
- [x] FIX-209 修复 ESC monitor 与子进程争夺 stdin（sudo 密码输入被拦截）、streamCallback 换行（\r→\n）、命令输出重复、.config 参数不立即生效等问题：Agent 新增 commandRunning 标志 + SetCommandRunning/IsCommandRunning 方法；rawOutputWriter 实时输出 \n→\r\n 转换 + [🔴]> 前缀；syncedOnOffParam 辅助函数使 .config 设置即时同步到 agent；confirm-tool 默认改为 custom 模式。[BUILD-215]
- [x] FIX-210 工作空间默认路径智能检测：双击启动时自动使用可执行文件所在目录作为默认工作空间，终端启动时使用当前工作目录。新增 workspace/detect_common.go / detect_darwin.go / detect_linux.go / detect_windows.go 实现跨平台启动方式检测，main() 在 workspace 初始化后自动 os.Chdir 到工作空间根目录。
- [x] FIX-211 修复 .set description 无法保存生效的问题：i18n SystemPromptIdentity 节中增加 {AGENT_DESCRIPTION} 占位符，新增 KeyAgentDefaultDescription 默认描述键，Agent 构建系统提示词时从 cfg.LLM.AgentDescription 读取并替换占位符。
- [x] FEATURE-215 数据库双写策略重构（memory+history 同时写入 PG 和本地 bbolt，其他数据仅本地 bbolt）：[BUILD-217]
  - [x] 创建 DualStore 包装器，memory 和 history 操作同时写入 bbolt 和 PG，其他操作仅写入 bbolt
  - [x] PGStore 简化为仅 memory 和 history 两张表，移除 context/schedules/taskplans/token_usage/sessions 表
  - [x] 启动时如 PG 可用，自动增量迁移 bbolt 中未同步的 memory 和 history 数据
  - [x] .db 命令显示当前数据库连接状态
  - [x] .db init/migrate/backup/restore 子命令仅处理 memory+history 表
- [x] FIX-216 HasUnfinished 检查不再将 cancelled 视为未完成步骤：\`HasUnfinished()\` 中 `StatusCancelled` 与 `StatusCompleted` 一样被视为"已完成"，允许在所有步骤均为已完成或已取消时创建新计划。[BUILD-220]
- [x] FEATURE-218 模拟 LLM 方法调用命令 `.simulate`：新增 `.simulate` 内置命令，接收模拟的 LLM 返回内容（XML 或 JSON 格式），使用与正常 LLM 调用完全一致的管道（ParseXMLToolCalls / executeToolCall）进行解析和执行测试。结果不加入对话上下文，仅用于调试和测试。同步加入 `.config` 开发者工具分组。[BUILD-221]
- [x] FIX-219 XML 模式 attempt_completion 退出逻辑修复：\`attemptCompAvailable\` 在 XML 模式下因 \`buildTools()\` 返回空列表而被误判为 false，导致 0 toolCall 时直接退出而非要求 LLM 调用 \`attempt_completion\`。修复为改用 \`buildToolsInternal()\` 判断。同步增强 continuePrompt 文本，更明确要求 LLM 必须调用 attempt_completion 并深思熟虑。[BUILD-222]
- [x] FEATURE-220 PostgreSQL 数据库连接超时配置：DBConfig 新增 Timeout 字段（默认 3 秒），DSN 追加 connect_timeout 参数，net.DialTimeout TCP 预检 + goroutine Ping 超时控制。支持 `.set db timeout <秒>` 子命令、`.db` 显示超时值、`.db status` 重新检测连接。所有 db.Close 用 safeCloseDB 超时保护。防止数据库不可达时程序长时间挂起。新增 i18n KeyDBTimeoutLabel/KeyDBStatusCmd 中英文翻译。[BUILD-223]
- [x] FEATURE-221 消息序号 `{MESSAGE_NO}` 注入：在每条用户消息和 XML 工具结果消息的 `<environment_details>` 中注入消息在 `a.messages` 数组中的索引序号，LLM 可直接将该序号作为 `adjust_context_start` 的 `target_index` 参数使用。新增 `formatUserMessage`/`formatXMLToolResult` 的 `messageNo` 参数。中英文 i18n 模板同步更新，OpenAI 模式工具描述和 Rules 节补充使用时说明。新增 RULES 节「指令不明确时搜索记忆」规则。[BUILD-224]
- [x] FIX-222 .model add 新增模型时默认使用最高优先级：`wizardEnterModelParams()` 中优先级默认值从 `template.Priority` 改为 `(len(h.cfg.Models) + 1) * 10`，确保新增模型默认排到最高优先级。[BUILD-225]
- [x] **FEATURE-224 System prompt 静态化**：将 system prompt 中的动态内容（`{CURRENT_TIME}`, `{CURRENT_FILES}`, `{CWD}`, `{WORKSPACE}`, `{TASK_TRACKING}`）移至每条 user message 的 `<environment_details>` 信封中，system prompt 会话内永不重建，提升 LLM 前缀缓存命中率。`{TASK}` 保留在 system prompt 表示原始任务目标，使用任务计划标题或 messagePointer 后首条用户消息。旧消息中的动态信封在新 user message 前自动剥离。Capabilities/Rules 节中 `{CWD}` 改为通用描述。Environment 节使用 XML 标签。`listFilesForPrompt` 空目录返回 `(empty directory)` 而非报错。[BUILD-231]
- [x] FIX-227 修复任务计划工具更新时 raw mode 下只有换行没有回车的问题：agent/taskplan_tools.go 中 4 处使用 `fmt.Println(formatted)` 在 enhanced mode raw terminal 下缺少 `\r` 导致输出错位，替换为 `a.defaultIO().Println(formatted)` 通过 UserIO 接口正确处理 `\r\n` 转换。[BUILD-235]
- [x] **FIX-228 修复 SetResultMode 死锁：getCurrentTaskDescription 在持锁期间再次获取 a.mu 导致死锁**：[BUILD-236]
  - `SetResultMode()` 使用 `a.mu.Lock()` 后调用 `getCurrentTaskDescription()`，该方法在无活跃任务计划时再次尝试 `a.mu.Lock()` 导致死锁
  - 修复：缩窄锁范围，`a.resultMode = mode` 赋值加锁后立即释放，system prompt 构建全程在锁外，`a.messages` 写入单独加锁
- [x] **FEATURE-227 重写 loop_detector：改用按行计数法替换基于正则和内容块的复杂检测机制**：[BUILD-232]
- [x] **FEATURE-229 工作模式增强：新增 Plan（规划）和 Research（调研）模式，default 改名 act，模式工具限制可配置，节分隔符程序化，提示符显示模式名**：[BUILD-238]
  - [x] 新增 Plan 模式（Identity 专用提示词、6个节、默认禁用执行/写入类工具）
  - [x] 新增 Research 模式（Identity 专用提示词、12个节、全功能工具）
  - [x] default 改名 act（兼容旧 config.json）
  - [x] 提示符显示模式名 `[👀][act]>`
  - [x] 模式切换时同步工具限制（SyncToolModes 优先读取模式特有配置）
  - [x] `.mode <名称> tools` 子命令管理模式工具开关
  - [x] `.set confirm-tool` 显示当前模式的有效设置
  - [x] 系统提示词 12 节可配置（ToolExamples/TaskProgress/EditingFiles/BrowserUsage 提升为独立节）
  - [x] 节分隔符程序化（`KeySectionSeparator` 动态追加，资源文件移除分节符）
- [x] FIX-230 修复 .set 中 description 不跟随当前工作模式的问题 + 修复 memory_search Usage 与参数不一致的问题 [BUILD-239]
- [x] FEATURE-230 循环检测温度自动调节：当检测到 LLM 输出循环时（threshold=3，原5），自动调整模型温度以打破死循环。使用振荡策略——温度按上升步长递增直到达到上限，然后按下降步长递减直到达到下限，循环往复。上升/下降步长可分别配置（默认 0.05/0.07），溢出部分自动累加到下一方向。新增 `.set loop-temp-*` 配置项（loop-temp-enabled/step-up/step-down/max=1.0/min=0.1）。[BUILD-240]
- [x] FEATURE-244 问题解决模型绑定：WorkMode 新增 ProblemModelID 字段，支持每个工作模式绑定独立的问题解决模型（循环二次判定使用）。循环判定优先使用 WorkMode 绑定的 ProblemModelID，其次使用 ModelID（文本模型），最后使用当前活跃模型。.mode 向导中新增问题解决模型设置/解绑选项。[BUILD-254]
- [x] FEATURE-245 `--unload-mode` 命令行参数：将当前模式下各节系统提示词内容导出到 `mode/{modeName}/` 目录下，每个节一个 `.md` 文件。用户可编辑这些文件来定制 Agent 行为。支持的节包含 Identity、ToolUsage、ResultMode、Capabilities、Rules、Objective、ExternalTools、Environment、ToolExamples、TaskProgress、EditingFiles、BrowserUsage 等。
- [ ] FEATURE-94 命令执行审计功能：在执行 execute_command 工具调用时，先将命令发送给 LLM 进行安全风险分析，LLM 判断命令是否存在风险（如删除文件、修改系统配置、网络操作等）。如果存在风险，提示用户确认后才能执行。支持通过 .set audit-enabled 配置、--audit-enabled/--audit-disabled 命令行参数、config.json 控制审计功能的开启/关闭。
- [x] FIX-232 修复 autoCompleteEndpoint 在非404错误下错误追加 /v1 后缀的问题：autoCompleteEndpoint 以无 API Key 调用 ListModels 时，把 401/403（端点正确但需认证）也视为 "连通性OK" 从而错误地接受带 /v1 的 URL。修复为：先试裸URL，仅404时追加 /v1；同时在 fetchModelSuggestions 带 API Key 后若 ListModels 失败且未含 /vN 后缀时再试一次 +/v1，/vN 后缀的 URL 不再追加 /v。[BUILD-242]
- [x] FIX-233 修复 ask_followup_question 工具在 XML 模式下选项不显示的问题：
  - 选项在 XML 模式下因 parseXMLChildrenToJSON 将 `<item>` 解析为 `{"item": [...]}` 而非 `{"options": [...]}` 而被忽略
  - 修复 askFollowupQuestionTool 增加对 `args["item"]` 的兼容检查
  - 重写交互逻辑：增加取消选项、序号选择+补充说明、无效输入重试
  - 修复中英文 Usage 示例用 `<options>` 包裹 `<item>`，引导 LLM 生成正确格式
  - 验证中英文 66 个 i18n 系统提示 key 完全一致无缺失 [BUILD-243]
- [x] FEATURE-231 模式专属模型配置与参数管理：每种工作模式可绑定独立模型（文本/视觉分开），并设置个性化的 LLM 参数覆盖（temperature/max_tokens/top_p/top_k/repetition_penalty/thinking/reasoning_effort/max_iterations/context_limit/tool_call_mode）。[BUILD-241]
  - [x] 扩展 WorkMode 数据结构（ModelID/VisionModelID/参数覆盖字段）
  - [x] 新增 Agent.ApplyWorkModeConfig() 方法统一模型选择和参数合并
  - [x] 模式切换时自动应用绑定模型
  - [x] 新增 `.mode` 向导式交互界面（一级菜单显示所有模式和模型、二级菜单编辑详情）
  - [x] 新增 `.mode model` 子命令绑定/解绑模型（交互式编号选择模型）
  - [x] 新增 `.mode param` 子命令管理参数覆盖
  - [x] `.model edit` 子命令编辑现有模型参数
  - [x] 修复端点版本号后缀检测（硬编码 /v1 改为正则 /v\d+$）
  - [x] 修复向导回退导航（按 0 返回上一步时保留已填值）
  - [x] 模型绑定向导显示实际模型列表供编号选择而非手动输入 ID
  - [x] 内建模式默认温度（act=0.6, plan=0.5, research=0.7）
- [x] FIX-234 修复 get_memory_slice / delete_memory / memory_search 工具在 raw terminal 下输出只有换行没有回车的问题：agent/memory_tools.go 中 3 处使用 `fmt.Println(...)` 在 enhanced mode raw terminal 下缺少 `\r` 导致输出错位，替换为 `a.defaultIO().Println(...)` 通过 UserIO 接口正确处理 `\r\n` 转换。[BUILD-244]
- [x] FEATURE-106 实现history命令翻页：支持通过上下键浏览、.history last/first 命令查看、编号重新执行历史命令，数据持久化到 bbolt [BUILD-68]
- [x] FEATURE-237 简化任务计划（Task Plan）机制：将 create_task_plan/update_task_step/insert_task_steps/remove_task_steps 四个 LLM 工具合并为统一的 track_task_progress 工具。LLM 只需一次性传递完整的 steps 数组（含 description + status），Manager 自动处理创建/替换。支持 step.description 多行文本（首行标题，后续详细内容）。plan description 应包含详细方案。Plan Mode 结束时自动调用 track_task_progress 记录方案。[BUILD-245]
- [x] **FEATURE-238 系统提示词示例优化：将 execute_command 示例改为 search_files，将 replace_in_file 数组示例改为 track_task_progress** [BUILD-247]
  - 将多工具调用示例中的 `<execute_command><command>ls -la</command></execute_command>` 替换为 search_files 示例（zh + en）
  - 将数组参数示例从 replace_in_file 改为 track_task_progress，展示 `<item>` 标签用于 steps 数组（zh + en）
  - 同步更新英文版 Plan Mode Strict Prohibitions 添加 `(prohibited:)` 标注
- [x] FEATURE-239 Ctrl+C 快速取消任务：当 LLM 输出或执行工具调用时，用户按 Ctrl+C 直接退出迭代到 co-shell 的命令提示符，跳过确认步骤。保留 ESC 先中断并提示用户是否取消的现有机制。两者行为分开：ESC = 暂停 + 确认，Ctrl+C = 立即取消。[BUILD-248]
- [x] FIX-240 循环检测机制优化：
  - [x] 移除非预期的 removeLastAssistantWithToolCalls：循环检测在流式阶段触发，当前迭代的 assistant 尚未追加到 a.messages，但代码错误删除了前一次迭代的正常消息和工具结果。修复为仅注入反馈提示，不再删除已有上下文 [BUILD-249]
  - [x] 内容循环检测改为每次迭代 reset（intra-iteration）：count 跨迭代累积会导致 LLM 复用常见短语时误触发 [BUILD-249]
  - [x] 新增工具调用循环检测（inter-iteration）：检测同工具 + 同参数在连续迭代中的重复调用，达到阈值时注入独立反馈提示 [BUILD-249]
  - [x] .mode 命令 mode 列表按名称排序，确保编号跨调用稳定 [BUILD-249]
- [x] FEATURE-242 系统提示词 WORK MODE 模板、工具结果去重、{TASK} 刷新修复、每条消息追加环境信息 [BUILD-252]
  - 将 RESULT MODE / Result Processing Mode 改为 WORK MODE / Work Mode Description 模板
  - 从 KeyXMLToolResultTemplate 移除 {TOOL_CALL_PARAMETERS}，避免工具结果中重复输出参数
  - 每轮用户输入后 rebuildSystemPrompt() 刷新 {TASK} 占位符内容
  - getCurrentTaskDescription() Priority 1 增加 HasUnfinished() 守卫，计划已归档/完成后不再返回标题
  - 新增 injectTimeAndMessageNo() 为所有 user/tool 消息追加 <time> 和 <message_no>
- [ ] FEATURE-45 自动更新机制（通过github）。
- [ ] ENHANCEMENT-49 性能基准测试。
- [ ] FEATURE-50 完整文档站。
- [x] FEATURE-120 新增Excel文件编辑工具（纯标准库实现 XLSX），提供 12 个拟人化 LLM 工具（excel_open/close/save/overview/read/edit/copy/paste/insert/delete/sheet/format），支持会话管理（永不超时/超过max同时提示LLM关闭）、复制粘贴剪贴板（含剪切模式）、行列插入删除、公式读写、样式引擎（字体/填充/边框/对齐/合并/行高/列宽/数字格式/merge/unmerge）、上下文保护（excel-max-cells可配置/默认1000）、reset/merge 两种格式模式（merge为默认，保留已有样式不改写）、中英文XML方法说明、.set 可配置参数（excel-max-sessions、excel-max-cells）、buildCount=289 [BUILD-289]
- [x] **FEATURE-121 新增Word文件编辑工具**：纯Go标准库实现（archive/zip + encoding/xml），提供11个拟人化LLM工具（word_open/close/save/overview/read/table_read/continue/erase/inspect_style/format/style_clone），支持HTML输出格式（simple/full双模式）和格式继承机制（same_style_as参数）。[BUILD-291]
- [ ] FEATURE-125 建立备用异常处理机制，以便在主LLM报错时，有另外独立的诊断渠道，可以通过异常信息库协助处理问题。
- [ ] FEATURE-128 增加移动端APP和co-shell-hub，以便用户能够在手机端安全的操控co-shell。
  - [x] 使用 Flutter 开发 iOS/Android 跨平台应用
  - [x] 所有移动端代码放在 mobile/ 目录下
  - [x] 通信协议：UDP + 首次请求密钥验证（无密钥不回包，降低被扫描风险）
  - [x] 实现 co-shell-hub（多 agent 管理服务端）
    - hub 监听 UDP 端口，处理握手验证
    - 管理多个 co-shell 实例的生命周期
    - 消息路由（根据 agent_id 转发）
    - 会话管理（创建/切换 agent）
  - [x] FEATURE-183 co-shell-hub 改进：简化认证流程，使用昵称+access key 替代公钥签名 [BUILD-180]
    - 新增 --add-client 命令注册移动端客户端
    - 新增 --gen-key 命令生成密钥对
    - 新增 --dev 开发模式（返回错误详情）
    - 新增 --log-dir/--log-level 日志配置
    - 握手协议简化：客户端发送 nickname + access_key
    - 消息自动添加 "<昵称>说：" 前缀
    - 移动端 Flutter 代码同步更新
  - [ ] 移动端支持多 agent 会话列表
  - [ ] 支持功能：聊天界面、语音输入、图片选择、任务计划查看
- [ ] FEATURE-129 增加语音识别模型，以便用户能够与co-shell通过语音进行沟通，计划支持GPU和CPU部署，可以通过co-shell自主安装所需要的模型和服务。
- [ ] FEATURE-136 在Agent策略中，增加让LLM预测用户下一步操作的机制，提供几个选项给用户选择，以便提升人机协同效率和自动化程序
- [x] FEATURE-241 死循环 LLM 二次判定机制
   - [x] 异步模式：检测到循环时不中断流式，后台 goroutine 调用判定 LLM
   - [x] 异步判定并发保护（loopJudgeInflight），判定期间忽略重复检测
   - [x] 同步模式：未启用判定时保持原有立即中断+温度调整行为
   - [x] 判定结果确认循环时清空所有检测计数器（content + toolCall）
   - [x] 判定收发均写入日志（LoopJudge request/response/result）
   - [x] 完整输出内容通过 lastLlmOutput 传给判定模型
   - [x] 提示词标注内容因循环检测被中断的信息
   - [x] 新增 loop_judge_enabled / show_loop_detection 配置项
   - [x] 所有模式默认温度改为 0
    - [x] BUILD-250
- [x] FEATURE-246 增加 LLM 交互完整日志，独立文件记录请求/响应 JSON [BUILD-256]
  - 新增 log/llm-interaction-YYYY-MM-DD.log 独立日志文件
  - 支持独立开关（.settings llm-log on/off），默认关闭
  - 流式输出实时记录，首 chunk 带 [RESP][assistant] 头，后续直接追加
  - 流结束时追加 usage JSON 并写入分隔符
  - LoopDetectThreshold 默认值从 3 改为 5
  - 循环二次判定（judgeLoop）自动写入同一日志文件
- [x] FEATURE-247 Token 计数机制完善与三层统计架构 [BUILD-258]
  - 修复流式 SSE finalUsage 未传递到 StreamEvent.Done 的问题
  - 修复 attempt_completion 路径未发送 token_usage 的问题
  - 实现三层 token 统计：迭代级（IterTokenDelta）、任务级（TaskTokenUsage）、会话级（TokenUsage）
  - 每次 LLM 迭代结束后显示本轮用量（prompt/completion/total）及性能指标（TTFT/输入速度/输出速度）
  - 任务完成时显示该任务累计 token 用量
  - `<environment_details>` 中的 `<context_window>` 改用单次迭代数据
  - 新增 LLM 性能时序追踪：llmCallStartTime/firstTokenTime/llmStreamEndTime
  - 重构 token 回调体系为 token_iter / token_task / .about（待完成）
  - 新增 `.model info` 显示 MaxModelLen
  - 新增 `knownMaxModelLen` 已知模型长度字典（DeepSeek v4 flash/pro = 1M）
  - `.model add`/`.model edit` 一键式向导增加 API 检测 MaxModelLen 步骤
  - `.model edit` 增加编辑模型最大上下文长度字段，支持 API 重新检测
  - 支持 K/M 单位输入（如 "64K"/"1M"）
  - 支持 `.set token-usage off` 模式跳过显示
- [x] FEATURE-249 上下文重新整理工具（reorganize_context）、LCS内容重复检测、自动上下文重整配置 [BUILD-262]
- [x] FEATURE-250 调试模式（Debug Mode）：在提交内容给LLM之前显示预览，用户可修改内容后发送。支持 --debug CLI参数 和 .set debug REPL命令，默认为关。新增 `[ 开发者 ]` 设置分组。 [BUILD-263]

- [x] FIX-251 修复 injectTimeAndMessageNo 未被调用导致历史消息缺少 environment_details 的问题 [BUILD-265]
  - injectTimeAndMessageNo 函数在 FEATURE-248 中定义但从未被调用，导致历史 tool/user 消息的 ContentParts 中没有独立的 `<environment_details>` 段
  - buildContextMessages() 不再重新注入 environment_details，改为在消息创建时立即冻结
  - 提取 buildFullEnvironmentDetails 公共方法，所有消息（user + tool）获得完整版 env 信息
  - 新增 add_images intent 必填参数，read_file 二进制检测，.session pop 子命令

- [x] FEATURE-252 为所有LLM工具添加意图(intent)参数，以便跟踪LLM调用意图 [BUILD-266]
  - 为 buildToolsInternal() 中除 add_images 外的所有工具定义添加必填 intent 参数
  - 补充说明：track_task_progress、attempt_completion、ask_followup_question、reorganize_context 四个工具不添加 intent 参数
  - executeToolCall() 返回结果时自动追加 [意图] 提醒 LLM 回顾原始调用目标
  - 同步更新中英文 XML 模式工具描述（zh_system.go / en_system.go）

- [x] FIX-252 修复 add_images 图片数据未发往 LLM 的问题 [BUILD-267]
- [x] FEATURE-253 内容循环检测改进：采用 M-max 三规则算法（diff==0递增/diff==1跟进/diff>1清空），移除 minLineLen 短行过滤，新增全局最大计数 M 机制，仅连续重复触发；visual_analysis 改为单文件接收（path 替代 paths）；<context_window> 输出格式化；reorganize 建议合并到上一条 user 消息 [BUILD-268]
- [x] FEATURE-254 system prompt 每次从磁盘重新读取配置文件：`rebuildSystemPrompt()` 中从磁盘重新加载 config.json；read_file 输出格式简化、start_line/end_line 改为必填；同步更新中英文 XML 模式工具说明。[BUILD-269]

- [x] FIX-255 XML 嵌套解析错误传播：`parseXMLChildrenToJSON` 递归解析 `<item>` 子元素时，如果子元素缺少闭合标签（如 `</item>`），`nestedErrors` 未被向上传播到父级 `parseErrors`，导致 XML 良构性错误被静默吞掉，最终被下游工具误报为 "missing 'search' and 'replace' fields"。修复为在 fallback 路径中将 `nestedErrors` 追加到 `parseErrors`。[BUILD-270]

- [x] FEATURE-256 `.session pop` 支持数字参数 N（默认 1），一次弹出 N 条非 system 消息，保留最后 1 条供编辑，前面的直接丢弃。[BUILD-272]

- [x] FIX-257 reorganize_context 消息结构规范：`<task>` 内容不再作为独立 user 消息，改为写入 taskInstructionCache，在迭代末尾 flush 到工具结果消息的 ContentParts 中（位于结果和 `<environment_details>` 之间），与 visual_analysis 行为一致。调整 taskInstructionCache flush 到 env injection 之前，确保 `<task>` 在 `<env>` 之前。[BUILD-273]

- [x] FIX-258 会话上下文持久化修复：[BUILD-275]
  - `store/bolt.go`: `SaveSession` 改为直接存裸 `[]byte`（不再内置 `SessionData` 包装序列化），`LoadSession` 在事务内复制数据避免 bbolt mmap 引用失效
  - `agent/agent.go`: `PersistSession()` / `PersistSessionNonSystem()` 自行构建 `SessionData` 序列化再写入；`RestoreSession()` 正常解包 `json.RawMessage` 格式的 `Messages` 字段
  - `RunStream` / `Run` 加 `defer PersistSessionNonSystem()` 覆盖 Ctrl+C、ESC 取消、错误退出、max iterations 所有路径
  - REPL `cleanup()` 中调用 `PersistSessionNonSystem()` 确保正常退出也持久化

- [x] FIX-259 reorganize_context 相关修复：[BUILD-276]
  - `reorganizeContextOnLoop()` 默认分支不再清空 `a.messages`（原来只保留 system + last user），避免循环检测时丢失全部历史消息
  - `reorganizeContextTool()` 写入 `taskInstructionCache` 时不再加 `<task>` 包装（由 flush 统一处理），修复 `<task>` 标签双重嵌套问题
  - `jsonValue()` 检测到数字前导零（如 `0067`）时返回带引号字符串而非裸数字，修复 `{"search": 0067}` 非法 JSON 错误

- [x] FIX-260 调试模式运行时切换（:debug on/off）：[BUILD-276]
  - `debugIntercept()` 中 `> ` 提示符支持 `:debug on`/`:debug off` 即时切换，消息照常发送
  - ESC 中断提示支持 `:debug on`/`:debug off` 即时切换后自动重试 LLM 调用

- [x] FIX-261 `.continue` 命令：发送当前完整上下文给 LLM，不追加新消息 [BUILD-276]
- [x] FIX-262 XML 解析调试日志增强：`stream_response.go` 在 XML 解析关键路径加 INFO 级别日志（原始内容预览、解析结果、报错详情）[BUILD-276]
- [x] FIX-263 `main.go` 启动顺序修复：`SetResultMode()` 移到 `RestoreSession()` 之前，删除重复的 `SetResultMode()` 调用，解决 session 恢复后被清零的问题 [BUILD-276]

- [x] FEATURE-264 上下文超限提前检测并跳过低质量工具执行：[BUILD-277]
  - 将上下文超限检测从迭代末尾提前到 toolCalls 执行前
  - 超限时通过 `reorganizePending` 标记跳过 assistant 消息添加和工具执行
  - 迭代末尾 flush 后追加干净的用户消息（含 `<task>必须整理上下文</task>` + `<environment_details>`）
  - LLM 已调用 `reorganize_context` 时不跳过执行也不追加重复提醒
  - 删除了迭代末尾的旧 FEATURE-249 超限检测逻辑（已无效的重复代码）

- [x] FEATURE-265 write_to_file 增加 mode 参数（new/rewrite/append）：[BUILD-277]
  - `agent/file_tools.go`：实现三种模式互斥验证（new 文件存在报错、rewrite/append 文件不存在报错、append 追加写入）
  - `agent/tools.go`：tool 定义增加必输 `mode` 参数
   - `i18n/zh_system.go` / `i18n/en_system.go`：更新 XML/JSON 示例及工具说明

- [x] **FEATURE-266 循环检测算法改进：用滑动窗口周期检测替代 M-max 行级计数** [BUILD-279]
  - 新的周期检测算法维护 FNV-1a hash 行缓冲区，检测末尾行序列是否存在周期 p（1..8）且重复次数 ≥ threshold
  - 正确识别所有周期模式：AAAA（p=1）、ABAB（p=2）、ABCABC（p=3）、ABACABAC（p=4）、ABCDABCD（p=4）等
  - ABACABAC 等 M-max 无法检测的模式现在能正确检测
  - 散乱分布/无规律行不会形成有效周期，保持正确的误报率控制
  - hash 碰撞保护：hash 匹配后做实际字符串比较确认
  - 所有旧测试保留，新增 ABACABAC 和 ABCDABCD 周期循环测试

- [x] **FEATURE-267 循环介入策略统一及工具调用循环适配** [BUILD-280]
  - 将 `LoopDetectEnabled`/`LoopTempEnabled`/`LoopReorganizeEnabled` 三个独立开关合并为 `LoopIntervention` 枚举值（off/retry/prompt/reorganize/temperature/random）
  - 工具调用循环（ToolCallLoopDetector）使用与内容循环相同的 LoopIntervention 策略
  - 所有循环检测 i18n 资源从 zh.go/en.go 迁移到独立的 zh_loop.go/en_loop.go
  - 循环判定 prompt 增加 `{ITERATIONS}` 占位符（最近 2 次无工具调用回复），新增 reason/exit_strategy 语义说明
  - 设置显示重构：`allLines` + `nextLines(N)` 计数模式改为 `allGroups [][]settingLine` 二维数组分组
  - Token 用量合并为单行，组标题 "[ 安全与异常 ]"
  - 默认工作模式改为 research，research 模式缺省去掉 ToolExamples 节
  - `LoopLongOutputThreshold` 默认值设为 65536

- [x] FIX-264 修复 Ctrl+C 中断后上下文持久化丢失最后 2-3 条消息：[BUILD-278]
- [x] FIX-272 修复循环二次判定 JSON 解析未跳过 think 标签的问题：在 `judgeLoop()` 提取 JSON 前先通过 `</think>` 标签定位实际返回内容，避免 think 内容中的 `{` 干扰解析 [BUILD-286]
- [x] **FEATURE-273 循环检测统合优化** [BUILD-287]：
  - 新增 `LoopEvent` 统一事件结构体，三种检测器都产生 `LoopEvent` 走统一的 `applyLoopIntervention()` 路径
  - 跨迭代内容重复：相似度阈值（`DuplicateContentThreshold`，默认 0.95），触发后走统一 LoopIntervention 而非 continuePrompt
  - 工具调用重复：移除旧 threshold，连续两次相同工具+参数即触发（ToolCallLoopDetector threshold=2）
  - 新增流内单行重复检测（`SingleLineLoopDetector`）：长行超限（默认 2048 字符）或窗口内周期重复（默认 128 字符窗口，minRepeat=3）
  - 新增 `.set` 支持：`loop-single-line-length`、`loop-single-line-window`
  - REPL：用户输入以 `.` 开头时，检查是否为本地可执行文件，如果不是则提示用 `:` 前缀并确认是否继续
- [x] **FEATURE-270 系统提示词简化 + 命令前缀冒号化 + 调研步骤补充** [BUILD-284]
  - 去掉 Capabilities/Rules/ToolUsage 的 shellEnabled/plan mode 分支逻辑，统一使用通用资源
  - 删除 keys.go/zh_system.go/en_system.go 中不再使用的 Shell/XML/ReadOnly key 和翻译块
  - 同步更新英文版各节与中文版结构一致
  - LLM 交互日志（llm-interaction-YYYY-MM-DD.log）补充 judgeLoop 收发内容
  - 新增 DefaultActSections() 供 act/research 模式使用（不含 ToolExamples），保留 DefaultBuiltInSections() 完整节列表
  - KeyWorkModeResearch 补充调研工作的 7 个基本步骤
  - **REPL 内置命令前缀从 `.` 改为 `:`**（如 :settings/:help/:plan 等），去掉了 `.` 兜底逻辑
  - 同步更新 i18n/zh.go/i18n/en.go 中所有命令引用、帮助文本、提示信息
  - 同步更新 cmd/config.go 配置向导中的命令名
- [x] **FEATURE-274 密码本(Vault)功能：AES+SM4 双加密引擎** [BUILD-288]：
  - 加密存储的账号/密码，支持 AES-256-GCM 和 SM4-GCM 国密算法
  - `@pwd:` / `@user:` / `@vault:` 占位符语法，在工具执行最后一刻注入真实值
  - LLM 不可见的敏感信息传递（占位符替换在工具回调前、用户确认后执行）
  - 用户确认机制复用现有 confirm-tool 框架，包含敏感占位符的调用强制确认
  - `store/vault.go` — 加密存储层（双引擎 + PBKDF2 密钥派生 + bbolt CRUD）
  - `agent/vault.go` — LLM 工具（vault_list / vault_add / vault_remove）+ 占位符解析注入 + 脱敏
  - `agent/tools.go` — `executeToolCall()` 集成占位符替换钩子
  - `cmd/vault.go` — `:vault` 内置命令（含 re-encrypt 迁移）
  - `config/config.go` — VaultConfig（enabled / timeout / algorithm）
  - 系统提示词增加密码本使用说明（中英文）

- [x] **FIX-275 修复 Excel 保存时共享字符串（type="s"）单元格丢失 t 属性导致第1行显示为 1,2,3...** [BUILD-292]
  - 根因：`xlsx/writer.go` 的 `writeSheetXML` 函数缺少 `type="s"` 写入分支
  - 模板文件第1行使用共享字符串（`type="s"`），但保存时 `t="s"` 属性丢失
  - Excel 将 SST 索引值（0,1,2...）当作字面值显示，而非查表取字符串
  - 修复：新增 `type="s"` 写入分支，正确输出 `t="s"` 属性

- [x] **FEATURE-276 会话（Session）管理功能增强**：[BUILD-298]
  - 导出当前会话上下文到外部 JSON 文件（:session export [path]）
  - 从外部会话文件重新导入上下文（:session import <path>）
  - 通过 attempt_completion 的 session_title / session_keywords 参数由 LLM 自动命名并保存会话
  - 手动命名保存会话（:session save [title]）
  - 列出已保存的会话（:session list）
  - 会话切换（:session switch <id>），自动保存当前会话
  - 删除已保存的会话（:session delete <id>）
  - 会话帮助（:session ?）
  - :new 保存当前会话后再新建空会话
  - 新增 :reset 命令重置当前会话消息
  - TODOs are stored in BoltDB sessions bucket with format sess-YYYYMMDDhhmmss-xxxxxxxx

- [x] FIX-295 修复 token 用量统计显示多余空行和重复区块的问题：[BUILD-325]
  - repl/repl.go: token_iter/token_task 事件处理仅在有内容时才打印分隔线
  - agent/run_stream.go: 消除 token_task 多次发射路径

- [x] FIX-299 修复 SingleLineLoopDetector 未接线及二次判定计数器未重置问题：[BUILD-299]
  - 将 SingleLineLoopDetector 接入 LoopDetector.AddChunk，修复单行超长（2048 字符）和窗口周期检测（128 字符）完全不可用的问题
  - 在 RunStream 中初始化 SingleLineLoopDetector 并注册到 LoopDetector
  - handleLoopDetection 中判定非循环时已会自动 Reset() 清空检测器缓冲区
  - 补充 attempt_completion 的 session_title/session_keywords 参数到 XML 多语言说明

- [x] FIX-277 优化默认参数：loop-detect-threshold=2，word/excel 工具除 delete 外默认 auto [BUILD-300]
  - config/config.go: LoopDetectThreshold 默认值从 5 改为 2
  - agent/agent.go: word_continue/word_format/excel_edit/excel_paste/excel_insert 从 confirm 改为 auto
  - agent/agent.go: word_erase/excel_delete 保持 confirm
- [x] FIX-278 excel_edit 增加 TSV fallback 解析和 i18n 用法示例补全 [BUILD-300]
  - agent/excel_tools.go: excelEditTool 中去掉 JSON 解析，改为纯 TSV 模式，保留多行 CSV fallback
  - i18n/zh_system.go / i18n/en_system.go: 补全 excel_edit XML 用法示例，改为 TSV 格式说明
- [x] FIX-279 循环检测计数器在干预后未重置问题
    - agent/loop.go: applyLoopIntervention 中按事件类型精准重置触发检测器，避免同模式刚发完反馈就再次触发
- [x] FEATURE-179 EML 文件解析工具：新增 bin/eml2json.py，解析 EML 文件（标题、收发件人、时间、正文、附件），以邮件时间命名文件夹输出 metadata.json 和附件文件 [BUILD-302]

- [x] **FEATURE-280 工具执行中支持 Ctrl+C 中断**：[BUILD-303]
   - 将 cancelCh 与 context.Context 绑定，创建 cancelCtx
   - executeToolCall() 中增加 context cancel 检查
   - executeSystemCommand() 使用进程组 kill 可杀嵌套进程（python3 | head）
   - run_stream.go 工具执行循环使用 cancelCtx 传递 Ctrl+C 信号

- [x] **FEATURE-17 0-tool-call 时不保留 assistant 文本消息**：[BUILD-304]
   - 当 LLM 返回纯文本且未调用 attempt_completion 时，不将 assistant 消息加入对话历史
   - 只追加一条强指令 user 消息，要求 LLM 要么调用 attempt_completion 要么调用工具
   - 避免 LLM 在后续迭代中"维护"自己之前不调用工具的判断
   - 同步更新 i18n 继续提示文案

- [x] **FEATURE-213 visual_analysis 支持一次识别多个图片，总数上限由系统参数配置**：[BUILD-306]
   - LLMConfig 新增 VisualAnalysisMaxImages 字段（默认 5）
   - 工具参数从 `path`（string）改为 `paths`（string array）
   - visualAnalysisTool 批量处理，超过上限时截断并提示
   - i18n 中英文翻译 + .set 参数注册
   - main.go build 计数 +1

- [x] FEATURE-214 增加 Kimi API 供应商支持（含视频输入）[BUILD-307]
    - config/provider.go - 新增 kimi ProviderPreset（endpoint: https://api.moonshot.cn，模型: kimi-k3/k2.7-code/k2.7-code-highspeed/k2.6/k2.5）
    - config/model_template.go - 新增 kimi-official ModelTemplate（支持多模态、Thinking、ToolCall，默认 1M 上下文）
    - llm/thinking_adapter.go - kimiThinkingAdapter 支持 thinking + reasoning_effort
    - llm/client.go - 新增 ContentPartVideoURL 类型 + ContentPartVideo 结构体
    - agent/image_tools.go - visual_analysis 扩展支持视频文件编码

- [x] FIX-283 修复 Windows 上 StdioIO.ReadKey() 和 ask_followup_question 工具无限循环、不等待用户输入的问题 [BUILD-312]
    - 根因1：handleAgentInput 在 stdio 模式未设置 UserIO，agent 回退到 fmtIO.ReadKey() 永远返回 (0, nil)
    - 根因2：StdioIO.ReadKey() 在 Windows 上未启用 raw console mode
    - 修改 repl/repl.go：stdio 模式下创建 StdioIO 并设置给 agent
    - 修改 repl/userio.go：StdioIO.ReadKey() 调用 MakeRaw() 启用 raw mode
    - 修改 repl/raw_term_windows.go：实现真正的 Windows Console API raw mode

- [x] **FEATURE-281 write_to_file 工具描述添加大文件分次写入建议** [BUILD-313]
    - 在 agent/tools.go 中 write_to_file 的 Description 末尾追加 PERFORMANCE TIP（OpenAI 模式）
    - 在 i18n/zh_system.go 和 i18n/en_system.go 的 XML 模式描述中追加中文/英文提示
    - 建议 LLM 写入超过 ~100 行的大文件时，先 new 首段再多次 append
    - 避免单次输出超长字符触发 LoopLongOutputThreshold（默认 32768 字符）误报

- [x] **FIX-284 修复 Windows 中文命令乱码及超时误报** [BUILD-314]
  - 问题1：Windows 上 execute_command 含中文字符时 UTF-8 编码在 cmd.exe 中变成乱码
  - 问题2：乱码导致 exit code 1 退出被 Windows 版 isSignaledExit() 误判为"超时"
  - 修复1：Win32 API GetACP() 动态获取系统活动代码页，UTF-8 ↔ ACP 双向转码，适配任何语言 Windows
  - 修复2：executeSystemCommand 增加 atomic.Bool 超时标记，仅实际杀死进程才报告超时

- [x] **FIX-285 修复同步路径循环检测未使用判模模型 exit_strategy 的问题** [BUILD-314]
  - 当 loop-intervention=prompt 且 loop-judge-enabled=on 时，流式路径（同步模式）的循环检测虽然调用了判模模型，但其返回的 exit_strategy 被丢弃
  - handleLoopDetection 仅用判模结果决定是否中断流，未保存 exit_strategy
  - RunStream 中同步循环处理块的 prompt 分支始终使用通用模板，无视判模结果
  - 修复：loopJudgeExitStrategy 字段暂存 exit_strategy，prompt 分支优先使用
  - 修复：同步路径反馈消息也应用 <task> 标签包裹，与 applyLoopIntervention 路径一致

- [x] **FEATURE-286 0-tool-call 处理方式参数 no-tool-action + SingleLineLoopDetector www 误报修复** [BUILD-316]
  - 新增 `no-tool-action` 配置参数（exit/retry/prompt），默认 retry，控制 LLM 返回 0 个工具调用时的后续行为
  - exit：不调工具视为任务完成，追加 assistant 消息后退出迭代循环
  - retry：丢弃当前 LLM 回复，不追加消息也不记录 memory，直接循环顶部重试
  - prompt：丢弃当前 LLM 回复，记录 memory，追加 continuePrompt 强指令消息后重试
  - 去掉现有的跨迭代内容去重检查（IsDuplicateContent），统一由新参数管理
  - 新增 `.set no-tool-action` 子命令支持
  - `.config` 向导及 `.set` 显示同步支持
  - 修复 SingleLineLoopDetector 字符级周期检测将 URL 中 `www` 误判为循环的问题
  - Rule (b) 中确认周期匹配后继续扫描整个 window，只有周期填满整个窗口且延伸到窗口之前的内容才触发
  - 短模式（如 `www` 仅 3 个字符重复）不再误报

- [x] **FEATURE-287 方法调用解析错误统一处理参数 parse-error-action** [BUILD-317]
  - 新增 `parse-error-action` 配置参数（exit/retry/prompt），默认 retry，控制所有方法调用解析错误（XML 解析错误、流式 tool call 增量校验失败、工具执行失败）的后续处理策略
  - exit：退出迭代循环，向用户报告错误
  - retry：无反馈，直接重发上下文
  - prompt：发送结构化错误反馈（含参考格式）给 LLM，让 LLM 自行修正
  - 统一四个错误路径的行为：XML 解析错误、流式错误+移除 assistant 消息后、工具执行失败、流式 tool call 全部无效
  - 新增 `.set parse-error-action` 子命令支持
  - `.config` 向导及 `.set` 显示同步支持

- [x] **FEATURE-288 命令行参数与 REPL 参数补齐** [BUILD-319]
  - CLI 补充缺失参数：--mode、--thinking-enabled、--reasoning-effort、--max-retries、--context-limit、--shell-session-enabled、--browser-enabled
  - `:set` 补充缺失参数：token-usage
  - CLI help (usage.go) 补齐缺失行和 i18n 翻译：--output-mode、--token-usage、--debug、--body-add、--init-capabilities、--init-rules
  - i18n/zh.go 补齐所有缺失 CLI help 项中文翻译

- [x] **FIX-289 单命令行模式下 Token 用量显示参数顺序错误** [BUILD-320]
  - 根因：executeSingleCommand() 的 token_iter 分支使用 i18n.T(i18n.KeyTokenUsageDisplay) 时只传了 4 个参数（prompt, completion, total, pct），但格式串期望 7 个参数（ft, prompt, inTPS, completion, outTPS, total, pct），导致 fmt.Sprintf 输出 `%!s(int=37066)` 等乱码
  - 修复：main.go 中 executeSingleCommand 的 token_iter 分支改为与 REPL repl.go 一致的参数顺序

- [x] **FEATURE-290 命令行 --session-id 参数：支持指定 session ID 追踪对话上下文** [BUILD-320]
  - 新增 `--session-id`（`-s`）命令行参数
  - 如果指定的 session ID 在 DB 中已存在，加载对应 session 的历史消息
  - 如果不存在，创建新 session（以该 ID 命名）
  - 执行后 PersistSessionNonSystem 写回该 session
  - 同步更新：usage.go help、i18n 翻译、`--help` 示例 12
  - 外部调用方可通过此参数管理多组独立对话上下文

- [x] FIX-291 代码块中 XML 方法调用被误解析为正式工具调用：在 ParseXMLToolCallsWithTools 前增加 stripCodeBlockXML，移除 ```...``` 包裹的代码块内容，避免 LLM 讨论 XML 调用格式时被误执行 [BUILD-321]
  - 附带修复：toolUsageKeyMap 中 browser_get_html → browser_get_rendered_html 键名映射，修复 FEATURE-223 改名后 XML 用法说明缺失的问题

- [x] FEATURE-292 移除 `<task>` 标签包裹策略：`<task>` 标签人为扭曲 LLM 注意力优先级，导致 LLM 只关注 `<task>` 中的内容而忽略同一 user message 中其他的进展展示。改为纯文本/无标签方式传递用户指令和补充输入。[BUILD-321]

- [x] FEATURE-293 no-tool-action 增强：增强 XML 工具调用识别，在 LLM 意图调用工具但格式错误时不走 no-tool-action（exit），而是走 parse-error-action（retry/prompt）。
  - [x] 将 NoToolAction 的默认值从 "retry" 改为 "exit"
          - [x] 阶段1：尾标签反向匹配 — 头标签不认识但尾标签是已知工具名时，产生 parse error
          - [x] 阶段2：参数签名检测 — 头尾都不认识但内部参数标签 ≥1 个匹配已知参数名时，产生 parse error
          - [x] hasToolAttempt 信号传递 — streamLLMResponse 返回额外布尔值，RunStream 根据该值分流到 parse-error-action

- [x] **FEATURE-294 系统提示词优化**：[BUILD-323]
  - [x] attempt_completion OpenAI 模式 Description 补齐 `session_title`/`session_keywords` 说明
  - [x] 解决 `.clinerules/问题解决策略.md` 中"外科手术式改动"与"简单优先"的逻辑矛盾（冲突时简单优先优先）
  - [x] XML 解析新增 `stripQuotedXMLContent()` — 忽略引号包裹的 XML 标签
  - [x] XML 解析新增 `stripThinkBlock()` — 剥离 `</think>` 前内容
  - [x] 将 `KeyAgentDefaultDescription{Act,Plan,Research}` 从 `zh.go`/`en.go` 迁移到 `*_system.go`
  - [x] 重构图 `zh_system.go`/`en_system.go` 头部：4 条原则嵌入 Identity，WorkMode 清空
  - [x] `exit_strategy` 提示要求改为纯前瞻性指导意见（不回溯、不评价、指明下一步）

- [x] **FEATURE-295 优化系统提示词结构，三个内置模式默认只保留6个节（Identity/ToolUsage/Capabilities/Rules/ExternalTools/Environment）**：[BUILD-326]
  - [x] 缩减 DefaultBuiltInSections() 为 Identity → ToolUsage → Capabilities → Rules → ExternalTools → Environment
  - [x] 缩减 DefaultActSections() 为相同的6个节
  - [x] 缩减 DefaultPlanSections() 为相同的6个节
  - [x] 新增 DefaultResearchSections() 独立函数，research 模式不再共用 act 的节配置
  - [x] 修复 OpenAI 模式 content_chunk 受 showLlmContent 控制，StreamEventToolCall 新增实时工具调用输出 `[🔧 tool_name]` 由 showTool 控制
  - [x] 验证编译通过

- [x] **FEATURE-296 XML 工具调用外层包裹标签 `<cs_tool_calls>`**：[BUILD-328]
  - [x] 新增 XMLToolWrapperTag 配置字段（默认 cs_tool_calls），支持通过 config.json + CLI + REPL 配置
  - [x] ParseXMLToolCallsWithTools 先提取 wrapper 标签块再解析内部工具调用
  - [x] 更新 i18n 中英文系统提示词示例
  - [x] 更新测试

- [x] **FEATURE-297 XML 工具调用标签添加可配置前缀**：[BUILD-332]
  - 去掉 `<cs_tool_calls>` 外框，工具标签和参数标签均使用可配置前缀（默认 `cs:`）
  - 新增 `xml-tag-prefix` 配置项（config.json + CLI + REPL）
  - 更新 ParseXMLToolCallsWithTools 解析逻辑 — 只识别带前缀标签，无前缀标签完全跳过
  - 更新 i18n 中英文系统提示词示例 — 使用 `{XML_TAG_PREFIX}` 占位符
  - 更新测试 + 新增 `bin/check_xml_prefix.py` 验证工具

- [x] **FEATURE-298 流式 XML 工具调用增量校验 — 提前发现拼写错误**：[BUILD-337]
   - 实现 StreamingXMLValidator 状态机，在 LLM 流式输出时逐 chunk 增量解析 XML 结构
   - 分层验证：L1 语法（标签名合法性、闭合匹配）、L2 语义（工具名已知性、参数名已知性）
   - 致命错误（拼写错误工具名、错误参数名、标签不匹配）立即终止 LLM 流，通过 cancel context 中断
   - 可恢复错误（不完整标签、未闭合标签）继续等待后续 chunk，仅流结束后报告
   - 集成到 streamLLMResponse 的 StreamEventContent 处理分支
   - 新增 `xml-stream-validate` 配置项（默认 on），可通过 `:set` 开关
   - 新增 i18n 错误消息键
   - 创建 `agent/xml_stream_validator.go` + `agent/xml_stream_validator_test.go`
   - [BUILD-337] 补全 L2 语义验证：processOpenTag 增加未知工具名/未知参数名即时致命检测，processCloseTag 标签不匹配从警告改为致命（UC-0001/0002/0003/0010），修复跨 chunk 拆分时 `<cs:` 前缀丢失导致名称无法识别的问题，19 个验证器测试全部通过
   - [BUILD-337] FEATURE-293 兜底增强：hasIncompleteToolCall 识别任何带 `cs:` 前缀的标签为工具调用意图（即使名字不精确匹配），避免结构损坏的调用走 no-tool-action 直接退出

- [x] **FEATURE-299 :set defaults 命令 — 重置非关键配置到系统默认值**：[BUILD-336]
   - 命令行改名 `:set defaults`，在 `:set` 输出末尾显示提示信息
   - 实现 `:set defaults` 子命令，将除 LLM、记忆与上下文、数据库以外的所有配置重置为系统默认值
   - 修复 `{AGENT_PRINCIPLES}` 默认 i18n 回退，新增 `KeyAgentDefaultPrinciples` 资源
   - `:set` 显示新增 principles 参数和当前 mode 的 description
   - 英文 i18n 资源与中文对齐（`en_system.go` 改用 `{AGENT_PRINCIPLES}` 占位符）
   - `:set defaults` 后立即同步 agent 生效（`h.agent.SetConfig()`）
   - `rebuildSystemPrompt()` 中补充 `AgentPrinciples` 回退路径 debug 日志，便于排查 `:context` 中 principles 缺失问题

- [x] **FIX-300 `:set defaults` 时自动重命名 mode 留存目录，恢复默认系统提示词**：[BUILD-338]
  - 根因：`work/mode/{mode}/IDENTITY.md`（FEATURE-299 引入 `{AGENT_PRINCIPLES}` 占位符之前创建）缺少占位符，principles 被静默丢弃；用户对留存配置无感知
  - 影响：act mode 无自定义 IDENTITY.md → 回退 i18n 模板（含占位符）→ 正常；plan/research 等自定义 mode → principles 缺失
  - 修复：`handleSetDefault()` 在参数重置并保存 config.json 后，扫描运行目录 `{cwd}/mode/` 下与已知 mode 名（内置 act/plan/research + cfg.WorkModes 用户自定义）同名的目录，列出清单后经用户 y/n 确认，将目录重命名为 `原名.YYYYMMDD`（冲突追加序号），使系统提示词自然回退到 i18n 默认模板；重命名失败不阻断参数重置，仅汇总报告
  - 不影响 `agent/system_prompt.go` 构建逻辑，不误伤非 mode 名目录（如 act_old、claim）
  - 新增单元测试覆盖清单收集、冲突序号、自定义 mode、重命名成功/部分失败场景

---

## v0.5.1 — 补丁版

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-28
> **里程碑**: Bug 修复

### 功能清单

- [x] 在 ROADMAP.md 中新建 v0.5.1 版本计划段，更新版本发布记录表
- [x] 更新 main.go 版本常量从 v0.5.0-Beta2 改为 v0.5.1
- [x] FIX-190 增强循环检测能力：新增内容级循环检测（checkContentLoop），在每次收到 LLM 流式输出 chunk 时立即检测 accumulated 内容中是否存在重复文本块（整段重复、URL编码重复字符等），使用滑动窗口自动匹配最佳块大小，复用现有 threshold 参数 [BUILD-192]
- [x] FIX-190 补充：新增 attempt_completion 工具（function calling + XML 模式），修复用户消息模板未应用的问题 [BUILD-193]
- [x] FIX-191 修复 i18n 中 <task> 闭合标签错误，修复 {TASK} 只含纯指令而非格式化消息的问题（agent.go/run_stream.go/run.go/loop.go + i18n）

---

## v0.5.0 — Beta2（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-12
> **里程碑**: 功能完善，稳定可用


### 功能清单

- [x] FEATURE-91 任务计划（Task Plan / Checklist）系统 [BUILD-108]
- [x] FEATURE-82 对话上下文指系统改进：引入 messagePointer 机制，支持 LLM 在对话结束后能通过上下文指针来回移动，以在大量上下文空间中定位到上次继续的位置 [BUILD-113]
- [x] FEATURE-92 对话记忆保留机制：当 LLM 在迭代过程中自主完成任务时，在退出前的最后一次迭代中，应将 agent 迭代过程中的重要信息主动写入 memory（系统的 message 中会有类似的提示），以便能够在频繁切换对话主题时保持记忆连贯性。[BUILD-115]
- [x] FEATURE-93 工具调用历史追踪：LLM 每轮迭代的思考过程使用数字编号显示 [BUILD-116]
- [x] FEATURE-94 系统提示词 Size 优化：[BUILD-117]
  - 太长的一律放到最后，并且使用分隔符
  - 优化现有 i18n 键
- [x] FEATURE-95 会话记忆策略改进：在 Agent 每次收到 LLM 流式消息时，对工具调用结果消息设置 message_pointer，而用户消息、系统消息和助手消息不设 pointer [BUILD-118]
- [x] FEATURE-96 config.json 数据文件导出和导入 [BUILD-119]
- [x] FEATURE-97 Agent B 作为第二个智能体辅助主 Agent [BUILD-121]
- [x] FEATURE-98 ToolCall 消耗 Token 纳入 Token 统计 [BUILD-122]
- [x] FEATURE-99 新增 .settings_db 子命令管理数据库数据（key/data 增删改查）[BUILD-124]
- [x] FEATURE-100 PostgreSQL 数据库支持：支持持久化到 PostgreSQL，支持 SSL 连接、连接池（PGX 驱动），新增 .set db 配置和 .db 命令，config.json 持久化 [BUILD-125]
- [x] FEATURE-101 飞书集成 — co-shell-feishu-bridge 子项目 [BUILD-130]
  - co-shell-feishu-bridge 启动后自动运行一个 co-shell 子进程作为执行引擎，支持异步任务
  - 支持用户 `.card plan create/update/cancel` 创建/更新/取消任务计划卡片
  - 飞书消息处理集成
- [x] FEATURE-102 飞书被动响应增强：用户 `.deactive` 后 bridge 不再回复，`.active`后恢复正常 [BUILD-131]
- [x] FEATURE-103 飞书任务计划卡片自动更新：当 LLM 完成某个步骤或创建新计划时，卡片内容自动更新 [BUILD-132]
- [x] FEATURE-104 co-shell-feishu-bridge 长期记忆保持：关闭 co-shell-bridge 后恢复时保持先前记忆 [BUILD-149]
- [x] FEATURE-105 飞书用户多轮对话支持：bridge 将同一飞书用户的多次提问串联成同一对话上下文 [BUILD-149]
- [x] FEATURE-106 新工具支持：list_code_definition_names（列出代码定义名称）[BUILD-112]
- [x] FEATURE-107 新工具支持：PDF 文件读取工具 [BUILD-113]
- [x] FEATURE-108 新工具支持：access_mcp_resource [BUILD-119]
- [x] FEATURE-109 新工具支持：use_mcp_tool [BUILD-118]
- [x] FEATURE-110 增强 i18n/memory 回读在 context 中的可读性 [BUILD-119]
- [x] FEATURE-111 i18n Key 管理和翻译规范化 [BUILD-120]
- [x] FEATURE-112 Agent B 子命令增强：自动清理 Agent B 和主 Agent 历史（只在 agent_b 模块中处理 Agent B 的会话消息）；子 Agent 提示词增加 目标任务的描述（来自主 Agent 任务描述）[BUILD-123]
- [x] FEATURE-113 co-shell 设置向导支持 Tab / 数字快速选择供应商 [BUILD-127]
- [x] FEATURE-114 内置命令 Tab 补全增强：Tab 显示全部补全列表 [BUILD-128]
- [x] FEATURE-115 Agent B 快速任务/对话功能增强 [BUILD-133]
- [x] FEATURE-116 co-shell-bridge 启动效率优化：[BUILD-138]
  - 不启动 UserIO（用于 REPL）
  - 不启动 VT（不需要 LP 级别的 Shell 会话支持）
  - 不启动 REPL 输入历史
  - 不启动 Scheduler（不需要定时任务调度）
  - 不启动 Browser（不需要浏览器自动化功能）
  - 不启动自动升级检查
  - LLM 启动方式改为流式输出（但 bridge 模式下不启用增强输入）
  - 日志打印到 log/ 目录
  - 按更合理的初始化步骤启动 engine
  - 系统提示词中删除 Shell Session、Browser、Task Plan 相关的内容
- [x] FEATURE-117 子 Agent 新增 {TASK} 模板变量和指令增强：[BUILD-139]
  - 子 Agent 启动时自动从主 Agent 上下文获取完整消息列表
  - {TASK} 变量自动展开为子 Agent 格式化任务描述
  - 指令提示词统一（含子 Agent 规则、任务描述、系统提示词规则）
  - 工具列表过滤：只保留核心工具
  - 子 Agent 对话上下文格式改进
  - 子 Agent 支持截图
- [x] FEATURE-118 子 Agent 运行时自动清理消息上下文中的 system tool_result 以避免上下文膨胀：在子 Agent 运行开始和结束时，从消息历史中移除 system 角色的 tool_result 消息 [BUILD-139]
- [x] FEATURE-119 子 Agent 运行支持 TaskPlan（任务计划）：主 Agent 通过 `launch_sub_agent_with_plan` 可同时提交子 Agent 任务计划和指令，子 Agent 会自动使用 taskplan 工具管理计划内任务 [BUILD-140]
- [x] FEATURE-120 Agent 流式输出提取并替换 ${...} 变量 [BUILD-141]
- [x] FEATURE-121 Agent 改进：调整 system prompt 顺序、agent_name 作为程序名称传递给子 Agent、子 Agent 在过程中不再使用 agent_b_xxx 工具 [BUILD-141]
- [x] FEATURE-122 子 Agent 改进：限制子 Agent 的 tool_result 消息数（最多 5 个）、确保子 Agent 退出的消息写入主 Agent 记忆 [BUILD-141]
- [x] FEATURE-123 重复工具调用处理：增强消息裁剪合并逻辑，对相邻的同一工具 tool_use+tool_result 合并为一条 assistant 消息 [BUILD-142]
- [x] FEATURE-124 避免无限循环：agent 使用 LLM 工具返回后，进行工具使用结果去重处理：如果当前工具调用结果与前一次完全相同，则记录去重次数（第 N 次相同结果），并截断上下文（删除当前和前一轮迭代的助手消息和工具结果消息），以便 LLM 改变策略 [BUILD-143]
- [x] FEATURE-125 Agent 新增 `GetAgentName()` 方法 [BUILD-146]
- [x] FEATURE-126 .context compact 压缩时移除所有 tool_result 消息 [BUILD-147]
- [x] FEATURE-127 工作空间持久化配置：`.set workspace` 保存工作空间路径、`--workspace` 命令行参数 【BUILD-149】
- [x] FEATURE-128 Agent B 改进：Agent B 执行结果自动添加到主 Agent 内存中，Agent B 不包含工具调用结果的上下文信息 【BUILD-149】
- [x] FEATURE-129 .rule 命令的 [ 和 ] 键上下移动规则，r 重命名规则，w 切换启用/禁用，d 删除 【BUILD-150】
- [x] FEATURE-130 .rule 命令改进：列表左侧显示规则编号，编号按最长数字右对齐，启用/禁用规则后切换到同一页 【BUILD-151】
- [x] FEATURE-131 Taskplan 创建后消息指针自动调整到新计划之后，以便 LLM 能"看到"刚创建的任务计划 【BUILD-152】
- [x] FEATURE-132 `.plan` 命令交互式管理任务计划：支持通过上下键选择步骤、Enter 切换状态、+/- 调整位置等 【BUILD-152】
- [x] FEATURE-133 `.plan` 子命令增强：`.plan create` 交互创建 [BUILD-153]；`.plan remove` 交互式选择要删除的步骤 [BUILD-154]；`.plan insert` 交互式插入步骤 [BUILD-154]
- [x] FEATURE-134 `.plan` 步骤显示编号和控制键改进：步骤按使用顺序排列（步骤编码和子步骤），新增 v 查看已完成步骤，n 显示步骤备注，c 切换步骤完成状态，快捷键摘要显示在列表底部 【BUILD-155】
- [x] FEATURE-135 启动时 co-shell 自动根据当前工作空间通过 `.plan` 命令检查是否有未完成的任务计划 【BUILD-156】
- [x] FEATURE-136 `.plan` 编辑模式增强：新增 e 键编辑模式 —— 在任何步骤上按 e 进入编辑模式，按 a 添加步骤、d 删除、i 插入、e 编辑描述、m 编辑备注、←/→ 或 Ctrl+←/Ctrl+→ 移动步骤，编辑完成后按 ESC 预览并确认、Enter 直接保存 【BUILD-157】
- [x] FEATURE-137 任务计划移除步骤支持范围删除：`.plan remove` 参数支持单个、空格分隔的多个、横线范围、混合格式，如 `1 3-5 7`（删除 1,3,4,5,7）【BUILD-158】
- [x] FEATURE-138 `.plan` 增加 steps 子命令列出详细步骤 【BUILD-159】
- [x] FEATURE-139 启动时如果有未完成的任务计划，co-shell 自动将计划详情发送给 LLM 作为上下文，并提示用户是否继续 【BUILD-160】
- [x] FEATURE-140 Agent 系统提示词 TaskPlan 节增强：增加创建新计划时说明、示例和规则——1) Plan-Act-Reflect 方法论、2) 渐进式计划=简单开始+逐步调整、3) 步骤变动时更新计划  【BUILD-161】
- [x] FEATURE-141 `.plan` 交互界面改进：增加 `m` 键调整步骤顺序，选中步骤后移动到指定位置（手动输入目标位置） 【BUILD-162】
- [x] FEATURE-142 `.plan` 编辑模式增加 `v` 键还原被删除步骤：在 e 编辑模式下，按 v 显示最近一次删除的步骤列表，勾选需要还原的步骤后恢复到原位 【BUILD-163】
- [x] FEATURE-143 `.plan` 编辑模式增加 `i` / `a` 插入步骤时选择插入到选中步骤前/后：在 e 编辑模式下选中某步按 i 后弹窗让选择插入位置（之前/之后） 【BUILD-163】
- [x] FEATURE-144 `.plan` 列表显示所有子命令 【BUILD-163】
- [x] FEATURE-145 `.plan` e 编辑模式增加 `t` 键修改步骤状态：当前状态高亮显示，上下键选择，Enter 确定 【BUILD-164】
- [x] FEATURE-146 Browser Tool 改进：browser_screenshot 参数形式配置、`.browser` 命令改进 【BUILD-165】
- [x] FEATURE-147 子 Agent 执行时长信息反馈：【BUILD-166】
  - Agent 状态显示阶段增加秒数显示（思考 / 使用工具 / 工具已完成）
  - Agent B 执行完成后返回耗时信息
  - 子 Agent 执行完成后返回耗时信息
- [x] FEATURE-148 任务计划调整触发记忆归档：【BUILD-167】
  - 当使用 remove_steps 或 insert_steps 调整任务计划时，如果最终原计划步骤全部被清空，应归档旧计划（作为已取消）并删除空计划，允许 LLM 创建全新计划
  - 当使用 update_step_status 更新所有步骤状态为 completed 时，也应归档当前计划作为已完成，删除空计划，允许创建新计划
  - 归档的记忆内容包含当前进展（进度百分比）
- [x] FEATURE-149 .plan remove 命令增强：如果在交互式删除中删除了所有步骤，整个计划归档到记忆并删除，并提示用户可以建新计划了 【BUILD-168】
- [x] FEATURE-150 新工具：use_subagents（启动多个子 Agent 并行执行）【BUILD-168】
- [x] FEATURE-151 .rule 命令新增 rename 子命令 【BUILD-169】
- [x] FEATURE-152 工具定义名称统一：所有工具保留原始下划线名称（如 execute_command），移除描述中的 `（execute_command）` 后缀和冒号前缀，工具定义聚焦 param 而不是 description 【BUILD-170】
- [x] FEATURE-153 支持的供应商列表中加入 qwen-turbo-2026-01-19 【BUILD-170】
- [x] FEATURE-154 关闭 shell 会话时自动清理 VT 终端窗口内容 【BUILD-171】
- [x] FEATURE-155 消息去重改进：sendConversationUpdate 独立为方法、延迟调用（500ms 防抖）、消息对象含 time+text+isLoading 三字段、isLoading 变化时立即更新、仅 isLoaded 状态才加入 history 【BUILD-171】
- [x] FEATURE-156 任务计划 Step 备注显示：在 `.plan` 列表和 `remove` / `insert` / `view` 命令输出中显示步骤备注 【BUILD-172】
- [x] FEATURE-157 Agent 检查配置文件错误并提示：【BUILD-172】
  - 启动时检查 config.json 中是否包含无法识别的 key，如果有则打印警告并列出
- [x] FEATURE-158 工作空间改进：启动时检查 config.json 中的 workspace 是否变更，如变更则清理旧工作空间的记忆列表并初始化新 workspace 的 DB；`work/` 目录在 workspace 下创建 【BUILD-173】
- [x] FEATURE-159 REPL 增强：支持多行输入（`\` 行续接 + `{ }` 自动续接），LLM 调用按 ESC 后自动暂停并显示暂停后的输出，按任意键继续或 ESC 再次退出 【BUILD-173】
- [x] FEATURE-160 上下文压缩改进（compileContext）：在达到最大 token 数量时，优先压缩 tool_result 工具调用结果消息【BUILD-173】
- [x] FEATURE-161 多行输入时行号提示改变：第一行提示符显示 > ，续行提示符显示 >> 【BUILD-174】
- [x] FEATURE-162 命令执行结果中出现的图片 URL 自动下载到本地：【BUILD-174】
  - 提取命令输出中的图片 URL（如 ![alt](url)、<img src="url"> 或纯 URL）
  - 下载为 PNG/JPEG/GIF/WebP 格式到 workspace/{image-dir} 目录
  - 在命令输出中替换 URL 为本地路径
- [x] FEATURE-163 命令 `execute_command` 输出中的附带文件收集：【BUILD-174】
  - 当执行 shell 命令后，如果输出中包含 `📄` 文件标记，则将对应的文件内容一并返给 LLM
- [x] FEATURE-164 Chrome 直接下载图片 【BUILD-175】
- [x] FEATURE-165 代码搜索工具 search_files 增加 file_pattern 参数，支持限定文件扩展名或文件名模式 【BUILD-175】
- [x] FEATURE-166 上下文管理优化：system prompt 中的 ROADMAP 只保留当前版本的任务列表，已完成和未开始的版本只保留标题，不再展开任务项 【BUILD-176】
- [x] FEATURE-167 工具调用中文件写入类工具（write_to_file / replace_in_file）在返回结果时显示文件大小（bytes），并在系统提示词中指导 LLM 注意监控文件大小，防止产生过大文件 【BUILD-176】
- [x] FEATURE-168 新工具：image_tools（图片分析工具），用于 LLM 分析图片文件，支持 URL 和本地路径，返回 Base64 编码图片数据 【BUILD-176】
- [x] FEATURE-169 新工具：rename_file 工具，用于重命名文件 【BUILD-176】
- [x] FEATURE-170 子 Agent 改进：从主 Agent 继承工具调用模式、confirm-tool 设置等配置参数 【BUILD-176】
- [x] FEATURE-171 任务计划：支持 LLM 在步骤中直接完成（completed）状态标记，并自动归档已完成任务计划到记忆 【BUILD-176】
- [x] FEATURE-172 tool_use 列表去重，仅保留最近一条：当 LLM 连续两次返回完全相同的 tool_use（函数名称和参数完全相同）时，仅保留最近一次调用，删掉前一次 【BUILD-177】
- [x] ENHANCEMENT-173 RUN 流式输出工具调用和结果显示改进：【BUILD-178】
  - LLM 返回 tool_use 时即显示 `[🔧] 工具名称(参数...)` 并立即执行
  - 工具调用执行完成显示 `[🔧] 工具调用结果` + 结果摘要
  - 工具执行过程中无 LLM 思考步骤也无额外空行
  - 所有输出归流式输出的 callback（非工具函数内 print）
  - 首次调用前和末次调用后各输出一个分隔线
- [x] ENHANCEMENT-174 工具调用确认序号改进：序号改为每次任务会话独立，agent 初始化时重置为 0，工具调用确认对话中的序号现在持续增长且不会再滚动回 0 【BUILD-178】
- [x] ENHANCEMENT-175 循环检测优化：消息历史 tokens 整数溢出改为 uint64 防止溢出；新增 content-level 循环检测（checkContentLoop）【BUILD-178】
- [x] ENHANCEMENT-176 系统提示词优化：移除重复的 "Objective" 节，移除 PromptSection 和 WorkMode 内置节中的重复节 【BUILD-178】
- [x] ENHANCEMENT-177 工具调用确认序号显示优化：序号改为：实例上所有 agent 共享的全局序号的最后一个数字。【BUILD-178】
- [x] ENHANCEMENT-178 修改对话上下文时间戳前缀格式：从 "2026-05-12 10:15:30 - " 改为 "在 2026-05-12 10:15:30 说："，提升用户可读性。[BUILD-187]
- [x] FIX-97 修正qwen3.6在遇到写大文件时LLM输出无限循环的问题：[BUILD-177]
  - 在流式输出处增加每种event的日志输出（event类型、当前块内容），证明循环不是程序直接造成的
  - 修复 REPL 输出格式：command/tool_call 提示符从行首开始显示
  - 修复 loop.go：带 tool_calls 的 assistant 消息去掉时间戳前缀和索引前缀
- [x] FIX-180 修复 LLM 调用 write_to_file 时缺少 content 参数导致死循环的问题：增强工具参数描述明确性、改进错误反馈格式，帮助 LLM 更快理解并修正参数缺失问题。[BUILD-177]
- [x] FIX-181 修正qwen3.6在遇到写大文件时LLM输出无限循环的问题：[BUILD-181]
  - 在流式输出处增加每种event的日志输出（event类型、当前块内容），证明循环不是程序直接造成的
  - 新增 log.Raw 方法用于 SSE 原始数据流追踪
  - 增强 LoopDetector 单词重复检测模式（同一单词重复、交替两词模式）
  - 完善 streamLLMResponse debug 日志
  - 新增 docs/system-prompt-composition.md system prompt 组成文档
- [x] FEATURE-182 可配置的分模式自定义灵活的toolcall调用机制，先支持当前的标准OpenAI API tool call，增加实现类Cline XML式，并且可由用户通过配置文件自定义其他扩展模式，以便co-shell能够支持更多不同能力和工具调用标准的LLM，同时能够尝试避免一些LLM调用死循环的问题。[BUILD-187]
- [x] FIX-183 修复 .model add/switch 后模型切换不生效的问题：ModelManager 与 cfg.Models 双列表不同步导致 selectModelForCall 仍使用旧模型。在 cmd/model.go 的 saveModel/switchModel/removeModel/enableModel/disableModel/setPriority 方法中增加 modelMgr 同步逻辑。[BUILD-182]
- [x] FEATURE-184 工具调用确认机制扩展：1) 将所有工具调用（不限于 execute_command）都增加用户确认；2) 每个工具增加 confirm-tool 控制参数，可通过命令行、REPL、参数文件设置，默认需要用户确认；3) 用户确认时增加 G 选项（同意，且取消此方法需用户确认标志）；4) 数字计数器改为每个方法独立，当前任务结束后全部清 0。[BUILD-187]
- [x] FEATURE-86 支持永久记忆接入数据库。1）基于PostgreSQL进行设计；2）提供数据库连接配置方法，尽量简化数据库配置，仅提供地址（默认本地）、端口（数据库默认）、库名（默认为coshell_db，或其一个更好的名字）、schema使用默认public；3）如果配置数据库后，默认连接数据库，如果不通，则提示用户后依然使用本地库（不要因为远端数据库无法连接影响整体使用）。[BUILD-183]
- [x] FEATURE-185 程序默认使用当前工作目录名（最后一层）作为 agent name。用户依旧可以通过已经实现的命令行参数、REPL、配置文件配置。[BUILD-184]
- [x] FEATURE-186 数据库配置改为子命令模式：`.set db enabled on`、`.set db host 10.0.0.1` 等，模仿 confirm-tool 方式做成配置子集。DB 配置显示移到「记忆与上下文」分组中。新增交互式配置向导，首次运行 `.set db` 时自动引导用户完成 PostgreSQL 连接配置，支持连接测试和 bbolt 数据迁移。[BUILD-186]
- [x] FEATURE-187 改进 .model 子命令交互体验：switch/remove/enable/disable/info/set-priority/set-param 等需要选择模型的命令，当不提供模型 ID 参数时，先显示模型列表让用户通过编号选择，提升易用性。[BUILD-188]
- [x] ENHANCEMENT-188 XML 模式数组参数统一使用 `<item>` 标签：将 parseXMLChildrenToJSON/buildXMLToolDescription/buildXMLToolPrompt 中的 element 统一改为 item；修复 i18n 系统提示词中反引号导致 raw string literal 提前结束的问题；同步更新中英文示例。[BUILD-190]
- [x] FIX-189 修复 API URL 自动拼写时，如果 base URL 中已经有 /v1 后缀，不再重复添加 /v1。[BUILD-191]

---

## v0.4.0 — 发布候选版 RC2（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-03


### 功能清单

- [x] FEATURE-33 模型列表管理改进 [BUILD-6]
- [x] FEATURE-70 Agent 功能增强：
  - 对话上下文可视化 [BUILD-4]
  - 无限迭代循环控制 [BUILD-4]
  - 消息指针调整 [BUILD-5]
  - 大量文件写入支持 [BUILD-5]
  - 上下文清理 [BUILD-8]
  - 大小写不敏感的命令匹配 [BUILD-9]
- [x] FEATURE-71 配置管理改进：
  - 配置文件位置改进 [BUILD-4]
  - 配置热加载改进 [BUILD-4]
  - 多个配置合并 [BUILD-6]
- [x] FEATURE-72 REPL 改进：
  - ESC 取消 [BUILD-4]
  - 命令提示符 [BUILD-9]
- [x] FEATURE-73 MCP Server 增强：
  - MCP Server STDIO 模式、超时 [BUILD-4]
  - .mcp man 命令支持 [BUILD-7]
- [x] FEATURE-74 日志改进：
  - 日志级别过滤 [BUILD-4]
  - 结构化日志 [BUILD-6]
  - 错误堆栈追踪 [BUILD-6]
- [x] FEATURE-75 文件工具增强：
  - write_to_file 写入确认 [BUILD-5]
  - 文件搜索改进 [BUILD-5]
  - 文件写入大小验证 [BUILD-6]
  - 大文件写入优化 [BUILD-7]
- [x] FEATURE-76 上下文管理改进：
  - 上下文历史导入 [BUILD-5]
  - 上下文文本查看 [BUILD-5]
  - 自动上下文清理 [BUILD-8]
  - 上下文管理错误处理改进 [BUILD-8]
- [x] FEATURE-77 i18n 改进：
  - 系统提示词同步 [BUILD-6]
  - 用户消息模板 [BUILD-6]
  - 系统提示词优化 [BUILD-7]
  - 错误信息国际化 [BUILD-8]
- [x] FEATURE-78 设置向导改进：
  - 向导退出优化 [BUILD-7]
  - 供应商选择添加 [BUILD-7]
- [x] FEATURE-79 安全改进：
  - 输出安全过滤 [BUILD-8]
  - 文件路径安全验证 [BUILD-8]
- [x] FEATURE-80 循环检测优化：
  - LLM 调用循环自动检测和自修复 [BUILD-9]
  - 循环检测阈值可配置 [BUILD-9]
- [x] FEATURE-81 消息管理改进：
  - 用户消息中添加 {OS}、{SHELL}、{WORKSPACE}、{LOCALE} 模板变量 [BUILD-97]
  - 用户消息中添加 {TOOL_RESULT_MODE} 模板变量 [BUILD-97]
- [x] ENHANCEMENT-83 优化 REPL 交互体验：错误信息红色显示，调整消息裁剪阈值为对话使用量（非总量），限制消息裁剪后仍保留系统提示词 [BUILD-97]
- [x] FEATURE-84 工具调用执行流程改进：Agent 工具执行失败时，向 LLM 返回结构化错误信息，帮助 LLM 理解并修正 [BUILD-97]
- [x] FEATURE-85 新的确认模式和文件写入确认改进：[BUILD-107]
  - 实现三种确认模式：all（全部确认）、custom（选择性确认）、off（不确认）
  - 默认模式为 custom
  - 新的 ESL (Enhanced Selection List) 组件用于交互式选择
  - 文件写入确认对话框
- [x] FEATURE-86 持久的 memory 对比上下文：[BUILD-105]
  - 独立的记忆列表
  - 记忆自动过期
  - 记忆归档
  - 记忆搜索
- [x] FEATURE-87 子 Agent 改进：[BUILD-108]
  - 完整的子 Agent 工具列表
  - 可配超时时间
  - UI 提示
  - 任务编号
  - 日志
  - 支持截图
- [x] FEATURE-88 文件和系统工具的完善：[BUILD-108]
  - 文件追加工具（append_to_file）
  - 目录创建工具（create_directory）
  - 文件移动工具（move_file）
  - 文件复制工具（copy_file）
  - 文件删除工具（delete_file）
  - 文件重命名工具（rename_file）
- [x] FEATURE-89 工具调用返回信息改进：工具定义增加最佳实践说明 [BUILD-108]
- [x] FEATURE-90 子 Agent 新增替代主要 LLM 的功能：可配置子 Agent 使用的模型 [BUILD-110]

---

## v0.3.0 — 发布候选版（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-29


### 功能清单

- [x] FEATURE-64 消息管理：消息上下文 Base64 编码存储和读取 [BUILD-3]
- [x] FEATURE-65 工具调用管理：工具调用结果模式改进 [BUILD-3]
- [x] FEATURE-66 Agent 功能增强：
  - 支持多轮工具调用 [BUILD-3]
  - 增加 write_to_file 的写入大小限制 [BUILD-3]
  - 增加 execute_command 的确认机制 [BUILD-3]
- [x] FEATURE-67 系统消息管理：详细的系统提示词，包含当前配置、工具定义、角色设定等 [BUILD-3]
- [x] FEATURE-68 配置管理增强：
  - 断连后自动重连 [BUILD-3]
  - 系统提示词头部信息 [BUILD-3]
  - 最大迭代次数限制 [BUILD-3]
- [x] FEATURE-69 会话管理改进：
  - 会话消息指针移动 [BUILD-3]
  - 无限迭代默认使用 [BUILD-3]
  - 历史记录切换 [BUILD-3]

---

## v0.2.0 — Beta（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-27


### 功能清单

- [x] FEATURE-58 REPL 功能增强：支持联想记忆、历史记录 [BUILD-2]
- [x] FEATURE-59 Agent 核心流程优化：支持消息裁剪、上下文窗口管理 [BUILD-2]
- [x] FEATURE-60 工具调用增强：支持文件写入确认、工具调用链 [BUILD-2]
- [x] FEATURE-61 跨平台构建：支持 Linux/macOS/Windows [BUILD-2]
- [x] FEATURE-62 指令模板：支持用户自定义指令模板，.prompt 命令管理，持久化 [BUILD-2]
- [x] FEATURE-63 Shell 集成：co-shell 默认用最后一级目录名为 agent name [BUILD-2]

---

## v0.1.0 — Alpha（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-26


### 功能清单

- [x] FEATURE-1 REPL 交互界面（go-prompt，Tab 补全）[BUILD-1]
- [x] FEATURE-2 LLM 客户端抽象（OpenAI 兼容 API）[BUILD-1]
- [x] FEATURE-3 Agent 核心循环（LLM 调用 → 工具执行 → 迭代）[BUILD-1]
- [x] FEATURE-4 内置命令系统（.settings / .mcp / .rule / .memory / .context）[BUILD-1]
- [x] FEATURE-5 持久化存储（bbolt 记忆/上下文）[BUILD-1]
- [x] FEATURE-6 MCP 客户端管理器（多 Server 连接）[BUILD-1]
- [x] FEATURE-7 系统命令执行（超时控制）[BUILD-1]
- [x] FEATURE-8 配置管理（JSON 持久化到 ~/.co-shell/）[BUILD-1]
- [x] FEATURE-9 API初始设置（默认设置为deepseek，但Key留空）[BUILD-1]
- [x] FEATURE-10 API设置向导（co-shell启动后当系统大模型API参数不完整时，提示用户输入不完整的参数，比如默认deepseek配置不带key，那么就先提示用户输入正确的key并需要至测试成功为止）[BUILD-1]
- [x] FEATURE-11 系统命令直接运行（如果用户直接输入系统命令或执行程序在当前环境下可以直接执行，则直接执行用户的所有内容，而不用通过大模型解释。）[BUILD-1]
- [x] FEATURE-12 流式输出支持 [BUILD-1]
- [x] FEATURE-13 日志系统（文件日志，支持运行时开关）[BUILD-24]
- [x] FEATURE-14 API Key 脱敏显示 [BUILD-24]
- [x] FEATURE-15 命令行参数支持（--help/--version/--model/--endpoint/--api-key/--log）[BUILD-24]
- [x] FEATURE-16 命令行指令支持（-c/--cmd 执行单条自然语言或系统指令后退出）[BUILD-24]
- [x] FEATURE-18 会话历史管理（用户可以通过上、下键在自己输入的历史内容之间翻页，包括上一次执行co-shell时的内容）[BUILD-25]
- [x] FEATURE-19 基础错误处理和用户提示 [BUILD-25]
- [x] FEATURE-20 最大迭代次数可配置（--max-iterations 命令行参数、.settings max-iterations 运行时设置、config.json 持久化）[BUILD-27]
- [x] FEATURE-21 多配置文件位置支持（优先级：命令行参数指定 > 当前目录 config.json > ~/.co-shell/config.json）[BUILD-28]
- [x] FEATURE-22 多供应商支持（DeepSeek v4 / 阿里千问 / OpenAI 兼容兜底），设置向导支持供应商选择、自动打开官网获取 API Key [BUILD-29]
- [x] ENHANCEMENT-23 设置向导增强：Tab 键显示可选列表、上下键选择、ESC 退出、连接测试 [BUILD-31]
- [x] ENHANCEMENT-24 设置向导增强：OpenAI 兼容模式下输入端点后自动测试连通性，输入 API Key 后自动获取模型列表 [BUILD-32]
- [x] FEATURE-47 国际化（i18n）支持中文/英文，--lang 命令行参数，自动检测系统语言 [BUILD-33]
- [x] FEATURE-48 全局规则（--rule 命令行参数 / .rule 命令管理 / rule.md 自动加载） [BUILD-34]
- [x] ENHANCEMENT-50 配置文件增加注释说明 [BUILD-34]
- [x] FEATURE-51 命令行增强：.list tools/tasks 列出 LLM 工具和任务计划，.list commands 列出内置命令 [BUILD-45]
- [x] FEATURE-25 日志级别可配置：debug / info / warn / error / off [BUILD-36]
- [x] FEATURE-52 XML 输出格式支持（--xml / 模式切换），增强配置向导支持 XML 模式 [BUILD-42]
- [x] FEATURE-26 设置命令增强：新的设置键 confirm-file 等。.settings 子命令支持 Tab 自动补全，子命令统一化 [BUILD-47]
- [x] FEATURE-27 子 Agent 机制（快速执行独立任务而不阻塞主 Agent 循环） [BUILD-48]
- [x] FEATURE-28 图片理解支持，通过 --image 参数传入图片路径来读取图片，支持多图片，支持多模态 VLM [BUILD-49]
- [x] FEATURE-29 工具调用结果模式配置（result-mode，默认 full，支持 minimal） [BUILD-54]
- [x] FEATURE-30 .mcp 子命令增强：restart/remove/rename/add/man [BUILD-60]
- [x] FEATURE-32 多 Agent 支持（--agent 指定名称，配置独立） [BUILD-61]
- [x] FEATURE-33 .model 子命令管理模型列表 [BUILD-62]
- [x] FEATURE-46 输出格式化增强：支持 Markdown 渲染（-format markdown）和表格渲染 [BUILD-63]
- [x] FEATURE-37 配置热重载 [BUILD-65]
- [x] FEATURE-36 上下文管理（.context 命令）[BUILD-66]
- [x] FEATURE-39 REPl 持久化历史记录 [BUILD-67]
- [x] FEATURE-53 Tool/Agent 模式切换 [BUILD-68]
- [x] FEATURE-54 输出增强：代码块 Markdown 渲染 [BUILD-68]
- [x] FEATURE-55 上下文分支管理 [BUILD-69]
- [x] FEATURE-38 文件读取工具增强：支持 read_file start_line/end_line [BUILD-70]
- [x] FEATURE-39 代码搜索工具：search_files [BUILD-71]
- [x] FEATURE-40 基于关键字的记忆系统 [BUILD-72]
- [x] FEATURE-41 记忆自动总结 [BUILD-73]
- [x] FEATURE-42 交互体验优化：LLM 调用阶段显示当前阶段（思考/使用工具）[BUILD-74]
- [x] FEATURE-43 自动上下文清除：[BUILD-79]
- [x] FEATURE-44 增强显示：支持 LLM 思考和推理过程显示 [BUILD-80]
- [x] FEATURE-29 Toolcall 模式扩展 [BUILD-92]

---

## v1.0.0 — 正式版

> **状态**: 💡 构想中
> **目标日期**: 2026-07-01
> **里程碑**: 稳定可用，可发布


### 功能清单

- [ ] FEATURE-130 为co-shell正式发布中文名，以便中国用户能够记住并且具有亲和力，有利于推广。
- [ ] FEATURE-31 MCP Server 自动重连
- [ ] FEATURE-34 插件系统（WASM 插件支持）
- [ ] FEATURE-35 自定义 Prompt 模板
- [ ] FEATURE-37 输出格式化（JSON/表格/树形）
- [ ] FEATURE-38 命令别名
- [ ] FEATURE-40 管道支持（Pipe）
- [ ] FEATURE-56 MCP Hub 集成（发现和安装 MCP Server）
- [ ] FEATURE-57 社区插件市场
- [ ] FEATURE-58 多 Agent 协作
- [ ] FEATURE-59 可视化工作流编排
- [ ] FEATURE-60 远程执行（SSH）

### 优化

- [ ] ENHANCEMENT-41 启动速度优化
- [ ] ENHANCEMENT-42 内存使用优化
- [ ] ENHANCEMENT-43 大模型响应缓存

---

## 版本发布记录

| 版本 | 日期 | 状态 | 说明 |
|---|---|---|---|---|---|
| v0.1.0 | 2026-04-25 | ✅ 已完成 | Alpha 预览版 |
| v0.2.0 | 2026-04-27 | ✅ 已完成 | Beta 测试版 |
| v0.3.0 | 2026-04-29 | ✅ 已完成 | 发布候选版 |
| v0.4.0 | 2026-05-03 | ✅ 已完成 | 发布候选版 RC2 |
| v0.5.0 | 2026-05-12 | ✅ 已完成 | Beta2 测试版 |
| v0.5.1 | 2026-05-28 | ✅ 已完成 | 补丁版 |
| v0.6.0 | 2026-06-01 | ✅ 已完成 | Beta3 测试版 |
| v0.7.0 | 2026-08-01 | 🚧 开发中 | 输出架构重构里程碑 |
| v0.7.1 | 2026-08-05 | 🚧 开发中 | 循环检测优化（误判修复+judge 提示词+报警类型区分） |
| v0.7.2 | 2026-08-06 | 🚧 开发中 | 工作区配置外部化（PRINCIPLES.md + .rules/ + .rule 生效修复） |
| v0.7.3 | 2026-08-07 | ✅ 已完成 | :context 显示增强（tool_calls 块 + 控制字符 + retried_count + full 模式） |
| v0.7.4 | 2026-08-08 | 🚧 开发中 | 问题判定优化（统一问题判定机制 + report_problem 工具） |
| v0.7.5 | 2026-08-10 | 🚧 开发中 | 视觉识别上下文隔离（minimal 识别轮独立 + 结果回填） |
| v0.7.6 | 2026-08-10 | ✅ 已完成 | browser_screenshot 视觉识别一致化 + 循环介入 auto 策略 |
| v0.7.7 | 2026-08-16 | 🚧 开发中 | 输入统一（InputSource）+ Windows 补齐（FEATURE-306 + FIX-350） |
| v0.8.0 | 2026-08-21 | ✅ 已完成 | web UI 状态栏会话菜单（FEATURE-387） |
| v0.9.0 | 2026-08-21 | ✅ 已完成 | 工具调用交互标准化（FEATURE-388） |
| v0.9.1 | 2026-08-22 | 🚧 开发中 | Web UI 会话切换上下文修复（FIX-407） |
| v0.19.0 | 2026-08-27 | 🚧 开发中 | Web UI 文件预览/图片查看改进（FEATURE-444） |
| v1.0.0 | 2026-07-01 | 💡 构想中 | 正式版 |


### 发布条件

每个版本发布前需满足以下条件：

- [ ] `go build ./...` 编译通过
- [ ] `go vet ./...` 无警告
- [ ] 核心功能手动测试通过
- [ ] USAGE.md 使用文档完整
- [ ] CHANGELOG.md 更新

---

## 版本命名规范

```
v{major}.{minor}.{patch}
  │       │       └── 补丁版本：Bug 修复、小改进
  │       └────────── 次版本：新功能、非破坏性变更
  └────────────────── 主版本：重大变更、不兼容更新
```

### 状态标签

| 标签 | 含义 |
|---|---|
| 💡 构想中 | 初步想法，尚未开始 |
| 📋 规划中 | 已确定计划，待开发 |
| 🚧 开发中 | 正在开发 |
| ✅ 已完成 | 开发完成 |
| 🚀 已发布 | 正式发布 |

### 编号前缀说明

| 前缀 | 含义 |
|---|---|
| FEATURE- | 新特性或改进（New Feature/Enhancement） |
| FIX- | Bug 修复（Bug Fix） |

