# FEATURE-571 用例组 A/B：主区覆盖面板基础、多窗口与 tabs（UC-0001 ~ UC-0010）

- 分支：FEATURE-571（v0.85.0）｜类型：运行时（浏览器实测）+ 单元（`agent/feature571_test.go`）
- 前置：`work/co-shell-0.85.0.<os>.<arch> --serve --port <port>` 启动 Web UI（**不要用 `--dev`**，否则 `/static/ui-windows.js` 404；也不要与用户实例抢端口），浏览器打开该端口。
- 约定：面板 = 主信息区覆盖形态（`#uiPanel`，z-index 30）；浮窗 = 右下角小窗（`.ui-window`，z-index ≥ 60）；两者共用同一窗口壳与 `ui_window` 协议。
- 实测标记：`[已实测]` 表示已在本分支测试实例上通过浏览器断言；未标记的用例待逐条验收。

## 组 A：面板基础与自动打开

## FEATURE-571-UC-0001 单次 panel 渲染自动开面板 `[已实测]`

**步骤**：`render_ui(target:"panel", tree: card{...})`（不带 window 参数）。
**期望**：面板自动出现并铺满主消息区（实测 1210×638，主区满宽）；内容为所渲染控件；`#uiPanel` 由隐藏态（`display:none`）变为 `display:flex`；对话流中**不新增**流式控件块；无 JS 报错。

## FEATURE-571-UC-0002 panel 目标与 stream/window 目标互不干扰 `[已实测]`

**步骤**：连续 `render_ui(target:"stream")` → `render_ui(target:"window")` → `render_ui(target:"panel")`。
**期望**：stream 块仍在对话流（随内容上卷）；window 目标进右下浮窗；panel 目标进面板；三者内容互不串台。

## FEATURE-571-UC-0003 面板刷新同一窗口（缺省寻址）`[已实测]`

**步骤**：`render_ui(target:"panel", tree:T1)` → `render_ui(target:"panel", tree:T2)`（不带 window）。
**期望**：第二次渲染进**同一窗口**（窗口数不变、tabs 数不变），内容由 T1 变为 T2，不产生第二个面板。

## FEATURE-571-UC-0004 面板内 `ui_update` 原地刷新 `[已实测]`

**步骤**：面板渲染含 `id:"ui-x"` 的卡片 → `render_ui(target:"panel", update:"ui-x", tree:…)`。
**期望**：该卡片**原地替换**；窗口数量与 tabs 不变；树 id 由后端分配（`ui-N`），断言以 `card(ui-` 前缀的摘要为准。

## 组 B：多窗口与 tabs

## FEATURE-571-UC-0005 两次 panel 渲染生成两个面板窗口 `[已实测]`

**步骤**：`render_ui(target:"panel", window:"p1", title:"面板一", tree:…)` → `render_ui(target:"panel", window:"p2", title:"面板二", tree:…)`。
**期望**：面板 tabs 出现两项「面板一 / 面板二」；活动窗口为 p2，p1 的 holder 仍在文档内但 `display:none`（`ui_update` 仍可命中它）；`<ui_windows>` 清单列出两窗。

## FEATURE-571-UC-0006 点击 tab 切换活动窗口并同步后端 `[已实测]`

**步骤**：点击「面板一」tab。
**期望**：p1 holder 显示、p2 隐藏；后端收到 `{type:"ui_window_state"}` 且 `active=p1`；此前共观察到 7 条 ui_window_state（含 open/activate/close）。

## FEATURE-571-UC-0007 浮窗与面板并存 `[已实测]`

**步骤**：面板 p1 活动时再 `render_ui(target:"window", window:"f1", tree:…)`。
**期望**：`.ui-window[data-window-name=f1]` 可见（实测宽 420px，偏移 16/16px）；面板仍可见；两者内容各自独立。

## FEATURE-571-UC-0008 tabs 列出所有窗口（含已成浮窗者）

**步骤**：把某窗口从面板「收成浮窗」后观察 tabs。
**期望**：tabs 仍列出该窗口（点回即可恢复为面板形态），tabs 数量与窗口总数一致；活动 tab 有选中态（`aria-selected`/高亮）。

## FEATURE-571-UC-0009 左右滑动切换（触摸/窄屏）

**步骤**：在面板内容区向左/向右滑动（或模拟 touch 事件）。
**期望**：切换到上一个/下一个窗口；到达首/尾时不越界（或循环，按实现，需在验收记录中固定一种）；滑动不误触发内容滚动以外的操作。

## FEATURE-571-UC-0010 窄屏（≤720px）隐藏 tabs 仍可切换

**步骤**：把视口宽度调到 ≤720px。
**期望**：tabs 条被 CSS 隐藏（`#uiPanelTabs` 不可见）；`≡` 列表入口仍可见并可点击切换窗口；滑动切换仍可用；面板仍铺满主区、不出现横向滚动。

## 组 C：关闭与形态切换

## FEATURE-571-UC-0011 头部「收成右下角浮窗」按钮 `[已实测]`

**步骤**：面板活动窗口中点击头部 `⇱` 按钮。
**期望**：该窗口由 panel 形态翻为 float 形态（DOM 从 `#uiPanelBody` holder 变为 `.ui-window`，按级联偏移摆放）；后端收到 `ui_window_state` 且该窗 `surface=float`；再点一次可翻回面板；tabs 始终列出该窗口。

## FEATURE-571-UC-0012 每窗独立 ✕ 关闭 `[已实测]`

**步骤**：面板 p2 上点 ✕；另一窗口仍在。
**期望**：仅 p2 被移除（holder/tab 同步消失）；活动窗口切到剩余最后一个窗口；后端收到 `closed:["p2"]`；p1 内容不受影响。

## FEATURE-571-UC-0013 Esc 关闭活动窗口 `[已实测]`

**步骤**：焦点在页面内，按 `Esc`。
**期望**：活动窗口被关闭，其余窗口保留；后端收到含该窗名的 `closed` 名单（实测共 7 条 ui_window_state）；Esc 不影响文件预览等其它 Esc 语义（预览打开时由预览优先处理）。

## FEATURE-571-UC-0014 关闭最后一个窗口后面板消失

**步骤**：把窗口逐个关闭至 0。
**期望**：`#uiPanel` 回到 `display:none`（实测无窗口时 `class="ui-panel hidden"`、宽高 0），主信息区不再被遮挡、可正常点击；再渲染 panel 可重新打开。

## 组 D：跨会话与刷新恢复

## FEATURE-571-UC-0015 按会话写入 localStorage `[已实测]`

**步骤**：`UIWin.setSession("s-test")` 后渲染一个 panel 窗口（p-test/面板A）与一个 float 窗口（f-test/浮窗B）。
**期望**：`localStorage["co-shell-ui-windows:s-test"]` 存在，内容形如 `{active:"f-test", windows:[{name,surface,title,tree,summary},…]}`（实测长度 529 字节，两窗各带 tree JSON）；未设定会话 id 时不写入（`persist()` 直接返回）。

## FEATURE-571-UC-0016 刷新页面后按会话恢复 `[已实测]`

**步骤**：`browser_navigate` 重新加载页面 → `UIWin.setSession("s-test")`。
**期望**：从 localStorage 重建全部窗口——tabs 恢复为「面板A/浮窗B」两项（tabsCount=2）；holder `p-test`/`f-test` 均挂载；浮窗 f-test 宽 420px；两窗内容（`restored-check-A`/`restored-check-B`）均被重新渲染；面板出现在主区（实测 1210×638）。

## FEATURE-571-UC-0017 切走再切回（不刷新，内存桶）`[已实测]`

**步骤**：`setSession("s-other")` → `setSession("s-test")`。
**期望**：切到 s-other 时窗口全部收起（tabs 空、无 holder/floats）；切回 s-test 时原窗口原样挂回（DOM 未被销毁重建），内容与顺序、活动窗口均与切走前一致。

## FEATURE-571-UC-0018 会话隔离

**步骤**：会话 A 开两个窗口 → 切到会话 B（未开过窗口）→ 在 B 渲染一个新面板。
**期望**：B 只显示自己新开的窗口；A 的窗口不出现在 B；A、B 各自 localStorage 键独立（`co-shell-ui-windows:<A>`、`…:<B>`）；切回 A 恢复 A 的窗口。

## FEATURE-571-UC-0019 存储异常时降级不崩

**步骤**：(1) `localStorage.setItem` 抛异常（模拟配额满/隐私模式）；(2) 手工把存储值改成非法 JSON 后刷新并 `setSession`。
**期望**：(1) 仅 `console.warn("[ui-windows] persist failed")`，窗口仍正常显示与交互；(2) 非法 JSON 被忽略（`console.warn("[ui-windows] stored windows ignored")`），页面不抛未捕获异常、不白屏。

## 组 E：与文件预览并存

## FEATURE-571-UC-0020 预览盖在面板之上

**步骤**：面板渲染后，点击某文件行打开文件预览。
**期望**：预览层覆盖在面板之上（预览 z-index 40 > 面板 30）；关闭预览后露出面板原内容（面板不被销毁、不重渲染）。

## FEATURE-571-UC-0021 两者各自关闭互不影响

**步骤**：(1) 关掉预览 → 面板仍在；(2) 打开预览 → 关闭面板窗口。
**期望**：均只影响自身；不出现「关一个另一个跟着消失」或残留遮罩阻断点击。

## FEATURE-571-UC-0022 面板打开时不影响对话流与输入

**步骤**：面板打开状态下继续发送消息、滚动对话流、点击输入框。
**期望**：对话流正常追加与上卷；输入框可获得焦点并可输入；面板不拦截主区外的点击（覆盖层范围限定在主信息区）。

## 组 F：窗口清单与 LLM 引导

## FEATURE-571-UC-0023 `<ui_windows>` 清单格式 `[已实测]`

**步骤**：分别观察「无窗口」与「有 2 窗（1 panel 活动 + 1 float）」时的 environment_details。
**期望**：无窗口时为 `<ui_windows>none</ui_windows>`；有窗口时每窗一行 `name | surface | active | title | summary`（摘要形如 `card(ui-7)`，实测摘要前缀 `card(ui-`），活动窗口 `active=true`。

## FEATURE-571-UC-0024 清单驱动「新建 vs 更新」

**步骤**：`ui_window(action:"open", title:"监控面板")` 建窗 → 下轮 LLM 依据清单用 `render_ui(target:"panel", window:"监控面板", …)` 更新；再重名 `ui_window(open)` 同名。
**期望**：LLM 依据清单选择更新既有窗口而非新建（窗口数不增长）；同名 open 不产生重复窗口，仅刷新标题/形态。

## FEATURE-571-UC-0025 前端操作即时反映到清单（无状态漂移）`[已实测]`

**步骤**：依次执行 开面板 / 切 tab / 收成浮窗 / ✕ / Esc。
**期望**：每一步都发出 `{type:"ui_window_state", windows:[…], closed:[…]}`（实测 7 条），后端 `<ui_windows>` 与前端所见一致：名字、surface、active、顺序均匹配；关闭的窗口不再出现在清单中。

## FEATURE-571-UC-0026 名称校验与窗口上限（单元 `[已实测]`）

**单元用例**（`go test ./agent/ -run 'Feature571|UIWindow' -v`）：
1. 非法窗口名（含 `/`、`\`、控制字符、超 32 字符）→ 拒绝并返回可自纠的中英错误。
2. 合法名（字母数字、CJK、空格、`-`、`_`、`.`）→ 通过。
3. 窗口数超过上限 8 → 拒绝新开；`SyncUIWindowsPayload` 超限条目被忽略。
4. 窗口名缺省 → `default`。
**期望**：全部符合上述判定；`agent/feature571_test.go` 5 个测试（PanelRenderRegistersWindow / NameAndSurfaceValidation / WindowCap / UIWindowsBlock / SyncFromFrontend）全部 PASS。

## 组 G：零回归

## FEATURE-571-UC-0027 `target=stream` 渲染不变

**步骤**：`render_ui(tree:…)`（缺省 target）渲染含表格/表单/图表的控件。
**期望**：仍在对话流中按块渲染，块顺序与位置不变；不误进面板或浮窗。

## FEATURE-571-UC-0028 `target=window` 渲染不变 `[已实测]`

**步骤**：`render_ui(target:"window", tree:…)`（不带 window 名）。
**期望**：进 `#uiWindow` 浮窗；首次复用原始浮窗本体，多窗时克隆并级联偏移（16/16px、右下角），z-index 递增；缺省名 `default`；✕ 关闭按钮仍可用。

## FEATURE-571-UC-0029 `ui_update` / `update` 命中各种窗口

**步骤**：对 (a) 活动面板窗口、(b) 隐藏的非活动面板窗口、(c) 浮窗 分别 `render_ui(update:"<id>", …)`。
**期望**：三种情况均原地替换成功；隐藏窗口的更新在切回时可见（不丢失）。

## FEATURE-571-UC-0030 actions / waiting 交互不变

**步骤**：在面板中渲染带 `actions:[{on:"click",…}]` 的控件；(1) 普通渲染点击；(2) `render_ui(waiting:true)` 后点击。
**期望**：(1) 点击作为新用户消息开启新一轮；(2) 点击作为等待结果立即返回给当前 LLM 调用；两种路径的面板窗口均不被意外关闭。

## FEATURE-571-UC-0031 四主题与窄屏折叠

**步骤**：切换全部主题（含暗色）；把视口压到 ≤720px 后再拉回宽屏。
**期望**：面板 tabs、按钮、关闭图标在各主题下对比度正常、无不可读文字；窄屏下 tabs 隐藏、`≡` 与滑动可用；拉回宽屏后 tabs 恢复；面板尺寸随主区重新套用（`UI.applyWindowSize`）不错位。

## FEATURE-571-UC-0032 编译与静态检查全绿 `[已实测]`

**步骤**：`go build ./... && go vet ./...`（一次性）→ `go test ./agent/`；`grep -rn "eval(\|new Function" web/static/ui-windows.js`；`browser_console` 检查无 JS 报错。
**期望**：编译与 vet 无输出；`go test ./agent/ -run 'Feature571|UIWindow'` → ok；既有 UC-50~UC-59 用例仍全绿；控制台无报错与未捕获异常。

## FEATURE-571-UC-0033 提交与版本合规

**步骤**：`git branch --show-current`、`git log --oneline -3`、`git tag --list 'v0.85.0'`。
**期望**：开发提交均在 `FEATURE-571` 分支（**main 无直接提交**）；`main.go` version=0.85.0 且 build 计数与 `cmd/co-shell-hub/main.go` 的 `hubBuild` 一致；用户确认后才合并 main 并打 tag `v0.85.0`。
