// Package web - FIX-589 workspace tree level API tests.
//
// FIX-589 turned GET /api/tree into a per-level endpoint: by default it returns
// the requested directory (workspace root when "path" is empty) with its
// immediate children only, ?path= selects the directory and ?depth=full keeps
// the recursive listing. These tests lock that contract, the git status /
// changed-file badges of every level, the exclusion behaviour and the
// path-traversal protection.
//
// Author: L.Shuang
// Created: 2026-10-03
// Last Modified: 2026-10-03
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// fix589Layout builds a small nested workspace:
//
//	top.txt  sub/a.txt  sub/deep/b.txt  other/c.txt
func fix589Layout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"sub/deep", "other"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"top.txt", "sub/a.txt", "sub/deep/b.txt", "other/c.txt"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// fix589Server serves the given workspace root with no exclusions.
func fix589Server(t *testing.T, root string) *httptest.Server {
	t.Helper()
	s := NewServer(root, ServerOptions{Lang: "zh", Version: "0.0.0", Build: "0"})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return ts
}

// TestFIX589TreeLevelDefault checks that the default response carries the root
// and its immediate children only, and stays small.
func TestFIX589TreeLevelDefault(t *testing.T) {
	root := fix589Layout(t)
	ts := fix589Server(t, root)

	var node treeNode
	getJSON(t, ts.URL+"/api/tree", &node)
	if !node.Dir || node.Path != "" {
		t.Fatalf("root node = %+v, want dir with empty path", node)
	}
	names := map[string]bool{}
	for _, c := range node.Children {
		names[c.Name] = true
		if c.Dir && len(c.Children) != 0 {
			t.Errorf("dir %q must be a leaf in the level response (got %d children)", c.Name, len(c.Children))
		}
		if c.Dir && c.Path != c.Name {
			t.Errorf("dir %q path = %q, want %q", c.Name, c.Path, c.Name)
		}
	}
	for _, want := range []string{"sub", "other", "top.txt"} {
		if !names[want] {
			t.Errorf("root level missing %q (got %v)", want, names)
		}
	}
	for _, leaked := range []string{"deep", "a.txt", "b.txt", "c.txt"} {
		if names[leaked] {
			t.Errorf("deeper entry %q leaked into the root level", leaked)
		}
	}
	body, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 100*1024 {
		t.Errorf("level response = %d bytes, want < 100KB", len(body))
	}
}

// TestFIX589TreeLevelPathMatchesFull compares one level response with the same
// directory inside the recursive listing: name, path, type, git status and
// changed-file counts must be identical.
func TestFIX589TreeLevelPathMatchesFull(t *testing.T) {
	root := fix589Layout(t)
	ts := fix589Server(t, root)

	var lvl treeNode
	getJSON(t, ts.URL+"/api/tree?path="+url.QueryEscape("sub"), &lvl)
	if lvl.Name != "sub" || lvl.Path != "sub" || !lvl.Dir {
		t.Fatalf("node = %+v, want dir sub", lvl)
	}
	var full treeNode
	getJSON(t, ts.URL+"/api/tree?depth=full", &full)
	sub := findNode(t, &full, "sub")
	if len(lvl.Children) != len(sub.Children) {
		t.Fatalf("level children = %d, full children = %d", len(lvl.Children), len(sub.Children))
	}
	for i, c := range lvl.Children {
		f := sub.Children[i]
		if c.Name != f.Name || c.Path != f.Path || c.Dir != f.Dir || c.Status != f.Status || c.Changes != f.Changes {
			t.Errorf("child %d = %+v, want %+v", i, c, f)
		}
	}
}

// TestFIX589TreeLevelTraversalEqualsFull walks the API level by level (the way
// the sidebar does) and compares the collected paths with the recursive
// listing: the on-demand traversal must not lose or invent nodes.
func TestFIX589TreeLevelTraversalEqualsFull(t *testing.T) {
	root := fix589Layout(t)
	ts := fix589Server(t, root)

	level := map[string]bool{}
	queue := []string{""}
	for len(queue) > 0 {
		rel := queue[0]
		queue = queue[1:]
		u := ts.URL + "/api/tree"
		if rel != "" {
			u += "?path=" + url.QueryEscape(rel)
		}
		var n treeNode
		getJSON(t, u, &n)
		for _, c := range n.Children {
			if level[c.Path] {
				t.Fatalf("duplicate path %q across levels", c.Path)
			}
			level[c.Path] = true
			if c.Dir {
				queue = append(queue, c.Path)
			}
		}
	}

	var full treeNode
	getJSON(t, ts.URL+"/api/tree?depth=full", &full)
	want := map[string]bool{}
	for _, c := range full.Children {
		collectTreePaths(c, "", want)
	}
	if len(level) != len(want) {
		t.Fatalf("level traversal = %d paths, full listing = %d paths", len(level), len(want))
	}
	for p := range want {
		if !level[p] {
			t.Errorf("path %q missing from the level traversal", p)
		}
	}
}

// TestFIX589TreePathSafety rejects absolute paths, traversal and paths outside
// the workspace, and reports missing entries without leaking server paths.
func TestFIX589TreePathSafety(t *testing.T) {
	root := fix589Layout(t)
	ts := fix589Server(t, root)

	bad := []string{"../", "sub/../..", "/etc", "sub/deep/../../..", "nope", "top.txt", "sub//deep"}
	for _, p := range bad {
		resp, err := http.Get(ts.URL + "/api/tree?path=" + url.QueryEscape(p))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Errorf("path %q → %d, want 400 or 404", p, resp.StatusCode)
		}
	}

	// An empty path is the workspace root and must still work.
	var ok treeNode
	getJSON(t, ts.URL+"/api/tree?path=", &ok)
	if !ok.Dir || ok.Path != "" {
		t.Errorf("empty path node = %+v, want the workspace root", ok)
	}
}

// TestFIX589TreeExcludedPathNotReachable verifies the level API cannot be used
// to read a folder the tree intentionally hides (FEATURE-553).
func TestFIX589TreeExcludedPathNotReachable(t *testing.T) {
	root := fix589Layout(t)
	nameSet := map[string]bool{"other": true}
	s := NewServer(root, ServerOptions{
		Lang: "zh", Version: "0.0.0", Build: "0",
		TreeExcludes: func() (map[string]bool, map[string]bool) { return nameSet, nil },
	})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)

	var node treeNode
	getJSON(t, ts.URL+"/api/tree", &node)
	for _, c := range node.Children {
		if c.Name == "other" {
			t.Errorf("excluded dir leaked into the root level")
		}
	}
	for _, u := range []string{"/api/tree?path=other", "/api/tree?path=other&depth=full"} {
		resp, err := http.Get(ts.URL + u)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s → %d, want 404", u, resp.StatusCode)
		}
	}
}

// TestFIX589TreeChangesMatchFull checks the git badges of the level API: file
// status letters and the recursive changed-file count of directories match the
// recursive listing (and the numbers the pre-FIX-589 tree reported).
func TestFIX589TreeChangesMatchFull(t *testing.T) {
	if testing.Short() || !gitAvailable(t) {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"sub/x.txt", "sub/y.txt"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	gitInit(t, root)
	// Modify hello.txt (M) and sub/x.txt (M); add untracked sub/z.txt (U).
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "x.txt"), []byte("x2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "z.txt"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	ts := fix589Server(t, root)

	var lvl treeNode
	getJSON(t, ts.URL+"/api/tree", &lvl)
	if lvl.Changes != 3 {
		t.Errorf("root changes = %d, want 3", lvl.Changes)
	}
	if got := findNode(t, &lvl, "hello.txt").Status; got != "M" {
		t.Errorf("hello.txt status = %q, want M", got)
	}
	if got := findNode(t, &lvl, "sub").Changes; got != 2 {
		t.Errorf("sub changes = %d, want 2", got)
	}

	var subLvl treeNode
	getJSON(t, ts.URL+"/api/tree?path=sub", &subLvl)
	if subLvl.Changes != 2 {
		t.Errorf("sub level changes = %d, want 2", subLvl.Changes)
	}
	if got := findNode(t, &subLvl, "z.txt").Status; got != "U" {
		t.Errorf("z.txt status = %q, want U", got)
	}
	if got := findNode(t, &subLvl, "y.txt").Status; got != "" {
		t.Errorf("y.txt status = %q, want empty", got)
	}

	var full treeNode
	getJSON(t, ts.URL+"/api/tree?depth=full", &full)
	if full.Changes != lvl.Changes {
		t.Errorf("full root changes = %d, level = %d", full.Changes, lvl.Changes)
	}
	if got := findNode(t, &full, "sub").Changes; got != subLvl.Changes {
		t.Errorf("full sub changes = %d, level = %d", got, subLvl.Changes)
	}
}
