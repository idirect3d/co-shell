package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
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
	Name() string
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

	mu      sync.RWMutex
	agents  map[string]agentLink
	order   []string // stable agent id order
	current map[*Conn]string // per-client current agent id
}

// NewProxy creates a Proxy and connects to every configured agent. Agents that
// fail to connect are skipped (logged) so a partial outage does not block the
// gateway; they can be retried later.
func NewProxy(ctx context.Context, cfgs []AgentConfig) *Proxy {
	p := &Proxy{
		ctx:     ctx,
		agents:  make(map[string]agentLink),
		current: make(map[*Conn]string),
	}
	for _, cfg := range cfgs {
		ac, err := NewAgentConn(ctx, cfg)
		if err != nil {
			log.Printf("gateway: connect agent %s (%s) failed: %v", cfg.ID, cfg.WSURL, err)
			continue
		}
		p.agents[cfg.ID] = ac
		p.order = append(p.order, cfg.ID)
		log.Printf("gateway: connected agent %s (%s)", cfg.ID, cfg.WSURL)
	}
	return p
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
	p.mu.Lock()
	p.agents[cfg.ID] = ac
	p.order = append(p.order, cfg.ID)
	p.mu.Unlock()
	log.Printf("gateway: connected agent %s (%s)", cfg.ID, cfg.WSURL)
	return nil
}

// RemoveAgent disconnects and unregisters an agent.
func (p *Proxy) RemoveAgent(id string) {
	p.mu.Lock()
	ac, ok := p.agents[id]
	if ok {
		ac.Close()
		delete(p.agents, id)
	}
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
	default:
		p.forwardToCurrent(c, env)
	}
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
		ac := p.agents[id]
		name := id
		if ac != nil {
			name = ac.Name()
		}
		agents = append(agents, map[string]string{"id": id, "name": name})
	}
	cur := p.current[c]
	p.mu.RUnlock()
	payload, _ := json.Marshal(map[string]interface{}{"agents": agents, "current": cur})
	_ = c.Send(&Envelope{Type: MsgTypeListAgents, Payload: payload})
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

// Close tears down all agent connections.
func (p *Proxy) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ac := range p.agents {
		ac.Close()
	}
	p.agents = map[string]agentLink{}
	p.order = nil
	p.current = map[*Conn]string{}
}
