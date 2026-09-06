# FEATURE-484 移动端功能打通 — 测试用例

> 分支：FEATURE-484 ｜ 版本：v0.38.0
> 目标：收尾 FEATURE-128 移动端功能（修复编译错误、打通 hub 注册联调、补齐任务计划查看、完善配置与体验）

## UC-0001 main.dart 编译错误修复

- **前置**：mobile/ 工程存在
- **步骤**：
  1. 检查 `mobile/lib/main.dart` 第 28 行 import 语句
  2. 确认末尾多余反引号已移除
- **预期**：`import 'package:provider/provider.dart';` 无多余反引号；`flutter analyze` 无语法错误

## UC-0002 mobile 工程可编译

- **前置**：main.dart 编译错误已修复
- **步骤**：在 mobile/ 目录执行 `flutter analyze`（或 `flutter build` 对应平台）
- **预期**：无编译错误，工程可正常构建

## UC-0003 hub 注册联调（握手字段核对）

- **前置**：co-shell-hub-legacy 已启动，已用 `--add-client 我的手机` 注册客户端拿到 access_key
- **步骤**：
  1. 核对 `mobile/lib/utils/udp_client.dart` 握手字段（type/nickname/access_key）
  2. 核对 `hub/` 包实际校验逻辑是否一致
  3. 将 access_key 填入 `mobile/lib/config/constants.dart` 的 hubAccessKey
- **预期**：握手字段与 hub 校验逻辑一致；移动端能成功注册并收到 hub 回包

## UC-0004 移动端收发消息

- **前置**：注册联调成功
- **步骤**：通过移动端向 hub 发送一条消息，观察 hub 转发与回复
- **预期**：消息能到达 hub 并路由到对应 agent，回复能回到移动端

## UC-0005 任务计划查看功能（hub 端）

- **前置**：hub 端已新增查询任务计划的 UDP 消息类型
- **步骤**：移动端发送查询任务计划请求
- **预期**：hub 返回当前 agent 的任务计划数据（标题/步骤/状态）

## UC-0006 任务计划查看功能（mobile 端）

- **前置**：hub 端查询接口可用
- **步骤**：移动端新增界面与 provider 方法，展示任务计划
- **预期**：移动端能展示任务计划标题、步骤列表及完成状态

## UC-0007 服务器地址持久化

- **前置**：shared_preferences 已接入
- **步骤**：在连接设置中修改服务器地址，重启应用
- **预期**：服务器地址被持久化，重启后仍保留用户配置

## UC-0008 语音/图片发送协议核对

- **前置**：chat_screen 的 sendMessage 支持 images 字段
- **步骤**：核对 sendMessage 传参（content/images）与 hub 端消费逻辑
- **预期**：images 字段被 hub 正确消费，语音/图片发送协议一致

## UC-0009 端到端回归

- **前置**：以上功能均完成
- **步骤**：完整走一遍 连接设置 → 注册 → 选择 agent → 发送文本/图片 → 查看任务计划
- **预期**：全流程可用，无编译错误，无协议不一致
