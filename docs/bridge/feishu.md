# co-shell 飞书桥接器使用指南

## 概述

`co-shell-feishu-bridge` 是一个独立的网关程序，它将飞书（Lark）机器人连接到 co-shell。通过飞书 WebSocket 长连接，用户可以：

- 在飞书聊天中直接向 co-shell 发送自然语言指令
- 接收 co-shell 的 AI 处理结果回复
- 支持文本、文件、图片等多种消息类型
- 三种工作模式适应不同使用场景

**安全特性**：采用我方主动连接飞书 WebSocket 长链接的方式，无需暴露任何公网端口，无需配置反向代理或防火墙规则。

---

## 架构图

```
┌─────────────┐     WebSocket      ┌──────────────────┐    子进程调用    ┌──────────────┐
│  飞书服务器   │ ◄──────────────► │ co-shell-feishu- │ ◄────────────► │   co-shell   │
│  open.feishu │   长连接（我方主动） │ bridge           │   co-shell -c   │   AI Agent   │
│  .cn         │                   │                  │                 │              │
└─────────────┘                   └──────────────────┘                 └──────────────┘
```

---

## 前置条件

1. **co-shell** 已编译或已安装（版本 ≥ 0.4.0）
2. **飞书管理员账号**，用于创建和配置飞书应用
3. **Go 1.22+**（如需自行编译桥接器）

---

## 快速开始

### 1. 编译桥接器

```bash
cd /path/to/co-shell
go build -o work/co-shell-feishu-bridge ./cmd/co-shell-feishu-bridge/
```

### 2. 配置飞书应用

#### 2.1 创建飞书应用

1. 登录 [飞书开放平台](https://open.feishu.cn/app)
2. 点击「创建企业自建应用」
3. 填写应用名称（如 "co-shell 助手"）和描述
4. 创建完成后，进入应用详情页

#### 2.2 获取凭证

1. 在左侧菜单选择「凭证与基础信息」
2. 记录 **App ID** 和 **App Secret**
   - App ID 格式如：`cli_a5b3c4d5e6f7g8h9`
   - App Secret 格式如：`a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`

#### 2.3 配置事件订阅

1. 在左侧菜单选择「事件与回调」
2. 点击「添加事件」
3. 搜索并添加以下事件：
   - `im.message.receive_v1`（接收消息）
4. 在「事件订阅」页面，**事件类型**选择 `im.message.receive_v1`
5. 注意：**不需要**配置回调 URL，因为桥接器使用 WebSocket 长连接

#### 2.4 配置权限

1. 在左侧菜单选择「权限管理」
2. 添加以下权限：
   - `im:message`（获取与发送单聊、群组消息）
   - `im:message:send_as_bot`（以机器人身份发送消息）
   - `im:resource`（获取消息中的资源文件，如图片、文件）
3. 点击「批量开通」

#### 2.5 发布应用

1. 在左侧菜单选择「版本管理与发布」
2. 创建新版本，填写版本号和更新说明
3. 点击「保存」
4. 点击「申请发布」
5. 等待管理员审核通过（如果是自建应用，可自行审核）

#### 2.6 添加机器人到聊天

- **单聊**：搜索应用名称，直接发送消息
- **群聊**：在群设置中添加机器人

### 3. 启动桥接器

```bash
# 基本启动（必需参数）
./work/co-shell-feishu-bridge \
  --app-id cli_a5b3c4d5e6f7g8h9 \
  --app-secret a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

# 指定工作空间和模式
./work/co-shell-feishu-bridge \
  --app-id cli_a5b3c4d5e6f7g8h9 \
  --app-secret a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6 \
  --workspace /path/to/workspace \
  --mode pool

# 指定 co-shell 路径（如果不在 PATH 中）
./work/co-shell-feishu-bridge \
  --app-id cli_a5b3c4d5e6f7g8h9 \
  --app-secret a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6 \
  --co-shell-path /usr/local/bin/co-shell
```

首次启动后，配置会自动保存到工作空间下的 `feishu-bridge.json` 文件中，下次启动可以省略参数：

```bash
# 使用已保存的配置启动
./work/co-shell-feishu-bridge
```

### 4. 使用

启动成功后，在飞书中向机器人发送消息即可。

**示例**：

```
用户：帮我查一下当前目录有哪些文件
机器人：当前目录包含以下文件：
  - main.go
  - config.json
  - README.md
  ...

用户：写一个 Python 脚本，计算斐波那契数列前20项
机器人：已创建 fibonacci.py 文件，内容如下：
  def fibonacci(n):
      ...
```

---

## 命令行参数

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--app-id` | | 飞书应用 App ID（必填） | |
| `--app-secret` | | 飞书应用 App Secret（必填） | |
| `--co-shell-path` | | co-shell 可执行文件路径 | 从 PATH 查找 |
| `--workspace` | `-w` | co-shell 工作空间路径 | 当前目录 |
| `--config` | `-c` | co-shell 配置文件路径 | `{workspace}/config.json` |
| `--mode` | | 工作模式（sync/pool/preempt） | `sync` |
| `--log-level` | | 日志级别（debug/info/warn/error/off） | `info` |
| `--help` | `-h` | 显示帮助信息 | |
| `--version` | `-v` | 显示版本信息 | |

---

## 工作模式详解

### sync（同步模式，默认）

逐条执行消息，前一条处理完成后才处理下一条。后续消息排队等待。

```
消息A ──► 处理A ──► 回复A
                      │
消息B ────────────────► 处理B ──► 回复B
```

**适用场景**：需要严格顺序执行的场景，如文件操作、状态依赖的任务。

### pool（队列模式）

当前任务完成后，将队列中积累的所有消息合并为一条指令，批量处理。

```
消息A ──► 处理A ──► 回复A
                      │
消息B ──► 等待 ──────► 合并处理B+C ──► 回复B
消息C ──► 等待 ──────►              ──► 回复C
```

**适用场景**：快速连续提问，希望合并处理的场景。

### preempt（抢占模式）

新消息到达时，立即中断当前正在执行的任务，开始处理新消息。

```
消息A ──► 处理A（被中断）
                      │
消息B ──► 中断A ────► 处理B ──► 回复B
                      │
消息C ──► 中断B ────► 处理C ──► 回复C
```

**适用场景**：需要快速响应的场景，新指令优先级高于当前任务。

---

## 配置文件说明

桥接器涉及**两个独立的配置文件**，不要混淆：

### 1. 桥接器配置文件 `feishu-bridge.json`

保存飞书桥接器自身的参数，位于工作空间目录下：

```json
{
  "app_id": "cli_a5b3c4d5e6f7g8h9",
  "app_secret": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "co_shell_path": "/usr/local/bin/co-shell",
  "workspace": "/path/to/workspace",
  "co_shell_config_path": "/path/to/workspace/config.json",
  "mode": "sync",
  "log_level": "info"
}
```

| 字段 | 说明 |
|------|------|
| `app_id` | 飞书应用 App ID |
| `app_secret` | 飞书应用 App Secret |
| `co_shell_path` | co-shell 可执行文件路径 |
| `workspace` | co-shell 工作空间路径 |
| `co_shell_config_path` | **co-shell 的** config.json 路径 |
| `mode` | 工作模式 |
| `log_level` | 日志级别 |

下次启动时，如果命令行参数未指定，将自动从该文件加载配置。命令行参数优先级高于配置文件。

### 2. co-shell 配置文件 `config.json`

这是 co-shell 自身的配置文件（LLM 参数、MCP 服务器等），由 `--config` / `-c` 参数指定，默认路径为 `{workspace}/config.json`。桥接器通过 `--config` 参数告诉 co-shell 使用哪个配置文件，但**不会修改 co-shell 的配置内容**。

> **安全提示**：`feishu-bridge.json` 包含 App Secret 敏感信息，**不要**将其提交到版本控制中。建议将 `feishu-bridge.json` 添加到 `.gitignore`。

---

## 消息类型支持

### 文本消息

直接发送文本，co-shell 将作为自然语言指令处理。

### 文件消息

发送文件时，桥接器会自动下载文件到工作空间的 `upload/` 目录，并回复文件保存路径。之后可以发送指令让 co-shell 处理该文件。

### 图片消息

发送图片时，桥接器会自动下载图片到工作空间的 `upload/` 目录，并回复图片保存路径。

---

## 群聊使用

在群聊中使用时，需要 @机器人 才能触发处理。桥接器会自动移除 @mention 前缀。

```
@co-shell 助手 帮我查一下今天的天气
```

---

## 日志

日志输出到标准错误（stderr），可通过 `--log-level` 控制详细程度：

- `debug`：最详细，包含 WebSocket 通信细节
- `info`：常规信息（默认）
- `warn`：仅警告和错误
- `error`：仅错误
- `off`：关闭日志

---

## 故障排除

### 连接失败

```
❌ 启动失败: cannot get tenant access token: token API error: code=10003 msg=...
```

**原因**：App ID 或 App Secret 错误。

**解决**：检查飞书开放平台中的应用凭证。

### 权限不足

```
❌ 发送消息失败: send message API error: code=230001 msg=permission denied
```

**原因**：应用未获得发送消息的权限。

**解决**：在飞书开放平台中检查并添加 `im:message` 和 `im:message:send_as_bot` 权限。

### 消息未收到

1. 确认应用已发布并审核通过
2. 确认机器人已添加到聊天中
3. 检查桥接器日志是否有错误信息
4. 确认 WebSocket 连接状态（启动时显示 `✅ 已连接到飞书`）

### 自动重连

如果网络断开，桥接器会自动尝试重连，采用指数退避策略（1秒 → 2秒 → 4秒 → ... → 60秒）。重连成功后自动恢复消息处理。

---

## 安全建议

1. **App Secret 保护**：不要在版本控制中提交 `feishu-bridge.json`，建议将其添加到 `.gitignore`
2. **网络隔离**：桥接器只需要出站访问 `open.feishu.cn`，不需要入站端口
3. **权限最小化**：在飞书开放平台中仅授予必要的权限
4. **日志监控**：定期检查日志，发现异常连接或请求

---

## 与 co-shell 主程序的关系

`co-shell-feishu-bridge` 是一个独立的程序，与 co-shell 主程序分开编译和运行。它通过子进程方式调用 co-shell：

```
co-shell-feishu-bridge ──► exec co-shell -c "指令" -w 工作空间
```

这意味着：
- co-shell 的配置（config.json）完全复用
- co-shell 的所有功能（MCP、记忆、计划等）均可使用
- 桥接器本身不依赖 co-shell 的内部模块，只依赖可执行文件
