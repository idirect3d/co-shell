package gateway

import (
	"strings"
	"testing"
)

// FEATURE-609: the create-agent form refreshes the port field with the latest
// recommended free port on every entry, while a value the user typed themselves
// is preserved.
func TestFeature609FrontendPortRefresh(t *testing.T) {
	page := webIndexHTML
	for _, want := range []string{
		"var lastSuggestedPort = '';",
		"curPort === lastSuggestedPort",
		"lastSuggestedPort = String(j.recommended_port);",
		"lastSuggestedPort='';",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("static page must contain %q", want)
		}
	}
}

// FEATURE-609: the recommended port starts from the manager's base port (the
// Web UI port + 1) and skips ports already used by a managed agent.
func TestFeature609RecommendedPortSkipsRegistered(t *testing.T) {
	m := newTestManager(t) // base port 28500
	got := m.RecommendedPort()
	if got < 28500 {
		t.Fatalf("RecommendedPort = %d, want >= base port 28500", got)
	}
	if portInUse(got) {
		t.Fatalf("RecommendedPort = %d, want a free port", got)
	}
	if _, err := m.CreateManaged("a1", t.TempDir(), "", "", false, true, got, "", false, false); err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	if next := m.RecommendedPort(); next <= got {
		t.Fatalf("RecommendedPort = %d, want > %d once %d is registered", next, got, got)
	}
}
