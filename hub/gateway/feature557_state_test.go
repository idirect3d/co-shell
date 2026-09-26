package gateway

import (
	"path/filepath"
	"testing"
)

// TestBoardThreeStateTransitions pins FEATURE-557 A-001..A-003, A-005..A-009 and
// A-013: a roster entry carries three states driven by events — register
// (online), unregister (not registered, entry kept) and link loss (offline) —
// and an unregistered entry ignores every online/offline transition so it can
// never degrade back into "offline".
func TestBoardThreeStateTransitions(t *testing.T) {
	b, _ := newTestBoard(t, "a1")

	// A-001: a registration marks the entry registered and online.
	if a, _ := b.Agent("a1"); !a.Registered || !a.Online {
		t.Fatalf("after Register: registered=%v online=%v, want true/true", a.Registered, a.Online)
	}

	// A-002: the connect hook re-announces on every link; that must not churn
	// the revision its peers have cached.
	rev := b.RosterRev()
	b.Register("a1", "a1", "duty of a1")
	if got := b.RosterRev(); got != rev {
		t.Errorf("re-register bumped the roster revision %d -> %d", rev, got)
	}

	// A-003/A-005: unregister flips the state but keeps the entry, and a second
	// unregister is a no-op.
	if _, changed := b.SetUnregistered("a1"); !changed {
		t.Fatal("SetUnregistered reported no change for a registered agent")
	}
	a, ok := b.Agent("a1")
	if !ok {
		t.Fatal("SetUnregistered dropped the roster entry, want it kept")
	}
	if a.Registered || a.Online {
		t.Errorf("after unregister: registered=%v online=%v, want false/false", a.Registered, a.Online)
	}
	if _, changed := b.SetUnregistered("a1"); changed {
		t.Error("SetUnregistered on an already unregistered agent reported a change")
	}

	// A-006/A-007/A-008: an unregistered entry must not become "offline" on a
	// disconnect, nor light up again when its link returns or a frame arrives.
	if _, changed := b.SetOffline("a1"); changed {
		t.Error("SetOffline flipped an unregistered entry to offline")
	}
	if _, changed := b.MarkOnline("a1"); changed {
		t.Error("MarkOnline lit up an unregistered entry")
	}
	b.mu.Lock()
	flipped := b.touchLocked("a1")
	b.mu.Unlock()
	if flipped {
		t.Error("a board frame lit up an unregistered entry")
	}
	if a, _ := b.Agent("a1"); a.Registered || a.Online {
		t.Errorf("unregistered entry changed state: registered=%v online=%v", a.Registered, a.Online)
	}

	// A-009: the board switch came back on -> registered and online again.
	b.Register("a1", "a1", "duty of a1")
	if a, _ := b.Agent("a1"); !a.Registered || !a.Online {
		t.Errorf("after re-register: registered=%v online=%v, want true/true", a.Registered, a.Online)
	}

	// A-013: the stop-agent path (handleAgentGone -> SetOffline) takes a
	// registered agent offline without losing its registration.
	if _, changed := b.SetOffline("a1"); !changed {
		t.Error("SetOffline on a registered online agent reported no change")
	}
	if a, _ := b.Agent("a1"); !a.Registered || a.Online {
		t.Errorf("after stop: registered=%v online=%v, want true/false", a.Registered, a.Online)
	}
}

// TestListAgentsBoardState pins FEATURE-557 A-012: /api/agents reports the
// three-state indicator, while board_enabled stays as the "has an icon" flag for
// older UIs.
func TestListAgentsBoardState(t *testing.T) {
	w, b := newWebUIForAgents(t)
	b.Register("board-on", "board-on", "duty")
	// board-off never registers: it stays unregistered.
	agents := listAgents(t, w)
	if got := agents["board-on"].BoardState; got != BoardStateOnline {
		t.Errorf("board-on board_state = %q, want %q", got, BoardStateOnline)
	}
	if got := agents["board-on"].BoardEnabled; !got {
		t.Errorf("board-on board_enabled = %v, want true", got)
	}
	if got := agents["board-off"].BoardState; got != BoardStateUnregistered {
		t.Errorf("board-off board_state = %q, want %q", got, BoardStateUnregistered)
	}
	if got := agents["board-off"].BoardEnabled; got {
		t.Errorf("board-off board_enabled = %v, want false", got)
	}
	// Unregistering keeps the entry on the roster but drops the icon.
	if _, changed := b.SetUnregistered("board-on"); !changed {
		t.Fatal("SetUnregistered reported no change")
	}
	if got := listAgents(t, w)["board-on"].BoardState; got != BoardStateUnregistered {
		t.Errorf("after unregister board_state = %q, want %q", got, BoardStateUnregistered)
	}
	// A stopped/lost link keeps the registration: grey (offline) icon.
	b.Register("board-on", "board-on", "duty")
	b.SetOffline("board-on")
	agents = listAgents(t, w)
	if got := agents["board-on"].BoardState; got != BoardStateOffline {
		t.Errorf("after link loss board_state = %q, want %q", got, BoardStateOffline)
	}
	if got := agents["board-on"].BoardEnabled; !got {
		t.Errorf("offline entry board_enabled = %v, want true", got)
	}
}

// TestUnregisterKeepsConversations pins FEATURE-557 A-004: unregistering only
// flips the state — the roster entry, its conversations and its messages stay
// readable, because the agent must remain reachable from the hub UI.
func TestUnregisterKeepsConversations(t *testing.T) {
	b, _ := newTestBoard(t, "a1", "a2")
	if _, _, err := b.Send(SendRequest{From: "a1", To: []string{"a2"}, Content: "hi", TimeoutSecs: 60}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	before := len(b.Conversations())
	if before == 0 {
		t.Fatal("no conversation recorded")
	}

	b.SetUnregistered("a1")

	if _, ok := b.Agent("a1"); !ok {
		t.Error("roster entry gone after unregister")
	}
	if got := len(b.Conversations()); got != before {
		t.Errorf("conversations = %d, want %d after unregister", got, before)
	}
	msgs := b.Messages("a1+a2", 0, 50)
	if len(msgs) != 1 || msgs[0].Content != "hi" {
		t.Errorf("messages after unregister = %+v, want the message kept", msgs)
	}
}

// TestNewBoardRegisteredState pins FEATURE-557 A-010/A-011: a restart restores
// every entry offline, keeps the registered flag, and reads an entry persisted
// before FEATURE-557 (no "registered" field) as "not registered" — the
// conservative reading, since such an agent re-announces itself on reconnect.
func TestNewBoardRegisteredState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub-board.db")
	store, err := OpenBoardStore(path)
	if err != nil {
		t.Fatalf("OpenBoardStore: %v", err)
	}
	b, err := NewBoard(store)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	b.Register("new-format", "N", "duty n")
	// Rewrite one entry the way the previous release stored it: online, with no
	// registered field at all.
	if err := store.SaveAgent(AgentInfo{ID: "legacy", Name: "L", Duty: "duty l", Online: true}); err != nil {
		t.Fatalf("SaveAgent: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store2, err := OpenBoardStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		if cerr := store2.Close(); cerr != nil {
			t.Errorf("close store: %v", cerr)
		}
	}()
	b2, err := NewBoard(store2)
	if err != nil {
		t.Fatalf("NewBoard(restored): %v", err)
	}
	n, ok := b2.Agent("new-format")
	if !ok || !n.Registered || n.Online {
		t.Errorf("new-format after restart: ok=%v registered=%v online=%v, want true/true/false", ok, n.Registered, n.Online)
	}
	l, ok := b2.Agent("legacy")
	if !ok || l.Registered || l.Online {
		t.Errorf("legacy after restart: ok=%v registered=%v online=%v, want true/false/false", ok, l.Registered, l.Online)
	}
}
