// Package config - regression tests for the config save path (FIX-541).
//
// Author: L.Shuang
// Created: 2026-09-19
// MIT License - Copyright (c) 2026 L.Shuang

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFromFileMissingKeepsEmptyLoadedPath pins the existing semantics that
// loadConfigByPriority (main.go) relies on: a non-existent path must report an
// empty loaded path so the caller keeps searching the remaining candidate
// locations (FEATURE-484). FIX-541 changed only the internal configPath field,
// not this return value.
func TestLoadFromFileMissingKeepsEmptyLoadedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.json")

	cfg, loadedPath, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile on missing file: unexpected error: %v", err)
	}
	if loadedPath != "" {
		t.Fatalf("loaded path = %q, want empty (missing file must not report a load)", loadedPath)
	}
	if cfg == nil {
		t.Fatal("LoadFromFile returned nil config")
	}
	// The requested path must still be remembered so Save() can write back to it.
	if cfg.configPath != path {
		t.Fatalf("cfg.configPath = %q, want %q", cfg.configPath, path)
	}
}

// TestSaveCreatesMissingDirAndFile is the FIX-541 core case: saving a config
// whose target directory and file do not exist must recreate both instead of
// failing with ENOENT or silently writing somewhere else. This is the shared
// config scenario (~/.co-shell/config.json on a fresh machine).
func TestSaveCreatesMissingDirAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "co-shell", "nested", "config.json")

	cfg, _, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save must recreate the missing directory: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file was not created at %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory, want a file", path)
	}

	// Re-loading the same path must now succeed and report it as the source,
	// which is what makes a shared config survive a restart.
	reloaded, loadedPath, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("re-load saved config: %v", err)
	}
	if loadedPath != path {
		t.Fatalf("loaded path = %q, want %q", loadedPath, path)
	}
	if reloaded == nil {
		t.Fatal("reloaded config is nil")
	}
}

// TestSaveOverwritesExistingFile guards the normal case against regressions
// from the directory-creation change.
func TestSaveOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg, loadedPath, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loadedPath != path {
		t.Fatalf("loaded path = %q, want %q", loadedPath, path)
	}

	cfg.LogLevel = "existing-file-overwrite"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	again, _, err := LoadFromFile(path, nil)
	if err != nil {
		t.Fatalf("re-load: %v", err)
	}
	if again.LogLevel != "existing-file-overwrite" {
		t.Fatalf("saved value = %q, want %q", again.LogLevel, "existing-file-overwrite")
	}
}

// TestSaveWithoutPathOrWorkspaceStillErrors keeps the guard for the case where
// there is neither an explicit path nor a workspace to fall back to.
func TestSaveWithoutPathOrWorkspaceStillErrors(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Save(); err == nil {
		t.Fatal("Save without configPath and without workspace must return an error")
	}
}
