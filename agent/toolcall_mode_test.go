package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

func TestParseXMLToolCalls_CommandWithSpecialChars(t *testing.T) {
	xmlInput := "<execute_command>\n<command>cd /Users/direct3d/agent/researcher/research/浏览器自动化与模拟人操作技术调研 && curl -s \"https://html.duckduckgo.com/html/?q=browser+automation+framework+Selenium+Playwright+Puppeteer+comparison+2025\" | grep -oP 'class=\"result__snippet\"[^>]*>[^<]*' | head -20\n</command>\n<timeout_seconds>\n30\n</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "execute_command" {
		t.Errorf("expected tool name 'execute_command', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	cmd, ok := args["command"]
	if !ok {
		t.Fatalf("missing 'command' argument, args: %v", args)
	}
	cmdStr, ok := cmd.(string)
	if !ok {
		t.Fatalf("expected 'command' to be a string, got %T: %v", cmd, cmd)
	}
	if len(cmdStr) == 0 {
		t.Fatal("expected non-empty command string")
	}
	t.Logf("command: %s", cmdStr)

	ts, ok := args["timeout_seconds"]
	if !ok {
		t.Fatalf("missing 'timeout_seconds' argument, args: %v", args)
	}
	tsFloat, ok := ts.(float64)
	if !ok {
		t.Fatalf("expected 'timeout_seconds' to be a number, got %T: %v", ts, ts)
	}
	if tsFloat != 30 {
		t.Errorf("expected timeout_seconds=30, got %v", tsFloat)
	}
}

func TestParseXMLToolCalls_CDATA(t *testing.T) {
	xmlInput := "<execute_command>\n<command><![CDATA[cd /path && curl -s \"https://example.com/?q=test&lang=go\" | grep -oP 'pattern' | head -20]]></command>\n<timeout_seconds>30</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	cmd, ok := args["command"]
	if !ok {
		t.Fatalf("missing 'command' argument, args: %v", args)
	}
	cmdStr, ok := cmd.(string)
	if !ok {
		t.Fatalf("expected 'command' to be a string, got %T: %v", cmd, cmd)
	}
	if len(cmdStr) == 0 {
		t.Fatal("expected non-empty command string")
	}
	t.Logf("command: %s", cmdStr)
}

func TestParseXMLToolCalls_SpecialCharsWithoutCDATA(t *testing.T) {
	xmlInput := "<execute_command>\n<command>curl -s \"https://example.com/?q=test&lang=go\" | grep -oP 'class=\"result__snippet\"[^>]*>[^<]*' | head -20</command>\n<timeout_seconds>30</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "execute_command" {
		t.Errorf("expected tool name 'execute_command', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	cmd, ok := args["command"]
	if !ok {
		t.Fatalf("missing 'command' argument, args: %v", args)
	}
	cmdStr, ok := cmd.(string)
	if !ok {
		t.Fatalf("expected 'command' to be a string, got %T: %v", cmd, cmd)
	}
	if len(cmdStr) == 0 {
		t.Fatal("expected non-empty command string")
	}
	t.Logf("command: %s", cmdStr)

	if !strings.Contains(cmdStr, "&") {
		t.Error("expected command to contain '&'")
	}
	if !strings.Contains(cmdStr, "<") {
		t.Error("expected command to contain '<'")
	}
	if !strings.Contains(cmdStr, ">") {
		t.Error("expected command to contain '>'")
	}
}

func TestParseXMLToolCalls_ParamNameTypo(t *testing.T) {
	xmlInput := "<execute_command>\n<commmand>ls -la</command>\n<timeout_seconds>30</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "参数") && !strings.Contains(errStr, "commmand") {
		t.Errorf("error message should mention the parameter issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)

	tag, ok := args["tag"]
	if !ok {
		t.Fatalf("missing 'tag' field in error arguments, args: %v", args)
	}
	tagStr, ok := tag.(string)
	if !ok {
		t.Fatalf("expected 'tag' to be a string, got %T: %v", tag, tagStr)
	}
	if tagStr != "execute_command" {
		t.Errorf("expected tag 'execute_command', got %q", tagStr)
	}
}

func TestParseXMLToolCalls_ParamMissingCloseTag(t *testing.T) {
	xmlInput := "<execute_command>\n<command>ls -la\n<timeout_seconds>30</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "参数") {
		t.Errorf("error message should mention the parameter issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidTagNameWithEquals(t *testing.T) {
	xmlInput := "<update_task_step>\n<step_id>1</step_id>\n<status>completed</status>\n<parameter=note>This is a note</parameter=note>\n</update_task_step>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "=") {
		t.Errorf("error message should mention the '=' character issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidTagNameWithSpace(t *testing.T) {
	xmlInput := "<update_task_step>\n<step_id>1</step_id>\n<status>completed</status>\n<parameter name=note>This is a note</parameter name=note>\n</update_task_step>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "空格") && !strings.Contains(errStr, "parameter") {
		t.Errorf("error message should mention the space or attribute issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_ToolTagWithSpace(t *testing.T) {
	xmlInput := "<execute_command timeout=30>\n<command>ls -la</command>\n</execute_command>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "空格") && !strings.Contains(errStr, "属性") {
		t.Errorf("error message should mention the space or attribute issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_ToolTagWithEquals(t *testing.T) {
	xmlInput := "<execute_command=xxx>\n<command>ls -la</command>\n</execute_command=xxx>"

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "=") {
		t.Errorf("error message should mention the '=' character issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidParamName(t *testing.T) {
	tools := []llm.Tool{
		{
			Name: "execute_command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Optional timeout in seconds",
					},
				},
				"required": []string{"command"},
			},
		},
	}

	xmlInput := "<execute_command>\n<commmand>ls -la</commmand>\n<timeout_seconds>30</timeout_seconds>\n</execute_command>"

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "commmand") {
		t.Errorf("error message should mention the invalid parameter name 'commmand', got: %s", errStr)
	}
	if !strings.Contains(errStr, "合法参数") {
		t.Errorf("error message should mention '合法参数', got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_MissingParentCloseTag(t *testing.T) {
	xmlInput := "<execute_command><command></command>"

	tools := []llm.Tool{
		{
			Name: "execute_command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
				},
			},
		},
	}

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	if !strings.Contains(errStr, "闭合标签") && !strings.Contains(errStr, "execute_command") {
		t.Errorf("error message should mention the missing close tag for execute_command, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)

	tag, ok := args["tag"]
	if !ok {
		t.Fatalf("missing 'tag' field in error arguments, args: %v", args)
	}
	tagStr, ok := tag.(string)
	if !ok {
		t.Fatalf("expected 'tag' to be a string, got %T: %v", tag, tag)
	}
	if tagStr != "execute_command" {
		t.Errorf("expected tag 'execute_command', got %q", tagStr)
	}
}

// TestStripCodeBlockXML_NoCodeBlock verifies that content without code blocks
// passes through unchanged.
func TestStripCodeBlockXML_NoCodeBlock(t *testing.T) {
	input := "Hello world\n<read_file>\n  <path>test.txt</path>\n</read_file>"
	result := stripCodeBlockXML(input)
	if result != input {
		t.Errorf("expected unchanged content, got:\n%s", result)
	}
}

// TestStripCodeBlockXML_XmlCodeBlock verifies that content inside ```xml...```
// code blocks is removed (FIX-291, Scenario 1).
func TestStripCodeBlockXML_XmlCodeBlock(t *testing.T) {
	input := "First, let me read:\n\n```xml\n<read_file>\n  <intent>Need to examine file</intent>\n  <path>src/main.go</path>\n  <start_line>1</start_line>\n  <end_line>50</end_line>\n</read_file>\n```\n\nThen execute:"
	result := stripCodeBlockXML(input)
	// The code block content should be removed, but surrounding text preserved
	if strings.Contains(result, "read_file") {
		t.Errorf("code block content should be removed, got:\n%s", result)
	}
	if !strings.Contains(result, "First, let me read") {
		t.Errorf("text before code block should be preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "Then execute") {
		t.Errorf("text after code block should be preserved, got:\n%s", result)
	}
	t.Logf("Stripped result:\n%s", result)
}

// TestStripCodeBlockXML_PlainCodeBlock verifies that content inside ```...```
// code blocks is removed (FIX-291, Scenario 2).
func TestStripCodeBlockXML_PlainCodeBlock(t *testing.T) {
	input := "Example:\n\n```\n<write_to_file>\n  <intent>test</intent>\n  <mode>new</mode>\n  <path>test.txt</path>\n  <content>Hello</content>\n</write_to_file>\n```"
	result := stripCodeBlockXML(input)
	if strings.Contains(result, "write_to_file") {
		t.Errorf("code block content should be removed, got:\n%s", result)
	}
	if !strings.Contains(result, "Example") {
		t.Errorf("text before code block should be preserved")
	}
	t.Logf("Stripped result:\n%s", result)
}

// TestStripCodeBlockXML_MultipleCodeBlocks verifies that multiple code blocks
// are all removed.
func TestStripCodeBlockXML_MultipleCodeBlocks(t *testing.T) {
	input := "First:\n```xml\n<list_files>\n  <path>/tmp</path>\n</list_files>\n```\n\nSecond:\n```\n<execute_command>\n  <command>ls</command>\n</execute_command>\n```\n\nDone."
	result := stripCodeBlockXML(input)
	if strings.Contains(result, "list_files") {
		t.Errorf("first code block content should be removed")
	}
	if strings.Contains(result, "execute_command") {
		t.Errorf("second code block content should be removed")
	}
	if !strings.Contains(result, "Done.") {
		t.Errorf("text after all code blocks should be preserved")
	}
	t.Logf("Stripped result:\n%s", result)
}

// TestParseXMLToolCallsWithTools_IgnoresCodeBlockXML verifies that when XML
// tool calls are inside a code block, they are NOT parsed as real tool calls
// (FIX-291, Scenario 3).
func TestParseXMLToolCallsWithTools_IgnoresCodeBlockXML(t *testing.T) {
	input := "Let me explore:\n\n```xml\n<list_files>\n  <intent>explore</intent>\n  <path>/home</path>\n</list_files>\n```\n\n```xml\n<execute_command>\n  <intent>list</intent>\n  <command>ls -la</command>\n</execute_command>\n```\n\nAll done."

	calls := ParseXMLToolCallsWithTools(input, []llm.Tool{
		{
			Name: "list_files",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"intent": map[string]interface{}{"type": "string"},
					"path":   map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name: "execute_command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"intent":  map[string]interface{}{"type": "string"},
					"command": map[string]interface{}{"type": "string"},
				},
			},
		},
	})
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls (all XML in code blocks), got %d", len(calls))
		for _, c := range calls {
			t.Logf("  call: %s", c.Name)
		}
	}
}

// TestParseXMLToolCallsWithTools_MixedCodeBlockAndReal verifies that code
// block content is ignored while real XML tool calls outside code blocks
// are still correctly parsed (FIX-291, Scenario 4).
func TestParseXMLToolCallsWithTools_MixedCodeBlockAndReal(t *testing.T) {
	input := "Example usage:\n\n```xml\n<search_files>\n  <intent>example</intent>\n  <path>src</path>\n  <regex>func main</regex>\n  <file_pattern>*.go</file_pattern>\n</search_files>\n```\n\nNow actually searching:\n<search_files>\n  <intent>search for main</intent>\n  <path>src</path>\n  <regex>func main</regex>\n  <file_pattern>*.go</file_pattern>\n</search_files>"

	calls := ParseXMLToolCallsWithTools(input, []llm.Tool{
		{
			Name: "search_files",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"intent":       map[string]interface{}{"type": "string"},
					"path":         map[string]interface{}{"type": "string"},
					"regex":        map[string]interface{}{"type": "string"},
					"file_pattern": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"intent", "path", "regex"},
			},
		},
	})
	if len(calls) != 1 {
		t.Fatalf("expected 1 real tool call, got %d", len(calls))
	}
	if calls[0].Name != "search_files" {
		t.Errorf("expected 'search_files', got %q", calls[0].Name)
	}
	// Verify it's the real one (with "search for main" intent)
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}
	if intent, ok := args["intent"].(string); !ok || intent != "search for main" {
		t.Errorf("expected intent 'search for main', got %q", intent)
	}
}

// TestStripCodeBlockXML_BacktickAtContentStart verifies that content starting
// with a code block is handled correctly.
func TestStripCodeBlockXML_BacktickAtContentStart(t *testing.T) {
	input := "```\n<execute_command>\n  <command>ls</command>\n</execute_command>\n```\n\nDone."
	result := stripCodeBlockXML(input)
	if strings.Contains(result, "execute_command") {
		t.Errorf("code block at start should be removed, got:\n%s", result)
	}
	if !strings.Contains(result, "Done.") {
		t.Errorf("text after should be preserved")
	}
}

// TestStripCodeBlockXML_IncompleteFence verifies that a code block without
// a closing fence causes the rest of content to be treated as code block.
func TestStripCodeBlockXML_IncompleteFence(t *testing.T) {
	input := "Code:\n```\n<write_to_file>\n  <content>test</content>\n</write_to_file>"
	result := stripCodeBlockXML(input)
	// Without closing fence, everything after opening should be stripped
	if !strings.Contains(result, "Code:") {
		t.Errorf("text before code block should be preserved")
	}
	if strings.Contains(result, "write_to_file") {
		t.Errorf("unclosed code block content should be stripped")
	}
	t.Logf("Result: %q", result)
}

func TestParseXMLToolCalls_ItemMissingCloseTag(t *testing.T) {
	// FIX-255: When <item> inside a <replacements> block is missing its </item>
	// closing tag, the parser should propagate the nested parse error up to the
	// caller, producing an _xml_parse_error that clearly states the error.
	// Previously the nested error was silently swallowed and the tool received
	// a plain string instead of an array, causing confusing "missing 'search'
	// and 'replace' fields" errors.
	xmlInput := "<replace_in_file>\n<intent>update report</intent>\n<path>test.md</path>\n<replacements>\n  <item>\n    <search>old content</search>\n    <replace>new content</replace>\n</replacements>\n</replace_in_file>"

	tools := []llm.Tool{
		{
			Name: "replace_in_file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"intent": map[string]interface{}{
						"type":        "string",
						"description": "intent",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "file path",
					},
					"replacements": map[string]interface{}{
						"type":        "array",
						"description": "replacements",
					},
				},
			},
		},
	}

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q\nArguments: %s", call.Name, call.Arguments)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse error arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	errMsg, ok := args["error"]
	if !ok {
		t.Fatalf("missing 'error' field in error arguments, args: %v", args)
	}
	errStr, ok := errMsg.(string)
	if !ok {
		t.Fatalf("expected 'error' to be a string, got %T: %v", errMsg, errMsg)
	}

	// The error message should mention the missing </item> closing tag
	if !strings.Contains(errStr, "item") {
		t.Errorf("error message should mention <item>, got: %s", errStr)
	}
	if !strings.Contains(errStr, "闭合标签") {
		t.Errorf("error message should mention '闭合标签' (missing closing tag), got: %s", errStr)
	}

	// The error should clearly state the root cause: <item> is missing its closing tag.
	// It should NOT produce misleading downstream errors about parameter fields.
	if !strings.Contains(errStr, "<item> 缺少闭合标签") {
		t.Errorf("error should state root cause '<item> 缺少闭合标签', got: %s", errStr)
	}

	t.Logf("Error message: %s", errStr)

	tag, ok := args["tag"]
	if !ok {
		t.Fatalf("missing 'tag' field in error arguments, args: %v", args)
	}
	tagStr, ok := tag.(string)
	if !ok {
		t.Fatalf("expected 'tag' to be a string, got %T: %v", tag, tag)
	}
	if tagStr != "replace_in_file" {
		t.Errorf("expected tag 'replace_in_file', got %q", tagStr)
	}
}

// TestParseXMLToolCallsWithTools_Stage1_TailTagReverseMatch verifies that when the opening
// tag is misspelled but the closing tag matches a known tool name, an _xml_parse_error
// is produced (FEATURE-293 stage 1).
func TestParseXMLToolCallsWithTools_Stage1_TailTagReverseMatch(t *testing.T) {
	tools := []llm.Tool{
		{
			Name: "read_file",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"path":   map[string]interface{}{"type": "string"},
					"intent": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"intent", "path"},
			},
		},
	}
	// Head tag "reed_file" is unknown, but tail tag "read_file" is known
	xmlInput := "<reed_file>\n  <path>/tmp/test.txt</path>\n  <intent>read file</intent>\n</read_file>"

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 _xml_parse_error call (stage1), got %d", len(calls))
	}
	if calls[0].Name != "_xml_parse_error" {
		t.Errorf("expected tool name '_xml_parse_error', got %q", calls[0].Name)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v", err)
	}
	errMsg, _ := args["error"].(string)
	if !strings.Contains(errMsg, "read_file") {
		t.Errorf("error message should mention 'read_file', got: %s", errMsg)
	}
	tag, _ := args["tag"].(string)
	if tag != "reed_file" {
		t.Errorf("expected tag 'reed_file', got %q", tag)
	}
}

// TestParseXMLToolCallsWithTools_Stage2_ParamSignature verifies that when both head and tail
// tags are unknown but inner parameter tags match known parameters, an _xml_parse_error
// is produced (FEATURE-293 stage 2).
func TestParseXMLToolCallsWithTools_Stage2_ParamSignature(t *testing.T) {
	tools := []llm.Tool{
		{
			Name: "read_file",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"path":   map[string]interface{}{"type": "string"},
					"intent": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"intent", "path"},
			},
		},
	}
	// Neither head nor tail match any known tool name, but 'path' is a known parameter.
	// Both open and close use the same unknown name 'reed_file'.
	xmlInput := "<reed_file>\n  <path>/tmp/test.txt</path>\n</reed_file>"

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 _xml_parse_error call (stage2), got %d", len(calls))
	}
	if calls[0].Name != "_xml_parse_error" {
		t.Errorf("expected tool name '_xml_parse_error', got %q", calls[0].Name)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v", err)
	}
	errMsg, _ := args["error"].(string)
	if !strings.Contains(errMsg, "path") {
		t.Errorf("error message should mention 'path', got: %s", errMsg)
	}
	tag, _ := args["tag"].(string)
	if tag != "reed_file" {
		t.Errorf("expected tag 'reed_file', got %q", tag)
	}
}

// TestParseXMLToolCallsWithTools_HTMLNotMisdetectedAsToolCall verifies that common HTML
// tags (<div>, <p>) with no known parameter names do NOT trigger stage 2 detection.
// TestHasIncompleteToolCall_DetectsKnownTool verifies that hasIncompleteToolCall
// returns true when the content contains an opening <known_tool_name> tag
// that didn't result in a valid XML parse (FEATURE-293 phase 3).
func TestHasIncompleteToolCall_DetectsKnownTool(t *testing.T) {
	tools := []llm.Tool{
		{Name: "read_file"},
		{Name: "execute_command"},
		{Name: "write_to_file"},
	}
	// Content has <read_file appearing but broken XML
	content := "Let me try:\n<read_file path=\"/tmp\">\nwait..."
	got := hasIncompleteToolCall(content, tools)
	if !got {
		t.Errorf("expected hasIncompleteToolCall=true for <read_file in content")
	}
}

// TestHasIncompleteToolCall_SkipsNonToolTags verifies that known non-tool tags
// (thinking, answer, etc.) are not detected as incomplete tool calls.
func TestHasIncompleteToolCall_SkipsNonToolTags(t *testing.T) {
	tools := []llm.Tool{
		{Name: "read_file"},
		{Name: "execute_command"},
	}
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"thinking tag", "<thinking>let me process...</thinking>", false},
		{"answer tag", "<answer>42</answer>", false},
		{"plain text", "The answer is 42", false},
		{"known tool tag", "<read_file><path>/tmp</path></read_file>", true},
		{"incomplete known tool", "<execute_command\nI need to", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasIncompleteToolCall(tt.content, tools)
			if got != tt.want {
				t.Errorf("hasIncompleteToolCall=%v, want=%v for content: %q", got, tt.want, tt.content)
			}
		})
	}
}

// TestHasIncompleteToolCall_NoFalsePositivesForHTML verifies that common HTML
// tags (div, p, span, etc.) do NOT trigger hasIncompleteToolCall.
func TestHasIncompleteToolCall_NoFalsePositivesForHTML(t *testing.T) {
	tools := []llm.Tool{
		{Name: "read_file"},
		{Name: "execute_command"},
	}
	htmlContent := "<div class=\"main\">\n  <p>hello</p>\n</div>"
	got := hasIncompleteToolCall(htmlContent, tools)
	if got {
		t.Errorf("HTML content should not trigger hasIncompleteToolCall")
	}
}

func TestParseXMLToolCallsWithTools_HTMLNotMisdetectedAsToolCall(t *testing.T) {
	tools := []llm.Tool{
		{
			Name: "read_file",
			Parameters: map[string]interface{}{
				"properties": map[string]interface{}{
					"path":   map[string]interface{}{"type": "string"},
					"intent": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"intent", "path"},
			},
		},
	}
	// <div> is not a known tool, <p> is not a known parameter — should produce 0 calls
	xmlInput := "<div>\n  <p>hello world</p>\n</div>"

	calls := ParseXMLToolCallsWithTools(xmlInput, tools)
	if len(calls) != 0 {
		t.Fatalf("expected 0 tool calls (HTML should not be detected), got %d", len(calls))
	}
}
