# AI Coding 开发入门教程（纯操作版）

> 写给零基础入门者 · 不讲概念 · 只有步骤
> 更新时间：2026年4月

---

## 目录

- [准备工作：搞到一个大模型 API Key](#准备工作搞到一个大模型-api-key)
- [第一部分：VS Code 配置大模型](#第一部分vscode-配置大模型)
- [第二部分：Cline 插件配置（VS Code 内）](#第二部分cline-插件配置vscode-内)
- [第三部分：Cherry Studio 配置（桌面独立应用）](#第三部分cherry-studio-配置桌面独立应用)
- [常见问题](#常见问题)

---

## 准备工作：搞到一个大模型 API Key

**你需要先有一个大模型的 API Key**，下面任选一个就行（推荐第 1 个）：

| 提供商 | 官网 | 费用 | 推荐理由 |
|--------|------|------|----------|
| ✅ **DeepSeek** | platform.deepseek.com | 非常便宜（新用户送额度） | 国产、便宜、中文好 |
| ✅ **OpenAI** | platform.openai.com | 按量付费 | 最通用，但需翻墙+绑信用卡 |
| ✅ **硅基流动 (SiliconFlow)** | cloud.silflow.cn | 注册送额度 | 国内可直接访问，提供多种模型 |
| ✅ **阿里百炼** | bailian.console.aliyun.com | 新用户送额度 | 国内大厂，稳定 |
| ✅ **Ollama（本地）** | ollama.com | 免费 | 完全本地运行，吃电脑配置 |

> 💡 **新手最推荐：硅基流动**。国内直接访问，注册送 14 元额度（够用很久），而且支持 DeepSeek、Qwen 等多种模型，方便一键切换。

---

## 第一步：安装 VS Code

> 如果你已经安装了 VS Code，跳到 [第二步](#第二步安装-cline-插件)。

**① 下载 VS Code**

打开浏览器，访问：**https://code.visualstudio.com**

点击页面中间的蓝色按钮 **"Download for Mac"**（Mac 用户），或选择对应的系统版本。

**② 安装 VS Code**

- **Mac 用户**：下载的是 `.zip` 文件，解压后将 `Visual Studio Code.app` 拖入 **应用程序** 文件夹。
- **Windows 用户**：下载 `.exe` 文件，双击运行，一路点"下一步"完成安装。

**③ 打开 VS Code**

从启动台（Mac）或开始菜单（Windows）找到 **Visual Studio Code** 并打开。

---

## 第二步：安装 Cline 插件

> Cline 是 VS Code 里最流行的 AI Coding 插件之一，能让你在编辑器里直接对话 AI 写代码。

**① 打开插件市场**

点击 VS Code 左侧活动栏的 **方块图标**（扩展商店），或者按快捷键：

- Mac：`⇧ + ⌘ + X`
- Windows：`Ctrl + Shift + X`

**② 搜索 Cline**

在搜索框里输入：**`Cline`**

**③ 安装插件**

在搜索结果中找到 **"Cline"**（图标是一个橙色的机器人头），点击右侧绿色的 **Install** 按钮。

> ⚠️ 注意：Cline 插件可能有多个同名或类似名称（如 "Cline - Autonomous Coding Agent"），安装下载量最高的那个（通常是 10万+ 下载）。

**④ 等待安装完成**

安装过程只需要几秒钟，安装成功后右侧按钮会变成设置齿轮图标。

---

## 第三步：配置 Cline 的大模型

这一步是核心，**分几种不同提供商的情况**，选你有的那个跟着做就行。

### 情况 A：使用 DeepSeek（推荐，便宜好用）

**① 打开 Cline 设置**

点击 VS Code 左侧活动栏的 **Cline 图标**（橙色机器人头），打开 Cline 面板。

> 如果左侧没有 Cline 图标，检查一下 VS Code 顶部菜单栏 → View → Appearance → Activity Bar 确保已勾选。

**② 选择 API 提供商**

在 Cline 面板顶部，找到 **API Provider** 下拉菜单，选择：

```
DeepSeek
```

> 如果下拉菜单里没有 DeepSeek，选 `OpenAI Compatible`，然后在下方输入自定义地址。

**③ 填写 API 地址和 Key**

在出现的输入框中，填入：

| 字段 | 填写内容 |
|------|----------|
| **API Base URL** | `https://api.deepseek.com` |
| **API Key** | 粘贴你在 DeepSeek 平台申请的 Key |
| **Model** | 选 `deepseek-chat`（通用对话）或 `deepseek-coder`（代码专用） |

**④ 测试连接**

点击 **"Check"**  **"Test"** 按钮，如果显示绿色成功提示，说明配置正确。

### 情况 B：使用 OpenAI

**① 打开 Cline 设置**（同上）

**② 选择 API 提供商**

```
OpenAI
```

**③ 填写 API Key**

| 字段 | 填写内容 |
|------|----------|
| **API Key** | 粘贴你的 OpenAI API Key |
| **Model** | 选 `gpt-4o` 或 `gpt-4o-mini` |

> ⚠️ OpenAI 的默认 API 地址是 `https://api.openai.com`，一般不需要修改。

### 情况 C：使用硅基流动（SiliconFlow）

**① 打开 Cline 设置**

**② 选择 API 提供商**

```
OpenAI Compatible
```

**③ 填写自定义参数**

| 字段 | 填写内容 |
|------|----------|
| **Base URL** | `https://api.siliconflow.cn/v1` |
| **API Key** | 你在硅基流动平台申请的 Key |
| **Model** | 填写 `deepseek-ai/DeepSeek-V3` 或 `Qwen/Qwen2.5-Coder-7B-Instruct` |

### 情况 D：使用本地 Ollama（免费，不吃网络）

**前提**：已安装 Ollama，并下载了模型（如 `ollama pull qwen2.5-coder`）

**① 打开 Cline 设置**

**② 选择 API 提供商**

```
Ollama
```

**③ 配置本地地址**

| 字段 | 填写内容 |
|------|----------|
| **Base URL** | `http://localhost:11434` |
| **Model** | 填写你下载的模型名称，如 `qwen2.5-coder` |

### 通用配置项（所有提供商都要调）

在 Cline 面板下方，找到以下设置：

| 参数 | 建议值 | 说明 |
|------|--------|------|
| **Temperature** | `0.2` ~ `0.5` | 越低越严谨，写代码建议 0.2 |
| **Max Tokens** | `8192` 或更大 | 模型一次最多输出的字数 |
| **Context Window** | 按模型支持的最大值填 | 上下文窗口大小 |

**设置完毕！** 现在你可以在 Cline 面板底部的输入框中，用中文描述你的需求，AI 就会开始帮你写代码了。

---

## 第四步：Cherry Studio 配置（可选，但推荐）

> Cherry Studio 是一个**独立桌面应用**，相当于一个"大模型集合器"，可以让你在桌面端统一管理和使用各种大模型。它不是 VS Code 插件，而是单独运行的。

### 4.1 下载安装 Cherry Studio

**① 访问官网下载**

打开浏览器：**https://cherrystudio.app** 或 **https://github.com/CherryHQ/cherry-studio**

**② 选择系统版本**

- **Mac 用户**：下载 `.dmg` 文件
- **Windows 用户**：下载 `.exe` 文件

**③ 安装**

- **Mac**：双击 `.dmg`，将 Cherry Studio 拖入 **应用程序** 文件夹。
- **Windows**：双击 `.exe`，一路下一步安装。

**④ 打开 Cherry Studio**

安装后打开，你会看到一个类似聊天软件的界面。

### 4.2 配置 DeepSeek（以 DeepSeek 为例）

**① 打开设置**

点击左下角的 **齿轮图标**（设置），进入设置页面。

**② 添加模型提供商**

左侧菜单选择 **"模型提供商"**，点击右侧的 **"添加"** 按钮。

**③ 选择提供商**

在弹出的列表中找到并选择 **"DeepSeek"**。

**④ 填写 API Key**

| 字段 | 填写内容 |
|------|----------|
| **API Key** | 粘贴你的 DeepSeek API Key |

其他字段（Base URL 等）会自动填充，不用改。

**⑤ 选择模型**

返回聊天界面，左上角的下拉菜单中，选择 `deepseek-chat` 或 `deepseek-coder`。

**⑥ 开始对话**

在底部输入框打字，按回车发送，Cherry Studio 就会调用 DeepSeek 回复你。

### 4.3 配置 OpenAI

**① 设置 → 模型提供商 → 添加 → 选择 OpenAI**

**② 填入 API Key**

| 字段 | 填写 |
|------|----------|
| **API Key** | 粘贴你的 OpenAI API Key |

**③ 选择模型**

回到聊天界面，选择 `gpt-4o` 或 `gpt-4o-mini`。

### 4.4 配置硅基流动

**① 设置 → 模型提供商 → 添加 →  OpenAI Compatible**

**② 填写参数**

| 字段 | 填写内容 |
|------|----------|
| **名称** | 随便起名，如 `硅基流动` |
| **API Base URL** | `https://api.siliconflow.cn/v1` |
| **API Key** | 粘贴你的硅基流动 Key |

**③ 添加模型**

在 **模型列表** 中，点击 **"添加模型"**，输入模型名称如 `deepseek-ai/DeepSeek-V3`。

### 4.5 配置 Ollama（本地）

**① 设置 → 模型提供商 → 添加 → 选择 Ollama**

**② 填写地址**

| 字段 | 填写内容 |
|------|----------|
| **Base URL** | `http://localhost:11434` |

**③ 选择模型**

Cherry Studio 会自动检测你本地 Ollama 中已下载的模型，在下拉菜单中选择即可。

### 4.6 让 Cherry Studio 对接 VS Code（进阶）

Cherry Studio 有一个很有用的功能：**AI 对话内容可以直接复制到 VS Code**。

**操作方式：**

1. 在 Cherry Studio 中让 AI 生成代码
2. 点击代码块右上角的 **复制** 按钮
3. 回到 VS Code 中，粘贴到你的文件中

---

## 快速参考卡

### 常用模型的 API 地址速查表

| 提供商 | Base URL | 推荐模型 |
|--------|----------|----------|
| **DeepSeek** | `https://api.deepseek.com` | `deepseek-chat`, `deepseek-coder` |
| **OpenAI** | `https://api.openai.com/v1` | `gpt-4o`, `gpt-4o-mini` |
| **硅基流动** | `https://api.siliconflow.cn/v1` | `deepseek-ai/DeepSeek-V3`, `Qwen/Qwen2.5-Coder-7B-Instruct` |
| **阿里百炼** | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-turbo`, `qwen-plus` |
| **Ollama（本地）** | `http://localhost:11434` | `qwen2.5-coder` 等本地模型 |

### 新手推荐组合

```
VS Code + Cline插件 + 硅基流动（API Key）
```

这套组合最省事：不用翻墙、注册送额度、一键配置。

---

## 常见问题

### Q1：Cline 插件没有显示在左侧栏？

检查：
- 确保安装成功（扩展商店里显示已安装）
- 重启 VS Code
- 如果还是没有，按快捷键 `⌘ + ⇧ + P`（Mac）或 `Ctrl + Shift + P`（Win），输入 `Cline: Focus on Cline View` 回车

### Q2：提示 "API Key 无效"？

- 检查 Key 是否复制完整（包括所有字符）
- 检查 Key 前后是否有空格
- 去对应平台重新生成一个新的 Key 试试

### Q3：Cline 回复报错 "401" 或 "403"？

- **401**：API Key 错误或未填写
- **403**：账户余额不足或 Key 被禁用

### Q4：Cherry Studio 添加模型后看不到？

- 添加模型提供商后，需要回到聊天界面，**重新打开左上角模型下拉菜单**，新模型才会出现
- 如果还是没有，重启 Cherry Studio

### Q5：用哪个模型写代码最好？

| 你的情况 | 推荐模型 |
|----------|----------|
| 要便宜又要好用 | DeepSeek (`deepseek-coder`) |
| 追求最强效果 | GPT-4o |
| 免费、本地运行 | Qwen2.5-Coder（通过 Ollama） |
| 国内、免翻墙 | 硅基流动上的 DeepSeek-V3 |

---

> **下一步做什么？**
>
> 配置完成后，试试在 Cline 中输入：
> ```
> 帮我用 Python 写一个计算器程序
> ```
>
> 或者：
> ```
> 用 HTML + CSS 写一个漂亮的个人主页
> ```
>
> AI 就会自动生成代码，你只需要复制粘贴运行即可。遇到任何报错，直接把报错信息复制给 AI，它会帮你修复。
