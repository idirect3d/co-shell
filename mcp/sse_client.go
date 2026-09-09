package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// sseClient is a self-contained SSE MCP client. It connects to a remote server
// over Server-Sent Events, waits for the "endpoint" event, then sends JSON-RPC
// requests over HTTP POST to that endpoint while receiving responses over the
// SSE stream. It correctly handles a relative-path endpoint (e.g.
// "/message?sessionId=...") by resolving it against the base URL.
type sseClient struct {
	baseURL      *url.URL
	endpoint     *url.URL
	httpClient   *http.Client
	requestID    atomic.Int64
	responses    map[int64]chan jsonrpcResponse
	mu           sync.Mutex
	done         chan struct{}
	initialized  bool
	endpointChan chan struct{}
}

// newSSEClient creates an SSE MCP client for the given base URL.
func newSSEClient(baseURL string) (*sseClient, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return &sseClient{
		baseURL:      parsedURL,
		httpClient:   &http.Client{},
		responses:    make(map[int64]chan jsonrpcResponse),
		done:         make(chan struct{}),
		endpointChan: make(chan struct{}),
	}, nil
}

// start opens the SSE stream and waits for the endpoint event.
func (c *sseClient) start(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	go c.readSSE(resp.Body)

	select {
	case <-c.endpointChan:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for endpoint")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for endpoint")
	}
}

func (c *sseClient) readSSE(reader io.ReadCloser) {
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var event, data string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if event != "" && data != "" {
				c.handleSSEEvent(event, data)
				event = ""
				data = ""
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

func (c *sseClient) handleSSEEvent(event, data string) {
	switch event {
	case "endpoint":
		endpoint, err := url.Parse(data)
		if err != nil {
			return
		}
		// Resolve a relative endpoint against the base URL so it becomes
		// absolute (the upstream mcp-go client rejects relative endpoints).
		if !endpoint.IsAbs() {
			endpoint = c.baseURL.ResolveReference(endpoint)
		}
		c.endpoint = endpoint
		close(c.endpointChan)

	case "message":
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return
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

func (c *sseClient) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}
	if c.endpoint == nil {
		return nil, fmt.Errorf("endpoint not received")
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, respBody)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case rpcResp := <-ch:
		if rpcResp.Error != nil {
			return nil, errors.New(rpcResp.Error.Message)
		}
		return rpcResp.Result, nil
	}
}

// initialize performs the MCP initialize handshake.
func (c *sseClient) initialize(ctx context.Context) error {
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
	notifBody, _ := json.Marshal(notif)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint.String(), bytes.NewReader(notifBody))
	if err == nil {
		httpReq.Header.Set("Content-Type", "application/json")
		if resp, err := c.httpClient.Do(httpReq); err == nil {
			resp.Body.Close()
		}
	}
	c.initialized = true
	return nil
}

// listTools returns the tools exposed by the server.
func (c *sseClient) listTools(ctx context.Context) ([]tool, error) {
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
func (c *sseClient) callTool(ctx context.Context, name string, args map[string]any) (*callToolResult, error) {
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

// close shuts down the SSE client.
func (c *sseClient) close() error {
	select {
	case <-c.done:
		return nil
	default:
		close(c.done)
	}
	c.mu.Lock()
	for _, ch := range c.responses {
		close(ch)
	}
	c.responses = make(map[int64]chan jsonrpcResponse)
	c.mu.Unlock()
	return nil
}
