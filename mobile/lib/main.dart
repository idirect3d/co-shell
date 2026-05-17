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
import 'config/constants.dart';
import 'providers/chat_provider.dart';
import 'screens/agent_list_screen.dart';

void main() {
  // 设置状态栏样式
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
    ),
  );

  runApp(const CoShellApp());
}

class CoShellApp extends StatelessWidget {
  const CoShellApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => ChatProvider()),
      ],
      child: MaterialApp(
        title: Constants.appName,
        debugShowCheckedModeBanner: false,
        themeMode: ThemeMode.system, // 自适应系统明/暗模式
        theme: _lightTheme, // 浅色主题
        darkTheme: _darkTheme, // 深色主题（终端风格）
        home: const AgentListScreen(), // 首页：Agent 列表
      ),
    );
  }

  /// 浅色主题（终端风格）
  static final ThemeData _lightTheme = ThemeData(
    useMaterial3: true,
    brightness: Brightness.light,
    // 终端风格字体
    textTheme: const TextTheme(
      displayLarge: TextStyle(fontFamily: 'Courier New', fontSize: 24, fontWeight: FontWeight.bold),
      headlineMedium: TextStyle(fontFamily: 'Courier New', fontSize: 18, fontWeight: FontWeight.bold),
      bodyLarge: TextStyle(fontFamily: 'Courier New', fontSize: 16),
      bodyMedium: TextStyle(fontFamily: 'Courier New', fontSize: 14),
      bodySmall: TextStyle(fontFamily: 'Courier New', fontSize: 12),
    ),
    // 终端风格配色
    colorScheme: const ColorScheme.light(
      primary: Colors.black,
      secondary: Colors.green,
      surface: Color(0xFFF5F5F5),
      error: Colors.red,
      onPrimary: Colors.white,
      onSecondary: Colors.black,
      onSurface: Colors.black,
      onError: Colors.white,
    ),
    // AppBar 样式
    appBarTheme: const AppBarTheme(
      backgroundColor: Colors.black,
      foregroundColor: Colors.white,
      elevation: 0,
      systemOverlayStyle: SystemUiOverlayStyle.light,
    ),
    // 按钮样式
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        textStyle: const TextStyle(fontFamily: 'Courier New'),
      ),
    ),
    // 输入框样式
    inputDecorationTheme: const InputDecorationTheme(
      border: OutlineInputBorder(),
      hintStyle: TextStyle(fontFamily: 'Courier New'),
      labelStyle: TextStyle(fontFamily: 'Courier New'),
    ),
  );

  /// 深色主题（终端风格 - 经典黑底绿字）
  static final ThemeData _darkTheme = ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    // 终端风格字体
    textTheme: const TextTheme(
      displayLarge: TextStyle(fontFamily: 'Courier New', fontSize: 24, fontWeight: FontWeight.bold, color: Color(0xFF00FF00)),
      headlineMedium: TextStyle(fontFamily: 'Courier New', fontSize: 18, fontWeight: FontWeight.bold, color: Color(0xFF00FF00)),
      bodyLarge: TextStyle(fontFamily: 'Courier New', fontSize: 16, color: Color(0xFF00DD00)),
      bodyMedium: TextStyle(fontFamily: 'Courier New', fontSize: 14, color: Color(0xFF00CC00)),
      bodySmall: TextStyle(fontFamily: 'Courier New', fontSize: 12, color: Color(0xFF00AA00)),
    ),
    // 终端风格配色 - 黑底绿字
    colorScheme: const ColorScheme.dark(
      primary: Color(0xFF00FF00),
      secondary: Color(0xFF00DD00),
      surface: Color(0xFF1A1A1A),
      error: Color(0xFFFF0000),
      onPrimary: Color(0xFF0A0A0A),
      onSecondary: Color(0xFF0A0A0A),
      onSurface: Color(0xFF00FF00),
      onError: Color(0xFF0A0A0A),
    ),
    // AppBar 样式
    appBarTheme: const AppBarTheme(
      backgroundColor: Color(0xFF1A1A1A),
      foregroundColor: Color(0xFF00FF00),
      elevation: 0,
      systemOverlayStyle: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.dark,
      ),
    ),
    // 按钮样式
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: const Color(0xFF00FF00),
        foregroundColor: const Color(0xFF0A0A0A),
        textStyle: const TextStyle(fontFamily: 'Courier New'),
      ),
    ),
    // 输入框样式
    inputDecorationTheme: const InputDecorationTheme(
      border: OutlineInputBorder(),
      hintStyle: TextStyle(fontFamily: 'Courier New', color: Color(0xFF00AA00)),
      labelStyle: TextStyle(fontFamily: 'Courier New', color: Color(0xFF00FF00)),
    ),
    // 卡片样式
    cardTheme: const CardThemeData(
      color: Color(0xFF1A1A1A),
      elevation: 0,
    ),
  );
}