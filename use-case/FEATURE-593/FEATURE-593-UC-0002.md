# FEATURE-593-UC-0002 Info.plist 上架项：出口合规声明 / ATS 收窄 / CFBundleName

## 用例信息

- 分支：FEATURE-593（版本 v0.92.0）
- 目的：验证 Info.plist 补齐上架必需项（出口合规）、ATS 由全局放开收窄为仅 WebView 内容、CFBundleName 由 Runner 改为 co-shell，并在归档产物中生效。

## 用例步骤与预期

| # | 操作 | 预期结果 |
|---|------|----------|
| 1 | `plutil -lint mobile/ios/Runner/Info.plist` | 输出 `OK` |
| 2 | `plutil -p mobile/ios/Runner/Info.plist` 查 `ITSAppUsesNonExemptEncryption` | 值为 `0`（false，声明仅使用豁免加密） |
| 3 | 同上看 `NSAppTransportSecurity` | 含 `NSAllowsArbitraryLoadsInWebContent=1`、`NSAllowsLocalNetworking=1`，且**不含**全局 `NSAllowsArbitraryLoads` |
| 4 | 归档产物 `plutil -p <archive>/Products/Applications/Runner.app/Info.plist` | `CFBundleName=co-shell`、`ITSAppUsesNonExemptEncryption=0`、`NSAllowsArbitraryLoadsInWebContent=1` |
| 5 | 真机（TestFlight）配置 hub 地址并加载 | 页面正常加载，Cookie 注入与自签名证书放行不受 ATS 收窄影响 |

## 实测结果

- 状态：✅ 步骤 1~4 通过，步骤 5（真机加载）待 TestFlight 验证（2026-10-03，BUILD-1199）
  - `plutil -lint mobile/ios/Runner/Info.plist` → `OK`
  - 归档产物：`CFBundleName = "co-shell"`、`ITSAppUsesNonExemptEncryption = false`、`NSAllowsArbitraryLoadsInWebContent = true`、`NSAllowsLocalNetworking = true`（已无全局 `NSAllowsArbitraryLoads`）
