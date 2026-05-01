# 五一福利来了！co-shell送给全体劳动者的生产力工具

> **10MB、零依赖、多平台、国产大模型全支持——这个 AI Agent 有点东西。**

---

五一劳动节，致敬每一位用代码改变世界的劳动者。

今天不送红包、不送优惠券，送一个真正能替你干活的工具——

**co-shell：全世界最小、最努力的 AI 命令行智能体。**

完全免费、完全开源（MIT 协议），下载即用。

---

## 一、先看效果：它真的能干活

在介绍功能之前，先看看 co-shell 实际产出的过程和成果。

<movie>

以下报告全部由 co-shell **自主完成**——从联网搜索、资料收集、数据整理到报告撰写，全程无需人工干预。

### 报告一：北冰洋品牌深度调研报告

> **14页** | 从1936年北平制冰厂到今天的品牌复兴，涵盖品牌沿革、产权变动、产品体系、市场竞争等全方位分析

**报告摘要：** 北冰洋品牌发轫于1936年成立的北平制冰厂，历经近一个世纪的风雨沉浮。报告追溯了其从制冰厂到民族饮料品牌象征的完整历程，深入分析了"水淹七军"事件、与百事可乐的合资博弈、以及2011年后的品牌复兴战略。

> [查看完整报告（Markdown）](https://github.com/idirect3d/co-shell/blob/main/samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.md)
> [下载 Word 文档（DOCX）](https://github.com/idirect3d/co-shell/blob/main/samples/research/arctic-ocean-brand-research/arctic-ocean-brand-research-report.docx)

### 报告二：北京动物园大熊猫最新动态研究报告

> **13页** | 覆盖2023-2026年大熊猫馆升级、新熊猫入驻、萌兰辟谣等最新动态

**报告摘要：** 系统梳理了北京动物园大熊猫群体近年来的重大变化——大熊猫馆设施升级、明星熊猫"古古"去世、新熊猫"吉年"与"福将"抵京、萌兰出国传闻辟谣、旅美熊猫"丫丫"回国历程等。

> [查看完整报告（Markdown）](https://github.com/idirect3d/co-shell/blob/main/samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.md)
> [下载 Word 文档（DOCX）](https://github.com/idirect3d/co-shell/blob/main/samples/research/beijing-zoo-panda-news/beijing-zoo-panda-latest-report.docx)

### 报告三：中国再保险市场2026-2027年调研报告

> **26页** | 专业级市场分析，评估伊朗战争对中国再保险市场的冲击

**报告摘要：** 基于对维基百科等公开来源的系统性资料收集，全面评估2026年伊朗战争对中国再保险市场的影响。涵盖霍尔木兹海峡危机、全球能源市场动荡、战争风险溢价飙升、制裁合规风险等深度分析。

> [查看完整报告（Markdown）](https://github.com/idirect3d/co-shell/blob/main/samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.md)
> [下载 Word 文档（DOCX）](https://github.com/idirect3d/co-shell/blob/main/samples/research/china-reinsurance-market-2026-2027/china-reinsurance-market-report-2026-2027.docx)

### 报告四：中国运-10飞机深度调研报告

> **13页** | 中国首架自主研制喷气式客机的完整技术档案

**报告摘要：** 深入研究了运-10（代号"708工程"）从1970年立项到1986年终止的完整历程，涵盖历史背景、技术参数、试飞数据、项目终止原因分析及其对后续ARJ21、C919等国产大飞机项目的先导意义。

> [查看完整报告（Markdown）](https://github.com/idirect3d/co-shell/blob/main/samples/research/yun10/Y-10-in-depth-research-report.md)
> [下载 Word 文档（DOCX）](https://github.com/idirect3d/co-shell/blob/main/samples/research/yun10/Y-10-in-depth-research-report.docx)

> **提示：** 这些报告不仅生成 Markdown 版本，还能自动转换为符合 GB/T 9704-2012 国家公文格式标准的 Word 文档——小标宋标题、黑体/楷体/仿宋层级标题、固定28磅行距、首行缩进2字符，直接可用。

---

## 二、它到底有多小？——10MB 的降维打击

先看一组数据：

| 框架 | 大小 | 安装复杂度 |
|------|------|-----------|
| **OpenHands** | **~100MB+**（含 Docker） | 装 Docker → 拉镜像 → 配环境 |
| **OpenClaw** | **~80MB+**（含 node_modules） | 装 Node.js → npm install 半天 |
| **LangChain** | **~50MB+**（含依赖） | 装 Python → 建虚拟环境 → pip install 150+ 包 |
| **CrewAI** | **~30MB+**（含依赖） | 装 Python → pip install → 可能冲突 |
| **Hermes** | **~200MB+**（含模型） | 装 Python → 下载模型 → 配 CUDA |
| **co-shell** | **~10MB** | **下载 → chmod +x → 开用** |

**10MB 是什么概念？**

- **比一张高清照片还小。** iPhone 拍的 HEIC 照片大约 5~8MB，co-shell 才比它大一点点。
- **可以放在 U 盘里随身携带。** 一个 32GB 的 U 盘，能装 3,200 个 co-shell。
- **从点击下载到开始使用，不超过 60 秒。** 泡面的水还没烧开，你的 AI Agent 已经就位了。
- **树莓派、老旧笔记本、云服务器上都能跑得飞起。** 不挑食，不娇气。

这不是「轻量级」的修辞，这是**真正的降维打击**。

---

## 三、为什么「零依赖」是开发者最大的福音？

你经历过「依赖地狱」吗？

```
pip install something
→ 依赖 A v2.3，但已安装 A v1.8
→ 依赖 B 需要 Python 3.10，但你用的是 3.12
→ 依赖 C 和依赖 D 冲突
→ ... 一天过去了，还没装好
```

**co-shell：一个二进制，零依赖。**

```
不需要 Node.js
不需要 Docker
不需要 JVM
不需要 npm/brew/apt
不需要虚拟环境
不需要容器运行时

只需要一个操作系统
和一颗想偷懒的心
```

**零依赖的真正意义**不是「省了硬盘空间」，而是：

- **永远不会出现版本冲突** — 没有依赖可以冲突
- **永远不会因为升级而 break** — 没有依赖可以 break 你
- **永远不会出现「在我电脑上能跑啊」** — 因为每台电脑都一样
- **永远不会被供应链攻击** — 没有第三方依赖可以被投毒

**一个二进制，就是最极致的「纯天然」。**

当然，如果Agent运行时确实需要上面这些工具，那么他会自己想办法安装，不需要提前准备。

---

## 四、一个二进制，跑遍天下

开发者最痛的是什么？**环境不一致。**

你在 macOS 上写得开开心心，部署到 Linux 服务器上炸了——Python 版本不对、某个包没装、动态链接库找不到……

**co-shell 不存在这个问题。**

```
macOS（Intel）—— 你的 MacBook Pro
macOS（Apple Silicon）—— 你的 MacBook Air M 芯片
Linux（amd64）—— 你的阿里云/腾讯云/AWS 服务器
Linux（arm64）—— 你的树莓派、Oracle 免费云
Windows（x86_64）—— 你的公司电脑
Windows（ARM64）—— 你的 Surface Pro X
```

**一个二进制 = 全平台覆盖。**

不用为每个平台单独编译，不用纠结「这个工具 Linux 能不能用」，不用在 Windows 上装 WSL 就为了跑一个命令行工具。

从你的 MacBook 到公司的 Windows 台式机，从家里的树莓派 NAS 到云上的 Linux 服务器——**只要选择一个平台支持的文件就全部搞定了。**

---

## 五、最努力的 Agent——它真的在干活

别人有的它有，别人没有的它也有。co-shell 内置 **7 大类工具** + **MCP 协议**接入整个 AI 工具生态：

| 能力 | 说明 |
|------|------|
| **文件操作** | 读、写、搜索、替换，一套带走 |
| **命令执行** | 智能识别系统命令，超时控制，安全确认 |
| **代码搜索** | 正则匹配 + 结构分析，源码级理解 |
| **子 Agent** | 进程级隔离，独立执行，安全可靠 |
| **定时任务** | 内置 cron 调度器，自然语言设置提醒 |
| **任务计划** | 唯一 Checklist 单例模式，完成自动归档 |
| **持久化记忆** | 语义搜索 + 历史切片，越用越聪明 |
| **记忆搜索** | 关键词过滤、时间范围、发言人筛选，精准回溯 |
| **多模态理解** | 图片识别、视觉理解（UI 设计图、报错截图、图表） |
| **国际化** | 中英文自动切换 |
| **MCP 协议** | 接入整个 AI 工具生态 |

**它不摸鱼，不请假，不喊累。** 你不喊停，它绝不罢工。对，那个叫龙虾的，就是说你呢！

### 任务编排：把复杂任务拆成清单，逐个击破

面对一个复杂任务，co-shell 不会盲目行动。它会自动将任务拆解为可执行的步骤清单，然后按顺序逐一完成。

```
任务：调研北冰洋品牌并生成报告
├── 步骤 1：搜索北冰洋品牌历史资料
├── 步骤 2：分析产权变动与市场竞争格局
├── 步骤 3：整理品牌复兴战略
├── 步骤 4：撰写完整调研报告
└── 步骤 5：转换为 Word 文档
```

每一步完成后自动标记进度，遇到问题可以动态调整步骤顺序、插入新步骤或移除不必要的步骤。整个执行过程透明可见，你可以随时查看当前进展到哪一步。

### 长期记忆：越用越聪明，不会"转头就忘"

传统 AI 助手最大的痛点是什么？**每次对话都是全新的开始，它记不住你。**

co-shell 内置持久化记忆系统，所有对话历史自动保存到本地数据库。你可以随时回溯历史对话，也可以通过关键词搜索找到之前的任何讨论内容。

```bash
# 搜索记忆：查找之前讨论过的某个话题
"搜索记忆，关键词：北冰洋、品牌战略"

# 查看最近对话历史
"查看最近 10 条对话记录"

# 按时间范围筛选
"搜索 4 月份关于市场分析的讨论"
```

记忆支持多维检索——按关键词、按时间范围、按发言人筛选，精准定位你需要的信息。**它不会"转头就忘"，每一次对话都在为下一次积累经验。**

---

## 六、安全可控——劳动者的靠谱伙伴

AI 再强，安全第一。

co-shell 的**四级命令确认机制**：

```
Enter        → 执行（确认安全）
C            → 取消（反悔了）
A            → 全部批准（批量执行）
任意输入     → 修改命令后重新评估（二次确认）
```

再加上Ctrl+C终止一切，没有后台黑盒，没有偷摸的sub-agent，运行内容全透明。

还有**可审计日志**——每一步操作都有日志，出了问题可以追溯。

**强大，但不失控。这就是劳动者的工具哲学。**

---

## 七、支持国产大模型——中国人的 Agent 当然要支持中国模型

co-shell 通过灵活的 LLM 配置，**全面支持国产大模型**：

| 模型 | 一句话评价 |
|------|-----------|
| **DeepSeek**（深度求索） | 国产之光，推理能力一流 |
| **通义千问**（阿里 Qwen） | 中文理解最好的模型之一 |
| **Kimi**（月之暗面） | 超长上下文，文档分析利器 |
| **GLM**（智谱 AI） | 开源生态最完善的国产模型 |
| **MiniMax** | 对话体验流畅 |
| **零一万物**（Yi） | 性价比高 |

**怎么切换？一句话的事。**

```bash
# 用 DeepSeek 干活
./co-shell --model deepseek-chat

# 用通义千问
./co-shell --model qwen-max

# 用 Kimi 分析长文档
./co-shell --model moonshot-v1
```

**不绑定任何厂商，想用哪个用哪个。**

---

## 八、可以被灵活调用——不是封闭花园，是开放桥梁

co-shell 原生支持 **MCP 协议（Model Context Protocol）**——AI 工具领域的 USB 接口标准。

这意味着你可以：

**嵌入 CI/CD 流水线**
```bash
# 在 GitHub Actions 里直接调用
./co-shell "分析这次代码变更的风险"
```

**嵌入 IDE**
```bash
# 在 VSCode 终端里直接使用
./co-shell "帮我重构这个函数，保持接口不变"
```

**嵌入监控系统**
```bash
# 定时任务自动执行
./co-shell "每天凌晨2点检查服务器日志，发现异常就报警"
```

**被其他程序调用**
```python
# 你的 Python 脚本里调用 co-shell
result = subprocess.run(['./co-shell', '分析这份数据'], capture_output=True)
```

**通过 MCP 接入整个 AI 工具生态**
```bash
# 接入任意 MCP 服务器
./co-shell --mcp-server "github.com/example/awesome-mcp"
```

它不是一座孤岛，而是一座桥梁。**你想把它嵌入哪里，它就能嵌入哪里。**

---

## 九、多模态？安排上了（需模型支持）

文字理解是基本功，但 co-shell 还能看懂图片。

```bash
# 分析一张设计图
./co-shell "帮我看看这张 UI 设计图，用 HTML+CSS 实现它"

# 识别截图中的错误信息
./co-shell "这个报错是什么意思？怎么修复？"

# 分析图表数据
./co-shell "根据这张销售趋势图，给我写一份总结报告"
```

不是简单的 OCR，而是真正理解图片内容。设计的草图、报错截图、数据图表、手写笔记——**它都能看懂。**

---

## 十、怎么用？60 秒上手

### 下载

从 GitHub Releases 页面下载对应平台的压缩包：

**https://github.com/idirect3d/co-shell/releases**

| 平台 | 架构 | 下载链接 |
|------|------|---------|
| macOS | Intel | [co-shell-v0.3.0-darwin-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-amd64.zip) |
| macOS | Apple Silicon | [co-shell-v0.3.0-darwin-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip) |
| Linux | x86_64 | [co-shell-v0.3.0-linux-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-amd64.zip) |
| Linux | ARM64 | [co-shell-v0.3.0-linux-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-linux-arm64.zip) |
| Windows | x86_64 | [co-shell-v0.3.0-windows-amd64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip) |
| Windows | ARM64 | [co-shell-v0.3.0-windows-arm64.zip](https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-arm64.zip) |

### 安装（以 macOS Apple Silicon 为例）

```bash
# 下载
curl -L -o co-shell.zip https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-darwin-arm64.zip

# 解压
unzip co-shell.zip && rm co-shell.zip

# 启动
./co-shell
```

**Windows 用户：**
```powershell
# PowerShell
Invoke-WebRequest -Uri https://github.com/idirect3d/co-shell/releases/download/v0.3.0/co-shell-v0.3.0-windows-amd64.zip -OutFile co-shell.zip
Expand-Archive -Path co-shell.zip -DestinationPath .
.\co-shell.exe
```

### 配置 API Key

首次启动会自动弹出设置向导，选择你的模型提供商，输入 API Key 即可。

也可以手动配置：

```bash
.set api-key sk-your-api-key-here
.set endpoint https://api.deepseek.com/v1
.set model deepseek-chat
```

### 开始使用

```bash
列出当前目录所有文件
查找所有大于 100MB 的文件
为我的电脑出一份安全体检报告
帮我创建一个新的 Go 项目
搜索所有包含 "TODO" 的代码文件
分析这份日志，找出异常
帮我分析伊朗战事对俄罗斯农业产品市场的影响
```

---

## 写在最后

五一一整天，你的代码不会自己写完，你的服务器不会自己维护，你的日志不会自己分析——

**但 co-shell 可以帮你。**

10MB，零依赖，多平台，国产大模型全支持，四级安全确认，三层超时控制，MCP 协议开放，多模态理解……

**该有的全都有，不该有的（依赖）一个没有。**

**GitHub：https://github.com/idirect3d/co-shell**
**下载：https://github.com/idirect3d/co-shell/releases**
**点亮星星，见证最小 Agent 的成长之路。**

---

> *谨以此文，致敬每一位用代码改变世界的劳动者。五一快乐！*

---

*以上内容基于 co-shell v0.3.0 RC1 实际数据撰写*
*报告样本均为 co-shell 自主生成，未经人工修改*

#五一福利 #开源 #AIAgent #开发者工具 #co-shell #命令行 #送给劳动者的礼物 #MakeTerminalSmartAgain #国产大模型 #DeepSeek
