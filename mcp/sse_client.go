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

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// sseClient is a fixed SSE MCP client that correctly handles a relative-path
// "endpoint" event (e.g. "/message?sessionId=...") by resolving it against the
// base URL. The upstream mark3labs/mcp-go v0.8.3 SSEMCPClient rejects such
// endpoints because it compares endpoint.Host (empty for a relative path)
// against baseURL.Host, so it never sets the endpoint and later fails with
// "endpoint not received". This implementation drops that host check and
// resolves relative endpoints with baseURL.ResolveReference.
type sseClient struct {
	baseURL       *url.URL
	endpoint      *url.URL
	httpClient    *http.Client
	requestID     atomic.Int64
	responses     map[int64]chan client.RPCResponse
	mu            sync.RWMutex
	done          chan struct{}
	initialized   bool
	endpointChan  chan struct{}
	capabilities  mcp.ServerCapabilities
}

// newSSEClient creates a fixed SSE MCP client for the given base URL.
func newSSEClient(baseURL string) (*sseClient, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return &sseClient{
		baseURL:      parsedURL,
		httpClient:   &http.Client{},
		responses:    make(map[int64]chan client.RPCResponse),
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
		// Resolve a relative endpoint (e.g. "/message?sessionId=...") against
		// the base URL so it becomes an absolute URL. This is the fix over the
		// upstream client, which rejects relative endpoints.
		if !endpoint.IsAbs() {
			endpoint = c.baseURL.ResolveReference(endpoint)
		}
		c.endpoint = endpoint
		close(c.endpointChan)

	case "message":
		var baseMessage struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id,omitempty"`
			Method  string          `json:"method,omitempty"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &baseMessage); err != nil {
			return
		}
		if baseMessage.ID == nil {
			return // notification; not used by co-shell
		}
		c.mu.RLock()
		ch, ok := c.responses[*baseMessage.ID]
		c.mu.RUnlock()
		if ok {
			if baseMessage.Error != nil {
				ch <- client.RPCResponse{Error: &baseMessage.Error.Message}
			} else {
				ch <- client.RPCResponse{Response: &baseMessage.Result}
			}
			c.mu.Lock()
			delete(c.responses, *baseMessage.ID)
			c.mu.Unlock()
		}
	}
}

func (c *sseClient) sendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}
	if c.endpoint == nil {
		return nil, fmt.Errorf("endpoint not received")
	}

	id := c.requestID.Add(1)
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Request: mcp.Request{Method: method},
	}
	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &request.Params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal params: %w", err)
		}
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	responseChan := make(chan client.RPCResponse, 1)
	c.mu.Lock()
	c.responses[id] = responseChan
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint.String(), bytes.NewReader(requestBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		if response.Error != nil {
			return nil, errors.New(*response.Error)
		}
		return response.Response, nil
	}
}

// Initialize implements client.MCPClient.
func (c *sseClient) Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	params := struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ClientInfo      mcp.Implementation     `json:"clientInfo"`
		Capabilities    mcp.ClientCapabilities `json:"capabilities"`
	}{
		ProtocolVersion: request.Params.ProtocolVersion,
		ClientInfo:      request.Params.ClientInfo,
		Capabilities:    request.Params.Capabilities,
	}
	response, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}
	var result mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	c.capabilities = result.Capabilities

	// Send initialized notification.
	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{Method: "notifications/initialized"},
	}
	notificationBytes, _ := json.Marshal(notification)
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint.String(), bytes.NewReader(notificationBytes))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		if resp, err := c.httpClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	c.initialized = true
	return &result, nil
}

// Ping implements client.MCPClient.
func (c *sseClient) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

// ListTools implements client.MCPClient.
func (c *sseClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	response, err := c.sendRequest(ctx, "tools/list", request.Params)
	if err != nil {
		return nil, err
	}
	var result mcp.ListToolsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// CallTool implements client.MCPClient.
func (c *sseClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	response, err := c.sendRequest(ctx, "tools/call", request.Params)
	if err != nil {
		return nil, err
	}
	var result mcp.CallToolResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// Close implements client.MCPClient.
func (c *sseClient) Close() error {
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
	c.responses = make(map[int64]chan client.RPCResponse)
	c.mu.Unlock()
	return nil
}

// OnNotification implements client.MCPClient (notifications are not used by
// co-shell, so this is a no-op).
func (c *sseClient) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
}

// The following methods are part of the client.MCPClient interface but are not
// used by co-shell; they return a "not supported" error.

func (c *sseClient) ListResources(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	return nil, fmt.Errorf("resources not supported")
}
func (c *sseClient) ListResourceTemplates(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	return nil, fmt.Errorf("resource templates not supported")
}
func (c *sseClient) ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return nil, fmt.Errorf("read resource not supported")
}
func (c *sseClient) Subscribe(ctx context.Context, request mcp.SubscribeRequest) error {
	return fmt.Errorf("subscribe not supported")
}
func (c *sseClient) Unsubscribe(ctx context.Context, request mcp.UnsubscribeRequest) error {
	return fmt.Errorf("unsubscribe not supported")
}
func (c *sseClient) ListPrompts(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	return nil, fmt.Errorf("prompts not supported")
}
func (c *sseClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("get prompt not supported")
}
func (c *sseClient) SetLevel(ctx context.Context, request mcp.SetLevelRequest) error {
	return fmt.Errorf("set level not supported")
}
func (c *sseClient) Complete(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return nil, fmt.Errorf("complete not supported")
}
