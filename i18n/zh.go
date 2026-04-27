// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
// Package i18n - Chinese translations.
package i18n

var zhMessages = map[string]string{
	// General
	KeyCancelled:      "已取消",
	KeySetupCancelled: "❌ 设置未完成，退出。",
	KeyYes:            "是",
	KeyNo:             "否",
	KeyOn:             "开",
	KeyOff:            "关",
	KeyError:          "错误",
	KeyWarning:        "警告",
	KeySuccess:        "成功",
	KeyUnlimited:      "不限制",
	KeyDefault:        "默认",

	// Wizard
	KeyWizardTitle:       "🔧 co-shell API 设置向导",
	KeyWizardDescription: "使用 co-shell 前需要配置 LLM API。\n随时按 ESC 退出。",
	KeySelectProvider:    "📌 选择 LLM 提供商（回车默认，Tab 显示列表）",
	KeyProviderSelected:  "📌 已选择提供商: %s",
	KeyProviderLabel:     "📌 提供商",
	KeyEndpointLabel:     "📌 API 端点",
	KeyEndpointRequired:  "📌 API 端点（必填）",
	KeyAPIKeyLabel:       "📌 API 密钥（必填）",
	KeyAPIKeyRequired:    "🔑 输入 API 密钥以获取可用模型。",
	KeyModelName:         "📌 模型名称",
	KeyAPITest:           "🔄 正在测试 API 连接...",
	KeyAPITestOK:         " ✅ 连接成功！",
	KeyAPITestFail:       "\n❌ 连接测试失败: %v\n",
	KeyFetchModels:       "🔄 正在获取可用模型...",
	KeyFetchModelsOK:     " ✅ 找到 %d 个可用模型！",
	KeyFetchModelsFail:   "\n❌ 获取模型失败: %v\n",
	KeyEndpointTest:      "🔄 正在测试端点连通性...",
	KeyEndpointTestOK:    " ✅ 端点可达！",
	KeyEndpointTestFail:  "\n❌ 端点连接失败: %v\n",
	KeyEndpointRetry:     "⚠️ 请检查端点 URL 并重新输入。",
	KeyAPIKeyGetPrompt:   "🔑 API 密钥是调用 %s API 的凭证。",
	KeyAPIKeyManualGet:   "   手动获取 API 密钥并粘贴到下方。",
	KeyAPIKeyOpenPage:    "   打开 %s API 密钥页面？",
	KeyAPIKeyOpeningPage: "   🔗 正在打开: %s",
	KeyAPIKeyManualOpen:  "   请访问: %s",
	KeyEmptyField:        "⚠️ 此项不能为空，请重新输入。",
	KeyInvalidChoice:     "⚠️ 无效选择，请输入数字 1-%d 或提供商名称。",
	KeyConfigSaved:       "✅ 配置已保存到 ~/.co-shell/config.json",

	// REPL
	KeyGoodbye:     "\n👋 再见！",
	KeyExit:        "👋 再见！",
	KeyCleanup:     "正在清理...",
	KeyCleanupDone: " 完成。",
	KeyUnknownCmd:  "❌ 未知命令: %s\n输入 .help 查看可用命令列表",
	KeyCmdError:    "❌ 错误: %v",
	KeyCmdExecFail: "❌ 命令执行失败: %v",
	KeyAgentFail:   "❌ 处理失败: %v",
	KeyAgentHint:   "💡 提示: 请检查 API 配置是否正确，输入 .settings 查看当前配置",
	KeyOutputTitle: "📋 命令输出:",
	KeyOutputSep:   "────────────────────────────────────────────",
	KeyToolCall:    "🛠 调用工具: %s\n",

	// Settings - Messages
	KeySettingsUpdated:  "✅ API 密钥已更新",
	KeyEndpointUpdated:  "✅ 端点已更新",
	KeyModelUpdated:     "✅ 模型已更新",
	KeyTempUpdated:      "✅ 温度已设置为 %.1f",
	KeyMaxTokensUpdated: "✅ 最大令牌数已设置为 %d",
	KeyProviderUpdated:  "✅ 提供商已设置为 %s（端点: %s，模型: %s）",

	// Settings - Config Show
	KeyConfigTitle:         "LLM 配置:",
	KeyConfigProvider:      "  提供商:        %s\n",
	KeyConfigEndpoint:      "  端点:          %s\n",
	KeyConfigModel:         "  模型:          %s\n",
	KeyConfigTemperature:   "  温度:          %.1f\n",
	KeyConfigMaxTokens:     "  最大令牌数:    %d\n",
	KeyConfigMaxIterations: "  最大迭代次数:  %s\n",
	KeyConfigShowThinking:  "  显示思考过程:  %s\n",
	KeyConfigShowCommand:   "  显示命令:      %s\n",
	KeyConfigShowOutput:    "  显示输出:      %s\n",
	KeyConfigLogging:       "日志: %s\n",
	KeyConfigMCPServers:    "MCP 服务器: %d\n",
	KeyConfigRules:         "规则: %d\n",

	// Settings - Labels
	KeySettingsLabel:        "设置",
	KeyAPIKeyLabelSetting:   "API 密钥",
	KeyEndpointLabelSetting: "端点",
	KeyModelLabel:           "模型",
	KeyTempLabel:            "温度",
	KeyMaxTokensLabel:       "最大令牌数",
	KeyProviderLabelSetting: "提供商",

	// Settings - Extended
	KeySettingsLabelLog:           "日志",
	KeySettingsLabelShowThinking:  "显示思考过程",
	KeySettingsLabelShowCommand:   "显示命令",
	KeySettingsLabelShowOutput:    "显示输出",
	KeySettingsLabelMaxIterations: "最大迭代次数",
	KeySettingsLabelProvider:      "提供商",

	// Settings - Display
	KeyShowThinking:  "显示思考过程: %s",
	KeyShowCommand:   "显示命令: %s",
	KeyShowOutput:    "显示输出: %s",
	KeyLogEnabled:    "日志: %s",
	KeyMaxIterations: "最大迭代次数: %d",

	// MCP
	KeyMCPAlreadyExists: "MCP 服务器 '%s' 已存在",
	KeyMCPAdded:         "✅ MCP 服务器 '%s' 已添加",
	KeyMCPRemoved:       "✅ MCP 服务器 '%s' 已移除",
	KeyMCPNotFound:      "MCP 服务器 '%s' 未找到",
	KeyMCPEnabled:       "✅ MCP 服务器 '%s' 已启用",
	KeyMCPDisabled:      "✅ MCP 服务器 '%s' 已禁用",
	KeyMCPEmpty:         "未配置 MCP 服务器",
	KeyMCPListTitle:     "MCP 服务器:",

	// Rule
	KeyRuleAdded:   "✅ 规则已添加",
	KeyRuleRemoved: "✅ 规则已移除",
	KeyRuleCleared: "✅ 所有规则已清除",
	KeyRuleInvalid: "❌ 无效索引: %d",
	KeyRuleNoRules: "未配置规则",

	// Memory
	KeyMemorySaved:   "✅ 记忆已保存",
	KeyMemoryDeleted: "✅ 记忆已删除",
	KeyMemoryCleared: "✅ 所有记忆已清除",
	KeyMemoryEmpty:   "暂无记忆条目",
	KeyMemoryGet:     "记忆 #%d:\n  %s",

	// Context
	KeyContextShow:  "当前上下文:\n%s",
	KeyContextEmpty: "上下文为空",
	KeyContextReset: "✅ 上下文已重置",
	KeyContextSet:   "✅ %s 已设置为: %s",

	// Agent
	KeyNoopClientError: "LLM 未配置。请使用 .settings api-key <your-key> 设置 API 密钥",

	// Config format
	KeyConfigFormat: `LLM 配置:
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s

  %-20s %-30s %s
  %-20s %-30s %s
  %-20s %-30s %s

  %-20s %-30d %s
  %-20s %-30d %s
  %-20s %-30s %s`,

	// REPL - Additional
	KeyWelcomeTip: "输入 .help 查看可用命令，或直接输入自然语言！\n   例如：\"列出当前目录的文件\"",

	KeyUnknownCommand: "❌ 未知命令: %s\n输入 .help 查看可用命令列表",
	KeyCmdFailed:      "命令执行失败",
	KeyProcessFailed:  "处理失败",
	KeyCheckConfig:    "💡 提示: 请检查 API 配置是否正确，输入 .settings 查看当前配置",
	KeyCleaningUp:     "正在清理...",
	KeyDone:           " 完成。",

	// Help
	KeyHelpTitle:        "可用命令:",
	KeyHelpNLTitle:      "  自然语言:",
	KeyHelpNLDesc:       "    直接输入自然语言请求，我会帮你执行。",
	KeyHelpBuiltinTitle: "  内置命令（以 . 开头）:",
	KeyHelpSettings:     "    .set          - 管理 LLM API 设置（密钥、模型、端点等）",

	KeyHelpMCP:          "    .mcp          - 管理 MCP 服务器连接",
	KeyHelpRule:         "    .rule         - 管理 AI 全局规则",
	KeyHelpMemory:       "    .memory       - 管理记忆和持久知识",
	KeyHelpContext:      "    .context      - 管理对话上下文",
	KeyHelpList:         "    .list         - 查看历史任务列表",
	KeyHelpLast:         "    .last         - 查看最近的历史任务",
	KeyHelpFirst:        "    .first        - 查看最早的历史任务",
	KeyHelpHelp:         "    .help         - 显示此帮助信息",
	KeyHelpExit:         "    .exit         - 退出 co-shell",
	KeyHelpExampleTitle: "  示例:",
	KeyHelpExample1:     "    ❯ 列出当前目录的所有文件",
	KeyHelpExample2:     "    ❯ 查找所有超过 100MB 的大文件",
	KeyHelpExample3:     "    ❯ .settings model gpt-4o",
	KeyHelpExample4:     "    ❯ .mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp",
	KeyHelpExample5:     "    ❯ .rule add \"删除文件前先确认\"",

	// CLI Help
	KeyCLIHelpTitle:     "co-shell v%s - 智能命令行 Shell",
	KeyCLIHelpUsage:     "用法:",
	KeyCLIHelpUsageREPL: "  co-shell [选项]                    启动交互式 REPL",
	KeyCLIHelpUsageCmd:  "  co-shell [选项] <指令>             执行单条指令后退出",
	KeyCLIHelpOptions:   "选项:",
	KeyCLIHelpConfig:    "  -c, --config <path>    指定配置文件路径（默认: ~/.co-shell/config.json）",
	KeyCLIHelpModel:     "  -m, --model <name>     临时指定模型名称（覆盖配置文件）",
	KeyCLIHelpEndpoint:  "  -e, --endpoint <url>   临时指定 API 端点（覆盖配置文件）",
	KeyCLIHelpAPIKey:    "  -k, --api-key <key>    临时指定 API Key（覆盖配置文件）",
	KeyCLIHelpLang:      "      --lang <code>      设置语言（zh/en，默认自动检测）",
	KeyCLIHelpLog:       "      --log on|off       临时指定日志开关（覆盖配置文件）",
	KeyCLIHelpMaxIter:   "      --max-iterations   最大迭代次数（-1 为不限制，默认 10）",
	KeyCLIHelpVersion:   "  -v, --version          显示版本信息",
	KeyCLIHelpHelp:      "  -h, --help             显示帮助信息",
	KeyCLIHelpExamples:  "示例:",
	KeyCLIHelpEx1:       "  co-shell                             启动交互式 REPL",
	KeyCLIHelpEx2:       "  co-shell 列出当前目录的文件           执行自然语言指令",
	KeyCLIHelpEx3:       `  co-shell "cat ~/.co-shell/config.json"  执行系统命令`,
	KeyCLIHelpEx4:       "  co-shell -m deepseek-chat 你好       指定模型并执行指令",
	KeyCLIHelpEx5:       "  co-shell -k sk-xxxx --log off        临时指定 API Key 并关闭日志",
	KeyCLIHelpEx6:       "  co-shell --lang en                    以英文界面启动",
	KeyCLIHelpEx7:       "  co-shell --max-iterations 20 列出文件  设置最大迭代次数并执行指令",
	KeyCLIHelpName:      "  --name, -n <名称>                    指定 agent 名称（默认：co-shell）",
	KeyAgentSaid:        "%s %s 说：",
	KeyCLIHelpEx8:       "  co-shell -w /path/to/workspace         使用自定义工作区启动",

	// CLI Help - Workspace
	KeyCLIHelpWorkspace: "  -w, --workspace <path>  工作区路径（默认：当前目录）",

	// Command Confirmation
	KeyCmdConfirmTitle:       "⚡ 即将执行命令: %s",
	KeyCmdConfirmDisabled:    "命令执行确认: 关闭",
	KeyCmdConfirmEnabled:     "命令执行确认: 开启",
	KeyCmdConfirmDisableWarn: "⚠️ 警告: 关闭命令执行确认后，AI 将直接执行命令而不经您确认，可能存在安全风险（如误删文件、无限循环等）。请谨慎操作。",

	KeyCmdConfirmPrompt:     "请选择操作:\n  [Enter] 批准执行\n  [A] 本次都批准\n  [C] 取消\n  其他输入: 作为补充说明，AI 将重新评估\n请输入: ",
	KeyCmdConfirmApprove:    "a",
	KeyCmdConfirmApproveAll: "aa",
	KeyCmdConfirmCancel:     "c",
	KeyCmdConfirmModify:     "m",
	KeyCmdConfirmInvalid:    "无效输入，请直接回车批准、输入 a 本次都批准、输入 c 取消，或输入补充说明。",
	KeyCmdConfirmCancelled:  "已取消。",
	KeyCmdConfirmModifyHint: "请输入补充说明，AI 将重新评估: ",

	// Disclaimer
	KeyDisclaimerTitle: "⚠️ 风险声明",

	KeyDisclaimerBody: `co-shell 是一个由大语言模型（LLM）驱动的智能命令行工具。
AI 模型可能会生成并执行以下类型的危险命令：

  • 删除文件或目录（如 rm -rf / 等）
  • 格式化磁盘（如 mkfs、format 等）
  • 修改系统关键配置（如 /etc/passwd、/etc/shadow 等）
  • 关闭或重启系统（如 shutdown、reboot 等）
  • 下载并执行未知来源的程序
  • 泄露敏感信息（如 API Key、密码、密钥等）

继续使用本程序即表示您已充分了解上述风险，
并同意自行承担所有因使用本程序可能导致的任何损失或损害。
开发者和发布者不承担任何责任。`,
	KeyDisclaimerPrompt:  "是否接受上述声明并继续？[Y/n] ",
	KeyDisclaimerYes:     "y",
	KeyDisclaimerNo:      "n",
	KeyDisclaimerRefused: "您已拒绝风险声明，程序退出。",

	// Wizard command
	KeyWizardCmdRunning: "🔄 正在启动 API 设置向导...\n",
	KeyWizardCmdDone:    "✅ API 设置向导已完成。\n",
	KeyHelpWizard:       "    .wizard        - 重新启动 API 设置向导",

	// Settings help table
	KeySettingsHelpTitle:        "📋 .set 参数清单",
	KeySettingsColParam:         "参数名",
	KeySettingsColValues:        "可选项 / 值范围",
	KeySettingsColDesc:          "说明",
	KeySettingsDescAPIKey:       "设置 API 密钥",
	KeySettingsDescEndpoint:     "设置 API 端点 URL",
	KeySettingsDescModel:        "设置模型名称",
	KeySettingsDescTemp:         "设置温度参数（越高越随机）",
	KeySettingsDescMaxTokens:    "设置最大输出令牌数",
	KeySettingsDescShowThinking: "显示 AI 思考过程",
	KeySettingsDescShowCommand:  "显示执行的系统命令",
	KeySettingsDescShowOutput:   "显示命令执行输出",
	KeySettingsDescConfirmCmd:   "执行命令前需确认",
	KeySettingsDescLog:          "日志开关",
	KeySettingsDescMaxIter:      "最大迭代次数（-1=不限）",
	KeySettingsDescToolTimeout:  "工具调用超时（0=不限）",
	KeySettingsDescCmdTimeout:   "命令执行超时（0=不限）",
	KeySettingsDescLLMTimeout:   "LLM 请求超时（0=不限）",
	KeySettingsHelpFooter:       "💡 使用 .set <参数名> <值> 修改参数，例如: .set model deepseek-chat",
	KeySettingsCurrentTitle:     "当前配置:",

	// Config show column 3 labels
	KeyCol3Provider:    "提供商(deepseek/qwen/openai)",
	KeyCol3Endpoint:    "API服务器",
	KeyCol3Model:       "模型ID",
	KeyCol3Temperature: "温度(0.0 ~ 2.0)",
	KeyCol3MaxTokens:   "最大输出令牌数(1 ~ N（不限制）)",
	KeyCol3MaxIter:     "最大迭代次数(-1 ~ N)",
	KeyCol3Thinking:    "显示思考过程(on|off)",
	KeyCol3Command:     "显示命令(on|off)",
	KeyCol3Output:      "显示输出(on|off)",
	KeyCol3Confirm:     "命令确认(on|off)",
	KeyCol3ToolTimeout: "工具调用超时(0 ~ N 秒)",
	KeyCol3CmdTimeout:  "命令执行超时(0 ~ N 秒)",
	KeyCol3LLMTimeout:  "LLM 请求超时(0 ~ N 秒)",
	KeyCol3Log:         "日志(on|off)",
	KeyCol3ResultMode:  "结果模式(minimal/explain/analyze/free)",
	KeyCol3APIKey:      "API 密钥",
	KeyCol3Name:        "Agent 名称",
	KeyCol3Desc:        "Agent 描述",
	KeyCol3Principles:  "Agent 核心原则",

	// History list
	KeyListTitle:     "📋 历史任务列表:",
	KeyListEmpty:     "暂无历史记录。",
	KeyListReExecute: "输入编号重新执行，或输入其他内容继续。",
	KeyListInvalid:   "无效编号，请输入 1-%d 之间的数字。",
	KeyLastUsage:     "用法: .last [N] — 显示最近 N 条历史记录（默认 10）",
	KeyFirstUsage:    "用法: .first [N] — 显示最早 N 条历史记录（默认 10）",
	KeyListUsage:     "用法: .list [start] [end] — 显示历史记录范围（序号从 1 开始）",

	// Custom
	KeyCustom: "自定义",

	// System Prompt
	KeySystemPromptIdentity: `## 你的身份

你是 %s，一个智能命令行助手，帮助用户通过自然语言与系统交互。

%s

%s`,
	KeySystemPromptTitle: `你是 co-shell，一个由 Go 语言编写的智能命令行应用程序，可通过自然语言指令直接操作系统。

当前环境:
- 平台: %s (%s)
- Shell: %s
- 当前时间: %s
- 工作目录: %s
- 主机名: %s
- 用户: %s`,
	KeySystemPromptCapabilities: `你拥有以下能力:
1. 执行系统命令 (%s)
2. 调用 MCP（Model Context Protocol）工具
3. 读写文件
4. 管理记忆和上下文
5. 你还有很多核心技能让你无所不能，比如启动一给你的分身sub-agent`,
	KeySystemPromptRules: `重要规则:
- 使用 "execute_command" 工具运行系统命令，使用对应的 MCP 工具名称进行 MCP 操作。
- 除非用户特别指定，否则优先使用标准系统命令（如 cat、ls、dir、type），而不是编写脚本或程序。
- 主动探索系统以发现可用工具（如检查 PATH、常见工具目录）。如果找不到所需工具，尝试安装它，或使用脚本和编程语言（Shell、Python、Go、Node.js 等）编写自定义工具来满足用户需求。
- 在执行命令前，始终解释你要做什么。
- 对于破坏性操作（删除、覆盖、rm -rf 等），先请求确认。
- 使用用户偏好的语言进行回复，而且如果有你认为不确定就无法完成最终目标的事用户没有说清楚，你就要大胆的向用户提出你的疑问。
- 你有完全的自主权，可以为每个任务选择最佳的工具和方法——请自行判断。`,
	KeySystemPromptResultMode: `结果处理模式:
%s`,
}
