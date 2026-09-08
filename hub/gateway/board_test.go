package gateway

import (
	"encoding/json"
	"testing"
)

// fakeAgentLink is a stub agentLink that records pushed messages.
type fakeAgentLink struct {
	id   string
	sent [][]byte
}

func (f *fakeAgentLink) ID() string { return f.id }
func (f *fakeAgentLink) Name() string { return f.id }
func (f *fakeAgentLink) Subscribe(c *Conn)   {}
func (f *fakeAgentLink) Unsubscribe(c *Conn) {}
func (f *fakeAgentLink) Close()              {}
func (f *fakeAgentLink) Send(data []byte) error {
	f.sent = append(f.sent, data)
	return nil
}

// newTestBoard builds a Board with a fake agent-link resolver for the given
// agent ids.
func newTestBoard(ids ...string) (*Board, map[string]*fakeAgentLink) {
	b := NewBoard()
	links := map[string]*fakeAgentLink{}
	for _, id := range ids {
		links[id] = &fakeAgentLink{id: id}
	}
	b.SetAgentLink(func(id string) agentLink {
		return links[id]
	})
	for _, id := range ids {
		b.RegisterRole(AgentRole{ID: id, Name: id, Role: "role-" + id})
	}
	return b, links
}

// TestBoardFullFlow verifies the complete collaboration state machine:
// post -> claim -> dm -> confirm -> result.
func TestBoardFullFlow(t *testing.T) {
	b, links := newTestBoard("agent-a", "agent-b")

	// Post a request from agent-a.
	req := b.Post("agent-a", "help me", "please review my Go code", "role-agent-b")
	if req.Status != BoardStatusOpen {
		t.Fatalf("post status = %s, want open", req.Status)
	}
	// Broadcast notify should reach agent-b (not the requester).
	if len(links["agent-b"].sent) != 1 {
		t.Fatalf("agent-b notify count = %d, want 1", len(links["agent-b"].sent))
	}
	if len(links["agent-a"].sent) != 0 {
		t.Fatalf("requester should not be notified, got %d", len(links["agent-a"].sent))
	}

	// Claim by agent-b.
	if err := b.Claim(req.ID, "agent-b"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// Requester should be notified of the claim.
	if len(links["agent-a"].sent) != 1 {
		t.Fatalf("requester claim notify count = %d, want 1", len(links["agent-a"].sent))
	}

	// DM from agent-b to agent-a.
	if err := b.DM(req.ID, "agent-b", "can you share the repo?"); err != nil {
		t.Fatalf("dm: %v", err)
	}
	if len(links["agent-a"].sent) != 2 {
		t.Fatalf("requester dm notify count = %d, want 2", len(links["agent-a"].sent))
	}

	// Confirm by the requester -> creates a task pushed to the assignee.
	task, err := b.Confirm(req.ID, "agent-a")
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if task.Assignee != "agent-b" {
		t.Fatalf("task assignee = %s, want agent-b", task.Assignee)
	}
	// The assignee should have received the board_task push.
	if len(links["agent-b"].sent) != 2 {
		t.Fatalf("assignee task push count = %d, want 2", len(links["agent-b"].sent))
	}
	// Verify the pushed message is a board_task.
	var pushed map[string]interface{}
	if err := json.Unmarshal(links["agent-b"].sent[1], &pushed); err != nil {
		t.Fatalf("unmarshal pushed: %v", err)
	}
	if pushed["type"] != "board_task" {
		t.Fatalf("pushed type = %v, want board_task", pushed["type"])
	}

	// Result from the assignee.
	if err := b.Result(task.ID, "agent-b", "done, here is the review"); err != nil {
		t.Fatalf("result: %v", err)
	}
	// Requester should be notified of the result.
	if len(links["agent-a"].sent) != 3 {
		t.Fatalf("requester result notify count = %d, want 3", len(links["agent-a"].sent))
	}
	// Request status should be done.
	if got := b.List(false); len(got) != 1 || got[0].Status != BoardStatusDone {
		t.Fatalf("request status after result = %v, want done", got)
	}
}

// TestBoardRoleRegistry verifies role registration and listing.
func TestBoardRoleRegistry(t *testing.T) {
	b := NewBoard()
	b.RegisterRole(AgentRole{ID: "a", Name: "Agent A", Role: "backend"})
	b.RegisterRole(AgentRole{ID: "b", Name: "Agent B", Role: "docs"})

	roles := b.Roles()
	if len(roles) != 2 {
		t.Fatalf("roles count = %d, want 2", len(roles))
	}
	found := map[string]bool{}
	for _, r := range roles {
		found[r.ID] = true
	}
	if !found["a"] || !found["b"] {
		t.Fatalf("roles missing expected ids: %v", found)
	}
}

// TestBoardClaimErrors verifies claim rejects non-open requests.
func TestBoardClaimErrors(t *testing.T) {
	b, _ := newTestBoard("agent-a", "agent-b")
	req := b.Post("agent-a", "t", "d", "")

	// Claim twice: second should fail.
	if err := b.Claim(req.ID, "agent-b"); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := b.Claim(req.ID, "agent-b"); err == nil {
		t.Fatalf("second claim should fail (not open)")
	}
	// Claim unknown request.
	if err := b.Claim("req-999", "agent-b"); err == nil {
		t.Fatalf("claim unknown request should fail")
	}
}

// TestBoardConfirmRequesterOnly verifies only the requester can confirm.
func TestBoardConfirmRequesterOnly(t *testing.T) {
	b, _ := newTestBoard("agent-a", "agent-b")
	req := b.Post("agent-a", "t", "d", "")
	if err := b.Claim(req.ID, "agent-b"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// A non-requester cannot confirm.
	if _, err := b.Confirm(req.ID, "agent-b"); err == nil {
		t.Fatalf("non-requester confirm should fail")
	}
	// The requester can.
	if _, err := b.Confirm(req.ID, "agent-a"); err != nil {
		t.Fatalf("requester confirm: %v", err)
	}
}

// TestBoardDMOnlyParticipants verifies only participants can DM.
func TestBoardDMOnlyParticipants(t *testing.T) {
	b, _ := newTestBoard("agent-a", "agent-b", "agent-c")
	req := b.Post("agent-a", "t", "d", "")
	if err := b.Claim(req.ID, "agent-b"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// agent-c is not a participant.
	if err := b.DM(req.ID, "agent-c", "hi"); err == nil {
		t.Fatalf("non-participant DM should fail")
	}
	// Participants can DM.
	if err := b.DM(req.ID, "agent-a", "hi"); err != nil {
		t.Fatalf("participant DM: %v", err)
	}
}

// TestBoardListOpenOnly verifies List(openOnly=true) filters closed requests.
func TestBoardListOpenOnly(t *testing.T) {
	b, _ := newTestBoard("agent-a", "agent-b")
	req := b.Post("agent-a", "t", "d", "")
	if err := b.Claim(req.ID, "agent-b"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// After claim, the request is no longer open.
	if got := b.List(true); len(got) != 0 {
		t.Fatalf("open-only list = %d, want 0", len(got))
	}
	if got := b.List(false); len(got) != 1 {
		t.Fatalf("full list = %d, want 1", len(got))
	}
}
