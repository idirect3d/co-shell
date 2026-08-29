package agent

import (
	"encoding/json"
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// TestReproMetaNested verifies the root cause: when the LLM nests <path> and
// <regex> inside the <meta> tag, parseXMLChildrenToJSON faithfully places them
// inside the meta object, so args["path"] is absent at the top level and
// searchFilesTool reports "path argument is required".
func TestReproMetaNested(t *testing.T) {
	tools := []llm.Tool{
		{
			Name: "search_files",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta":         map[string]interface{}{"type": "object"},
					"path":         map[string]interface{}{"type": "string"},
					"regex":        map[string]interface{}{"type": "string"},
					"file_pattern": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"meta", "path", "regex"},
			},
		},
	}

	// Simulate the LLM output that triggers the bug: path and regex are
	// mistakenly nested INSIDE the meta tag.
	input := `<cs:search_files>
  <cs:file_pattern>*.go</cs:file_pattern>
  <cs:meta>
    <cs:affected_objects>/Users/direct3d/github/co-shell/agent</cs:affected_objects>
    <cs:intent>定位 jsonValue 函数实现</cs:intent>
    <cs:path>/Users/direct3d/github/co-shell/agent</cs:path>
    <cs:progress>
      <cs:item>
        <cs:description>调查现状</cs:description>
        <cs:index>0</cs:index>
        <cs:status>in_progress</cs:status>
      </cs:item>
    </cs:progress>
    <cs:regex>func jsonValue</cs:regex>
    <cs:risk>low</cs:risk>
    <cs:risk_reason>只读搜索</cs:risk_reason>
  </cs:meta>
</cs:search_files>`

	calls := ParseXMLToolCallsWithTools(input, tools)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	t.Logf("parsed args: %v", args)

	// Confirm the bug: path is inside meta, not at top level.
	if _, ok := args["path"]; !ok {
		t.Logf("CONFIRMED: args[path] absent at top level (nested inside meta)")
	} else {
		t.Logf("args[path] present at top level")
	}
	if meta, ok := args["meta"].(map[string]interface{}); ok {
		if p, ok := meta["path"]; ok {
			t.Logf("CONFIRMED: meta[path] = %v", p)
		}
	}
}
