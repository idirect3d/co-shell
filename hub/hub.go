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
	"path/filepath"
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
	// AutoStart indicates whether to start this agent on hub startup.
	AutoStart bool `json:"auto_start,omitempty"`
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
	// Auth holds the authentication configuration.
	Auth *AuthConfig `json:"auth,omitempty"`
	// LazyMode indicates whether to start agents on demand (when message received).
	LazyMode bool `json:"lazy_mode,omitempty"`
}

// DefaultConfig returns the default hub configuration.
func DefaultConfig() *HubConfig {
	return &HubConfig{
		Port:        12800,
		CoShellPath: "co-shell",
		Workspace:   ".",
		LazyMode:    true,
		Agents:      []AgentConfig{},
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

// SaveConfig saves the hub configuration to a JSON file.
// It preserves the auth section from the existing config if present.
func SaveConfig(path string, cfg *HubConfig) error {
	// Load existing config to preserve auth section
	existing, err := os.ReadFile(path)
	if err == nil {
		var existingCfg map[string]interface{}
		if err := json.Unmarshal(existing, &existingCfg); err == nil {
			if authData, ok := existingCfg["auth"]; ok {
				// Marshal cfg without auth
				cfg.Auth = nil
				data, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal config: %w", err)
				}
				// Unmarshal back to map to merge auth
				var cfgMap map[string]interface{}
				if err := json.Unmarshal(data, &cfgMap); err == nil {
					cfgMap["auth"] = authData
					data, err = json.MarshalIndent(cfgMap, "", "  ")
					if err != nil {
						return fmt.Errorf("failed to marshal config with auth: %w", err)
					}
					return os.WriteFile(path, data, 0644)
				}
			}
		}
	}

	// No existing auth section, just save normally
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// DiscoverAgents scans the workspace directory for subdirectories containing
// co-shell config.json files and returns them as AgentConfig entries.
func DiscoverAgents(workspace string) []AgentConfig {
	var agents []AgentConfig

	entries, err := os.ReadDir(workspace)
	if err != nil {
		log.Printf("Cannot read workspace %s: %v", workspace, err)
		return agents
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories
		if entry.Name()[0] == '.' {
			continue
		}

		configPath := filepath.Join(workspace, entry.Name(), "config.json")
		if _, err := os.Stat(configPath); err == nil {
			agents = append(agents, AgentConfig{
				ID:         entry.Name(),
				Name:       entry.Name(),
				Workspace:  filepath.Join(workspace, entry.Name()),
				ConfigPath: configPath,
			})
			log.Printf("Discovered agent: %s (workspace: %s)", entry.Name(), filepath.Join(workspace, entry.Name()))
		}
	}

	return agents
}

// Hub manages multiple co-shell agent instances and UDP communication.
type Hub struct {
	config  *HubConfig
	agents  map[string]*AgentSession
	auth    *AuthConfig
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
func New(config *HubConfig, auth *AuthConfig) (*Hub, error) {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Hub{
		config: config,
		agents: make(map[string]*AgentSession),
		auth:   auth,
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
	log.Printf("Workspace: %s", h.config.Workspace)
	log.Printf("Lazy mode: %v", h.config.LazyMode)
	log.Printf("Agents: %d", len(h.config.Agents))

	// Start UDP listener
	if err := h.startUDP(); err != nil {
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}

	// Start agents that have AutoStart enabled (only if not in lazy mode)
	if !h.config.LazyMode {
		h.mu.RLock()
		for id, agent := range h.agents {
			if agent.config.AutoStart {
				h.mu.RUnlock()
				if err := h.startAgent(id); err != nil {
					log.Printf("Failed to auto-start agent %s: %v", id, err)
				}
				h.mu.RLock()
			}
		}
		h.mu.RUnlock()
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
	case "create_agent":
		h.handleCreateAgent(remoteAddr, msg)
	default:
		h.sendError(remoteAddr, fmt.Sprintf("unknown message type: %s", msgType))
	}
}

// handleHandshake responds to handshake requests.
func (h *Hub) handleHandshake(remoteAddr *net.UDPAddr, msg map[string]interface{}) {
	pubKey, _ := h.auth.GetHubPublicKey()

	response := map[string]interface{}{
		"type":        "handshake_ack",
		"timestamp":   time.Now().UnixMilli(),
		"hub_version": "0.1.0",
		"public_key":  pubKey,
	}

	h.sendJSON(remoteAddr, response)
}

// handleGetAgents returns the list of available agents.
func (h *Hub) handleGetAgents(remoteAddr *net.UDPAddr) {
	h.mu.RLock()
	agents := make([]map[string]interface{}, 0)
	for id, agent := range h.agents {
		agents = append(agents, map[string]interface{}{
			"id":         id,
			"name":       agent.config.Name,
			"is_running": agent.IsRunning(),
		})
	}
	h.mu.RUnlock()

	response := map[string]interface{}{
		"type":   "agents_list",
		"agents": agents,
	}

	h.sendJSON(remoteAddr, response)
}

// handleCreateAgent creates a new agent from a mobile client request.
func (h *Hub) handleCreateAgent(remoteAddr *net.UDPAddr, msg map[string]interface{}) {
	name, _ := msg["name"].(string)
	if name == "" {
		h.sendError(remoteAddr, "missing name field")
		return
	}

	count := 1
	if countVal, ok := msg["count"].(float64); ok {
		count = int(countVal)
		if count < 1 {
			count = 1
		}
	}

	// Create agent folders
	created := make([]AgentConfig, 0)
	for i := 1; i <= count; i++ {
		agentID := fmt.Sprintf("%s-%d", name, i)
		agentWorkspace := filepath.Join(h.config.Workspace, agentID)

		// Create workspace directory
		if err := os.MkdirAll(agentWorkspace, 0755); err != nil {
			h.sendError(remoteAddr, fmt.Sprintf("failed to create workspace for %s: %v", agentID, err))
			return
		}

		// For the first agent, create a default config.json
		if i == 1 {
			defaultConfig := map[string]interface{}{
				"llm": map[string]interface{}{
					"temperature":     0.5,
					"max_iterations":  1000,
					"confirm_command": true,
				},
				"mcp": map[string]interface{}{
					"servers": []interface{}{},
				},
				"rules": []interface{}{},
			}
			configData, _ := json.MarshalIndent(defaultConfig, "", "  ")
			os.WriteFile(filepath.Join(agentWorkspace, "config.json"), configData, 0644)
		} else {
			// Copy config from the first agent
			srcConfig := filepath.Join(h.config.Workspace, fmt.Sprintf("%s-%d", name, 1), "config.json")
			dstConfig := filepath.Join(agentWorkspace, "config.json")
			if data, err := os.ReadFile(srcConfig); err == nil {
				os.WriteFile(dstConfig, data, 0644)
			}
		}

		agentCfg := AgentConfig{
			ID:         agentID,
			Name:       agentID,
			Workspace:  agentWorkspace,
			ConfigPath: filepath.Join(agentWorkspace, "config.json"),
		}

		h.mu.Lock()
		h.agents[agentID] = &AgentSession{
			config: agentCfg,
		}
		h.mu.Unlock()

		created = append(created, agentCfg)
		log.Printf("Created agent: %s (workspace: %s)", agentID, agentWorkspace)
	}

	// Save updated config
	h.config.Agents = append(h.config.Agents, created...)

	response := map[string]interface{}{
		"type":   "agent_created",
		"agents": created,
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

	// Start agent if not running (lazy mode)
	if !agent.IsRunning() {
		if err := h.startAgent(agentID); err != nil {
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

// startAgent starts a co-shell agent process.
func (h *Hub) startAgent(agentID string) error {
	h.mu.RLock()
	agent, exists := h.agents[agentID]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	workspace := agent.config.Workspace
	if workspace == "" {
		workspace = h.config.Workspace
	}

	return agent.Start(h.config.CoShellPath, workspace)
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
