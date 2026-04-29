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
// Package i18n provides internationalization support for co-shell.
// It supports Chinese (zh) and English (en) with easy extensibility.
//
// Language selection priority:
//  1. --lang CLI flag (highest priority)
//  2. LANG / LC_ALL environment variable
//  3. Default to Chinese (zh)
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang defines a language code.
type Lang string

const (
	LangZH Lang = "zh"
	LangEN Lang = "en"
)

// currentLang holds the currently active language.
var (
	mu          sync.RWMutex
	currentLang Lang = LangZH
)

// DetectLang detects the user's preferred language from environment.
func DetectLang() Lang {
	env := os.Getenv("LANG")
	if env == "" {
		env = os.Getenv("LC_ALL")
	}
	if env == "" {
		return LangZH
	}

	env = strings.ToLower(env)
	switch {
	case strings.HasPrefix(env, "zh"):
		return LangZH
	case strings.HasPrefix(env, "en"):
		return LangEN
	default:
		return LangZH
	}
}

// SetLang sets the current language from a string code.
// Returns true if the language is supported.
func SetLang(code string) bool {
	mu.Lock()
	defer mu.Unlock()

	switch strings.ToLower(code) {
	case "zh", "zh-cn", "zh_cn", "chinese":
		currentLang = LangZH
		return true
	case "en", "en-us", "en_us", "english":
		currentLang = LangEN
		return true
	default:
		return false
	}
}

// GetLang returns the current language code.
func GetLang() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// Init initializes the i18n system.
// If langFlag is provided and valid, it takes highest priority.
// Otherwise, detects from environment.
func Init(langFlag string) {
	if langFlag != "" && SetLang(langFlag) {
		return
	}
	_ = SetLang(string(DetectLang()))
}

// T returns the translated string for the given key.
// If the key is not found, returns the key itself as fallback.
func T(key string) string {
	mu.RLock()
	lang := currentLang
	mu.RUnlock()

	if msg := lookup(lang, key); msg != "" {
		return msg
	}

	// Fallback to Chinese
	if msg := lookup(LangZH, key); msg != "" {
		return msg
	}

	return key
}

// TF returns the translated string with fmt.Sprintf-style formatting.
func TF(key string, args ...interface{}) string {
	msg := T(key)
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// lookup finds a translation for a given language and key.
func lookup(lang Lang, key string) string {
	switch lang {
	case LangZH:
		if msg, ok := zhMessages[key]; ok {
			return msg
		}
	case LangEN:
		if msg, ok := enMessages[key]; ok {
			return msg
		}
	}
	return ""
}

// Translation key constants for compile-time safety and discoverability.
const (
	// General
	KeyCancelled      = "cancelled"
	KeySetupCancelled = "setup_cancelled"
	KeyYes            = "yes"
	KeyNo             = "no"
	KeyOn             = "on"
	KeyOff            = "off"
	KeyError          = "error"
	KeyWarning        = "warning"
	KeySuccess        = "success"
	KeyUnlimited      = "unlimited"
	KeyDefault        = "default"

	// Wizard - General
	KeyWizardTitle       = "wizard_title"
	KeyWizardDescription = "wizard_description"
	KeySelectProvider    = "wizard_select_provider"
	KeyProviderSelected  = "wizard_provider_selected"
	KeyProviderLabel     = "wizard_provider_label"
	KeyEndpointLabel     = "wizard_endpoint_label"
	KeyEndpointRequired  = "wizard_endpoint_required"
	KeyAPIKeyLabel       = "wizard_api_key_label"
	KeyAPIKeyRequired    = "wizard_api_key_required"
	KeyModelName         = "wizard_model_name"
	KeyAPITest           = "wizard_api_test"
	KeyAPITestOK         = "wizard_api_test_ok"
	KeyAPITestFail       = "wizard_api_test_fail"
	KeyAPITestPrompt     = "wizard_api_test_prompt"
	KeyFetchModels       = "wizard_fetch_models"
	KeyFetchModelsOK     = "wizard_fetch_models_ok"
	KeyFetchModelsFail   = "wizard_fetch_models_fail"
	KeyEndpointTest      = "wizard_endpoint_test"
	KeyEndpointTestOK    = "wizard_endpoint_test_ok"
	KeyEndpointTestFail  = "wizard_endpoint_test_fail"
	KeyEndpointRetry     = "wizard_endpoint_retry"
	KeyAPIKeyGetPrompt   = "wizard_api_key_get_prompt"
	KeyAPIKeyManualGet   = "wizard_api_key_manual_get"
	KeyAPIKeyOpenPage    = "wizard_api_key_open_page"
	KeyAPIKeyOpeningPage = "wizard_api_key_opening_page"
	KeyAPIKeyManualOpen  = "wizard_api_key_manual_open"
	KeyEmptyField        = "wizard_empty_field"
	KeyInvalidChoice     = "wizard_invalid_choice"
	KeyConfigSaved       = "wizard_config_saved"

	// REPL
	KeyGoodbye     = "repl_goodbye"
	KeyExit        = "repl_exit"
	KeyCleanup     = "repl_cleanup"
	KeyCleanupDone = "repl_cleanup_done"
	KeyUnknownCmd  = "repl_unknown_cmd"
	KeyCmdError    = "repl_cmd_error"
	KeyCmdExecFail = "repl_cmd_exec_fail"
	KeyAgentFail   = "repl_agent_fail"
	KeyAgentHint   = "repl_agent_hint"
	KeyOutputTitle = "repl_output_title"
	KeyOutputSep   = "repl_output_sep"
	KeyToolCall    = "repl_tool_call"

	// Settings - Labels
	KeySettingsLabel        = "settings_label"
	KeyAPIKeyLabelSetting   = "settings_api_key_label"
	KeyEndpointLabelSetting = "settings_endpoint_label"
	KeyModelLabel           = "settings_model_label"
	KeyTempLabel            = "settings_temp_label"
	KeyMaxTokensLabel       = "settings_max_tokens_label"
	KeyProviderLabelSetting = "settings_provider_label"

	// Settings - Messages
	KeySettingsUpdated  = "settings_updated"
	KeyEndpointUpdated  = "settings_endpoint_updated"
	KeyModelUpdated     = "settings_model_updated"
	KeyTempUpdated      = "settings_temp_updated"
	KeyMaxTokensUpdated = "settings_max_tokens_updated"
	KeyShowThinking     = "settings_show_thinking"
	KeyShowCommand      = "settings_show_command"
	KeyShowOutput       = "settings_show_output"
	KeyLogEnabled       = "settings_log_enabled"
	KeyMaxIterations    = "settings_max_iterations"
	KeyProviderUpdated  = "settings_provider_updated"

	// Settings - Config Show
	KeyConfigTitle         = "config_title"
	KeyConfigProvider      = "config_provider"
	KeyConfigEndpoint      = "config_endpoint"
	KeyConfigModel         = "config_model"
	KeyConfigTemperature   = "config_temperature"
	KeyConfigMaxTokens     = "config_max_tokens"
	KeyConfigMaxIterations = "config_max_iterations"
	KeyConfigShowThinking  = "config_show_thinking"
	KeyConfigShowCommand   = "config_show_command"
	KeyConfigShowOutput    = "config_show_output"
	KeyConfigLogging       = "config_logging"
	KeyConfigMCPServers    = "config_mcp_servers"
	KeyConfigRules         = "config_rules"

	// MCP
	KeyMCPAlreadyExists = "mcp_already_exists"
	KeyMCPAdded         = "mcp_added"
	KeyMCPRemoved       = "mcp_removed"
	KeyMCPNotFound      = "mcp_not_found"
	KeyMCPEnabled       = "mcp_enabled"
	KeyMCPDisabled      = "mcp_disabled"
	KeyMCPEmpty         = "mcp_empty"
	KeyMCPListTitle     = "mcp_list_title"

	// Rule
	KeyRuleAdded   = "rule_added"
	KeyRuleRemoved = "rule_removed"
	KeyRuleCleared = "rule_cleared"
	KeyRuleInvalid = "rule_invalid"
	KeyRuleNoRules = "rule_no_rules"

	// Memory
	KeyMemorySaved   = "memory_saved"
	KeyMemoryDeleted = "memory_deleted"
	KeyMemoryCleared = "memory_cleared"
	KeyMemoryEmpty   = "memory_empty"
	KeyMemoryGet     = "memory_get"

	// Context
	KeyContextShow  = "context_show"
	KeyContextEmpty = "context_empty"
	KeyContextReset = "context_reset"
	KeyContextSet   = "context_set"

	// Agent
	KeyNoopClientError = "noop_client_error"

	// Settings - Extended
	KeySettingsLabelLog           = "settings_label_log"
	KeySettingsLabelShowThinking  = "settings_label_show_thinking"
	KeySettingsLabelShowCommand   = "settings_label_show_command"
	KeySettingsLabelShowOutput    = "settings_label_show_output"
	KeySettingsLabelMaxIterations = "settings_label_max_iterations"
	KeySettingsLabelProvider      = "settings_label_provider"

	// Config format
	KeyConfigFormat = "config_format"

	// REPL - Additional
	KeyWelcomeTip     = "repl_welcome_tip"
	KeyUnknownCommand = "repl_unknown_command"
	KeyCmdFailed      = "repl_cmd_failed"
	KeyProcessFailed  = "repl_process_failed"
	KeyCheckConfig    = "repl_check_config"
	KeyCleaningUp     = "repl_cleaning_up"
	KeyDone           = "repl_done"

	// Help
	KeyHelpTitle        = "help_title"
	KeyHelpNLTitle      = "help_nl_title"
	KeyHelpNLDesc       = "help_nl_desc"
	KeyHelpBuiltinTitle = "help_builtin_title"
	KeyHelpSettings     = "help_settings"
	KeyHelpMCP          = "help_mcp"
	KeyHelpRule         = "help_rule"
	KeyHelpMemory       = "help_memory"
	KeyHelpContext      = "help_context"
	KeyHelpList         = "help_list"
	KeyHelpLast         = "help_last"
	KeyHelpFirst        = "help_first"
	KeyHelpImage        = "help_image"
	KeyHelpPlan         = "help_plan"
	KeyHelpWizard       = "help_wizard"
	KeyHelpHelp         = "help_help"
	KeyHelpExit         = "help_exit"
	KeyHelpExampleTitle = "help_example_title"
	KeyHelpExample1     = "help_example_1"
	KeyHelpExample2     = "help_example_2"
	KeyHelpExample3     = "help_example_3"
	KeyHelpExample4     = "help_example_4"
	KeyHelpExample5     = "help_example_5"

	// CLI Help
	KeyCLIHelpTitle     = "cli_help_title"
	KeyCLIHelpUsage     = "cli_help_usage"
	KeyCLIHelpUsageREPL = "cli_help_usage_repl"
	KeyCLIHelpUsageCmd  = "cli_help_usage_cmd"
	KeyCLIHelpOptions   = "cli_help_options"
	KeyCLIHelpConfig    = "cli_help_config"
	KeyCLIHelpModel     = "cli_help_model"
	KeyCLIHelpEndpoint  = "cli_help_endpoint"
	KeyCLIHelpAPIKey    = "cli_help_api_key"
	KeyCLIHelpLang      = "cli_help_lang"
	KeyCLIHelpLog       = "cli_help_log"
	KeyCLIHelpMaxIter   = "cli_help_max_iter"
	KeyCLIHelpVersion   = "cli_help_version"
	KeyCLIHelpHelp      = "cli_help_help"
	KeyCLIHelpExamples  = "cli_help_examples"
	KeyCLIHelpEx1       = "cli_help_ex1"
	KeyCLIHelpEx2       = "cli_help_ex2"
	KeyCLIHelpEx3       = "cli_help_ex3"
	KeyCLIHelpEx4       = "cli_help_ex4"
	KeyCLIHelpEx5       = "cli_help_ex5"
	KeyCLIHelpEx6       = "cli_help_ex6"
	KeyCLIHelpEx7       = "cli_help_ex7"

	// Disclaimer
	KeyDisclaimerTitle   = "disclaimer_title"
	KeyDisclaimerBody    = "disclaimer_body"
	KeyDisclaimerPrompt  = "disclaimer_prompt"
	KeyDisclaimerYes     = "disclaimer_yes"
	KeyDisclaimerNo      = "disclaimer_no"
	KeyDisclaimerRefused = "disclaimer_refused"

	// Command Confirmation
	KeyCmdConfirmTitle       = "cmd_confirm_title"
	KeyCmdConfirmPrompt      = "cmd_confirm_prompt"
	KeyCmdConfirmApprove     = "cmd_confirm_approve"
	KeyCmdConfirmApproveAll  = "cmd_confirm_approve_all"
	KeyCmdConfirmCancel      = "cmd_confirm_cancel"
	KeyCmdConfirmModify      = "cmd_confirm_modify"
	KeyCmdConfirmInvalid     = "cmd_confirm_invalid"
	KeyCmdConfirmCancelled   = "cmd_confirm_cancelled"
	KeyCmdConfirmModifyHint  = "cmd_confirm_modify_hint"
	KeyCmdConfirmDisabled    = "cmd_confirm_disabled"
	KeyCmdConfirmEnabled     = "cmd_confirm_enabled"
	KeyCmdConfirmDisableWarn = "cmd_confirm_disable_warn"

	// History list
	KeyListTitle     = "list_title"
	KeyListEmpty     = "list_empty"
	KeyListReExecute = "list_re_execute"
	KeyListInvalid   = "list_invalid"
	KeyLastUsage     = "last_usage"
	KeyFirstUsage    = "first_usage"
	KeyListUsage     = "list_usage"

	// Agent output
	KeyAgentSaid = "agent_said"

	// CLI Help - Name
	KeyCLIHelpName = "cli_help_name"

	// CLI Help - Workspace
	KeyCLIHelpWorkspace = "cli_help_workspace"

	// CLI Help - Example 8
	KeyCLIHelpEx8 = "cli_help_ex8"

	// CLI Help - Example 9-11 (new parameters)
	KeyCLIHelpEx9  = "cli_help_ex9"
	KeyCLIHelpEx10 = "cli_help_ex10"
	KeyCLIHelpEx11 = "cli_help_ex11"

	// CLI Help - Image
	KeyCLIHelpImage = "cli_help_image"

	// CLI Help - LLM Behavior
	KeyCLIHelpTemperature    = "cli_help_temperature"
	KeyCLIHelpMaxTokens      = "cli_help_max_tokens"
	KeyCLIHelpShowThinking   = "cli_help_show_thinking"
	KeyCLIHelpShowCommand    = "cli_help_show_command"
	KeyCLIHelpShowOutput     = "cli_help_show_output"
	KeyCLIHelpConfirmCommand = "cli_help_confirm_command"
	KeyCLIHelpResultMode     = "cli_help_result_mode"

	// CLI Help - Agent Identity
	KeyCLIHelpDescription = "cli_help_description"
	KeyCLIHelpPrinciples  = "cli_help_principles"

	// CLI Help - Timeout
	KeyCLIHelpToolTimeout = "cli_help_tool_timeout"
	KeyCLIHelpCmdTimeout  = "cli_help_cmd_timeout"
	KeyCLIHelpLLMTimeout  = "cli_help_llm_timeout"

	// Custom
	KeyCustom = "custom"

	// System Prompt
	KeySystemPromptTitle        = "system_prompt_title"
	KeySystemPromptEnv          = "system_prompt_env"
	KeySystemPromptCapabilities = "system_prompt_capabilities"
	KeySystemPromptRules        = "system_prompt_rules"
	KeySystemPromptResultMode   = "system_prompt_result_mode"
	KeySystemPromptIdentity     = "system_prompt_identity"

	// Timeout settings
	KeyConfigToolTimeout   = "config_tool_timeout"
	KeyConfigCmdTimeout    = "config_cmd_timeout"
	KeyConfigLLMTimeout    = "config_llm_timeout"
	KeySettingsToolTimeout = "settings_tool_timeout"
	KeySettingsCmdTimeout  = "settings_cmd_timeout"
	KeySettingsLLMTimeout  = "settings_llm_timeout"

	// Wizard command
	KeyWizardCmdRunning = "wizard_cmd_running"
	KeyWizardCmdDone    = "wizard_cmd_done"

	// Settings help table
	KeySettingsHelpTitle        = "settings_help_title"
	KeySettingsColParam         = "settings_col_param"
	KeySettingsColValues        = "settings_col_values"
	KeySettingsColDesc          = "settings_col_desc"
	KeySettingsDescAPIKey       = "settings_desc_api_key"
	KeySettingsDescEndpoint     = "settings_desc_endpoint"
	KeySettingsDescModel        = "settings_desc_model"
	KeySettingsDescTemp         = "settings_desc_temp"
	KeySettingsDescMaxTokens    = "settings_desc_max_tokens"
	KeySettingsDescShowThinking = "settings_desc_show_thinking"
	KeySettingsDescShowCommand  = "settings_desc_show_command"
	KeySettingsDescShowOutput   = "settings_desc_show_output"
	KeySettingsDescConfirmCmd   = "settings_desc_confirm_cmd"
	KeySettingsDescLog          = "settings_desc_log"
	KeySettingsDescMaxIter      = "settings_desc_max_iter"
	KeySettingsDescMaxRetries   = "settings_desc_max_retries"
	KeySettingsDescResultMode   = "settings_desc_result_mode"
	KeySettingsDescName         = "settings_desc_name"
	KeySettingsDescDescription  = "settings_desc_description"
	KeySettingsDescPrinciples   = "settings_desc_principles"
	KeySettingsDescToolTimeout  = "settings_desc_tool_timeout"
	KeySettingsDescCmdTimeout   = "settings_desc_cmd_timeout"
	KeySettingsDescLLMTimeout   = "settings_desc_llm_timeout"
	KeySettingsHelpFooter       = "settings_help_footer"
	KeySettingsCurrentTitle     = "settings_current_title"

	// Config show column 3 labels (i18n for the third column)
	KeyCol3Provider     = "col3_provider"
	KeyCol3Endpoint     = "col3_endpoint"
	KeyCol3Model        = "col3_model"
	KeyCol3Temperature  = "col3_temperature"
	KeyCol3MaxTokens    = "col3_max_tokens"
	KeyCol3MaxIter      = "col3_max_iter"
	KeyCol3MaxRetries   = "col3_max_retries"
	KeyCol3Thinking     = "col3_thinking"
	KeyCol3Command      = "col3_command"
	KeyCol3Output       = "col3_output"
	KeyCol3Confirm      = "col3_confirm"
	KeyCol3ToolTimeout  = "col3_tool_timeout"
	KeyCol3CmdTimeout   = "col3_cmd_timeout"
	KeyCol3LLMTimeout   = "col3_llm_timeout"
	KeyCol3Log          = "col3_log"
	KeyCol3ResultMode   = "col3_result_mode"
	KeyCol3APIKey       = "col3_api_key"
	KeyCol3Name         = "col3_name"
	KeyCol3Desc         = "col3_desc"
	KeyCol3Principles   = "col3_principles"
	KeyCol3Vision       = "col3_vision"
	KeyCol3ContextLimit = "col3_context_limit"

	// Context limit
	KeyContextLimitLabel    = "context_limit_label"
	KeyContextLimitUpdated  = "context_limit_updated"
	KeySettingsDescCtxLimit = "settings_desc_ctx_limit"
	KeyConfigContextLimit   = "config_context_limit"

	// Memory enabled
	KeyCol3MemoryEnabled     = "col3_memory_enabled"
	KeySettingsDescMemory    = "settings_desc_memory"
	KeyMemoryEnabledUpdated  = "memory_enabled_updated"
	KeyCLIHelpMemoryEnabled  = "cli_help_memory_enabled"
	KeyCLIHelpMemoryDisabled = "cli_help_memory_disabled"
)
