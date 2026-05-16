// Author: L.Shuang
// Created: 2026-05-17
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package hub implements the co-shell-hub service that manages multiple
// co-shell agent instances and handles UDP communication with mobile clients.
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// AgentConfig holds the configuration for a single co-shell agent instance.
type AgentConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Workspace is the workspace path for this agent.
	Workspace string `json:"workspace,omitempty"`
	// ConfigPath is the path to the co-shell config file.
	ConfigPath string `json:"config_path,omitempty"`
	// NameFlag is the --name flag value.
	NameFlag string `json:"name_flag,omitempty"`
}

// HubConfig holds the hub configuration.
type HubConfig struct {
	// Port is the UDP port to listen on.
	Port int `json:"port"`
	// Agents is the list of configured agents.
	Agents []AgentConfig `json:"agents"`
	// CoShellPath is the path to the co-shell executable.
	CoShellPath string `json:"co_shell_path,omitempty"`
	// Workspace is the base workspace directory.
	Workspace string `json:"workspace"`
}

// DefaultConfig returns the default hub configuration.
func DefaultConfig() *HubConfig {
	return &HubConfig{
		Port:        8080,
		CoShellPath: "co-shell",
		Workspace:   ".",
		Agents: []AgentConfig{
			{
				ID:   "default",
				Name: "默认助手",
			},
		},
	}
}

// LoadConfig loads the hub configuration from a JSON file.
func LoadConfig(path string) (*HubConfig, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("cannot parse config file: %w", err)
	}

	return cfg, nil
}

// Hub manages multiple co-shell agent instances and UDP communication.
type Hub struct {
	config  *HubConfig
	agents  map[string]*AgentSession
	mu      sync.RWMutex
	udpConn *net.UDPConn
	ctx     context.Context
	cancel  context.CancelFunc
}

// AgentSession represents a running co-shell agent session.
type AgentSession struct {
	config    AgentConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	mu        sync.RWMutex
}

// New creates a new Hub instance.
func New(config *HubConfig) (*Hub, error) {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Hub{
		config: config,
		agents: make(map[string]*AgentSession),
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize agent sessions
	for _, agentCfg := range config.Agents {
		h.agents[agentCfg.ID] = &AgentSession{
			config: agentCfg,
		}
	}

	return h, nil
}

// Start starts the hub service.
func (h *Hub) Start() error {
	log.Printf("Starting co-shell-hub...")
	log.Printf("UDP port: %d", h.config.Port)
	log.Printf("Agents: %d", len(h.config.Agents))

	// Start UDP listener
	if err := h.startUDP(); err != nil {
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}

	log.Println("co-shell-hub started")
	return nil
}

// Stop stops the hub service.
func (h *Hub) Stop() {
	log.Println("Stopping co-shell-hub...")
	h.cancel()

	h.mu.Lock()
	for _, agent := range h.agents {
		agent.Stop()
	}
	h.mu.Unlock()

	if h.udpConn != nil {
		h.udpConn.Close()
	}

	log.Println("co-shell-hub stopped")
}

// startUDP starts the UDP listener.
func (h *Hub) startUDP() error {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: h.config.Port,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port %d: %w", h.config.Port, err)
	}

	h.udpConn = conn

	// Start reading loop
	go h.readLoop(conn)

	log.Printf("UDP listener started on port %d", h.config.Port)
	return nil
}

// readLoop reads incoming UDP packets and processes them.
func (h *Hub) readLoop(conn *net.UDPConn) {
	buf := make([]byte, 65535)

	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if h.ctx.Err() == nil {
				log.Printf("UDP read error: %v", err)
			}
			continue
		}

		go h.handleMessage(buf[:n], remoteAddr)
	}
}

// handleMessage processes an incoming message from a client.
func (h *Hub) handleMessage(data []byte, remoteAddr *net.UDPAddr) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(remoteAddr, "invalid JSON")
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		h.sendError(remoteAddr, "missing type field")
		return
	}

	switch msgType {
	case "handshake":
		h.handleHandshake(remoteAddr, msg)
	case "message":
		h.handleClientMessage(remoteAddr, msg)
	case "get_agents":
		h.handleGetAgents(remoteAddr)
	default:
		h.sendError(remoteAddr, fmt.Sprintf("unknown message type: %s", msgType))
	}
}

// handleHandshake responds to handshake requests.
func (h *Hub) handleHandshake(remoteAddr *net.UDPAddr, msg map[string]interface{}) {
	response := map[string]interface{}{
		"type":        "handshake_ack",
		"timestamp":   time.Now().UnixMilli(),
		"hub_version": "0.1.0",
	}

	h.sendJSON(remoteAddr, response)
}

// handleGetAgents returns the list of available agents.
func (h *Hub) handleGetAgents(remoteAddr *net.UDPAddr) {
	h.mu.RLock()
	agents := make([]map[string]interface{}, 0)
	for id, agent := range h.agents {
		agents = append(agents, map[string]interface{}{
			"id":   id,
			"name": agent.config.Name,
		})
	}
	h.mu.RUnlock()

	response := map[string]interface{}{
		"type":   "agents_list",
		"agents": agents,
	}

	h.sendJSON(remoteAddr, response)
}

// handleClientMessage routes a client message to the appropriate agent.
func (h *Hub) handleClientMessage(remoteAddr *net.UDPAddr, msg map[string]interface{}) {
	agentID, _ := msg["agent_id"].(string)
	if agentID == "" {
		agentID = "default"
	}

	content, _ := msg["content"].(string)

	h.mu.RLock()
	agent, exists := h.agents[agentID]
	h.mu.RUnlock()

	if !exists {
		h.sendError(remoteAddr, fmt.Sprintf("agent not found: %s", agentID))
		return
	}

	// Start agent if not running
	if !agent.IsRunning() {
		if err := agent.Start(h.config.CoShellPath, h.config.Workspace); err != nil {
			log.Printf("Failed to start agent %s: %v", agentID, err)
			h.sendError(remoteAddr, fmt.Sprintf("failed to start agent: %v", err))
			return
		}
	}

	// Send message to agent
	if err := agent.Send(content); err != nil {
		h.sendError(remoteAddr, fmt.Sprintf("failed to send message: %v", err))
		return
	}

	// Read response from agent (blocking with timeout)
	go func() {
		response, err := agent.ReadResponse(30 * time.Second)
		if err != nil {
			log.Printf("Agent %s response error: %v", agentID, err)
			return
		}

		responseMsg := map[string]interface{}{
			"type":      "message",
			"content":   response,
			"timestamp": time.Now().UnixMilli(),
		}

		h.sendJSON(remoteAddr, responseMsg)
	}()
}

// sendJSON sends a JSON message to a remote UDP address.
func (h *Hub) sendJSON(remoteAddr *net.UDPAddr, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	_, err = h.udpConn.WriteTo(bytes, remoteAddr)
	if err != nil {
		log.Printf("UDP write error: %v", err)
	}
}

// sendError sends an error message to a remote UDP address.
func (h *Hub) sendError(remoteAddr *net.UDPAddr, errMsg string) {
	response := map[string]interface{}{
		"type":    "error",
		"message": errMsg,
	}
	h.sendJSON(remoteAddr, response)
}

// Run starts the hub and waits for shutdown signal.
func (h *Hub) Run() {
	if err := h.Start(); err != nil {
		log.Fatalf("Failed to start hub: %v", err)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	h.Stop()
}
