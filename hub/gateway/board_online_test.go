// Author: L.Shuang
// Created: 2026-09-17
//
// FEATURE-530 regression tests: the roster must converge when an agent's link is
// dropped and re-established. A serve-mode agent accepts a single WS client, so
// another client (hub Web UI relay, browser) replaces the board link; the board
// used to stay frozen ("each side only sees itself online").
package gateway

import (
	"strings"
	"sync"
	"testing"
)

// onlineTestLink is a stub agent connection that records what was pushed to it.
type onlineTestLink struct {
	id     string
	mu     sync.Mutex
	frames []string
}

func (l *onlineTestLink) ID() string          { return l.id }
func (l *onlineTestLink) Name() string        { return l.id }
func (l *onlineTestLink) Subscribe(c *Conn)   {}
func (l *onlineTestLink) Unsubscribe(c *Conn) {}
func (l *onlineTestLink) Close()              {}

func (l *onlineTestLink) Send(data []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.frames = append(l.frames, string(data))
	return nil
}

func (l *onlineTestLink) frameCount(msgType string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, f := range l.frames {
		if strings.Contains(f, `"`+msgType+`"`) {
			n++
		}
	}
	return n
}

// TestBoardRosterConvergesAfterLinkLoss reproduces the reported defect: agent a
// goes offline when its link is replaced, then reconnects. The roster must flip
// it back online with a higher revision and the fresh roster must be pushed down
// the new link, otherwise both agents keep seeing each other offline.
func TestBoardRosterConvergesAfterLinkLoss(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("new board: %v", err)
	}
	a := &onlineTestLink{id: "a"}
	peer := &onlineTestLink{id: "b"}
	links := map[string]*onlineTestLink{"a": a, "b": peer}
	b.SetAgentLink(func(id string) agentLink {
		if l, ok := links[id]; ok {
			return l
		}
		return nil
	})

	_, revRegistered := b.Register("a", "alpha", "")
	_, revPeer := b.Register("b", "beta", "")

	// The agent's socket is taken over by another client: the hub loses the link
	// and marks the agent offline.
	revOffline, changed := b.SetOffline("a")
	if !changed {
		t.Fatal("SetOffline must report the online -> offline flip")
	}
	if revOffline <= revRegistered || revOffline <= revPeer {
		t.Fatalf("offline rev %d must exceed %d/%d", revOffline, revRegistered, revPeer)
	}
	if info, _ := b.Agent("a"); info.Online {
		t.Fatal("agent must be offline after the link was dropped")
	}

	// Reconnect: the link is installed again and the agent is reachable, so the
	// roster must flip online, raise the revision and be broadcast.
	revOnline, changed := b.MarkOnline("a")
	if !changed {
		t.Fatal("MarkOnline must report the offline -> online flip")
	}
	if revOnline <= revOffline {
		t.Fatalf("online rev %d must exceed offline rev %d", revOnline, revOffline)
	}
	if info, _ := b.Agent("a"); !info.Online {
		t.Fatal("agent must be online again after the link was re-established")
	}
	b.BroadcastRoster()
	if got := a.frameCount(MsgTypeBoardRoster); got == 0 {
		t.Error("the fresh roster must be pushed to the reconnected agent")
	}
	if got := peer.frameCount(MsgTypeBoardRoster); got == 0 {
		t.Error("the fresh roster must be pushed to every other agent")
	}

	// Re-installing a link without a state change must not churn the revision.
	revAgain, changed := b.MarkOnline("a")
	if changed {
		t.Error("MarkOnline must not report a change when the agent is already online")
	}
	if revAgain != revOnline {
		t.Fatalf("revision churned on a no-op reconnect: %d -> %d", revOnline, revAgain)
	}
}

// TestMarkOnlineIgnoresUnknownAgent guards the reconnect path against inventing
// roster entries: name and duty only arrive with the registration frame.
func TestMarkOnlineIgnoresUnknownAgent(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("new board: %v", err)
	}
	rev, changed := b.MarkOnline("ghost")
	if changed {
		t.Error("MarkOnline must ignore an agent that never registered")
	}
	if rev != b.RosterRev() || rev != 0 {
		t.Fatalf("unknown agent must not bump the revision, got %d", rev)
	}
	if _, ok := b.Agent("ghost"); ok {
		t.Error("MarkOnline must not create a roster entry")
	}
}

// TestRegisterDoesNotChurnRevision verifies that a repeated registration (the
// connect hook plus the periodic heartbeat) keeps the revision stable, so the
// roster text injected into every agent does not change for nothing.
func TestRegisterDoesNotChurnRevision(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("new board: %v", err)
	}
	_, first := b.Register("a", "alpha", "duty-1")
	_, second := b.Register("a", "alpha", "duty-1")
	if second != first {
		t.Fatalf("identical re-register changed rev %d -> %d", first, second)
	}
	_, third := b.Register("a", "alpha", "duty-2")
	if third <= second {
		t.Fatalf("changed duty must bump the revision, got %d", third)
	}
	// An offline agent coming back is a real change.
	if _, changed := b.SetOffline("a"); !changed {
		t.Fatal("SetOffline must report the flip")
	}
	_, revBack := b.Register("a", "alpha", "duty-2")
	if info, _ := b.Agent("a"); !info.Online {
		t.Fatal("re-registering an offline agent must bring it back online")
	}
	if revBack <= third {
		t.Fatalf("re-register rev %d must exceed %d", revBack, third)
	}
}

// TestSendTouchesAgentBackOnline verifies that any board traffic from an agent
// flips it online with a broadcastable revision: a sender must never stay
// invisible in the roster while it is talking to the hub.
func TestSendTouchesAgentBackOnline(t *testing.T) {
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("new board: %v", err)
	}
	b.SetAgentLink(func(id string) agentLink { return nil })
	b.Register("a", "alpha", "")
	b.Register("b", "beta", "")
	revOffline, _ := b.SetOffline("a")

	if _, _, err := b.Send(SendRequest{
		From:        "a",
		To:          []string{"b"},
		Content:     "ping",
		TimeoutSecs: 10,
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	info, ok := b.Agent("a")
	if !ok {
		t.Fatal("agent a must stay in the roster")
	}
	if !info.Online {
		t.Error("a board frame from an agent must flip it online")
	}
	if b.RosterRev() <= revOffline {
		t.Errorf("the online flip must raise the revision (was %d, now %d)", revOffline, b.RosterRev())
	}
}
