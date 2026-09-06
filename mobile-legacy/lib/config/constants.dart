// Author: L.Shuang
// Created: 2026-05-17
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

/// co-shell 应用常量定义
class Constants {
  Constants._();

  /// co-shell-hub 默认 UDP 端口
  /// 128 = 2^7，是计算中的基础数字
  static const int hubPort = 12800;

  /// 默认服务器地址（用于演示）
  static const String defaultServerAddress = '192.168.1.100';

  /// UDP 请求超时时间（秒）
  static const int udpRequestTimeout = 30;

  /// 握手超时时间（秒）
  static const int handshakeTimeout = 5;

  /// 应用名称
  static const String appName = 'co-der';

  /// 应用版本
  static const String appVersion = '0.1.0';

  /// 移动端昵称（在 Hub 中注册时使用的名称）
  /// 修改为你在 hub 上注册的昵称
  static const String clientNickname = '我的手机';

  /// Hub 访问密钥（从 hub 注册时获取）
  /// 使用 --add-client 注册后得到的 public_key
  static const String hubAccessKey = '';
}
