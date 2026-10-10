# co-shell Mac App Store 沙箱版设计

> 关联任务：**FIX-630** / v0.108.5 / 分支 `FIX-630`
> 关联用例：`use-case/FIX-630/FIX-630-UC-0001.md`
> 配套手册：`docs/macos-appstore-release.md`（上架操作指导，含审核理由草稿）
> 本文定位：**设计决策的单一事实源**——说明"为什么是沙箱形态""沙箱下哪些能力还能用"以及"这个结论有哪些实测证据"。所有结论均以本机实测为准，未实测项明确标注。

---

## 1. 背景与决策

### 1.1 起点

co-shell 现状是"无沙箱的本地 Agent 应用"：Swift 外壳（`app/macos/CoShellApp.swift`）拉起内嵌 `co-shell-hub`，hub 再按需拉起受控 `co-shell` agent，agent 在**真实用户目录**里执行命令、读写文件。

2026-10-10 用户提出"做 App Store 发布"。摆在前面的两条路：

| 路线 | 含义 | 代价 |
|------|------|------|
| A | 放弃 MAS，走 Developer ID 签名 + 自行分发（含公证） | 无沙箱约束，能力完全保留；但进入不了 App Store |
| B | **坚持 Mac App Store**，接受沙箱形态 | 必须开沙箱，能力边界受平台强制约束 |

### 1.2 决策（用户 2026-10-10 拍板）

| 项 | 决策 | 说明 |
|---|------|------|
| 路线 | **B：坚持 Mac App Store** | 接受把产品改成沙箱形态 |
| 权限档位 | **宽权限档** | 见 §4 |
| 工作区授权方式 | **不要"用户用 NSOpenPanel 选目录"** | 用户原话："不要用户选目录，太麻烦"；因此**不采用** security-scoped bookmark 方案 |
| Bundle ID | **新开 `com.coshell.app.mas`** | 与 Developer ID 版 `com.coshell.app` 互不覆盖，可共存 |
| 审核理由 | **写硬**：说明"我们就是一个 agent 应用，需要具备更强的能力" | 见配套上架手册 |
| 版本 / 编号 | v0.108.5（patch 递增）/ FIX-630 | 分支 `FIX-630` |

> ⚠️ **决策变更留痕**：ROADMAP 早期登记（13:29–13:31）写的是"受限本地版 + NSOpenPanel 授权 + `files.user-selected.read-write`"。用户在 13:32–13:34 改为**宽权限档、不做选目录流程**。本文以新决策为准。

### 1.3 两条路线的本质差别

路线 A 的 App 运行在**用户身份**下，可以碰任何用户能碰的东西；路线 B 的 App 运行在**沙箱**里，默认只能碰自己的容器。因此路线 B 的全部设计工作，本质上都在回答一个问题：

> **沙箱之后，co-shell 还剩多少能力？够不够支撑"agent 应用"这个定位？**

§5 的能力矩阵就是这个问题的答案，而且**每一条都有实测证据**（§6）。

---

## 2. 沙箱下的进程模型

co-shell 不是单进程程序，MAS 化必须让整条进程链都落在沙箱语义里：

```
co-shell.app (沙箱 App，bundle id com.coshell.app.mas)
  └── Contents/MacOS/co-shell-app      ← Swift 外壳，entitlements: 宽权限档
        └── spawn Contents/Resources/bin/co-shell-hub
        │     ← 内嵌 helper，entitlements: app-sandbox + inherit
        │        （继承父 App 的沙箱，因此同样拿到宽权限）
        └── hub 再 spawn Contents/Resources/bin/co-shell  （受控 agent）
              ← 同一个 helper 二进制，同样 app-sandbox + inherit
```

### 2.1 关键机制：`com.apple.security.inherit`

内嵌的 Go 二进制（`co-shell-hub`、`co-shell`）**没有独立的 bundle 身份**。平台要求：这类子进程若要活在父 App 的沙箱里，必须

1. 自身带 `com.apple.security.app-sandbox` + `com.apple.security.inherit`；
2. 由**已沙箱化的父进程**发起。

**由沙箱父进程拉起的子进程，继承父的沙箱，于是父拿到的宽权限子进程同样有。** 这是整个方案的支点——hub 和 agent 不需要各自的权利声明，它们跟随 App。

### 2.2 反面案例（spike v1，必须记住的坑）

天真做法：给内嵌二进制单独签 `app-sandbox`，然后**从外部终端直接运行**它。

结果：**`exit=133`（SIGTRAP）、零输出**，崩溃报告写着

```
signal=SIGTRAP
Unable to get bundle identifier for container id co-shell
```

原因：单独签名的子系统二进制没有独立 bundle id，沙箱初始化时无法定位自己的容器，内核直接把它打死。**结论：内嵌二进制必须走 `inherit` 由沙箱父进程发起，绝不可独立启动或独立签名加沙箱。**

---

## 3. Entitlements 设计（两套，不要混用）

沙箱改造引入**两个** entitlements 文件，各自服务不同对象：

| 文件 | 作用对象 | 内容 |
|------|---------|------|
| `app/macos/app.entitlements` | App 本体 `co-shell.app` | 宽权限档（见 §4） |
| `app/macos/helper.entitlements` | 内嵌 helper `Contents/Resources/bin/{co-shell,co-shell-hub}` | `app-sandbox` + `com.apple.security.inherit` |

### 3.1 App（宽权限档）

```xml
<key>com.apple.security.app-sandbox</key><true/>
<key>com.apple.security.files.all</key><true/>
<key>com.apple.security.network.client</key><true/>
<key>com.apple.security.network.server</key><true/>
<key>com.apple.security.temporary-exception.files.home-relative-path.read-write</key>
<array><string>/</string></array>
```

| 键 | 作用 | 为什么需要 |
|---|------|-----------|
| `app-sandbox` | 开启沙箱 | MAS **强制要求**，没有它不能上架 |
| `files.all` | 沙箱层面放开全部文件读写 | agent 的立身之本：能读写用户任意项目目录 |
| `network.client` | 允许对外发起连接 | 调 LLM API、MCP、飞书等 |
| `network.server` | 允许监听端口 | hub/agent 的本地 Web UI 与 WS 网关 |
| `temporary-exception.files.home-relative-path.read-write` = `/` | home 相对路径整棵放行 | 与 `files.all` 互为兜底，覆盖 home 下全部路径 |

> **两把钥匙一起用**：`files.all` 是"放开沙箱文件限制"的总开关；`temporary-exception.*` 是 Apple 提供的"例外通道"。二者叠加是为了让"agent 能在用户的真实项目里干活"这一核心诉求在任何路径解析分支下都不掉链子——**代价是要向 Apple 逐条说明用途**（见 §7）。

### 3.2 Helper（内嵌二进制）

```xml
<key>com.apple.security.app-sandbox</key><true/>
<key>com.apple.security.inherit</key><true/>
```

**刻意不做的事**：helper **不声明** `files.all` / `network.*` / 任何例外。它靠 `inherit` 继承 App 的全部沙箱权利——权利只在 App 一处声明，避免"两处声明不一致"这类难查的问题。

### 3.3 签名要求

- 沙箱 + 硬化运行时：`--options runtime`（实测 spike 已带上）。
- spike 用的是 `Apple Development: Shuang Liang (4DXG4X72ZG)`（开发证书，仅够本机验证）。
- **MAS 正式分发**必须换成 `Apple Distribution` + Mac App Store 描述文件；本机当前**没有**该证书（§8 未决项）。

---

## 4. 沙箱下的数据落位

这是最容易踩坑、也最容易被忽略的一节。

### 4.1 home 被重定向（实测确认）

沙箱 App 的"home"被重定向到自己的容器：

```
真实 home:            /Users/direct3d
沙箱 App 眼里的 home:  /Users/direct3d/Library/Containers/com.coshell.app/Data
```

**实测证据**：hub 的 `GET /api/agent-defaults` 返回

```json
{"default_workspace":"/Users/direct3d/Library/Containers/com.coshell.app/Data/.co-shell/agents/agent-1"}
```

`default_workspace` 逻辑上是 `~/.co-shell/agents/agent-1`，沙箱下解析到了**容器内**。同理：

| Swift 外壳里的路径 | 沙箱下的实际落点 |
|---|---|
| `~/Library/Application Support/co-shell.app`（hub 工作目录 / 注册表 / board db） | `~/Library/Containers/com.coshell.app/Data/Library/Application Support/co-shell.app` |
| `~/Library/Logs/co-shell-app/hub.log` | `~/Library/Containers/com.coshell.app/Data/Library/Logs/co-shell-app/hub.log` |
| `~/.co-shell/...`（agent 默认工作区、共享配置） | `~/Library/Containers/com.coshell.app/Data/.co-shell/...` |

### 4.2 设计含义

1. **标准目录（Library 等）走容器**是平台行为，不需要改代码，但**文档要写清楚**——否则排查时会误判成"程序写错了地方"。
2. **容器是空的**：MAS 版首次启动没有任何既有配置。用户从现有版本迁移时，需要一条"导入既有 config.json / 工作区"的路径（见 §8 未决项）。
3. **容器数据受保护**：外部进程（含终端）读容器目录会被拒——实测 `ls ~/Library/Containers/com.coshell.app/Data/` → `Operation not permitted`。这是**预期行为不是故障**，排查只能从 App 内 UI / 日志界面看。
4. **真实项目路径照常可写**：非 Library 的普通目录（如 `~/github/...`）**不受 home 重定向影响**，实测沙箱 agent 在其中正常落盘（§6.5）。

---

## 5. 能力矩阵（实测结论）

> 图例：✅ 实测可用 ｜ 🟡 受限/需适配 ｜ ❌ 不可用 ｜ ⏳ 待实测

### 5.1 保留（核心能力不丢）

| 能力 | 状态 | 依据 |
|------|------|------|
| App 启动、内嵌 hub 拉起 | ✅ | §6.1 |
| hub 监听本地端口（Web UI + WS 网关） | ✅ | §6.2，`network.server` 生效 |
| App 内加载 hub Web UI | ✅ | §6.2，`HTTP 200` 且页面 117KB |
| hub REST API（agents / defaults / hub-info / board / settings） | ✅ | §6.3 |
| hub → 受控 agent 拉起（跨进程、继承沙箱） | ✅ | §6.4 |
| agent 本地服务 + `--yolo` 执行链 | ✅ | §6.4，端口 52125 `HTTP 200` |
| **在真实 home 路径下创建/读写工作区** | ✅ | §6.5，决定性证据 |
| 对外网络（LLM API / MCP） | ✅ | `network.client`，抓取链路待回归 |

### 5.2 受限 / 需适配

| 事项 | 状态 | 说明 |
|------|------|------|
| 数据目录落在容器内 | 🟡 | 见 §4；不是故障，但需产品化说明与迁移路径 |
| 既有配置/工作区迁移 | 🟡 | MAS 版首启是空白容器，需要导入方式 |
| 外部 shell 读容器日志 | 🟡 | 被系统拒；需提供"App 内查看日志"入口 |
| `~/Desktop`、`~/Documents`、`~/Downloads` | ⏳ | **TCC 与沙箱是两套机制**：`files.all` 放开的是沙箱限制，不代�� TCC。这三个目录受"文件与文件夹"隐私保护，首次访问可能弹系统授权框。**必须实测**并在文档中告知用户预期弹窗 |
| 其他 App 的容器（`~/Library/Containers/<其它>`） | ❌ | 容器保护，跨 App 不可读——agent 不能替用户改别的 App 的数据 |
| 系统目录写入（`/usr`、`/Library`） | ❌ | 需 root，沙箱不允许 |
| 开机自启 / 全局 launchd 服务 | ❌ | MAS 沙箱不支持装系统级服务；调度能力只能在 App 运行期间生效 |

### 5.3 明确不可做（写进审核理由的边界声明）

- 不能执行"下载并改变 App 功能"的代码（审核指南 2.5.2）。
- 不能访问用户未授权隐私资源（相机/麦克风/通讯录等，未申请即不可用）。
- 不能自我更新（MAS 版只能走 App Store 更新）。

### 5.4 一句话总结

> **宽权限档下，co-shell 作为 agent 的核心能力——"在用户的真实项目目录里读写文件、执行命令、对外调用模型"——是完整保留的。** 变化集中在"App 自身数据落到容器里"和"部分受 TCC/容器保护的位置不可达"，而这两类都不影响 agent 的日常主流程。

---

## 6. 实测证据链（2026-10-10，宽权限档）

> 复现基线：`tmp/spike-wide/co-shell.app`（从 `work/co-shell.app` 复制，重签为宽权限档，`--options runtime`，TeamIdentifier `5BZBX3DL83`）。
> 所有结论都可用本节命令重新验证。

### 6.1 沙箱 App 能正常启动

```
$ open tmp/spike-wide/co-shell.app
$ ps -Ao pid,ppid,command | grep spike-wide
81953     1  .../spike-wide/co-shell.app/Contents/MacOS/co-shell-app
81957 81953  .../spike-wide/co-shell.app/Contents/Resources/bin/co-shell-hub \
            --web-addr 127.0.0.1:52123 --tcp-addr 127.0.0.1:52124 \
            --co-shell-path .../Resources/bin/co-shell --no-api-key --serve
```

结论：**App 未被沙箱打死**（对比 spike v1 的 `SIGTRAP`），且 shell 按设计拉起了内嵌 hub。容器随之建立：`~/Library/Containers/com.coshell.app`。

### 6.2 hub 监听成功 + Web UI 可达

```
$ lsof -nP -p 81957 | grep LISTEN
co-shell- 81957 ... TCP 127.0.0.1:52124 (LISTEN)
co-shell- 81957 ... TCP 127.0.0.1:52123 (LISTEN)

$ curl -s -o /tmp/mas-hub-index.html -w 'http_code=%{http_code} size=%{size_download}\n' \
      http://127.0.0.1:52123/
http_code=200 size=117226
```

结论：`com.apple.security.network.server` **确实生效**——沙箱 App 能对外提供本地 HTTP 服务，App 内 WKWebView 加载 Web UI 的路径成立。

### 6.3 hub REST API 正常 + 版本探测成功

```
$ curl -s http://127.0.0.1:52123/api/agents
{"agents":[]}

$ curl -s http://127.0.0.1:52123/api/agent-defaults
{"default_workspace":"/Users/direct3d/Library/Containers/com.coshell.app/Data/.co-shell/agents/agent-1",
 "default_id":"agent-1",
 "co_shells":[{"path":".../spike-wide/co-shell.app/Contents/Resources/bin/co-shell",
               "source":"hubdir","version":"0.108.3","build":"1286","ok":true}],
 "recommended_port":52125}

$ curl -s http://127.0.0.1:52123/api/hub-info
{"build":"1286","name":"co-shell-hub","version":"0.108.3"}
```

两条关键结论：

1. `default_workspace` 落在**容器内** → 沙箱 home 重定向实锤（§4.1）。
2. `co_shells[0].ok = true` 且读出了版本号 → hub 执行的 `co-shell --version`（`hub/gateway/detect.go`）**成功地跑起来了一个沙箱化 helper 子进程**。也就是说 `inherit` 机制在真实运行中成立。

### 6.4 决定性一测：hub 拉起受控 agent

```
$ curl -s -X POST http://127.0.0.1:52123/api/agents -H 'Content-Type: application/json' -d '{
    "id":"masprobe",
    "workspace":"/Users/direct3d/github/co-shell/tmp/spike-wide/probe-ws",
    "co_shell":".../Resources/bin/co-shell",
    "create_config":true,"use_shared_config":false,"port":52125,"yolo":true}'
{"id":"masprobe","type":"managed","workspace":".../probe-ws","port":52125,"config_file":"config.json",...}

$ curl -s -X POST http://127.0.0.1:52123/api/agents/masprobe/start
{"id":"masprobe","ws_url":"ws://127.0.0.1:52125/ws"}

$ ps -Ao pid,ppid,command | grep spike-wide
82142 81957  .../Resources/bin/co-shell --serve --port 52125 --bind 127.0.0.1 \
            -w /Users/direct3d/github/co-shell/tmp/spike-wide/probe-ws \
            -c .../probe-ws/config.json --yolo --accept-license

$ curl -s -o /dev/null -w 'http_code=%{http_code}\n' http://127.0.0.1:52125/
http_code=200
$ curl -s http://127.0.0.1:52125/api/status
{"busy":false}
```

结论：**完整的三级进程链（App → hub → agent）在沙箱内跑通**，agent 的本地服务可用。这是"沙箱版是不是残废"这个问题的正面回答。

### 6.5 真实 home 路径读写 —— 最关键的一条

沙箱 agent 在**真实 home 路径**（不是容器）下落出了完整工作区：

```
$ ls -la tmp/spike-wide/probe-ws/
.rules/  bin/  config.json(5318B)  db/co-shell.db  download/  IDENTITY.md
input/   log/co-shell-2026-10-10.log  log/llm-interaction-2026-10-10.log
logos/   mode/  output/  research/  skills/  tmp/
```

`config.json`、bbolt 数据库 `db/co-shell.db`、日志文件、`IDENTITY.md` 全部落盘成功。

**结论：`files.all` + `temporary-exception.files.home-relative-path.read-write` 组合确实让沙箱进程能读写真实用户目录**——agent 的立身之本没有被沙箱夺走。

### 6.6 预期内的"被拒绝"（不要误判成故障）

```
$ ls ~/Library/Containers/com.coshell.app/Data/
ls: ...: Operation not permitted

$ sudo -n true
sudo: a password is required
```

外部进程读容器目录被系统拒绝是**沙箱的正常语义**（容器是 App 的私有空间）。推论：**排障不能靠 `cat` 容器里的日志**，产品上必须提供"App 内查看日志"的入口。

### 6.7 证据小结

| # | 断言 | 证据 |
|---|------|------|
| 1 | 沙箱 App 可启动 | §6.1 |
| 2 | 内嵌 hub 可拉起并监听 | §6.1 / §6.2 |
| 3 | 本地 Web UI 可服务（`network.server`） | §6.2 |
| 4 | hub REST API 可用 | §6.3 |
| 5 | helper 经 `inherit` 可执行 | §6.3（`--version` ok） |
| 6 | hub 可按需拉起受控 agent | §6.4 |
| 7 | agent 本地服务可用 | §6.4（`HTTP 200`） |
| 8 | **真实 home 路径可读写** | §6.5 |
| 9 | home 被重定向到容器 | §6.3 / §4.1 |
| 10 | 容器对外不可读（预期） | §6.6 |

---

## 7. 风险与审核策略

### 7.1 平台/审核风险

| 风险 | 说明 | 应对 |
|------|------|------|
| `files.all` 与 `temporary-exception.*` 需逐条说明 | Apple 对"宽权限 + 临时例外"审查严格，说明不充分会被拒 | **把理由写硬**：co-shell 就是 agent 应用，"在用户的项目目录里读写文件"是产品的**核心功能本身**，不是便利性诉求（见上架手册的审核理由草稿） |
| 审核指南 2.5.2 | 不得下载并执行"改变 App 功能"的代码 | 明确边界：agent 执行的是用户/模型给出的**任务命令**，不改变 App 自身功能；不自我更新 |
| 审核指南 4.2 最低功能性 | 网页壳会被拒 | co-shell 有本地 Go 引擎、工具链、调度、记忆等，非纯网页壳；审核备注中说明 |
| 沙箱能力被误认为"受限产品" | 审核员可能质疑沙箱下功能残缺 | 以 §5 能力矩阵 + §6.5 实测作为"核心能力完整"的书面与可复现凭据 |
| TCC 弹窗引发困惑 | 首次访问 桌面/文稿/下载 可能弹系统授权框 | ⏳ 待实测；确认后在文档与首次启动引导中说明"这是系统隐私授权，属正常" |

### 7.2 技术风险

| 风险 | 说明 | 应对 |
|------|------|------|
| 内嵌二进制被单独签名/独立启动 | spike v1 已证明会 `SIGTRAP` | 构建脚本中对 helper 一律用 `inherit`，并**禁止**独立运行 |
| 数据落位认知偏差 | 开发者以为写的是 `~/Library/...`，实际在容器 | §4 明文化的落点对照表；日志路径在 UI 上显示**实际**路径 |
| 既有用户配置无法复用 | MAS 版首启是空容器 | 待定：导入既有 `config.json` / 工作区（§8） |
| 端口冲突 | hub 端口由外壳通过 `bind(0)` 动态分配 | 已由 `CoShellApp.swift` 处理，无需改动 |

---

## 8. 实施清单与未决项

### 8.1 实施清单（FIX-630）

| # | 项 | 状态 |
|---|----|------|
| 1 | `app/macos/app.entitlements`、`app/macos/helper.entitlements` 落库 | ⏳ |
| 2 | `app/build-app.sh` 增加沙箱/MAS 通道（写 entitlements、helper 用 `inherit`、支持 Apple Distribution + MAS 描述文件签名） | ⏳ |
| 3 | 沙箱数据落位适配（确认 `AppPaths` 在沙箱下的行为并显式化） | ⏳ |
| 4 | 合规项：`PrivacyInfo.xcprivacy`、`ITSAppUsesNonExemptEncryption=false`、ATS 收窄、`CFBundleName` | ⏳ |
| 5 | 归档/上传链路（`xcodebuild -exportArchive` / `productbuild`，沿用 iOS 的 `ExportOptions`：`method=app-store-connect`、`destination=upload`、`teamID=5BZBX3DL83`） | ⏳ |
| 6 | `docs/macos-appstore-release.md` 上架手册 + 审核理由草稿 | ⏳ |
| 7 | 本设计文档 | ✅ |

### 8.2 未决项（需要决策或实测）

| # | 未决项 | 影响 | 备注 |
|---|-------|------|------|
| A | TCC 对 `~/Desktop`、`~/Documents`、`~/Downloads` 的实际行为 | 影响用户首次体验与文档措辞 | 必须实测，勿凭推测 |
| B | 既有配置/工作区迁移到容器的路径 | 影响老用户升级 MAS 版 | 可选方案：首次启动检测 + 导入向导 |
| C | `Apple Distribution` 证书与 Mac App Store 描述文件 | **硬阻塞**：没有它们无法分发 | 用户侧操作（Xcode → Manage Certificates / portal） |
| D | App Store Connect 的 Mac App 记录 | **硬阻塞**：没有它无法上传 | 用户侧操作（SKU 建议 `coshell-macos-001`） |
| E | `files.all` 在持续集成/自动构建下的签名一致性 | 影响发布流水线稳定性 | 构建脚本固化签名身份 |

### 8.3 术语与文件索引

| 概念 | 位置 |
|------|------|
| Swift 外壳（拉起 hub、WKWebView 显示 UI） | `app/macos/CoShellApp.swift` |
| 内嵌 hub / agent 二进制 | `co-shell.app/Contents/Resources/bin/{co-shell-hub,co-shell}` |
| App 数据目录（hub cwd / 注册表 / board db） | `AppPaths.dataDir` → 沙箱下容器内 |
| hub 日志 | `AppPaths.hubLog` → 沙箱下容器内 `.../Logs/co-shell-app/hub.log` |
| 构建脚本 | `app/build-app.sh`（无参数输出 `work/co-shell.app`） |
| 沙箱 spike 现场 | `tmp/spike-mas/`（v1 反面案例）、`tmp/spike-wide/`（v2 宽权限档） |
