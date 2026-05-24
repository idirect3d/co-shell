package agent

import (
	"encoding/json"
	"testing"
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
