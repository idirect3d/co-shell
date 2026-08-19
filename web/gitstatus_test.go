// Package web - git status badge tests (FEATURE-375): gitStatusMap parsing
// (M/A/D/R/U, non-repo, clean tree) and /api/tree status/changes fields.
//
// Author: L.Shuang
// Created: 2026-08-19
// Last Modified: 2026-08-19
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitAvailable reports whether the git binary can be executed.
func gitAvailable(t *testing.T) bool {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	return true
}

// git runs git in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// gitInit prepares dir as a git repository with an initial commit of all
// existing files.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
}

// TestGitStatusMap verifies the normalized status codes for each change type
// (UC-0001).
func TestGitStatusMap(t *testing.T) {
	if testing.Short() || !gitAvailable(t) {
		t.Skip("git not available")
	}
	root := t.TempDir()
	// Initial tracked files (committed by gitInit).
	for _, f := range []string{"a.txt", "d.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("v1"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	gitInit(t, root)
	// M: modify a tracked file (working tree, unstaged)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	// U: untracked file
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	// A: staged new file
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("staged"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "c.txt")
	// R: staged rename d.txt -> e.txt (stage only these two paths)
	if err := os.Rename(filepath.Join(root, "d.txt"), filepath.Join(root, "e.txt")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A", "d.txt", "e.txt")
	m := gitStatusMap(root)
	want := map[string]string{"a.txt": "M", "b.txt": "U", "c.txt": "A", "e.txt": "R"}
	for p, code := range want {
		if m[p] != code {
			t.Errorf("status[%q] = %q, want %q (map=%v)", p, m[p], code, m)
		}
	}
}

// TestGitStatusMapNonRepo verifies a non-repo workspace yields an empty map
// (UC-0002).
func TestGitStatusMapNonRepo(t *testing.T) {
	root := t.TempDir()
	if m := gitStatusMap(root); len(m) != 0 {
		t.Errorf("non-repo map = %v, want empty", m)
	}
}

// TestGitStatusMapClean verifies a clean repo yields an empty map (UC-0003).
func TestGitStatusMapClean(t *testing.T) {
	if testing.Short() || !gitAvailable(t) {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	gitInit(t, root)
	if m := gitStatusMap(root); len(m) != 0 {
		t.Errorf("clean map = %v, want empty", m)
	}
}

// findNode locates a child by name.
func findNode(t *testing.T, n *treeNode, name string) *treeNode {
	t.Helper()
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("node %q not found", name)
	return nil
}

// TestTreeStatusFields verifies /api/tree carries status on files and
// recursive changes on directories (UC-0004/0005).
func TestTreeStatusFields(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "sub", "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "y.txt"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
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
	s := NewServer(root, ServerOptions{Lang: "zh", Version: "0.0.0", Build: "0"})
	ts := httptest.NewServer(s.mux)
	defer ts.Close()
	var root2 treeNode
	getJSON(t, ts.URL+"/api/tree", &root2)
	if got := findNode(t, &root2, "hello.txt").Status; got != "M" {
		t.Errorf("hello.txt status = %q, want M", got)
	}
	sub := findNode(t, &root2, "sub")
	if sub.Changes != 2 {
		t.Errorf("sub changes = %d, want 2", sub.Changes)
	}
	if root2.Changes != 3 {
		t.Errorf("root changes = %d, want 3", root2.Changes)
	}
	if got := findNode(t, sub, "z.txt").Status; got != "U" {
		t.Errorf("z.txt status = %q, want U", got)
	}
	if got := findNode(t, sub, "y.txt").Status; got != "" {
		t.Errorf("y.txt status = %q, want empty", got)
	}
}

// TestTreeStatusNonRepo verifies the tree is unchanged (no status/changes
// fields) when the workspace is not a git repository (UC-0006).
func TestTreeStatusNonRepo(t *testing.T) {
	_, ts, _ := newTestServer(t)
	var root treeNode
	getJSON(t, ts.URL+"/api/tree", &root)
	for _, c := range root.Children {
		if c.Status != "" || c.Changes != 0 {
			t.Errorf("node %q has status=%q changes=%d, want none", c.Name, c.Status, c.Changes)
		}
	}
}
