package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Control message types understood by the proxy (in addition to the raw
// business messages that are forwarded transparently to the current agent).
const (
	// MsgTypeSwitchAgent switches the client's current agent.
	MsgTypeSwitchAgent = "switch_agent"
	// MsgTypeListAgents returns the list of connected agents.
	MsgTypeListAgents = "list_agents"
)

// switchAgentRequest is the payload of MsgTypeSwitchAgent.
type switchAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// agentLink abstracts a connection to one co-shell agent so the proxy can be
// unit-tested with stubs.
type agentLink interface {
	ID() string
	Subscribe(c *Conn)
	Unsubscribe(c *Conn)
	Send(data []byte) error
	Close()
}

// Proxy implements gateway.Handler: it owns the connections to all configured
// co-shell agents and routes each client's business messages to its current
// agent, forwarding agent events back to the client. It does not interpret
// co-shell business logic — it only proxies.
type Proxy struct {
	ctx context.Context

	// sweepOnce guards the single board timeout sweeper goroutine.
	sweepOnce sync.Once

	mu      sync.RWMutex
	agents  map[string]agentLink
	order   []string         // stable agent id order
	current map[*Conn]string // per-client current agent id

	// removed marks agents whose supervision was stopped on purpose
	// (RemoveAgent); a supervisor must not re-dial them.
	removed map[string]bool

	// board is the agent message board (FEATURE-530). It is nil when the
	// proxy is created without one (e.g. in tests), in which case board_*
	// messages are forwarded transparently like any other business message.
	board *Board
}

// agentRetryMin / agentRetryMax bound the re-dial backoff of one agent link.
const (
	agentRetryMin = 2 * time.Second
	agentRetryMax = 30 * time.Second
)

// NewProxy creates a Proxy and starts one supervisor per configured agent.
// A supervisor keeps a live connection to its agent and re-dials when the link
// drops: a serve-mode agent accepts a single WS client, so any other client
// (hub Web UI relay, browser) replaces our socket. Without the re-dial the
// board link stayed dead and the agent was frozen offline forever (FEATURE-530).
func NewProxy(ctx context.Context, cfgs []AgentConfig) *Proxy {
	p := &Proxy{
		ctx:     ctx,
		agents:  make(map[string]agentLink),
		current: make(map[*Conn]string),
		removed: make(map[string]bool),
	}
	for _, cfg := range cfgs {
		ac, err := NewAgentConn(ctx, cfg)
		if err != nil {
			log.Printf("gateway: connect agent %s (%s) failed: %v", cfg.ID, cfg.WSURL, err)
		}
		go p.superviseAgent(cfg, ac)
	}
	return p
}

// superviseAgent owns one agent link: it installs the given connection (or
// dials a fresh one) and re-dials with backoff after the link drops, until the
// proxy context is cancelled or the agent is removed.
func (p *Proxy) superviseAgent(cfg AgentConfig, first *AgentConn) {
	ac := first
	delay := agentRetryMin
	for {
		if p.ctx.Err() != nil {
			return
		}
		if ac == nil {
			if p.isRemoved(cfg.ID) {
				return
			}
			dialed, err := NewAgentConn(p.ctx, cfg)
			if err != nil {
				log.Printf("gateway: connect agent %s (%s) failed: %v", cfg.ID, cfg.WSURL, err)
				if !p.waitRetry(delay) {
					return
				}
				if delay *= 2; delay > agentRetryMax {
					delay = agentRetryMax
				}
				continue
			}
			ac = dialed
		}
		delay = agentRetryMin
		cur := ac
		cur.SetOnAgentMessage(p.routeAgentMessage)
		cur.SetOnClose(func(id string) { p.dropAgentLink(cur, id) })
		p.installAgentLink(cfg, cur)
		<-cur.Done()
		ac = nil
		if !p.waitRetry(delay) {
			return
		}
	}
}

// waitRetry sleeps before the next dial; it reports false when the proxy is
// shutting down.
func (p *Proxy) waitRetry(d time.Duration) bool {
	select {
	case <-p.ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// installAgentLink registers ac as the agent's link and tells the board the
// agent is reachable again (its link is up before any board frame arrives).
func (p *Proxy) installAgentLink(cfg AgentConfig, ac *AgentConn) {
	p.mu.Lock()
	p.agents[cfg.ID] = ac
	if !containsID(p.order, cfg.ID) {
		p.order = append(p.order, cfg.ID)
	}
	b := p.board
	p.mu.Unlock()
	log.Printf("gateway: connected agent %s (%s)", cfg.ID, cfg.WSURL)
	if b != nil {
		b.MarkOnline(cfg.ID)
		// Push the current roster down the fresh link so a reconnected agent
		// immediately learns who else is online.
		b.BroadcastRoster()
	}
}

// dropAgentLink handles one connection going away. A connection that was
// already replaced by a newer one is ignored: only the current link may take
// an agent offline (a superseded socket used to mark a live agent offline and
// left the board without a working link).
func (p *Proxy) dropAgentLink(ac *AgentConn, agentID string) {
	p.mu.Lock()
	cur, ok := p.agents[agentID]
	if !ok || cur != agentLink(ac) {
		p.mu.Unlock()
		return
	}
	delete(p.agents, agentID)
	p.order = removeID(p.order, agentID)
	p.mu.Unlock()
	p.handleAgentGone(agentID)
}

// containsID reports whether ids holds id.
func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// removeID returns ids without id.
func removeID(ids []string, id string) []string {
	out := ids[:0:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}

// markRemoved records that an agent must not be re-dialed (RemoveAgent).
// The caller must hold p.mu.
func (p *Proxy) markRemoved(id string) {
	if p.removed == nil {
		p.removed = make(map[string]bool)
	}
	p.removed[id] = true
}

// isRemoved reports whether supervision of id was stopped on purpose.
func (p *Proxy) isRemoved(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.removed[id]
}

// boardSweepInterval is how often expired reply obligations are checked.
const boardSweepInterval = 5 * time.Second

// SetBoard attaches the agent message board to the proxy and wires the board's
// agent-connection resolver to the proxy's agent registry (FEATURE-530). The
// board can then push roster and message frames down to agents.
func (p *Proxy) SetBoard(b *Board) {
	p.mu.Lock()
	p.board = b
	if b != nil {
		b.SetAgentLink(func(id string) agentLink {
			p.mu.RLock()
			defer p.mu.RUnlock()
			return p.agents[id]
		})
	}
	p.mu.Unlock()
	if b != nil {
		p.startBoardSweeper()
	}
}

// startBoardSweeper launches the single background goroutine that turns expired
// reply obligations into board_timeout events for the original senders.
func (p *Proxy) startBoardSweeper() {
	p.sweepOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(boardSweepInterval)
			defer ticker.Stop()
			for {
				select {
				case <-p.ctx.Done():
					return
				case <-ticker.C:
					p.sweepBoardTimeouts()
				}
			}
		}()
	})
}

// sweepBoardTimeouts emits one board_timeout frame per message whose reply
// window elapsed without an answer.
func (p *Proxy) sweepBoardTimeouts() {
	p.mu.RLock()
	b := p.board
	p.mu.RUnlock()
	if b == nil {
		return
	}
	for _, ev := range b.SweepTimeouts() {
		b.Notify(ev.From, MsgTypeBoardTimeout, ev)
	}
}

// agentBoardFrame is the flat board frame shape co-shell agents speak: the
// board fields sit directly next to "type". Gateway clients (TCP / browser)
// keep using Envelope instead.
type agentBoardFrame struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Duty        string   `json:"duty"`
	To          []string `json:"to"`
	Content     string   `json:"content"`
	ReplyTo     string   `json:"reply_to"`
	NoReply     bool     `json:"no_reply"`
	State       string   `json:"state"`
	TimeoutSecs int      `json:"timeout_seconds"`
	ReqID       string   `json:"req_id"`
}

// boardSendRequest is a parsed board_send, shared by the agent (flat) path and
// the gateway client (envelope) path.
type boardSendRequest struct {
	To          []string `json:"to"`
	Content     string   `json:"content"`
	ReplyTo     string   `json:"reply_to"`
	NoReply     bool     `json:"no_reply"`
	State       string   `json:"state"`
	TimeoutSecs int      `json:"timeout_seconds"`
	ReqID       string   `json:"req_id"`
}

// routeAgentMessage handles frames an agent sends up to the hub. Board frames
// (registration, sends) are consumed by the Board; every other agent message is
// relayed to subscribed clients by AgentConn.broadcast.
func (p *Proxy) routeAgentMessage(agentID string, msg []byte) {
	p.mu.RLock()
	b := p.board
	p.mu.RUnlock()
	if b == nil {
		return
	}
	var frame agentBoardFrame
	if err := json.Unmarshal(msg, &frame); err != nil {
		return
	}
	switch frame.Type {
	case MsgTypeBoardRegister, MsgTypeBoardIdentity:
		// board_identity carries the same name/duty payload as board_register:
		// co-shell sends it when its IDENTITY.md changed (FEATURE-538), so the
		// roster refreshes without waiting for the 60s heartbeat.
		info, rev := b.Register(agentID, frame.Name, frame.Duty)
		b.Notify(agentID, MsgTypeBoardRegistered, map[string]interface{}{
			"ok":          true,
			"agent_id":    info.ID,
			"name":        info.Name,
			"duty":        info.Duty,
			"duty_source": info.DutyFrom,
			"roster_rev":  rev,
		})
		b.BroadcastRoster()
		log.Printf("board: agent %s reported identity (type=%s, duty source=%s)", agentID, frame.Type, info.DutyFrom)
	case MsgTypeBoardUnregister:
		// The agent turned its board switch off (FEATURE-557). The roster entry
		// is kept — the agent stays reachable and its conversations stay
		// readable — only the state flips to "not registered".
		if _, changed := b.SetUnregistered(agentID); changed {
			b.BroadcastRoster()
			log.Printf("board: agent %s is no longer registered", agentID)
		}
	case MsgTypeBoardSend:
		ack := p.sendBoardMessage(agentID, boardSendRequest{
			To:          frame.To,
			Content:     frame.Content,
			ReplyTo:     frame.ReplyTo,
			NoReply:     frame.NoReply,
			State:       frame.State,
			TimeoutSecs: frame.TimeoutSecs,
			ReqID:       frame.ReqID,
		})
		b.Notify(agentID, MsgTypeBoardSent, ack)
	}
}

// handleAgentGone marks a disconnected agent offline and refreshes the roster.
// The caller (dropAgentLink) has already dropped the link, so a live agent that
// still has a newer connection is never taken offline.
func (p *Proxy) handleAgentGone(agentID string) {
	p.mu.RLock()
	b := p.board
	p.mu.RUnlock()
	if b == nil {
		return
	}
	if _, changed := b.SetOffline(agentID); changed {
		b.BroadcastRoster()
		log.Printf("board: agent %s went offline", agentID)
	}
}

// AgentIDs returns the ids of successfully connected agents.
func (p *Proxy) AgentIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, len(p.order))
	copy(out, p.order)
	return out
}

// AddAgent connects a new agent (by WS URL) and registers it for routing.
// It returns an error if the agent id is already present or the dial fails.
func (p *Proxy) AddAgent(cfg AgentConfig) error {
	p.mu.Lock()
	if _, exists := p.agents[cfg.ID]; exists {
		p.mu.Unlock()
		return fmt.Errorf("agent %q already connected", cfg.ID)
	}
	p.mu.Unlock()

	ac, err := NewAgentConn(p.ctx, cfg)
	if err != nil {
		return err
	}
	// The supervisor owns the link from here on: it installs the connection
	// and re-dials it when a serve-mode takeover drops it (FEATURE-530).
	go p.superviseAgent(cfg, ac)
	return nil
}

// RemoveAgent disconnects and unregisters an agent.
func (p *Proxy) RemoveAgent(id string) {
	p.mu.Lock()
	p.markRemoved(id)
	ac, ok := p.agents[id]
	if ok {
		delete(p.agents, id)
	}
	p.mu.Unlock()
	// Close outside the lock: the close hook takes p.mu again.
	if ok {
		ac.Close()
	}
	p.mu.Lock()
	for i, oid := range p.order {
		if oid == id {
			p.order = append(p.order[:i], p.order[i+1:]...)
			break
		}
	}
	// Re-point any client currently on this agent to the first remaining one.
	for c, cur := range p.current {
		if cur == id {
			if len(p.order) > 0 {
				p.current[c] = p.order[0]
			} else {
				delete(p.current, c)
			}
		}
	}
	p.mu.Unlock()
	p.handleAgentGone(id)
}

// OnConnect subscribes the client to its default (first) agent.
func (p *Proxy) OnConnect(_ context.Context, c *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.order) == 0 {
		return
	}
	def := p.order[0]
	p.current[c] = def
	if ac := p.agents[def]; ac != nil {
		ac.Subscribe(c)
	}
}

// OnDisconnect unsubscribes the client from all agents.
func (p *Proxy) OnDisconnect(_ context.Context, c *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ac := range p.agents {
		ac.Unsubscribe(c)
	}
	delete(p.current, c)
}

// HandleMessage routes a client message. Control messages (switch_agent,
// list_agents) are handled here; everything else is forwarded verbatim to the
// client's current agent.
func (p *Proxy) HandleMessage(ctx context.Context, c *Conn, env *Envelope) {
	switch env.Type {
	case MsgTypeSwitchAgent:
		p.handleSwitchAgent(c, env)
	case MsgTypeListAgents:
		p.handleListAgents(c)
	case MsgTypeBoardSend:
		p.handleBoardClient(c, env)
	case MsgTypeBoardRegister, MsgTypeBoardUnregister:
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"board registration is agent-only"`)})
	default:
		p.forwardToCurrent(c, env)
	}
}

// handleBoardClient handles a board_send sent by a gateway client (browser or
// TCP client). The sender identity is the client's current agent.
func (p *Proxy) handleBoardClient(c *Conn, env *Envelope) {
	p.mu.RLock()
	b := p.board
	agentID := p.current[c]
	p.mu.RUnlock()
	if b == nil {
		p.forwardToCurrent(c, env)
		return
	}
	if agentID == "" {
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"no agent selected"`)})
		return
	}
	ack := p.sendBoardMessage(agentID, parseBoardSend(env))
	payload, err := json.Marshal(ack)
	if err != nil {
		return
	}
	_ = c.Send(&Envelope{Type: MsgTypeBoardSent, Payload: payload})
}

// parseBoardSend decodes a board_send from a gateway client envelope.
func parseBoardSend(env *Envelope) boardSendRequest {
	var req boardSendRequest
	if env != nil {
		_ = json.Unmarshal(env.Payload, &req)
	}
	return req
}

// sendBoardMessage validates and relays one board_send on behalf of the given
// sender, returning the immediate acknowledgement payload.
func (p *Proxy) sendBoardMessage(senderID string, req boardSendRequest) map[string]interface{} {
	ack := map[string]interface{}{"req_id": req.ReqID}
	p.mu.RLock()
	b := p.board
	p.mu.RUnlock()
	if b == nil {
		ack["ok"] = false
		ack["error"] = "board is not attached"
		return ack
	}
	msg, deliveries, err := b.Send(SendRequest{
		From:        senderID,
		To:          req.To,
		Content:     req.Content,
		ReplyTo:     req.ReplyTo,
		NoReply:     req.NoReply,
		State:       req.State,
		TimeoutSecs: req.TimeoutSecs,
	})
	if err != nil {
		ack["ok"] = false
		ack["error"] = err.Error()
		return ack
	}
	ack["ok"] = true
	ack["message_id"] = msg.ID
	ack["conversation"] = msg.Conv
	ack["deliveries"] = deliveries
	ack["timeout_seconds"] = msg.Timeout
	return ack
}

// handleSwitchAgent changes the client's current agent and re-subscribes it.
func (p *Proxy) handleSwitchAgent(c *Conn, env *Envelope) {
	var req switchAgentRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil || req.AgentID == "" {
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"bad switch_agent payload"`)})
		return
	}
	p.mu.Lock()
	ac, ok := p.agents[req.AgentID]
	if !ok {
		p.mu.Unlock()
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"unknown agent"`)})
		return
	}
	// Unsubscribe from old agent, subscribe to new.
	if old := p.current[c]; old != "" && old != req.AgentID {
		if oldAC := p.agents[old]; oldAC != nil {
			oldAC.Unsubscribe(c)
		}
	}
	p.current[c] = req.AgentID
	ac.Subscribe(c)
	p.mu.Unlock()
	_ = c.Send(&Envelope{Type: MsgTypeSwitchAgent, Payload: json.RawMessage(`"ok"`)})
}

// handleListAgents returns the connected agent list to the client.
func (p *Proxy) handleListAgents(c *Conn) {
	p.mu.RLock()
	agents := make([]map[string]string, 0, len(p.order))
	for _, id := range p.order {
		agents = append(agents, map[string]string{"id": id, "name": p.boardNameLocked(id)})
	}
	cur := p.current[c]
	p.mu.RUnlock()
	payload, _ := json.Marshal(map[string]interface{}{"agents": agents, "current": cur})
	_ = c.Send(&Envelope{Type: MsgTypeListAgents, Payload: payload})
}

// boardNameLocked returns the display name of an agent: the name it reported
// on the board (its IDENTITY.md agent-name, FEATURE-538), falling back to the
// ID when it never registered. The caller must hold p.mu.
func (p *Proxy) boardNameLocked(id string) string {
	if p.board != nil {
		if info, ok := p.board.Agent(id); ok && strings.TrimSpace(info.Name) != "" {
			return info.Name
		}
	}
	return id
}

// forwardToCurrent forwards a business message to the client's current agent.
func (p *Proxy) forwardToCurrent(c *Conn, env *Envelope) {
	p.mu.RLock()
	ac := p.agents[p.current[c]]
	p.mu.RUnlock()
	if ac == nil {
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"no agent connected"`)})
		return
	}
	// The business payload is the co-shell clientMessage JSON. Forward it
	// verbatim (transparent proxy, no business interpretation).
	if err := ac.Send(env.Payload); err != nil {
		_ = c.Send(&Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"forward failed"`)})
	}
}

// Close tears down all agent connections. An agent link's close hook
// (dropAgentLink) takes p.mu as well, so the links are detached while holding
// the lock and closed only after it is released: closing them under p.mu
// self-deadlocks and hangs the whole shutdown (FIX-561).
func (p *Proxy) Close() {
	p.mu.Lock()
	links := make([]agentLink, 0, len(p.agents))
	for _, ac := range p.agents {
		links = append(links, ac)
	}
	p.agents = map[string]agentLink{}
	p.order = nil
	p.current = map[*Conn]string{}
	p.mu.Unlock()

	for _, ac := range links {
		ac.Close()
	}
}
