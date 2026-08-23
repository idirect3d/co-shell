// Package web - FEATURE-425 tests: line-range file reading (on-demand
// preview loading) and git diff line-level parsing for the text previewer.
//
// Author: L.Shuang
// Created: 2026-08-23
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestFileLinesRange verifies /api/file with start/end returns the requested
// line slice plus the total line count (UC-006).
func TestFileLinesRange(t *testing.T) {
	_, ts, root := newTestServer(t)
	// hello.txt is "hi" (1 line). Write a multi-line file.
	content := "l1\nl2\nl3\nl4\nl5\n"
	if err := os.WriteFile(filepath.Join(root, "multi.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var body struct {
		Total int      `json:"total"`
		Lines []string `json:"lines"`
	}
	getJSON(t, ts.URL+"/api/file?path=multi.txt&start=2&end=4", &body)
	if body.Total != 5 {
		t.Errorf("total = %d, want 5", body.Total)
	}
	if len(body.Lines) != 3 || body.Lines[0] != "l2" || body.Lines[2] != "l4" {
		t.Errorf("lines = %v, want [l2 l3 l4]", body.Lines)
	}
}

// TestFileLinesOpenEnd verifies an empty end reads to EOF.
func TestFileLinesOpenEnd(t *testing.T) {
	_, ts, root := newTestServer(t)
	content := "a\nb\nc\n"
	if err := os.WriteFile(filepath.Join(root, "open.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Total int      `json:"total"`
		Lines []string `json:"lines"`
	}
	getJSON(t, ts.URL+"/api/file?path=open.txt&start=2", &body)
	if body.Total != 3 || len(body.Lines) != 2 || body.Lines[0] != "b" {
		t.Errorf("open-end read = total %d lines %v, want 3 [b c]", body.Total, body.Lines)
	}
}

// TestFileLinesInvalid verifies bad start/end are rejected.
func TestFileLinesInvalid(t *testing.T) {
	_, ts, _ := newTestServer(t)
	for _, q := range []string{"start=0", "start=abc", "start=3&end=2"} {
		resp, err := http.Get(ts.URL + "/api/file?path=hello.txt&" + q)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("query %q status = %d, want 400", q, resp.StatusCode)
		}
	}
}

// TestParseGitDiff verifies unified diff parsing maps added/deleted lines to
// working-tree line numbers.
func TestParseGitDiff(t *testing.T) {
	diff := `diff --git a/f.go b/f.go
index 111..222 100644
--- a/f.go
+++ b/f.go
@@ -1,4 +1,5 @@
 line1
-line2
+line2b
 line3
+line4new
`
	got := parseGitDiff(diff)
	want := []gitDiffLine{
		{Line: 2, Status: "del"},
		{Line: 2, Status: "add"},
		{Line: 4, Status: "add"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestGitDiffNonRepo verifies /api/gitdiff returns an empty list when the
// workspace is not a git repository (UC-008).
func TestGitDiffNonRepo(t *testing.T) {
	_, ts, _ := newTestServer(t)
	var body struct {
		Lines []gitDiffLine `json:"lines"`
	}
	getJSON(t, ts.URL+"/api/gitdiff?path=hello.txt", &body)
	if len(body.Lines) != 0 {
		t.Errorf("non-repo gitdiff = %v, want empty", body.Lines)
	}
}

// TestGitDiffJSONShape verifies the gitdiff response decodes into the expected
// JSON shape (lines array of {line,status}).
func TestGitDiffJSONShape(t *testing.T) {
	_, ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/gitdiff?path=hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw map[string][]map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["lines"]; !ok {
		t.Errorf("response missing 'lines' key: %v", raw)
	}
}
