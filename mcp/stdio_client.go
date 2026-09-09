package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// stdioClient is a self-contained MCP client that talks to a local server
// process over stdio using newline-delimited JSON-RPC 2.0 messages.
type stdioClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	scanner     *bufio.Scanner
	requestID   atomic.Int64
	responses   map[int64]chan jsonrpcResponse
	mu          sync.Mutex
	initialized bool
	done        chan struct{}
	closeOnce   sync.Once
}

// newStdioClient launches the given command and returns a stdio MCP client.
func newStdioClient(command string, args []string) (*stdioClient, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot open stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot open stdout: %w", err)
	}
	// Send the server's stderr to our own stderr so errors are visible.
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start %q: %w", command, err)
	}

	c := &stdioClient{
		cmd:       cmd,
		stdin:     stdin,
		scanner:   bufio.NewScanner(stdout),
		responses: make(map[int64]chan jsonrpcResponse),
		done:      make(chan struct{}),
	}
	// Increase scanner buffer for large tool results.
	c.scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	go c.readLoop()
	return c, nil
}

// readLoop reads newline-delimited JSON-RPC responses from the server.
func (c *stdioClient) readLoop() {
	defer close(c.done)
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.responses[resp.ID]
		if ok {
			delete(c.responses, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

// sendRequest sends a JSON-RPC request and waits for its response.
func (c *stdioClient) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}

	id := c.requestID.Add(1)
	req := jsonrpcRequest{JSONRPC: jsonrpcVersion, ID: id, Method: method}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		req.Params = raw
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ch := make(chan jsonrpcResponse, 1)
	c.mu.Lock()
	c.responses[id] = ch
	c.mu.Unlock()

	if _, err := c.stdin.Write(append(body, '\n')); err != nil {
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("server process exited")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// initialize performs the MCP initialize handshake.
func (c *stdioClient) initialize(ctx context.Context) error {
	params := initializeParams{
		ProtocolVersion: mcpProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      implementation{Name: "co-shell", Version: "0.1.0"},
	}
	if _, err := c.sendRequest(ctx, "initialize", params); err != nil {
		return err
	}
	// Send the initialized notification.
	notif := jsonrpcNotification{JSONRPC: jsonrpcVersion, Method: "notifications/initialized"}
	body, _ := json.Marshal(notif)
	if _, err := c.stdin.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}
	c.initialized = true
	return nil
}

// listTools returns the tools exposed by the server.
func (c *stdioClient) listTools(ctx context.Context) ([]tool, error) {
	raw, err := c.sendRequest(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var res listToolsResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list: %w", err)
	}
	return res.Tools, nil
}

// callTool invokes a tool on the server.
func (c *stdioClient) callTool(ctx context.Context, name string, args map[string]any) (*callToolResult, error) {
	params := callToolParams{Name: name, Arguments: args}
	raw, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	var res callToolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call: %w", err)
	}
	return &res, nil
}

// close terminates the server process.
func (c *stdioClient) close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.cmd.Process != nil {
			err = c.cmd.Process.Kill()
		}
		c.stdin.Close()
	})
	return err
}
