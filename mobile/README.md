# mobile/ — co-shell 移动端（原生 iOS）

本目录为 co-shell 移动端客户端，采用**纯原生 iOS**（Swift + UIKit + WKWebView）实现，通过浏览器控件渲染 co-shell hub 的 web UI。

> 旧版 Flutter UDP 客户端完整保留在 `mobile-legacy/`，本目录不再使用 Flutter。

## 功能

- **系统设置页**（原生 UI）：输入 hub 服务端地址（如 `https://192.168.3.19:23311`）与访问 KEY，存于 iOS Keychain。
- **WKWebView 渲染 hub 页面**：加载前自动注入 `access_key` Cookie，用户无需在网页中重复输入访问 KEY。
- **自签名证书信任**：ATS 例外 + WKWebView 证书校验放行，支持 hub 自签名 https。
- **导航**：未配置 → 显示设置页；已配置 → 显示 hub WebView；随时可从 WebView 右上角"设置"返回配置页。

## 目录结构

```
mobile/ios/
  Runner.xcodeproj/        # Xcode 工程（纯 Swift 单 target，无 CocoaPods/Flutter）
  Runner/
    AppDelegate.swift      # 应用入口（纯 UIKit）
    SceneDelegate.swift    # 场景入口，挂 RootViewController
    RootViewController.swift   # 导航控制器：按配置状态选设置页或 WebView
    SettingsStore.swift        # Keychain 存储（服务端地址 + 访问 KEY）
    SettingsViewController.swift # 原生系统设置页
    WebViewController.swift    # WKWebView 壳（注入 Cookie、放行自签名证书）
    Info.plist             # 含 ATS 例外与本地网络权限
    Assets.xcassets/       # App 图标
    Base.lproj/LaunchScreen.storyboard
```

## 构建

```bash
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
cd mobile/ios
xcodebuild -project Runner.xcodeproj -scheme Runner \
  -destination 'platform=iOS Simulator,name=iPhone 17 Pro' build
```

真机运行需在 Xcode 中配置签名（`DEVELOPMENT_TEAM`）。
