package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIdentity writes content to {dir}/IDENTITY.md.
func writeIdentity(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, IdentityFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write identity: %v", err)
	}
}

// readIdentity returns {dir}/IDENTITY.md.
func readIdentity(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, IdentityFileName))
	if err != nil {
		t.Fatalf("read identity: %v", err)
	}
	return string(data)
}

// TestIdentityReadDescription covers reading the agent-description section
// (FEATURE-538): a missing file or section yields "" instead of an error.
func TestIdentityReadDescription(t *testing.T) {
	dir := t.TempDir()
	if got := ReadIdentityDescription(dir); got != "" {
		t.Fatalf("missing file: got %q, want empty", got)
	}
	writeIdentity(t, dir, "## agent-name\nwork\n")
	if got := ReadIdentityDescription(dir); got != "" {
		t.Fatalf("missing section: got %q, want empty", got)
	}
	writeIdentity(t, dir, "## agent-name\nwork\n\n## agent-description\n严谨务实。\n\n## user-name\n匿名\n")
	if got := ReadIdentityDescription(dir); got != "严谨务实。" {
		t.Fatalf("got %q, want the description text", got)
	}
}

// TestIdentityWriteReplacesSection proves a rewrite touches only the
// agent-description body: other sections and free-form text stay verbatim.
func TestIdentityWriteReplacesSection(t *testing.T) {
	dir := t.TempDir()
	writeIdentity(t, dir, "intro line\n\n## agent-name\nwork\n\n## agent-description\nold\n\n## user-name\n匿名\n")
	if err := WriteIdentityDescription(dir, "a1", "new text"); err != nil {
		t.Fatalf("WriteIdentityDescription: %v", err)
	}
	want := "intro line\n\n## agent-name\nwork\n\n## agent-description\nnew text\n\n## user-name\n匿名\n"
	if got := readIdentity(t, dir); got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

// TestIdentityWriteAppendsSection covers a file that has no agent-description
// section yet.
func TestIdentityWriteAppendsSection(t *testing.T) {
	dir := t.TempDir()
	writeIdentity(t, dir, "## agent-name\nwork\n")
	if err := WriteIdentityDescription(dir, "a1", "duty"); err != nil {
		t.Fatalf("WriteIdentityDescription: %v", err)
	}
	got := readIdentity(t, dir)
	if !strings.Contains(got, "## agent-name\nwork\n") {
		t.Fatalf("existing section changed: %q", got)
	}
	if !strings.Contains(got, "## agent-description\nduty") {
		t.Fatalf("section not appended: %q", got)
	}
}

// TestIdentityWriteCreatesFile covers the first write into a workspace: the file
// is created and agent-name is initialised from the hub agent id.
func TestIdentityWriteCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteIdentityDescription(dir, "agent-7", "hello"); err != nil {
		t.Fatalf("WriteIdentityDescription: %v", err)
	}
	got := readIdentity(t, dir)
	if !strings.Contains(got, "## agent-name\nagent-7") {
		t.Fatalf("agent-name not initialised: %q", got)
	}
	if v := ReadIdentityDescription(dir); v != "hello" {
		t.Fatalf("round trip = %q, want hello", v)
	}
}

// TestIdentityWriteEmptyValue keeps the heading while clearing the body.
func TestIdentityWriteEmptyValue(t *testing.T) {
	dir := t.TempDir()
	writeIdentity(t, dir, "## agent-description\nold\n\n## user-name\n匿名\n")
	if err := WriteIdentityDescription(dir, "a1", ""); err != nil {
		t.Fatalf("WriteIdentityDescription: %v", err)
	}
	if got := ReadIdentityDescription(dir); got != "" {
		t.Fatalf("value = %q, want empty", got)
	}
	if !strings.Contains(readIdentity(t, dir), "## agent-description") {
		t.Fatal("heading was dropped")
	}
	if !strings.Contains(readIdentity(t, dir), "## user-name\n匿名") {
		t.Fatal("following section was dropped")
	}
}

// TestIdentityWriteRejectsBadWorkspace: an empty or missing workspace must fail
// instead of creating a file somewhere unexpected.
func TestIdentityWriteRejectsBadWorkspace(t *testing.T) {
	if err := WriteIdentityDescription("", "a1", "x"); err == nil {
		t.Fatal("expected an error for an empty workspace")
	}
	if _, err := readIdentityDescription(""); err == nil {
		t.Fatal("expected an error for an empty workspace")
	}
	missing := filepath.Join(t.TempDir(), "nope")
	if err := WriteIdentityDescription(missing, "a1", "x"); err == nil {
		t.Fatal("expected an error for a missing workspace")
	}
	if _, err := os.Stat(filepath.Join(missing, IdentityFileName)); !os.IsNotExist(err) {
		t.Fatal("no file may be created for a missing workspace")
	}
}
