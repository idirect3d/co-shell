# FEATURE-613 验收测试用例（UC-0001）：hub agent 列表拖动排序

> 权威依据：`ROADMAP.md` v0.101.0 / FEATURE-613；分支 **FEATURE-613**，版本 **v0.101.0**。
> 相关代码：`hub/gateway/manager.go`（agent registry 与落盘）、`hub/gateway/webui.go`（HTTP API）、`hub/gateway/webui_static.go`（hub 内嵌单页前端）。

> 已确认决策（用户确认，不可改动）：
> ① **持久化到 `hub-agents.json`**——新增后端排序 API，重启 hub 后保持、多端一致；
> ② **长按约 300ms 后进入排序拖动**：长按卡片再上下拖动即可排序；未长按的普通拖动仍用于滚动列表，水平左滑仍为删除；
> ③ 版本归属 **v0.101.0**（FEATURE 走 minor 递增），任务编号 **FEATURE-613**。

> 已确认权衡（用户确认）：为避免抢占列表滚动，排序手势改为**长按约 300ms 后**才进入——普通拖动（含触屏滑动）仍用于滚动列表，水平左滑仍为删除；长按期间指针移动超过阈值或提前抬起都会取消长按。

> 已知限制（本次不做）：拖动到列表边缘不自动滚动（长列表请先滚动到目标位置再长按拖动）。

## 用例清单

| 编号 | 名称 | 类型 | 对应需求 |
|------|------|------|----------|
| A-001 | `Manager.Reorder` 按给定 id 顺序重排并落盘 `hub-agents.json` | 单测 | ① |
| A-002 | `Reorder` 拒绝 id 数量不足（缺 id） | 单测 | ① |
| A-003 | `Reorder` 拒绝未知 id / 多余 id | 单测 | ① |
| A-004 | `Reorder` 拒绝重复 id | 单测 | ① |
| A-005 | `Reorder` 的 id 比较大小写不敏感（与 `findAgentLocked` 口径一致） | 单测 | ① |
| A-006 | `Reorder` 校验失败时不改变内存中的顺序（原子性） | 单测 | ① |
| A-007 | `Reorder` 后 `Agents()` 顺序即新顺序，且重新 `NewManager` 载入一致 | 单测 | ① |
| A-008 | `PUT /api/agents/order` 合法请求 → 200 且顺序已持久化 | API 单测 | ① |
| A-009 | `PUT /api/agents/order` 非法 JSON → 400 | API 单测 | ① |
| A-010 | `PUT /api/agents/order` 未知 / 缺失 id → 400，且顺序不变 | API 单测 | ① |
| A-011 | 路由优先级：`/api/agents/order` 不被 `PUT /api/agents/{id}` 吞掉 | API 单测 | ① |
| B-001 | 前端含拖动排序实现（排序入口函数 + 拖动类名 + 上报接口） | 静态断言 | ② |
| B-002 | 长按门槛：仅长按约 300ms 后进入排序；未长按的水平滑动仍走 swipe 删除 | 静态断言 | ② |
| B-003 | 拖动期间暂停 3s 轮询的列表重建（避免打断拖动/覆盖新顺序） | 静态断言 | ② |
| B-004 | 松手后把新顺序上报后端；失败则回退原顺序 | 静态断言 | ①② |
| B-005 | 排序后 `agentsSignature` 能感知顺序变化（顺序参与重建签名） | 静态断言 | ② |
| B-006 | 长按取消条件：阈值内移动 / 提前抬起取消长按，不进入排序 | 静态断言 | ② |
| C-001 | 运行时：把第二个 agent 拖到第一个上方，列表顺序随之改变 | 运行时 | ② |
| C-002 | 运行时：刷新页面后新顺序保持 | 运行时 | ①② |
| C-003 | 运行时：重启 hub 后新顺序保持（验证 `hub-agents.json`） | 运行时 | ① |
| C-004 | 运行时：左滑仍能露出删除按钮，且不触发排序 | 运行时 | ② 回归 |
| C-005 | 运行时：点击卡片仍能切换 agent（拖动逻辑不误伤点击） | 运行时 | 回归 |
| C-006 | 运行时：拖动过程中列表不因 3s 轮询重建而闪回旧顺序 | 运行时 | ② |
| C-007 | 运行时：拖动排序后再删除一个 agent，剩余顺序保持相对关系 | 运行时 | ① |
| C-008 | 运行时：长按后未移动即松手 → 顺序不变（不误触发） | 运行时 | ② |

## 实现要点（预期，供实现与评审参考）

**后端**
- `hub/gateway/manager.go` 新增 `Reorder(ids []string) error`：按 `ids` 给出的顺序重排 `m.agents` 并 `save()` 落盘。校验口径与 `findAgentLocked` 一致（`strings.EqualFold` 大小写不敏感、`TrimSpace`）；`ids` 必须**不重不漏**地覆盖全部已注册 agent，否则返回错误且**不修改内存**。
- `hub/gateway/webui.go` 新增 `PUT /api/agents/order`（body：`{"ids":["a","b"]}`），调用 `Reorder`；成功 200，非法请求 400（复用 `writeJSON` + `agentErrorStatus` 的既有风格）。
- 注意 Go 1.22+ `ServeMux` 优先级：字面量模式 `/api/agents/order` 比 `/api/agents/{id}` 更具体，故 `PUT /api/agents/order` 不会被 `handleUpdateAgent` 吞掉（A-011 断言此点）。

**前端（`hub/gateway/webui_static.go` 内嵌单页）**
- 每个 `.agent-wrap` 记 `data-id`；新增排序手势：`mousedown`/`touchstart` 后启动约 300ms 长按定时器——期间指针移动超过阈值或提前抬起即取消长按（交回既有滚动 / `attachSwipe` 左滑删除路径）；长按到时则进入排序（卡片浮起、跟随指针、与兄弟元素实时 `insertBefore` 重排），进入排序后水平移动不再触发删除。
- 拖动期间置 `sorting` 标志：`refresh()` 在 `sorting` 为真时跳过 `renderList()`，避免 3s 轮询打断拖动或覆盖新顺序。
- 松手后收集 `listEl` 内的 `data-id` 新顺序，`PUT /api/agents/order` 上报；成功则同步本地 `agents` 数组顺序，失败则恢复原顺序并 `renderList()`。
- 保持既有不变量：点击卡片切换 agent、点击 chevron 打开配置、`_moved` 抑制"拖动结束后的误点击"。不破坏 FIX-597 的监听器配对要求（`listen`/`unlisten` 成对、无泄漏）。

## 用例详情

### A. 后端单测（`hub/gateway/`）

**A-001 `Manager.Reorder` 按给定顺序重排并落盘**
- 前置：`NewManager(ctx, tmp/hub-agents.json, ...)`，创建 3 个 agent（a、b、c，创建顺序即初始顺序）。
- 动作：`Reorder([]string{"c","a","b"})`。
- 断言：`Agents()` 顺序为 c、a、b；重新读取 `hub-agents.json` 解析后顺序一致。

**A-002 拒绝 id 数量不足**
- 动作：`Reorder([]string{"a","b"})`（共 3 个 agent）。
- 断言：返回错误；`Agents()` 仍为原顺序。

**A-003 拒绝未知 / 多余 id**
- 动作：`Reorder([]string{"a","b","zzz"})`（`zzz` 未注册）。
- 断言：返回错误；顺序不变。

**A-004 拒绝重复 id**
- 动作：`Reorder([]string{"a","a","b"})`（长度 3 但重复）。
- 断言：返回错误；顺序不变。

**A-005 大小写不敏感**
- 前置：注册 id 为 `Agent-1`。
- 动作：`Reorder([]string{"agent-1"})`。
- 断言：成功，顺序不变（与 `findAgentLocked` 口径一致）。

**A-006 校验失败不改变内存顺序（原子性）**
- 动作：一次非法 `Reorder`。
- 断言：失败后 `Agents()` 与调用前逐项相等，且磁盘文件未变化。

**A-007 重排后重新载入一致**
- 动作：`Reorder` 后构造新的 `NewManager` 指向同一文件。
- 断言：新 Manager 的 `Agents()` 顺序等于重排后的顺序。

### B. HTTP API 单测（`hub/gateway/`）

**A-008 合法请求 → 200 且持久化**
- 动作：`PUT /api/agents/order`，body `{"ids":["b","a"]}`，`httptest.NewRecorder()`。
- 断言：状态码 200；`hub-agents.json` 顺序为 b、a。

**A-009 非法 JSON → 400**
- 动作：body 为 `{`。
- 断言：状态码 400。

**A-010 未知 / 缺失 id → 400 且顺序不变**
- 动作：body `{"ids":["a"]}`（缺 id）与 `{"ids":["a","zzz"]}`（未知 id）各一次。
- 断言：均 400；顺序不变。

**A-011 路由优先级**
- 动作：`mux` 上分别请求 `PUT /api/agents/order` 与 `PUT /api/agents/{id}`。
- 断言：前者进入 `handleReorderAgents`（不会被当作 id=`order` 的 agent 更新）。

### C. 前端静态断言（`hub/gateway/`，参照 `fix597_test.go` / `fix560_test.go` 的 `feat559ReadFile(webIndexHTML)` 模式）

**B-001 拖动排序实现存在**
- 断言：页面含排序入口（如 `function attachSort(`）与拖动样式类（如 `.agent-wrap.sorting` 或等价类名），且包含 `PUT`/`/api/agents/order` 上报调用。

**B-002 长按门槛**
- 断言：排序只由**约 300ms 长按**触发（存在长按时长常量/字面量）；未长按的路径仍保留既有 `setSwipe(...)`（删除手势），二者互斥。

**B-003 轮询让步**
- 断言：`refresh()` 中存在 `sorting`（或等价）标志判断，拖动期间不执行 `renderList()`。

**B-004 失败回退**
- 断言：上报后的错误分支会恢复原顺序（重新 `renderList()` 或回滚本地数组）。

**B-005 顺序参与签名**
- 断言：`agentsSignature()` 的签名随列表顺序变化而变化（顺序敏感），保证他端改动顺序时本端能重建。

**B-006 长按取消条件**
- 断言：长按定时器在指针移动超过阈值或提前抬起时被清除，且清除路径不会进入排序；长按到时才置排序标志。

### D. 运行时用例（浏览器实操，`--serve` + 调试浏览器）

**C-001 长按拖动改变顺序**
- 步骤：hub 页面打开 agent 列表；长按第二个卡片约 300ms 直到其浮起，再向上/向下拖到目标位置后松手。
- 断言：列表 DOM 顺序改变，且目标位置正确。

**C-002 刷新后保持**
- 步骤：刷新页面并等待一次轮询。
- 断言：顺序为拖动后的顺序。

**C-003 重启后保持**
- 步骤：重启 hub 进程，重新打开页面。
- 断言：顺序仍为拖动后的顺序；`hub-agents.json` 内数组顺序一致。

**C-004 左滑仍是删除**
- 步骤：在卡片上水平左滑。
- 断言：删除按钮按原行为露出，且**未**触发排序（顺序不变）。

**C-005 点击仍切换 agent**
- 步骤：单击任一卡片。
- 断言：主视图切换到该 agent，顺序不变。

**C-006 拖动不被轮询打断**
- 步骤：拖动中跨过 ≥1 个 3s 轮询周期再松手。
- 断言：拖动过程中列表不闪回旧顺序。

**C-007 删除后相对顺序保持**
- 步骤：拖动排序后，左滑并删除其中一个 agent。
- 断言：剩余 agent 相对顺序不变。

**C-008 长按未移动 → 不改变顺序**
- 步骤：长按某卡片约 300ms 后（不移动）立即松手。
- 断言：列表顺序不变，且不产生多余的上报请求（或上报后顺序等价）。
