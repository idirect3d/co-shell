// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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
  提供商:        %s
  端点:          %s
  模型:          %s
  温度:          %.1f
  最大令牌数:    %d
  最大迭代次数:  %s
  显示思考过程:  %s
  显示命令:      %s
  显示输出:      %s

日志: %s

MCP 服务器: %d
规则: %d`,

	// REPL - Additional
	KeyWelcomeTip:     "输入 .help 查看可用命令，或直接输入自然语言！",
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
	KeyHelpSettings:     "    .settings     - 管理 LLM API 设置（密钥、模型、端点等）",
	KeyHelpMCP:          "    .mcp          - 管理 MCP 服务器连接",
	KeyHelpRule:         "    .rule         - 管理 AI 全局规则",
	KeyHelpMemory:       "    .memory       - 管理记忆和持久知识",
	KeyHelpContext:      "    .context      - 管理对话上下文",
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

	// Custom
	KeyCustom: "自定义",
}
