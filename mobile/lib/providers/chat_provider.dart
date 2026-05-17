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
  String? _currentAgentId;

  List<Message> get messages => _messages;
  bool get isConnected => _isConnected;
  bool get isSending => _isSending;
  String? get serverAddress => _serverAddress;
  int? get serverPort => _serverPort;
  String? get currentAgentId => _currentAgentId;

  /// 连接到 co-shell-hub 服务器
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

  /// 获取 Agent 列表
  Future<List<Map<String, dynamic>>> getAgents() async {
    if (!_isConnected) {
      throw Exception('未连接到服务器');
    }

    final result = await _udpClient.sendRequest({
      'type': 'get_agents',
    });

    if (result == null) {
      throw Exception('获取 Agent 列表失败');
    }

    final agents = result['agents'] as List<dynamic>?;
    if (agents == null) {
      return [];
    }

    return agents.map((e) => e as Map<String, dynamic>).toList();
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
      final result = await _udpClient.sendRequest({
        'type': 'message',
        'agent_id': _currentAgentId ?? 'default',
        'content': text,
        if (imagePaths != null) 'images': imagePaths,
      });

      if (result != null && result['type'] == 'message') {
        final responseText = result['content'] as String?;
        if (responseText != null && responseText.isNotEmpty) {
          _addAssistantMessage(responseText);
        }
      }
    } catch (e) {
      _addSystemMessage('发送失败: $e');
    } finally {
      _isSending = false;
      notifyListeners();
    }
  }

  /// 设置当前 Agent
  void setCurrentAgent(String agentId) {
    _currentAgentId = agentId;
    _messages.clear();
    _addSystemMessage('已切换到 Agent: $agentId');
    notifyListeners();
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

  /// 添加助手消息
  void _addAssistantMessage(String text) {
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