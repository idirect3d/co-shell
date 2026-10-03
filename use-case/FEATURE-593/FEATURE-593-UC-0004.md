# FEATURE-593-UC-0004 iOS 真机安装与运行验证（TestFlight 内测 + 数据线直装兜底）

## 用例信息

- 分支：FEATURE-593（版本 v0.92.0）
- 目的：验证构建能被分发到真机并正常启动，覆盖 ① TestFlight 内测链路、② 数据线直装（TestFlight 邀请未在设备侧落地时的兜底路径）。

## 前置条件

- 付费 Apple Developer Program 账号（Team `5BZBX3DL83`，云托管 Apple Distribution 签名）
- iPhone 11 Pro（UDID `00008030-000179E434FB802E`）已信任本机
- 可访问的 hub 服务端（https，可自签名）+ 访问 KEY

## 用例步骤与预期

| # | 操作 | 预期结果 | 实测 |
|---|------|----------|------|
| 1 | ASC 存在 App 记录 | App ID `6818714760`、Bundle `com.coshell.mobile` 存在 | ✅ 2026-10-03 |
| 2 | `xcodebuild -exportArchive … destination=upload` | 日志出现 `Upload succeeded` | ✅ |
| 3 | ASC → TestFlight → iOS 构建版本 | 构建 `0.92.0 (1)` 状态"准备提交"（= 可测试），90 天后过期 | ✅ |
| 4 | 内测组「co-shell 内测」包含测试员 | 测试员 `idirect3d@hotmail.com` 状态"已邀请" | ✅ |
| 5 | `xcrun devicectl device install app --device <UDID> <Runner.app>` | 输出 `App installed`、bundleID 正确 | ✅ |
| 6 | `xcrun devicectl device process launch --device <UDID> com.coshell.mobile` | 输出 `Launched application with com.coshell.mobile`（不崩溃） | ✅ |
| 7 | 手机端：设置页填 hub 地址 + 访问 KEY → 保存并连接 | 切到 WebView 并加载 hub 页面；**无需**在网页里再输访问 KEY（Cookie 注入生效）；自签名 https 可访问 | ⬜ 待用户验证 |
| 8 | 手机端：版本号 / 冷启动 / 横竖屏 / iPad 布局 | 设置页底部显示 `v0.92.0`；启动屏→首屏无白闪；横竖屏与 iPad 布局正常 | ⬜ 待用户验证 |

## 实测结果

- 步骤 1~6 **通过**（2026-10-03）：直装后设备侧确认 `co-shell / com.coshell.mobile / 0.92.0 / 1`，且能成功拉起进程。
- 步骤 7~8 待用户在有 hub 服务端的网络环境下于真机确认。
- 备注：TestFlight 邀请在设备侧未落地时（TestFlight 仅显示空状态"兑换"，因其使用 iPhone 的 App Store 账号而非独立登录），可用步骤 5~6 的直装路径兜底；详见 `docs/mobile-appstore-release.md` §9。
