package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// newTestManager returns a Manager backed by a temp registry file. The
// co-shell path points at /bin/true because these tests never launch anything.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(context.Background(), filepath.Join(t.TempDir(), "agents.json"), "/bin/true", 28500)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

// readRegistry parses the persisted registry file, proving the update reached
// disk (not just memory).
func readRegistry(t *testing.T, m *Manager) []AgentSpec {
	t.Helper()
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var specs []AgentSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("unmarshal registry: %v", err)
	}
	return specs
}

func findAgent(specs []AgentSpec, id string) (AgentSpec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return AgentSpec{}, false
}

// freePort returns a loopback port that is free at call time.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

func TestManagerUpdateUnknownAgent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Update("missing", AgentPatch{Name: strPtr("x")}); !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}

// TestManagerUpdateManagedFields covers the editable fields of a managed agent
// and proves the change is persisted (FEATURE-520).
func TestManagerUpdateManagedFields(t *testing.T) {
	m := newTestManager(t)
	wsOld := filepath.Join(t.TempDir(), "old")
	spec, err := m.CreateManaged("a1", "a1", wsOld, "", "", false, true, 0, "--accept-license")
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}

	wsNew := filepath.Join(t.TempDir(), "new")
	port := freePort(t)
	updated, err := m.Update("a1", AgentPatch{
		Name:            strPtr("renamed"),
		Workspace:       strPtr(wsNew),
		Port:            intPtr(port),
		CoShell:         strPtr("/usr/local/bin/co-shell"),
		UseSharedConfig: boolPtr(false),
		ExtraArgs:       strPtr("--accept-license --log-level debug"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.ID != spec.ID || updated.Type != AgentTypeManaged {
		t.Fatalf("id/type must be immutable, got id=%q type=%q", updated.ID, updated.Type)
	}
	if updated.Name != "renamed" || updated.Workspace != wsNew || updated.Port != port ||
		updated.CoShell != "/usr/local/bin/co-shell" || updated.UseSharedConfig ||
		updated.ExtraArgs != "--accept-license --log-level debug" {
		t.Fatalf("unexpected updated spec: %+v", updated)
	}
	if _, err := os.Stat(wsNew); err != nil {
		t.Fatalf("workspace not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wsNew, "config.json")); err != nil {
		t.Fatalf("per-agent config.json not created: %v", err)
	}

	persisted, ok := findAgent(readRegistry(t, m), "a1")
	if !ok {
		t.Fatal("agent a1 missing from persisted registry")
	}
	if persisted.Name != "renamed" || persisted.Workspace != wsNew || persisted.Port != port ||
		persisted.ExtraArgs != "--accept-license --log-level debug" {
		t.Fatalf("persisted spec not updated: %+v", persisted)
	}
}

// TestManagerUpdatePartialPatchLeavesOtherFields proves a patch only touches the
// fields it carries.
func TestManagerUpdatePartialPatchLeavesOtherFields(t *testing.T) {
	m := newTestManager(t)
	spec, err := m.CreateManaged("a1", "a1", filepath.Join(t.TempDir(), "ws"), "/opt/co-shell", "", false, true, 0, "--accept-license")
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	updated, err := m.Update("a1", AgentPatch{Name: strPtr("note")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "note" || updated.Port != spec.Port || updated.Workspace != spec.Workspace ||
		updated.CoShell != "/opt/co-shell" || updated.ExtraArgs != "--accept-license" || !updated.UseSharedConfig {
		t.Fatalf("untouched fields changed: %+v", updated)
	}
}

// TestManagerUpdateEmptyNameFallsBackToID matches CreateManaged semantics.
func TestManagerUpdateEmptyNameFallsBackToID(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.CreateManaged("a1", "a1", filepath.Join(t.TempDir(), "ws"), "", "", false, true, 0, ""); err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	updated, err := m.Update("a1", AgentPatch{Name: strPtr("   ")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "a1" {
		t.Fatalf("expected name fallback to id, got %q", updated.Name)
	}
}

// TestManagerUpdateKeepsCurrentPort: re-submitting the port already in use by
// this very agent must succeed (it is used by the running agent itself).
func TestManagerUpdateKeepsCurrentPort(t *testing.T) {
	m := newTestManager(t)
	spec, err := m.CreateManaged("a1", "a1", filepath.Join(t.TempDir(), "ws"), "", "", false, true, 0, "")
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	port := spec.Port
	ln, err := net.Listen("tcp", "127.0.0.1:"+itoa(port))
	if err == nil {
		// The agent's own port is bound: updating to it must still be accepted.
		defer ln.Close()
	}
	if _, err := m.Update("a1", AgentPatch{Port: &port, Name: strPtr("same-port")}); err != nil {
		t.Fatalf("Update with unchanged port: %v", err)
	}
}

func TestManagerUpdateRejectsBusyPort(t *testing.T) {
	m := newTestManager(t)
	spec, err := m.CreateManaged("a1", "a1", filepath.Join(t.TempDir(), "ws"), "", "", false, true, 0, "")
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	if _, err := m.Update("a1", AgentPatch{Port: &busy}); err == nil {
		t.Fatal("expected an error for a port already in use")
	} else if errors.Is(err, ErrInvalidAgent) {
		// A busy port is a conflict (409), not a client-side validation error (400).
		t.Fatalf("busy port must not be reported as ErrInvalidAgent: %v", err)
	}
	// Neither memory nor disk may change on this failure.
	if got, _ := findAgent(m.Agents(), "a1"); got.Port != spec.Port {
		t.Fatalf("in-memory port changed on failure: %d", got.Port)
	}
	if got, _ := findAgent(readRegistry(t, m), "a1"); got.Port != spec.Port {
		t.Fatalf("persisted port changed on failure: %d", got.Port)
	}
}

func TestManagerUpdateRejectsInvalidInput(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.CreateManaged("a1", "a1", filepath.Join(t.TempDir(), "ws"), "", "", false, true, 0, ""); err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	if _, err := m.AddExternal("e1", "e1", "ws://127.0.0.1:29001/ws"); err != nil {
		t.Fatalf("AddExternal: %v", err)
	}

	cases := []struct {
		name  string
		id    string
		patch AgentPatch
	}{
		{"managed: port out of range", "a1", AgentPatch{Port: intPtr(700000)}},
		{"managed: blank workspace", "a1", AgentPatch{Workspace: strPtr("   ")}},
		{"external: blank ws_url", "e1", AgentPatch{WSURL: strPtr("  ")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.Update(tc.id, tc.patch)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !errors.Is(err, ErrInvalidAgent) {
				t.Fatalf("expected ErrInvalidAgent, got %v", err)
			}
		})
	}
}

func TestManagerUpdateExternalWSURL(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.AddExternal("e1", "e1", "ws://127.0.0.1:29001/ws"); err != nil {
		t.Fatalf("AddExternal: %v", err)
	}
	wsURL := "ws://10.0.0.9:29005/ws"
	updated, err := m.Update("e1", AgentPatch{Name: strPtr("remote-a"), WSURL: strPtr(wsURL)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Type != AgentTypeExternal || updated.WSURL != wsURL || updated.Name != "remote-a" {
		t.Fatalf("unexpected external spec: %+v", updated)
	}
	// Managed-only fields must be ignored for an external agent.
	if _, err := m.Update("e1", AgentPatch{Workspace: strPtr("/tmp/ignored"), Port: intPtr(29999)}); err != nil {
		t.Fatalf("Update managed-only fields on external agent: %v", err)
	}
	persisted, _ := findAgent(readRegistry(t, m), "e1")
	if persisted.Workspace != "" || persisted.Port != 0 || persisted.WSURL != wsURL {
		t.Fatalf("external spec polluted with managed fields: %+v", persisted)
	}
}

// itoa avoids importing strconv for a single call in the port test.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [8]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
