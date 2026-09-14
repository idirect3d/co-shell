# FEATURE-520 测试用例：Hub Agent 信息与启动参数支持修改

## 背景

co-shell-hub 的 Agent 列表卡片右侧箭头目前打开**只读详情页**（`hub/gateway/webui_static.go` 的 `viewDetail` + `showAgentDetail`），字段以 `<div class="val">` 呈现；添加 Agent 之后无法再调整其信息与启动参数。

本任务把该只读页替换为**可编辑的修改表单**，字段与「新建 Agent」界面保持一致；后端新增 `Manager.Update` 与 `PUT /api/agents/{id}`。

## 需求（用户确认）

1. 只读详情页 → 可编辑修改界面，字段与新建 Agent 一致：
   - 本地（managed）：Workspace 路径 / 端口号 / ID / 备注 / co-shell 可执行程序 / 共享配置 / 补充运行参数
   - 远程（external）：主机地址 / 端口号 / ID / 备注（字段集与本地**完全一致**）
2. **ID 只读展示、不可修改**（唯一键，改名需迁移注册表与代理连接，本次不做）。
3. 对**运行中**的 Agent 保存修改后仅持久化并提示「需重启 agent 后生效」，**不自动重启**。
4. **远程 Agent 与本地使用完全一致的字段集**，本地专有字段（Workspace / 端口 / co-shell / 共享配置 / 补充运行参数）对其**置灰不可编辑**。

## 验收前置

- 分支：`FEATURE-520`；版本 `0.57.0`；BUILD `1021`
- 启动 hub（不动用生产实例、独立端口/独立注册表）：
  ```bash
  go run ./cmd/co-shell-hub --serve \
    --web-addr 127.0.0.1:12901 --tcp-addr 127.0.0.1:12900 \
    --registry /tmp/fe520-hub-agents.json --base-port 28500
  ```
- Web UI：`http://127.0.0.1:12901/`
- **实现约束**：hub 前端 HTML 存放在 Go 原始字符串字面量（反引号）中，新增 JS 内**禁止出现反引号与 `${}`**，字符串一律用单引号拼接（沿用现有风格）。

---

## UC-0001 静态校验：路由、方法与前端语法（运行时）

**类型**：运行时（源码静态检索 + Go 编译）
**步骤**：
1. `grep -n "PUT /api/agents/{id}" hub/gateway/webui.go`
2. `grep -n "func (w \*WebUI) handleUpdateAgent" hub/gateway/webui.go`
3. `grep -n "func (m \*Manager) Update" hub/gateway/manager.go`
4. 检查 `hub/gateway/webui_static.go` 中 `webIndexHTML` 字面量内是否出现反引号：
   `grep -c '`' hub/gateway/webui_static.go`（应仅出现界定字面量的 2 个）

**预期**：
- 路由 `PUT /api/agents/{id}` 已注册，handler 与 `Manager.Update` 均存在
- 反引号计数 = 2（未破坏 Go 原始字符串）
- `go build ./...` 通过

---

## UC-0002 接口：GET /api/agents 返回修改界面所需的全部字段（运行时）

**类型**：运行时（HTTP）
**步骤**：
1. 启动 hub，`curl -s http://127.0.0.1:12901/api/agents`（若已有 agent 则复用；否则先用 `POST /api/agents` 建一个本地 agent）
2. 检查返回的 agent 对象键集合

**预期**：本地 agent 对象含 `id`、`name`、`type`、`workspace`、`port`、`co_shell`、`use_shared_config`、`extra_args`，足以回填修改表单（无需额外接口）。

---

## UC-0003 运行时：本地 Agent 修改「备注」并持久化

**类型**：运行时（HTTP + 文件）
**步骤**：
1. `curl -s -X PUT http://127.0.0.1:12901/api/agents/<id> -H 'Content-Type: application/json' -d '{"name":"改后备注"}'`
2. `curl -s http://127.0.0.1:12901/api/agents` 检查 `name`
3. `cat /tmp/fe520-hub-agents.json` 检查 `name`

**预期**：
- 步骤 1 返回 200 且响应体 `name == "改后备注"`
- 步骤 2、3 中该 agent 的 `name` 均为 `改后备注`（内存 + 落盘一致）

---

## UC-0004 运行时：本地 Agent 修改端口 / workspace / co-shell / 共享配置 / 补充参数并落盘

**类型**：运行时（HTTP + 文件）
**步骤**：
1. `curl -s -X PUT http://127.0.0.1:12901/api/agents/<id> -H 'Content-Type: application/json' -d '{"workspace":"/tmp/fe520-ws-b","port":28555,"co_shell":"/usr/local/bin/co-shell","use_shared_config":true,"extra_args":"--accept-license --log-level debug"}'`
2. `cat /tmp/fe520-hub-agents.json`

**预期**：注册表中该 agent 的 `workspace`、`port`、`co_shell`、`use_shared_config`、`extra_args` 均为新值；`/tmp/fe520-ws-b` 目录被创建。

---

## UC-0005 运行时：端口被占用时保存被拒绝且不落库

**类型**：运行时（HTTP 负例）
**步骤**：
1. 先占用一个端口：`python3 -m http.server 28566 --bind 127.0.0.1 &`
2. `curl -s -o /tmp/fe520-resp.json -w '%{http_code}' -X PUT http://127.0.0.1:12901/api/agents/<id> -H 'Content-Type: application/json' -d '{"port":28566}'`
3. 读取 `/tmp/fe520-resp.json` 与注册表

**预期**：
- HTTP 状态为 400 或 409，响应体 `error` 含端口占用提示
- 该 agent 的 `port` **保持原值**（未落库）

---

## UC-0006 运行时：修改不存在的 Agent 返回 404

**类型**：运行时（HTTP 负例）
**步骤**：
1. `curl -s -o /tmp/fe520-404.json -w '%{http_code}' -X PUT http://127.0.0.1:12901/api/agents/__no_such_agent__ -H 'Content-Type: application/json' -d '{"name":"x"}'`

**预期**：HTTP 404，响应体 `error` 指明 agent 不存在。

---

## UC-0007 运行时：非法请求体被拒绝

**类型**：运行时（HTTP 负例）
**步骤**：
1. `curl -s -o /dev/null -w '%{http_code}' -X PUT http://127.0.0.1:12901/api/agents/<id> -H 'Content-Type: application/json' -d 'not-json'`
2. `curl -s -o /dev/null -w '%{http_code}' -X PUT http://127.0.0.1:12901/api/agents/<id> -H 'Content-Type: application/json' -d '{"port":700000}'`

**预期**：均为 400，且不改变注册表内容。

---

## UC-0008 运行时：卡片箭头打开的是可编辑表单而非只读详情（浏览器 DOM）

**类型**：运行时（浏览器）
**步骤**：
1. 打开 `http://127.0.0.1:12901/`，展开左侧抽屉的 Agent 列表
2. 点击某个 Agent 卡片右侧的箭头（`.chev`）
3. 在控制台执行：
   ```js
   var v = document.getElementById('viewDetail');
   ({
     visible: !v.classList.contains('hidden'),
     inputs: v.querySelectorAll('input, select').length,
     readOnlyVals: v.querySelectorAll('.val').length
   })
   ```

**预期**：
- `visible === true`
- `inputs >= 5`（表单控件存在）
- `readOnlyVals === 0`（不再有只读 `.val` 字段）

---

## UC-0009 运行时：修改界面字段与新建界面一致（浏览器 DOM 对照）

**类型**：运行时（浏览器）
**步骤**：
1. 打开新建界面（列表底部「新建」按钮），记录本地模式字段：Workspace 路径 / 端口号 / ID / 备注 / co-shell 可执行程序 / 共享配置 / 补充运行参数
2. 返回列表，点击本地 Agent 卡片箭头进入修改界面
3. 逐项核对修改界面是否具备相同 7 个字段（标签文案与控件类型一致）

**预期**：修改界面本地模式下 7 个字段与新建界面一一对应，标签文案一致（ID 字段存在但为只读）。

---

## UC-0010 运行时：ID 字段只读不可修改（浏览器 DOM）

**类型**：运行时（浏览器）
**步骤**：
1. 进入某 Agent 的修改界面
2. 在控制台执行：
   ```js
   var el = document.getElementById('d-id');
   ({ tag: el ? el.tagName : null, readOnly: el ? (el.readOnly === true || el.disabled === true) : null, value: el ? el.value : null })
   ```

**预期**：`readOnly === true`（或 disabled），且 `value` 等于该 Agent 的 ID；尝试编辑无效。

---

## UC-0011 运行时：运行中 Agent 保存后不自动重启，并提示需重启生效

**类型**：运行时（HTTP + 浏览器）
**步骤**：
1. 在列表中把某本地 Agent 启动（开关置开），记录其进程 PID：
   `pgrep -f "co-shell --serve --port <port>"`
2. 通过修改界面修改「备注」（或「补充运行参数」）并保存
3. 再次 `pgrep -f "co-shell --serve --port <port>"` 比对 PID
4. 观察界面提示

**预期**：
- 保存成功（无报错），界面出现「需重启 agent 后生效」类提示
- 前后 PID **完全一致**（未自动重启）
- 注册表文件已写入新值

---

## UC-0012 运行时：远程 Agent 字段集一致、不适用字段置灰（浏览器 DOM）

**类型**：运行时（浏览器）
**步骤**：
1. 用「远程」模式添加一个 external agent（主机 `127.0.0.1` + 某端口）
2. 点击该卡片箭头进入修改界面
3. 在控制台执行：
   ```js
   var v = document.getElementById('viewDetail');
   ['d-ws','d-host','d-port','d-coshell','d-shared','d-extra'].map(function(id){
     var el = document.getElementById(id);
     return { id: id, exists: !!el, disabled: el ? el.disabled : null };
   })
   ```

**预期**：控件集与本地 Agent **完全相同**；对远程 Agent，Workspace / co-shell / 共享配置 / 补充运行参数为 `disabled === true`（置灰），主机地址与端口号可编辑，备注可编辑；保存后 `ws_url` 按新「主机地址 + 端口号」重建。

**对向校验（本地 Agent）**：本地 Agent 的「主机地址」为 `disabled === true`，其余字段可编辑。

---

## UC-0013 界面回归：修改后列表即时刷新且不影响启动/停止/删除

**类型**：运行时（浏览器 + HTTP）
**步骤**：
1. 在修改界面改「备注」并保存，返回列表
2. 核对列表条目显示的名称
3. 对同一 Agent 依次执行启动 / 停止 / 删除

**预期**：
- 列表显示已更新的名称（保存后自动 `refresh()`）
- 启动 / 停止 / 删除行为与修改前一致，无回归

---

## UC-0014 编译与版本同步（运行时）

**类型**：运行时（构建）
**步骤**：
1. `go build ./... && go vet ./...`
2. `grep -n 'const version' main.go`、`grep -n 'hubVersion' cmd/co-shell-hub/main.go`
3. 编译 hub 到 `work/` 并启动，`curl -s http://127.0.0.1:12901/api/hub-info`

**预期**：
- `go build ./... && go vet ./...` 全绿
- `main.go` version = `0.57.0`，`cmd/co-shell-hub/main.go` hubVersion = `0.57.0`（两者一致）
- `/api/hub-info` 返回 `version == "0.57.0"`

---

## 用例清单汇总

| 编号 | 类型 | 验证点 |
|------|------|--------|
| UC-0001 | 静态 | 路由/handler/Manager.Update 存在；Go 原始字符串未被反引号破坏 |
| UC-0002 | 接口 | GET /api/agents 字段足以回填修改表单 |
| UC-0003 | 接口 | 修改备注 → 内存 + 落盘一致 |
| UC-0004 | 接口 | 修改 workspace/端口/co-shell/共享配置/参数 → 落盘，目录创建 |
| UC-0005 | 接口负例 | 端口占用被拒绝且不落库 |
| UC-0006 | 接口负例 | 不存在的 agent → 404 |
| UC-0007 | 接口负例 | 非法 JSON / 非法端口 → 400 |
| UC-0008 | 界面 | 箭头打开的是可编辑表单（无 `.val` 只读字段） |
| UC-0009 | 界面 | 修改界面字段与新建界面一致 |
| UC-0010 | 界面 | ID 只读不可改 |
| UC-0011 | 界面 | 运行中保存不自动重启 + 提示需重启 |
| UC-0012 | 界面 | 远程 agent 字段集一致、不适用字段置灰（本地则主机地址置灰） |
| UC-0013 | 界面回归 | 保存后列表刷新；启动/停止/删除无回归 |
| UC-0014 | 构建 | 编译/静态检查全绿 + 版本号同步 0.57.0 |
