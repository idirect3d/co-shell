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

import 'dart:async';
import 'dart:convert';
import 'dart:io';

/// UDP 通信客户端
///
/// 使用 UDP 协议与 co-shell-bridge 通信
/// 首次连接时发送握手请求进行身份验证
class UdpClient {
  RawDatagramSocket? _datagramSocket;
  InternetAddress? _serverAddress;
  int? _serverPort;
  bool _isConnected = false;

  // 回调
  Function(String)? onMessageReceived;

  /// 连接到服务器
  Future<bool> connect(String address, int port) async {
    try {
      _serverAddress = InternetAddress(address);
      _serverPort = port;

      // 创建 UDP socket
      _datagramSocket = await RawDatagramSocket.bind(
        InternetAddress.anyIPv4,
        0, // 系统自动分配端口
      );

      _isConnected = true;

      // 开始监听 incoming 消息
      _startListening();

      // 发送首次握手请求
      final success = await _sendHandshake();

      if (!success) {
        _isConnected = false;
      }

      return success;
    } catch (e) {
      _isConnected = false;
      return false;
    }
  }

  /// 发送首次握手请求
  Future<bool> _sendHandshake() async {
    try {
      final handshakeData = {
        'type': 'handshake',
        'timestamp': DateTime.now().millisecondsSinceEpoch,
      };

      final jsonData = jsonEncode(handshakeData);
      final bytes = utf8.encode(jsonData);

      _datagramSocket?.send(bytes, _serverAddress!, _serverPort!);

      // 等待握手响应（超时 5 秒）
      final start = DateTime.now();
      while (DateTime.now().difference(start).inSeconds < 5) {
        final packet = _datagramSocket?.receive();
        if (packet != null) {
          final data = utf8.decode(packet.data);
          final jsonData = jsonDecode(data) as Map<String, dynamic>;
          if (jsonData['type'] == 'handshake_ack') {
            return true;
          }
        }
        await Future.delayed(const Duration(milliseconds: 100));
      }

      return false;
    } catch (e) {
      return false;
    }
  }

  /// 开始监听 incoming 消息
  void _startListening() {
    _datagramSocket?.listen((RawSocketEvent event) {
      if (event == RawSocketEvent.read) {
        try {
          final packet = _datagramSocket?.receive();
          if (packet != null) {
            final data = utf8.decode(packet.data);
            final jsonData = jsonDecode(data) as Map<String, dynamic>;

            if (jsonData['type'] == 'message') {
              final message = jsonData['content'] as String;
              onMessageReceived?.call(message);
            }
          }
        } catch (e) {
          // 忽略解析错误
        }
      }
    });
  }

  /// 发送消息
  Future<bool> send(String text, {List<String>? imagePaths}) async {
    if (!_isConnected || _datagramSocket == null) {
      return false;
    }

    try {
      final messageData = {
        'type': 'message',
        'content': text,
        'timestamp': DateTime.now().millisecondsSinceEpoch,
        if (imagePaths != null) 'images': imagePaths,
      };

      final jsonData = jsonEncode(messageData);
      final bytes = utf8.encode(jsonData);

      _datagramSocket?.send(bytes, _serverAddress!, _serverPort!);
      return true;
    } catch (e) {
      return false;
    }
  }

  /// 断开连接
  void disconnect() {
    _isConnected = false;
    _datagramSocket?.close();
    _datagramSocket = null;
  }

  /// 释放资源
  void dispose() {
    disconnect();
  }
}