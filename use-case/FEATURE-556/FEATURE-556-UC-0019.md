# FEATURE-556 验收测试用例：浏览器 CDP 能力补强（P0 稳定性 + P1 配置化）

> 需求（用户提出）：「当前 co-shell 已经具备通过 CDP 操作浏览器的能力，但是似乎并没有实现所有 CDP 的功能和特性，
> 仔细查一下 CDP 还有什么功能咱们没有实现，比如配置浏览器等。给出你的建议。」→ 经讨论确认实施 P0 稳定性 + P1 配置化。
>
> 现状盘点（编码前已核实）：
> - 全仓库仅调用 **9 个 CDP 方法**：`Page.navigate`、`Page.getLayoutMetrics`、`Page.captureScreenshot`、
>   `Page.navigateToHistoryEntry`、`Runtime.evaluate`、`DOM.getDocument`、`DOM.getOuterHTML`、
>   `Input.dispatchMouseEvent`、`Input.dispatchKeyEvent`；
> - `browser/cdp.go:130-146` 的 `readLoop` **丢弃全部 CDP 事件**（只按 `id` 匹配响应）→ 无法事件化等待、遇 alert 弹窗会卡到超时；
> - 单页单连接、无 Target 管理；`browser/chrome.go:85-97` 启动参数硬编码；browser 配置仅 4 键（`config/config.go:515-526`）；
> - 元素定位靠自写 JS 遍历（`cdp.go:446-500`），无 `DOM.querySelector/getBoxModel` → 只能猜坐标；
> - 已知缺陷：`GoBack` 用非法 `entryId:-1`（`cdp.go:504`）、兜底吞错、`WaitForPageLoad` 无上限重试（`cdp.go:263-268`）、
>   整页截图缺 `captureBeyondViewport`（`cdp.go:290-304`）、`Click` 无 `mouseMoved`、`Type` 用 Unicode 码点当虚拟键码（`cdp.go:345`）、无 dialog 处理。
>
> 设计口径（经 ask_user 与用户确认）：
> - 版本归属 **v0.75.0**，任务编号 **FEATURE-556**，分支 **FEATURE-556**（基于 main）；
> - 范围 = **P0 稳定性 + P1 配置化**（P2：网络观测/多标签/console 采集/元素截图/PDF/上传/mock **另立 FEATURE**，不在本轮）；
> - 依赖策略 = **坚持零依赖手写 CDP**，只实现 Agent 有用的域，不引入 cdproto 等第三方协议库；
> - 「配置记住」三层：① co-shell 配置持久化到 `config.json`；② 浏览器会话态沿用 `workspace/browser-data` profile；
>   ③ CDP 会话级覆盖（视口/UA/语言/时区/证书/下载）在**建连后 apply-on-connect 自动重放**；
> - **不改变现有默认行为**：不配置任何新键时，Chrome 启动参数与既有 11 个 `browser_*` 工具行为与 v0.74.1 等价；
> - `browser_click` / `browser_type` / `browser_screenshot` 新增的 `selector` 为**可选**参数，不传时保留原坐标路径。

## 环境准备

- 本机已安装 Chrome / Chromium（`browser.IsChromeAvailable()` 为真），或让 co-shell 自行启动（默认 headless、端口 9222）。
- 本地测试页由测试代码临时生成为 `testdata/browser/*.html`（含延迟渲染页、alert 页、confirm 页、hover 菜单页、长页面、input 页、下载页）。
- 自签 HTTPS 用例需要本地 TLS 服务（测试代码内生成自签证书）。
- 集成用例统一以 `testing.Short()` 门控 + `IsChromeAvailable()` 跳过，避免 CI 无浏览器环境失败。
- 运行时（R 组）用例由用户在 Web UI / 终端实测确认。

## A. 单元用例（离线，不依赖 Chrome）

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| A-001 | 事件分发基础 | 向 `readLoop` 投递一条 `{"id":0,"method":"Page.loadEventFired","params":{...}}` | 订阅该事件的回调被触发且参数完整；不影响并发 pending 响应 |
| A-002 | 响应路径回归 | 投递 `{"id":7,"result":{...}}` | 仍按原路径返回给 `Call` 的等待者（事件改造不破坏响应分发） |
| A-003 | 事件与响应混流 | 交替投递事件与响应 | 两者各归其位，无串号、无 panic |
| A-004 | WaitEvent 超时 | `WaitEvent(ctx)`，超时内无事件到达 | 按 deadline 返回错误；订阅被清理（不残留回调、不泄漏 goroutine） |
| A-005 | 取消订阅 | `Subscribe` 后 `Unsubscribe`，再投递事件 | 回调不再被调用 |
| A-006 | 特殊键映射 | 对 Enter/Tab/Escape/Backspace/Delete/ArrowUp|Down|Left|Right/Home/End/PageUp/PageDown 生成按键参数 | `windowsVirtualKeyCode`/`code`/`key` 与浏览器约定一致（Enter=13、Tab=9、Escape=27、Backspace=8、方向键=37~40 等） |
| A-007 | 快捷键组合 | 生成 Ctrl+A、Ctrl+C、Ctrl+V、Ctrl+X、Ctrl+Z、Cmd+A（darwin） | `modifiers` 位正确（Ctrl=2，Meta/Cmd=4）；darwin 下用 Meta、其他平台用 Ctrl |
| A-008 | 文本输入路径 | 输入「你好，世界🙂」 | 走 `Input.insertText`（不经过 keyCode），文本原样传递 |
| A-009 | 默认 args 等价性 | 用默认配置构造 Chrome 启动参数 | 与改造前硬编码列表**逐项等价**（`--remote-debugging-port/--no-first-run/--no-default-browser-check/--disable-background-networking/--disable-sync/--disable-translate/--disable-extensions/--disable-default-apps/--mute-audio/--disable-features=TranslateUI/--window-size=1280,800` + headless + user-data-dir） |
| A-010 | 配置化 args | 配置 proxy/user-agent/language/window-size/download-dir/extensions=on/ignore-cert-errors=on | 生成对应 `--proxy-server/--user-agent/--lang/--window-size/--disable-extensions(不出现)/--ignore-certificate-errors/--headless` 等，顺序稳定可断言 |
| A-011 | apply-on-connect 序列 | 全量配置后生成 apply 调用集 | 包含 `Network.setUserAgentOverride`、`Network.setExtraHTTPHeaders`、`Emulation.setDeviceMetricsOverride`、`Emulation.setLocaleOverride`、`Emulation.setTimezoneOverride`、`Security.setIgnoreCertificateErrors`、`Browser.setDownloadBehavior`；**未配置的项不产生调用** |
| A-012 | dialog 策略解析 | `accept` / `dismiss` / `auto` 三种取值 | `auto` 展开为「除 beforeunload 外自动 accept」；非法值回退默认并告警 |
| A-013 | 整页截图参数 | `fullPage=true` | 参数含 `captureBeyondViewport=true` 与由 `Page.getLayoutMetrics` 计算的 `clip` |
| A-014 | 配置往返 | 写入全部新增 `browser_*` 字段 → 读回 | 值与类型一致（bool/int/string/数组）；旧配置（无新键）加载后全部取默认值，不报错 |
| A-015 | profile 选择 | `persist=on` / `persist=off` | on → 使用 `workspace/browser-data`；off → 使用独立临时目录，且退出时清理 |
| A-016 | 工具参数兼容 | 解析 `browser_click`/`browser_type`/`browser_screenshot` 的参数 | 不传 `selector` 时仍按 `x/y`、`text`、`quality/full_page` 解析（向后兼容）；同时传 selector 与坐标时以 selector 为准并给出提示 |
| A-017 | 下载目录解析 | 配置 download-dir 为空 / 相对路径 / 绝对路径 | 空 → 默认 `workspace/download/browser`；相对 → 相对 workspace；绝对 → 原样（并限制在 workspace 或显式允许） |

## B. 集成用例（真实 Chrome，`-short` 跳过 / 无 Chrome 跳过）

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| B-001 | 事件化等待（延迟渲染页） | 打开 1.5s 后才有内容的本地页 | `Navigate` 返回后目标元素已存在（旧实现会拿到半成品页面） |
| B-002 | alert 不阻塞 | 页面 onload 弹 `alert("hello")` | 导航与截图不被阻塞；工具返回信息中包含弹窗类型与文案 |
| B-003 | confirm 策略 | `dialog-policy=dismiss` 打开 confirm 页 | `confirm` 返回 false，页面走取消分支；工具返回提示包含该结果 |
| B-004 | 中文输入 | 向 input 输入「你好，世界」 | `input.value` 完全一致 |
| B-005 | 特殊键 | 输入后按 Enter | 页面 keydown 记录到 Enter 且表单提交被触发 |
| B-006 | hover 触发菜单 | hover 到菜单项 | 子菜单变为可见（旧实现无 mouseMoved，点不开） |
| B-007 | 右键事件 | 右键点击元素 | 页面记录到 `contextmenu` 事件 |
| B-008 | 双击 | `clickCount=2` 双击元素 | 页面记录到 `dblclick` |
| B-009 | 滚轮滚动 | `mouseWheel` 向下滚动 | `scrollY` 增大；虚拟滚动容器亦发生滚动 |
| B-010 | selector 点击 | `browser_click(selector="#btn")`，元素初始在视口外 | 元素自动滚动到视口内并被点中，页面状态改变 |
| B-011 | selector 输入（含清空） | `browser_type(selector="#name", text="abc", clear=true)` | 旧值被清空，新值为 `abc` |
| B-012 | selector 截图 | `browser_screenshot(selector="#card")` | 生成图片且宽高≈元素尺寸（±2px） |
| B-013 | WaitForSelector 超时 | 查询不存在的 selector（短超时） | 明确报「等待元素超时」并给出 selector；不长时间阻塞 |
| B-014 | 整页截图 | 长页面（含 fixed 头部）全页截图 | 图片高度覆盖全文档；fixed 头部不重复出现/不丢失 |
| B-015 | GoBack 正确性 | A→B 后 `browser_go_back` | URL 回到 A；不出现非法 `entryId` 的报错路径（无兜底吞错） |
| B-016 | 导航快速失败 | 导航到不存在域名 | 明确错误返回；无 30 秒白等（旧实现会卡满默认超时） |
| B-017 | 证书例外 | 自签 HTTPS 本地服务 | 默认失败；`ignore-cert-errors=on` 时成功打开 |
| B-018 | UA 生效 | 配置 user-agent | `navigator.userAgent` 与配置一致；未配置时保持默认 |
| B-019 | 视口生效 | 配置 viewport=1440x900 | `window.innerWidth/Height` 与配置一致（默认 1280x800） |
| B-020 | 语言与时区 | 配置 language=zh-CN、timezone=Asia/Shanghai | `navigator.language`、`Intl.DateTimeFormat().resolvedOptions().timeZone` 与配置一致 |
| B-021 | 下载落地 | 点击触发下载的链接 | 文件出现在 `workspace/download/browser/`，大小 > 0 |
| B-022 | 代理参数 | 配置 proxy=http://127.0.0.1:1080 | Chrome 启动参数含 `--proxy-server=http://127.0.0.1:1080`（不依赖真实代理可用性） |
| B-023 | profile 持久化 | `persist=on` 写入 localStorage → 重启浏览器再读；`persist=off` 重复 | on：localStorage 保留；off：为空（环境干净） |
| B-024 | 默认行为回归 | 不配置任何新键，跑既有 browser 用例集 | 启动参数与 v0.74.1 等价；11 个既有 `browser_*` 工具调用方式与行为不变 |
| B-025 | 并发安全 | 同时触发事件订阅与多次 `Call` | 无 data race（`-race` 通过）、无死锁 |

## C. 运行时用例（用户实测：Web UI / 终端 `.set` 与工具调用）

> 运行时用例是本次验收的主要抓手：**每一档功能都要能在真实站点上走通**，而不是只看单测。

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| R-001 | 弹窗不卡死（真实站点） | 打开含 `confirm`/`alert` 的站点（如门户登录退出确认） | 工具在秒级返回、不卡到超时；返回信息含弹窗类型与文案；用户无需手工干预 |
| R-002 | 中文搜索实测 | `browser_navigate` 到搜索页 → `browser_type` 输入中文关键词 → 回车/点搜索 | 输入框显示正确中文；结果页正常加载；截图可辨识 |
| R-003 | selector 表单操作 | 在真实登录页用 `browser_click(selector=...)`、`browser_type(selector=..., clear=true)` 填表 | 表单字段被正确填写与勾选（不要求真正登录成功） |
| R-004 | hover 菜单实测 | 真实站点顶部导航 hover → 子菜单出现 → 点子项 | 子菜单展开并可跳转（旧实现无 mouseMoved，通常失败） |
| R-005 | 元素级截图 | `browser_screenshot(selector=...)` | 返回的图片只包含目标元素区域，元素清晰完整 |
| R-006 | 下载落地 | 真实站点点击下载链接 | 文件出现在工作区 `download/browser/`，可用 `list_files` 看到 |
| R-007 | 证书例外开关 | `.set browser-ignore-cert-errors on` → 重启浏览器 → 访问内网自签 HTTPS 站点；再 `off` 复测 | on：站点可访问；off：回到证书报错（默认行为） |
| R-008 | UA 配置与记忆 | `.set browser-user-agent "..."` → 重启浏览器 → 查 `navigator.userAgent`；**重启 co-shell** 后再查 | 浏览器内 UA 为配置值；重启 co-shell 后依然生效（配置记住） |
| R-009 | 视口配置与记忆 | `.set browser-viewport 1440x900` → 重启浏览器 → 查 `window.innerWidth/Height`；重启 co-shell 后复查 | 视口为 1440x900；重启后仍生效 |
| R-010 | 代理配置 | `.set browser-proxy http://127.0.0.1:1080` → 重启浏览器 → 查看启动参数/实际出网 | 启动参数含 `--proxy-server`；无代理服务时报错可读，不影响其他功能 |
| R-011 | profile 持久化开关 | `.set browser-persist off` → 打开站点写入登录态 → 重启浏览器 | off：登录态清空（干净环境，便于复现）；`.set on` 后恢复保留 |
| R-012 | 设置页可见可改 | 打开 Web「系统设置」，找到新增的全部 browser-* 项 | 显示当前值/默认值，可修改并保存；保存后 `config.json` 落盘且立即生效 |
| R-013 | `.set` 列表与 reset | `.set` 查看 browser 组；`.set reset` 后复查 | 新增项按分类显示；reset 后回到默认值且 `config.json` 中对应键被复位 |
| R-014 | 默认行为回归（工具面） | 不配置任何新键，依次调用 11 个既有 browser 工具 | 行为与 v0.74.1 一致（坐标点击、`full_page` 截图、`browser_get_html`、前进/后退等） |
| R-015 | 默认行为回归（提示词） | 查看系统提示词中 browser 工具说明 | 既有工具说明无回归；新增参数（selector 等）有中英双语说明 |
| R-016 | 长任务稳定性 | 连续执行 10 轮「导航 → 截图 → selector 点击 → 取值」 | 无卡死、无节奏性超时；会话结束时浏览器可被正常关闭（`browser_close`） |
| R-017 | 首次启动无回归 | 全新工作区（无 browser-data）首次启用浏览器 | 自动创建 profile 目录并正常启动；`browser_close` 后可再次启动 |

## D. 缺陷修复专项验收（对应已核实的 7 个缺陷）

| ID | 缺陷（修复前事实） | 修复后验收方式 | 预期 |
|----|--------------------|----------------|------|
| D-001 | `GoBack` 用 `Page.navigateToHistoryEntry{entryId:-1}`（非法，靠 JS 兜底） | 观察 CDP 调用序列（日志/单测）+ 实测 A→B→back | 使用合法路径（`Page.getNavigationHistory` + 正确 entryId 或直接 `history.back` + 等待）；无「先报错再兜底」的浪费调用 |
| D-002 | 兜底分支吞掉错误并 `return "", nil` | 单测：让主路径失败 | 返回明确错误（含原因），不再静默成功 |
| D-003 | `WaitForPageLoad` evaluate 失败时无上限重试至 30s | 导航到不可达域名计时 | 明确失败且耗时显著小于默认超时（< 10s） |
| D-004 | 整页截图缺 `captureBeyondViewport` | 长页面 `full_page=true` 截图 | 高度覆盖全文档、fixed 元素不重复/不丢失 |
| D-005 | `Click` 无 `mouseMoved` | hover 菜单用例 + CDP 调用序列 | 点击前先发 `mouseMoved`；hover 类组件可点通 |
| D-006 | `Type` 用 Unicode 码点当虚拟键码 | 中文输入用例（B-004） | 中文/emoji 正确输入；特殊键走正确键码 |
| D-007 | 无 dialog 处理，弹窗即卡死 | 弹窗用例（B-002/B-003、R-001） | 弹窗自动按策略处理并回报，不再卡死 |

## 验收结论记录表（执行时填写）

| 组别 | 用例数 | 通过 | 失败 | 备注 |
|------|--------|------|------|------|
| A 单元用例 | 17 | | | |
| B 集成用例 | 25 | | | |
| C 运行时用例 | 17 | | | |
| D 缺陷专项 | 7 | | | |
| **合计** | **66** | | | |

## 通过标准

1. **A 组 17 条全绿**（离线可复现，`go test ./browser/ -run ...`／`go test ./...` 含 `-race`）。
2. **B 组 25 条全绿**（本机有 Chrome 时执行；无 Chrome 环境自动跳过并记录，不作为失败）。
3. **C 组 17 条由用户实测确认通过**（Web UI 真实站点操作）。
4. **D 组 7 条全部验证为已修复**。
5. 默认行为回归（A-009、B-024、R-014、R-015）必须完全等价 —— 这是"不改变现有默认行为"的硬约束。
6. `go build ./... && go vet ./...` 一次通过；`go test ./... -short` 无新增失败。
7. 每个 Phase 独立编译、提交到 `FEATURE-556` 分支并标注 BUILD 计数，可单独回滚。
