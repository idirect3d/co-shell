package agent

import (
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/mcp"
)

// TestInjectMetaParamAllTools verifies every tool built by buildToolsInternal
// carries the unified meta object parameter (required, first) and no longer
// declares a top-level intent parameter. Vision tools additionally declare the
// required instruct parameter (FEATURE-447).
func TestInjectMetaParamAllTools(t *testing.T) {
	ag := &Agent{
		toolCallEnabled:       true,
		intentExposureEnabled: true,
		mcpMgr:                mcp.NewManager(),
		toolModes:             map[string]string{},
	}
	ag.SetConfig(&config.Config{})
	ag.SyncToolModes(&config.Config{})

	tools := ag.buildToolsInternal()
	if len(tools) == 0 {
		t.Fatal("buildToolsInternal returned no tools")
	}

	for _, tool := range tools {
		props, _ := tool.Parameters["properties"].(map[string]interface{})
		if props == nil {
			t.Errorf("tool %q: no properties map", tool.Name)
			continue
		}
		// FEATURE-450: track_task_progress no longer requires meta. Assert meta
		// is NOT declared and NOT in its required list, then skip the generic
		// meta-required assertions for it.
		// FEATURE-452: attempt_completion now requires meta again (it reports
		// final progress via meta.progress), so it follows the generic path.
		if tool.Name == "track_task_progress" {
			if _, ok := props["meta"]; ok {
				t.Errorf("tool %q: meta should not be declared", tool.Name)
			}
			for _, s := range requiredStrings(tool.Parameters["required"]) {
				if s == "meta" {
					t.Errorf("tool %q: meta should not be in required list", tool.Name)
				}
			}
			continue
		}
		// FIX-510: reorganize_context keeps the meta property declaration (it may
		// be supplied optionally) but must NOT list meta as required, so a
		// context reorganization is never blocked by meta format problems.
		if tool.Name == "reorganize_context" {
			if _, ok := props["meta"]; !ok {
				t.Errorf("tool %q: meta should stay declared as optional", tool.Name)
			}
			for _, s := range requiredStrings(tool.Parameters["required"]) {
				if s == "meta" {
					t.Errorf("tool %q: meta must not be in required list", tool.Name)
				}
			}
			continue
		}
		// meta must be declared.
		if _, ok := props["meta"]; !ok {
			t.Errorf("tool %q: missing meta parameter declaration", tool.Name)
		}
		// legacy top-level intent must be removed.
		if _, ok := props["intent"]; ok {
			t.Errorf("tool %q: legacy top-level intent still declared", tool.Name)
		}
		// meta must be in required list.
		required := requiredStrings(tool.Parameters["required"])
		hasMeta := false
		for _, s := range required {
			if s == "meta" {
				hasMeta = true
			}
		}
		if !hasMeta {
			t.Errorf("tool %q: meta not in required list", tool.Name)
		}
		// Vision tools must declare instruct (required).
		if tool.Name == "visual_analysis" || tool.Name == "browser_screenshot" {
			if _, ok := props["instruct"]; !ok {
				t.Errorf("tool %q: missing instruct parameter", tool.Name)
			}
			hasInstruct := false
			for _, s := range required {
				if s == "instruct" {
					hasInstruct = true
				}
			}
			if !hasInstruct {
				t.Errorf("tool %q: instruct not in required list", tool.Name)
			}
		}
	}
}

// TestInjectMetaParamUnit verifies injectMetaParam on a single tool definition.
func TestInjectMetaParamUnit(t *testing.T) {
	tool := &llm.Tool{
		Name: "sample_tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"intent": map[string]interface{}{"type": "string"},
				"path":   map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"intent", "path"},
		},
	}
	injectMetaParam(tool)

	props := tool.Parameters["properties"].(map[string]interface{})
	if _, ok := props["intent"]; ok {
		t.Error("intent should be removed")
	}
	if _, ok := props["meta"]; !ok {
		t.Error("meta should be added")
	}
	required := tool.Parameters["required"].([]interface{})
	if len(required) != 2 || required[0] != "meta" || required[1] != "path" {
		t.Errorf("required = %v, want [meta path]", required)
	}
}

// requiredStrings normalizes a tool's "required" field (which may be a
// []string or []interface{} of strings) into a []string for assertion.
func requiredStrings(v interface{}) []string {
	switch r := v.(type) {
	case []string:
		return r
	case []interface{}:
		out := make([]string, 0, len(r))
		for _, item := range r {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
