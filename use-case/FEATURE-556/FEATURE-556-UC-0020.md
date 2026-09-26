# FEATURE-556 验收测试用例（Phase 3）：6 项 CDP 能力补强（P2 批次）

> 前序：`FEATURE-556-UC-0019.md` 覆盖 **Phase 1 = P0 稳定性** 与 **Phase 2 = P1 配置化**（已提交，commit d173409 / b04faa8 / e97ce18，BUILD-1121~1123）。
> 本文件覆盖 **Phase 3 = P2 能力补强**（6 项 CDP 能力，用户决策 2026-09-26）。
>
> 需求（ROADMAP 原文 P2 清单）：「补足 Agent 实际用得上的 CDP 能力域」——原清单含 **console 采集 / 文件上传 / PDF 导出 /
> 网络观测 / 多标签 Target / Fetch mock / 元素截图 / 弱网模拟** 共 8 项。
>
> 范围裁剪（经 ask_user 与用户确认，2026-09-26）：
> - **元素截图**已在 Phase 1 落地（`CaptureScreenshotClip` + `browser_screenshot` 的 `selector` 参数，B-012 已通过）→ 不在本轮；
> - **弱网模拟**（`Network.emulateNetworkConditions`）本轮 **不做**；
> - 因此本轮范围 = **6 项**：① console 采集；② 文件上传；③ PDF 导出；④ 网络观测；⑤ 多标签 Target；⑥ Fetch mock。
>
> 硬约束（项目规范 + 用户决策）：
> - 任务编号 **FEATURE-556**、分支 **FEATURE-556**（**不新建分支、不另立 FEATURE**）、版本仍 **v0.75.0**（`main.go` 版本号不变，**只递增 BUILD**）；
> - **零新增第三方依赖**：继续手写 CDP（不引入 cdproto / chromedp）；
> - **默认行为不变**：未使用新工具时，不产生额外 CDP 调用、不产生额外输出（`Runtime.enable`/`Log.enable`/`Network.enable`/`Fetch.enable` 等一律**按需开启**）；
> - 新增 6 个工具（`browser_console`/`browser_upload`/`browser_pdf`/`browser_network`/`browser_tab`/`browser_mock`）的描述与提示词**中英双语**；
> - **单文件 ≤ 500 行**；测试与实现同批提交；**不合并 main**。
>
> 实现落点（编码前规划）：
> - CDP 层：`browser/cdp_console.go`（`Runtime.consoleAPICalled`/`Runtime.exceptionThrown`/`Log.entryAdded` + 有界环形缓冲 + `TakeMessages`）、
>   `browser/cdp_upload.go`（`DOM.querySelector` → `DOM.setFileInputFiles`）、`browser/cdp_pdf.go`（`Page.printToPDF` → base64 落盘）、
>   `browser/cdp_network.go`（`Network.enable` + `requestWillBeSent`/`responseReceived`/`loadingFailed` + 有界缓冲 + 过滤）、
>   `browser/cdp_target.go`（`Target.*`，切换页面 = 重连目标页 `webSocketDebuggerUrl`）、`browser/cdp_fetch.go`（`Fetch.enable` + `requestPaused` → continue/fulfill/fail，含超时兜底）。
> - agent 层：新增 6 个工具，按「声明 `agent/tools.go` + 用法 key `agent/toolcall_mode.go` + 自动批准策略 `agent/agent.go`」三处注册；i18n 新增中英键。
> - 采集器挂载点：`ChromeManager.attachSession`（建连时创建，与既有 `DialogManager` 同构）。

## 环境准备

- 本机已安装 Chrome / Chromium（`browser.IsChromeAvailable()` 为真）；集成用例统一由 `testing.Short()` + `IsChromeAvailable()` 门控。
- 静态测试页与资产位于 `testdata/browser/`（新增 `console.html`：控制台各级输出 + 未捕获异常；`upload.html`：`<input type="file" multiple>`；
  真实上传目标文件 `testdata/browser/files/uc556-upload.txt`）；沿用既有 `basic.html`/`delayed.html`/`long.html`。
- 网络观测用例由 `httptest` 提供 `/ok.png`（200）与不确定路径（404），并包含一个指向不存在主机的请求以获得 `loadingFailed`。
- Fetch mock 用例页面 `mock.html` 通过 `<img id="mk" src="/mock-target.png">` 加载目标资源；未启用 mock 时应拿到真实静态文件，启用后应拿到 mock 内容。
- 运行时（C 组）用例由用户在 Web UI / 终端实测确认。

## A. 单元用例（离线，不依赖真实 Chrome）

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| A-013 | console 事件解析（`consoleAPICalled`） | 投递 `Runtime.consoleAPICalled`：`type=log/warning/error`，args 混合字符串、数字、`{}` 对象（含远程对象描述） | 条目含 `level`（log/warning/error）与文本化消息；字符串原样、数字转字符串、对象取 `description`/JSON；不 panic、不丢条目 |
| A-014 | console 事件解析（`exceptionThrown`） | 投递 `Runtime.exceptionThrown`（含 `exceptionDetails.text`/`url`/`lineNumber`） | 生成 `error` 级条目，文本含异常描述与来源位置（url:line）；可被 `TakeMessages` 取出 |
| A-015 | console 事件解析（`Log.entryAdded`） | 分别投递 `source=network`、`level=error|warning|info`、`text` 非空的条目 | 条目按原 `level`/`source` 归类，文本保留；`network` 源条目标记来源为网络 |
| A-016 | 有界环形缓冲与消费语义 | 连续投递 250 条 console 事件；随后 `TakeMessages()` 两次 | 缓冲区只保留最新上限条数（设计上限 200）；`TakeMessages` 只返回**新增未消费**条目，第二次调用返回空（不重复返回） |
| A-017 | 单条截断与返回限量 | 投递一条 800 字符消息 + 连续投递 8 条短消息后触发工具层汇总 | 单条在工具返回中被截断到约 200 字符（带省略标记）；工具返回最多列出 5 条新增条目 |
| A-018 | console 未启用零调用 | 不调用 `browser_console`，仅用既有 `browser_navigate`/`browser_click`（mock CDP caller 记录方法名） | 不出现 `Runtime.enable`/`Log.enable`，也不出现任何 console 相关 CDP 调用（默认行为不变） |
| A-019 | 上传参数构造 | mock：`DOM.querySelector` 返回 `nodeId=42`，调用 `SetFileInputFiles(ctx, "#f", []string{abs1, abs2})` | 依次调用 `DOM.querySelector{selector:"#f"}` 与 `DOM.setFileInputFiles{files:[abs1,abs2], nodeId:42}`；参数类型/字段名与 CDP 规范一致 |
| A-020 | 上传路径解析与校验 | 传入 workspace 相对路径 `input/a.txt`（存在于工作区）与不存在路径；相对路径含 `..` 越界 | 相对路径解析为 workspace 内绝对路径并由调用方校验存在；不存在 → 明确错误（双语）；越界路径被拒绝，均不发起 `setFileInputFiles` |
| A-021 | 上传 selector 未命中 | mock：`DOM.querySelector` 返回 `nodeId=0`（未匹配） | 返回明确错误「选择器未匹配到元素」（双语），**不**调用 `DOM.setFileInputFiles` |
| A-022 | 上传多文件顺序 | `files` 传 3 个路径 | `DOM.setFileInputFiles.files` 与传入顺序完全一致，数量一致（不排序、不去重） |
| A-023 | PDF 参数构造 | 分别调用默认与显式参数（`landscape=true`、`printBackground=false`、`paperWidth/Height`、`scale`）构造 `Page.printToPDF` 参数 | 默认 `printBackground=true`；未显式给出的可选项**不写入**参数（保持 Chrome 默认）；给出时原样写入且类型正确 |
| A-024 | PDF base64 落盘 | 用已知字节的 base64 调用落盘；再用非法 base64 调用 | 落盘文件字节与解码结果一致，返回路径 + 字节数；非法 base64 → 明确错误且**不产生**残留文件 |
| A-025 | PDF 默认文件名生成 | 页面标题含 `/ : * ?` 等非法字符、标题为空两种情形，均不传 `path` | 落 `<workspace>/download/browser/` 下；非法字符被替换为 `-`；标题缺失时回退 `page.pdf`；扩展名补 `.pdf` |
| A-026 | 网络请求-响应关联 | 先投 `Network.requestWillBeSent`（reqId=R1），再投 `responseReceived`（R1，status=200，mimeType=image/png） | 同 reqId 合并为**一条**条目（不重复建条目），最终含 `status=200`、`mimeType`、`type`、`url`、`method` |
| A-027 | 网络失败记录 | 投 `Network.loadingFailed`（R2，`errorText`/`blockedReason`、`canceled` 可选） | 条目 `failed=true` 且含失败原因文本；资源类型/URL 保留；`canceled=true` 时同样标记但不改变失败态 |
| A-028 | 网络缓冲上限与过滤 | 投 350 条事件；再以 `only_failures=true`/`limit=N` 查询 | 缓冲只保留最新上限条数（设计上限 300）；`only_failures` 只返回失败条目；`limit` 生效且不超过请求值 |
| A-029 | 失败摘要限量 | 一次导航产生 6 条失败请求 | 工具返回追加的「新增失败请求」摘要最多 3 条（按出现顺序取前 3），其余仅计数（不逐条列出） |
| A-030 | 标签列表映射 | 用固定 `Target.getTargets` 结果（含 `type=page`×2、`type=service_worker`×1） | 只列出 `type=page`；每条含 `targetId`/`url`/`title`；当前页被标记（且标记唯一） |
| A-031 | 标签选择解析与错误 | `index` 越界、`target_id` 不存在、两者均未提供 | 三种情形均返回明确错误（双语）；越界错误中给出可用范围 |
| A-032 | switch 选择逻辑 | 选中某个 page target（提供 `webSocketDebuggerUrl`） | 返回该 target 的 WS URL 供调用方重连（且返回 targetId/url）；不存在的 `webSocketDebuggerUrl` → 明确错误 |
| A-033 | close 当前页行为 | 关闭「当前页」target 与关闭「非当前页」target | 非当前页：正常关闭、返回成功；当前页：关闭成功但返回提示需重新选择页面（不得留下指向已关闭页的连接） |
| A-034 | Fetch 规则匹配 | 规则：子串 `api/user`、glob `*/mock-*.png`、精确 URL；用命中/未命中/大小写不同/带查询串的 URL 匹配 | 子串与 glob 命中符合预期；**未命中返回 continue**；大小写敏感策略固定且被测试固化；带查询串时按 URL 前缀/子串语义明确记录在测试中 |
| A-035 | continue 参数构造 | 命中 continue 规则处理 `Fetch.requestPaused` | 调用 `Fetch.continueRequest{requestId}`，**不携带**额外改写字段（保持原请求） |
| A-036 | fulfill 参数构造 | 规则给出 `status=201`、`headers={"X-Mock":"1"}`、文本 body / base64 body | 调用 `Fetch.fulfillRequest{requestId,responseCode:201,responseHeaders:[...],body:<base64>}`；文本 body 被正确 base64；`responseHeaders` 的 `name/value` 结构正确 |
| A-037 | fail 参数构造与校验 | 规则 `errorReason=Failed`/`Aborted`；再传非法 `errorReason` | 合法值 → `Fetch.failRequest{requestId,errorReason}`；非法值 → 明确错误，规则不被加入 |
| A-038 | Fetch 未启用/关闭语义 | ① 规则表为空时执行页面导航；② `add` 后 `off`、再次 `off`、`clear` | ① 不调用 `Fetch.enable`（默认行为不变）；② 首次 `off`/`clear` 调用 `Fetch.disable` 并清空规则，重复 `off` 幂等不报错、不重复 `disable` |

## B. 集成用例（真实 headless Chrome；`testing.Short()` + `IsChromeAvailable()` 门控）

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| B-026 | console 采集端到端 | 打开 `console.html`（页面执行 `console.log/info/warn/error` + 抛未捕获异常 + 请求一个 404 资源），等待稳定后调用 `browser_console` | 返回条目中含 `log`/`info`/`warning`/`error` 各级文本；含未捕获异常的 error 条目（带来源）；含 404 资源对应的 `network` 源 error 条目 |
| B-027 | console clear 与限量 | ① 采集一次后 `browser_console{clear:true}`；② 触发新的 8 条输出后再采集 | ① `clear` 后立即采集返回空（或仅剩 clear 之后的新条目）；② 返回最多 5 条、且长文本被截断（含省略标记），其余仅计数 |
| B-028 | 文件上传（单/多文件与失败路径） | 打开 `upload.html`；① 上传 `testdata/browser/files/uc556-upload.txt`；② 上传两个文件；③ 上传不存在的路径 | ① 页面 `input.files[0].name` 等于目标文件名（经 `browser_evaluate` 断言）；② `input.files.length==2`；③ 工具返回明确错误（双语）且 `input.files.length` 不变 |
| B-029 | PDF 导出 | 打开 `basic.html`：① 默认参数导出；② 指定 `path` + `landscape=true` 导出 | ① 默认落 `<workspace>/download/browser/` 下，文件存在、size>0、前 4 字节为 `%PDF`；② 指定路径文件存在且 size>0；返回文本含落盘路径与字节数 |
| B-030 | 网络观测（成功 + 失败） | 打开含 `/ok.png`（200）与 `/missing.png`（404）的页面，并触发一次指向不存在主机的请求；调用 `browser_network` | 条目含 200（mimeType `image/png`）与 404（`failed=false` 但 status=404，或被明确标记为失败——按实现对「HTTP 错误码」与「传输层失败」区分记录）；不存在主机的请求记录为 `failed=true` 且含失败原因 |
| B-031 | 网络过滤与限量 | 同上页面：① `only_failures=true`；② `limit=1` | ① 只返回失败条目（无 200 条目；口径为传输层失败 loadingFailed，不含 HTTP 4xx/5xx 响应）；② 返回条目数 ≤ 1 |
| B-032 | 多标签 open/list/switch/close | ① `browser_tab{action:"open", url:<about:blank 或 basic.html>}`；② `list`；③ `switch` 到新标签后 `browser_evaluate` 断言 URL/内容；④ `close` 新标签；⑤ 再次 `list` | ① 返回新 targetId；② 列表含 ≥2 个 page 且「当前」标记唯一指向原页；③ 切换后 evaluate 在新页执行（URL 为新页），旧连接已关闭（后续 evaluate 不报错、指向新页）；④ 关闭成功；⑤ 列表恢复为 1 个 page |
| B-033 | 多标签边界路径 | ① `switch` 用越界 `index`；② `switch` 用不存在的 `target_id`；③ `close` 当前页后调用 `list` 与 `browser_evaluate` | ①② 返回明确错误（双语，含可用范围/可用 id 提示），当前页不变；③ 不 panic：`list` 正常返回剩余页面并提示需重新选页；`evaluate` 不挂起——**工具层**下次操作自动重连到剩余可用标签，**裸 CDP 客户端**在已关闭连接上立即报错（集成测试 `TestIntegrationP3Tabs` 即断言后者） |
| B-034 | Fetch mock 命中（fulfill） | 打开 `mock.html` 前 `browser_mock{action:"add", match:"*/mock-target.png", status:200, body:"MOCK-BODY-556"}`，再打开页面；用 `browser_evaluate` 读取图片 `naturalWidth` 或改为 fetch 该 URL 读文本 | 请求被 fulfill：页面拿到 mock 内容（文本等于 `MOCK-BODY-556`，或图片尺寸等于 mock 数据尺寸），且**不**命中真实静态文件（可通过 mock body 内容区分） |
| B-035 | Fetch mock 未命中 / fail / 恢复默认 | ① 仅对 `*/mock-target.png` 加规则时加载页面中的其他资源（未命中）；② 新增 `fail{errorReason:Failed}` 规则后重新加载命中资源；③ `browser_mock{action:"off"}` 后再加载 | ① 未命中请求正常 continue（页面其他资源正常加载，无卡顿）；② 命中资源加载失败（`loadingFailed` 记录，图片 `naturalWidth==0`）；③ `off` 后恢复真实文件内容；**全程页面无因暂停而卡死**（超时兜底生效） |

## C. 运行时用例（用户在 Web UI / 终端实测）

| ID | 用例 | 步骤 | 预期结果 |
|----|------|------|----------|
| C-001 | 工具可见性与双语 | 在 Web UI 工具列表 / `:tools`（或等价入口）查看新增工具，切换中英文 | 6 个工具（`browser_console`/`browser_upload`/`browser_pdf`/`browser_network`/`browser_tab`/`browser_mock`）均出现且描述完整；中英文均非空、无未翻译占位 |
| C-002 | 默认行为不变（回归） | 不使用任何新工具，走一遍既有 `browser_navigate` → `browser_click` → `browser_type` → `browser_screenshot` → `browser_get_rendered_html` | 行为与 BUILD-1123 一致；**无**额外 console/network/mock 相关输出；无新增 CDP 报错 |
| C-003 | console 运行时 | 让 Agent 打开一个会输出控制台信息的页面并「汇总控制台错误」 | Agent 能给出控制台条目摘要（级别 + 文本），与浏览器 DevTools 观察一致 |
| C-004 | 上传运行时 | 让 Agent 把工作区内的一个文件上传到页面的文件选择框 | 上传成功，页面显示所选文件名 |
| C-005 | PDF 运行时 | 让 Agent 把当前页导出为 PDF 并给出路径 | PDF 落地、可用系统阅读器打开且内容与页面一致 |
| C-006 | 网络观测运行时 | 让 Agent 报告当前页面的失败请求 | 能列出失败 URL 与原因；无失败时明确回答「无失败请求」 |
| C-007 | 多标签运行时 | 让 Agent「新开一个标签打开 X，读取标题后切回原标签并读出原标签标题」 | 切换正确：新标签标题与切回后的原标签标题各自正确，不串页 |
| C-008 | mock 运行时 | 让 Agent 对某接口/资源加 mock，验证页面表现，再 `off` 恢复 | 加 mock 后页面表现符合 mock；`off` 后恢复真实资源；页面未卡死 |
| C-009 | 无新增配置键（回归） | 查看浏览器相关设置（`:set` / Web 设置页） | 键集合与 BUILD-1123 一致（P3 **不引入**新配置键），无未定义项、无重复项 |
| C-010 | BUILD 与版本校验 | 执行 `work/co-shell-0.75.0.darwin.arm64 --version` | 输出 `co-shell v0.75.0 [BUILD-1124]`（版本号仍 0.75.0，BUILD 递增；hub 同步为 `hubBuild = 1124`） |

## 覆盖矩阵（6 项能力 × 用例）

| 能力 | 单元用例 | 集成用例 | 运行时用例 |
|------|----------|----------|------------|
| ① console 采集 | A-013 ~ A-018 | B-026、B-027 | C-003 |
| ② 文件上传 | A-019 ~ A-022 | B-028 | C-004 |
| ③ PDF 导出 | A-023 ~ A-025 | B-029 | C-005 |
| ④ 网络观测 | A-026 ~ A-029 | B-030、B-031 | C-006 |
| ⑤ 多标签 Target | A-030 ~ A-033 | B-032、B-033 | C-007 |
| ⑥ Fetch mock | A-034 ~ A-038 | B-034、B-035 | C-008 |
| 全局约束（零依赖 / 默认行为不变 / 双语 / BUILD） | A-018、A-038 | — | C-001、C-002、C-009、C-010 |

## 说明与已知边界

1. **元素截图**与**弱网模拟**不在本文件范围内（前者已在 Phase 1 的 UC-0019 B-012 覆盖，后者本轮不做）。
2. console / network 采集器采用**有界环形缓冲**（设计上限 console 200 条、network 300 条），因此「更早的条目被淘汰」是**预期行为**，用例只断言「最新条目可见 + 上限生效」。
3. `browser_tab` 的 `switch` 依赖 Chrome 调试端点的 `webSocketDebuggerUrl`（`/json/list` 类接口），因此用例要求 ChromeManager 已连接（`browser_navigate` 或 `Start` 之后）；未连接时应给出明确错误（不 panic）。
4. Fetch mock 的匹配语义（子串 / glob / 大小写敏感）以 A-034 固化为准；若实现调整为大小写不敏感，须同步修订 A-034 与本节。
5. `testdata/browser/files/uc556-upload.txt` 为**新增测试资产**；既有 `download.html` 引用的 `files/uc556-payload.txt` 缺失属已知资产布局缺口，**不在本轮修复范围**（仅记录）。
6. 用例通过标准：A 组随实现提交的单测文件（`browser/cdp_*_test.go`）全部通过；B 组 `go test ./browser/`（含真实 Chrome）全绿；C 组由用户实测确认。
