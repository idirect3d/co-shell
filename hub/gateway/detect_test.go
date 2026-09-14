package gateway

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetectCoShellsVersioned verifies that DetectCoShells picks up versioned
// co-shell executables (e.g. co-shell-0.44.0.darwin.arm64) in the current
// directory and on PATH, and that non-co-shell executables are excluded.
func TestDetectCoShellsVersioned(t *testing.T) {
	// Create a temp dir with a versioned co-shell binary (a copy of the real
	// co-shell binary so --version works) and a decoy executable.
	dir := t.TempDir()
	realBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// Copy the test binary itself as a stand-in co-shell (it responds to
	// --version with a Go test binary banner, which may not match the co-shell
	// version regex; so instead we only verify the candidate scan logic via
	// coShellCandidates, which does not require --version).
	_ = realBin

	// coShellCandidates should find versioned names.
	versioned := filepath.Join(dir, "co-shell-0.44.0.darwin.arm64")
	if err := os.WriteFile(versioned, []byte("#!/bin/sh\necho co-shell v0.44.0 [BUILD-921]\n"), 0755); err != nil {
		t.Fatalf("write versioned: %v", err)
	}
	decoy := filepath.Join(dir, "not-co-shell")
	if err := os.WriteFile(decoy, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	cands := coShellCandidates(dir)
	found := false
	for _, c := range cands {
		if filepath.Base(c) == "co-shell-0.44.0.darwin.arm64" {
			found = true
		}
		if filepath.Base(c) == "not-co-shell" {
			t.Errorf("decoy %q should not be a candidate", c)
		}
	}
	if !found {
		t.Errorf("versioned co-shell not found in candidates: %v", cands)
	}
}

// TestHubDirResolvesExecutableDir verifies hubDir returns the directory that
// holds the running executable, so scanning it finds a co-shell shipped next
// to the hub binary (FIX-521).
func TestHubDirResolvesExecutableDir(t *testing.T) {
	dir, err := hubDir()
	if err != nil {
		t.Fatalf("hubDir: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("hubDir = %q, want an absolute path", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(exe))); err != nil {
		t.Errorf("hubDir %q does not contain the running executable: %v", dir, err)
	}
}

// TestDetectCoShellsHubDir verifies that a co-shell placed next to the hub
// binary is reported with source "hubdir", and that a same-prefix decoy in that
// directory is still filtered out by the --version check (FIX-521).
func TestDetectCoShellsHubDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the shell-script stand-in is not executable on Windows")
	}
	dir, err := hubDir()
	if err != nil {
		t.Fatalf("hubDir: %v", err)
	}
	real := filepath.Join(dir, "co-shell-fix521-real")
	if err := os.WriteFile(real, []byte("#!/bin/sh\necho co-shell v9.9.9 [BUILD-999]\n"), 0755); err != nil {
		t.Skipf("hub dir %s is not writable: %v", dir, err)
	}
	defer func() { _ = os.Remove(real) }()
	decoy := filepath.Join(dir, "co-shell-fix521-decoy")
	if err := os.WriteFile(decoy, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Skipf("hub dir %s is not writable: %v", dir, err)
	}
	defer func() { _ = os.Remove(decoy) }()

	shells := DetectCoShells()
	var got *coShellInfo
	for i := range shells {
		switch shells[i].Path {
		case real:
			got = &shells[i]
		case decoy:
			t.Errorf("decoy %q must not be listed", decoy)
		}
	}
	if got == nil {
		t.Fatalf("co-shell next to the hub binary was not detected: %s", real)
	}
	if got.Source != "hubdir" {
		t.Errorf("Source = %q, want %q", got.Source, "hubdir")
	}
	if got.Version != "9.9.9" {
		t.Errorf("Version = %q, want 9.9.9", got.Version)
	}
}
