# co-shell 使用说明

> 智能命令行 Shell — 通过自然语言与 AI Agent 交互，智能编排和执行系统命令。

---

## 目录

- [快速开始](#快速开始)
- [内置命令](#内置命令)
  - [.settings — LLM 设置](#settings--llm-设置)
  - [.mcp — MCP Server 管理](#mcp--mcp-server-管理)
  - [.rule — 全局规则管理](#rule--全局规则管理)
  - [.memory — 记忆管理](#memory--记忆管理)
  - [.context — 上下文管理](#context--上下文管理)
  - [.image — 多模态图片缓存管理](#image--多模态图片缓存管理)
  - [.plan — 任务计划管理](#plan--任务计划管理)
- [自然语言交互](#自然语言交互)
- [MCP 集成](#mcp-集成)
- [配置文件](#配置文件)
- [常见问题](#常见问题)

---

## 快速开始

### 1. 启动

```bash
./co-shell
```

启动后你会看到欢迎界面：

```
╔══════════════════════════════════════╗
║         co-shell v0.1.0              ║
║   Intelligent Command-Line Shell     ║
╚══════════════════════════════════════╝

Type .help for available commands, or just type in natural language!

❯
```

### 2. 配置 API Key

首次使用需要设置 LLM API 密钥：

```bash
❯ .settings api-key sk-your-api-key-here
```

### 3. 选择模型（可选）

```bash
❯ .settings model gpt-4o          # OpenAI
❯ .settings model deepseek-chat   # DeepSeek
❯ .settings model qwen-plus       # 通义千问
```

### 4. 设置 API Endpoint（可选，默认 OpenAI）

```bash
# DeepSeek
❯ .settings endpoint https://api.deepseek.com/v1

# 通义千问
❯ .settings endpoint https://dashscope.aliyuncs.com/compatible-mode/v1

# 本地 Ollama
❯ .settings endpoint http://localhost:11434/v1
❯ .settings model llama3
```

### 5. 开始使用

```bash
❯ 列出当前目录下所有文件
❯ 查找所有大于 100MB 的文件
❯ 帮我创建一个新的 Go 项目
```

---

## 内置命令

所有内置命令以 `.` 开头，支持 Tab 自动补全。

### .settings — LLM 设置

管理 LLM API 参数。

```bash
.settings                          # 查看当前所有设置
.settings api-key <key>            # 设置 API Key
.settings endpoint <url>           # 设置 API Endpoint URL
.settings model <model>            # 设置模型名称
.settings temperature <value>      # 设置温度 (0.0-2.0)
.settings max-tokens <count>       # 设置最大 Token 数 (1-128000)
```

**示例：**

```bash
❯ .settings
Current Settings:
  API Key:      sk-****abcd
  Endpoint:     https://api.openai.com/v1
  Model:        gpt-4o
  Temperature:  0.7
  Max Tokens:   4096

❯ .settings temperature 0.3
✅ Temperature set to 0.3

❯ .settings model deepseek-chat
✅ Model updated
```

### .mcp — MCP Server 管理

管理 MCP (Model Context Protocol) 服务器连接。

```bash
.mcp                              # 列出所有已连接的 MCP Server
.mcp add <name> <cmd> [args...]   # 添加并连接一个新的 MCP Server
.mcp remove <name>                # 断开并移除一个 MCP Server
.mcp list                         # 列出所有 MCP Server 及其工具
.mcp enable <name>                # 启用一个 MCP Server
.mcp disable <name>               # 禁用一个 MCP Server
```

**示例：**

```bash
# 添加文件系统 MCP Server
❯ .mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp
✅ MCP server "filesystem" added and connected

# 查看所有 MCP Server 及其工具
❯ .mcp list
MCP Servers:

  📡 filesystem
    Tools:
      • read_file - Read the complete contents of a file
      • write_file - Write text content to a file
      • list_directory - List the contents of a directory
      • search_files - Search for files matching a pattern

# 移除 MCP Server
❯ .mcp remove filesystem
✅ MCP server "filesystem" removed
```

### .rule — 全局规则管理

设置 AI Agent 的行为规则，规则会自动注入到 System Prompt 中。

```bash
.rule                    # 列出所有规则
.rule add <text>         # 添加一条新规则
.rule remove <index>     # 按索引删除规则
.rule clear              # 清除所有规则
```

**示例：**

```bash
❯ .rule add "Always confirm before deleting files"
✅ Rule added: Always confirm before deleting files

❯ .rule add "Use English for all responses"
✅ Rule added: Use English for all responses

❯ .rule
Global Rules:
  [0] Always confirm before deleting files
  [1] Use English for all responses

❯ .rule remove 0
✅ Rule removed: Always confirm before deleting files
```

### .memory — 记忆管理

持久化存储键值对记忆，Agent 在对话中可以读取这些记忆。

```bash
.memory                    # 列出所有记忆
.memory save <key> <value> # 保存一条记忆
.memory get <key>          # 获取一条记忆
.memory search <query>     # 按前缀搜索记忆
.memory delete <key>       # 删除一条记忆
.memory clear              # 清除所有记忆
```

**示例：**

```bash
❯ .memory save language zh-CN
✅ Memory saved: language = zh-CN

❯ .memory save preference "Always use verbose output"
✅ Memory saved: preference = Always use verbose output

❯ .memory search language
Memory entries matching "language":
  language = zh-CN

❯ .memory
Memory:
  language = zh-CN
  preference = Always use verbose output
```

### .context — 上下文管理

管理当前会话的上下文变量。

```bash
.context                  # 查看当前上下文摘要
.context show             # 查看详细上下文
.context reset            # 重置上下文（清除对话历史）
.context set <k> <v>      # 设置上下文变量
```

**示例：**

```bash
❯ .context set mode expert
✅ Context set: mode = expert

❯ .context show
Current Context:
  mode: expert

❯ .context reset
✅ Context reset. Conversation history cleared.
```

### .image — 多模态图片缓存管理

管理用于多模态输入的图片缓存。

```bash
.image                    # 列出所有缓存的图片
.image add <path>         # 添加一张图片到缓存
.image remove <index>     # 按索引移除一张图片
.image clear              # 清除所有缓存的图片
.image list               # 列出所有缓存的图片
```

**示例：**

```bash
❯ .image add /path/to/photo.jpg
✅ Image added: /path/to/photo.jpg

❯ .image
Cached Images:
  [0] /path/to/photo.jpg
  [1] /path/to/diagram.png

❯ .image remove 0
✅ Image removed: /path/to/photo.jpg
```

### .plan — 任务计划管理

管理多步骤任务计划，支持创建、查看、修改和跟踪进度。

```bash
.plan                     # 列出所有任务计划
.plan list                # 列出所有任务计划
.plan view <id>           # 查看指定任务计划的详细内容
.plan create <title>      # 创建一个新的任务计划
.plan insert <id> <pos>   # 在指定位置插入新步骤
.plan remove <id> <step>  # 移除指定步骤
.plan update <id> <step>  # 更新指定步骤的状态或内容
```

**示例：**

```bash
❯ .plan create "部署新版本"
✅ Task plan created: id=plan-001, title=部署新版本

❯ .plan view plan-001
Task Plan: 部署新版本 (plan-001)
  Status: 进行中
  Steps:
    [0] ⏳ 拉取最新代码
    [1] ⏳ 运行测试
    [2] ⏳ 构建镜像
    [3] ⏳ 部署到服务器

❯ .plan update plan-001 0
✅ Step 0 marked as completed
```

---

## 自然语言交互

co-shell 的核心功能：直接用自然语言描述你想做的事情。

### 文件操作

```bash
❯ 列出当前目录下所有 Go 文件
❯ 查找包含 "TODO" 的所有文件
❯ 把 src/ 目录下所有 .log 文件打包成 logs.tar.gz
❯ 统计项目中的代码行数
```

### 系统管理

```bash
❯ 看看磁盘还剩多少空间
❯ 当前系统负载怎么样
❯ 帮我杀掉占用 8080 端口的进程
❯ 查看系统运行了多久
```

### 开发辅助

```bash
❯ 帮我创建一个新的 Go 模块，包含一个 HTTP 服务器
❯ 查找所有未使用的导入
❯ 帮我格式化当前目录下所有 Go 文件
❯ 运行测试并只看失败的用例
```

### 组合任务

```bash
❯ 先备份 /var/log 目录，然后清理 7 天前的日志
❯ 找出最大的 5 个文件，然后按大小排序显示
❯ 检查 80 和 443 端口是否在监听，如果没有就启动 nginx
```

---

## MCP 集成

MCP (Model Context Protocol) 让 co-shell 可以接入各种外部工具生态。

### 常用 MCP Server 示例

```bash
# 文件系统操作
❯ .mcp add fs npx @modelcontextprotocol/server-filesystem /path/to/project

# GitHub 集成
❯ .mcp add github npx @modelcontextprotocol/server-github

# 数据库查询
❯ .mcp add db npx @modelcontextprotocol/server-sqlite ./data.db

# 自定义 MCP Server
❯ .mcp add my-tool /path/to/my-mcp-server --flag value
```

### 连接后使用

MCP 工具会自动注册为 Agent 可调用的工具，你只需用自然语言描述即可：

```bash
❯ 读取 /tmp/test.txt 的内容
# → Agent 会自动调用 filesystem 的 read_file 工具

❯ 在 GitHub 上搜索 co-shell 相关的仓库
# → Agent 会自动调用 github 的 search_repositories 工具
```

---

## 配置文件

配置文件存储在 `~/.co-shell/config.json`，格式如下：

```json
{
  "llm": {
    "api_key": "sk-xxx",
    "endpoint": "https://api.openai.com/v1",
    "model": "gpt-4o",
    "temperature": 0.7,
    "max_tokens": 4096
  },
  "mcp": {
    "servers": [
      {
        "name": "filesystem",
        "command": "npx",
        "args": ["@modelcontextprotocol/server-filesystem", "/tmp"],
        "enabled": true
      }
    ]
  },
  "rules": [
    "Always confirm before deleting files"
  ]
}
```

数据库文件存储在 `~/.co-shell/co-shell.db`（bbolt 嵌入式 KV 数据库）。

---

## 常见问题

### Q: 启动后提示 "LLM not configured"？

A: 需要先设置 API Key：

```bash
❯ .settings api-key sk-your-key
```

### Q: 支持哪些模型？

A: 所有兼容 OpenAI API 格式的模型都支持，包括：

- OpenAI: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`
- DeepSeek: `deepseek-chat`, `deepseek-reasoner`
- 通义千问: `qwen-plus`, `qwen-max`
- 本地模型: 通过 Ollama (`llama3`, `qwen2` 等)
- Anthropic: 通过 API 转换网关

### Q: MCP Server 连接失败？

A: 确保：
1. MCP Server 的命令和参数正确
2. 所需依赖已安装（如 `npx` 需要 Node.js）
3. 路径和权限正确

### Q: 如何退出？

```bash
❯ exit
❯ quit
❯ .exit
❯ .quit
# 或按 Ctrl+C
```

### Q: 如何清除所有数据？

```bash
rm -rf ~/.co-shell
```
