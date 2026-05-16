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

import 'package:flutter/foundation.dart';
import '../utils/udp_client.dart';
import '../models/message.dart';

/// 聊天状态管理 Provider
class ChatProvider extends ChangeNotifier {
  final UdpClient _udpClient = UdpClient();
  final List<Message> _messages = [];
  bool _isConnected = false;
  bool _isSending = false;
  String? _serverAddress;
  int? _serverPort;

  List<Message> get messages => _messages;
  bool get isConnected => _isConnected;
  bool get isSending => _isSending;
  String? get serverAddress => _serverAddress;
  int? get serverPort => _serverPort;

  /// 连接到 co-shell-bridge 服务器
  Future<bool> connect(String address, int port) async {
    try {
      _serverAddress = address;
      _serverPort = port;
      
      final success = await _udpClient.connect(address, port);
      _isConnected = success;
      notifyListeners();
      
      if (success) {
        _addSystemMessage('已连接到 $address:$port');
      }
      
      return success;
    } catch (e) {
      _addSystemMessage('连接失败: $e');
      _isConnected = false;
      notifyListeners();
      return false;
    }
  }

  /// 断开连接
  void disconnect() {
    _udpClient.disconnect();
    _isConnected = false;
    _addSystemMessage('已断开连接');
    notifyListeners();
  }

  /// 发送消息
  Future<void> sendMessage(String text, {List<String>? imagePaths}) async {
    if (!_isConnected || _isSending) return;

    _isSending = true;
    notifyListeners();

    try {
      // 添加用户消息到列表
      final userMessage = Message(
        id: DateTime.now().millisecondsSinceEpoch.toString(),
        text: text,
        isUser: true,
        imagePaths: imagePaths,
        timestamp: DateTime.now(),
      );
      _messages.add(userMessage);
      notifyListeners();

      // 通过 UDP 发送
      final success = await _udpClient.send(text, imagePaths: imagePaths);
      
      if (!success) {
        _addSystemMessage('发送失败，请检查连接状态');
      }

      // 等待服务器响应（通过 UDP 监听器自动添加到消息列表）
      // 这里设置一个超时，防止无限等待
    } finally {
      _isSending = false;
      notifyListeners();
    }
  }

  /// 添加系统消息
  void _addSystemMessage(String text) {
    final systemMessage = Message(
      id: DateTime.now().millisecondsSinceEpoch.toString(),
      text: text,
      isUser: false,
      isSystem: true,
      timestamp: DateTime.now(),
    );
    _messages.add(systemMessage);
    notifyListeners();
  }

  /// 添加收到的消息（由 UDP 监听器调用）
  void addReceivedMessage(String text) {
    final assistantMessage = Message(
      id: DateTime.now().millisecondsSinceEpoch.toString(),
      text: text,
      isUser: false,
      timestamp: DateTime.now(),
    );
    _messages.add(assistantMessage);
    notifyListeners();
  }

  /// 清空消息
  void clearMessages() {
    _messages.clear();
    _addSystemMessage('对话已清空');
    notifyListeners();
  }

  @override
  void dispose() {
    _udpClient.dispose();
    super.dispose();
  }
}