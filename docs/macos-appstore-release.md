# co-shell Mac App Store 上架指导

> 适用范围：`app/` 下的 Swift 外壳 + 内嵌 Go 引擎（Bundle ID **`com.coshell.app.mas`**，Team `5BZBX3DL83`）
> 关联任务：**FIX-630 / v0.108.5**
> 设计依据：`docs/macos-appstore-sandbox-design.md`（沙箱能力矩阵、entitlements 取舍、实测证据链）
> 本机构建环境：Xcode 27.0 / macOS 27.0.1 / Go 1.25（`go` 在 `/opt/homebrew/bin`）

---

## 0. 全流程一览

```
① 备齐证书与描述文件  →  ② 建 App Store Connect App 记录  →  ③④ 构建沙箱版并打包
   →  ⑤ 上传  →  ⑥ 填元数据 + 审核备注  →  ⑦ 提交审核
```

③④ 已脚本化，一条命令完成：

```bash
export PATH="$PATH:/opt/homebrew/bin"
app/mas-package.sh          # 产物：work/mas/co-shell-<版本>.pkg
```

---

## 1. 前置条件（缺一不可）

| 项 | 说明 | 怎么确认 |
|---|---|---|
| Apple Developer Program（付费） | 免费账号不能上架 | developer.apple.com → Membership details |
| **Apple Distribution 证书** | 给 App 本体签名 | `security find-identity -v \| grep "Apple Distribution"` |
| **Mac Installer Distribution 证书** | 给 `.pkg` 签名 | `security find-identity -v \| grep "3rd Party Mac Developer Installer"` |
| **Mac App Store 描述文件** | 绑定 App ID `com.coshell.app.mas` | portal → Profiles，或由 Xcode 自动生成 |
| App Store Connect App 记录 | 上传的前提条件 | ASC → 我的 App |
| 演示用 LLM API Key | 审核员要能真的把 App 跑起来 | 见 §5 |

### 1.1 本机现状（2026-10-10 实测）

| 项 | 状态 |
|---|---|
| 账号 | ✅ 付费 Program，Team `5BZBX3DL83`（iOS 版已分发过，资格没问题） |
| 钥匙串 | ❌ **只有** `Apple Development: Shuang Liang (4DXG4X72ZG)`；缺 `Apple Distribution` 与 `3rd Party Mac Developer Installer` |
| 描述文件 | ❌ 只有 2 个 iOS 的；**无任何 macOS 描述文件** |
| App Store Connect | ❌ 尚无 Mac App 记录 |

> 这三条是**硬阻塞**：没有它们，§4 的打包与上传走不通。
> `app/mas-package.sh` 在开工前就会检查证书并明确报错，**绝不会**悄悄退回开发证书、签出一个看起来正常但传不上去的包。

---

## 2. 备齐证书与描述文件（用户操作，约 10 分钟）

### 2.1 建两张证书

Xcode → **Settings → Accounts** → 选中 Team `5BZBX3DL83` → **Manage Certificates…** → 左下 **+**

| 点选 | 得到 | 用途 |
|---|---|---|
| **Apple Distribution** | `Apple Distribution: Shuang Liang (5BZBX3DL83)` | 给 App 签名 |
| **Mac Installer Distribution** | `3rd Party Mac Developer Installer: Shuang Liang (5BZBX3DL83)` | 给 `.pkg` 签名 |

> ⚠️ 私钥只存在登录钥匙串里。**任何情况下不要把导出的 `.p12` 外传、入 git、放第三方 CI 明文。**

验证：

```bash
security find-identity -v | grep -E "Apple Distribution|3rd Party Mac Developer Installer"
```

### 2.2 建 App ID 与描述文件

1. <https://developer.apple.com/account/resources/identifiers> → **+** → App IDs → App
   - Description：`co-shell macOS`
   - Bundle ID（Explicit）：**`com.coshell.app.mas`**
   - 不需要勾选任何 Capability（沙箱与文件权限由 entitlements 表达）
2. <https://developer.apple.com/account/resources/profiles> → **+** → **Mac App Store** → 选 App ID `com.coshell.app.mas` → 选上面那张 Apple Distribution 证书 → 生成并下载

下载到的 `.provisionprofile` 交给构建脚本内嵌：

```bash
CO_SHELL_MAS_PROFILE=~/Downloads/xxx.provisionprofile app/mas-package.sh
```

> 不设置 `CO_SHELL_MAS_PROFILE` 时脚本会告警并继续（便于本机验证）；**正式上传必须带上**。

### 2.3 建 App Store Connect App 记录

<https://appstoreconnect.apple.com> → 我的 App → **+** → 新建 App

| 字段 | 填什么 |
|---|---|
| 平台 | **macOS** |
| 名称 | `co-shell`（App Store 名称全局唯一，被占用就换，如 `co-shell – 自建 AI 终端`） |
| 主要语言 | 简体中文 |
| Bundle ID | `com.coshell.app.mas`（下拉里应已出现） |
| SKU | `coshell-macos-001` |
| 用户访问权限 | 完全访问 |

---

## 3. 构建沙箱版

```bash
export PATH="$PATH:/opt/homebrew/bin"

# 只构建，不打包（调试用）
CO_SHELL_MAS=1 app/build-app.sh work/mas

# 构建 + 打包签名（正式流程，见 §4）
app/mas-package.sh                    # 默认输出 work/mas/
```

`CO_SHELL_MAS=1` 会切换出一整套沙箱形态（细节见设计文档 §3）：

| 产物 | 变化 |
|---|---|
| Bundle ID | `com.coshell.app.mas`（与 Developer ID 版 `com.coshell.app` 互不覆盖，可共存） |
| App 签名 | `app/macos/app.entitlements`（宽权限档）+ `--options runtime` |
| helper 签名 | `app/macos/helper.entitlements`（`app-sandbox` + `inherit`） |
| 隐私清单 | `Contents/Resources/PrivacyInfo.xcprivacy` |
| 描述文件 | 由 `CO_SHELL_MAS_PROFILE` 指定，内嵌为 `Contents/embedded.provisionprofile` |

**不带 `CO_SHELL_MAS=1` 时行为与改动前完全一致**（非沙箱开发版）。

自查：

```bash
codesign -d --entitlements - work/mas/co-shell.app                  # 应有 app-sandbox / files.all / network.*
codesign -d --entitlements - work/mas/co-shell.app/Contents/Resources/bin/co-shell   # 应只有 app-sandbox / inherit
codesign --verify --deep --strict work/mas/co-shell.app             # 应通过
```

---

## 4. 打包与上传

### 4.1 打包

```bash
app/mas-package.sh
```

脚本做四件事：① 校验两张证书是否在钥匙串里（缺了就报错退出，**不会**用开发证书凑合）；② 构建沙箱版；③ 核对 bundle id 是 `com.coshell.app.mas` 且确实带沙箱 entitlements（否则拒绝打包）；④ `productbuild` 用 `3rd Party Mac Developer Installer` 签出 `.pkg`。

产物：`work/mas/co-shell-<版本>.pkg`

### 4.2 上传

择一：

```bash
# 方式 A：命令行（需要 App 专用密码）
xcrun altool --upload-app -f work/mas/co-shell-0.108.5.pkg --type macos \
             -u <apple-id> -p <app-specific-password>

# 方式 B：Transporter.app（图形界面，把 .pkg 拖进去即可，最省事）
open -a Transporter work/mas
```

> App 专用密码在 <https://appleid.apple.com> → 登录与安全 → App 专用密码 生成。

上传成功后回到 App Store Connect，等构建版本处理完（几分钟），在「构建版本」里选中它。

---

## 5. 上架元数据清单

| 项 | 要求 | 备注 |
|---|---|---|
| 截图 | 4 档：**1280×800 / 1440×900 / 2560×1600 / 2880×1800** | macOS 要求 |
| 描述 | 讲清"本地 AI Agent"，别写成网页壳 | |
| 关键词 | 100 字符内 | |
| 支持 URL | 必填 | |
| **隐私政策 URL** | 必填 | 沙箱 + 宽文件权限尤其要有 |
| 年龄分级 | 按问卷填 | |
| 出口合规 | 已在 `Info.plist` 声明 `ITSAppUsesNonExemptEncryption=false` | 免去每次上传被问 |
| **演示 LLM API Key** | **必须提供** | 审核员没有 key 就跑不起来，容易被判"功能不可用"而拒 |
| **审核备注** | 见 §6 | 这是本次上架成败的关键之一 |

---

## 6. 审核理由（草稿）

> 用途：粘贴到 App Store Connect → App 审核信息 → **备注（Notes）**。
> 英文版给审核团队，中文版备查。核心口径：**co-shell 就是一个 agent 应用，宽文件权限是它的功能本身，不是便利性诉求。**

### 6.1 English (for App Review)

```
co-shell is a LOCAL AI AGENT application. It is not a website wrapper: it ships a
native engine (Go) plus a Swift shell, runs entirely on the user's own Mac, and
the developer operates no backend servers.

WHY WE REQUEST com.apple.security.files.all AND THE FILE TEMPORARY EXCEPTION

Reading and writing files in the user's own project directories, at the user's
explicit direction, IS the product's core function -- not a convenience feature.
The user opens the app, points it at a project they own, and asks the agent to
work on it: inspect code, edit files, run commands, produce documents. Limiting
the app to its own container would remove the entire point of the product, since
the app would be unable to touch the very files the user is asking about. This is
the same category of tool as a terminal emulator or an IDE, both of which operate
on arbitrary user-chosen files.

  * com.apple.security.files.all
      The agent must read and write files wherever the user can, because the
      target of any given task is chosen by the user at runtime, not at build
      time.

  * com.apple.security.temporary-exception.files.home-relative-path.read-write
    (value: "/")
      Used together with the above so that path resolution anywhere under the
      user's home directory -- the normal case for developer project trees --
      never falls through to a denied branch.

HOW THE USER STAYS IN CONTROL

  * The app never scans, indexes or uploads the user's files on its own. It only
    touches files as part of a task the user explicitly requested in that
    session.
  * The user configures their own model endpoint and API key. Prompts go
    directly from the user's Mac to the endpoint the user chose. The developer
    operates no servers and receives no user content.
  * No analytics, no tracking, no advertising SDKs. PrivacyInfo.xcprivacy
    declares NSPrivacyTracking = false and no collected data types.
  * The app does not download or execute code that changes its own
    functionality, and it does not self-update. Updates ship exclusively through
    the App Store.
  * Command execution happens locally, under the user's own account, only as
    directed by the user in that session.

SANDBOX POSTURE

The app is fully sandboxed (com.apple.security.app-sandbox) and runs under a
hardened runtime. Its two embedded helper executables carry only
com.apple.security.inherit and rely entirely on the app's sandbox -- they declare
no rights of their own.

A demo API key is provided in the review notes so you can exercise the agent
end to end.
```

### 6.2 中文备查

co-shell 是一个**本地 AI Agent 应用**。它不是网页壳：内含原生引擎（Go）+ Swift 外壳，完全运行在用户自己的 Mac 上，开发者不运营任何后端服务。

**为什么申请 `files.all` 与文件临时例外**

"按用户指示，在用户自己的项目目录里读写文件"**就是本产品的核心功能**，不是便利性功能。用户打开 App、指向自己的项目、让 agent 干活：查看代码、改文件、跑命令、产出文档。如果只允许 App 访问自己的容器，产品就失去了全部意义——它恰恰碰不到用户正在问的那些文件。这类工具与终端模拟器、IDE 属于同一类别，二者同样操作任意用户指定的文件。

- `com.apple.security.files.all`：agent 必须能在用户能访问的范围内读写文件，因为每个任务的目标是**用户运行时**选定的，而非构建期固定的。
- `com.apple.security.temporary-exception.files.home-relative-path.read-write`（值 `/`）：与上一条配合使用，确保 home 目录下的路径解析（开发者项目树的常见情形）不会落到被拒分支。

**用户如何保持掌控**

- App 不会自行扫描、索引或上传用户文件；只在用户于该会话中明确要求的任务范围内触碰文件。
- 用户自行配置模型端点与 API Key，提示词由用户机器直连用户所选端点。开发者不运营服务器，也收不到任何用户内容。
- 无统计、无追踪、无广告 SDK；`PrivacyInfo.xcprivacy` 声明 `NSPrivacyTracking = false` 且不采集任何数据类型。
- 不下载执行会改变自身功能的代码，不自我更新；更新只通过 App Store。
- 命令执行发生在本地、以用户自身账号权限、且仅在用户该会话的指示下进行。

**沙箱形态**

App 完全沙箱化（`com.apple.security.app-sandbox`）并启用硬化运行时；两个内嵌 helper 仅带 `com.apple.security.inherit`，完全依赖 App 的沙箱，不自行声明任何权限。

审核备注中提供演示 API Key，可完整跑通 agent 流程。

---

## 7. 沙箱排错

排障前先记住一条：**沙箱 App 的"家目录"被重定向到容器**，很多"文件不见了"其实只是位置变了。

| 目录 | 非沙箱位置 | 沙箱下实际位置 |
|---|---|---|
| App 数据（hub 注册表 / board db） | `~/Library/Application Support/co-shell.app` | `~/Library/Containers/com.coshell.app.mas/Data/Library/Application Support/co-shell.app` |
| hub 日志 | `~/Library/Logs/co-shell-app/hub.log` | `~/Library/Containers/com.coshell.app.mas/Data/Library/Logs/co-shell-app/hub.log` |
| agent 默认工作区 | `~/.co-shell/agents/...` | 容器内 `.co-shell/agents/...` |

### 7.1 「日志打不开 / Operation not permitted」

**不是故障**。容器是 App 的私有空间，其它进程（包括终端）读不了——这是沙箱的正常语义。

**正确做法**：用 App 菜单栏 → **打开日志目录**（本版新增），Finder 会由 App 自己打开容器日志目录。

### 7.2 「App 启动就崩」

按顺序查：

```bash
# 1) 内嵌二进制是否带了 app-sandbox 但没带 inherit（这是必死的组合）
codesign -d --entitlements - work/mas/co-shell.app/Contents/Resources/bin/co-shell

# 2) 子进程是否被“单独启动”过
#    内嵌 helper 绝不能脱离沙箱父进程直接运行
```

崩溃报告里出现 `Unable to get bundle identifier for container id ...` 就是这个问题，原因与处理见设计文档 §2.2（spike v1 反面案例）。

### 7.3 「打包时的 write: Permission denied 与 payload 里的 `._*`」

`productbuild` 在**本地构建**的 bundle 上会出现这个现象：系统托管的 `com.apple.provenance` 扩展属性被 cpio 编码成 AppleDouble（`._*`）条目。

实测结论：`xattr -cr` 与 `ditto --noextattr` **都清不掉**它（这是系统管理的属性），所以脚本不再做无效清理。实践中无害；若上传时校验报错，再考虑在干净的构建机上重新打包。

### 7.4 桌面 / 文稿 / 下载（TCC 保护目录）—— 已实测

**实测结论（2026-10-10）：这三个目录在本权限档下静默可读写，不弹窗、不拒绝。**

证据：沙箱 agent 在 `~/Documents` 与 `~/Downloads` 下完整落盘成功（`config.json` / `db` / `IDENTITY.md`），全程无授权弹窗，且系统 TCC 日志无 coshell 相关条目（TCC 未介入）。详见 `docs/macos-appstore-sandbox-design.md` §6.8。

所以**不需要**在首次启动引导里说明弹窗。反过来也要知道：macOS 上并不存在「首次访问任意文件夹就弹窗确认」的机制——TCC 只覆盖固定位置（桌面/文稿/下载/iCloud/外接宗卷），而沙箱的逐次授权通道是文件选择器（Powerbox）。因此对「访问用户的任意项目目录」这个需求，宽权限档仍是唯一可行路径。

### 7.5 「agent 起不来 / 端口冲突」

hub 端口由 Swift 外壳用 `bind(0)` 动态分配，正常不会冲突。若 agent 起不来，优先看日志（§7.1），其次是 workspace 的数据库锁——**同一个 workspace 不要同时跑两个实例**（bbolt 会 `cannot open database: timeout`）。

### 7.6 一键自查

```bash
APP=work/mas/co-shell.app
/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' $APP/Contents/Info.plist   # com.coshell.app.mas
/usr/libexec/PlistBuddy -c 'Print :ITSAppUsesNonExemptEncryption' $APP/Contents/Info.plist   # false
codesign -d --entitlements - $APP | grep -E "app-sandbox|files.all|network"      # 宽权限档
codesign -d --entitlements - $APP/Contents/Resources/bin/co-shell-hub | grep inherit
codesign --verify --deep --strict --verbose=2 $APP                                # 签名严检
ls $APP/Contents/Resources/PrivacyInfo.xcprivacy                                  # 隐私清单
```

---

## 8. 参考

| 内容 | 位置 |
|---|---|
| **沙箱版设计**（决策 / 进程模型 / entitlements / 容器落位 / 能力矩阵 / 实测证据） | `docs/macos-appstore-sandbox-design.md` |
| 移动端（iOS）上架指导 | `docs/mobile-appstore-release.md` |
| 构建脚本 | `app/build-app.sh`（`CO_SHELL_MAS=1` 沙箱/MAS 通道）、`app/mas-package.sh`（打包上传） |
| App entitlements | `app/macos/app.entitlements` |
| helper entitlements | `app/macos/helper.entitlements` |
| 隐私清单 | `app/macos/PrivacyInfo.xcprivacy` |
| Swift 外壳 | `app/macos/CoShellApp.swift` |
| 任务登记 | `ROADMAP.md` → FIX-630 |
| 用例 | `use-case/FIX-630/` |

Apple 文档：<https://developer.apple.com/documentation/security/app-sandbox>、<https://developer.apple.com/documentation/bundleresources/describing-use-of-required-reason-api>

### 未决项（发布前需确认）

| # | 事项 | 说明 |
|---|---|---|
| ~~A~~ | ~~TCC 对 `~/Desktop`、`~/Documents`、`~/Downloads` 的实际行为~~ **已实测（2026-10-10）：静默允许、不弹窗** | 见 §7.4 |
| B | 老用户从非沙箱版迁移配置/工作区 | 沙箱版首启是空容器，迁移路径待设计 |
| C | 隐私清单的理由代码（reason codes） | 应按 Xcode 生成的隐私报告核对后再提交 |
| D | 证书 / 描述文件 / ASC 记录 | 用户侧硬阻塞，见 §1.1 |
