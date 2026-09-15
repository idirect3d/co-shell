# FEATURE-524 测试用例（增补）：窗口尺寸可控 + 屏幕保护 + 控件最小尺寸与响应式布局

> 本文档是 `FEATURE-524-UC-0001.md` 的增补（编号续 UC-56 → 从 **UC-57** 起）。
> 归属：版本 **0.59.0** 不变；任务号 **FEATURE-524**；分支 `FEATURE-524`（本会话已在该分支）。

## 背景

窗口模式（UC-50/51/54/55）落地后，窗口尺寸对 LLM 完全不可控：

| 项 | 现状 | 位置 |
|---|---|---|
| 宽度 | 固定 `min(420px, 100vw - 32px)` | `web/static/style.css:3820` |
| 高度 | 无 `height`，内容驱动；上限 `min(70vh, 640px)` | `style.css:3821` |
| 尺寸干预 | 无任何工具参数，无 JS 参与 | `agent/ui_tools.go:238-267`、`ui.js` |
| row 折叠 | `flex-wrap` + `basis 220px`，「放不下就换行」但会被 `flex-shrink` 压瘦，且无 `min-width` 保护 | `style.css:3659-3664` |
| 折叠判据 | 无容器查询，窗口宽度与视口无关时无法按窗口宽度折叠 | 同上 |

## 需求与已确认决策

需求（用户原话要点）：① 可指定窗口大小；② 屏幕分辨率保护（不得超出）；③ 内部控件大小自适应且有保护性最低宽/高；④ 布局控件按窗口宽度自适应，压缩到最小后转全纵向；⑤ 滚动条兜底保证全部内容可浏览。

已确认决策：

| # | 决策 | 结论 |
|---|---|---|
| Q1 | 版本归属 | **保持 0.59.0**（与窗口模式同一发布单元） |
| Q2 | 任务/分支 | **沿用 FEATURE-524 分支**，作为该特性增强 |
| Q3 | 尺寸参数形态 | **仅档位枚举** `auto` / `small` / `medium` / `large`（不收 px，避免 LLM 用错）；档位按**视口比例**换算，见 R1 |
| Q4 | 超限策略 | **静默钳制**到视口可用区，并在 `ui_window` 回执中体现 |
| Q5 | row 折叠实现 | **`@container` 精确折叠 + `flex-wrap`/`min-width` 兜底** |

## 实现契约（本文档待确认的核心）

### 1. 工具参数

| 参数 | 类型 | 必填 | 取值 | 说明 |
|---|---|---|---|---|
| `action` | string | 是 | `open` / `close` | 不变 |
| `title` | string | 否 | ≤ 60 runes | 不变 |
| `size` | string | 否 | `auto`(默认) / `small` / `medium` / `large` | **新增**；仅 `open` 有意义，`close` 忽略 |

后端（`agent/ui_tools.go`）**只做形态校验**：枚举外取值报错（i18n 中英）。后端不猜用户屏幕，因此不在此处钳制像素。

### 2. 档位尺寸（**视口比例**，已按修订 R1 确认）

统一以**可用区**为基准：`可用宽 = 视口宽 − 32`、`可用高 = 视口高 − 32`（左右/上下各留 16px，`fixed` 右下定位因此永不溢出）。

| 档位 | 宽 | 高 | 生效方式 | 说明 |
|---|---|---|---|---|
| `auto`（默认） | `min(420, 视口宽−32)` | 内容驱动 | 不写内联尺寸，走 CSS：`width: min(420px, 100vw−32px)` + `max-height: min(70vh, 640px)` | **与现状逐像素一致**（UC-67 回归保护） |
| `small` | `可用宽 × 1/2` | `可用高 × 1/2` | 写内联 `width`/`height`（钳制后 px） | 紧凑面板 |
| `medium` | `可用宽 × 2/3` | `可用高 × 2/3` | 同上 | 常规面板 |
| `large` | `可用宽 × 1` | `可用高 × 1` | 同上 | 表格 / 两栏版式，铺满可用区 |

- 档位是**比例**，同一档位在不同屏幕/视口下像素值不同——UC-60/61/68/71 一律用公式断言，不写死像素。
- `auto` 的 `max-height: min(70vh, 640px)` **只作用于 `auto`**；显式档位由钳制规则决定上限（可用区），否则 `large` 永远无法铺满大屏。
- 取整统一 `Math.round`；高度基准用 `innerHeight − 32`，与宽度同一套换算。

### 3. 钳制规则（前端 `openWindow`，优先级从上到下）

```text
可用宽 = 视口宽 - 32            // 左右各留 16px
可用高 = 视口高 - 32            // 上下各留 16px
标称宽 = 可用宽 × 比例(档位)     // auto → min(420, 可用宽)
标称高 = 可用高 × 比例(档位)     // auto → 内容驱动（不写 height）
实际宽 = clamp(标称宽, 0, 可用宽)   // ① 视口保护优先（绝不溢出）
实际高 = clamp(标称高, 0, 可用高)   // ① 视口保护优先（绝不溢出）
实际宽 = max(实际宽, MIN_WIN_W)     // ② 最小尺寸，前提 MIN_WIN_W ≤ 可用宽
实际高 = max(实际高, MIN_WIN_H)     // ② 最小尺寸，前提 MIN_WIN_H ≤ 可用高
```

- **视口保护优先于最小尺寸保护**：视口比最小值还小时，允许小于最小值，但**绝不溢出视口**。
- 视口变化（`resize`）时对已开窗口重新钳制一次，保证拖动/缩放浏览器后仍不溢出。
- 实际生效尺寸写入窗口 DOM 的 `data-ui-win-w` / `data-ui-win-h`（供断言与排查），不持久化。

### 4. 最小尺寸与折叠断点

| 项 | 值 | 理由 |
|---|---|---|
| `MIN_WIN_W` | 240px | 窗口最小可用宽 |
| `MIN_WIN_H` | 160px | 窗口最小可用高（保住标题栏 + 两行内容） |
| row 子项 `min-width` | 180px | 控件保护性最小宽，不许被压成窄条 |
| 折叠断点 | 容器宽 ≤ **380px** 时折叠 | 两列最少需 `180×2+10 = 370px`；380 提前 10px 折叠，避免贴边参差 |
| kv 窄屏 | 容器 ≤ 380px 时 `grid-template-columns: minmax(0,1fr)` | 键值改上下两行，防止 `max-content` 键列撑宽 |
| 滚动兜底 | `.ui-window-body { overflow: auto }`（已有）+ 折叠后仍溢出时纵向滚动 | 保证全部内容可达 |

> 折叠实现：窗口 body 声明 `container-type: inline-size`，用 `@container (max-width: 380px)` 把 `.ui-row` 切成 `flex-direction: column`、子项 `flex: 0 0 auto; width: 100%`。老浏览器忽略 `@container` 时退回现有 `flex-wrap` 换行（UC-65 验证不会横向溢出）。

### 5. 回执文案（i18n 中英）

后端不可知视口尺寸，因此回执为「**请求档位 + 尺寸受视口保护可能被收窄**」的表述，示例如：

- 中文：`已打开窗口「标题」（尺寸档位 medium，若视口不足会自动收窄）`
- 英文：`Opened window "title" (size preset medium; narrowed automatically if the viewport is small)`

实际生效像素由前端写入 `data-ui-win-w/h` 暴露，供人工排查与 UC-60/61/62 断言。

## 修订记录（本轮已确认，替代原「待确认点」）

| # | 原方案 | 修订后（**当前规格**） |
|---|---|---|
| R1 | 档位为固定 px 标称值（small 320×320 / medium 460×480 / large 720×600） | **改为视口比例**：`small = 可用区 1/2`、`medium = 2/3`、`large = 100%`（可用区 = 视口 − 32px）；`auto` 行为与现状**完全一致**（零回归） |
| R2 | 仅「回执提示 + 前端 data 属性」暴露实际值 | **另新增上行消息 `viewport`**：前端 WS 连接后与 `resize` 时上报 `innerWidth/innerHeight`，后端存入 RuntimeInfo 并注入 `environment_details` 的 `<runtime_info>`（新增 `<channel>` 与 `<viewport>`） |

`size` 允许在窗口已开时再次 `open` 改尺寸（UC-68 按「允许，且仍复用同一窗口」编写）——维持不变。

## 验收前置

```bash
cd /Users/direct3d/github/co-shell
go build ./... && go vet ./...                     # 编译与静态检查
go test ./agent/... -run 'TestUIWindow|TestUIUC'   # 工具层单测

# 测试实例（不动生产实例 28256 / PID 49539）
rm -rf /tmp/feat524 && mkdir -p /tmp/feat524
work/co-shell-0.59.0.darwin.arm64 --serve --port 28267 -w /tmp/feat524
```

浏览器断言统一在 `browser_evaluate` 中执行（`getBoundingClientRect` / `getComputedStyle` / `data-*` 属性），并配合 `browser_screenshot` 视觉复核。

---

## A 组：工具层校验（Go 单测，`agent/ui_window_test.go` 扩写）

### UC-57 档位枚举合法值全部接受

**目标**：`ui_window(action=open, size=<档位>)` 对 `auto`/`small`/`medium`/`large` 与**缺省**均返回成功回执。

**步骤**：表驱动依次调用 `uiWindowTool`，参数分别为 `missing`（不传 `size`）、`auto`、`small`、`medium`、`large`，以及带空格的 `" large "`。

**断言**：
1. 全部无 error；
2. 回执非空且包含档位名（缺省时按 `auto` 处理）；
3. `takePendingUIWindow()` 取出的 `uiWindowRequest.Size` 为规范化后的小写档位（空值归一为 `auto`）。

### UC-58 非法档位被拒绝

**目标**：枚举外取值必须报错，不得静默降级。

**步骤**：传入 `size` = `huge` / `LARGE!` / `1000` / `""`（空串按缺省处理，应**成功**）。

**断言**：
1. `huge` / `LARGE!` / `1000` 返回 error；
2. 错误文案同时包含中英（`i18n` 键存在且 zh/en 均已填值——由 `i18n` 完整性测试兜底）；
3. 空串等价缺省 → 成功且 `Size == "auto"`。

### UC-59 `close` 忽略 `size`，事件携带档位

**目标**：尺寸只属于打开动作；`ui_window` 事件必须把档位下发到前端。

**步骤**：`action=close, size=large` → 取 `pendingUIWindow`；再 `action=open, size=large` → 经 `UIWindowEvent()` 生成事件。

**断言**：
1. close 时 `Size == ""`（被清空，与 `Title` 同处理）；
2. `EventUIWindow` 的 `Meta` 含 `ui_window_size=large`（键名 `MetaKeyUIWindowSize`）；
3. 未指定时 `Meta` 中 `ui_window_size` 为 `auto`。

---

## B 组：前端尺寸与保护（浏览器 `browser_evaluate` + `browser_screenshot`）

> 统一前置：测试实例 `--serve --port 28267 -w /tmp/feat524`，模型侧通过 `ui_window`/`render_ui(target="window")` 打开窗口；断言用 `document.getElementById("uiWindow")`。

### UC-60 比例档位生效：small / medium / large 尺寸符合公式

**步骤**：视口固定为 1280×900（`可用宽 = 1248`、`可用高 = 868`），依次以三个档位打开窗口。

**期望值（公式推导，不写死）**：`small = 624×434`、`medium = 832×579`、`large = 1248×868`。

**断言**（每次 `getBoundingClientRect()`）：
1. `Math.round(rect.width)` / `Math.round(rect.height)` 分别等于上表期望值（±2px 容差，浏览器取整差异）；
2. 每档实测值都满足 `rect.width ≈ (innerWidth − 32) × 比例`、`rect.height ≈ (innerHeight − 32) × 比例`（按公式重算，验证的是比例语义）；
3. `#uiWindow.dataset.uiWinW/H` 与实测值一致；
4. 三次都只有一个 `#uiWindow`（复用，不叠加）。

### UC-61 视口保护：超限被静默钳制且不溢出

**步骤**：把浏览器视口调到 **400×360**（`可用宽 368`、`可用高 328`），请求 `size=large`（比例 100%）。

**断言**：
1. `rect.right <= innerWidth`、`rect.bottom <= innerHeight`（**不溢出**，核心红线）；
2. `rect.width <= innerWidth - 32`、`rect.height <= innerHeight - 32`（钳制到可用区）；
3. 再切到 **300×240** 的极小视口并请求 `size=large`：`rect.right <= 300`、`rect.bottom <= 240` 仍成立（**视口保护优先于最小尺寸**，允许小于 240×160）；
4. `dataset.uiWinW/H` 记录的是**钳制后**的实际值，而非标称值；
5. 触发一次 `resize`（视口再缩小）后尺寸被**重新钳制**，仍不溢出；
6. 无 JS 异常（`console` 无 error）。

### UC-62 最小尺寸保护与其优先级

**步骤**：(a) 视口 1280×900 请求 `small`（比例 1/2 → 624×434）；(b) 视口压到 **200×160**（可用宽 168、可用高 128，均小于最小值）请求 `small`。

**断言**：
1. (a) 宽 ≥ `MIN_WIN_W(240)`、高 ≥ `MIN_WIN_H(160)`，且实测等于 624×434（比例值远大于最小值，最小尺寸不生效）；
2. (b) **视口保护优先**：宽 ≤ 200−32、高 ≤ 160−32，允许小于 240/160，但**绝不溢出视口**；
3. `getComputedStyle` 断言窗口 `min-width/min-height` 仍为 240/160（规则存在，只是被视口压制）；
4. 两例中 `#uiWindowBody` 仍可滚动（`scrollHeight > clientHeight`），内容不会被裁掉无法访问。

### UC-63 控件保护性最小宽度

**目标**：row 子项不被压成窄条。

**步骤**：`size=small` 打开含 `row{col,col,col}` 的窗口。

**断言**：
1. 每个 `.ui-col` 的 `getBoundingClientRect().width >= 180`；
2. 无横向溢出：`.ui-window-body` 的 `scrollWidth <= clientWidth + 1`（或折叠已生效，见 UC-64）。

### UC-64 容器查询折叠：窄窗口下 row 转全纵向

**步骤**（比例档位下窗口宽不再固定，需按视口控制容器宽）：
1. 视口 1280×900，`size=large`（宽 1248 > 380）打开 `row{col,col,col}` 树，读 `.ui-row` 计算样式与子项几何；
2. 视口 **600×900**，`size=small`（`(600−32)/2 = 284 ≤ 380`）打开同一棵树；
3. 回到视口 1280×900，用 JS 强制 `#uiWindow.style.width = "300px"`（**视口宽、容器窄**）后再次读取。

**断言**：
1. 步骤 1：`getComputedStyle(row).flexDirection === "row"`，子项 `x` 坐标**不同**（真横向并排）；
2. 步骤 2：`flexDirection === "column"`，子项 `x` 坐标**相同**且宽度都接近容器内容宽（≥ 容器宽 − 20）；
3. 步骤 3：宽视口 + 窄容器下仍为 `column`（证明判据是**容器宽**，走 `@container` 而非媒体查询）。

### UC-65 折叠兜底（无 `@container` 支持时）

**步骤**：在页面内用 JS 移除窗口 body 的 `container-type`（模拟老浏览器不支持容器查询），并强制窗口宽 284px（视口 600×900）。

**断言**：
1. `.ui-row` 的 `flexDirection` 退回 `row`，但 `flex-wrap: wrap` 使子项换行；
2. **横向不溢出**：`.ui-window-body` 的 `scrollWidth <= clientWidth + 1`；
3. 全部子项仍在 DOM 中且 `width >= 180`（可被纵向滚动看到）。

### UC-66 滚动兜底：内容超出时全部可达

**步骤**：`size=small`（320×320）打开一棵明显超高 + 超宽的表（≥ 40 行、8 列）。

**断言**：
1. `.ui-window-body` 出现滚动：`scrollHeight > clientHeight`；
2. 把它 `scrollTop = scrollHeight` 后，**最后一个单元格** 的 `getBoundingClientRect().top <= body 底边` 且 `bottom >= body 顶边`（即滚到底后确实可见）；
3. 宽表由 `.ui-table-wrap` 内部横向滚动承担：`.ui-table-wrap.scrollWidth > clientWidth` 且窗口本身 `scrollWidth <= clientWidth + 1`。

### UC-67 默认档位（auto）零回归

**步骤**：不传 `size` 打开窗口，视口 1280×900；内容分别给「很短」与「很长」两棵树。

**断言**：
1. 宽 = `min(420, 1280−32)` = 420；
2. 高度：短内容时 < 200px（内容驱动，不撑满）；长内容时 = `min(70vh, 640)` = 640 后内部滚动；
3. 与 BUILD-1041 的既有行为一致（对照 UC-50/51/54/55 全部仍通过）。

### UC-68 已开窗口改档位（复用同一实例）

**步骤**：视口 1280×900，`open(size=small, title=A)`（624）→ `open(size=large, title=B)`（1248）。

**断言**：
1. `document.querySelectorAll("#uiWindow").length === 1`（不叠加）；
2. `rect.width` 由 624 变为 1248（`可用宽 × 1/2` → `× 1`，尺寸被更新而非沿用旧值）；
3. 标题更新为 `B`（沿用 UC-55 结论）；
4. 窗口内既有内容不被清空（`open` 只调尺寸/标题，不动 body）；
5. `dataset.uiWinW/H` 同步刷新为 1248 / 868。

### UC-69 关闭后重开：无残留尺寸

**步骤**：`open(size=large)` → 关闭（✕）→ `open()`（不传 size，即 auto）。

**断言**：
1. 关闭后 `#uiWindow` 隐藏且 `window.UI.isWindowOpen() === false`；
2. 重开后宽 = 420（auto 标称，未沿用上次的 720）；
3. `dataset.uiWinW/H` 已刷新，无旧值残留。

---

## C 组：文案与契约

### UC-70 回执与错误文案 i18n 中英齐备

**断言**：
1. 新增 i18n 键（`KeyUIToolParamWindowSize`、`KeyUIErrWindowSize`、`KeyUIWindowOpenSummary`（含档位）等）在 `i18n/zh_ui.go` 与 `i18n/en_ui.go` 均有值且非空；
2. 系统提示词中 `ui_window` 工具说明同步更新中英两版（工具 schema 的 `description` 走 `i18n.T`，不硬编码）；
3. `ui_window` 回执中出现档位名，并含「视口不足会自动收窄」的提示语义（中英各自成句）。

---

## D 组：本轮修订新增（比例档位边界 + 环境感知）

### UC-71 比例档位的数学闭合与边界

**目标**：证明档位严格等于「可用区 × 比例」，且在边界视口下仍然闭合（不溢出、可滚动）。

**步骤**：在下列视口依次 `open(size=large|medium|small)`，每例都按公式重算期望值：

| 视口 | 可用区 | `small` (1/2) | `medium` (2/3) | `large` (1) |
|---|---|---|---|---|
| 1280×900 | 1248×868 | 624×434 | 832×579 | 1248×868 |
| 1024×768 | 992×736 | 496×368 | 661×491 | 992×736 |
| 800×600 | 768×568 | 384×284 | 512×379 | 768×568 |
| 375×667（手机竖屏） | 343×635 | 171×317 | 228×423 | 343×635 |

**断言**（每例）：
1. `rect.width ≈ (innerWidth − 32) × 比例`、`rect.height ≈ (innerHeight − 32) × 比例`（±2px）；
2. `rect.right <= innerWidth && rect.bottom <= innerHeight`（**任何一例都不溢出**）；
3. `dataset.uiWinW/H` 与实测一致；
4. 375×667 下 `small`（171×317）虽宽 < `MIN_WIN_W(240)`，但**不溢出**——再次确认「视口保护 > 最小尺寸」；
5. `resize` 后再读 `dataset.uiWinW/H`：值随视口变化而更新（重钳制生效）。

### UC-72 environment_details 输出 channel 与 viewport（含上行 viewport 消息）

**目标**：LLM 能感知「当前跑在什么终端」与「视窗多大」。

**步骤**：
1. `--serve` 实例中打开页面；用 `browser_evaluate` 读 `window.innerWidth/innerHeight`；
2. 改变视口尺寸，再次发起一轮对话；
3. 取该轮 `environment_details` 的 `<runtime_info>` 文本（或直接用 Go 单测断言 RuntimeInfo 组装结果）；
4. 校验 WS 上行消息：`{"type":"viewport", ...}`。

**断言**：
1. Web 端 `<runtime_info>` 新增 `<channel>web</channel>`；TUI / mobile / feishu 等通道由 Go 单测覆盖取值映射（`tui` / `mobile` / `feishu` / `web`）；
2. `<viewport>` 输出形如 `1280x900`（`宽x高`），且与 `window.innerWidth/innerHeight` 完全一致；
3. 视口变化后（`resize` 上报）下一轮 `<viewport>` 数值同步更新；
4. **未上报时**（TUI / 旧客户端）：不输出 `<viewport>` 或输出占位值，**不得报错、不得 panic**；
5. 上行消息为 WS JSON `{"type":"viewport","value":{"w":<int>,"h":<int>}}`（字段名与 `web/server.go` 的 `clientMessage` 一致），`web/session.go` 有对应 `case "viewport"` 分支；
6. 非法/越界数值（0、负数、超大）被忽略或钳制，不写入 RuntimeInfo，且不中断会话。

---

## E 组：系统设置面板开关（ui_enabled / ui_context_prune 接入「智能体」页）

### UC-73 设置面板「智能体」页暴露两个开关

**目标**：用户能在系统设置里看到（而不只是写 config.json）组件渲染与窗口渲染的开关。

**步骤**：
1. 以 `--serve` 启动实例，打开设置面板 →「智能体」组；
2. 读取渲染出的开关项（key / value / default / desc）；
3. 对照 `cmd/settings_web.go` 的 agentGroup 定义与 `SettingsJSON()` 输出。

**断言**：
1. 面板出现 `ui-enabled`（UI 组件与窗口渲染）与 `ui-context-prune`（组件树上下文裁剪）两个 **bool** 项；
2. 两者均带当前值与默认值，默认均为 `on`（与 `config.DefaultConfig()` 的 `UIEnabled` / `UIContextPrune` = true 一致）；
3. 描述文案中英齐备（`KeyCol3UIEnabled` / `KeyCol3UIContextPrune` 在 `i18n/zh_system.go` 与 `i18n/en_system.go` 均有取值），风格与同组 `board-enabled` 一致（`...(on|off)`）；
4. Go 单测 `TestSettingsJSONIncludesUISwitches` 通过。

### UC-74 开关切换可落盘且免重启生效

**目标**：不只是“显示得出来”，而是**改得动**——写回 `config.json` 并在当前会话内立即生效。

**步骤**：
1. 面板中把 `ui-enabled` 置为 off（前端提交 `settings_set {key:"ui-enabled", value:"off"}`，与服务端 `.set` 同一链路）；
2. 检查 `config.json` 顶层 `ui_enabled`；
3. 再置为 on，校验恢复；对 `ui-context-prune` 重复一次。

**断言**：
1. `settings_result.ok = true`（CLI 路径：`Handle(["ui-enabled","off"])` 返回 nil error）；
2. `config.json` 顶层出现 `"ui_enabled": false`（键存在且为 false，非 `omitempty` 缺失），`ui_context_prune` 同理；
3. 关闭总开关后，`agent/tools.go` 的 `if a.uiEnabled()` 分支不再注册 `render_ui` / `ui_window`（`SetConfig` 已重指向，**无需重启进程**）；重新打开后工具恢复；
4. Go 单测 `TestSetUISwitchesPersist` 通过（含从磁盘反读校验）。

### UC-75 非法值与非布尔输入的容错

**步骤**：
1. 面板/CLI 提交 `ui-enabled = maybe`；
2. 提交不携带 value 的查询请求；
3. 提交非法值后检查 `config.json` 是否被写坏。

**断言**：
1. 非法值返回错误（`usage: .set ui-enabled on|off` / `ui-context-prune on|off`），面板显示失败结果；
2. 查询形式返回当前状态（`...: on|off`）；
3. `config.json` 不会因非法输入而损坏（仍为合法 JSON，开关值保持原状）；
4. Go 单测 `TestSetUISwitchesQueryAndInvalidValue` 通过。

---

## 非目标（本期明确不做）

1. 自由 px 尺寸参数（`width`/`height` 数值）；
2. 窗口拖动 / 最小化 / 最大化 / 持久化尺寸（仍为「单窗口 + 固定右下 + 仅关闭按钮」）；
3. 多窗口并存；
4. 引入任何 JS 测量/`ResizeObserver` 依赖来做折叠（折叠走 `@container` + flex 兜底）；
5. 视口变化时的「尺寸记忆」——`resize` 只做**再钳制**，不恢复用户曾经希望的大尺寸。

## 用例与验收对应表

| 用例 | 覆盖的需求点 | 验收方式 |
|---|---|---|
| UC-57/58/59/70 | ① 可指定大小（契约与校验） | Go 单测 |
| UC-60/61/62 | ① + ② 尺寸生效与屏幕分辨率保护 | 浏览器断言 + 截图 |
| UC-63/64/65 | ③ + ④ 控件最小尺寸与 row 纵向折叠 | 浏览器断言 + 截图 |
| UC-66 | ⑤ 滚动兜底 | 浏览器断言 |
| UC-67 | 无回归（auto 与现状一致） | 浏览器断言 + 原 UC-50/51/54/55 复跑 |
| UC-68/69 | 单窗口复用语义与状态清理 | 浏览器断言 |
| UC-71 | 比例档位数学闭合与边界（4 种视口 × 3 档位） | 浏览器断言 + 截图 |
| UC-72 | R2：channel / viewport 注入与上行协议 | Go 单测 + 浏览器断言 |
| UC-73 | 设置面板「智能体」页暴露窗口/组件渲染开关 | Go 单测 + 浏览器断言 + 截图 |
| UC-74/75 | 开关落盘与免重启生效、非法值容错 | Go 单测 + 浏览器断言 + `config.json` 校验 |
