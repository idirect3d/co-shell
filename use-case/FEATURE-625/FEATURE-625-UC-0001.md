# FEATURE-625 UC-0001：co-shell.app 原型「双击即用」验证

> 分支：**FEATURE-625**　版本：**v0.108.0**　BUILD：**待编译**
> 需求（用户 2026-10-09 23:05 提出）：做一个 Developer ID 直发的 `co-shell.app` 原型（不上架 App Store），名称 `co-shell.app`，内含 hub 与 co-shell 二进制，先验证「双击即用」体验；本次**尽量不动 Go 代码**，以便未来 go 版与 app 版共用同一份二进制、同步发布。
>
> 范围：本次为**最小可验证原型** —— 目标是「双击能开、UI 能用、任务能跑、退出干净」，**不做** App 图标 / 配置 GUI 化 / 开机自启 / 公证（notarization）。

## 术语

- **外壳（shell）**：`Contents/MacOS/co-shell-app`，Swift 编写的 App 主进程，负责拉起 hub 并承载 WKWebView。
- **内嵌二进制**：`Contents/Resources/bin/{co-shell,co-shell-hub}`，随 bundle 分发，与 go 版同一份产物。
- **数据目录**：`~/Library/Application Support/co-shell.app/`。
- **hub 日志**：`~/Library/Logs/co-shell-app/hub.log`。
- `(M)` = 需运行时手动 / 浏览器断言；`(R)` = 离线静态校验。

## 设计基线（本次实现基准，供确认）

| 项 | 设计 |
|----|------|
| bundle 标识 | `com.coshell.app`，主可执行 `co-shell-app`，显示名 `co-shell` |
| 内嵌二进制 | `Contents/Resources/bin/co-shell`、`Contents/Resources/bin/co-shell-hub` |
| 数据目录 | `~/Library/Application Support/co-shell.app/`，首次启动自动创建 |
| 端口 | 外壳探测空闲端口，经 `--web-addr` / `--tcp-addr` 传入 hub（避免 hub 自动递增后外壳不知实际端口） |
| hub 启动参数 | `--config` / `--registry` / `--co-shell-path` / `--no-api-key` / `--web-addr` / `--tcp-addr` |
| 就绪判定 | 轮询 `http://127.0.0.1:<webPort>` 直到可连；超时 30s 显示错误页 |
| 退出回收 | `applicationWillTerminate` 中 `terminate()` hub 子进程 |
| 沙箱 | **不开**（Developer ID 直发路线），无 entitlements |
| 签名 | Apple Development 证书（本机可跑）；分发他人需 Developer ID 证书 |
| Go 代码改动 | **仅** `version`/`build`、`hubVersion`/`hubBuild` 字符串（发布管理），无功能代码改动 |

## 用例清单

### UC-01 (R) bundle 结构完整

- 断言：`co-shell.app/Contents/` 下存在 `Info.plist`、`MacOS/co-shell-app`、`Resources/bin/co-shell`、`Resources/bin/co-shell-hub`。
- 断言：`plutil -lint Info.plist` 通过；`CFBundleExecutable == co-shell-app`、`CFBundleIdentifier == com.coshell.app`。

### UC-02 (R) 签名有效且未启用沙箱

- 断言：`codesign --verify --strict co-shell.app` 退出码 0。
- 断言：`codesign -d --entitlements - co-shell.app` 中**不含** `com.apple.security.app-sandbox`。

### UC-03 (R) 内嵌二进制可独立运行

- 断言：`Contents/Resources/bin/co-shell --help` 正常输出；`Contents/Resources/bin/co-shell-hub --help` 正常输出且列出 `--web-addr` / `--registry` / `--co-shell-path`。

### UC-04 (M) 双击启动，窗口打开且 UI 加载成功

- 步骤：Finder 双击 `co-shell.app`（或 `open co-shell.app`）。
- 断言：出现应用窗口与 Dock 图标；窗口内容为 hub 的 Web UI（**非**白屏、**非**错误页）；窗口标题含 `co-shell`。

### UC-05 (M) hub 子进程被拉起

- 断言：`ps aux | grep co-shell-hub` 可见内嵌路径的 hub 进程，参数含 `--web-addr 127.0.0.1:<port>` 与 `--co-shell-path <bundle>/Contents/Resources/bin/co-shell`。

### UC-06 (M) 数据目录与 hub 日志创建

- 断言：`~/Library/Application Support/co-shell.app/` 存在（首次启动自动创建）。
- 断言：`~/Library/Logs/co-shell-app/hub.log` 存在且非空（含 hub 启动输出）。

### UC-07 (M) 端口冲突时不失败

- 前置：先手动占用一个端口，或另起一个 hub 实例占用默认端口。
- 断言：App 仍启动成功（外壳探测到的空闲端口生效）；Web UI 正常加载，不出现端口占用错误页。

### UC-08 (M) 在 UI 中跑通一个任务

- 步骤：在 Web UI 中创建/选择一个 agent，发送一条简单指令（如「列出当前工作区文件」）。
- 断言：agent 正常响应并返回内容；会话记录可在 UI 中看到。

### UC-09 (M) WKWebView 内 WebSocket 连通（流式输出）

- 步骤：发送一条会触发较长输出的指令。
- 断言：内容**流式**刷新（逐块出现），而非一次性整段返回 —— 证明 WKWebView 内 WebSocket 正常。

### UC-10 (M) 退出 App 后 hub 子进程被回收

- 步骤：⌘Q 退出应用。
- 断言：`ps aux | grep co-shell-hub` 不再有该内嵌 hub 进程（无残留孤儿进程）。

### UC-11 (M) 二次启动复用数据目录

- 步骤：退出后再次双击启动。
- 断言：启动成功；此前创建的 agent / 会话记录仍在（数据目录持久化生效）；无端口冲突。

### UC-12 (M) hub 异常退出时有可见提示

- 步骤：App 运行中，手动 `kill` 掉 hub 子进程。
- 断言：窗口出现可识别的错误提示（而非静默白屏或无限转圈）。

### UC-13 (R) 构建脚本幂等可重复

- 断言：连续执行两次 `app/build-app.sh`，两次均成功（第二次覆盖旧产物，无残留导致的失败）。

### UC-14 (R) 未改动 Go 功能代码

- 断言：`git diff main --stat` 中，Go 文件的改动**仅限** `main.go` 与 `cmd/co-shell-hub/main.go` 的 `version`/`build` 字符串；其余 Go 文件无改动。

## 备注

- **UC-07 / UC-12 为健壮性用例**：若最小原型阶段暂不实现，须在交付说明中显式标注为「未覆盖」，不得默认视为通过。
- **UC-08 / UC-09 依赖可用的 LLM 配置**：将复用宿主机已有的 `config.json` 或数据目录内的配置；若模型未配置，以「能打开会话界面并成功建立 WebSocket 连接」作为降级断言。
- 本原型**不开沙箱**，故不验证沙箱相关行为（沙箱结论见本会话此前的真机实测记录）。
