# FEATURE-607 UC-0001：下载判定扩展（X-Client-IP）+ hub 配置开关

> 分支：**FEATURE-607**　版本：**v0.96.0**　BUILD：**1219**
> 判定口径：`allowDownload = 启用 --download-enabled 且 ( co-shell 自身 bind 非 loopback 或 请求头 X-Client-IP 为非 loopback IP )`。
> 本机浏览器直接访问（bind loopback 且无 X-Client-IP）仍**不**提供下载；经 hub 转发的移动端/远程客户端（co-shell 绑 127.0.0.1）在启用开关后可下载。hub 反代必须注入真实客户端地址并**清除客户端自带的同名头**（防伪造）。

## 背景（现象与根因）

- **现象**：co-shell 的 `--download-enabled` 只对「bind 非 loopback」生效（`web/server.go:1478` `downloadEnabled() = opts.DownloadEnabled && s.isRemote()`，而 `isRemote()` 仅依据 bind 地址，见 `web/server.go:1468`）。移动端/远程经 hub 访问时，co-shell 实际绑定 `127.0.0.1`（`hub/gateway/manager.go:448` `buildArgs`），`isRemote()` 恒为 false，因此**移动端永远拿不到下载能力**。
- **根因**：co-shell 无法区分「本机浏览器直接访问」与「经 hub 转发的远程客户端」——两者在 TCP 层都来自 127.0.0.1。
- **方案**：hub 反向代理（`hub/gateway/proxyhttp.go:36` `ServeHTTP`，底层 `httputil.NewSingleHostReverseProxy`，`proxyhttp.go:99`）在转发 `/agent/{id}/` 时注入真实客户端地址 `X-Client-IP = r.RemoteAddr`（保留 `IP:port` 原样），并**先删除客户端自带的同名头**；co-shell 据此把「远程」判定扩展为「bind 非 loopback 或 客户端非 loopback」。hub 侧同时提供「启用下载功能」开关，打开则启动该 agent 时注入 `--download-enabled`。

## 变更点

| 位置 | 变更 |
|---|---|
| `web/server.go` `downloadEnabled()` | 改为 `opts.DownloadEnabled && s.isRemoteRequest(r)` |
| `web/server.go` 新增 `isRemoteClient(r)` / `isRemoteRequest(r)` | 解析 `X-Client-IP` 的 host，非 loopback 即为远程客户端；`isRemoteRequest = isRemote() \|\| isRemoteClient(r)` |
| `web/server.go` bootstrap（`"remote"` 字段） | 改为 `s.isRemoteRequest(r)`，使经 hub 的远程访问也禁用 reveal、显示下载图标 |
| `web/server.go` `handleOpen` / `handleReveal` | 远程禁用改用 `isRemoteRequest(r)`（语义一致） |
| `web/static/app.js` 文件树 actions | 可下载时把「在文件夹中显示」图标替换为下载图标；本机直接访问保持 reveal |
| `i18n/zh.go`、`i18n/en.go` | `--download-enabled` 帮助文案去掉「仅对非本机监听地址生效」/「only effective when serving to non-loopback addresses」 |
| `hub/gateway/proxyhttp.go` | 反代前 `Header.Del("X-Client-IP")` + `Header.Set("X-Client-IP", r.RemoteAddr)` |
| `hub/gateway/manager.go` | `AgentSpec` 新增 `DownloadEnabled bool`（`json:"download_enabled,omitempty"`）；`buildArgs` 注入 `--download-enabled`（参照 `YOLO` 模式） |
| `hub/gateway/webui_static.go`、`webui.go` | agent 配置表单新增「启用下载功能」开关；创建/更新 API 读写该字段 |
| 测试 | `web/server_test.go`、`hub/gateway` 相关 `*_test.go` 覆盖各组合 |

## 用例与结果

> 状态列在实施完成前为 `待验证`；每项须有可复现证据（命令 + 输出、或代码断言）。

### A. co-shell 下载判定（服务端）

- **UC-01（R）总开关关闭时一律拒绝**：`--download-enabled` 未启用时，`bind=127.0.0.1` 与 `bind=0.0.0.0` × 带/不带 `X-Client-IP` 的全部组合，`GET /api/download` 均返回 **403**（`download disabled`）。
- **UC-02（R）本机直接访问不提供下载**：`bind=127.0.0.1`、无 `X-Client-IP`、已启用开关 → **403**。
- **UC-03（R）本机经 hub 访问仍不提供下载**：`bind=127.0.0.1`、`X-Client-IP: 127.0.0.1:52345`、已启用开关 → **403**。
- **UC-04（R）经 hub 的移动端/远程客户端可下载**：`bind=127.0.0.1`、`X-Client-IP: 192.168.3.39:51000`、已启用开关 → **200**，`Content-Disposition` 以 `attachment` 开头，body 为文件内容。
- **UC-05（R）远程直连保持现状**：`bind=0.0.0.0`、无 `X-Client-IP`、已启用开关 → **200**（不回归原有行为）。
- **UC-06（R）远程 bind + 客户端头并存**：`bind=0.0.0.0`、`X-Client-IP: 10.0.0.9:40000` → **200**。
- **UC-07（R）IPv6 loopback 视为本机**：`bind=127.0.0.1`、`X-Client-IP: [::1]:52345` → **403**。
- **UC-08（R）不可解析的 X-Client-IP 不放大权限**：`bind=127.0.0.1` 下，`X-Client-IP` 为 `abc`、`not-an-ip`、空串等无法解析为 IP 的值 → **403**（解析失败即不视为远程；无端口的裸 IP 仍按其地址判 loopback）。
- **UC-09（R）路径穿越与目录（安全回归）**：可下载场景下 `path=../../etc/passwd` → **403**；`path=<目录>` → **404**。

### B. bootstrap 下发字段（前端依据）

- **UC-10（R）本机直接访问**：`bind=127.0.0.1`、无 `X-Client-IP` → `remote=false`、`downloadEnabled=false`。
- **UC-11（R）经 hub 的远程客户端**：`bind=127.0.0.1`、`X-Client-IP: 192.168.3.39:51000`、已启用开关 → `remote=true`、`downloadEnabled=true`。
- **UC-12（R）经 hub 的远程客户端但开关关闭**：同上但未启用开关 → `remote=true`、`downloadEnabled=false`。
- **UC-13（R）远程 bind**：`bind=0.0.0.0` → `remote=true`（不论有无 `X-Client-IP`）。

### C. open / reveal 的远程禁用一致性

- **UC-14（R）经 hub 的远程客户端不可 reveal/open**：`bind=127.0.0.1`、`X-Client-IP: 192.168.3.39:51000` → `POST /api/reveal`、`POST /api/open` 均 **403**。
- **UC-15（R）本机直接访问仍可 reveal/open**：`bind=127.0.0.1`、无 `X-Client-IP` → 与改动前一致（本机仍可调用）。

### D. hub 反向代理注入 X-Client-IP

- **UC-16（R）注入真实客户端地址**：经 `/agent/{id}/...` 访问时，上游 co-shell 收到的 `X-Client-IP` == 该请求的 `r.RemoteAddr`（`IP:port` 原样保留）。
- **UC-17（R）覆盖伪造头**：客户端自带 `X-Client-IP: 1.2.3.4:9999` 时，上游收到的是真实 `RemoteAddr`（伪造值被清除）。
- **UC-18（R）非代理路径不受影响**：hub 自身路由（非 `/agent/{id}/`）不注入该头。

### E. hub agent 配置「启用下载功能」开关

- **UC-19（R）开关打开注入参数**：`AgentSpec.DownloadEnabled=true` → `buildArgs` 结果包含 `--download-enabled`。
- **UC-20（R）开关关闭不注入（零值不改行为）**：`DownloadEnabled=false` → 命令行**不含** `--download-enabled`；既有 `hub-agents.json` 不带该字段时行为不变。
- **UC-21（R）API 与持久化**：创建/更新 agent 带 `download_enabled=true` → 落盘 JSON 含 `"download_enabled":true`，重新加载（重启 hub）后仍为 true。
- **UC-22（R）Web UI 表单**：agent 配置界面含「启用下载功能」开关，能正确回填（打开/关闭）并提交保存。
- **UC-23（R）端到端启动参数**：hub 配置开关打开 → 实际启动的 co-shell 子进程命令行含 `--download-enabled`；关闭 → 不含。

### F. i18n 与前端图标

- **UC-24（S）帮助文案更新**：`i18n/zh.go`、`i18n/en.go` 中 `--download-enabled` 一行不再包含「仅对非本机监听地址生效」/「only effective when serving to non-loopback addresses」。
- **UC-25（R）前端图标（可下载）**：bootstrap `remote=true` + `downloadEnabled=true` 时，文件行显示下载图标（`i-download`）且无 reveal 按钮；目录行无动作。
- **UC-26（R）前端图标（远程不可下载）**：`remote=true` + `downloadEnabled=false` 时，文件行无下载图标（亦无 reveal）。
- **UC-27（R）前端图标（本机）**：`remote=false` 时保持「在文件夹中显示」图标（与改动前一致）。

### G. 静态与回归

- **UC-28（S）静态检查**：`go build ./... && go vet ./...` 通过；`node --check web/static/app.js` 通过；hub 独立模块 `go build/vet/test` 通过。
- **UC-29（S）测试无新增失败**：`go test ./web/... ./hub/... -short`（及 `go test ./... -short`）失败集合与改动前基线**一致**，无新增失败。
- **UC-30（R）真实链路验收（人工）**：真实 hub（配置开关打开）+ co-shell，用移动端（或模拟移动端来源）经 hub 访问 → 文件树出现下载图标 → 点击可成功下载工作区文件；本机直接访问 co-shell 时无下载入口。

## 不在本次范围 / 待人工验证

- iOS 客户端**零改动**（不涉及 WKWebView header 注入）。
- 「远程」判定不再改动 WebSocket 通道（下载走 HTTP，`/api/bootstrap` 亦为 HTTP）。
- UC-30 依赖真实移动端/内网 hub 环境，由用户在正式环境确认。
