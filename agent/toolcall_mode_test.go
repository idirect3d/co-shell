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
