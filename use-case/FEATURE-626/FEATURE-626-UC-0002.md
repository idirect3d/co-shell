# FEATURE-626 UC-0002：App 启动内嵌 hub 时传 --serve（不再额外弹系统浏览器）

> 分支：**FEATURE-626**　版本：**v0.108.1**　BUILD：**待编译**
> 需求（用户 2026-10-10 10:06 提出）："稍微改一下 app，启动 hub 时，需要传 —serve 参数，以便不开浏览器。"
> 归属确认（用户 2026-10-10 10:07）：沿用当前分支 FEATURE-626，作为该任务的追加改动，版本 v0.108.1，一起提交、一起等确认。
> 验证方式确认（用户 2026-10-10 10:07）：构建到独立目录（不碰 work/co-shell.app），执行方仅做构建 + 参数静态验证；由用户手动双击新 App 体验。

## 背景与现状

双击 `co-shell.app` 时，App（Swift shell）用 WKWebView 窗口显示 hub 的 Web UI；同时它拉起的 `co-shell-hub` **未带 `--serve`**，hub 因此又调用 `web.OpenBrowser` 打开一次系统默认浏览器 —— 表现为「同一个页面出现两份」（App 窗口 + 系统浏览器）。

- hub 侧逻辑：`cmd/co-shell-hub/main.go:338-346`——`if !cfg.Serve { web.OpenBrowser(browserURL(cfg.WebAddr, scheme)) }`。
- App 侧参数：`app/macos/CoShellApp.swift:387-392`——`--web-addr / --tcp-addr / --co-shell-path / --no-api-key`，**缺 `--serve`**。

## 设计基线（本次改动基准）

| 项 | 设计 |
|----|------|
| 改动点 | `app/macos/CoShellApp.swift` 的 `startHub()` 中 `proc.arguments` 追加 `"--serve"` |
| 注释 | 在该参数旁注明：App 自身用 WKWebView 展示同一页面，无需系统浏览器 |
| 不改动 | Go 代码（main.go / cmd/co-shell-hub / hub/gateway）、hub 的 `--serve` 语义、App 其它参数与逻辑、是否新增界面开关（不做） |
| 版本 | v0.108.1；main.go `build` 与 cmd/co-shell-hub `hubBuild` 同步 +1（App 的 Info.plist 构建时从 main.go 读取） |
| 构建 | `app/build-app.sh [独立输出目录]`（默认输出 `work/`，本次**不覆盖运行中的 `work/co-shell.app`**） |

## 术语

- **(R)** 离线静态校验（读源码 / 检查产物 / 比对文件），执行方可直接完成。
- **(M)** 运行时手动断言，需用户双击 App 并在界面/终端观察后确认。
- **受控 agent**：hub 按 registry 拉起的 `co-shell --serve ...` 子进程。

## 用例清单

### UC-01 (R) App 启动参数含 --serve

- 断言：`app/macos/CoShellApp.swift` 中 `proc.arguments` 数组包含元素 `"--serve"`。
- 断言：原有四个参数 `--web-addr`、`--tcp-addr`、`--co-shell-path`、`--no-api-key` 及其取值方式保持不变。
- 验证命令：
  ```bash
  sed -n '380,395p' app/macos/CoShellApp.swift
  ```

### UC-02 (R) 构建成功且产物可用

- 断言：`app/build-app.sh <独立输出目录>` 退出码为 0，`swiftc` 无错误，`codesign` 成功。
- 断言：产物为可双击的 bundle：`<独立输出目录>/co-shell.app/Contents/MacOS/co-shell-app` 存在且为可执行文件。
- 断言：`Contents/Resources/bin/` 下同时存在 `co-shell` 与 `co-shell-hub`。
- 验证命令：
  ```bash
  app/build-app.sh tmp/app-verify
  ls -l tmp/app-verify/co-shell.app/Contents/MacOS/ tmp/app-verify/co-shell.app/Contents/Resources/bin/
  codesign -dv tmp/app-verify/co-shell.app 2>&1 | head -5
  ```

### UC-03 (R) 未改动 hub 的 --serve 语义（hub 行为不回归）

- 断言：`cmd/co-shell-hub/main.go` 中 `if !cfg.Serve { ... OpenBrowser ... }` 逻辑保持原样（本次不动 Go 代码）。
- 断言：`git diff main.go cmd/co-shell-hub/main.go` 中除 build/version 计数外无逻辑改动。
- 说明：不带 `--serve` 手动启动 hub 仍应打开浏览器（该行为属既有功能，本次只改 App 的传参）。

### UC-04 (R) 版本一致性与构建计数

- 断言：`main.go` 的 `build` 与 `cmd/co-shell-hub/main.go` 的 `hubBuild` 相等，且为改动前的值 +1。
- 断言：App bundle 的 `CFBundleShortVersionString` / `CFBundleVersion` 等于 main.go 的 `version` / `build`（由 build-app.sh 写入）。
- 验证命令：
  ```bash
  grep -nE '^\s*(version|build)\s*=' main.go
  grep -nE 'hubVersion|hubBuild' cmd/co-shell-hub/main.go
  /usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' tmp/app-verify/co-shell.app/Contents/Info.plist
  /usr/libexec/PlistBuddy -c 'Print CFBundleVersion' tmp/app-verify/co-shell.app/Contents/Info.plist
  ```

### UC-05 (R) 改动范围最小

- 断言：本次 `git diff --name-only`（相对改动前）只含 `app/macos/CoShellApp.swift`、`ROADMAP.md`、`main.go`、`cmd/co-shell-hub/main.go`（后两者仅构建计数）。
- 断言：未改动 `app/build-app.sh`、`app/macos/Info.plist`、`hub/` 与 `web/` 任何文件。

### UC-06 (M) 双击新 App：无系统浏览器被打开

- 前置：先记录当前浏览器进程基线。
  ```bash
  ps -Ao pid,lstart,command | grep -iE "Safari|Chrome|Firefox|Edge" | grep -v grep
  ```
- 步骤：双击 `<独立输出目录>/co-shell.app`。
- 断言：**没有**新的浏览器窗口/标签被打开指向 `http://127.0.0.1:<web端口>`；浏览器进程列表与基线一致（无新实例）。
- 断言：终端**不**出现既有的「Web 界面已启动」之类的浏览器拉起副作用（本条以窗口观察为准）。

### UC-07 (M) 新 App 拉起的 hub 命令行含 --serve

- 命令：
  ```bash
  ps -Ao pid,ppid,command | grep co-shell-hub | grep -v grep
  ```
- 断言：新 App 的 hub 子进程命令行**包含 `--serve`**（对照改动前只有 `--web-addr/--tcp-addr/--co-shell-path/--no-api-key`）。
- 断言：hub 仍在监听 App 分配的 `--web-addr` 端口。

### UC-08 (M) App 窗口内页面正常（功能不回归）

- 断言：App 窗口内 WKWebView 正常加载 hub 外壳页面（agent 切换栏可见、非空白）。
- 断言：页面处于「已连接」状态，agent 列表可显示。
- 断言：hub 端口可从本机访问：
  ```bash
  curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<web端口>/
  ```
  返回 `200`。

### UC-09 (M) 受控 agent 仍以 --serve 启动（回归）

- 命令：
  ```bash
  ps -Ao pid,ppid,command | grep "co-shell --serve" | grep -v grep
  ```
- 断言：hub 拉起的 agent 命令行仍含 `--serve`（agent 侧本来就不弹浏览器，本次不应改变）。
- 断言：agent 的 Web UI 在 App 内可正常使用（发一条指令有响应）。

### UC-10 (M) 退出 App 时 hub 子进程被终止（回归）

- 步骤：退出新构建的 App（Cmd+Q）。
- 断言：`ps` 中该 App 对应的 `co-shell-hub` 进程消失（原有 `terminationHandler` / 退出清理行为不回归）。
- 断言：`work/co-shell.app`（正在运行的原 App）与当前会话**不受影响**（本次验证全程使用独立输出目录）。

## 用例结果记录

| 用例 | 类型 | 结果 | 备注 |
|------|------|------|------|
| UC-01 | R | ✅ 通过 | 源码参数含 `--serve`，其余四参数不变 |
| UC-02 | R | ✅ 通过 | `app/build-app.sh tmp/app-verify` 成功；bundle/签名/内嵌二进制齐备（v0.108.1 build 1284） |
| UC-03 | R | ✅ 通过 | hub 侧 `if !cfg.Serve` 逻辑未改；Go 代码零逻辑改动 |
| UC-04 | R | ✅ 通过 | `main.go` build 与 `hubBuild` = 1284；Info.plist = 0.108.1 / 1284 |
| UC-05 | R | ✅ 通过 | 改动仅 `CoShellApp.swift` + `ROADMAP.md` + 两处构建号 + 本用例文件 |
| UC-06 | M | ✅ 用户确认 | 用户 2026-10-10 10:13 确认测试通过并指示合并 |
| UC-07 | M | ✅ 用户确认 | 同上（hub 命令行含 `--serve`） |
| UC-08 | M | ✅ 用户确认 | 同上（App 内页面正常） |
| UC-09 | M | ✅ 用户确认 | 同上（受控 agent 仍为 `--serve`） |
| UC-10 | M | ✅ 用户确认 | 同上（退出 App 回收 hub 子进程） |
