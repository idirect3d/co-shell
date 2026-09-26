# FEATURE-557 验收测试用例：hub UI 优化（抽屉浮动化 + 公告板喇叭标识）

> 需求（用户提出）：
> 1. 宽屏（`min-width:901px`）下 hub 的 Agents 抽屉从左弹出时会把 co-shell 主界面（iframe 舞台）整体向右挤压；窄屏是浮动弹出、不影响右侧内容。现要求**宽屏向窄屏对齐**——抽屉始终**浮动叠加**，弹出时不挤压 co-shell 主界面内容。
> 2. hub **Agents 清单**中，对启用 `board-enabled` 的 agent，在**名字右边**增加一个**聊天气泡矢量图标**（与 co-shell 状态栏「会话列表」图标 `i-sessions` 同款几何）。
>
> 交互决策（经 ask_user 确认）：
> - 版本归属 **v0.76.0**、任务编号 **FEATURE-557**（基于 main，v0.75.0 已合并打 tag）；
> - 「启用 board-enabled」判定口径 = **hub 公告板 roster 注册**：`GET /api/agents` 返回的每个 agent 带 `board_enabled` 字段，取 `w.board.Agent(id)` 是否命中（含 `online=false` 的历史注册项）；**从未注册过公告板的 agent 不显示图标**。
>
> 改动范围（预期实现）：`hub/gateway/webui_static.go`（CSS + `renderList()`）、`hub/gateway/webui.go`（`agentView.BoardEnabled` + `handleListAgents`）、版本入口 `main.go` / `cmd/co-shell-hub/main.go`。

## 用例清单

| 编号 | 名称 | 类型 |
|------|------|------|
| A-001 | `GET /api/agents` 在公告板未挂载时 `board_enabled=false` 且不 panic | 单测 |
| A-002 | 公告板 roster 内的 agent `board_enabled=true` | 单测 |
| A-003 | 从未注册公告板的 agent `board_enabled=false` | 单测 |
| A-004 | roster 内 `online=false`（离线历史项）仍 `board_enabled=true` | 单测 |
| A-005 | 静态资源不含宽屏平移规则 `body.drawer-open #stage` | 单测 |
| A-006 | 静态资源含聊天气泡图标 symbol 与清单渲染/样式挂钩 | 单测 |
| B-001 | 宽屏打开抽屉不再挤压 co-shell 主界面 | 运行时 |
| B-002 | 宽屏抽屉为浮动叠加（覆盖在内容之上，带投影） | 运行时 |
| B-003 | 宽屏抽屉开关交互全部保持可用 | 运行时 |
| B-004 | 宽屏点击遮罩/外部关闭抽屉 | 运行时 |
| B-005 | 宽屏固定（pin）抽屉行为不变 | 运行时 |
| B-006 | 窄屏（≤900px）行为不变（遮罩 + 覆盖） | 运行时 |
| B-007 | 手机屏（≤640px）抽屉占满视口宽（回归） | 运行时 |
| B-008 | Agents 卡片交互回归（切换/启停/滑动删除/箭头） | 运行时 |
| B-009 | roster 内 agent 名称右侧显示聊天气泡 | 运行时 |
| B-010 | 非 roster agent 不显示聊天气泡 | 运行时 |
| B-011 | 图标样式与卡片布局（颜色跟随文字、不撑破省略号） | 运行时 |
| B-012 | 图标不影响名称区点击语义 | 运行时 |
| B-013 | 版本号与 BUILD 同步（CLI/Web 显示） | 运行时 |

---

## A 组：单元测试（Go）

### A-001 `GET /api/agents` 在公告板未挂载时 `board_enabled=false` 且不 panic

- 前置：`WebUI` 未调用 `SetBoard`（`w.board == nil`，测试环境常见）。
- 步骤：请求 `GET /api/agents`。
- 预期：HTTP 200；每个 agent 的 `board_enabled` 均为 `false`；服务不 panic。

### A-002 公告板 roster 内的 agent `board_enabled=true`

- 前置：`WebUI` 挂载公告板；注册 agent `a1`（`board.Register("a1", ...)`）。
- 步骤：请求 `GET /api/agents`。
- 预期：`agents[]` 中 `id == "a1"` 的项 `board_enabled == true`。

### A-003 从未注册公告板的 agent `board_enabled=false`

- 前置：registry 中存在 agent `a2`，公告板 roster 中没有它。
- 步骤：请求 `GET /api/agents`。
- 预期：`a2` 的 `board_enabled == false`。

### A-004 roster 内 `online=false`（离线历史项）仍 `board_enabled=true`

- 前置：`a1` 曾注册（roster 有记录），随后被置为 offline（`MarkOffline` / 重连前状态）。
- 步骤：请求 `GET /api/agents`。
- 预期：`a1` 的 `board_enabled == true`（roster 命中即显示，不要求在线）。

### A-005 静态资源不含宽屏平移规则 `body.drawer-open #stage`

- 步骤：检查内嵌页面常量 `webIndexHTML`。
- 预期：
  1. **不含** `body.drawer-open #stage` 相关 `left:` 平移规则（含 `@media (min-width:901px)` 内的那条）；
  2. `#stage` 保持 `inset:0` 全屏定位（不再有抽屉打开时被右推的路径）。

### A-006 静态资源含聊天气泡图标 symbol 与清单渲染/样式挂钩

- 步骤：检查内嵌页面常量 `webIndexHTML`。
- 预期：
  1. 存在聊天气泡矢量图标定义（`<symbol id="i-board">`，几何与 co-shell 状态栏 `i-sessions` 一致，`viewBox` + `currentColor` 描边/填充）；
  2. `renderList()` 生成的卡片 HTML 中，`board_enabled` 为真时名称后追加该图标（`<use href="#i-board-*">`）；
  3. 有对应 CSS 类（图标尺寸以内联样式或 `.ico` 系列约束），且不新增外部资源请求。

---

## B 组：运行时验收（浏览器实测）

### B-001 宽屏打开抽屉不再挤压 co-shell 主界面

- 前置：浏览器窗口宽度 > 900px（如 1512×897）；hub 已连接至少 1 个 agent，主区域显示该 co-shell 的 iframe。
- 步骤：
  1. 记录主界面中若干锚点元素的屏幕位置（如 co-shell 标题栏 logo、输入框左边缘、消息区左边缘）；
  2. 鼠标移到屏幕左边缘停留（或将指针移入 hub 徽标并点击）打开 Agents 抽屉。
- 预期：
  1. 上述锚点元素**位置与宽度完全不变**（无右移、无横向压缩）；
  2. iframe 内容不发生重排（co-shell 内不出现横向滚动条）；
  3. 抽屉结束后 iframe 仍保持满屏占位。

### B-002 宽屏抽屉为浮动叠加（覆盖在内容之上，带投影）

- 前置：同 B-001。
- 步骤：打开抽屉，观察抽屉与主界面交界。
- 预期：
  1. 抽屉为 `position:fixed` 浮层，宽度 480px（`--panel-w`），位于主界面**之上**（z-index 高于 iframe），左侧内容被遮挡而非被推走；
  2. 抽屉右边界有投影/分隔线，视觉上呈现「浮起」；
  3. 抽屉顶部仍在 co-shell 标题栏之下（不遮挡标题栏，保持 FEATURE-515 现状）。

### B-003 宽屏抽屉开关交互全部保持可用

- 预置：宽屏。
- 步骤：依次执行 ① 左边缘热区悬停约 1s 打开；② 移出抽屉自动收起（未固定时）；③ 再次打开；④ 点击 hub 徽标切换开关；⑤ 抽屉内 Agents/Chat 面板折叠切换。
- 预期：各操作均正常生效；打开/关闭动画顺滑（位移过渡不抖动）；无 JS 报错（控制台干净）。

### B-004 宽屏点击遮罩/外部关闭抽屉

- 前置：宽屏，抽屉已打开（未固定）。
- 步骤：
  1. 点击抽屉右侧的主界面区域；
  2. 观察抽屉状态。
- 预期：抽屉关闭（沿用 FEATURE-492 的「点击外部关闭」语义）。
- 说明：宽屏由「推移」改为「覆盖」后，遮罩点击区域是否仍按原设计生效需实测确认（窄屏遮罩 `#scrim.show` 行为不受影响）。

### B-005 宽屏固定（pin）抽屉行为不变

- 前置：宽屏。
- 步骤：打开抽屉 → 点击徽标右侧 pin 图标 → 鼠标移出抽屉 → 观察。
- 预期：抽屉保持打开（pin 生效，颜色高亮、图标旋转），再次点击 pin 取消固定后移出即自动收起。

### B-006 窄屏（≤900px）行为不变（遮罩 + 覆盖）

- 前置：窗口宽度 ≤ 900px（如 800px），且 > 640px。
- 步骤：打开抽屉。
- 预期：
  1. 主界面不移动（与改造前一致）；
  2. 抽屉浮在内容之上并显示半透明遮罩（`#scrim.show`）；
  3. 投影/边界视觉效果与宽屏一致（改造后两档统一）。

### B-007 手机屏（≤640px）抽屉占满视口宽（回归）

- 前置：窗口宽度 ≤ 640px（或使用移动端模拟）。
- 步骤：打开抽屉。
- 预期：抽屉宽 = 100vw；打开的徽标条宽同步 100vw（FEATURE-537 行为保持）。

### B-008 Agents 卡片交互回归（切换/启停/滑动删除/箭头）

- 前置：宽屏 + 抽屉打开，Agents 面板展开；至少 1 个 managed agent、1 个 external agent。
- 步骤：
  1. 点击 managed agent 卡片 → 主界面切换到该 agent 的 co-shell；
  2. 点击卡片右侧开关 → 状态灯与开关状态变化（启动/停止）；
  3. 左滑卡片 → 露出「删除」按钮；
  4. 点击卡片右侧 `›` → 打开该 agent 的只读详情/配置视图。
- 预期：四项均正常；抽屉浮动化改造未破坏卡片事件（含「点击外部关闭」与卡片点击的优先级）。

### B-009 roster 内 agent 名称右侧显示聊天气泡（与 co-shell 状态栏会话图标同款）

- 前置：至少一个 agent 已启用 `board_enabled=true` 并已向 hub 注册（roster 中存在该 agent，`online` 可为 true 或 false）。
- 步骤：打开 Agents 抽屉，观察该 agent 卡片名称。
- 预期：
  1. 名称右侧紧邻显示一个聊天气泡矢量图标（与 co-shell 状态栏「会话列表」图标外观一致；非 emoji、非字体图标）；
  2. 图标为矢量绘制，缩放不模糊；`title`/`tooltip` 语义合理（如「已启用公告板」）。

### B-010 非 roster agent 不显示聊天气泡

- 前置：存在一个从未注册公告板的 agent（如 `board_enabled` 关闭的 managed agent，或未接入公告板的外部 agent）。
- 步骤：观察其卡片。
- 预期：名称右侧**没有**喇叭图标；其余卡片信息（状态灯、开关、箭头）不受影响。

### B-011 图标样式与卡片布局（颜色跟随文字、不撑破省略号）

- 前置：存在一个名称较长的 roster agent。
- 步骤：观察卡片。
- 预期：
  1. 图标颜色随名称文字色（`currentColor`），深色/浅色主题下均清晰可见；
  2. 图标尺寸与卡片内既有图标（开关/箭头）协调，不高于名称行高；
  3. 名称仍按现有规则以省略号截断，图标固定显示在截断文本之后，不换行、不撑破卡片。

### B-012 图标不影响名称区点击语义

- 步骤：分别点击 ① 图标本身；② 图标左侧名称文字。
- 预期：两者都触发「切换到该 agent」的卡片点击语义，不会误打开配置视图（`›`）或误触发开关/滑动删除。

### B-013 版本号与 BUILD 同步（CLI/Web 显示）

- 步骤：
  1. `work/co-shell-0.76.0.darwin.arm64 --version`（或等效命令）；
  2. 启动 `work/co-shell-hub-0.76.0.darwin.arm64 --serve` 并查看徽标版本号。
- 预期：co-shell 与 co-shell-hub 均显示 `0.76.0`，BUILD 号一致；`main.go` 与 `cmd/co-shell-hub/main.go` 中的版本/构建号同步（`[BUILD-1126]` 起）。

---

## 验收判定

- **必须通过**：A-001 ~ A-006（自动）、B-001、B-002、B-003、B-006、B-007、B-008、B-009、B-010、B-011、B-012。
- **回归确认**：B-004、B-005、B-013。
- 全部通过后由用户确认合入 main（版本 v0.76.0）。
