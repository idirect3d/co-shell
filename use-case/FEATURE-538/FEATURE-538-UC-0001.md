# FEATURE-538 用例：hub 的「Agent 描述」改接工作空间 IDENTITY.md（+ ID 唯一、公告板口径）（分支 FEATURE-538 / v0.68.0）

> 分支：**FEATURE-538**（基于 main，v0.67.0 已合并 / tag `v0.67.0`）
> 版本：v0.68.0　　任务编号：FEATURE-538　　基线 BUILD：1077

## 背景（改造前的问题）

1. hub 侧 `AgentSpec.Name` 在界面上叫「职责描述」，实际存储在 `hub-agents.json` 里，与 co-shell 工作空间 `IDENTITY.md` 的 `agent-description` **是两份数据**，容易不一致。
2. 公告板 roster 的「职责」优先取 hub 的该字段（`cmd/co-shell-hub/main.go` 的 duty resolver），若只填了 id 就会把 **agent id 当成职责**显示（实测：6 个 agent 全部 `duty_source=hub` 且 duty==id）。
3. 公告板 roster 的展示名是 agent 自报名字（可能重名，实测 `co-flow` 与 `researcher` 都叫 `work`），缺少唯一标识的展示口径。
4. 新增 external agent 的路径没有 ID 重复/格式校验；managed 路径仅校验重复。

## 实现（改动面）

- `hub/gateway/manager.go`：`AgentSpec` 去掉 `Name` 字段（registry 不再存储）；`CreateManaged` / `Update` / `AddExternal` 统一走 ID 校验（非空、禁空格与 `/ \ : * ? " < > |`、大小写不敏感去重）。
- `hub/gateway/identity.go`（新增）：hub 侧极简 IDENTITY.md 读写（`## agent-description` 小节；读取/写回时保留其它小节与自由文本）。
- `hub/gateway/webui.go`：新增 `GET /api/agents/{id}/identity`（实时读取）与 `PUT /api/agents/{id}/identity`（写回）；create/add-external 的错误信息回传给前端。
- `hub/gateway/webui_static.go`：三处标签「职责描述」→「Agent描述」；external 该输入框只读置灰并给出提示；保存后提示「已写入 {workspace}/IDENTITY.md，即时生效（下一轮对话）」，不再提示重启；列表行显示自报名字、悬停（title）显示 ID。
- `hub/gateway/board.go` + `cmd/co-shell-hub/main.go`：roster 的职责取值改为「managed 读 `{workspace}/IDENTITY.md` 的 `agent-description`，external 用 agent 上报值」。
- `web/session.go`：公告板上报的 duty 由 `cfg.LLM.AgentDuty` 改为 `ag.IdentityDescription()`（external 也能正确显示 Agent 描述）。

## 设计口径（已与用户确认）

1. **仅 managed**：hub 直接读写 `{workspace}/IDENTITY.md`；**external 只读置灰**（提示由对方在自己工作空间的「身份与个性」里修改）。
2. **不再存储**：registry 里删除该字段（`AgentSpec.Name`），旧 JSON 中的 `name` 字段被忽略（`json.Unmarshal` 忽略未知字段），不做迁移。
3. **实时读取**：每次打开 agent 设置/详情界面时由 hub 现读文件，不使用缓存。
4. **保存提示**：写回文件后提示「即时生效（下一轮对话生效）」，**不要求重启**（IDENTITY.md 每轮构建提示词时读取）。
5. **ID 唯一**：新增 agent（managed 与 external 两条路径）均校验；不区分大小写去重；非法字符（空格、`/ \ : * ? " < > |`）拒绝并给出明确错误。
6. **公告板口径**：唯一标识 = hub 配置的 ID（API 一律用 ID）；界面（hub agent 列表 / agent 侧 roster）主要显示 agent 自报名字，鼠标悬停显示 ID；「职责」= `IDENTITY.md` 的 `agent-description`（external 用其上报值，上报来源同为此字段）。

---

## UC-0001（静态）界面标签与只读态

**前置**：`hub/gateway/webui_static.go` 为当前分支版本。

**预期**：
- 新建 managed 表单（`#m-name`）、新增 external 表单（`#e-name`）、agent 详情（`#d-name`）三处标签均为「Agent描述」，且**不再**出现「职责描述」字样；
- external 详情中的该输入框带 `readonly`（或 `disabled`）并且有「外部 agent 由对方修改」类提示；
- 新增 managed 表单里不再有「ID 唯一」以外的名称/备注输入（原 `#m-name` 改为 Agent 描述语义，不参与 registry 存储）；
- 列表行渲染 `name || ID` 且 `title` 属性为 ID。

**验证方式**：`grep -n "Agent描述\|职责描述\|readonly" hub/gateway/webui_static.go`。

---

## UC-0002（静态）接口与路由

**前置**：同 UC-0001。

**预期**：`hub/gateway/webui.go` 中存在
- `GET /api/agents/{id}/identity`（返回 `{agent_id, agent_description, source, readonly, workspace}`）；
- `PUT /api/agents/{id}/identity`（body `{"agent_description": "..."}`，成功 200；非 managed 返回 409/400 并提示只读；agent 不存在 404；字段超长 400）。

**验证方式**：`grep -n "api/agents/{id}/identity" hub/gateway/webui.go`。

---

## UC-0003（单测）IDENTITY.md 读写（hub 侧实现）

**前置**：`go test ./gateway/ -run Identity`。

**预期**：
- `identityRead(dir)`：能解析 `## agent-description` 下的文本；文件缺失/无该小节 → 返回空串且不报错；
- `identityWrite(dir, v)`：文件中已有该小节 → 原地替换（其它小节与自由文本逐字保留）；无该小节 → 追加新小节且不改动原有内容；空值 → 写入空（保留标题）；
- 往返测试：写入后读取一致；
- 路径安全：`dir` 为空或不存在时返回错误，不创建意外文件。

**验证方式**：新增 `hub/gateway/identity_test.go`，table-driven。

---

## UC-0004（单测）ID 唯一与格式校验

**预期**：
- managed：重复 ID（完全相同 / 仅大小写不同）→ 报错（HTTP 409）；
- external：同样规则，重复/大小写变体均被拒；
- 非法 ID：空、含空格、含 `/ \ : * ? " < > |` → 报错（HTTP 400）；
- 合法 ID：`agent-1`、`co_flow.v2` 等通过。

**验证方式**：`hub/gateway/manager_test.go` 扩展 + `go test ./gateway/`。

---

## UC-0005（单测）registry 不再存储名称/职责

**预期**：
- `AgentSpec` 结构体不再含 `Name` 字段（`grep -n "Name" hub/gateway/manager.go` 只剩注释或无关字段）；
- 旧 registry JSON（含 `"name": "xxx"`）仍能正常加载（未知字段被忽略），加载后列表不报错；
- `git diff` 中 `webui.go` / `cmd/co-shell-hub/main.go` 不再有 `req.Name` / `spec.Name` 的读写。

**验证方式**：`hub/gateway/manager_test.go` 增加“旧 JSON 带 name 字段可加载”的用例。

---

## UC-0006（运行时）设置界面实时显示与保存写回 IDENTITY.md

**前置**：hub 以 `--serve` 启动（临时实例，独立 registry / board-db）；存在 1 个 managed agent，其工作空间 `{ws}` 内有 `IDENTITY.md`。

**操作**：
1. 打开 hub → Agents 面板 → 进入该 agent 详情；
2. 观察「Agent描述」输入框的值；
3. 修改为一个新值 → 点「保存修改」；
4. 用 `cat {ws}/IDENTITY.md` 复核文件。

**预期**：
- 进入界面时输入框显示的是 `{ws}/IDENTITY.md` 中 `agent-description` 的**当前值**（先手动改文件为 `X`，界面应显示 `X` —— 证明是实时读取而非缓存/registry）；
- 保存成功后界面提示「已写入 …/IDENTITY.md，即时生效（下一轮对话生效）」，**不出现**「需重启」字样；
- `cat` 显示 `## agent-description` 小节内容已被替换为新值，且文件中其它小节（`agent-name` / `user-name` 等）逐字保留；
- 再次打开详情，显示新值。

**验证方式**：`curl GET/PUT /api/agents/{id}/identity` + `cat IDENTITY.md` + 浏览器截图。

---

## UC-0007（运行时）external agent 的「Agent描述」只读

**前置**：hub 中已添加 1 个 external agent（host/port）。

**预期**：
- 详情界面中「Agent描述」输入框为只读/置灰，并提示“外部 agent 无本地工作空间，请在对方 co-shell 的『身份与个性』中修改”；
- 直接调用 `PUT /api/agents/{id}/identity` 返回 409（或 400）且带明确错误信息，不会写任何文件；
- `GET` 对 external 返回其公告板上报值（若在线）或空串 + `readonly: true`。

**验证方式**：`curl -X PUT` 观察状态码 + 浏览器截图。

---

## UC-0008（运行时）新增 agent 的 ID 唯一与格式校验

**操作**：在「新建 Agent」中分别提交：① 与现有 agent 相同的 ID；② 仅大小写不同的 ID；③ 含空格的 ID；④ 含 `/` 的 ID；⑤ 合法新 ID。external 表单同样试一遍 ①②③。

**预期**：
- ① ② → 提示 ID 已存在（HTTP 409），页面显示错误文案，不创建；
- ③ ④ → 提示 ID 含非法字符（HTTP 400），不创建；
- ⑤ → 创建成功，列表出现新行；restart hub 后仍存在（已落 registry）。

**验证方式**：`curl -X POST /api/agents` 与 `/api/agents/external` + `cat hub-agents.json`。

---

## UC-0009（运行时）公告板标识与职责口径

**前置**：hub 与至少 2 个 agent 在线；managed agent 的 `IDENTITY.md` 已写有 `agent-description`。

**预期**：
- `GET /api/board/conversations` 的 `roster[]`：`id` = hub 配置的 ID（唯一标识）；`duty` = 该 agent 的 `agent-description`（managed 由 hub 读文件；external 由 agent 上报）；
- 修好 `duty_source`：managed agent 命中文件值时为 `hub`，external 为上报表述时为 `agent`；
- agent 侧系统提示词里的 `<board_agents>`：显示**自报名字**（可能重名），唯一性由 `id` 属性承载；名字与 ID 不一致时，prompt 文本可同时看到 ID（避免撞名歧义）；
- hub Web UI 的 Agents 列表：主显示自报名字（离线时回退 ID），`title` 悬停显示 ID。

**验证方式**：`curl /api/board/conversations | jq '.roster[]'` + 对比 `cat {ws}/IDENTITY.md` + 浏览器悬停截图。

---

## UC-0010（回归）构建与既有功能

**预期**：
- `hub/`：`go build ./... && go vet ./... && go test ./gateway/` 全绿（含新增 identity/ID 校验用例）；
- 根目录：`go build ./... && go vet ./...` 通过；
- FEATURE-537 的 Chat 两级视图/发言/窄屏功能不受影响（同一前端文件，需回归截图）；
- `main.go` 与 `cmd/co-shell-hub/main.go` 的版本号/build 同步为 0.68.0 / 1077。

---

## 实测记录（完成后填写）

| 用例 | 结果 | 证据 |
|------|------|------|
| UC-0001 静态标签与只读态 | ✅ 通过 | `grep -n "Agent描述\|职责描述" hub/gateway/webui_static.go`：L424/L436/L451 三处均为「Agent描述」，全文件无「职责描述」；`#e-name` 带 `readonly`（L436）+ 提示行（L437）；列表行 `title=ID`（L938）；详情标题 `detailTitle = a.id + ' · 修改'`（L1084） |
| UC-0002 接口与路由 | ✅ 通过 | `hub/gateway/webui.go` L83-84 注册 `GET/PUT /api/agents/{id}/identity`；运行时状态码 200/400/409/404 均符合（见 UC-0006~0008） |
| UC-0003 IDENTITY.md 读写单测 | ✅ 通过 | hub 目录 `go test ./gateway/ -run TestIdentity`：读取、替换（其它小节逐字保留）、追加、首建（agent-name=ID）、空值保留标题、非法工作区拒绝 全绿 |
| UC-0004 ID 唯一与格式校验单测 | ✅ 通过 | `go test ./gateway/ -run 'TestValidateAgentID\|TestManagerRejectsDuplicateAndInvalidID'`：14 组格式用例 + 7 组重复/非法用例全绿，拒绝时不落盘 |
| UC-0005 registry 去 Name | ✅ 通过 | `grep -n "Name" hub/gateway/manager.go` 无 AgentSpec.Name/AgentPatch.Name；`TestManagerRegistryIgnoresLegacyNameField`（旧 JSON 含 `name` 可加载）通过；运行时 registry 落盘无 `name` 字段 |
| UC-0006 实时显示与写回 | ✅ 通过 | 临时 hub（127.0.0.1:28290，独立 registry/board-db）+ curl：POST 创建（带 `agent_description`）→ 201 且文件写入；GET → `{source:"file",readonly:false}` 实时读文件；PUT → 200，文件 `## agent-description` 被替换、`## agent-name`/`## user-name` 逐字保留 |
| UC-0007 external 只读 | ✅ 通过 | `GET /api/agents/ext1/identity` → `{readonly:true,source:"agent"}`；`PUT` → 409，文案「外部 Agent 的 Agent 描述为只读，请在对方 co-shell 的「身份与个性」中修改」 |
| UC-0008 新增 ID 校验 | ✅ 通过 | `POST /api/agents` 重复 ID 大小写变体 → 409 `agent already exists`；`bad/id` → 400 `id must not contain "/"`；`bad id` → 400；合法 ID → 201 且落 registry |
| UC-0009 公告板口径 | ✅ 单测通过 / 未做在线双 agent 实测 | `TestBoardDutyPrefersAgentReport`（上报值优先，无上报时读文件且 `duty_source=hub`）、`TestBoardRegisterAndDutyPriority` 全绿；本次临时实例无 agent 在线，roster 为空，在线双 agent 用例待用户环境复核 |
| UC-0010 回归 | ✅ 通过 | hub：`go build ./... && go vet ./... && go test ./gateway/` 全绿；根目录：`go build ./... && go vet ./...` 通过；版本/build = 0.68.0/1078；产物 `work/co-shell-0.68.0.darwin.arm64`、`work/co-shell-hub-0.68.0.darwin.arm64` |

## 实现后备注（替代原待确认项）

1. **写文件边界**：workspace 目录必须已存在（否则 400 且不创建任何文件）；`IDENTITY.md` 不存在时创建，且首建时写入 `## agent-name` = agent ID。
2. **空值口径**：Agent 描述允许为空，界面用 placeholder「（未设置）」占位，公告板 duty 为空串（不回退 ID）。
3. **external 实时性**：改为 co-shell 主动上报（IDENTITY.md mtime/size 轮询 4s + 本机设置界面写身份时立即推送 `board_identity` 帧），不再依赖 60s 心跳；旧 hub 忽略该帧时退化为心跳上报。
4. **「其它展示位改为 ID」清单**：
   - agent 详情标题 `detailTitle`：`(a.name || a.id) + ' · 修改'` → `a.id + ' · 修改'`；
   - agent 列表行：新增 `title` 属性 = ID（主显仍为自报名字，`esc(a.name || a.id)`）；
   - Chat 参与者名：沿用公告板自报名字（UC-0009 口径，未改）；
   - staging iframe：仅 `data-id` 与 `/agent/{id}/` src，无标题元素，无需改动。
