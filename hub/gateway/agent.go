package gateway

import (
	"context"
	"encoding/json"
	"log"
	"sync"
)

// AgentConfig describes one co-shell agent that the hub connects to.
type AgentConfig struct {
	// ID is the unique agent identifier used for routing.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// WSURL is the agent's WebSocket endpoint, e.g. "ws://127.0.0.1:8399/ws".
	WSURL string `json:"ws_url"`
}

// AgentConn manages a single WebSocket client connection to one co-shell
// agent. It forwards the agent's server messages (events, sessions, state,
// ask, etc.) to every subscribed client connection.
type AgentConn struct {
	cfg    AgentConfig
	ws     *wsClient
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.RWMutex
	clients map[*Conn]struct{} // subscribed gateway client connections
	closed  bool
}

// NewAgentConn dials the agent's WebSocket endpoint and starts its read loop.
func NewAgentConn(ctx context.Context, cfg AgentConfig) (*AgentConn, error) {
	ws, err := dialWS(cfg.WSURL)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithCancel(ctx)
	ac := &AgentConn{
		cfg:     cfg,
		ws:      ws,
		ctx:     cctx,
		cancel:  cancel,
		clients: make(map[*Conn]struct{}),
	}
	go ac.readLoop()
	return ac, nil
}

// ID returns the agent identifier.
func (ac *AgentConn) ID() string { return ac.cfg.ID }

// Name returns the agent display name.
func (ac *AgentConn) Name() string { return ac.cfg.Name }

// Subscribe registers a gateway client to receive this agent's messages.
func (ac *AgentConn) Subscribe(c *Conn) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.closed {
		return
	}
	ac.clients[c] = struct{}{}
}

// Unsubscribe removes a gateway client.
func (ac *AgentConn) Unsubscribe(c *Conn) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	delete(ac.clients, c)
}

// Send forwards a client message (already JSON-encoded) to the agent.
func (ac *AgentConn) Send(data []byte) error {
	return ac.ws.WriteText(data)
}

// Close tears down the agent connection.
func (ac *AgentConn) Close() {
	ac.cancel()
	ac.mu.Lock()
	ac.closed = true
	ac.mu.Unlock()
	ac.ws.Close()
}

// readLoop reads server messages from the agent and broadcasts them to all
// subscribed gateway clients.
func (ac *AgentConn) readLoop() {
	for {
		select {
		case <-ac.ctx.Done():
			return
		default:
		}
		msg, err := ac.ws.ReadMessage()
		if err != nil {
			if ac.ctx.Err() == nil {
				log.Printf("gateway: agent %s read error: %v", ac.cfg.ID, err)
			}
			ac.Close()
			return
		}
		ac.broadcast(msg)
	}
}

// broadcast sends a raw agent message to every subscribed client, wrapped in
// an envelope tagged with the agent id so the client knows its origin.
func (ac *AgentConn) broadcast(msg []byte) {
	env := &Envelope{
		Type:    "agent_event",
		Payload: json.RawMessage(msg),
	}
	ac.mu.RLock()
	clients := make([]*Conn, 0, len(ac.clients))
	for c := range ac.clients {
		clients = append(clients, c)
	}
	ac.mu.RUnlock()
	for _, c := range clients {
		if err := c.Send(env); err != nil {
			ac.Unsubscribe(c)
		}
	}
}
