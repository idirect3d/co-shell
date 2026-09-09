// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-09-10
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/log"
)

// ToolInfo holds metadata about an MCP tool.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	ServerName  string
}

// ServerStatus represents the status of an MCP server connection.
type ServerStatus struct {
	Name  string
	Alive bool
	Tools []ToolInfo
	Error string
}

// conn is the transport-agnostic interface implemented by both the stdio and
// SSE MCP clients. It is fully self-contained (no third-party MCP dependency).
type conn interface {
	initialize(ctx context.Context) error
	listTools(ctx context.Context) ([]tool, error)
	callTool(ctx context.Context, name string, args map[string]any) (*callToolResult, error)
	close() error
}

// Manager manages multiple MCP server connections.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*mcpClient
}

// mcpClient wraps a single MCP client connection.
type mcpClient struct {
	conn  conn
	tools []ToolInfo
	name  string
}

// NewManager creates a new MCP manager.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*mcpClient),
	}
}

// AddServer starts and connects to an MCP server. When url is non-empty the
// server is connected over SSE (remote); otherwise it is launched as a local
// stdio process.
func (m *Manager) AddServer(name, command string, args []string, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("server %q already exists", name)
	}

	var c conn
	var err error
	if url != "" {
		// Remote SSE server.
		sse, err := newSSEClient(url)
		if err != nil {
			return fmt.Errorf("cannot create MCP client for %q: %w", name, err)
		}
		if err := sse.start(context.Background()); err != nil {
			return fmt.Errorf("cannot connect to MCP server %q: %w", name, err)
		}
		c = sse
	} else {
		// Local stdio server.
		c, err = newStdioClient(command, args)
		if err != nil {
			return fmt.Errorf("cannot create MCP client for %q: %w", name, err)
		}
	}
	if err != nil {
		return fmt.Errorf("cannot create MCP client for %q: %w", name, err)
	}

	// Initialize the client.
	if err := c.initialize(context.Background()); err != nil {
		c.close()
		return fmt.Errorf("cannot initialize MCP server %q: %w", name, err)
	}

	// List available tools.
	toolsResult, err := c.listTools(context.Background())
	if err != nil {
		c.close()
		return fmt.Errorf("cannot list tools from %q: %w", name, err)
	}

	var tools []ToolInfo
	for _, t := range toolsResult {
		tools = append(tools, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  name,
		})
	}

	m.servers[name] = &mcpClient{
		conn:  c,
		tools: tools,
		name:  name,
	}

	return nil
}

// RemoveServer disconnects and removes an MCP server.
func (m *Manager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, exists := m.servers[name]
	if !exists {
		return fmt.Errorf("server %q not found", name)
	}

	err := c.conn.close()
	delete(m.servers, name)
	return err
}

// TestServer reconnects to an MCP server (connectivity test) and refreshes its
// cached tool list, returning the current tool call names.
func (m *Manager) TestServer(name, command string, args []string, url string) ([]string, error) {
	// Drop any existing connection so AddServer can reconnect fresh.
	if err := m.RemoveServer(name); err != nil {
		// Ignore "not found" — the server may simply not be connected yet.
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}
	}
	if err := m.AddServer(name, command, args, url); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	var names []string
	if c, ok := m.servers[name]; ok {
		for _, t := range c.tools {
			names = append(names, t.Name)
		}
	}
	return names, nil
}

// ListServers returns the status of all connected MCP servers.
func (m *Manager) ListServers() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var statuses []ServerStatus
	for name, c := range m.servers {
		statuses = append(statuses, ServerStatus{
			Name:  name,
			Alive: true,
			Tools: c.tools,
		})
	}
	return statuses
}

// GetAllTools returns all tools from all connected MCP servers.
func (m *Manager) GetAllTools() []ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []ToolInfo
	for _, c := range m.servers {
		allTools = append(allTools, c.tools...)
	}
	return allTools
}

// CallTool invokes a tool on the appropriate MCP server.
func (m *Manager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find which server has this tool.
	var target *mcpClient
	for _, c := range m.servers {
		for _, t := range c.tools {
			if t.Name == toolName {
				target = c
				break
			}
		}
		if target != nil {
			break
		}
	}

	if target == nil {
		return "", fmt.Errorf("tool %q not found on any MCP server", toolName)
	}

	// Convert args to JSON.
	argBytes, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("cannot marshal arguments: %w", err)
	}
	var rawArgs map[string]interface{}
	if err := json.Unmarshal(argBytes, &rawArgs); err != nil {
		return "", fmt.Errorf("cannot unmarshal arguments: %w", err)
	}

	log.Info("MCP CallTool: server=%s, tool=%s, args=%v", target.name, toolName, args)

	result, err := target.conn.callTool(ctx, toolName, rawArgs)
	if err != nil {
		log.Error("MCP CallTool failed: server=%s, tool=%s, error: %v", target.name, toolName, err)
		return "", fmt.Errorf("cannot call tool %q: %w", toolName, err)
	}

	// Format the result.
	var output string
	for _, content := range result.Content {
		switch content.Type {
		case "text":
			output += content.Text
		case "image":
			output += fmt.Sprintf("[Image: %s]", content.MIMEType)
		default:
			output += fmt.Sprintf("[Content: %s]", content.Type)
		}
	}

	return output, nil
}

// Close disconnects all MCP servers.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, c := range m.servers {
		if err := c.conn.close(); err != nil {
			lastErr = fmt.Errorf("error closing %q: %w", name, err)
		}
	}
	m.servers = make(map[string]*mcpClient)
	return lastErr
}
