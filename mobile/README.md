# co-der - co-shell 移动端 APP

跨平台移动客户端，支持 iOS 和 Android，通过 UDP 与 co-shell-hub 通信。

## 功能特性

- 实时聊天界面
- 语音输入（speech_to_text 插件）
- 图片选择与发送（image_picker 插件）
- UDP 通信 + 首次握手密钥验证
- 多平台支持（iOS/Android）

## 项目结构

```
mobile/
├── lib/
│   ├── main.dart                    # 应用入口
│   ├── models/
│   │   └── message.dart             # 消息模型
│   ├── providers/
│   │   └── chat_provider.dart       # 聊天状态管理
│   ├── screens/
│   │   └── chat_screen.dart         # 聊天主屏幕
│   └── utils/
│       └── udp_client.dart          # UDP 通信客户端
├── android/                         # Android 平台文件
├── ios/                             # iOS 平台文件
├── pubspec.yaml                     # 依赖配置
└── README.md                        # 本文档
```

## 环境要求

- Flutter SDK >= 3.1.0
- Dart SDK >= 3.1.0
- Xcode（iOS 开发）
- Android Studio / SDK（Android 开发）

## 安装依赖

```bash
cd mobile
flutter pub get
```

## 运行项目

### Android

```bash
flutter run
```

### iOS

```bash
flutter run -d ios
```

## 构建发布版本

### Android APK

```bash
flutter build apk --release
```

### Android App Bundle（用于 Google Play）

```bash
flutter build appbundle --release
```

### iOS

```bash
flutter build ios --release
```

## 通信协议

### UDP 通信

- 客户端启动时创建 UDP socket
- 首次连接发送 handshake 请求
- 服务器验证后返回 handshake_ack
- 后续消息通过 JSON 格式传输

### 消息格式

```json
// 客户端 -> 服务器
{
  "type": "message",
  "content": "用户输入文本",
  "timestamp": 1234567890,
  "images": ["/path/to/image1.jpg"]
}

// 服务器 -> 客户端
{
  "type": "message",
  "content": "LLM 返回文本",
  "timestamp": 1234567890
}
```

## 权限配置

### Android (`android/app/src/main/AndroidManifest.xml`)

```xml
<uses-permission android:name="android.permission.INTERNET"/>
<uses-permission android:name="android.permission.RECORD_AUDIO"/>
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE"/>
<uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE"/>
```

### iOS (`ios/Runner/Info.plist`)

```xml
<key>NSMicrophoneUsageDescription</key>
<string>需要麦克风权限以进行语音输入</string>
<key>NSPhotoLibraryUsageDescription</key>
<string>需要相册权限以选择图片</string>
```

## 许可证

MIT License - 参见项目根目录 LICENSE 文件

## 作者

L.Shuang