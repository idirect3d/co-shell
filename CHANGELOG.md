# Changelog

All notable changes to co-shell will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.5.1] - 2026-05-28

### Added
- 内容级循环检测（checkContentLoop）：在 LLM 流式输出中检测重复文本块，使用滑动窗口自动匹配最佳块大小
- 用户消息模板统一格式化：`formatUserMessage` 应用于所有用户消息入队路径，确保 `{TASK_TRACKING}` 和 `{CURRENT_TIME}` 占位符被正确填充
- `attempt_completion` 工具：在 OpenAI 模式和 XML 模式中注册，为 LLM 提供报告任务完成结果的标准方式

### Fixed
- 修复用户消息未按模板格式化的问题：用户消息段缺少 `{TASK_TRACKING}` 和 `{CURRENT_TIME}` 内容
- 修复 API URL 自动拼写时重复添加 `/v1` 后缀的问题

### Changed
- BUILD 192 → 193

## [0.5.0-Beta2] - 2026-05-24

### Added
- ToolCall 模式系统：支持 OpenAI 原生 function calling 和 XML 格式两种工具调用模式
- XML 工具调用模式：通过系统提示词嵌入 XML 格式说明，不发送 `tools` 参数，兼容不支持 function calling 的模型
- `--toolcall-mode` CLI 参数，支持 `openai` / `xml` 两种模式
- `.set tool mode` 运行时切换工具调用模式
- `.set tool mode-system-prompt` 自定义各模式的系统提示词
- 工具调用错误处理模块：`agent/tool_error.go`，统一处理工具调用异常
- 流式响应处理模块：`agent/stream_response.go`，重构流式输出处理逻辑
- 非流式运行入口：`agent/run.go`，支持非流式回退
- 流式运行入口：`agent/run_stream.go`，重构流式运行逻辑
- 系统提示词 i18n 重构：拆分为 `i18n/keys.go` + `en_system.go` + `zh_system.go`，支持多语言系统提示词
- 模型命令交互改进：`.model` 子命令支持更友好的交互体验
- `build-release.sh` — 全平台 Release 构建脚本，以版本号为参数自动完成编译和打包
- 文档更新：README 新增 Bridge 和 PostgreSQL 扩展功能章节，同步更新中英文文档

### Changed
- `agent/loop.go` 重构：拆分为 `run.go`、`run_stream.go`、`stream_response.go`、`tool_error.go`，单文件从 938 行降至合理规模
- `i18n/i18n.go` 重构：拆分为 `keys.go` + `en_system.go` + `zh_system.go`，系统提示词独立管理
- 日志级别默认值从空值改为 `info`，确保默认开启 info 级别日志输出
- ToolCallMode 默认值从 `openai` 改为 `xml`（更兼容不支持 function calling 的模型）
- BUILD 150 → 192

## [0.4.0] - 2026-05-13

### Added
- PostgreSQL 存储支持：`.set db` 配置数据库连接，支持 bbolt 与 PostgreSQL 双存储后端
- 多模型管理：`.model` 命令（add/list/switch/remove/test），支持多模型配置和运行时切换
- 模型配置模板：支持 DeepSeek / OpenAI / Anthropic / Google Gemini / Ollama 等主流提供商
- 模型能力检测：自动测试模型的文本、视觉、工具调用、思考能力
- 思考/推理模式：`thinking-enabled` / `reasoning-effort` 配置，支持 DeepSeek 思考模式和 OpenAI reasoning_effort
- 采样参数控制：`top-p` / `top-k` / `repetition-penalty` 配置和 CLI 参数
- 输出控制开关：`show-llm-thinking` / `show-llm-content` / `show-tool` / `show-tool-input` / `show-tool-output` / `show-command` / `show-command-output` 运行时切换
- 表情符号前缀：`emoji-enabled` 配置，支持 emoji 和纯文本两种输出前缀模式
- Token 用量显示：`token-usage` 配置（on/off/none），支持 `stream_options.include_usage`
- 自定义请求体属性：`body-additions` 配置和 `--body-add` CLI 参数
- 上下文起始模式：`context-start` 配置（window/task/smart），支持 LLM 自动调整上下文起点
- 循环检测：`loop-detect-enabled` / `loop-detect-threshold` / `loop-detect-max-window` 配置
- 消息去重：`dedup-enabled` / `dedup-feature-ratio` / `dedup-match-ratio` / `dedup-sim-threshold` / `dedup-max-history` / `dedup-repeat-limit` 配置
- 错误跟踪：`error-max-single-count` / `error-max-type-count` 配置
- 搜索配置：`search-max-line-length` / `search-max-result-bytes` / `search-context-lines` 运行时配置
- 记忆搜索配置：`memory-search-max-content-len` / `memory-search-max-results` 运行时配置
- 超时配置：`tool-timeout` / `cmd-timeout` / `llm-timeout` 运行时配置
- 日志级别：`log-level` 配置和 `--log-level` CLI 参数（debug/info/warn/error/off）
- 启动 Logo 开关：`show-logo` 配置和 `--show-logo` CLI 参数
- 飞书集成：co-shell-feishu-bridge 独立程序，支持飞书机器人消息转发
- 配置热加载：`.set` 命令修改配置后自动保存并生效
- 设置显示优化：`.set` 命令显示所有可配置参数及其当前值
- 设置命令层级：`.set llm` / `.set agent` / `.set safety` / `.set search` / `.set log` / `.set db` 子命令
- 设置即时生效清单文档：`docs/log/settings-immediate-effect-inventory.md`
- 日志输出清单文档：`docs/log/log-output-inventory.md`

### Changed
- 配置结构重构：`LLMConfig` 新增大量字段，`Config` 结构体优化
- 模型管理重构：从单一模型配置升级为多模型管理器架构
- Agent 初始化流程重构：支持模型管理器、配置热加载
- 系统提示词重构：支持多语言、多模型、多模式
- 工具调用流程重构：支持工具模式（disabled/confirm/auto）和结果模式
- 输出控制重构：支持细粒度显示开关
- 会话管理重构：支持上下文起始模式
- BUILD 150 → 191

### Fixed
- 子代理配置继承：子代理通过环境变量 `CO_SHELL_CONFIG_PATH` 继承父代理配置
- 模型切换后 LLM 客户端未更新导致使用旧模型的问题
- 配置热加载后 Agent 状态未同步的问题
- 多语言系统提示词切换不完整的问题
- 工具调用模式配置不生效的问题

## [0.3.0] - 2026-04-29

### Added
- Multimodal model support: image input, visual understanding, 👀 indicator in version display
- Image cache management: `.image` built-in command (add/remove/clear/list), `add_images`/`remove_images`/`clear_images` LLM tools
- Agent identity customization: `name`/`description`/`principles` fields in config, `.set name/description/principles` runtime modification
- Task plan management: `.plan` built-in command (list/view/create/insert/remove/update), 6 LLM tools for plan CRUD and progress tracking
- Batch command execution: "Approve All" option inheritable to sub-agents
- 12 new CLI flags: `--temperature`, `--max-tokens`, `--show-thinking`, `--show-command`, `--show-output`, `--confirm-command`, `--result-mode`, `--description`, `--principles`, `--tool-timeout`, `--cmd-timeout`, `--llm-timeout`
- Vision support auto-detection: `ListModels` returns `ModelInfo` with `VisionSupport`, wizard auto-fetches vision capability
- Image path propagation to sub-agents via `--image` flag
- `--vision` CLI flag and `.set vision` runtime toggle for vision support

### Fixed
- Setup wizard panic on empty model list (index out of range)
- Help text inconsistencies: config default path, max-iterations default value, missing parameter descriptions in `.set` and `--help`

### Changed
- Version bumped from v0.1.0 to v0.3.0
- `ListModels` return type from `[]string` to `[]ModelInfo` (with VisionSupport field)
- Enhanced help documentation with complete parameter descriptions

## [0.2.0] - 2026-04-27

### Added
- Agent built-in file operation tools: `read_file`, `search_files`, `list_code_definition_names`, `replace_in_file`, `write_to_file`
- Workspace architecture: `--workspace` flag, auto-created `bin/`, `db/`, `log/`, `output/`, `tmp/` subdirectories
- Agent system prompt multi-language support (auto-switch Chinese/English based on i18n)
- Sub-agent support: launch child co-shell processes as sub-agents, shared I/O, result collection
- Scheduled task execution: crontab-like scheduling for sub-agents
- `-c`/`--config` flag for custom config file path, `config.LoadFromFile()` method
- `--name`/`-n` flag for custom agent name, `Said()` method with timestamp and name
- `.list`/`.last`/`.first` help documentation in `.help` command

### Fixed
- Sub-agent command argument conflict with `-c` flag causing config path loss

## [0.1.0] - 2026-04-26

### Added
- REPL interactive interface (go-prompt, Tab completion)
- LLM client abstraction (OpenAI-compatible API, streaming support)
- Agent core loop (LLM call → tool execution → iteration)
- Built-in command system (`.set` / `.mcp` / `.rule` / `.memory` / `.context` / `.list` / `.last` / `.first` / `.wizard`)
- Persistent storage (bbolt for memory/context)
- MCP client manager (multi-server connection)
- System command execution (timeout control, command confirmation)
- Configuration management (JSON persistence to `~/.co-shell/`, multi-location loading)
- API initial setup with setup wizard (multi-provider support)
- Direct system command execution (bypass LLM for native commands)
- Logging system (file log, runtime toggle)
- API Key masking (show first 4 + last 4 chars)
- Command-line flags (`--help` / `--version` / `--model` / `--endpoint` / `--api-key` / `--log` / `--max-iterations` / `--lang`)
- Single-command mode (`-c`/`--cmd` flag)
- Session history (arrow key navigation, cross-session persistence, `.list`/`.last`/`.first` commands)
- Internationalization (i18n) support: Chinese and English, `--lang` flag, auto-detect system language
- Multi-provider support (DeepSeek v4 / Alibaba Qwen / OpenAI-compatible fallback)
- Result processing modes (minimal / explain / analyze / free)
- Configurable timeouts for all operations
- Cross-platform support (macOS / Linux / Windows)
- Streaming output with thinking process display
- Setup wizard enhancements: Tab completion, arrow key selection, ESC exit, connection test
- Setup wizard auto-test: endpoint connectivity check, auto-fetch model list on API key input

### Fixed
- DeepSeek thinking mode: `reasoning_content` not passed back causing API 400 error
- `max_iterations=0` in config.json causing Agent to use internal default of 10
- Stream completion causing non-streaming API call and halving iteration count
- Message history incomplete (assistant with tool_calls but missing tool messages) causing API 400 error

    - 阶段 7 · 窗口尺寸档位 + 屏幕保护 + 响应式折叠 + channel/viewport 感知 [BUILD-1043]：ui_window 新增 size 档位（auto/small/medium/large，按视口可用区比例 small=1/2、medium=2/3、large=1，auto 保持原行为）；前端按视口钳制并写入 #uiWindow 的 data-ui-win-w/h；窗口最小 240x160、row 子项 min(180px,100%)、.ui-window-body overflow:auto；@container (max-width:380px) 时 .ui-row 转 column、.ui-kv 转单列；RuntimeInfo 新增 Channel 与 ViewportW/H 并注入 <runtime_info>，新增上行消息 {"type":"viewport"}。
    - 阶段 8 · 设置面板接入开关 [BUILD-1044]：`ui_enabled`（UI 组件与窗口渲染总开关，同时管 `render_ui` + `ui_window`）与 `ui_context_prune`（组件树上下文裁剪）从"仅 config 层"接入系统设置「智能体设置」页——`cmd/settings_web.go` 的 agentGroup 新增两个 bool 项（含默认值，供面板显示 diff）；`cmd/settings.go` 的 `.set` 分发白名单与 `formatSettings` 全览同步；`cmd/settings_agent.go` 新增 `ui-enabled` / `ui-context-prune` 两个 case（写回 cfg → Save() → SetConfig()，无需重启即生效，因工具注册与上下文构建都读 agent 的 cfg）；新增 i18n 键 `KeyCol3UIEnabled` / `KeyCol3UIContextPrune` / `KeySettingUIEnabledStatus` / `KeySettingUIContextPruneStatus`（中英齐备）；新增单测 `cmd/ui_switch_settings_test.go`（面板含两项 + `:set` 落盘反读 + 非法值容错），用例 UC-73~UC-75。浏览器实测（28267）：面板「智能体设置」组第 17/18 项即为这两个开关（默认 on），点击切换返回「UI 组件与窗口渲染: 开/关」并写入 `~/.co-shell/config.json`（`"ui_enabled": true` / `"ui_context_prune": true`）。
