package web

import (
	"os"
	"strings"
	"testing"
)

// FIX-617: gitStatusMap must run `git --no-optional-locks status ...`, so the
// file-tree refresh never refreshes/writes the index and therefore never takes
// .git/index.lock. Without it every /api/tree request briefly held the lock
// (measured ~7ms, ~45% of the command's runtime) and concurrent git writes
// (add/commit/merge/tag, from a tool call or the user) died with
// "Unable to create '.git/index.lock': File exists".
func TestFix617GitStatusNoOptionalLocks(t *testing.T) {
	data, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	src := string(data)

	// A-001: the tree refresh opts out of the optional index lock.
	if !strings.Contains(src, `exec.CommandContext(ctx, "git", "--no-optional-locks", "status", "--porcelain", "-z")`) {
		t.Error("gitStatusMap does not pass --no-optional-locks to git status")
	}
	// The locking form (the one that refreshes and writes the index) must be gone.
	if strings.Contains(src, `exec.CommandContext(ctx, "git", "status", "--porcelain", "-z")`) {
		t.Error("the index-locking `git status` invocation is still present")
	}

	// A-002: the other git calls are read-only and never take the index lock;
	// they must stay untouched (surgical change).
	for _, want := range []string{
		`exec.Command("git", "diff", "--", filepath.ToSlash(rel))`,
		`exec.Command("git", "log", "-1", "--format=%H", "--", filepath.ToSlash(rel))`,
		`exec.Command("git", "rev-parse", "--verify", parent)`,
		`exec.Command("git", "diff", parent, c, "--", filepath.ToSlash(rel))`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("read-only git call changed or missing: %q", want)
		}
	}
}
