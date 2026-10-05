# co-shell 移动端（iOS）发布指导

> 适用范围：`mobile/ios` 原生 iOS 工程（Bundle ID `com.coshell.mobile`，Team `5BZBX3DL83`）
> 本次目标：先走 **TestFlight 内测**，真机验证通过后再考虑公开审核
> 关联任务：FEATURE-593 / v0.92.0 / BUILD-1199

---

## 0. 前置条件（缺一不可）

| 项 | 说明 | 怎么确认 |
|---|---|---|
| Apple Developer Program（付费） | 免费账号**不能**上传 TestFlight / 上架 | <https://developer.apple.com/account> → Membership details：显示 *Apple Developer Program* + 续费日 |
| App Store Connect 有该 App 记录 | Bundle ID 必须与工程一致 | <https://appstoreconnect.apple.com> → 我的 App → 新建 App（Bundle ID 选 `com.coshell.mobile`；名称 "co-shell" 需全局唯一，被占用就换） |
| Xcode 已登录 Apple ID | 自动签名要用 | Xcode → Settings → Accounts（团队选 `5BZBX3DL83`） |
| 分发证书 | 首次分发归档时由 Xcode 自动创建 | 本机当前只有 `Apple Development`，属正常 |

> ⚠️ 钥匙串安全：私钥只在登录钥匙串里（`login.keychain-db`）。**任何情况下不要把导出的 `.p12` 外传、入 git、放第三方 CI 明文**，上架时生成的 Apple Distribution 证书尤其敏感。

---

## 0.5 本机实测：账号已具备分发资格（2026-10-03 实机验证）

- Xcode 里只登录了 **1 个 Apple ID**，它只绑定了一个团队：
  - `teamName = "Shuang Liang (Personal Team)"`、`teamType = "Personal Team"`、`isFreeProvisioningTeam = 1`、`teamID = 5BZBX3DL83`
  - 即 **免费个人团队（Personal Team）**
- 实测结论（用本机 Xcode 27 自动签名跑通全流程）：
  - ✅ 带签名归档成功（`ARCHIVE SUCCEEDED`）
  - ✅ App Store 分发包导出成功：`mobile/ios/build/ipa/co-shell.ipa`（755 KB），签名 `Apple Distribution: Shuang Liang (5BZBX3DL83)`（Cloud Managed Apple Distribution，2027-10-03 到期），描述文件 `iOS Team Store Provisioning Profile: com.coshell.mobile`（`beta-reports-active = true`）
  - ⚠️ 唯一阻塞：**App Store Connect 尚无该 App 记录**，上传报错：
    `App record with bundle identifier "com.coshell.mobile" not found on App Store Connect. Create an app record on App Store Connect, or distribute the app from Xcode, and then try again.`
- 说明：Xcode 偏好里仍显示 `Personal Team / isFreeProvisioningTeam = 1`，属**免费时期的旧缓存**（免费团队无法导出带 `beta-reports-active` 的 Store 包）；想刷新显示可在 Xcode → Settings → Accounts 移除并重新添加该 Apple ID。
### 0.5.1 解除阻塞：创建 App 记录（用户操作，约 3 分钟）

<https://appstoreconnect.apple.com> → 我的 App → **+** → **新建 App**

| 字段 | 填什么 |
|---|---|
| 平台 | iOS |
| 名称 | `co-shell`（App Store 名称全局唯一，被占用就换，如 `co-shell – 自建 AI 终端`） |
| 主要语言 | 简体中文 |
| Bundle ID | 选 `com.coshell.mobile`（下拉里已存在，由 Xcode 自动创建） |
| SKU | 任意唯一串，如 `coshell-ios-001` |
| 用户访问权限 | 完全访问 |

创建后即可重新上传（命令见 §2 方式 B；或 Xcode Organizer → Distribute App → 会提示创建记录）。

### 0.5.2 内部测试员配置（无需 App Review）

1. **Users and Access** → **+** 添加测试员 Apple ID（角色如 Developer / Customer Support 均可）
2. 我的 App → **TestFlight** → **Internal Testing** → 建组 → 添加测试员 → 分配构建
3. 测试员手机装 **TestFlight** App → 用该 Apple ID 登录 → 接受邀请 → 安装 co-shell
- 未付费期间的临时替代（能马上在手机上用，但不属于 TestFlight 发布）：
  - **免费团队真机调试**：Xcode 打开 `mobile/ios/Runner.xcodeproj` → 顶栏选真机（本机可见 `lsip11p` = iPhone 11 Pro / iOS 27.0.1，当前 **offline**，需数据线连接）→ Run；App 能装上手机，**7 天后需重新 Run**。
  - **模拟器**：本机报 `CoreSimulator is out of date`，需先 `sudo xcodebuild -runFirstLaunch`（或重装命令行工具）才能使用。

---

## 1. 归档（Archive）

```bash
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
cd mobile/ios

xcodebuild -project Runner.xcodeproj -scheme Runner \
  -configuration Release \
  -destination 'generic/platform=iOS' \
  -archivePath build/Runner.xcarchive \
  -allowProvisioningUpdates \
  archive
```

- `-allowProvisioningUpdates` 让 Xcode 自动创建 Apple Distribution 证书与描述文件（首次会联网申请）。
- 成功标志：`** ARCHIVE SUCCEEDED **`；产物 `build/Runner.xcarchive`。
- 若报签名/账号类错误：先在 Xcode 里打开 `Runner.xcodeproj`，Target → Signing & Capabilities 勾选 *Automatically manage signing*，手动跑一次归档，Xcode 会把账号问题讲得比命令行清楚。

## 2. 上传到 App Store Connect

### 方式 A（推荐、最省事）：Xcode 图形界面
Xcode → **Window → Organizer** → 选中刚生成的归档 → **Distribute App** → **App Store Connect** → **Upload** → 一路 Next。

### 方式 B：命令行导出并上传

```bash
cd mobile/ios
cat > build/ExportOptions.plist <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>method</key>
	<string>app-store-connect</string>
	<key>destination</key>
	<string>upload</string>
	<key>teamID</key>
	<string>5BZBX3DL83</string>
	<key>uploadSymbols</key>
	<true/>
	<key>manageAppVersionAndBuildNumber</key>
	<false/>
</dict>
</plist>
PLIST

xcodebuild -exportArchive \
  -archivePath build/Runner.xcarchive \
  -exportOptionsPlist build/ExportOptions.plist \
  -allowProvisioningUpdates
```

- 老版本 Xcode 若报 `method` 非法，把 `app-store-connect` 换成 `app-store`。
- 上传成功后 App Store Connect → 我的 App → **TestFlight** 会出现该构建，**处理中约 5~60 分钟**（会跑自动合规检查）。

## 3. 版本号纪律（每次上传前必做）

- `MARKETING_VERSION`（对外版本号，如 `0.92.0`）在 `mobile/ios/Runner.xcodeproj/project.pbxproj` 的 Debug/Release 两处。
- `CURRENT_PROJECT_VERSION`（构建号）**同一版本号下每次上传必须递增**（1 → 2 → 3 …），否则 ASC 拒收重复构建号。
- 本次交付：`0.92.0 / 1`，与主程序 `0.92.0 / BUILD-1199` 对齐；设置页版本号已改为从 Bundle 动态读取，无需再手改界面文字。

---

## 4. TestFlight 内测（本次目标）

1. 上传完成后：App Store Connect → 我的 App → **TestFlight**，等待构建从"处理中"变为"准备好提交"。
2. 填 **Test Information**（测试信息）：Beta App Description、反馈邮箱、（外部测试需）测试说明。
3. **内部测试**（最快，无需审核）：Users and Access 里添加测试员 Apple ID（最多 100 人）→ TestFlight → Internal Testing → 把构建分配给该组 → 测试员在 iPhone 上装 TestFlight 并接受邀请。
4. **外部测试**：需先过 **Beta App Review**（首个构建必须审核，通常 1~2 天，后续构建通常免审），最多 10,000 人；构建有效期 90 天。
5. 构建合规提示：因为 Info.plist 已声明 `ITSAppUsesNonExemptEncryption=false`，上传后**不会再被要求回答加密问题**。

## 5. 真机验证清单（拿到 TestFlight 构建后逐项确认）

| # | 验证项 | 期望 |
|---|--------|------|
| 1 | 首次启动 | 进入设置页，无崩溃 |
| 2 | 填 hub 地址 + 访问 KEY → 保存并连接 | 切到 WebView 并加载 hub 页面 |
| 3 | 页面加载 | **无需**在网页里再输入访问 KEY（Cookie 注入生效） |
| 4 | 自签名 https hub | 能正常加载（证书放行生效） |
| 5 | 交互 | 发消息、切换会话、查看任务进展均正常 |
| 6 | 横竖屏 / iPad 全屏与分屏 | 布局正常，无控件被遮挡 |
| 7 | 设置页底部版本号 | 显示 `v0.92.0`（与构建一致） |
| 8 | 冷启动 | 启动屏 → 首屏**无白闪** |
| 9 | 修改地址后返回 | 重新加载成新地址 |

> 其中 3、4、5、6、8 只有真机才能确认；本机模拟器组件当前过期（`CoreSimulator is out of date`），如要在本机模拟器验证，先跑 `sudo xcodebuild -runFirstLaunch` 或重装 Xcode 命令行工具。

## 6. 公开审核前还需要补（本次不做，先记着）

- **截图**：iPhone 6.9"/6.7" 至少 3 张；工程 `TARGETED_DEVICE_FAMILY = "1,2"`（支持 iPad）→ **必须补 iPad 13" 截图**；如不想提供，可把工程改为仅 iPhone 后重新归档。
- **隐私政策 URL + 支持 URL**（ASC 必填；可用 GitHub Pages 托管静态页）。
- **App 隐私问卷**：选"不收集数据"（与 `PrivacyInfo.xcprivacy` 中 `NSPrivacyCollectedDataTypes=[]`、`NSPrivacyTracking=false` 保持一致）。
- **审核备注（App Review Information）**：说明"这是私有服务 co-shell 的原生客户端，需自建 hub 服务端"，并给出**演示 hub 地址 + 访问 KEY + 操作步骤**。
- **年龄分级、分类（建议 Developer Tools 或 Utilities）、关键词、描述、价格**。
- 出口合规无需再答（已由 Info.plist 声明）。

## 7. 已知风险与应对

| 风险 | 触发点 | 应对 |
|------|--------|------|
| **Guideline 4.2**（Minimum Functionality） | App 主体是 WKWebView 壳 | 审核备注写清产品定位（自建服务客户端）+ 提供可用演示环境；必要时补充原生功能说明 |
| **Guideline 2.1**（无法测试） | 审核员访问不到你的 hub | 提供公网可访问的 demo hub + 访问 KEY，并写明操作步骤 |
| ATS 相关问询 | 加载 http / 自签名证书 | 已把全局放开收窄为 `NSAllowsArbitraryLoadsInWebContent`；备注说明为何需要 |
| 构建号重复被拒 | 同版本号重复上传 | 每次上传前 `CURRENT_PROJECT_VERSION` +1 |
| `ITMS-91053` | 缺隐私清单 | 已补 `PrivacyInfo.xcprivacy` |
| 签名失败 | 无分发证书 / 账号未登录 | Xcode 里打开工程跑一次自动签名归档；确认已加入付费开发者计划 |

## 8. 本次已完成的工程侧准备（FEATURE-593 / v0.92.0 / BUILD-1199）

| 项 | 变更 |
|---|---|
| 隐私清单 | 新增 `mobile/ios/Runner/PrivacyInfo.xcprivacy`（不追踪 / 不收集数据 / UserDefaults 理由 `CA92.1`），并注册进工程 Resources |
| 出口合规 | `Info.plist` 新增 `ITSAppUsesNonExemptEncryption = false` |
| ATS | 全局 `NSAllowsArbitraryLoads` → `NSAllowsArbitraryLoadsInWebContent`（保留 `NSAllowsLocalNetworking`） |
| 应用名 | `CFBundleName`：`Runner` → `co-shell` |
| 版本号 | App `MARKETING_VERSION` 0.39.0 → **0.92.0**（Debug/Release），`CURRENT_PROJECT_VERSION = 1` |
| 设置页 | 版本号由硬编码改为从 Bundle 动态读取 |
| 主程序 | `main.go`、`cmd/co-shell-hub/main.go` 同步为 `0.92.0` / BUILD `1199` |

实测：`go build ./... && go vet ./...` 通过；Release 归档（无签名）`exit=0`；产物含 `PrivacyInfo.xcprivacy`，`CFBundleName=co-shell`、版本 `0.92.0/1`。

---

## 9. 常见问题：TestFlight 里看不到 co-shell，只有一个"兑换"按钮

### 现象
TestFlight App 打开后只有「兑换」按钮，找不到头像/退出登录按钮，也没有 co-shell。

### 关键认知（不是故障）
1. **TestFlight 没有独立的登录界面**，它固定使用 iPhone 的 **App Store 账号**（设置 → 你的名字 → 媒体与购买项目）。所以在 TestFlight 里看不到头像、也找不到"退出登录"是**正常现象**。
2. 「兑换」按钮是"当前账号下没有可测试 App"的**空状态**；**内部测试不需要兑换码**（兑换码只用于外部测试的公开链接）。
3. 服务端（ASC）状态正常时，问题一定在设备账号侧。

### 先确认服务端状态（正常应如下）
- App Store Connect → TestFlight → 群组：`内部群组 · N 个测试员 · 1 个构建版本`
- 测试员页：你的 Apple ID 状态 = **已邀请 + 日期**
- 构建行：`版本 x.y.z / 构建版本 N / 准备提交 / 90 天后过期`，群组列显示你的内测组

### 解决路径（按成功率排序）

**路径 A：把 iPhone 的 App Store 账号切换成被邀请的 Apple ID**
1. iPhone「设置」→ 顶部**你的名字** → **媒体与购买项目** → 查看当前登录的 Apple ID；
2. 若不是被邀请的邮箱 → 「退出登录」→ 用被邀请的 Apple ID 登录（⚠️ 只影响 App Store，**不影响 iCloud 数据**）；
3. 上滑杀掉 TestFlight → 重开 → **下拉刷新**。

**路径 B：接受邀请邮件**
在被邀请邮箱（**含垃圾邮件**）里找 Apple 的 "You're invited to test …" 邮件 → 点 **View in TestFlight**。

**路径 C：跳过 TestFlight，数据线直装（付费账号推荐备选）**
付费开发者账号可用开发签名直接安装到自己的设备，**有效期 1 年**（免费账号只有 7 天）：
```bash
# 1) 连上 iPhone 并在手机上"信任此电脑"，确认设备可见
xcrun devicectl list devices
# 2) 构建（自动签名）
cd mobile/ios
xcodebuild -project Runner.xcodeproj -scheme Runner -configuration Debug \
  -destination 'id=<DEVICE_UDID>' -allowProvisioningUpdates build
# 3) 安装到设备
xcrun devicectl device install app --device <DEVICE_UDID> <path-to-Runner.app>
```
> 也可以用 Xcode 打开 `Runner.xcodeproj`，选中真机后直接按 Run。

### 进阶排查
- 邮箱被拦截 / 长时间收不到邀请 → 在 ASC 里**删掉该测试员再重新添加**（重新发邀请邮件）。
- 构建未真正分配 → 群组页 → 构建版本 → **添加构建版本**，勾选目标构建并添加（添加后会通知测试员）。
- 构建 90 天过期 → 递增 `CURRENT_PROJECT_VERSION` 后重新归档上传。

---

## 10. 内测分发进展记录（2026-10-03）

| 项目 | 状态 |
|---|---|
| App Store Connect App | **App ID `6818714760`**，名称 `co-shell`，Bundle ID `com.coshell.mobile`，SKU `coshell-ios-001`，主要语言简体中文 |
| 构建 | **`0.92.0 (1)` 上传成功**，状态"准备提交"（= 可测试），90 天后过期 |
| 内测组 | 「**co-shell 内测**」（勾选"可访问所有构建"），1 个测试员、1 个构建版本 |
| 测试员 | `idirect3d@hotmail.com`（梁爽 · 内部），状态 **已邀请 2026-10-03** |
| 分发签名 | `Apple Distribution: Shuang Liang (5BZBX3DL83)`（Cloud Managed，2027-10-03 到期），描述文件 `iOS Team Store Provisioning Profile: com.coshell.mobile`（`beta-reports-active=true`） |
| **真机直装（已完成）** | 用 `devicectl` 将 Debug 版安装到 **iPhone 11 Pro**（UDID `00008030-000179E434FB802E`），签名 `Apple Development: Shuang Liang`（**1 年有效**）；命令见 §9 路径 C |

### 待办
1. **手机端验证**：打开桌面上的 `co-shell`（若提示"不受信任的开发者"→ 设置 → 通用 → VPN 与设备管理 → 信任；若提示需要开发者模式 → 设置 → 隐私与安全性 → 开发者模式），然后按 §5 的 7 项清单验证（Cookie 注入 / 自签名证书 / 横竖屏 / iPad / 版本号 v0.92.0 / 冷启动无白闪）。
2. **TestFlight 邀请落地**：把 iPhone 的「媒体与购买项目」账号切到 `idirect3d@hotmail.com`，或点邀请邮件里的 View in TestFlight；落地后即可分发给小团队其他成员。
3. **构建到期**：TestFlight 构建 90 天后过期，到期前把 `CURRENT_PROJECT_VERSION` 递增（1 → 2）后重新归档上传。
4. **工程合并**：FEATURE-593 的改动仍在分支 `FEATURE-593`（提交 `2700f836`），待验收后合并 main 并打 `v0.92.0` 标签。

---

## 11. FIX-608 构建上传记录（2026-10-05）

| 项目 | 状态 |
|---|---|
| 版本号 | **`0.96.2 (2)`** —— `MARKETING_VERSION` 由 0.92.0 对齐到 co-shell 主程序版本 0.96.2，`CURRENT_PROJECT_VERSION` 1→2（同 App 递归上传需要递增构建号） |
| 归档 | `mobile/ios/build/FIX-608.xcarchive` → `** ARCHIVE SUCCEEDED **` |
| 上传 | `xcodebuild -exportArchive … -exportOptionsPlist build/ExportOptions-upload.plist` → `Progress 100%: Upload succeeded`、`** EXPORT SUCCEEDED **`（2026-10-05 09:19） |
| 内容 | FIX-608：iOS `WebViewController` 实现 WKDownload（附件响应 → 下载落盘到 `Documents/download/` + 完成后分享面板），修复移动端下载工作区文件时主工作区白屏/无响应；前端 `downloadFile` 改为隐藏 iframe 触发、不再整页导航 |
| 后续 | ① ASC 处理约 5~60 分钟；② 把该构建分配给内测组「co-shell 内测」；③ 手机 TestFlight 更新（若邀请仍未在设备侧落地，用 §9 路径 C 的 `devicectl` 直装兜底）；④ 按 `use-case/FIX-608/FIX-608-UC-0001.md` 的 UC-11~15 做真机验收 |
