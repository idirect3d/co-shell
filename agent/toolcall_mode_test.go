package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

func TestParseXMLToolCalls_CommandWithSpecialChars(t *testing.T) {
	// Simulate the exact XML from the user's feedback
	xmlInput := `<execute_command>
<command>cd /Users/direct3d/agent/researcher/research/浏览器自动化与模拟人操作技术调研 && curl -s "https://html.duckduckgo.com/html/?q=browser+automation+framework+Selenium+Playwright+Puppeteer+comparison+2025" | grep -oP 'class="result__snippet"[^>]*>[^<]*' | head -20
</command>
<timeout_seconds>
30
</timeout_seconds>
</execute_command>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "execute_command" {
		t.Errorf("expected tool name 'execute_command', got %q", call.Name)
	}

	// Parse the arguments JSON
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	// Check command
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

	// Check timeout_seconds
	ts, ok := args["timeout_seconds"]
	if !ok {
		t.Fatalf("missing 'timeout_seconds' argument, args: %v", args)
	}
	// timeout_seconds should be a number
	tsFloat, ok := ts.(float64)
	if !ok {
		t.Fatalf("expected 'timeout_seconds' to be a number, got %T: %v", ts, ts)
	}
	if tsFloat != 30 {
		t.Errorf("expected timeout_seconds=30, got %v", tsFloat)
	}
}

func TestParseXMLToolCalls_CDATA(t *testing.T) {
	xmlInput := `<execute_command>
<command><![CDATA[cd /path && curl -s "https://example.com/?q=test&lang=go" | grep -oP 'pattern' | head -20]]></command>
<timeout_seconds>30</timeout_seconds>
</execute_command>`

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
	// LLM puts special chars like '<', '>', '&' in content without CDATA wrapping.
	// The parser should still be able to find the closing tag and extract content.
	xmlInput := `<execute_command>
<command>curl -s "https://example.com/?q=test&lang=go" | grep -oP 'class="result__snippet"[^>]*>[^<]*' | head -20</command>
<timeout_seconds>30</timeout_seconds>
</execute_command>`

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

	// Verify the command contains the special chars
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
	// LLM misspells parameter name "command" as "commmand" (note: 3 m's).
	// The opening tag is <commmand> but the closing tag is </command> (correct spelling).
	// This mismatch means the parser cannot find the matching close tag for <commmand>,
	// and should return an _xml_parse_error instead of attempting to execute.
	xmlInput := `<execute_command>
<commmand>ls -la</command>
<timeout_seconds>30</timeout_seconds>
</execute_command>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	// Parse the error arguments
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

	// The error message should mention the parameter parse issue
	if !strings.Contains(errStr, "参数") && !strings.Contains(errStr, "commmand") {
		t.Errorf("error message should mention the parameter issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)

	// Verify the tag name is correct
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
	// LLM writes a parameter without a closing tag (e.g., <command>ls -la without </command>).
	// The parser should detect the missing closing tag and return an _xml_parse_error.
	xmlInput := `<execute_command>
<command>ls -la
<timeout_seconds>30</timeout_seconds>
</execute_command>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	// Parse the error arguments
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

	// The error message should mention the parameter parse issue
	if !strings.Contains(errStr, "参数") {
		t.Errorf("error message should mention the parameter issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidTagNameWithEquals(t *testing.T) {
	// LLM uses attribute-like syntax: <parameter=step_id> instead of <step_id>value</step_id>
	// The parser should detect the '=' in the tag name and return an _xml_parse_error.
	xmlInput := `<update_task_step>
<step_id>1</step_id>
<status>completed</status>
<parameter=note>This is a note</parameter=note>
</update_task_step>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	// Parse the error arguments
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

	// The error message should mention the '=' character issue
	if !strings.Contains(errStr, "=") {
		t.Errorf("error message should mention the '=' character issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidTagNameWithSpace(t *testing.T) {
	// LLM uses attribute-like syntax: <parameter name=step_id> instead of <step_id>value</step_id>
	// The parser should detect the space in the tag name and return an _xml_parse_error.
	xmlInput := `<update_task_step>
<step_id>1</step_id>
<status>completed</status>
<parameter name=note>This is a note</parameter name=note>
</update_task_step>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call (error), got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "_xml_parse_error" {
		t.Fatalf("expected error tool name '_xml_parse_error', got %q", call.Name)
	}

	// Parse the error arguments
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

	// The error message should mention the space issue
	if !strings.Contains(errStr, "空格") && !strings.Contains(errStr, "parameter") {
		t.Errorf("error message should mention the space or attribute issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_ToolTagWithSpace(t *testing.T) {
	// LLM adds attribute to the tool tag: <execute_command timeout=30>
	// The parser should detect the space in the tool tag and return an _xml_parse_error.
	xmlInput := `<execute_command timeout=30>
<command>ls -la</command>
</execute_command>`

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

	// The error message should mention the space/attribute issue
	if !strings.Contains(errStr, "空格") && !strings.Contains(errStr, "属性") {
		t.Errorf("error message should mention the space or attribute issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_ToolTagWithEquals(t *testing.T) {
	// LLM uses attribute-like syntax on the tool tag: <execute_command=xxx>
	// The parser should detect the '=' in the tag name and return an _xml_parse_error.
	xmlInput := `<execute_command=xxx>
<command>ls -la</command>
</execute_command=xxx>`

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

	// The error message should mention the '=' character issue
	if !strings.Contains(errStr, "=") {
		t.Errorf("error message should mention the '=' character issue, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_InvalidParamName(t *testing.T) {
	// LLM uses a misspelled parameter name "commmand" (3 m's) instead of "command".
	// With tools provided, ParseXMLToolCallsWithTools should detect this mismatch.
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

	// Note: <commmand> has 3 m's, but the closing tag is </commmand> (also 3 m's),
	// so the XML structure is valid. The parameter name just doesn't match the tool definition.
	xmlInput := `<execute_command>
<commmand>ls -la</commmand>
<timeout_seconds>30</timeout_seconds>
</execute_command>`

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

	// The error message should mention the invalid parameter name
	if !strings.Contains(errStr, "commmand") {
		t.Errorf("error message should mention the invalid parameter name 'commmand', got: %s", errStr)
	}
	if !strings.Contains(errStr, "合法参数") {
		t.Errorf("error message should mention '合法参数', got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)
}

func TestParseXMLToolCalls_MissingParentCloseTag(t *testing.T) {
	// <execute_command> is opened but never closed — only its child <command> is closed.
	// The parser should detect that </execute_command> is missing and return an _xml_parse_error
	// with a clear message, not a confusing downstream error about parameter parsing.
	xmlInput := `<execute_command><command></command>`

	// Use ParseXMLToolCallsWithTools so that execute_command is recognized as a known tool.
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

	// Parse the error arguments
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

	// The error message should clearly state that the closing tag is missing
	if !strings.Contains(errStr, "闭合标签") && !strings.Contains(errStr, "execute_command") {
		t.Errorf("error message should mention the missing close tag for execute_command, got: %s", errStr)
	}
	t.Logf("Error message: %s", errStr)

	// Verify the tag name in the error
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

func TestParseXMLToolCalls_CDATAWithXMLContent(t *testing.T) {

	// CDATA wrapping content that contains XML-like tags
	xmlInput := `<write_to_file>
<path>output/result.md</path>
<content><![CDATA[# Result

This is an example of XML content:
<note>
  <to>User</to>
  <message>Hello</message>
</note>]]></content>
</write_to_file>`

	calls := ParseXMLToolCalls(xmlInput)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v\nJSON: %s", err, call.Arguments)
	}

	content, ok := args["content"]
	if !ok {
		t.Fatalf("missing 'content' argument, args: %v", args)
	}
	contentStr, ok := content.(string)
	if !ok {
		t.Fatalf("expected 'content' to be a string, got %T: %v", content, content)
	}
	if len(contentStr) == 0 {
		t.Fatal("expected non-empty content string")
	}
	t.Logf("content: %s", contentStr)
}
