// Package main - built-in system directory creation tests (FEATURE-501).

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureBuiltinDirsCreatesAll verifies UC-001: a fresh workspace gets all
// 12 built-in directories, and none is reported as failed.
func TestEnsureBuiltinDirsCreatesAll(t *testing.T) {
	root := t.TempDir()

	if failed := ensureBuiltinDirs(root); len(failed) != 0 {
		t.Fatalf("ensureBuiltinDirs failed for %v, want none", failed)
	}
	if len(builtinDirs) != 12 {
		t.Fatalf("builtinDirs has %d entries, want 12", len(builtinDirs))
	}
	for _, name := range builtinDirs {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Errorf("directory %q not created: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q exists but is not a directory", name)
		}
	}
}

// TestEnsureBuiltinDirsKeepsExistingContent verifies UC-002: existing files
// inside an already-present directory are not touched.
func TestEnsureBuiltinDirsKeepsExistingContent(t *testing.T) {
	root := t.TempDir()
	rulesDir := filepath.Join(root, ".rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rulePath := filepath.Join(rulesDir, "team.md")
	if err := os.WriteFile(rulePath, []byte("team rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	if failed := ensureBuiltinDirs(root); len(failed) != 0 {
		t.Fatalf("ensureBuiltinDirs failed for %v, want none", failed)
	}
	data, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("existing file gone: %v", err)
	}
	if string(data) != "team rule" {
		t.Errorf("existing file content = %q, want %q", data, "team rule")
	}
}

// TestEnsureBuiltinDirsFillsMissing verifies UC-003: only missing directories
// are created; pre-existing ones stay untouched.
func TestEnsureBuiltinDirsFillsMissing(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"input", "output"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if failed := ensureBuiltinDirs(root); len(failed) != 0 {
		t.Fatalf("ensureBuiltinDirs failed for %v, want none", failed)
	}
	for _, name := range builtinDirs {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("directory %q not present after fill: %v", name, err)
		}
	}
}

// TestEnsureBuiltinDirsFileOccupiesPath verifies UC-004: when a plain file
// occupies a target path, the function reports it as failed instead of
// panicking, and still creates the remaining directories.
func TestEnsureBuiltinDirsFileOccupiesPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "research"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	failed := ensureBuiltinDirs(root)
	if len(failed) != 1 || failed[0] != "research" {
		t.Fatalf("failed = %v, want [research]", failed)
	}
	// Other directories must still be created.
	for _, name := range builtinDirs {
		if name == "research" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("directory %q not created: %v", name, err)
		}
	}
}

// TestEnsureBuiltinDirsIdempotent verifies UC-008: repeated calls are safe and
// report no failures.
func TestEnsureBuiltinDirsIdempotent(t *testing.T) {
	root := t.TempDir()
	if failed := ensureBuiltinDirs(root); len(failed) != 0 {
		t.Fatalf("first call failed for %v", failed)
	}
	if failed := ensureBuiltinDirs(root); len(failed) != 0 {
		t.Fatalf("second call failed for %v, want none", failed)
	}
}
