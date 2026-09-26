# FEATURE-559 验收测试用例（UC-0001）：hub 抽屉文案国际化（分区标题统一为 Agents / Agent 聊天室）

> 需求（用户提出）：「现在 hub 中包含两个抽屉，Agents 和 Chat，命名上是不是有些不一致？一个有 s 一个没有，这样合适吗？你有什么建议？」
> 结论：并列分区应同形态；且同一抽屉内「标题英文 + 正文中文」的语言混用比单复数更影响观感。
> 用户决策：**按中文/国际化进行优化，英文名都用复数形式。**

> 交互决策（经 ask_user 确认）：
> - ① 改造范围 = **抽屉内所有可见文案**（分区标题 + 按钮 + 提示/tooltip）全部纳入 i18n；
> - ② 语言来源 = **跟随操作系统语言**，可用 `--lang` 配置（与 co-shell 现有策略一致）；
> - ③ 中文分区标题 = **Agents / Agent 聊天室**（中英之间加空格）；
> - ④ 复数化范围 = **仅两个分区标题**（英文界面 **Agents / Chats**）；
> - ⑤ 版本归属 **v0.76.2**（patch 递增）、任务编号 **FEATURE-559**。

> 改动范围（预期实现）：
> `cmd/co-shell-hub/main.go`（新增 `--lang` 参数并传入 gateway）、
> `hub/gateway/lang.go`（新增：语言解析，`--lang` > `LANG`/`LC_ALL` > 默认 zh）、
> `hub/gateway/webui.go`（`handleIndex` 注入当前语言；客户端可见错误文案取词）、
> `hub/gateway/webui_static.go`（引入 zh/en 字典与取词函数，覆盖抽屉内全部可见文案；分区标题改为取词）、
> `hub/gateway/webui_detect.go` / `hub/gateway/webui_board.go`（固定客户端可见错误串取词，如纳入）。

## 用例清单

| 编号 | 名称 | 类型 |
|------|------|------|
| A-001 | 语言解析：`--lang en` 显式覆盖（即使 `LANG=zh_CN.UTF-8`） | 单测 |
| A-002 | 语言解析：未给 `--lang` 时按 `LANG`/`LC_ALL` 判定（`zh_*`→zh，`en_*`→en） | 单测 |
| A-003 | 语言解析：`LANG` 为空/未知（如 `fr_FR`）与非法 `--lang`（`fr`）→ 回退默认 zh | 单测 |
| A-004 | `handleIndex` 注入语言：`/` 返回 HTML 含当前语言注入点（默认 zh） | 单测 |
| A-005 | 字典完整性：zh 与 en 键集合一致（无缺键/多余键） | 单测 |
| A-006 | 标题取值：en → `Agents` / `Chats`；zh → `Agents` / `Agent 聊天室` | 单测 |
| A-007 | 静态页无残留硬编码 UI 文案：抽屉范围内中文/英文 UI 字面量已全部走取词 | 单测（静态断言） |
| A-008 | 取词函数缺键回退：未知键返回键名或 zh 兜底，不抛异常 | 单测 |
| A-009 | 后端固定客户端错误串**保持英文原文**（按决策不纳入 i18n） | 单测（回归断言） |
| A-010 | 版本号/构建号：hub 与 co-shell 同为 `0.76.2` / `BUILD-1130` | 单测 |
| B-001 | 默认（未传 `--lang`，系统语言中文）打开 hub → 分区标题为 `Agents` / `Agent 聊天室` | 运行时 |
| B-002 | `--lang en` 启动 → 分区标题为 `Agents` / `Chats` | 运行时 |
| B-003 | 中文界面下抽屉内**全部**文案为中文（无英文标题/按钮/提示混入） | 运行时 |
| B-004 | 英文界面下抽屉内**全部**文案为英文（无中文残留） | 运行时 |
| B-005 | tooltip 随语言切换（钉住、返回 Agent 列表、收起、修改配置、启动/停止、公告板状态） | 运行时 |
| B-006 | placeholder 随语言切换（Workspace 路径、端口号、ID、主机地址、访问 KEY、输入消息…） | 运行时 |
| B-007 | Chat 面板文案随语言切换（会话/公告、空态、共 N 条、发送、已受理/已拒绝/待补充/免回复/等待 Ns/窗口 Ns） | 运行时 |
| B-008 | 抽屉「设置」面板文案随语言切换（HTTPS 访问、访问控制、白名单、KEY、生成自签名证书、保存提示） | 运行时 |
| B-009 | 前端 alert/提示前缀随语言切换（保存失败/启动失败/停止失败/删除失败/创建失败…） | 运行时 |
| B-010 | 主区空态文案随语言切换（暂无 Agent / 点击创建或添加 Agent） | 运行时 |
| B-011 | 语言切换只影响文案：抽屉展开/收起/浮动、图标、公告板功能行为不变 | 运行时（回归） |
| B-012 | 参数校验提示随语言切换（端口非 1-65535、未填主机地址、未填 Workspace 路径、端口占用） | 运行时 |

---

## A 组：单元测试（Go）

- **A-001**：`SetLangFromFlag("en")` + 环境 `LANG=zh_CN.UTF-8` → 期望 `en`（flag 优先级最高）。
- **A-002**：`LANG=zh_CN.UTF-8` → `zh`；`LANG=en_US.UTF-8`（`LC_ALL` 空）→ `en`；`LANG` 空而 `LC_ALL=zh_TW.UTF-8` → `zh`。
- **A-003**：`LANG=fr_FR.UTF-8` → `zh`；`SetLangFromFlag("fr")` → `zh`（非法值不 panic、不报错）。
- **A-004**：构造 `WebUI{lang}` 调 `handleIndex` → 响应体包含语言注入（如 `__HUB_LANG__` 或已替换为 `"zh"`），且 `Content-Type: text/html; charset=utf-8`。
- **A-005**：解析两个字典（导出为包级变量或通过测试辅助读取）→ 断言 `len(zhKeys)` == `len(enKeys)` 且键集合相等。
- **A-006**：断言 `zh["paneAgents"] == "Agents"`、`zh["paneChats"] == "Agent 聊天室"`、`en["paneAgents"] == "Agents"`、`en["paneChats"] == "Chats"`。
- **A-007**：对 `webui_static.go` 做静态断言：分区标题行（`id="agentsHead"` / `id="chatHead"`）不含内联字面量标题，而含取词调用；抽屉范围内不残留中文 UI 字面量（保留注释与 `co-shell`／`Agent`／`Workspace` 等专有名词白名单；按实现以清单方式断言）。
- **A-008**：`t("no_such_key")` → 返回键名（或 zh 兜底值），不 panic。
- **A-009**（决策：不纳入）：静态断言 `webui.go` 中 `agent not found` / `id and workspace are required` 等固定错误串未被改动，仍为英文原文（前端仅本地化其前缀，如「创建失败: 」）。
- **A-010**：`/api/hub-info` 返回 `{"version":"0.76.2","build":"1130"}`。

## B 组：运行时用例（真实 hub 二进制）

启动方式（独立端口 + 独立 registry/board，避免影响线上实例）：

```bash
work/co-shell-hub-0.76.2.darwin.arm64 -serve -web-addr 127.0.0.1:28399 \
  -registry /tmp/feat559-agents.json -board-db /tmp/feat559-board.db
# 英文界面：追加 -lang en
```

- **B-001/B-002**：打开 `http://127.0.0.1:28399/`，展开左抽屉，读取两个分区标题文本；两种语言各验一次。
- **B-003/B-004**：通读抽屉内全部可见文案（Agents 列表、新建/编辑表单、设置面板、Chat 两级视图），确认单语言、无混排。
- **B-005/B-006**：hover 各按钮/徽标读取 `title`；检查各输入框 `placeholder`。
- **B-007**：构造至少 1 个会话（agent 注册公告板后发帖），检查会话列表/详情/徽章/发送按钮文案。
- **B-008**：打开设置面板（HTTPS/访问控制），检查开关标题、提示、按钮与保存回执。
- **B-009/B-012**：制造失败（填写非法端口、空主机地址、占用端口）观察 alert 文案。
- **B-010**：清空 registry 启动，观察主区空态文案。
- **B-011**：切换 `--lang` 前后对比抽屉展开/收起、宽屏浮动叠加、公告板收发消息行为一致。

## C 组：回归

- **C-001**：`go build ./... && go vet ./...`（root 模块）与 hub 模块 `go build ./... && go vet ./...` 均通过；`go test ./hub/gateway/` 通过。
- **C-002**：Agent 列表/新建/编辑/启动/停止/删除、公告板会话与消息、设置保存等功能不受影响。
- **C-003**：既有静态断言用例（FIX-558、FEATURE-557）仍通过——尤其 FEATURE-557 的「首页含 `i-board` symbol、宽屏不含推移规则」断言不得被改文案时误伤。
- **C-004**：`co-shell-hub -serve --version`（或 `/api/hub-info`）显示 `0.76.2 [BUILD-1130]`。

---

## 边界与待确认项（请重点确认）

1. **后端错误文案（已决策：不纳入）**
   hub 后端返回的客户端可见错误串保持**硬编码英文原文**（如 `agent not found`、`id and workspace are required`），前端仅本地化其前缀（如「创建失败: agent not found」）。本次**不改** `webui.go` / `webui_detect.go` / `webui_board.go` 中的固定错误串。

2. **主区空态文案（已决策：纳入）**
   「暂无 Agent，点击上方"运行"箭头…」与 `title="创建或添加 Agent"` 虽不在抽屉内，但属同一页面，**一并本地化**，避免出现「局部中文、局部英文」的新混排。

3. **是否新增/更新文档说明**
   在 `--help` 中列出 `--lang`，并在 ROADMAP 任务详情记录实现与验证结论；不改 README（hub 的启动参数说明不在 README 主体）。

4. **复数化的边界**
   仅两个分区标题用复数（英文 `Agents` / `Chats`）。抽屉内其它文案中的 `Agent` 一词（如「新建 Agent」「返回 Agent 列表」）保持原样，中文语境下不加复数。

## 实施顺序（预期）

1. `hub/gateway/lang.go`：语言解析（`--lang` > `LANG`/`LC_ALL` > 默认 zh）+ 语言校验/回退。
2. `cmd/co-shell-hub/main.go`：新增 `--lang` 参数，注入 `gateway.WebUIConfig`。
3. `hub/gateway/webui.go`：`handleIndex` 注入当前语言；后端客户端可见错误串取词。
4. `hub/gateway/webui_static.go`：zh/en 字典 + 取词函数 + `data-i18n*` 应用；分区标题改为取词（英文 `Agents` / `Chats`，中文 `Agents` / `Agent 聊天室`）。
5. 新增 `hub/gateway/feature559_test.go` 覆盖 A 组；`go build ./... && go vet ./...` + hub 模块测试。
6. 编译产物（co-shell 与 co-shell-hub 同版本 0.76.2 / BUILD-1130），真实二进制跑 B 组。
7. 提交到 `FEATURE-559`，等用户实测确认后再合并 main 并打 tag `v0.76.2`。
