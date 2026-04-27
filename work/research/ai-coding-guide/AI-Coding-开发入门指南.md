# 🚀 AI Coding 开发入门指南

> 面向零基础新手 · 手把手教你配置 AI 编程环境  
> 更新时间：2026 年 4 月

---

## 📖 目录

1. [什么是 AI Coding？](#1-什么是-ai-coding)
2. [准备工作：获取大模型 API Key](#2-准备工作获取大模型-api-key)
3. [工具一：VSCode + Cline 插件](#3-工具一vscode--cline-插件)
4. [工具二：Cherry Studio](#4-工具二cherry-studio)
5. [常见问题](#5-常见问题)

---

## 1. 什么是 AI Coding？

**AI Coding** 就是用人工智能来辅助你写代码。你可以像跟同事聊天一样，用自然语言告诉 AI 你想要什么功能，它帮你生成代码、修改 Bug、解释代码逻辑。

本教程涉及的三个核心工具：

| 工具 | 作用 |
|------|------|
| **VSCode** | 代码编辑器（写代码的地方） |
| **Cline 插件** | VSCode 里的 AI 助手（帮你写代码） |
| **Cherry Studio** | 桌面 AI 客户端（聊天、调试、管理） |

它们本身**不包含 AI 模型**，需要连接大模型提供商的 API 才能工作。

---

## 2. 准备工作：获取大模型 API Key

AI 工具需要连接"大模型"（比如 GPT-4、Claude、DeepSeek 等）才能回答问题。你需要去模型提供商那里注册并获取一串密钥（API Key）。

### 常见模型提供商推荐

| 提供商 | 推荐模型 | 特点 | 官网 |
|--------|---------|------|------|
| **DeepSeek** | deepseek-chat | ✅ 便宜、中文好、编程强 | https://platform.deepseek.com |
| **OpenAI** | gpt-4o / gpt-4o-mini | ✅ 通用最强 | https://platform.openai.com |
| **Anthropic** | claude-sonnet-4 | ✅ 编程能力顶尖 | https://console.anthropic.com |
| **硅基流动 (SiliconFlow)** | 各类开源模型 | ✅ 国内直连、便宜、无需翻墙 | https://siliconflow.cn |
| **阿里云百炼** | qwen-max / qwen-turbo | ✅ 国内直连、免费额度多 | https://bailian.aliyun.com |

> 💡 **新手建议**：先从 **DeepSeek** 或 **硅基流动** 开始，注册简单、价格便宜、国内网络可直接访问。

### 获取步骤（以 DeepSeek 为例）

1. 打开 https://platform.deepseek.com
2. 注册账号（手机号或邮箱）
3. 进入控制台 → **API** 页面
4. 点击 **创建 API Key**，复制并保存好（⚠️ 关闭页面后就看不到了）
5. 充一点钱（比如 10 元），能用很久

> 获取到 API Key 后，后面所有工具配置都需要用到它。

---

## 3. 工具一：VSCode + Cline 插件

### 3.1 安装 VSCode

1. 打开官网下载安装：https://code.visualstudio.com
2. 安装完成后打开 VSCode，界面是英文的（后面可以装中文插件）

### 3.2 安装 Cline 插件

1. 点击左侧工具栏的 **扩展图标**（四个方块，或按快捷键 `Cmd+Shift+X`）
2. 在搜索框输入 **"Cline"**
3. 找到 **Cline**（图标是一个机器人头），点击 **Install**
4. 安装完成后，左侧会出现一个机器人图标

![示意：扩展搜索 Cline]

### 3.3 配置 Cline 连接大模型

点击左侧的 **机器人图标** → 点击顶部的 **齿轮图标（设置）** → 进入配置页面。

你需要配置以下几个关键参数：

#### ⚙️ 核心配置参数

| 参数 | 说明 | 示例值 |
|------|------|--------|
| **API Provider** | 选择你的模型提供商 | OpenAI / Anthropic / DeepSeek / ... |
| **API Key** | 你注册获取的密钥 | sk-xxxxxxxxxxxxxxxx |
| **Base URL** | API 接口地址（部分提供商需要） | https://api.deepseek.com |
| **Model ID** | 要使用的具体模型名称 | deepseek-chat / gpt-4o / claude-sonnet-4-20250514 |

#### 📋 常见提供商配置模板

**① DeepSeek（推荐新手）**

```
API Provider: OpenAI (兼容模式)
Base URL:    https://api.deepseek.com
API Key:     你的 DeepSeek API Key
Model ID:    deepseek-chat
```

**② 硅基流动（免翻墙、便宜）**

```
API Provider: OpenAI (兼容模式)
Base URL:    https://api.siliconflow.cn/v1
API Key:     你的硅基流动 API Key
Model ID:    deepseek-ai/DeepSeek-V3  （或其他模型）
```

> 硅基流动上可选模型很多，在官网 https://siliconflow.cn/models 查看可用模型列表。

**③ OpenAI（需翻墙）**

```
API Provider: OpenAI
Base URL:    https://api.openai.com （默认，可不填）
API Key:     你的 OpenAI API Key
Model ID:    gpt-4o
```

**④ Anthropic Claude**

```
API Provider: Anthropic
Base URL:    https://api.anthropic.com （默认，可不填）
API Key:     你的 Anthropic API Key
Model ID:    claude-sonnet-4-20250514
```

**⑤ 阿里云百炼（国内直连）**

```
API Provider: OpenAI (兼容模式)
Base URL:    https://dashscope.aliyuncs.com/compatible-mode/v1
API Key:     你的阿里云百炼 API Key
Model ID:    qwen-max
```

### 3.4 开始使用 Cline

配置完成后：

1. 在 VSCode 中打开一个项目文件夹（`File → Open Folder`）
2. 点击左侧机器人图标打开 Cline 面板
3. 在底部的输入框中输入你想做的事情，比如：
   - *"帮我创建一个计算器 HTML 页面"*
   -"给这个函数加上注释"*
   - *"这段代码有 Bug，帮我修一下"*
4. Cline 会自动分析、生成代码，你可以点 **Accept** 或 **Reject** 来决定是否采纳

> ✅ **恭喜！你已经成功配置了 AI Coding 环境！**

---

## 4. 工具二：Cherry Studio

Cher Studio 是一个桌面 AI 客户端，可以理解成"AI 聊天桌面版"，支持多种模型，用来问问题、写代码、管理对话历史等。

### 4.1 下载安装

1. 打开官网下载：https://cherrystudio.ai 或去 GitHub Release 页面
2. 下载对应系统的安装包（macOS 选 `.dmg`，Windows 选 `.exe`）
3. 安装并打开

### 4.2 配置模型提供商

打开 Cherry Studio 后：

. 点击左下角的 **⚙️ 设置** 按钮
2. 进入 **模型提供商** 设置页面
3. 点击 **添加模型提供商**

#### ⚙️ 配置参数说明

| 参数 | 说明 | 示例值 |
|------|------|--------|
| **名称** | 给这个提供商起个名字 | DeepSeek / 硅基流动 |
| **API 地址** | 接口地址 | https://api.deepseek.com/v1 |
| **API Key** | 你的密钥 | sk-xxxxxxxx |
| **模型列表** | 可用的模型名称 | deepseek-chat |

#### 📋 常见配置模板

**① DeepSeek**

```
名称：        DeepSeek
API 地址：    https://api.deepseek.com/v1
API Key：     你的 DeepSeek Key
模型列表：    deepseek-chat
```

**② 硅基流动**

```
名称：        硅基流动
API 地址：    https://api.siliconflow.cn/v1
API Key：     你的硅基流动 Key
模型列表：    Qwen/Qwen2.5-72B-Instruct / deepseek-ai/DeepSeek-V3（去官网选你想要的）
```

**③ OpenAI**

```
名称：        OpenAI
 地址：    https://api.openai.com/v1
API Key：     你的 OpenAI Key
模型列表：    gpt-4o / gpt-4o-mini
```

**④ 阿里云百炼**

```
名称：        阿里云百炼
API 地址：    https://dashscope.aliyuncs.com/compatible-mode/v1
API Key：     你的阿里百炼 Key
模型列表：    qwen-max / qwen-turbo
```

### 4.3 开始使用 Cherry Studio

配置完成后：

1. 在左上角选择你配置好的模型
2. 就可以在对话框里跟 AI 聊天了
3. Cherry Studio 还内置了**代码高亮**、**Markdown 预览**、**对话导出**等功能

---

## 5. 常见问题

### ❓ API Key 是什么？在哪里找？

API Key 是一串密钥（如 `sk-xxxxxxxxxxxx`），用来验证你的身份。去模型提供商的官网注册 → 控制台 → API Keys 页面创建。

### ❓ 为什么配置好了但提示报错？

- **检查 API Key**：是否复制完整，注意不要有多余空格
- **检查 Base URL**：地址末尾有没有 `/v1`，看提供商要求
- **检查余额**：账户里是否有足够余额
- **检查网络**：国内访问 OpenAI/Anthropic 需要翻墙

### ❓ 免费模型有哪些？

| 服务 | 免费模型 | 说明 |
|------|---------|------|
| **GitHub Models** | gpt-4o-mini 等 | 需 GitHub 账号，有每日限额 |
| **硅基流动** | 部分开源模型免费 | 注册就送额度 |
| **阿里云百炼** | qwen-turbo | 有免费调用额度 |
| **DeepSeek** | - | 充值门槛低，10 元用很久 |

### ❓ Cline 和 Cherry Studio 有什么区别？

| 对比 | Cline | Cherry Studio |
|------|-------|--------------|
| 定位 | VSCode 里的编程助手 | 桌面 AI 聊天客户端 |
| 核心功能 | 直接在项目中写代码、改文件 | 聊天、问答、管理对话 |
| 适用场景 | 编程开发 | 通用问答、代码片段探查 |

> 两者不冲突，可以同时使用！

### ❓ 配置完没反应 / 很慢？

- 先检查网络连接
- 检查 API Key 是否有效
- 尝试换个更快的模型（如 `deepseek-chat` 比 `gpt-4o` 快且）

---

## 🔗 总结

```
你需要做的就 3 步：
1️⃣ 去模型提供商注册 → 获取 API Key
2️⃣ 在工具里填上 API Key + 地址 + 模型名
3️⃣ 开始用 AI 帮你写代码！
```

> **推荐新手路线**：  
> 硅基流动注册（免翻墙）→ VSCode + Cline →  DeepSeek-V3 模型 → 开始写代码 🎉

---

*Happy Coding! 🚀*
