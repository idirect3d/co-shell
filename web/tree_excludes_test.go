// Package web - workspace tree exclusion tests (FEATURE-553): the exclusions
// come from the injected TreeExcludes sets, a NAME hides folders at any depth,
// a relative PATH hides exactly that folder, files are never matched, and an
// empty list hides nothing.
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTreeTestServer builds a Server over a temp workspace containing the given
// directories, with the exclusion sets injected the same way main.go does.
func newTreeTestServer(t *testing.T, names, paths, layout []string) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	for _, dir := range layout {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	pathSet := map[string]bool{}
	for _, p := range paths {
		pathSet[p] = true
	}
	s := NewServer(root, ServerOptions{
		Lang: "zh", Version: "0.0.0", Build: "0",
		TreeExcludes: func() (map[string]bool, map[string]bool) { return nameSet, pathSet },
	})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return ts
}

// collectTreePaths walks a tree node and records every workspace-relative path.
func collectTreePaths(n *treeNode, prefix string, out map[string]bool) {
	rel := n.Name
	if prefix != "" {
		rel = prefix + "/" + n.Name
	}
	out[rel] = true
	for _, c := range n.Children {
		collectTreePaths(c, rel, out)
	}
}

// treePaths returns every path present in /api/tree, keyed by relative path.
func treePaths(t *testing.T, ts *httptest.Server) map[string]bool {
	t.Helper()
	var root treeNode
	// FIX-589: the default response covers one level only; this helper walks the
	// whole tree, so it asks for the recursive listing explicitly.
	getJSON(t, ts.URL+"/api/tree?depth=full", &root)
	out := map[string]bool{}
	for _, c := range root.Children {
		collectTreePaths(c, "", out)
	}
	return out
}

// TestTreeExclusions verifies the FEATURE-553 exclusion semantics.
func TestTreeExclusions(t *testing.T) {
	cases := []struct {
		name    string
		names   []string
		paths   []string
		layout  []string
		hidden  []string
		visible []string
	}{
		{
			name:    "default list hides db and .git only",
			names:   []string{"db", ".git"},
			layout:  []string{"db", ".git", "node_modules", "log", "tmp", "output"},
			hidden:  []string{"db", ".git"},
			visible: []string{"node_modules", "log", "tmp", "output"},
		},
		{
			name:    "name matches folders at any depth",
			names:   []string{"node_modules"},
			layout:  []string{"node_modules", "web/node_modules", "web/other"},
			hidden:  []string{"node_modules", "web/node_modules"},
			visible: []string{"web/other"},
		},
		{
			name:    "relative path matches exactly",
			paths:   []string{"docs/private"},
			layout:  []string{"docs/private", "docs/public", "other/private"},
			hidden:  []string{"docs/private"},
			visible: []string{"docs/public", "other/private"},
		},
		{
			name:    "empty list hides nothing",
			layout:  []string{"db", ".git"},
			visible: []string{"db", ".git"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTreeTestServer(t, tc.names, tc.paths, tc.layout)
			paths := treePaths(t, ts)
			for _, p := range tc.hidden {
				if paths[p] {
					t.Errorf("tree contains excluded path %q", p)
				}
			}
			for _, p := range tc.visible {
				if !paths[p] {
					t.Errorf("tree is missing visible path %q", p)
				}
			}
		})
	}
}

// TestTreeExcludesIgnoreFiles verifies UC-0004: only folders are matched, so a
// file whose name is listed stays visible while a folder of that name
// (here sub/notes.md) is hidden.
func TestTreeExcludesIgnoreFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("note"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub", "notes.md"), 0755); err != nil {
		t.Fatal(err)
	}
	nameSet := map[string]bool{"notes.md": true}
	s := NewServer(root, ServerOptions{
		Lang: "zh", Version: "0.0.0", Build: "0",
		TreeExcludes: func() (map[string]bool, map[string]bool) { return nameSet, nil },
	})
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)

	var tree treeNode
	getJSON(t, ts.URL+"/api/tree?depth=full", &tree)
	dirs := map[string]bool{}
	for _, c := range tree.Children {
		dirs[c.Name] = c.Dir
	}
	if isDir, ok := dirs["notes.md"]; !ok || isDir {
		t.Fatalf("listed file must stay visible as a file, got %v (dir=%v)", ok, isDir)
	}
	for _, c := range tree.Children {
		if c.Name != "sub" {
			continue
		}
		for _, nested := range c.Children {
			if nested.Name == "notes.md" {
				t.Fatalf("folder named notes.md must be hidden")
			}
		}
	}
}
