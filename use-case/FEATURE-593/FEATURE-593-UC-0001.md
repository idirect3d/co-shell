# FEATURE-593-UC-0001 隐私清单（PrivacyInfo.xcprivacy）存在、合规且被工程打包

## 用例信息

- 分支：FEATURE-593（版本 v0.92.0）
- 目的：验证 iOS 17+ 上架必需的隐私清单文件存在、内容合规，且确实被 Xcode 打包进 `.app`（否则上传后报 `ITMS-91053 Missing API declaration`）。

## 前置条件

- 本机有 Xcode（本机为 Xcode 27.0 / iOS SDK 27.0）

## 用例步骤与预期

| # | 操作 | 预期结果 |
|---|------|----------|
| 1 | `cat mobile/ios/Runner/PrivacyInfo.xcprivacy` | 含 `NSPrivacyTracking=false`、`NSPrivacyCollectedDataTypes` 为空数组、`NSPrivacyAccessedAPITypes` 含 `NSPrivacyAccessedAPICategoryUserDefaults` 且理由为 `CA92.1` |
| 2 | `plutil -lint mobile/ios/Runner/PrivacyInfo.xcprivacy` | 输出 `OK`（plist 语法合法） |
| 3 | `grep -c PrivacyInfo mobile/ios/Runner.xcodeproj/project.pbxproj` | ≥ 3（PBXBuildFile + PBXFileReference + Resources 构建阶段 + group 子项） |
| 4 | 执行 Release 归档后 `ls <archive>/Products/Applications/Runner.app/PrivacyInfo.xcprivacy` | 文件存在（已随资源打包） |

## 实测结果

- 状态：✅ 通过（2026-10-03，BUILD-1199）
  - `plutil -lint mobile/ios/Runner/PrivacyInfo.xcprivacy` → `OK`
  - `grep -c PrivacyInfo mobile/ios/Runner.xcodeproj/project.pbxproj` → `4`（PBXBuildFile / PBXFileReference / group 子项 / Resources 阶段）
  - 归档后 `ls build/Runner.xcarchive/Products/Applications/Runner.app/PrivacyInfo.xcprivacy` → 存在（292 字节）
