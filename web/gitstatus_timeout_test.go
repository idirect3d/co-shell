// Package web - git status timeout tests (FIX-463): gitStatusMap must not
// block forever when the git command hangs. A fake `git` on PATH that sleeps
// is used to simulate a hung git status; gitStatusMap must return nil within
// the 3s timeout instead of stalling the file-tree API.
//
// Author: L.Shuang
// Created: 2026-09-01
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGitStatusMapTimeout verifies gitStatusMap returns nil within the 3s
// timeout when the git command hangs (UC-0002). A fake `git` script that
// sleeps is placed on PATH so the real git is never invoked.
func TestGitStatusMapTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// A fake git that hangs (sleeps) instead of returning.
	bin := t.TempDir()
	fakeGit := filepath.Join(bin, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexec sleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	// A git repo so gitStatusMap proceeds past the .git check.
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+oldPath)

	start := time.Now()
	m := gitStatusMap(root)
	elapsed := time.Since(start)

	if m != nil {
		t.Errorf("gitStatusMap = %v, want nil on timeout", m)
	}
	if elapsed >= 5*time.Second {
		t.Errorf("gitStatusMap took %v, want < 5s (timeout should fire at 3s)", elapsed)
	}
}
