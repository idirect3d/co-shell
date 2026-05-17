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

/// 消息模型
class Message {
  final String id;
  final String text;
  final bool isUser;
  final bool isSystem;
  final List<String>? imagePaths;
  final DateTime timestamp;

  Message({
    required this.id,
    required this.text,
    required this.isUser,
    this.isSystem = false,
    this.imagePaths,
    required this.timestamp,
  });

  /// 从 JSON 创建消息
  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      text: json['text'] as String,
      isUser: json['isUser'] as bool,
      isSystem: json['isSystem'] as bool? ?? false,
      imagePaths: (json['imagePaths'] as List<dynamic>?)?.cast<String>(),
      timestamp: DateTime.parse(json['timestamp'] as String),
    );
  }

  /// 转换为 JSON
  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'text': text,
      'isUser': isUser,
      'isSystem': isSystem,
      'imagePaths': imagePaths,
      'timestamp': timestamp.toIso8601String(),
    };
  }

  /// 复制并修改
  Message copyWith({
    String? id,
    String? text,
    bool? isUser,
    bool? isSystem,
    List<String>? imagePaths,
    DateTime? timestamp,
  }) {
    return Message(
      id: id ?? this.id,
      text: text ?? this.text,
      isUser: isUser ?? this.isUser,
      isSystem: isSystem ?? this.isSystem,
      imagePaths: imagePaths ?? this.imagePaths,
      timestamp: timestamp ?? this.timestamp,
    );
  }
}