# co-shell 版本计划

> 版本号格式：`v{major}.{minor}.{patch}`

---

## 当前版本

> **版本**: v0.1.0 — Alpha
> **BUILD**: 00035
> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XXXXX]` 标记完成时的编译版本。

---

## v0.1.0 — Alpha（已完成）

> **状态**: ✅ 已完成
> **目标日期**: 2026-04-25

### 功能清单

- [x] FEATURE-00001 REPL 交互界面（go-prompt，Tab 补全）[BUILD-00001]
- [x] FEATURE-00002 LLM 客户端抽象（OpenAI 兼容 API）[BUILD-00001]
- [x] FEATURE-00003 Agent 核心循环（LLM 调用 → 工具执行 → 迭代）[BUILD-00001]
- [x] FEATURE-00004 内置命令系统（.settings / .mcp / .rule / .memory / .context）[BUILD-00001]
- [x] FEATURE-00005 持久化存储（bbolt 记忆/上下文）[BUILD-00001]
- [x] FEATURE-00006 MCP 客户端管理器（多 Server 连接）[BUILD-00001]
- [x] FEATURE-00007 系统命令执行（超时控制）[BUILD-00001]
- [x] FEATURE-00008 配置管理（JSON 持久化到 ~/.co-shell/）[BUILD-00001]
- [x] FEATURE-00009 API初始设置（默认设置为deepseek，但Key留空）[BUILD-00001]
- [x] FEATURE-00010 API设置向导（co-shell启动后当系统大模型API参数不完整时，提示用户输入不完整的参数，比如默认deepseek配置不带key，那么就先提示用户输入正确的key并需要至测试成功为止）[BUILD-00001]
- [x] FEATURE-00011 系统命令直接运行（如果用户直接输入系统命令或执行程序在当前环境下可以直接执行，则直接执行用户输入的所有内容，而不用通过大模型解释。）[BUILD-00001]
- [x] FEATURE-00012 流式输出支持 [BUILD-00001]
- [x] FEATURE-00013 日志系统（文件日志，支持运行时开关）[BUILD-00024]
- [x] FEATURE-00014 API Key 脱敏显示 [BUILD-00024]
- [x] FEATURE-00015 命令行参数支持（--help/--version/--model/--endpoint/--api-key/--log）[BUILD-00024]
- [x] FEATURE-00016 命令行指令支持（-c/--cmd 执行单条自然语言或系统指令后退出）[BUILD-00024]
- [x] FEATURE-00018 会话历史管理（用户可以通过上、下键在自己输入的历史内容之间翻页，包括上一次执行co-shell时的内容）[BUILD-00025]
- [x] FEATURE-00019 基础错误处理和用户提示 [BUILD-00025]
- [x] FEATURE-00020 最大迭代次数可配置（--max-iterations 命令行参数、.settings max-iterations 运行时设置、config.json 持久化）[BUILD-00027]
- [x] FEATURE-00021 多配置文件位置支持（优先级：命令行参数指定 > 当前目录 config.json > ~/.co-shell/config.json）[BUILD-00028]
- [x] FEATURE-00022 多供应商支持（DeepSeek v4 / 阿里千问 / OpenAI 兼容兜底），设置向导支持供应商选择、自动打开官网获取 API Key [BUILD-00029]
- [x] ENHANCEMENT-00023 设置向导增强：Tab 键显示可选列表、上下键选择、ESC 退出、连接测试 [BUILD-00031]
- [x] ENHANCEMENT-00024 设置向导增强：OpenAI 兼容模式下输入端点后自动测试连通性，输入 API Key 后自动获取模型列表 [BUILD-00032]
- [x] FEATURE-00047 国际化（i18n）支持中文/英文，--lang 命令行参数，自动检测系统语言 [BUILD-00033]


## v0.2.0 — Beta

> **状态**: 🚧 开发中
> **目标日期**: 2026-04-27
> **里程碑**: 功能完善，可日常使用

### 功能清单

- [x] FEATURE-00051 多平台支持（macOS/Linux/Windows）[BUILD-00035]
- [x] FEATURE-00052 首次运行风险声明 [BUILD-00036]
- [ ] FEATURE-00053 命令执行确认机制（执行命令前等待用户确认：批准/拒绝/修改后重新评估），由配置控制开关

- [ ] FEATURE-00025 会话历史管理（保存/恢复对话）


- [ ] FEATURE-00026 多轮对话上下文管理
- [ ] FEATURE-00027 系统命令执行安全沙箱
- [ ] FEATURE-00028 命令执行确认机制（危险操作）
- [ ] ENHANCEMENT-00029 更好的错误处理和用户提示
- [ ] FEATURE-00030 配置文件热重载
- [ ] FEATURE-00031 MCP Server 自动重连

### 技术债务

- [ ] ENHANCEMENT-00032 单元测试覆盖核心逻辑（Agent Loop、命令解析）

---

## v0.3.0 — RC1

> **状态**: 🚧 开发中
> **目标日期**: 2026-04-29
> **里程碑**: 功能完整，可发布预览

### 功能清单

- [ ] FEATURE-00034 插件系统（WASM 插件支持）
- [ ] FEATURE-00035 自定义 Prompt 模板
- [ ] FEATURE-00036 多会话管理（Tab 切换）
- [ ] FEATURE-00037 输出格式化（JSON/表格/树形）
- [ ] FEATURE-00038 命令别名
- [ ] FEATURE-00039 批量命令执行
- [ ] FEATURE-00040 管道支持（Pipe）

### 优化

- [ ] ENHANCEMENT-00041 启动速度优化
- [ ] ENHANCEMENT-00042 内存使用优化
- [ ] ENHANCEMENT-00043 大模型响应缓存

---

## v1.0.0 — 正式版

> **状态**: 🚧 开发中
> **目标日期**: 2026-05-02
> **里程碑**: 稳定可用，可发布

### 功能清单

- [ ] FEATURE-00044 Homebrew 安装支持
- [ ] FEATURE-00045 自动更新机制
- [ ] FEATURE-00046 多平台发布（macOS/Linux/Windows）
- [ ] FEATURE-00048 主题系统
- [ ] ENHANCEMENT-00049 性能基准测试
- [ ] FEATURE-00050 完整文档站


## v1.1.0 — 增强版

> **状态**: 💡 构想中
> **里程碑**: 生态建设

### 功能清单

- [ ] FEATURE-00056 MCP Hub 集成（发现和安装 MCP Server）
- [ ] FEATURE-00057 社区插件市场
- [ ] FEATURE-00058 多 Agent 协作
- [ ] FEATURE-00059 可视化工作流编排
- [ ] FEATURE-00060 远程执行（SSH）


## 版本发布记录

| 版本 | 日期 | 状态 | 说明 |
|---|---|---|---|
| v0.1.0 | 2026-04-25 | ✅ 已完成 | Alpha 预览版 |
| v0.2.0 | 2026-04-27 | 🚧 开发中 | Beta 测试版 |
| v0.3.0 | 2026-04-29 | 🚧 开发中 | 发布候选版 |
| v1.0.0 | 2026-05-02 | 🚧 开发中 | 正式版 |

### 发布条件

每个版本发布前需满足以下条件：

- [ ] `go build ./...` 编译通过
- [ ] `go vet ./...` 无警告
- [ ] 核心功能手动测试通过
- [ ] USAGE.md 使用文档完整
- [ ] CHANGELOG.md 更新

---

## 版本命名规范

```
v{major}.{minor}.{patch}
  │       │       └── 补丁版本：Bug 修复、小改进
  │       └────────── 次版本：新功能、非破坏性变更
  └────────────────── 主版本：重大变更、不兼容更新
```

### 状态标签

| 标签 | 含义 |
|---|---|
| 💡 构想中 | 初步想法，尚未开始 |
| 📋 规划中 | 已确定计划，待开发 |
| 🚧 开发中 | 正在开发 |
| ✅ 已完成 | 开发完成 |
| 🚀 已发布 | 正式发布 |

### 编号前缀说明

| 前缀 | 含义 |
|---|---|
| FEATURE- | 新特性（New Feature） |
| ENHANCEMENT- | 改进（Enhancement） |
| FIX- | Bug 修复（Bug Fix） |
