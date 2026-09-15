# FEATURE-526 测试用例：Web UI 思考深度灯泡 + 上下文长度(K)

## 背景

现状（代码定位，非推测）：

| 位置 | 现状 |
|---|---|
| 状态栏模型菜单 `buildModelMenuItem()`（`web/static/app.js` 6277-6297） | 行内只有 `img.model-logo` + `span.model-id` + `span.model-ctx`，**完全没有思考信息** |
| 模型配置卡片第二行 `renderModelsBody()`（`web/static/app.js` 6186-6202） | 用 `#i-think` 灯泡表示**能力位** `m.thinking`（"支持思考"），无法区分思考深度；第二行末尾是 `· P{priority}`，**没有上下文长度** |
| 后端模型条目 `WebModel`（`cmd/model_web.go` 21-40） | 已下发能力位 `thinking` 与 `max_model_len`，**未下发**模型级思考开关 `thinking_enabled` 与思考深度 `reasoning_effort` |
| 模型配置字段（`config/model_template.go` 79-113） | `ThinkingEnabled *bool`（nil = 未设置/跟随全局）、`ReasoningEffort *string`（""/low/medium/high/xhigh 等） |

因此：状态栏菜单无法体现思考深度，配置卡片只显示"支持思考"这一笼统含义，且用户看不到上下文长度。

## 需求（用户确认）

1. 状态栏模型选择列表中，**模型名字的右边**，若模型"打开了思考"，显示代表**思考深度**的灯泡图标；共 5 种：空灯泡（未设置）/ L（Low）/ M（Medium）/ H（High）/ X（Xhigh）。其中除空灯泡外的 4 个为本次**新绘制矢量图**。
2. 模型配置界面中已显示的灯泡（`#i-think`）按上面的规则更新（同一模型在两处显示一致）。
3. 模型配置界面中，每个模型信息的**第二行右边**加上下文最大长度的数字，单位 K。

## 决策（用户拍板）

| 项 | 决定 |
|---|---|
| 版本 / 任务号 / 分支 | **v0.60.0** / **FEATURE-526** / `FEATURE-526` |
| 灯泡显示条件 | `capabilities.thinking === true && thinking_enabled !== false`（支持思考且未被显式关闭才显示；两处同一条件） |
| 深度映射 | `""` / `none` / `default` → 空灯泡；`low` → L；`medium` → M；`high` → H；`xhigh` / `max` → X；**其它未知值 → 空灯泡**（保守，不误显示为高深度） |
| 上下文长度 | `max_model_len > 0` 时用既有 `fmtLenShort()`（1K = 1024）；`max_model_len <= 0`（未知）时显示 **`?K`** |
| 非目标 | 不改 `#i-think` 图形本身；不改模型新增/编辑向导；不改 `WebTemplate`；不引入 emoji 或字体图标；不动状态栏除灯泡外的其它元素；不重构无关代码 |

## 实现契约（编码依据）

### 后端（`cmd/model_web.go`）

| 项 | 约定 |
|---|---|
| 新增字段 | `ThinkingEnabled bool \`json:"thinking_enabled"\``、`ReasoningEffort string \`json:"reasoning_effort"\`` |
| `thinking_enabled` 语义 | `m.ThinkingEnabled == nil \|\| *m.ThinkingEnabled`（nil = 未显式关闭 → true；false → false） |
| `reasoning_effort` 语义 | `m.ReasoningEffort` 解引用，nil → `""` |
| 不变项 | `Thinking` 仍是能力位 `m.Capabilities.Thinking`，语义不变 |

### 前端图标（`web/static/index.html`）

| 项 | 约定 |
|---|---|
| 新增 symbol | `i-think-low`、`i-think-medium`、`i-think-high`、`i-think-xhigh` |
| 风格 | 与既有 `#i-think` 一致：`viewBox="0 0 24 24"`、`fill="none"`、`stroke="currentColor"`、`stroke-width="1.6"`、`stroke-linejoin/linecap="round"` |
| 字母实现 | 灯罩内用 `<path>` / `<line>` 描画 L / M / H / X 笔画，**不使用 `<text>`**（避免字体依赖） |
| 既有 `#i-think` | 保持不变，作为"未设置"的空灯泡 |

### 前端逻辑（`web/static/app.js`）

| 项 | 约定 |
|---|---|
| 纯函数 `thinkDepthOf(m)` | 返回 `""`（不显示）/ `"none"`（空灯泡）/ `"low"` / `"medium"` / `"high"` / `"xhigh"`，判定规则见上表（大小写归一化） |
| 纯函数 `thinkIconEl(m)` | 返回 `mkIcon(...)` DOM 或 `null`；id 映射：`none → i-think`、`low → i-think-low`、`medium → i-think-medium`、`high → i-think-high`、`xhigh → i-think-xhigh` |
| 复用点 1 | `buildModelMenuItem()`：在 `span.model-id` 之后、`span.model-ctx` 之前插入 `thinkIconEl(m)`（`null` 则不插） |
| 复用点 2 | `renderModelsBody()`：能力图标改为"灯泡 = `thinkIconEl(m)`"；**vision / tool_call 图标保持原样** |
| 上下文长度（卡片） | 在第二行 `· P{priority}` 之后追加 `span.model-ctx`，文本 = `fmtLenShort(m.max_model_len) \|\| "?K"` |
| 布局（`web/static/style.css`） | `.model-meta` 改为 `display:flex; flex-wrap:wrap; align-items:center;`（模型名文本包进 `span.model-meta-main`），末尾的 `span.model-ctx` 依赖既有 `margin-left:auto` 贴右 |

## 验证环境（独立实例，不影响运行中的生产实例）

```bash
cd /Users/direct3d/github/co-shell
go build -o work/co-shell-0.60.0.darwin.arm64 .
./work/co-shell-0.60.0.darwin.arm64 --serve --port 12881    # 独立端口，不自动打开浏览器
```

浏览器打开 `http://127.0.0.1:12881/`，用调试浏览器（browser_* 工具）核对界面。
`app.js` 为经典脚本，`renderModels(models, templates)` / `renderModelsBody()` / `buildModelMenuItem()` 均为全局函数，可在 console 用**合成模型数据**驱动渲染，做确定性验证（见 UC-0003 ~ UC-0008）。

---

## UC-0001 状态栏菜单：支持思考且深度为 Low → 显示 L 灯泡（核心）

**类型**：运行时（真实实例）
**步骤**：
1. 打开 `http://127.0.0.1:12881/`，点击状态栏的模型选择器，展开模型菜单。
2. 找到"支持思考 + `reasoning_effort = low`"的模型（若无，用 UC-0003 的合成数据方式构造）。
3. 观察该行的结构：logo → 模型 ID → **灯泡** → 上下文长度（右对齐）。

**预期**：
- 模型名字右边出现灯泡图标，灯罩内有字母 **L**。
- 该行其余元素（logo、ID、右侧长度）位置与改动前一致，行不换行。
- 未打开思考的模型行**没有**灯泡元素。

---

## UC-0002 配置卡片：灯泡与菜单一致 + 第二行右侧显示上下文长度（核心）

**类型**：运行时（真实实例）
**步骤**：
1. 打开模型配置界面（模型管理弹窗），列出模型卡片。
2. 对同一模型，分别读取配置卡片第二行与状态栏菜单行里的灯泡 `<use href>` 值。
3. 观察配置卡片第二行的内容与右端。

**预期**：
- 同一模型两处灯泡的 `href` **完全相同**（一致性），例如同为 `#i-think-low`。
- 配置卡片第二行右端显示上下文长度，如 `128K`（`max_model_len = 131072`）。
- 第二行仍以 `· P{priority}` 结尾后再接长度，三行结构（ID / meta / endpoint）不变。

---

## UC-0003 五种灯泡状态齐全且可区分（合成数据，覆盖 5 态）

**类型**：运行时（浏览器 console 注入合成模型数据）
**步骤**：
1. 在 console 执行（合成数据驱动渲染，不改任何配置文件）：

```js
const mk = (id, effort, extra) => Object.assign({
  id, name: id, provider: "qwen", model: "qwen3-max", endpoint: "",
  api_key: "", priority: 1, enabled: true, vision: false, tool_call: false,
  thinking: true, thinking_enabled: true, max_model_len: 131072,
  available: true,
}, extra || {});
const models = [
  mk("m-none", ""), mk("m-low", "low"), mk("m-medium", "medium"),
  mk("m-high", "high"), mk("m-xhigh", "xhigh"),
];
renderModels(models, []);
renderModelsBody();
```
2. 在模型管理弹窗里逐行读取灯泡 `use` 的 href；再调用 `buildModelMenuItem(models[i], () => {}, null)` 读取菜单行的 href。
3. 放大截图观察 5 个灯泡（灯罩内空 / L / M / H / X）。

**预期**：
- 卡片侧 href 依次为 `#i-think`、`#i-think-low`、`#i-think-medium`、`#i-think-high`、`#i-think-xhigh`。
- 菜单侧 5 个 href 与卡片侧**逐项相同**。
- 截图可见 5 种状态的灯罩内分别可辨认为 空 / L / M / H / X，且共享同一灯体轮廓。

---

## UC-0004 深度映射边界：none / default / 未知值 → 空灯泡；xhigh / max → X

**类型**：运行时（合成数据）
**步骤**：
1. console 依次渲染 `reasoning_effort` 为 `""`、`"none"`、`"default"`、`"LOW"`（大写）、`"xhigh"`、`"max"`、`"turbo"`（未知值）的模型。
2. 读取每个模型在配置卡片的灯泡 href。

**预期**：

| reasoning_effort | 期望灯泡 href |
|---|---|
| `""` | `#i-think`（空） |
| `"none"` | `#i-think`（空） |
| `"default"` | `#i-think`（空） |
| `"LOW"` | `#i-think-low`（大小写归一化） |
| `"xhigh"` | `#i-think-xhigh` |
| `"max"` | `#i-think-xhigh` |
| `"turbo"`（未知） | `#i-think`（空，不误显示高深度） |

---

## UC-0005 模型不支持思考（capabilities.thinking = false）→ 两处都不显示灯泡

**类型**：运行时（合成数据）
**步骤**：
1. console 渲染 `thinking: false`、其余字段正常的模型。
2. 查看配置卡片第二行与状态栏菜单行。

**预期**：
- 配置卡片第二行**没有**灯泡元素（`svg.ico` 中不含 `use[href^="#i-think"]`）。
- 状态栏菜单行模型 ID 之后**没有**灯泡元素。
- vision / tool_call 图标（如模型声明了）仍正常显示。

---

## UC-0006 能力支持但被显式关闭（thinking_enabled = false）→ 两处都不显示灯泡

**类型**：运行时（合成数据）
**步骤**：
1. console 渲染 `thinking: true, thinking_enabled: false, reasoning_effort: "high"` 的模型。
2. 查看两处。

**预期**：
- 配置卡片与状态栏菜单**都不显示灯泡**（即使深度配了 high）。
- 再渲染 `thinking_enabled: true` 的同一模型 → 两处都出现 `#i-think-high`。

---

## UC-0007 上下文长度显示：128K / 1024K / ?K / 非整数

**类型**：运行时（合成数据）+ 间接单元覆盖
**步骤**：
1. console 渲染 `max_model_len` 分别为 `131072`、`1048576`、`1536`、`0`、`undefined` 的模型。
2. 读取每个模型配置卡片第二行右端 `span.model-ctx` 的文本。

**预期**：

| max_model_len | 期望显示 |
|---|---|
| `131072` | `128K` |
| `1048576` | `1024K` |
| `1536` | `1.5K`（与 `fmtLenShort()` 既有行为一致） |
| `0` | `?K` |
| `undefined` | `?K` |

- 长度文本贴在第二行右端（`margin-left:auto`），不与 `· P{priority}` 重叠；长 provider/model 文本时仍可读（必要时该行换行，长度仍靠右）。

---

## UC-0008 灯泡与长度图标均为矢量实现，无 emoji / 无字体依赖

**类型**：静态 + 运行时
**步骤**：
1. `grep -n "i-think" web/static/index.html`，查看 5 个 symbol 定义。
2. 在浏览器中检查灯泡 DOM：`document.querySelectorAll('symbol[id^="i-think"] *')`，确认节点为 `path`/`line`/`circle`，**不含 `text`**。
3. 检查渲染出的灯泡 SVG 使用 `use[href]` 引用 symbol（矢量雪碧图机制），不是 `<img>` 或字体图标。
4. `grep -rn "💡\|🔅" web/static/`（无匹配）。

**预期**：
- 存在 5 个 symbol：`i-think`（既有）、`i-think-low`、`i-think-medium`、`i-think-high`、`i-think-xhigh`。
- 4 个新 symbol 与 `i-think` 同 `viewBox="0 0 24 24"`、同描边风格；字母为几何笔画（path/line）。
- 无 emoji 字符、无 `<text>` 字体字形。

---

## UC-0009 状态栏菜单布局不回归（不换行 / 长度仍右对齐 / 高亮可用）

**类型**：运行时（浏览器 + 截图）
**步骤**：
1. 展开模型菜单，截图（含至少一个带灯泡的行）。
2. 检查 `.model-menu-item` 的行内元素顺序与样式：`model-id` 后为灯泡，`model-ctx` 仍 `margin-left:auto`。
3. 切换模型（点击另一行），确认 `active` 高亮、`unavailable` 置灰行为不变。

**预期**：
- 菜单行仍**单行不换行**（`white-space: nowrap` 未被破坏），宽度随最长行自适应。
- 上下文长度仍在行尾右对齐（`margin-left:auto` 生效）。
- 点击切换模型、`unavailable` 置灰、`默认` 行显示行为均与改动前一致。

---

## UC-0010 配置卡片布局不回归（三行结构 / 交互不受影响）

**类型**：运行时（浏览器 + 截图）
**步骤**：
1. 打开模型管理弹窗，截图一张含多行模型的图。
2. 依次操作：点击行选中（边框高亮）、点击模型 ID 打开编辑向导、点击右侧开关启用/禁用。
3. 检查第二行（provider · model + 图标 + 优先级 + 长度）、第三行（endpoint）是否仍成立。

**预期**：
- 每行仍为：logo / (ID、meta、url) / 开关；`.model-meta`（第二行）在 `.model-url`（第三行）之上。
- 灯泡与长度都在第二行，长度贴右端；`· P{priority}` 仍在长度左侧。
- 行选中、ID 点击进编辑、启用/禁用开关均正常，无 JS 报错（console 无 error）。

---

## UC-0011 后端 JSON 新增字段与语义正确（Go 单元测试）

**类型**：单元测试（`cmd/model_web_test.go`，新增测试函数）
**步骤**：
1. 构造 3 个 `config.ModelConfig`：`ThinkingEnabled = nil`、`= ptr(false)`、`= ptr(true)`；配套设置 `Capabilities.Thinking`、`ReasoningEffort`（含 nil 与 `"xhigh"` 两种）。
2. 调用 `ModelWebJSON()`，检查返回的 `WebModel` 字段。
3. 断言 JSON 序列化键名为 `thinking_enabled` / `reasoning_effort`。

**预期**：
- `ThinkingEnabled == nil` → `thinking_enabled = true`；`ptr(false)` → `false`；`ptr(true)` → `true`。
- `ReasoningEffort == nil` → `reasoning_effort = ""`；`ptr("xhigh")` → `"xhigh"`。
- `thinking`（能力位）仍等于 `Capabilities.Thinking`，语义未变。

```bash
go test ./cmd/ -run TestModelWebJSONThinkingFields -v
```

---

## UC-0012 编译 / 静态检查 / 既有测试无回归

**类型**：命令行
**步骤**：

```bash
cd /Users/direct3d/github/co-shell
go build ./... && go vet ./...          # 必须一次性串行执行
go test ./... -short                     # 既有测试无回归
go build -o work/co-shell-0.60.0.darwin.arm64 .   # 受影响应用产物
```

**预期**：
- 三条命令全部成功（`go build ./... && go vet ./...` 一条命令、无中断）。
- `go test ./... -short` 全绿（含 UC-0011 新增测试）。
- `work/` 下产出带版本号的当前架构可执行文件。

---

## 验收对照（全部通过才可合并）

| 验收标准 | 覆盖用例 |
|---|---|
| 1. 状态栏菜单与配置卡片对同一模型灯泡一致，思考关闭时不显示 | UC-0002、UC-0006 |
| 2. 5 种灯泡状态（空/L/M/H/X）可区分，全部 SVG 矢量，4 个为新绘制 | UC-0003、UC-0004、UC-0008 |
| 3. 配置卡片第二行右侧显示上下文长度（K，如 128K） | UC-0002、UC-0007 |
| 4. `go build ./... && go vet ./...` 一次性通过 | UC-0012 |
| 5. 用例位于 `use-case/FEATURE-526/` 且用户确认通过 | 本文件 |
| 回归：菜单与卡片原有布局与交互不被破坏 | UC-0009、UC-0010 |
| 后端字段语义正确 | UC-0011 |
