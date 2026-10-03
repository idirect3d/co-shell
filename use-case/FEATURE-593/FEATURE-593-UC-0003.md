# FEATURE-593-UC-0003 版本号同步与 Release 归档构建

## 用例信息

- 分支：FEATURE-593（版本 v0.92.0）
- 目的：验证 App 版本号与主程序/hub 版本号已同步，且 Release 归档可成功产出带正确版本号、含隐私清单的 `.app`。

## 用例步骤与预期

| # | 操作 | 预期结果 |
|---|------|----------|
| 1 | `grep -n "MARKETING_VERSION\|CURRENT_PROJECT_VERSION" mobile/ios/Runner.xcodeproj/project.pbxproj` | Debug/Release 均为 `MARKETING_VERSION = 0.92.0`、`CURRENT_PROJECT_VERSION = 1` |
| 2 | `grep -n "CFBundleShortVersionString" mobile/ios/Runner/SettingsViewController.swift` | 设置页版本号从 Bundle 动态读取，无硬编码 `v0.39.0` |
| 3 | `grep -n '^const version\|^const build' main.go` 与 `grep -n "hubVersion\|hubBuild" cmd/co-shell-hub/main.go` | 均为 `0.92.0` / `1199`，两个入口保持一致 |
| 4 | `go build ./... && go vet ./...` | 无错误、无 vet 告警 |
| 5 | `xcodebuild -project Runner.xcodeproj -scheme Runner -configuration Release -destination 'generic/platform=iOS' -archivePath build/Runner.xcarchive archive CODE_SIGNING_ALLOWED=NO` | `** BUILD SUCCEEDED **` |
| 6 | 归档产物 Info.plist 查版本号 | `CFBundleShortVersionString=0.92.0`、`CFBundleVersion=1` |
| 7 | 归档产物查隐私清单 | `Runner.app/PrivacyInfo.xcprivacy` 存在 |
| 8 | `work/` 下产物 | `co-shell-0.92.0.darwin.arm64`、`co-shell-hub-0.92.0.darwin.arm64` 已生成 |

## 实测结果

- 状态：✅ 通过（2026-10-03，BUILD-1199）
  - `go build ./... && go vet ./...` → 无错误、无告警
  - Release 归档（无签名）`exit=0`（BUILD SUCCEEDED），产物 `CFBundleShortVersionString=0.92.0`、`CFBundleVersion=1`、`CFBundleName=co-shell`
  - `work/co-shell-0.92.0.darwin.arm64`（19.2 MB）、`work/co-shell-hub-0.92.0.darwin.arm64`（13.7 MB）已生成
  - 注：真实分发需带签名重新归档（见 `docs/mobile-appstore-release.md`）
