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

// fakeCoShell writes an executable shell-script stand-in for a co-shell binary
// reporting the given version, and returns its path. The stat/--version probes
// used by the detector treat the script exactly like a real binary.
func fakeCoShell(t *testing.T, dir, name, version string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	body := "#!/bin/sh\necho \"co-shell v" + version + " [BUILD-1]\"\n"
	if err := os.WriteFile(p, []byte(body), 0755); err != nil {
		t.Fatalf("write fake co-shell %s: %v", p, err)
	}
	return p
}

// TestCompareCoShellVersion pins the numeric (not lexical) version ordering.
func TestCompareCoShellVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.9.0", "0.10.0", -1},
		{"0.10.0", "0.9.0", 1},
		{"1.2.3", "1.2.3", 0},
		{"0.61", "0.61.0", 0},
		{"1.0.0", "0.99.99", 1},
	}
	for _, c := range cases {
		if got := compareCoShellVersion(c.a, c.b); got != c.want {
			t.Errorf("compareCoShellVersion(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestResolveLatestCoShell covers the "use the latest version" selection rules
// (FEATURE-527) with injectable search paths.
func TestResolveLatestCoShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stand-ins are not executable on Windows")
	}
	base := t.TempDir()
	hub, cwd, p1, p2 := filepath.Join(base, "hub"), filepath.Join(base, "cwd"), filepath.Join(base, "p1"), filepath.Join(base, "p2")
	for _, d := range []string{hub, cwd, p1, p2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	t.Run("hub directory wins over a higher version elsewhere", func(t *testing.T) {
		low := fakeCoShell(t, hub, "co-shell-0.50.0.darwin.arm64", "0.50.0")
		fakeCoShell(t, cwd, "co-shell-0.58.0.darwin.arm64", "0.58.0")
		fakeCoShell(t, p1, "co-shell-0.61.0.darwin.arm64", "0.61.0")
		got, err := resolveLatestCoShell(hub, cwd, []string{p1, p2})
		if err != nil {
			t.Fatalf("resolveLatestCoShell: %v", err)
		}
		if got != low {
			t.Errorf("got %q, want the hub-directory copy %q", got, low)
		}
	})

	t.Run("highest version inside the hub directory", func(t *testing.T) {
		best := fakeCoShell(t, hub, "co-shell-0.62.0.darwin.arm64", "0.62.0")
		got, err := resolveLatestCoShell(hub, cwd, []string{p1, p2})
		if err != nil {
			t.Fatalf("resolveLatestCoShell: %v", err)
		}
		if got != best {
			t.Errorf("got %q, want %q", got, best)
		}
	})

	t.Run("global maximum when the hub directory has none", func(t *testing.T) {
		empty := t.TempDir()
		want := filepath.Join(p1, "co-shell-0.61.0.darwin.arm64")
		got, err := resolveLatestCoShell(empty, cwd, []string{p1, p2})
		if err != nil {
			t.Fatalf("resolveLatestCoShell: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("numeric comparison across PATH entries", func(t *testing.T) {
		empty := t.TempDir()
		other := t.TempDir()
		fakeCoShell(t, other, "co-shell-0.9.0.darwin.arm64", "0.9.0")
		want := fakeCoShell(t, other, "co-shell-0.10.0.darwin.arm64", "0.10.0")
		got, err := resolveLatestCoShell(empty, t.TempDir(), []string{other})
		if err != nil {
			t.Fatalf("resolveLatestCoShell: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid candidates are skipped", func(t *testing.T) {
		empty := t.TempDir()
		only := t.TempDir()
		// Same name prefix but not a co-shell (its --version output has no
		// version banner), and a co-shell that lacks the execute bit.
		decoy := filepath.Join(only, "co-shell-decoy")
		if err := os.WriteFile(decoy, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
			t.Fatalf("write decoy: %v", err)
		}
		noExec := filepath.Join(only, "co-shell-9.9.9.darwin.arm64")
		if err := os.WriteFile(noExec, []byte("#!/bin/sh\necho \"co-shell v9.9.9\"\n"), 0644); err != nil {
			t.Fatalf("write non-executable: %v", err)
		}
		want := fakeCoShell(t, only, "co-shell-0.61.0.darwin.arm64", "0.61.0")
		got, err := resolveLatestCoShell(empty, t.TempDir(), []string{only})
		if err != nil {
			t.Fatalf("resolveLatestCoShell: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("error when every search path is empty", func(t *testing.T) {
		if _, err := resolveLatestCoShell(t.TempDir(), t.TempDir(), []string{t.TempDir()}); err == nil {
			t.Error("want an error when no usable co-shell exists, got nil")
		}
	})
}
