# FEATURE-616 验收测试用例（UC-0001）：消息指示器 hover 定位（划过竖线即跳转、标题对齐消息区顶部）

> 权威依据：`ROADMAP.md` v0.102.0 / FEATURE-616；分支 **FEATURE-616**，版本 **v0.102.0**。
> 相关代码：`web/static/app.js`（消息指示器 = 消息可视化图表 `#msgViz` / `#msgVizTrack`：`initMsgViz()`、`msgVizFlush()`）、`web/static/style.css`（`.msgviz` / `.msgviz-line`）。

> 已确认决策（用户确认，不可改动）：
> ① **划过即定位**：鼠标划过某条竖线（`.msgviz-line`），主信息窗口（`#streamB`）即时滚动到该线代表的消息块（`.ev`）——**无防抖、无延迟**；
> ② **对齐口径**：跳转后该块的**标题行（`.ev-head`）正好位于消息区顶部**（块顶边与滚动区顶边对齐）；
> ③ **移出保持**：鼠标移出指示器后停留在跳转后的位置，**不回滚**；
> ④ 版本归属 **v0.102.0**（FEATURE 走 minor 递增），任务编号 **FEATURE-616**。

> 已知限制（本次不做）：
> - 触屏无 hover 概念，本次只覆盖鼠标（指针）hover；触屏仍走既有「点击竖线定位」。
> - 目标块靠近会话末尾时，滚动区已到底而无法把标题顶到最上方——浏览器自然钳制在能滚到的最大位置（不视为缺陷）。
> - 窄屏（`body.narrow`，指示器换行显示）交互一致，不额外做窄屏专属适配。

## 用例清单

| 编号 | 名称 | 类型 | 对应需求 |
|------|------|------|----------|
| A-001 | `msgVizLocate` 存在，且用「块矩形顶 vs 滚动区矩形顶」求位移 | 静态断言 | ② |
| A-002 | 定位写入 `streamB.scrollTop`（增量赋值） | 静态断言 | ② |
| A-003 | `#msgVizTrack` 上存在 `mouseover` 委托（`closest(".msgviz-line")`） | 静态断言 | ① |
| A-004 | 拖拽平移期间不触发 hover 定位（`dragging` 守卫） | 静态断言 | ① 回归 |
| A-005 | `_box` 缺失/已断开时安全返回（不抛错） | 静态断言 | 健壮性 |
| A-006 | 既有「点击竖线定位」与「拖拽平移」代码仍在 | 静态断言 | 回归 |
| B-001 | 真实 mouseover 划过竖线 → 对应块标题行对齐消息区顶部（\|Δ\|≤1px） | 运行时 | ①② |
| B-002 | 依次划过不同竖线 → 分别定位到各自块（\|Δ\|≤1px） | 运行时 | ①② |
| B-003 | 移出指示器后位置保持（不回滚） | 运行时 | ③ |
| B-004 | 快速连续划过多条竖线 → 逐条即时定位（每次都生效，无防抖） | 运行时 | ① |
| B-005 | 拖拽平移过程中划过竖线不触发定位 | 运行时 | ① 回归 |
| B-006 | 点击竖线仍能定位（回归） | 运行时 | 回归 |
| B-007 | 轨道空白处（非竖线）划过不改变滚动位置 | 运行时 | ① |
| B-008 | 空指示器（无竖线）划过不报错 | 运行时 | 健壮性 |
| B-009 | `_box` 为 null / 已从文档移除的竖线划过不报错 | 运行时 | 健壮性 |
| B-010 | 目标块已位于视口顶部时划过 → 位置不变（幂等，不抖动） | 运行时 | ② |

## 实现要点（预期，供实现与评审参考）

**改动文件（仅 1 个）**：`web/static/app.js`

- 新增模块级函数 `msgVizLocate(line)`：取 `line._box`（消息块 `.ev`），若不存在或 `!box.isConnected` 则直接返回；否则把该块顶边对齐到滚动区顶边：
  `streamB.scrollTop += box.getBoundingClientRect().top - streamB.getBoundingClientRect().top;`
  — **为什么不用 `box.offsetTop`**：`.stream-b` 是 `position:static`，故 `.ev` 的 `offsetParent` 是 `#stream-wrap`，`offsetTop` 会把 35px 的 `.stream-head` 与 `.stream` 的内边距算进去，导致定位偏低；用「两者矩形差」与 offsetParent 链无关，且能正确处理 `.ev` 的 1px 边框，保证标题行精确贴顶。
- 在 `initMsgViz()` 中为 `#msgVizTrack` 增加**事件委托** `mouseover` 监听：命中 `e.target.closest(".msgviz-line")` 时调用 `msgVizLocate(line)`；`dragging`（拖拽平移中）为真时直接返回，避免与平移手势互相干扰。委托方式使后续 `msgVizFlush()` 绘制的竖线无需逐一挂监听。
- **不改动**：`msgVizFlush()` 的既有 `click` 定位、`initMsgViz()` 的 `pointerdown/move/up` 拖拽平移、`title` tooltip、`msgVizRecolor`。
- **不新增 CSS**（hover 加粗与原生 tooltip 保持原样）。

**交互不变量**
- 指示器位于固定不滚动的 `.stream-head` 内，hover 定位滚动的是 `#streamB`，故指针悬停的目标竖线不会因滚动而移动（不会自激/抖动）。
- 定位滚动会触发 `#streamB` 的 scroll 监听 → `updateFollowState()` 重算 `followOutput`：滚离底部即停止自动跟随（与既有行为一致，属预期）。

## 用例详情

### A. Go 静态断言单测（`web/feature616_test.go`，读 `static/app.js` 断言）

**A-001 `msgVizLocate` 存在并按矩形差求位移**
- 动作：`os.ReadFile("static/app.js")`。
- 断言：含 `function msgVizLocate(line)`；含 `box.getBoundingClientRect().top - streamB.getBoundingClientRect().top`；**不含**「用 `box.offsetTop` 直接赋给 `streamB.scrollTop`」的写法（该写法会因 `.stream-b` 为 `position:static` 而定位偏低）。

**A-002 写入 `streamB.scrollTop`（增量）**
- 断言：含 `streamB.scrollTop +=`（增量赋值，与当前滚动位置无关地求位移）。

**A-003 轨道上存在 mouseover 委托**
- 断言：含 `msgVizTrack.addEventListener("mouseover"`；含 `closest(".msgviz-line")`；命中后调用 `msgVizLocate(`。

**A-004 拖拽守卫**
- 断言：说明 `mouseover` 回调内含 `if (dragging) return;`。

**A-005 `_box` 缺失/已断开时安全返回**
- 断言：`msgVizLocate` 内含 `if (!box || !box.isConnected) return;`。

**A-006 既有行为保留（回归）**
- 断言：仍含既有「点击竖线定位」（`line.addEventListener("click"` 或 `initMsgViz` 的 pointerup 定位分支）与「拖拽平移」（`msgViz.addEventListener("pointerdown"` / `"pointermove"`）代码。

### B. 运行时用例（`--serve --dev` + 调试浏览器）

> 说明：调试浏览器工具没有独立的「悬停」动作。B-001/B-002/B-004/B-006 通过 `browser_click` 在目标竖线坐标上移动真实指针（移动过程浏览器会派发真实 `mouseover`/`click`），页面内预置**探针脚本**记录每次 `mouseover` 后的滚动与对齐数据；B-003/B-005/B-007/B-008/B-009/B-010 用 `browser_evaluate` 量测或直接派发事件。

**B-001 划过竖线 → 标题行对齐消息区顶部**
- 步骤：打开含若干消息块（指示器有多条竖线）的会话；取第 k 条竖线及其 `_box`；把真实指针移到该竖线坐标。
- 断言：`|box.getBoundingClientRect().top - streamB.getBoundingClientRect().top| ≤ 1`（该块标题行位于消息区顶部）。

**B-002 不同竖线分别定位**
- 步骤：对 3 条不同竖线各重复一次 B-001。
- 断言：每次偏差 ≤1px，且定位到的块与竖线 `_box` 一致。

**B-003 移出后保持**
- 步骤：定位到某块后，把指针移到指示器区域之外。
- 断言：`streamB.scrollTop` 与移出前一致（不回滚）。

**B-004 快速划过即时定位**
- 步骤：连续划过 ≥5 条竖线。
- 断言：每次 `mouseover` 都触发一次定位（探针计数的对齐记录数 = 划过条数），无防抖导致的跳过。

**B-005 拖拽平移不触发定位**
- 步骤：在轨道上 `pointerdown` 后水平拖动，拖动过程中划过竖线。
- 断言：拖动期间 `streamB.scrollTop` 不因 hover 改变（仅 `msgVizTrack` 的 `transform` 平移）。

**B-006 点击定位仍可用（回归）**
- 步骤：单击任一竖线。
- 断言：对应块滚动进入消息区视口。

**B-007 轨道空白划过不定位**
- 步骤：向 `#msgVizTrack` 派发 `mouseover`（target 为 track 本身，非竖线）。
- 断言：`streamB.scrollTop` 不变。

**B-008 空指示器不报错**
- 步骤：在无消息块的会话（指示器为空）划过指示器。
- 断言：`browser_console` 无异常，滚动位置不变。

**B-009 失效竖线不报错**
- 步骤：对某条竖线置 `line._box = null` 后划过；另对一个已从文档移除的块对应竖线划过。
- 断言：不抛异常，滚动位置不变。

**B-010 幂等（不抖动）**
- 步骤：对某块使其已位于消息区顶部后，重复划过其竖线 3 次。
- 断言：`streamB.scrollTop` 不变。

## 验证与交付

- 语法检查：`node --check web/static/app.js`。
- 单测：`go test ./web/ -run Feature616`（新增 `web/feature616_test.go`）；随后 `go test ./... -short` 全绿。
- 编译检查：`go build ./... && go vet ./...`。
- 编译产物：按规范编译受影响应用（co-shell）到 `work/co-shell-0.102.0.darwin.arm64`（本机平台）。
- 运行时实测：`work/co-shell-0.102.0.darwin.arm64 --serve --dev --port <p>` + 调试浏览器执行 B-001~B-010，并截图确认「块标题贴顶」的视觉效果。
- 交付：提交到分支 **FEATURE-616**（不合并）；等待用户确认测试通过后再 `/merge`。
