# FEATURE-609 UC-0001：hub 端口配置策略优化（默认端口占用自动递增 + 托管 agent 端口基点取 web 端口+1）

> 分支：**FEATURE-609**　版本：**v0.97.0**　BUILD：**1223**
> 目标：① 未显式指定 `--tcp-addr` 时默认 `127.0.0.1:27256`，被占用则端口 +1 直到空闲；显式指定（命令行或配置）时被占用直接退出。② 未显式指定 `--web-addr` 时默认 `127.0.0.1:28256`，规则同上。③ 新建本地托管 agent 的默认端口号 = hub Web 端口 + 1，配置界面自动探测占用（占用则 +1），每次进入配置界面都重新探测。

## 背景与口径

- **“已显式指定”判定（用户确认）**：命令行 flag（`--tcp-addr`/`--web-addr`）**或** `hub-gateway.json` 中的 `tcp_addr`/`web_addr` 任一存在即视为已指定 → 被占用时直接退出（不自动递增）。
- **“未指定”（内置默认）**：两处都没有该字段 → 使用内置默认端口并自动 +1 直到空闲。
- **托管 agent 端口基点**：`--base-port`（或配置 `base_port`）为显式覆盖；两者都未给出时，基点 = 解析后的 hub Web 端口 + 1（用户确认 Q3）。
- **适用范围（用户确认 Q4）**：需求 ③ 仅适用于**本地托管 agent**（创建本地 Agent 表单的端口默认值）；远程 agent 的推荐端口逻辑不变。
- **自动递增探测口径**：对 `host:port` 做一次 `net.Listen` 探测，能绑定即空闲（探测后立即关闭）。递增上限 65535。

## 变更点（计划）

| 位置 | 变更 |
|---|---|
| `cmd/co-shell-hub/main.go` `loadConfig()` | 不再预填 `127.0.0.1:12801/12802`；缺失字段留空，由后续解析决定 |
| `cmd/co-shell-hub/main.go` 新增 `resolveListenAddr(addr, host, defPort, explicit)` | 显式 → 原样返回；非显式 → 从默认端口起探测，返回首个空闲 `host:port` |
| `cmd/co-shell-hub/main.go` 新增 `addrPort(addr, fallback)` + BasePort 推导 | `cfg.BasePort <= 0` 时取 `addrPort(cfg.WebAddr)+1` |
| `cmd/co-shell-hub/main.go` 默认端口常量与 `printUsage()`/文件头注释 | 默认改为 27256（TCP）/28256（Web）；base-port 说明为“默认 = Web 端口+1” |
| `hub/gateway/webui_static.go` `loadDefaults()` | 记录上次自动推荐值；字段为空或仍等于上次推荐值时用最新探测结果刷新（每次进入重新探测） |
| 测试 | `cmd/co-shell-hub/main_test.go` 覆盖 `resolveListenAddr`/`addrPort`/base 推导；隔离实例运行时验证 |

## 用例与结果

> 状态列在实施完成前为 `待验证`；`(R)` = 可自动化/命令行复现，`(M)` = 需人工启动实例验收。

### A. hub TCP 监听地址（`--tcp-addr`）

- **UC-01（R）** 未指定 `--tcp-addr` 且 27256 空闲 → hub 实际监听 `127.0.0.1:27256`。
- **UC-02（R）** 未指定 `--tcp-addr` 且 27256 被占用 → 自动改用 `127.0.0.1:27257`（+1 直到空闲）。
- **UC-03（R）** 未指定且 27256、27257 均被占用 → 自动改用 `127.0.0.1:27258`。
- **UC-04（R）** 显式 `--tcp-addr 127.0.0.1:27256` 且该端口被占用 → 进程启动失败并以非 0 退出（打印 `tcp listen` 错误），**不**自动递增。
- **UC-05（R）** 显式 `--tcp-addr 127.0.0.1:<空闲端口>` → 正常监听该端口。
- **UC-06（R）** `hub-gateway.json` 配置 `tcp_addr` 且被占用 → 视同“已指定”，启动失败退出（不递增）。

### B. hub Web 监听地址（`--web-addr`）

- **UC-07（R）** 未指定 `--web-addr` 且 28256 空闲 → hub 实际监听 `127.0.0.1:28256`。
- **UC-08（R）** 未指定 `--web-addr` 且 28256 被占用 → 自动改用 `127.0.0.1:28257`，且启动日志打印最终地址。
- **UC-09（R）** 显式 `--web-addr 127.0.0.1:28256` 且被占用 → 进程启动失败退出（不递增）。
- **UC-10（R）** `hub-gateway.json` 配置 `web_addr` 且被占用 → 视同“已指定”，启动失败退出。
- **UC-11（R）** TCP 与 Web 独立递增互不影响：TCP 默认端口被占用（→27257）而 Web 默认端口空闲（→28256）时，两者各自取正确地址。

### C. 托管 agent 默认端口基点与配置界面

- **UC-12（R）** 未指定 `--base-port`/`base_port`：默认基点 = 解析后的 Web 端口 + 1。Web 端口为 28256 → 基点 28257；Web 端口自动递增到 28257 → 基点 28258。
- **UC-13（R）** 显式 `--base-port N` → 基点用 N，不被 Web 端口推导覆盖。
- **UC-14（R）** `hub-gateway.json` 配置 `base_port` → 视同已指定，基点用该值。
- **UC-15（R）** `/api/agent-defaults` 的 `recommended_port`：从基点起跳过“已注册 agent 占用端口”与“系统实际占用端口”，返回首个空闲端口；基点端口被占用时返回 +1 后的空闲端口。
- **UC-16（R）** 每次进入创建表单都重新探测：`loadDefaults()` 每次进入创建表单都会请求 `/api/agent-defaults`；当端口字段为空或仍等于上次自动推荐值时，用最新探测结果刷新字段。
- **UC-17（R）** 用户手动填入的端口不被覆盖：端口字段值 ≠ 上次自动推荐值 时，重新进入表单保持不变。
- **UC-18（M）** 真实启动 hub（无参数）→ TCP 27256、Web 28256；打开配置界面新建本地 Agent，端口默认值显示 28257；占用 28257 后重进界面，默认值变为 28258。

### D. 回归与一致性

- **UC-19（R）** `--help`/文件头文案更新：TCP 默认 27256、Web 默认 28256，`--base-port` 说明为“默认 = Web 端口+1”。
- **UC-20（R）** `go build ./... && go vet ./...` 一次性通过；`go test ./cmd/co-shell-hub/... -short`、hub 模块 `go test ./hub/gateway/... -short` 全部通过。
- **UC-21（R）** 版本一致：`main.go` `version=0.97.0`、`cmd/co-shell-hub/main.go` `hubVersion=0.97.0`；产物 `--version` 自检一致。

## 验证结果（2026-10-05）

| 用例 | 结论 | 证据 |
|---|---|---|
| UC-01/02/03/05/07/08/09/11（R） | ✅ 通过 | 单测 `TestResolveListenAddr*`、`TestDefaultPortConstants`；隔离实例 `work/fe609-verify/report.txt`：默认 tcp 27256、默认 web（28256 被占用）取 28257；第二/第三个 hub tcp 27257/27258、web 28258/28259；显式占用退出码 1 |
| UC-04（R） | ✅ 通过 | 隔离实例：显式 `--tcp-addr 127.0.0.1:27256` 且被占用 → `tcp listen: ... address already in use`、退出码 1 |
| UC-06/UC-10（R） | ✅ 通过 | 隔离实例：配置文件 `tcp_addr` 占用 → 退出码 1（`--web-addr`/`web_addr` 同理由 `resolveListenAddr(explicit=true)` 原样返回实现），单测 `TestResolveListenAddrExplicit` |
| UC-12/13/14（R） | ✅ 通过 | 单测 `TestAgentBasePort`；隔离实例 `recommended_port` = web 端口+1（28259）、`--base-port 30000` → `recommended_port` 30000 |
| UC-15/16/17（R） | ✅ 通过 | 单测 `TestFeature609RecommendedPortSkipsRegistered`、`TestFeature609FrontendPortRefresh` |
| UC-18（M） | ⏳ 待人工 | 需人工打开 hub Web UI 新建本地 Agent，确认端口默认值随 web 端口变化（后端 `recommended_port` 已由隔离实例证实） |
| UC-19/UC-20/UC-21（R） | ✅ 通过 | `go build ./... && go vet ./...`；`go test ./cmd/co-shell-hub/...`、hub `go test ./gateway/...` 全部 PASS；版本 0.97.0 / BUILD-1223 |
