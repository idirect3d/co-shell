package gateway

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// fakeAgentLink is a stub agentLink that records pushed messages.
type fakeAgentLink struct {
	id   string
	sent [][]byte
}

func (f *fakeAgentLink) ID() string             { return f.id }
func (f *fakeAgentLink) Name() string           { return f.id }
func (f *fakeAgentLink) Subscribe(c *Conn)      {}
func (f *fakeAgentLink) Unsubscribe(c *Conn)    {}
func (f *fakeAgentLink) Close()                 {}
func (f *fakeAgentLink) Send(data []byte) error {
	f.sent = append(f.sent, data)
	return nil
}

// pushes returns the pushed frames decoded as generic maps.
func (f *fakeAgentLink) pushes(t *testing.T) []map[string]interface{} {
	t.Helper()
	out := make([]map[string]interface{}, 0, len(f.sent))
	for _, raw := range f.sent {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("decode pushed frame: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// newTestBoard builds a Board with a stub link resolver plus registered agents.
func newTestBoard(t *testing.T, ids ...string) (*Board, map[string]*fakeAgentLink) {
	t.Helper()
	b, err := NewBoard(nil)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	links := map[string]*fakeAgentLink{}
	for _, id := range ids {
		links[id] = &fakeAgentLink{id: id}
	}
	b.SetAgentLink(func(id string) agentLink { return links[id] })
	for _, id := range ids {
		b.Register(id, id, "duty of "+id)
	}
	return b, links
}

// TestBoardRegisterAndDutyPriority verifies registration and that the hub-side
// duty entry wins over the agent-reported one (decision 9).
func TestBoardRegisterAndDutyPriority(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a")
	b.Register("agent-b", "B", "agent reported duty")

	info, ok := b.Agent("agent-b")
	if !ok {
		t.Fatal("agent-b not registered")
	}
	if info.Duty != "agent reported duty" || info.DutyFrom != DutySourceAgent {
		t.Fatalf("duty=%q source=%q, want agent reported/agent", info.Duty, info.DutyFrom)
	}

	b.SetDutyResolver(func(id string) (string, bool) {
		if id == "agent-b" {
			return "hub duty", true
		}
		return "", false
	})
	info, _ = b.Register("agent-b", "B", "agent reported duty")
	if info.Duty != "hub duty" || info.DutyFrom != DutySourceHub {
		t.Fatalf("duty=%q source=%q, want hub duty/hub", info.Duty, info.DutyFrom)
	}
	if len(b.Roster()) != 2 {
		t.Fatalf("roster size = %d, want 2", len(b.Roster()))
	}
	if _, changed := b.SetOffline("agent-b"); !changed {
		t.Fatal("SetOffline should report a change")
	}
	if info, _ := b.Agent("agent-b"); info.Online {
		t.Fatal("agent-b should be offline")
	}
}

// TestBoardBroadcastExcludesSender verifies decision 3 and the broadcast
// conversation key.
func TestBoardBroadcastExcludesSender(t *testing.T) {
	b, links := newTestBoard(t, "agent-a", "agent-b", "agent-c")

	msg, deliveries, err := b.Send(SendRequest{
		From: "agent-a", To: []string{BoardBroadcastTarget}, Content: "help",
		TimeoutSecs: 60,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.Conv != BoardBroadcastTarget {
		t.Fatalf("conversation = %q, want %q", msg.Conv, BoardBroadcastTarget)
	}
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2 (sender excluded)", len(deliveries))
	}
	for _, d := range deliveries {
		if !d.OK {
			t.Fatalf("delivery to %s failed: %s", d.To, d.Error)
		}
	}
	if len(links["agent-a"].sent) != 0 {
		t.Fatal("sender must not receive its own broadcast")
	}
	if got := links["agent-b"].pushes(t); got[0]["type"] != MsgTypeBoardMessage {
		t.Fatalf("downlink type = %v, want %s", got[0]["type"], MsgTypeBoardMessage)
	}
}

// TestBoardDirectConversationAndReply verifies the direct conversation key and
// that a reply inherits the conversation and clears the awaiting entry.
func TestBoardDirectConversationAndReply(t *testing.T) {
	b, links := newTestBoard(t, "agent-a", "agent-b")

	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{"agent-b"}, Content: "please review",
		TimeoutSecs: 300,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.Conv != "agent-a+agent-b" {
		t.Fatalf("conversation = %q, want agent-a+agent-b", msg.Conv)
	}
	if pending := b.Pending(msg.ID); len(pending) != 1 || pending[0].Status != AwaitingPending {
		t.Fatalf("pending = %+v, want one pending entry", pending)
	}

	reply, _, err := b.Send(SendRequest{
		From: "agent-b", To: []string{"agent-a"}, Content: "on it",
		ReplyTo: msg.ID, State: BoardStateHandled, TimeoutSecs: 300,
	})
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if reply.Conv != "agent-a+agent-b" {
		t.Fatalf("reply conversation = %q, want inherited agent-a+agent-b", reply.Conv)
	}
	if pending := b.Pending(msg.ID); len(pending) != 1 || pending[0].Status != AwaitingAnswered {
		t.Fatalf("pending = %+v, want answered", pending)
	}
	if len(links["agent-a"].sent) == 0 {
		t.Fatal("the reply must be delivered back to the sender")
	}
}

// TestBoardSendValidation covers the parameter rules (timeout required and
// bounded, state only on replies, unknown reply target).
func TestBoardSendValidation(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a", "agent-b")

	cases := []struct {
		name string
		req  SendRequest
	}{
		{"empty content", SendRequest{From: "agent-a", To: []string{"agent-b"}, TimeoutSecs: 60}},
		{"timeout missing", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x"}},
		{"timeout too small", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 4}},
		{"timeout too large", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 86401}},
		{"state on new post", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 60, State: BoardStateHandled}},
		{"missing state on reply", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 60, ReplyTo: "m-1"}},
		{"bad state", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 60, ReplyTo: "m-1", State: "whatever"}},
		{"unknown reply target", SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "x", TimeoutSecs: 60, ReplyTo: "m-99", State: BoardStateHandled}},
		{"no sender", SendRequest{To: []string{"agent-b"}, Content: "x", TimeoutSecs: 60}},
	}
	for _, tc := range cases {
		if _, _, err := b.Send(tc.req); err == nil {
			t.Fatalf("%s: expected an error", tc.name)
		}
	}
}

// TestBoardUnknownRecipient reports a delivery failure instead of dropping the
// message silently.
func TestBoardUnknownRecipient(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a")
	_, deliveries, err := b.Send(SendRequest{
		From: "agent-a", To: []string{"agent-x"}, Content: "hello", TimeoutSecs: 60,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].OK || deliveries[0].Error != "unknown agent" {
		t.Fatalf("deliveries = %+v, want one failure (unknown agent)", deliveries)
	}
}

// TestBoardNoReplySkipsAwaiting verifies that a no-reply message never creates
// a reply obligation (so it can never time out).
func TestBoardNoReplySkipsAwaiting(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a", "agent-b")
	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{"agent-b"}, Content: "fyi",
		NoReply: true, TimeoutSecs: 60,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if pending := b.Pending(msg.ID); len(pending) != 0 {
		t.Fatalf("pending = %+v, want none for no_reply", pending)
	}
	if events := b.SweepTimeouts(); len(events) != 0 {
		t.Fatalf("timeouts = %+v, want none", events)
	}
}

// TestBoardTimeoutSweep verifies that an unanswered message produces exactly
// one timeout event once its window elapses.
func TestBoardTimeoutSweep(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a", "agent-b", "agent-c")
	now := time.Unix(1_700_000_000, 0)
	b.now = func() time.Time { return now }

	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{BoardBroadcastTarget}, Content: "help",
		TimeoutSecs: 30,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	// One recipient answers in time.
	if _, _, err := b.Send(SendRequest{
		From: "agent-b", To: []string{"agent-a"}, Content: "mine",
		ReplyTo: msg.ID, State: BoardStateHandled, TimeoutSecs: 60,
	}); err != nil {
		t.Fatalf("reply: %v", err)
	}

	if events := b.SweepTimeouts(); len(events) != 0 {
		t.Fatalf("sweep before deadline should be empty, got %+v", events)
	}
	now = now.Add(31 * time.Second)
	events := b.SweepTimeouts()
	if len(events) != 1 {
		t.Fatalf("events = %+v, want exactly one", events)
	}
	if len(events[0].Agents) != 1 || events[0].Agents[0] != "agent-c" {
		t.Fatalf("timed-out agents = %v, want [agent-c]", events[0].Agents)
	}
	if again := b.SweepTimeouts(); len(again) != 0 {
		t.Fatalf("second sweep = %+v, want none", again)
	}
}

// TestBoardConversationsAndMessages covers the hub Web UI listing: the
// broadcast conversation always comes first.
func TestBoardConversationsAndMessages(t *testing.T) {
	b, _ := newTestBoard(t, "agent-a", "agent-b")
	if _, _, err := b.Send(SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "dm", TimeoutSecs: 60}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, _, err := b.Send(SendRequest{From: "agent-a", To: []string{BoardBroadcastTarget}, Content: "all", TimeoutSecs: 60}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	convs := b.Conversations()
	if len(convs) != 2 || convs[0].Key != BoardBroadcastTarget {
		t.Fatalf("conversations = %+v, want broadcast first", convs)
	}
	msgs := b.Messages("agent-a+agent-b", 0, 50)
	if len(msgs) != 1 || msgs[0].Content != "dm" {
		t.Fatalf("messages = %+v, want the dm", msgs)
	}
}

// TestBoardPersistenceRoundTrip verifies bbolt persistence: roster, messages,
// conversations and the sequence counter survive a restart.
func TestBoardPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub-board.db")
	store, err := OpenBoardStore(path)
	if err != nil {
		t.Fatalf("OpenBoardStore: %v", err)
	}
	b, err := NewBoard(store)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	b.Register("agent-a", "A", "duty a")
	b.Register("agent-b", "B", "duty b")
	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{BoardBroadcastTarget}, Content: "persist me",
		TimeoutSecs: 60,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	rev := b.RosterRev()
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store2, err := OpenBoardStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		if err := store2.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()
	b2, err := NewBoard(store2)
	if err != nil {
		t.Fatalf("NewBoard(restored): %v", err)
	}
	if got := b2.RosterRev(); got != rev {
		t.Fatalf("roster rev = %d, want %d", got, rev)
	}
	if len(b2.Roster()) != 2 {
		t.Fatalf("roster = %+v, want 2 agents", b2.Roster())
	}
	for _, a := range b2.Roster() {
		if a.Online {
			t.Fatalf("agent %s should be offline after restart", a.ID)
		}
	}
	msgs := b2.Messages(BoardBroadcastTarget, 0, 50)
	if len(msgs) != 1 || msgs[0].ID != msg.ID || msgs[0].Content != "persist me" {
		t.Fatalf("messages after restart = %+v", msgs)
	}
	// The sequence counter must continue, not restart.
	next, _, err := b2.Send(SendRequest{From: "agent-a", To: []string{"agent-b"}, Content: "after restart", TimeoutSecs: 60})
	if err != nil {
		t.Fatalf("Send after restart: %v", err)
	}
	if next.Seq <= msg.Seq {
		t.Fatalf("seq = %d, want > %d", next.Seq, msg.Seq)
	}
}
