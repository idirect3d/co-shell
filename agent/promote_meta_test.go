package agent

import (
	"testing"
)

// TestPromoteMisplacedMetaParams verifies that tool parameters mistakenly nested
// inside the meta object are promoted to the top level, while legitimate meta
// fields (intent/risk/risk_reason/affected_objects/progress) stay inside meta.
func TestPromoteMisplacedMetaParams(t *testing.T) {
	// Case 1: path and regex nested inside meta -> promoted to top level.
	args := map[string]interface{}{
		"file_pattern": "*.go",
		"meta": map[string]interface{}{
			"affected_objects": "/Users/direct3d/github/co-shell/agent",
			"intent":           "定位 jsonValue 函数实现",
			"path":             "/Users/direct3d/github/co-shell/agent",
			"progress":         []interface{}{map[string]interface{}{"index": float64(0), "status": "in_progress"}},
			"regex":            "func jsonValue",
			"risk":             "low",
			"risk_reason":      "只读搜索",
		},
	}
	promoteMisplacedMetaParams(args)

	if p, ok := args["path"].(string); !ok || p != "/Users/direct3d/github/co-shell/agent" {
		t.Errorf("path not promoted: args=%v", args)
	}
	if r, ok := args["regex"].(string); !ok || r != "func jsonValue" {
		t.Errorf("regex not promoted: args=%v", args)
	}
	// Legitimate meta fields must remain inside meta.
	meta := args["meta"].(map[string]interface{})
	if _, ok := meta["intent"]; !ok {
		t.Errorf("intent should stay inside meta: %v", args)
	}
	if _, ok := meta["risk"]; !ok {
		t.Errorf("risk should stay inside meta: %v", args)
	}
	if _, ok := meta["affected_objects"]; !ok {
		t.Errorf("affected_objects should stay inside meta: %v", args)
	}
	if _, ok := meta["progress"]; !ok {
		t.Errorf("progress should stay inside meta: %v", args)
	}

	// Case 2: top-level path already present -> must NOT be overwritten by meta's.
	args2 := map[string]interface{}{
		"path": "/top/level",
		"meta": map[string]interface{}{
			"path": "/inside/meta",
			"risk": "low",
		},
	}
	promoteMisplacedMetaParams(args2)
	if p, ok := args2["path"].(string); !ok || p != "/top/level" {
		t.Errorf("top-level path should win: %v", args2)
	}

	// Case 3: no meta object -> no-op.
	args3 := map[string]interface{}{"path": "/x"}
	promoteMisplacedMetaParams(args3)
	if _, ok := args3["path"]; !ok {
		t.Errorf("no-meta case should be a no-op: %v", args3)
	}
}
