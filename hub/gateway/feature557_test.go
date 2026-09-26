package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// listAgents drives GET /api/agents through the handler and returns the agents
// keyed by id (FEATURE-557 test helper).
func listAgents(t *testing.T, w *WebUI) map[string]agentView {
	t.Helper()
	rr := httptest.NewRecorder()
	w.handleListAgents(rr, httptest.NewRequest(http.MethodGet, "/api/agents", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("handleListAgents status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body struct {
		Agents []agentView `json:"agents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /api/agents: %v", err)
	}
	out := make(map[string]agentView, len(body.Agents))
	for _, a := range body.Agents {
		out[a.ID] = a
	}
	return out
}

// newWebUIForAgents builds a WebUI over a manager with two external agents
// (unreachable endpoints: the list handler never connects to them).
func newWebUIForAgents(t *testing.T) (*WebUI, *Board) {
	t.Helper()
	ctx := context.Background()
	mgr, err := NewManager(ctx, filepath.Join(t.TempDir(), "hub-agents.json"), "/bin/echo", 30100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for _, id := range []string{"board-on", "board-off"} {
		if _, err := mgr.AddExternal(id, "ws://127.0.0.1:1/ws"); err != nil {
			t.Fatalf("AddExternal(%s): %v", id, err)
		}
	}
	proxy := NewProxy(ctx, nil)
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	t.Cleanup(func() {
		proxy.Close()
		mgr.StopAll()
		if cerr := b.Close(); cerr != nil {
			t.Errorf("board close: %v", cerr)
		}
	})
	return &WebUI{board: b, manager: mgr, proxy: proxy}, b
}

// TestListAgentsBoardEnabled pins FEATURE-557 A-001..A-004: board_enabled
// mirrors the hub message board roster (registration only happens while the
// agent's board-enabled switch is on), an offline roster entry still counts,
// and a WebUI without a board reports false instead of panicking.
func TestListAgentsBoardEnabled(t *testing.T) {
	w, b := newWebUIForAgents(t)
	if _, rev := b.Register("board-on", "board-on", "duty"); rev == 0 {
		t.Fatal("Register did not bump the roster revision")
	}
	// A roster entry may be offline (the hub restarted, the agent is stopped):
	// the switch is a configuration, so the marker must stay.
	b.mu.Lock()
	b.agents["board-on"].Online = false
	b.mu.Unlock()

	agents := listAgents(t, w)
	if got := agents["board-on"].BoardEnabled; !got {
		t.Errorf("board-on board_enabled = %v, want true", got)
	}
	if got := agents["board-off"].BoardEnabled; got {
		t.Errorf("board-off board_enabled = %v, want false", got)
	}

	// No board attached (e.g. a hub started without one): false, no panic.
	w.board = nil
	if got := listAgents(t, w)["board-on"].BoardEnabled; got {
		t.Errorf("board_enabled without a board = %v, want false", got)
	}
}

// TestHubIndexDrawerFloatsAndMarksBoard pins FEATURE-557 A-005/A-006 on the
// embedded shell: the drawer overlays the iframe at every width (no more
// wide-screen rule shifting #stage aside), and the agent list renders the board
// (chat bubble) marker from board_state — bright while registered and online,
// grey while registered but offline, absent while unregistered (same artwork as
// the co-shell status-bar session icon).
func TestHubIndexDrawerFloatsAndMarksBoard(t *testing.T) {
	if strings.Contains(webIndexHTML, "body.drawer-open #stage") {
		t.Error("wide-screen rule shifting #stage behind the drawer is still present")
	}
	if strings.Contains(webIndexHTML, "@media (min-width:901px)") {
		t.Error("wide-screen media query should be gone: the drawer floats at every width")
	}
	if !strings.Contains(webIndexHTML, "box-shadow:2px 0 12px") {
		t.Error("drawer shadow must apply at every width (it used to be narrow-screen only)")
	}
	if !strings.Contains(webIndexHTML, `id="i-board"`) {
		t.Error("icon sprite is missing the i-board (chat bubble) symbol")
	}
	if !strings.Contains(webIndexHTML, "a.board_state") {
		t.Error("agent list does not read board_state when rendering the row")
	}
	if !strings.Contains(webIndexHTML, `#i-board`) {
		t.Error("the agent row does not reference the i-board symbol")
	}
	// A-014: the three states are distinguishable in the markup — bright while
	// online, grey (.bmark.off) while offline, and no marker when unregistered.
	if !strings.Contains(webIndexHTML, ".agent .bmark.off") {
		t.Error("grey (offline) board marker style is missing")
	}
	if !strings.Contains(webIndexHTML, "已启用公告板·在线") || !strings.Contains(webIndexHTML, "已启用公告板·离线") {
		t.Error("board marker tooltips for the online/offline states are missing")
	}
	if !strings.Contains(webIndexHTML, `a.board_state === 'online' || a.board_state === 'offline'`) {
		t.Error("unregistered agents must not render a board marker")
	}
}
