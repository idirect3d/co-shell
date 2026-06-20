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
	KeyCancelled:        "已取消",
	KeySetupCancelled:   "❌ 设置未完成，退出。",
	KeyYes:              "是",
	KeyNo:               "否",
	KeyOn:               "开",
	KeyOff:              "关",
	KeyError:            "错误",
	KeyWarning:          "警告",
	KeySuccess:          "成功",
	KeyUnlimited:        "不限制",
	KeyDefault:          "默认",
	KeyUnknown:          "未知",
	KeySectionSeparator: "\n====\n",

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
	KeyConfigShowOutput:    "  显示命令输出:  %s\n",
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
	KeySettingsLabelShowOutput:    "显示命令输出", // DEPRECATED
	KeySettingsLabelMaxIterations: "最大迭代次数",
	KeySettingsLabelProvider:      "提供商",

	// New output control keys (ENHANCEMENT-126)
	KeySettingsLabelShowLlmThinking:   "显示 LLM 思考过程",
	KeySettingsLabelShowLlmContent:    "显示 LLM 内容",
	KeySettingsLabelShowTool:          "显示工具调用名",
	KeySettingsLabelShowToolInput:     "显示工具调用参数",
	KeySettingsLabelShowToolOutput:    "显示工具返回数据",
	KeySettingsLabelShowCommandOutput: "显示命令返回数据",

	// Settings - Display
	KeyShowThinking:  "显示思考过程: %s",
	KeyShowCommand:   "显示命令: %s",
	KeyShowOutput:    "显示命令输出: %s", // DEPRECATED
	KeyLogEnabled:    "日志: %s",
	KeyMaxIterations: "最大迭代次数: %d",

	// New output control display keys (ENHANCEMENT-126)
	KeyShowLlmThinking:   "显示 LLM 思考过程: %s",
	KeyShowLlmContent:    "显示 LLM 内容: %s",
	KeyShowTool:          "显示工具调用名: %s",
	KeyShowToolInput:     "显示工具调用参数: %s",
	KeyShowToolOutput:    "显示工具返回数据: %s",
	KeyShowCommandOutput: "显示命令返回数据: %s",

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
   %-20s %-30s %s
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
	KeyHelpList:         "    .list         - 查看历史用户输入命令列表（同 .history）",
	KeyHelpLast:         "    .last         - 查看最近的历史任务",
	KeyHelpFirst:        "    .first        - 查看最早的历史任务",
	KeyHelpImage:        "    .image        - 管理多模态图片缓存（add/remove/clear/list）",
	KeyHelpPlan:         "    .plan         - 管理任务计划（list/view/create/insert/remove/update）",
	KeyHelpBodyAdd:      "    .body-add     - 向 LLM 请求体添加自定义 JSON 属性",
	KeyHelpBodyRemove:   "    .body-remove  - 从 LLM 请求体删除自定义 JSON 属性",
	KeyHelpBodyDisplay:  "    .body-display - 显示 LLM 请求体中的自定义 JSON 属性",
	KeyHelpNew:          "    .new          - 清空当前会话，开始全新对话",
	KeyHelpHelp:         "    .help         - 显示此帮助信息",
	KeyHelpExit:         "    .exit         - 退出 co-shell",
	KeyHelpExampleTitle: "  示例:",
	KeyHelpExample1:     "列出当前目录的所有文件",
	KeyHelpExample2:     "查找所有超过 100MB 的大文件",
	KeyHelpExample3:     ".settings model gpt-4o",
	KeyHelpExample4:     ".mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp",
	KeyHelpExample5:     ".rule add \"删除文件前先确认\"",

	// CLI Help
	KeyCLIHelpTitle:                "co-shell v%s - 智能命令行 Shell",
	KeyCLIHelpUsage:                "用法:",
	KeyCLIHelpUsageREPL:            "  co-shell [选项]                         启动交互式 REPL",
	KeyCLIHelpUsageCmd:             "  co-shell [选项] <指令>                  执行单条指令后退出",
	KeyCLIHelpOptions:              "选项:",
	KeyCLIHelpConfig:               "  -c, --config <path>       指定配置文件路径（默认: {workspace}/config.json）",
	KeyCLIHelpModel:                "  -m, --model <name>        临时指定模型名称（覆盖配置文件）",
	KeyCLIHelpEndpoint:             "  -e, --endpoint <url>      临时指定 API 端点（覆盖配置文件）",
	KeyCLIHelpAPIKey:               "  -k, --api-key <key>       临时指定 API Key（覆盖配置文件）",
	KeyCLIHelpLang:                 "      --lang <code>         设置语言（zh/en，默认自动检测）",
	KeyCLIHelpLog:                  "      --log on|off          临时指定日志开关（覆盖配置文件）",
	KeyCLIHelpMaxIter:              "      --max-iterations      最大迭代次数（-1 为不限制，默认 1000）",
	KeyCLIHelpImage:                "  -i, --image <path>        图片文件路径（多张图片用逗号分隔），用于多模态输入",
	KeyCLIHelpVersion:              "  -v, --version             显示版本信息",
	KeyCLIHelpHelp:                 "  -h, --help                显示帮助信息",
	KeyCLIHelpExamples:             "示例:",
	KeyCLIHelpEx1:                  "  co-shell                             启动交互式 REPL",
	KeyCLIHelpEx2:                  "  co-shell 列出当前目录的文件           执行自然语言指令",
	KeyCLIHelpEx3:                  `  co-shell "cat ~/.co-shell/config.json"  执行系统命令`,
	KeyCLIHelpEx4:                  "  co-shell -m deepseek-chat 你好       指定模型并执行指令",
	KeyCLIHelpEx5:                  "  co-shell -k sk-xxxx --log off        临时指定 API Key 并关闭日志",
	KeyCLIHelpEx6:                  "  co-shell --lang en                    以英文界面启动",
	KeyCLIHelpEx7:                  "  co-shell --max-iterations 20 列出文件  设置最大迭代次数并执行指令",
	KeyCLIHelpName:                 "  --name, -n <名称>                    指定 agent 名称（默认：co-shell）",
	KeyAgentSaid:                   "%s %s 说：",
	KeyAgentDefaultDescription:     "一位经验丰富的软件工程师，在多种编程语言、框架、设计模式和最佳实践方面拥有广泛知识",
	KeyAgentDefaultDescriptionAct:  "你是一位经验丰富的软件工程师，在多种编程语言、框架、设计模式和最佳实践方面拥有广泛知识，你当前处于**行动模式（Act Mode）**。",
	KeyAgentDefaultDescriptionPlan: "你是一位经验丰富的系统架构师，在多种编程语言、框架、设计模式和最佳实践方面拥有广泛知识，你当前处于**规划模式（Plan Mode）**。",
	KeyAgentDefaultDescriptionResearch: `你是一个全科研究员，擅长搜集专业资料和专业报告的编写，能够以专业视角开展相关的调查研究工作，你当前处于**调研模式（Research Mode）**
在此模式下，你的核心职责是：
- **收集信息**：搜索代码库、查阅文档、浏览网页、收集相关资料
- **分析整理**：梳理调研结果、归纳关键发现、评估不同方案的优劣
- **输出报告**：将调研成果整理为结构化的研究报告（Markdown/Word 格式），保存在 ./research/ 下

你可以使用所有可用的工具来获取信息。请注意保存原始资料以便审稿验证。
`,
	KeyCLIHelpEx8:  "  co-shell -w /path/to/workspace         使用自定义工作区启动",
	KeyCLIHelpEx9:  "  co-shell --temperature 0.8 写一首诗    指定温度参数并执行指令",
	KeyCLIHelpEx10: "  co-shell --show-thinking on --show-command on 分析日志  显示思考过程和命令",
	KeyCLIHelpEx11: "  co-shell --result-mode analyze \"查看系统状态\"  以分析模式处理结果",

	// CLI Help - LLM Behavior
	KeyCLIHelpTemperature:  "      --temperature <n>   温度参数（0.0 ~ 2.0，覆盖配置文件）",
	KeyCLIHelpMaxTokens:    "      --max-tokens <n>   最大输出令牌数（覆盖配置文件）",
	KeyCLIHelpShowThinking: "      --show-thinking    显示 AI 思考过程（on/off，覆盖配置文件）",
	KeyCLIHelpShowCommand:  "      --show-command     显示执行的系统命令（on/off，覆盖配置文件）",
	KeyCLIHelpConfirmTool:  "      --confirm-tool  工具调用前需确认（on/off，覆盖配置文件）。\n                          可控制工具: execute_command, read_file, write_to_file,\n                          replace_in_file, search_files, list_code_definition_names,\n                          add_images, remove_images, clear_images, update_settings,\n                          list_settings, ask_followup_question, adjust_context_start,\n                          launch_sub_agent, schedule_task, create_task_plan,\n                          update_task_step, insert_task_steps, remove_task_steps,\n                          view_task_plan, get_memory_slice,\n                          memory_search, delete_memory, shell_start, shell_exec,\n                          shell_get_output, shell_stop 及 MCP 工具",
	KeyCLIHelpResultMode:   "      --result-mode      结果处理模式（minimal/explain/analyze/free，覆盖配置文件）",

	// CLI Help - New output control (ENHANCEMENT-126)
	KeyCLIHelpShowLlmThinking:   "      --show-llm-thinking  显示 LLM 思考过程（on/off，覆盖配置文件）",
	KeyCLIHelpShowLlmContent:    "      --show-llm-content   显示 LLM 返回内容（on/off，覆盖配置文件）",
	KeyCLIHelpShowTool:          "      --show-tool          显示工具调用名（on/off，覆盖配置文件）",
	KeyCLIHelpShowToolInput:     "      --show-tool-input    显示工具调用参数（on/off，覆盖配置文件）",
	KeyCLIHelpShowToolOutput:    "      --show-tool-output   显示工具返回数据（on/off，覆盖配置文件）",
	KeyCLIHelpShowCommandOutput: "      --show-command-output 显示命令返回数据（on/off，覆盖配置文件）",

	// CLI Help - Agent Identity
	KeyCLIHelpDescription: "      --description <text>  指定 agent 描述/专长（覆盖配置文件）",
	KeyCLIHelpPrinciples:  "      --principles <text>   指定 agent 核心原则（覆盖配置文件）",

	// CLI Help - Timeout
	KeyCLIHelpToolTimeout:       "      --tool-timeout <s>  工具调用超时秒数（0=不限，覆盖配置文件）",
	KeyCLIHelpCmdTimeout:        "      --cmd-timeout <s>   系统命令执行超时秒数（0=不限，覆盖配置文件）",
	KeyCLIHelpLLMTimeout:        "      --llm-timeout <s>   LLM API 请求超时秒数（0=不限，覆盖配置文件）",
	KeyCLIHelpTopP:              "  --top-p <value>               Top-P 采样参数（0.0 ~ 1.0，-1 不发送，覆盖配置文件）\n",
	KeyCLIHelpTopK:              "  --top-k <value>               Top-K 采样参数（>= 1 的整数，-1 不发送，覆盖配置文件）\n",
	KeyCLIHelpRepetitionPenalty: "  --repetition-penalty <value>  重复惩罚参数（0.0 ~ 2.0，-1 不发送，覆盖配置文件）\n",

	// CLI Help - Loop Detection & Dedup (FIX-179)
	KeyCLIHelpLoopDetect: "      --loop-detect-enabled  启用循环检测（on/off，覆盖配置文件）",
	KeyCLIHelpDedup:      "      --dedup-enabled      启用消息去重检测（on/off，覆盖配置文件）",

	"col3_search_max_line_length":           "搜索单行最大字符数",
	"col3_search_max_result_bytes":          "搜索结果最大字节数",
	"col3_search_context_lines":             "搜索匹配上下文行数",
	"col3_mcp":                              "MCP 服务器数量",
	"col3_rules":                            "规则数量",
	"settings_desc_search_max_line_length":  "搜索文件时单行最大字符数，超长截断（默认 8192）",
	"settings_desc_search_max_result_bytes": "搜索文件时返回结果最大字节数，超长截断（默认 65536）",

	// Search results
	"search_result_found":         "在 %s 目录下找到 %d 处匹配模式 %q 的结果：",
	"search_result_found_trunc":   "在 %s 目录下找到 %d 处匹配模式 %q 的结果，但有 %d 行内容超长返回被截断（见行尾标注）：",
	"search_result_found_partial": "在 %s 目录下找到至少 %d 处匹配模式 %q 的结果，由于内容超长，无法全部返回：",

	"search_result_none":        "在 %s 目录下未找到匹配模式 %q 的结果",
	"search_line_truncated":     "（...后面被截断%d字符）",
	"search_result_file_header": "%s:%d-%d:",
	"search_result_match_line":  "%d: %s",

	// CLI Help - Workspace
	KeyCLIHelpWorkspace: "  -w, --workspace <path>  工作区路径（默认：当前目录）",

	// Command Confirmation
	KeyCmdConfirmTitle:       "⚡ 即将执行命令: %s",
	KeyCmdConfirmRiskWarning: "⚠️ 风险提示: 请仔细检查命令内容，确认无误后再执行。\n    AI 可能生成危险命令（如删除文件、修改系统配置等），请谨慎操作并根据使用经验自行判断风险。",
	KeyCmdConfirmDisabled:    "命令执行确认: 关闭",
	KeyCmdConfirmEnabled:     "命令执行确认: 开启",
	KeyCmdConfirmDisableWarn: "⚠️ 警告: 关闭命令执行确认后，AI 将直接执行命令而不经您确认，可能存在安全风险（如误删文件、无限循环等）。请谨慎操作并根据使用经验自行判断风险。",

	KeyCmdConfirmPrompt:       "请选择操作:\n  [Enter] 批准执行\n  [A] 本次都批准\n  [G] 永久自动执行此工具\n  [D] 永久禁用此工具\n  [C] 取消\n  输入数字: 批准后续 N 次此方法调用\n  其他输入: 暂不执行，输入的内容将提交AI 重新评估\n请输入: ",
	KeyErrorRiskWarning:       "⚠️ 风险提示: 错误反复出现可能表明 AI 陷入了死循环或遇到了无法解决的问题。\n    请关注错误内容，必要时选择 [C] 取消操作以避免潜在风险（如：将您的Token余额用尽或信息外泄）。",
	KeyCmdConfirmApprove:      "a",
	KeyCmdConfirmApproveAll:   "aa",
	KeyCmdConfirmCancel:       "c",
	KeyCmdConfirmModify:       "m",
	KeyCmdConfirmInvalid:      "无效输入，请直接回车批准、输入 a 本次都批准、输入 g 永久自动执行、输入 d 永久禁用、输入 c 取消、输入数字批准后续 N 次，或输入补充说明。",
	KeyCmdConfirmCancelled:    "已取消。",
	KeyCmdConfirmModifyHint:   "请输入补充说明，AI 将重新评估: ",
	KeyCmdConfirmDisableTool:  "此工具已设为永久自动执行（G 选项）",
	KeyCmdConfirmApproveG:     "g",
	KeyCmdConfirmApproveGDesc: "永久自动执行此工具",
	KeyCmdConfirmApproveD:     "d",
	KeyCmdConfirmApproveDDesc: "永久禁用此工具",
	KeyCmdConfirmDisableToolD: "此工具已被永久禁用（D 选项）",
	KeyCmdConfirmCountPrefix:  "✅ 已批准后续 ",
	KeyCmdConfirmCountSuffix:  " 次此方法调用",

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
	KeyHelpModel:        "    .model        - 多模型管理（add/list/remove/switch/info）",
	KeyHelpSection:      "    .section      - 自定义提示词节管理（add/list/remove）",
	KeyHelpMode:         "    .mode         - 工作模式管理（list/switch/create/edit）",
	KeyModeCurrent:      "当前工作模式: %s",
	KeyModeList:         "工作模式列表:",
	KeyModeNotFound:     "工作模式未找到",
	KeyModeExists:       "工作模式已存在",
	KeyModeSwitched:     "✅ 已切换到工作模式: %s",
	KeyModeAdded:        "✅ 已创建工作模式: %s",
	KeyModeRemoved:      "✅ 已删除工作模式: %s",
	KeyCol3WorkMode:     "工作模式",
	KeySectionAdded:     "✅ 节已添加: %s",
	KeySectionRemoved:   "✅ 节已删除: %s",
	KeySectionCleared:   "✅ 所有自定义节已清空",
	KeySectionNoSects:   "暂无自定义节",
	KeySectionList:      "节列表:",
	KeySectionInvalid:   "无效的节名称",

	// Settings help table
	KeySettingsHelpTitle:             "📋 .set 参数清单",
	KeySettingsColParam:              "参数名",
	KeySettingsColValues:             "可选项 / 值范围",
	KeySettingsColDesc:               "说明",
	KeySettingsDescAPIKey:            "设置 API 密钥",
	KeySettingsDescEndpoint:          "设置 API 端点 URL",
	KeySettingsDescModel:             "设置模型名称",
	KeySettingsDescTemp:              "设置温度参数（越高越随机）",
	KeySettingsDescMaxTokens:         "设置最大输出令牌数",
	KeySettingsDescShowThinking:      "显示 AI 思考过程",
	KeySettingsDescShowCommand:       "显示执行的系统命令",
	KeySettingsDescShowOutput:        "显示命令执行返回数据",
	KeySettingsDescConfirmCmd:        "执行命令前需确认",
	KeySettingsDescLog:               "日志开关",
	KeySettingsDescMaxIter:           "最大迭代次数（-1=不限）",
	KeySettingsDescMaxRetries:        "LLM 临时错误重试次数（默认 3）",
	KeySettingsDescResultMode:        "结果处理模式（minimal/explain/analyze/free）",
	KeySettingsDescName:              "设置 Agent 名称",
	KeySettingsDescDescription:       "设置 Agent 描述/专长",
	KeySettingsDescPrinciples:        "设置 Agent 核心原则",
	KeySettingsDescTopP:              "Top-P 采样参数（0.0 ~ 1.0，-1 不发送）",
	KeySettingsDescTopK:              "Top-K 采样参数（>= 1 的整数，-1 不发送）",
	KeySettingsDescRepetitionPenalty: "重复惩罚参数（0.0 ~ 2.0，-1 不发送）",
	KeySettingsDescTokenUsage:        "Token 用量显示模式（on=显示并发送, off=不显示但发送, none=不显示也不发送）",
	KeySettingsDescToolTimeout:       "工具调用超时（0=不限）",
	KeySettingsDescCmdTimeout:        "命令执行超时（0=不限）",
	KeySettingsDescLLMTimeout:        "LLM 请求超时（0=不限）",

	// New settings descriptions (ENHANCEMENT-126)
	KeySettingsDescLlmThinking:   "显示 LLM 思考过程",
	KeySettingsDescLlmContent:    "显示 LLM 返回的主要内容",
	KeySettingsDescTool:          "显示工具调用方法名",
	KeySettingsDescToolInput:     "显示工具调用输入参数",
	KeySettingsDescToolOutput:    "显示工具调用返回数据",
	KeySettingsDescCommandOutput: "显示命令执行返回数据",

	KeySettingsHelpFooter:   "💡 使用 .set <参数名> <值> 修改参数，例如: .set model deepseek-chat\n   .set db 查看/配置数据库（首次运行自动启动配置向导）",
	KeySettingsCurrentTitle: "当前配置:",

	// Memory enabled
	KeyCol3MemoryEnabled:     "记忆(on|off)",
	KeySettingsDescMemory:    "持久化记忆功能开关",
	KeyMemoryEnabledUpdated:  "✅ 记忆功能已设置为: %s",
	KeyCLIHelpMemoryEnabled:  "      --memory-enabled   启用持久化记忆功能（覆盖配置文件）",
	KeyCLIHelpMemoryDisabled: "      --memory-disabled  禁用持久化记忆功能（覆盖配置文件）",

	// Plan enabled
	KeyCol3PlanEnabled:     "任务计划(on|off)",
	KeySettingsDescPlan:    "任务计划功能开关",
	KeyPlanEnabledUpdated:  "✅ 任务计划功能已设置为: %s",
	KeyCLIHelpPlanEnabled:  "      --plan-enabled    启用任务计划功能（覆盖配置文件）",
	KeyCLIHelpPlanDisabled: "      --plan-disabled   禁用任务计划功能（覆盖配置文件）",

	// SubAgent enabled
	KeyCol3SubAgentEnabled:     "子代理(on|off)",
	KeySettingsDescSubAgent:    "子代理功能开关",
	KeySubAgentEnabledUpdated:  "✅ 子代理功能已设置为: %s",
	KeyCLIHelpSubAgentEnabled:  "      --subagent-enabled    启用子代理功能（覆盖配置文件）",
	KeyCLIHelpSubAgentDisabled: "      --subagent-disabled   禁用子代理功能（覆盖配置文件）",

	// ToolCall enabled
	KeyCLIHelpToolCallEnabled:  "      --toolcall-enabled   启用工具调用功能（覆盖配置文件）",
	KeyCLIHelpToolCallDisabled: "      --toolcall-disabled  禁用工具调用功能（覆盖配置文件）",

	// Config show column 3 labels
	KeyCol3Provider:     "提供商(deepseek/qwen/xiaomi/zhipu/openai)",
	KeyCol3Endpoint:     "API服务器",
	KeyCol3Model:        "模型ID",
	KeyCol3Temperature:  "温度(0.0 ~ 2.0)",
	KeyCol3MaxTokens:    "最大输出令牌数(-1[不发送] ~ N)",
	KeyCol3MaxIter:      "最大迭代次数(-1 ~ N)",
	KeyCol3MaxRetries:   "LLM 重试次数(0 ~ N)",
	KeyCol3Thinking:     "显示思考过程(on|off)",
	KeyCol3Command:      "显示命令(on|off)",
	KeyCol3Output:       "显示命令输出(on|off)",
	KeyCol3Confirm:      "全局确认模式(confirm/auto/disabled/custom)",
	KeyCol3ToolTimeout:  "工具调用超时(0 ~ N 秒)",
	KeyCol3CmdTimeout:   "命令执行超时(0 ~ N 秒)",
	KeyCol3LLMTimeout:   "LLM 请求超时(0 ~ N 秒)",
	KeyCol3Log:          "日志级别(debug/info/warn/error/off)",
	KeyCol3ResultMode:   "结果模式(minimal/explain/analyze/free)",
	KeyCol3APIKey:       "API 密钥",
	KeyCol3Name:         "Agent 名称",
	KeyCol3Desc:         "Agent 描述",
	KeyCol3Principles:   "Agent 核心原则",
	KeyCol3Vision:       "视觉识别(on|off)",
	KeyCol3ContextLimit: "对话上下文限制(0=不包含历史, N=最近N条, -1=不限制)",

	// New column 3 labels (ENHANCEMENT-126)
	KeyCol3LlmThinking:   "显示 LLM 思考过程(on|off)",
	KeyCol3LlmContent:    "显示 LLM 内容(on|off)",
	KeyCol3Tool:          "显示工具调用名(on|off)",
	KeyCol3ToolInput:     "显示工具调用参数(on|off)",
	KeyCol3ToolOutput:    "显示工具返回数据(on|off)",
	KeyCol3CommandOutput: "显示命令返回数据(on|off)",

	// Context limit
	KeyContextLimitLabel:    "对话上下文限制",
	KeyContextLimitUpdated:  "✅ 对话上下文限制已设置为: %d（将包含最近 %d 条对话消息）",
	KeySettingsDescCtxLimit: "设置对话上下文限制（0=不包含历史, N=最近N条, -1=所有）",
	KeyConfigContextLimit:   "  对话上下文限制: %s\n",

	// History list
	KeyListTitle:     "📋 历史任务列表:",
	KeyListEmpty:     "暂无历史记录。",
	KeyListReExecute: "输入编号重新执行，或输入其他内容继续。",
	KeyListInvalid:   "无效编号，请输入 1-%d 之间的数字。",
	KeyLastUsage:     "用法: .last [N] — 显示最近 N 条历史记录（默认 10）",
	KeyFirstUsage:    "用法: .first [N] — 显示最早 N 条历史记录（默认 10）",
	KeyListUsage:     "用法: .list [start] [end] — 显示历史记录范围（序号从 1 开始）",

	// Session
	KeySessionTitle:          "📋 当前会话",
	KeySessionTotalMessages:  "总消息数",
	KeySessionRoleSystem:     "系统消息",
	KeySessionRoleUser:       "用户消息",
	KeySessionRoleAssistant:  "助手消息",
	KeySessionRoleTool:       "工具消息",
	KeySessionContextLimit:   "上下文限制",
	KeySessionNoHistory:      "无历史记录（仅当前输入）",
	KeySessionModel:          "模型",
	KeySessionProvider:       "提供商",
	KeySessionAgentName:      "Agent 名称",
	KeySessionRecentMessages: "最近消息（最多显示 10 条）:",

	// History command
	KeyHelpHistory: "    .history            - 查看用户输入命令历史（.history last/first [N]）",
	KeyHelpSession: "    .session            - 查看当前会话信息（消息数、角色分布等）",
	KeyHistoryUsage: `📋 .history 子命令:
  .history [start] [end]    查看历史范围
  .history last [N]         查看最近 N 条（默认 10）
  .history first [N]        查看最早 N 条（默认 10）
  输入编号可重新执行历史命令`,

	// Memory search config
	KeyCol3MemorySearchMaxContentLen: "记忆搜索内容最大长度",
	KeyCol3MemorySearchMaxResults:    "记忆搜索最大结果数",
	KeySettingsDescMemSearchMaxLen:   "记忆搜索结果中内容的最大字符长度，超长截断（默认 512）",
	KeySettingsDescMemSearchMaxRes:   "记忆搜索返回的最大结果数（默认 100）",

	// Thinking enabled
	KeyCol3ThinkingEnabled:   "AI 思考功能(on|off)",
	KeyCol3ReasoningEffort:   "推理努力程度(low/medium/high)",
	KeyCol3ToolCallEnabled:   "工具调用(on|off)",
	KeyCol3MaxModelLen:       "模型最大上下文长度(tokens)",
	KeyCol3TopP:              "Top-P 采样参数",
	KeyCol3TopK:              "Top-K 采样参数",
	KeyCol3RepetitionPenalty: "重复惩罚参数",
	KeyCol3TokenUsage:        "Token 用量显示模式(on/off/none)",

	// Settings group titles
	KeySettingsGroupIdentity: "[ 身份与个性 ]",
	KeySettingsGroupModel:    "[ 智能体设置 ]",

	// Model selection column 3 labels
	KeyCol3DefaultToolModel:     "默认工具模型",
	KeyCol3DefaultVisionModel:   "默认视觉模型",
	KeyCol3DefaultProblemModel:  "默认问题解决模型",
	KeySettingsGroupDisplay:     "[ 显示与输出 ]",
	KeySettingsGroupSafety:      "[ 安全与确认 ]",
	KeySettingsGroupMemory:      "[ 记忆与上下文 ]",
	KeySettingsGroupTask:        "[ 任务与子代理 ]",
	KeySettingsGroupSearchDebug: "[ 搜索与调试 ]",

	// Error settings column 3 labels
	KeyCol3ErrorMaxSingleCount: "相同错误最大出现次数",
	KeyCol3ErrorMaxTypeCount:   "最大错误类型数",

	// Loop detection settings (FIX-179)
	KeyCol3LoopDetectEnabled:     "循环检测(on|off)",
	KeyCol3LoopDetectThreshold:   "循环检测阈值(重复次数)",
	KeyLoopDetectFeedback:        "⚠️ 检测到你的输出陷入了死循环（连续重复相似内容，详见后面错误信息）。\n你需要先停下来，休息一下换换脑子。深呼吸，我来指导你脱离出去。首先，围绕用户任务的终极目标（即<task></task>中的内容）进行思考，评估一下距离目标偏离了多少，然后换个思路和方向解决问题。\n\n错误信息：%s",
	KeyToolCallLoopFeedback:      "⚠️ 检测到工具调用陷入了循环: 工具「%s」在连续多轮迭代中使用了完全相同的参数。请立即停止当前做法，换用完全不同的工具或方法。如果：\n1. 需要读文件，试试 search_files 来找线索\n2. 需要修改代码，先完整理解上下文再动手\n3. 不确定怎么做，停下来问用户更多信息\n\n记住：保持冷静，换个思路，不要重复做同样的事。",
	KeyCol3LoopDetectMaxWindow:   "循环检测滑动窗口大小",
	KeySettingsDescLoopDetect:    "循环检测开关，检测LLM输出是否陷入死循环",
	KeySettingsDescLoopThreshold: "循环检测阈值，连续重复内容触发干预的次数（默认 5）",
	KeySettingsDescLoopWindow:    "循环检测滑动窗口大小，用于检查重复模式的历史块数（默认 20）",
	KeyLoopDetectEnabledUpdated:  "✅ 循环检测已设置为: %s",
	KeyCLIHelpLoopDetectEnabled:  "      --loop-detect-enabled  启用循环检测功能（覆盖配置文件）",
	KeyCLIHelpLoopDetectDisabled: "      --loop-detect-disabled 禁用循环检测功能（覆盖配置文件）",

	// Settings confirmation (FEATURE-131)
	KeySettingsConfirmTitle:          "⚠️ co-shell 将自主修改系统参数",
	KeySettingsConfirmRiskWarning:    "⚠️ 风险提示：修改系统参数可能影响 co-shell 的行为和稳定性，请谨慎操作。",
	KeySettingsConfirmPrompt:         "请选择操作:\n  [A] 批准执行\n  [C] 取消\n  其他输入: 暂停并补充说明\n请输入: ",
	KeySettingsConfirmRejected:       "用户已拒绝修改系统参数",
	KeySettingsConfirmRejectedResult: "用户拒绝了参数修改请求，未应用任何更改。",
	KeySettingsConfirmApplied:        "已成功应用以下参数修改",
	KeySettingsConfirmFailed:         "以下参数修改失败",
	KeySettingsConfirmResult:         "参数修改完成：%d 个成功，%d 个失败",
	KeySettingsConfirmPaused:         "用户暂停了操作，补充说明",

	// Emoji prefix keys (ENHANCEMENT-131)
	KeyEmojiPrefixUser:        "[👤]> ",
	KeyEmojiPrefixAssistant:   "[🐚]> ",
	KeyEmojiPrefixToolInput:   "[⚙️]< ",
	KeyEmojiPrefixToolOutput:  "[⚙️]> ",
	KeyEmojiPrefixCmdInput:    "[🔴]< ",
	KeyEmojiPrefixCmdOutput:   "[🔴]> ",
	KeyEmojiPrefixInfo:        "[ℹ️] ",
	KeyEmojiPrefixError:       "[❌] ",
	KeyEmojiPrefixWarning:     "[⚠️] ",
	KeyEmojiPrefixSuccess:     "[✅] ",
	KeyEmojiPrefixThinking:    "[💬] ",
	KeyEmojiPrefixOutputTitle: "[📋] 命令输出:",
	KeyEmojiPrefixOutputSep:   "────────────────────────────────────────────",

	// Emoji enabled settings
	KeyCol3EmojiEnabled:     "表情符号(on|off)",
	KeySettingsDescEmoji:    "使用表情符号区分不同角色的输出",
	KeyEmojiEnabledUpdated:  "✅ 表情符号已设置为: %s",
	KeyCLIHelpEmojiEnabled:  "      --emoji-enabled on|off  启用表情符号（覆盖配置文件）",
	KeyCLIHelpEmojiDisabled: "      --emoji-disabled  禁用表情符号（覆盖配置文件）",

	// Show logo
	KeyCLIHelpShowLogo: "      --show-logo on|off      显示 ASCII art Logo（覆盖配置文件）",

	// Init capabilities/rules
	KeyCLIHelpInputMode: "      --input-mode         REPL 输入模式（enhanced=增强交互/stdio=标准输入，覆盖配置文件）",

	// Context start mode (FEATURE-103)
	KeyCol3ContextStartMode:       "上下文起始模式(window/task/smart)",
	KeySettingsDescCtxStart:       "设置上下文起始模式（window=固定窗口, task=任务模式, smart=智能调整）",
	KeyContextStartUpdated:        "✅ 上下文起始模式已设置为: %s",
	KeyCLIHelpContextStart:        "      --context-start <mode>   上下文起始模式（window/task/smart，覆盖配置文件）",
	KeyContextStartWindow:         "window",
	KeyContextStartWindowDesc:     "固定窗口模式，上下文为最后 N 条消息",
	KeyContextStartTask:           "task",
	KeyContextStartTaskDesc:       "任务模式，上下文指针随任务边界自动移动",
	KeyContextStartSmart:          "smart",
	KeyContextStartSmartDesc:      "智能模式，LLM 可通过 adjust_context_start 工具自行决定上下文起始位置",
	KeyAdjustContextStartDesc:     "调整上下文起始指针的位置。让 LLM 根据当前上下文内容动态决定保留多少历史对话，忽略不相关的早期对话。仅在 smart 模式下可用。",
	KeyAdjustContextStartResult:   "✅ 上下文起始指针已从索引 %d 调整到索引 %d，新上下文包含 %d 条消息",
	KeyAdjustContextStartNotSmart: "⚠️ adjust_context_start 工具仅在 smart 模式下可用，当前模式为: %s",
	KeyAdjustContextStartPrompt:   "上下文起始索引",

	// Database (PostgreSQL) related keys (FEATURE-86)
	KeyDBConnecting:        "正在连接 PostgreSQL 数据库 %s:%d/%s...",
	KeyDBConnected:         "✅ PostgreSQL 数据库连接成功 (%s:%d/%s)",
	KeyDBConnectFailed:     "⚠️ PostgreSQL 数据库连接失败: %v",
	KeyDBFallbackToLocal:   "⚠️ 将使用本地 bbolt 存储作为替代",
	KeyDBConfigLabel:       "数据库配置",
	KeyDBHostLabel:         "数据库地址",
	KeyDBPortLabel:         "数据库端口",
	KeyDBNameLabel:         "数据库名称",
	KeyDBSchemaLabel:       "数据库 Schema",
	KeyDBStatusLabel:       "状态",
	KeyDBStatusConnected:   "✅ 已连接",
	KeyDBStatusFailed:      "❌ 连接失败",
	KeyDBStatusNone:        "未连接",
	KeyDBTimeoutLabel:      "连接超时（秒）",
	KeyDBStatusCmd:         "重新检测数据库连接状态",
	KeyDBMigrateDescMemory: "从本地 bbolt 增量同步 memory 和 history 到 PostgreSQL",
	KeyDBUserLabel:         "数据库用户",
	KeyDBPasswordLabel:     "数据库密码",
	KeyDBEnabledLabel:      "数据库存储",
	KeyDBNotConfigured:     "未配置 PostgreSQL 数据库，使用本地 bbolt 存储",
	KeyDBMigrating:         "正在迁移数据到 PostgreSQL...",
	KeyDBMigrationComplete: "✅ 数据迁移完成",
	KeyDBMigrationFailed:   "❌ 数据迁移失败: %v",
	KeyDBSubCmdDesc:        "数据库持久化存储（.set db 查看详情）",
	KeyDBInitDesc:          "初始化 PostgreSQL 数据库（删除所有表并重建）",
	KeyDBMigrateDesc:       "从本地 bbolt 迁移数据到 PostgreSQL",

	// DB backup/restore (FEATURE-86)
	KeyDBBackupTitle:    "备份 PostgreSQL 数据到 CSV 文件",
	KeyDBRestoreTitle:   "从备份恢复 PostgreSQL 数据",
	KeyDBBackupDir:      "正在备份数据库到 %s/ ...",
	KeyDBRestoreDir:     "正在从 %s/ 恢复数据...",
	KeyDBBackupDone:     "✅ 数据库备份完成! 备份文件保存在 %s/",
	KeyDBRestoreDone:    "✅ 数据恢复完成! 已从 %s/ 恢复数据",
	KeyDBBackupFailed:   "备份失败: %w",
	KeyDBRestoreFailed:  "恢复失败: %w",
	KeyDBNoBackupFound:  "❌ 未找到任何备份",
	KeyDBSelectBackup:   "请选择要恢复的备份编号 (输入 q 取消): ",
	KeyDBRestoreWarning: "⚠️  恢复数据将覆盖 PostgreSQL 数据库中所有现有数据!",
	KeyDBRestoreConfirm: "是否继续恢复? (y/n, 默认: n): ",
	KeyDBBackupCancel:   "❌ 已取消备份",
	KeyDBRestoreCancel:  "❌ 已取消恢复",

	// Shell session
	KeyCol3ShellSessionEnabled:         "持续Shell会话(on|off)",
	KeyCol3ShellSessionTimeout:         "持续Shell超时(0=无限制)",
	KeySettingsDescShellSessionEnabled: "控制是否启用持续Shell会话（启用时替换 execute_command）",
	KeySettingsDescShellSessionTimeout: "Shell 命令超时时间（秒，0 表示无限制）",

	// Browser config (FEATURE-200)
	KeyCol3BrowserEnabled:          "浏览器(on|off)",
	KeyCol3BrowserPort:             "浏览器端口(默认9222)",
	KeyCol3BrowserHeadless:         "无头模式(on|off)",
	KeySettingsDescBrowserEnabled:  "启用浏览器自动化工具（通过CDP控制Chrome）",
	KeySettingsDescBrowserPort:     "Chrome DevTools Protocol调试端口",
	KeySettingsDescBrowserHeadless: "是否以无头模式运行Chrome（无窗口）",

	// Confirm-tool mode descriptions (FEATURE-200)
	KeyModeConfirmDesc:  "需要人工确认",
	KeyModeAutoDesc:     "自动批准执行",
	KeyModeDisabledDesc: "禁用此工具",
	KeyModeCustomDesc:   "工具单独定制",

	// Tool call mode (FEATURE-182)
	KeyToolCallMode:         "工具调用模式",
	KeyToolCallModeUpdated:  "✅ 工具调用模式已设置为: %s",
	KeyInvalidToolCallMode:  "无效的工具调用模式: %s（可选值: openai, xml）",
	KeyCol3ToolCallMode:     "工具调用模式(openai|xml)",
	KeySettingsDescToolMode: "工具调用模式（openai: 标准API, xml: 内嵌XML标签）",
	KeyCLIHelpToolCallMode:  "      --toolcall-mode   工具调用模式（openai/xml，覆盖配置文件）",

	// Config wizard group titles (FEATURE-200)
	KeyWizardGroupModelMgr:   "[ 模型管理 ]",
	KeyWizardGroupWorkMode:   "[ 工作模式与提示词 ]",
	KeyWizardGroupMultimodal: "[ 多模态与MCP ]",
	KeyWizardGroupDevTools:   "[ 开发者工具 ]",
	KeyWizardGroupMCPRule:    "[ MCP 与规则 ]",

	// Config wizard (FEATURE-197)
	KeyHelpConfig:             "    .config     - 配置向导（菜单式配置）",
	KeyConfigWizardTitle:      "╔══════════════════════════════════════════════╗\n║         co-shell 配置向导                    ║\n╚══════════════════════════════════════════════╝",
	KeyConfigWizardIntro:      "请选择配置分类，逐级进入设置参数。",
	KeyConfigGroupTitle:       "── 配置分类 ──────────────────────────────",
	KeyConfigGroupPrompt:      "  [P] 退出         请选择 [1-%d/P]: ",
	KeyConfigParamPrompt:      "  [P] 返回上一级  请选择 [1-%d/P]: ",
	KeyConfigValueLabParam:    "  参数: %s",
	KeyConfigValueLabCurrent:  "  当前值: %s",
	KeyConfigValuePrompt:      "  输入新值（回车保持不变，[P] 返回，[Q] 退出）: ",
	KeyConfigExited:           "配置向导已退出。",
	KeyConfigValueUnchanged:   "  值未改变。",
	KeyConfigInvalidChoice:    "无效选择，请重新输入。",
	KeyConfigValOnOff:         "请输入 on 或 off",
	KeyConfigValMinExplAnFree: "请输入 minimal/explain/analyze/free",
	KeyConfigValWinTaskSmart:  "请输入 window/task/smart",
	KeyConfigValDebugOff:      "请输入 debug/info/warn/error/off",
	KeyConfigValUnlimited:     "不限制",
	KeyConfigValCtxLimit:      "请输入正整数、off 或 unlimited",
	KeyConfigValCtxStart:      "请输入 window/task/smart",

	// Simulate (FEATURE-218)
	KeyHelpSimulate:        "    .simulate      - 模拟 LLM 方法调用，进行解析和执行测试",
	KeySimulatePromptInput: "请输入要模拟的 LLM 方法调用内容: ",
	KeySimulateNoContent:   "内容为空，已取消",
	KeyContinuePrompt: `你没有调用任何工具就返回了纯文本回答，这不是有效的任务完成方式。请立即决定下一步：
- 如果任务确实已经全部完成（请深思熟虑后确认），必须调用 attempt_completion 工具提交最终结果。
- 如果任务尚未完成或你还有任何疑问，请调用合适的工具继续执行。

注意：只有当你确认所有任务步骤都已成功完成、结果已向用户呈现后，才可以调用 attempt_completion。一旦调用，系统将结束当前任务。`,
	KeySimulatePartial:       "部分执行完成（有错误发生）",
	KeySimulateParsingResult: "解析到 %d 个方法调用",
	KeySimulateLabelArgs:     "参数: ",
	KeySimulateLabelError:    "错误",
	KeySimulateLabelSuccess:  "✅ 执行成功",
	KeySimulateLabelResult:   "结果: ",
}
