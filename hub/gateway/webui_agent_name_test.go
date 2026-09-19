package gateway

import (
	"testing"
)

// TestAgentDisplayName pins the FIX-542 semantics: the hub-configured ID is the
// default display name, and a board-reported name is only trusted while the
// agent is currently connected and its name is not blank.
func TestAgentDisplayName(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	defer func() { _ = b.Close() }()
	b.Register("uaad", "co-shell", "duty") // reported a name in a previous run
	b.Register("blank", "   ", "duty")     // reported a whitespace-only name

	w := &WebUI{board: b}

	cases := []struct {
		id        string
		connected bool
		want      string
	}{
		{"uaad", true, "co-shell"},   // online + real name: show the reported name
		{"uaad", false, "uaad"},      // offline: stale name must not leak
		{"blank", true, "blank"},     // whitespace-only name counts as unreported
		{"unknown", true, "unknown"}, // never registered: fall back to ID
	}
	for _, c := range cases {
		if got := w.agentDisplayName(c.id, c.connected); got != c.want {
			t.Errorf("agentDisplayName(%q, %v) = %q, want %q", c.id, c.connected, got, c.want)
		}
	}

	// A nil board must never panic and always resolves to the ID.
	w.board = nil
	if got := w.agentDisplayName("uaad", true); got != "uaad" {
		t.Errorf("agentDisplayName with nil board = %q, want %q", got, "uaad")
	}
}
