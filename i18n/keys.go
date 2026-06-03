// Author: L.Shuang
// Created: 2026-05-22
// Last Modified: 2026-05-22
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
// Package i18n - Translation key constants.
package i18n

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

	// New output control keys (ENHANCEMENT-126)
	KeyShowLlmThinking   = "settings_show_llm_thinking"
	KeyShowLlmContent    = "settings_show_llm_content"
	KeyShowTool          = "settings_show_tool"
	KeyShowToolInput     = "settings_show_tool_input"
	KeyShowToolOutput    = "settings_show_tool_output"
	KeyShowCommandOutput = "settings_show_command_output"
	KeyMaxIterations     = "settings_max_iterations"
	KeyProviderUpdated   = "settings_provider_updated"

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
	KeySettingsLabelLog               = "settings_label_log"
	KeySettingsLabelShowThinking      = "settings_label_show_thinking"
	KeySettingsLabelShowCommand       = "settings_label_show_command"
	KeySettingsLabelShowOutput        = "settings_label_show_output"
	KeySettingsLabelShowLlmThinking   = "settings_label_show_llm_thinking"
	KeySettingsLabelShowLlmContent    = "settings_label_show_llm_content"
	KeySettingsLabelShowTool          = "settings_label_show_tool"
	KeySettingsLabelShowToolInput     = "settings_label_show_tool_input"
	KeySettingsLabelShowToolOutput    = "settings_label_show_tool_output"
	KeySettingsLabelShowCommandOutput = "settings_label_show_command_output"
	KeySettingsLabelMaxIterations     = "settings_label_max_iterations"
	KeySettingsLabelProvider          = "settings_label_provider"

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
	KeyHelpBodyAdd      = "help_body_add"
	KeyHelpBodyRemove   = "help_body_remove"
	KeyHelpBodyDisplay  = "help_body_display"
	KeyHelpNew          = "help_new"
	KeyHelpModel        = "help_model"
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
	KeyCmdConfirmTitle        = "cmd_confirm_title"
	KeyCmdConfirmRiskWarning  = "cmd_confirm_risk_warning"
	KeyCmdConfirmPrompt       = "cmd_confirm_prompt"
	KeyCmdConfirmApprove      = "cmd_confirm_approve"
	KeyCmdConfirmApproveAll   = "cmd_confirm_approve_all"
	KeyCmdConfirmCancel       = "cmd_confirm_cancel"
	KeyCmdConfirmModify       = "cmd_confirm_modify"
	KeyCmdConfirmInvalid      = "cmd_confirm_invalid"
	KeyCmdConfirmCancelled    = "cmd_confirm_cancelled"
	KeyCmdConfirmModifyHint   = "cmd_confirm_modify_hint"
	KeyCmdConfirmDisabled     = "cmd_confirm_disabled"
	KeyCmdConfirmEnabled      = "cmd_confirm_enabled"
	KeyCmdConfirmDisableWarn  = "cmd_confirm_disable_warn"
	KeyCmdConfirmDisableTool  = "cmd_confirm_disable_tool"
	KeyCmdConfirmDisableToolD = "cmd_confirm_disable_tool_d"
	KeyCmdConfirmApproveG     = "cmd_confirm_approve_g"
	KeyCmdConfirmApproveGDesc = "cmd_confirm_approve_g_desc"
	KeyCmdConfirmApproveD     = "cmd_confirm_approve_d"
	KeyCmdConfirmApproveDDesc = "cmd_confirm_approve_d_desc"
	KeyCmdConfirmCountPrefix  = "cmd_confirm_count_prefix"
	KeyCmdConfirmCountSuffix  = "cmd_confirm_count_suffix"
	KeyErrorRiskWarning       = "error_risk_warning"

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
	KeyCLIHelpTemperature  = "cli_help_temperature"
	KeyCLIHelpMaxTokens    = "cli_help_max_tokens"
	KeyCLIHelpShowThinking = "cli_help_show_thinking"
	KeyCLIHelpShowCommand  = "cli_help_show_command"
	KeyCLIHelpConfirmTool  = "cli_help_confirm_tool"
	KeyCLIHelpResultMode   = "cli_help_result_mode"

	// CLI Help - New output control (ENHANCEMENT-126)
	KeyCLIHelpShowLlmThinking   = "cli_help_show_llm_thinking"
	KeyCLIHelpShowLlmContent    = "cli_help_show_llm_content"
	KeyCLIHelpShowTool          = "cli_help_show_tool"
	KeyCLIHelpShowToolInput     = "cli_help_show_tool_input"
	KeyCLIHelpShowToolOutput    = "cli_help_show_tool_output"
	KeyCLIHelpShowCommandOutput = "cli_help_show_command_output"

	// CLI Help - Agent Identity
	KeyCLIHelpDescription = "cli_help_description"
	KeyCLIHelpPrinciples  = "cli_help_principles"

	// CLI Help - Timeout
	KeyCLIHelpToolTimeout       = "cli_help_tool_timeout"
	KeyCLIHelpCmdTimeout        = "cli_help_cmd_timeout"
	KeyCLIHelpLLMTimeout        = "cli_help_llm_timeout"
	KeyCLIHelpTopP              = "cli_help_top_p"
	KeyCLIHelpTopK              = "cli_help_top_k"
	KeyCLIHelpRepetitionPenalty = "cli_help_repetition_penalty"

	// Custom
	KeyCustom = "custom"

	// System Prompt
	KeySystemPromptIdentity     = "system_prompt_identity"
	KeyAnonymousUser            = "anonymous_user"
	KeySystemPromptObjective    = "system_prompt_objective"
	KeySystemPromptEnvironment  = "system_prompt_environment"
	KeySystemPromptCapabilities = "system_prompt_capabilities"
	KeySystemPromptRules        = "system_prompt_rules"
	KeySystemPromptResultMode   = "system_prompt_result_mode"

	// System Prompt - legacy keys (not used in buildSystemPromptWithMode, kept for reference)
	KeySystemPromptToolUsage    = "system_prompt_tool_usage"
	KeySystemPromptToolUsageXML = "system_prompt_tool_usage_xml"
	KeySystemPromptEnv          = "system_prompt_env"

	KeySystemPromptEditingFiles = "system_prompt_editing_files"

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
	KeySettingsHelpTitle             = "settings_help_title"
	KeySettingsColParam              = "settings_col_param"
	KeySettingsColValues             = "settings_col_values"
	KeySettingsColDesc               = "settings_col_desc"
	KeySettingsDescAPIKey            = "settings_desc_api_key"
	KeySettingsDescEndpoint          = "settings_desc_endpoint"
	KeySettingsDescModel             = "settings_desc_model"
	KeySettingsDescTemp              = "settings_desc_temp"
	KeySettingsDescMaxTokens         = "settings_desc_max_tokens"
	KeySettingsDescShowThinking      = "settings_desc_show_thinking"
	KeySettingsDescShowCommand       = "settings_desc_show_command"
	KeySettingsDescShowOutput        = "settings_desc_show_output"
	KeySettingsDescConfirmCmd        = "settings_desc_confirm_cmd"
	KeySettingsDescLog               = "settings_desc_log"
	KeySettingsDescMaxIter           = "settings_desc_max_iter"
	KeySettingsDescMaxRetries        = "settings_desc_max_retries"
	KeySettingsDescResultMode        = "settings_desc_result_mode"
	KeySettingsDescName              = "settings_desc_name"
	KeySettingsDescDescription       = "settings_desc_description"
	KeySettingsDescPrinciples        = "settings_desc_principles"
	KeySettingsDescToolTimeout       = "settings_desc_tool_timeout"
	KeySettingsDescCmdTimeout        = "settings_desc_cmd_timeout"
	KeySettingsDescLLMTimeout        = "settings_desc_llm_timeout"
	KeySettingsDescTopP              = "settings_desc_top_p"
	KeySettingsDescTopK              = "settings_desc_top_k"
	KeySettingsDescRepetitionPenalty = "settings_desc_repetition_penalty"
	KeySettingsDescTokenUsage        = "settings_desc_token_usage"
	KeySettingsHelpFooter            = "settings_help_footer"
	KeySettingsCurrentTitle          = "settings_current_title"

	// Settings descriptions - New output control (ENHANCEMENT-126)
	KeySettingsDescLlmThinking   = "settings_desc_llm_thinking"
	KeySettingsDescLlmContent    = "settings_desc_llm_content"
	KeySettingsDescTool          = "settings_desc_tool"
	KeySettingsDescToolInput     = "settings_desc_tool_input"
	KeySettingsDescToolOutput    = "settings_desc_tool_output"
	KeySettingsDescCommandOutput = "settings_desc_command_output"

	// Config show column 3 labels
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

	// Config show column 3 labels - New output control (ENHANCEMENT-126)
	KeyCol3LlmThinking   = "col3_llm_thinking"
	KeyCol3LlmContent    = "col3_llm_content"
	KeyCol3Tool          = "col3_tool"
	KeyCol3ToolInput     = "col3_tool_input"
	KeyCol3ToolOutput    = "col3_tool_output"
	KeyCol3CommandOutput = "col3_command_output"

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

	// Plan enabled
	KeyCol3PlanEnabled     = "col3_plan_enabled"
	KeySettingsDescPlan    = "settings_desc_plan"
	KeyPlanEnabledUpdated  = "plan_enabled_updated"
	KeyCLIHelpPlanEnabled  = "cli_help_plan_enabled"
	KeyCLIHelpPlanDisabled = "cli_help_plan_disabled"

	// SubAgent enabled
	KeyCol3SubAgentEnabled     = "col3_sub_agent_enabled"
	KeySettingsDescSubAgent    = "settings_desc_sub_agent"
	KeySubAgentEnabledUpdated  = "sub_agent_enabled_updated"
	KeyCLIHelpSubAgentEnabled  = "cli_help_sub_agent_enabled"
	KeyCLIHelpSubAgentDisabled = "cli_help_sub_agent_disabled"

	// ToolCall enabled
	KeyCLIHelpToolCallEnabled  = "cli_help_tool_call_enabled"
	KeyCLIHelpToolCallDisabled = "cli_help_tool_call_disabled"

	// Search settings
	KeyCol3SearchMaxLineLength          = "col3_search_max_line_length"
	KeyCol3SearchMaxResultBytes         = "col3_search_max_result_bytes"
	KeyCol3SearchContextLines           = "col3_search_context_lines"
	KeySettingsDescSearchMaxLineLength  = "settings_desc_search_max_line_length"
	KeySettingsDescSearchMaxResultBytes = "settings_desc_search_max_result_bytes"
	KeySettingsDescSearchContextLines   = "settings_desc_search_context_lines"

	// MCP and Rules column 3 labels
	KeyCol3MCP   = "col3_mcp"
	KeyCol3Rules = "col3_rules"

	// Session
	KeySessionTitle          = "session_title"
	KeySessionTotalMessages  = "session_total_messages"
	KeySessionRoleSystem     = "session_role_system"
	KeySessionRoleUser       = "session_role_user"
	KeySessionRoleAssistant  = "session_role_assistant"
	KeySessionRoleTool       = "session_role_tool"
	KeySessionContextLimit   = "session_context_limit"
	KeySessionNoHistory      = "session_no_history"
	KeySessionModel          = "session_model"
	KeySessionProvider       = "session_provider"
	KeySessionAgentName      = "session_agent_name"
	KeySessionRecentMessages = "session_recent_messages"

	// History command
	KeyHelpHistory  = "help_history"
	KeyHelpSession  = "help_session"
	KeyHistoryUsage = "history_usage"

	// Search results (used in searchFilesTool output)
	KeySearchResultFound        = "search_result_found"
	KeySearchResultFoundTrunc   = "search_result_found_trunc"
	KeySearchResultFoundPartial = "search_result_found_partial"
	KeySearchResultNone         = "search_result_none"
	KeySearchLineTruncated      = "search_line_truncated"
	KeySearchResultFileHeader   = "search_result_file_header"
	KeySearchResultMatchLine    = "search_result_match_line"

	// Memory search config
	KeyCol3MemorySearchMaxContentLen = "col3_memory_search_max_content_len"
	KeyCol3MemorySearchMaxResults    = "col3_memory_search_max_results"
	KeySettingsDescMemSearchMaxLen   = "settings_desc_mem_search_max_len"
	KeySettingsDescMemSearchMaxRes   = "settings_desc_mem_search_max_res"

	// Thinking enabled
	KeyCol3ThinkingEnabled   = "col3_thinking_enabled"
	KeyCol3ReasoningEffort   = "col3_reasoning_effort"
	KeyCol3ToolCallEnabled   = "col3_tool_call_enabled"
	KeyCol3MaxModelLen       = "col3_max_model_len"
	KeyCol3TopP              = "col3_top_p"
	KeyCol3TopK              = "col3_top_k"
	KeyCol3RepetitionPenalty = "col3_repetition_penalty"
	KeyCol3TokenUsage        = "col3_token_usage"

	// Model selection column 3 labels
	KeyCol3DefaultToolModel    = "col3_default_tool_model"
	KeyCol3DefaultVisionModel  = "col3_default_vision_model"
	KeyCol3DefaultProblemModel = "col3_default_problem_model"

	// Settings group titles
	KeySettingsGroupIdentity    = "settings_group_identity"
	KeySettingsGroupModel       = "settings_group_model"
	KeySettingsGroupDisplay     = "settings_group_display"
	KeySettingsGroupSafety      = "settings_group_safety"
	KeySettingsGroupMemory      = "settings_group_memory"
	KeySettingsGroupTask        = "settings_group_task"
	KeySettingsGroupSearchDebug = "settings_group_search_debug"

	// Error settings column 3 labels
	KeyCol3ErrorMaxSingleCount = "col3_error_max_single_count"
	KeyCol3ErrorMaxTypeCount   = "col3_error_max_type_count"

	// Loop detection settings (FIX-179)
	KeyCol3LoopDetectEnabled     = "col3_loop_detect_enabled"
	KeyCol3LoopDetectThreshold   = "col3_loop_detect_threshold"
	KeyCol3LoopDetectMaxWindow   = "col3_loop_detect_max_window"
	KeySettingsDescLoopDetect    = "settings_desc_loop_detect"
	KeySettingsDescLoopThreshold = "settings_desc_loop_threshold"
	KeySettingsDescLoopWindow    = "settings_desc_loop_window"
	KeyLoopDetectEnabledUpdated  = "loop_detect_enabled_updated"
	KeyCLIHelpLoopDetectEnabled  = "cli_help_loop_detect_enabled"
	KeyCLIHelpLoopDetectDisabled = "cli_help_loop_detect_disabled"
	KeyCLIHelpLoopDetect         = "cli_help_loop_detect"
	KeyCLIHelpDedup              = "cli_help_dedup"

	// Settings confirmation (FEATURE-131)
	KeySettingsConfirmTitle          = "settings_confirm_title"
	KeySettingsConfirmRiskWarning    = "settings_confirm_risk_warning"
	KeySettingsConfirmPrompt         = "settings_confirm_prompt"
	KeySettingsConfirmRejected       = "settings_confirm_rejected"
	KeySettingsConfirmRejectedResult = "settings_confirm_rejected_result"
	KeySettingsConfirmApplied        = "settings_confirm_applied"
	KeySettingsConfirmFailed         = "settings_confirm_failed"
	KeySettingsConfirmResult         = "settings_confirm_result"
	KeySettingsConfirmPaused         = "settings_confirm_paused"

	// Emoji prefix keys (ENHANCEMENT-131)
	KeyEmojiPrefixUser        = "emoji_prefix_user"
	KeyEmojiPrefixAssistant   = "emoji_prefix_assistant"
	KeyEmojiPrefixToolInput   = "emoji_prefix_tool_input"
	KeyEmojiPrefixToolOutput  = "emoji_prefix_tool_output"
	KeyEmojiPrefixCmdInput    = "emoji_prefix_cmd_input"
	KeyEmojiPrefixCmdOutput   = "emoji_prefix_cmd_output"
	KeyEmojiPrefixInfo        = "emoji_prefix_info"
	KeyEmojiPrefixError       = "emoji_prefix_error"
	KeyEmojiPrefixWarning     = "emoji_prefix_warning"
	KeyEmojiPrefixSuccess     = "emoji_prefix_success"
	KeyEmojiPrefixThinking    = "emoji_prefix_thinking"
	KeyEmojiPrefixOutputTitle = "emoji_prefix_output_title"
	KeyEmojiPrefixOutputSep   = "emoji_prefix_output_sep"

	// Emoji enabled settings
	KeyCol3EmojiEnabled     = "col3_emoji_enabled"
	KeySettingsDescEmoji    = "settings_desc_emoji"
	KeyEmojiEnabledUpdated  = "emoji_enabled_updated"
	KeyCLIHelpEmojiEnabled  = "cli_help_emoji_enabled"
	KeyCLIHelpEmojiDisabled = "cli_help_emoji_disabled"

	// Show logo
	KeyCLIHelpShowLogo = "cli_help_show_logo"

	// Init capabilities/rules
	KeyCLIHelpInitCapabilities = "cli_help_init_capabilities"
	KeyCLIHelpInitRules        = "cli_help_init_rules"

	// Context start mode (FEATURE-103)
	KeyCol3ContextStartMode       = "col3_context_start_mode"
	KeySettingsDescCtxStart       = "settings_desc_ctx_start"
	KeyContextStartUpdated        = "context_start_updated"
	KeyCLIHelpContextStart        = "cli_help_context_start"
	KeyContextStartWindow         = "context_start_window"
	KeyContextStartWindowDesc     = "context_start_window_desc"
	KeyContextStartTask           = "context_start_task"
	KeyContextStartTaskDesc       = "context_start_task_desc"
	KeyContextStartSmart          = "context_start_smart"
	KeyContextStartSmartDesc      = "context_start_smart_desc"
	KeyAdjustContextStartDesc     = "adjust_context_start_desc"
	KeyAdjustContextStartResult   = "adjust_context_start_result"
	KeyAdjustContextStartNotSmart = "adjust_context_start_not_smart"
	KeyAdjustContextStartPrompt   = "adjust_context_start_prompt"

	// Database (PostgreSQL) related keys (FEATURE-86)
	KeyDBConnecting        = "db_connecting"
	KeyDBConnected         = "db_connected"
	KeyDBConnectFailed     = "db_connect_failed"
	KeyDBFallbackToLocal   = "db_fallback_to_local"
	KeyDBConfigLabel       = "db_config_label"
	KeyDBHostLabel         = "db_host_label"
	KeyDBPortLabel         = "db_port_label"
	KeyDBNameLabel         = "db_name_label"
	KeyDBSchemaLabel       = "db_schema_label"
	KeyDBUserLabel         = "db_user_label"
	KeyDBPasswordLabel     = "db_password_label"
	KeyDBEnabledLabel      = "db_enabled_label"
	KeyDBNotConfigured     = "db_not_configured"
	KeyDBMigrating         = "db_migrating"
	KeyDBMigrationComplete = "db_migration_complete"
	KeyDBMigrationFailed   = "db_migration_failed"

	// DB sub-command display (FEATURE-186)
	KeyDBSubCmdDesc  = "db_sub_cmd_desc"
	KeyDBInitDesc    = "db_init_desc"
	KeyDBMigrateDesc = "db_migrate_desc"

	// DB backup/restore (FEATURE-86)
	KeyDBBackupTitle    = "db_backup_title"
	KeyDBRestoreTitle   = "db_restore_title"
	KeyDBBackupDir      = "db_backup_dir"
	KeyDBRestoreDir     = "db_restore_dir"
	KeyDBBackupDone     = "db_backup_done"
	KeyDBRestoreDone    = "db_restore_done"
	KeyDBBackupFailed   = "db_backup_failed"
	KeyDBRestoreFailed  = "db_restore_failed"
	KeyDBNoBackupFound  = "db_no_backup_found"
	KeyDBSelectBackup   = "db_select_backup"
	KeyDBRestoreWarning = "db_restore_warning"
	KeyDBRestoreConfirm = "db_restore_confirm"
	KeyDBBackupCancel   = "db_backup_cancel"
	KeyDBRestoreCancel  = "db_restore_cancel"

	// Unknown (fallback display)
	KeyUnknown = "unknown"

	// Tool call mode (FEATURE-182)
	KeyToolCallMode         = "tool_call_mode"
	KeyToolCallModeUpdated  = "tool_call_mode_updated"
	KeyInvalidToolCallMode  = "invalid_tool_call_mode"
	KeyCol3ToolCallMode     = "col3_tool_call_mode"
	KeySettingsDescToolMode = "settings_desc_tool_mode"
	KeyCLIHelpToolCallMode  = "cli_help_tool_call_mode"

	// XML mode supplementary rules
	KeySystemPromptXMLRules        = "system_prompt_xml_rules"
	KeySystemPromptXMLExamples     = "system_prompt_xml_examples"
	KeySystemPromptXMLGuidelines   = "system_prompt_xml_guidelines"
	KeySystemPromptXMLTaskProgress = "system_prompt_xml_task_progress"
	// OpenAI/standard tool usage supplementary rules (non-XML versions)
	KeySystemPromptToolUsageExamples     = "system_prompt_tool_usage_examples"
	KeySystemPromptToolUsageTaskProgress = "system_prompt_tool_usage_task_progress"

	// XML tool result template (FIX-190)
	KeyXMLToolResultTemplate = "xml_tool_result_template"

	// Tool result — no task plan (FIX-190)
	KeyToolResultNoPlan = "tool_result_no_plan"

	// Tool result — with task plan (FIX-190)
	// Placeholders: {TASK_PLAN}
	KeyToolResultWithPlan = "tool_result_with_plan"

	// User message template for subsequent instructions during a task (FIX-190)
	// Placeholders: {INSTRUCTION}, {TASK_TRACKING}, {CURRENT_TIME}
	KeyUserMessageTemplate = "user_message_template"

	// Tool usage examples (FIX-190)
	KeyToolUsageExecuteCommand      = "tool_usage_execute_command"
	KeyToolUsageReadFile            = "tool_usage_read_file"
	KeyToolUsageSearchFiles         = "tool_usage_search_files"
	KeyToolUsageListFiles           = "tool_usage_list_files"
	KeyToolUsageListCodeDefNames    = "tool_usage_list_code_definition_names"
	KeyToolUsageReplaceInFile       = "tool_usage_replace_in_file"
	KeyToolUsageWriteToFile         = "tool_usage_write_to_file"
	KeyToolUsageAddImages           = "tool_usage_add_images"
	KeyToolUsageRemoveImages        = "tool_usage_remove_images"
	KeyToolUsageClearImages         = "tool_usage_clear_images"
	KeyToolUsageLaunchSubAgent      = "tool_usage_launch_sub_agent"
	KeyToolUsageScheduleTask        = "tool_usage_schedule_task"
	KeyToolUsageCreateTaskPlan      = "tool_usage_create_task_plan"
	KeyToolUsageUpdateTaskStep      = "tool_usage_update_task_step"
	KeyToolUsageInsertTaskSteps     = "tool_usage_insert_task_steps"
	KeyToolUsageRemoveTaskSteps     = "tool_usage_remove_task_steps"
	KeyToolUsageViewTaskPlan        = "tool_usage_view_task_plan"
	KeyToolUsageGetMemorySlice      = "tool_usage_get_memory_slice"
	KeyToolUsageMemorySearch        = "tool_usage_memory_search"
	KeyToolUsageDeleteMemory        = "tool_usage_delete_memory"
	KeyToolUsageUpdateSettings      = "tool_usage_update_settings"
	KeyToolUsageListSettings        = "tool_usage_list_settings"
	KeyToolUsageAskFollowupQuestion = "tool_usage_ask_followup_question"
	KeyToolUsageAdjustContextStart  = "tool_usage_adjust_context_start"
	KeyToolUsageAttemptCompletion   = "tool_usage_attempt_completion"

	// Shell session tool usage examples (XML mode)
	KeyToolUsageShellStart     = "tool_usage_shell_start"
	KeyToolUsageShellSend      = "tool_usage_shell_send"
	KeyToolUsageShellGetOutput = "tool_usage_shell_get_output"
	KeyToolUsageShellStop      = "tool_usage_shell_stop"

	// Shell session (column labels)
	KeyCol3ShellSessionEnabled = "col3_shell_session_enabled"
	KeyCol3ShellSessionTimeout = "col3_shell_session_timeout"

	KeySettingsDescShellSessionEnabled = "settings_desc_shell_session_enabled"
	KeySettingsDescShellSessionTimeout = "settings_desc_shell_session_timeout"

	// Shell session alternative system prompts
	KeySystemPromptToolUsageShell    = "system_prompt_tool_usage_shell"
	KeySystemPromptToolUsageXMLShell = "system_prompt_tool_usage_xml_shell"
	KeySystemPromptCapabilitiesShell = "system_prompt_capabilities_shell"
	KeySystemPromptRulesShell        = "system_prompt_rules_shell"

	// Shell session tool usage descriptions (XML mode, shell-enabled)
	KeyToolUsageShellSendShell      = "tool_usage_shell_send_shell"
	KeyToolUsageShellGetOutputShell = "tool_usage_shell_get_output_shell"
	KeyToolUsageShellWindowContent  = "tool_usage_shell_window_content"
	KeyToolUsageShellReset          = "tool_usage_shell_reset"

	// Section command keys
	KeySectionAdded   = "section_added"
	KeySectionRemoved = "section_removed"
	KeySectionCleared = "section_cleared"
	KeySectionInvalid = "section_invalid"
	KeySectionNoSects = "section_no_sections"
	KeySectionList    = "section_list"

	// Mode command keys
	KeyModeAdded        = "mode_added"
	KeyModeRemoved      = "mode_removed"
	KeyModeNotFound     = "mode_not_found"
	KeyModeExists       = "mode_already_exists"
	KeyModeList         = "mode_list"
	KeyModeCurrent      = "mode_current"
	KeyModeSwitched     = "mode_switched"
	KeyCol3WorkMode     = "col3_work_mode"
	KeySettingsDescMode = "settings_desc_mode"
	KeyCLIHelpMode      = "cli_help_mode"
	KeyHelpMode         = "help_mode"
	KeyHelpSection      = "help_section"
	KeyModeNoSects      = "mode_no_sections"

	// Config wizard (FEATURE-197)
	KeyHelpConfig             = "help_config"
	KeyConfigWizardTitle      = "config_wizard_title"
	KeyConfigWizardIntro      = "config_wizard_intro"
	KeyConfigGroupTitle       = "config_group_title"
	KeyConfigGroupPrompt      = "config_group_prompt"
	KeyConfigParamPrompt      = "config_param_prompt"
	KeyConfigValueLabParam    = "config_value_lab_param"
	KeyConfigValueLabCurrent  = "config_value_lab_current"
	KeyConfigValuePrompt      = "config_value_prompt"
	KeyConfigExited           = "config_exited"
	KeyConfigValueUnchanged   = "config_value_unchanged"
	KeyConfigInvalidChoice    = "config_invalid_choice"
	KeyConfigValOnOff         = "config_val_on_off"
	KeyConfigValMinExplAnFree = "config_val_min_expl_an_free"
	KeyConfigValWinTaskSmart  = "config_val_win_task_smart"
	KeyConfigValDebugOff      = "config_val_debug_off"
	KeyConfigValUnlimited     = "config_val_unlimited"
	KeyConfigValCtxLimit      = "config_val_ctx_limit"
	KeyConfigValCtxStart      = "config_val_ctx_start"
)
