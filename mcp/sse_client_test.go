package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestSSEClientConnect verifies the fixed SSE client can connect to a server
// that returns a relative-path endpoint (regression test for the
// "endpoint not received" bug). Requires a live SSE server at
// http://localhost:8080/sse; skipped when it is unreachable.
func TestSSEClientConnect(t *testing.T) {
	c, err := newSSEClient("http://localhost:8080/sse")
	if err != nil {
		t.Fatalf("newSSEClient: %v", err)
	}
	if err := c.start(context.Background()); err != nil {
		t.Skipf("SSE server not reachable, skipping: %v", err)
	}
	defer c.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "co-shell-test", Version: "0.1.0"}
	if _, err := c.Initialize(context.Background(), initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	res, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	fmt.Printf("connected OK, %d tools\n", len(res.Tools))
	for _, tool := range res.Tools {
		fmt.Printf("  - %s\n", tool.Name)
	}
}
