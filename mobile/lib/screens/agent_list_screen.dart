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

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../config/constants.dart';
import '../providers/chat_provider.dart';
import 'chat_screen.dart';

/// Agent 会话列表页面
class AgentListScreen extends StatefulWidget {
  const AgentListScreen({super.key});

  @override
  State<AgentListScreen> createState() => _AgentListScreenState();
}

class _AgentListScreenState extends State<AgentListScreen> {
  List<Map<String, dynamic>> _agents = [];
  bool _isLoading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadAgents();
  }

  /// 加载 agent 列表
  Future<void> _loadAgents() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final provider = context.read<ChatProvider>();
      final agents = await provider.getAgents();
      
      setState(() {
        _agents = agents;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  /// 选择 agent 并进入聊天
  void _selectAgent(String agentId, String agentName) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => ChatScreen(agentId: agentId, agentName: agentName),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('co-shell'),
        actions: [
          // 刷新按钮
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadAgents,
            tooltip: '刷新',
          ),
          // 设置按钮
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => _showSettingsDialog(),
            tooltip: '设置',
          ),
        ],
      ),
      body: _buildBody(),
    );
  }

  /// 构建页面主体
  Widget _buildBody() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, size: 48, color: Colors.red),
            const SizedBox(height: 16),
            Text('加载失败: $_error'),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _loadAgents,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    if (_agents.isEmpty) {
      return const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.people_outline, size: 64, color: Colors.grey),
            SizedBox(height: 16),
            Text('暂无 Agent', style: TextStyle(fontSize: 18)),
            SizedBox(height: 8),
            Text('请检查连接设置', style: TextStyle(color: Colors.grey)),
          ],
        ),
      );
    }

    return ListView.builder(
      itemCount: _agents.length,
      itemBuilder: (context, index) {
        final agent = _agents[index];
        return ListTile(
          leading: CircleAvatar(
            backgroundColor: Theme.of(context).colorScheme.primaryContainer,
            child: const Icon(Icons.smart_toy, size: 24),
          ),
          title: Text(agent['name'] as String),
          subtitle: Text(agent['id'] as String),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => _selectAgent(agent['id'] as String, agent['name'] as String),
        );
      },
    );
  }

  /// 显示设置对话框
  void _showSettingsDialog() {
    showDialog(
      context: context,
      builder: (context) {
        return const ConnectionSettingsDialog();
      },
    );
  }
}

/// 连接设置对话框
class ConnectionSettingsDialog extends StatefulWidget {
  const ConnectionSettingsDialog({super.key});

  @override
  State<ConnectionSettingsDialog> createState() => _ConnectionSettingsDialogState();
}

class _ConnectionSettingsDialogState extends State<ConnectionSettingsDialog> {
  final _addressController = TextEditingController();
  final _portController = TextEditingController();
  String? _error;

  @override
  void initState() {
    super.initState();
    final provider = context.read<ChatProvider>();
    _addressController.text = provider.serverAddress ?? Constants.defaultServerAddress;
    _portController.text = provider.serverPort?.toString() ?? Constants.hubPort.toString();
  }

  @override
  void dispose() {
    _addressController.dispose();
    _portController.dispose();
    super.dispose();
  }

  /// 保存设置并连接
  Future<void> _saveAndConnect() async {
    final address = _addressController.text.trim();
    final port = int.tryParse(_portController.text.trim()) ?? Constants.hubPort;

    if (address.isEmpty) {
      setState(() {
        _error = '请输入服务器地址';
      });
      return;
    }

    try {
      setState(() {
        _error = null;
      });

      final navigator = Navigator.of(context);
      final provider = Provider.of<ChatProvider>(context, listen: false);
      final success = await provider.connect(address, port);

      if (!success) {
        setState(() {
          _error = '连接失败，请检查地址和端口';
        });
      } else {
        navigator.pop();
      }
    } catch (e) {
      setState(() {
        _error = '连接失败: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('连接设置'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: _addressController,
              decoration: const InputDecoration(
                labelText: '服务器地址',
                hintText: '例如: 192.168.1.100',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _portController,
              decoration: InputDecoration(
                labelText: '端口',
                hintText: '例如: ${Constants.hubPort}',
                border: const OutlineInputBorder(),
              ),
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.only(top: 8.0),
                child: Text(
                  _error!,
                  style: const TextStyle(color: Colors.red),
                ),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
        ElevatedButton(
          onPressed: _saveAndConnect,
          child: const Text('连接'),
        ),
      ],
    );
  }
}