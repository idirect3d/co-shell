package mcp

import "encoding/json"

// JSON-RPC 2.0 protocol version.
const jsonrpcVersion = "2.0"

// MCP protocol version supported by this client.
const mcpProtocolVersion = "2024-11-05"

// jsonrpcRequest is a JSON-RPC 2.0 request message.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is a JSON-RPC 2.0 response message.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError is a JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// jsonrpcNotification is a JSON-RPC 2.0 notification (no id).
type jsonrpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// implementation identifies the client to the server (initialize).
type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// initializeParams carries the initialize request parameters.
type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      implementation `json:"clientInfo"`
}

// initializeResult is the server's response to initialize.
type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      implementation `json:"serverInfo"`
}

// tool is an MCP tool definition returned by tools/list.
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// listToolsResult is the server's response to tools/list.
type listToolsResult struct {
	Tools []tool `json:"tools"`
}

// callToolParams carries the tools/call request parameters.
type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// content is a single content item in a tools/call result. It is decoded
// generically so both text and image content are handled.
type content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// callToolResult is the server's response to tools/call.
type callToolResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}
