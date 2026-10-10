# FEATURE-626 UC-0001：hub 体验优化 —— agent「自动启动」开关 + 欢迎页改造

> 分支：**FEATURE-626**　版本：**v0.108.1**　BUILD：**待编译**
> 需求（用户 2026-10-09 23:35 提出）：
> ① agent 设置中增加**自动启动滑动开关**，打开后 hub 启动时自动启动该 agent，hub **自动切换到排在最前面自动启动的那个 agent**；
> ② 欢迎页/无 agent 提示（原「暂无 Agent。点击上方"运行"箭头，或左上角 co-shell-hub 徽标再点"管理"创建或添加 Agent。」）改为**两个选项**：「{+号矢量图} 新建智能体」与「{运行三角箭头} 启动全部智能体」；
> ③ 当**没有 agent 被自动启动**时，和没有 agent 时一样，**显示欢迎页**。
>
> 动机：配合 `co-shell.app`（FEATURE-625）的「双击即用」体验——双击 App 后应直接进入可用智能体，而不是停在空壳界面。
>
> 现状调研：`AutoStart` 字段只存在于 `hub/hub.go`（legacy hub，用于 `cmd/co-shell-hub-legacy`），实际运行的 `hub/gateway/` 无此能力，本次为全新实现。

## 术语

- **自动启动（auto_start）**：agent 的一个布尔配置，为真时 hub 启动后自动拉起该 agent。
- **自动切换**：hub 启动并完成自动启动后，前端自动选中"列表中**最靠前**的已自动启动 agent"。
- **欢迎页**：agent 列表为空（或无可用 agent）时展示的引导界面。
- `(M)` = 需运行时手动/浏览器断言；`(R)` = 离线静态校验。

## 设计基线（本次实现基准，供确认）

| 项 | 设计 |
|----|------|
| 数据字段 | `hub/gateway` 的 agent 规格新增 `auto_start`（bool，`omitempty`），随 registry 持久化到 `hub-agents.json` |
| 后端更新接口 | 复用 `PUT /api/agents/{id}`（`handleUpdateAgent`）支持更新 `auto_start` |
| 自动启动时机 | hub 启动、加载 registry 之后，遍历并启动所有 `auto_start=true` 的 agent |
| 自动切换 | 前端在收到 agent 列表后，若存在 `auto_start` 的 agent，则自动选中列表中**最靠前**的那个 |
| 欢迎页触发条件 | ① agent 列表为空；**或** ② 列表中没有任何 `auto_start=true` 的 agent（即无自动启动） |
| 欢迎页内容 | 两个可点选项：「**+ 新建智能体**」与「**▶ 启动全部智能体**」 |
| 前端开关 | agent 设置面板新增滑动开关，样式与既有开关一致 |
| i18n | 新增文案均提供中英双语 |

## 用例清单

### UC-01 (R) agent 规格含 auto_start 且可持久化

- 断言：`hub/gateway` 中 agent 规格结构含 `auto_start` 字段（JSON tag `auto_start`）。
- 断言：创建/更新 agent 后，`hub-agents.json` 中对应条目出现（或正确反映）`auto_start`。
- 断言：`auto_start=false` 时不写入该键（`omitempty` 生效）。

### UC-02 (R) 更新接口支持 auto_start

- 断言：`PUT /api/agents/{id}` 可接收并落地 `auto_start`，返回体反映最新值。

### UC-03 (R) 编译与静态检查通过

- 断言：`go build ./... && go vet ./...` 通过。
- 断言：hub 包既有单元测试除既有基线失败外无新增失败。

### UC-04 (M) agent 设置面板存在「自动启动」滑动开关

- 步骤：打开 hub Web UI → 展开某 agent 的设置面板。
- 断言：存在标签为「自动启动」的滑动开关；初始状态与该 agent 的 `auto_start` 一致。

### UC-05 (M) 开关状态持久化

- 步骤：把某 agent 的「自动启动」打开 → 刷新页面。
- 断言：开关仍为打开状态；`hub-agents.json` 中该 agent 的 `auto_start` 为 `true`。

### UC-06 (M) hub 启动时自动启动标记的 agent

- 前置：agent A 设 `auto_start=true` 且处于停止状态。
- 步骤：重启 hub。
- 断言：hub 启动后 A 自动被启动（`running=true`），无需手动点「运行」。

### UC-07 (M) 多个自动启动时自动切换到列表最前者

- 前置：agent A、B、C 中 B 与 C 设 `auto_start=true`，列表顺序为 A、B、C。
- 步骤：重启 hub，打开 Web UI。
- 断言：B 与 C 均被自动启动；**当前选中/展示的是 B**（列表中最靠前的自动启动项）。

### UC-08 (M) 无 agent 时显示欢迎页与两个选项

- 前置：agent 列表为空（清空 registry 或使用全新数据目录）。
- 断言：欢迎页显示，且包含「**+ 新建智能体**」与「**▶ 启动全部智能体**」两个可点选项；不再出现旧的「点击上方"运行"箭头…」纯文案。

### UC-09 (M) 点「+ 新建智能体」进入新建流程

- 步骤：在欢迎页点击「+ 新建智能体」。
- 断言：进入新建 agent 的界面/弹窗（与既有点「管理」后新建的入口一致）。

### UC-10 (M) 点「▶ 启动全部智能体」启动全部 agent

- 前置：存在 2 个以上处于停止状态的 agent。
- 步骤：在欢迎页点击「▶ 启动全部智能体」。
- 断言：全部 agent 被启动（`running=true`）；界面离开欢迎页进入可用状态。

### UC-11 (M) 有 agent 但无自动启动 → 显示欢迎页

- 前置：存在 agent（列表非空），但**没有任何** agent 设 `auto_start=true`。
- 步骤：重启 hub 并打开 Web UI。
- 断言：显示的是**欢迎页**（与无 agent 时一致），而非某个 agent 的界面。

### UC-12 (M) 自动启动未成功时仍显示欢迎页

- 前置：某 agent 设 `auto_start=true`，但其 workspace 不存在或启动会失败。
- 步骤：重启 hub。
- 断言：不因启动失败而卡死或报错崩溃；界面仍显示欢迎页（无可用自动启动 agent）。

### UC-13 (R) 新增文案具备中英双语

- 断言：欢迎页两个选项、设置面板开关标签在 `zh`/`en` 两套文案中均有对应条目。

## 备注

- **UC-12 为健壮性用例**：若最小实现暂不处理启动失败场景，须在交付说明中显式标注「未覆盖」。
- UC-08/11/12 的"显示欢迎页"判定，以「欢迎页容器可见且含两个选项」为准。
- 本任务不改动 co-shell 主程序，仅改 `hub/gateway`。
