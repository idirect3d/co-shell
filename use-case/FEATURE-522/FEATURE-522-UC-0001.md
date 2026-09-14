# FEATURE-522 测试用例：dark-muted（暗色低对比）主题——主消息区信息块正文字体调暗

## 背景

现有 4 个色调（`dark` / `light` / `light-tp` / `paper`）中，`dark` 为主题默认色。dark 主题下**主消息区信息块**（`.ev`）的背景为近黑深灰：

| 位置 | 变量 | 值 |
|---|---|---|
| 信息块卡片 | `--bg-panel` | `#10141d` |
| 代码块 / LLM 正文底 | `--mono-bg` | `#0d1119` |
| 块标题栏 / 工具参数子块 | `--bg-elev` | `#161b26` |

而块正文文字继承 `--fg` = `#d5dbe7`，与 `--bg-panel` 的 WCAG 对比度约 **13.5:1**（亮白偏刺眼）；代码块底 `#0d1119` 上更高。用户诉求：在**背景为黑或接近黑色的深灰区域**避免使用亮白色/高对比亮色作文字色，**适当调暗字体颜色**；**工作区与标题栏不动**，重点是主消息区的信息块。

## 需求（用户确认）

1. 基于当前 `dark` 主题样式，**新增一套主题风格**。
2. 目的：在背景为黑或接近黑色的深灰区域，避免亮白色或其他高对比亮色作为文字颜色，**适当调暗**字体颜色。
3. **工作区、标题栏不用调**，主要是主消息区的信息块。
4. 主题标识（`data-theme` 值）= `dark-muted`，显示名「暗色低对比」；归属版本 **v0.58.0**；任务号 **FEATURE-522**。
5. **调暗范围：仅信息块正文**（`.ev-body` 的正文 / 代码 / 思考 / 引用）；**块标题与警示色亮度保持现状**。
6. 接入方式：**加入主题循环切换按钮 + 系统设置下拉**（与现有 4 个色调并列）。

## 实现契约（需用户确认）

| 项 | 约定 |
|---|---|
| 新变量 | `--ev-body-fg`（默认 `var(--fg)`）、`--ev-body-fg-dim`（默认 `var(--fg-dim)`），在 dark 基础块声明 → 其余主题行为完全不变 |
| `dark-muted` 取值 | `--ev-body-fg: #a9b1c1`（≈8.7:1，dark 为 13.5:1，对比度降约 36%）；`--ev-body-fg-dim: #7b8395`（≈4.9:1，dark 为 6.1:1） |
| 受影响选择器 | `.ev-body`（改用 `--ev-body-fg`）；`.ev.thinking .ev-body`、`.ev-body.md blockquote`、`.ev-body.md h5/h6`（改用 `--ev-body-fg-dim`） |
| 配色来源 | `dark-muted` 不重定义调色板，直接继承 `:root`（即 dark）→ 工作区/标题栏与 dark **逐项相同** |
| 循环顺序 | `dark → dark-muted → light → light-tp → paper → dark` |
| 图标 | 新增 sprite symbol `i-theme-muted`（半明半暗圆，表意「低对比」） |
| logo | `dark-muted` 作为独立色调槽（`logos/logo-dark-muted.png`），与 FEATURE-477 机制一致 |
| 涉及文件 | `web/static/style.css`、`web/static/app.js`、`web/static/index.html`、`cmd/settings_web.go`、`web/server.go`、`ROADMAP.md`、`main.go`、`cmd/co-shell-hub/main.go` |
| 非目标（明确不做） | ① 不改工作区（`#sidebar`/`.tree`/文件树）与标题栏（`#topbar`/`.brand*`）任何样式；② 不改块标题栏（`.ev-head`）颜色；③ 不改警示色（工具标题 `#fbbf24`、错误 `#FF0000`、警告 `var(--warn)`、监督块 `#ffffff`/`#ff58f3`）；④ 不改 `.tool-params-body`（工具**输入参数**子块，背景 `--bg-elev`）、diff 增删色、`.ev.meta` 统计行；⑤ 不改动既有 4 个色调的任何取值 |

## 验收前置

```bash
cd /Users/direct3d/github/co-shell
# 1) 编译到 work/（规范：先编 work/ 再原子替换 ~/bin/）
go build -o work/co-shell . && go build -o work/co-shell-hub ./cmd/co-shell-hub

# 2) 启动独立测试实例（不使用生产实例 28256、独立端口与 workdir）
rm -rf /tmp/fe522 && mkdir -p /tmp/fe522
cd /tmp/fe522 && /Users/direct3d/github/co-shell/work/co-shell --serve --port 28257 --workdir /tmp/fe522
```

判定脚本统一在浏览器 `evaluate` 中执行（`getComputedStyle` + `getBoundingClientRect`），并配合 `browser_screenshot` 视觉复核。

---

## 用例

### UC-0001 主题枚举接入完整性（静态）
- **步骤**：分别 grep 5 个源文件中的 `dark-muted`。
- **预期**：`style.css` 有 `[data-theme="dark-muted"]` 变量覆盖块；`app.js` 的 `themeIcon()` / `themeMode()` / `applyTheme()` / `themeToggle` 循环 / 设置下拉 `opts` 均含 `dark-muted`；`index.html` 有 `id="i-theme-muted"` 的 symbol；`cmd/settings_web.go` 的 `theme-mode` `Options` 含 `dark-muted`；`web/server.go` 的 `logoThemeFromParam` 白名单含 `dark-muted`。
- **判定**：全部命中即通过（缺一即失败）。

### UC-0002 主题可切换并生效（运行时）
- **步骤**：设置 `localStorage["co-shell-theme"]="dark-muted"` → 触发 `applyTheme()`；读 `<html data-theme>`。
- **预期**：`data-theme === "dark-muted"`；`#themeToggle` 内 `<use>` 的 `href === "#i-theme-muted"`。
- **判定**：两项均成立。

### UC-0003 循环切换顺序（运行时）
- **步骤**：从 `dark` 起连点 `#themeToggle` 5 次，每次记 `<html data-theme>`。
- **预期**：序列为 `dark → dark-muted → light → light-tp → paper`，第 6 次回到 `dark`。
- **判定**：序列与预期完全一致。

### UC-0004 主消息区正文字体被调暗（核心，运行时）
- **步骤**：在 dark 与 dark-muted 下分别取一个 `.ev-body`（LLM 正文块）的 `getComputedStyle().color`；按 WCAG 相对亮度公式计算各自与其背景（`--bg-panel` `#10141d`）的对比度比值。
- **预期**：dark = `rgb(213, 219, 231)`（#d5dbe7），对比度 ≈13.5:1；dark-muted = `rgb(169, 177, 193)`（#a9b1c1），对比度 ≈8.7:1；**对比度下降 ≥25%** 且仍 ≥7:1（AAA 正文可读性下限）。
- **判定**：取值精确匹配 + 对比度区间成立。

### UC-0005 代码块 / 行内代码随正文调暗（运行时）
- **步骤**：dark-muted 下取 `.ev-body.md pre code` 与 `.ev-body.md code` 的 `color` 及 `background-color`。
- **预期**：`color === rgb(169, 177, 193)`（继承已调暗的正文色）；`background-color === rgb(13, 17, 25)`（仍为 `--mono-bg`，背景未改）。
- **判定**：两项均成立。

### UC-0006 思考（THINK）块文字调暗（运行时）
- **步骤**：dark 与 dark-muted 下分别取 `.ev.thinking .ev-body` 的 `color`。
- **预期**：dark = `rgb(139, 147, 165)`（#8b93a5）；dark-muted = `rgb(123, 131, 149)`（#7b8395）。
- **判定**：取值精确匹配且 dark-muted 更暗。

### UC-0007 块标题与警示色保持现状（运行时，反向用例）
- **步骤**：dark 与 dark-muted 下分别取以下元素的 `color`（或 `::before` 的 `background-color`）并对比：
  `.ev-head`（灰）、`.ev.tool .ev-head`（#fbbf24）、`.ev.level-error .ev-body`（#FF0000）、`.ev.supervisor .ev-body`（#ffffff）、`.ev.level-warning .ev-body`（`--warn` #facc15）、`.ev.user-msg .ev-head`（accent）。
- **预期**：两个主题下**逐项完全相同**（未调暗）。
- **判定**：无任何一项不同。

### UC-0008 工作区与标题栏保持 dark 配色（运行时，核心反向用例）
- **步骤**：dark 与 dark-muted 下分别取并比对：`#topbar` 的 `background-color`/`border-bottom-color`、`.brand-name` 的 `color`、`.tree-row.dir > .name` 的 `color`、`#sidebar .panel-head` 的 `background-color`/`color`、`.tree` 的 `background-color`、`.stream-head` 的 `background-color`、`#streamTitle`（或 `.stc-face`）的 `color`。
- **预期**：逐项完全相同（`dark-muted` 继承 `:root` 调色板，未重定义任何调色板变量）。
- **判定**：无任何一项不同。

### UC-0009 既有 4 个色调不受影响（回归，运行时）
- **步骤**：依次切到 `dark` / `light` / `light-tp` / `paper`，取 `.ev-body` 的 `color`，并与各自 `--fg` 计算值比对。
- **预期**：分别为 `rgb(213, 219, 231)` / `rgb(29, 36, 51)` / `rgb(29, 36, 51)` / `rgb(74, 59, 42)`，即等于本主题 `--fg`（变量默认回落正确）。
- **判定**：四项均与预期一致。

### UC-0010 系统设置下拉可选并持久化（运行时）
- **步骤**：打开系统设置 → 外观 → 「主题」下拉，确认第 3 项为 `dark-muted`；选中它 → 检查 `<html data-theme>` 与 `localStorage["co-shell-theme"]`；刷新页面后再看。
- **预期**：下拉列表为 `auto, dark, dark-muted, light, light-tp, paper`；选中即生效；`localStorage = "dark-muted"`；刷新后仍为 `dark-muted`。
- **判定**：四项均成立。

### UC-0011 构建 / 版本 / 部署（构建校验）
- **步骤**：`go build ./... && go vet ./...`（一次性执行）；`node --check web/static/app.js`；`work/co-shell --version`；比对 `~/bin/co-shell`。
- **预期**：编译与 vet 全绿；JS 语法检查通过；版本输出 `0.58.0`、BUILD `1023`；`~/bin/co-shell` 为同一构建（原子替换、新 inode 可执行）。
- **判定**：全部成立。

### UC-0012 logo 色调槽可用（运行时，接口侧）
- **步骤**：`GET /logos/dark-muted`（未配置时）；`DELETE /api/logo?theme=dark-muted`；`POST /api/logo` 携带 `theme=dark-muted`（可用 1×1 PNG 验证后删除）。
- **预期**：GET 返回 404（非 500）；DELETE 返回 `{"ok":true}`；POST 返回 `{"ok":true}` 且落盘 `logos/logo-dark-muted.png`；非法主题名仍返回 400。
- **判定**：与预期一致（用例结束删除测试 logo，恢复原状）。

### UC-0013 主题按钮图标与配色（运行时）
- **步骤**：dark-muted 下取 `#themeToggle` 的 `getComputedStyle().color` 与内部 `<use>` 的 `href`；取图标实际渲染尺寸。
- **预期**：图标引用 `#i-theme-muted`；图标 `color` 与其他主题同源（继承主题文字色，非硬编码白色）；渲染尺寸 16×16px（与既有 `.ico` 规则一致）。
- **判定**：三项均成立。

### UC-0014 视觉复核（截图）
- **步骤**：在同一个含「用户消息 / LLM 正文 / 代码块 / 思考 / 工具调用 / 错误」多样块会话下，分别以 dark 与 dark-muted 截主消息区图（同窗口尺寸、同滚动位置）。
- **预期**：dark-muted 的块正文明显更柔和、无亮白刺眼感，标题栏/工作区/块标题/警示色外观无变化；文字仍清晰可读（无「灰到看不清」）。
- **判定**：视觉确认通过（截图留档）。

---

## 通过标准

- **必须全过**：UC-0001 ~ UC-0013。
- **UC-0014 为主观视觉确认**：需用户或视觉复核确认「调暗适度」（既明显降低对比度，又不影响可读性）；如认为过暗/过亮，按用户意见微调 `--ev-body-fg` / `--ev-body-fg-dim` 后重跑 UC-0004、UC-0009、UC-0014。

## 回归风险点

1. **变量回落**：`--ev-body-fg` 若在某主题未定义会导致 `color` 声明失效并回退继承——UC-0009 覆盖 4 个既有主题。
2. **选择器特异性**：`.ev.level-error .ev-body`、`.ev.supervisor .ev-body`、`.ev.thinking .ev-body` 的特异性高于 `.ev-body`；本次用「变量驱动」而非新增选择器，避免特异性冲突——UC-0006/UC-0007 验证。
3. **`node --check`**：`app.js` 改动为字母串与条件分支，语法错误会直接导致 UI 白屏——UC-0011 验证。
4. **hub 版本同步**：`cmd/co-shell-hub/main.go` 的 `hubVersion` 必须与 `main.go` 的 `version` 一致——UC-0011 验证。

---

## 追加需求（第二轮，2026-09-15）：正文再调暗 + 录入框 / 会话标题栏

### 需求（用户确认）

1. **主消息区正文字色再调暗一档**：用户选定档位「中」→ `--ev-body-fg` = `#8f97a9`（≈6.4:1，原 `#a9b1c1` 8.70:1）；次级文字按同比例跟随（实现时按可读性下限处理，见下）。
2. **录入框与会话标题栏也调整**：范围为「文字色 + 录入框背景/边框一并压暗」。

### 实现契约（追加）

| 项 | 约定 |
|---|---|
| 正文 | `--ev-body-fg`：`#a9b1c1` → **`#8f97a9`**（对 `--bg-panel` #10141d ≈6.4:1；对代码块底 #0d1119 ≈6.5:1） |
| 次级 | `--ev-body-fg-dim`：`#7b8395` → **`#767e90`**（≈4.60:1）。**不严格按比例降**（按比例会到 ≈3.7:1，低于 WCAG AA 4.5:1，且 `.ev.thinking` 整体另有 `opacity:.75` 叠加）；取 AA 下限附近的 4.60:1 |
| 录入框 | 新增变量 `--input-fg` / `--input-bg` / `--input-border`（默认分别回落 `var(--fg)` / `var(--mono-bg)` / `var(--border)`）；`dark-muted` 下：文字 `#8f97a9`、背景 `#090c12`（原 `#0d1119`）、边框 `#1a2030`（原 `#232a3a`） |
| 会话标题栏 | 新增变量 `--stream-head-fg`（默认 `var(--fg)`）；`.stream-title` 与窄屏 `.stc-face` 改用该变量；`dark-muted` 下 `#8f97a9`。**标题栏背景 `.stream-head` 不变** |
| focus 态 | `#input:focus` 仍为 accent 边框（激活态提示色，与本主题「警示/强调色保持现状」一致） |
| 非目标（本轮不动） | ① `#input::placeholder`（占位符文字，用户未选该项）；② `.ask-line input` / `.identity-input` 等次要输入框；③ 状态条（`.sb-*`）；④ 工作区与顶栏（仍与 dark 完全一致）；⑤ 块标题与警示色 |

### 追加用例

### UC-0015 正文再调暗一档（核心）
- **步骤**：隔离实例中取 dark-muted 下 `.ev.llm .ev-body` 的 `color`，并与 dark 值比较，计算与 `--bg-panel` #10141d 的对比度。
- **预期**：`rgb(143, 151, 169)`（#8f97a9），≈6.4:1；dark 仍为 `rgb(213, 219, 231)`（≈13.5:1）；相比上一轮（8.70:1）再降约 26%。
- **判定**：取值与对比度区间成立。

### UC-0016 次级文字再调暗且不低于 WCAG AA
- **步骤**：取 dark-muted 下 `.ev.thinking .ev-body` 与 `.ev-body.md blockquote` 的 `color`，计算对比度。
- **预期**：`rgb(118, 126, 144)`（#767e90），对 `--bg-panel` 对比度 **≥4.5:1**（实测 ≈4.60:1）；dark 仍为 `rgb(139, 147, 165)`。
- **判定**：两项均成立（若 <4.5:1 视为不合格）。

### UC-0017 录入框文字/背景/边框一并压暗
- **步骤**：分别在 dark 与 dark-muted 下取 `#input` 的 `color` / `background-color` / `border-*-color`；再切 light / light-tp / paper 观察是否受变量改动影响。
- **预期**：dark-muted = 文字 `rgb(143,151,169)`、背景 `rgb(9,12,18)`、边框 `rgb(26,32,48)`；dark = `rgb(213,219,231)` / `rgb(13,17,25)` / `rgb(35,42,58)`；light/light-tp/paper 与改动前一致（各自 `--fg` / `--mono-bg` / `--border`）。
- **判定**：全部成立。

### UC-0018 会话标题栏文字压暗、背景不变
- **步骤**：取 `#streamTitle`（宽屏输入框）与 `.stc-face`（窄屏循环显示）的 `color`，以及 `.stream-head` 的 `background-color`，在 dark 与 dark-muted 下比对。
- **预期**：标题文字 dark `rgb(213,219,231)` → dark-muted `rgb(143,151,169)`；`.stream-head` 背景两主题均为 `rgb(16,20,29)`（未变）。
- **判定**：成立。

### UC-0019 反向用例：本轮改动不外溢
- **步骤**：dark 与 dark-muted 下比对 `#topbar` 底色/底边框、`.brand-name`、`.tree-row.dir > .name`、`.ev.tool .ev-head`（#fbbf24）、`.ev.level-error .ev-body`（#FF0000）、`.ev.supervisor .ev-body`（#ffffff）、`#input::placeholder` 计算色。
- **预期**：顶栏/工作区各项两主题完全一致；警示色逐项一致；占位符色未变（两主题相同）。
- **判定**：无任何一项不同。

### UC-0020 视觉复核（截图）
- **步骤**：同载具下分别以 dark 与 dark-muted 截图（含消息区 + 底部录入框 + 会话标题栏）。
- **预期**：dark-muted 的正文、次级文字、录入框（含底色/边框）、会话标题均明显更柔和；警告/错误仍醒目；工作区与顶栏无变化。
- **判定**：视觉确认通过（截图留档）。

### 追加通过标准

- UC-0015 ~ UC-0019 **必须全过**；UC-0020 由用户主观确认「再调暗一档是否合适（不过暗、仍可读）」。
- 若用户认为仍偏亮或过暗，仅需调整 `[data-theme="dark-muted"]` 块内 4 个色值 + 2 个正文变量，重跑 UC-0015 ~ UC-0019 与 UC-0020。
