# AI Coding 开发入门教程（纯操作版）

> 不讲概念，只讲。跟着步骤走，10 分钟搞定 AI 编程环境。

---

## 第一章：安装必备工具

### 1.1 安装 VS Code

**Step 1** 打开浏览器，访问：https://code.visualstudio.com/

**Step2** 点击下载按钮（自动识别你的操作系统），下载安装包。

**Step 3** 双击安装包，点击「下一步」完成安装。

**Step 4** 打开 VS Code，界面应该是英文的，继续往下走，后面装完中文插件会自动切换。

---

### 1.2 安装 Cherry Studio

**Step 1** 打开浏览器，访问 Cherry Studio 官方 GitHub 发布页：  
https://github.com/CherryHQ/cherry-studio/releases

**Step 2** 找到最新版本，下载对应你系统的安装包：
- Windows：下载 `Cherry-Studio-x.x.x-win.exe`
- macOS（Intel）：下载 `Cherry-Studio-x.x.x-x64.dmg`
- macOS（Apple Silicon，如 M1/M2/M3/M4）：下载 `Cherry-Studio-x.x.x-arm64.dmg`

**Step 3** 双击安装包，将 Cherry Studio 拖入「应用程序」文件夹（macOS）或运行安装程序（Windows）。

**Step 4** 打开 Cherry Studio，界面如下：

```
┌─────────────────────────────────────────┐
│  💬 新对话     ⚙️ 设置      📚 知识库     │
│─────────────────────────────────────────│
│                                         │
│         欢迎使用 Cherry Studio           │
│        请先在设置中添加模型             │
│                                         │
└─────────────────────────────────────────┘
```

---

### 1.3 VS Code 安装 Cline 插件

**Step 1** 打开 VS Code，点击左侧活动栏的 **方块图标**（扩展商店）。

**Step 2** 在搜索框输入：`Cline`

**Step 3** 找到 **Cline**（作者：saoudrizwan），点击「Install」。

**Step 4** 安装完成后，左侧会出现一个 **机器人图标**，点击它：

```
左侧活动栏出现 🤖 Cline 图标 → 点击进入 Cline 面板
```

---

## 第二章：获取大模型 API Key

> ⚠️ **重要**：AI Coding 需要调用大模型的 API，你需要先注册并获取 API Key。

### 2.1 OpenAI（GPT-4o / GPT-4o-mini）

**Step 1** 访问：https://platform.openai.com/signup

**Step 2** 注册账号（需要手机号验证，建议用 Gmail 注册）。

**Step 3** 登录后，点击右上角头像 → 「API keys」。

**Step 4** 点击「Create new secret key」，输入名称后复制保存。

**Step 5** 充值：进入 Billing → 点击「Add payment details」→ 充值至少 $5。

> 💰 **费用参考**：GPT-4o-mini 非常便宜，日常 Coding 使用 $5 能用很久。

---

### 2.2 Anthropic（Claude Sonnet 4 / Claude 3.5 Sonnet）

**Step 1** 访问：https://console.anthropic.com/

**Step 2** 注册账号并登录。

**Step 3** 点击左侧「API Keys」→「Create Key」。

**Step 4** 复制并保存生成的 API Key。

**Step 5 充值：点击「Billing」→ 添加充值。

> 💡 **推荐**：Claude Sonnet 4 是当前 AI Coding 领域公认的最强模型之一，代码能力极强。

---

### 2.3 Google（Gemini 2.5 Pro / Flash）

**Step 1** 访问：https://aistudio.google.com/

**Step 2** 用 Google 账号登录。

**Step 3** 点击左侧「Get API key」→「Create API key」。

**Step 4** 选择（或新建）一个 Google Cloud 项目，生成 Key 并复制。

> 💡 **免费福利**：Gemini API 目前有 **免费配额**，无需充值即可使用，非常适合入门体验。

---

### 2.4 DeepSeek（国产，性价比之王）

**Step 1** 访问：https://platform.deepseek.com/

**Step 2** 注册账号（用手机号即可，无需国际信用卡）。

**Step 3** 登录后，点击左侧「API Keys」→「创建 API Key」。

**Step 4** 复制并保存 API Key。

**Step 5** 充值：点击「充值」，支持微信/支付宝，充 10 元就能用很久。

> ✅ **强烈推荐入门首选**：价格极低，国内访问速度快，无需科学上网。

---

### 2.5 通义千问（阿里云）

**Step 1** 访问：https://www.aliyun.com/product/ali-lab/model-studio

**Step 2** 用阿里云/支付宝账号登录。

**Step 3** 开通「模型服务灵积（DashScope）」服务。

**Step 4** 进入「API Key 管理」→ 创建 API Key。

**Step 5** 复制保存。通义千问有免费额度可用。

---

## 第三章：配置 VS Code + Cline

### 3.1 进入 Cline 设置

**Step 1** 点击 VS Code 左侧的 🤖 Cline 图标。

**Step 2** 点击 Cline 面板顶部的 **齿轮图标**（⚙️ Settings）。

**Step 3** 你会看到设置界面，核心配置项如下：

```
┌──────────────────────────────────────┐
│  🤖 Cline Settings                    │
│──────────────────────────────────────│
│  API Provider  [▼ OpenAI           ] │
│  API Key       [__________________]  │
│  Model         [gpt-4o           ]   │
│  Max Tokens    [8192              ]  │
│  Temperature   [0.0 ─────●──── 2.0]  │
└──────────────────────────────────────┘
```

---

### 32 配置 OpenAI（示例）

**Step 1** API Provider 选择：`OpenAI`

**Step 2** API Key 填入：在第 2.1 节获取的 `sk-...` 开头的 Key

**Step 3** Model 填写：`gpt-4o` 或 `gpt-4o-mini`

**Step 4** 其他保持默认：

| 参数 | 推荐值 | 说明 |
|------|--------|------|
| Max Tokens | `8192` | 单次最大输出长度 |
| Temperature | `0` | Coding 任务建议 0，结果更稳定 |

---

### 3.3 配置 Anthropic Claude

**Step 1** API Provider 选择：`Anthropic`

**Step 2** API Key 填入：第 2.2 节获取的 `sk-ant-...` 开头的 Key

**Step 3** Model 填写：`claude-sonnet-4-20250514`（最新版 Sonnet 4）

或 `claude-3-5-sonnet-20241022`（上一代，但非常稳定）

---

### 3.4 配置 DeepSeek（国内用户首选）

**Step 1** API Provider 选择：`Open Compatible`

**Step 2** 填入以下：

| 参数 | 值 |
|------|-----|
| Base URL | `https://api.deepseek.com` |
| API Key | 第 2.4 节获取的 DeepSeek Key |
| Model | `deepseek-chat` |

**界面填写示例：**

```
API Provider:  [▼ OpenAI Compatible   ]
Base URL:      [https://api.deepseek.com]
API Key:       [sk-xxxxxxxxxxxxxxxxxxxx]
Model:         [deepseek-chat           ]
```

---

### 3.5 配置通义千问

**Step 1** API Provider 选择：`OpenAI Compatible`

**Step 2** 填入：

| 参数 | 值 |
|------|-----|
| Base URL | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| API Key | 第 2.5 节获取的 Key |
| Model | `qwen-plus` 或 `qwen-turbo` |

---

### 3.6 配置 Google Gemini

**Step 1** API Provider 选择：`OpenAI Compatible`

**Step 2** 填入：

| 参数 | 值 |
|------|-----|
| Base URL | `https://generativelanguage.googleapis.com/v1beta/openai/` |
| API Key | 第 2.3 节获取的 Key |
| Model | `gemini-2.5-pro-exp-03-25` 或 `gemini-2.0-flash-exp` |

---

### 3.7 使用 Cline 开始 AI Coding

**Step 1** 在 VS Code 中打开你的项目文件夹：`文件 → 打开文件夹`

**Step 2** 按 `Cmd+Shift+P`（macOS）或 `Ctrl+Shift+P`（Windows），输入 `Cline: Open`。

**Step 3** 在 Cline 面板底部的输入框，直接输入你的需求，例如：

```
帮我创建一个 Python 计算器程序，支持加减乘除
```

**Step 4** Cline 会自动：
1. 分析你的需求
2. 创建文件并写入代码
3. 在终端中运行测试
4. 如果出错会自动修复

**Step 5** 你只需要点击「Approve」/「Save」按钮确认每次操作：

```
┌──────────────────────────────────────────┐
│  🤖 Cline 想要创建 calculator.py         │
│                                          │
│  [✏️ 查看差异]  [✅ Approve]  [❌ Reject] │
└──────────────────────────────────────────┘
```

> 💡 **提示**：每次 Cline 执行操作前都会请求你的批准，这是安全设计。

---

## 第四章：配置 Cherry Studio

### 4.1 添加模型提供商

**Step 1** 打开 Cherry Studio，点击左下角的 **⚙️ 设置**。

**Step 2** 点击左侧「模型提供商」。

**Step 3** 点击「添加模型」，选择提供商类型。

---

### 4.2 添加 OpenAI

**Step 1** 提供商选择：`OpenAI`

**Step 2** 填写：

```
API Key:   sk-xxxxxxxxxxxxxxxxxxxx
API Host:  https://api.openai.com
```

**Step 3** 点击「检查」按钮测试连接，显示「连接成功」即可。

**Step 4** 在下方「模型列表」中，点击「添加模型」：

```
模型名称:  gpt-4o
```

**Step 5** 点击「保存」。

---

### 4.3 添加 DeepSeek

**Step 1** 提供商选择：`OpenAI`

**Step 2** 填写：

```
API Key:   sk-xxxxxxxxxxxxxxxxxxxx
API Host:  https://api.deepseek.com
```

**Step 3** 点击「检查」→ 连接成功后，添加模型：

```
模型名称:  deepseek-chat
```

**Step 4** 点击「保存」。

---

### 4.4 添加通义千问

**Step 1** 提供商选择：`OpenAI`

**Step 2** 填写：

```
API Key:   sk-xxxxxxxxxxxxxxxxxxxx
API Host:  https://dashscope.aliyuncs.com/compatible-mode/v1
```

**Step 3** 添加模型：

```
模型名称:  qwen-plus
```

**Step 4** 点击「保存」。

---

### 4.5 添加 Anthropic Claude（如果 Cherry Studio 支持）

**Step 1** 提供商选择：`Anthropic`

**Step 2** 填写：

```
API Key:   sk-ant-xxxxxxxxxxxxxxxxxxxx
API Host:  https://api.anthropic.com
```

**Step 3** 添加模型：

```
模型名称:  claude-sonnet-4-20250514
```

**Step 4** 点击「保存」。

---

### 4.6 开始对话

**Step 1** 回到 Cherry Studio 主界面，点击左上角「💬 新对话」。

**Step 2** 在顶部的模型选择下拉框中，选择你刚添加的模型：

```
[▼ gpt-4o                    ]
 [gpt-4o]                    ✓
 [deepseek-chat]
 [qwen-plus]
```

**Step 3** 在底部输入框输入问题，例如：

```
用 Python 写一个文件批量重命名工具
```

**Step 4** 按 Enter 发送，AI 就会回复。

---

## 第五章：完整配置速查表

### 5.1 各模型配置参数一览

| 提供商 | API Provider 类型 | Base URL | 推荐 Model |
|--------|-------------------|----------|------------|
| OpenAI | OpenAI | https://api.openai.com | gpt-4o / gpt-4o-mini |
| Anthropic | Anthropic | https://api.anthropic.com | claude-sonnet-4-20250514 |
| Google Gemini | OpenAI Compatible | https://generativelanguage.googleapis.com/v1beta/openai/ | gemini-2.5-pro-exp-03-25 |
| DeepSeek | OpenAI Compatible | https://api.deepseek.com | deepseek-chat |
| 通义千问 | OpenAI Compatible | https://dashscope.aliyuncs.com/compatible-mode/v1 | qwen-plus |

### 5.2 Cline 推荐 Temperature 设置

| 任务类型 | Temperature 值 |
|----------|---------------|
| 代码生成 / 修复 0 |
| 代码重构 | 0.1 ~ 0.2 |
| 生成注释 / 文档 | 0.3 ~ 0.5 |
| 创意编程 | 0.5 ~ 0.7 |

---

## 第六章：常见问题

### Q1：Cline 提示 "API Key not configured"

**原因**：没有正确填写 API Key。

**解决**：点击 Cline 设置 ⚙️，检查 API Key 是否填入，注意不要有空格。

---

### Q2：连接超时 / Network Error

**原因**：网络无法访问 API 地址。

**解决**：
- OpenAI/Anthropic：需要能够访问国际网络
- DeepSeek/通义千问：国内直连，无需科学上网

---

### Q3：Cline 创建文件时一直转圈不响应

**原因**：可能是模型负载高或 API 配额不足。

**解决**：
1. 检查 API 账户余额是否充足
2. 切换到其他模型试试
3. 减小 Max Tokens 值（如设为 4096）

---

### Q4：Cherry Studio 测试连接失败

**原因**：API Host 或 Key 填写错误。

**解决**：
1. 仔细核对 API Host 地址，注意不要多写 `v1/` 路径
2. 检查 API Key 是否复制完整
3. 确认账户余额充足

---

### Q5：Cline 建议使用哪个模型最好？

**目前推荐排序**（2026年4月）：

| 排名 | 模型 | 适合场景 | 费用 |
|------|------|---------|------|
| 🥇 | Claude Sonnet 4 | 复杂项目、全栈开发 | 中等 |
| 🥈 | GPT-4o | 通用开发 | 中等 |
| 🥉 | DeepSeek Chat | 日常开发、入门学习 | 极低 |
| 免费 | Gemini 2.5 Pro | 体验尝鲜 | 免费 |

> 💡 **入门建议**：先用 DeepSeek（便宜、国内直连），上手后再尝试 Claude 或 GPT-4o。

---

## 附录：操作清单

完成全部配置后，逐项打钩确认：

- [ ] VS Code 已安装
- [ ] Cline 插件已安装
- [ ] Cherry Studio 已安装
- [ ] 至少一个 API Key 已获取
- [ ] Cline 中已配置 API Provider
- [ ] Cline 中测试对话成功
- [ ] Cherry Studio 中已添加模型提供商
- [ ] Cherry Studio 中对话成功

---

*祝你 AI Coding 愉快！ 🚀*
