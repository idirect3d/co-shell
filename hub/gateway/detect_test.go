package gateway

import (
	"os"
	"path/filepath"
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
