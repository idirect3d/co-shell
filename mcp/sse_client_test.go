package mcp

import (
	"context"
	"fmt"
	"testing"
)

// TestSSEClientConnect verifies the self-contained SSE client can connect to a
// server that returns a relative-path endpoint. Requires a live SSE server at
// http://localhost:8080/sse; skipped when it is unreachable.
func TestSSEClientConnect(t *testing.T) {
	c, err := newSSEClient("http://localhost:8080/sse")
	if err != nil {
		t.Fatalf("newSSEClient: %v", err)
	}
	if err := c.start(context.Background()); err != nil {
		t.Skipf("SSE server not reachable, skipping: %v", err)
	}
	defer c.close()

	if err := c.initialize(context.Background()); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	tools, err := c.listTools(context.Background())
	if err != nil {
		t.Fatalf("listTools: %v", err)
	}
	fmt.Printf("connected OK, %d tools\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  - %s\n", tool.Name)
	}
}
