// Author: L.Shuang
// Created: 2026-08-28
// Last Modified: 2026-08-28
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

package agent

import "testing"

// fullMeta returns a complete, valid meta object for tests.
func fullMeta() map[string]interface{} {
	return map[string]interface{}{
		"meta": map[string]interface{}{
			"intent":           "test intent",
			"risk":             "high",
			"risk_reason":      "core config file",
			"affected_objects": []interface{}{"/abs/path/to/main.go"},
			"progress":         []interface{}{map[string]interface{}{"index": float64(0), "description": "step", "status": "in_progress"}},
		},
	}
}

// TestAssessRiskValid verifies a valid LLM risk is returned (UC-0005).
func TestAssessRiskValid(t *testing.T) {
	risk, reason, err := assessRisk(fullMeta())
	if err != nil {
		t.Fatalf("assessRisk: %v", err)
	}
	if risk != RiskHigh {
		t.Errorf("risk = %q, want %q", risk, RiskHigh)
	}
	if reason != "core config file" {
		t.Errorf("reason = %q, want %q", reason, "core config file")
	}
}

// TestAssessRiskMissing verifies a missing meta object is an error (UC-0005).
func TestAssessRiskMissing(t *testing.T) {
	_, _, err := assessRisk(map[string]interface{}{})
	if err == nil {
		t.Error("missing meta should error")
	}
}

// TestAssessRiskInvalid verifies an invalid risk value is an error.
func TestAssessRiskInvalid(t *testing.T) {
	m := fullMeta()
	m["meta"].(map[string]interface{})["risk"] = "extreme"
	_, _, err := assessRisk(m)
	if err == nil {
		t.Error("invalid risk should error")
	}
}

// TestValidateMeta verifies every meta field is required (FEATURE-447).
func TestValidateMeta(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{"missing intent", func(m map[string]interface{}) { delete(m["meta"].(map[string]interface{}), "intent") }},
		{"missing risk", func(m map[string]interface{}) { delete(m["meta"].(map[string]interface{}), "risk") }},
		{"missing risk_reason", func(m map[string]interface{}) { delete(m["meta"].(map[string]interface{}), "risk_reason") }},
		{"missing affected_objects", func(m map[string]interface{}) { delete(m["meta"].(map[string]interface{}), "affected_objects") }},
		{"missing progress", func(m map[string]interface{}) { delete(m["meta"].(map[string]interface{}), "progress") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fullMeta()
			tc.mutate(m)
			if err := validateMeta(m); err == nil {
				t.Errorf("validateMeta should error when %s", tc.name)
			}
		})
	}
	// A complete meta object passes.
	if err := validateMeta(fullMeta()); err != nil {
		t.Errorf("validateMeta should pass for complete meta: %v", err)
	}
}

// TestAffectedFilesFromLLM verifies affected files come from the LLM's
// affected_objects field (UC-0008).
func TestAffectedFilesFromLLM(t *testing.T) {
	files := (&Agent{}).affectedFiles(map[string]interface{}{
		"meta": map[string]interface{}{
			"affected_objects": []interface{}{"/abs/path/to/main.go", "/abs/path/to/agent/"},
		},
	})
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Path != "/abs/path/to/main.go" {
		t.Errorf("file[0].path = %q", files[0].Path)
	}
}

// TestAffectedFilesTrailingColon verifies a trailing colon the LLM may append
// after a path is stripped so the path matches the workspace tree exactly.
func TestAffectedFilesTrailingColon(t *testing.T) {
	files := (&Agent{}).affectedFiles(map[string]interface{}{
		"meta": map[string]interface{}{
			"affected_objects": []interface{}{"/abs/path/to/main.go:", "/abs/path/to/agent/:", "  /abs/path/to/other.go  "},
		},
	})
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}
	if files[0].Path != "/abs/path/to/main.go" {
		t.Errorf("file[0].path = %q, want /abs/path/to/main.go", files[0].Path)
	}
	if files[1].Path != "/abs/path/to/agent" {
		t.Errorf("file[1].path = %q, want /abs/path/to/agent", files[1].Path)
	}
	if files[2].Path != "/abs/path/to/other.go" {
		t.Errorf("file[2].path = %q, want /abs/path/to/other.go", files[2].Path)
	}
}

// TestAffectedFilesMax3 verifies affected files are capped at 3 (UC-0009).
func TestAffectedFilesMax3(t *testing.T) {
	files := (&Agent{}).affectedFiles(map[string]interface{}{
		"meta": map[string]interface{}{
			"affected_objects": []interface{}{"/a", "/b", "/c", "/d", "/e"},
		},
	})
	if len(files) != maxPredictedFiles {
		t.Errorf("got %d files, want %d", len(files), maxPredictedFiles)
	}
}

// TestAffectedFilesSensitive verifies a sensitive path is flagged (UC-0004).
func TestAffectedFilesSensitive(t *testing.T) {
	files := (&Agent{}).affectedFiles(map[string]interface{}{
		"meta": map[string]interface{}{
			"affected_objects": []interface{}{"/Users/direct3d/.ssh/config"},
		},
	})
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if !files[0].Sensitive {
		t.Errorf("file should be flagged sensitive")
	}
}

// TestAffectedFilesRelativize verifies absolute paths under the workspace are
// converted to workspace-relative paths so the frontend can match the tree.
func TestAffectedFilesRelativize(t *testing.T) {
	a := &Agent{workspacePath: "/ws"}
	files := a.affectedFiles(map[string]interface{}{
		"meta": map[string]interface{}{
			"affected_objects": []interface{}{"/ws/src/main.go", "/ws/agent/", "/outside/other.go"},
		},
	})
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}
	if files[0].Path != "src/main.go" {
		t.Errorf("file[0].path = %q, want src/main.go", files[0].Path)
	}
	if files[1].Path != "agent" {
		t.Errorf("file[1].path = %q, want agent", files[1].Path)
	}
	// Paths outside the workspace stay absolute.
	if files[2].Path != "/outside/other.go" {
		t.Errorf("file[2].path = %q, want /outside/other.go", files[2].Path)
	}
}

// TestParseProgressSteps verifies progress parsing (UC-0011).
func TestParseProgressSteps(t *testing.T) {
	steps, err := parseProgressSteps(map[string]interface{}{
		"meta": map[string]interface{}{
			"progress": []interface{}{
				map[string]interface{}{"index": float64(0), "description": "step 0", "status": "in_progress"},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseProgressSteps: %v", err)
	}
	if len(steps) != 1 || steps[0].Index != 0 || steps[0].Status != "in_progress" {
		t.Errorf("steps = %+v", steps)
	}
}

// TestParseProgressStepsEmpty verifies empty/absent progress yields nil
// (UC-0014).
func TestParseProgressStepsEmpty(t *testing.T) {
	steps, err := parseProgressSteps(map[string]interface{}{})
	if err != nil || steps != nil {
		t.Errorf("empty progress: steps=%v err=%v, want nil/nil", steps, err)
	}
}

// TestParseProgressStepsMissingIndex verifies a missing index is an error.
func TestParseProgressStepsMissingIndex(t *testing.T) {
	_, err := parseProgressSteps(map[string]interface{}{
		"meta": map[string]interface{}{
			"progress": []interface{}{
				map[string]interface{}{"description": "step", "status": "pending"},
			},
		},
	})
	if err == nil {
		t.Error("missing index should error")
	}
}
