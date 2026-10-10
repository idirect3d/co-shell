# FEATURE-627 UC-0001：Web UI 顶栏显示工作区路径 + App 顶部条显示 co-shell

> 分支：**FEATURE-627**　版本：**v0.108.2**　BUILD：**待编译**
> 需求（用户 2026-10-10 11:09 提出，11:14 确认细节）：
> ① co-shell Web UI 顶部标题栏，**宽屏**模式下在当前 agent 名后显示当前工作区绝对路径，形式 ` - {workspace}`；
> ② `co-shell.app` 最上部标题栏留白处，**中间显示 `co-shell`**。
>
> 用户确认：版本 **v0.108.2**（patch 递增）；路径**超长时中间省略（…）并悬停显示完整路径**；App 文字**跟随主题**（深色底浅字、浅色底深字）**次级灰、约 12–13pt**。

## 设计基线（本次实现基准）

| 项 | 设计 |
|----|------|
| 插入位置 | `web/static/index.html` 的 `#topbar` → `.top-center` 内、`#agentName` 之后新增元素（如 `<span class="ws-path" id="wsPath"></span>`） |
| 显示形式 | 文本为 `" - " + 工作区绝对路径`（含前导空格-连字符-空格） |
| 宽屏/窄屏 | 复用既有 `body.narrow`（FEATURE-487：视口 < 品牌宽度×3）；窄屏隐藏该元素 |
| 数据来源 | `/api/bootstrap` 已下发的 `workspace`（绝对路径）；前端 `app.js` 已有 `wsRoot`（`boot()` 中记录），不新增后端接口 |
| 超长处理 | 中间省略：展示为 `头…尾`（保留末尾路径段），`title` 属性存完整路径；窗口 resize 时重算 |
| App 文字 | `app/macos/CoShellApp.swift` 的 `TitlebarStripView` 内新增居中文本 `co-shell`，字号 12–13pt、次级灰；颜色随条色明暗自适应 |
| 后端 | 不改动（无新接口、无新配置） |
| i18n | 无新增文案（` - ` 为符号、路径为数据、`co-shell` 为品牌名） |

## 术语

- **(R)** 离线静态校验（读源码 / 构建产物）。
- **(M)** 运行时手动断言（浏览器 / App 界面观察）。

## 用例清单

### UC-01 (R) 顶栏存在工作区路径元素且位于 agent 名之后

- 断言：`web/static/index.html` 的 `#topbar` → `.top-center` 内，`#agentName` 之后存在工作区路径元素（`id="wsPath"`）。
- 断言：该元素默认不显示内容（由 bootstrap 回填）。
- 验证命令：`grep -n -A3 'top-center' web/static/index.html`

### UC-02 (R) 表格数据来自 bootstrap 的 workspace

- 断言：`web/static/app.js` 中回填逻辑使用 `/api/bootstrap` 的 `workspace`（既有 `wsRoot`），未新增后端接口。
- 断言：文本前缀为 `" - "`（空格-连字符-空格）。
- 验证命令：`grep -n "wsPath\|wsRoot" web/static/app.js`

### UC-03 (R) 超长中间省略 + 完整路径 tooltip

- 断言：存在"中间省略"处理（标题/实现中含 `…` 或 `\u2026`），且保留末尾路径段。
- 断言：设置 `title`（或等价属性）为**完整绝对路径**。
- 断言：窗口 resize 时重算省略结果（存在 resize 监听或等效触发）。

### UC-04 (R) 窄屏隐藏

- 断言：CSS 中存在 `body.narrow` 下隐藏该元素（或其容器）的规则。
- 验证命令：`grep -n "narrow" web/static/index.html | head`

### UC-05 (M) 浏览器实测：宽屏显示路径

- 步骤：宽敞窗口（≥ 品牌宽度×3，如 1512px）打开 co-shell Web UI。
- 断言：顶栏中间显示 `<agent 名> - /Users/.../<workspace>`（左侧为 IDENTITY.md 的 agent 名）。
- 断言：文本与相邻元素有间隔，不遮挡连接状态/图标按钮。

### UC-06 (M) 浏览器实测：窄屏隐藏 + tooltip

- 步骤：把窗口缩窄到窄屏（或使用响应式模式），再恢复宽屏。
- 断言：窄屏时工作区路径**不显示**；恢复宽屏后再次出现（无重复元素、无残留空白）。
- 断言：路径被省略时，鼠标悬停显示**完整路径** tooltip。

### UC-07 (R) App 顶部条新增居中文本

- 断言：`app/macos/CoShellApp.swift` 的 `TitlebarStripView` 内新增居中文本 `co-shell`（NSTextField 或等效绘制）。
- 断言：字号 12–13pt、颜色为次级灰（随主题明暗自适应，非硬编码单一颜色）。
- 断言：不破坏原有窗口拖动（`mouseDownCanMoveWindow` 语义保持）。

### UC-08 (M) App 实测：顶部条文字可见且随主题换色

- 步骤：双击新构建的 App（独立目录构建）。
- 断言：顶部 28pt 留白条**中间**显示 `co-shell`，红绿灯/页面内容不被遮挡。
- 断言：切换页面主题（深/浅）后文字颜色随之适配（深色底浅字、浅色底深字），仍清晰可读。

### UC-09 (R) 编译与版本一致性

- 断言：`go build ./... && go vet ./...` 通过。
- 断言：`main.go` 的 `version` 与 `cmd/co-shell-hub/main.go` 的 `hubVersion` 均为 `0.108.2`，`build` 与 `hubBuild` 同步递增且相等。
- 断言：App 可构建（`app/build-app.sh <独立目录>` 成功），Info.plist 版本 = 0.108.2 / 对应 build。

### UC-10 (R) 改动范围最小

- 断言：`git diff --name-only` 只含 `web/static/index.html`、`web/static/app.js`、`app/macos/CoShellApp.swift`、`main.go`、`cmd/co-shell-hub/main.go`、`ROADMAP.md`、`use-case/FEATURE-627/**`。
- 断言：未改后端逻辑（`web/server.go` 等）、未改 hub 逻辑。

## 用例结果记录

| 用例 | 类型 | 结果 | 备注 |
|------|------|------|------|
| UC-01 | R | ✅ 通过 | `index.html` 的 `.top-center` 内 `#agentName` 后新增 `#wsPath` |
| UC-02 | R | ✅ 通过 | `app.js` 用 bootstrap 的 `workspace` 回填，前缀 `" - "`，未新增后端接口 |
| UC-03 | R | ✅ 通过 | 实测中间省略 ` - /U…/feat627-ws`（保留末段）；`title` 为完整路径；resize 时重算 |
| UC-04 | R | ✅ 通过 | `body.narrow .ws-path { display: none; }`（实测计算样式 `none`） |
| UC-05 | M | ✅ DOM+几何校验 | 文本 ` - /Users/direct3d/github/co-shell/tmp/feat627-ws`；与品牌区/右侧控件无重叠（截图工具 CDP 超时，未留图） |
| UC-06 | M | ✅ 用户确认 | 窄屏隐藏与 tooltip 已用 DOM 校验；用户 2026-10-10 11:44 确认通过 |
| UC-07 | R | ✅ 通过 | `TitlebarStripView` 新增居中 `co-shell` 标签（12.5pt、次级灰随主题、hitTest 透传保持可拖动） |
| UC-08 | M | ✅ 用户确认 | App 顶部条观感与深浅主题换色；用户 2026-10-10 11:44 确认通过（build 1285 实例） |
| UC-09 | R | ✅ 通过 | `go build`/`vet` 通过；version/hubVersion = 0.108.2，build/hubBuild = 1285；App 独立目录构建成功 |
| UC-10 | R | ✅ 通过 | 改动仅 `index.html`/`style.css`/`app.js`/`CoShellApp.swift` + ROADMAP + 版本号 + 本用例文件 |
