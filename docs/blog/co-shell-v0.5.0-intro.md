# 一个命令，无限可能——co-shell v0.5.0 正式发布

> 10MB 的智能命令行 Shell，让你的电脑听懂人话。

## 前言

还记得那些记不住的命令参数吗？

```
find . -type f -name "*.go" | xargs wc -l | tail -1
```

如果换成这样呢？

```
❯ 统计一下这个项目里所有 Go 代码的行数
```

这就是 co-shell——一个只有 10MB、**零外部依赖**的命令行工具，但它能让你的电脑**听懂人话**。

### 安装有多简单？

**下载 → 解压 → 运行。三步走完，不需要任何依赖。**

没有 Python、没有 Node.js、没有 Docker、没有 Java 运行时——连 Go 运行时都不需要。一个独立的可执行文件，下载到任何目录就能直接运行。

**macOS Apple Silicon 用户：**
```bash
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-0.5.0-Beta2-darwin-arm64.zip
unzip co-shell.zip && rm co-shell.zip
./co-shell
```

**Windows x86 用户：**
```powershell
Invoke-WebRequest -Uri https://github.com/idirect3d/co-shell/releases/download/v0.5.0-Beta2/co-shell-0.5.0-Beta2-windows-amd64.zip -OutFile co-shell.zip
Expand-Archive -Path co-shell.zip -DestinationPath .
.\co-shell.exe
```

> 其他平台（macOS Intel、Linux x86/ARM、Windows ARM）的下载链接请点击文末「阅读原文」查看。

## 什么是 co-shell？

简单说，co-shell 是一个**智能命令行 Shell**。你只需要用自然语言描述你想做什么，它就会自动理解、编排并执行相应的系统命令。

但它远不止是一个"命令翻译器"。

## v0.5.0 亮点速览

### 🎯 双模式工具调用

这是本次版本最大的亮点。co-shell 同时支持两种工具调用模式：

- **OpenAI 标准模式**：对于 DeepSeek-V4 等支持良好的模型，使用原生 Function Calling，效率最高
- **XML 模式**：对于 Qwen 3.x 等支持不稳定的模型，使用 XML 标签格式，兼容性最好

两种模式可以在运行时通过 `.set tool mode openai|xml` 随时切换，无需重启。

> 为什么需要两种模式？因为现实很骨感——DeepSeek-V4 表现良好，Qwen 3.x 复杂场景频繁调用失败甚至死循环，Mimo 直接罢工。XML 模式通过精心设计的提示词和容错解析，让所有模型都能稳定使用工具。

### 🧠 多模型管理

不再局限于一个模型。co-shell 支持配置多个模型参数并快速切换：

- `.model add` — 添加新模型（内置 DeepSeek、Qwen、GLM、Ollama 等模板）
- `.model switch` — 运行时切换模型
- `.model test` — 自动检测模型的视觉、工具调用、思考能力
- 系统根据任务能力自动选择优先级最高的模型

### 💾 双存储后端

支持两种存储方式：

- **bbolt**（嵌入式，零配置，开箱即用）
- **PostgreSQL**（集中记忆管理，多设备共享）

通过 `.set db` 命令配置，支持连接测试和数据迁移。

### 🔄 会话持久化

程序中断重启后，对话上下文自动恢复。再也不用担心意外退出丢失对话了。

### 🛡️ 工具调用确认

所有工具调用（不限于命令执行）都支持用户确认：

- 每个工具可独立控制是否需要确认
- 支持数字计数器（批准后面 N 次）
- 支持 G 选项（同意并取消此工具后续确认）

### 🔁 循环检测与去重

针对 LLM 输出死循环问题，co-shell 内置了两道防线：

- **流式循环检测**：监控输出中的重复模式，达到阈值自动纠正
- **消息级去重**：特征词匹配 + Jaccard 相似度，连续重复 3 次发送警告

### 🌐 飞书集成

通过 co-shell-feishu-bridge 独立程序，你可以在飞书上跟机器人对话，远程操控 co-shell。

### 🎨 更多细节

- 思考/推理模式开关
- 输出控制开关（thinking / tool / command 独立控制）
- 上下文起始模式（window / task / smart）
- 模型参数模板（内置各厂商推荐参数）

## 适用场景

- **开发者**：代码分析、文件操作、项目重构
- **运维人员**：系统管理、日志分析、批量操作
- **普通用户**：文件整理、格式转换、日常自动化
- **团队协作**：通过飞书机器人共享 co-shell 能力

## 结语

co-shell 的核心理念很简单：**你的想象力就是它的燃料，命令就是一切。**

它不是一个臃肿的 AI 平台，不是一个需要复杂部署的云服务——它只是一个命令，一个 10MB 的可执行文件，下载就能用。

但它能调用几乎任何命令，编排几乎任何任务，连接几乎任何工具。

> 一个命令，无限可能。

---

**项目地址**: https://github.com/idirect3d/co-shell

**下载地址**: https://github.com/idirect3d/co-shell/releases/tag/v0.5.0-Beta2（点击「阅读原文」查看所有平台下载链接）

*支持 macOS、Linux、Windows 全平台，x86 和 ARM 架构全覆盖。*
