// Package gateway - Agent bulletin board (FEATURE-490).
//
// The board lets co-shell agents connected to the hub collaborate: one agent
// posts a help request, other agents whose role matches claim it, the two
// sides may clarify via direct messages, and the assignee executes the task
// and returns the result. The hub is the natural rendezvous point because it
// already holds a WebSocket connection to every agent.
//
// Message flow (all over the existing WS gateway):
//
//	post_request  (requester -> hub)  create Request(open), broadcast notify
//	claim         (responder -> hub)  Request -> claimed, notify requester
//	dm            (either -> hub)     relay direct message to the other side
//	confirm       (requester -> hub)  create Task, push board_task to assignee
//	result        (assignee -> hub)   Task -> done, notify requester
//
// The hub only arbitrates state; it never interprets task content.
package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Board message types carried in the Envelope.Type field (in addition to the
// existing control types switch_agent / list_agents).
const (
	// MsgTypeBoardPost publishes a help request.
	MsgTypeBoardPost = "board_post"
	// MsgTypeBoardList lists open requests (polling / inspection).
	MsgTypeBoardList = "board_list"
	// MsgTypeBoardClaim claims an open request.
	MsgTypeBoardClaim = "board_claim"
	// MsgTypeBoardDM relays a direct message between requester and assignee.
	MsgTypeBoardDM = "board_dm"
	// MsgTypeBoardConfirm confirms a claimed request and starts execution.
	MsgTypeBoardConfirm = "board_confirm"
	// MsgTypeBoardResult reports a task result from the assignee.
	MsgTypeBoardResult = "board_result"
)

// Board request status values.
const (
	BoardStatusOpen        = "open"
	BoardStatusClaimed     = "claimed"
	BoardStatusNegotiating = "negotiating"
	BoardStatusExecuting   = "executing"
	BoardStatusDone        = "done"
	BoardStatusFailed      = "failed"
	BoardStatusCancelled   = "cancelled"
)

// BoardRequest is one help request posted to the board.
type BoardRequest struct {
	ID           string    `json:"id"`
	RequesterID  string    `json:"requester_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	RequiredRole string    `json:"required_role,omitempty"`
	Status       string    `json:"status"`
	AssigneeID   string    `json:"assignee_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BoardDM is one direct message in a request thread.
type BoardDM struct {
	RequestID string    `json:"request_id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Time      time.Time `json:"time"`
}

// BoardTask is the execution unit created when a request is confirmed.
type BoardTask struct {
	ID         string    `json:"id"`
	RequestID  string    `json:"request_id"`
	Requester  string    `json:"requester"`
	Assignee   string    `json:"assignee"`
	Instruction string   `json:"instruction"`
	Status     string    `json:"status"`
	Result     string    `json:"result,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AgentRole is the role declaration of one connected agent.
type AgentRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Board is the hub-side bulletin board state machine. It is safe for
// concurrent use. It does not own the agent connections; the Proxy injects
// them via SetAgentLink so the board can push messages down to agents.
type Board struct {
	mu       sync.RWMutex
	requests map[string]*BoardRequest
	dms      map[string][]BoardDM // request_id -> thread
	tasks    map[string]*BoardTask
	roles    map[string]AgentRole // agent id -> role
	seq      int

	// agentLink returns the connection for an agent id (nil if absent).
	// Set by the Proxy so the board can push board_* messages to agents.
	agentLink func(id string) agentLink
}

// NewBoard creates an empty board.
func NewBoard() *Board {
	return &Board{
		requests: make(map[string]*BoardRequest),
		dms:      make(map[string][]BoardDM),
		tasks:    make(map[string]*BoardTask),
		roles:    make(map[string]AgentRole),
	}
}

// SetAgentLink installs the agent-connection resolver used to push messages
// down to agents. Called by the Proxy after construction.
func (b *Board) SetAgentLink(fn func(id string) agentLink) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.agentLink = fn
}

// nextID returns a monotonically increasing request/task id.
func (b *Board) nextID(prefix string) string {
	b.seq++
	return fmt.Sprintf("%s-%d", prefix, b.seq)
}

// RegisterRole records an agent's role declaration (id/name/role).
func (b *Board) RegisterRole(r AgentRole) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roles[r.ID] = r
}

// Roles returns a copy of the role registry.
func (b *Board) Roles() []AgentRole {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]AgentRole, 0, len(b.roles))
	for _, r := range b.roles {
		out = append(out, r)
	}
	return out
}

// Post creates a new open request and broadcasts a board_notify to every
// connected agent (except the requester). Returns the created request.
func (b *Board) Post(requesterID, title, description, requiredRole string) *BoardRequest {
	b.mu.Lock()
	req := &BoardRequest{
		ID:           b.nextID("req"),
		RequesterID:  requesterID,
		Title:        title,
		Description:  description,
		RequiredRole: requiredRole,
		Status:       BoardStatusOpen,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	b.requests[req.ID] = req
	b.mu.Unlock()

	b.broadcastNotify(requesterID, "board_request", req)
	return req
}

// List returns all requests (optionally only open ones).
func (b *Board) List(openOnly bool) []*BoardRequest {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]*BoardRequest, 0, len(b.requests))
	for _, r := range b.requests {
		if openOnly && r.Status != BoardStatusOpen {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Claim marks an open request as claimed by the given agent. Returns an error
// if the request is not open or already claimed.
func (b *Board) Claim(requestID, agentID string) error {
	b.mu.Lock()
	req, ok := b.requests[requestID]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("request %q not found", requestID)
	}
	if req.Status != BoardStatusOpen {
		b.mu.Unlock()
		return fmt.Errorf("request %q is not open (status=%s)", requestID, req.Status)
	}
	req.Status = BoardStatusClaimed
	req.AssigneeID = agentID
	req.UpdatedAt = time.Now()
	b.mu.Unlock()

	b.notifyAgent(req.RequesterID, "board_claimed", map[string]interface{}{
		"request_id": requestID,
		"assignee":   agentID,
	})
	return nil
}

// DM relays a direct message from one participant to the other in a request
// thread. The message is stored and pushed to the recipient.
func (b *Board) DM(requestID, from, content string) error {
	b.mu.Lock()
	req, ok := b.requests[requestID]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("request %q not found", requestID)
	}
	// Only the requester and the assignee may participate.
	if from != req.RequesterID && from != req.AssigneeID {
		b.mu.Unlock()
		return fmt.Errorf("agent %q is not a participant of request %q", from, requestID)
	}
	to := req.RequesterID
	if from == req.RequesterID {
		to = req.AssigneeID
	}
	dm := BoardDM{RequestID: requestID, From: from, To: to, Content: content, Time: time.Now()}
	b.dms[requestID] = append(b.dms[requestID], dm)
	b.mu.Unlock()

	b.notifyAgent(to, "board_dm", map[string]interface{}{
		"request_id": requestID,
		"from":       from,
		"content":    content,
	})
	return nil
}

// Confirm transitions a claimed request into execution: it creates a Task
// assigned to the assignee and pushes board_task down to that agent.
func (b *Board) Confirm(requestID, requesterID string) (*BoardTask, error) {
	b.mu.Lock()
	req, ok := b.requests[requestID]
	if !ok {
		b.mu.Unlock()
		return nil, fmt.Errorf("request %q not found", requestID)
	}
	if req.RequesterID != requesterID {
		b.mu.Unlock()
		return nil, fmt.Errorf("only the requester can confirm request %q", requestID)
	}
	if req.Status != BoardStatusClaimed && req.Status != BoardStatusNegotiating {
		b.mu.Unlock()
		return nil, fmt.Errorf("request %q is not claimable (status=%s)", requestID, req.Status)
	}
	req.Status = BoardStatusExecuting
	req.UpdatedAt = time.Now()
	task := &BoardTask{
		ID:          b.nextID("task"),
		RequestID:   requestID,
		Requester:   req.RequesterID,
		Assignee:    req.AssigneeID,
		Instruction: req.Description,
		Status:      BoardStatusExecuting,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	b.tasks[task.ID] = task
	b.mu.Unlock()

	b.pushTask(task)
	return task, nil
}

// Result records a task result from the assignee and notifies the requester.
func (b *Board) Result(taskID, assigneeID, result string) error {
	b.mu.Lock()
	task, ok := b.tasks[taskID]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}
	if task.Assignee != assigneeID {
		b.mu.Unlock()
		return fmt.Errorf("agent %q is not the assignee of task %q", assigneeID, taskID)
	}
	task.Status = BoardStatusDone
	task.Result = result
	task.UpdatedAt = time.Now()
	if req, ok := b.requests[task.RequestID]; ok {
		req.Status = BoardStatusDone
		req.UpdatedAt = time.Now()
	}
	b.mu.Unlock()

	b.notifyAgent(task.Requester, "board_result", map[string]interface{}{
		"task_id":    taskID,
		"request_id": task.RequestID,
		"assignee":   task.Assignee,
		"result":     result,
	})
	return nil
}

// broadcastNotify pushes a board_* notification to every connected agent
// except the excluded one.
func (b *Board) broadcastNotify(excludeID, kind string, payload interface{}) {
	b.mu.RLock()
	link := b.agentLink
	roles := make([]AgentRole, 0, len(b.roles))
	for _, r := range b.roles {
		roles = append(roles, r)
	}
	b.mu.RUnlock()
	if link == nil {
		return
	}
	for _, r := range roles {
		if r.ID == excludeID {
			continue
		}
		if ac := link(r.ID); ac != nil {
			msg := map[string]interface{}{
				"type":    "dynamic_event",
				"kind":    kind,
				"value":   payload,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := ac.Send(data); err != nil {
				log.Printf("board: notify %s failed: %v", r.ID, err)
			}
		}
	}
}

// notifyAgent pushes a board_* notification to a single agent.
func (b *Board) notifyAgent(agentID, kind string, payload interface{}) {
	b.mu.RLock()
	link := b.agentLink
	b.mu.RUnlock()
	if link == nil {
		return
	}
	if ac := link(agentID); ac != nil {
		msg := map[string]interface{}{
			"type":  "dynamic_event",
			"kind":  kind,
			"value": payload,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return
		}
		if err := ac.Send(data); err != nil {
			log.Printf("board: notify %s failed: %v", agentID, err)
		}
	}
}

// pushTask pushes a board_task execution message to the assignee agent.
func (b *Board) pushTask(task *BoardTask) {
	b.mu.RLock()
	link := b.agentLink
	b.mu.RUnlock()
	if link == nil {
		return
	}
	if ac := link(task.Assignee); ac != nil {
		msg := map[string]interface{}{
			"type":        "board_task",
			"task_id":     task.ID,
			"request_id":  task.RequestID,
			"requester":   task.Requester,
			"instruction": task.Instruction,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return
		}
		if err := ac.Send(data); err != nil {
			log.Printf("board: push task %s to %s failed: %v", task.ID, task.Assignee, err)
		}
	}
}
