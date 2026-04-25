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
// Package i18n - English translations.
package i18n

var enMessages = map[string]string{
	// General
	KeyCancelled:      "Cancelled",
	KeySetupCancelled: "❌ Setup incomplete, exiting.",
	KeyYes:            "Yes",
	KeyNo:             "No",
	KeyOn:             "On",
	KeyOff:            "Off",
	KeyError:          "Error",
	KeyWarning:        "Warning",
	KeySuccess:        "Success",
	KeyUnlimited:      "Unlimited",
	KeyDefault:        "Default",

	// Wizard
	KeyWizardTitle:       "🔧 co-shell API Setup Wizard",
	KeyWizardDescription: "Configure LLM API before using co-shell.\nPress ESC at any time to exit.",
	KeySelectProvider:    "📌 Select LLM Provider (Enter for default, Tab to show list)",
	KeyProviderSelected:  "📌 Selected provider: %s",
	KeyProviderLabel:     "📌 Provider",
	KeyEndpointLabel:     "📌 API Endpoint",
	KeyEndpointRequired:  "📌 API Endpoint (required)",
	KeyAPIKeyLabel:       "📌 API Key (required)",
	KeyAPIKeyRequired:    "🔑 Enter API Key to fetch available models.",
	KeyModelName:         "📌 Model Name",
	KeyAPITest:           "🔄 Testing API connection...",
	KeyAPITestOK:         " ✅ Connection successful!",
	KeyAPITestFail:       "\n❌ Connection test failed: %v\n",
	KeyFetchModels:       "🔄 Fetching available models...",
	KeyFetchModelsOK:     " ✅ Found %d available models!",
	KeyFetchModelsFail:   "\n❌ Failed to fetch models: %v\n",
	KeyEndpointTest:      "🔄 Testing endpoint connectivity...",
	KeyEndpointTestOK:    " ✅ Endpoint reachable!",
	KeyEndpointTestFail:  "\n❌ Endpoint connection failed: %v\n",
	KeyEndpointRetry:     "⚠️ Please check the endpoint URL and re-enter.",
	KeyAPIKeyGetPrompt:   "🔑 An API key is required to call the %s API.",
	KeyAPIKeyManualGet:   "   Manually get an API key and paste it below.",
	KeyAPIKeyOpenPage:    "   Open %s API key page?",
	KeyAPIKeyOpeningPage: "   🔗 Opening: %s",
	KeyAPIKeyManualOpen:  "   Please visit: %s",
	KeyEmptyField:        "⚠️ This field cannot be empty, please re-enter.",
	KeyInvalidChoice:     "⚠️ Invalid choice, please enter number 1-%d or provider name.",
	KeyConfigSaved:       "✅ Configuration saved to ~/.co-shell/config.json",

	// REPL
	KeyGoodbye:     "\n👋 Goodbye!",
	KeyExit:        "exit",
	KeyCleanup:     "Cleaning up...",
	KeyCleanupDone: "Done.",
	KeyUnknownCmd:  "Unknown command: %s",
	KeyCmdError:    "Command error",
	KeyCmdExecFail: "Command execution failed",
	KeyAgentFail:   "Agent error",
	KeyAgentHint:   "Type help or .help to see available commands.",
	KeyOutputTitle: "=== Command Output ===",
	KeyOutputSep:   "---",
	KeyToolCall:    "Tool call",

	// Settings - Labels
	KeySettingsLabel:        "Settings",
	KeyAPIKeyLabelSetting:   "API Key",
	KeyEndpointLabelSetting: "Endpoint",
	KeyModelLabel:           "Model",
	KeyTempLabel:            "Temperature",
	KeyMaxTokensLabel:       "Max Tokens",
	KeyProviderLabelSetting: "Provider",

	// Settings - Messages
	KeySettingsUpdated:  "✅ Settings updated.",
	KeyEndpointUpdated:  "✅ Endpoint updated.",
	KeyModelUpdated:     "✅ Model updated.",
	KeyTempUpdated:      "✅ Temperature set to %.1f.",
	KeyMaxTokensUpdated: "✅ Max tokens set to %d.",
	KeyShowThinking:     "Show thinking: %s",
	KeyShowCommand:      "Show command: %s",
	KeyShowOutput:       "Show output: %s",
	KeyLogEnabled:       "Logging: %s",
	KeyMaxIterations:    "Max iterations: %s",
	KeyProviderUpdated:  "✅ Provider updated.",

	// Settings - Config Show
	KeyConfigTitle:         "Current Configuration:",
	KeyConfigProvider:      "  Provider:      %s\n",
	KeyConfigEndpoint:      "  Endpoint:      %s\n",
	KeyConfigModel:         "  Model:         %s\n",
	KeyConfigTemperature:   "  Temperature:   %.1f\n",
	KeyConfigMaxTokens:     "  Max Tokens:    %d\n",
	KeyConfigMaxIterations: "  Max Iterations: %s\n",
	KeyConfigShowThinking:  "  Show Thinking: %s\n",
	KeyConfigShowCommand:   "  Show Command:  %s\n",
	KeyConfigShowOutput:    "  Show Output:   %s\n",
	KeyConfigLogging:       "  Logging:       %s\n",
	KeyConfigMCPServers:    "  MCP Servers:   %s\n",
	KeyConfigRules:         "  Rules:         %s\n",

	// MCP
	KeyMCPAlreadyExists: "MCP server already exists: %s",
	KeyMCPAdded:         "✅ MCP server added: %s",
	KeyMCPRemoved:       "✅ MCP server removed: %s",
	KeyMCPNotFound:      "MCP server not found: %s",
	KeyMCPEnabled:       "✅ MCP server enabled: %s",
	KeyMCPDisabled:      "✅ MCP server disabled: %s",
	KeyMCPEmpty:         "No MCP servers configured.",
	KeyMCPListTitle:     "MCP Servers:",

	// Rule
	KeyRuleAdded:   "✅ Rule added: %s",
	KeyRuleRemoved: "✅ Rule removed: %s",
	KeyRuleCleared: "✅ All rules cleared.",
	KeyRuleInvalid: "Invalid rule index: %s",
	KeyRuleNoRules: "No rules configured.",

	// Memory
	KeyMemorySaved:   "✅ Memory saved: %s = %s",
	KeyMemoryDeleted: "✅ Memory deleted: %s",
	KeyMemoryCleared: "✅ All memory cleared.",
	KeyMemoryEmpty:   "No memory entries.",
	KeyMemoryGet:     "%s = %s",

	// Context
	KeyContextShow:  "Current Context:",
	KeyContextEmpty: "Context is empty.",
	KeyContextReset: "✅ Context reset.",
	KeyContextSet:   "✅ Context set: %s = %s",

	// Agent
	KeyNoopClientError: "LLM client not configured. Please run the setup wizard or configure API settings.",

	// Settings - Extended
	KeySettingsLabelLog:           "Log",
	KeySettingsLabelShowThinking:  "Show Thinking",
	KeySettingsLabelShowCommand:   "Show Command",
	KeySettingsLabelShowOutput:    "Show Output",
	KeySettingsLabelMaxIterations: "Max Iterations",
	KeySettingsLabelProvider:      "Provider",

	// Config format
	KeyConfigFormat: `LLM Configuration:
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
	KeyWelcomeTip: "💡 Type natural language commands or system commands directly.\n   Type .help for available commands.\n   Example: \"List files in current directory\"",

	KeyUnknownCommand: "❌ Unknown command: %s",
	KeyCmdFailed:      "Command failed",
	KeyProcessFailed:  "Processing failed",
	KeyCheckConfig:    "💡 Check your API configuration with .settings",
	KeyCleaningUp:     "Cleaning up...",
	KeyDone:           " Done.",

	// Help
	KeyHelpTitle:        "📖 co-shell Help",
	KeyHelpNLTitle:      "Natural Language Commands:",
	KeyHelpNLDesc:       "    Just type what you want to do, and co-shell will figure it out.",
	KeyHelpBuiltinTitle: "Built-in Commands:",
	KeyHelpSettings:     "    .set                - View/change LLM API settings",

	KeyHelpMCP:          "    .mcp                - Manage MCP server connections",
	KeyHelpRule:         "    .rule               - Manage global rules",
	KeyHelpMemory:       "    .memory             - Manage memory and knowledge",
	KeyHelpContext:      "    .context            - Manage conversation context",
	KeyHelpHelp:         "    .help               - Show this help message",
	KeyHelpExit:         "    .exit               - Exit co-shell",
	KeyHelpExampleTitle: "Examples:",
	KeyHelpExample1:     "    ❯ \"List files in current directory\"",
	KeyHelpExample2:     "    ❯ \"What is the weather today?\"",
	KeyHelpExample3:     "    ❯ ls -la",
	KeyHelpExample4:     "    ❯ .mcp add filesystem npx @modelcontextprotocol/server-filesystem /tmp",
	KeyHelpExample5:     "    ❯ .rule add \"Always confirm before deleting files\"",

	// CLI Help
	KeyCLIHelpTitle:     "co-shell v%s - Intelligent Command-Line Shell",
	KeyCLIHelpUsage:     "Usage:",
	KeyCLIHelpUsageREPL: "  co-shell [options]                    Start interactive REPL",
	KeyCLIHelpUsageCmd:  "  co-shell [options] <command>          Execute a single command and exit",
	KeyCLIHelpOptions:   "Options:",
	KeyCLIHelpConfig:    "  -c, --config <path>    Config file path (default: ~/.co-shell/config.json)",
	KeyCLIHelpModel:     "  -m, --model <name>     Temporarily override model (overrides config)",
	KeyCLIHelpEndpoint:  "  -e, --endpoint <url>   Temporarily override API endpoint (overrides config)",
	KeyCLIHelpAPIKey:    "  -k, --api-key <key>    Temporarily override API Key (overrides config)",
	KeyCLIHelpLang:      "      --lang <code>      Set language (zh/en, auto-detect by default)",
	KeyCLIHelpLog:       "      --log on|off       Temporarily enable/disable logging (overrides config)",
	KeyCLIHelpMaxIter:   "      --max-iterations   Max iterations (-1 for unlimited, default 10)",
	KeyCLIHelpVersion:   "  -v, --version          Show version information",
	KeyCLIHelpHelp:      "  -h, --help             Show this help message",
	KeyCLIHelpExamples:  "Examples:",
	KeyCLIHelpEx1:       "  co-shell                             Start interactive REPL",
	KeyCLIHelpEx2:       `  co-shell "list files in current dir"  Execute natural language command`,
	KeyCLIHelpEx3:       `  co-shell "cat ~/.co-shell/config.json"  Execute system command`,
	KeyCLIHelpEx4:       "  co-shell -m gpt-4o hello              Specify model and execute command",
	KeyCLIHelpEx5:       "  co-shell -k sk-xxxx --log off         Temporarily set API Key and disable logging",
	KeyCLIHelpEx6:       "  co-shell --lang zh                     Start with Chinese interface",
	KeyCLIHelpEx7:       "  co-shell --max-iterations 20 list files  Set max iterations and execute command",

	// Command Confirmation
	KeyCmdConfirmTitle:       "⚡ About to execute command: %s",
	KeyCmdConfirmDisabled:    "Command confirmation: Off",
	KeyCmdConfirmEnabled:     "Command confirmation: On",
	KeyCmdConfirmDisableWarn: "⚠️ Warning: Disabling command confirmation will allow AI to execute commands directly without your approval. This may pose security risks (e.g., accidental file deletion, infinite loops). Proceed with caution.",

	KeyCmdConfirmPrompt:     "Choose an action:\n  [Enter] Approve and execute\n  [A] Approve all for this request\n  [C] Cancel\n  Other input: supplementary instructions for AI to re-evaluate\nEnter: ",
	KeyCmdConfirmApprove:    "a",
	KeyCmdConfirmApproveAll: "aa",
	KeyCmdConfirmCancel:     "c",
	KeyCmdConfirmModify:     "m",
	KeyCmdConfirmInvalid:    "Invalid input. Press Enter to approve, enter a to approve all, enter c to cancel, or type supplementary instructions.",
	KeyCmdConfirmCancelled:  "Cancelled.",
	KeyCmdConfirmModifyHint: "Enter additional instructions for the AI to re-evaluate: ",

	// Disclaimer
	KeyDisclaimerTitle: "⚠️ Disclaimer",

	KeyDisclaimerBody: `co-shell is an intelligent command-line tool powered by Large Language Models (LLM).
AI models may generate and execute dangerous commands, including but not limited to:

  • Deleting files or directories (e.g., rm -rf /)
  • Formatting disks (e.g., mkfs, format)
  • Modifying critical system configurations (e.g., /etc/passwd, /etc/shadow)
  • Shutting down or rebooting the system (e.g., shutdown, reboot)
  • Downloading and executing untrusted programs
  • Leaking sensitive information (e.g., API Keys, passwords, private keys)

By continuing to use this software, you acknowledge that you fully understand
the above risks and agree to assume all responsibility for any loss or damage
that may result from using this program. The developers and publishers assume
no liability whatsoever.`,
	KeyDisclaimerPrompt:  "Do you accept the above disclaimer and wish to continue? [Y/n] ",
	KeyDisclaimerYes:     "y",
	KeyDisclaimerNo:      "n",
	KeyDisclaimerRefused: "You have declined the disclaimer. Exiting.",

	// Wizard command
	KeyWizardCmdRunning: "🔄 Starting API setup wizard...\n",
	KeyWizardCmdDone:    "✅ API setup wizard completed.\n",
	KeyHelpWizard:       "    .wizard        - Restart the API setup wizard",

	// Settings help table
	KeySettingsHelpTitle:        "📋 .set Parameter List",
	KeySettingsColParam:         "Parameter",
	KeySettingsColValues:        "Values / Range",
	KeySettingsColDesc:          "Description",
	KeySettingsDescAPIKey:       "Set API key",
	KeySettingsDescEndpoint:     "Set API endpoint URL",
	KeySettingsDescModel:        "Set model name",
	KeySettingsDescTemp:         "Set temperature (higher = more random)",
	KeySettingsDescMaxTokens:    "Set max output tokens",
	KeySettingsDescShowThinking: "Show AI thinking process",
	KeySettingsDescShowCommand:  "Show executed system commands",
	KeySettingsDescShowOutput:   "Show command execution output",
	KeySettingsDescConfirmCmd:   "Confirm before executing commands",
	KeySettingsDescLog:          "Enable/disable logging",
	KeySettingsDescMaxIter:      "Max iterations (-1=unlimited)",
	KeySettingsDescToolTimeout:  "Tool call timeout (0=unlimited)",
	KeySettingsDescCmdTimeout:   "Command timeout (0=unlimited)",
	KeySettingsDescLLMTimeout:   "LLM request timeout (0=unlimited)",
	KeySettingsHelpFooter:       "💡 Use .set <parameter> <value> to modify, e.g.: .set model gpt-4o",
	KeySettingsCurrentTitle:     "Current Configuration:",

	// Config show column 3 labels
	KeyCol3Provider:    "provider(deepseek/qwen/openai)",
	KeyCol3Endpoint:    "API server",
	KeyCol3Model:       "model ID",
	KeyCol3Temperature: "temperature(0.0 ~ 2.0)",
	KeyCol3MaxTokens:   "max output tokens(1 ~ N (unlimited))",
	KeyCol3MaxIter:     "max iterations(-1 ~ N)",
	KeyCol3Thinking:    "show thinking(on|off)",
	KeyCol3Command:     "show command(on|off)",
	KeyCol3Output:      "show output(on|off)",
	KeyCol3Confirm:     "confirm command(on|off)",
	KeyCol3ToolTimeout: "tool timeout(0 ~ N sec)",
	KeyCol3CmdTimeout:  "cmd timeout(0 ~ N sec)",
	KeyCol3LLMTimeout:  "LLM timeout(0 ~ N sec)",
	KeyCol3Log:         "logging(on|off)",
	KeyCol3ResultMode:  "result mode(minimal/explain/analyze/free)",
	KeyCol3APIKey:      "API key",

	// History list
	KeyListTitle:     "📋 History Tasks:",
	KeyListEmpty:     "No history entries.",
	KeyListReExecute: "Enter a number to re-execute, or type something else to continue.",
	KeyListInvalid:   "Invalid number, please enter a number between 1-%d.",
	KeyLastUsage:     "Usage: .last [N] — Show last N history entries (default 10)",
	KeyFirstUsage:    "Usage: .first [N] — Show first N history entries (default 10)",
	KeyListUsage:     "Usage: .list [start] [end] — Show history range (index starts from 1)",

	// Custom
	KeyCustom: "Custom",
}
