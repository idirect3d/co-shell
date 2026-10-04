package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateAgentID covers the ID rules of FEATURE-538: non-empty, no
// whitespace and none of the path/URL-hostile characters; a valid ID comes back
// trimmed.
func TestValidateAgentID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain", "agent-1", "agent-1", true},
		{"underscore dot", "co_flow.v2", "co_flow.v2", true},
		{"trimmed", "  agent-2  ", "agent-2", true},
		{"empty", "", "", false},
		{"blank", "   ", "", false},
		{"inner space", "agent 1", "", false},
		{"slash", "a/b", "", false},
		{"backslash", "a\\b", "", false},
		{"colon", "a:b", "", false},
		{"star", "a*b", "", false},
		{"question", "a?b", "", false},
		{"quote", "a\"b", "", false},
		{"angle", "a<b", "", false},
		{"pipe", "a|b", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateAgentID(tc.in)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("validateAgentID(%q) = %q, %v; want %q, nil", tc.in, got, err, tc.want)
				}
				return
			}
			if !errors.Is(err, ErrInvalidAgentID) {
				t.Fatalf("validateAgentID(%q) error = %v; want ErrInvalidAgentID", tc.in, err)
			}
		})
	}
}

// TestManagerRejectsDuplicateAndInvalidID covers the create/add-external ID
// rules: duplicates (including case variants) report ErrAgentExists, invalid IDs
// report ErrInvalidAgentID, and nothing is registered on rejection.
func TestManagerRejectsDuplicateAndInvalidID(t *testing.T) {
	m := newTestManager(t)
	ws := filepath.Join(t.TempDir(), "ws")
	if _, err := m.CreateManaged("agent-1", ws, "", "", false, true, 0, "", false, false); err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	if _, err := m.AddExternal("remote-1", "ws://127.0.0.1:29001/ws"); err != nil {
		t.Fatalf("AddExternal: %v", err)
	}

	cases := []struct {
		name string
		in   string
		want error
	}{
		{"managed duplicate", "agent-1", ErrAgentExists},
		{"managed case variant", "Agent-1", ErrAgentExists},
		{"managed invalid char", "bad/id", ErrInvalidAgentID},
		{"managed blank", "  ", ErrInvalidAgentID},
		{"external duplicate", "remote-1", ErrAgentExists},
		{"external case variant", "Remote-1", ErrAgentExists},
		{"external invalid char", "bad id", ErrInvalidAgentID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.CreateManaged(tc.in, ws, "", "", false, true, 0, "", false, false)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CreateManaged(%q) error = %v; want %v", tc.in, err, tc.want)
			}
			_, err = m.AddExternal(tc.in, "ws://127.0.0.1:29002/ws")
			if !errors.Is(err, tc.want) {
				t.Fatalf("AddExternal(%q) error = %v; want %v", tc.in, err, tc.want)
			}
		})
	}
	if got := len(m.Agents()); got != 2 {
		t.Fatalf("rejected IDs must not be registered, agent count = %d, want 2", got)
	}
	if got := len(readRegistry(t, m)); got != 2 {
		t.Fatalf("rejected IDs must not reach disk, count = %d, want 2", got)
	}
}

// TestManagerRegistryIgnoresLegacyNameField proves an old registry file that
// still carries a "name" field loads cleanly: the field is no longer part of
// AgentSpec (FEATURE-538), so json.Unmarshal simply ignores it.
func TestManagerRegistryIgnoresLegacyNameField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")
	legacy := `[{"id":"a1","name":"old remark","type":"managed","workspace":"` +
		filepath.Join(dir, "ws") + `","port":28510}]`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatalf("write legacy registry: %v", err)
	}
	m, err := NewManager(context.Background(), path, "/bin/true", 28500)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	specs := m.Agents()
	if len(specs) != 1 || specs[0].ID != "a1" || specs[0].Workspace == "" {
		t.Fatalf("legacy registry not loaded: %+v", specs)
	}
}

// TestBoardDutyPrefersAgentReport covers the roster duty precedence of
// FEATURE-538: what the agent reports (its IDENTITY.md description) wins, and
// the hub-side resolver is only the fallback used when nothing was reported.
func TestBoardDutyPrefersAgentReport(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	b.SetDutyResolver(func(agentID string) (string, bool) {
		if agentID == "a1" {
			return "from file", true
		}
		return "", false
	})

	info, _ := b.Register("a1", "work", "reported by agent")
	if info.Duty != "reported by agent" || info.DutyFrom != DutySourceAgent {
		t.Fatalf("reported duty must win: duty=%q source=%q", info.Duty, info.DutyFrom)
	}

	fallback, _ := b.Register("a1", "work", "")
	if fallback.Duty != "from file" || fallback.DutyFrom != DutySourceHub {
		t.Fatalf("resolver fallback mismatch: duty=%q source=%q", fallback.Duty, fallback.DutyFrom)
	}

	empty, _ := b.Register("a3", "other", "")
	if empty.Duty != "" || empty.DutyFrom != DutySourceAgent {
		t.Fatalf("unknown agent must keep an empty duty: duty=%q source=%q", empty.Duty, empty.DutyFrom)
	}
}
