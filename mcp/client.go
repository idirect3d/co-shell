package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
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
	Name   string
	Alive  bool
	Tools  []ToolInfo
	Error  string
}

// Manager manages multiple MCP server connections.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*mcpClient
}

// mcpClient wraps a single MCP client connection.
type mcpClient struct {
	client client.MCPClient
	tools  []ToolInfo
	name   string
}

// NewManager creates a new MCP manager.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*mcpClient),
	}
}

// AddServer starts and connects to an MCP server.
func (m *Manager) AddServer(name, command string, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("server %q already exists", name)
	}

	// Create stdio-based MCP client
	c, err := client.NewStdioMCPClient(
		command,
		args...,
	)
	if err != nil {
		return fmt.Errorf("cannot create MCP client for %q: %w", name, err)
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "co-shell",
		Version: "0.1.0",
	}

	_, err = c.Initialize(context.Background(), initRequest)
	if err != nil {
		c.Close()
		return fmt.Errorf("cannot initialize MCP server %q: %w", name, err)
	}

	// List available tools
	toolsResult, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		c.Close()
		return fmt.Errorf("cannot list tools from %q: %w", name, err)
	}

	var tools []ToolInfo
	for _, t := range toolsResult.Tools {
		tools = append(tools, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  name,
		})
	}

	m.servers[name] = &mcpClient{
		client: c,
		tools:  tools,
		name:   name,
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

	err := c.client.Close()
	delete(m.servers, name)
	return err
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

	// Find which server has this tool
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

	// Convert args to JSON
	argBytes, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("cannot marshal arguments: %w", err)
	}

	var rawArgs map[string]interface{}
	if err := json.Unmarshal(argBytes, &rawArgs); err != nil {
		return "", fmt.Errorf("cannot unmarshal arguments: %w", err)
	}

	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = toolName
	callRequest.Params.Arguments = rawArgs

	result, err := target.client.CallTool(ctx, callRequest)
	if err != nil {
		return "", fmt.Errorf("cannot call tool %q: %w", toolName, err)
	}

	// Format the result
	var output string
	for _, content := range result.Content {
		switch v := content.(type) {
		case mcp.TextContent:
			output += v.Text
		case mcp.ImageContent:
			output += fmt.Sprintf("[Image: %s]", v.MIMEType)
		case mcp.ResourceContent:
			output += fmt.Sprintf("[Resource: %s]", v.URI)
		default:
			output += fmt.Sprintf("[Unknown content type: %T]", content)
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
		if err := c.client.Close(); err != nil {
			lastErr = fmt.Errorf("error closing %q: %w", name, err)
		}
	}
	m.servers = make(map[string]*mcpClient)
	return lastErr
}

// StdioMCPClient is a simple stdio-based MCP client implementation.
type StdioMCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr io.ReadCloser
	mu     sync.Mutex
}

// NewStdioMCPClient creates a new stdio-based MCP client.
func NewStdioMCPClient(command string, args ...string) (*StdioMCPClient, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start MCP server: %w", err)
	}

	return &StdioMCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		stderr: stderr,
	}, nil
}

// SendRequest sends a JSON-RPC request and reads the response.
func (c *StdioMCPClient) SendRequest(request []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Write request
	if _, err := c.stdin.Write(request); err != nil {
		return nil, fmt.Errorf("cannot write request: %w", err)
	}
	if _, err := c.stdin.Write([]byte("\n")); err != nil {
		return nil, fmt.Errorf("cannot write newline: %w", err)
	}

	// Read response
	if c.stdout.Scan() {
		return c.stdout.Bytes(), nil
	}

	if err := c.stdout.Err(); err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}

	return nil, io.EOF
}

// Close terminates the MCP server process.
func (c *StdioMCPClient) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}
